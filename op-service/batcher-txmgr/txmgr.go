package batcher_txmgr

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	kzgsdk "github.com/domicon-labs/kzg-sdk"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/txmgr/metrics"
	domiconabi "github.com/ethereum-optimism/optimism/packages/domicon-abi"
)

const (
	// Geth requires a minimum fee bump of 10% for tx resubmission
	priceBump int64 = 10
	dSrsSize        = 1 << 16
)

// new = old * (100 + priceBump) / 100
var (
	priceBumpPercent = big.NewInt(100 + priceBump)
	oneHundred       = big.NewInt(100)
	ninetyNine       = big.NewInt(99)
)

// TxManager is an interface that allows callers to reliably publish txs,
// bumping the gas price if needed, and obtain the receipt of the resulting tx.
//
//go:generate mockery --name TxManager --output ./mocks
type TxManager interface {
	// Send is used to create & send a transaction. It will handle increasing
	// the gas price & ensuring that the transaction remains in the transaction pool.
	// It can be stopped by cancelling the provided context; however, the transaction
	// may be included on L1 even if the context is cancelled.
	//
	// NOTE: Send can be called concurrently, the nonce will be managed internally.
	Send(ctx context.Context, candidate TxCandidate) (*types.Receipt, error)

	// From returns the sending address associated with the instance of the transaction manager.
	// It is static for a single instance of a TxManager.
	From() common.Address

	// BlockNumber returns the most recent block number from the underlying network.
	BlockNumber(ctx context.Context) (uint64, error)

	// Close the underlying connection
	Close()
}

// ETHBackend is the set of methods that the transaction manager uses to resubmit gas & determine
// when transactions are included on L1.
type ETHBackend interface {
	// BlockNumber returns the most recent block number.
	BlockNumber(ctx context.Context) (uint64, error)

	// CallContract executes an eth_call against the provided contract.
	CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)

	// TransactionReceipt queries the backend for a receipt associated with
	// txHash. If lookup does not fail, but the transaction is not found,
	// nil should be returned for both values.
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)

	// SendTransaction submits a signed transaction to L1.
	SendTransaction(ctx context.Context, tx *types.Transaction) error

	// These functions are used to estimate what the basefee & priority fee should be set to.
	// TODO(CLI-3318): Maybe need a generic interface to support different RPC providers
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	SuggestGasTipCap(ctx context.Context) (*big.Int, error)
	// NonceAt returns the account nonce of the given account.
	// The block number can be nil, in which case the nonce is taken from the latest known block.
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
	// PendingNonceAt returns the pending nonce.
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	// EstimateGas returns an estimate of the amount of gas needed to execute the given
	// transaction against the current pending block.
	EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error)
	// Close the underlying eth connection
	Close()
}

// SimpleTxManager is a implementation of TxManager that performs linear fee
// bumping of a tx until it confirms.
type SimpleTxManager struct {
	cfg     Config // embed the config directly
	name    string
	chainID *big.Int

	backend       ETHBackend
	domiconClient dial.RollupProvider
	l             log.Logger
	metr          metrics.TxMetricer

	index     *uint64
	indexLock sync.RWMutex

	pending atomic.Int64

	l1DomiconCommitmentContract *bind.BoundContract
	domiconSdk                  *kzgsdk.DomiconSdk
}

// NewSimpleTxManager initializes a new SimpleTxManager with the passed Config.
func NewSimpleTxManager(name string, l log.Logger, m metrics.TxMetricer, domiconClient dial.RollupProvider, cfg CLIConfig, kzgSrsPath string, commitContractAddr string) (*SimpleTxManager, error) {
	conf, err := NewConfig(cfg, l, commitContractAddr)
	if err != nil {
		return nil, err
	}
	return NewSimpleTxManagerFromConfig(name, l, m, domiconClient, conf, kzgSrsPath)
}

// NewSimpleTxManager initializes a new SimpleTxManager with the passed Config.
func NewSimpleTxManagerFromConfig(name string, l log.Logger, m metrics.TxMetricer, domiconClient dial.RollupProvider, conf Config, kzgSrsPath string) (*SimpleTxManager, error) {
	if err := conf.Check(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client, ok := conf.Backend.(*ethclient.Client)
	if !ok {
		return nil, fmt.Errorf("convert ethclient failed")
	}

	contractAbi, err := abi.JSON(strings.NewReader(domiconabi.L1DomiconCommitment))
	if err != nil {
		return nil, fmt.Errorf("parse L1DomiconCommitment abi failed: %w", err)
	}
	contract := bind.NewBoundContract(conf.L1DomiconCommitmentContractAddr, contractAbi, client, client, nil)

	domSdk, err := kzgsdk.InitDomiconSdk(dSrsSize, kzgSrsPath)
	if err != nil {
		return nil, fmt.Errorf("init domicon sdk failed: %w", err)
	}
	return &SimpleTxManager{
		chainID:                     conf.ChainID,
		name:                        name,
		cfg:                         conf,
		backend:                     conf.Backend,
		domiconClient:               domiconClient,
		l:                           l.New("service", name),
		metr:                        m,
		l1DomiconCommitmentContract: contract,
		domiconSdk:                  domSdk,
	}, nil
}

func (m *SimpleTxManager) From() common.Address {
	return m.cfg.From
}

func (m *SimpleTxManager) BlockNumber(ctx context.Context) (uint64, error) {
	return m.backend.BlockNumber(ctx)
}

func (m *SimpleTxManager) Close() {
	m.backend.Close()
}

// TxCandidate is a transaction candidate that can be submitted to ask the
// [TxManager] to construct a transaction with gas price bounds.
type TxCandidate struct {
	// TxData is the transaction data to be used in the constructed tx.
	TxData []byte
	// To is the recipient of the constructed tx. Nil means contract creation.
	To *common.Address
	// GasLimit is the gas limit to be used in the constructed tx.
	GasLimit uint64
	// Value is the value to be used in the constructed tx.
	Value *big.Int
}

// Send is used to publish a transaction with incrementally higher gas prices
// until the transaction eventually confirms. This method blocks until an
// invocation of sendTx returns (called with differing gas prices). The method
// may be canceled using the passed context.
//
// The transaction manager handles all signing. If and only if the gas limit is 0, the
// transaction manager will do a gas estimation.
//
// NOTE: Send can be called concurrently, the nonce will be managed internally.
func (m *SimpleTxManager) Send(ctx context.Context, candidate TxCandidate) (*types.Receipt, error) {
	m.metr.RecordPendingTx(m.pending.Add(1))
	defer func() {
		m.metr.RecordPendingTx(m.pending.Add(-1))
	}()
	receipt, err := m.send(ctx, candidate)
	if err != nil {
		m.resetNonce()
	}
	return receipt, err
}

// send performs the actual transaction creation and sending.
func (m *SimpleTxManager) send(ctx context.Context, candidate TxCandidate) (*types.Receipt, error) {
	if m.cfg.TxSendTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.cfg.TxSendTimeout)
		defer cancel()
	}
	tx, err := retry.Do(ctx, 30, retry.Fixed(2*time.Second), func() (*CDInfo, error) {
		tx, err := m.craftCD(ctx, candidate)
		if err != nil {
			m.l.Warn("Failed to create a transaction, will retry", "err", err)
		}
		return tx, err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create the tx: %w", err)
	}
	return m.sendTx(ctx, tx)
}

type CDInfo struct {
	Index uint64
	Len   uint64
	To    common.Address
	From  common.Address
	CM    [48]byte
	Sig   []byte
	Data  []byte
}

// craftTx creates the signed transaction
func (m *SimpleTxManager) craftCD(ctx context.Context, candidate TxCandidate) (*CDInfo, error) {
	rawCD := &CDInfo{
		To:   *candidate.To,
		From: m.cfg.From,
		Data: candidate.TxData,
	}
	err := m.loadNextIndex(ctx, m.cfg.From)
	if err != nil {
		return nil, fmt.Errorf("failed to load next index: %w", err)
	}
	rawCD.Index = *m.index
	length := len(candidate.TxData)
	rawCD.Len = uint64(length)
	digest, err := m.domiconSdk.GenerateDataCommit(candidate.TxData)
	if err != nil {
		return nil, fmt.Errorf("failed to generate data commit: %w", err)
	}
	rawCD.CM = digest.Bytes()
	singer := kzgsdk.NewEIP155FdSigner(big.NewInt(11155111))
	sigHash, sigData, err := kzgsdk.SignFd(m.cfg.From, rawCD.To, 0, *m.index, uint64(length), rawCD.CM[:], singer, m.cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign commitment data : %w", err)
	}
	log.Info("craftCD", "sigHash", sigHash)
	addr, err := kzgsdk.FdGetSender(singer, sigData, m.cfg.From, rawCD.To, 0, rawCD.Index, rawCD.Len, rawCD.CM[:])
	if err != nil {
		log.Info("FdGetSender", "err", err, "addr", addr)
	}
	rawCD.Sig = sigData

	m.l.Info("Creating CD", "to", rawCD.To, "from", m.cfg.From)
	return rawCD, nil
}

// signWithNextIndex returns a signed transaction with the next available index.
// The nonce is fetched once using eth_getTransactionCount with "latest", and
// then subsequent calls simply increment this number. If the transaction manager
// is reset, it will query the eth_getTransactionCount nonce again. If signing
// fails, the nonce is not incremented.
func (m *SimpleTxManager) loadNextIndex(ctx context.Context, userAddr common.Address) error {
	m.indexLock.Lock()
	defer m.indexLock.Unlock()

	if m.index == nil {
		results := new([]interface{})
		err := m.l1DomiconCommitmentContract.Call(&bind.CallOpts{}, results, "indices", userAddr)
		if err != nil {
			return fmt.Errorf("call failed: %w", err)
		}
		index := *abi.ConvertType((*results)[0], new(big.Int)).(*big.Int)
		if !index.IsUint64() {
			return fmt.Errorf("index is not uint64: %w", err)
		}
		tmpIndex := index.Uint64()
		m.index = &tmpIndex
	} else {
		*m.index++
	}

	m.metr.RecordNonce(*m.index)
	return nil
}

// resetNonce resets the internal nonce tracking. This is called if any pending send
// returns an error.
func (m *SimpleTxManager) resetNonce() {
	m.indexLock.Lock()
	defer m.indexLock.Unlock()
	m.index = nil
}

// send submits the same transaction several times with increasing gas prices as necessary.
// It waits for the transaction to be confirmed on chain.
func (m *SimpleTxManager) sendTx(ctx context.Context, rawCD *CDInfo) (*types.Receipt, error) {
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sendState := NewSendState(m.cfg.SafeAbortNonceTooLowCount, m.cfg.TxNotInMempoolTimeout)
	receiptChan := make(chan *types.Receipt, 1)
	publishAndWait := func(rawCD *CDInfo, bumpFees bool) *CDInfo {
		wg.Add(1)
		rawCD, hash, published := m.publishTx(ctx, rawCD, sendState)
		if published {
			log.Info("publish success", "hash", hash)
			go func() {
				defer wg.Done()
				m.waitForTx(ctx, *hash, sendState, receiptChan)
			}()
		} else {
			log.Info("publish failed")
			wg.Done()
		}
		return rawCD
	}

	// Immediately publish a transaction before starting the resumbission loop
	rawCD = publishAndWait(rawCD, false)

	ticker := time.NewTicker(m.cfg.ResubmissionTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Don't resubmit a transaction if it has been mined, but we are waiting for the conf depth.
			if sendState.IsWaitingForConfirmation() {
				continue
			}
			// If we see lots of unrecoverable errors (and no pending transactions) abort sending the transaction.
			if sendState.ShouldAbortImmediately() {
				m.l.Warn("Aborting transaction submission")
				return nil, errors.New("aborted transaction sending")
			}
			rawCD = publishAndWait(rawCD, true)

		case <-ctx.Done():
			return nil, ctx.Err()

		case receipt := <-receiptChan:
			m.metr.RecordGasBumpCount(sendState.bumpCount)
			m.metr.TxConfirmed(receipt)
			return receipt, nil
		}
	}
}

// publishTx publishes the transaction to the transaction pool. If it receives any underpriced errors
// it will bump the fees and retry.
// Returns the latest fee bumped tx, and a boolean indicating whether the tx was sent or not
func (m *SimpleTxManager) publishTx(ctx context.Context, rawCD *CDInfo, sendState *SendState) (*CDInfo, *common.Hash, bool) {
	updateLogFields := func(rawCD *CDInfo) log.Logger {
		//return m.l.New("hash", rawCD.Hash(), "nonce", rawCD.Nonce(), "gasTipCap", rawCD.GasTipCap(), "gasFeeCap", rawCD.GasFeeCap())
		return m.l.New("from", rawCD.From)
	}
	l := updateLogFields(rawCD)

	l.Info("Publishing transaction")

	ctxCancel, cancel := context.WithCancel(ctx)
	defer cancel()
	rollupClient, err := m.domiconClient.RollupClient(ctxCancel)
	if err != nil {
		cancel()
		return rawCD, nil, false
	}

	for {
		if sendState.IsWaitingForConfirmation() {
			// there is a chance the previous tx goes into "waiting for confirmation" state
			// during the increaseGasPrice call; continue waiting rather than resubmit the tx
			return rawCD, nil, false
		}

		cCtx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
		hash, err := rollupClient.SendDA(cCtx, rawCD.Index, rawCD.Len, rawCD.To, rawCD.From, hexutil.Bytes(rawCD.CM[:]),
			hexutil.Bytes(rawCD.Sig), hexutil.Bytes(rawCD.Data))

		cancel()
		sendState.ProcessSendError(err)

		if err == nil {
			m.metr.TxPublished("")
			log.Info("Transaction successfully published")
			return rawCD, &hash, true
		}

		switch {
		case errStringMatch(err, core.ErrNonceTooLow):
			l.Warn("nonce too low", "err", err)
			m.metr.TxPublished("nonce_to_low")
		case errStringMatch(err, context.Canceled):
			m.metr.RPCError()
			l.Warn("transaction send cancelled", "err", err)
			m.metr.TxPublished("context_cancelled")
		case errStringMatch(err, txpool.ErrAlreadyKnown):
			l.Warn("resubmitted already known transaction", "err", err)
			m.metr.TxPublished("tx_already_known")
		case errStringMatch(err, txpool.ErrReplaceUnderpriced):
			l.Warn("transaction replacement is underpriced", "err", err)
			m.metr.TxPublished("tx_replacement_underpriced")
			continue // retry with fee bump
		case errStringMatch(err, txpool.ErrUnderpriced):
			l.Warn("transaction is underpriced", "err", err)
			m.metr.TxPublished("tx_underpriced")
			continue // retry with fee bump
		default:
			m.metr.RPCError()
			l.Error("unable to publish transaction", "err", err)
			m.metr.TxPublished("unknown_error")
		}

		// on non-underpriced error return immediately; will retry on next resubmission timeout
		return rawCD, nil, false
	}
}

// waitForTx calls waitMined, and then sends the receipt to receiptChan in a non-blocking way if a receipt is found
// for the transaction. It should be called in a separate goroutine.
func (m *SimpleTxManager) waitForTx(ctx context.Context, hash common.Hash, sendState *SendState, receiptChan chan *types.Receipt) {
	t := time.Now()
	// Poll for the transaction to be ready & then send the result to receiptChan
	receipt, err := m.waitMined(ctx, hash, sendState)
	if err != nil {
		// this will happen if the tx was successfully replaced by a tx with bumped fees
		log.Info("Transaction receipt not found", "err", err)
		return
	}
	select {
	case receiptChan <- receipt:
		m.metr.RecordTxConfirmationLatency(time.Since(t).Milliseconds())
	default:
	}
}

// waitMined waits for the transaction to be mined or for the context to be cancelled.
func (m *SimpleTxManager) waitMined(ctx context.Context, hash common.Hash, sendState *SendState) (*types.Receipt, error) {
	queryTicker := time.NewTicker(m.cfg.ReceiptQueryInterval)
	defer queryTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-queryTicker.C:
			if receipt := m.queryReceipt(ctx, hash, sendState); receipt != nil {
				return receipt, nil
			}
		}
	}
}

// queryReceipt queries for the receipt and returns the receipt if it has passed the confirmation depth
func (m *SimpleTxManager) queryReceipt(ctx context.Context, txHash common.Hash, sendState *SendState) *types.Receipt {
	ctx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
	defer cancel()
	receipt, err := m.backend.TransactionReceipt(ctx, txHash)
	if err != nil {
		log.Info("TransactionReceipt", "err", err)
	}
	if errors.Is(err, ethereum.NotFound) {
		sendState.TxNotMined(txHash)
		m.l.Trace("Transaction not yet mined", "hash", txHash)
		return nil
	} else if err != nil {
		m.metr.RPCError()
		m.l.Info("Receipt retrieval failed", "hash", txHash, "err", err)
		return nil
	} else if receipt == nil {
		m.metr.RPCError()
		m.l.Warn("Receipt and error are both nil", "hash", txHash)
		return nil
	}

	// Receipt is confirmed to be valid from this point on
	log.Info("TxMined")
	sendState.TxMined(txHash)

	txHeight := receipt.BlockNumber.Uint64()
	tipHeight, err := m.backend.BlockNumber(ctx)
	if err != nil {
		m.l.Error("Unable to fetch block number", "err", err)
		return nil
	}

	m.l.Debug("Transaction mined, checking confirmations", "hash", txHash, "txHeight", txHeight,
		"tipHeight", tipHeight, "numConfirmations", m.cfg.NumConfirmations)

	// The transaction is considered confirmed when
	// txHeight+numConfirmations-1 <= tipHeight. Note that the -1 is
	// needed to account for the fact that confirmations have an
	// inherent off-by-one, i.e. when using 1 confirmation the
	// transaction should be confirmed when txHeight is equal to
	// tipHeight. The equation is rewritten in this form to avoid
	// underflows.
	if txHeight+m.cfg.NumConfirmations <= tipHeight+1 {
		m.l.Info("Transaction confirmed", "hash", txHash)
		return receipt
	}

	// Safe to subtract since we know the LHS above is greater.
	confsRemaining := (txHeight + m.cfg.NumConfirmations) - (tipHeight + 1)
	m.l.Debug("Transaction not yet confirmed", "hash", txHash, "confsRemaining", confsRemaining)
	return nil
}

// increaseGasPrice takes the previous transaction, clones it, and returns it with fee values that
// are at least `priceBump` percent higher than the previous ones to satisfy Geth's replacement
// rules, and no lower than the values returned by the fee suggestion algorithm to ensure it
// doesn't linger in the mempool. Finally to avoid runaway price increases, fees are capped at a
// `feeLimitMultiplier` multiple of the suggested values.
func (m *SimpleTxManager) increaseGasPrice(ctx context.Context, tx *types.Transaction) (*types.Transaction, error) {
	m.l.Info("bumping gas price for tx", "hash", tx.Hash(), "gasTipCap", tx.GasTipCap(), "gasFeeCap", tx.GasFeeCap(), "gaslimit", tx.Gas())
	tip, basefee, err := m.suggestGasPriceCaps(ctx)
	if err != nil {
		m.l.Warn("failed to get suggested gas tip and basefee", "err", err)
		return nil, err
	}
	bumpedTip, bumpedFee := updateFees(tx.GasTipCap(), tx.GasFeeCap(), tip, basefee, m.l)

	if err := m.checkLimits(tip, basefee, bumpedTip, bumpedFee); err != nil {
		return nil, err
	}

	rawTx := &types.DynamicFeeTx{
		ChainID:    tx.ChainId(),
		Nonce:      tx.Nonce(),
		GasTipCap:  bumpedTip,
		GasFeeCap:  bumpedFee,
		To:         tx.To(),
		Value:      tx.Value(),
		Data:       tx.Data(),
		AccessList: tx.AccessList(),
	}

	// Re-estimate gaslimit in case things have changed or a previous gaslimit estimate was wrong
	gas, err := m.backend.EstimateGas(ctx, ethereum.CallMsg{
		From:      m.cfg.From,
		To:        rawTx.To,
		GasTipCap: bumpedTip,
		GasFeeCap: bumpedFee,
		Data:      rawTx.Data,
	})
	if err != nil {
		// If this is a transaction resubmission, we sometimes see this outcome because the
		// original tx can get included in a block just before the above call. In this case the
		// error is due to the tx reverting with message "block number must be equal to next
		// expected block number"
		m.l.Warn("failed to re-estimate gas", "err", err, "gaslimit", tx.Gas(),
			"gasFeeCap", bumpedFee, "gasTipCap", bumpedTip)
		return nil, err
	}
	if tx.Gas() != gas {
		m.l.Info("re-estimated gas differs", "oldgas", tx.Gas(), "newgas", gas,
			"gasFeeCap", bumpedFee, "gasTipCap", bumpedTip)
	}
	rawTx.Gas = gas

	ctx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
	defer cancel()
	newTx, err := m.cfg.Signer(ctx, m.cfg.From, types.NewTx(rawTx))
	if err != nil {
		m.l.Warn("failed to sign new transaction", "err", err)
		return tx, nil
	}
	return newTx, nil
}

// suggestGasPriceCaps suggests what the new tip & new basefee should be based on the current L1 conditions
func (m *SimpleTxManager) suggestGasPriceCaps(ctx context.Context) (*big.Int, *big.Int, error) {
	cCtx, cancel := context.WithTimeout(ctx, m.cfg.NetworkTimeout)
	defer cancel()
	tip, err := m.backend.SuggestGasTipCap(cCtx)
	if err != nil {
		m.metr.RPCError()
		return nil, nil, fmt.Errorf("failed to fetch the suggested gas tip cap: %w", err)
	} else if tip == nil {
		return nil, nil, errors.New("the suggested tip was nil")
	}
	cCtx, cancel = context.WithTimeout(ctx, m.cfg.NetworkTimeout)
	defer cancel()
	head, err := m.backend.HeaderByNumber(cCtx, nil)
	if err != nil {
		m.metr.RPCError()
		return nil, nil, fmt.Errorf("failed to fetch the suggested basefee: %w", err)
	} else if head.BaseFee == nil {
		return nil, nil, errors.New("txmgr does not support pre-london blocks that do not have a basefee")
	}
	return tip, head.BaseFee, nil
}

func (m *SimpleTxManager) checkLimits(tip, basefee, bumpedTip, bumpedFee *big.Int) error {
	// If below threshold, don't apply multiplier limit
	if thr := m.cfg.FeeLimitThreshold; thr != nil && thr.Cmp(bumpedFee) == 1 {
		return nil
	}

	// Make sure increase is at most [FeeLimitMultiplier] the suggested values
	feeLimitMult := big.NewInt(int64(m.cfg.FeeLimitMultiplier))
	maxTip := new(big.Int).Mul(tip, feeLimitMult)
	if bumpedTip.Cmp(maxTip) > 0 {
		return fmt.Errorf("bumped tip cap %v is over %dx multiple of the suggested value", bumpedTip, m.cfg.FeeLimitMultiplier)
	}
	maxFee := calcGasFeeCap(new(big.Int).Mul(basefee, feeLimitMult), maxTip)
	if bumpedFee.Cmp(maxFee) > 0 {
		return fmt.Errorf("bumped fee cap %v is over %dx multiple of the suggested value", bumpedFee, m.cfg.FeeLimitMultiplier)
	}
	return nil
}

// calcThresholdValue returns ceil(x * priceBumpPercent / 100)
// It guarantees that x is increased by at least 1
func calcThresholdValue(x *big.Int) *big.Int {
	threshold := new(big.Int).Mul(priceBumpPercent, x)
	threshold.Add(threshold, ninetyNine)
	threshold.Div(threshold, oneHundred)
	return threshold
}

// updateFees takes an old transaction's tip & fee cap plus a new tip & basefee, and returns
// a suggested tip and fee cap such that:
//
//	(a) each satisfies geth's required tx-replacement fee bumps (we use a 10% increase), and
//	(b) gasTipCap is no less than new tip, and
//	(c) gasFeeCap is no less than calcGasFee(newBaseFee, newTip)
func updateFees(oldTip, oldFeeCap, newTip, newBaseFee *big.Int, lgr log.Logger) (*big.Int, *big.Int) {
	newFeeCap := calcGasFeeCap(newBaseFee, newTip)
	lgr = lgr.New("old_gasTipCap", oldTip, "old_gasFeeCap", oldFeeCap,
		"new_gasTipCap", newTip, "new_gasFeeCap", newFeeCap,
		"new_basefee", newBaseFee)
	thresholdTip := calcThresholdValue(oldTip)
	thresholdFeeCap := calcThresholdValue(oldFeeCap)
	if newTip.Cmp(thresholdTip) >= 0 && newFeeCap.Cmp(thresholdFeeCap) >= 0 {
		lgr.Debug("Using new tip and feecap")
		return newTip, newFeeCap
	} else if newTip.Cmp(thresholdTip) >= 0 && newFeeCap.Cmp(thresholdFeeCap) < 0 {
		// Tip has gone up, but basefee is flat or down.
		// TODO(CLI-3714): Do we need to recalculate the FC here?
		lgr.Debug("Using new tip and threshold feecap")
		return newTip, thresholdFeeCap
	} else if newTip.Cmp(thresholdTip) < 0 && newFeeCap.Cmp(thresholdFeeCap) >= 0 {
		// Basefee has gone up, but the tip hasn't. Recalculate the feecap because if the tip went up a lot
		// not enough of the feecap may be dedicated to paying the basefee.
		lgr.Debug("Using threshold tip and recalculated feecap")
		return thresholdTip, calcGasFeeCap(newBaseFee, thresholdTip)

	} else {
		// TODO(CLI-3713): Should we skip the bump in this case?
		lgr.Debug("Using threshold tip and threshold feecap")
		return thresholdTip, thresholdFeeCap
	}
}

// calcGasFeeCap deterministically computes the recommended gas fee cap given
// the base fee and gasTipCap. The resulting gasFeeCap is equal to:
//
//	gasTipCap + 2*baseFee.
func calcGasFeeCap(baseFee, gasTipCap *big.Int) *big.Int {
	return new(big.Int).Add(
		gasTipCap,
		new(big.Int).Mul(baseFee, big.NewInt(2)),
	)
}

// errStringMatch returns true if err.Error() is a substring in target.Error() or if both are nil.
// It can accept nil errors without issue.
func errStringMatch(err, target error) bool {
	if err == nil && target == nil {
		return true
	} else if err == nil || target == nil {
		return false
	}
	return strings.Contains(err.Error(), target.Error())
}

package derive

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type DataIter interface {
	Next(ctx context.Context) (eth.Data, error)
}

type L1TransactionFetcher interface {
	InfoAndTxsByHash(ctx context.Context, hash common.Hash) (eth.BlockInfo, types.Transactions, error)
	FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error)
}

type DomiconDAFetcher interface {
	FileDataByHash(ctx context.Context, hash common.Hash) ([]byte, error)
}

// DataSourceFactory readers raw transactions from a given block & then filters for
// batch submitter transactions.
// This is not a stage in the pipeline, but a wrapper for another stage in the pipeline
type DataSourceFactory struct {
	log              log.Logger
	dsCfg            DataSourceConfig
	fetcher          L1TransactionFetcher
	domiconDAFetcher DomiconDAFetcher
}

func NewDataSourceFactory(log log.Logger, cfg *rollup.Config, fetcher L1TransactionFetcher, daFetcher DomiconDAFetcher) *DataSourceFactory {
	return &DataSourceFactory{log: log, dsCfg: DataSourceConfig{l1Signer: cfg.L1Signer(), batchInboxAddress: cfg.BatchInboxAddress}, fetcher: fetcher, domiconDAFetcher: daFetcher}
}

// OpenData returns a DataIter. This struct implements the `Next` function.
func (ds *DataSourceFactory) OpenData(ctx context.Context, id eth.BlockID, batcherAddr common.Address) DataIter {
	return NewDataSource(ctx, ds.log, ds.dsCfg, ds.fetcher, id, batcherAddr, ds.domiconDAFetcher)
}

// DataSourceConfig regroups the mandatory rollup.Config fields needed for DataFromEVMTransactions.
type DataSourceConfig struct {
	l1Signer          types.Signer
	batchInboxAddress common.Address
}

// DataSource is a fault tolerant approach to fetching data.
// The constructor will never fail & it will instead re-attempt the fetcher
// at a later point.
type DataSource struct {
	// Internal state + data
	open bool
	data []eth.Data
	// Required to re-attempt fetching
	id      eth.BlockID
	dsCfg   DataSourceConfig
	fetcher L1TransactionFetcher
	log     log.Logger

	batcherAddr      common.Address
	domiconDAFetcher DomiconDAFetcher
}

// NewDataSource creates a new calldata source. It suppresses errors in fetching the L1 block if they occur.
// If there is an error, it will attempt to fetch the result on the next call to `Next`.
func NewDataSource(ctx context.Context, log log.Logger, dsCfg DataSourceConfig, fetcher L1TransactionFetcher, block eth.BlockID, batcherAddr common.Address, domiconDAFetcher DomiconDAFetcher) DataIter {
	_, txs, err := fetcher.InfoAndTxsByHash(ctx, block.Hash)
	_, receiptSli, errRecp := fetcher.FetchReceipts(ctx, block.Hash)
	if err != nil || errRecp != nil {
		return &DataSource{
			open:             false,
			id:               block,
			dsCfg:            dsCfg,
			fetcher:          fetcher,
			log:              log,
			batcherAddr:      batcherAddr,
			domiconDAFetcher: domiconDAFetcher,
		}
	} else {
		return &DataSource{
			open: true,
			//data: DataFromEVMTransactions(dsCfg, batcherAddr, txs, log.New("origin", block)),
			data: DataFromDomiconTransactions(dsCfg, batcherAddr, txs, log.New("origin", block), domiconDAFetcher, receiptSli),
		}
	}
}

// Next returns the next piece of data if it has it. If the constructor failed, this
// will attempt to reinitialize itself. If it cannot find the block it returns a ResetError
// otherwise it returns a temporary error if fetching the block returns an error.
func (ds *DataSource) Next(ctx context.Context) (eth.Data, error) {
	if !ds.open {
		_, txs, err := ds.fetcher.InfoAndTxsByHash(ctx, ds.id.Hash)
		_, receiptSli, errRecp := ds.fetcher.FetchReceipts(ctx, ds.id.Hash)
		if err == nil && errRecp == nil {
			ds.open = true
			ds.data = DataFromDomiconTransactions(ds.dsCfg, ds.batcherAddr, txs, log.New("origin", ds.id), ds.domiconDAFetcher, receiptSli)
		} else if errors.Is(err, ethereum.NotFound) {
			return nil, NewResetError(fmt.Errorf("failed to open calldata source: %w", err))
		} else {
			return nil, NewTemporaryError(fmt.Errorf("failed to open calldata source: %w", err))
		}
	}
	if len(ds.data) == 0 {
		return nil, io.EOF
	} else {
		data := ds.data[0]
		ds.data = ds.data[1:]
		return data, nil
	}
}

// DataFromEVMTransactions filters all of the transactions and returns the calldata from transactions
// that are sent to the batch inbox address from the batch sender address.
// This will return an empty array if no valid transactions are found.
func DataFromEVMTransactions(dsCfg DataSourceConfig, batcherAddr common.Address, txs types.Transactions, log log.Logger) []eth.Data {
	var out []eth.Data
	for j, tx := range txs {
		if to := tx.To(); to != nil && *to == dsCfg.batchInboxAddress {
			seqDataSubmitter, err := dsCfg.l1Signer.Sender(tx) // optimization: only derive sender if To is correct
			if err != nil {
				log.Warn("tx in inbox with invalid signature", "index", j, "txHash", tx.Hash(), "err", err)
				continue // bad signature, ignore
			}
			// some random L1 user might have sent a transaction to our batch inbox, ignore them
			if seqDataSubmitter != batcherAddr {
				log.Warn("tx in inbox with unauthorized submitter", "index", j, "txHash", tx.Hash(), "err", err)
				continue // not an authorized batch submitter, ignore
			}
			out = append(out, tx.Data())
		}
	}
	return out
}

func DataFromDomiconTransactions(dsCfg DataSourceConfig, batcherAddr common.Address, txs types.Transactions, log log.Logger, domiconDAFetcher DomiconDAFetcher, receipts types.Receipts) []eth.Data {
	log.Info("hddtest DataFromDomiconTransactions")
	var out []eth.Data
	for j, tx := range txs {
		if to := tx.To(); to != nil && *to == dsCfg.batchInboxAddress {
			// check tx receipt status
			if !CheckTxReceiptStatus(tx.Hash(), receipts) {
				log.Warn("CheckTxReceiptStatus", "tx hash", tx.Hash(), "receipt status", types.ReceiptStatusFailed)
				continue
			}
			// check tx result
			inputData := hexutil.Encode(tx.Data())
			log.Info("DataFromDomiconTransactions", "inputData len", len(inputData))
			if len(inputData) != 842 {
				log.Warn("tx inputData length is incorrect")
				continue
			}
			userAddrTmp := inputData[226:266]
			userAddr := common.HexToAddress("0x" + userAddrTmp)
			log.Info("DataFromDomiconTransactions", "userAddr parsed from tx input is", userAddr, "batcherAddr", batcherAddr)
			// some random L1 user might have sent a transaction to our batch inbox, ignore them
			if userAddr != batcherAddr {
				log.Warn("tx in inbox with unauthorized submitter", "index", j, "txHash", tx.Hash())
				continue // not an authorized batch submitter, ignore
			}

			da, err := domiconDAFetcher.FileDataByHash(context.Background(), tx.Hash())
			if err != nil {
				log.Warn("FileDataByHash failed", "tx hash", tx.Hash(), "error", err)
				continue
			}
			log.Info("DataFromDomiconTransactions", "find DA data with len", len(da))
			out = append(out, da)
		}
	}
	return out
}

func CheckTxReceiptStatus(txHash common.Hash, receipts types.Receipts) bool {
	for _, r := range receipts {
		if r.TxHash == txHash {
			if r.Status == types.ReceiptStatusSuccessful {
				return true
			}
		}
	}
	return false
}

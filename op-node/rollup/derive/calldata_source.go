package derive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	domiconabi "github.com/ethereum-optimism/optimism/packages/domicon-abi"
)

type DataIter interface {
	Next(ctx context.Context) (eth.Data, error)
}

type L1TransactionFetcher interface {
	InfoAndTxsByHash(ctx context.Context, hash common.Hash) (eth.BlockInfo, types.Transactions, error)
}

// DataSourceFactory readers raw transactions from a given block & then filters for
// batch submitter transactions.
// This is not a stage in the pipeline, but a wrapper for another stage in the pipeline
type DataSourceFactory struct {
	log     log.Logger
	dsCfg   DataSourceConfig
	fetcher L1TransactionFetcher
}

func NewDataSourceFactory(log log.Logger, cfg *rollup.Config, fetcher L1TransactionFetcher) *DataSourceFactory {
	return &DataSourceFactory{log: log, dsCfg: DataSourceConfig{l1Signer: cfg.L1Signer(), batchInboxAddress: cfg.BatchInboxAddress}, fetcher: fetcher}
}

// OpenData returns a DataIter. This struct implements the `Next` function.
func (ds *DataSourceFactory) OpenData(ctx context.Context, id eth.BlockID, batcherAddr common.Address) DataIter {
	return NewDataSource(ctx, ds.log, ds.dsCfg, ds.fetcher, id, batcherAddr)
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

	batcherAddr common.Address

	domiconNodesRpc []string
}

// NewDataSource creates a new calldata source. It suppresses errors in fetching the L1 block if they occur.
// If there is an error, it will attempt to fetch the result on the next call to `Next`.
func NewDataSource(ctx context.Context, log log.Logger, dsCfg DataSourceConfig, fetcher L1TransactionFetcher, block eth.BlockID, batcherAddr common.Address) DataIter {
	_, txs, err := fetcher.InfoAndTxsByHash(ctx, block.Hash)
	l1EthRpc := ""
	domiconNodesRpc, _ := getDomiconNodesRpc(ctx, common.HexToAddress("0xl1domiconNodesContractAddr"), log, l1EthRpc)
	if err != nil || len(domiconNodesRpc) == 0 {
		return &DataSource{
			open:            false,
			id:              block,
			dsCfg:           dsCfg,
			fetcher:         fetcher,
			log:             log,
			batcherAddr:     batcherAddr,
			domiconNodesRpc: []string{},
		}
	} else {
		return &DataSource{
			open: true,
			data: DataFromEVMTransactions(dsCfg, batcherAddr, txs, log.New("origin", block)),
		}
	}
}

// Next returns the next piece of data if it has it. If the constructor failed, this
// will attempt to reinitialize itself. If it cannot find the block it returns a ResetError
// otherwise it returns a temporary error if fetching the block returns an error.
func (ds *DataSource) Next(ctx context.Context) (eth.Data, error) {
	if !ds.open {
		if _, txs, err := ds.fetcher.InfoAndTxsByHash(ctx, ds.id.Hash); err == nil {
			ds.open = true
			ds.data = DataFromEVMTransactions(ds.dsCfg, ds.batcherAddr, txs, log.New("origin", ds.id))
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

func DataFromDomiconTransactions(dsCfg DataSourceConfig, batcherAddr common.Address, txs types.Transactions, log log.Logger, domiconNodesRpc []string) []eth.Data {
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
			//da := getDAData(domiconNodesRpc)
			out = append(out, tx.Data())
		}
	}
	return out
}

func getDomiconNodesRpc(ctx context.Context, l1DomiconNodesContractAddr common.Address, log log.Logger, l1EthRpc string) (map[string]common.Address, error) {
	nodesAddrRpc := make(map[string]common.Address)
	l1Client, err := dial.DialEthClientWithTimeout(ctx, dial.DefaultDialTimeout, log, l1EthRpc)
	if err != nil {
		return nodesAddrRpc, fmt.Errorf("failed to dial L1 RPC: %w", err)
	}
	domiconNodesAbi, err := abi.JSON(strings.NewReader(domiconabi.DomiconNodes))
	if err != nil {
		return nodesAddrRpc, fmt.Errorf("parse DomiconNodes abi failed: %w", err)
	}
	l1DomiconNodesContract := bind.NewBoundContract(l1DomiconNodesContractAddr, domiconNodesAbi, l1Client, l1Client, nil)

	bcNodeAddrs := new([]interface{})
	err = l1DomiconNodesContract.Call(&bind.CallOpts{}, bcNodeAddrs, "BROADCAST_NODES")
	if err != nil {
		return nodesAddrRpc, err
	}
	log.Info("selectBestNode", "addresses:", (*bcNodeAddrs)[0])
	addrSli, ok := (*bcNodeAddrs)[0].([]common.Address)
	if !ok {
		return nodesAddrRpc, errors.New("broadcast node address error format")
	}

	log.Info("msg", "addrSli", addrSli)
	for i, addr := range addrSli {
		if i > 20 {
			break
		}
		log.Info("msg", "addr", addr)
		bcNodeInfo := new([]interface{})
		l1DomiconNodesContract.Call(&bind.CallOpts{}, bcNodeInfo, "broadcastingNodes", addr)
		nodeAddr, _ := (*bcNodeInfo)[0].(common.Address)
		nodeRpc, _ := (*bcNodeInfo)[1].(string)
		log.Info("bcNodeInfo", "nodeAddr", nodeAddr, "nodeRpc", nodeRpc)
		nodesAddrRpc[nodeRpc] = nodeAddr
	}

	return nodesAddrRpc, nil
}
func getDAData(ctx context.Context, log log.Logger, nodesAddrRpc map[string]common.Address, hash common.Hash) []byte {
	//todo
	for rpcUrl, _ := range nodesAddrRpc {
		domiconClient, err := dial.NewStaticL2RollupProvider(ctx, log, rpcUrl)
		if err != nil {
			continue
		}
		client, err := domiconClient.RollupClient(ctx)
		if err != nil {
			continue
		}
		da, err := client.FileDataByHash(ctx, hash)
		if err == nil {
			return da
		}
	}

	return []byte{}
}

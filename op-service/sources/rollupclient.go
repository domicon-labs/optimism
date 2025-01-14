package sources

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type RollupClient struct {
	rpc client.RPC
}

type RPCFileData struct {
	Sender     common.Address `json:"sender"`
	Submmiter  common.Address `json:"submmiter"`
	Length     hexutil.Uint64 `json:"length"`
	Index      hexutil.Uint64 `json:"index"`
	Commitment hexutil.Bytes  `json:"commitment"`
	Data       hexutil.Bytes  `json:"data"`
	Sign       hexutil.Bytes  `json:"sign"`
	TxHash     common.Hash    `json:"txhash"`
}

func NewRollupClient(rpc client.RPC) *RollupClient {
	return &RollupClient{rpc}
}

func (r *RollupClient) OutputAtBlock(ctx context.Context, blockNum uint64) (*eth.OutputResponse, error) {
	var output *eth.OutputResponse
	err := r.rpc.CallContext(ctx, &output, "optimism_outputAtBlock", hexutil.Uint64(blockNum))
	return output, err
}

func (r *RollupClient) SyncStatus(ctx context.Context) (*eth.SyncStatus, error) {
	var output *eth.SyncStatus
	err := r.rpc.CallContext(ctx, &output, "optimism_syncStatus")
	return output, err
}

func (r *RollupClient) RollupConfig(ctx context.Context) (*rollup.Config, error) {
	var output *rollup.Config
	err := r.rpc.CallContext(ctx, &output, "optimism_rollupConfig")
	return output, err
}

func (r *RollupClient) Version(ctx context.Context) (string, error) {
	var output string
	err := r.rpc.CallContext(ctx, &output, "optimism_version")
	return output, err
}

func (r *RollupClient) SendDA(ctx context.Context, index, length uint64, broadcaster, user common.Address, commitment, sign, data hexutil.Bytes) (common.Hash, error) {
	log.Info("msg sendDA", "index", index)
	log.Info("msg sendDA", "length", length)
	log.Info("msg sendDA", "broadcaster", broadcaster)
	log.Info("msg sendDA", "user", user)
	log.Info("msg sendDA", "commitment", commitment)
	log.Info("msg sendDA", "sign", sign)
	//log.Info("msg sendDA", "data", data)
	var result common.Hash
	err := r.rpc.CallContext(ctx, &result, "optimism_sendDA", index, length, 0, broadcaster, user, commitment, sign, data)
	if err == nil {
		log.Info("msg sendDA", "call optimism_sendDA success hash", result.Hex())
	} else {
		log.Info("msg sendDA", "call optimism_sendDA faild with error:", err, "hash", result.Hex())
	}
	return result, err
}

func (r *RollupClient) FileDataByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	log.Info("FileDataByHash", "hash", hash)
	var rpcFileData RPCFileData
	err := r.rpc.CallContext(ctx, &rpcFileData, "optimism_fileDataByHash", hash)
	return rpcFileData.Data, err
}

func (r *RollupClient) StartSequencer(ctx context.Context, unsafeHead common.Hash) error {
	return r.rpc.CallContext(ctx, nil, "admin_startSequencer", unsafeHead)
}

func (r *RollupClient) StopSequencer(ctx context.Context) (common.Hash, error) {
	var result common.Hash
	err := r.rpc.CallContext(ctx, &result, "admin_stopSequencer")
	return result, err
}

func (r *RollupClient) SequencerActive(ctx context.Context) (bool, error) {
	var result bool
	err := r.rpc.CallContext(ctx, &result, "admin_sequencerActive")
	return result, err
}

func (r *RollupClient) SetLogLevel(ctx context.Context, lvl log.Lvl) error {
	return r.rpc.CallContext(ctx, nil, "admin_setLogLevel", lvl.String())
}

func (r *RollupClient) Close() {
	r.rpc.Close()
}

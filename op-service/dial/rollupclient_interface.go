package dial

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// RollupClientInterface is an interface for providing a RollupClient
// It does not describe all of the functions a RollupClient has, only the ones used by the L2 Providers and their callers
type RollupClientInterface interface {
	OutputAtBlock(ctx context.Context, blockNum uint64) (*eth.OutputResponse, error)
	SyncStatus(ctx context.Context) (*eth.SyncStatus, error)
	RollupConfig(ctx context.Context) (*rollup.Config, error)
	SendDA(ctx context.Context, index, length uint64, broadcaster, user common.Address, commitment, sign, data hexutil.Bytes) (common.Hash, error)
	StartSequencer(ctx context.Context, unsafeHead common.Hash) error
	SequencerActive(ctx context.Context) (bool, error)
	FileDataByHash(ctx context.Context, hash common.Hash) ([]byte, error)
	Close()
}

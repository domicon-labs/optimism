package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-service/dial"
	domiconabi "github.com/ethereum-optimism/optimism/packages/domicon-abi"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type DaSource struct {
	workingNode     *dial.StaticL2RollupProvider
	domiconNodesRpc map[string]common.Address
	log             log.Logger
}

func NewDaSource(ctx context.Context, log log.Logger, cfg *DaSourceConfig) (*DaSource, error) {
	nodesAddrRpc := make(map[string]common.Address)
	l1Client, err := dial.DialEthClientWithTimeout(ctx, dial.DefaultDialTimeout, log, cfg.L1URL)
	if err != nil {
		return &DaSource{}, fmt.Errorf("dial L1 failed", cfg.L1URL, err)
	}
	domiconNodesAbi, err := abi.JSON(strings.NewReader(domiconabi.DomiconNodes))
	if err != nil {
		return &DaSource{}, err
	}

	l1DomiconNodesContract := bind.NewBoundContract(common.HexToAddress(cfg.L1DomiconNodesContract), domiconNodesAbi, l1Client, l1Client, nil)

	bcNodeAddrs := new([]interface{})
	err = l1DomiconNodesContract.Call(&bind.CallOpts{}, bcNodeAddrs, "BROADCAST_NODES")
	if err != nil {
		return &DaSource{}, fmt.Errorf("call L1DomiconNodesContract BROADCAST_NODES failed", err)
	}
	log.Info("selectBestNode", "addresses:", (*bcNodeAddrs)[0])
	addrSli, ok := (*bcNodeAddrs)[0].([]common.Address)
	if !ok {
		return &DaSource{}, fmt.Errorf("convert node address failed")
	}
	if len(addrSli) == 0 {
		return &DaSource{}, fmt.Errorf("There is no broadcast node")
	}
	log.Info("msg", "addrSli", addrSli)
	var firstNodeRpc string = ""
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
		if i == 0 {
			firstNodeRpc = nodeRpc
			continue
		}
		nodesAddrRpc[nodeRpc] = nodeAddr
	}

	domiconClient, err := dial.NewStaticL2RollupProvider(ctx, log, firstNodeRpc)
	if err != nil {
		return &DaSource{}, nil
	}

	return &DaSource{
		workingNode:     domiconClient,
		domiconNodesRpc: nodesAddrRpc,
		log:             log,
	}, nil
}

func (d *DaSource) TryNextNode(ctx context.Context, rpc string) (*dial.StaticL2RollupProvider, error) {
	return dial.NewStaticL2RollupProvider(ctx, d.log, rpc)
}

func (d *DaSource) FileDataByHash(ctx context.Context, hash common.Hash) ([]byte, error) {
	log.Info("hddtest FileDataByHash", "hash", hash)
	if d.workingNode != nil {
		domiconClient, _ := d.workingNode.RollupClient(ctx)
		da, err := domiconClient.FileDataByHash(ctx, hash)
		if err == nil {
			return da, nil
		} else {
			log.Warn("FileDataByHash failed", "error", err)
		}
	}
	for rpc := range d.domiconNodesRpc {
		log.Info("FileDataByHash", "try rpc", rpc)
		nextNode, err := d.TryNextNode(ctx, rpc)
		if err != nil {
			continue
		}
		d.workingNode = nextNode
		domiconClient, _ := d.workingNode.RollupClient(ctx)
		da, err := domiconClient.FileDataByHash(ctx, hash)
		if err == nil {
			return da, nil
		} else {
			log.Warn("FileDataByHash failed again", "error", err, "rpc", rpc)
		}
	}
	return []byte{}, nil
}

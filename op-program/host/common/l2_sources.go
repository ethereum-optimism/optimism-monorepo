package common

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrNoSources               = errors.New("no sources specified")
	ErrNoL2ForRollup           = errors.New("no L2 RPC available for rollup")
	ErrNoRollupForL2           = errors.New("no rollup config available for L2 RPC")
	ErrNoRollupForExperimental = errors.New("no rollup config available for L2 experimental RPC")
)

type L2Sources struct {
	Sources map[uint64]*L2Source
}

func NewL2SourcesFromURLs(ctx context.Context, logger log.Logger, configs []*rollup.Config, l2URLs []string, l2ExperimentalURLs []string) (*L2Sources, error) {
	l2Clients, err := connectRPCs(ctx, logger, l2URLs)
	if err != nil {
		return nil, err
	}
	l2ExperimentalClients, err := connectRPCs(ctx, logger, l2URLs)
	if err != nil {
		return nil, err
	}
	return NewL2Sources(ctx, logger, configs, l2Clients, l2ExperimentalClients)
}

func connectRPCs(ctx context.Context, logger log.Logger, urls []string) ([]client.RPC, error) {
	l2Clients := make([]client.RPC, len(urls))
	for i, url := range urls {
		logger.Info("Connecting to L2 source", "url", url)
		// eth_getProof calls are expensive and takes time, so we use a longer timeout
		rpc, err := client.NewRPC(ctx, logger, url, client.WithDialAttempts(10), client.WithCallTimeout(5*time.Minute))
		if err != nil {
			return nil, fmt.Errorf("failed to connect to rpc URL %s: %w", url, err)
		}
		l2Clients[i] = rpc
	}
	return l2Clients, nil
}

func NewL2Sources(ctx context.Context, logger log.Logger, configs []*rollup.Config, l2Clients []client.RPC, l2ExperimentalClients []client.RPC) (*L2Sources, error) {
	if len(configs) == 0 {
		return nil, ErrNoSources
	}
	rollupConfigs := make(map[uint64]*rollup.Config)
	for _, rollupCfg := range configs {
		rollupConfigs[rollupCfg.L2ChainID.Uint64()] = rollupCfg
	}
	l2RPCs := make(map[uint64]client.RPC)
	for _, rpc := range l2Clients {
		chainID, err := loadChainID(ctx, rpc)
		if err != nil {
			return nil, fmt.Errorf("failed to load chain ID: %w", err)
		}
		l2RPCs[chainID] = rpc
		if _, ok := rollupConfigs[chainID]; !ok {
			return nil, fmt.Errorf("%w: %v", ErrNoRollupForL2, chainID)
		}
	}

	l2ExperimentalRPCs := make(map[uint64]client.RPC)
	for _, rpc := range l2ExperimentalClients {
		chainID, err := loadChainID(ctx, rpc)
		if err != nil {
			return nil, fmt.Errorf("failed to load chain ID: %w", err)
		}
		l2ExperimentalRPCs[chainID] = rpc
		if _, ok := rollupConfigs[chainID]; !ok {
			return nil, fmt.Errorf("%w: %v", ErrNoRollupForExperimental, chainID)
		}
	}

	sources := make(map[uint64]*L2Source)
	for _, rollupCfg := range rollupConfigs {
		chainID := rollupCfg.L2ChainID.Uint64()
		l2RPC, ok := l2RPCs[chainID]
		if !ok {
			return nil, fmt.Errorf("%w: %v", ErrNoL2ForRollup, chainID)
		}
		l2ExperimentalRPC := l2ExperimentalRPCs[chainID] // Allowed to be nil
		source, err := NewL2SourceFromRPC(logger, rollupCfg, l2RPC, l2ExperimentalRPC)
		if err != nil {
			return nil, fmt.Errorf("failed to create l2 source for chain ID %v: %w", chainID, err)
		}
		sources[chainID] = source
	}

	return &L2Sources{
		Sources: sources,
	}, nil
}

func loadChainID(ctx context.Context, rpc client.RPC) (uint64, error) {
	var id hexutil.Big
	err := rpc.CallContext(ctx, &id, "eth_chainId")
	if err != nil {
		return 0, err
	}
	return (*big.Int)(&id).Uint64(), nil
}

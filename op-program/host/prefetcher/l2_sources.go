package prefetcher

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrNoSources                = errors.New("no sources specified")
	ErrNoL2ForRollup            = errors.New("no L2 RPC available for rollup")
	ErrNoRollupForL2            = errors.New("no rollup config available for L2 RPC")
	ErrDuplicateL2URLs          = errors.New("multiple L2 URLs provided for chain")
	ErrNoRollupForExperimental  = errors.New("no rollup config available for L2 experimental RPC")
	ErrDuplicateExperimentsURLs = errors.New("multiple experimental URLs provided for chain")
)

type RetryingL2Sources struct {
	Sources map[eth.ChainID]*RetryingL2Source
}

func NewRetryingL2SourcesFromURLs(ctx context.Context, logger log.Logger, configs []*rollup.Config, l2URLs []string, l2ExperimentalURLs []string) (*RetryingL2Sources, error) {
	l2Clients, err := connectRPCs(ctx, logger, l2URLs)
	if err != nil {
		return nil, err
	}
	l2ExperimentalClients, err := connectRPCs(ctx, logger, l2ExperimentalURLs)
	if err != nil {
		return nil, err
	}
	return NewRetryingL2Sources(ctx, logger, configs, l2Clients, l2ExperimentalClients)
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

func NewRetryingL2Sources(ctx context.Context, logger log.Logger, configs []*rollup.Config, l2Clients []client.RPC, l2ExperimentalClients []client.RPC) (*RetryingL2Sources, error) {
	if len(configs) == 0 {
		return nil, ErrNoSources
	}
	rollupConfigs := make(map[eth.ChainID]*rollup.Config)
	for _, rollupCfg := range configs {
		rollupConfigs[eth.ChainIDFromBig(rollupCfg.L2ChainID)] = rollupCfg
	}
	l2RPCs := make(map[eth.ChainID]client.RPC)
	for _, rpc := range l2Clients {
		chainID, err := loadChainID(ctx, rpc)
		if err != nil {
			return nil, fmt.Errorf("failed to load chain ID: %w", err)
		}
		if _, ok := l2RPCs[chainID]; ok {
			return nil, fmt.Errorf("%w %v", ErrDuplicateL2URLs, chainID)
		}
		l2RPCs[chainID] = rpc
		if _, ok := rollupConfigs[chainID]; !ok {
			return nil, fmt.Errorf("%w: %v", ErrNoRollupForL2, chainID)
		}
	}

	l2ExperimentalRPCs := make(map[eth.ChainID]client.RPC)
	for _, rpc := range l2ExperimentalClients {
		chainID, err := loadChainID(ctx, rpc)
		if err != nil {
			return nil, fmt.Errorf("failed to load chain ID: %w", err)
		}
		if _, ok := l2ExperimentalRPCs[chainID]; ok {
			return nil, fmt.Errorf("%w %v", ErrDuplicateExperimentsURLs, chainID)
		}
		l2ExperimentalRPCs[chainID] = rpc
		if _, ok := rollupConfigs[chainID]; !ok {
			return nil, fmt.Errorf("%w: %v", ErrNoRollupForExperimental, chainID)
		}
	}

	sources := make(map[eth.ChainID]*RetryingL2Source, len(configs))
	for _, rollupCfg := range rollupConfigs {
		chainID := eth.ChainIDFromBig(rollupCfg.L2ChainID)
		l2RPC, ok := l2RPCs[chainID]
		if !ok {
			return nil, fmt.Errorf("%w: %v", ErrNoL2ForRollup, chainID)
		}
		l2ExperimentalRPC := l2ExperimentalRPCs[chainID] // Allowed to be nil
		source, err := common.NewL2SourceFromRPC(logger, rollupCfg, l2RPC, l2ExperimentalRPC)
		if err != nil {
			return nil, fmt.Errorf("failed to create l2 source for chain ID %v: %w", chainID, err)
		}
		sources[chainID] = NewRetryingL2Source(logger, source)
	}

	return &RetryingL2Sources{
		Sources: sources,
	}, nil
}

func (s *RetryingL2Sources) ForChainID(chainID eth.ChainID) (types.L2Source, error) {
	source, ok := s.Sources[chainID]
	if !ok {
		return nil, fmt.Errorf("no source available for chain ID: %v", chainID)
	}
	return source, nil
}

func (s *RetryingL2Sources) ForChainIDWithoutRetries(chainID eth.ChainID) (types.L2Source, error) {
	retrying, ok := s.Sources[chainID]
	if !ok {
		return nil, fmt.Errorf("no source available for chain ID: %v", chainID)
	}
	return retrying.source, nil
}

func loadChainID(ctx context.Context, rpc client.RPC) (eth.ChainID, error) {
	var id hexutil.Big
	err := rpc.CallContext(ctx, &id, "eth_chainId")
	if err != nil {
		return eth.ChainID{}, err
	}
	return eth.ChainIDFromBig((*big.Int)(&id)), nil
}

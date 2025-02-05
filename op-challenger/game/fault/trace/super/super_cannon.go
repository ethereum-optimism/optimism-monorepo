package super

import (
	"context"
	"fmt"
	"math/big"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/cannon"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/split"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/metrics"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

func NewSuperCannonTraceAccessor(
	logger log.Logger,
	m metrics.Metricer,
	cfg vm.Config,
	serverExecutor vm.OracleServerExecutor,
	prestateProvider PreimagePrestateProvider,
	rootProvider RootProvider,
	cannonPrestate string,
	dir string,
	l1Head eth.BlockID,
	splitDepth types.Depth,
	prestateBlock uint64,
	poststateBlock uint64,
) (*trace.Accessor, error) {
	rollupCfgs, err := NewRollupConfigs(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load rollup configs: %w", err)
	}
	outputProvider := NewSuperTraceProvider(logger, rollupCfgs, prestateProvider, rootProvider, l1Head, splitDepth, prestateBlock, poststateBlock)
	cannonCreator := func(ctx context.Context, localContext common.Hash, depth types.Depth, claimInfo ClaimInfo) (types.TraceProvider, error) {
		logger := logger.New("agreedPrestate", claimInfo.AgreedPrestate, "claim", claimInfo.Claim, "localContext", localContext)
		subdir := filepath.Join(dir, localContext.Hex())
		localInputs := utils.LocalGameInputs{
			L1Head:         l1Head.Hash,
			L2OutputRoot:   crypto.Keccak256Hash(claimInfo.AgreedPrestate),
			AgreedPreState: claimInfo.AgreedPrestate,
			L2Claim:        claimInfo.Claim,
			L2BlockNumber:  new(big.Int).SetUint64(poststateBlock),
		}
		provider := cannon.NewTraceProvider(logger, m.ToTypedVmMetrics(cfg.VmType.String()), cfg, serverExecutor, prestateProvider, cannonPrestate, localInputs, subdir, depth)
		return provider, nil
	}

	cache := NewProviderCache(m, "super_cannon_provider", cannonCreator)
	selector := split.NewSplitProviderSelector(outputProvider, splitDepth, SuperRootSplitAdapter(outputProvider, cache.GetOrCreate))
	return trace.NewAccessor(selector), nil
}

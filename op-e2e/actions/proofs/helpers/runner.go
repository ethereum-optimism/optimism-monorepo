package helpers

import (
	"context"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/fakebeacon"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/host"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-program/host/prefetcher"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type L1 interface {
}

type L2 interface {
	RollupClient() *sources.RollupClient
}

func WithPreInteropDefaults(t helpers.Testing, l2ClaimBlockNum uint64, l2 *helpers.L2Verifier, l2Eng *helpers.L2Engine) FixtureInputParam {
	return func(f *FixtureInputs) {
		// Fetch the pre and post output roots for the fault proof.
		l2PreBlockNum := l2ClaimBlockNum - 1
		if l2ClaimBlockNum == 0 {
			// If we are at genesis, we assert that we don't move the chain at all.
			l2PreBlockNum = 0
		}
		rollupClient := l2.RollupClient()
		preRoot, err := rollupClient.OutputAtBlock(t.Ctx(), l2PreBlockNum)
		require.NoError(t, err)
		claimRoot, err := rollupClient.OutputAtBlock(t.Ctx(), l2ClaimBlockNum)
		require.NoError(t, err)

		f.L2BlockNumber = l2ClaimBlockNum
		f.L2Claim = common.Hash(claimRoot.OutputRoot)
		f.L2Head = preRoot.BlockRef.Hash
		f.L2OutputRoot = common.Hash(preRoot.OutputRoot)
		f.L2ChainID = eth.ChainIDFromBig(l2.RollupCfg.L2ChainID)

		f.L2Sources = []*FaultProofProgramL2Source{
			{
				Node:        l2,
				Engine:      l2Eng,
				ChainConfig: l2Eng.L2Chain().Config(),
			},
		}
	}
}

func RunFaultProofProgram(t helpers.Testing, logger log.Logger, l1 *helpers.L1Miner, checkResult CheckResult, fixtureInputParams ...FixtureInputParam) {
	l1Head := l1.L1Chain().CurrentBlock()

	fixtureInputs := &FixtureInputs{
		L1Head: l1Head.Hash(),
	}
	for _, apply := range fixtureInputParams {
		apply(fixtureInputs)
	}
	require.Greater(t, len(fixtureInputs.L2Sources), 0, "Must specify at least one L2 source")

	// Run the fault proof program from the state transition from L2 block l2ClaimBlockNum - 1 -> l2ClaimBlockNum.
	workDir := t.TempDir()
	var err error
	if IsKonaConfigured() {
		fakeBeacon := fakebeacon.NewBeacon(
			logger,
			l1.BlobStore(),
			l1.L1Chain().Genesis().Time(),
			12,
		)
		require.NoError(t, fakeBeacon.Start("127.0.0.1:0"))
		defer fakeBeacon.Close()

		rollupCfgs := make([]*rollup.Config, 0, len(fixtureInputs.L2Sources))
		l2Endpoints := make([]string, 0, len(fixtureInputs.L2Sources))
		for _, source := range fixtureInputs.L2Sources {
			rollupCfgs = append(rollupCfgs, source.Node.RollupCfg)
			l2Endpoints = append(l2Endpoints, source.Engine.HTTPEndpoint())
		}

		err = RunKonaNative(t, workDir, rollupCfgs, l1.HTTPEndpoint(), fakeBeacon.BeaconAddr(), l2Endpoints, *fixtureInputs)
		checkResult(t, err)
	} else {
		programCfg := NewOpProgramCfg(fixtureInputs)
		withInProcessPrefetcher := hostcommon.WithPrefetcher(func(ctx context.Context, logger log.Logger, kv kvstore.KV, cfg *config.Config) (hostcommon.Prefetcher, error) {
			return CreateInprocessPrefetcher(t, ctx, logger, l1, kv, cfg, fixtureInputs)
		})
		ctx, cancel := context.WithTimeout(t.Ctx(), 2*time.Minute)
		defer cancel()
		err = hostcommon.FaultProofProgram(ctx, logger, programCfg, withInProcessPrefetcher)
		checkResult(t, err)
	}
}

func CreateInprocessPrefetcher(
	t helpers.Testing,
	ctx context.Context,
	logger log.Logger,
	l1 *helpers.L1Miner,
	kv kvstore.KV,
	cfg *config.Config,
	fixtureInputs *FixtureInputs,
) (hostcommon.Prefetcher, error) {
	// Set up in-process L1 sources
	l1Cl := l1.L1ClientSimple(t)
	l1BlobFetcher := l1.BlobSource()

	// Set up in-process L2 source
	var rpcClients []client.RPC
	for _, source := range fixtureInputs.L2Sources {
		rpcClients = append(rpcClients, source.Engine.RPCClient())
	}
	sources, err := prefetcher.NewRetryingL2Sources(ctx, logger, cfg.Rollups, rpcClients, nil)
	require.NoError(t, err, "failed to create L2 client")

	executor := host.MakeProgramExecutor(logger, cfg)
	return prefetcher.NewPrefetcher(logger, l1Cl, l1BlobFetcher, fixtureInputs.L2ChainID, sources, kv, executor, cfg.L2Head, cfg.AgreedPrestate), nil
}

package helpers

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/fakebeacon"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/host"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-program/host/prefetcher"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type L1 interface {
}

type L2 interface {
	RollupClient() *sources.RollupClient
}

func RunFaultProofProgram(t helpers.Testing, logger log.Logger, l1 *helpers.L1Miner, l2 *helpers.L2Verifier, l2Eng *helpers.L2Engine, l2ChainConfig *core.Genesis, l2ClaimBlockNum uint64, checkResult CheckResult, fixtureInputParams ...FixtureInputParam) {
	// Fetch the pre and post output roots for the fault proof.
	l2PreBlockNum := l2ClaimBlockNum - 1
	if l2ClaimBlockNum == 0 {
		// If we are at genesis, we assert that we don't move the chain at all.
		l2PreBlockNum = 0
	}
	preRoot, err := l2.RollupClient().OutputAtBlock(t.Ctx(), l2PreBlockNum)
	require.NoError(t, err)
	claimRoot, err := l2.RollupClient().OutputAtBlock(t.Ctx(), l2ClaimBlockNum)
	require.NoError(t, err)
	l1Head := l1.L1Chain().CurrentBlock()

	fixtureInputs := &FixtureInputs{
		L2BlockNumber: l2ClaimBlockNum,
		L2Claim:       common.Hash(claimRoot.OutputRoot),
		L2Head:        preRoot.BlockRef.Hash,
		L2OutputRoot:  common.Hash(preRoot.OutputRoot),
		L2ChainID:     l2.RollupCfg.L2ChainID.Uint64(),
		L1Head:        l1Head.Hash(),
	}
	for _, apply := range fixtureInputParams {
		apply(fixtureInputs)
	}

	// Run the fault proof program from the state transition from L2 block l2ClaimBlockNum - 1 -> l2ClaimBlockNum.
	workDir := t.TempDir()
	if IsKonaConfigured() {
		fakeBeacon := fakebeacon.NewBeacon(
			logger,
			l1.BlobStore(),
			l1.L1Chain().Genesis().Time(),
			12,
		)
		require.NoError(t, fakeBeacon.Start("127.0.0.1:0"))
		defer fakeBeacon.Close()

		err := RunKonaNative(t, workDir, l2.RollupCfg, l1.HTTPEndpoint(), fakeBeacon.BeaconAddr(), l2Eng.HTTPEndpoint(), *fixtureInputs)
		checkResult(t, err)
	} else {
		programCfg := NewOpProgramCfg(
			t,
			l2.RollupCfg,
			l2ChainConfig.Config,
			fixtureInputs,
		)
		withInProcessPrefetcher := hostcommon.WithPrefetcher(func(ctx context.Context, logger log.Logger, kv kvstore.KV, cfg *config.Config) (hostcommon.Prefetcher, error) {
			// Set up in-process L1 sources
			l1Cl := l1.L1Client(t, l2.RollupCfg)
			l1BlobFetcher := l1.BlobSource()

			// Set up in-process L2 source
			sources, err := prefetcher.NewRetryingL2Sources(ctx, logger, []*rollup.Config{l2.RollupCfg}, []client.RPC{l2Eng.RPCClient()}, nil)
			require.NoError(t, err, "failed to create L2 client")

			executor := host.MakeProgramExecutor(logger, programCfg)
			return prefetcher.NewPrefetcher(logger, l1Cl, l1BlobFetcher, l2.RollupCfg.L2ChainID.Uint64(), sources, kv, executor, cfg.L2Head, cfg.AgreedPrestate), nil
		})
		err = hostcommon.FaultProofProgram(t.Ctx(), logger, programCfg, withInProcessPrefetcher)
		checkResult(t, err)
	}
	tryDumpTestFixture(t, err, t.Name(), l2.RollupCfg, l2ChainConfig, *fixtureInputs, workDir)
}

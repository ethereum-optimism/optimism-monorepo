package proofs

import (
	"context"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestL2OutputSubmitterFaultProofs(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t, e2esys.WithAllocType(config.AllocTypeStandard))
	cfg.NonFinalizedProposals = true // speed up the time till we see output proposals

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	l1Client := sys.NodeClient("l1")

	rollupClient := sys.RollupClient("sequencer")

	disputeGameFactory, err := bindings.NewDisputeGameFactoryCaller(cfg.L1Deployments.DisputeGameFactoryProxy, l1Client)
	require.Nil(t, err)

	initialGameCount, err := disputeGameFactory.GameCount(&bind.CallOpts{})
	require.Nil(t, err)

	l2Verif := sys.NodeClient("verifier")
	_, err = geth.WaitForBlock(big.NewInt(6), l2Verif)
	require.Nil(t, err)

	timeoutCh := time.After(15 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		latestGameCount, err := disputeGameFactory.GameCount(&bind.CallOpts{})
		require.Nil(t, err)

		if latestGameCount.Cmp(initialGameCount) > 0 {
			caller := batching.NewMultiCaller(l1Client.Client(), batching.DefaultBatchSize)
			committedL2Output, err := disputeGameFactory.GameAtIndex(&bind.CallOpts{}, new(big.Int).Sub(latestGameCount, common.Big1))
			require.Nil(t, err)
			proxy, err := contracts.NewFaultDisputeGameContract(context.Background(), metrics.NoopContractMetrics, committedL2Output.Proxy, caller)
			require.Nil(t, err)
			claim, err := proxy.GetClaim(context.Background(), 0)
			require.Nil(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, gameBlockNumber, err := proxy.GetBlockRange(ctx)
			require.Nil(t, err)
			l2Output, err := rollupClient.OutputAtBlock(ctx, gameBlockNumber)
			require.Nil(t, err)
			require.EqualValues(t, l2Output.OutputRoot, claim.Value)
			break
		}

		select {
		case <-timeoutCh:
			t.Fatalf("State root oracle not updated")
		case <-ticker.C:
		}
	}
}

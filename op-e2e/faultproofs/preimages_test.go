package faultproofs

import (
	"context"
	"fmt"
	"testing"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-program/client/boot"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/disputegame"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestLocalPreimages(t *testing.T) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)
	tests := []struct {
		key preimage.Key
	}{
		{key: boot.L1HeadLocalIndex},
		{key: boot.L2OutputRootLocalIndex},
		{key: boot.L2ClaimLocalIndex},
		{key: boot.L2ClaimBlockNumberLocalIndex},
		// We don't check client.L2ChainIDLocalIndex because e2e tests use a custom chain configuration
		// which requires using a custom chain ID indicator so op-program will load the full rollup config and
		// genesis from the preimage oracle
	}
	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("preimage-%v", test.key), func(t *testing.T) {
			op_e2e.InitParallel(t, op_e2e.UsesCannon)

			ctx := context.Background()
			sys, _ := StartFaultDisputeSystem(t)
			t.Cleanup(sys.Close)

			disputeGameFactory := disputegame.NewFactoryHelper(t, ctx, sys)
			game := disputeGameFactory.StartOutputCannonGame(ctx, "sequencer", 3, common.Hash{0x01, 0xaa})
			require.NotNil(t, game)
			claim := game.DisputeLastBlock(ctx)

			// Create the root of the cannon trace.
			claim = claim.Attack(ctx, common.Hash{0x01})

			game.LogGameData(ctx)

			game.VerifyPreimage(ctx, claim, test.key)

			game.LogGameData(ctx)
		})
	}
}

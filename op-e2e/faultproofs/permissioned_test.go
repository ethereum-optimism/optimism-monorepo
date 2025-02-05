package faultproofs

import (
	"context"
	"testing"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/challenger"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/disputegame"
	"github.com/ethereum/go-ethereum/common"
)

func TestPermissionedGameType(t *testing.T) {
	op_e2e.InitParallel(t, op_e2e.UsesCannon)

	ctx := context.Background()
	sys, _ := StartFaultDisputeSystem(t)
	t.Cleanup(sys.Close)

	gameFactory := disputegame.NewFactoryHelper(t, ctx, sys, disputegame.WithFactoryPrivKey(sys.Cfg.Secrets.Proposer))

	game := gameFactory.StartPermissionedGame(ctx, "sequencer", 1, common.Hash{0x01, 0xaa})

	// Start a challenger with both cannon and alphabet support
	gameFactory.StartChallenger(ctx, "TowerDefense",
		challenger.WithValidPrestateRequired(),
		challenger.WithInvalidCannonPrestate(),
		challenger.WithPermissioned(t, sys),
		challenger.WithPrivKey(sys.Cfg.Secrets.Alice),
	)

	// Wait for the challenger to respond
	game.RootClaim(ctx).WaitForCounterClaim(ctx)
}

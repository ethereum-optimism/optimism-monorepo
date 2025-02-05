package p2p

import (
	"math/big"
	"testing"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestTxGossip(t *testing.T) {
	op_e2e.InitParallel(t)
	cfg := e2esys.DefaultSystemConfig(t)
	gethOpts := []geth.GethOption{
		geth.WithP2P(),
	}
	cfg.GethOptions["sequencer"] = append(cfg.GethOptions["sequencer"], gethOpts...)
	cfg.GethOptions["verifier"] = append(cfg.GethOptions["verifier"], gethOpts...)
	sys, err := cfg.Start(t)
	require.NoError(t, err, "Start system")

	seqClient := sys.NodeClient("sequencer")
	verifClient := sys.NodeClient("verifier")
	geth.ConnectP2P(t, seqClient, verifClient)

	// This prevents the below tx-sending from flaking in CI
	_, err = geth.WaitForBlock(big.NewInt(10), verifClient)
	require.NoError(t, err)

	// Send a transaction to the verifier and it should be gossiped to the sequencer and included in a block.
	helpers.SendL2Tx(t, cfg, verifClient, cfg.Secrets.Alice, func(opts *helpers.TxOpts) {
		opts.ToAddr = &common.Address{0xaa}
		opts.Value = common.Big1
		opts.VerifyOnClients(seqClient, verifClient)
	})
}

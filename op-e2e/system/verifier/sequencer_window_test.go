package verifier

import (
	"context"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMissingBatchE2E(t *testing.T) {
	op_e2e.InitParallel(t)
	// Note this test zeroes the balance of the batch-submitter to make the batches unable to go into L1.
	// The test logs may look scary, but this is expected:
	// 'batcher unable to publish transaction    role=batcher   err="insufficient funds for gas * price + value"'

	cfg := e2esys.DefaultSystemConfig(t)
	// small sequence window size so the test does not take as long
	cfg.DeployConfig.SequencerWindowSize = 4

	// Specifically set batch submitter balance to stop batches from being included
	cfg.Premine[cfg.Secrets.Addresses().Batcher] = big.NewInt(0)

	sys, err := cfg.Start(t)
	require.Nil(t, err, "Error starting up system")

	l2Seq := sys.NodeClient("sequencer")
	l2Verif := sys.NodeClient("verifier")

	seqRollupClient := sys.RollupClient("sequencer")

	// Transactor Account
	ethPrivKey := cfg.Secrets.Alice

	// Submit TX to L2 sequencer node
	receipt := helpers.SendL2Tx(t, cfg, l2Seq, ethPrivKey, func(opts *helpers.TxOpts) {
		opts.ToAddr = &common.Address{0xff, 0xff}
		opts.Value = big.NewInt(1_000_000_000)
	})

	// Wait until the block it was first included in shows up in the safe chain on the verifier
	_, err = geth.WaitForBlock(receipt.BlockNumber, l2Verif)
	require.Nil(t, err, "Waiting for block on verifier")

	// Assert that the transaction is not found on the verifier
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = l2Verif.TransactionReceipt(ctx, receipt.TxHash)
	require.Equal(t, ethereum.NotFound, err, "Found transaction in verifier when it should not have been included")

	// Wait a short time for the L2 reorg to occur on the sequencer as well.
	err = wait.ForSafeBlock(ctx, seqRollupClient, receipt.BlockNumber.Uint64())
	require.Nil(t, err, "timeout waiting for L2 reorg on sequencer safe head")

	// Assert that the reconciliation process did an L2 reorg on the sequencer to remove the invalid block
	ctx2, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	block, err := l2Seq.BlockByNumber(ctx2, receipt.BlockNumber)
	if err != nil {
		require.Equal(t, "not found", err.Error(), "A not found error indicates the chain must have re-orged back before it")
	} else {
		require.NotEqual(t, block.Hash(), receipt.BlockHash, "L2 Sequencer did not reorg out transaction on it's safe chain")
	}
}

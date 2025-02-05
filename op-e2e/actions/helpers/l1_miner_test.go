package helpers

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestL1Miner_BuildBlock(gt *testing.T) {
	t := NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, DefaultRollupTestParams())
	sd := e2eutils.Setup(t, dp, DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)
	miner := NewL1Miner(t, log, sd.L1Cfg)
	t.Cleanup(func() {
		_ = miner.Close()
	})

	cl := miner.EthClient()
	signer := types.LatestSigner(sd.L1Cfg.Config)

	// send a tx to the miner
	tx := types.MustSignNewTx(dp.Secrets.Alice, signer, &types.DynamicFeeTx{
		ChainID:   sd.L1Cfg.Config.ChainID,
		Nonce:     0,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: new(big.Int).Add(miner.l1Chain.CurrentBlock().BaseFee, big.NewInt(2*params.GWei)),
		Gas:       params.TxGas,
		To:        &dp.Addresses.Bob,
		Value:     e2eutils.Ether(2),
	})
	require.NoError(gt, cl.SendTransaction(t.Ctx(), tx))

	// make an empty block, even though a tx may be waiting
	miner.ActL1StartBlock(10)(t)
	miner.ActL1EndBlock(t)
	header := miner.l1Chain.CurrentBlock()
	bl := miner.l1Chain.GetBlockByHash(header.Hash())
	require.Equal(t, uint64(1), bl.NumberU64())
	require.Zero(gt, bl.Transactions().Len())

	// now include the tx when we want it to
	miner.ActL1StartBlock(10)(t)
	miner.ActL1IncludeTx(dp.Addresses.Alice)(t)
	miner.ActL1EndBlock(t)
	header = miner.l1Chain.CurrentBlock()
	bl = miner.l1Chain.GetBlockByHash(header.Hash())
	require.Equal(t, uint64(2), bl.NumberU64())
	require.Equal(t, 1, bl.Transactions().Len())
	require.Equal(t, tx.Hash(), bl.Transactions()[0].Hash())

	// now make a replica that syncs these two blocks from the miner
	replica := NewL1Replica(t, log, sd.L1Cfg)
	t.Cleanup(func() {
		_ = replica.Close()
	})
	replica.ActL1Sync(miner.CanonL1Chain())(t)
	replica.ActL1Sync(miner.CanonL1Chain())(t)
	require.Equal(t, replica.l1Chain.CurrentBlock().Hash(), miner.l1Chain.CurrentBlock().Hash())
}

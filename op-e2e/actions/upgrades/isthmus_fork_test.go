package upgrades

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsthmusActivationAtGenesis(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	env := helpers.SetupEnv(t, helpers.WithActiveGenesisFork(rollup.Isthmus))

	// Start op-nodes
	env.Seq.ActL2PipelineFull(t)
	env.Verifier.ActL2PipelineFull(t)

	// Verify Isthmus is active at genesis
	l2Head := env.Seq.L2Unsafe()
	require.NotZero(t, l2Head.Hash)
	require.True(t, env.SetupData.RollupCfg.IsIsthmus(l2Head.Time), "Isthmus should be active at genesis")

	// build empty L1 block
	env.Miner.ActEmptyBlock(t)

	// Build L2 chain and advance safe head
	env.Seq.ActL1HeadSignal(t)
	env.Seq.ActBuildToL1Head(t)

	block := env.VerifEngine.L2Chain().CurrentBlock()
	verifyIsthmusHeaderWithdrawalsRoot(gt, env.SeqEngine.RPCClient(), block, true)
}

// There are 2 stages pre-Isthmus that we need to test:
// 1. Pre-Canyon: withdrawals root should be nil
// 2. Post-Canyon: withdrawals root should be EmptyWithdrawalsHash
func TestWithdrawlsRootPreCanyonAndIsthmus(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	genesisBlock := hexutil.Uint64(0)
	canyonOffset := hexutil.Uint64(2)

	log := testlog.Logger(t, log.LvlDebug)

	dp.DeployConfig.L1CancunTimeOffset = &canyonOffset

	// Activate pre-canyon forks at genesis, and schedule Canyon the block after
	dp.DeployConfig.L2GenesisRegolithTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisCanyonTimeOffset = &canyonOffset
	dp.DeployConfig.L2GenesisDeltaTimeOffset = nil
	dp.DeployConfig.L2GenesisEcotoneTimeOffset = nil
	dp.DeployConfig.L2GenesisFjordTimeOffset = nil
	dp.DeployConfig.L2GenesisGraniteTimeOffset = nil
	dp.DeployConfig.L2GenesisHoloceneTimeOffset = nil
	dp.DeployConfig.L2GenesisIsthmusTimeOffset = nil
	require.NoError(t, dp.DeployConfig.Check(log), "must have valid config")

	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	_, _, _, sequencer, engine, verifier, _, _ := helpers.SetupReorgTestActors(t, dp, sd, log)

	// start op-nodes
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	verifyPreCanyonHeaderWithdrawalsRoot(gt, engine.L2Chain().CurrentBlock())

	// build blocks until canyon activates
	sequencer.ActBuildL2ToCanyon(t)

	// Send withdrawal transaction
	// Bind L2 Withdrawer Contract
	ethCl := engine.EthClient()
	l2withdrawer, err := bindings.NewL2ToL1MessagePasser(predeploys.L2ToL1MessagePasserAddr, ethCl)
	require.Nil(t, err, "binding withdrawer on L2")

	// Initiate Withdrawal
	l2opts, err := bind.NewKeyedTransactorWithChainID(dp.Secrets.Alice, new(big.Int).SetUint64(dp.DeployConfig.L2ChainID))
	require.Nil(t, err)
	l2opts.Value = big.NewInt(500)

	l2withdrawer.Receive(l2opts)

	// mine blocks
	sequencer.ActL2EmptyBlock(t)
	sequencer.ActL2EmptyBlock(t)

	verifyPreIsthmusHeaderWithdrawalsRoot(gt, engine.L2Chain().CurrentBlock())
}

// In this section, we will test the following combinations
// 1. Withdrawals root before isthmus w/ and w/o L2toL1 withdrawal
// 2. Withdrawals root at isthmus w/ and w/o L2toL1 withdrawal
// 3. Withdrawals root after isthmus w/ and w/o L2toL1 withdrawal
func TestWithdrawalsRootBeforeAtAndAfterIsthmus(t *testing.T) {
	tests := []struct {
		name              string
		f                 func(gt *testing.T, withdrawalTx bool, withdrawalTxBlock, totalBlocks int)
		withdrawalTx      bool
		withdrawalTxBlock int
		totalBlocks       int
	}{
		{"BeforeIsthmusWithoutWithdrawalTx", testWithdrawlsRootAtIsthmus, false, 0, 1},
		{"BeforeIsthmusWithWithdrawalTx", testWithdrawlsRootAtIsthmus, true, 1, 1},
		{"AtIsthmusWithoutWithdrawalTx", testWithdrawlsRootAtIsthmus, false, 0, 2},
		{"AtIsthmusWithWithdrawalTx", testWithdrawlsRootAtIsthmus, true, 2, 2},
		{"AfterIsthmusWithoutWithdrawalTx", testWithdrawlsRootAtIsthmus, false, 0, 3},
		{"AfterIsthmusWithWithdrawalTx", testWithdrawlsRootAtIsthmus, true, 3, 3},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			test.f(t, test.withdrawalTx, test.withdrawalTxBlock, test.totalBlocks)
		})
	}
}

func testWithdrawlsRootAtIsthmus(gt *testing.T, withdrawalTx bool, withdrawalTxBlock, totalBlocks int) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	genesisBlock := hexutil.Uint64(0)
	isthmusOffset := hexutil.Uint64(2)

	log := testlog.Logger(t, log.LvlDebug)

	dp.DeployConfig.L2GenesisRegolithTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisCanyonTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisIsthmusTimeOffset = &isthmusOffset
	dp.DeployConfig.L2GenesisDeltaTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisEcotoneTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisFjordTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisGraniteTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisHoloceneTimeOffset = &genesisBlock
	require.NoError(t, dp.DeployConfig.Check(log), "must have valid config")

	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	_, _, _, sequencer, engine, verifier, _, _ := helpers.SetupReorgTestActors(t, dp, sd, log)

	// start op-nodes
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	verifyPreIsthmusHeaderWithdrawalsRoot(gt, engine.L2Chain().CurrentBlock())

	ethCl := engine.EthClient()
	for i := 1; i <= totalBlocks; i++ {
		var tx *types.Transaction

		sequencer.ActL2StartBlock(t)

		if withdrawalTx && withdrawalTxBlock == i {
			l2withdrawer, err := bindings.NewL2ToL1MessagePasser(predeploys.L2ToL1MessagePasserAddr, ethCl)
			require.Nil(t, err, "binding withdrawer on L2")

			// Initiate Withdrawal
			// Bind L2 Withdrawer Contract and invoke the Receive function
			l2opts, err := bind.NewKeyedTransactorWithChainID(dp.Secrets.Alice, new(big.Int).SetUint64(dp.DeployConfig.L2ChainID))
			require.Nil(t, err)
			l2opts.Value = big.NewInt(500)
			tx, err = l2withdrawer.Receive(l2opts)
			require.Nil(t, err)

			// include the transaction
			engine.ActL2IncludeTx(dp.Addresses.Alice)(t)
		}
		sequencer.ActL2EndBlock(t)

		if withdrawalTx && withdrawalTxBlock == i {
			// wait for withdrawal to be included in a block
			receipt, err := geth.WaitForTransaction(tx.Hash(), ethCl, 10*time.Duration(dp.DeployConfig.L2BlockTime)*time.Second)
			require.Nil(t, err, "withdrawal initiated on L2 sequencer")
			require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "transaction had incorrect status")
		}
	}
	rpcCl := engine.RPCClient()

	// we set withdrawals root only at or after isthmus
	if totalBlocks >= 2 {
		verifyIsthmusHeaderWithdrawalsRoot(gt, rpcCl, engine.L2Chain().CurrentBlock(), true)
	}
}

func TestWithdrawlsRootPostIsthmus(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	genesisBlock := hexutil.Uint64(0)
	isthmusOffset := hexutil.Uint64(2)

	log := testlog.Logger(t, log.LvlDebug)

	dp.DeployConfig.L2GenesisRegolithTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisCanyonTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisIsthmusTimeOffset = &isthmusOffset
	dp.DeployConfig.L2GenesisDeltaTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisEcotoneTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisFjordTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisGraniteTimeOffset = &genesisBlock
	dp.DeployConfig.L2GenesisHoloceneTimeOffset = &genesisBlock
	require.NoError(t, dp.DeployConfig.Check(log), "must have valid config")

	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	_, _, _, sequencer, engine, verifier, _, _ := helpers.SetupReorgTestActors(t, dp, sd, log)

	// start op-nodes
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	verifyPreIsthmusHeaderWithdrawalsRoot(gt, engine.L2Chain().CurrentBlock())

	rpcCl := engine.RPCClient()
	verifyIsthmusHeaderWithdrawalsRoot(gt, rpcCl, engine.L2Chain().CurrentBlock(), false)

	// Send withdrawal transaction
	// Bind L2 Withdrawer Contract
	ethCl := engine.EthClient()
	l2withdrawer, err := bindings.NewL2ToL1MessagePasser(predeploys.L2ToL1MessagePasserAddr, ethCl)
	require.Nil(t, err, "binding withdrawer on L2")

	// Initiate Withdrawal
	l2opts, err := bind.NewKeyedTransactorWithChainID(dp.Secrets.Alice, new(big.Int).SetUint64(dp.DeployConfig.L2ChainID))
	require.Nil(t, err)
	l2opts.Value = big.NewInt(500)

	tx, err := l2withdrawer.Receive(l2opts)
	require.Nil(t, err)

	// build blocks until Isthmus activates
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)
	sequencer.ActL2StartBlock(t)
	engine.ActL2IncludeTx(dp.Addresses.Alice)(t)
	sequencer.ActL2EndBlock(t)

	// wait for withdrawal to be included in a block
	receipt, err := geth.WaitForTransaction(tx.Hash(), ethCl, 10*time.Duration(dp.DeployConfig.L2BlockTime)*time.Second)
	require.Nil(t, err, "withdrawal initiated on L2 sequencer")
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "transaction had incorrect status")

	verifyIsthmusHeaderWithdrawalsRoot(gt, rpcCl, engine.L2Chain().CurrentBlock(), true)
}

// Pre-Canyon, the withdrawals root field in the header should be nil
func verifyPreCanyonHeaderWithdrawalsRoot(gt *testing.T, header *types.Header) {
	require.Nil(gt, header.WithdrawalsHash)
}

// Post-Canyon, the withdrawals root field in the header should be EmptyWithdrawalsHash
func verifyPreIsthmusHeaderWithdrawalsRoot(gt *testing.T, header *types.Header) {
	require.Equal(gt, types.EmptyWithdrawalsHash, *header.WithdrawalsHash)
}

func verifyIsthmusHeaderWithdrawalsRoot(gt *testing.T, rpcCl client.RPC, header *types.Header, l2toL1MPPresent bool) {
	getStorageRoot := func(rpcCl client.RPC, ctx context.Context, address common.Address, blockTag string) common.Hash {
		var getProofResponse *eth.AccountResult
		err := rpcCl.CallContext(ctx, &getProofResponse, "eth_getProof", address, []common.Hash{}, blockTag)
		assert.Nil(gt, err)
		assert.NotNil(gt, getProofResponse)
		return getProofResponse.StorageHash
	}

	if !l2toL1MPPresent {
		require.Equal(gt, types.EmptyWithdrawalsHash, *header.WithdrawalsHash)
	} else {
		storageHash := getStorageRoot(rpcCl, context.Background(), predeploys.L2ToL1MessagePasserAddr, "latest")
		require.Equal(gt, *header.WithdrawalsHash, storageHash)
	}
}

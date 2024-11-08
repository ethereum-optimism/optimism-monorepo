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
	verifyIsthmusHeaderWithdrawalsRoot(gt, block, false)
}

// There are 2 stages pre-Isthmus that we need to test:
// 1. Pre-Canyon: withdrawals root should be nil
// 2. Post-Canyon: withdrawals root should be EmptyWithdrawalsHash
func TestWithdrawlsRootPreIsthmus(gt *testing.T) {
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

func TestWithdrawlsRootAtIsthmus(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	genesisBlock := hexutil.Uint64(0)
	isthmusOffset := hexutil.Uint64(2)

	log := testlog.Logger(t, log.LvlDebug)

	// dp.DeployConfig.L1CancunTimeOffset = &genesisBlock

	// Activate pre-canyon forks at genesis, and schedule Canyon the block after
	// dummyAddr := common.Address{19: 0x01}
	// dp.DeployConfig.OutputOracleDeployConfig = genesis.OutputOracleDeployConfig{
	// 	L2OutputOracleSubmissionInterval:  1,
	// 	L2OutputOracleStartingTimestamp:   1,
	// 	L2OutputOracleStartingBlockNumber: 3,
	// 	L2OutputOracleProposer:            dummyAddr,
	// 	L2OutputOracleChallenger:          dummyAddr,
	// }
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

	getStorageRoot := func(rpcCl client.RPC, ctx context.Context, address common.Address, blockTag string) common.Hash {
		var getProofResponse *eth.AccountResult
		err := rpcCl.CallContext(ctx, &getProofResponse, "eth_getProof", address, []common.Hash{}, blockTag)
		assert.Nil(t, err)
		assert.NotNil(t, getProofResponse)
		return getProofResponse.StorageHash
	}

	rpcCl := engine.RPCClient()
	currentBlock := engine.L2Chain().CurrentBlock()
	t.Log("Current block number: ", currentBlock.Number)
	storageHash := getStorageRoot(rpcCl, context.Background(), predeploys.L2ToL1MessagePasserAddr, "latest")
	t.Log("Storage hash: ", storageHash)
	// require.Equal(t, *currentBlock.WithdrawalsHash, storageHash)

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
	engine.ActL2IncludeTx(dp.Addresses.Alice)(t)
	sequencer.ActL2EndBlock(t)

	receipt, err := geth.WaitForTransaction(tx.Hash(), ethCl, 10*time.Duration(dp.DeployConfig.L2BlockTime)*time.Second)
	require.Nil(t, err, "withdrawal initiated on L2 sequencer")
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "transaction had incorrect status")

	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)

	currentBlock = engine.L2Chain().CurrentBlock()
	t.Log("Current block number: ", currentBlock.Number)
	storageHash = getStorageRoot(rpcCl, context.Background(), predeploys.L2ToL1MessagePasserAddr, "latest")
	require.Equal(t, *currentBlock.WithdrawalsHash, storageHash)
	// verifyIsthmusHeaderWithdrawalsRoot(gt, engine.L2Chain().CurrentBlock(), true)
}

// Pre-Canyon, the withdrawals root field in the header should be nil
func verifyPreCanyonHeaderWithdrawalsRoot(gt *testing.T, header *types.Header) {
	require.Nil(gt, header.WithdrawalsHash)
}

// Post-Canyon, the withdrawals root field in the header should be EmptyWithdrawalsHash
func verifyPreIsthmusHeaderWithdrawalsRoot(gt *testing.T, header *types.Header) {
	require.Equal(gt, types.EmptyWithdrawalsHash, *header.WithdrawalsHash)
}

func verifyIsthmusHeaderWithdrawalsRoot(gt *testing.T, header *types.Header, l2toL1MPPresent bool) {
	if !l2toL1MPPresent {
		require.Equal(gt, types.EmptyWithdrawalsHash, *header.WithdrawalsHash)
	} else {
		require.NotEqual(gt, types.EmptyWithdrawalsHash, *header.WithdrawalsHash)
	}
}

// func getL2ToL1MP(t *testing.T, engine *e2eutils.Engine) *types.Transaction {
// 	// Get the L2 to L1 message passing tx
// 	l2ToL1MP, err := engine.L2Chain().CurrentBlock().GetL2ToL1MessagePassingTx()
// 	require.NoError(t, err)
// 	return l2ToL1MP
// }

// func getL2ToL1MPStorageRoot(t *testing.T, ethCl *ethclient.Client) string {
// 	proof, err := ethCl.GetProof(ctx, predeploys.L2ToL1MessagePasserAddr, []common.Hash{}, blockHash.String())
// 	rootValue, err := ethCl.StorageAt(context.Background(), predeploys.L2ToL1MessagePasserAddr, rootIdx, nil)
// 	require.NoError(t, err)
// 	require.Equal(t, expectedHash, common.BytesToHash(rootValue), msg)
// }

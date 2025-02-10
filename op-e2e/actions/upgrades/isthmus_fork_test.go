package upgrades

import (
	"bytes"
	"context"
	"log/slog"
	"math/big"
	"testing"
	"time"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	batcherFlags "github.com/ethereum-optimism/optimism/op-batcher/flags"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	upgradesHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/upgrades/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm/program"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"
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

	_, err = l2withdrawer.Receive(l2opts)
	require.Nil(t, err)

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

func TestIsthmusNetworkUpgradeTransactions(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	isthmusOffset := hexutil.Uint64(4)

	log := testlog.Logger(t, log.LevelDebug)

	zero := hexutil.Uint64(0)

	// Activate all forks at genesis, and schedule Ecotone the block after
	dp.DeployConfig.L2GenesisHoloceneTimeOffset = &zero
	dp.DeployConfig.L2GenesisIsthmusTimeOffset = &isthmusOffset
	dp.DeployConfig.L1PragueTimeOffset = nil
	// New forks have to be added here...
	require.NoError(t, dp.DeployConfig.Check(log), "must have valid config")

	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	_, _, _, sequencer, engine, verifier, _, _ := helpers.SetupReorgTestActors(t, dp, sd, log)
	ethCl := engine.EthClient()

	// build a single block to move away from the genesis with 0-values in L1Block contract
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)

	// start op-nodes
	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	// Build to the isthmus block
	sequencer.ActBuildL2ToIsthmus(t)

	// get latest block
	latestBlock, err := ethCl.BlockByNumber(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, sequencer.L2Unsafe().Number, latestBlock.Number().Uint64())

	transactions := latestBlock.Transactions()

	// L1Block: 1 set-L1-info + 1 deploy
	// See [derive.IsthmusNetworkUpgradeTransactions]
	require.Equal(t, 2, len(transactions))

	// Contract deployment transaction
	txn := transactions[1]
	receipt, err := ethCl.TransactionReceipt(context.Background(), txn.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "block hashes deployment tx must pass")
	require.NotEmpty(t, txn.Data(), "upgrade tx must provide input data")

	// EIP-2935 contract is deployed
	expectedBlockHashAddress := crypto.CreateAddress(derive.BlockHashDeployerAddress, 0)
	require.Equal(t, predeploys.EIP2935ContractAddr, expectedBlockHashAddress)
	code := verifyCodeHashMatches(t, ethCl, predeploys.EIP2935ContractAddr, predeploys.EIP2935ContractCodeHash)
	require.Equal(t, predeploys.EIP2935ContractCode, code)

	// Test that the beacon-block-root has been set
	checkRecentBlockHash := func(blockNumber uint64, expectedHash common.Hash, msg string) {
		historyBufferLength := uint64(8191)
		bufferIdx := common.BigToHash(new(big.Int).SetUint64(blockNumber % historyBufferLength))

		rootValue, err := ethCl.StorageAt(context.Background(), predeploys.EIP2935ContractAddr, bufferIdx, nil)
		require.NoError(t, err)
		require.Equal(t, expectedHash, common.BytesToHash(rootValue), msg)
	}

	// Legacy check:
	// > The first block is an exception in upgrade-networks,
	// > since the recent-block-hash contract isn't there at Isthmus activation,
	// > and the recent-block-hash insertion is processed at the start of the block before deposit txs.
	// > If the contract was permissionlessly deployed before, the contract storage will be updated (but not in this test).
	checkRecentBlockHash(latestBlock.NumberU64(), common.Hash{}, "isthmus activation block has no data yet (since contract wasn't there)")

	// Build empty L2 block, to pass Isthmus activation
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)

	// Test the L2 block after activation: it should have the most recent block hash
	latestBlock, err = ethCl.BlockByNumber(context.Background(), nil)
	require.NoError(t, err)
	checkRecentBlockHash(latestBlock.NumberU64()-1, latestBlock.Header().ParentHash, "post-activation")
}

func TestSetCodeTxTypeIsthmus(gt *testing.T) {
	// go-ethereum test called TestEIP7702 reimplemented here
	// https://github.com/ethereum/go-ethereum/blob/39638c81c56db2b2dfe6f51999ffd3029ee212cb/core/blockchain_test.go#L4180
	t := actionsHelpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20,
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	dp := e2eutils.MakeDeployParams(t, p)
	dp.DeployConfig.ActivateForkAtGenesis(rollup.Isthmus)
	minTs := hexutil.Uint64(0)
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)

	var (
		aa = common.HexToAddress("0x000000000000000000000000000000000000aaaa")
		bb = common.HexToAddress("0x000000000000000000000000000000000000bbbb")
	)

	// Create 2 contracts, (1) writes 42 to slot 42, (2) calls (1)
	store42Program := program.New().Sstore(0x42, 0x42)
	callBobProgram := program.New().Call(nil, dp.Addresses.Bob, 1, 0, 0, 0, 0)

	alloc := *actionsHelpers.DefaultAlloc
	alloc.L2Alloc = make(map[common.Address]types.Account)
	alloc.L2Alloc[aa] = types.Account{
		Code: store42Program.Bytes(),
	}
	alloc.L2Alloc[bb] = types.Account{
		Code: callBobProgram.Bytes(),
	}

	sd := e2eutils.Setup(t, dp, &alloc)
	log := testlog.Logger(t, log.LevelError)
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)
	_, verifier := actionsHelpers.SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{})
	rollupSeqCl := sequencer.RollupClient()
	cl := seqEngine.EthClient()

	batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, &actionsHelpers.BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		BatcherKey:           dp.Secrets.Batcher,
		DataAvailabilityType: batcherFlags.CalldataType,
	}, rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	miner.ActEmptyBlock(t)

	sequencer.ActL2StartBlock(t)

	// Sign authorization tuples.
	// The way the auths are combined, it becomes
	// 1. tx -> addr1 which is delegated to 0xaaaa
	// 2. addr1:0xaaaa calls into addr2:0xbbbb
	// 3. addr2:0xbbbb  writes to storage
	auth1, err := types.SignSetCode(dp.Secrets.Alice, types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(dp.DeployConfig.L2ChainID),
		Address: bb,
		Nonce:   1,
	})
	require.NoError(gt, err, "failed to sign auth1")
	auth2, err := types.SignSetCode(dp.Secrets.Bob, types.SetCodeAuthorization{
		Address: aa,
		Nonce:   0,
	})
	require.NoError(gt, err, "failed to sign auth2")

	txdata := &types.SetCodeTx{
		ChainID:   uint256.NewInt(dp.DeployConfig.L2ChainID),
		Nonce:     0,
		To:        dp.Addresses.Alice,
		Gas:       500000,
		GasFeeCap: uint256.NewInt(5000000000),
		GasTipCap: uint256.NewInt(2),
		AuthList:  []types.SetCodeAuthorization{auth1, auth2},
	}
	signer := types.NewPragueSigner(new(big.Int).SetUint64(dp.DeployConfig.L2ChainID))
	tx := types.MustSignNewTx(dp.Secrets.Alice, signer, txdata)

	err = cl.SendTransaction(t.Ctx(), tx)
	require.NoError(gt, err, "failed to send set code tx")

	require.NoError(t, seqEngine.EngineApi.IncludeTx(tx, dp.Addresses.Alice), "failed to include set code tx")

	sequencer.ActL2EndBlock(t)

	// Verify delegation designations were deployed.
	bobCode, err := cl.PendingCodeAt(t.Ctx(), dp.Addresses.Bob)
	require.NoError(gt, err, "failed to get bob code")
	want := types.AddressToDelegation(auth2.Address)
	if !bytes.Equal(bobCode, want) {
		t.Fatalf("addr1 code incorrect: got %s, want %s", common.Bytes2Hex(bobCode), common.Bytes2Hex(want))
	}
	aliceCode, err := cl.PendingCodeAt(t.Ctx(), dp.Addresses.Alice)
	require.NoError(gt, err, "failed to get alice code")
	want = types.AddressToDelegation(auth1.Address)
	if !bytes.Equal(aliceCode, want) {
		t.Fatalf("addr2 code incorrect: got %s, want %s", common.Bytes2Hex(aliceCode), common.Bytes2Hex(want))
	}

	// Verify delegation executed the correct code.
	fortyTwo := common.BytesToHash([]byte{0x42})
	actual, err := cl.PendingStorageAt(t.Ctx(), dp.Addresses.Bob, fortyTwo)
	require.NoError(gt, err, "failed to get addr1 storage")

	if !bytes.Equal(actual, fortyTwo[:]) {
		t.Fatalf("addr2 storage wrong: expected %d, got %d", fortyTwo, actual)
	}

	// batch submit to L1. batcher should submit span batches.
	batcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)

	// ensure verifier can verify the batch (needs authorization list or tx will fail)
	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	require.Equal(t, sequencer.L2Unsafe(), sequencer.L2Safe())
	require.Equal(t, verifier.L2Unsafe(), verifier.L2Safe())
	require.Equal(t, sequencer.L2Safe(), verifier.L2Safe())
}

func TestSetCodeTxTypePreIsthmus(gt *testing.T) {
	// Ensure that batches that include SetCodeTxs are dropped if before Isthmus
	// Sets up a network with Isthmus starting at block 1
	// Send SetCodeTx at block 3
	// Verifier pipeline uses Isthmus starting at block 5
	// Ensure verifier drops the batch with a SetCodeTx too early

	t := actionsHelpers.NewDefaultTesting(gt)
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   20,
		SequencerWindowSize: 24,
		ChannelTimeout:      20,
		L1BlockTime:         12,
		AllocType:           config.AllocTypeStandard,
	}
	dp := e2eutils.MakeDeployParams(t, p)
	dp.DeployConfig.ActivateForkAtOffset(rollup.Isthmus, 2)
	minTs := hexutil.Uint64(0)
	upgradesHelpers.ApplyDeltaTimeOffset(dp, &minTs)

	var (
		aa = common.HexToAddress("0x000000000000000000000000000000000000aaaa")
		bb = common.HexToAddress("0x000000000000000000000000000000000000bbbb")
	)

	// Create 2 contracts, (1) writes 42 to slot 42, (2) calls (1)
	store42Program := program.New().Sstore(0x42, 0x42)
	callBobProgram := program.New().Call(nil, dp.Addresses.Bob, 1, 0, 0, 0, 0)

	alloc := actionsHelpers.DefaultAlloc
	alloc.L2Alloc = make(map[common.Address]types.Account)
	alloc.L2Alloc[aa] = types.Account{
		Code: store42Program.Bytes(),
	}
	alloc.L2Alloc[bb] = types.Account{
		Code: callBobProgram.Bytes(),
	}

	sd := e2eutils.Setup(t, dp, alloc)
	log, captureLogger := testlog.CaptureLogger(t, log.LevelDebug)
	miner, seqEngine, sequencer := actionsHelpers.SetupSequencerTest(t, sd, log)

	l1F := miner.L1Client(t, sd.RollupCfg)
	blobSrc := miner.BlobStore()
	syncCfg := &sync.Config{}
	cfg := actionsHelpers.DefaultVerifierCfg()
	jwtPath := e2eutils.WriteDefaultJWT(t)
	verifierEngine := actionsHelpers.NewL2Engine(t, log.New("role", "verifier-engine"), sd.L2Cfg, jwtPath, actionsHelpers.EngineWithP2P())
	engCl := verifierEngine.EngineClient(t, sd.RollupCfg)

	newIsthmusTime := uint64(sd.RollupCfg.Genesis.L2Time + 10)
	newRollupCfg := *sd.RollupCfg
	newRollupCfg.IsthmusTime = &newIsthmusTime
	verifier := actionsHelpers.NewL2Verifier(t, log.New("role", "verifier"), l1F, blobSrc, altda.Disabled, engCl, sd.RollupCfg, syncCfg, cfg.SafeHeadListener, &newRollupCfg)

	rollupSeqCl := sequencer.RollupClient()
	cl := seqEngine.EthClient()

	batcher := actionsHelpers.NewL2Batcher(log, sd.RollupCfg, &actionsHelpers.BatcherCfg{
		MinL1TxSize:          0,
		MaxL1TxSize:          128_000,
		BatcherKey:           dp.Secrets.Batcher,
		DataAvailabilityType: batcherFlags.CalldataType,
		ForceSubmitSpanBatch: true,
	}, rollupSeqCl, miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	sequencer.ActL2PipelineFull(t)
	verifier.ActL2PipelineFull(t)

	miner.ActEmptyBlock(t)

	sequencer.ActL2StartBlock(t) // 1
	sequencer.ActL2EndBlock(t)

	sequencer.ActL2StartBlock(t) // 2
	sequencer.ActL2EndBlock(t)

	sequencer.ActL2StartBlock(t) // 3 (bad)

	auth1, err := types.SignSetCode(dp.Secrets.Alice, types.SetCodeAuthorization{
		ChainID: *uint256.NewInt(dp.DeployConfig.L2ChainID),
		Address: bb,
		Nonce:   1,
	})
	require.NoError(gt, err, "failed to sign auth1")
	auth2, err := types.SignSetCode(dp.Secrets.Bob, types.SetCodeAuthorization{
		Address: aa,
		Nonce:   0,
	})
	require.NoError(gt, err, "failed to sign auth2")

	txdata := &types.SetCodeTx{
		ChainID:   uint256.NewInt(dp.DeployConfig.L2ChainID),
		Nonce:     0,
		To:        dp.Addresses.Alice,
		Gas:       500000,
		GasFeeCap: uint256.NewInt(5000000000),
		GasTipCap: uint256.NewInt(2),
		AuthList:  []types.SetCodeAuthorization{auth1, auth2},
	}
	signer := types.NewPragueSigner(new(big.Int).SetUint64(dp.DeployConfig.L2ChainID))
	tx := types.MustSignNewTx(dp.Secrets.Alice, signer, txdata)

	err = cl.SendTransaction(t.Ctx(), tx)
	require.NoError(gt, err, "failed to send set code tx")

	seqEngine.EngineApi.IncludeTx(tx, dp.Addresses.Alice)

	sequencer.ActL2EndBlock(t)

	// Verify delegation designations were deployed.
	bobCode, err := cl.PendingCodeAt(t.Ctx(), dp.Addresses.Bob)
	require.NoError(gt, err, "failed to get bob code")
	want := types.AddressToDelegation(auth2.Address)
	if !bytes.Equal(bobCode, want) {
		t.Fatalf("addr1 code incorrect: got %s, want %s", common.Bytes2Hex(bobCode), common.Bytes2Hex(want))
	}
	aliceCode, err := cl.PendingCodeAt(t.Ctx(), dp.Addresses.Alice)
	require.NoError(gt, err, "failed to get alice code")
	want = types.AddressToDelegation(auth1.Address)
	if !bytes.Equal(aliceCode, want) {
		t.Fatalf("addr2 code incorrect: got %s, want %s", common.Bytes2Hex(aliceCode), common.Bytes2Hex(want))
	}

	// Verify delegation executed the correct code.
	fortyTwo := common.BytesToHash([]byte{0x42})
	actual, err := cl.PendingStorageAt(t.Ctx(), dp.Addresses.Bob, fortyTwo)
	require.NoError(gt, err, "failed to get addr1 storage")

	if !bytes.Equal(actual, fortyTwo[:]) {
		t.Fatalf("addr2 storage wrong: expected %d, got %d", fortyTwo, actual)
	}

	// batch submit to L1. batcher should submit span batches.
	batcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	sequencer.ActL1HeadSignal(t)
	sequencer.ActL2PipelineFull(t)

	verifier.ActL1HeadSignal(t)
	verifier.ActL2PipelineFull(t)

	levelFilter := testlog.NewLevelFilter(slog.LevelWarn)
	msgFilter := testlog.NewMessageFilter("sequencers may not embed any SetCode transactions before Isthmus")
	msg := captureLogger.FindLog(levelFilter, msgFilter)
	require.NotNil(t, msg)

	// ensure sequencer has the latest block finalized
	require.Equal(t, sequencer.L2Unsafe(), sequencer.L2Safe())

	// ensure verifier dropped the last singular batch
	require.Equal(t, uint64(2), verifier.SyncStatus().SafeL2.Number)
	require.Equal(t, uint64(2), verifier.SyncStatus().UnsafeL2.Number)
}

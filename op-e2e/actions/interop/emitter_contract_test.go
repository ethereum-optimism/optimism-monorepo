package interop

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/emit"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/interop/contracts/bindings/inbox"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	stypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type userWithKeys struct {
	key     devkeys.ChainUserKey
	secret  *ecdsa.PrivateKey
	address common.Address
}

func TestEmitterContract(gt *testing.T) {
	var (
		t      = helpers.NewDefaultTesting(gt)
		is     *InteropSetup
		actors *InteropActors
		aliceA *userWithKeys
		aliceB *userWithKeys
		emitTx *types.Transaction
	)
	resetTest := func() {
		is = SetupInterop(t)
		actors = is.CreateActors()
		aliceA = setupUser(t, is, actors.ChainA, 0)
		aliceB = setupUser(t, is, actors.ChainB, 0)
		initializeChainState(t, actors)
		emitTx = initializeEmitterContractTest(t, aliceA, actors)
	}

	gt.Run("success", func(_ *testing.T) {
		resetTest()

		// Execute message on destination chain and verify that the heads progress
		execTx := newExecuteMessageTx(t, actors, actors.ChainB, aliceB, emitTx)
		includeTxOnChain(t, actors, actors.ChainB, execTx, aliceB.address)
		assertHeads(t, actors.ChainB, 3, 3, 3, 3)
	})

	gt.Run("failure with conflicting message", func(_ *testing.T) {
		resetTest()

		// Create a message with a conflicting payload
		fakeMessage := []byte("this message was never emitted")
		auth := newL2TxOpts(t, aliceB.secret, actors.ChainB)
		id := idForTx(t, actors, emitTx)
		contract, err := inbox.NewInbox(predeploys.CrossL2InboxAddr, actors.ChainB.SequencerEngine.EthClient())
		require.NoError(t, err)
		tx, err := contract.ValidateMessage(auth, id, crypto.Keccak256Hash(fakeMessage))
		require.NoError(t, err)

		// Process the invalid message attempt and verify that only the local unsafe head progresses
		includeTxOnChain(t, actors, actors.ChainB, tx, auth.From)
		assertHeads(t, actors.ChainB, 3, 3, 2, 2)
	})
}

func setupUser(t helpers.Testing, is *InteropSetup, chain *Chain, keyIndex int) *userWithKeys {
	userKey := devkeys.ChainUserKeys(chain.RollupCfg.L2ChainID)(uint64(keyIndex))
	secret, err := is.Keys.Secret(userKey)
	require.NoError(t, err)
	return &userWithKeys{
		key:     userKey,
		secret:  secret,
		address: crypto.PubkeyToAddress(secret.PublicKey),
	}
}

func newL2TxOpts(t helpers.Testing, key *ecdsa.PrivateKey, chain *Chain) *bind.TransactOpts {
	auth, err := bind.NewKeyedTransactorWithChainID(key, chain.RollupCfg.L2ChainID)
	require.NoError(t, err)
	auth.GasTipCap = big.NewInt(params.GWei)
	return auth
}

func newEmitMessageTx(t helpers.Testing, chain *Chain, user *userWithKeys, emitContract common.Address, msgData []byte) *types.Transaction {
	auth := newL2TxOpts(t, user.secret, chain)
	emitter, err := emit.NewEmit(emitContract, chain.SequencerEngine.EthClient())
	require.NoError(t, err)

	tx, err := emitter.EmitData(auth, msgData)
	require.NoError(t, err)

	return tx
}

func newExecuteMessageTx(t helpers.Testing, actors *InteropActors, destChain *Chain, executor *userWithKeys, srcTx *types.Transaction) *types.Transaction {
	// Create the id and payload
	id := idForTx(t, actors, srcTx)
	receipt, err := actors.ChainA.SequencerEngine.EthClient().TransactionReceipt(t.Ctx(), srcTx.Hash())
	require.NoError(t, err)
	payload := stypes.LogToMessagePayload(receipt.Logs[0])

	// Create the tx to validate the message
	inboxContract, err := inbox.NewInbox(predeploys.CrossL2InboxAddr, destChain.SequencerEngine.EthClient())
	require.NoError(t, err)
	auth := newL2TxOpts(t, executor.secret, destChain)
	tx, err := inboxContract.ValidateMessage(auth, id, crypto.Keccak256Hash(payload))
	require.NoError(t, err)

	return tx
}

func idForTx(t helpers.Testing, actors *InteropActors, tx *types.Transaction) inbox.Identifier {
	receipt, err := actors.ChainA.SequencerEngine.EthClient().TransactionReceipt(t.Ctx(), tx.Hash())
	require.NoError(t, err)
	block, err := actors.ChainA.SequencerEngine.EthClient().BlockByNumber(t.Ctx(), receipt.BlockNumber)
	require.NoError(t, err)

	return inbox.Identifier{
		Origin:      *tx.To(),
		BlockNumber: receipt.BlockNumber,
		LogIndex:    common.Big0,
		Timestamp:   big.NewInt(int64(block.Time())),
		ChainId:     actors.ChainA.RollupCfg.L2ChainID,
	}
}

func initializeChainState(t helpers.Testing, actors *InteropActors) {
	// Initialize both chain states
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)

	// Sync supervisors
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)

	// Verify initial state
	statusA := actors.ChainA.Sequencer.SyncStatus()
	statusB := actors.ChainB.Sequencer.SyncStatus()
	require.Equal(t, uint64(0), statusA.UnsafeL2.Number)
	require.Equal(t, uint64(0), statusB.UnsafeL2.Number)

	// Complete initial sync
	actors.Supervisor.ProcessFull(t)
}

func initializeEmitterContractTest(t helpers.Testing, aliceA *userWithKeys, actors *InteropActors) *types.Transaction {
	// Deploy message contract and emit a log on ChainA
	// This issues two blocks to ChainA
	auth := newL2TxOpts(t, aliceA.secret, actors.ChainA)
	emitContract, tx, _, err := emit.DeployEmit(auth, actors.ChainA.SequencerEngine.EthClient())
	require.NoError(t, err)
	includeTxOnChain(t, actors, actors.ChainA, tx, aliceA.address)
	emitTx := newEmitMessageTx(t, actors.ChainA, aliceA, emitContract, []byte("test message"))
	includeTxOnChain(t, actors, actors.ChainA, emitTx, aliceA.address)

	// Catch ChainB up to the same height/time as ChainA
	includeTxOnChain(t, actors, actors.ChainB, nil, aliceA.address)
	includeTxOnChain(t, actors, actors.ChainB, nil, aliceA.address)

	// Verify initial state
	assertHeads(t, actors.ChainA, 2, 2, 2, 2)
	assertHeads(t, actors.ChainB, 2, 2, 2, 2)

	return emitTx
}

func includeTxOnChain(t helpers.Testing, actors *InteropActors, chain *Chain, tx *types.Transaction, sender common.Address) {
	// Create L2 block on the given chain
	chain.Sequencer.ActL2StartBlock(t)
	if tx != nil {
		err := chain.SequencerEngine.EngineApi.IncludeTx(tx, sender)
		require.NoError(t, err)
	}
	chain.Sequencer.ActL2EndBlock(t)

	// Sync the chain and the supervisor
	chain.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)

	// Add to L1
	chain.Batcher.ActSubmitAll(t)
	actors.L1Miner.ActL1StartBlock(12)(t)
	actors.L1Miner.ActL1IncludeTx(chain.BatcherAddr)(t)
	actors.L1Miner.ActL1EndBlock(t)

	// Complete L1 data processing
	chain.Sequencer.ActL2EventsUntil(t, event.Is[derive.ExhaustedL1Event], 100, false)
	actors.Supervisor.SignalLatestL1(t)
	chain.Sequencer.SyncSupervisor(t)
	chain.Sequencer.ActL2PipelineFull(t)

	// Final sync of both chains
	actors.ChainA.Sequencer.SyncSupervisor(t)
	actors.ChainB.Sequencer.SyncSupervisor(t)
	actors.Supervisor.ProcessFull(t)
	actors.ChainA.Sequencer.ActL2PipelineFull(t)
	actors.ChainB.Sequencer.ActL2PipelineFull(t)
}

func assertHeads(t helpers.Testing, chain *Chain, unsafe, localSafe, crossUnsafe, safe uint64) {
	status := chain.Sequencer.SyncStatus()
	require.Equal(t, unsafe, status.UnsafeL2.ID().Number)
	require.Equal(t, crossUnsafe, status.CrossUnsafeL2.ID().Number)
	require.Equal(t, localSafe, status.LocalSafeL2.ID().Number)
	require.Equal(t, safe, status.SafeL2.ID().Number)
}

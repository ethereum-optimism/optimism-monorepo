package altda

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/config"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-alt-da/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/node/safedb"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

// Devnet allocs should have alt-da mode enabled for these tests to pass

// L2AltDA is a test harness for manipulating AltDA state.
type L2AltDA struct {
	log        log.Logger
	storage    *altda.DAErrFaker
	daMgr      *altda.DA
	altDACfg   altda.Config
	contract   *bindings.DataAvailabilityChallenge
	batcher    *helpers.L2Batcher
	sequencer  *helpers.L2Sequencer
	engine     *helpers.L2Engine
	engCl      *sources.EngineClient
	sd         *e2eutils.SetupData
	dp         *e2eutils.DeployParams
	miner      *helpers.L1Miner
	alice      *helpers.CrossLayerUser
	lastComm   []byte
	lastCommBn uint64
}

type AltDAParam func(p *e2eutils.TestParams)

func NewL2AltDA(t helpers.Testing, params ...AltDAParam) *L2AltDA {
	p := &e2eutils.TestParams{
		MaxSequencerDrift:   40,
		SequencerWindowSize: 12,
		ChannelTimeout:      12,
		L1BlockTime:         12,
		UseAltDA:            true,
		AllocType:           config.AllocTypeAltDA,
	}
	for _, apply := range params {
		apply(p)
	}
	log := testlog.Logger(t, log.LvlDebug)

	dp := e2eutils.MakeDeployParams(t, p)
	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)

	require.True(t, sd.RollupCfg.AltDAEnabled())

	miner := helpers.NewL1Miner(t, log, sd.L1Cfg)
	l1Client := miner.EthClient()

	jwtPath := e2eutils.WriteDefaultJWT(t)
	engine := helpers.NewL2Engine(t, log, sd.L2Cfg, jwtPath)
	engCl := engine.EngineClient(t, sd.RollupCfg)

	storage := &altda.DAErrFaker{Client: altda.NewMockDAClient(log)}

	l1F, err := sources.NewL1Client(miner.RPCClient(), log, nil, sources.L1ClientDefaultConfig(sd.RollupCfg, false, sources.RPCKindBasic))
	require.NoError(t, err)

	altDACfg, err := sd.RollupCfg.GetOPAltDAConfig()
	require.NoError(t, err)

	daMgr := altda.NewAltDAWithStorage(log, altDACfg, storage, &altda.NoopMetrics{})

	sequencer := helpers.NewL2Sequencer(t, log, l1F, miner.BlobStore(), daMgr, engCl, sd.RollupCfg, 0, nil)
	miner.ActL1SetFeeRecipient(common.Address{'A'})
	sequencer.ActL2PipelineFull(t)

	batcher := helpers.NewL2Batcher(log, sd.RollupCfg, helpers.AltDABatcherCfg(dp, storage), sequencer.RollupClient(), l1Client, engine.EthClient(), engCl)

	addresses := e2eutils.CollectAddresses(sd, dp)
	cl := engine.EthClient()
	l2UserEnv := &helpers.BasicUserEnv[*helpers.L2Bindings]{
		EthCl:          cl,
		Signer:         types.LatestSigner(sd.L2Cfg.Config),
		AddressCorpora: addresses,
		Bindings:       helpers.NewL2Bindings(t, cl, engine.GethClient()),
	}
	alice := helpers.NewCrossLayerUser(log, dp.Secrets.Alice, rand.New(rand.NewSource(0xa57b)), p.AllocType)
	alice.L2.SetUserEnv(l2UserEnv)

	contract, err := bindings.NewDataAvailabilityChallenge(sd.RollupCfg.AltDAConfig.DAChallengeAddress, l1Client)
	require.NoError(t, err)

	challengeWindow, err := contract.ChallengeWindow(nil)
	require.NoError(t, err)
	require.Equal(t, altDACfg.ChallengeWindow, challengeWindow.Uint64())

	resolveWindow, err := contract.ResolveWindow(nil)
	require.NoError(t, err)
	require.Equal(t, altDACfg.ResolveWindow, resolveWindow.Uint64())

	return &L2AltDA{
		log:       log,
		storage:   storage,
		daMgr:     daMgr,
		altDACfg:  altDACfg,
		contract:  contract,
		batcher:   batcher,
		sequencer: sequencer,
		engine:    engine,
		engCl:     engCl,
		sd:        sd,
		dp:        dp,
		miner:     miner,
		alice:     alice,
	}
}

func (a *L2AltDA) StorageClient() *altda.DAErrFaker {
	return a.storage
}

func (a *L2AltDA) NewVerifier(t helpers.Testing) *helpers.L2Verifier {
	jwtPath := e2eutils.WriteDefaultJWT(t)
	engine := helpers.NewL2Engine(t, a.log, a.sd.L2Cfg, jwtPath)
	engCl := engine.EngineClient(t, a.sd.RollupCfg)
	l1F, err := sources.NewL1Client(a.miner.RPCClient(), a.log, nil, sources.L1ClientDefaultConfig(a.sd.RollupCfg, false, sources.RPCKindBasic))
	require.NoError(t, err)

	daMgr := altda.NewAltDAWithStorage(a.log, a.altDACfg, a.storage, &altda.NoopMetrics{})

	verifier := helpers.NewL2Verifier(t, a.log, l1F, a.miner.BlobStore(), daMgr, engCl, a.sd.RollupCfg, &sync.Config{}, safedb.Disabled, nil)

	return verifier
}

func (a *L2AltDA) ActSequencerIncludeTx(t helpers.Testing) {
	a.alice.L2.ActResetTxOpts(t)
	a.alice.L2.ActSetTxToAddr(&a.dp.Addresses.Bob)(t)
	a.alice.L2.ActMakeTx(t)

	a.sequencer.ActL2PipelineFull(t)

	a.sequencer.ActL2StartBlock(t)
	a.engine.ActL2IncludeTx(a.alice.Address())(t)
	a.sequencer.ActL2EndBlock(t)
}

func (a *L2AltDA) ActNewL2Tx(t helpers.Testing) {
	a.ActSequencerIncludeTx(t)

	a.batcher.ActL2BatchBuffer(t)
	a.batcher.ActL2ChannelClose(t)
	a.batcher.ActL2BatchSubmit(t, func(tx *types.DynamicFeeTx) {
		// skip txdata version byte
		a.lastComm = tx.Data[1:]
	})

	a.miner.ActL1StartBlock(12)(t)
	a.miner.ActL1IncludeTx(a.dp.Addresses.Batcher)(t)
	a.miner.ActL1EndBlock(t)

	a.lastCommBn = a.miner.L1Chain().CurrentBlock().Number.Uint64()
}

func (a *L2AltDA) ActDeleteLastInput(t helpers.Testing) {
	require.NoError(t, a.storage.Client.DeleteData(a.lastComm))
}

func (a *L2AltDA) ActChallengeLastInput(t helpers.Testing) {
	a.ActChallengeInput(t, a.lastComm, a.lastCommBn)

	a.log.Info("challenged last input", "block", a.lastCommBn)
}

func (a *L2AltDA) ActChallengeInput(t helpers.Testing, comm []byte, bn uint64) {
	bondValue, err := a.contract.BondSize(&bind.CallOpts{})
	require.NoError(t, err)

	txOpts, err := bind.NewKeyedTransactorWithChainID(a.dp.Secrets.Alice, a.sd.L1Cfg.Config.ChainID)
	require.NoError(t, err)

	txOpts.Value = bondValue
	_, err = a.contract.Deposit(txOpts)
	require.NoError(t, err)

	a.miner.ActL1StartBlock(12)(t)
	a.miner.ActL1IncludeTx(a.alice.Address())(t)
	a.miner.ActL1EndBlock(t)

	txOpts, err = bind.NewKeyedTransactorWithChainID(a.dp.Secrets.Alice, a.sd.L1Cfg.Config.ChainID)
	require.NoError(t, err)

	_, err = a.contract.Challenge(txOpts, big.NewInt(int64(bn)), comm)
	require.NoError(t, err)

	a.miner.ActL1StartBlock(12)(t)
	a.miner.ActL1IncludeTx(a.alice.Address())(t)
	a.miner.ActL1EndBlock(t)
}

func (a *L2AltDA) ActExpireLastInput(t helpers.Testing) {
	reorgWindow := a.altDACfg.ResolveWindow + a.altDACfg.ChallengeWindow
	for a.miner.L1Chain().CurrentBlock().Number.Uint64() <= a.lastCommBn+reorgWindow {
		a.miner.ActL1StartBlock(12)(t)
		a.miner.ActL1EndBlock(t)
	}
}

func (a *L2AltDA) ActResolveInput(t helpers.Testing, comm []byte, input []byte, bn uint64) {
	txOpts, err := bind.NewKeyedTransactorWithChainID(a.dp.Secrets.Alice, a.sd.L1Cfg.Config.ChainID)
	require.NoError(t, err)

	_, err = a.contract.Resolve(txOpts, big.NewInt(int64(bn)), comm, input)
	require.NoError(t, err)

	a.miner.ActL1StartBlock(12)(t)
	a.miner.ActL1IncludeTx(a.alice.Address())(t)
	a.miner.ActL1EndBlock(t)
}

func (a *L2AltDA) ActResolveLastChallenge(t helpers.Testing) {
	// remove derivation byte prefix
	input, err := a.storage.GetInput(t.Ctx(), altda.Keccak256Commitment(a.lastComm[1:]))
	require.NoError(t, err)

	a.ActResolveInput(t, a.lastComm, input, a.lastCommBn)
}

func (a *L2AltDA) ActL1Blocks(t helpers.Testing, n uint64) {
	for i := uint64(0); i < n; i++ {
		a.miner.ActL1StartBlock(12)(t)
		a.miner.ActL1EndBlock(t)
	}
}

func (a *L2AltDA) GetLastTxBlock(t helpers.Testing) *types.Block {
	rcpt := a.alice.L2.LastTxReceipt(t)
	blk, err := a.engine.EthClient().BlockByHash(t.Ctx(), rcpt.BlockHash)
	require.NoError(t, err)
	return blk
}

func (a *L2AltDA) ActL1Finalized(t helpers.Testing) {
	latest := a.miner.L1Chain().CurrentBlock().Number.Uint64()
	a.miner.ActL1Safe(t, latest)
	a.miner.ActL1Finalize(t, latest)
	a.sequencer.ActL1FinalizedSignal(t)
}

// Commitment is challenged but never resolved, chain reorgs when challenge window expires.
func TestAltDA_ChallengeExpired(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	harness := NewL2AltDA(t)

	// generate enough initial l1 blocks to have a finalized head.
	harness.ActL1Blocks(t, 5)

	// Include a new l2 transaction, submitting an input commitment to the l1.
	harness.ActNewL2Tx(t)

	// Challenge the input commitment on the l1 challenge contract.
	harness.ActChallengeLastInput(t)

	blk := harness.GetLastTxBlock(t)

	// catch up the sequencer derivation pipeline with the new l1 blocks.
	harness.sequencer.ActL2PipelineFull(t)

	// create enough l1 blocks to expire the resolve window.
	harness.ActExpireLastInput(t)

	// catch up the sequencer derivation pipeline with the new l1 blocks.
	harness.sequencer.ActL2PipelineFull(t)

	// the L1 finalized signal should trigger altDA to finalize the engine queue.
	harness.ActL1Finalized(t)

	// move one more block for engine controller to update.
	harness.ActL1Blocks(t, 1)
	harness.sequencer.ActL2PipelineFull(t)

	// get new block with same number to compare
	newBlk, err := harness.engine.EthClient().BlockByNumber(t.Ctx(), blk.Number())
	require.NoError(t, err)

	// reorg happened even though data was available
	require.NotEqual(t, blk.Hash(), newBlk.Hash())

	// now delete the data from the storage service so it is not available at all
	// to the verifier derivation pipeline.
	harness.ActDeleteLastInput(t)

	syncStatus := harness.sequencer.SyncStatus()

	// verifier is able to sync with expired missing data
	verifier := harness.NewVerifier(t)
	verifier.ActL2PipelineFull(t)
	verifier.ActL1FinalizedSignal(t)

	verifSyncStatus := verifier.SyncStatus()

	require.Equal(t, syncStatus.FinalizedL2, verifSyncStatus.FinalizedL2)
}

// Commitment is challenged after sequencer derived the chain but data disappears. A verifier
// derivation pipeline stalls until the challenge is resolved and then resumes with data from the contract.
func TestAltDA_ChallengeResolved(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	harness := NewL2AltDA(t)

	// include a new l2 transaction, submitting an input commitment to the l1.
	harness.ActNewL2Tx(t)

	// generate 3 l1 blocks.
	harness.ActL1Blocks(t, 3)

	// challenge the input commitment for that l2 transaction on the l1 challenge contract.
	harness.ActChallengeLastInput(t)

	// catch up sequencer derivation pipeline.
	// this syncs the latest event within the AltDA manager.
	harness.sequencer.ActL2PipelineFull(t)

	// resolve the challenge on the l1 challenge contract.
	harness.ActResolveLastChallenge(t)

	// catch up the sequencer derivation pipeline with the new l1 blocks.
	// this syncs the resolved status and input data within the AltDA manager.
	harness.sequencer.ActL2PipelineFull(t)

	// finalize l1
	harness.ActL1Finalized(t)

	// delete the data from the storage service so it is not available at all
	// to the verifier derivation pipeline.
	harness.ActDeleteLastInput(t)

	syncStatus := harness.sequencer.SyncStatus()

	// new verifier is able to sync and resolve the input from calldata
	verifier := harness.NewVerifier(t)
	verifier.ActL2PipelineFull(t)
	verifier.ActL1FinalizedSignal(t)

	verifSyncStatus := verifier.SyncStatus()

	require.Equal(t, syncStatus.SafeL2, verifSyncStatus.SafeL2)
}

// DA storage service goes offline while sequencer keeps making blocks. When storage comes back online, it should be able to catch up.
func TestAltDA_StorageError(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	harness := NewL2AltDA(t)

	// include a new l2 transaction, submitting an input commitment to the l1.
	harness.ActNewL2Tx(t)

	txBlk := harness.GetLastTxBlock(t)

	// mock a storage client error when trying to get the pre-image.
	// this simulates the storage service going offline for example.
	harness.storage.ActGetPreImageFail()

	// try to derive the l2 chain from the submitted inputs commitments.
	// the storage call will fail the first time then succeed.
	harness.sequencer.ActL2PipelineFull(t)

	// sequencer derivation was able to sync to latest l1 origin
	syncStatus := harness.sequencer.SyncStatus()
	require.Equal(t, uint64(1), syncStatus.SafeL2.Number)
	require.Equal(t, txBlk.Hash(), syncStatus.SafeL2.Hash)
}

// L1 chain reorgs a resolved challenge so it expires instead causing
// the l2 chain to reorg as well.
func TestAltDA_ChallengeReorg(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	harness := NewL2AltDA(t)

	// New L2 tx added to a batch and committed to L1
	harness.ActNewL2Tx(t)

	// add a buffer of L1 blocks
	harness.ActL1Blocks(t, 3)

	// challenge the input commitment
	harness.ActChallengeLastInput(t)

	// keep track of the block where the L2 tx was included
	blk := harness.GetLastTxBlock(t)

	// progress derivation pipeline
	harness.sequencer.ActL2PipelineFull(t)

	// resolve the challenge so pipeline can progress
	harness.ActResolveLastChallenge(t)

	// derivation marks the challenge as resolve, chain is not impacted
	harness.sequencer.ActL2PipelineFull(t)

	// Rewind the L1, essentially reorging the challenge resolution
	harness.miner.ActL1RewindToParent(t)

	// Now the L1 chain advances without the challenge resolution
	// so the challenge is expired.
	harness.ActExpireLastInput(t)

	// derivation pipeline reorgs the commitment out of the chain
	harness.sequencer.ActL2PipelineFull(t)

	newBlk, err := harness.engine.EthClient().BlockByNumber(t.Ctx(), blk.Number())
	require.NoError(t, err)

	// confirm the reorg did happen
	require.NotEqual(t, blk.Hash(), newBlk.Hash())
}

// Sequencer stalls as data is not available, batcher keeps posting, untracked commitments are
// challenged and resolved, then sequencer resumes and catches up.
func TestAltDA_SequencerStalledMultiChallenges(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	a := NewL2AltDA(t)

	// create a new tx on l2 and commit it to l1
	a.ActNewL2Tx(t)

	// keep track of the related commitment
	comm1 := a.lastComm
	input1, err := a.storage.GetInput(t.Ctx(), altda.Keccak256Commitment(comm1[1:]))
	bn1 := a.lastCommBn
	require.NoError(t, err)

	// delete it from the DA provider so the pipeline cannot verify it
	a.ActDeleteLastInput(t)

	// build more empty l2 unsafe blocks as the l1 origin progresses
	a.ActL1Blocks(t, 10)
	a.sequencer.ActBuildToL1HeadUnsafe(t)

	// build another L2 block without advancing derivation
	a.alice.L2.ActResetTxOpts(t)
	a.alice.L2.ActSetTxToAddr(&a.dp.Addresses.Bob)(t)
	a.alice.L2.ActMakeTx(t)

	a.sequencer.ActL2StartBlock(t)
	a.engine.ActL2IncludeTx(a.alice.Address())(t)
	a.sequencer.ActL2EndBlock(t)

	a.batcher.ActL2BatchBuffer(t)
	a.batcher.ActL2ChannelClose(t)
	a.batcher.ActL2BatchSubmit(t, func(tx *types.DynamicFeeTx) {
		a.lastComm = tx.Data[1:]
	})

	// include it in L1
	a.miner.ActL1StartBlock(12)(t)
	a.miner.ActL1IncludeTx(a.dp.Addresses.Batcher)(t)
	a.miner.ActL1EndBlock(t)

	a.sequencer.ActL1HeadSignal(t)

	unsafe := a.sequencer.L2Unsafe()
	unsafeBlk, err := a.engine.EthClient().BlockByHash(t.Ctx(), unsafe.Hash)
	require.NoError(t, err)

	// advance the pipeline until it errors out as it is still stuck
	// on deriving the first commitment
	a.sequencer.ActL2EventsUntil(t, func(ev event.Event) bool {
		x, ok := ev.(rollup.EngineTemporaryErrorEvent)
		if ok {
			require.ErrorContains(t, x.Err, "failed to fetch input data")
		}
		return ok
	}, 100, false)

	// keep track of the second commitment
	comm2 := a.lastComm
	_, err = a.storage.GetInput(t.Ctx(), altda.Keccak256Commitment(comm2[1:]))
	require.NoError(t, err)
	a.lastCommBn = a.miner.L1Chain().CurrentBlock().Number.Uint64()

	// ensure the second commitment is distinct from the first
	require.NotEqual(t, comm1, comm2)

	// challenge the last commitment while the pipeline is stuck on the first
	a.ActChallengeLastInput(t)

	// resolve the latest commitment before the first one is event challenged.
	a.ActResolveLastChallenge(t)

	// now we delete it to force the pipeline to resolve the second commitment
	// from the challenge data.
	a.ActDeleteLastInput(t)

	// finally challenge the first commitment
	a.ActChallengeInput(t, comm1, bn1)

	// resolve it immediately so we can resume derivation
	a.ActResolveInput(t, comm1, input1, bn1)

	// pipeline can go on
	a.sequencer.ActL2PipelineFull(t)

	// verify that the chain did not reorg out
	safeBlk, err := a.engine.EthClient().BlockByNumber(t.Ctx(), unsafeBlk.Number())
	require.NoError(t, err)
	require.Equal(t, unsafeBlk.Hash(), safeBlk.Hash())
}

// Verify that finalization happens based on altDA windows.
// based on l2_batcher_test.go L2Finalization
func TestAltDA_Finalization(gt *testing.T) {
	t := helpers.NewDefaultTesting(gt)
	a := NewL2AltDA(t)

	// build L1 block #1
	a.ActL1Blocks(t, 1)
	a.miner.ActL1SafeNext(t)

	// Fill with l2 blocks up to the L1 head
	a.sequencer.ActL1HeadSignal(t)
	a.sequencer.ActBuildToL1Head(t)

	a.sequencer.ActL2PipelineFull(t)
	a.sequencer.ActL1SafeSignal(t)
	require.Equal(t, uint64(1), a.sequencer.SyncStatus().SafeL1.Number)

	// add L1 block #2
	a.ActL1Blocks(t, 1)
	a.miner.ActL1SafeNext(t)
	a.miner.ActL1FinalizeNext(t)
	a.sequencer.ActL1HeadSignal(t)
	a.sequencer.ActBuildToL1Head(t)

	// Catch up derivation
	a.sequencer.ActL2PipelineFull(t)
	a.sequencer.ActL1FinalizedSignal(t)
	a.sequencer.ActL1SafeSignal(t)

	// commit all the l2 blocks to L1
	a.batcher.ActSubmitAll(t)
	a.miner.ActL1StartBlock(12)(t)
	a.miner.ActL1IncludeTx(a.dp.Addresses.Batcher)(t)
	a.miner.ActL1EndBlock(t)

	// verify
	a.sequencer.ActL2PipelineFull(t)

	// fill with more unsafe L2 blocks
	a.sequencer.ActL1HeadSignal(t)
	a.sequencer.ActBuildToL1Head(t)

	// submit those blocks too, block #4
	a.batcher.ActSubmitAll(t)
	a.miner.ActL1StartBlock(12)(t)
	a.miner.ActL1IncludeTx(a.dp.Addresses.Batcher)(t)
	a.miner.ActL1EndBlock(t)

	// add some more L1 blocks #5, #6
	a.miner.ActEmptyBlock(t)
	a.miner.ActEmptyBlock(t)

	// and more unsafe L2 blocks
	a.sequencer.ActL1HeadSignal(t)
	a.sequencer.ActBuildToL1Head(t)

	// move safe/finalize markers: finalize the L1 chain block with the first batch, but not the second
	a.miner.ActL1SafeNext(t)     // #2 -> #3
	a.miner.ActL1SafeNext(t)     // #3 -> #4
	a.miner.ActL1FinalizeNext(t) // #1 -> #2
	a.miner.ActL1FinalizeNext(t) // #2 -> #3

	// L1 safe and finalized as expected
	a.sequencer.ActL2PipelineFull(t)
	a.sequencer.ActL1FinalizedSignal(t)
	a.sequencer.ActL1SafeSignal(t)
	a.sequencer.ActL1HeadSignal(t)
	require.Equal(t, uint64(6), a.sequencer.SyncStatus().HeadL1.Number)
	require.Equal(t, uint64(4), a.sequencer.SyncStatus().SafeL1.Number)
	require.Equal(t, uint64(3), a.sequencer.SyncStatus().FinalizedL1.Number)
	// l2 cannot finalize yet as the challenge window is not passed
	require.Equal(t, uint64(0), a.sequencer.SyncStatus().FinalizedL2.Number)

	// expire the challenge window so these blocks can no longer be challenged
	a.ActL1Blocks(t, a.altDACfg.ChallengeWindow)

	// advance derivation and finalize altDA via the L1 signal
	a.sequencer.ActL2PipelineFull(t)
	a.ActL1Finalized(t)
	a.sequencer.ActL2PipelineFull(t) // finality event needs to be processed

	// given 12s l1 time and 1s l2 time, l2 should be 12 * 3 = 36 blocks finalized
	require.Equal(t, uint64(36), a.sequencer.SyncStatus().FinalizedL2.Number)
}

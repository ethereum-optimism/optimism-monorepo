package altda

import (
	"context"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func RandomData(rng *rand.Rand, size int) []byte {
	out := make([]byte, size)
	rng.Read(out)
	return out
}

func RandomCommitment(rng *rand.Rand) CommitmentData {
	return NewKeccak256Commitment(RandomData(rng, 32))
}

func l1Ref(n uint64) eth.L1BlockRef {
	return eth.L1BlockRef{Number: n}
}

func bID(n uint64) eth.BlockID {
	return eth.BlockID{Number: n}
}

// TestFinalization checks that the finalized L1 block ref is returned correctly when pruning with and without challenges
func TestFinalization(t *testing.T) {
	logger := testlog.Logger(t, log.LevelInfo)
	cfg := Config{
		ResolveWindow:   6,
		ChallengeWindow: 6,
	}
	rng := rand.New(rand.NewSource(1234))
	state := NewState(logger, &NoopMetrics{}, cfg)

	c1 := RandomCommitment(rng)
	bn1 := uint64(2)

	// Track a commitment without a challenge
	state.TrackCommitment(c1, l1Ref(bn1))
	require.NoError(t, state.ExpireCommitments(bID(7)))
	require.Empty(t, state.expiredCommitments)
	require.NoError(t, state.ExpireCommitments(bID(8)))
	require.Empty(t, state.commitments)

	state.Prune(bID(bn1))
	require.Equal(t, eth.L1BlockRef{}, state.lastPrunedCommitment)
	state.Prune(bID(7))
	require.Equal(t, eth.L1BlockRef{}, state.lastPrunedCommitment)
	state.Prune(bID(8))
	require.Equal(t, l1Ref(bn1), state.lastPrunedCommitment)

	// Track a commitment, challenge it, & then resolve it
	c2 := RandomCommitment(rng)
	bn2 := uint64(20)
	state.TrackCommitment(c2, l1Ref(bn2))
	require.Equal(t, ChallengeUninitialized, state.GetChallengeStatus(c2, bn2))
	state.CreateChallenge(c2, bID(24), bn2)
	require.Equal(t, ChallengeActive, state.GetChallengeStatus(c2, bn2))
	require.NoError(t, state.ResolveChallenge(c2, bID(30), bn2, nil))
	require.Equal(t, ChallengeResolved, state.GetChallengeStatus(c2, bn2))

	// Expire Challenges & Comms after challenge period but before resolve end & assert they are not expired yet
	require.NoError(t, state.ExpireCommitments(bID(28)))
	require.Empty(t, state.expiredCommitments)
	state.ExpireChallenges(bID(28))
	require.Empty(t, state.expiredChallenges)

	// Now fully expire them
	require.NoError(t, state.ExpireCommitments(bID(30)))
	require.Empty(t, state.commitments)
	state.ExpireChallenges(bID(30))
	require.Empty(t, state.challenges)

	// Now finalize everything
	state.Prune(bID(20))
	require.Equal(t, l1Ref(bn1), state.lastPrunedCommitment)
	state.Prune(bID(28))
	require.Equal(t, l1Ref(bn1), state.lastPrunedCommitment)
	state.Prune(bID(32))
	require.Equal(t, l1Ref(bn2), state.lastPrunedCommitment)
}

// TestExpireChallenges expires challenges and prunes the state for longer windows
// with commitments every 6 blocks.
func TestExpireChallenges(t *testing.T) {
	logger := testlog.Logger(t, log.LevelInfo)

	cfg := Config{
		ResolveWindow:   90,
		ChallengeWindow: 90,
	}

	rng := rand.New(rand.NewSource(1234))
	state := NewState(logger, &NoopMetrics{}, cfg)

	comms := make(map[uint64]CommitmentData)

	i := uint64(3713854)

	// increment new commitments every 6 blocks
	for ; i < 3713948; i += 6 {
		comm := RandomCommitment(rng)
		comms[i] = comm
		logger.Info("set commitment", "block", i, "comm", comm)
		state.TrackCommitment(comm, l1Ref(i))

		require.NoError(t, state.ExpireCommitments(bID(i)))
		state.ExpireChallenges(bID(i))
	}

	// activate a couple of subsequent challenges
	state.CreateChallenge(comms[3713926], bID(3713948), 3713926)
	state.CreateChallenge(comms[3713932], bID(3713950), 3713932)

	// continue incrementing commitments
	for ; i < 3714038; i += 6 {
		comm := RandomCommitment(rng)
		comms[i] = comm
		logger.Info("set commitment", "block", i)
		state.TrackCommitment(comm, l1Ref(i))

		require.NoError(t, state.ExpireCommitments(bID(i)))
		state.ExpireChallenges(bID(i))
	}

	// Jump ahead to the end of the resolve window for comm included in block 3713926 which triggers a reorg
	state.ExpireChallenges(bID(3714106))
	require.ErrorIs(t, state.ExpireCommitments(bID(3714106)), ErrReorgRequired)
}

// TestDAChallengeDetached tests the lookahead + reorg handling of the da state
func TestDAChallengeDetached(t *testing.T) {
	logger := testlog.Logger(t, log.LevelWarn)

	cfg := Config{
		ResolveWindow:   6,
		ChallengeWindow: 6,
	}

	rng := rand.New(rand.NewSource(1234))
	state := NewState(logger, &NoopMetrics{}, cfg)

	c1 := RandomCommitment(rng)
	c2 := RandomCommitment(rng)

	// c1 at bn1 is missing, pipeline stalls
	state.TrackCommitment(c1, l1Ref(1))

	// c2 at bn2 is challenged at bn3
	state.CreateChallenge(c2, bID(3), uint64(2))
	require.Equal(t, ChallengeActive, state.GetChallengeStatus(c2, uint64(2)))

	// c1 is finally challenged at bn5
	state.CreateChallenge(c1, bID(5), uint64(1))

	// c2 expires but should not trigger a reset because we're waiting for c1 to expire
	state.ExpireChallenges(bID(10))
	err := state.ExpireCommitments(bID(10))
	require.NoError(t, err)

	// c1 expires finally
	state.ExpireChallenges(bID(11))
	err = state.ExpireCommitments(bID(11))
	require.ErrorIs(t, err, ErrReorgRequired)

	// pruning finalized block is safe. It should not prune any commitments yet.
	state.Prune(bID(1))
	require.Equal(t, eth.L1BlockRef{}, state.lastPrunedCommitment)

	// Perform reorg back to bn2
	state.ClearCommitments()

	// pipeline discovers c2 at bn2
	state.TrackCommitment(c2, l1Ref(2))
	// it is already marked as expired so it will be skipped without needing a reorg
	require.Equal(t, ChallengeExpired, state.GetChallengeStatus(c2, uint64(2)))

	// later when we get to finalizing block 10 + margin, the pending challenge is safely pruned
	// Note: We need to go through the expire then prune steps
	state.ExpireChallenges(bID(201))
	err = state.ExpireCommitments(bID(201))
	require.ErrorIs(t, err, ErrReorgRequired)
	state.Prune(bID(201))
	require.True(t, state.NoCommitments())
}

// cannot import from testutils at this time because of import cycle
type mockL1Fetcher struct {
	mock.Mock
}

func (m *mockL1Fetcher) InfoAndTxsByHash(ctx context.Context, hash common.Hash) (eth.BlockInfo, types.Transactions, error) {
	out := m.Mock.Called(hash)
	return out.Get(0).(eth.BlockInfo), out.Get(1).(types.Transactions), out.Error(2)
}

func (m *mockL1Fetcher) ExpectInfoAndTxsByHash(hash common.Hash, info eth.BlockInfo, transactions types.Transactions, err error) {
	m.Mock.On("InfoAndTxsByHash", hash).Once().Return(info, transactions, err)
}

func (m *mockL1Fetcher) FetchReceipts(ctx context.Context, blockHash common.Hash) (eth.BlockInfo, types.Receipts, error) {
	out := m.Mock.Called(blockHash)
	return *out.Get(0).(*eth.BlockInfo), out.Get(1).(types.Receipts), out.Error(2)
}

func (m *mockL1Fetcher) ExpectFetchReceipts(hash common.Hash, info eth.BlockInfo, receipts types.Receipts, err error) {
	m.Mock.On("FetchReceipts", hash).Once().Return(&info, receipts, err)
}

func (m *mockL1Fetcher) L1BlockRefByNumber(ctx context.Context, num uint64) (eth.L1BlockRef, error) {
	out := m.Mock.Called(num)
	return out.Get(0).(eth.L1BlockRef), out.Error(1)
}

func (m *mockL1Fetcher) ExpectL1BlockRefByNumber(num uint64, ref eth.L1BlockRef, err error) {
	m.Mock.On("L1BlockRefByNumber", num).Once().Return(ref, err)
}

func TestAdvanceChallengeOrigin(t *testing.T) {
	logger := testlog.Logger(t, log.LevelWarn)
	ctx := context.Background()

	l1F := &mockL1Fetcher{}
	defer l1F.AssertExpectations(t)

	storage := NewMockDAClient(logger)

	daddr := common.HexToAddress("0x978e3286eb805934215a88694d80b09aded68d90")
	pcfg := Config{
		ChallengeWindow: 90, ResolveWindow: 90, DAChallengeContractAddress: daddr,
	}

	bhash := common.HexToHash("0xd438144ffab918b1349e7cd06889c26800c26d8edc34d64f750e3e097166a09c")
	bhash2 := common.HexToHash("0xd000004ffab918b1349e7cd06889c26800c26d8edc34d64f750e3e097166a09c")
	bn := uint64(19)
	comm := Keccak256Commitment(common.FromHex("eed82c1026bdd0f23461dd6ca515ef677624e63e6fc0ff91e3672af8eddf579d"))

	state := NewState(logger, &NoopMetrics{}, pcfg)

	da := NewAltDAWithState(logger, pcfg, storage, &NoopMetrics{}, state)

	receipts := types.Receipts{&types.Receipt{
		Type:   2,
		Status: 1,
		Logs: []*types.Log{
			{
				BlockNumber: bn,
				Address:     daddr,
				Topics: []common.Hash{
					common.HexToHash("0xa448afda7ea1e3a7a10fcab0c29fe9a9dd85791503bf0171f281521551c7ec05"),
				},
			},
			{
				BlockNumber: bn,
				Address:     daddr,
				Topics: []common.Hash{
					common.HexToHash("0xc5d8c630ba2fdacb1db24c4599df78c7fb8cf97b5aecde34939597f6697bb1ad"),
					common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000000e"),
				},
				Data: common.FromHex("0x00000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000002100eed82c1026bdd0f23461dd6ca515ef677624e63e6fc0ff91e3672af8eddf579d00000000000000000000000000000000000000000000000000000000000000"),
			},
		},
		BlockNumber: big.NewInt(int64(bn)),
	}}
	id := eth.BlockID{
		Number: bn,
		Hash:   bhash,
	}
	l1F.ExpectFetchReceipts(bhash, nil, receipts, nil)

	// Advance the challenge origin & ensure that we track the challenge
	err := da.AdvanceChallengeOrigin(ctx, l1F, id)
	require.NoError(t, err)

	c, has := state.GetChallenge(comm, 14)
	require.True(t, has)
	require.Equal(t, ChallengeActive, c.challengeStatus)

	// Advance the challenge origin until the challenge should be expired
	for i := bn + 1; i < bn+1+pcfg.ChallengeWindow; i++ {
		id2 := eth.BlockID{
			Number: i,
			Hash:   bhash2,
		}
		l1F.ExpectFetchReceipts(bhash2, nil, nil, nil)
		err = da.AdvanceChallengeOrigin(ctx, l1F, id2)
		require.NoError(t, err)
	}
	state.Prune(bID(bn + 1 + pcfg.ChallengeWindow + pcfg.ResolveWindow))

	_, has = state.GetChallenge(comm, 14)
	require.False(t, has)
}

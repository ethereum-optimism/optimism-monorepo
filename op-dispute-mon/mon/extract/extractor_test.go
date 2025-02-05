package extract

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	monTypes "github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/stretchr/testify/require"

	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

var (
	mockRootClaim = common.HexToHash("0x1234")
	ignoredGames  = []common.Address{common.HexToAddress("0xdeadbeef")}
)

func TestExtractor_Extract(t *testing.T) {
	t.Run("FetchGamesError", func(t *testing.T) {
		extractor, _, games, _, _ := setupExtractorTest(t)
		games.err = errors.New("boom")
		_, _, _, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.ErrorIs(t, err, games.err)
		require.Equal(t, 1, games.calls)
	})

	t.Run("CreateGameErrorLog", func(t *testing.T) {
		extractor, creator, games, logs, _ := setupExtractorTest(t)
		games.games = []gameTypes.GameMetadata{{}}
		creator.err = errors.New("boom")
		enriched, ignored, failed, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		require.Equal(t, 1, failed)
		require.Zero(t, ignored)
		require.Len(t, enriched, 0)
		require.Equal(t, 1, games.calls)
		require.Equal(t, 1, creator.calls)
		require.Equal(t, 0, creator.caller.metadataCalls)
		require.Equal(t, 0, creator.caller.claimsCalls)
		verifyLogs(t, logs, 1, 0, 0, 0)
	})

	t.Run("MetadataFetchErrorLog", func(t *testing.T) {
		extractor, creator, games, logs, _ := setupExtractorTest(t)
		games.games = []gameTypes.GameMetadata{{}}
		creator.caller.metadataErr = errors.New("boom")
		enriched, ignored, failed, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		require.Zero(t, ignored)
		require.Equal(t, 1, failed)
		require.Len(t, enriched, 0)
		require.Equal(t, 1, games.calls)
		require.Equal(t, 1, creator.calls)
		require.Equal(t, 1, creator.caller.metadataCalls)
		require.Equal(t, 0, creator.caller.claimsCalls)
		verifyLogs(t, logs, 0, 1, 0, 0)
	})

	t.Run("ClaimsFetchErrorLog", func(t *testing.T) {
		extractor, creator, games, logs, _ := setupExtractorTest(t)
		games.games = []gameTypes.GameMetadata{{}}
		creator.caller.claimsErr = errors.New("boom")
		enriched, ignored, failed, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		require.Zero(t, ignored)
		require.Equal(t, 1, failed)
		require.Len(t, enriched, 0)
		require.Equal(t, 1, games.calls)
		require.Equal(t, 1, creator.calls)
		require.Equal(t, 1, creator.caller.metadataCalls)
		require.Equal(t, 1, creator.caller.claimsCalls)
		verifyLogs(t, logs, 0, 0, 1, 0)
	})

	t.Run("Success", func(t *testing.T) {
		extractor, creator, games, _, _ := setupExtractorTest(t)
		games.games = []gameTypes.GameMetadata{{}}
		enriched, ignored, failed, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		require.Zero(t, ignored)
		require.Zero(t, failed)
		require.Len(t, enriched, 1)
		require.Equal(t, 1, games.calls)
		require.Equal(t, 1, creator.calls)
		require.Equal(t, 1, creator.caller.metadataCalls)
		require.Equal(t, 1, creator.caller.claimsCalls)
	})

	t.Run("EnricherFails", func(t *testing.T) {
		enricher := &mockEnricher{err: errors.New("whoops")}
		extractor, _, games, logs, _ := setupExtractorTest(t, enricher)
		games.games = []gameTypes.GameMetadata{{}}
		enriched, ignored, failed, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		require.Zero(t, ignored)
		require.Equal(t, 1, failed)
		l := logs.FindLogs(testlog.NewAttributesContainsFilter("err", "failed to enrich game"))
		require.Len(t, l, 1, "Should have logged error")
		require.Len(t, enriched, 0, "Should not return games that failed to enrich")
	})

	t.Run("EnricherSuccess", func(t *testing.T) {
		enricher := &mockEnricher{}
		extractor, _, games, _, _ := setupExtractorTest(t, enricher)
		games.games = []gameTypes.GameMetadata{{}}
		enriched, ignored, failed, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		require.Zero(t, ignored)
		require.Zero(t, failed)
		require.Len(t, enriched, 1)
		require.Equal(t, 1, enricher.calls)
	})

	t.Run("MultipleEnrichersMultipleGames", func(t *testing.T) {
		enricher1 := &mockEnricher{}
		enricher2 := &mockEnricher{}
		extractor, _, games, _, _ := setupExtractorTest(t, enricher1, enricher2)
		games.games = []gameTypes.GameMetadata{{Proxy: common.Address{0xaa}}, {Proxy: common.Address{0xbb}}}
		enriched, ignored, failed, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		require.Zero(t, ignored)
		require.Zero(t, failed)
		require.Len(t, enriched, 2)
		require.Equal(t, 2, enricher1.calls)
		require.Equal(t, 2, enricher2.calls)
	})

	t.Run("IgnoreGames", func(t *testing.T) {
		enricher1 := &mockEnricher{}
		extractor, _, games, logs, _ := setupExtractorTest(t, enricher1)
		// Two games, one of which is ignored
		games.games = []gameTypes.GameMetadata{{Proxy: ignoredGames[0]}, {Proxy: common.Address{0xaa}}}
		enriched, ignored, failed, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		// Should ignore one and enrich the other
		require.Equal(t, 1, ignored)
		require.Zero(t, failed)
		require.Len(t, enriched, 1)
		require.Equal(t, 1, enricher1.calls)
		require.Equal(t, enriched[0].Proxy, common.Address{0xaa})
		require.NotNil(t, logs.FindLog(
			testlog.NewLevelFilter(log.LevelWarn),
			testlog.NewMessageFilter("Ignoring game"),
			testlog.NewAttributesFilter("game", ignoredGames[0].Hex())))
	})

	t.Run("UseCachedValueOnFailure", func(t *testing.T) {
		enricher := &mockEnricher{
			action: func(game *monTypes.EnrichedGameData) error {
				game.Status = gameTypes.GameStatusDefenderWon
				return nil
			},
		}
		extractor, _, games, _, cl := setupExtractorTest(t, enricher)
		gameA := common.Address{0xaa}
		gameB := common.Address{0xbb}
		games.games = []gameTypes.GameMetadata{{Proxy: gameA}, {Proxy: gameB}}

		// First fetch succeeds and the results should be cached
		enriched, ignored, failed, err := extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		require.Zero(t, ignored)
		require.Zero(t, failed)
		require.Len(t, enriched, 2)
		require.Equal(t, 2, enricher.calls)
		firstUpdateTime := cl.Now()
		// All results should have current LastUpdateTime
		for _, data := range enriched {
			require.Equal(t, firstUpdateTime, data.LastUpdateTime)
		}

		cl.AdvanceTime(2 * time.Minute)
		secondUpdateTime := cl.Now()
		enricher.action = func(game *monTypes.EnrichedGameData) error {
			if game.Proxy == gameA {
				return errors.New("boom")
			}
			// Updated games will have a different status
			game.Status = gameTypes.GameStatusChallengerWon
			return nil
		}
		// Second fetch fails for one of the two games, it's cached value should be used.
		enriched, ignored, failed, err = extractor.Extract(context.Background(), common.Hash{}, 0)
		require.NoError(t, err)
		require.Zero(t, ignored)
		require.Equal(t, 1, failed)
		require.Len(t, enriched, 2)
		require.Equal(t, 4, enricher.calls)
		// The returned games are not in a fixed order, create a map to look up the game we need to assert
		actual := make(map[common.Address]*monTypes.EnrichedGameData)
		for _, data := range enriched {
			actual[data.Proxy] = data
		}
		require.Contains(t, actual, gameA)
		require.Contains(t, actual, gameB)
		require.Equal(t, actual[gameA].Status, gameTypes.GameStatusDefenderWon)   // Uses cached value from game A
		require.Equal(t, actual[gameB].Status, gameTypes.GameStatusChallengerWon) // Updates game B
		require.Equal(t, firstUpdateTime, actual[gameA].LastUpdateTime)
		require.Equal(t, secondUpdateTime, actual[gameB].LastUpdateTime)
	})
}

func verifyLogs(t *testing.T, logs *testlog.CapturingHandler, createErr, metadataErr, claimsErr, durationErr int) {
	errorLevelFilter := testlog.NewLevelFilter(log.LevelError)
	createMessageFilter := testlog.NewAttributesContainsFilter("err", "failed to create contracts")
	l := logs.FindLogs(errorLevelFilter, createMessageFilter)
	require.Len(t, l, createErr)
	fetchMessageFilter := testlog.NewAttributesContainsFilter("err", "failed to fetch game metadata")
	l = logs.FindLogs(errorLevelFilter, fetchMessageFilter)
	require.Len(t, l, metadataErr)
	claimsMessageFilter := testlog.NewAttributesContainsFilter("err", "failed to fetch game claims")
	l = logs.FindLogs(errorLevelFilter, claimsMessageFilter)
	require.Len(t, l, claimsErr)
	durationMessageFilter := testlog.NewAttributesContainsFilter("err", "failed to fetch game duration")
	l = logs.FindLogs(errorLevelFilter, durationMessageFilter)
	require.Len(t, l, durationErr)
}

func setupExtractorTest(t *testing.T, enrichers ...Enricher) (*Extractor, *mockGameCallerCreator, *mockGameFetcher, *testlog.CapturingHandler, *clock.DeterministicClock) {
	logger, capturedLogs := testlog.CaptureLogger(t, log.LvlDebug)
	games := &mockGameFetcher{}
	caller := &mockGameCaller{rootClaim: mockRootClaim}
	creator := &mockGameCallerCreator{caller: caller}
	cl := clock.NewDeterministicClock(time.Unix(48294294, 58))
	extractor := NewExtractor(
		logger,
		cl,
		creator.CreateGameCaller,
		games.FetchGames,
		ignoredGames,
		5,
		enrichers...,
	)
	return extractor, creator, games, capturedLogs, cl
}

type mockGameFetcher struct {
	calls int
	err   error
	games []gameTypes.GameMetadata
}

func (m *mockGameFetcher) FetchGames(_ context.Context, _ common.Hash, _ uint64) ([]gameTypes.GameMetadata, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.games, nil
}

type mockGameCallerCreator struct {
	calls  int
	err    error
	caller *mockGameCaller
}

func (m *mockGameCallerCreator) CreateGameCaller(_ context.Context, _ gameTypes.GameMetadata) (GameCaller, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.caller, nil
}

type mockGameCaller struct {
	metadataCalls    int
	metadataErr      error
	claimsCalls      int
	claimsErr        error
	rootClaim        common.Hash
	claims           []faultTypes.Claim
	requestedCredits []common.Address
	creditsErr       error
	credits          map[common.Address]*big.Int
	extraCredit      []*big.Int
	balanceErr       error
	balance          *big.Int
	delayDuration    time.Duration
	balanceAddr      common.Address
	withdrawalsCalls int
	withdrawalsErr   error
	withdrawals      []*contracts.WithdrawalRequest
	resolvedErr      error
	resolved         map[int]bool
}

func (m *mockGameCaller) GetWithdrawals(_ context.Context, _ rpcblock.Block, _ ...common.Address) ([]*contracts.WithdrawalRequest, error) {
	m.withdrawalsCalls++
	if m.withdrawalsErr != nil {
		return nil, m.withdrawalsErr
	}
	if m.withdrawals != nil {
		return m.withdrawals, nil
	}
	return []*contracts.WithdrawalRequest{
		{
			Timestamp: big.NewInt(1),
			Amount:    big.NewInt(2),
		},
		{
			Timestamp: big.NewInt(3),
			Amount:    big.NewInt(4),
		},
	}, nil
}

func (m *mockGameCaller) GetGameMetadata(_ context.Context, _ rpcblock.Block) (contracts.GameMetadata, error) {
	m.metadataCalls++
	if m.metadataErr != nil {
		return contracts.GameMetadata{}, m.metadataErr
	}
	return contracts.GameMetadata{
		L1Head:    common.Hash{0xaa},
		RootClaim: mockRootClaim,
	}, nil
}

func (m *mockGameCaller) GetAllClaims(_ context.Context, _ rpcblock.Block) ([]faultTypes.Claim, error) {
	m.claimsCalls++
	if m.claimsErr != nil {
		return nil, m.claimsErr
	}
	return m.claims, nil
}

func (m *mockGameCaller) GetCredits(_ context.Context, _ rpcblock.Block, recipients ...common.Address) ([]*big.Int, error) {
	m.requestedCredits = recipients
	if m.creditsErr != nil {
		return nil, m.creditsErr
	}
	response := make([]*big.Int, 0, len(recipients))
	for _, recipient := range recipients {
		credit, ok := m.credits[recipient]
		if !ok {
			credit = big.NewInt(0)
		}
		response = append(response, credit)
	}
	response = append(response, m.extraCredit...)
	return response, nil
}

func (m *mockGameCaller) GetBalanceAndDelay(_ context.Context, _ rpcblock.Block) (*big.Int, time.Duration, common.Address, error) {
	if m.balanceErr != nil {
		return nil, 0, common.Address{}, m.balanceErr
	}
	return m.balance, m.delayDuration, m.balanceAddr, nil
}

func (m *mockGameCaller) IsResolved(_ context.Context, _ rpcblock.Block, claims ...faultTypes.Claim) ([]bool, error) {
	if m.resolvedErr != nil {
		return nil, m.resolvedErr
	}
	resolved := make([]bool, len(claims))
	for i, claim := range claims {
		resolved[i] = m.resolved[claim.ContractIndex]
	}
	return resolved, nil
}

type mockEnricher struct {
	err    error
	calls  int
	action func(game *monTypes.EnrichedGameData) error
}

func (m *mockEnricher) Enrich(_ context.Context, _ rpcblock.Block, _ GameCaller, game *monTypes.EnrichedGameData) error {
	m.calls++
	if m.action != nil {
		return m.action(game)
	}
	return m.err
}

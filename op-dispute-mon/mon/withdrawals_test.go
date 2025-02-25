package mon

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	monTypes "github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

var (
	weth1 = common.Address{0x1a}
	weth2 = common.Address{0x2b}

	honestActor1    = common.Address{0x11, 0xaa}
	honestActor2    = common.Address{0x22, 0xbb}
	honestActor3    = common.Address{0x33, 0xcc}
	dishonestActor4 = common.Address{0x44, 0xdd}

	nowUnix = int64(10_000)
)

func makeGames(distributionMode faultTypes.BondDistributionMode, noWithdrawalRequest bool) []*monTypes.EnrichedGameData {
	weth1Balance := big.NewInt(4200)
	weth2Balance := big.NewInt(6000)

	game1 := &monTypes.EnrichedGameData{
		GameMetadata: types.GameMetadata{Proxy: common.Address{0x11, 0x11, 0x11}},
		Credits: map[common.Address]*big.Int{
			honestActor1: big.NewInt(3),
			honestActor2: big.NewInt(1),
		},
		BondDistributionMode: distributionMode,
		WithdrawalRequests: map[common.Address]*contracts.WithdrawalRequest{
			honestActor1: {Amount: big.NewInt(3), Timestamp: big.NewInt(nowUnix - 101)}, // Claimable
			honestActor2: {Amount: big.NewInt(1), Timestamp: big.NewInt(nowUnix - 99)},  // Not claimable
		},
		WETHContract:  weth1,
		ETHCollateral: weth1Balance,
		WETHDelay:     100 * time.Second,
	}
	game2 := &monTypes.EnrichedGameData{
		GameMetadata:         types.GameMetadata{Proxy: common.Address{0x22, 0x22, 0x22}},
		BondDistributionMode: distributionMode,
		Credits: map[common.Address]*big.Int{
			honestActor1: big.NewInt(46),
			honestActor2: big.NewInt(1),
		},
		WithdrawalRequests: map[common.Address]*contracts.WithdrawalRequest{
			honestActor1: {Amount: big.NewInt(3), Timestamp: big.NewInt(nowUnix - 501)}, // Claimable
			honestActor2: {Amount: big.NewInt(1), Timestamp: big.NewInt(nowUnix)},       // Not claimable
		},
		WETHContract:  weth2,
		ETHCollateral: weth2Balance,
		WETHDelay:     500 * time.Second,
	}
	game3 := &monTypes.EnrichedGameData{
		GameMetadata:         types.GameMetadata{Proxy: common.Address{0x33, 0x33, 0x33}},
		BondDistributionMode: distributionMode,
		Credits: map[common.Address]*big.Int{
			honestActor3:    big.NewInt(2),
			dishonestActor4: big.NewInt(4),
		},
		WithdrawalRequests: map[common.Address]*contracts.WithdrawalRequest{
			honestActor3:    {Amount: big.NewInt(2), Timestamp: big.NewInt(nowUnix - 1)}, // Claimable
			dishonestActor4: {Amount: big.NewInt(4), Timestamp: big.NewInt(nowUnix - 5)}, // Claimable
		},
		WETHContract:  weth2,
		ETHCollateral: weth2Balance,
		WETHDelay:     0 * time.Second,
	}
	if noWithdrawalRequest {
		// eth_call will return 0s, not nil, when no withdrawal request is present
		game1.WithdrawalRequests[honestActor2] = &contracts.WithdrawalRequest{
			Amount:    big.NewInt(0),
			Timestamp: big.NewInt(0),
		}
	}
	return []*monTypes.EnrichedGameData{game1, game2, game3}
}

func TestCheckWithdrawals(t *testing.T) {
	tests := []struct {
		name                string
		distributionMode    faultTypes.BondDistributionMode
		noWithdrawalRequest bool
	}{
		{
			name:             "Legacy",
			distributionMode: faultTypes.LegacyDistributionMode,
		},
		{
			name:             "Normal",
			distributionMode: faultTypes.NormalDistributionMode,
		},
		{
			name:                "Normal-NoWithdrawalRequest",
			distributionMode:    faultTypes.NormalDistributionMode,
			noWithdrawalRequest: true,
		},
		{
			name:             "Refund",
			distributionMode: faultTypes.RefundDistributionMode,
		},
		{
			name:                "Refund-NoWithdrawalRequest",
			distributionMode:    faultTypes.RefundDistributionMode,
			noWithdrawalRequest: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			now := time.Unix(nowUnix, 0)
			cl := clock.NewDeterministicClock(now)
			logger, logs := testlog.CaptureLogger(t, log.LvlInfo)
			metrics := &stubWithdrawalsMetrics{
				matching:  make(map[common.Address]int),
				divergent: make(map[common.Address]int),
			}
			honestActors := monTypes.NewHonestActors([]common.Address{honestActor1, honestActor2, honestActor3})
			withdrawals := NewWithdrawalMonitor(logger, cl, metrics, honestActors)
			games := makeGames(test.distributionMode, test.noWithdrawalRequest)
			withdrawals.CheckWithdrawals(games)

			require.Equal(t, metrics.matchCalls, 2)
			require.Equal(t, metrics.divergeCalls, 2)
			require.Len(t, metrics.matching, 2)
			require.Len(t, metrics.divergent, 2)
			require.Contains(t, metrics.matching, weth1)
			require.Contains(t, metrics.matching, weth2)
			require.Contains(t, metrics.divergent, weth1)
			require.Contains(t, metrics.divergent, weth2)
			require.Equal(t, metrics.matching[weth1], 2)
			require.Equal(t, metrics.matching[weth2], 3)
			require.Equal(t, metrics.divergent[weth1], 0)
			require.Equal(t, metrics.divergent[weth2], 1)

			require.Len(t, metrics.honestWithdrawable, 3)
			requireBigInt := func(name string, expected, actual *big.Int) {
				require.Truef(t, expected.Cmp(actual) == 0, "Expected %v withdrawable to be %v but was %v", name, expected, actual)
			}
			requireBigInt("honest addr1", big.NewInt(6), metrics.honestWithdrawable[honestActor1])
			if test.noWithdrawalRequest && test.distributionMode != faultTypes.UndecidedDistributionMode {
				// Withdrawal should have been initiated but hasn't been so the bond is treated as unclaimed
				requireBigInt("honest addr2", big.NewInt(1), metrics.honestWithdrawable[honestActor2])
			} else {
				requireBigInt("honest addr2", big.NewInt(0), metrics.honestWithdrawable[honestActor2])
			}
			requireBigInt("honest addr3", big.NewInt(2), metrics.honestWithdrawable[honestActor3])
			require.Nil(t, metrics.honestWithdrawable[dishonestActor4], "should only report withdrawable credits for honest actors")

			findUnclaimedCreditWarning := func(game common.Address, actor common.Address) *testlog.CapturedRecord {
				return logs.FindLog(
					testlog.NewLevelFilter(log.LevelWarn),
					testlog.NewMessageFilter("Found unclaimed credit"),
					testlog.NewAttributesFilter("game", game.Hex()),
					testlog.NewAttributesFilter("recipient", actor.Hex()))
			}
			requireUnclaimedWarning := func(game common.Address, actor common.Address) {
				require.NotNil(t, findUnclaimedCreditWarning(game, actor))
			}
			noUnclaimedWarning := func(game common.Address, actor common.Address) {
				require.Nil(t, findUnclaimedCreditWarning(game, actor))
			}
			// Game 1, unclaimed for honestActor1 only
			requireUnclaimedWarning(games[0].Proxy, honestActor1)
			noUnclaimedWarning(games[0].Proxy, honestActor2)
			noUnclaimedWarning(games[0].Proxy, honestActor3)
			noUnclaimedWarning(games[0].Proxy, dishonestActor4)

			// Game 2, unclaimed for honestActor1 only
			requireUnclaimedWarning(games[1].Proxy, honestActor1)
			noUnclaimedWarning(games[1].Proxy, honestActor2)
			noUnclaimedWarning(games[1].Proxy, honestActor3)
			noUnclaimedWarning(games[1].Proxy, dishonestActor4)

			// Game 3, unclaimed for honestActor3 only
			// dishonestActor4 has unclaimed credits but we don't track them
			noUnclaimedWarning(games[2].Proxy, honestActor1)
			noUnclaimedWarning(games[2].Proxy, honestActor2)
			requireUnclaimedWarning(games[2].Proxy, honestActor3)
			noUnclaimedWarning(games[2].Proxy, dishonestActor4)
		})
	}
}

func TestWithdrawalNotInitiated(t *testing.T) {
	now := time.Unix(nowUnix, 0)
	cl := clock.NewDeterministicClock(now)
	logger, logs := testlog.CaptureLogger(t, log.LvlInfo)
	metrics := &stubWithdrawalsMetrics{
		matching:  make(map[common.Address]int),
		divergent: make(map[common.Address]int),
	}
	honestActors := monTypes.NewHonestActors([]common.Address{honestActor1, honestActor2, honestActor3})
	withdrawals := NewWithdrawalMonitor(logger, cl, metrics, honestActors)
	games := []*monTypes.EnrichedGameData{
		{
			GameMetadata: types.GameMetadata{Proxy: common.Address{0x11, 0x11, 0x11}},
			Credits: map[common.Address]*big.Int{
				honestActor1: big.NewInt(3),
			},
			BondDistributionMode: faultTypes.NormalDistributionMode,
			WithdrawalRequests: map[common.Address]*contracts.WithdrawalRequest{
				honestActor1: {Amount: big.NewInt(0), Timestamp: big.NewInt(0)},
			},
			WETHContract:  weth1,
			ETHCollateral: big.NewInt(3),
			WETHDelay:     100 * time.Second,
		},
	}
	withdrawals.CheckWithdrawals(games)

	require.NotNil(t, logs.FindLog(testlog.NewMessageFilter("Found uninitiated withdrawal"),
		testlog.NewAttributesFilter("recipient", honestActor1.Hex()),
		testlog.NewAttributesFilter("game", games[0].Proxy.Hex()),
		testlog.NewAttributesFilter("amount", "3")))

	require.Truef(t, big.NewInt(3).Cmp(metrics.honestWithdrawable[honestActor1]) == 0,
		"Expected %v withdrawable to be %v but was %v", honestActor1, 3, metrics.honestWithdrawable[honestActor1])
}

type stubWithdrawalsMetrics struct {
	matchCalls         int
	divergeCalls       int
	matching           map[common.Address]int
	divergent          map[common.Address]int
	honestWithdrawable map[common.Address]*big.Int
}

func (s *stubWithdrawalsMetrics) RecordHonestWithdrawableAmounts(honestWithdrawable map[common.Address]*big.Int) {
	s.honestWithdrawable = honestWithdrawable
}

func (s *stubWithdrawalsMetrics) RecordWithdrawalRequests(addr common.Address, matches bool, count int) {
	if matches {
		s.matchCalls++
		s.matching[addr] = count
	} else {
		s.divergeCalls++
		s.divergent[addr] = count
	}
}

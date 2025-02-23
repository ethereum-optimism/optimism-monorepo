package mon

import (
	"math/big"
	"time"

	challengerTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-dispute-mon/mon/types"
	"github.com/ethereum-optimism/optimism/op-service/bigs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type WithdrawalMetrics interface {
	RecordWithdrawalRequests(delayedWeth common.Address, matches bool, count int)
	RecordHonestWithdrawableAmounts(map[common.Address]*big.Int)
}

type WithdrawalMonitor struct {
	logger       log.Logger
	clock        RClock
	metrics      WithdrawalMetrics
	honestActors types.HonestActors
}

func NewWithdrawalMonitor(logger log.Logger, clock RClock, metrics WithdrawalMetrics, honestActors types.HonestActors) *WithdrawalMonitor {
	return &WithdrawalMonitor{
		logger:       logger,
		clock:        clock,
		metrics:      metrics,
		honestActors: honestActors,
	}
}

func (w *WithdrawalMonitor) CheckWithdrawals(games []*types.EnrichedGameData) {
	now := w.clock.Now() // Use a consistent time for all checks
	matching := make(map[common.Address]int)
	divergent := make(map[common.Address]int)
	honestWithdrawableAmounts := make(map[common.Address]*big.Int)
	for address := range w.honestActors {
		honestWithdrawableAmounts[address] = big.NewInt(0)
	}
	for _, game := range games {
		matches, diverges := w.validateGameWithdrawals(game, now, honestWithdrawableAmounts)
		matching[game.WETHContract] += matches
		divergent[game.WETHContract] += diverges
	}
	for contract, count := range matching {
		w.metrics.RecordWithdrawalRequests(contract, true, count)
	}
	for contract, count := range divergent {
		w.metrics.RecordWithdrawalRequests(contract, false, count)
	}
	w.metrics.RecordHonestWithdrawableAmounts(honestWithdrawableAmounts)
}

func (w *WithdrawalMonitor) validateGameWithdrawals(game *types.EnrichedGameData, now time.Time, honestWithdrawableAmounts map[common.Address]*big.Int) (int, int) {
	matching := 0
	divergent := 0
	for recipient, withdrawalAmount := range game.WithdrawalRequests {
		switch game.BondDistributionMode {
		case challengerTypes.LegacyDistributionMode:
			if bigs.Equal(withdrawalAmount.Amount, game.Credits[recipient]) {
				matching++
			} else {
				divergent++
				w.logger.Error("Withdrawal request amount does not match credit", "game", game.Proxy, "recipient", recipient, "credit", game.Credits[recipient], "withdrawal", game.WithdrawalRequests[recipient].Amount)
			}
		case challengerTypes.UndecidedDistributionMode:
			// DelayedWETH should not have any withdrawal request yet because the bond distribution mode is undecided
			if !bigs.IsZero(withdrawalAmount.Amount) {
				divergent++
				w.logger.Error("Withdrawal request created before bond distribution mode set", "game", game.Proxy, "recipient", recipient, "withdrawal", game.WithdrawalRequests[recipient].Amount)
			}
		case challengerTypes.NormalDistributionMode, challengerTypes.RefundDistributionMode:
			// The withdrawal request is only created on the first claim to claimCredit, so it may not have been set.
			// If it has been set, it should match the game credit amount.
			if bigs.IsZero(withdrawalAmount.Amount) || bigs.Equal(withdrawalAmount.Amount, game.Credits[recipient]) {
				matching++
			} else {
				divergent++
				w.logger.Error("Withdrawal request amount does not match credit", "game", game.Proxy, "recipient", recipient, "credit", game.Credits[recipient], "withdrawal", game.WithdrawalRequests[recipient].Amount)
			}
		default:
			// Treat unknown distribution mode as divergent - better to alert than to ignore.
			divergent++
			w.logger.Error("Unsupported distribution mode", "game", game.Proxy, "recipient", recipient, "mode", game.BondDistributionMode)
		}

		if w.honestActors.Contains(recipient) {
			if game.BondDistributionMode != challengerTypes.UndecidedDistributionMode && bigs.IsZero(withdrawalAmount.Amount) && !bigs.IsZero(game.Credits[recipient]) {
				w.logger.Warn("Found uninitiated withdrawal", "recipient", recipient, "game", game.Proxy, "amount", game.Credits[recipient])
				// Treat credits as withdrawable because the first step of withdrawing can be performed
				total := honestWithdrawableAmounts[recipient]
				total = new(big.Int).Add(total, game.Credits[recipient])
				honestWithdrawableAmounts[recipient] = total
			}
			if bigs.IsPositive(withdrawalAmount.Amount) && time.Unix(withdrawalAmount.Timestamp.Int64(), 0).Add(game.WETHDelay).Before(now) {
				// Credits are fully withdrawable
				total := honestWithdrawableAmounts[recipient]
				total = new(big.Int).Add(total, withdrawalAmount.Amount)
				honestWithdrawableAmounts[recipient] = total
				w.logger.Warn("Found unclaimed credit", "recipient", recipient, "game", game.Proxy, "amount", withdrawalAmount.Amount)
			}
		}
	}
	return matching, divergent
}

package types

import (
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum/go-ethereum/common"
)

// EnrichedClaim extends the faultTypes.Claim with additional context.
type EnrichedClaim struct {
	faultTypes.Claim
	Resolved bool
}

type EnrichedGameData struct {
	types.GameMetadata
	LastUpdateTime        time.Time
	L1Head                common.Hash
	L1HeadNum             uint64
	L2BlockNumber         uint64
	RootClaim             common.Hash
	Status                types.GameStatus
	MaxClockDuration      uint64
	BlockNumberChallenged bool
	BlockNumberChallenger common.Address
	Claims                []EnrichedClaim

	AgreeWithClaim    bool
	ExpectedRootClaim common.Hash

	// Recipients maps addresses to true if they are a bond recipient in the game.
	Recipients map[common.Address]bool

	// Credits records the paid out bonds for the game, keyed by recipient.
	Credits map[common.Address]*big.Int

	// WithdrawalRequests maps recipients with withdrawal requests in DelayedWETH for this game.
	WithdrawalRequests map[common.Address]*contracts.WithdrawalRequest

	// WETHContract is the address of the DelayedWETH contract used by this game
	// The contract is potentially shared by multiple games.
	WETHContract common.Address

	// WETHDelay is the delay applied before credits can be withdrawn.
	WETHDelay time.Duration

	// ETHCollateral is the ETH balance of the (potentially shared) WETHContract
	// This ETH balance will be used to pay out any bonds required by the games
	// that use the same DelayedWETH contract.
	ETHCollateral *big.Int
}

// BidirectionalTree is a tree of claims represented as a flat list of claims.
// This keeps the tree structure identical to how claims are stored in the contract.
type BidirectionalTree struct {
	Claims []*BidirectionalClaim
}

type BidirectionalClaim struct {
	Claim    *faultTypes.Claim
	Children []*BidirectionalClaim
}

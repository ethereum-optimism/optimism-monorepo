package rollup

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-node/params"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
)

// maxChannelBankSize is the amount of memory space, in number of bytes,
// till the bank is pruned by removing channels, starting with the oldest channel.
// It's value is changed with the Fjord network upgrade.
const (
	maxChannelBankSizeBedrock = 100_000_000
	maxChannelBankSizeFjord   = 1_000_000_000
)

// MaxRLPBytesPerChannel is the maximum amount of bytes that will be read from
// a channel. This limit is set when decoding the RLP.
const (
	maxRLPBytesPerChannelBedrock = 10_000_000
	maxRLPBytesPerChannelFjord   = 100_000_000
)

// Fjord changes the max sequencer drift to a protocol constant. It was previously configurable via
// the rollup config.
// From Fjord, the max sequencer drift for a given block timestamp should be learned via the
// ChainSpec instead of reading the rollup configuration field directly.
const maxSequencerDriftFjord = 1800

type ForkName string

const (
	Bedrock  ForkName = "bedrock"
	Regolith ForkName = "regolith"
	Canyon   ForkName = "canyon"
	Delta    ForkName = "delta"
	Ecotone  ForkName = "ecotone"
	Fjord    ForkName = "fjord"
	Granite  ForkName = "granite"
	Holocene ForkName = "holocene"
	Interop  ForkName = "interop"
	// ADD NEW FORKS TO AllForks BELOW!
	None ForkName = "none"
)

var AllForks = []ForkName{
	Bedrock,
	Regolith,
	Canyon,
	Delta,
	Ecotone,
	Fjord,
	Granite,
	Holocene,
	Interop,
	// ADD NEW FORKS HERE!
}

func ForksFrom(fork ForkName) []ForkName {
	for i, f := range AllForks {
		if f == fork {
			return AllForks[i:]
		}
	}
	panic(fmt.Sprintf("invalid fork: %s", fork))
}

var nextFork = func() map[ForkName]ForkName {
	m := make(map[ForkName]ForkName, len(AllForks))
	for i, f := range AllForks {
		if i == len(AllForks)-1 {
			m[f] = None
			break
		}
		m[f] = AllForks[i+1]
	}
	return m
}()

func IsValidFork(fork ForkName) bool {
	_, ok := nextFork[fork]
	return ok
}

type ChainSpec struct {
	config      *Config
	currentFork ForkName
}

func NewChainSpec(config *Config) *ChainSpec {
	return &ChainSpec{config: config}
}

// L2ChainID returns the chain ID of the L2 chain.
func (s *ChainSpec) L2ChainID() *big.Int {
	return s.config.L2ChainID
}

// L2GenesisTime returns the genesis time of the L2 chain.
func (s *ChainSpec) L2GenesisTime() uint64 {
	return s.config.Genesis.L2Time
}

// IsCanyon returns true if t >= canyon_time
func (s *ChainSpec) IsCanyon(t uint64) bool {
	return s.config.IsCanyon(t)
}

// IsHolocene returns true if t >= holocene_time
func (s *ChainSpec) IsHolocene(t uint64) bool {
	return s.config.IsHolocene(t)
}

// MaxChannelBankSize returns the maximum number of bytes the can allocated inside the channel bank
// before pruning occurs at the given timestamp.
func (s *ChainSpec) MaxChannelBankSize(t uint64) uint64 {
	if s.config.IsFjord(t) {
		return maxChannelBankSizeFjord
	}
	return maxChannelBankSizeBedrock
}

// ChannelTimeout returns the channel timeout constant.
func (s *ChainSpec) ChannelTimeout(t uint64) uint64 {
	if s.config.IsGranite(t) {
		return params.ChannelTimeoutGranite
	}
	return s.config.ChannelTimeoutBedrock
}

// MaxRLPBytesPerChannel returns the maximum amount of bytes that will be read from
// a channel at a given timestamp.
func (s *ChainSpec) MaxRLPBytesPerChannel(t uint64) uint64 {
	if s.config.IsFjord(t) {
		return maxRLPBytesPerChannelFjord
	}
	return maxRLPBytesPerChannelBedrock
}

// IsFeatMaxSequencerDriftConstant specifies in which fork the max sequencer drift change to a
// constant will be performed.
func (s *ChainSpec) IsFeatMaxSequencerDriftConstant(t uint64) bool {
	return s.config.IsFjord(t)
}

// MaxSequencerDrift returns the maximum sequencer drift for the given block timestamp. Until Fjord,
// this was a rollup configuration parameter. Since Fjord, it is a constant, so its effective value
// should always be queried via the ChainSpec.
func (s *ChainSpec) MaxSequencerDrift(t uint64) uint64 {
	if s.IsFeatMaxSequencerDriftConstant(t) {
		return maxSequencerDriftFjord
	}
	return s.config.MaxSequencerDrift
}

func (s *ChainSpec) CheckForkActivation(log log.Logger, block eth.L2BlockRef) {
	if s.currentFork == Interop {
		return
	}

	if s.currentFork == "" {
		// Initialize currentFork if it is not set yet
		s.currentFork = Bedrock
		if s.config.IsRegolith(block.Time) {
			s.currentFork = Regolith
		}
		if s.config.IsCanyon(block.Time) {
			s.currentFork = Canyon
		}
		if s.config.IsDelta(block.Time) {
			s.currentFork = Delta
		}
		if s.config.IsEcotone(block.Time) {
			s.currentFork = Ecotone
		}
		if s.config.IsFjord(block.Time) {
			s.currentFork = Fjord
		}
		if s.config.IsGranite(block.Time) {
			s.currentFork = Granite
		}
		if s.config.IsHolocene(block.Time) {
			s.currentFork = Holocene
		}
		if s.config.IsInterop(block.Time) {
			s.currentFork = Interop
		}
		log.Info("Current hardfork version detected", "forkName", s.currentFork)
		return
	}

	foundActivationBlock := false

	switch nextFork[s.currentFork] {
	case Regolith:
		foundActivationBlock = s.config.IsRegolithActivationBlock(block.Time)
	case Canyon:
		foundActivationBlock = s.config.IsCanyonActivationBlock(block.Time)
	case Delta:
		foundActivationBlock = s.config.IsDeltaActivationBlock(block.Time)
	case Ecotone:
		foundActivationBlock = s.config.IsEcotoneActivationBlock(block.Time)
	case Fjord:
		foundActivationBlock = s.config.IsFjordActivationBlock(block.Time)
	case Granite:
		foundActivationBlock = s.config.IsGraniteActivationBlock(block.Time)
	case Holocene:
		foundActivationBlock = s.config.IsHoloceneActivationBlock(block.Time)
	case Interop:
		foundActivationBlock = s.config.IsInteropActivationBlock(block.Time)
	}

	if foundActivationBlock {
		s.currentFork = nextFork[s.currentFork]
		log.Info("Detected hardfork activation block", "forkName", s.currentFork, "timestamp", block.Time, "blockNum", block.Number, "hash", block.Hash)
	}
}

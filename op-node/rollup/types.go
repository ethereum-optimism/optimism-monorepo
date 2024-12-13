package rollup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var (
	ErrBlockTimeZero                 = errors.New("block time cannot be 0")
	ErrMissingChannelTimeout         = errors.New("channel timeout must be set, this should cover at least a L1 block time")
	ErrInvalidSeqWindowSize          = errors.New("sequencing window size must at least be 2")
	ErrInvalidMaxSeqDrift            = errors.New("maximum sequencer drift must be greater than 0")
	ErrMissingGenesisL1Hash          = errors.New("genesis L1 hash cannot be empty")
	ErrMissingGenesisL2Hash          = errors.New("genesis L2 hash cannot be empty")
	ErrGenesisHashesSame             = errors.New("achievement get! rollup inception: L1 and L2 genesis cannot be the same")
	ErrMissingGenesisL2Time          = errors.New("missing L2 genesis time")
	ErrMissingBatcherAddr            = errors.New("missing genesis system config batcher address")
	ErrMissingScalar                 = errors.New("missing genesis system config scalar")
	ErrMissingGasLimit               = errors.New("missing genesis system config gas limit")
	ErrMissingBatchInboxAddress      = errors.New("missing batch inbox address")
	ErrMissingDepositContractAddress = errors.New("missing deposit contract address")
	ErrMissingL1ChainID              = errors.New("L1 chain ID must not be nil")
	ErrMissingL2ChainID              = errors.New("L2 chain ID must not be nil")
	ErrChainIDsSame                  = errors.New("L1 and L2 chain IDs must be different")
	ErrL1ChainIDNotPositive          = errors.New("L1 chain ID must be non-zero and positive")
	ErrL2ChainIDNotPositive          = errors.New("L2 chain ID must be non-zero and positive")
)

type Genesis struct {
	// The L1 block that the rollup starts *after* (no derived transactions)
	L1 eth.BlockID `json:"l1"`
	// The L2 block the rollup starts from (no transactions, pre-configured state)
	L2 eth.BlockID `json:"l2"`
	// Timestamp of L2 block
	L2Time uint64 `json:"l2_time"`
	// Initial system configuration values.
	// The L2 genesis block may not include transactions, and thus cannot encode the config values,
	// unlike later L2 blocks.
	SystemConfig eth.SystemConfig `json:"system_config"`
}

type AltDAConfig struct {
	// L1 DataAvailabilityChallenge contract proxy address
	DAChallengeAddress common.Address `json:"da_challenge_contract_address,omitempty"`
	// CommitmentType specifies which commitment type can be used. Defaults to Keccak (type 0) if not present
	CommitmentType string `json:"da_commitment_type"`
	// DA challenge window value set on the DAC contract. Used in alt-da mode
	// to compute when a commitment can no longer be challenged.
	DAChallengeWindow uint64 `json:"da_challenge_window"`
	// DA resolve window value set on the DAC contract. Used in alt-da mode
	// to compute when a challenge expires and trigger a reorg if needed.
	DAResolveWindow uint64 `json:"da_resolve_window"`
}

type Config struct {
	// Genesis anchor point of the rollup
	Genesis Genesis `json:"genesis"`
	// Seconds per L2 block
	BlockTime uint64 `json:"block_time"`
	// Sequencer batches may not be more than MaxSequencerDrift seconds after
	// the L1 timestamp of their L1 origin time.
	//
	// Note: When L1 has many 1 second consecutive blocks, and L2 grows at fixed 2 seconds,
	// the L2 time may still grow beyond this difference.
	//
	// With Fjord, the MaxSequencerDrift becomes a constant. Use the ChainSpec
	// instead of reading this rollup configuration field directly to determine
	// the max sequencer drift for a given block based on the block's L1 origin.
	// Chains that activate Fjord at genesis may leave this field empty.
	MaxSequencerDrift uint64 `json:"max_sequencer_drift,omitempty"`
	// Number of epochs (L1 blocks) per sequencing window, including the epoch L1 origin block itself
	SeqWindowSize uint64 `json:"seq_window_size"`
	// Number of L1 blocks between when a channel can be opened and when it must be closed by.
	ChannelTimeoutBedrock uint64 `json:"channel_timeout"`
	// Required to verify L1 signatures
	L1ChainID *big.Int `json:"l1_chain_id"`
	// Required to identify the L2 network and create p2p signatures unique for this chain.
	L2ChainID *big.Int `json:"l2_chain_id"`

	// RegolithTime sets the activation time of the Regolith network-upgrade:
	// a pre-mainnet Bedrock change that addresses findings of the Sherlock contest related to deposit attributes.
	// "Regolith" is the loose deposited rock that sits on top of Bedrock.
	// Active if RegolithTime != nil && L2 block timestamp >= *RegolithTime, inactive otherwise.
	RegolithTime *uint64 `json:"regolith_time,omitempty"`

	// CanyonTime sets the activation time of the Canyon network upgrade.
	// Active if CanyonTime != nil && L2 block timestamp >= *CanyonTime, inactive otherwise.
	CanyonTime *uint64 `json:"canyon_time,omitempty"`

	// DeltaTime sets the activation time of the Delta network upgrade.
	// Active if DeltaTime != nil && L2 block timestamp >= *DeltaTime, inactive otherwise.
	DeltaTime *uint64 `json:"delta_time,omitempty"`

	// EcotoneTime sets the activation time of the Ecotone network upgrade.
	// Active if EcotoneTime != nil && L2 block timestamp >= *EcotoneTime, inactive otherwise.
	EcotoneTime *uint64 `json:"ecotone_time,omitempty"`

	// FjordTime sets the activation time of the Fjord network upgrade.
	// Active if FjordTime != nil && L2 block timestamp >= *FjordTime, inactive otherwise.
	FjordTime *uint64 `json:"fjord_time,omitempty"`

	// GraniteTime sets the activation time of the Granite network upgrade.
	// Active if GraniteTime != nil && L2 block timestamp >= *GraniteTime, inactive otherwise.
	GraniteTime *uint64 `json:"granite_time,omitempty"`

	// HoloceneTime sets the activation time of the Holocene network upgrade.
	// Active if HoloceneTime != nil && L2 block timestamp >= *HoloceneTime, inactive otherwise.
	HoloceneTime *uint64 `json:"holocene_time,omitempty"`

	// InteropTime sets the activation time for an experimental feature-set, activated like a hardfork.
	// Active if InteropTime != nil && L2 block timestamp >= *InteropTime, inactive otherwise.
	InteropTime *uint64 `json:"interop_time,omitempty"`

	// Note: below addresses are part of the block-derivation process,
	// and required to be the same network-wide to stay in consensus.

	// L1 address that batches are sent to.
	BatchInboxAddress common.Address `json:"batch_inbox_address"`
	// L1 Deposit Contract Address
	DepositContractAddress common.Address `json:"deposit_contract_address"`
	// L1 System Config Address
	L1SystemConfigAddress common.Address `json:"l1_system_config_address"`

	// L1 address that declares the protocol versions, optional (Beta feature)
	ProtocolVersionsAddress common.Address `json:"protocol_versions_address,omitempty"`

	// AltDAConfig. We are in the process of migrating to the AltDAConfig from these legacy top level values
	AltDAConfig *AltDAConfig `json:"alt_da,omitempty"`
}

// ValidateL1Config checks L1 config variables for errors.
func (cfg *Config) ValidateL1Config(ctx context.Context, client L1Client) error {
	// Validate the L1 Client Chain ID
	if err := cfg.CheckL1ChainID(ctx, client); err != nil {
		return err
	}

	// Validate the Rollup L1 Genesis Blockhash
	if err := cfg.CheckL1GenesisBlockHash(ctx, client); err != nil {
		return err
	}

	return nil
}

// ValidateL2Config checks L2 config variables for errors.
func (cfg *Config) ValidateL2Config(ctx context.Context, client L2Client, skipL2GenesisBlockHash bool) error {
	// Validate the L2 Client Chain ID
	if err := cfg.CheckL2ChainID(ctx, client); err != nil {
		return err
	}

	// Validate the Rollup L2 Genesis Blockhash if requested. We skip this when doing EL sync
	if skipL2GenesisBlockHash {
		return nil
	}
	if err := cfg.CheckL2GenesisBlockHash(ctx, client); err != nil {
		return err
	}

	return nil
}

func (cfg *Config) TimestampForBlock(blockNumber uint64) uint64 {
	return cfg.Genesis.L2Time + ((blockNumber - cfg.Genesis.L2.Number) * cfg.BlockTime)
}

func (cfg *Config) TargetBlockNumber(timestamp uint64) (num uint64, err error) {
	// subtract genesis time from timestamp to get the time elapsed since genesis, and then divide that
	// difference by the block time to get the expected L2 block number at the current time. If the
	// unsafe head does not have this block number, then there is a gap in the queue.
	genesisTimestamp := cfg.Genesis.L2Time
	if timestamp < genesisTimestamp {
		return 0, fmt.Errorf("did not reach genesis time (%d) yet", genesisTimestamp)
	}
	wallClockGenesisDiff := timestamp - genesisTimestamp
	// Note: round down, we should not request blocks into the future.
	blocksSinceGenesis := wallClockGenesisDiff / cfg.BlockTime
	return cfg.Genesis.L2.Number + blocksSinceGenesis, nil
}

type L1Client interface {
	ChainID(context.Context) (*big.Int, error)
	L1BlockRefByNumber(context.Context, uint64) (eth.L1BlockRef, error)
}

// CheckL1ChainID checks that the configured L1 chain ID matches the client's chain ID.
func (cfg *Config) CheckL1ChainID(ctx context.Context, client L1Client) error {
	id, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get L1 chain ID: %w", err)
	}
	if cfg.L1ChainID.Cmp(id) != 0 {
		return fmt.Errorf("incorrect L1 RPC chain id %d, expected %d", id, cfg.L1ChainID)
	}
	return nil
}

// CheckL1GenesisBlockHash checks that the configured L1 genesis block hash is valid for the given client.
func (cfg *Config) CheckL1GenesisBlockHash(ctx context.Context, client L1Client) error {
	l1GenesisBlockRef, err := client.L1BlockRefByNumber(ctx, cfg.Genesis.L1.Number)
	if err != nil {
		return fmt.Errorf("failed to get L1 genesis blockhash: %w", err)
	}
	if l1GenesisBlockRef.Hash != cfg.Genesis.L1.Hash {
		return fmt.Errorf("incorrect L1 genesis block hash %s, expected %s", l1GenesisBlockRef.Hash, cfg.Genesis.L1.Hash)
	}
	return nil
}

type L2Client interface {
	ChainID(context.Context) (*big.Int, error)
	L2BlockRefByNumber(context.Context, uint64) (eth.L2BlockRef, error)
}

// CheckL2ChainID checks that the configured L2 chain ID matches the client's chain ID.
func (cfg *Config) CheckL2ChainID(ctx context.Context, client L2Client) error {
	id, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get L2 chain ID: %w", err)
	}
	if cfg.L2ChainID.Cmp(id) != 0 {
		return fmt.Errorf("incorrect L2 RPC chain id %d, expected %d", id, cfg.L2ChainID)
	}
	return nil
}

// CheckL2GenesisBlockHash checks that the configured L2 genesis block hash is valid for the given client.
func (cfg *Config) CheckL2GenesisBlockHash(ctx context.Context, client L2Client) error {
	l2GenesisBlockRef, err := client.L2BlockRefByNumber(ctx, cfg.Genesis.L2.Number)
	if err != nil {
		return fmt.Errorf("failed to get L2 genesis blockhash: %w", err)
	}
	if l2GenesisBlockRef.Hash != cfg.Genesis.L2.Hash {
		return fmt.Errorf("incorrect L2 genesis block hash %s, expected %s", l2GenesisBlockRef.Hash, cfg.Genesis.L2.Hash)
	}
	return nil
}

// Check verifies that the given configuration makes sense
func (cfg *Config) Check() error {
	if cfg.BlockTime == 0 {
		return ErrBlockTimeZero
	}
	if cfg.ChannelTimeoutBedrock == 0 {
		return ErrMissingChannelTimeout
	}
	if cfg.SeqWindowSize < 2 {
		return ErrInvalidSeqWindowSize
	}
	if cfg.MaxSequencerDrift == 0 {
		return ErrInvalidMaxSeqDrift
	}
	if cfg.Genesis.L1.Hash == (common.Hash{}) {
		return ErrMissingGenesisL1Hash
	}
	if cfg.Genesis.L2.Hash == (common.Hash{}) {
		return ErrMissingGenesisL2Hash
	}
	if cfg.Genesis.L2.Hash == cfg.Genesis.L1.Hash {
		return ErrGenesisHashesSame
	}
	if cfg.Genesis.L2Time == 0 {
		return ErrMissingGenesisL2Time
	}
	if cfg.Genesis.SystemConfig.BatcherAddr == (common.Address{}) {
		return ErrMissingBatcherAddr
	}
	if cfg.Genesis.SystemConfig.Scalar == (eth.Bytes32{}) {
		return ErrMissingScalar
	}
	if cfg.Genesis.SystemConfig.GasLimit == 0 {
		return ErrMissingGasLimit
	}
	if cfg.BatchInboxAddress == (common.Address{}) {
		return ErrMissingBatchInboxAddress
	}
	if cfg.DepositContractAddress == (common.Address{}) {
		return ErrMissingDepositContractAddress
	}
	if cfg.L1ChainID == nil {
		return ErrMissingL1ChainID
	}
	if cfg.L2ChainID == nil {
		return ErrMissingL2ChainID
	}
	if cfg.L1ChainID.Cmp(cfg.L2ChainID) == 0 {
		return ErrChainIDsSame
	}
	if cfg.L1ChainID.Sign() < 1 {
		return ErrL1ChainIDNotPositive
	}
	if cfg.L2ChainID.Sign() < 1 {
		return ErrL2ChainIDNotPositive
	}
	if err := validateAltDAConfig(cfg); err != nil {
		return err
	}

	if err := checkFork(cfg.RegolithTime, cfg.CanyonTime, Regolith, Canyon); err != nil {
		return err
	}
	if err := checkFork(cfg.CanyonTime, cfg.DeltaTime, Canyon, Delta); err != nil {
		return err
	}
	if err := checkFork(cfg.DeltaTime, cfg.EcotoneTime, Delta, Ecotone); err != nil {
		return err
	}
	if err := checkFork(cfg.EcotoneTime, cfg.FjordTime, Ecotone, Fjord); err != nil {
		return err
	}
	if err := checkFork(cfg.FjordTime, cfg.GraniteTime, Fjord, Granite); err != nil {
		return err
	}
	if err := checkFork(cfg.GraniteTime, cfg.HoloceneTime, Granite, Holocene); err != nil {
		return err
	}

	return nil
}

// validateAltDAConfig checks the two approaches to configuring alt-da mode.
// If the legacy values are set, they are copied to the new location. If both are set, they are check for consistency.
func validateAltDAConfig(cfg *Config) error {
	if cfg.AltDAConfig != nil {
		if !(cfg.AltDAConfig.CommitmentType == altda.KeccakCommitmentString || cfg.AltDAConfig.CommitmentType == altda.GenericCommitmentString) {
			return fmt.Errorf("invalid commitment type: %v", cfg.AltDAConfig.CommitmentType)
		}
		if cfg.AltDAConfig.CommitmentType == altda.KeccakCommitmentString && cfg.AltDAConfig.DAChallengeAddress == (common.Address{}) {
			return errors.New("Must set da_challenge_contract_address for keccak commitments")
		} else if cfg.AltDAConfig.CommitmentType == altda.GenericCommitmentString && cfg.AltDAConfig.DAChallengeAddress != (common.Address{}) {
			return errors.New("Must set empty da_challenge_contract_address for generic commitments")
		}
	}
	return nil
}

// checkFork checks that fork A is before or at the same time as fork B
func checkFork(a, b *uint64, aName, bName ForkName) error {
	if a == nil && b == nil {
		return nil
	}
	if a == nil && b != nil {
		return fmt.Errorf("fork %s set (to %d), but prior fork %s missing", bName, *b, aName)
	}
	if a != nil && b == nil {
		return nil
	}
	if *a > *b {
		return fmt.Errorf("fork %s set to %d, but prior fork %s has higher offset %d", bName, *b, aName, *a)
	}
	return nil
}

func (c *Config) L1Signer() types.Signer {
	return types.NewCancunSigner(c.L1ChainID)
}

// IsRegolith returns true if the Regolith hardfork is active at or past the given timestamp.
func (c *Config) IsRegolith(timestamp uint64) bool {
	return c.RegolithTime != nil && timestamp >= *c.RegolithTime
}

// IsCanyon returns true if the Canyon hardfork is active at or past the given timestamp.
func (c *Config) IsCanyon(timestamp uint64) bool {
	return c.CanyonTime != nil && timestamp >= *c.CanyonTime
}

// IsDelta returns true if the Delta hardfork is active at or past the given timestamp.
func (c *Config) IsDelta(timestamp uint64) bool {
	return c.DeltaTime != nil && timestamp >= *c.DeltaTime
}

// IsEcotone returns true if the Ecotone hardfork is active at or past the given timestamp.
func (c *Config) IsEcotone(timestamp uint64) bool {
	return c.EcotoneTime != nil && timestamp >= *c.EcotoneTime
}

// IsFjord returns true if the Fjord hardfork is active at or past the given timestamp.
func (c *Config) IsFjord(timestamp uint64) bool {
	return c.FjordTime != nil && timestamp >= *c.FjordTime
}

// IsGranite returns true if the Granite hardfork is active at or past the given timestamp.
func (c *Config) IsGranite(timestamp uint64) bool {
	return c.GraniteTime != nil && timestamp >= *c.GraniteTime
}

// IsHolocene returns true if the Holocene hardfork is active at or past the given timestamp.
func (c *Config) IsHolocene(timestamp uint64) bool {
	return c.HoloceneTime != nil && timestamp >= *c.HoloceneTime
}

// IsInterop returns true if the Interop hardfork is active at or past the given timestamp.
func (c *Config) IsInterop(timestamp uint64) bool {
	return c.InteropTime != nil && timestamp >= *c.InteropTime
}

func (c *Config) IsRegolithActivationBlock(l2BlockTime uint64) bool {
	return c.IsRegolith(l2BlockTime) &&
		l2BlockTime >= c.BlockTime &&
		!c.IsRegolith(l2BlockTime-c.BlockTime)
}

func (c *Config) IsCanyonActivationBlock(l2BlockTime uint64) bool {
	return c.IsCanyon(l2BlockTime) &&
		l2BlockTime >= c.BlockTime &&
		!c.IsCanyon(l2BlockTime-c.BlockTime)
}

func (c *Config) IsDeltaActivationBlock(l2BlockTime uint64) bool {
	return c.IsDelta(l2BlockTime) &&
		l2BlockTime >= c.BlockTime &&
		!c.IsDelta(l2BlockTime-c.BlockTime)
}

// IsEcotoneActivationBlock returns whether the specified block is the first block subject to the
// Ecotone upgrade. Ecotone activation at genesis does not count.
func (c *Config) IsEcotoneActivationBlock(l2BlockTime uint64) bool {
	return c.IsEcotone(l2BlockTime) &&
		l2BlockTime >= c.BlockTime &&
		!c.IsEcotone(l2BlockTime-c.BlockTime)
}

// IsFjordActivationBlock returns whether the specified block is the first block subject to the
// Fjord upgrade.
func (c *Config) IsFjordActivationBlock(l2BlockTime uint64) bool {
	return c.IsFjord(l2BlockTime) &&
		l2BlockTime >= c.BlockTime &&
		!c.IsFjord(l2BlockTime-c.BlockTime)
}

// IsGraniteActivationBlock returns whether the specified block is the first block subject to the
// Granite upgrade.
func (c *Config) IsGraniteActivationBlock(l2BlockTime uint64) bool {
	return c.IsGranite(l2BlockTime) &&
		l2BlockTime >= c.BlockTime &&
		!c.IsGranite(l2BlockTime-c.BlockTime)
}

// IsHoloceneActivationBlock returns whether the specified block is the first block subject to the
// Holocene upgrade.
func (c *Config) IsHoloceneActivationBlock(l2BlockTime uint64) bool {
	return c.IsHolocene(l2BlockTime) &&
		l2BlockTime >= c.BlockTime &&
		!c.IsHolocene(l2BlockTime-c.BlockTime)
}

func (c *Config) IsInteropActivationBlock(l2BlockTime uint64) bool {
	return c.IsInterop(l2BlockTime) &&
		l2BlockTime >= c.BlockTime &&
		!c.IsInterop(l2BlockTime-c.BlockTime)
}

// IsActivationBlock returns the fork which activates at the block with time newTime if the previous
// block's time is oldTime. It return an empty ForkName if no fork activation takes place between
// those timestamps. It can be used for both, L1 and L2 blocks.
// TODO(12490): Currently only supports Holocene. Will be modularized in a follow-up.
func (c *Config) IsActivationBlock(oldTime, newTime uint64) ForkName {
	if c.IsHolocene(newTime) && !c.IsHolocene(oldTime) {
		return Holocene
	}
	return ""
}

func (c *Config) ActivateAtGenesis(hardfork ForkName) {
	// IMPORTANT! ordered from newest to oldest
	switch hardfork {
	case Interop:
		c.InteropTime = new(uint64)
		fallthrough
	case Holocene:
		c.HoloceneTime = new(uint64)
		fallthrough
	case Granite:
		c.GraniteTime = new(uint64)
		fallthrough
	case Fjord:
		c.FjordTime = new(uint64)
		fallthrough
	case Ecotone:
		c.EcotoneTime = new(uint64)
		fallthrough
	case Delta:
		c.DeltaTime = new(uint64)
		fallthrough
	case Canyon:
		c.CanyonTime = new(uint64)
		fallthrough
	case Regolith:
		c.RegolithTime = new(uint64)
		fallthrough
	case Bedrock:
		// default
	case None:
		break
	}
}

// ForkchoiceUpdatedVersion returns the EngineAPIMethod suitable for the chain hard fork version.
func (c *Config) ForkchoiceUpdatedVersion(attr *eth.PayloadAttributes) eth.EngineAPIMethod {
	if attr == nil {
		// Don't begin payload build process.
		return eth.FCUV3
	}
	ts := uint64(attr.Timestamp)
	if c.IsEcotone(ts) {
		// Cancun
		return eth.FCUV3
	} else if c.IsCanyon(ts) {
		// Shanghai
		return eth.FCUV2
	} else {
		// According to Ethereum engine API spec, we can use fcuV2 here,
		// but upstream Geth v1.13.11 does not accept V2 before Shanghai.
		return eth.FCUV1
	}
}

// NewPayloadVersion returns the EngineAPIMethod suitable for the chain hard fork version.
func (c *Config) NewPayloadVersion(timestamp uint64) eth.EngineAPIMethod {
	if c.IsEcotone(timestamp) {
		// Cancun
		return eth.NewPayloadV3
	} else {
		return eth.NewPayloadV2
	}
}

// GetPayloadVersion returns the EngineAPIMethod suitable for the chain hard fork version.
func (c *Config) GetPayloadVersion(timestamp uint64) eth.EngineAPIMethod {
	if c.IsEcotone(timestamp) {
		// Cancun
		return eth.GetPayloadV3
	} else {
		return eth.GetPayloadV2
	}
}

// GetOPAltDAConfig validates and returns the altDA config from the rollup config.
func (c *Config) GetOPAltDAConfig() (altda.Config, error) {
	if c.AltDAConfig == nil {
		return altda.Config{}, errors.New("no altDA config")
	}
	if c.AltDAConfig.DAChallengeWindow == uint64(0) {
		return altda.Config{}, errors.New("missing DAChallengeWindow")
	}
	if c.AltDAConfig.DAResolveWindow == uint64(0) {
		return altda.Config{}, errors.New("missing DAResolveWindow")
	}
	t, err := altda.CommitmentTypeFromString(c.AltDAConfig.CommitmentType)
	if err != nil {
		return altda.Config{}, err
	}
	return altda.Config{
		DAChallengeContractAddress: c.AltDAConfig.DAChallengeAddress,
		ChallengeWindow:            c.AltDAConfig.DAChallengeWindow,
		ResolveWindow:              c.AltDAConfig.DAResolveWindow,
		CommitmentType:             t,
	}, nil
}

func (c *Config) AltDAEnabled() bool {
	return c.AltDAConfig != nil
}

// SyncLookback computes the number of blocks to walk back in order to find the correct L1 origin.
// In alt-da mode longest possible window is challenge + resolve windows.
func (c *Config) SyncLookback() uint64 {
	if c.AltDAEnabled() {
		if win := (c.AltDAConfig.DAChallengeWindow + c.AltDAConfig.DAResolveWindow); win > c.SeqWindowSize {
			return win
		}
	}
	return c.SeqWindowSize
}

// Description outputs a banner describing the important parts of rollup configuration in a human-readable form.
// Optionally provide a mapping of L2 chain IDs to network names to label the L2 chain with if not unknown.
// The config should be config.Check()-ed before creating a description.
func (c *Config) Description(l2Chains map[string]string) string {
	// Find and report the network the user is running
	var banner string
	networkL2 := ""
	if l2Chains != nil {
		networkL2 = l2Chains[c.L2ChainID.String()]
	}
	if networkL2 == "" {
		networkL2 = "unknown L2"
	}
	networkL1 := params.NetworkNames[c.L1ChainID.String()]
	if networkL1 == "" {
		networkL1 = "unknown L1"
	}
	banner += fmt.Sprintf("L2 Chain ID: %v (%s)\n", c.L2ChainID, networkL2)
	banner += fmt.Sprintf("L1 Chain ID: %v (%s)\n", c.L1ChainID, networkL1)
	// Report the genesis configuration
	banner += "Bedrock starting point:\n"
	banner += fmt.Sprintf("  L2 starting time: %d ~ %s\n", c.Genesis.L2Time, fmtTime(c.Genesis.L2Time))
	banner += fmt.Sprintf("  L2 block: %s %d\n", c.Genesis.L2.Hash, c.Genesis.L2.Number)
	banner += fmt.Sprintf("  L1 block: %s %d\n", c.Genesis.L1.Hash, c.Genesis.L1.Number)
	// Report the upgrade configuration
	banner += "Post-Bedrock Network Upgrades (timestamp based):\n"
	banner += fmt.Sprintf("  - Regolith: %s\n", fmtForkTimeOrUnset(c.RegolithTime))
	banner += fmt.Sprintf("  - Canyon: %s\n", fmtForkTimeOrUnset(c.CanyonTime))
	banner += fmt.Sprintf("  - Delta: %s\n", fmtForkTimeOrUnset(c.DeltaTime))
	banner += fmt.Sprintf("  - Ecotone: %s\n", fmtForkTimeOrUnset(c.EcotoneTime))
	banner += fmt.Sprintf("  - Fjord: %s\n", fmtForkTimeOrUnset(c.FjordTime))
	banner += fmt.Sprintf("  - Granite: %s\n", fmtForkTimeOrUnset(c.GraniteTime))
	banner += fmt.Sprintf("  - Holocene: %s\n", fmtForkTimeOrUnset(c.HoloceneTime))
	banner += fmt.Sprintf("  - Interop: %s\n", fmtForkTimeOrUnset(c.InteropTime))
	// Report the protocol version
	banner += fmt.Sprintf("Node supports up to OP-Stack Protocol Version: %s\n", OPStackSupport)
	if c.AltDAConfig != nil {
		banner += fmt.Sprintf("Node supports Alt-DA Mode with CommitmentType %v\n", c.AltDAConfig.CommitmentType)
	}
	return banner
}

// LogDescription outputs a banner describing the important parts of rollup configuration in a log format.
// Optionally provide a mapping of L2 chain IDs to network names to label the L2 chain with if not unknown.
// The config should be config.Check()-ed before creating a description.
func (c *Config) LogDescription(log log.Logger, l2Chains map[string]string) {
	// Find and report the network the user is running
	networkL2 := ""
	if l2Chains != nil {
		networkL2 = l2Chains[c.L2ChainID.String()]
	}
	if networkL2 == "" {
		networkL2 = "unknown L2"
	}
	networkL1 := params.NetworkNames[c.L1ChainID.String()]
	if networkL1 == "" {
		networkL1 = "unknown L1"
	}

	log.Info("Rollup Config", "l2_chain_id", c.L2ChainID, "l2_network", networkL2, "l1_chain_id", c.L1ChainID,
		"l1_network", networkL1, "l2_start_time", c.Genesis.L2Time, "l2_block_hash", c.Genesis.L2.Hash.String(),
		"l2_block_number", c.Genesis.L2.Number, "l1_block_hash", c.Genesis.L1.Hash.String(),
		"l1_block_number", c.Genesis.L1.Number, "regolith_time", fmtForkTimeOrUnset(c.RegolithTime),
		"canyon_time", fmtForkTimeOrUnset(c.CanyonTime),
		"delta_time", fmtForkTimeOrUnset(c.DeltaTime),
		"ecotone_time", fmtForkTimeOrUnset(c.EcotoneTime),
		"fjord_time", fmtForkTimeOrUnset(c.FjordTime),
		"granite_time", fmtForkTimeOrUnset(c.GraniteTime),
		"holocene_time", fmtForkTimeOrUnset(c.HoloceneTime),
		"interop_time", fmtForkTimeOrUnset(c.InteropTime),
		"alt_da", c.AltDAConfig != nil,
	)
}

func (c *Config) ParseRollupConfig(in io.Reader) error {
	dec := json.NewDecoder(in)
	dec.DisallowUnknownFields()
	if err := dec.Decode(c); err != nil {
		return fmt.Errorf("failed to decode rollup config: %w", err)
	}
	return nil
}

func fmtForkTimeOrUnset(v *uint64) string {
	if v == nil {
		return "(not configured)"
	}
	if *v == 0 { // don't output the unix epoch time if it's really just activated at genesis.
		return "@ genesis"
	}
	return fmt.Sprintf("@ %-10v ~ %s", *v, fmtTime(*v))
}

func fmtTime(v uint64) string {
	return time.Unix(int64(v), 0).Format(time.UnixDate)
}

type Epoch uint64

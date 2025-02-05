package config

import (
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"slices"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrMissingTraceType              = errors.New("no supported trace types specified")
	ErrMissingDatadir                = errors.New("missing datadir")
	ErrMaxConcurrencyZero            = errors.New("max concurrency must not be 0")
	ErrMissingL2Rpc                  = errors.New("missing L2 rpc url")
	ErrMissingCannonAbsolutePreState = errors.New("missing cannon absolute pre-state")
	ErrMissingL1EthRPC               = errors.New("missing l1 eth rpc url")
	ErrMissingL1Beacon               = errors.New("missing l1 beacon url")
	ErrMissingGameFactoryAddress     = errors.New("missing game factory address")
	ErrMissingCannonSnapshotFreq     = errors.New("missing cannon snapshot freq")
	ErrMissingCannonInfoFreq         = errors.New("missing cannon info freq")

	ErrMissingRollupRpc     = errors.New("missing rollup rpc url")
	ErrMissingSupervisorRpc = errors.New("missing supervisor rpc url")

	ErrMissingAsteriscAbsolutePreState = errors.New("missing asterisc absolute pre-state")
	ErrMissingAsteriscSnapshotFreq     = errors.New("missing asterisc snapshot freq")
	ErrMissingAsteriscInfoFreq         = errors.New("missing asterisc info freq")

	ErrMissingAsteriscKonaAbsolutePreState = errors.New("missing asterisc kona absolute pre-state")
	ErrMissingAsteriscKonaSnapshotFreq     = errors.New("missing asterisc kona snapshot freq")
	ErrMissingAsteriscKonaInfoFreq         = errors.New("missing asterisc kona info freq")
)

const (
	DefaultPollInterval         = time.Second * 12
	DefaultCannonSnapshotFreq   = uint(1_000_000_000)
	DefaultCannonInfoFreq       = uint(10_000_000)
	DefaultAsteriscSnapshotFreq = uint(1_000_000_000)
	DefaultAsteriscInfoFreq     = uint(10_000_000)
	// DefaultGameWindow is the default maximum time duration in the past
	// that the challenger will look for games to progress.
	// The default value is 28 days. The worst case duration for a game is 16 days
	// (due to clock extension), plus 7 days WETH withdrawal delay leaving a 5 day
	// buffer to monitor games to ensure bonds are claimed.
	DefaultGameWindow   = 28 * 24 * time.Hour
	DefaultMaxPendingTx = 10
)

// Config is a well typed config that is parsed from the CLI params.
// This also contains config options for auxiliary services.
// It is used to initialize the challenger.
type Config struct {
	L1EthRpc             string           // L1 RPC Url
	L1Beacon             string           // L1 Beacon API Url
	GameFactoryAddress   common.Address   // Address of the dispute game factory
	GameAllowlist        []common.Address // Allowlist of fault game addresses
	GameWindow           time.Duration    // Maximum time duration to look for games to progress
	Datadir              string           // Data Directory
	MaxConcurrency       uint             // Maximum number of threads to use when progressing games
	PollInterval         time.Duration    // Polling interval for latest-block subscription when using an HTTP RPC provider
	AllowInvalidPrestate bool             // Whether to allow responding to games where the prestate does not match

	AdditionalBondClaimants []common.Address // List of addresses to claim bonds for in addition to the tx manager sender

	SelectiveClaimResolution bool // Whether to only resolve claims for the claimants in AdditionalBondClaimants union [TxSender.From()]

	TraceTypes []types.TraceType // Type of traces supported

	RollupRpc     string   // L2 Rollup RPC Url
	SupervisorRPC string   // L2 supervisor RPC URL
	L2Rpcs        []string // L2 RPC Url

	// Specific to the cannon trace provider
	Cannon                        vm.Config
	CannonAbsolutePreState        string   // File to load the absolute pre-state for Cannon traces from
	CannonAbsolutePreStateBaseURL *url.URL // Base URL to retrieve absolute pre-states for Cannon traces from

	// Specific to the asterisc trace provider
	Asterisc                            vm.Config
	AsteriscAbsolutePreState            string   // File to load the absolute pre-state for Asterisc traces from
	AsteriscAbsolutePreStateBaseURL     *url.URL // Base URL to retrieve absolute pre-states for Asterisc traces from
	AsteriscKona                        vm.Config
	AsteriscKonaAbsolutePreState        string   // File to load the absolute pre-state for AsteriscKona traces from
	AsteriscKonaAbsolutePreStateBaseURL *url.URL // Base URL to retrieve absolute pre-states for AsteriscKona traces from

	MaxPendingTx uint64 // Maximum number of pending transactions (0 == no limit)

	TxMgrConfig   txmgr.CLIConfig
	MetricsConfig opmetrics.CLIConfig
	PprofConfig   oppprof.CLIConfig
}

func NewConfig(
	gameFactoryAddress common.Address,
	l1EthRpc string,
	l1BeaconApi string,
	l2RollupRpc string,
	l2EthRpc string,
	datadir string,
	supportedTraceTypes ...types.TraceType,
) Config {
	return Config{
		L1EthRpc:           l1EthRpc,
		L1Beacon:           l1BeaconApi,
		RollupRpc:          l2RollupRpc,
		L2Rpcs:             []string{l2EthRpc},
		GameFactoryAddress: gameFactoryAddress,
		MaxConcurrency:     uint(runtime.NumCPU()),
		PollInterval:       DefaultPollInterval,

		TraceTypes: supportedTraceTypes,

		MaxPendingTx: DefaultMaxPendingTx,

		TxMgrConfig:   txmgr.NewCLIConfig(l1EthRpc, txmgr.DefaultChallengerFlagValues),
		MetricsConfig: opmetrics.DefaultCLIConfig(),
		PprofConfig:   oppprof.DefaultCLIConfig(),

		Datadir: datadir,

		Cannon: vm.Config{
			VmType:          types.TraceTypeCannon,
			L1:              l1EthRpc,
			L1Beacon:        l1BeaconApi,
			L2s:             []string{l2EthRpc},
			SnapshotFreq:    DefaultCannonSnapshotFreq,
			InfoFreq:        DefaultCannonInfoFreq,
			DebugInfo:       true,
			BinarySnapshots: true,
		},
		Asterisc: vm.Config{
			VmType:          types.TraceTypeAsterisc,
			L1:              l1EthRpc,
			L1Beacon:        l1BeaconApi,
			L2s:             []string{l2EthRpc},
			SnapshotFreq:    DefaultAsteriscSnapshotFreq,
			InfoFreq:        DefaultAsteriscInfoFreq,
			BinarySnapshots: true,
		},
		AsteriscKona: vm.Config{
			VmType:          types.TraceTypeAsteriscKona,
			L1:              l1EthRpc,
			L1Beacon:        l1BeaconApi,
			L2s:             []string{l2EthRpc},
			SnapshotFreq:    DefaultAsteriscSnapshotFreq,
			InfoFreq:        DefaultAsteriscInfoFreq,
			BinarySnapshots: true,
		},
		GameWindow: DefaultGameWindow,
	}
}

func (c Config) TraceTypeEnabled(t types.TraceType) bool {
	return slices.Contains(c.TraceTypes, t)
}

func (c Config) Check() error {
	if c.L1EthRpc == "" {
		return ErrMissingL1EthRPC
	}
	if c.L1Beacon == "" {
		return ErrMissingL1Beacon
	}
	if len(c.L2Rpcs) == 0 {
		return ErrMissingL2Rpc
	}
	if c.GameFactoryAddress == (common.Address{}) {
		return ErrMissingGameFactoryAddress
	}
	if len(c.TraceTypes) == 0 {
		return ErrMissingTraceType
	}
	if c.Datadir == "" {
		return ErrMissingDatadir
	}
	if c.MaxConcurrency == 0 {
		return ErrMaxConcurrencyZero
	}
	if c.TraceTypeEnabled(types.TraceTypeSuperCannon) {
		if c.SupervisorRPC == "" {
			return ErrMissingSupervisorRpc
		}
		if err := c.validateBaseCannonOptions(); err != nil {
			return err
		}
	}
	if c.TraceTypeEnabled(types.TraceTypeCannon) || c.TraceTypeEnabled(types.TraceTypePermissioned) {
		if c.RollupRpc == "" {
			return ErrMissingRollupRpc
		}
		if err := c.validateBaseCannonOptions(); err != nil {
			return err
		}
	}
	if c.TraceTypeEnabled(types.TraceTypeAsterisc) {
		if c.RollupRpc == "" {
			return ErrMissingRollupRpc
		}
		if err := c.Asterisc.Check(); err != nil {
			return fmt.Errorf("asterisc: %w", err)
		}
		if c.AsteriscAbsolutePreState == "" && c.AsteriscAbsolutePreStateBaseURL == nil {
			return ErrMissingAsteriscAbsolutePreState
		}
		if c.Asterisc.SnapshotFreq == 0 {
			return ErrMissingAsteriscSnapshotFreq
		}
		if c.Asterisc.InfoFreq == 0 {
			return ErrMissingAsteriscInfoFreq
		}
	}
	if c.TraceTypeEnabled(types.TraceTypeAsteriscKona) {
		if c.RollupRpc == "" {
			return ErrMissingRollupRpc
		}
		if err := c.AsteriscKona.Check(); err != nil {
			return fmt.Errorf("asterisc kona: %w", err)
		}
		if c.AsteriscKonaAbsolutePreState == "" && c.AsteriscKonaAbsolutePreStateBaseURL == nil {
			return ErrMissingAsteriscKonaAbsolutePreState
		}
		if c.AsteriscKona.SnapshotFreq == 0 {
			return ErrMissingAsteriscKonaSnapshotFreq
		}
		if c.AsteriscKona.InfoFreq == 0 {
			return ErrMissingAsteriscKonaInfoFreq
		}
	}
	if c.TraceTypeEnabled(types.TraceTypeAlphabet) || c.TraceTypeEnabled(types.TraceTypeFast) {
		if c.RollupRpc == "" {
			return ErrMissingRollupRpc
		}
	}
	if err := c.TxMgrConfig.Check(); err != nil {
		return err
	}
	if err := c.MetricsConfig.Check(); err != nil {
		return err
	}
	if err := c.PprofConfig.Check(); err != nil {
		return err
	}
	return nil
}

func (c Config) validateBaseCannonOptions() error {
	if err := c.Cannon.Check(); err != nil {
		return fmt.Errorf("cannon: %w", err)
	}
	if c.CannonAbsolutePreState == "" && c.CannonAbsolutePreStateBaseURL == nil {
		return ErrMissingCannonAbsolutePreState
	}
	if c.Cannon.SnapshotFreq == 0 {
		return ErrMissingCannonSnapshotFreq
	}
	if c.Cannon.InfoFreq == 0 {
		return ErrMissingCannonInfoFreq
	}
	return nil
}

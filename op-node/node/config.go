package node

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/flags"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum/go-ethereum/log"
)

type Config struct {
	L1 L1EndpointSetup
	L2 L2EndpointSetup

	Beacon L1BeaconEndpointSetup

	Supervisor SupervisorEndpointSetup

	Driver driver.Config

	Rollup rollup.Config

	// P2PSigner will be used for signing off on published content
	// if the node is sequencing and if the p2p stack is enabled
	P2PSigner p2p.SignerSetup

	RPC RPCConfig

	P2P p2p.SetupP2P

	Metrics MetricsConfig

	Pprof oppprof.CLIConfig

	// Used to poll the L1 for new finalized or safe blocks
	L1EpochPollInterval time.Duration

	ConfigPersistence ConfigPersistence

	// Path to store safe head database. Disabled when set to empty string
	SafeDBPath string

	// RuntimeConfigReloadInterval defines the interval between runtime config reloads.
	// Disabled if <= 0.
	// Runtime config changes should be picked up from log-events,
	// but if log-events are not coming in (e.g. not syncing blocks) then the reload ensures the config stays accurate.
	RuntimeConfigReloadInterval time.Duration

	// Optional
	Tracer Tracer

	Sync sync.Config

	// To halt when detecting the node does not support a signaled protocol version
	// change of the given severity (major/minor/patch). Disabled if empty.
	RollupHalt string

	// Cancel to request a premature shutdown of the node itself, e.g. when halting. This may be nil.
	Cancel context.CancelCauseFunc

	// Conductor is used to determine this node is the leader sequencer.
	ConductorEnabled    bool
	ConductorRpc        ConductorRPCFunc
	ConductorRpcTimeout time.Duration

	// AltDA config
	AltDA altda.CLIConfig
}

// ConductorRPCFunc retrieves the endpoint. The RPC may not immediately be available.
type ConductorRPCFunc func(ctx context.Context) (string, error)

type RPCConfig struct {
	ListenAddr  string
	ListenPort  int
	EnableAdmin bool
}

func (cfg *RPCConfig) HttpEndpoint() string {
	return fmt.Sprintf("http://%s:%d", cfg.ListenAddr, cfg.ListenPort)
}

type MetricsConfig struct {
	Enabled    bool
	ListenAddr string
	ListenPort int
}

func (m MetricsConfig) Check() error {
	if !m.Enabled {
		return nil
	}

	if m.ListenPort < 0 || m.ListenPort > math.MaxUint16 {
		return errors.New("invalid metrics port")
	}

	return nil
}

func (cfg *Config) LoadPersisted(log log.Logger) error {
	if !cfg.Driver.SequencerEnabled {
		return nil
	}
	if state, err := cfg.ConfigPersistence.SequencerState(); err != nil {
		return err
	} else if state != StateUnset {
		stopped := state == StateStopped
		if stopped != cfg.Driver.SequencerStopped {
			log.Warn(fmt.Sprintf("Overriding %v with persisted state", flags.SequencerStoppedFlag.Name), "stopped", stopped)
		}
		cfg.Driver.SequencerStopped = stopped
	} else {
		log.Info("No persisted sequencer state loaded")
	}
	return nil
}

// Check verifies that the given configuration makes sense
func (cfg *Config) Check() error {
	if err := cfg.L1.Check(); err != nil {
		return fmt.Errorf("l2 endpoint config error: %w", err)
	}
	if err := cfg.L2.Check(); err != nil {
		return fmt.Errorf("l2 endpoint config error: %w", err)
	}
	if cfg.Rollup.EcotoneTime != nil {
		if cfg.Beacon == nil {
			return fmt.Errorf("the Ecotone upgrade is scheduled (timestamp = %d) but no L1 Beacon API endpoint is configured", *cfg.Rollup.EcotoneTime)
		}
		if err := cfg.Beacon.Check(); err != nil {
			return fmt.Errorf("misconfigured L1 Beacon API endpoint: %w", err)
		}
	}
	if cfg.Rollup.InteropTime != nil {
		if cfg.Supervisor == nil {
			return fmt.Errorf("the Interop upgrade is scheduled (timestamp = %d) but no supervisor RPC endpoint is configured", *cfg.Rollup.InteropTime)
		}
		if err := cfg.Supervisor.Check(); err != nil {
			return fmt.Errorf("misconfigured supervisor RPC endpoint: %w", err)
		}
	}
	if err := cfg.Rollup.Check(); err != nil {
		return fmt.Errorf("rollup config error: %w", err)
	}
	if err := cfg.Metrics.Check(); err != nil {
		return fmt.Errorf("metrics config error: %w", err)
	}
	if err := cfg.Pprof.Check(); err != nil {
		return fmt.Errorf("pprof config error: %w", err)
	}
	if cfg.P2P != nil {
		if err := cfg.P2P.Check(); err != nil {
			return fmt.Errorf("p2p config error: %w", err)
		}
	}
	if !(cfg.RollupHalt == "" || cfg.RollupHalt == "major" || cfg.RollupHalt == "minor" || cfg.RollupHalt == "patch") {
		return fmt.Errorf("invalid rollup halting option: %q", cfg.RollupHalt)
	}
	if cfg.ConductorEnabled {
		if state, _ := cfg.ConfigPersistence.SequencerState(); state != StateUnset {
			return fmt.Errorf("config persistence must be disabled when conductor is enabled")
		}
		if !cfg.Driver.SequencerEnabled {
			return fmt.Errorf("sequencer must be enabled when conductor is enabled")
		}
	}
	if err := cfg.AltDA.Check(); err != nil {
		return fmt.Errorf("altDA config error: %w", err)
	}
	if cfg.AltDA.Enabled {
		log.Warn("Alt-DA Mode is a Beta feature of the MIT licensed OP Stack.  While it has received initial review from core contributors, it is still undergoing testing, and may have bugs or other issues.")
	}
	return nil
}

func (cfg *Config) P2PEnabled() bool {
	return cfg.P2P != nil && !cfg.P2P.Disabled()
}

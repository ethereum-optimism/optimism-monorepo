package opnode

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	altda "github.com/ethereum-optimism/optimism/op-alt-da"
	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-node/flags"
	"github.com/ethereum-optimism/optimism/op-node/node"
	p2pcli "github.com/ethereum-optimism/optimism/op-node/p2p/cli"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/rollup/engine"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	opflags "github.com/ethereum-optimism/optimism/op-service/flags"
)

// NewConfig creates a Config from the provided flags or environment variables.
func NewConfig(ctx *cli.Context, log log.Logger) (*node.Config, error) {
	if err := flags.CheckRequired(ctx); err != nil {
		return nil, err
	}

	rollupConfig, err := NewRollupConfigFromCLI(log, ctx)
	if err != nil {
		return nil, err
	}

	if !ctx.Bool(flags.RollupLoadProtocolVersions.Name) {
		log.Info("Not opted in to ProtocolVersions signal loading, disabling ProtocolVersions contract now.")
		rollupConfig.ProtocolVersionsAddress = common.Address{}
	}

	configPersistence := NewConfigPersistence(ctx)

	driverConfig := NewDriverConfig(ctx)

	p2pSignerSetup, err := p2pcli.LoadSignerSetup(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load p2p signer: %w", err)
	}

	p2pConfig, err := p2pcli.NewConfig(ctx, rollupConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load p2p config: %w", err)
	}

	l1Endpoint := NewL1EndpointConfig(ctx)

	l2Endpoint, err := NewL2EndpointConfig(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("failed to load l2 endpoints info: %w", err)
	}

	syncConfig, err := NewSyncConfig(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create the sync config: %w", err)
	}

	haltOption := ctx.String(flags.RollupHalt.Name)
	if haltOption == "none" {
		haltOption = ""
	}

	if ctx.IsSet(flags.HeartbeatEnabledFlag.Name) ||
		ctx.IsSet(flags.HeartbeatMonikerFlag.Name) ||
		ctx.IsSet(flags.HeartbeatURLFlag.Name) {
		log.Warn("Heartbeat functionality is not supported anymore, CLI flags will be removed in following release.")
	}
	conductorRPCEndpoint := ctx.String(flags.ConductorRpcFlag.Name)
	cfg := &node.Config{
		L1:         l1Endpoint,
		L2:         l2Endpoint,
		Rollup:     *rollupConfig,
		Driver:     *driverConfig,
		Beacon:     NewBeaconEndpointConfig(ctx),
		Supervisor: NewSupervisorEndpointConfig(ctx),
		RPC: node.RPCConfig{
			ListenAddr:  ctx.String(flags.RPCListenAddr.Name),
			ListenPort:  ctx.Int(flags.RPCListenPort.Name),
			EnableAdmin: ctx.Bool(flags.RPCEnableAdmin.Name),
		},
		Metrics: node.MetricsConfig{
			Enabled:    ctx.Bool(flags.MetricsEnabledFlag.Name),
			ListenAddr: ctx.String(flags.MetricsAddrFlag.Name),
			ListenPort: ctx.Int(flags.MetricsPortFlag.Name),
		},
		Pprof:                       oppprof.ReadCLIConfig(ctx),
		P2P:                         p2pConfig,
		P2PSigner:                   p2pSignerSetup,
		L1EpochPollInterval:         ctx.Duration(flags.L1EpochPollIntervalFlag.Name),
		RuntimeConfigReloadInterval: ctx.Duration(flags.RuntimeConfigReloadIntervalFlag.Name),
		ConfigPersistence:           configPersistence,
		SafeDBPath:                  ctx.String(flags.SafeDBPath.Name),
		Sync:                        *syncConfig,
		RollupHalt:                  haltOption,

		ConductorEnabled: ctx.Bool(flags.ConductorEnabledFlag.Name),
		ConductorRpc: func(context.Context) (string, error) {
			return conductorRPCEndpoint, nil
		},
		ConductorRpcTimeout: ctx.Duration(flags.ConductorRpcTimeoutFlag.Name),

		AltDA: altda.ReadCLIConfig(ctx),
	}

	if err := cfg.LoadPersisted(log); err != nil {
		return nil, fmt.Errorf("failed to load driver config: %w", err)
	}

	// conductor controls the sequencer state
	if cfg.ConductorEnabled {
		cfg.Driver.SequencerStopped = true
	}

	if err := cfg.Check(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func NewSupervisorEndpointConfig(ctx *cli.Context) node.SupervisorEndpointSetup {
	return &node.SupervisorEndpointConfig{
		SupervisorAddr: ctx.String(flags.SupervisorAddr.Name),
	}
}

func NewBeaconEndpointConfig(ctx *cli.Context) node.L1BeaconEndpointSetup {
	return &node.L1BeaconEndpointConfig{
		BeaconAddr:             ctx.String(flags.BeaconAddr.Name),
		BeaconHeader:           ctx.String(flags.BeaconHeader.Name),
		BeaconFallbackAddrs:    ctx.StringSlice(flags.BeaconFallbackAddrs.Name),
		BeaconCheckIgnore:      ctx.Bool(flags.BeaconCheckIgnore.Name),
		BeaconFetchAllSidecars: ctx.Bool(flags.BeaconFetchAllSidecars.Name),
	}
}

func NewL1EndpointConfig(ctx *cli.Context) *node.L1EndpointConfig {
	return &node.L1EndpointConfig{
		L1NodeAddr:       ctx.String(flags.L1NodeAddr.Name),
		L1TrustRPC:       ctx.Bool(flags.L1TrustRPC.Name),
		L1RPCKind:        sources.RPCProviderKind(strings.ToLower(ctx.String(flags.L1RPCProviderKind.Name))),
		RateLimit:        ctx.Float64(flags.L1RPCRateLimit.Name),
		BatchSize:        ctx.Int(flags.L1RPCMaxBatchSize.Name),
		HttpPollInterval: ctx.Duration(flags.L1HTTPPollInterval.Name),
		MaxConcurrency:   ctx.Int(flags.L1RPCMaxConcurrency.Name),
	}
}

func NewL2EndpointConfig(ctx *cli.Context, log log.Logger) (*node.L2EndpointConfig, error) {
	l2Addr := ctx.String(flags.L2EngineAddr.Name)
	fileName := ctx.String(flags.L2EngineJWTSecret.Name)
	var secret [32]byte
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return nil, fmt.Errorf("file-name of jwt secret is empty")
	}
	if data, err := os.ReadFile(fileName); err == nil {
		jwtSecret := common.FromHex(strings.TrimSpace(string(data)))
		if len(jwtSecret) != 32 {
			return nil, fmt.Errorf("invalid jwt secret in path %s, not 32 hex-formatted bytes", fileName)
		}
		copy(secret[:], jwtSecret)
	} else {
		log.Warn("Failed to read JWT secret from file, generating a new one now. Configure L2 geth with --authrpc.jwt-secret=" + fmt.Sprintf("%q", fileName))
		if _, err := io.ReadFull(rand.Reader, secret[:]); err != nil {
			return nil, fmt.Errorf("failed to generate jwt secret: %w", err)
		}
		if err := os.WriteFile(fileName, []byte(hexutil.Encode(secret[:])), 0o600); err != nil {
			return nil, err
		}
	}

	return &node.L2EndpointConfig{
		L2EngineAddr:      l2Addr,
		L2EngineJWTSecret: secret,
	}, nil
}

func NewConfigPersistence(ctx *cli.Context) node.ConfigPersistence {
	stateFile := ctx.String(flags.RPCAdminPersistence.Name)
	if stateFile == "" {
		return node.DisabledConfigPersistence{}
	}
	return node.NewConfigPersistence(stateFile)
}

func NewDriverConfig(ctx *cli.Context) *driver.Config {
	return &driver.Config{
		VerifierConfDepth:   ctx.Uint64(flags.VerifierL1Confs.Name),
		SequencerConfDepth:  ctx.Uint64(flags.SequencerL1Confs.Name),
		SequencerEnabled:    ctx.Bool(flags.SequencerEnabledFlag.Name),
		SequencerStopped:    ctx.Bool(flags.SequencerStoppedFlag.Name),
		SequencerMaxSafeLag: ctx.Uint64(flags.SequencerMaxSafeLagFlag.Name),
	}
}

func NewRollupConfigFromCLI(log log.Logger, ctx *cli.Context) (*rollup.Config, error) {
	network := ctx.String(opflags.NetworkFlagName)
	rollupConfigPath := ctx.String(opflags.RollupConfigFlagName)
	if ctx.Bool(flags.BetaExtraNetworks.Name) {
		log.Warn("The beta.extra-networks flag is deprecated and can be omitted safely.")
	}
	rollupConfig, err := NewRollupConfig(log, network, rollupConfigPath)
	if err != nil {
		return nil, err
	}
	applyOverrides(ctx, rollupConfig)
	return rollupConfig, nil
}

func NewRollupConfig(log log.Logger, network string, rollupConfigPath string) (*rollup.Config, error) {
	if network != "" {
		if rollupConfigPath != "" {
			log.Error(`Cannot configure network and rollup-config at the same time.
Startup will proceed to use the network-parameter and ignore the rollup config.
Conflicting configuration is deprecated, and will stop the op-node from starting in the future.
`, "network", network, "rollup_config", rollupConfigPath)
		}
		rollupConfig, err := chaincfg.GetRollupConfig(network)
		if err != nil {
			return nil, err
		}
		return rollupConfig, nil
	}

	file, err := os.Open(rollupConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rollup config: %w", err)
	}
	defer file.Close()

	var rollupConfig rollup.Config
	dec := json.NewDecoder(file)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rollupConfig); err != nil {
		return nil, fmt.Errorf("failed to decode rollup config: %w", err)
	}
	return &rollupConfig, nil
}

func applyOverrides(ctx *cli.Context, rollupConfig *rollup.Config) {
	if ctx.IsSet(opflags.CanyonOverrideFlagName) {
		canyon := ctx.Uint64(opflags.CanyonOverrideFlagName)
		rollupConfig.CanyonTime = &canyon
	}
	if ctx.IsSet(opflags.DeltaOverrideFlagName) {
		delta := ctx.Uint64(opflags.DeltaOverrideFlagName)
		rollupConfig.DeltaTime = &delta
	}
	if ctx.IsSet(opflags.EcotoneOverrideFlagName) {
		ecotone := ctx.Uint64(opflags.EcotoneOverrideFlagName)
		rollupConfig.EcotoneTime = &ecotone
	}
	if ctx.IsSet(opflags.FjordOverrideFlagName) {
		fjord := ctx.Uint64(opflags.FjordOverrideFlagName)
		rollupConfig.FjordTime = &fjord
	}
	if ctx.IsSet(opflags.GraniteOverrideFlagName) {
		granite := ctx.Uint64(opflags.GraniteOverrideFlagName)
		rollupConfig.GraniteTime = &granite
	}
	if ctx.IsSet(opflags.HoloceneOverrideFlagName) {
		holocene := ctx.Uint64(opflags.HoloceneOverrideFlagName)
		rollupConfig.HoloceneTime = &holocene
	}
}

func NewSyncConfig(ctx *cli.Context, log log.Logger) (*sync.Config, error) {
	if ctx.IsSet(flags.L2EngineSyncEnabled.Name) && ctx.IsSet(flags.SyncModeFlag.Name) {
		return nil, errors.New("cannot set both --l2.engine-sync and --syncmode at the same time")
	} else if ctx.IsSet(flags.L2EngineSyncEnabled.Name) {
		log.Error("l2.engine-sync is deprecated and will be removed in a future release. Use --syncmode=execution-layer instead.")
	}
	mode, err := sync.StringToMode(ctx.String(flags.SyncModeFlag.Name))
	if err != nil {
		return nil, err
	}

	engineKind := engine.Kind(ctx.String(flags.L2EngineKind.Name))
	cfg := &sync.Config{
		SyncMode:                       mode,
		SkipSyncStartCheck:             ctx.Bool(flags.SkipSyncStartCheck.Name),
		SupportsPostFinalizationELSync: engineKind.SupportsPostFinalizationELSync(),
	}
	if ctx.Bool(flags.L2EngineSyncEnabled.Name) {
		cfg.SyncMode = sync.ELSync
	}

	return cfg, nil
}

package flags

import (
	"fmt"
	"net/url"
	"runtime"
	"slices"
	"strings"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/flags"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/superchain"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	openum "github.com/ethereum-optimism/optimism/op-service/enum"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

const EnvVarPrefix = "OP_CHALLENGER"

func prefixEnvVars(name string) []string {
	return opservice.PrefixEnvVar(EnvVarPrefix, name)
}

var (
	faultDisputeVMs = []types.TraceType{types.TraceTypeCannon, types.TraceTypeAsterisc, types.TraceTypeAsteriscKona, types.TraceTypeSuperCannon}
	// Required Flags
	L1EthRpcFlag = &cli.StringFlag{
		Name:    "l1-eth-rpc",
		Usage:   "HTTP provider URL for L1.",
		EnvVars: prefixEnvVars("L1_ETH_RPC"),
	}
	L1BeaconFlag = &cli.StringFlag{
		Name:    "l1-beacon",
		Usage:   "Address of L1 Beacon API endpoint to use",
		EnvVars: prefixEnvVars("L1_BEACON"),
	}
	SupervisorRpcFlag = &cli.StringFlag{
		Name:    "supervisor-rpc",
		Usage:   "Provider URL for supervisor RPC",
		EnvVars: prefixEnvVars("SUPERVISOR_RPC"),
	}
	RollupRpcFlag = &cli.StringFlag{
		Name:    "rollup-rpc",
		Usage:   "HTTP provider URL for the rollup node",
		EnvVars: prefixEnvVars("ROLLUP_RPC"),
	}
	NetworkFlag = &cli.StringSliceFlag{
		Name:    flags.NetworkFlagName,
		Usage:   fmt.Sprintf("Predefined network selection. Available networks: %s", strings.Join(chaincfg.AvailableNetworks(), ", ")),
		EnvVars: prefixEnvVars("NETWORK"),
	}
	FactoryAddressFlag = &cli.StringFlag{
		Name:    "game-factory-address",
		Usage:   "Address of the fault game factory contract.",
		EnvVars: prefixEnvVars("GAME_FACTORY_ADDRESS"),
	}
	GameAllowlistFlag = &cli.StringSliceFlag{
		Name: "game-allowlist",
		Usage: "List of Fault Game contract addresses the challenger is allowed to play. " +
			"If empty, the challenger will play all games.",
		EnvVars: prefixEnvVars("GAME_ALLOWLIST"),
	}
	TraceTypeFlag = &cli.StringSliceFlag{
		Name:    "trace-type",
		Usage:   "The trace types to support. Valid options: " + openum.EnumString(types.TraceTypes),
		EnvVars: prefixEnvVars("TRACE_TYPE"),
		Value:   cli.NewStringSlice(types.TraceTypeCannon.String(), types.TraceTypeAsteriscKona.String()),
	}
	DatadirFlag = &cli.StringFlag{
		Name:    "datadir",
		Usage:   "Directory to store data generated as part of responding to games",
		EnvVars: prefixEnvVars("DATADIR"),
	}
	// Optional Flags
	MaxConcurrencyFlag = &cli.UintFlag{
		Name:    "max-concurrency",
		Usage:   "Maximum number of threads to use when progressing games",
		EnvVars: prefixEnvVars("MAX_CONCURRENCY"),
		Value:   uint(runtime.NumCPU()),
	}
	L2EthRpcFlag = &cli.StringSliceFlag{
		Name:    "l2-eth-rpc",
		Usage:   "URLs of L2 JSON-RPC endpoints to use (eth and debug namespace required)",
		EnvVars: prefixEnvVars("L2_ETH_RPC"),
	}
	MaxPendingTransactionsFlag = &cli.Uint64Flag{
		Name:    "max-pending-tx",
		Usage:   "The maximum number of pending transactions. 0 for no limit.",
		Value:   config.DefaultMaxPendingTx,
		EnvVars: prefixEnvVars("MAX_PENDING_TX"),
	}
	HTTPPollInterval = &cli.DurationFlag{
		Name:    "http-poll-interval",
		Usage:   "Polling interval for latest-block subscription when using an HTTP RPC provider.",
		EnvVars: prefixEnvVars("HTTP_POLL_INTERVAL"),
		Value:   config.DefaultPollInterval,
	}
	AdditionalBondClaimants = &cli.StringSliceFlag{
		Name:    "additional-bond-claimants",
		Usage:   "List of addresses to claim bonds for, in addition to the configured transaction sender",
		EnvVars: prefixEnvVars("ADDITIONAL_BOND_CLAIMANTS"),
	}
	PreStatesURLFlag = NewVMFlag("prestates-url", EnvVarPrefix, faultDisputeVMs, func(name string, envVars []string, traceTypeInfo string) cli.Flag {
		return &cli.StringFlag{
			Name: name,
			Usage: "Base URL to absolute prestates to use when generating trace data. " +
				"Prestates in this directory should be name as <commitment>.bin.gz <commitment>.json.gz or <commitment>.json " +
				traceTypeInfo,
			EnvVars: envVars,
		}
	})
	RollupConfigFlag = NewVMFlag("rollup-config", EnvVarPrefix, faultDisputeVMs, func(name string, envVars []string, traceTypeInfo string) cli.Flag {
		return &cli.StringSliceFlag{
			Name:    name,
			Usage:   "Rollup chain parameters " + traceTypeInfo,
			EnvVars: envVars,
		}
	})
	L2GenesisFlag = NewVMFlag("l2-genesis", EnvVarPrefix, faultDisputeVMs, func(name string, envVars []string, traceTypeInfo string) cli.Flag {
		return &cli.StringSliceFlag{
			Name:    name,
			Usage:   "Paths to the op-geth genesis file " + traceTypeInfo,
			EnvVars: envVars,
		}
	})
	CannonL2CustomFlag = &cli.BoolFlag{
		Name: "cannon-l2-custom",
		Usage: "Notify the op-program host that the L2 chain uses custom config to be loaded via the preimage oracle. " +
			"WARNING: This is incompatible with on-chain testing and must only be used for testing purposes.",
		EnvVars: prefixEnvVars("CANNON_L2_CUSTOM"),
		Value:   false,
		Hidden:  true,
	}
	CannonBinFlag = &cli.StringFlag{
		Name:    "cannon-bin",
		Usage:   "Path to cannon executable to use when generating trace data (cannon trace type only)",
		EnvVars: prefixEnvVars("CANNON_BIN"),
	}
	CannonServerFlag = &cli.StringFlag{
		Name:    "cannon-server",
		Usage:   "Path to executable to use as pre-image oracle server when generating trace data (cannon trace type only)",
		EnvVars: prefixEnvVars("CANNON_SERVER"),
	}
	CannonPreStateFlag = &cli.StringFlag{
		Name:    "cannon-prestate",
		Usage:   "Path to absolute prestate to use when generating trace data (cannon trace type only)",
		EnvVars: prefixEnvVars("CANNON_PRESTATE"),
	}
	CannonSnapshotFreqFlag = &cli.UintFlag{
		Name:    "cannon-snapshot-freq",
		Usage:   "Frequency of cannon snapshots to generate in VM steps (cannon trace type only)",
		EnvVars: prefixEnvVars("CANNON_SNAPSHOT_FREQ"),
		Value:   config.DefaultCannonSnapshotFreq,
	}
	CannonInfoFreqFlag = &cli.UintFlag{
		Name:    "cannon-info-freq",
		Usage:   "Frequency of cannon info log messages to generate in VM steps (cannon trace type only)",
		EnvVars: prefixEnvVars("CANNON_INFO_FREQ"),
		Value:   config.DefaultCannonInfoFreq,
	}
	AsteriscBinFlag = &cli.StringFlag{
		Name:    "asterisc-bin",
		Usage:   "Path to asterisc executable to use when generating trace data (asterisc trace type only)",
		EnvVars: prefixEnvVars("ASTERISC_BIN"),
	}
	AsteriscServerFlag = &cli.StringFlag{
		Name:    "asterisc-server",
		Usage:   "Path to executable to use as pre-image oracle server when generating trace data (asterisc trace type only)",
		EnvVars: prefixEnvVars("ASTERISC_SERVER"),
	}
	AsteriscKonaServerFlag = &cli.StringFlag{
		Name:    "asterisc-kona-server",
		Usage:   "Path to kona executable to use as pre-image oracle server when generating trace data (asterisc-kona trace type only)",
		EnvVars: prefixEnvVars("ASTERISC_KONA_SERVER"),
	}
	AsteriscPreStateFlag = &cli.StringFlag{
		Name:    "asterisc-prestate",
		Usage:   "Path to absolute prestate to use when generating trace data (asterisc trace type only)",
		EnvVars: prefixEnvVars("ASTERISC_PRESTATE"),
	}
	AsteriscKonaPreStateFlag = &cli.StringFlag{
		Name:    "asterisc-kona-prestate",
		Usage:   "Path to absolute prestate to use when generating trace data (asterisc-kona trace type only)",
		EnvVars: prefixEnvVars("ASTERISC_KONA_PRESTATE"),
	}
	AsteriscSnapshotFreqFlag = &cli.UintFlag{
		Name:    "asterisc-snapshot-freq",
		Usage:   "Frequency of asterisc snapshots to generate in VM steps (asterisc trace type only)",
		EnvVars: prefixEnvVars("ASTERISC_SNAPSHOT_FREQ"),
		Value:   config.DefaultAsteriscSnapshotFreq,
	}
	AsteriscInfoFreqFlag = &cli.UintFlag{
		Name:    "asterisc-info-freq",
		Usage:   "Frequency of asterisc info log messages to generate in VM steps (asterisc trace type only)",
		EnvVars: prefixEnvVars("ASTERISC_INFO_FREQ"),
		Value:   config.DefaultAsteriscInfoFreq,
	}
	GameWindowFlag = &cli.DurationFlag{
		Name: "game-window",
		Usage: "The time window which the challenger will look for games to progress and claim bonds. " +
			"This should include a buffer for the challenger to claim bonds for games outside the maximum game duration.",
		EnvVars: prefixEnvVars("GAME_WINDOW"),
		Value:   config.DefaultGameWindow,
	}
	SelectiveClaimResolutionFlag = &cli.BoolFlag{
		Name:    "selective-claim-resolution",
		Usage:   "Only resolve claims for the configured claimants",
		EnvVars: prefixEnvVars("SELECTIVE_CLAIM_RESOLUTION"),
	}
	UnsafeAllowInvalidPrestate = &cli.BoolFlag{
		Name:    "unsafe-allow-invalid-prestate",
		Usage:   "Allow responding to games where the absolute prestate is configured incorrectly. THIS IS UNSAFE!",
		EnvVars: prefixEnvVars("UNSAFE_ALLOW_INVALID_PRESTATE"),
		Hidden:  true, // Hidden as this is an unsafe flag added only for testing purposes
	}
)

// requiredFlags are checked by [CheckRequired]
var requiredFlags = []cli.Flag{
	L1EthRpcFlag,
	DatadirFlag,
	L1BeaconFlag,
}

// optionalFlags is a list of unchecked cli flags
var optionalFlags = []cli.Flag{
	RollupRpcFlag,
	NetworkFlag,
	FactoryAddressFlag,
	TraceTypeFlag,
	MaxConcurrencyFlag,
	SupervisorRpcFlag,
	L2EthRpcFlag,
	MaxPendingTransactionsFlag,
	HTTPPollInterval,
	AdditionalBondClaimants,
	GameAllowlistFlag,
	CannonL2CustomFlag,
	CannonBinFlag,
	CannonServerFlag,
	CannonPreStateFlag,
	CannonSnapshotFreqFlag,
	CannonInfoFreqFlag,
	AsteriscBinFlag,
	AsteriscServerFlag,
	AsteriscKonaServerFlag,
	AsteriscPreStateFlag,
	AsteriscKonaPreStateFlag,
	AsteriscSnapshotFreqFlag,
	AsteriscInfoFreqFlag,
	GameWindowFlag,
	SelectiveClaimResolutionFlag,
	UnsafeAllowInvalidPrestate,
}

func init() {
	optionalFlags = append(optionalFlags, oplog.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, PreStatesURLFlag.Flags()...)
	optionalFlags = append(optionalFlags, RollupConfigFlag.Flags()...)
	optionalFlags = append(optionalFlags, L2GenesisFlag.Flags()...)
	optionalFlags = append(optionalFlags, txmgr.CLIFlagsWithDefaults(EnvVarPrefix, txmgr.DefaultChallengerFlagValues)...)
	optionalFlags = append(optionalFlags, opmetrics.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oppprof.CLIFlags(EnvVarPrefix)...)

	Flags = append(requiredFlags, optionalFlags...)
}

// Flags contains the list of configuration options available to the binary.
var Flags []cli.Flag

func checkOutputProviderFlags(ctx *cli.Context) error {
	if !ctx.IsSet(RollupRpcFlag.Name) {
		return fmt.Errorf("flag %v is required", RollupRpcFlag.Name)
	}
	return nil
}

func CheckCannonBaseFlags(ctx *cli.Context) error {
	if !ctx.IsSet(flags.NetworkFlagName) &&
		!(RollupConfigFlag.IsSet(ctx, types.TraceTypeCannon) && L2GenesisFlag.IsSet(ctx, types.TraceTypeCannon)) {
		return fmt.Errorf("flag %v or %v and %v is required",
			flags.NetworkFlagName, RollupConfigFlag.EitherFlagName(types.TraceTypeCannon), L2GenesisFlag.EitherFlagName(types.TraceTypeCannon))
	}
	if ctx.IsSet(flags.NetworkFlagName) &&
		(RollupConfigFlag.IsSet(ctx, types.TraceTypeCannon) || L2GenesisFlag.IsSet(ctx, types.TraceTypeCannon) || ctx.Bool(CannonL2CustomFlag.Name)) {
		return fmt.Errorf("flag %v can not be used with %v, %v or %v",
			flags.NetworkFlagName, RollupConfigFlag.SourceFlagName(ctx, types.TraceTypeCannon), L2GenesisFlag.SourceFlagName(ctx, types.TraceTypeCannon), CannonL2CustomFlag.Name)
	}
	if ctx.Bool(CannonL2CustomFlag.Name) && !(RollupConfigFlag.IsSet(ctx, types.TraceTypeCannon) && L2GenesisFlag.IsSet(ctx, types.TraceTypeCannon)) {
		return fmt.Errorf("flag %v and %v must be set when %v is true",
			RollupConfigFlag.EitherFlagName(types.TraceTypeCannon), L2GenesisFlag.EitherFlagName(types.TraceTypeCannon), CannonL2CustomFlag.Name)
	}
	if !ctx.IsSet(CannonBinFlag.Name) {
		return fmt.Errorf("flag %s is required", CannonBinFlag.Name)
	}
	if !ctx.IsSet(CannonServerFlag.Name) {
		return fmt.Errorf("flag %s is required", CannonServerFlag.Name)
	}
	if !PreStatesURLFlag.IsSet(ctx, types.TraceTypeCannon) && !ctx.IsSet(CannonPreStateFlag.Name) {
		return fmt.Errorf("flag %s or %s is required", PreStatesURLFlag.EitherFlagName(types.TraceTypeCannon), CannonPreStateFlag.Name)
	}
	return nil
}

func CheckSuperCannonFlags(ctx *cli.Context) error {
	if !ctx.IsSet(SupervisorRpcFlag.Name) {
		return fmt.Errorf("flag %v is required", SupervisorRpcFlag.Name)
	}
	if err := CheckCannonBaseFlags(ctx); err != nil {
		return err
	}
	return nil
}

func CheckCannonFlags(ctx *cli.Context) error {
	if err := checkOutputProviderFlags(ctx); err != nil {
		return err
	}
	if err := CheckCannonBaseFlags(ctx); err != nil {
		return err
	}
	return nil
}

func CheckAsteriscBaseFlags(ctx *cli.Context, traceType types.TraceType) error {
	if err := checkOutputProviderFlags(ctx); err != nil {
		return err
	}
	if !ctx.IsSet(flags.NetworkFlagName) &&
		!(RollupConfigFlag.IsSet(ctx, traceType) && L2GenesisFlag.IsSet(ctx, traceType)) {
		return fmt.Errorf("flag %v or %v and %v is required",
			flags.NetworkFlagName, RollupConfigFlag.EitherFlagName(traceType), L2GenesisFlag.EitherFlagName(traceType))
	}
	if ctx.IsSet(flags.NetworkFlagName) &&
		(RollupConfigFlag.IsSet(ctx, traceType) || L2GenesisFlag.IsSet(ctx, traceType)) {
		return fmt.Errorf("flag %v can not be used with %v and %v",
			flags.NetworkFlagName, RollupConfigFlag.SourceFlagName(ctx, traceType), L2GenesisFlag.SourceFlagName(ctx, traceType))
	}
	if !ctx.IsSet(AsteriscBinFlag.Name) {
		return fmt.Errorf("flag %s is required", AsteriscBinFlag.Name)
	}
	return nil
}

func CheckAsteriscFlags(ctx *cli.Context) error {
	if err := CheckAsteriscBaseFlags(ctx, types.TraceTypeAsterisc); err != nil {
		return err
	}
	if !ctx.IsSet(AsteriscServerFlag.Name) {
		return fmt.Errorf("flag %s is required", AsteriscServerFlag.Name)
	}
	if !PreStatesURLFlag.IsSet(ctx, types.TraceTypeAsterisc) && !ctx.IsSet(AsteriscPreStateFlag.Name) {
		return fmt.Errorf("flag %s or %s is required", PreStatesURLFlag.EitherFlagName(types.TraceTypeAsterisc), AsteriscPreStateFlag.Name)
	}
	return nil
}

func CheckAsteriscKonaFlags(ctx *cli.Context) error {
	if err := CheckAsteriscBaseFlags(ctx, types.TraceTypeAsteriscKona); err != nil {
		return err
	}
	if !ctx.IsSet(AsteriscKonaServerFlag.Name) {
		return fmt.Errorf("flag %s is required", AsteriscKonaServerFlag.Name)
	}
	if !PreStatesURLFlag.IsSet(ctx, types.TraceTypeAsteriscKona) && !ctx.IsSet(AsteriscKonaPreStateFlag.Name) {
		return fmt.Errorf("flag %s or %s is required", PreStatesURLFlag.EitherFlagName(types.TraceTypeAsteriscKona), AsteriscKonaPreStateFlag.Name)
	}
	return nil
}

func CheckRequired(ctx *cli.Context, traceTypes []types.TraceType) error {
	for _, f := range requiredFlags {
		if !ctx.IsSet(f.Names()[0]) {
			return fmt.Errorf("flag %s is required", f.Names()[0])
		}
	}
	if !ctx.IsSet(L2EthRpcFlag.Name) {
		return fmt.Errorf("flag %s is required", L2EthRpcFlag.Name)
	}
	for _, traceType := range traceTypes {
		switch traceType {
		case types.TraceTypeCannon, types.TraceTypePermissioned:
			if err := CheckCannonFlags(ctx); err != nil {
				return err
			}
		case types.TraceTypeAsterisc:
			if err := CheckAsteriscFlags(ctx); err != nil {
				return err
			}
		case types.TraceTypeAsteriscKona:
			if err := CheckAsteriscKonaFlags(ctx); err != nil {
				return err
			}
		case types.TraceTypeSuperCannon:
			if err := CheckSuperCannonFlags(ctx); err != nil {
				return err
			}
		case types.TraceTypeAlphabet, types.TraceTypeFast:
			if err := checkOutputProviderFlags(ctx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid trace type %v. must be one of %v", traceType, types.TraceTypes)
		}
	}
	return nil
}

func parseTraceTypes(ctx *cli.Context) ([]types.TraceType, error) {
	var traceTypes []types.TraceType
	for _, typeName := range ctx.StringSlice(TraceTypeFlag.Name) {
		traceType := new(types.TraceType)
		if err := traceType.Set(typeName); err != nil {
			return nil, err
		}
		if !slices.Contains(traceTypes, *traceType) {
			traceTypes = append(traceTypes, *traceType)
		}
	}
	return traceTypes, nil
}

func FactoryAddress(ctx *cli.Context) (common.Address, error) {
	// Use FactoryAddressFlag in preference to Network. Allows overriding the default dispute game factory.
	if ctx.IsSet(FactoryAddressFlag.Name) {
		gameFactoryAddress, err := opservice.ParseAddress(ctx.String(FactoryAddressFlag.Name))
		if err != nil {
			return common.Address{}, err
		}
		return gameFactoryAddress, nil
	}
	networks := ctx.StringSlice(flags.NetworkFlagName)
	if len(networks) > 1 {
		return common.Address{}, fmt.Errorf("flag %v required when multiple networks specified", FactoryAddressFlag.Name)
	}
	if len(networks) == 0 {
		return common.Address{}, fmt.Errorf("flag %v or %v is required", FactoryAddressFlag.Name, flags.NetworkFlagName)
	}

	network := networks[0]
	chainCfg := chaincfg.ChainByName(network)
	if chainCfg == nil {
		var opts []string
		for _, cfg := range superchain.Chains {
			opts = append(opts, cfg.Name+"-"+cfg.Network)
		}
		return common.Address{}, fmt.Errorf("unknown chain: %v (Valid options: %v)", network, strings.Join(opts, ", "))
	}
	addrs := chainCfg.Addresses
	if addrs.DisputeGameFactoryProxy == nil {
		return common.Address{}, fmt.Errorf("dispute factory proxy not available for chain %v", network)
	}
	return *addrs.DisputeGameFactoryProxy, nil
}

// NewConfigFromCLI parses the Config from the provided flags or environment variables.
func NewConfigFromCLI(ctx *cli.Context, logger log.Logger) (*config.Config, error) {
	traceTypes, err := parseTraceTypes(ctx)
	if err != nil {
		return nil, err
	}
	if err := CheckRequired(ctx, traceTypes); err != nil {
		return nil, err
	}
	gameFactoryAddress, err := FactoryAddress(ctx)
	if err != nil {
		return nil, err
	}
	var allowedGames []common.Address
	if ctx.StringSlice(GameAllowlistFlag.Name) != nil {
		for _, addr := range ctx.StringSlice(GameAllowlistFlag.Name) {
			gameAddress, err := opservice.ParseAddress(addr)
			if err != nil {
				return nil, err
			}
			allowedGames = append(allowedGames, gameAddress)
		}
	}

	txMgrConfig := txmgr.ReadCLIConfig(ctx)
	metricsConfig := opmetrics.ReadCLIConfig(ctx)
	pprofConfig := oppprof.ReadCLIConfig(ctx)

	maxConcurrency := ctx.Uint(MaxConcurrencyFlag.Name)
	if maxConcurrency == 0 {
		return nil, fmt.Errorf("%v must not be 0", MaxConcurrencyFlag.Name)
	}
	var claimants []common.Address
	if ctx.IsSet(AdditionalBondClaimants.Name) {
		for _, addrStr := range ctx.StringSlice(AdditionalBondClaimants.Name) {
			claimant, err := opservice.ParseAddress(addrStr)
			if err != nil {
				return nil, fmt.Errorf("invalid additional claimant: %w", err)
			}
			claimants = append(claimants, claimant)
		}
	}

	getPrestatesUrl := func(traceType types.TraceType) (*url.URL, error) {
		var preStatesURL *url.URL
		if PreStatesURLFlag.IsSet(ctx, traceType) {
			val := PreStatesURLFlag.String(ctx, traceType)
			preStatesURL, err = url.Parse(val)
			if err != nil {
				return nil, fmt.Errorf("invalid %v (%v): %w", PreStatesURLFlag.SourceFlagName(ctx, traceType), val, err)
			}
		}
		return preStatesURL, nil
	}
	cannonPreStatesURL, err := getPrestatesUrl(types.TraceTypeCannon)
	if err != nil {
		return nil, err
	}
	asteriscPreStatesURL, err := getPrestatesUrl(types.TraceTypeAsterisc)
	if err != nil {
		return nil, err
	}
	asteriscKonaPreStatesURL, err := getPrestatesUrl(types.TraceTypeAsteriscKona)
	if err != nil {
		return nil, err
	}
	networks := ctx.StringSlice(flags.NetworkFlagName)
	l1EthRpc := ctx.String(L1EthRpcFlag.Name)
	l1Beacon := ctx.String(L1BeaconFlag.Name)
	l2Rpcs := ctx.StringSlice(L2EthRpcFlag.Name)
	return &config.Config{
		// Required Flags
		L1EthRpc:                l1EthRpc,
		L1Beacon:                l1Beacon,
		TraceTypes:              traceTypes,
		GameFactoryAddress:      gameFactoryAddress,
		GameAllowlist:           allowedGames,
		GameWindow:              ctx.Duration(GameWindowFlag.Name),
		MaxConcurrency:          maxConcurrency,
		L2Rpcs:                  l2Rpcs,
		MaxPendingTx:            ctx.Uint64(MaxPendingTransactionsFlag.Name),
		PollInterval:            ctx.Duration(HTTPPollInterval.Name),
		AdditionalBondClaimants: claimants,
		RollupRpc:               ctx.String(RollupRpcFlag.Name),
		SupervisorRPC:           ctx.String(SupervisorRpcFlag.Name),
		Cannon: vm.Config{
			VmType:            types.TraceTypeCannon,
			L1:                l1EthRpc,
			L1Beacon:          l1Beacon,
			L2s:               l2Rpcs,
			VmBin:             ctx.String(CannonBinFlag.Name),
			Server:            ctx.String(CannonServerFlag.Name),
			Networks:          networks,
			L2Custom:          ctx.Bool(CannonL2CustomFlag.Name),
			RollupConfigPaths: RollupConfigFlag.StringSlice(ctx, types.TraceTypeCannon),
			L2GenesisPaths:    L2GenesisFlag.StringSlice(ctx, types.TraceTypeCannon),
			SnapshotFreq:      ctx.Uint(CannonSnapshotFreqFlag.Name),
			InfoFreq:          ctx.Uint(CannonInfoFreqFlag.Name),
			DebugInfo:         true,
			BinarySnapshots:   true,
		},
		CannonAbsolutePreState:        ctx.String(CannonPreStateFlag.Name),
		CannonAbsolutePreStateBaseURL: cannonPreStatesURL,
		Datadir:                       ctx.String(DatadirFlag.Name),
		Asterisc: vm.Config{
			VmType:            types.TraceTypeAsterisc,
			L1:                l1EthRpc,
			L1Beacon:          l1Beacon,
			L2s:               l2Rpcs,
			VmBin:             ctx.String(AsteriscBinFlag.Name),
			Server:            ctx.String(AsteriscServerFlag.Name),
			Networks:          networks,
			RollupConfigPaths: RollupConfigFlag.StringSlice(ctx, types.TraceTypeAsterisc),
			L2GenesisPaths:    L2GenesisFlag.StringSlice(ctx, types.TraceTypeAsterisc),
			SnapshotFreq:      ctx.Uint(AsteriscSnapshotFreqFlag.Name),
			InfoFreq:          ctx.Uint(AsteriscInfoFreqFlag.Name),
			BinarySnapshots:   true,
		},
		AsteriscAbsolutePreState:        ctx.String(AsteriscPreStateFlag.Name),
		AsteriscAbsolutePreStateBaseURL: asteriscPreStatesURL,
		AsteriscKona: vm.Config{
			VmType:            types.TraceTypeAsteriscKona,
			L1:                l1EthRpc,
			L1Beacon:          l1Beacon,
			L2s:               l2Rpcs,
			VmBin:             ctx.String(AsteriscBinFlag.Name),
			Server:            ctx.String(AsteriscKonaServerFlag.Name),
			Networks:          networks,
			RollupConfigPaths: RollupConfigFlag.StringSlice(ctx, types.TraceTypeAsteriscKona),
			L2GenesisPaths:    L2GenesisFlag.StringSlice(ctx, types.TraceTypeAsteriscKona),
			SnapshotFreq:      ctx.Uint(AsteriscSnapshotFreqFlag.Name),
			InfoFreq:          ctx.Uint(AsteriscInfoFreqFlag.Name),
			BinarySnapshots:   true,
		},
		AsteriscKonaAbsolutePreState:        ctx.String(AsteriscKonaPreStateFlag.Name),
		AsteriscKonaAbsolutePreStateBaseURL: asteriscKonaPreStatesURL,
		TxMgrConfig:                         txMgrConfig,
		MetricsConfig:                       metricsConfig,
		PprofConfig:                         pprofConfig,
		SelectiveClaimResolution:            ctx.Bool(SelectiveClaimResolutionFlag.Name),
		AllowInvalidPrestate:                ctx.Bool(UnsafeAllowInvalidPrestate.Name),
	}, nil
}

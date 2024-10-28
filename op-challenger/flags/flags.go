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
	"github.com/ethereum-optimism/superchain-registry/superchain"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
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
	faultDisputeVMs = []types.TraceType{types.TraceTypeCannon, types.TraceTypeAsterisc, types.TraceTypeAsteriscKona}
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
	RollupRpcFlag = &cli.StringFlag{
		Name:    "rollup-rpc",
		Usage:   "HTTP provider URL for the rollup node",
		EnvVars: prefixEnvVars("ROLLUP_RPC"),
	}
	NetworkFlag        = flags.CLINetworkFlag(EnvVarPrefix, "")
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
		Value:   cli.NewStringSlice(types.TraceTypeCannon.String()),
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
	L2EthRpcFlag = &cli.StringFlag{
		Name:    "l2-eth-rpc",
		Usage:   "L2 Address of L2 JSON-RPC endpoint to use (eth and debug namespace required)  (cannon/asterisc trace type only)",
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
	CannonNetworkFlag = &cli.StringFlag{
		Name:    "cannon-network",
		Usage:   fmt.Sprintf("Deprecated: Use %v instead", flags.NetworkFlagName),
		EnvVars: prefixEnvVars("CANNON_NETWORK"),
	}
	CannonRollupConfigFlag = &cli.StringFlag{
		Name:    "cannon-rollup-config",
		Usage:   "Rollup chain parameters (cannon trace type only)",
		EnvVars: prefixEnvVars("CANNON_ROLLUP_CONFIG"),
	}
	CannonL2GenesisFlag = &cli.StringFlag{
		Name:    "cannon-l2-genesis",
		Usage:   "Path to the op-geth genesis file (cannon trace type only)",
		EnvVars: prefixEnvVars("CANNON_L2_GENESIS"),
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
	CannonL2Flag = &cli.StringFlag{
		Name:    "cannon-l2",
		Usage:   fmt.Sprintf("Deprecated: Use %v instead", L2EthRpcFlag.Name),
		EnvVars: prefixEnvVars("CANNON_L2"),
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
	AsteriscNetworkFlag = &cli.StringFlag{
		Name:    "asterisc-network",
		Usage:   fmt.Sprintf("Deprecated: Use %v instead", flags.NetworkFlagName),
		EnvVars: prefixEnvVars("ASTERISC_NETWORK"),
	}
	AsteriscRollupConfigFlag = &cli.StringFlag{
		Name:    "asterisc-rollup-config",
		Usage:   "Rollup chain parameters (asterisc trace type only)",
		EnvVars: prefixEnvVars("ASTERISC_ROLLUP_CONFIG"),
	}
	AsteriscL2GenesisFlag = &cli.StringFlag{
		Name:    "asterisc-l2-genesis",
		Usage:   "Path to the op-geth genesis file (asterisc trace type only)",
		EnvVars: prefixEnvVars("ASTERISC_L2_GENESIS"),
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
	RollupRpcFlag,
	L1BeaconFlag,
}

// optionalFlags is a list of unchecked cli flags
var optionalFlags = []cli.Flag{
	NetworkFlag,
	FactoryAddressFlag,
	TraceTypeFlag,
	MaxConcurrencyFlag,
	L2EthRpcFlag,
	MaxPendingTransactionsFlag,
	HTTPPollInterval,
	AdditionalBondClaimants,
	GameAllowlistFlag,
	CannonNetworkFlag,
	CannonRollupConfigFlag,
	CannonL2GenesisFlag,
	CannonBinFlag,
	CannonServerFlag,
	CannonPreStateFlag,
	CannonL2Flag,
	CannonSnapshotFreqFlag,
	CannonInfoFreqFlag,
	AsteriscNetworkFlag,
	AsteriscRollupConfigFlag,
	AsteriscL2GenesisFlag,
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
	optionalFlags = append(optionalFlags, txmgr.CLIFlagsWithDefaults(EnvVarPrefix, txmgr.DefaultChallengerFlagValues)...)
	optionalFlags = append(optionalFlags, opmetrics.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oppprof.CLIFlags(EnvVarPrefix)...)

	Flags = append(requiredFlags, optionalFlags...)
}

// Flags contains the list of configuration options available to the binary.
var Flags []cli.Flag

func CheckCannonFlags(ctx *cli.Context) error {
	if ctx.IsSet(CannonNetworkFlag.Name) && ctx.IsSet(flags.NetworkFlagName) {
		return fmt.Errorf("flag %v can not be used with %v", CannonNetworkFlag.Name, flags.NetworkFlagName)
	}
	if !ctx.IsSet(CannonNetworkFlag.Name) &&
		!ctx.IsSet(flags.NetworkFlagName) &&
		!(ctx.IsSet(CannonRollupConfigFlag.Name) && ctx.IsSet(CannonL2GenesisFlag.Name)) {
		return fmt.Errorf("flag %v, %v or %v and %v is required",
			CannonNetworkFlag.Name, flags.NetworkFlagName, CannonRollupConfigFlag.Name, CannonL2GenesisFlag.Name)
	}
	if ctx.IsSet(flags.NetworkFlagName) &&
		(ctx.IsSet(CannonRollupConfigFlag.Name) || ctx.IsSet(CannonL2GenesisFlag.Name)) {
		return fmt.Errorf("flag %v can not be used with %v and %v",
			flags.NetworkFlagName, CannonRollupConfigFlag.Name, CannonL2GenesisFlag.Name)
	}
	if ctx.IsSet(CannonNetworkFlag.Name) &&
		(ctx.IsSet(CannonRollupConfigFlag.Name) || ctx.IsSet(CannonL2GenesisFlag.Name)) {
		return fmt.Errorf("flag %v can not be used with %v and %v",
			CannonNetworkFlag.Name, CannonRollupConfigFlag.Name, CannonL2GenesisFlag.Name)
	}
	if !ctx.IsSet(CannonBinFlag.Name) {
		return fmt.Errorf("flag %s is required", CannonBinFlag.Name)
	}
	if !ctx.IsSet(CannonServerFlag.Name) {
		return fmt.Errorf("flag %s is required", CannonServerFlag.Name)
	}
	if !PreStatesURLFlag.IsSet(ctx, types.TraceTypeCannon) && !ctx.IsSet(CannonPreStateFlag.Name) {
		return fmt.Errorf("flag %s or %s is required", PreStatesURLFlag.DefaultName(), CannonPreStateFlag.Name)
	}
	return nil
}

func CheckAsteriscBaseFlags(ctx *cli.Context) error {
	if ctx.IsSet(AsteriscNetworkFlag.Name) && ctx.IsSet(flags.NetworkFlagName) {
		return fmt.Errorf("flag %v can not be used with %v", AsteriscNetworkFlag.Name, flags.NetworkFlagName)
	}
	if !ctx.IsSet(AsteriscNetworkFlag.Name) &&
		!ctx.IsSet(flags.NetworkFlagName) &&
		!(ctx.IsSet(AsteriscRollupConfigFlag.Name) && ctx.IsSet(AsteriscL2GenesisFlag.Name)) {
		return fmt.Errorf("flag %v, %v or %v and %v is required",
			AsteriscNetworkFlag.Name, flags.NetworkFlagName, AsteriscRollupConfigFlag.Name, AsteriscL2GenesisFlag.Name)
	}
	if ctx.IsSet(flags.NetworkFlagName) &&
		(ctx.IsSet(AsteriscRollupConfigFlag.Name) || ctx.IsSet(AsteriscL2GenesisFlag.Name)) {
		return fmt.Errorf("flag %v can not be used with %v and %v",
			flags.NetworkFlagName, AsteriscRollupConfigFlag.Name, AsteriscL2GenesisFlag.Name)
	}
	if ctx.IsSet(AsteriscNetworkFlag.Name) &&
		(ctx.IsSet(AsteriscRollupConfigFlag.Name) || ctx.IsSet(AsteriscL2GenesisFlag.Name)) {
		return fmt.Errorf("flag %v can not be used with %v and %v",
			AsteriscNetworkFlag.Name, AsteriscRollupConfigFlag.Name, AsteriscL2GenesisFlag.Name)
	}
	if !ctx.IsSet(AsteriscBinFlag.Name) {
		return fmt.Errorf("flag %s is required", AsteriscBinFlag.Name)
	}
	return nil
}

func CheckAsteriscFlags(ctx *cli.Context) error {
	if err := CheckAsteriscBaseFlags(ctx); err != nil {
		return err
	}
	if !ctx.IsSet(AsteriscServerFlag.Name) {
		return fmt.Errorf("flag %s is required", AsteriscServerFlag.Name)
	}
	if !PreStatesURLFlag.IsSet(ctx, types.TraceTypeAsterisc) && !ctx.IsSet(AsteriscPreStateFlag.Name) {
		return fmt.Errorf("flag %s or %s is required", PreStatesURLFlag.DefaultName(), AsteriscPreStateFlag.Name)
	}
	return nil
}

func CheckAsteriscKonaFlags(ctx *cli.Context) error {
	if err := CheckAsteriscBaseFlags(ctx); err != nil {
		return err
	}
	if !ctx.IsSet(AsteriscKonaServerFlag.Name) {
		return fmt.Errorf("flag %s is required", AsteriscKonaServerFlag.Name)
	}
	if !PreStatesURLFlag.IsSet(ctx, types.TraceTypeAsteriscKona) && !ctx.IsSet(AsteriscKonaPreStateFlag.Name) {
		return fmt.Errorf("flag %s or %s is required", PreStatesURLFlag.DefaultName(), AsteriscKonaPreStateFlag.Name)
	}
	return nil
}

func CheckRequired(ctx *cli.Context, traceTypes []types.TraceType) error {
	for _, f := range requiredFlags {
		if !ctx.IsSet(f.Names()[0]) {
			return fmt.Errorf("flag %s is required", f.Names()[0])
		}
	}
	// CannonL2Flag is checked because it is an alias with L2EthRpcFlag
	if !ctx.IsSet(CannonL2Flag.Name) && !ctx.IsSet(L2EthRpcFlag.Name) {
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
		case types.TraceTypeAlphabet, types.TraceTypeFast:
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

func getL2Rpc(ctx *cli.Context, logger log.Logger) (string, error) {
	if ctx.IsSet(CannonL2Flag.Name) && ctx.IsSet(L2EthRpcFlag.Name) {
		return "", fmt.Errorf("flag %v and %v must not be both set", CannonL2Flag.Name, L2EthRpcFlag.Name)
	}
	l2Rpc := ""
	if ctx.IsSet(CannonL2Flag.Name) {
		logger.Warn(fmt.Sprintf("flag %v is deprecated, please use %v", CannonL2Flag.Name, L2EthRpcFlag.Name))
		l2Rpc = ctx.String(CannonL2Flag.Name)
	}
	if ctx.IsSet(L2EthRpcFlag.Name) {
		l2Rpc = ctx.String(L2EthRpcFlag.Name)
	}
	return l2Rpc, nil
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
	if ctx.IsSet(flags.NetworkFlagName) {
		chainName := ctx.String(flags.NetworkFlagName)
		chainCfg := chaincfg.ChainByName(chainName)
		if chainCfg == nil {
			var opts []string
			for _, cfg := range superchain.OPChains {
				opts = append(opts, cfg.Chain+"-"+cfg.Superchain)
			}
			return common.Address{}, fmt.Errorf("unknown chain: %v (Valid options: %v)", chainName, strings.Join(opts, ", "))
		}
		addrs, ok := superchain.Addresses[chainCfg.ChainID]
		if !ok {
			return common.Address{}, fmt.Errorf("no addresses available for chain %v", chainName)
		}
		if addrs.DisputeGameFactoryProxy == (superchain.Address{}) {
			return common.Address{}, fmt.Errorf("dispute factory proxy not available for chain %v", chainName)
		}
		return common.Address(addrs.DisputeGameFactoryProxy), nil
	}
	return common.Address{}, fmt.Errorf("flag %v or %v is required", FactoryAddressFlag.Name, flags.NetworkFlagName)
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
	var cannonPreStatesURL *url.URL
	if PreStatesURLFlag.IsSet(ctx, types.TraceTypeCannon) {
		val := PreStatesURLFlag.String(ctx, types.TraceTypeCannon)
		cannonPreStatesURL, err = url.Parse(val)
		if err != nil {
			return nil, fmt.Errorf("invalid %v (%v): %w", PreStatesURLFlag.SourceFlagName(ctx, types.TraceTypeCannon), val, err)
		}
	}
	var asteriscPreStatesURL *url.URL
	if PreStatesURLFlag.IsSet(ctx, types.TraceTypeAsterisc) {
		val := PreStatesURLFlag.String(ctx, types.TraceTypeAsterisc)
		asteriscPreStatesURL, err = url.Parse(val)
		if err != nil {
			return nil, fmt.Errorf("invalid %v (%v): %w", PreStatesURLFlag.SourceFlagName(ctx, types.TraceTypeAsterisc), val, err)
		}
	}
	var asteriscKonaPreStatesURL *url.URL
	if PreStatesURLFlag.IsSet(ctx, types.TraceTypeAsteriscKona) {
		val := PreStatesURLFlag.String(ctx, types.TraceTypeAsteriscKona)
		asteriscKonaPreStatesURL, err = url.Parse(val)
		if err != nil {
			return nil, fmt.Errorf("invalid %v (%v): %w", PreStatesURLFlag.SourceFlagName(ctx, types.TraceTypeAsteriscKona), val, err)
		}
	}
	l2Rpc, err := getL2Rpc(ctx, logger)
	if err != nil {
		return nil, err
	}
	cannonNetwork := ctx.String(CannonNetworkFlag.Name)
	if ctx.IsSet(flags.NetworkFlagName) {
		cannonNetwork = ctx.String(flags.NetworkFlagName)
	}
	asteriscNetwork := ctx.String(AsteriscNetworkFlag.Name)
	if ctx.IsSet(flags.NetworkFlagName) {
		asteriscNetwork = ctx.String(flags.NetworkFlagName)
	}
	l1EthRpc := ctx.String(L1EthRpcFlag.Name)
	l1Beacon := ctx.String(L1BeaconFlag.Name)
	return &config.Config{
		// Required Flags
		L1EthRpc:                l1EthRpc,
		L1Beacon:                l1Beacon,
		TraceTypes:              traceTypes,
		GameFactoryAddress:      gameFactoryAddress,
		GameAllowlist:           allowedGames,
		GameWindow:              ctx.Duration(GameWindowFlag.Name),
		MaxConcurrency:          maxConcurrency,
		L2Rpc:                   l2Rpc,
		MaxPendingTx:            ctx.Uint64(MaxPendingTransactionsFlag.Name),
		PollInterval:            ctx.Duration(HTTPPollInterval.Name),
		AdditionalBondClaimants: claimants,
		RollupRpc:               ctx.String(RollupRpcFlag.Name),
		Cannon: vm.Config{
			VmType:           types.TraceTypeCannon,
			L1:               l1EthRpc,
			L1Beacon:         l1Beacon,
			L2:               l2Rpc,
			VmBin:            ctx.String(CannonBinFlag.Name),
			Server:           ctx.String(CannonServerFlag.Name),
			Network:          cannonNetwork,
			RollupConfigPath: ctx.String(CannonRollupConfigFlag.Name),
			L2GenesisPath:    ctx.String(CannonL2GenesisFlag.Name),
			SnapshotFreq:     ctx.Uint(CannonSnapshotFreqFlag.Name),
			InfoFreq:         ctx.Uint(CannonInfoFreqFlag.Name),
			DebugInfo:        true,
			BinarySnapshots:  true,
		},
		CannonAbsolutePreState:        ctx.String(CannonPreStateFlag.Name),
		CannonAbsolutePreStateBaseURL: cannonPreStatesURL,
		Datadir:                       ctx.String(DatadirFlag.Name),
		Asterisc: vm.Config{
			VmType:           types.TraceTypeAsterisc,
			L1:               l1EthRpc,
			L1Beacon:         l1Beacon,
			L2:               l2Rpc,
			VmBin:            ctx.String(AsteriscBinFlag.Name),
			Server:           ctx.String(AsteriscServerFlag.Name),
			Network:          asteriscNetwork,
			RollupConfigPath: ctx.String(AsteriscRollupConfigFlag.Name),
			L2GenesisPath:    ctx.String(AsteriscL2GenesisFlag.Name),
			SnapshotFreq:     ctx.Uint(AsteriscSnapshotFreqFlag.Name),
			InfoFreq:         ctx.Uint(AsteriscInfoFreqFlag.Name),
			BinarySnapshots:  true,
		},
		AsteriscAbsolutePreState:        ctx.String(AsteriscPreStateFlag.Name),
		AsteriscAbsolutePreStateBaseURL: asteriscPreStatesURL,
		AsteriscKona: vm.Config{
			VmType:           types.TraceTypeAsteriscKona,
			L1:               l1EthRpc,
			L1Beacon:         l1Beacon,
			L2:               l2Rpc,
			VmBin:            ctx.String(AsteriscBinFlag.Name),
			Server:           ctx.String(AsteriscKonaServerFlag.Name),
			Network:          asteriscNetwork,
			RollupConfigPath: ctx.String(AsteriscRollupConfigFlag.Name),
			L2GenesisPath:    ctx.String(AsteriscL2GenesisFlag.Name),
			SnapshotFreq:     ctx.Uint(AsteriscSnapshotFreqFlag.Name),
			InfoFreq:         ctx.Uint(AsteriscInfoFreqFlag.Name),
			BinarySnapshots:  true,
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

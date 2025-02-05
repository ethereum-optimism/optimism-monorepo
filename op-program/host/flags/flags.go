package flags

import (
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	service "github.com/ethereum-optimism/optimism/op-service"
	openum "github.com/ethereum-optimism/optimism/op-service/enum"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

const EnvVarPrefix = "OP_PROGRAM"

func prefixEnvVars(name string) []string {
	return service.PrefixEnvVar(EnvVarPrefix, name)
}

var (
	L2Custom = &cli.BoolFlag{
		Name: "l2.custom",
		Usage: "Override the L2 chain ID to the custom chain indicator for custom chain configuration not present in the client program. " +
			"WARNING: This is not compatible with on-chain execution and must only be used for testing.",
		EnvVars: prefixEnvVars("L2_CHAINID"),
		Value:   false,
		Hidden:  true,
	}
	RollupConfig = &cli.StringSliceFlag{
		Name:    "rollup.config",
		Usage:   "Rollup chain parameters",
		EnvVars: prefixEnvVars("ROLLUP_CONFIG"),
	}
	Network = &cli.StringSliceFlag{
		Name:    "network",
		Usage:   fmt.Sprintf("Predefined network selection. Available networks: %s", strings.Join(chaincfg.AvailableNetworks(), ", ")),
		EnvVars: prefixEnvVars("NETWORK"),
	}
	DataDir = &cli.StringFlag{
		Name:    "datadir",
		Usage:   "Directory to use for preimage data storage. Default uses in-memory storage",
		EnvVars: prefixEnvVars("DATADIR"),
	}
	DataFormat = &cli.StringFlag{
		Name:    "data.format",
		Usage:   fmt.Sprintf("Format to use for preimage data storage. Available formats: %s", openum.EnumString(types.SupportedDataFormats)),
		EnvVars: prefixEnvVars("DATA_FORMAT"),
		Value:   string(types.DataFormatDirectory),
	}
	L2NodeAddr = &cli.StringSliceFlag{
		Name:    "l2",
		Usage:   "Address of L2 JSON-RPC endpoint to use (eth and debug namespace required)",
		EnvVars: prefixEnvVars("L2_RPC"),
	}
	L2NodeExperimentalAddr = &cli.StringSliceFlag{
		Name:    "l2.experimental",
		Usage:   "Address of L2 JSON-RPC endpoint to use for experimental features (debug_executionWitness)",
		EnvVars: prefixEnvVars("L2_RPC_EXPERIMENTAL_RPC"),
	}
	L1Head = &cli.StringFlag{
		Name:    "l1.head",
		Usage:   "Hash of the L1 head block. Derivation stops after this block is processed.",
		EnvVars: prefixEnvVars("L1_HEAD"),
	}
	L2Head = &cli.StringFlag{
		Name:    "l2.head",
		Usage:   "Hash of the L2 block at l2.outputroot",
		EnvVars: prefixEnvVars("L2_HEAD"),
	}
	L2OutputRoot = &cli.StringFlag{
		Name:    "l2.outputroot",
		Usage:   "Agreed L2 Output Root to start derivation from. Used for non-interop games.",
		EnvVars: prefixEnvVars("L2_OUTPUT_ROOT"),
	}
	L2AgreedPrestate = &cli.StringFlag{
		Name: "l2.agreed-prestate",
		Usage: "Agreed L2 pre state pre-image to start derivation from. " +
			"l2.outputroot will be automatically set to the hash of the prestate. Used for interop-enabled games.",
		EnvVars: prefixEnvVars("L2_AGREED_PRESTATE"),
	}
	L2Claim = &cli.StringFlag{
		Name:    "l2.claim",
		Usage:   "Claimed L2 output root to validate",
		EnvVars: prefixEnvVars("L2_CLAIM"),
	}
	L2BlockNumber = &cli.Uint64Flag{
		Name:    "l2.blocknumber",
		Usage:   "Number of the L2 block that the claim is from",
		EnvVars: prefixEnvVars("L2_BLOCK_NUM"),
	}
	L2GenesisPath = &cli.StringSliceFlag{
		Name:    "l2.genesis",
		Usage:   "Path to the op-geth genesis file",
		EnvVars: prefixEnvVars("L2_GENESIS"),
	}
	L1NodeAddr = &cli.StringFlag{
		Name:    "l1",
		Usage:   "Address of L1 JSON-RPC endpoint to use (eth namespace required)",
		EnvVars: prefixEnvVars("L1_RPC"),
	}
	L1BeaconAddr = &cli.StringFlag{
		Name:    "l1.beacon",
		Usage:   "Address of L1 Beacon API endpoint to use",
		EnvVars: prefixEnvVars("L1_BEACON_API"),
	}
	L1TrustRPC = &cli.BoolFlag{
		Name:    "l1.trustrpc",
		Usage:   "Trust the L1 RPC, sync faster at risk of malicious/buggy RPC providing bad or inconsistent L1 data",
		EnvVars: prefixEnvVars("L1_TRUST_RPC"),
	}
	L1RPCProviderKind = &cli.GenericFlag{
		Name: "l1.rpckind",
		Usage: "The kind of RPC provider, used to inform optimal transactions receipts fetching, and thus reduce costs. Valid options: " +
			openum.EnumString(sources.RPCProviderKinds),
		EnvVars: prefixEnvVars("L1_RPC_KIND"),
		Value: func() *sources.RPCProviderKind {
			out := sources.RPCKindStandard
			return &out
		}(),
	}
	Exec = &cli.StringFlag{
		Name:    "exec",
		Usage:   "Run the specified client program as a separate process detached from the host. Default is to run the client program in the host process.",
		EnvVars: prefixEnvVars("EXEC"),
	}
	Server = &cli.BoolFlag{
		Name:    "server",
		Usage:   "Run in pre-image server mode without executing any client program.",
		EnvVars: prefixEnvVars("SERVER"),
	}
)

// Flags contains the list of configuration options available to the binary.
var Flags []cli.Flag

var requiredFlags = []cli.Flag{
	L1Head,
	L2Claim,
	L2BlockNumber,
}

var programFlags = []cli.Flag{
	L2Head,
	L2OutputRoot,
	L2AgreedPrestate,
	L2Custom,
	RollupConfig,
	Network,
	DataDir,
	DataFormat,
	L2NodeAddr,
	L2NodeExperimentalAddr,
	L2GenesisPath,
	L1NodeAddr,
	L1BeaconAddr,
	L1TrustRPC,
	L1RPCProviderKind,
	Exec,
	Server,
}

func init() {
	Flags = append(Flags, oplog.CLIFlags(EnvVarPrefix)...)
	Flags = append(Flags, requiredFlags...)
	Flags = append(Flags, programFlags...)
}

func CheckRequired(ctx *cli.Context) error {
	if ctx.Bool(L2Custom.Name) && ctx.IsSet(Network.Name) {
		return fmt.Errorf("flag %s cannot be used with named networks", L2Custom.Name)
	}
	for _, flag := range requiredFlags {
		if !ctx.IsSet(flag.Names()[0]) {
			return fmt.Errorf("flag %s is required", flag.Names()[0])
		}
	}
	if !ctx.IsSet(L2OutputRoot.Name) && !ctx.IsSet(L2AgreedPrestate.Name) {
		return fmt.Errorf("flag %s or %s is required", L2OutputRoot.Name, L2AgreedPrestate.Name)
	}
	if ctx.IsSet(L2OutputRoot.Name) && ctx.IsSet(L2AgreedPrestate.Name) {
		return fmt.Errorf("flag %s and %s must not be specified together", L2OutputRoot.Name, L2AgreedPrestate.Name)
	}
	if ctx.IsSet(L2Head.Name) && ctx.IsSet(L2AgreedPrestate.Name) {
		return fmt.Errorf("flag %s and %s must not be specified together", L2Head.Name, L2AgreedPrestate.Name)
	}
	if !ctx.IsSet(L2Head.Name) && ctx.IsSet(L2OutputRoot.Name) {
		return fmt.Errorf("flag %s is required when %s is specified", L2Head.Name, L2OutputRoot.Name)
	}
	return nil
}

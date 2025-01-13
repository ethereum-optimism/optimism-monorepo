package flags

import (
	"fmt"

	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	opflags "github.com/ethereum-optimism/optimism/op-service/flags"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
)

const EnvVarPrefix = "OP_NAT"

var (
	KurtosisDevnetManifest = &cli.StringFlag{
		Name:    "kurtosis.devnet.manifest",
		Value:   "",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "KURTOSIS_DEVNET_MANIFEST"),
		Usage:   "Path to the kurtosis-devnet manifest",
	}
	ExecutionRPC = &cli.StringFlag{
		Name:    "rpc.execution",
		Value:   "http://127.0.0.1:8545",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "RPC_EXECUTION"),
		Usage:   "Network Execution Layer RPC URL",
	}
	SenderSecretKey = &cli.StringFlag{
		Name:    "sender.key.secret",
		Value:   "",
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "SENDER_KEY_SECRET"),
		Usage:   "Sender secret key",
	}
	ReceiverPublicKeys = &cli.StringSliceFlag{
		Name:    "receiver.key.public",
		Value:   cli.NewStringSlice("0x9B383f8e4Cd5d3DD5F9006B6A508960A1e730375"), // OP-Sepolia Whale
		EnvVars: opservice.PrefixEnvVar(EnvVarPrefix, "RECEIVER_KEY_PUBLIC"),
		Usage:   "Receiver public keys",
	}
)

var requiredFlags = []cli.Flag{
	KurtosisDevnetManifest,
	SenderSecretKey,
}

var optionalFlags = []cli.Flag{
	ExecutionRPC,
	ReceiverPublicKeys,
}

func init() {
	optionalFlags = append(optionalFlags, oprpc.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oplog.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, opmetrics.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, oppprof.CLIFlags(EnvVarPrefix)...)
	optionalFlags = append(optionalFlags, opflags.CLIFlags(EnvVarPrefix, "")...)

	Flags = append(requiredFlags, optionalFlags...)
}

var Flags []cli.Flag

func CheckRequired(ctx *cli.Context) error {
	for _, f := range requiredFlags {
		if !ctx.IsSet(f.Names()[0]) {
			return fmt.Errorf("flag %s is required", f.Names()[0])
		}
	}
	return opflags.CheckRequiredXor(ctx)
}

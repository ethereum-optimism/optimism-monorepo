package wheel

import (
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	opservice "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/client"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-wheel/cheat"
	"github.com/ethereum-optimism/optimism/op-wheel/engine"
)

const envVarPrefix = "OP_WHEEL"

func prefixEnvVars(name string) []string {
	return []string{envVarPrefix + "_" + name}
}

var (
	GlobalGethLogLvlFlag = &cli.GenericFlag{
		Name:    "geth-log-level",
		Usage:   "Set the global geth logging level",
		EnvVars: prefixEnvVars("GETH_LOG_LEVEL"),
		Value:   oplog.NewLevelFlagValue(log.LevelError),
	}
	DataDirFlag = &cli.StringFlag{
		Name:      "data-dir",
		Usage:     "Geth data dir location.",
		Required:  true,
		TakesFile: true,
		EnvVars:   prefixEnvVars("DATA_DIR"),
	}
	EngineEndpoint = &cli.StringFlag{
		Name:     "engine",
		Usage:    "Authenticated Engine API RPC endpoint, can be HTTP/WS/IPC",
		Required: true,
		Value:    "http://localhost:8551/",
		EnvVars:  prefixEnvVars("ENGINE"),
	}
	EngineJWT = &cli.StringFlag{
		Name:    "engine.jwt-secret",
		Usage:   "JWT secret used to authenticate Engine API communication with. Takes precedence over engine.jwt-secret-path.",
		EnvVars: prefixEnvVars("ENGINE_JWT_SECRET"),
	}
	EngineJWTPath = &cli.StringFlag{
		Name:      "engine.jwt-secret-path",
		Usage:     "Path to JWT secret file used to authenticate Engine API communication with.",
		TakesFile: true,
		EnvVars:   prefixEnvVars("ENGINE_JWT_SECRET_PATH"),
	}
	EngineOpenEndpoint = &cli.StringFlag{
		Name:    "engine.open",
		Usage:   "Open Engine API RPC endpoint, can be HTTP/WS/IPC",
		Value:   "http://localhost:8545/",
		EnvVars: prefixEnvVars("ENGINE_OPEN"),
	}
	EngineVersion = &cli.IntFlag{
		Name:    "engine.version",
		Usage:   "Engine API version to use for Engine calls (1, 2, or 3)",
		EnvVars: prefixEnvVars("ENGINE_VERSION"),
		Action: func(ctx *cli.Context, ev int) error {
			if ev < 1 || ev > 3 {
				return fmt.Errorf("invalid Engine API version: %d", ev)
			}
			return nil
		},
	}
	FeeRecipientFlag = &cli.GenericFlag{
		Name:    "fee-recipient",
		Usage:   "fee-recipient of the block building",
		EnvVars: prefixEnvVars("FEE_RECIPIENT"),
		Value:   &TextFlag[*common.Address]{Value: &common.Address{1: 0x13, 2: 0x37}},
	}
	RandaoFlag = &cli.GenericFlag{
		Name:    "randao",
		Usage:   "randao value of the block building",
		EnvVars: prefixEnvVars("RANDAO"),
		Value:   &TextFlag[*common.Hash]{Value: &common.Hash{1: 0x13, 2: 0x37}},
	}
	BlockTimeFlag = &cli.Uint64Flag{
		Name:    "block-time",
		Usage:   "block time, interval of timestamps between blocks to build, in seconds",
		EnvVars: prefixEnvVars("BLOCK_TIME"),
		Value:   12,
	}
	BuildingTime = &cli.DurationFlag{
		Name:    "building-time",
		Usage:   "duration of of block building, this should be set to something lower than the block time.",
		EnvVars: prefixEnvVars("BUILDING_TIME"),
		Value:   time.Second * 6,
	}
	AllowGaps = &cli.BoolFlag{
		Name:    "allow-gaps",
		Usage:   "allow gaps in block building, like missed slots on the beacon chain.",
		EnvVars: prefixEnvVars("ALLOW_GAPS"),
	}
)

func withEngineFlags(flags ...cli.Flag) []cli.Flag {
	return append(append(flags,
		EngineEndpoint, EngineJWT, EngineJWTPath, EngineOpenEndpoint, EngineVersion),
		oplog.CLIFlags(envVarPrefix)...)
}

func ParseBuildingArgs(ctx *cli.Context) *engine.BlockBuildingSettings {
	return &engine.BlockBuildingSettings{
		BlockTime:    ctx.Uint64(BlockTimeFlag.Name),
		AllowGaps:    ctx.Bool(AllowGaps.Name),
		Random:       hashFlagValue(RandaoFlag.Name, ctx),
		FeeRecipient: addrFlagValue(FeeRecipientFlag.Name, ctx),
		BuildTime:    ctx.Duration(BuildingTime.Name),
	}
}

func CheatAction(readOnly bool, fn func(ctx *cli.Context, ch *cheat.Cheater) error) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		dataDir := ctx.String(DataDirFlag.Name)
		ch, err := cheat.OpenGethDB(dataDir, readOnly)
		if err != nil {
			return fmt.Errorf("failed to open geth db: %w", err)
		}
		return fn(ctx, ch)
	}
}

func CheatRawDBAction(readOnly bool, fn func(ctx *cli.Context, db ethdb.Database) error) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		dataDir := ctx.String(DataDirFlag.Name)
		db, err := cheat.OpenGethRawDB(dataDir, readOnly)
		if err != nil {
			return fmt.Errorf("failed to open raw geth db: %w", err)
		}
		return fn(ctx, db)
	}
}

func EngineAction(fn func(ctx *cli.Context, client *sources.EngineAPIClient, lgr log.Logger) error) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		lgr := initLogger(ctx)
		rpc, err := initEngineRPC(ctx, lgr)
		if err != nil {
			return fmt.Errorf("failed to dial Engine API endpoint %q: %w",
				ctx.String(EngineEndpoint.Name), err)
		}
		evp, err := initVersionProvider(ctx, lgr)
		if err != nil {
			return fmt.Errorf("failed to init Engine version provider: %w", err)
		}
		client := sources.NewEngineAPIClient(rpc, lgr, evp)
		return fn(ctx, client, lgr)
	}
}

func initLogger(ctx *cli.Context) log.Logger {
	logCfg := oplog.ReadCLIConfig(ctx)
	lgr := oplog.NewLogger(oplog.AppOut(ctx), logCfg)
	oplog.SetGlobalLogHandler(lgr.Handler())
	return lgr
}

func initEngineRPC(ctx *cli.Context, lgr log.Logger) (client.RPC, error) {
	jwtString := ctx.String(EngineJWT.Name) // no IsSet check; allow empty value to be overridden
	if jwtString == "" {
		if ctx.IsSet(EngineJWTPath.Name) {
			jwtData, err := os.ReadFile(ctx.String(EngineJWTPath.Name))
			if err != nil {
				return nil, fmt.Errorf("failed to read jwt: %w", err)
			}
			jwtString = string(jwtData)
		} else {
			return nil, errors.New("neither JWT secret string nor path provided")
		}
	}
	secret, err := parseJWTSecret(jwtString)
	if err != nil {
		return nil, err
	}
	endpoint := ctx.String(EngineEndpoint.Name)
	return client.NewRPC(ctx.Context, lgr, endpoint,
		client.WithGethRPCOptions(rpc.WithHTTPAuth(node.NewJWTAuth(secret))))
}

func parseJWTSecret(v string) (common.Hash, error) {
	v = strings.TrimSpace(v)
	v = "0x" + strings.TrimPrefix(v, "0x") // ensure prefix is there
	var out common.Hash
	if err := out.UnmarshalText([]byte(v)); err != nil {
		return common.Hash{}, fmt.Errorf("failed to parse JWT secret: %w", err)
	}
	return out, nil
}

func initVersionProvider(ctx *cli.Context, lgr log.Logger) (sources.EngineVersionProvider, error) {
	// static configuration takes precedent, if set
	if ctx.IsSet(EngineVersion.Name) {
		ev := ctx.Int(EngineVersion.Name)
		return engine.StaticVersionProvider(ev), nil
	}

	// otherwise get config from EL
	rpc, err := initOpenEngineRPC(ctx, lgr)
	if err != nil {
		return nil, err
	}

	cfg, err := engine.GetChainConfig(ctx.Context, rpc)
	if err != nil {
		return nil, err
	}
	return rollupFromGethConfig(cfg), nil
}

func initOpenEngineRPC(ctx *cli.Context, lgr log.Logger) (client.RPC, error) {
	openEP := ctx.String(EngineOpenEndpoint.Name)
	rpc, err := client.NewRPC(ctx.Context, lgr, openEP)
	if err != nil {
		return nil, fmt.Errorf("failed to dial open Engine endpoint %q: %w", openEP, err)
	}
	return rpc, nil
}

// rollupFromGethConfig returns a very incomplete rollup config with only the
// L2ChainID and (most) fork activation timestamps set.
//
// Because Delta was a pure CL fork, its time isn't set either.
//
// This incomplete [rollup.Config] can be used as a [sources.EngineVersionProvider].
func rollupFromGethConfig(cfg *params.ChainConfig) *rollup.Config {
	return &rollup.Config{
		L2ChainID: cfg.ChainID,

		RegolithTime: cfg.RegolithTime,
		CanyonTime:   cfg.CanyonTime,
		EcotoneTime:  cfg.EcotoneTime,
		GraniteTime:  cfg.GraniteTime,
		HoloceneTime: cfg.HoloceneTime,
		IsthmusTime:  cfg.IsthmusTime,
		InteropTime:  cfg.InteropTime,
	}
}

type Text interface {
	encoding.TextUnmarshaler
	fmt.Stringer
	comparable
}

type TextFlag[T Text] struct {
	Value T
}

func (a *TextFlag[T]) Set(value string) error {
	var defaultValue T
	if a.Value == defaultValue {
		return fmt.Errorf("cannot unmarshal into nil value")
	}
	return a.Value.UnmarshalText([]byte(value))
}

func (a *TextFlag[T]) String() string {
	var defaultValue T
	if a.Value == defaultValue {
		return "<nil>"
	}
	return a.Value.String()
}

func (a *TextFlag[T]) Get() T {
	return a.Value
}

func (a *TextFlag[T]) Clone() any {
	var out TextFlag[T]
	if err := out.Set(a.String()); err != nil {
		panic(fmt.Errorf("cannot clone invalid text value: %w", err))
	}
	return &out
}

var _ cli.Generic = (*TextFlag[*common.Address])(nil)

func textFlag[T Text](name string, usage string, value T) *cli.GenericFlag {
	return &cli.GenericFlag{
		Name:     name,
		Usage:    usage,
		EnvVars:  prefixEnvVars(strings.ToUpper(name)),
		Required: true,
		Value:    &TextFlag[T]{Value: value},
	}
}

func addrFlag(name string, usage string) *cli.GenericFlag {
	return textFlag[*common.Address](name, usage, new(common.Address))
}

func bytesFlag(name string, usage string) *cli.GenericFlag {
	return textFlag[*hexutil.Bytes](name, usage, new(hexutil.Bytes))
}

func hashFlag(name string, usage string) *cli.GenericFlag {
	return textFlag[*common.Hash](name, usage, new(common.Hash))
}

func bigFlag(name string, usage string) *cli.GenericFlag {
	return textFlag[*big.Int](name, usage, new(big.Int))
}

func addrFlagValue(name string, ctx *cli.Context) common.Address {
	return *ctx.Generic(name).(*TextFlag[*common.Address]).Value
}

func bytesFlagValue(name string, ctx *cli.Context) hexutil.Bytes {
	return *ctx.Generic(name).(*TextFlag[*hexutil.Bytes]).Value
}

func hashFlagValue(name string, ctx *cli.Context) common.Hash {
	return *ctx.Generic(name).(*TextFlag[*common.Hash]).Value
}

func bigFlagValue(name string, ctx *cli.Context) *big.Int {
	return ctx.Generic(name).(*TextFlag[*big.Int]).Value
}

var (
	CheatStorageGetCmd = &cli.Command{
		Name:    "get",
		Aliases: []string{"read"},
		Flags: []cli.Flag{
			DataDirFlag,
			addrFlag("address", "Address to read storage of"),
			hashFlag("key", "key in storage of address to read value"),
		},
		Action: CheatAction(true, func(ctx *cli.Context, ch *cheat.Cheater) error {
			return ch.RunAndClose(cheat.StorageGet(addrFlagValue("address", ctx), hashFlagValue("key", ctx), ctx.App.Writer))
		}),
	}
	CheatStorageSetCmd = &cli.Command{
		Name:    "set",
		Aliases: []string{"write"},
		Flags: []cli.Flag{
			DataDirFlag,
			addrFlag("address", "Address to write storage of"),
			hashFlag("key", "key in storage of address to set value of"),
			hashFlag("value", "the value to write"),
		},
		Action: CheatAction(false, func(ctx *cli.Context, ch *cheat.Cheater) error {
			return ch.RunAndClose(cheat.StorageSet(addrFlagValue("address", ctx), hashFlagValue("key", ctx), hashFlagValue("value", ctx)))
		}),
	}
	CheatStorageReadAll = &cli.Command{
		Name:    "read-all",
		Aliases: []string{"get-all"},
		Usage:   "Read all storage of the given account",
		Flags:   []cli.Flag{DataDirFlag, addrFlag("address", "Address to read all storage of")},
		Action: CheatAction(true, func(ctx *cli.Context, ch *cheat.Cheater) error {
			return ch.RunAndClose(cheat.StorageReadAll(addrFlagValue("address", ctx), ctx.App.Writer))
		}),
	}
	CheatStorageDiffCmd = &cli.Command{
		Name:  "diff",
		Usage: "Diff the storage of accounts A and B",
		Flags: []cli.Flag{DataDirFlag, hashFlag("a", "address of account A"), hashFlag("b", "address of account B")},
		Action: CheatAction(true, func(ctx *cli.Context, ch *cheat.Cheater) error {
			return ch.RunAndClose(cheat.StorageDiff(ctx.App.Writer, addrFlagValue("a", ctx), addrFlagValue("b", ctx)))
		}),
	}
	CheatStoragePatchCmd = &cli.Command{
		Name:  "patch",
		Usage: "Apply storage patch from STDIN to the given account address",
		Flags: []cli.Flag{DataDirFlag, addrFlag("address", "Address to patch storage of")},
		Action: CheatAction(false, func(ctx *cli.Context, ch *cheat.Cheater) error {
			return ch.RunAndClose(cheat.StoragePatch(os.Stdin, addrFlagValue("address", ctx)))
		}),
	}
	CheatStorageCmd = &cli.Command{
		Name: "storage",
		Subcommands: []*cli.Command{
			CheatStorageGetCmd,
			CheatStorageSetCmd,
			CheatStorageReadAll,
			CheatStorageDiffCmd,
			CheatStoragePatchCmd,
		},
	}
	CheatSetBalanceCmd = &cli.Command{
		Name: "balance",
		Flags: []cli.Flag{
			DataDirFlag,
			addrFlag("address", "Address to change balance of"),
			bigFlag("balance", "New balance of the account"),
		},
		Action: CheatAction(false, func(ctx *cli.Context, ch *cheat.Cheater) error {
			return ch.RunAndClose(cheat.SetBalance(addrFlagValue("address", ctx), bigFlagValue("balance", ctx)))
		}),
	}
	CheatSetCodeCmd = &cli.Command{
		Name: "code",
		Flags: []cli.Flag{
			DataDirFlag,
			addrFlag("address", "Address to change code of"),
			bytesFlag("code", "New code of the account"),
		},
		Action: CheatAction(false, func(ctx *cli.Context, ch *cheat.Cheater) error {
			return ch.RunAndClose(cheat.SetCode(addrFlagValue("address", ctx), bytesFlagValue("code", ctx)))
		}),
	}
	CheatSetNonceCmd = &cli.Command{
		Name: "nonce",
		Flags: []cli.Flag{
			DataDirFlag,
			addrFlag("address", "Address to change nonce of"),
			bigFlag("nonce", "New nonce of the account"),
		},
		Action: CheatAction(false, func(ctx *cli.Context, ch *cheat.Cheater) error {
			return ch.RunAndClose(cheat.SetNonce(addrFlagValue("address", ctx), bigFlagValue("balance", ctx).Uint64()))
		}),
	}
	CheatPrintHeadBlock = &cli.Command{
		Name:  "head-block",
		Usage: "dump head block as JSON",
		Flags: []cli.Flag{
			DataDirFlag,
		},
		Action: CheatRawDBAction(true, func(c *cli.Context, db ethdb.Database) error {
			enc := json.NewEncoder(c.App.Writer)
			enc.SetIndent("  ", "  ")
			block := rawdb.ReadHeadBlock(db)
			if block == nil {
				return enc.Encode(nil)
			}
			return enc.Encode(engine.RPCBlock{
				Header:       *block.Header(),
				Transactions: block.Transactions(),
			})
		}),
	}
	CheatPrintHeadHeader = &cli.Command{
		Name:  "head-header",
		Usage: "dump head header as JSON",
		Flags: []cli.Flag{
			DataDirFlag,
		},
		Action: CheatRawDBAction(true, func(c *cli.Context, db ethdb.Database) error {
			enc := json.NewEncoder(c.App.Writer)
			enc.SetIndent("  ", "  ")
			return enc.Encode(rawdb.ReadHeadHeader(db))
		}),
	}
	EngineBlockCmd = &cli.Command{
		Name:  "block",
		Usage: "build the next block using the Engine API",
		Flags: withEngineFlags(
			FeeRecipientFlag, RandaoFlag, BlockTimeFlag, BuildingTime, AllowGaps,
		),
		// TODO: maybe support transaction and tx pool engine flags, since we use op-geth?
		// TODO: reorg flag
		// TODO: finalize/safe flag

		Action: EngineAction(func(ctx *cli.Context, client *sources.EngineAPIClient, _ log.Logger) error {
			settings := ParseBuildingArgs(ctx)
			status, err := engine.Status(context.Background(), client.RPC)
			if err != nil {
				return err
			}
			payloadEnv, err := engine.BuildBlock(context.Background(), client, status, settings)
			if err != nil {
				return err
			}
			fmt.Fprintln(ctx.App.Writer, payloadEnv.ExecutionPayload.BlockHash)
			return nil
		}),
	}
	EngineAutoCmd = &cli.Command{
		Name:        "auto",
		Usage:       "Run a proof-of-nothing chain with fixed block time.",
		Description: "The block time can be changed. The execution engine must be synced to a post-Merge state first.",
		Flags: append(withEngineFlags(
			FeeRecipientFlag, RandaoFlag, BlockTimeFlag, BuildingTime, AllowGaps),
			opmetrics.CLIFlags(envVarPrefix)...),
		Action: EngineAction(func(ctx *cli.Context, client *sources.EngineAPIClient, l log.Logger) error {
			settings := ParseBuildingArgs(ctx)
			// TODO: finalize/safe flag

			metricsCfg := opmetrics.ReadCLIConfig(ctx)

			return opservice.CloseAction(ctx.Context, func(ctx context.Context) error {
				registry := opmetrics.NewRegistry()
				metrics := engine.NewMetrics("wheel", registry)
				if metricsCfg.Enabled {
					l.Info("starting metrics server", "addr", metricsCfg.ListenAddr, "port", metricsCfg.ListenPort)
					metricsSrv, err := opmetrics.StartServer(registry, metricsCfg.ListenAddr, metricsCfg.ListenPort)
					if err != nil {
						return fmt.Errorf("failed to start metrics server: %w", err)
					}
					defer func() {
						if err := metricsSrv.Stop(context.Background()); err != nil {
							l.Error("failed to stop metrics server: %w", err)
						}
					}()
				}
				return engine.Auto(ctx, metrics, client, l, settings)
			})
		}),
	}
	EngineStatusCmd = &cli.Command{
		Name:  "status",
		Flags: withEngineFlags(),
		Action: EngineAction(func(ctx *cli.Context, client *sources.EngineAPIClient, _ log.Logger) error {
			stat, err := engine.Status(context.Background(), client.RPC)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(ctx.App.Writer)
			enc.SetIndent("", "  ")
			return enc.Encode(stat)
		}),
	}
	EngineCopyCmd = &cli.Command{
		Name: "copy",
		Flags: withEngineFlags(
			&cli.StringFlag{
				Name:     "source",
				Usage:    "Unauthenticated regular eth JSON RPC to pull block data from, can be HTTP/WS/IPC.",
				Required: true,
				EnvVars:  prefixEnvVars("SOURCE"),
			},
		),
		Action: EngineAction(func(ctx *cli.Context, dest *sources.EngineAPIClient, _ log.Logger) error {
			rpcClient, err := rpc.DialOptions(context.Background(), ctx.String("source"))
			if err != nil {
				return fmt.Errorf("failed to dial engine source endpoint: %w", err)
			}
			source := client.NewBaseRPCClient(rpcClient)
			return engine.Copy(context.Background(), source, dest)
		}),
	}

	EngineCopyPayloadCmd = &cli.Command{
		Name:        "copy-payload",
		Description: "Take the block by number from source and insert it to the engine with NewPayload. No other calls are made.",
		Flags: withEngineFlags(
			&cli.StringFlag{
				Name:     "source",
				Usage:    "Unauthenticated regular eth JSON RPC to pull block data from, can be HTTP/WS/IPC.",
				Required: true,
				EnvVars:  prefixEnvVars("SOURCE"),
			},
			&cli.Uint64Flag{
				Name:     "number",
				Usage:    "Block number to copy from the source",
				Required: true,
				EnvVars:  prefixEnvVars("NUMBER"),
			},
		),
		Action: EngineAction(func(ctx *cli.Context, dest *sources.EngineAPIClient, _ log.Logger) error {
			rpcClient, err := rpc.DialOptions(context.Background(), ctx.String("source"))
			if err != nil {
				return fmt.Errorf("failed to dial engine source endpoint: %w", err)
			}
			source := client.NewBaseRPCClient(rpcClient)
			return engine.CopyPayload(context.Background(), ctx.Uint64("number"), source, dest)
		}),
	}

	EngineSetForkchoiceCmd = &cli.Command{
		Name:        "set-forkchoice",
		Description: "Set forkchoice, specify unsafe, safe and finalized blocks by number",
		Flags: withEngineFlags(
			&cli.Uint64Flag{
				Name:     "unsafe",
				Usage:    "Block number of block to set as latest block",
				Required: true,
				EnvVars:  prefixEnvVars("UNSAFE"),
			},
			&cli.Uint64Flag{
				Name:     "safe",
				Usage:    "Block number of block to set as safe block",
				Required: true,
				EnvVars:  prefixEnvVars("SAFE"),
			},
			&cli.Uint64Flag{
				Name:     "finalized",
				Usage:    "Block number of block to set as finalized block",
				Required: true,
				EnvVars:  prefixEnvVars("FINALIZED"),
			},
		),
		Action: EngineAction(func(ctx *cli.Context, client *sources.EngineAPIClient, _ log.Logger) error {
			return engine.SetForkchoice(ctx.Context, client, ctx.Uint64("finalized"), ctx.Uint64("safe"), ctx.Uint64("unsafe"))
		}),
	}

	EngineSetForkchoiceHashCmd = &cli.Command{
		Name:        "set-forkchoice-by-hash",
		Description: "Set forkchoice, specify unsafe, safe and finalized blocks by hash",
		Flags: withEngineFlags(
			&cli.StringFlag{
				Name:     "unsafe",
				Usage:    "Block hash of block to set as latest block",
				Required: true,
				EnvVars:  prefixEnvVars("UNSAFE"),
			},
			&cli.StringFlag{
				Name:     "safe",
				Usage:    "Block hash of block to set as safe block",
				Required: true,
				EnvVars:  prefixEnvVars("SAFE"),
			},
			&cli.StringFlag{
				Name:     "finalized",
				Usage:    "Block hash of block to set as finalized block",
				Required: true,
				EnvVars:  prefixEnvVars("FINALIZED"),
			},
		),
		Action: EngineAction(func(ctx *cli.Context, client *sources.EngineAPIClient, _ log.Logger) error {
			finalized := common.HexToHash(ctx.String("finalized"))
			safe := common.HexToHash(ctx.String("safe"))
			unsafe := common.HexToHash(ctx.String("unsafe"))
			return engine.SetForkchoiceByHash(ctx.Context, client, finalized, safe, unsafe)
		}),
	}

	EngineRewindCmd = &cli.Command{
		Name:        "rewind",
		Description: "Rewind chain by number (destructive!)",
		Flags: withEngineFlags(
			&cli.Uint64Flag{
				Name:     "to",
				Usage:    "Block number to rewind chain to",
				Required: true,
				EnvVars:  prefixEnvVars("REWIND_TO"),
			},
			&cli.BoolFlag{
				Name:    "set-head",
				Usage:   "Whether to also call debug_setHead when rewinding",
				EnvVars: prefixEnvVars("REWIND_SET_HEAD"),
			},
		),
		Action: EngineAction(func(ctx *cli.Context, client *sources.EngineAPIClient, lgr log.Logger) error {
			open, err := initOpenEngineRPC(ctx, lgr)
			if err != nil {
				return fmt.Errorf("failed to dial open RPC endpoint: %w", err)
			}
			return engine.Rewind(ctx.Context, lgr, client, open, ctx.Uint64("to"), ctx.Bool("set-head"))
		}),
	}

	EngineJSONCmd = &cli.Command{
		Name:        "json",
		Description: "read json values from remaining args, or STDIN, and use them as RPC params to call the engine RPC method (first arg)",
		Flags: withEngineFlags(
			&cli.BoolFlag{
				Name:     "stdin",
				Usage:    "Read params from stdin instead",
				Required: false,
				EnvVars:  prefixEnvVars("STDIN"),
			},
		),
		ArgsUsage: "<rpc-method-name> [params...]",
		Action: EngineAction(func(ctx *cli.Context, client *sources.EngineAPIClient, _ log.Logger) error {
			if ctx.NArg() == 0 {
				return fmt.Errorf("expected at least 1 argument: RPC method name")
			}
			var r io.Reader
			var args []string
			if ctx.Bool("stdin") {
				r = ctx.App.Reader
			} else {
				args = ctx.Args().Tail()
			}
			return engine.RawJSONInteraction(ctx.Context, client.RPC, ctx.Args().Get(0), args, r, ctx.App.Writer)
		}),
	}
)

var CheatCmd = &cli.Command{
	Name:  "cheat",
	Usage: "Cheating commands to modify a Geth database.",
	Description: "Each sub-command opens a Geth database, applies the cheat, and then saves and closes the database." +
		"The Geth node will live in its own false reality, other nodes cannot sync the cheated state if they process the blocks.",
	Subcommands: []*cli.Command{
		CheatStorageCmd,
		CheatSetBalanceCmd,
		CheatSetCodeCmd,
		CheatSetNonceCmd,
		CheatPrintHeadBlock,
		CheatPrintHeadHeader,
	},
}

var EngineCmd = &cli.Command{
	Name:        "engine",
	Usage:       "Engine API commands to build/reorg/rewind/finalize/copy blocks.",
	Description: "Each sub-command dials the engine API endpoint (with provided JWT secret) and then runs the action",
	Subcommands: []*cli.Command{
		EngineBlockCmd,
		EngineAutoCmd,
		EngineStatusCmd,
		EngineCopyCmd,
		EngineCopyPayloadCmd,
		EngineSetForkchoiceCmd,
		EngineSetForkchoiceHashCmd,
		EngineRewindCmd,
		EngineJSONCmd,
	},
}

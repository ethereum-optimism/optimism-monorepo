package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/cmd/chaindebug/utils"
	op_service "github.com/ethereum-optimism/optimism/op-service"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

var EnvPrefix = "OP_CHAINDEBUG"

// TODO: reassemble command

// TODO: analysis command

var (
	startFlag = &cli.Uint64Flag{
		Name:    "start",
		Value:   0,
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "START"),
	}
	endFlag = &cli.Uint64Flag{
		Name:    "end",
		Value:   0,
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "END"),
	}
	networkFlag = &cli.StringFlag{
		Name:    "network",
		Value:   "op-mainnet",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "NETWORK"),
	}
	outFlag = &cli.PathFlag{
		Name:    "out",
		Value:   "debug_out",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "OUT"),
	}
	txsInFlag = &cli.PathFlag{
		Name:    "txs-in",
		Value:   "debug_out/l1/l1-txs",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "TXS_IN"),
	}
	rpcFlag = &cli.StringFlag{
		Name:    "rpc",
		Value:   "http://localhost:8545",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "RPC"),
	}
	beaconFlag = &cli.StringFlag{
		Name:    "beacon",
		Value:   "http://localhost:9000",
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "BEACON"),
	}
	concurrentRequestsFlag = &cli.Uint64Flag{
		Name:    "concurrent-requests",
		Value:   10,
		EnvVars: op_service.PrefixEnvVar(EnvPrefix, "CONCURRENT_REQUESTS"),
	}
)

func main() {
	app := cli.NewApp()
	app.Name = "op-chaindebug"
	app.Usage = "Chain debug utils"
	app.Commands = []*cli.Command{
		{
			Name:  "download-l1",
			Usage: "Download L1 blocks of given range",
			Flags: cliapp.ProtectFlags(append([]cli.Flag{
				startFlag,
				endFlag,
				networkFlag,
				outFlag,
				rpcFlag,
				beaconFlag,
				concurrentRequestsFlag,
			}, oplog.CLIFlags(EnvPrefix)...)),

			Action: func(cliCtx *cli.Context) error {
				ctx := cliCtx.Context

				logCfg := oplog.ReadCLIConfig(cliCtx)
				logger := oplog.NewLogger(cliCtx.App.Writer, logCfg)

				start := cliCtx.Uint64(startFlag.Name)
				end := cliCtx.Uint64(endFlag.Name)
				network := cliCtx.String(networkFlag.Name)
				outDir := cliCtx.Path(outFlag.Name)
				rpcEndpoint := cliCtx.String(rpcFlag.Name)
				concurrentRequests := cliCtx.Uint64(concurrentRequestsFlag.Name)
				beaconEndpoint := cliCtx.String(beaconFlag.Name)

				beaconClient := sources.NewBeaconHTTPClient(client.NewBasicHTTPClient(beaconEndpoint, logger))
				beaconCfg := sources.L1BeaconClientConfig{FetchAllSidecars: false}
				beacon := sources.NewL1BeaconClient(beaconClient, beaconCfg)
				_, err := beacon.GetVersion(ctx)
				if err != nil {
					return fmt.Errorf("failed to check L1 Beacon API version: %w", err)
				}

				rollupCfg, err := chaincfg.GetRollupConfig(network)
				if err != nil {
					return err
				}
				logger.Info("Starting", "network", network, "out", outDir)

				cfg := &utils.DownloadConfig{
					StartNum:           start,
					EndNum:             end,
					Addr:               rpcEndpoint,
					ConcurrentRequests: concurrentRequests,
				}
				onL1Block, err := utils.OnL1Block(rollupCfg, logger, beacon, outDir)
				if err != nil {
					return err
				}
				logger.Info("Downloading range of blocks", "start", start, "end", end)
				if err := utils.DownloadRange(cliCtx.Context, logger, cfg, onL1Block); err != nil {
					return fmt.Errorf("failed to download range: %w", err)
				}
				logger.Info("Downloading gaps")
				if err := utils.DownloadGaps(cliCtx.Context, logger, cfg, onL1Block, filepath.Join(outDir, "l1-blocks")); err != nil {
					return fmt.Errorf("failed to download gaps: %w", err)
				}
				logger.Info("Done!")
				return nil
			},
		},
		{
			Name:  "download-l2",
			Usage: "Download L2 blocks of given range",
			Flags: cliapp.ProtectFlags(append([]cli.Flag{
				startFlag,
				endFlag,
				networkFlag,
				outFlag,
				rpcFlag,
				concurrentRequestsFlag,
			}, oplog.CLIFlags(EnvPrefix)...)),

			Action: func(cliCtx *cli.Context) error {
				logCfg := oplog.ReadCLIConfig(cliCtx)
				logger := oplog.NewLogger(cliCtx.App.Writer, logCfg)

				start := cliCtx.Uint64(startFlag.Name)
				end := cliCtx.Uint64(endFlag.Name)
				network := cliCtx.String(networkFlag.Name)
				outDir := cliCtx.Path(outFlag.Name)
				rpcEndpoint := cliCtx.String(rpcFlag.Name)
				concurrentRequests := cliCtx.Uint64(concurrentRequestsFlag.Name)

				rollupCfg, err := chaincfg.GetRollupConfig(network)
				if err != nil {
					return err
				}
				logger.Info("Starting", "network", network, "out", outDir)

				cfg := &utils.DownloadConfig{
					StartNum:           start,
					EndNum:             end,
					Addr:               rpcEndpoint,
					ConcurrentRequests: concurrentRequests,
				}
				onL2Block, err := utils.OnL2Block(rollupCfg, logger, outDir)
				if err != nil {
					return err
				}
				logger.Info("Downloading range of blocks", "start", start, "end", end)
				if err := utils.DownloadRange(cliCtx.Context, logger, cfg, onL2Block); err != nil {
					return fmt.Errorf("failed to download range: %w", err)
				}
				logger.Info("Downloading gaps")
				if err := utils.DownloadGaps(cliCtx.Context, logger, cfg, onL2Block, filepath.Join(outDir, "l2-blocks")); err != nil {
					return fmt.Errorf("failed to download gaps: %w", err)
				}
				logger.Info("Done!")
				return nil
			},
		},
		{
			Name:  "reassemble",
			Usage: "Reassemble batches",
			Flags: cliapp.ProtectFlags(append([]cli.Flag{
				networkFlag,
				outFlag,
				txsInFlag,
			}, oplog.CLIFlags(EnvPrefix)...)),

			Action: func(cliCtx *cli.Context) error {
				logCfg := oplog.ReadCLIConfig(cliCtx)
				logger := oplog.NewLogger(cliCtx.App.Writer, logCfg)

				network := cliCtx.String(networkFlag.Name)
				outDir := cliCtx.Path(outFlag.Name)
				txsDir := cliCtx.Path(txsInFlag.Name)

				rollupCfg, err := chaincfg.GetRollupConfig(network)
				if err != nil {
					return err
				}
				logger.Info("Starting", "network", network, "out", outDir)

				cfg := &utils.ReassembleConfig{
					TxsDir:           txsDir,
					ChannelsDir:      filepath.Join(outDir, "channels"),
					ImpliedBlocksDir: filepath.Join(outDir, "implied-blocks"),
				}
				if err := utils.Channels(cliCtx.Context, cfg, logger, rollupCfg); err != nil {
					return err
				}
				logger.Info("Done!")
				return nil
			},
		},
	}

	ctx := ctxinterrupt.WithSignalWaiterMain(context.Background())
	if err := app.RunContext(ctx, os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
}

package verify

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-program/host"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

type Runner struct {
	l1RpcUrl     string
	l1RpcKind    string
	l1BeaconUrl  string
	l2RpcUrl     string
	dataDir      string
	network      string
	chainCfg     *params.ChainConfig
	l2Client     *sources.L2Client
	logCfg       oplog.CLIConfig
	setupLog     log.Logger
	rollupCfg    *rollup.Config
	runInProcess bool
}

func NewRunner(l1RpcUrl string, l1RpcKind string, l1BeaconUrl string, l2RpcUrl string, dataDir string, network string, chainID uint64, runInProcess bool) (*Runner, error) {
	ctx := context.Background()
	logCfg := oplog.DefaultCLIConfig()
	logCfg.Level = log.LevelDebug

	setupLog := oplog.NewLogger(os.Stderr, logCfg)

	l2RawRpc, err := dial.DialRPCClientWithTimeout(ctx, dial.DefaultDialTimeout, setupLog, l2RpcUrl)
	if err != nil {
		return nil, fmt.Errorf("dial L2 client: %w", err)
	}

	rollupCfg, err := chainconfig.RollupConfigByChainID(chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to load rollup config: %w", err)
	}

	chainCfg, err := chainconfig.ChainConfigByChainID(chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to load chain config: %w", err)
	}

	l2ClientCfg := sources.L2ClientDefaultConfig(rollupCfg, false)
	l2RPC := client.NewBaseRPCClient(l2RawRpc)
	l2Client, err := sources.NewL2Client(l2RPC, setupLog, nil, l2ClientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create L2 client: %w", err)
	}

	return &Runner{
		l1RpcUrl:     l1RpcUrl,
		l1RpcKind:    l1RpcKind,
		l1BeaconUrl:  l1BeaconUrl,
		l2RpcUrl:     l2RpcUrl,
		dataDir:      dataDir,
		network:      network,
		chainCfg:     chainCfg,
		logCfg:       logCfg,
		setupLog:     setupLog,
		l2Client:     l2Client,
		rollupCfg:    rollupCfg,
		runInProcess: runInProcess,
	}, nil
}

func (r *Runner) RunBetweenBlocks(ctx context.Context, l1Head common.Hash, startBlockNum uint64, endBlockNumber uint64) error {
	if startBlockNum >= endBlockNumber {
		return fmt.Errorf("start block number %v must be less than end block number %v", startBlockNum, endBlockNumber)
	}

	l2Client, err := r.createL2Client(ctx)
	if err != nil {
		return err
	}
	defer l2Client.Close()

	agreedBlockInfo, agreedOutputRoot, err := outputAtBlockNum(ctx, l2Client, startBlockNum)
	if err != nil {
		return fmt.Errorf("failed to find starting block info: %w", err)
	}
	claimedBlockInfo, claimedOutputRoot, err := outputAtBlockNum(ctx, l2Client, endBlockNumber)
	if err != nil {
		return fmt.Errorf("failed to find ending block info: %w", err)
	}

	return r.run(ctx, l1Head, agreedBlockInfo, agreedOutputRoot, claimedOutputRoot, claimedBlockInfo)
}

func (r *Runner) createL2Client(ctx context.Context) (*sources.L2Client, error) {
	l2RawRpc, err := dial.DialRPCClientWithTimeout(ctx, dial.DefaultDialTimeout, r.setupLog, r.l2RpcUrl)
	if err != nil {
		return nil, fmt.Errorf("dial L2 client: %w", err)
	}
	l2ClientCfg := sources.L2ClientDefaultConfig(r.rollupCfg, false)
	l2RPC := client.NewBaseRPCClient(l2RawRpc)
	l2Client, err := sources.NewL2Client(l2RPC, r.setupLog, nil, l2ClientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create L2 client: %w", err)
	}
	return l2Client, nil
}

func (r *Runner) RunToFinalized(ctx context.Context) error {
	l1Client, err := dial.DialEthClientWithTimeout(ctx, dial.DefaultDialTimeout, r.setupLog, r.l1RpcUrl)
	if err != nil {
		return fmt.Errorf("failed to dial L1 client: %w", err)
	}

	l2Client, err := r.createL2Client(ctx)
	if err != nil {
		return err
	}
	defer l2Client.Close()

	l2Finalized, err := retryOp(ctx, func() (eth.BlockInfo, error) {
		return l2Client.InfoByLabel(ctx, eth.Finalized)
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve finalized L2 block: %w", err)
	}

	// Retrieve finalized L1 block after finalized L2 block to ensure it is
	l1Head, err := retryOp(ctx, func() (*types.Header, error) {
		return l1Client.HeaderByNumber(ctx, big.NewInt(rpc.FinalizedBlockNumber.Int64()))
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve finalized L1 block: %w", err)
	}

	// Process the 100 blocks leading up to finalized
	startBlockNum := uint64(0)
	if l2Finalized.NumberU64() > 100 {
		startBlockNum = l2Finalized.NumberU64() - 100
	}
	agreedBlockInfo, agreedOutputRoot, err := outputAtBlockNum(ctx, l2Client, startBlockNum)
	if err != nil {
		return fmt.Errorf("failed to find starting block info: %w", err)
	}
	claimedBlockInfo, claimedOutputRoot, err := outputAtBlockNum(ctx, l2Client, l2Finalized.NumberU64())
	if err != nil {
		return fmt.Errorf("failed to find ending block info: %w", err)
	}

	return r.run(ctx, l1Head.Hash(), agreedBlockInfo, agreedOutputRoot, claimedOutputRoot, claimedBlockInfo)
}

func (r *Runner) run(ctx context.Context, l1Head common.Hash, agreedBlockInfo eth.BlockInfo, agreedOutputRoot common.Hash, claimedOutputRoot common.Hash, claimedBlockInfo eth.BlockInfo) error {
	var err error
	if r.dataDir == "" {
		r.dataDir, err = os.MkdirTemp("", "oracledata")
		if err != nil {
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer func() {
			err := os.RemoveAll(r.dataDir)
			if err != nil {
				fmt.Println("Failed to remove temp dir:" + err.Error())
			}
		}()
	} else {
		if err := os.MkdirAll(r.dataDir, 0755); err != nil {
			fmt.Printf("Could not create data directory %v: %v", r.dataDir, err)
			os.Exit(1)
		}
	}
	fmt.Printf("Using dir: %s\n", r.dataDir)
	args := []string{
		"--log.level", "DEBUG",
		"--network", r.network,
		"--exec", "./bin/op-program-client",
		"--datadir", r.dataDir,
		"--l1.head", l1Head.Hex(),
		"--l2.head", agreedBlockInfo.Hash().Hex(),
		"--l2.outputroot", agreedOutputRoot.Hex(),
		"--l2.claim", claimedOutputRoot.Hex(),
		"--l2.blocknumber", strconv.FormatUint(claimedBlockInfo.NumberU64(), 10),
	}
	argsStr := strings.Join(args, " ")
	// args.txt is used by the verify job for offline verification in CI
	if err := os.WriteFile(filepath.Join(r.dataDir, "args.txt"), []byte(argsStr), 0644); err != nil {
		fmt.Printf("Could not write args: %v", err)
		os.Exit(1)
	}
	fmt.Printf("Configuration: %s\n", argsStr)

	if r.runInProcess {
		offlineCfg := config.NewConfig(
			r.rollupCfg, r.chainCfg, l1Head, agreedBlockInfo.Hash(), agreedOutputRoot, claimedOutputRoot, claimedBlockInfo.NumberU64())
		offlineCfg.DataDir = r.dataDir

		onlineCfg := *offlineCfg
		onlineCfg.L1URL = r.l1RpcUrl
		onlineCfg.L1BeaconURL = r.l1BeaconUrl
		onlineCfg.L2URL = r.l2RpcUrl
		if r.l1RpcKind != "" {
			onlineCfg.L1RPCKind = sources.RPCProviderKind(r.l1RpcKind)
		}

		fmt.Println("Running in online mode")
		err = host.Main(oplog.NewLogger(os.Stderr, r.logCfg), &onlineCfg)
		if err != nil {
			return fmt.Errorf("online mode failed: %w", err)
		}

		fmt.Println("Running in offline mode")
		err = host.Main(oplog.NewLogger(os.Stderr, r.logCfg), offlineCfg)
		if err != nil {
			return fmt.Errorf("offline mode failed: %w", err)
		}
	} else {
		fmt.Println("Running in online mode")
		onlineArgs := make([]string, len(args))
		copy(onlineArgs, args)
		onlineArgs = append(onlineArgs,
			"--l1", r.l1RpcUrl,
			"--l1.beacon", r.l1BeaconUrl,
			"--l2", r.l2RpcUrl)
		if r.l1RpcKind != "" {
			onlineArgs = append(onlineArgs, "--l1.rpckind", r.l1RpcKind)
		}
		err = runFaultProofProgram(ctx, onlineArgs)
		if err != nil {
			return fmt.Errorf("online mode failed: %w", err)
		}

		fmt.Println("Running in offline mode")
		err = runFaultProofProgram(ctx, args)
		if err != nil {
			return fmt.Errorf("offline mode failed: %w", err)
		}
	}
	return nil
}

func runFaultProofProgram(ctx context.Context, args []string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Hour)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./bin/op-program", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func outputAtBlockNum(ctx context.Context, l2Client *sources.L2Client, blockNum uint64) (eth.BlockInfo, common.Hash, error) {
	startBlockInfo, err := l2Client.InfoByNumber(ctx, blockNum)
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to retrieve info for block %v: %w", startBlockInfo, err)
	}

	output, err := retryOp(ctx, func() (*eth.OutputV0, error) {
		return l2Client.OutputV0AtBlock(ctx, startBlockInfo.Hash())
	})
	if err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to retrieve agreed output root for block %v: %w", startBlockInfo.Hash(), err)
	}
	return startBlockInfo, common.Hash(eth.OutputRoot(output)), nil
}

func retryOp[T any](ctx context.Context, op func() (T, error)) (T, error) {
	return retry.Do(ctx, 10, retry.Fixed(time.Second*2), op)
}

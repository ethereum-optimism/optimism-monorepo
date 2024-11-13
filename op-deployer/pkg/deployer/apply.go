package deployer

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script/forking"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"

	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

type ApplyConfig struct {
	L1RPCUrl   string
	Workdir    string
	PrivateKey string
	Logger     log.Logger

	privateKeyECDSA *ecdsa.PrivateKey
}

func (a *ApplyConfig) Check() error {
	if a.Workdir == "" {
		return fmt.Errorf("workdir must be specified")
	}

	if a.PrivateKey != "" {
		privECDSA, err := crypto.HexToECDSA(strings.TrimPrefix(a.PrivateKey, "0x"))
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		a.privateKeyECDSA = privECDSA
	}

	if a.Logger == nil {
		return fmt.Errorf("logger must be specified")
	}

	return nil
}

func (a *ApplyConfig) CheckLive() error {
	if a.privateKeyECDSA == nil {
		return fmt.Errorf("private key must be specified")
	}

	if a.L1RPCUrl == "" {
		return fmt.Errorf("l1RPCUrl must be specified")
	}

	return nil
}

func ApplyCLI() func(cliCtx *cli.Context) error {
	return func(cliCtx *cli.Context) error {
		logCfg := oplog.ReadCLIConfig(cliCtx)
		l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
		oplog.SetGlobalLogHandler(l.Handler())

		l1RPCUrl := cliCtx.String(L1RPCURLFlagName)
		workdir := cliCtx.String(WorkdirFlagName)
		privateKey := cliCtx.String(PrivateKeyFlagName)

		ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

		return Apply(ctx, ApplyConfig{
			L1RPCUrl:   l1RPCUrl,
			Workdir:    workdir,
			PrivateKey: privateKey,
			Logger:     l,
		})
	}
}

func Apply(ctx context.Context, cfg ApplyConfig) error {
	if err := cfg.Check(); err != nil {
		return fmt.Errorf("invalid config for apply: %w", err)
	}

	intent, err := pipeline.ReadIntent(cfg.Workdir)
	if err != nil {
		return fmt.Errorf("failed to read intent: %w", err)
	}

	st, err := pipeline.ReadState(cfg.Workdir)
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	if err := ApplyPipeline(ctx, ApplyPipelineOpts{
		L1RPCUrl:           cfg.L1RPCUrl,
		DeployerPrivateKey: cfg.privateKeyECDSA,
		Intent:             intent,
		State:              st,
		Logger:             cfg.Logger,
		StateWriter:        pipeline.WorkdirStateWriter(cfg.Workdir),
	}); err != nil {
		return err
	}

	return nil
}

type pipelineStage struct {
	name  string
	apply func() error
}

type ApplyPipelineOpts struct {
	L1RPCUrl           string
	DeployerPrivateKey *ecdsa.PrivateKey
	Intent             *state.Intent
	State              *state.State
	Logger             log.Logger
	StateWriter        pipeline.StateWriter
}

func ApplyPipeline(
	ctx context.Context,
	opts ApplyPipelineOpts,
) error {
	intent := opts.Intent
	st := opts.State

	progressor := func(curr, total int64) {
		opts.Logger.Info("artifacts download progress", "current", curr, "total", total)
	}

	l1ArtifactsFS, cleanupL1, err := artifacts.Download(ctx, intent.L1ContractsLocator, progressor)
	if err != nil {
		return fmt.Errorf("failed to download L1 artifacts: %w", err)
	}
	defer func() {
		if err := cleanupL1(); err != nil {
			opts.Logger.Warn("failed to clean up L1 artifacts", "err", err)
		}
	}()

	l2ArtifactsFS, cleanupL2, err := artifacts.Download(ctx, intent.L2ContractsLocator, progressor)
	if err != nil {
		return fmt.Errorf("failed to download L2 artifacts: %w", err)
	}
	defer func() {
		if err := cleanupL2(); err != nil {
			opts.Logger.Warn("failed to clean up L2 artifacts", "err", err)
		}
	}()

	bundle := pipeline.ArtifactsBundle{
		L1: l1ArtifactsFS,
		L2: l2ArtifactsFS,
	}

	var deployer common.Address
	var bcaster broadcaster.Broadcaster
	var l1Client *ethclient.Client
	var l1Host *script.Host
	if intent.DeploymentStrategy == state.DeploymentStrategyLive {
		l1RPC, err := rpc.Dial(opts.L1RPCUrl)
		if err != nil {
			return fmt.Errorf("failed to connect to L1 RPC: %w", err)
		}

		l1Client = ethclient.NewClient(l1RPC)

		chainID, err := l1Client.ChainID(ctx)
		if err != nil {
			return fmt.Errorf("failed to get chain ID: %w", err)
		}

		signer := opcrypto.SignerFnFromBind(opcrypto.PrivateKeySignerFn(opts.DeployerPrivateKey, chainID))
		deployer = crypto.PubkeyToAddress(opts.DeployerPrivateKey.PublicKey)

		bcaster, err = broadcaster.NewKeyedBroadcaster(broadcaster.KeyedBroadcasterOpts{
			Logger:  opts.Logger,
			ChainID: new(big.Int).SetUint64(intent.L1ChainID),
			Client:  l1Client,
			Signer:  signer,
			From:    deployer,
		})
		if err != nil {
			return fmt.Errorf("failed to create broadcaster: %w", err)
		}

		l1Host, err = env.DefaultScriptHost(
			bcaster,
			opts.Logger,
			deployer,
			bundle.L1,
			script.WithForkHook(func(cfg *script.ForkConfig) (forking.ForkSource, error) {
				src, err := forking.RPCSourceByNumber(cfg.URLOrAlias, l1RPC, *cfg.BlockNumber)
				if err != nil {
					return nil, fmt.Errorf("failed to create RPC fork source: %w", err)
				}
				return forking.Cache(src), nil
			}),
		)
		if err != nil {
			return fmt.Errorf("failed to create L1 script host: %w", err)
		}

		latest, err := l1Client.HeaderByNumber(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to get latest block: %w", err)
		}

		if _, err := l1Host.CreateSelectFork(
			script.ForkWithURLOrAlias("main"),
			script.ForkWithBlockNumberU256(latest.Number),
		); err != nil {
			return fmt.Errorf("failed to select fork: %w", err)
		}
	} else {
		deployer = common.Address{0x01}
		bcaster = broadcaster.NoopBroadcaster()
		l1Host, err = env.DefaultScriptHost(
			bcaster,
			opts.Logger,
			deployer,
			bundle.L1,
		)
		if err != nil {
			return fmt.Errorf("failed to create L1 script host: %w", err)
		}
	}

	pEnv := &pipeline.Env{
		StateWriter:  opts.StateWriter,
		L1ScriptHost: l1Host,
		L1Client:     l1Client,
		Logger:       opts.Logger,
		Broadcaster:  bcaster,
		Deployer:     deployer,
	}

	pline := []pipelineStage{
		{"init", func() error {
			if intent.DeploymentStrategy == state.DeploymentStrategyLive {
				return pipeline.InitLiveStrategy(ctx, pEnv, intent, st)
			} else {
				return pipeline.InitGenesisStrategy(pEnv, intent, st)
			}
		}},
		{"deploy-superchain", func() error {
			return pipeline.DeploySuperchain(pEnv, intent, st)
		}},
		{"deploy-implementations", func() error {
			return pipeline.DeployImplementations(pEnv, intent, st)
		}},
	}

	// Deploy all OP Chains first.
	for _, chain := range intent.Chains {
		chainID := chain.ID
		pline = append(pline, pipelineStage{
			fmt.Sprintf("deploy-opchain-%s", chainID.Hex()),
			func() error {
				return pipeline.DeployOPChain(pEnv, intent, st, chainID)
			},
		}, pipelineStage{
			fmt.Sprintf("deploy-alt-da-%s", chainID.Hex()),
			func() error {
				return pipeline.DeployAltDA(pEnv, intent, st, chainID)
			},
		}, pipelineStage{
			fmt.Sprintf("generate-l2-genesis-%s", chainID.Hex()),
			func() error {
				return pipeline.GenerateL2Genesis(pEnv, intent, bundle, st, chainID)
			},
		})
	}

	// Set start block after all OP chains have been deployed, since the
	// genesis strategy requires all the OP chains to exist in genesis.
	for _, chain := range intent.Chains {
		chainID := chain.ID
		pline = append(pline, pipelineStage{
			fmt.Sprintf("set-start-block-%s", chainID.Hex()),
			func() error {
				if intent.DeploymentStrategy == state.DeploymentStrategyLive {
					return pipeline.SetStartBlockLiveStrategy(ctx, pEnv, st, chainID)
				} else {
					return pipeline.SetStartBlockGenesisStrategy(pEnv, st, chainID)
				}
			},
		})
	}

	// Run through the pipeline. The state dump is captured between
	// every step.
	for _, stage := range pline {
		if err := stage.apply(); err != nil {
			return fmt.Errorf("error in pipeline stage apply: %w", err)
		}

		if intent.DeploymentStrategy == state.DeploymentStrategyGenesis {
			dump, err := pEnv.L1ScriptHost.StateDump()
			if err != nil {
				return fmt.Errorf("failed to dump state: %w", err)
			}
			st.L1StateDump = &state.GzipData[foundry.ForgeAllocs]{
				Data: dump,
			}
		}

		if _, err := pEnv.Broadcaster.Broadcast(ctx); err != nil {
			return fmt.Errorf("failed to broadcast stage %s: %w", stage.name, err)
		}
		if err := pEnv.StateWriter.WriteState(st); err != nil {
			return fmt.Errorf("failed to write state: %w", err)
		}
	}

	st.AppliedIntent = intent
	if err := pEnv.StateWriter.WriteState(st); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}

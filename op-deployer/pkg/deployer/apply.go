package deployer

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
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

	var l1Client *ethclient.Client
	var deployer common.Address
	var bcaster broadcaster.Broadcaster
	var startingNonce uint64
	if intent.DeploymentStrategy == state.DeploymentStrategyLive {
		if err := cfg.CheckLive(); err != nil {
			return fmt.Errorf("invalid config for apply: %w", err)
		}

		l1Client, err = ethclient.Dial(cfg.L1RPCUrl)
		if err != nil {
			return fmt.Errorf("failed to connect to L1 RPC: %w", err)
		}

		chainID, err := l1Client.ChainID(ctx)
		if err != nil {
			return fmt.Errorf("failed to get chain ID: %w", err)
		}

		signer := opcrypto.SignerFnFromBind(opcrypto.PrivateKeySignerFn(cfg.privateKeyECDSA, chainID))
		deployer = crypto.PubkeyToAddress(cfg.privateKeyECDSA.PublicKey)

		bcaster, err = broadcaster.NewKeyedBroadcaster(broadcaster.KeyedBroadcasterOpts{
			Logger:  cfg.Logger,
			ChainID: new(big.Int).SetUint64(intent.L1ChainID),
			Client:  l1Client,
			Signer:  signer,
			From:    deployer,
		})
		if err != nil {
			return fmt.Errorf("failed to create broadcaster: %w", err)
		}

		startingNonce, err = l1Client.NonceAt(ctx, deployer, nil)
		if err != nil {
			return fmt.Errorf("failed to get starting nonce: %w", err)
		}
	} else {
		deployer = common.Address{0x01}
		bcaster = broadcaster.NoopBroadcaster()
	}

	progressor := func(curr, total int64) {
		cfg.Logger.Info("artifacts download progress", "current", curr, "total", total)
	}

	l1ArtifactsFS, cleanupL1, err := artifacts.Download(ctx, intent.L1ContractsLocator, progressor)
	if err != nil {
		return fmt.Errorf("failed to download L1 artifacts: %w", err)
	}
	defer func() {
		if err := cleanupL1(); err != nil {
			cfg.Logger.Warn("failed to clean up L1 artifacts", "err", err)
		}
	}()

	l2ArtifactsFS, cleanupL2, err := artifacts.Download(ctx, intent.L2ContractsLocator, progressor)
	if err != nil {
		return fmt.Errorf("failed to download L2 artifacts: %w", err)
	}
	defer func() {
		if err := cleanupL2(); err != nil {
			cfg.Logger.Warn("failed to clean up L2 artifacts", "err", err)
		}
	}()

	bundle := pipeline.ArtifactsBundle{
		L1: l1ArtifactsFS,
		L2: l2ArtifactsFS,
	}

	l1Host, err := env.DefaultScriptHost(bcaster, cfg.Logger, deployer, bundle.L1, startingNonce)
	if err != nil {
		return fmt.Errorf("failed to create L1 script host: %w", err)
	}

	env := &pipeline.Env{
		StateWriter:  pipeline.WorkdirStateWriter(cfg.Workdir),
		L1ScriptHost: l1Host,
		L1Client:     l1Client,
		Logger:       cfg.Logger,
		Broadcaster:  bcaster,
		Deployer:     deployer,
	}

	if err := ApplyPipeline(ctx, env, bundle, intent, st); err != nil {
		return err
	}

	return nil
}

type pipelineStage struct {
	name  string
	apply func() error
}

func ApplyPipeline(
	ctx context.Context,
	env *pipeline.Env,
	bundle pipeline.ArtifactsBundle,
	intent *state.Intent,
	st *state.State,
) error {
	pline := []pipelineStage{
		{"init", func() error {
			if intent.DeploymentStrategy == state.DeploymentStrategyLive {
				return pipeline.InitLiveStrategy(ctx, env, intent, st)
			} else {
				return pipeline.InitGenesisStrategy(env, intent, st)
			}
		}},
		{"deploy-superchain", func() error {
			return pipeline.DeploySuperchain(env, intent, st)
		}},
		{"deploy-implementations", func() error {
			return pipeline.DeployImplementations(env, intent, st)
		}},
	}

	// Deploy all OP Chains first.
	for _, chain := range intent.Chains {
		chainID := chain.ID
		pline = append(pline, pipelineStage{
			fmt.Sprintf("deploy-opchain-%s", chainID.Hex()),
			func() error {
				if intent.DeploymentStrategy == state.DeploymentStrategyLive {
					return pipeline.DeployOPChainLiveStrategy(ctx, env, bundle, intent, st, chainID)
				} else {
					return pipeline.DeployOPChainGenesisStrategy(env, intent, st, chainID)
				}
			},
		}, pipelineStage{
			fmt.Sprintf("deploy-alt-da-%s", chainID.Hex()),
			func() error {
				return pipeline.DeployAltDA(env, intent, st, chainID)
			},
		}, pipelineStage{
			fmt.Sprintf("generate-l2-genesis-%s", chainID.Hex()),
			func() error {
				return pipeline.GenerateL2Genesis(env, intent, bundle, st, chainID)
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
					return pipeline.SetStartBlockLiveStrategy(ctx, env, st, chainID)
				} else {
					return pipeline.SetStartBlockGenesisStrategy(env, st, chainID)
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
		dump, err := env.L1ScriptHost.StateDump()
		if err != nil {
			return fmt.Errorf("failed to dump state: %w", err)
		}
		st.L1StateDump = &state.GzipData[foundry.ForgeAllocs]{
			Data: dump,
		}
		if _, err := env.Broadcaster.Broadcast(ctx); err != nil {
			return fmt.Errorf("failed to broadcast stage %s: %w", stage.name, err)
		}
		if err := env.StateWriter.WriteState(st); err != nil {
			return fmt.Errorf("failed to write state: %w", err)
		}
	}

	st.AppliedIntent = intent
	if err := env.StateWriter.WriteState(st); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}

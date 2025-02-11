package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"
	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

type Upgrader interface {
	Upgrade(host *script.Host, input json.RawMessage) error
	SupportsVersion(version string) bool
	ArtifactsURL() string
}

func UpgradeCLI(upgrader Upgrader) func(*cli.Context) error {
	return func(cliCtx *cli.Context) error {
		logCfg := oplog.ReadCLIConfig(cliCtx)
		lgr := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
		oplog.SetGlobalLogHandler(lgr.Handler())

		ctx, cancel := context.WithCancel(cliCtx.Context)
		defer cancel()

		l1RPC := cliCtx.String(deployer.L1RPCURLFlag.Name)
		if l1RPC == "" {
			return fmt.Errorf("missing required flag: %s", deployer.L1RPCURLFlag.Name)
		}
		deploymentTarget, err := deployer.NewDeploymentTarget(cliCtx.String(deployer.DeploymentTargetFlag.Name))
		if err != nil {
			return fmt.Errorf("failed to parse deployment target: %w", err)
		}

		artifactsURL := upgrader.ArtifactsURL()
		overrideArtifactsURL := cliCtx.String(OverrideArtifactsURLFlag.Name)
		if overrideArtifactsURL != "" {
			artifactsURL = overrideArtifactsURL
		}
		artifactsLocator, err := artifacts.NewLocatorFromURL(artifactsURL)
		if err != nil {
			return fmt.Errorf("failed to parse artifacts URL: %w", err)
		}

		rpcClient, err := rpc.Dial(l1RPC)
		if err != nil {
			return fmt.Errorf("failed to dial RPC %s: %w", l1RPC, err)
		}
		ethClient := ethclient.NewClient(rpcClient)

		chainID, err := ethClient.ChainID(ctx)
		if err != nil {
			return fmt.Errorf("failed to get chain ID: %w", err)
		}

		var bcaster broadcaster.Broadcaster
		depAddr := common.Address{'D'}
		switch deploymentTarget {
		case deployer.DeploymentTargetLive:
			privateKeyHex := cliCtx.String(deployer.PrivateKeyFlag.Name)
			if privateKeyHex == "" {
				return fmt.Errorf("%s flag is required for live deployment", deployer.PrivateKeyFlag.Name)
			}

			pk, err := crypto.HexToECDSA(privateKeyHex)
			if err != nil {
				return fmt.Errorf("failed to parse private key: %w", err)
			}

			depAddr = crypto.PubkeyToAddress(pk.PublicKey)

			bcaster, err = broadcaster.NewKeyedBroadcaster(broadcaster.KeyedBroadcasterOpts{
				Logger:  lgr,
				ChainID: chainID,
				Client:  ethClient,
				Signer:  opcrypto.SignerFnFromBind(opcrypto.PrivateKeySignerFn(pk, chainID)),
				From:    depAddr,
			})
			if err != nil {
				return fmt.Errorf("failed to create broadcaster: %w", err)
			}
		case deployer.DeploymentTargetCalldata:
			bcaster = new(broadcaster.CalldataBroadcaster)
		case deployer.DeploymentTargetNoop:
			bcaster = broadcaster.NoopBroadcaster()
		case deployer.DeploymentTargetGenesis:
			return fmt.Errorf("cannot upgrade into a genesis deployment")
		default:
			return fmt.Errorf("unknown deployment target: %s", deploymentTarget)
		}

		artifactsFS, err := artifacts.Download(ctx, artifactsLocator, artifacts.BarProgressor())
		if err != nil {
			return fmt.Errorf("failed to download L1 artifacts: %w", err)
		}

		host, err := env.DefaultForkedScriptHost(
			ctx,
			bcaster,
			lgr,
			depAddr,
			artifactsFS,
			rpcClient,
		)
		if err != nil {
			return fmt.Errorf("failed to create script host: %w", err)
		}

		configFilePath := cliCtx.String(ConfigFlag.Name)
		if configFilePath == "" {
			return fmt.Errorf("missing required flag: %s", ConfigFlag.Name)
		}
		cfgData, err := os.ReadFile(configFilePath)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		if err := upgrader.Upgrade(host, cfgData); err != nil {
			return fmt.Errorf("failed to upgrade: %w", err)
		}

		if deploymentTarget == deployer.DeploymentTargetCalldata {
			dump, err := bcaster.(*broadcaster.CalldataBroadcaster).Dump()
			if err != nil {
				return fmt.Errorf("failed to dump calldata: %w", err)
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(dump); err != nil {
				return fmt.Errorf("failed to encode calldata: %w", err)
			}
		} else if deploymentTarget == deployer.DeploymentTargetLive {
			if _, err := bcaster.Broadcast(ctx); err != nil {
				return fmt.Errorf("failed to broadcast: %w", err)
			}
		}

		return nil
	}
}

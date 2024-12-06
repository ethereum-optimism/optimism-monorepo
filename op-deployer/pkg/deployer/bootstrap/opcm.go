package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"

	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

type OPCMConfig struct {
	L1RPCUrl   string
	PrivateKey string
	Release    string
	Logger     log.Logger

	privateKeyECDSA *ecdsa.PrivateKey
}

func (c *OPCMConfig) Check() error {
	if c.L1RPCUrl == "" {
		return fmt.Errorf("l1RPCUrl must be specified")
	}

	if c.PrivateKey == "" {
		return fmt.Errorf("private key must be specified")
	}

	if c.Release == "" {
		return fmt.Errorf("release must be specified")
	}

	privECDSA, err := crypto.HexToECDSA(strings.TrimPrefix(c.PrivateKey, "0x"))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}
	c.privateKeyECDSA = privECDSA

	if c.Logger == nil {
		return fmt.Errorf("logger must be specified")
	}

	return nil
}

func OPCMCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	l1RPCUrl := cliCtx.String(deployer.L1RPCURLFlagName)
	privateKey := cliCtx.String(deployer.PrivateKeyFlagName)
	release := cliCtx.String(ReleaseFlagName)

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

	out, err := OPCM(ctx, OPCMConfig{
		L1RPCUrl:   l1RPCUrl,
		PrivateKey: privateKey,
		Release:    release,
		Logger:     l,
	})
	if err != nil {
		return fmt.Errorf("failed to deploy OPCM: %w", err)
	}

	if err := jsonutil.WriteJSON(out, ioutil.ToStdOut()); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

func OPCM(ctx context.Context, cfg OPCMConfig) (opcm.DeployOPCMOutput, error) {
	var out opcm.DeployOPCMOutput
	if err := cfg.Check(); err != nil {
		return out, fmt.Errorf("invalid config for OPCM: %w", err)
	}

	lgr := cfg.Logger
	progressor := func(curr, total int64) {
		lgr.Info("artifacts download progress", "current", curr, "total", total)
	}

	l1RPC, err := rpc.Dial(cfg.L1RPCUrl)
	if err != nil {
		return out, fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}

	l1Client := ethclient.NewClient(l1RPC)

	chainID, err := l1Client.ChainID(ctx)
	if err != nil {
		return out, fmt.Errorf("failed to get chain ID: %w", err)
	}
	chainIDU64 := chainID.Uint64()

	loc, err := artifacts.NewLocatorFromTag(cfg.Release)
	if err != nil {
		return out, fmt.Errorf("failed to create artifacts locator: %w", err)
	}

	artifactsFS, cleanup, err := artifacts.Download(ctx, loc, progressor)
	if err != nil {
		return out, fmt.Errorf("failed to download artifacts: %w", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			lgr.Warn("failed to clean up artifacts", "err", err)
		}
	}()

	signer := opcrypto.SignerFnFromBind(opcrypto.PrivateKeySignerFn(cfg.privateKeyECDSA, chainID))
	chainDeployer := crypto.PubkeyToAddress(cfg.privateKeyECDSA.PublicKey)

	bcaster, err := broadcaster.NewKeyedBroadcaster(broadcaster.KeyedBroadcasterOpts{
		Logger:  lgr,
		ChainID: chainID,
		Client:  l1Client,
		Signer:  signer,
		From:    chainDeployer,
	})
	if err != nil {
		return out, fmt.Errorf("failed to create broadcaster: %w", err)
	}

	host, err := env.DefaultForkedScriptHost(
		ctx,
		bcaster,
		lgr,
		chainDeployer,
		artifactsFS,
		l1RPC,
	)
	if err != nil {
		return out, fmt.Errorf("failed to create script host: %w", err)
	}

	lgr.Info("deploying OPCM", "l1ContractsRelease", cfg.Release)

	input, err := DeployOPCMInputForChain(cfg.Release, chainIDU64)
	if err != nil {
		return out, fmt.Errorf("error creating OPCM input: %w", err)
	}

	out, err = opcm.DeployOPCM(
		host,
		input,
	)
	if err != nil {
		return out, fmt.Errorf("error deploying implementations: %w", err)
	}

	if _, err := bcaster.Broadcast(ctx); err != nil {
		return out, fmt.Errorf("failed to broadcast: %w", err)
	}

	lgr.Info("deployed OPCM")

	return out, nil
}

func DeployOPCMInputForChain(release string, chainID uint64) (opcm.DeployOPCMInput, error) {
	superchain, err := standard.SuperchainFor(chainID)
	if err != nil {
		return opcm.DeployOPCMInput{}, fmt.Errorf("error getting superchain config: %w", err)
	}

	l1VersionsData, err := standard.L1VersionsFor(chainID)
	if err != nil {
		return opcm.DeployOPCMInput{}, fmt.Errorf("error getting L1 versions: %w", err)
	}
	releases, ok := l1VersionsData.Releases[release]
	if !ok {
		return opcm.DeployOPCMInput{}, fmt.Errorf("release not found: %s", release)
	}

	blueprints, err := standard.OPCMBlueprintsFor(chainID)
	if err != nil {
		return opcm.DeployOPCMInput{}, fmt.Errorf("error getting OPCM blueprints: %w", err)
	}

	return opcm.DeployOPCMInput{
		SuperchainConfig:   common.Address(*superchain.Config.SuperchainConfigAddr),
		ProtocolVersions:   common.Address(*superchain.Config.ProtocolVersionsAddr),
		L1ContractsRelease: strings.TrimPrefix(release, "op-contracts/"),

		AddressManagerBlueprint:           blueprints.AddressManager,
		ProxyBlueprint:                    blueprints.Proxy,
		ProxyAdminBlueprint:               blueprints.ProxyAdmin,
		L1ChugSplashProxyBlueprint:        blueprints.L1ChugSplashProxy,
		ResolvedDelegateProxyBlueprint:    blueprints.ResolvedDelegateProxy,
		AnchorStateRegistryBlueprint:      blueprints.AnchorStateRegistry,
		PermissionedDisputeGame1Blueprint: blueprints.PermissionedDisputeGame1,
		PermissionedDisputeGame2Blueprint: blueprints.PermissionedDisputeGame2,

		L1ERC721BridgeImpl:               releases.L1ERC721Bridge.ImplementationAddress,
		OptimismPortalImpl:               releases.OptimismPortal.ImplementationAddress,
		SystemConfigImpl:                 releases.SystemConfig.ImplementationAddress,
		OptimismMintableERC20FactoryImpl: releases.OptimismMintableERC20Factory.ImplementationAddress,
		L1CrossDomainMessengerImpl:       releases.L1CrossDomainMessenger.ImplementationAddress,
		L1StandardBridgeImpl:             releases.L1StandardBridge.ImplementationAddress,
		DisputeGameFactoryImpl:           releases.DisputeGameFactory.ImplementationAddress,
		DelayedWETHImpl:                  releases.DelayedWETH.ImplementationAddress,
		MipsImpl:                         releases.MIPS.Address,
	}, nil
}

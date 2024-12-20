package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	artifacts2 "github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

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
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

type DelayedWETHConfig struct {
	L1RPCUrl         string
	PrivateKey       string
	Logger           log.Logger
	ArtifactsLocator *artifacts2.Locator
	DelayedWethImpl  common.Address

	privateKeyECDSA *ecdsa.PrivateKey
}

func (c *DelayedWETHConfig) Check() error {
	if c.L1RPCUrl == "" {
		return fmt.Errorf("l1RPCUrl must be specified")
	}

	if c.PrivateKey == "" {
		return fmt.Errorf("private key must be specified")
	}

	privECDSA, err := crypto.HexToECDSA(strings.TrimPrefix(c.PrivateKey, "0x"))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}
	c.privateKeyECDSA = privECDSA

	if c.Logger == nil {
		return fmt.Errorf("logger must be specified")
	}

	if c.ArtifactsLocator == nil {
		return fmt.Errorf("artifacts locator must be specified")
	}

	return nil
}

func DelayedWETHCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	outfile := cliCtx.String(OutfileFlagName)
	config, err := NewDelayedWETHConfigFromClI(cliCtx, l)
	if err != nil {
		return err
	}

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

	dwo, err := DelayedWETH(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to deploy DelayedWETH: %w", err)
	}

	if err := jsonutil.WriteJSON(dwo, ioutil.ToStdOutOrFileOrNoop(outfile, 0o755)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

func NewDelayedWETHConfigFromClI(cliCtx *cli.Context, l log.Logger) (DelayedWETHConfig, error) {
	l1RPCUrl := cliCtx.String(deployer.L1RPCURLFlagName)
	privateKey := cliCtx.String(deployer.PrivateKeyFlagName)
	artifactsURLStr := cliCtx.String(ArtifactsLocatorFlagName)
	artifactsLocator := new(artifacts2.Locator)
	if err := artifactsLocator.UnmarshalText([]byte(artifactsURLStr)); err != nil {
		return DelayedWETHConfig{}, fmt.Errorf("failed to parse artifacts URL: %w", err)
	}
	delayedWethImpl := common.HexToAddress(cliCtx.String(DelayedWethImplFlagName))
	config := DelayedWETHConfig{
		L1RPCUrl:         l1RPCUrl,
		PrivateKey:       privateKey,
		Logger:           l,
		ArtifactsLocator: artifactsLocator,
		DelayedWethImpl:  delayedWethImpl,
	}
	return config, nil
}

func DelayedWETH(ctx context.Context, cfg DelayedWETHConfig) (opcm.DeployDelayedWETHOutput, error) {
	var dwo opcm.DeployDelayedWETHOutput
	if err := cfg.Check(); err != nil {
		return dwo, fmt.Errorf("invalid config for DelayedWETH: %w", err)
	}

	lgr := cfg.Logger
	progressor := func(curr, total int64) {
		lgr.Info("artifacts download progress", "current", curr, "total", total)
	}

	artifactsFS, cleanup, err := artifacts2.Download(ctx, cfg.ArtifactsLocator, progressor)
	if err != nil {
		return dwo, fmt.Errorf("failed to download artifacts: %w", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			lgr.Warn("failed to clean up artifacts", "err", err)
		}
	}()

	l1Client, err := ethclient.Dial(cfg.L1RPCUrl)
	if err != nil {
		return dwo, fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}

	chainID, err := l1Client.ChainID(ctx)
	if err != nil {
		return dwo, fmt.Errorf("failed to get chain ID: %w", err)
	}
	chainIDU64 := chainID.Uint64()

	superCfg, err := standard.SuperchainFor(chainIDU64)
	if err != nil {
		return dwo, fmt.Errorf("error getting superchain config: %w", err)
	}
	proxyAdmin, err := standard.SuperchainProxyAdminAddrFor(chainIDU64)
	if err != nil {
		return dwo, fmt.Errorf("error getting superchain proxy admin: %w", err)
	}
	delayedWethOwner, err := standard.SystemOwnerAddrFor(chainIDU64)
	if err != nil {
		return dwo, fmt.Errorf("error getting superchain system owner: %w", err)
	}

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
		return dwo, fmt.Errorf("failed to create broadcaster: %w", err)
	}

	l1RPC, err := rpc.Dial(cfg.L1RPCUrl)
	if err != nil {
		return dwo, fmt.Errorf("failed to connect to L1 RPC: %w", err)
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
		return dwo, fmt.Errorf("failed to create script host: %w", err)
	}

	var release string
	if cfg.ArtifactsLocator.IsTag() {
		release = cfg.ArtifactsLocator.Tag
	} else {
		release = "dev"
	}

	lgr.Info("deploying DelayedWETH", "release", release)

	superchainConfigAddr := common.Address(*superCfg.Config.SuperchainConfigAddr)

	dwo, err = opcm.DeployDelayedWETH(
		host,
		opcm.DeployDelayedWETHInput{
			Release:               release,
			ProxyAdmin:            proxyAdmin,
			SuperchainConfigProxy: superchainConfigAddr,
			DelayedWethImpl:       cfg.DelayedWethImpl,
			DelayedWethOwner:      delayedWethOwner,
			DelayedWethDelay:      big.NewInt(604800),
		},
	)
	if err != nil {
		return dwo, fmt.Errorf("error deploying DelayedWETH: %w", err)
	}

	if _, err := bcaster.Broadcast(ctx); err != nil {
		return dwo, fmt.Errorf("failed to broadcast: %w", err)
	}

	lgr.Info("deployed DelayedWETH")
	return dwo, nil
}

package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"
	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

type SuperchainConfig struct {
	L1RPCUrl         string
	PrivateKey       string
	Logger           log.Logger
	ArtifactsLocator *artifacts.Locator

	privateKeyECDSA *ecdsa.PrivateKey

	SuperchainProxyAdminOwner  common.Address
	ProtocolVersionsOwner      common.Address
	Guardian                   common.Address
	Paused                     bool
	RequiredProtocolVersion    params.ProtocolVersion
	RecommendedProtocolVersion params.ProtocolVersion
}

func (c *SuperchainConfig) Check() error {
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

	if c.SuperchainProxyAdminOwner == (common.Address{}) {
		return fmt.Errorf("superchain proxy admin owner must be specified")
	}

	if c.ProtocolVersionsOwner == (common.Address{}) {
		return fmt.Errorf("protocol versions owner must be specified")
	}

	if c.Guardian == (common.Address{}) {
		return fmt.Errorf("guardian must be specified")
	}

	return nil
}

func SuperchainCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	l1RPCUrl := cliCtx.String(deployer.L1RPCURLFlagName)
	privateKey := cliCtx.String(deployer.PrivateKeyFlagName)
	artifactsURLStr := cliCtx.String(ArtifactsLocatorFlagName)
	artifactsLocator := new(artifacts.Locator)
	if err := artifactsLocator.UnmarshalText([]byte(artifactsURLStr)); err != nil {
		return fmt.Errorf("failed to parse artifacts URL: %w", err)
	}

	superchainProxyAdminOwner := common.HexToAddress(cliCtx.String(SuperchainProxyAdminOwnerFlagName))
	protocolVersionsOwner := common.HexToAddress(cliCtx.String(ProtocolVersionsOwnerFlagName))
	guardian := common.HexToAddress(cliCtx.String(GuardianFlagName))
	paused := cliCtx.Bool(PausedFlagName)
	requiredVersionStr := cliCtx.String(RequiredProtocolVersionFlagName)
	recommendedVersionStr := cliCtx.String(RecommendedProtocolVersionFlagName)
	outfile := cliCtx.String(OutfileFlagName)

	cfg := SuperchainConfig{
		L1RPCUrl:                  l1RPCUrl,
		PrivateKey:                privateKey,
		Logger:                    l,
		ArtifactsLocator:          artifactsLocator,
		SuperchainProxyAdminOwner: superchainProxyAdminOwner,
		ProtocolVersionsOwner:     protocolVersionsOwner,
		Guardian:                  guardian,
		Paused:                    paused,
	}

	if err := cfg.RequiredProtocolVersion.UnmarshalText([]byte(requiredVersionStr)); err != nil {
		return fmt.Errorf("failed to parse required protocol version: %w", err)
	}
	if err := cfg.RecommendedProtocolVersion.UnmarshalText([]byte(recommendedVersionStr)); err != nil {
		return fmt.Errorf("failed to parse required protocol version: %w", err)
	}

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

	dso, err := Superchain(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to deploy superchain: %w", err)
	}

	if err := jsonutil.WriteJSON(dso, ioutil.ToStdOutOrFileOrNoop(outfile, 0o755)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

func Superchain(ctx context.Context, cfg SuperchainConfig) (opcm.DeploySuperchainOutput, error) {
	var dso opcm.DeploySuperchainOutput

	if err := cfg.Check(); err != nil {
		return dso, fmt.Errorf("invalid config for Superchain: %w", err)
	}

	lgr := cfg.Logger
	artifactsFS, err := artifacts.Download(ctx, cfg.ArtifactsLocator, artifacts.BarProgressor())
	if err != nil {
		return dso, fmt.Errorf("failed to download artifacts: %w", err)
	}

	l1Client, err := ethclient.Dial(cfg.L1RPCUrl)
	if err != nil {
		return dso, fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}

	chainID, err := l1Client.ChainID(ctx)
	if err != nil {
		return dso, fmt.Errorf("failed to get chain ID: %w", err)
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
		return dso, fmt.Errorf("failed to create broadcaster: %w", err)
	}

	l1RPC, err := rpc.Dial(cfg.L1RPCUrl)
	if err != nil {
		return dso, fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}

	l1Host, err := env.DefaultForkedScriptHost(
		ctx,
		bcaster,
		lgr,
		chainDeployer,
		artifactsFS,
		l1RPC,
	)
	if err != nil {
		return dso, fmt.Errorf("failed to create script host: %w", err)
	}

	dso, err = opcm.DeploySuperchain(
		l1Host,
		opcm.DeploySuperchainInput{
			SuperchainProxyAdminOwner:  cfg.SuperchainProxyAdminOwner,
			ProtocolVersionsOwner:      cfg.ProtocolVersionsOwner,
			Guardian:                   cfg.Guardian,
			Paused:                     cfg.Paused,
			RequiredProtocolVersion:    cfg.RequiredProtocolVersion,
			RecommendedProtocolVersion: cfg.RecommendedProtocolVersion,
		},
	)
	if err != nil {
		return dso, fmt.Errorf("error deploying superchain: %w", err)
	}

	if _, err := bcaster.Broadcast(ctx); err != nil {
		return dso, fmt.Errorf("failed to broadcast: %w", err)
	}

	lgr.Info("deployed superchain configuration")

	return dso, nil
}

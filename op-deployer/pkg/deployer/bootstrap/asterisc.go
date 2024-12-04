package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

type AsteriscConfig struct {
	L1RPCUrl         string
	PrivateKey       string
	Logger           log.Logger
	ArtifactsLocator *artifacts.Locator

	privateKeyECDSA *ecdsa.PrivateKey

	PreimageOracle common.Address
}

func (c *AsteriscConfig) Check() error {
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

	if c.PreimageOracle == (common.Address{}) {
		return fmt.Errorf("preimage oracle must be specified")
	}

	return nil
}

func AsteriscCLI(cliCtx *cli.Context) error {
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

	preimageOracle := common.HexToAddress(cliCtx.String(PreimageOracleFlagName))

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

	return Asterisc(ctx, AsteriscConfig{
		L1RPCUrl:         l1RPCUrl,
		PrivateKey:       privateKey,
		Logger:           l,
		ArtifactsLocator: artifactsLocator,
		PreimageOracle:   preimageOracle,
	})
}

func Asterisc(ctx context.Context, cfg AsteriscConfig) error {
	if err := cfg.Check(); err != nil {
		return fmt.Errorf("invalid config for Asterisc: %w", err)
	}

	lgr := cfg.Logger
	progressor := func(curr, total int64) {
		lgr.Info("artifacts download progress", "current", curr, "total", total)
	}

	artifactsFS, cleanup, err := artifacts.Download(ctx, cfg.ArtifactsLocator, progressor)
	if err != nil {
		return fmt.Errorf("failed to download artifacts: %w", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			lgr.Warn("failed to clean up artifacts", "err", err)
		}
	}()

	l1Client, err := ethclient.Dial(cfg.L1RPCUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}

	chainID, err := l1Client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
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
		return fmt.Errorf("failed to create broadcaster: %w", err)
	}

	l1RPC, err := rpc.Dial(cfg.L1RPCUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to L1 RPC: %w", err)
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
		return fmt.Errorf("failed to create script host: %w", err)
	}

	dgo, err := opcm.DeployAsterisc(
		l1Host,
		opcm.DeployAsteriscInput{
			PreimageOracle: cfg.PreimageOracle,
		},
	)
	if err != nil {
		return fmt.Errorf("error deploying asterisc VM: %w", err)
	}

	if _, err := bcaster.Broadcast(ctx); err != nil {
		return fmt.Errorf("failed to broadcast: %w", err)
	}

	lgr.Info("deployed asterisc VM")

	if err := jsonutil.WriteJSON(dgo, ioutil.ToStdOut()); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

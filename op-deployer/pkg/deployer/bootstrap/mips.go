package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/rpc"

	artifacts2 "github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
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
	"github.com/urfave/cli/v2"
)

type MIPSConfig struct {
	L1RPCUrl         string
	PrivateKey       string
	Logger           log.Logger
	ArtifactsLocator *artifacts2.Locator

	privateKeyECDSA *ecdsa.PrivateKey

	PreimageOracle common.Address
	MipsVersion    uint64
}

func (c *MIPSConfig) Check() error {
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

	if c.MipsVersion == 0 {
		return fmt.Errorf("mips version must be specified")
	}
	if c.MipsVersion != 1 && c.MipsVersion != 2 {
		return fmt.Errorf("mips version must be either 1 or 2")
	}

	return nil
}

func MIPSCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	l1RPCUrl := cliCtx.String(deployer.L1RPCURLFlagName)
	privateKey := cliCtx.String(deployer.PrivateKeyFlagName)
	outfile := cliCtx.String(OutfileFlagName)
	artifactsURLStr := cliCtx.String(ArtifactsLocatorFlagName)
	artifactsLocator := new(artifacts2.Locator)
	if err := artifactsLocator.UnmarshalText([]byte(artifactsURLStr)); err != nil {
		return fmt.Errorf("failed to parse artifacts URL: %w", err)
	}

	mipsVersion := cliCtx.Uint64(MIPSVersionFlagName)
	preimageOracle := common.HexToAddress(cliCtx.String(PreimageOracleFlagName))

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

	dmo, err := MIPS(ctx, MIPSConfig{
		L1RPCUrl:         l1RPCUrl,
		PrivateKey:       privateKey,
		Logger:           l,
		ArtifactsLocator: artifactsLocator,
		MipsVersion:      mipsVersion,
		PreimageOracle:   preimageOracle,
	})
	if err != nil {
		return fmt.Errorf("failed to deploy MIPS: %w", err)
	}

	if err := jsonutil.WriteJSON(dmo, ioutil.ToStdOutOrFileOrNoop(outfile, 0o755)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

func MIPS(ctx context.Context, cfg MIPSConfig) (opcm.DeployMIPSOutput, error) {
	var dmo opcm.DeployMIPSOutput
	if err := cfg.Check(); err != nil {
		return dmo, fmt.Errorf("invalid config for MIPS: %w", err)
	}

	lgr := cfg.Logger
	progressor := func(curr, total int64) {
		lgr.Info("artifacts download progress", "current", curr, "total", total)
	}

	artifactsFS, cleanup, err := artifacts2.Download(ctx, cfg.ArtifactsLocator, progressor)
	if err != nil {
		return dmo, fmt.Errorf("failed to download artifacts: %w", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			lgr.Warn("failed to clean up artifacts", "err", err)
		}
	}()

	l1RPC, err := rpc.Dial(cfg.L1RPCUrl)
	if err != nil {
		return dmo, fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}

	l1Client := ethclient.NewClient(l1RPC)

	chainID, err := l1Client.ChainID(ctx)
	if err != nil {
		return dmo, fmt.Errorf("failed to get chain ID: %w", err)
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
		return dmo, fmt.Errorf("failed to create broadcaster: %w", err)
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
		return dmo, fmt.Errorf("failed to create script host: %w", err)
	}

	var release string
	if cfg.ArtifactsLocator.IsTag() {
		release = cfg.ArtifactsLocator.Tag
	} else {
		release = "dev"
	}

	lgr.Info("deploying dispute game", "release", release)

	dmo, err = opcm.DeployMIPS(
		host,
		opcm.DeployMIPSInput{
			MipsVersion:    cfg.MipsVersion,
			PreimageOracle: cfg.PreimageOracle,
		},
	)
	if err != nil {
		return dmo, fmt.Errorf("error deploying dispute game: %w", err)
	}

	if _, err := bcaster.Broadcast(ctx); err != nil {
		return dmo, fmt.Errorf("failed to broadcast: %w", err)
	}

	lgr.Info("deployed dispute game")

	return dmo, nil
}

package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"os"
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
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

type ValidatorConfig struct {
	L1RPCUrl         string
	PrivateKey       string
	Logger           log.Logger
	ArtifactsLocator *artifacts.Locator
	Input            ValidatorInput

	privateKeyECDSA *ecdsa.PrivateKey
}

type ValidatorInput struct {
	Release                          string         `json:"release"`
	SuperchainConfig                 common.Address `json:"superchainConfig"`
	ProtocolVersions                 common.Address `json:"protocolVersions"`
	L1PAOMultisig                    common.Address `json:"l1PAOMultisig"`
	MIPS                             common.Address `json:"mips" evm:"mips"`
	Challenger                       common.Address `json:"challenger"`
	SuperchainConfigImpl             common.Address `json:"superchainConfigImpl"`
	ProtocolVersionsImpl             common.Address `json:"protocolVersionsImpl"`
	L1ERC721BridgeImpl               common.Address `json:"l1ERC721BridgeImpl"`
	OptimismPortalImpl               common.Address `json:"optimismPortalImpl"`
	SystemConfigImpl                 common.Address `json:"systemConfigImpl"`
	OptimismMintableERC20FactoryImpl common.Address `json:"optimismMintableERC20FactoryImpl"`
	L1CrossDomainMessengerImpl       common.Address `json:"l1CrossDomainMessengerImpl"`
	L1StandardBridgeImpl             common.Address `json:"l1StandardBridgeImpl"`
	DisputeGameFactoryImpl           common.Address `json:"disputeGameFactoryImpl"`
	AnchorStateRegistryImpl          common.Address `json:"anchorStateRegistryImpl"`
	DelayedWETHImpl                  common.Address `json:"delayedWETHImpl"`
	MIPSImpl                         common.Address `json:"mipsImpl" evm:"mipsImpl"`
}

type ValidatorOutput struct {
	Validator common.Address `json:"validator"`
}

func (c *ValidatorConfig) Check() error {
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

func ValidatorCLI(cliCtx *cli.Context) error {
	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	l1RPCUrl := cliCtx.String(deployer.L1RPCURLFlagName)
	privateKey := cliCtx.String(deployer.PrivateKeyFlagName)
	outfile := cliCtx.String(OutfileFlagName)
	artifactsURLStr := cliCtx.String(ArtifactsLocatorFlagName)
	configFile := cliCtx.String(ConfigFileFlag.Name)

	artifactsLocator := new(artifacts.Locator)
	if err := artifactsLocator.UnmarshalText([]byte(artifactsURLStr)); err != nil {
		return fmt.Errorf("failed to parse artifacts URL: %w", err)
	}

	// Read the input configuration from file
	configData, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var input ValidatorInput
	if err := json.Unmarshal(configData, &input); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	ctx := ctxinterrupt.WithCancelOnInterrupt(cliCtx.Context)

	dvo, err := Validator(ctx, ValidatorConfig{
		L1RPCUrl:         l1RPCUrl,
		PrivateKey:       privateKey,
		Logger:           l,
		ArtifactsLocator: artifactsLocator,
		Input:            input,
	})
	if err != nil {
		return fmt.Errorf("failed to deploy Validator: %w", err)
	}

	if err := jsonutil.WriteJSON(dvo, ioutil.ToStdOutOrFileOrNoop(outfile, 0o755)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

func Validator(ctx context.Context, cfg ValidatorConfig) (ValidatorOutput, error) {
	var output ValidatorOutput
	if err := cfg.Check(); err != nil {
		return output, fmt.Errorf("invalid config for Validator: %w", err)
	}

	lgr := cfg.Logger

	artifactsFS, err := artifacts.Download(ctx, cfg.ArtifactsLocator, artifacts.BarProgressor())
	if err != nil {
		return output, fmt.Errorf("failed to download artifacts: %w", err)
	}

	l1Client, err := ethclient.Dial(cfg.L1RPCUrl)
	if err != nil {
		return output, fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}

	chainID, err := l1Client.ChainID(ctx)
	if err != nil {
		return output, fmt.Errorf("failed to get chain ID: %w", err)
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
		return output, fmt.Errorf("failed to create broadcaster: %w", err)
	}

	l1RPC, err := rpc.Dial(cfg.L1RPCUrl)
	if err != nil {
		return output, fmt.Errorf("failed to connect to L1 RPC: %w", err)
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
		return output, fmt.Errorf("failed to create script host: %w", err)
	}

	// Execute the deployment script
	output, err = opcm.RunScriptSingle[ValidatorInput, ValidatorOutput](
		l1Host,
		cfg.Input,
		"DeployStandardValidator.s.sol",
		"DeployStandardValidator",
	)
	if err != nil {
		return output, fmt.Errorf("error deploying validator: %w", err)
	}

	// Broadcast the transactions
	if _, err := bcaster.Broadcast(ctx); err != nil {
		return output, fmt.Errorf("failed to broadcast: %w", err)
	}

	lgr.Info("deployed validator contracts")

	return output, nil
}

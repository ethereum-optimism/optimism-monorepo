package inspect

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/env"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/pipeline"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/urfave/cli/v2"
)

func L2SemversCLI(cliCtx *cli.Context) error {
	cliCfg, err := readConfig(cliCtx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cliCtx.Context, time.Minute)
	defer cancel()

	logCfg := oplog.ReadCLIConfig(cliCtx)
	l := oplog.NewLogger(oplog.AppOut(cliCtx), logCfg)
	oplog.SetGlobalLogHandler(l.Handler())

	globalState, err := pipeline.ReadState(cliCfg.Workdir)
	if err != nil {
		return fmt.Errorf("failed to read intent: %w", err)
	}
	chainState, err := globalState.Chain(cliCfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to find chain state: %w", err)
	}

	intent := globalState.AppliedIntent
	if intent == nil {
		return fmt.Errorf("can only run this command following a full apply")
	}
	if chainState.Allocs == nil {
		return fmt.Errorf("chain state does not have allocs")
	}

	artifactsFS, err := artifacts.Download(ctx, intent.L2ContractsLocator, artifacts.BarProgressor())
	if err != nil {
		return fmt.Errorf("failed to download L2 artifacts: %w", err)
	}

	ps, err := L2Semvers(L2SemversConfig{
		Lgr:        l,
		Artifacts:  artifactsFS,
		ChainState: chainState,
	})
	if err != nil {
		return fmt.Errorf("failed to get L2 semvers: %w", err)
	}

	if err := jsonutil.WriteJSON(ps, ioutil.ToStdOutOrFileOrNoop(cliCfg.Outfile, 0o666)); err != nil {
		return fmt.Errorf("failed to write rollup config: %w", err)
	}

	return nil
}

type L2SemversConfig struct {
	Lgr        log.Logger
	Artifacts  foundry.StatDirFs
	ChainState *state.ChainState
}

type L2PredeploySemvers struct {
	L2ToL1MessagePasser           string
	DeployerWhitelist             string
	WETH                          string
	L2CrossDomainMessenger        string
	L2StandardBridge              string
	SequencerFeeVault             string
	OptimismMintableERC20Factory  string
	L1BlockNumber                 string
	GasPriceOracle                string
	L1Block                       string
	LegacyMessagePasser           string
	L2ERC721Bridge                string
	OptimismMintableERC721Factory string
	BaseFeeVault                  string
	L1FeeVault                    string
	SchemaRegistry                string
	EAS                           string
	CrossL2Inbox                  string
	L2toL2CrossDomainMessenger    string
	SuperchainWETH                string
	ETHLiquidity                  string
	SuperchainTokenBridge         string
	OptimismMintableERC20         string
	OptimismMintableERC721        string
}

func L2Semvers(cfg L2SemversConfig) (*L2PredeploySemvers, error) {
	l := cfg.Lgr
	artifactsFS := cfg.Artifacts
	chainState := cfg.ChainState

	host, err := env.DefaultScriptHost(
		broadcaster.NoopBroadcaster(),
		l,
		common.Address{19: 0x01},
		artifactsFS,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create script host: %w", err)
	}
	host.ImportState(chainState.Allocs.Data)

	type contractToCheck struct {
		Address  common.Address
		FieldPtr *string
		Name     string
	}

	var ps L2PredeploySemvers

	contracts := []contractToCheck{
		{predeploys.L2ToL1MessagePasserAddr, &ps.L2ToL1MessagePasser, "L2ToL1MessagePasser"},
		{predeploys.DeployerWhitelistAddr, &ps.DeployerWhitelist, "DeployerWhitelist"},
		{predeploys.WETHAddr, &ps.WETH, "WETH"},
		{predeploys.L2CrossDomainMessengerAddr, &ps.L2CrossDomainMessenger, "L2CrossDomainMessenger"},
		{predeploys.L2StandardBridgeAddr, &ps.L2StandardBridge, "L2StandardBridge"},
		{predeploys.SequencerFeeVaultAddr, &ps.SequencerFeeVault, "SequencerFeeVault"},
		{predeploys.OptimismMintableERC20FactoryAddr, &ps.OptimismMintableERC20Factory, "OptimismMintableERC20Factory"},
		{predeploys.L1BlockNumberAddr, &ps.L1BlockNumber, "L1BlockNumber"},
		{predeploys.GasPriceOracleAddr, &ps.GasPriceOracle, "GasPriceOracle"},
		{predeploys.L1BlockAddr, &ps.L1Block, "L1Block"},
		{predeploys.LegacyMessagePasserAddr, &ps.LegacyMessagePasser, "LegacyMessagePasser"},
		{predeploys.L2ERC721BridgeAddr, &ps.L2ERC721Bridge, "L2ERC721Bridge"},
		{predeploys.OptimismMintableERC721FactoryAddr, &ps.OptimismMintableERC721Factory, "OptimismMintableERC721Factory"},
		{predeploys.BaseFeeVaultAddr, &ps.BaseFeeVault, "BaseFeeVault"},
		{predeploys.L1FeeVaultAddr, &ps.L1FeeVault, "L1FeeVault"},
		{predeploys.SchemaRegistryAddr, &ps.SchemaRegistry, "SchemaRegistry"},
		{predeploys.EASAddr, &ps.EAS, "EAS"},
	}
	for _, contract := range contracts {
		semver, err := ReadSemver(host, contract.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to read semver for %s: %w", contract.Name, err)
		}

		*contract.FieldPtr = semver
	}

	erc20Semver, err := findSemverBytecode(host, predeploys.OptimismMintableERC20FactoryAddr)
	if err == nil {
		ps.OptimismMintableERC20 = erc20Semver
	} else {
		l.Warn("failed to find semver for OptimismMintableERC20", "err", err)
	}

	erc721Semver, err := findSemverBytecode(host, predeploys.OptimismMintableERC721FactoryAddr)
	if err == nil {
		ps.OptimismMintableERC721 = erc721Semver
	} else {
		l.Warn("failed to find semver for OptimismMintableERC721", "err", err)
	}

	return &ps, nil
}

var versionSelector = []byte{0x54, 0xfd, 0x4d, 0x50}

func ReadSemver(host *script.Host, addr common.Address) (string, error) {
	data, _, err := host.Call(
		common.Address{19: 0x01},
		addr,
		bytes.Clone(versionSelector),
		1_000_000_000,
		uint256.NewInt(0),
	)
	if err != nil {
		return "", fmt.Errorf("failed to call version on %s: %w", addr, err)
	}

	// The second 32 bytes contain the length of the string
	length := new(big.Int).SetBytes(data[32:64]).Int64()
	// Start of the string data (after offset and length)
	stringStart := 64
	stringEnd := int64(stringStart) + length

	// Bounds check
	if stringEnd > int64(len(data)) {
		return "", fmt.Errorf("string data out of bounds")
	}

	return string(data[stringStart:stringEnd]), nil
}

const patternLen = 24

var semverRegexp = regexp.MustCompile(`^(\d+\.\d+\.\d+([\w.+\-]*))\x00`)
var codeAddr = common.HexToAddress("0xc0d3c0d3c0d3c0d3c0d3c0d3c0d3c0d3c0d30000")

func findSemverBytecode(host *script.Host, proxyAddr common.Address) (string, error) {
	var implAddr common.Address
	copy(implAddr[:], codeAddr[:])
	copy(implAddr[18:], proxyAddr[18:])

	bytecode := host.GetCode(implAddr)
	if len(bytecode) == 0 {
		return "", fmt.Errorf("failed to get bytecode for factory")
	}

	versionSelectorIndex := bytes.LastIndex(bytecode, versionSelector)
	if versionSelectorIndex == -1 {
		return "", fmt.Errorf("failed to find semver selector in factory bytecode")
	}

	for i := versionSelectorIndex; i < len(bytecode); i++ {
		if bytecode[i] == 0 {
			continue
		}

		if i+patternLen > len(bytecode) {
			break
		}

		slice := bytecode[i : i+patternLen]
		if slice[0] == 0x00 {
			continue
		}

		matches := semverRegexp.FindSubmatch(slice)
		if len(matches) == 0 {
			continue
		}

		return string(matches[1]), nil
	}

	return "", fmt.Errorf("failed to find semver in factory bytecode")
}

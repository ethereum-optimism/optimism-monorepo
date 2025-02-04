package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

var (
	// Use HexToHash(...).Hex() to ensure the strings are the correct length for a hash
	l1HeadValue        = common.HexToHash("0x111111").Hex()
	l2HeadValue        = common.HexToHash("0x222222").Hex()
	l2ClaimValue       = common.HexToHash("0x333333").Hex()
	l2OutputRoot       = common.HexToHash("0x444444").Hex()
	l2ClaimBlockNumber = uint64(1203)
	// Note: This is actually the L1 Sepolia genesis config. Just using it as an arbitrary, valid genesis config
	l2Genesis       = core.DefaultSepoliaGenesisBlock()
	l2GenesisConfig = l2Genesis.Config
)

func TestLogLevel(t *testing.T) {
	t.Run("RejectInvalid", func(t *testing.T) {
		verifyArgsInvalid(t, "unknown level: foo", addRequiredArgs("--log.level=foo"))
	})

	for _, lvl := range []string{"trace", "debug", "info", "error", "crit"} {
		lvl := lvl
		t.Run("AcceptValid_"+lvl, func(t *testing.T) {
			logger, _, err := runWithArgs(addRequiredArgs("--log.level", lvl))
			require.NoError(t, err)
			require.NotNil(t, logger)
		})
	}
}

func TestLogFormat(t *testing.T) {
	t.Run("RejectInvalid", func(t *testing.T) {
		verifyArgsInvalid(t, `unrecognized log-format: "foo"`, addRequiredArgs("--log.format=foo"))
	})

	for _, lvl := range []string{
		oplog.FormatJSON.String(),
		oplog.FormatTerminal.String(),
		oplog.FormatText.String(),
		oplog.FormatLogFmt.String(),
	} {
		lvl := lvl
		t.Run("AcceptValid_"+lvl, func(t *testing.T) {
			logger, _, err := runWithArgs(addRequiredArgs("--log.format", lvl))
			require.NoError(t, err)
			require.NotNil(t, logger)
		})
	}
}

func TestDefaultCLIOptionsMatchDefaultConfig(t *testing.T) {
	cfg := configForArgs(t, addRequiredArgs())
	rollupCfg, err := chaincfg.GetRollupConfig("op-sepolia")
	require.NoError(t, err)
	defaultCfg := config.NewSingleChainConfig(
		rollupCfg,
		chainconfig.OPSepoliaChainConfig(),
		common.HexToHash(l1HeadValue),
		common.HexToHash(l2HeadValue),
		common.HexToHash(l2OutputRoot),
		common.HexToHash(l2ClaimValue),
		l2ClaimBlockNumber)
	require.Equal(t, defaultCfg, cfg)
}

func TestNetwork(t *testing.T) {
	t.Run("Unknown", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid network: \"bar\"", replaceRequiredArg("--network", "bar"))
	})

	t.Run("AllowNetworkAndRollupConfig", func(t *testing.T) {
		configFile, rollupCfg := writeRollupConfigWithChainID(t, 4297842)
		cfg := configForArgs(t, addRequiredArgs("--rollup.config", configFile))
		require.Equal(t, []*rollup.Config{chaincfg.OPSepolia(), rollupCfg}, cfg.Rollups)
	})

	t.Run("RollupConfig", func(t *testing.T) {
		configFile := writeValidRollupConfig(t)
		genesisFile := writeValidGenesis(t)

		cfg := configForArgs(t, addRequiredArgsExcept("--network", "--rollup.config", configFile, "--l2.genesis", genesisFile))
		require.Len(t, cfg.Rollups, 1)
		require.Equal(t, *chaincfg.OPSepolia(), *cfg.Rollups[0])
	})

	t.Run("Multiple", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept("--network", "--network=op-mainnet,op-sepolia"))
		require.Len(t, cfg.Rollups, 2)
		opMainnetCfg, err := chaincfg.GetRollupConfig("op-mainnet")
		require.NoError(t, err)
		require.Equal(t, *opMainnetCfg, *cfg.Rollups[0])
		require.Equal(t, *chaincfg.OPSepolia(), *cfg.Rollups[1])
	})

	for _, name := range chaincfg.AvailableNetworks() {
		name := name
		expected, err := chaincfg.GetRollupConfig(name)
		require.NoError(t, err)
		t.Run("Network_"+name, func(t *testing.T) {
			args := replaceRequiredArg("--network", name)
			cfg := configForArgs(t, args)
			require.Len(t, cfg.Rollups, 1)
			require.Equal(t, *expected, *cfg.Rollups[0])
		})
	}
}

func TestDataDir(t *testing.T) {
	expected := "/tmp/mainTestDataDir"
	cfg := configForArgs(t, addRequiredArgs("--datadir", expected))
	require.Equal(t, expected, cfg.DataDir)
}

func TestDataFormat(t *testing.T) {
	for _, format := range types.SupportedDataFormats {
		format := format
		t.Run(fmt.Sprintf("Valid-%v", format), func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgs("--data.format", string(format)))
			require.Equal(t, format, cfg.DataFormat)
		})
	}

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid data format: foo", addRequiredArgs("--data.format", "foo"))
	})
}

func TestL2(t *testing.T) {
	t.Run("Single", func(t *testing.T) {
		expected := "https://example.com:8545"
		cfg := configForArgs(t, addRequiredArgs("--l2", expected))
		require.Equal(t, []string{expected}, cfg.L2URLs)
	})

	t.Run("Multiple", func(t *testing.T) {
		expected := []string{"https://example.com:8545", "https://example.com:9000"}
		cfg := configForArgs(t, addRequiredArgs("--l2", strings.Join(expected, ",")))
		require.Equal(t, expected, cfg.L2URLs)
	})
}

func TestL2Genesis(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		rollupCfgFile := writeValidRollupConfig(t)
		genesisFile := writeValidGenesis(t)
		cfg := configForArgs(t, addRequiredArgsExcept("--network", "--rollup.config", rollupCfgFile, "--l2.genesis", genesisFile))
		require.Equal(t, []*params.ChainConfig{l2GenesisConfig}, cfg.L2ChainConfigs)
	})

	t.Run("NotRequiredForSepolia", func(t *testing.T) {
		cfg := configForArgs(t, replaceRequiredArg("--network", "sepolia"))
		require.Equal(t, []*params.ChainConfig{chainconfig.OPSepoliaChainConfig()}, cfg.L2ChainConfigs)
	})
}

func TestMultipleNetworkConfigs(t *testing.T) {
	t.Run("MultipleCustomChains", func(t *testing.T) {
		rollupFile1, rollupCfg1 := writeRollupConfigWithChainID(t, 1)
		rollupFile2, rollupCfg2 := writeRollupConfigWithChainID(t, 2)
		genesisFile1, chainCfg1 := writeGenesisFileWithChainID(t, 1)
		genesisFile2, chainCfg2 := writeGenesisFileWithChainID(t, 2)
		cfg := configForArgs(t, addRequiredArgsExcept("--network",
			"--rollup.config", rollupFile1+","+rollupFile2,
			"--l2.genesis", genesisFile1+","+genesisFile2))
		require.Equal(t, []*rollup.Config{rollupCfg1, rollupCfg2}, cfg.Rollups)
		require.Equal(t, []*params.ChainConfig{chainCfg1, chainCfg2}, cfg.L2ChainConfigs)
	})

	t.Run("MixNetworkAndCustomChains", func(t *testing.T) {
		rollupFile, rollupCfg := writeRollupConfigWithChainID(t, 1)
		genesisFile, chainCfg := writeGenesisFileWithChainID(t, 1)
		cfg := configForArgs(t, addRequiredArgsExcept("--network",
			"--network", "op-sepolia",
			"--rollup.config", rollupFile,
			"--l2.genesis", genesisFile))
		require.Equal(t, []*rollup.Config{chaincfg.OPSepolia(), rollupCfg}, cfg.Rollups)
		require.Equal(t, []*params.ChainConfig{chainconfig.OPSepoliaChainConfig(), chainCfg}, cfg.L2ChainConfigs)
	})
}

func TestL2ChainID(t *testing.T) {
	t.Run("DefaultToNetworkChainID", func(t *testing.T) {
		cfg := configForArgs(t, replaceRequiredArg("--network", "op-mainnet"))
		require.Equal(t, eth.ChainIDFromUInt64(10), cfg.L2ChainID)
	})

	t.Run("DefaultToGenesisChainID", func(t *testing.T) {
		rollupCfgFile := writeValidRollupConfig(t)
		genesisFile := writeValidGenesis(t)
		cfg := configForArgs(t, addRequiredArgsExcept("--network", "--rollup.config", rollupCfgFile, "--l2.genesis", genesisFile))
		require.Equal(t, eth.ChainIDFromBig(l2GenesisConfig.ChainID), cfg.L2ChainID)
	})

	t.Run("OverrideToCustomIndicator", func(t *testing.T) {
		rollupCfgFile := writeValidRollupConfig(t)
		genesisFile := writeValidGenesis(t)
		cfg := configForArgs(t, addRequiredArgsExcept("--network",
			"--rollup.config", rollupCfgFile,
			"--l2.genesis", genesisFile,
			"--l2.custom"))
		require.Equal(t, boot.CustomChainIDIndicator, cfg.L2ChainID)
	})

	t.Run("ZeroWhenMultipleL2ChainsSpecified", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept("--network", "--network", "op-sepolia,op-mainnet"))
		require.Zero(t, cfg.L2ChainID)
	})
}

func TestL2Head(t *testing.T) {
	t.Run("RequiredWithOutputRoot", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l2.head is required when l2.outputroot is specified", addRequiredArgsExcept("--l2.head"))
	})

	t.Run("NotAllowedWithAgreedPrestate", func(t *testing.T) {
		req := requiredArgs()
		delete(req, "--l2.head")
		delete(req, "--l2.outputroot")
		args := append(toArgList(req), "--l2.head", l2HeadValue, "--l2.agreed-prestate", "0x1234")
		verifyArgsInvalid(t, "flag l2.head and l2.agreed-prestate must not be specified together", args)
	})

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, replaceRequiredArg("--l2.head", l2HeadValue))
		require.Equal(t, common.HexToHash(l2HeadValue), cfg.L2Head)
	})

	t.Run("NotRequiredForInterop", func(t *testing.T) {
		req := requiredArgs()
		delete(req, "--l2.head")
		delete(req, "--l2.outputroot")
		args := append(toArgList(req), "--l2.agreed-prestate", "0x1234")
		cfg := configForArgs(t, args)
		require.Equal(t, common.Hash{}, cfg.L2Head)
		require.True(t, cfg.InteropEnabled)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, config.ErrInvalidL2Head.Error(), replaceRequiredArg("--l2.head", "something"))
	})
}

func TestL2OutputRoot(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l2.outputroot or l2.agreed-prestate is required", addRequiredArgsExcept("--l2.outputroot"))
	})

	t.Run("NotRequiredWhenAgreedPrestateProvided", func(t *testing.T) {
		configForArgs(t, addRequiredArgsExceptMultiple([]string{"--l2.outputroot", "--l2.head"}, "--l2.agreed-prestate", "0x1234"))
	})

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, replaceRequiredArg("--l2.outputroot", l2OutputRoot))
		require.Equal(t, common.HexToHash(l2OutputRoot), cfg.L2OutputRoot)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, config.ErrInvalidL2OutputRoot.Error(), replaceRequiredArg("--l2.outputroot", "something"))
	})
}

func TestL2AgreedPrestate(t *testing.T) {
	t.Run("NotRequiredWhenL2OutputRootProvided", func(t *testing.T) {
		configForArgs(t, addRequiredArgsExceptMultiple([]string{"--l2.outputroot", "--l2.head"}, "--l2.agreed-prestate", "0x1234"))
	})

	t.Run("Valid", func(t *testing.T) {
		prestate := "0x1234"
		prestateBytes := common.FromHex(prestate)
		expectedOutputRoot := crypto.Keccak256Hash(prestateBytes)
		cfg := configForArgs(t, addRequiredArgsExceptMultiple([]string{"--l2.outputroot", "--l2.head"}, "--l2.agreed-prestate", prestate))
		require.Equal(t, expectedOutputRoot, cfg.L2OutputRoot)
		require.Equal(t, prestateBytes, cfg.AgreedPrestate)
	})

	t.Run("MustNotSpecifyWithL2OutputRoot", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l2.outputroot and l2.agreed-prestate must not be specified together", addRequiredArgs("--l2.agreed-prestate", "0x1234"))
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, config.ErrInvalidAgreedPrestate.Error(), addRequiredArgsExceptMultiple([]string{"--l2.outputroot", "--l2.head"}, "--l2.agreed-prestate", "something"))
	})

	t.Run("ZeroLength", func(t *testing.T) {
		verifyArgsInvalid(t, config.ErrInvalidAgreedPrestate.Error(), addRequiredArgsExceptMultiple([]string{"--l2.outputroot", "--l2.head"}, "--l2.agreed-prestate", "0x"))
	})
}

func TestL1Head(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l1.head is required", addRequiredArgsExcept("--l1.head"))
	})

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, replaceRequiredArg("--l1.head", l1HeadValue))
		require.Equal(t, common.HexToHash(l1HeadValue), cfg.L1Head)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, config.ErrInvalidL1Head.Error(), replaceRequiredArg("--l1.head", "something"))
	})
}

func TestL1(t *testing.T) {
	expected := "https://example.com:8545"
	cfg := configForArgs(t, addRequiredArgs("--l1", expected))
	require.Equal(t, expected, cfg.L1URL)
}

func TestL1TrustRPC(t *testing.T) {
	t.Run("DefaultFalse", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs())
		require.False(t, cfg.L1TrustRPC)
	})
	t.Run("Enabled", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs("--l1.trustrpc"))
		require.True(t, cfg.L1TrustRPC)
	})
	t.Run("EnabledWithArg", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs("--l1.trustrpc=true"))
		require.True(t, cfg.L1TrustRPC)
	})
	t.Run("Disabled", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs("--l1.trustrpc=false"))
		require.False(t, cfg.L1TrustRPC)
	})
}

func TestL1RPCKind(t *testing.T) {
	t.Run("DefaultBasic", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs())
		require.Equal(t, sources.RPCKindStandard, cfg.L1RPCKind)
	})
	for _, kind := range sources.RPCProviderKinds {
		t.Run(kind.String(), func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgs("--l1.rpckind", kind.String()))
			require.Equal(t, kind, cfg.L1RPCKind)
		})
	}
	t.Run("RequireLowercase", func(t *testing.T) {
		verifyArgsInvalid(t, "rpc kind", addRequiredArgs("--l1.rpckind", "AlChemY"))
	})
	t.Run("UnknownKind", func(t *testing.T) {
		verifyArgsInvalid(t, "\"foo\"", addRequiredArgs("--l1.rpckind", "foo"))
	})
}

func TestL2Claim(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l2.claim is required", addRequiredArgsExcept("--l2.claim"))
	})

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, replaceRequiredArg("--l2.claim", l2ClaimValue))
		require.EqualValues(t, common.HexToHash(l2ClaimValue), cfg.L2Claim)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, config.ErrInvalidL2Claim.Error(), replaceRequiredArg("--l2.claim", "something"))
	})

	t.Run("Allows all zero without prefix", func(t *testing.T) {
		cfg := configForArgs(t, replaceRequiredArg("--l2.claim", "0000000000000000000000000000000000000000000000000000000000000000"))
		require.EqualValues(t, common.Hash{}, cfg.L2Claim)
	})

	t.Run("Allows all zero with prefix", func(t *testing.T) {
		cfg := configForArgs(t, replaceRequiredArg("--l2.claim", "0x0000000000000000000000000000000000000000000000000000000000000000"))
		require.EqualValues(t, common.Hash{}, cfg.L2Claim)
	})
}

func TestL2Experimental(t *testing.T) {
	t.Run("DefaultEmpty", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs())
		require.Len(t, cfg.L2ExperimentalURLs, 0)
	})

	t.Run("Valid", func(t *testing.T) {
		expected := "https://example.com:8545"
		cfg := configForArgs(t, addRequiredArgs("--l2.experimental", expected))
		require.EqualValues(t, []string{expected}, cfg.L2ExperimentalURLs)
	})

	t.Run("Multiple", func(t *testing.T) {
		expected := []string{"https://example.com:8545", "https://example.com:9000"}
		cfg := configForArgs(t, addRequiredArgs("--l2.experimental", strings.Join(expected, ",")))
		require.EqualValues(t, expected, cfg.L2ExperimentalURLs)
	})
}

func TestL2BlockNumber(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l2.blocknumber is required", addRequiredArgsExcept("--l2.blocknumber"))
	})

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, replaceRequiredArg("--l2.blocknumber", strconv.FormatUint(l2ClaimBlockNumber, 10)))
		require.EqualValues(t, l2ClaimBlockNumber, cfg.L2ClaimBlockNumber)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid value \"something\" for flag -l2.blocknumber", replaceRequiredArg("--l2.blocknumber", "something"))
	})
}

func TestExec(t *testing.T) {
	t.Run("DefaultEmpty", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs())
		require.Equal(t, "", cfg.ExecCmd)
	})
	t.Run("Set", func(t *testing.T) {
		cmd := "/bin/echo"
		cfg := configForArgs(t, addRequiredArgs("--exec", cmd))
		require.Equal(t, cmd, cfg.ExecCmd)
	})
}

func TestServerMode(t *testing.T) {
	t.Run("DefaultFalse", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs())
		require.False(t, cfg.ServerMode)
	})
	t.Run("Enabled", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs("--server"))
		require.True(t, cfg.ServerMode)
	})
	t.Run("EnabledWithArg", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs("--server=true"))
		require.True(t, cfg.ServerMode)
	})
	t.Run("DisabledWithArg", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs("--server=false"))
		require.False(t, cfg.ServerMode)
	})
	t.Run("InvalidArg", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid boolean value \"foo\" for -server", addRequiredArgs("--server=foo"))
	})
}

func verifyArgsInvalid(t *testing.T, messageContains string, cliArgs []string) {
	_, _, err := runWithArgs(cliArgs)
	require.ErrorContains(t, err, messageContains)
}

func configForArgs(t *testing.T, cliArgs []string) *config.Config {
	_, cfg, err := runWithArgs(cliArgs)
	require.NoError(t, err)
	return cfg
}

func runWithArgs(cliArgs []string) (log.Logger, *config.Config, error) {
	cfg := new(config.Config)
	var logger log.Logger
	fullArgs := append([]string{"op-program"}, cliArgs...)
	err := run(fullArgs, func(log log.Logger, config *config.Config) error {
		logger = log
		cfg = config
		return nil
	})
	return logger, cfg, err
}

func addRequiredArgs(args ...string) []string {
	req := requiredArgs()
	combined := toArgList(req)
	return append(combined, args...)
}

func addRequiredArgsExcept(name string, optionalArgs ...string) []string {
	req := requiredArgs()
	delete(req, name)
	return append(toArgList(req), optionalArgs...)
}

func addRequiredArgsExceptMultiple(remove []string, optionalArgs ...string) []string {
	req := requiredArgs()
	for _, name := range remove {
		delete(req, name)
	}
	return append(toArgList(req), optionalArgs...)
}

func replaceRequiredArg(name string, value string) []string {
	req := requiredArgs()
	req[name] = value
	return toArgList(req)
}

// requiredArgs returns map of argument names to values which are the minimal arguments required
// to create a valid Config
func requiredArgs() map[string]string {
	return map[string]string{
		"--network":        "sepolia",
		"--l1.head":        l1HeadValue,
		"--l2.head":        l2HeadValue,
		"--l2.outputroot":  l2OutputRoot,
		"--l2.claim":       l2ClaimValue,
		"--l2.blocknumber": strconv.FormatUint(l2ClaimBlockNumber, 10),
	}
}

func writeValidGenesis(t *testing.T) string {
	genesis := l2Genesis
	return writeGenesis(t, genesis)
}

func writeGenesisFileWithChainID(t *testing.T, chainID uint64) (string, *params.ChainConfig) {
	genesis := *l2Genesis
	chainCfg := *genesis.Config
	chainCfg.ChainID = new(big.Int).SetUint64(chainID)
	genesis.Config = &chainCfg
	return writeGenesis(t, &genesis), &chainCfg
}

func writeGenesis(t *testing.T, genesis *core.Genesis) string {
	dir := t.TempDir()
	j, err := json.Marshal(genesis)
	require.NoError(t, err)
	genesisFile := dir + "/genesis.json"
	require.NoError(t, os.WriteFile(genesisFile, j, 0666))
	return genesisFile
}

func writeValidRollupConfig(t *testing.T) string {
	return writeRollupConfig(t, chaincfg.OPSepolia())
}

func writeRollupConfigWithChainID(t *testing.T, chainID uint64) (string, *rollup.Config) {
	rollupCfg := *chaincfg.OPSepolia()
	rollupCfg.L2ChainID = new(big.Int).SetUint64(chainID)
	return writeRollupConfig(t, &rollupCfg), &rollupCfg
}

func writeRollupConfig(t *testing.T, rollupCfg *rollup.Config) string {
	dir := t.TempDir()
	j, err := json.Marshal(&rollupCfg)
	require.NoError(t, err)
	cfgFile := dir + "/rollup.json"
	require.NoError(t, os.WriteFile(cfgFile, j, 0666))
	return cfgFile
}

func toArgList(req map[string]string) []string {
	var combined []string
	for name, value := range req {
		combined = append(combined, name)
		combined = append(combined, value)
	}
	return combined
}

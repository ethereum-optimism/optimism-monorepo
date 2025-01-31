package config

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

var (
	validRollupConfig    = chaincfg.OPSepolia()
	validL2Genesis       = chainconfig.OPSepoliaChainConfig()
	validL1Head          = common.Hash{0xaa}
	validL2Head          = common.Hash{0xbb}
	validL2Claim         = common.Hash{0xcc}
	validL2OutputRoot    = common.Hash{0xdd}
	validL2ClaimBlockNum = uint64(15)
	validAgreedPrestate  = []byte{1}
)

// TestValidConfigIsValid checks that the config provided by validConfig is actually valid
func TestValidConfigIsValid(t *testing.T) {
	err := validConfig().Check()
	require.NoError(t, err)
}

// TestValidInteropConfigIsValid checks that the config provided by validInteropConfig is actually valid
func TestValidInteropConfigIsValid(t *testing.T) {
	err := validInteropConfig().Check()
	require.NoError(t, err)
}

func TestL2BlockNum(t *testing.T) {
	t.Run("RequiredForPreInterop", func(t *testing.T) {
		cfg := validConfig()
		cfg.L2ChainID = eth.ChainID{}
		require.ErrorIs(t, cfg.Check(), ErrMissingL2ChainID)
	})

	t.Run("NotRequiredForInterop", func(t *testing.T) {
		cfg := validInteropConfig()
		cfg.L2ChainID = eth.ChainID{}
		require.NoError(t, cfg.Check())
	})
}

func TestRollupConfig(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		config := validConfig()
		config.Rollups = nil
		err := config.Check()
		require.ErrorIs(t, err, ErrNoL2Chains)
	})

	t.Run("Invalid", func(t *testing.T) {
		config := validConfig()
		config.Rollups = []*rollup.Config{{}}
		err := config.Check()
		require.ErrorIs(t, err, rollup.ErrBlockTimeZero)
	})

	t.Run("DisallowDuplicates", func(t *testing.T) {
		cfg := validConfig()
		cfg.Rollups = append(cfg.Rollups, validRollupConfig)
		require.ErrorIs(t, cfg.Check(), ErrDuplicateRollup)
	})
}

func TestL1HeadRequired(t *testing.T) {
	config := validConfig()
	config.L1Head = common.Hash{}
	err := config.Check()
	require.ErrorIs(t, err, ErrInvalidL1Head)
}

func TestL2Head(t *testing.T) {
	t.Run("RequiredPreInterop", func(t *testing.T) {
		config := validConfig()
		config.L2Head = common.Hash{}
		err := config.Check()
		require.ErrorIs(t, err, ErrInvalidL2Head)
	})

	t.Run("NotRequiredForInterop", func(t *testing.T) {
		config := validInteropConfig()
		config.L2Head = common.Hash{}
		err := config.Check()
		require.NoError(t, err)
	})
}

func TestL2OutputRootRequired(t *testing.T) {
	config := validConfig()
	config.L2OutputRoot = common.Hash{}
	err := config.Check()
	require.ErrorIs(t, err, ErrInvalidL2OutputRoot)
}

// The L2 claim may be provided by a dishonest actor so we must treat 0x00...00 as a real value.
func TestL2ClaimMayBeDefaultValue(t *testing.T) {
	config := validConfig()
	config.L2Claim = common.Hash{}
	require.NoError(t, config.Check())
}

func TestL2ClaimBlockNumberRequired(t *testing.T) {
	config := validConfig()
	config.L2ClaimBlockNumber = 0
	err := config.Check()
	require.ErrorIs(t, err, ErrInvalidL2ClaimBlock)
}

func TestL2GenesisRequired(t *testing.T) {
	config := validConfig()
	config.L2ChainConfigs = nil
	err := config.Check()
	require.ErrorIs(t, err, ErrMissingL2Genesis)
}

func TestL2Genesis_ExtraGenesisProvided(t *testing.T) {
	config := validConfig()
	config.L2ChainConfigs = append(config.L2ChainConfigs, &params.ChainConfig{ChainID: big.NewInt(422142)})
	require.ErrorIs(t, config.Check(), ErrNoRollupForGenesis)
}

func TestL2Genesis_GenesisMissingForChain(t *testing.T) {
	config := validConfig()
	secondConfig := *chaincfg.OPSepolia()
	secondConfig.L2ChainID = big.NewInt(422142)
	config.Rollups = append(config.Rollups, &secondConfig)
	require.ErrorIs(t, config.Check(), ErrNoGenesisForRollup)
}

func TestL2Genesis_Duplicate(t *testing.T) {
	config := validConfig()
	config.L2ChainConfigs = append(config.L2ChainConfigs, validL2Genesis)
	require.ErrorIs(t, config.Check(), ErrDuplicateGenesis)
}

func TestFetchingArgConsistency(t *testing.T) {
	t.Run("RequireL2WhenL1Set", func(t *testing.T) {
		cfg := validConfig()
		cfg.L1URL = "https://example.com:1234"
		require.ErrorIs(t, cfg.Check(), ErrL1AndL2Inconsistent)
	})
	t.Run("RequireL1WhenL2Set", func(t *testing.T) {
		cfg := validConfig()
		cfg.L2URLs = []string{"https://example.com:1234"}
		require.ErrorIs(t, cfg.Check(), ErrL1AndL2Inconsistent)
	})
	t.Run("AllowNeitherSet", func(t *testing.T) {
		cfg := validConfig()
		cfg.L1URL = ""
		cfg.L2URLs = []string{}
		require.NoError(t, cfg.Check())
	})
	t.Run("AllowNeitherSetNil", func(t *testing.T) {
		cfg := validConfig()
		cfg.L1URL = ""
		cfg.L2URLs = nil
		require.NoError(t, cfg.Check())
	})
	t.Run("AllowBothSet", func(t *testing.T) {
		cfg := validConfig()
		cfg.L1URL = "https://example.com:1234"
		cfg.L2URLs = []string{"https://example.com:4678"}
		require.NoError(t, cfg.Check())
	})
}

func TestFetchingEnabled(t *testing.T) {
	t.Run("FetchingNotEnabledWhenNoFetcherUrlsSpecified", func(t *testing.T) {
		cfg := validConfig()
		require.False(t, cfg.FetchingEnabled(), "Should not enable fetching when node URL not supplied")
	})

	t.Run("FetchingEnabledWhenFetcherUrlsSpecified", func(t *testing.T) {
		cfg := validConfig()
		cfg.L2URLs = []string{"https://example.com:1234"}
		require.False(t, cfg.FetchingEnabled(), "Should not enable fetching when node URL not supplied")
	})

	t.Run("FetchingNotEnabledWhenNoL1UrlSpecified", func(t *testing.T) {
		cfg := validConfig()
		cfg.L2URLs = []string{"https://example.com:1234"}
		require.False(t, cfg.FetchingEnabled(), "Should not enable L1 fetching when L1 node URL not supplied")
	})

	t.Run("FetchingNotEnabledWhenNoL2UrlSpecified", func(t *testing.T) {
		cfg := validConfig()
		cfg.L1URL = "https://example.com:1234"
		require.False(t, cfg.FetchingEnabled(), "Should not enable L2 fetching when L2 node URL not supplied")
	})

	t.Run("FetchingEnabledWhenBothFetcherUrlsSpecified", func(t *testing.T) {
		cfg := validConfig()
		cfg.L1URL = "https://example.com:1234"
		cfg.L1BeaconURL = "https://example.com:5678"
		cfg.L2URLs = []string{"https://example.com:91011"}
		require.True(t, cfg.FetchingEnabled(), "Should enable fetching when node URL supplied")
	})
}

func TestRequireDataDirInNonFetchingMode(t *testing.T) {
	cfg := validConfig()
	cfg.DataDir = ""
	cfg.L1URL = ""
	cfg.L2URLs = nil
	err := cfg.Check()
	require.ErrorIs(t, err, ErrDataDirRequired)
}

func TestRejectExecAndServerMode(t *testing.T) {
	cfg := validConfig()
	cfg.ServerMode = true
	cfg.ExecCmd = "echo"
	err := cfg.Check()
	require.ErrorIs(t, err, ErrNoExecInServerMode)
}

func TestCustomL2ChainID(t *testing.T) {
	t.Run("nonCustom", func(t *testing.T) {
		cfg := validConfig()
		require.Equal(t, cfg.L2ChainID, eth.ChainIDFromBig(validL2Genesis.ChainID))
	})
	t.Run("custom", func(t *testing.T) {
		customChainConfig := &params.ChainConfig{ChainID: big.NewInt(0x1212121212)}
		cfg := NewSingleChainConfig(validRollupConfig, customChainConfig, validL1Head, validL2Head, validL2OutputRoot, validL2Claim, validL2ClaimBlockNum)
		require.Equal(t, cfg.L2ChainID, boot.CustomChainIDIndicator)
	})
}

func TestAgreedPrestate(t *testing.T) {
	t.Run("requiredWithInterop-nil", func(t *testing.T) {
		cfg := validConfig()
		cfg.InteropEnabled = true
		cfg.AgreedPrestate = nil
		err := cfg.Check()
		require.ErrorIs(t, err, ErrMissingAgreedPrestate)
	})
	t.Run("requiredWithInterop-empty", func(t *testing.T) {
		cfg := validConfig()
		cfg.InteropEnabled = true
		cfg.AgreedPrestate = []byte{}
		err := cfg.Check()
		require.ErrorIs(t, err, ErrMissingAgreedPrestate)
	})

	t.Run("notRequiredWithoutInterop", func(t *testing.T) {
		cfg := validConfig()
		cfg.AgreedPrestate = nil
		require.NoError(t, cfg.Check())
	})

	t.Run("valid", func(t *testing.T) {
		cfg := validConfig()
		cfg.InteropEnabled = true
		cfg.AgreedPrestate = []byte{1}
		cfg.L2OutputRoot = crypto.Keccak256Hash(cfg.AgreedPrestate)
		require.NoError(t, cfg.Check())
	})

	t.Run("mustMatchL2OutputRoot", func(t *testing.T) {
		cfg := validConfig()
		cfg.InteropEnabled = true
		cfg.AgreedPrestate = []byte{1}
		cfg.L2OutputRoot = common.Hash{0xaa}
		require.ErrorIs(t, cfg.Check(), ErrInvalidAgreedPrestate)
	})
}

func TestDBFormat(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		cfg := validConfig()
		cfg.DataFormat = "foo"
		require.ErrorIs(t, cfg.Check(), ErrInvalidDataFormat)
	})
	for _, format := range types.SupportedDataFormats {
		format := format
		t.Run(fmt.Sprintf("%v", format), func(t *testing.T) {
			cfg := validConfig()
			cfg.DataFormat = format
			require.NoError(t, cfg.Check())
		})
	}
}

func validConfig() *Config {
	cfg := NewSingleChainConfig(validRollupConfig, validL2Genesis, validL1Head, validL2Head, validL2OutputRoot, validL2Claim, validL2ClaimBlockNum)
	cfg.DataDir = "/tmp/configTest"
	return cfg
}

func validInteropConfig() *Config {
	cfg := validConfig()
	cfg.InteropEnabled = true
	cfg.AgreedPrestate = validAgreedPrestate
	cfg.L2OutputRoot = crypto.Keccak256Hash(cfg.AgreedPrestate)
	return cfg
}

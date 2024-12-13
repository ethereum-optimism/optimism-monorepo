package config

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/chaincfg"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-program/chainconfig"
	"github.com/ethereum-optimism/optimism/op-program/client"
	"github.com/ethereum-optimism/optimism/op-program/host/types"
	"github.com/ethereum/go-ethereum/common"
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
)

// TestValidConfigIsValid checks that the config provided by validConfig is actually valid
func TestValidConfigIsValid(t *testing.T) {
	err := validConfig().Check()
	require.NoError(t, err)
}

func TestRollupConfig(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		config := validConfig()
		config.Rollup = nil
		err := config.Check()
		require.ErrorIs(t, err, ErrMissingRollupConfig)
	})

	t.Run("Invalid", func(t *testing.T) {
		config := validConfig()
		config.Rollup = &rollup.Config{}
		err := config.Check()
		require.ErrorIs(t, err, rollup.ErrBlockTimeZero)
	})
}

func TestL1HeadRequired(t *testing.T) {
	config := validConfig()
	config.L1Head = common.Hash{}
	err := config.Check()
	require.ErrorIs(t, err, ErrInvalidL1Head)
}

func TestL2HeadRequired(t *testing.T) {
	config := validConfig()
	config.L2Head = common.Hash{}
	err := config.Check()
	require.ErrorIs(t, err, ErrInvalidL2Head)
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
	config.L2ChainConfig = nil
	err := config.Check()
	require.ErrorIs(t, err, ErrMissingL2Genesis)
}

func TestFetchingArgConsistency(t *testing.T) {
	t.Run("RequireL2WhenL1Set", func(t *testing.T) {
		cfg := validConfig()
		cfg.L1URL = "https://example.com:1234"
		require.ErrorIs(t, cfg.Check(), ErrL1AndL2Inconsistent)
	})
	t.Run("RequireL1WhenL2Set", func(t *testing.T) {
		cfg := validConfig()
		cfg.L2URL = "https://example.com:1234"
		require.ErrorIs(t, cfg.Check(), ErrL1AndL2Inconsistent)
	})
	t.Run("AllowNeitherSet", func(t *testing.T) {
		cfg := validConfig()
		cfg.L1URL = ""
		cfg.L2URL = ""
		require.NoError(t, cfg.Check())
	})
	t.Run("AllowBothSet", func(t *testing.T) {
		cfg := validConfig()
		cfg.L1URL = "https://example.com:1234"
		cfg.L2URL = "https://example.com:4678"
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
		cfg.L2URL = "https://example.com:1234"
		require.False(t, cfg.FetchingEnabled(), "Should not enable fetching when node URL not supplied")
	})

	t.Run("FetchingNotEnabledWhenNoL1UrlSpecified", func(t *testing.T) {
		cfg := validConfig()
		cfg.L2URL = "https://example.com:1234"
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
		cfg.L2URL = "https://example.com:91011"
		require.True(t, cfg.FetchingEnabled(), "Should enable fetching when node URL supplied")
	})
}

func TestRequireDataDirInNonFetchingMode(t *testing.T) {
	cfg := validConfig()
	cfg.DataDir = ""
	cfg.L1URL = ""
	cfg.L2URL = ""
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
		require.Equal(t, cfg.L2ChainID, validL2Genesis.ChainID.Uint64())
	})
	t.Run("custom", func(t *testing.T) {
		customChainConfig := &params.ChainConfig{ChainID: big.NewInt(0x1212121212)}
		cfg := NewConfig(validRollupConfig, customChainConfig, validL1Head, validL2Head, validL2OutputRoot, validL2Claim, validL2ClaimBlockNum)
		require.Equal(t, cfg.L2ChainID, client.CustomChainIDIndicator)
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
	cfg := NewConfig(validRollupConfig, validL2Genesis, validL1Head, validL2Head, validL2OutputRoot, validL2Claim, validL2ClaimBlockNum)
	cfg.DataDir = "/tmp/configTest"
	return cfg
}

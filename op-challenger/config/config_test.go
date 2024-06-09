package config

import (
	"fmt"
	"net/url"
	"runtime"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

var (
	validL1EthRpc                        = "http://localhost:8545"
	validL1BeaconUrl                     = "http://localhost:9000"
	validGameFactoryAddress              = common.Address{0x23}
	validCannonBin                       = "./bin/cannon"
	validCannonOpProgramBin              = "./bin/op-program"
	validCannonNetwork                   = "mainnet"
	validCannonAbsolutPreState           = "pre.json"
	validCannonAbsolutPreStateBaseURL, _ = url.Parse("http://localhost/foo/")
	validDatadir                         = "/tmp/data"
	validL2Rpc                           = "http://localhost:9545"
	validRollupRpc                       = "http://localhost:8555"

	validAsteriscBin                       = "./bin/asterisc"
	validAsteriscOpProgramBin              = "./bin/op-program"
	validAsteriscNetwork                   = "mainnet"
	validAsteriscAbsolutPreState           = "pre.json"
	validAsteriscAbsolutPreStateBaseURL, _ = url.Parse("http://localhost/bar/")
)

var cannonTraceTypes = []TraceType{TraceTypeCannon, TraceTypePermissioned}
var asteriscTraceTypes = []TraceType{TraceTypeAsterisc}

func applyValidConfigForCannon(cfg *Config) {
	cfg.CannonConfig.VmBin = validCannonBin
	cfg.CannonConfig.Server = validCannonOpProgramBin
	cfg.CannonAbsolutePreStateBaseURL = validCannonAbsolutPreStateBaseURL
	cfg.CannonConfig.Network = validCannonNetwork
}

func applyValidConfigForAsterisc(cfg *Config) {
	cfg.AsteriscConfig.VmBin = validAsteriscBin
	cfg.AsteriscConfig.Server = validAsteriscOpProgramBin
	cfg.AsteriscAbsolutePreStateBaseURL = validAsteriscAbsolutPreStateBaseURL
	cfg.AsteriscConfig.Network = validAsteriscNetwork
}

func validConfig(traceType TraceType) Config {
	cfg := NewConfig(validGameFactoryAddress, validL1EthRpc, validL1BeaconUrl, validRollupRpc, validL2Rpc, validDatadir, traceType)
	if traceType == TraceTypeCannon || traceType == TraceTypePermissioned {
		applyValidConfigForCannon(&cfg)
	}
	if traceType == TraceTypeAsterisc {
		applyValidConfigForAsterisc(&cfg)
	}
	return cfg
}

// TestValidConfigIsValid checks that the config provided by validConfig is actually valid
func TestValidConfigIsValid(t *testing.T) {
	for _, traceType := range TraceTypes {
		traceType := traceType
		t.Run(traceType.String(), func(t *testing.T) {
			err := validConfig(traceType).Check()
			require.NoError(t, err)
		})
	}
}

func TestTxMgrConfig(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		config := validConfig(TraceTypeCannon)
		config.TxMgrConfig = txmgr.CLIConfig{}
		require.Equal(t, config.Check().Error(), "must provide a L1 RPC url")
	})
}

func TestL1EthRpcRequired(t *testing.T) {
	config := validConfig(TraceTypeCannon)
	config.L1EthRpc = ""
	require.ErrorIs(t, config.Check(), ErrMissingL1EthRPC)
}

func TestL1BeaconRequired(t *testing.T) {
	config := validConfig(TraceTypeCannon)
	config.L1Beacon = ""
	require.ErrorIs(t, config.Check(), ErrMissingL1Beacon)
}

func TestGameFactoryAddressRequired(t *testing.T) {
	config := validConfig(TraceTypeCannon)
	config.GameFactoryAddress = common.Address{}
	require.ErrorIs(t, config.Check(), ErrMissingGameFactoryAddress)
}

func TestSelectiveClaimResolutionNotRequired(t *testing.T) {
	config := validConfig(TraceTypeCannon)
	require.Equal(t, false, config.SelectiveClaimResolution)
	require.NoError(t, config.Check())
}

func TestGameAllowlistNotRequired(t *testing.T) {
	config := validConfig(TraceTypeCannon)
	config.GameAllowlist = []common.Address{}
	require.NoError(t, config.Check())
}

func TestCannonRequiredArgs(t *testing.T) {
	for _, traceType := range cannonTraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("TestCannonBinRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.CannonConfig.VmBin = ""
			require.ErrorIs(t, config.Check(), ErrMissingCannonBin)
		})

		t.Run(fmt.Sprintf("TestCannonServerRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.CannonConfig.Server = ""
			require.ErrorIs(t, config.Check(), ErrMissingCannonServer)
		})

		t.Run(fmt.Sprintf("TestCannonAbsolutePreStateOrBaseURLRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.CannonAbsolutePreState = ""
			config.CannonAbsolutePreStateBaseURL = nil
			require.ErrorIs(t, config.Check(), ErrMissingCannonAbsolutePreState)
		})

		t.Run(fmt.Sprintf("TestCannonAbsolutePreState-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.CannonAbsolutePreState = validCannonAbsolutPreState
			config.CannonAbsolutePreStateBaseURL = nil
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestCannonAbsolutePreStateBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.CannonAbsolutePreState = ""
			config.CannonAbsolutePreStateBaseURL = validCannonAbsolutPreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestMustNotSupplyBothCannonAbsolutePreStateAndBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.CannonAbsolutePreState = validCannonAbsolutPreState
			config.CannonAbsolutePreStateBaseURL = validCannonAbsolutPreStateBaseURL
			require.ErrorIs(t, config.Check(), ErrCannonAbsolutePreStateAndBaseURL)
		})

		t.Run(fmt.Sprintf("TestL2RpcRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.L2Rpc = ""
			require.ErrorIs(t, config.Check(), ErrMissingL2Rpc)
		})

		t.Run(fmt.Sprintf("TestCannonSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.CannonConfig.SnapshotFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingCannonSnapshotFreq)
			})
		})

		t.Run(fmt.Sprintf("TestCannonInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.CannonConfig.InfoFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingCannonInfoFreq)
			})
		})

		t.Run(fmt.Sprintf("TestCannonNetworkOrRollupConfigRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.CannonConfig.Network = ""
			cfg.CannonConfig.RollupConfigPath = ""
			cfg.CannonConfig.L2GenesisPath = "genesis.json"
			require.ErrorIs(t, cfg.Check(), ErrMissingCannonRollupConfig)
		})

		t.Run(fmt.Sprintf("TestCannonNetworkOrL2GenesisRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.CannonConfig.Network = ""
			cfg.CannonConfig.RollupConfigPath = "foo.json"
			cfg.CannonConfig.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrMissingCannonL2Genesis)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.CannonConfig.Network = validCannonNetwork
			cfg.CannonConfig.RollupConfigPath = "foo.json"
			cfg.CannonConfig.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrCannonNetworkAndRollupConfig)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndL2Genesis-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.CannonConfig.Network = validCannonNetwork
			cfg.CannonConfig.RollupConfigPath = ""
			cfg.CannonConfig.L2GenesisPath = "foo.json"
			require.ErrorIs(t, cfg.Check(), ErrCannonNetworkAndL2Genesis)
		})

		t.Run(fmt.Sprintf("TestNetworkMustBeValid-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.CannonConfig.Network = "unknown"
			require.ErrorIs(t, cfg.Check(), ErrCannonNetworkUnknown)
		})
	}
}

func TestAsteriscRequiredArgs(t *testing.T) {
	for _, traceType := range asteriscTraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("TestAsteriscBinRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscConfig.VmBin = ""
			require.ErrorIs(t, config.Check(), ErrMissingAsteriscBin)
		})

		t.Run(fmt.Sprintf("TestAsteriscServerRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscConfig.Server = ""
			require.ErrorIs(t, config.Check(), ErrMissingAsteriscServer)
		})

		t.Run(fmt.Sprintf("TestAsteriscAbsolutePreStateOrBaseURLRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscAbsolutePreState = ""
			config.AsteriscAbsolutePreStateBaseURL = nil
			require.ErrorIs(t, config.Check(), ErrMissingAsteriscAbsolutePreState)
		})

		t.Run(fmt.Sprintf("TestAsteriscAbsolutePreState-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscAbsolutePreState = validAsteriscAbsolutPreState
			config.AsteriscAbsolutePreStateBaseURL = nil
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAsteriscAbsolutePreStateBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscAbsolutePreState = ""
			config.AsteriscAbsolutePreStateBaseURL = validAsteriscAbsolutPreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestMustNotSupplyBothAsteriscAbsolutePreStateAndBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscAbsolutePreState = validAsteriscAbsolutPreState
			config.AsteriscAbsolutePreStateBaseURL = validAsteriscAbsolutPreStateBaseURL
			require.ErrorIs(t, config.Check(), ErrAsteriscAbsolutePreStateAndBaseURL)
		})

		t.Run(fmt.Sprintf("TestL2RpcRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.L2Rpc = ""
			require.ErrorIs(t, config.Check(), ErrMissingL2Rpc)
		})

		t.Run(fmt.Sprintf("TestAsteriscSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.AsteriscConfig.SnapshotFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscSnapshotFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.AsteriscConfig.InfoFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscInfoFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscNetworkOrRollupConfigRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscConfig.Network = ""
			cfg.AsteriscConfig.RollupConfigPath = ""
			cfg.AsteriscConfig.L2GenesisPath = "genesis.json"
			require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscRollupConfig)
		})

		t.Run(fmt.Sprintf("TestAsteriscNetworkOrL2GenesisRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscConfig.Network = ""
			cfg.AsteriscConfig.RollupConfigPath = "foo.json"
			cfg.AsteriscConfig.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscL2Genesis)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscConfig.Network = validAsteriscNetwork
			cfg.AsteriscConfig.RollupConfigPath = "foo.json"
			cfg.AsteriscConfig.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrAsteriscNetworkAndRollupConfig)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndL2Genesis-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscConfig.Network = validAsteriscNetwork
			cfg.AsteriscConfig.RollupConfigPath = ""
			cfg.AsteriscConfig.L2GenesisPath = "foo.json"
			require.ErrorIs(t, cfg.Check(), ErrAsteriscNetworkAndL2Genesis)
		})

		t.Run(fmt.Sprintf("TestNetworkMustBeValid-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscConfig.Network = "unknown"
			require.ErrorIs(t, cfg.Check(), ErrAsteriscNetworkUnknown)
		})
	}
}

func TestDatadirRequired(t *testing.T) {
	config := validConfig(TraceTypeAlphabet)
	config.Datadir = ""
	require.ErrorIs(t, config.Check(), ErrMissingDatadir)
}

func TestMaxConcurrency(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		config := validConfig(TraceTypeAlphabet)
		config.MaxConcurrency = 0
		require.ErrorIs(t, config.Check(), ErrMaxConcurrencyZero)
	})

	t.Run("DefaultToNumberOfCPUs", func(t *testing.T) {
		config := validConfig(TraceTypeAlphabet)
		require.EqualValues(t, runtime.NumCPU(), config.MaxConcurrency)
	})
}

func TestHttpPollInterval(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		config := validConfig(TraceTypeAlphabet)
		require.EqualValues(t, DefaultPollInterval, config.PollInterval)
	})
}

func TestRollupRpcRequired(t *testing.T) {
	for _, traceType := range TraceTypes {
		traceType := traceType
		t.Run(traceType.String(), func(t *testing.T) {
			config := validConfig(traceType)
			config.RollupRpc = ""
			require.ErrorIs(t, config.Check(), ErrMissingRollupRpc)
		})
	}
}

func TestRequireConfigForMultipleTraceTypesForCannon(t *testing.T) {
	cfg := validConfig(TraceTypeCannon)
	cfg.TraceTypes = []TraceType{TraceTypeCannon, TraceTypeAlphabet}
	// Set all required options and check its valid
	cfg.RollupRpc = validRollupRpc
	require.NoError(t, cfg.Check())

	// Require cannon specific args
	cfg.CannonAbsolutePreState = ""
	cfg.CannonAbsolutePreStateBaseURL = nil
	require.ErrorIs(t, cfg.Check(), ErrMissingCannonAbsolutePreState)
	cfg.CannonAbsolutePreState = validCannonAbsolutPreState

	// Require output cannon specific args
	cfg.RollupRpc = ""
	require.ErrorIs(t, cfg.Check(), ErrMissingRollupRpc)
}

func TestRequireConfigForMultipleTraceTypesForAsterisc(t *testing.T) {
	cfg := validConfig(TraceTypeAsterisc)
	cfg.TraceTypes = []TraceType{TraceTypeAsterisc, TraceTypeAlphabet}
	// Set all required options and check its valid
	cfg.RollupRpc = validRollupRpc
	require.NoError(t, cfg.Check())

	// Require asterisc specific args
	cfg.AsteriscAbsolutePreState = ""
	cfg.AsteriscAbsolutePreStateBaseURL = nil
	require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscAbsolutePreState)
	cfg.AsteriscAbsolutePreState = validAsteriscAbsolutPreState

	// Require output asterisc specific args
	cfg.RollupRpc = ""
	require.ErrorIs(t, cfg.Check(), ErrMissingRollupRpc)
}

func TestRequireConfigForMultipleTraceTypesForCannonAndAsterisc(t *testing.T) {
	cfg := validConfig(TraceTypeCannon)
	applyValidConfigForAsterisc(&cfg)

	cfg.TraceTypes = []TraceType{TraceTypeCannon, TraceTypeAsterisc, TraceTypeAlphabet, TraceTypeFast}
	// Set all required options and check its valid
	cfg.RollupRpc = validRollupRpc
	require.NoError(t, cfg.Check())

	// Require cannon specific args
	cfg.CannonConfig.VmBin = ""
	require.ErrorIs(t, cfg.Check(), ErrMissingCannonBin)
	cfg.CannonConfig.VmBin = validCannonBin

	// Require asterisc specific args
	cfg.AsteriscAbsolutePreState = ""
	cfg.AsteriscAbsolutePreStateBaseURL = nil
	require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscAbsolutePreState)
	cfg.AsteriscAbsolutePreState = validAsteriscAbsolutPreState

	// Require cannon specific args
	cfg.AsteriscConfig.Server = ""
	require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscServer)
	cfg.AsteriscConfig.Server = validAsteriscOpProgramBin

	// Check final config is valid
	require.NoError(t, cfg.Check())
}

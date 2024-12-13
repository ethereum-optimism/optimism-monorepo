package config

import (
	"fmt"
	"net/url"
	"runtime"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

var (
	validL1EthRpc                         = "http://localhost:8545"
	validL1BeaconUrl                      = "http://localhost:9000"
	validGameFactoryAddress               = common.Address{0x23}
	validCannonBin                        = "./bin/cannon"
	validCannonOpProgramBin               = "./bin/op-program"
	validCannonNetwork                    = "mainnet"
	validCannonAbsolutePreState           = "pre.json"
	validCannonAbsolutePreStateBaseURL, _ = url.Parse("http://localhost/foo/")
	validDatadir                          = "/tmp/data"
	validL2Rpc                            = "http://localhost:9545"
	validRollupRpc                        = "http://localhost:8555"

	validAsteriscBin                        = "./bin/asterisc"
	validAsteriscOpProgramBin               = "./bin/op-program"
	validAsteriscNetwork                    = "mainnet"
	validAsteriscAbsolutePreState           = "pre.json"
	validAsteriscAbsolutePreStateBaseURL, _ = url.Parse("http://localhost/bar/")

	validAsteriscKonaBin                        = "./bin/asterisc"
	validAsteriscKonaServerBin                  = "./bin/kona-host"
	validAsteriscKonaNetwork                    = "mainnet"
	validAsteriscKonaAbsolutePreState           = "pre.json"
	validAsteriscKonaAbsolutePreStateBaseURL, _ = url.Parse("http://localhost/bar/")
)

var cannonTraceTypes = []types.TraceType{types.TraceTypeCannon, types.TraceTypePermissioned}
var asteriscTraceTypes = []types.TraceType{types.TraceTypeAsterisc}
var asteriscKonaTraceTypes = []types.TraceType{types.TraceTypeAsteriscKona}

func applyValidConfigForCannon(cfg *Config) {
	cfg.Cannon.VmBin = validCannonBin
	cfg.Cannon.Server = validCannonOpProgramBin
	cfg.CannonAbsolutePreStateBaseURL = validCannonAbsolutePreStateBaseURL
	cfg.Cannon.Network = validCannonNetwork
}

func applyValidConfigForAsterisc(cfg *Config) {
	cfg.Asterisc.VmBin = validAsteriscBin
	cfg.Asterisc.Server = validAsteriscOpProgramBin
	cfg.AsteriscAbsolutePreStateBaseURL = validAsteriscAbsolutePreStateBaseURL
	cfg.Asterisc.Network = validAsteriscNetwork
}

func applyValidConfigForAsteriscKona(cfg *Config) {
	cfg.AsteriscKona.VmBin = validAsteriscKonaBin
	cfg.AsteriscKona.Server = validAsteriscKonaServerBin
	cfg.AsteriscKonaAbsolutePreStateBaseURL = validAsteriscKonaAbsolutePreStateBaseURL
	cfg.AsteriscKona.Network = validAsteriscKonaNetwork
}

func validConfig(traceType types.TraceType) Config {
	cfg := NewConfig(validGameFactoryAddress, validL1EthRpc, validL1BeaconUrl, validRollupRpc, validL2Rpc, validDatadir, traceType)
	if traceType == types.TraceTypeCannon || traceType == types.TraceTypePermissioned {
		applyValidConfigForCannon(&cfg)
	}
	if traceType == types.TraceTypeAsterisc {
		applyValidConfigForAsterisc(&cfg)
	}
	if traceType == types.TraceTypeAsteriscKona {
		applyValidConfigForAsteriscKona(&cfg)
	}
	return cfg
}

// TestValidConfigIsValid checks that the config provided by validConfig is actually valid
func TestValidConfigIsValid(t *testing.T) {
	for _, traceType := range types.TraceTypes {
		traceType := traceType
		t.Run(traceType.String(), func(t *testing.T) {
			err := validConfig(traceType).Check()
			require.NoError(t, err)
		})
	}
}

func TestTxMgrConfig(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		config := validConfig(types.TraceTypeCannon)
		config.TxMgrConfig = txmgr.CLIConfig{}
		require.Equal(t, config.Check().Error(), "must provide a L1 RPC url")
	})
}

func TestL1EthRpcRequired(t *testing.T) {
	config := validConfig(types.TraceTypeCannon)
	config.L1EthRpc = ""
	require.ErrorIs(t, config.Check(), ErrMissingL1EthRPC)
}

func TestL1BeaconRequired(t *testing.T) {
	config := validConfig(types.TraceTypeCannon)
	config.L1Beacon = ""
	require.ErrorIs(t, config.Check(), ErrMissingL1Beacon)
}

func TestGameFactoryAddressRequired(t *testing.T) {
	config := validConfig(types.TraceTypeCannon)
	config.GameFactoryAddress = common.Address{}
	require.ErrorIs(t, config.Check(), ErrMissingGameFactoryAddress)
}

func TestSelectiveClaimResolutionNotRequired(t *testing.T) {
	config := validConfig(types.TraceTypeCannon)
	require.Equal(t, false, config.SelectiveClaimResolution)
	require.NoError(t, config.Check())
}

func TestGameAllowlistNotRequired(t *testing.T) {
	config := validConfig(types.TraceTypeCannon)
	config.GameAllowlist = []common.Address{}
	require.NoError(t, config.Check())
}

func TestCannonRequiredArgs(t *testing.T) {
	for _, traceType := range cannonTraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("TestCannonBinRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.Cannon.VmBin = ""
			require.ErrorIs(t, config.Check(), ErrMissingCannonBin)
		})

		t.Run(fmt.Sprintf("TestCannonServerRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.Cannon.Server = ""
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
			config.CannonAbsolutePreState = validCannonAbsolutePreState
			config.CannonAbsolutePreStateBaseURL = nil
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestCannonAbsolutePreStateBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.CannonAbsolutePreState = ""
			config.CannonAbsolutePreStateBaseURL = validCannonAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAllowSupplyingBothCannonAbsolutePreStateAndBaseURL-%v", traceType), func(t *testing.T) {
			// Since the prestate baseURL might be inherited from the --prestate-urls option, allow overriding it with a specific prestate
			config := validConfig(traceType)
			config.CannonAbsolutePreState = validCannonAbsolutePreState
			config.CannonAbsolutePreStateBaseURL = validCannonAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestL2RpcRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.L2Rpc = ""
			require.ErrorIs(t, config.Check(), ErrMissingL2Rpc)
		})

		t.Run(fmt.Sprintf("TestCannonSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.Cannon.SnapshotFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingCannonSnapshotFreq)
			})
		})

		t.Run(fmt.Sprintf("TestCannonInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.Cannon.InfoFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingCannonInfoFreq)
			})
		})

		t.Run(fmt.Sprintf("TestCannonNetworkOrRollupConfigRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Cannon.Network = ""
			cfg.Cannon.RollupConfigPath = ""
			cfg.Cannon.L2GenesisPath = "genesis.json"
			require.ErrorIs(t, cfg.Check(), ErrMissingCannonRollupConfig)
		})

		t.Run(fmt.Sprintf("TestCannonNetworkOrL2GenesisRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Cannon.Network = ""
			cfg.Cannon.RollupConfigPath = "foo.json"
			cfg.Cannon.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrMissingCannonL2Genesis)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Cannon.Network = validCannonNetwork
			cfg.Cannon.RollupConfigPath = "foo.json"
			cfg.Cannon.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrCannonNetworkAndRollupConfig)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndL2Genesis-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Cannon.Network = validCannonNetwork
			cfg.Cannon.RollupConfigPath = ""
			cfg.Cannon.L2GenesisPath = "foo.json"
			require.ErrorIs(t, cfg.Check(), ErrCannonNetworkAndL2Genesis)
		})

		t.Run(fmt.Sprintf("TestNetworkMustBeValid-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Cannon.Network = "unknown"
			require.ErrorIs(t, cfg.Check(), ErrCannonNetworkUnknown)
		})

		t.Run(fmt.Sprintf("TestNetworkMayBeAnyChainID-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Cannon.Network = "467294"
			require.NoError(t, cfg.Check())
		})

		t.Run(fmt.Sprintf("TestNetworkInvalidWhenNotEntirelyNumeric-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Cannon.Network = "467294a"
			require.ErrorIs(t, cfg.Check(), ErrCannonNetworkUnknown)
		})

		t.Run(fmt.Sprintf("TestDebugInfoEnabled-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			require.True(t, cfg.Cannon.DebugInfo)
		})
	}
}

func TestAsteriscRequiredArgs(t *testing.T) {
	for _, traceType := range asteriscTraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("TestAsteriscBinRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.Asterisc.VmBin = ""
			require.ErrorIs(t, config.Check(), ErrMissingAsteriscBin)
		})

		t.Run(fmt.Sprintf("TestAsteriscServerRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.Asterisc.Server = ""
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
			config.AsteriscAbsolutePreState = validAsteriscAbsolutePreState
			config.AsteriscAbsolutePreStateBaseURL = nil
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAsteriscAbsolutePreStateBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscAbsolutePreState = ""
			config.AsteriscAbsolutePreStateBaseURL = validAsteriscAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAllowSupplingBothAsteriscAbsolutePreStateAndBaseURL-%v", traceType), func(t *testing.T) {
			// Since the prestate base URL might be inherited from the --prestate-urls option, allow overriding it with a specific prestate
			config := validConfig(traceType)
			config.AsteriscAbsolutePreState = validAsteriscAbsolutePreState
			config.AsteriscAbsolutePreStateBaseURL = validAsteriscAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestL2RpcRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.L2Rpc = ""
			require.ErrorIs(t, config.Check(), ErrMissingL2Rpc)
		})

		t.Run(fmt.Sprintf("TestAsteriscSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.Asterisc.SnapshotFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscSnapshotFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.Asterisc.InfoFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscInfoFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscNetworkOrRollupConfigRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Asterisc.Network = ""
			cfg.Asterisc.RollupConfigPath = ""
			cfg.Asterisc.L2GenesisPath = "genesis.json"
			require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscRollupConfig)
		})

		t.Run(fmt.Sprintf("TestAsteriscNetworkOrL2GenesisRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Asterisc.Network = ""
			cfg.Asterisc.RollupConfigPath = "foo.json"
			cfg.Asterisc.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscL2Genesis)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Asterisc.Network = validAsteriscNetwork
			cfg.Asterisc.RollupConfigPath = "foo.json"
			cfg.Asterisc.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrAsteriscNetworkAndRollupConfig)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndL2Genesis-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Asterisc.Network = validAsteriscNetwork
			cfg.Asterisc.RollupConfigPath = ""
			cfg.Asterisc.L2GenesisPath = "foo.json"
			require.ErrorIs(t, cfg.Check(), ErrAsteriscNetworkAndL2Genesis)
		})

		t.Run(fmt.Sprintf("TestNetworkMustBeValid-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.Asterisc.Network = "unknown"
			require.ErrorIs(t, cfg.Check(), ErrAsteriscNetworkUnknown)
		})

		t.Run(fmt.Sprintf("TestDebugInfoDisabled-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			require.False(t, cfg.Asterisc.DebugInfo)
		})
	}
}

func TestAsteriscKonaRequiredArgs(t *testing.T) {
	for _, traceType := range asteriscKonaTraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("TestAsteriscKonaBinRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscKona.VmBin = ""
			require.ErrorIs(t, config.Check(), ErrMissingAsteriscKonaBin)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaServerRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscKona.Server = ""
			require.ErrorIs(t, config.Check(), ErrMissingAsteriscKonaServer)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaAbsolutePreStateOrBaseURLRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscKonaAbsolutePreState = ""
			config.AsteriscKonaAbsolutePreStateBaseURL = nil
			require.ErrorIs(t, config.Check(), ErrMissingAsteriscKonaAbsolutePreState)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaAbsolutePreState-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscKonaAbsolutePreState = validAsteriscKonaAbsolutePreState
			config.AsteriscKonaAbsolutePreStateBaseURL = nil
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaAbsolutePreStateBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.AsteriscKonaAbsolutePreState = ""
			config.AsteriscKonaAbsolutePreStateBaseURL = validAsteriscKonaAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAllowSupplyingBothAsteriscKonaAbsolutePreStateAndBaseURL-%v", traceType), func(t *testing.T) {
			// Since the prestate base URL might be inherited from the --prestate-urls option, allow overriding it with a specific prestate
			config := validConfig(traceType)
			config.AsteriscKonaAbsolutePreState = validAsteriscKonaAbsolutePreState
			config.AsteriscKonaAbsolutePreStateBaseURL = validAsteriscKonaAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestL2RpcRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(traceType)
			config.L2Rpc = ""
			require.ErrorIs(t, config.Check(), ErrMissingL2Rpc)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.AsteriscKona.SnapshotFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscKonaSnapshotFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(traceType)
				cfg.AsteriscKona.InfoFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscKonaInfoFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaNetworkOrRollupConfigRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscKona.Network = ""
			cfg.AsteriscKona.RollupConfigPath = ""
			cfg.AsteriscKona.L2GenesisPath = "genesis.json"
			require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscKonaRollupConfig)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaNetworkOrL2GenesisRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscKona.Network = ""
			cfg.AsteriscKona.RollupConfigPath = "foo.json"
			cfg.AsteriscKona.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscKonaL2Genesis)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscKona.Network = validAsteriscKonaNetwork
			cfg.AsteriscKona.RollupConfigPath = "foo.json"
			cfg.AsteriscKona.L2GenesisPath = ""
			require.ErrorIs(t, cfg.Check(), ErrAsteriscKonaNetworkAndRollupConfig)
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndL2Genesis-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscKona.Network = validAsteriscKonaNetwork
			cfg.AsteriscKona.RollupConfigPath = ""
			cfg.AsteriscKona.L2GenesisPath = "foo.json"
			require.ErrorIs(t, cfg.Check(), ErrAsteriscKonaNetworkAndL2Genesis)
		})

		t.Run(fmt.Sprintf("TestNetworkMustBeValid-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			cfg.AsteriscKona.Network = "unknown"
			require.ErrorIs(t, cfg.Check(), ErrAsteriscKonaNetworkUnknown)
		})

		t.Run(fmt.Sprintf("TestDebugInfoDisabled-%v", traceType), func(t *testing.T) {
			cfg := validConfig(traceType)
			require.False(t, cfg.AsteriscKona.DebugInfo)
		})
	}
}

func TestDatadirRequired(t *testing.T) {
	config := validConfig(types.TraceTypeAlphabet)
	config.Datadir = ""
	require.ErrorIs(t, config.Check(), ErrMissingDatadir)
}

func TestMaxConcurrency(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		config := validConfig(types.TraceTypeAlphabet)
		config.MaxConcurrency = 0
		require.ErrorIs(t, config.Check(), ErrMaxConcurrencyZero)
	})

	t.Run("DefaultToNumberOfCPUs", func(t *testing.T) {
		config := validConfig(types.TraceTypeAlphabet)
		require.EqualValues(t, runtime.NumCPU(), config.MaxConcurrency)
	})
}

func TestHttpPollInterval(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		config := validConfig(types.TraceTypeAlphabet)
		require.EqualValues(t, DefaultPollInterval, config.PollInterval)
	})
}

func TestRollupRpcRequired(t *testing.T) {
	for _, traceType := range types.TraceTypes {
		traceType := traceType
		t.Run(traceType.String(), func(t *testing.T) {
			config := validConfig(traceType)
			config.RollupRpc = ""
			require.ErrorIs(t, config.Check(), ErrMissingRollupRpc)
		})
	}
}

func TestRequireConfigForMultipleTraceTypesForCannon(t *testing.T) {
	cfg := validConfig(types.TraceTypeCannon)
	cfg.TraceTypes = []types.TraceType{types.TraceTypeCannon, types.TraceTypeAlphabet}
	// Set all required options and check its valid
	cfg.RollupRpc = validRollupRpc
	require.NoError(t, cfg.Check())

	// Require cannon specific args
	cfg.CannonAbsolutePreState = ""
	cfg.CannonAbsolutePreStateBaseURL = nil
	require.ErrorIs(t, cfg.Check(), ErrMissingCannonAbsolutePreState)
	cfg.CannonAbsolutePreState = validCannonAbsolutePreState

	// Require output cannon specific args
	cfg.RollupRpc = ""
	require.ErrorIs(t, cfg.Check(), ErrMissingRollupRpc)
}

func TestRequireConfigForMultipleTraceTypesForAsterisc(t *testing.T) {
	cfg := validConfig(types.TraceTypeAsterisc)
	cfg.TraceTypes = []types.TraceType{types.TraceTypeAsterisc, types.TraceTypeAlphabet}
	// Set all required options and check its valid
	cfg.RollupRpc = validRollupRpc
	require.NoError(t, cfg.Check())

	// Require asterisc specific args
	cfg.AsteriscAbsolutePreState = ""
	cfg.AsteriscAbsolutePreStateBaseURL = nil
	require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscAbsolutePreState)
	cfg.AsteriscAbsolutePreState = validAsteriscAbsolutePreState

	// Require output asterisc specific args
	cfg.RollupRpc = ""
	require.ErrorIs(t, cfg.Check(), ErrMissingRollupRpc)
}

func TestRequireConfigForMultipleTraceTypesForCannonAndAsterisc(t *testing.T) {
	cfg := validConfig(types.TraceTypeCannon)
	applyValidConfigForAsterisc(&cfg)

	cfg.TraceTypes = []types.TraceType{types.TraceTypeCannon, types.TraceTypeAsterisc, types.TraceTypeAlphabet, types.TraceTypeFast}
	// Set all required options and check its valid
	cfg.RollupRpc = validRollupRpc
	require.NoError(t, cfg.Check())

	// Require cannon specific args
	cfg.Cannon.VmBin = ""
	require.ErrorIs(t, cfg.Check(), ErrMissingCannonBin)
	cfg.Cannon.VmBin = validCannonBin

	// Require asterisc specific args
	cfg.AsteriscAbsolutePreState = ""
	cfg.AsteriscAbsolutePreStateBaseURL = nil
	require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscAbsolutePreState)
	cfg.AsteriscAbsolutePreState = validAsteriscAbsolutePreState

	// Require cannon specific args
	cfg.Asterisc.Server = ""
	require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscServer)
	cfg.Asterisc.Server = validAsteriscOpProgramBin

	// Check final config is valid
	require.NoError(t, cfg.Check())
}

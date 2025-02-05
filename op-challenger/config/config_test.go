package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
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
	validSupervisorRpc                    = "http://localhost/supervisor"

	validAsteriscBin                        = "./bin/asterisc"
	validAsteriscOpProgramBin               = "./bin/op-program"
	validAsteriscNetwork                    = "mainnet"
	validAsteriscAbsolutePreState           = "pre.json"
	validAsteriscAbsolutePreStateBaseURL, _ = url.Parse("http://localhost/bar/")

	nonExistingFile                             = "path/to/nonexistent/file"
	validAsteriscKonaBin                        = "./bin/asterisc"
	validAsteriscKonaServerBin                  = "./bin/kona-host"
	validAsteriscKonaNetwork                    = "mainnet"
	validAsteriscKonaAbsolutePreState           = "pre.json"
	validAsteriscKonaAbsolutePreStateBaseURL, _ = url.Parse("http://localhost/bar/")
)

var cannonTraceTypes = []types.TraceType{types.TraceTypeCannon, types.TraceTypePermissioned, types.TraceTypeSuperCannon}
var asteriscTraceTypes = []types.TraceType{types.TraceTypeAsterisc}
var asteriscKonaTraceTypes = []types.TraceType{types.TraceTypeAsteriscKona}

func ensureExists(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	err = os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	return file.Close()
}

func applyValidConfigForSuperCannon(t *testing.T, cfg *Config) {
	cfg.SupervisorRPC = validSupervisorRpc
	applyValidConfigForCannon(t, cfg)
}

func applyValidConfigForCannon(t *testing.T, cfg *Config) {
	tmpDir := t.TempDir()
	vmBin := filepath.Join(tmpDir, validCannonBin)
	server := filepath.Join(tmpDir, validCannonOpProgramBin)
	err := ensureExists(vmBin)
	require.NoError(t, err)
	err = ensureExists(server)
	require.NoError(t, err)
	cfg.Cannon.VmBin = vmBin
	cfg.Cannon.Server = server
	cfg.CannonAbsolutePreStateBaseURL = validCannonAbsolutePreStateBaseURL
	cfg.Cannon.Networks = []string{validCannonNetwork}
}

func applyValidConfigForAsterisc(t *testing.T, cfg *Config) {
	tmpDir := t.TempDir()
	vmBin := filepath.Join(tmpDir, validAsteriscBin)
	server := filepath.Join(tmpDir, validAsteriscOpProgramBin)
	err := ensureExists(vmBin)
	require.NoError(t, err)
	err = ensureExists(server)
	require.NoError(t, err)
	cfg.Asterisc.VmBin = vmBin
	cfg.Asterisc.Server = server
	cfg.AsteriscAbsolutePreStateBaseURL = validAsteriscAbsolutePreStateBaseURL
	cfg.Asterisc.Networks = []string{validAsteriscNetwork}
}

func applyValidConfigForAsteriscKona(t *testing.T, cfg *Config) {
	tmpDir := t.TempDir()
	vmBin := filepath.Join(tmpDir, validAsteriscKonaBin)
	server := filepath.Join(tmpDir, validAsteriscKonaServerBin)
	err := ensureExists(vmBin)
	require.NoError(t, err)
	err = ensureExists(server)
	require.NoError(t, err)
	cfg.AsteriscKona.VmBin = vmBin
	cfg.AsteriscKona.Server = server
	cfg.AsteriscKonaAbsolutePreStateBaseURL = validAsteriscKonaAbsolutePreStateBaseURL
	cfg.AsteriscKona.Networks = []string{validAsteriscKonaNetwork}
}

func validConfig(t *testing.T, traceType types.TraceType) Config {
	cfg := NewConfig(validGameFactoryAddress, validL1EthRpc, validL1BeaconUrl, validRollupRpc, validL2Rpc, validDatadir, traceType)
	if traceType == types.TraceTypeSuperCannon {
		applyValidConfigForSuperCannon(t, &cfg)
	}
	if traceType == types.TraceTypeCannon || traceType == types.TraceTypePermissioned {
		applyValidConfigForCannon(t, &cfg)
	}
	if traceType == types.TraceTypeAsterisc {
		applyValidConfigForAsterisc(t, &cfg)
	}
	if traceType == types.TraceTypeAsteriscKona {
		applyValidConfigForAsteriscKona(t, &cfg)
	}
	return cfg
}

// TestValidConfigIsValid checks that the config provided by validConfig is actually valid
func TestValidConfigIsValid(t *testing.T) {
	for _, traceType := range types.TraceTypes {
		traceType := traceType
		t.Run(traceType.String(), func(t *testing.T) {
			err := validConfig(t, traceType).Check()
			require.NoError(t, err)
		})
	}
}

func TestTxMgrConfig(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		config := validConfig(t, types.TraceTypeCannon)
		config.TxMgrConfig = txmgr.CLIConfig{}
		require.Equal(t, config.Check().Error(), "must provide a L1 RPC url")
	})
}

func TestL1EthRpcRequired(t *testing.T) {
	config := validConfig(t, types.TraceTypeCannon)
	config.L1EthRpc = ""
	require.ErrorIs(t, config.Check(), ErrMissingL1EthRPC)
}

func TestL1BeaconRequired(t *testing.T) {
	config := validConfig(t, types.TraceTypeCannon)
	config.L1Beacon = ""
	require.ErrorIs(t, config.Check(), ErrMissingL1Beacon)
}

func TestGameFactoryAddressRequired(t *testing.T) {
	config := validConfig(t, types.TraceTypeCannon)
	config.GameFactoryAddress = common.Address{}
	require.ErrorIs(t, config.Check(), ErrMissingGameFactoryAddress)
}

func TestSelectiveClaimResolutionNotRequired(t *testing.T) {
	config := validConfig(t, types.TraceTypeCannon)
	require.Equal(t, false, config.SelectiveClaimResolution)
	require.NoError(t, config.Check())
}

func TestGameAllowlistNotRequired(t *testing.T) {
	config := validConfig(t, types.TraceTypeCannon)
	config.GameAllowlist = []common.Address{}
	require.NoError(t, config.Check())
}

func TestCannonRequiredArgs(t *testing.T) {
	for _, traceType := range cannonTraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("TestCannonBinRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.Cannon.VmBin = ""
			require.ErrorIs(t, config.Check(), vm.ErrMissingBin)
		})

		t.Run(fmt.Sprintf("TestCannonServerRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.Cannon.Server = ""
			require.ErrorIs(t, config.Check(), vm.ErrMissingServer)
		})

		t.Run(fmt.Sprintf("TestCannonAbsolutePreStateOrBaseURLRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.CannonAbsolutePreState = ""
			config.CannonAbsolutePreStateBaseURL = nil
			require.ErrorIs(t, config.Check(), ErrMissingCannonAbsolutePreState)
		})

		t.Run(fmt.Sprintf("TestCannonAbsolutePreState-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.CannonAbsolutePreState = validCannonAbsolutePreState
			config.CannonAbsolutePreStateBaseURL = nil
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestCannonAbsolutePreStateBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.CannonAbsolutePreState = ""
			config.CannonAbsolutePreStateBaseURL = validCannonAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAllowSupplyingBothCannonAbsolutePreStateAndBaseURL-%v", traceType), func(t *testing.T) {
			// Since the prestate baseURL might be inherited from the --prestate-urls option, allow overriding it with a specific prestate
			config := validConfig(t, traceType)
			config.CannonAbsolutePreState = validCannonAbsolutePreState
			config.CannonAbsolutePreStateBaseURL = validCannonAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestL2RpcRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.L2Rpcs = nil
			require.ErrorIs(t, config.Check(), ErrMissingL2Rpc)
		})

		t.Run(fmt.Sprintf("TestCannonSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(t, traceType)
				cfg.Cannon.SnapshotFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingCannonSnapshotFreq)
			})
		})

		t.Run(fmt.Sprintf("TestCannonInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(t, traceType)
				cfg.Cannon.InfoFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingCannonInfoFreq)
			})
		})

		t.Run(fmt.Sprintf("TestCannonNetworkOrRollupConfigRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Cannon.Networks = nil
			cfg.Cannon.RollupConfigPaths = nil
			cfg.Cannon.L2GenesisPaths = []string{"genesis.json"}
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingRollupConfig)
		})

		t.Run(fmt.Sprintf("TestCannonNetworkOrL2GenesisRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Cannon.Networks = nil
			cfg.Cannon.RollupConfigPaths = []string{"foo.json"}
			cfg.Cannon.L2GenesisPaths = nil
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingL2Genesis)
		})

		t.Run(fmt.Sprintf("TestMaySpecifyNetworkAndCustomConfigs-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Cannon.Networks = []string{validCannonNetwork}
			cfg.Cannon.RollupConfigPaths = []string{"foo.json"}
			cfg.Cannon.L2GenesisPaths = []string{"genesis.json"}
			require.NoError(t, cfg.Check())
		})

		t.Run(fmt.Sprintf("TestNetworkMustBeValid-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Cannon.Networks = []string{"unknown"}
			require.ErrorIs(t, cfg.Check(), vm.ErrNetworkUnknown)
		})

		t.Run(fmt.Sprintf("TestNetworkMayBeAnyChainID-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Cannon.Networks = []string{"467294"}
			require.NoError(t, cfg.Check())
		})

		t.Run(fmt.Sprintf("TestNetworkInvalidWhenNotEntirelyNumeric-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Cannon.Networks = []string{"467294a"}
			require.ErrorIs(t, cfg.Check(), vm.ErrNetworkUnknown)
		})

		t.Run(fmt.Sprintf("TestDebugInfoEnabled-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			require.True(t, cfg.Cannon.DebugInfo)
		})

		t.Run(fmt.Sprintf("TestVMBinExists-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Cannon.VmBin = nonExistingFile
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingBin)
		})

		t.Run(fmt.Sprintf("TestServerExists-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Cannon.Server = nonExistingFile
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingServer)
		})
	}
}

func TestAsteriscRequiredArgs(t *testing.T) {
	for _, traceType := range asteriscTraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("TestAsteriscBinRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.Asterisc.VmBin = ""
			require.ErrorIs(t, config.Check(), vm.ErrMissingBin)
		})

		t.Run(fmt.Sprintf("TestAsteriscServerRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.Asterisc.Server = ""
			require.ErrorIs(t, config.Check(), vm.ErrMissingServer)
		})

		t.Run(fmt.Sprintf("TestAsteriscAbsolutePreStateOrBaseURLRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.AsteriscAbsolutePreState = ""
			config.AsteriscAbsolutePreStateBaseURL = nil
			require.ErrorIs(t, config.Check(), ErrMissingAsteriscAbsolutePreState)
		})

		t.Run(fmt.Sprintf("TestAsteriscAbsolutePreState-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.AsteriscAbsolutePreState = validAsteriscAbsolutePreState
			config.AsteriscAbsolutePreStateBaseURL = nil
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAsteriscAbsolutePreStateBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.AsteriscAbsolutePreState = ""
			config.AsteriscAbsolutePreStateBaseURL = validAsteriscAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAllowSupplingBothAsteriscAbsolutePreStateAndBaseURL-%v", traceType), func(t *testing.T) {
			// Since the prestate base URL might be inherited from the --prestate-urls option, allow overriding it with a specific prestate
			config := validConfig(t, traceType)
			config.AsteriscAbsolutePreState = validAsteriscAbsolutePreState
			config.AsteriscAbsolutePreStateBaseURL = validAsteriscAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestL2RpcRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.L2Rpcs = nil
			require.ErrorIs(t, config.Check(), ErrMissingL2Rpc)
		})

		t.Run(fmt.Sprintf("TestAsteriscSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(t, traceType)
				cfg.Asterisc.SnapshotFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscSnapshotFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(t, traceType)
				cfg.Asterisc.InfoFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscInfoFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscNetworkOrRollupConfigRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Asterisc.Networks = nil
			cfg.Asterisc.RollupConfigPaths = nil
			cfg.Asterisc.L2GenesisPaths = []string{"genesis.json"}
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingRollupConfig)
		})

		t.Run(fmt.Sprintf("TestAsteriscNetworkOrL2GenesisRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Asterisc.Networks = nil
			cfg.Asterisc.RollupConfigPaths = []string{"foo.json"}
			cfg.Asterisc.L2GenesisPaths = nil
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingL2Genesis)
		})

		t.Run(fmt.Sprintf("MaySpecifyNetworkAndCustomConfigs-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Asterisc.Networks = []string{validAsteriscNetwork}
			cfg.Asterisc.RollupConfigPaths = []string{"foo.json"}
			cfg.Asterisc.L2GenesisPaths = []string{"genesis.json"}
			require.NoError(t, cfg.Check())
		})

		t.Run(fmt.Sprintf("TestNetworkMustBeValid-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Asterisc.Networks = []string{"unknown"}
			require.ErrorIs(t, cfg.Check(), vm.ErrNetworkUnknown)
		})

		t.Run(fmt.Sprintf("TestDebugInfoDisabled-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			require.False(t, cfg.Asterisc.DebugInfo)
		})

		t.Run(fmt.Sprintf("TestVMBinExists-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Asterisc.VmBin = nonExistingFile
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingBin)
		})

		t.Run(fmt.Sprintf("TestServerExists-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.Asterisc.Server = nonExistingFile
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingServer)
		})
	}
}

func TestAsteriscKonaRequiredArgs(t *testing.T) {
	for _, traceType := range asteriscKonaTraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("TestAsteriscKonaBinRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.AsteriscKona.VmBin = ""
			require.ErrorIs(t, config.Check(), vm.ErrMissingBin)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaServerRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.AsteriscKona.Server = ""
			require.ErrorIs(t, config.Check(), vm.ErrMissingServer)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaAbsolutePreStateOrBaseURLRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.AsteriscKonaAbsolutePreState = ""
			config.AsteriscKonaAbsolutePreStateBaseURL = nil
			require.ErrorIs(t, config.Check(), ErrMissingAsteriscKonaAbsolutePreState)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaAbsolutePreState-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.AsteriscKonaAbsolutePreState = validAsteriscKonaAbsolutePreState
			config.AsteriscKonaAbsolutePreStateBaseURL = nil
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaAbsolutePreStateBaseURL-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.AsteriscKonaAbsolutePreState = ""
			config.AsteriscKonaAbsolutePreStateBaseURL = validAsteriscKonaAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestAllowSupplyingBothAsteriscKonaAbsolutePreStateAndBaseURL-%v", traceType), func(t *testing.T) {
			// Since the prestate base URL might be inherited from the --prestate-urls option, allow overriding it with a specific prestate
			config := validConfig(t, traceType)
			config.AsteriscKonaAbsolutePreState = validAsteriscKonaAbsolutePreState
			config.AsteriscKonaAbsolutePreStateBaseURL = validAsteriscKonaAbsolutePreStateBaseURL
			require.NoError(t, config.Check())
		})

		t.Run(fmt.Sprintf("TestL2RpcRequired-%v", traceType), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.L2Rpcs = nil
			require.ErrorIs(t, config.Check(), ErrMissingL2Rpc)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(t, traceType)
				cfg.AsteriscKona.SnapshotFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscKonaSnapshotFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("MustNotBeZero", func(t *testing.T) {
				cfg := validConfig(t, traceType)
				cfg.AsteriscKona.InfoFreq = 0
				require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscKonaInfoFreq)
			})
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaNetworkOrRollupConfigRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.AsteriscKona.Networks = nil
			cfg.AsteriscKona.RollupConfigPaths = nil
			cfg.AsteriscKona.L2GenesisPaths = []string{"genesis.json"}
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingRollupConfig)
		})

		t.Run(fmt.Sprintf("TestAsteriscKonaNetworkOrL2GenesisRequired-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.AsteriscKona.Networks = nil
			cfg.AsteriscKona.RollupConfigPaths = []string{"foo.json"}
			cfg.AsteriscKona.L2GenesisPaths = nil
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingL2Genesis)
		})

		t.Run(fmt.Sprintf("MaySpecifyNetworkAndCustomConfig-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.AsteriscKona.Networks = []string{validAsteriscKonaNetwork}
			cfg.AsteriscKona.RollupConfigPaths = []string{"foo.json"}
			cfg.AsteriscKona.L2GenesisPaths = []string{"genesis.json"}
			require.NoError(t, cfg.Check())
		})

		t.Run(fmt.Sprintf("TestNetworkMustBeValid-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.AsteriscKona.Networks = []string{"unknown"}
			require.ErrorIs(t, cfg.Check(), vm.ErrNetworkUnknown)
		})

		t.Run(fmt.Sprintf("TestDebugInfoDisabled-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			require.False(t, cfg.AsteriscKona.DebugInfo)
		})

		t.Run(fmt.Sprintf("TestVMBinExists-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.AsteriscKona.VmBin = nonExistingFile
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingBin)
		})

		t.Run(fmt.Sprintf("TestServerExists-%v", traceType), func(t *testing.T) {
			cfg := validConfig(t, traceType)
			cfg.AsteriscKona.Server = nonExistingFile
			require.ErrorIs(t, cfg.Check(), vm.ErrMissingServer)
		})
	}
}

func TestDatadirRequired(t *testing.T) {
	config := validConfig(t, types.TraceTypeAlphabet)
	config.Datadir = ""
	require.ErrorIs(t, config.Check(), ErrMissingDatadir)
}

func TestMaxConcurrency(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		config := validConfig(t, types.TraceTypeAlphabet)
		config.MaxConcurrency = 0
		require.ErrorIs(t, config.Check(), ErrMaxConcurrencyZero)
	})

	t.Run("DefaultToNumberOfCPUs", func(t *testing.T) {
		config := validConfig(t, types.TraceTypeAlphabet)
		require.EqualValues(t, runtime.NumCPU(), config.MaxConcurrency)
	})
}

func TestHttpPollInterval(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		config := validConfig(t, types.TraceTypeAlphabet)
		require.EqualValues(t, DefaultPollInterval, config.PollInterval)
	})
}

func TestRollupRpcRequired(t *testing.T) {
	for _, traceType := range types.TraceTypes {
		traceType := traceType
		if traceType == types.TraceTypeSuperCannon {
			continue
		}
		t.Run(traceType.String(), func(t *testing.T) {
			config := validConfig(t, traceType)
			config.RollupRpc = ""
			require.ErrorIs(t, config.Check(), ErrMissingRollupRpc)
		})
	}
}

func TestRollupRpcNotRequiredForInterop(t *testing.T) {
	config := validConfig(t, types.TraceTypeSuperCannon)
	config.RollupRpc = ""
	require.NoError(t, config.Check())
}

func TestSupervisorRpc(t *testing.T) {
	for _, traceType := range types.TraceTypes {
		traceType := traceType
		if traceType == types.TraceTypeSuperCannon {
			t.Run("RequiredFor"+traceType.String(), func(t *testing.T) {
				config := validConfig(t, traceType)
				config.SupervisorRPC = ""
				require.ErrorIs(t, config.Check(), ErrMissingSupervisorRpc)
			})
		} else {
			t.Run("NotRequiredFor"+traceType.String(), func(t *testing.T) {
				config := validConfig(t, traceType)
				config.SupervisorRPC = ""
				require.NoError(t, config.Check())
			})
		}
	}
}

func TestRequireConfigForMultipleTraceTypesForCannon(t *testing.T) {
	cfg := validConfig(t, types.TraceTypeCannon)
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
	cfg := validConfig(t, types.TraceTypeAsterisc)
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
	cfg := validConfig(t, types.TraceTypeCannon)
	applyValidConfigForAsterisc(t, &cfg)

	cfg.TraceTypes = []types.TraceType{types.TraceTypeCannon, types.TraceTypeAsterisc, types.TraceTypeAlphabet, types.TraceTypeFast}
	// Set all required options and check its valid
	cfg.RollupRpc = validRollupRpc
	require.NoError(t, cfg.Check())

	// Require cannon specific args
	cfg.Cannon.VmBin = ""
	require.ErrorIs(t, cfg.Check(), vm.ErrMissingBin)
	tmpDir := t.TempDir()
	vmBin := filepath.Join(tmpDir, validCannonBin)
	err := ensureExists(vmBin)
	require.NoError(t, err)
	cfg.Cannon.VmBin = vmBin

	// Require asterisc specific args
	cfg.AsteriscAbsolutePreState = ""
	cfg.AsteriscAbsolutePreStateBaseURL = nil
	require.ErrorIs(t, cfg.Check(), ErrMissingAsteriscAbsolutePreState)
	cfg.AsteriscAbsolutePreState = validAsteriscAbsolutePreState

	cfg.Asterisc.Server = ""
	require.ErrorIs(t, cfg.Check(), vm.ErrMissingServer)
	server := filepath.Join(tmpDir, validAsteriscOpProgramBin)
	err = ensureExists(server)
	require.NoError(t, err)
	cfg.Asterisc.Server = server

	// Check final config is valid
	require.NoError(t, cfg.Check())
}

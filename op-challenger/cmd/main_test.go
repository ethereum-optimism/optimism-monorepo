package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-challenger/config"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

var (
	l1EthRpc                = "http://example.com:8545"
	l1Beacon                = "http://example.com:9000"
	gameFactoryAddressValue = "0xbb00000000000000000000000000000000000000"
	cannonNetwork           = "op-mainnet"
	otherCannonNetwork      = "op-sepolia"
	cannonBin               = "./bin/cannon"
	cannonServer            = "./bin/op-program"
	cannonPreState          = "./pre.json"
	datadir                 = "./test_data"
	cannonL2                = "http://example.com:9545"
	rollupRpc               = "http://example.com:8555"
)

func TestLogLevel(t *testing.T) {
	t.Run("RejectInvalid", func(t *testing.T) {
		verifyArgsInvalid(t, "unknown level: foo", addRequiredArgs(config.TraceTypeAlphabet, "--log.level=foo"))
	})

	for _, lvl := range []string{"trace", "debug", "info", "error", "crit"} {
		lvl := lvl
		t.Run("AcceptValid_"+lvl, func(t *testing.T) {
			logger, _, err := dryRunWithArgs(addRequiredArgs(config.TraceTypeAlphabet, "--log.level", lvl))
			require.NoError(t, err)
			require.NotNil(t, logger)
		})
	}
}

func TestDefaultCLIOptionsMatchDefaultConfig(t *testing.T) {
	cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet))
	defaultCfg := config.NewConfig(common.HexToAddress(gameFactoryAddressValue), l1EthRpc, l1Beacon, datadir, config.TraceTypeAlphabet)
	// Add in the extra CLI options required when using alphabet trace type
	defaultCfg.RollupRpc = rollupRpc
	require.Equal(t, defaultCfg, cfg)
}

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := config.NewConfig(common.HexToAddress(gameFactoryAddressValue), l1EthRpc, l1Beacon, datadir, config.TraceTypeAlphabet)
	// Add in options that are required based on the specific trace type
	// To avoid needing to specify unused options, these aren't included in the params for NewConfig
	cfg.RollupRpc = rollupRpc
	require.NoError(t, cfg.Check())
}

func TestL1ETHRPCAddress(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l1-eth-rpc is required", addRequiredArgsExcept(config.TraceTypeAlphabet, "--l1-eth-rpc"))
	})

	t.Run("Valid", func(t *testing.T) {
		url := "http://example.com:8888"
		cfg := configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--l1-eth-rpc", "--l1-eth-rpc="+url))
		require.Equal(t, url, cfg.L1EthRpc)
		require.Equal(t, url, cfg.TxMgrConfig.L1RPCURL)
	})
}

func TestL1Beacon(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l1-beacon is required", addRequiredArgsExcept(config.TraceTypeAlphabet, "--l1-beacon"))
	})

	t.Run("Valid", func(t *testing.T) {
		url := "http://example.com:8888"
		cfg := configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--l1-beacon", "--l1-beacon="+url))
		require.Equal(t, url, cfg.L1Beacon)
	})
}

func TestTraceType(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		expectedDefault := config.TraceTypeCannon
		cfg := configForArgs(t, addRequiredArgsExcept(expectedDefault, "--trace-type"))
		require.Equal(t, []config.TraceType{expectedDefault}, cfg.TraceTypes)
	})

	for _, traceType := range config.TraceTypes {
		traceType := traceType
		t.Run("Valid_"+traceType.String(), func(t *testing.T) {
			cfg := configForArgs(t, addRequiredArgs(traceType))
			require.Equal(t, []config.TraceType{traceType}, cfg.TraceTypes)
		})
	}

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, "unknown trace type: \"foo\"", addRequiredArgsExcept(config.TraceTypeAlphabet, "--trace-type", "--trace-type=foo"))
	})
}

func TestMultipleTraceTypes(t *testing.T) {
	t.Run("WithAllOptions", func(t *testing.T) {
		argsMap := requiredArgs(config.TraceTypeCannon)
		addRequiredOutputArgs(argsMap)
		args := toArgList(argsMap)
		// Add extra trace types (cannon is already specified)
		args = append(args,
			"--trace-type", config.TraceTypeAlphabet.String())
		args = append(args,
			"--trace-type", config.TraceTypePermissioned.String())
		cfg := configForArgs(t, args)
		require.Equal(t, []config.TraceType{config.TraceTypeCannon, config.TraceTypeAlphabet, config.TraceTypePermissioned}, cfg.TraceTypes)
	})
	t.Run("WithSomeOptions", func(t *testing.T) {
		argsMap := requiredArgs(config.TraceTypeCannon)
		addRequiredOutputArgs(argsMap)
		args := toArgList(argsMap)
		// Add extra trace types (cannon is already specified)
		args = append(args,
			"--trace-type", config.TraceTypeAlphabet.String())
		cfg := configForArgs(t, args)
		require.Equal(t, []config.TraceType{config.TraceTypeCannon, config.TraceTypeAlphabet}, cfg.TraceTypes)
	})

	t.Run("SpecifySameOptionMultipleTimes", func(t *testing.T) {
		argsMap := requiredArgs(config.TraceTypeCannon)
		args := toArgList(argsMap)
		// Add cannon trace type again
		args = append(args, "--trace-type", config.TraceTypeCannon.String())
		// We're fine with the same option being listed multiple times, just deduplicate them.
		cfg := configForArgs(t, args)
		require.Equal(t, []config.TraceType{config.TraceTypeCannon}, cfg.TraceTypes)
	})
}

func TestGameFactoryAddress(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag game-factory-address is required", addRequiredArgsExcept(config.TraceTypeAlphabet, "--game-factory-address"))
	})

	t.Run("Valid", func(t *testing.T) {
		addr := common.Address{0xbb, 0xcc, 0xdd}
		cfg := configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--game-factory-address", "--game-factory-address="+addr.Hex()))
		require.Equal(t, addr, cfg.GameFactoryAddress)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid address: foo", addRequiredArgsExcept(config.TraceTypeAlphabet, "--game-factory-address", "--game-factory-address=foo"))
	})
}

func TestGameAllowlist(t *testing.T) {
	t.Run("Optional", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--game-allowlist"))
		require.NoError(t, cfg.Check())
	})

	t.Run("Valid", func(t *testing.T) {
		addr := common.Address{0xbb, 0xcc, 0xdd}
		cfg := configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--game-allowlist", "--game-allowlist="+addr.Hex()))
		require.Contains(t, cfg.GameAllowlist, addr)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid address: foo", addRequiredArgsExcept(config.TraceTypeAlphabet, "--game-allowlist", "--game-allowlist=foo"))
	})
}

func TestTxManagerFlagsSupported(t *testing.T) {
	// Not a comprehensive list of flags, just enough to sanity check the txmgr.CLIFlags were defined
	cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet, "--"+txmgr.NumConfirmationsFlagName, "7"))
	require.Equal(t, uint64(7), cfg.TxMgrConfig.NumConfirmations)
}

func TestMaxConcurrency(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		expected := uint(345)
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet, "--max-concurrency", "345"))
		require.Equal(t, expected, cfg.MaxConcurrency)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"invalid value \"abc\" for flag -max-concurrency",
			addRequiredArgs(config.TraceTypeAlphabet, "--max-concurrency", "abc"))
	})

	t.Run("Zero", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"max-concurrency must not be 0",
			addRequiredArgs(config.TraceTypeAlphabet, "--max-concurrency", "0"))
	})
}

func TestMaxPendingTx(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		expected := uint64(345)
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet, "--max-pending-tx", "345"))
		require.Equal(t, expected, cfg.MaxPendingTx)
	})

	t.Run("Zero", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet, "--max-pending-tx", "0"))
		require.Equal(t, uint64(0), cfg.MaxPendingTx)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"invalid value \"abc\" for flag -max-pending-tx",
			addRequiredArgs(config.TraceTypeAlphabet, "--max-pending-tx", "abc"))
	})
}

func TestPollInterval(t *testing.T) {
	t.Run("UsesDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeCannon))
		require.Equal(t, config.DefaultPollInterval, cfg.PollInterval)
	})

	t.Run("Valid", func(t *testing.T) {
		expected := 100 * time.Second
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet, "--http-poll-interval", "100s"))
		require.Equal(t, expected, cfg.PollInterval)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"invalid value \"abc\" for flag -http-poll-interval",
			addRequiredArgs(config.TraceTypeAlphabet, "--http-poll-interval", "abc"))
	})
}

func TestCannonRequiredArgs(t *testing.T) {
	for _, traceType := range []config.TraceType{config.TraceTypeCannon, config.TraceTypePermissioned} {
		traceType := traceType
		t.Run(fmt.Sprintf("TestCannonBin-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--cannon-bin"))
			})

			t.Run("Required", func(t *testing.T) {
				verifyArgsInvalid(t, "flag cannon-bin is required", addRequiredArgsExcept(traceType, "--cannon-bin"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-bin", "--cannon-bin=./cannon"))
				require.Equal(t, "./cannon", cfg.CannonBin)
			})
		})

		t.Run(fmt.Sprintf("TestCannonServer-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--cannon-server"))
			})

			t.Run("Required", func(t *testing.T) {
				verifyArgsInvalid(t, "flag cannon-server is required", addRequiredArgsExcept(traceType, "--cannon-server"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-server", "--cannon-server=./op-program"))
				require.Equal(t, "./op-program", cfg.CannonServer)
			})
		})

		t.Run(fmt.Sprintf("TestCannonAbsolutePrestate-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--cannon-prestate"))
			})

			t.Run("Required", func(t *testing.T) {
				verifyArgsInvalid(t, "flag cannon-prestate is required", addRequiredArgsExcept(traceType, "--cannon-prestate"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-prestate", "--cannon-prestate=./pre.json"))
				require.Equal(t, "./pre.json", cfg.CannonAbsolutePreState)
			})
		})

		t.Run(fmt.Sprintf("TestCannonL2-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--cannon-l2"))
			})

			t.Run("RequiredForCannonTrace", func(t *testing.T) {
				verifyArgsInvalid(t, "flag cannon-l2 is required", addRequiredArgsExcept(traceType, "--cannon-l2"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType))
				require.Equal(t, cannonL2, cfg.CannonL2)
			})
		})

		t.Run(fmt.Sprintf("TestCannonSnapshotFreq-%v", traceType), func(t *testing.T) {
			t.Run("UsesDefault", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType))
				require.Equal(t, config.DefaultCannonSnapshotFreq, cfg.CannonSnapshotFreq)
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType, "--cannon-snapshot-freq=1234"))
				require.Equal(t, uint(1234), cfg.CannonSnapshotFreq)
			})

			t.Run("Invalid", func(t *testing.T) {
				verifyArgsInvalid(t, "invalid value \"abc\" for flag -cannon-snapshot-freq",
					addRequiredArgs(traceType, "--cannon-snapshot-freq=abc"))
			})
		})

		t.Run(fmt.Sprintf("TestCannonInfoFreq-%v", traceType), func(t *testing.T) {
			t.Run("UsesDefault", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType))
				require.Equal(t, config.DefaultCannonInfoFreq, cfg.CannonInfoFreq)
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgs(traceType, "--cannon-info-freq=1234"))
				require.Equal(t, uint(1234), cfg.CannonInfoFreq)
			})

			t.Run("Invalid", func(t *testing.T) {
				verifyArgsInvalid(t, "invalid value \"abc\" for flag -cannon-info-freq",
					addRequiredArgs(traceType, "--cannon-info-freq=abc"))
			})
		})

		t.Run(fmt.Sprintf("TestRequireEitherCannonNetworkOrRollupAndGenesis-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(
				t,
				"flag cannon-network or cannon-rollup-config and cannon-l2-genesis is required",
				addRequiredArgsExcept(traceType, "--cannon-network"))
			verifyArgsInvalid(
				t,
				"flag cannon-network or cannon-rollup-config and cannon-l2-genesis is required",
				addRequiredArgsExcept(traceType, "--cannon-network", "--cannon-rollup-config=rollup.json"))
			verifyArgsInvalid(
				t,
				"flag cannon-network or cannon-rollup-config and cannon-l2-genesis is required",
				addRequiredArgsExcept(traceType, "--cannon-network", "--cannon-l2-genesis=gensis.json"))
		})

		t.Run(fmt.Sprintf("TestMustNotSpecifyNetworkAndRollup-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(
				t,
				"flag cannon-network can not be used with cannon-rollup-config and cannon-l2-genesis",
				addRequiredArgsExcept(traceType, "--cannon-network",
					"--cannon-network", cannonNetwork, "--cannon-rollup-config=rollup.json"))
		})

		t.Run(fmt.Sprintf("TestCannonNetwork-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--cannon-network"))
			})

			t.Run("NotRequiredWhenRollupAndGenesIsSpecified", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-network",
					"--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-network", "--cannon-network", otherCannonNetwork))
				require.Equal(t, otherCannonNetwork, cfg.CannonNetwork)
			})
		})

		t.Run(fmt.Sprintf("TestCannonRollupConfig-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--cannon-rollup-config"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-network", "--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json"))
				require.Equal(t, "rollup.json", cfg.CannonRollupConfigPath)
			})
		})

		t.Run(fmt.Sprintf("TestCannonL2Genesis-%v", traceType), func(t *testing.T) {
			t.Run("NotRequiredForAlphabetTrace", func(t *testing.T) {
				configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--cannon-l2-genesis"))
			})

			t.Run("Valid", func(t *testing.T) {
				cfg := configForArgs(t, addRequiredArgsExcept(traceType, "--cannon-network", "--cannon-rollup-config=rollup.json", "--cannon-l2-genesis=genesis.json"))
				require.Equal(t, "genesis.json", cfg.CannonL2GenesisPath)
			})
		})
	}
}

func TestDataDir(t *testing.T) {
	for _, traceType := range config.TraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("RequiredFor-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(t, "flag datadir is required", addRequiredArgsExcept(traceType, "--datadir"))
		})
	}

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept(config.TraceTypeCannon, "--datadir", "--datadir=/foo/bar/cannon"))
		require.Equal(t, "/foo/bar/cannon", cfg.Datadir)
	})
}

func TestRollupRpc(t *testing.T) {
	for _, traceType := range config.TraceTypes {
		traceType := traceType

		t.Run(fmt.Sprintf("RequiredFor-%v", traceType), func(t *testing.T) {
			verifyArgsInvalid(t, "flag rollup-rpc is required", addRequiredArgsExcept(traceType, "--rollup-rpc"))
		})
	}

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeCannon))
		require.Equal(t, rollupRpc, cfg.RollupRpc)
	})
}

func TestGameWindow(t *testing.T) {
	t.Run("UsesDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet))
		require.Equal(t, config.DefaultGameWindow, cfg.GameWindow)
	})

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet, "--game-window=1m"))
		require.Equal(t, time.Duration(time.Minute), cfg.GameWindow)
	})

	t.Run("ParsesDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet, "--game-window=360h"))
		require.Equal(t, config.DefaultGameWindow, cfg.GameWindow)
	})
}

func TestUnsafeAllowInvalidPrestate(t *testing.T) {
	t.Run("DefaultsToFalse", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--unsafe-allow-invalid-prestate"))
		require.False(t, cfg.AllowInvalidPrestate)
	})

	t.Run("EnabledWithNoValue", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeCannon, "--unsafe-allow-invalid-prestate"))
		require.True(t, cfg.AllowInvalidPrestate)
	})

	t.Run("EnabledWithTrue", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeCannon, "--unsafe-allow-invalid-prestate=true"))
		require.True(t, cfg.AllowInvalidPrestate)
	})

	t.Run("DisabledWithFalse", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeCannon, "--unsafe-allow-invalid-prestate=false"))
		require.False(t, cfg.AllowInvalidPrestate)
	})
}

func TestAdditionalBondClaimants(t *testing.T) {
	t.Run("DefaultsToEmpty", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgsExcept(config.TraceTypeAlphabet, "--additional-bond-claimants"))
		require.Empty(t, cfg.AdditionalBondClaimants)
	})

	t.Run("Valid-Single", func(t *testing.T) {
		claimant := common.Address{0xaa}
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet, "--additional-bond-claimants", claimant.Hex()))
		require.Contains(t, cfg.AdditionalBondClaimants, claimant)
		require.Len(t, cfg.AdditionalBondClaimants, 1)
	})

	t.Run("Valid-Multiple", func(t *testing.T) {
		claimant1 := common.Address{0xaa}
		claimant2 := common.Address{0xbb}
		claimant3 := common.Address{0xcc}
		cfg := configForArgs(t, addRequiredArgs(config.TraceTypeAlphabet,
			"--additional-bond-claimants", fmt.Sprintf("%v,%v,%v", claimant1.Hex(), claimant2.Hex(), claimant3.Hex())))
		require.Contains(t, cfg.AdditionalBondClaimants, claimant1)
		require.Contains(t, cfg.AdditionalBondClaimants, claimant2)
		require.Contains(t, cfg.AdditionalBondClaimants, claimant3)
		require.Len(t, cfg.AdditionalBondClaimants, 3)
	})

	t.Run("Invalid-Single", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid additional claimant",
			addRequiredArgs(config.TraceTypeAlphabet, "--additional-bond-claimants", "nope"))
	})

	t.Run("Invalid-Multiple", func(t *testing.T) {
		claimant1 := common.Address{0xaa}
		claimant2 := common.Address{0xbb}
		verifyArgsInvalid(t, "invalid additional claimant",
			addRequiredArgs(config.TraceTypeAlphabet, "--additional-bond-claimants", fmt.Sprintf("%v,nope,%v", claimant1.Hex(), claimant2.Hex())))
	})
}

func verifyArgsInvalid(t *testing.T, messageContains string, cliArgs []string) {
	_, _, err := dryRunWithArgs(cliArgs)
	require.ErrorContains(t, err, messageContains)
}

func configForArgs(t *testing.T, cliArgs []string) config.Config {
	_, cfg, err := dryRunWithArgs(cliArgs)
	require.NoError(t, err)
	return cfg
}

func dryRunWithArgs(cliArgs []string) (log.Logger, config.Config, error) {
	cfg := new(config.Config)
	var logger log.Logger
	fullArgs := append([]string{"op-challenger"}, cliArgs...)
	testErr := errors.New("dry-run")
	err := run(context.Background(), fullArgs, func(ctx context.Context, log log.Logger, config *config.Config) (cliapp.Lifecycle, error) {
		logger = log
		cfg = config
		return nil, testErr
	})
	if errors.Is(err, testErr) { // expected error
		err = nil
	}
	return logger, *cfg, err
}

func addRequiredArgs(traceType config.TraceType, args ...string) []string {
	req := requiredArgs(traceType)
	combined := toArgList(req)
	return append(combined, args...)
}

func addRequiredArgsExcept(traceType config.TraceType, name string, optionalArgs ...string) []string {
	req := requiredArgs(traceType)
	delete(req, name)
	return append(toArgList(req), optionalArgs...)
}

func requiredArgs(traceType config.TraceType) map[string]string {
	args := map[string]string{
		"--l1-eth-rpc":           l1EthRpc,
		"--l1-beacon":            l1Beacon,
		"--game-factory-address": gameFactoryAddressValue,
		"--trace-type":           traceType.String(),
		"--datadir":              datadir,
	}
	switch traceType {
	case config.TraceTypeCannon, config.TraceTypePermissioned:
		addRequiredCannonArgs(args)
	case config.TraceTypeAlphabet:
		addRequiredOutputArgs(args)
	}
	return args
}

func addRequiredCannonArgs(args map[string]string) {
	args["--cannon-network"] = cannonNetwork
	args["--cannon-bin"] = cannonBin
	args["--cannon-server"] = cannonServer
	args["--cannon-prestate"] = cannonPreState
	args["--cannon-l2"] = cannonL2
	addRequiredOutputArgs(args)
}

func addRequiredOutputArgs(args map[string]string) {
	args["--rollup-rpc"] = rollupRpc
}

func toArgList(req map[string]string) []string {
	var combined []string
	for name, value := range req {
		combined = append(combined, fmt.Sprintf("%s=%s", name, value))
	}
	return combined
}

package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-dispute-mon/config"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/superchain"
	"github.com/stretchr/testify/require"
)

var (
	l1EthRpc                = "http://example.com:8545"
	rollupRpc               = "http://example.com:8555"
	gameFactoryAddressValue = "0xbb00000000000000000000000000000000000000"
)

func TestLogLevel(t *testing.T) {
	t.Run("RejectInvalid", func(t *testing.T) {
		verifyArgsInvalid(t, "unknown level: foo", addRequiredArgs("--log.level=foo"))
	})

	for _, lvl := range []string{"trace", "debug", "info", "error", "crit"} {
		lvl := lvl
		t.Run("AcceptValid_"+lvl, func(t *testing.T) {
			logger, _, err := dryRunWithArgs(addRequiredArgs("--log.level", lvl))
			require.NoError(t, err)
			require.NotNil(t, logger)
		})
	}
}

func TestDefaultCLIOptionsMatchDefaultConfig(t *testing.T) {
	cfg := configForArgs(t, addRequiredArgs())
	defaultCfg := config.NewConfig(common.HexToAddress(gameFactoryAddressValue), l1EthRpc, rollupRpc)
	require.Equal(t, defaultCfg, cfg)
}

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := config.NewConfig(common.HexToAddress(gameFactoryAddressValue), l1EthRpc, rollupRpc)
	require.NoError(t, cfg.Check())
}

func TestL1EthRpc(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag l1-eth-rpc is required", addRequiredArgsExcept("--l1-eth-rpc"))
	})

	t.Run("Valid", func(t *testing.T) {
		url := "http://example.com:9999"
		cfg := configForArgs(t, addRequiredArgsExcept("--l1-eth-rpc", "--l1-eth-rpc", url))
		require.Equal(t, url, cfg.L1EthRpc)
	})
}

func TestRollupRpc(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		verifyArgsInvalid(t, "flag rollup-rpc is required", addRequiredArgsExcept("--rollup-rpc"))
	})

	t.Run("Valid", func(t *testing.T) {
		url := "http://example.com:9999"
		cfg := configForArgs(t, addRequiredArgsExcept("--rollup-rpc", "--rollup-rpc", url))
		require.Equal(t, url, cfg.RollupRpc)
	})
}

func TestGameFactoryAddress(t *testing.T) {
	t.Run("RequiredIfNetworkNetSet", func(t *testing.T) {
		verifyArgsInvalid(t, "flag game-factory-address or network is required", addRequiredArgsExcept("--game-factory-address"))
	})

	t.Run("Valid", func(t *testing.T) {
		addr := common.Address{0x11, 0x22}
		cfg := configForArgs(t, addRequiredArgsExcept("--game-factory-address", "--game-factory-address", addr.Hex()))
		require.Equal(t, addr, cfg.GameFactoryAddress)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t, "invalid address: foo", addRequiredArgsExcept("--game-factory-address", "--game-factory-address", "foo"))
	})

	t.Run("OverridesNetwork", func(t *testing.T) {
		addr := common.Address{0xbb, 0xcc, 0xdd}
		cfg := configForArgs(t, addRequiredArgsExcept("--game-factory-address", "--game-factory-address", addr.Hex(), "--network", "op-sepolia"))
		require.Equal(t, addr, cfg.GameFactoryAddress)
	})
}

func TestNetwork(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		opSepoliaChainId := uint64(11155420)
		opSepolia, err := superchain.GetChain(opSepoliaChainId)
		require.NoError(t, err)
		opSepoliaCfg, err := opSepolia.Config()
		require.NoError(t, err)
		cfg := configForArgs(t, addRequiredArgsExcept("--game-factory-address", "--network=op-sepolia"))
		require.EqualValues(t, *opSepoliaCfg.Addresses.DisputeGameFactoryProxy, cfg.GameFactoryAddress)
	})

	t.Run("UnknownNetwork", func(t *testing.T) {
		verifyArgsInvalid(t, "unknown chain: not-a-network", addRequiredArgsExcept("--game-factory-address", "--network=not-a-network"))
	})
}

func TestHonestActors(t *testing.T) {
	t.Run("NotRequired", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs())
		require.Empty(t, cfg.HonestActors)
	})

	t.Run("SingleValue", func(t *testing.T) {
		addr := common.Address{0xbb}
		cfg := configForArgs(t, addRequiredArgs("--honest-actors", addr.Hex()))
		require.Len(t, cfg.HonestActors, 1)
		require.Contains(t, cfg.HonestActors, addr)
	})

	t.Run("MultiValue", func(t *testing.T) {
		addr1 := common.Address{0xaa}
		addr2 := common.Address{0xbb}
		addr3 := common.Address{0xcc}
		cfg := configForArgs(t, addRequiredArgs(
			"--honest-actors", addr1.Hex(),
			"--honest-actors", addr2.Hex(),
			"--honest-actors", addr3.Hex(),
		))
		require.Len(t, cfg.HonestActors, 3)
		require.Contains(t, cfg.HonestActors, addr1)
		require.Contains(t, cfg.HonestActors, addr2)
		require.Contains(t, cfg.HonestActors, addr3)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t,
			"invalid honest actor address: invalid address: 0xnope",
			addRequiredArgs("-honest-actors", "0xnope"))
	})
}

func TestMonitorInterval(t *testing.T) {
	t.Run("UsesDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs())
		require.Equal(t, config.DefaultMonitorInterval, cfg.MonitorInterval)
	})

	t.Run("Valid", func(t *testing.T) {
		expected := 100 * time.Second
		cfg := configForArgs(t, addRequiredArgs("--monitor-interval", "100s"))
		require.Equal(t, expected, cfg.MonitorInterval)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"invalid value \"abc\" for flag -monitor-interval",
			addRequiredArgs("--monitor-interval", "abc"))
	})
}

func TestGameWindow(t *testing.T) {
	t.Run("UsesDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs())
		require.Equal(t, config.DefaultGameWindow, cfg.GameWindow)
	})

	t.Run("Valid", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs("--game-window=1m"))
		require.Equal(t, time.Minute, cfg.GameWindow)
	})

	t.Run("ParsesDefault", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs("--game-window=672h"))
		require.Equal(t, config.DefaultGameWindow, cfg.GameWindow)
	})
}

func TestIgnoredGames(t *testing.T) {
	t.Run("NotRequired", func(t *testing.T) {
		cfg := configForArgs(t, addRequiredArgs())
		require.Empty(t, cfg.IgnoredGames)
	})

	t.Run("SingleValue", func(t *testing.T) {
		addr := common.Address{0xbb}
		cfg := configForArgs(t, addRequiredArgs("--ignored-games", addr.Hex()))
		require.Len(t, cfg.IgnoredGames, 1)
		require.Contains(t, cfg.IgnoredGames, addr)
	})

	t.Run("MultiValue", func(t *testing.T) {
		addr1 := common.Address{0xaa}
		addr2 := common.Address{0xbb}
		addr3 := common.Address{0xcc}
		cfg := configForArgs(t, addRequiredArgs(
			"--ignored-games", addr1.Hex(),
			"--ignored-games", addr2.Hex(),
			"--ignored-games", addr3.Hex(),
		))
		require.Len(t, cfg.IgnoredGames, 3)
		require.Contains(t, cfg.IgnoredGames, addr1)
		require.Contains(t, cfg.IgnoredGames, addr2)
		require.Contains(t, cfg.IgnoredGames, addr3)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(t,
			"invalid ignored game address: invalid address: 0xnope",
			addRequiredArgs("-ignored-games", "0xnope"))
	})
}

func TestMaxConcurrency(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		expected := uint(345)
		cfg := configForArgs(t, addRequiredArgs("--max-concurrency", "345"))
		require.Equal(t, expected, cfg.MaxConcurrency)
	})

	t.Run("Invalid", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"invalid value \"abc\" for flag -max-concurrency",
			addRequiredArgs("--max-concurrency", "abc"))
	})

	t.Run("Zero", func(t *testing.T) {
		verifyArgsInvalid(
			t,
			"max-concurrency must not be 0",
			addRequiredArgs("--max-concurrency", "0"))
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
	fullArgs := append([]string{"op-dispute-mon"}, cliArgs...)
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

func requiredArgs() map[string]string {
	args := map[string]string{
		"--l1-eth-rpc":           l1EthRpc,
		"--rollup-rpc":           rollupRpc,
		"--game-factory-address": gameFactoryAddressValue,
	}
	return args
}

func toArgList(req map[string]string) []string {
	var combined []string
	for name, value := range req {
		combined = append(combined, fmt.Sprintf("%s=%s", name, value))
	}
	return combined
}

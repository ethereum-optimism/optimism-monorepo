package bootstrap

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestNewDelayedWETHConfigFromCLI(t *testing.T) {
	ctx, err := parseCLIArgs(DelayedWETHFlags,
		"--artifacts-locator", "tag://op-contracts/v1.6.0",
		"--l1-rpc-url", "http://foo",
		"--private-key", "0x123456")
	require.NoError(t, err)

	logger := testlog.Logger(t, log.LvlInfo)
	cfg, err := NewDelayedWETHConfigFromClI(ctx, logger)
	require.NoError(t, err)
	require.Same(t, logger, cfg.Logger)
	require.Equal(t, "op-contracts/v1.6.0", cfg.ArtifactsLocator.Tag)
	require.True(t, cfg.ArtifactsLocator.IsTag())
	require.Equal(t, "0x123456", cfg.PrivateKey)
}

func parseCLIArgs(flags []cli.Flag, args ...string) (*cli.Context, error) {
	app := cli.NewApp()
	app.Flags = cliapp.ProtectFlags(flags)
	var ctx *cli.Context
	app.Action = func(c *cli.Context) error {
		ctx = c
		return nil
	}
	argsWithCmd := make([]string, len(args)+1)
	argsWithCmd[0] = "bootstrap"
	copy(argsWithCmd[1:], args)
	err := app.Run(argsWithCmd)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

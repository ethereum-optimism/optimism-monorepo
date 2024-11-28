package bootstrap

import (
	"reflect"
	"testing"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func TestNewDisputeGameConfigFromCLI(t *testing.T) {
	ctx, err := parseCLIArgs(DisputeGameFlags,
		"--artifacts-locator", "tag://op-contracts/v1.6.0",
		"--l1-rpc-url", "http://foo",
		"--private-key", "0x123456",

		"--game-type", "2",
		"--delayed-weth-proxy", common.Address{0xaa}.Hex(),
		"--anchor-state-registry-proxy", common.Address{0xbb}.Hex(),
		"--l2-chain-id", "901",
		"--proposer", common.Address{0xcc}.Hex(),
		"--challenger", common.Address{0xdd}.Hex(),
		"--vm", common.Address{0xee}.Hex(),
	)
	require.NoError(t, err)

	logger := testlog.Logger(t, log.LvlInfo)
	cfg, err := NewDisputeGameConfigFromCLI(ctx, logger)
	require.NoError(t, err)
	require.Same(t, logger, cfg.Logger)
	require.Equal(t, "op-contracts/v1.6.0", cfg.ArtifactsLocator.Tag)
	require.True(t, cfg.ArtifactsLocator.IsTag())
	require.Equal(t, "0x123456", cfg.PrivateKey)
	require.Equal(t, "FaultDisputeGame", cfg.GameKind)
	require.Equal(t, uint32(2), cfg.GameType)
	require.Equal(t, standard.DisputeAbsolutePrestate, cfg.AbsolutePrestate)
	require.Equal(t, standard.DisputeMaxGameDepth, cfg.MaxGameDepth)
	require.Equal(t, standard.DisputeSplitDepth, cfg.SplitDepth)
	require.Equal(t, standard.DisputeClockExtension, cfg.ClockExtension)
	require.Equal(t, standard.DisputeMaxClockDuration, cfg.MaxClockDuration)
	require.Equal(t, common.Address{0xaa}, cfg.DelayedWethProxy)
	require.Equal(t, common.Address{0xbb}, cfg.AnchorStateRegistryProxy)
	require.Equal(t, common.Address{0xcc}, cfg.Proposer)
	require.Equal(t, common.Address{0xdd}, cfg.Challenger)
	require.Equal(t, common.Address{0xee}, cfg.Vm)
	require.Equal(t, uint64(901), cfg.L2ChainId)

	// Check all fields are set to ensure any newly added fields don't get missed.
	cfgRef := reflect.ValueOf(cfg)
	cfgType := reflect.TypeOf(cfg)
	var unsetFields []string
	for i := 0; i < cfgRef.NumField(); i++ {
		field := cfgType.Field(i)
		if field.Type == reflect.TypeOf(cfg.privateKeyECDSA) {
			// privateKeyECDSA is only set when Check() is called so skip it.
			continue
		}
		if cfgRef.Field(i).IsZero() {
			unsetFields = append(unsetFields, field.Name)
		}
	}
	require.Empty(t, unsetFields, "Found unset fields in config")
}

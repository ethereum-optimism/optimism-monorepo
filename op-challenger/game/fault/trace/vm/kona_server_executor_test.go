package vm

import (
	"math/big"
	"slices"
	"testing"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestKonaFillHostCommand(t *testing.T) {
	dir := "mockdir"
	cfg := Config{
		L1:       "http://localhost:8888",
		L1Beacon: "http://localhost:9000",
		L2s:      []string{"http://localhost:9999"},
		Server:   "./bin/mockserver",
		Networks: []string{"op-mainnet"},
	}
	inputs := utils.LocalGameInputs{
		L1Head:        common.Hash{0x11},
		L2Head:        common.Hash{0x22},
		L2OutputRoot:  common.Hash{0x33},
		L2Claim:       common.Hash{0x44},
		L2BlockNumber: big.NewInt(3333),
	}
	vmConfig := NewKonaExecutor()

	args, err := vmConfig.OracleCommand(cfg, dir, inputs)
	require.NoError(t, err)

	require.True(t, slices.Contains(args, "single"))
	require.True(t, slices.Contains(args, "--server"))
	require.True(t, slices.Contains(args, "--l1-node-address"))
	require.True(t, slices.Contains(args, "--l1-beacon-address"))
	require.True(t, slices.Contains(args, "--l2-node-address"))
	require.True(t, slices.Contains(args, "--data-dir"))
	require.True(t, slices.Contains(args, "--l2-chain-id"))
	require.True(t, slices.Contains(args, "--l1-head"))
	require.True(t, slices.Contains(args, "--l2-head"))
	require.True(t, slices.Contains(args, "--l2-output-root"))
	require.True(t, slices.Contains(args, "--l2-claim"))
	require.True(t, slices.Contains(args, "--l2-block-number"))
}

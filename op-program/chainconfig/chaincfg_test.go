package chainconfig

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-program/chainconfig/test"
	"github.com/stretchr/testify/require"
)

// TestGetCustomRollupConfig tests loading the custom rollup configs from test embed FS.
func TestGetCustomRollupConfig(t *testing.T) {
	config, err := rollupConfigByChainID(901, test.TestCustomChainConfigFS)
	require.NoError(t, err)
	require.Equal(t, config.L1ChainID.Uint64(), uint64(900))
	require.Equal(t, config.L2ChainID.Uint64(), uint64(901))

	_, err = rollupConfigByChainID(900, test.TestCustomChainConfigFS)
	require.Error(t, err)
}

// TestGetCustomChainConfig tests loading the custom chain configs from test embed FS.
func TestGetCustomChainConfig(t *testing.T) {
	config, err := chainConfigByChainID(901, test.TestCustomChainConfigFS)
	require.NoError(t, err)
	require.Equal(t, config.ChainID.Uint64(), uint64(901))

	_, err = chainConfigByChainID(900, test.TestCustomChainConfigFS)
	require.Error(t, err)
}

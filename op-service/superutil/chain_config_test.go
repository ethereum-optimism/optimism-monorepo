package superutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadOPStackChainConfigFromChainID(t *testing.T) {
	t.Run("mainnet", func(t *testing.T) {
		chainID := uint64(10)
		cfg, err := LoadOPStackChainConfigFromChainID(chainID)
		require.NoError(t, err)
		require.Equal(t, chainID, cfg.ChainID.Uint64())
	})

	t.Run("nonexistent chain", func(t *testing.T) {
		chainID := uint64(23409527340)
		cfg, err := LoadOPStackChainConfigFromChainID(chainID)
		require.Error(t, err)
		require.Nil(t, cfg)
	})
}

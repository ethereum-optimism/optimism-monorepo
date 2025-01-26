package predeploys

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPredeployAddresses(t *testing.T) {
	t.Run("all addresses are loaded", func(t *testing.T) {
		// Check a few key addresses to ensure they're loaded correctly
		l2ToL1MessagePasser := Predeploys["L2ToL1MessagePasser"]
		require.NotNil(t, l2ToL1MessagePasser)
		require.Equal(t, "0x4200000000000000000000000000000000000016", l2ToL1MessagePasser.Address.Hex())
		require.False(t, l2ToL1MessagePasser.ProxyDisabled)

		weth := Predeploys["WETH"]
		require.NotNil(t, weth)
		require.Equal(t, "0x4200000000000000000000000000000000000006", weth.Address.Hex())
		require.True(t, weth.ProxyDisabled)

		// Check that all addresses are mapped correctly
		for name, predeploy := range Predeploys {
			// Verify the address is also in the address map
			byAddr, exists := PredeploysByAddress[predeploy.Address]
			require.True(t, exists, "address for %s not found in PredeploysByAddress", name)
			require.Equal(t, predeploy, byAddr)
		}
	})

	t.Run("governance token has enabled function", func(t *testing.T) {
		governanceToken := Predeploys["GovernanceToken"]
		require.NotNil(t, governanceToken)
		require.NotNil(t, governanceToken.Enabled)

		// Test the enabled function with mock configs
		type mockConfig struct{ governance bool }
		func (m mockConfig) GovernanceEnabled() bool { return m.governance }

		require.True(t, governanceToken.Enabled(mockConfig{governance: true}))
		require.False(t, governanceToken.Enabled(mockConfig{governance: false}))
	})
}

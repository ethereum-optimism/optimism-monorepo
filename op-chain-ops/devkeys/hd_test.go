package devkeys

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestMnemonicDevKeys(t *testing.T) {
	m, err := NewMnemonicDevKeys(TestMnemonic)
	require.NoError(t, err)

	t.Run("default", func(t *testing.T) {
		defaultAccount, err := m.Address(DefaultKey)
		require.NoError(t, err)
		// Sanity check against a well-known dev account address,
		// to ensure the mnemonic path is formatted with the right hardening at each path segment.
		require.Equal(t, common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"), defaultAccount)

		// Check that we can localize users to a chain
		chain1UserKey0, err := m.Address(ChainUserKeys(big.NewInt(1))(0))
		require.NoError(t, err)
		require.NotEqual(t, defaultAccount, chain1UserKey0)
	})

	t.Run("superchain-operator", func(t *testing.T) {
		keys := SuperchainOperatorKeys(big.NewInt(1))
		// Check that each key address and name are unique
		addrs := make(map[common.Address]struct{})
		names := make(map[string]struct{})
		for i := SuperchainOperatorRole(0); i < 20; i++ {
			key := keys(i)
			secret, err := m.Secret(key)
			require.NoError(t, err)
			addr, err := m.Address(key)
			require.NoError(t, err)
			require.Equal(t, crypto.PubkeyToAddress(secret.PublicKey), addr)
			addrs[addr] = struct{}{}
			names[key.String()] = struct{}{}
		}
		require.Len(t, addrs, 20, "unique address for each account")
		require.Len(t, names, 20, "unique name for each account")
	})

	t.Run("chain-operator", func(t *testing.T) {
		keys := ChainOperatorKeys(big.NewInt(1))
		// Check that each key address and name is unique
		addrs := make(map[common.Address]struct{})
		names := make(map[string]struct{})
		for i := ChainOperatorRole(0); i < 20; i++ {
			key := keys(i)
			secret, err := m.Secret(key)
			require.NoError(t, err)
			addr, err := m.Address(key)
			require.NoError(t, err)
			require.Equal(t, crypto.PubkeyToAddress(secret.PublicKey), addr)
			addrs[addr] = struct{}{}
			names[key.String()] = struct{}{}
		}
		require.Len(t, addrs, 20, "unique address for each account")
		require.Len(t, names, 20, "unique name for each account")
	})

}

package forking

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func setupTrie(t *testing.T) (*ForkedAccountsTrie, *MockForkSource) {
	mockSource := new(MockForkSource)
	stateRoot := common.HexToHash("0x1234")
	mockSource.On("StateRoot").Return(stateRoot)

	trie := &ForkedAccountsTrie{
		stateRoot: stateRoot,
		src:       mockSource,
		diff:      NewExportDiff(),
	}
	return trie, mockSource
}

func TestForkedAccountsTrie_GetAccount(t *testing.T) {
	trie, mockSource := setupTrie(t)
	addr := common.HexToAddress("0x1234")

	// Setup mock responses
	expectedNonce := uint64(1)
	expectedBalance := uint256.NewInt(100)
	expectedCode := []byte{1, 2, 3, 4}
	expectedCodeHash := crypto.Keccak256Hash(expectedCode)

	mockSource.On("Nonce", addr).Return(expectedNonce, nil)
	mockSource.On("Balance", addr).Return(expectedBalance, nil)
	mockSource.On("Code", addr).Return(expectedCode, nil)

	// Test initial account retrieval
	account, err := trie.GetAccount(addr)
	require.NoError(t, err)
	require.Equal(t, expectedNonce, account.Nonce)
	require.Equal(t, expectedBalance, uint256.NewInt(0).SetBytes(account.Balance.Bytes()))
	require.Equal(t, expectedCodeHash.Bytes(), account.CodeHash)

	// Update account and verify diff
	newNonce := uint64(2)
	newBalance := uint256.NewInt(200)
	account.Nonce = newNonce
	account.Balance = newBalance

	err = trie.UpdateAccount(addr, account, 0)
	require.NoError(t, err)

	// Verify updated account
	updatedAccount, err := trie.GetAccount(addr)
	require.NoError(t, err)
	require.Equal(t, newNonce, updatedAccount.Nonce)
	require.Equal(t, newBalance, uint256.NewInt(0).SetBytes(updatedAccount.Balance.Bytes()))
}

func TestForkedAccountsTrie_Storage(t *testing.T) {
	trie, mockSource := setupTrie(t)
	addr := common.HexToAddress("0x1234")
	key := common.HexToHash("0x1")
	value := common.HexToHash("0x2")

	// Setup mock for initial storage value
	mockSource.On("StorageAt", addr, key).Return(value, nil)

	// Test initial storage retrieval
	storageValue, err := trie.GetStorage(addr, key.Bytes())
	require.NoError(t, err)
	require.Equal(t, value.Bytes(), storageValue)

	// Update storage
	newValue := common.HexToHash("0x3")
	err = trie.UpdateStorage(addr, key.Bytes(), newValue.Bytes())
	require.NoError(t, err)

	// Verify updated storage
	updatedValue, err := trie.GetStorage(addr, key.Bytes())
	require.NoError(t, err)
	require.Equal(t, newValue.Bytes(), updatedValue)
}

func TestForkedAccountsTrie_ContractCode(t *testing.T) {
	trie, mockSource := setupTrie(t)
	addr := common.HexToAddress("0x1234")
	code := []byte{1, 2, 3, 4}
	codeHash := crypto.Keccak256Hash(code)

	// Setup mock for code retrieval
	mockSource.On("Code", addr).Return(code, nil)

	// Test initial code retrieval
	retrievedCode, err := trie.ContractCode(addr, codeHash)
	require.NoError(t, err)
	require.Equal(t, code, retrievedCode)

	// Update code
	newCode := []byte{5, 6, 7, 8}
	newCodeHash := crypto.Keccak256Hash(newCode)

	err = trie.UpdateContractCode(addr, newCodeHash, newCode)
	require.NoError(t, err)

	// Verify updated code
	updatedCode, err := trie.ContractCode(addr, newCodeHash)
	require.NoError(t, err)
	require.Equal(t, newCode, updatedCode)
}

func TestForkedAccountsTrie_DeleteAccount(t *testing.T) {
	trie, _ := setupTrie(t)
	addr := common.HexToAddress("0x1234")

	// Setup initial account
	account := &types.StateAccount{
		Nonce:    1,
		Balance:  uint256.NewInt(100),
		CodeHash: crypto.Keccak256([]byte{1, 2, 3, 4}),
	}

	err := trie.UpdateAccount(addr, account, 0)
	require.NoError(t, err)

	// Delete account
	err = trie.DeleteAccount(addr)
	require.NoError(t, err)

	// Verify account is marked as deleted in diff
	require.Nil(t, trie.diff.Account[addr])
}

func TestForkedAccountsTrie_Copy(t *testing.T) {
	trie, _ := setupTrie(t)
	addr := common.HexToAddress("0x1234")

	// Setup some initial state
	account := &types.StateAccount{
		Nonce:    1,
		Balance:  uint256.NewInt(100),
		CodeHash: crypto.Keccak256([]byte{1, 2, 3, 4}),
	}
	err := trie.UpdateAccount(addr, account, 0)
	require.NoError(t, err)

	// Make a copy
	cpy := trie.Copy()

	// Verify copy has same state
	require.Equal(t, trie.stateRoot, cpy.stateRoot)
	require.Equal(t, trie.diff.Account[addr].Nonce, cpy.diff.Account[addr].Nonce)
	require.True(t, trie.diff.Account[addr].Balance.Eq(cpy.diff.Account[addr].Balance))

	// Modify copy and verify original is unchanged
	newAccount := &types.StateAccount{
		Nonce:    2,
		Balance:  uint256.NewInt(200),
		CodeHash: crypto.Keccak256([]byte{5, 6, 7, 8}),
	}
	err = cpy.UpdateAccount(addr, newAccount, 0)
	require.NoError(t, err)

	originalAccount, err := trie.GetAccount(addr)
	require.NoError(t, err)
	require.Equal(t, uint64(1), originalAccount.Nonce)
	require.True(t, uint256.NewInt(100).Eq(uint256.NewInt(0).SetBytes(originalAccount.Balance.Bytes())))
}

func TestForkedAccountsTrie_HasDiff(t *testing.T) {
	trie, _ := setupTrie(t)

	// Initially no diff
	require.False(t, trie.HasDiff())

	// Add account change
	addr := common.HexToAddress("0x1234")
	account := &types.StateAccount{
		Nonce:    1,
		Balance:  uint256.NewInt(100),
		CodeHash: crypto.Keccak256([]byte{1, 2, 3, 4}),
	}
	err := trie.UpdateAccount(addr, account, 0)
	require.NoError(t, err)

	// Verify diff exists
	require.True(t, trie.HasDiff())

	// Clear diff
	trie.ClearDiff()
	require.False(t, trie.HasDiff())
}

func TestForkedAccountsTrie_UnsupportedOperations(t *testing.T) {
	trie, _ := setupTrie(t)

	require.Panics(t, func() { trie.GetKey([]byte{1, 2, 3}) })
	require.Panics(t, func() { trie.Commit(false) })
	require.Panics(t, func() { trie.Witness() })

	_, err := trie.NodeIterator(nil)
	require.Error(t, err)

	err = trie.Prove(nil, nil)
	require.Error(t, err)
}

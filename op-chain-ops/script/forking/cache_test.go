package forking

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockForkSource implements ForkSource interface for testing
type MockForkSource struct {
	mock.Mock
}

func (m *MockForkSource) URLOrAlias() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockForkSource) StateRoot() common.Hash {
	args := m.Called()
	return args.Get(0).(common.Hash)
}

func (m *MockForkSource) Nonce(addr common.Address) (uint64, error) {
	args := m.Called(addr)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockForkSource) Balance(addr common.Address) (*uint256.Int, error) {
	args := m.Called(addr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*uint256.Int), args.Error(1)
}

func (m *MockForkSource) StorageAt(addr common.Address, key common.Hash) (common.Hash, error) {
	args := m.Called(addr, key)
	return args.Get(0).(common.Hash), args.Error(1)
}

func (m *MockForkSource) Code(addr common.Address) ([]byte, error) {
	args := m.Called(addr)
	return args.Get(0).([]byte), args.Error(1)
}

func setupCache(t *testing.T) (*CachedSource, *MockForkSource) {
	mockSource := new(MockForkSource)
	stateRoot := common.HexToHash("0x1234")
	mockSource.On("StateRoot").Return(stateRoot)
	mockSource.On("URLOrAlias").Return("test_source")

	cached := Cache(mockSource)
	require.NotNil(t, cached)
	require.Equal(t, stateRoot, cached.StateRoot())
	require.Equal(t, "test_source", cached.URLOrAlias())

	return cached, mockSource
}

func TestCachedSource_Nonce(t *testing.T) {
	cached, mockSource := setupCache(t)
	addr := common.HexToAddress("0x1234")
	expectedNonce := uint64(42)

	// First call should hit the source
	mockSource.On("Nonce", addr).Return(expectedNonce, nil).Once()

	nonce, err := cached.Nonce(addr)
	require.NoError(t, err)
	require.Equal(t, expectedNonce, nonce)

	// Second call should use cache
	nonce, err = cached.Nonce(addr)
	require.NoError(t, err)
	require.Equal(t, expectedNonce, nonce)

	mockSource.AssertNumberOfCalls(t, "Nonce", 1)
}

func TestCachedSource_Balance(t *testing.T) {
	cached, mockSource := setupCache(t)
	addr := common.HexToAddress("0x5678")
	expectedBalance := uint256.NewInt(1000)

	// First call should hit the source
	mockSource.On("Balance", addr).Return(expectedBalance, nil).Once()

	balance, err := cached.Balance(addr)
	require.NoError(t, err)
	require.Equal(t, expectedBalance, balance)

	// Second call should use cache
	balance, err = cached.Balance(addr)
	require.NoError(t, err)
	require.Equal(t, expectedBalance, balance)

	// Verify the returned balance is a clone
	balance.Add(balance, uint256.NewInt(1))
	cachedBalance, _ := cached.Balance(addr)
	require.Equal(t, expectedBalance, cachedBalance)

	mockSource.AssertNumberOfCalls(t, "Balance", 1)
}

func TestCachedSource_Storage(t *testing.T) {
	cached, mockSource := setupCache(t)
	addr := common.HexToAddress("0x9abc")
	slot := common.HexToHash("0xdef0")
	expectedValue := common.HexToHash("0x1234")

	// First call should hit the source
	mockSource.On("StorageAt", addr, slot).Return(expectedValue, nil).Once()

	value, err := cached.StorageAt(addr, slot)
	require.NoError(t, err)
	require.Equal(t, expectedValue, value)

	// Second call should use cache
	value, err = cached.StorageAt(addr, slot)
	require.NoError(t, err)
	require.Equal(t, expectedValue, value)

	mockSource.AssertNumberOfCalls(t, "StorageAt", 1)
}

func TestCachedSource_Code(t *testing.T) {
	cached, mockSource := setupCache(t)
	addr := common.HexToAddress("0xdef0")
	expectedCode := []byte{1, 2, 3, 4}

	// First call should hit the source
	mockSource.On("Code", addr).Return(expectedCode, nil).Once()

	code, err := cached.Code(addr)
	require.NoError(t, err)
	require.Equal(t, expectedCode, code)

	// Second call should use cache
	code, err = cached.Code(addr)
	require.NoError(t, err)
	require.Equal(t, expectedCode, code)

	mockSource.AssertNumberOfCalls(t, "Code", 1)
}

func TestCachedSource_CacheEviction(t *testing.T) {
	cached, mockSource := setupCache(t)

	// Test nonce cache eviction
	for i := 0; i < 1001; i++ { // Cache size is 1000
		addr := common.BigToAddress(big.NewInt(int64(i)))
		mockSource.On("Nonce", addr).Return(uint64(i), nil).Once()
		_, _ = cached.Nonce(addr)
	}

	// This should cause first address to be evicted
	firstAddr := common.BytesToAddress([]byte{0})
	mockSource.On("Nonce", firstAddr).Return(uint64(0), nil).Once()
	_, _ = cached.Nonce(firstAddr)

	mockSource.AssertNumberOfCalls(t, "Nonce", 1002) // 1001 + 1 for evicted key
}

func TestCachedSource_MultipleStorageSlots(t *testing.T) {
	cached, mockSource := setupCache(t)
	addr := common.HexToAddress("0xabcd")
	slot1 := common.HexToHash("0x1111")
	slot2 := common.HexToHash("0x2222")
	value1 := common.HexToHash("0x3333")
	value2 := common.HexToHash("0x4444")

	mockSource.On("StorageAt", addr, slot1).Return(value1, nil).Once()
	mockSource.On("StorageAt", addr, slot2).Return(value2, nil).Once()

	// Different slots should trigger separate cache entries
	val1, err := cached.StorageAt(addr, slot1)
	require.NoError(t, err)
	require.Equal(t, value1, val1)

	val2, err := cached.StorageAt(addr, slot2)
	require.NoError(t, err)
	require.Equal(t, value2, val2)

	// Verify both are cached
	val1Again, _ := cached.StorageAt(addr, slot1)
	val2Again, _ := cached.StorageAt(addr, slot2)
	require.Equal(t, value1, val1Again)
	require.Equal(t, value2, val2Again)

	mockSource.AssertNumberOfCalls(t, "StorageAt", 2)
}

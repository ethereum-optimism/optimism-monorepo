package constraints

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

type mockWallet struct {
	balance types.Balance
	address types.Address
}

func (m mockWallet) Balance() types.Balance {
	return m.balance
}

func (m mockWallet) Address() types.Address {
	return m.address
}

func (m mockWallet) PrivateKey() types.Key {
	key, _ := crypto.HexToECDSA("123")
	return types.Key(key)
}

func (m mockWallet) SendETH(to types.Address, amount types.Balance) types.WriteInvocation[any] {
	panic("not implemented")
}

func (m mockWallet) Nonce() uint64 {
	return 0
}

func (m mockWallet) Sign(tx system.Transaction) (system.Transaction, error) {
	return tx, nil
}

func (m mockWallet) Send(ctx context.Context, tx system.Transaction) error {
	return nil
}

func (m mockWallet) Transactor() *bind.TransactOpts {
	return nil
}

var _ system.Wallet = (*mockWallet)(nil)

func newBigInt(x int64) *big.Int {
	return big.NewInt(x)
}

func TestWithBalance(t *testing.T) {
	tests := []struct {
		name           string
		walletBalance  types.Balance
		requiredAmount types.Balance
		expectPass     bool
	}{
		{
			name:           "balance greater than required",
			walletBalance:  types.NewBalance(newBigInt(200)),
			requiredAmount: types.NewBalance(newBigInt(100)),
			expectPass:     true,
		},
		{
			name:           "balance equal to required",
			walletBalance:  types.NewBalance(newBigInt(100)),
			requiredAmount: types.NewBalance(newBigInt(100)),
			expectPass:     false,
		},
		{
			name:           "balance less than required",
			walletBalance:  types.NewBalance(newBigInt(50)),
			requiredAmount: types.NewBalance(newBigInt(100)),
			expectPass:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wallet := mockWallet{
				balance: tt.walletBalance,
				address: common.HexToAddress("0x123"),
			}
			constraint := WithBalance(tt.requiredAmount)
			result := constraint.CheckWallet(wallet)
			assert.Equal(t, tt.expectPass, result, "balance check should match expected result")
		})
	}
}

func TestWalletConstraintFunc(t *testing.T) {
	called := false
	testFunc := WalletConstraintFunc(func(wallet system.Wallet) bool {
		called = true
		return true
	})

	wallet := mockWallet{
		balance: types.NewBalance(newBigInt(100)),
		address: common.HexToAddress("0x123"),
	}

	result := testFunc.CheckWallet(wallet)
	assert.True(t, called, "constraint function should have been called")
	assert.True(t, result, "constraint function should return true")
}

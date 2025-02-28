package validators

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/interfaces"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

// TestSystemTestHelper tests the basic implementation of systemTestHelper
func TestValidators(t *testing.T) {
	t.Run("multiple validators", func(t *testing.T) {
		walletGetter1, validator1 := AcquireL2WalletWithFunds(0, types.NewBalance(big.NewInt(1)))
		walletGetter2, validator2 := AcquireL2WalletWithFunds(0, types.NewBalance(big.NewInt(10)))
		lowLevelSystemGetter, validator3 := AcquireLowLevelSystem()

		// We create a system that has a low-level L1 chain and at least one wallet
		systestSystem := &mockSystem{
			l1: &mockChain{},
			l2s: []system.Chain{
				&mockChain{
					wallets: []system.Wallet{
						&mockWallet{
							balance: types.NewBalance(big.NewInt(2)),
						},
						&mockWallet{
							balance: types.NewBalance(big.NewInt(11)),
						},
					},
				},
			},
		}

		// Now we apply all validators, accumulating contexts

		systestT := systest.NewT(t)

		ctx1, err := validator1(systestT, systestSystem)
		systestT = systestT.WithContext(ctx1)
		require.NoError(t, err)

		ctx2, err := validator2(systestT, systestSystem)
		systestT = systestT.WithContext(ctx2)
		require.NoError(t, err)

		ctx3, err := validator3(systestT, systestSystem)
		systestT = systestT.WithContext(ctx3)
		require.NoError(t, err)

		ctx := systestT.Context()

		// Now we call all the getters to make sure they work
		wallet1 := walletGetter1(ctx)
		wallet2 := walletGetter2(ctx)
		lowLevelSystem := lowLevelSystemGetter(ctx)

		// And we ensure that the values are not mismatched
		require.NotEqual(t, wallet1, wallet2)
		require.NotEqual(t, wallet1, lowLevelSystem)
		require.NotEqual(t, wallet2, lowLevelSystem)

		// And that we got a lowlevelSystem
		require.NotNil(t, lowLevelSystem)
	})
}

type mockSystem struct {
	l1  system.Chain
	l2s []system.Chain
}

func (sys *mockSystem) Identifier() string {
	return "mock"
}

func (sys *mockSystem) L1() system.Chain {
	return sys.l1
}

func (sys *mockSystem) L2s() []system.Chain {
	return sys.l2s
}

type mockChain struct {
	wallets []system.Wallet
}

func (m *mockChain) RPCURL() string                                  { return "http://localhost:8545" }
func (m *mockChain) Client() (*ethclient.Client, error)              { return ethclient.Dial(m.RPCURL()) }
func (m *mockChain) ID() types.ChainID                               { return types.ChainID(big.NewInt(1)) }
func (m *mockChain) ContractsRegistry() interfaces.ContractsRegistry { return nil }
func (m *mockChain) Wallets(ctx context.Context) ([]system.Wallet, error) {
	return m.wallets, nil
}
func (m *mockChain) GasPrice(ctx context.Context) (*big.Int, error) {
	return big.NewInt(1), nil
}
func (m *mockChain) GasLimit(ctx context.Context, tx system.TransactionData) (uint64, error) {
	return 1000000, nil
}
func (m *mockChain) PendingNonceAt(ctx context.Context, address common.Address) (uint64, error) {
	return 0, nil
}
func (m *mockChain) SupportsEIP(ctx context.Context, eip uint64) bool {
	return true
}

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

var (
	_ system.Chain         = (*mockChain)(nil)
	_ system.LowLevelChain = (*mockChain)(nil)

	_ system.System = (*mockSystem)(nil)

	_ system.Wallet = (*mockWallet)(nil)
)

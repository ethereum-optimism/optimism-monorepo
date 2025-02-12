package interop

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func smokeTestScenario(chainIdx uint64, userSentinel interface{}) systest.SystemTestFunc {
	return func(t systest.T, sys system.System) {
		ctx := t.Context()
		logger := slog.With("test", "TestMinimal", "devnet", sys.Identifier())

		chain := sys.L2(chainIdx)
		logger = logger.With("chain", chain.ID())
		logger.InfoContext(ctx, "starting test")

		funds := types.NewBalance(big.NewInt(0.5 * constants.ETH))
		user := ctx.Value(userSentinel).(types.Wallet)

		scw0Addr := constants.SuperchainWETH
		scw0, err := chain.ContractsRegistry().SuperchainWETH(scw0Addr)
		require.NoError(t, err)
		logger.InfoContext(ctx, "using SuperchainWETH", "contract", scw0Addr)

		initialBalance, err := scw0.BalanceOf(user.Address()).Call(ctx)
		require.NoError(t, err)
		logger = logger.With("user", user.Address())
		logger.InfoContext(ctx, "initial balance retrieved", "balance", initialBalance)

		logger.InfoContext(ctx, "sending ETH to contract", "amount", funds)
		require.NoError(t, user.SendETH(scw0Addr, funds).Send(ctx).Wait())

		balance, err := scw0.BalanceOf(user.Address()).Call(ctx)
		require.NoError(t, err)
		logger.InfoContext(ctx, "final balance retrieved", "balance", balance)

		require.Equal(t, initialBalance.Add(funds), balance)
	}
}

func TestSystemWrapETH(t *testing.T) {
	chainIdx := uint64(0)         // We'll use the first L2 chain for this test
	testUserMarker := &struct{}{} // Sentinel for the user context value

	systest.SystemTest(t,
		smokeTestScenario(chainIdx, testUserMarker),
		walletFundsValidator(chainIdx, types.NewBalance(big.NewInt(1.0*constants.ETH)), testUserMarker),
	)
}

func TestInteropSystemNoop(t *testing.T) {
	systest.InteropSystemTest(t, func(t systest.T, sys system.InteropSystem) {
		slog.Info("noop")
	})
}

func TestSmokeTestFailure(t *testing.T) {
	// Create mock failing system
	mockAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	mockWallet := &mockFailingWallet{
		addr: mockAddr,
		key:  "mock-key",
		bal:  types.NewBalance(big.NewInt(1000000)),
	}
	mockChain := &mockFailingChain{
		id:     types.ChainID(big.NewInt(1234)),
		wallet: mockWallet,
		reg:    &mockRegistry{},
	}
	mockSys := &mockFailingSystem{chain: mockChain}

	// Run the smoke test logic and capture failures
	sentinel := &struct{}{}
	rt := NewRecordingT(context.WithValue(context.TODO(), sentinel, mockWallet))
	rt.TestScenario(
		smokeTestScenario(0, sentinel),
		mockSys,
	)

	// Verify that the test failed due to SendETH error
	require.True(t, rt.Failed(), "test should have failed")
	require.Contains(t, rt.Logs(), "transaction failure", "unexpected failure message")
}

/*
- deploy the contract (take bytecode from artifact, put it in a tx, with nil To addr)
- await tx confirmation
- from the receipt, or based on the nonce, you can determine the address where the code got deployed
- create client bindings over that address
- send emitLog call as a tx, to create a dummy log
- await confirmation (include + status code)
- send `validateMessage()` call as a tx, to consume the dummy log. The `Identifier` and `payloadHash` need to refer to the contents of the log we just emitted
- await confirmation (include + status code)
- await the op-supervisor to acknowledge the block (that the above tx was included in) to become cross-safe.
  - we could use op-geth, op-node, or op-supervisor RPC to do this.
  - op-geth `safe` block changes
  - op-node `syncStatus` RPC will include it
  - op-supervisor `crossSafe` RPC
*/
func TestInteropSystemEventLoggerReference(t *testing.T) {
	chainIdx := uint64(0)
	testUserMarker := &struct{}{} // Sentinel for the user context value

	systest.InteropSystemTest(t, func(t systest.T, sys system.InteropSystem) {
		ctx := t.Context()
		chain := sys.L2(chainIdx)
		rpcurl := chain.RPCURL()
		fmt.Println(rpcurl)
		client, err := ethclient.Dial(rpcurl)
		require.NoError(t, err)

		wallet := ctx.Value(testUserMarker).(types.Wallet)

		privateKey, err := crypto.HexToECDSA(wallet.PrivateKey())
		require.NoError(t, err)

		fromAddress := wallet.Address()
		nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
		require.NoError(t, err)

		gasPrice, err := client.SuggestGasPrice(context.Background())
		require.NoError(t, err)

		auth := bind.NewKeyedTransactor(privateKey)
		auth.Nonce = big.NewInt(int64(nonce))
		auth.Value = big.NewInt(0)       // in wei
		auth.GasLimit = uint64(30000000) // in units
		auth.GasPrice = gasPrice

		address, tx, instance, err := bindings.DeployEventLogger(auth, client)
		require.NoError(t, err)

		receipt, err := bind.WaitMined(context.Background(), client, tx)
		require.NoError(t, err)
		fmt.Println(receipt.Logs)
		require.Equal(t, receipt.Status, uint64(1)) // Check transaction succeeded

		fmt.Println(address.Hex())   // 0x147B8eb97fD247D06C4006D269c90C1908Fb5D54
		fmt.Println(tx.Hash().Hex()) // 0xdae8ba5444eefdc99f4d45cd0c4f24056cba6a02cefbf78066ef9f4188ff7dc0

		auth.Nonce = big.NewInt(int64(nonce + 1))
		tx2, err := instance.EmitLog(auth, [][32]byte{}, []byte{})
		require.NoError(t, err)

		receipt2, err := bind.WaitMined(context.Background(), client, tx2)
		require.NoError(t, err)
		fmt.Println(receipt2.Logs)
		require.Equal(t, receipt2.Status, uint64(1)) // Check transaction succeeded

		log := receipt2.Logs[0]

		fmt.Println("calling validateMessage")
		auth.Nonce = big.NewInt(int64(nonce + 2))
		gasPrice, err = client.SuggestGasPrice(ctx)
		require.NoError(t, err)
		auth.GasPrice = gasPrice

		block, err := client.BlockByHash(ctx, log.BlockHash)
		require.NoError(t, err)

		msgPayload := make([]byte, 0)
		for _, topic := range log.Topics {
			msgPayload = append(msgPayload, topic.Bytes()...)
		}
		msgPayload = append(msgPayload, log.Data...)
		expectedHash := common.BytesToHash(crypto.Keccak256(msgPayload))

		tx3, err := instance.ValidateMessage(auth, bindings.Identifier{
			Origin:      log.Address,
			BlockNumber: big.NewInt(int64(log.BlockNumber)),
			LogIndex:    big.NewInt(int64(log.Index)),
			Timestamp:   big.NewInt(int64(block.Time())),
			ChainId:     chain.ID(),
		}, expectedHash)

		require.NoError(t, err)

		fmt.Println("waiting for tx3", tx3.Hash().Hex())
		receipt3, err := bind.WaitMined(ctx, client, tx3)

		require.NoError(t, err)
		fmt.Println(receipt3.Logs)
		require.Equal(t, receipt3.Status, uint64(1)) // Check transaction succeeded
	},
		walletFundsValidator(chainIdx, types.NewBalance(big.NewInt(1.0*constants.ETH)), testUserMarker),
	)
}

func TestInteropSystemEventLogger(t *testing.T) {
	chainIdx := uint64(0)
	walletGetter, fundsValidator := AcquireL2WalletWithFunds(chainIdx, types.NewBalance(big.NewInt(1.0*constants.ETH)))

	systest.InteropSystemTest(t, func(t systest.T, sys system.InteropSystem) {
		ctx := t.Context()
		chain := sys.L2(chainIdx)
		rpcurl := chain.RPCURL()
		fmt.Println(rpcurl)
		client, err := ethclient.Dial(rpcurl)
		require.NoError(t, err)

		wallet := walletGetter(ctx)

		privateKey, err := crypto.HexToECDSA(wallet.PrivateKey())
		require.NoError(t, err)

		fromAddress := wallet.Address()
		nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
		require.NoError(t, err)

		gasPrice, err := client.SuggestGasPrice(context.Background())
		require.NoError(t, err)

		auth := bind.NewKeyedTransactor(privateKey)
		auth.Nonce = big.NewInt(int64(nonce))
		auth.Value = big.NewInt(0)       // in wei
		auth.GasLimit = uint64(30000000) // in units
		auth.GasPrice = gasPrice

		address, tx, instance, err := bindings.DeployEventLogger(auth, client)
		require.NoError(t, err)

		receipt, err := bind.WaitMined(context.Background(), client, tx)
		require.NoError(t, err)
		fmt.Println(receipt.Logs)
		require.Equal(t, receipt.Status, uint64(1)) // Check transaction succeeded

		fmt.Println(address.Hex())   // 0x147B8eb97fD247D06C4006D269c90C1908Fb5D54
		fmt.Println(tx.Hash().Hex()) // 0xdae8ba5444eefdc99f4d45cd0c4f24056cba6a02cefbf78066ef9f4188ff7dc0

		auth.Nonce = big.NewInt(int64(nonce + 1))
		tx2, err := instance.EmitLog(auth, [][32]byte{}, []byte{})
		require.NoError(t, err)

		receipt2, err := bind.WaitMined(context.Background(), client, tx2)
		require.NoError(t, err)
		fmt.Println(receipt2.Logs)
		require.Equal(t, receipt2.Status, uint64(1)) // Check transaction succeeded

		log := receipt2.Logs[0]

		fmt.Println("calling validateMessage")
		auth.Nonce = big.NewInt(int64(nonce + 2))
		gasPrice, err = client.SuggestGasPrice(ctx)
		require.NoError(t, err)
		auth.GasPrice = gasPrice

		block, err := client.BlockByHash(ctx, log.BlockHash)
		require.NoError(t, err)

		msgPayload := make([]byte, 0)
		for _, topic := range log.Topics {
			msgPayload = append(msgPayload, topic.Bytes()...)
		}
		msgPayload = append(msgPayload, log.Data...)
		expectedHash := common.BytesToHash(crypto.Keccak256(msgPayload))

		tx3, err := instance.ValidateMessage(auth, bindings.Identifier{
			Origin:      log.Address,
			BlockNumber: big.NewInt(int64(log.BlockNumber)),
			LogIndex:    big.NewInt(int64(log.Index)),
			Timestamp:   big.NewInt(int64(block.Time())),
			ChainId:     chain.ID(),
		}, expectedHash)

		require.NoError(t, err)

		fmt.Println("waiting for tx3", tx3.Hash().Hex())
		receipt3, err := bind.WaitMined(ctx, client, tx3)

		require.NoError(t, err)
		fmt.Println(receipt3.Logs)
		require.Equal(t, receipt3.Status, uint64(1)) // Check transaction succeeded
	},
		fundsValidator,
	)
}

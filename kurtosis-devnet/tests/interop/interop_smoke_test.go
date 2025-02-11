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
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	sdktypes "github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func smokeTestScenario(chainIdx uint64, walletGetter validators.WalletGetter) systest.SystemTestFunc {
	return func(t systest.T, sys system.System) {
		ctx := t.Context()
		logger := slog.With("test", "TestMinimal", "devnet", sys.Identifier())

		chain := sys.L2(chainIdx)
		logger = logger.With("chain", chain.ID())
		logger.InfoContext(ctx, "starting test")

		funds := sdktypes.NewBalance(big.NewInt(0.5 * constants.ETH))
		user := walletGetter(ctx)

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
	chainIdx := uint64(0) // We'll use the first L2 chain for this test

	walletGetter, fundsValidator := validators.AcquireL2WalletWithFunds(chainIdx, sdktypes.NewBalance(big.NewInt(1.0*constants.ETH)))

	systest.SystemTest(t,
		smokeTestScenario(chainIdx, walletGetter),
		fundsValidator,
	)
}

func TestInteropSystemNoop(t *testing.T) {
	systest.InteropSystemTest(t, func(t systest.T, sys system.InteropSystem) {
		slog.Info("noop")
	})
}

// TODO Since the mocked wallet now has to receive a valid private key,
// this test makes little sense
//
// func TestSmokeTestFailure(t *testing.T) {
// 	// Create mock failing system
// 	mockAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
// 	mockWallet := &mockFailingWallet{
// 		addr: mockAddr,
// 		key:  "mock-key",
// 		bal:  sdktypes.NewBalance(big.NewInt(1000000)),
// 	}
// 	mockChain := &mockFailingChain{
// 		id:     sdktypes.ChainID(big.NewInt(1234)),
// 		wallet: mockWallet,
// 		reg:    &mockRegistry{},
// 	}
// 	mockSys := &mockFailingSystem{chain: mockChain}

// 	// Run the smoke test logic and capture failures
// 	sentinel := &struct{}{}
// 	rt := NewRecordingT(context.WithValue(context.TODO(), sentinel, mockWallet))
// 	rt.TestScenario(
// 		smokeTestScenario(0, sentinel),
// 		mockSys,
// 	)
//
// 	// Verify that the test failed due to SendETH error
// 	require.True(t, rt.Failed(), "test should have failed")
// 	require.Contains(t, rt.Logs(), "transaction failure", "unexpected failure message")
// }

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
	walletGetter, fundsValidator := validators.AcquireL2WalletWithFunds(chainIdx, sdktypes.NewBalance(big.NewInt(1.0*constants.ETH)))

	systest.InteropSystemTest(t, func(t systest.T, sys system.InteropSystem) {
		ctx := t.Context()
		chain := sys.L2(chainIdx)

		// We need to create a ethclient.Client
		client, err := chain.Client()
		require.NoError(t, err)

		wallet := walletGetter(ctx)

		fromAddress := wallet.Address()
		nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
		require.NoError(t, err)

		gasPrice, err := client.SuggestGasPrice(context.Background())
		require.NoError(t, err)

		transactor := wallet.Transactor()
		require.NoError(t, err)

		transactor.Nonce = big.NewInt(int64(nonce))
		transactor.Value = big.NewInt(0)       // in wei
		transactor.GasLimit = uint64(30000000) // in units
		transactor.GasPrice = gasPrice

		// We'll deploy the EventLogger contract
		address, tx, instance, err := bindings.DeployEventLogger(transactor, client)
		require.NoError(t, err)
		fmt.Printf("Deploying EventLogger in transaction %s\n", tx.Hash().Hex())

		// And wait for the deployment transaction to mine successfully
		receipt, err := bind.WaitMined(context.Background(), client, tx)
		require.NoError(t, err)
		require.Equal(t, receipt.Status, uint64(1)) // Check transaction succeeded

		// Let the user know
		fmt.Printf("Deployed EventLogger at %s in transaction %s\n", address.Hex(), tx.Hash().Hex())

		// FIXME Check if we need to fill the nonce everytime
		transactor.Nonce = big.NewInt(int64(nonce + 1))

		// Now call EmitLog on the deployed contract
		tx2, err := instance.EmitLog(transactor, [][32]byte{}, []byte{})
		require.NoError(t, err)

		// And wait for the transaction to mine successfully
		receipt2, err := bind.WaitMined(context.Background(), client, tx2)
		require.NoError(t, err)
		require.Equal(t, receipt2.Status, uint64(1)) // Check transaction succeeded

		// Grab the first emitted log
		log := receipt2.Logs[0]

		// FIXME Check if we need to fill the nonce everytime
		// FIXME Check if we need to fill the gas price everytime
		transactor.Nonce = big.NewInt(int64(nonce + 2))
		gasPrice, err = client.SuggestGasPrice(ctx)
		require.NoError(t, err)
		transactor.GasPrice = gasPrice

		// Grab the block information for the block where the log was emitted
		block, err := client.BlockByHash(ctx, log.BlockHash)
		require.NoError(t, err)

		// Construct the expected payload to be verified
		msgPayload := make([]byte, 0)
		for _, topic := range log.Topics {
			msgPayload = append(msgPayload, topic.Bytes()...)
		}
		msgPayload = append(msgPayload, log.Data...)
		expectedHash := common.BytesToHash(crypto.Keccak256(msgPayload))

		fmt.Println("Validating message")

		// And validate the message
		tx3, err := instance.ValidateMessage(transactor, bindings.Identifier{
			Origin:      log.Address,
			BlockNumber: big.NewInt(int64(log.BlockNumber)),
			LogIndex:    big.NewInt(int64(log.Index)),
			Timestamp:   big.NewInt(int64(block.Time())),
			ChainId:     chain.ID(),
		}, expectedHash)
		require.NoError(t, err)

		// And wait for the transaction to mine successfully
		receipt3, err := bind.WaitMined(ctx, client, tx3)
		require.NoError(t, err)
		require.Equal(t, receipt3.Status, uint64(1)) // Check transaction succeeded
	},
		fundsValidator,
	)
}

// BELOW: refactoring in progress

// Helper struct to manage transaction creation and sending
type txManager struct {
	client *ethclient.Client
	auth   *bind.TransactOpts
	nonce  uint64
}

func newTxManager(ctx context.Context, client *ethclient.Client, wallet system.Wallet) (*txManager, error) {
	nonce, err := client.PendingNonceAt(ctx, wallet.Address())
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	auth := wallet.Transactor()
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)       // in wei
	auth.GasLimit = uint64(30000000) // in units
	auth.GasPrice = gasPrice

	return &txManager{
		client: client,
		auth:   auth,
		nonce:  nonce,
	}, nil
}

func (tm *txManager) nextTx(ctx context.Context) error {
	tm.nonce++
	tm.auth.Nonce = big.NewInt(int64(tm.nonce))
	gasPrice, err := tm.client.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}
	tm.auth.GasPrice = gasPrice
	return nil
}

func (tm *txManager) sendAndWait(ctx context.Context, tx interface{}) (*types.Receipt, error) {
	ethTx, ok := tx.(*types.Transaction)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}
	receipt, err := bind.WaitMined(ctx, tm.client, ethTx)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for transaction: %w", err)
	}
	if receipt.Status != uint64(1) {
		return nil, fmt.Errorf("transaction failed with status: %d", receipt.Status)
	}
	return receipt, nil
}

// Helper function to build a message identifier and compute its hash from a log entry
func buildMessageIdentifier(ctx context.Context, client *ethclient.Client, log *types.Log, chainID *big.Int) (bindings.Identifier, common.Hash, error) {
	block, err := client.BlockByHash(ctx, log.BlockHash)
	if err != nil {
		return bindings.Identifier{}, common.Hash{}, fmt.Errorf("failed to get block: %w", err)
	}

	// Build message payload and hash
	msgPayload := make([]byte, 0)
	for _, topic := range log.Topics {
		msgPayload = append(msgPayload, topic.Bytes()...)
	}
	msgPayload = append(msgPayload, log.Data...)

	identifier := bindings.Identifier{
		Origin:      log.Address,
		BlockNumber: big.NewInt(int64(log.BlockNumber)),
		LogIndex:    big.NewInt(int64(log.Index)),
		Timestamp:   big.NewInt(int64(block.Time())),
		ChainId:     chainID,
	}
	hash := common.BytesToHash(crypto.Keccak256(msgPayload))

	return identifier, hash, nil
}

// Helper functions for EventLogger contract interactions
type eventLogger struct {
	instance *bindings.EventLogger
	address  common.Address
	tm       *txManager
	client   *ethclient.Client
}

func deployEventLogger(ctx context.Context, tm *txManager) (*eventLogger, error) {
	address, tx, instance, err := bindings.DeployEventLogger(tm.auth, tm.client)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy EventLogger: %w", err)
	}

	if _, err := tm.sendAndWait(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to wait for deployment: %w", err)
	}

	return &eventLogger{
		instance: instance,
		address:  address,
		tm:       tm,
		client:   tm.client,
	}, nil
}

func (e *eventLogger) emitLog(ctx context.Context) (*types.Log, error) {
	if err := e.tm.nextTx(ctx); err != nil {
		return nil, fmt.Errorf("failed to prepare transaction: %w", err)
	}

	tx, err := e.instance.EmitLog(e.tm.auth, [][32]byte{}, []byte{})
	if err != nil {
		return nil, fmt.Errorf("failed to emit log: %w", err)
	}

	receipt, err := e.tm.sendAndWait(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for emit log: %w", err)
	}

	return receipt.Logs[0], nil
}

func (e *eventLogger) validateMessage(ctx context.Context, log *types.Log, chainID *big.Int) error {
	if err := e.tm.nextTx(ctx); err != nil {
		return fmt.Errorf("failed to prepare transaction: %w", err)
	}

	identifier, expectedHash, err := buildMessageIdentifier(ctx, e.client, log, chainID)
	if err != nil {
		return fmt.Errorf("failed to build message identifier: %w", err)
	}

	tx, err := e.instance.ValidateMessage(e.tm.auth, identifier, expectedHash)
	if err != nil {
		return fmt.Errorf("failed to validate message: %w", err)
	}

	if _, err := e.tm.sendAndWait(ctx, tx); err != nil {
		return fmt.Errorf("failed to wait for validate message: %w", err)
	}

	return nil
}

// The real test
func TestInteropSystemEventLogger(t *testing.T) {
	chainIdx := uint64(0)
	fundsNeeded := sdktypes.NewBalance(big.NewInt(1.0 * constants.ETH))

	walletGetter, fundsValidator := validators.AcquireL2WalletWithFunds(chainIdx, fundsNeeded)

	systest.InteropSystemTest(t, func(t systest.T, sys system.InteropSystem) {
		ctx := t.Context()
		chain := sys.L2(chainIdx)
		client, err := ethclient.Dial(chain.RPCURL())
		require.NoError(t, err)

		wallet := walletGetter(ctx)
		tm, err := newTxManager(ctx, client, wallet)
		require.NoError(t, err)

		// Deploy and setup contract
		logger, err := deployEventLogger(ctx, tm)
		require.NoError(t, err)
		t.Logf("Contract deployed at %s", logger.address.Hex())

		// Emit log
		log, err := logger.emitLog(ctx)
		require.NoError(t, err)

		// Validate message
		err = logger.validateMessage(ctx, log, chain.ID())
		require.NoError(t, err)
	},
		fundsValidator,
	)
}

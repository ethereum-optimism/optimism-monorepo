package isthmus

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	sdktypes "github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

// erc20BridgeTestScenario tests depositing an ERC20 token from L1 to L2 through the bridge
func erc20BridgeTestScenario(chainIdx uint64, walletGetter validators.WalletGetter) systest.SystemTestFunc {
	return func(t systest.T, sys system.System) {
		ctx := t.Context()

		// Get the L1 chain
		l1Chain, ok := sys.L1().(system.LowLevelChain)
		require.True(t, ok)

		// Get the L2 chain
		l2Chain, ok := sys.L2s()[chainIdx].(system.LowLevelChain)
		require.True(t, ok)

		logger := testlog.Logger(t, log.LevelInfo)
		logger.Info("Started ERC20 bridge test")

		// Get the user wallet
		user := walletGetter(ctx)

		// Connect to L1 and L2
		l1Client, err := ethclient.DialContext(ctx, l1Chain.RPCURL())
		require.NoError(t, err)
		t.Cleanup(func() { l1Client.Close() })

		l2Client, err := ethclient.DialContext(ctx, l2Chain.RPCURL())
		require.NoError(t, err)
		t.Cleanup(func() { l2Client.Close() })

		// Create transaction options for L1
		l1Opts, err := bind.NewKeyedTransactorWithChainID(user.PrivateKey(), (*big.Int)(l1Chain.ID()))
		require.NoError(t, err)

		// Create transaction options for L2
		l2Opts, err := bind.NewKeyedTransactorWithChainID(user.PrivateKey(), (*big.Int)(l2Chain.ID()))
		require.NoError(t, err)

		// Deploy a test ERC20 token on L1 (WETH)
		logger.Info("Deploying WETH token on L1")
		l1TokenAddress, tx, l1Token, err := bindings.DeployWETH(l1Opts, l1Client)
		require.NoError(t, err)

		// Wait for the token deployment transaction to be confirmed
		_, err = wait.ForReceiptOK(ctx, l1Client, tx.Hash())
		require.NoError(t, err, "Failed to deploy L1 token")
		logger.Info("Deployed L1 token", "address", l1TokenAddress)

		// Mint some tokens to the user (deposit ETH to get WETH)
		mintAmount := big.NewInt(params.Ether) // 1 ETH
		l1Opts.Value = mintAmount
		tx, err = l1Token.Deposit(l1Opts)
		require.NoError(t, err)
		_, err = wait.ForReceiptOK(ctx, l1Client, tx.Hash())
		require.NoError(t, err, "Failed to mint L1 tokens")
		l1Opts.Value = nil

		// Check the user's balance on L1
		l1Balance, err := l1Token.BalanceOf(&bind.CallOpts{}, user.Address())
		require.NoError(t, err)
		require.Equal(t, mintAmount, l1Balance, "User should have the minted tokens on L1")
		logger.Info("User has tokens on L1", "balance", l1Balance)

		// Create the corresponding L2 token using the OptimismMintableERC20Factory
		logger.Info("Creating L2 token via OptimismMintableERC20Factory")
		optimismMintableTokenFactory, err := bindings.NewOptimismMintableERC20Factory(predeploys.OptimismMintableERC20FactoryAddr, l2Client)
		require.NoError(t, err)

		// Create the L2 token
		l2TokenName := "L2 Test Token"
		l2TokenSymbol := "L2TEST"
		tx, err = optimismMintableTokenFactory.CreateOptimismMintableERC20(l2Opts, l1TokenAddress, l2TokenName, l2TokenSymbol)
		require.NoError(t, err)
		l2TokenReceipt, err := wait.ForReceiptOK(ctx, l2Client, tx.Hash())
		require.NoError(t, err, "Failed to create L2 token")

		// Extract the L2 token address from the event logs
		var l2TokenAddress common.Address
		for _, log := range l2TokenReceipt.Logs {
			createdEvent, err := optimismMintableTokenFactory.ParseOptimismMintableERC20Created(*log)
			if err == nil && createdEvent != nil {
				l2TokenAddress = createdEvent.LocalToken
				break
			}
		}
		require.NotEqual(t, common.Address{}, l2TokenAddress, "Failed to find L2 token address from events")
		logger.Info("Created L2 token", "address", l2TokenAddress)

		// Get the L2 token contract
		l2Token, err := bindings.NewOptimismMintableERC20(l2TokenAddress, l2Client)
		require.NoError(t, err)

		// Check initial L2 token balance (should be 0)
		initialL2Balance, err := l2Token.BalanceOf(&bind.CallOpts{}, user.Address())
		require.NoError(t, err)
		require.Equal(t, big.NewInt(0), initialL2Balance, "Initial L2 token balance should be 0")

		// Get the L1 standard bridge contract
		l1StandardBridgeAddress := common.HexToAddress("0x4200000000000000000000000000000000000010") // Standard address for L1StandardBridge
		l1StandardBridge, err := bindings.NewL1StandardBridge(l1StandardBridgeAddress, l1Client)
		require.NoError(t, err)

		// Approve the L1 bridge to spend tokens
		logger.Info("Approving L1 bridge to spend tokens")
		tx, err = l1Token.Approve(l1Opts, l1StandardBridgeAddress, mintAmount)
		require.NoError(t, err)
		_, err = wait.ForReceiptOK(ctx, l1Client, tx.Hash())
		require.NoError(t, err, "Failed to approve L1 bridge")

		// Amount to bridge
		bridgeAmount := big.NewInt(100000000000000000) // 0.1 token
		minGasLimit := uint32(200000)                  // Minimum gas limit for the L2 transaction

		// Bridge the tokens from L1 to L2
		logger.Info("Bridging tokens from L1 to L2", "amount", bridgeAmount)
		tx, err = l1StandardBridge.DepositERC20To(
			l1Opts,
			l1TokenAddress,
			l2TokenAddress,
			user.Address(),
			bridgeAmount,
			minGasLimit,
			[]byte{}, // No extra data
		)
		require.NoError(t, err)
		depositReceipt, err := wait.ForReceiptOK(ctx, l1Client, tx.Hash())
		require.NoError(t, err, "Failed to deposit tokens to L2")
		logger.Info("Deposit transaction confirmed on L1", "tx", tx.Hash().Hex())

		// Get the OptimismPortal contract to find the deposit event
		optimismPortalAddress := common.HexToAddress("0x4200000000000000000000000000000000000000") // Standard address for OptimismPortal
		optimismPortal, err := bindings.NewOptimismPortal(optimismPortalAddress, l1Client)
		require.NoError(t, err)

		// Find the TransactionDeposited event from the logs
		var depositFound bool
		for _, log := range depositReceipt.Logs {
			depositEvent, err := optimismPortal.ParseTransactionDeposited(*log)
			if err == nil && depositEvent != nil {
				logger.Info("Found deposit event", "from", depositEvent.From)
				depositFound = true
				break
			}
		}
		require.True(t, depositFound, "No deposit event found in transaction logs")

		// Wait for the deposit to be processed on L2
		// This may take some time as it depends on the L2 block time and the deposit processing
		logger.Info("Waiting for deposit to be processed on L2...")

		// Poll for the L2 balance to change
		err = wait.For(ctx, 30, func() (bool, error) {
			l2Balance, err := l2Token.BalanceOf(&bind.CallOpts{}, user.Address())
			if err != nil {
				return false, err
			}
			logger.Info("Current L2 balance", "balance", l2Balance)
			return l2Balance.Cmp(initialL2Balance) > 0, nil
		})
		require.NoError(t, err, "Timed out waiting for L2 balance to change")

		// Verify the final L2 balance
		finalL2Balance, err := l2Token.BalanceOf(&bind.CallOpts{}, user.Address())
		require.NoError(t, err)
		require.Equal(t, bridgeAmount, finalL2Balance, "L2 balance should match the bridged amount")
		logger.Info("Successfully verified tokens on L2", "balance", finalL2Balance)

		logger.Info("ERC20 bridge test completed successfully!")
	}
}

func TestERC20Bridge(t *testing.T) {
	chainIdx := uint64(0) // We'll use the first L2 chain for this test

	walletGetter, fundsValidator := validators.AcquireL2WalletWithFunds(
		chainIdx,
		sdktypes.NewBalance(big.NewInt(1.0*constants.ETH)),
	)

	systest.SystemTest(t,
		erc20BridgeTestScenario(chainIdx, walletGetter),
		fundsValidator,
	)
}

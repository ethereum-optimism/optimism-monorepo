package fees

import (
	"context"
	"errors"
	"log/slog"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

// TestFees verifies that L1/L2 fees are handled properly in different fork configurations
func TestFees(t *testing.T) {
	// Define which L2 chain we'll test
	chainIdx := uint64(0)

	// Get validators and getters for accessing the system and wallets
	lowLevelSystemGetter, lowLevelSystemValidator := validators.AcquireLowLevelSystem()
	walletGetter, walletValidator := validators.AcquireL2WalletWithFunds(chainIdx, types.NewBalance(big.NewInt(params.Ether)))

	// Run pre-regolith test
	forkGetter, forkValidator := validators.AcquireRequiresNotFork(chainIdx, rollup.Regolith)
	systest.SystemTest(t,
		feesTestScenario(lowLevelSystemGetter, walletGetter, chainIdx, forkGetter),
		lowLevelSystemValidator,
		walletValidator,
		forkValidator,
	)

	// Run regolith test
	forkGetter, forkValidator = validators.AcquireRequiresFork(chainIdx, rollup.Regolith)
	_, notForkValidator := validators.AcquireRequiresNotFork(chainIdx, rollup.Ecotone)
	systest.SystemTest(t,
		feesTestScenario(lowLevelSystemGetter, walletGetter, chainIdx, forkGetter),
		lowLevelSystemValidator,
		walletValidator,
		forkValidator,
		notForkValidator,
	)

	// Run ecotone test
	forkGetter, forkValidator = validators.AcquireRequiresFork(chainIdx, rollup.Ecotone)
	_, notForkValidator = validators.AcquireRequiresNotFork(chainIdx, rollup.Fjord)
	systest.SystemTest(t,
		feesTestScenario(lowLevelSystemGetter, walletGetter, chainIdx, forkGetter),
		lowLevelSystemValidator,
		walletValidator,
		forkValidator,
		notForkValidator,
	)

	// Run fjord test
	forkGetter, forkValidator = validators.AcquireRequiresFork(chainIdx, rollup.Fjord)
	systest.SystemTest(t,
		feesTestScenario(lowLevelSystemGetter, walletGetter, chainIdx, forkGetter),
		lowLevelSystemValidator,
		walletValidator,
		forkValidator,
	)
}

// stateGetterAdapter adapts the ethclient to implement the StateGetter interface
type stateGetterAdapter struct {
	ctx    context.Context
	t      systest.T
	client *ethclient.Client
}

// GetState implements the StateGetter interface
func (sga *stateGetterAdapter) GetState(addr common.Address, key common.Hash) common.Hash {
	var result common.Hash
	val, err := sga.client.StorageAt(sga.ctx, addr, key, nil)
	require.NoError(sga.t, err)
	copy(result[:], val)
	return result
}

// waitForTransaction polls for a transaction receipt until it is available or the context is canceled.
// It's a simpler version of the functionality in SimpleTxManager.
func waitForTransaction(ctx context.Context, client *ethclient.Client, hash common.Hash) (*gethTypes.Receipt, error) {
	ticker := time.NewTicker(500 * time.Millisecond) // Poll every 500ms
	defer ticker.Stop()

	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if receipt != nil && err == nil {
			return receipt, nil
		} else if err != nil && !errors.Is(err, ethereum.NotFound) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			// Continue polling
		}
	}
}

// feesTestScenario creates a test scenario for verifying fee calculations
func feesTestScenario(
	lowLevelSystemGetter validators.LowLevelSystemGetter,
	walletGetter validators.WalletGetter,
	chainIdx uint64,
	forkGetter validators.ForkConfigGetter,
) systest.SystemTestFunc {
	return func(t systest.T, sys system.System) {
		ctx := t.Context()

		// Get the low-level system and wallet
		llsys := lowLevelSystemGetter(ctx)
		wallet := walletGetter(ctx)
		fork := forkGetter(ctx)

		// Get the L2 client
		l2Chain := llsys.L2s()[chainIdx]
		l2Client, err := l2Chain.Client()
		require.NoError(t, err)

		// Get the L1 client
		l1Chain := llsys.L1()
		l1Client, err := l1Chain.Client()
		require.NoError(t, err)

		// TODO: Wait for first block after genesis
		// The genesis block has zero L1Block values and will throw off the GPO checks
		_, err = l2Client.HeaderByNumber(ctx, big.NewInt(1))
		require.NoError(t, err)

		// Get the genesis config
		chainConfig, err := l2Chain.ChainConfig()
		require.NoError(t, err)

		// Create state getter adapter for L1 cost function
		sga := &stateGetterAdapter{
			ctx:    ctx,
			t:      t,
			client: l2Client,
		}

		// Create L1 cost function
		l1CostFn := gethTypes.NewL1CostFunc(chainConfig, sga)

		// Get wallet private key and address
		fromAddr := wallet.Address()
		privateKey := wallet.PrivateKey()

		// Find gaspriceoracle contract
		gpoContract, err := bindings.NewGasPriceOracle(predeploys.GasPriceOracleAddr, l2Client)
		require.NoError(t, err)

		// Get wallet balance before test
		startBalance, err := l2Client.BalanceAt(ctx, fromAddr, big.NewInt(rpc.EarliestBlockNumber.Int64()))
		require.NoError(t, err)
		require.Greater(t, startBalance.Uint64(), big.NewInt(0).Uint64())

		// Get initial balances of fee recipients
		baseFeeRecipientStartBalance, err := l2Client.BalanceAt(ctx, predeploys.BaseFeeVaultAddr, big.NewInt(rpc.EarliestBlockNumber.Int64()))
		require.NoError(t, err)

		l1FeeRecipientStartBalance, err := l2Client.BalanceAt(ctx, predeploys.L1FeeVaultAddr, big.NewInt(rpc.EarliestBlockNumber.Int64()))
		require.NoError(t, err)

		sequencerFeeVaultStartBalance, err := l2Client.BalanceAt(ctx, predeploys.SequencerFeeVaultAddr, big.NewInt(rpc.EarliestBlockNumber.Int64()))
		require.NoError(t, err)

		genesisBlock, err := l2Client.BlockByNumber(ctx, big.NewInt(rpc.EarliestBlockNumber.Int64()))
		require.NoError(t, err)

		coinbaseStartBalance, err := l2Client.BalanceAt(ctx, genesisBlock.Coinbase(), big.NewInt(rpc.EarliestBlockNumber.Int64()))
		require.NoError(t, err)

		// Send a simple transfer from wallet to a test address
		transferAmount := big.NewInt(params.Ether / 10) // 0.1 ETH
		targetAddr := common.Address{0xff, 0xff}

		// Get suggested gas tip from the client instead of using a hardcoded value
		gasTip, err := l2Client.SuggestGasTipCap(ctx)
		require.NoError(t, err, "Failed to get suggested gas tip")

		// Estimate gas for the transaction instead of using a hardcoded value
		msg := ethereum.CallMsg{
			From:  fromAddr,
			To:    &targetAddr,
			Value: transferAmount,
		}
		gasLimit, err := l2Client.EstimateGas(ctx, msg)
		require.NoError(t, err, "Failed to estimate gas")

		// Create and sign transaction with the suggested values
		nonce, err := l2Client.PendingNonceAt(ctx, fromAddr)
		require.NoError(t, err)

		// Get latest header to get the base fee
		header, err := l2Client.HeaderByNumber(ctx, nil)
		require.NoError(t, err)

		// Calculate a reasonable gas fee cap based on the base fee
		// A common approach is to set fee cap to 2x the base fee + tip
		gasFeeCap := new(big.Int).Add(
			new(big.Int).Mul(header.BaseFee, big.NewInt(2)),
			gasTip,
		)

		txData := &gethTypes.DynamicFeeTx{
			ChainID:   l2Chain.ID(),
			Nonce:     nonce,
			GasTipCap: gasTip,
			GasFeeCap: gasFeeCap,
			Gas:       gasLimit,
			To:        &targetAddr,
			Value:     transferAmount,
			Data:      nil,
		}

		// Sign transaction
		tx := gethTypes.NewTx(txData)
		signedTx, err := gethTypes.SignTx(tx, gethTypes.LatestSignerForChainID(l2Chain.ID()), privateKey)
		require.NoError(t, err)

		// Send transaction
		err = l2Client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)

		// Wait for transaction receipt with timeout
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		receipt, err := waitForTransaction(ctx, l2Client, signedTx.Hash())
		require.NoError(t, err, "Failed to wait for transaction receipt")
		require.NotNil(t, receipt)
		require.Equal(t, gethTypes.ReceiptStatusSuccessful, receipt.Status)

		// Get block header where transaction was included
		header, err = l2Client.HeaderByNumber(ctx, receipt.BlockNumber)
		require.NoError(t, err)

		// Get final balances after transaction
		coinbaseEndBalance, err := l2Client.BalanceAt(ctx, header.Coinbase, header.Number)
		require.NoError(t, err)

		endBalance, err := l2Client.BalanceAt(ctx, fromAddr, header.Number)
		require.NoError(t, err)

		baseFeeRecipientEndBalance, err := l2Client.BalanceAt(ctx, predeploys.BaseFeeVaultAddr, header.Number)
		require.NoError(t, err)

		l1Header, err := l1Client.HeaderByNumber(ctx, nil)
		require.NoError(t, err)

		l1FeeRecipientEndBalance, err := l2Client.BalanceAt(ctx, predeploys.L1FeeVaultAddr, header.Number)
		require.NoError(t, err)

		sequencerFeeVaultEndBalance, err := l2Client.BalanceAt(ctx, predeploys.SequencerFeeVaultAddr, header.Number)
		require.NoError(t, err)

		// Calculate differences in balances
		baseFeeRecipientDiff := new(big.Int).Sub(baseFeeRecipientEndBalance, baseFeeRecipientStartBalance)
		l1FeeRecipientDiff := new(big.Int).Sub(l1FeeRecipientEndBalance, l1FeeRecipientStartBalance)
		sequencerFeeVaultDiff := new(big.Int).Sub(sequencerFeeVaultEndBalance, sequencerFeeVaultStartBalance)
		coinbaseDiff := new(big.Int).Sub(coinbaseEndBalance, coinbaseStartBalance)

		// Verify L2 fee
		l2Fee := new(big.Int).Mul(gasTip, new(big.Int).SetUint64(receipt.GasUsed))
		require.Equal(t, sequencerFeeVaultDiff, coinbaseDiff, "coinbase is always sequencer fee vault")
		require.Equal(t, l2Fee, coinbaseDiff, "l2 fee mismatch")
		require.Equal(t, l2Fee, sequencerFeeVaultDiff)

		// Verify base fee
		baseFee := new(big.Int).Mul(header.BaseFee, new(big.Int).SetUint64(receipt.GasUsed))
		require.Equal(t, baseFee, baseFeeRecipientDiff, "base fee mismatch")

		// Verify L1 fee
		txBytes, err := signedTx.MarshalBinary()
		require.NoError(t, err)

		// Calculate L1 fee based on transaction data and blocktime
		l1Fee := l1CostFn(tx.RollupCostData(), header.Time)
		require.Equal(t, l1Fee, l1FeeRecipientDiff, "L1 fee mismatch")

		// Verify GPO matches expected state
		gpoEcotone, err := gpoContract.IsEcotone(nil)
		require.NoError(t, err)

		ecotoneEnabled := fork.IsForkActivated(rollup.Ecotone)
		require.Equal(t, ecotoneEnabled, gpoEcotone, "GPO and chain must have same ecotone view")

		gpoFjord, err := gpoContract.IsFjord(nil)
		require.NoError(t, err)
		fjordEnabled := fork.IsForkActivated(rollup.Fjord)
		require.Equal(t, fjordEnabled, gpoFjord, "GPO and chain must have same fjord view")

		// Verify gas price oracle L1 fee calculation
		gpoL1Fee, err := gpoContract.GetL1Fee(&bind.CallOpts{}, txBytes)
		require.NoError(t, err)

		adjustedGPOFee := gpoL1Fee
		if fjordEnabled {
			// The fjord calculations are handled differently and may be bounded by the minimum value
			// This is a simplification as the full test adapts more precisely
		} else if regolithEnabled := fork.IsForkActivated(rollup.Regolith); regolithEnabled && !ecotoneEnabled {
			// For post-regolith (but pre-ecotone), adjust the GPO fee to account for signature overhead
			artificialGPOOverhead := big.NewInt(68 * 16) // 68 bytes to cover signature and RLP data
			l1BaseFee := l1Header.BaseFee
			adjustedGPOFee = new(big.Int).Sub(gpoL1Fee, new(big.Int).Mul(artificialGPOOverhead, l1BaseFee))
		}
		require.Equal(t, l1Fee, adjustedGPOFee, "GPO reports L1 fee mismatch")

		// Verify receipt L1 fee
		require.Equal(t, receipt.L1Fee, l1Fee, "l1 fee in receipt is correct")

		// Calculate total fee and verify wallet balance difference
		totalFeeRecipient := new(big.Int).Add(baseFeeRecipientDiff, sequencerFeeVaultDiff)
		totalFee := new(big.Int).Add(totalFeeRecipient, l1FeeRecipientDiff)

		balanceDiff := new(big.Int).Sub(startBalance, endBalance)
		balanceDiff.Sub(balanceDiff, transferAmount)
		require.Equal(t, balanceDiff, totalFee, "balances should add up")
	}
}

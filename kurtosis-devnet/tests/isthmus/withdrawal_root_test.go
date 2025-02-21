package isthmus

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/lmittmann/w3"
)

func withdrawalRootTestScenario(chainIdx uint64, walletGetter validators.WalletGetter) systest.SystemTestFunc {
	return func(t systest.T, sys system.System) {
		ctx := t.Context()

		chain, ok := sys.L2s()[0].(system.LowLevelChain)
		require.True(t, ok)
		chain.Client()

		logger := testlog.Logger(t, log.LevelInfo)
		logger.Info("Started test")

		user := walletGetter(ctx)

		// Sad eth clients
		rpcCl, err := client.NewRPC(ctx, logger, chain.RPCURL())
		require.NoError(t, err)
		t.Cleanup(rpcCl.Close)
		ethCl, err := sources.NewEthClient(rpcCl, logger, nil, sources.DefaultEthClientConfig(10))
		require.NoError(t, err)

		gethCl, err := ethclient.DialContext(ctx, chain.RPCURL())
		require.NoError(t, err)
		t.Cleanup(gethCl.Close)

		// Determine pre-state
		preBlock, err := gethCl.BlockByNumber(ctx, nil)
		require.NoError(t, err)
		logger.Info("Got pre-state block", "hash", preBlock.Hash(), "number", preBlock.Number())

		preBlockHash := preBlock.Hash()
		preProof, err := ethCl.GetProof(ctx, predeploys.L2ToL1MessagePasserAddr, nil, preBlockHash.String())
		require.NoError(t, err)
		preWithdrawalsRoot := preProof.StorageHash

		logger.Info("Got pre proof", "storage hash", preWithdrawalsRoot)

		// check isthmus withdrawals-root in the block matches the state
		gotPre := preBlock.WithdrawalsRoot()
		require.NotNil(t, gotPre)
		require.Equal(t, preWithdrawalsRoot, *gotPre, "withdrawals root in block is what we expect")

		chainID := (*big.Int)(chain.ID())
		signer := gtypes.LatestSignerForChainID(chainID)
		priv := user.PrivateKey()
		require.NoError(t, err)

		// construct call input, ugly but no bindings...
		funcInitiateWithdrawal := w3.MustNewFunc(`initiateWithdrawal(address, uint256, bytes memory)`, "")
		args, err := funcInitiateWithdrawal.EncodeArgs(
			common.Address{},
			big.NewInt(1_000_000),
			[]byte{},
		)
		require.NoError(t, err)

		nonce, err := gethCl.PendingNonceAt(ctx, user.Address())
		require.NoError(t, err)

		// sign a tx to trigger a withdrawal (no ETH value, just a message), submit it
		// Get current base fee to ensure transaction gets included
		header, err := gethCl.HeaderByNumber(ctx, nil)
		require.NoError(t, err)
		baseFee := header.BaseFee

		// Set gas tip to 2x current base fee and total fee cap to 3x base fee to ensure inclusion
		gasTipCap := new(big.Int).Mul(baseFee, big.NewInt(2))
		gasFeeCap := new(big.Int).Mul(baseFee, big.NewInt(3))

		tx, err := gtypes.SignNewTx(priv, signer, &gtypes.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			Gas:       21624,
			To:        &predeploys.L2ToL1MessagePasserAddr,
			Value:     big.NewInt(0),
			Data:      args,
		})
		require.NoError(t, err, "sign tx")

		// Try to simulate the transaction first to check for errors
		_, err = gethCl.CallContract(ctx, ethereum.CallMsg{
			From:      user.Address(),
			To:        tx.To(),
			Gas:       tx.Gas(),
			GasPrice:  tx.GasFeeCap(),
			GasTipCap: tx.GasTipCap(),
			Value:     tx.Value(),
			Data:      tx.Data(),
		}, nil)
		if err != nil {
			logger.Error("Transaction simulation failed", "err", err)
		}

		err = gethCl.SendTransaction(ctx, tx)
		require.NoError(t, err, "send tx")

		// Find when the withdrawal was included
		rec, err := wait.ForReceipt(ctx, gethCl, tx.Hash(), gtypes.ReceiptStatusSuccessful)
		if err != nil {
			logger.Error("Transaction was not included", "err", err)
		}
		require.NoError(t, err)

		// Load the storage at this particular block
		postBlockHash := rec.BlockHash
		postProof, err := ethCl.GetProof(ctx, predeploys.L2ToL1MessagePasserAddr, nil, postBlockHash.String())
		require.NoError(t, err, "Error getting L2ToL1MessagePasser contract proof")
		postWithdrawalsRoot := postProof.StorageHash

		// Check that the withdrawals-root changed
		require.NotEqual(t, preWithdrawalsRoot, postWithdrawalsRoot, "withdrawals storage root changes")

		postBlock, err := gethCl.BlockByHash(ctx, postBlockHash)
		require.NoError(t, err)
		logger.Info("Got post-state block", "hash", postBlock.Hash(), "number", postBlock.Number())

		gotPost := postBlock.WithdrawalsRoot()
		require.NotNil(t, gotPost)
		require.Equal(t, postWithdrawalsRoot, *gotPost, "block contains new withdrawals root")

		logger.Info("Done!")
	}
}

func TestWithdrawalsRoot(t *testing.T) {
	chainIdx := uint64(0) // We'll use the first L2 chain for this test

	walletGetter, fundsValidator := validators.AcquireL2WalletWithFunds(
		chainIdx,
		types.NewBalance(big.NewInt(1.0*constants.ETH)),
	)

	systest.SystemTest(t,
		withdrawalRootTestScenario(chainIdx, walletGetter),
		fundsValidator,
	)
}

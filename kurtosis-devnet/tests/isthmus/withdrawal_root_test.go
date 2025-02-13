package isthmus

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	gtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/solabi"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func withdrawalRootTestScenario(chainIdx uint64, userSentinel interface{}) systest.SystemTestFunc {
	return func(t systest.T, sys system.System) {
		ctx := t.Context()

		chain := sys.L2(chainIdx)
		logger := testlog.Logger(t, log.LevelInfo)
		logger.Info("Started test")

		user := ctx.Value(userSentinel).(types.Wallet)

		// Sad eth clients
		rpcCl, err := client.NewRPC(ctx, logger, chain.RPCURL())
		require.NoError(t, err)
		t.Cleanup(rpcCl.Close)
		ethCl, err := sources.NewEthClient(rpcCl, logger, nil, &sources.EthClientConfig{
			MaxConcurrentRequests: 10,
			MaxRequestsPerBatch:   int(10),
			RPCProviderKind:       sources.RPCKindAny,
		})
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

		// check isthmus withdrawals-root in the block matches the state
		gotPre := preBlock.WithdrawalsRoot()
		require.NotNil(t, gotPre)
		require.Equal(t, preWithdrawalsRoot, *gotPre, "withdrawals root in block is what we expect")

		chainID := (*big.Int)(chain.ID())
		signer := gtypes.LatestSignerForChainID(chainID)
		priv, err := crypto.HexToECDSA(user.PrivateKey())
		require.NoError(t, err)

		// construct call input, ugly but no bindings...
		var input bytes.Buffer
		require.NoError(t, solabi.WriteSignature(&input, []byte("initiateWithdrawal(address,uint256,bytes)")[:4])) // selector
		require.NoError(t, solabi.WriteAddress(&input, common.Address{}))
		require.NoError(t, solabi.WriteUint256(&input, big.NewInt(1000_000)))
		require.NoError(t, solabi.WriteUint256(&input, big.NewInt(4+20+32+32))) // calldata offset to length data
		require.NoError(t, solabi.WriteUint256(&input, big.NewInt(0)))          // length

		// sign a tx to trigger a withdrawal (no ETH value, just a message), submit it
		tx, err := gtypes.SignNewTx(priv, signer, &gtypes.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     0,
			GasTipCap: big.NewInt(10),
			GasFeeCap: big.NewInt(200),
			Gas:       params.TxGas,
			To:        &predeploys.L2ToL1MessagePasserAddr,
			Value:     big.NewInt(0),
			Data:      input.Bytes(),
		})
		require.NoError(t, err, "sign tx")
		require.NoError(t, gethCl.SendTransaction(ctx, tx))

		// Find when the withdrawal was included
		rec, err := wait.ForReceipt(ctx, gethCl, tx.Hash(), gtypes.ReceiptStatusSuccessful)
		require.NoError(t, err)

		// Load the storage at this particular block
		postBlockHash := rec.BlockHash
		postProof, err := ethCl.GetProof(ctx, predeploys.L2ToL1MessagePasserAddr, nil, postBlockHash.String())
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
	chainIdx := uint64(0)         // We'll use the first L2 chain for this test
	testUserMarker := &struct{}{} // Sentinel for the user context value

	systest.SystemTest(t,
		withdrawalRootTestScenario(chainIdx, testUserMarker),
		walletFundsValidator(chainIdx, types.NewBalance(big.NewInt(1.0*constants.ETH)), testUserMarker),
	)
}

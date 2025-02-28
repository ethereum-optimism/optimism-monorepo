package proofs_test

import (
	"context"
	"math/big"
	"testing"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestIsthmusExcludedPredeploys(gt *testing.T) {
	// Ensures that if EIP-6110, EIP-7251, or EIP-7002 predeploys are deployed manually after the fork,
	// Isthmus block processing still works correctly. Also ensures that if requests are sent to these
	// contracts, they are not processed and do not show up in the block body or requests hash.

	allocs := *actionsHelpers.DefaultAlloc
	allocs.L2Alloc = make(map[common.Address]types.Account)

	// Deploy EIP-7251 and EIP-7002 contracts
	allocs.L2Alloc[params.WithdrawalQueueAddress] = types.Account{ // EIP-7002
		Code:    params.WithdrawalQueueCode,
		Nonce:   1,
		Balance: new(big.Int),
	}
	allocs.L2Alloc[params.ConsolidationQueueAddress] = types.Account{ // EIP-7251
		Code:    params.ConsolidationQueueCode,
		Nonce:   1,
		Balance: new(big.Int),
	}

	t := actionsHelpers.NewDefaultTesting(gt)
	testCfg := &helpers.TestCfg[interface{}]{
		Hardfork: helpers.Isthmus,
		Allocs:   &allocs,
	}

	tp := helpers.NewTestParams()
	env := helpers.NewL2FaultProofEnv(t, testCfg, tp, helpers.NewBatcherCfg())

	dp := e2eutils.MakeDeployParams(t, actionsHelpers.DefaultRollupTestParams())

	engine := env.Engine
	sequencer := env.Sequencer

	ethCl := engine.EthClient()
	signer := types.NewPragueSigner(new(big.Int).SetUint64(dp.DeployConfig.L2ChainID))

	sequencer.ActL2StartBlock(t)

	ret, err := ethCl.CallContract(context.Background(), ethereum.CallMsg{
		To:   &params.WithdrawalQueueAddress,
		Data: []byte{},
	}, nil)

	require.NoError(t, err)
	fee := new(uint256.Int).SetBytes(ret)

	// Send a transaction to the EIP-7251 contract
	txdata := &types.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(dp.DeployConfig.L2ChainID),
		Nonce:     0,
		To:        &params.WithdrawalQueueAddress,
		Gas:       500000,
		Data:      make([]byte, 56),
		Value:     fee.ToBig(),
		GasFeeCap: new(big.Int).SetUint64(5000000000),
		GasTipCap: new(big.Int).SetUint64(2),
	}
	tx := types.MustSignNewTx(dp.Secrets.Alice, signer, txdata)

	err = ethCl.SendTransaction(t.Ctx(), tx)
	require.NoError(gt, err, "failed to send withdrawal request tx")

	_, err = engine.EngineApi.IncludeTx(tx, dp.Addresses.Alice)
	require.NoError(gt, err, "failed to include tx")

	ret, err = ethCl.CallContract(context.Background(), ethereum.CallMsg{
		To:   &params.ConsolidationQueueAddress,
		Data: []byte{},
	}, nil)

	require.NoError(t, err)
	fee = new(uint256.Int).SetBytes(ret)

	// Send a transaction to the EIP-7251 contract
	txdata = &types.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(dp.DeployConfig.L2ChainID),
		Nonce:     1,
		To:        &params.ConsolidationQueueAddress,
		Gas:       500000,
		Data:      make([]byte, 96),
		Value:     fee.ToBig(),
		GasFeeCap: new(big.Int).SetUint64(5000000000),
		GasTipCap: new(big.Int).SetUint64(2),
	}
	tx = types.MustSignNewTx(dp.Secrets.Alice, signer, txdata)

	err = ethCl.SendTransaction(t.Ctx(), tx)
	require.NoError(gt, err, "failed to send consolidation queue request tx")

	_, err = engine.EngineApi.IncludeTx(tx, dp.Addresses.Alice)
	require.NoError(gt, err, "failed to include tx")

	sequencer.ActL2EndBlock(t)

	// ensure requests hash is still empty
	latestBlock, err := ethCl.BlockByNumber(t.Ctx(), nil)
	require.NoError(t, err, "error fetching latest block")
	require.Equal(t, types.EmptyRequestsHash, *latestBlock.RequestsHash())

	// get receipt
	receipt, err := ethCl.TransactionReceipt(context.Background(), tx.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "transaction must pass")

	env.RunFaultProofProgram(t, latestBlock.NumberU64()-1, func(t actionsHelpers.Testing, err error) {
		require.NoError(t, err, "no error expected running FP program")
	})
}

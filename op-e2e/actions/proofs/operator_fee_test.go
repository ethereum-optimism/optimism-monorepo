package proofs

import (
	"context"
	"math/big"
	"testing"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const OperatorFeeScalar = uint32(20000)
const OperatorFeeConstant = uint64(500)

func Test_Operator_Fee_Constistency(gt *testing.T) {

	runIsthmusDerivationTest := func(gt *testing.T, testCfg *helpers.TestCfg[any]) {
		t := actionsHelpers.NewDefaultTesting(gt)

		env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())

		t.Logf("L2 Genesis Time: %d, IsthmusTime: %d ", env.Sequencer.RollupCfg.Genesis.L2Time, *env.Sequencer.RollupCfg.IsthmusTime)

		sysCfgContract, err := bindings.NewSystemConfig(env.Sd.RollupCfg.L1SystemConfigAddress, env.Miner.EthClient())
		require.NoError(t, err)

		sysCfgOwner, err := bind.NewKeyedTransactorWithChainID(env.Dp.Secrets.Deployer, env.Sd.RollupCfg.L1ChainID)
		require.NoError(t, err)

		// Update the operator fee parameters
		sysCfgContract.SetOperatorFeeScalars(sysCfgOwner, OperatorFeeScalar, OperatorFeeConstant)

		env.Miner.ActL1StartBlock(12)(t)
		env.Miner.ActL1IncludeTx(env.Dp.Addresses.Deployer)(t)
		env.Miner.ActL1EndBlock(t)

		// sequence L2 blocks, and submit with new batcher
		env.Sequencer.ActL1HeadSignal(t)
		env.Sequencer.ActBuildToL1Head(t)
		env.Batcher.ActSubmitAll(t)
		env.Miner.ActL1StartBlock(12)(t)
		env.Miner.ActL1EndBlock(t)

		aliceInitialBalance, err := env.Engine.EthClient().BalanceAt(context.Background(), env.Alice.Address(), nil)
		require.NoError(t, err)

		env.Sequencer.ActL2StartBlock(t)
		// Send an L2 tx
		env.Alice.L2.ActResetTxOpts(t)
		env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)
		env.Alice.L2.ActMakeTx(t)
		env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
		env.Sequencer.ActL2EndBlock(t)

		receipt := env.Alice.L2.LastTxReceipt(t)

		// Check that the operator fee was applied
		require.Equal(t, OperatorFeeScalar, uint32(*receipt.OperatorFeeScalar))
		require.Equal(t, OperatorFeeConstant, *receipt.OperatorFeeConstant)

		l1FeeVaultBalance, err := env.Engine.EthClient().BalanceAt(context.Background(), predeploys.L1FeeVaultAddr, nil)
		require.NoError(t, err)

		OperatorFeeVaultBalance, err := env.Engine.EthClient().BalanceAt(context.Background(), predeploys.OperatorFeeVaultAddr, nil)
		require.NoError(t, err)

		aliceFinalBalance, err := env.Engine.EthClient().BalanceAt(context.Background(), env.Alice.Address(), nil)
		require.NoError(t, err)

		// Check that the operator fee sent to the vault is correct
		require.Equal(t,
			new(big.Int).Add(
				new(big.Int).Div(
					new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), new(big.Int).SetUint64(uint64(OperatorFeeScalar))),
					new(big.Int).SetUint64(1e6),
				),
				new(big.Int).SetUint64(OperatorFeeConstant),
			),
			OperatorFeeVaultBalance,
		)

		// Check that no Ether has been minted or burned
		require.Equal(t,
			aliceInitialBalance,
			new(big.Int).Add(
				aliceFinalBalance,
				new(big.Int).Add(
					new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), receipt.EffectiveGasPrice),
					new(big.Int).Add(l1FeeVaultBalance, OperatorFeeVaultBalance),
				),
			),
		)

		l2SafeHead := env.Sequencer.L2Safe()

		env.RunFaultProofProgram(t, l2SafeHead.Number, testCfg.CheckResult, testCfg.InputParams...)
	}

	matrix := helpers.NewMatrix[any]()
	defer matrix.Run(gt)

	matrix.AddTestCase(
		"HonestClaim-OperatorFeeConstistency",
		nil,
		helpers.NewForkMatrix(helpers.Isthmus),
		runIsthmusDerivationTest,
		helpers.ExpectNoError(),
	)

	matrix.AddTestCase(
		"JunkClaim-OperatorFeeConstistency",
		nil,
		helpers.NewForkMatrix(helpers.Isthmus),
		runIsthmusDerivationTest,
		helpers.ExpectError(claim.ErrClaimNotValid),
		helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
	)
}

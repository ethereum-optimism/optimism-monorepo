package proofs

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	hostcommon "github.com/ethereum-optimism/optimism/op-program/host/common"
	hostconfig "github.com/ethereum-optimism/optimism/op-program/host/config"
	"github.com/ethereum-optimism/optimism/op-program/host/kvstore"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func Test_OPProgramAction_Precompiles(gt *testing.T) {
	tests := []precompileTestCase{
		{
			name:        "ecrecover",
			address:     common.BytesToAddress([]byte{0x01}),
			input:       common.FromHex("18c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c000000000000000000000000000000000000000000000000000000000000001c73b1693892219d736caba55bdb67216e485557ea6b6af75f37096c9aa6a5a75feeb940b1d03b21e36b0e47e79769f095fe2ab855bd91e3a38756b7d75a9c4549"),
			accelerated: true,
		},
		{
			name:    "sha256",
			address: common.BytesToAddress([]byte{0x02}),
			input:   common.FromHex("68656c6c6f20776f726c64"),
		},
		{
			name:    "ripemd160",
			address: common.BytesToAddress([]byte{0x03}),
			input:   common.FromHex("68656c6c6f20776f726c64"),
		},
		{
			name:        "bn256Pairing",
			address:     common.BytesToAddress([]byte{0x08}),
			input:       common.FromHex("1c76476f4def4bb94541d57ebba1193381ffa7aa76ada664dd31c16024c43f593034dd2920f673e204fee2811c678745fc819b55d3e9d294e45c9b03a76aef41209dd15ebff5d46c4bd888e51a93cf99a7329636c63514396b4a452003a35bf704bf11ca01483bfa8b34b43561848d28905960114c8ac04049af4b6315a416782bb8324af6cfc93537a2ad1a445cfd0ca2a71acd7ac41fadbf933c2a51be344d120a2a4cf30c1bf9845f20c6fe39e07ea2cce61f0c9bb048165fe5e4de877550111e129f1cf1097710d41c4ac70fcdfa5ba2023c6ff1cbeac322de49d1b6df7c2032c61a830e3c17286de9462bf242fca2883585b93870a73853face6a6bf411198e9393920d483a7260bfb731fb5d25f1aa493335a9e71297e485b7aef312c21800deef121f1e76426a00665e5c4479674322d4f75edadd46debd5cd992f6ed090689d0585ff075ec9e99ad690c3395bc4b313370b38ef355acdadcd122975b12c85ea5db8c6deb4aab71808dcb408fe3d1e7690c43d37b4ce6cc0166fa7daa"),
			accelerated: true,
		},
		{
			name:    "blake2F",
			address: common.BytesToAddress([]byte{0x09}),
			input:   common.FromHex("0000000048c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b61626300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000001"),
		},
		{
			name:        "kzgPointEvaluation",
			address:     common.BytesToAddress([]byte{0x0a}),
			input:       common.FromHex("01e798154708fe7789429634053cbf9f99b619f9f084048927333fce637f549b564c0a11a0f704f4fc3e8acfe0f8245f0ad1347b378fbf96e206da11a5d3630624d25032e67a7e6a4910df5834b8fe70e6bcfeeac0352434196bdf4b2485d5a18f59a8d2a1a625a17f3fea0fe5eb8c896db3764f3185481bc22f91b4aaffcca25f26936857bc3a7c2539ea8ec3a952b7873033e038326e87ed3e1276fd140253fa08e9fc25fb2d9a98527fc22a2c9612fbeafdad446cbc7bcdbdcd780af2c16a"),
			accelerated: true,
		},
	}

	for _, test := range tests {
		gt.Run(test.name, func(t *testing.T) {
			runPrecompileTest(t, test)
		})
	}
}

type precompileTestCase struct {
	name        string
	address     common.Address
	input       []byte
	accelerated bool
}

func runPrecompileTest(gt *testing.T, testCase precompileTestCase) {
	testCfg := &helpers.TestCfg[any]{
		Hardfork:    helpers.LatestFork,
		CheckResult: helpers.ExpectNoError(),
	}
	t := actionsHelpers.NewDefaultTesting(gt)
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())

	// Build a block on L2 with 1 tx.
	env.Alice.L2.ActResetTxOpts(t)
	env.Alice.L2.ActSetTxToAddr(&testCase.address)(t)
	env.Alice.L2.ActSetTxCalldata(testCase.input)(t)
	env.Alice.L2.ActMakeTx(t)

	env.Sequencer.ActL2StartBlock(t)
	env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
	env.Sequencer.ActL2EndBlock(t)
	env.Alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)

	// Instruct the batcher to submit the block to L1, and include the transaction.
	env.Batcher.ActSubmitAll(t)
	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)

	// Finalize the block with the batch on L1.
	env.Miner.ActL1SafeNext(t)
	env.Miner.ActL1FinalizeNext(t)

	// Instruct the sequencer to derive the L2 chain from the data on L1 that the batcher just posted.
	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	l1Head := env.Miner.L1Chain().CurrentBlock()
	l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()

	// Ensure there is only 1 block on L1.
	require.Equal(t, uint64(1), l1Head.Number.Uint64())
	// Ensure the block is marked as safe before we attempt to fault prove it.
	require.Equal(t, uint64(1), l2SafeHead.Number.Uint64())

	defaultParam := helpers.WithPreInteropDefaults(t, l2SafeHead.Number.Uint64(), env.Sequencer.L2Verifier, env.Engine)
	fixtureInputParams := []helpers.FixtureInputParam{defaultParam, helpers.WithL1Head(l1Head.Hash())}
	var fixtureInputs helpers.FixtureInputs
	for _, apply := range fixtureInputParams {
		apply(&fixtureInputs)
	}
	programCfg := helpers.NewOpProgramCfg(&fixtureInputs)
	// Create an external in-memory kv store so we can inspect the precompile results.
	kv := kvstore.NewMemKV()
	withInProcessPrefetcher := hostcommon.WithPrefetcher(func(ctx context.Context, logger log.Logger, _kv kvstore.KV, cfg *hostconfig.Config) (hostcommon.Prefetcher, error) {
		return helpers.CreateInprocessPrefetcher(t, ctx, logger, env.Miner, kv, cfg, &fixtureInputs)
	})
	ctx, cancel := context.WithTimeout(t.Ctx(), 2*time.Minute)
	defer cancel()
	require.NoError(t, hostcommon.FaultProofProgram(ctx, testlog.Logger(t, log.LevelDebug).New("role", "program"), programCfg, withInProcessPrefetcher))

	rules := env.Engine.L2Chain().Config().Rules(l2SafeHead.Number, true, l2SafeHead.Time)
	precompile := vm.ActivePrecompiledContracts(rules)[testCase.address]
	gas := precompile.RequiredGas(testCase.input)
	precompileKey := createPrecompileKey(testCase.address, testCase.input, gas)
	// If accelerated, make sure that the precompile was fetched from the host.
	if testCase.accelerated {
		programResult, err := kv.Get(precompileKey)
		require.NoError(t, err)

		precompileSuccess := [1]byte{1}
		expected, err := precompile.Run(testCase.input)
		expected = append(precompileSuccess[:], expected...)
		require.NoError(t, err)
		require.EqualValues(t, expected, programResult)
	} else {
		_, err := kv.Get(precompileKey)
		require.ErrorIs(t, kvstore.ErrNotFound, err)
	}
}

func createPrecompileKey(precompileAddress common.Address, input []byte, gas uint64) common.Hash {
	hintBytes := append(precompileAddress.Bytes(), binary.BigEndian.AppendUint64(nil, gas)...)
	return preimage.PrecompileKey(crypto.Keccak256Hash(append(hintBytes, input...))).PreimageKey()
}

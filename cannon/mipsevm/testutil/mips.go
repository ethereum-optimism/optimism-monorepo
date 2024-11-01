package testutil

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	preimage "github.com/ethereum-optimism/optimism/op-preimage"
)

// maxStepGas should be less than the L1 gas limit
const maxStepGas = 20_000_000

type MIPSEVM struct {
	sender      vm.AccountRef
	startingGas uint64
	env         *vm.EVM
	evmState    *state.StateDB
	addrs       *Addresses
	localOracle mipsevm.PreimageOracle
	artifacts   *Artifacts
	// Track step execution for logging purposes
	lastStep                uint64
	lastStepInput           []byte
	lastPreimageOracleInput []byte
}

func newMIPSEVM(contracts *ContractMetadata, opts ...evmOption) *MIPSEVM {
	env, evmState := NewEVMEnv(contracts)
	sender := vm.AccountRef{0x13, 0x37}
	startingGas := uint64(maxStepGas)
	evm := &MIPSEVM{sender, startingGas, env, evmState, contracts.Addresses, nil, contracts.Artifacts, math.MaxUint64, nil, nil}
	for _, opt := range opts {
		opt(evm)
	}
	return evm
}

type evmOption func(c *MIPSEVM)

func WithSourceMapTracer(t *testing.T, ver MipsVersion) evmOption {
	return func(evm *MIPSEVM) {
		evm.SetSourceMapTracer(t, ver)
	}
}

func WithTracingHooks(tracer *tracing.Hooks) evmOption {
	return func(evm *MIPSEVM) {
		evm.SetTracer(tracer)
	}
}

func WithLocalOracle(oracle mipsevm.PreimageOracle) evmOption {
	return func(evm *MIPSEVM) {
		evm.SetLocalOracle(oracle)
	}
}

func (m *MIPSEVM) SetTracer(tracer *tracing.Hooks) {
	m.env.Config.Tracer = tracer
}

func (m *MIPSEVM) SetLocalOracle(oracle mipsevm.PreimageOracle) {
	m.localOracle = oracle
}

func (m *MIPSEVM) SetSourceMapTracer(t *testing.T, version MipsVersion) {
	m.env.Config.Tracer = SourceMapTracer(t, version, m.artifacts.MIPS, m.artifacts.Oracle, m.addrs)
}

// Step is a pure function that computes the poststate from the VM state encoded in the StepWitness.
func (m *MIPSEVM) Step(t *testing.T, stepWitness *mipsevm.StepWitness, step uint64, stateHashFn mipsevm.HashFn) []byte {
	m.lastStep = step
	m.lastStepInput = nil
	m.lastPreimageOracleInput = nil

	// we take a snapshot so we can clean up the state, and isolate the logs of this instruction run.
	snap := m.env.StateDB.Snapshot()

	if stepWitness.HasPreimage() {
		t.Logf("reading preimage key %x at offset %d", stepWitness.PreimageKey, stepWitness.PreimageOffset)
		poInput, err := m.encodePreimageOracleInput(t, stepWitness.PreimageKey, stepWitness.PreimageValue, stepWitness.PreimageOffset, mipsevm.LocalContext{})
		m.lastPreimageOracleInput = poInput
		require.NoError(t, err, "encode preimage oracle input")
		_, leftOverGas, err := m.env.Call(m.sender, m.addrs.Oracle, poInput, m.startingGas, common.U2560)
		require.NoErrorf(t, err, "evm should not fail, took %d gas", m.startingGas-leftOverGas)
	}

	input := EncodeStepInput(t, stepWitness, mipsevm.LocalContext{}, m.artifacts.MIPS)
	m.lastStepInput = input
	ret, leftOverGas, err := m.env.Call(m.sender, m.addrs.MIPS, input, m.startingGas, common.U2560)
	require.NoError(t, err, "evm should not fail")
	require.Len(t, ret, 32, "expecting 32-byte state hash")
	// remember state hash, to check it against state
	postHash := common.Hash(*(*[32]byte)(ret))
	logs := m.evmState.Logs()
	require.Equal(t, 1, len(logs), "expecting a log with post-state")
	evmPost := logs[0].Data

	stateHash, err := stateHashFn(evmPost)
	require.NoError(t, err, "state hash could not be computed")
	require.Equal(t, stateHash, postHash, "logged state must be accurate")

	m.env.StateDB.RevertToSnapshot(snap)
	t.Logf("EVM step %d took %d gas, and returned stateHash %s", step, m.startingGas-leftOverGas, postHash)
	return evmPost
}

func EncodeStepInput(t *testing.T, wit *mipsevm.StepWitness, localContext mipsevm.LocalContext, mips *foundry.Artifact) []byte {
	input, err := mips.ABI.Pack("step", wit.State, wit.ProofData, localContext)
	require.NoError(t, err)
	return input
}

func (m *MIPSEVM) encodePreimageOracleInput(t *testing.T, preimageKey [32]byte, preimageValue []byte, preimageOffset arch.Word, localContext mipsevm.LocalContext) ([]byte, error) {
	if preimageKey == ([32]byte{}) {
		return nil, errors.New("cannot encode pre-image oracle input, witness has no pre-image to proof")
	}
	localOracle := m.localOracle
	oracle := m.artifacts.Oracle

	switch preimage.KeyType(preimageKey[0]) {
	case preimage.LocalKeyType:
		if len(preimageValue) > 32+8 {
			return nil, fmt.Errorf("local pre-image exceeds maximum size of 32 bytes with key 0x%x", preimageKey)
		}
		preimagePart := preimageValue[8:]
		var tmp [32]byte
		copy(tmp[:], preimagePart)
		input, err := oracle.ABI.Pack("loadLocalData",
			new(big.Int).SetBytes(preimageKey[1:]),
			localContext,
			tmp,
			new(big.Int).SetUint64(uint64(len(preimagePart))),
			new(big.Int).SetUint64(uint64(preimageOffset)),
		)
		require.NoError(t, err)
		return input, nil
	case preimage.Keccak256KeyType:
		input, err := oracle.ABI.Pack(
			"loadKeccak256PreimagePart",
			new(big.Int).SetUint64(uint64(preimageOffset)),
			preimageValue[8:])
		require.NoError(t, err)
		return input, nil
	case preimage.PrecompileKeyType:
		if localOracle == nil {
			return nil, errors.New("local oracle is required for precompile preimages")
		}
		preimage := localOracle.GetPreimage(preimage.Keccak256Key(preimageKey).PreimageKey())
		precompile := common.BytesToAddress(preimage[:20])
		requiredGas := binary.BigEndian.Uint64(preimage[20:28])
		callInput := preimage[28:]
		input, err := oracle.ABI.Pack(
			"loadPrecompilePreimagePart",
			new(big.Int).SetUint64(uint64(preimageOffset)),
			precompile,
			requiredGas,
			callInput,
		)
		require.NoError(t, err)
		return input, nil
	default:
		return nil, fmt.Errorf("unsupported pre-image type %d, cannot prepare preimage with key %x offset %d for oracle",
			preimageKey[0], preimageKey, preimageOffset)
	}
}

func (m *MIPSEVM) assertPreimageOracleReverts(t *testing.T, preimageKey [32]byte, preimageValue []byte, preimageOffset arch.Word) {
	poInput, err := m.encodePreimageOracleInput(t, preimageKey, preimageValue, preimageOffset, mipsevm.LocalContext{})
	require.NoError(t, err, "encode preimage oracle input")
	_, _, evmErr := m.env.Call(m.sender, m.addrs.Oracle, poInput, m.startingGas, common.U2560)

	require.ErrorContains(t, evmErr, "execution reverted")
}

func LogStepFailureAtCleanup(t *testing.T, mipsEvm *MIPSEVM) {
	t.Cleanup(func() {
		if t.Failed() {
			// Note: For easier debugging of a failing step, see MIPS.t.sol#test_step_debug_succeeds()
			t.Logf("Failed while executing step %d with\n\tstep input: %x\n\tpreimageOracle input: %x", mipsEvm.lastStep, mipsEvm.lastStepInput, mipsEvm.lastPreimageOracleInput)
		}
	})
}

type EvmValidator struct {
	evm    *MIPSEVM
	hashFn mipsevm.HashFn
}

// NewEvmValidator creates a validator that can be run repeatedly across multiple steps
func NewEvmValidator(t *testing.T, hashFn mipsevm.HashFn, contracts *ContractMetadata, opts ...evmOption) *EvmValidator {
	evm := newMIPSEVM(contracts, opts...)
	LogStepFailureAtCleanup(t, evm)

	return &EvmValidator{
		evm:    evm,
		hashFn: hashFn,
	}
}

func (v *EvmValidator) ValidateEVM(t *testing.T, stepWitness *mipsevm.StepWitness, step uint64, goVm mipsevm.FPVM) {
	evmPost := v.evm.Step(t, stepWitness, step, v.hashFn)
	goPost, _ := goVm.GetState().EncodeWitness()
	require.Equal(t, hexutil.Bytes(goPost).String(), hexutil.Bytes(evmPost).String(),
		"mipsevm produced different state than EVM")
}

// ValidateEVM runs a single evm step and validates against an FPVM poststate
func ValidateEVM(t *testing.T, stepWitness *mipsevm.StepWitness, step uint64, goVm mipsevm.FPVM, hashFn mipsevm.HashFn, contracts *ContractMetadata, opts ...evmOption) {
	validator := NewEvmValidator(t, hashFn, contracts)
	validator.ValidateEVM(t, stepWitness, step, goVm)
}

type ErrMatcher func(*testing.T, []byte)

func CreateNoopErrorMatcher() ErrMatcher {
	return func(t *testing.T, ret []byte) {}
}

// CreateErrorStringMatcher matches an Error(string)
func CreateErrorStringMatcher(expect string) ErrMatcher {
	return func(t *testing.T, ret []byte) {
		require.Greaterf(t, len(ret), 4, "Return data length should be greater than 4 bytes: %x", ret)
		unpacked, decodeErr := abi.UnpackRevert(ret)
		require.NoError(t, decodeErr, "Failed to unpack revert reason")
		require.Contains(t, unpacked, expect, "Revert reason mismatch")
	}
}

// CreateCustomErrorMatcher matches a custom error given the error signature
func CreateCustomErrorMatcher(sig string) ErrMatcher {
	return func(t *testing.T, ret []byte) {
		expect := crypto.Keccak256([]byte(sig))[:4]
		require.EqualValuesf(t, expect, ret, "return value is %x", ret)
	}
}

// AssertEVMReverts runs a single evm step from an FPVM prestate and asserts that the VM panics
func AssertEVMReverts(t *testing.T, state mipsevm.FPVMState, contracts *ContractMetadata, tracer *tracing.Hooks, ProofData []byte, matcher ErrMatcher) {
	encodedWitness, _ := state.EncodeWitness()
	stepWitness := &mipsevm.StepWitness{
		State:     encodedWitness,
		ProofData: ProofData,
	}
	input := EncodeStepInput(t, stepWitness, mipsevm.LocalContext{}, contracts.Artifacts.MIPS)
	startingGas := uint64(maxStepGas)

	env, evmState := NewEVMEnv(contracts)
	env.Config.Tracer = tracer
	sender := common.Address{0x13, 0x37}
	ret, _, err := env.Call(vm.AccountRef(sender), contracts.Addresses.MIPS, input, startingGas, common.U2560)

	require.EqualValues(t, err, vm.ErrExecutionReverted)
	matcher(t, ret)

	logs := evmState.Logs()
	require.Equal(t, 0, len(logs))
}

func AssertPreimageOracleReverts(t *testing.T, preimageKey [32]byte, preimageValue []byte, preimageOffset arch.Word, contracts *ContractMetadata, opts ...evmOption) {
	evm := newMIPSEVM(contracts, opts...)
	LogStepFailureAtCleanup(t, evm)

	evm.assertPreimageOracleReverts(t, preimageKey, preimageValue, preimageOffset)
}

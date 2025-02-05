package tests

import (
	"io"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	mttestutil "github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded/testutil"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/singlethreaded"
	sttestutil "github.com/ethereum-optimism/optimism/cannon/mipsevm/singlethreaded/testutil"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/testutil"
)

type VMFactory func(po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, log log.Logger, opts ...testutil.StateOption) mipsevm.FPVM

func singleThreadedVmFactory(po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, log log.Logger, opts ...testutil.StateOption) mipsevm.FPVM {
	state := singlethreaded.CreateEmptyState()
	mutator := sttestutil.NewStateMutatorSingleThreaded(state)
	for _, opt := range opts {
		opt(mutator)
	}
	return singlethreaded.NewInstrumentedState(state, po, stdOut, stdErr, nil)
}

func multiThreadedVmFactory(po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, log log.Logger, opts ...testutil.StateOption) mipsevm.FPVM {
	state := multithreaded.CreateEmptyState()
	mutator := mttestutil.NewStateMutatorMultiThreaded(state)
	for _, opt := range opts {
		opt(mutator)
	}
	return multithreaded.NewInstrumentedState(state, po, stdOut, stdErr, log, nil)
}

type ElfVMFactory func(t require.TestingT, elfFile string, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, log log.Logger) mipsevm.FPVM

func singleThreadElfVmFactory(t require.TestingT, elfFile string, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, log log.Logger) mipsevm.FPVM {
	state, meta := testutil.LoadELFProgram(t, elfFile, singlethreaded.CreateInitialState, true)
	fpvm := singlethreaded.NewInstrumentedState(state, po, stdOut, stdErr, meta)
	require.NoError(t, fpvm.InitDebug())
	return fpvm
}

func multiThreadElfVmFactory(t require.TestingT, elfFile string, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, log log.Logger) mipsevm.FPVM {
	state, meta := testutil.LoadELFProgram(t, elfFile, multithreaded.CreateInitialState, false)
	fpvm := multithreaded.NewInstrumentedState(state, po, stdOut, stdErr, log, meta)
	require.NoError(t, fpvm.InitDebug())
	return fpvm
}

type ProofGenerator func(t require.TestingT, state mipsevm.FPVMState, memoryProofAddresses ...arch.Word) []byte

func singleThreadedProofGenerator(t require.TestingT, state mipsevm.FPVMState, memoryProofAddresses ...arch.Word) []byte {
	var proofData []byte

	insnProof := state.GetMemory().MerkleProof(state.GetPC())
	proofData = append(proofData, insnProof[:]...)

	for _, addr := range memoryProofAddresses {
		memProof := state.GetMemory().MerkleProof(addr)
		proofData = append(proofData, memProof[:]...)
	}

	return proofData
}

func multiThreadedProofGenerator(t require.TestingT, state mipsevm.FPVMState, memoryProofAddresses ...arch.Word) []byte {
	mtState, ok := state.(*multithreaded.State)
	if !ok {
		require.Fail(t, "Failed to cast FPVMState to multithreaded State type")
	}

	proofData := mtState.EncodeThreadProof()
	insnProof := mtState.GetMemory().MerkleProof(mtState.GetPC())
	proofData = append(proofData, insnProof[:]...)

	for _, addr := range memoryProofAddresses {
		memProof := mtState.GetMemory().MerkleProof(addr)
		proofData = append(proofData, memProof[:]...)
	}

	return proofData
}

type VersionedVMTestCase struct {
	Name           string
	Contracts      *testutil.ContractMetadata
	StateHashFn    mipsevm.HashFn
	VMFactory      VMFactory
	ElfVMFactory   ElfVMFactory
	ProofGenerator ProofGenerator
}

func GetSingleThreadedTestCase(t require.TestingT) VersionedVMTestCase {
	return VersionedVMTestCase{
		Name:           "single-threaded",
		Contracts:      testutil.TestContractsSetup(t, testutil.MipsSingleThreaded),
		StateHashFn:    singlethreaded.GetStateHashFn(),
		VMFactory:      singleThreadedVmFactory,
		ElfVMFactory:   singleThreadElfVmFactory,
		ProofGenerator: singleThreadedProofGenerator,
	}
}

func GetMultiThreadedTestCase(t require.TestingT) VersionedVMTestCase {
	return VersionedVMTestCase{
		Name:           "multi-threaded",
		Contracts:      testutil.TestContractsSetup(t, testutil.MipsMultithreaded),
		StateHashFn:    multithreaded.GetStateHashFn(),
		VMFactory:      multiThreadedVmFactory,
		ElfVMFactory:   multiThreadElfVmFactory,
		ProofGenerator: multiThreadedProofGenerator,
	}
}

func GetMipsVersionTestCases(t require.TestingT) []VersionedVMTestCase {
	if arch.IsMips32 {
		return []VersionedVMTestCase{
			GetSingleThreadedTestCase(t),
			GetMultiThreadedTestCase(t),
		}
	} else {
		// 64-bit only supports MTCannon
		return []VersionedVMTestCase{
			GetMultiThreadedTestCase(t),
		}
	}
}

type threadProofTestcase struct {
	Name  string
	Proof []byte
}

func GenerateEmptyThreadProofVariations(t require.TestingT) []threadProofTestcase {
	defaultThreadProof := multiThreadedProofGenerator(t, multithreaded.CreateEmptyState())
	zeroBytesThreadProof := make([]byte, multithreaded.THREAD_WITNESS_SIZE)
	copy(zeroBytesThreadProof[multithreaded.SERIALIZED_THREAD_SIZE:], defaultThreadProof[multithreaded.SERIALIZED_THREAD_SIZE:])
	nilBytesThreadProof := defaultThreadProof[multithreaded.SERIALIZED_THREAD_SIZE:]
	return []threadProofTestcase{
		{Name: "default thread proof", Proof: defaultThreadProof},
		{Name: "zeroed thread bytes proof", Proof: zeroBytesThreadProof},
		{Name: "nil thread bytes proof", Proof: nilBytesThreadProof},
	}
}

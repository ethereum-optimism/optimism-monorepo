package singlethreaded

import (
	"io"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type InstrumentedState struct {
	meta       mipsevm.Metadata
	sleepCheck mipsevm.SymbolMatcher

	state *State

	stdOut io.Writer
	stdErr io.Writer

	memoryTracker *exec.MemoryTrackerImpl
	stackTracker  exec.TraceableStackTracker

	preimageOracle *exec.TrackingPreimageOracleReader
}

var _ mipsevm.FPVM = (*InstrumentedState)(nil)

func NewInstrumentedState(state *State, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer) *InstrumentedState {
	return &InstrumentedState{
		sleepCheck:     func(addr uint32) bool { return false },
		state:          state,
		stdOut:         stdOut,
		stdErr:         stdErr,
		memoryTracker:  exec.NewMemoryTracker(state.Memory),
		stackTracker:   &exec.NoopStackTracker{},
		preimageOracle: exec.NewTrackingPreimageOracleReader(po),
	}
}

func (m *InstrumentedState) InitDebug(meta mipsevm.Metadata) error {
	stackTracker, err := exec.NewStackTracker(m.state, meta)
	if err != nil {
		return err
	}
	m.meta = meta
	m.stackTracker = stackTracker
	m.sleepCheck = meta.CreateSymbolMatcher("runtime.notesleep")
	return nil
}

func (m *InstrumentedState) Step(proof bool) (wit *mipsevm.StepWitness, err error) {
	m.preimageOracle.Reset()
	m.memoryTracker.Reset(proof)

	if proof {
		insnProof := m.state.Memory.MerkleProof(m.state.Cpu.PC)
		encodedWitness, stateHash := m.state.EncodeWitness()
		wit = &mipsevm.StepWitness{
			State:     encodedWitness,
			StateHash: stateHash,
			ProofData: insnProof[:],
		}
	}
	err = m.mipsStep()
	if err != nil {
		return nil, err
	}

	if proof {
		memProof := m.memoryTracker.MemProof()
		wit.ProofData = append(wit.ProofData, memProof[:]...)
		lastPreimageKey, lastPreimage, lastPreimageOffset := m.preimageOracle.LastPreimage()
		if lastPreimageOffset != ^uint32(0) {
			wit.PreimageOffset = lastPreimageOffset
			wit.PreimageKey = lastPreimageKey
			wit.PreimageValue = lastPreimage
		}
	}
	return
}

func (m *InstrumentedState) CheckInfiniteLoop() bool {
	return m.sleepCheck(m.state.GetPC())
}

func (m *InstrumentedState) LastPreimage() ([32]byte, []byte, uint32) {
	return m.preimageOracle.LastPreimage()
}

func (m *InstrumentedState) GetState() mipsevm.FPVMState {
	return m.state
}

func (m *InstrumentedState) GetDebugInfo() *mipsevm.DebugInfo {
	return &mipsevm.DebugInfo{
		Pages:               m.state.Memory.PageCount(),
		MemoryUsed:          hexutil.Uint64(m.state.Memory.UsageRaw()),
		NumPreimageRequests: m.preimageOracle.NumPreimageRequests(),
		TotalPreimageSize:   m.preimageOracle.TotalPreimageSize(),
	}
}

func (m *InstrumentedState) Traceback() {
	m.stackTracker.Traceback()
}

func (m *InstrumentedState) LookupSymbol(addr uint32) string {
	if m.meta == nil {
		return ""
	}
	return m.meta.LookupSymbol(addr)
}

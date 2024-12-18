package multithreaded

import (
	"io"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/exec"
)

type InstrumentedState struct {
	state *State

	log    log.Logger
	stdOut io.Writer
	stdErr io.Writer

	memoryTracker *exec.MemoryTrackerImpl
	stackTracker  ThreadedStackTracker
	statsTracker  StatsTracker

	preimageOracle *exec.TrackingPreimageOracleReader
	meta           mipsevm.Metadata
}

var _ mipsevm.FPVM = (*InstrumentedState)(nil)

func NewInstrumentedState(state *State, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, log log.Logger, meta mipsevm.Metadata) *InstrumentedState {
	return &InstrumentedState{
		state:          state,
		log:            log,
		stdOut:         stdOut,
		stdErr:         stdErr,
		memoryTracker:  exec.NewMemoryTracker(state.Memory),
		stackTracker:   &NoopThreadedStackTracker{},
		statsTracker:   NoopStatsTracker(),
		preimageOracle: exec.NewTrackingPreimageOracleReader(po),
		meta:           meta,
	}
}

func (m *InstrumentedState) InitDebug() error {
	stackTracker, err := NewThreadedStackTracker(m.state, m.meta)
	if err != nil {
		return err
	}
	m.stackTracker = stackTracker
	return nil
}

func (m *InstrumentedState) EnableStats() {
	m.statsTracker = NewStatsTracker()
}

func (m *InstrumentedState) Step(proof bool) (wit *mipsevm.StepWitness, err error) {
	m.preimageOracle.Reset()
	m.memoryTracker.Reset(proof)

	if proof {
		proofData := make([]byte, 0)
		threadProof := m.state.EncodeThreadProof()
		insnProof := m.state.Memory.MerkleProof(m.state.GetPC())
		proofData = append(proofData, threadProof[:]...)
		proofData = append(proofData, insnProof[:]...)

		encodedWitness, stateHash := m.state.EncodeWitness()
		wit = &mipsevm.StepWitness{
			State:     encodedWitness,
			StateHash: stateHash,
			ProofData: proofData,
		}
	}
	err = m.mipsStep()
	if err != nil {
		return nil, err
	}

	if proof {
		memProof := m.memoryTracker.MemProof()
		memProof2 := m.memoryTracker.MemProof2()
		wit.ProofData = append(wit.ProofData, memProof[:]...)
		wit.ProofData = append(wit.ProofData, memProof2[:]...)
		lastPreimageKey, lastPreimage, lastPreimageOffset := m.preimageOracle.LastPreimage()
		if lastPreimageOffset != ^arch.Word(0) {
			wit.PreimageOffset = lastPreimageOffset
			wit.PreimageKey = lastPreimageKey
			wit.PreimageValue = lastPreimage
		}
	}
	return
}

func (m *InstrumentedState) CheckInfiniteLoop() bool {
	return false
}

func (m *InstrumentedState) LastPreimage() ([32]byte, []byte, arch.Word) {
	return m.preimageOracle.LastPreimage()
}

func (m *InstrumentedState) GetState() mipsevm.FPVMState {
	return m.state
}

func (m *InstrumentedState) GetDebugInfo() *mipsevm.DebugInfo {
	debugInfo := &mipsevm.DebugInfo{
		Pages:               m.state.Memory.PageCount(),
		MemoryUsed:          hexutil.Uint64(m.state.Memory.UsageRaw()),
		NumPreimageRequests: m.preimageOracle.NumPreimageRequests(),
		TotalPreimageSize:   m.preimageOracle.TotalPreimageSize(),
		TotalSteps:          m.state.GetStep(),
	}
	m.statsTracker.populateDebugInfo(debugInfo)
	return debugInfo
}

func (m *InstrumentedState) Traceback() {
	m.stackTracker.Traceback()
}

func (m *InstrumentedState) LookupSymbol(addr arch.Word) string {
	if m.meta == nil {
		return ""
	}
	return m.meta.LookupSymbol(addr)
}

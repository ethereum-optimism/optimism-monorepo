package mipsevm

import (
	"errors"
	"io"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/core"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/program"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/singlethreaded"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
)

type Debug struct {
	stack  []uint32
	caller []uint32
	meta   *program.Metadata
}

type InstrumentedState struct {
	state *singlethreaded.State

	stdOut io.Writer
	stdErr io.Writer

	lastMemAccess   uint32
	memProofEnabled bool
	memProof        [28 * 32]byte

	preimageOracle *trackingOracle

	// cached pre-image data, including 8 byte length prefix
	lastPreimage []byte
	// key for above preimage
	lastPreimageKey [32]byte
	// offset we last read from, or max uint32 if nothing is read this step
	lastPreimageOffset uint32

	debug        Debug
	debugEnabled bool
}

func NewInstrumentedState(state *singlethreaded.State, po core.PreimageOracle, stdOut, stdErr io.Writer) *InstrumentedState {
	return &InstrumentedState{
		state:          state,
		stdOut:         stdOut,
		stdErr:         stdErr,
		preimageOracle: &trackingOracle{po: po},
	}
}

func NewInstrumentedStateFromFile(stateFile string, po core.PreimageOracle, stdOut, stdErr io.Writer) (*InstrumentedState, error) {
	state, err := jsonutil.LoadJSON[singlethreaded.State](stateFile)
	if err != nil {
		return nil, err
	}
	return &InstrumentedState{
		state:          state,
		stdOut:         stdOut,
		stdErr:         stdErr,
		preimageOracle: &trackingOracle{po: po},
	}, nil
}

func (m *InstrumentedState) InitDebug(meta *program.Metadata) error {
	if meta == nil {
		return errors.New("metadata is nil")
	}
	m.debugEnabled = true
	m.debug.meta = meta
	return nil
}

func (m *InstrumentedState) Step(proof bool) (wit *core.StepWitness, err error) {
	m.memProofEnabled = proof
	m.lastMemAccess = ^uint32(0)
	m.lastPreimageOffset = ^uint32(0)

	if proof {
		insnProof := m.state.Memory.MerkleProof(m.state.Cpu.PC)
		encodedWitness, stateHash := m.state.EncodeWitness()
		wit = &core.StepWitness{
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
		wit.ProofData = append(wit.ProofData, m.memProof[:]...)
		if m.lastPreimageOffset != ^uint32(0) {
			wit.PreimageOffset = m.lastPreimageOffset
			wit.PreimageKey = m.lastPreimageKey
			wit.PreimageValue = m.lastPreimage
		}
	}
	return
}

func (m *InstrumentedState) LastPreimage() ([32]byte, []byte, uint32) {
	return m.lastPreimageKey, m.lastPreimage, m.lastPreimageOffset
}

func (m *InstrumentedState) GetState() core.FPVMState {
	return m.state
}

func (m *InstrumentedState) GetDebugInfo() *core.DebugInfo {
	return &core.DebugInfo{
		Pages:               m.state.Memory.PageCount(),
		NumPreimageRequests: m.preimageOracle.numPreimageRequests,
		TotalPreimageSize:   m.preimageOracle.totalPreimageSize,
	}
}

type trackingOracle struct {
	po                  core.PreimageOracle
	totalPreimageSize   int
	numPreimageRequests int
}

func (d *trackingOracle) Hint(v []byte) {
	d.po.Hint(v)
}

func (d *trackingOracle) GetPreimage(k [32]byte) []byte {
	d.numPreimageRequests++
	preimage := d.po.GetPreimage(k)
	d.totalPreimageSize += len(preimage)
	return preimage
}

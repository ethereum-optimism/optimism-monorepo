package matrix

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/merkle"
	"github.com/ethereum-optimism/optimism/op-challenger/game/keccak/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
)

// StateMatrix implements a stateful keccak sponge with the ability to create state commitments after each permutation
type StateMatrix struct {
	s *state
	// prestateLeaf is the last prestate leaf.
	// Used to retrieve the prestate to squeeze.
	prestateLeaf types.Leaf
	// poststateLeaf is the last poststate leaf.
	// Used to retrieve the poststate to squeeze.
	poststateLeaf types.Leaf
	// merkleTree is the internal [merkle.BinaryMerkleTree] used to generate proofs
	merkleTree *merkle.BinaryMerkleTree
}

var (
	ErrInvalidMaxLen            = errors.New("invalid max length to absorb")
	ErrIncorrectCommitmentCount = errors.New("incorrect number of commitments for input length")
	ErrValid                    = errors.New("state commitments are valid")
	uint256Size                 = 32
)

// Challenge creates a [types.Challenge] to invalidate the provided preimage data if possible.
// [ErrValid] is returned if the provided inputs are valid and no challenge can be created.
func Challenge(data io.Reader, commitments []common.Hash) (types.Challenge, error) {
	s := NewStateMatrix()
	lastValidState := s.PackState()
	var lastValidLeaf types.Leaf
	var firstInvalidLeaf types.Leaf
	for i := 0; ; i++ {
		if i >= len(commitments) {
			// There should have been more commitments.
			// The contracts should prevent this so it can't be challenged, return an error
			return types.Challenge{}, ErrIncorrectCommitmentCount
		}
		claimedCommitment := commitments[i]
		_, err := s.absorbNextLeafInput(data, func() common.Hash { return claimedCommitment })
		isEOF := errors.Is(err, io.EOF)
		if err != nil && !isEOF {
			return types.Challenge{}, fmt.Errorf("failed to verify inputs: %w", err)
		}
		validCommitment := s.StateCommitment()

		if firstInvalidLeaf == (types.Leaf{}) {
			if validCommitment != claimedCommitment {
				lastValidLeaf = s.prestateLeaf
				firstInvalidLeaf = s.poststateLeaf
			} else {
				lastValidState = s.PackState()
			}
		}
		if isEOF {
			if i < len(commitments)-1 {
				// We got too many commitments
				// The contracts should prevent this so it can't be challenged, return an error
				return types.Challenge{}, ErrIncorrectCommitmentCount
			}
			break
		}
	}
	if firstInvalidLeaf != (types.Leaf{}) {
		var prestateProof merkle.Proof
		if lastValidLeaf != (types.Leaf{}) {
			prestateProof = s.merkleTree.ProofAtIndex(lastValidLeaf.Index)
		}
		poststateProof := s.merkleTree.ProofAtIndex(firstInvalidLeaf.Index)
		return types.Challenge{
			StateMatrix:    lastValidState,
			Prestate:       lastValidLeaf,
			PrestateProof:  prestateProof,
			Poststate:      firstInvalidLeaf,
			PoststateProof: poststateProof,
		}, nil
	}
	return types.Challenge{}, ErrValid
}

// NewStateMatrix creates a new state matrix initialized with the initial, zero keccak block.
func NewStateMatrix() *StateMatrix {
	return &StateMatrix{
		s:          newLegacyKeccak256(),
		merkleTree: merkle.NewBinaryMerkleTree(),
	}
}

// StateCommitment returns the state commitment for the current state matrix.
// Additional data may be absorbed after calling this method.
func (d *StateMatrix) StateCommitment() common.Hash {
	buf := d.PackState()
	return crypto.Keccak256Hash(buf)
}

// PackState packs the state in to the solidity ABI encoding required for the state matrix
func (d *StateMatrix) PackState() []byte {
	buf := make([]byte, 0, len(d.s.a)*uint256Size)
	for _, v := range d.s.a {
		buf = append(buf, math.U256Bytes(new(big.Int).SetUint64(v))...)
	}
	return buf
}

// newLeafWithPadding creates a new [Leaf] from inputs, padding the input to the [BlockSize].
func newLeafWithPadding(input []byte, index uint64, commitment common.Hash) types.Leaf {
	// TODO(client-pod#480): Add actual keccak padding to ensure the merkle proofs are correct (for readData)
	var paddedInput [types.BlockSize]byte
	copy(paddedInput[:], input)
	return types.Leaf{
		Input:           paddedInput,
		Index:           index,
		StateCommitment: commitment,
	}
}

func (d *StateMatrix) AbsorbUpTo(in io.Reader, maxLen int) (types.InputData, error) {
	if maxLen < types.BlockSize || maxLen%types.BlockSize != 0 {
		return types.InputData{}, ErrInvalidMaxLen
	}
	input := make([]byte, 0, maxLen)
	commitments := make([]common.Hash, 0, maxLen/types.BlockSize)
	for len(input)+types.BlockSize <= maxLen {
		readData, err := d.absorbNextLeafInput(in, d.StateCommitment)
		if errors.Is(err, io.EOF) {
			input = append(input, readData...)
			commitments = append(commitments, d.StateCommitment())
			return types.InputData{
				Input:       input,
				Commitments: commitments,
				Finalize:    true,
			}, io.EOF
		} else if err != nil {
			return types.InputData{}, err
		}
		input = append(input, readData...)
		commitments = append(commitments, d.StateCommitment())
	}

	return types.InputData{
		Input:       input,
		Commitments: commitments,
		Finalize:    false,
	}, nil
}

// PrestateWithProof returns the prestate leaf with its merkle proof.
func (d *StateMatrix) PrestateWithProof() (types.Leaf, merkle.Proof) {
	proof := d.merkleTree.ProofAtIndex(d.prestateLeaf.Index)
	return d.prestateLeaf, proof
}

// PoststateWithProof returns the poststate leaf with its merkle proof.
func (d *StateMatrix) PoststateWithProof() (types.Leaf, merkle.Proof) {
	proof := d.merkleTree.ProofAtIndex(d.poststateLeaf.Index)
	return d.poststateLeaf, proof
}

// absorbNextLeafInput reads up to [BlockSize] bytes from in and absorbs them into the state matrix.
// If EOF is reached while reading, the state matrix is finalized and [io.EOF] is returned.
func (d *StateMatrix) absorbNextLeafInput(in io.Reader, stateCommitment func() common.Hash) ([]byte, error) {
	data := make([]byte, types.BlockSize)
	read := 0
	final := false
	for read < types.BlockSize {
		n, err := in.Read(data[read:])
		if errors.Is(err, io.EOF) {
			read += n
			final = true
			break
		} else if err != nil {
			return nil, err
		}
		read += n
	}
	input := data[:read]
	// Don't add the padding if we read a full block of input data, even if we reached EOF.
	// Just absorb the full block and return so the caller can capture the state commitment after the block
	// The next call will read no data from the Reader (already at EOF) and so add the final padding as an
	// additional block. We can then return EOF to indicate there are no further blocks.
	final = final && len(input) < types.BlockSize
	d.absorbLeafInput(input, final)
	commitment := stateCommitment()
	if d.poststateLeaf == (types.Leaf{}) {
		d.prestateLeaf = types.Leaf{}
		d.poststateLeaf = newLeafWithPadding(input, 0, commitment)
	} else {
		d.prestateLeaf = d.poststateLeaf
		d.poststateLeaf = newLeafWithPadding(input, d.prestateLeaf.Index+1, commitment)
	}
	d.merkleTree.AddLeaf(d.poststateLeaf.Hash())
	if final {
		return input, io.EOF
	}
	return input, nil
}

// absorbLeafInput absorbs the specified data into the keccak sponge.
// If final is true, the data is padded to the required length, otherwise it must be exactly [types.BlockSize] bytes.
func (d *StateMatrix) absorbLeafInput(data []byte, final bool) {
	if !final && len(data) != types.BlockSize {
		panic("sha3: Incorrect leaf data length")
	}
	_, _ = d.s.Write(data[:])
	if final {
		d.s.padAndPermute(d.s.dsbyte)
	}
}

// Hash finalizes the keccak permutation and returns the final hash.
// No further leaves can be absorbed after this is called
func (d *StateMatrix) Hash() (h common.Hash) {
	_, _ = d.s.Read(h[:])
	return h
}

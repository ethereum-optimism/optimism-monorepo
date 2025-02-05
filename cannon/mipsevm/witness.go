package mipsevm

import (
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum/go-ethereum/common"
)

type LocalContext common.Hash

type StepWitness struct {
	// encoded state witness
	State     []byte
	StateHash common.Hash

	ProofData []byte

	PreimageKey    [32]byte // zeroed when no pre-image is accessed
	PreimageValue  []byte   // including the 8-byte length prefix
	PreimageOffset arch.Word
}

func (wit *StepWitness) HasPreimage() bool {
	return wit.PreimageKey != ([32]byte{})
}

type HashFn func(sw []byte) (common.Hash, error)

func AppendBoolToWitness(witnessData []byte, boolVal bool) []byte {
	if boolVal {
		return append(witnessData, 1)
	} else {
		return append(witnessData, 0)
	}
}

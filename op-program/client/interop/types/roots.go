package types

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	IntermediateTransitionVersion = byte(255)
)

type OptimisticBlock struct {
	BlockHash  common.Hash
	OutputRoot eth.Bytes32
}

type TransitionState struct {
	SuperRoot       []byte
	PendingProgress []OptimisticBlock
	Step            uint64
}

func (t *TransitionState) String() string {
	return fmt.Sprintf("{SuperRoot: %x, PendingProgress: %v, Step: %d}", t.SuperRoot, t.PendingProgress, t.Step)
}

func (i *TransitionState) Version() byte {
	return IntermediateTransitionVersion
}

func (i *TransitionState) Marshal() []byte {
	rlpData, err := rlp.EncodeToBytes(i)
	if err != nil {
		panic(err)
	}
	return append([]byte{IntermediateTransitionVersion}, rlpData...)
}

func (i *TransitionState) Hash() common.Hash {
	data := i.Marshal()
	return crypto.Keccak256Hash(data)
}

func UnmarshalTransitionState(data []byte) (*TransitionState, error) {
	if len(data) == 0 {
		return nil, eth.ErrInvalidSuperRoot
	}
	switch data[0] {
	case IntermediateTransitionVersion:
		return unmarshalTransitionSate(data)
	case eth.SuperRootVersionV1:
		return &TransitionState{SuperRoot: data}, nil
	default:
		return nil, eth.ErrInvalidSuperRootVersion
	}
}

func unmarshalTransitionSate(data []byte) (*TransitionState, error) {
	if len(data) == 0 {
		return nil, eth.ErrInvalidSuperRoot
	}
	var state TransitionState
	if err := rlp.DecodeBytes(data[1:], &state); err != nil {
		return nil, err
	}
	return &state, nil
}

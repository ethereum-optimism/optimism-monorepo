package frontend

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
)

const maxJobIDLength = 100

var ErrInvalidJobID = errors.New("invalid job ID")

type JobID string

func (id JobID) MarshalText() ([]byte, error) {
	if len(id) > maxJobIDLength {
		return nil, ErrInvalidJobID
	}
	return []byte(id), nil
}

func (id *JobID) UnmarshalText(data []byte) error {
	if len(data) > maxJobIDLength {
		return ErrInvalidJobID
	}
	*id = JobID(data)
	return nil
}

type BuildOpts struct {
	// Parent block to build on top of
	Parent common.Hash `json:"parent"`

	// L1Origin overrides the L1 origin of the block.
	// Optional, by default the L1 origin of the parent block
	// is progressed when first allowed (respecting time invariants).
	L1Origin *common.Hash `json:"l1Origin,omitempty"`
}

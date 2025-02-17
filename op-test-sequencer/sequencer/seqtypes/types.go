package seqtypes

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
)

const maxIDLength = 100

var ErrInvalidID = errors.New("invalid ID")

type genericID string

func (id genericID) String() string {
	return string(id)
}

func (id genericID) MarshalText() ([]byte, error) {
	if len(id) > maxIDLength {
		return nil, ErrInvalidID
	}
	return []byte(id), nil
}

func (id *genericID) UnmarshalText(data []byte) error {
	if len(data) > maxIDLength {
		return ErrInvalidID
	}
	*id = genericID(data)
	return nil
}

type JobID genericID

func (id JobID) String() string {
	return genericID(id).String()
}

func (id JobID) MarshalText() ([]byte, error) {
	return genericID(id).MarshalText()
}

func (id *JobID) UnmarshalText(data []byte) error {
	return (*genericID)(id).UnmarshalText(data)
}

type BuilderID genericID

func (id BuilderID) String() string {
	return genericID(id).String()
}

func (id BuilderID) MarshalText() ([]byte, error) {
	return genericID(id).MarshalText()
}

func (id *BuilderID) UnmarshalText(data []byte) error {
	return (*genericID)(id).UnmarshalText(data)
}

type SignerID genericID

func (id SignerID) String() string {
	return genericID(id).String()
}

func (id SignerID) MarshalText() ([]byte, error) {
	return genericID(id).MarshalText()
}

func (id *SignerID) UnmarshalText(data []byte) error {
	return (*genericID)(id).UnmarshalText(data)
}

var ErrUnknownBuilder = errors.New("unknown builder")
var ErrUnknownJob = errors.New("unknown job")

type BuildOpts struct {
	// Parent block to build on top of
	Parent common.Hash `json:"parent"`

	// L1Origin overrides the L1 origin of the block.
	// Optional, by default the L1 origin of the parent block
	// is progressed when first allowed (respecting time invariants).
	L1Origin *common.Hash `json:"l1Origin,omitempty"`
}

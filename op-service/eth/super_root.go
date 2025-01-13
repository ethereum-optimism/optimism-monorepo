package eth

import (
	"encoding/binary"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrInvalidSuperRoot        = errors.New("invalid super root")
	ErrInvalidSuperRootVersion = errors.New("invalid super root version")
	SuperRootVersionV1         = byte(1)
)

const (
	// SuperRootVersionV1MinLen is the minimum length of a V1 super root prior to hashing
	// Must contain a 1 byte version, uint64 timestamp and at least one chain's output root hash
	SuperRootVersionV1MinLen = 1 + 8 + 32
)

type Super interface {
	Version() byte
	Marshal() []byte
}

func SuperRoot(super Super) Bytes32 {
	marshaled := super.Marshal()
	return Bytes32(crypto.Keccak256Hash(marshaled))
}

type ChainIDAndOutput struct {
	ChainID uint64
	Output  Bytes32
}

func (c *ChainIDAndOutput) Marshal() []byte {
	d := make([]byte, 64)
	binary.BigEndian.PutUint64(d[24:32], c.ChainID)
	copy(d[32:], c.Output[:])
	return d
}

type SuperV1 struct {
	Timestamp uint64
	Chains    []ChainIDAndOutput
}

func (o *SuperV1) Version() byte {
	return SuperRootVersionV1
}

func (o *SuperV1) Marshal() []byte {
	buf := make([]byte, 0, 9+len(o.Chains)*64)
	version := o.Version()
	buf = append(buf, version)
	buf = binary.BigEndian.AppendUint64(buf, o.Timestamp)
	for _, o := range o.Chains {
		buf = append(buf, o.Marshal()...)
	}
	return buf
}

func UnmarshalSuperRoot(data []byte) (Super, error) {
	if len(data) < 1 {
		return nil, ErrInvalidSuperRoot
	}
	ver := data[0]
	switch ver {
	case SuperRootVersionV1:
		return unmarshalSuperRootV1(data)
	default:
		return nil, ErrInvalidSuperRootVersion
	}
}

func unmarshalSuperRootV1(data []byte) (*SuperV1, error) {
	// Must contain the version, timestamp and at least one output root.
	if len(data) < SuperRootVersionV1MinLen {
		return nil, ErrInvalidSuperRoot
	}
	// Must contain complete chain output roots
	if (len(data)-9)%32 != 0 {
		return nil, ErrInvalidSuperRoot
	}
	var output SuperV1
	// data[:1] is the version
	output.Timestamp = binary.BigEndian.Uint64(data[1:9])
	for i := 9; i < len(data); i += 64 {
		chainOutput := ChainIDAndOutput{
			ChainID: binary.BigEndian.Uint64(data[i+24 : i+32]),
			Output:  Bytes32(data[i+32 : i+64]),
		}
		output.Chains = append(output.Chains, chainOutput)
	}
	return &output, nil
}

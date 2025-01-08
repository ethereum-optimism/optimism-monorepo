package eth

import (
	"encoding/binary"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type OutputResponse struct {
	Version               Bytes32     `json:"version"`
	OutputRoot            Bytes32     `json:"outputRoot"`
	BlockRef              L2BlockRef  `json:"blockRef"`
	WithdrawalStorageRoot common.Hash `json:"withdrawalStorageRoot"`
	StateRoot             common.Hash `json:"stateRoot"`
	Status                *SyncStatus `json:"syncStatus"`
}

type SafeHeadResponse struct {
	L1Block  BlockID `json:"l1Block"`
	SafeHead BlockID `json:"safeHead"`
}

var (
	ErrInvalidOutput        = errors.New("invalid output")
	ErrInvalidOutputVersion = errors.New("invalid output version")

	OutputVersionV0 = Bytes32{}
	OutputVersionV1 = Bytes32{0x01}
)

const (
	OutputVersionV0Len = 128

	// OutputVersionV1MinLen is the minimum length of a V1 output root prior to hashing
	// Must contain a 32 byte version, uint64 timestamp and at least one chain's output root hash
	OutputVersionV1MinLen = 32 + 8 + 32
)

type Output interface {
	// Version returns the version of the L2 output
	Version() Bytes32

	// Marshal a L2 output into a byte slice for hashing
	Marshal() []byte
}

type OutputV0 struct {
	StateRoot                Bytes32
	MessagePasserStorageRoot Bytes32
	BlockHash                common.Hash
}

func (o *OutputV0) Version() Bytes32 {
	return OutputVersionV0
}

func (o *OutputV0) Marshal() []byte {
	var buf [OutputVersionV0Len]byte
	version := o.Version()
	copy(buf[:32], version[:])
	copy(buf[32:], o.StateRoot[:])
	copy(buf[64:], o.MessagePasserStorageRoot[:])
	copy(buf[96:], o.BlockHash[:])
	return buf[:]
}

type OutputV1 struct {
	Timestamp uint64
	Outputs   []Bytes32
}

func (o *OutputV1) Version() Bytes32 {
	return OutputVersionV1
}

func (o *OutputV1) Marshal() []byte {
	buf := make([]byte, 0, 40+len(o.Outputs)*32)
	version := o.Version()
	buf = append(buf, version[:]...)
	buf = binary.BigEndian.AppendUint64(buf, o.Timestamp)
	for _, o := range o.Outputs {
		buf = append(buf, o[:]...)
	}
	return buf
}

// OutputRoot returns the keccak256 hash of the marshaled L2 output
func OutputRoot(output Output) Bytes32 {
	marshaled := output.Marshal()
	return Bytes32(crypto.Keccak256Hash(marshaled))
}

func UnmarshalOutput(data []byte) (Output, error) {
	if len(data) < 32 {
		return nil, ErrInvalidOutput
	}
	var ver Bytes32
	copy(ver[:], data[:32])
	switch ver {
	case OutputVersionV0:
		return unmarshalOutputV0(data)
	case OutputVersionV1:
		return unmarshalOutputV1(data)
	default:
		return nil, ErrInvalidOutputVersion
	}
}

func unmarshalOutputV0(data []byte) (*OutputV0, error) {
	if len(data) != OutputVersionV0Len {
		return nil, ErrInvalidOutput
	}
	var output OutputV0
	// data[:32] is the version
	copy(output.StateRoot[:], data[32:64])
	copy(output.MessagePasserStorageRoot[:], data[64:96])
	copy(output.BlockHash[:], data[96:128])
	return &output, nil
}

func unmarshalOutputV1(data []byte) (*OutputV1, error) {
	// Must contain the version, timestamp and at least one output root.
	if len(data) < OutputVersionV1MinLen {
		return nil, ErrInvalidOutput
	}
	// Must contain complete chain output roots
	if (len(data)-40)%32 != 0 {
		return nil, ErrInvalidOutput
	}
	var output OutputV1
	// data[:32] is the version
	output.Timestamp = binary.BigEndian.Uint64(data[32:40])
	for i := 40; i < len(data); i += 32 {
		chainOutput := Bytes32{}
		copy(chainOutput[:], data[i:i+32])
		output.Outputs = append(output.Outputs, chainOutput)
	}
	return &output, nil
}

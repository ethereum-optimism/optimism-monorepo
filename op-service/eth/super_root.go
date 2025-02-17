package eth

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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
	ChainID ChainID
	Output  Bytes32
}

func (c *ChainIDAndOutput) Marshal() []byte {
	d := make([]byte, 64)
	chainID := c.ChainID.Bytes32()
	copy(d[0:32], chainID[:])
	copy(d[32:], c.Output[:])
	return d
}

func NewSuperV1(timestamp uint64, chains ...ChainIDAndOutput) *SuperV1 {
	slices.SortFunc(chains, func(a, b ChainIDAndOutput) int {
		return a.ChainID.Cmp(b.ChainID)
	})
	return &SuperV1{
		Timestamp: timestamp,
		Chains:    chains,
	}
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
			ChainID: ChainIDFromBytes32([32]byte(data[i : i+32])),
			Output:  Bytes32(data[i+32 : i+64]),
		}
		output.Chains = append(output.Chains, chainOutput)
	}
	return &output, nil
}

type ChainRootInfo struct {
	ChainID ChainID `json:"chainID"`
	// Canonical is the output root of the latest canonical block at a particular Timestamp.
	Canonical Bytes32 `json:"canonical"`
	// Pending is the output root preimage for the latest block at a particular Timestamp prior to validation of
	// executing messages. If the original block was valid, this will be the preimage of the
	// output root from the Canonical array. If it was invalid, it will be the output root preimage from the
	// Optimistic Block Deposited Transaction added to the deposit-only block.
	Pending []byte `json:"pending"`
}

type chainRootInfoMarshalling struct {
	ChainID   ChainID       `json:"chainID"`
	Canonical common.Hash   `json:"canonical"`
	Pending   hexutil.Bytes `json:"pending"`
}

func (i ChainRootInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(&chainRootInfoMarshalling{
		ChainID:   i.ChainID,
		Canonical: common.Hash(i.Canonical),
		Pending:   i.Pending,
	})
}

func (i *ChainRootInfo) UnmarshalJSON(input []byte) error {
	var dec chainRootInfoMarshalling
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	i.ChainID = dec.ChainID
	i.Canonical = Bytes32(dec.Canonical)
	i.Pending = dec.Pending
	return nil
}

type SuperRootResponse struct {
	CrossSafeDerivedFrom BlockID `json:"crossSafeDerivedFrom"`
	Timestamp            uint64  `json:"timestamp"`
	SuperRoot            Bytes32 `json:"superRoot"`
	Version              byte    `json:"version"`
	// Chains is the list of ChainRootInfo for each chain in the dependency set.
	// It represents the state of the chain at or before the Timestamp.
	Chains []ChainRootInfo `json:"chains"`
}

type superRootResponseMarshalling struct {
	CrossSafeDerivedFrom BlockID         `json:"crossSafeDerivedFrom"`
	Timestamp            hexutil.Uint64  `json:"timestamp"`
	SuperRoot            common.Hash     `json:"superRoot"`
	Version              hexutil.Bytes   `json:"version"`
	Chains               []ChainRootInfo `json:"chains"`
}

func (r SuperRootResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(&superRootResponseMarshalling{
		CrossSafeDerivedFrom: r.CrossSafeDerivedFrom,
		Timestamp:            hexutil.Uint64(r.Timestamp),
		SuperRoot:            common.Hash(r.SuperRoot),
		Version:              hexutil.Bytes{r.Version},
		Chains:               r.Chains,
	})
}

func (r *SuperRootResponse) UnmarshalJSON(input []byte) error {
	var dec superRootResponseMarshalling
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	r.CrossSafeDerivedFrom = dec.CrossSafeDerivedFrom
	r.Timestamp = uint64(dec.Timestamp)
	r.SuperRoot = Bytes32(dec.SuperRoot)
	if len(dec.Version) != 1 {
		return ErrInvalidSuperRootVersion
	}
	r.Version = dec.Version[0]
	r.Chains = dec.Chains
	return nil
}

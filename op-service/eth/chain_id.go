package eth

import (
	"fmt"
	"math"
	"math/big"

	"github.com/holiman/uint256"
)

type ChainID uint256.Int

func ChainIDFromBig(chainID *big.Int) ChainID {
	return ChainID(*uint256.MustFromBig(chainID))
}

func ChainIDFromUInt64(i uint64) ChainID {
	return ChainID(*uint256.NewInt(i))
}

func ChainIDFromBytes32(b [32]byte) ChainID {
	val := new(uint256.Int).SetBytes(b[:])
	return ChainID(*val)
}

func ParseDecimalChainID(chainID string) (ChainID, error) {
	v, err := uint256.FromDecimal(chainID)
	if err != nil {
		return ChainID{}, err
	}
	return ChainID(*v), nil
}

func (id ChainID) String() string {
	return ((*uint256.Int)(&id)).Dec()
}

func (id ChainID) ToUInt32() (uint32, error) {
	v := (*uint256.Int)(&id)
	if !v.IsUint64() {
		return 0, fmt.Errorf("ChainID too large for uint32: %v", id)
	}
	v64 := v.Uint64()
	if v64 > math.MaxUint32 {
		return 0, fmt.Errorf("ChainID too large for uint32: %v", id)
	}
	return uint32(v64), nil
}

func (id ChainID) Bytes32() [32]byte {
	return (*uint256.Int)(&id).Bytes32()
}

// EvilChainIDToUInt64 converts a ChainID to a uint64 and panic's if the ChainID is too large for a UInt64
// It is "evil" because 32 byte ChainIDs should be universally supported which this method breaks. It is provided
// for legacy purposes to facilitate a transition to full 32 byte chain ID support and should not be used in new code.
// Existing calls should be replaced with full 32 byte support whenever possible.
func EvilChainIDToUInt64(id ChainID) uint64 {
	v := (*uint256.Int)(&id)
	if !v.IsUint64() {
		panic(fmt.Errorf("ChainID too large for uint64: %v", id))
	}
	return v.Uint64()
}

func (id *ChainID) ToBig() *big.Int {
	return (*uint256.Int)(id).ToBig()
}

func (id ChainID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

func (id *ChainID) UnmarshalText(data []byte) error {
	var x uint256.Int
	err := x.UnmarshalText(data)
	if err != nil {
		return err
	}
	*id = ChainID(x)
	return nil
}

func (id ChainID) Cmp(other ChainID) int {
	return (*uint256.Int)(&id).Cmp((*uint256.Int)(&other))
}

package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// ChainIndex represents the lifetime of a chain in a dependency set.
type ChainIndex uint32

func (ci ChainIndex) String() string {
	return strconv.FormatUint(uint64(ci), 10)
}

func (ci ChainIndex) MarshalText() ([]byte, error) {
	return []byte(ci.String()), nil
}

func (ci *ChainIndex) UnmarshalText(data []byte) error {
	v, err := strconv.ParseUint(string(data), 10, 32)
	if err != nil {
		return err
	}
	*ci = ChainIndex(v)
	return nil
}

type ExecutingMessage struct {
	Chain     ChainIndex // same as ChainID for now, but will be indirect, i.e. translated to full ID, later
	BlockNum  uint64
	LogIdx    uint32
	Timestamp uint64
	Hash      common.Hash
}

func (s *ExecutingMessage) String() string {
	return fmt.Sprintf("ExecMsg(chainIndex: %s, block: %d, log: %d, time: %d, logHash: %s)",
		s.Chain, s.BlockNum, s.LogIdx, s.Timestamp, s.Hash)
}

type Message struct {
	Identifier  Identifier  `json:"identifier"`
	PayloadHash common.Hash `json:"payloadHash"`
}

type Identifier struct {
	Origin      common.Address
	BlockNumber uint64
	LogIndex    uint32
	Timestamp   uint64
	ChainID     ChainID // flat, not a pointer, to make Identifier safe as map key
}

type identifierMarshaling struct {
	Origin      common.Address `json:"origin"`
	BlockNumber hexutil.Uint64 `json:"blockNumber"`
	LogIndex    hexutil.Uint64 `json:"logIndex"`
	Timestamp   hexutil.Uint64 `json:"timestamp"`
	ChainID     hexutil.U256   `json:"chainID"`
}

func (id Identifier) MarshalJSON() ([]byte, error) {
	var enc identifierMarshaling
	enc.Origin = id.Origin
	enc.BlockNumber = hexutil.Uint64(id.BlockNumber)
	enc.LogIndex = hexutil.Uint64(id.LogIndex)
	enc.Timestamp = hexutil.Uint64(id.Timestamp)
	enc.ChainID = (hexutil.U256)(id.ChainID)
	return json.Marshal(&enc)
}

func (id *Identifier) UnmarshalJSON(input []byte) error {
	var dec identifierMarshaling
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	id.Origin = dec.Origin
	id.BlockNumber = uint64(dec.BlockNumber)
	if dec.LogIndex > math.MaxUint32 {
		return fmt.Errorf("log index too large: %d", dec.LogIndex)
	}
	id.LogIndex = uint32(dec.LogIndex)
	id.Timestamp = uint64(dec.Timestamp)
	id.ChainID = (ChainID)(dec.ChainID)
	return nil
}

type SafetyLevel string

func (lvl SafetyLevel) String() string {
	return string(lvl)
}

func (lvl SafetyLevel) Valid() bool {
	switch lvl {
	case Finalized, CrossSafe, LocalSafe, CrossUnsafe, LocalUnsafe:
		return true
	default:
		return false
	}
}

func (lvl SafetyLevel) MarshalText() ([]byte, error) {
	return []byte(lvl), nil
}

func (lvl *SafetyLevel) UnmarshalText(text []byte) error {
	if lvl == nil {
		return errors.New("cannot unmarshal into nil SafetyLevel")
	}
	x := SafetyLevel(text)
	if !x.Valid() {
		return fmt.Errorf("unrecognized safety level: %q", text)
	}
	*lvl = x
	return nil
}

// AtLeastAsSafe returns true if the receiver is at least as safe as the other SafetyLevel.
// Safety levels are assumed to graduate from LocalUnsafe to LocalSafe to CrossUnsafe to CrossSafe, with Finalized as the strongest.
func (lvl *SafetyLevel) AtLeastAsSafe(min SafetyLevel) bool {
	relativeSafety := map[SafetyLevel]int{
		Invalid:     0,
		LocalUnsafe: 1,
		LocalSafe:   2,
		CrossUnsafe: 3,
		CrossSafe:   4,
		Finalized:   5,
	}
	// if either level is not recognized, return false
	_, ok := relativeSafety[*lvl]
	if !ok {
		return false
	}
	_, ok = relativeSafety[min]
	if !ok {
		return false
	}
	// compare the relative safety levels to determine if the receiver is at least as safe as the other
	return relativeSafety[*lvl] >= relativeSafety[min]
}

const (
	// Finalized is CrossSafe, with the additional constraint that every
	// dependency is derived only from finalized L1 input data.
	// This matches RPC label "finalized".
	Finalized SafetyLevel = "finalized"
	// CrossSafe is as safe as LocalSafe, with all its dependencies
	// also fully verified to be reproducible from L1.
	// This matches RPC label "safe".
	CrossSafe SafetyLevel = "safe"
	// LocalSafe is verified to be reproducible from L1,
	// without any verified cross-L2 dependencies.
	// This does not have an RPC label.
	LocalSafe SafetyLevel = "local-safe"
	// CrossUnsafe is as safe as LocalUnsafe,
	// but with verified cross-L2 dependencies that are at least CrossUnsafe.
	// This does not have an RPC label.
	CrossUnsafe SafetyLevel = "cross-unsafe"
	// LocalUnsafe is the safety of the tip of the chain. This matches RPC label "unsafe".
	LocalUnsafe SafetyLevel = "unsafe"
	// Invalid is the safety of when the message or block is not matching the expected data.
	Invalid SafetyLevel = "invalid"
)

type ChainID uint256.Int

func ChainIDFromBig(chainID *big.Int) ChainID {
	return ChainID(*uint256.MustFromBig(chainID))
}

func ChainIDFromUInt64(i uint64) ChainID {
	return ChainID(*uint256.NewInt(i))
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

type ReferenceView struct {
	Local eth.BlockID `json:"local"`
	Cross eth.BlockID `json:"cross"`
}

func (v ReferenceView) String() string {
	return fmt.Sprintf("View(local: %s, cross: %s)", v.Local, v.Cross)
}

type BlockSeal struct {
	Hash      common.Hash
	Number    uint64
	Timestamp uint64
}

func (s BlockSeal) String() string {
	return fmt.Sprintf("BlockSeal(hash:%s, number:%d, time:%d)", s.Hash, s.Number, s.Timestamp)
}

func (s BlockSeal) ID() eth.BlockID {
	return eth.BlockID{Hash: s.Hash, Number: s.Number}
}

func (s BlockSeal) MustWithParent(parent eth.BlockID) eth.BlockRef {
	ref, err := s.WithParent(parent)
	if err != nil {
		panic(err)
	}
	return ref
}

func (s BlockSeal) WithParent(parent eth.BlockID) (eth.BlockRef, error) {
	// prevent parent attachment if the parent is not the previous block,
	// and the block is not the genesis block
	if s.Number != parent.Number+1 && s.Number != 0 {
		return eth.BlockRef{}, fmt.Errorf("invalid parent block %s to combine with %s", parent, s)
	}
	return eth.BlockRef{
		Hash:       s.Hash,
		Number:     s.Number,
		ParentHash: parent.Hash,
		Time:       s.Timestamp,
	}, nil
}

func (s BlockSeal) ForceWithParent(parent eth.BlockID) eth.BlockRef {
	return eth.BlockRef{
		Hash:       s.Hash,
		Number:     s.Number,
		ParentHash: parent.Hash,
		Time:       s.Timestamp,
	}
}

func BlockSealFromRef(ref eth.BlockRef) BlockSeal {
	return BlockSeal{
		Hash:      ref.Hash,
		Number:    ref.Number,
		Timestamp: ref.Time,
	}
}

// PayloadHashToLogHash converts the payload hash to the log hash
// it is the concatenation of the log's address and the hash of the log's payload,
// which is then hashed again. This is the hash that is stored in the log storage.
// The logHash can then be used to traverse from the executing message
// to the log the referenced initiating message.
func PayloadHashToLogHash(payloadHash common.Hash, addr common.Address) common.Hash {
	msg := make([]byte, 0, 2*common.HashLength)
	msg = append(msg, addr.Bytes()...)
	msg = append(msg, payloadHash.Bytes()...)
	return crypto.Keccak256Hash(msg)
}

// LogToMessagePayload is the data that is hashed to get the payloadHash
// it is the concatenation of the log's topics and data
// the implementation is based on the interop messaging spec
func LogToMessagePayload(l *ethTypes.Log) []byte {
	msg := make([]byte, 0)
	for _, topic := range l.Topics {
		msg = append(msg, topic.Bytes()...)
	}
	msg = append(msg, l.Data...)
	return msg
}

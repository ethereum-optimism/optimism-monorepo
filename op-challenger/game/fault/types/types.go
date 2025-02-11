package types

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrGameDepthReached   = errors.New("game depth reached")
	ErrL2BlockNumberValid = errors.New("l2 block number is valid")
	ErrNotInSync          = errors.New("local node too far behind")
)

type GameType uint32

const (
	CannonGameType       GameType = 0
	PermissionedGameType GameType = 1
	AsteriscGameType     GameType = 2
	AsteriscKonaGameType GameType = 3
	SuperCannonGameType  GameType = 4
	OPSuccinctGameType   GameType = 6
	FastGameType         GameType = 254
	AlphabetGameType     GameType = 255
	KailuaGameType       GameType = 1337
	UnknownGameType      GameType = math.MaxUint32
)

func (t GameType) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t GameType) String() string {
	switch t {
	case CannonGameType:
		return "cannon"
	case PermissionedGameType:
		return "permissioned"
	case AsteriscGameType:
		return "asterisc"
	case AsteriscKonaGameType:
		return "asterisc-kona"
	case SuperCannonGameType:
		return "super-cannon"
	case OPSuccinctGameType:
		return "op-succinct"
	case FastGameType:
		return "fast"
	case AlphabetGameType:
		return "alphabet"
	case KailuaGameType:
		return "kailua"
	default:
		return fmt.Sprintf("<invalid: %d>", t)
	}
}

type TraceType string

const (
	TraceTypeAlphabet     TraceType = "alphabet"
	TraceTypeFast         TraceType = "fast"
	TraceTypeCannon       TraceType = "cannon"
	TraceTypeAsterisc     TraceType = "asterisc"
	TraceTypeAsteriscKona TraceType = "asterisc-kona"
	TraceTypePermissioned TraceType = "permissioned"
	TraceTypeSuperCannon  TraceType = "super-cannon"
)

var TraceTypes = []TraceType{TraceTypeAlphabet, TraceTypeCannon, TraceTypePermissioned, TraceTypeAsterisc, TraceTypeAsteriscKona, TraceTypeFast, TraceTypeSuperCannon}

func (t TraceType) String() string {
	return string(t)
}

// Set implements the Set method required by the [cli.Generic] interface.
func (t *TraceType) Set(value string) error {
	if !ValidTraceType(TraceType(value)) {
		return fmt.Errorf("unknown trace type: %q", value)
	}
	*t = TraceType(value)
	return nil
}

func (t *TraceType) Clone() any {
	cpy := *t
	return &cpy
}

func ValidTraceType(value TraceType) bool {
	for _, t := range TraceTypes {
		if t == value {
			return true
		}
	}
	return false
}

func (t TraceType) GameType() GameType {
	switch t {
	case TraceTypeCannon:
		return CannonGameType
	case TraceTypePermissioned:
		return PermissionedGameType
	case TraceTypeAsterisc:
		return AsteriscGameType
	case TraceTypeAsteriscKona:
		return AsteriscKonaGameType
	case TraceTypeFast:
		return FastGameType
	case TraceTypeAlphabet:
		return AlphabetGameType
	case TraceTypeSuperCannon:
		return SuperCannonGameType
	default:
		return UnknownGameType
	}
}

type ClockReader interface {
	Now() time.Time
}

// PreimageOracleData encapsulates the preimage oracle data
// to load into the onchain oracle.
type PreimageOracleData struct {
	IsLocal      bool
	OracleKey    []byte
	oracleData   []byte
	OracleOffset uint32

	// 4844 blob data
	BlobFieldIndex uint64
	BlobCommitment []byte
	BlobProof      []byte
}

// GetIdent returns the ident for the preimage oracle data.
func (p *PreimageOracleData) GetIdent() *big.Int {
	return new(big.Int).SetBytes(p.OracleKey[1:])
}

// GetPreimageWithoutSize returns the preimage for the preimage oracle data.
func (p *PreimageOracleData) GetPreimageWithoutSize() []byte {
	return p.oracleData[8:]
}

// GetPreimageWithSize returns the preimage with its length prefix.
func (p *PreimageOracleData) GetPreimageWithSize() []byte {
	return p.oracleData
}

func (p *PreimageOracleData) GetPrecompileAddress() common.Address {
	return common.BytesToAddress(p.oracleData[8:28])
}

func (p *PreimageOracleData) GetPrecompileRequiredGas() uint64 {
	return binary.BigEndian.Uint64(p.oracleData[28:36])
}

func (p *PreimageOracleData) GetPrecompileInput() []byte {
	return p.oracleData[36:]
}

// NewPreimageOracleData creates a new [PreimageOracleData] instance.
func NewPreimageOracleData(key []byte, data []byte, offset uint32) *PreimageOracleData {
	return &PreimageOracleData{
		IsLocal:      len(key) > 0 && key[0] == byte(preimage.LocalKeyType),
		OracleKey:    key,
		oracleData:   data,
		OracleOffset: offset,
	}
}

func NewPreimageOracleBlobData(key []byte, data []byte, offset uint32, fieldIndex uint64, commitment []byte, proof []byte) *PreimageOracleData {
	return &PreimageOracleData{
		IsLocal:        false,
		OracleKey:      key,
		oracleData:     data,
		OracleOffset:   offset,
		BlobFieldIndex: fieldIndex,
		BlobCommitment: commitment,
		BlobProof:      proof,
	}
}

// StepCallData encapsulates the data needed to perform a step.
type StepCallData struct {
	ClaimIndex uint64
	IsAttack   bool
	StateData  []byte
	Proof      []byte
}

// TraceAccessor defines an interface to request data from a TraceProvider with additional context for the game position.
// This can be used to implement split games where lower layers of the game may have different values depending on claims
// at higher levels in the game.
type TraceAccessor interface {
	// Get returns the claim value at the requested position, evaluated in the context of the specified claim (ref).
	Get(ctx context.Context, game Game, ref Claim, pos Position) (common.Hash, error)

	// GetStepData returns the data required to execute the step at the specified position,
	// evaluated in the context of the specified claim (ref).
	GetStepData(ctx context.Context, game Game, ref Claim, pos Position) (prestate []byte, proofData []byte, preimageData *PreimageOracleData, err error)

	// GetL2BlockNumberChallenge returns the data required to prove the correct L2 block number of the root claim.
	// Returns ErrL2BlockNumberValid if the root claim is known to come from the same block as the claimed L2 block.
	GetL2BlockNumberChallenge(ctx context.Context, game Game) (*InvalidL2BlockNumberChallenge, error)
}

// PrestateProvider defines an interface to request the absolute prestate.
type PrestateProvider interface {
	// AbsolutePreStateCommitment is the commitment of the pre-image value of the trace that transitions to the trace value at index 0
	AbsolutePreStateCommitment(ctx context.Context) (hash common.Hash, err error)
}

// TraceProvider is a generic way to get a claim value at a specific step in the trace.
type TraceProvider interface {
	PrestateProvider

	// Get returns the claim value at the requested index.
	// Get(i) = Keccak256(GetPreimage(i))
	Get(ctx context.Context, i Position) (common.Hash, error)

	// GetStepData returns the data required to execute the step at the specified trace index.
	// This includes the pre-state of the step (not hashed), the proof data required during step execution
	// and any pre-image data that needs to be loaded into the oracle prior to execution (may be nil)
	// The prestate returned from GetStepData for trace 10 should be the pre-image of the claim from trace 9
	GetStepData(ctx context.Context, i Position) (prestate []byte, proofData []byte, preimageData *PreimageOracleData, err error)

	// GetL2BlockNumberChallenge returns the data required to prove the correct L2 block number of the root claim.
	// Returns ErrL2BlockNumberValid if the root claim is known to come from the same block as the claimed L2 block.
	GetL2BlockNumberChallenge(ctx context.Context) (*InvalidL2BlockNumberChallenge, error)
}

// ClaimData is the core of a claim. It must be unique inside a specific game.
type ClaimData struct {
	Value common.Hash
	Bond  *big.Int
	Position
}

func (c *ClaimData) ValueBytes() [32]byte {
	responseBytes := c.Value.Bytes()
	var responseArr [32]byte
	copy(responseArr[:], responseBytes[:32])
	return responseArr
}

type ClaimID common.Hash

// Claim extends ClaimData with information about the relationship between two claims.
// It uses ClaimData to break cyclicity without using pointers.
// If the position of the game is Depth 0, IndexAtDepth 0 it is the root claim
// and the Parent field is empty & meaningless.
type Claim struct {
	ClaimData
	// WARN: CounteredBy is a mutable field in the FaultDisputeGame contract
	//       and rely on it for determining whether to step on leaf claims.
	//       When caching is implemented for the Challenger, this will need
	//       to be changed/removed to avoid invalid/stale contract state.
	CounteredBy common.Address
	Claimant    common.Address
	Clock       Clock
	// Location of the claim & it's parent inside the contract. Does not exist
	// for claims that have not made it to the contract.
	ContractIndex       int
	ParentContractIndex int
}

func (c Claim) ID() ClaimID {
	return ClaimID(crypto.Keccak256Hash(
		c.Position.ToGIndex().Bytes(),
		c.Value.Bytes(),
		big.NewInt(int64(c.ParentContractIndex)).Bytes(),
	))
}

// IsRoot returns true if this claim is the root claim.
func (c Claim) IsRoot() bool {
	return c.Position.IsRootPosition()
}

// Clock tracks the chess clock for a claim.
type Clock struct {
	// Duration is the time elapsed on the chess clock at the last update.
	Duration time.Duration

	// Timestamp is the time that the clock was last updated.
	Timestamp time.Time
}

// NewClock creates a new Clock instance.
func NewClock(duration time.Duration, timestamp time.Time) Clock {
	return Clock{
		Duration:  duration,
		Timestamp: timestamp,
	}
}

type InvalidL2BlockNumberChallenge struct {
	Output *eth.OutputResponse
	Header *ethTypes.Header
}

func NewInvalidL2BlockNumberProof(output *eth.OutputResponse, header *ethTypes.Header) *InvalidL2BlockNumberChallenge {
	return &InvalidL2BlockNumberChallenge{
		Output: output,
		Header: header,
	}
}

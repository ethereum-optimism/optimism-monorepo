package contracts

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/solabi"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

const (
	eventExecutingMessage = "ExecutingMessage"
)

var (
	ErrEventNotFound = errors.New("event not found")
)

type contractIdentifier struct {
	// Origin represents the address that initiated the message
	// it is used in combination with the MsgHash to uniquely identify a message
	// and is hashed into the log hash, not stored directly.
	Origin      common.Address
	LogIndex    *big.Int
	BlockNumber *big.Int
	ChainId     *big.Int
	Timestamp   *big.Int
}

type CrossL2Inbox struct {
	contract *batching.BoundContract
}

func NewCrossL2Inbox() *CrossL2Inbox {
	abi := snapshots.LoadCrossL2InboxABI()
	return &CrossL2Inbox{
		contract: batching.NewBoundContract(abi, predeploys.CrossL2InboxAddr),
	}
}

func (i *CrossL2Inbox) DecodeExecutingMessageLog(l *ethTypes.Log) (types.ExecutingMessage, error) {
	if l.Address != i.contract.Addr() {
		return types.ExecutingMessage{}, fmt.Errorf("%w: log not from CrossL2Inbox", ErrEventNotFound)
	}
	// use DecodeEvent to check the name of the event
	// but the actual decoding is done manually to extract the contract identifier
	name, _, err := i.contract.DecodeEvent(l)
	if errors.Is(err, batching.ErrUnknownEvent) {
		return types.ExecutingMessage{}, fmt.Errorf("%w: %v", ErrEventNotFound, err.Error())
	} else if err != nil {
		return types.ExecutingMessage{}, fmt.Errorf("failed to decode event: %w", err)
	}
	if name != eventExecutingMessage {
		return types.ExecutingMessage{}, fmt.Errorf("%w: event %v not an ExecutingMessage event", ErrEventNotFound, name)
	}
	// the second topic is the hash of the payload (the first is the event ID)
	msgHash := l.Topics[1]
	// the first 32 bytes of the data are the msgHash, so we skip them
	identifierBytes := bytes.NewReader(l.Data[32:])
	identifier, err := identifierFromBytes(identifierBytes)
	if err != nil {
		return types.ExecutingMessage{}, fmt.Errorf("failed to read contract identifier: %w", err)
	}
	chainID, err := types.ChainIDFromBig(identifier.ChainId).ToUInt32()
	if err != nil {
		return types.ExecutingMessage{}, fmt.Errorf("failed to convert chain ID %v to uint32: %w", identifier.ChainId, err)
	}
	hash := types.PayloadHashToLogHash(msgHash, identifier.Origin)
	return types.ExecutingMessage{
		Chain:     types.ChainIndex(chainID), // TODO(#11105): translate chain ID to chain index
		Hash:      hash,
		BlockNum:  identifier.BlockNumber.Uint64(),
		LogIdx:    uint32(identifier.LogIndex.Uint64()),
		Timestamp: identifier.Timestamp.Uint64(),
	}, nil
}

// identifierFromBytes reads a contract identifier from a byte stream.
// it follows the spec and matches the CrossL2Inbox.json definition,
// rather than relying on reflection, as that can be error-prone regarding struct ordering
func identifierFromBytes(identifierBytes io.Reader) (contractIdentifier, error) {
	origin, err := solabi.ReadAddress(identifierBytes)
	if err != nil {
		return contractIdentifier{}, fmt.Errorf("failed to read origin address: %w", err)
	}
	originAddr := common.BytesToAddress(origin[:])
	blockNumber, err := solabi.ReadUint256(identifierBytes)
	if err != nil {
		return contractIdentifier{}, fmt.Errorf("failed to read block number: %w", err)
	}
	logIndex, err := solabi.ReadUint256(identifierBytes)
	if err != nil {
		return contractIdentifier{}, fmt.Errorf("failed to read log index: %w", err)
	}
	timestamp, err := solabi.ReadUint256(identifierBytes)
	if err != nil {
		return contractIdentifier{}, fmt.Errorf("failed to read timestamp: %w", err)
	}
	chainID, err := solabi.ReadUint256(identifierBytes)
	if err != nil {
		return contractIdentifier{}, fmt.Errorf("failed to read chain ID: %w", err)
	}
	return contractIdentifier{
		Origin:      originAddr,
		BlockNumber: blockNumber,
		LogIndex:    logIndex,
		Timestamp:   timestamp,
		ChainId:     chainID,
	}, nil
}

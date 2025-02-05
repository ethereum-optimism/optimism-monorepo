package logs

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// searchCheckpoint is both a checkpoint for searching, as well as a checkpoint for sealing blocks.
type searchCheckpoint struct {
	blockNum uint64
	// seen logs *after* the seal of the mentioned block, i.e. not part of this block, but building on top of it.
	// There is at least one checkpoint per L2 block with logsSince == 0, i.e. the exact block boundary.
	logsSince uint32
	timestamp uint64
}

func newSearchCheckpoint(blockNum uint64, logsSince uint32, timestamp uint64) searchCheckpoint {
	return searchCheckpoint{
		blockNum:  blockNum,
		logsSince: logsSince,
		timestamp: timestamp,
	}
}

func newSearchCheckpointFromEntry(data Entry) (searchCheckpoint, error) {
	if data.Type() != TypeSearchCheckpoint {
		return searchCheckpoint{}, fmt.Errorf("%w: attempting to decode search checkpoint but was type %s", types.ErrDataCorruption, data.Type())
	}
	return searchCheckpoint{
		blockNum:  binary.LittleEndian.Uint64(data[1:9]),
		logsSince: binary.LittleEndian.Uint32(data[9:13]),
		timestamp: binary.LittleEndian.Uint64(data[13:21]),
	}, nil
}

// encode creates a checkpoint entry
// type 0: "search checkpoint" <type><uint64 block number: 8 bytes><uint32 logsSince count: 4 bytes><uint64 timestamp: 8 bytes> = 21 bytes
func (s searchCheckpoint) encode() Entry {
	var data Entry
	data[0] = uint8(TypeSearchCheckpoint)
	binary.LittleEndian.PutUint64(data[1:9], s.blockNum)
	binary.LittleEndian.PutUint32(data[9:13], s.logsSince)
	binary.LittleEndian.PutUint64(data[13:21], s.timestamp)
	return data
}

type canonicalHash struct {
	hash common.Hash
}

func newCanonicalHash(hash common.Hash) canonicalHash {
	return canonicalHash{hash: hash}
}

func newCanonicalHashFromEntry(data Entry) (canonicalHash, error) {
	if data.Type() != TypeCanonicalHash {
		return canonicalHash{}, fmt.Errorf("%w: attempting to decode canonical hash but was type %s", types.ErrDataCorruption, data.Type())
	}
	return newCanonicalHash(common.Hash(data[1:33])), nil
}

func (c canonicalHash) encode() Entry {
	var entry Entry
	entry[0] = uint8(TypeCanonicalHash)
	copy(entry[1:33], c.hash[:])
	return entry
}

type initiatingEvent struct {
	hasExecMsg bool
	logHash    common.Hash
}

func newInitiatingEventFromEntry(data Entry) (initiatingEvent, error) {
	if data.Type() != TypeInitiatingEvent {
		return initiatingEvent{}, fmt.Errorf("%w: attempting to decode initiating event but was type %s", types.ErrDataCorruption, data.Type())
	}
	flags := data[1]
	return initiatingEvent{
		hasExecMsg: flags&eventFlagHasExecutingMessage != 0,
		logHash:    common.Hash(data[2:34]),
	}, nil
}

func newInitiatingEvent(logHash common.Hash, hasExecMsg bool) initiatingEvent {
	return initiatingEvent{
		hasExecMsg: hasExecMsg,
		logHash:    logHash,
	}
}

// encode creates an initiating event entry
// type 2: "initiating event" <type><flags><event-hash: 20 bytes> = 22 bytes
func (i initiatingEvent) encode() Entry {
	var data Entry
	data[0] = uint8(TypeInitiatingEvent)
	flags := byte(0)
	if i.hasExecMsg {
		flags = flags | eventFlagHasExecutingMessage
	}
	data[1] = flags
	copy(data[2:34], i.logHash[:])
	return data
}

type executingLink struct {
	chain     uint32 // chain index, not a chain ID
	blockNum  uint64
	logIdx    uint32
	timestamp uint64
}

func newExecutingLink(msg types.ExecutingMessage) (executingLink, error) {
	if msg.LogIdx > 1<<24 {
		return executingLink{}, fmt.Errorf("log idx is too large (%v)", msg.LogIdx)
	}
	return executingLink{
		chain:     uint32(msg.Chain),
		blockNum:  msg.BlockNum,
		logIdx:    msg.LogIdx,
		timestamp: msg.Timestamp,
	}, nil
}

func newExecutingLinkFromEntry(data Entry) (executingLink, error) {
	if data.Type() != TypeExecutingLink {
		return executingLink{}, fmt.Errorf("%w: attempting to decode executing link but was type %s", types.ErrDataCorruption, data.Type())
	}
	timestamp := binary.LittleEndian.Uint64(data[16:24])
	return executingLink{
		chain:     binary.LittleEndian.Uint32(data[1:5]),
		blockNum:  binary.LittleEndian.Uint64(data[5:13]),
		logIdx:    uint32(data[13]) | uint32(data[14])<<8 | uint32(data[15])<<16,
		timestamp: timestamp,
	}, nil
}

// encode creates an executing link entry
// type 3: "executing link" <type><chain: 4 bytes><blocknum: 8 bytes><event index: 3 bytes><uint64 timestamp: 8 bytes> = 24 bytes
func (e executingLink) encode() Entry {
	var entry Entry
	entry[0] = uint8(TypeExecutingLink)
	binary.LittleEndian.PutUint32(entry[1:5], e.chain)
	binary.LittleEndian.PutUint64(entry[5:13], e.blockNum)

	entry[13] = byte(e.logIdx)
	entry[14] = byte(e.logIdx >> 8)
	entry[15] = byte(e.logIdx >> 16)

	binary.LittleEndian.PutUint64(entry[16:24], e.timestamp)
	return entry
}

type executingCheck struct {
	hash common.Hash
}

func newExecutingCheck(hash common.Hash) executingCheck {
	return executingCheck{hash: hash}
}

func newExecutingCheckFromEntry(data Entry) (executingCheck, error) {
	if data.Type() != TypeExecutingCheck {
		return executingCheck{}, fmt.Errorf("%w: attempting to decode executing check but was type %s", types.ErrDataCorruption, data.Type())
	}
	return newExecutingCheck(common.Hash(data[1:33])), nil
}

// encode creates an executing check entry
// type 4: "executing check" <type><event-hash: 32 bytes> = 33 bytes
func (e executingCheck) encode() Entry {
	var entry Entry
	entry[0] = uint8(TypeExecutingCheck)
	copy(entry[1:33], e.hash[:])
	return entry
}

type paddingEntry struct{}

// encoding of the padding entry
// type 5: "padding" <type><padding: 33 bytes> = 34 bytes
func (e paddingEntry) encode() Entry {
	var entry Entry
	entry[0] = uint8(TypePadding)
	return entry
}

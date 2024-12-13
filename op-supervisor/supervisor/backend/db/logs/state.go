package logs

import (
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/entrydb"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// logContext is a buffer on top of the DB,
// where blocks and logs can be applied to.
//
// Rules:
//
//		if entry_index % 256 == 0: must be type 0. For easy binary search.
//		else if end_of_block: also type 0.
//		else:
//		    after type 0: type 1
//		    after type 1: type 2 iff any event and space, otherwise type 0
//		    after type 2: type 3 iff executing, otherwise type 2 or 0
//		    after type 3: type 4
//		    after type 4: type 2 iff any event and space, otherwise type 0
//	     after type 5: any
//
// Type 0 can repeat: seal the block, then start a search checkpoint, then a single canonical hash.
// Type 0 may also be used as padding: type 2 only starts when it will not be interrupted by a search checkpoint.
//
// Types (<type> = 1 byte):
// type 0: "checkpoint" <type><uint64 block number: 8 bytes><uint32 logsSince count: 4 bytes><uint64 timestamp: 8 bytes> = 21 bytes
// type 1: "canonical hash" <type><parent blockhash: 32 bytes> = 33 bytes
// type 2: "initiating event" <type><event flags: 1 byte><event-hash: 32 bytes> = 34 bytes
// type 3: "executing link" <type><chain: 4 bytes><blocknum: 8 bytes><event index: 3 bytes><uint64 timestamp: 8 bytes> = 24 bytes
// type 4: "executing check" <type><event-hash: 32 bytes> = 33 bytes
// type 5: "padding" <type><padding: 33 bytes> = 34 bytes
// other types: future compat. E.g. for linking to L1, registering block-headers as a kind of initiating-event, tracking safe-head progression, etc.
//
// Right-pad each entry that is not 34 bytes.
//
// We insert a checkpoint for every search interval and block sealing event,
// and these may overlap as the same thing.
// Such seal has logsSince == 0, i.e. wrapping up the last block and starting a fresh list of logs.
//
// event-flags: each bit represents a boolean value, currently only two are defined
// * event-flags & 0x01 - true if the initiating event has an executing link that should follow. Allows detecting when the executing link failed to write.
// event-hash: H(origin, timestamp, payloadhash); enough to check identifier matches & payload matches.
type logContext struct {
	// next entry index, including the contents of `out`
	nextEntryIndex entrydb.EntryIdx

	// blockHash of the last sealed block.
	// A block is not considered sealed until we know its block hash.
	// While we process logs we keep the parent-block of said logs around as sealed block.
	blockHash common.Hash
	// blockNum of the last sealed block
	blockNum uint64
	// timestamp of the last sealed block
	timestamp uint64

	// number of logs since the last sealed block
	logsSince uint32

	// payload-hash of the log-event that was last processed. (may not be fully processed, see doneLog)
	logHash common.Hash

	// executing message that might exist for the current log event.
	// Might be incomplete; if !logDone while we already processed the initiating event,
	// then we know an executing message is still coming.
	execMsg *types.ExecutingMessage

	need EntryTypeFlag

	// buffer of entries not yet in the DB.
	// This is generated as objects are applied.
	// E.g. you can build multiple hypothetical blocks with log events on top of the state,
	// before flushing the entries to a DB.
	// However, no entries can be read from the DB while objects are being applied.
	out []Entry
}

func (l *logContext) NextIndex() entrydb.EntryIdx {
	return l.nextEntryIndex
}

// SealedBlock returns the block that we are building on top of, and if it is sealed.
func (l *logContext) SealedBlock() (hash common.Hash, num uint64, ok bool) {
	if !l.hasCompleteBlock() {
		return common.Hash{}, 0, false
	}
	return l.blockHash, l.blockNum, true
}

func (l *logContext) SealedTimestamp() (timestamp uint64, ok bool) {
	if !l.hasCompleteBlock() {
		return 0, false
	}
	return l.timestamp, true
}

func (l *logContext) hasCompleteBlock() bool {
	return !l.need.Any(FlagCanonicalHash)
}

func (l *logContext) hasIncompleteLog() bool {
	return l.need.Any(FlagInitiatingEvent | FlagExecutingLink | FlagExecutingCheck)
}

func (l *logContext) hasReadableLog() bool {
	return l.logsSince > 0 && !l.hasIncompleteLog()
}

// InitMessage returns the current initiating message, if any is available.
func (l *logContext) InitMessage() (hash common.Hash, logIndex uint32, ok bool) {
	if !l.hasReadableLog() {
		return common.Hash{}, 0, false
	}
	return l.logHash, l.logsSince - 1, true
}

// ExecMessage returns the current executing message, if any is available.
func (l *logContext) ExecMessage() *types.ExecutingMessage {
	if l.hasCompleteBlock() && l.hasReadableLog() && l.execMsg != nil {
		return l.execMsg
	}
	return nil
}

// ApplyEntry applies an entry on top of the current state.
func (l *logContext) ApplyEntry(entry Entry) error {
	// Wrap processEntry to add common useful error message info
	err := l.processEntry(entry)
	if err != nil {
		return fmt.Errorf("failed to process type %s entry at idx %d (%x): %w", entry.Type().String(), l.nextEntryIndex, entry[:], err)
	}
	return nil
}

// processEntry decodes and applies an entry to the state.
// Entries may not be applied if we are in the process of generating entries from objects.
// These outputs need to be flushed before inputs can be accepted.
func (l *logContext) processEntry(entry Entry) error {
	if len(l.out) != 0 {
		panic("can only apply without appending if the state is still empty")
	}
	switch entry.Type() {
	case TypeSearchCheckpoint:
		current, err := newSearchCheckpointFromEntry(entry)
		if err != nil {
			return err
		}
		l.blockNum = current.blockNum
		l.blockHash = common.Hash{}
		l.logsSince = current.logsSince
		l.timestamp = current.timestamp
		l.need.Add(FlagCanonicalHash)
		// Log data after the block we are sealing remains to be seen
		if l.logsSince == 0 {
			l.logHash = common.Hash{}
			l.execMsg = nil
		}
	case TypeCanonicalHash:
		if !l.need.Any(FlagCanonicalHash) {
			return errors.New("not ready for canonical hash entry, already sealed the last block")
		}
		canonHash, err := newCanonicalHashFromEntry(entry)
		if err != nil {
			return err
		}
		l.blockHash = canonHash.hash
		l.need.Remove(FlagCanonicalHash)
	case TypeInitiatingEvent:
		if !l.hasCompleteBlock() {
			return errors.New("did not complete block seal, cannot add log")
		}
		if l.hasIncompleteLog() {
			return errors.New("cannot process log before last log completes")
		}
		evt, err := newInitiatingEventFromEntry(entry)
		if err != nil {
			return err
		}
		l.execMsg = nil // clear the old state
		l.logHash = evt.logHash
		if evt.hasExecMsg {
			l.need.Add(FlagExecutingLink | FlagExecutingCheck)
		} else {
			l.logsSince += 1
		}
		l.need.Remove(FlagInitiatingEvent)
	case TypeExecutingLink:
		if !l.need.Any(FlagExecutingLink) {
			return errors.New("unexpected executing-link")
		}
		link, err := newExecutingLinkFromEntry(entry)
		if err != nil {
			return err
		}
		l.execMsg = &types.ExecutingMessage{
			Chain:     types.ChainIndex(link.chain), // TODO(#11105): translate chain ID to chain index
			BlockNum:  link.blockNum,
			LogIdx:    link.logIdx,
			Timestamp: link.timestamp,
			Hash:      common.Hash{}, // not known yet
		}
		l.need.Remove(FlagExecutingLink)
		l.need.Add(FlagExecutingCheck)
	case TypeExecutingCheck:
		if l.need.Any(FlagExecutingLink) {
			return errors.New("need executing link to be applied before the check part")
		}
		if !l.need.Any(FlagExecutingCheck) {
			return errors.New("unexpected executing check")
		}
		link, err := newExecutingCheckFromEntry(entry)
		if err != nil {
			return err
		}
		l.execMsg.Hash = link.hash
		l.need.Remove(FlagExecutingCheck)
		l.logsSince += 1
	case TypePadding:
		if l.need.Any(FlagPadding) {
			l.need.Remove(FlagPadding)
		} else {
			l.need.Remove(FlagPadding2)
		}
	default:
		return fmt.Errorf("unknown entry type: %s", entry.Type())
	}
	l.nextEntryIndex += 1
	return nil
}

// appendEntry add the entry to the output-buffer,
// and registers it as last processed entry type, and increments the next entry-index.
func (l *logContext) appendEntry(obj EntryObj) {
	entry := obj.encode()
	l.out = append(l.out, entry)
	l.nextEntryIndex += 1
}

// infer advances the logContext in cases where complex entries contain multiple implied entries
// eg. a SearchCheckpoint implies a CannonicalHash will follow
// this also handles inserting the searchCheckpoint at the set frequency, and padding entries
func (l *logContext) infer() error {
	// We force-insert a checkpoint whenever we hit the known fixed interval.
	if l.nextEntryIndex%searchCheckpointFrequency == 0 {
		l.need.Add(FlagSearchCheckpoint)
	}
	if l.need.Any(FlagSearchCheckpoint) {
		l.appendEntry(newSearchCheckpoint(l.blockNum, l.logsSince, l.timestamp))
		l.need.Add(FlagCanonicalHash) // always follow with a canonical hash
		l.need.Remove(FlagSearchCheckpoint)
		return nil
	}
	if l.need.Any(FlagCanonicalHash) {
		l.appendEntry(newCanonicalHash(l.blockHash))
		l.need.Remove(FlagCanonicalHash)
		return nil
	}
	if l.need.Any(FlagPadding) {
		l.appendEntry(paddingEntry{})
		l.need.Remove(FlagPadding)
		return nil
	}
	if l.need.Any(FlagPadding2) {
		l.appendEntry(paddingEntry{})
		l.need.Remove(FlagPadding2)
		return nil
	}
	if l.need.Any(FlagInitiatingEvent) {
		// If we are running out of space for log-event data,
		// write some checkpoints as padding, to pass the checkpoint.
		if l.execMsg != nil { // takes 3 total. Need to avoid the checkpoint.
			switch l.nextEntryIndex % searchCheckpointFrequency {
			case searchCheckpointFrequency - 1:
				l.need.Add(FlagPadding)
				return nil
			case searchCheckpointFrequency - 2:
				l.need.Add(FlagPadding | FlagPadding2)
				return nil
			}
		}
		evt := newInitiatingEvent(l.logHash, l.execMsg != nil)
		l.appendEntry(evt)
		l.need.Remove(FlagInitiatingEvent)
		if l.execMsg == nil {
			l.logsSince += 1
		}
		return nil
	}
	if l.need.Any(FlagExecutingLink) {
		link, err := newExecutingLink(*l.execMsg)
		if err != nil {
			return fmt.Errorf("failed to create executing link: %w", err)
		}
		l.appendEntry(link)
		l.need.Remove(FlagExecutingLink)
		return nil
	}
	if l.need.Any(FlagExecutingCheck) {
		l.appendEntry(newExecutingCheck(l.execMsg.Hash))
		l.need.Remove(FlagExecutingCheck)
		l.logsSince += 1
		return nil
	}
	return io.EOF
}

// inferFull advances the logContext until it cannot infer any more entries.
func (l *logContext) inferFull() error {
	for i := 0; i < 10; i++ {
		err := l.infer()
		if err == nil {
			continue
		}
		if err == io.EOF { // wrapped io.EOF does not count.
			return nil
		} else {
			return err
		}
	}
	panic("hit sanity limit")
}

// forceBlock force-overwrites the state, to match the given sealed block as starting point (excl)
func (l *logContext) forceBlock(upd eth.BlockID, timestamp uint64) error {
	if l.nextEntryIndex != 0 {
		return errors.New("can only bootstrap on top of an empty state")
	}
	l.blockHash = upd.Hash
	l.blockNum = upd.Number
	l.timestamp = timestamp
	l.logsSince = 0
	l.execMsg = nil
	l.logHash = common.Hash{}
	l.need = 0
	l.out = nil
	return l.inferFull() // apply to the state as much as possible
}

// SealBlock applies a block header on top of the current state.
// This seals the state; no further logs of this block may be added with ApplyLog.
func (l *logContext) SealBlock(parent common.Hash, upd eth.BlockID, timestamp uint64) error {
	// If we don't have any entries yet, allow any block to start things off
	if l.nextEntryIndex != 0 {
		if err := l.inferFull(); err != nil { // ensure we can start applying
			return err
		}
		if l.blockHash != parent {
			return fmt.Errorf("%w: cannot apply block %s (parent %s) on top of %s", types.ErrConflict, upd, parent, l.blockHash)
		}
		if l.blockHash != (common.Hash{}) && l.blockNum+1 != upd.Number {
			return fmt.Errorf("%w: cannot apply block %d on top of %d", types.ErrConflict, upd.Number, l.blockNum)
		}
		if l.timestamp > timestamp {
			return fmt.Errorf("%w: block timestamp %d must be equal or larger than current timestamp %d", types.ErrConflict, timestamp, l.timestamp)
		}
	}
	l.blockHash = upd.Hash
	l.blockNum = upd.Number
	l.timestamp = timestamp
	l.logsSince = 0
	l.execMsg = nil
	l.logHash = common.Hash{}
	l.need.Add(FlagSearchCheckpoint)
	return l.inferFull() // apply to the state as much as possible
}

// ApplyLog applies a log on top of the current state.
// The parent-block that the log comes after must be applied with ApplyBlock first.
func (l *logContext) ApplyLog(parentBlock eth.BlockID, logIdx uint32, logHash common.Hash, execMsg *types.ExecutingMessage) error {
	if parentBlock == (eth.BlockID{}) {
		return fmt.Errorf("genesis does not have logs: %w", types.ErrOutOfOrder)
	}
	if err := l.inferFull(); err != nil { // ensure we can start applying
		return err
	}
	if !l.hasCompleteBlock() {
		if l.blockNum == 0 {
			return fmt.Errorf("%w: should not have logs in block 0", types.ErrOutOfOrder)
		} else {
			return errors.New("cannot append log before last known block is sealed")
		}
	}
	// check parent block
	if l.blockHash != parentBlock.Hash {
		return fmt.Errorf("%w: log builds on top of block %s, but have block %s", types.ErrOutOfOrder, parentBlock, l.blockHash)
	}
	if l.blockNum != parentBlock.Number {
		return fmt.Errorf("%w: log builds on top of block %d, but have block %d", types.ErrOutOfOrder, parentBlock.Number, l.blockNum)
	}
	// check if log fits on top. The length so far == the index of the next log.
	if logIdx != l.logsSince {
		return fmt.Errorf("%w: expected event index %d, cannot append %d", types.ErrOutOfOrder, l.logsSince, logIdx)
	}
	l.logHash = logHash
	l.execMsg = execMsg
	l.need.Add(FlagInitiatingEvent)
	if execMsg != nil {
		l.need.Add(FlagExecutingLink | FlagExecutingCheck)
	}
	return l.inferFull() // apply to the state as much as possible
}

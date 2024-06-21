package db

import (
	"fmt"
	"io"
)

type iterator struct {
	db           *DB
	nextEntryIdx int64

	current logContext

	entriesRead int64
}

func (i *iterator) NextLog() (blockNum uint64, logIdx uint32, evtHash TruncatedHash, outErr error) {
	for i.nextEntryIdx <= i.db.lastEntryIdx {
		entryIdx := i.nextEntryIdx
		entry, err := i.db.readEntry(entryIdx)
		if err != nil {
			outErr = fmt.Errorf("failed to read entry %v: %w", i, err)
			return
		}
		i.nextEntryIdx++
		i.entriesRead++
		switch entry[0] {
		case typeSearchCheckpoint:
			current, err := parseSearchCheckpoint(entry)
			if err != nil {
				outErr = fmt.Errorf("failed to parse search checkpoint at idx %v: %w", entryIdx, err)
				return
			}
			i.current.blockNum = current.blockNum
			i.current.logIdx = current.logIdx
		case typeCanonicalHash:
			// Skip
		case typeInitiatingEvent:
			i.current, evtHash, err = parseInitiatingEvent(i.current, entry)
			if err != nil {
				outErr = fmt.Errorf("failed to parse initiating event at idx %v: %w", entryIdx, err)
				return
			}
			blockNum = i.current.blockNum
			logIdx = i.current.logIdx
			return
		case typeExecutingCheck:
		// TODO(optimism#10857): Handle this properly
		case typeExecutingLink:
		// TODO(optimism#10857): Handle this properly
		default:
			outErr = fmt.Errorf("unknown entry type at idx %v %v", entryIdx, entry[0])
			return
		}
	}
	outErr = io.EOF
	return
}

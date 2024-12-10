package batcher

import (
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive/params"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// txData represents the data for a single transaction.
//
// Note: The batcher currently sends exactly one frame per transaction. This
// might change in the future to allow for multiple frames from possibly
// different channels.
type txData struct {
	frames []frameData
	asBlob bool // indicates whether this should be sent as blob
}

func singleFrameTxData(frame frameData) txData {
	return txData{frames: []frameData{frame}}
}

// ID returns the id for this transaction data. Its String() can be used as a map key.
func (td *txData) ID() txID {
	id := make(txID, 0, len(td.frames))
	for _, f := range td.frames {
		id = append(id, f.id)
	}
	return id
}

// CallData returns the transaction data as calldata.
// It's a version byte (0) followed by the concatenated frames for this transaction.
func (td *txData) CallData() []byte {
	data := make([]byte, 1, 1+td.Len())
	data[0] = params.DerivationVersion0
	for _, f := range td.frames {
		data = append(data, f.data...)
	}
	return data
}

func (td *txData) Blobs() ([]*eth.Blob, error) {
	blobs := make([]*eth.Blob, 0, len(td.frames))
	for _, f := range td.frames {
		var blob eth.Blob
		if err := blob.FromData(append([]byte{params.DerivationVersion0}, f.data...)); err != nil {
			return nil, err
		}
		blobs = append(blobs, &blob)
	}
	return blobs, nil
}

// Len returns the sum of all the sizes of data in all frames.
// Len only counts the data itself and doesn't account for the version byte(s).
func (td *txData) Len() (l int) {
	for _, f := range td.frames {
		l += len(f.data)
	}
	return l
}

// Frames returns the single frame of this tx data.
func (td *txData) Frames() []frameData {
	return td.frames
}

// txID is an opaque identifier for a transaction.
// Its internal fields should not be inspected after creation & are subject to change.
// Its String() can be used for comparisons and works as a map key.
type txID []frameID

func (id txID) String() string {
	return id.string(func(id derive.ChannelID) string { return id.String() })
}

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (id txID) TerminalString() string {
	return id.string(func(id derive.ChannelID) string { return id.TerminalString() })
}

func (id txID) string(chIDStringer func(id derive.ChannelID) string) string {
	var (
		sb      strings.Builder
		curChID derive.ChannelID
	)
	for _, f := range id {
		if f.chID == curChID {
			sb.WriteString(fmt.Sprintf("+%d", f.frameNumber))
		} else {
			if curChID != (derive.ChannelID{}) {
				sb.WriteString("|")
			}
			curChID = f.chID
			sb.WriteString(fmt.Sprintf("%s:%d", chIDStringer(f.chID), f.frameNumber))
		}
	}
	return sb.String()
}

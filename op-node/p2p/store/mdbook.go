package store

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum/go-ethereum/log"
	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	mdCacheSize        = 100
	mdRecordExpiration = time.Hour * 24 * 7
)

var metadataBase = ds.NewKey("/peers/md")

// LastUpdate requires atomic update operations. Use the helper functions SetLastUpdated and LastUpdated to modify and access this field.
type metadataRecord struct {
	LastUpdate   int64        `json:"lastUpdate"` // unix timestamp in seconds
	PeerMetadata PeerMetadata `json:"peerMetadata"`
}

type PeerMetadata struct {
	ENR       string `json:"enr"`
	OPStackID uint64 `json:"opStackID"`
}

func (m *metadataRecord) SetLastUpdated(t time.Time) {
	atomic.StoreInt64(&m.LastUpdate, t.Unix())
}

func (m *metadataRecord) LastUpdated() time.Time {
	return time.Unix(atomic.LoadInt64(&m.LastUpdate), 0)
}

func (m *metadataRecord) MarshalBinary() (data []byte, err error) {
	return json.Marshal(m)
}

func (m *metadataRecord) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, m)
}

type metadataBook struct {
	mu   sync.RWMutex
	book *recordsBook[peer.ID, *metadataRecord]
}

func newMetadataRecord() *metadataRecord {
	return new(metadataRecord)
}

func newMetadataBook(ctx context.Context, logger log.Logger, clock clock.Clock, store ds.Batching) (*metadataBook, error) {
	book, err := newRecordsBook[peer.ID, *metadataRecord](ctx, logger, clock, store, mdCacheSize, mdRecordExpiration, metadataBase, genNew, peerIDKey)
	if err != nil {
		return nil, err
	}
	return &metadataBook{book: book}, nil
}

func (m *metadataBook) startGC() {
	m.book.startGC()
}

func (m *metadataBook) GetPeerMetadata(id peer.ID) (PeerMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, err := m.book.getRecord(id)
	// If the record is not found, return an empty PeerMetadata
	if errors.Is(err, errUnknownRecord) {
		return PeerMetadata{}, nil
	}
	if err != nil {
		return PeerMetadata{}, err
	}
	return record.PeerMetadata, nil
}

// Apply simply overwrites the record with the new one.
// presently, metadata is only collected during peering, so this is fine.
// if in the future this data can be updated or expanded, this function will need to be updated.
func (md *metadataRecord) Apply(rec *metadataRecord) {
	*rec = *md
}

func (m *metadataBook) SetPeerMetadata(id peer.ID, md PeerMetadata) (PeerMetadata, error) {
	rec := newMetadataRecord()
	rec.PeerMetadata = md
	rec.SetLastUpdated(m.book.clock.Now())
	m.mu.Lock()
	defer m.mu.Unlock()
	v, err := m.book.setRecord(id, rec)
	return v.PeerMetadata, err
}

func (m *metadataBook) Close() {
	m.book.Close()
}

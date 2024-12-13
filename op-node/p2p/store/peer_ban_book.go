package store

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum/go-ethereum/log"
	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	peerBanCacheSize        = 100
	peerBanRecordExpiration = time.Hour * 24 * 7
)

var peerBanExpirationsBase = ds.NewKey("/peers/ban_expiration")

type peerBanRecord struct {
	Expiry     int64 `json:"expiry"`     // unix timestamp in seconds
	LastUpdate int64 `json:"lastUpdate"` // unix timestamp in seconds
}

func (s *peerBanRecord) SetLastUpdated(t time.Time) {
	s.LastUpdate = t.Unix()
}

func (s *peerBanRecord) LastUpdated() time.Time {
	return time.Unix(s.LastUpdate, 0)
}

func (s *peerBanRecord) MarshalBinary() (data []byte, err error) {
	return json.Marshal(s)
}

func (s *peerBanRecord) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, s)
}

type peerBanUpdate time.Time

func (p peerBanUpdate) Apply(rec *peerBanRecord) {
	rec.Expiry = time.Time(p).Unix()
}

type peerBanBook struct {
	mu   sync.RWMutex
	book *recordsBook[peer.ID, *peerBanRecord]
}

func newPeerBanBook(ctx context.Context, logger log.Logger, clock clock.Clock, store ds.Batching) (*peerBanBook, error) {
	book, err := newRecordsBook[peer.ID, *peerBanRecord](ctx, logger, clock, store, peerBanCacheSize, peerBanRecordExpiration, peerBanExpirationsBase, genNew, peerIDKey)
	if err != nil {
		return nil, err
	}
	return &peerBanBook{book: book}, nil
}

func (d *peerBanBook) startGC() {
	d.book.startGC()
}

func (d *peerBanBook) GetPeerBanExpiration(id peer.ID) (time.Time, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rec, err := d.book.getRecord(id)
	if errors.Is(err, errUnknownRecord) {
		return time.Time{}, ErrUnknownBan
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(rec.Expiry, 0), nil
}

func (d *peerBanBook) SetPeerBanExpiration(id peer.ID, expirationTime time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if expirationTime == (time.Time{}) {
		return d.book.deleteRecord(id)
	}
	_, err := d.book.setRecord(id, peerBanUpdate(expirationTime))
	return err
}

func (d *peerBanBook) Close() {
	d.book.Close()
}

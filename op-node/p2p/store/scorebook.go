package store

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum/go-ethereum/log"
	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-base32"
)

const (
	scoreCacheSize = 100
)

var scoresBase = ds.NewKey("/peers/scores")

// LastUpdate requires atomic update operations. Use the helper functions SetLastUpdated and
// LastUpdated to modify and access this field.
type scoreRecord struct {
	LastUpdate int64      `json:"lastUpdate"` // unix timestamp in seconds
	PeerScores PeerScores `json:"peerScores"`
}

func (s *scoreRecord) SetLastUpdated(t time.Time) {
	atomic.StoreInt64(&s.LastUpdate, t.Unix())
}

func (s *scoreRecord) LastUpdated() time.Time {
	return time.Unix(atomic.LoadInt64(&s.LastUpdate), 0)
}

func (s *scoreRecord) MarshalBinary() (data []byte, err error) {
	return serializeScoresV0(*s)
}

func (s *scoreRecord) UnmarshalBinary(data []byte) error {
	r, err := deserializeScoresV0(data)
	if err != nil {
		return err
	}
	*s = r
	return nil
}

type scoreBook struct {
	mu   sync.RWMutex
	book *recordsBook[peer.ID, *scoreRecord]
}

func peerIDKey(id peer.ID) ds.Key {
	return ds.NewKey(base32.RawStdEncoding.EncodeToString([]byte(id)))
}

func newScoreBook(ctx context.Context, logger log.Logger, clock clock.Clock, store ds.Batching, retain time.Duration) (*scoreBook, error) {
	book, err := newRecordsBook[peer.ID, *scoreRecord](ctx, logger, clock, store, scoreCacheSize, retain, scoresBase, genNew, peerIDKey)
	if err != nil {
		return nil, err
	}
	return &scoreBook{book: book}, nil
}

func (d *scoreBook) startGC() {
	d.book.startGC()
}

func (d *scoreBook) GetPeerScores(id peer.ID) (PeerScores, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	record, err := d.book.getRecord(id)
	if errors.Is(err, errUnknownRecord) {
		return PeerScores{}, nil // return zeroed scores by default
	}
	if err != nil {
		return PeerScores{}, err
	}
	return record.PeerScores, nil
}

func (d *scoreBook) GetPeerScore(id peer.ID) (float64, error) {
	scores, err := d.GetPeerScores(id)
	if err != nil {
		return 0, err
	}
	return scores.Gossip.Total, nil
}

func (d *scoreBook) SetScore(id peer.ID, diff ScoreDiff) (PeerScores, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v, err := d.book.setRecord(id, diff)
	return v.PeerScores, err
}

func (d *scoreBook) Close() {
	d.book.Close()
}

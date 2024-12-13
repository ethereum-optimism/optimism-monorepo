package store

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru/v2"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

const (
	maxPruneBatchSize = 20
)

type record interface {
	SetLastUpdated(time.Time)
	LastUpdated() time.Time
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

type recordDiff[V record] interface {
	Apply(v V)
}

var errUnknownRecord = errors.New("unknown record")

func genNew[T any]() *T {
	return new(T)
}

// recordsBook is a generic K-V store to embed in the extended-peerstore.
// It prunes old entries to keep the store small.
// The recordsBook can be wrapped to customize typing and introduce synchronization.
type recordsBook[K ~string, V record] struct {
	ctx          context.Context
	cancelFn     context.CancelFunc
	clock        clock.Clock
	log          log.Logger
	bgTasks      sync.WaitGroup
	store        ds.Batching
	cache        *lru.Cache[K, V]
	newRecord    func() V
	dsBaseKey    ds.Key
	dsEntryKey   func(K) ds.Key
	recordExpiry time.Duration // pruning is disabled if this is 0
}

func newRecordsBook[K ~string, V record](ctx context.Context, logger log.Logger, clock clock.Clock, store ds.Batching, cacheSize int, recordExpiry time.Duration,
	dsBaseKey ds.Key, newRecord func() V, dsEntryKey func(K) ds.Key) (*recordsBook[K, V], error) {
	cache, err := lru.New[K, V](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create records cache: %w", err)
	}

	ctx, cancelFn := context.WithCancel(ctx)
	book := &recordsBook[K, V]{
		ctx:          ctx,
		cancelFn:     cancelFn,
		clock:        clock,
		log:          logger,
		store:        store,
		cache:        cache,
		newRecord:    newRecord,
		dsBaseKey:    dsBaseKey,
		dsEntryKey:   dsEntryKey,
		recordExpiry: recordExpiry,
	}
	return book, nil
}

func (d *recordsBook[K, V]) startGC() {
	if d.recordExpiry == 0 {
		return
	}
	startGc(d.ctx, d.log, d.clock, &d.bgTasks, d.prune)
}

func (d *recordsBook[K, V]) dsKey(key K) ds.Key {
	return d.dsBaseKey.Child(d.dsEntryKey(key))
}

func (d *recordsBook[K, V]) deleteRecord(key K) error {
	// If access to this isn't synchronized, removing from the cache first can result in the stored
	// item being cached again before it is deleted.
	err := d.store.Delete(d.ctx, d.dsKey(key))
	d.cache.Remove(key)
	if err == nil || errors.Is(err, ds.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("failed to delete entry with key %v: %w", key, err)
}

// You must read lock the recordsBook before calling this, and only unlock when you have extracted
// the values you want from the value of type V. There's no way to conveniently pass an extractor
// function parameterized on V here without breaking this out into a top-level function.
func (d *recordsBook[K, V]) getRecord(key K) (v V, err error) {
	if val, ok := d.cache.Get(key); ok {
		if d.hasExpired(val) {
			return v, errUnknownRecord
		}
		return val, nil
	}
	data, err := d.store.Get(d.ctx, d.dsKey(key))
	if errors.Is(err, ds.ErrNotFound) {
		return v, errUnknownRecord
	} else if err != nil {
		return v, fmt.Errorf("failed to load value of key %v: %w", key, err)
	}
	v = d.newRecord()
	if err := v.UnmarshalBinary(data); err != nil {
		return v, fmt.Errorf("invalid value for key %v: %w", key, err)
	}
	if d.hasExpired(v) {
		return v, errUnknownRecord
	}
	// This is safe with a read lock as it's self-synchronized.
	d.cache.Add(key, v)
	return v, nil
}

// You should lock the records book before calling this, and unlock it when you copy any values out
// of the returned value.
func (d *recordsBook[K, V]) setRecord(key K, diff recordDiff[V]) (V, error) {
	rec, err := d.getRecord(key)
	if errors.Is(err, errUnknownRecord) { // instantiate new record if it does not exist yet
		rec = d.newRecord()
	} else if err != nil {
		return d.newRecord(), err
	}
	rec.SetLastUpdated(d.clock.Now())
	diff.Apply(rec)
	data, err := rec.MarshalBinary()
	if err != nil {
		return d.newRecord(), fmt.Errorf("failed to encode record for key %v: %w", key, err)
	}
	err = d.store.Put(d.ctx, d.dsKey(key), data)
	if err != nil {
		return d.newRecord(), fmt.Errorf("storing updated record for key %v: %w", key, err)
	}
	d.cache.Add(key, rec)
	return rec, nil
}

// prune deletes entries from the store that are older than the configured prune expiration.
// Entries that are eligible for deletion may still be present either because the prune function hasn't yet run or
// because they are still preserved in the in-memory cache after having been deleted from the database.
// Such expired entries are filtered out in getRecord
func (d *recordsBook[K, V]) prune() error {
	results, err := d.store.Query(d.ctx, query.Query{
		Prefix: d.dsBaseKey.String(),
	})
	if err != nil {
		return err
	}
	pending := 0
	batch, err := d.store.Batch(d.ctx)
	if err != nil {
		return err
	}
	for result := range results.Next() {
		// Bail out if the context is done
		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		default:
		}
		v := d.newRecord()
		if err := v.UnmarshalBinary(result.Value); err != nil {
			return err
		}
		if d.hasExpired(v) {
			if pending > maxPruneBatchSize {
				if err := batch.Commit(d.ctx); err != nil {
					return err
				}
				batch, err = d.store.Batch(d.ctx)
				if err != nil {
					return err
				}
				pending = 0
			}
			pending++
			if err := batch.Delete(d.ctx, ds.NewKey(result.Key)); err != nil {
				return err
			}
		}
	}
	if err := batch.Commit(d.ctx); err != nil {
		return err
	}
	return nil
}

func (d *recordsBook[K, V]) hasExpired(v V) bool {
	return v.LastUpdated().Add(d.recordExpiry).Before(d.clock.Now())
}

func (d *recordsBook[K, V]) Close() {
	d.cancelFn()
	d.bgTasks.Wait()
}

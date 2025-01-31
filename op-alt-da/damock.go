package altda

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/log"
)

// MockDAClient mocks a DA storage provider to avoid running an HTTP DA server
// in unit tests. MockDAClient is goroutine-safe.
type MockDAClient struct {
	mu                     sync.Mutex
	CommitmentType         CommitmentType
	GenericCommitmentCount uint16 // next generic commitment (use counting commitment instead of hash to help with testing)
	store                  ethdb.KeyValueStore
	StoreCount             int
	log                    log.Logger
	dropEveryNthPut        uint // 0 means nothing gets dropped, 1 means every put errors, etc.
	setInputRequestCount   uint // number of put requests received, irrespective of whether they were successful
}

func NewMockDAClient(log log.Logger) *MockDAClient {
	return &MockDAClient{
		CommitmentType: Keccak256CommitmentType,
		store:          memorydb.New(),
		StoreCount:     0,
		log:            log,
	}
}

// NewCountingGenericCommitmentMockDAClient creates a MockDAClient that uses counting commitments.
// It's commitments are big-endian encoded uint16s of 0, 1, 2, etc. instead of actual hash or altda-layer related commitments.
// Used for testing to make sure we receive commitments in order following Holocene strict ordering rules.
func NewCountingGenericCommitmentMockDAClient(log log.Logger) *MockDAClient {
	return &MockDAClient{
		CommitmentType:         GenericCommitmentType,
		GenericCommitmentCount: 0,
		store:                  memorydb.New(),
		StoreCount:             0,
		log:                    log,
	}
}

// Fakes a da server that drops/errors on every Nth put request.
// Useful for testing the batcher's error handling.
// 0 means nothing gets dropped, 1 means every put errors, etc.
func (c *MockDAClient) DropEveryNthPut(n uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropEveryNthPut = n
}

func (c *MockDAClient) GetInput(ctx context.Context, key CommitmentData) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.log.Debug("Getting input", "key", key)
	bytes, err := c.store.Get(key.Encode())
	if err != nil {
		return nil, ErrNotFound
	}
	return bytes, nil
}

func (c *MockDAClient) SetInput(ctx context.Context, data []byte) (CommitmentData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setInputRequestCount++
	var key CommitmentData
	if c.CommitmentType == GenericCommitmentType {
		countCommitment := make([]byte, 2)
		binary.BigEndian.PutUint16(countCommitment, c.GenericCommitmentCount)
		key = NewGenericCommitment(countCommitment)
	} else {
		key = NewKeccak256Commitment(data)
	}
	var action string = "put"
	if c.dropEveryNthPut > 0 && c.setInputRequestCount%c.dropEveryNthPut == 0 {
		action = "dropped"
	}
	c.log.Debug("Setting input", "action", action, "key", key, "data", fmt.Sprintf("%x", data))
	if action == "dropped" {
		return nil, errors.New("put dropped")
	}
	err := c.store.Put(key.Encode(), data)
	if err == nil {
		c.GenericCommitmentCount++
		c.StoreCount++
	}
	return key, err
}

func (c *MockDAClient) DeleteData(key []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.log.Debug("Deleting data", "key", key)
	err := c.store.Delete(key)
	if err == nil {
		c.StoreCount--
	}
	return err
}

type DAErrFaker struct {
	Client *MockDAClient

	getInputErr error
	setInputErr error
}

func (f *DAErrFaker) GetInput(ctx context.Context, key CommitmentData) ([]byte, error) {
	if err := f.getInputErr; err != nil {
		f.getInputErr = nil
		return nil, err
	}
	return f.Client.GetInput(ctx, key)
}

func (f *DAErrFaker) SetInput(ctx context.Context, data []byte) (CommitmentData, error) {
	if err := f.setInputErr; err != nil {
		f.setInputErr = nil
		return nil, err
	}
	return f.Client.SetInput(ctx, data)
}

func (f *DAErrFaker) ActGetPreImageFail() {
	f.getInputErr = errors.New("get input failed")
}

func (f *DAErrFaker) ActSetPreImageFail() {
	f.setInputErr = errors.New("set input failed")
}

var Disabled = &AltDADisabled{}

var ErrNotEnabled = errors.New("altDA not enabled")

// AltDADisabled is a noop AltDA implementation for stubbing.
type AltDADisabled struct{}

func (d *AltDADisabled) GetInput(ctx context.Context, l1 L1Fetcher, commitment CommitmentData, blockId eth.L1BlockRef) (eth.Data, error) {
	return nil, ErrNotEnabled
}

func (d *AltDADisabled) Reset(ctx context.Context, base eth.L1BlockRef, baseCfg eth.SystemConfig) error {
	return io.EOF
}

func (d *AltDADisabled) Finalize(ref eth.L1BlockRef) {
}

func (d *AltDADisabled) OnFinalizedHeadSignal(f HeadSignalFn) {
}

func (d *AltDADisabled) AdvanceL1Origin(ctx context.Context, l1 L1Fetcher, blockId eth.BlockID) error {
	return ErrNotEnabled
}

// FakeDAServer is a fake DA server for e2e tests.
// It is a small wrapper around DAServer that allows for setting request latencies,
// to mimic a DA service with slow responses (eg. eigenDA with 10 min batching interval).
type FakeDAServer struct {
	*DAServer
	putRequestLatency time.Duration
	getRequestLatency time.Duration
	// outOfOrderResponses is a flag that, when set, causes the server to send responses out of order.
	// It will only respond to pairs of request, returning the second response first, and waiting 1 second before sending the first response.
	// This is used to test the batcher's ability to handle out of order responses, while still ensuring holocene's strict ordering rules.
	outOfOrderResponses bool
	oooMu               sync.Mutex
	oooWaitChan         chan struct{}
}

func NewFakeDAServer(host string, port int, log log.Logger) *FakeDAServer {
	store := NewMemStore()
	fakeDAServer := &FakeDAServer{
		DAServer:          NewDAServer(host, port, store, log, true),
		putRequestLatency: 0,
		getRequestLatency: 0,
	}
	return fakeDAServer
}

func (s *FakeDAServer) HandleGet(w http.ResponseWriter, r *http.Request) {
	time.Sleep(s.getRequestLatency)
	s.DAServer.HandleGet(w, r)
}

func (s *FakeDAServer) HandlePut(w http.ResponseWriter, r *http.Request) {
	time.Sleep(s.putRequestLatency)
	if s.outOfOrderResponses {
		s.oooMu.Lock()
		if s.oooWaitChan == nil {
			s.log.Info("Received put request while in out-of-order mode, waiting for next request")
			s.oooWaitChan = make(chan struct{})
			s.oooMu.Unlock()
			<-s.oooWaitChan
			time.Sleep(1 * time.Second)
		} else {
			s.log.Info("Received second put request in out-of-order mode, responding to this one first, then the first one")
			close(s.oooWaitChan)
			s.oooWaitChan = nil
			s.oooMu.Unlock()
		}
	}
	s.DAServer.HandlePut(w, r)
}

func (s *FakeDAServer) Start() error {
	err := s.DAServer.Start()
	if err != nil {
		return err
	}
	// Override the HandleGet/Put method registrations
	mux := http.NewServeMux()
	mux.HandleFunc("/get/", s.HandleGet)
	mux.HandleFunc("/put", s.HandlePut)
	s.httpServer.Handler = mux
	return nil
}

func (s *FakeDAServer) SetPutRequestLatency(latency time.Duration) {
	s.log.Info("Setting put request latency", "latency", latency)
	s.putRequestLatency = latency
}

func (s *FakeDAServer) SetGetRequestLatency(latency time.Duration) {
	s.log.Info("Setting get request latency", "latency", latency)
	s.getRequestLatency = latency
}

// When ooo=true, causes the server to send responses out of order.
// It will only respond to pairs of request, returning the second response first, and waiting 1 second before sending the first response.
// This is used to test the batcher's ability to handle out of order responses, while still ensuring holocene's strict ordering rules.
func (s *FakeDAServer) SetOutOfOrderResponses(ooo bool) {
	s.log.Info("Setting out of order responses", "ooo", ooo)
	s.outOfOrderResponses = ooo
}

type MemStore struct {
	db   map[string][]byte
	lock sync.RWMutex
}

func NewMemStore() *MemStore {
	return &MemStore{
		db: make(map[string][]byte),
	}
}

// Get retrieves the given key if it's present in the key-value store.
func (s *MemStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if entry, ok := s.db[string(key)]; ok {
		return common.CopyBytes(entry), nil
	}
	return nil, ErrNotFound
}

// Put inserts the given value into the key-value store.
func (s *MemStore) Put(ctx context.Context, key []byte, value []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.db[string(key)] = common.CopyBytes(value)
	return nil
}

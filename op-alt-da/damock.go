package altda

import (
	"context"
	"errors"
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
// in unit tests.
type MockDAClient struct {
	CommitmentType CommitmentType
	store          ethdb.KeyValueStore
	log            log.Logger
}

func NewMockDAClient(log log.Logger) *MockDAClient {
	return &MockDAClient{
		CommitmentType: Keccak256CommitmentType,
		store:          memorydb.New(),
		log:            log,
	}
}

func (c *MockDAClient) GetInput(ctx context.Context, key CommitmentData, l1BlockNum uint64) ([]byte, error) {
	bytes, err := c.store.Get(key.Encode())
	if err != nil {
		return nil, ErrNotFound
	}
	return bytes, nil
}

func (c *MockDAClient) SetInput(ctx context.Context, data []byte) (CommitmentData, error) {
	key := NewCommitmentData(c.CommitmentType, data)
	return key, c.store.Put(key.Encode(), data)
}

func (c *MockDAClient) DeleteData(key []byte) error {
	return c.store.Delete(key)
}

type DAErrFaker struct {
	Client *MockDAClient

	getInputErr error
	setInputErr error
}

func (f *DAErrFaker) GetInput(ctx context.Context, key CommitmentData, l1BlockNum uint64) ([]byte, error) {
	if err := f.getInputErr; err != nil {
		f.getInputErr = nil
		return nil, err
	}
	return f.Client.GetInput(ctx, key, l1BlockNum)
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
	s.putRequestLatency = latency
}

func (s *FakeDAServer) SetGetRequestLatency(latency time.Duration) {
	s.getRequestLatency = latency
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

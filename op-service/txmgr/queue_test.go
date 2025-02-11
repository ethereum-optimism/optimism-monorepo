package txmgr

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/txmgr/metrics"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type queueFunc func(id int, candidate TxCandidate, receiptCh chan TxReceipt[int], q *Queue[int]) bool

func sendQueueFunc(id int, candidate TxCandidate, receiptCh chan TxReceipt[int], q *Queue[int]) bool {
	q.Send(id, candidate, receiptCh)
	return true
}

func trySendQueueFunc(id int, candidate TxCandidate, receiptCh chan TxReceipt[int], q *Queue[int]) bool {
	return q.TrySend(id, candidate, receiptCh)
}

type queueCall struct {
	call   queueFunc // queue call (either Send or TrySend, use function helpers above)
	queued bool      // true if the send was queued
	txErr  bool      // true if the tx send should return an error
}

type testTx struct {
	sendErr bool // error to return from send for this tx
}

type mockBackendWithNonce struct {
	mockBackend
}

func newMockBackendWithNonce(g *gasPricer) *mockBackendWithNonce {
	return &mockBackendWithNonce{
		mockBackend: mockBackend{
			g:        g,
			minedTxs: make(map[common.Hash]minedTxInfo),
		},
	}
}

func (b *mockBackendWithNonce) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return uint64(len(b.minedTxs)), nil
}

func TestQueue_Send(t *testing.T) {
	testCases := []struct {
		name   string      // name of the test
		max    uint64      // max concurrency of the queue
		calls  []queueCall // calls to the queue
		txs    []testTx    // txs to generate from the factory (and potentially error in send)
		nonces []uint64    // expected sent tx nonces after all calls are made
		// With Holocene, it is important that transactions are included on chain in the same order as they are sent.
		// The txmgr.Queue.Send() method should ensure nonces are determined _synchronously_ even if transactions
		// are otherwise launched asynchronously.
		confirmedIds []uint // expected tx Ids after all calls are made
	}{
		{
			name: "success",
			max:  5,
			calls: []queueCall{
				{call: trySendQueueFunc, queued: true},
				{call: trySendQueueFunc, queued: true},
			},
			txs: []testTx{
				{},
				{},
			},
			nonces:       []uint64{0, 1},
			confirmedIds: []uint{0, 1},
		},
		{
			name: "no limit",
			max:  0,
			calls: []queueCall{
				{call: trySendQueueFunc, queued: true},
				{call: trySendQueueFunc, queued: true},
			},
			txs: []testTx{
				{},
				{},
			},
			nonces:       []uint64{0, 1},
			confirmedIds: []uint{0, 1},
		},
		{
			name: "single threaded",
			max:  1,
			calls: []queueCall{
				{call: trySendQueueFunc, queued: true},
				{call: trySendQueueFunc, queued: false},
				{call: trySendQueueFunc, queued: false},
			},
			txs: []testTx{
				{},
			},
			nonces:       []uint64{0},
			confirmedIds: []uint{0},
		},
		{
			name: "single threaded blocking",
			max:  1,
			calls: []queueCall{
				{call: trySendQueueFunc, queued: true},
				{call: trySendQueueFunc, queued: false},
				{call: sendQueueFunc, queued: true},
				{call: sendQueueFunc, queued: true},
			},
			txs: []testTx{
				{},
				{},
				{},
			},
			nonces:       []uint64{0, 1, 2},
			confirmedIds: []uint{0, 2, 3},
		},
		{
			name: "dual threaded blocking",
			max:  2,
			calls: []queueCall{
				{call: trySendQueueFunc, queued: true},
				{call: trySendQueueFunc, queued: true},
				{call: trySendQueueFunc, queued: false},
				{call: sendQueueFunc, queued: true},
				{call: sendQueueFunc, queued: true},
				{call: sendQueueFunc, queued: true},
			},
			txs: []testTx{
				{},
				{},
				{},
				{},
				{},
			},
			nonces:       []uint64{0, 1, 2, 3, 4},
			confirmedIds: []uint{0, 1, 3, 4, 5},
		},
		{
			name: "subsequent txs fail after tx failure",
			max:  1,
			calls: []queueCall{
				{call: sendQueueFunc, queued: true},
				{call: sendQueueFunc, queued: true, txErr: true},
				{call: sendQueueFunc, queued: true, txErr: true},
			},
			txs: []testTx{
				{},
				{sendErr: true},
				{},
			},
			nonces:       []uint64{0, 1},
			confirmedIds: []uint{0},
		},
	}
	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			conf := configWithNumConfs(1)
			conf.ReceiptQueryInterval = 1 * time.Second            // simulate a network send
			conf.ResubmissionTimeout.Store(int64(2 * time.Second)) // resubmit to detect errors
			conf.SafeAbortNonceTooLowCount = 1
			backend := newMockBackendWithNonce(newGasPricer(3))
			mgr := &SimpleTxManager{
				chainID: conf.ChainID,
				name:    "TEST",
				cfg:     conf,
				backend: backend,
				l:       testlog.Logger(t, log.LevelCrit),
				metr:    &metrics.NoopTxMetrics{},
			}

			// track the nonces, and return any expected errors from tx sending
			var (
				nonces       []uint64
				nonceForTxId map[uint]uint64 // maps from txid to nonce
				nonceMu      sync.Mutex
			)
			nonceForTxId = make(map[uint]uint64)
			sendTx := func(ctx context.Context, tx *types.Transaction) error {
				index := int(tx.Data()[0])
				nonceMu.Lock()
				nonces = append(nonces, tx.Nonce())
				nonceMu.Unlock()
				var testTx *testTx
				if index < len(test.txs) {
					testTx = &test.txs[index]
				}
				if testTx != nil && testTx.sendErr {
					return core.ErrNonceTooLow
				}

				txHash := tx.Hash()
				nonceMu.Lock()
				backend.mine(&txHash, tx.GasFeeCap(), nil)
				nonceForTxId[uint(index)] = tx.Nonce()
				nonceMu.Unlock()
				return nil
			}
			backend.setTxSender(sendTx)

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			queue := NewQueue[int](ctx, mgr, test.max)

			// make all the queue calls given in the test case
			receiptChs := make([]chan TxReceipt[int], len(test.calls))
			for i, c := range test.calls {
				msg := fmt.Sprintf("Call %d", i)
				candidate := TxCandidate{
					TxData: []byte{byte(i)},
					To:     &common.Address{},
				}
				if i == 0 {
					// Make the first tx much larger to expose
					// any race conditions in the queue
					candidate.TxData = make([]byte, 100_000)
				}
				receiptChs[i] = make(chan TxReceipt[int], 1)
				queued := c.call(i, candidate, receiptChs[i], queue)
				require.Equal(t, c.queued, queued, msg)
			}
			// wait for the queue to drain (all txs complete or failed)
			_ = queue.Wait()

			// NOTE the backend in this test does not order transactions based on the nonce
			// So what we want to check is that the txs match expectations when they are ordered
			// in the same way as the nonces.
			slices.Sort(nonces)
			require.Equal(t, test.nonces, nonces, "expected nonces do not match")
			for i, id := range test.confirmedIds {
				require.Equal(t, nonces[i], nonceForTxId[id],
					"nonce for tx id %d was %d instead of %d", id, nonceForTxId[id], nonces[i])
			}

			// check receipts
			for i, c := range test.calls {
				if !c.queued {
					// non-queued txs won't have a tx result
					continue
				}
				msg := fmt.Sprintf("Receipt %d", i)
				r := <-receiptChs[i]
				if c.txErr {
					require.Error(t, r.Err, msg)
				} else {
					require.NoError(t, r.Err, msg)
				}
			}
		})
	}
}

// mockBackendWithConfirmationDelay is a mock backend that delays the confirmation of transactions
type mockBackendWithConfirmationDelay struct {
	mockBackend
	cachedTxs map[common.Hash]*types.Transaction
}

// newMockBackendWithConfirmationDelay creates a new mock backend with a confirmation delay. It accepts
// a waitGroup which will be decremented when a transaction is sent.
func newMockBackendWithConfirmationDelay(g *gasPricer, wg *sync.WaitGroup) *mockBackendWithConfirmationDelay {
	b := &mockBackendWithConfirmationDelay{}
	b.cachedTxs = make(map[common.Hash]*types.Transaction)
	b.minedTxs = make(map[common.Hash]minedTxInfo)
	b.g = g

	sendTx := func(ctx context.Context, tx *types.Transaction) error {
		_, exists := b.cachedTxs[tx.Hash()]
		if !exists {
			b.cachedTxs[tx.Hash()] = tx
			wg.Done()
		}
		return nil
	}
	b.setTxSender(sendTx)

	return b
}

// MineAll mines all transactions in the cache.
func (b *mockBackendWithConfirmationDelay) MineAll() {
	for hash, tx := range b.cachedTxs {
		b.mine(&hash, tx.GasFeeCap(), nil)
	}
}

// Simple test that we can call q.Send() up to the maxPending limit without blocking.
func TestQueue_Send_MaxPendingMetrics(t *testing.T) {
	maxPending := 5

	// boilerplate setup
	wg := sync.WaitGroup{}
	backend := newMockBackendWithConfirmationDelay(newGasPricer(3), &wg)
	metrics := metrics.FakeTxMetrics{}
	conf := configWithNumConfs(1)
	conf.Backend = backend
	conf.NetworkTimeout = 1 * time.Second
	conf.ChainID = big.NewInt(1)
	mgr, err := NewSimpleTxManagerFromConfig("TEST", testlog.Logger(t, log.LevelDebug), &metrics, conf)
	require.NoError(t, err)

	// Construct queue with maxPending limit, mocks and fakes
	q := NewQueue[int](context.Background(), mgr, uint64(maxPending))

	// Send maxPending transactions
	for nonce := 0; nonce < maxPending; nonce++ {
		wg.Add(1) // Allows us to wait for this transaction to be cached by the backend
		q.Send(nonce, TxCandidate{}, make(chan TxReceipt[int], 1))
	}

	// Check that all of the transactions are pending
	require.EqualValues(t, maxPending, metrics.PendingTxs())

	// Wait for the backend to cache all of the transactions
	wg.Wait()

	// Mine the transactions (should cause the pending transactions to drop to 0)
	backend.MineAll()
	require.Eventually(t, func() bool {
		t.Log("Pending txs", metrics.PendingTxs())
		return metrics.PendingTxs() == 0
	}, 5*time.Second, 1*time.Second, "PendingTxs metric should drop to 0 after all transactions are mined")
}

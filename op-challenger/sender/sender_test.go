package sender

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

func TestSendAndWaitQueueWithMaxPending(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	txMgr := &stubTxMgr{sending: make(map[byte]chan *types.Receipt)}
	sender := NewTxSender(ctx, testlog.Logger(t, log.LevelInfo), txMgr, 5)

	tx := func(i byte) txmgr.TxCandidate {
		return txmgr.TxCandidate{TxData: []byte{i}}
	}

	sendAsync := func(txs ...txmgr.TxCandidate) chan []txmgr.TxCandidate {
		ch := make(chan []txmgr.TxCandidate, 1)
		go func() {
			err := sender.SendAndWaitSimple("testing", txs...)
			require.NoError(t, err)
			ch <- txs
			close(ch)
		}()
		return ch
	}

	wait := func(ch chan []txmgr.TxCandidate) []txmgr.TxCandidate {
		select {
		case rcpts := <-ch:
			return rcpts
		case <-ctx.Done():
			require.FailNow(t, "Timeout waiting for receipt")
			return nil
		}
	}

	batch1 := sendAsync(tx(1), tx(2), tx(3))
	batch2 := sendAsync(tx(4), tx(5))
	require.Eventually(t, func() bool {
		return txMgr.sentCount() == 5
	}, 10*time.Second, 1*time.Millisecond, "Wait for first transactions to send")

	require.Len(t, batch1, 0, "Should not have completed batch1")
	require.Len(t, batch2, 0, "Should not have completed batch2")

	// Send a third batch after the first set have started sending to avoid races
	batch3 := sendAsync(tx(6))
	require.Len(t, batch3, 0, "Should not have completed batch3")

	// Sends the 6th tx after one of the previous ones completes
	txMgr.txSuccess(tx(5))
	require.Eventually(t, func() bool {
		return txMgr.sentCount() == 6
	}, 10*time.Second, 1*time.Millisecond, "Wait for final transaction to send")
	require.Len(t, batch1, 0, "Should not have completed batch1")
	require.Len(t, batch2, 0, "Should not have completed batch2")
	require.Len(t, batch3, 0, "Should not have completed batch3")

	// Batches complete as soon as they are sent
	txMgr.txSuccess(tx(6))
	require.Len(t, wait(batch3), 1, "Batch3 should complete")
	require.Len(t, batch1, 0, "Should not have completed batch1")
	require.Len(t, batch2, 0, "Should not have completed batch2")

	txMgr.txSuccess(tx(2))
	txMgr.txSuccess(tx(3))
	require.Len(t, batch1, 0, "Should not have completed batch1")
	require.Len(t, batch2, 0, "Should not have completed batch2")

	txMgr.txSuccess(tx(1))
	require.Len(t, wait(batch1), 3, "Batch1 should complete")
	require.Len(t, batch2, 0, "Should not have completed batch2")

	txMgr.txSuccess(tx(4))
	require.Len(t, wait(batch2), 2, "Batch2 should complete")
}

func TestSendAndWaitReturnIndividualErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	txMgr := &stubTxMgr{
		sending: make(map[byte]chan *types.Receipt),
		syncStatus: map[byte]uint64{
			0: types.ReceiptStatusSuccessful,
			1: types.ReceiptStatusFailed,
			2: types.ReceiptStatusSuccessful,
		},
	}
	sender := NewTxSender(ctx, testlog.Logger(t, log.LevelInfo), txMgr, 500)

	tx := func(i byte) txmgr.TxCandidate {
		return txmgr.TxCandidate{TxData: []byte{i}}
	}

	errs := sender.SendAndWaitDetailed("testing", tx(0), tx(1), tx(2))
	require.Len(t, errs, 3)
	require.NoError(t, errs[0])
	require.ErrorIs(t, errs[1], ErrTransactionReverted)
	require.NoError(t, errs[2])
}

type stubTxMgr struct {
	m          sync.Mutex
	sending    map[byte]chan *types.Receipt
	syncStatus map[byte]uint64
}

func (s *stubTxMgr) IsClosed() bool {
	return false
}

func (s *stubTxMgr) Send(ctx context.Context, candidate txmgr.TxCandidate) (*types.Receipt, error) {
	ch := s.recordTx(candidate)
	return <-ch, nil
}

// SendAsync simply wraps Send to make it non blocking. It does not guarantee transaction nonce ordering,
// unlike the production txMgr.
func (s *stubTxMgr) SendAsync(ctx context.Context, candidate txmgr.TxCandidate, ch chan txmgr.SendResponse) {
	go func() {
		receipt, err := s.Send(ctx, candidate)
		resp := txmgr.SendResponse{Receipt: receipt, Err: err}
		ch <- resp
	}()
}

func (s *stubTxMgr) recordTx(candidate txmgr.TxCandidate) chan *types.Receipt {
	s.m.Lock()
	defer s.m.Unlock()
	id := candidate.TxData[0]
	if _, ok := s.sending[id]; ok {
		// Shouldn't happen if tests are well written, but double check...
		panic("Sending duplicate transaction")
	}
	ch := make(chan *types.Receipt, 1)
	if status, ok := s.syncStatus[id]; ok {
		ch <- &types.Receipt{Status: status}
	} else {
		s.sending[id] = ch
	}
	return ch
}

func (s *stubTxMgr) txSuccess(candidate txmgr.TxCandidate) {
	s.m.Lock()
	defer s.m.Unlock()
	ch, ok := s.sending[candidate.TxData[0]]
	if !ok {
		// Shouldn't happen if tests are well written, but double check...
		panic(fmt.Sprintf("Completing unknown transaction: %v Known: %v", candidate.TxData[0], maps.Keys(s.sending)))
	}
	ch <- &types.Receipt{Status: types.ReceiptStatusSuccessful}
	close(ch)
}

func (s *stubTxMgr) sentCount() int {
	s.m.Lock()
	defer s.m.Unlock()
	return len(s.sending)
}

func (s *stubTxMgr) From() common.Address {
	panic("unsupported")
}

func (s *stubTxMgr) BlockNumber(_ context.Context) (uint64, error) {
	panic("unsupported")
}

func (s *stubTxMgr) API() rpc.API {
	panic("unimplemented")
}

func (s *stubTxMgr) Close() {
}

func (s *stubTxMgr) SuggestGasPriceCaps(context.Context) (*big.Int, *big.Int, *big.Int, error) {
	panic("unimplemented")
}

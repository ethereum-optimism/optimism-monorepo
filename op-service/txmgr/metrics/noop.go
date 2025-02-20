package metrics

import (
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/core/types"
)

type NoopTxMetrics struct{}

func (*NoopTxMetrics) RecordNonce(uint64)                {}
func (*NoopTxMetrics) RecordPendingTx(int64)             {}
func (*NoopTxMetrics) RecordGasBumpCount(int)            {}
func (*NoopTxMetrics) RecordTxConfirmationLatency(int64) {}
func (*NoopTxMetrics) TxConfirmed(*types.Receipt)        {}
func (*NoopTxMetrics) TxPublished(string)                {}
func (*NoopTxMetrics) RecordBaseFee(*big.Int)            {}
func (*NoopTxMetrics) RecordBlobBaseFee(*big.Int)        {}
func (*NoopTxMetrics) RecordTipCap(*big.Int)             {}
func (*NoopTxMetrics) RPCError()                         {}

type FakeTxMetrics struct {
	NoopTxMetrics
	pendingTxs atomic.Uint64
}

func (m *FakeTxMetrics) RecordPendingTx(p int64) {
	m.pendingTxs.Store(uint64(p))
}

func (m *FakeTxMetrics) PendingTxs() uint64 {
	return m.pendingTxs.Load()
}

package batcher

import (
	"context"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"

	"github.com/ethereum-optimism/optimism/op-service/txmgr"
)

type TestBatchSubmitter struct {
	*BatchSubmitter
	ttm *txmgr.TestTxManager
}

// JamTxPool is for testing ONLY. It sends a txpool-blocking transaction. This function must be
// called *before* the batcher starts submitting batches to ensure successful jamming, and will
// error out otherwise.
func (l *TestBatchSubmitter) JamTxPool(ctx context.Context) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.running {
		return errors.New("tried to jam tx pool but batcher is already running")
	}
	var candidate *txmgr.TxCandidate
	var err error
	cc := l.channelMgr.cfgProvider.ChannelConfig(true)
	if cc.UseBlobs {
		candidate = l.calldataTxCandidate([]byte{})
	} else if candidate, err = l.blobTxCandidate(emptyTxData); err != nil {
		return err
	}
	if candidate.GasLimit, err = core.IntrinsicGas(candidate.TxData, nil, nil, false, true, true, false); err != nil {
		return err
	}

	simpleTxMgr, ok := l.Txmgr.(*txmgr.SimpleTxManager)
	if !ok {
		return errors.New("txmgr is not a SimpleTxManager")
	}
	l.ttm = &txmgr.TestTxManager{
		SimpleTxManager: simpleTxMgr,
	}
	l.Log.Info("sending txpool blocking test tx")
	if err := l.ttm.JamTxPool(ctx, *candidate); err != nil {
		return err
	}
	return nil
}

// Wait on the jamming transaction, and return error if it completes successfully. (Tests should
// expect the blocking transaction to result in error from the context being cancelled.)
func (l *TestBatchSubmitter) WaitOnJammingTx(ctx context.Context) error {
	err := l.ttm.WaitOnJammingTx(ctx)
	if err == nil {
		return errors.New("txpool blocking tx didn't block!")
	}
	if strings.Contains(err.Error(), txpool.ErrAlreadyReserved.Error()) {
		return errors.New("txpool blocking tx failed because other tx in mempool is blocking it")
	}
	l.Log.Info("done waiting on jamming tx", "err", err)
	return nil
}

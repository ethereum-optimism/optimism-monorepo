package txmgr

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
)

var (
	testHash = common.HexToHash("0x01")
)

const testSafeAbortNonceTooLowCount = 3

func newSendState() *SendState {
	return newSendStateWithTimeout(time.Hour, time.Now)
}

func newSendStateWithTimeout(t time.Duration, now func() time.Time) *SendState {
	return NewSendStateWithNow(testSafeAbortNonceTooLowCount, t, now)
}

func processNSendErrors(sendState *SendState, err error, n int) {
	for i := 0; i < n; i++ {
		sendState.ProcessSendError(err)
	}
}

// TestSendStateNoAbortAfterInit asserts that the default SendState won't
// trigger an abort even after the safe abort interval has elapsed.
func TestSendStateNoAbortAfterInit(t *testing.T) {
	sendState := newSendState()
	require.Nil(t, sendState.CriticalError())
	require.False(t, sendState.IsWaitingForConfirmation())
}

// TestSendStateNoAbortAfterProcessNilError asserts that nil errors are not
// considered for abort status.
func TestSendStateNoAbortAfterProcessNilError(t *testing.T) {
	sendState := newSendState()

	processNSendErrors(sendState, nil, testSafeAbortNonceTooLowCount)
	require.Nil(t, sendState.CriticalError())
}

// TestSendStateNoAbortAfterProcessOtherError asserts that non-nil errors other
// than ErrNonceTooLow are not considered for abort status.
func TestSendStateNoAbortAfterProcessOtherError(t *testing.T) {
	sendState := newSendState()

	otherError := errors.New("other error")
	processNSendErrors(sendState, otherError, testSafeAbortNonceTooLowCount)
	require.Nil(t, sendState.CriticalError())
}

// TestSendStateAbortSafelyAfterNonceTooLowNoTxPublished asserts that we will abort after the very
// first none-too-low error if a tx hasn't yet been published.
func TestSendStateAbortSafelyAfterNonceTooLowNoTxPublished(t *testing.T) {
	sendState := newSendState()

	sendState.ProcessSendError(core.ErrNonceTooLow)
	require.ErrorIs(t, sendState.CriticalError(), core.ErrNonceTooLow)
}

// TestSendStateAbortSafelyAfterNonceTooLowButNoTxMined asserts that we will
// abort after the safe abort interval has elapsed if we haven't mined a tx.
func TestSendStateAbortSafelyAfterNonceTooLowButNoTxMined(t *testing.T) {
	sendState := newSendState()

	sendState.ProcessSendError(nil)
	sendState.ProcessSendError(core.ErrNonceTooLow)
	require.Nil(t, sendState.CriticalError())
	sendState.ProcessSendError(core.ErrNonceTooLow)
	require.Nil(t, sendState.CriticalError())
	sendState.ProcessSendError(core.ErrNonceTooLow)
	require.ErrorIs(t, sendState.CriticalError(), core.ErrNonceTooLow)
}

// TestSendStateMiningTxCancelsAbort asserts that a tx getting mined after
// processing ErrNonceTooLow takes precedence and doesn't cause an abort.
func TestSendStateMiningTxCancelsAbort(t *testing.T) {
	sendState := newSendState()

	sendState.ProcessSendError(core.ErrNonceTooLow)
	sendState.ProcessSendError(core.ErrNonceTooLow)
	sendState.TxMined(testHash)
	require.Nil(t, sendState.CriticalError())
	sendState.ProcessSendError(core.ErrNonceTooLow)
	require.Nil(t, sendState.CriticalError())
}

// TestSendStateReorgingTxResetsAbort asserts that unmining a tx does not
// consider ErrNonceTooLow's prior to being mined when determining whether
// to abort.
func TestSendStateReorgingTxResetsAbort(t *testing.T) {
	sendState := newSendState()

	sendState.ProcessSendError(nil)
	sendState.ProcessSendError(core.ErrNonceTooLow)
	sendState.ProcessSendError(core.ErrNonceTooLow)
	sendState.TxMined(testHash)
	sendState.TxNotMined(testHash)
	sendState.ProcessSendError(core.ErrNonceTooLow)
	require.Nil(t, sendState.CriticalError())
}

// TestSendStateNoAbortEvenIfNonceTooLowAfterTxMined asserts that we will not
// abort if we continue to get ErrNonceTooLow after a tx has been mined.
//
// NOTE: This is the most crucial role of the SendState, as we _expect_ to get
// ErrNonceTooLow failures after one of our txs has been mined, but that
// shouldn't cause us to not continue waiting for confirmations.
func TestSendStateNoAbortEvenIfNonceTooLowAfterTxMined(t *testing.T) {
	sendState := newSendState()

	sendState.TxMined(testHash)
	processNSendErrors(
		sendState, core.ErrNonceTooLow, testSafeAbortNonceTooLowCount,
	)
	require.Nil(t, sendState.CriticalError())
}

// TestSendStateSafeAbortIfNonceTooLowPersistsAfterUnmine asserts that we will
// correctly abort if we continue to get ErrNonceTooLow after a tx is unmined
// but not remined.
func TestSendStateSafeAbortIfNonceTooLowPersistsAfterUnmine(t *testing.T) {
	sendState := newSendState()

	sendState.ProcessSendError(nil)
	sendState.TxMined(testHash)
	sendState.TxNotMined(testHash)
	sendState.ProcessSendError(core.ErrNonceTooLow)
	sendState.ProcessSendError(core.ErrNonceTooLow)
	require.Nil(t, sendState.CriticalError())
	sendState.ProcessSendError(core.ErrNonceTooLow)
	require.ErrorIs(t, sendState.CriticalError(), core.ErrNonceTooLow)
}

// TestSendStateSafeAbortWhileCallingNotMinedOnUnminedTx asserts that we will
// correctly abort if we continue to call TxNotMined on txns that haven't been
// mined.
func TestSendStateSafeAbortWhileCallingNotMinedOnUnminedTx(t *testing.T) {
	sendState := newSendState()

	processNSendErrors(
		sendState, core.ErrNonceTooLow, testSafeAbortNonceTooLowCount,
	)
	sendState.TxNotMined(testHash)
	require.ErrorIs(t, sendState.CriticalError(), core.ErrNonceTooLow)
}

// TestSendStateIsWaitingForConfirmationAfterTxMined asserts that we are waiting
// for confirmation after a tx is mined.
func TestSendStateIsWaitingForConfirmationAfterTxMined(t *testing.T) {
	sendState := newSendState()

	testHash2 := common.HexToHash("0x02")

	sendState.TxMined(testHash)
	require.True(t, sendState.IsWaitingForConfirmation())
	sendState.TxMined(testHash2)
	require.True(t, sendState.IsWaitingForConfirmation())
}

// TestSendStateIsNotWaitingForConfirmationAfterTxUnmined asserts that we are
// not waiting for confirmation after a tx is mined then unmined.
func TestSendStateIsNotWaitingForConfirmationAfterTxUnmined(t *testing.T) {
	sendState := newSendState()

	sendState.TxMined(testHash)
	sendState.TxNotMined(testHash)
	require.False(t, sendState.IsWaitingForConfirmation())
}

func stepClock(step time.Duration) func() time.Time {
	i := 0
	return func() time.Time {
		var start time.Time
		i += 1
		return start.Add(time.Duration(i) * step)
	}
}

// TestSendStateTimeoutAbort ensure that this will abort if it passes the tx pool timeout
// when no successful transactions have been recorded
func TestSendStateTimeoutAbort(t *testing.T) {
	sendState := newSendStateWithTimeout(10*time.Millisecond, stepClock(20*time.Millisecond))
	require.ErrorIs(t, sendState.CriticalError(), ErrMempoolDeadlineExpired, "Should abort after timing out")
}

// TestSendStateNoTimeoutAbortIfPublishedTx ensure that this will not abort if there is
// a successful transaction send.
func TestSendStateNoTimeoutAbortIfPublishedTx(t *testing.T) {
	sendState := newSendStateWithTimeout(10*time.Millisecond, stepClock(20*time.Millisecond))
	sendState.ProcessSendError(nil)
	require.Nil(t, sendState.CriticalError(), "Should not abort if published transaction successfully")
}

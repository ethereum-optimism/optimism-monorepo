package p2p

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slog"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"

	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestPingService(t *testing.T) {
	peers := []peer.ID{"a", "b", "c"}
	log, captLog := testlog.CaptureLogger(t, slog.LevelDebug)

	pingCount := 0
	pingFn := PingFn(func(ctx context.Context, peerID peer.ID) <-chan ping.Result {
		out := make(chan ping.Result, 1)
		switch pingCount % 3 {
		case 0:
			// success
			out <- ping.Result{
				RTT:   time.Millisecond * 10,
				Error: nil,
			}
		case 1:
			// fake timeout
		case 2:
			// error
			out <- ping.Result{
				RTT:   0,
				Error: errors.New("fake error"),
			}
		}
		close(out)
		pingCount += 1
		return out
	})

	fakeClock := clock.NewDeterministicClock(time.Now())
	peersFn := PeersFn(func() []peer.ID {
		return peers
	})

	srv := NewPingService(log, pingFn, peersFn, fakeClock)

	trace := make(chan string)
	srv.trace = func(work string) {
		trace <- work
	}

	// wait for ping service to get online
	require.Equal(t, "started", <-trace)
	fakeClock.AdvanceTime(pingRound)
	// wait for first round to start and complete
	require.Equal(t, "pingPeers start", <-trace)
	require.Equal(t, "pingPeers end", <-trace)
	// see if client has hit all 3 cases we simulated on the server-side
	require.Equal(t, 3, pingCount, "pinged 3 peers")
	require.NotNil(t, captLog.FindLog(testlog.NewMessageContainsFilter("ping-pong")), "case 0")
	require.NotNil(t, captLog.FindLog(testlog.NewMessageContainsFilter("failed to ping peer, context cancelled")), "case 1")
	require.NotNil(t, captLog.FindLog(testlog.NewMessageContainsFilter("failed to ping peer, communication error")), "case 2")
	captLog.Clear()

	// advance clock again to proceed to second round, and wait for the round to start and complete
	fakeClock.AdvanceTime(pingRound)
	require.Equal(t, "pingPeers start", <-trace)
	require.Equal(t, "pingPeers end", <-trace)
	// see if client has hit all 3 cases we simulated on the server-side
	require.Equal(t, 6, pingCount, "pinged 3 peers again")
	require.NotNil(t, captLog.FindLog(testlog.NewMessageContainsFilter("ping-pong")), "case 0")
	require.NotNil(t, captLog.FindLog(testlog.NewMessageContainsFilter("failed to ping peer, context cancelled")), "case 1")
	require.NotNil(t, captLog.FindLog(testlog.NewMessageContainsFilter("failed to ping peer, communication error")), "case 2")
	captLog.Clear()

	srv.Close()
}

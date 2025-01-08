package game

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-service/clock"
)

// TestMonitorGames tests that the monitor can handle a new head event
// and resubscribe to new heads if the subscription errors.
func TestMonitorGames(t *testing.T) {
	t.Run("Schedules games", func(t *testing.T) {
		addr1 := common.Address{0xaa}
		addr2 := common.Address{0xbb}
		monitor, source, sched, mockHeadSource, preimages, _ := setupMonitorTest(t, []common.Address{})
		source.games = []types.GameMetadata{newFDG(addr1, 9999), newFDG(addr2, 9999)}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			headerNotSent := true
			for {
				if len(sched.Scheduled()) >= 1 {
					break
				}
				sub := mockHeadSource.Sub()
				if sub == nil {
					continue
				}
				if headerNotSent {
					select {
					case sub.headers <- &ethtypes.Header{
						Number: big.NewInt(1),
					}:
						headerNotSent = false
					case <-ctx.Done():
						return
					default:
					}
				}
				// Just to avoid a tight loop
				time.Sleep(100 * time.Millisecond)
			}
			mockHeadSource.SetErr(fmt.Errorf("eth subscribe test error"))
			cancel()
		}()

		monitor.StartMonitoring()
		<-ctx.Done()
		monitor.StopMonitoring()
		require.Len(t, sched.Scheduled(), 1)
		require.Equal(t, []common.Address{addr1, addr2}, sched.Scheduled()[0])
		require.GreaterOrEqual(t, preimages.ScheduleCount(), 1, "Should schedule preimage checks")
	})

	t.Run("Resubscribes on error", func(t *testing.T) {
		addr1 := common.Address{0xaa}
		addr2 := common.Address{0xbb}
		monitor, source, sched, mockHeadSource, preimages, _ := setupMonitorTest(t, []common.Address{})
		source.games = []types.GameMetadata{newFDG(addr1, 9999), newFDG(addr2, 9999)}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			// Wait for the subscription to be created
			waitErr := wait.For(context.Background(), 5*time.Second, func() (bool, error) {
				return mockHeadSource.Sub() != nil, nil
			})
			require.NoError(t, waitErr)
			mockHeadSource.Sub().errChan <- fmt.Errorf("test error")
			for {
				if len(sched.Scheduled()) >= 1 {
					break
				}
				sub := mockHeadSource.Sub()
				if sub == nil {
					continue
				}
				select {
				case sub.headers <- &ethtypes.Header{
					Number: big.NewInt(1),
				}:
				case <-ctx.Done():
					return
				default:
				}
				// Just to avoid a tight loop
				time.Sleep(100 * time.Millisecond)
			}
			mockHeadSource.SetErr(fmt.Errorf("eth subscribe test error"))
			cancel()
		}()

		monitor.StartMonitoring()
		<-ctx.Done()
		monitor.StopMonitoring()
		require.NotEmpty(t, sched.Scheduled()) // We might get more than one update scheduled.
		require.Equal(t, []common.Address{addr1, addr2}, sched.Scheduled()[0])
		require.GreaterOrEqual(t, preimages.ScheduleCount(), 1, "Should schedule preimage checks")
	})
}

func TestMonitorCreateAndProgressGameAgents(t *testing.T) {
	monitor, source, sched, _, _, _ := setupMonitorTest(t, []common.Address{})

	addr1 := common.Address{0xaa}
	addr2 := common.Address{0xbb}
	source.games = []types.GameMetadata{newFDG(addr1, 9999), newFDG(addr2, 9999)}

	require.NoError(t, monitor.progressGames(context.Background(), common.Hash{0x01}, 0))

	require.Len(t, sched.Scheduled(), 1)
	require.Equal(t, []common.Address{addr1, addr2}, sched.Scheduled()[0])
}

func TestMonitorOnlyScheduleSpecifiedGame(t *testing.T) {
	addr1 := common.Address{0xaa}
	addr2 := common.Address{0xbb}
	monitor, source, sched, _, _, stubClaimer := setupMonitorTest(t, []common.Address{addr2})
	source.games = []types.GameMetadata{newFDG(addr1, 9999), newFDG(addr2, 9999)}

	require.NoError(t, monitor.progressGames(context.Background(), common.Hash{0x01}, 0))

	require.Len(t, sched.Scheduled(), 1)
	require.Equal(t, []common.Address{addr2}, sched.Scheduled()[0])
	require.Equal(t, 1, stubClaimer.scheduledGames)
}

func newFDG(proxy common.Address, timestamp uint64) types.GameMetadata {
	return types.GameMetadata{
		Proxy:     proxy,
		Timestamp: timestamp,
	}
}

func setupMonitorTest(
	t *testing.T,
	allowedGames []common.Address,
) (*gameMonitor, *stubGameSource, *stubScheduler, *mockNewHeadSource, *stubPreimageScheduler, *mockScheduler) {
	logger := testlog.Logger(t, log.LevelDebug)
	source := &stubGameSource{}
	sched := &stubScheduler{}
	preimages := &stubPreimageScheduler{}
	mockHeadSource := &mockNewHeadSource{}
	stubClaimer := &mockScheduler{}
	monitor := newGameMonitor(
		logger,
		clock.NewSimpleClock(),
		source,
		sched,
		preimages,
		time.Duration(0),
		stubClaimer,
		allowedGames,
		mockHeadSource,
	)
	return monitor, source, sched, mockHeadSource, preimages, stubClaimer
}

type mockNewHeadSource struct {
	sync.Mutex
	sub *mockSubscription
	err error
}

func (m *mockNewHeadSource) Sub() *mockSubscription {
	m.Lock()
	defer m.Unlock()
	return m.sub
}

func (m *mockNewHeadSource) SetSub(sub *mockSubscription) {
	m.Lock()
	defer m.Unlock()
	m.sub = sub
}

func (m *mockNewHeadSource) SetErr(err error) {
	m.Lock()
	defer m.Unlock()
	m.err = err
}

func (m *mockNewHeadSource) Subscribe(
	_ context.Context,
	namespace string,
	ch any,
	_ ...any,
) (ethereum.Subscription, error) {
	m.Lock()
	defer m.Unlock()
	if namespace != "eth" {
		return nil, fmt.Errorf("only support eth RPC subscription, got %q", namespace)
	}
	errChan := make(chan error)
	m.sub = &mockSubscription{errChan, (ch).(chan<- *ethtypes.Header)}
	if m.err != nil {
		return nil, m.err
	}
	return m.sub, nil
}

type mockScheduler struct {
	scheduleErr    error
	scheduledGames int
}

func (m *mockScheduler) Schedule(_ uint64, games []types.GameMetadata) error {
	m.scheduledGames += len(games)
	return m.scheduleErr
}

type mockSubscription struct {
	errChan chan error
	headers chan<- *ethtypes.Header
}

func (m *mockSubscription) Unsubscribe() {}

func (m *mockSubscription) Err() <-chan error {
	return m.errChan
}

type stubGameSource struct {
	fetchErr error
	games    []types.GameMetadata
}

func (s *stubGameSource) GetGamesAtOrAfter(
	_ context.Context,
	_ common.Hash,
	_ uint64,
) ([]types.GameMetadata, error) {
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	return s.games, nil
}

type stubScheduler struct {
	sync.Mutex
	scheduled [][]common.Address
}

func (s *stubScheduler) Scheduled() [][]common.Address {
	s.Lock()
	defer s.Unlock()
	return s.scheduled
}

func (s *stubScheduler) Schedule(games []types.GameMetadata, blockNumber uint64) error {
	s.Lock()
	defer s.Unlock()
	var addrs []common.Address
	for _, game := range games {
		addrs = append(addrs, game.Proxy)
	}
	s.scheduled = append(s.scheduled, addrs)
	return nil
}

type stubPreimageScheduler struct {
	sync.Mutex
	scheduleCount int
}

func (s *stubPreimageScheduler) Schedule(_ common.Hash, _ uint64) error {
	s.Lock()
	defer s.Unlock()
	s.scheduleCount++
	return nil
}

func (s *stubPreimageScheduler) ScheduleCount() int {
	s.Lock()
	defer s.Unlock()
	return s.scheduleCount
}

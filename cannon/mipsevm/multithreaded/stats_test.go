package multithreaded

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
)

func TestStatsTracker(t *testing.T) {
	cases := []struct {
		name       string
		operations []Operation
		expected   *mipsevm.DebugInfo
	}{
		{
			name:       "Successful RMW operation",
			operations: []Operation{ll(1, 3), scSuccess(1, 13)},
			expected:   &mipsevm.DebugInfo{RmwSuccessCount: 1, MaxStepsBetweenLLAndSC: 10},
		},
		{
			name:       "Failed RMW operation",
			operations: []Operation{ll(1, 3), scFail(1, 13)},
			expected:   &mipsevm.DebugInfo{RmwFailCount: 1, MaxStepsBetweenLLAndSC: 10},
		},
		{
			name:       "Failed isolated sc op",
			operations: []Operation{scFail(1, 13)},
			expected:   &mipsevm.DebugInfo{RmwFailCount: 1},
		},
		{
			name:       "Failed isolated sc op preceded by successful sc op",
			operations: []Operation{ll(1, 1), scSuccess(1, 10), scFail(1, 23)},
			expected:   &mipsevm.DebugInfo{RmwSuccessCount: 1, RmwFailCount: 1, MaxStepsBetweenLLAndSC: 9},
		},
		{
			name:       "Multiple RMW operations",
			operations: []Operation{ll(1, 1), scSuccess(1, 2), ll(2, 3), scFail(2, 5), ll(3, 6), scSuccess(3, 16), ll(2, 18), scSuccess(2, 20), ll(1, 21), scFail(1, 30)},
			expected:   &mipsevm.DebugInfo{RmwSuccessCount: 3, RmwFailCount: 2, MaxStepsBetweenLLAndSC: 10},
		},
		{
			name:       "Multiple RMW operations exceeding cache size",
			operations: []Operation{ll(1, 1), ll(2, 2), ll(3, 3), ll(4, 4), scSuccess(4, 5), scFail(3, 6), scFail(2, 7), scFail(1, 8)},
			expected:   &mipsevm.DebugInfo{RmwSuccessCount: 1, RmwFailCount: 3, MaxStepsBetweenLLAndSC: 5},
		},
		{
			name:       "Interleaved RMW operations",
			operations: []Operation{ll(1, 5), ll(2, 10), scSuccess(2, 15), scFail(1, 25)},
			expected:   &mipsevm.DebugInfo{RmwSuccessCount: 1, RmwFailCount: 1, MaxStepsBetweenLLAndSC: 20},
		},
		{
			name:       "Invalidate reservation",
			operations: []Operation{invalidateReservation()},
			expected:   &mipsevm.DebugInfo{ReservationInvalidationCount: 1},
		},
		{
			name:       "Invalidate reservation multiple times",
			operations: []Operation{invalidateReservation(), invalidateReservation()},
			expected:   &mipsevm.DebugInfo{ReservationInvalidationCount: 2},
		},
		{
			name:       "Force preemption",
			operations: []Operation{forcePreempt()},
			expected:   &mipsevm.DebugInfo{ForcedPreemptionCount: 1},
		},
		{
			name:       "Force preemption multiple times",
			operations: []Operation{forcePreempt(), forcePreempt()},
			expected:   &mipsevm.DebugInfo{ForcedPreemptionCount: 2},
		},
		{
			name:       "Preempt thread 0 for thread 0",
			operations: []Operation{activateThread(0, 10), activateThread(0, 20), activateThread(0, 21)},
			expected:   &mipsevm.DebugInfo{IdleStepCountThread0: 0},
		},
		{
			name:       "Preempt thread 0 for different thread",
			operations: []Operation{activateThread(1, 10), activateThread(0, 20), activateThread(0, 21), activateThread(1, 22), activateThread(0, 25)},
			expected:   &mipsevm.DebugInfo{IdleStepCountThread0: 13},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stats := newStatsTracker(3)
			for _, op := range c.operations {
				op(stats)
			}

			// Validate expectations
			actual := &mipsevm.DebugInfo{}
			stats.populateDebugInfo(actual)
			require.Equal(t, c.expected, actual)
		})
	}
}

type Operation func(tracker StatsTracker)

func ll(threadId Word, step uint64) Operation {
	return func(tracker StatsTracker) {
		tracker.trackLL(threadId, step)
	}
}

func scSuccess(threadId Word, step uint64) Operation {
	return func(tracker StatsTracker) {
		tracker.trackSCSuccess(threadId, step)
	}
}

func scFail(threadId Word, step uint64) Operation {
	return func(tracker StatsTracker) {
		tracker.trackSCFailure(threadId, step)
	}
}

func invalidateReservation() Operation {
	return func(tracker StatsTracker) {
		tracker.trackReservationInvalidation()
	}
}

func forcePreempt() Operation {
	return func(tracker StatsTracker) {
		tracker.trackForcedPreemption()
	}
}

func activateThread(tid Word, step uint64) Operation {
	return func(tracker StatsTracker) {
		tracker.trackThreadActivated(tid, step)
	}
}

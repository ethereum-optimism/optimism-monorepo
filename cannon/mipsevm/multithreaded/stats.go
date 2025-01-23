package multithreaded

import (
	lru "github.com/hashicorp/golang-lru/v2/simplelru"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
)

// Define stats interface
type StatsTracker interface {
	trackLL(threadId Word, step uint64)
	trackSCSuccess(threadId Word, step uint64)
	trackSCFailure(threadId Word, step uint64)
	trackReservationInvalidation()
	trackForcedPreemption()
	trackThreadActivated(tid Word, step uint64)
	populateDebugInfo(debugInfo *mipsevm.DebugInfo)
}

// Noop implementation for when tracking is disabled
type noopStatsTracker struct{}

func NoopStatsTracker() StatsTracker {
	return &noopStatsTracker{}
}

func (s *noopStatsTracker) trackLL(threadId Word, step uint64)             {}
func (s *noopStatsTracker) trackSCSuccess(threadId Word, step uint64)      {}
func (s *noopStatsTracker) trackSCFailure(threadId Word, step uint64)      {}
func (s *noopStatsTracker) trackReservationInvalidation()                  {}
func (s *noopStatsTracker) trackForcedPreemption()                         {}
func (s *noopStatsTracker) trackThreadActivated(tid Word, step uint64)     {}
func (s *noopStatsTracker) populateDebugInfo(debugInfo *mipsevm.DebugInfo) {}

var _ StatsTracker = (*noopStatsTracker)(nil)

// Actual implementation
type statsTrackerImpl struct {
	// State
	lastLLStepByThread    *lru.LRU[Word, uint64]
	activeThreadId        Word
	lastActiveStepThread0 uint64
	// Stats
	rmwSuccessCount        uint64
	rmwFailCount           uint64
	maxStepsBetweenLLAndSC uint64
	// Tracks RMW reservation invalidation due to reserved memory being accessed outside of the RMW sequence
	reservationInvalidationCount uint64
	forcedPreemptionCount        uint64
	idleStepCountThread0         uint64
}

func (s *statsTrackerImpl) populateDebugInfo(debugInfo *mipsevm.DebugInfo) {
	debugInfo.RmwSuccessCount = s.rmwSuccessCount
	debugInfo.RmwFailCount = s.rmwFailCount
	debugInfo.MaxStepsBetweenLLAndSC = s.maxStepsBetweenLLAndSC
	debugInfo.ReservationInvalidationCount = s.reservationInvalidationCount
	debugInfo.ForcedPreemptionCount = s.forcedPreemptionCount
	debugInfo.IdleStepCountThread0 = s.idleStepCountThread0
}

func (s *statsTrackerImpl) trackLL(threadId Word, step uint64) {
	s.lastLLStepByThread.Add(threadId, step)
}

func (s *statsTrackerImpl) trackSCSuccess(threadId Word, step uint64) {
	s.rmwSuccessCount += 1
	s.recordStepsBetweenLLAndSC(threadId, step)
}

func (s *statsTrackerImpl) trackSCFailure(threadId Word, step uint64) {
	s.rmwFailCount += 1
	s.recordStepsBetweenLLAndSC(threadId, step)
}

func (s *statsTrackerImpl) recordStepsBetweenLLAndSC(threadId Word, scStep uint64) {
	// Track rmw steps if we have the last ll step in our cache
	if llStep, ok := s.lastLLStepByThread.Get(threadId); ok {
		diff := scStep - llStep
		if diff > s.maxStepsBetweenLLAndSC {
			s.maxStepsBetweenLLAndSC = diff
		}
		// Purge ll step since the RMW seq is now complete
		s.lastLLStepByThread.Remove(threadId)
	}
}

func (s *statsTrackerImpl) trackReservationInvalidation() {
	s.reservationInvalidationCount += 1
}

func (s *statsTrackerImpl) trackForcedPreemption() {
	s.forcedPreemptionCount += 1
}

func (s *statsTrackerImpl) trackThreadActivated(tid Word, step uint64) {
	if s.activeThreadId == Word(0) && tid != Word(0) {
		// Thread 0 has been deactivated, start tracking to capture idle steps
		s.lastActiveStepThread0 = step
	} else if s.activeThreadId != Word(0) && tid == Word(0) {
		// Thread 0 has been activated, record idle steps
		idleSteps := step - s.lastActiveStepThread0
		s.idleStepCountThread0 += idleSteps
	}
	s.activeThreadId = tid
}

func NewStatsTracker() StatsTracker {
	return newStatsTracker(5)
}

func newStatsTracker(cacheSize int) StatsTracker {
	llStepCache, err := lru.NewLRU[Word, uint64](cacheSize, nil)
	if err != nil {
		panic(err) // negative size parameter may produce an error
	}

	return &statsTrackerImpl{
		lastLLStepByThread: llStepCache,
	}
}

var _ StatsTracker = (*statsTrackerImpl)(nil)

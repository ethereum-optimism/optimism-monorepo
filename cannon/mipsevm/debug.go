package mipsevm

import "github.com/ethereum/go-ethereum/common/hexutil"

type DebugInfo struct {
	Pages               int            `json:"pages"`
	MemoryUsed          hexutil.Uint64 `json:"memory_used"`
	NumPreimageRequests int            `json:"num_preimage_requests"`
	TotalPreimageSize   int            `json:"total_preimage_size"`
	TotalSteps          uint64         `json:"total_steps"`
	//  Multithreading-related stats below
	RmwSuccessCount              uint64 `json:"rmw_success_count"`
	RmwFailCount                 uint64 `json:"rmw_fail_count"`
	MaxStepsBetweenLLAndSC       uint64 `json:"max_steps_between_ll_and_sc"`
	ReservationInvalidationCount uint64 `json:"reservation_invalidation_count"`
	ForcedPreemptionCount        uint64 `json:"forced_preemption_count"`
	IdleStepCountThread0         uint64 `json:"idle_step_count_thread_0"`
}

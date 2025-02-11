package eth

type SupervisorSyncStatus struct {
	// MinSyncedL1 is the highest L1 block that has been processed by all supervisor nodes.
	// This is not the same as the latest L1 block known to the supervisor,
	// but rather the L1 block view of the supervisor nodes.
	// This L1 block may not be fully derived into L2 data on all nodes yet.
	MinSyncedL1 L1BlockRef                             `json:"minSyncedL1"`
	Chains      map[ChainID]*SupervisorChainSyncStatus `json:"chains"`
}

// SupervisorChainStatus is the status of a chain as seen by the supervisor.
type SupervisorChainSyncStatus struct {
	// LocalUnsafe is the latest L2 block that has been processed by the supervisor.
	LocalUnsafe BlockRef `json:"localUnsafe"`
}

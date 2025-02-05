package eth

type SupervisorStatus struct {
	// MinSyncedL1 is the highest L1 block that has been processed by all supervisor nodes.
	// This is not the same as the latest L1 block known to the supervisor,
	// but rather the L1 block view of the supervisor nodes.
	// This may not be fully derived into the L2 data of a particular node yet.
	// The local-safe L2 blocks were produced/included fully from the L1 chain up to _but excluding_ this L1 block.
	MinSyncedL1 L1BlockRef                         `json:"minSyncedL1"`
	Chains      map[ChainID]*SupervisorChainStatus `json:"chains"`
}

// SupervisorChainStatus is the status of a chain as seen by the supervisor.
type SupervisorChainStatus struct {
	// LocalUnsafe is the latest L2 block that has been processed by the supervisor.
	LocalUnsafe BlockRef `json:"localUnsafe"`
}

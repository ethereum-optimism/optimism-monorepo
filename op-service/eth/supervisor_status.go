package eth

// SupervisorChainStatus is the status of a chain as seen by the supervisor.
type SupervisorChainStatus struct {
	// LocalDerived is the latest L2 block that the chain was derived from.
	LocalDerived BlockRef `json:"localDerived"`
	// LocalDerivedFrom is the origin of LocalDerived.
	LocalDerivedFrom L1BlockRef `json:"localDerivedFrom"`
	// LocalUnsafe is the absolute tip of the L2 chain.
	LocalUnsafe BlockRef `json:"localUnsafe"`
}

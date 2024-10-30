package cross

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type SafeStartDeps interface {
	Check(chain types.ChainID, blockNum uint64, logIdx uint32, logHash common.Hash) (includedIn types.BlockSeal, err error)

	CrossDerivedFrom(chainID types.ChainID, derived eth.BlockID) (derivedFrom types.BlockSeal, err error)

	DependencySet() depset.DependencySet
}

// CrossSafeHazards checks if the given messages all exist and pass invariants.
// It returns a hazard-set: if any intra-block messaging happened,
// these hazard blocks have to be verified.
func CrossSafeHazards(d SafeStartDeps, chainID types.ChainID, inL1DerivedFrom eth.BlockID,
	candidate types.BlockSeal, execMsgs []*types.ExecutingMessage) (hazards map[types.ChainIndex]types.BlockSeal, err error) {

	hazards = make(map[types.ChainIndex]types.BlockSeal)

	// Warning for future: If we have sub-second distinct blocks (different block number),
	// we need to increase precision on the above timestamp invariant.
	// Otherwise a local block can depend on a future local block of the same chain,
	// simply by pulling in a block of another chain,
	// which then depends on a block of the original chain,
	// all with the same timestamp, without message cycles.

	depSet := d.DependencySet()

	if len(execMsgs) > 0 {
		if ok, err := depSet.CanExecuteAt(chainID, candidate.Timestamp); err != nil {
			return nil, fmt.Errorf("cannot check message execution of block %s (chain %s): %w", candidate, chainID, err)
		} else if !ok {
			return nil, fmt.Errorf("cannot execute messages in block %s (chain %s): %w", candidate, chainID, types.ErrConflict)
		}
	}

	// check all executing messages
	for _, msg := range execMsgs {
		initChainID, err := depSet.ChainIDFromIndex(msg.Chain)
		if err != nil {
			if errors.Is(err, types.ErrUnknownChain) {
				err = fmt.Errorf("msg %s may not execute from unknown chain %s: %w", msg, msg.Chain, types.ErrConflict)
			}
			return nil, err
		}
		if ok, err := depSet.CanInitiateAt(initChainID, msg.Timestamp); err != nil {
			return nil, fmt.Errorf("cannot check message initiation of msg %s (chain %s): %w", msg, chainID, err)
		} else if !ok {
			return nil, fmt.Errorf("cannot allow initiating message %s (chain %s): %w", msg, chainID, types.ErrConflict)
		}
		if msg.Timestamp < candidate.Timestamp {
			// If timestamp is older: invariant ensures non-cyclic ordering relative to other messages.
			// Check that the block that they are included in is cross-safe already.
			includedIn, err := d.Check(initChainID, msg.BlockNum, msg.LogIdx, msg.Hash)
			if err != nil {
				return nil, fmt.Errorf("executing msg %s failed check: %w", msg, err)
			}
			initDerivedFrom, err := d.CrossDerivedFrom(initChainID, includedIn.ID())
			if err != nil {
				return nil, fmt.Errorf("msg %s included in non-cross-safe block %s: %w", msg, includedIn, err)
			}
			if initDerivedFrom.Number > inL1DerivedFrom.Number {
				return nil, fmt.Errorf("msg %s was included in block %s derived from %s which is not in cross-safe scope %s: %w",
					msg, includedIn, initDerivedFrom, inL1DerivedFrom, types.ErrOutOfScope)
			}
		} else if msg.Timestamp == candidate.Timestamp {
			// If timestamp is equal: we have to inspect ordering of individual
			// log events to ensure non-cyclic cross-chain message ordering.
			// And since we may have back-and-forth messaging, we cannot wait till the initiating side is cross-safe.
			// Thus check that it was included in a local-safe block,
			// and then proceed with transitive block checks,
			// to ensure the local block we depend on is becoming cross-safe also.
			includedIn, err := d.Check(initChainID, msg.BlockNum, msg.LogIdx, msg.Hash)
			if err != nil {
				return nil, fmt.Errorf("executing msg %s failed check: %w", msg, err)
			}
			// As a hazard block, it will be checked to be included in a cross-safe block,
			// or right after a cross-safe block in a local-safe block, in HazardSafeFrontierChecks.
			if existing, ok := hazards[msg.Chain]; ok {
				if existing != includedIn {
					return nil, fmt.Errorf("found dependency on %s (chain %d), but already depend on %s", includedIn, initChainID, chainID)
				}
			} else {
				// Mark it as hazard block
				hazards[msg.Chain] = includedIn
			}
		} else {
			// Timestamp invariant is broken: executing message tries to execute future block.
			// The predeploy inbox contract should not have allowed this executing message through.
			return nil, fmt.Errorf("executing message %s in %s breaks timestamp invariant", msg, candidate)
		}
	}
	return hazards, nil
}

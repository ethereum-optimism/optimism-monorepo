package cross

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type msgKey struct {
	chainIndex types.ChainIndex
	logIndex   uint32
}

var (
	ErrCycle           = errors.New("cycle detected")
	ErrInvalidLogIndex = errors.New("executing message references invalid log index")
)

type CycleCheckDeps interface {
	OpenBlock(chainID types.ChainID, blockNum uint64) (seal types.BlockSeal, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
}

// validateExecMsgs ensures all executing message log indices are valid
func validateExecMsgs(logCount uint32, execMsgs map[uint32]*types.ExecutingMessage) error {
	for logIdx := range execMsgs {
		if logIdx >= logCount {
			return fmt.Errorf("%w: log index %d >= log count %d", ErrInvalidLogIndex, logIdx, logCount)
		}
	}
	return nil
}

// HazardCycleChecks performs a hazard-check where block.timestamp == execMsg.timestamp:
// here the timestamp invariant alone does not ensure ordering of messages.
// To be fully confident that there are no intra-block cyclic message dependencies,
// we have to sweep through the executing messages and check the hazards.
func HazardCycleChecks(d CycleCheckDeps, inTimestamp uint64, hazards map[types.ChainIndex]types.BlockSeal) error {
	// Algorithm: breadth-first-search (BFS).
	// Types of incoming edges:
	//   - the previous log event in the block
	//   - executing another event
	// Work:
	//   1. for each node with in-degree 0 (i.e. no dependencies), add it to the result, remove it from the work.
	//   2. along with removing, remove the outgoing edges
	//   3. if there is no node left with in-degree 0, then there is a cycle

	inDegreeNon0 := make(map[msgKey]uint32)
	inDegree0 := make(map[msgKey]struct{})
	outgoingEdges := make(map[msgKey][]msgKey)

	for hazardChainIndex, hazardBlock := range hazards {
		// TODO(#11105): translate chain index to chain ID
		hazardChainID := types.ChainIDFromUInt64(uint64(hazardChainIndex))
		bl, logCount, msgs, err := d.OpenBlock(hazardChainID, hazardBlock.Number)
		if err != nil {
			return fmt.Errorf("failed to open block: %w", err)
		}
		if bl != hazardBlock {
			return fmt.Errorf("tried to open block %s of chain %s, but got different block %s than expected, use a reorg lock for consistency", hazardBlock, hazardChainID, bl)
		}

		// Validate executing message indices
		if err := validateExecMsgs(logCount, msgs); err != nil {
			return err
		}

		for i := uint32(0); i < logCount; i++ {
			k := msgKey{
				chainIndex: hazardChainIndex,
				logIndex:   i,
			}
			if i == 0 {
				// first log in block does not have a dependency
				inDegree0[k] = struct{}{}
			} else {
				// add edge: prev log <> current log
				inDegreeNon0[k] = 1
			}
		}

		// Add edges for executing messages to their initiating messages
		// If the initiating message is itself an executing message (checked via msgs map),
		// we create an edge to maintain proper dependency ordering
		for execLogIdx, m := range msgs {
			if m.Timestamp != inTimestamp {
				continue // no need to worry about this edge. Already enforced by timestamp invariant
			}

			// Add edge from the initiating message to this executing message
			k := msgKey{
				chainIndex: m.Chain,
				logIndex:   m.LogIdx,
			}

			// The executing message itself is referenced by its log index in this block
			execKey := msgKey{
				chainIndex: hazardChainIndex,
				logIndex:   execLogIdx,
			}

			inDegreeNon0[execKey] += 1
			outgoingEdges[k] = append(outgoingEdges[k], execKey)
		}
	}

	for {
		// Process all nodes that have no incoming edges
		for k := range inDegree0 {
			// Remove all outgoing edges from this node
			for _, out := range outgoingEdges[k] {
				count := inDegreeNon0[out]
				count -= 1
				if count == 0 {
					delete(inDegreeNon0, out)
					inDegree0[out] = struct{}{}
				} else {
					inDegreeNon0[out] = count
				}
			}
			delete(outgoingEdges, k)
			delete(inDegree0, k)
		}

		if len(inDegree0) == 0 {
			if len(inDegreeNon0) == 0 {
				// Done, without cycles!
				return nil
			} else {
				// Some nodes left, but no nodes left with in-degree of 0. There must be a cycle.
				return ErrCycle
			}
		}
	}
}

package cross

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type msgKey struct {
	chainIndex types.ChainIndex
	logIndex   uint32
}

var (
	ErrCycle           = errors.New("cycle detected")
	ErrInvalidLogIndex = errors.New("executing message references invalid log index")
	ErrSelfReferencing = errors.New("executing message references itself")
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
				prevKey := msgKey{
					chainIndex: hazardChainIndex,
					logIndex:   i - 1,
				}
				inDegreeNon0[k] = 1
				outgoingEdges[prevKey] = append(outgoingEdges[prevKey], k)
			}
		}

		// Add edges for executing messages to their initiating messages
		// If the initiating message is itself an executing message (checked via msgs map),
		// we create an edge to maintain proper dependency ordering
		for execLogIdx, m := range msgs {
			// Skip if the message is not from the correct timestamp
			if m.Timestamp != inTimestamp {
				continue
			}

			initKey := msgKey{
				chainIndex: m.Chain,
				logIndex:   m.LogIdx,
			}
			execKey := msgKey{
				chainIndex: hazardChainIndex,
				logIndex:   execLogIdx,
			}

			// Disallow self-referencing messages
			if initKey == execKey {
				return ErrSelfReferencing
			}

			// Add the edge
			inDegreeNon0[execKey] += 1
			outgoingEdges[initKey] = append(outgoingEdges[initKey], execKey)
		}
	}

	// TODO: Remove after finishing this function
	fmt.Println("Built graph:")
	logMermaidDiagram(inDegree0, inDegreeNon0, outgoingEdges)

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

				fmt.Println("Found cycle; remaining sub-graph:")
				logMermaidDiagram(inDegree0, inDegreeNon0, outgoingEdges)
				return ErrCycle
			}
		}
	}
}

// GenerateMermaidDiagram creates a Mermaid flowchart diagram from the graph data
func GenerateMermaidDiagram(inDegree0 map[msgKey]struct{}, inDegreeNon0 map[msgKey]uint32, outgoingEdges map[msgKey][]msgKey) string {
	var sb strings.Builder

	sb.WriteString("flowchart TD\n")

	// Helper function to get a unique ID for each node
	getNodeID := func(k msgKey) string {
		return fmt.Sprintf("N%d_%d", k.chainIndex, k.logIndex)
	}

	// Helper function to get a label for each node
	getNodeLabel := func(k msgKey) string {
		return fmt.Sprintf("C%d:L%d", k.chainIndex, k.logIndex)
	}

	// Function to add a node to the diagram
	addNode := func(k msgKey, inDegree uint32) {
		nodeID := getNodeID(k)
		nodeLabel := getNodeLabel(k)
		var shape string
		if inDegree == 0 {
			shape = "((%s))"
		} else {
			shape = "[%s]"
		}
		sb.WriteString(fmt.Sprintf("    %s"+shape+"\n", nodeID, nodeLabel))
	}

	// Add all nodes
	for k := range inDegree0 {
		addNode(k, 0)
	}
	for k, inDegree := range inDegreeNon0 {
		addNode(k, inDegree)
	}

	// Add all edges
	for from, tos := range outgoingEdges {
		fromID := getNodeID(from)
		for _, to := range tos {
			toID := getNodeID(to)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
		}
	}

	// Add a legend
	sb.WriteString("    subgraph Legend\n")
	sb.WriteString("        L1((In-Degree 0))\n")
	sb.WriteString("        L2[In-Degree > 0]\n")
	sb.WriteString("    end\n")

	return sb.String()
}

// Helper function to generate a Mermaid diagram and log it
func logMermaidDiagram(inDegree0 map[msgKey]struct{}, inDegreeNon0 map[msgKey]uint32, outgoingEdges map[msgKey][]msgKey) {
	diagram := GenerateMermaidDiagram(inDegree0, inDegreeNon0, outgoingEdges)
	fmt.Printf("\n%s", diagram)
}

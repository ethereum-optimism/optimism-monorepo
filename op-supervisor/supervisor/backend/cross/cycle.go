package cross

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

var (
	ErrFailedToOpenBlock = errors.New("failed to open block")
	ErrCycle             = errors.New("cycle detected")
	ErrInvalidLogIndex   = errors.New("executing message references invalid log index")
	ErrSelfReferencing   = errors.New("executing message references itself")
	ErrUnknownChain      = errors.New("executing message references unknown chain")
)

// node represents a log entry in our graph.
type node struct {
	chainIndex types.ChainIndex
	logIndex   uint32
}

// graph is a directed graph of message dependencies.
type graph struct {
	inDegree0     map[node]struct{}
	inDegreeNon0  map[node]uint32
	outgoingEdges map[node][]node
}

// addEdge adds a directed edge from -> to in the graph.
func (g *graph) addEdge(from, to node) {
	// Remove the target from inDegree0 if it's there
	delete(g.inDegree0, to)

	// Add or increment the target's in-degree count
	g.inDegreeNon0[to] += 1

	// Add the outgoing edge
	g.outgoingEdges[from] = append(g.outgoingEdges[from], to)
}

// CycleCheckDeps is an interface for checking cyclical dependencies between logs.
type CycleCheckDeps interface {
	OpenBlock(chainID types.ChainID, blockNum uint64) (seal types.BlockSeal, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
}

// HazardCycleChecks checks for cyclical dependencies between logs at the given timestamp.
// Here the timestamp invariant alone does not ensure ordering of messages.
//
// We perform this check in 3 steps:
//   - Gather all logs across all hazard blocks at the given timestamp.
//   - Build the logs into a directed graph of dependencies between logs.
//   - Check the graph for cycles.
//
// The edges of the graph are determined by:
//   - For all logs except the first in a block, there is an edge from the previous log.
//   - For all executing messages, there is an edge from the initiating message.
//
// The edges between sequential logs ensure the graph is well-connected and free of any
// disjoint subgraphs that would make cycle checking more difficult.
//
// The cycle check is performed by executing Kahn's topological sort algorithm which
// succeeds if and only if a graph is acyclic.
//
// Returns nil if no cycles are found or ErrCycle if a cycle is detected.
func HazardCycleChecks(d CycleCheckDeps, inTimestamp uint64, hazards map[types.ChainIndex]types.BlockSeal) error {
	g, err := buildGraph(d, inTimestamp, hazards)
	if err != nil {
		return err
	}

	return checkForCycles(g)
}

// gatherLogs collects all log counts and executing messages across all hazard blocks.
// Returns:
// - map of chain index to its log count
// - map of chain index to map of log index to executing message (nil if doesn't exist or ignored)
func gatherLogs(d CycleCheckDeps, inTimestamp uint64, hazards map[types.ChainIndex]types.BlockSeal) (
	map[types.ChainIndex]uint32,
	map[types.ChainIndex]map[uint32]*types.ExecutingMessage,
	error,
) {
	logCounts := make(map[types.ChainIndex]uint32)
	execMsgs := make(map[types.ChainIndex]map[uint32]*types.ExecutingMessage)

	for hazardChainIndex, hazardBlock := range hazards {
		// TODO(#11105): translate chain index to chain ID
		hazardChainID := types.ChainIDFromUInt64(uint64(hazardChainIndex))
		bl, logCount, msgs, err := d.OpenBlock(hazardChainID, hazardBlock.Number)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open block: %w", err)
		}
		if bl != hazardBlock {
			return nil, nil, fmt.Errorf("tried to open block %s of chain %s, but got different block %s than expected, use a reorg lock for consistency", hazardBlock, hazardChainID, bl)
		}

		// Validate executing message indices
		if err := validateExecMsgIndices(logCount, msgs); err != nil {
			return nil, nil, err
		}

		logCounts[hazardChainIndex] = logCount

		// Initialize map for this chain if it has executing messages
		if len(msgs) > 0 {
			if _, exists := execMsgs[hazardChainIndex]; !exists {
				execMsgs[hazardChainIndex] = make(map[uint32]*types.ExecutingMessage)
			}
		}

		// Process executing messages - only include those at inTimestamp
		for logIdx, msg := range msgs {
			if msg.Timestamp == inTimestamp {
				execMsgs[hazardChainIndex][logIdx] = msg
			}
		}
	}

	return logCounts, execMsgs, nil
}

// validateExecMsgIndices ensures all executing message log indices are valid
func validateExecMsgIndices(logCount uint32, execMsgs map[uint32]*types.ExecutingMessage) error {
	for logIdx := range execMsgs {
		if logIdx >= logCount {
			return fmt.Errorf("%w: log index %d >= log count %d", ErrInvalidLogIndex, logIdx, logCount)
		}
	}
	return nil
}

// buildGraph constructs a dependency graph from the hazard blocks.
func buildGraph(d CycleCheckDeps, inTimestamp uint64, hazards map[types.ChainIndex]types.BlockSeal) (*graph, error) {
	g := &graph{
		inDegree0:     make(map[node]struct{}),
		inDegreeNon0:  make(map[node]uint32),
		outgoingEdges: make(map[node][]node),
	}

	logCounts, execMsgs, err := gatherLogs(d, inTimestamp, hazards)
	if err != nil {
		return nil, err
	}

	// Add nodes for each log in the block, and add edges between sequential logs
	for hazardChainIndex, logCount := range logCounts {
		for i := uint32(0); i < logCount; i++ {
			k := node{
				chainIndex: hazardChainIndex,
				logIndex:   i,
			}

			if i == 0 {
				// First log in block has no dependencies
				g.inDegree0[k] = struct{}{}
			} else {
				// Add edge: prev log <> current log
				prevKey := node{
					chainIndex: hazardChainIndex,
					logIndex:   i - 1,
				}
				g.addEdge(prevKey, k)
			}
		}
	}

	// Add edges for executing messages to their initiating messages
	for hazardChainIndex, msgs := range execMsgs {
		for execLogIdx, m := range msgs {
			// Error if the chain is unknown
			if _, ok := hazards[m.Chain]; !ok {
				return nil, ErrUnknownChain
			}

			// Check if we care about the init message
			initChainMsgs, ok := execMsgs[m.Chain]
			if !ok {
				continue
			}
			if _, ok := initChainMsgs[m.LogIdx]; !ok {
				continue
			}

			initKey := node{
				chainIndex: m.Chain,
				logIndex:   m.LogIdx,
			}
			execKey := node{
				chainIndex: hazardChainIndex,
				logIndex:   execLogIdx,
			}

			// Disallow self-referencing messages
			if initKey == execKey {
				return nil, ErrSelfReferencing
			}

			// Add the edge
			g.addEdge(initKey, execKey)
		}
	}

	return g, nil
}

// checkForCycles uses Kahn's topological sort algorithm to check for cycles in the graph.
// It returns nil for acyclic graphs and ErrCycle for cyclic graphs.
//
// Algorithm:
//  1. for each node with in-degree 0 (i.e. no dependencies), add it to the result, remove it from the work.
//  2. along with removing, remove the outgoing edges
//  3. if there is no node left with in-degree 0, then there is a cycle
func checkForCycles(g *graph) error {
	for {
		// Process all nodes that have no incoming edges
		for k := range g.inDegree0 {
			// Remove all outgoing edges from this node
			for _, out := range g.outgoingEdges[k] {
				count := g.inDegreeNon0[out]
				count -= 1
				if count == 0 {
					delete(g.inDegreeNon0, out)
					g.inDegree0[out] = struct{}{}
				} else {
					g.inDegreeNon0[out] = count
				}
			}
			delete(g.outgoingEdges, k)
			delete(g.inDegree0, k)
		}

		if len(g.inDegree0) == 0 {
			if len(g.inDegreeNon0) == 0 {
				// Done, without cycles!
				return nil
			} else {
				// Some nodes left, but no nodes left with in-degree of 0. There must be a cycle.
				return ErrCycle
			}
		}
	}
}

// GenerateMermaidDiagram creates a Mermaid flowchart diagram from the graph data for debugging.
func GenerateMermaidDiagram(g *graph) string {
	var sb strings.Builder

	sb.WriteString("flowchart TD\n")

	// Helper function to get a unique ID for each node
	getNodeID := func(k node) string {
		return fmt.Sprintf("N%d_%d", k.chainIndex, k.logIndex)
	}

	// Helper function to get a label for each node
	getNodeLabel := func(k node) string {
		return fmt.Sprintf("C%d:L%d", k.chainIndex, k.logIndex)
	}

	// Function to add a node to the diagram
	addNode := func(k node, inDegree uint32) {
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
	for k := range g.inDegree0 {
		addNode(k, 0)
	}
	for k, inDegree := range g.inDegreeNon0 {
		addNode(k, inDegree)
	}

	// Add all edges
	for from, tos := range g.outgoingEdges {
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

// logMermaidDiagram logs a Mermaid diagram for debugging.
func logMermaidDiagram(label string, g *graph) {
	diagram := GenerateMermaidDiagram(g)
	fmt.Printf("%s:\n%s", label, diagram)
}

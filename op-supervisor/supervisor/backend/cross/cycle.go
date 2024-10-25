package cross

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// msgKey is a unique identifier for a node in the graph.
type msgKey struct {
	chainIndex types.ChainIndex
	logIndex   uint32
}

// graph is a directed graph of message dependencies.
type graph struct {
	inDegree0     map[msgKey]struct{}
	inDegreeNon0  map[msgKey]uint32
	outgoingEdges map[msgKey][]msgKey
}

// addEdge adds a directed edge from -> to in the graph.
func (g *graph) addEdge(from, to msgKey) {
	// Remove the target from inDegree0 if it's there
	delete(g.inDegree0, to)

	// Add or increment the target's in-degree count
	g.inDegreeNon0[to] += 1

	// Add the outgoing edge
	g.outgoingEdges[from] = append(g.outgoingEdges[from], to)
}

var (
	ErrFailedToOpenBlock = errors.New("failed to open block")
	ErrCycle             = errors.New("cycle detected")
	ErrInvalidLogIndex   = errors.New("executing message references invalid log index")
	ErrSelfReferencing   = errors.New("executing message references itself")
	ErrUnknownChain      = errors.New("executing message references unknown chain")
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

// buildGraph constructs a dependency graph from the hazard blocks.
func buildGraph(d CycleCheckDeps, inTimestamp uint64, hazards map[types.ChainIndex]types.BlockSeal) (*graph, error) {
	g := &graph{
		inDegree0:     make(map[msgKey]struct{}),
		inDegreeNon0:  make(map[msgKey]uint32),
		outgoingEdges: make(map[msgKey][]msgKey),
	}

	for hazardChainIndex, hazardBlock := range hazards {
		// TODO(#11105): translate chain index to chain ID
		hazardChainID := types.ChainIDFromUInt64(uint64(hazardChainIndex))
		bl, logCount, msgs, err := d.OpenBlock(hazardChainID, hazardBlock.Number)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToOpenBlock, err)
		}
		if bl != hazardBlock {
			return nil, fmt.Errorf("tried to open block %s of chain %s, but got different block %s than expected, use a reorg lock for consistency", hazardBlock, hazardChainID, bl)
		}

		// Validate executing message indices
		if err := validateExecMsgs(logCount, msgs); err != nil {
			return nil, err
		}

		// Add nodes for each log in the block, and add edges between sequential logs
		for i := uint32(0); i < logCount; i++ {
			k := msgKey{
				chainIndex: hazardChainIndex,
				logIndex:   i,
			}
			if i == 0 {
				// First log in block has no dependencies.=
				g.inDegree0[k] = struct{}{}
			} else {
				// Add edge: prev log <> current log
				prevKey := msgKey{
					chainIndex: hazardChainIndex,
					logIndex:   i - 1,
				}
				g.addEdge(prevKey, k)
			}
		}

		// Add edges for executing messages to their initiating messages
		for execLogIdx, m := range msgs {
			// Skip if the message is not from the correct timestamp
			if m.Timestamp != inTimestamp {
				continue
			}

			// Skip if the chain is unknown
			if _, ok := hazards[m.Chain]; !ok {
				return nil, ErrUnknownChain
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
				return nil, ErrSelfReferencing
			}

			// Add the edge
			g.addEdge(initKey, execKey)
		}
	}

	return g, nil
}

// checkGraphForCycle checks if the given graph contains any cycles.
// Returns nil if no cycles are found or ErrCycle if a cycle is detected.
// It modifies the graph in-place.
func checkGraphForCycle(g *graph) error {
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
	g, err := buildGraph(d, inTimestamp, hazards)
	if err != nil {
		return err
	}

	logMermaidDiagram("Built graph", g)

	if err := checkGraphForCycle(g); err != nil {
		if err == ErrCycle {
			logMermaidDiagram("Found cycle; remaining sub-graph", g)
		}
		return err
	}

	return nil
}

// GenerateMermaidDiagram creates a Mermaid flowchart diagram from the graph data
func GenerateMermaidDiagram(g *graph) string {
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

// Helper function to generate a Mermaid diagram and log it
func logMermaidDiagram(label string, g *graph) {
	diagram := GenerateMermaidDiagram(g)
	fmt.Printf("%s:\n%s", label, diagram)
}

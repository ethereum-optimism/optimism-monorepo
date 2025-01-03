package utils

import "github.com/ethereum/go-ethereum/common"

type Node struct {
	Hash       common.Hash
	Number     uint64
	Timestamp  uint64
	ParentHash common.Hash

	// optional, if L2 block
	DerivedFrom common.Hash

	// optional, if L2 block
	L1Origin common.Hash

	Labels map[string]struct{}
}

func Analysis() {
	//graph := make(map[common.Hash]*Node)

	// TODO load all L1 blocks into graph

	// TODO load all L2 blocks into graph
	// label what was seen where

	// TODO load all implied L1 origins into graph

	// TODO label all blocks with perfect L1origin increment

	// TODO label all blocks included in L1 too late for sequence window

	//- construct labeled graph of all block-hashes
	//- label what block is seen where
	//- label what block meets L1 timedrift criteria
	//- label what block is derived from which L1 block
	//- label what block meets the sequencing window
	//- label what resyncing produces as canonical blocks

	// TODO: visualization idea: use git graphs
	// See https://mermaid.js.org/syntax/gitgraph.html

	// TODO iterate over L1 blocks:
	// TODO for each batch tx, create a tag.

	// TODO checkout a branch for the canonical L2
	// TODO merge a L1 block whenever the L1 origin is included in the L2
	//	- "checkout L2, merge L1"
	// TODO branch off whenever the L2 forks
	//  - keep merging L1 blocks into these L2 forks, for each L1 origin

	// TODO for each singular batch, create a merge-commit into L1 when the channel completes.

	// TODO for each span-batch, create a branch

	// TODO cherry-pick an L2 block into a channel if it's included as batch

}

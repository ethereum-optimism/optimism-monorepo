package interop

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/cross"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/processors"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func ReceiptsToExecutingMessages(receipts ethtypes.Receipts) ([]*supervisortypes.ExecutingMessage, error) {
	var execMsgs []*supervisortypes.ExecutingMessage
	for _, rcpt := range receipts {
		for _, l := range rcpt.Logs {
			execMsg, err := processors.DecodeExecutingMessageLog(l)
			if err != nil {
				return nil, err
			}
			execMsgs = append(execMsgs, execMsg)
		}
	}
	return execMsgs, nil
}

func Consolidate(deps ConsolidateCheckDeps,
	oracle l2.Oracle,
	transitionState *types.TransitionState,
	superRoot *eth.SuperV1,
) (eth.Bytes32, error) {
	var consolidatedChains []eth.ChainIDAndOutput

	for i, chain := range superRoot.Chains {
		progress := transitionState.PendingProgress[i]
		receipts := oracle.ReceiptsByBlockHash(progress.BlockHash, chain.ChainID)
		execMsgs, err := ReceiptsToExecutingMessages(receipts)
		if err != nil {
			return eth.Bytes32{}, err
		}
		block := oracle.BlockByHash(progress.BlockHash, chain.ChainID)
		candidate := supervisortypes.BlockSeal{
			Hash:      progress.BlockHash,
			Number:    block.NumberU64(),
			Timestamp: block.Time(),
		}
		hazards, err := getHazards(progress.BlockHash, execMsgs)
		if err != nil {
			return eth.Bytes32{}, err
		}
		if err := checkHazards(deps, &candidate, hazards); err != nil {
			// TODO: replace with deposit-only block if ErrConflict, ErrCycle, or ErrFuture
			return eth.Bytes32{}, err
		}
		consolidatedChains = append(consolidatedChains, eth.ChainIDAndOutput{
			ChainID: chain.ChainID,
			// TODO: when applicable, use the deposit-only output root
			Output: chain.Output,
		})
	}
	consolidatedSuper := &eth.SuperV1{
		Timestamp: superRoot.Timestamp,
		Chains:    consolidatedChains,
	}
	return eth.SuperRoot(consolidatedSuper), nil
}

type ConsolidateCheckDeps interface {
	cross.UnsafeFrontierCheckDeps
	cross.CycleCheckDeps
}

func checkHazards(
	deps ConsolidateCheckDeps,
	candidate *supervisortypes.BlockSeal,
	hazards map[supervisortypes.ChainIndex]supervisortypes.BlockSeal,
) error {
	// TODO: for each of these hazard checks, ensure that they're scoped to the super root timestamp of the chain
	if err := cross.HazardUnsafeFrontierChecks(deps, hazards); err != nil {
		return err
	}
	if err := cross.HazardCycleChecks(deps, candidate.Timestamp, hazards); err != nil {
		return err
	}
	return nil
}

// getHazards treats all executing messages as hazards
func getHazards(
	blockHash common.Hash,
	execMsgs []*supervisortypes.ExecutingMessage,
) (map[supervisortypes.ChainIndex]supervisortypes.BlockSeal, error) {
	hazards := make(map[supervisortypes.ChainIndex]supervisortypes.BlockSeal)
	for _, execMsg := range execMsgs {
		hazards[execMsg.Chain] = supervisortypes.BlockSeal{
			Hash:      blockHash,
			Number:    execMsg.BlockNum,
			Timestamp: execMsg.Timestamp,
		}
	}
	return hazards, nil
}

type consolidateCheckDeps struct {
	oracle l2.Oracle
	heads  map[supervisortypes.ChainID]*ethtypes.Block
	depset depset.DependencySet
}

func newConsolidateCheckDeps(chains []eth.ChainIDAndOutput, oracle l2.Oracle) *consolidateCheckDeps {
	// TODO: handle case where dep set changes in a given timestamp
	// TODO: Also replace dep set stubs with the actual dependency set in the RollupConfig.
	deps := make(map[supervisortypes.ChainID]*depset.StaticConfigDependency)
	heads := make(map[supervisortypes.ChainID]*ethtypes.Block)
	for i, chain := range chains {
		deps[supervisortypes.ChainIDFromUInt64(chain.ChainID)] = &depset.StaticConfigDependency{
			ChainIndex:     supervisortypes.ChainIndex(i),
			ActivationTime: 0,
			HistoryMinTime: 0,
		}
		output := oracle.OutputByRoot(common.Hash(chain.Output), chain.ChainID)
		outputV0, ok := output.(*eth.OutputV0)
		if !ok {
			// TODO: return an error instead
			panic(fmt.Sprintf("unexpected output type: %T", output))
		}
		head := oracle.BlockByHash(outputV0.BlockHash, chain.ChainID)
		heads[supervisortypes.ChainIDFromUInt64(chain.ChainID)] = head
	}
	depset, err := depset.NewStaticConfigDependencySet(deps)
	if err != nil {
		panic(fmt.Sprintf("unexpected error: failed to create dependency set: %v", err))
	}
	return &consolidateCheckDeps{oracle: oracle, heads: heads, depset: depset}
}

func (d *consolidateCheckDeps) Check(
	chain supervisortypes.ChainID,
	blockNum uint64,
	timestamp uint64,
	logIdx uint32,
	logHash common.Hash,
) (includedIn supervisortypes.BlockSeal, err error) {
	head := d.heads[chain]
	if head == nil {
		return supervisortypes.BlockSeal{}, fmt.Errorf("head not found for chain %v", chain)
	}
	// We can assume the oracle has the block the executing message is in
	block := BlockByNumber(d.oracle, head, blockNum, chain.ToBig().Uint64())
	return supervisortypes.BlockSeal{
		Hash:      block.Hash(),
		Number:    block.NumberU64(),
		Timestamp: block.Time(),
	}, nil
}

func (d *consolidateCheckDeps) IsCrossUnsafe(chainID supervisortypes.ChainID, block eth.BlockID) error {
	// TODO: assumed to be cross-unsafe
	// But if the block a future block, then retunr an error
	return nil
}

func (d *consolidateCheckDeps) IsLocalUnsafe(chainID supervisortypes.ChainID, block eth.BlockID) error {
	// Always assumed to be local-unsafe
	return nil
}

func (d *consolidateCheckDeps) ParentBlock(chainID supervisortypes.ChainID, parentOf eth.BlockID) (parent eth.BlockID, err error) {
	head := d.heads[chainID]
	if head == nil {
		return eth.BlockID{}, fmt.Errorf("head not found for chain %v", chainID)
	}
	block := BlockByNumber(d.oracle, head, parentOf.Number-1, chainID.ToBig().Uint64())
	return eth.BlockID{
		Hash:   block.Hash(),
		Number: block.NumberU64(),
	}, nil
}

func (d *consolidateCheckDeps) OpenBlock(
	chainID supervisortypes.ChainID,
	blockNum uint64,
) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*supervisortypes.ExecutingMessage, err error) {
	head := d.heads[chainID]
	if head == nil {
		return eth.BlockRef{}, 0, nil, fmt.Errorf("head not found for chain %v", chainID)
	}
	block := BlockByNumber(d.oracle, head, blockNum, chainID.ToBig().Uint64())
	ref = eth.BlockRef{
		Hash:   block.Hash(),
		Number: block.NumberU64(),
	}
	receipts := d.oracle.ReceiptsByBlockHash(block.Hash(), chainID.ToBig().Uint64())
	execs, err := ReceiptsToExecutingMessages(receipts)
	if err != nil {
		return eth.BlockRef{}, 0, nil, err
	}
	execMsgs = make(map[uint32]*supervisortypes.ExecutingMessage, len(execs))
	for _, exec := range execs {
		execMsgs[exec.LogIdx] = exec
	}
	return ref, uint32(len(execs)), execMsgs, nil
}

func (d *consolidateCheckDeps) DependencySet() depset.DependencySet {
	return d.depset
}

func BlockByNumber(oracle l2.Oracle, head *ethtypes.Block, blockNum uint64, chainID uint64) *ethtypes.Block {
	if head.NumberU64() < blockNum {
		return nil
	}
	for {
		if head.NumberU64() == blockNum {
			return head
		}
		if blockNum == 0 {
			return nil
		}
		// TODO: maintain a cache at the earliest head
		head = oracle.BlockByHash(head.ParentHash(), chainID)
	}
}

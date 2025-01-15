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

func ReceiptsToExecutingMessages(depset depset.ChainIndexFromID, receipts ethtypes.Receipts) ([]*supervisortypes.ExecutingMessage, uint32, error) {
	var execMsgs []*supervisortypes.ExecutingMessage
	var logCount uint32
	for _, rcpt := range receipts {
		logCount += uint32(len(rcpt.Logs))
		for _, l := range rcpt.Logs {
			execMsg, err := processors.DecodeExecutingMessageLog(l, depset)
			if err != nil {
				return nil, 0, err
			}
			// TODO: e2e test for both executing and non-executing messages in the logs
			if execMsg != nil {
				execMsgs = append(execMsgs, execMsg)
			}
		}
	}
	return execMsgs, logCount, nil
}

func RunConsolidation(deps ConsolidateCheckDeps,
	oracle l2.Oracle,
	transitionState *types.TransitionState,
	superRoot *eth.SuperV1,
) (eth.Bytes32, error) {
	var consolidatedChains []eth.ChainIDAndOutput

	for i, chain := range superRoot.Chains {
		progress := transitionState.PendingProgress[i]

		// TODO(13776): hint block data execution in case the pending progress is not canonical so we can fetch the correct receipts
		block, receipts := oracle.ReceiptsByBlockHash(progress.BlockHash, chain.ChainID)
		execMsgs, _, err := ReceiptsToExecutingMessages(deps.DependencySet(), receipts)
		if err != nil {
			return eth.Bytes32{}, err
		}

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
			// TODO(13776): replace with deposit-only block if ErrConflict, ErrCycle, or ErrFuture
			return eth.Bytes32{}, err
		}
		consolidatedChains = append(consolidatedChains, eth.ChainIDAndOutput{
			ChainID: chain.ChainID,
			// TODO(13776): when applicable, use the deposit-only block output root
			Output: progress.OutputRoot,
		})
	}
	consolidatedSuper := &eth.SuperV1{
		Timestamp: superRoot.Timestamp + 1,
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
	if err := cross.HazardCycleChecks(deps.DependencySet(), deps, candidate.Timestamp, hazards); err != nil {
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
	heads  map[uint64]*ethtypes.Block
	depset depset.DependencySet
	// block by number cache per chain
	hashByNum map[uint64]map[uint64]common.Hash
}

func newConsolidateCheckDeps(chains []eth.ChainIDAndOutput, oracle l2.Oracle) (*consolidateCheckDeps, error) {
	// TODO: handle case where dep set changes in a given timestamp
	// TODO: Also replace dep set stubs with the actual dependency set in the RollupConfig.
	deps := make(map[eth.ChainID]*depset.StaticConfigDependency)
	heads := make(map[uint64]*ethtypes.Block)
	hashByNum := make(map[uint64]map[uint64]common.Hash)

	for i, chain := range chains {
		deps[eth.ChainIDFromUInt64(chain.ChainID)] = &depset.StaticConfigDependency{
			ChainIndex:     supervisortypes.ChainIndex(i),
			ActivationTime: 0,
			HistoryMinTime: 0,
		}
		output := oracle.OutputByRoot(common.Hash(chain.Output), chain.ChainID)
		outputV0, ok := output.(*eth.OutputV0)
		if !ok {
			return nil, fmt.Errorf("unexpected output type: %T", output)
		}
		head := oracle.BlockByHash(outputV0.BlockHash, chain.ChainID)
		heads[chain.ChainID] = head

		hashByNum[chain.ChainID] = make(map[uint64]common.Hash)
		hashByNum[chain.ChainID][head.NumberU64()] = head.Hash()
	}
	depset, err := depset.NewStaticConfigDependencySet(deps)
	if err != nil {
		return nil, fmt.Errorf("unexpected error: failed to create dependency set: %w", err)
	}

	return &consolidateCheckDeps{
		oracle:    oracle,
		heads:     heads,
		depset:    depset,
		hashByNum: hashByNum,
	}, nil
}

func (d *consolidateCheckDeps) Check(
	chain eth.ChainID,
	blockNum uint64,
	timestamp uint64,
	logIdx uint32,
	logHash common.Hash,
) (includedIn supervisortypes.BlockSeal, err error) {
	// We can assume the oracle has the block the executing message is in
	block, err := d.BlockByNumber(d.oracle, blockNum, chain.ToBig().Uint64())
	if err != nil {
		return supervisortypes.BlockSeal{}, err
	}
	return supervisortypes.BlockSeal{
		Hash:      block.Hash(),
		Number:    block.NumberU64(),
		Timestamp: block.Time(),
	}, nil
}

func (d *consolidateCheckDeps) IsCrossUnsafe(chainID eth.ChainID, block eth.BlockID) error {
	// Assumed to be cross-unsafe. And hazard checks will catch any future blocks prior to calling this
	return nil
}

func (d *consolidateCheckDeps) IsLocalUnsafe(chainID eth.ChainID, block eth.BlockID) error {
	// Always assumed to be local-unsafe
	return nil
}

func (d *consolidateCheckDeps) ParentBlock(chainID eth.ChainID, parentOf eth.BlockID) (parent eth.BlockID, err error) {
	block, err := d.BlockByNumber(d.oracle, parentOf.Number-1, chainID.ToBig().Uint64())
	if err != nil {
		return eth.BlockID{}, err
	}
	return eth.BlockID{
		Hash:   block.Hash(),
		Number: block.NumberU64(),
	}, nil
}

func (d *consolidateCheckDeps) OpenBlock(
	chainID eth.ChainID,
	blockNum uint64,
) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*supervisortypes.ExecutingMessage, err error) {
	block, err := d.BlockByNumber(d.oracle, blockNum, chainID.ToBig().Uint64())
	if err != nil {
		return eth.BlockRef{}, 0, nil, err
	}
	ref = eth.BlockRef{
		Hash:   block.Hash(),
		Number: block.NumberU64(),
	}
	_, receipts := d.oracle.ReceiptsByBlockHash(block.Hash(), chainID.ToBig().Uint64())
	execs, logCount, err := ReceiptsToExecutingMessages(d.depset, receipts)
	if err != nil {
		return eth.BlockRef{}, 0, nil, err
	}
	execMsgs = make(map[uint32]*supervisortypes.ExecutingMessage, len(execs))
	for _, exec := range execs {
		execMsgs[exec.LogIdx] = exec
	}
	return ref, uint32(logCount), execMsgs, nil
}

func (d *consolidateCheckDeps) DependencySet() depset.DependencySet {
	return d.depset
}

func (d *consolidateCheckDeps) BlockByNumber(oracle l2.Oracle, blockNum uint64, chainID uint64) (*ethtypes.Block, error) {
	head := d.heads[chainID]
	if head == nil {
		return nil, fmt.Errorf("head not found for chain %v", chainID)
	}
	if head.NumberU64() < blockNum {
		return nil, nil
	}
	hash, ok := d.hashByNum[chainID][blockNum]
	if ok {
		return oracle.BlockByHash(hash, chainID), nil
	}
	for head.NumberU64() > blockNum {
		head = oracle.BlockByHash(head.ParentHash(), chainID)
		d.hashByNum[chainID][head.NumberU64()] = head.Hash()
	}
	return head, nil
}

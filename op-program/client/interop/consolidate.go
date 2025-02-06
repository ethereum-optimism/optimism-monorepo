package interop

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-program/client/boot"
	"github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-program/client/l1"
	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/cross"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/processors"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
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

func fetchAgreedBlockHashes(oracle l2.Oracle, superRoot *eth.SuperV1) ([]common.Hash, error) {
	agreedBlockHashes := make([]common.Hash, len(superRoot.Chains))
	for i, chain := range superRoot.Chains {
		output := oracle.OutputByRoot(common.Hash(chain.Output), chain.ChainID)
		outputV0, ok := output.(*eth.OutputV0)
		if !ok {
			return nil, fmt.Errorf("unsupported L2 output version: %d", output.Version())
		}
		agreedBlockHashes[i] = common.Hash(outputV0.BlockHash)
	}
	return agreedBlockHashes, nil
}

func RunConsolidation(
	logger log.Logger,
	bootInfo *boot.BootInfoInterop,
	l1PreimageOracle l1.Oracle,
	l2PreimageOracle l2.Oracle,
	transitionState *types.TransitionState,
	superRoot *eth.SuperV1,
	tasks taskExecutor,
) (eth.Bytes32, error) {
	deps, err := newConsolidateCheckDeps(transitionState, superRoot.Chains, l2PreimageOracle)
	if err != nil {
		return eth.Bytes32{}, fmt.Errorf("failed to create consolidate check deps: %w", err)
	}

	var consolidatedChains []eth.ChainIDAndOutput

	agreedBlockHashes, err := fetchAgreedBlockHashes(l2PreimageOracle, superRoot)
	if err != nil {
		return eth.Bytes32{}, err
	}

	for i, chain := range superRoot.Chains {
		progress := transitionState.PendingProgress[i]

		// It's possible that the optimistic block is not canonical.
		// So we use the blockDataByHash hint to trigger a block rebuild to ensure that the block data, including receipts, are available.
		_ = l2PreimageOracle.BlockDataByHash(agreedBlockHashes[i], progress.BlockHash, chain.ChainID)

		optimisticBlock, receipts := l2PreimageOracle.ReceiptsByBlockHash(progress.BlockHash, chain.ChainID)
		execMsgs, _, err := ReceiptsToExecutingMessages(deps.DependencySet(), receipts)
		if err != nil {
			return eth.Bytes32{}, err
		}

		candidate := supervisortypes.BlockSeal{
			Hash:      progress.BlockHash,
			Number:    optimisticBlock.NumberU64(),
			Timestamp: optimisticBlock.Time(),
		}
		consolidatedOutputRoot := progress.OutputRoot
		if err := checkHazards(deps, candidate, chain.ChainID, execMsgs); err != nil {
			if !isInvalidMessageError(err) {
				return eth.Bytes32{}, err
			}
			chainAgreedPrestate := superRoot.Chains[i]
			_, outputRoot, err := buildDepositOnlyBlock(
				logger,
				bootInfo,
				l1PreimageOracle,
				l2PreimageOracle,
				chainAgreedPrestate,
				tasks,
				optimisticBlock,
			)
			if err != nil {
				return eth.Bytes32{}, err
			}
			consolidatedOutputRoot = outputRoot
		}
		consolidatedChains = append(consolidatedChains, eth.ChainIDAndOutput{
			ChainID: chain.ChainID,
			Output:  consolidatedOutputRoot,
		})
	}
	consolidatedSuper := &eth.SuperV1{
		Timestamp: superRoot.Timestamp + 1,
		Chains:    consolidatedChains,
	}
	return eth.SuperRoot(consolidatedSuper), nil
}

func isInvalidMessageError(err error) bool {
	// TODO(#14011): Create an error category for InvalidExecutingMessage errors in the cross package for easier maintenance.
	return errors.Is(err, supervisortypes.ErrConflict) ||
		errors.Is(err, cross.ErrExecMsgHasInvalidIndex) ||
		errors.Is(err, cross.ErrExecMsgUnknownChain) ||
		errors.Is(err, cross.ErrCycle)
}

type ConsolidateCheckDeps interface {
	cross.UnsafeFrontierCheckDeps
	cross.CycleCheckDeps
	Contains(chain eth.ChainID, query supervisortypes.ContainsQuery) (includedIn supervisortypes.BlockSeal, err error)
}

func checkHazards(
	deps ConsolidateCheckDeps,
	candidate supervisortypes.BlockSeal,
	chainID eth.ChainID,
	execMsgs []*supervisortypes.ExecutingMessage,
) error {
	hazards, err := cross.CrossUnsafeHazards(deps, chainID, candidate, execMsgs)
	if err != nil {
		return err
	}
	if err := cross.HazardUnsafeFrontierChecks(deps, hazards); err != nil {
		return err
	}
	if err := cross.HazardCycleChecks(deps.DependencySet(), deps, candidate.Timestamp, hazards); err != nil {
		return err
	}
	return nil
}

type consolidateCheckDeps struct {
	oracle      l2.Oracle
	depset      depset.DependencySet
	canonBlocks map[eth.ChainID]*l2.CanonicalBlockHeaderOracle
}

func newConsolidateCheckDeps(transitionState *types.TransitionState, chains []eth.ChainIDAndOutput, oracle l2.Oracle) (*consolidateCheckDeps, error) {
	// TODO: handle case where dep set changes in a given timestamp
	// TODO: Also replace dep set stubs with the actual dependency set in the RollupConfig.
	deps := make(map[eth.ChainID]*depset.StaticConfigDependency)
	for i, chain := range chains {
		deps[chain.ChainID] = &depset.StaticConfigDependency{
			ChainIndex:     supervisortypes.ChainIndex(i),
			ActivationTime: 0,
			HistoryMinTime: 0,
		}
	}

	canonBlocks := make(map[eth.ChainID]*l2.CanonicalBlockHeaderOracle)
	for i, chain := range chains {
		progress := transitionState.PendingProgress[i]
		// This is the optimistic head. It's OK if it's replaced by a deposits-only block.
		// Because by then the replacement block won't be used for hazard checks.
		// TODO(#14012): for extra safety, ensure the l2 oracle used for checks isn't affected by block reexec.
		head := oracle.BlockByHash(progress.BlockHash, chain.ChainID)
		blockByHash := func(hash common.Hash) *ethtypes.Block {
			return oracle.BlockByHash(hash, chain.ChainID)
		}
		canonBlocks[chain.ChainID] = l2.NewCanonicalBlockHeaderOracle(head.Header(), blockByHash)
	}

	depset, err := depset.NewStaticConfigDependencySet(deps)
	if err != nil {
		return nil, fmt.Errorf("unexpected error: failed to create dependency set: %w", err)
	}

	return &consolidateCheckDeps{
		oracle:      oracle,
		depset:      depset,
		canonBlocks: canonBlocks,
	}, nil
}

func (d *consolidateCheckDeps) Contains(chain eth.ChainID, query supervisortypes.ContainsQuery) (includedIn supervisortypes.BlockSeal, err error) {
	// We can assume the oracle has the block the executing message is in
	block, err := d.BlockByNumber(d.oracle, query.BlockNum, chain)
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
	block, err := d.BlockByNumber(d.oracle, parentOf.Number-1, chainID)
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
	block, err := d.BlockByNumber(d.oracle, blockNum, chainID)
	if err != nil {
		return eth.BlockRef{}, 0, nil, err
	}
	ref = eth.BlockRef{
		Hash:   block.Hash(),
		Number: block.NumberU64(),
	}
	_, receipts := d.oracle.ReceiptsByBlockHash(block.Hash(), chainID)
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

func (d *consolidateCheckDeps) BlockByNumber(oracle l2.Oracle, blockNum uint64, chainID eth.ChainID) (*ethtypes.Block, error) {
	head := d.canonBlocks[chainID].GetHeaderByNumber(blockNum)
	if head == nil {
		return nil, fmt.Errorf("head not found for chain %v", chainID)
	}
	return d.oracle.BlockByHash(head.Hash(), chainID), nil
}

var _ ConsolidateCheckDeps = (*consolidateCheckDeps)(nil)

func buildDepositOnlyBlock(
	logger log.Logger,
	bootInfo *boot.BootInfoInterop,
	l1PreimageOracle l1.Oracle,
	l2PreimageOracle l2.Oracle,
	chainAgreedPrestate eth.ChainIDAndOutput,
	tasks taskExecutor,
	optimisticBlock *ethtypes.Block,
) (common.Hash, eth.Bytes32, error) {
	rollupCfg, err := bootInfo.Configs.RollupConfig(chainAgreedPrestate.ChainID)
	if err != nil {
		return common.Hash{}, eth.Bytes32{}, fmt.Errorf("no rollup config available for chain ID %v: %w", chainAgreedPrestate.ChainID, err)
	}
	l2ChainConfig, err := bootInfo.Configs.ChainConfig(chainAgreedPrestate.ChainID)
	if err != nil {
		return common.Hash{}, eth.Bytes32{}, fmt.Errorf("no chain config available for chain ID %v: %w", chainAgreedPrestate.ChainID, err)
	}
	blockHash, outputRoot, err := tasks.BuildDepositOnlyBlock(
		logger,
		rollupCfg,
		l2ChainConfig,
		bootInfo.L1Head,
		chainAgreedPrestate.Output,
		l1PreimageOracle,
		l2PreimageOracle,
		optimisticBlock,
	)
	if err != nil {
		return common.Hash{}, eth.Bytes32{}, err
	}
	return blockHash, outputRoot, nil
}

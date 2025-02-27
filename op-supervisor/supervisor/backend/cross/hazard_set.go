package cross

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type HazardDeps interface {
	Contains(chain eth.ChainID, query types.ContainsQuery) (types.BlockSeal, error)
	DependencySet() depset.DependencySet
	IsCrossValidBlock(chainID eth.ChainID, block eth.BlockID) error
	OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
}

// HazardSet tracks blocks that must be checked before a candidate can be promoted
type HazardSet struct {
	entries map[types.ChainIndex]types.BlockSeal
}

// NewHazardSet creates a new HazardSet with the given dependencies and initial block
func NewHazardSet(deps HazardDeps, logger log.Logger, chainID eth.ChainID, block types.BlockSeal) (*HazardSet, error) {
	if deps == nil {
		return nil, fmt.Errorf("hazard dependencies cannot be nil")
	}
	h := &HazardSet{
		entries: make(map[types.ChainIndex]types.BlockSeal),
	}
	logger.Debug("Building new HazardSet", "chainID", chainID, "block", block)
	if err := h.build(deps, logger, chainID, block); err != nil {
		return nil, fmt.Errorf("failed to build hazard set: %w", err)
	}
	logger.Debug("Successfully built HazardSet", "chainID", chainID, "block", block)
	return h, nil
}

func NewHazardSetFromEntries(entries map[types.ChainIndex]types.BlockSeal) *HazardSet {
	return &HazardSet{entries: entries}
}

// potentialHazard represents a block that needs to be processed for hazards
type potentialHazard struct {
	chainID eth.ChainID
	block   types.BlockSeal
}

// checkChainCanExecute verifies that a chain can execute messages at a given timestamp.
// If there are any executing messages, then the chain must be able to execute at the timestamp.
func (h *HazardSet) checkChainCanExecute(depSet depset.DependencySet, chainID eth.ChainID, block types.BlockSeal, execMsgs map[uint32]*types.ExecutingMessage) error {
	if len(execMsgs) > 0 {
		if ok, err := depSet.CanExecuteAt(chainID, block.Timestamp); err != nil {
			return fmt.Errorf("cannot check message execution of block %s (chain %s): %w", block, chainID, err)
		} else if !ok {
			return fmt.Errorf("cannot execute messages in block %s (chain %s): %w", block, chainID, types.ErrConflict)
		}
	}
	return nil
}

// checkChainCanInitiate verifies that a chain can initiate messages at a given timestamp.
// The chain must be able to initiate at the timestamp of the message we're referencing.
func (h *HazardSet) checkChainCanInitiate(depSet depset.DependencySet, initChainID eth.ChainID, candidate types.BlockSeal, msg *types.ExecutingMessage) error {
	if ok, err := depSet.CanInitiateAt(initChainID, msg.Timestamp); err != nil {
		return fmt.Errorf("cannot check message initiation of msg %s (chain %s): %w", msg, initChainID, err)
	} else if !ok {
		return fmt.Errorf("cannot allow initiating message %s (chain %s): %w", msg, initChainID, types.ErrConflict)
	}
	return nil
}

// checkMessageWithOlderTimestamp handles messages from past blocks.
// It ensures non-cyclic ordering relative to other messages.
func (h *HazardSet) checkMessageWithOlderTimestamp(deps HazardDeps, msg *types.ExecutingMessage, initChainID eth.ChainID, includedIn types.BlockSeal, candidateTimestamp uint64) error {
	if err := deps.IsCrossValidBlock(initChainID, includedIn.ID()); err != nil {
		return fmt.Errorf("msg %s included in non-cross-safe block %s: %w", msg, includedIn, err)
	}
	// Run expiry window invariant check *after* verifying that the message is non-conflicting.
	expiresAt := msg.Timestamp + deps.DependencySet().MessageExpiryWindow()
	if expiresAt < candidateTimestamp {
		return fmt.Errorf("timestamp of message %s (chain %s) has expired: %d < %d: %w", msg, initChainID, expiresAt, candidateTimestamp, types.ErrConflict)
	}
	return nil
}

// checkMessageWithCurrentTimestamp handles messages from the same time as the candidate block.
// We have to inspect ordering of individual log events to ensure non-cyclic cross-chain message ordering.
// And since we may have back-and-forth messaging, we cannot wait till the initiating side is cross-safe.
// Thus check that it was included in a local-safe block, and then proceed with transitive block checks,
// to ensure the local block we depend on is becoming cross-safe also.
// Also returns a boolean indicating if the message already exists in the hazard set.
func (h *HazardSet) checkMessageWithCurrentTimestamp(msg *types.ExecutingMessage, initChainID eth.ChainID, includedIn types.BlockSeal) (bool, error) {
	existing, ok := h.entries[msg.Chain]
	if ok {
		if existing.ID() != includedIn.ID() {
			return true, fmt.Errorf("found dependency on %s (chain %d), but already depend on %s", includedIn, initChainID, existing)
		}
	}
	return ok, nil
}

// build adds a block to the hazard set and recursively adds any blocks that it depends on.
// Warning for future: If we have sub-second distinct blocks (different block number),
// we need to increase precision on the above timestamp invariant.
// Otherwise a local block can depend on a future local block of the same chain,
// simply by pulling in a block of another chain,
// which then depends on a block of the original chain,
// all with the same timestamp, without message cycles.
func (h *HazardSet) build(deps HazardDeps, logger log.Logger, chainID eth.ChainID, block types.BlockSeal) error {
	depSet := deps.DependencySet()
	stack := []potentialHazard{{chainID: chainID, block: block}}

	for len(stack) > 0 {
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		candidate := next.block
		destChainID := next.chainID
		logger.Debug("Processing block for hazards", "chainID", destChainID, "block", candidate)

		// Get the block and ensure it's allowed to execute messages.
		opened, _, execMsgs, err := deps.OpenBlock(destChainID, candidate.Number)
		if err != nil {
			return fmt.Errorf("failed to open block: %w", err)
		}
		if opened.ID() != candidate.ID() {
			return fmt.Errorf("unsafe L2 DB has %s, but candidate cross-safe was %s: %w", opened, candidate, types.ErrConflict)
		}
		if err := h.checkChainCanExecute(depSet, destChainID, candidate, execMsgs); err != nil {
			return err
		}

		for _, msg := range execMsgs {
			logger.Debug("Processing message", "chainID", destChainID, "block", candidate, "msg", msg)

			// Get the source chain, ensure it's allowed to initiate messages, and contains the initiating message.
			srcChainID, err := depSet.ChainIDFromIndex(msg.Chain)
			if err != nil {
				if errors.Is(err, types.ErrUnknownChain) {
					err = fmt.Errorf("msg %s may not execute from unknown chain %s: %w", msg, msg.Chain, types.ErrConflict)
				}
				return err
			}
			if err := h.checkChainCanInitiate(depSet, srcChainID, candidate, msg); err != nil {
				return err
			}
			includedIn, err := deps.Contains(srcChainID,
				types.ContainsQuery{
					Timestamp: msg.Timestamp,
					BlockNum:  msg.BlockNum,
					LogIdx:    msg.LogIdx,
					LogHash:   msg.Hash,
				})
			if err != nil {
				return fmt.Errorf("executing msg %s failed inclusion check: %w", msg, err)
			}

			if msg.Timestamp < candidate.Timestamp {
				if err := h.checkMessageWithOlderTimestamp(deps, msg, srcChainID, includedIn, candidate.Timestamp); err != nil {
					return err
				}
			} else if msg.Timestamp == candidate.Timestamp {
				exists, err := h.checkMessageWithCurrentTimestamp(msg, srcChainID, includedIn)
				if err != nil {
					return err
				}

				if !exists {
					logger.Debug("Adding block to the hazard set", "chainID", srcChainID, "block", includedIn)
					h.entries[msg.Chain] = includedIn
					stack = append(stack, potentialHazard{
						chainID: srcChainID,
						block:   includedIn,
					})
				}
			} else {
				return fmt.Errorf("executing message %s in %s breaks timestamp invariant", msg, candidate)
			}
		}
	}
	return nil
}

func (h *HazardSet) Entries() map[types.ChainIndex]types.BlockSeal {
	if h == nil {
		return nil
	}
	return h.entries
}

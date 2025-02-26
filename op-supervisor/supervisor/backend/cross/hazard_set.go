package cross

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// MaxHazardBlockChecks is the maximum number of blocks that can be processed when building a hazard set.
// It is a safety limit to prevent potential infinite loops or excessive resource consumption.
const MaxHazardBlockChecks = 10000

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

// build adds a block to the hazard set and recursively adds any blocks that it depends on.
// If a block has already been added, it will be skipped.
func (h *HazardSet) build(deps HazardDeps, logger log.Logger, chainID eth.ChainID, block types.BlockSeal) error {
	// Warning for future: If we have sub-second distinct blocks (different block number),
	// we need to increase precision on the above timestamp invariant.
	// Otherwise a local block can depend on a future local block of the same chain,
	// simply by pulling in a block of another chain,
	// which then depends on a block of the original chain,
	// all with the same timestamp, without message cycles.

	// Process blocks until the stack is empty or we hit the limit
	depSet := deps.DependencySet()
	stack := []potentialHazard{{chainID: chainID, block: block}}
	blocksProcessed := 0
	for len(stack) > 0 {
		if blocksProcessed >= MaxHazardBlockChecks {
			return fmt.Errorf("exceeded maximum number of blocks to process (%d): potential cycle or excessive dependencies", MaxHazardBlockChecks)
		}
		blocksProcessed++

		// Get the next block from the stack
		next := stack[len(stack)-1]
		candidate := next.block
		stack = stack[:len(stack)-1]
		logger.Debug("Processing block for hazards", "chainID", next.chainID, "block", candidate, "processed", blocksProcessed)

		// Open the block to get its messages
		opened, _, execMsgs, err := deps.OpenBlock(next.chainID, candidate.Number)
		if err != nil {
			return fmt.Errorf("failed to open block: %w", err)
		}
		if opened.ID() != candidate.ID() {
			return fmt.Errorf("unsafe L2 DB has %s, but candidate cross-safe was %s: %w", opened, candidate, types.ErrConflict)
		}
		logger.Debug("Block opened", "chainID", next.chainID, "block", opened, "execMsgs", len(execMsgs))

		// invariant: if there are any executing messages, then the chain must be able to execute at the timestamp
		if len(execMsgs) > 0 {
			if ok, err := depSet.CanExecuteAt(next.chainID, candidate.Timestamp); err != nil {
				return fmt.Errorf("cannot check message execution of block %s (chain %s): %w", candidate, next.chainID, err)
			} else if !ok {
				return fmt.Errorf("cannot execute messages in block %s (chain %s): %w", candidate, next.chainID, types.ErrConflict)
			}
		}

		// check all executing messages
		for _, msg := range execMsgs {
			logger.Debug("Processing executing message", "chainID", next.chainID, "block", candidate, "msg", msg)

			initChainID, err := depSet.ChainIDFromIndex(msg.Chain)
			if err != nil {
				if errors.Is(err, types.ErrUnknownChain) {
					err = fmt.Errorf("msg %s may not execute from unknown chain %s: %w", msg, msg.Chain, types.ErrConflict)
				}
				return err
			}

			// invariant: the chain must be able to initiate at the timestamp of the message we're referencing
			if ok, err := depSet.CanInitiateAt(initChainID, msg.Timestamp); err != nil {
				return fmt.Errorf("cannot check message initiation of msg %s (chain %s): %w", msg, chainID, err)
			} else if !ok {
				return fmt.Errorf("cannot allow initiating message %s (chain %s): %w", msg, chainID, types.ErrConflict)
			}

			// invariant: the message must not be in the future
			if msg.Timestamp > candidate.Timestamp {
				return fmt.Errorf("executing message %s in %s breaks timestamp invariant", msg, candidate)
			}

			includedIn, err := deps.Contains(initChainID,
				types.ContainsQuery{
					Timestamp: msg.Timestamp,
					BlockNum:  msg.BlockNum,
					LogIdx:    msg.LogIdx,
					LogHash:   msg.Hash,
				})
			if err != nil {
				return fmt.Errorf("executing msg %s failed check: %w", msg, err)
			}

			if msg.Timestamp < candidate.Timestamp {
				// If timestamp is older: invariant ensures non-cyclic ordering relative to other messages.
				// Ensure that the block that they are included in is cross-safe.
				if err := deps.IsCrossValidBlock(initChainID, includedIn.ID()); err != nil {
					return fmt.Errorf("msg %s included in non-cross-safe block %s: %w", msg, includedIn, err)
				}
			} else if msg.Timestamp == candidate.Timestamp {
				// If timestamp is equal: we have to inspect ordering of individual
				// log events to ensure non-cyclic cross-chain message ordering.
				// And since we may have back-and-forth messaging, we cannot wait till the initiating side is cross-safe.
				// Thus check that it was included in a local-safe block,
				// and then proceed with transitive block checks,
				// to ensure the local block we depend on is becoming cross-safe also.
				logger.Debug("Checking message with current timestamp", "msg", msg, "candidate", candidate)

				if existing, ok := h.entries[msg.Chain]; ok {
					if existing.ID() != includedIn.ID() {
						return fmt.Errorf("found dependency on %s (chain %d), but already depend on %s", includedIn, initChainID, chainID)
					}
				} else {
					// If we got here we have a hazard block so add it to the set and stack
					logger.Debug("Adding hazard block into HazardSet", "chainID", initChainID, "block", includedIn)
					h.entries[msg.Chain] = includedIn

					stack = append(stack, potentialHazard{
						chainID: initChainID,
						block:   includedIn,
					})
				}
			}
		}
	}

	logger.Debug("Successfully built HazardSet", "chainID", chainID, "block", block, "hazards", len(h.entries))
	return nil
}

func (h *HazardSet) Entries() map[types.ChainIndex]types.BlockSeal {
	if h == nil {
		return nil
	}
	return h.entries
}

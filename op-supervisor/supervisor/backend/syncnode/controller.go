package syncnode

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
)

type chainsDB interface {
	LocalSafe(types.ChainID) (types.BlockSeal, types.BlockSeal, error)
	UpdateLocalSafe(types.ChainID, eth.BlockRef, eth.BlockRef) error
	UpdateCrossSafe(types.ChainID, eth.BlockRef, eth.BlockRef) error
}

// SyncNodeController handles the sync node operations across multiple sync nodes
type SyncNodesController struct {
	logger      log.Logger
	controllers locks.RWMap[types.ChainID, SyncControl]

	db chainsDB

	depSet depset.DependencySet
}

// NewSyncNodeController creates a new SyncNodeController
func NewSyncNodesController(l log.Logger, depset depset.DependencySet, db chainsDB) *SyncNodesController {
	return &SyncNodesController{
		logger: l,
		depSet: depset,
		db:     db,
	}
}

func (snc *SyncNodesController) AttachNodeController(id types.ChainID, ctrl SyncControl) error {
	if !snc.depSet.HasChain(id) {
		return fmt.Errorf("chain %v not in dependency set", id)
	}
	snc.controllers.Set(id, ctrl)
	snc.maybeInit(id)
	return nil
}

// maybeInit initializes the chain database if it is not already initialized
// it checks if the Local Safe database is empty, and loads it with the Anchor Point if so
func (snc *SyncNodesController) maybeInit(id types.ChainID) {
	_, _, err := snc.db.LocalSafe(id)
	if errors.Is(err, types.ErrFuture) {
		snc.logger.Debug("initializing chain database", "chain", id)
		ctrl, ok := snc.controllers.Get(id)
		if !ok {
			snc.logger.Warn("missing controller for chain. Not initializing", "chain", id)
			return
		}
		pair, err := ctrl.AnchorPoint(context.Background())
		if err != nil {
			snc.logger.Warn("failed to get anchor point", "chain", id, "error", err)
			return
		}
		if err := snc.db.UpdateCrossSafe(id, pair.Derived, pair.Derived); err != nil {
			snc.logger.Warn("failed to initialize cross safe", "chain", id, "error", err)
		}
		if err := snc.db.UpdateLocalSafe(id, pair.DerivedFrom, pair.Derived); err != nil {
			snc.logger.Warn("failed to initialize local safe", "chain", id, "error", err)
		}
		snc.logger.Debug("initialized chain database", "chain", id, "anchor", pair)
	} else {
		snc.logger.Debug("chain database already initialized", "chain", id)
	}
}

// DeriveFromL1 derives the L2 blocks from the L1 block reference for all the chains
// if any chain fails to derive, the first error is returned
func (snc *SyncNodesController) DeriveFromL1(ref eth.BlockRef) error {
	snc.logger.Debug("deriving from L1", "ref", ref)
	returns := make(chan error, len(snc.depSet.Chains()))
	wg := sync.WaitGroup{}
	// for now this function just prints all the chain-ids of controlled nodes, as a placeholder
	for _, chain := range snc.depSet.Chains() {
		wg.Add(1)
		go func() {
			returns <- snc.DeriveToEnd(chain, ref)
			wg.Done()
		}()
	}
	wg.Wait()
	// collect all errors
	errors := []error{}
	for i := 0; i < len(snc.depSet.Chains()); i++ {
		err := <-returns
		if err != nil {
			errors = append(errors, err)
		}
	}
	// log all errors, but only return the first one
	if len(errors) > 0 {
		snc.logger.Warn("sync nodes failed to derive from L1", "errors", errors)
		return errors[0]
	}
	return nil
}

// DeriveToEnd derives the L2 blocks from the L1 block reference for a single chain
// it will continue to derive until no more blocks are derived
func (snc *SyncNodesController) DeriveToEnd(id types.ChainID, ref eth.BlockRef) error {
	ctrl, ok := snc.controllers.Get(id)
	if !ok {
		snc.logger.Warn("missing controller for chain. Not attempting derivation", "chain", id)
		return nil // maybe return an error?
	}
	for {
		derived, err := ctrl.TryDeriveNext(context.Background(), ref)
		if err != nil {
			return err
		}
		// if no more blocks are derived, we are done
		// (or something? this exact behavior is yet to be defined by the node)
		if derived == (eth.BlockRef{}) {
			return nil
		}
		// record the new L2 to the local database
		if err := snc.db.UpdateLocalSafe(id, ref, derived); err != nil {
			return err
		}
	}
}

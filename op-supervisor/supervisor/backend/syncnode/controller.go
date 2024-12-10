package syncnode

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/log"
)

// SyncNodeController handles the sync node operations across multiple sync nodes
type SyncNodesController struct {
	logger      log.Logger
	controllers locks.RWMap[types.ChainID, SyncControl]

	depSet depset.DependencySet
}

// NewSyncNodeController creates a new SyncNodeController
func NewSyncNodesController(l log.Logger, depset depset.DependencySet) *SyncNodesController {
	return &SyncNodesController{
		logger: l,
		depSet: depset,
	}
}

func (snc *SyncNodesController) AttachNodeController(id types.ChainID, ctrl SyncControl) error {
	if !snc.depSet.HasChain(id) {
		return fmt.Errorf("chain %v not in dependency set", id)
	}
	snc.controllers.Set(id, ctrl)
	return nil
}

func (snc *SyncNodesController) DeriveFromL1(ref eth.BlockRef) error {
	snc.logger.Debug("deriving from L1", "ref", ref)

	// for now this function just prints all the chain-ids of controlled nodes, as a placeholder
	for _, chain := range snc.depSet.Chains() {
		ctrl, ok := snc.controllers.Get(chain)
		if !ok {
			snc.logger.Warn("missing controller for chain", "chain", chain)
			continue
		}
		cid, err := ctrl.ChainID(context.Background())
		if err != nil {
			snc.logger.Warn("failed to get chain id", "chain", chain, "err", err)
			continue
		}
		snc.logger.Debug("got chain id", "chain", chain, "cid", cid)
	}
	return nil
}

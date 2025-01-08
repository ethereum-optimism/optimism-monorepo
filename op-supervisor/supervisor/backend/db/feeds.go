package db

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	gethevent "github.com/ethereum/go-ethereum/event"
)

func (db *ChainsDB) SubscribeLocalUnsafe(chainID types.ChainID, c chan<- types.BlockSeal) (gethevent.Subscription, error) {
	sub, ok := db.localUnsafeFeeds.Get(chainID)
	if !ok {
		return nil, fmt.Errorf("cannot subscribe to local-unsafe: %w: %s", types.ErrUnknownChain, chainID)
	}
	return sub.Subscribe(c), nil
}

func (db *ChainsDB) SubscribeCrossUnsafe(chainID types.ChainID, c chan<- types.BlockSeal) (gethevent.Subscription, error) {
	sub, ok := db.localUnsafeFeeds.Get(chainID)
	if !ok {
		return nil, fmt.Errorf("cannot subscribe to cross-unsafe: %w: %s", types.ErrUnknownChain, chainID)
	}
	return sub.Subscribe(c), nil
}

func (db *ChainsDB) SubscribeLocalSafe(chainID types.ChainID, c chan<- types.DerivedBlockSealPair) (gethevent.Subscription, error) {
	sub, ok := db.localSafeFeeds.Get(chainID)
	if !ok {
		return nil, fmt.Errorf("cannot subscribe to cross-safe: %w: %s", types.ErrUnknownChain, chainID)
	}
	return sub.Subscribe(c), nil
}

func (db *ChainsDB) SubscribeCrossSafe(chainID types.ChainID, c chan<- types.DerivedBlockSealPair) (gethevent.Subscription, error) {
	sub, ok := db.crossSafeFeeds.Get(chainID)
	if !ok {
		return nil, fmt.Errorf("cannot subscribe to cross-safe: %w: %s", types.ErrUnknownChain, chainID)
	}
	return sub.Subscribe(c), nil
}

func (db *ChainsDB) SubscribeFinalized(chainID types.ChainID, c chan<- types.BlockSeal) (gethevent.Subscription, error) {
	sub, ok := db.l2FinalityFeeds.Get(chainID)
	if !ok {
		return nil, fmt.Errorf("cannot subscribe to finalized: %w: %s", types.ErrUnknownChain, chainID)
	}
	return sub.Subscribe(c), nil
}

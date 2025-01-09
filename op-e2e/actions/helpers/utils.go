package helpers

import (
	"github.com/ethereum-optimism/optimism/op-e2e/config"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/node/safedb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
)

func DefaultRollupTestParams() *e2eutils.TestParams {
	return &e2eutils.TestParams{
		MaxSequencerDrift:   40,
		SequencerWindowSize: 120,
		ChannelTimeout:      120,
		L1BlockTime:         15,
		AllocType:           config.DefaultAllocType,
	}
}

var DefaultAlloc = &e2eutils.AllocParams{PrefundTestUsers: true}

type VerifierCfg struct {
	SafeHeadListener safeDB
}

type VerifierOpt func(opts *VerifierCfg)

func WithSafeHeadListener(l safeDB) VerifierOpt {
	return func(opts *VerifierCfg) {
		opts.SafeHeadListener = l
	}
}

func DefaultVerifierCfg() *VerifierCfg {
	return &VerifierCfg{
		SafeHeadListener: safedb.Disabled,
	}
}

func EngineWithP2P() EngineOption {
	return func(ethCfg *ethconfig.Config, nodeCfg *node.Config) error {
		p2pKey, err := crypto.GenerateKey()
		if err != nil {
			return err
		}
		nodeCfg.P2P = p2p.Config{
			MaxPeers:    100,
			NoDiscovery: true,
			ListenAddr:  "127.0.0.1:0",
			PrivateKey:  p2pKey,
		}
		return nil
	}
}

type SequencerCfg struct {
	VerifierCfg
}

func DefaultSequencerConfig() *SequencerCfg {
	return &SequencerCfg{VerifierCfg: *DefaultVerifierCfg()}
}

type SequencerOpt func(opts *SequencerCfg)

func WithVerifierOpts(opts ...VerifierOpt) SequencerOpt {
	return func(cfg *SequencerCfg) {
		for _, opt := range opts {
			opt(&cfg.VerifierCfg)
		}
	}
}

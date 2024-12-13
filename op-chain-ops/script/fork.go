package script

import (
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/forking"
	"github.com/ethereum/go-ethereum/common"
)

// ForkOption modifies a ForkConfig, and can be used by Host internals,
// like the forking cheatcodes, to customize the forking action.
type ForkOption func(cfg *ForkConfig) error

// ForkHook is a callback to the user of the Host,
// to translate an intent to fork into a source of data that can be forked with.
type ForkHook func(opts *ForkConfig) (forking.ForkSource, error)

// ForkConfig is a bundle of data to express a fork intent
type ForkConfig struct {
	URLOrAlias  string
	BlockNumber *uint64      // latest if nil
	Transaction *common.Hash // up to pre-state of given transaction
}

func ForkWithURLOrAlias(urlOrAlias string) ForkOption {
	return func(cfg *ForkConfig) error {
		cfg.URLOrAlias = urlOrAlias
		return nil
	}
}

func ForkWithBlockNumberU256(num *big.Int) ForkOption {
	return func(cfg *ForkConfig) error {
		if !num.IsUint64() {
			return fmt.Errorf("block number %s is too large", num.String())
		}
		v := num.Uint64()
		cfg.BlockNumber = &v
		return nil
	}
}

func ForkWithTransaction(txHash common.Hash) ForkOption {
	return func(cfg *ForkConfig) error {
		cfg.Transaction = &txHash
		return nil
	}
}

// onFork is called by script-internals to translate a fork-intent into forks data-source.
func (h *Host) onFork(opts ...ForkOption) (forking.ForkSource, error) {
	cfg := &ForkConfig{}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	return h.hooks.OnFork(cfg)
}

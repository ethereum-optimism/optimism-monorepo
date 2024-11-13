package script

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/forking"
)

func (c *CheatCodesPrecompile) CreateFork_31ba3498(urlOrAlias string) (*big.Int, error) {
	return c.createFork(ForkWithURLOrAlias(urlOrAlias))
}

func (c *CheatCodesPrecompile) CreateFork_6ba3ba2b(urlOrAlias string, block *big.Int) (*big.Int, error) {
	return c.createFork(ForkWithURLOrAlias(urlOrAlias), ForkWithBlockNumberU256(block))
}

func (c *CheatCodesPrecompile) CreateFork_7ca29682(urlOrAlias string, txHash common.Hash) (*big.Int, error) {
	return c.createFork(ForkWithURLOrAlias(urlOrAlias), ForkWithTransaction(txHash))
}

// createFork implements vm.createFork:
// https://book.getfoundry.sh/cheatcodes/create-fork
func (c *CheatCodesPrecompile) createFork(opts ...ForkOption) (*big.Int, error) {
	src, err := c.h.onFork(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to setup fork source: %w", err)
	}
	id, err := c.h.state.CreateFork(src)
	if err != nil {
		return nil, fmt.Errorf("failed to create fork: %w", err)
	}
	return id.U256().ToBig(), nil
}

func (c *CheatCodesPrecompile) CreateSelectFork_98680034(urlOrAlias string) (*big.Int, error) {
	return c.createSelectFork(ForkWithURLOrAlias(urlOrAlias))
}

func (c *CheatCodesPrecompile) CreateSelectFork_71ee464d(urlOrAlias string, block *big.Int) (*big.Int, error) {
	return c.createSelectFork(ForkWithURLOrAlias(urlOrAlias), ForkWithBlockNumberU256(block))
}

func (c *CheatCodesPrecompile) CreateSelectFork_84d52b7a(urlOrAlias string, txHash common.Hash) (*big.Int, error) {
	return c.createSelectFork(ForkWithURLOrAlias(urlOrAlias), ForkWithTransaction(txHash))
}

// createSelectFork implements vm.createSelectFork:
// https://book.getfoundry.sh/cheatcodes/create-select-fork
func (c *CheatCodesPrecompile) createSelectFork(opts ...ForkOption) (*big.Int, error) {
	return c.h.CreateSelectFork(opts...)
}

// ActiveFork implements vm.activeFork:
// https://book.getfoundry.sh/cheatcodes/active-fork
func (c *CheatCodesPrecompile) ActiveFork() (*uint256.Int, error) {
	id, active := c.h.state.ActiveFork()
	if !active {
		return nil, errors.New("no active fork")
	}
	return id.U256(), nil
}

// convenience method, to repeat the same URLOrAlias as the given fork when setting up a new fork
func (c *CheatCodesPrecompile) forkURLOption(id forking.ForkID) ForkOption {
	return func(cfg *ForkConfig) error {
		urlOrAlias, err := c.h.state.ForkURLOrAlias(id)
		if err != nil {
			return err
		}
		return ForkWithURLOrAlias(urlOrAlias)(cfg)
	}
}

func (c *CheatCodesPrecompile) RollFork_d9bbf3a1(block *big.Int) error {
	id, ok := c.h.state.ActiveFork()
	if !ok {
		return errors.New("no active fork")
	}
	return c.rollFork(id, c.forkURLOption(id), ForkWithBlockNumberU256(block))
}

func (c *CheatCodesPrecompile) RollFork_0f29772b(txHash common.Hash) error {
	id, ok := c.h.state.ActiveFork()
	if !ok {
		return errors.New("no active fork")
	}
	return c.rollFork(id, c.forkURLOption(id), ForkWithTransaction(txHash))
}

func (c *CheatCodesPrecompile) RollFork_d74c83a4(forkID *big.Int, block *big.Int) error {
	id := forking.ForkIDFromBig(forkID)
	return c.rollFork(id, c.forkURLOption(id), ForkWithBlockNumberU256(block))
}

func (c *CheatCodesPrecompile) RollFork_f2830f7b(forkID *uint256.Int, txHash common.Hash) error {
	id := forking.ForkID(*forkID)
	return c.rollFork(id, c.forkURLOption(id), ForkWithTransaction(txHash))
}

// rollFork implements vm.rollFork:
// https://book.getfoundry.sh/cheatcodes/roll-fork
func (c *CheatCodesPrecompile) rollFork(id forking.ForkID, opts ...ForkOption) error {
	src, err := c.h.onFork(opts...)
	if err != nil {
		return fmt.Errorf("cannot setup fork source for roll-fork change: %w", err)
	}
	return c.h.state.ResetFork(id, src)
}

// MakePersistent_57e22dde implements vm.makePersistent:
// https://book.getfoundry.sh/cheatcodes/make-persistent
func (c *CheatCodesPrecompile) MakePersistent_57e22dde(account0 common.Address) {
	c.h.state.MakePersistent(account0)
}

func (c *CheatCodesPrecompile) MakePersistent_4074e0a8(account0, account1 common.Address) {
	c.h.state.MakePersistent(account0)
	c.h.state.MakePersistent(account1)
}

func (c *CheatCodesPrecompile) MakePersistent_efb77a75(account0, account1, account2 common.Address) {
	c.h.state.MakePersistent(account0)
	c.h.state.MakePersistent(account1)
	c.h.state.MakePersistent(account2)
}

func (c *CheatCodesPrecompile) MakePersistent_1d9e269e(accounts []common.Address) {
	for _, addr := range accounts {
		c.h.state.MakePersistent(addr)
	}
}

// RevokePersistent_997a0222 implements vm.revokePersistent:
// https://book.getfoundry.sh/cheatcodes/revoke-persistent
func (c *CheatCodesPrecompile) RevokePersistent_997a0222(addr common.Address) {
	c.h.state.RevokePersistent(addr)
}

func (c *CheatCodesPrecompile) RevokePersistent_3ce969e6(addrs []common.Address) {
	for _, addr := range addrs {
		c.h.state.RevokePersistent(addr)
	}
}

// IsPersistent implements vm.isPersistent:
// https://book.getfoundry.sh/cheatcodes/is-persistent
func (c *CheatCodesPrecompile) IsPersistent(addr common.Address) bool {
	return c.h.state.IsPersistent(addr)
}

// AllowCheatcodes implements vm.allowCheatcodes:
// https://book.getfoundry.sh/cheatcodes/allow-cheatcodes
func (c *CheatCodesPrecompile) AllowCheatcodes(addr common.Address) {
	c.h.AllowCheatcodes(addr)
}

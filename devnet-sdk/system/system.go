package system

import (
	"fmt"
	"slices"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/devnet-sdk/shell/env"
)

type system struct {
	identifier string
	l1         Chain
	l2s        []Chain
}

// system implements System
var _ System = (*system)(nil)

func NewSystemFromURL(url string) (System, error) {
	devnetEnv, err := env.LoadDevnetFromURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet from URL: %w", err)
	}

	sys, err := systemFromDevnet(devnetEnv.Config, devnetEnv.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create system from devnet: %w", err)
	}
	return sys, nil
}

func (s *system) L1() Chain {
	return s.l1
}

func (s *system) L2(chainID uint64) Chain {
	return s.l2s[chainID]
}

func (s *system) Identifier() string {
	return s.identifier
}

func (s *system) addChains(chains ...*descriptors.Chain) error {
	for _, chainDesc := range chains {
		if chainDesc.ID == "" {
			s.l1 = chainFromDescriptor(chainDesc)
		} else {
			s.l2s = append(s.l2s, chainFromDescriptor(chainDesc))
		}
	}
	return nil
}

func systemFromDevnet(dn descriptors.DevnetEnvironment, identifier string) (System, error) {
	sys := &system{identifier: identifier}

	if err := sys.addChains(append(dn.L2, dn.L1)...); err != nil {
		return nil, err
	}

	if slices.Contains(dn.Features, "interop") {
		return &interopSystem{system: sys}, nil
	}
	return sys, nil
}

type interopSystem struct {
	*system
}

// interopSystem implements InteropSystem
var _ InteropSystem = (*interopSystem)(nil)

func (i *interopSystem) InteropSet() InteropSet {
	return i.system // TODO: the interop set might not contain all L2s
}

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

func (s *system) L2s() []Chain {
	return s.l2s
}

func (s *system) Identifier() string {
	return s.identifier
}

func (s *system) addChains(chains ...*descriptors.Chain) error {
	for _, chainDesc := range chains {
		if chainDesc.ID == "" {
			l1, err := chainFromDescriptor(chainDesc)
			if err != nil {
				return fmt.Errorf("failed to add L1 chain: %w", err)
			}
			s.l1 = l1
		} else {
			l2, err := chainFromDescriptor(chainDesc)
			if err != nil {
				return fmt.Errorf("failed to add L2 chain: %w", err)
			}
			s.l2s = append(s.l2s, l2)
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

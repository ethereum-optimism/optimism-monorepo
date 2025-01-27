package system

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
)

type system struct {
	identifier string
	l1         Chain
	l2s        []Chain
}

// system implements System
var _ System = (*system)(nil)

func NewSystemFromEnv(envVar string) (System, error) {
	devnetFile := os.Getenv(envVar)
	if devnetFile == "" {
		return nil, fmt.Errorf("env var '%s' is unset", envVar)
	}
	devnet, err := devnetFromFile(devnetFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse devnet file: %w", err)
	}

	// Extract basename without extension from devnetFile path
	basename := devnetFile
	if lastSlash := strings.LastIndex(basename, "/"); lastSlash >= 0 {
		basename = basename[lastSlash+1:]
	}
	if lastDot := strings.LastIndex(basename, "."); lastDot >= 0 {
		basename = basename[:lastDot]
	}

	sys, err := systemFromDevnet(*devnet, basename)
	if err != nil {
		return nil, fmt.Errorf("failed to create system from devnet file: %w", err)
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

// devnetFromFile reads a DevnetEnvironment from a JSON file.
func devnetFromFile(devnetFile string) (*descriptors.DevnetEnvironment, error) {
	data, err := os.ReadFile(devnetFile)
	if err != nil {
		return nil, fmt.Errorf("error reading devnet file: %w", err)
	}

	var config descriptors.DevnetEnvironment
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}
	return &config, nil
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

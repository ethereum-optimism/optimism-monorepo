package script

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script/addresses"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func checkABI(abiData *abi.ABI, methodSignature string) bool {
	for _, m := range abiData.Methods {
		if m.Sig == methodSignature {
			return true
		}
	}
	return false
}

// WithScript deploys a script contract, at a create-address based on the ScriptDeployer.
// The returned cleanup function wipes the script account again (but not the storage).
func WithScript[B any](h *Host, name string, contract string) (b *B, cleanup func(), err error) {
	// load contract artifact
	artifact, err := h.af.ReadArtifact(name, contract)
	if err != nil {
		return nil, nil, fmt.Errorf("could not load script artifact: %w", err)
	}

	deployer := addresses.ScriptDeployer
	deployNonce := h.state.GetNonce(deployer)
	// compute address of script contract to be deployed
	addr := crypto.CreateAddress(deployer, deployNonce)
	h.Label(addr, contract)
	h.AllowCheatcodes(addr)    // before constructor execution, give our script cheatcode access
	h.state.MakeExcluded(addr) // scripts are persistent across forks

	// init bindings (with ABI check)
	bindings, err := MakeBindings[B](h.ScriptBackendFn(addr), func(abiDef string) bool {
		return checkABI(&artifact.ABI, abiDef)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make bindings: %w", err)
	}

	// Scripts can be very large
	h.EnforceMaxCodeSize(false)
	defer h.EnforceMaxCodeSize(true)
	// deploy the script contract
	deployedAddr, err := h.Create(deployer, artifact.Bytecode.Object)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to deploy script: %w", err)
	}
	if deployedAddr != addr {
		return nil, nil, fmt.Errorf("deployed to unexpected address %s, expected %s", deployedAddr, addr)
	}
	h.RememberArtifact(addr, artifact, contract)
	return bindings, func() {
		h.Wipe(addr)
	}, nil
}

// WithPrecompileAtAddress turns a struct into a precompile,
// and inserts it as override at the given address in the host.
// A cleanup function is returned, to remove the precompile override again.
func WithPrecompileAtAddress[E any](h *Host, addr common.Address, elem E, opts ...PrecompileOption[E]) (cleanup func(), err error) {
	if h.HasPrecompileOverride(addr) {
		return nil, fmt.Errorf("already have existing precompile override at %s", addr)
	}
	precompile, err := NewPrecompile[E](elem, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to construct precompile: %w", err)
	}
	h.SetPrecompile(addr, precompile)
	h.Label(addr, fmt.Sprintf("%T", precompile.Precompile))
	return func() {
		h.SetPrecompile(addr, nil)
	}, nil
}

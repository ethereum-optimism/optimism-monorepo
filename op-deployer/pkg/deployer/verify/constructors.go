package verify

import (
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// GetConstructorArgs returns the ABI‑encoded constructor arguments for the specified contract, so
// that they can be passed to etherscan for contract verification.
func GetConstructorArgs(contractName string, state *state.State) string {
	// Normalize the contract name (for example, to lower-case)
	switch contractName {
	case "ProxyAdminAddress":
		addr := state.AppliedIntent.SuperchainRoles.ProxyAdminOwner
		padded := common.LeftPadBytes(addr.Bytes(), 32)
		return hexutil.Encode(padded)[2:]

	default:
		return ""
	}
}

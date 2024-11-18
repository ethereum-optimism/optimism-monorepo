package script

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/vm"
)

// CheatCodesPrecompile implements the Forge vm cheatcodes.
// Note that forge-std wraps these cheatcodes,
// and provides additional convenience functions that use these cheatcodes.
type CheatCodesPrecompile struct {
	h *Host
}

// AccessControlledPrecompile wraps a precompile,
// and checks that the caller has cheatcode access.
type AccessControlledPrecompile struct {
	h     *Host
	inner vm.PrecompiledContract
}

var _ vm.PrecompiledContract = (*AccessControlledPrecompile)(nil)

func (c *AccessControlledPrecompile) RequiredGas(input []byte) uint64 {
	// call-frame is not open yet, and prank is ignored for cheatcode access-checking.
	accessor := c.h.SelfAddress()
	_, ok := c.h.allowedCheatcodes[accessor]
	if !ok {
		// Don't just return infinite gas, we can allow it to run,
		// and then revert with a proper error message.
		return 0
	}
	return c.inner.RequiredGas(input)
}

func (c *AccessControlledPrecompile) Run(input []byte) ([]byte, error) {
	// call-frame is not open yet, and prank is ignored for cheatcode access-checking.
	accessor := c.h.SelfAddress()
	if !c.h.AllowedCheatcodes(accessor) {
		c.h.log.Error("Cheatcode access denied!", "caller", accessor, "label", c.h.labels[accessor])
		return encodeRevert(fmt.Errorf("call by %s to cheatcode precompile is not allowed", accessor))
	}
	return c.inner.Run(input)
}

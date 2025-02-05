package addresses

import "github.com/ethereum/go-ethereum/common"

var (
	// DefaultSenderAddr is known as DEFAULT_SENDER = address(uint160(uint256(keccak256("foundry default caller"))))
	DefaultSenderAddr = common.HexToAddress("0x1804c8AB1F12E6bbf3894d4083f33e07309d1f38")
	// DefaultScriptAddr is the address of the initial executing script, computed from:
	// cast compute-address --nonce 1 0x1804c8AB1F12E6bbf3894d4083f33e07309d1f38
	DefaultScriptAddr = common.HexToAddress("0x7FA9385bE102ac3EAc297483Dd6233D62b3e1496")
	// VMAddr is known as VM_ADDRESS = address(uint160(uint256(keccak256("hevm cheat code"))));
	VMAddr = common.HexToAddress("0x7109709ECfa91a80626fF3989D68f67F5b1DD12D")
	// ConsoleAddr is known as CONSOLE, "console.log" in ascii.
	// Utils like console.sol and console2.sol work by executing a staticcall to this address.
	ConsoleAddr = common.HexToAddress("0x000000000000000000636F6e736F6c652e6c6f67")
	// ScriptDeployer is used for temporary scripts address(uint160(uint256(keccak256("op-stack script deployer"))))
	ScriptDeployer = common.HexToAddress("0x76Ce131128F3616871f8CDA86d18fAB44E4d0D8B")
	// ForgeDeployer is used by some scripts as a default deployer address, e.g. makeAddr("deployer")
	ForgeDeployer = common.HexToAddress("0xaE0bDc4eEAC5E950B67C6819B118761CaAF61946")
)

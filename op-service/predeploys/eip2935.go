package predeploys

import "github.com/ethereum/go-ethereum/common"

// EIP-4788 defines a deterministic deployment transaction that deploys the beacon-block-roots contract.
// To embed the contract in genesis, we want the deployment-result, not the contract-creation tx input code.
// Since the contract deployment result is deterministic and the same across every chain,
// the bytecode can be easily verified by comparing it with chains like Goerli.
// During deployment it does not modify any contract storage, the storage starts empty.
// See https://goerli.etherscan.io/tx/0xdf52c2d3bbe38820fff7b5eaab3db1b91f8e1412b56497d88388fb5d4ea1fde0
// And https://eips.ethereum.org/EIPS/eip-4788
var (
	EIP2935ContractAddr     = common.HexToAddress("0x0F792be4B0c0cb4DAE440Ef133E90C0eCD48CCCC")
	EIP2935ContractCode     = common.FromHex("0x3373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500")
	EIP2935ContractCodeHash = common.HexToHash("0x6e49e66782037c0555897870e29fa5e552daf4719552131a0abce779daec0a5d")
	EIP2935ContractDeployer = common.HexToAddress("0xE9f0662359Bb2c8111840eFFD73B9AFA77CbDE10")
)

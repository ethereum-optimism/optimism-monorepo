package predeploys

import "github.com/ethereum/go-ethereum/common"

// EIP-2935 defines a deterministic deployment transaction that deploys the recent block hashes contract.
// See https://eips.ethereum.org/EIPS/eip-2935
var (
	EIP2935ContractAddr     = common.HexToAddress("0x0F792be4B0c0cb4DAE440Ef133E90C0eCD48CCCC")
	EIP2935ContractCode     = common.FromHex("0x3373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500")
	EIP2935ContractCodeHash = common.HexToHash("0x6e49e66782037c0555897870e29fa5e552daf4719552131a0abce779daec0a5d")
	EIP2935ContractDeployer = common.HexToAddress("0xE9f0662359Bb2c8111840eFFD73B9AFA77CbDE10")
)

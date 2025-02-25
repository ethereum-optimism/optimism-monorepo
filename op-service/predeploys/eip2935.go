package predeploys

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

// EIP-2935 defines a deterministic deployment transaction that deploys the recent block hashes contract.
// See https://eips.ethereum.org/EIPS/eip-2935
var (
	EIP2935ContractAddr     = params.HistoryStorageAddress
	EIP2935ContractCode     = params.HistoryStorageCode
	EIP2935ContractCodeHash = common.HexToHash("0x6e49e66782037c0555897870e29fa5e552daf4719552131a0abce779daec0a5d")
	EIP2935ContractDeployer = common.HexToAddress("0x3462413Af4609098e1E27A490f554f260213D685")
)

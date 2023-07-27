// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-bindings/solc"
)

const BobaTuringCreditStorageLayoutJSON = "{\"storage\":[{\"astId\":1000,\"contract\":\"contracts/boba/BobaTuringCredit.sol:BobaTuringCredit\",\"label\":\"owner\",\"offset\":0,\"slot\":\"0\",\"type\":\"t_address\"},{\"astId\":1001,\"contract\":\"contracts/boba/BobaTuringCredit.sol:BobaTuringCredit\",\"label\":\"prepaidBalance\",\"offset\":0,\"slot\":\"1\",\"type\":\"t_mapping(t_address,t_uint256)\"},{\"astId\":1002,\"contract\":\"contracts/boba/BobaTuringCredit.sol:BobaTuringCredit\",\"label\":\"turingToken\",\"offset\":0,\"slot\":\"2\",\"type\":\"t_address\"},{\"astId\":1003,\"contract\":\"contracts/boba/BobaTuringCredit.sol:BobaTuringCredit\",\"label\":\"turingPrice\",\"offset\":0,\"slot\":\"3\",\"type\":\"t_uint256\"},{\"astId\":1004,\"contract\":\"contracts/boba/BobaTuringCredit.sol:BobaTuringCredit\",\"label\":\"ownerRevenue\",\"offset\":0,\"slot\":\"4\",\"type\":\"t_uint256\"}],\"types\":{\"t_address\":{\"encoding\":\"inplace\",\"label\":\"address\",\"numberOfBytes\":\"20\"},\"t_mapping(t_address,t_uint256)\":{\"encoding\":\"mapping\",\"label\":\"mapping(address =\u003e uint256)\",\"numberOfBytes\":\"32\",\"key\":\"t_address\",\"value\":\"t_uint256\"},\"t_uint256\":{\"encoding\":\"inplace\",\"label\":\"uint256\",\"numberOfBytes\":\"32\"}}}"

var BobaTuringCreditStorageLayout = new(solc.StorageLayout)

var BobaTuringCreditDeployedBin = "0x608060405234801561001057600080fd5b50600436106100c85760003560e01c80638da5cb5b11610081578063f2fde38b1161005b578063f2fde38b146101b2578063f7cd3be8146101c5578063fd892278146101d857600080fd5b80638da5cb5b14610176578063a52b962d14610196578063e24dfcde146101a957600080fd5b80630ceff204116100b25780630ceff2041461010957806335d6eac41461011e578063853383921461013157600080fd5b8062292526146100cd57806309da3981146100e9575b600080fd5b6100d660045481565b6040519081526020015b60405180910390f35b6100d66100f73660046110b1565b60016020526000908152604090205481565b61011c6101173660046110cc565b6101eb565b005b61011c61012c3660046110b1565b610420565b6002546101519073ffffffffffffffffffffffffffffffffffffffff1681565b60405173ffffffffffffffffffffffffffffffffffffffff90911681526020016100e0565b6000546101519073ffffffffffffffffffffffffffffffffffffffff1681565b6100d66101a43660046110b1565b610589565b6100d660035481565b61011c6101c03660046110b1565b61062f565b61011c6101d33660046110cc565b610771565b61011c6101e63660046110e5565b610818565b60005473ffffffffffffffffffffffffffffffffffffffff16331480610227575060005473ffffffffffffffffffffffffffffffffffffffff16155b610292576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f63616c6c6572206973206e6f7420746865206f776e657200000000000000000060448201526064015b60405180910390fd5b60025473ffffffffffffffffffffffffffffffffffffffff16610337576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602560248201527f436f6e747261637420686173206e6f7420796574206265656e20696e6974696160448201527f6c697a65640000000000000000000000000000000000000000000000000000006064820152608401610289565b6004548111156103a3576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152600e60248201527f496e76616c696420416d6f756e740000000000000000000000000000000000006044820152606401610289565b80600460008282546103b59190611140565b909155505060408051338152602081018390527f447d53be88e315476bdbe2e63cef309461f6305d09aada67641c29e6b897e301910160405180910390a160005460025461041d9173ffffffffffffffffffffffffffffffffffffffff918216911683610aed565b50565b60005473ffffffffffffffffffffffffffffffffffffffff1633148061045c575060005473ffffffffffffffffffffffffffffffffffffffff16155b6104c2576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f63616c6c6572206973206e6f7420746865206f776e65720000000000000000006044820152606401610289565b60025473ffffffffffffffffffffffffffffffffffffffff1615610542576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601d60248201527f436f6e747261637420686173206265656e20696e697469616c697a65640000006044820152606401610289565b600280547fffffffffffffffffffffffff00000000000000000000000000000000000000001673ffffffffffffffffffffffffffffffffffffffff92909216919091179055565b60006003546000036105f7576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601060248201527f556e6c696d6974656420637265646974000000000000000000000000000000006044820152606401610289565b60035473ffffffffffffffffffffffffffffffffffffffff831660009081526001602052604090205461062991610bc6565b92915050565b60005473ffffffffffffffffffffffffffffffffffffffff1633148061066b575060005473ffffffffffffffffffffffffffffffffffffffff16155b6106d1576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f63616c6c6572206973206e6f7420746865206f776e65720000000000000000006044820152606401610289565b73ffffffffffffffffffffffffffffffffffffffff81166106f157600080fd5b600080547fffffffffffffffffffffffff00000000000000000000000000000000000000001673ffffffffffffffffffffffffffffffffffffffff83169081179091556040805133815260208101929092527f5c486528ec3e3f0ea91181cff8116f02bfa350e03b8b6f12e00765adbb5af85c910160405180910390a150565b60005473ffffffffffffffffffffffffffffffffffffffff163314806107ad575060005473ffffffffffffffffffffffffffffffffffffffff16155b610813576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f63616c6c6572206973206e6f7420746865206f776e65720000000000000000006044820152606401610289565b600355565b60025473ffffffffffffffffffffffffffffffffffffffff166108bd576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602560248201527f436f6e747261637420686173206e6f7420796574206265656e20696e6974696160448201527f6c697a65640000000000000000000000000000000000000000000000000000006064820152608401610289565b81600003610927576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152600e60248201527f496e76616c696420616d6f756e740000000000000000000000000000000000006044820152606401610289565b73ffffffffffffffffffffffffffffffffffffffff81163b6109a5576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152600e60248201527f4164647265737320697320454f410000000000000000000000000000000000006044820152606401610289565b6109cf817f2f7adf4300000000000000000000000000000000000000000000000000000000610bd9565b610a35576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f496e76616c69642048656c70657220436f6e74726163740000000000000000006044820152606401610289565b73ffffffffffffffffffffffffffffffffffffffff811660009081526001602052604081208054849290610a6a908490611153565b9091555050604080513381526020810184905273ffffffffffffffffffffffffffffffffffffffff83168183015290517f63611f4b2e0fff4acd8e17bd95ebb62a3bc834c76cf85e7a972a502990b6257a9181900360600190a1600254610ae99073ffffffffffffffffffffffffffffffffffffffff16333085610bf5565b5050565b60405173ffffffffffffffffffffffffffffffffffffffff8316602482015260448101829052610bc19084907fa9059cbb00000000000000000000000000000000000000000000000000000000906064015b604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529190526020810180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167fffffffff0000000000000000000000000000000000000000000000000000000090931692909217909152610c59565b505050565b6000610bd28284611166565b9392505050565b6000610be483610d65565b8015610bd25750610bd28383610dc9565b60405173ffffffffffffffffffffffffffffffffffffffff80851660248301528316604482015260648101829052610c539085907f23b872dd0000000000000000000000000000000000000000000000000000000090608401610b3f565b50505050565b6000610cbb826040518060400160405280602081526020017f5361666545524332303a206c6f772d6c6576656c2063616c6c206661696c65648152508573ffffffffffffffffffffffffffffffffffffffff16610e989092919063ffffffff16565b805190915015610bc15780806020019051810190610cd991906111a1565b610bc1576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602a60248201527f5361666545524332303a204552433230206f7065726174696f6e20646964206e60448201527f6f742073756363656564000000000000000000000000000000000000000000006064820152608401610289565b6000610d91827f01ffc9a700000000000000000000000000000000000000000000000000000000610dc9565b80156106295750610dc2827fffffffff00000000000000000000000000000000000000000000000000000000610dc9565b1592915050565b604080517fffffffff000000000000000000000000000000000000000000000000000000008316602480830191909152825180830390910181526044909101909152602080820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167f01ffc9a700000000000000000000000000000000000000000000000000000000178152825160009392849283928392918391908a617530fa92503d91506000519050828015610e81575060208210155b8015610e8d5750600081115b979650505050505050565b6060610ea78484600085610eaf565b949350505050565b606082471015610f41576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602660248201527f416464726573733a20696e73756666696369656e742062616c616e636520666f60448201527f722063616c6c00000000000000000000000000000000000000000000000000006064820152608401610289565b73ffffffffffffffffffffffffffffffffffffffff85163b610fbf576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601d60248201527f416464726573733a2063616c6c20746f206e6f6e2d636f6e74726163740000006044820152606401610289565b6000808673ffffffffffffffffffffffffffffffffffffffff168587604051610fe891906111e7565b60006040518083038185875af1925050503d8060008114611025576040519150601f19603f3d011682016040523d82523d6000602084013e61102a565b606091505b5091509150610e8d82828660608315611044575081610bd2565b8251156110545782518084602001fd5b816040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016102899190611203565b803573ffffffffffffffffffffffffffffffffffffffff811681146110ac57600080fd5b919050565b6000602082840312156110c357600080fd5b610bd282611088565b6000602082840312156110de57600080fd5b5035919050565b600080604083850312156110f857600080fd5b8235915061110860208401611088565b90509250929050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b8181038181111561062957610629611111565b8082018082111561062957610629611111565b60008261119c577f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fd5b500490565b6000602082840312156111b357600080fd5b81518015158114610bd257600080fd5b60005b838110156111de5781810151838201526020016111c6565b50506000910152565b600082516111f98184602087016111c3565b9190910192915050565b60208152600082518060208401526112228160408501602087016111c3565b601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe016919091016040019291505056fea164736f6c6343000813000a"

func init() {
	if err := json.Unmarshal([]byte(BobaTuringCreditStorageLayoutJSON), BobaTuringCreditStorageLayout); err != nil {
		panic(err)
	}

	layouts["BobaTuringCredit"] = BobaTuringCreditStorageLayout
	deployedBytecodes["BobaTuringCredit"] = BobaTuringCreditDeployedBin
}

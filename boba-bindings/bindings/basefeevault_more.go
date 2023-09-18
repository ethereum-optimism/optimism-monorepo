// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"encoding/json"

	"github.com/bobanetwork/v3-anchorage/boba-bindings/solc"
)

const BaseFeeVaultStorageLayoutJSON = "{\"storage\":[{\"astId\":1000,\"contract\":\"src/L2/BaseFeeVault.sol:BaseFeeVault\",\"label\":\"totalProcessed\",\"offset\":0,\"slot\":\"0\",\"type\":\"t_uint256\"}],\"types\":{\"t_uint256\":{\"encoding\":\"inplace\",\"label\":\"uint256\",\"numberOfBytes\":\"32\"}}}"

var BaseFeeVaultStorageLayout = new(solc.StorageLayout)

var BaseFeeVaultDeployedBin = "0x6080604052600436106100695760003560e01c806384411d651161004357806384411d6514610140578063d0e12f9014610164578063d3e5792b146101a557600080fd5b80630d9019e1146100755780633ccfd60b146100d357806354fd4d50146100ea57600080fd5b3661007057005b600080fd5b34801561008157600080fd5b506100a97f000000000000000000000000000000000000000000000000000000000000000081565b60405173ffffffffffffffffffffffffffffffffffffffff90911681526020015b60405180910390f35b3480156100df57600080fd5b506100e86101d9565b005b3480156100f657600080fd5b506101336040518060400160405280600581526020017f312e342e3000000000000000000000000000000000000000000000000000000081525081565b6040516100ca9190610630565b34801561014c57600080fd5b5061015660005481565b6040519081526020016100ca565b34801561017057600080fd5b506101987f000000000000000000000000000000000000000000000000000000000000000081565b6040516100ca91906106b4565b3480156101b157600080fd5b506101567f000000000000000000000000000000000000000000000000000000000000000081565b7f00000000000000000000000000000000000000000000000000000000000000004710156102b4576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604a60248201527f4665655661756c743a207769746864726177616c20616d6f756e74206d75737460448201527f2062652067726561746572207468616e206d696e696d756d207769746864726160648201527f77616c20616d6f756e7400000000000000000000000000000000000000000000608482015260a4015b60405180910390fd5b6000479050806000808282546102ca91906106c8565b9091555050604080518281527f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff166020820152338183015290517fc8a211cc64b6ed1b50595a9fcb1932b6d1e5a6e8ef15b60e5b1f988ea9086bba9181900360600190a17f38e04cbeb8c10f8f568618aa75be0f10b6729b8b4237743b4de20cbcde2839ee817f0000000000000000000000000000000000000000000000000000000000000000337f00000000000000000000000000000000000000000000000000000000000000006040516103b89493929190610707565b60405180910390a160017f000000000000000000000000000000000000000000000000000000000000000060018111156103f4576103f461064a565b0361050d5760007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff168260405160006040518083038185875af1925050503d8060008114610473576040519150601f19603f3d011682016040523d82523d6000602084013e610478565b606091505b5050905080610509576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603060248201527f4665655661756c743a206661696c656420746f2073656e642045544820746f2060448201527f4c322066656520726563697069656e740000000000000000000000000000000060648201526084016102ab565b5050565b604080516020810182526000815290517fe11013dd0000000000000000000000000000000000000000000000000000000081527342000000000000000000000000000000000000109163e11013dd918491610590917f0000000000000000000000000000000000000000000000000000000000000000916188b891600401610748565b6000604051808303818588803b1580156105a957600080fd5b505af11580156105bd573d6000803e3d6000fd5b505050505050565b6000815180845260005b818110156105eb576020818501810151868301820152016105cf565b818111156105fd576000602083870101525b50601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169290920160200192915050565b60208152600061064360208301846105c5565b9392505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602160045260246000fd5b600281106106b0577f4e487b7100000000000000000000000000000000000000000000000000000000600052602160045260246000fd5b9052565b602081016106c28284610679565b92915050565b60008219821115610702577f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b500190565b84815273ffffffffffffffffffffffffffffffffffffffff8481166020830152831660408201526080810161073f6060830184610679565b95945050505050565b73ffffffffffffffffffffffffffffffffffffffff8416815263ffffffff8316602082015260606040820152600061073f60608301846105c556fea164736f6c634300080f000a"

func init() {
	if err := json.Unmarshal([]byte(BaseFeeVaultStorageLayoutJSON), BaseFeeVaultStorageLayout); err != nil {
		panic(err)
	}

	layouts["BaseFeeVault"] = BaseFeeVaultStorageLayout
	deployedBytecodes["BaseFeeVault"] = BaseFeeVaultDeployedBin
}

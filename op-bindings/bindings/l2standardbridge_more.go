// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-bindings/solc"
)

const L2StandardBridgeStorageLayoutJSON = "{\"storage\":[{\"astId\":28331,\"contract\":\"contracts/L2/L2StandardBridge.sol:L2StandardBridge\",\"label\":\"spacer_0_0_20\",\"offset\":0,\"slot\":\"0\",\"type\":\"t_address\"},{\"astId\":28334,\"contract\":\"contracts/L2/L2StandardBridge.sol:L2StandardBridge\",\"label\":\"spacer_1_0_20\",\"offset\":0,\"slot\":\"1\",\"type\":\"t_address\"},{\"astId\":28341,\"contract\":\"contracts/L2/L2StandardBridge.sol:L2StandardBridge\",\"label\":\"deposits\",\"offset\":0,\"slot\":\"2\",\"type\":\"t_mapping(t_address,t_mapping(t_address,t_uint256))\"},{\"astId\":28346,\"contract\":\"contracts/L2/L2StandardBridge.sol:L2StandardBridge\",\"label\":\"__gap\",\"offset\":0,\"slot\":\"3\",\"type\":\"t_array(t_uint256)47_storage\"}],\"types\":{\"t_address\":{\"encoding\":\"inplace\",\"label\":\"address\",\"numberOfBytes\":\"20\"},\"t_array(t_uint256)47_storage\":{\"encoding\":\"inplace\",\"label\":\"uint256[47]\",\"numberOfBytes\":\"1504\"},\"t_mapping(t_address,t_mapping(t_address,t_uint256))\":{\"encoding\":\"mapping\",\"label\":\"mapping(address =\u003e mapping(address =\u003e uint256))\",\"numberOfBytes\":\"32\",\"key\":\"t_address\",\"value\":\"t_mapping(t_address,t_uint256)\"},\"t_mapping(t_address,t_uint256)\":{\"encoding\":\"mapping\",\"label\":\"mapping(address =\u003e uint256)\",\"numberOfBytes\":\"32\",\"key\":\"t_address\",\"value\":\"t_uint256\"},\"t_uint256\":{\"encoding\":\"inplace\",\"label\":\"uint256\",\"numberOfBytes\":\"32\"}}}"

var L2StandardBridgeStorageLayout = new(solc.StorageLayout)

var L2StandardBridgeDeployedBin = "0x6080604052600436106100e15760003560e01c8063662a633a1161007f5780638f601f66116100595780638f601f66146102fb578063927ede2d14610341578063a3a7954814610375578063e11013dd1461038857600080fd5b8063662a633a146102945780637f46ddb2146102a757806387087623146102db57600080fd5b806332b7006d116100bb57806332b7006d146101e65780633cb747bf146101f9578063540abf731461025257806354fd4d501461027257600080fd5b80630166a07a146101a057806309fc8843146101c05780631635f5fd146101d357600080fd5b3661019b57333b1561017a576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603760248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20616e20454f4100000000000000000060648201526084015b60405180910390fd5b61019933333462030d406040518060200160405280600081525061039b565b005b600080fd5b3480156101ac57600080fd5b506101996101bb36600461202c565b6105da565b6101996101ce3660046120dd565b610a0e565b6101996101e1366004612130565b610ae5565b6101996101f43660046121a3565b610fe1565b34801561020557600080fd5b507f00000000000000000000000000000000000000000000000000000000000000005b60405173ffffffffffffffffffffffffffffffffffffffff90911681526020015b60405180910390f35b34801561025e57600080fd5b5061019961026d3660046121f7565b611086565b34801561027e57600080fd5b5061028761109f565b60405161024991906122e4565b6101996102a236600461202c565b611142565b3480156102b357600080fd5b506102287f000000000000000000000000000000000000000000000000000000000000000081565b3480156102e757600080fd5b506101996102f63660046122f7565b61122f565b34801561030757600080fd5b5061033361031636600461237a565b600260209081526000928352604080842090915290825290205481565b604051908152602001610249565b34801561034d57600080fd5b506102287f000000000000000000000000000000000000000000000000000000000000000081565b6101996103833660046122f7565b6112ce565b6101996103963660046123b3565b6112dd565b82341461042a576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603e60248201527f5374616e646172644272696467653a206272696467696e6720455448206d757360448201527f7420696e636c7564652073756666696369656e74204554482076616c756500006064820152608401610171565b8373ffffffffffffffffffffffffffffffffffffffff168573ffffffffffffffffffffffffffffffffffffffff167f2849b43074093a05396b6f2a937dee8565b15a48a7b3d4bffb732a5017380af58584604051610489929190612416565b60405180910390a37f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16633dbb202b847f0000000000000000000000000000000000000000000000000000000000000000631635f5fd60e01b8989898860405160240161050e949392919061242f565b604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529181526020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167fffffffff000000000000000000000000000000000000000000000000000000009485161790525160e086901b90921682526105a192918890600401612478565b6000604051808303818588803b1580156105ba57600080fd5b505af11580156105ce573d6000803e3d6000fd5b50505050505050505050565b3373ffffffffffffffffffffffffffffffffffffffff7f0000000000000000000000000000000000000000000000000000000000000000161480156106f857507f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff167f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16636e296e456040518163ffffffff1660e01b8152600401602060405180830381865afa1580156106bc573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906106e091906124bd565b73ffffffffffffffffffffffffffffffffffffffff16145b6107aa576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604160248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20746865206f7468657220627269646760648201527f6500000000000000000000000000000000000000000000000000000000000000608482015260a401610171565b6107b387611326565b15610901576107c28787611388565b610874576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604a60248201527f5374616e646172644272696467653a2077726f6e672072656d6f746520746f6b60448201527f656e20666f72204f7074696d69736d204d696e7461626c65204552433230206c60648201527f6f63616c20746f6b656e00000000000000000000000000000000000000000000608482015260a401610171565b6040517f40c10f1900000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff8581166004830152602482018590528816906340c10f1990604401600060405180830381600087803b1580156108e457600080fd5b505af11580156108f8573d6000803e3d6000fd5b50505050610983565b73ffffffffffffffffffffffffffffffffffffffff8088166000908152600260209081526040808320938a168352929052205461093f908490612509565b73ffffffffffffffffffffffffffffffffffffffff8089166000818152600260209081526040808320948c168352939052919091209190915561098390858561142f565b8473ffffffffffffffffffffffffffffffffffffffff168673ffffffffffffffffffffffffffffffffffffffff168873ffffffffffffffffffffffffffffffffffffffff167fd59c65b35445225835c83f50b6ede06a7be047d22e357073e250d9af537518cd878787876040516109fd9493929190612569565b60405180910390a450505050505050565b333b15610a9d576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603760248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20616e20454f410000000000000000006064820152608401610171565b610ae03333348686868080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525061039b92505050565b505050565b3373ffffffffffffffffffffffffffffffffffffffff7f000000000000000000000000000000000000000000000000000000000000000016148015610c0357507f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff167f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16636e296e456040518163ffffffff1660e01b8152600401602060405180830381865afa158015610bc7573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610beb91906124bd565b73ffffffffffffffffffffffffffffffffffffffff16145b610cb5576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604160248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20746865206f7468657220627269646760648201527f6500000000000000000000000000000000000000000000000000000000000000608482015260a401610171565b823414610d44576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603a60248201527f5374616e646172644272696467653a20616d6f756e742073656e7420646f657360448201527f206e6f74206d6174636820616d6f756e742072657175697265640000000000006064820152608401610171565b3073ffffffffffffffffffffffffffffffffffffffff851603610de9576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602360248201527f5374616e646172644272696467653a2063616e6e6f742073656e6420746f207360448201527f656c6600000000000000000000000000000000000000000000000000000000006064820152608401610171565b7f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff168473ffffffffffffffffffffffffffffffffffffffff1603610ec4576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602860248201527f5374616e646172644272696467653a2063616e6e6f742073656e6420746f206d60448201527f657373656e6765720000000000000000000000000000000000000000000000006064820152608401610171565b8373ffffffffffffffffffffffffffffffffffffffff168573ffffffffffffffffffffffffffffffffffffffff167f31b2166ff604fc5672ea5df08a78081d2bc6d746cadce880747f3643d819e83d858585604051610f259392919061259f565b60405180910390a36000610f4a855a8660405180602001604052806000815250611503565b905080610fd9576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602360248201527f5374616e646172644272696467653a20455448207472616e736665722066616960448201527f6c656400000000000000000000000000000000000000000000000000000000006064820152608401610171565b505050505050565b333b15611070576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603760248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20616e20454f410000000000000000006064820152608401610171565b61107f8533338787878761151d565b5050505050565b61109687873388888888886116b1565b50505050505050565b60606110ca7f0000000000000000000000000000000000000000000000000000000000000000611a6f565b6110f37f0000000000000000000000000000000000000000000000000000000000000000611a6f565b61111c7f0000000000000000000000000000000000000000000000000000000000000000611a6f565b60405160200161112e939291906125c2565b604051602081830303815290604052905090565b73ffffffffffffffffffffffffffffffffffffffff871615801561118f575073ffffffffffffffffffffffffffffffffffffffff861673deaddeaddeaddeaddeaddeaddeaddeaddead0000145b156111a6576111a18585858585610ae5565b6111b5565b6111b5868887878787876105da565b8473ffffffffffffffffffffffffffffffffffffffff168673ffffffffffffffffffffffffffffffffffffffff168873ffffffffffffffffffffffffffffffffffffffff167fb0444523268717a02698be47d0803aa7468c00acbed2f8bd93a0459cde61dd89878787876040516109fd9493929190612569565b333b156112be576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603760248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20616e20454f410000000000000000006064820152608401610171565b610fd986863333888888886116b1565b610fd98633878787878761151d565b6113203385348686868080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525061039b92505050565b50505050565b6000611352827f1d1d8b6300000000000000000000000000000000000000000000000000000000611bac565b806113825750611382827fec4fc8e300000000000000000000000000000000000000000000000000000000611bac565b92915050565b60008273ffffffffffffffffffffffffffffffffffffffff1663c01e1bd66040518163ffffffff1660e01b8152600401602060405180830381865afa1580156113d5573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906113f991906124bd565b73ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff1614905092915050565b60405173ffffffffffffffffffffffffffffffffffffffff8316602482015260448101829052610ae09084907fa9059cbb00000000000000000000000000000000000000000000000000000000906064015b604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529190526020810180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167fffffffff0000000000000000000000000000000000000000000000000000000090931692909217909152611bcf565b600080600080845160208601878a8af19695505050505050565b60008773ffffffffffffffffffffffffffffffffffffffff1663c01e1bd66040518163ffffffff1660e01b8152600401602060405180830381865afa15801561156a573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061158e91906124bd565b90507fffffffffffffffffffffffff215221522152215221522152215221522153000073ffffffffffffffffffffffffffffffffffffffff891601611615576116108787878787878080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525061039b92505050565b611625565b61162588828989898989896116b1565b8673ffffffffffffffffffffffffffffffffffffffff168873ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff167f73d170910aba9e6d50b102db522b1dbcd796216f5128b445aa2135272886497e8989888860405161169f9493929190612569565b60405180910390a45050505050505050565b6116ba88611326565b15611808576116c98888611388565b61177b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604a60248201527f5374616e646172644272696467653a2077726f6e672072656d6f746520746f6b60448201527f656e20666f72204f7074696d69736d204d696e7461626c65204552433230206c60648201527f6f63616c20746f6b656e00000000000000000000000000000000000000000000608482015260a401610171565b6040517f9dc29fac00000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff878116600483015260248201869052891690639dc29fac90604401600060405180830381600087803b1580156117eb57600080fd5b505af11580156117ff573d6000803e3d6000fd5b5050505061189c565b61182a73ffffffffffffffffffffffffffffffffffffffff8916873087611cdb565b73ffffffffffffffffffffffffffffffffffffffff8089166000908152600260209081526040808320938b1683529290522054611868908590612638565b73ffffffffffffffffffffffffffffffffffffffff808a166000908152600260209081526040808320938c16835292905220555b8573ffffffffffffffffffffffffffffffffffffffff168773ffffffffffffffffffffffffffffffffffffffff168973ffffffffffffffffffffffffffffffffffffffff167f7ff126db8024424bbfd9826e8ab82ff59136289ea440b04b39a0df1b03b9cabf888887876040516119169493929190612569565b60405180910390a47f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16633dbb202b7f0000000000000000000000000000000000000000000000000000000000000000630166a07a60e01b8a8c8b8b8b8a8a6040516024016119a09796959493929190612650565b604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529181526020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167fffffffff000000000000000000000000000000000000000000000000000000009485161790525160e085901b9092168252611a3392918890600401612478565b600060405180830381600087803b158015611a4d57600080fd5b505af1158015611a61573d6000803e3d6000fd5b505050505050505050505050565b606081600003611ab257505060408051808201909152600181527f3000000000000000000000000000000000000000000000000000000000000000602082015290565b8160005b8115611adc5780611ac6816126ad565b9150611ad59050600a83612714565b9150611ab6565b60008167ffffffffffffffff811115611af757611af7612728565b6040519080825280601f01601f191660200182016040528015611b21576020820181803683370190505b5090505b8415611ba457611b36600183612509565b9150611b43600a86612757565b611b4e906030612638565b60f81b818381518110611b6357611b6361276b565b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a905350611b9d600a86612714565b9450611b25565b949350505050565b6000611bb783611d39565b8015611bc85750611bc88383611d9d565b9392505050565b6000611c31826040518060400160405280602081526020017f5361666545524332303a206c6f772d6c6576656c2063616c6c206661696c65648152508573ffffffffffffffffffffffffffffffffffffffff16611e6c9092919063ffffffff16565b805190915015610ae05780806020019051810190611c4f919061279a565b610ae0576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602a60248201527f5361666545524332303a204552433230206f7065726174696f6e20646964206e60448201527f6f742073756363656564000000000000000000000000000000000000000000006064820152608401610171565b60405173ffffffffffffffffffffffffffffffffffffffff808516602483015283166044820152606481018290526113209085907f23b872dd0000000000000000000000000000000000000000000000000000000090608401611481565b6000611d65827f01ffc9a700000000000000000000000000000000000000000000000000000000611d9d565b80156113825750611d96827fffffffff00000000000000000000000000000000000000000000000000000000611d9d565b1592915050565b604080517fffffffff000000000000000000000000000000000000000000000000000000008316602480830191909152825180830390910181526044909101909152602080820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167f01ffc9a700000000000000000000000000000000000000000000000000000000178152825160009392849283928392918391908a617530fa92503d91506000519050828015611e55575060208210155b8015611e615750600081115b979650505050505050565b6060611ba484846000858573ffffffffffffffffffffffffffffffffffffffff85163b611ef5576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601d60248201527f416464726573733a2063616c6c20746f206e6f6e2d636f6e74726163740000006044820152606401610171565b6000808673ffffffffffffffffffffffffffffffffffffffff168587604051611f1e91906127bc565b60006040518083038185875af1925050503d8060008114611f5b576040519150601f19603f3d011682016040523d82523d6000602084013e611f60565b606091505b5091509150611e6182828660608315611f7a575081611bc8565b825115611f8a5782518084602001fd5b816040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161017191906122e4565b73ffffffffffffffffffffffffffffffffffffffff81168114611fe057600080fd5b50565b60008083601f840112611ff557600080fd5b50813567ffffffffffffffff81111561200d57600080fd5b60208301915083602082850101111561202557600080fd5b9250929050565b600080600080600080600060c0888a03121561204757600080fd5b873561205281611fbe565b9650602088013561206281611fbe565b9550604088013561207281611fbe565b9450606088013561208281611fbe565b93506080880135925060a088013567ffffffffffffffff8111156120a557600080fd5b6120b18a828b01611fe3565b989b979a50959850939692959293505050565b803563ffffffff811681146120d857600080fd5b919050565b6000806000604084860312156120f257600080fd5b6120fb846120c4565b9250602084013567ffffffffffffffff81111561211757600080fd5b61212386828701611fe3565b9497909650939450505050565b60008060008060006080868803121561214857600080fd5b853561215381611fbe565b9450602086013561216381611fbe565b935060408601359250606086013567ffffffffffffffff81111561218657600080fd5b61219288828901611fe3565b969995985093965092949392505050565b6000806000806000608086880312156121bb57600080fd5b85356121c681611fbe565b9450602086013593506121db604087016120c4565b9250606086013567ffffffffffffffff81111561218657600080fd5b600080600080600080600060c0888a03121561221257600080fd5b873561221d81611fbe565b9650602088013561222d81611fbe565b9550604088013561223d81611fbe565b945060608801359350612252608089016120c4565b925060a088013567ffffffffffffffff8111156120a557600080fd5b60005b83811015612289578181015183820152602001612271565b838111156113205750506000910152565b600081518084526122b281602086016020860161226e565b601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169290920160200192915050565b602081526000611bc8602083018461229a565b60008060008060008060a0878903121561231057600080fd5b863561231b81611fbe565b9550602087013561232b81611fbe565b945060408701359350612340606088016120c4565b9250608087013567ffffffffffffffff81111561235c57600080fd5b61236889828a01611fe3565b979a9699509497509295939492505050565b6000806040838503121561238d57600080fd5b823561239881611fbe565b915060208301356123a881611fbe565b809150509250929050565b600080600080606085870312156123c957600080fd5b84356123d481611fbe565b93506123e2602086016120c4565b9250604085013567ffffffffffffffff8111156123fe57600080fd5b61240a87828801611fe3565b95989497509550505050565b828152604060208201526000611ba4604083018461229a565b600073ffffffffffffffffffffffffffffffffffffffff80871683528086166020840152508360408301526080606083015261246e608083018461229a565b9695505050505050565b73ffffffffffffffffffffffffffffffffffffffff841681526060602082015260006124a7606083018561229a565b905063ffffffff83166040830152949350505050565b6000602082840312156124cf57600080fd5b8151611bc881611fbe565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b60008282101561251b5761251b6124da565b500390565b8183528181602085013750600060208284010152600060207fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0601f840116840101905092915050565b73ffffffffffffffffffffffffffffffffffffffff8516815283602082015260606040820152600061246e606083018486612520565b8381526040602082015260006125b9604083018486612520565b95945050505050565b600084516125d481846020890161226e565b80830190507f2e000000000000000000000000000000000000000000000000000000000000008082528551612610816001850160208a0161226e565b6001920191820152835161262b81600284016020880161226e565b0160020195945050505050565b6000821982111561264b5761264b6124da565b500190565b600073ffffffffffffffffffffffffffffffffffffffff808a1683528089166020840152808816604084015280871660608401525084608083015260c060a08301526126a060c083018486612520565b9998505050505050505050565b60007fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff82036126de576126de6124da565b5060010190565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fd5b600082612723576127236126e5565b500490565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b600082612766576127666126e5565b500690565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b6000602082840312156127ac57600080fd5b81518015158114611bc857600080fd5b600082516127ce81846020870161226e565b919091019291505056fea164736f6c634300080f000a"

func init() {
	if err := json.Unmarshal([]byte(L2StandardBridgeStorageLayoutJSON), L2StandardBridgeStorageLayout); err != nil {
		panic(err)
	}

	layouts["L2StandardBridge"] = L2StandardBridgeStorageLayout
	deployedBytecodes["L2StandardBridge"] = L2StandardBridgeDeployedBin
}

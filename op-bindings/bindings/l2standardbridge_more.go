// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-bindings/solc"
)

const L2StandardBridgeStorageLayoutJSON = "{\"storage\":[{\"astId\":1000,\"contract\":\"contracts/L2/L2StandardBridge.sol:L2StandardBridge\",\"label\":\"spacer_0_0_20\",\"offset\":0,\"slot\":\"0\",\"type\":\"t_address\"},{\"astId\":1001,\"contract\":\"contracts/L2/L2StandardBridge.sol:L2StandardBridge\",\"label\":\"spacer_1_0_20\",\"offset\":0,\"slot\":\"1\",\"type\":\"t_address\"},{\"astId\":1002,\"contract\":\"contracts/L2/L2StandardBridge.sol:L2StandardBridge\",\"label\":\"deposits\",\"offset\":0,\"slot\":\"2\",\"type\":\"t_mapping(t_address,t_mapping(t_address,t_uint256))\"},{\"astId\":1003,\"contract\":\"contracts/L2/L2StandardBridge.sol:L2StandardBridge\",\"label\":\"__gap\",\"offset\":0,\"slot\":\"3\",\"type\":\"t_array(t_uint256)47_storage\"}],\"types\":{\"t_address\":{\"encoding\":\"inplace\",\"label\":\"address\",\"numberOfBytes\":\"20\"},\"t_array(t_uint256)47_storage\":{\"encoding\":\"inplace\",\"label\":\"uint256[47]\",\"numberOfBytes\":\"1504\",\"base\":\"t_uint256\"},\"t_mapping(t_address,t_mapping(t_address,t_uint256))\":{\"encoding\":\"mapping\",\"label\":\"mapping(address =\u003e mapping(address =\u003e uint256))\",\"numberOfBytes\":\"32\",\"key\":\"t_address\",\"value\":\"t_mapping(t_address,t_uint256)\"},\"t_mapping(t_address,t_uint256)\":{\"encoding\":\"mapping\",\"label\":\"mapping(address =\u003e uint256)\",\"numberOfBytes\":\"32\",\"key\":\"t_address\",\"value\":\"t_uint256\"},\"t_uint256\":{\"encoding\":\"inplace\",\"label\":\"uint256\",\"numberOfBytes\":\"32\"}}}"

var L2StandardBridgeStorageLayout = new(solc.StorageLayout)

var L2StandardBridgeDeployedBin = "0x6080604052600436106100ec5760003560e01c806354fd4d501161008a5780638f601f66116100595780638f601f661461034e578063927ede2d14610394578063a3a79548146103c8578063e11013dd146103db57600080fd5b806354fd4d50146102c5578063662a633a146102e75780637f46ddb2146102fa578063870876231461032e57600080fd5b806332b7006d116100c657806332b7006d1461020657806336c717c1146102195780633cb747bf14610272578063540abf73146102a557600080fd5b80630166a07a146101c057806309fc8843146101e05780631635f5fd146101f357600080fd5b366101bb57333b15610185576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603760248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20616e20454f4100000000000000000060648201526084015b60405180910390fd5b6101b973deaddeaddeaddeaddeaddeaddeaddeaddead000033333462030d40604051806020016040528060008152506103ee565b005b600080fd5b3480156101cc57600080fd5b506101b96101db366004612372565b6104c9565b6101b96101ee366004612423565b6108b6565b6101b9610201366004612476565b61098d565b6101b96102143660046124e9565b610e5a565b34801561022557600080fd5b507f00000000000000000000000000000000000000000000000000000000000000005b60405173ffffffffffffffffffffffffffffffffffffffff90911681526020015b60405180910390f35b34801561027e57600080fd5b507f0000000000000000000000000000000000000000000000000000000000000000610248565b3480156102b157600080fd5b506101b96102c036600461253d565b610f34565b3480156102d157600080fd5b506102da610f79565b604051610269919061262a565b6101b96102f5366004612372565b61101c565b34801561030657600080fd5b506102487f000000000000000000000000000000000000000000000000000000000000000081565b34801561033a57600080fd5b506101b961034936600461263d565b61108f565b34801561035a57600080fd5b506103866103693660046126c0565b600260209081526000928352604080842090915290825290205481565b604051908152602001610269565b3480156103a057600080fd5b506102487f000000000000000000000000000000000000000000000000000000000000000081565b6101b96103d636600461263d565b611163565b6101b96103e93660046126f9565b6111a7565b7fffffffffffffffffffffffff215221522152215221522152215221522153000073ffffffffffffffffffffffffffffffffffffffff87160161043d5761043885858585856111f0565b6104c1565b60008673ffffffffffffffffffffffffffffffffffffffff1663c01e1bd66040518163ffffffff1660e01b8152600401602060405180830381865afa15801561048a573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906104ae919061275c565b90506104bf878288888888886113d4565b505b505050505050565b3373ffffffffffffffffffffffffffffffffffffffff7f0000000000000000000000000000000000000000000000000000000000000000161480156105e757507f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff167f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16636e296e456040518163ffffffff1660e01b8152600401602060405180830381865afa1580156105ab573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906105cf919061275c565b73ffffffffffffffffffffffffffffffffffffffff16145b610699576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604160248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20746865206f7468657220627269646760648201527f6500000000000000000000000000000000000000000000000000000000000000608482015260a40161017c565b6106a28761171b565b156107f0576106b1878761177d565b610763576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604a60248201527f5374616e646172644272696467653a2077726f6e672072656d6f746520746f6b60448201527f656e20666f72204f7074696d69736d204d696e7461626c65204552433230206c60648201527f6f63616c20746f6b656e00000000000000000000000000000000000000000000608482015260a40161017c565b6040517f40c10f1900000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff8581166004830152602482018590528816906340c10f1990604401600060405180830381600087803b1580156107d357600080fd5b505af11580156107e7573d6000803e3d6000fd5b50505050610872565b73ffffffffffffffffffffffffffffffffffffffff8088166000908152600260209081526040808320938a168352929052205461082e9084906127a8565b73ffffffffffffffffffffffffffffffffffffffff8089166000818152600260209081526040808320948c168352939052919091209190915561087290858561189d565b6104bf878787878787878080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525061197192505050565b333b15610945576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603760248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20616e20454f41000000000000000000606482015260840161017c565b6109883333348686868080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152506111f092505050565b505050565b3373ffffffffffffffffffffffffffffffffffffffff7f000000000000000000000000000000000000000000000000000000000000000016148015610aab57507f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff167f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16636e296e456040518163ffffffff1660e01b8152600401602060405180830381865afa158015610a6f573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610a93919061275c565b73ffffffffffffffffffffffffffffffffffffffff16145b610b5d576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604160248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20746865206f7468657220627269646760648201527f6500000000000000000000000000000000000000000000000000000000000000608482015260a40161017c565b823414610bec576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603a60248201527f5374616e646172644272696467653a20616d6f756e742073656e7420646f657360448201527f206e6f74206d6174636820616d6f756e74207265717569726564000000000000606482015260840161017c565b3073ffffffffffffffffffffffffffffffffffffffff851603610c91576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602360248201527f5374616e646172644272696467653a2063616e6e6f742073656e6420746f207360448201527f656c660000000000000000000000000000000000000000000000000000000000606482015260840161017c565b7f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff168473ffffffffffffffffffffffffffffffffffffffff1603610d6c576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602860248201527f5374616e646172644272696467653a2063616e6e6f742073656e6420746f206d60448201527f657373656e676572000000000000000000000000000000000000000000000000606482015260840161017c565b610dae85858585858080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152506119ff92505050565b6000610dcb855a8660405180602001604052806000815250611aa0565b9050806104c1576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602360248201527f5374616e646172644272696467653a20455448207472616e736665722066616960448201527f6c65640000000000000000000000000000000000000000000000000000000000606482015260840161017c565b333b15610ee9576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603760248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20616e20454f41000000000000000000606482015260840161017c565b610f2d853333878787878080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152506103ee92505050565b5050505050565b6104bf87873388888888888080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152506113d492505050565b6060610fa47f0000000000000000000000000000000000000000000000000000000000000000611aba565b610fcd7f0000000000000000000000000000000000000000000000000000000000000000611aba565b610ff67f0000000000000000000000000000000000000000000000000000000000000000611aba565b604051602001611008939291906127bf565b604051602081830303815290604052905090565b73ffffffffffffffffffffffffffffffffffffffff8716158015611069575073ffffffffffffffffffffffffffffffffffffffff861673deaddeaddeaddeaddeaddeaddeaddeaddead0000145b156110805761107b858585858561098d565b6104bf565b6104bf868887878787876104c9565b333b1561111e576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603760248201527f5374616e646172644272696467653a2066756e6374696f6e2063616e206f6e6c60448201527f792062652063616c6c65642066726f6d20616e20454f41000000000000000000606482015260840161017c565b6104c186863333888888888080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152506113d492505050565b6104c1863387878787878080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152506103ee92505050565b6111ea3385348686868080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152506111f092505050565b50505050565b82341461127f576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603e60248201527f5374616e646172644272696467653a206272696467696e6720455448206d757360448201527f7420696e636c7564652073756666696369656e74204554482076616c75650000606482015260840161017c565b61128b85858584611bf7565b7f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16633dbb202b847f0000000000000000000000000000000000000000000000000000000000000000631635f5fd60e01b898989886040516024016113089493929190612835565b604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529181526020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167fffffffff000000000000000000000000000000000000000000000000000000009485161790525160e086901b909216825261139b9291889060040161287e565b6000604051808303818588803b1580156113b457600080fd5b505af11580156113c8573d6000803e3d6000fd5b50505050505050505050565b6113dd8761171b565b1561152b576113ec878761177d565b61149e576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604a60248201527f5374616e646172644272696467653a2077726f6e672072656d6f746520746f6b60448201527f656e20666f72204f7074696d69736d204d696e7461626c65204552433230206c60648201527f6f63616c20746f6b656e00000000000000000000000000000000000000000000608482015260a40161017c565b6040517f9dc29fac00000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff868116600483015260248201859052881690639dc29fac90604401600060405180830381600087803b15801561150e57600080fd5b505af1158015611522573d6000803e3d6000fd5b505050506115bf565b61154d73ffffffffffffffffffffffffffffffffffffffff8816863086611c98565b73ffffffffffffffffffffffffffffffffffffffff8088166000908152600260209081526040808320938a168352929052205461158b9084906128c3565b73ffffffffffffffffffffffffffffffffffffffff8089166000908152600260209081526040808320938b16835292905220555b6115cd878787878786611cf6565b7f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16633dbb202b7f0000000000000000000000000000000000000000000000000000000000000000630166a07a60e01b898b8a8a8a8960405160240161164d969594939291906128db565b604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529181526020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167fffffffff000000000000000000000000000000000000000000000000000000009485161790525160e085901b90921682526116e09291879060040161287e565b600060405180830381600087803b1580156116fa57600080fd5b505af115801561170e573d6000803e3d6000fd5b5050505050505050505050565b6000611747827f1d1d8b6300000000000000000000000000000000000000000000000000000000611d84565b806117775750611777827fec4fc8e300000000000000000000000000000000000000000000000000000000611d84565b92915050565b60006117a9837f1d1d8b6300000000000000000000000000000000000000000000000000000000611d84565b15611852578273ffffffffffffffffffffffffffffffffffffffff1663c01e1bd66040518163ffffffff1660e01b8152600401602060405180830381865afa1580156117f9573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061181d919061275c565b73ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16149050611777565b8273ffffffffffffffffffffffffffffffffffffffff1663d6c0b2c46040518163ffffffff1660e01b8152600401602060405180830381865afa1580156117f9573d6000803e3d6000fd5b60405173ffffffffffffffffffffffffffffffffffffffff83166024820152604481018290526109889084907fa9059cbb00000000000000000000000000000000000000000000000000000000906064015b604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529190526020810180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167fffffffff0000000000000000000000000000000000000000000000000000000090931692909217909152611da7565b8373ffffffffffffffffffffffffffffffffffffffff168673ffffffffffffffffffffffffffffffffffffffff168673ffffffffffffffffffffffffffffffffffffffff167fb0444523268717a02698be47d0803aa7468c00acbed2f8bd93a0459cde61dd898686866040516119e993929190612936565b60405180910390a46104c1868686868686611eb3565b8373ffffffffffffffffffffffffffffffffffffffff1673deaddeaddeaddeaddeaddeaddeaddeaddead000073ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fb0444523268717a02698be47d0803aa7468c00acbed2f8bd93a0459cde61dd89868686604051611a8c93929190612936565b60405180910390a46111ea84848484611f3b565b600080600080845160208601878a8af19695505050505050565b606081600003611afd57505060408051808201909152600181527f3000000000000000000000000000000000000000000000000000000000000000602082015290565b8160005b8115611b275780611b1181612974565b9150611b209050600a836129db565b9150611b01565b60008167ffffffffffffffff811115611b4257611b426129ef565b6040519080825280601f01601f191660200182016040528015611b6c576020820181803683370190505b5090505b8415611bef57611b816001836127a8565b9150611b8e600a86612a1e565b611b999060306128c3565b60f81b818381518110611bae57611bae612a32565b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a905350611be8600a866129db565b9450611b70565b949350505050565b8373ffffffffffffffffffffffffffffffffffffffff1673deaddeaddeaddeaddeaddeaddeaddeaddead000073ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167f73d170910aba9e6d50b102db522b1dbcd796216f5128b445aa2135272886497e868686604051611c8493929190612936565b60405180910390a46111ea84848484611fa8565b60405173ffffffffffffffffffffffffffffffffffffffff808516602483015283166044820152606481018290526111ea9085907f23b872dd00000000000000000000000000000000000000000000000000000000906084016118ef565b8373ffffffffffffffffffffffffffffffffffffffff168673ffffffffffffffffffffffffffffffffffffffff168673ffffffffffffffffffffffffffffffffffffffff167f73d170910aba9e6d50b102db522b1dbcd796216f5128b445aa2135272886497e868686604051611d6e93929190612936565b60405180910390a46104c1868686868686612007565b6000611d8f8361207f565b8015611da05750611da083836120e3565b9392505050565b6000611e09826040518060400160405280602081526020017f5361666545524332303a206c6f772d6c6576656c2063616c6c206661696c65648152508573ffffffffffffffffffffffffffffffffffffffff166121b29092919063ffffffff16565b8051909150156109885780806020019051810190611e279190612a61565b610988576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602a60248201527f5361666545524332303a204552433230206f7065726174696f6e20646964206e60448201527f6f74207375636365656400000000000000000000000000000000000000000000606482015260840161017c565b8373ffffffffffffffffffffffffffffffffffffffff168573ffffffffffffffffffffffffffffffffffffffff168773ffffffffffffffffffffffffffffffffffffffff167fd59c65b35445225835c83f50b6ede06a7be047d22e357073e250d9af537518cd868686604051611f2b93929190612936565b60405180910390a4505050505050565b8273ffffffffffffffffffffffffffffffffffffffff168473ffffffffffffffffffffffffffffffffffffffff167f31b2166ff604fc5672ea5df08a78081d2bc6d746cadce880747f3643d819e83d8484604051611f9a929190612a83565b60405180910390a350505050565b8273ffffffffffffffffffffffffffffffffffffffff168473ffffffffffffffffffffffffffffffffffffffff167f2849b43074093a05396b6f2a937dee8565b15a48a7b3d4bffb732a5017380af58484604051611f9a929190612a83565b8373ffffffffffffffffffffffffffffffffffffffff168573ffffffffffffffffffffffffffffffffffffffff168773ffffffffffffffffffffffffffffffffffffffff167f7ff126db8024424bbfd9826e8ab82ff59136289ea440b04b39a0df1b03b9cabf868686604051611f2b93929190612936565b60006120ab827f01ffc9a7000000000000000000000000000000000000000000000000000000006120e3565b801561177757506120dc827fffffffff000000000000000000000000000000000000000000000000000000006120e3565b1592915050565b604080517fffffffff000000000000000000000000000000000000000000000000000000008316602480830191909152825180830390910181526044909101909152602080820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167f01ffc9a700000000000000000000000000000000000000000000000000000000178152825160009392849283928392918391908a617530fa92503d9150600051905082801561219b575060208210155b80156121a75750600081115b979650505050505050565b6060611bef84846000858573ffffffffffffffffffffffffffffffffffffffff85163b61223b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601d60248201527f416464726573733a2063616c6c20746f206e6f6e2d636f6e7472616374000000604482015260640161017c565b6000808673ffffffffffffffffffffffffffffffffffffffff1685876040516122649190612a9c565b60006040518083038185875af1925050503d80600081146122a1576040519150601f19603f3d011682016040523d82523d6000602084013e6122a6565b606091505b50915091506121a7828286606083156122c0575081611da0565b8251156122d05782518084602001fd5b816040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161017c919061262a565b73ffffffffffffffffffffffffffffffffffffffff8116811461232657600080fd5b50565b60008083601f84011261233b57600080fd5b50813567ffffffffffffffff81111561235357600080fd5b60208301915083602082850101111561236b57600080fd5b9250929050565b600080600080600080600060c0888a03121561238d57600080fd5b873561239881612304565b965060208801356123a881612304565b955060408801356123b881612304565b945060608801356123c881612304565b93506080880135925060a088013567ffffffffffffffff8111156123eb57600080fd5b6123f78a828b01612329565b989b979a50959850939692959293505050565b803563ffffffff8116811461241e57600080fd5b919050565b60008060006040848603121561243857600080fd5b6124418461240a565b9250602084013567ffffffffffffffff81111561245d57600080fd5b61246986828701612329565b9497909650939450505050565b60008060008060006080868803121561248e57600080fd5b853561249981612304565b945060208601356124a981612304565b935060408601359250606086013567ffffffffffffffff8111156124cc57600080fd5b6124d888828901612329565b969995985093965092949392505050565b60008060008060006080868803121561250157600080fd5b853561250c81612304565b9450602086013593506125216040870161240a565b9250606086013567ffffffffffffffff8111156124cc57600080fd5b600080600080600080600060c0888a03121561255857600080fd5b873561256381612304565b9650602088013561257381612304565b9550604088013561258381612304565b9450606088013593506125986080890161240a565b925060a088013567ffffffffffffffff8111156123eb57600080fd5b60005b838110156125cf5781810151838201526020016125b7565b838111156111ea5750506000910152565b600081518084526125f88160208601602086016125b4565b601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169290920160200192915050565b602081526000611da060208301846125e0565b60008060008060008060a0878903121561265657600080fd5b863561266181612304565b9550602087013561267181612304565b9450604087013593506126866060880161240a565b9250608087013567ffffffffffffffff8111156126a257600080fd5b6126ae89828a01612329565b979a9699509497509295939492505050565b600080604083850312156126d357600080fd5b82356126de81612304565b915060208301356126ee81612304565b809150509250929050565b6000806000806060858703121561270f57600080fd5b843561271a81612304565b93506127286020860161240a565b9250604085013567ffffffffffffffff81111561274457600080fd5b61275087828801612329565b95989497509550505050565b60006020828403121561276e57600080fd5b8151611da081612304565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000828210156127ba576127ba612779565b500390565b600084516127d18184602089016125b4565b80830190507f2e00000000000000000000000000000000000000000000000000000000000000808252855161280d816001850160208a016125b4565b600192019182015283516128288160028401602088016125b4565b0160020195945050505050565b600073ffffffffffffffffffffffffffffffffffffffff80871683528086166020840152508360408301526080606083015261287460808301846125e0565b9695505050505050565b73ffffffffffffffffffffffffffffffffffffffff841681526060602082015260006128ad60608301856125e0565b905063ffffffff83166040830152949350505050565b600082198211156128d6576128d6612779565b500190565b600073ffffffffffffffffffffffffffffffffffffffff80891683528088166020840152808716604084015280861660608401525083608083015260c060a083015261292a60c08301846125e0565b98975050505050505050565b73ffffffffffffffffffffffffffffffffffffffff8416815282602082015260606040820152600061296b60608301846125e0565b95945050505050565b60007fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff82036129a5576129a5612779565b5060010190565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fd5b6000826129ea576129ea6129ac565b500490565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b600082612a2d57612a2d6129ac565b500690565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b600060208284031215612a7357600080fd5b81518015158114611da057600080fd5b828152604060208201526000611bef60408301846125e0565b60008251612aae8184602087016125b4565b919091019291505056fea164736f6c634300080f000a"

func init() {
	if err := json.Unmarshal([]byte(L2StandardBridgeStorageLayoutJSON), L2StandardBridgeStorageLayout); err != nil {
		panic(err)
	}

	layouts["L2StandardBridge"] = L2StandardBridgeStorageLayout
	deployedBytecodes["L2StandardBridge"] = L2StandardBridgeDeployedBin
}

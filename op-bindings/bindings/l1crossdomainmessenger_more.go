// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-bindings/solc"
)

const L1CrossDomainMessengerStorageLayoutJSON = "{\"storage\":[{\"astId\":24887,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"spacer_0_0_20\",\"offset\":0,\"slot\":\"0\",\"type\":\"t_address\"},{\"astId\":27873,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"_initialized\",\"offset\":20,\"slot\":\"0\",\"type\":\"t_uint8\"},{\"astId\":27876,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"_initializing\",\"offset\":21,\"slot\":\"0\",\"type\":\"t_bool\"},{\"astId\":28487,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"__gap\",\"offset\":0,\"slot\":\"1\",\"type\":\"t_array(t_uint256)50_storage\"},{\"astId\":27745,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"_owner\",\"offset\":0,\"slot\":\"51\",\"type\":\"t_address\"},{\"astId\":27865,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"__gap\",\"offset\":0,\"slot\":\"52\",\"type\":\"t_array(t_uint256)49_storage\"},{\"astId\":28038,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"_paused\",\"offset\":0,\"slot\":\"101\",\"type\":\"t_bool\"},{\"astId\":28143,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"__gap\",\"offset\":0,\"slot\":\"102\",\"type\":\"t_array(t_uint256)49_storage\"},{\"astId\":28158,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"_status\",\"offset\":0,\"slot\":\"151\",\"type\":\"t_uint256\"},{\"astId\":28202,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"__gap\",\"offset\":0,\"slot\":\"152\",\"type\":\"t_array(t_uint256)49_storage\"},{\"astId\":24939,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"spacer_201_0_32\",\"offset\":0,\"slot\":\"201\",\"type\":\"t_mapping(t_bytes32,t_bool)\"},{\"astId\":24944,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"spacer_202_0_32\",\"offset\":0,\"slot\":\"202\",\"type\":\"t_mapping(t_bytes32,t_bool)\"},{\"astId\":24949,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"successfulMessages\",\"offset\":0,\"slot\":\"203\",\"type\":\"t_mapping(t_bytes32,t_bool)\"},{\"astId\":24952,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"xDomainMsgSender\",\"offset\":0,\"slot\":\"204\",\"type\":\"t_address\"},{\"astId\":24955,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"msgNonce\",\"offset\":0,\"slot\":\"205\",\"type\":\"t_uint240\"},{\"astId\":24960,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"receivedMessages\",\"offset\":0,\"slot\":\"206\",\"type\":\"t_mapping(t_bytes32,t_bool)\"},{\"astId\":24965,\"contract\":\"contracts/L1/L1CrossDomainMessenger.sol:L1CrossDomainMessenger\",\"label\":\"__gap\",\"offset\":0,\"slot\":\"207\",\"type\":\"t_array(t_uint256)42_storage\"}],\"types\":{\"t_address\":{\"encoding\":\"inplace\",\"label\":\"address\",\"numberOfBytes\":\"20\"},\"t_array(t_uint256)42_storage\":{\"encoding\":\"inplace\",\"label\":\"uint256[42]\",\"numberOfBytes\":\"1344\"},\"t_array(t_uint256)49_storage\":{\"encoding\":\"inplace\",\"label\":\"uint256[49]\",\"numberOfBytes\":\"1568\"},\"t_array(t_uint256)50_storage\":{\"encoding\":\"inplace\",\"label\":\"uint256[50]\",\"numberOfBytes\":\"1600\"},\"t_bool\":{\"encoding\":\"inplace\",\"label\":\"bool\",\"numberOfBytes\":\"1\"},\"t_bytes32\":{\"encoding\":\"inplace\",\"label\":\"bytes32\",\"numberOfBytes\":\"32\"},\"t_mapping(t_bytes32,t_bool)\":{\"encoding\":\"mapping\",\"label\":\"mapping(bytes32 =\u003e bool)\",\"numberOfBytes\":\"32\",\"key\":\"t_bytes32\",\"value\":\"t_bool\"},\"t_uint240\":{\"encoding\":\"inplace\",\"label\":\"uint240\",\"numberOfBytes\":\"30\"},\"t_uint256\":{\"encoding\":\"inplace\",\"label\":\"uint256\",\"numberOfBytes\":\"32\"},\"t_uint8\":{\"encoding\":\"inplace\",\"label\":\"uint8\",\"numberOfBytes\":\"1\"}}}"

var L1CrossDomainMessengerStorageLayout = new(solc.StorageLayout)

var L1CrossDomainMessengerDeployedBin = "0x6080604052600436106101755760003560e01c80637dea7cc3116100cb578063b28ade251161007f578063ecc7042811610059578063ecc70428146103f7578063f2fde38b1461045c578063f69f81511461047c57600080fd5b8063b28ade2514610390578063d764ad0b146103b0578063db505d80146103c357600080fd5b80638456cb59116100b05780638456cb59146103205780638da5cb5b14610335578063b1b1b2091461036057600080fd5b80637dea7cc3146102f45780638129fc1c1461030b57600080fd5b80633f827a5a1161012d5780636425666b116101075780636425666b146102715780636e296e45146102ca578063715018a6146102df57600080fd5b80633f827a5a1461020357806354fd4d501461022b5780635c975abb1461024d57600080fd5b80632828d7e81161015e5780632828d7e8146101c35780633dbb202b146101d95780633f4ba83a146101ee57600080fd5b8063028f85f71461017a5780630c568498146101ad575b600080fd5b34801561018657600080fd5b5061018f601081565b60405167ffffffffffffffff90911681526020015b60405180910390f35b3480156101b957600080fd5b5061018f6103e881565b3480156101cf57600080fd5b5061018f6103f881565b6101ec6101e7366004611de8565b6104ac565b005b3480156101fa57600080fd5b506101ec610710565b34801561020f57600080fd5b50610218600181565b60405161ffff90911681526020016101a4565b34801561023757600080fd5b50610240610722565b6040516101a49190611ec9565b34801561025957600080fd5b5060655460ff165b60405190151581526020016101a4565b34801561027d57600080fd5b506102a57f000000000000000000000000000000000000000000000000000000000000000081565b60405173ffffffffffffffffffffffffffffffffffffffff90911681526020016101a4565b3480156102d657600080fd5b506102a56107c5565b3480156102eb57600080fd5b506101ec6108b1565b34801561030057600080fd5b5061018f62030d4081565b34801561031757600080fd5b506101ec6108c3565b34801561032c57600080fd5b506101ec610ac0565b34801561034157600080fd5b5060335473ffffffffffffffffffffffffffffffffffffffff166102a5565b34801561036c57600080fd5b5061026161037b366004611ee3565b60cb6020526000908152604090205460ff1681565b34801561039c57600080fd5b5061018f6103ab366004611efc565b610ad0565b6101ec6103be366004611f50565b610b1c565b3480156103cf57600080fd5b506102a57f000000000000000000000000000000000000000000000000000000000000000081565b34801561040357600080fd5b5061044e60cd547dffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff167e010000000000000000000000000000000000000000000000000000000000001790565b6040519081526020016101a4565b34801561046857600080fd5b506101ec610477366004611fd6565b6111ad565b34801561048857600080fd5b50610261610497366004611ee3565b60ce6020526000908152604090205460ff1681565b6105e57f00000000000000000000000000000000000000000000000000000000000000006104db858585610ad0565b347fd764ad0b0000000000000000000000000000000000000000000000000000000061054760cd547dffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff167e010000000000000000000000000000000000000000000000000000000000001790565b338a34898c8c604051602401610563979695949392919061203c565b604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529190526020810180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167fffffffff000000000000000000000000000000000000000000000000000000009093169290921790915261127d565b8373ffffffffffffffffffffffffffffffffffffffff167fcb0f7ffd78f9aee47a248fae8db181db6eee833039123e026dcbff529522e52a33858561066a60cd547dffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff167e010000000000000000000000000000000000000000000000000000000000001790565b8660405161067c95949392919061209b565b60405180910390a260405134815233907f8ebb2ec2465bdb2a06a66fc37a0963af8a2a6a1479d81d56fdb8cbb98096d5469060200160405180910390a2505060cd80547dffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff808216600101167fffff0000000000000000000000000000000000000000000000000000000000009091161790555050565b610718611332565b6107206113b3565b565b606061074d7f0000000000000000000000000000000000000000000000000000000000000000611430565b6107767f0000000000000000000000000000000000000000000000000000000000000000611430565b61079f7f0000000000000000000000000000000000000000000000000000000000000000611430565b6040516020016107b1939291906120e9565b604051602081830303815290604052905090565b60cc5460009073ffffffffffffffffffffffffffffffffffffffff167fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff215301610894576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603560248201527f43726f7373446f6d61696e4d657373656e6765723a2078446f6d61696e4d657360448201527f7361676553656e646572206973206e6f7420736574000000000000000000000060648201526084015b60405180910390fd5b5060cc5473ffffffffffffffffffffffffffffffffffffffff1690565b6108b9611332565b6107206000611565565b6000547501000000000000000000000000000000000000000000900460ff161580801561090e575060005460017401000000000000000000000000000000000000000090910460ff16105b806109405750303b158015610940575060005474010000000000000000000000000000000000000000900460ff166001145b6109cc576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602e60248201527f496e697469616c697a61626c653a20636f6e747261637420697320616c72656160448201527f647920696e697469616c697a6564000000000000000000000000000000000000606482015260840161088b565b600080547fffffffffffffffffffffff00ffffffffffffffffffffffffffffffffffffffff16740100000000000000000000000000000000000000001790558015610a5257600080547fffffffffffffffffffff00ffffffffffffffffffffffffffffffffffffffffff1675010000000000000000000000000000000000000000001790555b610a5a6115dc565b8015610abd57600080547fffffffffffffffffffff00ffffffffffffffffffffffffffffffffffffffffff169055604051600181527f7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb38474024989060200160405180910390a15b50565b610ac8611332565b6107206116d3565b600062030d40610ae160108561218e565b6103e8610af66103f863ffffffff871661218e565b610b0091906121ed565b610b0a9190612214565b610b149190612214565b949350505050565b600260975403610b88576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601f60248201527f5265656e7472616e637947756172643a207265656e7472616e742063616c6c00604482015260640161088b565b6002609755610b9561172e565b60f087901c60018114610c50576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152605560248201527f43726f7373446f6d61696e4d657373656e6765723a206f6e6c7920766572736960448201527f6f6e2031206d657373616765732061726520737570706f72746564206166746560648201527f722074686520426564726f636b20757067726164650000000000000000000000608482015260a40161088b565b6000610c96898989898989898080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525061179b92505050565b9050610ca06117be565b15610cb957853414610cb457610cb4612240565b610e0b565b3415610d6d576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152605060248201527f43726f7373446f6d61696e4d657373656e6765723a2076616c7565206d75737460448201527f206265207a65726f20756e6c657373206d6573736167652069732066726f6d2060648201527f612073797374656d206164647265737300000000000000000000000000000000608482015260a40161088b565b600081815260ce602052604090205460ff16610e0b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603060248201527f43726f7373446f6d61696e4d657373656e6765723a206d65737361676520636160448201527f6e6e6f74206265207265706c6179656400000000000000000000000000000000606482015260840161088b565b610e14876118e2565b15610ec7576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604360248201527f43726f7373446f6d61696e4d657373656e6765723a2063616e6e6f742073656e60448201527f64206d65737361676520746f20626c6f636b65642073797374656d206164647260648201527f6573730000000000000000000000000000000000000000000000000000000000608482015260a40161088b565b600081815260cb602052604090205460ff1615610f66576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603660248201527f43726f7373446f6d61696e4d657373656e6765723a206d65737361676520686160448201527f7320616c7265616479206265656e2072656c6179656400000000000000000000606482015260840161088b565b610f7261afc88661226f565b5a1015611001576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603760248201527f43726f7373446f6d61696e4d657373656e6765723a20696e737566666963696560448201527f6e742067617320746f2072656c6179206d657373616765000000000000000000606482015260840161088b565b60cc80547fffffffffffffffffffffffff00000000000000000000000000000000000000001673ffffffffffffffffffffffffffffffffffffffff8a16179055600061109d8861105561138861afc8612287565b5a6110609190612287565b8988888080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525061195992505050565b60cc80547fffffffffffffffffffffffff00000000000000000000000000000000000000001661dead179055905080151560010361113857600082815260cb602052604080822080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff001660011790555183917f4641df4a962071e12719d8c8c8e5ac7fc4d97b927346a3d7a335b1f7517e133c91a2611197565b600082815260ce602052604080822080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff001660011790555183917f99d0e048484baa1b1540b1367cb128acd7ab2946d1ed91ec10e3c85e4bf51b8f91a25b505060016097555050505050505050565b905090565b6111b5611332565b73ffffffffffffffffffffffffffffffffffffffff8116611258576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602660248201527f4f776e61626c653a206e6577206f776e657220697320746865207a65726f206160448201527f6464726573730000000000000000000000000000000000000000000000000000606482015260840161088b565b610abd81611565565b73ffffffffffffffffffffffffffffffffffffffff163b151590565b6040517fe9e05c4200000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff7f0000000000000000000000000000000000000000000000000000000000000000169063e9e05c429084906112fa90889083908990600090899060040161229e565b6000604051808303818588803b15801561131357600080fd5b505af1158015611327573d6000803e3d6000fd5b505050505050505050565b60335473ffffffffffffffffffffffffffffffffffffffff163314610720576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820181905260248201527f4f776e61626c653a2063616c6c6572206973206e6f7420746865206f776e6572604482015260640161088b565b6113bb611973565b606580547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff001690557f5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa335b60405173ffffffffffffffffffffffffffffffffffffffff909116815260200160405180910390a1565b60608160000361147357505060408051808201909152600181527f3000000000000000000000000000000000000000000000000000000000000000602082015290565b8160005b811561149d5780611487816122f6565b91506114969050600a8361232e565b9150611477565b60008167ffffffffffffffff8111156114b8576114b8612342565b6040519080825280601f01601f1916602001820160405280156114e2576020820181803683370190505b5090505b8415610b14576114f7600183612287565b9150611504600a86612371565b61150f90603061226f565b60f81b81838151811061152457611524612385565b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a90535061155e600a8661232e565b94506114e6565b6033805473ffffffffffffffffffffffffffffffffffffffff8381167fffffffffffffffffffffffff0000000000000000000000000000000000000000831681179093556040519116919082907f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e090600090a35050565b6000547501000000000000000000000000000000000000000000900460ff16611687576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602b60248201527f496e697469616c697a61626c653a20636f6e7472616374206973206e6f74206960448201527f6e697469616c697a696e67000000000000000000000000000000000000000000606482015260840161088b565b60cc80547fffffffffffffffffffffffff00000000000000000000000000000000000000001661dead1790556116bb6119df565b6116c3611a8a565b6116cb611b3e565b610720611c13565b6116db61172e565b606580547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff001660011790557f62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a2586114063390565b60655460ff1615610720576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601060248201527f5061757361626c653a2070617573656400000000000000000000000000000000604482015260640161088b565b60006117ab878787878787611cc5565b8051906020012090509695505050505050565b60003373ffffffffffffffffffffffffffffffffffffffff7f0000000000000000000000000000000000000000000000000000000000000000161480156111a857507f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff167f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16639bf62d826040518163ffffffff1660e01b8152600401602060405180830381865afa1580156118a2573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906118c691906123b4565b73ffffffffffffffffffffffffffffffffffffffff1614905090565b600073ffffffffffffffffffffffffffffffffffffffff821630148061195357507f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16145b92915050565b600080600080845160208601878a8af19695505050505050565b60655460ff16610720576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601460248201527f5061757361626c653a206e6f7420706175736564000000000000000000000000604482015260640161088b565b6000547501000000000000000000000000000000000000000000900460ff16610720576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602b60248201527f496e697469616c697a61626c653a20636f6e7472616374206973206e6f74206960448201527f6e697469616c697a696e67000000000000000000000000000000000000000000606482015260840161088b565b6000547501000000000000000000000000000000000000000000900460ff16611b35576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602b60248201527f496e697469616c697a61626c653a20636f6e7472616374206973206e6f74206960448201527f6e697469616c697a696e67000000000000000000000000000000000000000000606482015260840161088b565b61072033611565565b6000547501000000000000000000000000000000000000000000900460ff16611be9576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602b60248201527f496e697469616c697a61626c653a20636f6e7472616374206973206e6f74206960448201527f6e697469616c697a696e67000000000000000000000000000000000000000000606482015260840161088b565b606580547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00169055565b6000547501000000000000000000000000000000000000000000900460ff16611cbe576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602b60248201527f496e697469616c697a61626c653a20636f6e7472616374206973206e6f74206960448201527f6e697469616c697a696e67000000000000000000000000000000000000000000606482015260840161088b565b6001609755565b6060868686868686604051602401611ce2969594939291906123d1565b604080517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08184030181529190526020810180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff167fd764ad0b0000000000000000000000000000000000000000000000000000000017905290509695505050505050565b73ffffffffffffffffffffffffffffffffffffffff81168114610abd57600080fd5b60008083601f840112611d9857600080fd5b50813567ffffffffffffffff811115611db057600080fd5b602083019150836020828501011115611dc857600080fd5b9250929050565b803563ffffffff81168114611de357600080fd5b919050565b60008060008060608587031215611dfe57600080fd5b8435611e0981611d64565b9350602085013567ffffffffffffffff811115611e2557600080fd5b611e3187828801611d86565b9094509250611e44905060408601611dcf565b905092959194509250565b60005b83811015611e6a578181015183820152602001611e52565b83811115611e79576000848401525b50505050565b60008151808452611e97816020860160208601611e4f565b601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169290920160200192915050565b602081526000611edc6020830184611e7f565b9392505050565b600060208284031215611ef557600080fd5b5035919050565b600080600060408486031215611f1157600080fd5b833567ffffffffffffffff811115611f2857600080fd5b611f3486828701611d86565b9094509250611f47905060208501611dcf565b90509250925092565b600080600080600080600060c0888a031215611f6b57600080fd5b873596506020880135611f7d81611d64565b95506040880135611f8d81611d64565b9450606088013593506080880135925060a088013567ffffffffffffffff811115611fb757600080fd5b611fc38a828b01611d86565b989b979a50959850939692959293505050565b600060208284031215611fe857600080fd5b8135611edc81611d64565b8183528181602085013750600060208284010152600060207fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0601f840116840101905092915050565b878152600073ffffffffffffffffffffffffffffffffffffffff808916602084015280881660408401525085606083015263ffffffff8516608083015260c060a083015261208e60c083018486611ff3565b9998505050505050505050565b73ffffffffffffffffffffffffffffffffffffffff861681526080602082015260006120cb608083018688611ff3565b905083604083015263ffffffff831660608301529695505050505050565b600084516120fb818460208901611e4f565b80830190507f2e000000000000000000000000000000000000000000000000000000000000008082528551612137816001850160208a01611e4f565b60019201918201528351612152816002840160208801611e4f565b0160020195945050505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b600067ffffffffffffffff808316818516818304811182151516156121b5576121b561215f565b02949350505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fd5b600067ffffffffffffffff80841680612208576122086121be565b92169190910492915050565b600067ffffffffffffffff8083168185168083038211156122375761223761215f565b01949350505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052600160045260246000fd5b600082198211156122825761228261215f565b500190565b6000828210156122995761229961215f565b500390565b73ffffffffffffffffffffffffffffffffffffffff8616815284602082015267ffffffffffffffff84166040820152821515606082015260a0608082015260006122eb60a0830184611e7f565b979650505050505050565b60007fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff82036123275761232761215f565b5060010190565b60008261233d5761233d6121be565b500490565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b600082612380576123806121be565b500690565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b6000602082840312156123c657600080fd5b8151611edc81611d64565b868152600073ffffffffffffffffffffffffffffffffffffffff808816602084015280871660408401525084606083015283608083015260c060a083015261241c60c0830184611e7f565b9897505050505050505056fea164736f6c634300080f000a"

func init() {
	if err := json.Unmarshal([]byte(L1CrossDomainMessengerStorageLayoutJSON), L1CrossDomainMessengerStorageLayout); err != nil {
		panic(err)
	}

	layouts["L1CrossDomainMessenger"] = L1CrossDomainMessengerStorageLayout
	deployedBytecodes["L1CrossDomainMessenger"] = L1CrossDomainMessengerDeployedBin
}

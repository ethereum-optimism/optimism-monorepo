// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-bindings/solc"
)

const FaultDisputeGameStorageLayoutJSON = "{\"storage\":[{\"astId\":1000,\"contract\":\"src/dispute/FaultDisputeGame.sol:FaultDisputeGame\",\"label\":\"createdAt\",\"offset\":0,\"slot\":\"0\",\"type\":\"t_userDefinedValueType(Timestamp)1015\"},{\"astId\":1001,\"contract\":\"src/dispute/FaultDisputeGame.sol:FaultDisputeGame\",\"label\":\"resolvedAt\",\"offset\":8,\"slot\":\"0\",\"type\":\"t_userDefinedValueType(Timestamp)1015\"},{\"astId\":1002,\"contract\":\"src/dispute/FaultDisputeGame.sol:FaultDisputeGame\",\"label\":\"status\",\"offset\":16,\"slot\":\"0\",\"type\":\"t_enum(GameStatus)1009\"},{\"astId\":1003,\"contract\":\"src/dispute/FaultDisputeGame.sol:FaultDisputeGame\",\"label\":\"claimData\",\"offset\":0,\"slot\":\"1\",\"type\":\"t_array(t_struct(ClaimData)1010_storage)dyn_storage\"},{\"astId\":1004,\"contract\":\"src/dispute/FaultDisputeGame.sol:FaultDisputeGame\",\"label\":\"credit\",\"offset\":0,\"slot\":\"2\",\"type\":\"t_mapping(t_address,t_uint256)\"},{\"astId\":1005,\"contract\":\"src/dispute/FaultDisputeGame.sol:FaultDisputeGame\",\"label\":\"claims\",\"offset\":0,\"slot\":\"3\",\"type\":\"t_mapping(t_userDefinedValueType(ClaimHash)1012,t_bool)\"},{\"astId\":1006,\"contract\":\"src/dispute/FaultDisputeGame.sol:FaultDisputeGame\",\"label\":\"subgames\",\"offset\":0,\"slot\":\"4\",\"type\":\"t_mapping(t_uint256,t_array(t_uint256)dyn_storage)\"},{\"astId\":1007,\"contract\":\"src/dispute/FaultDisputeGame.sol:FaultDisputeGame\",\"label\":\"subgameAtRootResolved\",\"offset\":0,\"slot\":\"5\",\"type\":\"t_bool\"},{\"astId\":1008,\"contract\":\"src/dispute/FaultDisputeGame.sol:FaultDisputeGame\",\"label\":\"initialized\",\"offset\":1,\"slot\":\"5\",\"type\":\"t_bool\"}],\"types\":{\"t_address\":{\"encoding\":\"inplace\",\"label\":\"address\",\"numberOfBytes\":\"20\"},\"t_array(t_struct(ClaimData)1010_storage)dyn_storage\":{\"encoding\":\"dynamic_array\",\"label\":\"struct IFaultDisputeGame.ClaimData[]\",\"numberOfBytes\":\"32\",\"base\":\"t_struct(ClaimData)1010_storage\"},\"t_array(t_uint256)dyn_storage\":{\"encoding\":\"dynamic_array\",\"label\":\"uint256[]\",\"numberOfBytes\":\"32\",\"base\":\"t_uint256\"},\"t_bool\":{\"encoding\":\"inplace\",\"label\":\"bool\",\"numberOfBytes\":\"1\"},\"t_enum(GameStatus)1009\":{\"encoding\":\"inplace\",\"label\":\"enum GameStatus\",\"numberOfBytes\":\"1\"},\"t_mapping(t_address,t_uint256)\":{\"encoding\":\"mapping\",\"label\":\"mapping(address =\u003e uint256)\",\"numberOfBytes\":\"32\",\"key\":\"t_address\",\"value\":\"t_uint256\"},\"t_mapping(t_uint256,t_array(t_uint256)dyn_storage)\":{\"encoding\":\"mapping\",\"label\":\"mapping(uint256 =\u003e uint256[])\",\"numberOfBytes\":\"32\",\"key\":\"t_uint256\",\"value\":\"t_array(t_uint256)dyn_storage\"},\"t_mapping(t_userDefinedValueType(ClaimHash)1012,t_bool)\":{\"encoding\":\"mapping\",\"label\":\"mapping(ClaimHash =\u003e bool)\",\"numberOfBytes\":\"32\",\"key\":\"t_userDefinedValueType(ClaimHash)1012\",\"value\":\"t_bool\"},\"t_struct(ClaimData)1010_storage\":{\"encoding\":\"inplace\",\"label\":\"struct IFaultDisputeGame.ClaimData\",\"numberOfBytes\":\"160\"},\"t_uint128\":{\"encoding\":\"inplace\",\"label\":\"uint128\",\"numberOfBytes\":\"16\"},\"t_uint256\":{\"encoding\":\"inplace\",\"label\":\"uint256\",\"numberOfBytes\":\"32\"},\"t_uint32\":{\"encoding\":\"inplace\",\"label\":\"uint32\",\"numberOfBytes\":\"4\"},\"t_userDefinedValueType(Claim)1011\":{\"encoding\":\"inplace\",\"label\":\"Claim\",\"numberOfBytes\":\"32\"},\"t_userDefinedValueType(ClaimHash)1012\":{\"encoding\":\"inplace\",\"label\":\"ClaimHash\",\"numberOfBytes\":\"32\"},\"t_userDefinedValueType(Clock)1013\":{\"encoding\":\"inplace\",\"label\":\"Clock\",\"numberOfBytes\":\"16\"},\"t_userDefinedValueType(Position)1014\":{\"encoding\":\"inplace\",\"label\":\"Position\",\"numberOfBytes\":\"16\"},\"t_userDefinedValueType(Timestamp)1015\":{\"encoding\":\"inplace\",\"label\":\"Timestamp\",\"numberOfBytes\":\"8\"}}}"

var FaultDisputeGameStorageLayout = new(solc.StorageLayout)

var FaultDisputeGameDeployedBin = "0x6080604052600436106101d85760003560e01c80638d450a9511610102578063d8cc1a3c11610095578063f8f43ff611610064578063f8f43ff6146106f7578063fa24f74314610717578063fa315aa91461073b578063fdffbb281461076e57600080fd5b8063d8cc1a3c14610646578063e1f0c37614610666578063ec5e630814610699578063f3f7214e146106cc57600080fd5b8063c55cd0c7116100d1578063c55cd0c71461055b578063c6f0308c1461056e578063cf09e0d0146105f8578063d5d44d801461061957600080fd5b80638d450a9514610489578063bbdc02db146104bc578063bcef3b55146104fd578063c395e1ca1461053a57600080fd5b8063609d33341161017a57806368800abf1161014957806368800abf146103f95780638129fc1c1461042c5780638980e0cc146104345780638b85902b1461044957600080fd5b8063609d33341461037157806360e2746414610386578063632247ea146103a65780636361506d146103b957600080fd5b80632810e1d6116101b65780632810e1d6146102a057806335fef567146102b55780633a768463146102ca57806354fd4d501461031b57600080fd5b80630356fe3a146101dd57806319effeb41461021f578063200d2ed214610265575b600080fd5b3480156101e957600080fd5b507f00000000000000000000000000000000000000000000000000000000000000005b6040519081526020015b60405180910390f35b34801561022b57600080fd5b5060005461024c9068010000000000000000900467ffffffffffffffff1681565b60405167ffffffffffffffff9091168152602001610216565b34801561027157600080fd5b5060005461029390700100000000000000000000000000000000900460ff1681565b6040516102169190613385565b3480156102ac57600080fd5b50610293610781565b6102c86102c33660046133c6565b61097e565b005b3480156102d657600080fd5b5060405173ffffffffffffffffffffffffffffffffffffffff7f0000000000000000000000000000000000000000000000000000000000000000168152602001610216565b34801561032757600080fd5b506103646040518060400160405280600581526020017f302e342e3000000000000000000000000000000000000000000000000000000081525081565b6040516102169190613453565b34801561037d57600080fd5b5061036461098e565b34801561039257600080fd5b506102c86103a1366004613488565b6109a1565b6102c86103b43660046134c1565b610a51565b3480156103c557600080fd5b50367ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe81013560f01c90036020013561020c565b34801561040557600080fd5b507f000000000000000000000000000000000000000000000000000000000000000061020c565b6102c86111e6565b34801561044057600080fd5b5060015461020c565b34801561045557600080fd5b50367ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe81013560f01c90036040013561020c565b34801561049557600080fd5b507f000000000000000000000000000000000000000000000000000000000000000061020c565b3480156104c857600080fd5b5060405163ffffffff7f0000000000000000000000000000000000000000000000000000000000000000168152602001610216565b34801561050957600080fd5b50367ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe81013560f01c90033561020c565b34801561054657600080fd5b5061020c6105553660046134f6565b50600090565b6102c86105693660046133c6565b61157a565b34801561057a57600080fd5b5061058e610589366004613528565b611586565b6040805163ffffffff909816885273ffffffffffffffffffffffffffffffffffffffff968716602089015295909416948601949094526fffffffffffffffffffffffffffffffff9182166060860152608085015291821660a08401521660c082015260e001610216565b34801561060457600080fd5b5060005461024c9067ffffffffffffffff1681565b34801561062557600080fd5b5061020c610634366004613488565b60026020526000908152604090205481565b34801561065257600080fd5b506102c861066136600461358a565b61161d565b34801561067257600080fd5b507f000000000000000000000000000000000000000000000000000000000000000061024c565b3480156106a557600080fd5b507f000000000000000000000000000000000000000000000000000000000000000061020c565b3480156106d857600080fd5b506040516fffffffffffffffffffffffffffffffff8152602001610216565b34801561070357600080fd5b506102c8610712366004613614565b611bfb565b34801561072357600080fd5b5061072c612094565b60405161021693929190613640565b34801561074757600080fd5b507f000000000000000000000000000000000000000000000000000000000000000061020c565b6102c861077c366004613528565b6120f1565b600080600054700100000000000000000000000000000000900460ff1660028111156107af576107af613356565b146107e6576040517f67fe195000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60055460ff16610822576040517f9a07664600000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff16600160008154811061084e5761084e61366e565b6000918252602090912060059091020154640100000000900473ffffffffffffffffffffffffffffffffffffffff161461088957600161088c565b60025b6000805467ffffffffffffffff421668010000000000000000027fffffffffffffffffffffffffffffffff0000000000000000ffffffffffffffff82168117835592935083927fffffffffffffffffffffffffffffff00ffffffffffffffffffffffffffffffff167fffffffffffffffffffffffffffffff000000000000000000ffffffffffffffff9091161770010000000000000000000000000000000083600281111561093d5761093d613356565b02179055600281111561095257610952613356565b6040517f5e186f09b9c93491f14e277eea7faa5de6a2d4bda75a79af7a3684fbfb42da6090600090a290565b61098a82826000610a51565b5050565b606061099c60406020612552565b905090565b73ffffffffffffffffffffffffffffffffffffffff8116600081815260026020526040808220805490839055905190929083908381818185875af1925050503d8060008114610a0c576040519150601f19603f3d011682016040523d82523d6000602084013e610a11565b606091505b5050905080610a4c576040517f83e6cc6b00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b505050565b60008054700100000000000000000000000000000000900460ff166002811115610a7d57610a7d613356565b14610ab4576040517f67fe195000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600060018481548110610ac957610ac961366e565b600091825260208083206040805160e0810182526005909402909101805463ffffffff808216865273ffffffffffffffffffffffffffffffffffffffff6401000000009092048216948601949094526001820154169184019190915260028101546fffffffffffffffffffffffffffffffff90811660608501526003820154608085015260049091015480821660a0850181905270010000000000000000000000000000000090910490911660c0840152919350909190610b8e90839086906125e916565b90506000610c2e826fffffffffffffffffffffffffffffffff167e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b67ffffffffffffffff169050861580610c705750610c6d7f000000000000000000000000000000000000000000000000000000000000000060026136cc565b81145b8015610c7a575084155b15610cb1576040517fa42637bc00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b7f0000000000000000000000000000000000000000000000000000000000000000811115610d0b576040517f56f57b2b00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b610d367f000000000000000000000000000000000000000000000000000000000000000060016136cc565b8103610d4857610d48868885886125f1565b835160009063ffffffff90811614610da8576001856000015163ffffffff1681548110610d7757610d7761366e565b906000526020600020906005020160040160109054906101000a90046fffffffffffffffffffffffffffffffff1690505b60c0850151600090610dcc9067ffffffffffffffff165b67ffffffffffffffff1690565b67ffffffffffffffff1642610df6610dbf856fffffffffffffffffffffffffffffffff1660401c90565b67ffffffffffffffff16610e0a91906136cc565b610e1491906136e4565b90507f000000000000000000000000000000000000000000000000000000000000000060011c677fffffffffffffff1667ffffffffffffffff82161115610e87576040517f3381d11400000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000604082901b421760008a8152608087901b6fffffffffffffffffffffffffffffffff8d1617602052604081209192509060008181526003602052604090205490915060ff1615610f05576040517f80497e3b00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60016003600083815260200190815260200160002060006101000a81548160ff02191690831515021790555060016040518060e001604052808d63ffffffff168152602001600073ffffffffffffffffffffffffffffffffffffffff1681526020013373ffffffffffffffffffffffffffffffffffffffff168152602001346fffffffffffffffffffffffffffffffff1681526020018c8152602001886fffffffffffffffffffffffffffffffff168152602001846fffffffffffffffffffffffffffffffff16815250908060018154018082558091505060019003906000526020600020906005020160009091909190915060008201518160000160006101000a81548163ffffffff021916908363ffffffff16021790555060208201518160000160046101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060408201518160010160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060608201518160020160006101000a8154816fffffffffffffffffffffffffffffffff02191690836fffffffffffffffffffffffffffffffff1602179055506080820151816003015560a08201518160040160006101000a8154816fffffffffffffffffffffffffffffffff02191690836fffffffffffffffffffffffffffffffff16021790555060c08201518160040160106101000a8154816fffffffffffffffffffffffffffffffff02191690836fffffffffffffffffffffffffffffffff1602179055505050600460008c81526020019081526020016000206001808054905061119a91906136e4565b8154600181018355600092835260208320015560405133918c918e917f9b3245740ec3b155098a55be84957a4da13eaf7f14a8bc6f53126c0b9350f2be91a45050505050505050505050565b600554610100900460ff1615611228576040517f0dc149f000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b7f0000000000000000000000000000000000000000000000000000000000000000367ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe81013560f01c900360400135116112df576040517ff40239db000000000000000000000000000000000000000000000000000000008152367ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe81013560f01c90033560048201526024015b60405180910390fd5b60663611156112f65763c407e0256000526004601cfd5b6040805160e08101825263ffffffff808252600060208301818152329484019485526fffffffffffffffffffffffffffffffff348116606086019081527ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe369081013560f01c90033560808701908152600160a088018181524280861660c08b0190815283548085018555938952995160059384027fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf681018054995192909b167fffffffffffffffff0000000000000000000000000000000000000000000000009099169890981764010000000073ffffffffffffffffffffffffffffffffffffffff928316021790995599517fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf7870180547fffffffffffffffffffffffff000000000000000000000000000000000000000016919099161790975591517fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf8850180547fffffffffffffffffffffffffffffffff0000000000000000000000000000000016918516919091179055517fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf9840155935194519481167001000000000000000000000000000000009590911694909402939093177fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cfa9093019290925581547fffffffffffffffffffffffffffffffffffffffffffffffff00000000000000001667ffffffffffffffff90931692909217905580546101007fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00ff909116179055565b61098a82826001610a51565b6001818154811061159657600080fd5b60009182526020909120600590910201805460018201546002830154600384015460049094015463ffffffff8416955064010000000090930473ffffffffffffffffffffffffffffffffffffffff908116949216926fffffffffffffffffffffffffffffffff91821692918082169170010000000000000000000000000000000090041687565b60008054700100000000000000000000000000000000900460ff16600281111561164957611649613356565b14611680576040517f67fe195000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600187815481106116955761169561366e565b6000918252602082206005919091020160048101549092506fffffffffffffffffffffffffffffffff16908715821760011b90506116f47f000000000000000000000000000000000000000000000000000000000000000060016136cc565b611790826fffffffffffffffffffffffffffffffff167e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b67ffffffffffffffff16146117d1576040517f5f53dd9800000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b60008089156118c0576118247f00000000000000000000000000000000000000000000000000000000000000007f00000000000000000000000000000000000000000000000000000000000000006136e4565b6001901b611843846fffffffffffffffffffffffffffffffff166127b2565b67ffffffffffffffff16611857919061372a565b156118945761188b61187c60016fffffffffffffffffffffffffffffffff871661373e565b865463ffffffff166000612858565b600301546118b6565b7f00000000000000000000000000000000000000000000000000000000000000005b91508490506118ea565b600385015491506118e761187c6fffffffffffffffffffffffffffffffff8616600161376f565b90505b600882901b60088a8a6040516119019291906137a3565b6040518091039020901b14611942576040517f696550ff00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600061194d8c61293c565b9050600061195c836003015490565b6040517fe14ced320000000000000000000000000000000000000000000000000000000081527f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff169063e14ced32906119d6908f908f908f908f908a906004016137fc565b6020604051808303816000875af11580156119f5573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190611a199190613836565b600485015491149150600090600290611ac4906fffffffffffffffffffffffffffffffff167e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b611b60896fffffffffffffffffffffffffffffffff167e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b611b6a919061384f565b611b749190613870565b67ffffffffffffffff161590508115158103611bbc576040517ffb4e40dd00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b505085547fffffffffffffffff0000000000000000000000000000000000000000ffffffff163364010000000002179095555050505050505050505050565b60008054700100000000000000000000000000000000900460ff166002811115611c2757611c27613356565b14611c5e576040517f67fe195000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600080600080611c6d8661296b565b93509350935093506000611c8385858585612d98565b905060007f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff16637dc0d1d06040518163ffffffff1660e01b8152600401602060405180830381865afa158015611cf2573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190611d169190613897565b905060018903611e0e5773ffffffffffffffffffffffffffffffffffffffff81166352f0f3ad8a84611d72367ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe81013560f01c90036020013590565b6040517fffffffff0000000000000000000000000000000000000000000000000000000060e086901b16815260048101939093526024830191909152604482015260206064820152608481018a905260a4015b6020604051808303816000875af1158015611de4573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190611e089190613836565b50612089565b60028903611e3a5773ffffffffffffffffffffffffffffffffffffffff81166352f0f3ad8a8489611d72565b60038903611e665773ffffffffffffffffffffffffffffffffffffffff81166352f0f3ad8a8487611d72565b60048903611fde5760006fffffffffffffffffffffffffffffffff861615611efe57611ec46fffffffffffffffffffffffffffffffff87167f0000000000000000000000000000000000000000000000000000000000000000612e57565b611eee907f00000000000000000000000000000000000000000000000000000000000000006136cc565b611ef99060016136cc565b611f20565b7f00000000000000000000000000000000000000000000000000000000000000005b905073ffffffffffffffffffffffffffffffffffffffff82166352f0f3ad8b8560405160e084901b7fffffffff000000000000000000000000000000000000000000000000000000001681526004810192909252602482015260c084901b604482015260086064820152608481018b905260a4016020604051808303816000875af1158015611fb3573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190611fd79190613836565b5050612089565b60058903612057576040517f52f0f3ad000000000000000000000000000000000000000000000000000000008152600481018a9052602481018390524660c01b6044820152600860648201526084810188905273ffffffffffffffffffffffffffffffffffffffff8216906352f0f3ad9060a401611dc5565b6040517fff137e6500000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b505050505050505050565b7f0000000000000000000000000000000000000000000000000000000000000000367ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe81013560f01c90033560606120ea61098e565b9050909192565b60008054700100000000000000000000000000000000900460ff16600281111561211d5761211d613356565b14612154576040517f67fe195000000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600182815481106121695761216961366e565b6000918252602082206005919091020160048101549092506121ab90700100000000000000000000000000000000900460401c67ffffffffffffffff16610dbf565b60048301549091506000906121dd90700100000000000000000000000000000000900467ffffffffffffffff16610dbf565b6121e7904261384f565b9050677fffffffffffffff7f000000000000000000000000000000000000000000000000000000000000000060011c1661222182846138b4565b67ffffffffffffffff1611612262576040517ff2440b5300000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000848152600460205260409020805485158015612282575060055460ff165b156122b9576040517ff1a9458100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b801580156122c657508515155b1561232b578454640100000000900473ffffffffffffffffffffffffffffffffffffffff16600081156122f95781612315565b600187015473ffffffffffffffffffffffffffffffffffffffff165b90506123218188612f0c565b5050505050505050565b60006fffffffffffffffffffffffffffffffff815b8381101561247157600085828154811061235c5761235c61366e565b60009182526020808320909101548083526004909152604090912054909150156123b2576040517f9a07664600000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000600182815481106123c7576123c761366e565b600091825260209091206005909102018054909150640100000000900473ffffffffffffffffffffffffffffffffffffffff16158015612420575060048101546fffffffffffffffffffffffffffffffff908116908516115b1561245e576001810154600482015473ffffffffffffffffffffffffffffffffffffffff90911695506fffffffffffffffffffffffffffffffff1693505b50508061246a906138d7565b9050612340565b506124b973ffffffffffffffffffffffffffffffffffffffff83161561249757826124b3565b600188015473ffffffffffffffffffffffffffffffffffffffff165b88612f0c565b86547fffffffffffffffff0000000000000000000000000000000000000000ffffffff1664010000000073ffffffffffffffffffffffffffffffffffffffff84160217875560008881526004602052604081206125159161331c565b8760000361232157600580547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff001660011790555050505050505050565b6060600061258984367ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe81013560f01c90036136cc565b90508267ffffffffffffffff1667ffffffffffffffff8111156125ae576125ae61390f565b6040519080825280601f01601f1916602001820160405280156125d8576020820181803683370190505b509150828160208401375092915050565b151760011b90565b60006126106fffffffffffffffffffffffffffffffff8416600161376f565b9050600061262082866001612858565b9050600086901a8380612713575061265960027f000000000000000000000000000000000000000000000000000000000000000061372a565b60048301546002906126fd906fffffffffffffffffffffffffffffffff167e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b6127079190613870565b67ffffffffffffffff16145b1561276b5760ff81166001148061272d575060ff81166002145b612766576040517ff40239db000000000000000000000000000000000000000000000000000000008152600481018890526024016112d6565b6127a9565b60ff8116156127a9576040517ff40239db000000000000000000000000000000000000000000000000000000008152600481018890526024016112d6565b50505050505050565b60008061283f837e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b600167ffffffffffffffff919091161b90920392915050565b600080826128a15761289c6fffffffffffffffffffffffffffffffff86167f0000000000000000000000000000000000000000000000000000000000000000612ff9565b6128bc565b6128bc856fffffffffffffffffffffffffffffffff166131c0565b9050600184815481106128d1576128d161366e565b906000526020600020906005020191505b60048201546fffffffffffffffffffffffffffffffff82811691161461293457815460018054909163ffffffff1690811061291f5761291f61366e565b906000526020600020906005020191506128e2565b509392505050565b600080600080600061294d8661296b565b935093509350935061296184848484612d98565b9695505050505050565b600080600080600085905060006001828154811061298b5761298b61366e565b600091825260209091206004600590920201908101549091507f000000000000000000000000000000000000000000000000000000000000000090612a62906fffffffffffffffffffffffffffffffff167e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b67ffffffffffffffff1611612aa3576040517fb34b5c2200000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000815b60048301547f000000000000000000000000000000000000000000000000000000000000000090612b6a906fffffffffffffffffffffffffffffffff167e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b67ffffffffffffffff169250821115612be657825463ffffffff16612bb07f000000000000000000000000000000000000000000000000000000000000000060016136cc565b8303612bba578391505b60018181548110612bcd57612bcd61366e565b9060005260206000209060050201935080945050612aa7565b600481810154908401546fffffffffffffffffffffffffffffffff91821691166000816fffffffffffffffffffffffffffffffff16612c4f612c3a856fffffffffffffffffffffffffffffffff1660011c90565b6fffffffffffffffffffffffffffffffff1690565b6fffffffffffffffffffffffffffffffff161490508015612d34576000612c87836fffffffffffffffffffffffffffffffff166127b2565b67ffffffffffffffff161115612cea576000612cc1612cb960016fffffffffffffffffffffffffffffffff861661373e565b896001612858565b6003810154600490910154909c506fffffffffffffffffffffffffffffffff169a50612d0e9050565b7f00000000000000000000000000000000000000000000000000000000000000009a505b600386015460048701549099506fffffffffffffffffffffffffffffffff169750612d8a565b6000612d56612cb96fffffffffffffffffffffffffffffffff8516600161376f565b6003808901546004808b015492840154930154909e506fffffffffffffffffffffffffffffffff9182169d50919b50169850505b505050505050509193509193565b60006fffffffffffffffffffffffffffffffff84168103612dfe578282604051602001612de19291909182526fffffffffffffffffffffffffffffffff16602082015260400190565b604051602081830303815290604052805190602001209050612e4f565b60408051602081018790526fffffffffffffffffffffffffffffffff8087169282019290925260608101859052908316608082015260a0016040516020818303038152906040528051906020012090505b949350505050565b600080612ee4847e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b67ffffffffffffffff1690508083036001841b600180831b0386831b17039250505092915050565b60028101546fffffffffffffffffffffffffffffffff167fffffffffffffffffffffffffffffffff000000000000000000000000000000018101612f7c576040517ff1a9458100000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b600280830180547fffffffffffffffffffffffffffffffff00000000000000000000000000000000166fffffffffffffffffffffffffffffffff17905573ffffffffffffffffffffffffffffffffffffffff84166000908152602091909152604081208054839290612fef9084906136cc565b9091555050505050565b600081613098846fffffffffffffffffffffffffffffffff167e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b67ffffffffffffffff16116130d9576040517fb34b5c2200000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6130e2836131c0565b905081613181826fffffffffffffffffffffffffffffffff167e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b67ffffffffffffffff16116131ba576131b761319e8360016136cc565b6fffffffffffffffffffffffffffffffff83169061326c565b90505b92915050565b60008119600183011681613254827e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b67ffffffffffffffff169390931c8015179392505050565b6000806132f9847e09010a0d15021d0b0e10121619031e080c141c0f111807131b17061a05041f7f07c4acdd0000000000000000000000000000000000000000000000000000000067ffffffffffffffff831160061b83811c63ffffffff1060051b1792831c600181901c17600281901c17600481901c17600881901c17601081901c170260fb1c1a1790565b67ffffffffffffffff169050808303600180821b0385821b179250505092915050565b508054600082559060005260206000209081019061333a919061333d565b50565b5b80821115613352576000815560010161333e565b5090565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602160045260246000fd5b60208101600383106133c0577f4e487b7100000000000000000000000000000000000000000000000000000000600052602160045260246000fd5b91905290565b600080604083850312156133d957600080fd5b50508035926020909101359150565b6000815180845260005b8181101561340e576020818501810151868301820152016133f2565b81811115613420576000602083870101525b50601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169290920160200192915050565b6020815260006131b760208301846133e8565b73ffffffffffffffffffffffffffffffffffffffff8116811461333a57600080fd5b60006020828403121561349a57600080fd5b81356134a581613466565b9392505050565b803580151581146134bc57600080fd5b919050565b6000806000606084860312156134d657600080fd5b83359250602084013591506134ed604085016134ac565b90509250925092565b60006020828403121561350857600080fd5b81356fffffffffffffffffffffffffffffffff811681146134a557600080fd5b60006020828403121561353a57600080fd5b5035919050565b60008083601f84011261355357600080fd5b50813567ffffffffffffffff81111561356b57600080fd5b60208301915083602082850101111561358357600080fd5b9250929050565b600080600080600080608087890312156135a357600080fd5b863595506135b3602088016134ac565b9450604087013567ffffffffffffffff808211156135d057600080fd5b6135dc8a838b01613541565b909650945060608901359150808211156135f557600080fd5b5061360289828a01613541565b979a9699509497509295939492505050565b60008060006060848603121561362957600080fd5b505081359360208301359350604090920135919050565b63ffffffff8416815282602082015260606040820152600061366560608301846133e8565b95945050505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b600082198211156136df576136df61369d565b500190565b6000828210156136f6576136f661369d565b500390565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fd5b600082613739576137396136fb565b500690565b60006fffffffffffffffffffffffffffffffff838116908316818110156137675761376761369d565b039392505050565b60006fffffffffffffffffffffffffffffffff80831681851680830382111561379a5761379a61369d565b01949350505050565b8183823760009101908152919050565b8183528181602085013750600060208284010152600060207fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0601f840116840101905092915050565b6060815260006138106060830187896137b3565b82810360208401526138238186886137b3565b9150508260408301529695505050505050565b60006020828403121561384857600080fd5b5051919050565b600067ffffffffffffffff838116908316818110156137675761376761369d565b600067ffffffffffffffff8084168061388b5761388b6136fb565b92169190910692915050565b6000602082840312156138a957600080fd5b81516134a581613466565b600067ffffffffffffffff80831681851680830382111561379a5761379a61369d565b60007fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff82036139085761390861369d565b5060010190565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fdfea164736f6c634300080f000a"


func init() {
	if err := json.Unmarshal([]byte(FaultDisputeGameStorageLayoutJSON), FaultDisputeGameStorageLayout); err != nil {
		panic(err)
	}

	layouts["FaultDisputeGame"] = FaultDisputeGameStorageLayout
	deployedBytecodes["FaultDisputeGame"] = FaultDisputeGameDeployedBin
	immutableReferences["FaultDisputeGame"] = true
}

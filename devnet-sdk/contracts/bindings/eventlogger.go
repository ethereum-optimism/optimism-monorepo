// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// Identifier is an auto generated low-level Go binding around an user-defined struct.
type Identifier struct {
	Origin      common.Address
	BlockNumber *big.Int
	LogIndex    *big.Int
	Timestamp   *big.Int
	ChainId     *big.Int
}

// EventLoggerMetaData contains all meta data concerning the EventLogger contract.
var EventLoggerMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"emitLog\",\"inputs\":[{\"name\":\"_topics\",\"type\":\"bytes32[]\",\"internalType\":\"bytes32[]\"},{\"name\":\"_data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"validateMessage\",\"inputs\":[{\"name\":\"_id\",\"type\":\"tuple\",\"internalType\":\"structIdentifier\",\"components\":[{\"name\":\"origin\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"blockNumber\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"logIndex\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"chainId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"name\":\"_msgHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"}]",
	Bin: "0x6080604052348015600e575f80fd5b506105208061001c5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c8063ab4d6f7514610038578063edebc13b14610054575b5f80fd5b610052600480360381019061004d91906101dd565b610070565b005b61006e600480360381019061006991906102d1565b6100f0565b005b73420000000000000000000000000000000000002273ffffffffffffffffffffffffffffffffffffffff1663ab4d6f7583836040518363ffffffff1660e01b81526004016100bf9291906104c3565b5f604051808303815f87803b1580156100d6575f80fd5b505af11580156100e8573d5f803e3d5ffd5b505050505050565b8060405181848237845f6020028701356001602002880135600260200289013560036020028a0135845f8114610144576001811461014c5760028114610155576003811461015f576004811461016a575f80fd5b8787a0610172565b848888a1610172565b83858989a2610172565b8284868a8aa3610172565b818385878b8ba45b505050505050505050505050565b5f80fd5b5f80fd5b5f80fd5b5f60a082840312156101a1576101a0610188565b5b81905092915050565b5f819050919050565b6101bc816101aa565b81146101c6575f80fd5b50565b5f813590506101d7816101b3565b92915050565b5f8060c083850312156101f3576101f2610180565b5b5f6102008582860161018c565b92505060a0610211858286016101c9565b9150509250929050565b5f80fd5b5f80fd5b5f80fd5b5f8083601f84011261023c5761023b61021b565b5b8235905067ffffffffffffffff8111156102595761025861021f565b5b60208301915083602082028301111561027557610274610223565b5b9250929050565b5f8083601f8401126102915761029061021b565b5b8235905067ffffffffffffffff8111156102ae576102ad61021f565b5b6020830191508360018202830111156102ca576102c9610223565b5b9250929050565b5f805f80604085870312156102e9576102e8610180565b5b5f85013567ffffffffffffffff81111561030657610305610184565b5b61031287828801610227565b9450945050602085013567ffffffffffffffff81111561033557610334610184565b5b6103418782880161027c565b925092505092959194509250565b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6103788261034f565b9050919050565b6103888161036e565b8114610392575f80fd5b50565b5f813590506103a38161037f565b92915050565b5f6103b76020840184610395565b905092915050565b6103c88161036e565b82525050565b5f819050919050565b6103e0816103ce565b81146103ea575f80fd5b50565b5f813590506103fb816103d7565b92915050565b5f61040f60208401846103ed565b905092915050565b610420816103ce565b82525050565b60a082016104365f8301836103a9565b6104425f8501826103bf565b506104506020830183610401565b61045d6020850182610417565b5061046b6040830183610401565b6104786040850182610417565b506104866060830183610401565b6104936060850182610417565b506104a16080830183610401565b6104ae6080850182610417565b50505050565b6104bd816101aa565b82525050565b5f60c0820190506104d65f830185610426565b6104e360a08301846104b4565b939250505056fea26469706673582212202cee193ba83679fae70a7e3b1c200831da13d65383c496d48a41dfef7af1d2e364736f6c634300081a0033",
}

// EventLoggerABI is the input ABI used to generate the binding from.
// Deprecated: Use EventLoggerMetaData.ABI instead.
var EventLoggerABI = EventLoggerMetaData.ABI

// EventLoggerBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use EventLoggerMetaData.Bin instead.
var EventLoggerBin = EventLoggerMetaData.Bin

// DeployEventLogger deploys a new Ethereum contract, binding an instance of EventLogger to it.
func DeployEventLogger(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *EventLogger, error) {
	parsed, err := EventLoggerMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(EventLoggerBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &EventLogger{EventLoggerCaller: EventLoggerCaller{contract: contract}, EventLoggerTransactor: EventLoggerTransactor{contract: contract}, EventLoggerFilterer: EventLoggerFilterer{contract: contract}}, nil
}

// EventLogger is an auto generated Go binding around an Ethereum contract.
type EventLogger struct {
	EventLoggerCaller     // Read-only binding to the contract
	EventLoggerTransactor // Write-only binding to the contract
	EventLoggerFilterer   // Log filterer for contract events
}

// EventLoggerCaller is an auto generated read-only Go binding around an Ethereum contract.
type EventLoggerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EventLoggerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EventLoggerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EventLoggerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EventLoggerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EventLoggerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EventLoggerSession struct {
	Contract     *EventLogger      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EventLoggerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EventLoggerCallerSession struct {
	Contract *EventLoggerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// EventLoggerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EventLoggerTransactorSession struct {
	Contract     *EventLoggerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// EventLoggerRaw is an auto generated low-level Go binding around an Ethereum contract.
type EventLoggerRaw struct {
	Contract *EventLogger // Generic contract binding to access the raw methods on
}

// EventLoggerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EventLoggerCallerRaw struct {
	Contract *EventLoggerCaller // Generic read-only contract binding to access the raw methods on
}

// EventLoggerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EventLoggerTransactorRaw struct {
	Contract *EventLoggerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEventLogger creates a new instance of EventLogger, bound to a specific deployed contract.
func NewEventLogger(address common.Address, backend bind.ContractBackend) (*EventLogger, error) {
	contract, err := bindEventLogger(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &EventLogger{EventLoggerCaller: EventLoggerCaller{contract: contract}, EventLoggerTransactor: EventLoggerTransactor{contract: contract}, EventLoggerFilterer: EventLoggerFilterer{contract: contract}}, nil
}

// NewEventLoggerCaller creates a new read-only instance of EventLogger, bound to a specific deployed contract.
func NewEventLoggerCaller(address common.Address, caller bind.ContractCaller) (*EventLoggerCaller, error) {
	contract, err := bindEventLogger(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EventLoggerCaller{contract: contract}, nil
}

// NewEventLoggerTransactor creates a new write-only instance of EventLogger, bound to a specific deployed contract.
func NewEventLoggerTransactor(address common.Address, transactor bind.ContractTransactor) (*EventLoggerTransactor, error) {
	contract, err := bindEventLogger(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EventLoggerTransactor{contract: contract}, nil
}

// NewEventLoggerFilterer creates a new log filterer instance of EventLogger, bound to a specific deployed contract.
func NewEventLoggerFilterer(address common.Address, filterer bind.ContractFilterer) (*EventLoggerFilterer, error) {
	contract, err := bindEventLogger(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EventLoggerFilterer{contract: contract}, nil
}

// bindEventLogger binds a generic wrapper to an already deployed contract.
func bindEventLogger(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(EventLoggerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EventLogger *EventLoggerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _EventLogger.Contract.EventLoggerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EventLogger *EventLoggerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EventLogger.Contract.EventLoggerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EventLogger *EventLoggerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EventLogger.Contract.EventLoggerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EventLogger *EventLoggerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _EventLogger.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EventLogger *EventLoggerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EventLogger.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EventLogger *EventLoggerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EventLogger.Contract.contract.Transact(opts, method, params...)
}

// EmitLog is a paid mutator transaction binding the contract method 0xedebc13b.
//
// Solidity: function emitLog(bytes32[] _topics, bytes _data) returns()
func (_EventLogger *EventLoggerTransactor) EmitLog(opts *bind.TransactOpts, _topics [][32]byte, _data []byte) (*types.Transaction, error) {
	return _EventLogger.contract.Transact(opts, "emitLog", _topics, _data)
}

// EmitLog is a paid mutator transaction binding the contract method 0xedebc13b.
//
// Solidity: function emitLog(bytes32[] _topics, bytes _data) returns()
func (_EventLogger *EventLoggerSession) EmitLog(_topics [][32]byte, _data []byte) (*types.Transaction, error) {
	return _EventLogger.Contract.EmitLog(&_EventLogger.TransactOpts, _topics, _data)
}

// EmitLog is a paid mutator transaction binding the contract method 0xedebc13b.
//
// Solidity: function emitLog(bytes32[] _topics, bytes _data) returns()
func (_EventLogger *EventLoggerTransactorSession) EmitLog(_topics [][32]byte, _data []byte) (*types.Transaction, error) {
	return _EventLogger.Contract.EmitLog(&_EventLogger.TransactOpts, _topics, _data)
}

// ValidateMessage is a paid mutator transaction binding the contract method 0xab4d6f75.
//
// Solidity: function validateMessage((address,uint256,uint256,uint256,uint256) _id, bytes32 _msgHash) returns()
func (_EventLogger *EventLoggerTransactor) ValidateMessage(opts *bind.TransactOpts, _id Identifier, _msgHash [32]byte) (*types.Transaction, error) {
	return _EventLogger.contract.Transact(opts, "validateMessage", _id, _msgHash)
}

// ValidateMessage is a paid mutator transaction binding the contract method 0xab4d6f75.
//
// Solidity: function validateMessage((address,uint256,uint256,uint256,uint256) _id, bytes32 _msgHash) returns()
func (_EventLogger *EventLoggerSession) ValidateMessage(_id Identifier, _msgHash [32]byte) (*types.Transaction, error) {
	return _EventLogger.Contract.ValidateMessage(&_EventLogger.TransactOpts, _id, _msgHash)
}

// ValidateMessage is a paid mutator transaction binding the contract method 0xab4d6f75.
//
// Solidity: function validateMessage((address,uint256,uint256,uint256,uint256) _id, bytes32 _msgHash) returns()
func (_EventLogger *EventLoggerTransactorSession) ValidateMessage(_id Identifier, _msgHash [32]byte) (*types.Transaction, error) {
	return _EventLogger.Contract.ValidateMessage(&_EventLogger.TransactOpts, _id, _msgHash)
}

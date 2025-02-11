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
}

// EventLoggerABI is the input ABI used to generate the binding from.
// Deprecated: Use EventLoggerMetaData.ABI instead.
var EventLoggerABI = EventLoggerMetaData.ABI

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

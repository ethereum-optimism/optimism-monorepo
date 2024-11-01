// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package emit

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

// EmitMetaData contains all meta data concerning the Emit contract.
var EmitMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"emitData\",\"inputs\":[{\"name\":\"_data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"DataEmitted\",\"inputs\":[{\"name\":\"_data\",\"type\":\"bytes\",\"indexed\":true,\"internalType\":\"bytes\"}],\"anonymous\":false}]",
	Bin: "0x6080604052348015600e575f80fd5b5060ff8061001b5f395ff3fe6080604052348015600e575f80fd5b50600436106026575f3560e01c8063d836083e14602a575b5f80fd5b60396035366004607c565b603b565b005b8181604051604992919060e3565b604051908190038120907fe00bbfe6f6f8f1bbed2da38e3f5a139c6f9da594ab248a3cf8b44fc73627772c905f90a25050565b5f8060208385031215608c575f80fd5b823567ffffffffffffffff8082111560a2575f80fd5b818501915085601f83011260b4575f80fd5b81358181111560c1575f80fd5b86602082850101111560d1575f80fd5b60209290920196919550909350505050565b818382375f910190815291905056fea164736f6c6343000819000a",
}

// EmitABI is the input ABI used to generate the binding from.
// Deprecated: Use EmitMetaData.ABI instead.
var EmitABI = EmitMetaData.ABI

// EmitBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use EmitMetaData.Bin instead.
var EmitBin = EmitMetaData.Bin

// DeployEmit deploys a new Ethereum contract, binding an instance of Emit to it.
func DeployEmit(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Emit, error) {
	parsed, err := EmitMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(EmitBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Emit{EmitCaller: EmitCaller{contract: contract}, EmitTransactor: EmitTransactor{contract: contract}, EmitFilterer: EmitFilterer{contract: contract}}, nil
}

// Emit is an auto generated Go binding around an Ethereum contract.
type Emit struct {
	EmitCaller     // Read-only binding to the contract
	EmitTransactor // Write-only binding to the contract
	EmitFilterer   // Log filterer for contract events
}

// EmitCaller is an auto generated read-only Go binding around an Ethereum contract.
type EmitCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EmitTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EmitTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EmitFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EmitFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EmitSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EmitSession struct {
	Contract     *Emit             // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EmitCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EmitCallerSession struct {
	Contract *EmitCaller   // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// EmitTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EmitTransactorSession struct {
	Contract     *EmitTransactor   // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EmitRaw is an auto generated low-level Go binding around an Ethereum contract.
type EmitRaw struct {
	Contract *Emit // Generic contract binding to access the raw methods on
}

// EmitCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EmitCallerRaw struct {
	Contract *EmitCaller // Generic read-only contract binding to access the raw methods on
}

// EmitTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EmitTransactorRaw struct {
	Contract *EmitTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEmit creates a new instance of Emit, bound to a specific deployed contract.
func NewEmit(address common.Address, backend bind.ContractBackend) (*Emit, error) {
	contract, err := bindEmit(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Emit{EmitCaller: EmitCaller{contract: contract}, EmitTransactor: EmitTransactor{contract: contract}, EmitFilterer: EmitFilterer{contract: contract}}, nil
}

// NewEmitCaller creates a new read-only instance of Emit, bound to a specific deployed contract.
func NewEmitCaller(address common.Address, caller bind.ContractCaller) (*EmitCaller, error) {
	contract, err := bindEmit(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EmitCaller{contract: contract}, nil
}

// NewEmitTransactor creates a new write-only instance of Emit, bound to a specific deployed contract.
func NewEmitTransactor(address common.Address, transactor bind.ContractTransactor) (*EmitTransactor, error) {
	contract, err := bindEmit(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EmitTransactor{contract: contract}, nil
}

// NewEmitFilterer creates a new log filterer instance of Emit, bound to a specific deployed contract.
func NewEmitFilterer(address common.Address, filterer bind.ContractFilterer) (*EmitFilterer, error) {
	contract, err := bindEmit(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EmitFilterer{contract: contract}, nil
}

// bindEmit binds a generic wrapper to an already deployed contract.
func bindEmit(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(EmitABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Emit *EmitRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Emit.Contract.EmitCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Emit *EmitRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Emit.Contract.EmitTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Emit *EmitRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Emit.Contract.EmitTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Emit *EmitCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Emit.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Emit *EmitTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Emit.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Emit *EmitTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Emit.Contract.contract.Transact(opts, method, params...)
}

// EmitData is a paid mutator transaction binding the contract method 0xd836083e.
//
// Solidity: function emitData(bytes _data) returns()
func (_Emit *EmitTransactor) EmitData(opts *bind.TransactOpts, _data []byte) (*types.Transaction, error) {
	return _Emit.contract.Transact(opts, "emitData", _data)
}

// EmitData is a paid mutator transaction binding the contract method 0xd836083e.
//
// Solidity: function emitData(bytes _data) returns()
func (_Emit *EmitSession) EmitData(_data []byte) (*types.Transaction, error) {
	return _Emit.Contract.EmitData(&_Emit.TransactOpts, _data)
}

// EmitData is a paid mutator transaction binding the contract method 0xd836083e.
//
// Solidity: function emitData(bytes _data) returns()
func (_Emit *EmitTransactorSession) EmitData(_data []byte) (*types.Transaction, error) {
	return _Emit.Contract.EmitData(&_Emit.TransactOpts, _data)
}

// EmitDataEmittedIterator is returned from FilterDataEmitted and is used to iterate over the raw logs and unpacked data for DataEmitted events raised by the Emit contract.
type EmitDataEmittedIterator struct {
	Event *EmitDataEmitted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EmitDataEmittedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EmitDataEmitted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EmitDataEmitted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EmitDataEmittedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EmitDataEmittedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EmitDataEmitted represents a DataEmitted event raised by the Emit contract.
type EmitDataEmitted struct {
	Data common.Hash
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterDataEmitted is a free log retrieval operation binding the contract event 0xe00bbfe6f6f8f1bbed2da38e3f5a139c6f9da594ab248a3cf8b44fc73627772c.
//
// Solidity: event DataEmitted(bytes indexed _data)
func (_Emit *EmitFilterer) FilterDataEmitted(opts *bind.FilterOpts, _data [][]byte) (*EmitDataEmittedIterator, error) {

	var _dataRule []interface{}
	for _, _dataItem := range _data {
		_dataRule = append(_dataRule, _dataItem)
	}

	logs, sub, err := _Emit.contract.FilterLogs(opts, "DataEmitted", _dataRule)
	if err != nil {
		return nil, err
	}
	return &EmitDataEmittedIterator{contract: _Emit.contract, event: "DataEmitted", logs: logs, sub: sub}, nil
}

// WatchDataEmitted is a free log subscription operation binding the contract event 0xe00bbfe6f6f8f1bbed2da38e3f5a139c6f9da594ab248a3cf8b44fc73627772c.
//
// Solidity: event DataEmitted(bytes indexed _data)
func (_Emit *EmitFilterer) WatchDataEmitted(opts *bind.WatchOpts, sink chan<- *EmitDataEmitted, _data [][]byte) (event.Subscription, error) {

	var _dataRule []interface{}
	for _, _dataItem := range _data {
		_dataRule = append(_dataRule, _dataItem)
	}

	logs, sub, err := _Emit.contract.WatchLogs(opts, "DataEmitted", _dataRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EmitDataEmitted)
				if err := _Emit.contract.UnpackLog(event, "DataEmitted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDataEmitted is a log parse operation binding the contract event 0xe00bbfe6f6f8f1bbed2da38e3f5a139c6f9da594ab248a3cf8b44fc73627772c.
//
// Solidity: event DataEmitted(bytes indexed _data)
func (_Emit *EmitFilterer) ParseDataEmitted(log types.Log) (*EmitDataEmitted, error) {
	event := new(EmitDataEmitted)
	if err := _Emit.contract.UnpackLog(event, "DataEmitted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

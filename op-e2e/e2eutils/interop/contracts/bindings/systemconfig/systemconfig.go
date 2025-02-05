// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package systemconfig

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

// SystemconfigMetaData contains all meta data concerning the Systemconfig contract.
var SystemconfigMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"addDependency\",\"inputs\":[{\"name\":\"_chainId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"}]",
}

// SystemconfigABI is the input ABI used to generate the binding from.
// Deprecated: Use SystemconfigMetaData.ABI instead.
var SystemconfigABI = SystemconfigMetaData.ABI

// Systemconfig is an auto generated Go binding around an Ethereum contract.
type Systemconfig struct {
	SystemconfigCaller     // Read-only binding to the contract
	SystemconfigTransactor // Write-only binding to the contract
	SystemconfigFilterer   // Log filterer for contract events
}

// SystemconfigCaller is an auto generated read-only Go binding around an Ethereum contract.
type SystemconfigCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SystemconfigTransactor is an auto generated write-only Go binding around an Ethereum contract.
type SystemconfigTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SystemconfigFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type SystemconfigFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SystemconfigSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type SystemconfigSession struct {
	Contract     *Systemconfig     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// SystemconfigCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type SystemconfigCallerSession struct {
	Contract *SystemconfigCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// SystemconfigTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type SystemconfigTransactorSession struct {
	Contract     *SystemconfigTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// SystemconfigRaw is an auto generated low-level Go binding around an Ethereum contract.
type SystemconfigRaw struct {
	Contract *Systemconfig // Generic contract binding to access the raw methods on
}

// SystemconfigCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type SystemconfigCallerRaw struct {
	Contract *SystemconfigCaller // Generic read-only contract binding to access the raw methods on
}

// SystemconfigTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type SystemconfigTransactorRaw struct {
	Contract *SystemconfigTransactor // Generic write-only contract binding to access the raw methods on
}

// NewSystemconfig creates a new instance of Systemconfig, bound to a specific deployed contract.
func NewSystemconfig(address common.Address, backend bind.ContractBackend) (*Systemconfig, error) {
	contract, err := bindSystemconfig(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Systemconfig{SystemconfigCaller: SystemconfigCaller{contract: contract}, SystemconfigTransactor: SystemconfigTransactor{contract: contract}, SystemconfigFilterer: SystemconfigFilterer{contract: contract}}, nil
}

// NewSystemconfigCaller creates a new read-only instance of Systemconfig, bound to a specific deployed contract.
func NewSystemconfigCaller(address common.Address, caller bind.ContractCaller) (*SystemconfigCaller, error) {
	contract, err := bindSystemconfig(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &SystemconfigCaller{contract: contract}, nil
}

// NewSystemconfigTransactor creates a new write-only instance of Systemconfig, bound to a specific deployed contract.
func NewSystemconfigTransactor(address common.Address, transactor bind.ContractTransactor) (*SystemconfigTransactor, error) {
	contract, err := bindSystemconfig(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &SystemconfigTransactor{contract: contract}, nil
}

// NewSystemconfigFilterer creates a new log filterer instance of Systemconfig, bound to a specific deployed contract.
func NewSystemconfigFilterer(address common.Address, filterer bind.ContractFilterer) (*SystemconfigFilterer, error) {
	contract, err := bindSystemconfig(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &SystemconfigFilterer{contract: contract}, nil
}

// bindSystemconfig binds a generic wrapper to an already deployed contract.
func bindSystemconfig(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(SystemconfigABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Systemconfig *SystemconfigRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Systemconfig.Contract.SystemconfigCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Systemconfig *SystemconfigRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Systemconfig.Contract.SystemconfigTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Systemconfig *SystemconfigRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Systemconfig.Contract.SystemconfigTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Systemconfig *SystemconfigCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Systemconfig.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Systemconfig *SystemconfigTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Systemconfig.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Systemconfig *SystemconfigTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Systemconfig.Contract.contract.Transact(opts, method, params...)
}

// AddDependency is a paid mutator transaction binding the contract method 0xa89c793c.
//
// Solidity: function addDependency(uint256 _chainId) returns()
func (_Systemconfig *SystemconfigTransactor) AddDependency(opts *bind.TransactOpts, _chainId *big.Int) (*types.Transaction, error) {
	return _Systemconfig.contract.Transact(opts, "addDependency", _chainId)
}

// AddDependency is a paid mutator transaction binding the contract method 0xa89c793c.
//
// Solidity: function addDependency(uint256 _chainId) returns()
func (_Systemconfig *SystemconfigSession) AddDependency(_chainId *big.Int) (*types.Transaction, error) {
	return _Systemconfig.Contract.AddDependency(&_Systemconfig.TransactOpts, _chainId)
}

// AddDependency is a paid mutator transaction binding the contract method 0xa89c793c.
//
// Solidity: function addDependency(uint256 _chainId) returns()
func (_Systemconfig *SystemconfigTransactorSession) AddDependency(_chainId *big.Int) (*types.Transaction, error) {
	return _Systemconfig.Contract.AddDependency(&_Systemconfig.TransactOpts, _chainId)
}

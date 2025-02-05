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

// SuperchainWETHMetaData contains all meta data concerning the SuperchainWETH contract.
var SuperchainWETHMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"fallback\",\"stateMutability\":\"payable\"},{\"type\":\"receive\",\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"allowance\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"approve\",\"inputs\":[{\"name\":\"guy\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"wad\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"balanceOf\",\"inputs\":[{\"name\":\"src\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"crosschainBurn\",\"inputs\":[{\"name\":\"_from\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"crosschainMint\",\"inputs\":[{\"name\":\"_to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"decimals\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"deposit\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"name\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"relayETH\",\"inputs\":[{\"name\":\"_from\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"sendETH\",\"inputs\":[{\"name\":\"_to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_chainId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"msgHash_\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"supportsInterface\",\"inputs\":[{\"name\":\"_interfaceId\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"symbol\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"totalSupply\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"transfer\",\"inputs\":[{\"name\":\"dst\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"wad\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"transferFrom\",\"inputs\":[{\"name\":\"src\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"dst\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"wad\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"version\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"withdraw\",\"inputs\":[{\"name\":\"_amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"Approval\",\"inputs\":[{\"name\":\"src\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"guy\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"wad\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"CrosschainBurn\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"CrosschainMint\",\"inputs\":[{\"name\":\"to\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Deposit\",\"inputs\":[{\"name\":\"dst\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"wad\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RelayETH\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"source\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"SendETH\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"destination\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Transfer\",\"inputs\":[{\"name\":\"src\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"dst\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"wad\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Withdrawal\",\"inputs\":[{\"name\":\"src\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"wad\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"InvalidCrossDomainSender\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotCustomGasToken\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"Unauthorized\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ZeroAddress\",\"inputs\":[]}]",
}

// SuperchainWETHABI is the input ABI used to generate the binding from.
// Deprecated: Use SuperchainWETHMetaData.ABI instead.
var SuperchainWETHABI = SuperchainWETHMetaData.ABI

// SuperchainWETH is an auto generated Go binding around an Ethereum contract.
type SuperchainWETH struct {
	SuperchainWETHCaller     // Read-only binding to the contract
	SuperchainWETHTransactor // Write-only binding to the contract
	SuperchainWETHFilterer   // Log filterer for contract events
}

// SuperchainWETHCaller is an auto generated read-only Go binding around an Ethereum contract.
type SuperchainWETHCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SuperchainWETHTransactor is an auto generated write-only Go binding around an Ethereum contract.
type SuperchainWETHTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SuperchainWETHFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type SuperchainWETHFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SuperchainWETHSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type SuperchainWETHSession struct {
	Contract     *SuperchainWETH   // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// SuperchainWETHCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type SuperchainWETHCallerSession struct {
	Contract *SuperchainWETHCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts         // Call options to use throughout this session
}

// SuperchainWETHTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type SuperchainWETHTransactorSession struct {
	Contract     *SuperchainWETHTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts         // Transaction auth options to use throughout this session
}

// SuperchainWETHRaw is an auto generated low-level Go binding around an Ethereum contract.
type SuperchainWETHRaw struct {
	Contract *SuperchainWETH // Generic contract binding to access the raw methods on
}

// SuperchainWETHCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type SuperchainWETHCallerRaw struct {
	Contract *SuperchainWETHCaller // Generic read-only contract binding to access the raw methods on
}

// SuperchainWETHTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type SuperchainWETHTransactorRaw struct {
	Contract *SuperchainWETHTransactor // Generic write-only contract binding to access the raw methods on
}

// NewSuperchainWETH creates a new instance of SuperchainWETH, bound to a specific deployed contract.
func NewSuperchainWETH(address common.Address, backend bind.ContractBackend) (*SuperchainWETH, error) {
	contract, err := bindSuperchainWETH(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETH{SuperchainWETHCaller: SuperchainWETHCaller{contract: contract}, SuperchainWETHTransactor: SuperchainWETHTransactor{contract: contract}, SuperchainWETHFilterer: SuperchainWETHFilterer{contract: contract}}, nil
}

// NewSuperchainWETHCaller creates a new read-only instance of SuperchainWETH, bound to a specific deployed contract.
func NewSuperchainWETHCaller(address common.Address, caller bind.ContractCaller) (*SuperchainWETHCaller, error) {
	contract, err := bindSuperchainWETH(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHCaller{contract: contract}, nil
}

// NewSuperchainWETHTransactor creates a new write-only instance of SuperchainWETH, bound to a specific deployed contract.
func NewSuperchainWETHTransactor(address common.Address, transactor bind.ContractTransactor) (*SuperchainWETHTransactor, error) {
	contract, err := bindSuperchainWETH(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHTransactor{contract: contract}, nil
}

// NewSuperchainWETHFilterer creates a new log filterer instance of SuperchainWETH, bound to a specific deployed contract.
func NewSuperchainWETHFilterer(address common.Address, filterer bind.ContractFilterer) (*SuperchainWETHFilterer, error) {
	contract, err := bindSuperchainWETH(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHFilterer{contract: contract}, nil
}

// bindSuperchainWETH binds a generic wrapper to an already deployed contract.
func bindSuperchainWETH(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(SuperchainWETHABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SuperchainWETH *SuperchainWETHRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SuperchainWETH.Contract.SuperchainWETHCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SuperchainWETH *SuperchainWETHRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.SuperchainWETHTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SuperchainWETH *SuperchainWETHRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.SuperchainWETHTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SuperchainWETH *SuperchainWETHCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SuperchainWETH.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SuperchainWETH *SuperchainWETHTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SuperchainWETH *SuperchainWETHTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.contract.Transact(opts, method, params...)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_SuperchainWETH *SuperchainWETHCaller) Allowance(opts *bind.CallOpts, owner common.Address, spender common.Address) (*big.Int, error) {
	var out []interface{}
	err := _SuperchainWETH.contract.Call(opts, &out, "allowance", owner, spender)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_SuperchainWETH *SuperchainWETHSession) Allowance(owner common.Address, spender common.Address) (*big.Int, error) {
	return _SuperchainWETH.Contract.Allowance(&_SuperchainWETH.CallOpts, owner, spender)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address owner, address spender) view returns(uint256)
func (_SuperchainWETH *SuperchainWETHCallerSession) Allowance(owner common.Address, spender common.Address) (*big.Int, error) {
	return _SuperchainWETH.Contract.Allowance(&_SuperchainWETH.CallOpts, owner, spender)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address src) view returns(uint256)
func (_SuperchainWETH *SuperchainWETHCaller) BalanceOf(opts *bind.CallOpts, src common.Address) (*big.Int, error) {
	var out []interface{}
	err := _SuperchainWETH.contract.Call(opts, &out, "balanceOf", src)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address src) view returns(uint256)
func (_SuperchainWETH *SuperchainWETHSession) BalanceOf(src common.Address) (*big.Int, error) {
	return _SuperchainWETH.Contract.BalanceOf(&_SuperchainWETH.CallOpts, src)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address src) view returns(uint256)
func (_SuperchainWETH *SuperchainWETHCallerSession) BalanceOf(src common.Address) (*big.Int, error) {
	return _SuperchainWETH.Contract.BalanceOf(&_SuperchainWETH.CallOpts, src)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_SuperchainWETH *SuperchainWETHCaller) Decimals(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _SuperchainWETH.contract.Call(opts, &out, "decimals")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_SuperchainWETH *SuperchainWETHSession) Decimals() (uint8, error) {
	return _SuperchainWETH.Contract.Decimals(&_SuperchainWETH.CallOpts)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_SuperchainWETH *SuperchainWETHCallerSession) Decimals() (uint8, error) {
	return _SuperchainWETH.Contract.Decimals(&_SuperchainWETH.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_SuperchainWETH *SuperchainWETHCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _SuperchainWETH.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_SuperchainWETH *SuperchainWETHSession) Name() (string, error) {
	return _SuperchainWETH.Contract.Name(&_SuperchainWETH.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_SuperchainWETH *SuperchainWETHCallerSession) Name() (string, error) {
	return _SuperchainWETH.Contract.Name(&_SuperchainWETH.CallOpts)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 _interfaceId) view returns(bool)
func (_SuperchainWETH *SuperchainWETHCaller) SupportsInterface(opts *bind.CallOpts, _interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _SuperchainWETH.contract.Call(opts, &out, "supportsInterface", _interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 _interfaceId) view returns(bool)
func (_SuperchainWETH *SuperchainWETHSession) SupportsInterface(_interfaceId [4]byte) (bool, error) {
	return _SuperchainWETH.Contract.SupportsInterface(&_SuperchainWETH.CallOpts, _interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 _interfaceId) view returns(bool)
func (_SuperchainWETH *SuperchainWETHCallerSession) SupportsInterface(_interfaceId [4]byte) (bool, error) {
	return _SuperchainWETH.Contract.SupportsInterface(&_SuperchainWETH.CallOpts, _interfaceId)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_SuperchainWETH *SuperchainWETHCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _SuperchainWETH.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_SuperchainWETH *SuperchainWETHSession) Symbol() (string, error) {
	return _SuperchainWETH.Contract.Symbol(&_SuperchainWETH.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_SuperchainWETH *SuperchainWETHCallerSession) Symbol() (string, error) {
	return _SuperchainWETH.Contract.Symbol(&_SuperchainWETH.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_SuperchainWETH *SuperchainWETHCaller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SuperchainWETH.contract.Call(opts, &out, "totalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_SuperchainWETH *SuperchainWETHSession) TotalSupply() (*big.Int, error) {
	return _SuperchainWETH.Contract.TotalSupply(&_SuperchainWETH.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_SuperchainWETH *SuperchainWETHCallerSession) TotalSupply() (*big.Int, error) {
	return _SuperchainWETH.Contract.TotalSupply(&_SuperchainWETH.CallOpts)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() view returns(string)
func (_SuperchainWETH *SuperchainWETHCaller) Version(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _SuperchainWETH.contract.Call(opts, &out, "version")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() view returns(string)
func (_SuperchainWETH *SuperchainWETHSession) Version() (string, error) {
	return _SuperchainWETH.Contract.Version(&_SuperchainWETH.CallOpts)
}

// Version is a free data retrieval call binding the contract method 0x54fd4d50.
//
// Solidity: function version() view returns(string)
func (_SuperchainWETH *SuperchainWETHCallerSession) Version() (string, error) {
	return _SuperchainWETH.Contract.Version(&_SuperchainWETH.CallOpts)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address guy, uint256 wad) returns(bool)
func (_SuperchainWETH *SuperchainWETHTransactor) Approve(opts *bind.TransactOpts, guy common.Address, wad *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.contract.Transact(opts, "approve", guy, wad)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address guy, uint256 wad) returns(bool)
func (_SuperchainWETH *SuperchainWETHSession) Approve(guy common.Address, wad *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Approve(&_SuperchainWETH.TransactOpts, guy, wad)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address guy, uint256 wad) returns(bool)
func (_SuperchainWETH *SuperchainWETHTransactorSession) Approve(guy common.Address, wad *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Approve(&_SuperchainWETH.TransactOpts, guy, wad)
}

// CrosschainBurn is a paid mutator transaction binding the contract method 0x2b8c49e3.
//
// Solidity: function crosschainBurn(address _from, uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHTransactor) CrosschainBurn(opts *bind.TransactOpts, _from common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.contract.Transact(opts, "crosschainBurn", _from, _amount)
}

// CrosschainBurn is a paid mutator transaction binding the contract method 0x2b8c49e3.
//
// Solidity: function crosschainBurn(address _from, uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHSession) CrosschainBurn(_from common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.CrosschainBurn(&_SuperchainWETH.TransactOpts, _from, _amount)
}

// CrosschainBurn is a paid mutator transaction binding the contract method 0x2b8c49e3.
//
// Solidity: function crosschainBurn(address _from, uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHTransactorSession) CrosschainBurn(_from common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.CrosschainBurn(&_SuperchainWETH.TransactOpts, _from, _amount)
}

// CrosschainMint is a paid mutator transaction binding the contract method 0x18bf5077.
//
// Solidity: function crosschainMint(address _to, uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHTransactor) CrosschainMint(opts *bind.TransactOpts, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.contract.Transact(opts, "crosschainMint", _to, _amount)
}

// CrosschainMint is a paid mutator transaction binding the contract method 0x18bf5077.
//
// Solidity: function crosschainMint(address _to, uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHSession) CrosschainMint(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.CrosschainMint(&_SuperchainWETH.TransactOpts, _to, _amount)
}

// CrosschainMint is a paid mutator transaction binding the contract method 0x18bf5077.
//
// Solidity: function crosschainMint(address _to, uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHTransactorSession) CrosschainMint(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.CrosschainMint(&_SuperchainWETH.TransactOpts, _to, _amount)
}

// Deposit is a paid mutator transaction binding the contract method 0xd0e30db0.
//
// Solidity: function deposit() payable returns()
func (_SuperchainWETH *SuperchainWETHTransactor) Deposit(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SuperchainWETH.contract.Transact(opts, "deposit")
}

// Deposit is a paid mutator transaction binding the contract method 0xd0e30db0.
//
// Solidity: function deposit() payable returns()
func (_SuperchainWETH *SuperchainWETHSession) Deposit() (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Deposit(&_SuperchainWETH.TransactOpts)
}

// Deposit is a paid mutator transaction binding the contract method 0xd0e30db0.
//
// Solidity: function deposit() payable returns()
func (_SuperchainWETH *SuperchainWETHTransactorSession) Deposit() (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Deposit(&_SuperchainWETH.TransactOpts)
}

// RelayETH is a paid mutator transaction binding the contract method 0x4f0edcc9.
//
// Solidity: function relayETH(address _from, address _to, uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHTransactor) RelayETH(opts *bind.TransactOpts, _from common.Address, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.contract.Transact(opts, "relayETH", _from, _to, _amount)
}

// RelayETH is a paid mutator transaction binding the contract method 0x4f0edcc9.
//
// Solidity: function relayETH(address _from, address _to, uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHSession) RelayETH(_from common.Address, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.RelayETH(&_SuperchainWETH.TransactOpts, _from, _to, _amount)
}

// RelayETH is a paid mutator transaction binding the contract method 0x4f0edcc9.
//
// Solidity: function relayETH(address _from, address _to, uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHTransactorSession) RelayETH(_from common.Address, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.RelayETH(&_SuperchainWETH.TransactOpts, _from, _to, _amount)
}

// SendETH is a paid mutator transaction binding the contract method 0x64a197f3.
//
// Solidity: function sendETH(address _to, uint256 _chainId) payable returns(bytes32 msgHash_)
func (_SuperchainWETH *SuperchainWETHTransactor) SendETH(opts *bind.TransactOpts, _to common.Address, _chainId *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.contract.Transact(opts, "sendETH", _to, _chainId)
}

// SendETH is a paid mutator transaction binding the contract method 0x64a197f3.
//
// Solidity: function sendETH(address _to, uint256 _chainId) payable returns(bytes32 msgHash_)
func (_SuperchainWETH *SuperchainWETHSession) SendETH(_to common.Address, _chainId *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.SendETH(&_SuperchainWETH.TransactOpts, _to, _chainId)
}

// SendETH is a paid mutator transaction binding the contract method 0x64a197f3.
//
// Solidity: function sendETH(address _to, uint256 _chainId) payable returns(bytes32 msgHash_)
func (_SuperchainWETH *SuperchainWETHTransactorSession) SendETH(_to common.Address, _chainId *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.SendETH(&_SuperchainWETH.TransactOpts, _to, _chainId)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address dst, uint256 wad) returns(bool)
func (_SuperchainWETH *SuperchainWETHTransactor) Transfer(opts *bind.TransactOpts, dst common.Address, wad *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.contract.Transact(opts, "transfer", dst, wad)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address dst, uint256 wad) returns(bool)
func (_SuperchainWETH *SuperchainWETHSession) Transfer(dst common.Address, wad *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Transfer(&_SuperchainWETH.TransactOpts, dst, wad)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address dst, uint256 wad) returns(bool)
func (_SuperchainWETH *SuperchainWETHTransactorSession) Transfer(dst common.Address, wad *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Transfer(&_SuperchainWETH.TransactOpts, dst, wad)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address src, address dst, uint256 wad) returns(bool)
func (_SuperchainWETH *SuperchainWETHTransactor) TransferFrom(opts *bind.TransactOpts, src common.Address, dst common.Address, wad *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.contract.Transact(opts, "transferFrom", src, dst, wad)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address src, address dst, uint256 wad) returns(bool)
func (_SuperchainWETH *SuperchainWETHSession) TransferFrom(src common.Address, dst common.Address, wad *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.TransferFrom(&_SuperchainWETH.TransactOpts, src, dst, wad)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address src, address dst, uint256 wad) returns(bool)
func (_SuperchainWETH *SuperchainWETHTransactorSession) TransferFrom(src common.Address, dst common.Address, wad *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.TransferFrom(&_SuperchainWETH.TransactOpts, src, dst, wad)
}

// Withdraw is a paid mutator transaction binding the contract method 0x2e1a7d4d.
//
// Solidity: function withdraw(uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHTransactor) Withdraw(opts *bind.TransactOpts, _amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.contract.Transact(opts, "withdraw", _amount)
}

// Withdraw is a paid mutator transaction binding the contract method 0x2e1a7d4d.
//
// Solidity: function withdraw(uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHSession) Withdraw(_amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Withdraw(&_SuperchainWETH.TransactOpts, _amount)
}

// Withdraw is a paid mutator transaction binding the contract method 0x2e1a7d4d.
//
// Solidity: function withdraw(uint256 _amount) returns()
func (_SuperchainWETH *SuperchainWETHTransactorSession) Withdraw(_amount *big.Int) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Withdraw(&_SuperchainWETH.TransactOpts, _amount)
}

// Fallback is a paid mutator transaction binding the contract fallback function.
//
// Solidity: fallback() payable returns()
func (_SuperchainWETH *SuperchainWETHTransactor) Fallback(opts *bind.TransactOpts, calldata []byte) (*types.Transaction, error) {
	return _SuperchainWETH.contract.RawTransact(opts, calldata)
}

// Fallback is a paid mutator transaction binding the contract fallback function.
//
// Solidity: fallback() payable returns()
func (_SuperchainWETH *SuperchainWETHSession) Fallback(calldata []byte) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Fallback(&_SuperchainWETH.TransactOpts, calldata)
}

// Fallback is a paid mutator transaction binding the contract fallback function.
//
// Solidity: fallback() payable returns()
func (_SuperchainWETH *SuperchainWETHTransactorSession) Fallback(calldata []byte) (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Fallback(&_SuperchainWETH.TransactOpts, calldata)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_SuperchainWETH *SuperchainWETHTransactor) Receive(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SuperchainWETH.contract.RawTransact(opts, nil) // calldata is disallowed for receive function
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_SuperchainWETH *SuperchainWETHSession) Receive() (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Receive(&_SuperchainWETH.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_SuperchainWETH *SuperchainWETHTransactorSession) Receive() (*types.Transaction, error) {
	return _SuperchainWETH.Contract.Receive(&_SuperchainWETH.TransactOpts)
}

// SuperchainWETHApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the SuperchainWETH contract.
type SuperchainWETHApprovalIterator struct {
	Event *SuperchainWETHApproval // Event containing the contract specifics and raw log

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
func (it *SuperchainWETHApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SuperchainWETHApproval)
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
		it.Event = new(SuperchainWETHApproval)
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
func (it *SuperchainWETHApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SuperchainWETHApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SuperchainWETHApproval represents a Approval event raised by the SuperchainWETH contract.
type SuperchainWETHApproval struct {
	Src common.Address
	Guy common.Address
	Wad *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed src, address indexed guy, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) FilterApproval(opts *bind.FilterOpts, src []common.Address, guy []common.Address) (*SuperchainWETHApprovalIterator, error) {

	var srcRule []interface{}
	for _, srcItem := range src {
		srcRule = append(srcRule, srcItem)
	}
	var guyRule []interface{}
	for _, guyItem := range guy {
		guyRule = append(guyRule, guyItem)
	}

	logs, sub, err := _SuperchainWETH.contract.FilterLogs(opts, "Approval", srcRule, guyRule)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHApprovalIterator{contract: _SuperchainWETH.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed src, address indexed guy, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *SuperchainWETHApproval, src []common.Address, guy []common.Address) (event.Subscription, error) {

	var srcRule []interface{}
	for _, srcItem := range src {
		srcRule = append(srcRule, srcItem)
	}
	var guyRule []interface{}
	for _, guyItem := range guy {
		guyRule = append(guyRule, guyItem)
	}

	logs, sub, err := _SuperchainWETH.contract.WatchLogs(opts, "Approval", srcRule, guyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SuperchainWETHApproval)
				if err := _SuperchainWETH.contract.UnpackLog(event, "Approval", log); err != nil {
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

// ParseApproval is a log parse operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed src, address indexed guy, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) ParseApproval(log types.Log) (*SuperchainWETHApproval, error) {
	event := new(SuperchainWETHApproval)
	if err := _SuperchainWETH.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SuperchainWETHCrosschainBurnIterator is returned from FilterCrosschainBurn and is used to iterate over the raw logs and unpacked data for CrosschainBurn events raised by the SuperchainWETH contract.
type SuperchainWETHCrosschainBurnIterator struct {
	Event *SuperchainWETHCrosschainBurn // Event containing the contract specifics and raw log

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
func (it *SuperchainWETHCrosschainBurnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SuperchainWETHCrosschainBurn)
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
		it.Event = new(SuperchainWETHCrosschainBurn)
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
func (it *SuperchainWETHCrosschainBurnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SuperchainWETHCrosschainBurnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SuperchainWETHCrosschainBurn represents a CrosschainBurn event raised by the SuperchainWETH contract.
type SuperchainWETHCrosschainBurn struct {
	From   common.Address
	Amount *big.Int
	Sender common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterCrosschainBurn is a free log retrieval operation binding the contract event 0xb90795a66650155983e242cac3e1ac1a4dc26f8ed2987f3ce416a34e00111fd4.
//
// Solidity: event CrosschainBurn(address indexed from, uint256 amount, address indexed sender)
func (_SuperchainWETH *SuperchainWETHFilterer) FilterCrosschainBurn(opts *bind.FilterOpts, from []common.Address, sender []common.Address) (*SuperchainWETHCrosschainBurnIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _SuperchainWETH.contract.FilterLogs(opts, "CrosschainBurn", fromRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHCrosschainBurnIterator{contract: _SuperchainWETH.contract, event: "CrosschainBurn", logs: logs, sub: sub}, nil
}

// WatchCrosschainBurn is a free log subscription operation binding the contract event 0xb90795a66650155983e242cac3e1ac1a4dc26f8ed2987f3ce416a34e00111fd4.
//
// Solidity: event CrosschainBurn(address indexed from, uint256 amount, address indexed sender)
func (_SuperchainWETH *SuperchainWETHFilterer) WatchCrosschainBurn(opts *bind.WatchOpts, sink chan<- *SuperchainWETHCrosschainBurn, from []common.Address, sender []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _SuperchainWETH.contract.WatchLogs(opts, "CrosschainBurn", fromRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SuperchainWETHCrosschainBurn)
				if err := _SuperchainWETH.contract.UnpackLog(event, "CrosschainBurn", log); err != nil {
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

// ParseCrosschainBurn is a log parse operation binding the contract event 0xb90795a66650155983e242cac3e1ac1a4dc26f8ed2987f3ce416a34e00111fd4.
//
// Solidity: event CrosschainBurn(address indexed from, uint256 amount, address indexed sender)
func (_SuperchainWETH *SuperchainWETHFilterer) ParseCrosschainBurn(log types.Log) (*SuperchainWETHCrosschainBurn, error) {
	event := new(SuperchainWETHCrosschainBurn)
	if err := _SuperchainWETH.contract.UnpackLog(event, "CrosschainBurn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SuperchainWETHCrosschainMintIterator is returned from FilterCrosschainMint and is used to iterate over the raw logs and unpacked data for CrosschainMint events raised by the SuperchainWETH contract.
type SuperchainWETHCrosschainMintIterator struct {
	Event *SuperchainWETHCrosschainMint // Event containing the contract specifics and raw log

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
func (it *SuperchainWETHCrosschainMintIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SuperchainWETHCrosschainMint)
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
		it.Event = new(SuperchainWETHCrosschainMint)
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
func (it *SuperchainWETHCrosschainMintIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SuperchainWETHCrosschainMintIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SuperchainWETHCrosschainMint represents a CrosschainMint event raised by the SuperchainWETH contract.
type SuperchainWETHCrosschainMint struct {
	To     common.Address
	Amount *big.Int
	Sender common.Address
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterCrosschainMint is a free log retrieval operation binding the contract event 0xde22baff038e3a3e08407cbdf617deed74e869a7ba517df611e33131c6e6ea04.
//
// Solidity: event CrosschainMint(address indexed to, uint256 amount, address indexed sender)
func (_SuperchainWETH *SuperchainWETHFilterer) FilterCrosschainMint(opts *bind.FilterOpts, to []common.Address, sender []common.Address) (*SuperchainWETHCrosschainMintIterator, error) {

	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _SuperchainWETH.contract.FilterLogs(opts, "CrosschainMint", toRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHCrosschainMintIterator{contract: _SuperchainWETH.contract, event: "CrosschainMint", logs: logs, sub: sub}, nil
}

// WatchCrosschainMint is a free log subscription operation binding the contract event 0xde22baff038e3a3e08407cbdf617deed74e869a7ba517df611e33131c6e6ea04.
//
// Solidity: event CrosschainMint(address indexed to, uint256 amount, address indexed sender)
func (_SuperchainWETH *SuperchainWETHFilterer) WatchCrosschainMint(opts *bind.WatchOpts, sink chan<- *SuperchainWETHCrosschainMint, to []common.Address, sender []common.Address) (event.Subscription, error) {

	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _SuperchainWETH.contract.WatchLogs(opts, "CrosschainMint", toRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SuperchainWETHCrosschainMint)
				if err := _SuperchainWETH.contract.UnpackLog(event, "CrosschainMint", log); err != nil {
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

// ParseCrosschainMint is a log parse operation binding the contract event 0xde22baff038e3a3e08407cbdf617deed74e869a7ba517df611e33131c6e6ea04.
//
// Solidity: event CrosschainMint(address indexed to, uint256 amount, address indexed sender)
func (_SuperchainWETH *SuperchainWETHFilterer) ParseCrosschainMint(log types.Log) (*SuperchainWETHCrosschainMint, error) {
	event := new(SuperchainWETHCrosschainMint)
	if err := _SuperchainWETH.contract.UnpackLog(event, "CrosschainMint", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SuperchainWETHDepositIterator is returned from FilterDeposit and is used to iterate over the raw logs and unpacked data for Deposit events raised by the SuperchainWETH contract.
type SuperchainWETHDepositIterator struct {
	Event *SuperchainWETHDeposit // Event containing the contract specifics and raw log

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
func (it *SuperchainWETHDepositIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SuperchainWETHDeposit)
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
		it.Event = new(SuperchainWETHDeposit)
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
func (it *SuperchainWETHDepositIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SuperchainWETHDepositIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SuperchainWETHDeposit represents a Deposit event raised by the SuperchainWETH contract.
type SuperchainWETHDeposit struct {
	Dst common.Address
	Wad *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterDeposit is a free log retrieval operation binding the contract event 0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c.
//
// Solidity: event Deposit(address indexed dst, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) FilterDeposit(opts *bind.FilterOpts, dst []common.Address) (*SuperchainWETHDepositIterator, error) {

	var dstRule []interface{}
	for _, dstItem := range dst {
		dstRule = append(dstRule, dstItem)
	}

	logs, sub, err := _SuperchainWETH.contract.FilterLogs(opts, "Deposit", dstRule)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHDepositIterator{contract: _SuperchainWETH.contract, event: "Deposit", logs: logs, sub: sub}, nil
}

// WatchDeposit is a free log subscription operation binding the contract event 0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c.
//
// Solidity: event Deposit(address indexed dst, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) WatchDeposit(opts *bind.WatchOpts, sink chan<- *SuperchainWETHDeposit, dst []common.Address) (event.Subscription, error) {

	var dstRule []interface{}
	for _, dstItem := range dst {
		dstRule = append(dstRule, dstItem)
	}

	logs, sub, err := _SuperchainWETH.contract.WatchLogs(opts, "Deposit", dstRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SuperchainWETHDeposit)
				if err := _SuperchainWETH.contract.UnpackLog(event, "Deposit", log); err != nil {
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

// ParseDeposit is a log parse operation binding the contract event 0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c.
//
// Solidity: event Deposit(address indexed dst, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) ParseDeposit(log types.Log) (*SuperchainWETHDeposit, error) {
	event := new(SuperchainWETHDeposit)
	if err := _SuperchainWETH.contract.UnpackLog(event, "Deposit", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SuperchainWETHRelayETHIterator is returned from FilterRelayETH and is used to iterate over the raw logs and unpacked data for RelayETH events raised by the SuperchainWETH contract.
type SuperchainWETHRelayETHIterator struct {
	Event *SuperchainWETHRelayETH // Event containing the contract specifics and raw log

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
func (it *SuperchainWETHRelayETHIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SuperchainWETHRelayETH)
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
		it.Event = new(SuperchainWETHRelayETH)
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
func (it *SuperchainWETHRelayETHIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SuperchainWETHRelayETHIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SuperchainWETHRelayETH represents a RelayETH event raised by the SuperchainWETH contract.
type SuperchainWETHRelayETH struct {
	From   common.Address
	To     common.Address
	Amount *big.Int
	Source *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterRelayETH is a free log retrieval operation binding the contract event 0xe5479bb8ebad3b9ac81f55f424a6289cf0a54ff2641708f41dcb2b26f264d359.
//
// Solidity: event RelayETH(address indexed from, address indexed to, uint256 amount, uint256 source)
func (_SuperchainWETH *SuperchainWETHFilterer) FilterRelayETH(opts *bind.FilterOpts, from []common.Address, to []common.Address) (*SuperchainWETHRelayETHIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SuperchainWETH.contract.FilterLogs(opts, "RelayETH", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHRelayETHIterator{contract: _SuperchainWETH.contract, event: "RelayETH", logs: logs, sub: sub}, nil
}

// WatchRelayETH is a free log subscription operation binding the contract event 0xe5479bb8ebad3b9ac81f55f424a6289cf0a54ff2641708f41dcb2b26f264d359.
//
// Solidity: event RelayETH(address indexed from, address indexed to, uint256 amount, uint256 source)
func (_SuperchainWETH *SuperchainWETHFilterer) WatchRelayETH(opts *bind.WatchOpts, sink chan<- *SuperchainWETHRelayETH, from []common.Address, to []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SuperchainWETH.contract.WatchLogs(opts, "RelayETH", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SuperchainWETHRelayETH)
				if err := _SuperchainWETH.contract.UnpackLog(event, "RelayETH", log); err != nil {
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

// ParseRelayETH is a log parse operation binding the contract event 0xe5479bb8ebad3b9ac81f55f424a6289cf0a54ff2641708f41dcb2b26f264d359.
//
// Solidity: event RelayETH(address indexed from, address indexed to, uint256 amount, uint256 source)
func (_SuperchainWETH *SuperchainWETHFilterer) ParseRelayETH(log types.Log) (*SuperchainWETHRelayETH, error) {
	event := new(SuperchainWETHRelayETH)
	if err := _SuperchainWETH.contract.UnpackLog(event, "RelayETH", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SuperchainWETHSendETHIterator is returned from FilterSendETH and is used to iterate over the raw logs and unpacked data for SendETH events raised by the SuperchainWETH contract.
type SuperchainWETHSendETHIterator struct {
	Event *SuperchainWETHSendETH // Event containing the contract specifics and raw log

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
func (it *SuperchainWETHSendETHIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SuperchainWETHSendETH)
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
		it.Event = new(SuperchainWETHSendETH)
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
func (it *SuperchainWETHSendETHIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SuperchainWETHSendETHIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SuperchainWETHSendETH represents a SendETH event raised by the SuperchainWETH contract.
type SuperchainWETHSendETH struct {
	From        common.Address
	To          common.Address
	Amount      *big.Int
	Destination *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterSendETH is a free log retrieval operation binding the contract event 0xed98a2ff78833375c368471a747cdf0633024dde3f870feb08a934ac5be83402.
//
// Solidity: event SendETH(address indexed from, address indexed to, uint256 amount, uint256 destination)
func (_SuperchainWETH *SuperchainWETHFilterer) FilterSendETH(opts *bind.FilterOpts, from []common.Address, to []common.Address) (*SuperchainWETHSendETHIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SuperchainWETH.contract.FilterLogs(opts, "SendETH", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHSendETHIterator{contract: _SuperchainWETH.contract, event: "SendETH", logs: logs, sub: sub}, nil
}

// WatchSendETH is a free log subscription operation binding the contract event 0xed98a2ff78833375c368471a747cdf0633024dde3f870feb08a934ac5be83402.
//
// Solidity: event SendETH(address indexed from, address indexed to, uint256 amount, uint256 destination)
func (_SuperchainWETH *SuperchainWETHFilterer) WatchSendETH(opts *bind.WatchOpts, sink chan<- *SuperchainWETHSendETH, from []common.Address, to []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SuperchainWETH.contract.WatchLogs(opts, "SendETH", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SuperchainWETHSendETH)
				if err := _SuperchainWETH.contract.UnpackLog(event, "SendETH", log); err != nil {
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

// ParseSendETH is a log parse operation binding the contract event 0xed98a2ff78833375c368471a747cdf0633024dde3f870feb08a934ac5be83402.
//
// Solidity: event SendETH(address indexed from, address indexed to, uint256 amount, uint256 destination)
func (_SuperchainWETH *SuperchainWETHFilterer) ParseSendETH(log types.Log) (*SuperchainWETHSendETH, error) {
	event := new(SuperchainWETHSendETH)
	if err := _SuperchainWETH.contract.UnpackLog(event, "SendETH", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SuperchainWETHTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the SuperchainWETH contract.
type SuperchainWETHTransferIterator struct {
	Event *SuperchainWETHTransfer // Event containing the contract specifics and raw log

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
func (it *SuperchainWETHTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SuperchainWETHTransfer)
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
		it.Event = new(SuperchainWETHTransfer)
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
func (it *SuperchainWETHTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SuperchainWETHTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SuperchainWETHTransfer represents a Transfer event raised by the SuperchainWETH contract.
type SuperchainWETHTransfer struct {
	Src common.Address
	Dst common.Address
	Wad *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed src, address indexed dst, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) FilterTransfer(opts *bind.FilterOpts, src []common.Address, dst []common.Address) (*SuperchainWETHTransferIterator, error) {

	var srcRule []interface{}
	for _, srcItem := range src {
		srcRule = append(srcRule, srcItem)
	}
	var dstRule []interface{}
	for _, dstItem := range dst {
		dstRule = append(dstRule, dstItem)
	}

	logs, sub, err := _SuperchainWETH.contract.FilterLogs(opts, "Transfer", srcRule, dstRule)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHTransferIterator{contract: _SuperchainWETH.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed src, address indexed dst, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *SuperchainWETHTransfer, src []common.Address, dst []common.Address) (event.Subscription, error) {

	var srcRule []interface{}
	for _, srcItem := range src {
		srcRule = append(srcRule, srcItem)
	}
	var dstRule []interface{}
	for _, dstItem := range dst {
		dstRule = append(dstRule, dstItem)
	}

	logs, sub, err := _SuperchainWETH.contract.WatchLogs(opts, "Transfer", srcRule, dstRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SuperchainWETHTransfer)
				if err := _SuperchainWETH.contract.UnpackLog(event, "Transfer", log); err != nil {
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

// ParseTransfer is a log parse operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed src, address indexed dst, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) ParseTransfer(log types.Log) (*SuperchainWETHTransfer, error) {
	event := new(SuperchainWETHTransfer)
	if err := _SuperchainWETH.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SuperchainWETHWithdrawalIterator is returned from FilterWithdrawal and is used to iterate over the raw logs and unpacked data for Withdrawal events raised by the SuperchainWETH contract.
type SuperchainWETHWithdrawalIterator struct {
	Event *SuperchainWETHWithdrawal // Event containing the contract specifics and raw log

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
func (it *SuperchainWETHWithdrawalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SuperchainWETHWithdrawal)
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
		it.Event = new(SuperchainWETHWithdrawal)
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
func (it *SuperchainWETHWithdrawalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SuperchainWETHWithdrawalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SuperchainWETHWithdrawal represents a Withdrawal event raised by the SuperchainWETH contract.
type SuperchainWETHWithdrawal struct {
	Src common.Address
	Wad *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterWithdrawal is a free log retrieval operation binding the contract event 0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65.
//
// Solidity: event Withdrawal(address indexed src, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) FilterWithdrawal(opts *bind.FilterOpts, src []common.Address) (*SuperchainWETHWithdrawalIterator, error) {

	var srcRule []interface{}
	for _, srcItem := range src {
		srcRule = append(srcRule, srcItem)
	}

	logs, sub, err := _SuperchainWETH.contract.FilterLogs(opts, "Withdrawal", srcRule)
	if err != nil {
		return nil, err
	}
	return &SuperchainWETHWithdrawalIterator{contract: _SuperchainWETH.contract, event: "Withdrawal", logs: logs, sub: sub}, nil
}

// WatchWithdrawal is a free log subscription operation binding the contract event 0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65.
//
// Solidity: event Withdrawal(address indexed src, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) WatchWithdrawal(opts *bind.WatchOpts, sink chan<- *SuperchainWETHWithdrawal, src []common.Address) (event.Subscription, error) {

	var srcRule []interface{}
	for _, srcItem := range src {
		srcRule = append(srcRule, srcItem)
	}

	logs, sub, err := _SuperchainWETH.contract.WatchLogs(opts, "Withdrawal", srcRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SuperchainWETHWithdrawal)
				if err := _SuperchainWETH.contract.UnpackLog(event, "Withdrawal", log); err != nil {
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

// ParseWithdrawal is a log parse operation binding the contract event 0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65.
//
// Solidity: event Withdrawal(address indexed src, uint256 wad)
func (_SuperchainWETH *SuperchainWETHFilterer) ParseWithdrawal(log types.Log) (*SuperchainWETHWithdrawal, error) {
	event := new(SuperchainWETHWithdrawal)
	if err := _SuperchainWETH.contract.UnpackLog(event, "Withdrawal", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

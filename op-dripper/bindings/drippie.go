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

// DrippieDripAction is an auto generated low-level Go binding around an user-defined struct.
type DrippieDripAction struct {
	Target common.Address
	Data   []byte
	Value  *big.Int
}

// DrippieDripConfig is an auto generated low-level Go binding around an user-defined struct.
type DrippieDripConfig struct {
	Reentrant   bool
	Interval    *big.Int
	Dripcheck   common.Address
	Checkparams []byte
	Actions     []DrippieDripAction
}

// DrippieMetaData contains all meta data concerning the Drippie contract.
var DrippieMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"_owner\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"receive\",\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"CALL\",\"inputs\":[{\"name\":\"_target\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_data\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"_value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"success_\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"data_\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"DELEGATECALL\",\"inputs\":[{\"name\":\"_target\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[{\"name\":\"success_\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"data_\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"create\",\"inputs\":[{\"name\":\"_name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"_config\",\"type\":\"tuple\",\"internalType\":\"structDrippie.DripConfig\",\"components\":[{\"name\":\"reentrant\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"interval\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"dripcheck\",\"type\":\"address\",\"internalType\":\"contractIDripCheck\"},{\"name\":\"checkparams\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"actions\",\"type\":\"tuple[]\",\"internalType\":\"structDrippie.DripAction[]\",\"components\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"addresspayable\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}]}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"created\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"drip\",\"inputs\":[{\"name\":\"_name\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"drips\",\"inputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumDrippie.DripStatus\"},{\"name\":\"config\",\"type\":\"tuple\",\"internalType\":\"structDrippie.DripConfig\",\"components\":[{\"name\":\"reentrant\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"interval\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"dripcheck\",\"type\":\"address\",\"internalType\":\"contractIDripCheck\"},{\"name\":\"checkparams\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"actions\",\"type\":\"tuple[]\",\"internalType\":\"structDrippie.DripAction[]\",\"components\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"addresspayable\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}]},{\"name\":\"last\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"count\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"executable\",\"inputs\":[{\"name\":\"_name\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getDripCount\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getDripInterval\",\"inputs\":[{\"name\":\"_name\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getDripStatus\",\"inputs\":[{\"name\":\"_name\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"enumDrippie.DripStatus\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"owner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"setOwner\",\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"status\",\"inputs\":[{\"name\":\"_name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"_status\",\"type\":\"uint8\",\"internalType\":\"enumDrippie.DripStatus\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"withdrawERC20\",\"inputs\":[{\"name\":\"_asset\",\"type\":\"address\",\"internalType\":\"contractERC20\"},{\"name\":\"_to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"withdrawERC20\",\"inputs\":[{\"name\":\"_asset\",\"type\":\"address\",\"internalType\":\"contractERC20\"},{\"name\":\"_to\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"withdrawERC721\",\"inputs\":[{\"name\":\"_asset\",\"type\":\"address\",\"internalType\":\"contractERC721\"},{\"name\":\"_to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_id\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"withdrawETH\",\"inputs\":[{\"name\":\"_to\",\"type\":\"address\",\"internalType\":\"addresspayable\"},{\"name\":\"_amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"withdrawETH\",\"inputs\":[{\"name\":\"_to\",\"type\":\"address\",\"internalType\":\"addresspayable\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"DripCreated\",\"inputs\":[{\"name\":\"nameref\",\"type\":\"string\",\"indexed\":true,\"internalType\":\"string\"},{\"name\":\"name\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"config\",\"type\":\"tuple\",\"indexed\":false,\"internalType\":\"structDrippie.DripConfig\",\"components\":[{\"name\":\"reentrant\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"interval\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"dripcheck\",\"type\":\"address\",\"internalType\":\"contractIDripCheck\"},{\"name\":\"checkparams\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"actions\",\"type\":\"tuple[]\",\"internalType\":\"structDrippie.DripAction[]\",\"components\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"addresspayable\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}]}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"DripExecuted\",\"inputs\":[{\"name\":\"nameref\",\"type\":\"string\",\"indexed\":true,\"internalType\":\"string\"},{\"name\":\"name\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"executor\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"DripStatusUpdated\",\"inputs\":[{\"name\":\"nameref\",\"type\":\"string\",\"indexed\":true,\"internalType\":\"string\"},{\"name\":\"name\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"enumDrippie.DripStatus\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"OwnerUpdated\",\"inputs\":[{\"name\":\"user\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"newOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ReceivedETH\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"WithdrewERC20\",\"inputs\":[{\"name\":\"withdrawer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"recipient\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"asset\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"WithdrewERC721\",\"inputs\":[{\"name\":\"withdrawer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"recipient\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"asset\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"id\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"WithdrewETH\",\"inputs\":[{\"name\":\"withdrawer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"recipient\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false}]",
}

// DrippieABI is the input ABI used to generate the binding from.
// Deprecated: Use DrippieMetaData.ABI instead.
var DrippieABI = DrippieMetaData.ABI

// Drippie is an auto generated Go binding around an Ethereum contract.
type Drippie struct {
	DrippieCaller     // Read-only binding to the contract
	DrippieTransactor // Write-only binding to the contract
	DrippieFilterer   // Log filterer for contract events
}

// DrippieCaller is an auto generated read-only Go binding around an Ethereum contract.
type DrippieCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DrippieTransactor is an auto generated write-only Go binding around an Ethereum contract.
type DrippieTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DrippieFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type DrippieFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DrippieSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type DrippieSession struct {
	Contract     *Drippie          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// DrippieCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type DrippieCallerSession struct {
	Contract *DrippieCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// DrippieTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type DrippieTransactorSession struct {
	Contract     *DrippieTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// DrippieRaw is an auto generated low-level Go binding around an Ethereum contract.
type DrippieRaw struct {
	Contract *Drippie // Generic contract binding to access the raw methods on
}

// DrippieCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type DrippieCallerRaw struct {
	Contract *DrippieCaller // Generic read-only contract binding to access the raw methods on
}

// DrippieTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type DrippieTransactorRaw struct {
	Contract *DrippieTransactor // Generic write-only contract binding to access the raw methods on
}

// NewDrippie creates a new instance of Drippie, bound to a specific deployed contract.
func NewDrippie(address common.Address, backend bind.ContractBackend) (*Drippie, error) {
	contract, err := bindDrippie(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Drippie{DrippieCaller: DrippieCaller{contract: contract}, DrippieTransactor: DrippieTransactor{contract: contract}, DrippieFilterer: DrippieFilterer{contract: contract}}, nil
}

// NewDrippieCaller creates a new read-only instance of Drippie, bound to a specific deployed contract.
func NewDrippieCaller(address common.Address, caller bind.ContractCaller) (*DrippieCaller, error) {
	contract, err := bindDrippie(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &DrippieCaller{contract: contract}, nil
}

// NewDrippieTransactor creates a new write-only instance of Drippie, bound to a specific deployed contract.
func NewDrippieTransactor(address common.Address, transactor bind.ContractTransactor) (*DrippieTransactor, error) {
	contract, err := bindDrippie(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &DrippieTransactor{contract: contract}, nil
}

// NewDrippieFilterer creates a new log filterer instance of Drippie, bound to a specific deployed contract.
func NewDrippieFilterer(address common.Address, filterer bind.ContractFilterer) (*DrippieFilterer, error) {
	contract, err := bindDrippie(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &DrippieFilterer{contract: contract}, nil
}

// bindDrippie binds a generic wrapper to an already deployed contract.
func bindDrippie(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(DrippieABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Drippie *DrippieRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Drippie.Contract.DrippieCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Drippie *DrippieRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Drippie.Contract.DrippieTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Drippie *DrippieRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Drippie.Contract.DrippieTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Drippie *DrippieCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Drippie.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Drippie *DrippieTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Drippie.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Drippie *DrippieTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Drippie.Contract.contract.Transact(opts, method, params...)
}

// Created is a free data retrieval call binding the contract method 0x82cb6b72.
//
// Solidity: function created(uint256 ) view returns(string)
func (_Drippie *DrippieCaller) Created(opts *bind.CallOpts, arg0 *big.Int) (string, error) {
	var out []interface{}
	err := _Drippie.contract.Call(opts, &out, "created", arg0)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Created is a free data retrieval call binding the contract method 0x82cb6b72.
//
// Solidity: function created(uint256 ) view returns(string)
func (_Drippie *DrippieSession) Created(arg0 *big.Int) (string, error) {
	return _Drippie.Contract.Created(&_Drippie.CallOpts, arg0)
}

// Created is a free data retrieval call binding the contract method 0x82cb6b72.
//
// Solidity: function created(uint256 ) view returns(string)
func (_Drippie *DrippieCallerSession) Created(arg0 *big.Int) (string, error) {
	return _Drippie.Contract.Created(&_Drippie.CallOpts, arg0)
}

// Drips is a free data retrieval call binding the contract method 0x4d7fba6e.
//
// Solidity: function drips(string ) view returns(uint8 status, (bool,uint256,address,bytes,(address,bytes,uint256)[]) config, uint256 last, uint256 count)
func (_Drippie *DrippieCaller) Drips(opts *bind.CallOpts, arg0 string) (struct {
	Status uint8
	Config DrippieDripConfig
	Last   *big.Int
	Count  *big.Int
}, error) {
	var out []interface{}
	err := _Drippie.contract.Call(opts, &out, "drips", arg0)

	outstruct := new(struct {
		Status uint8
		Config DrippieDripConfig
		Last   *big.Int
		Count  *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Status = *abi.ConvertType(out[0], new(uint8)).(*uint8)
	outstruct.Config = *abi.ConvertType(out[1], new(DrippieDripConfig)).(*DrippieDripConfig)
	outstruct.Last = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.Count = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// Drips is a free data retrieval call binding the contract method 0x4d7fba6e.
//
// Solidity: function drips(string ) view returns(uint8 status, (bool,uint256,address,bytes,(address,bytes,uint256)[]) config, uint256 last, uint256 count)
func (_Drippie *DrippieSession) Drips(arg0 string) (struct {
	Status uint8
	Config DrippieDripConfig
	Last   *big.Int
	Count  *big.Int
}, error) {
	return _Drippie.Contract.Drips(&_Drippie.CallOpts, arg0)
}

// Drips is a free data retrieval call binding the contract method 0x4d7fba6e.
//
// Solidity: function drips(string ) view returns(uint8 status, (bool,uint256,address,bytes,(address,bytes,uint256)[]) config, uint256 last, uint256 count)
func (_Drippie *DrippieCallerSession) Drips(arg0 string) (struct {
	Status uint8
	Config DrippieDripConfig
	Last   *big.Int
	Count  *big.Int
}, error) {
	return _Drippie.Contract.Drips(&_Drippie.CallOpts, arg0)
}

// Executable is a free data retrieval call binding the contract method 0xfc3e3eba.
//
// Solidity: function executable(string _name) view returns(bool)
func (_Drippie *DrippieCaller) Executable(opts *bind.CallOpts, _name string) (bool, error) {
	var out []interface{}
	err := _Drippie.contract.Call(opts, &out, "executable", _name)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Executable is a free data retrieval call binding the contract method 0xfc3e3eba.
//
// Solidity: function executable(string _name) view returns(bool)
func (_Drippie *DrippieSession) Executable(_name string) (bool, error) {
	return _Drippie.Contract.Executable(&_Drippie.CallOpts, _name)
}

// Executable is a free data retrieval call binding the contract method 0xfc3e3eba.
//
// Solidity: function executable(string _name) view returns(bool)
func (_Drippie *DrippieCallerSession) Executable(_name string) (bool, error) {
	return _Drippie.Contract.Executable(&_Drippie.CallOpts, _name)
}

// GetDripCount is a free data retrieval call binding the contract method 0xf1d42b47.
//
// Solidity: function getDripCount() view returns(uint256)
func (_Drippie *DrippieCaller) GetDripCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Drippie.contract.Call(opts, &out, "getDripCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetDripCount is a free data retrieval call binding the contract method 0xf1d42b47.
//
// Solidity: function getDripCount() view returns(uint256)
func (_Drippie *DrippieSession) GetDripCount() (*big.Int, error) {
	return _Drippie.Contract.GetDripCount(&_Drippie.CallOpts)
}

// GetDripCount is a free data retrieval call binding the contract method 0xf1d42b47.
//
// Solidity: function getDripCount() view returns(uint256)
func (_Drippie *DrippieCallerSession) GetDripCount() (*big.Int, error) {
	return _Drippie.Contract.GetDripCount(&_Drippie.CallOpts)
}

// GetDripInterval is a free data retrieval call binding the contract method 0x90547c14.
//
// Solidity: function getDripInterval(string _name) view returns(uint256)
func (_Drippie *DrippieCaller) GetDripInterval(opts *bind.CallOpts, _name string) (*big.Int, error) {
	var out []interface{}
	err := _Drippie.contract.Call(opts, &out, "getDripInterval", _name)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetDripInterval is a free data retrieval call binding the contract method 0x90547c14.
//
// Solidity: function getDripInterval(string _name) view returns(uint256)
func (_Drippie *DrippieSession) GetDripInterval(_name string) (*big.Int, error) {
	return _Drippie.Contract.GetDripInterval(&_Drippie.CallOpts, _name)
}

// GetDripInterval is a free data retrieval call binding the contract method 0x90547c14.
//
// Solidity: function getDripInterval(string _name) view returns(uint256)
func (_Drippie *DrippieCallerSession) GetDripInterval(_name string) (*big.Int, error) {
	return _Drippie.Contract.GetDripInterval(&_Drippie.CallOpts, _name)
}

// GetDripStatus is a free data retrieval call binding the contract method 0x0d8f4697.
//
// Solidity: function getDripStatus(string _name) view returns(uint8)
func (_Drippie *DrippieCaller) GetDripStatus(opts *bind.CallOpts, _name string) (uint8, error) {
	var out []interface{}
	err := _Drippie.contract.Call(opts, &out, "getDripStatus", _name)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetDripStatus is a free data retrieval call binding the contract method 0x0d8f4697.
//
// Solidity: function getDripStatus(string _name) view returns(uint8)
func (_Drippie *DrippieSession) GetDripStatus(_name string) (uint8, error) {
	return _Drippie.Contract.GetDripStatus(&_Drippie.CallOpts, _name)
}

// GetDripStatus is a free data retrieval call binding the contract method 0x0d8f4697.
//
// Solidity: function getDripStatus(string _name) view returns(uint8)
func (_Drippie *DrippieCallerSession) GetDripStatus(_name string) (uint8, error) {
	return _Drippie.Contract.GetDripStatus(&_Drippie.CallOpts, _name)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Drippie *DrippieCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Drippie.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Drippie *DrippieSession) Owner() (common.Address, error) {
	return _Drippie.Contract.Owner(&_Drippie.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Drippie *DrippieCallerSession) Owner() (common.Address, error) {
	return _Drippie.Contract.Owner(&_Drippie.CallOpts)
}

// CALL is a paid mutator transaction binding the contract method 0x6e2d44ae.
//
// Solidity: function CALL(address _target, bytes _data, uint256 _value) payable returns(bool success_, bytes data_)
func (_Drippie *DrippieTransactor) CALL(opts *bind.TransactOpts, _target common.Address, _data []byte, _value *big.Int) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "CALL", _target, _data, _value)
}

// CALL is a paid mutator transaction binding the contract method 0x6e2d44ae.
//
// Solidity: function CALL(address _target, bytes _data, uint256 _value) payable returns(bool success_, bytes data_)
func (_Drippie *DrippieSession) CALL(_target common.Address, _data []byte, _value *big.Int) (*types.Transaction, error) {
	return _Drippie.Contract.CALL(&_Drippie.TransactOpts, _target, _data, _value)
}

// CALL is a paid mutator transaction binding the contract method 0x6e2d44ae.
//
// Solidity: function CALL(address _target, bytes _data, uint256 _value) payable returns(bool success_, bytes data_)
func (_Drippie *DrippieTransactorSession) CALL(_target common.Address, _data []byte, _value *big.Int) (*types.Transaction, error) {
	return _Drippie.Contract.CALL(&_Drippie.TransactOpts, _target, _data, _value)
}

// DELEGATECALL is a paid mutator transaction binding the contract method 0xedee6239.
//
// Solidity: function DELEGATECALL(address _target, bytes _data) payable returns(bool success_, bytes data_)
func (_Drippie *DrippieTransactor) DELEGATECALL(opts *bind.TransactOpts, _target common.Address, _data []byte) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "DELEGATECALL", _target, _data)
}

// DELEGATECALL is a paid mutator transaction binding the contract method 0xedee6239.
//
// Solidity: function DELEGATECALL(address _target, bytes _data) payable returns(bool success_, bytes data_)
func (_Drippie *DrippieSession) DELEGATECALL(_target common.Address, _data []byte) (*types.Transaction, error) {
	return _Drippie.Contract.DELEGATECALL(&_Drippie.TransactOpts, _target, _data)
}

// DELEGATECALL is a paid mutator transaction binding the contract method 0xedee6239.
//
// Solidity: function DELEGATECALL(address _target, bytes _data) payable returns(bool success_, bytes data_)
func (_Drippie *DrippieTransactorSession) DELEGATECALL(_target common.Address, _data []byte) (*types.Transaction, error) {
	return _Drippie.Contract.DELEGATECALL(&_Drippie.TransactOpts, _target, _data)
}

// Create is a paid mutator transaction binding the contract method 0xe551cdaa.
//
// Solidity: function create(string _name, (bool,uint256,address,bytes,(address,bytes,uint256)[]) _config) returns()
func (_Drippie *DrippieTransactor) Create(opts *bind.TransactOpts, _name string, _config DrippieDripConfig) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "create", _name, _config)
}

// Create is a paid mutator transaction binding the contract method 0xe551cdaa.
//
// Solidity: function create(string _name, (bool,uint256,address,bytes,(address,bytes,uint256)[]) _config) returns()
func (_Drippie *DrippieSession) Create(_name string, _config DrippieDripConfig) (*types.Transaction, error) {
	return _Drippie.Contract.Create(&_Drippie.TransactOpts, _name, _config)
}

// Create is a paid mutator transaction binding the contract method 0xe551cdaa.
//
// Solidity: function create(string _name, (bool,uint256,address,bytes,(address,bytes,uint256)[]) _config) returns()
func (_Drippie *DrippieTransactorSession) Create(_name string, _config DrippieDripConfig) (*types.Transaction, error) {
	return _Drippie.Contract.Create(&_Drippie.TransactOpts, _name, _config)
}

// Drip is a paid mutator transaction binding the contract method 0x67148cd2.
//
// Solidity: function drip(string _name) returns()
func (_Drippie *DrippieTransactor) Drip(opts *bind.TransactOpts, _name string) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "drip", _name)
}

// Drip is a paid mutator transaction binding the contract method 0x67148cd2.
//
// Solidity: function drip(string _name) returns()
func (_Drippie *DrippieSession) Drip(_name string) (*types.Transaction, error) {
	return _Drippie.Contract.Drip(&_Drippie.TransactOpts, _name)
}

// Drip is a paid mutator transaction binding the contract method 0x67148cd2.
//
// Solidity: function drip(string _name) returns()
func (_Drippie *DrippieTransactorSession) Drip(_name string) (*types.Transaction, error) {
	return _Drippie.Contract.Drip(&_Drippie.TransactOpts, _name)
}

// SetOwner is a paid mutator transaction binding the contract method 0x13af4035.
//
// Solidity: function setOwner(address newOwner) returns()
func (_Drippie *DrippieTransactor) SetOwner(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "setOwner", newOwner)
}

// SetOwner is a paid mutator transaction binding the contract method 0x13af4035.
//
// Solidity: function setOwner(address newOwner) returns()
func (_Drippie *DrippieSession) SetOwner(newOwner common.Address) (*types.Transaction, error) {
	return _Drippie.Contract.SetOwner(&_Drippie.TransactOpts, newOwner)
}

// SetOwner is a paid mutator transaction binding the contract method 0x13af4035.
//
// Solidity: function setOwner(address newOwner) returns()
func (_Drippie *DrippieTransactorSession) SetOwner(newOwner common.Address) (*types.Transaction, error) {
	return _Drippie.Contract.SetOwner(&_Drippie.TransactOpts, newOwner)
}

// Status is a paid mutator transaction binding the contract method 0x9bc94d01.
//
// Solidity: function status(string _name, uint8 _status) returns()
func (_Drippie *DrippieTransactor) Status(opts *bind.TransactOpts, _name string, _status uint8) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "status", _name, _status)
}

// Status is a paid mutator transaction binding the contract method 0x9bc94d01.
//
// Solidity: function status(string _name, uint8 _status) returns()
func (_Drippie *DrippieSession) Status(_name string, _status uint8) (*types.Transaction, error) {
	return _Drippie.Contract.Status(&_Drippie.TransactOpts, _name, _status)
}

// Status is a paid mutator transaction binding the contract method 0x9bc94d01.
//
// Solidity: function status(string _name, uint8 _status) returns()
func (_Drippie *DrippieTransactorSession) Status(_name string, _status uint8) (*types.Transaction, error) {
	return _Drippie.Contract.Status(&_Drippie.TransactOpts, _name, _status)
}

// WithdrawERC20 is a paid mutator transaction binding the contract method 0x44004cc1.
//
// Solidity: function withdrawERC20(address _asset, address _to, uint256 _amount) returns()
func (_Drippie *DrippieTransactor) WithdrawERC20(opts *bind.TransactOpts, _asset common.Address, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "withdrawERC20", _asset, _to, _amount)
}

// WithdrawERC20 is a paid mutator transaction binding the contract method 0x44004cc1.
//
// Solidity: function withdrawERC20(address _asset, address _to, uint256 _amount) returns()
func (_Drippie *DrippieSession) WithdrawERC20(_asset common.Address, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawERC20(&_Drippie.TransactOpts, _asset, _to, _amount)
}

// WithdrawERC20 is a paid mutator transaction binding the contract method 0x44004cc1.
//
// Solidity: function withdrawERC20(address _asset, address _to, uint256 _amount) returns()
func (_Drippie *DrippieTransactorSession) WithdrawERC20(_asset common.Address, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawERC20(&_Drippie.TransactOpts, _asset, _to, _amount)
}

// WithdrawERC200 is a paid mutator transaction binding the contract method 0x9456fbcc.
//
// Solidity: function withdrawERC20(address _asset, address _to) returns()
func (_Drippie *DrippieTransactor) WithdrawERC200(opts *bind.TransactOpts, _asset common.Address, _to common.Address) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "withdrawERC200", _asset, _to)
}

// WithdrawERC200 is a paid mutator transaction binding the contract method 0x9456fbcc.
//
// Solidity: function withdrawERC20(address _asset, address _to) returns()
func (_Drippie *DrippieSession) WithdrawERC200(_asset common.Address, _to common.Address) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawERC200(&_Drippie.TransactOpts, _asset, _to)
}

// WithdrawERC200 is a paid mutator transaction binding the contract method 0x9456fbcc.
//
// Solidity: function withdrawERC20(address _asset, address _to) returns()
func (_Drippie *DrippieTransactorSession) WithdrawERC200(_asset common.Address, _to common.Address) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawERC200(&_Drippie.TransactOpts, _asset, _to)
}

// WithdrawERC721 is a paid mutator transaction binding the contract method 0x4025feb2.
//
// Solidity: function withdrawERC721(address _asset, address _to, uint256 _id) returns()
func (_Drippie *DrippieTransactor) WithdrawERC721(opts *bind.TransactOpts, _asset common.Address, _to common.Address, _id *big.Int) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "withdrawERC721", _asset, _to, _id)
}

// WithdrawERC721 is a paid mutator transaction binding the contract method 0x4025feb2.
//
// Solidity: function withdrawERC721(address _asset, address _to, uint256 _id) returns()
func (_Drippie *DrippieSession) WithdrawERC721(_asset common.Address, _to common.Address, _id *big.Int) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawERC721(&_Drippie.TransactOpts, _asset, _to, _id)
}

// WithdrawERC721 is a paid mutator transaction binding the contract method 0x4025feb2.
//
// Solidity: function withdrawERC721(address _asset, address _to, uint256 _id) returns()
func (_Drippie *DrippieTransactorSession) WithdrawERC721(_asset common.Address, _to common.Address, _id *big.Int) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawERC721(&_Drippie.TransactOpts, _asset, _to, _id)
}

// WithdrawETH is a paid mutator transaction binding the contract method 0x4782f779.
//
// Solidity: function withdrawETH(address _to, uint256 _amount) returns()
func (_Drippie *DrippieTransactor) WithdrawETH(opts *bind.TransactOpts, _to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "withdrawETH", _to, _amount)
}

// WithdrawETH is a paid mutator transaction binding the contract method 0x4782f779.
//
// Solidity: function withdrawETH(address _to, uint256 _amount) returns()
func (_Drippie *DrippieSession) WithdrawETH(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawETH(&_Drippie.TransactOpts, _to, _amount)
}

// WithdrawETH is a paid mutator transaction binding the contract method 0x4782f779.
//
// Solidity: function withdrawETH(address _to, uint256 _amount) returns()
func (_Drippie *DrippieTransactorSession) WithdrawETH(_to common.Address, _amount *big.Int) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawETH(&_Drippie.TransactOpts, _to, _amount)
}

// WithdrawETH0 is a paid mutator transaction binding the contract method 0x690d8320.
//
// Solidity: function withdrawETH(address _to) returns()
func (_Drippie *DrippieTransactor) WithdrawETH0(opts *bind.TransactOpts, _to common.Address) (*types.Transaction, error) {
	return _Drippie.contract.Transact(opts, "withdrawETH0", _to)
}

// WithdrawETH0 is a paid mutator transaction binding the contract method 0x690d8320.
//
// Solidity: function withdrawETH(address _to) returns()
func (_Drippie *DrippieSession) WithdrawETH0(_to common.Address) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawETH0(&_Drippie.TransactOpts, _to)
}

// WithdrawETH0 is a paid mutator transaction binding the contract method 0x690d8320.
//
// Solidity: function withdrawETH(address _to) returns()
func (_Drippie *DrippieTransactorSession) WithdrawETH0(_to common.Address) (*types.Transaction, error) {
	return _Drippie.Contract.WithdrawETH0(&_Drippie.TransactOpts, _to)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_Drippie *DrippieTransactor) Receive(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Drippie.contract.RawTransact(opts, nil) // calldata is disallowed for receive function
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_Drippie *DrippieSession) Receive() (*types.Transaction, error) {
	return _Drippie.Contract.Receive(&_Drippie.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_Drippie *DrippieTransactorSession) Receive() (*types.Transaction, error) {
	return _Drippie.Contract.Receive(&_Drippie.TransactOpts)
}

// DrippieDripCreatedIterator is returned from FilterDripCreated and is used to iterate over the raw logs and unpacked data for DripCreated events raised by the Drippie contract.
type DrippieDripCreatedIterator struct {
	Event *DrippieDripCreated // Event containing the contract specifics and raw log

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
func (it *DrippieDripCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DrippieDripCreated)
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
		it.Event = new(DrippieDripCreated)
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
func (it *DrippieDripCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DrippieDripCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DrippieDripCreated represents a DripCreated event raised by the Drippie contract.
type DrippieDripCreated struct {
	Nameref common.Hash
	Name    string
	Config  DrippieDripConfig
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterDripCreated is a free log retrieval operation binding the contract event 0xe38d8d98e6cc66f6f520d483c6c5a89289681f897799c4c29d767cf57e76d9a6.
//
// Solidity: event DripCreated(string indexed nameref, string name, (bool,uint256,address,bytes,(address,bytes,uint256)[]) config)
func (_Drippie *DrippieFilterer) FilterDripCreated(opts *bind.FilterOpts, nameref []string) (*DrippieDripCreatedIterator, error) {

	var namerefRule []interface{}
	for _, namerefItem := range nameref {
		namerefRule = append(namerefRule, namerefItem)
	}

	logs, sub, err := _Drippie.contract.FilterLogs(opts, "DripCreated", namerefRule)
	if err != nil {
		return nil, err
	}
	return &DrippieDripCreatedIterator{contract: _Drippie.contract, event: "DripCreated", logs: logs, sub: sub}, nil
}

// WatchDripCreated is a free log subscription operation binding the contract event 0xe38d8d98e6cc66f6f520d483c6c5a89289681f897799c4c29d767cf57e76d9a6.
//
// Solidity: event DripCreated(string indexed nameref, string name, (bool,uint256,address,bytes,(address,bytes,uint256)[]) config)
func (_Drippie *DrippieFilterer) WatchDripCreated(opts *bind.WatchOpts, sink chan<- *DrippieDripCreated, nameref []string) (event.Subscription, error) {

	var namerefRule []interface{}
	for _, namerefItem := range nameref {
		namerefRule = append(namerefRule, namerefItem)
	}

	logs, sub, err := _Drippie.contract.WatchLogs(opts, "DripCreated", namerefRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DrippieDripCreated)
				if err := _Drippie.contract.UnpackLog(event, "DripCreated", log); err != nil {
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

// ParseDripCreated is a log parse operation binding the contract event 0xe38d8d98e6cc66f6f520d483c6c5a89289681f897799c4c29d767cf57e76d9a6.
//
// Solidity: event DripCreated(string indexed nameref, string name, (bool,uint256,address,bytes,(address,bytes,uint256)[]) config)
func (_Drippie *DrippieFilterer) ParseDripCreated(log types.Log) (*DrippieDripCreated, error) {
	event := new(DrippieDripCreated)
	if err := _Drippie.contract.UnpackLog(event, "DripCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DrippieDripExecutedIterator is returned from FilterDripExecuted and is used to iterate over the raw logs and unpacked data for DripExecuted events raised by the Drippie contract.
type DrippieDripExecutedIterator struct {
	Event *DrippieDripExecuted // Event containing the contract specifics and raw log

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
func (it *DrippieDripExecutedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DrippieDripExecuted)
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
		it.Event = new(DrippieDripExecuted)
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
func (it *DrippieDripExecutedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DrippieDripExecutedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DrippieDripExecuted represents a DripExecuted event raised by the Drippie contract.
type DrippieDripExecuted struct {
	Nameref   common.Hash
	Name      string
	Executor  common.Address
	Timestamp *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterDripExecuted is a free log retrieval operation binding the contract event 0xea21435419aad9c54a9d90e2522b6f60bd566401f36fcef661f5f5a28cc0d2c6.
//
// Solidity: event DripExecuted(string indexed nameref, string name, address executor, uint256 timestamp)
func (_Drippie *DrippieFilterer) FilterDripExecuted(opts *bind.FilterOpts, nameref []string) (*DrippieDripExecutedIterator, error) {

	var namerefRule []interface{}
	for _, namerefItem := range nameref {
		namerefRule = append(namerefRule, namerefItem)
	}

	logs, sub, err := _Drippie.contract.FilterLogs(opts, "DripExecuted", namerefRule)
	if err != nil {
		return nil, err
	}
	return &DrippieDripExecutedIterator{contract: _Drippie.contract, event: "DripExecuted", logs: logs, sub: sub}, nil
}

// WatchDripExecuted is a free log subscription operation binding the contract event 0xea21435419aad9c54a9d90e2522b6f60bd566401f36fcef661f5f5a28cc0d2c6.
//
// Solidity: event DripExecuted(string indexed nameref, string name, address executor, uint256 timestamp)
func (_Drippie *DrippieFilterer) WatchDripExecuted(opts *bind.WatchOpts, sink chan<- *DrippieDripExecuted, nameref []string) (event.Subscription, error) {

	var namerefRule []interface{}
	for _, namerefItem := range nameref {
		namerefRule = append(namerefRule, namerefItem)
	}

	logs, sub, err := _Drippie.contract.WatchLogs(opts, "DripExecuted", namerefRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DrippieDripExecuted)
				if err := _Drippie.contract.UnpackLog(event, "DripExecuted", log); err != nil {
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

// ParseDripExecuted is a log parse operation binding the contract event 0xea21435419aad9c54a9d90e2522b6f60bd566401f36fcef661f5f5a28cc0d2c6.
//
// Solidity: event DripExecuted(string indexed nameref, string name, address executor, uint256 timestamp)
func (_Drippie *DrippieFilterer) ParseDripExecuted(log types.Log) (*DrippieDripExecuted, error) {
	event := new(DrippieDripExecuted)
	if err := _Drippie.contract.UnpackLog(event, "DripExecuted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DrippieDripStatusUpdatedIterator is returned from FilterDripStatusUpdated and is used to iterate over the raw logs and unpacked data for DripStatusUpdated events raised by the Drippie contract.
type DrippieDripStatusUpdatedIterator struct {
	Event *DrippieDripStatusUpdated // Event containing the contract specifics and raw log

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
func (it *DrippieDripStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DrippieDripStatusUpdated)
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
		it.Event = new(DrippieDripStatusUpdated)
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
func (it *DrippieDripStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DrippieDripStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DrippieDripStatusUpdated represents a DripStatusUpdated event raised by the Drippie contract.
type DrippieDripStatusUpdated struct {
	Nameref common.Hash
	Name    string
	Status  uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterDripStatusUpdated is a free log retrieval operation binding the contract event 0x407cb3ad05e60ec498fb39417c7a4f6b82d5ba80f82fe512a37b02c93181a2a1.
//
// Solidity: event DripStatusUpdated(string indexed nameref, string name, uint8 status)
func (_Drippie *DrippieFilterer) FilterDripStatusUpdated(opts *bind.FilterOpts, nameref []string) (*DrippieDripStatusUpdatedIterator, error) {

	var namerefRule []interface{}
	for _, namerefItem := range nameref {
		namerefRule = append(namerefRule, namerefItem)
	}

	logs, sub, err := _Drippie.contract.FilterLogs(opts, "DripStatusUpdated", namerefRule)
	if err != nil {
		return nil, err
	}
	return &DrippieDripStatusUpdatedIterator{contract: _Drippie.contract, event: "DripStatusUpdated", logs: logs, sub: sub}, nil
}

// WatchDripStatusUpdated is a free log subscription operation binding the contract event 0x407cb3ad05e60ec498fb39417c7a4f6b82d5ba80f82fe512a37b02c93181a2a1.
//
// Solidity: event DripStatusUpdated(string indexed nameref, string name, uint8 status)
func (_Drippie *DrippieFilterer) WatchDripStatusUpdated(opts *bind.WatchOpts, sink chan<- *DrippieDripStatusUpdated, nameref []string) (event.Subscription, error) {

	var namerefRule []interface{}
	for _, namerefItem := range nameref {
		namerefRule = append(namerefRule, namerefItem)
	}

	logs, sub, err := _Drippie.contract.WatchLogs(opts, "DripStatusUpdated", namerefRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DrippieDripStatusUpdated)
				if err := _Drippie.contract.UnpackLog(event, "DripStatusUpdated", log); err != nil {
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

// ParseDripStatusUpdated is a log parse operation binding the contract event 0x407cb3ad05e60ec498fb39417c7a4f6b82d5ba80f82fe512a37b02c93181a2a1.
//
// Solidity: event DripStatusUpdated(string indexed nameref, string name, uint8 status)
func (_Drippie *DrippieFilterer) ParseDripStatusUpdated(log types.Log) (*DrippieDripStatusUpdated, error) {
	event := new(DrippieDripStatusUpdated)
	if err := _Drippie.contract.UnpackLog(event, "DripStatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DrippieOwnerUpdatedIterator is returned from FilterOwnerUpdated and is used to iterate over the raw logs and unpacked data for OwnerUpdated events raised by the Drippie contract.
type DrippieOwnerUpdatedIterator struct {
	Event *DrippieOwnerUpdated // Event containing the contract specifics and raw log

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
func (it *DrippieOwnerUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DrippieOwnerUpdated)
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
		it.Event = new(DrippieOwnerUpdated)
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
func (it *DrippieOwnerUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DrippieOwnerUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DrippieOwnerUpdated represents a OwnerUpdated event raised by the Drippie contract.
type DrippieOwnerUpdated struct {
	User     common.Address
	NewOwner common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterOwnerUpdated is a free log retrieval operation binding the contract event 0x8292fce18fa69edf4db7b94ea2e58241df0ae57f97e0a6c9b29067028bf92d76.
//
// Solidity: event OwnerUpdated(address indexed user, address indexed newOwner)
func (_Drippie *DrippieFilterer) FilterOwnerUpdated(opts *bind.FilterOpts, user []common.Address, newOwner []common.Address) (*DrippieOwnerUpdatedIterator, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Drippie.contract.FilterLogs(opts, "OwnerUpdated", userRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &DrippieOwnerUpdatedIterator{contract: _Drippie.contract, event: "OwnerUpdated", logs: logs, sub: sub}, nil
}

// WatchOwnerUpdated is a free log subscription operation binding the contract event 0x8292fce18fa69edf4db7b94ea2e58241df0ae57f97e0a6c9b29067028bf92d76.
//
// Solidity: event OwnerUpdated(address indexed user, address indexed newOwner)
func (_Drippie *DrippieFilterer) WatchOwnerUpdated(opts *bind.WatchOpts, sink chan<- *DrippieOwnerUpdated, user []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var userRule []interface{}
	for _, userItem := range user {
		userRule = append(userRule, userItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Drippie.contract.WatchLogs(opts, "OwnerUpdated", userRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DrippieOwnerUpdated)
				if err := _Drippie.contract.UnpackLog(event, "OwnerUpdated", log); err != nil {
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

// ParseOwnerUpdated is a log parse operation binding the contract event 0x8292fce18fa69edf4db7b94ea2e58241df0ae57f97e0a6c9b29067028bf92d76.
//
// Solidity: event OwnerUpdated(address indexed user, address indexed newOwner)
func (_Drippie *DrippieFilterer) ParseOwnerUpdated(log types.Log) (*DrippieOwnerUpdated, error) {
	event := new(DrippieOwnerUpdated)
	if err := _Drippie.contract.UnpackLog(event, "OwnerUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DrippieReceivedETHIterator is returned from FilterReceivedETH and is used to iterate over the raw logs and unpacked data for ReceivedETH events raised by the Drippie contract.
type DrippieReceivedETHIterator struct {
	Event *DrippieReceivedETH // Event containing the contract specifics and raw log

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
func (it *DrippieReceivedETHIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DrippieReceivedETH)
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
		it.Event = new(DrippieReceivedETH)
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
func (it *DrippieReceivedETHIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DrippieReceivedETHIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DrippieReceivedETH represents a ReceivedETH event raised by the Drippie contract.
type DrippieReceivedETH struct {
	From   common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterReceivedETH is a free log retrieval operation binding the contract event 0x4103257eaac983ca79a70d28f90dfc4fa16b619bb0c17ee7cab0d4034c279624.
//
// Solidity: event ReceivedETH(address indexed from, uint256 amount)
func (_Drippie *DrippieFilterer) FilterReceivedETH(opts *bind.FilterOpts, from []common.Address) (*DrippieReceivedETHIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _Drippie.contract.FilterLogs(opts, "ReceivedETH", fromRule)
	if err != nil {
		return nil, err
	}
	return &DrippieReceivedETHIterator{contract: _Drippie.contract, event: "ReceivedETH", logs: logs, sub: sub}, nil
}

// WatchReceivedETH is a free log subscription operation binding the contract event 0x4103257eaac983ca79a70d28f90dfc4fa16b619bb0c17ee7cab0d4034c279624.
//
// Solidity: event ReceivedETH(address indexed from, uint256 amount)
func (_Drippie *DrippieFilterer) WatchReceivedETH(opts *bind.WatchOpts, sink chan<- *DrippieReceivedETH, from []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _Drippie.contract.WatchLogs(opts, "ReceivedETH", fromRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DrippieReceivedETH)
				if err := _Drippie.contract.UnpackLog(event, "ReceivedETH", log); err != nil {
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

// ParseReceivedETH is a log parse operation binding the contract event 0x4103257eaac983ca79a70d28f90dfc4fa16b619bb0c17ee7cab0d4034c279624.
//
// Solidity: event ReceivedETH(address indexed from, uint256 amount)
func (_Drippie *DrippieFilterer) ParseReceivedETH(log types.Log) (*DrippieReceivedETH, error) {
	event := new(DrippieReceivedETH)
	if err := _Drippie.contract.UnpackLog(event, "ReceivedETH", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DrippieWithdrewERC20Iterator is returned from FilterWithdrewERC20 and is used to iterate over the raw logs and unpacked data for WithdrewERC20 events raised by the Drippie contract.
type DrippieWithdrewERC20Iterator struct {
	Event *DrippieWithdrewERC20 // Event containing the contract specifics and raw log

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
func (it *DrippieWithdrewERC20Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DrippieWithdrewERC20)
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
		it.Event = new(DrippieWithdrewERC20)
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
func (it *DrippieWithdrewERC20Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DrippieWithdrewERC20Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DrippieWithdrewERC20 represents a WithdrewERC20 event raised by the Drippie contract.
type DrippieWithdrewERC20 struct {
	Withdrawer common.Address
	Recipient  common.Address
	Asset      common.Address
	Amount     *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterWithdrewERC20 is a free log retrieval operation binding the contract event 0x6b00f1c7883f053ba83e907fd1965b22fffe3c4111383e725f04638a566cdbfa.
//
// Solidity: event WithdrewERC20(address indexed withdrawer, address indexed recipient, address indexed asset, uint256 amount)
func (_Drippie *DrippieFilterer) FilterWithdrewERC20(opts *bind.FilterOpts, withdrawer []common.Address, recipient []common.Address, asset []common.Address) (*DrippieWithdrewERC20Iterator, error) {

	var withdrawerRule []interface{}
	for _, withdrawerItem := range withdrawer {
		withdrawerRule = append(withdrawerRule, withdrawerItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}
	var assetRule []interface{}
	for _, assetItem := range asset {
		assetRule = append(assetRule, assetItem)
	}

	logs, sub, err := _Drippie.contract.FilterLogs(opts, "WithdrewERC20", withdrawerRule, recipientRule, assetRule)
	if err != nil {
		return nil, err
	}
	return &DrippieWithdrewERC20Iterator{contract: _Drippie.contract, event: "WithdrewERC20", logs: logs, sub: sub}, nil
}

// WatchWithdrewERC20 is a free log subscription operation binding the contract event 0x6b00f1c7883f053ba83e907fd1965b22fffe3c4111383e725f04638a566cdbfa.
//
// Solidity: event WithdrewERC20(address indexed withdrawer, address indexed recipient, address indexed asset, uint256 amount)
func (_Drippie *DrippieFilterer) WatchWithdrewERC20(opts *bind.WatchOpts, sink chan<- *DrippieWithdrewERC20, withdrawer []common.Address, recipient []common.Address, asset []common.Address) (event.Subscription, error) {

	var withdrawerRule []interface{}
	for _, withdrawerItem := range withdrawer {
		withdrawerRule = append(withdrawerRule, withdrawerItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}
	var assetRule []interface{}
	for _, assetItem := range asset {
		assetRule = append(assetRule, assetItem)
	}

	logs, sub, err := _Drippie.contract.WatchLogs(opts, "WithdrewERC20", withdrawerRule, recipientRule, assetRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DrippieWithdrewERC20)
				if err := _Drippie.contract.UnpackLog(event, "WithdrewERC20", log); err != nil {
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

// ParseWithdrewERC20 is a log parse operation binding the contract event 0x6b00f1c7883f053ba83e907fd1965b22fffe3c4111383e725f04638a566cdbfa.
//
// Solidity: event WithdrewERC20(address indexed withdrawer, address indexed recipient, address indexed asset, uint256 amount)
func (_Drippie *DrippieFilterer) ParseWithdrewERC20(log types.Log) (*DrippieWithdrewERC20, error) {
	event := new(DrippieWithdrewERC20)
	if err := _Drippie.contract.UnpackLog(event, "WithdrewERC20", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DrippieWithdrewERC721Iterator is returned from FilterWithdrewERC721 and is used to iterate over the raw logs and unpacked data for WithdrewERC721 events raised by the Drippie contract.
type DrippieWithdrewERC721Iterator struct {
	Event *DrippieWithdrewERC721 // Event containing the contract specifics and raw log

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
func (it *DrippieWithdrewERC721Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DrippieWithdrewERC721)
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
		it.Event = new(DrippieWithdrewERC721)
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
func (it *DrippieWithdrewERC721Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DrippieWithdrewERC721Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DrippieWithdrewERC721 represents a WithdrewERC721 event raised by the Drippie contract.
type DrippieWithdrewERC721 struct {
	Withdrawer common.Address
	Recipient  common.Address
	Asset      common.Address
	Id         *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterWithdrewERC721 is a free log retrieval operation binding the contract event 0x30b478a5e196e55886228aa87ba74a7dfeba655e0a4d7ba275eabfc22aabb7a8.
//
// Solidity: event WithdrewERC721(address indexed withdrawer, address indexed recipient, address indexed asset, uint256 id)
func (_Drippie *DrippieFilterer) FilterWithdrewERC721(opts *bind.FilterOpts, withdrawer []common.Address, recipient []common.Address, asset []common.Address) (*DrippieWithdrewERC721Iterator, error) {

	var withdrawerRule []interface{}
	for _, withdrawerItem := range withdrawer {
		withdrawerRule = append(withdrawerRule, withdrawerItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}
	var assetRule []interface{}
	for _, assetItem := range asset {
		assetRule = append(assetRule, assetItem)
	}

	logs, sub, err := _Drippie.contract.FilterLogs(opts, "WithdrewERC721", withdrawerRule, recipientRule, assetRule)
	if err != nil {
		return nil, err
	}
	return &DrippieWithdrewERC721Iterator{contract: _Drippie.contract, event: "WithdrewERC721", logs: logs, sub: sub}, nil
}

// WatchWithdrewERC721 is a free log subscription operation binding the contract event 0x30b478a5e196e55886228aa87ba74a7dfeba655e0a4d7ba275eabfc22aabb7a8.
//
// Solidity: event WithdrewERC721(address indexed withdrawer, address indexed recipient, address indexed asset, uint256 id)
func (_Drippie *DrippieFilterer) WatchWithdrewERC721(opts *bind.WatchOpts, sink chan<- *DrippieWithdrewERC721, withdrawer []common.Address, recipient []common.Address, asset []common.Address) (event.Subscription, error) {

	var withdrawerRule []interface{}
	for _, withdrawerItem := range withdrawer {
		withdrawerRule = append(withdrawerRule, withdrawerItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}
	var assetRule []interface{}
	for _, assetItem := range asset {
		assetRule = append(assetRule, assetItem)
	}

	logs, sub, err := _Drippie.contract.WatchLogs(opts, "WithdrewERC721", withdrawerRule, recipientRule, assetRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DrippieWithdrewERC721)
				if err := _Drippie.contract.UnpackLog(event, "WithdrewERC721", log); err != nil {
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

// ParseWithdrewERC721 is a log parse operation binding the contract event 0x30b478a5e196e55886228aa87ba74a7dfeba655e0a4d7ba275eabfc22aabb7a8.
//
// Solidity: event WithdrewERC721(address indexed withdrawer, address indexed recipient, address indexed asset, uint256 id)
func (_Drippie *DrippieFilterer) ParseWithdrewERC721(log types.Log) (*DrippieWithdrewERC721, error) {
	event := new(DrippieWithdrewERC721)
	if err := _Drippie.contract.UnpackLog(event, "WithdrewERC721", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DrippieWithdrewETHIterator is returned from FilterWithdrewETH and is used to iterate over the raw logs and unpacked data for WithdrewETH events raised by the Drippie contract.
type DrippieWithdrewETHIterator struct {
	Event *DrippieWithdrewETH // Event containing the contract specifics and raw log

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
func (it *DrippieWithdrewETHIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DrippieWithdrewETH)
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
		it.Event = new(DrippieWithdrewETH)
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
func (it *DrippieWithdrewETHIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DrippieWithdrewETHIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DrippieWithdrewETH represents a WithdrewETH event raised by the Drippie contract.
type DrippieWithdrewETH struct {
	Withdrawer common.Address
	Recipient  common.Address
	Amount     *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterWithdrewETH is a free log retrieval operation binding the contract event 0x1f12aa8b6d492dd9b98e2b00b0b20830c2a7ded65afac13b60d169a034ae90bc.
//
// Solidity: event WithdrewETH(address indexed withdrawer, address indexed recipient, uint256 amount)
func (_Drippie *DrippieFilterer) FilterWithdrewETH(opts *bind.FilterOpts, withdrawer []common.Address, recipient []common.Address) (*DrippieWithdrewETHIterator, error) {

	var withdrawerRule []interface{}
	for _, withdrawerItem := range withdrawer {
		withdrawerRule = append(withdrawerRule, withdrawerItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _Drippie.contract.FilterLogs(opts, "WithdrewETH", withdrawerRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return &DrippieWithdrewETHIterator{contract: _Drippie.contract, event: "WithdrewETH", logs: logs, sub: sub}, nil
}

// WatchWithdrewETH is a free log subscription operation binding the contract event 0x1f12aa8b6d492dd9b98e2b00b0b20830c2a7ded65afac13b60d169a034ae90bc.
//
// Solidity: event WithdrewETH(address indexed withdrawer, address indexed recipient, uint256 amount)
func (_Drippie *DrippieFilterer) WatchWithdrewETH(opts *bind.WatchOpts, sink chan<- *DrippieWithdrewETH, withdrawer []common.Address, recipient []common.Address) (event.Subscription, error) {

	var withdrawerRule []interface{}
	for _, withdrawerItem := range withdrawer {
		withdrawerRule = append(withdrawerRule, withdrawerItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _Drippie.contract.WatchLogs(opts, "WithdrewETH", withdrawerRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DrippieWithdrewETH)
				if err := _Drippie.contract.UnpackLog(event, "WithdrewETH", log); err != nil {
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

// ParseWithdrewETH is a log parse operation binding the contract event 0x1f12aa8b6d492dd9b98e2b00b0b20830c2a7ded65afac13b60d169a034ae90bc.
//
// Solidity: event WithdrewETH(address indexed withdrawer, address indexed recipient, uint256 amount)
func (_Drippie *DrippieFilterer) ParseWithdrewETH(log types.Log) (*DrippieWithdrewETH, error) {
	event := new(DrippieWithdrewETH)
	if err := _Drippie.contract.UnpackLog(event, "WithdrewETH", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

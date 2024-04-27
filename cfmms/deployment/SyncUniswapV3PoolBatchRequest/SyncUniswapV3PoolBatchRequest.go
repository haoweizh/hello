// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package SyncUniswapV3PoolBatchRequest

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

// SyncUniswapV3PoolBatchRequestMetaData contains all meta data concerning the SyncUniswapV3PoolBatchRequest contract.
var SyncUniswapV3PoolBatchRequestMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"pools\",\"type\":\"address[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]",
	Bin: "0x608060405234801561000f575f80fd5b50604051610b68380380610b6883398181016040528101906100319190610568565b5f815167ffffffffffffffff81111561004d5761004c6103d2565b5b60405190808252806020026020018201604052801561008657816020015b61007361035b565b81526020019060019003908161006b5790505b5090505f5b82518110156102fc575f8382815181106100a8576100a76105af565b5b602002602001015190506100c18161032a60201b60201c565b156100cc57506102eb565b6100d461035b565b5f808373ffffffffffffffffffffffffffffffffffffffff16633850c7bd6040518163ffffffff1660e01b815260040160e060405180830381865afa15801561011f573d5f803e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061014391906106de565b5050505050915091505f8473ffffffffffffffffffffffffffffffffffffffff1663f30dba93836040518263ffffffff1660e01b8152600401610186919061078a565b61010060405180830381865afa1580156101a2573d5f803e3d5ffd5b505050506040513d601f19601f820116820180604052508101906101c691906108c0565b5050505050509150508473ffffffffffffffffffffffffffffffffffffffff16631a6865026040518163ffffffff1660e01b8152600401602060405180830381865afa158015610218573d5f803e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061023c9190610971565b845f01906fffffffffffffffffffffffffffffffff1690816fffffffffffffffffffffffffffffffff168152505082846020019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505081846040019060020b908160020b81525050808460600190600f0b9081600f0b81525050838787815181106102da576102d96105af565b5b602002602001018190525050505050505b806102f5906109c9565b905061008b565b505f8160405160200161030f9190610b47565b60405160208183030381529060405290506020810180590381f35b5f808273ffffffffffffffffffffffffffffffffffffffff163b036103525760019050610356565b5f90505b919050565b60405180608001604052805f6fffffffffffffffffffffffffffffffff1681526020015f73ffffffffffffffffffffffffffffffffffffffff1681526020015f60020b81526020015f600f0b81525090565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b610408826103c2565b810181811067ffffffffffffffff82111715610427576104266103d2565b5b80604052505050565b5f6104396103ad565b905061044582826103ff565b919050565b5f67ffffffffffffffff821115610464576104636103d2565b5b602082029050602081019050919050565b5f80fd5b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6104a282610479565b9050919050565b6104b281610498565b81146104bc575f80fd5b50565b5f815190506104cd816104a9565b92915050565b5f6104e56104e08461044a565b610430565b9050808382526020820190506020840283018581111561050857610507610475565b5b835b81811015610531578061051d88826104bf565b84526020840193505060208101905061050a565b5050509392505050565b5f82601f83011261054f5761054e6103be565b5b815161055f8482602086016104d3565b91505092915050565b5f6020828403121561057d5761057c6103b6565b5b5f82015167ffffffffffffffff81111561059a576105996103ba565b5b6105a68482850161053b565b91505092915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52603260045260245ffd5b6105e581610479565b81146105ef575f80fd5b50565b5f81519050610600816105dc565b92915050565b5f8160020b9050919050565b61061b81610606565b8114610625575f80fd5b50565b5f8151905061063681610612565b92915050565b5f61ffff82169050919050565b6106528161063c565b811461065c575f80fd5b50565b5f8151905061066d81610649565b92915050565b5f60ff82169050919050565b61068881610673565b8114610692575f80fd5b50565b5f815190506106a38161067f565b92915050565b5f8115159050919050565b6106bd816106a9565b81146106c7575f80fd5b50565b5f815190506106d8816106b4565b92915050565b5f805f805f805f60e0888a0312156106f9576106f86103b6565b5b5f6107068a828b016105f2565b97505060206107178a828b01610628565b96505060406107288a828b0161065f565b95505060606107398a828b0161065f565b945050608061074a8a828b0161065f565b93505060a061075b8a828b01610695565b92505060c061076c8a828b016106ca565b91505092959891949750929550565b61078481610606565b82525050565b5f60208201905061079d5f83018461077b565b92915050565b5f6fffffffffffffffffffffffffffffffff82169050919050565b6107c7816107a3565b81146107d1575f80fd5b50565b5f815190506107e2816107be565b92915050565b5f81600f0b9050919050565b6107fd816107e8565b8114610807575f80fd5b50565b5f81519050610818816107f4565b92915050565b5f819050919050565b6108308161081e565b811461083a575f80fd5b50565b5f8151905061084b81610827565b92915050565b5f8160060b9050919050565b61086681610851565b8114610870575f80fd5b50565b5f815190506108818161085d565b92915050565b5f63ffffffff82169050919050565b61089f81610887565b81146108a9575f80fd5b50565b5f815190506108ba81610896565b92915050565b5f805f805f805f80610100898b0312156108dd576108dc6103b6565b5b5f6108ea8b828c016107d4565b98505060206108fb8b828c0161080a565b975050604061090c8b828c0161083d565b965050606061091d8b828c0161083d565b955050608061092e8b828c01610873565b94505060a061093f8b828c016105f2565b93505060c06109508b828c016108ac565b92505060e06109618b828c016106ca565b9150509295985092959890939650565b5f60208284031215610986576109856103b6565b5b5f610993848285016107d4565b91505092915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f6109d38261081e565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8203610a0557610a0461099c565b5b600182019050919050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b610a42816107a3565b82525050565b610a5181610479565b82525050565b610a6081610606565b82525050565b610a6f816107e8565b82525050565b608082015f820151610a895f850182610a39565b506020820151610a9c6020850182610a48565b506040820151610aaf6040850182610a57565b506060820151610ac26060850182610a66565b50505050565b5f610ad38383610a75565b60808301905092915050565b5f602082019050919050565b5f610af582610a10565b610aff8185610a1a565b9350610b0a83610a2a565b805f5b83811015610b3a578151610b218882610ac8565b9750610b2c83610adf565b925050600181019050610b0d565b5085935050505092915050565b5f6020820190508181035f830152610b5f8184610aeb565b90509291505056fe",
}

// SyncUniswapV3PoolBatchRequestABI is the input ABI used to generate the binding from.
// Deprecated: Use SyncUniswapV3PoolBatchRequestMetaData.ABI instead.
var SyncUniswapV3PoolBatchRequestABI = SyncUniswapV3PoolBatchRequestMetaData.ABI

// SyncUniswapV3PoolBatchRequestBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use SyncUniswapV3PoolBatchRequestMetaData.Bin instead.
var SyncUniswapV3PoolBatchRequestBin = SyncUniswapV3PoolBatchRequestMetaData.Bin

// DeploySyncUniswapV3PoolBatchRequest deploys a new Ethereum contract, binding an instance of SyncUniswapV3PoolBatchRequest to it.
func DeploySyncUniswapV3PoolBatchRequest(auth *bind.TransactOpts, backend bind.ContractBackend, pools []common.Address) (common.Address, *types.Transaction, *SyncUniswapV3PoolBatchRequest, error) {
	parsed, err := SyncUniswapV3PoolBatchRequestMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(SyncUniswapV3PoolBatchRequestBin), backend, pools)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &SyncUniswapV3PoolBatchRequest{SyncUniswapV3PoolBatchRequestCaller: SyncUniswapV3PoolBatchRequestCaller{contract: contract}, SyncUniswapV3PoolBatchRequestTransactor: SyncUniswapV3PoolBatchRequestTransactor{contract: contract}, SyncUniswapV3PoolBatchRequestFilterer: SyncUniswapV3PoolBatchRequestFilterer{contract: contract}}, nil
}

// SyncUniswapV3PoolBatchRequest is an auto generated Go binding around an Ethereum contract.
type SyncUniswapV3PoolBatchRequest struct {
	SyncUniswapV3PoolBatchRequestCaller     // ReadServe-only binding to the contract
	SyncUniswapV3PoolBatchRequestTransactor // WriteServe-only binding to the contract
	SyncUniswapV3PoolBatchRequestFilterer   // Log filterer for contract events
}

// SyncUniswapV3PoolBatchRequestCaller is an auto generated read-only Go binding around an Ethereum contract.
type SyncUniswapV3PoolBatchRequestCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SyncUniswapV3PoolBatchRequestTransactor is an auto generated write-only Go binding around an Ethereum contract.
type SyncUniswapV3PoolBatchRequestTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SyncUniswapV3PoolBatchRequestFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type SyncUniswapV3PoolBatchRequestFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SyncUniswapV3PoolBatchRequestSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type SyncUniswapV3PoolBatchRequestSession struct {
	Contract     *SyncUniswapV3PoolBatchRequest // Generic contract binding to set the session for
	CallOpts     bind.CallOpts                  // Call options to use throughout this session
	TransactOpts bind.TransactOpts              // Transaction auth options to use throughout this session
}

// SyncUniswapV3PoolBatchRequestCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type SyncUniswapV3PoolBatchRequestCallerSession struct {
	Contract *SyncUniswapV3PoolBatchRequestCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                        // Call options to use throughout this session
}

// SyncUniswapV3PoolBatchRequestTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type SyncUniswapV3PoolBatchRequestTransactorSession struct {
	Contract     *SyncUniswapV3PoolBatchRequestTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                        // Transaction auth options to use throughout this session
}

// SyncUniswapV3PoolBatchRequestRaw is an auto generated low-level Go binding around an Ethereum contract.
type SyncUniswapV3PoolBatchRequestRaw struct {
	Contract *SyncUniswapV3PoolBatchRequest // Generic contract binding to access the raw methods on
}

// SyncUniswapV3PoolBatchRequestCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type SyncUniswapV3PoolBatchRequestCallerRaw struct {
	Contract *SyncUniswapV3PoolBatchRequestCaller // Generic read-only contract binding to access the raw methods on
}

// SyncUniswapV3PoolBatchRequestTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type SyncUniswapV3PoolBatchRequestTransactorRaw struct {
	Contract *SyncUniswapV3PoolBatchRequestTransactor // Generic write-only contract binding to access the raw methods on
}

// NewSyncUniswapV3PoolBatchRequest creates a new instance of SyncUniswapV3PoolBatchRequest, bound to a specific deployed contract.
func NewSyncUniswapV3PoolBatchRequest(address common.Address, backend bind.ContractBackend) (*SyncUniswapV3PoolBatchRequest, error) {
	contract, err := bindSyncUniswapV3PoolBatchRequest(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &SyncUniswapV3PoolBatchRequest{SyncUniswapV3PoolBatchRequestCaller: SyncUniswapV3PoolBatchRequestCaller{contract: contract}, SyncUniswapV3PoolBatchRequestTransactor: SyncUniswapV3PoolBatchRequestTransactor{contract: contract}, SyncUniswapV3PoolBatchRequestFilterer: SyncUniswapV3PoolBatchRequestFilterer{contract: contract}}, nil
}

// NewSyncUniswapV3PoolBatchRequestCaller creates a new read-only instance of SyncUniswapV3PoolBatchRequest, bound to a specific deployed contract.
func NewSyncUniswapV3PoolBatchRequestCaller(address common.Address, caller bind.ContractCaller) (*SyncUniswapV3PoolBatchRequestCaller, error) {
	contract, err := bindSyncUniswapV3PoolBatchRequest(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &SyncUniswapV3PoolBatchRequestCaller{contract: contract}, nil
}

// NewSyncUniswapV3PoolBatchRequestTransactor creates a new write-only instance of SyncUniswapV3PoolBatchRequest, bound to a specific deployed contract.
func NewSyncUniswapV3PoolBatchRequestTransactor(address common.Address, transactor bind.ContractTransactor) (*SyncUniswapV3PoolBatchRequestTransactor, error) {
	contract, err := bindSyncUniswapV3PoolBatchRequest(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &SyncUniswapV3PoolBatchRequestTransactor{contract: contract}, nil
}

// NewSyncUniswapV3PoolBatchRequestFilterer creates a new log filterer instance of SyncUniswapV3PoolBatchRequest, bound to a specific deployed contract.
func NewSyncUniswapV3PoolBatchRequestFilterer(address common.Address, filterer bind.ContractFilterer) (*SyncUniswapV3PoolBatchRequestFilterer, error) {
	contract, err := bindSyncUniswapV3PoolBatchRequest(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &SyncUniswapV3PoolBatchRequestFilterer{contract: contract}, nil
}

// bindSyncUniswapV3PoolBatchRequest binds a generic wrapper to an already deployed contract.
func bindSyncUniswapV3PoolBatchRequest(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(SyncUniswapV3PoolBatchRequestABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SyncUniswapV3PoolBatchRequest *SyncUniswapV3PoolBatchRequestRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SyncUniswapV3PoolBatchRequest.Contract.SyncUniswapV3PoolBatchRequestCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SyncUniswapV3PoolBatchRequest *SyncUniswapV3PoolBatchRequestRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SyncUniswapV3PoolBatchRequest.Contract.SyncUniswapV3PoolBatchRequestTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SyncUniswapV3PoolBatchRequest *SyncUniswapV3PoolBatchRequestRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SyncUniswapV3PoolBatchRequest.Contract.SyncUniswapV3PoolBatchRequestTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SyncUniswapV3PoolBatchRequest *SyncUniswapV3PoolBatchRequestCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SyncUniswapV3PoolBatchRequest.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SyncUniswapV3PoolBatchRequest *SyncUniswapV3PoolBatchRequestTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SyncUniswapV3PoolBatchRequest.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SyncUniswapV3PoolBatchRequest *SyncUniswapV3PoolBatchRequestTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SyncUniswapV3PoolBatchRequest.Contract.contract.Transact(opts, method, params...)
}

// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package GetUniswapV2PoolDataBatchRequest

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

// GetUniswapV2PoolDataBatchRequestMetaData contains all meta data concerning the GetUniswapV2PoolDataBatchRequest contract.
var GetUniswapV2PoolDataBatchRequestMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"pools\",\"type\":\"address[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]",
	Bin: "0x608060405234801561000f575f80fd5b50604051610cf0380380610cf0833981810160405281019061003191906108b6565b5f815167ffffffffffffffff81111561004d5761004c610720565b5b60405190808252806020026020018201604052801561008657816020015b610073610679565b81526020019060019003908161006b5790505b5090505f5b825181101561061a575f8382815181106100a8576100a76108fd565b5b602002602001015190506100c18161064860201b60201c565b156100cc5750610609565b6100d4610679565b8173ffffffffffffffffffffffffffffffffffffffff16630dfe16816040518163ffffffff1660e01b8152600401602060405180830381865afa15801561011d573d5f803e3d5ffd5b505050506040513d601f19601f82011682018060405250810190610141919061092a565b815f019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff16815250508173ffffffffffffffffffffffffffffffffffffffff1663d21220a76040518163ffffffff1660e01b8152600401602060405180830381865afa1580156101c0573d5f803e3d5ffd5b505050506040513d601f19601f820116820180604052508101906101e4919061092a565b816040019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505061022d815f015161064860201b60201c565b15610239575050610609565b61024c816040015161064860201b60201c565b15610258575050610609565b5f80825f015173ffffffffffffffffffffffffffffffffffffffff166040516024016040516020818303038152906040527f313ce567000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff838183161783525050505060405161030491906109c1565b5f604051808303815f865af19150503d805f811461033d576040519150601f19603f3d011682016040523d82523d5f602084013e610342565b606091505b509150915081156103b1575f60208251036103a1578180602001905181019061036b9190610a0a565b90505f81148061037b575060ff81115b1561038a575050505050610609565b80846020019060ff16908160ff16815250506103ab565b5050505050610609565b506103ba565b50505050610609565b5f80846040015173ffffffffffffffffffffffffffffffffffffffff166040516024016040516020818303038152906040527f313ce567000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff838183161783525050505060405161046791906109c1565b5f604051808303815f865af19150503d805f81146104a0576040519150601f19603f3d011682016040523d82523d5f602084013e6104a5565b606091505b50915091508115610518575f602082510361050657818060200190518101906104ce9190610a0a565b90505f8114806104de575060ff81115b156104ef5750505050505050610609565b80866060019060ff16908160ff1681525050610512565b50505050505050610609565b50610523565b505050505050610609565b8573ffffffffffffffffffffffffffffffffffffffff16630902f1ac6040518163ffffffff1660e01b8152600401606060405180830381865afa15801561056c573d5f803e3d5ffd5b505050506040513d601f19601f820116820180604052508101906105909190610ab1565b50866080018760a001826dffffffffffffffffffffffffffff166dffffffffffffffffffffffffffff16815250826dffffffffffffffffffffffffffff166dffffffffffffffffffffffffffff168152505050848888815181106105f7576105f66108fd565b5b60200260200101819052505050505050505b8061061390610b2e565b905061008b565b505f8160405160200161062d9190610ccf565b60405160208183030381529060405290506020810180590381f35b5f808273ffffffffffffffffffffffffffffffffffffffff163b036106705760019050610674565b5f90505b919050565b6040518060c001604052805f73ffffffffffffffffffffffffffffffffffffffff1681526020015f60ff1681526020015f73ffffffffffffffffffffffffffffffffffffffff1681526020015f60ff1681526020015f6dffffffffffffffffffffffffffff1681526020015f6dffffffffffffffffffffffffffff1681525090565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b61075682610710565b810181811067ffffffffffffffff8211171561077557610774610720565b5b80604052505050565b5f6107876106fb565b9050610793828261074d565b919050565b5f67ffffffffffffffff8211156107b2576107b1610720565b5b602082029050602081019050919050565b5f80fd5b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6107f0826107c7565b9050919050565b610800816107e6565b811461080a575f80fd5b50565b5f8151905061081b816107f7565b92915050565b5f61083361082e84610798565b61077e565b90508083825260208201905060208402830185811115610856576108556107c3565b5b835b8181101561087f578061086b888261080d565b845260208401935050602081019050610858565b5050509392505050565b5f82601f83011261089d5761089c61070c565b5b81516108ad848260208601610821565b91505092915050565b5f602082840312156108cb576108ca610704565b5b5f82015167ffffffffffffffff8111156108e8576108e7610708565b5b6108f484828501610889565b91505092915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52603260045260245ffd5b5f6020828403121561093f5761093e610704565b5b5f61094c8482850161080d565b91505092915050565b5f81519050919050565b5f81905092915050565b5f5b8381101561098657808201518184015260208101905061096b565b5f8484015250505050565b5f61099b82610955565b6109a5818561095f565b93506109b5818560208601610969565b80840191505092915050565b5f6109cc8284610991565b915081905092915050565b5f819050919050565b6109e9816109d7565b81146109f3575f80fd5b50565b5f81519050610a04816109e0565b92915050565b5f60208284031215610a1f57610a1e610704565b5b5f610a2c848285016109f6565b91505092915050565b5f6dffffffffffffffffffffffffffff82169050919050565b610a5781610a35565b8114610a61575f80fd5b50565b5f81519050610a7281610a4e565b92915050565b5f63ffffffff82169050919050565b610a9081610a78565b8114610a9a575f80fd5b50565b5f81519050610aab81610a87565b92915050565b5f805f60608486031215610ac857610ac7610704565b5b5f610ad586828701610a64565b9350506020610ae686828701610a64565b9250506040610af786828701610a9d565b9150509250925092565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f610b38826109d7565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8203610b6a57610b69610b01565b5b600182019050919050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b610ba7816107e6565b82525050565b5f60ff82169050919050565b610bc281610bad565b82525050565b610bd181610a35565b82525050565b60c082015f820151610beb5f850182610b9e565b506020820151610bfe6020850182610bb9565b506040820151610c116040850182610b9e565b506060820151610c246060850182610bb9565b506080820151610c376080850182610bc8565b5060a0820151610c4a60a0850182610bc8565b50505050565b5f610c5b8383610bd7565b60c08301905092915050565b5f602082019050919050565b5f610c7d82610b75565b610c878185610b7f565b9350610c9283610b8f565b805f5b83811015610cc2578151610ca98882610c50565b9750610cb483610c67565b925050600181019050610c95565b5085935050505092915050565b5f6020820190508181035f830152610ce78184610c73565b90509291505056fe",
}

// GetUniswapV2PoolDataBatchRequestABI is the input ABI used to generate the binding from.
// Deprecated: Use GetUniswapV2PoolDataBatchRequestMetaData.ABI instead.
var GetUniswapV2PoolDataBatchRequestABI = GetUniswapV2PoolDataBatchRequestMetaData.ABI

// GetUniswapV2PoolDataBatchRequestBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use GetUniswapV2PoolDataBatchRequestMetaData.Bin instead.
var GetUniswapV2PoolDataBatchRequestBin = GetUniswapV2PoolDataBatchRequestMetaData.Bin

// DeployGetUniswapV2PoolDataBatchRequest deploys a new Ethereum contract, binding an instance of GetUniswapV2PoolDataBatchRequest to it.
func DeployGetUniswapV2PoolDataBatchRequest(auth *bind.TransactOpts, backend bind.ContractBackend, pools []common.Address) (common.Address, *types.Transaction, *GetUniswapV2PoolDataBatchRequest, error) {
	parsed, err := GetUniswapV2PoolDataBatchRequestMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(GetUniswapV2PoolDataBatchRequestBin), backend, pools)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &GetUniswapV2PoolDataBatchRequest{GetUniswapV2PoolDataBatchRequestCaller: GetUniswapV2PoolDataBatchRequestCaller{contract: contract}, GetUniswapV2PoolDataBatchRequestTransactor: GetUniswapV2PoolDataBatchRequestTransactor{contract: contract}, GetUniswapV2PoolDataBatchRequestFilterer: GetUniswapV2PoolDataBatchRequestFilterer{contract: contract}}, nil
}

// GetUniswapV2PoolDataBatchRequest is an auto generated Go binding around an Ethereum contract.
type GetUniswapV2PoolDataBatchRequest struct {
	GetUniswapV2PoolDataBatchRequestCaller     // ReadServe-only binding to the contract
	GetUniswapV2PoolDataBatchRequestTransactor // WriteServe-only binding to the contract
	GetUniswapV2PoolDataBatchRequestFilterer   // Log filterer for contract events
}

// GetUniswapV2PoolDataBatchRequestCaller is an auto generated read-only Go binding around an Ethereum contract.
type GetUniswapV2PoolDataBatchRequestCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GetUniswapV2PoolDataBatchRequestTransactor is an auto generated write-only Go binding around an Ethereum contract.
type GetUniswapV2PoolDataBatchRequestTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GetUniswapV2PoolDataBatchRequestFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type GetUniswapV2PoolDataBatchRequestFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GetUniswapV2PoolDataBatchRequestSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type GetUniswapV2PoolDataBatchRequestSession struct {
	Contract     *GetUniswapV2PoolDataBatchRequest // Generic contract binding to set the session for
	CallOpts     bind.CallOpts                     // Call options to use throughout this session
	TransactOpts bind.TransactOpts                 // Transaction auth options to use throughout this session
}

// GetUniswapV2PoolDataBatchRequestCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type GetUniswapV2PoolDataBatchRequestCallerSession struct {
	Contract *GetUniswapV2PoolDataBatchRequestCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                           // Call options to use throughout this session
}

// GetUniswapV2PoolDataBatchRequestTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type GetUniswapV2PoolDataBatchRequestTransactorSession struct {
	Contract     *GetUniswapV2PoolDataBatchRequestTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                           // Transaction auth options to use throughout this session
}

// GetUniswapV2PoolDataBatchRequestRaw is an auto generated low-level Go binding around an Ethereum contract.
type GetUniswapV2PoolDataBatchRequestRaw struct {
	Contract *GetUniswapV2PoolDataBatchRequest // Generic contract binding to access the raw methods on
}

// GetUniswapV2PoolDataBatchRequestCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type GetUniswapV2PoolDataBatchRequestCallerRaw struct {
	Contract *GetUniswapV2PoolDataBatchRequestCaller // Generic read-only contract binding to access the raw methods on
}

// GetUniswapV2PoolDataBatchRequestTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type GetUniswapV2PoolDataBatchRequestTransactorRaw struct {
	Contract *GetUniswapV2PoolDataBatchRequestTransactor // Generic write-only contract binding to access the raw methods on
}

// NewGetUniswapV2PoolDataBatchRequest creates a new instance of GetUniswapV2PoolDataBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV2PoolDataBatchRequest(address common.Address, backend bind.ContractBackend) (*GetUniswapV2PoolDataBatchRequest, error) {
	contract, err := bindGetUniswapV2PoolDataBatchRequest(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV2PoolDataBatchRequest{GetUniswapV2PoolDataBatchRequestCaller: GetUniswapV2PoolDataBatchRequestCaller{contract: contract}, GetUniswapV2PoolDataBatchRequestTransactor: GetUniswapV2PoolDataBatchRequestTransactor{contract: contract}, GetUniswapV2PoolDataBatchRequestFilterer: GetUniswapV2PoolDataBatchRequestFilterer{contract: contract}}, nil
}

// NewGetUniswapV2PoolDataBatchRequestCaller creates a new read-only instance of GetUniswapV2PoolDataBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV2PoolDataBatchRequestCaller(address common.Address, caller bind.ContractCaller) (*GetUniswapV2PoolDataBatchRequestCaller, error) {
	contract, err := bindGetUniswapV2PoolDataBatchRequest(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV2PoolDataBatchRequestCaller{contract: contract}, nil
}

// NewGetUniswapV2PoolDataBatchRequestTransactor creates a new write-only instance of GetUniswapV2PoolDataBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV2PoolDataBatchRequestTransactor(address common.Address, transactor bind.ContractTransactor) (*GetUniswapV2PoolDataBatchRequestTransactor, error) {
	contract, err := bindGetUniswapV2PoolDataBatchRequest(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV2PoolDataBatchRequestTransactor{contract: contract}, nil
}

// NewGetUniswapV2PoolDataBatchRequestFilterer creates a new log filterer instance of GetUniswapV2PoolDataBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV2PoolDataBatchRequestFilterer(address common.Address, filterer bind.ContractFilterer) (*GetUniswapV2PoolDataBatchRequestFilterer, error) {
	contract, err := bindGetUniswapV2PoolDataBatchRequest(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV2PoolDataBatchRequestFilterer{contract: contract}, nil
}

// bindGetUniswapV2PoolDataBatchRequest binds a generic wrapper to an already deployed contract.
func bindGetUniswapV2PoolDataBatchRequest(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(GetUniswapV2PoolDataBatchRequestABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_GetUniswapV2PoolDataBatchRequest *GetUniswapV2PoolDataBatchRequestRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _GetUniswapV2PoolDataBatchRequest.Contract.GetUniswapV2PoolDataBatchRequestCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_GetUniswapV2PoolDataBatchRequest *GetUniswapV2PoolDataBatchRequestRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _GetUniswapV2PoolDataBatchRequest.Contract.GetUniswapV2PoolDataBatchRequestTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_GetUniswapV2PoolDataBatchRequest *GetUniswapV2PoolDataBatchRequestRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _GetUniswapV2PoolDataBatchRequest.Contract.GetUniswapV2PoolDataBatchRequestTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_GetUniswapV2PoolDataBatchRequest *GetUniswapV2PoolDataBatchRequestCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _GetUniswapV2PoolDataBatchRequest.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_GetUniswapV2PoolDataBatchRequest *GetUniswapV2PoolDataBatchRequestTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _GetUniswapV2PoolDataBatchRequest.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_GetUniswapV2PoolDataBatchRequest *GetUniswapV2PoolDataBatchRequestTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _GetUniswapV2PoolDataBatchRequest.Contract.contract.Transact(opts, method, params...)
}

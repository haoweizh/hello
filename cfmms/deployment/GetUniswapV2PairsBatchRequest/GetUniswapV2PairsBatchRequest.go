// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package GetUniswapV2PairsBatchRequest

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

// GetUniswapV2PairsBatchRequestMetaData contains all meta data concerning the GetUniswapV2PairsBatchRequest contract.
var GetUniswapV2PairsBatchRequestMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"from\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"step\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"factory\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]",
	Bin: "0x608060405234801561000f575f80fd5b506040516104e83803806104e883398181016040528101906100319190610239565b5f838361003e91906102b6565b90505f8167ffffffffffffffff81111561005b5761005a6102e9565b5b6040519080825280602002602001820160405280156100895781602001602082028036833780820191505090505b5090505f5b8281101561017a578373ffffffffffffffffffffffffffffffffffffffff16631e3dd18b82886100be9190610316565b6040518263ffffffff1660e01b81526004016100da9190610358565b6020604051808303815f875af11580156100f6573d5f803e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061011a9190610371565b82828151811061012d5761012c61039c565b5b602002602001019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff16815250508080610172906103c9565b91505061008e565b505f8160405160200161018d91906104c7565b60405160208183030381529060405290506020810180590381f35b5f80fd5b5f819050919050565b6101be816101ac565b81146101c8575f80fd5b50565b5f815190506101d9816101b5565b92915050565b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f610208826101df565b9050919050565b610218816101fe565b8114610222575f80fd5b50565b5f815190506102338161020f565b92915050565b5f805f606084860312156102505761024f6101a8565b5b5f61025d868287016101cb565b935050602061026e868287016101cb565b925050604061027f86828701610225565b9150509250925092565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f6102c0826101ac565b91506102cb836101ac565b92508282039050818111156102e3576102e2610289565b5b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b5f610320826101ac565b915061032b836101ac565b925082820190508082111561034357610342610289565b5b92915050565b610352816101ac565b82525050565b5f60208201905061036b5f830184610349565b92915050565b5f60208284031215610386576103856101a8565b5b5f61039384828501610225565b91505092915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52603260045260245ffd5b5f6103d3826101ac565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff820361040557610404610289565b5b600182019050919050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b610442816101fe565b82525050565b5f6104538383610439565b60208301905092915050565b5f602082019050919050565b5f61047582610410565b61047f818561041a565b935061048a8361042a565b805f5b838110156104ba5781516104a18882610448565b97506104ac8361045f565b92505060018101905061048d565b5085935050505092915050565b5f6020820190508181035f8301526104df818461046b565b90509291505056fe",
}

// GetUniswapV2PairsBatchRequestABI is the input ABI used to generate the binding from.
// Deprecated: Use GetUniswapV2PairsBatchRequestMetaData.ABI instead.
var GetUniswapV2PairsBatchRequestABI = GetUniswapV2PairsBatchRequestMetaData.ABI

// GetUniswapV2PairsBatchRequestBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use GetUniswapV2PairsBatchRequestMetaData.Bin instead.
var GetUniswapV2PairsBatchRequestBin = GetUniswapV2PairsBatchRequestMetaData.Bin

// DeployGetUniswapV2PairsBatchRequest deploys a new Ethereum contract, binding an instance of GetUniswapV2PairsBatchRequest to it.
func DeployGetUniswapV2PairsBatchRequest(auth *bind.TransactOpts, backend bind.ContractBackend, from *big.Int, step *big.Int, factory common.Address) (common.Address, *types.Transaction, *GetUniswapV2PairsBatchRequest, error) {
	parsed, err := GetUniswapV2PairsBatchRequestMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(GetUniswapV2PairsBatchRequestBin), backend, from, step, factory)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &GetUniswapV2PairsBatchRequest{GetUniswapV2PairsBatchRequestCaller: GetUniswapV2PairsBatchRequestCaller{contract: contract}, GetUniswapV2PairsBatchRequestTransactor: GetUniswapV2PairsBatchRequestTransactor{contract: contract}, GetUniswapV2PairsBatchRequestFilterer: GetUniswapV2PairsBatchRequestFilterer{contract: contract}}, nil
}

// GetUniswapV2PairsBatchRequest is an auto generated Go binding around an Ethereum contract.
type GetUniswapV2PairsBatchRequest struct {
	GetUniswapV2PairsBatchRequestCaller     // ReadServe-only binding to the contract
	GetUniswapV2PairsBatchRequestTransactor // WriteServe-only binding to the contract
	GetUniswapV2PairsBatchRequestFilterer   // Log filterer for contract events
}

// GetUniswapV2PairsBatchRequestCaller is an auto generated read-only Go binding around an Ethereum contract.
type GetUniswapV2PairsBatchRequestCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GetUniswapV2PairsBatchRequestTransactor is an auto generated write-only Go binding around an Ethereum contract.
type GetUniswapV2PairsBatchRequestTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GetUniswapV2PairsBatchRequestFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type GetUniswapV2PairsBatchRequestFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GetUniswapV2PairsBatchRequestSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type GetUniswapV2PairsBatchRequestSession struct {
	Contract     *GetUniswapV2PairsBatchRequest // Generic contract binding to set the session for
	CallOpts     bind.CallOpts                  // Call options to use throughout this session
	TransactOpts bind.TransactOpts              // Transaction auth options to use throughout this session
}

// GetUniswapV2PairsBatchRequestCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type GetUniswapV2PairsBatchRequestCallerSession struct {
	Contract *GetUniswapV2PairsBatchRequestCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                        // Call options to use throughout this session
}

// GetUniswapV2PairsBatchRequestTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type GetUniswapV2PairsBatchRequestTransactorSession struct {
	Contract     *GetUniswapV2PairsBatchRequestTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                        // Transaction auth options to use throughout this session
}

// GetUniswapV2PairsBatchRequestRaw is an auto generated low-level Go binding around an Ethereum contract.
type GetUniswapV2PairsBatchRequestRaw struct {
	Contract *GetUniswapV2PairsBatchRequest // Generic contract binding to access the raw methods on
}

// GetUniswapV2PairsBatchRequestCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type GetUniswapV2PairsBatchRequestCallerRaw struct {
	Contract *GetUniswapV2PairsBatchRequestCaller // Generic read-only contract binding to access the raw methods on
}

// GetUniswapV2PairsBatchRequestTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type GetUniswapV2PairsBatchRequestTransactorRaw struct {
	Contract *GetUniswapV2PairsBatchRequestTransactor // Generic write-only contract binding to access the raw methods on
}

// NewGetUniswapV2PairsBatchRequest creates a new instance of GetUniswapV2PairsBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV2PairsBatchRequest(address common.Address, backend bind.ContractBackend) (*GetUniswapV2PairsBatchRequest, error) {
	contract, err := bindGetUniswapV2PairsBatchRequest(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV2PairsBatchRequest{GetUniswapV2PairsBatchRequestCaller: GetUniswapV2PairsBatchRequestCaller{contract: contract}, GetUniswapV2PairsBatchRequestTransactor: GetUniswapV2PairsBatchRequestTransactor{contract: contract}, GetUniswapV2PairsBatchRequestFilterer: GetUniswapV2PairsBatchRequestFilterer{contract: contract}}, nil
}

// NewGetUniswapV2PairsBatchRequestCaller creates a new read-only instance of GetUniswapV2PairsBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV2PairsBatchRequestCaller(address common.Address, caller bind.ContractCaller) (*GetUniswapV2PairsBatchRequestCaller, error) {
	contract, err := bindGetUniswapV2PairsBatchRequest(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV2PairsBatchRequestCaller{contract: contract}, nil
}

// NewGetUniswapV2PairsBatchRequestTransactor creates a new write-only instance of GetUniswapV2PairsBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV2PairsBatchRequestTransactor(address common.Address, transactor bind.ContractTransactor) (*GetUniswapV2PairsBatchRequestTransactor, error) {
	contract, err := bindGetUniswapV2PairsBatchRequest(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV2PairsBatchRequestTransactor{contract: contract}, nil
}

// NewGetUniswapV2PairsBatchRequestFilterer creates a new log filterer instance of GetUniswapV2PairsBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV2PairsBatchRequestFilterer(address common.Address, filterer bind.ContractFilterer) (*GetUniswapV2PairsBatchRequestFilterer, error) {
	contract, err := bindGetUniswapV2PairsBatchRequest(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV2PairsBatchRequestFilterer{contract: contract}, nil
}

// bindGetUniswapV2PairsBatchRequest binds a generic wrapper to an already deployed contract.
func bindGetUniswapV2PairsBatchRequest(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(GetUniswapV2PairsBatchRequestABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_GetUniswapV2PairsBatchRequest *GetUniswapV2PairsBatchRequestRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _GetUniswapV2PairsBatchRequest.Contract.GetUniswapV2PairsBatchRequestCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_GetUniswapV2PairsBatchRequest *GetUniswapV2PairsBatchRequestRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _GetUniswapV2PairsBatchRequest.Contract.GetUniswapV2PairsBatchRequestTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_GetUniswapV2PairsBatchRequest *GetUniswapV2PairsBatchRequestRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _GetUniswapV2PairsBatchRequest.Contract.GetUniswapV2PairsBatchRequestTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_GetUniswapV2PairsBatchRequest *GetUniswapV2PairsBatchRequestCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _GetUniswapV2PairsBatchRequest.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_GetUniswapV2PairsBatchRequest *GetUniswapV2PairsBatchRequestTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _GetUniswapV2PairsBatchRequest.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_GetUniswapV2PairsBatchRequest *GetUniswapV2PairsBatchRequestTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _GetUniswapV2PairsBatchRequest.Contract.contract.Transact(opts, method, params...)
}

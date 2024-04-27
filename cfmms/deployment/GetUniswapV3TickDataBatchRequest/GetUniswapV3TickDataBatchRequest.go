// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package GetUniswapV3TickDataBatchRequest

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

// GetUniswapV3TickDataBatchRequestMetaData contains all meta data concerning the GetUniswapV3TickDataBatchRequest contract.
var GetUniswapV3TickDataBatchRequestMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"zeroForOne\",\"type\":\"bool\"},{\"internalType\":\"int24\",\"name\":\"currentTick\",\"type\":\"int24\"},{\"internalType\":\"uint16\",\"name\":\"numTicks\",\"type\":\"uint16\"},{\"internalType\":\"int24\",\"name\":\"tickSpacing\",\"type\":\"int24\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]",
	Bin: "0x608060405234801561000f575f80fd5b50604051610f2c380380610f2c83398181016040528101906100319190610948565b5f8261ffff1667ffffffffffffffff8111156100505761004f6109bf565b5b60405190808252806020026020018201604052801561008957816020015b610076610822565b81526020019060019003908161006e5790505b5090505f5b8361ffff168110156103a4575f806100ae8988878b6103d360201b60201c565b915091505f8973ffffffffffffffffffffffffffffffffffffffff1663f30dba93846040518263ffffffff1660e01b81526004016100ec91906109fb565b61010060405180830381865afa158015610108573d5f803e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061012c9190610b5b565b5050505050509150507ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff2761860020b8360020b121561020f577ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff2761892508185858151811061019a57610199610c0c565b5b60200260200101515f019015159081151581525050828585815181106101c3576101c2610c0c565b5b60200260200101516020019060020b908160020b81525050808585815181106101ef576101ee610c0c565b5b602002602001015160400190600f0b9081600f0b815250505050506103a4565b7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff2761861023990610c66565b60020b8360020b13156102f2577ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff2761892508185858151811061027d5761027c610c0c565b5b60200260200101515f019015159081151581525050828585815181106102a6576102a5610c0c565b5b60200260200101516020019060020b908160020b81525050808585815181106102d2576102d1610c0c565b5b602002602001015160400190600f0b9081600f0b815250505050506103a4565b8185858151811061030657610305610c0c565b5b60200260200101515f0190151590811515815250508285858151811061032f5761032e610c0c565b5b60200260200101516020019060020b908160020b815250508085858151811061035b5761035a610c0c565b5b602002602001015160400190600f0b9081600f0b81525050838061037e90610cac565b9450508861038c578261039a565b6001836103999190610cf3565b5b975050505061008e565b5f82436040516020016103b8929190610e71565b60405160208183030381529060405290506020810180590381f35b5f805f8460020b8660020b816103ec576103eb610e9f565b5b0590505f8660020b12801561041a57505f8560020b8760020b8161041357610412610e9f565b5b0760020b14155b15610429578080600190039150505b8315610514575f806104408361060360201b60201c565b915091505f8160ff166001901b60018360ff166001901b030190505f818b73ffffffffffffffffffffffffffffffffffffffff16635339c296866040518263ffffffff1660e01b81526004016104969190610ee7565b602060405180830381865afa1580156104b1573d5f803e3d5ffd5b505050506040513d601f19601f820116820180604052508101906104d59190610f00565b1690505f8114159550856104f057888360ff16860302610509565b886105008261062c60201b60201c565b840360ff168603025b9650505050506105f9565b5f806105286001840161060360201b60201c565b915091505f60018260ff166001901b031990505f818b73ffffffffffffffffffffffffffffffffffffffff16635339c296866040518263ffffffff1660e01b81526004016105769190610ee7565b602060405180830381865afa158015610591573d5f803e3d5ffd5b505050506040513d601f19601f820116820180604052508101906105b59190610f00565b1690505f8114159550856105d657888360ff0360ff166001870101026105f2565b88836105e78361070560201b60201c565b0360ff166001870101025b9650505050505b5094509492505050565b5f8060088360020b901d91506101008360020b8161062457610623610e9f565b5b079050915091565b5f808211610638575f80fd5b700100000000000000000000000000000000821061065e57608082901c91506080810190505b68010000000000000000821061067c57604082901c91506040810190505b640100000000821061069657602082901c91506020810190505b6201000082106106ae57601082901c91506010810190505b61010082106106c557600882901c91506008810190505b601082106106db57600482901c91506004810190505b600482106106f157600282901c91506002810190505b60028210610700576001810190505b919050565b5f808211610711575f80fd5b60ff90505f6fffffffffffffffffffffffffffffffff80168316111561073c57608081039050610744565b608082901c91505b5f67ffffffffffffffff8016831611156107635760408103905061076b565b604082901c91505b5f63ffffffff8016831611156107865760208103905061078e565b602082901c91505b5f61ffff8016831611156107a7576010810390506107af565b601082901c91505b5f60ff8016831611156107c7576008810390506107cf565b600882901c91505b5f600f831611156107e5576004810390506107ed565b600482901c91505b5f6003831611156108035760028103905061080b565b600282901c91505b5f60018316111561081d576001810390505b919050565b60405180606001604052805f151581526020015f60020b81526020015f600f0b81525090565b5f80fd5b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6108758261084c565b9050919050565b6108858161086b565b811461088f575f80fd5b50565b5f815190506108a08161087c565b92915050565b5f8115159050919050565b6108ba816108a6565b81146108c4575f80fd5b50565b5f815190506108d5816108b1565b92915050565b5f8160020b9050919050565b6108f0816108db565b81146108fa575f80fd5b50565b5f8151905061090b816108e7565b92915050565b5f61ffff82169050919050565b61092781610911565b8114610931575f80fd5b50565b5f815190506109428161091e565b92915050565b5f805f805f60a0868803121561096157610960610848565b5b5f61096e88828901610892565b955050602061097f888289016108c7565b9450506040610990888289016108fd565b93505060606109a188828901610934565b92505060806109b2888289016108fd565b9150509295509295909350565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6109f5816108db565b82525050565b5f602082019050610a0e5f8301846109ec565b92915050565b5f6fffffffffffffffffffffffffffffffff82169050919050565b610a3881610a14565b8114610a42575f80fd5b50565b5f81519050610a5381610a2f565b92915050565b5f81600f0b9050919050565b610a6e81610a59565b8114610a78575f80fd5b50565b5f81519050610a8981610a65565b92915050565b5f819050919050565b610aa181610a8f565b8114610aab575f80fd5b50565b5f81519050610abc81610a98565b92915050565b5f8160060b9050919050565b610ad781610ac2565b8114610ae1575f80fd5b50565b5f81519050610af281610ace565b92915050565b610b018161084c565b8114610b0b575f80fd5b50565b5f81519050610b1c81610af8565b92915050565b5f63ffffffff82169050919050565b610b3a81610b22565b8114610b44575f80fd5b50565b5f81519050610b5581610b31565b92915050565b5f805f805f805f80610100898b031215610b7857610b77610848565b5b5f610b858b828c01610a45565b9850506020610b968b828c01610a7b565b9750506040610ba78b828c01610aae565b9650506060610bb88b828c01610aae565b9550506080610bc98b828c01610ae4565b94505060a0610bda8b828c01610b0e565b93505060c0610beb8b828c01610b47565b92505060e0610bfc8b828c016108c7565b9150509295985092959890939650565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52603260045260245ffd5b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f610c70826108db565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8000008203610ca257610ca1610c39565b5b815f039050919050565b5f610cb682610a8f565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8203610ce857610ce7610c39565b5b600182019050919050565b5f610cfd826108db565b9150610d08836108db565b92508282039050627fffff81137fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff80000082121715610d4757610d46610c39565b5b92915050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b610d7f816108a6565b82525050565b610d8e816108db565b82525050565b610d9d81610a59565b82525050565b606082015f820151610db75f850182610d76565b506020820151610dca6020850182610d85565b506040820151610ddd6040850182610d94565b50505050565b5f610dee8383610da3565b60608301905092915050565b5f602082019050919050565b5f610e1082610d4d565b610e1a8185610d57565b9350610e2583610d67565b805f5b83811015610e55578151610e3c8882610de3565b9750610e4783610dfa565b925050600181019050610e28565b5085935050505092915050565b610e6b81610a8f565b82525050565b5f6040820190508181035f830152610e898185610e06565b9050610e986020830184610e62565b9392505050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601260045260245ffd5b5f8160010b9050919050565b610ee181610ecc565b82525050565b5f602082019050610efa5f830184610ed8565b92915050565b5f60208284031215610f1557610f14610848565b5b5f610f2284828501610aae565b9150509291505056fe",
}

// GetUniswapV3TickDataBatchRequestABI is the input ABI used to generate the binding from.
// Deprecated: Use GetUniswapV3TickDataBatchRequestMetaData.ABI instead.
var GetUniswapV3TickDataBatchRequestABI = GetUniswapV3TickDataBatchRequestMetaData.ABI

// GetUniswapV3TickDataBatchRequestBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use GetUniswapV3TickDataBatchRequestMetaData.Bin instead.
var GetUniswapV3TickDataBatchRequestBin = GetUniswapV3TickDataBatchRequestMetaData.Bin

// DeployGetUniswapV3TickDataBatchRequest deploys a new Ethereum contract, binding an instance of GetUniswapV3TickDataBatchRequest to it.
func DeployGetUniswapV3TickDataBatchRequest(auth *bind.TransactOpts, backend bind.ContractBackend, pool common.Address, zeroForOne bool, currentTick *big.Int, numTicks uint16, tickSpacing *big.Int) (common.Address, *types.Transaction, *GetUniswapV3TickDataBatchRequest, error) {
	parsed, err := GetUniswapV3TickDataBatchRequestMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(GetUniswapV3TickDataBatchRequestBin), backend, pool, zeroForOne, currentTick, numTicks, tickSpacing)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &GetUniswapV3TickDataBatchRequest{GetUniswapV3TickDataBatchRequestCaller: GetUniswapV3TickDataBatchRequestCaller{contract: contract}, GetUniswapV3TickDataBatchRequestTransactor: GetUniswapV3TickDataBatchRequestTransactor{contract: contract}, GetUniswapV3TickDataBatchRequestFilterer: GetUniswapV3TickDataBatchRequestFilterer{contract: contract}}, nil
}

// GetUniswapV3TickDataBatchRequest is an auto generated Go binding around an Ethereum contract.
type GetUniswapV3TickDataBatchRequest struct {
	GetUniswapV3TickDataBatchRequestCaller     // ReadServe-only binding to the contract
	GetUniswapV3TickDataBatchRequestTransactor // WriteServe-only binding to the contract
	GetUniswapV3TickDataBatchRequestFilterer   // Log filterer for contract events
}

// GetUniswapV3TickDataBatchRequestCaller is an auto generated read-only Go binding around an Ethereum contract.
type GetUniswapV3TickDataBatchRequestCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GetUniswapV3TickDataBatchRequestTransactor is an auto generated write-only Go binding around an Ethereum contract.
type GetUniswapV3TickDataBatchRequestTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GetUniswapV3TickDataBatchRequestFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type GetUniswapV3TickDataBatchRequestFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GetUniswapV3TickDataBatchRequestSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type GetUniswapV3TickDataBatchRequestSession struct {
	Contract     *GetUniswapV3TickDataBatchRequest // Generic contract binding to set the session for
	CallOpts     bind.CallOpts                     // Call options to use throughout this session
	TransactOpts bind.TransactOpts                 // Transaction auth options to use throughout this session
}

// GetUniswapV3TickDataBatchRequestCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type GetUniswapV3TickDataBatchRequestCallerSession struct {
	Contract *GetUniswapV3TickDataBatchRequestCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                           // Call options to use throughout this session
}

// GetUniswapV3TickDataBatchRequestTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type GetUniswapV3TickDataBatchRequestTransactorSession struct {
	Contract     *GetUniswapV3TickDataBatchRequestTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                           // Transaction auth options to use throughout this session
}

// GetUniswapV3TickDataBatchRequestRaw is an auto generated low-level Go binding around an Ethereum contract.
type GetUniswapV3TickDataBatchRequestRaw struct {
	Contract *GetUniswapV3TickDataBatchRequest // Generic contract binding to access the raw methods on
}

// GetUniswapV3TickDataBatchRequestCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type GetUniswapV3TickDataBatchRequestCallerRaw struct {
	Contract *GetUniswapV3TickDataBatchRequestCaller // Generic read-only contract binding to access the raw methods on
}

// GetUniswapV3TickDataBatchRequestTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type GetUniswapV3TickDataBatchRequestTransactorRaw struct {
	Contract *GetUniswapV3TickDataBatchRequestTransactor // Generic write-only contract binding to access the raw methods on
}

// NewGetUniswapV3TickDataBatchRequest creates a new instance of GetUniswapV3TickDataBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV3TickDataBatchRequest(address common.Address, backend bind.ContractBackend) (*GetUniswapV3TickDataBatchRequest, error) {
	contract, err := bindGetUniswapV3TickDataBatchRequest(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV3TickDataBatchRequest{GetUniswapV3TickDataBatchRequestCaller: GetUniswapV3TickDataBatchRequestCaller{contract: contract}, GetUniswapV3TickDataBatchRequestTransactor: GetUniswapV3TickDataBatchRequestTransactor{contract: contract}, GetUniswapV3TickDataBatchRequestFilterer: GetUniswapV3TickDataBatchRequestFilterer{contract: contract}}, nil
}

// NewGetUniswapV3TickDataBatchRequestCaller creates a new read-only instance of GetUniswapV3TickDataBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV3TickDataBatchRequestCaller(address common.Address, caller bind.ContractCaller) (*GetUniswapV3TickDataBatchRequestCaller, error) {
	contract, err := bindGetUniswapV3TickDataBatchRequest(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV3TickDataBatchRequestCaller{contract: contract}, nil
}

// NewGetUniswapV3TickDataBatchRequestTransactor creates a new write-only instance of GetUniswapV3TickDataBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV3TickDataBatchRequestTransactor(address common.Address, transactor bind.ContractTransactor) (*GetUniswapV3TickDataBatchRequestTransactor, error) {
	contract, err := bindGetUniswapV3TickDataBatchRequest(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV3TickDataBatchRequestTransactor{contract: contract}, nil
}

// NewGetUniswapV3TickDataBatchRequestFilterer creates a new log filterer instance of GetUniswapV3TickDataBatchRequest, bound to a specific deployed contract.
func NewGetUniswapV3TickDataBatchRequestFilterer(address common.Address, filterer bind.ContractFilterer) (*GetUniswapV3TickDataBatchRequestFilterer, error) {
	contract, err := bindGetUniswapV3TickDataBatchRequest(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &GetUniswapV3TickDataBatchRequestFilterer{contract: contract}, nil
}

// bindGetUniswapV3TickDataBatchRequest binds a generic wrapper to an already deployed contract.
func bindGetUniswapV3TickDataBatchRequest(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(GetUniswapV3TickDataBatchRequestABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_GetUniswapV3TickDataBatchRequest *GetUniswapV3TickDataBatchRequestRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _GetUniswapV3TickDataBatchRequest.Contract.GetUniswapV3TickDataBatchRequestCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_GetUniswapV3TickDataBatchRequest *GetUniswapV3TickDataBatchRequestRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _GetUniswapV3TickDataBatchRequest.Contract.GetUniswapV3TickDataBatchRequestTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_GetUniswapV3TickDataBatchRequest *GetUniswapV3TickDataBatchRequestRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _GetUniswapV3TickDataBatchRequest.Contract.GetUniswapV3TickDataBatchRequestTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_GetUniswapV3TickDataBatchRequest *GetUniswapV3TickDataBatchRequestCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _GetUniswapV3TickDataBatchRequest.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_GetUniswapV3TickDataBatchRequest *GetUniswapV3TickDataBatchRequestTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _GetUniswapV3TickDataBatchRequest.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_GetUniswapV3TickDataBatchRequest *GetUniswapV3TickDataBatchRequestTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _GetUniswapV3TickDataBatchRequest.Contract.contract.Transact(opts, method, params...)
}

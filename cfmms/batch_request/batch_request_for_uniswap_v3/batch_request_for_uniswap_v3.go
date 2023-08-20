package batch_request_for_uniswap_v3

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/deployment/GetUniswapV3PoolDataBatchRequest"
	"hello/cfmms/deployment/GetUniswapV3TickDataBatchRequest"
	"hello/cfmms/deployment/SyncUniswapV3PoolBatchRequest"
	"math/big"
	"strings"
)

func GetPoolDataBatchRequest(targetAddress []common.Address, client *ethclient.Client) ([]byte, error) {

	byteCode := GetUniswapV3PoolDataBatchRequest.GetUniswapV3PoolDataBatchRequestMetaData.Bin

	argsCodeAbi, err := abi.JSON(strings.NewReader(GetUniswapV3PoolDataBatchRequest.GetUniswapV3PoolDataBatchRequestMetaData.ABI))

	if err != nil {
		fmt.Println("abi.JSON error", err)
		return nil, err
	}

	argsByteCode, err := argsCodeAbi.Pack("", targetAddress)
	if err != nil {
		fmt.Println("argsCodeAbi.Pack error", err)
		return nil, err
	}

	callMsg := ethereum.CallMsg{
		Data:       append(common.FromHex(byteCode), argsByteCode...),
		AccessList: nil,
	}

	// 执行 Eth_call
	result, err := client.CallContract(context.Background(), callMsg, nil)
	if err != nil {
		fmt.Println("client.CallContract GetPoolDataBatchRequest error", err)
		return nil, err
	}

	return result, err

}

func GetV3PoolDataBatchRequest(address common.Address, client *ethclient.Client) error {

	return nil
}

type UniswapV3TickData struct {
	Initialized  bool
	Tick         *big.Int
	LiquidityNet *big.Int
}

// for simulate swap
func GetUniV3TickDataBatchRequest(poolAddress common.Address, tickSpacing *big.Int, tickStart *big.Int, zeroForOne *big.Int, numTicks *big.Int, blockNumber *big.Int, client *ethclient.Client) ([]byte, error) {

	// TODO

	byteCode := GetUniswapV3TickDataBatchRequest.GetUniswapV3TickDataBatchRequestMetaData.Bin

	argsCodeAbi, err := abi.JSON(strings.NewReader(GetUniswapV3TickDataBatchRequest.GetUniswapV3TickDataBatchRequestMetaData.ABI))

	if err != nil {
		fmt.Println("abi.JSON error", err)
		return nil, err
	}

	args := []interface{}{poolAddress, zeroForOne, tickStart, numTicks, tickSpacing}

	argsByteCode, err := argsCodeAbi.Pack("", args)
	if err != nil {
		fmt.Println("argsCodeAbi.Pack error", err)
		return nil, err
	}

	callMsg := ethereum.CallMsg{
		Data:       append(common.FromHex(byteCode), argsByteCode...),
		AccessList: nil,
	}

	// 执行 Eth_call
	result, err := client.CallContract(context.Background(), callMsg, nil)
	if err != nil {
		fmt.Println("client.CallContract GetUniswapV3TickDataBatchRequest error", err)
		return nil, err
	}

	return result, nil
}

func SyncV3PoolBatchRequest(address common.Address, client *ethclient.Client) ([]byte, error) {

	// TODO

	byteCode := SyncUniswapV3PoolBatchRequest.SyncUniswapV3PoolBatchRequestMetaData.Bin

	argsCodeAbi, err := abi.JSON(strings.NewReader(SyncUniswapV3PoolBatchRequest.SyncUniswapV3PoolBatchRequestMetaData.ABI))

	if err != nil {
		fmt.Println("abi.JSON error", err)
		return nil, err
	}

	args := address

	argsByteCode, err := argsCodeAbi.Pack("", args)
	if err != nil {
		fmt.Println("argsCodeAbi.Pack error", err)
		return nil, err
	}

	callMsg := ethereum.CallMsg{
		Data:       append(common.FromHex(byteCode), argsByteCode...),
		AccessList: nil,
	}

	// 执行 Eth_call
	result, err := client.CallContract(context.Background(), callMsg, nil)
	if err != nil {
		fmt.Println("client.CallContract GetPoolDataBatchRequest error", err)
		return nil, err
	}

	return result, err

}

package batch_request_for_uniswap_v3

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/deployment/GetUniswapV3PoolDataBatchRequest"
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

func GetV3PoolDataBatchRequest(client *ethclient.Client) error {

	return nil
}

func GetUniswapV3TickDataBatchRequest(client *ethclient.Client) error {
	return nil
}

func SyncV3PoolBatchRequest(client *ethclient.Client) error {
	return nil
}

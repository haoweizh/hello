package batch_request_for_uniswap_v2

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/deployment/GetUniswapV2PairsBatchRequest"
	"hello/cfmms/deployment/GetUniswapV2PoolDataBatchRequest"
	"log"
	"math/big"
	"strings"
)

func Get_pairs_batch_request(factory common.Address, from, setp *big.Int, client *ethclient.Client) []string {
	var pairs []string

	// TODO: eth_call 获取合约返回值

	byteCode := GetUniswapV2PairsBatchRequest.GetUniswapV2PairsBatchRequestMetaData.Bin

	argsCodeAbi, err := abi.JSON(strings.NewReader(GetUniswapV2PairsBatchRequest.GetUniswapV2PairsBatchRequestMetaData.ABI))

	if err != nil {
		log.Fatal(err)
	}

	argsByteCode, _ := argsCodeAbi.Pack("", from, setp, factory)

	callMsg := ethereum.CallMsg{
		Data:       append(common.FromHex(byteCode), argsByteCode...),
		AccessList: nil,
	}

	// 执行Eth_call
	result, err := client.CallContract(context.Background(), callMsg, nil)
	if err != nil {
		log.Fatal(err)
	}

	hexString := hex.EncodeToString(result)
	hexString = hexString[128:]

	for i := 0; i < len(hexString); i += 64 {
		pairs = append(pairs, hexString[i:i+64])
	}

	return pairs

}

// 一次最多请求数量127   len(pool) <= 127
func Get_pool_data_batch_request(pool []string, client *ethclient.Client) {

	var target_addresses []common.Address

	for _, v := range pool {
		target_addresses = append(target_addresses, common.HexToAddress(v))
	}

	byteCode := GetUniswapV2PoolDataBatchRequest.GetUniswapV2PoolDataBatchRequestMetaData.Bin

	argsCodeAbi, err := abi.JSON(strings.NewReader(GetUniswapV2PoolDataBatchRequest.GetUniswapV2PoolDataBatchRequestMetaData.ABI))

	if err != nil {
		log.Fatal("argsCodeAbi ", err)

	}

	fmt.Println("target_addresses ", target_addresses)

	argsByteCode, _ := argsCodeAbi.Pack("", target_addresses)

	fmt.Println("argsByteCode ", argsByteCode)

	temp := append(common.FromHex(byteCode), argsByteCode...)
	fmt.Println("temp ", hex.EncodeToString(temp))
	callMsg := ethereum.CallMsg{
		Data:       append(common.FromHex(byteCode), argsByteCode...),
		AccessList: nil,
	}

	// 执行Eth_call
	result, err := client.CallContract(context.Background(), callMsg, nil)
	fmt.Println("result ", result)
	if err != nil {
		fmt.Println("err ", err)
		log.Fatal(err)
	}

	hexString := hex.EncodeToString(result)
	fmt.Println("Get_pool_data_batch_request reuslt raw", hexString)

	hexString = hexString[128:]

	fmt.Println("Get_pool_data_batch_request reuslt", hexString)

	type PoolData struct {
		TokenA         common.Address
		TokenADecimals int64
		TokenB         common.Address
		TokenBDecimals int64
		Reserve0       *big.Int
		Reserve1       *big.Int
	}

	//for i := 0; i < len(hexString); i += 64 * 6 {
	//
	//	var poolData PoolData
	//
	//	poolData.TokenA = common.HexToAddress(hexString[i : i+64])
	//	poolData.TokenADecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[i+64 : i+128])).Int64())
	//	poolData.TokenB = common.HexToAddress(hexString[i+128 : i+192])
	//	poolData.TokenBDecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[i+192 : i+256])).Int64())
	//	poolData.Reserve0 = new(big.Int).SetBytes(common.FromHex(hexString[i+256 : i+320]))
	//	poolData.Reserve1 = new(big.Int).SetBytes(common.FromHex(hexString[i+320 : i+384]))
	//
	//	fmt.Println("poolData ", poolData)
	//
	//}

	oneStructLen := 64 * 6

	nums := len(hexString) / oneStructLen

	for i := 1; i < nums+1; i++ {

		var poolData PoolData

		poolData.TokenA = common.HexToAddress(hexString[(i-1)*oneStructLen : (i-1)*oneStructLen+64])
		poolData.TokenADecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+64 : (i-1)*oneStructLen+128])).Int64())
		poolData.TokenB = common.HexToAddress(hexString[(i-1)*oneStructLen+128 : (i-1)*oneStructLen+192])
		poolData.TokenBDecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+192 : (i-1)*oneStructLen+256])).Int64())
		poolData.Reserve0 = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+256 : (i-1)*oneStructLen+320]))
		poolData.Reserve1 = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+320 : (i-1)*oneStructLen+384]))
		fmt.Println("poolData ", poolData)
	}

}

func get_v2_pool_data_batch_request() {

}

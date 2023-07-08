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
	"log"
	"math/big"
	"strings"
)

func Get_pairs_batch_request(factory common.Address, from, setp *big.Int, client *ethclient.Client) []string {
	var pairs []string

	// TODO: eth_call 获取合约返回值

	byteCode := GetUniswapV2PairsBatchRequest.GetUniswapV2PairsBatchRequestMetaData.Bin

	fmt.Println(GetUniswapV2PairsBatchRequest.GetUniswapV2PairsBatchRequestMetaData.ABI)

	argsCodeAbi, err := abi.JSON(strings.NewReader(GetUniswapV2PairsBatchRequest.GetUniswapV2PairsBatchRequestMetaData.ABI))

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(factory)
	fmt.Println(from)
	fmt.Println(setp)

	argsByteCode, _ := argsCodeAbi.Pack("", from, setp, factory)

	fmt.Println("argsByteCode========================>", argsByteCode)

	//byteCode := "0x6080604052348015600f57600080fd5b5060405160dc38038060dc8339818101604052810190602d91906068565b506090565b600080fd5b6000819050919050565b6048816037565b8114605257600080fd5b50565b6000815190506062816041565b92915050565b600060208284031215607b57607a6032565b5b60006087848285016055565b91505092915050565b603f80609d6000396000f3fe6080604052600080fdfea26469706673582212208cc017beed578b851e8578d2df31fe36c4108efceefd0f2d8f0be696ca6f85ee64736f6c634300081200330000000000000000000000000000000000000000000000000000000000000001"

	callMsg := ethereum.CallMsg{
		Data:       append(common.FromHex(byteCode), argsByteCode...),
		AccessList: nil,
	}

	// 执行静态调用
	result, err := client.CallContract(context.Background(), callMsg, nil)
	if err != nil {
		log.Fatal(err)
	}

	hexString := hex.EncodeToString(result)
	hexString = hexString[128:]

	for pair := range hexString {
		pairs = append(pairs, hexString[pair*64:(pair+1)*64])
	}

	//fmt.Println("hexString========================>", hexString)
	//
	//fmt.Println("result address ========================>", common.HexToAddress(hexString[:64]))
	//fmt.Println("result address ========================>", common.HexToAddress(hexString[64:128]))
	//fmt.Println("result address ========================>", common.HexToAddress(hexString[128:192]))
	//
	//fmt.Println("result========================>", result)
	//
	//fmt.Println("result========================>", len(result))
	////fmt.Println("result========================>", result)
	////res, _ := argsCodeAbi.Unpack("", result)
	////fmt.Println("Owner: %v", common.BytesToAddress(result).Hex())
	////res3 := FormatHex(hexutil.Encode(result))

	fmt.Println("pairs", pairs)

	return pairs

}

func get_pool_data_batch_request() {

}

func get_v2_pool_data_batch_request() {

}

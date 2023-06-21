package dex

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"
	"log"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	mainnetRpcWS    = "wss://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG"
	mainnetRpcHttps = "https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG"
)

var wg sync.WaitGroup

type DexFactory struct {
	FactoryAddress       common.Address // 工厂合约地址
	StartBlock           *big.Int       // 合约部署的起始区块
	Abi                  string         // 合约ABI
	PoolCreateMethodName string         // 创建交易对的方法名
}

type Dex struct {
	Name    string
	Factory DexFactory
	Pools   []Pool // 该工厂下的所有交易对
}

type Pool struct {
	// TODO: 交易对的信息
}

var precessGetPairFromLogMap = map[string]func(vLog types.Log, dex Dex){
	"UniswapV2": processUniswapV2PairFromLog,
	"UniswapV3": processUniswapV3PairFromLog,
}

func processUniswapV3PairFromLog(vLog types.Log, dex Dex) {

	fmt.Println(dex.Name)
	//	解析日志事件

	contractAbi, err := abi.JSON(strings.NewReader(dex.Factory.Abi))
	if err != nil {
		log.Fatal(err)
	}

	data, err := contractAbi.Unpack(dex.Factory.PoolCreateMethodName, vLog.Data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("univ3poolAddress:", data[0]) // pair address

	var topics [4]string
	for i := range vLog.Topics {
		topics[i] = vLog.Topics[i].Hex()
	}
	fmt.Println(topics[0]) // 0xe79e73da417710ae99aa2088575580a60415d359acfad9cdd3382d59c80281d4
}

func processUniswapV2PairFromLog(vLog types.Log, dex Dex) {
	fmt.Println(dex.Name)
	//	解析日志事件
	contractAbi, err := abi.JSON(strings.NewReader(dex.Factory.Abi))
	if err != nil {
		log.Fatal(err)
	}
	data, err := contractAbi.Unpack(dex.Factory.PoolCreateMethodName, vLog.Data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("合约地址：", data[0]) // pair address

	var topics [4]string
	for i := range vLog.Topics {
		topics[i] = vLog.Topics[i].Hex()
	}
	fmt.Println("token0地址:", topics[0]) // 0xe79e73da417710ae99aa2088575580a60415d359acfad9cdd3382d59c80281d4
	fmt.Println("token1地址:", topics[1]) // 0xe79e73da417710ae99aa2088575580a60415d359acfad9cdd3382d59c80281d4

	// todo: 通过token0和token1地址获取token信息
}

func initDexs() []Dex {

	dexs := []Dex{
		{
			Name: "UniswapV2",
			Factory: DexFactory{
				FactoryAddress:       common.HexToAddress(ToChecksumAddress("0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f")),
				StartBlock:           big.NewInt(10000835),
				PoolCreateMethodName: "PairCreated",
				Abi:                  "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_feeToSetter\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token0\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token1\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"pair\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"PairCreated\",\"type\":\"event\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"allPairs\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"allPairsLength\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenA\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenB\",\"type\":\"address\"}],\"name\":\"createPair\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"pair\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"feeTo\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"feeToSetter\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"getPair\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_feeTo\",\"type\":\"address\"}],\"name\":\"setFeeTo\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"internalType\":\"address\",\"name\":\"_feeToSetter\",\"type\":\"address\"}],\"name\":\"setFeeToSetter\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
			},
		},
		{
			Name: "UniswapV3",
			Factory: DexFactory{
				FactoryAddress:       common.HexToAddress(ToChecksumAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984")),
				StartBlock:           big.NewInt(12369621),
				PoolCreateMethodName: "PoolCreated",
				Abi:                  "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint24\",\"name\":\"fee\",\"type\":\"uint24\"},{\"indexed\":true,\"internalType\":\"int24\",\"name\":\"tickSpacing\",\"type\":\"int24\"}],\"name\":\"FeeAmountEnabled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"oldOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnerChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token0\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token1\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint24\",\"name\":\"fee\",\"type\":\"uint24\"},{\"indexed\":false,\"internalType\":\"int24\",\"name\":\"tickSpacing\",\"type\":\"int24\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"}],\"name\":\"PoolCreated\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenA\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenB\",\"type\":\"address\"},{\"internalType\":\"uint24\",\"name\":\"fee\",\"type\":\"uint24\"}],\"name\":\"createPool\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint24\",\"name\":\"fee\",\"type\":\"uint24\"},{\"internalType\":\"int24\",\"name\":\"tickSpacing\",\"type\":\"int24\"}],\"name\":\"enableFeeAmount\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint24\",\"name\":\"\",\"type\":\"uint24\"}],\"name\":\"feeAmountTickSpacing\",\"outputs\":[{\"internalType\":\"int24\",\"name\":\"\",\"type\":\"int24\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint24\",\"name\":\"\",\"type\":\"uint24\"}],\"name\":\"getPool\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"parameters\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"factory\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token0\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token1\",\"type\":\"address\"},{\"internalType\":\"uint24\",\"name\":\"fee\",\"type\":\"uint24\"},{\"internalType\":\"int24\",\"name\":\"tickSpacing\",\"type\":\"int24\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"setOwner\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
			},
		},
	}

	return dexs
}

func scanLogFromRpc(dex Dex, blockSpace *big.Int) {
	defer wg.Done()
	client, err := ethclient.Dial(mainnetRpcWS)
	if err != nil {
		log.Fatal(err)
	}
	currencyBlockNumber, _ := client.BlockNumber(context.Background())
	fmt.Println(currencyBlockNumber)
	contractAddress := dex.Factory.FactoryAddress

	for startBlock := dex.Factory.StartBlock; startBlock.Cmp(big.NewInt(int64(currencyBlockNumber))) < 0; startBlock.Add(startBlock, blockSpace) {

		fmt.Println("StartBlock", startBlock.String())
		var endBlock *big.Int
		if big.NewInt(0).Add(startBlock, blockSpace).Cmp(big.NewInt(int64(currencyBlockNumber))) > 0 {
			endBlock = big.NewInt(int64(currencyBlockNumber))
		} else {
			endBlock = big.NewInt(0).Add(startBlock, blockSpace)
		}

		fmt.Println("EndBlock", endBlock.String())

		query := ethereum.FilterQuery{
			FromBlock: startBlock,
			ToBlock:   endBlock,
			Addresses: []common.Address{
				contractAddress,
			},
		}
		fmt.Println("当前扫描地址是", contractAddress)

		logs, err := client.FilterLogs(context.Background(), query)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(fmt.Sprintf("从 %v 到 %v 扫描到的日志数量 %v", startBlock.String(), endBlock.String(), len(logs)))

		for _, vLog := range logs {

			if len(vLog.Data) == 0 {
				continue
			}

			if fn, ok := precessGetPairFromLogMap[dex.Name]; ok {
				fn(vLog, dex)
			} else {
				fmt.Println("未找到处理函数")
			}

		}
		time.Sleep(1 * time.Millisecond)
	}
}

func GetAllPools() {
	// TODO: 从RPC获取所有的交易对
	// setp1: 获取所有的dex
	dexs := initDexs()
	// setp2: 遍历dex，获取所有的交易对
	fmt.Println(dexs)

	wg.Add(len(dexs))

	for _, value := range dexs {
		go scanLogFromRpc(value, big.NewInt(10000))
	}
	wg.Wait()
}

// tools Funcs
func ToChecksumAddress(address string) string {
	address = strings.Replace(strings.ToLower(address), "0x", "", 1)
	hash := sha3.NewLegacyKeccak256()
	_, _ = hash.Write([]byte(address))
	sum := hash.Sum(nil)
	digest := hex.EncodeToString(sum)

	b := strings.Builder{}
	b.WriteString("0x")

	for i := 0; i < len(address); i++ {
		a := address[i]
		if a > '9' {
			d, _ := strconv.ParseInt(digest[i:i+1], 16, 8)

			if d >= 8 {
				// Upper case it
				a -= 'a' - 'A'
				b.WriteByte(a)
			} else {
				// Keep it lower
				b.WriteByte(a)
			}
		} else {
			// Keep it lower
			b.WriteByte(a)
		}
	}

	return b.String()
}

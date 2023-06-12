package dex

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"
	"log"
	"math/big"
	"strconv"
	"strings"
)

var (
	mainnetRpcWS    = "wss://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG"
	mainnetRpcHttps = "https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG"
)

type DexFactory struct {
	FactoryAddress common.Address // 工厂合约地址
	StartBlock     *big.Int       // 合约部署的起始区块
}

type Dex struct {
	Name    string
	Factory DexFactory
	Pools   []Pool // 该工厂下的所有交易对
}

type Pool struct {
	// TODO: 交易对的信息
}

func initDexs() []Dex {
	dexs := []Dex{
		{
			Name: "UniswapV2",
			Factory: DexFactory{
				FactoryAddress: common.HexToAddress(ToChecksumAddress("0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f")),
				StartBlock:     big.NewInt(0),
			},
		},
		{
			Name: "UniswapV3",
			Factory: DexFactory{
				FactoryAddress: common.HexToAddress(ToChecksumAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984")),
				StartBlock:     big.NewInt(0),
			},
		},
	}
	return dexs
}

func scanLogFromRpc(dex Dex) {
	client, err := ethclient.Dial(mainnetRpcWS)
	if err != nil {
		log.Fatal(err)
	}

	contractAddress := dex.Factory.FactoryAddress
	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
	}

	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case vLog := <-logs:
			fmt.Println(vLog) // pointer to event log
		}
	}
}

func getAllPools() {
	// TODO: 从RPC获取所有的交易对
	// setp1: 获取所有的dex
	dexs := initDexs()
	// setp2: 遍历dex，获取所有的交易对

	for _, dex := range dexs {
		go scanLogFromRpc(dex)
	}

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

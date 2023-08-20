package cfmms

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/dex"
	"hello/cfmms/pool"
	"math/big"
)

func SyncPairsTest() {

	//client, err := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	client, err := ethclient.Dial("http://188.40.132.112:8545")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}

	var initDex []dex.Dex

	//initDex = append(initDex, &dex.UniswapV2Dex{
	//	FactoryAddress: common.HexToAddress("0x5c69bee701ef814a2b6a3edd4b1652cb9cc5aa6f"),
	//	CreationBlock:  10000835,
	//})

	initDex = append(initDex, &dex.UniswapV3Dex{
		FactoryAddress: common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984"),
		CreationBlock:  big.NewInt(12369621),
	})

	SyncPairs(initDex, client, "./")
	//v2pool := CreateNewPool(&pool.UniswapV2Pool{
	//	Address: common.HexToAddress("0x88d97d199b9ed37c29d846d00d443de980832a22"),
	//}, client)
	//fmt.Println(v2pool)
}

func CreateNewPool(poolType pool.Pool, client *ethclient.Client) any {

	poolIns, err := poolType.NewFromAddress(poolType.GetAddress(), client)
	if err != nil {
		fmt.Println("NewFromAddress error")
		return nil
	}
	return poolIns

}

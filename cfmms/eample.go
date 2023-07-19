package cfmms

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/dex"
)

func SyncPairsTest() {

	client, err := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	//client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}

	var initDex []dex.Dex

	initDex = append(initDex, &dex.UniswapV2Dex{
		FactoryAddress: common.HexToAddress("0x5c69bee701ef814a2b6a3edd4b1652cb9cc5aa6f"),
		CreationBlock:  10000835,
	})

	SyncPairs(initDex, client, "./")

}

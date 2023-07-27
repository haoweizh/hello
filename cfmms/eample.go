package cfmms

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/dex"
	"math/big"
)

func SyncPairsTest() {

	client, err := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	//client, err := ethclient.Dial("http://localhost:8545")
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

}

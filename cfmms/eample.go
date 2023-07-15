package cfmms

import (
	"github.com/ethereum/go-ethereum/common"
	"hello/cfmms/dex"
)

func SyncPairsTest() {
	//TODO implement me
	var initDex []dex.Dex

	initDex = append(initDex, &dex.UniswapV2Dex{
		FactoryAddress: common.HexToAddress("0x5c69bee701ef814a2b6a3edd4b1652cb9cc5aa6f"),
		CreationBlock:  10000835,
	})

	for _, d := range initDex {
		d.GetAllPools()
	}

}

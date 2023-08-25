package cfmms

import (
	"encoding/json"
	"fmt"
	"hello/cfmms/dex"
	"hello/cfmms/pool"
	"os"
	"sync"
	"time"
)

func SyncPoolsFromCheckpoint() {

}

func ConstructCheckpoint(dexes []dex.Dex, aggregatedPools *sync.Map, latest_block uint64, checkpoint_path string) {

	// save in sql

	mp := make(map[string]any)

	mp["checkpoint_timestamp"] = time.Now().Unix()
	mp["block_number"] = latest_block

	// Add dexes to checkpoint
	dexesArray := make([]map[string]interface{}, 0)
	for _, dexIns := range dexes {
		dex_map := make(map[string]interface{})
		dex_map["factory_address"] = dexIns.GetFactoryAddress()
		dex_map["block_number"] = latest_block

		switch dexIns.(type) {
		case *dex.UniswapV2Dex:
			dex_map["dex_variant"] = "UniswapV2"
			dex_map["fee"] = dexIns.(*dex.UniswapV2Dex).Fee

		case *dex.UniswapV3Dex:
			dex_map["dex_variant"] = "UniswapV3"
		}
		dexesArray = append(dexesArray, dex_map)
	}

	mp["dexes"] = dexesArray

	pools_array := make([]map[string]interface{}, 0)

	aggregatedPools.Range(func(key, p interface{}) bool {
		pool_map := make(map[string]interface{})
		switch p.(type) {
		case pool.UniswapV2Pool:
			pool_map["pool_variant"] = "UniswapV2"
			pool_map["address"] = p.(pool.UniswapV2Pool).Address
			pool_map["token_a"] = p.(pool.UniswapV2Pool).TokenA
			pool_map["token_b"] = p.(pool.UniswapV2Pool).TokenB
			pool_map["fee"] = p.(pool.UniswapV2Pool).Fee
			pool_map["token_a_decimals"] = p.(pool.UniswapV2Pool).TokenADecimals
			pool_map["token_b_decimals"] = p.(pool.UniswapV2Pool).TokenBDecimals

			pools_array = append(pools_array, pool_map)

		case pool.UniswapV3Pool:
			pool_map["dex_variant"] = "UniswapV3"
			pool_map["address"] = p.(pool.UniswapV3Pool).Address
			pool_map["token_a"] = p.(pool.UniswapV3Pool).TokenA
			pool_map["token_a_decimals"] = p.(pool.UniswapV3Pool).TokenB
			pool_map["token_b"] = p.(pool.UniswapV3Pool).TokenADecimals
			pool_map["token_b_decimals"] = p.(pool.UniswapV3Pool).TokenBDecimals
			pool_map["liquidity"] = p.(pool.UniswapV3Pool).Liquidity
			pool_map["sqrt_price"] = p.(pool.UniswapV3Pool).SqrtPrice
			pool_map["fee"] = p.(pool.UniswapV3Pool).Fee
			pool_map["tick"] = p.(pool.UniswapV3Pool).Tick
			pool_map["tick_spacing"] = p.(pool.UniswapV3Pool).TickSpacing
			pool_map["liquidity_net"] = p.(pool.UniswapV3Pool).LiquidityNet

			pools_array = append(pools_array, pool_map)

		}
		pools_array = append(pools_array, pool_map)

		return true
	})

	fmt.Println("pools_array", pools_array)

	mp["pools"] = pools_array

	dataBytes, err := json.Marshal(mp)

	if err != nil {
		fmt.Println("序列化时发生错误:", err)
		return
	}

	//保存 abi
	err = os.WriteFile(checkpoint_path+"checkpoint.json", dataBytes, 0644)
	if err != nil {
		fmt.Println("写入checkpoint文件时发生错误:", err)
		return
	}

}

func DeConstructCheckpoint(checkpoint_path string) {
}

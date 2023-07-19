package cfmms

import (
	"encoding/json"
	"fmt"
	"hello/cfmms/dex"
	"hello/cfmms/pool"
	"os"
	"time"
)

func SyncPoolsFromCheckpoint() {

}

func ConstructCheckpoint(dexes []dex.Dex, pools []pool.Pool, latest_block uint64, checkpoint_path string) {

	mp := make(map[string]any)

	mp["checkpoint_timestamp"] = time.Now()
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
	for _, p := range pools {
		pool_map := make(map[string]interface{})

		switch p.(type) {
		case *pool.UniswapV2Pool:
			pool_map["pool_variant"] = "UniswapV2"
			pool_map["address"] = p.(*pool.UniswapV2Pool).Address
			pool_map["token_a"] = p.(*pool.UniswapV2Pool).TokenA
			pool_map["token_b"] = p.(*pool.UniswapV2Pool).TokenB
			pool_map["fee"] = p.(*pool.UniswapV2Pool).Fee
			pool_map["token_a_decimals"] = p.(*pool.UniswapV2Pool).TokenADecimals
			pool_map["token_b_decimals"] = p.(*pool.UniswapV2Pool).TokenBDecimals

			pools_array = append(pools_array, pool_map)
		case *pool.UniswapV3Pool:
			pool_map["dex_variant"] = "UniswapV3"
			pool_map["address"] = p.(*pool.UniswapV3Pool).Address
			pool_map["token_a"] = p.(*pool.UniswapV3Pool).TokenA
			pool_map["token_a_decimals"] = p.(*pool.UniswapV3Pool).TokenB
			pool_map["token_b"] = p.(*pool.UniswapV3Pool).TokenADecimals
			pool_map["token_b_decimals"] = p.(*pool.UniswapV3Pool).TokenBDecimals
			pool_map["liquidity"] = p.(*pool.UniswapV3Pool).Liquidity
			pool_map["sqrt_price"] = p.(*pool.UniswapV3Pool).SqrtPrice
			pool_map["fee"] = p.(*pool.UniswapV3Pool).Fee
			pool_map["tick"] = p.(*pool.UniswapV3Pool).Tick
			pool_map["tick_spacing"] = p.(*pool.UniswapV3Pool).TickSpacing
			pool_map["liquidity_net"] = p.(*pool.UniswapV3Pool).LiquidityNet

			pools_array = append(pools_array, pool_map)
		}
		pools_array = append(pools_array, pool_map)
	}
	// Todo: 写入文件

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

	//	let mut dexes = vec![];
	//
	//	let checkpoint_json: serde_json::Value = serde_json::from_str(
	//		read_to_string(checkpoint_path)
	//	.expect("Error when reading in checkpoint json")
	//	.as_str(),
	//)
	//	.expect("Error when converting checkpoint file contents to serde_json::Value");
	//
	//	let block_number = checkpoint_json
	//	.get("block_number")
	//	.expect("Could not get block_number from checkpoint")
	//	.as_u64()
	//	.expect("Could not convert block_number to u64");
	//
	//	for dex_data in checkpoint_json
	//	.get("dexes")
	//	.expect("Could not get checkpoint_data")
	//	.as_array()
	//	.expect("Could not unwrap checkpoint json into array")
	//	.iter()
	//	{
	//		let dex = deconstruct_dex_from_checkpoint(
	//		dex_data
	//		.as_object()
	//		.expect("Dex checkpoint is not formatted correctly"),
	//	);
	//
	//		dexes.push(dex);
	//	}
	//
	//	//get all pools
	//	let pools_array = checkpoint_json
	//	.get("pools")
	//	.expect("Could not get pools from checkpoint")
	//	.as_array()
	//	.expect("Could not convert pools to value array");
	//
	//	let pools = deconstruct_pools_from_checkpoint(pools_array);
	//
	//	(dexes, pools, BlockNumber::Number(block_number.into()))

	//dexex := make([]dex.Dex, 0)

}

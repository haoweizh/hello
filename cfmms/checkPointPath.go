package cfmms

import (
	"hello/cfmms/dex"
	"hello/cfmms/pool"
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
			dex_map["dex_name"] = "UniswapV2"


	}

}

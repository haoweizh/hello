package cfmms

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/dex"
	"hello/cfmms/pool"
)

func SyncPairs(dexes []dex.Dex, client *ethclient.Client, checkpoint_path string) {

	SyncPairsWithThrottle(dexes, 100000, client, 0, checkpoint_path)
}

func SyncPairsWithThrottle(dexes []dex.Dex, step uint64, client *ethclient.Client, requests_per_second_limit uint64, checkpoint_path string) {

}

func SyncPairsWithStep() {

}

func RemoveEmptyPools(pools []pool.Pool) []pool.Pool {
	//let mut cleaned_pools = vec![];
	//
	//for pool in pools {
	//	match pool {
	//	Pool::UniswapV2(uniswap_v2_pool) => {
	//	if !uniswap_v2_pool.token_a.is_zero() {
	//	cleaned_pools.push(pool)
	//}
	//}
	//	Pool::UniswapV3(uniswap_v3_pool) => {
	//	if !uniswap_v3_pool.token_a.is_zero() {
	//	cleaned_pools.push(pool)
	//}
	//}
	//}
	//}
	//
	//cleaned_pools

	var cleaned_pools []pool.Pool

	for _, p := range pools {
		if p.(*pool.UniswapV2Pool).TokenA != Address0 {
			cleaned_pools = append(cleaned_pools, p)
		}
		if p.(*pool.UniswapV3Pool).TokenA != Address0 {
			cleaned_pools = append(cleaned_pools, p)
		}

	}

	return cleaned_pools
}

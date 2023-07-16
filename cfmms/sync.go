package cfmms

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/dex"
	"hello/cfmms/pool"
	"hello/cfmms/utils"
)

func SyncPairs(dexes []dex.Dex, client *ethclient.Client, checkpoint_path string) {

	SyncPairsWithThrottle(dexes, 100000, client, 0, checkpoint_path)
}

func SyncPairsWithThrottle(dexes []dex.Dex, step uint64, client *ethclient.Client, requests_per_second_limit uint64, checkpoint_path string) {

}

func SyncPairsWithStep() {

}

func RemoveEmptyPools(pools []pool.Pool) []pool.Pool {

	var cleaned_pools []pool.Pool

	for _, p := range pools {

		switch p.(type) {
		case *pool.UniswapV2Pool:
			if p.(*pool.UniswapV2Pool).TokenA != utils.Address0 {
				cleaned_pools = append(cleaned_pools, p)
			}
		case *pool.UniswapV3Pool:
			if p.(*pool.UniswapV3Pool).TokenA != utils.Address0 {
				cleaned_pools = append(cleaned_pools, p)
			}
		}

	}

	return cleaned_pools
}

package cfmms

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/dex"
	"hello/cfmms/pool"
	"hello/cfmms/utils"
	"sync"
)

func SyncPairs(dexes []dex.Dex, client *ethclient.Client, checkpoint_path string) {

	SyncPairsWithThrottle(dexes, 100000, client, 0, checkpoint_path)
}

func SyncPairsWithThrottle(dexes []dex.Dex, step uint64, client *ethclient.Client, requests_per_second_limit uint64, checkpoint_path string) {

	wg := sync.WaitGroup{}

	currentBlock, _ := client.BlockNumber(context.Background())

	request_throttle := utils.NewThrottle(requests_per_second_limit)

	var aggregatedPools []pool.Pool

	for _, dexIns := range dexes {
		wg.Add(1)

		go func(dexIns dex.Dex) {
			defer wg.Done()

			// Get all pools from the dex
			pools, err := dexIns.GetAllPools(request_throttle, client, step)
			if err != nil {
				// Handle error
				fmt.Println(err)
				return
			}

			// Get all pool data and sync the pool
			err = dexIns.GetAllPoolsData(pools.(*[]pool.Pool), request_throttle, client)
			if err != nil {
				// Handle error
				fmt.Println(err)
				return
			}

			// Clean empty pools
			pools = RemoveEmptyPools(pools.([]pool.Pool))

			// Append pools to aggregatedPools
			aggregatedPools = append(aggregatedPools, pools.([]pool.Pool)...)

		}(dexIns)
	}
	wg.Wait()

	if checkpoint_path != "" {
		ConstructCheckpoint(dexes, aggregatedPools, currentBlock, checkpoint_path)
	}

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

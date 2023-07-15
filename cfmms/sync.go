package cfmms

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/dex"
)

func SyncPairs(dexes []dex.Dex, client *ethclient.Client, checkpoint_path string) {

	SyncPairsWithThrottle(dexes, 100000, client, 0, checkpoint_path)
}

func SyncPairsWithThrottle(dexes []dex.Dex, step uint64, client *ethclient.Client, requests_per_second_limit uint64, checkpoint_path string) {

}

func SyncPairsWithStep() {

}

func RemoveEmptyPools() {

}

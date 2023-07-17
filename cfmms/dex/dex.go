package dex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms"
	"hello/cfmms/pool"
)

type Dex interface {
	PoolCreatedEventSignature() []byte
	NewPoolFromEvent(address common.Address, client *ethclient.Client)
	NewEmptyPoolFromEvent(log any)
	GetAllPools(requestThrottle *cfmms.Throttle, client *ethclient.Client, step uint64) ([]pool.Pool, error)
	GetAllPoolsData(pool *[]pool.Pool, requestThrottle *cfmms.Throttle, client *ethclient.Client) error

	GetPoolWithBestLiquidity()
	GetAllPoolsForPair()
	GetAllPoolsFromLogs()
	GetAllPoolsFromLogsWithinRange()
	GetFactoryAddress() common.Address
}

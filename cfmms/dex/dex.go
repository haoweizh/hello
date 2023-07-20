package dex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/pool"
	"hello/cfmms/utils"
)

type Dex interface {
	PoolCreatedEventSignature() []byte
	NewPoolFromEvent(address common.Address, client *ethclient.Client)
	NewEmptyPoolFromEvent(log any) any
	GetAllPools(requestThrottle *utils.Throttle, client *ethclient.Client, step uint64) (any, error)
	GetAllPoolsData(pool *[]pool.Pool, requestThrottle *utils.Throttle, client *ethclient.Client) error

	GetPoolWithBestLiquidity()
	GetAllPoolsForPair()
	GetAllPoolsFromLogsWithinRange()
	GetFactoryAddress() common.Address
}

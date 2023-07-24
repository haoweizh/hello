package dex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/pool"
)

type Dex interface {
	PoolCreatedEventSignature() []byte
	NewPoolFromEvent(address common.Address, client *ethclient.Client)
	NewEmptyPoolFromEvent(log any) any
	GetAllPools(client *ethclient.Client, step int64) (any, error)
	GetAllPoolsData(pool *[]pool.Pool, client *ethclient.Client) error

	GetPoolWithBestLiquidity()
	GetAllPoolsForPair()
	GetAllPoolsFromLogsWithinRange()
	GetFactoryAddress() common.Address
}

package pool

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
)

type PoolType int

const (
	UniswapV2PoolType PoolType = iota
	UniswapV3PoolType
)

type Pool interface {
	GetPoolType() PoolType
	NewFromAddress(address string)
	NewFromEventLog(log any)
	NewEmptyPoolFromEventLog(log any)

	// TODO: add more functions
	SyncPool() (err error)
	CalculatePrice() (price float64)
	GetPoolData(client *ethclient.Client) (pool batch_request_for_uniswap_v2.PoolData)
	GetAddress()
	SimulateSwap()
	SimulateSwapMut()
}

type UniswapV3Pool struct {
	FactoryAddress string
	Token0Address  string
}

func (pool *UniswapV3Pool) GetPoolType() PoolType {
	return UniswapV3PoolType
}

func ConvertToDecimals() {

}

func ConvertToCommonDecimals() {

}

func simulateRoute() {

}

func simulateRouteMut() {

}

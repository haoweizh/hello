package pool

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Pool interface {
	NewFromAddress(address common.Address, client *ethclient.Client) Pool
	NewFromEventLog(address common.Address, client *ethclient.Client)
	NewEmptyPoolFromEventLog(log any)

	// TODO: add more functions
	DataIsPopulated() bool
	SyncPool() (err error)
	CalculatePrice() (price float64)
	GetPoolData(address common.Address, client *ethclient.Client)
	GetAddress() any
	SimulateSwap()
	SimulateSwapMut()
}

type UniswapV3Pool struct {
	FactoryAddress string
	Token0Address  string
}

func ConvertToDecimals() {

}

func ConvertToCommonDecimals() {

}

func simulateRoute() {

}

func simulateRouteMut() {

}

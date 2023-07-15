package pool

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Pool interface {
	NewFromAddress(address common.Address, client *ethclient.Client) (any, error)
	NewFromEventLog(address common.Address, client *ethclient.Client)
	NewEmptyPoolFromEventLog(log any)

	// TODO: add more functions
	DataIsPopulated() bool
	SyncPool() (err error)
	CalculatePrice() (price float64)
	GetPoolData(address common.Address, client *ethclient.Client) error
	GetAddress() any
	SimulateSwap()
	SimulateSwapMut()
}

func ConvertToDecimals() {

}

func ConvertToCommonDecimals() {

}

func simulateRoute() {

}

func simulateRouteMut() {

}

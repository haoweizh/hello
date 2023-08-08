package pool

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

type Pool interface {
	NewFromAddress(address common.Address, client *ethclient.Client) (any, error)
	NewFromEventLog(address common.Address, client *ethclient.Client)
	NewEmptyPoolFromEventLog(log any)

	// TODO: add more functions
	DataIsPopulated() bool
	SyncPool() (err error)
	CalculatePrice(baseToken common.Address)
	GetPoolData(client *ethclient.Client) error
	GetAddress() common.Address
	SimulateSwap(tokenIn common.Address, amoutnIn *big.Int)
	SimulateSwapMut(tokenIn common.Address, amountIn *big.Int) uint64
}

func ConvertToDecimals() {

}

func ConvertToCommonDecimals() {

}

func simulateRoute() {

}

func simulateRouteMut() {

}

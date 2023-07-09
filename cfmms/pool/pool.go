package pool

type PoolType int

const (
	UniswapV2PoolType PoolType = iota
	UniswapV3PoolType
)

type Pool interface {
	GetPoolType() PoolType
	NewFromAddress(address string) (pool Pool, err error)
	NewFromEventLog(log any) (pool Pool, err error)
	NewEmptyPoolFromEventLog(log any) (pool Pool, err error)

	// TODO: add more functions
	SyncPool() (err error)
	CalculatePrice() (price float64)
	GetPoolData()
	Address()
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

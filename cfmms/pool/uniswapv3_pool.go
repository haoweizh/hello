package pool

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v3"
	"math/big"
)

type UniswapV3Pool struct {
	Address        common.Address
	TokenA         common.Address
	TokenADecimals int64
	TokenB         common.Address
	TokenBDecimals int64
	Liquidity      *big.Int
	SqrtPrice      *big.Int
	Fee            *big.Int
	Tick           *big.Int
	TickSpacing    *big.Int
	LiquidityNet   *big.Int
}

func (u *UniswapV3Pool) NewFromAddress(address common.Address, client *ethclient.Client) (any, error) {
	//  从地址创建一个新的UniswapV3Pool
	v3pool := UniswapV3Pool{
		Address:        address,
		TokenA:         nil,
		TokenADecimals: 0,
		TokenB:         nil,
		TokenBDecimals: 0,
		Liquidity:      nil,
		SqrtPrice:      nil,
		Fee:            nil,
		Tick:           nil,
		TickSpacing:    nil,
		LiquidityNet:   nil,
	}
	v3pool.GetPoolData(address, client)

	return v3pool, nil
}

func (u *UniswapV3Pool) NewFromEventLog(address common.Address, client *ethclient.Client) {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Pool) NewEmptyPoolFromEventLog(log any) {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Pool) DataIsPopulated() bool {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Pool) SyncPool() (err error) {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Pool) CalculatePrice() (price float64) {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Pool) GetPoolData(address common.Address, client *ethclient.Client) error {
	get_v3_pool_data_batch_request := batch_request_for_uniswap_v3.GetV3PoolDataBatchRequest(address, client)
	return nil
}

func (u *UniswapV3Pool) GetAddress() any {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Pool) SimulateSwap() {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Pool) SimulateSwapMut() {
	//TODO implement me
	panic("implement me")
}

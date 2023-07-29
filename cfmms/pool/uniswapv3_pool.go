package pool

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v3"
	"hello/cfmms/utils"
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
		TokenA:         common.Address{},
		TokenADecimals: 0,
		TokenB:         common.Address{},
		TokenBDecimals: 0,
		Liquidity:      nil,
		SqrtPrice:      nil,
		Fee:            nil,
		Tick:           nil,
		TickSpacing:    nil,
		LiquidityNet:   nil,
	}
	v3pool.GetPoolData(client)

	fmt.Println("v3pool:", v3pool)
	fmt.Println("v3pool:", v3pool.LiquidityNet)
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

func (u *UniswapV3Pool) GetPoolData(client *ethclient.Client) error {
	result, _ := batch_request_for_uniswap_v3.GetPoolDataBatchRequest([]common.Address{
		u.Address,
	}, client)

	hexString := hex.EncodeToString(result)

	hexString = hexString[128:]

	oneStructLen := 64 * 10 // solidity struct length

	nums := len(hexString) / oneStructLen

	for i := 1; i < nums+1; i++ {
		u.TokenA = common.HexToAddress(hexString[(i-1)*oneStructLen : (i-1)*oneStructLen+64])
		u.TokenADecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+64 : (i-1)*oneStructLen+128])).Int64())
		u.TokenB = common.HexToAddress(hexString[(i-1)*oneStructLen+128 : (i-1)*oneStructLen+192])
		u.TokenBDecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+192 : (i-1)*oneStructLen+256])).Int64())
		u.Liquidity = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+256 : (i-1)*oneStructLen+320]))
		u.SqrtPrice = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+320 : (i-1)*oneStructLen+384]))

		// tick 为负数处理
		tickData, isNegative := utils.BigIntIsNegative(common.FromHex(hexString[(i-1)*oneStructLen+384 : (i-1)*oneStructLen+448]))

		u.Tick = new(big.Int).SetBytes(tickData)
		if isNegative {
			u.Tick = new(big.Int).Neg(u.Tick)
		}
		u.TickSpacing = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+448 : (i-1)*oneStructLen+512]))
		u.Fee = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+512 : (i-1)*oneStructLen+576]))
		u.LiquidityNet = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+576 : (i-1)*oneStructLen+640]))
	}

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

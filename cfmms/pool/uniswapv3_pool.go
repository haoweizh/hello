package pool

import (
	"encoding/hex"
	"fmt"
	v3sdkUtils "github.com/daoleno/uniswapv3-sdk/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
	"hello/cfmms/abi_go/Erc20"
	"hello/cfmms/abi_go/UniswapV3PoolSol"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v3"
	"hello/cfmms/utils"
	"math"
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

//pub const MIN_SQRT_RATIO: U256 = U256([4295128739, 0, 0, 0]);
//pub const MAX_SQRT_RATIO: U256 = U256([6743328256752651558, 17280870778742802505, 4294805859, 0]);
//pub const SWAP_EVENT_SIGNATURE: H256 = H256([
//196, 32, 121, 249, 74, 99, 80, 215, 230, 35, 95, 41, 23, 73, 36, 249, 40, 204, 42, 200, 24,
//235, 100, 254, 216, 0, 78, 17, 95, 188, 202, 103,
//]);
//
//pub const U256_TWO: U256 = U256([2, 0, 0, 0]);
//pub const Q128: U256 = U256([0, 0, 1, 0]);
//pub const Q224: U256 = U256([0, 0, 0, 4294967296]);

var (
	MIN_RATIO      = big.NewInt(4295128739)
	MIN_SQRT_RATIO = MIN_RATIO.Lsh(MIN_RATIO, 192)
	MAX_RATIO      = big.NewInt(6743328256752651558)
	MAX_SQRT_RATIO = MAX_RATIO.Lsh(MAX_RATIO, 192)
)

const (
	MinTick = -887272  // The minimum tick that can be used on any pool.
	MaxTick = -MinTick // The maximum tick that can be used on any pool.
)

var (
	Q32             = big.NewInt(1 << 32)
	Q128, _         = big.NewInt(0).SetString("340282366920938463463374607431768211456", 10)
	Q224, _         = big.NewInt(0).SetString("26959946667150639794667015087019630673637144422540572481103610249216", 10)
	MinSqrtRatio    = big.NewInt(4295128739)                                                          // The sqrt ratio corresponding to the minimum tick that could be used on any pool.
	MaxSqrtRatio, _ = new(big.Int).SetString("1461446703485210103287273052203988822378723970342", 10) // The sqrt ratio corresponding to the maximum tick that could be used on any pool.
)

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
	return u.TokenA != utils.Address0 && u.TokenB != utils.Address0
}

func (u *UniswapV3Pool) SyncPool(address common.Address, client *ethclient.Client) (err error) {
	result, err := batch_request_for_uniswap_v3.SyncV3PoolBatchRequest(address, client)
	if err != nil {
		fmt.Println("SyncPool err:", err)
		return nil
	}
	fmt.Println("result:", result)

	// TODO: 解析result里面的数据
	return
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

func (u *UniswapV3Pool) GetAddress() common.Address {
	return u.Address
}

func (u *UniswapV3Pool) CalculatePrice(baseToken common.Address) float64 {

	tick, _ := v3sdkUtils.GetTickAtSqrtRatio(u.SqrtPrice)
	fmt.Println("tick", tick)
	shift := u.TokenADecimals - u.TokenBDecimals
	price := 0.0
	b := decimal.NewFromFloat(1.0001)
	if shift < 0 {

		price, _ = b.Pow(decimal.NewFromFloat(float64(tick))).Div(decimal.NewFromFloat(math.Pow(10, float64(-shift)))).Float64()
	} else {
		price, _ = b.Pow(decimal.NewFromFloat(float64(tick))).Mul(decimal.NewFromFloat(math.Pow(10, float64(shift)))).Float64()
	}

	if baseToken == u.TokenA {
		return price
	} else {
		return 1.0 / price
	}
	return price

}

func (u *UniswapV3Pool) SimulateSwap(tokenIn common.Address, amoutnIn *big.Int) {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Pool) SimulateSwapMut(tokenIn common.Address, amountIn *big.Int) uint64 {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Pool) getTickWord(tick int32, client *ethclient.Client) *big.Int {

	//let v3_pool = abi::IUniswapV3Pool::new(self.address, middleware);
	//let (word_position, _) = uniswap_v3_math::tick_bit_map::position(tick);
	//Ok(v3_pool.tick_bitmap(word_position).call().await?)

	ins, err := UniswapV3PoolSol.NewUniswapV3PoolSol(u.Address, client)

	if err != nil {
		fmt.Println("getTickWord err:", err)
		return nil
	}

	word_position, _ := position(tick)

	data, err := ins.TickBitmap(&bind.CallOpts{}, word_position)
	if err != nil {
		fmt.Println("getTickBitmap err:", err)
		return nil
	}

	return data

}

func position(tick int32) (int16, uint8) {
	wordPos := int16(tick >> 8)
	bitPos := uint8(tick & 0xFF)
	return wordPos, bitPos
}

func (u *UniswapV3Pool) getNextWord(wordPosition int16, client *ethclient.Client) *big.Int {
	//TODO implement me
	ins, err := UniswapV3PoolSol.NewUniswapV3PoolSol(u.Address, client)

	if err != nil {
		fmt.Println("getTickWord err:", err)
		return nil
	}
	data, err := ins.TickBitmap(&bind.CallOpts{}, wordPosition)
	if err != nil {
		fmt.Println("getTickBitmap err:", err)
		return nil
	}

	return data
}

func (u *UniswapV3Pool) getTickSpacing(client *ethclient.Client) *big.Int {
	//TODO implement me
	ins, err := UniswapV3PoolSol.NewUniswapV3PoolSol(u.Address, client)

	if err != nil {
		fmt.Println("getTickSpacing err:", err)
		return nil
	}
	data, err := ins.TickSpacing(&bind.CallOpts{})
	if err != nil {
		fmt.Println("getTickSpacing err:", err)
		return nil
	}

	return data
}

func (u *UniswapV3Pool) getTick(client *ethclient.Client) *big.Int {

	ins, err := UniswapV3PoolSol.NewUniswapV3PoolSol(u.Address, client)

	if err != nil {
		fmt.Println("getTickWord err:", err)
		return nil
	}
	data, err := ins.Slot0(&bind.CallOpts{})
	if err != nil {
		fmt.Println("getTickBitmap err:", err)
		return nil
	}

	return data.Tick

}

func (u *UniswapV3Pool) getTickInfo(tick *big.Int, client *ethclient.Client) struct {
	LiquidityGross                 *big.Int
	LiquidityNet                   *big.Int
	FeeGrowthOutside0X128          *big.Int
	FeeGrowthOutside1X128          *big.Int
	TickCumulativeOutside          *big.Int
	SecondsPerLiquidityOutsideX128 *big.Int
	SecondsOutside                 uint32
	Initialized                    bool
} {

	ins, err := UniswapV3PoolSol.NewUniswapV3PoolSol(u.Address, client)

	if err != nil {
		fmt.Println("getTickInfo err:", err)
	}
	data, err := ins.Ticks(&bind.CallOpts{}, tick)
	if err != nil {
		fmt.Println("getTickInfo err:", err)
	}

	return data
}

func (u *UniswapV3Pool) getLiquidityNet(tick *big.Int, client *ethclient.Client) *big.Int {
	return u.getTickInfo(tick, client).LiquidityNet
}
func (u *UniswapV3Pool) getInitialized(tick *big.Int, client *ethclient.Client) bool {
	return u.getTickInfo(tick, client).Initialized
}

func (u *UniswapV3Pool) getSlot0(client *ethclient.Client) struct {
	SqrtPriceX96               *big.Int
	Tick                       *big.Int
	ObservationIndex           uint16
	ObservationCardinality     uint16
	ObservationCardinalityNext uint16
	FeeProtocol                uint8
	Unlocked                   bool
} {

	ins, err := UniswapV3PoolSol.NewUniswapV3PoolSol(u.Address, client)

	if err != nil {
		fmt.Println("getSlot0 err:", err)
	}
	data, err := ins.Slot0(&bind.CallOpts{})
	if err != nil {
		fmt.Println("getSlot0 err:", err)
	}
	return data

}

func (u *UniswapV3Pool) getLiquidity(client *ethclient.Client) *big.Int {
	ins, err := UniswapV3PoolSol.NewUniswapV3PoolSol(u.Address, client)

	if err != nil {
		fmt.Println("getLiquidity err:", err)
	}
	data, err := ins.Liquidity(&bind.CallOpts{})
	if err != nil {
		fmt.Println("getLiquidity err:", err)
	}
	return data

}

func (u *UniswapV3Pool) getSqrtPrice(client *ethclient.Client) *big.Int {
	return u.getSlot0(client).SqrtPriceX96
}

func (u *UniswapV3Pool) updatePoolFromSwapLog(client *ethclient.Client) {
	//(_, _, self.sqrt_price, self.liquidity, self.tick) = self.decode_swap_log(swap_log);
	//
	//self.liquidity_net = self.get_liquidity_net(self.tick, middleware).await?;

}

//Returns reserve0, reserve1

func (u *UniswapV3Pool) decodeSwapLog(log any) {

	return
}

func (u *UniswapV3Pool) getTokenDecimals(client *ethclient.Client) (uint8, uint8) {
	insA, err := Erc20.NewErc20(u.TokenA, client)
	if err != nil {
		fmt.Println("getTokenDecimalsIns err:", err)
	}
	decimalA, err := insA.Decimals(nil)
	if err != nil {
		fmt.Println("getTokenDecimals err:", err)
	}

	insB, err := Erc20.NewErc20(u.TokenB, client)
	if err != nil {
		fmt.Println("getTokenDecimalsIns err:", err)
	}
	decimalB, err := insB.Decimals(nil)
	if err != nil {
		fmt.Println("getTokenDecimals err:", err)
	}

	return decimalA, decimalB

}

func (u *UniswapV3Pool) getFee(client *ethclient.Client) *big.Int {
	ins, err := UniswapV3PoolSol.NewUniswapV3PoolSol(u.Address, client)
	if err != nil {
		fmt.Println("getFee err:", err)
	}
	fee, err := ins.Fee(nil)

	if err != nil {
		fmt.Println("getFee err:", err)
	}

	return fee

}

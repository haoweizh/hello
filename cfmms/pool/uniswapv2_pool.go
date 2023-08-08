package pool

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/abi_go/Erc20"
	"hello/cfmms/abi_go/UniswapV2Pair"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
	"hello/cfmms/utils"
	"log"
	"math/big"
)

type UniswapV2Pool struct {
	Address        common.Address
	TokenA         common.Address
	TokenADecimals int64
	TokenB         common.Address
	TokenBDecimals int64
	Reserve0       *big.Int
	Reserve1       *big.Int
	Fee            int64
}

func NewUniswapv2pool(address common.Address, token_a common.Address, token_a_decimals int64, token_b common.Address, token_b_decimals int64, reserve0 *big.Int, reserve1 *big.Int, fee int64) *UniswapV2Pool {
	return &UniswapV2Pool{
		Address:        address,
		TokenA:         token_a,
		TokenADecimals: token_a_decimals,
		TokenB:         token_b,
		TokenBDecimals: token_b_decimals,
		Reserve0:       reserve0,
		Reserve1:       reserve1,
		Fee:            fee,
	}
}

func (v2pool *UniswapV2Pool) NewFromAddress(address common.Address, client *ethclient.Client) (any, error) {
	//TODO implement me
	err := v2pool.GetPoolData(client)

	if v2pool.DataIsPopulated() {
		return v2pool, nil
	}

	return v2pool, err
}

func (v2pool *UniswapV2Pool) NewFromEventLog(address common.Address, client *ethclient.Client) {

	//TODO implement me
	panic("implement me")
}

func (v2pool *UniswapV2Pool) NewEmptyPoolFromEventLog(log any) {
	//TODO implement me
	panic("implement me")
}

func (v2pool *UniswapV2Pool) SyncPool() (err error) {
	//TODO implement me
	panic("implement me")
}

func (v2pool *UniswapV2Pool) GetPoolData(client *ethclient.Client) error {
	//TODO implement me
	poolData, err := batch_request_for_uniswap_v2.Get_v2_pool_data_batch_request(v2pool.Address, client)

	if err != nil {
		log.Printf("Error in getting pool data: %v", err)
	}

	hexString := hex.EncodeToString(poolData)
	hexString = hexString[128:]
	oneStructLen := 64 * 6
	nums := len(hexString) / oneStructLen
	for i := 1; i < nums+1; i++ {
		v2pool.TokenA = common.HexToAddress(hexString[(i-1)*oneStructLen : (i-1)*oneStructLen+64])
		v2pool.TokenADecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+64 : (i-1)*oneStructLen+128])).Int64())
		v2pool.TokenB = common.HexToAddress(hexString[(i-1)*oneStructLen+128 : (i-1)*oneStructLen+192])
		v2pool.TokenBDecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+192 : (i-1)*oneStructLen+256])).Int64())
		v2pool.Reserve0 = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+256 : (i-1)*oneStructLen+320]))
		v2pool.Reserve1 = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+320 : (i-1)*oneStructLen+384]))
		v2pool.Fee = 300
		v2pool.Address = v2pool.Address
	}
	return err

}

func (v2pool *UniswapV2Pool) GetAddress() common.Address {
	return v2pool.Address
}

func (v2pool *UniswapV2Pool) DataIsPopulated() bool {

	if v2pool.TokenA == utils.Address0 || v2pool.TokenB == utils.Address0 || v2pool.Reserve0.BitLen() == 0 || v2pool.Reserve1.BitLen() == 0 {
		return false
	}

	return true
}

// selfMeth

func (v2pool *UniswapV2Pool) GetReserves(address common.Address, client *ethclient.Client) (*big.Int, *big.Int) {

	v2poolIns, err := UniswapV2Pair.NewUniswapV2Pair(v2pool.Address, client)
	if err != nil {
		fmt.Println("Failed to instantiate a Token contract: %v", err)
		return nil, nil
	}

	reserves, err := v2poolIns.GetReserves(nil)
	if err != nil {
		fmt.Println("Failed to get reserves: %v", err)
		return nil, nil
	}
	reserves0 := reserves.Reserve0
	reserves1 := reserves.Reserve1
	return reserves0, reserves1
}

func (v2pool *UniswapV2Pool) Syncpool(address common.Address, client *ethclient.Client) {
	v2pool.Reserve0, v2pool.Reserve1 = v2pool.GetReserves(address, client)
}

func (v2pool *UniswapV2Pool) GetTokenDecimals(client *ethclient.Client) (uint8, uint8) {

	tokenAIns, err := Erc20.NewErc20(v2pool.TokenA, client)
	if err != nil {
		fmt.Println("Failed to instantiate a Token contract: %v", err)
		return 0, 0
	}

	tokenADecimals, err := tokenAIns.Decimals(nil)
	if err != nil {
		fmt.Println("Failed to get tokenA decimals: %v", err)
		return 0, 0
	}

	tokenBIns, err := Erc20.NewErc20(v2pool.TokenB, client)
	if err != nil {
		fmt.Println("Failed to instantiate a Token contract: %v", err)
		return 0, 0
	}

	tokenBDecimals, err := tokenBIns.Decimals(nil)
	if err != nil {
		fmt.Println("Failed to get tokenA decimals: %v", err)
		return 0, 0
	}

	return tokenADecimals, tokenBDecimals

}

func (v2pool *UniswapV2Pool) GetTokenSymbol(client *ethclient.Client) (string, string) {
	tokenAIns, err := Erc20.NewErc20(v2pool.TokenA, client)
	if err != nil {
		fmt.Println("Failed to instantiate a Token contract: %v", err)
		return "", ""
	}

	tokenASymbol, err := tokenAIns.Symbol(nil)
	if err != nil {
		fmt.Println("Failed to get tokenA decimals: %v", err)
		return "", ""
	}

	tokenBIns, err := Erc20.NewErc20(v2pool.TokenB, client)
	if err != nil {
		fmt.Println("Failed to instantiate a Token contract: %v", err)
		return "", ""
	}

	tokenBSymbol, err := tokenBIns.Symbol(nil)
	if err != nil {
		fmt.Println("Failed to get tokenA decimals: %v", err)
		return "", ""
	}

	return tokenASymbol, tokenBSymbol

}

func (v2pool *UniswapV2Pool) GetToken0(client *ethclient.Client) (common.Address, error) {
	ins, err := UniswapV2Pair.NewUniswapV2Pair(v2pool.Address, client)
	if err != nil {
		fmt.Println("Failed to instantiate a Token contract: %v", err)
	}
	token0, err := ins.Token0(nil)
	if err != nil {
		fmt.Println("Failed to get token0: %v", err)
	}

	return token0, err

}
func (v2pool *UniswapV2Pool) GetToken1(client *ethclient.Client) (common.Address, error) {
	ins, err := UniswapV2Pair.NewUniswapV2Pair(v2pool.Address, client)
	if err != nil {
		fmt.Println("Failed to instantiate a Token contract: %v", err)
	}
	token1, err := ins.Token1(nil)
	if err != nil {
		fmt.Println("Failed to get token0: %v", err)
	}

	return token1, err

}

func (v2pool *UniswapV2Pool) CalculatePrice(baseToken common.Address) {
	_ = utils.Q64ToQ16(v2pool.CalculatePrice64X64(baseToken))
}

func (v2pool *UniswapV2Pool) CalculatePrice64X64(baseToken common.Address) uint64 {

	decimalShift := v2pool.TokenADecimals - v2pool.TokenBDecimals
	var r0, r1 *big.Int
	if decimalShift < 0 {
		r0 = new(big.Int).Mul(v2pool.Reserve0, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-decimalShift)), nil))
		r1 = v2pool.Reserve1
	} else {
		r0 = v2pool.Reserve0
		r1 = new(big.Int).Mul(v2pool.Reserve1, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimalShift)), nil))
	}

	if v2pool.TokenA == baseToken {
		data, err := utils.Div64X64(r1, r0)
		if err != nil {
			fmt.Println("Failed to calculate price: %v", err)
			return 0

		}
		return data.Uint64()
	} else {

		data, err := utils.Div64X64(r0, r1)
		if err != nil {
			fmt.Println("Failed to calculate price: %v", err)
			return 0
		}

		return data.Uint64()

	}

	return 0
}

func (v2pool *UniswapV2Pool) GetAmountOut(reserveIn, reserveOut *big.Int, amountIn uint64) uint64 {
	if amountIn == 0 || reserveIn.Cmp(big.NewInt(0)) == 0 || reserveOut.Cmp(big.NewInt(0)) == 0 {
		return 0
	}

	amountInWithFee := new(big.Int).Mul(big.NewInt(int64(amountIn)), big.NewInt(997))
	numerator := new(big.Int).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Int).Add(new(big.Int).Mul(reserveIn, big.NewInt(1000)), amountInWithFee)

	return new(big.Int).Div(numerator, denominator).Uint64()

}

func (v2pool *UniswapV2Pool) SimulateSwap(tokenIn common.Address, amountIn *big.Int) {
	if v2pool.TokenA == tokenIn {
		v2pool.GetAmountOut(v2pool.Reserve0, v2pool.Reserve1, amountIn.Uint64())
	} else {
		v2pool.GetAmountOut(v2pool.Reserve1, v2pool.Reserve0, amountIn.Uint64())
	}

}

func (v2pool *UniswapV2Pool) SimulateSwapMut(tokenIn common.Address, amountIn *big.Int) uint64 {

	if v2pool.TokenA == tokenIn {
		amountOut := v2pool.GetAmountOut(v2pool.Reserve0, v2pool.Reserve1, amountIn.Uint64())

		v2pool.Reserve0.Add(v2pool.Reserve0, amountIn)
		v2pool.Reserve1.Sub(v2pool.Reserve1, big.NewInt(int64(amountOut)))
		return amountOut
	} else {
		amountOut := v2pool.GetAmountOut(v2pool.Reserve1, v2pool.Reserve0, amountIn.Uint64())
		v2pool.Reserve0.Sub(v2pool.Reserve0, big.NewInt(int64(amountOut)))
		v2pool.Reserve1.Add(v2pool.Reserve1, amountIn)
		return amountOut
	}

	return 0
}

func (v2pool *UniswapV2Pool) updatePoolFromSyncLog(log any) {
	v2pool.decodeSyncLog(log)
}

func (v2pool *UniswapV2Pool) decodeSyncLog(log any) {

	// todo: 解析日志

}

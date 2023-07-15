package pool

import (
	"encoding/hex"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
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

func (pool *UniswapV2Pool) NewFromAddress(address common.Address, client *ethclient.Client) (any, error) {
	//TODO implement me
	err := pool.GetPoolData(address, client)

	if pool.DataIsPopulated() {
		return pool, nil
	}

	return pool, err
}

func (pool *UniswapV2Pool) NewFromEventLog(address common.Address, client *ethclient.Client) {

	//TODO implement me
	panic("implement me")
}

func (pool UniswapV2Pool) NewEmptyPoolFromEventLog(log any) {
	//TODO implement me
	panic("implement me")
}

func (pool UniswapV2Pool) SyncPool() (err error) {
	//TODO implement me
	panic("implement me")
}

func (pool UniswapV2Pool) CalculatePrice() (price float64) {
	//TODO implement me
	panic("implement me")
}

func (pool *UniswapV2Pool) GetPoolData(address common.Address, client *ethclient.Client) error {
	//TODO implement me
	poolData, err := batch_request_for_uniswap_v2.Get_v2_pool_data_batch_request(address, client)

	if err != nil {
		log.Printf("Error in getting pool data: %v", err)
	}

	hexString := hex.EncodeToString(poolData)
	hexString = hexString[128:]
	oneStructLen := 64 * 6
	nums := len(hexString) / oneStructLen
	for i := 1; i < nums+1; i++ {
		pool.TokenA = common.HexToAddress(hexString[(i-1)*oneStructLen : (i-1)*oneStructLen+64])
		pool.TokenADecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+64 : (i-1)*oneStructLen+128])).Int64())
		pool.TokenB = common.HexToAddress(hexString[(i-1)*oneStructLen+128 : (i-1)*oneStructLen+192])
		pool.TokenBDecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+192 : (i-1)*oneStructLen+256])).Int64())
		pool.Reserve0 = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+256 : (i-1)*oneStructLen+320]))
		pool.Reserve1 = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+320 : (i-1)*oneStructLen+384]))
		pool.Fee = 300
		pool.Address = address
	}
	return err

}

func (pool UniswapV2Pool) GetAddress() any {
	//TODO implement me
	return pool.Address
}

func (pool UniswapV2Pool) SimulateSwap() {
	//TODO implement me
	panic("implement me")
}

func (pool UniswapV2Pool) SimulateSwapMut() {
	//TODO implement me
	panic("implement me")
}

func (pool *UniswapV2Pool) DataIsPopulated() bool {

	//TODO implement me
	if pool.TokenA == cfmms.Address0 || pool.TokenB == cfmms.Address0 || pool.Reserve0.BitLen() == 0 || pool.Reserve1.BitLen() == 0 {
		return false
	}

	return true
}

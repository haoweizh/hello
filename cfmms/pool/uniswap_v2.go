package pool

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
	"log"
	"math/big"
)

var Address0 = common.HexToAddress(`0x0000000000000000000000000000000000000000`)

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

func (pool *UniswapV2Pool) NewFromAddress(address common.Address, client ethclient.Client) error {
	uniswapv2pool := NewUniswapv2pool(address, common.Address{}, 0, common.Address{}, 0, big.NewInt(0), big.NewInt(0), 300)
	err := uniswapv2pool.getPoolData(client)

	if !pool.dataIsPopulated() {
		log.Fatal("pool data is not populated")
		return err
	}
	return err

}

//
//func (pool *UniswapV2Pool) NewEmptyPoolFromEventLog(log any, client ethclient.Client) error {
//
//}

func (pool *UniswapV2Pool) getPoolData(client ethclient.Client) (err error) {
	err = batch_request_for_uniswap_v2.Get_v2_pool_data_batch_request(pool, client)
	return err
}

func (pool *UniswapV2Pool) dataIsPopulated() bool {
	if pool.Reserve0 != nil && pool.Reserve1 != nil && pool.TokenA != Address0 && pool.TokenB != Address0 {
		return true
	}
	return false
}

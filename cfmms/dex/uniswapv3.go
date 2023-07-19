package dex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/pool"
	"hello/cfmms/utils"
)

var (
	PairCreatedEventSignature = []byte("PairCreated(address,address,address,uint256)")
)

type UniswapV3Dex struct {
	FactoryAddress common.Address
	CreationBlock  int64
}

func (u *UniswapV3Dex) PoolCreatedEventSignature() []byte {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) NewPoolFromEvent(address common.Address, client *ethclient.Client) {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) NewEmptyPoolFromEvent(log any) {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetAllPools(requestThrottle *utils.Throttle, client *ethclient.Client, step uint64) ([]pool.Pool, error) {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetAllPoolsData(pool *[]pool.Pool, requestThrottle *utils.Throttle, client *ethclient.Client) error {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetPoolWithBestLiquidity() {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetAllPoolsForPair() {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetAllPoolsFromLogs() {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetAllPoolsFromLogsWithinRange() {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetFactoryAddress() common.Address {
	//TODO implement me
	panic("implement me")
}

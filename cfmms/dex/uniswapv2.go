package dex

import (
	"github.com/ethereum/go-ethereum/common"
)

type UniswapV2Dex struct {
	FactoryAddress common.Address
	CreationBlock  int64
	fee            int64
}

var PAIR_CREATED_EVENT_SIGNATURE = []byte("PairCreated(address,address,address,uint256)")

func NewUniswapV2Dex(factory_address common.Address, creation_block int64, fee int64) *UniswapV2Dex {
	return &UniswapV2Dex{
		FactoryAddress: factory_address,
		CreationBlock:  creation_block,
		fee:            fee,
	}
}

func (u UniswapV2Dex) PoolCreatedEventSignature() []byte {
	//TODO implement me
	return PAIR_CREATED_EVENT_SIGNATURE
}

func (u UniswapV2Dex) NewPoolFromEvent(log any) {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV2Dex) NewEmptyPoolFromEvent(log any) {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV2Dex) GetAllPools() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV2Dex) GetAllPoolsData() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV2Dex) GetPolWithBestLiquidity() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV2Dex) GetAllPoolsForPair() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV2Dex) GetAllPoolsFromLogs() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV2Dex) GetAllPoolsFromLogsWithinRange() {
	//TODO implement me
	panic("implement me")
}

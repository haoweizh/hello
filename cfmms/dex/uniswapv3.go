package dex

import "github.com/ethereum/go-ethereum/common"

var (
	PairCreatedEventSignature = []byte("PairCreated(address,address,address,uint256)")
)

type UniswapV3Dex struct {
}

func NewUniswapV3Dex(factoytaddress common.Address, creationblock int64, fee int64) *Dex {

}

func (u UniswapV3Dex) PoolCreatedEventSignature() []byte {
	//TODO implement me
	return PairCreatedEventSignature
}

func (u UniswapV3Dex) NewPoolFromEvent(log any) {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV3Dex) NewEmptyPoolFromEvent(log any) {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV3Dex) GetAllPools() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV3Dex) GetAllPoolsData() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV3Dex) GetPolWithBestLiquidity() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV3Dex) GetAllPoolsForPair() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV3Dex) GetAllPoolsFromLogs() {
	//TODO implement me
	panic("implement me")
}

func (u UniswapV3Dex) GetAllPoolsFromLogsWithinRange() {
	//TODO implement me
	panic("implement me")
}

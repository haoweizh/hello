package dex

import "github.com/ethereum/go-ethereum/common"

type DexVariant int

const (
	UniswapV2 DexVariant = iota
	UniswapV3
)

func (v DexVariant) PoolCreatedEventSignature() []byte {
	switch v {
	case UniswapV2:
		return []byte("PairCreated(address,address,address,uint256)")
	case UniswapV3:
		return []byte("PoolCreated(address,address,uint24,int24,int24)")
	default:
		return nil
	}
	return nil
}

type Dex interface {
	PoolCreatedEventSignature() []byte
	NewPoolFromEvent(log any)
	NewEmptyPoolFromEvent(log any)
	GetAllPools()
	GetAllPoolsData()

	GetPolWithBestLiquidity()
	GetAllPoolsForPair()
	GetAllPoolsFromLogs()
	GetAllPoolsFromLogsWithinRange()
}

func NewDex(factoytaddress common.Address, variant DexVariant, creationblock int64, fee int64) Dex {
	if fee == 0 {
		fee = 300
	}

	switch variant {
	case UniswapV2:
		//return NewUniswapV2Dex(factoytaddress, creationblock, fee).(Dex)
	case UniswapV3:
		//return NewUniswapV3Dex(factoytaddress, creationblock, fee)
	default:
		return nil
	}
	return nil
}

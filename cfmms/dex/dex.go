package dex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

type Dex interface {
	PoolCreatedEventSignature() []byte
	NewPoolFromEvent(address common.Address, client *ethclient.Client)
	NewEmptyPoolFromEvent(log any) any
	GetAllPools(client *ethclient.Client, step *big.Int) (any, error)
	GetAllPoolsData(pools any, client *ethclient.Client) error

	GetPoolWithBestLiquidity()
	GetAllPoolsForPair()
	GetAllPoolsFromLogsWithinRange()
	GetFactoryAddress() common.Address
}

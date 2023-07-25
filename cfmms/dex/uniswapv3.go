package dex

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/abi_go/UniswapV3Factory"
	"hello/cfmms/pool"
	"log"
	"math/big"
)

var (
	PairCreatedEventSignature = []byte("PairCreated(address,address,address,uint256)")
)

type UniswapV3Dex struct {
	FactoryAddress common.Address
	CreationBlock  *big.Int
}

func (u *UniswapV3Dex) PoolCreatedEventSignature() []byte {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) NewPoolFromEvent(address common.Address, client *ethclient.Client) {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) NewEmptyPoolFromEvent(log any) any {

	var pool1 pool.Pool
	return pool1

}

func (u *UniswapV3Dex) GetAllPools(client *ethclient.Client, step *big.Int) (any, error) {
	//TODO implement me

	current_block, err := client.BlockNumber(context.Background())
	if err != nil {
		fmt.Println("client.BlockNumber error")
		return nil, err
	}

	pools := u.GetAllPoolsFromLogs(big.NewInt(int64(current_block)), step, client)

	return pools, nil

}

func (u *UniswapV3Dex) GetAllPoolsData(pool *[]pool.Pool, client *ethclient.Client) error {
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

func (u *UniswapV3Dex) GetAllPoolsFromLogs(currentBlock *big.Int, step *big.Int, client *ethclient.Client) []pool.Pool {
	//TODO implement me
	aggregatedPairs := make([]pool.Pool, 0)

	ins, err := UniswapV3Factory.NewUniswapV3Factory(u.FactoryAddress, client)
	if err != nil {
		fmt.Println("UniswapV3Factory.NewUniswapV3Factory error")
		return nil
	}

	for fromBlock := u.CreationBlock; fromBlock.Cmp(currentBlock) < 0; fromBlock = big.NewInt(0).Add(fromBlock, step) {
		end := big.NewInt(0).Add(fromBlock, step).Uint64()
		res, err := ins.FilterPoolCreated(&bind.FilterOpts{
			Start:   fromBlock.Uint64(),
			End:     &end,
			Context: nil,
		}, nil, nil, nil)
		if err != nil {
			fmt.Println("Failed to filterLog contract instance")
			log.Fatal(err)
		}
		fmt.Println(res)

		fmt.Println(res.Event)

		for res.Next() {
			if res.Event.Raw.Removed {
				continue
			}
			// 拿到地址 neweventFromAddress
			fmt.Println("finnnnnnnnnnnnnn", res.Event.Pool)

			poolFromEvent := u.NewEmptyPoolFromEvent(res.Event.Pool)

			fmt.Println("poolFromEvent", poolFromEvent)
			aggregatedPairs = append(aggregatedPairs, poolFromEvent.(pool.Pool))
		}

	}

	return aggregatedPairs
}

func (u *UniswapV3Dex) GetAllPoolsFromLogsWithinRange() {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetFactoryAddress() common.Address {
	//TODO implement me
	panic("implement me")
}

package dex

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/abi_go/UniswapV3Factory"
	"hello/cfmms/pool"
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

	it := log.(*UniswapV3Factory.UniswapV3FactoryPoolCreated)

	return pool.UniswapV3Pool{
		Address:        it.Pool,
		TokenA:         it.Token0,
		TokenADecimals: 0,
		TokenB:         it.Token1,
		TokenBDecimals: 0,
		Liquidity:      nil,
		SqrtPrice:      nil,
		Fee:            it.Fee,
		Tick:           nil,
		TickSpacing:    it.TickSpacing,
		LiquidityNet:   nil,
	}

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

func (u *UniswapV3Dex) GetAllPoolsData(v3pools any, client *ethclient.Client) error {

	pools := v3pools.([]pool.UniswapV3Pool)
	fmt.Println("pools", pools)
	//  

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

func (u *UniswapV3Dex) GetAllPoolsFromLogs(currentBlock *big.Int, step *big.Int, client *ethclient.Client) []pool.UniswapV3Pool {
	//TODO implement me
	aggregatedPairs := make([]pool.UniswapV3Pool, 0)

	ins, err := UniswapV3Factory.NewUniswapV3Factory(u.FactoryAddress, client)
	if err != nil {
		fmt.Println("UniswapV3Factory.NewUniswapV3Factory error")
		return nil
	}
	for fromBlock := u.CreationBlock; fromBlock.Cmp(currentBlock) < 0; fromBlock = big.NewInt(0).Add(fromBlock, step) {
		end := big.NewInt(0).Add(fromBlock, step).Uint64()

		fmt.Println("fromBlock", fromBlock, "currentBlock", currentBlock, "end", end)

		res, err := ins.FilterPoolCreated(&bind.FilterOpts{
			Start:   fromBlock.Uint64(),
			End:     &end,
			Context: nil,
		}, nil, nil, nil)
		if err != nil {
			fmt.Println("Failed to filterLog contract instance", err)
			return nil
		}
		for res.Next() {
			if res.Event.Raw.Removed {
				continue
			}
			poolFromEvent := u.NewEmptyPoolFromEvent(res.Event)
			fmt.Println("poolFromEvent", poolFromEvent)
			aggregatedPairs = append(aggregatedPairs, poolFromEvent.(pool.UniswapV3Pool))
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

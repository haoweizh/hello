package dex

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/abi_go/UniswapV3Factory"
	"hello/cfmms/pool"
	"hello/cfmms/utils"
	"log"
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

func (u *UniswapV3Dex) NewEmptyPoolFromEvent(log any) pool.Pool {
	let tokens = ethers::abi::decode(&[ParamType::Uint(32), ParamType::Address], &log.data)?;
	let token_a = H160::from(log.topics[0]);
	let token_b = H160::from(log.topics[1]);
	let fee = tokens[0].to_owned().into_uint().unwrap().as_u32();
	let address = tokens[1].to_owned().into_address().unwrap();

	Ok(Pool::UniswapV3(UniswapV3Pool {
	address,
	token_a,
	token_b,
	token_a_decimals: 0,
	token_b_decimals: 0,
	fee,
	liquidity: 0,
	sqrt_price: U256::zero(),
	tick_spacing: 0,
	tick: 0,
	liquidity_net: 0,
	}))



}

func (u *UniswapV3Dex) GetAllPools(requestThrottle *utils.Throttle, client *ethclient.Client, step int64) ([]pool.Pool, error) {
	//TODO implement me

	current_block, err := client.BlockNumber(context.Background())
	if err != nil {
		fmt.Println("client.BlockNumber error")
		return nil, err
	}

	pools := u.GetAllPoolsFromLogs(int64(current_block), step, requestThrottle, client)

	return pools, nil

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

func (u *UniswapV3Dex) GetAllPoolsFromLogs(currentBlock int64, step int64, requestThrottle *utils.Throttle, client *ethclient.Client) []pool.Pool {
	//TODO implement me
	aggregatedPairs := make([]pool.Pool, 0)

	ins, err := UniswapV3Factory.NewUniswapV3Factory(u.FactoryAddress, client)
	if err != nil {
		fmt.Println("UniswapV3Factory.NewUniswapV3Factory error")
		return nil
	}

	for fromBlock := u.CreationBlock; fromBlock < currentBlock; fromBlock += step {
		requestThrottle.IncrementOrSleep(1)
		end := uint64(fromBlock + step)
		res, err := ins.FilterPoolCreated(&bind.FilterOpts{
			Start:   uint64(fromBlock),
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
			aggregatedPairs = append(aggregatedPairs, poolFromEvent)
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

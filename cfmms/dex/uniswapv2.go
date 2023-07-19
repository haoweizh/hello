package dex

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/schollz/progressbar/v3"
	"hello/cfmms/abi_go/UniswapV2Factory"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
	"hello/cfmms/pool"
	"hello/cfmms/utils"
	"log"
)

type UniswapV2Dex struct {
	FactoryAddress common.Address
	CreationBlock  uint64
	Fee            int64
}

var PAIR_CREATED_EVENT_SIGNATURE = []byte("PairCreated(address,address,address,uint256)")

func NewUniswapV2Dex(factory_address common.Address, creation_block uint64, fee int64) *UniswapV2Dex {
	return &UniswapV2Dex{
		FactoryAddress: factory_address,
		CreationBlock:  creation_block,
		Fee:            fee,
	}
}

func (univ2 *UniswapV2Dex) PoolCreatedEventSignature() []byte {
	//TODO implement me

	return PAIR_CREATED_EVENT_SIGNATURE
}

func (univ2 *UniswapV2Dex) NewPoolFromEvent(address common.Address, client *ethclient.Client) {
	//TODO implement me

	//let tokens = ethers::abi::decode(&[ParamType::Address, ParamType::Uint(256)], &log.data)?;
	//let pair_address = tokens[0].to_owned().into_address().unwrap();
	//Pool::new_from_address(pair_address, DexVariant::UniswapV2, middleware).await

	// uniswapv2 factory 合约Abi  method: PairCreated

	//univ2.Pool.NewFromAddress(address, client)

	//pool.UniswapV2Pool.NewFromEventLog(address, client)

	//univ2.Pool.NewFromAddress(address, client)

	var v2Pool pool.UniswapV2Pool
	res, err := v2Pool.NewFromAddress(address, client)

	if err != nil {

		fmt.Println("NewFromAddress error")
		log.Fatal(err)
	}
	fmt.Println(res)

}

func (univ2 *UniswapV2Dex) NewEmptyPoolFromEvent(log any) {
	//TODO implement me
	panic("implement me")
}

func (univ2 *UniswapV2Dex) GetAllPools(requestThrottle *utils.Throttle, client *ethclient.Client, step uint64) (any, error) {

	pools, err := univ2.getAllPairsViaBatchedCalls(client, requestThrottle)

	return pools, err
}

func (univ2 *UniswapV2Dex) GetAllPoolsData(pool *[]pool.Pool, requestThrottle *utils.Throttle, client *ethclient.Client) error {
	//TODO implement me
	panic("implement me")
}

func (univ2 *UniswapV2Dex) GetPoolWithBestLiquidity() {
	//TODO implement me
	panic("implement me")
}

func (univ2 *UniswapV2Dex) GetAllPoolsForPair() {
	//TODO implement me
	panic("implement me")
}

func (univ2 *UniswapV2Dex) GetAllPoolsFromLogsWithinRange() {
	//TODO implement me
	panic("implement me")
}

func (univ2 *UniswapV2Dex) GetFactoryAddress() common.Address {
	return common.HexToAddress("0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f")
}

func (univ2 *UniswapV2Dex) getAllPairsViaBatchedCalls(client *ethclient.Client, requestThrottle *utils.Throttle) ([]pool.UniswapV2Pool, error) {
	ins, err := UniswapV2Factory.NewUniswapV2Factory(univ2.FactoryAddress, client)
	if err != nil {
		fmt.Println("NewUniswapV2Factory error")
		return nil, err
	}
	allpairslen, err := ins.AllPairsLength(nil)
	if err != nil {
		fmt.Println("AllPairsLength error")
		return nil, err
	}

	// initialize progress bar
	bar := progressbar.Default(allpairslen.Int64())

	pairs := make([]any, 0, allpairslen.Int64())

	step := int64(766) // 超出报错  max batch size for this call until codesize is too large

	var idxTo int64
	if step > allpairslen.Int64() {
		idxTo = allpairslen.Int64()
	} else {
		idxTo = step
	}

	for idxFrom := int64(0); idxFrom < allpairslen.Int64(); idxFrom += step {

		requestThrottle.IncrementOrSleep(1)

		pairs = append(pairs, batch_request_for_uniswap_v2.Get_pairs_batch_request(univ2.FactoryAddress, idxFrom, step, client))

		idxFrom = idxTo
		if idxTo+step > allpairslen.Int64() {
			idxTo = allpairslen.Int64() - 1
		} else {
			idxTo += step
		}

		err := bar.Add(int(step))
		if err != nil {
			return nil, err
		}

	}

	fmt.Println(pairs)
	pools := make([]pool.UniswapV2Pool, 0)

	for _, v := range pairs {
		pools = append(pools, pool.UniswapV2Pool{Address: v.(common.Address)})
	}
	fmt.Println(pools)

	return pools, err

}

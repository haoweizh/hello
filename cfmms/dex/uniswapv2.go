package dex

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
	"hello/cfmms/abi_go/UniswapV2Factory"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
	"hello/cfmms/pool"
	"log"
	"sync"
	"time"
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

func (univ2 *UniswapV2Dex) NewEmptyPoolFromEvent(log any) any {
	//TODO implement me
	panic("implement me")
}

func (univ2 *UniswapV2Dex) GetAllPools(client *ethclient.Client, step int64) (any, error) {

	var pools *sync.Map
	var err error
	// 限制速率 创建一个每秒允许 100 个事件的速率限制器，并且允许 100 个突发事件。
	limiter := rate.NewLimiter(rate.Every(time.Second/100), 100)
	if limiter.Allow() {
		pools, err = univ2.getAllPairsViaBatchedCalls(client)
		if err != nil {
			fmt.Println("getAllPairsViaBatchedCalls error")
			return nil, err
		}
	}

	return pools, err
}

func (univ2 *UniswapV2Dex) GetAllPoolsData(pool *[]pool.Pool, client *ethclient.Client) error {
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

func (univ2 *UniswapV2Dex) getAllPairsViaBatchedCalls(client *ethclient.Client) (*sync.Map, error) {

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

	//pairs := make([]any, 0, allpairslen.Int64())
	pairs := sync.Map{}

	step := int64(766) // 超出报错  max batch size for this call until codesize is too large

	var idxTo int64
	if step > allpairslen.Int64() {
		idxTo = allpairslen.Int64()
	} else {
		idxTo = step
	}

	for idxFrom := int64(0); idxFrom < allpairslen.Int64(); idxFrom += step {

		//pairs = append(pairs, batch_request_for_uniswap_v2.Get_pairs_batch_request(univ2.FactoryAddress, idxFrom, step, client))

		idxFrom = idxTo
		if idxTo+step > allpairslen.Int64() {
			idxTo = allpairslen.Int64() - 1
		} else {
			idxTo += step
		}
		fmt.Println("idxFrom:", idxFrom, "idxTo:", idxTo)
		for k, v := range batch_request_for_uniswap_v2.GetPairsBatchRequest(univ2.FactoryAddress, idxFrom, idxTo, client) {
			pairs.Store(k, v)
		}

		err := bar.Add(int(step))
		if err != nil {
			fmt.Println("bar.Add error")
			return &sync.Map{}, err
		}

	}

	//pools := make([]pool.UniswapV2Pool, 0)

	pools := sync.Map{}

	pairs.Range(func(k, v interface{}) bool {
		pools.Store(k, pool.UniswapV2Pool{Address: v.(common.Address)})
		return true
	})

	return &pools, err

}

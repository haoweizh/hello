package dex

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/abi_go/UniswapV2Factory"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
	"hello/cfmms/pool"
	"log"
	"math/big"
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

func (univ2 *UniswapV2Dex) GetAllPools(client *ethclient.Client, step *big.Int) (any, error) {

	var pools *sync.Map
	var err error
	pools, err = univ2.getAllPairsViaBatchedCalls(client)
	if err != nil {
		fmt.Println("getAllPairsViaBatchedCalls error", err)
		return nil, err
	}
	return pools, err
}

func (univ2 *UniswapV2Dex) GetAllPoolsData(v2pools any, client *ethclient.Client) error {
	//TODO implement me

	pools := v2pools.(*sync.Map)

	//fmt.Println("GetAllPoolsData start", pools)
	fmt.Println("GetAllPoolsData start")
	//step := big.NewInt(127) // 超出报错  max batch size for this call until codesize is too large

	fi, ok := pools.Load(0)
	if ok {
		fmt.Println("pools", fi.(pool.UniswapV2Pool).Address)
	}

	var poolArray []any
	pools.Range(func(key, value any) bool {

		poolArray = append(poolArray, value.(pool.UniswapV2Pool).Address.String())
		return true

	})

	v2poolsData := &sync.Map{}
	wg := &sync.WaitGroup{}
	chunkSize := 127 // max batch size for this call until codesize is too large
	chunks := splitSlice(poolArray, chunkSize)

	for _, chunk := range chunks {
		wg.Add(1)
		go func(chunk []any) {
			defer wg.Done()
			var temp []string
			for _, item := range chunk {
				temp = append(temp, item.(string))
			}
			result := batch_request_for_uniswap_v2.Get_pool_data_batch_request(temp, client)
			hexString := hex.EncodeToString(result)
			hexString = hexString[128:]
			oneStructLen := 64 * 6
			nums := len(hexString) / oneStructLen
			var poolDataList []pool.UniswapV2Pool
			for i := 1; i < nums+1; i++ {
				var poolData pool.UniswapV2Pool
				poolData.Address = common.HexToAddress(temp[i-1])
				poolData.TokenA = common.HexToAddress(hexString[(i-1)*oneStructLen : (i-1)*oneStructLen+64])
				poolData.TokenADecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+64 : (i-1)*oneStructLen+128])).Int64())
				poolData.TokenB = common.HexToAddress(hexString[(i-1)*oneStructLen+128 : (i-1)*oneStructLen+192])
				poolData.TokenBDecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+192 : (i-1)*oneStructLen+256])).Int64())
				poolData.Reserve0 = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+256 : (i-1)*oneStructLen+320]))
				poolData.Reserve1 = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+320 : (i-1)*oneStructLen+384]))
				poolData.Fee = 300
				poolDataList = append(poolDataList, poolData)
			}

			for i, v2Pool := range poolDataList {
				v2poolsData.Store(i, v2Pool)
			}
		}(chunk)
		time.Sleep(time.Second / 20)
	}

	wg.Wait()

	// 拿到数据
	if v, ok := v2poolsData.Load(0); ok {
		fmt.Println(v)
	}
	return nil
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
		fmt.Println("NewUniswapV2Factory error", err)
		return nil, err
	}
	allpairslen, err := ins.AllPairsLength(nil)
	if err != nil {
		fmt.Println("AllPairsLength error", err)
		return nil, err
	}

	// TODO: remove this  调试减少数量
	allpairslen = big.NewInt(766)
	// initialize progress bar

	pairs := sync.Map{}
	wg := sync.WaitGroup{}

	step := big.NewInt(766) // 超出报错  max batch size for this call until codesize is too large

	for i := big.NewInt(0); allpairslen.Cmp(i) > 0; i = big.NewInt(0).Add(i, step) {

		var to *big.Int
		if big.NewInt(0).Add(i, step).Cmp(allpairslen) == 1 {
			to = allpairslen
		} else {
			to = big.NewInt(0).Add(i, step)
		}
		wg.Add(1)
		go func(i, to *big.Int) {
			defer wg.Done()
			for k, v := range batch_request_for_uniswap_v2.GetPairsBatchRequest(univ2.FactoryAddress, i, to, client) {
				pairs.Store(k+int(i.Int64()), v)
			}
		}(i, to)
		time.Sleep(time.Second / 25) // 限制速率 根据节点性能调整
	}
	wg.Wait()

	pools := sync.Map{}

	pairs.Range(func(k, v interface{}) bool {
		pools.Store(k, pool.UniswapV2Pool{Address: common.HexToAddress(v.(string))})
		return true
	})

	return &pools, err

}

func splitSlice(slice []any, chunkSize int) [][]any {
	var chunks [][]any
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

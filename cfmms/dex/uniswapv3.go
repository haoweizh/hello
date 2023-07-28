package dex

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/abi_go/UniswapV3Factory"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v3"
	"hello/cfmms/pool"
	"math/big"
	"sync"
	"time"
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

	currentBlock, err := client.BlockNumber(context.Background())

	// FIXME: 测试用
	currentBlock = uint64(12379621)

	if err != nil {
		fmt.Println("client.BlockNumber error")
		return nil, err
	}

	pools := u.GetAllPoolsFromLogs(big.NewInt(int64(currentBlock)), step, client)

	return pools, nil

}

func (u *UniswapV3Dex) GetAllPoolsData(v3pools any, client *ethclient.Client) *sync.Map {

	var targetAddress []any

	v3pools.(*sync.Map).Range(func(key, value any) bool {
		targetAddress = append(targetAddress, key)
		return true
	})

	chunkSize := 56 // max batch size for this call until codesize is too large
	chunks := splitSlice(targetAddress, chunkSize)

	wg := &sync.WaitGroup{}
	poolsWithData := &sync.Map{}

	for _, chunk := range chunks {
		wg.Add(1)
		go func(chunk []any) {
			defer wg.Done()
			var temp []common.Address
			for _, item := range chunk {
				temp = append(temp, item.(common.Address))
			}
			result, err := batch_request_for_uniswap_v3.GetPoolDataBatchRequest(temp, client)
			if err != nil {
				fmt.Println("batch_request_for_uniswap_v3.GetPoolDataBatchRequest error", err)
				return
			}
			data := decodeResult(temp, result)
			for i := 0; i < len(data); i++ {
				poolsWithData.Store(i, data[i])
			}

		}(chunk)
		time.Sleep(time.Second / 20)
	}

	wg.Wait()

	return poolsWithData
}

func (u *UniswapV3Dex) GetPoolWithBestLiquidity() {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetAllPoolsForPair() {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetAllPoolsFromLogs(currentBlock *big.Int, step *big.Int, client *ethclient.Client) *sync.Map {
	//aggregatedPairs := make([]pool.UniswapV3Pool, 0)

	aggregatedPairs := &sync.Map{}

	ins, err := UniswapV3Factory.NewUniswapV3Factory(u.FactoryAddress, client)
	if err != nil {
		fmt.Println("get uniswav3factory ins error", err)
		return nil
	}

	wg := &sync.WaitGroup{}

	for fromBlock := u.CreationBlock; fromBlock.Cmp(currentBlock) < 0; fromBlock = big.NewInt(0).Add(fromBlock, step) {

		wg.Add(1)
		go func(fromBlock *big.Int, step *big.Int) {
			defer wg.Done()

			end := big.NewInt(0).Add(fromBlock, step).Uint64()

			fmt.Println("fromBlock", fromBlock, "currentBlock", currentBlock, "end", end)

			res, err := ins.FilterPoolCreated(&bind.FilterOpts{
				Start:   fromBlock.Uint64(),
				End:     &end,
				Context: nil,
			}, nil, nil, nil)
			if err != nil {
				fmt.Println("Failed to filterLog contract instance", err)
				return
			}
			for res.Next() {
				if res.Event.Raw.Removed {
					continue
				}
				poolFromEvent := u.NewEmptyPoolFromEvent(res.Event)
				aggregatedPairs.Store(poolFromEvent.(pool.UniswapV3Pool).Address, poolFromEvent)
			}

		}(fromBlock, step)

		time.Sleep(time.Second / 20)

	}

	wg.Wait()
	return aggregatedPairs
}

func (u *UniswapV3Dex) GetAllPoolsFromLogsWithinRange() {
	//TODO implement me
	panic("implement me")
}

func (u *UniswapV3Dex) GetFactoryAddress() common.Address {

	return u.FactoryAddress
}

func decodeResult(temp []common.Address, result interface{}) []pool.UniswapV3Pool {

	hexString := hex.EncodeToString(result.([]byte))

	hexString = hexString[128:]

	oneStructLen := 64 * 10 // solidity struct length

	nums := len(hexString) / oneStructLen
	var poolDataList []pool.UniswapV3Pool
	for i := 1; i < nums+1; i++ {
		var poolData pool.UniswapV3Pool
		poolData.Address = temp[i-1]
		poolData.TokenA = common.HexToAddress(hexString[(i-1)*oneStructLen : (i-1)*oneStructLen+64])
		poolData.TokenADecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+64 : (i-1)*oneStructLen+128])).Int64())
		poolData.TokenB = common.HexToAddress(hexString[(i-1)*oneStructLen+128 : (i-1)*oneStructLen+192])
		poolData.TokenBDecimals = int64(new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+192 : (i-1)*oneStructLen+256])).Int64())
		poolData.Liquidity = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+256 : (i-1)*oneStructLen+320]))
		poolData.SqrtPrice = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+320 : (i-1)*oneStructLen+384]))
		poolData.Tick = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+384 : (i-1)*oneStructLen+448]))
		poolData.TickSpacing = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+448 : (i-1)*oneStructLen+512]))
		poolData.Fee = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+512 : (i-1)*oneStructLen+576]))
		poolData.LiquidityNet = new(big.Int).SetBytes(common.FromHex(hexString[(i-1)*oneStructLen+576 : (i-1)*oneStructLen+640]))
		poolDataList = append(poolDataList, poolData)
	}

	return poolDataList

}

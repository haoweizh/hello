package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"hello/cfmms"
	"hello/cfmms/abi_go/UniswapV2Factory"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
	"hello/cfmms/dex"
	"hello/cfmms/pool"
	"hello/cfmms/utils"
	"log"
	"math/big"
	"testing"
)

const (
	expBase = 10
)

func Test_Cfmms(t *testing.T) {
	//cfmms.Cmd()
	//cfmms.GetAbiFromEtherscan("./cfmms/contracts_helper.json", "./cfmms/abi/")
	cfmms.GenerateGoFilesByAbi("./cfmms/abi", "cfmms/abi_go/")
	//cfmms.GenerateDeploymentGoFile([]string{"./contracts/uniswap_v2", "./contracts/uniswap_v3"}, "cfmms/deployment/")
}

func Test_Get_pairs_batch_request(t *testing.T) {

	client, err := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	//client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	//fmt.Println("we have a connection")
	factory := common.HexToAddress("0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f") //uniswav2_pool

	//
	from := big.NewInt(0)
	step := big.NewInt(766)
	pairs := batch_request_for_uniswap_v2.GetPairsBatchRequest(factory, from, step, client)
	fmt.Println(pairs)
	batch_request_for_uniswap_v2.Get_pool_data_batch_request(pairs[:127], client)

}

func Test_NewFromAddress(t *testing.T) {
	client, err := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	//client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	var v2pool pool.UniswapV2Pool
	res, _ := v2pool.NewFromAddress(common.HexToAddress("0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc"), client)

	fmt.Println(res)
	//address := res.GetAddress()
	//fmt.Println(address.(common.Address))
}

func Test_Abigo_Feature(t *testing.T) {

	//client, err := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	client, err := ethclient.Dial("wss://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	//client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}

	uniswapv2 := &dex.UniswapV2Dex{
		FactoryAddress: common.HexToAddress("0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f"),
		CreationBlock:  10000835,
	}

	ins, err := UniswapV2Factory.NewUniswapV2Factory(common.HexToAddress("0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f"), client)
	if err != nil {

		fmt.Println("Failed to new contract instance")
		log.Fatal(err)
	}

	current, _ := client.BlockNumber(context.Background())
	for i := uniswapv2.CreationBlock; i < current; i = i + 10000 {

		end := i + 10000

		fmt.Println("start", i, "end", end)
		res, err := ins.FilterPairCreated(&bind.FilterOpts{
			Start:   i,
			End:     &end,
			Context: nil,
		}, nil, nil)
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
			fmt.Println("finnnnnnnnnnnnnn", res.Event.Pair)
			//res = append(res, &entity.PairCreated{
			//	BlockNumber: iterator.Event.Raw.BlockNumber,
			//	Pair:        iterator.Event.Pair,
			//	Token0:      iterator.Event.Token0,
			//	Token1:      iterator.Event.Token1,
			//})
		}
	}

}

func Test_RemoveEmptyPools(t *testing.T) {

	//var pools []pool.Pool
	//
	//v2 := append(pools, &pool.UniswapV2Pool{
	//	Address:  common.HexToAddress("0x1"),
	//	Reserve0: big.NewInt(0),
	//}, &pool.UniswapV3Pool{
	//	Address: common.HexToAddress("0x2"),
	//})
	//
	//cfmms.RemoveEmptyPools(v2)
}

func Test_SyncPairsTest(t *testing.T) {
	cfmms.SyncPairsTest()
}

func Test_PriceV2(t *testing.T) {
	client, err := ethclient.Dial("http://188.40.132.112:8545")
	//client, err := ethclient.Dial("ws://188.40.132.112:8546")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}

	cfmms.CalculatePrice(client)
}

func Test_PriceV3(t *testing.T) {
	client, err := ethclient.Dial("http://188.40.132.112:8545")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}

	cfmms.CalculatePriceV3(client)
}

func Test_Fraction(t *testing.T) {
	deno := big.NewInt(0)
	deno.SetString("45649181384567604151811", 10)
	num := big.NewInt(0)
	num.SetString("15815507900982712632", 10)
	nul := utils.Fraction{
		Denominator: deno,
		Numerator:   num,
	}

	fmt.Println(nul.Quotient())
	fmt.Println(nul.Remainder())
	fmt.Println(nul.ToFixed(19))
	fmt.Println(nul.ToSignificant(8))
}

func Test_SimulateSwapV2(t *testing.T) {
	client, err := ethclient.Dial("http://188.40.132.112:8545")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	cfmms.SimulateSwapV2(client)

	//big.Int 2 big.Float
	//intValue := big.NewInt(123456789)
	//floatValue := new(big.Float).SetInt(intValue)
	//result, _ := floatValue.Float64()
	//fmt.Println(result)

}

func Test_DebugTraceCall(t *testing.T) {
	// todo: 模拟交易
	// todo: 查貔貅币种
	// todo: 含税币种

	rc, err := rpc.Dial("ws://188.40.132.112:8546")
	if err != nil {
		log.Printf("failed to dial: %v", err)
		return
	}
	gc := gethclient.New(rc)

	transactions := make(chan *types.Transaction, 100)
	_, err = gc.SubscribeFullPendingTransactions(context.Background(), transactions)
	if err != nil {
		log.Printf("failed to SubscribePendingTransactions: %v", err)
		return
	}
	log.Print("subscribed pending txs now")
	for {
		select {
		case transaction := <-transactions:
			// 这里的transaction是完整数据，可以直接使用
			txBytes, err := transaction.MarshalJSON()
			if err != nil {
				continue
			}

			// todo: debug_traceCall

			var result interface{}
			tracerOpt := "callTracer"
			chainId, err := ethclient.NewClient(rc).NetworkID(context.Background())
			if err != nil {
				fmt.Println("Failed to get chainId", err)
				return
			}
			arg, _ := toCallArg(transaction, chainId)
			logConfig := logger.Config{
				DisableStack:   true,
				DisableStorage: true,
				Debug:          true,
			}
			err = ethclient.NewClient(rc).Client().CallContext(context.Background(), &result, "debug_traceCall", arg, "latest", &tracers.TraceConfig{Config: &logConfig, Tracer: &tracerOpt})
			//if err != nil {
			//	return
			//}

			log.Printf("received tx %s", string(txBytes))
			log.Printf("received tx %s", result)
		}
	}
}

func toCallArg(tx *types.Transaction, chainID *big.Int) (interface{}, error) {
	from, err := types.Sender(types.LatestSignerForChainID(chainID), tx)
	if err != nil {
		return nil, err
	}
	arg := map[string]interface{}{
		"from": from,
		"to":   tx.To(),
	}
	if len(tx.Data()) > 0 {
		arg["data"] = hexutil.Bytes(tx.Data())
	}
	if tx.Value() != nil {
		arg["value"] = (*hexutil.Big)(tx.Value())
	}
	if tx.Gas() != 0 {
		arg["gas"] = hexutil.Uint64(tx.Gas())
	}
	if tx.GasPrice() != nil {
		arg["gasPrice"] = (*hexutil.Big)(tx.GasPrice())
	}
	return arg, nil
}

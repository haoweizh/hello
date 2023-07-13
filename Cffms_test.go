package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
	"hello/cfmms/pool"
	"math/big"
	"testing"
)

const (
	expBase = 10
)

func Test_Cfmms(t *testing.T) {
	//cfmms.Cmd()
	//cfmms.GetAbiFromEtherscan("./cfmms/contracts_helper.json", "./cfmms/abi/")
	//cfmms.GenerateGoFilesByAbi("./cfmms/abi", "cfmms/abi_go/")
	cfmms.GenerateDeploymentGoFile([]string{"./contracts/uniswap_v2", "./contracts/uniswap_v3"}, "cfmms/deployment/")
}

func Test_Get_pairs_batch_request(t *testing.T) {

	client, err := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	//client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	//fmt.Println("we have a connection")
	factory := common.HexToAddress("0x5c69bee701ef814a2b6a3edd4b1652cb9cc5aa6f") //uniswav2_pool
	from := big.NewInt(3064)
	step := big.NewInt(3727)
	pairs := batch_request_for_uniswap_v2.Get_pairs_batch_request(factory, from, step, client)
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
	res := v2pool.NewFromAddress(common.HexToAddress("0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc"), client)

	address := res.(*pool.UniswapV2Pool).GetAddress()
	fmt.Println(address.(common.Address))
}

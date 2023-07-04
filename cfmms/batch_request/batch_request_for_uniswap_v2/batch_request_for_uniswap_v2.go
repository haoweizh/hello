package batch_request_for_uniswap_v2

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/deployment/GetUniswapV2PairsBatchRequest"
	"log"
	"math/big"
)

func Get_pairs_batch_request(auth *bind.TransactOpts, factory common.Address, from, setp *big.Int, client *ethclient.Client) []string {
	var pairs []string

	address, tx, instance, err := GetUniswapV2PairsBatchRequest.DeployGetUniswapV2PairsBatchRequest(auth, client, from, setp, factory)

	fmt.Println(address.Hex())
	fmt.Println(tx.Data())
	fmt.Println(instance)
	if err != nil {
		log.Fatal(err)
	}

	return pairs

}

func get_pool_data_batch_request() {

}

func get_v2_pool_data_batch_request() {

}

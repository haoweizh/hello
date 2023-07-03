package batch_request_for_uniswap_v2

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

func get_pairs_batch_request(factor common.Address, from big.Int, setp big.Int, client *ethclient.Client) []string {
	var pairs []string

	//constructor_args := []interface{}{factor, from, setp}

	return pairs

}

func get_pool_data_batch_request() {

}

func get_v2_pool_data_batch_request() {

}

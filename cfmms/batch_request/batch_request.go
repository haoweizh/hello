package batch_request

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
	"hello/cfmms/pool/uniswav2_pool"
)

type BatchRequest interface {
	Get_pairs_batch_request() []string
	Get_pool_data_batch_request() []batch_request_for_uniswap_v2.PoolData
	Get_v2_pool_data_batch_request(pool *uniswav2_pool.UniswapV2Pool, client *ethclient.Client) error
}

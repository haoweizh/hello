package batch_request

import (
	"github.com/ethereum/go-ethereum/ethclient"
)

type BatchRequest interface {
	Get_pairs_batch_request() []string
	Get_pool_data_batch_request() any
	Get_v2_pool_data_batch_request(pool any, client *ethclient.Client) error
}

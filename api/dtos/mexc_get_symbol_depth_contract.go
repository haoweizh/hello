package dtos

// 备注: [411.8, 10, 1] 411.8为价格，10为此价格的合约张数, 1为订单数量
type MexcContractDepthHttpResp struct {
	Success bool `json:"success"`
	Code    int  `json:"code"`
	Data    struct {
		Asks      [][]float64 `json:"asks"`
		Bids      [][]float64 `json:"bids"`
		Version   int64       `json:"version"`
		Timestamp int         `json:"timestamp"`
	} `json:"data"`
}

type MexcContractDepthCommitsResp struct {
	Success bool `json:"success"`
	Code    int  `json:"code"`
	Data    []struct {
		Asks    [][]float64 `json:"asks"`
		Bids    [][]float64 `json:"bids"`
		Version int64       `json:"version"`
	} `json:"data"`
}

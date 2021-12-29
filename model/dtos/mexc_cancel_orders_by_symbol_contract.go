package dtos

type MexcContractCancelOrderBySymbolResp struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

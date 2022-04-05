package dtos

type MexcContractQueryOrderResp struct {
	Success bool `json:"success"`
	Code    int  `json:"code"`
	Data    struct {
		OrderID      string  `json:"orderId"`
		Symbol       string  `json:"symbol"`
		PositionID   int64   `json:"positionId"`
		Price        float64 `json:"price"`
		Vol          float64 `json:"vol"`
		Leverage     int64   `json:"leverage"`
		Side         int32   `json:"side"`
		Category     int32   `json:"category"`
		OrderType    int32   `json:"orderType"`
		DealAvgPrice float64 `json:"dealAvgPrice"`
		DealVol      float64 `json:"dealVol"`
		OrderMargin  float64 `json:"orderMargin"`
		TakerFee     float64 `json:"takerFee"`
		MakerFee     float64 `json:"makerFee"`
		Profit       float64 `json:"profit"`
		FeeCurrency  string  `json:"feeCurrency"`
		OpenType     int32   `json:"openType"`
		State        int32   `json:"state"`
		ExternalOid  string  `json:"externalOid"`
		ErrorCode    int32   `json:"errorCode"`
		UsedMargin   int32   `json:"usedMargin"`
		CreateTime   int64   `json:"createTime"`
		UpdateTime   int64   `json:"updateTime"`
	} `json:"data"`
}

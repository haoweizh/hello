package dtos

// {"channel":"rs.sub.depth","data":"success","ts":1640381555996}
type MexcSubscriptionWsResp struct {
	Channel string `json:"channel"`
	Data    string `json:"data"`
	Ts      int    `json:"ts"`
}

// MexcPongWsResp {"channel":"pong","data":1640381565996,"ts":1640381565996}
type MexcPongWsResp struct {
	Channel string `json:"channel"`
	Data    int    `json:"data"`
	Ts      int    `json:"ts"`
}

type MexcContractDepthWsResp struct {
	Channel string `json:"channel"`
	Data    struct {
		Asks    [][]float64 `json:"asks"`
		Bids    [][]float64 `json:"bids"`
		Version int64       `json:"version"`
	} `json:"data"`
	Symbol string `json:"symbol"`
	Ts     int    `json:"ts"`
}

type MexcContractTickerResp struct {
	Channel string `json:"channel"`
	Data    struct {
		Ask1          float64 `json:"ask1"`
		Bid1          float64 `json:"bid1"`
		ContractID    int     `json:"contractId"`
		FairPrice     float64 `json:"fairPrice"`
		FundingRate   float64 `json:"fundingRate"`
		High24Price   float64 `json:"high24Price"`
		IndexPrice    float64 `json:"indexPrice"`
		LastPrice     float64 `json:"lastPrice"`
		Lower24Price  float64 `json:"lower24Price"`
		MaxBidPrice   float64 `json:"maxBidPrice"`
		MinAskPrice   float64 `json:"minAskPrice"`
		RiseFallRate  float64 `json:"riseFallRate"`
		RiseFallValue float64 `json:"riseFallValue"`
		Symbol        string  `json:"symbol"`
		Timestamp     int     `json:"timestamp"`
		HoldVol       float64 `json:"holdVol"`
		Volume24      float64 `json:"volume24"`
	} `json:"data"`
	Symbol string `json:"symbol"`
	Ts     int64  `json:"ts"`
}

func (entity *MexcContractTickerResp) IsValidChannel() bool {
	return entity.Channel == "push.ticker"
}

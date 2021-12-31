package dtos

type MexcSpotQueryOrderResp struct {
	Code int `json:"code"`
	Data []struct {
		ID           string `json:"id"`
		Symbol       string `json:"symbol"`
		Price        string `json:"price"`
		Quantity     string `json:"quantity"`
		State        string `json:"state"`
		Type         string `json:"type"`
		DealQuantity string `json:"deal_quantity"`
		DealAmount   string `json:"deal_amount"`
		CreateTime   int64  `json:"create_time"`
	} `json:"data"`
}

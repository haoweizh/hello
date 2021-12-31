package dtos

const (
	emptyOrderID = ""
	successMsg   = "success"
)

type MexcSpotCancelOrderBySymbolResp struct {
	Code int `json:"code"`
	Data []struct {
		Msg           string `json:"msg"`
		OrderID       string `json:"order_id"`
		ClientOrderID string `json:"client_order_id,omitempty"`
	} `json:"data"`
}

func (resp *MexcSpotCancelOrderBySymbolResp) GetFailedOrderIDsMexc() (ret []string) {
	if len(resp.Data) == 0 {
		return
	}

	for _, data := range resp.Data {
		if data.OrderID != emptyOrderID && data.Msg != successMsg {
			ret = append(ret, data.OrderID)
		}
	}

	return ret
}

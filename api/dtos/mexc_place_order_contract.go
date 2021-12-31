package dtos

type MexcContractPlaceOrderRequest struct {
	Symbol   string  `json:"symbol"`   // 合约名
	Price    float64 `json:"price"`    // 价格
	Vol      float64 `json:"vol"`      // 数量
	Leverage int32   `json:"leverage"` // 杠杆倍数，逐仓时杠杆倍数必须传入
	Side     int32   `json:"side"`     // 订单方向 1开多，2平空，3开空，4平多
	Type     int32   `json:"type"`     // 订单类型 1：限价单，2：Post Only只做Maker，3：立即成交或立即取消，4：全部成交或者全部取消，5：市价单，6：市价转现价
	OpenType int32   `json:"openType"` // 开仓类型，1：逐仓，2：全仓
}

type MexcContractPlaceOrderResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    int64  `json:"data"`
}

func GenMexcContractPlaceOrderRequest(orderSide, orderType, symbol string, price, amount float64) (request *MexcContractPlaceOrderRequest) {
	// convert orderType to type
	var typeInReq int32
	if orderType == "LIMIT_ORDER" {
		typeInReq = 1
	}

	// convert orderSide to side
	var side int32
	if orderSide == "Buy" {
		side = 1
	} else {
		side = 3
	}

	return &MexcContractPlaceOrderRequest{
		Symbol:   symbol,
		Price:    price,
		Vol:      amount,
		Leverage: 5,          // 杠杆倍数，逐仓时杠杆倍数必须传入, 影响占用的保证金
		Side:     side,       // 1开多，2平空，3开空，4平多
		Type:     typeInReq,  // 1：限价单，2：Post Only只做Maker，3：立即成交或立即取消，4：全部成交或者全部取消，5：市价单，6：市价转现价
		OpenType: 2,          // 1：逐仓，2：全仓
	}
}

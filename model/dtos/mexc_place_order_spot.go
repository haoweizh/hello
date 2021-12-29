package dtos

import (
	"hello/model"
	"strconv"
)

type MexcOrderType string
type MexcTradeType string

const (
	LIMIT_ORDER         MexcOrderType = "LIMIT_ORDER"         // 限价订单
	POST_ONLY           MexcOrderType = "POST_ONLY"           // 限价做市单
	IMMEDIATE_OR_CANCEL MexcOrderType = "IMMEDIATE_OR_CANCEL" // 下单即撤销

	BID MexcTradeType = "BID" //买单
	ASK MexcTradeType = "ASK" //卖单
)

type MexcSpotPlaceOrderRequest struct {
	OrderType     string `json:"order_type"`
	Price         string `json:"price"`
	Quantity      string `json:"quantity"`
	Symbol        string `json:"symbol"`
	TradeType     string `json:"trade_type"`
	ClientOrderID string `json:"client_order_id,omitempty"`
}

type MexcSpotPlaceOrderResponse struct {
	Code int64  `json:"code"`
	Data string `json:"data"`
}

func GenMexcSpotPlaceOrderRequest(orderSide, orderType, symbol string, price, amount float64) (request *MexcSpotPlaceOrderRequest) {
	var tradeType MexcTradeType
	if orderSide == model.OrderSideBuy {
		tradeType = BID
	} else {
		tradeType = ASK
	}

	return &MexcSpotPlaceOrderRequest{
		OrderType: orderType,
		Price:     strconv.FormatFloat(price, 'f', -1, 64),
		Quantity:  strconv.FormatFloat(amount, 'f', -1, 64),
		Symbol:    symbol,
		TradeType: string(tradeType),
	}
}

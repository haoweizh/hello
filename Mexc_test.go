package main

import (
	"fmt"
	"hello/api"
	"hello/model"
	"testing"
)

//func Test_cancelOrdersMexc(t *testing.T) {
//	// t.Log(cancelOrdersMexc(key, secret, "ETH_USDT"))  // 取消现货订单
//	t.Log(api.cancelOrdersMexc(key, secret, "APX_USDT_SWAP")) // 取消合约订单
//}

func Test_placeOrderMexc(t *testing.T) {
	model.NewConfig()
	marketInfos := api.GetMarketInfos(model.Mexc)
	model.SetMarketInfos(model.Mexc, marketInfos)
	//api.GetPositions(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.Mexc)
	//_, rate := api.GetFundingRate(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.Mexc, `ETH_PERP`)
	//t.Log(rate)
	order := api.PlaceOrder(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.OrderSideBuy, model.OrderTypeLimit,
		model.Mexc, "ZIL_PERP", ``, 0.1193, 0.1193, 10, false, nil, nil)
	//t.Log(order.OrderId)
	//orderId := `266320760518756864`
	//order := api.QueryOrderById(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.Mexc, `ZIL_PERP`, ``, orderId)
	t.Log(fmt.Sprintf(`%v %f %s`, order.Status, order.DealAmount, order.OrderSide))
	api.CancelOrders(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.Mexc, `ZEC_PERP`)
}

//func Test_queryOrderMexc(t *testing.T) {
//	order := &model.Order{
//		OrderId: "123",
//		Market:  model.Mexc,
//		Symbol:  "BTC_USDT",
//	}
//	api.queryOrderMexc(key, secret, order)
//	t.Log(order)
//}
//
//func Test_getPositionsMexc(t *testing.T) {
//	_, _, ret := api.getPositionsMexc(key, secret)
//	t.Log(fmt.Sprintf("%+v", ret))
//}
//
//func Test_WsDepthServeMexc(t *testing.T) {
//	_, err := api.WsDepthServeMexc(nil, nil, true)
//	if err != nil {
//		return
//	}
//	t.Log("sub end")
//}

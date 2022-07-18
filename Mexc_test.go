package main

import (
	"fmt"
	"hello/api"
	"hello/model"
	"sync"
	"testing"
)

//func Test_cancelOrdersMexc(t *testing.T) {
//	// t.Log(cancelOrdersMexc(key, secret, "ETH_USDT"))  // 取消现货订单
//	t.Log(api.cancelOrdersMexc(key, secret, "APX_USDT_SWAP")) // 取消合约订单
//}

func Test_placeOrderMexc(t *testing.T) {
	testSync := sync.Map{}
	testSync.Store(`test`, make([]int, 0))
	array, _ := testSync.Load(`test`)
	array = append(array.([]int), 1)
	array = append(array.([]int), 2)
	array = append(array.([]int), 3)
	array = append(array.([]int), 4)
	array2, _ := testSync.Load(`test`)
	fmt.Println(fmt.Sprintf(`%v`, array2))
	fmt.Println(fmt.Sprintf(`%v`, array))
	model.NewConfig()
	marketInfos := api.GetMarketInfos(model.Ftx)
	model.SetMarketInfos(model.Mexc, marketInfos)
	//api.GetPositions(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.Mexc)
	//_, rate := api.GetFundingRate(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.Mexc, `ETH_PERP`)
	//t.Log(rate)
	symbol := `MKR_PERP`
	order := api.PlaceOrder(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.OrderSideBuy, model.OrderTypeLimit,
		model.Mexc, symbol, ``, 2014.31, 2054.31, 0.11343811097662475, false, nil, nil)
	//t.Log(order.OrderId)
	//orderId := `266320760518756864`
	//order := api.QueryOrderById(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.Mexc, symbol, ``, orderId)
	t.Log(fmt.Sprintf(`%v %f %s`, order.Status, order.DealAmount, order.OrderSide))
	api.CancelOrders(model.AppConfig.MexcKey, model.AppConfig.MexcSecret, model.Mexc, symbol)
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

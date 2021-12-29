package api

import (
	"fmt"
	"hello/model"
	"testing"
)

const (
	key    = "mx0ZRKQfI4KAq611TA"
	secret = "5ccd5831305142c19fef30794bc33dc7"
)

func Test_cancelOrdersMexc(t *testing.T) {
	// t.Log(cancelOrdersMexc(key, secret, "ETH_USDT"))  // 取消现货订单
	t.Log(cancelOrdersMexc(key, secret, "APX_USDT_SWAP")) // 取消合约订单
}

func Test_placeOrderMexc(t *testing.T) {
	success, marketInfos := getMarketsMexc(key, secret)
	if !success {
		t.Log("failed to get market infos")
		return
	}
	model.SetMarketInfos(model.Mexc, marketInfos)

	// t.Log(placeOrderMexc(key, secret, nil, "Buy", "LIMIT_ORDER", "ETH_USDT", 3900, 0.002))
	symbol := "APX_USDT_SWAP"
	t.Log(placeOrderMexc(key, secret, nil, "Buy", "LIMIT_ORDER", symbol, 0.207760, 1))
	t.Log(cancelOrdersMexc(key, secret, symbol))
}

func Test_queryOrderMexc(t *testing.T) {
	order := &model.Order{
		OrderId: "123",
		Market:  model.Mexc,
		Symbol:  "BTC_USDT",
	}
	queryOrderMexc(key, secret, order)
	t.Log(order)
}

func Test_getPositionsMexc(t *testing.T) {
	_, _, ret := getPositionsMexc(key, secret)
	t.Log(fmt.Sprintf("%+v", ret))
}

func Test_WsDepthServeMexc(t *testing.T) {
	WsDepthServeMexc(nil, nil, true)

	t.Log("sub end")
}

func Test_mexcGetContractSymbolDepth(t *testing.T) {
	resp, err := mexcGetContractSymbolDepth("BTC_USDT")
	if err != nil {
		t.Log(fmt.Sprintf("failed %v", err))
		t.Fail()
		return
	}
	fmt.Printf("%v", resp)
}

func Test_mexcGetContractSymbolDepthCommits(t *testing.T) {
	resp, err := mexcGetContractSymbolDepthCommits("BTC_USDT")
	if err != nil {
		t.Log(fmt.Sprintf("failed %v", err))
		t.Fail()
		return
	}
	fmt.Printf("%v", resp)
}

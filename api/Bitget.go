package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/api/dtos"
	"hello/model"
	"hello/util"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//const bitgetRestUrl = "https://api.bitget.com"
//const bitgetSpotWsUrl = "wss://ws.bitget.com/spot/v1/stream"
//const bitgetPerpWsUrl = "wss://ws.bitget.com/mix/v1/stream"

//func getMarketsBitget(key, secret string) (marketInfos map[string]*model.MarketInfo) {
//	marketInfos = make(map[string]*model.MarketInfo)
//	getMarketsBitgetSpot(marketInfos)
//	getMarketsBitgetPerp(marketInfos)
//	return marketInfos
//}

//func getMarketsBitgetSpot(marketInfos map[string]*model.MarketInfo) {
//	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/spot/v1/public/products", "", map[string]string{}, 30)
//	spotResp := &dtos.BitgetSpotMarketResp{}
//	spotJsonErr := json.Unmarshal(httpResp, spotResp)
//	if spotResp == nil || spotResp.Code != "00000" {
//		util.Notice(fmt.Sprintf("get bitget spot market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, spotJsonErr))
//		return
//	}
//	for _, symbolInfo := range spotResp.Data {
//		if symbolInfo.Status != "online" && symbolInfo.QuoteCoin != "USDT" {
//			continue
//		}
//		marketInfo := &model.MarketInfo{Name: symbolInfo.Symbol}
//		priceDecimal, _ := strconv.Atoi(symbolInfo.PriceScale)
//		marketInfo.PriceDecimal = priceDecimal
//		marketInfo.PriceIncrement = 1 / math.Pow10(priceDecimal)
//		amountPrecision, _ := strconv.Atoi(symbolInfo.QuantityScale)
//		marketInfo.SizeIncrement = 1 / math.Pow10(amountPrecision)
//		marketInfo.SizeMin, _ = strconv.ParseFloat(symbolInfo.MinTradeAmount, 64)
//		marketInfo.SizeMax, _ = strconv.ParseFloat(symbolInfo.MaxTradeAmount, 64)
//		marketInfo.UsdtMin, _ = strconv.ParseFloat(symbolInfo.MinTradeUSDT, 64)
//		marketInfos[marketInfo.Name] = marketInfo
//	}
//}

//func getMarketsBitgetPerp(marketInfos map[string]*model.MarketInfo) {
//	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/mix/v1/market/contracts?productType=umcbl", "", map[string]string{}, 30)
//	perpResp := &dtos.BitgetPerpMarketResp{}
//	perpJsonErr := json.Unmarshal(httpResp, perpResp)
//	if perpResp == nil || perpResp.Code != "00000" {
//		util.Notice(fmt.Sprintf("get bitget perp market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
//		return
//	}
//	for _, perpInfo := range perpResp.Data {
//		if perpInfo.QuoteCoin != "USDT" || perpInfo.SymbolStatus != "normal" || perpInfo.SymbolType != "perpetual" ||
//			perpInfo.OffTime != "-1" || perpInfo.LimitOpenTime != "-1" {
//			continue
//		}
//		symbol := perpInfo.BaseCoin + model.GetPerpTail(model.Bitget)
//		marketInfo := &model.MarketInfo{Name: symbol, CTCurrency: perpInfo.BaseCoin}
//		marketInfo.PriceDecimal, _ = strconv.Atoi(perpInfo.PricePlace)
//		priceEndStep, _ := strconv.ParseFloat(perpInfo.PriceEndStep, 64)
//		marketInfo.PriceIncrement = priceEndStep * (1 / math.Pow10(marketInfo.PriceDecimal))
//		marketInfo.SizeMin, _ = strconv.ParseFloat(perpInfo.MinTradeNum, 64)
//		marketInfo.SizeIncrement, _ = strconv.ParseFloat(perpInfo.SizeMultiplier, 64)
//		//marketInfo.CTValue, _ = strconv.ParseFloat(perpInfo.ContractSize, 64)
//		marketInfo.BuyLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.BuyLimitPriceRatio, 64)
//		marketInfo.SellLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.SellLimitPriceRatio, 64)
//		marketInfos[symbol] = marketInfo
//	}
//}

//func maintainChannelBitget() {
//	if !channelMaintainingXT {
//		channelMaintainingXT = true
//		go func() {
//			for true {
//				time.Sleep(time.Second * 20)
//				if err := sendToAllConnections(model.Bitget, []byte(`ping`)); err != nil {
//					util.SocketInfo("xt channel ping error " + err.Error())
//				}
//			}
//		}()
//	}
//}

//var subscribeHandlerBitget = func(connection *websocket.Conn, subscribes []interface{}, keyChannel string) error {
//	var err error = nil
//	var params []map[string]string
//	for _, subscribe := range subscribes {
//		symbol := strings.Split(subscribe.(string), "_")[0]
//		if strings.Contains(subscribe.(string), model.GetPerpTail(model.Bitget)) {
//			if keyChannel == model.SubscribeMarkPrice {
//				params = append(params, map[string]string{"instType": "MC", "channel": "ticker", "instId": symbol})
//			} else {
//				params = append(params, map[string]string{"instType": "mc", "channel": "books1", "instId": symbol})
//			}
//		} else {
//			params = append(params, map[string]string{"instType": "sp", "channel": "books5", "instId": symbol})
//		}
//	}
//	subscribeMap := make(map[string]interface{})
//	subscribeMap["op"] = "subscribe"
//	subscribeMap["args"] = params
//	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
//	if err = sendToConnection(connection, subscribeMessage); err != nil {
//		util.SocketInfo(" bitget can not subscribe %s %s", subscribeMessage, err.Error())
//	}
//	util.Notice(`bitget subscribed ` + string(subscribeMessage))
//	time.Sleep(1200 * time.Millisecond)
//	return err
//}

//func WsDepthServeBitget(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
//	markPriceWsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
//		//util.Notice(fmt.Sprintf("ws data: %s", event))
//		if len(event) == 4 {
//			return
//		}
//		tickerWsResp := &dtos.BitgetTickerWsResp{}
//		jsonErr := json.Unmarshal(event, tickerWsResp)
//		if jsonErr != nil {
//			util.SocketInfo(`bitget fail to unmarshal ticker ws data json ` + jsonErr.Error())
//			return
//		}
//		if tickerWsResp.Arg.InstType == "mc" && tickerWsResp.Action == "snapshot" {
//			for _, ticker := range tickerWsResp.Data {
//				symbol := ticker.SymbolId
//				price, _ := strconv.ParseFloat(ticker.MarkPrice, 64)
//				ts := int(ticker.SystemTime)
//				oldMarkPrice := markets.GetMarkPrice(symbol, model.Bitget)
//				if oldMarkPrice != nil && oldMarkPrice.Ts > ts {
//					return
//				}
//				markPrice := &model.MarkPrice{MarkPrice: price, Ts: ts}
//				markets.SetMarkPrice(symbol, model.Bitget, markPrice)
//			}
//		}
//	}
//	bookWsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
//		//util.Notice(fmt.Sprintf("ws data: %s", event))
//		if len(event) == 4 {
//			return
//		}
//		bookWsResp := &dtos.BitgetBoosWsResp{}
//		jsonErr := json.Unmarshal(event, bookWsResp)
//		if jsonErr != nil {
//			util.SocketInfo(`bitget fail to unmarshal book ws data json ` + jsonErr.Error())
//			return
//		}
//		if (bookWsResp.Arg.InstType == "sp" || bookWsResp.Arg.InstType == "mc") && bookWsResp.Action == "snapshot" {
//			if bookWsResp.Arg.InstId == "" || bookWsResp.Data == nil {
//				return
//			}
//			var symbol string
//			if bookWsResp.Arg.InstType == "sp" {
//				symbol = bookWsResp.Arg.InstId[0:len(bookWsResp.Arg.InstId)-4] + model.GetSpotTail(model.Bitget)
//			} else {
//				symbol = bookWsResp.Arg.InstId[0:len(bookWsResp.Arg.InstId)-4] + model.GetPerpTail(model.Bitget)
//			}
//
//			bidAsk := model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
//			if len(bookWsResp.Data) > 1 {
//				return
//			}
//			bidPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][0], 64)
//			bidAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][1], 64)
//			bids := make([]model.Tick, 0)
//			bids = append(bids, model.Tick{Price: bidPrice, Amount: bidAmount})
//			bidAsk.Bids = bids
//
//			askPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][0], 64)
//			askAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][1], 64)
//			asks := make([]model.Tick, 0)
//			asks = append(asks, model.Tick{Price: askPrice, Amount: askAmount})
//			bidAsk.Asks = asks
//
//			bidAsk.Ts, _ = strconv.Atoi(bookWsResp.Data[0].Ts)
//			bidAsk.UpdateId, _ = strconv.ParseInt(bookWsResp.Data[0].Ts, 10, 64)
//			haveOld, old := markets.GetBidAsk(symbol, model.Bitget)
//			if haveOld && old.UpdateId > bidAsk.UpdateId {
//				return
//			}
//			if markets.SetBidAsk(symbol, model.Bitget, &bidAsk) {
//				//util.Info(fmt.Sprintf("perp symbol: %s now bidAsk: %v", symbol, bidAsk))
//				for function, handler := range model.GetFunctions(model.Bitget, symbol) {
//					if handler != nil {
//						settings := model.GetSetting(function, model.Bitget, symbol)
//						for _, setting := range settings {
//							go handler(setting, &bidAsk)
//						}
//					}
//				}
//			}
//		}
//	}
//	channels = make([]chan struct{}, 0)
//	symbols := model.GetMarketSymbols(model.Bitget)
//	spotSubscribes := make([]interface{}, 0)
//	futureSubscribes := make([]interface{}, 0)
//	for symbol := range symbols {
//		if strings.Contains(symbol, model.GetPerpTail(model.Bitget)) {
//			futureSubscribes = append(futureSubscribes, symbol)
//		} else {
//			spotSubscribes = append(spotSubscribes, symbol)
//		}
//	}
//	GetWSSubscribes(model.Bitget, model.SubscribeDepth)
//	spotBookChannels, spotBookErr := WebSocketClient(model.Bitget, bitgetSpotWsUrl, model.SubscribeDepth,
//		spotSubscribes, subscribeHandlerBitget, bookWsHandler, orderHandler, 30)
//	if spotBookErr == nil {
//		util.Notice(`finish connect public Bitget spot book wss `)
//		channels = append(channels, spotBookChannels...)
//	}
//	time.Sleep(time.Second * 1)
//
//	perpBookChannels, perpBookErr := WebSocketClient(model.Bitget, bitgetPerpWsUrl, model.SubscribeDepth,
//		futureSubscribes, subscribeHandlerBitget, bookWsHandler, orderHandler, 30)
//	if perpBookErr == nil {
//		util.Notice(`finish connect public Bitget perp book wss `)
//		channels = append(channels, perpBookChannels...)
//	}
//	time.Sleep(time.Second * 1)
//
//	markPriceChannels, markPriceErr := WebSocketClient(model.Bitget, bitgetPerpWsUrl, model.SubscribeMarkPrice,
//		futureSubscribes, subscribeHandlerBitget, markPriceWsHandler, nil, 30)
//	if markPriceErr == nil {
//		util.Notice(`finish connect public xt Bitget mark price wss `)
//		channels = append(channels, markPriceChannels...)
//	}
//	return channels, nil
//}

func getBalanceBitget(key string, secret string) (success bool, balances []*model.Balance) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl}
	httpResp, httpErr := client.DoGet("/api/spot/v1/account/assets", map[string]string{})
	bitgetBalanceResp := &dtos.BitgetBalanceResp{}
	jsonErr := json.Unmarshal(httpResp, bitgetBalanceResp)
	if bitgetBalanceResp == nil || bitgetBalanceResp.Code != "00000" {
		util.SocketInfo(fmt.Sprintf("fail to refresh spot balance bitget, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		time.Sleep(time.Second * 2)
		return getBalanceBitget(key, secret)
	}
	balances = make([]*model.Balance, 0)
	for _, account := range bitgetBalanceResp.Data {
		balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Bitget, Coin: account.CoinName}
		balance.FrozenAmount, _ = strconv.ParseFloat(account.Frozen, 64)
		balance.Available, _ = strconv.ParseFloat(account.Available, 64)
		balance.AvailableWithBorrow, _ = strconv.ParseFloat(account.Available, 64)
		balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
		priceGet, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.GetSpotTail(model.Bitget), model.Bitget)
		if priceGet {
			balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
		}
		balances = append(balances, balance)
	}
	return true, balances
}

func getPositionsBitget(key, secret string) (success bool, positions []*model.Position, posBalance float64) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl}
	assetHttpResp, assetHttpErr := client.DoGet("/api/mix/v1/account/accounts", map[string]string{"productType": "umcbl"})
	bitgetAssertResp := &dtos.BitgetAssertResp{}
	jsonErr := json.Unmarshal(assetHttpResp, bitgetAssertResp)
	if bitgetAssertResp == nil || bitgetAssertResp.Code != "00000" {
		util.SocketInfo(fmt.Sprintf("fail to refresh contract asset bitget, resp: %s httpErr: %v, jsonErr: %v", assetHttpResp, assetHttpErr, jsonErr))
		time.Sleep(time.Second * 2)
		return getPositionsBitget(key, secret)
	}

	positionHttpResp, positionHttpErr := client.DoGet("/api/mix/v1/position/allPosition", map[string]string{"productType": "umcbl"})
	bitgetPositionResp := &dtos.BitgetPositionResp{}
	positionJsonErr := json.Unmarshal(positionHttpResp, bitgetPositionResp)
	if bitgetPositionResp == nil || bitgetPositionResp.Code != "00000" {
		util.SocketInfo(fmt.Sprintf("fail to refresh contract position bitget, resp: %s httpErr: %v, jsonErr: %v", positionHttpResp, positionHttpErr, positionJsonErr))
		time.Sleep(time.Second * 2)
		return getPositionsBitget(key, secret)
	}

	for _, asset := range bitgetAssertResp.Data {
		if asset.MarginCoin == `USDT` {
			assetBalance, _ := strconv.ParseFloat(asset.Available, 64)
			posBalance += assetBalance
		}
	}

	positions = make([]*model.Position, 0)
	for _, contract := range bitgetPositionResp.Data {
		currency := contract.Symbol
		position := &model.Position{Market: model.Bitget, Ts: util.GetNowUnixMillion(), Currency: currency}
		position.Direction = contract.HoldSide
		if position.Direction == "long" {
			position.Frozen, _ = strconv.ParseFloat(contract.Locked, 64)
			position.Free, _ = strconv.ParseFloat(contract.Total, 64)
		} else {
			frozen, _ := strconv.ParseFloat(contract.Locked, 64)
			position.Frozen = -1 * frozen
			total, _ := strconv.ParseFloat(contract.Total, 64)
			position.Free = -1 * total
		}
		position.LeverRate = int64(contract.Leverage)
		position.EntryPrice, _ = strconv.ParseFloat(contract.AverageOpenPrice, 64)
		position.Margin, _ = strconv.ParseFloat(contract.Margin, 64)
		positions = append(positions, position)
	}
	return true, positions, posBalance
}

//func setBitgetPositionMode() {
//	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl}
//	params := map[string]string{"productType": "umcbl", "holdMode": "single_hold"}
//	httpResp, httpErr := client.DoPost("/api/mix/v1/account/setPositionMode", string(util.JsonEncodeToByte(params)))
//	jsonData, jsonErr := util.NewJSON(httpResp)
//	code, _ := jsonData.Get("code").String()
//	if jsonData == nil || code != "00000" {
//		util.SocketInfo(fmt.Sprintf("fail to set Bitget Position Mode, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
//	}
//}

func placeOrderBitget(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	priceSpot, decimalSpot := model.FormatPrice(model.Bitget, symbol, orderSide, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Bitget, symbol, amount)))
	priceStr := util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
	if strings.Contains(symbol, model.GetPerpTail(model.Bitget)) {
		var tradeOrderSide string
		if orderSide == model.OrderSideBuy {
			tradeOrderSide = "buy_single"
		} else {
			tradeOrderSide = "sell_single"
		}

		client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl}
		params := map[string]string{
			"symbol":     symbol,
			"marginCoin": "USDT",
			"size":       amountStr,
			"price":      priceStr,
			"side":       tradeOrderSide,
			"orderType":  orderType,
		}
		httpResp, httpErr := client.DoPost("/api/mix/v1/account/setPositionMode", string(util.JsonEncodeToByte(params)))
		bitgetOrderResp := &dtos.BitgetOrderResp{}
		jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
		if bitgetOrderResp == nil || bitgetOrderResp.Code != "00000" {
			util.Notice(fmt.Sprintf("fail to create bitget spot order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		} else {
			order.Status = model.CarryStatusWorking
			order.OrderId = bitgetOrderResp.Data.OrderId
		}
	} else {
		client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl}
		params := map[string]string{
			"symbol":    symbol,
			"force":     "normal",
			"quantity":  amountStr,
			"price":     priceStr,
			"side":      orderSide,
			"orderType": orderType,
		}
		httpResp, httpErr := client.DoPost("/api/spot/v1/trade/orders", string(util.JsonEncodeToByte(params)))
		bitgetOrderResp := &dtos.BitgetOrderResp{}
		jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
		if bitgetOrderResp == nil || bitgetOrderResp.Code != "00000" {
			util.Notice(fmt.Sprintf("fail to create bitget spot order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		} else {
			order.Status = model.CarryStatusWorking
			order.OrderId = bitgetOrderResp.Data.OrderId
		}
	}
}

func cancelOrdersBitget(key, secret, symbol string) (result bool) {
	if strings.Contains(symbol, model.GetPerpTail(model.Bitget)) {
		client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl}
		planHttpResp, planHttpErr := client.DoGet("/api/spot/v1/trade/open-orders", map[string]string{"symbol": symbol})
		bitgetPerpOpenOrderResp := &dtos.BitgetPerpOpenOrderResp{}
		jsonErr := json.Unmarshal(planHttpResp, bitgetPerpOpenOrderResp)
		if bitgetPerpOpenOrderResp == nil || bitgetPerpOpenOrderResp.Code != "00000" {
			util.Notice(fmt.Sprintf("fail to get bitget perp open order resp: %s httpErr: %v, jsonErr: %v", planHttpResp, planHttpErr, jsonErr))
			return false
		}
		var orderIds []string
		for _, openOrder := range bitgetPerpOpenOrderResp.Data.OrderList {
			orderIds = append(orderIds, openOrder.OrderId)
		}

		params := map[string]interface{}{
			"symbol":     symbol,
			"marginCoin": "USDT",
			"orderIds":   orderIds,
		}
		httpResp, httpErr := client.DoPost("/api/mix/v1/account/setPositionMode", string(util.JsonEncodeToByte(params)))
		jsonData, jsonErr := util.NewJSON(httpResp)
		code, _ := jsonData.Get("code").String()
		if jsonData == nil || code != "00000" {
			util.SocketInfo(fmt.Sprintf("fail to canal Bitget perp order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
			return false
		}
	} else {
		client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl}
		planHttpResp, planHttpErr := client.DoPost("/api/spot/v1/trade/open-orders", string(util.JsonEncodeToByte(map[string]string{"symbol": symbol})))
		bitgetSpotOpenOrderResp := &dtos.BitgetSpotOpenOrderResp{}
		jsonErr := json.Unmarshal(planHttpResp, bitgetSpotOpenOrderResp)
		if bitgetSpotOpenOrderResp == nil || bitgetSpotOpenOrderResp.Code != "00000" {
			util.Notice(fmt.Sprintf("fail to get bitget spot open order resp: %s httpErr: %v, jsonErr: %v", planHttpResp, planHttpErr, jsonErr))
			return false
		}
		var orderIds []string
		for _, openOrder := range bitgetSpotOpenOrderResp.Data {
			orderIds = append(orderIds, openOrder.OrderId)
		}

		params := map[string]interface{}{
			"symbol":   symbol,
			"orderIds": orderIds,
		}
		httpResp, httpErr := client.DoPost("/api/spot/v1/trade/cancel-batch-orders", string(util.JsonEncodeToByte(params)))
		jsonData, jsonErr := util.NewJSON(httpResp)
		code, _ := jsonData.Get("code").String()
		if jsonData == nil || code != "00000" {
			util.SocketInfo(fmt.Sprintf("fail to canal Bitget spot order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
			return false
		}
	}
	return true
}

func getFundingRateBitget(key, secret, symbol string) (fundingRate *model.FundingRate) {
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/mix/v1/market/current-fundRate?symbol="+symbol, "", map[string]string{}, 30)
	bitgetFundingResp := &dtos.BitgetFundingResp{}
	perpJsonErr := json.Unmarshal(httpResp, bitgetFundingResp)
	if bitgetFundingResp == nil || bitgetFundingResp.Code != "00000" {
		util.Notice(fmt.Sprintf("get bitget perp market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
		return
	}
	rate, _ := strconv.ParseFloat(bitgetFundingResp.Data.FundingRate, 64)
	fundingRate = &model.FundingRate{
		Rate:       rate,
		UpdateTime: util.GetNow().Unix(),
		ExpireTime: util.GetNow().Unix() + 3600000/1000,
	}
	return fundingRate
}

//func transferBitget(transferType string, amount float64) {
//	var from, to string
//	amountStr := strconv.FormatFloat(amount, 'f', 0, 64)
//	if transferType == "MAIN_UMFUTURE" {
//		to = "mix_usdt"
//		from = "spot"
//	} else {
//		from = "mix_usdt"
//		to = "spot"
//	}
//
//	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl}
//	params := map[string]string{
//		"coin":     "USDT",
//		"fromType": from,
//		"toType":   to,
//		"amount":   amountStr,
//	}
//	httpResp, httpErr := client.DoPost("/api/spot/v1/wallet/transfer", string(util.JsonEncodeToByte(params)))
//	jsonData, jsonErr := util.NewJSON(httpResp)
//	code, _ := jsonData.Get("code").String()
//	if jsonData == nil || code != "00000" {
//		util.Notice(fmt.Sprintf("fail to transfer bitget resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
//	}
//}

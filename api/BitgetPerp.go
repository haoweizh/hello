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

const bitgetPerpWsUrl = "wss://ws.bitget.com/mix/v1/stream"

var channelMaintainingBitgetPerp = false

func getMarketsBitgetPerp(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/mix/v1/market/contracts?productType=umcbl", "", map[string]string{}, 30)
	perpResp := &dtos.BitgetPerpMarketResp{}
	perpJsonErr := json.Unmarshal(httpResp, perpResp)
	if perpResp == nil || perpResp.Code != "00000" {
		util.Notice(fmt.Sprintf("get bitget perp market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
		return
	}
	marketInfos = make(map[string]*model.MarketInfo)
	for _, perpInfo := range perpResp.Data {
		if perpInfo.QuoteCoin != "USDT" || perpInfo.SymbolStatus != "normal" || perpInfo.SymbolType != "perpetual" ||
			perpInfo.OffTime != "-1" || perpInfo.LimitOpenTime != "-1" {
			continue
		}
		symbol := perpInfo.BaseCoin + model.UniStandardTail[model.MarketTypePerp]
		marketInfo := &model.MarketInfo{Name: symbol, CTCurrency: perpInfo.BaseCoin}
		marketInfo.PriceDecimal, _ = strconv.Atoi(perpInfo.PricePlace)
		priceEndStep, _ := strconv.ParseFloat(perpInfo.PriceEndStep, 64)
		marketInfo.PriceIncrement = priceEndStep * (1 / math.Pow10(marketInfo.PriceDecimal))
		marketInfo.SizeMin, _ = strconv.ParseFloat(perpInfo.MinTradeNum, 64)
		marketInfo.SizeIncrement, _ = strconv.ParseFloat(perpInfo.SizeMultiplier, 64)
		//marketInfo.CTValue, _ = strconv.ParseFloat(perpInfo.ContractSize, 64)
		marketInfo.BuyLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.BuyLimitPriceRatio, 64)
		marketInfo.SellLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.SellLimitPriceRatio, 64)
		marketInfo.MoneyMin = 5.0
		marketInfos[symbol] = marketInfo
	}
	return marketInfos
}

func setBitgetPositionMode(key, secret string) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]string{"productType": "umcbl", "holdMode": "single_hold"}
	httpResp, httpErr := client.DoPost("/api/mix/v1/account/setPositionMode", string(util.JsonEncodeToByte(params)))
	jsonData, jsonErr := util.NewJSON(httpResp)
	code, _ := jsonData.Get("code").String()
	if jsonData == nil || code != "00000" {
		util.SocketInfo(fmt.Sprintf("fail to set Bitget Position Mode, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	}
}

func WsDepthServeBitgetPerp(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	bookWsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		//util.Notice(fmt.Sprintf("ws data: %s", event))
		if len(event) == 4 {
			return
		}
		bookWsResp := &dtos.BitgetBoosWsResp{}
		jsonErr := json.Unmarshal(event, bookWsResp)
		if jsonErr != nil {
			util.SocketInfo(`bitget fail to unmarshal book ws data json ` + jsonErr.Error())
			return
		}
		if bookWsResp.Arg.InstType == "mc" && bookWsResp.Action == "snapshot" {
			if bookWsResp.Arg.InstId == "" || bookWsResp.Data == nil {
				return
			}
			symbol := bookWsResp.Arg.InstId[0:len(bookWsResp.Arg.InstId)-4] + model.UniStandardTail[model.MarketTypePerp]
			bidAsk := model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
			if len(bookWsResp.Data) > 1 {
				return
			}
			bidPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][0], 64)
			bidAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][1], 64)
			bids := make([]model.Tick, 0)
			bids = append(bids, model.Tick{Price: bidPrice, Amount: bidAmount})
			bidAsk.Bids = bids

			askPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][0], 64)
			askAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][1], 64)
			asks := make([]model.Tick, 0)
			asks = append(asks, model.Tick{Price: askPrice, Amount: askAmount})
			bidAsk.Asks = asks

			bidAsk.Ts, _ = strconv.Atoi(bookWsResp.Data[0].Ts)
			bidAsk.UpdateId, _ = strconv.ParseInt(bookWsResp.Data[0].Ts, 10, 64)
			haveOld, old := markets.GetBidAsk(symbol, model.BitgetPerp)
			if haveOld && old.UpdateId > bidAsk.UpdateId {
				return
			}
			if markets.SetBidAsk(symbol, model.BitgetPerp, &bidAsk) {
				//util.Info(fmt.Sprintf("perp symbol: %s now bidAsk: %v", symbol, bidAsk))
				funcHandlers := GetFunctions(model.BitgetPerp, symbol)
				if funcHandlers != nil {
					funcHandlers.Range(func(function, value interface{}) bool {
						if model.IgnoreFunctions[function.(string)] {
							return true
						}
						setting := GetSetting(function.(string), model.BitgetPerp, symbol)
						if setting != nil && value != nil {
							go value.(model.CarryHandler)(setting, &bidAsk)
						}
						return true
					})
				}
			}
		}
	}
	markPriceWsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		//util.Notice(fmt.Sprintf("ws data: %s", event))
		if len(event) == 4 {
			return
		}
		tickerWsResp := &dtos.BitgetTickerWsResp{}
		jsonErr := json.Unmarshal(event, tickerWsResp)
		if jsonErr != nil {
			util.SocketInfo(`bitget fail to unmarshal ticker ws data json ` + jsonErr.Error())
			return
		}
		if tickerWsResp.Arg.InstType == "mc" && tickerWsResp.Action == "snapshot" {
			for _, tickerData := range tickerWsResp.Data {
				symbol := tickerData.SymbolId[0:len(tickerData.SymbolId)-4] + model.UniStandardTail[model.MarketTypePerp]
				price, _ := strconv.ParseFloat(tickerData.MarkPrice, 64)
				ticker := &model.Ticker{MarkPrice: price, Ts: int(tickerData.SystemTime)}
				markets.SetTicker(symbol, model.BitgetPerp, ticker)
				//rate, _ := strconv.ParseFloat(tickerData.CapitalRate, 64)
				//fundingRate := &model.FundingRate{
				//	Rate:       rate,
				//	UpdateTime: util.GetNow(),
				//	ExpireTime: tickerData.NextSettleTime,
				//}
				//model.SetFundingRate(model.BitgetPerp, symbol, fundingRate)
			}
		}
	}

	channels = make([]chan struct{}, 0)
	symbols := GetMarketSymbols(model.BitgetPerp)
	futureSubscribes := make([]interface{}, 0)
	for symbol := range symbols {
		futureSubscribes = append(futureSubscribes, symbol)
	}
	perpBookChannels, perpBookErr := WebSocketClient(model.BitgetPerp, bitgetPerpWsUrl,
		futureSubscribes, subscribeHandlerBitgetPerpBookTicker, bookWsHandler, orderHandler, 30)
	if perpBookErr == nil {
		util.Notice(`finish connect public Bitget perp book wss `)
		channels = append(channels, perpBookChannels...)
	} else {
		util.Notice(`fail to connect public Bitget perp book wss `)
		return nil, perpBookErr
	}
	time.Sleep(time.Second * 1)

	markPriceChannels, markPriceErr := WebSocketClient(model.BitgetPerp, bitgetPerpWsUrl,
		futureSubscribes, subscribeHandlerBitgetPerpMarkPrice, markPriceWsHandler, nil, 30)
	if markPriceErr == nil {
		util.Notice(`finish connect public xt Bitget mark price wss `)
		channels = append(channels, markPriceChannels...)
	} else {
		util.Notice(`fail to connect public Bitget mark price wss `)
		return nil, markPriceErr
	}
	go maintainChannelBitgetPerp()
	return channels, nil
}

var subscribeHandlerBitgetPerpBookTicker = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	var params []map[string]string
	for _, subscribe := range subscribes {
		symbol := strings.Split(subscribe.(string), "_")[0]
		params = append(params, map[string]string{"instType": "mc", "channel": "books1", "instId": symbol})
	}
	subscribeMap := make(map[string]interface{})
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = params
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.BitgetPerp, connection, subscribeMessage); err != nil {
		util.SocketInfo(" bitget can not subscribe %s %s", subscribeMessage, err.Error())
	}
	util.Notice(`bitget subscribed ` + string(subscribeMessage))
	time.Sleep(1200 * time.Millisecond)
	return err
}

var subscribeHandlerBitgetPerpMarkPrice = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	var params []map[string]string
	for _, subscribe := range subscribes {
		symbol := strings.Split(subscribe.(string), "_")[0]
		params = append(params, map[string]string{"instType": "MC", "channel": "ticker", "instId": symbol})
	}
	subscribeMap := make(map[string]interface{})
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = params
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.BitgetPerp, connection, subscribeMessage); err != nil {
		util.SocketInfo(" bitget can not subscribe %s %s", subscribeMessage, err.Error())
	}
	util.Notice(`bitget subscribed ` + string(subscribeMessage))
	time.Sleep(1200 * time.Millisecond)
	return err
}

func maintainChannelBitgetPerp() {
	if !channelMaintainingBitgetPerp {
		channelMaintainingBitgetPerp = true
		go func() {
			for true {
				time.Sleep(time.Second * 20)
				if err := SendToAllConnections(model.BitgetPerp, []byte(`ping`)); err != nil {
					util.SocketInfo("xt channel ping error " + err.Error())
				}
			}
		}()
	}
}

func getPositionsBitgetPerp(key, secret string) (success bool, positions []*model.Position, accountValue, availableU float64) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	assetHttpResp, assetHttpErr := client.DoGet("/api/mix/v1/account/accounts", map[string]string{"productType": "umcbl"})
	bitgetAssertResp := &dtos.BitgetAssertResp{}
	jsonErr := json.Unmarshal(assetHttpResp, bitgetAssertResp)
	if bitgetAssertResp == nil || bitgetAssertResp.Code != "00000" {
		util.SocketInfo(fmt.Sprintf("fail to refresh contract asset bitget, resp: %s httpErr: %v, jsonErr: %v", assetHttpResp, assetHttpErr, jsonErr))
		time.Sleep(time.Second * 2)
		return getPositionsBitgetPerp(key, secret)
	}

	positionHttpResp, positionHttpErr := client.DoGet("/api/mix/v1/position/allPosition", map[string]string{"productType": "umcbl"})
	bitgetPositionResp := &dtos.BitgetPositionResp{}
	positionJsonErr := json.Unmarshal(positionHttpResp, bitgetPositionResp)
	if bitgetPositionResp == nil || bitgetPositionResp.Code != "00000" {
		util.SocketInfo(fmt.Sprintf("fail to refresh contract position bitget, resp: %s httpErr: %v, jsonErr: %v", positionHttpResp, positionHttpErr, positionJsonErr))
		time.Sleep(time.Second * 2)
		return getPositionsBitgetPerp(key, secret)
	}

	for _, asset := range bitgetAssertResp.Data {
		if asset.MarginCoin == `USDT` {
			availableUsdt, _ := strconv.ParseFloat(asset.Available, 64)
			equityUsdt, _ := strconv.ParseFloat(asset.UsdtEquity, 64)
			accountValue += equityUsdt
			availableU += availableUsdt
		}
	}

	positions = make([]*model.Position, 0)
	for _, contract := range bitgetPositionResp.Data {
		isSuccess, _, coin := model.GetCoinFromDialect(model.BitgetPerp, contract.Symbol)
		if !isSuccess {
			continue
		}
		currency := coin + model.UniStandardTail[model.MarketTypePerp]
		position := &model.Position{Market: model.BitgetPerp, Ts: util.GetNowUnixMillion(), Currency: currency}
		position.Direction = contract.HoldSide
		if position.Direction == "long" {
			position.Frozen, _ = strconv.ParseFloat(contract.Locked, 64)
			position.Holding, _ = strconv.ParseFloat(contract.Total, 64)
		} else {
			frozen, _ := strconv.ParseFloat(contract.Locked, 64)
			position.Frozen = -1 * frozen
			total, _ := strconv.ParseFloat(contract.Total, 64)
			position.Holding = -1 * total
		}
		position.LeverRate = int64(contract.Leverage)
		position.EntryPrice, _ = strconv.ParseFloat(contract.AverageOpenPrice, 64)
		position.Margin, _ = strconv.ParseFloat(contract.Margin, 64)
		positions = append(positions, position)
	}
	return true, positions, accountValue, availableU
}

func getFundingRateBitgetPerp(key, secret, symbol string) (fundingRate *model.FundingRate) {
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
		UpdateTime: util.GetNow(),
		ExpireTime: util.GetNow().Unix() + 3600000/1000,
	} //没有过期时间
	return fundingRate
}

func placeOrderBitgetPerp(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Notice("fail to place perp order, GetFromStandard: " + symbol)
		return
	}
	priceSpot, decimalSpot := model.FormatPrice(model.BitgetPerp, symbol, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.BitgetPerp, symbol, amount, priceSpot)))
	priceStr := util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
	var tradeOrderSide string
	if orderSide == model.OrderSideBuy {
		tradeOrderSide = "buy_single"
	} else {
		tradeOrderSide = "sell_single"
	}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]string{
		"symbol":     dialectSymbol,
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
}

func cancelOrdersBitgetPerp(key, secret, symbol string) (result bool) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Notice("fail to cancel perp order, GetFromStandard: " + symbol)
		return false
	}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	planHttpResp, planHttpErr := client.DoGet("/api/spot/v1/trade/open-orders", map[string]string{"symbol": dialectSymbol})
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
		"symbol":     dialectSymbol,
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

	return true
}

func queryOrderBitgetPerp(key, secret, symbol string, orderId string) (order *model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Notice("fail to query perp order, GetFromStandard: " + symbol)
		return order
	}
	order = &model.Order{Market: model.BitgetPerp, Status: model.CarryStatusFail, OrderId: orderId, Symbol: symbol}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	httpResp, httpErr := client.DoGet("/api/mix/v1/order/detail", map[string]string{"symbol": dialectSymbol, "orderId": orderId})
	orderDetailResp := &dtos.BitgetPerpOrderDetailResp{}
	perpJsonErr := json.Unmarshal(httpResp, orderDetailResp)
	if orderDetailResp == nil || orderDetailResp.Code != "00000" {
		util.Notice(fmt.Sprintf("get bitget perp order detail error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
		return order
	} else {
		order.DealPrice = orderDetailResp.Data.PriceAvg
		order.DealAmount = orderDetailResp.Data.FilledQty
		order.UnfilledQuantity = orderDetailResp.Data.Size - orderDetailResp.Data.FilledQty
		if orderDetailResp.Data.State == "canceled" {
			order.Status = model.CarryStatusFail
		} else if orderDetailResp.Data.State == "filled" || orderDetailResp.Data.State == "partially_filled" {
			order.Status = model.CarryStatusSuccess
		}
	}
	return order
}

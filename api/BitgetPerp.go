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

func getMarketsBitgetPerp() (marketInfos map[string]*model.MarketInfo) {
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
		marketInfo := &model.MarketInfo{Market: model.BitgetPerp, Name: symbol, CTCurrency: perpInfo.BaseCoin}
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
	if httpErr != nil {
		util.Notice(fmt.Sprintf(`fail to do post when setBitgetPositionMode %s`, httpErr.Error()))
		return
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Notice(fmt.Sprintf(`fail to NewJson when setBitgetPositionMode %s`, jsonErr.Error()))
		return
	}
	if jsonData != nil {
		code, codeErr := jsonData.Get("code").String()
		if code != "00000" || codeErr != nil {
			util.Notice(fmt.Sprintf("fail to set Bitgetperp Position Mode, resp: %s codeErr: %v", httpResp, codeErr))
		}
	}
}

func WsDepthServeBitgetPerp(markets *model.Markets) (channels []chan struct{}, err error) {
	bookWsHandler := func(event []byte) {
		//util.Notice(fmt.Sprintf("bitget perp ws book ticker: %s", event))
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
			if bookWsResp.Arg.InstId == "" || !util.EndWith(bookWsResp.Arg.InstId, "USDT") || bookWsResp.Data == nil {
				return
			}
			symbol := bookWsResp.Arg.InstId[0:len(bookWsResp.Arg.InstId)-4] + model.UniStandardTail[model.MarketTypePerp]
			bidAsk := model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
			if len(bookWsResp.Data) > 1 ||
				len(bookWsResp.Data[0].Bids) < 1 || len(bookWsResp.Data[0].Bids[0]) < 2 ||
				len(bookWsResp.Data[0].Asks) < 1 || len(bookWsResp.Data[0].Asks[0]) < 2 {
				return
			}
			bidPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][0], 64)
			bidAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][1], 64)
			bids := make([]model.Tick, 0)
			bids = append(bids, model.Tick{Price: bidPrice, Amount: bidAmount, Market: model.BitgetPerp, Symbol: symbol})
			bidAsk.Bids = bids
			askPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][0], 64)
			askAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][1], 64)
			asks := make([]model.Tick, 0)
			asks = append(asks, model.Tick{Price: askPrice, Amount: askAmount, Market: model.BitgetPerp, Symbol: symbol})
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
						setting := GetSetting(function.(string), model.BitgetPerp, symbol)
						if setting != nil && value != nil && value.(model.CarryHandler) != nil {
							go value.(model.CarryHandler)(setting, &bidAsk)
						}
						return true
					})
				}
			}
		}
	}
	markPriceWsHandler := func(event []byte) {
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
				if tickerData.SymbolId == "" {
					continue
				}
				_, _, coin := model.GetCoinFromDialect(model.BitgetPerp, tickerData.SymbolId)
				symbol := coin + model.UniStandardTail[model.MarketTypePerp]
				price, _ := strconv.ParseFloat(tickerData.MarkPrice, 64)
				ticker := &model.MarkPriceInfo{MarkPrice: price, Ts: int(tickerData.SystemTime)}
				markets.SetMarkPriceInfo(symbol, model.BitgetPerp, ticker)
				rate, _ := strconv.ParseFloat(tickerData.CapitalRate, 64)
				fundingRate := &model.FundingRate{
					Rate:       rate,
					UpdateTime: util.GetNow(),
					ExpireTime: tickerData.NextSettleTime / 1000,
				}
				model.SetFundingRate(model.BitgetPerp, symbol, fundingRate)
			}
		}
	}
	channels = make([]chan struct{}, 0)
	symbols := GetMarketSymbols(model.BitgetPerp)
	futureSubscribes := make([]interface{}, 0)
	for symbol := range symbols {
		futureSubscribes = append(futureSubscribes, symbol)
	}
	markPriceChannels, markPriceErr := WebSocketClient(model.BitgetPerp, bitgetPerpWsUrl,
		futureSubscribes, subscribeHandlerBitgetPerpMarkPrice, markPriceWsHandler, 30)
	if markPriceErr == nil {
		util.Info(`finish connect public Bitget mark price wss `)
		channels = append(channels, markPriceChannels...)
	} else {
		util.Notice(`fail to connect public Bitget mark price wss `)
		return nil, markPriceErr
	}
	time.Sleep(time.Second * 1)
	perpBookChannels, perpBookErr := WebSocketClient(model.BitgetPerp, bitgetPerpWsUrl,
		futureSubscribes, subscribeHandlerBitgetPerpBookTicker, bookWsHandler, 30)
	if perpBookErr == nil {
		util.Info(`finish connect public Bitget perp book wss `)
		channels = append(channels, perpBookChannels...)
	} else {
		util.Notice(`fail to connect public Bitget perp book wss `)
		return nil, perpBookErr
	}
	go maintainChannelBitgetPerp()
	return channels, nil
}

var subscribeHandlerBitgetPerpBookTicker = func(market string, connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	var params []map[string]string
	for _, subscribe := range subscribes {
		success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, subscribe.(string))
		if !success {
			continue
		}
		symbol := strings.Split(dialectSymbol, "_")[0]
		params = append(params, map[string]string{"instType": "mc", "channel": "books1", "instId": symbol})
	}
	subscribeMap := make(map[string]interface{})
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = params
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.BitgetPerp, connection, subscribeMessage); err != nil {
		util.Info(" bitget can not subscribe %s %s", subscribeMessage, err.Error())
	}
	util.Info(`bitget subscribed ` + string(subscribeMessage))
	time.Sleep(time.Second)
	return err
}

var subscribeHandlerBitgetPerpMarkPrice = func(market string, connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	var params []map[string]string
	for _, subscribe := range subscribes {
		success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, subscribe.(string))
		if !success {
			continue
		}
		symbol := strings.Split(dialectSymbol, "_")[0]
		params = append(params, map[string]string{"instType": "MC", "channel": "ticker", "instId": symbol})
	}
	subscribeMap := make(map[string]interface{})
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = params
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.BitgetPerp, connection, subscribeMessage); err != nil {
		util.Info(" bitget can not subscribe %s %s", subscribeMessage, err.Error())
	}
	util.Info(`bitget subscribed ` + string(subscribeMessage))
	time.Sleep(time.Second)
	return err
}

func maintainChannelBitgetPerp() {
	if !channelMaintainingBitgetPerp {
		channelMaintainingBitgetPerp = true
		go func() {
			for {
				time.Sleep(time.Second * 20)
				if err := SendToAllConnections(model.BitgetPerp, []byte(`ping`)); err != nil {
					util.Info("bitget perp channel ping error " + err.Error())
				} else {
					util.Info("bitget perp channel ping success")
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
		util.Notice(fmt.Sprintf("fail to refresh bitgetperp asset , resp: %s httpErr: %v, jsonErr: %v", assetHttpResp, assetHttpErr, jsonErr))
		time.Sleep(time.Minute)
		return getPositionsBitgetPerp(key, secret)
	} else {
		util.SocketInfo(fmt.Sprintf("get bitgetperp asset success, resp: %s ", assetHttpResp))
	}
	positionHttpResp, positionHttpErr := client.DoGet("/api/mix/v1/position/allPosition", map[string]string{"productType": "umcbl"})
	bitgetPositionResp := &dtos.BitgetPositionResp{}
	positionJsonErr := json.Unmarshal(positionHttpResp, bitgetPositionResp)
	if bitgetPositionResp == nil || bitgetPositionResp.Code != "00000" {
		util.Notice(fmt.Sprintf("fail to refresh bitgetperp position, resp: %s httpErr: %v, jsonErr: %v", positionHttpResp, positionHttpErr, positionJsonErr))
		time.Sleep(time.Minute)
		return getPositionsBitgetPerp(key, secret)
	} else {
		util.SocketInfo(fmt.Sprintf("get bitgetperp position success, resp: %s ", positionHttpResp))
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
		total, _ := strconv.ParseFloat(contract.Total, 64)
		if total == 0 {
			continue
		}
		if position.Direction == "long" {
			position.Frozen, _ = strconv.ParseFloat(contract.Locked, 64)
			position.Holding = total
		} else {
			frozen, _ := strconv.ParseFloat(contract.Locked, 64)
			position.Frozen = -1 * frozen
			position.Holding = -1 * total
		}
		position.LeverRate = int64(contract.Leverage)
		position.EntryPrice, _ = strconv.ParseFloat(contract.AverageOpenPrice, 64)
		position.Margin, _ = strconv.ParseFloat(contract.Margin, 64)
		positions = append(positions, position)
	}
	if len(positions) == 0 && accountValue > 0 {
		util.Notice(fmt.Sprintf(`pos error bitgetperp %d`, len(bitgetPositionResp.Data)))
	}
	return true, positions, accountValue, availableU
}

func getFundingRateBitgetPerp(symbol string) (fundingRate *model.FundingRate) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Notice("fail to get perp funding rate , GetFromStandard: " + symbol)
		return
	}
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/mix/v1/market/current-fundRate?symbol="+dialectSymbol, "", map[string]string{}, 30)
	bitgetFundingResp := &dtos.BitgetFundingResp{}
	perpJsonErr := json.Unmarshal(httpResp, bitgetFundingResp)
	if bitgetFundingResp == nil || bitgetFundingResp.Code != "00000" {
		util.Notice(fmt.Sprintf("get bitget perp funding rate error, %s resp: %s, httpErr: %v, jsonErr: %v",
			symbol, httpResp, httpErr, perpJsonErr))
		return
	}
	rate, _ := strconv.ParseFloat(bitgetFundingResp.Data.FundingRate, 64)
	return &model.FundingRate{
		Rate:       rate,
		UpdateTime: util.GetNow(),
		ExpireTime: util.GetNow().Unix() + 3600} //没有过期时间
}

func placeOrderBitgetPerp(key, secret string, order *model.Order, orderSide, orderType, orderParam, symbol string, price, amount float64) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Notice("fail to place perp order, GetFromStandard: " + symbol)
		return
	}
	reduceOnly := false
	if orderParam == model.ReduceOnly {
		reduceOnly = true
	}
	priceSpot, decimalSpot := model.FormatPrice(model.BitgetPerp, symbol, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.BitgetPerp, symbol, amount, priceSpot, reduceOnly)))
	priceStr := util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
	var tradeOrderSide string
	if orderSide == model.OrderSideBuy {
		tradeOrderSide = "buy_single"
	} else {
		tradeOrderSide = "sell_single"
	}
	ordType := ``
	if orderType == model.OrderTypeMarket {
		ordType = `market`
	} else if orderType == model.OrderTypeLimit {
		ordType = `limit`
	}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]interface{}{
		"symbol":     dialectSymbol,
		"marginCoin": "USDT",
		"size":       amountStr,
		"price":      priceStr,
		"side":       tradeOrderSide,
		"orderType":  ordType,
		"reduceOnly": reduceOnly}
	httpResp, httpErr := client.DoPost("/api/mix/v1/order/placeOrder", string(util.JsonEncodeToByte(params)))
	bitgetOrderResp := &dtos.BitgetOrderResp{}
	jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
	util.Notice(fmt.Sprintf(`place bitgetperp %v`, params))
	if bitgetOrderResp == nil {
		util.Notice(fmt.Sprintf("fail to create bitget perp order no resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	} else {
		if bitgetOrderResp.Code == "00000" {
			order.Status = model.CarryStatusWorking
			order.OrderId = bitgetOrderResp.Data.OrderId
		}
		util.Notice(fmt.Sprintf("create bitget perp order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		order.ErrCode = bitgetOrderResp.Code
	}
}

func cancelOrdersBitgetPerp(key, secret, symbol string) (result bool) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Notice("fail to cancel bitget perp order, GetFromStandard: " + symbol)
		return false
	}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]interface{}{
		"symbol":     dialectSymbol,
		"marginCoin": "USDT",
	}
	httpResp, httpErr := client.DoPost("/api/mix/v1/order/cancel-symbol-orders", string(util.JsonEncodeToByte(params)))
	if httpErr != nil {
		util.Notice(fmt.Sprintf(`fail to do post when cancelOrdersBitgetPerp %s`, httpErr.Error()))
		return
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Notice(fmt.Sprintf(`fail to NewJson when cancelOrdersBitgetPerp %s`, jsonErr.Error()))
		return
	}
	if jsonData != nil {
		code, _ := jsonData.Get("code").String()
		if code == "00000" {
			return true
		}
	}
	return false
}

func queryOrderBitgetPerp(key, secret, symbol string, orderId string) (order *model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Notice("fail to query bitget perp order, GetFromStandard: " + symbol)
		return order
	}
	order = &model.Order{Market: model.BitgetPerp, Status: model.CarryStatusWorking, OrderId: orderId, Symbol: symbol}
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
		intOrderTime, _ := strconv.ParseInt(orderDetailResp.Data.CTime, 10, 64)
		order.OrderTime = time.UnixMilli(intOrderTime)
		intUpdateTime, _ := strconv.ParseInt(orderDetailResp.Data.UTime, 10, 64)
		order.OrderUpdateTime = time.UnixMilli(intUpdateTime)
		order.UnfilledQuantity = orderDetailResp.Data.Size - orderDetailResp.Data.FilledQty
		if orderDetailResp.Data.State == "canceled" {
			order.Status = model.CarryStatusFail
		} else if orderDetailResp.Data.State == "filled" || orderDetailResp.Data.State == "partially_filled" {
			order.Status = model.CarryStatusSuccess
		}
	}
	return order
}

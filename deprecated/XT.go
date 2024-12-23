package deprecated

//
//import (
//	"encoding/json"
//	"fmt"
//	"github.com/gorilla/websocket"
//	"hello/api/dtos"
//	"hello/model"
//	"hello/util"
//	"math"
//	"net/http"
//	"net/url"
//	"strconv"
//	"strings"
//	"time"
//)
//
//const SpotBaseUrl = "https://sapi.xt.com"
//const SpotWsUrl = "wss://stream.xt.com/public"
//const PerpBaseUrl = "https://fapi.xt.com"
//const PerpWsUrl = "wss://fstream.xt.com/ws/market"
//
//var (
//	channelMaintainingXT = false
//)
//
//func getMarketsXT(key, secret string) (marketInfos map[string]*model.MarketInfo) {
//	marketInfos = make(map[string]*model.MarketInfo)
//	getMarketsXtSpot(marketInfos)
//	getMarketsXtPerp(marketInfos)
//	return marketInfos
//}
//
//func getMarketsXtPerp(marketInfos map[string]*model.MarketInfo) {
//	perpHttpResp, perpHttpErr := util.HttpRequest(http.MethodGet, PerpBaseUrl+"/future/market/v1/public/symbol/list", "", map[string]string{}, 30)
//	perpResp := &dtos.PerpMarketResp{}
//	perpJsonErr := json.Unmarshal(perpHttpResp, perpResp)
//	if perpResp == nil || perpResp.ReturnCode != 0 {
//		util.Notice(fmt.Sprintf("get xt perp market error, resp: %s, httpErr: %#v, jsonErr: %#v", perpHttpResp, perpHttpErr, perpJsonErr))
//		return
//	}
//	for _, perpInfo := range perpResp.Result {
//		if !perpInfo.IsOpenApi {
//			continue
//		}
//		if perpInfo.QuoteCoin != "usdt" || perpInfo.ContractType != "PERPETUAL" || perpInfo.State != 0 ||
//			!perpInfo.TradeSwitch || !strings.Contains(perpInfo.SupportOrderType, "LIMIT") || !strings.Contains(perpInfo.SupportPositionType, "CROSSED") {
//			continue
//		}
//		symbol := perpInfo.BaseCoin + model.GetPerpTail(model.XT)
//		marketInfo := &model.MarketInfo{Name: symbol, CTCurrency: perpInfo.BaseCoin}
//		marketInfo.PriceDecimal = perpInfo.PricePrecision
//		marketInfo.PriceIncrement = 1 / math.Pow10(marketInfo.PriceDecimal)
//		marketInfo.SizeMin, _ = strconv.ParseFloat(perpInfo.MinQty, 64)
//		marketInfo.SizeIncrement = 1 / math.Pow10(perpInfo.QuantityPrecision)
//		marketInfo.CTValue, _ = strconv.ParseFloat(perpInfo.ContractSize, 64)
//		marketInfo.BuyLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.MultiplierUp, 64)
//		marketInfo.SellLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.MultiplierDown, 64)
//		marketInfos[symbol] = marketInfo
//	}
//}
//
//func getMarketsXtSpot(marketInfos map[string]*model.MarketInfo) {
//	//spotHttpResp, spotHttpErr := util.HttpRequest(http.MethodGet, SpotBaseUrl+"/v4/public/symbol", "", map[string]string{}, 30)
//	spotHttpResp, spotHttpErr := xtSpotClient("", "", "/v4/public/symbol", http.MethodGet, map[string]interface{}{}, false)
//	spotResp := &dtos.SpotMarketResp{}
//	spotJsonErr := json.Unmarshal(spotHttpResp, spotResp)
//	if spotResp == nil || spotResp.Rc != 0 {
//		util.Notice(fmt.Sprintf("get xt spot market error, resp: %s, httpErr: %#v, jsonErr: %#v", spotHttpResp, spotHttpErr, spotJsonErr))
//		return
//	}
//
//A:
//	for _, symbolInfo := range spotResp.Result.Symbols {
//		if symbolInfo.State != "ONLINE" || !symbolInfo.TradingEnabled || !symbolInfo.OpenapiEnabled || symbolInfo.QuoteCurrency != "usdt" {
//			continue
//		}
//		haveLimitOrderType := false
//		for _, orderType := range symbolInfo.OrderTypes {
//			if orderType == "LIMIT" {
//				haveLimitOrderType = true
//				break
//			}
//		}
//		if !haveLimitOrderType {
//			continue
//		}
//
//		marketInfo := &model.MarketInfo{}
//		marketInfo.Name = symbolInfo.Symbol
//		marketInfo.PriceDecimal = symbolInfo.PricePrecision
//		for _, filter := range symbolInfo.Filters {
//			if filter.Filter == "PRICE" {
//				if filter.Max != "" {
//					marketInfo.PriceMax, _ = strconv.ParseFloat(filter.Max, 64)
//				}
//				if filter.TickSize != "" {
//					marketInfo.PriceIncrement, _ = strconv.ParseFloat(filter.TickSize, 64)
//				}
//			} else if filter.Filter == "QUANTITY" {
//				if filter.Min != "" {
//					marketInfo.SizeMin, _ = strconv.ParseFloat(filter.Min, 64)
//				}
//				if filter.Max != "" {
//					marketInfo.SizeMax, _ = strconv.ParseFloat(filter.Max, 64)
//				}
//				if filter.TickSize != "" {
//					marketInfo.SizeIncrement, _ = strconv.ParseFloat(filter.TickSize, 64)
//				}
//			} else if filter.Filter == "QUOTE_QTY FILTER" {
//				if filter.Min != "" {
//					marketInfo.UsdtMin, _ = strconv.ParseFloat(filter.Min, 64)
//				}
//			} else if filter.Filter == "PROTECTION_ONLINE FILTER" {
//				if filter.DurationSeconds != "" {
//					util.Notice(fmt.Sprintf("现货币种: %s，正处于开盘限价阶段，不参与市场交易", symbolInfo.Symbol))
//					continue A
//				}
//			} else if filter.Filter == "PROTECTION_LIMIT FILTER" {
//				if filter.BuyMaxDeviation != "" || filter.SellMaxDeviation != "" {
//					util.Notice(fmt.Sprintf("现货币种: %s，正处于订单限价保护阶段，不参与市场交易", symbolInfo.Symbol))
//					continue A
//				}
//			}
//		}
//		if marketInfo.PriceIncrement == 0 {
//			marketInfo.PriceIncrement = 1 / math.Pow10(marketInfo.PriceDecimal)
//		}
//		if marketInfo.SizeIncrement == 0 {
//			marketInfo.SizeIncrement = 1 / math.Pow10(symbolInfo.QuantityPrecision)
//		}
//		if marketInfo.SizeMin == 0 {
//			marketInfo.SizeMin = marketInfo.SizeIncrement
//		}
//		marketInfos[marketInfo.Name] = marketInfo
//	}
//}
//
//func maintainChannelXT() {
//	if !channelMaintainingXT {
//		channelMaintainingXT = true
//		go func() {
//			for true {
//				time.Sleep(time.Second * 10)
//				if err := sendToAllConnections(model.XT, []byte(`ping`)); err != nil {
//					util.SocketInfo("xt channel ping error " + err.Error())
//				}
//			}
//		}()
//	}
//}
//
//var subscribeHandlerXT = func(connection *websocket.conn, subscribes []interface{}, keyChannel string) error {
//	var err error = nil
//	var params []string
//	for _, subscribe := range subscribes {
//		params = append(params, subscribe.(string))
//	}
//	subscribeMap := make(map[string]interface{})
//	subscribeMap["id"] = strconv.Itoa(util.GetNow().Nanosecond())
//	subscribeMap["method"] = "subscribe"
//	subscribeMap["params"] = params
//	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
//	if err = sendToConnection(connection, subscribeMessage); err != nil {
//		util.SocketInfo("  can not subscribe %s %s", subscribeMessage, err.Error())
//	}
//	util.Notice(`xt subscribed ` + string(subscribeMessage))
//	time.Sleep(200 * time.Millisecond)
//	return err
//}
//
//func WsDepthServeXT(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
//	markPriceWsHandler := func(connection *websocket.conn, event []byte, orderHandler OrderHandler) {
//		if len(event) == 4 {
//			return
//		}
//		newJson, wsErr := util.NewJSON(event)
//		if wsErr != nil {
//			util.SocketInfo(`xt fail to unmarshal perp mark price ws topic json ` + wsErr.Error())
//			return
//		}
//		if newJson.Get("topic") != nil && newJson.Get("topic").MustString() == "mark_price" {
//			markPriceResp := &dtos.MarkPriceResp{}
//			jsonErr := json.Unmarshal(event, markPriceResp)
//			if jsonErr != nil {
//				util.SocketInfo(`xt fail to unmarshal perp mark ws price data json ` + jsonErr.Error())
//				return
//			}
//			if markPriceResp.Data.S != "" {
//				symbol := markPriceResp.Data.S
//				symbol = symbol[0:len(symbol)-5] + model.GetPerpTail(model.XT)
//				price, _ := strconv.ParseFloat(markPriceResp.Data.P, 64)
//				ts := markPriceResp.Data.T
//				oldMarkPrice := markets.GetMarkPrice(symbol, model.XT)
//				if oldMarkPrice != nil && oldMarkPrice.Ts > ts {
//					return
//				}
//				markPrice := &model.MarkPrice{MarkPrice: price, Ts: ts}
//				markets.SetMarkPrice(symbol, model.XT, markPrice)
//			}
//		}
//	}
//	bookWsHandler := func(connection *websocket.conn, event []byte, orderHandler OrderHandler) {
//		if len(event) == 4 {
//			return
//		}
//		newJson, wsErr := util.NewJSON(event)
//		if wsErr != nil {
//			util.SocketInfo(`xt fail to unmarshal spot book ws topic json ` + wsErr.Error())
//			return
//		}
//		if newJson.Get("topic") != nil && newJson.Get("topic").MustString() == "depth" {
//			bookWsResp := &dtos.BookWsResp{}
//			jsonErr := json.Unmarshal(event, bookWsResp)
//			if jsonErr != nil {
//				util.SocketInfo(`xt fail to unmarshal spot book ws data json ` + jsonErr.Error())
//				return
//			}
//			if bookWsResp.Data.S != "" {
//				var symbol string
//				bidAsk := model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
//				if bookWsResp.Data.Id == "" { //现货
//					symbol = bookWsResp.Data.S
//					bidAsk.Ts = bidAsk.TsReceived
//					//bidAsk.UpdateId = int64(bookWsResp.Data.I)
//					bidAsk.UpdateId = int64(bookWsResp.Data.T)
//				} else {
//					symbol = bookWsResp.Data.S[0:len(bookWsResp.Data.S)-5] + model.GetPerpTail(model.XT)
//					updateId, _ := strconv.ParseInt(bookWsResp.Data.Id, 10, 64)
//					bidAsk.Ts = bidAsk.TsReceived
//					bidAsk.UpdateId = updateId
//				}
//				bids := make([]model.Tick, 0)
//				for _, bid := range bookWsResp.Data.B {
//					price, _ := strconv.ParseFloat(bid[0], 64)
//					amount, _ := strconv.ParseFloat(bid[1], 64)
//					bids = append(bids, model.Tick{Price: price, Amount: amount})
//					break
//				}
//				bidAsk.Bids = bids
//				asks := make([]model.Tick, 0)
//				for _, bid := range bookWsResp.Data.A {
//					price, _ := strconv.ParseFloat(bid[0], 64)
//					amount, _ := strconv.ParseFloat(bid[1], 64)
//					asks = append(asks, model.Tick{Price: price, Amount: amount})
//					break
//				}
//				bidAsk.Asks = asks
//				haveOld, old := markets.GetBidAsk(symbol, model.XT)
//				if haveOld && old.UpdateId > bidAsk.UpdateId {
//					return
//				}
//				if markets.SetBidAsk(symbol, model.XT, &bidAsk) {
//					//util.Info(fmt.Sprintf("perp symbol: %s now bidAsk: %#v", symbol, bidAsk))
//					for function, handler := range model.GetFunctions(model.XT, symbol) {
//						if handler != nil {
//							settings := model.GetSetting(function, model.XT, symbol)
//							for _, setting := range settings {
//								go handler(setting, &bidAsk)
//							}
//						}
//					}
//				}
//			}
//		}
//	}
//	//spotBookUpdateWsHandler := func(connection *websocket.conn, event []byte, orderHandler OrderHandler) {
//	//
//	//}
//	channels = make([]chan struct{}, 0)
//	subscribes := GetWSSubscribes(model.XT, model.SubscribeDepth)
//
//	spotBookChannels, spotBookErr := WebSocketClient(model.XT, SpotWsUrl, model.SubscribeDepth,
//		subscribes, subscribeHandlerXT, bookWsHandler, orderHandler, 10)
//	if spotBookErr == nil {
//		util.Notice(`finish connect public xt spot book wss `)
//		channels = append(channels, spotBookChannels...)
//	}
//	time.Sleep(time.Second * 5)
//
//	perpBookChannels, perpBookErr := WebSocketClient(model.XT, PerpWsUrl, model.SubscribeDepth,
//		subscribes, subscribeHandlerXT, bookWsHandler, orderHandler, 10)
//	if perpBookErr == nil {
//		util.Notice(`finish connect public xt perp book wss `)
//		channels = append(channels, perpBookChannels...)
//	}
//
//	markPriceChannels, markPriceErr := WebSocketClient(model.XT, PerpWsUrl, model.SubscribeMarkPrice,
//		GetWSSubscribes(model.XT, model.SubscribeMarkPrice), subscribeHandlerXT, markPriceWsHandler, nil, 10)
//	if markPriceErr == nil {
//		util.Notice(`finish connect public xt perp mark price wss `)
//		channels = append(channels, markPriceChannels...)
//	}
//	return channels, nil
//}
//
//func getBalanceXT(key string, secret string) (success bool, balances []*model.Balance) {
//	//auth := dtos.NewAuth(dtos.SignedHttpAPI{Accesskey: key, Secretkey: secret}, "/v4/balances", "GET")
//	//auth.SetUrlencode(true)
//	//headers, headerErr := auth.CreatePayload(map[string]interface{}{})
//	//if headerErr != nil {
//	//	util.SocketInfo(fmt.Sprintf("fail to create xt spot http header, headerErr: %#v", headerErr))
//	//	return
//	//}
//	//httpResp, httpErr := util.HttpRequest(http.MethodGet, SpotBaseUrl+"/v4/balances", "", headers, 30)
//	httpResp, httpErr := xtSpotClient(key, secret, "/v4/balances", http.MethodGet, map[string]interface{}{}, true)
//	spotAccountResp := &dtos.BalanceResp{}
//	jsonErr := json.Unmarshal(httpResp, spotAccountResp)
//	if spotAccountResp == nil || spotAccountResp.Rc != 0 {
//		util.SocketInfo(fmt.Sprintf("fail to refresh spot balance xt, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
//		time.Sleep(time.Second * 2)
//		return getBalanceXT(key, secret)
//	}
//
//	balances = make([]*model.Balance, 0)
//	for _, account := range spotAccountResp.Result.Assets {
//		balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.XT, Coin: account.Currency}
//		balance.FrozenAmount, _ = strconv.ParseFloat(account.FrozenAmount, 64)
//		balance.Available, _ = strconv.ParseFloat(account.AvailableAmount, 64)
//		balance.AvailableWithBorrow, _ = strconv.ParseFloat(account.AvailableAmount, 64)
//		balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
//		usdValue, _ := strconv.ParseFloat(account.ConvertUsdtAmount, 64)
//		balance.UsdValue += usdValue
//		balances = append(balances, balance)
//	}
//	return true, balances
//}
//
//func getPositionsXT(key, secret string) (success bool, positions []*model.Position, posBalance float64) {
//	auth := dtos.NewAuthFuture(dtos.SignedFutureHttpAPI{Accesskey: key, Secretkey: secret}, "/future/user/v1/balance/list", "GET")
//	auth.SetUrlencode(true)
//	headers, headerErr := auth.CreatePayloadFuture(map[string]interface{}{})
//	if headerErr != nil {
//		util.SocketInfo(fmt.Sprintf("fail to create xt contract asset http header, headerErr: %#v", headerErr))
//		return
//	}
//	assetHttpResp, assetHttpErr := util.HttpRequest(http.MethodGet, PerpBaseUrl+"/future/user/v1/balance/list", "", headers, 30)
//	xtContractAssetResp := &dtos.XtContractAssetResp{}
//	jsonErr := json.Unmarshal(assetHttpResp, xtContractAssetResp)
//	if xtContractAssetResp == nil || xtContractAssetResp.ReturnCode != 0 {
//		util.SocketInfo(fmt.Sprintf("fail to refresh contract asset xt, resp: %s httpErr: %#v, jsonErr: %#v", assetHttpResp, assetHttpErr, jsonErr))
//		time.Sleep(time.Second * 2)
//		return getPositionsXT(key, secret)
//	}
//
//	positionAuth := dtos.NewAuthFuture(dtos.SignedFutureHttpAPI{Accesskey: key, Secretkey: secret}, "/future/user/v1/position/list", "GET")
//	positionAuth.SetUrlencode(true)
//	positionHeaders, positionHeaderErr := positionAuth.CreatePayloadFuture(map[string]interface{}{})
//	if positionHeaderErr != nil {
//		util.SocketInfo(fmt.Sprintf("fail to create xt contract position http header, headerErr: %#v", positionHeaderErr))
//		return
//	}
//	positionHttpResp, positionHttpErr := util.HttpRequest(http.MethodGet, PerpBaseUrl+"/future/user/v1/position/list", "", positionHeaders, 30)
//	xtContractPositionResp := &dtos.XtContractPositionResp{}
//	positionJsonErr := json.Unmarshal(positionHttpResp, xtContractPositionResp)
//	if xtContractPositionResp == nil || xtContractPositionResp.ReturnCode != 0 {
//		util.SocketInfo(fmt.Sprintf("fail to refresh contract position xt, resp: %s httpErr: %#v, jsonErr: %#v", positionHttpResp, positionHttpErr, positionJsonErr))
//		time.Sleep(time.Second * 2)
//		return getPositionsXT(key, secret)
//	}
//
//	for _, asset := range xtContractAssetResp.Result {
//		if asset.Coin == `usdt` {
//			assetBalance, _ := strconv.ParseFloat(asset.AvailableBalance, 64)
//			posBalance += assetBalance
//		}
//	}
//
//	positions = make([]*model.Position, 0)
//	for _, contract := range xtContractPositionResp.Result {
//		currency := contract.Symbol[0:len(contract.Symbol)-5] + model.GetPerpTail(model.XT)
//		position := &model.Position{Market: model.XT, Ts: util.GetNowUnixMillion(), Currency: currency}
//		position.Direction = contract.PositionSide
//		_, freeAmount := model.ParseRealAmount(model.XT, currency, float64(contract.AvailableCloseSize))
//		_, frozenAmount := model.ParseRealAmount(model.XT, currency, float64(contract.PositionSize-contract.AvailableCloseSize))
//		if position.Direction == "LONG" {
//			position.Frozen = frozenAmount
//			position.Free = freeAmount + position.Frozen
//		} else {
//			position.Frozen = -1 * frozenAmount
//			position.Free = -1*freeAmount + position.Frozen
//		}
//		position.LeverRate = int64(contract.Leverage)
//		position.EntryPrice = contract.EntryPrice
//		position.Margin = contract.OpenOrderMarginFrozen
//		positions = append(positions, position)
//	}
//	return true, positions, posBalance
//}
//
//func placeOrderXT(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
//	if strings.Contains(symbol, model.GetPerpTail(model.XT)) {
//		tradeSymbol := symbol[0:len(symbol)-5] + model.GetSpotTail(model.XT)
//		var tradeOrderSide, tradePositionSide, tradeOrderType string
//		if orderType == model.OrderTypeLimit {
//			tradeOrderType = `LIMIT`
//		} else if orderType == model.OrderTypeMarket {
//			tradeOrderType = `MARKET`
//		}
//		if orderSide == model.OrderSideBuy {
//			tradeOrderSide = "BUY"
//			tradePositionSide = "LONG"
//		} else {
//			tradeOrderSide = "SELL"
//			tradePositionSide = "SHORT"
//		}
//		priceFuture, decimal := model.FormatPrice(model.XT, symbol, orderSide, price)
//		priceStr := util.CutTailZero(strconv.FormatFloat(priceFuture, 'f', decimal, 64))
//		marketInfo := model.GetMarketInfo(model.XT, symbol)
//		if marketInfo == nil {
//			util.Notice(fmt.Sprintf(`[xtPlaceOrder] market info is nil for symbol %s`, symbol))
//			return
//		}
//		formattedAmount := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.XT, symbol, amount)))
//		//orderAmountReal, _ := strconv.ParseFloat(formattedAmount, 64)
//		body := map[string]interface{}{
//			"symbol":       tradeSymbol,
//			"orderSide":    tradeOrderSide,
//			"orderType":    tradeOrderType,
//			"origQty":      formattedAmount,
//			"price":        priceStr,
//			"positionSide": tradePositionSide,
//		}
//		param := url.Values{}
//		param.Set("symbol", tradeSymbol)
//		param.Set("orderSide", tradeOrderSide)
//		param.Set("orderType", tradeOrderType)
//		param.Set("origQty", formattedAmount)
//		param.Set("price", priceStr)
//		param.Set("positionSide", tradePositionSide)
//		auth := dtos.NewAuthFuture(dtos.SignedFutureHttpAPI{Accesskey: key, Secretkey: secret}, "/future/trade/v1/order/create", "POST")
//		auth.SetUrlencode(true)
//		headers, headerErr := auth.CreatePayloadFuture(body)
//		if headerErr != nil {
//			util.SocketInfo(fmt.Sprintf("fail to create xt contract order http header, headerErr: %#v", headerErr))
//			return
//		}
//		httpResp, httpErr := util.HttpRequest(http.MethodPost, PerpBaseUrl+"/future/trade/v1/order/create", param.Encode(), headers, 30)
//		xtContractOrderResp := &dtos.XtContractOrderResp{}
//		jsonErr := json.Unmarshal(httpResp, xtContractOrderResp)
//		if xtContractOrderResp == nil || xtContractOrderResp.ReturnCode != 0 {
//			util.Notice(fmt.Sprintf("fail to create xt future order, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
//		} else {
//			order.Status = model.CarryStatusWorking
//			//order.OrderId = strconv.FormatInt(xtContractOrderResp.Result, 10)
//		}
//	} else {
//		var tradeOrderSide, tradeOrderType string
//		if orderType == model.OrderTypeLimit {
//			tradeOrderType = `LIMIT`
//		} else if orderType == model.OrderTypeMarket {
//			tradeOrderType = `MARKET`
//		}
//		if orderSide == model.OrderSideBuy {
//			tradeOrderSide = "BUY"
//		} else {
//			tradeOrderSide = "SELL"
//		}
//		priceSpot, decimalSpot := model.FormatPrice(model.XT, symbol, orderSide, price)
//		amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.XT, symbol, amount)))
//		priceStr := util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
//		body := map[string]interface{}{
//			"symbol":      symbol,
//			"side":        tradeOrderSide,
//			"type":        tradeOrderType,
//			"timeInForce": "GTC",
//			"bizType":     "SPOT",
//			"quantity":    amountStr,
//			"price":       priceStr,
//		}
//		//auth := dtos.NewAuth(dtos.SignedHttpAPI{Accesskey: key, Secretkey: secret}, "/v4/order", "POST")
//		//headers, headerErr := auth.CreatePayload(body)
//		//if headerErr != nil {
//		//	util.SocketInfo(fmt.Sprintf("fail to create xt spot order header, headerErr: %#v", headerErr))
//		//	return
//		//}
//		//httpResp, httpErr := util.HttpRequest(http.MethodPost, SpotBaseUrl+"/v4/order", string(util.JsonEncodeToByte(body)), headers, 30)
//		httpResp, httpErr := xtSpotClient(key, secret, "/v4/order", http.MethodPost, body, true)
//		xtSpotOrderResp := &dtos.XtSpotOrderResp{}
//		jsonErr := json.Unmarshal(httpResp, xtSpotOrderResp)
//		if xtSpotOrderResp == nil || xtSpotOrderResp.Rc != 0 {
//			util.Notice(fmt.Sprintf("fail to create xt spot order resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
//		} else {
//			order.Status = model.CarryStatusWorking
//			order.OrderId = xtSpotOrderResp.Result.OrderId
//		}
//	}
//}
//
//func cancelOrdersXT(key, secret, symbol string) (result bool) {
//	if strings.Contains(symbol, model.GetPerpTail(model.XT)) {
//		postData := &url.Values{}
//		postData.Set("symbol", symbol[0:len(symbol)-5]+model.GetSpotTail(model.XT))
//		body := map[string]interface{}{"symbol": symbol[0:len(symbol)-5] + model.GetSpotTail(model.XT)}
//		auth := dtos.NewAuthFuture(dtos.SignedFutureHttpAPI{Accesskey: key, Secretkey: secret}, "/future/trade/v1/order/cancel-all", "POST")
//		auth.SetUrlencode(true)
//		headers, headerErr := auth.CreatePayloadFuture(body)
//		if headerErr != nil {
//			util.SocketInfo(fmt.Sprintf("fail to create xt contract cancel order header, headerErr: %#v", headerErr))
//			return
//		}
//		httpResp, httpErr := util.HttpRequest(http.MethodPost, PerpBaseUrl+"/future/trade/v1/order/cancel-all", postData.Encode(), headers, 30)
//		xtContractCommonResp := &dtos.XtContractCommonResp{}
//		jsonErr := json.Unmarshal(httpResp, xtContractCommonResp)
//		if xtContractCommonResp == nil || xtContractCommonResp.ReturnCode != 0 {
//			util.Notice(fmt.Sprintf("fail to cancel xt future order, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
//			return false
//		} else {
//			return true
//		}
//	} else {
//		body := map[string]interface{}{"symbol": symbol}
//		//auth := dtos.NewAuth(dtos.SignedHttpAPI{Accesskey: key, Secretkey: secret}, "/v4/open-order", "DELETE")
//		//headers, headerErr := auth.CreatePayload(body)
//		//if headerErr != nil {
//		//	util.SocketInfo(fmt.Sprintf("fail to create xt spot cancel order header, headerErr: %#v", headerErr))
//		//	return
//		//}
//		//httpResp, httpErr := util.HttpRequest(http.MethodDelete, SpotBaseUrl+"/v4/open-order", string(util.JsonEncodeToByte(body)), headers, 30)
//		httpResp, httpErr := xtSpotClient(key, secret, "/v4/open-order", http.MethodDelete, body, true)
//		xtCancelOrderResp := &dtos.XtCancelOrderResp{}
//		jsonErr := json.Unmarshal(httpResp, xtCancelOrderResp)
//		if xtCancelOrderResp == nil || xtCancelOrderResp.Rc != 0 {
//			util.Notice(fmt.Sprintf("fail to cancel xt spot order, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
//			return false
//		} else {
//			return true
//		}
//	}
//}
//
//func getFundingRateXT(key, secret, symbol string) (fundingRate *model.FundingRate) {
//	if key == `` || secret == `` {
//		keys, secrets := model.AppConfig.GetKeys(model.XT)
//		key = keys[0]
//		secret = secrets[0]
//	}
//	realSymbol := symbol[0:len(symbol)-5] + model.GetSpotTail(model.XT)
//	httpResp, httpErr := util.HttpRequest(http.MethodGet, PerpBaseUrl+"/future/market/v1/public/q/funding-rate?symbol="+realSymbol, "", map[string]string{}, 30)
//	xtFundingResp := &dtos.XtFundingResp{}
//	jsonErr := json.Unmarshal(httpResp, xtFundingResp)
//	if xtFundingResp == nil || xtFundingResp.ReturnCode != 0 {
//		util.Notice(fmt.Sprintf("fail to get xt future funding, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
//	} else {
//		rate, _ := strconv.ParseFloat(xtFundingResp.Result.FundingRate, 64)
//		fundingRate = &model.FundingRate{
//			Rate:       rate,
//			UpdateTime: util.GetNow().Unix(),
//			ExpireTime: xtFundingResp.Result.NextCollectionTime / 1000,
//		}
//	}
//	return
//}
//
//func transferXT(transferType string, amount float64) {
//	postData := &url.Values{}
//	postData.Set(`type`, transferType)
//	postData.Set(`asset`, `USDT`)
//	postData.Set(`amount`, strconv.FormatFloat(amount, 'f', 0, 64))
//}
//
//func openContractAccountXT(key, secret string) {
//	auth := dtos.NewAuthFuture(dtos.SignedFutureHttpAPI{Accesskey: key, Secretkey: secret}, "/future/user/v1/account/open", "POST")
//	auth.SetUrlencode(true)
//	headers, headerErr := auth.CreatePayloadFuture(map[string]interface{}{})
//	if headerErr != nil {
//		util.SocketInfo(fmt.Sprintf("fail to open xt contract account header, headerErr: %#v", headerErr))
//		return
//	}
//	httpResp, httpErr := util.HttpRequest(http.MethodPost, PerpBaseUrl+"/future/user/v1/account/open", "", headers, 30)
//	xtContractCommonResp := &dtos.XtContractCommonResp{}
//	jsonErr := json.Unmarshal(httpResp, xtContractCommonResp)
//	if xtContractCommonResp == nil || xtContractCommonResp.ReturnCode != 0 {
//		util.Notice(fmt.Sprintf("fail to open xt contract account, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
//	}
//}
//
//func xtSpotClient(key, secret, path, method string, body map[string]interface{}, needSign bool) ([]byte, error) {
//	headers := map[string]string{}
//	if needSign {
//		if key == `` || secret == `` {
//			keys, secrets := model.AppConfig.GetKeys(model.XT)
//			key = keys[0]
//			secret = secrets[0]
//		}
//		auth := dtos.NewAuth(dtos.SignedHttpAPI{Accesskey: key, Secretkey: secret}, path, method)
//		if method == http.MethodGet {
//			auth.SetUrlencode(true)
//		}
//		signHeaders, headerErr := auth.CreatePayload(body)
//		if headerErr != nil {
//			util.SocketInfo(fmt.Sprintf("fail to create xt spot header, headerErr: %#v", headerErr))
//			return []byte(""), headerErr
//		}
//		headers = signHeaders
//	}
//
//	if method == http.MethodGet {
//		return util.HttpRequest(method, SpotBaseUrl+path, "", headers, 30)
//	} else {
//		return util.HttpRequest(method, SpotBaseUrl+path, string(util.JsonEncodeToByte(body)), headers, 30)
//	}
//}

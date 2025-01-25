package deprecated

//
//import (
//	"crypto/hmac"
//	"crypto/sha256"
//	"encoding/base64"
//	"encoding/json"
//	"fmt"
//	"hello/api"
//	"hello/api/dtos"
//	"hello/model"
//	"hello/util"
//	"math"
//	"net/http"
//	"strconv"
//	"strings"
//	"time"
//)
//
//const bitgetRestUrl = "https://api.bitget.com"
//const bitgetPublic = "wss://ws.bitget.com/v2/ws/public"
//const bitgetPrivate = `wss://ws.bitget.com/v2/ws/private`
//
//func maintainConnsBitget(market string, accounts []*model.Account) {
//	for _, account := range accounts {
//		model.AppEnvironment.PriConnecting.Store(market+account.Key, false)
//	}
//	for {
//		publicConnKey := api.GetPublicConnKey(market, ``)
//		connTick, _ := model.AppEnvironment.ConnTick.Load(publicConnKey)
//		if connTick != nil {
//			if err := api.SendToConnections(market, connTick.(map[*model.WSConn]bool), []byte(`ping`)); err != nil {
//				util.Log(util.LogLevelError, fmt.Sprintf("tick conn maintain error %s %s", market, err.Error()))
//			}
//		}
//		for _, account := range accounts {
//			success := false
//			connKey := api.getPrivateConnKey(market, account.Key, ``)
//			valueUpdate, _ := model.AppEnvironment.ConnOrder.Load(connKey)
//			if valueUpdate != nil {
//				if err := valueUpdate.(*model.WSConn).WriteMsg([]byte(`ping`)); err != nil {
//					//valueUpdate.(*model.WSConn).Close()
//					model.AppEnvironment.ConnOrder.Delete(connKey)
//					util.Log(util.LogLevelError, fmt.Sprintf("order update conn maintain error %s %s", market, err.Error()))
//				} else {
//					success = true
//				}
//			}
//			if !success {
//				WsOrderServeBitget(market, account)
//			}
//		}
//		time.Sleep(time.Second * 20)
//	}
//}
//
//var wsOrderConnHandlerBitget = func(market, key string, event []byte) {
//	resJson, _ := util.NewJSON(event)
//	if resJson == nil {
//		return
//	}
//	instType := ``
//	if market == model.BitgetSpot {
//		instType = `SPOT`
//	} else if market == model.BitgetPerp {
//		instType = `USDT-FUTURES`
//	}
//	respEvent := resJson.Get(`event`).MustString()
//	code := resJson.Get(`code`).MustInt()
//	if respEvent == `login` && code == 0 {
//		connKey := api.getPrivateConnKey(market, key, ``)
//		value, _ := model.AppEnvironment.ConnOrder.Load(connKey)
//		if value == nil {
//			util.Log(util.LogLevelError, fmt.Sprintf("ignore msg %s %s", market, string(event)))
//			return
//		}
//		subStr := fmt.Sprintf(`{"op":"subscribe","args":[{"instType": "%s","channel":"orders","instId":"default"}]}`, instType)
//		//登录成功后新增账户订阅
//		if market == model.BitgetPerp {
//			subStr = fmt.Sprintf(`{"op":"subscribe","args":[{"instType": "%s","channel":"orders","instId":"default"},{"instType":"%s","channel":"account","coin":"default"},{"instType":"%s","channel":"positions","instId":"default"}]}`, instType, instType, instType)
//		}
//		if err := value.(*model.WSConn).WriteMsg([]byte(subStr)); err != nil {
//			model.AppEnvironment.ConnOrder.Delete(connKey)
//		}
//	} else if respEvent == `trade` && code == 0 {
//	}
//	//判断事件是snapshot
//	if resJson.Get(`action`).MustString() == `snapshot` {
//		if resJson.GetPath(`arg`, `channel`).MustString() == `orders` {
//			dataArray := resJson.Get(`data`).MustArray()
//			for _, data := range dataArray {
//				orderId := data.(map[string]interface{})[`orderId`].(string)
//				dealAmount, _ := strconv.ParseFloat(data.(map[string]interface{})[`accBaseVolume`].(string), 64)
//				status := model.CarryStatusWorking
//				if data.(map[string]interface{})[`status`].(string) == `filled` {
//					status = model.CarryStatusSuccess
//				}
//				api.UpdateOrderDeal(market, orderId, status, string(event), dealAmount)
//			}
//		} else if resJson.GetPath(`arg`, `channel`).MustString() == `account` {
//			dataArray := resJson.Get(`data`).MustArray()
//			collateral := &model.Collateral{AccountKey: key}
//			collateral.Available, _ = strconv.ParseFloat(dataArray[0].(map[string]interface{})[`maxTransferOut`].(string), 64)
//			//util.Log(util.LogLevelInfo, fmt.Sprintf("bitget unified %s %f", collateral.AccountKey, collateral.Available))
//			model.CollateralHandler(key, false, collateral)
//		} else if resJson.GetPath(`arg`, `channel`).MustString() == `positions` {
//			dataArray := resJson.Get(`data`).MustArray()
//			//util.Log(util.LogLevelInfo, fmt.Sprintf("bitget positions num %d", len(dataArray)))
//			if dataArray != nil && len(dataArray) >= model.BitgetPosLimit {
//				model.CollateralHandler(key, true, nil)
//			}
//		}
//	}
//}
//
//func wsLoginBitget(account *model.Account, conn *model.WSConn) (success bool) {
//	ts := time.Now().Unix()
//	hash := hmac.New(sha256.New, []byte(account.Secret))
//	hash.Write([]byte(fmt.Sprintf(`%dGET/user/verify`, ts)))
//	sign := base64.StdEncoding.EncodeToString(hash.Sum(nil))
//	if err := conn.WriteMsg([]byte(fmt.Sprintf(`{"op":"login","args":[{"apiKey":"%s","passphrase":"%s","timestamp":"%d","sign":"%s"}]}`,
//		account.Key, model.AppConfig.Phase, ts, sign))); err != nil {
//		return false
//	}
//	return true
//}
//
//func WsOrderServeBitget(market string, account *model.Account) {
//	if account == nil {
//		return
//	}
//	replaced := model.AppEnvironment.PriConnecting.CompareAndSwap(market+account.Key, false, true)
//	if !replaced {
//		return
//	}
//	defer func() {
//		select {
//		case <-time.After(time.Second * 30):
//		}
//		model.AppEnvironment.PriConnecting.Store(market+account.Key, false)
//	}()
//	connKey := api.getPrivateConnKey(market, account.Key, ``)
//	conn, err := model.WsPrivateClient(account, &model.AppEnvironment.ConnOrder, connKey, market, bitgetPrivate, wsOrderConnHandlerBitget)
//	if err != nil {
//		util.Log(util.LogLevelError, "can not create web socket "+err.Error())
//	} else if conn != nil {
//		if wsLoginBitget(account, conn) {
//			model.AppEnvironment.ConnOrder.Store(connKey, conn)
//		}
//	}
//}
//
//func getMarketsBitgetSpot() (marketInfos map[string]*model.MarketInfo) {
//	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/v2/spot/public/symbols", "", map[string]string{}, 30)
//	spotResp := &dtos.BitgetSpotMarketResp{}
//	spotJsonErr := json.Unmarshal(httpResp, spotResp)
//	if spotResp == nil || spotResp.Code != "00000" {
//		util.Log(util.LogLevelError, fmt.Sprintf(fmt.Sprintf(
//			"get bitget spot market error, resp: %s, httpErr: %#v, jsonErr: %#v", httpResp, httpErr, spotJsonErr)))
//		return
//	}
//	marketInfos = make(map[string]*model.MarketInfo)
//	for _, symbolInfo := range spotResp.Data {
//		if symbolInfo.Status != "online" || symbolInfo.QuoteCoin != "USDT" {
//			continue
//		}
//		tail := model.UniStandardTail[model.MarketTypeSpot]
//		symbol := symbolInfo.BaseCoin + tail
//		marketInfo := &model.MarketInfo{Symbol: symbol, Market: model.BitgetSpot, CTCurrency: strings.ToUpper(symbolInfo.BaseCoin), FundingRateInterval: 8 * 3600000}
//		marketInfo.PriceDecimal, _ = strconv.Atoi(symbolInfo.PricePrecision)
//		marketInfo.PriceIncrement = 1 / math.Pow10(marketInfo.PriceDecimal)
//		amountPrecision, _ := strconv.Atoi(symbolInfo.QuantityPrecision)
//		marketInfo.SizeIncrement = 1 / math.Pow10(amountPrecision)
//		marketInfo.SizeMin, _ = strconv.ParseFloat(symbolInfo.MinTradeAmount, 64)
//		if marketInfo.SizeMin == 0 {
//			marketInfo.SizeMin = marketInfo.SizeIncrement
//		}
//		marketInfo.SizeMax, _ = strconv.ParseFloat(symbolInfo.MaxTradeAmount, 64)
//		if tail == `_USDT` {
//			marketInfo.MoneyMin, _ = strconv.ParseFloat(symbolInfo.MinTradeUSDT, 64)
//		}
//		//marketInfo.BuyLimitPriceRatio, _ = strconv.ParseFloat(symbolInfo.BuyLimitPriceRatio, 64)
//		//marketInfo.SellLimitPriceRatio, _ = strconv.ParseFloat(symbolInfo.SellLimitPriceRatio, 64)
//		marketInfos[marketInfo.Symbol] = marketInfo
//	}
//	return marketInfos
//}
//
//func parseBidAskBitget(bookWsResp *dtos.BitgetBoosWsResp) (bidAsk *model.BidAsk) {
//	if bookWsResp == nil {
//		return nil
//	}
//	market := model.BitgetSpot
//	marketType := model.MarketTypeSpot
//	if bookWsResp.Arg.InstType == `SPOT` {
//		market = model.BitgetSpot
//		marketType = model.MarketTypeSpot
//	} else if bookWsResp.Arg.InstType == `USDT-FUTURES` {
//		market = model.BitgetPerp
//		marketType = model.MarketTypePerp
//	} else {
//		return nil
//	}
//	success, _, symbol := model.GetFromDialect(market, marketType, bookWsResp.Arg.InstId)
//	if !success {
//		return nil
//	}
//	switch bookWsResp.Action {
//	case `snapshot`:
//		bidAsk = &model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
//		if len(bookWsResp.Data) > 1 ||
//			len(bookWsResp.Data[0].Bids) < 1 || len(bookWsResp.Data[0].Bids[0]) < 2 ||
//			len(bookWsResp.Data[0].Asks) < 1 || len(bookWsResp.Data[0].Asks[0]) < 2 {
//			return nil
//		}
//		bidPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][0], 64)
//		bidAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][1], 64)
//		bids := make([]model.Tick, 0)
//		bids = append(bids, model.Tick{Price: bidPrice, Amount: bidAmount, Market: market, Symbol: symbol})
//		bidAsk.Bids = bids
//		askPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][0], 64)
//		askAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][1], 64)
//		asks := make([]model.Tick, 0)
//		asks = append(asks, model.Tick{Price: askPrice, Amount: askAmount, Market: market, Symbol: symbol})
//		bidAsk.Asks = asks
//		bidAsk.Ts, _ = strconv.Atoi(bookWsResp.Data[0].Ts)
//		bidAsk.UpdateId, _ = strconv.ParseInt(bookWsResp.Data[0].Ts, 10, 64)
//	case `update`:
//
//	}
//	return bidAsk
//}
//
//var tickHandlerBitget = func(market string, conn *model.WSConn, event []byte) {
//	if strings.Contains(string(event), `ping`) {
//		err := conn.WriteMsg([]byte(`pong`))
//		if err != nil {
//			return
//		}
//		return
//	}
//	bookWsResp := &dtos.BitgetBoosWsResp{}
//	jsonErr := json.Unmarshal(event, bookWsResp)
//	if jsonErr != nil {
//		//util.SocketInfo(`bitget fail to unmarshal book ws data json ` + jsonErr.Error())
//		return
//	}
//	bidAsk := parseBidAskBitget(bookWsResp)
//	if bidAsk == nil || bidAsk.Bids.Len() == 0 {
//		return
//	}
//	symbol := bidAsk.Bids[0].Symbol
//	if model.AppEnvironment.SetBidAsk(market, symbol, bidAsk) {
//		funcHandlers := api.GetFunctions(market, symbol)
//		if funcHandlers != nil {
//			funcHandlers.Range(func(function, value interface{}) bool {
//				setting := api.GetSetting(function.(string), market, symbol)
//				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
//					go value.(model.CarryHandler)(setting, bidAsk)
//				}
//				return true
//			})
//		}
//	}
//}
//
//var subscribeHandlerBitget = func(market string, connection *model.WSConn, subscribes []interface{}) error {
//	var err error = nil
//	var params []string
//	for _, subscribe := range subscribes {
//		params = append(params, subscribe.(string))
//	}
//	subscribeMap := make(map[string]interface{})
//	subscribeMap["op"] = "subscribe"
//	subscribeMap["args"] = params
//	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
//	if err = connection.WriteMsg(subscribeMessage); err != nil {
//		util.Log(util.LogLevelInfo, fmt.Sprintf("%s can not subscribe %s %s", market, subscribeMessage, err.Error()))
//	}
//	return err
//}
//
//func getBalanceBitgetSpot(key string, secret string) (success bool, balances []*model.Balance) {
//	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
//	httpResp, httpErr := client.DoGet("/api/v2/spot/account/assets", map[string]string{})
//	bitgetBalanceResp := &dtos.BitgetBalanceResp{}
//	jsonErr := json.Unmarshal(httpResp, bitgetBalanceResp)
//	if bitgetBalanceResp == nil || bitgetBalanceResp.Code != "00000" {
//		util.Log(util.LogLevelError, fmt.Sprintf(
//			"fail to refresh bitgetspot balance, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
//		time.Sleep(time.Second * 2)
//		return getBalanceBitgetSpot(key, secret)
//	}
//	balances = make([]*model.Balance, 0)
//	for _, account := range bitgetBalanceResp.Data {
//		balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.BitgetSpot, Coin: strings.ToUpper(account.CoinName)}
//		balance.FrozenAmount, _ = strconv.ParseFloat(account.Frozen, 64)
//		balance.AvailableWithBorrow, _ = strconv.ParseFloat(account.Available, 64)
//		balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
//		priceGet, price := api.GetPriceForce(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.BitgetSpot)
//		//priceGet, bidAsk := model.AppEnvironment.GetBidAsk(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.BitgetSpot)
//		if priceGet {
//			balance.UsdValue = balance.Amount * price
//		}
//		balances = append(balances, balance)
//	}
//	return true, balances
//}
//
//// GetPriceBitgetSpot
//func _(account *model.Account, symbol string) (price float64) {
//	_, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
//	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: account.Key, ApiSecretKey: account.Secret}
//	resp, _ := client.DoGet("/api/v2/spot/market/tickers", map[string]string{`symbol`: dialectSymbol})
//	if resp == nil {
//		return
//	}
//	result, _ := util.NewJSON(resp)
//	if result == nil {
//		return
//	}
//	array := result.Get(`data`).MustArray()
//	if len(array) == 0 {
//		return 0
//	}
//	bidPr := array[0].(map[string]interface{})[`bidPr`]
//	priceBid, err := strconv.ParseFloat(bidPr.(string), 64)
//	if err != nil {
//		return
//	}
//	return priceBid
//}
//
//func placeOrderBitgetSpot(account *model.Account, isWs bool, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
//	priceSpot, decimalSpot := model.FormatPrice(model.BitgetSpot, symbol, price)
//	priceStr := util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
//	ordType := ``
//	if orderType == model.OrderTypeMarket {
//		ordType = `market`
//	} else if orderType == model.OrderTypeLimit {
//		ordType = `limit`
//	}
//	formattedAmount, format := model.GetAmountInMarket(model.BitgetSpot, symbol, amount, priceSpot, false)
//	amountStr := util.CutTailZero(fmt.Sprintf(format, formattedAmount))
//	if orderSide == model.OrderSideBuy && orderType == model.OrderTypeMarket {
//		amountStr = fmt.Sprintf(`%f`, amount*priceSpot)
//	}
//	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
//	if !success {
//		util.Log(util.LogLevelError, "fail to place spot order, GetFromStandard: "+symbol)
//		return
//	}
//	if isWs {
//		msg := fmt.Sprintf(`{"op":"trade","args":[{"id":"%s","instType":"SPOT","instId":"%s","channel":"place-order","params":{
//			"orderType":"%s","side":"%s","size":"%s","price":"%s","force":"gtc","clientOid":"%s"}}]}`,
//			order.ClientOrdId, dialectSymbol, orderType, orderSide, amountStr, priceStr, order.ClientOrdId)
//		connKey := api.getPrivateConnKey(model.BitgetSpot, account.Key, model.MarketTypeSpot)
//		value, _ := model.AppEnvironment.ConnOrder.Load(connKey)
//		if value == nil {
//			order.Status = model.CarryStatusFail
//		} else {
//			if err := value.(*model.WSConn).WriteMsg([]byte(msg)); err != nil {
//				model.AppEnvironment.ConnOrder.Delete(connKey)
//				order.Status = model.CarryStatusFail
//				util.Log(util.LogLevelError, fmt.Sprintf(`fail to place bitgetSpot order return: %s`, err.Error()))
//			}
//		}
//		if order.Status == model.CarryStatusFail {
//			api.HandleWsOrderConnFail(account, model.BitgetSpot, order)
//		}
//	} else {
//		client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: account.Key, ApiSecretKey: account.Secret}
//		params := map[string]interface{}{
//			"symbol":    dialectSymbol,
//			"force":     "gtc",
//			"size":      amountStr,
//			"price":     priceStr,
//			"side":      orderSide,
//			"orderType": ordType,
//			"clientOid": order.ClientOrdId}
//		strParams := string(util.JsonEncodeToByte(params))
//		util.Log(util.LogLevelInfo, fmt.Sprintf(`place bitgetSpot %s`, strParams))
//		httpResp, httpErr := client.DoPost("/api/v2/spot/trade/place-order", strParams)
//		bitgetOrderResp := &dtos.BitgetOrderResp{}
//		jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
//		if bitgetOrderResp == nil {
//			util.Log(util.LogLevelError, fmt.Sprintf(
//				"fail to create bitget spot order %s resp: %s httpErr: %#v, jsonErr: %#v", strParams, httpResp, httpErr, jsonErr))
//		} else if len(strings.Trim(bitgetOrderResp.Code, `0`)) == 0 {
//			order.Status = model.CarryStatusWorking
//			order.OrderId = bitgetOrderResp.Data.OrderId
//		} else {
//			order.Status = model.CarryStatusFail
//			order.ErrCode = bitgetOrderResp.Code
//			util.Log(util.LogLevelError, fmt.Sprintf(
//				"fail to create bitget spot order resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
//		}
//	}
//}
//
//func batchCancelBitgetSpot(key, secret string, orders []*model.Order) (result bool) {
//	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
//	params := map[string]string{`batchMode`: `multiple`}
//	orderList := make(map[string]string)
//	for _, order := range orders {
//		orderList[`orderId`] = order.OrderId
//		_, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, order.Symbol)
//		orderList[`symbol`] = dialectSymbol
//	}
//	httpResp, httpErr := client.DoPost(`/api/v2/spot/trade/batch-cancel-order`, string(util.JsonEncodeToByte(params)))
//	if httpErr != nil {
//		util.Log(util.LogLevelError, fmt.Sprintf(`batchCancelBitgetSpot fail to post %s`, httpErr.Error()))
//		return false
//	}
//	jsonData, jsonErr := util.NewJSON(httpResp)
//	if jsonErr != nil {
//		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when batchCancelBitgetSpot %s`, jsonErr.Error()))
//		return false
//	}
//	if jsonData != nil {
//		if jsonData.Get("message").MustString() == "success" {
//			util.Log(util.LogLevelInfo, fmt.Sprintf("success to batch cancel bitget spot order: %d",
//				len(jsonData.GetPath(`data`, `successList`).MustArray())))
//			return true
//		} else {
//			util.Log(util.LogLevelError, fmt.Sprintf("fail to batch cancel bitget spot order, code: %s %s",
//				jsonData.Get("code").MustString(), string(httpResp)))
//		}
//	}
//	return false
//}
//
//func cancelOrderBitgetSpot(key, secret, symbol, orderId string) (result bool) {
//	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
//	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
//	if !success {
//		util.Log(util.LogLevelError, "fail to cancel bitget spot order, GetFromStandard: "+symbol)
//		return false
//	}
//	param := map[string]string{`orderId`: orderId, `symbol`: dialectSymbol}
//	httpResp, httpErr := client.DoPost("/api/v2/spot/trade/cancel-order", string(util.JsonEncodeToByte(param)))
//	if httpErr != nil {
//		util.Log(util.LogLevelError, fmt.Sprintf(`fail to post when cancelOrderBitgetSpot %s`, httpErr.Error()))
//		return false
//	}
//	jsonData, jsonErr := util.NewJSON(httpResp)
//	if jsonErr != nil {
//		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when cancelOrderBitgetSpot %s`, jsonErr.Error()))
//		return false
//	}
//	if jsonData != nil {
//		code, _ := jsonData.Get("code").String()
//		if code == "00000" {
//			util.Log(util.LogLevelInfo, fmt.Sprintf("success to cancel bitget spot order: %s", symbol))
//			return true
//		} else {
//			util.Log(util.LogLevelError, fmt.Sprintf("fail to cancel bitget spot order, code: %s %s", code, string(httpResp)))
//		}
//	}
//	return false
//}
//
//func cancelOrdersBitgetSpot(key, secret, symbol string) (result bool) {
//	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
//	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
//	if !success {
//		util.Log(util.LogLevelError, "fail to cancel bitget spot order, GetFromStandard: "+symbol)
//		return false
//	}
//	httpResp, httpErr := client.DoPost("/api/v2/spot/trade/cancel-symbol-order", string(util.JsonEncodeToByte(map[string]interface{}{"symbol": dialectSymbol})))
//	if httpErr != nil {
//		util.Log(util.LogLevelError, fmt.Sprintf(`fail to post when cancelOrdersBitgetSpot %s`, httpErr.Error()))
//		return false
//	}
//	jsonData, jsonErr := util.NewJSON(httpResp)
//	if jsonErr != nil {
//		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when cancelOrdersBitgetSpot %s`, jsonErr.Error()))
//		return false
//	}
//	if jsonData != nil {
//		code, _ := jsonData.Get("code").String()
//		if code == "00000" {
//			util.Log(util.LogLevelInfo, fmt.Sprintf("success to cancel bitget spot order: %s", symbol))
//			return true
//		} else {
//			util.Log(util.LogLevelError, fmt.Sprintf("fail to cancel bitget spot order, code: %s %s", code, string(httpResp)))
//		}
//	}
//	return false
//}
//
//// parseOrderBitgetSpot 由于不同查询返回格式不同，price dealPrice amount等值再不同方法中分别设置
//func parseOrderBitgetSpot(resp *dtos.BitgetSpotOrderDetailResp) (orders []*model.Order) {
//	if resp == nil || resp.Data == nil || len(resp.Data) == 0 {
//		return nil
//	}
//	orders = make([]*model.Order, 0)
//	for _, orderResp := range resp.Data {
//		_, _, symbol := model.GetFromDialect(model.BitgetSpot, model.MarketTypeSpot, orderResp.Symbol)
//		order := &model.Order{Market: model.BitgetSpot, Status: model.CarryStatusWorking, OrderId: orderResp.OrderId,
//			ClientOrdId: orderResp.ClientOid, Symbol: symbol}
//		intOrderTime, _ := strconv.ParseInt(orderResp.CTime, 10, 64)
//		order.OrderTime = time.UnixMilli(intOrderTime)
//		order.DealAmount, _ = strconv.ParseFloat(orderResp.BaseVolume, 64)
//		order.UnfilledQuantity = order.Amount - order.DealAmount
//		order.OrderSide = orderResp.Side
//		// 需要根据不同的查询api 用不同的返回值设置price dealPrice amount
//		order.Price, _ = strconv.ParseFloat(orderResp.PriceAvg, 64)
//		order.DealPrice, _ = strconv.ParseFloat(orderResp.BasePrice, 64)
//		order.Amount, _ = strconv.ParseFloat(orderResp.Size, 64)
//		if orderResp.OrderType == `limit` {
//			order.OrderType = model.OrderTypeLimit
//		} else if orderResp.OrderType == `market` {
//			order.OrderType = model.OrderTypeMarket
//			if order.Price > 0 {
//				order.Amount /= order.Price
//			}
//		}
//		switch orderResp.Status {
//		case `live`, `partially_filled`:
//			order.Status = model.CarryStatusWorking
//		case `cancelled`:
//			order.Status = model.CarryStatusFail
//		case `filled`:
//			order.Status = model.CarryStatusSuccess
//		}
//		orders = append(orders, order)
//	}
//	return orders
//}
//
//func queryOpenOrdersBitgetSpot(key, secret, symbol string) (orders []*model.Order) {
//	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
//	var param map[string]string
//	if symbol != "" {
//		_, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
//		param = map[string]string{"symbol": dialectSymbol}
//	}
//	httpResp, httpErr := client.DoGet("/api/v2/spot/trade/unfilled-orders", param)
//	if httpErr != nil {
//		util.Log(util.LogLevelError, fmt.Sprintf("fail to queryOpenOrdersBitgetSpot %s %s", symbol, httpErr.Error()))
//		return nil
//	}
//	orderDetailResp := &dtos.BitgetSpotOrderDetailResp{}
//	perpJsonErr := json.Unmarshal(httpResp, orderDetailResp)
//	if orderDetailResp == nil || orderDetailResp.Code != "00000" {
//		util.Log(util.LogLevelError, fmt.Sprintf(
//			"queryOpenOrdersBitgetSpot fail error http %#v json %#v resp %s", httpErr, perpJsonErr, httpResp))
//		return nil
//	}
//	orders = parseOrderBitgetSpot(orderDetailResp)
//	util.Log(util.LogLevelInfo, fmt.Sprintf("queryOpenOrdersBitgetSpot %s %d", symbol, len(orders)))
//	return orders
//}
//
//// queryOrderBitgetSpot 由于price dealPrice amount的计算方法不同，本查询中进行了修订
//func queryOrderBitgetSpot(key, secret, orderId string) (order *model.Order) {
//	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
//	httpResp, httpErr := client.DoGet("/api/v2/spot/trade/orderInfo", map[string]string{`orderId`: orderId})
//	orderDetailResp := &dtos.BitgetSpotOrderDetailResp{}
//	perpJsonErr := json.Unmarshal(httpResp, orderDetailResp)
//	if orderDetailResp == nil || orderDetailResp.Code != "00000" {
//		util.Log(util.LogLevelError, fmt.Sprintf(
//			"get bitget spot order detail error, resp: %s, httpErr: %#v, jsonErr: %#v", httpResp, httpErr, perpJsonErr))
//		return nil
//	}
//	orders := parseOrderBitgetSpot(orderDetailResp)
//	if len(orders) == 0 {
//		return nil
//	}
//	order = orders[0]
//	order.Price, _ = strconv.ParseFloat(orderDetailResp.Data[0].Price, 64)
//	order.DealPrice, _ = strconv.ParseFloat(orderDetailResp.Data[0].PriceAvg, 64)
//	order.Amount, _ = strconv.ParseFloat(orderDetailResp.Data[0].Size, 64)
//	if order.Price == 0 {
//		order.Price = order.DealPrice
//	}
//	if orderDetailResp.Data[0].OrderType == `market` && orderDetailResp.Data[0].Side == `buy` {
//		order.Amount /= order.Price
//	}
//	util.Log(util.LogLevelInfo, fmt.Sprintf(`%s %s query result %s %s %f`,
//		orders[0].Market, orders[0].OrderId, orders[0].Symbol, orders[0].Status, orders[0].DealAmount))
//	return orders[0]
//}

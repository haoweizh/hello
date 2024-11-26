package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const restBinance = `https://api.binance.com`
const restDataBinance = `https://data.binance.com`
const wsBinance = "wss://stream.binance.com:9443"
const wsBinanceSpotApi = `wss://ws-api.binance.com:443/ws-api/v3`
const wsStepBinance = 60

func GetMarketsBinance(account *model.Account, market string) (marketInfos map[string]*model.MarketInfo) {
	util.Notice(fmt.Sprintf("start to GetMarketsBinance %s", account.Key))
	marketInfos = make(map[string]*model.MarketInfo)
	client := binance.NewClient(account.Key, account.Secret)
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	stats, _ := client.NewListPriceChangeStatsService().Do(context.Background())
	if err != nil {
		util.Notice(fmt.Sprintf("GetMarketsBinance %s err: %s %v休息五分钟", account.Key, err.Error(), exchangeInfo))
		time.Sleep(time.Minute * 5)
		return GetMarketsBinance(account, market)
	}
	for _, item := range exchangeInfo.Symbols {
		if item.Status != "TRADING" || item.QuoteAsset != model.DialectTail[model.MarketTypeSpot][market] || !item.IsMarginTradingAllowed {
			continue
		}
		tail := model.UniStandardTail[model.MarketTypeSpot]
		marketInfo := &model.MarketInfo{Market: market, Name: item.BaseAsset + tail}
		for _, data := range item.Filters {
			filterType := data[`filterType`].(string)
			if filterType == `PRICE_FILTER` {
				if data[`tickSize`] != nil {
					marketInfo.PriceIncrement, _ = strconv.ParseFloat(data[`tickSize`].(string), 64)
				}
				marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
			} else if filterType == `LOT_SIZE` {
				if data[`minQty`] != nil {
					marketInfo.SizeMin, _ = strconv.ParseFloat(data[`minQty`].(string), 64)
				}
				if data[`maxQty`] != nil {
					marketInfo.SizeMax, _ = strconv.ParseFloat(data[`maxQty`].(string), 64)
				}
				if data[`stepSize`] != nil {
					marketInfo.SizeIncrement, _ = strconv.ParseFloat(data[`stepSize`].(string), 64)
				}
			} else if filterType == `NOTIONAL` {
				if data[`minNotional`] != nil {
					marketInfo.MoneyMin, _ = strconv.ParseFloat(data[`minNotional`].(string), 64)
				} else if tail == `_USDT` {
					marketInfo.MoneyMin = 10
				}
			}
		}
		marketInfos[marketInfo.Name] = marketInfo
	}
	for _, stat := range stats {
		if stat == nil {
			continue
		}
		success, marketType, coin := model.GetCoinFromDialect(model.BinanceSpot, stat.Symbol)
		if !success {
			continue
		}
		name := coin + model.UniStandardTail[marketType]
		if marketInfos[name] != nil {
			marketInfos[name].TradeAmount, _ = strconv.ParseFloat(stat.QuoteVolume, 64)
		}
	}
	return marketInfos
}

var KLineMsgHandlerBinanceSpot = func(market string, event []byte) {
	result, wsErr := util.NewJSON(event)
	if wsErr != nil {
		util.Notice(`binance fail to unmarshal json ` + wsErr.Error())
		return
	}
	result = result.Get(`data`)
	if result == nil {
		return
	}
	subscribe, _ := result.Get("e").String()
	dialectSymbol := result.Get(`s`).MustString()
	if subscribe != `kline` {
		return
	}
	success, _, coin := model.GetCoinFromDialect(market, dialectSymbol)
	if !success {
		return
	}
	standardSymbol := coin + model.UniStandardTail[model.MarketTypeSpot]
	if market == model.BinanceMargin {
		standardSymbol = coin + model.UniStandardTail[model.MarketTypeSpot]
	}
	createdAt := time.UnixMilli(result.Get(`E`).MustInt64())
	result = result.Get(`k`)
	candle := &model.Candle{Market: market, Symbol: standardSymbol, CreatedAt: createdAt,
		Begin: time.UnixMilli(result.Get(`t`).MustInt64()), Seconds: 60,
		End: time.UnixMilli(result.Get(`T`).MustInt64())}
	candle.PriceOpen, _ = strconv.ParseFloat(result.Get(`o`).MustString(), 64)
	candle.PriceClose, _ = strconv.ParseFloat(result.Get(`c`).MustString(), 64)
	candle.PriceHigh, _ = strconv.ParseFloat(result.Get(`h`).MustString(), 64)
	candle.PriceLow, _ = strconv.ParseFloat(result.Get(`l`).MustString(), 64)
	candle.Volume, _ = strconv.ParseFloat(result.Get(`v`).MustString(), 64)
	candle.VolumeQuote, _ = strconv.ParseFloat(result.Get(`q`).MustString(), 64)
	if model.AppEnvironment.SetCandle(candle.Symbol, candle.Market, candle) {
		for _, handler := range model.CandleHandlers {
			handler(model.AppEnvironment, candle)
		}
	}
}

func WsKLineBinanceSpot(environment *model.Environment, market string, symbols map[string]bool) (
	socketMap map[*websocket.Conn]bool, msgChans []chan struct{}, connectErr error) {

	subs := make([]interface{}, 0)
	for symbol := range symbols {
		_, _, _, dialectSymbol := model.GetFromStandard(market, symbol)
		subs = append(subs, strings.ToLower(dialectSymbol)+`@kline_1m`)
	}
	socketMap, msgChans, connectErr = WebSocketClient(market, wsBinance+`/stream`, subs,
		subscribeHandlerBinance, KLineMsgHandlerBinanceSpot, wsStepBinance)
	environment.MsgChanKLine.Store(market, msgChans)
	return
}

var wsHandlerBinance = func(market string, event []byte) {
	result, wsErr := util.NewJSON(event)
	if wsErr != nil {
		util.Notice(`binance fail to unmarshal json ` + wsErr.Error())
		return
	}
	id := result.Get(`id`).MustInt()
	if id > 0 {
		subIdBinance.Store(id, false)
		return
	}
	subscribe, _ := result.Get("stream").String()
	result = result.Get(`data`)
	//data := new(binance.WsBookTickerEvent)
	//wsErr := json.Unmarshal(event, &data)
	if result == nil {
		return
	}
	dialectSymbol := result.Get(`s`).MustString()
	updateId := result.Get(`u`).MustInt64()
	if strings.Contains(subscribe, `@depth`) {
		updateId = result.Get(`lastUpdateId`).MustInt64()
	}
	success, _, coin := model.GetCoinFromDialect(market, dialectSymbol)
	if !success {
		return
	}
	standardSymbol := coin + model.UniStandardTail[model.MarketTypeSpot]
	if market == model.BinanceMargin {
		standardSymbol = coin + model.UniStandardTail[model.MarketTypeSpot]
	}
	haveOld, old := model.AppEnvironment.GetBidAsk(market, standardSymbol)
	if haveOld && old.UpdateId > updateId {
		return
	}
	if strings.Contains(subscribe, `@depth`) {
		handleDepthBinance(model.AppEnvironment, result, market, standardSymbol, updateId)
	} else if strings.Contains(subscribe, `@bookTicker`) {
		handleTickerBinance(model.AppEnvironment, result, market, standardSymbol, updateId)
	}
}

var subIdBinance sync.Map

var subscribeHandlerBinance = func(market string, connection *websocket.Conn, subscribes []interface{}) (err error) {
	subParam := make(map[string]interface{})
	subParam["method"] = "SUBSCRIBE"
	subParam["params"] = subscribes
	txId := time.Now().UnixMilli()
	subParam["id"] = txId
	subParamJson, _ := json.Marshal(subParam)
	subIdBinance.Store(txId, true)
	for {
		if err = SendToConnection(market, connection, subParamJson); err != nil {
			util.SocketInfo("binance spot can not subscribe %s %s", subParamJson, err.Error())
		}
		util.Notice(`%s send subscribe: %s `, market, subParamJson)
		time.Sleep(time.Millisecond * 300)
		loadIdBool, _ := subIdBinance.Load(txId)
		if loadIdBool.(bool) {
			break
		} else {
			util.Notice(`%s retry subscribe %s `, market)
		}
	}
	return err
}

func handleTickerBinance(environment *model.Environment, json *simplejson.Json, market, standardSymbol string, updateId int64) {
	bidPrice, _ := strconv.ParseFloat(json.Get(`b`).MustString(), 64)
	bidAmount, _ := strconv.ParseFloat(json.Get(`B`).MustString(), 64)
	askPrice, _ := strconv.ParseFloat(json.Get(`a`).MustString(), 64)
	askAmount, _ := strconv.ParseFloat(json.Get(`A`).MustString(), 64)
	ts := json.Get(`E`).MustInt()
	now := int(time.Now().UnixNano() / int64(time.Millisecond))
	if ts == 0 {
		ts = now
	}
	if bidPrice > 0 && bidAmount > 0 && askPrice > 0 && askAmount > 0 {
		bidAsk := model.BidAsk{Ts: ts, TsReceived: now, UpdateId: updateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: market, Symbol: standardSymbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: market, Symbol: standardSymbol}}}
		haveOld, old := environment.GetBidAsk(market, standardSymbol)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
			return
		}
		if environment.SetBidAsk(market, standardSymbol, &bidAsk) {
			funcHandlers := GetFunctions(market, standardSymbol)
			if funcHandlers != nil {
				funcHandlers.Range(func(function, value interface{}) bool {
					setting := GetSetting(function.(string), market, standardSymbol)
					if setting != nil && value != nil && value.(model.CarryHandler) != nil {
						go value.(model.CarryHandler)(setting, &bidAsk)
					}
					return true
				})
			}
		}
	}
}

func handleDepthBinance(environment *model.Environment, json *simplejson.Json, market, standardSymbol string, updateId int64) {
	now := int(util.GetNowUnixMillion())
	bidAsk := model.BidAsk{UpdateId: updateId, Ts: now, TsReceived: now}
	var bids, asks []interface{}
	bidArray, _ := json.Get(`bids`).Array()
	bids = bidArray
	askArray, _ := json.Get(`asks`).Array()
	asks = askArray
	bidAsk.Bids = make([]model.Tick, len(bids))
	for i, value := range bids {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: market, Symbol: standardSymbol}
	}
	bidAsk.Asks = make([]model.Tick, len(asks))
	for i, value := range asks {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: market, Symbol: standardSymbol}
	}
	sort.Sort(bidAsk.Asks)
	sort.Sort(sort.Reverse(bidAsk.Bids))
	if environment.SetBidAsk(market, standardSymbol, &bidAsk) {
		funcHandlers := GetFunctions(market, standardSymbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), market, standardSymbol)
				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
					go value.(model.CarryHandler)(setting, &bidAsk)
				}
				return true
			})
		}
	}
}

func placeOrderBinanceSpot(account *model.Account, isWs bool, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	decimal := 0
	price, decimal = model.FormatPrice(model.BinanceSpot, symbol, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	formattedAmount := model.GetAmountInMarket(model.BinanceSpot, symbol, amount, price, false)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	order.Price = price
	order.TriggerPrice = price
	if !success {
		return
	}
	if isWs {
		if orderSide == model.OrderSideBuy {
			orderSide = string(binance.SideTypeBuy)
		} else if orderSide == model.OrderSideSell {
			orderSide = string(binance.SideTypeSell)
		}
		ts := time.Now().UnixMilli()
		param := url.Values{}
		param.Set("symbol", dialectSymbol)
		param.Set("side", orderSide)
		param.Set("type", strings.ToUpper(orderType))
		param.Set("timeInForce", `GTC`)
		param.Set(`price`, priceStr)
		param.Set(`quantity`, amountStr)
		param.Set(`apiKey`, account.Key)
		param.Set(`timestamp`, fmt.Sprintf(`%d`, ts))
		hash := hmac.New(sha256.New, []byte(account.Secret))
		hash.Write([]byte(param.Encode()))
		msg := fmt.Sprintf(`{"id": "%s","method": "order.place","params":{"symbol": "%s","side": "%s","type": "%s",
			"timeInForce": "GTC","price": "%s","quantity": "%s","apiKey": "%s","signature": "%s","timestamp": %d}}`,
			order.OrderId, dialectSymbol, orderSide, strings.ToUpper(orderType), priceStr, amountStr, account.Key,
			hex.EncodeToString(hash.Sum(nil)), ts)
		value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, model.BinanceSpot, account.Key)
		if value == nil || value.(*model.WSConn).Conn == nil {
			return
		}
		if err := value.(*model.WSConn).Conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			util.Notice(fmt.Sprintf(`fail to place binancespot order return: %s`, err.Error()))
		}
	} else {
		client := binance.NewClient(account.Key, account.Secret)
		service := client.NewCreateOrderService().Symbol(dialectSymbol).Quantity(amountStr)
		if orderSide == model.OrderSideBuy {
			service.Side(binance.SideTypeBuy)
		} else if orderSide == model.OrderSideSell {
			service.Side(binance.SideTypeSell)
		}
		if orderType == model.OrderTypeMarket {
			service.Type(binance.OrderTypeMarket)
		} else if orderType == model.OrderTypeLimit {
			service.Type(binance.OrderTypeLimit)
			service.Price(priceStr)
			service.TimeInForce(binance.TimeInForceTypeGTC)
		}
		orderResponse, err := service.Do(context.Background())
		if err != nil {
			util.Notice(fmt.Sprintf(`placeOrderBinanceSpot err: %s amount %s`, err.Error(), amountStr))
			order.ErrCode = err.Error()
			order.OrderId = ``
		} else {
			order.OrderId = strconv.FormatInt(orderResponse.OrderID, 10)
			order.Amount, _ = strconv.ParseFloat(orderResponse.OrigQuantity, 64)
		}
	}
}

func parseOrderBinanceSpotSdk(orderResp *binance.Order) (order *model.Order) {
	if orderResp == nil {
		return nil
	}
	order = &model.Order{Market: model.BinanceSpot, Status: model.CarryStatusFail}
	order.OrderId = strconv.FormatInt(orderResp.OrderID, 10)
	_, marketType, coin := model.GetCoinFromDialect(model.BinanceSpot, orderResp.Symbol)
	order.Symbol = coin + model.UniStandardTail[marketType]
	order.OrderSide = strings.ToLower(string(orderResp.Side))
	order.OrderType = strings.ToLower(string(orderResp.Type))
	order.Amount, _ = strconv.ParseFloat(orderResp.OrigQuantity, 64)
	order.Price, _ = strconv.ParseFloat(orderResp.Price, 64)
	order.DealAmount, _ = strconv.ParseFloat(orderResp.ExecutedQuantity, 64)
	order.OrderTime = time.UnixMilli(orderResp.Time)
	order.OrderUpdateTime = time.UnixMilli(orderResp.UpdateTime)
	order.Status = model.GetOrderStatus(model.BinanceSpot, string(orderResp.Status))
	if order.Status != model.CarryStatusSuccess && order.Status != model.CarryStatusFail {
		order.Status = model.CarryStatusWorking
	}
	if order.DealAmount > 0 && order.DealPrice == 0 {
		order.DealPrice = order.Price
	}
	return order
}

// 兼容spot margin
func parseOrderBinance(market string, orderJson *simplejson.Json) (order *model.Order) {
	if orderJson == nil {
		return nil
	}
	order = &model.Order{Market: market}
	suc, _, coin := model.GetCoinFromDialect(market, orderJson.Get(`symbol`).MustString())
	if suc {
		order.Symbol = coin + model.UniStandardTail[model.MarketTypeSpot]
	}
	order.OrderId = strconv.Itoa(orderJson.Get("orderId").MustInt())
	order.OrderTime = time.UnixMilli(orderJson.Get("transactTime").MustInt64())
	order.Price, _ = strconv.ParseFloat(orderJson.Get(`price`).MustString(), 64)
	order.Amount, _ = strconv.ParseFloat(orderJson.Get("origQty").MustString(), 64)
	order.DealPrice, _ = strconv.ParseFloat(orderJson.Get(`executedQty`).MustString(), 64)
	if strings.EqualFold(orderJson.Get(`side`).MustString(), model.OrderSideSell) {
		order.OrderSide = model.OrderSideSell
	} else if strings.EqualFold(orderJson.Get(`side`).MustString(), model.OrderSideBuy) {
		order.OrderSide = model.OrderSideBuy
	}
	order.Status = model.GetOrderStatus(market, orderJson.Get(`status`).MustString())
	order.OrderType = GetStandardOrderType(market, orderJson.Get(`type`).MustString())
	return order
}

var wsOrderUpdateBinance = func(market, key string, msg []byte) {
	value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, key)
	if value != nil && value.(*model.WSConn).Conn != nil {
		value.(*model.WSConn).LastMsgTime = time.Now().UnixMilli()
	}
	resJson, _ := util.NewJSON(msg)
	if resJson == nil {
		return
	}
	switch resJson.Get(`e`).MustString() {
	case `ORDER_TRADE_UPDATE`:
		order, _ := model.AppEnvironment.CrossOrders.Load(strconv.Itoa(resJson.GetPath(`o`, `i`).MustInt()))
		if order != nil {
			order.(*model.Order).DealAmount, _ = strconv.ParseFloat(resJson.GetPath(`o`, `z`).MustString(), 64)
		}
	case `executionReport`:
		order, _ := model.AppEnvironment.CrossOrders.Load(strconv.Itoa(resJson.Get(`i`).MustInt()))
		if order != nil {
			order.(*model.Order).DealAmount, _ = strconv.ParseFloat(resJson.Get(`z`).MustString(), 64)
		}
	}
}

var wsActHandlerBinance = func(market, key string, event []byte) {
	value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, market, key)
	if value != nil && value.(*model.WSConn).Conn != nil {
		value.(*model.WSConn).LastMsgTime = time.Now().UnixMilli()
	}
	responseJson, err := util.NewJSON(event)
	if err == nil && responseJson != nil {
		idInt := responseJson.GetPath(`result`, `orderId`).MustInt()
		wsResp := model.WSResp{RequestId: responseJson.Get(`id`).MustString(), OrderId: strconv.Itoa(idInt)}
		status := responseJson.Get(`status`).MustInt()
		if status == 200 {
			wsResp.Success = true
		} else {
			wsResp.Success = false
			code := responseJson.GetPath(`error`, `code`).MustInt()
			wsResp.Msg = fmt.Sprintf(`%d%s`, code, responseJson.GetPath(`error`, `msg`))
		}
		model.AppEnvironment.WSRespChan <- wsResp
	}
}

func WsOrderServeBinance(account *model.Account, market string) {
	if account == nil {
		return
	}
	apiUrl := ``
	streamUrl := ``
	if market == model.BinanceSpot {
		apiUrl = wsBinanceSpotApi
		streamUrl = wsBinance
	} else if market == model.BinancePerp {
		apiUrl = wsBinancePerpApi
		streamUrl = wsBinancePerp
	}
	value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, market, account.Key)
	if value == nil || value.(*model.WSConn).Conn == nil || time.Now().UnixMilli()-value.(*model.WSConn).LastMsgTime > 180000 {
		conn, err := WsAccountClient(market, account.Key, apiUrl, wsActHandlerBinance)
		if err != nil {
			util.Notice(fmt.Sprintf(`fail to create account ws %s %s`, market, err.Error()))
		}
		util.StoreSyncMap(&model.AppEnvironment.ConnOrder, &model.WSConn{Conn: conn}, market, account.Key)
	}
	valueUpdate, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, account.Key)
	if valueUpdate == nil || valueUpdate.(*model.WSConn).Conn == nil || time.Now().UnixMilli()-valueUpdate.(*model.WSConn).LastMsgTime > 180000 {
		_, listenKey := RenewListenKeyBinance(account, market)
		conn, err := WsAccountClient(market, account.Key, fmt.Sprintf(`%s/ws/%s`, streamUrl, listenKey), wsOrderUpdateBinance)
		if err == nil {
			util.StoreSyncMap(&model.AppEnvironment.ConnOrderUpdate, &model.WSConn{Conn: conn}, market, account.Key)
		} else {
			util.Notice(fmt.Sprintf(`fail to create order update ws %s %s`, market, err.Error()))
		}
	}
}

func cancelOrderBinance(key, secret, market, symbol, orderId string) (suc bool, order *model.Order) {
	var path string
	if market == model.BinanceSpot {
		path = "/api/v3/order"
	} else if market == model.BinanceMargin {
		path = `/sapi/v1/margin/order`
	}
	responseBody := signedRequestBinance(key, secret, model.BinanceSpot, http.MethodDelete, restBinance+path,
		true, map[string]interface{}{`symbol`: symbol, `orderId`: orderId})
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		order = parseOrderBinance(market, orderJson)
		if order != nil && order.Status == model.CarryStatusFail {
			return true, order
		}
	}
	return false, nil
}

func cancelOrdersBinance(key, secret, market, symbol string) bool {
	success, marketType, coin, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if !success {
		return false
	}
	client := binance.NewClient(key, secret)
	var err error
	if market == model.BinanceSpot {
		_, err = client.NewCancelOpenOrdersService().Symbol(dialectSymbol).Do(context.Background())
	} else if market == model.BinanceMargin {
		_, err = client.NewCancelMarginOrderService().Symbol(dialectSymbol).Do(context.Background())
	}
	if err != nil && !strings.Contains(err.Error(), `-2010`) && !strings.Contains(err.Error(), `-2011`) {
		util.Notice("cancelOrdersBinance err: " + err.Error() + " symbol: " + symbol + " marketType: " + marketType + " coin: " + coin + " But dialectSymbol: " + dialectSymbol)
		return false
	}
	return true
}

func getBalanceBinanceMargin(key, secret string) (success bool, balances []*model.Balance) {
	client := binance.NewClient(key, secret)
	balanceResp, err := client.NewGetMarginAccountService().Do(context.Background())
	if err != nil {
		util.Notice(`fail to refresh binance balance ` + err.Error())
		time.Sleep(time.Minute * 5)
		return getBalanceBinanceMargin(key, secret)
	}
	if !balanceResp.TradeEnabled {
		util.Notice(`binance margin balance can not trade`)
		return false, balances
	}
	balances = make([]*model.Balance, 0)
	for _, data := range balanceResp.UserAssets {
		if data.Asset == "" {
			continue
		}
		coin := data.Asset
		balance := &model.Balance{
			Market:      model.BinanceMargin,
			Coin:        coin,
			ID:          model.BinanceMargin + `_` + coin + `_` + util.GetNow().Format(time.RFC3339)[0:10],
			BalanceTime: util.GetNow(),
			AccountId:   key}
		if data.Free != "" { // 持仓,此处按照不进行借币计算
			balance.AvailableWithBorrow, _ = strconv.ParseFloat(data.Free, 64)
		}
		if data.Locked != "" {
			lockAmount, _ := strconv.ParseFloat(data.Locked, 64)
			balance.Amount = balance.AvailableWithBorrow + lockAmount
		}
		if balance.UsdValue == 0 && balance.Amount > 0 {
			symbolStandard := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
			_, price := GetPriceForce(symbolStandard, model.BinanceMargin)
			balance.UsdValue = balance.Amount * price
		}
		balances = append(balances, balance)
	}
	return true, balances
}

func getBalanceBinanceSpot(key string, secret string) (success bool, balances []*model.Balance) {
	client := binance.NewClient(key, secret)
	balanceResp, err := client.NewGetAccountService().Do(context.Background())
	if err != nil {
		util.Notice(`fail to refresh binance balance ` + err.Error())
		time.Sleep(time.Minute * 5)
		return getBalanceBinanceSpot(key, secret)
	}
	if !balanceResp.CanTrade {
		util.Notice(`binance balance can not trade`)
		return false, balances
	}
	balances = make([]*model.Balance, 0)
	for _, data := range balanceResp.Balances {
		if data.Asset == "" {
			continue
		}
		coin := data.Asset
		balance := &model.Balance{
			Market:      model.BinanceSpot,
			Coin:        coin,
			ID:          model.BinanceSpot + `_` + coin + `_` + util.GetNow().Format(time.RFC3339)[0:10],
			BalanceTime: util.GetNow(),
			AccountId:   key}
		if data.Free != "" { // 持仓,此处按照不进行借币计算
			balance.AvailableWithBorrow, _ = strconv.ParseFloat(data.Free, 64)
		}
		if data.Locked != "" {
			lockAmount, _ := strconv.ParseFloat(data.Locked, 64)
			balance.Amount = balance.AvailableWithBorrow + lockAmount
		}
		if balance.UsdValue == 0 && balance.Amount > 0 {
			symbolStandard := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
			_, price := GetPriceForce(symbolStandard, model.BinanceSpot)
			balance.UsdValue = balance.Amount * price
		}
		//if asset[`netAsset`] != nil {
		//	balance.Amount, _ = strconv.ParseFloat(asset[`netAsset`].(string), 64)
		//}
		//if asset[`borrowed`] != nil { //已借数量
		//	balance.Borrow, _ = strconv.ParseFloat(asset[`borrowed`].(string), 64)
		//}
		balances = append(balances, balance)
	}
	return true, balances
}

func queryOpenOrdersBinanceSpot(key, secret, symbol string) (orders []*model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if success || symbol == `` {
		orders = make([]*model.Order, 0)
		listOpenOrderService := binance.NewClient(key, secret).NewListOpenOrdersService()
		if symbol != `` {
			listOpenOrderService = listOpenOrderService.Symbol(dialectSymbol)
		}
		resArray, err := listOpenOrderService.Do(context.Background())
		if err != nil {
			util.Notice(`queryOpenOrdersBinanceSpot err %s %s %s`, symbol, dialectSymbol, err.Error())
		}
		for _, res := range resArray {
			order := parseOrderBinanceSpotSdk(res)
			orders = append(orders, order)
		}
	}
	return
}

func queryOrderBinanceSpot(key, secret, symbol string, orderId string) (order *model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if success {
		orderIdInt, _ := strconv.ParseInt(orderId, 10, 64)
		client := binance.NewClient(key, secret)
		orderResp, err := client.NewGetOrderService().Symbol(dialectSymbol).OrderID(orderIdInt).Do(context.Background())
		if err != nil {
			util.Notice("queryOrderBinanceSpot err: " + err.Error())
			return
		}
		order = parseOrderBinanceSpotSdk(orderResp)
	}
	return
}

func parseTransfer(value map[string]interface{}) (balance *model.Balance, external bool) {
	balance = &model.Balance{}
	if value[`id`] != nil {
		balance.ID = value[`id`].(string)
	}
	if value[`coin`] != nil {
		balance.Coin = value[`coin`].(string)
	}
	if value[`amount`] != nil {
		balance.Amount, _ = strconv.ParseFloat(value[`amount`].(string), 64)
	}
	if value[`status`] != nil {
		balance.Status = value[`status`].(json.Number).String()
	}
	if value[`address`] != nil {
		balance.Address = value[`address`].(string)
	}
	if value[`txId`] != nil {
		balance.TransactionId = value[`txId`].(string)
	}
	if value[`applyTime`] != nil {
		balance.CreatedAt, _ = time.Parse(time.DateTime, value[`applyTime`].(string))
	}
	if value[`insertTime`] != nil {
		insertTime, timeErr := value[`insertTime`].(json.Number).Int64()
		if timeErr == nil {
			balance.CreatedAt = time.Unix(insertTime, 0)
		}
	}
	if value[`completeTime`] != nil {
		balance.UpdatedAt, _ = time.Parse(time.DateTime, value[`completeTime`].(string))
	}
	if value[`transferType`] != nil {
		transferType, typeErr := value[`transferType`].(json.Number).Int64()
		if typeErr == nil {
			if transferType == 0 {
				external = true
			}
		}
	}
	return balance, external
}

// GetWithdrawInfo
// status 0:Email Sent,1:Cancelled 2:Awaiting Approval 3:Rejected 4:Processing 5:Failure 6:Completed
// transferType: 1 for internal transfer, 0 for external transfer
func GetWithdrawInfo(market, key, secret string) (balances []*model.Balance) {
	balances = make([]*model.Balance, 0)
	responseBody := signedRequestBinance(key, secret, model.BinanceSpot, http.MethodGet, restBinance+`/sapi/v1/capital/withdraw/history`, true, nil)
	withdrawJson, withdrawErr := util.NewJSON(responseBody)
	if withdrawErr == nil {
		for _, data := range withdrawJson.MustArray() {
			if data == nil {
				continue
			}
			balance, external := parseTransfer(data.(map[string]interface{}))
			if balance != nil && external == true {
				balance.Action = -1
				balance.Market = market
				balances = append(balances, balance)
			}
		}
	}
	responseBody = signedRequestBinance(key, secret, model.BinanceSpot, http.MethodGet, restBinance+`/sapi/v1/capital/deposit/hisrec`, true, nil)
	depositJson, depositErr := util.NewJSON(responseBody)
	if depositErr == nil {
		for _, data := range depositJson.MustArray() {
			if data == nil {
				continue
			}
			balance, external := parseTransfer(data.(map[string]interface{}))
			if balance != nil && external == true {
				balance.Action = 1
				balance.Market = market
				balances = append(balances, balance)
			}
		}
	}
	return balances
}

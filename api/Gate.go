package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/antihax/optional"
	gateApi "github.com/gateio/gateapi-go/v6"
	gateWs "github.com/gateio/gatews/go"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"time"
)

var apiClientsGate = make(map[string]*gateApi.APIClient)
var apiCtxGate = make(map[string]context.Context)
var pingGate = false
var pingGatePrivate = false

const wsStepGate = 100

func getClientGate(key, secret string) (apiClient *gateApi.APIClient, ctx context.Context) {
	if apiClientsGate[key] == nil {
		apiClientsGate[key] = gateApi.NewAPIClient(gateApi.NewConfiguration())
		apiCtxGate[key] = context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
			Key: key, Secret: secret})
	}
	return apiClientsGate[key], apiCtxGate[key]
}

func getMarketsGate(key, secret string) (success bool, marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	appendSpotMarketsGate(key, secret, marketInfos)
	appendFutureMarketGate(key, secret, marketInfos)
	return true, marketInfos
}

// 市场minSize按照张数计算的
func appendFutureMarketGate(key, secret string, marketInfos map[string]*model.MarketInfo) {
	client, ctx := getClientGate(key, secret)
	contracts, _, futureErr := client.FuturesApi.ListFuturesContracts(ctx, `usdt`, nil)
	if futureErr != nil {
		panicGateError(key, "ListFuturesContracts", futureErr)
		time.Sleep(time.Minute * 5)
		appendFutureMarketGate(key, secret, marketInfos)
		return
	}
	for _, contract := range contracts {
		if contract.InDelisting {
			continue
		}
		if !contract.EnableCredit {
			continue
		}
		marketInfo := &model.MarketInfo{Market: model.Gate}
		success, _, coin := model.GetCoinFromDialect(model.Gate, contract.Name)
		if !success {
			continue
		}
		marketInfo.Name = coin + model.UniStandardTail[model.MarketTypePerp]
		minPrice, _ := strconv.ParseFloat(contract.OrderPriceRound, 64)
		marketInfo.PriceIncrement = minPrice
		marketInfo.PriceDecimal = util.NumDecPlaces(minPrice)
		marketInfo.SizeMin = float64(contract.OrderSizeMin)
		marketInfo.SizeIncrement = marketInfo.SizeMin
		marketInfo.SizeMax = float64(contract.OrderSizeMax)
		marketInfo.CTCurrency = coin
		marketInfo.CTValue, _ = strconv.ParseFloat(contract.QuantoMultiplier, 64)
		marketInfo.BuyLimitPriceRatio, _ = strconv.ParseFloat(contract.OrderPriceDeviate, 64)
		marketInfo.SellLimitPriceRatio, _ = strconv.ParseFloat(contract.OrderPriceDeviate, 64)
		marketInfos[marketInfo.Name] = marketInfo
	}
}

// 市场minSize未根据最小下单美元数进行计算
func appendSpotMarketsGate(key, secret string, marketInfos map[string]*model.MarketInfo) {
	client, ctx := getClientGate(key, secret)
	spotCurrencyPairs, _, spotErr := client.SpotApi.ListCurrencyPairs(ctx)
	if spotErr != nil {
		time.Sleep(time.Minute * 5)
		panicGateError(key, "ListCurrencyPairs", spotErr)
		appendSpotMarketsGate(key, secret, marketInfos)
		return
	}
	for _, spot := range spotCurrencyPairs {
		success, _, coin := model.GetCoinFromDialect(model.Gate, spot.Id)
		if spot.TradeStatus != "tradable" || !success {
			continue
		}
		marketInfo := &model.MarketInfo{Market: model.Gate}
		marketInfo.Name = coin + model.UniStandardTail[model.MarketTypeSpot]
		marketInfo.PriceDecimal = int(spot.Precision)
		marketInfo.PriceIncrement = 1 / math.Pow10(int(spot.Precision))
		marketInfo.SizeIncrement = 1 / math.Pow10(int(spot.AmountPrecision))
		if spot.MinQuoteAmount != "" {
			marketInfo.MoneyMin, _ = strconv.ParseFloat(spot.MinQuoteAmount, 64)
			marketInfo.SizeMin = marketInfo.SizeIncrement
		}
		if spot.MinBaseAmount != "" {
			marketInfo.SizeMin, _ = strconv.ParseFloat(spot.MinBaseAmount, 64)
		}
		marketInfos[spot.Id] = marketInfo
	}
}

func setPosSideGate(key, secret string) {
	client, ctx := getClientGate(key, secret)
	mode, _, err := client.FuturesApi.SetDualMode(ctx, `usdt`, false)
	if err != nil {
		panicGateError(key, "setPosSideGate", err)
	}
	marshal, _ := json.Marshal(mode)
	util.Notice(fmt.Sprintf("set gate dual mode success,position: %s", marshal))
}

func setMarginSettingGate(key, secret string) {
	client, ctx := getClientGate(key, secret)
	mode, _, err := client.MarginApi.SetAutoRepay(ctx, "on")
	if err != nil {
		panicGateError(key, "setMarginSettingGate", err)
	}
	marshal, _ := json.Marshal(mode)
	util.Notice(fmt.Sprintf("set gate margin auto repay success,response: %s", marshal))
}

func TransferGate(key string, secret string, transferType, currency string, amount float64) {
	client, ctx := getClientGate(key, secret)
	param := gateApi.Transfer{Currency: currency, Amount: fmt.Sprintf("%f", amount), Settle: `usdt`}
	if transferType == "MAIN_UMFUTURE" {
		param.From = "spot"
		param.To = "futures"
		_, res, endErr := client.WalletApi.Transfer(ctx, param)
		if endErr != nil {
			if res != nil {
				util.Notice(fmt.Sprintf(`fail to transfer status %s`, res.Status))
			}
			panicGateError(key, "transferGate", endErr)
		}
	} else if transferType == "UMFUTURE_MAIN" {
		param.From = "futures"
		param.To = "spot"
		_, res, err := client.WalletApi.Transfer(ctx, param)
		if err != nil {
			panicGateError(key, "transferGate", err)
			util.Notice(fmt.Sprintf(`fail to transfer status %s`, res.Status))
		}
	}
}

func panicGateError(key, function string, err error) {
	var e gateApi.GateAPIError
	if errors.As(err, &e) {
		util.SocketInfo(fmt.Sprintf("key %s function: %s Gate API error, label: %s, message: %s",
			key, function, e.Label, e.Message))
	}
	util.Notice(function + err.Error())
}

var tickerHandler = gateWs.NewCallBack(func(msg *gateWs.UpdateMsg) {
	if msg.Error != nil && (!strings.Contains(msg.Error.Message, "futures.ping") && !strings.Contains(msg.Error.Message, "spot.ping")) {
		util.Notice(fmt.Sprintf("callback error in ticker: %s %s", msg.Channel, msg.Error.Error()))
		return
	}
	var bidAsk model.BidAsk
	var symbol string
	switch msg.Channel {
	case gateWs.ChannelSpotBookTicker:
		var update gateWs.SpotBookTickerMsg
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Notice(fmt.Sprintf("spot book ticker Unmarshal err:%s %s", model.Gate, err.Error()))
			return
		}
		success, _, coin := model.GetCoinFromDialect(model.Gate, update.CurrencyPair)
		if !success || len(strconv.Itoa(int(update.TimeInMilli))) != 13 {
			return
		}
		symbol = coin + model.UniStandardTail[model.MarketTypeSpot]
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(update.Bid, 64)
		bidAmount, _ := strconv.ParseFloat(update.BidSize, 64)
		askPrice, _ := strconv.ParseFloat(update.Ask, 64)
		askAmount, _ := strconv.ParseFloat(update.AskSize, 64)
		bidAsk = model.BidAsk{Ts: int(update.TimeInMilli), TsReceived: now, UpdateId: update.LastId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.Gate, Symbol: symbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.Gate, Symbol: symbol}}}
	// Periodically notify top bids and asks snapshot with limited levels.
	case gateWs.ChannelSpotOrderBook:
		var update gateWs.SpotUpdateAllDepthMsg
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Notice(fmt.Sprintf("spot book ticker Unmarshal err:%s %s", model.Gate, err.Error()))
			return
		}
		success, _, coin := model.GetCoinFromDialect(model.Gate, update.CurrencyPair)
		if !success || len(strconv.Itoa(int(update.TimeInMilli))) != 13 {
			return
		}
		symbol = coin + model.UniStandardTail[model.MarketTypeSpot]
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidAsk = model.BidAsk{Ts: int(update.TimeInMilli), TsReceived: now, UpdateId: update.LastUpdateId,
			Bids: []model.Tick{}, Asks: []model.Tick{}}
		for i := 0; i < len(update.Bid) && i < len(update.Ask); i++ {
			bidPrice, _ := strconv.ParseFloat(update.Bid[0][0], 64)
			bidAmount, _ := strconv.ParseFloat(update.Bid[0][1], 64)
			askPrice, _ := strconv.ParseFloat(update.Ask[0][0], 64)
			askAmount, _ := strconv.ParseFloat(update.Ask[0][1], 64)
			bidAsk.Bids = append(bidAsk.Bids, model.Tick{Price: bidPrice, Amount: bidAmount, Market: model.Gate, Symbol: symbol})
			bidAsk.Asks = append(bidAsk.Asks, model.Tick{Price: askPrice, Amount: askAmount, Market: model.Gate, Symbol: symbol})
		}
	// Push best bid and ask in real-time.
	case gateWs.ChannelFutureBookTicker:
		var update gateWs.FuturesBookTicker
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Notice(fmt.Sprintf("future book ticker Unmarshal err:%s %s", model.Gate, err.Error()))
			return
		}
		success, _, coin := model.GetCoinFromDialect(model.Gate, update.Contract)
		if !success || len(strconv.Itoa(int(update.TimeMillis))) != 13 {
			return
		}
		symbol = coin + model.UniStandardTail[model.MarketTypePerp]
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(update.BestBidPrice, 64)
		_, bidAmount := model.ParseRealAmount(model.Gate, symbol, float64(update.BestBidSize))
		askPrice, _ := strconv.ParseFloat(update.BestAskPrice, 64)
		_, askAmount := model.ParseRealAmount(model.Gate, symbol, float64(update.BestAskSize))
		bidAsk = model.BidAsk{Ts: int(update.TimeMillis), TsReceived: now, UpdateId: update.UpdateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.Gate, Symbol: symbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.Gate, Symbol: symbol}}}
	}
	markets := model.AppEnvironment
	haveOld, old := markets.GetBidAsk(symbol, model.Gate)
	if haveOld && old.Ts > bidAsk.Ts {
		return
	}
	if markets.SetBidAsk(symbol, model.Gate, &bidAsk) {
		funcHandlers := GetFunctions(model.Gate, symbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), model.Gate, symbol)
				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
					go value.(model.CarryHandler)(setting, &bidAsk)
				}
				return true
			})
		}
	}
})

var markPriceHandler = gateWs.NewCallBack(func(msg *gateWs.UpdateMsg) {
	if msg.Error != nil && (!strings.Contains(msg.Error.Message, "futures.ping") && !strings.Contains(msg.Error.Message, "spot.ping")) {
		util.Notice(fmt.Sprintf("callback error: %s %s", msg.Channel, msg.Error.Error()))
		return
	}
	if msg.Channel == gateWs.ChannelFutureTicker {
		var tickers []gateWs.FuturesTicker
		if err := json.Unmarshal(msg.Result, &tickers); err != nil {
			util.Notice(fmt.Sprintf("future mark price Unmarshal err:%s %s %s", model.Gate, err.Error(), msg.Result))
			return
		}
		for _, update := range tickers {
			success, _, coin := model.GetCoinFromDialect(model.Gate, update.Contract)
			if !success {
				return
			}
			symbol := coin + model.UniStandardTail[model.MarketTypePerp]
			price, _ := strconv.ParseFloat(update.MarkPrice, 64)
			ticker := &model.MarkPriceInfo{MarkPrice: price, Ts: int(msg.TimeMs)}
			markets := model.AppEnvironment
			markets.SetMarkPriceInfo(symbol, model.Gate, ticker)
		}
	}
	return
})

var wsAccountHandlerGate = func(market, key string, event []byte) {
	responseJson, err := util.NewJSON(event)
	if err != nil || responseJson == nil {
		return
	}
	valueSpot, _ := util.LoadSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypeSpot, key)
	valueFuture, _ := util.LoadSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypePerp, key)
	if valueSpot == nil || valueFuture == nil {
		return
	}
	if responseJson.Get(`ack`).MustBool() {
		return
	}
	channel := responseJson.Get(`channel`).MustString()
	ts := responseJson.Get(`time_ms`).MustInt64()
	if `futures.pong` == channel {
		valueFuture.(*model.WSConn).LastMsgTime = ts
	} else if `spot.pong` == channel {
		valueSpot.(*model.WSConn).LastMsgTime = ts
	} else {
		requestId := responseJson.Get(`request_id`).MustString()
		result := responseJson.GetPath(`header`, `status`).MustString()
		channel := responseJson.GetPath(`header`, `channel`).MustString()
		if !strings.Contains(channel, `_place`) {
			return
		}
		if result == `200` {
			model.AppEnvironment.WSRespChan <- model.WSResp{RequestId: requestId, Success: true}
		} else {
			errs := responseJson.GetPath(`data`, `errs`, `message`).MustString()
			model.AppEnvironment.WSRespChan <- model.WSResp{RequestId: requestId, Success: false, Msg: errs}
		}
	}
}

func maintainAccountChannelGate() {
	if pingGatePrivate {
		return
	}
	pingGatePrivate = true
	for {
		time.Sleep(time.Second * 20)
		accounts := model.AppConfig.GetAccounts(model.Gate)
		for _, account := range accounts {
			wsSpot, _ := util.LoadSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypeSpot, account.Key)
			if wsSpot != nil {
				if err := wsSpot.(*model.WSConn).Conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(
					`{"time": %d, "channel" : "spot.ping"}`, time.Now().Unix()))); err != nil {
					util.Notice(fmt.Sprintf("send account spot ping message err:%s %s", model.Gate, err.Error()))
				}
			}
			wsFuture, _ := util.LoadSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypePerp, account.Key)
			if wsFuture != nil {
				if err := wsFuture.(*model.WSConn).Conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(
					`{"time": %d, "channel" : "futures.ping"}`, time.Now().Unix()))); err != nil {
					util.Notice(fmt.Sprintf("send account futures ping message err:%s %s", model.Gate, err.Error()))
				}
			}
		}
	}
}

func WSAccountServeGate(account *model.Account) {
	if account == nil {
		return
	}
	valueSpot, _ := util.LoadSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypeSpot, account.Key)
	valueFuture, _ := util.LoadSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypePerp, account.Key)
	if valueSpot != nil && valueFuture != nil && valueSpot.(*model.WSConn).Conn != nil && valueFuture.(*model.WSConn).Conn != nil &&
		time.Now().UnixMilli()-valueSpot.(*model.WSConn).LastMsgTime < 60000 && time.Now().UnixMilli()-valueFuture.(*model.WSConn).LastMsgTime < 60000 {
		return
	}
	ts := time.Now().Unix()
	hashSpot := hmac.New(sha512.New, []byte(account.Secret))
	hashSpot.Write([]byte(fmt.Sprintf("api\nspot.login\n\n%d", ts)))
	signSpot := hex.EncodeToString(hashSpot.Sum(nil))
	connSpot, errSpot := WsAccountClient(model.Gate, account.Key, gateWs.BaseUrl, wsAccountHandlerGate)
	if errSpot != nil || connSpot == nil {
		util.Notice(fmt.Sprintf("gate wsAccount connect errSpot: %s %s", errSpot.Error(), account.Key))
		return
	}
	msgSpot := fmt.Sprintf(`{"time": %d,"channel": "spot.login","event": "api","payload": {"api_key": "%s",
    		"signature": "%s","timestamp": "%d","req_id": "request%d"}}`, ts, account.Key, signSpot, ts, ts)
	if err := connSpot.WriteMessage(websocket.TextMessage, []byte(msgSpot)); err != nil {
		util.DelSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypeSpot, account.Key)
	} else {
		util.StoreSyncMap(&model.AppEnvironment.AccountConns, &model.WSConn{Conn: connSpot}, model.Gate, model.MarketTypeSpot, account.Key)
	}
	hashFuture := hmac.New(sha512.New, []byte(account.Secret))
	hashFuture.Write([]byte(fmt.Sprintf("api\nfutures.login\n\n%d", ts)))
	signFuture := hex.EncodeToString(hashFuture.Sum(nil))
	msgFuture := fmt.Sprintf(`{"time": %d,"channel": "futures.login","event": "api","payload": {"api_key": "%s",
    		"signature": "%s","timestamp": "%d","req_id": "request%d"}}`, ts, account.Key, signFuture, ts, ts)
	connFuture, errFuture := WsAccountClient(model.Gate, account.Key, gateWs.FuturesUsdtUrl, wsAccountHandlerGate)
	if errFuture != nil || connFuture == nil {
		util.Notice(fmt.Sprintf("gate wsAccount connect errFuture: %s %s", errFuture.Error(), account.Key))
		return
	}
	if err := connFuture.WriteMessage(websocket.TextMessage, []byte(msgFuture)); err != nil {
		util.DelSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypePerp, account.Key)
	} else {
		util.StoreSyncMap(&model.AppEnvironment.AccountConns, &model.WSConn{Conn: connFuture}, model.Gate, model.MarketTypePerp, account.Key)
	}
	go maintainAccountChannelGate()
}

func WsDepthServeGateNew(environment *model.Environment, market string) (socketMap map[*websocket.Conn]bool, msgChans []chan struct{}, connectErr error) {
	var spotSubs, spotOrderBookSubs, futureSubs []interface{}
	socketMap = make(map[*websocket.Conn]bool)
	msgChans = make([]chan struct{}, 0)
	symbols := GetMarketSymbols(model.Gate)
	for symbol := range symbols {
		if strings.LastIndex(symbol, model.UniStandardTail[model.MarketTypeSpot]) == len(symbol)-len(model.UniStandardTail[model.MarketTypeSpot]) &&
			len(symbol)-len(model.UniStandardTail[model.MarketTypeSpot]) > 0 {
			spotSubs = append(spotSubs, symbol)
			spotOrderBookSubs = append(spotOrderBookSubs, []string{symbol, "5", "100ms"})
		} else if strings.LastIndex(symbol, model.UniStandardTail[model.MarketTypePerp]) == len(symbol)-len(model.UniStandardTail[model.MarketTypePerp]) &&
			len(symbol)-len(model.UniStandardTail[model.MarketTypePerp]) > 0 {
			futureSubs = append(futureSubs, symbol)
		}
	}
	//spotOrderBookSockets, spotOrderBookChannels, spotOrderBookErr := WebSocketClient(model.Gate, gateWs.BaseUrl, spotOrderBookSubs, subscribeHandler, wsHandler, wsStepGate)
	//if spotOrderBookErr == nil {
	//	util.Info(`finish connect public gate spot order book ws `)
	//	msgChans = append(msgChans, spotOrderBookChannels...)
	//	for conn, b := range spotOrderBookSockets {
	//		socketMap[conn] = b
	//	}
	//}
	spotBookTickerSockets, spotBookTickerChannels, spotBookTickerErr := WebSocketClient(model.Gate, gateWs.BaseUrl, spotSubs, subscribeHandler, wsHandler, wsStepGate)
	if spotBookTickerErr == nil {
		util.Info(`finish connect public gate spot book ticker ws `)
		msgChans = append(msgChans, spotBookTickerChannels...)
		for conn, b := range spotBookTickerSockets {
			socketMap[conn] = b
		}
	}
	perpBookTickerSockets, perpBookTickerChannels, perpBookTickerErr := WebSocketClient(model.Gate, gateWs.FuturesUsdtUrl, futureSubs, subscribeHandler, wsHandler, wsStepGate)
	if perpBookTickerErr == nil {
		util.Info(`finish connect public gate perp book ticker ws `)
		msgChans = append(msgChans, perpBookTickerChannels...)
		for conn, b := range perpBookTickerSockets {
			socketMap[conn] = b
		}
	}
	perpMarkPriceSockets, perpMarkPriceChannels, perpMarkPriceErr := WebSocketClient(model.Gate, gateWs.FuturesUsdtUrl, futureSubs, subscribeMarkPriceHandler, wsHandler, wsStepGate)
	if perpMarkPriceErr == nil {
		util.Info(`finish connect public gate perp mark price ws `)
		msgChans = append(msgChans, perpMarkPriceChannels...)
		for conn, b := range perpMarkPriceSockets {
			socketMap[conn] = b
		}
	}
	go maintainChannelGate()
	environment.SocketsTick.Store(market, socketMap)
	environment.MsgChanTick.Store(market, msgChans)
	return
}

var wsHandler = func(event []byte) {
	msg := &gateWs.UpdateMsg{}
	if err := json.Unmarshal(event, msg); err != nil {
		util.Notice(fmt.Sprintf("gate ws message Unmarshal err:%s", err.Error()))
		return
	}
	if msg.Channel == gateWs.ChannelFutureTicker {
		markPriceHandler(msg)
	} else {
		tickerHandler(msg)
	}
}

var subscribeMarkPriceHandler = func(market string, connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	var symbols []string
	for _, subscribe := range subscribes {
		_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, subscribe.(string))
		symbols = append(symbols, dialectSymbol)
	}
	subscribeMap := map[string]interface{}{
		"time":    time.Now().Unix(),
		"channel": "futures.tickers",
		"event":   "subscribe",
		"payload": symbols,
	}
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.Gate, connection, subscribeMessage); err != nil {
		util.SocketInfo(" gate can not subscribe perp symbols %s %s", subscribeMessage, err.Error())
	}
	util.Info(`gate subscribed ` + string(subscribeMessage))
	time.Sleep(500 * time.Millisecond)
	return err
}

var subscribeHandler = func(market string, connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	switch subscribes[0].(type) {
	case string: //ticker订阅
		if strings.LastIndex(subscribes[0].(string), model.UniStandardTail[model.MarketTypePerp]) ==
			len(subscribes[0].(string))-len(model.UniStandardTail[model.MarketTypePerp]) &&
			len(subscribes[0].(string))-len(model.UniStandardTail[model.MarketTypePerp]) > 0 { //合约ticker订阅
			var symbols []string
			for _, subscribe := range subscribes {
				_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, subscribe.(string))
				symbols = append(symbols, dialectSymbol)
			}
			subscribeMap := map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "futures.book_ticker",
				"event":   "subscribe",
				"payload": symbols,
			}
			subscribeMessage := util.JsonEncodeToByte(subscribeMap)
			if err = SendToConnection(model.Gate, connection, subscribeMessage); err != nil {
				util.SocketInfo(" gate can not subscribe perp symbols %s %s", subscribeMessage, err.Error())
			}
			util.Info(`gate subscribed ` + string(subscribeMessage))
			time.Sleep(500 * time.Millisecond)
		} else { //现货ticker订阅
			var symbols []string
			for _, subscribe := range subscribes {
				_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, subscribe.(string))
				symbols = append(symbols, dialectSymbol)
			}
			subscribeMap := map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "spot.book_ticker",
				"event":   "subscribe",
				"payload": symbols,
			}
			subscribeMessage := util.JsonEncodeToByte(subscribeMap)
			if err = SendToConnection(model.Gate, connection, subscribeMessage); err != nil {
				util.SocketInfo(" gate can not subscribe spot symbols %s %s", subscribeMessage, err.Error())
			}
			util.Info(`gate subscribed ` + string(subscribeMessage))
			time.Sleep(500 * time.Millisecond)
		}
	case []string: //orderbook订阅
		for _, subscribe := range subscribes {
			_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, subscribe.([]string)[0])
			subscribe.([]string)[0] = dialectSymbol
			subscribeMap := map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "spot.order_book",
				"event":   "subscribe",
				"payload": subscribe,
			}
			subscribeMessage := util.JsonEncodeToByte(subscribeMap)
			if err = SendToConnection(model.Gate, connection, subscribeMessage); err != nil {
				util.SocketInfo(" gate can not subscribe spot order book symbol %s %s", subscribeMessage, err.Error())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	return err
}

func maintainChannelGate() {
	if pingGate {
		return
	}
	pingGate = true
	for {
		time.Sleep(time.Second * 15)
		if err := SendToAllTickerSockets(model.Gate, util.JsonEncodeToByte(map[string]interface{}{"time": time.Now().Unix(), "channel": "spot.ping"})); err != nil {
			util.SocketInfo("gate channel ping error " + err.Error())
		}
		if err := SendToAllTickerSockets(model.Gate, util.JsonEncodeToByte(map[string]interface{}{"time": time.Now().Unix(), "channel": "futures.ping"})); err != nil {
			util.SocketInfo("gate channel ping error " + err.Error())
		}
	}
}

func getBalanceGate(key string, secret string) (success bool, balances []*model.Balance, totalInUsd float64, collateral *Collateral) {
	client, ctx := getClientGate(key, secret)
	portfolioAccount, _, portfolioErr := client.UnifiedApi.ListUnifiedAccounts(ctx, nil)
	if portfolioErr != nil {
		panicGateError(key, "getBalanceGate", portfolioErr)
		time.Sleep(time.Minute * 5)
		util.SocketInfo(`fail to refresh balance gate`)
		return getBalanceGate(key, secret)
	}
	if portfolioAccount.Locked {
		util.Notice(fmt.Sprintf("portfolio account is locked"))
		return false, balances, 0, nil
	}
	totalInUsd, _ = strconv.ParseFloat(portfolioAccount.UnifiedAccountTotalEquity, 64)
	collateralAvailable, _ := strconv.ParseFloat(portfolioAccount.TotalAvailableMargin, 64)
	totalMaintenanceMargin, _ := strconv.ParseFloat(portfolioAccount.TotalMaintenanceMargin, 64)
	collateral = &Collateral{Available: collateralAvailable, Occupied: totalMaintenanceMargin}
	balances = make([]*model.Balance, 0)
	for coin, item := range portfolioAccount.Balances {
		balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Gate, Coin: coin}
		balance.FrozenAmount, _ = strconv.ParseFloat(item.Freeze, 64)
		balance.Borrow, _ = strconv.ParseFloat(item.Borrowed, 64)
		balance.AvailableWithBorrow, _ = strconv.ParseFloat(item.Available, 64)
		balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
		canBorrow := 0.0 //不允许借币
		balance.AvailableWithBorrow = math.Max(0, balance.Amount) + canBorrow
		_, price := GetPriceForce(key, secret, balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.Gate)
		balance.UsdValue = balance.Amount * price
		balances = append(balances, balance)
	}
	return true, balances, totalInUsd, collateral
}

func getPositionsGate(key string, secret string) (success bool, positions []*Position) {
	client, ctx := getClientGate(key, secret)
	positionList, _, positionsErr := client.FuturesApi.ListPositions(ctx, `usdt`, nil)
	if positionsErr != nil {
		panicGateError(key, `getPositionsGate`, positionsErr)
		time.Sleep(time.Minute)
		util.SocketInfo(`fail to refresh future balance gate`)
		return getPositionsGate(key, secret)
	}
	positions = make([]*Position, 0)
	for _, item := range positionList {
		getCoin, _, coin := model.GetCoinFromDialect(model.Gate, item.Contract)
		if !getCoin {
			continue
		}
		currency := coin + model.UniStandardTail[model.MarketTypePerp]
		position := &Position{Market: model.Gate, Ts: util.GetNowUnixMillion(), Currency: currency}
		_, realAmount := model.ParseRealAmount(model.Gate, currency, float64(item.Size))
		position.Holding = realAmount
		position.LeverRate, _ = strconv.ParseInt(item.CrossLeverageLimit, 10, 64)
		position.EntryPrice, _ = strconv.ParseFloat(item.EntryPrice, 64)
		position.Margin, _ = strconv.ParseFloat(item.Margin, 64)
		position.LiquidationPrice, _ = strconv.ParseFloat(item.LiqPrice, 64)
		position.ProfitUnreal, _ = strconv.ParseFloat(item.UnrealisedPnl, 64)
		position.MinimumMaintenanceMargin, _ = strconv.ParseFloat(item.MaintenanceMargin, 64)
		currentMargin, _ := strconv.ParseFloat(item.Margin, 64)        //当前保证金，盈亏也算在里面
		initialMargin, _ := strconv.ParseFloat(item.InitialMargin, 64) //初始保证金
		position.Margin = math.Max(currentMargin, initialMargin)
		if position.Holding != 0 {
			positions = append(positions, position)
		}
	}
	return true, positions
}

// getPriceGate
func _(key, secret, symbol string) (success bool, price float64) {
	client, ctx := getClientGate(key, secret)
	_, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	if marketType == model.MarketTypeSpot {
		param := &gateApi.ListTickersOpts{CurrencyPair: optional.NewString(dialectSymbol)}
		tickers, _, _ := client.SpotApi.ListTickers(ctx, param)
		if tickers != nil && len(tickers) > 0 {
			tickerBytes, err := json.Marshal(tickers[0].Last)
			if err == nil {
				str := strings.Replace(string(tickerBytes), `"`, ``, -1)
				price, err = strconv.ParseFloat(str, 64)
				return err == nil, price
			}
		}
	} else if marketType == model.MarketTypePerp {
		contract, _, err := client.FuturesApi.GetFuturesContract(ctx, `usdt`, dialectSymbol)
		if err == nil {
			price, err = strconv.ParseFloat(contract.LastPrice, 64)
			return err == nil, price
		}
	}
	return false, 0
}

func cancelOrderGate(key, secret, symbol, orderId string) (result bool) {
	client, ctx := getClientGate(key, secret)
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	if success && marketType == model.MarketTypeSpot {
		param := &gateApi.CancelOrderOpts{}
		param.Account = optional.NewString("spot")
		order, _, err := client.SpotApi.CancelOrder(ctx, orderId, dialectSymbol, param)
		if err != nil {
			panicGateError(key, fmt.Sprintf("cancelSpotOrdersGate %s %s", dialectSymbol, orderId), err)
			return false
		}
		marshal, _ := json.Marshal(order)
		util.SocketInfo(`cancel related order response: %s`, marshal)
		return true
	} else if success && marketType == model.MarketTypePerp {
		order, _, err := client.FuturesApi.CancelFuturesOrder(ctx, `usdt`, orderId)
		if err != nil {
			panicGateError(key, `cancelFutureOrderGate`, err)
			return false
		}
		marshal, _ := json.Marshal(order)
		util.SocketInfo(`cancel future order response: %s`, marshal)
		return true
	}
	util.Notice(`cancel can not recognize gate symbol %s`, dialectSymbol)
	return false
}

func cancelOrdersGate(key string, secret string, symbol string) (result bool) {
	client, ctx := getClientGate(key, secret)
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	if success && marketType == model.MarketTypeSpot {
		param := &gateApi.CancelOrdersOpts{}
		param.Account = optional.NewString("spot")
		orders, _, err := client.SpotApi.CancelOrders(ctx, dialectSymbol, param)
		if err != nil {
			panicGateError(key, "cancelSpotOrdersGate", err)
			return false
		}
		marshal, _ := json.Marshal(orders)
		util.SocketInfo(`cancel related orders response: %s`, marshal)
		return true
	} else if success && marketType == model.MarketTypePerp {
		orders, _, err := client.FuturesApi.CancelFuturesOrders(ctx, `usdt`, dialectSymbol, nil)
		if err != nil {
			panicGateError(key, "cancelFutureOrdersGate", err)
			return false
		}
		marshal, _ := json.Marshal(orders)
		util.SocketInfo(`cancel future orders response: %s`, marshal)
		return true
	}
	util.Notice(`cancel orders can not recognize gate symbol %s`, symbol)
	return false
}

func placeOrderGate(account *model.Account, isWs bool, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	client, ctx := getClientGate(account.Key, account.Secret)
	orderPrice, decimal := model.FormatPrice(model.Gate, symbol, price)
	orderPriceStr := util.CutTailZero(strconv.FormatFloat(orderPrice, 'f', decimal, 64))
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	order.Symbol = symbol
	order.Price = orderPrice
	if !success {
		return
	}
	ts := time.Now().Unix()
	if marketType == model.MarketTypeSpot {
		relatedOrder := gateApi.Order{Price: orderPriceStr, Side: orderSide, CurrencyPair: dialectSymbol, Type: orderType}
		relatedOrder.Account = "spot"
		relatedOrder.Amount = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Gate, symbol, amount, price, false)))
		util.SocketInfo(`create spot order request: %v`, relatedOrder)
		if isWs {
			param := map[string]interface{}{"text": `t-` + order.OrderId, `currency_pair`: dialectSymbol, `type`: orderType,
				`account`: `spot`, `side`: orderSide, `amount`: relatedOrder.Amount, `price`: orderPriceStr}
			reqMap := map[string]interface{}{`time`: ts, `channel`: `spot.order_place`, `event`: `api`,
				`payload`: map[string]interface{}{`req_id`: order.OrderId, `req_param`: param}}
			wsOrderMsg := util.JsonEncodeToByte(reqMap)
			value, _ := util.LoadSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypeSpot, account.Key)
			if value != nil && value.(*model.WSConn).Conn != nil {
				if err := value.(*model.WSConn).Conn.WriteMessage(websocket.TextMessage, wsOrderMsg); err != nil {
					util.Notice(fmt.Sprintf(`fail to order gate ws %s %s`, string(wsOrderMsg), err.Error()))
				}
			}
		} else {
			createOrder, _, err := client.SpotApi.CreateOrder(ctx, relatedOrder)
			if err != nil {
				panicGateError(account.Key, "placeSpotOrderGate", err)
				order.Status = model.CarryStatusFail
				order.OrderId = ``
				order.ErrCode = err.Error()
			} else {
				orderResp, _ := json.Marshal(createOrder)
				util.Notice(`create spot order response: %s`, orderResp)
				order.OrderId = createOrder.Id
				//order.Symbol = createOrder.CurrencyPair
				secondUnix, _ := strconv.ParseInt(createOrder.CreateTime, 10, 64)
				order.OrderTime = time.Unix(secondUnix, 0)
				if orderPriceStr != createOrder.Price {
					util.Notice(fmt.Sprintf(`diff spot price req %s resp %s`, orderPriceStr, createOrder.Price))
				}
				order.Price, _ = strconv.ParseFloat(createOrder.Price, 64)
				order.OrderSide = createOrder.Side
				order.Amount, _ = strconv.ParseFloat(createOrder.Amount, 64)
				order.OrderType = createOrder.Type
				if createOrder.Status == "cancelled" {
					order.Status = model.CarryStatusFail
				} else {
					order.Status = model.CarryStatusWorking
				}
			}
		}
	} else if marketType == model.MarketTypePerp {
		futuresOrder := gateApi.FuturesOrder{Price: orderPriceStr, Contract: dialectSymbol}
		futuresOrder.Size, _ = strconv.ParseInt(util.CutTailZero(
			fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Gate, symbol, amount, price, false))), 10, 64)
		if orderSide == model.OrderSideSell {
			futuresOrder.Size = -1 * futuresOrder.Size
		}
		util.SocketInfo(`create future order request: %v`, futuresOrder)
		if isWs {
			param := map[string]interface{}{`contract`: dialectSymbol, `size`: futuresOrder.Size,
				`price`: orderPriceStr, `tif`: `gtc`, `text`: `t-` + order.OrderId}
			reqMap := map[string]interface{}{`time`: ts, `channel`: `futures.order_place`, `event`: `api`,
				`payload`: map[string]interface{}{`req_id`: order.OrderId, `req_param`: param}}
			wsOrderMsg := util.JsonEncodeToByte(reqMap)
			value, _ := util.LoadSyncMap(&model.AppEnvironment.AccountConns, model.Gate, model.MarketTypePerp, account.Key)
			if value != nil && value.(*model.WSConn).Conn != nil {
				if err := value.(*model.WSConn).Conn.WriteMessage(websocket.TextMessage, wsOrderMsg); err != nil {
					util.Notice(fmt.Sprintf(`fail to order gate ws %s %s`, string(wsOrderMsg), err.Error()))
				}
			}
		} else {
			createFuturesOrder, _, err := client.FuturesApi.CreateFuturesOrder(ctx, `usdt`, futuresOrder)
			if err != nil {
				panicGateError(account.Key, "placeFutureOrderGate", err)
				order.Status = model.CarryStatusFail
				order.OrderId = ``
				order.ErrCode = err.Error()
			} else {
				orderResp, _ := json.Marshal(createFuturesOrder)
				util.Notice(`create future order response: %s`, orderResp)
				if createFuturesOrder.IsLiq {
					util.Notice(fmt.Sprintf("warning warning, blow up!!!"))
				}
				order.OrderId = strconv.FormatInt(createFuturesOrder.Id, 10)
				order.OrderTime = time.Unix(int64(createFuturesOrder.CreateTime), 0)
				if orderPriceStr != createFuturesOrder.Price {
					util.Notice(fmt.Sprintf(`diff future price req %s resp %s`, orderPriceStr, createFuturesOrder.Price))
				}
				order.Price, _ = strconv.ParseFloat(createFuturesOrder.Price, 64)
				_, order.Amount = model.ParseRealAmount(model.Gate, order.Symbol, float64(createFuturesOrder.Size))
				order.Status = model.CarryStatusWorking
			}
		}
	}
}

func getMaxLoanGate(symbol string) (success bool, maxLoan float64) {
	v, _ := util.LoadSyncMap(model.MarketInfos, model.Gate, symbol)
	_, tickRelated := model.AppEnvironment.GetBidAsk(symbol, model.Gate)
	if tickRelated != nil && v != nil {
		maxLoan = v.(*model.MarketInfo).BorrowUsdtMax / tickRelated.Bids[0].Price
	}
	return true, maxLoan
}

func getFundingRateGate(key, secret, symbol string) (fundingRate *model.FundingRate) {
	client, ctx := getClientGate(key, secret)
	_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	contract, _, err := client.FuturesApi.GetFuturesContract(ctx, `usdt`, dialectSymbol)
	if err != nil {
		panicGateError(key, "getFundingRateGate", err)
		return nil
	}
	rate, _ := strconv.ParseFloat(contract.FundingRate, 64)
	return &model.FundingRate{
		Rate:       rate,
		UpdateTime: time.Now(),
		ExpireTime: int64(contract.FundingNextApply)}
}

// SetGateBidAsk 用于处理永续合约买卖一不准确（现货无需，因为订阅方式不同）
func SetGateBidAsk(key, secret, symbol string) {
	client, ctx := getClientGate(key, secret)
	_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	orderBook, _, err := client.FuturesApi.ListFuturesOrderBook(ctx, `usdt`, dialectSymbol,
		&gateApi.ListFuturesOrderBookOpts{Limit: optional.NewInt32(1)})
	if err != nil {
		panicGateError(key, "setFutureTicker", err)
	}
	result, oldBidAsk := model.AppEnvironment.GetBidAsk(symbol, model.Gate)
	if result && float64(oldBidAsk.Ts) > orderBook.Update*1000 || orderBook.Bids == nil || len(orderBook.Bids) < 1 ||
		orderBook.Asks == nil || len(orderBook.Asks) < 1 {
		return
	}
	bidPrice, _ := strconv.ParseFloat(orderBook.Bids[0].P, 64)
	_, bidAmount := model.ParseRealAmount(model.Gate, symbol, float64(orderBook.Bids[0].S))
	askPrice, _ := strconv.ParseFloat(orderBook.Asks[0].P, 64)
	_, askAmount := model.ParseRealAmount(model.Gate, symbol, float64(orderBook.Asks[0].S))
	bidAsk := model.BidAsk{Ts: int(orderBook.Update * 1000),
		TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond)),
		Bids:       []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.Gate, Symbol: symbol}},
		Asks:       []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.Gate, Symbol: symbol}}}
	model.AppEnvironment.SetBidAsk(symbol, model.Gate, &bidAsk)
}

func queryOrderGate(key, secret string, order *model.Order) {
	client, ctx := getClientGate(key, secret)
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, order.Symbol)
	if success && marketType == model.MarketTypePerp {
		orderFuture, _, err := client.FuturesApi.GetFuturesOrder(ctx, `usdt`, order.OrderId)
		if err != nil {
			panicGateError(key, "GetFuturesOrder", err)
			return
		}
		order.DealPrice, _ = strconv.ParseFloat(orderFuture.FillPrice, 64)
		if orderFuture.Status == `open` {
			order.Status = model.CarryStatusWorking
		} else if orderFuture.Status == `finished` {
			switch orderFuture.FinishAs {
			case `filled`:
				order.Status = model.CarryStatusSuccess
			case `cancelled`, `liquidated`, `ioc`, `auto_deleveraged`, `reduce_only`, `position_closed`, `reduce_out`:
				order.Status = model.CarryStatusFail
			}
		}
		order.OrderTime = time.Unix(int64(orderFuture.CreateTime), 0)
		order.OrderUpdateTime = time.Unix(int64(orderFuture.FinishTime), 0)
		_, order.DealAmount = model.ParseRealAmount(order.Market, order.Symbol, float64(orderFuture.Size-orderFuture.Left))
		util.SocketInfo(`%s %s %s query result:%s %f %v`,
			order.Market, order.Symbol, order.OrderId, order.Status, order.DealAmount, orderFuture)
	} else if success && marketType == model.MarketTypeSpot {
		orderSpot, _, err := client.SpotApi.GetOrder(ctx, order.OrderId, dialectSymbol, nil)
		if err != nil {
			panicGateError(key, "GetSpotOrder", err)
			return
		}
		intCreateTime, _ := strconv.ParseInt(orderSpot.CreateTime, 10, 64)
		intUpdateTime, _ := strconv.ParseInt(orderSpot.UpdateTime, 10, 64)
		order.OrderTime = time.Unix(intCreateTime, 0)
		order.OrderUpdateTime = time.Unix(intUpdateTime, 0)
		order.DealAmount, _ = strconv.ParseFloat(orderSpot.FilledTotal, 64)
		order.DealPrice, _ = strconv.ParseFloat(orderSpot.Price, 64)
		if order.DealPrice > 0 {
			order.DealAmount = order.DealAmount / order.DealPrice // FilledTotal是成交钱数，需要除以价格
		}
		switch orderSpot.Status {
		case `open`:
			order.Status = model.CarryStatusWorking
		case `closed`:
			order.Status = model.CarryStatusSuccess
		case `cancelled`:
			order.Status = model.CarryStatusFail
		}
		util.SocketInfo(`%s %s %s query result:%s %f %v`,
			order.Market, order.Symbol, order.OrderId, order.Status, order.DealAmount, orderSpot)
	}
}

package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"net/http"
	"sort"
	"strconv"
	"time"
)

const wsBybitSpot = `wss://stream.bybit.com/spot/quote/ws/v2`
const wsStepBybitSpot = 20

var bybitSpotSubConnection = make(map[string]*websocket.Conn)
var channelMaintainingBybitSpot = false

func maintainChannelBybitSpot(subscribes []interface{}) {
	if !channelMaintainingBybitSpot {
		channelMaintainingBybitSpot = true
		for true {
			time.Sleep(time.Minute)
			for _, value := range subscribes {
				_, bidAsk := model.AppMarkets.GetBidAsk(value.(string), model.BybitSpot)
				now := time.Now().UnixNano() / int64(time.Millisecond)
				if bidAsk == nil || now-int64(bidAsk.Ts) > 60000 {
					subCmd := fmt.Sprintf(
						`{"topic":"depth","event":"sub","params":{"symbol":"%s","binary":false}}`, value.(string))
					if bybitSpotSubConnection[value.(string)] != nil {
						if err := SendToConnection(model.BybitSpot, bybitSpotSubConnection[value.(string)],
							[]byte(subCmd)); err != nil {
							util.SocketInfo("bybitSpot can not resubscribe " + err.Error())
						}
					} else {
						util.Notice(`bybitSpot can not get connection for %s`, value.(string))
					}
					util.Notice(`send resubscribe %s`, subCmd)
				}
			}
		}
	}
}

var subscribeHandlerBybitSpot = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	//expire := util.GetNowUnixMillion() + 1000
	//toBeSign := fmt.Sprintf(`GET/realtime%d`, expire)
	//account := model.AppConfig.GetAccounts(model.BybitSpot)[0]
	//hash := hmac.New(sha256.New, []byte(account.Secret))
	//hash.Write([]byte(toBeSign))
	//sign := hex.EncodeToString(hash.Sum(nil))
	//authCmd := fmt.Sprintf(`{"op": "auth", "args": ["%s", %d, "%s"]}`, account.Key, expire, sign)
	//if err = SendToConnection(model.BybitSpot, connection, []byte(authCmd)); err != nil {
	//	util.SocketInfo("bybitSpot can not auth " + err.Error())
	//}
	for _, subscribe := range subscribes {
		subscribeMessage := fmt.Sprintf(
			`{"topic":"depth","event":"sub","params":{"symbol":"%s","binary":false}}`, subscribe)
		if err = SendToConnection(model.BybitSpot, connection, []byte(subscribeMessage)); err != nil {
			util.SocketInfo("bybitSpot can not subscribe " + err.Error())
			return err
		}
		bybitSpotSubConnection[subscribe.(string)] = connection
	}
	return err
}

func WsDepthServeBybitSpot(markets *model.Markets, orderHandler OrderHandler) ([]chan struct{}, error) {
	lastPingTime := util.GetNow().Unix()
	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		now := util.GetNow()
		if now.Unix()-lastPingTime > 30 { // ping ws server every 5 seconds
			lastPingTime = util.GetNow().Unix()
			if err := SendToAllConnections(model.BybitSpot, []byte(fmt.Sprintf(`{"ping":%d}`,
				now.UnixNano()/int64(time.Millisecond)))); err != nil {
				util.SocketInfo("bybit server ping client error " + err.Error())
			}
		}
		if len(event) == 0 {
			return
		}
		depthJson, depthErr := util.NewJSON(event)
		if depthJson == nil || depthErr != nil {
			return
		}
		if depthJson.Get(`topic`).MustString() == `depth` {
			data := depthJson.Get(`data`).MustMap()
			symbol, bidAsk := parseTickBybitSpot(data)
			if markets.SetBidAsk(symbol, model.BybitSpot, bidAsk) {
				for function, handler := range model.GetFunctions(model.BybitSpot, symbol) {
					if handler != nil {
						setting := model.GetSetting(function, model.BybitPerp, symbol)
						if setting != nil {
							go handler(setting, bidAsk)
						}
					}
				}
			}
		}
	}
	subscribes := GetWSSubscribes(model.BybitSpot, model.SubscribeDepth)
	bybitSpotSubConnection = make(map[string]*websocket.Conn)
	return WebSocketClient(model.BybitSpot, wsBybitSpot, subscribes, subscribeHandlerBybitSpot, wsHandler,
		orderHandler, wsStepBybitSpot)
}

func parseTickBybitSpot(data map[string]interface{}) (symbol string, bidAsk *model.BidAsk) {
	if data == nil {
		return ``, nil
	}
	bidAsk = &model.BidAsk{TsReceived: int(util.GetNowUnixMillion()), Bids: model.Ticks{}, Asks: model.Ticks{}}
	if data[`s`] != nil {
		symbol = model.GetStandardSymbol(model.BybitSpot, data[`s`].(string))
	}
	if data[`t`] != nil {
		bidAsk.UpdateId, _ = data[`t`].(json.Number).Int64()
	}
	if data[`b`] != nil {
		items := data[`b`].([]interface{})
		for _, item := range items {
			if len(item.([]string)) != 2 {
				continue
			}
			price, _ := strconv.ParseFloat(item.([]string)[0], 64)
			amount, _ := strconv.ParseFloat(item.([]string)[1], 64)
			bidAsk.Bids = append(bidAsk.Bids, model.Tick{
				Side: model.OrderSideBuy, Market: model.BybitSpot, Symbol: symbol, Price: price, Amount: amount})
		}
		sort.Sort(sort.Reverse(bidAsk.Bids))
	}
	if data[`a`] != nil {
		items := data[`a`].([]interface{})
		for _, item := range items {
			if len(item.([]string)) != 2 {
				continue
			}
			price, _ := strconv.ParseFloat(item.([]string)[0], 64)
			amount, _ := strconv.ParseFloat(item.([]string)[1], 64)
			bidAsk.Asks = append(bidAsk.Asks, model.Tick{
				Side: model.OrderSideSell, Market: model.BybitSpot, Symbol: symbol, Price: price, Amount: amount})
		}
		sort.Sort(bidAsk.Asks)
	}
	return
}

func getMarketsBybitSpot(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	response := SignedRequestBybit(key, secret, http.MethodGet, `/spot/v1/symbols`, nil)
	marketInfos = make(map[string]*model.MarketInfo)
	marketJson, err := util.NewJSON(response)
	if err == nil && marketJson.Get(`ret_code`) != nil && marketJson.Get(`ret_code`).MustInt64() == 0 {
		items, _ := marketJson.Get(`result`).Array()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value[`quoteCurrency`] == nil || value[`quoteCurrency`].(string) != `USDT` {
				continue
			}
			marketInfo := &model.MarketInfo{Market: model.BybitSpot}
			if value[`base_currency`] != nil {
				marketInfo.CTCurrency = value[`base_currency`].(string)
				marketInfo.Name = marketInfo.CTCurrency + model.GetSpotTail(model.BybitSpot)
				marketInfos[marketInfo.Name] = marketInfo
			}
			if value[`basePrecision`] != nil {
				marketInfo.SizeIncrement, _ = value[`basePrecision`].(json.Number).Float64()
			}
			if value[`minPricePrecision`] != nil {
				marketInfo.PriceIncrement, _ = strconv.ParseFloat(value[`minPricePrecision`].(string), 64)
				marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
			}
			if value[`minTradeQuantity`] != nil {
				marketInfo.SizeMin, _ = strconv.ParseFloat(value[`minTradeQuantity`].(string), 64)
			}
			if value[`maxTradeQuantity`] != nil {
				marketInfo.SizeMax, _ = strconv.ParseFloat(value[`maxTradeQuantity`].(string), 64)
			}
		}
	}
	return
}

package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const restBybit = `https://api.bybit.com`
const wsBybitPerp = `wss://stream.bybit.com/realtime_public`
const wsStepBybitPerp = 20

var channelMaintainingBybitPerp = false
var bybitPerpSubConnection = make(map[string]*websocket.Conn)

func maintainChannelBybitPerp(subscribes []interface{}) {
	if !channelMaintainingBybitPerp {
		channelMaintainingBybitPerp = true
		for true {
			time.Sleep(time.Minute)
			for _, value := range subscribes {
				_, bidAsk := model.AppMarkets.GetBidAsk(value.(string), model.BybitPerp)
				now := time.Now().UnixNano() / int64(time.Millisecond)
				if bidAsk == nil || now-int64(bidAsk.Ts) > 60000 {
					subCmd := fmt.Sprintf(`{"op": "subscribe", "args": ["orderBookL2_25.%s"]}`, value.(string))
					if bybitPerpSubConnection[value.(string)] != nil {
						if err := SendToConnection(model.BybitPerp, bybitPerpSubConnection[value.(string)],
							[]byte(subCmd)); err != nil {
							util.SocketInfo("bybitPerp can not resubscribe " + err.Error())
						}
					} else {
						util.Notice(`bybitPerp can not get connection for %s`, value.(string))
					}
					util.Notice(`send resubscribe %s`, subCmd)
				}
			}
		}
	}
}

var subscribeHandlerBybitPerp = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	//expire := util.GetNowUnixMillion() + 1000
	//toBeSign := fmt.Sprintf(`GET/realtime%d`, expire)
	//account := model.AppConfig.GetAccounts(model.BybitPerp)[0]
	//hash := hmac.New(sha256.New, []byte(account.Secret))
	//hash.Write([]byte(toBeSign))
	//sign := hex.EncodeToString(hash.Sum(nil))
	//authCmd := fmt.Sprintf(`{"op": "auth", "args": ["%s", %d, "%s"]}`, account.Key, expire, sign)
	//if err = SendToConnection(model.BybitPerp, connection, []byte(authCmd)); err != nil {
	//	util.SocketInfo("bybit can not auth " + err.Error())
	//}
	for _, subscribe := range subscribes {
		subscribeMap := make(map[string]interface{})
		subscribeMap[`op`] = `subscribe`
		subscribeMap[`args`] = subscribe
		subscribeMessage := util.JsonEncodeToByte(subscribeMap)
		if err = SendToConnection(model.BybitPerp, connection, subscribeMessage); err != nil {
			util.SocketInfo("bybitPerp can not subscribe " + err.Error())
			return err
		}
		bybitPerpSubConnection[subscribe.(string)] = connection
	}
	return err
}

func WsDepthServeBybitPerp(markets *model.Markets, orderHandler OrderHandler) ([]chan struct{}, error) {
	lastPingTime := util.GetNow().Unix()
	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		now := util.GetNow()
		if now.Unix()-lastPingTime > 30 { // ping ws server every 5 seconds
			lastPingTime = util.GetNow().Unix()
			if err := SendToAllConnections(model.BybitPerp, []byte(`{"op":"ping"}`)); err != nil {
				util.SocketInfo("bybit server ping client error " + err.Error())
			}
		}
		if len(event) == 0 {
			return
		}
		depthJson, depthErr := util.NewJSON(event)
		if depthJson == nil {
			return
		}
		topic := depthJson.Get(`topic`).MustString()
		ts := depthJson.Get(`timestamp_e6`).MustInt64()
		if depthErr != nil {
			util.SocketInfo(`bybit parse err` + string(event))
			return
		}
		if strings.Contains(topic, `orderBookL2_25.`) {
			symbol := model.GetStandardSymbol(model.BybitPerp, topic[strings.LastIndex(topic, `.`)+1:])
			handleOrderBookBybitPerp(markets, symbol, ts, depthJson)
		} else if topic == `position` {
		}
	}
	subscribes := GetWSSubscribes(model.BybitPerp, model.SubscribeDepth)
	bybitPerpSubConnection = make(map[string]*websocket.Conn)
	return WebSocketClient(model.BybitPerp, wsBybitPerp, subscribes, subscribeHandlerBybitPerp, wsHandler, orderHandler, wsStepBybitPerp)
}

func parseTickBybitPerp(item map[string]interface{}) (tick *model.Tick) {
	if item == nil {
		return nil
	}
	tick = &model.Tick{Market: model.BybitPerp}
	if item[`symbol`] != nil {
		tick.Symbol = model.GetStandardSymbol(model.BybitPerp, item[`symbol`].(string))
	}
	if item[`id`] != nil {
		tick.Id = item[`id`].(string)
	}
	if item[`size`] != nil {
		amount, err := item[`size`].(json.Number).Float64()
		if err == nil {
			tick.Amount = amount
		}
	}
	if item[`price`] != nil {
		price, err := strconv.ParseFloat(item[`price`].(string), 64)
		if err == nil {
			tick.Price = price
		}
	}
	if item[`side`] != nil {
		tick.Side = strings.ToLower(item[`side`].(string))
	}
	return tick
}

func handleOrderBookBybitPerp(markets *model.Markets, symbol string, ts int64, response *simplejson.Json) {
	if response == nil {
		return
	}
	action := response.Get(`type`).MustString()
	var bidAsk *model.BidAsk
	if action == `snapshot` {
		bidAsk = &model.BidAsk{}
		bidAsk.Bids = make([]model.Tick, 0)
		bidAsk.Asks = make([]model.Tick, 0)
		data, err := response.GetPath(`data`, `order_book`).Array()
		if err != nil {
			return
		}
		for _, value := range data {
			tick := parseTickBybitPerp(value.(map[string]interface{}))
			if tick.Side == model.OrderSideSell {
				bidAsk.Asks = append(bidAsk.Asks, *tick)
			} else if tick.Side == model.OrderSideBuy {
				bidAsk.Bids = append(bidAsk.Bids, *tick)
			}
		}
	} else if action == `delta` {
		_, bidAsk = markets.CopyBidAsk(symbol, model.BybitPerp)
		data := response.Get(`data`)
		if bidAsk == nil || data == nil {
			return
		}
		arrayDelete, errDelete := data.Get(`delete`).Array()
		arrayUpdate, errUpdate := data.Get(`update`).Array()
		arrayInsert, errInsert := data.Get(`insert`).Array()
		if errDelete != nil || errInsert != nil || errUpdate != nil {
			return
		}
		for _, value := range arrayInsert {
			tick := parseTickBybitPerp(value.(map[string]interface{}))
			if tick.Side == model.OrderSideBuy {
				bidAsk.Bids = append(bidAsk.Bids, *tick)
			}
			if tick.Side == model.OrderSideSell {
				bidAsk.Asks = append(bidAsk.Asks, *tick)
			}
			//util.SocketInfo(fmt.Sprintf(`+++++ %s %f %f`, tick.Side, tick.Price, tick.Amount))
		}
		for _, value := range arrayUpdate {
			tick := parseTickBybitPerp(value.(map[string]interface{}))
			//util.SocketInfo(fmt.Sprintf(`update %s %f %f`, tick.Side, tick.Price, tick.Amount))
			if tick.Side == model.OrderSideBuy {
				for key, bid := range bidAsk.Bids {
					if tick.Id == bid.Id {
						bidAsk.Bids[key] = *tick
					}
				}
			}
			if tick.Side == model.OrderSideSell {
				for key, ask := range bidAsk.Asks {
					if tick.Id == ask.Id {
						bidAsk.Asks[key] = *tick
					}
				}
			}
		}
		deleteMap := make(map[string]*model.Tick)
		for _, value := range arrayDelete {
			tick := parseTickBybitPerp(value.(map[string]interface{}))
			//util.SocketInfo(fmt.Sprintf(`----- %s %f %f`, tick.Side, tick.Price, tick.Amount))
			deleteMap[tick.Id] = tick
		}
		bidNew := make([]model.Tick, 0)
		askNew := make([]model.Tick, 0)
		for _, value := range bidAsk.Bids {
			if deleteMap[value.Id] == nil || deleteMap[value.Id].Side == model.OrderSideSell {
				bidNew = append(bidNew, value)
			}
		}
		for _, value := range bidAsk.Asks {
			if deleteMap[value.Id] == nil || deleteMap[value.Id].Side == model.OrderSideBuy {
				askNew = append(askNew, value)
			}
		}
		bidAsk.Bids = bidNew
		bidAsk.Asks = askNew
	}
	if bidAsk != nil {
		bidAsk.Ts = int(ts / 1000)
		bidAsk.TsReceived = int(util.GetNowUnixMillion())
		sort.Sort(bidAsk.Asks)
		sort.Sort(sort.Reverse(bidAsk.Bids))
		//util.SocketInfo(markets.ToStringBidAsk(bidAsk))
		if markets.SetBidAsk(symbol, model.BybitPerp, bidAsk) {
			for function, handler := range model.GetFunctions(model.BybitPerp, symbol) {
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

func getMarketsBybitPerp(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	response := SignedRequestBybit(key, secret, http.MethodGet, `/v2/public/symbols`, nil)
	marketInfos = make(map[string]*model.MarketInfo)
	marketJson, err := util.NewJSON(response)
	if err == nil && marketJson.Get(`ret_code`) != nil && marketJson.Get(`ret_code`).MustInt64() == 0 {
		items, _ := marketJson.Get(`result`).Array()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value[`status`] == nil || !strings.EqualFold(value[`status`].(string), `Trading`) || value[`quote_currency`] == nil ||
				value[`quote_currency`].(string) != `USDT` {
				continue
			}
			marketInfo := &model.MarketInfo{Market: model.BybitPerp}
			if value[`base_currency`] != nil {
				marketInfo.CTCurrency = value[`base_currency`].(string)
				marketInfo.Name = marketInfo.CTCurrency + model.GetPerpTail(model.BybitPerp)
				marketInfos[marketInfo.Name] = marketInfo
			}
			if value[`price_scale`] != nil {
				decimal, _ := value[`price_scale`].(json.Number).Int64()
				marketInfo.PriceDecimal = int(decimal)
			}
			if value[`price_filter`] != nil {
				priceFilter := value[`price_filter`].(map[string]interface{})
				if priceFilter != nil && priceFilter[`tick_size`] != nil {
					marketInfo.PriceIncrement, _ = strconv.ParseFloat(priceFilter[`tick_size`].(string), 64)
				}
			}
			if value[`lot_size_filter`] != nil {
				sizeFilter := value[`lot_size_filter`].(map[string]interface{})
				if sizeFilter != nil && sizeFilter[`max_trading_qty`] != nil {
					marketInfo.SizeMax, _ = sizeFilter[`max_trading_qty`].(json.Number).Float64()
				}
				if sizeFilter != nil && sizeFilter[`min_trading_qty`] != nil {
					marketInfo.SizeMin, _ = sizeFilter[`min_trading_qty`].(json.Number).Float64()
				}
				if sizeFilter != nil && sizeFilter[`qty_step`] != nil {
					marketInfo.SizeIncrement, _ = sizeFilter[`qty_step`].(json.Number).Float64()
				}
			}
		}
	}
	return
}

func SignedRequestBybit(key, secret, method, path string, body map[string]interface{}) []byte {
	if body == nil {
		body = make(map[string]interface{})
	}
	body[`api_key`] = key
	body[`timestamp`] = strconv.FormatInt(util.GetNowUnixMillion(), 10)
	uri := restBybit + path
	paramStr := util.ComposeParams(body)
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(paramStr))
	sign := hex.EncodeToString(hash.Sum(nil))
	body[`sign`] = sign
	paramStr = util.ComposeParams(body)
	//headers := map[string]string{"Content-Type": "application/json"}
	headers := map[string]string{`api_key`: key, `sign`: sign, "Content-Type": "application/json"}
	if method == `GET` {
		uri = uri + `?` + paramStr
	}
	//responseBody, _ := util.HttpRequest(method, uri, string(util.JsonEncodeToByte(body)), headers, 60)
	responseBody, _ := util.HttpRequest(method, uri, string(util.JsonEncodeToByte(body)), headers, 60)
	return responseBody
}

func cancelOrdersBybitPerp(key, secret, symbol string) bool {
	postData := make(map[string]interface{})
	path := ``
	method := ``
	coinPerp, isPerp := model.IsPerp(model.BybitPerp, symbol)
	coinSpot, isSpot := model.IsSpot(model.BybitSpot, symbol)
	if isSpot {
		path = `/spot/order/batch-cancel`
		method = http.MethodDelete
		postData[`symbol`] = coinSpot + `USDT`
	} else if isPerp {
		path = `/private/linear/order/cancel-all`
		method = http.MethodPost
		postData[`symbol`] = coinPerp + `USDT`
	} else {
		return false
	}
	response := SignedRequestBybit(key, secret, method, path, postData)
	cancelJson, err := util.NewJSON(response)
	if err == nil {
		if cancelJson.Get(`ret_code`).MustInt() == 0 {
			return true
		}
	}
	return false
}

func cancelOrderBybitPerp(key, secret, symbol, orderId string) (result bool, errCode, msg string, order *model.Order) {
	postData := make(map[string]interface{})
	postData[`order_id`] = orderId
	postData[`symbol`] = model.GetDialectSymbol(model.BybitPerp, symbol)
	response := SignedRequestBybit(key, secret, `POST`, `/v2/private/order/cancel`, postData)
	orderJson, err := util.NewJSON(response)
	result = false
	if err == nil {
		retCode := orderJson.Get(`ret_code`).MustInt64()
		if retCode == 0 {
			result = true
		}
		errCode = strconv.FormatInt(retCode, 10)
		msg = orderJson.Get(`ret_msg`).MustString()
		if orderJson.Get(`result`) != nil {
			item, _ := orderJson.Get(`result`).Map()
			if item != nil {
				order = &model.Order{}
				parseOrderBybitPerp(order, item)
			}
		}
		return
	}
	return false, ``, ``, nil
}

func queryOrderBybitPerp(key, secret, symbol, orderId string) (orders []*model.Order) {
	orders = make([]*model.Order, 0)
	postData := make(map[string]interface{})
	symbol = model.GetDialectSymbol(model.BybitPerp, symbol)
	postData[`symbol`] = model.GetDialectSymbol(model.BybitPerp, symbol)
	postData[`order_id`] = orderId
	response := SignedRequestBybit(key, secret, `GET`, `/open-api/order/list`, postData)
	util.SocketInfo(`query orders: ` + string(response))
	orderJson, err := util.NewJSON(response)
	if err == nil {
		orderJson = orderJson.GetPath(`result`, `data`)
		if orderJson == nil {
			return
		}
		orderArray, _ := orderJson.Array()
		for _, data := range orderArray {
			order := &model.Order{Market: model.BybitPerp}
			parseOrderBybitPerp(order, data.(map[string]interface{}))
			if order.OrderId != `` {
				orders = append(orders, order)
			}
		}
	}
	return
}

// timeInForce 有效选项:GoodTillCancel, ImmediateOrCancel, FillOrKill,PostOnly
func placeOrderBybitPerp(order *model.Order, key, secret, orderSide, orderType, timeInForce, symbol string, price, amount float64) {
	postData := make(map[string]interface{})
	symbol = model.GetDialectSymbol(model.BybitPerp, symbol)
	postData["side"] = strings.ToUpper(orderSide[0:1]) + orderSide[1:]
	postData["order_type"] = strings.ToUpper(orderType[0:1]) + orderType[1:]
	if orderType != model.OrderTypeMarket && orderType != model.OrderTypeStop {
		postData[`price`] = fmt.Sprintf(`%f`, price)
	}
	if timeInForce == `` {
		timeInForce = `GTC`
	}
	postData[`reduce_only`] = false
	postData[`close_on_trigger`] = false
	postData["symbol"] = symbol
	postData["qty"] = fmt.Sprintf(`%f`, amount)
	postData[`time_in_force`] = timeInForce
	response := SignedRequestBybit(key, secret, `POST`, `/private/linear/order/create`, postData)
	util.Notice(`place bybit` + string(response))
	orderJson, err := util.NewJSON(response)
	if err == nil {
		orderJson = orderJson.Get(`result`)
		if orderJson != nil {
			item, err := orderJson.Map()
			if err == nil {
				parseOrderBybitPerp(order, item)
			}
		}
	}
	return
}

func getFundingRateBybitPerp(key, secret, symbol string) (fundingRate float64, expire int64) {
	postData := make(map[string]interface{})
	symbol = model.GetDialectSymbol(model.BybitPerp, symbol)
	postData[`symbol`] = symbol
	response := SignedRequestBybit(key, secret, `GET`,
		`/private/linear/funding/predicted-funding`, postData)
	instrumentJson, err := util.NewJSON(response)
	if err == nil {
		retCode := instrumentJson.Get(`ret_code`).MustFloat64()
		if retCode != 0 {
			return 0, 0
		}
		fundingRate = instrumentJson.GetPath(`result`, `predicted_funding_rate`).MustFloat64()
	}
	now := util.GetNow()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	expire = (today.Unix() + 86400 - now.Unix()) % 28800
	return
}

func parseOrderBybitPerp(order *model.Order, item map[string]interface{}) {
	if order == nil {
		return
	}
	if item[`order_id`] != nil {
		order.OrderId = item[`order_id`].(string)
	}
	if item[`symbol`] != nil {
		order.Symbol = model.GetStandardSymbol(model.BybitPerp, item[`symbol`].(string))
	}
	if item[`side`] != nil {
		order.OrderSide = strings.ToLower(item[`side`].(string))
	}
	if item[`order_type`] != nil {
		order.OrderType = strings.ToLower(item[`order_type`].(string))
	}
	if item[`qty`] != nil {
		order.Amount, _ = item[`qty`].(json.Number).Float64()
	}
	if item[`price`] != nil {
		order.Price, _ = item[`price`].(json.Number).Float64()
	}
	if item[`last_exec_price`] != nil {
		order.DealPrice, _ = item[`last_exec_price`].(json.Number).Float64()
	}
	if item[`cum_exec_qty`] != nil {
		order.DealAmount, _ = item[`cum_exec_qty`].(json.Number).Float64()
	}
	if item[`cum_exec_fee`] != nil {
		order.Fee, _ = item[`cum_exec_fee`].(json.Number).Float64()
	}
	if item[`created_at`] != nil {
		order.OrderTime, _ = time.Parse(time.RFC3339, item[`created_at`].(string))
	}
	if item[`updated_at`] != nil {
		order.OrderUpdateTime, _ = time.Parse(time.RFC3339, item[`updated_at`].(string))
	}
	if item[`order_status`] != nil {
		order.Status = model.GetOrderStatus(model.BybitPerp, item[`order_status`].(string))
	}
	if order.Status != model.CarryStatusSuccess && order.Status != model.CarryStatusFail {
		order.Status = model.CarryStatusWorking
	}
	if order.DealAmount > 0 && order.DealPrice == 0 {
		order.DealPrice = order.Price
	}
	return
}

// GetWalletBybitPerp
func _(key, secret string) (balance float64, msg string) {
	postData := make(map[string]interface{})
	postData[`limit`] = `50`
	response := SignedRequestBybit(key, secret, `GET`,
		`/open-api/wallet/fund/records`, postData)
	util.Notice(`bybit wallet: ` + string(response))
	dataJson, err := util.NewJSON(response)
	if err == nil {
		data := dataJson.GetPath(`result`, `data`).MustArray()
		for _, item := range data {
			itemMap := item.(map[string]interface{})
			itemType := itemMap[`type`]
			if itemType == nil {
				continue
			}
			if balance == 0 && itemMap[`wallet_balance`] != nil {
				balance, _ = strconv.ParseFloat(itemMap[`wallet_balance`].(string), 64)
			}
			if itemType == `Withdraw` || itemType == `Deposit` {
				amount, _ := strconv.ParseFloat(itemMap[`amount`].(string), 64)
				msg += fmt.Sprintf("%s %s %f\n", itemMap[`exec_time`], itemType, amount)
			}
		}
	}
	return
}

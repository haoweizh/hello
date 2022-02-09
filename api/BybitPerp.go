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
	"math"
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
			time.Sleep(time.Minute * 5)
			for _, value := range subscribes {
				_, _, coin := model.GetCoinFromDialect(model.BybitPerp, value.(string))
				standardSymbol := coin + model.UniStandardTail[model.MarketTypePerp]
				_, bidAsk := model.AppMarkets.GetBidAsk(standardSymbol, model.BybitPerp)
				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 120000 {
					subCmd := fmt.Sprintf(`{"op": "subscribe", "args": ["orderBookL2_25.%s"]}`, value.(string))
					if bidAsk != nil {
						util.Notice(`maintain bybitperp timeout %s %s %d`,
							standardSymbol, time.Now().UnixMilli()-int64(bidAsk.Ts), bidAsk.Ts)
					}
					if bybitPerpSubConnection[standardSymbol] != nil {
						if err := SendToConnection(model.BybitPerp, bybitPerpSubConnection[standardSymbol],
							[]byte(subCmd)); err != nil {
							util.SocketInfo("bybitPerp can not resubscribe " + err.Error())
						}
					} else {
						util.Notice(`bybitPerp can not get connection for %s`, standardSymbol)
					}
					util.Notice(`send resubscribe %s %s`, model.BybitPerp, subCmd)
				}
				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 180000 {
					SetRequireReset(model.BybitPerp, true)
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
	for _, value := range subscribes {
		subCmd := fmt.Sprintf(`{"op": "subscribe", "args": ["orderBookL2_25.%s"]}`, value.(string))
		if err := SendToConnection(model.BybitPerp, connection, []byte(subCmd)); err != nil {
			util.SocketInfo("bybitPerp can not subscribe " + err.Error())
		}
		_, _, coin := model.GetCoinFromDialect(model.BybitPerp, value.(string))
		standardSymbol := coin + model.UniStandardTail[model.MarketTypePerp]
		bybitPerpSubConnection[standardSymbol] = connection
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
		str := depthJson.Get(`timestamp_e6`).MustString()
		if depthErr != nil {
			util.SocketInfo(`bybit parse err` + string(event))
			return
		}
		if strings.Contains(topic, `orderBookL2_25.`) {
			success, _, coin := model.GetCoinFromDialect(model.BybitPerp, topic[strings.LastIndex(topic, `.`)+1:])
			symbol := coin + model.UniStandardTail[model.MarketTypePerp]
			if success && str != `` {
				ts, _ := strconv.ParseInt(str, 10, 64)
				handleOrderBookBybitPerp(markets, symbol, ts/1000, depthJson)
			}
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
		_, _, coin := model.GetCoinFromDialect(model.BybitPerp, item[`symbol`].(string))
		tick.Symbol = coin + model.UniStandardTail[model.MarketTypePerp]
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
		bidAsk.Ts = int(ts)
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
	response := SignedRequestBybitPerp(key, secret, http.MethodGet, `/v2/public/symbols`, nil)
	marketInfos = make(map[string]*model.MarketInfo)
	marketJson, err := util.NewJSON(response)
	if err != nil || marketJson.Get(`ret_code`) == nil || marketJson.Get(`ret_code`).MustInt() != 0 {
		time.Sleep(time.Second * 2)
		getMarketsBybitPerp(key, secret)
	} else {
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
				marketInfo.Name = marketInfo.CTCurrency + model.UniStandardTail[model.MarketTypePerp]
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

func SignedRequestBybitPerp(key, secret, method, path string, body map[string]interface{}) []byte {
	if body == nil {
		body = make(map[string]interface{})
	}
	body[`api_key`] = key
	body[`timestamp`] = strconv.FormatInt(util.GetNowUnixMillion(), 10)
	if body[`symbol`] != nil && len(body[`symbol`].(string)) > 0 {
		_, _, _, dialectSymbol := model.GetFromStandard(model.BybitPerp, body[`symbol`].(string))
		body[`symbol`] = dialectSymbol
	}
	uri := restBybit + path
	paramStr := util.ComposeParams(body)
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(paramStr))
	sign := hex.EncodeToString(hash.Sum(nil))
	body[`sign`] = sign
	paramStr = util.ComposeParams(body)
	headers := map[string]string{`api_key`: key, `sign`: sign, "Content-Type": "application/json"}
	if method == `GET` {
		uri = uri + `?` + paramStr
	}
	responseBody, _ := util.HttpRequest(method, uri, string(util.JsonEncodeToByte(body)), headers, 60)
	util.SocketInfo(fmt.Sprintf(`bybitPerp key %s request %s %s body %v return %s`,
		key, uri, method, body, string(responseBody)))
	return responseBody
}

func cancelOrdersBybitPerp(key, secret, symbol string) bool {
	postData := make(map[string]interface{})
	path := `/private/linear/order/cancel-all`
	method := http.MethodPost
	postData[`symbol`] = symbol
	response := SignedRequestBybitPerp(key, secret, method, path, postData)
	cancelJson, err := util.NewJSON(response)
	if err == nil {
		if cancelJson.Get(`ret_code`).MustInt() == 0 {
			return true
		}
	}
	return false
}

func cancelOrderBybitPerp(key, secret, symbol, orderId string) (result bool, errCode, msg string) {
	postData := map[string]interface{}{`order_id`: orderId, `symbol`: symbol}
	response := SignedRequestBybitPerp(key, secret, `POST`, `/private/linear/order/cancel`, postData)
	orderJson, err := util.NewJSON(response)
	result = false
	if err == nil {
		retCode := orderJson.Get(`ret_code`).MustInt64()
		if retCode == 0 {
			result = true
		}
		errCode = strconv.FormatInt(retCode, 10)
		msg = orderJson.Get(`ret_msg`).MustString()
		//if orderJson.Get(`result`) != nil {
		//	item, _ := orderJson.Get(`result`).Map()
		//	if item != nil {
		//		parseOrderBybitPerp(order, item)
		//	}
		//}
		return
	}
	return false, ``, ``
}

func queryOrderBybitPerp(key, secret, symbol, orderId string) (order *model.Order) {
	postData := map[string]interface{}{`symbol`: symbol, `order_id`: orderId}
	response := SignedRequestBybitPerp(key, secret, `GET`, `/private/linear/order/list`, postData)
	orderJson, err := util.NewJSON(response)
	if err == nil {
		orderJson = orderJson.GetPath(`result`, `data`)
		if orderJson == nil {
			return
		}
		orderArray, _ := orderJson.Array()
		for _, data := range orderArray {
			order = &model.Order{Market: model.BybitSpot, Status: model.CarryStatusFail}
			parseOrderBybitPerp(order, data.(map[string]interface{}))
			if order.OrderId == orderId {
				return order
			}
		}
	}
	return nil
}

// timeInForce 有效选项:GoodTillCancel, ImmediateOrCancel, FillOrKill,PostOnly
func placeOrderBybitPerp(order *model.Order, key, secret, orderSide, orderType, timeInForce, symbol string, price, amount float64) {
	postData := make(map[string]interface{})
	postData["symbol"] = symbol
	postData["side"] = strings.ToUpper(orderSide[0:1]) + orderSide[1:]
	postData["order_type"] = strings.ToUpper(orderType[0:1]) + orderType[1:]
	postData[`position_idx`] = 0
	if orderType != model.OrderTypeMarket && orderType != model.OrderTypeStop {
		formattedPrice, decimal := model.FormatPrice(model.BybitPerp, symbol, orderSide, price)
		postData[`price`] = util.CutTailZero(strconv.FormatFloat(formattedPrice, 'f', decimal, 64))
	}
	if timeInForce == `` {
		timeInForce = `GoodTillCancel`
	}
	postData[`time_in_force`] = timeInForce
	postData[`reduce_only`] = false
	postData[`close_on_trigger`] = false
	postData["qty"] = fmt.Sprintf(`%f`, amount)
	response := SignedRequestBybitPerp(key, secret, `POST`, `/private/linear/order/create`, postData)
	orderJson, err := util.NewJSON(response)
	if err == nil {
		orderJson = orderJson.Get(`result`)
		if orderJson != nil {
			parseOrderBybitPerp(order, orderJson.MustMap())
		}
	}
	return
}

func setSettingsBybitPerp(key, secret, symbol string) (singleMode, crossPos bool) {
	postData := map[string]interface{}{`symbol`: symbol, `mode`: `MergedSingle`}
	response := SignedRequestBybitPerp(key, secret, http.MethodPost, `/private/linear/position/switch-mode`, postData)
	setJson, err := util.NewJSON(response)
	if err == nil && setJson != nil && setJson.Get(`ret_code`).MustInt() == 0 {
		singleMode = true
	} else {
		util.Notice(fmt.Sprintf(`fail to set bybitPerp %s pos mode to single`, symbol))
	}
	postData = map[string]interface{}{`symbol`: symbol, `is_isolated`: false, `buy_leverage`: 5, `sell_leverage`: 5}
	response = SignedRequestBybitPerp(key, secret, http.MethodPost, `/private/linear/position/switch-isolated`, postData)
	if err == nil && setJson != nil && setJson.Get(`ret_code`).MustInt() == 0 {
		crossPos = true
	} else {
		util.Notice(fmt.Sprintf(`fail to set bybitPerp %s pos mode to cross`, symbol))
	}
	return
}

func getPositionsBybitPerp(key, secret string) (success bool, positions []*model.Position, accountValue, available float64) {
	accountValue, available = getWalletBybitPerp(key, secret)
	response := SignedRequestBybitPerp(key, secret, http.MethodGet, `/private/linear/position/list`, nil)
	posJson, err := util.NewJSON(response)
	if err != nil || posJson == nil || posJson.Get(`ret_code`).MustInt() != 0 {
		util.SocketInfo(`fail to get bybitPerp positions`)
		time.Sleep(time.Second * 2)
		return getPositionsBybitPerp(key, secret)
	} else {
		items := posJson.Get(`result`).MustArray()
		positions = make([]*model.Position, 0)
		success = true
		for _, item := range items {
			value := item.(map[string]interface{})
			if value[`is_valid`] == nil || value[`data`] == nil {
				continue
			} else if !value[`is_valid`].(bool) {
				continue
			}
			value = value[`data`].(map[string]interface{})
			position := &model.Position{Market: model.BybitPerp}
			if value[`symbol`] != nil {
				_, _, coin := model.GetCoinFromDialect(model.BybitPerp, value[`symbol`].(string))
				position.Currency = coin + model.UniStandardTail[model.MarketTypePerp]
			}
			if value[`side`] != nil {
				position.Direction = strings.ToLower(value[`side`].(string))
			}
			if value[`entry_price`] != nil {
				position.EntryPrice, _ = value[`entry_price`].(json.Number).Float64()
			}
			if value[`liq_price`] != nil {
				position.LiquidationPrice, _ = value[`liq_price`].(json.Number).Float64()
			}
			if value[`leverage`] != nil {
				position.LeverRate, _ = value[`leverage`].(json.Number).Int64()
			}
			if value[`position_margin`] != nil {
				position.Margin, _ = value[`position_margin`].(json.Number).Float64()
			}
			if value[`bust_price`] != nil {
				position.BankruptcyPrice, _ = value[`bust_price`].(json.Number).Float64()
			}
			if value[`cum_realised_pnl`] != nil {
				position.ProfitReal, _ = value[`cum_realised_pnl`].(json.Number).Float64()
			}
			if value[`unrealised_pnl`] != nil {
				position.ProfitUnreal, _ = value[`unrealised_pnl`].(json.Number).Float64()
			}
			if value[`size`] != nil {
				position.Holding, _ = value[`size`].(json.Number).Float64()
				if position.Direction == model.OrderSideSell {
					position.Holding = -1 * math.Abs(position.Holding)
				}
			}
			if position.Holding != 0 {
				positions = append(positions, position)
			}
		}
	}
	return
}

func getFundingRateBybitPerp(key, secret, symbol string) (fundingRate *model.FundingRate) {
	postData := map[string]interface{}{`symbol`: symbol}
	response := SignedRequestBybitPerp(key, secret, http.MethodGet,
		`/public/linear/funding/prev-funding-rate`, postData)
	newJson, err := util.NewJSON(response)
	if err == nil {
		retCode := newJson.Get(`ret_code`).MustFloat64()
		if retCode != 0 {
			return nil
		}
		now := time.Now().Unix()
		rate := newJson.GetPath(`result`, `funding_rate`).MustFloat64()
		expireTime, _ := time.Parse(time.RFC3339, newJson.GetPath(`result`, `funding_rate_timestamp`).MustString())
		expire := expireTime.Unix()
		if expire%28800 == 0 {
			expire = expireTime.Unix() + 28800
		} else {
			expire = expireTime.Unix() + 3600
		}
		return &model.FundingRate{Rate: rate, ExpireTime: expire, UpdateTime: now}
	}
	return
}

func parseOrderBybitPerp(order *model.Order, item map[string]interface{}) {
	if item[`order_id`] != nil {
		order.OrderId = item[`order_id`].(string)
	}
	if item[`symbol`] != nil {
		_, _, coin := model.GetCoinFromDialect(model.BybitPerp, item[`symbol`].(string))
		order.Symbol = coin + model.UniStandardTail[model.MarketTypePerp]
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
	if item[`created_time`] != nil {
		order.OrderTime, _ = time.Parse(time.RFC3339, item[`created_time`].(string))
	}
	if item[`updated_time`] != nil {
		order.OrderUpdateTime, _ = time.Parse(time.RFC3339, item[`updated_time`].(string))
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

// 只计算了USDT的价值,忽略了其他保证金币种
func getWalletBybitPerp(key, secret string) (accountValueInU, availableU float64) {
	response := SignedRequestBybitPerp(key, secret, http.MethodGet, `/v2/private/wallet/balance`, nil)
	dataJson, err := util.NewJSON(response)
	if dataJson == nil || err != nil {
		time.Sleep(time.Second * 2)
		util.Notice(`fail to get bybitPerp wallet`)
		return getWalletBybitPerp(key, secret)
	} else {
		assets := dataJson.GetPath(`result`).MustMap()
		if assets == nil {
			return
		}
		usdAsset := assets[`USDT`].(map[string]interface{})
		if usdAsset[`equity`] != nil {
			accountValueInU, _ = usdAsset[`equity`].(json.Number).Float64()
		}
		if usdAsset[`available_balance`] != nil {
			availableU, _ = usdAsset[`available_balance`].(json.Number).Float64()
		}
	}
	return
}

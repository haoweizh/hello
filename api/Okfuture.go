package api

import (
	"bytes"
	"compress/flate"
	"fmt"
	"hello/model"
	"hello/util"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 交割合约API

var subscribeHandlerOKFuture = func(subscribes []interface{}, subType string) (err error) {
	subscribe := ``
	for _, item := range subscribes {
		if item == nil {
			continue
		}
		if subscribe == `` {
			subscribe = fmt.Sprintf(`"%s"`, item.(string))
		} else {
			subscribe = fmt.Sprintf(`%s,"%s"`, subscribe, item.(string))
		}
	}
	if err = sendToWs(
		model.OKFUTURE, []byte(fmt.Sprintf(`{"op": "subscribe", "args": [%s]}`, subscribe))); err != nil {
		util.SocketInfo("okfuture can not subscribe " + subscribe + err.Error())
	}
	return
}

func parseTickByOkFuture(data map[string]interface{}) (bidAsks *model.BidAsk, symbol string) {
	if data[`timestamp`] == nil || data == nil || data[`asks`] == nil || data[`bids`] == nil ||
		data[`instrument_id`] == nil {
		return
	}
	bidAsks = &model.BidAsk{}
	bidAsks.Asks = make([]model.Tick, len(data[`asks`].([]interface{})))
	bidAsks.Bids = make([]model.Tick, len(data[`bids`].([]interface{})))
	symbol = data[`instrument_id`].(string)
	if len(strings.Split(symbol, `-`)) > 1 {
		symbol = strings.ToLower(symbol[0:strings.LastIndex(symbol, `-`)])
	}
	for i, item := range data[`asks`].([]interface{}) {
		value := item.([]interface{})
		if len(value) < 2 {
			continue
		}
		price, _ := strconv.ParseFloat(value[0].(string), 64)
		amount, _ := strconv.ParseFloat(value[1].(string), 64)
		bidAsks.Asks[i] = model.Tick{Price: price, Amount: amount, Side: model.OrderSideSell, Symbol: symbol}
	}
	for i, item := range data[`bids`].([]interface{}) {
		value := item.([]interface{})
		if len(value) < 2 {
			continue
		}
		price, _ := strconv.ParseFloat(value[0].(string), 64)
		amount, _ := strconv.ParseFloat(value[1].(string), 64)
		bidAsks.Bids[i] = model.Tick{Price: price, Amount: amount, Side: model.OrderSideBuy, Symbol: symbol}
	}
	ts, _ := time.Parse(time.RFC3339, data[`timestamp`].(string))
	bidAsks.Ts = int(ts.UnixNano() / int64(time.Millisecond))
	bidAsks.TsReceived = int(util.GetNowUnixMillion())
	return
}

func WsDepthServeOKFuture(markets *model.Markets, errHandler ErrHandler) (chan struct{}, error) {
	lastPingTime := util.GetNow().Unix()
	wsHandler := func(event []byte) {
		if util.GetNow().Unix()-lastPingTime > 20 { // ping okfuture server every 30 seconds
			lastPingTime = util.GetNow().Unix()
			if err := sendToWs(model.OKFUTURE, []byte(`{"event":"ping"}`)); err != nil {
				util.SocketInfo("okfuture server ping client error " + err.Error())
			}
		}
		if len(event) == 0 {
			return
		}
		var out bytes.Buffer
		reader := flate.NewReader(bytes.NewReader(event))
		_, _ = io.Copy(&out, reader)
		event = out.Bytes()
		resultJson, err := util.NewJSON(event)
		if err != nil {
			return
		}
		table := resultJson.Get(`table`).MustString()
		switch table {
		case "futures/depth5":
			array := resultJson.Get(`data`).MustArray()
			if array != nil && len(array) > 0 {
				depthHandlerOkFuture(markets, array[0].(map[string]interface{}))
			}
		}
	}
	return WebSocketClient(model.OKFUTURE, model.AppConfig.WSUrls[model.OKFUTURE], model.SubscribeDepth,
		GetWSSubscribes(model.OKFUTURE, ``), subscribeHandlerOKFuture, wsHandler, errHandler)
}

func depthHandlerOkFuture(markets *model.Markets, data map[string]interface{}) {
	bidAsks, symbol := parseTickByOkFuture(data)
	if bidAsks == nil {
		return
	}
	sort.Sort(bidAsks.Asks)
	sort.Sort(sort.Reverse(bidAsks.Bids))
	if markets.SetBidAsk(symbol, model.OKFUTURE, bidAsks) {
		for function, handler := range model.GetFunctions(model.OKFUTURE, symbol) {
			settings := model.GetSetting(function, model.OKFUTURE, symbol)
			for _, setting := range settings {
				go handler(setting, bidAsks)
			}
		}
	}
}

func querySetInstrumentsOkFuture(key, secret string) {
	responseBody := SignedRequestOKSwap(key, secret, `GET`,
		`/api/futures/v3/instruments`, nil)
	instrumentJson, err := util.NewJSON(responseBody)
	if err == nil {
		//symbol = strings.ToUpper(symbol)
		for _, item := range instrumentJson.MustArray() {
			future := item.(map[string]interface{})
			if future[`underlying`] != nil && future[`alias`] != nil {
				setInstrument(model.OKFUTURE, strings.ToLower(future[`underlying`].(string)),
					future[`alias`].(string), future[`instrument_id`].(string))
			}
		}
	}
}

func parseAccountOkfuture(key string, account *model.Account, data map[string]interface{}) (balance *model.Balance) {
	if data[`currency`] == nil {
		return
	}
	balance = &model.Balance{
		AccountId:   key,
		Action:      0,
		BalanceTime: util.GetNow(),
		Market:      model.OKFUTURE,
		ID:          model.OKFUTURE + `_` + account.Currency + `_` + util.GetNow().String()[0:10],
	}
	account.Currency = strings.ToLower(data[`currency`].(string))
	// 账户权益
	if data[`equity`] != nil {
		account.Free, _ = strconv.ParseFloat(data[`equity`].(string), 64)
		balance.Amount, _ = strconv.ParseFloat(data[`equity`].(string), 64)
	}
	if data[`margin`] != nil {
		account.Margin, _ = strconv.ParseFloat(data[`margin`].(string), 64)
	}
	if data[`realized_pnl`] != nil {
		account.ProfitReal, _ = strconv.ParseFloat(data[`realized_pnl`].(string), 64)
	}
	if data[`margin_for_unfilled`] != nil {
		account.ProfitUnreal, _ = strconv.ParseFloat(data[`margin_for_unfilled`].(string), 64)
	}
	if data[`underlying`] != nil {
		account.Currency = strings.ToLower(data[`underlying`].(string))
	}
	if data[`currency`] != nil {
		balance.Coin = strings.ToLower(data[`currency`].(string))
	}
	return balance
}

func getBalanceOkfuture(key, secret string, accounts *model.Accounts) (success bool, balances []*model.Balance) {
	responseBody := SignedRequestOKSwap(``, ``, `GET`, "/api/futures/v3/accounts", nil)
	util.SocketInfo(`get okfuture balance: ` + string(responseBody))
	accountJson, err := util.NewJSON(responseBody)
	if err != nil || accountJson == nil || accountJson.Get(`info`) == nil {
		util.SocketInfo(`fail to get refresh accounts okfuture`)
		time.Sleep(time.Second * 2)
		return getBalanceOkfuture(key, secret, accounts)
	}
	balances = make([]*model.Balance, 0)
	items := accountJson.Get(`info`).MustMap()
	for i, value := range items {
		account := accounts.GetAccount(model.OKFUTURE, i)
		if account == nil {
			account = &model.Account{Market: model.OKFUTURE, Ts: util.GetNowUnixMillion()}
		}
		data := value.(map[string]interface{})
		balance := parseAccountOkfuture(key, account, data)
		if balance != nil {
			balances = append(balances, balance)
		}
		instrument := GetCurrentInstrument(key, secret, model.OKFUTURE, account.Currency)
		long, short := getHoldingOkfuture(key, secret, instrument)
		account.Free = long - short
		accounts.SetAccount(model.OKFUTURE, account.Currency, account)
	}
	return true, balances
}

func getHoldingOkfuture(key, secret, instrument string) (long, short float64) {
	responseBody := SignedRequestOKSwap(key, secret, `GET`,
		fmt.Sprintf(`/api/futures/v3/%s/position`, instrument), nil)
	accountJson, err := util.NewJSON(responseBody)
	if err != nil {
		util.Notice(`fail to get okfuture holding ` + err.Error())
		return
	}
	util.SocketInfo(fmt.Sprintf(`okfuture get holding return: %s`, string(responseBody)))
	holdingArray := accountJson.Get(`holding`).MustArray()
	for _, value := range holdingArray {
		holding := value.(map[string]interface{})
		if holding == nil {
			return
		}
		if holding[`long_qty`] != nil {
			long, _ = strconv.ParseFloat(holding[`long_qty`].(string), 64)
		}
		if holding[`short_qty`] != nil {
			short, _ = strconv.ParseFloat(holding[`short_qty`].(string), 64)
		}
	}
	util.SocketInfo(fmt.Sprintf(`get okfuture %s holding %f`, instrument, long-short))
	return long, short
}

// orderSide:  1:开多 2:开空 3:平多 4:平空
// orderType: 是否为对手价 0:不是 1:是
// price == `0` 市价单， != `0` 限价单
func placeOrderOkfuture(key, secret string, order *model.Order, orderSide, orderType, symbol, instrument, price, triggerPrice, size string) {
	switch orderSide {
	case model.OrderSideBuy:
		orderSide = `1`
	case model.OrderSideSell:
		orderSide = `2`
	case model.OrderSideLiquidateLong:
		orderSide = `3`
		long, _ := getHoldingOkfuture(key, secret, instrument)
		sizeFloat, _ := strconv.ParseFloat(size, 64)
		if long < sizeFloat {
			order.Amount = long
			util.Notice(fmt.Sprintf(`holding okfuture size %s long: %f`, size, long))
			_, strAmount := util.FormatNum(long, GetAmountDecimal(model.OKFUTURE, symbol))
			size = strAmount
		}
	case model.OrderSideLiquidateShort:
		orderSide = `4`
		_, short := getHoldingOkfuture(key, secret, instrument)
		sizeFloat, _ := strconv.ParseFloat(size, 64)
		if math.Abs(short) < sizeFloat {
			order.Amount = short
			util.Notice(fmt.Sprintf(`holding okfuture size %s to %f`, size, short))
			_, strAmount := util.FormatNum(math.Abs(short), GetAmountDecimal(model.OKFUTURE, symbol))
			size = strAmount
		}
	default:
		util.Notice(`wrong order side for placeOrderOkfuture ` + orderSide)
		return
	}
	postData := map[string]interface{}{`instrument_id`: instrument, `type`: orderSide, `size`: size}
	algo := `order`
	switch orderType {
	case model.OrderTypeLimit:
		postData[`price`] = price
		postData[`order_type`] = `0`
	case model.PostOnly:
		postData[`order_type`] = `1`
		postData[`price`] = price
	case model.OrderTypeMarket:
		postData[`order_type`] = `4`
		postData[`price`] = price
	case model.OrderTypeStop:
		algo = `order_algo`
		postData[`order_type`] = `1`
		postData[`trigger_price`] = triggerPrice
		priceValue, _ := strconv.ParseFloat(price, 64)
		if priceValue > 0 {
			postData[`algo_type`] = `1`
			postData[`algo_price`] = price
		} else {
			postData[`algo_type`] = `2`
		}
	default:
		util.Notice(`wrong order type for placeOrderOkfuture ` + orderType)
		return
	}
	responseBody := SignedRequestOKSwap(key, secret, http.MethodPost,
		`/api/futures/v3/`+algo, postData)
	resultJson, err := util.NewJSON(responseBody)
	if err == nil {
		result, _ := resultJson.Get(`result`).Bool()
		if !result {
			order.Status = model.CarryStatusFail
		}
		if orderType == model.OrderTypeStop {
			order.OrderId = resultJson.Get(`algo_id`).MustString()
		} else {
			order.OrderId = resultJson.Get(`order_id`).MustString()
		}
		util.Notice(fmt.Sprintf(`[挂单Ok future] %s side: %s type: %s price: %s size: %s order id: %s 返回%s`,
			instrument, orderSide, orderType, price, size, order.OrderId, string(responseBody)))
	}
}

func queryOrdersOkfuture(key, secret, instrument string) (orders []*model.Order) {
	param := url.Values{}
	param.Set(`order_type`, `1`)
	param.Set(`status`, `1`)
	responseBody := SignedRequestOKSwap(key, secret, `GET`,
		fmt.Sprintf(`/api/futures/v3/order_algo/%s?%s`, instrument, param.Encode()), nil)
	orderJson, _ := util.NewJSON(responseBody)
	ids := orderJson.MustArray()
	for _, value := range ids {
		item := value.(map[string]interface{})
		id := item[`algo_ids`].(string)
		if id == `` {
			continue
		}
		result, code, msg := cancelOrderOkfuture(key, secret, instrument, id, model.OrderTypeStop)
		util.SocketInfo(fmt.Sprintf(`queryOrdersOkfuture cancel algo id %v %s %s`, result, code, msg))
		time.Sleep(time.Second)
	}
	return
}

//status: 订单状态(0等待成交 1部分成交 2全部成交 -1撤单 4撤单处理中 5撤单中)
func queryOrderOkfuture(key, secret, instrument, orderType, orderId string) (dealAmount, dealPrice float64, status string) {
	if orderType == model.OrderTypeStop {
		param := url.Values{}
		param.Set(`order_type`, `1`)
		param.Set(`algo_id`, orderId)
		responseBody := SignedRequestOKSwap(key, secret, `GET`,
			fmt.Sprintf(`/api/futures/v3/order_algo/%s?%s`, instrument, param.Encode()), nil)
		orderJson, err := util.NewJSON(responseBody)
		if err != nil {
			return 0, -1, err.Error()
		}
		util.SocketInfo(fmt.Sprintf(`%s %s %s return: %s`, instrument, orderType, orderId, string(responseBody)))
		value := orderJson.MustArray()
		for _, item := range value {
			data := item.(map[string]interface{})
			if data[`algo_ids`] != nil && data[`algo_ids`].(string) == orderId {
				if data[`trigger_price`] != nil {
					dealPrice, _ = strconv.ParseFloat(data[`trigger_price`].(string), 64)
				}
				if data[`real_amount`] != nil {
					dealAmount, _ = strconv.ParseFloat(data[`real_amount`].(string), 64)
				}
				if data[`status`] != nil {
					if data[`status`] == `1` || data[`status`] == `4` {
						status = model.CarryStatusWorking
					} else if data[`status`] == `2` {
						status = model.CarryStatusSuccess
					} else if data[`status`] == `3` || data[`status`] == `5` || data[`status`] == `6` {
						status = model.CarryStatusFail
					}
				}
				if data[`order_id`] != nil && status == model.CarryStatusSuccess {
					return queryOrderOkfuture(key, secret, instrument, model.OrderTypeLimit, data[`order_id`].(string))
				}
			}
		}
	} else {
		responseBody := SignedRequestOKSwap(``, ``, `GET`,
			fmt.Sprintf(`/api/futures/v3/orders/%s/%s`, instrument, orderId), nil)
		orderJson, err := util.NewJSON(responseBody)
		if err != nil {
			return 0, -1, err.Error()
		}
		data := orderJson.MustMap()
		if data[`filled_qty`] != nil {
			dealAmount, _ = strconv.ParseFloat(data[`filled_qty`].(string), 64)
		}
		if data[`price_avg`] != nil {
			dealPrice, _ = strconv.ParseFloat(data[`price_avg`].(string), 64)
		}
		if data[`instrument_id`] != nil && data[`price`] != nil && dealPrice == 0 {
			dealPrice, _ = strconv.ParseFloat(data[`price`].(string), 64)
		}
		if data[`state`] != nil {
			status = model.GetOrderStatus(model.OKFUTURE, data[`state`].(string))
		}
	}
	return
}

func cancelOrderOkfuture(key, secret, instrument string, orderId string, orderType string) (result bool, errCode, msg string) {
	var responseBody []byte
	if orderType == model.OrderTypeStop {
		postData := make(map[string]interface{})
		postData[`instrument_id`] = instrument
		postData[`order_type`] = `1`
		orderIds := []string{orderId}
		postData[`algo_ids`] = orderIds
		responseBody = SignedRequestOKSwap(``, ``, `POST`,
			`/api/futures/v3/cancel_algos`, postData)
	} else {
		responseBody = SignedRequestOKSwap(key, secret, `POST`,
			fmt.Sprintf(`/api/futures/v3/cancel_order/%s/%s`, instrument, orderId), nil)
	}
	util.SocketInfo(string(responseBody))
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		result, _ = orderJson.Get(`result`).Bool()
		msg, _ = orderJson.Get(`error_message`).String()
		return result, ``, msg
	}
	return false, ``, err.Error()
}

func getCandlesOkfuture(key, secret, symbol, instrument, binSize string, start, end time.Time) (
	candles map[string]*model.Candle) {
	candles = make(map[string]*model.Candle)
	param := make(map[string]interface{})
	if binSize == `1d` {
		param[`granularity`] = `86400`
	}
	duration, _ := time.ParseDuration(`-24h`)
	start = start.Add(duration)
	duration, _ = time.ParseDuration(`-48h`)
	end = end.Add(duration)
	param[`start`] = fmt.Sprintf(`%d-%d-%dT%d:%d:%dZ`, start.Year(), start.Month(), start.Day(), 16, 0, 0)
	param[`end`] = fmt.Sprintf(`%d-%d-%dT%d:%d:%dZ`, end.Year(), end.Month(), end.Day(), 16, 0, 0)
	path := fmt.Sprintf(`/api/futures/v3/instruments/%s/candles?%s`, instrument, util.ComposeParams(param))
	response := SignedRequestOKSwap(key, secret, `GET`, path, nil)
	duration, _ = time.ParseDuration(`8h`)
	candleJson, err := util.NewJSON(response)
	if err == nil {
		candleJsons := candleJson.MustArray()
		for _, value := range candleJsons {
			item := value.([]interface{})
			if len(item) < 5 {
				continue
			}
			candle := &model.Candle{Market: model.OKFUTURE, SymbolInstrument: symbol, Period: binSize}
			candle.PriceOpen, _ = strconv.ParseFloat(item[1].(string), 64)
			candle.PriceHigh, _ = strconv.ParseFloat(item[2].(string), 64)
			candle.PriceLow, _ = strconv.ParseFloat(item[3].(string), 64)
			candle.PriceClose, _ = strconv.ParseFloat(item[4].(string), 64)
			start, _ := time.Parse(time.RFC3339, item[0].(string))
			start = start.Add(duration)
			candle.UTCDate = start.Format(time.RFC3339)[0:10]
			candles[candle.UTCDate] = candle
		}
	}
	return
}

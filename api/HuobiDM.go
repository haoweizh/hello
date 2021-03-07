package api

import (
	"encoding/json"
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

var subscribeHandlerHuobiDM = func(subscribes []interface{}, subType string) error {
	var err error = nil
	for _, v := range subscribes {
		subscribeMap := make(map[string]interface{})
		subscribeMap["id"] = strconv.Itoa(util.GetNow().Nanosecond())
		subscribeMap["sub"] = v
		subscribeMessage := util.JsonEncodeMapToByte(subscribeMap)
		if err = sendToWs(model.HuobiDM, subscribeMessage); err != nil {
			util.SocketInfo("huobiDM can not subscribe " + err.Error())
			return err
		}
		util.Notice(`huobi subscribed ` + string(subscribeMessage))
	}
	return err
}

func WsDepthServeHuobiDM(markets *model.Markets, errHandler model.ErrHandler) (chan struct{}, error) {
	wsHandler := func(event []byte) {
		res := util.UnGzip(event)
		responseJson, _ := util.NewJSON(res)
		if responseJson.Get(`ping`).MustInt() > 0 {
			pingMap := make(map[string]interface{})
			pingMap["pong"] = responseJson.Get(`ping`).MustInt()
			pingParams := util.JsonEncodeMapToByte(pingMap)
			if err := sendToWs(model.HuobiDM, pingParams); err != nil {
				util.SocketInfo("huobiDM server ping client error " + err.Error())
			}
		} else {
			responseJson = responseJson.Get(`tick`)
			bidAsk := model.BidAsk{}
			bids := responseJson.Get(`bids`).MustArray()
			asks := responseJson.Get(`asks`).MustArray()
			bidAsk.Bids = make([]model.Tick, len(bids))
			bidAsk.Asks = make([]model.Tick, len(asks))
			for i, item := range bids {
				value := item.([]interface{})
				if value == nil || len(value) < 2 {
					continue
				}
				price, _ := value[0].(json.Number).Float64()
				amount, _ := value[1].(json.Number).Float64()
				bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount}
			}
			for i, item := range asks {
				value := item.([]interface{})
				if value == nil || len(value) < 2 {
					continue
				}
				price, _ := value[0].(json.Number).Float64()
				amount, _ := value[1].(json.Number).Float64()
				bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount}
			}
			bidAsk.Ts = responseJson.Get(`ts`).MustInt()
			bidAsk.TsReceived = int(util.GetNowUnixMillion())
			symbol := responseJson.Get(`ch`).MustString()
			strs := strings.Split(symbol, `.`)
			if strs != nil && len(strs) > 1 {
				symbol = strings.ToLower(strs[1])
				sort.Sort(bidAsk.Asks)
				sort.Sort(sort.Reverse(bidAsk.Bids))
				if markets.SetBidAsk(symbol, model.HuobiDM, &bidAsk) {
					for function, handler := range model.GetFunctions(model.HuobiDM, symbol) {
						if handler != nil {
							settings := model.GetSetting(function, model.HuobiDM, symbol)
							for _, setting := range settings {
								go handler(setting, &bidAsk)
							}
						}
					}
				}
			}
		}
	}
	return WebSocketClient(model.HuobiDM, model.AppConfig.WSUrls[model.HuobiDM]+`ws`, model.SubscribeDepth,
		GetWSSubscribes(model.HuobiDM, model.SubscribeDepth), subscribeHandlerHuobiDM, wsHandler, errHandler)
}

func parseAccountHuobiDM(key string, account *model.Account, data map[string]interface{}) (balance *model.Balance) {
	if data[`symbol`] == nil {
		return nil
	}
	account.Currency = strings.ToLower(data[`symbol`].(string))
	balance = &model.Balance{
		AccountId:   key,
		BalanceTime: util.GetNow(),
		Coin:        account.Currency,
		Market:      model.HuobiDM,
		ID:          model.HuobiDM + `_` + account.Currency + `_` + util.GetNow().String()[0:10],
	}
	if data[`margin_balance`] != nil { // 账户权益
		account.Free, _ = data[`margin_balance`].(json.Number).Float64()
		balance.Amount, _ = data[`margin_balance`].(json.Number).Float64()
	}
	if data[`margin_frozen`] != nil { // 冻结保证金
		account.Frozen, _ = data[`margin_frozen`].(json.Number).Float64()
	}
	if data[`profit_real`] != nil { // 已实现盈亏
		account.ProfitReal, _ = data[`profit_real`].(json.Number).Float64()
	}
	if data[`profit_unreal`] != nil { // 未实现盈亏
		account.ProfitUnreal, _ = data[`profit_unreal`].(json.Number).Float64()
	}
	if data[`liquidation_price`] != nil { // 预估强平价
		account.LiquidationPrice, _ = data[`liquidation_price`].(json.Number).Float64()
	}
	if data[`lever_rate`] != nil { // 杠杆倍数
		account.LeverRate, _ = data[`lever_rate`].(json.Number).Int64()
	}
	return
}

func getBalanceHuobiDM(key, secret string, accounts *model.Accounts) (success bool, balances []*model.Balance) {
	responseBody := SignedRequestHuobi(key, secret, model.HuobiDM, `POST`, "/api/v1/contract_account_info", nil)
	util.SocketInfo(`get huobiDM balance: ` + string(responseBody))
	accountJson, err := util.NewJSON(responseBody)
	if err != nil || accountJson == nil || strings.ToLower(accountJson.Get(`status`).MustString()) != `ok` {
		time.Sleep(time.Second * 2)
		util.SocketInfo(`fail to get huobiDM balance`)
		return getBalanceHuobiDM(key, secret, accounts)
	}
	balances = make([]*model.Balance, 0)
	items := accountJson.Get(`data`).MustArray()
	for _, value := range items {
		account := &model.Account{Market: model.HuobiDM, Ts: util.GetNowUnixMillion()}
		data := value.(map[string]interface{})
		balance := parseAccountHuobiDM(key, account, data)
		if balance != nil {
			balances = append(balances, balance)
		}
		accounts.SetAccount(model.HuobiDM, account.Currency, account)
	}
	return true, balances
}

func getHoldingHuobiDM(key, secret string, accounts *model.Accounts) (success bool) {
	responseBody := SignedRequestHuobi(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_position_info`, nil)
	accountJson, err := util.NewJSON(responseBody)
	if err != nil || accountJson == nil || strings.ToLower(accountJson.Get(`status`).MustString()) != `ok` {
		util.Notice(`fail to refresh account huobiDM holding `)
		time.Sleep(time.Second * 2)
		return getHoldingHuobiDM(key, secret, accounts)
	}
	util.SocketInfo(fmt.Sprintf(`huobiDM get holding return: %s`, string(responseBody)))
	holdingArray := accountJson.Get(`data`).MustArray()
	for _, value := range holdingArray {
		holding := value.(map[string]interface{})
		if holding == nil {
			continue
		}
		if holding[`symbol`] != nil && holding[`contract_type`] != nil && holding[`direction`] != nil {
			symbol := holding[`symbol`].(string)
			switch holding[`contract_type`].(string) {
			case `this_week`:
				symbol = symbol + `_CW`
			case `next_week`:
				symbol = symbol + `_NW`
			case `quarter`:
				symbol = symbol + `_CQ`
			case `next_quarter`:
				symbol = symbol + `_NQ`
			}
			symbol += holding[`direction`].(string)
			symbol = strings.ToLower(symbol)
			account := &model.Account{Market: model.HuobiDM, Ts: util.GetNowUnixMillion(), Currency: symbol}
			if holding[`volume`] != nil { // 持仓量
				account.Holding, _ = holding[`volume`].(json.Number).Float64()
			}
			if holding[`available`] != nil { // 可平仓数量
				account.Free, _ = holding[`available`].(json.Number).Float64()
			}
			if holding[`frozen`] != nil {
				account.Frozen, _ = holding[`frozen`].(json.Number).Float64()
			}
			if holding[`cost_open`] != nil {
				account.EntryPrice, _ = holding[`cost_open`].(json.Number).Float64()
			}
			if holding[`profit_unreal`] != nil {
				account.ProfitUnreal, _ = holding[`profit_unreal`].(json.Number).Float64()
			}
			if holding[`profit`] != nil {
				account.ProfitReal, _ = holding[`profit`].(json.Number).Float64()
			}
			if holding[`position_margin`] != nil {
				account.Margin, _ = holding[`position_margin`].(json.Number).Float64()
			}
			if holding[`direction`] != nil {
				account.Direction, _ = holding[`direction`].(string)
			}
			if holding[`lever_rate`] != nil { // 杠杆倍数
				account.LeverRate, _ = holding[`lever_rate`].(json.Number).Int64()
			}
			util.SocketInfo(fmt.Sprintf(`get huobiDB %s holding %f`, account.Direction, account.Holding))
			accounts.SetAccount(model.HuobiDM, account.Currency, account)
		}
	}
	return true
}

func placeOrderHuobiDM(key, secret string, order *model.Order,
	orderSide, orderType, contractCode, symbol, lever, price, triggerPrice, size string) {
	if orderType != model.OrderTypeStop {
		return
	}
	// special for huobiDM contract
	triggerType := `ge`
	direction := `buy`
	offset := `close`
	switch orderSide {
	case model.OrderSideBuy:
		triggerType = `ge`
		direction = `buy`
		offset = `open`
	case model.OrderSideSell:
		triggerType = `le`
		direction = `sell`
		offset = `open`
	case model.OrderSideLiquidateShort:
		triggerType = `ge`
		direction = `buy`
		offset = `close`
		getHoldingHuobiDM(key, secret, model.AppAccounts)
		sizeFloat, _ := strconv.ParseFloat(size, 64)
		holding := math.Abs(model.AppAccounts.GetAccount(model.HuobiDM, symbol+model.OrderSideSell).Holding)
		util.Notice(fmt.Sprintf(`holding huobiDM size %s to %f`, size, holding))
		if holding < sizeFloat {
			_, strAmount := util.FormatNum(holding, GetAmountDecimal(model.HuobiDM, symbol))
			size = strAmount
		}
	case model.OrderSideLiquidateLong:
		triggerType = `le`
		direction = `sell`
		offset = `close`
		getHoldingHuobiDM(key, secret, model.AppAccounts)
		sizeFloat, _ := strconv.ParseFloat(size, 64)
		holding := math.Abs(model.AppAccounts.GetAccount(model.HuobiDM, symbol+model.OrderSideBuy).Holding)
		util.Notice(fmt.Sprintf(`holding huobiDM size %s to %f`, size, holding))
		if holding < sizeFloat {
			_, strAmount := util.FormatNum(holding, GetAmountDecimal(model.HuobiDM, symbol))
			size = strAmount
		}
	}
	param := map[string]interface{}{`contract_code`: contractCode, `trigger_type`: triggerType,
		`trigger_price`: triggerPrice, `order_price`: price, `volume`: size,
		`direction`: direction, `offset`: offset, `lever_rate`: lever}
	responseBody := SignedRequestHuobi(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_trigger_order`, param)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		data := orderJson.Get(`data`).MustMap()
		if data != nil {
			order.OrderId = data[`order_id_str`].(string)
		}
	}
}

func cancelOrderHuobiDM(key, secret, symbol, orderId string) (result bool, errCode, msg string) {
	if strings.Contains(symbol, `_`) {
		symbol = symbol[0:strings.Index(symbol, `_`)]
	}
	param := map[string]interface{}{`symbol`: symbol, `order_id`: orderId}
	responseBody := SignedRequestHuobi(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_trigger_cancel`, param)
	cancelJson, err := util.NewJSON(responseBody)
	if err == nil {
		successIds := cancelJson.GetPath(`data`, `successes`).MustString()
		if strings.Contains(successIds, orderId) {
			return true, ``, ``
		}
	}
	return false, ``, ``
}

//func cancelAllHuobiDM(contractCode string)  {
//	param := map[string]interface{}{`contract_code`: contractCode}
//	responseBody := SignedRequestHuobi(`POST`, `/api/v1/contract_trigger_cancel`, param)
//}

func queryOpenTriggerOrderHuobiDM(key, secret, symbol, orderId string) (isWorking bool) {
	if strings.Contains(symbol, `_`) {
		symbol = symbol[0:strings.Index(symbol, `_`)]
	}
	data := map[string]interface{}{`symbol`: symbol}
	responseBody := SignedRequestHuobi(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_trigger_openorders`, data)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		items := orderJson.GetPath(`data`, `orders`).MustArray()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value[`order_id_str`] != nil && value[`order_id_str`] == orderId {
				return true
			}
		}
	}
	return false
}

func queryHisTriggerOrderHuobiDM(key, secret, symbol, orderId string) (relatedOrderId string) {
	if strings.Contains(symbol, `_`) {
		symbol = symbol[0:strings.Index(symbol, `_`)]
	}
	data := map[string]interface{}{`symbol`: symbol, `trade_type`: `0`, `status`: `0`, `create_date`: `3`}
	responseBody := SignedRequestHuobi(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_trigger_hisorders`, data)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		items := orderJson.GetPath(`data`, `orders`).MustArray()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value[`order_id_str`] != nil && value[`order_id_str`] == orderId {
				if value[`relation_order_id`] != nil {
					return value[`relation_order_id`].(string)
				}
			}
		}
	}
	return `-1`
}

// status 1准备提交 2准备提交 3已提交 4部分成交 5部分成交已撤单 6全部成交 7已撤单 11撤单中
func queryOrderHuobiDM(key, secret, symbol, orderId string) (dealAmount, dealPrice float64, status string) {
	if strings.Contains(symbol, `_`) {
		symbol = symbol[0:strings.Index(symbol, `_`)]
	}
	data := map[string]interface{}{`symbol`: symbol, `order_id`: orderId}
	responseBody := SignedRequestHuobi(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_order_info`, data)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		items := orderJson.Get(`data`).MustArray()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value[`order_id_str`] != nil && value[`order_id_str`].(string) == orderId {
				if value[`trade_avg_price`] != nil {
					dealPrice, _ = value[`trade_avg_price`].(json.Number).Float64()
				}
				if value[`trade_volume`] != nil {
					dealAmount, _ = value[`trade_volume`].(json.Number).Float64()
				}
				if value[`status`] != nil {
					intStatus, _ := value[`status`].(json.Number).Int64()
					switch intStatus {
					case 1, 2, 3, 4, 11:
						status = model.CarryStatusWorking
					case 5, 6:
						status = model.CarryStatusSuccess
					case 7:
						status = model.CarryStatusFail
					}
				}
				return
			}
		}
	}
	return 0, 0, model.CarryStatusFail
}

func querySetInstrumentsHuobiDM(key, secret string) {
	responseBody := SignedRequestHuobi(key, secret, model.HuobiDM, `GET`, `/api/v1/contract_contract_info`, nil)
	instrumentJson, err := util.NewJSON(responseBody)
	if err == nil {
		for _, item := range instrumentJson.Get(`data`).MustArray() {
			future := item.(map[string]interface{})
			if future[`contract_code`] != nil && future[`contract_type`] != nil {
				setInstrument(model.HuobiDM, strings.ToLower(future[`symbol`].(string)),
					future[`contract_type`].(string), future[`contract_code`].(string))
			}
		}
	}
}

func getCandlesHuobiDM(key, secret, symbol, binSize string, start, end time.Time) (
	candles map[string]*model.Candle) {
	param := map[string]interface{}{`symbol`: symbol, `from`: strconv.FormatInt(start.Unix(), 10),
		`to`: strconv.FormatInt(end.Unix(), 10)}
	if binSize == `1d` {
		param[`period`] = `1day`
	}
	candles = make(map[string]*model.Candle)
	response := SignedRequestHuobi(key, secret, model.HuobiDM, `GET`, `/market/history/kline`, param)
	//duration, _ := time.ParseDuration(`8h`)
	candleJson, err := util.NewJSON(response)
	if err == nil {
		candleJsons := candleJson.Get(`data`).MustArray()
		for _, value := range candleJsons {
			item := value.(map[string]interface{})
			candle := &model.Candle{Market: model.HuobiDM, Symbol: symbol, Period: binSize}
			if item[`open`] != nil {
				candle.PriceOpen, _ = item[`open`].(json.Number).Float64()
			}
			if item[`high`] != nil {
				candle.PriceHigh, _ = item[`high`].(json.Number).Float64()
			}
			if item[`low`] != nil {
				candle.PriceLow, _ = item[`low`].(json.Number).Float64()
			}
			if item[`close`] != nil {
				candle.PriceClose, _ = item[`close`].(json.Number).Float64()
			}
			if item[`id`] != nil {
				unixSeconds, _ := item[`id`].(json.Number).Int64()
				candle.UTCDate = time.Unix(unixSeconds, 0).Format(time.RFC3339)[0:10]
			}
			candles[candle.UTCDate] = candle
		}
	}
	return
}

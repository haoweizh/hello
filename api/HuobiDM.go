package api

import (
	"encoding/json"
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"time"
)

const restHuobiDM = `api.hbdm.com`
const wsHuobiDM = `wss://api.hbdm.vn/linear-swap-ws`

var subscribeHandlerHuobiDM = func(subscribes []interface{}, subType string) error {
	var err error = nil
	for _, v := range subscribes {
		subscribeMap := make(map[string]interface{})
		subscribeMap["id"] = strconv.Itoa(util.GetNow().Nanosecond())
		subscribeMap["sub"] = v
		subscribeMessage := util.JsonEncodeToByte(subscribeMap)
		if err = sendToWs(model.HuobiDM, subscribeMessage); err != nil {
			util.SocketInfo("huobiDM can not subscribe " + err.Error())
			return err
		}
		util.Notice(`huobiDM subscribed ` + string(subscribeMessage))
	}
	return err
}

func WsDepthServeHuobiDM(markets *model.Markets, orderHandler OrderHandler) (chan struct{}, error) {
	wsHandler := func(channelKey string, event []byte, orderHandler OrderHandler) {
		res := util.UnGzip(event)
		responseJson, _ := util.NewJSON(res)
		if responseJson.Get(`ping`).MustInt() > 0 {
			pingMap := make(map[string]interface{})
			pingMap["pong"] = responseJson.Get(`ping`).MustInt()
			pingParams := util.JsonEncodeToByte(pingMap)
			if err := sendToWs(model.HuobiDM, pingParams); err != nil {
				util.SocketInfo("huobiDM server ping client error " + err.Error())
			}
		} else {
			tickJson := responseJson.Get(`tick`)
			if tickJson.Interface() == nil {
				return
			}
			symbol := responseJson.Get(`ch`).MustString()
			strs := strings.Split(symbol, `.`)
			if strs == nil || len(strs) <= 1 {
				return
			}
			symbol = strings.ToLower(strs[1])
			now := int(time.Now().UnixNano() / int64(time.Millisecond))

			bidAsk := model.BidAsk{Ts: responseJson.Get(`ts`).MustInt(), TsReceived: now, UpdateId: tickJson.Get("ts").MustInt64()}
			bid := tickJson.Get(`bid`).MustArray()
			ask := tickJson.Get(`ask`).MustArray()
			bidAsk.Bids = make([]model.Tick, 1)
			bidAsk.Asks = make([]model.Tick, 1)

			if bid == nil || len(bid) < 2 || ask == nil || len(ask) < 2 {
				return
			}

			bidAmount, _ := bid[1].(json.Number).Float64()
			bidPrice, _ := bid[0].(json.Number).Float64()
			bidSuccess, bidAmount := ParseRealAmount(model.HuobiDM, symbol, bidAmount)
			if !bidSuccess {
				return
			}

			askAmount, _ := ask[1].(json.Number).Float64()
			askPrice, _ := ask[0].(json.Number).Float64()
			askSuccess, askAmount := ParseRealAmount(model.HuobiDM, symbol, askAmount)
			if !askSuccess {
				return
			}

			bidAsk.Bids = []model.Tick{{Price: bidPrice, Amount: bidAmount}}
			bidAsk.Asks = []model.Tick{{Price: askPrice, Amount: askAmount}}

			haveOld, old := markets.GetBidAsk(symbol, model.HuobiDM)
			if haveOld && old.UpdateId > bidAsk.UpdateId {
				return
			}

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
	return WebSocketClient(model.HuobiDM, wsHuobiDM, model.SubscribeTicker,
		GetWSSubscribes(model.HuobiDM, model.SubscribeTicker), subscribeHandlerHuobiDM, wsHandler, orderHandler)
}

func parseBalanceHuobiDM(key string, data map[string]interface{}) (balance *model.Balance) {
	if data[`symbol`] == nil {
		return nil
	}
	currency := strings.ToLower(data[`symbol`].(string))
	balance = &model.Balance{
		AccountId:   key,
		BalanceTime: util.GetNow(),
		Coin:        currency,
		Market:      model.HuobiDM,
		ID:          model.HuobiDM + `_` + currency + `_` + util.GetNow().String()[0:10],
	}
	if data[`margin_balance`] != nil { // 账户权益
		balance.Amount, _ = data[`margin_balance`].(json.Number).Float64()
	}
	//if data[`margin_frozen`] != nil { // 冻结保证金
	//	account.Frozen, _ = data[`margin_frozen`].(json.Number).Float64()
	//}
	//if data[`profit_real`] != nil { // 已实现盈亏
	//	account.ProfitReal, _ = data[`profit_real`].(json.Number).Float64()
	//}
	//if data[`profit_unreal`] != nil { // 未实现盈亏
	//	account.ProfitUnreal, _ = data[`profit_unreal`].(json.Number).Float64()
	//}
	//if data[`liquidation_price`] != nil { // 预估强平价
	//	account.LiquidationPrice, _ = data[`liquidation_price`].(json.Number).Float64()
	//}
	//if data[`lever_rate`] != nil { // 杠杆倍数
	//	account.LeverRate, _ = data[`lever_rate`].(json.Number).Int64()
	//}
	return
}

func getMarketsHuobiDM() (marketInfos map[string]*model.MarketInfo) {
	//param := map[string]interface{}{`support_margin_mode`: "cross"}//全仓模式
	responseBody := SignedRequestHuobi(``, ``, `GET`, restHuobiDM, "/linear-swap-api/v1/swap_contract_info", nil)
	contractsJson, err := util.NewJSON(responseBody)

	marketInfos = make(map[string]*model.MarketInfo)
	if err != nil || contractsJson == nil || strings.ToLower(contractsJson.Get(`status`).MustString()) != `ok` {
		return marketInfos
	}

	items, _ := contractsJson.Get("data").Array()
	for _, item := range items {
		value := item.(map[string]interface{})

		//只获取all-全逐仓都支持、cross-全仓模式
		if value["support_margin_mode"] != nil && (value["support_margin_mode"].(string) == "all" || value["support_margin_mode"].(string) == "cross") {
			marketInfo := &model.MarketInfo{Name: strings.ToLower(value["contract_code"].(string))}

			if value["symbol"] != nil {
				marketInfo.CTCurrency = strings.ToLower(value["symbol"].(string))
			}
			if value["contract_size"] != nil {
				marketInfo.CTValue, _ = value["contract_size"].(json.Number).Float64()
			}
			if value["price_tick"] != nil {
				marketInfo.PriceIncrement, _ = value["price_tick"].(json.Number).Float64()
			}
			marketInfo.SizeIncrement = 1

			marketInfos[marketInfo.Name] = marketInfo
		}
	}
	return marketInfos
}

func getBalanceHuobiDM(key, secret string) (success bool, balances []*model.Balance) {
	responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobiDM, "/api/v1/contract_account_info", nil)
	util.SocketInfo(`get huobiDM balance: ` + string(responseBody))
	accountJson, err := util.NewJSON(responseBody)
	if err != nil || accountJson == nil || strings.ToLower(accountJson.Get(`status`).MustString()) != `ok` {
		time.Sleep(time.Second * 2)
		util.SocketInfo(`fail to get huobiDM balance`)
		return getBalanceHuobiDM(key, secret)
	}
	balances = make([]*model.Balance, 0)
	items := accountJson.Get(`data`).MustArray()
	for _, value := range items {
		data := value.(map[string]interface{})
		balance := parseBalanceHuobiDM(key, data)
		if balance != nil {
			balances = append(balances, balance)
		}
		//accounts.SetAccount(model.HuobiDM, account.Currency, account)
	}
	return true, balances
}

func getHoldingHuobiDM(key, secret, symbolSide string) (position *model.Position) {
	responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobiDM, `/api/v1/contract_position_info`, nil)
	accountJson, err := util.NewJSON(responseBody)
	if err != nil || accountJson == nil || strings.ToLower(accountJson.Get(`status`).MustString()) != `ok` {
		util.Notice(`fail to refresh account huobiDM holding `)
		time.Sleep(time.Second * 2)
		return getHoldingHuobiDM(key, secret, symbolSide)
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
			if symbol != symbolSide {
				continue
			}
			position = &model.Position{Market: model.HuobiDM, Ts: util.GetNowUnixMillion(), Currency: symbol}
			//if holding[`volume`] != nil { // 持仓量
			//	position.Holding, _ = holding[`volume`].(json.Number).Float64()
			//}
			if holding[`available`] != nil { // 可平仓数量
				position.Free, _ = holding[`available`].(json.Number).Float64()
			}
			if holding[`frozen`] != nil {
				position.Frozen, _ = holding[`frozen`].(json.Number).Float64()
			}
			if holding[`cost_open`] != nil {
				position.EntryPrice, _ = holding[`cost_open`].(json.Number).Float64()
			}
			if holding[`profit_unreal`] != nil {
				position.ProfitUnreal, _ = holding[`profit_unreal`].(json.Number).Float64()
			}
			if holding[`profit`] != nil {
				position.ProfitReal, _ = holding[`profit`].(json.Number).Float64()
			}
			if holding[`position_margin`] != nil {
				position.Margin, _ = holding[`position_margin`].(json.Number).Float64()
			}
			if holding[`direction`] != nil {
				position.Direction, _ = holding[`direction`].(string)
			}
			if holding[`lever_rate`] != nil { // 杠杆倍数
				position.LeverRate, _ = holding[`lever_rate`].(json.Number).Int64()
			}
			util.SocketInfo(fmt.Sprintf(`get huobiDB %s holding %f`, position.Direction, position.Free))
		}
	}
	return position
}

// 不适宜快速下单
func placeOrderHuobiDM(key, secret string, order *model.Order,
	orderSide, orderType, contractCode, symbol, price, triggerPrice, size string) {
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
		position := getHoldingHuobiDM(key, secret, symbol+model.OrderSideSell)
		sizeFloat, _ := strconv.ParseFloat(size, 64)
		if position != nil {
			holding := math.Abs(position.Free)
			util.Notice(fmt.Sprintf(`holding huobiDM size %s to %f`, size, holding))
			if holding < sizeFloat {
				_, strAmount := util.FormatNum(holding, GetAmountDecimal(model.HuobiDM))
				size = strAmount
			}
		} else {
			size = `0`
		}
	case model.OrderSideLiquidateLong:
		triggerType = `le`
		direction = `sell`
		offset = `close`
		position := getHoldingHuobiDM(key, secret, symbol+model.OrderSideBuy)
		if position != nil {
			sizeFloat, _ := strconv.ParseFloat(size, 64)
			holding := math.Abs(position.Free)
			util.Notice(fmt.Sprintf(`holding huobiDM size %s to %f`, size, holding))
			if holding < sizeFloat {
				_, strAmount := util.FormatNum(holding, GetAmountDecimal(model.HuobiDM))
				size = strAmount
			}
		} else {
			size = `0`
		}
	}
	//account := model.AppAccounts.GetAccount(market, symbol)
	//lever := `5`
	//if account != nil {
	//	lever = strconv.FormatInt(account.LeverRate, 10)
	//}
	param := map[string]interface{}{`contract_code`: contractCode, `trigger_type`: triggerType,
		`trigger_price`: triggerPrice, `order_price`: price, `volume`: size,
		`direction`: direction, `offset`: offset, `lever_rate`: `5`}
	responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobiDM, `/api/v1/contract_trigger_order`, param)
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
	responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobiDM, `/api/v1/contract_trigger_cancel`, param)
	cancelJson, err := util.NewJSON(responseBody)
	if err == nil {
		successIds := cancelJson.GetPath(`data`, `successes`).MustString()
		if strings.Contains(successIds, orderId) {
			return true, ``, ``
		}
	}
	return false, ``, ``
}

func queryOpenTriggerOrderHuobiDM(key, secret, symbol, orderId string) (isWorking bool) {
	if strings.Contains(symbol, `_`) {
		symbol = symbol[0:strings.Index(symbol, `_`)]
	}
	data := map[string]interface{}{`symbol`: symbol}
	responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobiDM, `/api/v1/contract_trigger_openorders`, data)
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
	responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobiDM, `/api/v1/contract_trigger_hisorders`, data)
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
	responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobiDM, `/api/v1/contract_order_info`, data)
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
	responseBody := SignedRequestHuobi(key, secret, `GET`, restHuobiDM, `/api/v1/contract_contract_info`, nil)
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
	response := SignedRequestHuobi(key, secret, `GET`, restHuobiDM, `/market/history/kline`, param)
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

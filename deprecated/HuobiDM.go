package deprecated

//
//import (
//	"crypto/hmac"
//	"crypto/sha256"
//	"encoding/base64"
//	"encoding/json"
//	"fmt"
//	"github.com/gorilla/websocket"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"net/http"
//	"net/url"
//	"sort"
//	"strconv"
//	"strings"
//	"time"
//)
//
//const restHuobiDM = `api.hbdm.vn`
//const wsHuobiDM = `wss://api.hbdm.vn/ws`
//
//var subscribeHandlerHuobiDM = func(connection *websocket.conn, subscribes []interface{}) error {
//	var err error = nil
//	for _, v := range subscribes {
//		subscribeMap := make(map[string]interface{})
//		subscribeMap["id"] = strconv.Itoa(util.GetNow().Nanosecond())
//		subscribeMap["sub"] = v
//		subscribeMessage := util.JsonEncodeToByte(subscribeMap)
//		if err = api.SendToConnection(model.HuobiDM, connection, subscribeMessage); err != nil {
//			util.SocketInfo("huobiDM can not subscribe " + err.Error())
//			return err
//		}
//		util.Notice(`huobiDM subscribed ` + string(subscribeMessage))
//	}
//	return err
//}
//
//func WsDepthServeHuobiDM(markets *model.Markets, orderHandler api.OrderHandler) ([]chan struct{}, error) {
//	wsHandler := func(connection *websocket.conn, event []byte, orderHandler api.OrderHandler) {
//		res := util.UnGzip(event)
//		responseJson, _ := util.NewJSON(res)
//		if responseJson.Get(`ping`).MustInt() > 0 {
//			pingMap := make(map[string]interface{})
//			pingMap["pong"] = responseJson.Get(`ping`).MustInt()
//			pingParams := util.JsonEncodeToByte(pingMap)
//			if err := api.SendToConnection(model.HuobiDM, connection, pingParams); err != nil {
//				util.SocketInfo("huobiDM server ping client error " + err.Error())
//			}
//		} else {
//			responseJson = responseJson.Get(`tick`)
//			symbol := responseJson.Get(`ch`).MustString()
//			bidAsk := model.BidAsk{}
//			bids := responseJson.Get(`bids`).MustArray()
//			asks := responseJson.Get(`asks`).MustArray()
//			bidAsk.Bids = make([]model.Tick, len(bids))
//			bidAsk.Asks = make([]model.Tick, len(asks))
//			for i, item := range bids {
//				value := item.([]interface{})
//				if value == nil || len(value) < 2 {
//					continue
//				}
//				price, _ := value[0].(json.Number).Float64()
//				amount, _ := value[1].(json.Number).Float64()
//				bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: model.HuobiDM, Symbol: symbol, Side: model.OrderSideBuy}
//			}
//			for i, item := range asks {
//				value := item.([]interface{})
//				if value == nil || len(value) < 2 {
//					continue
//				}
//				price, _ := value[0].(json.Number).Float64()
//				amount, _ := value[1].(json.Number).Float64()
//				bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: model.HuobiDM, Symbol: symbol, Side: model.OrderSideSell}
//			}
//			bidAsk.Ts = responseJson.Get(`ts`).MustInt()
//			bidAsk.TsReceived = int(util.GetNowUnixMillion())
//			strs := strings.Split(symbol, `.`)
//			if strs != nil && len(strs) > 1 {
//				symbol = strings.ToLower(strs[1])
//				sort.Sort(bidAsk.Asks)
//				sort.Sort(sort.Reverse(bidAsk.Bids))
//				if markets.SetBidAsk(symbol, model.HuobiDM, &bidAsk) {
//					for function, handler := range model.GetFunctions(model.HuobiDM, symbol) {
//						if handler != nil {
//							setting := model.GetSetting(function, model.HuobiDM, symbol)
//							if setting != nil {
//								go handler(setting, &bidAsk)
//							}
//						}
//					}
//				}
//			}
//		}
//	}
//	return api.WebSocketClient(model.HuobiDM, wsHuobiDM, api.GetWSSubscribes(model.HuobiDM, model.SubscribeDepth),
//		subscribeHandlerHuobiDM, wsHandler, orderHandler, wsStepHuobi)
//}
//
//func parseBalanceHuobiDM(key string, data map[string]interface{}) (balance *model.Balance) {
//	if data[`symbol`] == nil {
//		return nil
//	}
//	currency := strings.ToLower(data[`symbol`].(string))
//	balance = &model.Balance{
//		AccountId:   key,
//		BalanceTime: util.GetNow(),
//		Coin:        currency,
//		Market:      model.HuobiDM,
//		ID:          model.HuobiDM + `_` + currency + `_` + util.GetNow().String()[0:10],
//	}
//	if data[`margin_balance`] != nil { // 账户权益
//		balance.Amount, _ = data[`margin_balance`].(json.Number).Float64()
//	}
//	//if data[`margin_frozen`] != nil { // 冻结保证金
//	//	account.Frozen, _ = data[`margin_frozen`].(json.Number).Float64()
//	//}
//	//if data[`profit_real`] != nil { // 已实现盈亏
//	//	account.ProfitReal, _ = data[`profit_real`].(json.Number).Float64()
//	//}
//	//if data[`profit_unreal`] != nil { // 未实现盈亏
//	//	account.ProfitUnreal, _ = data[`profit_unreal`].(json.Number).Float64()
//	//}
//	//if data[`liquidation_price`] != nil { // 预估强平价
//	//	account.LiquidationPrice, _ = data[`liquidation_price`].(json.Number).Float64()
//	//}
//	//if data[`lever_rate`] != nil { // 杠杆倍数
//	//	account.LeverRate, _ = data[`lever_rate`].(json.Number).Int64()
//	//}
//	return
//}
//
//func getBalanceHuobiDM(key, secret string) (success bool, balances []*model.Balance) {
//	responseBody := SignedRequestHuobiDM(key, secret, model.HuobiDM, `POST`, "/api/v1/contract_account_info", nil)
//	util.SocketInfo(`get huobiDM balance: ` + string(responseBody))
//	accountJson, err := util.NewJSON(responseBody)
//	if err != nil || accountJson == nil || strings.ToLower(accountJson.Get(`status`).MustString()) != `ok` {
//		time.Sleep(time.Minute * 5)
//		util.SocketInfo(`fail to get huobiDM balance`)
//		return getBalanceHuobiDM(key, secret)
//	}
//	balances = make([]*model.Balance, 0)
//	items := accountJson.Get(`data`).MustArray()
//	for _, value := range items {
//		data := value.(map[string]interface{})
//		balance := parseBalanceHuobiDM(key, data)
//		if balance != nil {
//			balances = append(balances, balance)
//		}
//		//accounts.SetAccount(model.HuobiDM, account.Currency, account)
//	}
//	return true, balances
//}
//
//func getHoldingHuobiDM(key, secret, symbolSide string) (position *model.Position) {
//	responseBody := SignedRequestHuobiDM(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_position_info`, nil)
//	accountJson, err := util.NewJSON(responseBody)
//	if err != nil || accountJson == nil || strings.ToLower(accountJson.Get(`status`).MustString()) != `ok` {
//		util.Notice(`fail to refresh account huobiDM holding `)
//		time.Sleep(time.Minute * 5)
//		return getHoldingHuobiDM(key, secret, symbolSide)
//	}
//	util.SocketInfo(fmt.Sprintf(`huobiDM get holding return: %s`, string(responseBody)))
//	holdingArray := accountJson.Get(`data`).MustArray()
//	for _, value := range holdingArray {
//		holding := value.(map[string]interface{})
//		if holding == nil {
//			continue
//		}
//		if holding[`symbol`] != nil && holding[`contract_type`] != nil && holding[`direction`] != nil {
//			symbol := holding[`symbol`].(string)
//			switch holding[`contract_type`].(string) {
//			case `this_week`:
//				symbol = symbol + `_CW`
//			case `next_week`:
//				symbol = symbol + `_NW`
//			case `quarter`:
//				symbol = symbol + `_CQ`
//			case `next_quarter`:
//				symbol = symbol + `_NQ`
//			}
//			symbol += holding[`direction`].(string)
//			symbol = strings.ToLower(symbol)
//			if symbol != symbolSide {
//				continue
//			}
//			position = &model.Position{Market: model.HuobiDM, Ts: util.GetNowUnixMillion(), Currency: symbol}
//			//if holding[`volume`] != nil { // 持仓量
//			//	position.Holding, _ = holding[`volume`].(json.Number).Float64()
//			//}
//			if holding[`available`] != nil { // 可平仓数量
//				position.Holding, _ = holding[`available`].(json.Number).Float64()
//			}
//			if holding[`frozen`] != nil {
//				position.Frozen, _ = holding[`frozen`].(json.Number).Float64()
//			}
//			if holding[`cost_open`] != nil {
//				position.EntryPrice, _ = holding[`cost_open`].(json.Number).Float64()
//			}
//			if holding[`profit_unreal`] != nil {
//				position.ProfitUnreal, _ = holding[`profit_unreal`].(json.Number).Float64()
//			}
//			if holding[`profit`] != nil {
//				position.ProfitReal, _ = holding[`profit`].(json.Number).Float64()
//			}
//			if holding[`position_margin`] != nil {
//				position.Margin, _ = holding[`position_margin`].(json.Number).Float64()
//			}
//			if holding[`direction`] != nil {
//				position.Direction, _ = holding[`direction`].(string)
//			}
//			if holding[`lever_rate`] != nil { // 杠杆倍数
//				position.LeverRate, _ = holding[`lever_rate`].(json.Number).Int64()
//			}
//			util.SocketInfo(fmt.Sprintf(`get huobiDB %s holding %f`, position.Direction, position.Holding))
//		}
//	}
//	return position
//}
//
//// 不适宜快速下单
//func placeOrderHuobiDM(key, secret string, order *model.Order,
//	orderSide, orderType, contractCode, symbol string, price, triggerPrice, size float64) {
//	if orderType != model.OrderTypeStop {
//		return
//	}
//	// special for huobiDM contract
//	triggerType := `ge`
//	direction := `buy`
//	offset := `close`
//	switch orderSide {
//	case model.OrderSideBuy:
//		triggerType = `ge`
//		direction = `buy`
//		offset = `open`
//	case model.OrderSideSell:
//		triggerType = `le`
//		direction = `sell`
//		offset = `open`
//	case model.OrderSideLiquidateShort:
//		triggerType = `ge`
//		direction = `buy`
//		offset = `close`
//		position := getHoldingHuobiDM(key, secret, symbol+model.OrderSideSell)
//		if position != nil {
//			holding := math.Abs(position.Holding)
//			util.Notice(fmt.Sprintf(`holding huobiDM size %f to %f`, size, holding))
//			if holding < size {
//				size = holding
//			}
//		} else {
//			size = 0
//		}
//	case model.OrderSideLiquidateLong:
//		triggerType = `le`
//		direction = `sell`
//		offset = `close`
//		position := getHoldingHuobiDM(key, secret, symbol+model.OrderSideBuy)
//		if position != nil {
//			holding := math.Abs(position.Holding)
//			util.Notice(fmt.Sprintf(`holding huobiDM size %f to %f`, size, holding))
//			if holding < size {
//				size = holding
//			}
//		} else {
//			size = 0
//		}
//	}
//	//account := model.AppAccounts.GetAccount(market, symbol)
//	//lever := `5`
//	//if account != nil {
//	//	lever = strconv.FormatInt(account.LeverRate, 10)
//	//}
//	_, strPrice := util.FormatNum(price, 2)
//	_, strTriggerPrice := util.FormatNum(triggerPrice, 2)
//	_, strAmount := util.FormatNum(math.Floor(size), 0)
//	param := map[string]interface{}{`contract_code`: contractCode, `trigger_type`: triggerType,
//		`trigger_price`: strTriggerPrice, `order_price`: strPrice, `volume`: strAmount,
//		`direction`: direction, `offset`: offset, `lever_rate`: `5`}
//	responseBody := SignedRequestHuobiDM(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_trigger_order`, param)
//	orderJson, err := util.NewJSON(responseBody)
//	if err == nil {
//		data := orderJson.Get(`data`).MustMap()
//		if data != nil {
//			order.OrderId = data[`order_id_str`].(string)
//		}
//	}
//}
//
////cancelOrderHuobiDM
//func _(key, secret, symbol, orderId string) (result bool, errCode, msg string) {
//	if strings.Contains(symbol, `_`) {
//		symbol = symbol[0:strings.Index(symbol, `_`)]
//	}
//	param := map[string]interface{}{`symbol`: symbol, `order_id`: orderId}
//	responseBody := SignedRequestHuobiDM(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_trigger_cancel`, param)
//	cancelJson, err := util.NewJSON(responseBody)
//	if err == nil {
//		successIds := cancelJson.GetPath(`data`, `successes`).MustString()
//		if strings.Contains(successIds, orderId) {
//			return true, ``, ``
//		}
//	}
//	return false, ``, ``
//}
//
////func cancelAllHuobiDM(contractCode string)  {
////	param := map[string]interface{}{`contract_code`: contractCode}
////	responseBody := SignedRequestHuobi(`POST`, `/api/v1/contract_trigger_cancel`, param)
////}
//
//func queryOpenTriggerOrderHuobiDM(key, secret, symbol, orderId string) (isWorking bool) {
//	if strings.Contains(symbol, `_`) {
//		symbol = symbol[0:strings.Index(symbol, `_`)]
//	}
//	data := map[string]interface{}{`symbol`: symbol}
//	responseBody := SignedRequestHuobiDM(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_trigger_openorders`, data)
//	orderJson, err := util.NewJSON(responseBody)
//	if err == nil {
//		items := orderJson.GetPath(`data`, `orders`).MustArray()
//		for _, item := range items {
//			value := item.(map[string]interface{})
//			if value[`order_id_str`] != nil && value[`order_id_str`] == orderId {
//				return true
//			}
//		}
//	}
//	return false
//}
//
//func queryHisTriggerOrderHuobiDM(key, secret, symbol, orderId string) (relatedOrderId string) {
//	if strings.Contains(symbol, `_`) {
//		symbol = symbol[0:strings.Index(symbol, `_`)]
//	}
//	data := map[string]interface{}{`symbol`: symbol, `trade_type`: `0`, `status`: `0`, `create_date`: `3`}
//	responseBody := SignedRequestHuobiDM(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_trigger_hisorders`, data)
//	orderJson, err := util.NewJSON(responseBody)
//	if err == nil {
//		items := orderJson.GetPath(`data`, `orders`).MustArray()
//		for _, item := range items {
//			value := item.(map[string]interface{})
//			if value[`order_id_str`] != nil && value[`order_id_str`] == orderId {
//				if value[`relation_order_id`] != nil {
//					return value[`relation_order_id`].(string)
//				}
//			}
//		}
//	}
//	return `-1`
//}
//
//// status 1准备提交 2准备提交 3已提交 4部分成交 5部分成交已撤单 6全部成交 7已撤单 11撤单中
//func queryOrderHuobiDM(key, secret, symbol, orderId string) (dealAmount, dealPrice float64, status string) {
//	if strings.Contains(symbol, `_`) {
//		symbol = symbol[0:strings.Index(symbol, `_`)]
//	}
//	data := map[string]interface{}{`symbol`: symbol, `order_id`: orderId}
//	responseBody := SignedRequestHuobiDM(key, secret, model.HuobiDM, `POST`, `/api/v1/contract_order_info`, data)
//	orderJson, err := util.NewJSON(responseBody)
//	if err == nil {
//		items := orderJson.Get(`data`).MustArray()
//		for _, item := range items {
//			value := item.(map[string]interface{})
//			if value[`order_id_str`] != nil && value[`order_id_str`].(string) == orderId {
//				if value[`trade_avg_price`] != nil {
//					dealPrice, _ = value[`trade_avg_price`].(json.Number).Float64()
//				}
//				if value[`trade_volume`] != nil {
//					dealAmount, _ = value[`trade_volume`].(json.Number).Float64()
//				}
//				if value[`status`] != nil {
//					intStatus, _ := value[`status`].(json.Number).Int64()
//					switch intStatus {
//					case 1, 2, 3, 4, 11:
//						status = model.CarryStatusWorking
//					case 5, 6:
//						status = model.CarryStatusSuccess
//					case 7:
//						status = model.CarryStatusFail
//					}
//				}
//				return
//			}
//		}
//	}
//	return 0, 0, model.CarryStatusFail
//}
//
//func querySetInstrumentsHuobiDM() {
//	account := model.AppConfig.GetAccounts(model.Huobi)[0]
//	responseBody := SignedRequestHuobiDM(account.Key, account.Secret, model.HuobiDM, http.MethodGet,
//		`/api/v1/contract_contract_info`, nil)
//	instrumentJson, err := util.NewJSON(responseBody)
//	if err == nil {
//		for _, item := range instrumentJson.Get(`data`).MustArray() {
//			future := item.(map[string]interface{})
//			if future[`contract_code`] != nil && future[`contract_type`] != nil {
//				api.setInstrument(model.HuobiDM, strings.ToLower(future[`symbol`].(string)),
//					future[`contract_type`].(string), future[`contract_code`].(string))
//			}
//		}
//	}
//}
//
//func getCandlesHuobiDM(key, secret, symbol, binSize string, start, end time.Time) (
//	candles map[string]*model.Candle) {
//	param := map[string]interface{}{`symbol`: symbol, `from`: strconv.FormatInt(start.Unix(), 10),
//		`to`: strconv.FormatInt(end.Unix(), 10)}
//	if binSize == `1d` {
//		param[`period`] = `1day`
//	}
//	candles = make(map[string]*model.Candle)
//	response := SignedRequestHuobiDM(key, secret, model.HuobiDM, `GET`, `/market/history/kline`, param)
//	//duration, _ := time.ParseDuration(`8h`)
//	candleJson, err := util.NewJSON(response)
//	if err == nil {
//		candleJsons := candleJson.Get(`data`).MustArray()
//		for _, value := range candleJsons {
//			item := value.(map[string]interface{})
//			candle := &model.Candle{Market: model.HuobiDM, Symbol: symbol, Period: binSize}
//			if item[`open`] != nil {
//				candle.PriceOpen, _ = item[`open`].(json.Number).Float64()
//			}
//			if item[`high`] != nil {
//				candle.PriceHigh, _ = item[`high`].(json.Number).Float64()
//			}
//			if item[`low`] != nil {
//				candle.PriceLow, _ = item[`low`].(json.Number).Float64()
//			}
//			if item[`close`] != nil {
//				candle.PriceClose, _ = item[`close`].(json.Number).Float64()
//			}
//			if item[`id`] != nil {
//				unixSeconds, _ := item[`id`].(json.Number).Int64()
//				candle.UTCDate = time.Unix(unixSeconds, 0).Format(time.RFC3339)[0:10]
//			}
//			candles[candle.UTCDate] = candle
//		}
//	}
//	return
//}
//
//func SignedRequestHuobiDM(key, secret, market, method, path string, data map[string]interface{}) []byte {
//	restUrl := restHuobiDM
//	if market == model.Huobi {
//		restUrl = restHuobi
//	}
//	param := map[string]interface{}{"AccessKeyId": key, "SignatureMethod": "HmacSHA256",
//		"SignatureVersion": "2", `Timestamp`: url.QueryEscape(time.Now().UTC().Format("2006-01-02T15:04:05"))}
//	strData := ``
//	if method == `GET` {
//		for i, value := range data {
//			param[i] = value
//		}
//	} else if method == `POST` && data != nil {
//		strData = string(util.JsonEncodeToByte(data))
//	}
//	strParam := util.ComposeParams(param)
//	toBeSign := fmt.Sprintf("%s\n%s\n%s\n%s", method, restUrl, path, strParam)
//	hash := hmac.New(sha256.New, []byte(secret))
//	hash.WriteServe([]byte(toBeSign))
//	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(hash.Sum(nil)))
//	param["Signature"] = sign
//	requestUrl := fmt.Sprintf(`https://%s%s?%s`, restUrl, path, util.ComposeParams(param))
//	headers := map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn",
//		"User-Agent": "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.71 Safari/537.36"}
//	responseBody, _ := util.HttpRequest(method, requestUrl, strData, headers, 60)
//	util.SocketInfo(fmt.Sprintf(`%s %s %s`, requestUrl, strData, string(responseBody)))
//	return responseBody
//}

package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const restHuobi = `api.huobi.pro`
const wsHuobi = `wss://api.huobi.pro/ws`
const wsStepHuobi = 50
const restHuobiFuture = `api.hbdm.vn`
const wsHuobiFuture = `wss://api.hbdm.vn/linear-swap-ws`

// spot：现货账户, margin：逐仓杠杆账户, otc：OTC 账户, point：点卡账户, super-margin：全仓杠杆账户, investment: C2C杠杆借出账户,
// borrow: C2C杠杆借入账户，矿池账户: minepool, ETF账户: etf, 抵押借贷账户: crypto-loans
const spotAccount = "spot"
const marginAccountHuobi = `super-margin`

var huobiAccountMap = make(map[string]map[string]string) //key-type-accountId
var huobiPositionMap = make(map[string]*Position)

type HuobiMessage struct {
	Ping   int    `json:"ping"`
	Ch     string `json:"ch"`
	Ts     int    `json:"ts"`
	Req    string `json:"req"`
	Rep    string `json:"rep"`
	Status string `json:"status"`
	Id     string `json:"id"`
	Tick   struct {
		SeqNum float64     `json:"seqNum"`
		Amount float64     `json:"amount"` // 成交量
		Count  int         `json:"count"`  // 成交笔数
		Open   float64     `json:"open"`   // 开盘价
		Close  float64     `json:"close"`  // 收盘价,当K线为最晚的一根时，是最新成交价
		Low    float64     `json:"low"`    // 最低价
		High   float64     `json:"high"`   // 最高价
		Vol    float64     `json:"vol"`    // 成交额, 即 sum(每一笔成交价 * 该笔的成交量)
		Bids   [][]float64 `json:"bids"`
		Asks   [][]float64 `json:"asks"`
	} `json:"tick"`
}

var subscribeHandlerHuobi = func(market string, connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	for _, v := range subscribes {
		subscribeMap := make(map[string]interface{})
		subscribeMap["id"] = strconv.Itoa(util.GetNow().Nanosecond())
		subscribeMap["sub"] = v
		subscribeMessage := util.JsonEncodeToByte(subscribeMap)
		if err = connection.WriteMessage(websocket.TextMessage, subscribeMessage); err != nil {
			util.SocketInfo(" huobi can not subscribe %s %s", v, err.Error())
		}
		util.Info(`huobi subscribed ` + string(subscribeMessage))
	}
	return err
}

func WsDepthServeHuobiSpot(environment *model.Environment, market string) (socketMap map[*websocket.Conn]bool, msgChans []chan struct{}, connectErr error) {
	wsHandler := func(event []byte) {
		res := util.UnGzip(event)
		responseJson, jsonErr := util.NewJSON(res)
		if jsonErr != nil {
			util.Notice(fmt.Sprintf(`wsHandler fail to NewJson huobiSpot %s`, jsonErr.Error()))
			return
		}
		if responseJson.Get(`ping`).MustInt() > 0 {
			pingMap := make(map[string]interface{})
			pingMap["pong"] = responseJson.Get(`ping`).MustInt()
			pingParams := util.JsonEncodeToByte(pingMap)
			if err := SendToAllTickerSockets(model.HuobiSpot, pingParams); err != nil {
				util.SocketInfo("huobi server ping client error " + err.Error())
			}
		} else {
			tickJson := responseJson.Get(`tick`)
			if tickJson.Interface() == nil {
				return
			}
			now := int(time.Now().UnixNano() / int64(time.Millisecond))
			symbol := tickJson.Get("symbol").MustString()
			//eg: market.xrpbtc.depth.step0 => xrp_btc 当下仅支持usdt交易
			splits := strings.Split(symbol, `.`)
			if len(splits) > 1 {
				splitLen := len(splits[1])
				if splits[1][splitLen-3:] == `usdt` {
					symbol = splits[1][0 : splitLen-3]
				}
			}
			if symbol != "" {
				symbol = strings.ReplaceAll(symbol, "_", "")
				bidAsk := model.BidAsk{Ts: responseJson.Get("ts").MustInt(), TsReceived: now, UpdateId: tickJson.Get("quoteTime").MustInt64(),
					Bids: []model.Tick{{Price: tickJson.Get("bid").MustFloat64(), Amount: tickJson.Get("bidSize").MustFloat64(),
						Market: model.HuobiSpot, Symbol: symbol}},
					Asks: []model.Tick{{Price: tickJson.Get("ask").MustFloat64(), Amount: tickJson.Get("askSize").MustFloat64(),
						Market: model.HuobiSpot, Symbol: symbol}}}
				haveOld, old := environment.GetBidAsk(symbol, model.HuobiSpot)
				if haveOld && old.UpdateId > bidAsk.UpdateId {
					return
				}
				if environment.SetBidAsk(symbol, model.HuobiSpot, &bidAsk) {
					funcHandlers := GetFunctions(model.HuobiSpot, symbol)
					if funcHandlers != nil {
						funcHandlers.Range(func(function, value interface{}) bool {
							setting := GetSetting(function.(string), model.HuobiSpot, symbol)
							if setting != nil && value != nil && value.(model.CarryHandler) != nil {
								go value.(model.CarryHandler)(setting, &bidAsk)
							}
							return true
						})
					}
				}
			}
		}
	}
	wsHandlerDM := func(event []byte) {
		res := util.UnGzip(event)
		responseJson, jsonErr := util.NewJSON(res)
		if jsonErr != nil {
			util.Notice(fmt.Sprintf(`fail to NewJson wsHandlerDM huobi %s`, jsonErr.Error()))
			return
		}
		if responseJson.Get(`ping`).MustInt() > 0 {
			pingMap := make(map[string]interface{})
			pingMap["pong"] = responseJson.Get(`ping`).MustInt()
			pingParams := util.JsonEncodeToByte(pingMap)
			if wsErr := SendToAllTickerSockets(model.HuobiSpot, pingParams); wsErr != nil {
				util.SocketInfo("HuobiFuture server ping client error " + wsErr.Error())
			}
		} else {
			tickJson := responseJson.Get(`tick`)
			if tickJson.Interface() == nil {
				return
			}
			symbol := responseJson.Get(`ch`).MustString()
			splits := strings.Split(symbol, `.`)
			if splits == nil || len(splits) <= 1 {
				return
			}
			symbol = strings.ToLower(splits[1])
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
			bidSuccess, bidAmount := model.ParseRealAmount(model.HuobiSpot, symbol, bidAmount)
			if !bidSuccess {
				return
			}
			askAmount, _ := ask[1].(json.Number).Float64()
			askPrice, _ := ask[0].(json.Number).Float64()
			askSuccess, askAmount := model.ParseRealAmount(model.HuobiSpot, symbol, askAmount)
			if !askSuccess {
				return
			}
			bidAsk.Bids = []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.HuobiSpot, Symbol: symbol}}
			bidAsk.Asks = []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.HuobiSpot, Symbol: symbol}}
			haveOld, old := environment.GetBidAsk(symbol, model.HuobiSpot)
			if haveOld && old.UpdateId > bidAsk.UpdateId {
				return
			}
			if environment.SetBidAsk(symbol, model.HuobiPerp, &bidAsk) {
				funcHandlers := GetFunctions(model.HuobiPerp, symbol)
				if funcHandlers != nil {
					funcHandlers.Range(func(function, value interface{}) bool {
						setting := GetSetting(function.(string), model.HuobiPerp, symbol)
						if setting != nil && value != nil && value.(model.CarryHandler) != nil {
							go value.(model.CarryHandler)(setting, &bidAsk)
						}
						return true
					})
				}
			}
		}
	}
	var spotSubscribes, futureSubscribes []interface{}
	subscribes := GetWSSubscribes(model.HuobiSpot, model.SubscribeTicker)
	for _, subscribe := range subscribes {
		if strings.Contains(subscribe.(string), "-") {
			futureSubscribes = append(futureSubscribes, subscribe)
		} else {
			spotSubscribes = append(spotSubscribes, subscribe)
		}
	}
	socketMap = make(map[*websocket.Conn]bool)
	msgChans = make([]chan struct{}, 0)
	spotSockets, channels, channelErr := WebSocketClient(model.HuobiSpot, wsHuobi, spotSubscribes, subscribeHandlerHuobi, wsHandler, wsStepHuobi)
	dmSockets, dmChannels, dmChannelErr := WebSocketClient(model.HuobiSpot, wsHuobiFuture, futureSubscribes, subscribeHandlerHuobi, wsHandlerDM, wsStepHuobi)
	if channelErr == nil {
		msgChans = append(msgChans, channels...)
		for conn, b := range spotSockets {
			socketMap[conn] = b
		}
	}
	if dmChannelErr == nil {
		msgChans = append(msgChans, dmChannels...)
		for conn, b := range dmSockets {
			socketMap[conn] = b
		}
	}
	environment.SocketsTick.Store(market, socketMap)
	environment.MsgChanTick.Store(market, msgChans)
	return
}

func getMarketsHuobiSpot(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	spotResponseBody := SignedRequestHuobi(key, secret, http.MethodGet, restHuobi, `/v1/common/symbols`, nil)
	spotSymbolsJson, err := util.NewJSON(spotResponseBody)
	if err == nil && spotSymbolsJson != nil && strings.ToLower(spotSymbolsJson.Get(`status`).MustString()) == `ok` {
		items, _ := spotSymbolsJson.Get("data").Array()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value["symbol"] == nil || value["api-trading"].(string) == "disabled" || value["quote-currency"].(string) != "usdt" {
				continue
			}
			marketInfo := &model.MarketInfo{Market: model.HuobiSpot, Name: value["symbol"].(string)}
			if value["price-precision"] != nil {
				priceDecimal, _ := value["price-precision"].(json.Number).Int64()
				marketInfo.PriceDecimal = int(priceDecimal)
				marketInfo.PriceIncrement = 1 / math.Pow10(int(priceDecimal))
			}
			if value["limit-order-min-order-amt"] != nil {
				marketInfo.SizeMin, _ = value["limit-order-min-order-amt"].(json.Number).Float64()
			}
			amountPrecision, _ := value["amount-precision"].(json.Number).Int64()
			marketInfo.SizeIncrement = 1 / math.Pow10(int(amountPrecision))
			marketInfo.MoneyMin = 10
			marketInfos[marketInfo.Name] = marketInfo
		}
	} else {
		time.Sleep(time.Minute * 5)
		return getMarketsHuobiSpot(key, secret)
	}
	futureResponseBody := SignedRequestHuobi(key, secret, http.MethodGet, restHuobiFuture, `/linear-swap-api/v1/swap_contract_info`, nil)
	futureSymbolsJson, futureErr := util.NewJSON(futureResponseBody)
	if futureErr == nil && futureSymbolsJson != nil && strings.ToLower(futureSymbolsJson.Get(`status`).MustString()) == `ok` {
		items, _ := futureSymbolsJson.Get("data").Array()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value["support_margin_mode"].(string) != "all" && value["support_margin_mode"].(string) != "cross" {
				continue
			}
			marketInfo := &model.MarketInfo{Market: model.HuobiSpot, Name: strings.ToLower(value["contract_code"].(string))}
			if value["symbol"] != nil {
				marketInfo.CTCurrency = strings.ToLower(value["symbol"].(string))
			}
			if value["contract_size"] != nil {
				marketInfo.CTValue, _ = value["contract_size"].(json.Number).Float64()
			}
			if value["price_tick"] != nil {
				marketInfo.PriceIncrement, _ = value["price_tick"].(json.Number).Float64()
				marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
			}
			marketInfo.SizeIncrement = 1
			marketInfo.SizeMin = 1
			marketInfos[marketInfo.Name] = marketInfo
		}
	}
	return marketInfos
}

func SignedRequestHuobi(key, secret, method, host, path string, data map[string]interface{}) []byte {
	param := map[string]interface{}{"AccessKeyId": key, "SignatureMethod": "HmacSHA256",
		"SignatureVersion": "2", `Timestamp`: url.QueryEscape(time.Now().UTC().Format("2006-01-02T15:04:05"))}
	strData := ``
	if method == `GET` {
		for i, value := range data {
			param[i] = value
		}
	} else if method == `POST` && data != nil {
		strData = string(util.JsonEncodeToByte(data))
	}
	strParam := util.ComposeParams(param)
	toBeSign := fmt.Sprintf("%s\n%s\n%s\n%s", method, host, path, strParam)
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(toBeSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(hash.Sum(nil)))
	param["Signature"] = sign
	requestUrl := fmt.Sprintf(`https://%s%s?%s`, host, path, util.ComposeParams(param))
	headers := map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn",
		"User-Agent": "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.71 Safari/537.36"}
	responseBody, _ := util.HttpRequest(method, requestUrl, strData, headers, 60)
	util.SocketInfo(fmt.Sprintf(`%s %s %s`, requestUrl, strData, string(responseBody)))
	return responseBody
}

func GetAccountIdsHuobi(key, secret string) (err error) {
	responseBody := SignedRequestHuobi(key, secret, `GET`, restHuobi, "/v1/account/accounts", nil)
	util.SocketInfo(`get huobi accounts: ` + string(responseBody))
	accountJson, accountErr := util.NewJSON(responseBody)
	if accountErr == nil {
		accounts, _ := accountJson.Get("data").Array()
		for _, value := range accounts {
			account := value.(map[string]interface{})
			typeName := account["type"].(string)
			//accountIds[typeName] = account["id"].(json.Number).String()
			if huobiAccountMap[key] == nil {
				huobiAccountMap[key] = make(map[string]string)
			}
			if huobiAccountMap[key][typeName] == "" {
				huobiAccountMap[key][typeName] = account["id"].(json.Number).String()
			}
		}
	}
	return accountErr
}

// orderType: buy-market：市价买, sell-market：市价卖, buy-limit：限价买, sell-limit：限价卖
// huobi中amount在市价买单中指的是右侧的钱
func placeOrderHuobiSpot(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	postData := make(map[string]interface{})
	if symbol[len(symbol)-5:] == `-usdt` { //合约
		offset := "open"
		if huobiPositionMap[symbol] == nil ||
			(orderSide == model.OrderSideBuy && amount > math.Abs(huobiPositionMap[symbol].DirectionDetail[model.OrderSideSell])) ||
			(orderSide == model.OrderSideSell && amount > huobiPositionMap[symbol].DirectionDetail[model.OrderSideBuy]) { //没有持仓信息或者对手仓位小于当前数量，直接开仓
			offset = "open"
		} else {
			offset = "close"
		}
		postData["lever_rate"] = 5
		postData["contract_code"] = symbol
		postData["direction"] = orderSide
		postData["offset"] = offset
		if orderType == model.OrderTypeLimit {
			priceFuture, decimalFuture := model.FormatPrice(model.HuobiSpot, symbol, price)
			order.Price = priceFuture
			priceStrFuture := util.CutTailZero(strconv.FormatFloat(priceFuture, 'f', decimalFuture, 64))
			postData["price"] = priceStrFuture
			postData["order_price_type"] = "limit"
		} else if orderType == model.OrderTypeMarket {
			postData["order_price_type"] = "opponent"
		}
		v, _ := util.LoadSyncMap(model.MarketInfos, model.HuobiSpot, symbol)
		_, _, coin := model.GetCoinFromDialect(model.HuobiPerp, symbol)
		if v == nil || v.(model.MarketInfo).SizeIncrement == 0 || v.(model.MarketInfo).CTValue == 0 || v.(model.MarketInfo).CTCurrency != coin {
			return
		}
		postData["volume"] = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.HuobiSpot, symbol, amount, price, false)))
		responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobiFuture, "/linear-swap-api/v1/swap_cross_order", postData)
		orderJson, err := util.NewJSON(responseBody)
		if err == nil {
			status, _ := orderJson.Get("status").String()
			if status == "ok" {
				huobiPositionMap[symbol] = nil
				order.OrderId, _ = orderJson.Get("data").Get("order_id_str").String()
				order.Status = model.CarryStatusWorking
			} else if status == "error" {
				order.ErrCode, _ = orderJson.Get("err-code").String()
				order.Status = model.CarryStatusFail
			}
		}
		util.Notice(fmt.Sprintf(`[挂单huobi] %s side: %s type: %s price: %f amount: %f order id %s 返回%s`,
			symbol, orderSide, orderType, price, amount, order.OrderId, string(responseBody)))
	} else { //现货
		if orderSide == model.OrderSideBuy && orderType == model.OrderTypeLimit {
			postData["type"] = `buy-limit`
		} else if orderSide == model.OrderSideBuy && orderType == model.OrderTypeMarket {
			postData["type"] = `buy-market`
		} else if orderSide == model.OrderSideSell && orderType == model.OrderTypeLimit {
			postData["type"] = `sell-limit`
		} else if orderSide == model.OrderSideSell && orderType == model.OrderTypeMarket {
			postData["type"] = `sell-market`
		} else {
			util.Notice(fmt.Sprintf(`[parameter error] order side: %s order type: %s`, orderSide, orderType))
		}
		if huobiAccountMap[key] == nil || huobiAccountMap[key][spotAccount] == "" {
			_ = GetAccountIdsHuobi(key, secret)
		}
		postData["account-id"] = huobiAccountMap[key][spotAccount]
		postData["amount"] = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.HuobiSpot, symbol, amount, price, false)))
		postData["symbol"] = symbol
		if orderType == model.OrderTypeLimit {
			priceSpot, decimalSpot := model.FormatPrice(model.HuobiSpot, symbol, price)
			postData["price"] = util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
			order.Price = priceSpot
		}
		responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobi, "/v1/order/orders/place", postData)
		orderJson, err := util.NewJSON(responseBody)
		if err == nil {
			order.OrderId, _ = orderJson.Get("data").String()
		}
		util.Notice(fmt.Sprintf(`[挂单huobi] %s side: %s type: %s price: %f amount: %f order id %s 返回%s`,
			symbol, orderSide, orderType, price, amount, order.OrderId, string(responseBody)))
	}
}

func cancelOrdersHuobiSpot(key, secret, symbol string) (result bool) {
	postData := make(map[string]interface{})
	var host, path string
	if strings.Contains(symbol, "-usdt") {
		host = restHuobiFuture
		path = "/linear-swap-api/v1/swap_cross_cancelall"
		postData[`contract_code`] = symbol
	} else {
		host = restHuobi
		path = "/v1/order/orders/batchCancelOpenOrders"
		postData[`symbol`] = symbol
	}
	responseBody := SignedRequestHuobi(key, secret, `POST`, host, path, postData)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		status, _ := orderJson.Get("status").String()
		if status == "ok" {
			return true
		} else if status == "error" {
			//errCode, _ = orderJson.Get("err-code").String()
			//msg, _ = orderJson.Get(`err-msg`).String()
			return false
		}
	} else {
		return false
	}
	return false
}

func queryOrderHuobiSpot(key, secret, orderId string) (order *model.Order) {
	path := fmt.Sprintf("/v1/order/orders/%s", orderId)
	responseBody := SignedRequestHuobi(key, secret, `GET`, restHuobi, path, nil)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		order = &model.Order{Market: model.HuobiSpot}
		status, _ := orderJson.GetPath("data", "state").String()
		order.Status = model.GetOrderStatus(model.HuobiSpot, status)
		str, _ := orderJson.GetPath("data", "field-amount").String()
		if str != "" {
			dealAmount, _ := strconv.ParseFloat(str, 64)
			order.DealAmount = dealAmount
		}
		strDealPrice, _ := orderJson.GetPath(`data`, `price`).String()
		if strDealPrice != `` {
			dealPrice, _ := strconv.ParseFloat(strDealPrice, 64)
			order.DealPrice = dealPrice
		}
	}
	return
}

//func parseBalanceHuobi(key string, data map[string]interface{}, market string) (balance *model.Balance) {
//	if data == nil || data[`id`] == nil {
//		return nil
//	}
//	balance = &model.Balance{AccountId: key, Market: market}
//	balance.ID = model.HuobiSpot + `_` + data[`id`].(json.Number).String()
//	if data[`type`] != nil {
//		if data[`type`].(string) == `deposit` {
//			balance.Action = 1
//		} else if data[`type`].(string) == `withdraw` {
//			balance.Action = -1
//		}
//	}
//	if data[`currency`] != nil {
//		balance.Coin = strings.ToLower(data[`currency`].(string))
//	}
//	if data[`amount`] != nil {
//		balance.Amount, _ = data[`amount`].(json.Number).Float64()
//	}
//	if data[`address`] != nil {
//		balance.Address, _ = data[`address`].(string)
//	}
//	if data[`fee`] != nil {
//		balance.Fee = data[`fee`].(json.Number).String()
//	}
//	if data[`state`] != nil {
//		balance.Status, _ = data[`state`].(string)
//	}
//	if data[`updated-at`] != nil {
//		seconds, _ := data[`updated-at`].(json.Number).Int64()
//		balance.BalanceTime = time.Unix(seconds/1000, 0)
//		fmt.Println(balance.BalanceTime.String())
//	}
//	return balance
//}

//func getTransferHuobi(key, secret string) (balances []*model.Balance) {
//	data := map[string]interface{}{`type`: `deposit`}
//	response := SignedRequestHuobi(key, secret, http.MethodGet, restHuobi, `/v1/query/deposit-withdraw`, data)
//	util.SocketInfo(`query huobi deposit: ` + string(response))
//	responseJson, err := util.NewJSON(response)
//	if err == nil && responseJson != nil && responseJson.Get(`data`) != nil {
//		items := responseJson.Get(`data`).MustArray()
//		for _, item := range items {
//			balance := parseBalanceHuobi(key, item.(map[string]interface{}), model.HuobiSpot)
//			if balance != nil {
//				balances = append(balances, balance)
//			}
//		}
//	}
//	data = map[string]interface{}{`type`: `withdraw`}
//	response = SignedRequestHuobi(key, secret, http.MethodGet, restHuobi, `/v1/query/deposit-withdraw`, data)
//	util.SocketInfo(`query huobi withdraw: ` + string(response))
//	responseJson, err = util.NewJSON(response)
//	if err == nil && responseJson != nil && responseJson.Get(`data`) != nil {
//		items := responseJson.Get(`data`).MustArray()
//		for _, item := range items {
//			balance := parseBalanceHuobi(key, item.(map[string]interface{}), model.HuobiSpot)
//			if balances != nil {
//				balances = append(balances, balance)
//			}
//		}
//	}
//	return balances
//}
//
//func transferHuobi(key string, secret string, transferType string, amount float64) {
//	postData := make(map[string]interface{})
//	postData["currency"] = "usdt"
//	postData["amount"], _ = strconv.ParseFloat(fmt.Sprintf("%.8f", amount), 64)
//	postData["margin-account"] = "usdt"
//	if transferType == "MAIN_UMFUTURE" {
//		postData["from"] = "spot"
//		postData["to"] = "linear-swap"
//	} else {
//		postData["from"] = "linear-swap"
//		postData["to"] = "spot"
//	}
//	SignedRequestHuobi(key, secret, http.MethodPost, restHuobi, "/v2/account/transfer", postData)
//}

func getPositionsHuobiPerp(key string, secret string) (success bool, positions []*Position, accountValue, availableU float64) {
	postData := make(map[string]interface{})
	postData["margin_account"] = "USDT"
	response := SignedRequestHuobi(key, secret, http.MethodPost, restHuobiFuture, "/linear-swap-api/v1/swap_cross_account_position_info", postData)
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson == nil || strings.ToLower(responseJson.Get(`status`).MustString()) != `ok` {
		time.Sleep(time.Second * 2)
		util.SocketInfo(`fail to get HuobiFuture balance`)
		return getPositionsHuobiPerp(key, secret)
	}
	positions = make([]*Position, 0)
	contracts := responseJson.Get(`data`).Get(`positions`).MustArray()
	accountValue = responseJson.Get(`data`).Get(`margin_balance`).MustFloat64()
	availableU = responseJson.Get(`data`).Get(`margin_available`).MustFloat64()
	positionMap := make(map[string]*Position)
	for _, contract := range contracts {
		item := contract.(map[string]interface{})
		if item[`contract_code`] == nil {
			continue
		}
		currency := strings.ToLower(item[`contract_code`].(string))
		if positionMap[currency] == nil {
			positionMap[currency] = &Position{Market: model.HuobiSpot, Ts: util.GetNowUnixMillion(),
				Currency: currency, DirectionDetail: make(map[string]float64)}
		}
		if item[`cost_open`] != nil {
			positionMap[currency].EntryPrice, _ = item[`cost_open`].(json.Number).Float64()
		}
		if item[`volume`] != nil && item[`direction`] != nil {
			direction := item[`direction`].(string)
			amount, _ := item[`volume`].(json.Number).Float64()
			_, realAmount := model.ParseRealAmount(model.HuobiSpot, currency, amount)
			if direction == model.OrderSideSell {
				realAmount = realAmount * -1
			}
			positionMap[currency].DirectionDetail[direction] = realAmount
			positionMap[currency].Direction = direction
			positionMap[currency].Holding = positionMap[currency].Holding + realAmount
		}
		if item[`profit_unreal`] != nil {
			positionMap[currency].ProfitUnreal, _ = item[`profit_unreal`].(json.Number).Float64()
		}
	}
	for _, position := range positionMap {
		positions = append(positions, position)
	}
	huobiPositionMap = positionMap
	return true, positions, accountValue, availableU
}

// 资产账户 getBalanceHuobi
func getBalanceHuobiSpot(key string, secret string) (success bool, balances []*model.Balance) {
	if huobiAccountMap[key] == nil || huobiAccountMap[key][spotAccount] == "" {
		_ = GetAccountIdsHuobi(key, secret)
	}
	accountId := huobiAccountMap[key][marginAccountHuobi]
	path := fmt.Sprintf("/v1/account/accounts/%s/balance", accountId)
	response := SignedRequestHuobi(key, secret, http.MethodGet, restHuobi, path, nil)
	responseJson, err := util.NewJSON(response)
	if err == nil {
		balanceArray := responseJson.GetPath(`data`, `list`).MustArray()
		balanceMap := make(map[string]*model.Balance)
		for _, item := range balanceArray {
			value := item.(map[string]interface{})
			//trade: 交易余额，frozen: 冻结余额, loan: 待还借贷本金, interest: 待还借贷利息, lock: 锁仓, bank: 储蓄
			if value[`currency`] == nil || value[`type`] == nil || value[`balance`] == nil {
				continue
			}
			coin := value[`currency`].(string)
			balance := balanceMap[coin]
			if balance == nil {
				balance = &model.Balance{AccountId: accountId, BalanceTime: util.GetNow(), Market: model.HuobiSpot, Coin: coin}
				balanceMap[coin] = balance
			}
			switch value["type"] {
			case `trade`: // 此处未计算可以借入的金额
				balance.AvailableWithBorrow, _ = strconv.ParseFloat(value[`balance`].(string), 64)
			case `frozen`:
				balance.FrozenAmount, _ = strconv.ParseFloat(value[`balance`].(string), 64)
			case `loan`:
				balance.Borrow, _ = strconv.ParseFloat(value[`balance`].(string), 64)
				//balance.Borrow *= -1 // 借时为负数
			}
		}
		balances = make([]*model.Balance, 0)
		for _, balance := range balanceMap {
			balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
			priceGet, bidAsk := model.AppEnvironment.GetBidAsk(balance.Coin+`usdt`, model.HuobiSpot)
			if priceGet {
				balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
			}
			balance.ID = fmt.Sprintf(`%s_%s_%s`, balance.Market, balance.Coin, balance.BalanceTime.String()[0:10])
			balances = append(balances, balance)
		}
	} else {
		time.Sleep(time.Second * 2)
		util.SocketInfo(`fail to refresh balance huobi`)
		return getBalanceHuobiSpot(key, secret)
	}
	return true, balances
}

package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"net/http"
	"sort"
	"strconv"
	"strings"
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
			time.Sleep(time.Minute * 5)
			for _, subscribe := range subscribes {
				_, _, coin := model.GetCoinFromDialect(model.BybitSpot, subscribe.(string))
				standardSymbol := coin + model.UniStandardTail[model.MarketTypeSpot]
				_, bidAsk := model.AppMarkets.GetBidAsk(standardSymbol, model.BybitSpot)
				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 120000 {
					subscribeMessage := fmt.Sprintf(
						`{"topic":"depth","event":"sub","params":{"symbol":"%s","binary":false}}`, subscribe)
					if bidAsk != nil {
						util.Notice(`maintain bybitspot timeout %s %d %d`,
							standardSymbol, time.Now().UnixMilli()-int64(bidAsk.Ts), bidAsk.Ts)
					}
					if bybitSpotSubConnection[standardSymbol] != nil {
						if err := SendToConnection(model.BybitSpot, bybitSpotSubConnection[standardSymbol],
							[]byte(subscribeMessage)); err != nil {
							util.SocketInfo("bybitSpot can not resubscribe " + err.Error())
						}
					} else {
						util.Notice(`bybitSpot can not get connection for %s`, standardSymbol)
					}
					util.Notice(`send resubscribe %s %s`, model.BybitSpot, subscribeMessage)
				}
				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 180000 {
					SetRequireReset(model.BybitSpot, true)
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
		_, _, coin := model.GetCoinFromDialect(model.BybitSpot, subscribe.(string))
		standardSymbol := coin + model.UniStandardTail[model.MarketTypeSpot]
		bybitSpotSubConnection[standardSymbol] = connection
		util.Notice(`set bybitspot connection %s`, standardSymbol)
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
		//util.Notice(`get bybitspot tick %s`, string(event))
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
						setting := model.GetSetting(function, model.BybitSpot, symbol)
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

func SignedRequestBybitSpot(key, secret, method, path string, body map[string]interface{}) []byte {
	if body == nil {
		body = make(map[string]interface{})
	}
	if body[`symbol`] != nil && len(body[`symbol`].(string)) > 0 {
		_, _, _, dialectSymbol := model.GetFromStandard(model.BybitSpot, body[`symbol`].(string))
		body[`symbol`] = dialectSymbol
	}
	if body[`symbolId`] != nil && len(body[`symbolId`].(string)) > 0 {
		_, _, _, dialectSymbol := model.GetFromStandard(model.BybitSpot, body[`symbolId`].(string))
		body[`symbolId`] = dialectSymbol
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
	headers := map[string]string{`api_key`: key, `sign`: sign} //, "Content-Type": "application/json"}
	//if method == http.MethodGet {
	uri = uri + `?` + paramStr
	//}
	responseBody, _ := util.HttpRequest(method, uri, ``, headers, 60)
	util.SocketInfo(fmt.Sprintf(`bybitSpot key %s request %s %s body %v return %s`,
		key, uri, method, body, string(responseBody)))
	return responseBody
}

func parseTickBybitSpot(data map[string]interface{}) (symbol string, bidAsk *model.BidAsk) {
	if data == nil {
		return ``, nil
	}
	bidAsk = &model.BidAsk{TsReceived: int(util.GetNowUnixMillion()), Bids: model.Ticks{}, Asks: model.Ticks{}}
	if data[`s`] != nil {
		_, _, coin := model.GetCoinFromDialect(model.BybitSpot, data[`s`].(string))
		symbol = coin + model.UniStandardTail[model.MarketTypeSpot]
	}
	if data[`t`] != nil {
		ts, _ := data[`t`].(json.Number).Int64()
		bidAsk.Ts = int(ts)
	}
	if data[`b`] != nil {
		items := data[`b`].([]interface{})
		for _, item := range items {
			if len(item.([]interface{})) != 2 {
				continue
			}
			price, _ := strconv.ParseFloat(item.([]interface{})[0].(string), 64)
			amount, _ := strconv.ParseFloat(item.([]interface{})[1].(string), 64)
			bidAsk.Bids = append(bidAsk.Bids, model.Tick{
				Side: model.OrderSideBuy, Market: model.BybitSpot, Symbol: symbol, Price: price, Amount: amount})
		}
		sort.Sort(sort.Reverse(bidAsk.Bids))
	}
	if data[`a`] != nil {
		items := data[`a`].([]interface{})
		for _, item := range items {
			if len(item.([]interface{})) != 2 {
				continue
			}
			price, _ := strconv.ParseFloat(item.([]interface{})[0].(string), 64)
			amount, _ := strconv.ParseFloat(item.([]interface{})[1].(string), 64)
			bidAsk.Asks = append(bidAsk.Asks, model.Tick{
				Side: model.OrderSideSell, Market: model.BybitSpot, Symbol: symbol, Price: price, Amount: amount})
		}
		sort.Sort(bidAsk.Asks)
	}
	return
}

func getMarketsBybitSpot(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	response := SignedRequestBybitSpot(key, secret, http.MethodGet, `/spot/v1/symbols`, nil)
	marketInfos = make(map[string]*model.MarketInfo)
	marketJson, err := util.NewJSON(response)
	if err != nil || marketJson.Get(`ret_code`) == nil || marketJson.Get(`ret_code`).MustInt() != 0 {
		time.Sleep(time.Second * 2)
		return getMarketsBybitSpot(key, secret)
	} else {
		items, _ := marketJson.Get(`result`).Array()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value[`quoteCurrency`] == nil || value[`quoteCurrency`].(string) != `USDT` {
				continue
			}
			marketInfo := &model.MarketInfo{Market: model.BybitSpot}
			if value[`baseCurrency`] != nil {
				marketInfo.CTCurrency = value[`baseCurrency`].(string)
				marketInfo.Name = marketInfo.CTCurrency + model.UniStandardTail[model.MarketTypeSpot]
				marketInfos[marketInfo.Name] = marketInfo
			}
			if value[`basePrecision`] != nil {
				marketInfo.SizeIncrement, _ = strconv.ParseFloat(value[`basePrecision`].(string), 64)
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

func parseOrderBybitSpot(order *model.Order, item map[string]interface{}) {
	if item == nil {
		return
	}
	if item[`orderId`] != nil {
		order.OrderId = item[`orderId`].(string)
	}
	if item[`symbolName`] != nil {
		_, _, coin := model.GetCoinFromDialect(model.BybitSpot, item[`symbolName`].(string))
		order.Symbol = coin + model.UniStandardTail[model.MarketTypeSpot]
	}
	if item[`side`] != nil {
		order.OrderSide = strings.ToLower(item[`side`].(string))
	}
	if item[`type`] != nil {
		order.OrderType = strings.ToLower(item[`type`].(string))
	}
	if item[`origQty`] != nil {
		order.Amount, _ = strconv.ParseFloat(item[`origQty`].(string), 64)
	}
	if item[`price`] != nil {
		order.Price, _ = strconv.ParseFloat(item[`price`].(string), 64)
	}
	if item[`executedQty`] != nil {
		order.DealAmount, _ = strconv.ParseFloat(item[`executedQty`].(string), 64)
	}
	if item[`transactTime`] != nil {
		order.OrderTime, _ = time.Parse(time.RFC3339, item[`transactTime`].(string))
	}
	if item[`status`] != nil {
		order.Status = model.GetOrderStatus(model.BybitSpot, item[`status`].(string))
	}
	if order.Status != model.CarryStatusSuccess && order.Status != model.CarryStatusFail {
		order.Status = model.CarryStatusWorking
	}
	if order.DealAmount > 0 && order.DealPrice == 0 {
		order.DealPrice = order.Price
	}
	return
}

func placeOrdersBybitSpot(order *model.Order, key, secret, orderSide, orderType, timeInForce, symbol string,
	price, amount float64) {
	postData := make(map[string]interface{})
	path := `/spot/v1/order`
	postData[`symbol`] = symbol
	formattedAmount := model.GetAmountInMarket(model.BybitSpot, symbol, amount, price)
	postData["qty"] = util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	postData["side"] = strings.ToUpper(orderSide)
	postData["type"] = strings.ToUpper(orderType)
	if timeInForce == `` {
		timeInForce = `GTC`
	}
	postData[`timeInForce`] = timeInForce
	if orderType != model.OrderTypeMarket && orderType != model.OrderTypeStop {
		formattedPrice, decimal := model.FormatPrice(model.BybitSpot, symbol, orderSide, price)
		postData[`price`] = util.CutTailZero(strconv.FormatFloat(formattedPrice, 'f', decimal, 64))
	}
	response := SignedRequestBybitSpot(key, secret, http.MethodPost, path, postData)
	orderJson, err := util.NewJSON(response)
	if err == nil {
		orderJson = orderJson.Get(`result`)
		if orderJson != nil {
			parseOrderBybitSpot(order, orderJson.MustMap())
		}
	}
}

func cancelOrdersBybitSpot(key, secret, symbol string) bool {
	path := `/spot/order/batch-cancel`
	response := SignedRequestBybitSpot(key, secret, http.MethodDelete, path, map[string]interface{}{`symbol`: symbol})
	cancelJson, err := util.NewJSON(response)
	if err == nil {
		if cancelJson.Get(`ret_code`).MustInt() == 0 {
			return true
		}
	}
	return false
}

func cancelOrderBybitSpot(key, secret, symbol, orderId string) (result bool, errCode, msg string) {
	postData := make(map[string]interface{})
	postData[`orderId`] = orderId
	postData[`symbolId`] = symbol
	response := SignedRequestBybitSpot(key, secret, http.MethodDelete, `/spot/v1/order/fast`, postData)
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
		//		parseOrderBybitSpot(order, item)
		//	}
		//}
		return
	}
	return false, ``, ``
}

func queryOrderBybitSpot(key, secret, orderId string) (order *model.Order) {
	response := SignedRequestBybitSpot(key, secret, http.MethodGet, `/spot/v1/order`, map[string]interface{}{`orderId`: orderId})
	orderJson, err := util.NewJSON(response)
	if err == nil {
		orderJson = orderJson.GetPath(`result`)
		if orderJson == nil {
			return nil
		}
		order = &model.Order{Market: model.BybitPerp, Status: model.CarryStatusFail}
		parseOrderBybitSpot(order, orderJson.MustMap())
	}
	return nil
}

func getBalanceBybitSpot(key, secret string) (success bool, balances []*model.Balance) {
	response := SignedRequestBybitSpot(key, secret, http.MethodGet, `/spot/v1/account`, nil)
	balanceJson, err := util.NewJSON(response)
	if err != nil || balanceJson == nil || balanceJson.Get(`ret_code`).MustInt() != 0 {
		util.SocketInfo(`fail to get bybitspot balance`)
		time.Sleep(time.Second * 2)
		return getBalanceBybitSpot(key, secret)
	} else {
		balancesArray := balanceJson.GetPath(`result`, `balances`).MustArray()
		balances = []*model.Balance{}
		success = true
		for _, item := range balancesArray {
			value := item.(map[string]interface{})
			balance := &model.Balance{
				Amount:       0,
				FrozenAmount: 0,
				Coin:         "",
				Market:       model.BybitSpot,
				UsdValue:     0,
			}
			if value[`coin`] != nil {
				balance.Coin = value[`coin`].(string)
			}
			if value[`total`] != nil {
				balance.Amount, _ = strconv.ParseFloat(value[`total`].(string), 64)
			}
			if value[`free`] != nil {
				balance.AvailableWithBorrow, _ = strconv.ParseFloat(value[`free`].(string), 64)
			}
			if value[`locked`] != nil {
				balance.FrozenAmount, _ = strconv.ParseFloat(value[`locked`].(string), 64)
			}
			priceGet, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.BybitSpot)
			if priceGet {
				balance.Price = bidAsk.Bids[0].Price
				balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
			}
			balances = append(balances, balance)
		}
	}
	return success, balances
}

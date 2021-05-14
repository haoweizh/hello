package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var lastTickId = make(map[string]int64) // symbol - int64
var lastTradeTime = make(map[string]int64)

var subscribeHandlerBinance = func(subscribes []interface{}, subType string) error {
	var err error = nil
	for _, subscribe := range subscribes {
		subMsg := fmt.Sprintf(`{"method": "SUBSCRIBE","params":["%s"],"id": %d}`, subscribe, int(rand.Float64()*10000))
		if err = sendToWs(model.Binance, []byte(subMsg)); err != nil {
			util.SocketInfo("binance can not subscribe " + err.Error())
		}
	}
	return err
}

func WsDepthServeBinance(markets *model.Markets, orderHandler OrderHandler, requestUrl string) (chan struct{}, error) {
	wsHandler := func(channelKey string, event []byte, orderHandler OrderHandler) {
		json, err := util.NewJSON(event)
		if err != nil {
			util.SocketInfo(`binance fail to unmarshal json ` + err.Error())
			return
		}
		util.SocketInfo("收到币安ws信息：", json)
		subscribe, _ := json.Get("stream").String()
		symbol := model.GetSymbol(model.Binance, subscribe) //当前获取到的币种的推送
		var findSettingSymbol string                        //用来寻找配置的币种 对应setting的Symbol
		if symbol != "" {
			json = json.Get("data")
			if json == nil {
				return
			}
			bidAsk := model.BidAsk{}
			var bids, asks []interface{}

			tickId, _ := json.Get(`lastUpdateId`).Int64()
			depthUpdate, _ := json.Get(`e`).String()
			if tickId > 0 { //存在lastUpdateId字段 表示是现货深度推送
				if tickId > lastTickId[symbol] {
					lastTickId[symbol] = tickId
					bidAsk.Ts = int(util.GetNowUnixMillion())
					bidAsk.TsReceived = int(util.GetNowUnixMillion())
				} else {
					return
				}

				bidArray, _ := json.Get(`bids`).Array()
				bids = bidArray
				askArray, _ := json.Get(`asks`).Array()
				asks = askArray

				if strings.Contains(symbol, "USDT") {
					findSettingSymbol = strings.ReplaceAll(symbol, "USDT", "-PERP")
				}
			} else if depthUpdate == "depthUpdate" { //存在depthUpdate字段 表示是合约深度推送
				nowTradeTime, _ := json.Get(`T`).Int64()
				if nowTradeTime <= 0 || nowTradeTime < lastTradeTime[symbol] {
					return
				}
				lastTradeTime[symbol] = nowTradeTime
				bidAsk.Ts = int(util.GetNowUnixMillion())
				bidAsk.TsReceived = int(util.GetNowUnixMillion())

				bidArray, _ := json.Get(`b`).Array()
				bids = bidArray
				askArray, _ := json.Get(`a`).Array()
				asks = askArray
				symbol = json.Get(`s`).MustString()
				if strings.Contains(symbol, "USDT") {
					symbol = strings.ReplaceAll(symbol, "USDT", "-PERP")
				}

				findSettingSymbol = symbol
			}

			bidAsk.Bids = make([]model.Tick, len(bids))
			for i, value := range bids {
				if len(value.([]interface{})) < 2 {
					return
				}
				price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
				amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
				bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount}
			}
			bidAsk.Asks = make([]model.Tick, len(asks))
			for i, value := range asks {
				if len(value.([]interface{})) < 2 {
					return
				}
				price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
				amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
				bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount}
			}
			sort.Sort(bidAsk.Asks)
			sort.Sort(sort.Reverse(bidAsk.Bids))
			if markets.SetBidAsk(symbol, model.Binance, &bidAsk) {
				for function, handler := range model.GetFunctions(model.Binance, findSettingSymbol) {
					if handler != nil {
						settings := model.GetSetting(function, model.Binance, findSettingSymbol)
						for _, setting := range settings {
							go handler(setting, &bidAsk)
						}
					}
				}
			}
		}
	}
	//requestUrl := model.AppConfig.WSUrls[model.Binance]
	return WebSocketClient(model.Binance, requestUrl, model.SubscribeDepth,
		GetBinanceWSSubscribes(model.Binance, model.SubscribeDepth, requestUrl), subscribeHandlerBinance, wsHandler, orderHandler)
}

func GetBinanceWSSubscribes(market, subType, requestUrl string) []interface{} {
	symbols := model.GetMarketSymbols(market)
	subscribes := make([]interface{}, 0)
	for symbol := range symbols {
		if strings.Contains(symbol, "-PERP") {
			continue
		}

		subTypes := strings.Split(subType, `,`)
		for _, value := range subTypes {
			subscribe := GetWSSubscribe(market, symbol, value)
			if subscribe != `` {
				subscribes = append(subscribes, subscribe)
			}
		}
	}
	return subscribes
}

func signBinance(postData *url.Values, secretKey string) {
	postData.Set("recvWindow", "60000")
	ts := strconv.FormatInt(util.GetNow().UnixNano(), 10)[0:13]
	postData.Set("timestamp", ts)
	hash := hmac.New(sha256.New, []byte(secretKey))
	hash.Write([]byte(postData.Encode()))
	postData.Set("signature", hex.EncodeToString(hash.Sum(nil)))
}

// orderType: BUY SELL
// 注意，binance中amount无论是市价还是限价，都指的是要买入或者卖出的左侧币种，而非右侧的钱,所以在市价买入的时候
// 要把参数从左侧的币换成右测的钱
func placeOrderBinance(key, secret string, order *model.Order, orderSide, orderType, symbol, price, amount string) {
	postData := url.Values{}

	urls := strings.Split(model.AppConfig.RestUrls[model.Binance], ",")
	var urlMapping string
	if strings.Contains(symbol, "-PERP") {
		urlMapping = urls[1] + "/fapi/v1/order"

		symbol = strings.ReplaceAll(symbol, "-PERP", "USDT")
	} else {
		urlMapping = urls[0] + "/sapi/v1/margin/order"

		if orderSide == model.OrderSideSell {
			postData.Set("sideEffectType", "MARGIN_BUY")
		} else if orderSide == model.OrderSideBuy {
			postData.Set("sideEffectType", "AUTO_REPAY")
		}
	}

	if orderSide == model.OrderSideBuy {
		orderSide = `BUY`
	} else if orderSide == model.OrderSideSell {
		orderSide = `SELL`
	} else {
		util.Notice(fmt.Sprintf(`[parameter error] order side: %s`, orderSide))
	}
	if orderType == model.OrderTypeMarket {
		orderType = `MARKET`
		if orderSide == model.OrderSideBuy {
			amountFloat, _ := strconv.ParseFloat(amount, 64)
			priceFloat, _ := strconv.ParseFloat(price, 64)
			amountFloat = amountFloat / priceFloat
			amount = strconv.FormatFloat(math.Floor(amountFloat*100)/100, 'f', 2, 64)
		}
	} else if orderType == model.OrderTypeLimit {
		orderType = `LIMIT`
		postData.Set("price", price)
		postData.Set("timeInForce", "GTC")
	} else {
		util.Notice(fmt.Sprintf(`[parameter error] order type: %s`, orderType))
	}
	postData.Set("symbol", symbol)
	postData.Set("type", orderType)
	postData.Set("side", orderSide)
	postData.Set("quantity", amount)
	signBinance(&postData, secret)
	headers := map[string]string{"X-MBX-APIKEY": key}
	responseBody, _ := util.HttpRequest("POST", urlMapping+"?", postData.Encode(), headers, 60)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		orderIdInt, _ := orderJson.Get("orderId").Int()
		if orderIdInt != 0 {
			order.OrderId = strconv.Itoa(orderIdInt)
		}
		errCodeInt, _ := orderJson.Get("code").Int()
		if errCodeInt != 0 {
			order.OrderId = strconv.Itoa(errCodeInt)
		}
	}
	util.Notice(fmt.Sprintf(`[挂单binance] %s side: %s type: %s price: %s amount: %s order id %s 返回%s`,
		symbol, orderSide, orderType, price, amount, order.OrderId, string(responseBody)))
}

func cancelOrdersBinance(key string, secret string, symbol string) bool {
	postData := url.Values{}

	urls := strings.Split(model.AppConfig.RestUrls[model.Binance], ",")
	var urlMapping string
	if strings.Contains(symbol, "-PERP") {
		symbol = strings.ReplaceAll(symbol, "-PERP", "USDT")
		urlMapping = urls[1] + "/fapi/v1/allOpenOrders"
	} else {
		urlMapping = urls[0] + "/sapi/v1/margin/openOrders"
	}
	postData.Set("symbol", symbol)
	signBinance(&postData, secret)
	headers := map[string]string{"X-MBX-APIKEY": key}
	requestUrl := urlMapping + "?" + postData.Encode()
	responseBody, _ := util.HttpRequest("DELETE", requestUrl, "", headers, 60)
	util.Notice("binance cancel order" + string(responseBody))

	return true
}

func cancelOrderBinance(key, secret, symbol string, orderId string) (result bool, errCode, msg string) {
	postData := url.Values{}
	postData.Set("symbol", strings.ToUpper(strings.Replace(symbol, "_", "", 1)))
	postData.Set("orderId", orderId)
	signBinance(&postData, secret)
	headers := map[string]string{"X-MBX-APIKEY": key}
	requestUrl := model.AppConfig.RestUrls[model.Binance] + "/api/v3/order?" + postData.Encode()
	responseBody, _ := util.HttpRequest("DELETE", requestUrl, "", headers, 60)
	util.Notice("binance cancel order" + string(responseBody))

	return true, ``, ``
}

func queryOrderBinance(key, secret, symbol string, orderId string) (dealAmount, dealPrice float64, status string) {
	postData := url.Values{}
	postData.Set("symbol", strings.ReplaceAll(symbol, "-PERP", "USDT"))
	postData.Set("orderId", orderId)
	signBinance(&postData, secret)
	headers := map[string]string{"X-MBX-APIKEY": key}
	requestUrl := model.AppConfig.RestUrls[model.Binance] + "/api/v3/order?" + postData.Encode()
	responseBody, _ := util.HttpRequest("GET", requestUrl, "", headers, 60)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		str, _ := orderJson.Get("executedQty").String()
		if str != "" {
			dealAmount, _ = strconv.ParseFloat(str, 64)
		}
		strDealPrice, _ := orderJson.Get(`price`).String()
		if strDealPrice != `` {
			dealPrice, _ = strconv.ParseFloat(strDealPrice, 64)
		}
		status, _ = orderJson.Get("status").String()
		status = model.GetOrderStatus(model.Binance, status)
	}
	util.Notice(fmt.Sprintf("%s binance query order %f %s", status, dealAmount, responseBody))
	return dealAmount, dealPrice, status
}

func getBalanceBinance(key string, secret string) (success bool, balances []*model.Balance) {
	postData := url.Values{}
	signBinance(&postData, secret)
	urls := strings.Split(model.AppConfig.RestUrls[model.Binance], ",")
	requestUrl := urls[0] + "/sapi/v1/margin/account?" + postData.Encode()
	headers := map[string]string{"X-MBX-APIKEY": key}
	responseBody, _ := util.HttpRequest("GET", requestUrl, "", headers, 60)
	balanceJson, _ := util.NewJSON(responseBody)
	if balanceJson.Get("tradeEnabled").MustBool() {
		balances = make([]*model.Balance, 0)
		currencies, _ := balanceJson.Get("userAssets").Array()
		for _, value := range currencies {
			asset := value.(map[string]interface{})

			if asset[`asset`] == nil {
				continue
			}
			coin := asset["asset"].(string)
			balance := &model.Balance{
				Market:      model.Binance,
				Coin:        coin,
				ID:          model.Binance + `_` + coin + `_` + util.GetNow().Format(time.RFC3339)[0:10],
				BalanceTime: util.GetNow(),
				AccountId:   key}
			if asset[`netAsset`] != nil {
				balance.Available, _ = strconv.ParseFloat(asset[`netAsset`].(string), 64)
			}
			if asset[`free`] != nil { // 持仓
				balance.Amount, _ = strconv.ParseFloat(asset[`free`].(string), 64)
			}
			if asset[`borrowed`] != nil { //已借数量
				balance.Borrow, _ = strconv.ParseFloat(asset[`borrowed`].(string), 64)
			}

			balances = append(balances, balance)
			//totalInUsd += balance.UsdValue
		}

		return true, balances
	} else {
		time.Sleep(time.Second * 2)
		util.SocketInfo(`fail to refresh balance binance`)
		return getBalanceBinance(key, secret)
	}
}

func getPositionsBinance(key, secret string) (success bool, positions []*model.Position) {
	postData := url.Values{}
	signBinance(&postData, secret)
	urls := strings.Split(model.AppConfig.RestUrls[model.Binance], ",")
	requestUrl := urls[1] + "/fapi/v2/account?" + postData.Encode()
	headers := map[string]string{"X-MBX-APIKEY": key}
	responseBody, _ := util.HttpRequest("GET", requestUrl, "", headers, 60)
	positionJson, _ := util.NewJSON(responseBody)

	success = positionJson.Get("canTrade").MustBool()
	if success {
		positions = make([]*model.Position, 0)
		if positionJson != nil {
			data := positionJson.Get("positions").MustArray()
			for _, item := range data {
				position := &model.Position{Market: model.Binance, Ts: util.GetNowUnixMillion()}
				asset := item.(map[string]interface{})
				if asset[`symbol`] != nil {
					position.Currency = asset[`symbol`].(string)
				}
				if asset[`positionAmt`] != nil {
					position.Free, _ = strconv.ParseFloat(asset[`positionAmt`].(string), 64)
				}
				if asset[`entryPrice`] != nil {
					position.EntryPrice, _ = strconv.ParseFloat(asset[`entryPrice`].(string), 64)
				}

				positions = append(positions, position)
			}
		}
	}
	return success, positions
}

//func getAccountBinance(key, secret string, accounts *model.Accounts) (success bool) {
//	postData := url.Values{}
//	signBinance(&postData, secret)
//	headers := map[string]string{"X-MBX-APIKEY": key}
//	requestUrl := model.AppConfig.RestUrls[model.Binance] + "/api/v3/account?" + postData.Encode()
//	responseBody, _ := util.HttpRequest("GET", requestUrl, "", headers, 60)
//	balanceJson, _ := util.NewJSON(responseBody)
//	if balanceJson.Get("canTrade").MustBool() {
//		currencies, _ := balanceJson.Get("balances").Array()
//		for _, value := range currencies {
//			asset := value.(map[string]interface{})
//			free, _ := strconv.ParseFloat(asset["free"].(string), 64)
//			frozen, _ := strconv.ParseFloat(asset["locked"].(string), 64)
//			if free == 0 && frozen == 0 {
//				continue
//			}
//			currency := strings.ToLower(asset["asset"].(string))
//			account := &model.Account{Market: model.Binance, Currency: currency, Free: free, Frozen: frozen}
//			accounts.SetAccount(model.Binance, currency, account)
//		}
//		return true
//	} else {
//		time.Sleep(time.Second * 2)
//		util.SocketInfo(`fail to refresh accounts binance`)
//		return getAccountBinance(key, secret, accounts)
//	}
//}

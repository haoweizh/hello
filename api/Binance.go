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
)

var lastTickId = make(map[string]int64) // symbol - int64

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

func WsDepthServeBinance(markets *model.Markets, orderHandler OrderHandler) (chan struct{}, error) {
	wsHandler := func(channelKey string, event []byte, orderHandler OrderHandler) {
		json, err := util.NewJSON(event)
		if err != nil {
			util.SocketInfo(`binance fail to unmarshal json ` + err.Error())
			return
		}
		subscribe, _ := json.Get("stream").String()
		symbol := model.GetSymbol(model.Binance, subscribe)
		if symbol != "" {
			json = json.Get("data")
			if json == nil {
				return
			}
			bidAsk := model.BidAsk{}
			tickId, _ := json.Get(`lastUpdateId`).Int64()
			if tickId > lastTickId[symbol] {
				lastTickId[symbol] = tickId
				bidAsk.Ts = int(util.GetNowUnixMillion())
				bidAsk.TsReceived = int(util.GetNowUnixMillion())
			} else {
				return
			}
			bids, _ := json.Get(`bids`).Array()
			asks, _ := json.Get(`asks`).Array()
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
				for function, handler := range model.GetFunctions(model.Binance, symbol) {
					if handler != nil {
						settings := model.GetSetting(function, model.Binance, symbol)
						for _, setting := range settings {
							go handler(setting, &bidAsk)
						}
					}
				}
			}
		}
	}
	requestUrl := model.AppConfig.WSUrls[model.Binance]
	return WebSocketClient(model.Binance, requestUrl, model.SubscribeDepth,
		GetWSSubscribes(model.Binance, model.SubscribeDepth), subscribeHandlerBinance, wsHandler, orderHandler)
}
func signBinance(postData *url.Values, secretKey string) {
	postData.Set("recvWindow", "6000000")
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
	postData.Set("symbol", strings.ToUpper(strings.Replace(symbol, "_", "", 1)))
	postData.Set("type", orderType)
	postData.Set("side", orderSide)
	postData.Set("quantity", amount)
	signBinance(&postData, secret)
	headers := map[string]string{"X-MBX-APIKEY": key}
	responseBody, _ := util.HttpRequest("POST",
		model.AppConfig.RestUrls[model.Binance]+"/api/v3/order?", postData.Encode(), headers, 60)
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
	postData.Set("symbol", strings.ToUpper(strings.Replace(symbol, "_", "", 1)))
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

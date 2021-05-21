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
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const restBinance = "https://api.binance.com"
const restBinanceFuture = `https://fapi.binance.com`
const wsBinance = "wss://stream.binance.com:9443/stream"
const wsBinanceFuture = `wss://fstream.binance.com/stream`
const binanceMargin = `binanceMargin`
const binancePerp = `binancePerp`

var lastTickIdBinance = make(map[string]int64) // symbol - int64
var lastTradeTimeBinance = make(map[string]int64)
var channelMaintainingBinance = false

func maintainChannelBinance() {
	if !channelMaintainingBinance {
		channelMaintainingBinance = true
		for true {
			time.Sleep(time.Minute)
			err := sendToWs(binanceMargin, []byte(`{pong}`))
			if err != nil {
				util.SocketInfo("pong binance ws client error " + err.Error())
			}
			err = sendToWs(binancePerp, []byte(`{pong}`))
			if err != nil {
				util.SocketInfo("pong binance future ws client error " + err.Error())
			}
		}
	}
}

var subscribeHandlerBinance = func(subscribes []interface{}, keyChannel string) error {
	var err error = nil
	for _, subscribe := range subscribes {
		subMsg := fmt.Sprintf(`{"method": "SUBSCRIBE","params":["%s"],"id": %d}`, subscribe, int(rand.Float64()*10000))
		if err = sendToWs(keyChannel, []byte(subMsg)); err != nil {
			util.SocketInfo("binance can not subscribe " + err.Error())
		}
		time.Sleep(time.Millisecond * 150)
	}
	return err
}

func WsDepthServeBinance(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	wsHandler := func(channelKey string, event []byte, orderHandler OrderHandler) {
		json, err := util.NewJSON(event)
		if err != nil {
			util.SocketInfo(`binance fail to unmarshal json ` + err.Error())
			return
		}
		subscribe, _ := json.Get("stream").String()
		symbol := model.GetSymbol(model.Binance, subscribe) //当前获取到的币种的推送
		var findSettingSymbol string
		if symbol != "" {
			json = json.Get("data")
			if json == nil {
				return
			}
			bidAsk := model.BidAsk{}
			var bids, asks []interface{}
			tickId, _ := json.Get(`lastUpdateId`).Int64()
			depthUpdate, _ := json.Get(`e`).String()
			if tickId > 0 { //存在lastUpdateId字段 表示是现货深度推送，此区分方式不稳定，暂用
				if tickId > lastTickIdBinance[symbol] {
					lastTickIdBinance[symbol] = tickId
					bidAsk.Ts = int(util.GetNowUnixMillion())
					bidAsk.TsReceived = int(util.GetNowUnixMillion())
				} else {
					return
				}
				bidArray, _ := json.Get(`bids`).Array()
				bids = bidArray
				askArray, _ := json.Get(`asks`).Array()
				asks = askArray
				if symbol[len(symbol)-4:] == `USDT` {
					findSettingSymbol = symbol[0:len(symbol)-4] + "-PERP"
				}
			} else if depthUpdate == "depthUpdate" { //存在depthUpdate字段 表示是合约深度推送，此区分方式不稳定，暂用
				nowTradeTime, _ := json.Get(`T`).Int64()
				if nowTradeTime <= 0 || nowTradeTime < lastTradeTimeBinance[symbol] {
					return
				}
				lastTradeTimeBinance[symbol] = nowTradeTime
				bidAsk.Ts = json.Get(`E`).MustInt()
				bidAsk.TsReceived = int(util.GetNowUnixMillion())
				bidArray, _ := json.Get(`b`).Array()
				bids = bidArray
				askArray, _ := json.Get(`a`).Array()
				asks = askArray
				symbol = json.Get(`s`).MustString()
				if symbol[len(symbol)-4:] == `USDT` {
					symbol = symbol[0:len(symbol)-4] + "-PERP"
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
	channels = make([]chan struct{}, 2)
	//requestUrl := model.AppConfig.WSUrls[model.Binance]
	channels[0], err = WebSocketClient(binanceMargin, wsBinance, binanceMargin,
		GetWSSubscribes(model.Binance, model.SubscribeDepth), subscribeHandlerBinance, wsHandler, orderHandler)
	channels[1], err = WebSocketClient(binancePerp, wsBinanceFuture, binancePerp,
		GetWSSubscribes(model.Binance, model.SubscribeDepth), subscribeHandlerBinance, wsHandler, orderHandler)
	return channels, err
}

func signedRequestBinance(key, secret, method, requestUrl string, withApiKey bool, param *url.Values) []byte {
	if key == `` || secret == `` {
		keys, secrets := model.AppConfig.GetKeys(model.Binance)
		key = keys[0]
		secret = secrets[0]
	}
	if param == nil {
		param = &url.Values{}
	}
	if withApiKey {
		param.Set("recvWindow", "60000")
		ts := strconv.FormatInt(util.GetNow().UnixNano(), 10)[0:13]
		param.Set("timestamp", ts)
		hash := hmac.New(sha256.New, []byte(secret))
		hash.Write([]byte(param.Encode()))
		param.Set("signature", hex.EncodeToString(hash.Sum(nil)))
	}
	headers := map[string]string{"X-MBX-APIKEY": key}
	requestUrl = requestUrl + "?" + param.Encode()
	responseBody, _ := util.HttpRequest(method, requestUrl, "", headers, 60)
	logMsg := fmt.Sprintf(`binance key %s request %s body %v return %s`,
		key, requestUrl, param, string(responseBody))
	if strings.Contains(requestUrl, `/order`) {
		util.Notice(logMsg)
	} else if !strings.Contains(requestUrl, `exchangeInfo`) {
		util.SocketInfo(logMsg)
	}
	return responseBody
}

func setMarketInfoFilters(marketInfo *model.MarketInfo, filters []interface{}) {
	for _, filter := range filters {
		data := filter.(map[string]interface{})
		filterType := data[`filterType`].(string)
		if filterType == `PRICE_FILTER` {
			if data[`tickSize`] != nil {
				marketInfo.PriceIncrement, _ = strconv.ParseFloat(data[`tickSize`].(string), 64)
			}
			marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
		} else if filterType == `LOT_SIZE` {
			if data[`minQty`] != nil {
				marketInfo.SizeMin, _ = strconv.ParseFloat(data[`minQty`].(string), 64)
			}
			if data[`stepSize`] != nil {
				marketInfo.SizeIncrement, _ = strconv.ParseFloat(data[`stepSize`].(string), 64)
			}
		}
	}
}

// 查询MARGIN和永续合约信息，不包含其他市场
func getMarketsBinance() (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	requestUrls := []string{restBinance + `/api/v3/exchangeInfo`, restBinanceFuture + `/fapi/v1/exchangeInfo`}
	for _, requestUrl := range requestUrls {
		responseBody := signedRequestBinance(``, ``, http.MethodGet, requestUrl, false, nil)
		resultJson, err := util.NewJSON(responseBody)
		if err == nil && resultJson.Get(`symbols`) != nil {
			data := resultJson.Get(`symbols`).MustArray()
			for _, item := range data {
				value := item.(map[string]interface{})
				if value[`quoteAsset`] == nil || value[`baseAsset`] == nil {
					continue
				}
				var symbol string
				if value[`contractType`] == nil {
					haveMargin := false
					if value[`permissions`] != nil {
						permissions := value[`permissions`].([]interface{})
						for _, permission := range permissions {
							if permission.(string) == `MARGIN` {
								haveMargin = true
							}
						}
					}
					if !haveMargin {
						continue
					}
					symbol = value[`baseAsset`].(string) + value[`quoteAsset`].(string)
				} else if value[`contractType`] != nil && value[`contractType`].(string) == `PERPETUAL` {
					symbol = value[`baseAsset`].(string) + `-PERP`
				} else {
					continue
				}
				marketInfo := &model.MarketInfo{Name: symbol, CTCurrency: value[`baseAsset`].(string)}
				setMarketInfoFilters(marketInfo, value[`filters`].([]interface{}))
				marketInfos[marketInfo.Name] = marketInfo
			}
		}
	}
	return marketInfos
}

// orderType: BUY SELL
// 注意，binance中amount无论是市价还是限价，都指的是要买入或者卖出的左侧币种，而非右侧的钱,所以在市价买入的时候
// 要把参数从左侧的币换成右测的钱
func placeOrderBinance(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	postData := &url.Values{}
	price, decimal := FormatPrice(model.Binance, symbol, orderSide, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	formattedAmount := GetAmountInMarket(model.Binance, symbol, amount)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	postData.Set("quantity", amountStr)
	var requestUrl string
	if symbol[len(symbol)-5:] == `-PERP` {
		requestUrl = restBinanceFuture + "/fapi/v1/order"
		symbol = strings.ReplaceAll(symbol, "-PERP", "USDT")
	} else {
		requestUrl = restBinance + "/sapi/v1/margin/order"
		if orderSide == model.OrderSideSell {
			postData.Set("sideEffectType", "MARGIN_BUY")
		} else if orderSide == model.OrderSideBuy {
			postData.Set("sideEffectType", "AUTO_REPAY")
		}
	}
	if orderSide == model.OrderSideBuy {
		postData.Set("side", `BUY`)
	} else if orderSide == model.OrderSideSell {
		postData.Set("side", `SELL`)
	}
	if orderType == model.OrderTypeMarket {
		postData.Set("type", `MARKET`)
	} else if orderType == model.OrderTypeLimit {
		postData.Set("type", `LIMIT`)
		postData.Set("price", priceStr)
		postData.Set("timeInForce", "GTC")
	}
	postData.Set("symbol", symbol)
	responseBody := signedRequestBinance(key, secret, http.MethodPost, requestUrl, true, postData)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		orderIdInt, _ := orderJson.Get("orderId").Int()
		if orderIdInt != 0 {
			order.OrderId = strconv.Itoa(orderIdInt)
		}
		errCodeInt, _ := orderJson.Get("code").Int()
		if errCodeInt != 0 {
			order.OrderId = ``
		}
	}
}

func cancelOrdersBinance(key string, secret string, symbol string) bool {
	postData := &url.Values{}
	var requestUrl string
	if symbol[len(symbol)-5:] == `-PERP` {
		symbol = symbol[len(symbol)-5:] + "USDT"
		requestUrl = restBinanceFuture + "/fapi/v1/allOpenOrders"
	} else {
		requestUrl = restBinance + "/sapi/v1/margin/openOrders"
	}
	postData.Set("symbol", symbol)
	signedRequestBinance(key, secret, http.MethodDelete, requestUrl, true, postData)
	return true
}

//func cancelOrderBinance(key, secret, symbol string, orderId string) (result bool, errCode, msg string) {
//	postData := url.Values{}
//	postData.Set("symbol", strings.ToUpper(strings.Replace(symbol, "_", "", 1)))
//	postData.Set("orderId", orderId)
//	signBinance(&postData, secret)
//	headers := map[string]string{"X-MBX-APIKEY": key}
//	requestUrl := model.AppConfig.RestUrls[model.Binance] + "/api/v3/order?" + postData.Encode()
//	responseBody, _ := util.HttpRequest("DELETE", requestUrl, "", headers, 60)
//	util.Notice("binance cancel order" + string(responseBody))
//	return true, ``, ``
//}
//
//func queryOrderBinance(key, secret, symbol string, orderId string) (dealAmount, dealPrice float64, status string) {
//	postData := url.Values{}
//	postData.Set("symbol", strings.ReplaceAll(symbol, "-PERP", "USDT"))
//	postData.Set("orderId", orderId)
//	signBinance(&postData, secret)
//	headers := map[string]string{"X-MBX-APIKEY": key}
//	requestUrl := model.AppConfig.RestUrls[model.Binance] + "/api/v3/order?" + postData.Encode()
//	responseBody, _ := util.HttpRequest("GET", requestUrl, "", headers, 60)
//	orderJson, err := util.NewJSON(responseBody)
//	if err == nil {
//		str, _ := orderJson.Get("executedQty").String()
//		if str != "" {
//			dealAmount, _ = strconv.ParseFloat(str, 64)
//		}
//		strDealPrice, _ := orderJson.Get(`price`).String()
//		if strDealPrice != `` {
//			dealPrice, _ = strconv.ParseFloat(strDealPrice, 64)
//		}
//		status, _ = orderJson.Get("status").String()
//		status = model.GetOrderStatus(model.Binance, status)
//	}
//	util.Notice(fmt.Sprintf("%s binance query order %f %s", status, dealAmount, responseBody))
//	return dealAmount, dealPrice, status
//}

func getBalanceBinance(key string, secret string) (success bool, balances []*model.Balance) {
	requestUrl := restBinance + "/sapi/v1/margin/account"
	responseBody := signedRequestBinance(key, secret, http.MethodGet, requestUrl, true, nil)
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
			if asset[`free`] != nil { // 持仓
				balance.Available, _ = strconv.ParseFloat(asset[`free`].(string), 64)
			}
			if asset[`netAsset`] != nil {
				balance.Amount, _ = strconv.ParseFloat(asset[`netAsset`].(string), 64)
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

func getPosBalBinance(key, secret string) (value float64) {
	responseBody := signedRequestBinance(key, secret, http.MethodGet, restBinanceFuture+`/fapi/v2/balance`, true, nil)
	positionJson, err := util.NewJSON(responseBody)
	if err != nil {
		return 0
	}
	assets := positionJson.MustArray()
	for _, asset := range assets {
		item := asset.(map[string]interface{})
		if item[`asset`].(string) == `USDT` {
			if item[`balance`] != nil {
				value, _ = strconv.ParseFloat(item[`balance`].(string), 64)
			}
		}
	}
	return value
}

func getPositionsBinance(key, secret string) (success bool, positions []*model.Position) {
	responseBody := signedRequestBinance(key, secret, http.MethodGet, restBinanceFuture+"/fapi/v2/account", true, nil)
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

func getFundingRateBinance(key, secret, symbol string) (fundingRate *model.FundingRate) {
	postData := &url.Values{}
	if symbol[len(symbol)-5:] != `-PERP` {
		return nil
	}
	symbol = symbol[0:len(symbol)-5] + `USDT`
	postData.Set(`symbol`, symbol)
	response := signedRequestBinance(key, secret, http.MethodGet, restBinanceFuture+`/fapi/v1/premiumIndex`, false, postData)
	fundingJson, err := util.NewJSON(response)
	if err != nil {
		return nil
	}
	rateStr := fundingJson.Get(`lastFundingRate`).MustString()
	rate, _ := strconv.ParseFloat(rateStr, 64)
	nextFundingTime := fundingJson.Get(`nextFundingTime`).MustInt64()
	fundingRate = &model.FundingRate{
		FundingTime: time.Time{},
		Rate:        rate,
		UpdateTime:  util.GetNow().Unix(),
		ExpireTime:  nextFundingTime / 1000,
		Symbol:      symbol,
	}
	return
}

func getMaxLoanBinance(key, secret, coin string) (success bool, maxBorrow float64) {
	postData := &url.Values{}
	postData.Set(`asset`, coin)
	response := signedRequestBinance(key, secret, http.MethodGet, restBinance+`/sapi/v1/margin/maxBorrowable`, true, postData)
	borrowJson, err := util.NewJSON(response)
	if err == nil {
		amount := borrowJson.Get(`amount`).MustFloat64()
		borrowLimit := borrowJson.GetPath(`borrowLimit`).MustFloat64()
		return true, math.Min(amount, borrowLimit)
	}
	return false, 0
}

// MARGIN_UMFUTURE 杠杆全仓钱包转向U本位合约钱包
// UMFUTURE_MARGIN U本位合约钱包转向杠杆全仓钱包
func transferBinance(key, secret, transferType string, amount float64) {
	postData := &url.Values{}
	postData.Set(`type`, transferType)
	postData.Set(`asset`, `USDT`)
	postData.Set(`amount`, strconv.FormatFloat(amount, 'f', 0, 64))
	signedRequestBinance(key, secret, http.MethodPost, restBinance+`/sapi/v1/asset/transfer`, true, postData)
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

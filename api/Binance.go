package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const restBinance = "https://api.binance.com"
const restBinanceFuture = `https://fapi.binance.com`
const wsBinance = "wss://stream.binance.com:9443/stream"
const wsBinanceFuture = `wss://fstream.binance.com/stream`

//const binanceMargin = `binanceMargin`
//const binancePerp = `binancePerp`
const wsStepBinance = 20

var lockWSBinance sync.Mutex
var lastTickIdBinance = make(map[string]int64) // symbol - int64
var lastTradeTimeBinance = make(map[string]int64)

func getLastTickIdBinance(symbol string) int64 {
	defer lockWSBinance.Unlock()
	lockWSBinance.Lock()
	return lastTickIdBinance[symbol]
}

func setLastTickIdBinance(symbol string, tickId int64) {
	defer lockWSBinance.Unlock()
	lockWSBinance.Lock()
	lastTickIdBinance[symbol] = tickId
}

func getLastTradeTimeBinance(symbol string) int64 {
	defer lockWSBinance.Unlock()
	lockWSBinance.Lock()
	return lastTradeTimeBinance[symbol]
}

func setLastTradeTimeBinance(symbol string, tradeTime int64) {
	defer lockWSBinance.Unlock()
	lockWSBinance.Lock()
	lastTradeTimeBinance[symbol] = tradeTime
}

//var channelMaintainingBinance = false

func maintainChannelBinance() {
	//if !channelMaintainingBinance {
	//	channelMaintainingBinance = true
	//	for true {
	//		time.Sleep(time.Minute * 5)
	//		ts := time.Now().UnixNano() / int64(time.Millisecond)
	//		pong := []byte(fmt.Sprintf(`{"method":"PONG","E":%d}`, ts))
	//		err := sendToWs(binanceMargin, pong)
	//		if err != nil {
	//			util.SocketInfo("pong binance ws client error " + err.Error())
	//		}
	//		err = sendToWs(binancePerp, pong)
	//		if err != nil {
	//			util.SocketInfo("pong binance future ws client error " + err.Error())
	//		}
	//	}
	//}
}

var subscribeHandlerBinance = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	for _, subscribe := range subscribes {
		subMsg := fmt.Sprintf(`{"method": "SUBSCRIBE","params":["%s"],"id": %d}`, subscribe, int(rand.Float64()*10000))
		if err = SendToConnection(model.Binance, connection, []byte(subMsg)); err != nil {
			util.SocketInfo(" binance can not subscribe %s %s", subscribe, err.Error())
		}
		time.Sleep(time.Millisecond * 300)
	}
	return err
}

func handleTickerBinance(markets *model.Markets, json *simplejson.Json, dialectSymbol string, updateId int64) {
	bidPrice, _ := strconv.ParseFloat(json.Get(`b`).MustString(), 64)
	bidAmount, _ := strconv.ParseFloat(json.Get(`B`).MustString(), 64)
	askPrice, _ := strconv.ParseFloat(json.Get(`a`).MustString(), 64)
	askAmount, _ := strconv.ParseFloat(json.Get(`A`).MustString(), 64)
	ts := json.Get(`E`).MustInt()
	now := int(time.Now().UnixNano() / int64(time.Millisecond))
	if ts == 0 {
		ts = now
	}
	bookTicker := json.Get(`e`).MustString()
	if dialectSymbol != `` && bidPrice > 0 && bidAmount > 0 && askPrice > 0 && askAmount > 0 {
		marketType := model.MarketTypeSpot
		if bookTicker == `bookTicker` { //有e字段 表示是U合约推送，否则现货，此区分方式不稳定，暂用
			marketType = model.MarketTypePerp
		}
		_, _, coin := model.GetCoinFromDialect(model.Binance, dialectSymbol)
		standardSymbol := coin + model.UniStandardTail[marketType]
		bidAsk := model.BidAsk{Ts: ts, TsReceived: now, UpdateId: updateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.Binance, Symbol: standardSymbol, Side: model.OrderSideBuy}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.Binance, Symbol: standardSymbol, Side: model.OrderSideSell}}}
		haveOld, old := markets.GetBidAsk(standardSymbol, model.Binance)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
			return
		}
		if markets.SetBidAsk(standardSymbol, model.Binance, &bidAsk) {
			for function, handler := range model.GetFunctions(model.Binance, standardSymbol) {
				if handler != nil {
					setting := model.GetSetting(function, model.Binance, standardSymbol)
					if setting != nil {
						go handler(setting, &bidAsk)
					}
				}
			}
		}
	}
}

func handleDepthBinance(markets *model.Markets, json *simplejson.Json, dialectSymbol string, updateId int64) {
	var standardSymbol string
	bidAsk := model.BidAsk{UpdateId: updateId}
	var bids, asks []interface{}
	tickId, _ := json.Get(`lastUpdateId`).Int64()
	depthUpdate, _ := json.Get(`e`).String()
	_, _, coin := model.GetCoinFromDialect(model.Binance, dialectSymbol)
	if tickId > 0 { //存在lastUpdateId字段 表示是现货深度推送，此区分方式不稳定，暂用
		standardSymbol = coin + model.UniStandardTail[model.MarketTypeSpot]
		if tickId > getLastTickIdBinance(standardSymbol) {
			setLastTickIdBinance(standardSymbol, tickId)
			bidAsk.Ts = int(util.GetNowUnixMillion())
			bidAsk.TsReceived = int(util.GetNowUnixMillion())
		} else {
			return
		}
		bidArray, _ := json.Get(`bids`).Array()
		bids = bidArray
		askArray, _ := json.Get(`asks`).Array()
		asks = askArray
	} else if depthUpdate == "depthUpdate" { //存在depthUpdate字段 表示是合约深度推送，此区分方式不稳定，暂用
		standardSymbol = coin + model.UniStandardTail[model.MarketTypePerp]
		nowTradeTime, _ := json.Get(`T`).Int64()
		if nowTradeTime <= 0 || nowTradeTime < getLastTradeTimeBinance(standardSymbol) {
			return
		}
		setLastTradeTimeBinance(standardSymbol, nowTradeTime)
		bidAsk.Ts = json.Get(`E`).MustInt()
		bidAsk.TsReceived = int(util.GetNowUnixMillion())
		bidArray, _ := json.Get(`b`).Array()
		bids = bidArray
		askArray, _ := json.Get(`a`).Array()
		asks = askArray
		dialectSymbol = json.Get(`s`).MustString()
	}
	bidAsk.Bids = make([]model.Tick, len(bids))
	for i, value := range bids {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: model.Binance, Symbol: standardSymbol, Side: model.OrderSideBuy}
	}
	bidAsk.Asks = make([]model.Tick, len(asks))
	for i, value := range asks {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: model.Binance, Symbol: standardSymbol, Side: model.OrderSideSell}
	}
	sort.Sort(bidAsk.Asks)
	sort.Sort(sort.Reverse(bidAsk.Bids))
	haveOld, old := markets.GetBidAsk(standardSymbol, model.Binance)
	if haveOld && old.UpdateId > bidAsk.UpdateId {
		return
	}
	if markets.SetBidAsk(standardSymbol, model.Binance, &bidAsk) {
		for function, handler := range model.GetFunctions(model.Binance, standardSymbol) {
			if handler != nil {
				setting := model.GetSetting(function, model.Binance, standardSymbol)
				if setting != nil {
					go handler(setting, &bidAsk)
				}
			}
		}
	}
}

func WsDepthServeBinance(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	subType := model.SubscribeTicker
	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		json, wsErr := util.NewJSON(event)
		if wsErr != nil {
			util.SocketInfo(`binance fail to unmarshal json ` + err.Error())
			return
		}
		subscribe, _ := json.Get("stream").String()
		json = json.Get(`data`)
		if json == nil {
			return
		}
		dialectSymbol := json.Get(`s`).MustString()
		updateId := json.Get(`u`).MustInt64()
		if dialectSymbol == `` {
			return
		}
		if strings.Contains(subscribe, `@depth`) {
			handleDepthBinance(markets, json, dialectSymbol, updateId)
		} else if strings.Contains(subscribe, `@bookTicker`) {
			handleTickerBinance(markets, json, dialectSymbol, updateId)
		}
	}
	channels = make([]chan struct{}, 0)
	//requestUrl := model.AppConfig.WSUrls[model.Binance]
	marginSubs := GetWSSubscribes(model.Binance, subType)
	perpSubs := GetWSSubscribes(model.Binance, subType)
	marginChans, marginErr := WebSocketClient(model.Binance, wsBinance, marginSubs,
		subscribeHandlerBinance, wsHandler, orderHandler, wsStepBinance)
	if marginErr != nil {
		util.SocketInfo(`fail to create binance margin conn %s`, marginErr.Error())
	}
	perpChans, perpErr := WebSocketClient(model.Binance, wsBinanceFuture, perpSubs,
		subscribeHandlerBinance, wsHandler, orderHandler, wsStepBinance)
	if perpErr != nil {
		util.SocketInfo(`fail to create binance margin conn %s`, perpErr.Error())
	}
	for _, perpChan := range perpChans {
		marginChans = append(marginChans, perpChan)
	}
	return marginChans, err
}

func signedRequestBinance(key, secret, method, requestUrl string, withApiKey bool, param *url.Values) []byte {
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
	responseJson, err := util.NewJSON(responseBody)
	if err != nil || responseJson == nil {
		util.Notice(`fail to parse json`)
		return nil
	}
	code := responseJson.Get(`code`).MustInt()
	if code != 0 && code != -3027 && code != 200 && code != -2011 {
		util.Notice(`request err %d`, code)
	}
	return responseBody
}

func setPosSideBinance(key, secret string) {
	postData := &url.Values{}
	requestUrl := restBinanceFuture + "/fapi/v1/positionSide/dual"
	postData.Set("dualSidePosition", `false`)
	signedRequestBinance(key, secret, http.MethodPost, requestUrl, true, postData)
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

// 查询SPOT和永续合约信息，不包含其他市场
func getMarketsBinance(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	requestUrls := []string{restBinance + `/api/v3/exchangeInfo`, restBinanceFuture + `/fapi/v1/exchangeInfo`}
	for _, requestUrl := range requestUrls {
		responseBody := signedRequestBinance(key, secret, http.MethodGet, requestUrl, false, nil)
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
					haveSpot := false
					if value[`permissions`] != nil {
						permissions := value[`permissions`].([]interface{})
						for _, permission := range permissions {
							if permission.(string) == `SPOT` {
								haveSpot = true
							}
						}
					}
					if !haveSpot {
						continue
					}
					//symbol = value[`baseAsset`].(string) + value[`quoteAsset`].(string)
					symbol = value[`baseAsset`].(string) + model.UniStandardTail[model.MarketTypeSpot]
				} else if value[`contractType`] != nil && value[`contractType`].(string) == `PERPETUAL` {
					symbol = value[`baseAsset`].(string) + model.UniStandardTail[model.MarketTypePerp]
				} else {
					continue
				}
				marketInfo := &model.MarketInfo{Market: model.Binance, Name: symbol,
					CTCurrency: value[`baseAsset`].(string), MoneyMin: 10}
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
	price, decimal := model.FormatPrice(model.Binance, symbol, orderSide, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	formattedAmount := model.GetAmountInMarket(model.Binance, symbol, amount, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	postData.Set("quantity", amountStr)
	var requestUrl string
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Binance, symbol)
	if success && marketType == model.MarketTypePerp {
		requestUrl = restBinanceFuture + "/fapi/v1/order"
	} else if success && marketType == model.MarketTypeSpot {
		requestUrl = restBinance + "/api/v3/order"
		//requestUrl = restBinance + "/sapi/v1/margin/order"
		//if orderSide == model.OrderSideSell {
		//	postData.Set("sideEffectType", "MARGIN_BUY")
		//} else if orderSide == model.OrderSideBuy {
		//	postData.Set("sideEffectType", "AUTO_REPAY")
		//}
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
	postData.Set("symbol", dialectSymbol)
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
			order.ErrCode = strconv.Itoa(errCodeInt)
		}
	}
}

func cancelOrdersBinance(key string, secret string, symbol string) bool {
	postData := &url.Values{}
	var requestUrl string
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Binance, symbol)
	if success && marketType == model.MarketTypePerp {
		requestUrl = restBinanceFuture + "/fapi/v1/allOpenOrders"
	} else if success && marketType == model.MarketTypeSpot {
		requestUrl = restBinance + "/api/v3/openOrders"
	}
	postData.Set("symbol", dialectSymbol)
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
//	postData.Set("symbol", strings.ReplaceAll(symbol, "-PERP", `USDT`))
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
	//requestUrl := restBinance + "/sapi/v1/margin/account"
	requestUrl := restBinance + "/api/v3/account"
	responseBody := signedRequestBinance(key, secret, http.MethodGet, requestUrl, true, nil)
	balanceJson, _ := util.NewJSON(responseBody)
	if balanceJson != nil && balanceJson.Get("canTrade").MustBool() {
		balances = make([]*model.Balance, 0)
		currencies, _ := balanceJson.Get("balances").Array()
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
			if asset[`free`] != nil { // 持仓,此处按照不进行借币计算
				balance.AvailableWithBorrow, _ = strconv.ParseFloat(asset[`free`].(string), 64)
			}
			if asset[`locked`] != nil {
				lockAmount, _ := strconv.ParseFloat(asset[`locked`].(string), 64)
				balance.Amount = balance.AvailableWithBorrow + lockAmount
			}
			if balance.UsdValue == 0 && balance.Amount > 0 {
				getTick, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.Binance)
				if getTick {
					balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
				}
			}
			//if asset[`netAsset`] != nil {
			//	balance.Amount, _ = strconv.ParseFloat(asset[`netAsset`].(string), 64)
			//}
			//if asset[`borrowed`] != nil { //已借数量
			//	balance.Borrow, _ = strconv.ParseFloat(asset[`borrowed`].(string), 64)
			//}
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

//func getPosBalBinance(key, secret string) (value float64) {
//	responseBody := signedRequestBinance(key, secret, http.MethodGet, restBinanceFuture+`/fapi/v2/balance`, true, nil)
//	positionJson, err := util.NewJSON(responseBody)
//	if err != nil {
//		return 0
//	}
//	assets := positionJson.MustArray()
//	for _, asset := range assets {
//		item := asset.(map[string]interface{})
//		if item[`asset`].(string) == `USDT` {
//			if item[`balance`] != nil {
//				value, _ = strconv.ParseFloat(item[`balance`].(string), 64)
//			}
//		}
//	}
//	return value
//}

func getPositionsBinance(key, secret string) (success bool, positions []*model.Position, accountValue, availableU float64) {
	responseBody := signedRequestBinance(key, secret, http.MethodGet, restBinanceFuture+"/fapi/v2/account", true, nil)
	positionJson, err := util.NewJSON(responseBody)
	if err != nil || positionJson == nil {
		return false, nil, 0, 0
	}
	success = positionJson.Get("canTrade").MustBool()
	if success {
		positions = make([]*model.Position, 0)
		totalJson := positionJson.Get(`totalWalletBalance`).MustString()
		availableJson := positionJson.Get(`availableBalance`).MustString()
		accountValue, _ = strconv.ParseFloat(totalJson, 64)
		availableU, _ = strconv.ParseFloat(availableJson, 64)
		data := positionJson.Get("positions").MustArray()
		for _, item := range data {
			position := &model.Position{Market: model.Binance, Ts: util.GetNowUnixMillion()}
			value := item.(map[string]interface{})
			if value[`symbol`] != nil {
				_, _, coin := model.GetCoinFromDialect(model.Binance, value[`symbol`].(string))
				position.Currency = coin + model.UniStandardTail[model.MarketTypePerp]
			}
			if value[`positionAmt`] != nil {
				position.Holding, _ = strconv.ParseFloat(value[`positionAmt`].(string), 64)
			}
			if value[`entryPrice`] != nil {
				position.EntryPrice, _ = strconv.ParseFloat(value[`entryPrice`].(string), 64)
			}
			if value[`unrealizedProfit`] != nil {
				position.ProfitUnreal, _ = strconv.ParseFloat(value[`unrealizedProfit`].(string), 64)
			}
			positions = append(positions, position)
		}
	}
	return success, positions, accountValue, availableU
}

func getFundingRateBinance(key, secret, symbol string) (fundingRate *model.FundingRate) {
	postData := &url.Values{}
	_, _, _, dialectSymbol := model.GetFromStandard(model.Binance, symbol)
	postData.Set(`symbol`, dialectSymbol)
	response := signedRequestBinance(key, secret, http.MethodGet, restBinanceFuture+`/fapi/v1/premiumIndex`, false, postData)
	fundingJson, err := util.NewJSON(response)
	if err != nil {
		return nil
	}
	rateStr := fundingJson.Get(`lastFundingRate`).MustString()
	rate, _ := strconv.ParseFloat(rateStr, 64)
	nextFundingTime := fundingJson.Get(`nextFundingTime`).MustInt64()
	fundingRate = &model.FundingRate{
		Rate:       rate,
		UpdateTime: util.GetNow().Unix(),
		ExpireTime: nextFundingTime / 1000}
	return
}

// getMaxLoanBinance
func _(key, secret, coin string) (success bool, maxBorrow float64) {
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
//			account := &model.Account{Market: model.Binance, Currency: currency, Holding: free+frozen, Frozen: frozen}
//			accounts.SetAccount(model.Binance, currency, account)
//		}
//		return true
//	} else {
//		time.Sleep(time.Second * 2)
//		util.SocketInfo(`fail to refresh accounts binance`)
//		return getAccountBinance(key, secret, accounts)
//	}
//}

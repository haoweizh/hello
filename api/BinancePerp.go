package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const restBinancePerp = `https://fapi.binance.com`
const wsBinancePerp = `wss://fstream.binance.com/stream`
const wsStepBinancePerp = 20

var channelMaintainingBinancePerp = false

func getMarketsBinancePerp(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	client := futures.NewClient(key, secret)
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	stats, errTicker := client.NewListPriceChangeStatsService().Do(context.Background())
	if err != nil || errTicker != nil {
		if err != nil {
			util.Notice("getMarketsBinancePerp err: " + err.Error())
		}
		if errTicker != nil {
			util.Notice("getMarketsBinancePerp price err: " + errTicker.Error())
		}
		time.Sleep(time.Minute * 5)
		getMarketsBinancePerp(key, secret)
		return marketInfos
	}
	for _, item := range exchangeInfo.Symbols {
		if item.QuoteAsset == "" || item.BaseAsset == "" {
			continue
		}
		if item.ContractType == `PERPETUAL` && item.Status == "TRADING" && item.QuoteAsset == model.DialectTail[model.MarketTypePerp][model.BinancePerp] {
			symbol := item.BaseAsset + model.UniStandardTail[model.MarketTypePerp]
			marketInfo := &model.MarketInfo{Market: model.BinancePerp, Name: symbol, MoneyMin: 5}
			marketInfos[marketInfo.Name] = marketInfo
			for _, data := range item.Filters {
				filterType := data[`filterType`].(string)
				switch filterType {
				case `PRICE_FILTER`:
					if data[`tickSize`] != nil {
						marketInfo.PriceIncrement, _ = strconv.ParseFloat(data[`tickSize`].(string), 64)
					}
					marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
				case `LOT_SIZE`:
					if data[`minQty`] != nil {
						marketInfo.SizeMin, _ = strconv.ParseFloat(data[`minQty`].(string), 64)
					}
					if data[`maxQty`] != nil {
						marketInfo.SizeMax, _ = strconv.ParseFloat(data[`maxQty`].(string), 64)
					}
					if data[`stepSize`] != nil {
						marketInfo.SizeIncrement, _ = strconv.ParseFloat(data[`stepSize`].(string), 64)
					}
				case `MIN_NOTIONAL`:
					if data[`notional`] != nil {
						marketInfo.MoneyMin, _ = strconv.ParseFloat(data[`notional`].(string), 64)
					}
				case `PERCENT_PRICE`:
					if data[`multiplierUp`] != nil {
						rate, _ := strconv.ParseFloat(data[`multiplierUp`].(string), 64)
						marketInfo.SellLimitPriceRatio = rate - 1
					}
					if data[`multiplierDown`] != nil {
						rate, _ := strconv.ParseFloat(data[`multiplierDown`].(string), 64)
						marketInfo.BuyLimitPriceRatio = 1 - rate
					}
				}
			}
			marketInfos[marketInfo.Name] = marketInfo
		}
	}
	for _, stat := range stats {
		if stat == nil {
			continue
		}
		success, marketType, coin := model.GetCoinFromDialect(model.BinancePerp, stat.Symbol)
		if !success {
			continue
		}
		name := coin + model.UniStandardTail[marketType]
		if marketInfos[name] != nil {
			marketInfos[name].TradeAmount, _ = strconv.ParseFloat(stat.QuoteVolume, 64)
		}
	}
	return marketInfos
}

func WsDepthServeBinancePerp(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	subType := model.SubscribeTicker + `,` + model.SubscribeMarkPrice
	//subType := model.SubscribeDepth
	wsHandlerBinancePerp := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		result, wsErr := util.NewJSON(event)
		if wsErr != nil {
			util.SocketInfo(`binance fail to unmarshal json ` + err.Error())
			return
		}
		subscribe, _ := result.Get("stream").String()
		result = result.Get(`data`)
		if result == nil {
			return
		}
		dialectSymbol := result.Get(`s`).MustString()
		success, _, coin := model.GetCoinFromDialect(model.BinancePerp, dialectSymbol)
		if !success {
			return
		}
		standardSymbol := coin + model.UniStandardTail[model.MarketTypePerp]
		updateId := result.Get(`u`).MustInt64()
		haveOld, old := markets.GetBidAsk(standardSymbol, model.BinancePerp)
		if haveOld && old.UpdateId > updateId {
			return
		}
		if strings.Contains(subscribe, `@depth`) {
			handleDepthBinancePerp(markets, result, standardSymbol, updateId)
		} else if strings.Contains(subscribe, `@bookTicker`) {
			handleTickerBinancePerp(markets, result, standardSymbol, updateId)
		} else if strings.Contains(subscribe, `@markPrice`) {
			handleMarkPriceBinancePerp(markets, result, standardSymbol)
		}
	}
	channels = make([]chan struct{}, 0)
	perpSubs := GetWSSubscribes(model.BinancePerp, subType)
	perpChans, perpErr := WebSocketClient(model.BinancePerp, wsBinancePerp, perpSubs,
		subscribeHandlerBinancePerp, wsHandlerBinancePerp, orderHandler, wsStepBinancePerp)
	if perpErr != nil {
		util.SocketInfo(`fail to create binance perp conn %s`, perpErr.Error())
	}
	return perpChans, err
}

var subscribeHandlerBinancePerp = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	subParam := make(map[string]interface{})
	subParam["method"] = "SUBSCRIBE"
	subParam["params"] = subscribes
	subParam["id"] = int(rand.Float64() * 10000)
	subParamJson, _ := json.Marshal(subParam)
	if err = SendToConnection(model.BinancePerp, connection, subParamJson); err != nil {
		util.SocketInfo("binance perp can not subscribe %s %s", subParamJson, err.Error())
	} else {
		util.Notice(fmt.Sprintf(`subscribe %s %s %d`, model.BinancePerp, subParamJson, len(subscribes)))
	}
	time.Sleep(time.Millisecond * 500)
	return err
}

func handleTickerBinancePerp(markets *model.Markets, json *simplejson.Json, standardSymbol string, updateId int64) {
	bidPrice, _ := strconv.ParseFloat(json.Get(`b`).MustString(), 64)
	bidAmount, _ := strconv.ParseFloat(json.Get(`B`).MustString(), 64)
	askPrice, _ := strconv.ParseFloat(json.Get(`a`).MustString(), 64)
	askAmount, _ := strconv.ParseFloat(json.Get(`A`).MustString(), 64)
	ts := json.Get(`E`).MustInt()
	now := int(time.Now().UnixNano() / int64(time.Millisecond))
	if ts == 0 {
		ts = now
	}
	if bidPrice > 0 && bidAmount > 0 && askPrice > 0 && askAmount > 0 {
		bidAsk := model.BidAsk{Ts: ts, TsReceived: now, UpdateId: updateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.BinancePerp, Symbol: standardSymbol, Side: model.OrderSideBuy}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.BinancePerp, Symbol: standardSymbol, Side: model.OrderSideSell}}}
		haveOld, old := markets.GetBidAsk(standardSymbol, model.BinancePerp)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
			return
		}
		if markets.SetBidAsk(standardSymbol, model.BinancePerp, &bidAsk) {
			funcHandlers := GetFunctions(model.BinancePerp, standardSymbol)
			if funcHandlers != nil {
				funcHandlers.Range(func(function, value interface{}) bool {
					setting := GetSetting(function.(string), model.BinancePerp, standardSymbol)
					if setting != nil && value != nil {
						go value.(model.CarryHandler)(setting, &bidAsk)
					}
					return true
				})
			}
		}
	}
}

func handleMarkPriceBinancePerp(markets *model.Markets, json *simplejson.Json, standardSymbol string) {
	markPrice, _ := strconv.ParseFloat(json.Get(`p`).MustString(), 64)
	markets.SetMarkPriceInfo(standardSymbol, model.BinancePerp, &model.MarkPriceInfo{MarkPrice: markPrice, Ts: json.Get(`E`).MustInt()})
	rate, _ := strconv.ParseFloat(json.Get(`r`).MustString(), 64)
	expireTime, _ := strconv.ParseInt(json.Get(`T`).MustString(), 10, 64)
	fundingRate := &model.FundingRate{
		Rate:       rate,
		UpdateTime: util.GetNow(),
		ExpireTime: expireTime / 1000,
	}
	util.Notice(fmt.Sprintf(`binance get market price %s %f %f %d`, standardSymbol, markPrice, rate, expireTime))
	model.SetFundingRate(model.BinancePerp, standardSymbol, fundingRate)
}

func handleDepthBinancePerp(markets *model.Markets, json *simplejson.Json, standardSymbol string, updateId int64) {
	bidAsk := model.BidAsk{UpdateId: updateId}
	var bids, asks []interface{}
	bidAsk.Ts = json.Get(`E`).MustInt()
	bidAsk.TsReceived = int(util.GetNowUnixMillion())
	bidArray, _ := json.Get(`b`).Array()
	bids = bidArray
	askArray, _ := json.Get(`a`).Array()
	asks = askArray
	bidAsk.Bids = make([]model.Tick, len(bids))
	for i, value := range bids {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: model.BinancePerp, Symbol: standardSymbol, Side: model.OrderSideBuy}
	}
	bidAsk.Asks = make([]model.Tick, len(asks))
	for i, value := range asks {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: model.BinancePerp, Symbol: standardSymbol, Side: model.OrderSideSell}
	}
	sort.Sort(bidAsk.Asks)
	sort.Sort(sort.Reverse(bidAsk.Bids))
	if markets.SetBidAsk(standardSymbol, model.BinancePerp, &bidAsk) {
		funcHandlers := GetFunctions(model.BinancePerp, standardSymbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), model.BinancePerp, standardSymbol)
				if setting != nil && value != nil {
					go value.(model.CarryHandler)(setting, &bidAsk)
				}
				return true
			})
		}
	}
}

func maintainChannelBinancePerp(subscribes []interface{}) {
	if !channelMaintainingBinancePerp {
		channelMaintainingBinancePerp = true
		for true {
			time.Sleep(time.Minute * 5)
			err := PongAllConnectionsInterval(model.BinancePerp, 500)
			if err != nil {
				util.SocketInfo("pong binance perp server error " + err.Error())
			}
			needReset := false
			for _, subscribe := range subscribes {
				dialectSymbol := strings.ToUpper(subscribe.(string)[0:strings.Index(subscribe.(string), `@`)])
				success, marketType, coin := model.GetCoinFromDialect(model.BinancePerp, dialectSymbol)
				if !success {
					continue
				}
				_, bidAsk := model.AppMarkets.GetBidAsk(coin+model.UniStandardTail[marketType], model.BinancePerp)
				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 180000 {
					util.Notice(fmt.Sprintf(`fail to get bidask binanceperp %s`, dialectSymbol))
					setRequireReset(model.BinancePerp)
					needReset = true
					break
				}
			}
			if !needReset {
				util.Notice(`no need reset %s`, model.BinancePerp)
			}
		}
	}
}

func placeOrderBinancePerp(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, triggerPrice, amount float64) {
	price, decimal := model.FormatPrice(model.BinancePerp, symbol, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	formattedAmount := model.GetAmountInMarket(model.BinancePerp, symbol, amount, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	order.Price = price
	order.TriggerPrice = price
	if success {
		client := futures.NewClient(key, secret)
		service := client.NewCreateOrderService().Symbol(dialectSymbol).Quantity(amountStr)
		if orderSide == model.OrderSideBuy {
			service.Side(futures.SideTypeBuy)
		} else if orderSide == model.OrderSideSell {
			service.Side(futures.SideTypeSell)
		}
		if orderType == model.OrderTypeMarket {
			service.Type(futures.OrderTypeMarket)
		} else if orderType == model.OrderTypeLimit {
			service.Type(futures.OrderTypeLimit)
			service.Price(priceStr)
			service.TimeInForce(futures.TimeInForceTypeGTC)
		} else if orderType == model.OrderTypeStop {
			stopPrice, stopDecimal := model.FormatPrice(model.BinancePerp, symbol, triggerPrice)
			stopPriceStr := util.CutTailZero(strconv.FormatFloat(stopPrice, 'f', stopDecimal, 64))
			order.TriggerPrice = stopPrice
			service.Type(futures.OrderTypeStop)
			service.Price(priceStr)
			service.StopPrice(stopPriceStr)
			service.PriceProtect(true)
		}
		orderResponse, err := service.Do(context.Background())
		if err != nil {
			util.Notice("placeOrderBinancePerp err: " + err.Error())
			order.OrderId = ``
			order.ErrCode = err.Error()
		} else {
			orderedAmount, _ := strconv.ParseFloat(orderResponse.OrigQuantity, 64)
			_, order.Amount = model.ParseRealAmount(model.BinancePerp, symbol, orderedAmount)
			order.OrderId = strconv.FormatInt(orderResponse.OrderID, 10)
		}
	}
}

func cancelOrderBinancePerp(key, secret, symbol, orderId string) bool {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	if !success {
		return false
	}
	client := futures.NewClient(key, secret)
	orderNum, _ := strconv.ParseInt(orderId, 10, 64)
	res, err := client.NewCancelOrderService().Symbol(dialectSymbol).OrderID(orderNum).Do(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), `code=-2011`) {
			return true
		}
		util.Notice("cancelOrderBinancePerp err: " + err.Error())
		return false
	} else if res.Status == `CANCELED` {
		return true
	}
	return false
}

func cancelOrdersBinancePerp(key, secret string, symbol string) bool {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	if !success {
		return false
	}
	client := futures.NewClient(key, secret)
	err := client.NewCancelAllOpenOrdersService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), `code=-2011`) {
			return true
		}
		util.Notice("cancelOrdersBinancePerp err: " + err.Error())
		return false
	}
	return true
}

// sdk暂不支持该接口
func getPositionsBinancePerp(key, secret string) (success bool, positions []*model.Position, accountValue, availableU float64) {
	responseBody := signedRequestBinance(key, secret, http.MethodGet, restBinancePerp+"/fapi/v2/account", true, nil)
	positionJson, err := util.NewJSON(responseBody)
	if err != nil || positionJson == nil {
		util.SocketInfo(`fail to refresh binance position `)
		time.Sleep(time.Minute * 5)
		return getPositionsBinancePerp(key, secret)
	}
	success = positionJson.Get("canTrade").MustBool()
	if success {
		positions = make([]*model.Position, 0)
		totalBalanceJson := positionJson.Get(`totalWalletBalance`).MustString()
		totalUnrealizedProfitJson := positionJson.Get(`totalUnrealizedProfit`).MustString()
		availableJson := positionJson.Get(`availableBalance`).MustString()
		unrealizedProfit, _ := strconv.ParseFloat(totalUnrealizedProfitJson, 64)
		totalBalance, _ := strconv.ParseFloat(totalBalanceJson, 64)
		accountValue = totalBalance + unrealizedProfit
		availableU, _ = strconv.ParseFloat(availableJson, 64)
		data := positionJson.Get("positions").MustArray()
		for _, item := range data {
			position := &model.Position{Market: model.BinancePerp, Ts: util.GetNowUnixMillion()}
			value := item.(map[string]interface{})
			if value[`symbol`] != nil {
				isSuccess, _, coin := model.GetCoinFromDialect(model.BinancePerp, value[`symbol`].(string))
				if !isSuccess {
					continue
				}
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

// 1m 3m 5m 15m 30m 1h 2h 4h 6h 8h 12h 1d 3d 1w 1M
func getCandlesBinancePerp(key, secret, symbol string, begin, end time.Time, limit, slotSeconds int) (
	candles []*model.Candle, isCache bool) {
	interval := `1D`
	switch slotSeconds {
	case 60:
		interval = `1m`
	case 1800:
		interval = `30m`
	case 3600:
		interval = `1h`
	case 86400:
		interval = `1d`
	}
	param := map[string]interface{}{`symbol`: symbol, `interval`: interval, `startTime`: begin.UnixMilli(), `endTime`: end.UnixMilli(), `limit`: limit}
	redisKey := fmt.Sprintf(`%s_%s_%s_%d_%d_%d`, model.BinancePerp, symbol, interval, begin.UnixMilli(), end.UnixMilli(), limit)
	var responseBody []byte
	if model.AppRedis != nil {
		temp, redisErr := model.AppRedis.Get(context.Background(), redisKey).Result()
		if redisErr == nil {
			responseBody = []byte(temp)
			isCache = true
		}
	}
	if responseBody == nil {
		isCache = false
		responseBody = signedRequestBinance(key, secret, http.MethodGet, restBinancePerp+"/fapi/v1/klines", true, param)
	}
	candleJson, err := util.NewJSON(responseBody)
	errMsg := ``
	if err != nil || candleJson == nil {
		if err != nil {
			errMsg = err.Error()
		}
		util.SocketInfo(`fail to get binance kline %s %s %s %d %s`, symbol, begin.String(), end.String(), slotSeconds, errMsg)
		return
	}
	items, itemErr := candleJson.Array()
	if itemErr != nil || len(items) == 0 {
		if model.AppRedis != nil {
			model.AppRedis.Del(context.Background(), redisKey)
		}
		if itemErr != nil {
			errMsg = itemErr.Error()
		}
		util.SocketInfo(`fail to get binance kline %s %s %s %d %s`, symbol, begin.String(), end.String(), slotSeconds, errMsg)
		return
	} else if !isCache && model.AppRedis != nil {
		util.Notice(fmt.Sprintf(`set candles to cache %s len %d`, redisKey, len(string(responseBody))))
		model.AppRedis.Set(context.Background(), redisKey, string(responseBody), 0)
	}
	candles = make([]*model.Candle, 0)
	for i := 0; i < len(items); i++ {
		candle := &model.Candle{Market: model.BinancePerp, Symbol: symbol, Seconds: slotSeconds}
		value := items[i].([]interface{})
		candle.PriceOpen, _ = strconv.ParseFloat(value[1].(string), 64)
		candle.PriceClose, _ = strconv.ParseFloat(value[4].(string), 64)
		candle.PriceHigh, _ = strconv.ParseFloat(value[2].(string), 64)
		candle.PriceLow, _ = strconv.ParseFloat(value[3].(string), 64)
		beginMilli, _ := value[0].(json.Number).Int64()
		candle.Begin = time.Unix(beginMilli/1000, 0).In(begin.Location())
		candles = append(candles, candle)
	}
	return
}

func signedRequestBinance(key, secret, method, requestUrl string, withApiKey bool, value map[string]interface{}) []byte {
	param := &url.Values{}
	if value != nil {
		for itemKey, itemValue := range value {
			if itemKey == `symbol` {
				_, _, _, itemValue = model.GetFromStandard(model.BinancePerp, itemValue.(string))
			}
			param.Set(itemKey, fmt.Sprintf(`%v`, itemValue))
		}
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

func getFundingRateBinancePerp(key, secret, symbol string) (fundingRate *model.FundingRate) {
	_, marketType, coin, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	client := futures.NewClient(key, secret)
	rateResp, err := client.NewPremiumIndexService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil {
		util.Notice("getFundingRateBinancePerp err: " + err.Error() + " symbol: " + symbol + " marketType: " + marketType + " coin: " + coin + " But dialectSymbol: " + dialectSymbol)
		return
	}
	rateStr := rateResp[0].LastFundingRate
	rate, _ := strconv.ParseFloat(rateStr, 64)
	nextFundingTime := rateResp[0].NextFundingTime
	fundingRate = &model.FundingRate{
		Rate:       rate,
		UpdateTime: util.GetNow(),
		ExpireTime: nextFundingTime / 1000}
	return
}

func queryOpenOrdersBinancePerp(key, secret, symbol string) (orders []*model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	if success {
		orders = make([]*model.Order, 0)
		client := futures.NewClient(key, secret)
		resArray, err := client.NewListOpenOrdersService().Symbol(dialectSymbol).Do(context.Background())
		if err != nil {
			util.Notice(`queryOpenOrdersBinancePerp err ` + err.Error())
		}
		for _, res := range resArray {
			order := &model.Order{Market: model.BinancePerp, Status: model.CarryStatusFail, Symbol: symbol}
			parseOrderBinancePerp(res, order)
			orders = append(orders, order)
		}
	}
	return
}

func parseOrderBinancePerp(res *futures.Order, order *model.Order) {
	if res != nil {
		order.OrderSide = strings.ToLower(string(res.Side))
		order.OrderType = strings.ToLower(string(res.Type))
		order.Amount, _ = strconv.ParseFloat(res.OrigQuantity, 64)
		order.Price, _ = strconv.ParseFloat(res.Price, 64)
		order.DealPrice, _ = strconv.ParseFloat(res.AvgPrice, 64)
		order.DealAmount, _ = strconv.ParseFloat(res.ExecutedQuantity, 64)
		order.OrderTime = time.Unix(res.Time, 0)
		order.Status = model.GetOrderStatus(model.BinancePerp, string(res.Status))
		order.OrderId = strconv.FormatInt(res.OrderID, 10)
		if strings.Contains(string(res.Type), `STOP`) {
			order.OrderType = model.OrderTypeStop
		} else if strings.Contains(string(res.Type), `LIMIT`) {
			order.OrderType = model.OrderTypeLimit
		} else if strings.Contains(string(res.Type), `MARKET`) {
			order.OrderType = model.OrderTypeMarket
		} else {
			order.OrderType = model.OrderTypeLimit
		}
		if order.Status != model.CarryStatusSuccess && order.Status != model.CarryStatusFail {
			order.Status = model.CarryStatusWorking
		}
		if order.DealAmount > 0 && order.DealPrice == 0 {
			order.DealPrice = order.Price
		}
	}
	return
}

func queryOrderBinancePerp(key, secret, symbol string, orderId string) (order *model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	if success {
		orderIdInt, _ := strconv.ParseInt(orderId, 10, 64)
		client := futures.NewClient(key, secret)
		orderResp, err := client.NewGetOrderService().Symbol(dialectSymbol).OrderID(orderIdInt).Do(context.Background())
		if err != nil {
			util.Notice("queryOrderBinancePerp err: " + err.Error())
			return
		}
		order = &model.Order{Market: model.BinancePerp, Status: model.CarryStatusFail, OrderId: orderId, Symbol: symbol}
		parseOrderBinancePerp(orderResp, order)
	}
	return
}

func setPosSideBinancePerp(key, secret string) {
	client := futures.NewClient(key, secret)
	err := client.NewChangePositionModeService().DualSide(false).Do(context.Background())
	if err != nil {
		util.Notice("setPosSideBinancePerp err: " + err.Error())
		return
	}
}

func getPriceBinancePerp(key, secret, symbol string) (success bool, price float64) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	if !success {
		return false, 0
	}
	client := futures.NewClient(key, secret)
	resPrice, err := client.NewListPricesService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil && !strings.Contains(err.Error(), `-2010`) {
		util.Notice(fmt.Sprintf("getPriceBinancePerp err: %s symbol %s %s",
			err.Error(), symbol, dialectSymbol))
		return false, 0
	}
	if len(resPrice) > 0 {
		price, err = strconv.ParseFloat(resPrice[0].Price, 64)
		return err == nil, price
	}
	return true, 0
}

func SetLeverageBinancePerp(key, secret, symbol string, leverage int) (success bool) {
	ok, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	if !ok {
		return false
	}
	client := futures.NewClient(key, secret)
	_, err := client.NewChangeLeverageService().Symbol(dialectSymbol).Leverage(leverage).Do(context.Background())
	if err != nil {
		util.Notice(fmt.Sprintf(`fail to set binanceperp leverage %s %s %d %s`,
			key, symbol, leverage, err.Error()))
		return false
	}
	return true
}

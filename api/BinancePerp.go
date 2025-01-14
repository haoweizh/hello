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
	"hello/model"
	"hello/util"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const restBinancePerp = `https://fapi.binance.com`
const wsBinancePerp = `wss://fstream.binance.com`

var listenKeys sync.Map // market*accountKey listenKey
var listenTime sync.Map // listenKey - time

func MaintainConnsBinance(market string, accounts []*model.Account) {
	for _, account := range accounts {
		model.AppEnvironment.PriConnecting.Store(market+account.Key, false)
	}
	for {
		for _, account := range accounts {
			value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, market, account.Key)
			valueUpdate, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, account.Key)
			if value != nil && valueUpdate != nil {
				keyValue, _ := util.LoadSyncMap(&listenKeys, market, account.Key)
				if keyValue != nil {
					ts, _ := listenTime.Load(keyValue.(string))
					if ts != nil && ts.(time.Time).Add(time.Minute*30).Before(time.Now()) {
						ExtendListenKeyBinance(account, market, keyValue.(string))
					}
				}
			} else {
				if value != nil {
					value.(*model.WSConn).Close()
				}
				if valueUpdate != nil {
					valueUpdate.(*model.WSConn).Close()
				}
				WsOrderServeBinance(account, market)
			}
			GetAccountFromWsAPI(account, wsAccountStatusV2, market)
		}
		time.Sleep(time.Second * 30)
	}
}

func getMarketsBinancePerp(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	client := futures.NewClient(key, secret)
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	stats, errTicker := client.NewListPriceChangeStatsService().Do(context.Background())
	fundingInfos, _ := client.NewFundingRateInfoService().Do(context.Background())
	if err != nil || errTicker != nil {
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("getMarketsBinancePerp err: %s", err.Error()))
		}
		if errTicker != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("getMarketsBinancePerp price err: %s", errTicker.Error()))
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
			marketInfo := &model.MarketInfo{Market: model.BinancePerp, Symbol: symbol, MoneyMin: 5, FundingRateInterval: 8 * 3600000}
			marketInfos[marketInfo.Symbol] = marketInfo
			for _, data := range item.Filters {
				filterType := data[`filterType`].(string)
				switch filterType {
				case `PRICE_FILTER`:
					if data[`tickSize`] != nil {
						marketInfo.PriceIncrement, _ = strconv.ParseFloat(data[`tickSize`].(string), 64)
					}
					marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
				case `LOT_SIZE`:
					marketInfo.SizeMin, _ = strconv.ParseFloat(data[`minQty`].(string), 64)
					marketInfo.SizeMax, _ = strconv.ParseFloat(data[`maxQty`].(string), 64)
					marketInfo.SizeIncrement, _ = strconv.ParseFloat(data[`stepSize`].(string), 64)
				case `MARKET_LOT_SIZE`:
					marketInfo.SizeMaxMarket, _ = strconv.ParseFloat(data[`maxQty`].(string), 64)
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
			marketInfos[marketInfo.Symbol] = marketInfo
		}
	}
	for _, stat := range stats {
		if stat == nil {
			continue
		}
		success, _, symbol := model.GetFromDialect(model.BinancePerp, model.MarketTypePerp, stat.Symbol)
		if !success {
			continue
		}
		if marketInfos[symbol] != nil {
			marketInfos[symbol].TradeAmount, _ = strconv.ParseFloat(stat.QuoteVolume, 64)
		}
	}
	for _, info := range fundingInfos {
		success, _, symbol := model.GetFromDialect(model.BinancePerp, model.MarketTypePerp, info.Symbol)
		if !success {
			continue
		}
		if marketInfos[symbol] != nil {
			marketInfos[symbol].FundingRateInterval = int(info.FundingIntervalHours) * 3600000
		}
	}
	return marketInfos
}

var wsHandlerBinancePerp = func(market string, conn *model.WSConn, event []byte) {
	result, wsErr := util.NewJSON(event)
	if wsErr != nil || result == nil {
		return
	}
	if result.Get(`data`).MustMap() != nil {
		result = result.Get(`data`)
	}
	dialectSymbol := result.Get(`s`).MustString()
	success, _, standardSymbol := model.GetFromDialect(model.BinancePerp, model.MarketTypePerp, dialectSymbol)
	if !success {
		return
	}
	subscribe := result.Get(`e`).MustString()
	updateId := result.Get(`u`).MustInt64()
	var bidAsk *model.BidAsk
	if strings.Contains(subscribe, `depthUpdate`) {
		bidAsk = parseTickDepthBinancePerp(result, standardSymbol, updateId)
	} else if strings.Contains(subscribe, `bookTicker`) {
		bidAsk = parseBookBinancePerp(result, standardSymbol, updateId)
	} else if strings.Contains(subscribe, `markPriceUpdate`) {
		handleMarkPriceBinancePerp(model.AppEnvironment, result, standardSymbol)
		return
	}
	if model.AppEnvironment.SetBidAsk(model.BinancePerp, standardSymbol, bidAsk) {
		funcHandlers := GetFunctions(model.BinancePerp, standardSymbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), model.BinancePerp, standardSymbol)
				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
					go value.(model.CarryHandler)(setting, bidAsk)
				}
				return true
			})
		}
	}
}

func parseBookBinancePerp(json *simplejson.Json, standardSymbol string, updateId int64) (bidAsk *model.BidAsk) {
	bidPrice, _ := strconv.ParseFloat(json.Get(`b`).MustString(), 64)
	bidAmount, _ := strconv.ParseFloat(json.Get(`B`).MustString(), 64)
	askPrice, _ := strconv.ParseFloat(json.Get(`a`).MustString(), 64)
	askAmount, _ := strconv.ParseFloat(json.Get(`A`).MustString(), 64)
	ts := json.Get(`E`).MustInt()
	now := int(time.Now().UnixNano() / int64(time.Millisecond))
	if bidPrice > 0 && bidAmount > 0 && askPrice > 0 && askAmount > 0 {
		bidAsk = &model.BidAsk{Ts: ts, TsReceived: now, UpdateId: updateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.BinancePerp, Symbol: standardSymbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.BinancePerp, Symbol: standardSymbol}}}
	}
	return bidAsk
}

func handleMarkPriceBinancePerp(environment *model.Environment, json *simplejson.Json, standardSymbol string) {
	markPrice, _ := strconv.ParseFloat(json.Get(`p`).MustString(), 64)
	environment.SetMarkPriceInfo(standardSymbol, model.BinancePerp, &model.MarkPriceInfo{MarkPrice: markPrice, Ts: json.Get(`E`).MustInt()})
	rate, _ := strconv.ParseFloat(json.Get(`r`).MustString(), 64)
	fundingRate := &model.FundingRate{
		Rate:       rate,
		UpdateTime: time.UnixMilli(json.Get(`E`).MustInt64()),
		ExpireTime: json.Get(`T`).MustInt64() / 1000,
	}
	//util.Notice(fmt.Sprintf(`binance get market price %s %f %f %d`, standardSymbol, markPrice, rate, fundingRate.ExpireTime))
	SetFundingRate(model.BinancePerp, standardSymbol, fundingRate)
}

func parseTickDepthBinancePerp(json *simplejson.Json, standardSymbol string, updateId int64) (bidAsk *model.BidAsk) {
	bidAsk = &model.BidAsk{UpdateId: updateId}
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
		bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: model.BinancePerp, Symbol: standardSymbol}
	}
	bidAsk.Asks = make([]model.Tick, len(asks))
	for i, value := range asks {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: model.BinancePerp, Symbol: standardSymbol}
	}
	sort.Sort(bidAsk.Asks)
	sort.Sort(sort.Reverse(bidAsk.Bids))
	return bidAsk
}

func ExtendListenKeyBinance(account *model.Account, market, listenKey string) (success bool) {
	if market == model.BinanceSpot {
		res := signedRequestBinance(account.Key, account.Secret, market, http.MethodPut, restBinance+`/api/v3/userDataStream`, false, map[string]interface{}{`listenKey`: listenKey})
		resJson, _ := util.NewJSON(res)
		if resJson != nil && resJson.Get(`code`).MustInt() == 0 {
			return true
		}
	} else {
		res := signedRequestBinance(account.Key, account.Secret, market, http.MethodPut, restBinancePerp+`/fapi/v1/listenKey`, true, nil)
		resJson, _ := util.NewJSON(res)
		if resJson != nil && len(resJson.Get(`listenKey`).MustString()) > 0 {
			resKey := resJson.Get(`listenKey`).MustString()
			listenTime.Store(resKey, time.Now())
			util.Log(util.LogLevelInfo, fmt.Sprintf("ExtendListenKeyBinance extend Listen Key: %s %s", listenKey, market))
			return true
		}
	}
	return false
}

func RenewListenKeyBinance(account *model.Account, market string) (success bool, listenKey string) {
	var response []byte
	if market == model.BinanceSpot {
		//signedRequestBinance(account.Key, account.Secret, model.BinanceSpot, http.MethodDelete,
		//	restBinance+`/api/v3/userDataStream`, true, nil)
		response = signedRequestBinance(account.Key, account.Secret, model.BinanceSpot, http.MethodPost,
			restBinance+`/api/v3/userDataStream`, false, nil)
	} else if market == model.BinancePerp {
		signedRequestBinance(account.Key, account.Secret, model.BinancePerp, http.MethodDelete,
			restBinancePerp+`/fapi/v1/listenKey`, true, nil)
		response = signedRequestBinance(account.Key, account.Secret, model.BinancePerp, http.MethodPost,
			restBinancePerp+`/fapi/v1/listenKey`, true, nil)
	}
	keyJson, _ := util.NewJSON(response)
	if keyJson != nil && len(keyJson.Get(`listenKey`).MustString()) > 0 {
		listenKey = keyJson.Get(`listenKey`).MustString()
		listenTime.Store(listenKey, time.Now())
		util.StoreSyncMap(&listenKeys, listenKey, market, account.Key)
		return true, listenKey
	}
	time.Sleep(time.Second * 3)
	util.Log(util.LogLevelError, `RenewListenKeyBinance fail to renew binanceperp listen key retry`)
	RenewListenKeyBinance(account, market)
	return false, ``
}

func getMarkPriceBinancePerp(account *model.Account, symbol string) (markPrice float64) {
	responseBody := signedRequestBinance(account.Key, account.Secret, model.BinancePerp,
		http.MethodGet, restBinancePerp+"/fapi/v1/premiumIndex", true, map[string]interface{}{`symbol`: symbol})
	markPriceJson, err := util.NewJSON(responseBody)
	if err == nil {
		markPrice, _ = strconv.ParseFloat(markPriceJson.Get(`markPrice`).MustString(), 64)
		mpTime := markPriceJson.Get(`time`).MustInt()
		model.AppEnvironment.SetMarkPriceInfo(symbol, model.BinancePerp, &model.MarkPriceInfo{MarkPrice: markPrice, Ts: mpTime})
	}
	return markPrice
}

// OrderTypeTrailStop时 price: activationPrice(目前不支持，只支持立即触发); triggerPrice: callBackRate
// callbackRate min 0.001, max 0.05 where 0.01 for 1% 注意此处和binance文档中不同，需要额外乘以100
// 特殊的自定义订单ID:
// "autoclose-"开头的字符串: 系统强平订单
// "adl_autoclose": ADL自动减仓订单
// "settlement_autoclose-": 下架或交割的结算订单
func placeOrderBinancePerp(account *model.Account, isWS bool, order *model.Order, orderSide, orderType, orderParam,
	symbol string, oriPrice, triggerPrice, amount float64) {
	price, decimal := model.FormatPrice(model.BinancePerp, symbol, oriPrice)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	reduceOnly := false
	if orderParam == model.ReduceOnly {
		reduceOnly = true
	}
	formattedAmount := model.GetAmountInMarket(model.BinancePerp, symbol, amount, price, reduceOnly)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	order.Price = price
	stopPrice, stopDecimal := model.FormatPrice(model.BinancePerp, symbol, triggerPrice)
	stopPriceStr := util.CutTailZero(strconv.FormatFloat(stopPrice, 'f', stopDecimal, 64))
	order.TriggerPrice = stopPrice
	if !success {
		return
	}
	if isWS {
		if orderSide == model.OrderSideBuy {
			orderSide = string(futures.SideTypeBuy)
		} else if orderSide == model.OrderSideSell {
			orderSide = string(futures.SideTypeSell)
		}
		ts := time.Now().UnixMilli()
		param := url.Values{}
		param.Set("symbol", dialectSymbol)
		param.Set("side", orderSide)
		param.Set("type", strings.ToUpper(orderType))
		param.Set("timeInForce", `GTC`)
		param.Set(`price`, priceStr)
		param.Set(`quantity`, amountStr)
		param.Set(`apiKey`, account.Key)
		param.Set(`newClientOrderId`, order.ClientOrdId)
		param.Set(`timestamp`, fmt.Sprintf(`%d`, ts))
		param.Set(`reduceOnly`, fmt.Sprintf("%v", reduceOnly))
		hash := hmac.New(sha256.New, []byte(account.Secret))
		hash.Write([]byte(param.Encode()))
		msg := fmt.Sprintf(`{"id": "%s","method": "order.place","params":{"symbol": "%s","side": "%s","type": "%s","timeInForce": "GTC",
			"price": "%s","quantity": "%s","apiKey": "%s","signature": "%s","timestamp": %d, "newClientOrderId":"%s","reduceOnly":"%s"}}`,
			order.ClientOrdId, dialectSymbol, orderSide, strings.ToUpper(orderType), priceStr, amountStr, account.Key,
			hex.EncodeToString(hash.Sum(nil)), ts, order.ClientOrdId, fmt.Sprintf("%v", reduceOnly))
		value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, model.BinancePerp, account.Key)
		if value == nil {
			order.Status = model.CarryStatusFail
		} else {
			if err := value.(*model.WSConn).WriteMsg([]byte(msg)); err != nil {
				order.Status = model.CarryStatusFail
				util.Log(util.LogLevelError,
					fmt.Sprintf(`placeOrderBinancePerp fail to place binanceperp order return: %s`, err.Error()))
			}
		}
		if order.Status == model.CarryStatusFail {
			HandleWsOrderConnFail(account, model.BinancePerp, order)
		}
	} else {
		client := futures.NewClient(account.Key, account.Secret)
		service := client.NewCreateOrderService().Symbol(dialectSymbol).Quantity(amountStr)
		if orderSide == model.OrderSideBuy {
			service.Side(futures.SideTypeBuy)
		} else if orderSide == model.OrderSideSell {
			service.Side(futures.SideTypeSell)
		}
		service.ReduceOnly(reduceOnly)
		switch orderType {
		case model.OrderTypeMarket:
			service.Type(futures.OrderTypeMarket)
		case model.OrderTypeLimit:
			service.Type(futures.OrderTypeLimit)
			service.Price(priceStr)
			service.TimeInForce(futures.TimeInForceTypeGTC)
		case model.OrderTypeStop:
			service.Type(futures.OrderTypeStop)
			service.Price(priceStr)
			service.StopPrice(stopPriceStr)
			service.PriceProtect(true)
		case model.OrderTypeTrailStop:
			stopPriceStr = util.CutTailZero(strconv.FormatFloat(100*triggerPrice, 'f', 1, 64))
			service.Type(futures.OrderTypeTrailingStopMarket)
			//if price > 0 {
			//	service.ActivationPrice(priceStr)
			//}
			service.CallbackRate(stopPriceStr)
		}
		service.NewClientOrderID(order.ClientOrdId)
		orderResponse, err := service.Do(context.Background())
		if err != nil {
			util.Log(util.LogLevelError, "placeOrderBinancePerp err: "+err.Error())
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
		//if strings.Contains(err.Error(), `code=-2011`) {
		//	return true
		//}
		util.Log(util.LogLevelError, `cancelOrderBinancePerp fail to cancel binanceperp order`+err.Error())
		return false
	} else if res.Status == `CANCELED` {
		return true
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to cancel order binanceperp %#v`, res))
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
		util.Log(util.LogLevelError, `cancelOrdersBinancePerp fail to cancel orders `+err.Error())
		return false
	}
	return true
}

// sdk暂不支持该接口
func getPositionsBinancePerp(key, secret string) (success bool, positions []*model.Position, accountValue, availableU, mmr float64) {
	responseBody := signedRequestBinance(key, secret, model.BinancePerp, http.MethodGet,
		restBinancePerp+"/fapi/v2/account", true, nil)
	positionJson, err := util.NewJSON(responseBody)
	if err != nil || positionJson == nil {
		util.Log(util.LogLevelError, `getPositionsBinancePerp fail to refresh binance position `)
		time.Sleep(time.Minute)
		return getPositionsBinancePerp(key, secret)
	}
	success = positionJson.Get("canTrade").MustBool()
	unrealizedProfit := 0.0
	var data []interface{}
	if success {
		positions = make([]*model.Position, 0)
		totalBalanceJson := positionJson.Get(`totalWalletBalance`).MustString()
		totalUnrealizedProfitJson := positionJson.Get(`totalUnrealizedProfit`).MustString()
		unrealizedProfit, _ = strconv.ParseFloat(totalUnrealizedProfitJson, 64)
		totalBalance, _ := strconv.ParseFloat(totalBalanceJson, 64)
		accountValue = totalBalance + unrealizedProfit
		availableU, _ = strconv.ParseFloat(positionJson.Get(`availableBalance`).MustString(), 64)
		data, err = positionJson.Get("positions").Array()
		for _, item := range data {
			position := &model.Position{Market: model.BinancePerp, Ts: util.GetNowUnixMillion()}
			value := item.(map[string]interface{})
			if value[`symbol`] != nil {
				isSuccess, _, symbol := model.GetFromDialect(model.BinancePerp, model.MarketTypePerp, value[`symbol`].(string))
				if !isSuccess {
					continue
				}
				position.Currency = symbol
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
			if position.Holding != 0 {
				positions = append(positions, position)
				//util.Log(util.LogLevelInfo, fmt.Sprintf(`get position binanceperp %#v`, position))
			}
		}
		totalMaintMargin, _ := strconv.ParseFloat(positionJson.Get("totalMaintMargin").MustString(), 64)
		if accountValue > 0 {
			mmr = totalMaintMargin / accountValue
		}
	} else {
		util.Log(util.LogLevelError, `getPositionsBinancePerp fail to refresh binance position `)
		time.Sleep(time.Minute)
		return getPositionsBinancePerp(key, secret)
	}
	if (unrealizedProfit != 0 && len(positions) == 0) || err != nil {
		util.Log(util.LogLevelError,
			fmt.Sprintf(`getPositionsBinancePerp pos error binanceperp %f %f %f 0 pos items %d`, accountValue, availableU, unrealizedProfit, len(data)))
		time.Sleep(time.Minute)
		return getPositionsBinancePerp(key, secret)
	}
	return success, positions, accountValue, availableU, mmr
}

// 1m 3m 5m 15m 30m 1h 2h 4h 6h 8h 12h 1d 3d 1w 1M
func getCandlesBinance(account *model.Account, market, symbol string, begin, end time.Time, limit, slotSeconds int) (
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
	redisKey := fmt.Sprintf(`%s_%s_%s_%d_%d_%d`, market, symbol, interval, begin.UnixMilli(), end.UnixMilli(), limit)
	var responseBody []byte
	if model.AppRedis != nil {
		temp, redisErr := model.AppRedis.Get(context.Background(), redisKey).Result()
		if redisErr == nil {
			responseBody = util.UnGzip([]byte(temp))
			isCache = true
			//util.Notice(fmt.Sprintf(`get candles from key %s %d`, redisKey, len(temp)))
		}
	}
	if responseBody == nil {
		isCache = false
		if market == model.BinanceSpot {
			responseBody = signedRequestBinance(account.Key, account.Secret, market, http.MethodGet,
				restDataBinance+"/api/v3/klines", false, param)
		} else if market == model.BinancePerp {
			responseBody = signedRequestBinance(account.Key, account.Secret, market, http.MethodGet,
				restBinancePerp+"/fapi/v1/klines", true, param)
		}
	}
	candleJson, err := util.NewJSON(responseBody)
	errMsg := ``
	if err != nil || candleJson == nil {
		if err != nil {
			errMsg = err.Error()
		}
		util.Log(util.LogLevelError, fmt.Sprintf(`getCandlesBinance fail to get binance kline %s %s %s %d %s`,
			symbol, begin.String(), end.String(), slotSeconds, errMsg))
		return
	}
	items, itemErr := candleJson.Array()
	if itemErr != nil || len(items) == 0 {
		if model.AppRedis != nil {
			model.AppRedis.Del(context.Background(), redisKey)
			util.Log(util.LogLevelError, fmt.Sprintf(`del redis key %s`, redisKey))
		}
		if itemErr != nil {
			errMsg = itemErr.Error()
		}
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to get binance kline %s %s %s %d %s`,
			symbol, begin.String(), end.String(), slotSeconds, errMsg))
		return
	} else if !isCache && model.AppRedis != nil {
		val := util.Compress(responseBody)
		util.Log(util.LogLevelError, fmt.Sprintf(`set candles to cache %s len %d compress %d`, redisKey, len(responseBody), len(val)))
		model.AppRedis.Set(context.Background(), redisKey, val, 0)
	}
	candles = make([]*model.Candle, 0)
	for i := 0; i < len(items); i++ {
		candle := &model.Candle{Market: market, Symbol: symbol, Seconds: slotSeconds}
		value := items[i].([]interface{})
		candle.PriceOpen, _ = strconv.ParseFloat(value[1].(string), 64)
		candle.PriceClose, _ = strconv.ParseFloat(value[4].(string), 64)
		candle.PriceHigh, _ = strconv.ParseFloat(value[2].(string), 64)
		candle.PriceLow, _ = strconv.ParseFloat(value[3].(string), 64)
		candle.Volume, _ = strconv.ParseFloat(value[7].(string), 64)
		beginMilli, _ := value[0].(json.Number).Int64()
		candle.Begin = time.Unix(beginMilli/1000, 0).In(begin.Location())
		candles = append(candles, candle)
	}
	return
}

func signedRequestBinance(key, secret, market, method, requestUrl string, withApiKey bool, value map[string]interface{}) []byte {
	param := &url.Values{}
	if value != nil {
		for itemKey, itemValue := range value {
			if itemKey == `symbol` {
				_, _, _, itemValue = model.GetFromStandard(market, itemValue.(string))
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
	if len(param.Encode()) > 0 {
		requestUrl = requestUrl + "?" + param.Encode()
	}
	responseBody, _ := util.HttpRequest(method, requestUrl, "", headers, 60)
	//logMsg := fmt.Sprintf(`signedRequestBinance binance key %s request %s body %#v return %s`,
	//	key, requestUrl, param, string(responseBody))
	//if strings.Contains(requestUrl, `/order`) {
	//	util.Log(util.LogLevelInfo, logMsg)
	//} else if !strings.Contains(requestUrl, `exchangeInfo`) {
	//	util.Log(util.LogLevelInfo, logMsg)
	//}
	responseJson, err := util.NewJSON(responseBody)
	if err != nil {
		util.Log(util.LogLevelError, `signedRequestBinance fail to parse json `+err.Error())
		return nil
	}
	if responseJson == nil {
		util.Log(util.LogLevelError, `signedRequestBinance no response data`)
		return nil
	}
	code := responseJson.Get(`code`).MustInt()
	if code != 0 && code != -3027 && code != 200 && code != -2011 {
		util.Log(util.LogLevelError, fmt.Sprintf(`signedRequestBinance request err %d`, code))
	}
	return responseBody
}

func getFundingRateBinancePerp(key, secret, symbol string) (fundingRate *model.FundingRate) {
	_, marketType, coin, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	client := futures.NewClient(key, secret)
	rateResp, err := client.NewPremiumIndexService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil {
		util.Log(util.LogLevelError, err.Error()+" getFundingRateBinancePerp symbol: "+symbol+" marketType: "+marketType+" coin: "+coin+" But dialectSymbol: "+dialectSymbol)
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
	if success || symbol == `` {
		orders = make([]*model.Order, 0)
		listOpenOrderService := futures.NewClient(key, secret).NewListOpenOrdersService()
		if symbol != `` {
			listOpenOrderService = listOpenOrderService.Symbol(dialectSymbol)
		}
		resArray, err := listOpenOrderService.Do(context.Background())
		if err != nil {
			util.Log(util.LogLevelError, `queryOpenOrdersBinancePerp`+err.Error())
		}
		for _, res := range resArray {
			order := &model.Order{Market: model.BinancePerp, Status: model.CarryStatusFail}
			parseOrderBinancePerp(res, order)
			orders = append(orders, order)
		}
	}
	return
}

// parseOrderJsBinance
//func _(market string, json *simplejson.Json) (order *model.Order) {
//	if json == nil {
//		return nil
//	}
//	order = &model.Order{Market: market}
//	symbol := json.Get(`s`).MustString()
//	success, marketType, coin := model.GetFromDialect(market, symbol)
//	if success {
//		order.Coin = coin
//		order.Symbol = coin + model.UniStandardTail[marketType]
//	}
//	if strings.EqualFold(json.Get(`S`).MustString(), model.OrderSideSell) {
//		order.OrderSide = model.OrderSideSell
//	} else if strings.EqualFold(json.Get(`S`).MustString(), model.OrderSideBuy) {
//		order.OrderSide = model.OrderSideBuy
//	}
//	order.OrderType = GetStandardOrderType(market, json.Get(`o`).MustString())
//	order.Amount, _ = strconv.ParseFloat(json.Get("q").MustString(), 64)
//	order.Price, _ = strconv.ParseFloat(json.Get(`p`).MustString(), 64)
//	order.DealPrice, _ = strconv.ParseFloat(json.Get("ap").MustString(), 64)
//	if marketType == model.MarketTypeSpot {
//		order.DealPrice, _ = strconv.ParseFloat(json.Get(`L`).MustString(), 64)
//	}
//	order.DealAmount, _ = strconv.ParseFloat(json.Get("z").MustString(), 64)
//	order.TriggerPrice, _ = strconv.ParseFloat(json.Get("sp").MustString(), 64)
//	order.OrderId = strconv.Itoa(json.Get("i").MustInt())
//	order.Fee, _ = strconv.ParseFloat(json.Get("n").MustString(), 64)
//	order.Status = model.GetOrderStatus(market, json.Get("X").MustString())
//	return order
//}

func parseOrderBinancePerp(res *futures.Order, order *model.Order) {
	if res != nil {
		if strings.Contains(strings.ToLower(string(res.Side)), `buy`) {
			order.OrderSide = model.OrderSideBuy
		} else if strings.Contains(strings.ToLower(string(res.Side)), `sell`) {
			order.OrderSide = model.OrderSideSell
		}
		_, _, order.Symbol = model.GetFromDialect(model.BinancePerp, model.MarketTypePerp, res.Symbol)
		order.Amount, _ = strconv.ParseFloat(res.OrigQuantity, 64)
		order.Price, _ = strconv.ParseFloat(res.Price, 64)
		order.DealPrice, _ = strconv.ParseFloat(res.AvgPrice, 64)
		order.DealAmount, _ = strconv.ParseFloat(res.ExecutedQuantity, 64)
		order.OrderTime = time.UnixMilli(res.Time)
		order.OrderUpdateTime = time.UnixMilli(res.UpdateTime)
		order.Status = model.GetOrderStatus(model.BinancePerp, string(res.Status))
		order.OrderId = strconv.FormatInt(res.OrderID, 10)
		order.ClientOrdId = res.ClientOrderID
		orderType := strings.Trim(string(res.Type), ` `)
		order.OrderType = GetStandardOrderType(model.BinancePerp, orderType)
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
		order = &model.Order{Market: model.BinancePerp, Status: model.CarryStatusFail, OrderId: orderId}
		if err != nil {
			util.Log(util.LogLevelError,
				fmt.Sprintf("queryOrderBinancePerp err %s id %s  err %s", symbol, orderId, err.Error()))
			if strings.Contains(err.Error(), `-2013`) {
				return nil
			}
			return
		}
		parseOrderBinancePerp(orderResp, order)
	}
	return
}

func setPosSideBinancePerp(key, secret string) {
	client := futures.NewClient(key, secret)
	err := client.NewChangePositionModeService().DualSide(false).Do(context.Background())
	if err != nil {
		util.Log(util.LogLevelError, `setPosSideBinancePerp`+err.Error())
		return
	}
}

// getPriceBinancePerp
func _(key, secret, symbol string) (success bool, price float64) {
	var dialectSymbol string
	success, _, _, dialectSymbol = model.GetFromStandard(model.BinancePerp, symbol)
	if !success {
		return false, 0
	}
	client := futures.NewClient(key, secret)
	resPrice, err := client.NewListPricesService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil && !strings.Contains(err.Error(), `-2010`) {
		util.Log(util.LogLevelError, fmt.Sprintf("getPriceBinancePerp err: %s symbol %s %s", err.Error(), symbol, dialectSymbol))
		return false, 0
	}
	if len(resPrice) > 0 {
		price, err = strconv.ParseFloat(resPrice[0].Price, 64)
		return err == nil, price
	}
	return true, 0
}

func setSymbolLeverageBinancePerp(account *model.Account, symbol string) (success bool) {
	ok, _, coin, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	if !ok {
		return false
	}
	client := futures.NewClient(account.Key, account.Secret)
	leverage := model.DefaultLeverage
	if model.CommonCoins[strings.ToLower(coin)] {
		leverage = 10
	}
	_, err := client.NewChangeLeverageService().Symbol(dialectSymbol).Leverage(leverage).Do(context.Background())
	if err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`setSymbolLeverageBinancePerp fail to set binanceperp leverage %s %d %s`,
			symbol, model.DefaultLeverage, err.Error()))
		return false
	}
	return true
}

func setLeverageBinancePerp(key, secret string) (success bool) {
	symbols := GetMarketSymbols(model.BinancePerp)
	for symbol, value := range symbols {
		if value == false {
			continue
		}
		ok, _, coin, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
		if !ok {
			continue
		}
		client := futures.NewClient(key, secret)
		leverage := model.DefaultLeverage
		if model.CommonCoins[strings.ToLower(coin)] {
			leverage = 10
		}
		_, err := client.NewChangeLeverageService().Symbol(dialectSymbol).Leverage(leverage).Do(context.Background())
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`setLeverageBinancePerp fail to set binanceperp leverage %s %s %d %s`,
				key, symbol, model.DefaultLeverage, err.Error()))
			continue
		}
		time.Sleep(time.Millisecond * 200)
	}
	return true
}

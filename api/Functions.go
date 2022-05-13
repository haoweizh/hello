package api

import (
	"fmt"
	"hello/model"
	"hello/util"
	"strconv"
	"strings"
	"sync"
	"time"
)

var requireReset sync.Map
var symbolLock sync.Mutex
var tradeMax = make(map[string]map[string][]float64)        // key - symbol - [maxBuy合约张数/币币个数, maxSell]
var okTradeMaxResetTime = make(map[string]map[string]int64) // key - symbol - init time in second
var okexCrossing sync.Map                                   // symbol - bool
var USDs = map[string]bool{`USD`: true, `usd`: true, `USDT`: true, `usdt`: true, `USDC`: true, `usdc`: true, `BUSD`: true, `busd`: true}

//var crossCount = make(map[string]map[string]map[string]int) // key - market - symbol - count
//func ClearCrossCount() {
//	crossCount = make(map[string]map[string]map[string]int)
//	okexCrossing = sync.Map{}
//}
//
//func GetCrossCount(key, market, symbol string) (count int) {
//	if crossCount == nil || crossCount[key] == nil || crossCount[key][market] == nil {
//		return 0
//	}
//	return crossCount[key][market][symbol]
//}
//
//func SetCrossCount(key, market, symbol string, count int) {
//	if crossCount == nil {
//		crossCount = make(map[string]map[string]map[string]int)
//	}
//	if crossCount[key] == nil {
//		crossCount[key] = make(map[string]map[string]int)
//	}
//	if crossCount[key][market] == nil {
//		crossCount[key][market] = make(map[string]int)
//	}
//	if market == model.OKEX {
//		okexCrossing.Store(symbol, true)
//	}
//	crossCount[key][market][symbol] = count
//}

func setRequireReset(market string) {
	maintaining, ok := model.ChannelMaintaining.Load(market)
	if !ok || !maintaining.(bool) {
		util.Notice(`require reset %s`, market)
		initTime, getTime := model.AppMarkets.WsInitTime.Load(market)
		if getTime && initTime != nil {
			duration, _ := time.ParseDuration(`600s`)
			checkTime := initTime.(time.Time).Add(duration)
			if util.GetNow().After(checkTime) {
				requireReset.Store(market, true)
			} else {
				util.Notice(`just reset ws channel ignore %s reset after %v`, market, checkTime)
			}
		}
	}
}

func GetTradeMaxOKEX(key, secret, symbol string, expireSecond int64) (success bool, maxBuy, maxSell float64) {
	defer symbolLock.Unlock()
	symbolLock.Lock()
	now := time.Now().Unix()
	if expireSecond < 0 {
		if tradeMax[key] != nil && tradeMax[key][symbol] != nil && len(tradeMax[key][symbol]) == 2 {
			return true, tradeMax[key][symbol][0], tradeMax[key][symbol][1]
		} else {
			return false, 0, 0
		}
	}
	if tradeMax[key] != nil && tradeMax[key][symbol] != nil && len(tradeMax[key][symbol]) == 2 &&
		okTradeMaxResetTime[key] != nil && now-okTradeMaxResetTime[key][symbol] < expireSecond {
		return true, tradeMax[key][symbol][0], tradeMax[key][symbol][1]
	}
	if tradeMax[key] == nil {
		tradeMax[key] = make(map[string][]float64)
	}
	if okTradeMaxResetTime[key] == nil {
		okTradeMaxResetTime[key] = make(map[string]int64)
	}
	success, maxBuy, maxSell = getMaxSizeOKEX(key, secret, symbol)
	if success {
		tradeMax[key][symbol] = []float64{maxBuy, maxSell}
		okTradeMaxResetTime[key][symbol] = now
	}
	return
}

func RequireDepthChanReset(markets *model.Markets, market string) bool {
	needReset, _ := requireReset.Load(market)
	if needReset != nil && needReset.(bool) {
		requireReset.Store(market, false)
		util.Notice(`clear need reset for market: ` + market)
		return true
	}
	now := util.GetNowUnixMillion()
	symbols := markets.GetSymbols()
	for symbol := range symbols {
		_, bidAsk := markets.GetBidAsk(symbol, market)
		if bidAsk == nil {
			continue
		}
		delay := float64(now - int64(bidAsk.Ts))
		if delay < model.AppConfig.Delay {
			return false
		}
	}
	util.Notice(fmt.Sprintf(`need reset %d %s %f`, now, market, model.AppConfig.Delay))
	return true
}

func MustCancel(key, secret, market, symbol, orderType, orderId string, mustCancel bool) (res bool) {
	sleepTime := 1
	for i := 0; i < 20; i++ {
		result, errCode, _ := CancelOrder(key, secret, market, symbol, orderType, orderId)
		res = result
		util.Notice(fmt.Sprintf(`[cancel] %s %s %s %s for %d times, return %t `,
			market, symbol, orderType, orderId, i, result))
		if result || !mustCancel || errCode == `0` {
			return result
		}
		if errCode == `3008` && i >= 3 {
			return result
		}
		//if result || !mustCancel { //3008:"submit cancel invalid order state
		//	break
		//} else if errCode == `429` || errCode == `4003` {
		//	util.Notice(`调用次数繁忙`)
		//}
		time.Sleep(time.Second * time.Duration(sleepTime))
		sleepTime *= 2
	}
	return res
}

// CancelOrders 暂不支持策略订单
func CancelOrders(key, secret, market, symbol string) (result bool) {
	switch market {
	case model.Kucoin:
		return cancelOrdersKucoin(symbol)
	case model.Gate:
		return cancelOrdersGate(key, secret, symbol)
	case model.Mexc:
		return cancelOrdersMexc(key, secret, symbol)
	case model.BinanceSpot:
		return cancelOrdersBinanceSpot(key, secret, symbol)
	case model.BinancePerp:
		return cancelOrdersBinancePerp(key, secret, symbol)
	case model.Ftx:
		return cancelOrdersFtx(key, secret, symbol)
	case model.BybitPerp:
		return cancelOrdersBybitPerp(key, secret, symbol)
	case model.BybitSpot:
		return cancelOrdersBybitSpot(key, secret, symbol)
	case model.OKEX:
		result, _, _ = cancelOrdersOKEX(key, secret, symbol)
		return result
	}
	return false
}

// CancelOrder 支持普通订单、stop订单
func CancelOrder(key, secret, market, symbol, orderType, orderId string) (result bool, errCode, msg string) {
	if model.AppConfig.Env == `test` {
		return true, ``, `test cancel`
	}
	errCode = `market-not-supported ` + market
	msg = `market not supported ` + market
	switch market {
	case model.OKEX:
		result, errCode, msg = cancelOrderOkex(key, secret, symbol, orderId, orderType)
	case model.BybitPerp:
		result, errCode, msg = cancelOrderBybitPerp(key, secret, symbol, orderId)
	case model.BybitSpot:
		result, errCode, msg = cancelOrderBybitSpot(key, secret, symbol, orderId)
	case model.Ftx:
		result = cancelOrderFtx(key, secret, orderType, orderId)
	case model.Gate:
		result = cancelOrderGate(key, secret, symbol, orderId)
	}
	util.Notice(fmt.Sprintf(`[cancel %s %v %s %s]`, orderId, result, market, symbol))
	return result, errCode, msg
}

func GetDayCandle(key, secret, market, symbol string, timeCandle time.Time) (candle *model.Candle) {
	//candle = model.GetCandle(market, symbol, `1d`, timeCandle.Format(time.RFC3339)[0:10])
	//if candle != nil && candle.N > 0 {
	//	return
	//}
	dBegin, _ := time.ParseDuration(`-960h`)
	dEnd, _ := time.ParseDuration(`24h`)
	begin := timeCandle.Add(dBegin)
	end := timeCandle.Add(dEnd)
	var candles map[string]*model.Candle
	switch market {
	//case model.Bitmex:
	//	candles = deprecated.getCandlesBitmex(key, secret, symbol, `1d`, begin, end, 20)
	case model.Ftx:
		candles = getCandlesFtx(key, secret, symbol, `1d`, begin, end, 40)
	case model.OKEX:
		candles = getCandlesOKEX(key, secret, symbol, `1D`, begin, end, 40)
	}
	keyedCandles := make(map[string]*model.Candle)
	for _, value := range candles {
		candleKey := market + symbol + value.Period + value.UTCDate
		keyedCandles[candleKey] = value
	}
	candleKey := market + symbol + `1d` + timeCandle.Format(time.RFC3339)[0:10]
	candle = keyedCandles[candleKey]
	if candle == nil {
		util.Notice(fmt.Sprintf(`error: can not get candle %s`, candleKey))
		return
	}
	candle.N = (candle.PriceHigh - candle.PriceLow) / 20
	for i := 1; i < 20; i++ {
		d, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		index := timeCandle.Add(d)
		currentKey := market + symbol + `1d` + index.Format(time.RFC3339)[0:10]
		candleCurrent := keyedCandles[currentKey]
		if candleCurrent == nil {
			util.Notice(fmt.Sprintf(`error: can not get candle %s %s`, currentKey, index.String()))
			continue
		}
		if candleCurrent.N > 0 {
			if i == 1 {
				candle.N += candleCurrent.N * 19 / 20
				break
			}
			candle.N += candleCurrent.N / 20
		} else {
			candle.N += (candleCurrent.PriceHigh - candleCurrent.PriceLow) / 20
		}
	}
	return candle
}

func GetBalances(key, secret, market string) (
	success bool, balances []*model.Balance, totalInUsd float64, collateral *model.Collateral) {
	//now := util.GetNow().Unix()
	//var update int64
	//balances, totalInUsd, collateral, update = model.GetBalance(market)
	//if now-update < delaySeconds {
	//	return true, balances, totalInUsd, collateral
	//}
	switch market {
	case model.Kucoin:
		success, balances = getBalanceKucoin(key, secret)
	case model.Gate:
		success, balances = getBalanceGate(key, secret)
	case model.Ftx:
		success, balances, totalInUsd = getBalanceFtx(key, secret)
	case model.OKEX:
		success, balances, totalInUsd, collateral = getBalanceOKEX(key, secret)
	//case model.Binance:
	//	success, balances = getBalanceBinance(key, secret)
	case model.BinanceSpot:
		success, balances = getBalanceBinanceSpot(key, secret)
	case model.BybitSpot:
		success, balances = getBalanceBybitSpot(key, secret)
	}
	if market != model.Ftx && market != model.OKEX {
		for _, balance := range balances {
			got, tick := model.AppMarkets.GetBidAsk(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], market)
			if got {
				totalInUsd += tick.Bids[0].Price * balance.Amount
			} else {
				if USDs[balance.Coin] {
					totalInUsd += balance.Amount
				}
			}
		}
	}
	return success, balances, totalInUsd, collateral
}

func GetTransfers(key, secret, market string) (balances []*model.Balance) {
	switch market {
	case model.Ftx:
		return getTransferFtx(key, secret)
	case model.OKEX:
		return getTransferOKEX(key, secret)
	}
	return balances
}

func GetFundingRate(key, secret, market, symbol string) (success bool, rate float64) {
	//非永续合约的资金费率为0
	_, marketType, _, _ := model.GetFromStandard(market, symbol)
	if marketType != model.MarketTypePerp {
		return true, 0
	}
	fundingRate := model.GetFundingRate(market, symbol)
	now := util.GetNow().Unix()
	if market == model.OKEX { // 针对ok用新expireTime返回旧数据问题的特殊处理
		if fundingRate != nil && now < fundingRate.ExpireTime-60 {
			return true, fundingRate.Rate
		} else if fundingRate != nil && now > fundingRate.ExpireTime-60 && now < fundingRate.ExpireTime+240 &&
			fundingRate.UpdateTime > fundingRate.ExpireTime-60 {
			if now < fundingRate.ExpireTime {
				return true, fundingRate.Rate
			} else {
				return true, fundingRate.RateNext
			}
		}
	} else {
		if fundingRate != nil && now < fundingRate.ExpireTime {
			if fundingRate.ExpireTime-now < 3600 && fundingRate.ExpireTime%28800 != 0 {
				// 临近1小时内，资金费率按8倍计算（此方法不够严谨）
				return true, fundingRate.Rate * 8
			}
			return true, fundingRate.Rate
		}
	}
	switch market {
	//case model.Bitmex:
	//	rate, expireTime = deprecated.getFundingRateBitmex(key, secret, symbol)
	//	model.SetFundingRate(market, symbol, &model.FundingRate{Rate: rate, ExpireTime: expireTime, UpdateTime: now})
	case model.BybitPerp:
		fundingRate = getFundingRateBybitPerp(key, secret, symbol)
		model.SetFundingRate(market, symbol, fundingRate)
	case model.BybitSpot:
		return true, 0
	case model.Ftx:
		fundingRate = GetFundingRatesFtx(key, secret, symbol)
		fundingRate.Rate *= 4
		model.SetFundingRate(market, symbol, fundingRate)
	case model.OKEX:
		fundingRate = getFundingRateOKEX(key, secret, symbol)
		model.SetFundingRate(market, symbol, fundingRate)
	case model.Mexc:
		fundingRate = getFundingRateMexc(key, secret, symbol)
		model.SetFundingRate(market, symbol, fundingRate)
	case model.BinanceSpot:
		return true, 0
	case model.BinancePerp:
		fundingRate = getFundingRateBinancePerp(key, secret, symbol)
		model.SetFundingRate(market, symbol, fundingRate)
	case model.Gate:
		fundingRate = getFundingRateGate(key, secret, symbol)
		model.SetFundingRate(market, symbol, fundingRate)
	case model.Kucoin:
		return true, 0
	}
	if fundingRate != nil && now < fundingRate.ExpireTime {
		return true, fundingRate.Rate
	}
	return false, 0
}

// GetMaxLoan
func _(key, secret, market, symbol string) (success bool, maxLoan float64) {
	switch market {
	case model.Gate:
		return getMaxLoanGate(symbol)
	case model.OKEX:
		return getMaxLoanOKEX(key, secret, symbol)
	}
	return false, 0
}

func QueryOpenTriggerOrders(key, secret, market, symbol string) (orders []*model.Order) {
	switch market {
	case model.OKEX:
		return queryOpenOrdersOKEX(key, secret, symbol, true)
	case model.Ftx:
		return queryTriggerOrdersFtx(key, secret, symbol)
	}
	return nil
}

func QueryOrderById(key, secret, market, symbol, orderType, orderId string) (order *model.Order) {
	order = &model.Order{
		OrderId: orderId, Symbol: symbol, Market: market, OrderType: orderType, Status: model.CarryStatusFail}
	switch market {
	case model.Gate:
		queryOrderGate(key, secret, order)
	case model.OKEX:
		order = queryOrderOKEX(key, secret, symbol, orderId, orderType)
		if order != nil {
			order.Market = market
			order.Symbol = symbol
		}
		return order
	case model.BinanceSpot:
		order = queryOrderBinanceSpot(key, secret, symbol, orderId)
	case model.BinancePerp:
		order = queryOrderBinancePerp(key, secret, symbol, orderId)
	case model.BybitPerp:
		order = queryOrderBybitPerp(key, secret, symbol, orderId)
	case model.BybitSpot:
		order = queryOrderBybitSpot(key, secret, orderId)
	case model.Ftx: // 查询是否是待成交状态，如果已成交或已取消，则ftx返回order not found信息，order为nil
		if orderType == model.OrderTypeStop {
			newOrderId := queryTriggerOrderId(key, secret, orderId)
			if newOrderId != `` {
				return queryOrderFtx(key, secret, newOrderId)
			} else {
				order.Status = queryTriggerOrderFtx(key, secret, symbol, orderId)
			}
		} else {
			return queryOrderFtx(key, secret, orderId)
		}
	case model.Mexc:
		order = queryOrderMexc(key, secret, symbol, orderId)
	}
	return order
}

// GetPositions
// accountValue: 账户权益
// availableU: 可用usd
func GetPositions(key, secret, market string) (success bool, positions []*model.Position, accountValue, availableU float64) {
	switch market {
	case model.Kucoin:
		return getPositionsKucoin(key, secret)
	case model.Gate:
		return getPositionsGate(key, secret)
	case model.Mexc:
		return getPositionsMexc(key, secret)
	case model.BinancePerp:
		return getPositionsBinancePerp(key, secret)
	case model.Ftx:
		var balances []*model.Balance
		success, balances, accountValue = getBalanceFtx(key, secret)
		for _, balance := range balances {
			if strings.EqualFold(balance.Coin, `usd`) {
				availableU = balance.Amount
			}
		}
		success, positions, _ = getPositionsFtx(key, secret)
		return success, positions, accountValue, availableU
	case model.BybitPerp:
		return getPositionsBybitPerp(key, secret)
	case model.OKEX:
		_, _, total, collateral := getBalanceOKEX(key, secret)
		success, positions = getPositionsOKEX(key, secret)
		return success, positions, total, collateral.Available
	}
	return false, nil, 0, 0
}

func MustPlaceOrder(key, secret, orderSide, orderType, market, symbol, orderParam,
	refreshType string, price, triggerPrice, amount float64, saveDB, isWs bool, setting *model.Setting) (order *model.Order) {
	retry := 10
	for i := 0; i < retry; i++ {
		order = PlaceOrder(key, secret, orderSide, orderType, market, symbol,
			orderParam, price, triggerPrice, amount, isWs, nil, setting)
		if order != nil && order.OrderId != `` && order.Status != model.CarryStatusFail {
			break
		} else {
			//if market == model.OKSwap && order != nil && order.ErrCode == `35010` {
			//	amountType = model.AmountTypeNew
			//	RefreshAccount(key, secret, model.OKSwap)
			//}
			time.Sleep(time.Second * 3)
			util.Notice(fmt.Sprintf(`fail to place order %d time, re order`, i))
		}
	}
	order.RefreshType = refreshType
	if saveDB {
		go model.AppDB.Save(order)
	}
	return order
}

// PlaceOrder orderSide: OrderSideBuy OrderSideSell OrderSideLiquidateLong OrderSideLiquidateShort
// orderType: OrderTypeLimit OrderTypeMarket
// amount:如果是限价单或市价卖单，amount是左侧币种的数量，如果是市价买单，amount是右测币种的数量
func PlaceOrder(key, secret, orderSide, orderType, market, symbol, orderParam string, price, triggerPrice,
	amount float64, isWs bool, postOrder model.PostOrder, setting *model.Setting) (order *model.Order) {
	start := util.GetNowUnixMillion()
	markSide := model.OrderSideBuy
	switch orderSide {
	case model.OrderSideBuy, model.OrderSideLiquidateShort:
		markSide = model.OrderSideBuy
	case model.OrderSideSell, model.OrderSideLiquidateLong:
		markSide = model.OrderSideSell
	}
	if amount < 0.0001 {
		util.Notice(`can not place order with amount 0`)
		return &model.Order{OrderSide: markSide, OrderType: orderType, Market: market, Symbol: symbol,
			Price: price, Amount: 0, OrderId: ``, ErrCode: ``, TriggerPrice: triggerPrice,
			Status: model.CarryStatusFail, DealAmount: 0, DealPrice: price, OrderTime: util.GetNow()}
	}
	order = &model.Order{OrderSide: markSide, OrderType: orderType, Market: market, Symbol: symbol, Price: price,
		Amount: amount, DealAmount: 0, DealPrice: price, TriggerPrice: triggerPrice,
		OrderTime: util.GetNow(), UnfilledQuantity: amount, AmountType: key}
	//util.Notice(fmt.Sprintf(`...%s %s %s before order %d amount: %f price:%f triggerPrice:%f`,
	//	orderSide, market, symbol, start, amount, price, triggerPrice))
	if model.AppConfig.Env == `test` {
		return
	}
	switch market {
	case model.Kucoin:
		placeOrderKucoin(order, orderSide, orderType, symbol, price, amount)
	case model.Gate:
		placeOrderGate(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.OKEX:
		placeOrderOKEX(key, secret, isWs, order)
	case model.BinanceSpot:
		placeOrderBinanceSpot(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.BinancePerp:
		placeOrderBinancePerp(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.BybitPerp:
		placeOrderBybitPerp(order, key, secret, orderSide, orderType, orderParam, symbol, price, amount)
	case model.BybitSpot:
		placeOrdersBybitSpot(order, key, secret, orderSide, orderType, orderParam, symbol, price, amount)
	case model.Ftx:
		placeOrderFtx(order, key, secret, orderSide, orderType, orderParam, symbol, price, triggerPrice, amount)
	case model.Mexc:
		placeOrderMexc(key, secret, order, orderSide, orderType, symbol, price, amount)
	}
	if order.OrderId == "0" || strings.Trim(order.OrderId, ` `) == "" {
		order.Status = model.CarryStatusFail
		order.OrderId = fmt.Sprintf(`%s_error_%d`, order.ErrCode, time.Now().UnixNano())
	} else if order.Status == `` {
		order.Status = model.CarryStatusWorking
	}
	end := util.GetNowUnixMillion()
	util.Notice(fmt.Sprintf(`...%s %s %s return order at %d distance %d %s %s price %f id %s`,
		orderSide, market, symbol, end, end-start, order.Status, order.ErrCode, order.Price, order.OrderId))
	if isWs && market == model.OKEX {
		order.Status = model.CarryStatusSuccess
		order.OrderId = strconv.FormatInt(time.Now().UnixNano(), 10) + symbol
	}
	if postOrder != nil && setting.Market != model.OKEX {
		go postOrder(order, setting)
	}
	return order
}

func GetWSSubscribes(market, subType string) []interface{} {
	symbols := model.GetMarketSymbols(market)
	subscribes := make([]interface{}, 0)
	for symbol := range symbols {
		subTypes := strings.Split(subType, `,`)
		for _, value := range subTypes {
			subscribe := GetWSSubscribe(market, symbol, value)
			if subscribe == nil || subscribe == "" {
				continue
			}
			duplicated := false
			for _, sub := range subscribes {
				if market == model.Ftx { // subscribe类型为[]string
					itemSub := sub.([]string)
					itemSubscribe := subscribe.([]string)
					if itemSub[0] == itemSubscribe[0] && itemSub[1] == itemSubscribe[1] {
						duplicated = true
						break
					}
				} else { // subscribe类型为string
					if sub.(string) == subscribe.(string) {
						duplicated = true
						break
					}
				}
			}
			if !duplicated {
				subscribes = append(subscribes, subscribe)
			}
		}
	}
	if market == model.Bitmex {
		subscribes = append(subscribes, `position`)
		subscribes = append(subscribes, `order`)
	}
	switch market {
	case model.OKEX:
		go maintainChannelOKEX(subscribes)
	case model.BinanceSpot:
		go maintainChannelBinanceSpot(subscribes)
	case model.BinancePerp:
		go maintainChannelBinancePerp(subscribes)
	case model.Ftx:
		go maintainChannelFtx(subscribes)
	case model.BybitPerp:
		go maintainChannelBybitPerp(subscribes)
	case model.BybitSpot:
		go maintainChannelBybitSpot(subscribes)
	case model.Mexc:
		go maintainChannelMexc(subscribes)
	}
	return subscribes
}

func GetWSSubscribe(market, symbol, subType string) (subscribe interface{}) {
	_, _, _, dialectSymbol := model.GetFromStandard(market, symbol)
	switch market {
	case model.Mexc:
		switch subType {
		case mexcContractDepthIncSubType:
			return fmt.Sprintf(`{"method":"sub.depth","param":{"symbol":"%s","compress":true}}`, dialectSymbol)
		case mexcContractDepthFullSubType:
			return fmt.Sprintf(`{"method":"sub.depth.full","param":{"symbol":"%s","limit":5}}`, dialectSymbol)
		case mexcContractTickerSubType:
			return fmt.Sprintf(`{"method":"sub.ticker","param":{"symbol":"%s"}}`, dialectSymbol)
		}
	case model.OKEX:
		return dialectSymbol
	case model.BinancePerp:
		if subType == model.SubscribeDepth {
			return strings.ToLower(dialectSymbol) + `@depth5@100ms`
		}
		return strings.ToLower(dialectSymbol) + `@bookTicker`
	case model.BinanceSpot: // XRPUSDT: XRPUSDT@depth5   XRP-PERP: XRPUSDT@depth5
		if subType == model.SubscribeDepth {
			return strings.ToLower(dialectSymbol) + `@depth5@100ms`
		}
		return strings.ToLower(dialectSymbol) + `@bookTicker`
	//case model.Bitmex:
	//	if subType == model.SubscribeDeal {
	//		return `trade:` + model.GetDialectPerp(model.Bitmex, symbol)
	//	} else if subType == model.SubscribeDepth {
	//		//return `quote:` + DialectSymbol[Bitmex][symbol]
	//		//return `orderBookL2:` + DialectSymbol[Bitmex][symbol]
	//		//return `orderBookL2_25:` + DialectSymbol[Bitmex][symbol]
	//		return `orderBook10:` + model.GetDialectPerp(model.Bitmex, symbol)
	//	}
	//case model.Huobi: // xrpbtc: market.xrpbtc.mbp.refresh.
	//	if subType == model.SubscribeTicker {
	//		return "market." + symbol + ".bbo"
	//	}
	//case model.HuobiDM:
	//	return fmt.Sprintf(`market.%s.depth.step6`, symbol)
	//case model.Coinpark: //BTC_USDT bibox_sub_spot_BTC_USDT_ticker
	//	//return `bibox_sub_spot_` + strings.ToUpper(symbol) + `_ticker`
	//	return `bibox_sub_spot_` + strings.ToUpper(symbol) + `_depth`
	case model.BybitPerp:
		return dialectSymbol
	case model.BybitSpot:
		return dialectSymbol
	case model.Ftx:
		if subType == model.SubscribeDepth {
			return []string{`orderbook`, dialectSymbol}
		} else if subType == model.SubscribeTicker {
			return []string{`ticker`, dialectSymbol}
		}
	case model.DFuture:
		return `dfuture.market.` + dialectSymbol + `.kline.1min`
	}
	return ""
}

// Transfer
func _(key, secret, market, transferType string, amount float64) {
	if market == model.Gate {
		transferGate(key, secret, transferType, amount)
	} else if market == model.Kucoin {
		transferKucoin(transferType, amount)
	}
}

func GetMarketInfos(market string) (marketInfo map[string]*model.MarketInfo) {
	accounts := model.AppConfig.GetAccounts(market)
	switch market {
	case model.Ftx:
		return getMarketsFtx(accounts[0].Key, accounts[0].Secret)
	case model.OKEX:
		return getMarketsOKEX(accounts[0].Key, accounts[0].Secret)
	case model.Mexc:
		return getMarketsMexc(accounts[0].Key, accounts[0].Secret)
	case model.BinanceSpot:
		return getMarketsBinanceSpot(accounts[0].Key, accounts[0].Secret)
	case model.BinancePerp:
		return getMarketsBinancePerp(accounts[0].Key, accounts[0].Secret)
	case model.Gate:
		_, marketInfo = getMarketsGate(accounts[0].Key, accounts[0].Secret)
	case model.Kucoin:
		_, marketInfo = getMarketsKucoin(``)
	case model.BybitPerp:
		return getMarketsBybitPerp(accounts[0].Key, accounts[0].Secret)
	case model.BybitSpot:
		return getMarketsBybitSpot(accounts[0].Key, accounts[0].Secret)
	}
	return
}

// 搬砖过滤币种 `AMPL`, `IOTA` REEF LUNA
// 某些主流币 BTC ETH LINK
// 币种对不上 REAL, DFL, QI, WSB, TRADE,FAME,BIFI,TON,BOX,PAY
// 法币 GBP CUSDT `TRYB``BRZ``CAD``EUR` `SUSD` `USDC` `TUSD`USDT EURT
// 平台币 `GT` `FTT` `BNB` `OKB` MX
// ftx预测`TRUMP``BOLSONARO`
func filterCross(market, symbol string) bool {
	filterCoins := map[string]bool{`AMPL`: true, `IOTA`: true, `REEF`: true, `LUNA`: true, `BTC`: true, `ETH`: true, `LINK`: true,
		`REAL`: true, `DFL`: true, `QI`: true, `WSB`: true, `TRADE`: true, `FAME`: true, `BIFI`: true, `TON`: true,
		`BOX`: true, `PAY`: true, `GTC`: true,
		`GBP`: true, `CUSDT`: true, `TRYB`: true, `BRZ`: true, `CAD`: true, `EUR`: true, `SUSD`: true, `USDC`: true,
		`TUSD`: true, `USDT`: true, `EURT`: true,
		`GT`: true, `FTT`: true, `BNB`: true, `OKB`: true, `MX`: true, `TRUMP`: true, `BOLSONARO`: true, `DEFI`: true}
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	if filterCoins[coin] {
		return true
	}
	// ftx波动率产品
	filterWord := []string{`IBVOL`, `BVOL`, `MOVE`, `BEAR`, `BULL`, `HEDGE`, `HALF`, `EDFIBULL`, `DEFIHEDGE`, `DEIFHALF`, `DEFIBEAR`}
	for _, word := range filterWord {
		if strings.Contains(symbol, word) {
			return true
		}
	}
	switch market {
	case model.Ftx:
		switch coin {
		case `PRIV`, `ALT`, `SHIT`, `MID`, `EXCH`, `DRGN`, `FTT`:
			return true
		}
	case model.Gate:
		switch coin {
		// BTT价格异常
		case `BTT`, `GT`:
			return true
		}
	case model.OKEX:
		switch coin {
		case `OKB`:
			return true
		}
	case model.BinancePerp, model.BinanceSpot:
		switch coin {
		case `BNB`:
			return true
		}
	case model.BybitSpot:
		switch coin {
		case `GAS`:
			return true
		}
	case model.Mexc: //不支持主流币种期货下单
		switch coin {
		case `BTC`, `ETH`, `LTC`:
			return true
		}
	}
	return false
}

// InitCrossMarketInfos 用以初始化cross carry的各个币种市场，调用前需要truncate settings数据库表，本方法会从新插入
func InitCrossMarketInfos(markets []string) {
	infoPool := make(map[string][]*model.MarketInfo) // coin - []marketInfos
	// model.Binance, model.Ftx,
	//markets := []string{model.BybitPerp, model.BybitSpot, model.OKEX, model.Ftx, model.Gate}
	//markets := model.GetMarkets()
	for _, market := range markets {
		marketInfo := GetMarketInfos(market)
		for _, info := range marketInfo {
			success, _, coin, _ := model.GetFromStandard(market, info.Name)
			if success && coin != `` {
				if infoPool[coin] == nil {
					infoPool[coin] = make([]*model.MarketInfo, 0)
				}
				if !filterCross(info.Market, info.Name) {
					infoPool[coin] = append(infoPool[coin], info)
				}
			}
		}
	}
	for coin, infos := range infoPool {
		if len(infos) >= 2 {
			for _, info := range infos {
				setting := &model.Setting{Valid: true, Function: model.FunctionCross, Market: info.Market,
					Symbol: info.Name, Coin: coin, OpenShortMargin: 0.025, CloseShortMargin: 0.025}
				model.AppDB.Save(setting)
				util.Notice(fmt.Sprintf(`save setting %s %s %s %v`, info.Market, info.Name, coin, setting.Valid))
			}
		}
	}
}

// InitMarketInfos 只支持现货SPOT和永续PERP SWAP
func InitMarketInfos() (success bool) {
	success = true
	markets := model.GetMarkets()
	for _, market := range markets {
		accounts := model.AppConfig.GetAccounts(market)
		switch market {
		case model.Mexc:
			marketInfos := getMarketsMexc(accounts[0].Key, accounts[0].Secret)
			model.SetMarketInfos(market, marketInfos)
		case model.Ftx:
			model.SetMarketInfos(market, getMarketsFtx(accounts[0].Key, accounts[0].Secret))
		case model.OKEX:
			model.SetMarketInfos(market, getMarketsOKEX(accounts[0].Key, accounts[0].Secret))
			for _, account := range accounts {
				accountMode := getAccountConfigOKEX(account.Key, account.Secret)
				util.Notice(`okex config and set: ` + accountMode)
				if accountMode != `net_mode` {
					if !setAccountModeOKEX(account.Key, account.Secret) {
						success = false
					}
				}
			}
		//case model.Binance:
		//	model.SetMarketInfos(market, getMarketsBinance(accounts[0].Key, accounts[0].Secret))
		//	for _, account := range accounts {
		//		setPosSideBinance(account.Key, account.Secret)
		//	}
		case model.BinanceSpot:
			model.SetMarketInfos(market, getMarketsBinanceSpot(accounts[0].Key, accounts[0].Secret))
		case model.BinancePerp:
			model.SetMarketInfos(market, getMarketsBinancePerp(accounts[0].Key, accounts[0].Secret))
			for _, account := range accounts {
				setPosSideBinancePerp(account.Key, account.Secret)
			}
		case model.Gate:
			for _, account := range accounts {
				setPosSideGate(account.Key, account.Secret)
				setMarginSettingGate(account.Key, account.Secret)
			}
			var marketInfos map[string]*model.MarketInfo
			success, marketInfos = getMarketsGate(accounts[0].Key, accounts[0].Secret)
			if success {
				model.SetMarketInfos(market, marketInfos)
			}
		case model.Kucoin:
			_, marketInfos := getMarketsKucoin("")
			model.SetMarketInfos(market, marketInfos)
			setFutureAutoDeposit()
		case model.BybitPerp:
			marketInfos := getMarketsBybitPerp(accounts[0].Key, accounts[0].Secret)
			model.SetMarketInfos(market, marketInfos)
			go func() {
				for _, account := range accounts {
					for symbol := range marketInfos {
						setSettingsBybitPerp(account.Key, account.Secret, symbol)
						time.Sleep(time.Minute)
					}
				}
			}()
		case model.BybitSpot:
			model.SetMarketInfos(market, getMarketsBybitSpot(accounts[0].Key, accounts[0].Secret))
		}
	}
	return success
}

func CreateMarketDepthServer(markets *model.Markets, market string, orderHandler OrderHandler) (
	channels []chan struct{}) {
	util.Notice(" create depth chan for " + market)
	channels = make([]chan struct{}, 1)
	var err error
	switch market {
	case model.Kucoin:
		channels, err = WsDepthServeKucoin()
	case model.Gate:
		err = WsDepthServeGate()
	case model.OKEX:
		channels, err = WsDepthServeOKEX(model.GetMarketSymbols(model.OKEX), orderHandler)
	//case model.Binance:
	//	channels, err = WsDepthServeBinance(markets, nil)
	case model.BinanceSpot:
		channels, err = WsDepthServeBinanceSpot(markets, nil)
	case model.BinancePerp:
		channels, err = WsDepthServeBinancePerp(markets, nil)
	case model.BybitPerp:
		channels, err = WsDepthServeBybitPerp(markets, orderHandler)
	case model.BybitSpot:
		channels, err = WsDepthServeBybitSpot(markets, orderHandler)
	case model.Ftx:
		channels, err = WsDepthServeFtx(markets, nil)
	case model.Mexc:
		channels, err = WsDepthServeMexc(markets, nil, true)
	}
	if err != nil {
		util.Notice(market + ` can not create depth server ` + err.Error())
	}
	return channels
}

func SendMails(title, msg string) {
	toMails := strings.Split(model.AppConfig.Mail, `,`)
	for _, mail := range toMails {
		if len(strings.TrimSpace(mail)) == 0 {
			continue
		}
		err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, mail, title, msg)
		if err != nil {
			util.Notice(`fail to send mail title %s msg %s to %s err %s`, title, msg, mail, err.Error())
		}
	}
}

//func InitCoinBalance(key, secret, function, market string) {
//	InitMarketInfos()
//	settings := model.GetSettings(function, market)
//	_, balances, _ := GetBalances(key, secret, market, 0)
//	balanceMap := make(map[string]*model.Balance)
//	for _, balance := range balances {
//		balanceMap[balance.Coin] = balance
//	}
//	i := 0
//	for _, items := range settings {
//		coin := model.GetCoin(items[0].Market, items[0].Symbol)
//		balance := balanceMap[coin]
//		if balance == nil {
//			if model.MarketInfos[market] == nil {
//				continue
//			}
//			related := items[0].GetRelatedSymbol()
//			marketInfo := model.MarketInfos[market][related]
//			if marketInfo == nil {
//				continue
//			}
//			price := GetLastPrice(key, secret, market, related)
//			order := PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeMarket, market, related, related,
//				``, ``, ``, price, price, marketInfo.SizeMin, false, nil)
//			if order.OrderId == `` {
//				i++
//				fmt.Println(fmt.Sprintf(`%d order return :%s %s`, i, order.ErrCode, related))
//			} else {
//				fmt.Println(fmt.Sprintf(`%s success order`, related))
//			}
//		}
//	}
//}

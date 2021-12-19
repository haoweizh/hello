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

var channelLock sync.Mutex
var instruments = make(map[string]map[string]map[string]string) // market - symbol - (quarter;bi_quarter) - instrument
var requireReset = make(map[string]bool)
var instrumentLock sync.Mutex

func SetRequireReset(market string, reset bool) {
	channelLock.Lock()
	defer channelLock.Unlock()
	requireReset[market] = reset
}

func RequireDepthChanReset(markets *model.Markets, market string) bool {
	channelLock.Lock()
	defer channelLock.Unlock()
	if requireReset[market] {
		requireReset[market] = false
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
	util.Notice(`need reset %d %s %d`, now, market, model.AppConfig.Delay)
	return true
}

func MustCancel(key, secret, market, symbol, instrument, orderType, orderId string, mustCancel bool) (
	res bool, order *model.Order) {
	sleepTime := 1
	for i := 0; i < 20; i++ {
		result, errCode, _, cancelOrder := CancelOrder(key, secret, market, symbol, instrument, orderType, orderId)
		res = result
		order = cancelOrder
		util.Notice(fmt.Sprintf(`[cancel] %s %s %s %s %s for %d times, return %t `,
			market, symbol, instrument, orderType, orderId, i, result))
		if result || !mustCancel || errCode == `0` {
			return result, cancelOrder
		}
		if errCode == `3008` && i >= 3 {
			return result, cancelOrder
		}
		//if result || !mustCancel { //3008:"submit cancel invalid order state
		//	break
		//} else if errCode == `429` || errCode == `4003` {
		//	util.Notice(`调用次数繁忙`)
		//}
		time.Sleep(time.Second * time.Duration(sleepTime))
		sleepTime *= 2
	}
	return res, order
}

func CancelOrders(key, secret, market, symbol string) (result bool) {
	switch market {
	case model.Kucoin:
		return cancelOrdersKucoin(symbol)
	case model.Gate:
		return cancelOrdersGate(key, secret, symbol)
	case model.Huobi:
		return cancelOrdersHuobi(key, secret, symbol)
	case model.Binance:
		return cancelOrdersBinance(key, secret, symbol)
	case model.Ftx:
		return cancelOrdersFtx(key, secret, symbol)
	case model.OKEX:
		result, _, _ = cancelOrdersOKEX(key, secret, symbol)
		return result
	}
	return false
}

func CancelOrder(key, secret, market, symbol, instrument, orderType, orderId string) (
	result bool, errCode, msg string, order *model.Order) {
	if instrument == `` {
		instrument = symbol
	}
	if model.AppConfig.Env == `test` {
		return true, ``, `test cancel`,
			&model.Order{Market: market, Symbol: symbol, OrderId: orderId, Status: model.CarryStatusFail}
	}
	errCode = `market-not-supported ` + market
	msg = `market not supported ` + market
	switch market {
	case model.Huobi:
		result, errCode, msg = cancelOrderHuobi(key, secret, orderId)
	//case model.HuobiDM:
	//	result, errCode, msg = cancelOrderHuobiDM(key, secret, symbol, orderId)
	case model.OKEX:
		result, errCode, msg = cancelOrderOkex(key, secret, symbol, orderId, orderType)
	case model.Binance:
		//result, errCode, msg = cancelOrderBinance(key, secret, symbol, orderId)
	case model.Coinpark:
		result, errCode, msg = cancelOrderCoinpark(key, secret, orderId)
	case model.Bitmex:
		result, errCode, msg = cancelOrderBitmex(key, secret, orderId)
	case model.Bybit:
		result, errCode, msg, order = cancelOrderBybit(key, secret, symbol, orderId)
	case model.Ftx:
		result = cancelOrderFtx(key, secret, orderType, orderId)
	case model.Gate:
		result = cancelOrderGate(key, secret, symbol, orderId)
	}
	util.Notice(fmt.Sprintf(`[cancel %s %v %s %s]`, orderId, result, market, symbol))
	return result, errCode, msg, order
}

func GetCurrentInstrument(market, symbol string) (currentInstrument string) {
	querySetter := querySetInstrumentsHuobiDM
	currentType := `quarter`
	//nextType := `bi_quarter`
	switch market {
	//case model.OKFUTURE:
	//	querySetter = querySetInstrumentsOkFuture
	//	//nextType = `bi_quarter`
	case model.HuobiDM:
		querySetter = querySetInstrumentsHuobiDM
		//nextType = `next_quarter`
		symbol = symbol[0:strings.Index(symbol, `_`)]
	case model.OKEX:
		return symbol
	default:
		return symbol
	}
	querySetter()
	if instruments == nil || instruments[market] == nil || instruments[market][symbol] == nil {
		return ``
	}
	return instruments[market][symbol][currentType]
}

func setInstrument(market, symbol, alias, instrument string) {
	instrumentLock.Lock()
	defer instrumentLock.Unlock()
	if instruments[market] == nil {
		instruments[market] = make(map[string]map[string]string)
	}
	if instruments[market][symbol] == nil {
		instruments[market][symbol] = make(map[string]string)
	}
	instruments[market][symbol][alias] = instrument
}

func GetDayCandle(key, secret, market, symbol, instrument string, timeCandle time.Time) (candle *model.Candle) {
	if symbol == `` {
		symbol = instrument
	}
	if instrument == `` {
		instrument = symbol
	}
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
	case model.Bitmex:
		candles = getCandlesBitmex(key, secret, symbol, `1d`, begin, end, 20)
	case model.Ftx:
		candles = getCandlesFtx(key, secret, symbol, `1d`, begin, end, 40)
	case model.OKEX:
		candles = getCandlesOKEX(key, secret, instrument, `1D`, begin, end, 40)
	case model.HuobiDM:
		candles = getCandlesHuobiDM(key, secret, symbol, `1d`, begin, time.Now())
	}
	keyedCandles := make(map[string]*model.Candle)
	for _, value := range candles {
		keyedCandles[market+symbol+value.Period+value.UTCDate] = value
	}
	candle = keyedCandles[market+symbol+`1d`+timeCandle.Format(time.RFC3339)[0:10]]
	if candle == nil {
		util.Notice(fmt.Sprintf(`error: can not get candle %s %s %s %s`,
			market, symbol, `1d`, timeCandle.String()))
		return
	}
	candle.N = (candle.PriceHigh - candle.PriceLow) / 20
	for i := 1; i < 20; i++ {
		d, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		index := timeCandle.Add(d)
		candleCurrent := keyedCandles[market+symbol+`1d`+index.Format(time.RFC3339)[0:10]]
		if candleCurrent == nil {
			util.Notice(fmt.Sprintf(`error: can not get candle %s %s`, `1d`, index.String()))
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

func GetBalance(key, secret, market, coin string) (balance *model.Balance) {
	success, balances, _, _ := GetBalances(key, secret, market)
	if !success {
		return
	}
	for _, item := range balances {
		if item.Coin == coin {
			return item
		}
	}
	return
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
	case model.HuobiDM:
		success, balances = getBalanceHuobiDM(key, secret)
	case model.Huobi:
		success, balances = getBalanceHuobi(key, secret)
	case model.Binance:
		success, balances = getBalanceBinance(key, secret)
	}
	if market != model.Ftx && market != model.OKEX {
		for _, balance := range balances {
			symbol := balance.Coin + model.GetSpotTail(market)
			getTick, tick := model.AppMarkets.GetBidAsk(symbol, market)
			if getTick {
				totalInUsd += tick.Bids[0].Price * balance.Amount
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
	case model.Huobi, model.HuobiDM:
		return getTransferHuobi(key, secret)
	}
	return balances
}

func GetFundingRate(key, secret, market, symbol string, lock *sync.Mutex) (success bool, rate float64) {
	if lock != nil {
		defer lock.Unlock()
		lock.Lock()
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
			return true, fundingRate.Rate
		}
	}
	if fundingRate != nil {
		util.Notice(fmt.Sprintf(`before update funding %s %s rate %f expire %d neext: %f update: %d`,
			market, symbol, fundingRate.Rate, fundingRate.ExpireTime, fundingRate.RateNext, fundingRate.UpdateTime))
	}
	var expireTime int64
	switch market {
	case model.Bitmex:
		rate, expireTime = getFundingRateBitmex(key, secret, symbol)
		model.SetFundingRate(market, symbol, &model.FundingRate{Rate: rate, ExpireTime: expireTime, UpdateTime: now})
	case model.Bybit:
		rate, expireTime = getFundingRateBybit(key, secret, symbol)
		model.SetFundingRate(market, symbol, &model.FundingRate{Rate: rate, ExpireTime: expireTime, UpdateTime: now})
	case model.Ftx:
		return true, 0
		//rates := getFundingRatesFtx()
		//symbolRates := make(map[string][]*model.FundingRate)
		//for _, rate := range rates {
		//	if symbolRates[rate.Symbol] == nil {
		//		symbolRates[rate.Symbol] = make([]*model.FundingRate, 0)
		//	}
		//	symbolRates[rate.Symbol] = append(symbolRates[rate.Symbol], rate)
		//}
		//duration, _ := time.ParseDuration(`3600s`)
		//nextHour := now.Add(duration)
		//nextHour = time.Date(nextHour.Year(), nextHour.Month(), nextHour.Day(),
		//	nextHour.Hour(), 0, 0, 0, now.Location())
		//for symbol, value := range symbolRates {
		//	model.SetFundingRate(market, symbol, value, nextHour.Unix())
		//}
		//fundingRate, expireTime = model.GetFundingRate(market, symbol)
	case model.OKEX:
		fundingRate = getFundingRateOKEX(key, secret, symbol)
		model.SetFundingRate(market, symbol, fundingRate)
	case model.Binance:
		fundingRate = getFundingRateBinance(key, secret, symbol)
		model.SetFundingRate(market, symbol, fundingRate)
	case model.Huobi:
		return true, 0
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

func GetMaxLoan(key, secret, market, coin string) (success bool, maxLoan float64) {
	switch market {
	case model.Gate:
		return getMaxLoanGate(coin)
	case model.OKEX:
		return getMaxLoanOKEX(key, secret, coin)
	case model.Binance:
		return true, 0
		//return getMaxLoanBinance(key, secret, coin)
	}
	return false, 0
}

func QueryOrderById(key, secret, market, symbol, instrument, orderType, orderId string) (order *model.Order) {
	if instrument == `` {
		instrument = symbol
	}
	order = &model.Order{
		OrderId: orderId, Symbol: symbol, Market: market, Instrument: instrument, OrderType: orderType, Status: model.CarryStatusFail}
	switch market {
	case model.Gate:
		queryOrderGate(key, secret, order)
	case model.Huobi:
		order.DealAmount, order.DealPrice, order.Status = queryOrderHuobi(key, secret, orderId)
	case model.HuobiDM:
		if orderType == model.OrderTypeStop {
			isWorking := queryOpenTriggerOrderHuobiDM(key, secret, symbol, orderId)
			if isWorking {
				order.Status = model.CarryStatusWorking
			} else {
				relatedOrderId := queryHisTriggerOrderHuobiDM(key, secret, symbol, orderId)
				if relatedOrderId == `-1` || relatedOrderId == `` {
					order.Status = model.CarryStatusFail
				} else {
					order.DealAmount, order.DealPrice, order.Status = queryOrderHuobiDM(key, secret, symbol, relatedOrderId)
				}
			}
		} else {
			order.DealAmount, order.DealPrice, order.Status = queryOrderHuobiDM(key, secret, symbol, orderId)
		}
	case model.OKEX:
		order = queryOrderOKEX(key, secret, instrument, orderId, orderType)
		if order != nil {
			order.Market = market
			order.Symbol = symbol
		}
		return order
	//case model.OKFUTURE:
	//	dealAmount, dealPrice, status = queryOrderOkfuture(key, secret, instrument, orderType, orderId)
	case model.Binance:
		//dealAmount, dealPrice, status = queryOrderBinance(key, secret, symbol, orderId)
	case model.Coinpark:
		order.DealAmount, order.DealPrice, order.Status = queryOrderCoinpark(key, secret, orderId)
	case model.Bybit:
		orders := queryOrderBybit(key, secret, symbol, orderId)
		for _, value := range orders {
			if value.OrderId == orderId {
				return value
			}
		}
	case model.Ftx: // 查询是否是待成交状态，如果已成交或已取消，则ftx返回order not found信息，order为nil
		if orderType == model.OrderTypeStop {
			newOrderId := queryTriggerOrderId(key, secret, orderId)
			if newOrderId != `` {
				return queryOrderFtx(key, secret, newOrderId)
			} else {
				order.Status = queryOpenTriggerOrders(key, secret, symbol, orderId)
			}
		} else {
			return queryOrderFtx(key, secret, orderId)
		}
	}
	return order
}

func GetPosition(market, symbol, address string) (success bool, position *model.Position) {
	switch market {
	case model.DFuture:
		if symbol[len(symbol)-4:] == `usdt` {
			symbol = symbol[0 : len(symbol)-4]
		}
		for true {
			success, position = getPositionsDFuture(symbol, address)
			if success {
				return
			}
			time.Sleep(time.Second * 5)
		}
	}
	return false, nil
}

func GetPositions(key, secret, market string) (success bool, positions []*model.Position, posBalance float64) {
	switch market {
	case model.Kucoin:
		return getPositionsKucoin(key, secret)
	case model.Gate:
		return getPositionsGate(key, secret)
	case model.Huobi:
		return getPositionsHuobi(key, secret)
	case model.Binance:
		return getPositionsBinance(key, secret)
	case model.Ftx:
		_, _, totalInU := getBalanceFtx(key, secret)
		success, positions, _ = getPositionsFtx(key, secret)
		return success, positions, totalInU
	case model.OKEX:
		_, _, _, collateral := getBalanceOKEX(key, secret)
		success, positions, _ = getPositionsOKEX(key, secret)
		return success, positions, collateral.Available
	case model.DFuture:
		symbols := model.GetMarketSymbols(market)
		positions = make([]*model.Position, 0)
		for symbol := range symbols {
			temp, pos := getPositionsDFuture(symbol, model.AppConfig.FutureAddress)
			if temp {
				positions = append(positions, pos)
			} else {
				success = temp
			}
		}
		return success, positions, 0
	}
	return false, nil, 0
}

func MustPlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument, orderParam,
	refreshType string, price, triggerPrice, amount float64, saveDB, isWs bool, setting *model.Setting) (order *model.Order) {
	retry := 10
	for i := 0; i < retry; i++ {
		order = PlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument,
			orderParam, refreshType, price, triggerPrice, amount, saveDB, isWs, nil, setting)
		if order != nil && order.OrderId != `` {
			break
		} else {
			//if market == model.OKSwap && order != nil && order.ErrCode == `35010` {
			//	amountType = model.AmountTypeNew
			//	RefreshAccount(key, secret, model.OKSwap)
			//}
			time.Sleep(time.Second)
			util.Notice(fmt.Sprintf(`fail to place order %d time, re order`, i))
		}
	}
	return order
}

// PlaceOrder orderSide: OrderSideBuy OrderSideSell OrderSideLiquidateLong OrderSideLiquidateShort
// orderType: OrderTypeLimit OrderTypeMarket
// amount:如果是限价单或市价卖单，amount是左侧币种的数量，如果是市价买单，amount是右测币种的数量
func PlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument, orderParam, refreshType string,
	price, triggerPrice, amount float64, saveDB, isWs bool, postOrder model.PostOrder, setting *model.Setting) (order *model.Order) {
	if instrument == `` {
		instrument = symbol
	}
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
			Price: price, Amount: 0, OrderId: ``, ErrCode: ``, RefreshType: refreshType, TriggerPrice: triggerPrice,
			Status: model.CarryStatusFail, DealAmount: 0, DealPrice: price, OrderTime: util.GetNow()}
	}
	order = &model.Order{OrderSide: markSide, OrderType: orderType, Market: market, Symbol: symbol, Price: price,
		Amount: amount, DealAmount: 0, DealPrice: price, RefreshType: refreshType, TriggerPrice: triggerPrice,
		OrderTime: util.GetNow(), UnfilledQuantity: amount, Instrument: instrument, AmountType: key}
	util.Notice(fmt.Sprintf(`...%s %s %s before order %d amount: %f price:%f triggerPrice:%f`,
		orderSide, market, symbol, start, amount, price, triggerPrice))
	if model.AppConfig.Env == `test` {
		order.Status = model.CarryStatusWorking
		order.OrderId = fmt.Sprintf(`%s%s%d`, market, symbol, util.GetNow().UnixNano())
		order.DealPrice = price
		order.DealAmount = amount
		if saveDB {
			go model.AppDB.Save(order)
		}
		return
	}
	switch market {
	case model.Kucoin:
		placeOrderKucoin(order, orderSide, orderType, symbol, price, amount)
	case model.Gate:
		placeOrderGate(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.DFuture:
		if symbol[len(symbol)-4:] == `usdt` {
			symbol = symbol[0 : len(symbol)-4]
		}
		order.OrderId = refreshType + time.Now().String()
		if orderParam == `close` {
			closeDFuture(key, secret, symbol, triggerPrice, price, amount)
		} else {
			openDFuture(key, secret, orderSide, symbol, triggerPrice, price, amount)
		}
	case model.Huobi:
		placeOrderHuobi(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.HuobiDM:
		placeOrderHuobiDM(key, secret, order, orderSide, orderType, instrument, symbol, price, triggerPrice, amount)
	case model.OKEX:
		placeOrderOKEX(key, secret, isWs, order)
	case model.Binance:
		placeOrderBinance(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.Coinpark:
		placeOrderCoinpark(key, secret, order, orderSide, orderType, symbol, price, amount)
		if order.ErrCode == `4003` {
			util.Notice(`【发现4003错误】sleep 3 minutes`)
			time.Sleep(time.Minute * 3)
		}
	case model.Bitmex:
		placeOrderBitmex(order, key, secret, orderSide, orderType, orderParam, symbol, price, amount)
	case model.Bybit:
		placeOrderBybit(order, key, secret, orderSide, orderType, orderParam, symbol, price, amount)
	case model.Ftx:
		placeOrderFtx(order, key, secret, orderSide, orderType, orderParam, symbol, price, triggerPrice, amount)
	}
	if order.OrderId == "0" || strings.Trim(order.OrderId, ` `) == "" {
		order.Status = model.CarryStatusFail
	} else if order.Status == `` {
		order.Status = model.CarryStatusWorking
	}
	end := util.GetNowUnixMillion()
	util.Notice(fmt.Sprintf(`...%s %s %s return order at %d distance %d %s %s price %f id %s`,
		orderSide, market, symbol, end, end-start, order.Status, order.ErrCode, order.Price, order.OrderId))
	order.RefreshType = refreshType
	if saveDB {
		if isWs && market == model.OKEX {
			order.Status = model.CarryStatusSuccess
			order.OrderId = strconv.FormatInt(time.Now().UnixNano(), 10)
		}
		if order.OrderId == `` {
			order.OrderId = fmt.Sprintf(`%s_error_%d`, order.ErrCode, time.Now().UnixNano())
		}
		util.Notice(`save order %s %s %s %s %s %f`,
			order.RefreshType, order.Market, order.Symbol, order.OrderId, order.OrderSide, order.Amount)
		go model.AppDB.Save(order)
	}
	if postOrder != nil {
		go postOrder(order, setting)
	}
	return
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
	if market == model.Bitmex || market == model.Bybit {
		subscribes = append(subscribes, `position`)
	}
	if market == model.Bitmex {
		subscribes = append(subscribes, `order`)
	}
	switch market {
	case model.OKEX:
		go maintainChannelOKEX()
	case model.Binance:
		go maintainChannelBinance()
	case model.Ftx:
		go maintainChannelFtx(subscribes)
	}
	return subscribes
}

func GetWSSubscribe(market, symbol, subType string) (subscribe interface{}) {
	switch market {
	case model.Huobi: // xrpbtc: market.xrpbtc.mbp.refresh.
		if subType == model.SubscribeTicker {
			return "market." + symbol + ".bbo"
		}
	case model.HuobiDM:
		return fmt.Sprintf(`market.%s.depth.step6`, symbol)
	case model.OKEX: // 未来基于market兼容okfuture LTC-USD-190628
		return GetCurrentInstrument(market, symbol)
	case model.Binance: // XRPUSDT: XRPUSDT@depth5   XRP-PERP: XRPUSDT@depth5
		if symbol[len(symbol)-5:] == `-PERP` {
			symbol = symbol[0:len(symbol)-5] + `USDT`
		}
		if subType == model.SubscribeDepth {
			return strings.ToLower(symbol) + `@depth5@100ms`
		}
		return strings.ToLower(symbol) + `@bookTicker`
	case model.Coinpark: //BTC_USDT bibox_sub_spot_BTC_USDT_ticker
		//return `bibox_sub_spot_` + strings.ToUpper(symbol) + `_ticker`
		return `bibox_sub_spot_` + strings.ToUpper(symbol) + `_depth`
	case model.Bitmex:
		if subType == model.SubscribeDeal {
			return `trade:` + model.GetDialectSymbol(model.Bitmex, symbol)
		} else if subType == model.SubscribeDepth {
			//return `quote:` + DialectSymbol[Bitmex][symbol]
			//return `orderBookL2:` + DialectSymbol[Bitmex][symbol]
			//return `orderBookL2_25:` + DialectSymbol[Bitmex][symbol]
			return `orderBook10:` + model.GetDialectSymbol(model.Bitmex, symbol)
		}
		return ``
	case model.Bybit:
		subSymbol := strings.ToUpper(symbol[0:strings.Index(symbol, `_`)])
		if subType == model.SubscribeDeal {
			return `trade.` + subSymbol
		} else if subType ==
			model.SubscribeDepth {
			//return `orderBook_200.100ms.` + subSymbol
			return `orderBookL2_25.` + subSymbol
		}
	case model.Ftx:
		if subType == model.SubscribeDepth {
			return []string{`orderbook`, symbol}
		} else if subType == model.SubscribeTicker {
			return []string{`ticker`, symbol}
		}
	case model.DFuture:
		return `dfuture.market.` + symbol + `.kline.1min`
	}
	return ""
}

func InitCarryFtx(key, secret string, start uint) {
	rates := getFundingRatesFtx(key, secret)
	symbolRates := make(map[string]bool)
	for _, rate := range rates {
		if symbolRates[rate.Symbol] == false {
			symbolRates[rate.Symbol] = true
		}
	}
	marketInfos := getMarketsFtx(key, secret)
	_ = model.AppDB.AutoMigrate(&model.Setting{})
	for symbol := range symbolRates {
		setting := &model.Setting{
			Valid:            true,
			Function:         model.FunctionCarry,
			OpenShortMargin:  0.013,
			CloseShortMargin: -0.013,
			Market:           model.Ftx,
			Symbol:           symbol,
			AmountLimit:      0,
			ID:               start,
		}
		// HOLY 1INCH too easy to be single completed order
		if marketInfos[setting.SymbolRelated] == nil || symbol == `FTT-PERP` || symbol == `USDT-PERP` || symbol == `BTC-PERP` ||
			symbol == `ETH-PERP` || symbol == `LINK-PERP` {
			continue
		}
		//if !marketInfos[related].CanBorrow {
		//	setting.CloseShortMargin = -1
		//	fmt.Println(related + `do not have borrow`)
		//}
		start++
		model.AppDB.Save(setting)
		fmt.Println(fmt.Sprintf(`%s %s saved %d %f`, symbol, setting.SymbolRelated, start, setting.CloseShortMargin))
	}
}

func Transfer(key, secret, market, transferType string, amount float64) {
	if market == model.Binance {
		transferBinance(key, secret, transferType, amount)
	} else if market == model.Huobi {
		transferHuobi(key, secret, transferType, amount)
	} else if market == model.Gate {
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
	case model.Binance:
		return getMarketsBinance(accounts[0].Key, accounts[0].Secret)
	case model.Gate:
		_, marketInfo = getMarketsGate(accounts[0].Key, accounts[0].Secret)
	case model.Kucoin:
		_, marketInfo = getMarketsKucoin(``)
	case model.Huobi:
		return getMarketsHuobi(accounts[0].Key, accounts[0].Secret)
	}
	return
}

func filterCross(market, symbol string) bool {
	switch market {
	case model.Ftx:
		switch symbol {
		case `DEFI-PERP`, `PRIV-PERP`, `ALT-PERP`, `SHIT-PERP`, `MID-PERP`, `EXCH-PERP`, `DRGN-PERP`:
			return true
		}
	}
	return false
}

// InitCrossMarketInfos 用以初始化cross carry的各个币种市场，调用前需要truncate settings数据库表，本方法会从新插入
func InitCrossMarketInfos() {
	infoPool := make(map[string][]*model.MarketInfo) // coin - []marketInfos
	markets := []string{model.Binance, model.Ftx, model.Gate, model.OKEX}
	for _, market := range markets {
		marketInfo := GetMarketInfos(market)
		for _, info := range marketInfo {
			coin := model.GetCrossCoin(market, info.Name)
			if coin != `` {
				if infoPool[coin] == nil {
					infoPool[coin] = make([]*model.MarketInfo, 0)
				}
				infoPool[coin] = append(infoPool[coin], info)
			}
		}
	}
	for coin, infos := range infoPool {
		if len(infos) > 1 {
			for _, info := range infos {
				setting := &model.Setting{Valid: true, Function: model.FunctionCross, Market: info.Market, Symbol: info.Name, Coin: coin}
				if filterCross(setting.Market, setting.Symbol) {
					continue
				}
				model.AppDB.Save(setting)
				fmt.Println(fmt.Sprintf(`save setting %s %s %s %v`, info.Market, info.Name, coin, setting.Valid))
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
		case model.Ftx:
			model.SetMarketInfos(model.Ftx, getMarketsFtx(accounts[0].Key, accounts[0].Secret))
		case model.OKEX:
			model.SetMarketInfos(model.OKEX, getMarketsOKEX(accounts[0].Key, accounts[0].Secret))
			for _, account := range accounts {
				accountMode := getAccountConfigOKEX(account.Key, account.Secret)
				util.Notice(`okex config and set: ` + accountMode)
				if accountMode != `net_mode` {
					if !setAccountModeOKEX(account.Key, account.Secret) {
						success = false
					}
				}
			}
		case model.Binance:
			model.SetMarketInfos(model.Binance, getMarketsBinance(accounts[0].Key, accounts[0].Secret))
			for _, account := range accounts {
				setPosSideBinance(account.Key, account.Secret)
			}
		case model.Huobi:
			model.SetMarketInfos(model.Huobi, getMarketsHuobi(accounts[0].Key, accounts[0].Secret))
		case model.Gate:
			for _, account := range accounts {
				setPosSideGate(account.Key, account.Secret)
				setMarginSettingGate(account.Key, account.Secret)
			}
			var marketInfos map[string]*model.MarketInfo
			success, marketInfos = getMarketsGate(accounts[0].Key, accounts[0].Secret)
			if success {
				model.SetMarketInfos(model.Gate, marketInfos)
			}
		case model.Kucoin:
			_, marketInfos := getMarketsKucoin("")
			model.SetMarketInfos(model.Kucoin, marketInfos)
			setFutureAutoDeposit()
		}
	}
	return success
}

func SetBidAsk(key, secret, market, symbol string) {
	switch market {
	case model.Gate:
		tailPerp := model.GetPerpTail(model.Gate)
		if symbol[len(symbol)-len(tailPerp):] == tailPerp {
			setBidAskGate(key, secret, symbol)
		}
	}
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
	case model.Huobi:
		channels, err = WsDepthServeHuobi(markets, nil)
		//channels[0] = channel
	case model.HuobiDM:
		channels, err = WsDepthServeHuobiDM(markets, nil)
	case model.OKEX:
		channels, err = WsDepthServeOKEX(model.GetMarketSymbols(model.OKEX), orderHandler)
	case model.Binance:
		channels, err = WsDepthServeBinance(markets, nil)
	case model.Coinpark:
		channels, err = WsDepthServeCoinpark(markets, nil)
	case model.Bitmex:
		channels, err = WsDepthServeBitmex(markets, nil)
	case model.Bybit:
		channels, err = WsDepthServeBybit(markets, nil)
	case model.Ftx:
		channels, err = WsDepthServeFtx(markets, nil)
	case model.DFuture:
		channels, err = WsDepthServeDFuture(markets, nil)
	}
	if err != nil {
		util.Notice(market + ` can not create depth server ` + err.Error())
	}
	return channels
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

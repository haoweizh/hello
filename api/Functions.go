package api

import (
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"strings"
	"sync"
	"time"
)

var channelLock sync.Mutex
var instrumentLock sync.Mutex
var instruments = make(map[string]map[string]map[string]string) // market - symbol - (quarter;bi_quarter) - instrument

func RequireDepthChanReset(markets *model.Markets, market string) bool {
	channelLock.Lock()
	defer channelLock.Unlock()
	needReset := true
	now := util.GetNowUnixMillion()
	symbols := markets.GetSymbols()
	for symbol := range symbols {
		_, bidAsk := markets.GetBidAsk(symbol, market)
		if bidAsk == nil {
			continue
		}
		delay := float64(now - int64(bidAsk.Ts))
		if delay < model.AppConfig.Delay {
			needReset = false
		}
	}
	return needReset
}

func GetPriceDistance(market, symbol string) float64 {
	switch symbol {
	case `btcusd_p`:
		switch market {
		case model.Bitmex, model.Bybit:
			return 0.5
		case model.OKSwap:
			return 0.1
		}
	case `ethusd_p`:
		switch market {
		case model.Bitmex, model.Bybit:
			return 0.05
		case model.OKSwap:
			return 0.01
		}
	}
	return 0
}

// 根据不同的网站返回价格小数位
func GetPriceDecimal(market, symbol string) float64 {
	switch market {
	case model.Coinpark:
		switch symbol {
		case `cp_usdt`:
			return 4
		case `cp_eth`, `cp_btc`:
			return 8
		}
	case model.Bitmex:
		switch symbol {
		case `btcusd_p`:
			return 0.5
		case `ethusd_p`:
			return 1.5
		}
	case model.Bybit:
		switch symbol {
		case `btcusd_p`:
			return 0.5
		case `ethusd_p`:
			return 1.5
		}
	case model.OKSwap:
		switch symbol {
		case `btcusd_p`:
			return 1
		case `ethusd_p`:
			return 2
		}
	case model.Ftx:
		switch symbol {
		case `BTC-PERP`:
			return 0
		case `ETH-PERP`, `LINK-PERP`, `BCH-PERP`, `BSV-PERP`:
			return 2
		case `ETC-PERP`:
			return 4
		case `EOS-PERP`:
			return 5
		case `XRP-PERP`:
			return 6
		}
	case model.HuobiDM:
		return 2
	case model.OKFUTURE:
		if strings.Contains(strings.ToLower(symbol), `btc`) {
			return 1
		} else if strings.Contains(strings.ToLower(symbol), `eth`) {
			return 2
		}
	}
	return 8
}

func GetAmountDecimal(market, symbol string) float64 {
	switch market {
	case model.OKEX:
		switch symbol {
		case `eos_usdt`, `btc_usdt`:
			return 44
		}
	case model.Bitmex, model.Bybit, model.OKSwap:
		return 0
	case model.OKFUTURE, model.HuobiDM:
		return 0
	}
	return 4
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
	case model.HuobiDM:
		result, errCode, msg = cancelOrderHuobiDM(key, secret, symbol, orderId)
	case model.OKEX:
		result, errCode, msg = cancelOrderOkex(key, secret, symbol, orderId)
	case model.OKFUTURE:
		result, errCode, msg = cancelOrderOkfuture(key, secret, instrument, orderId, orderType)
	case model.Binance:
		result, errCode, msg = cancelOrderBinance(key, secret, symbol, orderId)
	case model.Coinpark:
		result, errCode, msg = cancelOrderCoinpark(orderId)
	case model.Bitmex:
		result, errCode, msg = cancelOrderBitmex(key, secret, orderId)
	case model.Bybit:
		result, errCode, msg, order = cancelOrderBybit(key, secret, symbol, orderId)
	case model.OKSwap:
		result = cancelOrderOKSwap(key, secret, symbol, orderId)
	case model.Ftx:
		result = cancelOrderFtx(key, secret, orderType, orderId)
	}
	util.Notice(fmt.Sprintf(`[cancel %s %v %s %s]`, orderId, result, market, symbol))
	return result, errCode, msg, order
}

func QueryOrders(key, secret, market, instrument string) (
	orders []*model.Order) {
	switch market {
	case model.OKFUTURE:
		return queryOrdersOkfuture(key, secret, instrument)
	default:
		util.Notice(market + ` not supported`)
	}
	return nil
}

func GetCurrentInstrument(key, secret, market, symbol string) (currentInstrument string) {
	querySetter := querySetInstrumentsHuobiDM
	currentType := `quarter`
	//nextType := `bi_quarter`
	switch market {
	case model.OKFUTURE:
		querySetter = querySetInstrumentsOkFuture
		//nextType = `bi_quarter`
	case model.HuobiDM:
		querySetter = querySetInstrumentsHuobiDM
		//nextType = `next_quarter`
		symbol = symbol[0:strings.Index(symbol, `_`)]
	case model.OKSwap, model.OKEX:
		return symbol
	default:
		return symbol
	}
	querySetter(key, secret)
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
	candle = model.GetCandle(market, symbol+instrument, `1d`, timeCandle.Format(time.RFC3339)[0:10])
	if candle != nil && candle.N > 0 {
		return
	}
	candle = &model.Candle{}
	model.AppDB.Where(`market = ? and symbol = ? and period = ? and utc_date = ?`,
		market, symbol, `1d`, timeCandle.String()[0:10]).First(candle)
	if candle.N > 0 {
		return
	}
	dBegin, _ := time.ParseDuration(`-480h`)
	dEnd, _ := time.ParseDuration(`24h`)
	begin := timeCandle.Add(dBegin)
	end := timeCandle.Add(dEnd)
	var candles map[string]*model.Candle
	switch market {
	case model.Bitmex:
		candles = getCandlesBitmex(key, secret, symbol, `1d`, begin, end, 20)
	case model.Ftx:
		candles = getCandlesFtx(key, secret, symbol, `1d`, begin, end, 20)
	case model.OKEX:
		candles = getCandlesOKEX(key, secret, symbol, `1D`, begin, end, 20)
	case model.OKFUTURE:
		candles = getCandlesOkfuture(key, secret, symbol, instrument, `1d`, begin, end)
	case model.HuobiDM:
		candles = getCandlesHuobiDM(key, secret, symbol, `1d`, begin, time.Now())
	}
	for _, value := range candles {
		value.SymbolInstrument = value.SymbolInstrument + instrument
		c := model.GetCandle(value.Market, value.SymbolInstrument, value.Period, value.UTCDate)
		if c == nil || c.N == 0 {
			candleDB := &model.Candle{}
			model.AppDB.Where(`market = ? and symbol = ? and period = ? and utc_date = ?`,
				market, value.SymbolInstrument, `1d`, value.UTCDate).First(candleDB)
			if candleDB.N > 0 {
				value.N = candleDB.N
			}
			model.SetCandle(market, value.SymbolInstrument, `1d`, value.UTCDate, value)
		}
	}
	candle = model.GetCandle(market, symbol+instrument, `1d`, timeCandle.Format(time.RFC3339)[0:10])
	if candle == nil {
		util.Notice(fmt.Sprintf(`error: can not get candle %s %s %s %s`,
			market, symbol, `1d`, timeCandle.String()))
		return
	}
	candle.N = (candle.PriceHigh - candle.PriceLow) / 20
	for i := 1; i < 20; i++ {
		d, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		index := timeCandle.Add(d)
		candleCurrent := model.GetCandle(market, symbol+instrument, `1d`, index.Format(time.RFC3339)[0:10])
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
	model.AppDB.Save(&candle)
	model.SetCandle(market, symbol+instrument, `1d`, timeCandle.Format(time.RFC3339)[0:10], candle)
	return candle
}

func GetBalance(key, secret, market, coin string, delaySeconds int64) (balance *model.Balance) {
	success, balances := GetBalances(key, secret, market, delaySeconds)
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

func GetBalances(key, secret, market string, delaySeconds int64) (success bool, balances []*model.Balance) {
	now := util.GetNow().Unix()
	var update int64
	balances, update = model.GetBalance(market)
	if now-update < delaySeconds {
		return true, balances
	}
	switch market {
	case model.Ftx:
		success, balances = getBalanceFtx(key, secret)
	case model.OKEX:
		success, balances = getBalanceOKEX(key, secret)
	case model.OKFUTURE:
		success, balances = getBalanceOkfuture(key, secret)
	case model.HuobiDM:
		success, balances = getBalanceHuobiDM(key, secret)
	}
	model.SetBalance(market, balances, now)
	return
}

func GetTransfers(key, secret, market string) (balances []*model.Balance) {
	switch market {
	case model.Ftx:
		return getTransferFtx(key, secret)
	case model.OKEX, model.OKSwap, model.OKFUTURE:
		return getTransferOKEX(key, secret)
	case model.Huobi, model.HuobiDM:
		return getTransferHuobi(key, secret)
	}
	return balances
}

func GetUSDBalance(key, secret, market string) (balance float64) {
	balance = 0
	switch market {
	case model.Ftx:
		_, balances := getBalanceFtx(key, secret)
		for _, item := range balances {
			balance += item.UsdValue * 0.9
		}
		//case model.OKFUTURE:
		//	balance = getUSDBalanceOkfuture(key, secret)
	}
	return
}

func GetBtcBalance(key, secret, market string) (balance float64) {
	today := util.GetNow()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	//balance = model.GetBtcBalance(market, today)
	if balance > 0 {
		return
	}
	switch market {
	case model.Bitmex:
		balance = getBtcBalanceBitmex(key, secret)
		//model.SetBtcBalance(market, today, balance)
	}
	return
}

func GetFundingRate(market, symbol string) (fundingRate interface{}, expireTime int64) {
	fundingRate, expireTime = model.GetFundingRate(market, symbol)
	now := util.GetNow()
	if now.Unix()-60 < expireTime {
		return fundingRate, expireTime
	}
	util.Notice(fmt.Sprintf(`before update funding %s %s rate %f expire %d`,
		market, symbol, fundingRate, expireTime))
	switch market {
	case model.Bitmex:
		fundingRate, expireTime = getFundingRateBitmex(symbol)
		model.SetFundingRate(market, symbol, fundingRate, expireTime)
	case model.Bybit:
		fundingRate, expireTime = getFundingRateBybit(symbol)
		model.SetFundingRate(market, symbol, fundingRate, expireTime)
	case model.OKSwap:
		fundingRate, expireTime = getFundingRateOKSwap(symbol)
		model.SetFundingRate(market, symbol, fundingRate, expireTime)
	case model.Ftx:
		rates := getFundingRatesFtx()
		symbolRates := make(map[string][]*model.FundingRate)
		for _, rate := range rates {
			if symbolRates[rate.Symbol] == nil {
				symbolRates[rate.Symbol] = make([]*model.FundingRate, 0)
			}
			symbolRates[rate.Symbol] = append(symbolRates[rate.Symbol], rate)
		}
		duration, _ := time.ParseDuration(`3600s`)
		nextHour := now.Add(duration)
		nextHour = time.Date(nextHour.Year(), nextHour.Month(), nextHour.Day(),
			nextHour.Hour(), 0, 0, 0, now.Location())
		for symbol, value := range symbolRates {
			model.SetFundingRate(market, symbol, value, nextHour.Unix())
		}
		fundingRate, expireTime = model.GetFundingRate(market, symbol)
	}
	return
}

func QueryOrderById(key, secret, market, symbol, instrument, orderType, orderId string) (order *model.Order) {
	if instrument == `` {
		instrument = symbol
	}
	var dealAmount, dealPrice float64
	var status string
	switch market {
	case model.Huobi:
		dealAmount, dealPrice, status = queryOrderHuobi(key, secret, orderId)
	case model.HuobiDM:
		if orderType == model.OrderTypeStop {
			isWorking := queryOpenTriggerOrderHuobiDM(key, secret, symbol, orderId)
			if isWorking {
				status = model.CarryStatusWorking
			} else {
				relatedOrderId := queryHisTriggerOrderHuobiDM(key, secret, symbol, orderId)
				if relatedOrderId == `-1` || relatedOrderId == `` {
					status = model.CarryStatusFail
				} else {
					dealAmount, dealPrice, status = queryOrderHuobiDM(key, secret, symbol, relatedOrderId)
				}
			}
		} else {
			dealAmount, dealPrice, status = queryOrderHuobiDM(key, secret, symbol, orderId)
		}
	case model.OKEX:
		order = queryOrderOKEX(key, secret, instrument, orderId)
		order.Market = market
		order.Symbol = symbol
		return order
	case model.OKFUTURE:
		dealAmount, dealPrice, status = queryOrderOkfuture(key, secret, instrument, orderType, orderId)
	case model.Binance:
		dealAmount, dealPrice, status = queryOrderBinance(key, secret, symbol, orderId)
	case model.Coinpark:
		dealAmount, dealPrice, status = queryOrderCoinpark(orderId)
	case model.Bybit:
		orders := queryOrderBybit(key, secret, symbol, orderId)
		for _, value := range orders {
			if value.OrderId == orderId {
				return value
			}
		}
	case model.OKSwap:
		return queryOrderOKSwap(key, secret, symbol, orderId)
	case model.Ftx:
		if orderType == model.OrderTypeStop {
			newOrderId := queryTriggerOrderId(key, secret, orderId)
			if newOrderId != `` {
				return queryOrderFtx(key, secret, newOrderId)
			} else {
				status = queryOpenTriggerOrders(key, secret, symbol, orderId)
			}
		} else {
			return queryOrderFtx(key, secret, orderId)
		}
	}
	return &model.Order{OrderId: orderId, Symbol: symbol, Market: market, DealAmount: dealAmount, DealPrice: dealPrice,
		Status: status, Instrument: instrument, OrderType: orderType}
}

func GetPositions(key, secret, market string) (success bool, positions []*model.Position) {
	switch market {
	case model.Ftx:
		return getPositionsFtx(key, secret)
	case model.OKEX:
		return getPositionsOKEX(key, secret)
	}
	return false, nil
}

func MustPlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument, accountType, orderParam,
	refreshType string, price, triggerPrice, amount float64, saveDB bool) (order *model.Order) {
	retry := 10
	for i := 0; i < retry; i++ {
		order = PlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument, accountType,
			orderParam, refreshType, price, triggerPrice, amount, saveDB)
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

// orderSide: OrderSideBuy OrderSideSell OrderSideLiquidateLong OrderSideLiquidateShort
// orderType: OrderTypeLimit OrderTypeMarket
// amount:如果是限价单或市价卖单，amount是左侧币种的数量，如果是市价买单，amount是右测币种的数量
func PlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument, accountType, orderParam,
	refreshType string, price, triggerPrice, amount float64, saveDB bool) (order *model.Order) {
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
	if market == model.OKSwap {
		amount = amount / 100
	}
	price, strPrice := util.FormatNum(price, GetPriceDecimal(market, symbol))
	triggerPrice, strTriggerPrice := util.FormatNum(triggerPrice, GetPriceDecimal(market, symbol))
	_, strAmount := util.FormatNum(amount, GetAmountDecimal(market, symbol))
	util.Notice(fmt.Sprintf(`...%s %s %s before order %d amount:%s %f price:%s %f triggerPrice:%s`,
		orderSide, market, symbol, start, strAmount, amount, strPrice, price, strTriggerPrice))
	if model.AppConfig.Env == `test` {
		order.Status = model.CarryStatusSuccess
		order.OrderId = fmt.Sprintf(`%s%s%d`, market, symbol, util.GetNow().UnixNano())
		order.DealPrice = price
		order.DealAmount = amount
		if saveDB {
			go model.AppDB.Save(&order)
		}
		return
	}
	switch market {
	case model.Huobi:
		placeOrderHuobi(key, secret, order, orderSide, orderType, symbol, strPrice, strAmount)
	case model.HuobiDM:
		placeOrderHuobiDM(key, secret, order, orderSide, orderType, instrument, symbol, strPrice, strTriggerPrice, strAmount)
	case model.OKEX:
		placeOrderOKEX(key, secret, order)
	case model.OKFUTURE:
		placeOrderOkfuture(key, secret, order, orderSide, orderType, symbol, instrument, strPrice, strTriggerPrice, strAmount)
	case model.Binance:
		placeOrderBinance(key, secret, order, orderSide, orderType, symbol, strPrice, strAmount)
	case model.Coinpark:
		placeOrderCoinpark(order, orderSide, orderType, symbol, strPrice, strAmount)
		if order.ErrCode == `4003` {
			util.Notice(`【发现4003错误】sleep 3 minutes`)
			time.Sleep(time.Minute * 3)
		}
	case model.Bitmex:
		placeOrderBitmex(order, key, secret, orderSide, orderType, orderParam, symbol, strPrice, strAmount)
	case model.Bybit:
		placeOrderBybit(order, key, secret, orderSide, orderType, orderParam, symbol, strPrice, strAmount)
	case model.Ftx:
		placeOrderFtx(order, key, secret, orderSide, orderType, accountType, orderParam, symbol, strPrice,
			strTriggerPrice, fmt.Sprintf(`%f`, amount))
	case model.OKSwap:
		//account := model.AppAccounts.GetAccount(model.OKSwap, model.OrderSideSell+symbol)
		//if orderSide == model.OrderSideSell {
		//	account = model.AppAccounts.GetAccount(model.OKSwap, model.OrderSideBuy+symbol)
		//	if amountType != model.AmountTypeNew && account != nil && account.Free > amount*100 { // 平多
		//		orderSide = `3`
		//	} else { // 开空
		//		orderSide = `2`
		//	}
		//} else if orderSide == model.OrderSideBuy {
		//	if amountType != model.AmountTypeNew && account != nil && math.Abs(account.Free) > amount*100 { // 平空
		//		orderSide = `4`
		//	} else { // 开多
		//		orderSide = `1`
		//	}
		//}
		//placeOrderOKSwap(order, key, secret, orderSide, `0`, symbol, strPrice, strAmount)
	}
	if order.OrderId == "0" || order.OrderId == "" {
		order.Status = model.CarryStatusFail
	} else if order.Status == `` {
		order.Status = model.CarryStatusWorking
	}
	end := util.GetNowUnixMillion()
	util.Notice(fmt.Sprintf(`...%s %s %s return order at %d distance %d %s`,
		orderSide, market, symbol, end, end-start, order.Status))
	order.RefreshType = refreshType
	if saveDB {
		go model.AppDB.Save(&order)
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
			if subscribe != `` {
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
	return subscribes
}

func GetWSSubscribe(market, symbol, subType string) (subscribe interface{}) {
	switch market {
	case model.Huobi: // xrp_btc: market.xrpbtc.mbp.refresh.
		return "market." + strings.Replace(symbol, "_", "", 1) + ".mbp.refresh.5"
	case model.HuobiDM:
		return fmt.Sprintf(`market.%s.depth.step6`, symbol)
	case model.OKEX: // 未来基于market兼容okfuture LTC-USD-190628
		return GetCurrentInstrument(``, ``, market, symbol)
	case model.OKFUTURE:
		// btc-usd futures/ticker:BTC-USD-170310
		instrument := GetCurrentInstrument(``, ``, market, symbol)
		return `futures/depth5:` + instrument
	case model.Binance: // xrp_btc: xrpbtc@depth5
		if len(symbol) > 4 && symbol[0:4] == `bch_` {
			symbol = `bchabc_` + symbol[4:]
		}
		return strings.ToLower(strings.Replace(symbol, "_", "", 1)) + `@depth5`
	case model.Coinpark: //BTC_USDT bibox_sub_spot_BTC_USDT_ticker
		//return `bibox_sub_spot_` + strings.ToUpper(symbol) + `_ticker`
		return `bibox_sub_spot_` + strings.ToUpper(symbol) + `_depth`
	//case model.OKSwap:
	//	return `swap/depth5:` + model.GetDialectSymbol(model.OKSwap, symbol)
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

func InitCarryFtx(start uint) {
	rates := getFundingRatesFtx()
	symbolRates := make(map[string]bool)
	for _, rate := range rates {
		if symbolRates[rate.Symbol] == false {
			symbolRates[rate.Symbol] = true
		}
	}
	marketInfos := getMarketsFtx()
	model.AppDB.AutoMigrate(&model.Setting{})
	for symbol := range symbolRates {
		setting := &model.Setting{
			Valid:             true,
			Function:          model.FunctionCarry,
			OpenShortMargin:   0.013,
			CloseShortMargin:  -0.013,
			GridPriceDistance: -0.004,
			Market:            model.Ftx,
			Symbol:            symbol,
			AmountLimit:       0,
			ID:                start,
		}
		related := setting.GetRelatedSymbol()
		// HOLY 1INCH too easy to be single completed order
		if marketInfos[related] == nil || symbol == `FTT-PERP` || symbol == `USDT-PERP` || symbol == `BTC-PERP` ||
			symbol == `ETH-PERP` || symbol == `LINK-PERP` {
			continue
		}
		//if !marketInfos[related].CanBorrow {
		//	setting.CloseShortMargin = -1
		//	fmt.Println(related + `do not have borrow`)
		//}
		start++
		model.AppDB.Save(&setting)
		fmt.Println(fmt.Sprintf(`%s %s saved %d %f`, symbol, related, start, setting.CloseShortMargin))
	}
}

// 只支持现货SPOT和永续PERP SWAP
func InitMarketInfos() (success bool) {
	success = true
	markets := model.GetMarkets()
	for _, market := range markets {
		switch market {
		case model.Ftx:
			model.MarketInfos[model.Ftx] = getMarketsFtx()
		case model.OKEX:
			model.MarketInfos[model.OKEX] = getMarketsOKEX()
			if getAccountConfigOKEX(``, ``) != `net_mode` {
				if !setAccountModeOKEX(``, ``) {
					success = false
				}
			}
		}
	}
	return success
}

func FormatPrice(market, symbol string, price float64) (formattedPrice float64) {
	marketInfo := model.MarketInfos[market][symbol]
	if marketInfo == nil || marketInfo.SizeIncrement == 0 {
		return 0
	}
	return marketInfo.PriceIncrement * math.Round(price/marketInfo.PriceIncrement)
}

func ParseRealAmount(market, symbol string, amount float64) (success bool, realAmount float64) {
	marketInfo := model.MarketInfos[market][symbol]
	if marketInfo == nil || marketInfo.SizeIncrement == 0 || marketInfo.CTValue == 0 ||
		marketInfo.CTCurrency != model.GetCoin(market, symbol) {
		return false, 0
	}
	return true, amount * marketInfo.CTValue
}

func FormatAmount(market, symbol string, amount float64, round bool) (formattedAmount float64) {
	marketInfo := model.MarketInfos[market][symbol]
	if marketInfo == nil || marketInfo.SizeIncrement == 0 {
		return 0
	}
	if marketInfo.CTValue > 0 && marketInfo.CTCurrency == model.GetCoin(market, symbol) {
		amount = amount / marketInfo.CTValue
	}
	if round {
		formattedAmount = math.Round(amount/marketInfo.SizeIncrement) * marketInfo.SizeIncrement
	} else {
		formattedAmount = math.Floor(amount/marketInfo.SizeIncrement) * marketInfo.SizeIncrement
	}
	if formattedAmount < marketInfo.SizeMin {
		return 0
	}
	return formattedAmount
}

// GetMarketInfo
func _(market, symbol string) (borrowAble float64) {
	switch market {
	case model.Ftx:
		return getMarketInfoFtx(symbol)
	}
	return 0
}

func GetLastPrice(key, secret, market, symbol string) float64 {
	switch market {
	case model.OKEX:
		return getLastPriceOKEX(key, secret, symbol)
	}
	return 0
}

func InitCoinBalance(key, secret, function, market string) {
	InitMarketInfos()
	settings := model.GetSettings(function, market)
	_, balances := GetBalances(key, secret, market, 0)
	balanceMap := make(map[string]*model.Balance)
	for _, balance := range balances {
		balanceMap[balance.Coin] = balance
	}
	i := 0
	for _, items := range settings {
		coin := model.GetCoin(items[0].Market, items[0].Symbol)
		balance := balanceMap[coin]
		if balance == nil {
			if model.MarketInfos[market] == nil {
				continue
			}
			related := items[0].GetRelatedSymbol()
			marketInfo := model.MarketInfos[market][related]
			if marketInfo == nil {
				continue
			}
			price := GetLastPrice(key, secret, market, related)
			order := PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeMarket, market, related, related,
				``, ``, ``, price, price, marketInfo.SizeMin, false)
			if order.OrderId == `` {
				i++
				fmt.Println(fmt.Sprintf(`%d order return :%s %s`, i, order.ErrCode, related))
			} else {
				fmt.Println(fmt.Sprintf(`%s success order`, related))
			}
		}
	}
}

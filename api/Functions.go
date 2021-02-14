package api

import (
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
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
		case model.Bitmex, model.Bybit, model.Fmex:
			return 0.5
		case model.OKSwap:
			return 0.1
		}
	case `ethusd_p`:
		switch market {
		case model.Bitmex, model.Bybit, model.Fmex:
			return 0.05
		case model.OKSwap:
			return 0.01
		}
	}
	return 0
}

func GetMinAmount(market, symbol string) float64 {
	switch market {
	case model.Bitmex:
		return 1
	case model.Bybit:
		return 1
	case model.OKSwap:
		switch symbol {
		case `btcusd_p`:
			return 100
		case `ethusd_p`:
			return 100
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
			return 4
		}
	case model.Bitmex, model.Bybit, model.Fmex, model.OKSwap:
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
		result, errCode, msg = cancelOrderHuobi(orderId)
	case model.HuobiDM:
		result, errCode, msg = cancelOrderHuobiDM(symbol, orderId)
	case model.OKEX:
		result, errCode, msg = cancelOrderOkex(symbol, orderId)
	case model.OKFUTURE:
		result, errCode, msg = cancelOrderOkfuture(instrument, orderId, orderType)
	case model.Binance:
		result, errCode, msg = cancelOrderBinance(symbol, orderId)
	case model.Fcoin:
		result, errCode, msg = cancelOrderFcoin(key, secret, orderId)
	case model.Coinpark:
		result, errCode, msg = cancelOrderCoinpark(orderId)
	case model.Bitmex:
		result, errCode, msg = cancelOrderBitmex(key, secret, orderId)
	case model.Fmex:
		result, errCode, msg, order = cancelOrderFmex(key, secret, orderId)
		if order != nil {
			order.Symbol = symbol
		}
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

func QueryOrders(key, secret, market, symbol, instrument, states, accountTypes string, before, after int64) (
	orders []*model.Order) {
	switch market {
	case model.Fcoin:
		orders = make([]*model.Order, 0)
		if strings.Contains(accountTypes, model.AccountTypeNormal) {
			normal := queryOrdersFcoin(key, secret, symbol, states, model.AccountTypeNormal, before, after)
			for _, value := range normal {
				orders = append(orders, value)
			}
		}
		if strings.Contains(accountTypes, model.AccountTypeLever) {
			lever := queryOrdersFcoin(key, secret, symbol, states, model.AccountTypeLever, before, after)
			for _, value := range lever {
				orders = append(orders, value)
			}
		}
		return orders
	case model.Fmex:
		return queryOrdersFmex(key, secret, symbol)
	case model.OKFUTURE:
		return queryOrdersOkfuture(key, secret, instrument)
	default:
		util.Notice(market + ` not supported`)
	}
	return nil
}

func GetCurrentInstrument(market, symbol string) (currentInstrument string, isNext bool) {
	querySetter := querySetInstrumentsHuobiDM
	currentType := `quarter`
	nextType := `bi_quarter`
	switch market {
	case model.OKFUTURE:
		querySetter = querySetInstrumentsOkFuture
		nextType = `bi_quarter`
	case model.HuobiDM:
		querySetter = querySetInstrumentsHuobiDM
		nextType = `next_quarter`
		symbol = symbol[0:strings.Index(symbol, `_`)]
	default:
		return ``, false
	}
	if instruments == nil || instruments[market] == nil || instruments[market][symbol] == nil {
		querySetter()
	}
	if instruments == nil || instruments[market] == nil || instruments[market][symbol] == nil {
		util.Notice(fmt.Sprintf(`fatal error: can not get instrument %s %s`, market, symbol))
		return ``, false
	}
	instrument := instruments[market][symbol][currentType]
	instrumentNext := instruments[market][symbol][nextType]
	index := strings.LastIndex(instrument, `-`)
	if index == -1 {
		index = len(symbol) - 1
	}
	year, _ := strconv.ParseInt(`20`+instrument[index+1:index+3], 10, 64)
	month, _ := strconv.ParseInt(instrument[index+3:index+5], 10, 64)
	day, _ := strconv.ParseInt(instrument[index+5:index+7], 10, 64)
	today := time.Now().In(time.UTC)
	duration, _ := time.ParseDuration(`312h`)
	days13 := today.Add(duration)
	date := time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, today.Location())
	if today.After(date) {
		util.Notice(`future go cross ` + symbol + date.String())
		querySetter()
	}
	if days13.Before(date) {
		return instrument, false
	} else {
		return instrumentNext, true
	}
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
	if instrument == `` {
		instrument = symbol
	}
	candle = model.GetCandle(market, symbol, `1d`, timeCandle.Format(time.RFC3339)[0:10])
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
	case model.OKFUTURE:
		candles = getCandlesOkfuture(key, secret, symbol, instrument, `1d`, begin, end)
	case model.HuobiDM:
		candles = getCandlesHuobiDM(symbol, `1d`, begin, time.Now())
	}
	for _, value := range candles {
		c := model.GetCandle(value.Market, value.Symbol, value.Period, value.UTCDate)
		if c == nil || c.N == 0 {
			candleDB := &model.Candle{}
			model.AppDB.Where(`market = ? and symbol = ? and period = ? and utc_date = ?`,
				market, symbol, `1d`, value.UTCDate).First(candleDB)
			if candleDB.N > 0 {
				value.N = candleDB.N
			}
			model.SetCandle(market, symbol, `1d`, value.UTCDate, value)
		}
	}
	candle = model.GetCandle(market, symbol, `1d`, timeCandle.Format(time.RFC3339)[0:10])
	if candle == nil {
		util.Notice(fmt.Sprintf(`error: can not get candle %s %s %s %s`,
			market, symbol, `1d`, timeCandle.String()))
		return
	}
	candle.N = (candle.PriceHigh - candle.PriceLow) / 20
	for i := 1; i < 20; i++ {
		d, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		index := timeCandle.Add(d)
		candleCurrent := model.GetCandle(market, symbol, `1d`, index.Format(time.RFC3339)[0:10])
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
	model.SetCandle(market, symbol, `1d`, timeCandle.Format(time.RFC3339)[0:10], candle)
	return candle
}

func GetBalance(key, secret, market string, delaySeconds int64) (balances []*model.Balance) {
	now := util.GetNow().Unix()
	var update int64
	balances, update = model.GetBalance(market)
	if now-update < delaySeconds {
		return balances
	}
	switch market {
	case model.Ftx:
		balances = getBalanceFtx(key, secret)
	case model.OKEX:
		balances = getBalanceOKEX(key, secret)
	case model.OKFUTURE:
		balances = getBalanceOkfuture(model.AppAccounts)
	case model.HuobiDM:
		balances = getBalanceHuobiDM(model.AppAccounts)
	}
	model.SetBalance(market, balances, now)
	return
}

func GetTransfers(key, secret, market string) (balances []*model.Balance) {
	switch market {
	case model.Ftx:
		return getTransferFtx(key, secret)
	case model.OKEX, model.OKSwap, model.OKFUTURE:
		return getTransferOK(key, secret)
	case model.Huobi, model.HuobiDM:
		return getTransferHuobi()
	}
	return balances
}

func GetUSDBalance(key, secret, market string) (balance float64) {
	balance = 0
	switch market {
	case model.Ftx:
		balances := getBalanceFtx(key, secret)
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
		dealAmount, dealPrice, status = queryOrderHuobi(orderId)
	case model.HuobiDM:
		if orderType == model.OrderTypeStop {
			isWorking := queryOpenTriggerOrderHuobiDM(symbol, orderId)
			if isWorking {
				status = model.CarryStatusWorking
			} else {
				relatedOrderId := queryHisTriggerOrderHuobiDM(symbol, orderId)
				if relatedOrderId == `-1` || relatedOrderId == `` {
					status = model.CarryStatusFail
				} else {
					dealAmount, dealPrice, status = queryOrderHuobiDM(symbol, relatedOrderId)
				}
			}
		} else {
			dealAmount, dealPrice, status = queryOrderHuobiDM(symbol, orderId)
		}
	case model.OKEX:
		dealAmount, dealPrice, status = queryOrderOkex(symbol, orderId)
	case model.OKFUTURE:
		dealAmount, dealPrice, status = queryOrderOkfuture(instrument, orderType, orderId)
	case model.Binance:
		dealAmount, dealPrice, status = queryOrderBinance(symbol, orderId)
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

func RefreshCoinAccount(key, secret, setMarket, symbol, setCoin, accountType string) {
	//util.Notice(fmt.Sprintf(`[RefreshCoinAccount]%s %s %s %s`, setMarket, symbol, setCoin, accountType))
	switch setMarket {
	case model.Fcoin:
		if accountType == model.AccountTypeLever {
			setMarket = fmt.Sprintf(`%s_%s_%s`, setMarket, model.AccountTypeLever,
				strings.Replace(symbol, `_`, ``, 1))
			accounts := getLeverAccountFcoin(key, secret)
			for market, value := range accounts {
				if market == setMarket {
					for coin, account := range value {
						if coin == setCoin {
							util.Notice(fmt.Sprintf(`[update single coin]%s %s %s`, setMarket, symbol, setCoin))
							model.AppAccounts.SetAccount(setMarket, coin, account)
						}
					}
				}
			}
		} else if accountType == model.AccountTypeNormal {
			currencies, fcoinAccounts := getAccountFcoin(key, secret)
			for i := 0; i < len(currencies); i++ {
				if currencies[i] == setCoin {
					util.Notice(fmt.Sprintf(`[update single coin]%s %s %s`, setMarket, symbol, setCoin))
					model.AppAccounts.SetAccount(setMarket, currencies[i], fcoinAccounts[i])
				}
			}
		}
	}
}

//RefreshExclusive
func _(key, secret, market string, coins map[string]bool) {
	switch market {
	case model.Fcoin:
		accounts := getLeverAccountFcoin(key, secret)
		for key, value := range accounts {
			for coin, account := range value {
				if coins[coin] == false {
					model.AppAccounts.SetAccount(key, coin, account)
				}
			}
		}
		currencies, fcoinAccounts := getAccountFcoin(key, secret)
		for i := 0; i < len(currencies); i++ {
			if coins[currencies[i]] == false {
				model.AppAccounts.SetAccount(model.Fcoin, currencies[i], fcoinAccounts[i])
			}
		}
	}
}

var refreshTime = make(map[string]*time.Time)

func RefreshAccount(key, secret, market string) {
	util.SocketInfo(`refresh all accounts in market ` + market)
	duration, _ := time.ParseDuration(`-10s`)
	now := time.Now()
	checkTime := now.Add(duration)
	if refreshTime[market] != nil && refreshTime[market].After(checkTime) {
		return
	} else {
		refreshTime[market] = &now
	}
	model.AppAccounts.ClearAccounts(market)
	switch market {
	case model.Huobi:
		getAccountHuobiSpot(model.AppAccounts)
	case model.HuobiDM:
		getBalanceHuobiDM(model.AppAccounts)
		getHoldingHuobiDM(model.AppAccounts)
	case model.OKEX:
		getAccountOkex(model.AppAccounts)
	case model.OKFUTURE:
		getBalanceOkfuture(model.AppAccounts)
	case model.OKSwap:
		symbols := model.GetMarketSymbols(model.OKSwap)
		for symbol := range symbols {
			getAccountOKSwap(key, secret, symbol, model.AppAccounts)
		}
	case model.Binance:
		getAccountBinance(model.AppAccounts)
	case model.Coinpark:
		getAccountCoinpark(model.AppAccounts)
	case model.Bitmex:
		getAccountBitmex(key, secret, model.AppAccounts)
	case model.Bybit:
		symbols := model.GetMarketSymbols(model.Bybit)
		for symbol := range symbols {
			getAccountBybit(key, secret, symbol, model.AppAccounts)
		}
	case model.Ftx:
		getAccountFtx(key, secret, model.AppAccounts)
	}
}

func PlaceSyncOrders(key, secret, orderSide, orderType, market, symbol, instrument, amountType, accountType, orderParam,
	refreshType string, price, triggerPrice, amount float64, saveDB bool, channel chan *model.Order, retry int) {
	var order *model.Order
	i := 0
	forever := false
	if retry < 0 {
		forever = true
	}
	for ; i < retry || forever; i++ {
		order = PlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument, amountType, accountType,
			orderParam, refreshType, price, triggerPrice, amount, saveDB)
		if order != nil && order.OrderId != `` {
			break
		} else {
			if market == model.OKSwap && order != nil && order.ErrCode == `35010` {
				amountType = model.AmountTypeNew
				RefreshAccount(key, secret, model.OKSwap)
			}
			time.Sleep(time.Millisecond * 100)
			util.Notice(fmt.Sprintf(`fail to place order %d time, re order`, i))
		}
	}
	if i == retry {
		util.Notice(fmt.Sprintf(`fatal err: fail to order for %d times`, i))
	}
	if order == nil {
		order = &model.Order{}
	}
	channel <- order
}

func MustPlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument, amountType, accountType, orderParam,
	refreshType string, price, triggerPrice, amount float64, saveDB bool) (order *model.Order) {
	retry := 10
	for i := 0; i < retry; i++ {
		order = PlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument, amountType, accountType,
			orderParam, refreshType, price, triggerPrice, amount, saveDB)
		if order != nil && order.OrderId != `` {
			break
		} else {
			if market == model.OKSwap && order != nil && order.ErrCode == `35010` {
				amountType = model.AmountTypeNew
				RefreshAccount(key, secret, model.OKSwap)
			}
			time.Sleep(time.Second)
			util.Notice(fmt.Sprintf(`fail to place order %d time, re order`, i))
		}
	}
	return order
}

// orderSide: OrderSideBuy OrderSideSell OrderSideLiquidateLong OrderSideLiquidateShort
// orderType: OrderTypeLimit OrderTypeMarket
// amount:如果是限价单或市价卖单，amount是左侧币种的数量，如果是市价买单，amount是右测币种的数量
func PlaceOrder(key, secret, orderSide, orderType, market, symbol, instrument, amountType, accountType, orderParam,
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
		OrderTime: util.GetNow(), UnfilledQuantity: amount, Instrument: instrument}
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
		placeOrderHuobi(order, orderSide, orderType, symbol, strPrice, strAmount)
	case model.HuobiDM:
		account := model.AppAccounts.GetAccount(market, symbol)
		lever := `5`
		if account != nil {
			lever = strconv.FormatInt(account.LeverRate, 10)
		}
		placeOrderHuobiDM(order, orderSide, orderType, instrument, symbol, lever, strPrice, strTriggerPrice, strAmount)
	case model.OKEX:
		placeOrderOkex(order, orderSide, orderType, symbol, strPrice, strAmount)
	case model.OKFUTURE:
		placeOrderOkfuture(order, orderSide, orderType, symbol, instrument, strPrice, strTriggerPrice, strAmount)
	case model.Binance:
		placeOrderBinance(order, orderSide, orderType, symbol, strPrice, strAmount)
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
		account := model.AppAccounts.GetAccount(model.OKSwap, model.OrderSideSell+symbol)
		if orderSide == model.OrderSideSell {
			account = model.AppAccounts.GetAccount(model.OKSwap, model.OrderSideBuy+symbol)
			if amountType != model.AmountTypeNew && account != nil && account.Free > amount*100 { // 平多
				orderSide = `3`
			} else { // 开空
				orderSide = `2`
			}
		} else if orderSide == model.OrderSideBuy {
			if amountType != model.AmountTypeNew && account != nil && math.Abs(account.Free) > amount*100 { // 平空
				orderSide = `4`
			} else { // 开多
				orderSide = `1`
			}
		}
		placeOrderOKSwap(order, key, secret, orderSide, `0`, symbol, strPrice, strAmount)
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

func GetPrice(key, secret, symbol string) (buy float64, err error) {
	if model.AppConfig == nil {
		model.NewConfig()
	}
	strs := strings.Split(symbol, "_")
	strs[0] = strings.ToUpper(strings.TrimSpace(strs[0]))
	strs[1] = strings.ToUpper(strings.TrimSpace(strs[1]))
	if strs[0] == strs[1] {
		return 1, nil
	}
	symbol = strings.TrimSpace(strings.ToLower(symbol))
	result, price := model.AppMarkets.GetPrice(symbol)
	if result {
		return price, nil
	}
	price = model.AppConfig.SymbolPrice[symbol]
	if price > 0 && util.GetNowUnixMillion()-model.AppConfig.UpdatePriceTime[symbol] < 3600000 {
		return price, nil
	}
	model.AppConfig.SetUpdatePriceTime(symbol, util.GetNowUnixMillion())
	if strs[0] == `BIX` || strs[1] == `BIX` || strs[0] == `CP` || strs[1] == `CP` {
		return getBuyPriceCoinpark(symbol)
	}
	if strs[0] == `FT` || strs[1] == `FT` || model.AppConfig.InChina == 1 {
		return getBuyPriceFcoin(key, secret, symbol)
	}
	return getBuyPriceFcoin(key, secret, symbol)
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
		if market == model.OKSwap {
			subscribes = append(subscribes, model.GetWSSubscribePos(market, symbol))
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
	case model.OKEX: // xrp_btc: ok_sub_spot_xrp_btc_depth_5
		return "ok_sub_spot_" + symbol + "_depth_5"
	case model.OKFUTURE:
		// btc-usd futures/ticker:BTC-USD-170310
		instrument, _ := GetCurrentInstrument(market, symbol)
		return `futures/depth5:` + instrument
	case model.Binance: // xrp_btc: xrpbtc@depth5
		if len(symbol) > 4 && symbol[0:4] == `bch_` {
			symbol = `bchabc_` + symbol[4:]
		}
		return strings.ToLower(strings.Replace(symbol, "_", "", 1)) + `@depth5`
	case model.Fcoin:
		if subType == model.SubscribeDeal {
			// btc_usdt: trade.btcusdt
			return `trade.` + strings.ToLower(strings.Replace(symbol, "_", "", 1))
		} else if subType == model.SubscribeDepth {
			// btc_usdt: depth.L20.btcusdt
			return `depth.L20.` + strings.ToLower(strings.Replace(symbol, "_", "", 1))
		}
	case model.Fmex:
		if subType == model.SubscribeDepth {
			// btc_usdt: depth.L20.btcusdt
			return `depth.L20.` + symbol
		} else if subType == model.SubscribeDeal {
			return `trade.` + strings.ToUpper(symbol)
		}
	case model.Coinpark: //BTC_USDT bibox_sub_spot_BTC_USDT_ticker
		//return `bibox_sub_spot_` + strings.ToUpper(symbol) + `_ticker`
		return `bibox_sub_spot_` + strings.ToUpper(symbol) + `_depth`
	case model.OKSwap:
		return `swap/depth5:` + model.GetDialectSymbol(model.OKSwap, symbol)
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
		return []string{`orderbook`, symbol}
		//return []string{`ticker`, symbol}
	case model.Coinbig:
		switch symbol {
		case `btc_usdt`:
			return `27`
		case `eth_usdt`:
			return `28`
		}
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
			symbol == `ETH-PERP` || symbol == `DOGE-PERP` || symbol == `LINK-PERP` {
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

func InitMarketInfos() {
	markets := model.GetMarkets()
	for _, market := range markets {
		if market == model.Ftx {
			marketInfos := getMarketsFtx()
			model.MarketInfos[model.Ftx] = marketInfos
		}
	}
}

func FormatAmount(market, symbol string, amount float64) (formattedAmount float64) {
	marketInfo := model.MarketInfos[market][symbol]
	if marketInfo == nil || marketInfo.SizeIncrement == 0 {
		return 0
	}
	return math.Floor(amount/marketInfo.SizeIncrement) * marketInfo.SizeIncrement
}

func GetMarketInfo(market, symbol string) (borrowAble float64) {
	switch market {
	case model.Ftx:
		return getMarketInfoFtx(symbol)
	}
	return 0
}

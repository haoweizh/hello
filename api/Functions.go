package api

import (
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"strconv"
	"strings"
	"sync"
	"time"
)

var balanceLock = sync.Map{}     // key - locker
var positionLock = sync.Map{}    // key - locker
var mustPlaceLock = &sync.Map{}  // key - *sync.Mutex{}
var mustCancelLock = &sync.Map{} // key - *sync.Mutex{}
var requireReset sync.Map
var lastPrice = sync.Map{}            // market_symbol, price
var lastPriceTime = sync.Map{}        // market_symbol, Time
var tradeMax = &sync.Map{}            // key - symbol - [maxBuy, maxSell][]float64
var okTradeMaxResetTime = &sync.Map{} // key - symbol - init time in second
var okexCrossing = sync.Map{}         // symbol - bool
var USDs = map[string]bool{`USD`: true, `usd`: true, `USDT`: true, `usdt`: true, `USDC`: true, `usdc`: true, `BUSD`: true, `busd`: true}

func SetRequireReset(market string) {
	maintaining, _ := model.ChannelMaintaining.Load(market)
	if maintaining == nil || !maintaining.(bool) {
		util.Notice(`require reset %s`, market)
		initTime, getTime := model.AppEnvironment.WsInitTime.Load(market)
		if getTime && initTime != nil {
			checkTime := initTime.(time.Time).Add(time.Millisecond * time.Duration(model.AppConfig.Delay*5))
			if util.GetNow().After(checkTime) {
				requireReset.Store(market, true)
				util.Notice(`ready to reset ws channel %s reset after %v`, market, checkTime)
			}
		}
	}
}

func GetTradeMaxOKEX(key, secret, symbol string, expireSecond int64) (success bool, maxBuy, maxSell float64) {
	v, ok := util.LoadSyncMap(tradeMax, key, symbol)
	if expireSecond < 0 && ok && v != nil {
		return true, v.([]float64)[0], v.([]float64)[1]
	}
	vTime, okTime := util.LoadSyncMap(okTradeMaxResetTime, key, symbol)
	if v != nil && ok && vTime != nil && okTime && time.Now().Unix()-vTime.(int64) < expireSecond {
		return true, v.([]float64)[0], v.([]float64)[1]
	}
	success, maxBuy, maxSell = getMaxSizeOKEX(key, secret, symbol)
	if success {
		util.StoreSyncMap(tradeMax, []float64{maxBuy, maxSell}, key, symbol)
		util.StoreSyncMap(okTradeMaxResetTime, time.Now().Unix(), key, symbol)
	}
	return success, maxBuy, maxSell
}

func RequireKLineReset(environment *model.Environment, market string, symbols map[string]bool) (reset bool) {
	for symbol := range symbols {
		_, candle := environment.GetKLine(symbol, market)
		if candle == nil || candle.CreatedAt.Add(time.Duration(candle.Seconds)*time.Second).UnixMilli()+int64(model.AppConfig.Delay) <
			time.Now().UnixMilli() {
			reset = true
			if candle == nil {
				util.Notice(`RequireKLineReset symbol %s nil candle`, symbol)
			} else {
				util.Notice(`RequireKLineReset symbol %s candle time %s`, symbol, candle.CreatedAt.String())
			}
			break
		}
	}
	util.Notice(`RequireKLineReset %s %v`, market, reset)
	return reset
}

func RequireDepthChanReset(environment *model.Environment, market string) bool {
	needReset, ok := requireReset.Load(market)
	if ok && needReset != nil && needReset.(bool) {
		requireReset.Store(market, false)
		util.Notice(`clear need reset for market: ` + market)
		return true
	}
	now := util.GetNowUnixMillion()
	validSymbolNum := 0
	validSymbols := make(map[string]bool)
	symbols := GetMarketSymbols(market)
	for symbol := range symbols {
		if len(strings.Trim(symbol, ` `)) == 0 {
			validSymbolNum++
		}
		_, bidAsk := environment.GetBidAsk(symbol, market)
		if bidAsk == nil {
			continue
		}
		delay := float64(now - int64(bidAsk.Ts))
		if delay < model.AppConfig.Delay {
			validSymbolNum++
			validSymbols[symbol] = true
			//util.Notice(fmt.Sprintf(`RequireDepthChanReset valid %d %s %s %f<%f`,
			//	validSymbolNum, market, symbol, delay, model.AppConfig.Delay))
		} else {
			//util.Info(fmt.Sprintf(`RequireDepthChanReset delay too long %s %s %f`, market, symbol, delay))
		}
	}
	needReset = float64(validSymbolNum) < float64(len(symbols))*0.8 || len(symbols)-validSymbolNum > 50
	for funcName := range model.TickHandlers {
		settings := GetSettings(funcName, market)
		if settings == nil {
			continue
		}
		settings.Range(func(key, value interface{}) bool {
			if value == nil {
				return true
			}
			setting := value.(*model.Setting)
			if symbols[setting.Symbol] != true {
				return true
			}
			if setting.Function != model.FunctionCross && !validSymbols[setting.Symbol] {
				util.Notice(fmt.Sprintf(`need reset for important time out %s %s %s`,
					market, setting.Function, setting.Symbol))
				needReset = true
				return false
			}
			return true
		})
	}
	util.Info(fmt.Sprintf(`RequireDepthChanReset %d  %f valid %d in %d %s needReset %v`,
		now, model.AppConfig.Delay, validSymbolNum, len(symbols), market, needReset))
	return needReset.(bool)
}

func MustCancel(key, secret, market, symbol, orderType, orderId string, mustCancel bool) (res bool) {
	var lock *sync.Mutex
	lockValue, _ := mustCancelLock.Load(key)
	if lockValue == nil {
		lock = &sync.Mutex{}
		mustCancelLock.Store(key, lock)
	} else {
		lock = lockValue.(*sync.Mutex)
	}
	defer lock.Unlock()
	lock.Lock()
	sleepTime := 10
	for i := 0; i < 4; i++ {
		result, errCode, msg := CancelOrder(key, secret, market, symbol, orderType, orderId)
		res = result
		util.Notice(fmt.Sprintf(`[cancel] %s %s %s %s for %d times, return %t code %s msg %s `,
			market, symbol, orderType, orderId, i, result, errCode, msg))
		if result || !mustCancel || errCode == `0` {
			time.Sleep(time.Millisecond * 50)
			return result
		}
		if errCode == `3008` && i >= 3 {
			return result
		}
		if market == model.OKEX && strings.Contains(msg, `All operations failed`) {
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

func CancelAll(key, secret, market string) {
	switch market {
	case model.OKEX:
		cancelAllOkex(key, secret)
	case model.BinanceSpot:
		orders := queryOpenOrdersBinanceSpot(key, secret, ``)
		for _, order := range orders {
			result, _ := cancelOrderBinance(key, secret, market, order.Symbol, order.OrderId)
			util.Notice(fmt.Sprintf(`cancelAllBinace %s id %s return %v`,
				order.Symbol, order.OrderId, result))
			time.Sleep(time.Millisecond * 100)
		}
	case model.BinancePerp:
		orders := queryOpenOrdersBinancePerp(key, secret, ``)
		for _, order := range orders {
			result := cancelOrderBinancePerp(key, secret, order.Symbol, order.OrderId)
			util.Notice(fmt.Sprintf(`cancelAllBinancePerp %s id %s return %v`,
				order.Symbol, order.OrderId, result))
			time.Sleep(time.Millisecond * 100)
		}
	}
}

// CancelOrders 暂不支持策略订单
func CancelOrders(key, secret, market, symbol string) (result bool) {
	switch market {
	case model.BitgetPerp:
		result = cancelOrdersBitgetPerp(key, secret, symbol)
	case model.BitgetSpot:
		result = cancelOrdersBitgetSpot(key, secret, symbol)
	case model.KucoinSpot:
		result = cancelOrdersKucoinSpot(symbol)
	case model.KucoinPerp:
		result = cancelOrdersKucoinPerp(symbol)
	case model.Gate:
		result = cancelOrdersGate(key, secret, symbol)
	case model.Mexc:
		result = cancelOrdersMexc(key, secret, symbol)
	case model.BinanceSpot, model.BinanceMargin:
		result = cancelOrdersBinance(key, secret, market, symbol)
	case model.BinancePerp:
		result = cancelOrdersBinancePerp(key, secret, symbol)
	case model.Ftx:
		result = cancelOrdersFtx(key, secret, symbol)
	case model.Bybit:
		result = cancelOrdersBybit(key, secret, symbol)
	case model.OKEX:
		result, _, _ = cancelOrdersOKEX(key, secret, symbol)
	case model.HuobiSpot:
		result = cancelOrdersHuobiSpot(key, secret, symbol)
	}
	time.Sleep(time.Second * 2)
	util.Notice(fmt.Sprintf(`cancel all orders %s %s return %v`, market, symbol, result))
	return result
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
	//case model.BybitPerp:
	//	result, errCode, msg = cancelOrderBybitPerp(key, secret, symbol, orderId)
	//case model.BybitSpot:
	//	result, errCode, msg = cancelOrderBybitSpot(key, secret, symbol, orderId)
	case model.Ftx:
		result = cancelOrderFtx(key, secret, orderType, orderId)
	case model.Gate:
		result = cancelOrderGate(key, secret, symbol, orderId)
	case model.BinancePerp:
		result = cancelOrderBinancePerp(key, secret, symbol, orderId)
	case model.BinanceSpot:
		result, _ = cancelOrderBinance(key, secret, market, symbol, orderId)
	}
	util.Notice(fmt.Sprintf(`[cancel %s %v %s %s]`, orderId, result, market, symbol))
	return result, errCode, msg
}

func GetMarkPrice(account *model.Account, market, symbol string) (markPrice float64) {
	switch market {
	case model.BinancePerp:
		return getMarkPriceBinancePerp(account, symbol)
	}
	return
}

var CandleSeconds = []int{60, 1800, 3600, 86400}

func CombineCandles(account *model.Account, market, symbol string, slotSeconds int, begin, end time.Time) (combinedCandles model.Candles) {
	period := 0
	slots := 1
	for i := len(CandleSeconds) - 1; i >= 0; i-- {
		if slotSeconds%CandleSeconds[i] == 0 {
			period = CandleSeconds[i]
			slots = slotSeconds / CandleSeconds[i]
			break
		}
	}
	if period == 0 {
		return nil
	}
	candles := getCandle(account, market, symbol, period, begin, end)
	if slots == 1 {
		return candles
	}
	combinedCandles = make([]*model.Candle, 0)
	for i := 0; i <= len(candles)-slots; {
		if candles[i] != nil && candles[i].Begin.Unix()%int64(slotSeconds) == 0 {
			combine := &model.Candle{Market: candles[i].Market,
				Symbol:     candles[i].Symbol,
				Begin:      candles[i].Begin,
				Seconds:    candles[i].Seconds * slots,
				PriceOpen:  candles[i].PriceOpen,
				PriceHigh:  candles[i].PriceHigh,
				PriceLow:   candles[i].PriceLow,
				PriceClose: candles[i+slots-1].PriceClose,
				Volume:     candles[i].Volume}
			nextTime := candles[i].Begin.Unix() + int64(slotSeconds)
			for i = i + 1; i < len(candles) && candles[i].Begin.Unix() < nextTime; i++ {
				combine.Volume += candles[i].Volume
				if combine.PriceLow > candles[i].PriceLow {
					combine.PriceLow = candles[i].PriceLow
				}
				if combine.PriceHigh < candles[i].PriceHigh {
					combine.PriceHigh = candles[i].PriceHigh
				}
			}
			combinedCandles = append(combinedCandles, combine)
		} else {
			i++
		}
	}
	return combinedCandles
}

// GetCandle slotSeconds: candle的以秒计算宽度
func getCandle(account *model.Account, market, symbol string, slotSeconds int, begin, end time.Time) (candles []*model.Candle) {
	count := (end.Unix() - begin.Unix()) / int64(slotSeconds)
	limit := 100
	if market == model.BinancePerp {
		limit = 480
	} else if market == model.OKEX {
		limit = 300
	}
	if int(count) > limit {
		duration, _ := time.ParseDuration(fmt.Sprintf(`%ds`, limit*slotSeconds))
		candles = append(getCandle(account, market, symbol, slotSeconds, begin, begin.Add(duration)),
			getCandle(account, market, symbol, slotSeconds, begin.Add(duration), end)...)
	} else {
		isCache := false
		switch market {
		case model.Ftx:
			candles = getCandlesFtx(account, symbol, begin, end, slotSeconds)
		case model.OKEX:
			candles, isCache = getCandlesOKEX(account, symbol, begin, end, int(count), slotSeconds)
		case model.BinancePerp, model.BinanceSpot:
			candles, isCache = getCandlesBinance(account, market, symbol, begin, end, int(count), slotSeconds)
		case model.GXZQ:
			candles, isCache = getCandlesGXZQDB(symbol, begin, end, slotSeconds)
		}
		msg := fmt.Sprintf(`get candles %s %s %d seconds %s %d`,
			market, symbol, slotSeconds, begin.Format(time.RFC3339), len(candles))
		oldMsg, ok := util.LoadSyncMap(&model.CarryInfo, `GetCandle`)
		if ok && oldMsg != nil {
			msg = oldMsg.(string) + msg
		}
		if !isCache {
			util.Notice(msg)
			time.Sleep(time.Millisecond * 100)
		}
	}
	return
}

func GetMultiCandle(account *model.Account, market string, slotSeconds int, begin, end time.Time,
	settings map[string]*model.Setting, saveDB bool) (candles model.Candles) {
	count := (end.Unix() - begin.Unix()) / int64(slotSeconds)
	limit := 100
	if market == model.BinancePerp {
		limit = 480
	} else if market == model.OKEX {
		limit = 300
	} else if market == model.GXZQ {
		limit = 10000
	}
	if int(count) > limit {
		duration, _ := time.ParseDuration(fmt.Sprintf(`%ds`, limit*slotSeconds))
		candles = append(GetMultiCandle(account, market, slotSeconds, begin, begin.Add(duration), settings, saveDB),
			GetMultiCandle(account, market, slotSeconds, begin.Add(duration), end, settings, saveDB)...)
	} else {
		candles = make([]*model.Candle, count*int64(len(settings)))
		util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get multi candles slot%d count%d %s %s`,
			slotSeconds, count, begin.String(), end.String()), `GetCandle`)
		i := 0
		for symbol := range settings {
			var temp model.Candles
			var isCache bool
			switch market {
			case model.Ftx:
				temp = getCandlesFtx(account, symbol, begin, end, slotSeconds)
			case model.OKEX:
				temp, isCache = getCandlesOKEX(account, symbol, begin, end, int(count), slotSeconds)
			case model.BinancePerp, model.BinanceSpot:
				temp, isCache = getCandlesBinance(account, market, symbol, begin, end, int(count), slotSeconds)
			case model.GXZQ:
				temp, isCache = getCandlesGXZQDB(symbol, begin, end, slotSeconds)
			}
			for j := 0; j < temp.Len(); j++ {
				candles[j*len(settings)+i] = temp[j]
			}
			i++
			if !isCache {
				time.Sleep(time.Millisecond * 100)
			} else {
				//util.Notice(fmt.Sprintf(`get candles from cache %s %s %v %v %d %d`,
				//	market, symbol, begin, end, count, slotSeconds))
			}
		}
		if saveDB {
			model.AppDB.Save(candles)
		}
	}
	return
}

// GetPriceForce 返回tick价格
func GetPriceForce(_, _, symbol, market string) (result bool, price float64) {
	getBidAsk, bidAsk := model.AppEnvironment.GetBidAsk(symbol, market)
	if getBidAsk && bidAsk != nil {
		return true, bidAsk.Bids[0].Price
	}
	markets := GetMarkets()
	for _, m := range markets {
		getBidAsk, bidAsk = model.AppEnvironment.GetBidAsk(symbol, m)
		if getBidAsk && bidAsk != nil {
			return true, bidAsk.Bids[0].Price
		}
	}
	value, okPrice := lastPrice.Load(market + `_` + symbol)
	priceTime, okTime := lastPriceTime.Load(market + `_` + symbol)
	if okPrice && okTime && value != nil && priceTime.(time.Time).Add(time.Minute*10).After(time.Now()) {
		return true, value.(float64)
	}
	coins := strings.Split(symbol, `_`)
	if len(coins) == 2 && coins[0] == coins[1] {
		return true, 1
	}
	marketInfo := model.GetMarketInfo(market, symbol)
	if marketInfo == nil {
		//util.Info(fmt.Sprintf(`not in market infos %s %s %s %s`, market, symbol, key, secret[0:1]))
		return false, 0
	}
	lastPriceTime.Store(market+`_`+symbol, time.Now().Add(time.Second*14400))
	lastPrice.Store(market+`_`+symbol, price)
	return result, price
}

var getEquityTime = &sync.Map{}
var equityMsg = &sync.Map{}

func GetMarketEquity(index int) (msg string) {
	markets := GetMarkets()
	accounts := model.GetAccounts(index)
	if accounts == nil {
		return
	}
	value, ok := getEquityTime.Load(index)
	valueMsg, okMsg := equityMsg.Load(index)
	if ok && value != nil && okMsg && valueMsg != nil && value.(time.Time).Add(time.Minute*10).After(time.Now()) {
		return valueMsg.(string)
	}
	inAll := 0.0
	for _, market := range markets {
		if accounts[market] == nil {
			continue
		}
		//util.Notice(fmt.Sprintf(`try to get value for %s account %s`, market, accounts[market].Key[:5]))
		_, _, equity, _ := GetBalances(accounts[market].Key, accounts[market].Secret, market)
		if equity == 0 && !accounts[market].IsUnified {
			_, _, equity, _ = GetPositions(accounts[market].Key, accounts[market].Secret, market)
		}
		inAll += equity
		msg += fmt.Sprintf("%s: %f\n", market, equity)
		//util.Notice(fmt.Sprintf(`try to get value done %s account %s %s`, market, accounts[market].Key[:5], msg))
	}
	msg += fmt.Sprintf("账户总权益InUsd: %f\n", inAll)
	getEquityTime.Store(index, time.Now())
	equityMsg.Store(index, msg)
	return msg
}

func GetBalances(key, secret, market string) (
	success bool, balances []*model.Balance, totalInUsd float64, collateral *Collateral) {
	lock, _ := balanceLock.Load(key)
	if lock == nil {
		lock = &sync.Mutex{}
		balanceLock.Store(key, lock)
	}
	lock.(*sync.Mutex).Lock()
	defer func() {
		time.Sleep(time.Millisecond * 100)
		lock.(*sync.Mutex).Unlock()
	}()
	//now := util.GetNow().Unix()
	//var update int64
	//balances, totalInUsd, collateral, update = model.GetBalance(market)
	//if now-update < delaySeconds {
	//	return true, balances, totalInUsd, collateral
	//}
	switch market {
	case model.BitgetSpot:
		success, balances = getBalanceBitgetSpot(key, secret)
	case model.KucoinSpot:
		success, balances = getBalanceKucoinSpot(key, secret)
	case model.Gate:
		success, balances, totalInUsd, collateral = getBalanceGate(key, secret)
	case model.Ftx:
		success, balances, totalInUsd = getBalanceFtx(key, secret)
	case model.OKEX:
		success, balances, totalInUsd, collateral = getBalanceOKEX(key, secret)
	case model.BinanceSpot:
		success, balances = getBalanceBinanceSpot(key, secret)
	case model.BinanceMargin:
		success, balances = getBalanceBinanceMargin(key, secret)
	case model.Bybit:
		success, balances, totalInUsd, collateral = getBalanceBybit(key, secret)
	case model.HuobiSpot:
		success, balances = getBalanceHuobiSpot(key, secret)
	}
	accounts := model.AppConfig.GetAccounts(market)
	if len(accounts) > 0 && !accounts[0].IsUnified {
		for _, balance := range balances {
			if USDs[balance.Coin] {
				totalInUsd += balance.Amount
			} else {
				symbolStandard := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
				_, price := GetPriceForce(key, secret, symbolStandard, market)
				totalInUsd += price * balance.Amount
			}
		}
	}
	//util.Notice(fmt.Sprintf(`get balances %s %s %f %d %v`,
	//	market, key[:5], totalInUsd, len(balances), success))
	return success, balances, totalInUsd, collateral
}

func GetTransfers(key, secret, market string) (balances []*model.Balance) {
	switch market {
	case model.Ftx:
		return getTransferFtx(key, secret)
	case model.OKEX:
		return getTransferOKEX(key, secret)
	case model.BinanceSpot, model.BinancePerp, model.BinanceMargin:
		return GetWithdrawInfo(market, key, secret)
	}
	return balances
}

func GetFundingRate(key, secret, market, symbol string) (success bool, rate float64, updateTime time.Time) {
	//非永续合约的资金费率为0
	_, marketType, _, _ := model.GetFromStandard(market, symbol)
	if marketType != model.MarketTypePerp {
		return true, 0, util.GetNow()
	}
	value, ok := util.LoadSyncMap(model.FundingRates, market, symbol)
	var fundingRate *model.FundingRate
	if ok && value != nil {
		fundingRate = value.(*model.FundingRate)
	}
	now := util.GetNow().Unix()
	if fundingRate != nil && now < fundingRate.ExpireTime && fundingRate.UpdateTime.Add(time.Minute*5).After(time.Now()) {
		return true, fundingRate.Rate, fundingRate.UpdateTime
	}
	switch market {
	//case model.Bitmex:
	//	rate, expireTime = deprecated.getFundingRateBitmex(key, secret, symbol)
	//	model.SetFundingRate(market, symbol, &model.FundingRate{Rate: rate, ExpireTime: expireTime, UpdateTime: now})
	case model.BitgetPerp:
		fundingRate = getFundingRateBitgetPerp(symbol)
	case model.Bybit:
		fundingRate = getFundingRateBybit(symbol)
	case model.Ftx:
		fundingRate = GetFundingRatesFtx(key, secret, symbol)
	case model.OKEX:
		fundingRate = getFundingRateOKEX(key, secret, symbol)
	case model.Mexc:
		fundingRate = getFundingRateMexc(key, secret, symbol)
	case model.BinancePerp:
		fundingRate = getFundingRateBinancePerp(key, secret, symbol)
	case model.Gate:
		fundingRate = getFundingRateGate(key, secret, symbol)
	case model.Kucoin:
		fundingRate = &model.FundingRate{Rate: 0, RateNext: 0, UpdateTime: util.GetNow(), ExpireTime: now + 300}
	}
	if fundingRate == nil || now > fundingRate.ExpireTime {
		return false, 0, util.GetNow()
	}
	model.SetFundingRate(market, symbol, fundingRate)
	value, _ = util.LoadSyncMap(model.FundingRates, market, symbol)
	if value != nil {
		return true, value.(*model.FundingRate).Rate, value.(*model.FundingRate).UpdateTime
	}
	return false, 0, util.GetNow()
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

func QueryOpenOrders(key, secret, market, symbol string) (orders []*model.Order) {
	orders = make([]*model.Order, 0)
	switch market {
	case model.OKEX:
		orders = queryOpenOrdersOKEX(key, secret, symbol, true)
		temp := queryOpenOrdersOKEX(key, secret, symbol, false)
		for _, order := range temp {
			orders = append(orders, order)
		}
	case model.Ftx:
		orders = queryOrdersFtx(key, secret, symbol, true)
		for _, order := range queryOrdersFtx(key, secret, symbol, false) {
			orders = append(orders, order)
		}
	case model.BinancePerp:
		orders = queryOpenOrdersBinancePerp(key, secret, symbol)
	case model.BinanceSpot:
		orders = queryOpenOrdersBinanceSpot(key, secret, symbol)
	}
	return orders
}

func QueryOrderById(key, secret, market, symbol, orderType, orderId string) (order *model.Order) {
	order = &model.Order{
		OrderId: orderId, Symbol: symbol, Market: market, OrderType: orderType, Status: model.CarryStatusFail}
	switch market {
	case model.BitgetPerp:
		order = queryOrderBitgetPerp(key, secret, symbol, orderId)
	case model.BitgetSpot:
		order = queryOrderBitgetSpot(key, secret, symbol, orderId)
	case model.KucoinSpot:
		order = queryOrderKucoinSpot(symbol, orderId)
	case model.KucoinPerp:
		order = queryOrderKucoinPerp(symbol, orderId)
	case model.Gate:
		queryOrderGate(key, secret, order)
	case model.OKEX:
		order = queryOrderOKEX(key, secret, symbol, orderId, orderType)
		return order
	case model.BinanceSpot:
		order = queryOrderBinanceSpot(key, secret, symbol, orderId)
	case model.BinancePerp:
		order = queryOrderBinancePerp(key, secret, symbol, orderId)
	case model.HuobiPerp:
		order = queryOrderHuobiPerp(key, secret, symbol, orderId)
	case model.Bybit:
		order = queryOrderBybit(key, secret, symbol, orderId)
	case model.HuobiSpot:
		order = queryOrderHuobiSpot(key, secret, orderId)
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
func GetPositions(key, secret, market string) (success bool, positions []*Position, accountValue, availableU float64) {
	lock, _ := positionLock.Load(key)
	if lock == nil {
		lock = &sync.Mutex{}
		positionLock.Store(key, lock)
	}
	lock.(*sync.Mutex).Lock()
	defer func() {
		time.Sleep(time.Millisecond * 100)
		lock.(*sync.Mutex).Unlock()
	}()
	switch market {
	case model.BitgetPerp:
		success, positions, accountValue, availableU = getPositionsBitgetPerp(key, secret)
	case model.KucoinPerp:
		success, positions, accountValue, availableU = getPositionsKucoinPerp(key, secret)
	case model.Gate:
		_, _, total, collateral := getBalanceGate(key, secret)
		success, positions = getPositionsGate(key, secret)
		accountValue, availableU = total, collateral.Available
	case model.Mexc:
		success, positions, accountValue, availableU = getPositionsMexc(key, secret)
	case model.BinancePerp:
		success, positions, accountValue, availableU = getPositionsBinancePerp(key, secret)
	case model.Ftx:
		var balances []*model.Balance
		success, balances, accountValue = getBalanceFtx(key, secret)
		for _, balance := range balances {
			if strings.EqualFold(balance.Coin, `usd`) {
				availableU += balance.Amount
			}
		}
		success, positions, _ = getPositionsFtx(key, secret)
	case model.Bybit:
		_, _, total, collateral := getBalanceBybit(key, secret)
		success, positions, _ = getPositionsBybit(key, secret)
		accountValue, availableU = total, collateral.Available
	case model.HuobiPerp:
		success, positions, accountValue, availableU = getPositionsHuobiPerp(key, secret)
	case model.OKEX:
		_, _, total, collateral := getBalanceOKEX(key, secret)
		success, positions = getPositionsOKEX(key, secret)
		accountValue, availableU = total, collateral.Available
	}
	//util.Notice(fmt.Sprintf(`get positions %s %s %f %f %d %v`,
	//	market, key[:5], accountValue, availableU, len(positions), success))
	return success, positions, accountValue, availableU
}

func GetStandardOrderType(market, dialectType string) (standardType string) {
	switch market {
	case model.BinancePerp:
		switch dialectType {
		case `STOP`:
			return model.OrderTypeStop
		case `LIMIT`:
			return model.OrderTypeLimit
		case `MARKET`:
			return model.OrderTypeMarket
		case `TRAILING_STOP_MARKET`:
			return model.OrderTypeTrailStop
		default:
			return model.OrderTypeLimit
		}
	case model.BinanceSpot, model.BinanceMargin:
		switch dialectType {
		case `LIMIT`:
			return model.OrderTypeLimit
		case `MARKET`:
			return model.OrderTypeMarket
		case `STOP_LOSS`, `TAKE_PROFIT`, `STOP_LOSS_LIMIT`, `TAKE_PROFIT_LIMIT`:
			return model.OrderTypeStop
		default:
			return `` // LIMIT_MAKER
		}
	}
	return ``
}

func MustPlaceOrder(key, secret, orderSide, orderType, market, symbol, orderParam,
	refreshType string, price, triggerPrice, amount float64, setting *model.Setting) (orders []*model.Order) {
	var lock *sync.Mutex
	lockValue, _ := mustPlaceLock.Load(key)
	if lockValue == nil {
		lock = &sync.Mutex{}
		mustPlaceLock.Store(key, lock)
	} else {
		lock = lockValue.(*sync.Mutex)
	}
	defer lock.Unlock()
	lock.Lock()
	retry := 2
	for i := 0; i < retry; i++ {
		v, _ := util.LoadSyncMap(model.MarketInfos, market, symbol)
		if v == nil {
			util.Notice(`fail to get market info when must place %s %s`, market, symbol)
			continue
		}
		if v.(*model.MarketInfo).SizeMax > 0 && amount > v.(*model.MarketInfo).SizeMax {
			ordersLeft := MustPlaceOrder(key, secret, orderSide, orderType, market, symbol, orderParam, refreshType, price, triggerPrice, amount/2, setting)
			orders = MustPlaceOrder(key, secret, orderSide, orderType, market, symbol, orderParam, refreshType, price, triggerPrice, amount/2, setting)
			if ordersLeft == nil || len(ordersLeft) == 0 {
				return orders
			} else if orders == nil || len(orders) == 0 {
				return ordersLeft
			}
			for _, order := range ordersLeft {
				orders = append(orders, order)
			}
			return orders
		} else {
			order := PlaceOrder(key, secret, orderSide, orderType, market, symbol,
				orderParam, price, triggerPrice, amount, false, nil, setting)
			if order != nil && order.OrderId != `` && order.Status != model.CarryStatusFail {
				order.RefreshType = refreshType
				return []*model.Order{order}
			} else { // binance perp 下单返回 -4005 代表数量太大，分两半继续下单
				if order != nil && strings.Contains(order.ErrCode, `-4005`) {
					v.(*model.MarketInfo).SizeMax = 0.52 * amount
					util.Notice(fmt.Sprintf(`binance perp 下单返回 -4005 代表数量%f太大，减小marketInfo size max %f`,
						amount, v.(*model.MarketInfo).SizeMax))
				}
				// <APIError> code=-2027, msg=Exceeded the maximum allowable position at current leverage.
				//if order != nil && strings.Contains(order.ErrCode, `-2027`) {
				//	break
				//}
				time.Sleep(time.Second * 10)
				util.Notice(fmt.Sprintf(`fail to place order %d time, re order`, i))
			}
		}
	}
	return orders
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
	if amount == 0 {
		util.Notice(fmt.Sprintf(`can not place order with amount 0 , %s %s %s %s`, orderSide, orderType, market, symbol))
		return &model.Order{OrderSide: markSide, OrderType: orderType, Market: market, Symbol: symbol,
			Price: price, Amount: 0, OrderId: ``, ErrCode: ``, TriggerPrice: triggerPrice,
			Status: model.CarryStatusFail, DealAmount: 0, DealPrice: price, OrderTime: util.GetNow()}
	}
	account := model.AppConfig.GetAccountFromKeyIndex(market, key, -1)
	order = &model.Order{OrderSide: markSide, OrderType: orderType, Market: market, Symbol: symbol, Price: price,
		Amount: amount, DealAmount: 0, DealPrice: price, TriggerPrice: triggerPrice,
		OrderTime: util.GetNow(), UnfilledQuantity: amount, AccountIndex: account.Index}
	//util.Notice(fmt.Sprintf(`...%s %s %s before order %d amount: %f price:%f triggerPrice:%f`,
	//	orderSide, market, symbol, start, amount, price, triggerPrice))
	if model.AppConfig.Env == `test` {
		return
	}
	switch market {
	case model.BitgetPerp:
		placeOrderBitgetPerp(key, secret, order, orderSide, orderType, orderParam, symbol, price, amount)
	case model.BitgetSpot:
		placeOrderBitgetSpot(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.KucoinSpot:
		placeOrderKucoinSpot(order, orderSide, orderType, symbol, price, amount)
	case model.KucoinPerp:
		placeOrderKucoinPerp(order, orderSide, orderType, symbol, price, amount)
	case model.Gate:
		placeOrderGate(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.OKEX:
		placeOrderOKEX(account, isWs, order)
		if isWs {
			order.OrderId = strconv.FormatInt(time.Now().UnixNano(), 10) + symbol
		}
	case model.BinanceSpot:
		placeOrderBinanceSpot(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.BinancePerp:
		placeOrderBinancePerp(key, secret, order, orderSide, orderType, symbol, price, triggerPrice, amount)
	case model.Bybit:
		placeOrderBybit(key, secret, order, orderSide, orderType, orderParam, symbol, price, amount)
	case model.HuobiSpot:
		placeOrderHuobiSpot(key, secret, order, orderSide, orderType, symbol, price, amount)
	case model.HuobiPerp:
		placeOrderHuobiPerp(key, secret, order, orderSide, orderType, ``, symbol, price, price, amount)
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
	order.TriggerPrice = triggerPrice
	end := util.GetNowUnixMillion()
	util.Notice(fmt.Sprintf(`...%s %s %s return order at %d distance %d %s %s price %f %f amount %f %f trigger %f %f id %s`,
		orderSide, market, symbol, end, end-start, order.Status, order.ErrCode, price, order.Price, amount, order.Amount,
		triggerPrice, order.TriggerPrice, order.OrderId))
	if postOrder != nil && setting != nil {
		go postOrder(order)
	}
	return order
}

func GetWSSubscribes(market, subType string) []interface{} {
	symbols := GetMarketSymbols(market)
	subscribes := make([]interface{}, 0)
	for symbol := range symbols {
		if len(strings.Trim(symbol, ` `)) == 0 {
			continue
		}
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
	case model.BinanceSpot, model.BinanceMargin:
		go maintainChannelBinance(market, subscribes)
	case model.BinancePerp:
		go maintainChannelBinancePerp(subscribes)
	case model.Ftx:
		go maintainChannelFtx(subscribes)
	//case model.BybitPerp:
	//	go maintainChannelBybitPerp(subscribes)
	//case model.BybitSpot:
	//	go maintainChannelBybitSpot(subscribes)
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
		} else if subType == model.SubscribeTicker {
			return strings.ToLower(dialectSymbol) + `@bookTicker`
		} else if subType == model.SubscribeMarkPrice {
			return strings.ToLower(dialectSymbol) + `@markPrice@1s`
			//return `!markPrice@arr`
		}
	case model.BinanceSpot, model.BinanceMargin: // XRPUSDT: XRPUSDT@depth5   XRP-PERP: XRPUSDT@depth5
		if subType == model.SubscribeDepth {
			return strings.ToLower(dialectSymbol) + `@depth5@100ms`
		}
		return strings.ToLower(dialectSymbol) + `@bookTicker`
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

// FilterCross
// 由于gate下线暂时不平仓的币 BCD OXY CRU
// 搬砖过滤币种 AMPL IOTA REEF MIR SOS
// 某些主流币 BTC ETH LINK
// 币种对不上 REAL, DFL, QI, WSB, TRADE,FAME,BIFI,TON,BOX,PAY,BTT
// 法币 GBP CUSDT TRYB BRZ CAD EUR SUSD USDC TUSD`USDT EURT USD BUSD LDBUSD LDUSDT
// ftx预测TRUMP BOLSONARO
func FilterCross(market, symbol string) bool {
	filterCoins := map[string]bool{`AMPL`: true, `IOTA`: true, `REEF`: true, `MIR`: true, `LUNA`: true, // `UST`: true,
		`BTC`: true, `ETH`: true, `LINK`: true, `SOS`: true, // `BTT`: true,
		`REAL`: true, `DFL`: true, `QI`: true, `WSB`: true, `TRADE`: true, `FAME`: true, `BIFI`: true, `TON`: true,
		`BOX`: true, `PAY`: true, `GTC`: true, `OXY`: true, `CRU`: true, `BCD`: true,
		`GBP`: true, `CUSDT`: true, `TRYB`: true, `BRZ`: true, `CAD`: true, `EUR`: true, `SUSD`: true, `USDC`: true,
		`TUSD`: true, `USDT`: true, `EURT`: true, `USD`: true, `BUSD`: true, `LDUSDT`: true, `LDBUSD`: true,
		`TRUMP`: true, `BOLSONARO`: true, `DEFI`: true}
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
	case model.Gate:
		switch coin {
		case `GT`:
			return true
		}
	case model.Ftx:
		switch coin {
		case `PRIV`, `ALT`, `SHIT`, `MID`, `EXCH`, `DRGN`, `FTT`:
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
	case model.Mexc: //不支持主流币种期货下单
		switch coin {
		case `BTC`, `ETH`, `LTC`, `FTT`, `MX`:
			return true
		}
	}
	return false
}

const topMarketInfoLenCross = 30

// InitCrossMarketInfos 用以初始化cross carry的各个币种市场，调用前需要truncate settings数据库表，本方法会从新插入
func InitCrossMarketInfos(markets []string) {
	infoPool := make(map[string][]*model.MarketInfo) // coin - []marketInfos
	topCoins := make(map[string]bool)
	for _, market := range markets {
		InitMarketInfos(market)
		if market != model.OKEX && market != model.BinancePerp {
			continue
		}
		_, topInfos := getSortedInfos(market, topMarketInfoLenCross)
		for name, info := range topInfos {
			_, _, coin, _ := model.GetFromStandard(info.Market, name)
			if !model.CommonCoins[strings.ToLower(coin)] {
				topCoins[coin] = true
			}
		}
	}
	model.MarketInfos.Range(func(key, value any) bool {
		if value == nil {
			return true
		}
		success, _, coin, _ := model.GetFromStandard(value.(*model.MarketInfo).Market, value.(*model.MarketInfo).Name)
		if success && coin != `` {
			if infoPool[coin] == nil {
				infoPool[coin] = make([]*model.MarketInfo, 0)
			}
			if !FilterCross(value.(*model.MarketInfo).Market, value.(*model.MarketInfo).Name) {
				infoPool[coin] = append(infoPool[coin], value.(*model.MarketInfo))
			}
		}
		return true
	})
	var settingsDb []*model.Setting
	model.AppDB.Find(&settingsDb)
	settingsDbMap := make(map[string]*model.Setting)
	for _, setting := range settingsDb {
		settingsDbMap[fmt.Sprintf(`%s_%s_%s`, setting.Function, setting.Market, setting.Symbol)] = setting
	}
	model.AppDB.Model(&settingsDb).Where(`function=?`, model.FunctionCross).Updates(map[string]interface{}{`valid`: false})
	for coin, infos := range infoPool {
		util.Notice(`handle coin %s %d`, coin, len(infos))
		scoreOpen := 0.015
		scoreClose := 0.005
		if len(infos) >= 2 {
			topCross := ``
			if topCoins[coin] {
				topCross = model.TopCross
			}
			for _, info := range infos {
				if settingsDbMap[fmt.Sprintf(`%s_%s_%s`, model.FunctionCross, info.Market, info.Name)] == nil {
					setting := &model.Setting{
						Valid:            true,
						Function:         model.FunctionCross,
						WSType:           model.WSTypeTicker,
						Market:           info.Market,
						Symbol:           info.Name,
						Coin:             coin,
						OpenShortMargin:  scoreOpen,
						CloseShortMargin: scoreClose,
						SymbolRelated:    topCross}
					util.Notice(fmt.Sprintf(`save setting %s %s %s %v`, info.Market, info.Name, coin, setting.Valid))
					model.AppDB.Save(setting)
				} else {
					model.AppDB.Model(&settingsDb).Where("market= ? and symbol= ? and function= ?",
						info.Market, info.Name, model.FunctionCross).Updates(map[string]interface{}{
						`valid`:              true,
						`open_short_margin`:  scoreOpen,
						`close_short_margin`: scoreClose,
						`symbol_related`:     topCross})
				}
			}
		}
	}
}

var marketInfoInitializing = false

// InitMarketInfos 只支持现货SPOT和永续PERP SWAP
func InitMarketInfos(market string) (success bool) {
	if marketInfoInitializing {
		return
	}
	util.Notice(fmt.Sprintf(`start to init market infos %s`, market))
	marketInfoInitializing = true
	defer func() {
		marketInfoInitializing = false
	}()
	success = true
	accounts := model.AppConfig.GetAccounts(market)
	var marketInfos map[string]*model.MarketInfo
	switch market {
	case model.Mexc:
		marketInfos = getMarketsMexc(accounts[0].Key, accounts[0].Secret)
	case model.Ftx:
		marketInfos = getMarketsFtx(accounts[0].Key, accounts[0].Secret)
	case model.OKEX:
		marketInfos = getMarketsOKEX(accounts[0].Key, accounts[0].Secret)
		for _, account := range accounts {
			accountMode := getAccountConfigOKEX(account.Key, account.Secret)
			util.Notice(`okex config and set: ` + accountMode)
			if accountMode != `net_mode` {
				if !setAccountModeOKEX(account.Key, account.Secret) {
					success = false
				}
			}
		}
		go func() {
			for _, account := range accounts {
				setLeverageOkx(account)
			}
		}()
	case model.HuobiSpot:
		marketInfos = getMarketsHuobiSpot(accounts[0].Key, accounts[0].Secret)
	case model.BinanceSpot:
		marketInfos = GetMarketsBinance(accounts[0], market, model.MarketTypeSpot)
	case model.BinanceMargin:
		marketInfos = GetMarketsBinance(accounts[0], market, model.MarketTypeMargin)
	case model.BinancePerp:
		marketInfos = getMarketsBinancePerp(accounts[0].Key, accounts[0].Secret)
		go func() {
			for _, account := range accounts {
				setPosSideBinancePerp(account.Key, account.Secret)
				setLeverageBinancePerp(account.Key, account.Secret)
			}
		}()
	case model.Gate:
		for _, account := range accounts {
			setPosSideGate(account.Key, account.Secret)
			setMarginSettingGate(account.Key, account.Secret)
		}
		_, marketInfos = getMarketsGate(accounts[0].Key, accounts[0].Secret)
	case model.KucoinSpot:
		marketInfos = getMarketsKucoinSpot(accounts[0].Key)
	case model.KucoinPerp:
		marketInfos = getMarketsKucoinPerp(accounts[0].Key)
		setFutureAutoDeposit()
	case model.Bybit:
		marketInfos = getMarketsBybit()
		go func() {
			for _, account := range accounts {
				setBybitMarginLeverage(account.Key, account.Secret)
				time.Sleep(time.Second)
				setBybitPerpLeverage(account.Key, account.Secret)
			}
		}()
	case model.BitgetSpot:
		marketInfos = getMarketsBitgetSpot()
	case model.BitgetPerp:
		marketInfos = getMarketsBitgetPerp()
		setBitgetPositionMode(accounts[0].Key, accounts[0].Secret)
	}
	for _, setting := range appSettings {
		if setting.Market == market && marketInfos[setting.Symbol] == nil && strings.Trim(setting.Symbol, ` `) != `` {
			setting.Valid = false
			util.Notice(`warning %s %s un-list from market`, market, setting.Symbol)
		}
	}
	model.MarketInfos.Range(func(key, value any) bool {
		if strings.Index(key.(string), market) == 0 {
			model.MarketInfos.Delete(key)
		}
		return true
	})
	for symbol, info := range marketInfos {
		util.StoreSyncMap(model.MarketInfos, info, market, symbol)
	}
	return success
}

func CreateAccountWsServer(market string) {
	switch market {
	case model.BinancePerp:
		go WsAccountServeBinancePerp()
	case model.BinanceSpot:
		go WsAccountServeBinanceSpot()
	case model.OKEX:
		WsAccountServeOKEX()
		go maintainAccountConnOKEX()
	}
}

func CreateMarketKLineWS(environment *model.Environment, market string, symbols map[string]bool) (
	socketMap map[*websocket.Conn]bool, channels []chan struct{}) {
	switch market {
	case model.BinanceSpot:
		util.Notice(" create KLine ws chan for " + market)
		socketMap, channels, _ = WsKLineBinanceSpot(environment, market, symbols)
	}
	return
}

func CreateMarketTickerWS(environment *model.Environment, market string) (
	socketMap map[*websocket.Conn]bool, channels []chan struct{}) {
	model.ChannelMaintaining.Store(market, true)
	util.Notice(" create depth chan for " + market)
	channels = make([]chan struct{}, 1)
	var err error
	switch market {
	case model.KucoinSpot:
		channels, err = WsDepthServeKucoinSpot()
	case model.KucoinPerp:
		channels, err = WsDepthServeKucoinPerp()
	case model.Gate:
		socketMap, channels, err = WsDepthServeGateNew(environment, market)
	case model.OKEX:
		socketMap, channels, err = WsDepthServeOKEX(environment, market, GetMarketSymbols(model.OKEX))
	case model.BinanceSpot, model.BinanceMargin:
		socketMap, channels, err = WsDepthServeBinance(environment, market)
	case model.BinancePerp:
		socketMap, channels, err = WsDepthServeBinancePerp(environment, market)
	case model.HuobiPerp:
		socketMap, channels, err = WsDepthServeHuobiPerp(environment, market)
	case model.Bybit:
		socketMap, channels, err = WsDepthServeBybit(environment, market)
	case model.HuobiSpot:
		socketMap, channels, err = WsDepthServeHuobiSpot(environment, market)
	case model.Ftx:
		socketMap, channels, err = WsDepthServeFtx(environment, market)
	case model.Mexc:
		socketMap, channels, err = WsDepthServeMexc(environment, market, true)
	case model.BitgetSpot:
		socketMap, channels, err = WsDepthServeBitgetSpot(environment, market)
	case model.BitgetPerp:
		socketMap, channels, err = WsDepthServeBitgetPerp(environment, market)
	}
	if err != nil {
		util.Notice(market + ` can not create depth server ` + err.Error())
	}
	model.AppEnvironment.WsInitTime.Store(market, util.GetNow())
	model.ChannelMaintaining.Store(market, false)
	return socketMap, channels
}

func SendMails(title, msg string) {
	util.Notice(fmt.Sprintf(`try to send mails %s %s`, title, msg))
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

func SetSymbolLeverage(account *model.Account, market, symbol string) (success bool) {
	switch market {
	case model.BinancePerp:
		return setSymbolLeverageBinancePerp(account, symbol)
	case model.Bybit:
		return setSymbolLeverageBybit(account, symbol)
	case model.OKEX:
		return setSymbolLeverageOkx(account, symbol)
	}
	return false
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

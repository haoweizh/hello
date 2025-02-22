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

var balanceLock = sync.Map{}          // key - locker
var positionLock = sync.Map{}         // key - locker
var mustPlaceLock = &sync.Map{}       // key - *sync.Mutex{}
var mustCancelLock = &sync.Map{}      // key - *sync.Mutex{}
var tradeMax = &sync.Map{}            // key - symbol - [maxBuy, maxSell][]float64
var okTradeMaxResetTime = &sync.Map{} // key - symbol - init time in second
var okexCrossing = sync.Map{}         // symbol - bool
var USDs = map[string]bool{`USD`: true, `usd`: true, `USDT`: true, `usdt`: true, `USDC`: true, `usdc`: true, `BUSD`: true, `busd`: true}

func GetTradeMaxOKEX(account *model.Account, symbol string, expireSecond int64) (success bool, maxBuy, maxSell float64) {
	v, ok := util.LoadSyncMap(tradeMax, account.Key, symbol)
	if expireSecond < 0 && ok && v != nil {
		return true, v.([]float64)[0], v.([]float64)[1]
	}
	vTime, okTime := util.LoadSyncMap(okTradeMaxResetTime, account.Key, symbol)
	if v != nil && ok && vTime != nil && okTime && time.Now().Unix()-vTime.(int64) < expireSecond {
		return true, v.([]float64)[0], v.([]float64)[1]
	}
	success, maxBuy, maxSell = getMaxSizeOKEX(account, symbol)
	if success {
		util.StoreSyncMap(tradeMax, []float64{maxBuy, maxSell}, account.Key, symbol)
		util.StoreSyncMap(okTradeMaxResetTime, time.Now().Unix(), account.Key, symbol)
	}
	return success, maxBuy, maxSell
}

// RequireKLineReset
func _(environment *model.Environment, market string, symbols map[string]bool) (reset bool) {
	for symbol := range symbols {
		_, candle := environment.GetKLine(symbol, market)
		if candle == nil || candle.CreatedAt.Add(time.Duration(candle.Seconds)*time.Second).UnixMilli()+int64(model.AppConfig.Delay) <
			time.Now().UnixMilli() {
			reset = true
			if candle == nil {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`RequireKLineReset symbol %s nil candle`, symbol))
			} else {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`RequireKLineReset symbol %s candle time %s`, symbol, candle.CreatedAt.String()))
			}
			break
		}
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`RequireKLineReset %s %#v`, market, reset))
	return reset
}

func RequireConnTickReset(environment *model.Environment, market string) bool {
	needReset, ok := environment.PubChanNeedReset.Load(market)
	if ok && needReset != nil && needReset.(bool) {
		environment.PubChanNeedReset.Store(market, false)
		util.Log(util.LogLevelInfo, `clear need reset for market: `+market)
		return true
	}
	if model.AppConfig.SpecialChan == `1` && (market == model.BinancePerp || market == model.BinanceSpot || market == model.OKEX || market == model.Gate) {
		return false
	}
	initTime, _ := model.AppEnvironment.WsInitTime.Load(market)
	if initTime != nil {
		if initTime.(time.Time).Add(time.Minute * 2).After(time.Now()) {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`just reset %s no need %s`, market, initTime.(time.Time).String()))
			return false
		}
	}
	now := time.Now().UnixMilli()
	validSymbolNum := 0
	validSymbols := make(map[string]bool)
	symbols := GetMarketSymbols(market)
	for symbol := range symbols {
		if len(strings.Trim(symbol, ` `)) == 0 {
			validSymbolNum++
		}
		_, bidAsk := environment.GetBidAsk(market, symbol)
		if bidAsk == nil {
			continue
		}
		delay := float64(now - int64(bidAsk.Ts))
		if delay < model.AppConfig.Delay {
			validSymbolNum++
			validSymbols[symbol] = true
			//if market == model.BinancePerp {
			//	util.Notice(fmt.Sprintf(`RequireConnTickReset valid %d %s %s %f<%f`,
			//		validSymbolNum, market, symbol, delay, model.AppConfig.Delay))
		}
		//} else if market == model.BinancePerp {
		//	util.Notice(fmt.Sprintf(`RequireConnTickReset delay too long %s %s %f`, market, symbol, delay))
		//}
	}
	needReset = validSymbolNum*7 < 6*len(symbols) && len(symbols)-validSymbolNum > 50
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
				util.Log(util.LogLevelInfo, fmt.Sprintf(`need reset for important time out %s %s %s`,
					market, setting.Function, setting.Symbol))
				needReset = true
				return false
			}
			return true
		})
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`require conn tick reset %d  %f valid %d in %d %s needReset %#v`,
		now, model.AppConfig.Delay, validSymbolNum, len(symbols), market, needReset))
	return needReset.(bool)
}

func MustCancel(account *model.Account, market, symbol, orderType, orderId string, mustCancel bool) (res bool) {
	var lock *sync.Mutex
	lockValue, _ := mustCancelLock.Load(account.Key)
	if lockValue == nil {
		lock = &sync.Mutex{}
		mustCancelLock.Store(account.Key, lock)
	} else {
		lock = lockValue.(*sync.Mutex)
	}
	defer lock.Unlock()
	lock.Lock()
	sleepTime := 10
	for i := 0; i < 4; i++ {
		result, errCode, msg := CancelOrder(account, market, symbol, orderType, orderId)
		res = result
		util.Log(util.LogLevelInfo, fmt.Sprintf(`[cancel] %s %s %s %s for %d times, return %t code %s msg %s `,
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

func CancelAll(account *model.Account, market string) {
	switch market {
	case model.OKEX:
		cancelAllOkex(account)
	case model.Bybit:
		cancelAllBybit(account.Key, account.Secret, `linear`)
		cancelAllBybit(account.Key, account.Secret, `spot`)
	case model.BinanceSpot:
		orders := queryOpenOrdersBinanceSpot(account.Key, account.Secret, ``)
		for _, order := range orders {
			result, _ := cancelOrderBinanceSpot(account.Key, account.Secret, market, order.Symbol, order.OrderId)
			util.Log(util.LogLevelInfo, fmt.Sprintf(`cancelAll orders success BinanceSpot %s id %s return %#v`,
				order.Symbol, order.OrderId, result))
			time.Sleep(time.Millisecond * 100)
		}
	case model.BinancePerp:
		orders := queryOpenOrdersBinancePerp(account.Key, account.Secret, ``)
		for _, order := range orders {
			result := cancelOrderBinancePerp(account.Key, account.Secret, order.Symbol, order.OrderId)
			util.Log(util.LogLevelInfo, fmt.Sprintf(`cancelAll orders success BinancePerp %s id %s return %#v`,
				order.Symbol, order.OrderId, result))
			time.Sleep(time.Millisecond * 100)
		}
	case model.Gate:
		cancelAllGate(account.Key, account.Secret)
		//case model.BitgetSpot:
		//	// 只支持50个以内的order
		//	orders := deprecated.queryOpenOrdersBitgetSpot(key, secret, ``)
		//	deprecated.batchCancelBitgetSpot(key, secret, orders)
		//case model.BitgetPerp:
		//	deprecated.cancelAllBitgetPerp(key, secret)
	}
}

// CancelOrders 暂不支持策略订单
func CancelOrders(account *model.Account, market, symbol string) (result bool) {
	switch market {
	case model.Gate:
		result = cancelOrdersGate(account.Key, account.Secret, symbol)
	case model.BinanceSpot, model.BinanceMargin:
		result = cancelOrdersBinance(account.Key, account.Secret, market, symbol)
	case model.BinancePerp:
		result = cancelOrdersBinancePerp(account.Key, account.Secret, symbol)
	case model.Bybit:
		result = cancelOrdersBybit(account.Key, account.Secret, symbol)
	case model.OKEX:
		result, _, _ = cancelOrdersOKEX(account, symbol)
	}
	time.Sleep(time.Second * 2)
	util.Log(util.LogLevelInfo, fmt.Sprintf(`cancel symbol orders %s %s return %#v`, market, symbol, result))
	return result
}

// CancelOrder 支持普通订单、stop订单
func CancelOrder(account *model.Account, market, symbol, orderType, orderId string) (result bool, errCode, msg string) {
	if model.AppConfig.Env == `test` {
		return true, ``, `test cancel`
	}
	errCode = `` + market
	msg = `` + market
	switch market {
	case model.OKEX:
		result, errCode, msg = cancelOrderOkex(account, symbol, orderId, orderType)
	case model.Bybit:
		result = cancelOrderBybit(account.Key, account.Secret, symbol, orderId)
	case model.Gate:
		result = cancelOrderGate(account.Key, account.Secret, symbol, orderId)
	case model.BinancePerp:
		result = cancelOrderBinancePerp(account.Key, account.Secret, symbol, orderId)
	case model.BinanceSpot:
		result, _ = cancelOrderBinanceSpot(account.Key, account.Secret, market, symbol, orderId)
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`[cancel %s %#v %s %s]`, orderId, result, market, symbol))
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
		case model.OKEX:
			candles, isCache = getCandlesOKEX(account, symbol, begin, end, int(count), slotSeconds)
		case model.BinancePerp, model.BinanceSpot:
			candles, isCache = getCandlesBinance(account, market, symbol, begin, end, int(count), slotSeconds)
			//case model.GXZQ:
			//	candles, isCache = deprecated.getCandlesGXZQDB(symbol, begin, end, slotSeconds)
			//case model.Ftx:
			//	candles = deprecated.getCandlesFtx(account, symbol, begin, end, slotSeconds)
		}
		msg := fmt.Sprintf(`get candles %s %s %d seconds %s %d`,
			market, symbol, slotSeconds, begin.Format(time.RFC3339), len(candles))
		oldMsg, ok := util.LoadSyncMap(&model.CarryInfo, `GetCandle`)
		if ok && oldMsg != nil {
			msg = oldMsg.(string) + msg
		}
		if !isCache {
			util.Log(util.LogLevelInfo, msg)
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
			//case model.Ftx:
			//	temp = deprecated.getCandlesFtx(account, symbol, begin, end, slotSeconds)
			case model.OKEX:
				temp, isCache = getCandlesOKEX(account, symbol, begin, end, int(count), slotSeconds)
			case model.BinancePerp, model.BinanceSpot:
				temp, isCache = getCandlesBinance(account, market, symbol, begin, end, int(count), slotSeconds)
			}
			for j := 0; temp != nil && j < temp.Len(); j++ {
				candles[j*len(settings)+i] = temp[j]
			}
			i++
			if !isCache {
				time.Sleep(time.Millisecond * 100)
			} else {
				//util.Notice(fmt.Sprintf(`get candles from cache %s %s %#v %#v %d %d`,
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
func GetPriceForce(symbol, market string) (result bool, price float64) {
	getBidAsk, bidAsk := model.AppEnvironment.GetBidAsk(market, symbol)
	if getBidAsk && bidAsk != nil {
		return true, bidAsk.Bids[0].Price
	}
	//markets := GetMarkets()
	//_, _, coin, _ := model.GetFromStandard(market, symbol)
	//symbolSpot := coin + model.UniStandardTail[model.MarketTypeSpot]
	//symbolPerp := coin + model.UniStandardTail[model.MarketTypePerp]
	//for _, m := range markets {
	//	getBidAsk, bidAsk = model.AppEnvironment.GetBidAsk(m, symbolSpot)
	//	if getBidAsk && bidAsk != nil {
	//		return true, bidAsk.Bids[0].Price
	//	}
	//	getBidAsk, bidAsk = model.AppEnvironment.GetBidAsk(m, symbolPerp)
	//	if getBidAsk && bidAsk != nil {
	//		return true, bidAsk.Bids[0].Price
	//	}
	//}
	//value, okPrice := lastPrice.Load(market + `_` + symbol)
	//priceTime, okTime := lastPriceTime.Load(market + `_` + symbol)
	//if okPrice && okTime && value != nil && priceTime.(time.Time).Add(time.Minute*10).After(time.Now()) {
	//	return true, value.(float64)
	//}
	//coins := strings.Split(symbol, `_`)
	//if len(coins) == 2 && coins[0] == coins[1] {
	//	return true, 1
	//}
	//marketInfo := model.GetMarketInfo(market, symbol)
	//if marketInfo == nil {
	//	//util.Info(fmt.Sprintf(`not in market infos %s %s %s %s`, market, symbol, key, secret[0:1]))
	//	return false, 0
	//}
	//lastPriceTime.Store(market+`_`+symbol, time.Now().Add(time.Second*14400))
	//lastPrice.Store(market+`_`+symbol, price)
	//util.Log(util.LogLevelError, fmt.Sprintf(`fail to get price for %s %s`, market, symbol))
	return false, price
}

var getEquityTime = &sync.Map{}
var equityMsg = &sync.Map{}

func GetMarketEquity(index int) (msg string) {
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
	for _, market := range model.AppEnvironment.Markets {
		if accounts[market] == nil {
			continue
		}
		//util.Notice(fmt.Sprintf(`try to get value for %s account %s`, market, accounts[market].Key[:5]))
		_, _, equity, _ := GetBalances(accounts[market], market)
		if equity == 0 && !accounts[market].IsUnified {
			_, _, equity, _, _ = GetPositions(accounts[market], market)
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

// GetBalances 本方法用于搬砖过程中获取现货，由于某些交易所在有持仓的情况下可能返回现货持仓为0且接口显示正确返回，故判断所有持仓为0时为接口错误
// 如果是新账户，需要手动下单产生持仓
func GetBalances(account *model.Account, market string) (
	success bool, balances []*model.Balance, totalInUsd float64, collateral *model.Collateral) {
	lock, _ := balanceLock.Load(account.Key)
	if lock == nil {
		lock = &sync.Mutex{}
		balanceLock.Store(account.Key, lock)
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
	//case model.BitgetSpot:
	//	success, balances = deprecated.getBalanceBitgetSpot(key, secret)
	case model.Gate:
		success, balances, totalInUsd, collateral = getBalanceGate(account.Key, account.Key)
	case model.OKEX:
		success, balances, totalInUsd, collateral = getBalanceOKEX(account)
	case model.BinanceSpot:
		success, totalInUsd, balances = getBalanceBinanceSpot(account.Key, account.Secret)
	case model.BinanceMargin:
		success, balances = getBalanceBinanceMargin(account.Key, account.Secret)
	case model.Bybit:
		success, balances, totalInUsd, collateral = getBalanceBybit(account.Key, account.Secret)
	}
	accounts := model.AppConfig.GetAccounts(market)
	if len(accounts) > 0 && !accounts[0].IsUnified && totalInUsd == 0 {
		for _, balance := range balances {
			if USDs[balance.Coin] {
				totalInUsd += balance.Amount
			} else {
				symbolStandard := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
				_, price := GetPriceForce(symbolStandard, market)
				totalInUsd += price * balance.Amount
				if price == 0 && balance.Amount > 0 {
					setting := GetSetting(model.FunctionCross, market, symbolStandard)
					msg := fmt.Sprintf(`can not get usd value for %s %s %s value%f amt %f`,
						account.Key, market, symbolStandard, totalInUsd, balance.Amount)
					if setting != nil {
						msg = `with setting ` + msg
					} else {
						msg = `no setting ` + msg
					}
					util.Log(util.LogLevelInfo, msg)
				}
			}
		}
	}
	//util.Notice(fmt.Sprintf(`get balances %s %s %f %d %#v`,
	//	market, key[:5], totalInUsd, len(balances), success))
	return success, balances, totalInUsd, collateral
}

func GetTransfers(account *model.Account, market string) (balances []*model.Balance) {
	switch market {
	case model.OKEX:
		return getTransferOKEX(account)
	case model.BinanceSpot, model.BinancePerp, model.BinanceMargin:
		return GetWithdrawInfo(market, account.Key, account.Secret)
	}
	return balances
}

func GetFundingRate(account *model.Account, market, symbol string, cacheOnly bool) (success, useRest bool, rate *model.FundingRate) {
	//非永续合约的资金费率为0
	_, marketType, _, _ := model.GetFromStandard(market, symbol)
	if marketType != model.MarketTypePerp {
		return true, false, &model.FundingRate{
			Rate:       0,
			RateNext:   0,
			UpdateTime: time.Time{},
			ExpireTime: time.Now().Unix() + 86400,
		}
	}
	value, ok := util.LoadSyncMap(model.FundingRates, market, symbol)
	var fundingRate *model.FundingRate
	if ok && value != nil {
		fundingRate = value.(*model.FundingRate)
	}
	now := time.Now().Unix()
	if fundingRate != nil && now < fundingRate.ExpireTime && now-fundingRate.UpdateTime.Unix() < 300 {
		return true, false, fundingRate
	}
	//if fundingRate == nil {
	//	util.Log(util.LogLevelInfo, fmt.Sprintf(`get funding rate fail from ws nil %s %s %#v`, market, symbol, fundingRate))
	//} else {
	//	util.Log(util.LogLevelInfo, fmt.Sprintf(`get funding rate fail from ws %s %s %d %#v`, market, symbol, now-fundingRate.UpdateTime.Unix(), fundingRate))
	//}
	if cacheOnly {
		return false, false, nil
	}
	switch market {
	//case model.BitgetPerp: // bitgetPerp rest接口无发获取expire time，要么直接返回，要么使用原有的
	//	if fundingRate != nil && now-fundingRate.UpdateTime.Unix() < 300 {
	//		return true, false, fundingRate
	//	} else {
	//		newFRate := deprecated.getFundingRateBitgetPerp(symbol)
	//		if newFRate != nil {
	//			if fundingRate != nil {
	//				newFRate.ExpireTime = fundingRate.ExpireTime
	//			}
	//			fundingRate = newFRate
	//		} else {
	//			return false, true, nil
	//		}
	//	}
	case model.Bybit:
		fundingRate = getFundingRateBybit(symbol)
	case model.OKEX:
		fundingRate = getFundingRateOKEX(account, symbol)
	case model.BinancePerp:
		fundingRate = getFundingRateBinancePerp(account.Key, account.Secret, symbol)
	case model.Gate:
		fundingRate = getFundingRateGate(account.Key, account.Secret, symbol)
	}
	if fundingRate == nil || now > fundingRate.ExpireTime {
		time.Sleep(time.Minute)
		return false, true, nil
	}
	//util.Log(util.LogLevelInfo, fmt.Sprintf(`get funding rate from rest %s %s %#v`, market, symbol, fundingRate))
	time.Sleep(time.Millisecond * 80)
	SetFundingRate(market, symbol, fundingRate)
	return true, true, fundingRate
}

// GetMaxLoan
func _(account *model.Account, market, symbol string) (success bool, maxLoan float64) {
	switch market {
	case model.Gate:
		return getMaxLoanGate(symbol)
	case model.OKEX:
		return getMaxLoanOKEX(account, symbol)
	}
	return false, 0
}

func QueryOpenOrders(account *model.Account, market, symbol string) (orders []*model.Order) {
	orders = make([]*model.Order, 0)
	switch market {
	case model.OKEX:
		orders = queryOpenOrdersOKEX(account, symbol, true)
		temp := queryOpenOrdersOKEX(account, symbol, false)
		for _, order := range temp {
			orders = append(orders, order)
		}
	case model.Bybit:
		orders = queryOpenOrdersBybit(account.Key, account.Secret, symbol)
	case model.Gate:
		orders = queryOpenOrdersGate(account.Key, account.Secret, symbol)
	case model.BinancePerp:
		orders = queryOpenOrdersBinancePerp(account.Key, account.Secret, symbol)
	case model.BinanceSpot:
		orders = queryOpenOrdersBinanceSpot(account.Key, account.Secret, symbol)
	}
	return orders
}

func QueryOrderById(account *model.Account, market, symbol, orderType, orderId string) (order *model.Order) {
	switch market {
	//case model.BitgetPerp:
	//	order = deprecated.queryOrderBitgetPerp(key, secret, symbol, orderId)
	//case model.BitgetSpot:
	//	order = deprecated.queryOrderBitgetSpot(key, secret, orderId)
	case model.Gate:
		order = queryOrderGate(account.Key, account.Secret, symbol, orderId)
	case model.OKEX:
		order = queryOrderOKEX(account, symbol, orderId, orderType)
	case model.BinanceSpot:
		order = queryOrderBinanceSpot(account.Key, account.Secret, symbol, orderId)
	case model.BinancePerp:
		order = queryOrderBinancePerp(account.Key, account.Secret, symbol, orderId)
	case model.Bybit:
		order = queryOrderBybit(account.Key, account.Secret, symbol, orderId)
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`query by id %s %s %s %#v`, market, symbol, orderId, order))
	return order
}

// GetPositions 本方法用于搬砖过程中获取持仓，由于某些交易所在有持仓的情况下可能返回positions为0且接口显示正确返回，故判断所有持仓为0时为接口错误
// 如果是新账户，需要手动下单产生持仓
// accountValue: 账户权益
// availableU: 可用usd
func GetPositions(account *model.Account, market string) (success bool, positions []*model.Position, accountValue, availableU, mmr float64) {
	lock, _ := positionLock.Load(account.Key)
	if lock == nil {
		lock = &sync.Mutex{}
		positionLock.Store(account.Key, lock)
	}
	lock.(*sync.Mutex).Lock()
	defer func() {
		time.Sleep(time.Millisecond * 100)
		lock.(*sync.Mutex).Unlock()
	}()
	switch market {
	//case model.BitgetPerp:
	//	success, positions, accountValue, availableU, mmr = deprecated.getPositionsBitgetPerp(key, secret)
	case model.Gate:
		_, _, total, collateral := getBalanceGate(account.Key, account.Secret)
		success, positions = getPositionsGate(account.Key, account.Secret)
		accountValue, availableU, mmr = total, collateral.Available, collateral.Rate
	case model.BinancePerp:
		success, positions, accountValue, availableU, mmr = getPositionsBinancePerp(account.Key, account.Secret)
	case model.Bybit:
		_, _, total, collateral := getBalanceBybit(account.Key, account.Secret)
		success, positions, _ = getPositionsBybit(account.Key, account.Secret)
		accountValue, availableU, mmr = total, collateral.Available, collateral.Rate
	case model.OKEX:
		_, _, total, collateral := getBalanceOKEX(account)
		success, positions = getPositionsOKEX(account)
		accountValue, availableU, mmr = total, collateral.Available, collateral.Rate
	}
	//util.Notice(fmt.Sprintf(`get positions %s %s %f %f %d %#v`,
	//	market, key[:5], accountValue, availableU, len(positions), success))
	return success, positions, accountValue, availableU, mmr
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

func MustPlaceOrder(account *model.Account, orderSide, orderType, market, symbol, orderParam,
	refreshType string, price, triggerPrice, amount float64, useLock bool) (orders []*model.Order) {
	if useLock {
		var lock *sync.Mutex
		lockValue, _ := mustPlaceLock.Load(account.Key)
		if lockValue == nil {
			lock = &sync.Mutex{}
			mustPlaceLock.Store(account.Key, lock)
		} else {
			lock = lockValue.(*sync.Mutex)
		}
		defer lock.Unlock()
		lock.Lock()
	}
	retry := 2
	for i := 0; i < retry; i++ {
		v, _ := util.LoadSyncMap(model.MarketInfos, market, symbol)
		if v == nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to get market info when must place %s %s`, market, symbol))
			continue
		}
		sizeMax := v.(*model.MarketInfo).SizeMax
		if orderType == model.OrderTypeTrailStop || orderType == model.OrderTypeMarket {
			sizeMax = v.(*model.MarketInfo).SizeMaxMarket
		}
		if sizeMax > 0 && amount > sizeMax {
			ordersLeft := MustPlaceOrder(account, orderSide, orderType, market, symbol, orderParam, refreshType, price,
				triggerPrice, sizeMax, false)
			orders = MustPlaceOrder(account, orderSide, orderType, market, symbol, orderParam, refreshType, price,
				triggerPrice, amount-sizeMax, false)
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
			order := PlaceOrder(account, orderSide, orderType, market, symbol, orderParam, refreshType, price,
				triggerPrice, amount, false, nil)
			if order != nil && order.OrderId != `` && order.Status != model.CarryStatusFail {
				order.RefreshType = refreshType
				return []*model.Order{order}
			} else { // binance perp 下单返回 -4005 代表数量太大，分两半继续下单
				if order != nil && strings.Contains(order.ErrCode, `-4005`) {
					v.(*model.MarketInfo).SizeMax = 0.52 * amount
					util.Log(util.LogLevelError, fmt.Sprintf(`binance perp 下单返回 -4005 代表数量%f太大，减小marketInfo size max %f`,
						amount, v.(*model.MarketInfo).SizeMax))
				}
				// <APIError> code=-2027, msg=Exceeded the maximum allowable position at current leverage.
				//if order != nil && strings.Contains(order.ErrCode, `-2027`) {
				//	break
				//}
				time.Sleep(time.Second * 10)
				util.Log(util.LogLevelError, fmt.Sprintf(`fail to place order %d time, re order`, i))
			}
		}
	}
	return orders
}

// PlaceOrder orderSide: OrderSideBuy OrderSideSell OrderSideLiquidateLong OrderSideLiquidateShort
// orderType: OrderTypeLimit OrderTypeMarket
// amount:如果是限价单或市价卖单，amount是左侧币种的数量，如果是市价买单，amount是右测币种的数量
func PlaceOrder(account *model.Account, orderSide, orderType, market, symbol, orderParam, funcType string, price, triggerPrice,
	amount float64, isWs bool, postOrder model.PostOrder) (order *model.Order) {
	model.AppEnvironment.LastOrderMilli.Store(account.Key, time.Now().UnixMilli())
	markSide := model.OrderSideBuy
	switch orderSide {
	case model.OrderSideBuy, model.OrderSideLiquidateShort:
		markSide = model.OrderSideBuy
	case model.OrderSideSell, model.OrderSideLiquidateLong:
		markSide = model.OrderSideSell
	}
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	if amount == 0 {
		util.Log(util.LogLevelError, fmt.Sprintf(`can not place order with amount 0 , %s %s %s %s`, orderSide, orderType, market, symbol))
		return &model.Order{OrderSide: markSide, OrderType: orderType, Market: market, Symbol: symbol, Coin: coin, Price: price,
			Amount: 0, TriggerPrice: triggerPrice, RefreshType: funcType, Status: model.CarryStatusFail, DealAmount: 0, DealPrice: price, OrderTime: util.GetNow()}
	}
	clientOrdId := strconv.FormatInt(time.Now().UnixMicro(), 10)[3:] + orderSide[0:1]
	order = &model.Order{OrderId: clientOrdId, ClientOrdId: clientOrdId, RefreshType: funcType,
		OrderSide: markSide, OrderType: orderType, Market: market, Symbol: symbol, Price: price, Amount: amount, DealAmount: 0, Coin: coin,
		DealPrice: price, TriggerPrice: triggerPrice, OrderTime: util.GetNow(), UnfilledQuantity: amount, AccountIndex: account.Index, Status: model.CarryStatusWorking}
	//util.Notice(fmt.Sprintf(`...%s %s %s before order %d amount: %f price:%f triggerPrice:%f`,
	//	orderSide, market, symbol, start, amount, price, triggerPrice))
	if model.AppConfig.Env == `test` {
		return
	}
	if market == model.BitgetPerp || market == model.BitgetSpot {
		isWs = false
	}
	switch market {
	//case model.BitgetPerp:
	//	deprecated.placeOrderBitgetPerp(account, isWs, order, orderSide, orderType, orderParam, symbol, price, amount)
	//case model.BitgetSpot:
	//	deprecated.placeOrderBitgetSpot(account, isWs, order, orderSide, orderType, symbol, price, amount)
	case model.Gate:
		placeOrderGate(account, isWs, order, orderSide, orderType, orderParam, symbol, price, amount)
	case model.OKEX:
		placeOrderOKEX(account, isWs, order, orderParam)
	case model.BinanceSpot:
		placeOrderBinanceSpot(account, isWs, order, orderSide, orderType, symbol, price, amount)
	case model.BinancePerp:
		placeOrderBinancePerp(account, isWs, order, orderSide, orderType, orderParam, symbol, price, triggerPrice, amount)
	case model.Bybit:
		placeOrderBybit(account, isWs, order, orderParam)
	}
	if isWs {
		model.AppEnvironment.ReqIdOrders.Store(order.ClientOrdId, order)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`store order %s %s %s %s %s %#v`, market, coin, symbol, orderSide, order.ClientOrdId, order))
	}
	if !isWs || order.Status != model.CarryStatusWorking {
		if order.OrderId == "0" || strings.Trim(order.OrderId, ` `) == "" {
			order.Status = model.CarryStatusFail
			order.OrderId = fmt.Sprintf(`%s_error_%d`, order.ErrCode, time.Now().UnixNano())
		} else if order.Status == `` {
			order.Status = model.CarryStatusWorking
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(
			`...%s %s %s return order at %s %s price %f %f amount %f %f trigger %f %f id %s`,
			orderSide, market, symbol, order.Status, order.ErrCode, price, order.Price, amount, order.Amount,
			triggerPrice, order.TriggerPrice, order.OrderId))
		if postOrder != nil {
			if order.Status != model.CarryStatusFail {
				model.AppEnvironment.OrderIdOrders.Store(order.OrderId, order)
			}
			go postOrder(order)
		}
	}
	model.AppDB.Save(order)
	return order
}

var filterCoins = map[string]bool{`REEF`: true}

// FilterCross
// 由于gate下线暂时不平仓的币 BCD OXY CRU
// 搬砖过滤币种 AMPL IOTA REEF MIR SOS
// 某些主流币 BTC ETH LINK
// 币种对不上 REAL, DFL, QI, WSB, TRADE,FAME,BIFI,TON,BOX,PAY,BTT
// 法币 GBP CUSDT TRYB BRZ CAD EUR SUSD USDC TUSD`USDT EURT USD BUSD LDBUSD LDUSDT
// ftx预测TRUMP BOLSONARO
func FilterCross(market, symbol string) bool {
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	if filterCoins[strings.ToUpper(coin)] {
		return true
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
	case model.BinanceSpot:
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

//const topMarketInfoLenCross = 30

// InitCrossMarketInfos 用以初始化cross carry的各个币种市场，调用前需要truncate settings数据库表，本方法会从新插入
func InitCrossMarketInfos(markets []string) {
	infoPool := make(map[string][]*model.MarketInfo) // coin - []marketInfos
	util.Log(util.LogLevelInfo, fmt.Sprintf(`begin to init cross market infos %#v`, markets))
	model.MarketInfos.Clear()
	for _, market := range markets {
		InitMarketInfos(market)
	}
	model.MarketInfos.Range(func(key, value any) bool {
		if value == nil {
			return true
		}
		success, _, coin, _ := model.GetFromStandard(value.(*model.MarketInfo).Market, value.(*model.MarketInfo).Symbol)
		if success && coin != `` {
			if infoPool[coin] == nil {
				infoPool[coin] = make([]*model.MarketInfo, 0)
			}
			if !FilterCross(value.(*model.MarketInfo).Market, value.(*model.MarketInfo).Symbol) {
				infoPool[coin] = append(infoPool[coin], value.(*model.MarketInfo))
			}
		}
		return true
	})
	var settingsDb []*model.Setting
	model.AppDB.Find(&settingsDb)
	settingsDbMap := make(map[string]*model.Setting)
	model.AppDB.Model(&settingsDb).Where(`function=?`, model.FunctionCross).Updates(map[string]interface{}{`liquidated`: true})
	for _, setting := range settingsDb {
		info, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
		if info == nil || info.(*model.MarketInfo).DeListing {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`update to not liquidated %s %s`, setting.Market, setting.Symbol))
			if setting.Chance >= 0 {
				setting.Chance = -1
			}
			model.AppDB.Model(&settingsDb).Where(`function=? and market=? and symbol=?`, model.FunctionCross, setting.Market, setting.Symbol).
				Updates(map[string]interface{}{`liquidated`: false})
		}
		settingsDbMap[fmt.Sprintf(`%s_%s_%s`, setting.Function, setting.Market, setting.Symbol)] = setting
	}
	for coin, infos := range infoPool {
		//util.Log(util.LogLevelInfo, fmt.Sprintf(`handle coin %s %d`, coin, len(infos)))
		scoreOpen := 0.015
		scoreClose := 0.005
		if len(infos) >= 2 {
			havePerp := false
			for _, info := range infos {
				_, marketType, _, _ := model.GetFromStandard(info.Market, info.Symbol)
				if marketType == model.MarketTypePerp {
					havePerp = true
					break
				}
			}
			if !havePerp {
				continue
			}
			for _, info := range infos {
				if info.DeListing {
					continue
				}
				if settingsDbMap[fmt.Sprintf(`%s_%s_%s`, model.FunctionCross, info.Market, info.Symbol)] == nil {
					setting := &model.Setting{
						Valid:             true,
						Liquidated:        true,
						Function:          model.FunctionCross,
						WSType:            model.WSTypeTicker,
						Market:            info.Market,
						Symbol:            info.Symbol,
						Coin:              coin,
						OpenShortMargin:   scoreOpen,
						CloseShortMargin:  scoreClose,
						AmountRate:        10.0,
						AmountRateCombine: 1.5,
						PriceX:            1, GridAmount: 1}
					util.Log(util.LogLevelInfo, fmt.Sprintf(`save setting %s %s %s %#v`, info.Market, info.Symbol, coin, setting.Valid))
					model.AppDB.Save(setting)
					accounts := model.AppConfig.GetAccounts(info.Market)
					for _, account := range accounts {
						suc := SetSymbolLeverage(account, info.Market, info.Symbol)
						util.Log(util.LogLevelInfo, fmt.Sprintf(`add new gate setting index %d %s set leverage %f %f %f %v`,
							account.Index, info.Symbol, account.GateLeverMax, account.GateLeverMin, account.GateRiskLimit, suc))
					}
				}
			}
		} else if len(infos) == 1 && infos[0].Symbol[0:2] == `10` && !infos[0].DeListing {
			if settingsDbMap[fmt.Sprintf(`%s_%s_%s`, model.FunctionCross, infos[0].Market, infos[0].Symbol)] == nil {
				setting := &model.Setting{
					Valid:             true,
					Liquidated:        true,
					Function:          model.FunctionCross,
					WSType:            model.WSTypeTicker,
					Market:            infos[0].Market,
					Symbol:            infos[0].Symbol,
					Coin:              coin,
					OpenShortMargin:   scoreOpen,
					CloseShortMargin:  scoreClose,
					AmountRate:        10.0,
					AmountRateCombine: 1.5,
					PriceX:            1, GridAmount: 1}
				util.Log(util.LogLevelInfo, fmt.Sprintf(`save setting %s %s %s %#v`, infos[0].Market, infos[0].Symbol, coin, setting.Valid))
				model.AppDB.Save(setting)
			}
		}
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`end to init cross market infos`))
}

var marketInfoInitializing = false

// InitMarketInfos 只支持现货SPOT和永续PERP SWAP
func InitMarketInfos(market string) (success bool) {
	if marketInfoInitializing {
		return
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`begin to init market infos %s`, market))
	marketInfoInitializing = true
	defer func() {
		marketInfoInitializing = false
	}()
	success = true
	accounts := model.AppConfig.GetAccounts(market)
	var marketInfos map[string]*model.MarketInfo
	switch market {
	//case model.Mexc:
	//	marketInfos = deprecated.getMarketsMexc(accounts[0].Key, accounts[0].Secret)
	//case model.Ftx:
	//	marketInfos = deprecated.getMarketsFtx(accounts[0].Key, accounts[0].Secret)
	case model.OKEX:
		marketInfos = getMarketsOKEX(accounts[0])
	//case model.HuobiSpot:
	//	marketInfos = deprecated.getMarketsHuobiSpot(accounts[0].Key, accounts[0].Secret)
	case model.BinanceSpot:
		marketInfos = GetMarketsBinance(accounts[0], market)
	case model.BinancePerp:
		marketInfos = getMarketsBinancePerp(accounts[0].Key, accounts[0].Secret)
	case model.Gate:
		_, marketInfos = getMarketsGate(accounts[0].Key, accounts[0].Secret)
	//case model.KucoinSpot:
	//	marketInfos = deprecated.getMarketsKucoinSpot(accounts[0].Key)
	//case model.KucoinPerp:
	//	marketInfos = deprecated.getMarketsKucoinPerp(accounts[0].Key)
	//	deprecated.setFutureAutoDeposit()
	case model.Bybit:
		marketInfos = getMarketsBybit()
		//case model.BitgetSpot:
		//	marketInfos = deprecated.getMarketsBitgetSpot()
		//case model.BitgetPerp:
		//	marketInfos = deprecated.getMarketsBitgetPerp()
	}
	for _, setting := range model.AppEnvironment.Settings {
		if setting.Market == market && marketInfos[setting.Symbol] == nil && strings.Trim(setting.Symbol, ` `) != `` {
			setting.Valid = false
			util.Log(util.LogLevelInfo, fmt.Sprintf(`warning %s %s un-list from market`, market, setting.Symbol))
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

func SendMails(title, msg string) {
	util.Log(util.LogLevelInfo, fmt.Sprintf(`try to send mails %s %s`, title, msg))
	toMails := strings.Split(model.AppConfig.Mail, `,`)
	for _, mail := range toMails {
		if len(strings.TrimSpace(mail)) == 0 {
			continue
		}
		err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, mail, title, msg)
		if err != nil {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`fail to send mail title %s msg %s to %s err %s`, title, msg, mail, err.Error()))
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
	//case model.BitgetPerp:
	//	return deprecated.setSymbolLeverageBitgetPerp(account, symbol)
	case model.Gate:
		return setSymbolLeverageGate(account, symbol)
	}
	return false
}

func SetFundingRate(market, symbol string, fundingRate *model.FundingRate) {
	if fundingRate == nil {
		return
	}
	if fundingRate.ExpireTime == 0 {
		if market == model.BitgetPerp || market == model.Gate {
			value, _ := util.LoadSyncMap(model.FundingRates, market, symbol)
			if value != nil {
				fundingRate.ExpireTime = value.(*model.FundingRate).ExpireTime
			}
		}
	}
	util.StoreSyncMap(model.FundingRates, fundingRate, market, symbol)
}

func GetBills(account *model.Account, begin, end int64) (success bool, fundingFees []*model.FundingFee) {
	switch account.Market {
	case model.OKEX:
		_, fundingFees = getBillsOkx(account, begin, end, `8`)
		if fundingFees == nil {
			fundingFees = []*model.FundingFee{}
		}
		_, interestFees := getBillsOkx(account, begin, end, `7`)
		for _, fee := range interestFees {
			fundingFees = append(fundingFees, fee)
		}
	case model.Gate:
		return getBillsGate(account, begin, end)
	case model.Bybit:
		return getBillsBybit(account, begin, end)
	case model.BinancePerp:
		return getBillsBinance(account, begin, end)
	default:
		util.Log(util.LogLevelInfo, fmt.Sprintf(`un-support market %s`, account.Market))
	}
	return false, fundingFees
}

func GetInterest(account *model.Account) (success bool) {
	switch account.Market {
	case model.OKEX:
		getInterestOkx(account)
	case model.Bybit:
		getInterestBybit(account)
	case model.Gate:
		getInterestGate(account)
	}
	return false
}

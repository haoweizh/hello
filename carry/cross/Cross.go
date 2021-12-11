package cross

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"time"
)

// todo 1. postOrderCarry
const loseRateMax = -0.005
const winRateMin = 0.005
const jump = 7.0

//const openMaxInU = 10000.0

func checkSetCrossing(value bool) (before bool) {
	crossLock.Lock()
	defer crossLock.Unlock()
	if value && crossing {
		return crossing
	} else {
		temp := crossing
		crossing = value
		return temp
	}
}

func createFromPosition(key, secret string, setting *model.Setting, price float64) (carryStatus *CarryStatus) {
	if positions[key] == nil {
		positions[key] = make(map[string][]*model.Position)
	}
	if positions[key][setting.Market] == nil {
		success, apiPositions, posBalance := api.GetPositions(key, secret, setting.Market)
		if success {
			positions[key][setting.Market] = apiPositions
			perpMarginUsd[key][setting.Market] = posBalance
		}
	}
	for _, position := range positions[key][setting.Market] {
		if position == nil || position.Currency != setting.Symbol {
			continue
		}
		carryStatus = &CarryStatus{Market: setting.Market, Symbol: setting.Symbol, LimitSell: math.NaN(),
			LimitBuy: math.NaN(), Holding: position.Free, Type: model.SymbolTypePerp}
		if perpHoldInU[key] == nil {
			perpHoldInU[key] = make(map[string]float64)
		}
		perpHoldInU[key][setting.Market] += math.Abs(carryStatus.Holding) * price
	}
	if carryStatus == nil {
		carryStatus = &CarryStatus{Key: key, Secret: secret, Market: setting.Market, Symbol: setting.Symbol,
			LimitSell: math.NaN(), LimitBuy: math.NaN(), Holding: 0, Type: model.SymbolTypePerp}
	}
	return carryStatus
}

func createFromBalance(key, secret string, setting *model.Setting) (carryStatus *CarryStatus) {
	if balances[key] == nil {
		balances[key] = make(map[string][]*model.Balance)
	}
	if usdAmount[key] == nil {
		usdAmount[key] = make(map[string]float64)
	}
	if balances[key][setting.Market] == nil {
		success, apiBalances, totalInUsd, collateral := api.GetBalances(key, secret, setting.Market)
		if success && totalInUsd > 0 {
			setBalance(key, setting.Market, apiBalances)
			setSpotBalance(key, setting.Market, totalInUsd)
		}
		if collateral != nil {
			setCollateral(key, collateral)
		}
	}
	for _, balance := range balances[key][setting.Market] {
		if balance == nil || strings.ToUpper(balance.Coin) != setting.Coin {
			continue
		}
		if (strings.ToUpper(balance.Coin) == `USDT` && setting.Market != model.Ftx) ||
			(strings.ToUpper(balance.Coin) == `USD` && setting.Market == model.Ftx) {
			usdAmount[key][setting.Market] += balance.Amount
		}
		// 可用usd数量需要减去现有所有借币负债总额
		if balance.UsdValue < 0 {
			usdAmount[key][setting.Market] -= balance.UsdValue
		}
		carryStatus = &CarryStatus{Market: setting.Market, Symbol: setting.Symbol, LimitSell: balance.AvailableWithBorrow,
			LimitBuy: math.NaN(), Holding: balance.Amount, ValueInUsd: balance.UsdValue, Type: model.SymbolTypeSpot,
			RateInAll: balance.UsdValue / spotBalance[key][setting.Market]}
	}
	if carryStatus == nil {
		carryStatus = &CarryStatus{Key: key, Secret: secret, Market: setting.Market, Symbol: setting.Symbol, LimitSell: 0,
			LimitBuy: math.NaN(), Holding: 0, ValueInUsd: 0, RateInAll: 0, Type: model.SymbolTypeSpot}
	}
	return carryStatus
}

func initStatus(key, secret string, setting *model.Setting) {
	if setting == nil {
		return
	}
	tailSpot := model.GetSpotTail(setting.Market)
	tailPerp := model.GetPerpTail(setting.Market)
	var carryStatus *CarryStatus
	fundingRate := 0.0
	getTick, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	if !getTick || tick == nil {
		return
	}
	if setting.Symbol[len(setting.Symbol)-len(tailSpot):] == tailSpot {
		carryStatus = createFromBalance(key, secret, setting)
	} else if setting.Symbol[len(setting.Symbol)-len(tailPerp):] == tailPerp {
		carryStatus = createFromPosition(key, secret, setting, tick.Bids[0].Price)
		_, fundingRate = api.GetFundingRate(setting.Market, setting.Symbol, nil)
		fundingRate *= 0.9
	}
	if carryStatus == nil {
		return
	}
	switch setting.Market {
	case model.OKEX, model.Ftx:
		carryStatus.IsUniAccount = true
	default:
		carryStatus.IsUniAccount = false
	}
	status[setting.Coin][setting.Market][setting.Symbol][key] = carryStatus
	if carryStatus.Type == model.SymbolTypePerp {
		carryStatus.ValueInUsd = carryStatus.Holding * tick.Bids[0].Price
		if carryStatus.IsUniAccount && spotBalance[key][setting.Market] > 0 {
			carryStatus.RateInAll = carryStatus.ValueInUsd / spotBalance[key][setting.Market]
		} else if !carryStatus.IsUniAccount && perpMarginUsd[key][setting.Market] > 0 {
			carryStatus.RateInAll = carryStatus.ValueInUsd / perpMarginUsd[key][setting.Market]
		}
	}
	now := time.Now().Unix()
	resetTime := getOKTradeMaxResetTime(key, setting.Symbol) + 600
	if setting.Market == model.OKEX && now > resetTime {
		setOKTradeMaxResetTime(key, setting.Symbol)
		getMax, maxBuy, maxSell := api.GetMaxSize(key, secret, setting.Symbol)
		if getMax {
			carryStatus.LimitSell = maxSell
			carryStatus.LimitBuy = maxBuy
		}
	}
	if carryStatus.RateInAll > 0 {
		carryStatus.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*carryStatus.RateInAll), winRateMin) + fundingRate
		carryStatus.TradeLineSell = math.Max(setting.OpenShortMargin*(0.5-jump*carryStatus.RateInAll), loseRateMax) - fundingRate
	} else {
		carryStatus.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*carryStatus.RateInAll), loseRateMax) + fundingRate
		carryStatus.TradeLineSell = math.Max(setting.OpenShortMargin*(0.5-jump*carryStatus.RateInAll), winRateMin) - fundingRate
	}
	if carryStatus.RateInAll > 0.5 {
		carryStatus.TradeLineBuy = 0
	}
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	doReverts := strings.Split(model.AppConfig.CarryClose, `,`)
	accountRates := strings.Split(model.AppConfig.AccountRate, `,`)
	for i := 1; i < len(keys); i++ {
		if keys[i] == key {
			rate, _ := strconv.ParseFloat(accountRates[i], 64)
			carryStatus.TradeLineBuy *= rate
			carryStatus.TradeLineSell *= rate
			if setting.Market == model.OKEX {
				collateral := GetCollateral(key)
				if collateral != nil && collateral.Available > 0 && (collateral.Available-collateral.Occupied)/collateral.Available < 0.1 {
					doReverts[i] = `true`
				}
			}
			localUsdAmount := getUsdAmount(key, setting.Market)
			localSpotBalance := getSpotBalance(key, setting.Market)
			if carryStatus.Type == model.SymbolTypeSpot && (localSpotBalance == 0 || localUsdAmount/localSpotBalance < 0.2) {
				doReverts[i] = `true`
			}
			if carryStatus.Type == model.SymbolTypePerp && perpHoldInU[key] != nil {
				if !carryStatus.IsUniAccount && (getPerpMarginUsd(key, setting.Market) == 0 ||
					perpHoldInU[key][setting.Market]/getPerpMarginUsd(key, setting.Market) < 0.2) {
					doReverts[i] = `true`
				}
				if carryStatus.IsUniAccount && (spotBalance[key][setting.Market] == 0 ||
					perpHoldInU[key][setting.Market]/spotBalance[key][setting.Market] < 0.2) {
					doReverts[i] = `true`
				}
			}
			if doReverts[i] == `true` && carryStatus.Holding > 0 {
				carryStatus.TradeLineBuy = 1
			} else if doReverts[i] == `true` && carryStatus.Holding < 0 {
				carryStatus.TradeLineSell = 1
			}
		}
	}
}

func ClearCross() {
	timer := time.NewTimer(time.Second)
	for {
		<-timer.C
		for true {
			if !checkSetCrossing(true) {
				break
			} else {
				time.Sleep(time.Millisecond * 10)
			}
		}
		if status == nil {
			status = make(map[string]map[string]map[string]map[string]*CarryStatus)
		}
		balances = make(map[string]map[string][]*model.Balance)
		usdAmount = make(map[string]map[string]float64)
		positions = make(map[string]map[string][]*model.Position)
		perpHoldInU = make(map[string]map[string]float64)
		coinSettings := model.GetCoinSettings(model.FunctionCross)
		for coin, settings := range coinSettings {
			if status[coin] == nil {
				status[coin] = make(map[string]map[string]map[string]*CarryStatus)
			}
			for _, setting := range settings {
				if setting == nil {
					continue
				}
				if status[coin][setting.Market] == nil {
					status[coin][setting.Market] = make(map[string]map[string]*CarryStatus)
				}
				if status[coin][setting.Market][setting.Symbol] == nil {
					status[coin][setting.Market][setting.Symbol] = make(map[string]*CarryStatus)
				}
				keys, secrets := model.AppConfig.GetKeys(setting.Market)
				for i, key := range keys {
					initStatus(key, secrets[i], setting)
				}
			}
		}
		for coin, settings := range coinSettings {
			//makeEqual(settings, status[coin])
			makeEqual()
			fmt.Println(fmt.Sprintf(`%s %d`, coin, len(settings)))
		}
		timer.Reset(time.Second * 60)
	}
}

// settings []*model.Setting, coinStatus map[string]map[string]map[string]*CarryStatus
func makeEqual() {
	//var holdings []float64
	//for _, setting := range settings {
	//	keys, _ := model.AppConfig.GetKeys(setting.Market)
	//	if holdings == nil {
	//		holdings = make([]float64, len(keys))
	//	}
	//	for i, key := range keys {
	//		if coinStatus[setting.Market] != nil && coinStatus[setting.Market][setting.Symbol] != nil ||
	//			coinStatus[setting.Market][setting.Symbol][key] == nil {
	//			util.Notice(`fail to get status makeEqual %s %s %s`,
	//				setting.Market, setting.Symbol, key)
	//			continue
	//		}
	//		holdings[i] += coinStatus[setting.Market][setting.Symbol][key].Holding
	//	}
	//}
	//var price float64
	//var settingEqual *model.Setting
	//orderSide := ``
	//for i, holding := range holdings {
	//	for _, setting := range settings {
	//		keys, secrets := model.AppConfig.GetKeys(setting.Market)
	//		tickGet, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	//		if coinStatus[setting.Market] != nil && coinStatus[setting.Market][setting.Symbol] != nil ||
	//			coinStatus[setting.Market][setting.Symbol][keys[i]] == nil || !tickGet {
	//			util.Notice(`fail to get status makeEqual %s %s %s`,
	//				setting.Market, setting.Symbol, keys[i])
	//			continue
	//		}
	//		carryStatus := coinStatus[setting.Market][setting.Symbol][keys[i]]
	//		if holding*tick.Bids[0].Price > 10 {
	//			orderSide = model.OrderSideSell
	//			if (math.IsNaN(carryStatus.LimitBuy) || carryStatus.LimitBuy > math.Abs(holding)) &&
	//				tick.Bids[0].Price > price {
	//				price = tick.Bids[0].Price
	//				settingEqual = setting
	//			}
	//			go api.CancelOrders(keys[i], secrets[i], setting.Market, setting.Symbol)
	//		}
	//		if holding*tick.Asks[0].Price < -10 {
	//			orderSide = model.OrderSideBuy
	//			if (math.IsNaN(carryStatus.LimitSell) || carryStatus.LimitSell > math.Abs(holding)) &&
	//				(tick.Asks[0].Price < price || price == 0) {
	//				price = tick.Asks[0].Price
	//				settingEqual = setting
	//			}
	//			go api.CancelOrders(keys[i], secrets[i], setting.Market, setting.Symbol)
	//		}
	//	}
	//	if price > 0 && settingEqual != nil {
	//		amount := math.Min(90000000, math.Min(math.Abs(holding), 20000/price))
	//		amount = model.GetAmountInMarket(settingEqual.Market, settingEqual.Symbol, amount)
	//		if amount > 0 {
	//			keys, secrets := model.AppConfig.GetKeys(settingEqual.Market)
	//			api.PlaceOrder(keys[i], secrets[i], orderSide, model.OrderTypeLimit, settingEqual.Market,
	//				settingEqual.Symbol, settingEqual.Symbol, ``, model.FunctionComplement, price, price,
	//				amount, true, false, nil, nil)
	//		}
	//	}
	//}
}

// ProcessCross todo 计算fundingRate后30s不下单
var ProcessCross = func(setting *model.Setting, tick *model.BidAsk) {
	if !doCross && model.AppConfig.Handle == `1` {
		go ClearCross()
		doCross = true
		return
	}
	million := util.GetNowUnixMillion()
	delayTick := int64(0)
	if tick != nil {
		delayTick = million - int64(tick.Ts)
	}
	settings := model.GetCoinSetting(setting.Function, setting.Coin)
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || model.AppPause ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) || status[setting.Coin] == nil ||
		status[setting.Coin][setting.Market] == nil || status[setting.Coin][setting.Market][setting.Symbol] == nil ||
		settings == nil || len(settings) == 0 || model.IsTickTimeout(setting.Market, delayTick) {
		return
	}
	for _, settingRelate := range settings {
		tickGet, tickRelate := model.AppMarkets.GetBidAsk(settingRelate.Symbol, settingRelate.Market)
		if !tickGet || model.IsRelatedTickTimeout(settingRelate.Market, million-int64(tickRelate.Ts)) {
			continue
		}
		keysRelate, _ := model.AppConfig.GetKeys(settingRelate.Market)
		for i, keyRelated := range keysRelate {
			if status[settingRelate.Coin] == nil || status[settingRelate.Coin][settingRelate.Market] == nil ||
				status[settingRelate.Coin][settingRelate.Market][settingRelate.Symbol] != nil ||
				status[settingRelate.Coin][settingRelate.Market][settingRelate.Symbol][keyRelated] == nil || !tickGet {
				util.Notice(`fail to get status makeEqual %s %s %s`,
					settingRelate.Market, settingRelate.Symbol, keyRelated)
				continue
			}
			statusCross := status[setting.Coin][setting.Market][setting.Symbol][keys[i]]
			statusRelate := status[settingRelate.Coin][settingRelate.Market][setting.Symbol][keyRelated]
			if statusCross == nil || statusRelate == nil {
				continue
			}
			statusBuy, statusSell, amount, priceBuy, priceSell :=
				calcAmount(statusCross, statusRelate, tick, tickRelate)
			if amount > 0 {
				go placeCross(statusBuy, statusSell, priceBuy, priceSell, amount)
				return
			}
		}
	}
}

func calcAmount(carryStatus, carryStatusRelate *CarryStatus, tick,
	tickRelate *model.BidAsk) (statusBuy, statusSell *CarryStatus, amount, priceBuy, priceSell float64) {
	var bidAmount, askAmount float64
	scoreOpen := 1 - tickRelate.Asks[0].Price/tick.Bids[0].Price
	scoreClose := tickRelate.Bids[0].Price/tick.Asks[0].Price - 1
	mark := fmt.Sprintf(`%s-%s<->%s-%s`, carryStatus.Market, carryStatus.Symbol, carryStatusRelate.Market, carryStatusRelate.Symbol)
	model.AppMetric.AddCarry(mark, scoreOpen, scoreClose)
	if carryStatus.TradeLineSell < scoreOpen && carryStatusRelate.TradeLineBuy < scoreOpen {
		util.Notice(`cross trade `)
		statusBuy = carryStatusRelate
		statusSell = carryStatus
		priceSell = tick.Bids[0].Price
		priceBuy = tickRelate.Asks[0].Price
		askAmount = tick.Bids[0].Amount
		bidAmount = tickRelate.Asks[0].Amount
	}
	if carryStatus.TradeLineBuy < scoreClose && carryStatusRelate.TradeLineSell < scoreClose {
		util.Notice(`cross trade `)
		statusSell = carryStatusRelate
		statusBuy = carryStatus
		priceSell = tickRelate.Bids[0].Price
		priceBuy = tick.Asks[0].Price
		askAmount = tickRelate.Bids[0].Amount
		bidAmount = tick.Bids[0].Amount
	}
	// todo test all markets real amount
	_, bidAmount = model.ParseRealAmount(statusBuy.Market, statusBuy.Symbol, bidAmount)
	_, askAmount = model.ParseRealAmount(statusSell.Market, statusSell.Symbol, askAmount)
	if !math.IsNaN(statusSell.LimitSell) {
		askAmount = math.Min(statusSell.LimitSell, askAmount)
	}
	if !math.IsNaN(statusBuy.LimitBuy) {
		bidAmount = math.Min(statusBuy.LimitBuy, bidAmount)
	}
	if statusBuy.Type == model.SymbolTypeSpot || carryStatusRelate.IsUniAccount {
		bidAmount = math.Min(getUsdAmount(statusBuy.Key, carryStatusRelate.Market)/priceBuy/5, bidAmount)
	} else if statusBuy.Type == model.SymbolTypePerp {
		bidAmount = math.Min(getPerpMarginUsd(statusBuy.Key, carryStatusRelate.Market)/priceBuy/5, bidAmount)
	}
	// todo binance 要求下单金额大于10u，gate要求大于1u
	//openValueMin := setting.AmountLimit
	//if setting.Market == model.OKEX || setting.Market == model.Binance {
	//	now := time.Now()
	//	if now.Hour()%8 == 0 && now.Minute() == 0 && now.Second() < 30 {
	//		return
	//	}
	//	fundingRateSuccess, fundingRate := api.GetFundingRate(setting.Market, setting.Symbol, nil)
	//	if !fundingRateSuccess {
	//		return
	//	}
	//	fundingRate *= 0.9
	//}
	amount = math.Min(bidAmount, askAmount)
	//balanceBuy := getBalance(key, statusBuy.Market, coin)
	//balanceSell := getBalance(key, statusSell.Market, coin)
	//balanceUSD := getBalance(key, statusBuy.Market, `usd`)
	//usdLowLine := 0.1 * balanceBuy
	//// usd所剩太少且还要再买 || 反向持仓太多且还要再卖 || 下单太小
	//if (statusBuy.Type == model.SymbolTypeSpot && stat) || statusBuy.ValueInUsd
	////if statusBuy.RateInAll > 0.5 || statusBuy.ValueInUsd < statusBuy.lo|| statusSell.RateInAll (balance.UsdValue < 0 && coinRate > 0.5)) ||
	//	math.Abs(amount)*priceBuy < openValueMin {
	//	amount = 0
	//}
	//amount = model.FormatAmountPair(setting.Market, setting.Symbol, setting.SymbolRelated, amount)
	//if model.OKEX == setting.Market && amount > 0 {
	//	amountInPerp := model.GetAmountInMarket(setting.Market, setting.Symbol, amount)
	//	maxBuyPerp, maxSellPerp := getTradeMax(key, setting.Symbol)
	//	maxBuyRelated, maxSellRelated := getTradeMax(key, setting.SymbolRelated)
	//	maxBuyRelated += balance.Borrow
	//	maxSellRelated = math.Max(maxSellRelated, balance.AvailableWithBorrow)
	//	if sidePerp == model.OrderSideBuy && sideRelated == model.OrderSideSell {
	//		amountInPerp = math.Min(amountInPerp, maxBuyPerp)
	//		amount = math.Min(amount, maxSellRelated)
	//	} else if sidePerp == model.OrderSideSell && sideRelated == model.OrderSideBuy {
	//		amountInPerp = math.Min(amountInPerp, maxSellPerp)
	//		amount = math.Min(amount, maxBuyRelated)
	//	}
	//	_, amountInReal := model.ParseRealAmount(setting.Market, setting.Symbol, amountInPerp)
	//	amount = math.Min(amount, amountInReal)
	//	amount = model.FormatAmountPair(setting.Market, setting.Symbol, setting.SymbolRelated, amount)
	//} else if model.Ftx == setting.Market && amount > 90000000 {
	//	amount = 90000000
	//}
	fmt.Println(priceSell)
	return statusBuy, statusSell, amount, priceBuy, priceSell
}

func placeCross(statusBuy, statusSell *CarryStatus, priceBuy, priceSell, amount float64) {
	if !checkSetCrossing(true) {
		defer checkSetCrossing(false)
	} else {
		//util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
		return
	}
	//placeSuccess := true
	//if setting.Market == model.OKEX {
	//	placeSuccess = api.PlacePairOKEX(key, model.GetCoin(setting.Market, setting.Symbol), sidePerp, sideRelated,
	//		model.OrderTypeLimit, perpPrice, relatedPrice, amount)
	//} else {
	//	go api.PlaceOrder(key, secret, sidePerp, model.OrderTypeLimit, setting.Market, setting.Symbol,
	//		``, ``, model.FunctionCarry, perpPrice, perpPrice,
	//		amount, true, true, postOrderCarry)
	//	api.PlaceOrder(key, secret, sideRelated, model.OrderTypeLimit, setting.Market, setting.SymbolRelated,
	//		``, ``, model.FunctionCarry, relatedPrice, relatedPrice,
	//		amount, true, true, postOrderCarry)
	//	time.Sleep(time.Second / 5)
	//}
	//if placeSuccess {
	//	usdAvailable := getUsdAvailable(key)
	//	balanceAllValue := getBalanceAll(key)
	//	if sidePerp == model.OrderSideSell {
	//		perpPrice = tickPerp.Bids[0].Price
	//		relatedPrice = tickRelated.Asks[0].Price
	//		setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)+amount)
	//		balance.Amount += amount
	//		balance.AvailableWithBorrow += amount
	//		balance.UsdValue += amount * perpPrice
	//		if carryType == carryTypeOpen {
	//			usdAvailable -= amount * perpPrice
	//			setUsdAvailable(key, usdAvailable)
	//		}
	//	} else if sidePerp == model.OrderSideBuy {
	//		perpPrice = tickPerp.Asks[0].Price
	//		relatedPrice = tickRelated.Bids[0].Price
	//		setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)-amount)
	//		balance.Amount -= amount
	//		balance.AvailableWithBorrow -= amount
	//		balance.UsdValue -= amount * perpPrice
	//		if carryType == carryTypeRevert {
	//			usdAvailable += amount * relatedPrice
	//			setUsdAvailable(key, usdAvailable)
	//		}
	//	}
	//	setCarryBalance(key, coin, balance)
	//	setUsdRate(key, usdAvailable/balanceAllValue)
	//}
}

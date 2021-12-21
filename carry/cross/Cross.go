package cross

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
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

func createContractMarket(key, secret, market string) (cm *contractMarket) {
	cm = &contractMarket{key: key, market: market}
	success, positions, posBalance := api.GetPositions(key, secret, market)
	if success {
		cm.positions = make(map[string]*model.Position)
		for _, position := range positions {
			cm.positions[position.Currency] = position
			// 只考虑USD(T)交易
			cm.contractValueInU = position.EntryPrice * math.Abs(position.Free)
		}
		cm.collateralsInU = posBalance
	}
	setContractMarket(key, market, cm)
	return
}

func createSpotMarket(key, secret, market string) (sm *spotMarket) {
	sm = &spotMarket{key: key, market: market}
	success, balances, totalInUsd, collateral := api.GetBalances(key, secret, market)
	if success {
		tail := model.GetSpotTail(market)
		sm.balances = make(map[string]*model.Balance)
		sm.accountValueInU = totalInUsd
		sm.collateral = collateral
		for _, balance := range balances {
			sm.balances[balance.Coin+tail] = balance
			if strings.EqualFold(balance.Coin, `usd`) || strings.EqualFold(balance.Coin, `usdt`) {
				sm.availableU += balance.Amount
			}
			// 可用usd数量需要减去现有所有借币负债总额
			if balance.UsdValue < 0 {
				sm.availableU -= math.Abs(balance.UsdValue)
			}
		}
	}
	setSpotMarket(key, market, sm)
	return
}

func createFromPosition(key, secret string, setting *model.Setting) (carryStatus *CarryStatus) {
	cm := getContractMarket(key, setting.Market)
	if cm == nil {
		cm = createContractMarket(key, secret, setting.Market)
	}
	if cm == nil {
		return nil
	}
	carryStatus = &CarryStatus{isSpot: false, market: setting.Market, symbol: setting.Symbol, key: key, secret: secret,
		LimitSell: math.NaN(), LimitBuy: math.NaN(), TradeLineBuy: setting.OpenShortMargin, TradeLineSell: setting.CloseShortMargin,
	}
	if cm.positions[setting.Symbol] != nil {
		carryStatus.Holding = cm.positions[setting.Symbol].Free
		carryStatus.ValueInUsd = math.Abs(carryStatus.Holding) * cm.positions[setting.Symbol].EntryPrice
		if cm.collateralsInU > 0 {
			carryStatus.RateInAll = carryStatus.ValueInUsd / cm.collateralsInU
			if cm.contractValueInU/cm.collateralsInU < 0.2 {
				if carryStatus.Holding > 0 {
					carryStatus.TradeLineBuy = 1
				} else if carryStatus.Holding < 0 {
					carryStatus.TradeLineSell = 1
				}
			}
		} else {
			carryStatus.TradeLineBuy = 1
			carryStatus.TradeLineSell = 1
		}
	}
	return carryStatus
}

func createFromBalance(key, secret string, setting *model.Setting) (carryStatus *CarryStatus) {
	sm := getSpotMarket(key, setting.Market)
	if sm == nil {
		sm = createSpotMarket(key, secret, setting.Market)
	}
	if sm == nil {
		return
	}
	carryStatus = &CarryStatus{isSpot: true, market: setting.Market, symbol: setting.Symbol, key: key, secret: secret,
		LimitSell:     math.NaN(),
		LimitBuy:      math.NaN(),
		TradeLineBuy:  setting.OpenShortMargin,
		TradeLineSell: setting.CloseShortMargin,
	}
	if sm.balances[setting.Symbol] != nil {
		carryStatus.Holding = sm.balances[setting.Symbol].Amount
		carryStatus.ValueInUsd = sm.balances[setting.Symbol].UsdValue
		carryStatus.RateInAll = carryStatus.ValueInUsd / sm.accountValueInU
		doRevert := false
		if sm.accountValueInU <= 0 || sm.availableU/sm.accountValueInU < 0.2 ||
			(sm.collateral != nil && (sm.collateral.Rate < 10 || sm.collateral.Available <= 0 ||
				(sm.collateral.Available-sm.collateral.Occupied)/sm.collateral.Available < 0.1)) {
			doRevert = true
		}
		if doRevert && carryStatus.Holding > 0 {
			carryStatus.TradeLineBuy = 1
		} else if doRevert && carryStatus.Holding < 0 {
			carryStatus.TradeLineSell = 1
		}
	}
	return
}

func initStatus(key, secret string, carryClose bool, carryRate float64, setting *model.Setting) {
	if setting == nil {
		return
	}
	tailSpot := model.GetSpotTail(setting.Market)
	tailPerp := model.GetPerpTail(setting.Market)
	var status *CarryStatus
	fundingRate := 0.0
	if setting.Symbol[len(setting.Symbol)-len(tailSpot):] == tailSpot {
		status = createFromBalance(key, secret, setting)
	} else if setting.Symbol[len(setting.Symbol)-len(tailPerp):] == tailPerp {
		status = createFromPosition(key, secret, setting)
		_, fundingRate = api.GetFundingRate(key, secret, setting.Market, setting.Symbol, nil)
		fundingRate *= 0.9
	}
	if carryStatus == nil {
		return
	}
	setCarryStatus(setting.Coin, setting.Market, setting.Symbol, key, status)
	now := time.Now().Unix()
	resetTime := getOKTradeMaxResetTime(key, setting.Symbol) + 600
	if setting.Market == model.OKEX && now > resetTime {
		setOKTradeMaxResetTime(key, setting.Symbol)
		getMax, maxBuy, maxSell := api.GetMaxSize(key, secret, setting.Symbol)
		if getMax {
			status.LimitSell = maxSell
			status.LimitBuy = maxBuy
		}
	}
	if status.RateInAll > 0 {
		status.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*status.RateInAll), winRateMin) + fundingRate
		status.TradeLineSell = math.Max(setting.OpenShortMargin*(0.5-jump*status.RateInAll), loseRateMax) - fundingRate
	} else {
		status.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*status.RateInAll), loseRateMax) + fundingRate
		status.TradeLineSell = math.Max(setting.OpenShortMargin*(0.5-jump*status.RateInAll), winRateMin) - fundingRate
	}
	if status.RateInAll > 0.5 {
		status.TradeLineBuy = 1
	}
	status.TradeLineBuy *= carryRate
	status.TradeLineSell *= carryRate
	if carryClose && status.Holding > 0 {
		status.TradeLineBuy = 1
	} else if carryClose && status.Holding < 0 {
		status.TradeLineSell = 1
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
		spotMarkets = make(map[string]map[string]*spotMarket)
		contractMarkets = make(map[string]map[string]*contractMarket)
		coinSettings := model.GetCoinSettings(model.FunctionCross)
		for i := 0; i < model.AppConfig.GetCrossLen(); i++ {
			for _, settings := range coinSettings {
				for _, setting := range settings {
					if setting == nil {
						continue
					}
					account := model.AppConfig.GetAccounts(setting.Market)[i]
					if account != nil {
						initStatus(account.Key, account.Secret, account.CarryClose, account.CarryRate, setting)
					}
				}
				makeEqual(i, settings)
			}
		}
		timer.Reset(time.Second * 60)
	}
}

// settings []*model.Setting, coinStatus map[string]map[string]map[string]*CarryStatus
func makeEqual(accountIndex int, settings []*model.Setting) {
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
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || model.AppPause ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) ||
		settings == nil || len(settings) == 0 || model.IsTickTimeout(setting.Market, delayTick) {
		return
	}
	for _, settingRelate := range settings {
		tickGet, tickRelate := model.AppMarkets.GetBidAsk(settingRelate.Symbol, settingRelate.Market)
		if !tickGet || setting.ID == settingRelate.ID ||
			(model.AppConfig.Env != `test` && model.IsRelatedTickTimeout(settingRelate.Market, million-int64(tickRelate.Ts))) {
			continue
		}
		for i := 0; i < model.AppConfig.GetCrossLen(); i++ {
			account := model.AppConfig.GetAccounts(setting.Market)[i]
			accountRelate := model.AppConfig.GetAccounts(settingRelate.Market)[i]
			if account == nil || accountRelate == nil {
				continue
			}
			status := getCarryStatus(setting.Coin, setting.Market, setting.Symbol, account.Key)
			statusRelate := getCarryStatus(settingRelate.Coin, settingRelate.Market, settingRelate.Symbol, accountRelate.Key)
			if status == nil || statusRelate == nil {
				util.Notice(`fail to get status  %s %s-%s %s-%s %s %s`,
					setting.Coin, setting.Market, setting.Symbol, settingRelate.Market, settingRelate.Symbol, account.Key, accountRelate.Key)
				continue
			}
			statusBuy, statusSell, amount, priceBuy, priceSell := calcAmount(status, statusRelate, tick, tickRelate)
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
	if scoreOpen > 0.1 || scoreClose > 0.1 {
		scoreOpen = 0
		scoreClose = 0
		msg := fmt.Sprintf(`different coin %s %s %s %s %f %f`, carryStatus.market, carryStatus.symbol,
			carryStatusRelate.market, carryStatusRelate.symbol, scoreOpen, scoreClose)
		for _, address := range model.TeamMails {
			err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, address,
				`不同币种`, msg)
			if err != nil {
				util.Notice(`fail to send mail msg %s %s`, msg, err.Error())
			}
		}
		return nil, nil, 0, 0, 0
	}
	mark := fmt.Sprintf(`%s-%s<->%s-%s`, carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol)
	if scoreOpen > 0.01 || -1*scoreClose < -0.01 {
		model.AppMetric.AddCarry(mark, scoreOpen, -1*scoreClose)
	}
	if carryStatus.TradeLineSell < scoreOpen && carryStatusRelate.TradeLineBuy < scoreOpen {
		statusBuy = carryStatusRelate
		statusSell = carryStatus
		priceSell = tick.Bids[0].Price
		priceBuy = tickRelate.Asks[0].Price
		askAmount = tick.Bids[0].Amount
		bidAmount = tickRelate.Asks[0].Amount
	}
	if carryStatus.TradeLineBuy < scoreClose && carryStatusRelate.TradeLineSell < scoreClose {
		statusSell = carryStatusRelate
		statusBuy = carryStatus
		priceSell = tickRelate.Bids[0].Price
		priceBuy = tick.Asks[0].Price
		askAmount = tickRelate.Bids[0].Amount
		bidAmount = tick.Bids[0].Amount
	}
	if statusBuy == nil || statusSell == nil {
		return nil, nil, 0, 0, 0
	}
	// todo test all markets real amount
	_, bidAmount = model.ParseRealAmount(statusBuy.market, statusBuy.symbol, bidAmount)
	_, askAmount = model.ParseRealAmount(statusSell.market, statusSell.symbol, askAmount)
	if !math.IsNaN(statusSell.LimitSell) {
		askAmount = math.Min(statusSell.LimitSell, askAmount)
	}
	if !math.IsNaN(statusBuy.LimitBuy) {
		bidAmount = math.Min(statusBuy.LimitBuy, bidAmount)
	}
	buyMarketU := 0.0
	if statusBuy.isSpot {
		buyMarket := getSpotMarket(statusBuy.key, statusBuy.market)
		if buyMarket != nil {
			buyMarketU = buyMarket.availableU
		}
	}
	bidAmount = math.Min(buyMarketU/priceBuy/5, bidAmount)
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
	fmt.Sprintf(`%f`, priceSell)
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

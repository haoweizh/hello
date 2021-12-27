package cross

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"sort"
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
	success, positions, usdInFuture := api.GetPositions(key, secret, market)
	if success {
		cm.positions = make(map[string]*model.Position)
		for _, position := range positions {
			cm.positions[position.Currency] = position
			// 只考虑USD(T)交易
			cm.contractValueInU += position.EntryPrice * math.Abs(position.Free)
		}
		cm.collateralsInU = usdInFuture
	}
	contractMarkets[key] = cm
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
	spotMarkets[key] = sm
	return
}

func createFromPosition(key, secret string, setting *model.Setting) (carryStatus *CarryStatus) {
	if contractMarkets[key] == nil {
		contractMarkets[key] = createContractMarket(key, secret, setting.Market)
		if (setting.Market == model.OKEX || setting.Market == model.Ftx) && spotMarkets[key] == nil {
			spotMarkets[key] = createSpotMarket(key, secret, setting.Market)
		}
	}
	if contractMarkets[key] == nil {
		return nil
	}
	carryStatus = &CarryStatus{isSpot: false, market: setting.Market, symbol: setting.Symbol, key: key, secret: secret,
		LimitSell: math.NaN(), LimitBuy: math.NaN(), TradeLineBuy: setting.OpenShortMargin, TradeLineSell: setting.CloseShortMargin,
	}
	if contractMarkets[key].positions[setting.Symbol] != nil {
		carryStatus.Holding = contractMarkets[key].positions[setting.Symbol].Free
		carryStatus.ValueInUsd = math.Abs(carryStatus.Holding)*contractMarkets[key].positions[setting.Symbol].EntryPrice +
			contractMarkets[key].positions[setting.Symbol].ProfitUnreal
	}
	carryStatus.RateInAll = carryStatus.ValueInUsd / contractMarkets[key].collateralsInU
	holdLimit := holdingLimitInU
	account0 := model.AppConfig.GetAccounts(setting.Market)[0]
	if account0.Key != key {
		holdLimit /= 10
	}
	if contractMarkets[key].contractValueInU/contractMarkets[key].collateralsInU > 4 ||
		carryStatus.ValueInUsd > holdLimit {
		util.Notice(fmt.Sprintf(`杠杆较高，停止开仓 %s %f %f`,
			key, contractMarkets[key].contractValueInU, contractMarkets[key].collateralsInU))
		if carryStatus.Holding > 0 {
			carryStatus.TradeLineBuy = 1
		} else if carryStatus.Holding < 0 {
			carryStatus.TradeLineSell = 1
		}
	}
	return carryStatus
}

func createFromBalance(key, secret string, setting *model.Setting) (carryStatus *CarryStatus) {
	if spotMarkets[key] == nil {
		spotMarkets[key] = createSpotMarket(key, secret, setting.Market)
	}
	if spotMarkets[key] == nil {
		return
	}
	carryStatus = &CarryStatus{isSpot: true, market: setting.Market, symbol: setting.Symbol, key: key, secret: secret,
		LimitSell:     math.NaN(),
		LimitBuy:      math.NaN(),
		TradeLineBuy:  setting.OpenShortMargin,
		TradeLineSell: setting.CloseShortMargin,
	}
	if spotMarkets[key].balances[setting.Symbol] != nil {
		carryStatus.Holding = spotMarkets[key].balances[setting.Symbol].Amount
		carryStatus.ValueInUsd = spotMarkets[key].balances[setting.Symbol].UsdValue
		carryStatus.RateInAll = carryStatus.ValueInUsd / spotMarkets[key].accountValueInU
		doRevert := false
		if spotMarkets[key].accountValueInU <= 0 || spotMarkets[key].availableU/spotMarkets[key].accountValueInU < 0.2 ||
			(spotMarkets[key].collateral != nil && (spotMarkets[key].collateral.Rate < 10 || spotMarkets[key].collateral.Available <= 0 ||
				(spotMarkets[key].collateral.Available-spotMarkets[key].collateral.Occupied)/spotMarkets[key].collateral.Available < 0.1)) {
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

// todo 增加更新status在网页上的显示数据
func initStatus(key, secret string, carryClose bool, carryRate float64, setting *model.Setting) (status *CarryStatus) {
	if setting == nil {
		return
	}
	tailSpot := model.GetSpotTail(setting.Market)
	tailPerp := model.GetPerpTail(setting.Market)
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
	return
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
		spotMarkets = make(map[string]*spotMarket)
		contractMarkets = make(map[string]*contractMarket)
		coinSettings := model.GetCoinSettings(model.FunctionCross)
		for i := 0; i < model.AppConfig.GetCrossLen(); i++ {
			for _, settings := range coinSettings {
				equalStatuses := make([]*CarryStatus, len(settings))
				for j, setting := range settings {
					if setting == nil {
						continue
					}
					account := model.AppConfig.GetAccounts(setting.Market)[i]
					if account != nil {
						equalStatuses[j] = initStatus(account.Key, account.Secret, account.CarryClose, account.CarryRate, setting)
					}
				}
				makeEqual(equalStatuses)
			}
		}
		timer.Reset(time.Second * 60)
	}
}

// bybit 缺少按照symbol cancel all
// settings []*model.Setting, coinStatus map[string]map[string]map[string]*CarryStatus
func makeEqual(statuses []*CarryStatus) (success bool, msg string) {
	var holding, holdingInU, price float64
	orderSide := ``
	var equalStatus *CarryStatus
	bids := model.Ticks{}
	asks := model.Ticks{}
	bidStatus := make(map[string]*CarryStatus)
	askStatus := make(map[string]*CarryStatus)
	for _, status := range statuses {
		holding += status.Holding
		getTick, tick := model.AppMarkets.GetBidAsk(status.symbol, status.market)
		if !getTick {
			return false, fmt.Sprintf(`no tick when equal %s %s`, status.market, status.symbol)
		}
		bids = append(bids, tick.Bids[0])
		asks = append(asks, tick.Asks[0])
		bidStatus[fmt.Sprintf(`%s_%s`, status.market, status.symbol)] = status
		askStatus[fmt.Sprintf(`%s_%s`, status.market, status.symbol)] = status
		holdingInU += holding * tick.Bids[0].Price
	}
	if holdingInU > 10 {
		orderSide = model.OrderSideSell
		sort.Sort(sort.Reverse(bids))
		for _, bid := range bids {
			status := bidStatus[fmt.Sprintf(`%s_%s`, bid.Market, bid.Symbol)]
			if status == nil {
				util.Notice(fmt.Sprintf(`no status when holding in U: %f`, holdingInU))
				continue
			}
			if math.IsNaN(status.LimitSell) || status.LimitSell > holding {
				equalStatus = status
				price = bid.Price
			}
			go api.CancelOrders(status.key, status.secret, status.market, status.symbol)
		}
	}
	if holding < -10 {
		orderSide = model.OrderSideBuy
		sort.Sort(asks)
		for _, ask := range asks {
			status := askStatus[fmt.Sprintf(`%s_%s`, ask.Market, ask.Symbol)]
			if math.IsNaN(status.LimitBuy) || status.LimitBuy > math.Abs(holding) {
				equalStatus = status
				price = ask.Price
			}
			go api.CancelOrders(status.key, status.secret, status.market, status.symbol)
		}
	}
	if equalStatus != nil {
		amount := math.Min(90000000, math.Abs(holding))
		checkAmount := model.GetAmountInMarket(equalStatus.market, equalStatus.symbol, amount)
		if checkAmount > 0 {
			api.PlaceOrder(equalStatus.key, equalStatus.secret, orderSide, model.OrderTypeLimit, equalStatus.market,
				equalStatus.symbol, equalStatus.symbol, ``, model.FunctionComplement, price, price, amount,
				true, true, nil, nil)
		}
	}
	return
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
				go placeCross(statusBuy, statusSell, priceBuy, priceSell, amount, setting, settingRelate)
				return
			}
		}
	}
}

func calcAmount(carryStatus, carryStatusRelate *CarryStatus, tick,
	tickRelate *model.BidAsk) (statusBuy, statusSell *CarryStatus, amount, priceBuy, priceSell float64) {
	var bidAmount, askAmount float64
	score := 1 - tickRelate.Asks[0].Price/tick.Bids[0].Price
	scoreRelate := tickRelate.Bids[0].Price/tick.Asks[0].Price - 1
	if score > 0.1 || scoreRelate > 0.1 {
		score = 0
		scoreRelate = 0
		msg := fmt.Sprintf(`different coin %s %s %s %s %f %f`, carryStatus.market, carryStatus.symbol,
			carryStatusRelate.market, carryStatusRelate.symbol, score, scoreRelate)
		for _, address := range model.TeamMails {
			err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, address,
				`不同币种`, msg)
			if err != nil {
				util.Notice(`fail to send mail msg %s %s`, msg, err.Error())
			}
		}
		return nil, nil, 0, 0, 0
	}
	mark := fmt.Sprintf(`%s-%s|%s-%s`, carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol)
	if score > 0.01 || -1*scoreRelate < -0.01 {
		model.AppMetric.AddCarry(mark, score, -1*scoreRelate)
	}
	if carryStatus.TradeLineSell < score && carryStatusRelate.TradeLineBuy < score {
		statusSell = carryStatus
		statusBuy = carryStatusRelate
		priceSell = tick.Bids[0].Price
		priceBuy = tickRelate.Asks[0].Price
		askAmount = tick.Bids[0].Amount
		bidAmount = tickRelate.Asks[0].Amount
	}
	if carryStatus.TradeLineBuy < scoreRelate && carryStatusRelate.TradeLineSell < scoreRelate {
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
	buyLimit := math.Min(statusBuy.ValueInUsd/15, openValueLimit)
	if statusBuy.isSpot && spotMarkets[statusBuy.key] != nil {
		buyLimit = math.Min(spotMarkets[statusBuy.key].availableU/5, buyLimit)
	}
	bidAmount = math.Min(bidAmount, buyLimit/priceBuy)
	// todo binance 要求下单金额大于10u，gate要求大于1u
	//openValueMin := setting.AmountLimit
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

func placeCross(statusBuy, statusSell *CarryStatus, priceBuy, priceSell, amount float64, setting, relateSetting *model.Setting) {
	if !checkSetCrossing(true) {
		defer checkSetCrossing(false)
	} else {
		//util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
		return
	}
	//todo postcarry
	placeSuccess := true
	if statusBuy.market == model.OKEX && statusSell.market == model.OKEX {
		var sidePerp, sideRelated string
		var perpPrice, relatedPrice float64
		coin := model.GetCoin(statusBuy.market, statusBuy.symbol)
		if statusBuy.isSpot {
			sideRelated = model.OrderSideBuy
			relatedPrice = priceBuy
			sidePerp = model.OrderSideSell
			perpPrice = priceSell
		} else {
			sideRelated = model.OrderSideSell
			relatedPrice = priceSell
			sidePerp = model.OrderSideBuy
			perpPrice = priceBuy
		}
		placeSuccess = api.PlacePairOKEX(statusBuy.key, coin, sidePerp, sideRelated, model.OrderTypeLimit, model.FunctionCross, perpPrice, relatedPrice, amount)
	} else {
		go api.PlaceOrder(statusBuy.key, statusBuy.secret, model.OrderSideBuy, model.OrderTypeLimit, statusBuy.market, statusBuy.symbol,
			``, ``, model.FunctionCross, priceBuy, priceBuy,
			amount, true, true, postOrderCross, nil)
		api.PlaceOrder(statusSell.key, statusSell.secret, model.OrderSideSell, model.OrderTypeLimit, statusSell.market, statusSell.symbol,
			``, ``, model.FunctionCross, priceSell, priceSell,
			amount, true, true, postOrderCross, nil)
		time.Sleep(time.Second / 5)
	}
	if placeSuccess {
		var settingBuy, settingSell model.Setting
		if statusBuy.symbol == setting.Symbol {
			settingBuy = *setting
			settingSell = *relateSetting
		} else {
			settingBuy = *relateSetting
			settingSell = *setting
		}
		placeStatus(statusBuy, priceBuy, &settingBuy, amount, model.OrderSideBuy)
		placeStatus(statusSell, priceSell, &settingSell, amount, model.OrderSideSell)
	}
}

func placeStatus(status *CarryStatus, price float64, setting *model.Setting, amount float64, orderSide string) {
	if model.OrderSideBuy == orderSide {
		if status.isSpot {
			sMarket := spotMarkets[status.key]
			balance := sMarket.balances[status.symbol]
			balance.Amount += amount
			balance.UsdValue = balance.Amount * price
			sMarket.availableU -= amount * price
			if status.market == model.Ftx {
				contractMarkets[status.key].collateralsInU -= amount * price
			} else if status.market == model.OKEX {
				sMarket.collateral.Available -= amount * price
				sMarket.collateral.Occupied += amount * price
				contractMarkets[status.key].collateralsInU -= amount * price
			}
		} else {
			pMarket := contractMarkets[status.key]
			position := pMarket.positions[status.symbol]
			position.Free += amount
			position.EntryPrice = price
			pMarket.collateralsInU -= amount * price * 0.2
			if status.market == model.Ftx {
				spotMarkets[status.key].availableU -= amount * price * 0.2
			} else if status.market == model.OKEX {
				spotMarkets[status.key].collateral.Available -= amount * price * 0.1
				spotMarkets[status.key].collateral.Occupied += amount * price * 0.1
				spotMarkets[status.key].availableU -= amount * price * 0.1
			}
		}
	} else {
		if status.isSpot {
			sMarket := spotMarkets[status.key]
			balance := sMarket.balances[status.symbol]
			balance.Amount -= amount
			balance.UsdValue = balance.Amount * price
			sMarket.availableU += amount * price
			if status.market == model.Ftx {
				contractMarkets[status.key].collateralsInU += amount * price
			} else if status.market == model.OKEX {
				sMarket.collateral.Available += amount * price
				sMarket.collateral.Occupied -= amount * price
				contractMarkets[status.key].collateralsInU += amount * price
			}
		} else {
			pMarket := contractMarkets[status.key]
			position := pMarket.positions[status.symbol]
			position.Free -= amount
			position.EntryPrice = price
			pMarket.collateralsInU += amount * price * 0.2
			if status.market == model.Ftx {
				spotMarkets[status.key].availableU += amount * price * 0.2
			} else if status.market == model.OKEX {
				spotMarkets[status.key].collateral.Available += amount * price * 0.1
				spotMarkets[status.key].collateral.Occupied -= amount * price * 0.1
				spotMarkets[status.key].availableU += amount * price * 0.1
			}
		}
	}
	account := model.AppConfig.GetAccountFromKey(status.market, status.key)
	initStatus(status.key, status.secret, account.CarryClose, account.CarryRate, setting)
}

var postOrderCross = func(order *model.Order, setting *model.Setting) {
	//if order == nil {
	//	return
	//}
	//if order.HaveId() {
	//	addLastCarry(order, setting)
	//	addCarryResult(order.AmountType, order.Market, true)
	//} else {
	//	unknownFail := true
	//	account := model.AppConfig.GetAccountFromKey(order.Market, order.AmountType)
	//	if account != nil {
	//		switch order.Market {
	//		case model.OKEX:
	//			if InsufficientCodeOKEX[order.ErrCode] {
	//				util.Notice(`reset %s trade max with %s %s`, order.Market, order.ErrCode, order.AmountType)
	//				resetTradeMax(account.Key, account.Secret, model.OKEX)
	//				unknownFail = false
	//			}
	//		case model.Binance:
	//			if strings.Contains(InsufficientCodeBinance, order.ErrCode) {
	//				util.Notice(`reset binance trade max with %s %s`, order.ErrCode, order.AmountType)
	//				clearCarry(account.Key, account.Secret, order.Market)
	//				unknownFail = false
	//			}
	//		}
	//	}
	//	if unknownFail {
	//		addCarryResult(order.AmountType, order.Market, false)
	//	} else {
	//		addCarryResult(order.AmountType, order.Market, true)
	//	}
	//}
}

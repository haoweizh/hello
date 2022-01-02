package cross

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
				sm.availableU += math.Min(balance.Amount, balance.AvailableWithBorrow)
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

func createFromPosition(account *model.Account, setting *model.Setting, valueLimit float64) (carryStatus *CarryStatus, doRevert bool) {
	key := account.Key
	if contractMarkets[key] == nil {
		contractMarkets[key] = createContractMarket(key, account.Secret, setting.Market)
		if (setting.Market == model.OKEX || setting.Market == model.Ftx) && spotMarkets[key] == nil {
			spotMarkets[key] = createSpotMarket(key, account.Secret, setting.Market)
		}
	}
	getTick, ticks := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	if contractMarkets[key] == nil || !getTick {
		return nil, false
	}
	price := ticks.Asks[0].Price
	carryStatus = &CarryStatus{isSpot: false, market: setting.Market, symbol: setting.Symbol, account: account,
		setting:      setting,
		LimitSell:    math.Min(contractMarkets[key].collateralsInU/5, openValueLimit) / price,
		LimitBuy:     math.Min(contractMarkets[key].collateralsInU/5, openValueLimit) / price,
		TradeLineBuy: setting.OpenShortMargin, TradeLineSell: setting.CloseShortMargin,
	}
	if setting.Market == model.Gate {
		marketInfo := model.GetMarketInfo(setting.Market, setting.Symbol)
		if marketInfo != nil {
			_, amount := model.ParseRealAmount(setting.Market, setting.Symbol, marketInfo.SizeMax)
			carryStatus.LimitBuy = math.Min(carryStatus.LimitBuy, amount)
			carryStatus.LimitSell = math.Min(carryStatus.LimitSell, amount)
		}
	}
	valueInUsd := 0.0
	if contractMarkets[key].positions[setting.Symbol] != nil {
		carryStatus.Holding = contractMarkets[key].positions[setting.Symbol].Free
		valueInUsd = math.Abs(carryStatus.Holding) * price
		carryStatus.RateInAll = valueInUsd / contractMarkets[key].collateralsInU
	}
	if contractMarkets[key].contractValueInU/contractMarkets[key].collateralsInU > 3 || valueInUsd > valueLimit ||
		valueInUsd/contractMarkets[key].collateralsInU > 0.5 {
		//util.Notice(fmt.Sprintf(`杠杆较高，停止开仓 %s %f %f %f %f`,
		//	key, contractMarkets[key].contractValueInU, contractMarkets[key].collateralsInU, valueInUsd, valueLimit))
		doRevert = true
	}
	return carryStatus, doRevert
}

func createFromBalance(account *model.Account, setting *model.Setting, valueLimit float64) (carryStatus *CarryStatus, doRevert bool) {
	key := account.Key
	if spotMarkets[key] == nil {
		spotMarkets[key] = createSpotMarket(key, account.Secret, setting.Market)
	}
	getTick, ticks := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	if spotMarkets[key] == nil || !getTick {
		return
	}
	price := ticks.Asks[0].Price
	carryStatus = &CarryStatus{isSpot: true, market: setting.Market, symbol: setting.Symbol, account: account,
		setting:       setting,
		LimitSell:     0,
		LimitBuy:      math.Min(openValueLimit, math.Min(spotMarkets[key].availableU/5, spotMarkets[key].accountValueInU/15)) / price,
		TradeLineBuy:  setting.OpenShortMargin,
		TradeLineSell: setting.CloseShortMargin,
	}
	if spotMarkets[key].balances[setting.Symbol] != nil {
		balance := spotMarkets[key].balances[setting.Symbol]
		carryStatus.Holding = balance.Amount
		// 暂时不让借币
		carryStatus.LimitSell = math.Min(math.Min(balance.Amount, balance.AvailableWithBorrow), openValueLimit/price)
		carryStatus.RateInAll = math.Abs(carryStatus.Holding * ticks.Asks[0].Price / spotMarkets[key].accountValueInU)
	}
	if spotMarkets[key].availableU/spotMarkets[key].accountValueInU < 0.2 ||
		spotMarkets[key].accountValueInU <= 0 || carryStatus.RateInAll > 0.5 {
		doRevert = true
	}
	if spotMarkets[key].balances[setting.Symbol] != nil &&
		math.Abs(spotMarkets[key].balances[setting.Symbol].UsdValue) > valueLimit {
		doRevert = true
	}
	if spotMarkets[key].collateral != nil && (spotMarkets[key].collateral.Rate < 10 ||
		(spotMarkets[key].collateral.Available-spotMarkets[key].collateral.Occupied)/spotMarkets[key].collateral.Available < 0.1) {
		doRevert = true
	}
	return carryStatus, doRevert
}

func initStatus(account *model.Account, setting *model.Setting) (status *CarryStatus) {
	if setting == nil {
		return
	}
	tailSpot := model.GetSpotTail(setting.Market)
	tailPerp := model.GetPerpTail(setting.Market)
	fundingRate := 0.0
	doRevert := false
	holdLimit := holdingLimitInU
	account0 := model.AppConfig.GetAccounts(setting.Market)[0]
	if account0.Key != account.Key {
		holdLimit /= 10
	}
	if setting.Symbol[len(setting.Symbol)-len(tailSpot):] == tailSpot {
		status, doRevert = createFromBalance(account, setting, holdLimit)
	} else if setting.Symbol[len(setting.Symbol)-len(tailPerp):] == tailPerp {
		status, doRevert = createFromPosition(account, setting, holdLimit)
		_, fundingRate = api.GetFundingRate(account.Key, account.Secret, setting.Market, setting.Symbol, nil)
		fundingRate *= 0.9
	}
	if carryStatus == nil || status == nil {
		return
	}
	if setting.Market == model.Ftx {
		status.LimitBuy = math.Min(status.LimitBuy, 90000000)
		status.LimitSell = math.Min(status.LimitSell, 90000000)
	}
	if setting.Market == model.OKEX {
		success, maxBuy, maxSell := api.GetTradeMaxOKEX(account.Key, account.Secret, setting.Symbol, 600)
		if success {
			status.LimitBuy = math.Min(status.LimitBuy, maxBuy)
			status.LimitSell = math.Min(status.LimitSell, maxSell)
		}
	}
	setCarryStatus(setting.Coin, setting.Market, setting.Symbol, account.Key, status)
	jump := 5.0
	if status.isSpot {
		jump = 10
	}
	if status.Holding > 0 {
		status.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*status.RateInAll), winRateMin) + fundingRate
		status.TradeLineSell = math.Max(setting.CloseShortMargin*(0.5-jump*status.RateInAll), loseRateMax) - fundingRate
	} else {
		status.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*status.RateInAll), loseRateMax) + fundingRate
		status.TradeLineSell = math.Max(setting.CloseShortMargin*(0.5-jump*status.RateInAll), winRateMin) - fundingRate
	}
	status.TradeLineBuy *= account.CarryRate
	status.TradeLineSell *= account.CarryRate
	if status.Holding > 0 && (doRevert || account.CarryClose) {
		status.TradeLineBuy = 1
	} else if status.Holding < 0 && (doRevert || account.CarryClose) {
		status.TradeLineSell = 1
	}
	return
}

func ClearCross() {
	for doCross {
		for true {
			if !checkSetCrossing(true) {
				break
			} else {
				time.Sleep(time.Millisecond * 200)
			}
		}
		if lastOrderSymbol != nil && len(lastOrderSymbol) == 0 {
			util.Notice(`...... no change pass make equal`)
		} else {
			util.Notice(`...... enter clearing cross`)
			clearSuccess := true
			spotMarkets = make(map[string]*spotMarket)
			contractMarkets = make(map[string]*contractMarket)
			coinSettings := model.GetCoinSettings(model.FunctionCross)
			for i := 0; i < model.AppConfig.GetCrossLen(); i++ {
				for coin, settings := range coinSettings {
					equalStatuses := make([]*CarryStatus, len(settings))
					for j, setting := range settings {
						account := model.AppConfig.GetAccounts(setting.Market)[i]
						if setting == nil || len(coin) == 0 || coin != setting.Symbol[0:len(coin)] || account == nil {
							util.Notice(`can not equal`)
							clearSuccess = false
							continue
						}
						equalStatuses[j] = initStatus(account, setting)
					}
					makeEqual(coin, equalStatuses)
				}
			}
			if clearSuccess {
				lastOrderSymbol = make(map[string]map[string]string)
			} else {
				lastOrderSymbol = nil
			}
			util.Notice(`...... exit clearing cross`)
		}
		checkSetCrossing(false)
		time.Sleep(time.Second * 60)
	}
}

// bybit 缺少按照symbol cancel all
// settings []*model.Setting, coinStatus map[string]map[string]map[string]*CarryStatus
func makeEqual(coin string, statuses []*CarryStatus) (success bool, msg string) {
	var holding, holdingInU, price float64
	orderSide := ``
	var equalStatus *CarryStatus
	bids := model.Ticks{}
	asks := model.Ticks{}
	bidStatus := make(map[string]*CarryStatus)
	askStatus := make(map[string]*CarryStatus)
	for _, status := range statuses {
		if status == nil {
			util.Notice(`warning: fail to get one status %s`, coin)
			return false, `fail to equal for one nil status`
		}
		holding += status.Holding
		getTick, tick := model.AppMarkets.GetBidAsk(status.symbol, status.market)
		if !getTick {
			return false, fmt.Sprintf(`no tick when equal %s %s`, status.market, status.symbol)
		}
		bids = append(bids, tick.Bids[0])
		asks = append(asks, tick.Asks[0])
		bidStatus[fmt.Sprintf(`%s_%s`, status.market, status.symbol)] = status
		askStatus[fmt.Sprintf(`%s_%s`, status.market, status.symbol)] = status
		holdingInU += status.Holding * tick.Bids[0].Price
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
			go api.CancelOrders(status.account.Key, status.account.Secret, status.market, status.symbol)
		}
	}
	if holdingInU < -10 {
		orderSide = model.OrderSideBuy
		sort.Sort(asks)
		for _, ask := range asks {
			status := askStatus[fmt.Sprintf(`%s_%s`, ask.Market, ask.Symbol)]
			if math.IsNaN(status.LimitBuy) || status.LimitBuy > math.Abs(holding) {
				equalStatus = status
				price = ask.Price
			}
			go api.CancelOrders(status.account.Key, status.account.Secret, status.market, status.symbol)
		}
	}
	if equalStatus != nil {
		amount := math.Min(90000000, math.Abs(holding))
		checkAmount := model.GetAmountInMarket(equalStatus.market, equalStatus.symbol, amount)
		if checkAmount > 0 {
			api.PlaceOrder(equalStatus.account.Key, equalStatus.account.Secret, orderSide, model.OrderTypeLimit,
				equalStatus.market, equalStatus.symbol, equalStatus.symbol, ``, model.FunctionComplement,
				price, price, amount, true, true, nil, nil)
		}
	}
	return
}

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
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) || setting.Valid == false ||
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
				continue
			}
			statusBuy, statusSell, amount, priceBuy, priceSell := calcAmount(i, setting.Coin, status, statusRelate, tick, tickRelate)
			if amount > 0 {
				go placeCross(statusBuy, statusSell, priceBuy, priceSell, amount)
				return
			}
		}
	}
}

func calcAmount(index int, coin string, carryStatus, carryStatusRelate *CarryStatus, tick,
	tickRelate *model.BidAsk) (statusBuy, statusSell *CarryStatus, amount, priceBuy, priceSell float64) {
	now := time.Now()
	if now.Hour()%8 == 0 && now.Minute() == 0 && now.Second() < 30 {
		return
	}
	if getCarryStop(carryStatus.account.Key) || getCarryStop(carryStatusRelate.account.Key) {
		util.Debug(`stop carry for 10 times unknown carry %s or %s %s`,
			carryStatus.account.Key, carryStatusRelate.account.Key, coin)
		return
	}
	var bidAmount, askAmount float64
	score := 1 - tickRelate.Asks[0].Price/tick.Bids[0].Price
	scoreRelate := tickRelate.Bids[0].Price/tick.Asks[0].Price - 1
	if (score > 0.1 || scoreRelate > 0.1) && (!isValidSymbol(carryStatus.market, carryStatus.symbol) ||
		!isValidSymbol(carryStatusRelate.market, carryStatusRelate.symbol)) {
		msg := fmt.Sprintf(`different coin %s %s %s %s %f %f`, carryStatus.market, carryStatus.symbol,
			carryStatusRelate.market, carryStatusRelate.symbol, score, scoreRelate)
		minute := time.Now().Minute()
		second := time.Now().Second()
		if minute == 0 && second == 0 {
			for _, address := range model.TeamMails {
				err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, address,
					`不同币种`, msg)
				if err != nil {
					util.Notice(`fail to send mail msg %s %s`, msg, err.Error())
				}
			}
		}
		return nil, nil, 0, 0, 0
	}
	mark := fmt.Sprintf(`%s_%s|%s_%s`, carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol)
	if score > 0.01 {
		model.AppMetric.AddCarry(mark, score, 0)
	}
	if carryStatus.TradeLineSell < score && carryStatusRelate.TradeLineBuy < score {
		statusSell = carryStatus
		statusBuy = carryStatusRelate
		priceSell = tick.Bids[0].Price
		priceBuy = tickRelate.Asks[0].Price
		askAmount = tick.Bids[0].Amount * 0.9
		bidAmount = tickRelate.Asks[0].Amount * 0.9
	}
	if carryStatus.TradeLineBuy < scoreRelate && carryStatusRelate.TradeLineSell < scoreRelate {
		statusSell = carryStatusRelate
		statusBuy = carryStatus
		priceSell = tickRelate.Bids[0].Price
		priceBuy = tick.Asks[0].Price
		askAmount = tickRelate.Bids[0].Amount * 0.9
		bidAmount = tick.Bids[0].Amount * 0.9
	}
	// 为了同一对交易对冲不出现两次，对前后进行排序
	mark = fmt.Sprintf(`%s-%s`, carryStatus.market, carryStatus.symbol)
	markRelate := fmt.Sprintf(`%s-%s`, carryStatusRelate.market, carryStatusRelate.symbol)
	coinValue := coin
	if !carryStatus.isSpot {
		coinValue += `永`
	}
	coinValueRelate := coin
	if !carryStatusRelate.isSpot {
		coinValueRelate += `永`
	}
	if mark < markRelate {
		mark = fmt.Sprintf(`%s|%s`, mark, markRelate)
		model.SetMonitorInfo(strconv.Itoa(index), `cross`, mark, []string{
			carryStatus.market, coinValue, carryStatusRelate.market, coinValueRelate,
			fmt.Sprintf(`%.1f, %.1f`, 100*carryStatus.TradeLineBuy, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.1f, %.1f`, 100*carryStatusRelate.TradeLineBuy, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.1f, %.1f`, 100*score, 100*scoreRelate),
			fmt.Sprintf(`%.0e, %.0e`, carryStatus.LimitBuy, carryStatus.LimitSell),
			fmt.Sprintf(`%.0e, %.0e`, carryStatusRelate.LimitBuy, carryStatusRelate.LimitSell),
			fmt.Sprintf(`%v`, statusBuy != nil && statusSell != nil)})
	} else {
		mark = fmt.Sprintf(`%s|%s`, markRelate, mark)
		model.SetMonitorInfo(strconv.Itoa(index), `cross`, mark, []string{
			carryStatusRelate.market, coinValueRelate, carryStatus.market, coinValue,
			fmt.Sprintf(`%.1f, %.1f`, 100*carryStatusRelate.TradeLineBuy, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.1f, %.1f`, 100*carryStatus.TradeLineBuy, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.1f, %.1f`, 100*score, 100*scoreRelate),
			fmt.Sprintf(`%.0e, %.0e`, carryStatusRelate.LimitBuy, carryStatusRelate.LimitSell),
			fmt.Sprintf(`%.0e, %.0e`, carryStatus.LimitBuy, carryStatus.LimitSell),
			fmt.Sprintf(`%v`, statusBuy != nil && statusSell != nil)})
	}
	if statusBuy == nil || statusSell == nil {
		return nil, nil, 0, 0, 0
	}
	if !isFresh(statusBuy.account.Key, statusBuy.market, statusBuy.symbol) {
		initLimitBuyAndSell(statusBuy, statusBuy.setting, priceBuy)
	}
	if !isFresh(statusSell.account.Key, statusSell.market, statusSell.symbol) {
		initLimitBuyAndSell(statusSell, statusSell.setting, priceSell)
	}
	amount = math.Min(math.Min(statusBuy.LimitBuy, bidAmount), math.Min(statusSell.LimitSell, askAmount))
	if amount > 0 {
		amount = model.FormatCrossPair(statusBuy.market, statusSell.market, statusBuy.symbol, statusSell.symbol, amount, priceBuy)
	}
	return statusBuy, statusSell, amount, priceBuy, priceSell
}

func initLimitBuyAndSell(status *CarryStatus, setting *model.Setting, price float64) {
	if status.isSpot {
		status.LimitBuy = math.Min(openValueLimit, math.Min(spotMarkets[status.account.Key].availableU/5, spotMarkets[status.account.Key].accountValueInU/15)) / price
		balance := spotMarkets[status.account.Key].balances[setting.Symbol]
		status.LimitSell = math.Min(math.Min(balance.Amount, balance.AvailableWithBorrow), openValueLimit/price)
	} else {
		status.LimitSell = math.Min(contractMarkets[status.account.Key].collateralsInU/5, openValueLimit) / price
		status.LimitBuy = math.Min(contractMarkets[status.account.Key].collateralsInU/5, openValueLimit) / price
		if setting.Market == model.Gate {
			marketInfo := model.GetMarketInfo(setting.Market, setting.Symbol)
			if marketInfo != nil {
				_, amount := model.ParseRealAmount(setting.Market, setting.Symbol, marketInfo.SizeMax)
				status.LimitBuy = math.Min(status.LimitBuy, amount)
				status.LimitSell = math.Min(status.LimitSell, amount)
			}
		}
	}
	if setting.Market == model.Ftx {
		status.LimitBuy = math.Min(status.LimitBuy, 90000000)
		status.LimitSell = math.Min(status.LimitSell, 90000000)
	}
	if setting.Market == model.OKEX {
		success, maxBuy, maxSell := api.GetTradeMaxOKEX(status.account.Key, status.account.Secret, setting.Symbol, 600)
		if success {
			status.LimitBuy = math.Min(status.LimitBuy, maxBuy)
			status.LimitSell = math.Min(status.LimitSell, maxSell)
		}
	}
}

func placeCross(statusBuy, statusSell *CarryStatus, priceBuy, priceSell, amount float64) {
	if !checkSetCrossing(true) {
		defer checkSetCrossing(false)
	} else {
		//util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
		return
	}
	util.Notice(fmt.Sprintf(`place cross %s %s -> %s %s at %f %f amount %f`,
		statusSell.market, statusSell.symbol, statusBuy.market, statusBuy.symbol, priceSell, priceBuy, amount))
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
		placeSuccess = api.PlacePairOKEX(statusBuy.account.Key, coin, sidePerp, sideRelated, model.OrderTypeLimit,
			model.FunctionCross, perpPrice, relatedPrice, amount)
	} else {
		go api.PlaceOrder(statusBuy.account.Key, statusBuy.account.Secret, model.OrderSideBuy, model.OrderTypeLimit,
			statusBuy.market, statusBuy.symbol, ``, ``, model.FunctionCross, priceBuy, priceBuy,
			amount, true, true, PostOrderCross, statusBuy.setting)
		api.PlaceOrder(statusSell.account.Key, statusSell.account.Secret, model.OrderSideSell, model.OrderTypeLimit,
			statusSell.market, statusSell.symbol, ``, ``, model.FunctionCross, priceSell, priceSell,
			amount, true, true, PostOrderCross, statusSell.setting)
		time.Sleep(time.Second / 5)
	}
	if placeSuccess {
		placeStatus(statusBuy, priceBuy, amount)
		placeStatus(statusSell, priceSell, -1*amount)
	}
}

func placeStatus(status *CarryStatus, price float64, amount float64) {
	if status.isSpot {
		sm := spotMarkets[status.account.Key]
		balance := sm.balances[status.symbol]
		if balance == nil {
			util.Notice(fmt.Sprintf(`warning no balance %s %s %s`,
				status.account.Key, status.market, status.symbol))
			balance = &model.Balance{Amount: amount, UsdValue: amount * price, Market: status.market, Coin: status.setting.Coin}
		} else {
			balance.Amount += amount
			balance.UsdValue = balance.Amount * price
		}
		sm.availableU -= amount * price
		if status.market == model.Ftx {
			contractMarkets[status.account.Key].collateralsInU -= amount * price
		} else if status.market == model.OKEX {
			sm.collateral.Available -= amount * price
			//sm.collateral.Occupied += amount * price
			contractMarkets[status.account.Key].collateralsInU -= amount * price
		}
	} else {
		cm := contractMarkets[status.account.Key]
		position := cm.positions[status.symbol]
		originFreeAbs := 0.0
		if position == nil {
			position = &model.Position{Free: amount, EntryPrice: price, Market: status.market, Currency: status.setting.Symbol}
		} else {
			originFreeAbs = math.Abs(position.Free)
			position.Free += amount
			position.EntryPrice = price
		}
		changeU := (originFreeAbs - math.Abs(position.Free)) * price
		cm.collateralsInU += changeU * 0.2
		cm.contractValueInU += changeU
		if status.market == model.Ftx {
			spotMarkets[status.account.Key].availableU += changeU * 0.2
		} else if status.market == model.OKEX {
			spotMarkets[status.account.Key].collateral.Available += changeU * 0.1
			spotMarkets[status.account.Key].collateral.Occupied -= changeU * 0.1
			spotMarkets[status.account.Key].availableU += changeU * 0.1
		}
	}
	account := model.AppConfig.GetAccountFromKey(status.market, status.account.Key)
	initStatus(account, status.setting)
	setFresh(account.Key, status.market, status.symbol)
}

var PostOrderCross = func(order *model.Order, setting *model.Setting) {
	if order == nil {
		return
	}
	if setting == nil {
		setting = model.GetSetting(model.FunctionCross, order.Market, order.Symbol)
	}
	account := model.AppConfig.GetAccountFromKey(order.Market, order.AmountType)
	if order.HaveId() {
		if account != nil {
			status := getCarryStatus(setting.Coin, setting.Market, setting.Symbol, account.Key)
			maxBuy := status.LimitBuy
			maxSell := status.LimitSell
			if order.OrderSide == model.OrderSideBuy {
				maxBuy -= order.Amount
				maxSell += order.Amount
			} else if order.OrderSide == model.OrderSideSell {
				maxBuy += order.Amount
				maxSell -= order.Amount
			}
			status.LimitSell = math.Min(status.LimitSell, maxSell)
			status.LimitBuy = math.Min(status.LimitBuy, maxBuy)
		}
		addLastCarry(order, setting)
		addCarryResult(order.AmountType, order.Market, true)
	} else {
		unknownFail := true
		if account != nil {
			switch order.Market {
			case model.OKEX:
				if InsufficientCodeOKEX[order.ErrCode] {
					util.Notice(`reset %s trade max with %s %s`, order.Market, order.ErrCode, order.AmountType)
					status := getCarryStatus(setting.Coin, setting.Market, setting.Symbol, account.Key)
					getMax, maxBuy, maxSell := api.GetTradeMaxOKEX(account.Key, account.Secret, setting.Symbol, 0)
					if getMax {
						status.LimitSell = math.Min(status.LimitSell, maxSell)
						status.LimitBuy = math.Min(status.LimitBuy, maxBuy)
					}
					unknownFail = false
				}
			case model.Binance:
				if strings.Contains(InsufficientCodeBinance, order.ErrCode) {
					util.Notice(`reset binance trade max with %s %s`, order.ErrCode, order.AmountType)
					spotMarkets[order.AmountType] = nil
					contractMarkets[order.AmountType] = nil
					initStatus(account, setting)
					unknownFail = false
				}
			}
		}
		if unknownFail {
			addCarryResult(order.AmountType, order.Market, false)
		} else {
			addCarryResult(order.AmountType, order.Market, true)
		}
	}
}

func setSettingStatus(setting *model.Setting, status bool) {
	time.Sleep(time.Minute * 20)
	setting.Valid = status
}

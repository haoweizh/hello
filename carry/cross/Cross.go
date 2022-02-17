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
	success, positions, accountValue, availableU := api.GetPositions(key, secret, market)
	settings := model.GetSettings(model.FunctionCross, market)
	if success {
		cm.positions = make(map[string]*model.Position)
		for _, position := range positions {
			cm.positions[position.Currency] = position
			getTick, tick := model.AppMarkets.GetBidAsk(position.Currency, market)
			if settings != nil && settings[position.Currency] != nil && getTick {
				cm.contractValueInU += tick.Bids[0].Price * math.Abs(position.Holding)
			}
		}
		cm.accountValueInU = accountValue
		cm.collateralsAvailable = availableU
	}
	contractMarkets.Store(key, cm)
	util.Notice(`create cm %s`, key)
	return
}

func createSpotMarket(key, secret, market string) (sm *spotMarket) {
	sm = &spotMarket{key: key, market: market}
	success, balances, totalInUsd, collateral := api.GetBalances(key, secret, market)
	//for _, balance := range balances {
	//	if balance.UsdValue == 0 && balance.Amount > 0 {
	//		util.Notice(fmt.Sprintf(`usdvalue 0 %s %s %f`, market, balance.Coin, balance.Amount))
	//	}
	//}
	if success {
		sm.balances = make(map[string]*model.Balance)
		sm.accountValueInU = totalInUsd
		sm.collateral = collateral
		for _, balance := range balances {
			sm.balances[balance.Coin+model.UniStandardTail[model.MarketTypeSpot]] = balance
			if strings.EqualFold(balance.Coin, `usd`) || strings.EqualFold(balance.Coin, `usdt`) {
				sm.availableU += math.Min(balance.Amount, balance.AvailableWithBorrow)
			}
			// 可用usd数量需要减去现有所有借币负债总额
			if balance.UsdValue < 0 {
				sm.availableU -= math.Abs(balance.UsdValue)
			}
		}
	}
	util.Notice(`create sm %s`, key)
	spotMarkets.Store(key, sm)
	return
}

func createFromPosition(account *model.Account, setting *model.Setting, valueLimit float64) (carryStatus *CarryStatus, doRevert bool) {
	key := account.Key
	value, ok := contractMarkets.Load(key)
	if value == nil || !ok {
		contractMarkets.Store(key, createContractMarket(key, account.Secret, setting.Market))
		value, _ = contractMarkets.Load(key)
		spotValue, spotOk := spotMarkets.Load(key)
		if (setting.Market == model.OKEX || setting.Market == model.Ftx) && (spotValue == nil || !spotOk) {
			spotMarkets.Store(key, createSpotMarket(key, account.Secret, setting.Market))
		}
	}
	if value == nil {
		util.Notice(fmt.Sprintf(`nil contract market %s %s`, setting.Market, setting.Symbol))
		return nil, false
	}
	cm := value.(*contractMarket)
	getTick, ticks := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	price := 0.0
	if getTick {
		price = ticks.Asks[0].Price
	} else if cm.positions[setting.Symbol] != nil {
		price = cm.positions[setting.Symbol].EntryPrice
		util.Notice(`no tick price, use position price %s %s %f`, setting.Market, setting.Symbol, price)
	}
	limitAmount := 0.0
	availableAmount := 0.0
	if price > 0 {
		limitAmount = math.Min(cm.accountValueInU/5, math.Min(cm.collateralsAvailable, openValueLimit)) / price
		availableAmount = cm.collateralsAvailable / price
	}
	carryStatus = &CarryStatus{isSpot: false, market: setting.Market, symbol: setting.Symbol, account: account,
		setting:       setting,
		LimitSell:     limitAmount,
		LimitBuy:      limitAmount,
		AvailableSell: availableAmount,
		AvailableBuy:  availableAmount,
		TradeLineBuy:  setting.OpenShortMargin,
		TradeLineSell: setting.CloseShortMargin}
	valueInUsd := 0.0
	if cm.positions[setting.Symbol] != nil {
		carryStatus.Holding = cm.positions[setting.Symbol].Holding
		valueInUsd = math.Abs(carryStatus.Holding) * price
		carryStatus.RateInAll = valueInUsd / cm.accountValueInU
	}
	lever := 2.0
	if setting.Market == model.Ftx || setting.Market == model.OKEX {
		lever = 1
	}
	if cm.contractValueInU/cm.accountValueInU > lever || valueInUsd > valueLimit || valueInUsd/cm.accountValueInU > 0.5 {
		//util.Notice(fmt.Sprintf(`杠杆较高，停止开仓 %s %f %f %f %f`,
		//	key, contractMarkets[key].contractValueInU, contractMarkets[key].collateralsInU, valueInUsd, valueLimit))
		doRevert = true
	}
	return carryStatus, doRevert
}

func createFromBalance(account *model.Account, setting *model.Setting, valueLimit float64) (carryStatus *CarryStatus, doRevert bool) {
	key := account.Key
	value, ok := spotMarkets.Load(key)
	if value == nil || !ok {
		spotMarkets.Store(key, createSpotMarket(key, account.Secret, setting.Market))
		value, ok = spotMarkets.Load(key)
	}
	if value == nil {
		util.Notice(fmt.Sprintf(`nil spot market %s %s`, setting.Market, setting.Symbol))
		return
	}
	sm := value.(*spotMarket)
	getTick, ticks := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	price := 0.0
	if getTick {
		price = ticks.Asks[0].Price
	} else {
		util.Notice(`fail to get ticket %s %s`, setting.Market, setting.Symbol)
	}
	limitBuy, limitSell, availableBuy := 0.0, 0.0, 0.0
	if price > 0 {
		limitBuy = math.Min(openValueLimit, math.Min(sm.availableU/5, sm.accountValueInU/15)) / price
		availableBuy = sm.availableU / price
	}
	carryStatus = &CarryStatus{isSpot: true, market: setting.Market, symbol: setting.Symbol, account: account,
		setting:       setting,
		LimitSell:     0,
		LimitBuy:      limitBuy,
		AvailableSell: 0,
		AvailableBuy:  availableBuy,
		TradeLineBuy:  setting.OpenShortMargin,
		TradeLineSell: setting.CloseShortMargin,
	}
	if sm.balances[setting.Symbol] != nil {
		balance := sm.balances[setting.Symbol]
		if price > 0 {
			limitSell = math.Min(math.Min(math.Max(balance.Amount, 0), balance.AvailableWithBorrow), openValueLimit/price)
		}
		carryStatus.Holding = balance.Amount
		// 暂不支持借币
		carryStatus.LimitSell = limitSell
		carryStatus.RateInAll = math.Abs(carryStatus.Holding * price / sm.accountValueInU)
		carryStatus.AvailableSell = carryStatus.LimitSell
	}
	usdLowLine := math.Min(100000, 0.1*sm.accountValueInU)
	if sm.availableU < usdLowLine || carryStatus.RateInAll > 0.6 {
		doRevert = true
	}
	if sm.balances[setting.Symbol] != nil && math.Abs(sm.balances[setting.Symbol].UsdValue) > valueLimit {
		doRevert = true
	}
	if sm.collateral != nil && (sm.collateral.Rate < 10 ||
		(sm.collateral.Available-sm.collateral.Occupied)/sm.collateral.Available < 0.1) {
		doRevert = true
	}
	return carryStatus, doRevert
}

func initStatus(account *model.Account, setting *model.Setting) (status *CarryStatus) {
	if setting == nil {
		return
	}
	_, marketType, _, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	fundingRate := 0.0
	doRevert := false
	localLimit := holdingLimitInU
	account0 := model.AppConfig.GetAccounts(setting.Market)[0]
	if account0.Key != account.Key {
		localLimit /= 10
	}
	if marketType == model.MarketTypeSpot {
		status, doRevert = createFromBalance(account, setting, localLimit)
	} else if marketType == model.MarketTypePerp {
		status, doRevert = createFromPosition(account, setting, localLimit)
		_, fundingRate = api.GetFundingRate(account.Key, account.Secret, setting.Market, setting.Symbol)
		fundingRate *= 0.9
	}
	if statuses == nil || status == nil {
		return
	}
	marketInfo := model.GetMarketInfo(setting.Market, setting.Symbol)
	if marketInfo != nil && marketInfo.SizeMax > 0 {
		_, amount := model.ParseRealAmount(setting.Market, setting.Symbol, marketInfo.SizeMax)
		status.LimitBuy = math.Min(status.LimitBuy, amount)
		status.LimitSell = math.Min(status.LimitSell, amount)
		status.AvailableBuy = math.Min(status.AvailableBuy, amount)
		status.AvailableSell = math.Min(status.AvailableSell, amount)
	}
	if setting.Market == model.OKEX {
		success, maxBuy, maxSell := api.GetTradeMaxOKEX(account.Key, account.Secret, setting.Symbol, 600)
		if success {
			status.LimitBuy = math.Min(status.LimitBuy, maxBuy)
			status.LimitSell = math.Min(status.LimitSell, maxSell)
			status.AvailableBuy = math.Min(status.AvailableBuy, maxBuy)
			status.AvailableSell = math.Min(status.AvailableSell, maxSell)
		}
	}
	setCarryStatus(setting.Coin, setting.Market, setting.Symbol, account.Key, status)
	jump := 8.0
	revertJump := 3.0
	if status.Holding > 0 {
		status.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*status.RateInAll), lowestScore) + fundingRate
		status.TradeLineSell = math.Max(setting.CloseShortMargin*(0.5-revertJump*status.RateInAll), lowestScore) - fundingRate
	} else if status.Holding == 0 {
		status.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5-jump*status.RateInAll), lowestScore) + fundingRate
		status.TradeLineSell = math.Max(setting.CloseShortMargin*(0.5+jump*status.RateInAll), lowestScore) - fundingRate
	} else if status.Holding < 0 {
		status.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5-revertJump*status.RateInAll), lowestScore) + fundingRate
		status.TradeLineSell = math.Max(setting.CloseShortMargin*(0.5+jump*status.RateInAll), lowestScore) - fundingRate
	}
	status.TradeLineBuy *= account.CarryRate
	status.TradeLineSell *= account.CarryRate
	if doRevert || account.CarryClose {
		if status.Holding >= 0 {
			status.TradeLineBuy = 1
		}
		if status.Holding <= 0 {
			status.TradeLineSell = 1
		}
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
		util.Notice(`...... enter clearing cross all`)
		coinSettings := model.GetCoinSettings(model.FunctionCross)
		waitEqual := make(map[int]bool)
		equalChannel := make(chan int, 1)
		markets := model.GetMarkets()
		for i := 0; i < model.AppConfig.GetCrossLen(); i++ {
			accounts := make(map[string]*model.Account)
			indexAccounts := model.GetAccounts(i)
			for _, market := range markets {
				accounts[market] = indexAccounts[market]
			}
			needEqual := false
			if lastCrosses == nil {
				needEqual = true
			} else {
				for _, account := range accounts {
					if getLastCrosses(account.Key) != nil {
						needEqual = true
						break
					}
				}
			}
			if !needEqual && time.Now().Minute()%10 != 0 {
				util.Notice(`...... no change pass make equal`)
				continue
			}
			waitEqual[i] = true
			go equalAccount(i, equalChannel, accounts, coinSettings)
		}
		for true {
			index := <-equalChannel
			waitEqual[index] = false
			finish := true
			for _, value := range waitEqual {
				if value == true {
					finish = false
				}
			}
			if finish {
				break
			}
		}
		util.Notice(`...... exit clearing cross all`)
		checkSetCrossing(false)
		time.Sleep(time.Minute * 2)
	}
}

func equalAccount(i int, equalChan chan int, accounts map[string]*model.Account, coinSettings map[string][]*model.Setting) {
	needEqual := false
	keys := ``
	for _, account := range accounts {
		spotMarkets.Delete(account.Key)
		contractMarkets.Delete(account.Key)
		keys += account.Key + `,`
	}
	util.Notice(`...... enter clearing cross %d %s`, i, keys)
	for coin, settings := range coinSettings {
		equalStatuses := make([]*CarryStatus, len(settings))
		for j, setting := range settings {
			account := accounts[setting.Market]
			if setting == nil || len(coin) == 0 || coin != setting.Symbol[0:len(coin)] || account == nil {
				util.Notice(`can not equal`)
				continue
			}
			equalStatuses[j] = initStatus(account, setting)
		}
		coinEqual, _ := equalCoin(coin, equalStatuses)
		if coinEqual == false {
			needEqual = true
		}
	}
	if !needEqual {
		for _, account := range accounts {
			setLastCrosses(account.Key, nil)
		}
	}
	equalChan <- i
	util.Notice(`...... exit clearing cross %d`, i)
}

// bybit 缺少按照symbol cancel all
// settings []*model.Setting, coinStatus map[string]map[string]map[string]*CarryStatus
func equalCoin(coin string, statuses []*CarryStatus) (isEqual bool, msg string) {
	var holding, price float64
	orderSide := ``
	var equalStatus *CarryStatus
	bids := model.Ticks{}
	asks := model.Ticks{}
	bidStatus := make(map[string]*CarryStatus)
	askStatus := make(map[string]*CarryStatus)
	tickTimes := make(map[string]int) // market_symbol_ts
	isEqual = true
	holdStr := ``
	for _, status := range statuses {
		if status == nil {
			util.Notice(`warning: fail to get one status %s`, coin)
			return false, `fail to equal for one nil status`
		}
		holding += status.Holding
		holdStr += fmt.Sprintf(`[%s %s %f]`, status.market, status.symbol, status.Holding)
		getTick, tick := model.AppMarkets.GetBidAsk(status.symbol, status.market)
		getFunding, rate := api.GetFundingRate(status.account.Key, status.account.Secret, status.market, status.symbol)
		if !getTick || !getFunding {
			return false, fmt.Sprintf(`no tick or funding rate when equal %s %s`, status.market, status.symbol)
		}
		tickTimes[status.market+status.symbol] = tick.Ts
		bids = append(bids, model.Tick{Market: tick.Bids[0].Market, Symbol: tick.Bids[0].Symbol,
			Amount: tick.Bids[0].Amount, Price: tick.Bids[0].Price * (1 + rate)})
		asks = append(asks, model.Tick{Market: tick.Asks[0].Market, Symbol: tick.Asks[0].Symbol,
			Amount: tick.Asks[0].Amount, Price: tick.Asks[0].Price * (1 + rate)})
		bidStatus[fmt.Sprintf(`%s_%s`, status.market, status.symbol)] = status
		askStatus[fmt.Sprintf(`%s_%s`, status.market, status.symbol)] = status
		if price == 0 {
			price = bids[0].Price + asks[0].Price
		}
	}
	holdingInU := holding * price
	if math.Abs(holdingInU) < 10 {
		if time.Now().Minute()%50 == 0 {
			//util.Notice(fmt.Sprintf(`clear holding every 50 mins %s %f %f %f`, coin, holding, price, holdingInU))
			for _, status := range statuses {
				go api.CancelOrders(status.account.Key, status.account.Secret, status.market, status.symbol)
			}
		}
	} else {
		isEqual = false
	}
	now := util.GetNowUnixMillion()
	if holdingInU > 10 {
		orderSide = model.OrderSideSell
		sort.Sort(sort.Reverse(bids))
		util.Notice(fmt.Sprintf(`need equal %s sell holding %f worth %f list %s %v`,
			coin, holding, holdingInU, holdStr, bids))
		for i := 0; i < len(bids); i++ {
			status := bidStatus[fmt.Sprintf(`%s_%s`, bids[i].Market, bids[i].Symbol)]
			if status == nil {
				util.Notice(fmt.Sprintf(`no status when holding in U: %f`, holdingInU))
				continue
			}
			go api.CancelOrders(status.account.Key, status.account.Secret, status.market, status.symbol)
			if equalStatus != nil || now-int64(tickTimes[status.market+status.symbol]) > 300 {
				continue
			}
			if status.AvailableSell > holding {
				equalStatus = status
			} else {
				checkAmount := model.GetAmountInMarket(status.market, status.symbol, status.AvailableSell, bids[i].Price)
				if checkAmount > 0 {
					equalStatus = status
					holding = status.AvailableSell
				} else {
					util.Notice(fmt.Sprintf(`check amount 0 sell %s %s %f %f`,
						status.market, status.symbol, status.AvailableSell, bids[i].Price))
				}
			}
		}
	}
	if holdingInU < -10 {
		orderSide = model.OrderSideBuy
		sort.Sort(asks)
		util.Notice(fmt.Sprintf(`need equal %s buy holding %f worth %f list %s %v`,
			coin, holding, holdingInU, holdStr, asks))
		for i := 0; i < len(asks); i++ {
			status := askStatus[fmt.Sprintf(`%s_%s`, asks[i].Market, asks[i].Symbol)]
			if status == nil {
				util.Notice(fmt.Sprintf(`no status when holding in U: %f`, holdingInU))
				continue
			}
			go api.CancelOrders(status.account.Key, status.account.Secret, status.market, status.symbol)
			if equalStatus != nil || now-int64(tickTimes[status.market+status.symbol]) > 300 {
				continue
			}
			if math.IsNaN(status.AvailableBuy) || status.AvailableBuy > math.Abs(holding) {
				equalStatus = status
			} else if !math.IsNaN(status.AvailableBuy) {
				checkAmount := model.GetAmountInMarket(status.market, status.symbol, status.AvailableBuy, asks[i].Price)
				if checkAmount > 0 {
					equalStatus = status
					holding = status.AvailableBuy
				} else {
					util.Notice(fmt.Sprintf(`check amount 0 buy %s %s %f %f`,
						status.market, status.symbol, status.AvailableBuy, asks[i].Price))
				}
			}
		}
	}
	if equalStatus != nil {
		amount := math.Abs(holding)
		if equalStatus.market == model.Ftx {
			amount = math.Min(90000000, math.Abs(holding))
		}
		checkAmount := model.GetAmountInMarket(equalStatus.market, equalStatus.symbol, amount, price)
		if checkAmount > 0 {
			time.Sleep(time.Second)
			getTick, tick := model.AppMarkets.GetBidAsk(equalStatus.symbol, equalStatus.market)
			if !getTick {
				return
			}
			if orderSide == model.OrderSideBuy {
				price = tick.Asks[0].Price
			} else if orderSide == model.OrderSideSell {
				price = tick.Bids[0].Price
			}
			util.Notice(fmt.Sprintf(`do equal %s %s %s at %f %f %f amount %f`,
				coin, equalStatus.market, equalStatus.symbol, price, tick.Asks[0].Price, tick.Bids[0].Price, amount))
			api.PlaceOrder(equalStatus.account.Key, equalStatus.account.Secret, orderSide, model.OrderTypeLimit,
				equalStatus.market, equalStatus.symbol, ``, model.FunctionComplement,
				price, price, amount, true, true, nil, nil)
			if equalStatus.market == model.Gate {
				api.SetGateBidAsk(equalStatus.account.Key, equalStatus.account.Secret, equalStatus.symbol)
			}
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
	settings := model.GetCoinSetting(setting.Function, setting.Coin)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || model.AppPause ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) || setting.Valid == false ||
		settings == nil || len(settings) == 0 || million-int64(tick.Ts) > 100 {
		return
	}
	for _, settingRelate := range settings {
		tickGet, tickRelate := model.AppMarkets.GetBidAsk(settingRelate.Symbol, settingRelate.Market)
		if !tickGet || setting.ID == settingRelate.ID ||
			(model.AppConfig.Env != `test` && million-int64(tickRelate.Ts) > 300) {
			continue
		}
		for i := model.AppConfig.GetCrossLen() - 1; i >= 0; i-- {
			account := model.AppConfig.GetAccounts(setting.Market)[i]
			accountRelate := model.AppConfig.GetAccounts(settingRelate.Market)[i]
			if account == nil || accountRelate == nil {
				continue
			}
			status := getCarryStatus(setting.Coin, setting.Market, setting.Symbol, account.Key)
			statusRelate := getCarryStatus(settingRelate.Coin, settingRelate.Market, settingRelate.Symbol, accountRelate.Key)
			if status == nil || statusRelate == nil || status == statusRelate {
				continue
			}
			statusBuy, statusSell, amount, priceBuy, priceSell := calcAmount(i, setting.Coin, status, statusRelate, tick, tickRelate)
			if amount > 0 {
				placeCross(statusBuy, statusSell, priceBuy, priceSell, amount)
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
	stopStatus, _ := carryStop.Load(carryStatus.account.Key)
	stopRelate, _ := carryStop.Load(carryStatusRelate.account.Key)
	if (stopStatus != nil && stopStatus.(bool)) || (stopRelate != nil || stopRelate.(bool)) {
		util.Debug(`stop carry for 10 times unknown carry %s or %s %s`,
			carryStatus.account.Key, carryStatusRelate.account.Key, coin)
		return
	}
	var bidAmount, askAmount float64
	priceAskRelate := tickRelate.Asks[0].Price
	priceBidRelate := tickRelate.Bids[0].Price
	priceAsk := tick.Asks[0].Price
	priceBid := tick.Bids[0].Price
	amountBidRelate := tickRelate.Bids[0].Amount
	amountAskRelate := tickRelate.Asks[0].Amount
	amountBid := tick.Bids[0].Amount
	amountAsk := tick.Asks[0].Amount
	score := 1 - priceAskRelate/priceBid
	scoreRelate := priceBidRelate/priceAsk - 1
	mark := fmt.Sprintf(`%s_%s|%s_%s`, carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol)
	if score > 0.01 {
		model.AppMetric.AddCarry(mark, score, 0)
	}
	lineAll := carryStatus.TradeLineSell + carryStatusRelate.TradeLineBuy
	if (carryStatus.TradeLineSell < score && carryStatusRelate.TradeLineBuy < score) ||
		(lineAll > 0 && score > 1.2*lineAll) || (lineAll < 0 && score > 0.4*lineAll) {
		statusSell = carryStatus
		statusBuy = carryStatusRelate
		priceSell = priceBid
		priceBuy = priceAskRelate
		askAmount = amountBid
		bidAmount = amountAskRelate
	}
	lineAll = carryStatus.TradeLineBuy + carryStatusRelate.TradeLineSell
	if (carryStatus.TradeLineBuy < scoreRelate && carryStatusRelate.TradeLineSell < scoreRelate) ||
		(lineAll > 0 && scoreRelate > 1.2*lineAll) || (lineAll < 0 && scoreRelate > 0.4*lineAll) {
		statusSell = carryStatusRelate
		statusBuy = carryStatus
		priceSell = priceBidRelate
		priceBuy = priceAsk
		askAmount = amountBidRelate
		bidAmount = amountAsk
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
		model.SetMonitorInfo(strconv.Itoa(index), `cross`, mark, []string{coin, carryStatus.market, coinValue,
			fmt.Sprintf(`%.1f`, 100*carryStatus.TradeLineBuy),
			fmt.Sprintf(`%.1f`, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatus.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatus.LimitSell),
			carryStatusRelate.market, coinValueRelate,
			fmt.Sprintf(`%.1f`, 100*carryStatusRelate.TradeLineBuy),
			fmt.Sprintf(`%.1f`, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitSell),
			fmt.Sprintf(`%.1f`, 100*scoreRelate),
			fmt.Sprintf(`%.1f`, 100*score),
			fmt.Sprintf(`%v`, statusBuy != nil && statusSell != nil)})
	} else {
		mark = fmt.Sprintf(`%s|%s`, markRelate, mark)
		model.SetMonitorInfo(strconv.Itoa(index), `cross`, mark, []string{coin, carryStatusRelate.market, coinValueRelate,
			fmt.Sprintf(`%.1f`, 100*carryStatusRelate.TradeLineBuy),
			fmt.Sprintf(`%.1f`, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitSell),
			carryStatus.market, coinValue,
			fmt.Sprintf(`%.1f`, 100*carryStatus.TradeLineBuy),
			fmt.Sprintf(`%.1f`, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatus.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatus.LimitSell),
			fmt.Sprintf(`%.1f`, 100*score),
			fmt.Sprintf(`%.1f`, 100*scoreRelate),
			fmt.Sprintf(`%v`, statusBuy != nil && statusSell != nil)})
	}
	if statusBuy == nil || statusSell == nil {
		return nil, nil, 0, 0, 0
	}
	if !isLastCross(statusBuy.account.Key, statusBuy.market, statusBuy.symbol) {
		initLimitBuyAndSell(statusBuy, statusBuy.setting, priceBuy)
	}
	if !isLastCross(statusSell.account.Key, statusSell.market, statusSell.symbol) {
		initLimitBuyAndSell(statusSell, statusSell.setting, priceSell)
	}
	amount = math.Min(math.Min(statusBuy.LimitBuy, bidAmount), math.Min(statusSell.LimitSell, askAmount))
	if amount > 0 {
		amount = model.FormatCrossPair(statusBuy.market, statusSell.market, statusBuy.symbol, statusSell.symbol, amount, priceBuy)
	}
	if (score > 0.15 || scoreRelate > 0.15) || ((score > 0.1 || scoreRelate > 0.1) &&
		(!isValidSymbol(carryStatus.market, carryStatus.symbol) ||
			!isValidSymbol(carryStatusRelate.market, carryStatusRelate.symbol))) {
		title := `不同币种`
		if score > 0.15 || scoreRelate > 0.15 {
			title = `价差不可思议`
		}
		msg := fmt.Sprintf(`different coin %s %s %s %s %f %f`, carryStatus.market, carryStatus.symbol,
			carryStatusRelate.market, carryStatusRelate.symbol, score, scoreRelate)
		minute := time.Now().Minute()
		second := time.Now().Second()
		if minute == 0 && second == 0 {
			for _, address := range model.TeamMails {
				err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, address, title, msg)
				if err != nil {
					util.Notice(`fail to send mail msg %s %s`, msg, err.Error())
				}
			}
		}
		return nil, nil, 0, 0, 0
	}
	return statusBuy, statusSell, amount, priceBuy, priceSell
}

func initLimitBuyAndSell(status *CarryStatus, setting *model.Setting, price float64) {
	if status.isSpot {
		value, ok := spotMarkets.Load(status.account.Key)
		if !ok || value == nil { // 此时正在进行每分钟的清理找平
			return
		}
		sm := value.(*spotMarket)
		status.LimitBuy = math.Min(openValueLimit, math.Min(sm.availableU/5, sm.accountValueInU/15)) / price
		balance := sm.balances[setting.Symbol]
		if balance != nil {
			status.LimitSell = math.Min(math.Min(math.Max(balance.Amount, 0), balance.AvailableWithBorrow), openValueLimit/price)
		} else {
			status.LimitSell = 0
		}
	} else {
		value, ok := contractMarkets.Load(status.account.Key)
		if value == nil || !ok {
			return
		}
		cm := value.(*contractMarket)
		limitAmount := math.Min(cm.accountValueInU/5, math.Min(cm.collateralsAvailable, openValueLimit)) / price
		availableAmount := cm.collateralsAvailable / price
		status.LimitSell = limitAmount
		status.LimitBuy = limitAmount
		status.AvailableSell = availableAmount
		status.AvailableBuy = availableAmount
	}
}

func placeCross(statusBuy, statusSell *CarryStatus, priceBuy, priceSell, amount float64) {
	if !checkSetCrossing(true) {
		defer checkSetCrossing(false)
	} else {
		return
	}
	util.Notice(fmt.Sprintf(`place cross %s %s -> %s %s at %f %f amount %f`,
		statusSell.market, statusSell.symbol, statusBuy.market, statusBuy.symbol, priceSell, priceBuy, amount))
	placeSuccess := true
	if statusBuy.market == model.OKEX && statusSell.market == model.OKEX {
		placeSuccess = api.PlacePairOKEX(statusBuy.account.Key, statusBuy.symbol, statusSell.symbol, model.OrderTypeLimit,
			model.FunctionCross, priceBuy, priceSell, amount)
	} else {
		go api.PlaceOrder(statusBuy.account.Key, statusBuy.account.Secret, model.OrderSideBuy, model.OrderTypeLimit,
			statusBuy.market, statusBuy.symbol, ``, model.FunctionCross, priceBuy, priceBuy,
			amount, true, true, PostOrderCross, statusBuy.setting)
		go api.PlaceOrder(statusSell.account.Key, statusSell.account.Secret, model.OrderSideSell, model.OrderTypeLimit,
			statusSell.market, statusSell.symbol, ``, model.FunctionCross, priceSell, priceSell,
			amount, true, true, PostOrderCross, statusSell.setting)
		time.Sleep(time.Second / 4)
	}
	if placeSuccess {
		placeStatus(statusBuy, priceBuy, amount)
		placeStatus(statusSell, priceSell, -1*amount)
	}
}

func placeStatus(status *CarryStatus, price float64, amount float64) {
	valueSpot, okSpot := spotMarkets.Load(status.account.Key)
	valueContract, okContract := contractMarkets.Load(status.account.Key)
	if !okSpot || valueSpot == nil || !okContract || valueContract == nil {
		return
	}
	sm := valueSpot.(*spotMarket)
	cm := valueContract.(*contractMarket)
	if status.isSpot {
		balance := sm.balances[status.symbol]
		if balance == nil {
			util.Notice(fmt.Sprintf(`warning no balance %s %s %s`,
				status.account.Key, status.market, status.symbol))
			balance = &model.Balance{Amount: amount, UsdValue: amount * price, Market: status.market, Coin: status.setting.Coin}
		} else {
			balance.Amount += amount
			balance.UsdValue = balance.Amount * price
			balance.AvailableWithBorrow += amount
		}
		sm.availableU -= amount * price
		if status.market == model.Ftx {
			if cm != nil {
				cm.collateralsAvailable -= amount * price
			}
		} else if status.market == model.OKEX {
			sm.collateral.Available -= amount * price
			if cm != nil {
				cm.collateralsAvailable -= amount * price
			}
		}
	} else {
		position := cm.positions[status.symbol]
		originFreeAbs := 0.0
		if position == nil {
			position = &model.Position{Holding: amount, EntryPrice: price, Market: status.market, Currency: status.setting.Symbol}
		} else {
			originFreeAbs = math.Abs(position.Holding)
			position.Holding += amount
			position.EntryPrice = price
		}
		changeU := (originFreeAbs - math.Abs(position.Holding)) * price
		cm.collateralsAvailable += changeU * 0.2
		cm.contractValueInU += changeU
		if status.market == model.Ftx {
			sm.availableU += changeU * 0.2
		} else if status.market == model.OKEX {
			sm.collateral.Available += changeU * 0.1
			sm.collateral.Occupied -= changeU * 0.1
			sm.availableU += changeU * 0.1
		}
	}
	account := model.AppConfig.GetAccountFromKey(status.market, status.account.Key)
	initStatus(account, status.setting)
	setLastCross(account.Key, status.market, status.symbol)
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
		//if account != nil {
		//	status := getCarryStatus(setting.Coin, setting.Market, setting.Symbol, account.Key)
		//	placeStatus(status, order.Price, order.Amount)
		//maxBuy := status.LimitBuy
		//maxSell := status.LimitSell
		//if order.OrderSide == model.OrderSideBuy {
		//	maxBuy -= order.Amount
		//	maxSell += order.Amount
		//} else if order.OrderSide == model.OrderSideSell {
		//	maxBuy += order.Amount
		//	maxSell -= order.Amount
		//}
		//status.LimitSell = math.Min(status.LimitSell, maxSell)
		//status.LimitBuy = math.Min(status.LimitBuy, maxBuy)
		//}
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
			case model.BinancePerp, model.BinanceSpot:
				if strings.Contains(InsufficientCodeBinance, order.ErrCode) {
					util.Notice(`reset binance trade max with %s %s`, order.ErrCode, order.AmountType)
					spotMarkets.Delete(order.AmountType)
					contractMarkets.Delete(order.AmountType)
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

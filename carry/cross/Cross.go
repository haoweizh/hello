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
	"sync"
	"time"
)

func createContractMarket(key, secret, market string) (cm *contractMarket) {
	success, positions, accountValue, availableU := api.GetPositions(key, secret, market)
	util.Notice(fmt.Sprintf(`get positions %s %s %v account value %f available u %f`,
		market, key, success, accountValue, availableU))
	settings := api.GetSettings(model.FunctionCross, market)
	if success {
		cm = &contractMarket{key: key, market: market}
		cm.positions = make(map[string]*api.Position)
		for _, position := range positions {
			cm.positions[position.Currency] = position
			if settings != nil {
				value, ok := settings.Load(position.Currency)
				if ok && value != nil {
					_, price := api.GetPriceForce(position.Currency, market)
					if price > 0 {
						cm.contractValueInU += price * math.Abs(position.Holding)
					} else {
						cm.contractValueInU += position.EntryPrice * math.Abs(position.Holding)
					}
				} else if math.Abs(position.Holding) > 0 {
					util.Info(fmt.Sprintf(`holding absent pos %s %s %f`, market, position.Currency, position.Holding))
				}
			}
		}
		cm.accountValueInU = accountValue
		cm.collateralsAvailable = availableU
	} else {
		util.Notice(`fail to createContractMarket %s %s`, market, key)
		return nil
	}
	return
}

func createSpotMarket(key, secret, market string) (sm *spotMarket) {
	//util.Info(fmt.Sprintf(`create sm %s %s`, key[:5], market))
	success, balances, totalInUsd, collateral := api.GetBalances(key, secret, market)
	if success {
		sm = &spotMarket{key: key, market: market}
		sm.balances = make(map[string]*model.Balance)
		sm.accountValueInU = totalInUsd
		sm.collateral = collateral
		settings := api.GetSettings(model.FunctionCross, market)
		for _, balance := range balances {
			symbol := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
			sm.balances[symbol] = balance
			if strings.EqualFold(balance.Coin, `usd`) || strings.EqualFold(balance.Coin, `usdt`) {
				sm.availableU += math.Min(balance.Amount, balance.AvailableWithBorrow)
			}
			// 可用usd数量需要减去现有所有借币负债总额
			if balance.UsdValue < 0 {
				sm.availableU -= math.Abs(balance.UsdValue)
			}
			if settings != nil {
				value, ok := settings.Load(symbol)
				if (!ok || value == nil) && balance.Amount > 0 {
					util.Info(fmt.Sprintf(`holding absent bal %s %s %f`, market, symbol, balance.Amount))
				}
			}
		}
	} else {
		util.Notice(`fail to createSpotMarket %s %s`, market, key)
		return nil
	}
	return
}

// absentRevert: 当cm或sm中没有这个symbol时，是否设置成revert模式
func createFromPosition(account *model.Account, setting *model.Setting, valueLimit float64, absentRevert bool) (carryStatus *CarryStatus, doRevert bool) {
	key := account.Key
	value, ok := contractMarkets.Load(key)
	if value == nil || !ok {
		contractMarkets.Store(key, createContractMarket(key, account.Secret, setting.Market))
		value, _ = contractMarkets.Load(key)
		spotValue, spotOk := spotMarkets.Load(key)
		if account.IsUnified && (spotValue == nil || !spotOk) {
			spotMarkets.Store(key, createSpotMarket(key, account.Secret, setting.Market))
		}
	}
	if value == nil {
		util.NoticeLess(fmt.Sprintf(`nil contract market %s %s`, setting.Market, setting.Symbol))
		return nil, false
	}
	cm := value.(*contractMarket)
	getPrice, price := api.GetPriceForce(setting.Symbol, setting.Market)
	if !getPrice || price == 0 {
		//price = cm.positions[setting.Symbol].EntryPrice
		//util.Notice(`no tick price, use position price %s %s %f`, setting.Market, setting.Symbol, price)
		return nil, false
	}
	limitAmount := 0.0
	availableAmount := 0.0
	limitAmount = math.Min(cm.accountValueInU/5, cm.collateralsAvailable) / price
	availableAmount = cm.collateralsAvailable / price
	carryStatus = &CarryStatus{isSpot: false, market: setting.Market, symbol: setting.Symbol, account: account,
		setting:       setting,
		LimitSell:     limitAmount,
		LimitBuy:      limitAmount,
		AvailableSell: availableAmount,
		AvailableBuy:  availableAmount}
	valueInUsd := 0.0
	if cm.positions[setting.Symbol] != nil {
		carryStatus.Holding = cm.positions[setting.Symbol].Holding
		valueInUsd = math.Abs(carryStatus.Holding) * price
		carryStatus.RateInAll = valueInUsd / cm.accountValueInU
	} else {
		if absentRevert {
			//doRevert = true
		}
		//util.Info(fmt.Sprintf(`symbol absent revert %s %s`, setting.Market, setting.Symbol))
	}
	// bitgetperp可以开仓或减仓，不越过0反向开仓
	if setting.Market == model.BitgetPerp && cm.positions[setting.Symbol] != nil {
		if cm.positions[setting.Symbol].Holding > 0 {
			carryStatus.reduceOnlySell = true
			carryStatus.LimitSell = cm.positions[setting.Symbol].Holding
		} else if cm.positions[setting.Symbol].Holding < 0 {
			carryStatus.reduceOnlyBuy = true
			carryStatus.LimitBuy = -1 * cm.positions[setting.Symbol].Holding
		}
	}
	rateLimitPosition := 2.8
	rateLimitHolding := 0.28
	if setting.Market == model.Gate {
		rateLimitPosition = 1.3
	}
	if cm.contractValueInU/cm.accountValueInU > rateLimitPosition || valueInUsd > valueLimit ||
		valueInUsd/cm.accountValueInU > rateLimitHolding ||
		(setting.Market == model.BitgetPerp && len(cm.positions) > BitgetPosLimit) {
		doRevert = true
	}
	return carryStatus, doRevert
}

// absentRevert: 当cm或sm中没有这个symbol时，是否设置成revert模式
func createFromBalance(account *model.Account, setting *model.Setting, valueLimit float64, absentRevert bool) (carryStatus *CarryStatus, doRevert bool) {
	key := account.Key
	value, ok := spotMarkets.Load(key)
	if value == nil || !ok {
		spotMarkets.Store(key, createSpotMarket(key, account.Secret, setting.Market))
		value, ok = spotMarkets.Load(key)
	}
	success, price := api.GetPriceForce(setting.Symbol, setting.Market)
	if value == nil || !success || price == 0 {
		util.NoticeLess(fmt.Sprintf(`nil spot market %s %s getPrice %v %f`, setting.Market, setting.Symbol, success, price))
		return nil, true
	}
	sm := value.(*spotMarket)
	limitBuy, limitSell, availableBuy := 0.0, 0.0, 0.0
	if setting.Function == model.FunctionCross {
		limitBuy = math.Min(sm.availableU/5, sm.accountValueInU/15) / price
	} else if setting.Function == model.FunctionQueue {
		limitBuy = sm.availableU * 0.9 / price
	}
	availableBuy = sm.availableU / price
	carryStatus = &CarryStatus{isSpot: true, market: setting.Market, symbol: setting.Symbol, account: account,
		setting:       setting,
		LimitSell:     0,
		LimitBuy:      limitBuy,
		AvailableSell: 0,
		AvailableBuy:  availableBuy}
	if sm.balances[setting.Symbol] != nil {
		balance := sm.balances[setting.Symbol]
		limitSell = math.Min(math.Max(balance.Amount, 0), balance.AvailableWithBorrow)
		carryStatus.Holding = balance.Amount
		// 暂不支持借币
		carryStatus.LimitSell = limitSell
		carryStatus.RateInAll = math.Abs(carryStatus.Holding * price / sm.accountValueInU)
		carryStatus.AvailableSell = carryStatus.LimitSell
	} else {
		if absentRevert {
			//doRevert = true
		}
		// warning: bybit现货偶发出现实际持有某个币种，但是sm.balances中没有该币种
		//if setting.Market == model.Bybit {
		//	util.Info(fmt.Sprintf(`symbol absent revert %s %s %d`, setting.Market, setting.Symbol, len(sm.balances)))
		//	return nil, true
		//}
	}
	usdLowLine := math.Min(100000, 0.2*sm.accountValueInU)
	if sm.availableU < usdLowLine || carryStatus.RateInAll > 0.2 {
		doRevert = true
	}
	if sm.balances[setting.Symbol] != nil && math.Abs(sm.balances[setting.Symbol].UsdValue) > valueLimit {
		doRevert = true
	}
	if setting.Market == model.OKEX && sm.collateral != nil && sm.collateral.Rate < 10 {
		//(sm.collateral.Available-sm.collateral.Occupied)/sm.collateral.Available < 0.1) {
		doRevert = true
	}
	return carryStatus, doRevert
}

// absentRevert: 当cm或sm中没有这个symbol时，是否设置成revert模式
func initStatus(account *model.Account, setting *model.Setting, absentRevert bool) (status *CarryStatus) {
	if setting == nil {
		return
	}
	_, marketType, _, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	doRevert := false
	localLimit := holdingLimitInU
	account0 := model.AppConfig.GetAccounts(setting.Market)[0]
	if account0.Key != account.Key {
		localLimit /= 10
	}
	if marketType == model.MarketTypeSpot {
		status, doRevert = createFromBalance(account, setting, localLimit, absentRevert)
	} else if marketType == model.MarketTypePerp {
		status, doRevert = createFromPosition(account, setting, localLimit, absentRevert)
	}
	if setting.Chance < 0 {
		doRevert = true
	}
	if status == nil {
		util.NoticeLess(fmt.Sprintf(`fail to create status %s %s`, setting.Market, setting.Symbol))
		return nil
	}
	getFundingRate, rate := api.GetFundingRate(account.Key, account.Secret, setting.Market, setting.Symbol)
	if getFundingRate {
		status.FundingRate = rate.Rate
		status.FundingRateUpdateTime = rate.UpdateTime
	}
	fundingKey := fmt.Sprintf(`funding_%s_%s`, setting.Market, setting.Symbol)
	fundingTime, ok := notifyTime.Load(fundingKey)
	if !(ok && fundingTime.(time.Time).Add(time.Minute*60).After(time.Now())) && math.Abs(status.FundingRate) > 0.03 {
		notifyTime.Store(fundingKey, time.Now())
		go func() {
			msg := fmt.Sprintf(`%s %f`, fundingKey, status.FundingRate)
			err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, `haoweizh@qq.com`, msg, msg)
			if err != nil {
				util.Notice(`fail to send mail msg %s %s`, msg, err.Error())
			}
		}()
	}
	v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
	var marketInfo *model.MarketInfo
	if v != nil {
		marketInfo = v.(*model.MarketInfo)
	}
	if marketInfo != nil && marketInfo.SizeMax > 0 {
		_, amount := model.ParseRealAmount(setting.Market, setting.Symbol, marketInfo.SizeMax)
		status.AvailableBuy = math.Min(status.AvailableBuy, amount)
		status.AvailableSell = math.Min(status.AvailableSell, amount)
		if status.market == model.Mexc { // mexc要求持仓不能超过1500张合约
			status.AvailableBuy = math.Min(status.AvailableBuy, 1400*marketInfo.SizeIncrement-status.Holding)
			status.AvailableSell = math.Min(status.AvailableSell, 1400*marketInfo.SizeIncrement+status.Holding)
		}
	}
	if setting.Market == model.OKEX {
		success, maxBuy, maxSell := api.GetTradeMaxOKEX(account.Key, account.Secret, setting.Symbol, 600)
		if success {
			status.AvailableBuy = math.Min(status.AvailableBuy, maxBuy)
			status.AvailableSell = math.Min(status.AvailableSell, maxSell)
		}
	}
	status.LimitBuy = math.Min(status.LimitBuy, status.AvailableBuy)
	status.LimitSell = math.Min(status.LimitSell, status.AvailableSell)
	initTradeLine(account, setting, status, doRevert)
	util.StoreSyncMap(carryStatusMap, status, setting.Coin, setting.Market, setting.Symbol, account.Key)
	return
}

func initTradeLine(account *model.Account, setting *model.Setting, status *CarryStatus, doRevert bool) {
	standardScoreBuy := math.Max(standardScoreOpen, setting.OpenShortMargin)
	standardScoreSell := math.Max(standardScoreOpen, setting.OpenShortMargin)
	getTick, ticks := model.AppEnvironment.GetBidAsk(setting.Market, setting.Symbol)
	price := 0.0
	if getTick {
		price = ticks.Asks[0].Price
	} else {
		util.NoticeLess(`fail to get ticket %s %s`, setting.Market, setting.Symbol)
	}
	jumpOpen := 80.0
	jumpClose := -20.0
	jumpBuy := jumpOpen
	jumpSell := jumpOpen
	if status.Holding*price < -100 {
		jumpBuy = jumpClose
		jumpSell = jumpOpen
		standardScoreBuy = setting.CloseShortMargin
		status.LimitBuy = math.Min(status.LimitBuy, math.Abs(status.Holding))
	} else if status.Holding*price > 100 {
		jumpBuy = jumpOpen
		jumpSell = jumpClose
		standardScoreSell = setting.CloseShortMargin
		status.LimitSell = math.Min(status.LimitSell, status.Holding)
	}
	status.TradeLineBuy = math.Max(standardScoreBuy*(0.5+jumpBuy*status.RateInAll), lowestScore) + status.FundingRate
	status.TradeLineSell = math.Max(standardScoreSell*(0.5+jumpSell*status.RateInAll), lowestScore) - status.FundingRate
	status.TradeLineBuy *= account.CarryRate
	status.TradeLineSell *= account.CarryRate
	//tradeLineExtra := getTradeLineExtra(setting.Coin, setting.CloseShortMargin)
	//if tradeLineExtra != nil {
	//	status.TradeLineBuy += tradeLineExtra.buyExtra
	//	status.TradeLineSell += tradeLineExtra.sellExtra
	//} else {
	//	util.Notice(fmt.Sprintf(`fatal error fail to get trade line extra %s`, setting.Coin))
	//}
	if doRevert || account.CarryClose {
		if status.Holding > 0 {
			status.TradeLineBuy = 1
			status.LimitBuy = 0
			status.AvailableBuy = 0
			//status.TradeLineSell = math.Min(status.TradeLineSell, 0.0004)
		} else if status.Holding < 0 {
			status.TradeLineSell = 1
			status.LimitSell = 0
			status.AvailableSell = 0
			//status.TradeLineBuy = math.Min(status.TradeLineBuy, 0.0004)
		} else if status.Holding == 0 {
			status.TradeLineBuy = 1
			status.TradeLineSell = 1
			status.LimitBuy = 0
			status.LimitSell = 0
			status.AvailableBuy = 0
			status.AvailableSell = 0
		}
	}
}

func ClearCross() {
	for doCross {
		for {
			if !api.CheckSetProcessing(model.FunctionCross, model.FunctionCross, model.FunctionCross, true) {
				break
			} else {
				time.Sleep(time.Millisecond * 10)
			}
		}
		for {
			leftOrders := 0
			model.AppEnvironment.CrossOrders.Range(func(k, v interface{}) bool {
				leftOrders++
				return true
			})
			if leftOrders > 0 {
				util.Notice(`left orders is %d`, leftOrders)
			} else {
				break
			}
			time.Sleep(time.Second * 10)
		}
		today := util.GetNow()
		today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
		carryRows, _ := model.AppDB.Model(model.Order{}).Select(`sum(price*abs(amount)),refresh_type`).
			Where(`order_time>?`, today).Group(`refresh_type`).Rows()
		var compInU, crossInU float64
		for carryRows.Next() {
			var amountInU float64
			var refreshType string
			_ = carryRows.Scan(&amountInU, &refreshType)
			if refreshType == model.FunctionComplement {
				compInU = amountInU
			} else if refreshType == model.FunctionCross {
				crossInU = amountInU
			}
		}
		err := carryRows.Close()
		if err != nil {
			continue
		}
		msg := fmt.Sprintf(`comp %f cross %f`, compInU, crossInU)
		util.Notice(msg)
		if model.AppConfig.Handle == `1` {
			if (compInU > 55000 && compInU/(compInU+crossInU) > 0.33) && model.AppConfig.Equal != `true` {
				model.AppConfig.Handle = `0`
				title := `comp too much set handle 0`
				util.Notice(title)
				api.SendMails(title, msg)
			} else {
				equalAccounts()
			}
		}
		api.CheckSetProcessing(model.FunctionCross, model.FunctionCross, model.FunctionCross, false)
		time.Sleep(time.Hour * 1)
	}
}

func equalAccounts() {
	util.Notice(`...... enter clearing cross all`)
	waitEqual := make(map[int]bool)
	equalChannel := make(chan int, 1)
	markets := api.GetMarkets()
	//needWaitEqual := false // 是否需要进入等待环节
	for i := 0; i < api.GetCrossLen(); i++ {
		accounts := make(map[string]*model.Account)
		indexAccounts := model.GetAccounts(i)
		for _, market := range markets {
			accounts[market] = indexAccounts[market]
		}
		waitEqual[i] = true
		go equalAccount(i, equalChannel, accounts)
	}
	for {
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
}

func equalAccount(i int, equalChan chan int, accounts map[string]*model.Account) {
	if accounts[model.BitgetPerp] != nil {
		liquidateBitgetPerp(accounts[model.BitgetPerp])
	}
	keys := ``
	for _, account := range accounts {
		if account.Index != i {
			continue
		}
		spotMarkets.Delete(account.Key)
		contractMarkets.Delete(account.Key)
		keys += account.Key + `,`
	}
	value := api.GetCoinSettings(model.FunctionCross)
	if value != nil {
		value.Range(func(coin, settings interface{}) bool {
			equalStatuses := make([]*CarryStatus, len(settings.([]*model.Setting)))
			for j, setting := range settings.([]*model.Setting) {
				account := accounts[setting.Market]
				if setting == nil || len(coin.(string)) == 0 || coin != setting.Symbol[0:len(coin.(string))] || account == nil {
					util.Notice(`can not equal`)
					continue
				}
				equalStatuses[j] = initStatus(account, setting, false)
			}
			for index := 0; index <= 10; index++ {
				coinEqual, leftHoldingInU, _ := equalCoin(coin.(string), equalStatuses)
				if index > 0 {
					util.Info(`equal coin %s account %d equal %v left hold u %f`, coin, i, coinEqual, leftHoldingInU)
				}
				if math.Abs(leftHoldingInU) < SmallInU || coinEqual {
					break
				}
				if index == 10 {
					api.SendMails(fmt.Sprintf(`fail equal after 10 time %s`, coin),
						fmt.Sprintf(`%s holding %f`, coin, leftHoldingInU))
				}
			}
			return true
		})
	}
	lastCrosses = sync.Map{}
	equalChan <- i
	util.Notice(`...... exit clearing cross %d`, i)
}

// bybit 缺少按照symbol cancel all
// settings []*model.Setting, coinStatus map[string]map[string]map[string]*CarryStatus
func equalCoin(coin string, statuses []*CarryStatus) (isEqual bool, holdingInU float64, msg string) {
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
	noTicks := ``
	for _, status := range statuses {
		if status == nil {
			util.NoticeLess(`warning: fail to get one status %s`, coin)
			return false, 0, `fail to equal for one nil status`
		}
		holding += status.Holding
		holdStr += fmt.Sprintf(`[%s %s %f]`, status.market, status.symbol, status.Holding)
		getTick, tick := model.AppEnvironment.GetBidAsk(status.market, status.symbol)
		getFunding, rate := api.GetFundingRate(status.account.Key, status.account.Secret, status.market, status.symbol)
		if !getTick || !getFunding || rate == nil {
			noTicks += coin + status.market
			continue
		}
		tickTimes[status.market+status.symbol] = tick.Ts
		bids = append(bids, model.Tick{Market: tick.Bids[0].Market, Symbol: tick.Bids[0].Symbol,
			Amount: tick.Bids[0].Amount, Price: tick.Bids[0].Price * (1 + rate.Rate)})
		asks = append(asks, model.Tick{Market: tick.Asks[0].Market, Symbol: tick.Asks[0].Symbol,
			Amount: tick.Asks[0].Amount, Price: tick.Asks[0].Price * (1 + rate.Rate)})
		bidStatus[fmt.Sprintf(`%s_%s`, status.market, status.symbol)] = status
		askStatus[fmt.Sprintf(`%s_%s`, status.market, status.symbol)] = status
		if price == 0 {
			price = bids[0].Price
		}
	}
	holdingInU = holding * price
	if math.Abs(holdingInU) < SmallInU {
		if time.Now().Hour() == 10 && time.Now().Minute()%50 == 0 {
			//util.Notice(fmt.Sprintf(`clear holding every 10:50 %s %f %f %f`, coin, holding, price, holdingInU))
			for _, status := range statuses {
				if status.setting == nil || status.setting.Function == model.FunctionQueue {
					continue
				}
				time.Sleep(time.Millisecond * 100)
				go api.CancelOrders(status.account.Key, status.account.Secret, status.market, status.symbol)
			}
		}
	} else {
		isEqual = false
		if math.Abs(holdingInU) > compTooBig {
			coinSettings := api.GetCoinSettings(model.FunctionCross)
			if coinSettings != nil {
				value, _ := coinSettings.Load(coin)
				if value != nil {
					for _, setting := range value.([]*model.Setting) {
						if model.AppConfig.Equal != `true` {
							setting.Valid = false
						}
						util.Notice(fmt.Sprintf(`too big comp %s %s %f %f %s`,
							setting.Market, setting.Symbol, holdingInU, holding, holdStr))
					}
				}
			}
			api.SendMails(`too big equal`, fmt.Sprintf(`%s holding in u %f`, coin, holdingInU))
		}
	}
	errMsg := ``
	now := util.GetNowUnixMillion()
	if holdingInU > SmallInU {
		orderSide = model.OrderSideSell
		sort.Sort(sort.Reverse(bids))
		util.Notice(fmt.Sprintf(`need equal no tick %s, %s sell holding %f worth %f list %s %v`,
			coin, noTicks, holding, holdingInU, holdStr, bids))
		for i := 0; i < len(bids); i++ {
			status := bidStatus[fmt.Sprintf(`%s_%s`, bids[i].Market, bids[i].Symbol)]
			if status == nil {
				util.Notice(fmt.Sprintf(`no status when holding in U: %f %s %s`, holdingInU, bids[i].Market, bids[i].Symbol))
				continue
			}
			if status.setting != nil && status.setting.Function != model.FunctionQueue {
				util.Notice(fmt.Sprintf(`equal cancel orders %s %s`, status.market, status.symbol))
				go api.CancelOrders(status.account.Key, status.account.Secret, status.market, status.symbol)
				if now-int64(tickTimes[status.market+status.symbol]) > 10000 || status.TradeLineSell > 0.5 {
					errMsg += fmt.Sprintf(`%s %s delay too long or trade line sell %d %f`,
						status.market, status.symbol, now-int64(tickTimes[status.market+status.symbol]), status.TradeLineSell)
					continue
				}
			}
			checkAmount := model.GetAmountInMarket(status.market, status.symbol, math.Abs(holding), price, status.reduceOnlySell)
			if checkAmount <= 0 {
				errMsg += fmt.Sprintf(`check amount %s %s %f < 0`, status.market, status.symbol, checkAmount)
				continue
			}
			if status.AvailableSell > holding {
				equalStatus = status
			} else {
				checkAmount = model.GetAmountInMarket(status.market, status.symbol, status.AvailableSell/2, price, status.reduceOnlySell)
				if checkAmount > 0 && status.AvailableSell*price > SmallInU {
					equalStatus = status
					holding = status.AvailableSell
				} else {
					errMsg += fmt.Sprintf(`check amount 0 sell %s %s %f %f checked amount %f`,
						status.market, status.symbol, status.AvailableSell, bids[i].Price, checkAmount)
				}
			}
		}
	}
	if holdingInU < -SmallInU {
		orderSide = model.OrderSideBuy
		sort.Sort(asks)
		util.Notice(fmt.Sprintf(`need equal no tick %s, %s buy holding %f worth %f list %s %v`,
			coin, noTicks, holding, holdingInU, holdStr, asks))
		for i := 0; i < len(asks); i++ {
			status := askStatus[fmt.Sprintf(`%s_%s`, asks[i].Market, asks[i].Symbol)]
			if status == nil {
				util.Notice(fmt.Sprintf(`no status when holding in U: %f %s %s`, holdingInU, asks[i].Market, asks[i].Symbol))
				continue
			}
			if status.setting != nil && status.setting.Function != model.FunctionQueue {
				util.Notice(fmt.Sprintf(`equal cancel orders %s %s`, status.market, status.symbol))
				go api.CancelOrders(status.account.Key, status.account.Secret, status.market, status.symbol)
				if equalStatus != nil || now-int64(tickTimes[status.market+status.symbol]) > 10000 || status.TradeLineBuy > 0.5 {
					errMsg += fmt.Sprintf(`%s %s delay too long or trade line buy %d %f`,
						status.market, status.symbol, now-int64(tickTimes[status.market+status.symbol]), status.TradeLineBuy)
					continue
				}
			}
			checkAmount := model.GetAmountInMarket(status.market, status.symbol, math.Abs(holding), price, status.reduceOnlyBuy)
			if checkAmount <= 0 {
				errMsg += fmt.Sprintf(`check amount %s %s %f < 0`, status.market, status.symbol, checkAmount)
				continue
			}
			if math.IsNaN(status.AvailableBuy) || status.AvailableBuy > math.Abs(holding) {
				equalStatus = status
			} else if !math.IsNaN(status.AvailableBuy) {
				checkAmount = model.GetAmountInMarket(status.market, status.symbol, status.AvailableBuy/2, price, status.reduceOnlyBuy)
				if checkAmount > 0 && status.AvailableBuy*price > SmallInU {
					equalStatus = status
					holding = status.AvailableBuy
				} else {
					errMsg += fmt.Sprintf(`check amount 0 buy %s %s %f %f checked amount %f`,
						status.market, status.symbol, status.AvailableBuy, asks[i].Price, checkAmount)
				}
			}
		}
	}
	if equalStatus != nil {
		amount := math.Abs(holding)
		if equalStatus.market == model.Ftx {
			amount = math.Min(90000000, math.Abs(holding))
		}
		amount = math.Min(amount, compLimitInU/price)
		getTick, tick := model.AppEnvironment.GetBidAsk(equalStatus.market, equalStatus.symbol)
		if !getTick {
			equalStatus.AvailableBuy, equalStatus.AvailableSell = 0, 0
			util.Notice(`no tick when equal return %s %s %s`, coin, equalStatus.symbol, equalStatus.market)
			return isEqual, holdingInU, ``
		}
		reduceOnly := equalStatus.reduceOnlySell
		if orderSide == model.OrderSideBuy {
			reduceOnly = equalStatus.reduceOnlyBuy
			price = tick.Asks[0].Price
		} else if orderSide == model.OrderSideSell {
			price = tick.Bids[0].Price
		}
		checkAmount := model.GetAmountInMarket(equalStatus.market, equalStatus.symbol, amount, price, reduceOnly)
		if checkAmount > 0 {
			util.Notice(fmt.Sprintf(`do equal %s %s %s at %f %f %f %d amount %f holding %f worthU %f status holding %f`,
				coin, equalStatus.market, equalStatus.symbol, price, tick.Asks[0].Price, tick.Bids[0].Price, tick.Ts, amount, holding, holdingInU, equalStatus.Holding))
			order := api.PlaceOrder(equalStatus.account.Key, equalStatus.account.Secret, orderSide, model.OrderTypeLimit,
				equalStatus.market, equalStatus.symbol, ``, model.FunctionComplement, price, price, amount, false, nil)
			if order != nil && order.Status != model.CarryStatusFail {
				if orderSide == model.OrderSideBuy {
					equalStatus.Holding += amount
					holdingInU += amount * price
				} else {
					equalStatus.Holding -= amount
					holdingInU -= amount * price
				}
				saveCross(order, equalStatus.TradeLineBuy, equalStatus.TradeLineSell, equalStatus.Holding)
				if equalStatus.market == model.Gate {
					api.SetGateBidAsk(equalStatus.account.Key, equalStatus.account.Secret, equalStatus.symbol)
				}
				if orderSide == model.OrderSideSell {
					placeStatus(equalStatus, price, -1*amount)
				} else if orderSide == model.OrderSideBuy {
					placeStatus(equalStatus, price, amount)
				}
			} else {
				if orderSide == model.OrderSideBuy {
					equalStatus.AvailableBuy = 0
				} else if orderSide == model.OrderSideSell {
					equalStatus.AvailableSell = 0
				}
			}
		}
	} else if math.Abs(holdingInU) > SmallInU {
		isEqual = true // 可能由于头寸太小，不满足所有市场的下单要求，而holdingU刚好大于10u，此时认为已平
		minute := time.Now().Minute()
		second := time.Now().Second()
		util.Notice(fmt.Sprintf(`can not get status %s %f err msg %s`, coin, holdingInU, errMsg))
		if minute == 0 && second == 0 {
			go api.SendMails(`equal error`, fmt.Sprintf(`can not get status for %s when holding %f`, coin, holdingInU))
		}
	}
	return isEqual, holdingInU, ``
}

// ProcessCross setting.Chance<0时该币种只关仓
// setting.OpenShortMargin OpenShortMargin不等于0时作为开舱标准价格，否则使用通用价格
// setting.CloseShortMargin CloseShortMargin作为开关舱标准价格
var ProcessCross = func(setting *model.Setting, tick *model.BidAsk) {
	// 所有cross之间互斥
	if !api.CheckSetProcessing(setting.Function, setting.Function, setting.Function, true) {
		defer api.CheckSetProcessing(setting.Function, setting.Function, setting.Function, false)
	} else {
		return
	}
	if !doCross && model.AppConfig.Handle == `1` {
		go ClearCross()
		doCross = true
		return
	}
	million := util.GetNowUnixMillion()
	coinSettings := api.GetCoinSettings(setting.Function)
	var settings []*model.Setting
	if coinSettings == nil {
		settings = nil
	} else {
		value, _ := coinSettings.Load(setting.Coin)
		if value != nil {
			settings = value.([]*model.Setting)
		}
	}
	maintaining, ok := model.ChannelMaintaining.Load(setting.Market)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || (ok && maintaining.(bool)) ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) || setting.Valid == false ||
		settings == nil || len(settings) == 0 {
		return
	}
	tickLimit := 50
	switch tick.Bids[0].Market {
	case model.Gate, model.BitgetPerp, model.BitgetSpot:
		tickLimit = 30
	case model.BinanceSpot, model.BinancePerp:
		tickLimit = 20
	case model.Bybit, model.OKEX:
		tickLimit = 60
	}
	if int(million)-tick.Ts > tickLimit {
		return
	}
	for _, settingRelate := range settings {
		tickGet, tickRelate := model.AppEnvironment.GetBidAsk(settingRelate.Market, settingRelate.Symbol)
		if !tickGet || setting.ID == settingRelate.ID || (!model.NonRTTicker[tick.Bids[0].Market] && model.NonRTTicker[tickRelate.Bids[0].Market]) ||
			(model.AppConfig.Env != `test`) {
			continue
		}
		switch tickRelate.Bids[0].Market {
		case model.Gate, model.BitgetPerp, model.BitgetSpot, model.BinanceSpot, model.BinancePerp:
			tickLimit = 50
		case model.Bybit, model.OKEX:
			tickLimit = 80
		}
		if int(million)-tick.Ts > tickLimit {
			continue
		}
		for i := api.GetCrossLen() - 1; i >= 0; i-- {
			account := model.AppConfig.GetAccounts(setting.Market)[i]
			accountRelate := model.AppConfig.GetAccounts(settingRelate.Market)[i]
			if account == nil || accountRelate == nil {
				continue
			}
			status, okStatus := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, account.Key)
			statusRelate, okRelate := util.LoadSyncMap(carryStatusMap, settingRelate.Coin, settingRelate.Market, settingRelate.Symbol, accountRelate.Key)
			if status == nil || statusRelate == nil || status == statusRelate || !okStatus || !okRelate {
				continue
			}
			statusBuy, statusSell, amount, priceBuy, priceSell, tickBuy, tickSell :=
				calcAmount(i, setting.Coin, status.(*CarryStatus), statusRelate.(*CarryStatus), tick, tickRelate)
			if amount > 0 {
				placeBuyStr := fmt.Sprintf(`%s_%s_%s`, statusBuy.market, statusBuy.symbol, model.OrderSideBuy)
				placeSellStr := fmt.Sprintf(`%s_%s_%s`, statusSell.market, statusSell.symbol, model.OrderSideSell)
				placeBuyValue := fmt.Sprintf(`%f_%f`, tickBuy.Asks[0].Price, tickBuy.Asks[0].Amount)
				placeSellValue := fmt.Sprintf(`%f_%f`, tickSell.Bids[0].Price, tickSell.Bids[0].Amount)
				value, ok := placeTick.Load(placeBuyStr)
				if ok && value != nil && value.(string) == placeBuyValue {
					util.Notice(fmt.Sprintf(`tick static %s %s`, placeBuyStr, placeBuyValue))
					return
				}
				value, ok = placeTick.Load(placeSellStr)
				if ok && value != nil && value.(string) == placeSellValue {
					util.Notice(fmt.Sprintf(`tick static %s %s`, placeSellStr, placeSellValue))
					return
				}
				placeCross(statusBuy, statusSell, priceBuy, priceSell, amount)
				nowTs := time.Now().UnixMilli()
				util.Notice(fmt.Sprintf(`cross delay %s %d relate %s %d, %s %s %s %s`,
					tick.Bids[0].Market, nowTs-int64(tick.Ts), tickRelate.Bids[0].Market, nowTs-int64(tickRelate.Ts),
					placeBuyStr, placeBuyValue, placeSellStr, placeSellValue))
				placeTick.Store(placeBuyStr, placeBuyValue)
				placeTick.Store(placeSellStr, placeSellValue)
				return
			}
		}
	}
}

func breakMarkPrice(account *model.Account, setting *model.Setting, price float64, orderSide string) bool {
	v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
	var marketInfo *model.MarketInfo
	if v != nil {
		marketInfo = v.(*model.MarketInfo)
	}
	if marketInfo != nil && orderSide == model.OrderSideBuy && marketInfo.BuyLimitPriceRatio > 0 {
		markPriceInfo := model.AppEnvironment.GetMarkPriceInfo(setting.Symbol, setting.Market)
		if markPriceInfo == nil {
			util.Notice(fmt.Sprintf("币种：%s 市场 %s 有限价条件，但是没有标记价格", setting.Symbol, setting.Market))
			api.GetMarkPrice(account, setting.Market, setting.Symbol)
			return true
		}
		bidMaxPrice := markPriceInfo.MarkPrice * (1 + marketInfo.BuyLimitPriceRatio)
		bidMinPrice := markPriceInfo.MarkPrice * (1 - marketInfo.BuyLimitPriceRatio)
		if price > bidMaxPrice || price < bidMinPrice {
			util.NoticeLess(fmt.Sprintf("币种：%s %s 被限买价，买上浮：%f，标记价：%f，限最高买价：%f，限最低买价：%f，当前最佳买价：%f",
				setting.Market, setting.Symbol, marketInfo.BuyLimitPriceRatio, markPriceInfo.MarkPrice, bidMaxPrice, bidMinPrice, price))
			return true
		}
	} else if marketInfo != nil && orderSide == model.OrderSideSell && marketInfo.SellLimitPriceRatio > 0 {
		markPriceInfo := model.AppEnvironment.GetMarkPriceInfo(setting.Symbol, setting.Market)
		if markPriceInfo == nil {
			util.Notice(fmt.Sprintf("币种：%s 市场 %s 有限价条件，但是没有标记价格", setting.Symbol, setting.Market))
			api.GetMarkPrice(account, setting.Market, setting.Symbol)
			return true
		}
		askMaxPrice := markPriceInfo.MarkPrice * (1 + marketInfo.SellLimitPriceRatio)
		askMinPrice := markPriceInfo.MarkPrice * (1 - marketInfo.SellLimitPriceRatio)
		if price > askMaxPrice || price < askMinPrice {
			util.NoticeLess(fmt.Sprintf("币种：%s %s 被限卖价，卖下浮：%f，标记价：%f，限最高卖价：%f，限最低卖价：%f，当前最佳卖价：%f",
				setting.Market, setting.Symbol, marketInfo.SellLimitPriceRatio, markPriceInfo.MarkPrice, askMaxPrice, askMinPrice, price))
			//perpAskPrice = askMinPrice
			return true
		}
	}
	return false
}

func checkTradeLine(statusBuy, statusSell *CarryStatus, score, price float64) (valid, haveLimit bool, limit float64) {
	limit = math.Max(score*100000/price, 0)
	if statusBuy.Holding >= 0 && statusSell.Holding <= 0 {
		return score > statusBuy.TradeLineBuy && score > statusSell.TradeLineSell, true, limit
		//} else if statusBuy.Holding < 0 && statusSell.Holding > 0 {
		//	small := math.Min(statusBuy.TradeLineBuy, statusSell.TradeLineSell)
		//	big := math.Max(statusBuy.TradeLineBuy, statusSell.TradeLineSell)
		//	return score*3 > small*2+big
	} else {
		marketDis := (statusBuy.TradeLineBuy + statusSell.TradeLineSell) / 2
		if score > marketDis {
			return true, true, limit
		}
		if statusBuy.account.CarryClose && statusBuy.Holding < 0 {
			limit = math.Min(limit, math.Abs(statusBuy.Holding))
		}
		if statusSell.account.CarryClose && statusSell.Holding > 0 {
			limit = math.Min(limit, statusSell.Holding)
		}
		return score > marketDis, true, limit
	}
}

func calcAmount(index int, coin string, carryStatus, carryStatusRelate *CarryStatus, tick, tickRelate *model.BidAsk) (
	statusBuy, statusSell *CarryStatus, amount, priceBuy, priceSell float64, tickBuy, tickSell *model.BidAsk) {
	stopStatus, okStatus := carryStop.Load(carryStatus.account.Key)
	stopRelate, okRelate := carryStop.Load(carryStatusRelate.account.Key)
	if (okStatus && stopStatus.(bool)) || (okRelate && stopRelate.(bool)) {
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
	score := (priceBid - priceAskRelate) / math.Max(priceBid, priceAskRelate)
	scoreRelate := (priceBidRelate - priceAsk) / math.Max(priceAsk, priceBidRelate)
	mark := fmt.Sprintf(`%s_%s|%s_%s`, carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol)
	if score > 0.01 && util.DoDebug {
		model.AppMetric.AddCarry(mark, score, 0)
	}
	// 根据负资金费率进行权重调整,小于负万五的，负千分之几，就再乘以几
	//tradeLineSell := carryStatus.TradeLineSell + carryStatusRelate.FundingRate
	//tradeLineBuy := carryStatus.TradeLineBuy - carryStatusRelate.FundingRate
	//tradeLineSellRelate := carryStatusRelate.TradeLineSell + carryStatus.FundingRate
	//tradeLineBuyRelate := carryStatusRelate.TradeLineBuy - carryStatus.FundingRate
	//if carryStatus.isSpot && !carryStatusRelate.isSpot && carryStatusRelate.market != model.Ftx && carryStatusRelate.FundingRate < -0.005 {
	//	temp := math.Min(7.5, 1000*math.Abs(carryStatusRelate.FundingRate))/2 - 1
	//	tradeLineBuyRelate += carryStatusRelate.FundingRate * temp
	//	tradeLineSellRelate -= carryStatusRelate.FundingRate * temp
	//} else if !carryStatus.isSpot && carryStatus.market != model.Ftx && carryStatusRelate.isSpot && carryStatus.FundingRate < -0.005 {
	//	temp := math.Min(7.5, 1000*math.Abs(carryStatus.FundingRate))/2 - 1
	//	tradeLineBuy += carryStatus.FundingRate * temp
	//	tradeLineSell -= carryStatus.FundingRate * temp
	//}
	valid, haveLimit, limit := checkTradeLine(carryStatusRelate, carryStatus, score, priceBid)
	if valid {
		statusSell = carryStatus
		statusBuy = carryStatusRelate
		tickSell = tick
		tickBuy = tickRelate
		priceSell = priceBid
		priceBuy = priceAskRelate
		askAmount = amountBid
		bidAmount = amountAskRelate
	} else {
		valid, haveLimit, limit = checkTradeLine(carryStatus, carryStatusRelate, scoreRelate, priceBid)
		if valid {
			statusSell = carryStatusRelate
			statusBuy = carryStatus
			tickSell = tickRelate
			tickBuy = tick
			priceSell = priceBidRelate
			priceBuy = priceAsk
			askAmount = amountBidRelate
			bidAmount = amountAsk
		}
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
	green := false
	if math.Abs(carryStatusRelate.FundingRate) > 0.001 || math.Abs(carryStatus.FundingRate) > 0.001 {
		green = true
	}
	fundingStr, fundingStrRelate := ``, ``
	if !carryStatus.isSpot {
		fundingStr = fmt.Sprintf(`%.5f %d:%d`, 100*carryStatus.FundingRate,
			carryStatus.FundingRateUpdateTime.Hour(), carryStatus.FundingRateUpdateTime.Minute())
	}
	if !carryStatusRelate.isSpot {
		fundingStrRelate = fmt.Sprintf(`%.5f %d:%d`, 100*carryStatusRelate.FundingRate,
			carryStatusRelate.FundingRateUpdateTime.Hour(), carryStatusRelate.FundingRateUpdateTime.Minute())
	}
	var infoValue []string
	if mark < markRelate {
		mark = fmt.Sprintf(`%s|%s`, mark, markRelate)
		infoValue = []string{coin, carryStatus.market, coinValue, fundingStr,
			fmt.Sprintf(`%.1f`, 100*carryStatus.TradeLineBuy),
			fmt.Sprintf(`%.1f`, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatus.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatus.LimitSell),
			carryStatusRelate.market, coinValueRelate, fundingStrRelate,
			fmt.Sprintf(`%.1f`, 100*carryStatusRelate.TradeLineBuy),
			fmt.Sprintf(`%.1f`, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitSell),
			fmt.Sprintf(`%.1f`, 100*scoreRelate),
			fmt.Sprintf(`%.1f`, 100*score),
			fmt.Sprintf(`%v`, green)}
	} else {
		mark = fmt.Sprintf(`%s|%s`, markRelate, mark)
		infoValue = []string{coin, carryStatusRelate.market, coinValueRelate, fundingStrRelate,
			fmt.Sprintf(`%.1f`, 100*carryStatusRelate.TradeLineBuy),
			fmt.Sprintf(`%.1f`, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitSell),
			carryStatus.market, coinValue, fundingStr,
			fmt.Sprintf(`%.1f`, 100*carryStatus.TradeLineBuy),
			fmt.Sprintf(`%.1f`, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatus.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatus.LimitSell),
			fmt.Sprintf(`%.1f`, 100*score),
			fmt.Sprintf(`%.1f`, 100*scoreRelate),
			fmt.Sprintf(`%v`, green)}
	}
	model.SetMonitorInfo(strconv.Itoa(index), model.FunctionCross, mark, infoValue)
	if statusBuy == nil {
		return nil, nil, 0, 0, 0, nil, nil
	}
	if breakMarkPrice(statusBuy.account, statusBuy.setting, priceBuy, model.OrderSideBuy) ||
		breakMarkPrice(statusSell.account, statusSell.setting, priceSell, model.OrderSideSell) {
		return nil, nil, 0, 0, 0, nil, nil
	}
	// 如果上一次交易不是本交易对，但上一次交易很可能影响了资金状况，需要对本carryStatus的可买卖数量进行调整
	lastSymbol, ok := util.LoadSyncMap(&lastCrosses, statusBuy.account.Key, statusBuy.market)
	if !(ok && lastSymbol != nil && lastSymbol.(string) == statusBuy.symbol) {
		initLimitBuyAndSell(statusBuy, statusBuy.setting, priceBuy)
	}
	lastSymbol, ok = util.LoadSyncMap(&lastCrosses, statusSell.account.Key, statusSell.market)
	if !(ok && lastSymbol != nil && lastSymbol.(string) == statusSell.symbol) {
		initLimitBuyAndSell(statusSell, statusSell.setting, priceSell)
	}
	amount = math.Min(math.Min(statusBuy.LimitBuy, bidAmount), math.Min(statusSell.LimitSell, askAmount))
	if amount > 0 {
		if haveLimit {
			amount = math.Min(amount, limit)
		}
		amount = math.Min(amount, openValueLimit/priceSell)
		amount = FormatCrossPair(statusBuy.market, statusSell.market, statusBuy.symbol, statusSell.symbol, amount, priceBuy)
	}
	if checkScoreLimit(carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol, amount, score, scoreRelate) {
		return nil, nil, 0, 0, 0, nil, nil
	}
	return statusBuy, statusSell, amount, priceBuy, priceSell, tickBuy, tickSell
}

func checkScoreLimit(market, symbol, marketRelate, symbolRelate string, amount, score, scoreRelate float64) (invalid bool) {
	if amount > 0 && ((score > 0.3 || scoreRelate > 0.3) ||
		((score > 0.07 || scoreRelate > 0.07) && (market == model.Gate || marketRelate == model.Gate)) ||
		((score > 0.1 || scoreRelate > 0.1) && (!isValidSymbol(market, symbol) || !isValidSymbol(marketRelate, symbolRelate)))) {
		invalid = true
	}
	checkKey := fmt.Sprintf(`%s_%s_%s_%s`, market, symbol, marketRelate, symbolRelate)
	lastTime, ok := notifyTime.Load(checkKey)
	if !(ok && lastTime.(time.Time).Add(time.Minute*60).After(time.Now())) {
		title := `币种价差大`
		checkKeyRelate := fmt.Sprintf(`%s_%s_%s_%s`, marketRelate, symbolRelate, market, symbol)
		if score > 0.15 || scoreRelate > 0.15 {
			title = `价差不可思议`
		}
		msg := fmt.Sprintf(`价差提醒 %s %s %s %s %f %f`,
			market, symbol, marketRelate, symbolRelate, score, scoreRelate)
		if invalid {
			notifyTime.Store(checkKey, time.Now())
			notifyTime.Store(checkKeyRelate, time.Now())
			go func() {
				err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth,
					`haoweizh@qq.com`, title, msg)
				if err != nil {
					util.Notice(`fail to send mail msg %s %s`, msg, err.Error())
				}
			}()
		} else if score > 0.05 || scoreRelate > 0.05 {
			notifyTime.Store(checkKey, time.Now())
			notifyTime.Store(checkKeyRelate, time.Now())
		}
	}
	return
}

func initLimitBuyAndSell(status *CarryStatus, setting *model.Setting, price float64) {
	if status.isSpot {
		value, ok := spotMarkets.Load(status.account.Key)
		if !ok || value == nil { // 此时正在进行每分钟的清理找平
			return
		}
		sm := value.(*spotMarket)
		status.LimitBuy = math.Min(status.LimitBuy, math.Min(openValueLimit, math.Min(sm.availableU/5, sm.accountValueInU/15))/price)
		balance := sm.balances[setting.Symbol]
		if balance != nil {
			status.LimitSell = math.Min(status.LimitSell, math.Min(math.Min(math.Max(balance.Amount, 0), balance.AvailableWithBorrow), openValueLimit/price))
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
		status.LimitSell = math.Min(status.LimitSell, limitAmount)
		status.LimitBuy = math.Min(status.LimitBuy, limitAmount)
		status.AvailableSell = math.Min(status.AvailableSell, availableAmount)
		status.AvailableBuy = math.Min(status.AvailableBuy, availableAmount)
	}
}

func placeCross(statusBuy, statusSell *CarryStatus, priceBuy, priceSell, amount float64) {
	score := (priceSell - priceBuy) / math.Max(priceBuy, priceSell)
	util.Notice(fmt.Sprintf(`place cross %s %s -> %s %s at %f %f amount %f score %f hold %f buy %f hold %f sell %f`,
		statusSell.market, statusSell.symbol, statusBuy.market, statusBuy.symbol, priceSell, priceBuy, amount,
		score, statusBuy.Holding, statusBuy.TradeLineBuy, statusSell.Holding, statusSell.TradeLineSell))
	if statusBuy.market == model.OKEX && statusSell.market == model.OKEX {
		requestId := strconv.FormatInt(time.Now().UnixMilli(), 10)[3:] + model.OKEX
		orderBuy := &model.Order{OrderSide: model.OrderSideBuy, OrderType: model.OrderTypeLimit, Market: model.OKEX,
			Symbol: statusBuy.symbol, Price: priceBuy, Amount: amount, RefreshType: model.FunctionCross, OrderTime: util.GetNow(),
			UnfilledQuantity: amount, AccountIndex: statusBuy.account.Index, Status: model.CarryStatusWorking, Function: model.Open,
			OrderId: requestId + statusBuy.symbol, LineBuy: statusBuy.TradeLineBuy, LineSell: statusSell.TradeLineSell}
		orderSell := &model.Order{OrderSide: model.OrderSideSell, OrderType: model.OrderTypeLimit, Market: model.OKEX,
			Symbol: statusSell.symbol, Price: priceSell, Amount: amount, RefreshType: model.FunctionCross, OrderTime: util.GetNow(),
			UnfilledQuantity: amount, AccountIndex: statusSell.account.Index, Status: model.CarryStatusWorking, Function: model.Open,
			OrderId: requestId + statusSell.symbol, LineBuy: statusSell.TradeLineBuy, LineSell: statusSell.TradeLineSell}
		if statusBuy.Holding*-1 >= amount {
			orderBuy.Function = model.Close
		}
		if statusSell.Holding >= amount {
			orderSell.Function = model.Close
		}
		orderBuy.Coin = statusBuy.setting.Coin
		orderSell.Coin = statusSell.setting.Coin
		model.AppEnvironment.WSOrderMap.Store(requestId+model.OrderSideBuy, orderBuy)
		model.AppEnvironment.WSOrderMap.Store(requestId+model.OrderSideSell, orderSell)
		success, msg := api.PlacePairOKEX(statusBuy.account, requestId, statusBuy.symbol, statusSell.symbol, model.OrderTypeLimit, priceBuy, priceSell, amount)
		if !success {
			orderBuy.Status, orderSell.Status = model.CarryStatusFail, model.CarryStatusFail
			orderBuy.ErrCode, orderSell.ErrCode = msg, msg
		}
	} else {
		go func() {
			orderParam := ``
			if statusBuy.reduceOnlyBuy {
				orderParam = model.ReduceOnly
			}
			api.PlaceOrder(statusBuy.account.Key, statusBuy.account.Secret, model.OrderSideBuy, model.OrderTypeLimit,
				statusBuy.market, statusBuy.symbol, orderParam, model.FunctionCross, priceBuy, priceBuy, amount, true, PostOrderCross)
		}()
		go func() {
			orderParam := ``
			if statusSell.reduceOnlySell {
				orderParam = model.ReduceOnly
			}
			api.PlaceOrder(statusSell.account.Key, statusSell.account.Secret, model.OrderSideSell, model.OrderTypeLimit,
				statusSell.market, statusSell.symbol, orderParam, model.FunctionCross, priceSell, priceSell, amount, true, PostOrderCross)
		}()
		time.Sleep(time.Second * 4)
	}
	placeStatus(statusBuy, priceBuy, amount)
	placeStatus(statusSell, priceSell, -1*amount)
}

func placeStatus(status *CarryStatus, price float64, amount float64) {
	valueSpot, _ := spotMarkets.Load(status.account.Key)
	valueContract, _ := contractMarkets.Load(status.account.Key)
	if status.isSpot && valueSpot != nil {
		balance := valueSpot.(*spotMarket).balances[status.symbol]
		if balance == nil {
			util.Notice(fmt.Sprintf(`warning no balance %s %s %s`,
				status.account.Key, status.market, status.symbol))
			balance = &model.Balance{Amount: amount, UsdValue: amount * price, Market: status.market, Coin: status.setting.Coin}
			valueSpot.(*spotMarket).balances[status.symbol] = balance
		} else {
			balance.Amount += amount
			balance.UsdValue = balance.Amount * price
			balance.AvailableWithBorrow += amount
		}
		valueSpot.(*spotMarket).availableU -= amount * price
		if status.account.IsUnified {
			if valueContract != nil {
				valueContract.(*contractMarket).collateralsAvailable -= amount * price
			}
			valueSpot.(*spotMarket).collateral.Available -= amount * price
		}
	} else if valueContract != nil {
		position := valueContract.(*contractMarket).positions[status.symbol]
		originFreeAbs := 0.0
		if position == nil {
			position = &api.Position{Holding: amount, EntryPrice: price, Market: status.market, Currency: status.setting.Symbol}
			valueContract.(*contractMarket).positions[status.symbol] = position
		} else {
			originFreeAbs = math.Abs(position.Holding)
			position.Holding += amount
			position.EntryPrice = price
		}
		changeU := (originFreeAbs - math.Abs(position.Holding)) * price
		valueContract.(*contractMarket).collateralsAvailable += changeU * 0.2
		valueContract.(*contractMarket).contractValueInU += changeU
		if valueSpot != nil {
			if status.market == model.Ftx {
				valueSpot.(*spotMarket).availableU += changeU * 0.2
			} else if status.market == model.OKEX {
				valueSpot.(*spotMarket).collateral.Available += changeU * 0.1
				valueSpot.(*spotMarket).collateral.Occupied -= changeU * 0.1
				valueSpot.(*spotMarket).availableU += changeU * 0.1
			} else if status.account.IsUnified {
				valueSpot.(*spotMarket).collateral.Available += changeU * 0.2
				valueSpot.(*spotMarket).collateral.Occupied -= changeU * 0.2
				valueSpot.(*spotMarket).availableU += changeU * 0.2
			}
		}
	}
	account := model.AppConfig.GetAccountFromKeyIndex(status.market, status.account.Key, -1)
	initStatus(account, status.setting, true)
	util.StoreSyncMap(&lastCrosses, status.symbol, account.Key, status.market)
}

func handleCross(account *model.Account, order *model.Order) {
	time.Sleep(time.Minute)
	if !order.HaveId() {
		order.Status = model.CarryStatusFail
		order.OrderId = fmt.Sprintf("%d%s%s", time.Now().UnixMilli(), order.Market, order.Symbol)
	}
	model.AppDB.Save(order)
	v, _ := util.LoadSyncMap(model.MarketInfos, order.Market, order.Symbol)
	var marketInfo *model.MarketInfo
	if v != nil {
		marketInfo = v.(*model.MarketInfo)
	} else {
		return
	}
	leftAmt := order.Amount - order.DealAmount
	leftAmtInMkt := model.GetAmountInMarket(order.Market, order.Symbol, leftAmt, order.Price, false)
	if leftAmtInMkt > marketInfo.SizeMin && leftAmtInMkt*order.Price > marketInfo.MoneyMin {
		canceled, errCode, errMsg := api.CancelOrder(account.Key, account.Secret, order.Market, order.Symbol, model.OrderTypeLimit, order.OrderId)
		compOrder := api.PlaceOrder(account.Key, account.Secret, order.OrderSide, model.OrderTypeMarket, order.Market, order.Symbol,
			``, model.FunctionComplement, order.Price, order.Price, order.Amount-order.DealAmount, false, nil)
		model.AppDB.Save(compOrder)
		util.Notice(fmt.Sprintf(`post handle cancel order %s %s %s side %s %v type %s code %s msg %s not deal %f, comp %v`,
			order.OrderId, order.Market, order.Symbol, order.OrderSide, canceled, order.OrderType, errCode, errMsg, leftAmt, compOrder))
	} else {
		util.Notice(fmt.Sprintf(`post handle done %s %s %s %s %f`,
			order.Market, order.Symbol, order.OrderId, order.OrderType, order.DealAmount))
	}
	model.AppEnvironment.CrossOrders.Delete(order.OrderId)
}

var PostOrderCross = func(order *model.Order) {
	if order == nil {
		return
	}
	setting := api.GetSetting(model.FunctionCross, order.Market, order.Symbol)
	if setting == nil {
		util.Notice(fmt.Sprintf(`fail to get setting %s %s %s`, model.FunctionCross, order.Market, order.Symbol))
		return
	}
	account := model.AppConfig.GetAccountFromKeyIndex(order.Market, ``, order.AccountIndex)
	go handleCross(account, order)
	if order.HaveId() {
		addLastCarry(order, setting)
		addCarryResult(account.Key, order.Market, ``, true)
	} else {
		unknownFail := true
		if account != nil {
			status, ok := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, account.Key)
			switch order.Market {
			case model.OKEX:
				if InsufficientCodeOKEX[order.ErrCode] && setting != nil {
					util.Notice(`reset %s trade max with %s account index %s`, order.Market, order.ErrCode, order.AccountIndex)
					getMax, maxBuy, maxSell := api.GetTradeMaxOKEX(account.Key, account.Secret, setting.Symbol, 0)
					if getMax && ok && status != nil {
						status.(*CarryStatus).LimitSell = math.Min(status.(*CarryStatus).LimitSell, maxSell)
						status.(*CarryStatus).LimitBuy = math.Min(status.(*CarryStatus).LimitBuy, maxBuy)
					}
					unknownFail = false
				}
				//case model.BinancePerp, model.BinanceSpot:
				//	if strings.Contains(InsufficientCodeBinance, order.ErrCode) {
				//		util.Notice(`reset binance trade max with %s account index %d`, order.ErrCode, order.AccountIndex)
				//		spotMarkets.Delete(account.Key)
				//		contractMarkets.Delete(account.Key)
				//		initStatus(account, setting, true)
				//		unknownFail = false
				//	}
			}
			if ok && status != nil {
				if order.OrderSide == model.OrderSideBuy {
					status.(*CarryStatus).TradeLineBuy = 1
				} else if order.OrderSide == model.OrderSideSell {
					status.(*CarryStatus).TradeLineSell = 1
				}
			}
			util.Notice(fmt.Sprintf(`set 1 trade line after fail %s %s %s`, setting.Market, setting.Symbol, order.OrderSide))
		}
		if unknownFail {
			addCarryResult(account.Key, order.Market, order.ErrCode, false)
		} else {
			addCarryResult(account.Key, order.Market, ``, true)
		}
	}
}

// FormatCrossPair 不支持以BTC或ETH计价的交易对，只支持USD类
func FormatCrossPair(marketBuy, marketSell, symbolBuy, symbolSell string, amount, price float64) (
	formattedAmount float64) {
	v, _ := util.LoadSyncMap(model.MarketInfos, marketBuy, symbolBuy)
	var marketInfoBuy, marketInfoSell *model.MarketInfo
	if v != nil {
		marketInfoBuy = v.(*model.MarketInfo)
	}
	v, _ = util.LoadSyncMap(model.MarketInfos, marketSell, symbolSell)
	if v != nil {
		marketInfoSell = v.(*model.MarketInfo)
	}
	if marketInfoBuy == nil || marketInfoSell == nil {
		util.Notice(`format %s %s %s %s %v %v`, marketBuy, marketSell, symbolBuy, symbolSell, marketInfoBuy, marketInfoSell)
		//api.InitMarketInfos()
		marketInfoTime, ok := getMarketInfoMail.Load(`FormatCrossPair`)
		if !(ok && marketInfoTime.(time.Time).Add(time.Minute*60).After(time.Now())) {
			notifyTime.Store(`FormatCrossPair`, time.Now())
			go api.SendMails(`FormatCrossPair removed symbols`, fmt.Sprintf(`format %s %s %s %s %v %v`,
				marketBuy, marketSell, symbolBuy, symbolSell, marketInfoBuy, marketInfoSell))
		}
		return
	}
	incBuy := marketInfoBuy.SizeIncrement
	incSell := marketInfoSell.SizeIncrement
	minBuy := marketInfoBuy.SizeMin
	minSell := marketInfoSell.SizeMin
	success, _, coin, _ := model.GetFromStandard(marketBuy, symbolBuy)
	if success && marketInfoBuy.CTCurrency == coin && marketInfoBuy.CTValue > 0 {
		incBuy, minBuy = incBuy*marketInfoBuy.CTValue, minBuy*marketInfoBuy.CTValue
	}
	success, _, coin, _ = model.GetFromStandard(marketSell, symbolSell)
	if success && marketInfoSell.CTCurrency == coin && marketInfoSell.CTValue > 0 {
		incSell, minSell = incSell*marketInfoSell.CTValue, minSell*marketInfoSell.CTValue
	}
	if marketBuy == model.Bybit {
		minBuy = math.Max(5.5/price, minBuy)
	}
	if marketSell == model.Bybit {
		minSell = math.Max(5.5/price, minSell)
	}
	sizeInc := math.Max(incBuy, incSell)
	formattedAmount = math.Floor(amount/sizeInc) * sizeInc
	if formattedAmount < math.Max(minBuy, minSell) {
		return 0
	}
	if (marketInfoBuy.MoneyMin > 0 && formattedAmount*price < marketInfoBuy.MoneyMin) ||
		(marketInfoSell.MoneyMin > 0 && formattedAmount*price < marketInfoSell.MoneyMin) {
		return 0
	}
	if marketInfoSell.Market == model.Mexc {
		formattedAmount = math.Min(formattedAmount, marketInfoSell.SizeIncrement*1000)
	}
	if marketInfoBuy.Market == model.Mexc {
		formattedAmount = math.Min(formattedAmount, marketInfoBuy.SizeIncrement*1000)
	}
	if formattedAmount*price < 6 && (marketInfoSell.Market == model.Mexc || marketInfoBuy.Market == model.Mexc) {
		return 0
	}
	return formattedAmount
}

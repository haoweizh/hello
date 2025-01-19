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

func createContractMarket(key, secret, market string) (cm *contractMarket) {
	success, positions, accountValue, availableU, mmr := api.GetPositions(key, secret, market)
	util.Log(util.LogLevelInfo, fmt.Sprintf(`get positions %s %s %#v account value %f available u %f maintain rate %f positions %d`,
		market, key, success, accountValue, availableU, mmr, len(positions)))
	settings := api.GetSettings(model.FunctionCross, market)
	if success {
		cm = &contractMarket{key: key, market: market}
		cm.positions = make(map[string]*model.Position)
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
					util.Log(util.LogLevelInfo, fmt.Sprintf(`holding absent pos %s %s %f`, market, position.Currency, position.Holding))
				}
			}
		}
		cm.accountValueInU = accountValue
		cm.collateralsAvailable = availableU
		cm.mmr = mmr
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to createContractMarket %s %s`, market, key))
		return nil
	}
	return
}

func createSpotMarket(key, secret, market string) (sm *spotMarket) {
	success, balances, totalInUsd, collateral := api.GetBalances(key, secret, market)
	util.Log(util.LogLevelInfo, fmt.Sprintf(`create spot market %s %d`, market, len(balances)))
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
				util.Log(util.LogLevelInfo, fmt.Sprintf(`create spot %s %s %f %f`,
					market, balance.Coin, balance.Amount, balance.AvailableWithBorrow))
			}
			// 可用usd数量需要减去现有所有借币负债总额
			if balance.UsdValue < 0 {
				sm.availableU -= math.Abs(balance.UsdValue)
			}
			if settings != nil {
				value, ok := settings.Load(symbol)
				if (!ok || value == nil) && balance.Amount > 0 {
					util.Log(util.LogLevelInfo, fmt.Sprintf(`holding absent bal %s %s %f`, market, symbol, balance.Amount))
				}
			}
		}
		//if collateral != nil {
		//	util.Log(util.LogLevelInfo, fmt.Sprintf(`collateral for sm available u %s %f to %f maintain rate %f`,
		//		market, sm.availableU, collateral.Available, collateral.Rate))
		//	sm.availableU = math.Min(sm.availableU, collateral.Available)
		//}
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to createSpotMarket %s %s`, market, key))
		return nil
	}
	return
}

// absentRevert: 当cm或sm中没有这个symbol时，是否设置成revert模式
func createFromPosition(account *model.Account, setting *model.Setting, valueLimit float64) (carryStatus *model.CarryStatus, doRevert bool) {
	key := account.Key
	value, ok := contractMarkets.Load(key)
	if value == nil || !ok {
		contractMarkets.Store(key, createContractMarket(key, account.Secret, setting.Market))
		value, _ = contractMarkets.Load(key)
		spotValue, spotOk := spotMarkets.Load(key)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`success set cm %d %s`, account.Index, setting.Market))
		if account.IsUnified && (spotValue == nil || !spotOk) {
			spotMarkets.Store(key, createSpotMarket(key, account.Secret, setting.Market))
		}
	}
	if value == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`nil contract market %s %s`, setting.Market, setting.Symbol))
		return nil, false
	}
	cm := value.(*contractMarket)
	handledActValueInU := cm.accountValueInU
	if setting.Market == model.Gate {
		handledActValueInU = 0.6 * cm.accountValueInU
	}
	_, price := api.GetPriceForce(setting.Symbol, setting.Market)
	limitAmount := 0.0
	availableAmount := 0.0
	if price > 0 {
		limitAmount = math.Min(handledActValueInU/5, cm.collateralsAvailable) / price
		if setting.Market == model.Gate {
			item, loaded := util.LoadSyncMap(&model.AppEnvironment.RiskLimitsGate, account.Key, setting.Symbol)
			if loaded {
				riskLimit := 0.9 * item.(float64) / price
				holding := 0.0
				if cm.positions[setting.Symbol] != nil {
					holding = cm.positions[setting.Symbol].Holding
					riskLimit -= math.Abs(cm.positions[setting.Symbol].Holding)
				}
				limitAmount = math.Min(limitAmount, riskLimit)
				util.Log(util.LogLevelInfo, fmt.Sprintf(`set limit %s risk %f %f holding %f price %f`,
					setting.Symbol, riskLimit, limitAmount, holding, price))
			}
		}
		availableAmount = cm.collateralsAvailable / price
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %s %s price 0`, setting.Market, setting.Symbol))
		doRevert = true
	}
	carryStatus = &model.CarryStatus{IsSpot: false, Market: setting.Market, Symbol: setting.Symbol, Account: account,
		Setting:       setting,
		LimitSell:     limitAmount,
		LimitBuy:      limitAmount,
		AvailableSell: availableAmount,
		AvailableBuy:  availableAmount}
	valueInUsd := 0.0
	if cm.positions[setting.Symbol] != nil {
		carryStatus.Holding = cm.positions[setting.Symbol].Holding
		valueInUsd = math.Abs(carryStatus.Holding) * price
		carryStatus.RateInAll = valueInUsd / handledActValueInU
		if carryStatus.Holding < 0 {
			carryStatus.AvailableBuy = math.Max(availableAmount, math.Abs(carryStatus.Holding))
		}
		if carryStatus.Holding > 0 {
			carryStatus.AvailableSell = math.Max(availableAmount, carryStatus.Holding)
		}
	}
	// bitgetperp可以开仓或减仓，不越过0反向开仓
	if setting.Market == model.BitgetPerp && cm.positions[setting.Symbol] != nil {
		if cm.positions[setting.Symbol].Holding > 0 {
			carryStatus.ReduceOnlySell = true
			carryStatus.LimitSell = cm.positions[setting.Symbol].Holding
		} else if cm.positions[setting.Symbol].Holding < 0 {
			carryStatus.ReduceOnlyBuy = true
			carryStatus.LimitBuy = -1 * cm.positions[setting.Symbol].Holding
		}
	}
	rateLimitPosition := 2.8
	rateLimitHolding := 0.28
	switch setting.Market {
	case model.OKEX, model.Gate:
		if cm.mmr < 1.5 {
			util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %d %s %s mmr %f`,
				account.Index, setting.Market, setting.Symbol, cm.mmr))
			doRevert = true
		}
	case model.Bybit, model.BitgetPerp, model.BinancePerp:
		if cm.mmr > 0.66 {
			util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %s %s %s mmr %f`,
				account.Key, setting.Market, setting.Symbol, cm.mmr))
			doRevert = true
		}
	}
	if cm.contractValueInU/handledActValueInU > rateLimitPosition || valueInUsd > valueLimit ||
		valueInUsd/handledActValueInU > rateLimitHolding || (cm.collateralsAvailable < MarginULowLimit && cm.collateralsAvailable/handledActValueInU < 0.1) ||
		(setting.Market == model.BitgetPerp && (len(cm.positions) > model.BitgetPosLimit && carryStatus.Holding == 0)) {
		util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %d %s %s value big %f %f %f %f %f %f margin u %f pos len %d`,
			account.Index, setting.Market, setting.Symbol, cm.contractValueInU, handledActValueInU, rateLimitPosition, valueInUsd, valueLimit, rateLimitHolding, cm.contractValueInU, len(cm.positions)))
		doRevert = true
	}
	return carryStatus, doRevert
}

// absentRevert: 当cm或sm中没有这个symbol时，是否设置成revert模式
func createFromBalance(account *model.Account, setting *model.Setting, valueLimit float64) (carryStatus *model.CarryStatus, doRevert bool) {
	key := account.Key
	value, ok := spotMarkets.Load(key)
	if value == nil || !ok {
		spotMarkets.Store(key, createSpotMarket(key, account.Secret, setting.Market))
		value, ok = spotMarkets.Load(key)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`success set sm %d %s`, account.Index, setting.Market))
	}
	success, price := api.GetPriceForce(setting.Symbol, setting.Market)
	if value == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`nil spot market %s %s getPrice %#v %f`, setting.Market, setting.Symbol, success, price))
		return nil, true
	}
	sm := value.(*spotMarket)
	handledActValueInU := sm.accountValueInU
	if setting.Market == model.Gate {
		handledActValueInU *= 0.6
	}
	limitBuy, limitSell, availableBuy := 0.0, 0.0, 0.0
	if price > 0 {
		limitBuy = math.Min(sm.availableU/5, handledActValueInU/15) / price
		availableBuy = sm.availableU / price
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %s %s price 0`, setting.Market, setting.Symbol))
		doRevert = true
	}
	carryStatus = &model.CarryStatus{IsSpot: true, Market: setting.Market, Symbol: setting.Symbol, Account: account,
		Setting:       setting,
		LimitSell:     0,
		LimitBuy:      limitBuy,
		AvailableSell: 0,
		AvailableBuy:  availableBuy}
	if sm.balances[setting.Symbol] != nil {
		var balance *model.Balance
		balance = sm.balances[setting.Symbol]
		limitSell = math.Min(math.Max(balance.Amount, 0), balance.AvailableWithBorrow)
		carryStatus.Holding = balance.Amount
		// 暂不支持借币
		carryStatus.LimitSell, carryStatus.AvailableSell = limitSell, limitSell
		carryStatus.RateInAll = math.Abs(carryStatus.Holding * price / handledActValueInU)
	}
	usdLowLine := math.Min(100000, 0.1*handledActValueInU)
	if sm.availableU < usdLowLine || carryStatus.RateInAll > 0.2 {
		doRevert = true
	}
	if sm.balances[setting.Symbol] != nil && math.Abs(sm.balances[setting.Symbol].UsdValue) > valueLimit {
		doRevert = true
	}
	if setting.Market == model.Bybit && sm.collateral != nil && sm.collateral.Rate > 0.7 {
		doRevert = true
	} else if setting.Market == model.OKEX && sm.collateral != nil && sm.collateral.Rate < 1.5 {
		//(sm.collateral.Available-sm.collateral.Occupied)/sm.collateral.Available < 0.1) {
		doRevert = true
	} else if setting.Market == model.Gate && sm.collateral != nil && sm.collateral.Rate < 1.5 {
		doRevert = true
	}
	if doRevert {
		util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %d %s %s value big balance u %f<%f || %f>0.2 %f`,
			account.Index, setting.Market, setting.Symbol, sm.availableU, usdLowLine, carryStatus.RateInAll, valueLimit))
	}
	return carryStatus, doRevert
}

// absentRevert: 当cm或sm中没有这个symbol时，是否设置成revert模式
func initStatus(account *model.Account, setting *model.Setting, dirtyInit bool) (status *model.CarryStatus) {
	if setting == nil {
		return
	}
	//util.Log(util.LogLevelInfo, fmt.Sprintf(`start to init status %s %s %s`, setting.Coin, setting.Market, setting.Symbol))
	_, marketType, _, _ := model.GetFromStandard(setting.Market, setting.Symbol)
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
	}
	if setting.Chance < 0 {
		doRevert = true
	}
	if status == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to create status %s %s`, setting.Market, setting.Symbol))
		return nil
	}
	v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
	var marketInfo *model.MarketInfo
	if v != nil {
		marketInfo = v.(*model.MarketInfo)
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to create status get marketInfo %s %s`, setting.Market, setting.Symbol))
		return nil
	}
	if marketInfo != nil && marketInfo.SizeMax > 0 {
		status.AvailableBuy = math.Min(status.AvailableBuy, marketInfo.SizeMax)
		status.AvailableSell = math.Min(status.AvailableSell, marketInfo.SizeMax)
		if status.Market == model.Mexc { // mexc要求持仓不能超过1500张合约
			status.AvailableBuy = math.Min(status.AvailableBuy, 1400*marketInfo.SizeIncrement-status.Holding)
			status.AvailableSell = math.Min(status.AvailableSell, 1400*marketInfo.SizeIncrement+status.Holding)
		}
	}
	if setting.Market == model.OKEX {
		success, maxBuy, maxSell := api.GetTradeMaxOKEX(account.Key, account.Secret, setting.Symbol, 600)
		time.Sleep(time.Millisecond * 80)
		if success {
			status.AvailableBuy = math.Min(status.AvailableBuy, maxBuy)
			status.AvailableSell = math.Min(status.AvailableSell, maxSell)
		}
	}
	status.LimitBuy = math.Min(status.LimitBuy, status.AvailableBuy)
	status.LimitSell = math.Min(status.LimitSell, status.AvailableSell)
	initTradeLine(account, setting, status, doRevert, dirtyInit)
	util.StoreSyncMap(carryStatusMap, status, setting.Coin, setting.Market, setting.Symbol, account.Key)
	return
}

func initTradeLine(account *model.Account, setting *model.Setting, status *model.CarryStatus, doRevert, dirtyInit bool) {
	standardScoreBuy := math.Max(standardScoreOpen, setting.OpenShortMargin)
	standardScoreSell := math.Max(standardScoreOpen, setting.OpenShortMargin)
	getTick, ticks := model.AppEnvironment.GetBidAsk(setting.Market, setting.Symbol)
	price := 0.0
	if getTick {
		price = ticks.Asks[0].Price
	} else {
		util.LogLess(util.LogLevelError, fmt.Sprintf(`fail to get ticket %s %s`, setting.Market, setting.Symbol))
	}
	jumpOpen := 30.0
	switch setting.Market {
	case model.BinanceSpot, model.BinancePerp, model.BitgetPerp, model.BitgetSpot:
		jumpOpen = 20
	}
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
	if !dirtyInit || status.TradeLineBuy < 1 {
		status.TradeLineBuy = math.Max(standardScoreBuy*(0.5+jumpBuy*status.RateInAll), lowestScore) * account.CarryRate
	}
	if !dirtyInit || status.TradeLineSell < 1 {
		status.TradeLineSell = math.Max(standardScoreSell*(0.5+jumpSell*status.RateInAll), lowestScore) * account.CarryRate
	}
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

// FailOrdersReconnect 用于处理ws下单，但没有返回orderId的情况，此时有可能有成交
func FailOrdersReconnect() {
	ts := time.Now().Unix()
	failOrders := make(map[int]map[string]*model.Order)
	model.AppEnvironment.ReqIdOrders.Range(func(requestId, value interface{}) bool {
		if value == nil {
			return true
		}
		order := value.(*model.Order)
		orderTs := value.(*model.Order).OrderTime.Unix()
		if ts-orderTs > 180 && value.(*model.Order).OrderId == value.(*model.Order).ClientOrdId {
			if failOrders[order.AccountIndex] == nil {
				failOrders[order.AccountIndex] = make(map[string]*model.Order)
			}
			failOrders[order.AccountIndex][order.Market] = order
			util.Log(util.LogLevelInfo, fmt.Sprintf(`get old and del %s %s req %s %#v`,
				value.(*model.Order).Market, value.(*model.Order).Symbol, requestId, value))
			model.AppEnvironment.ReqIdOrders.Delete(requestId)
		}
		return true
	})
	for index, marketMap := range failOrders {
		accounts := model.GetAccounts(index)
		for market, order := range marketMap {
			api.HandleWsOrderConnFail(accounts[market], market, order)
		}
	}
}

var ClearCross = func() {
	if model.AppEnvironment.CrossEqualing {
		return
	}
	model.AppEnvironment.CrossEqualing = true
	doEqual := false
	if time.Now().Minute() == 30 {
		doEqual = true
	}
	traceId := time.Now().Unix()
	util.Log(util.LogLevelInfo, fmt.Sprintf("begin to clearing cross get set %s %v do equal %v %d",
		model.FunctionCross, model.AppEnvironment.CrossEqualing, doEqual, traceId))
	FailOrdersReconnect()
	compOrders.Clear()
	carryStatusMap.Clear()
	spotMarkets.Clear()
	contractMarkets.Clear()
	coinCrossing.Clear()
	model.AppEnvironment.ReqIdOrders.Clear()
	if model.AppConfig.Handle == `1` {
		equalAccounts(doEqual, traceId)
	}
	model.AppEnvironment.CrossEqualing = false
	util.Log(util.LogLevelInfo, fmt.Sprintf("end to clearing cross get set %v %d", model.AppEnvironment.CrossEqualing, traceId))
}

func equalAccounts(doEqual bool, traceId int64) {
	util.Log(util.LogLevelInfo, fmt.Sprintf(`enter clearing cross all %d`, traceId))
	waitEqual := make(map[int]bool)
	equalChannel := make(chan int, 1)
	if !doEqual {
		api.InitCrossMarketInfos(model.AppEnvironment.Markets)
		api.PrepareSettings()
		for _, market := range model.AppEnvironment.Markets {
			model.AppEnvironment.PubChanNeedReset.Store(market, true)
		}
	}
	//needWaitEqual := false // 是否需要进入等待环节
	for i := 0; i < api.GetCrossLen(); i++ {
		accounts := make(map[string]*model.Account)
		indexAccounts := model.GetAccounts(i)
		for _, market := range model.AppEnvironment.Markets {
			accounts[market] = indexAccounts[market]
		}
		waitEqual[i] = true
		go equalAccount(i, equalChannel, accounts, doEqual, traceId)
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
	for i := 0; i < api.GetCrossLen(); i++ {
		indexAccounts := model.GetAccounts(i)
		for _, market := range model.AppEnvironment.Markets {
			if !doEqual {
				liquidateSmallContracts(indexAccounts[market], market)
			}
			account := indexAccounts[market]
			if market == model.Gate && account != nil && model.AppConfig.GetCrossStyles()[i] == crossGrid {
				gateCm, _ := contractMarkets.Load(account.Key)
				if gateCm != nil {
					updateMoneyPerStep(gateCm.(*contractMarket))
				}
			}
		}
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`exit clearing cross all %d`, traceId))
}

func updateMoneyPerStep(gateCm *contractMarket) {
	value := api.GetCoinSettings(model.FunctionCross)
	if value == nil {
		return
	}
	value.Range(func(coin, settings interface{}) bool {
		item, _ := util.LoadSyncMap(carryCoinMap, coin.(string), `0`)
		if item == nil {
			return true
		}
		carryCoin := item.(*model.CarryCoin)
		symbol := coin.(string) + model.UniStandardTail[model.MarketTypePerp]
		if gateCm.positions == nil || gateCm.positions[symbol] == nil {
			return true
		}
		//status, _ := util.LoadSyncMap(carryStatusMap, coin.(string), model.Gate, symbol, account.Key)
		//if status == nil {
		//	return true
		//}
		//gateStatus := status.(*model.CarryStatus)
		pos := gateCm.positions[symbol]
		price := pos.EntryPrice
		if price == 0 {
			_, price = api.GetPriceForce(symbol, model.Gate)
		}
		if price == 0 {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`gate rist limit 0 price %s %s`, symbol, coin.(string)))
			return true
		}
		moneyRiskLimit := pos.RiskLimit / 20
		if moneyRiskLimit < carryCoin.MoneyPerStep {
			moneyInAll := carryCoin.MoneyPerStep*float64(carryCoin.CurrentStep) + carryCoin.MoneyCurStep
			carryCoin.CurrentStep = int((moneyInAll) / moneyRiskLimit)
			carryCoin.MoneyPerStep = moneyRiskLimit
			carryCoin.MoneyCurStep = moneyInAll - float64(carryCoin.CurrentStep)*carryCoin.MoneyPerStep
			util.Log(util.LogLevelInfo, fmt.Sprintf(`gate rist limit %s %f %f<%f price %f in all %f`,
				symbol, pos.RiskLimit, moneyRiskLimit, carryCoin.MoneyPerStep, price, moneyInAll))
			model.AppDB.Model(carryCoin).Where(`coin=? and account_index=?`, carryCoin.Coin, `0`).Updates(
				map[string]interface{}{`current_step`: carryCoin.CurrentStep, `money_cur_step`: carryCoin.MoneyCurStep,
					`money_per_step`: carryCoin.MoneyPerStep})
			util.Log(util.LogLevelInfo, fmt.Sprintf(`update money per step %s`, coin))
		}
		return true
	})
	return
}

func equalAccount(i int, equalChan chan int, accounts map[string]*model.Account, doEqual bool, traceId int64) {
	util.Log(util.LogLevelInfo, fmt.Sprintf(`begin to clearing cross %d trace %d`, i, traceId))
	for market, account := range accounts {
		if account.Index != i {
			continue
		}
		api.CancelAll(account.Key, account.Secret, market)
	}
	value := api.GetCoinSettings(model.FunctionCross)
	if value != nil {
		value.Range(func(coin, settings interface{}) bool {
			equalStatuses := make([]*model.CarryStatus, len(settings.([]*model.Setting)))
			for j, setting := range settings.([]*model.Setting) {
				account := accounts[setting.Market]
				if setting == nil || len(coin.(string)) == 0 || account == nil {
					util.Log(util.LogLevelError, `can not equal`)
					continue
				}
				equalStatuses[j] = initStatus(account, setting, false)
				if equalStatuses[j] == nil {
					util.Log(util.LogLevelError, fmt.Sprintf(`store carry nil coin %s %s %s %s %d`,
						setting.Coin, setting.Market, setting.Symbol, account.Key, account.Index))
					//} else {
					//	util.Log(util.LogLevelInfo, fmt.Sprintf(`create init status %s %s %d %#v`, setting.Coin, setting.Market, account.Index, equalStatuses[j]))
				}
			}
			coinCrossing.Store(coin.(string), false)
			for index := 0; index <= 10 && doEqual; index++ {
				coinEqual, leftHolding, errMsg := equalCoin(i, coin.(string), equalStatuses)
				if !coinEqual {
					util.Log(util.LogLevelInfo, fmt.Sprintf(
						`equal coin %s account %d equal %#v left hold %f err %s`, coin, i, coinEqual, leftHolding, errMsg))
				} else {
					break
				}
				if index == 10 {
					api.SendMails(fmt.Sprintf(`fail equal after 10 time %s`, coin),
						fmt.Sprintf(`%s holding %f`, coin, leftHolding))
				}
			}
			if model.AppConfig.GetCrossStyles()[i] == crossGrid {
				_, _, _, _, bidHolding, price, _ := getHolding(equalStatuses)
				if price == 0 {
					return true
				}
				valueCarryCoin, ok := util.LoadSyncMap(carryCoinMap, coin.(string), `0`)
				if !ok || valueCarryCoin == nil {
					carryCoin := api.GetCarryCoin(coin.(string))
					if carryCoin == nil {
						moneyPerStep, _ := strconv.ParseFloat(model.AppConfig.GetMoneyPerStep()[i], 64)
						carryCoin = &model.CarryCoin{Coin: coin.(string), MoneyPerStep: moneyPerStep}
						util.StoreSyncMap(carryCoinMap, carryCoin, coin.(string), `0`)
						model.AppDB.Save(carryCoin)
					} else {
						util.StoreSyncMap(carryCoinMap, carryCoin, coin.(string), `0`)
						util.Log(util.LogLevelInfo, fmt.Sprintf(`get a carry coin from db %v %f value %f`, coin, bidHolding, bidHolding*price))
					}
				}
			}
			return true
		})
	}
	equalChan <- i
	util.Log(util.LogLevelInfo, fmt.Sprintf(`exit clearing cross account index %d trace %d`, i, traceId))
}

// getHolding
// 注意:
//
//		1.获取到的是经过gridAmount 和priceX调整过后的price和amount
//	 2.获得到的价格是经过funding rate加权后的价格，实际下单时要进行还原
func getHolding(statuses []*model.CarryStatus) (bids, asks model.Ticks, statusMap map[string]*model.CarryStatus,
	holding, bidHolding, price float64, holdStr string) {
	bids = model.Ticks{}
	asks = model.Ticks{}
	statusMap = make(map[string]*model.CarryStatus)
	for _, status := range statuses {
		if status == nil {
			util.Log(util.LogLevelError, `warning: fail to get one status`)
			return
		}
		if status.Holding > 0 {
			bidHolding += status.Holding
		}
		holding += status.Holding * status.Setting.GridAmount
		holdStr += fmt.Sprintf(`[%s %s %f]`, status.Market, status.Symbol, status.Holding)
		marketInfo := model.GetMarketInfo(status.Market, status.Symbol)
		getTick, tick := model.AppEnvironment.GetBidAsk(status.Market, status.Symbol)
		if !getTick || marketInfo == nil {
			continue
		}
		getFunding, _, _, handledRate := handledFRate(status.Account, status.Market, status.Symbol, marketInfo.FundingRateInterval)
		if !getFunding {
			continue
		}
		priceBid := tick.Bids[0].Price * (1 + handledRate)
		priceAsk := tick.Asks[0].Price * (1 + handledRate)
		bids = append(bids, model.Tick{Ts: int64(tick.Ts), Market: tick.Bids[0].Market, Symbol: tick.Bids[0].Symbol,
			Amount: tick.Bids[0].Amount, Price: priceBid})
		asks = append(asks, model.Tick{Ts: int64(tick.Ts), Market: tick.Asks[0].Market, Symbol: tick.Asks[0].Symbol,
			Amount: tick.Asks[0].Amount, Price: priceAsk})
		statusMap[fmt.Sprintf(`%s_%s`, status.Market, status.Symbol)] = status
		if price == 0 {
			price = bids[0].Price / status.Setting.PriceX
		}
		if !status.Setting.Valid {
			util.Log(util.LogLevelError, fmt.Sprintf(`setting still invalid %s %s %s funding %f interval %d %#v`,
				status.Market, status.Symbol, status.Setting.Coin, handledRate, marketInfo.FundingRateInterval, status.Setting))
			status.Setting.Valid = true
		}
	}
	return bids, asks, statusMap, holding, bidHolding, price, holdStr
}

// settings []*model.Setting, coinStatus map[string]map[string]map[string]*CarryStatus
func equalCoin(index int, coin string, statuses []*model.CarryStatus) (isEqual bool, holding float64, errMsg string) {
	bids, asks, statusMap, holdingValue, _, holdingPrice, holdStr := getHolding(statuses)
	util.Log(util.LogLevelInfo, fmt.Sprintf(`compare holding %s status num index %d %d %s`, coin, index, len(statuses), holdStr))
	if math.IsNaN(holdingValue) {
		util.Log(util.LogLevelError, `hold value is NaN `)
		for _, status := range statusMap {
			util.Log(util.LogLevelError, fmt.Sprintf(`hold value is NaN %#v %#v`, status.Setting.GridAmount, status.Setting.PriceX))
		}
		return true, 0, ""
	}
	holding = holdingValue
	if math.Abs(holding) > compTooBig/holdingPrice {
		coinSettings := api.GetCoinSettings(model.FunctionCross)
		if coinSettings != nil {
			value, _ := coinSettings.Load(coin)
			if value != nil {
				for _, setting := range value.([]*model.Setting) {
					setting.Valid = false
					setting.MarketRelated = `too big comp when equal`
					util.Log(util.LogLevelInfo, fmt.Sprintf(`too big comp %s %s %f %f`,
						setting.Market, setting.Symbol, holding*holdingPrice, holding))
				}
			}
		}
		api.SendMails(`too big to equal`, fmt.Sprintf(`%s holding in money %f`, coin, holding*holdingPrice))
		return false, holding, fmt.Sprintf(`too big comp %s %f`, coin, holding)
	} else if math.Abs(holding) < CompLineInMoney/holdingPrice {
		return true, holding, ``
	}
	var equalStatus *model.CarryStatus
	sort.Sort(sort.Reverse(bids))
	for i := 0; i < len(bids); i++ {
		getBid, bidAsk := model.AppEnvironment.GetBidAsk(bids[i].Market, bids[i].Symbol)
		if !getBid || time.Now().UnixMilli()-int64(bidAsk.Ts) > 10000 {
			continue
		}
		price := bidAsk.Bids[0].Price * (1 - compSlide)
		if holding > CompLineInMoney/holdingPrice {
			status := statusMap[fmt.Sprintf(`%s_%s`, bids[i].Market, bids[i].Symbol)]
			if status == nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`no status when holding: %f %s %s`, holding, bids[i].Market, bids[i].Symbol))
				continue
			}
			checkAmount := model.GetAmountInMarket(status.Market, status.Symbol, math.Abs(holding/status.Setting.GridAmount), price, status.ReduceOnlySell)
			if checkAmount <= 0 {
				errMsg += fmt.Sprintf(`check amount %s %s %f < 0`, status.Market, status.Symbol, checkAmount)
				continue
			}
			amount := 0.0
			if status.AvailableSell > holding/status.Setting.GridAmount {
				equalStatus = status
				amount = math.Abs(holding) / status.Setting.GridAmount
			} else {
				checkAmount = model.GetAmountInMarket(status.Market, status.Symbol, status.AvailableSell, price, status.ReduceOnlySell)
				if checkAmount > 0 && status.AvailableSell*price > SmallInU {
					equalStatus = status
					//holding = status.AvailableSell
					amount = status.AvailableSell
				} else {
					util.Log(util.LogLevelError, fmt.Sprintf(`check 0 amount in %#v`, status))
					continue
				}
			}
			util.Log(util.LogLevelInfo, fmt.Sprintf(`need equal holding sell %s %f list %s tick ts %d price %f=%f slide %f equal status %#v`,
				coin, holding, holdStr, time.Now().UnixMilli()-int64(bidAsk.Ts), bids[i].Price, bidAsk.Bids[0].Price, price, equalStatus))
			dealAmount := placeEqual(equalStatus, price, amount, model.OrderSideSell) * equalStatus.Setting.GridAmount
			holding += dealAmount
			equalStatus.Holding += dealAmount
		}
	}
	sort.Sort(asks)
	for i := 0; i < len(asks); i++ {
		getAsk, bidAsk := model.AppEnvironment.GetBidAsk(asks[i].Market, asks[i].Symbol)
		if !getAsk || time.Now().UnixMilli()-int64(bidAsk.Ts) > 10000 {
			continue
		}
		price := bidAsk.Asks[0].Price * (1 + compSlide)
		if holding < -CompLineInMoney/holdingPrice {
			status := statusMap[fmt.Sprintf(`%s_%s`, asks[i].Market, asks[i].Symbol)]
			if status == nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`no status when holding: %f %s %s`, holding, asks[i].Market, asks[i].Symbol))
				continue
			}
			checkAmount := model.GetAmountInMarket(status.Market, status.Symbol, math.Abs(holding)/status.Setting.GridAmount, price, status.ReduceOnlyBuy)
			if checkAmount <= 0 {
				errMsg += fmt.Sprintf(`check amount %s %s %f < 0`, status.Market, status.Symbol, checkAmount)
				continue
			}
			amount := 0.0
			if math.IsNaN(status.AvailableBuy) || status.AvailableBuy > math.Abs(holding)/status.Setting.GridAmount {
				equalStatus = status
				amount = math.Abs(holding) / status.Setting.GridAmount
			} else if !math.IsNaN(status.AvailableBuy) {
				checkAmount = model.GetAmountInMarket(status.Market, status.Symbol, status.AvailableBuy, price, status.ReduceOnlyBuy)
				if checkAmount > 0 && status.AvailableBuy*price > SmallInU {
					equalStatus = status
					//holding = status.AvailableBuy
					amount = status.AvailableBuy
				} else {
					util.Log(util.LogLevelError, fmt.Sprintf(`check 0 amount in %#v`, status))
					continue
				}
			} else {
				continue
			}
			util.Log(util.LogLevelInfo, fmt.Sprintf(`need equal holding buy %s %f list %s tick ts %d price %f=%f slide %f equal status %#v`,
				coin, holding, holdStr, time.Now().UnixMilli()-int64(bidAsk.Ts), asks[i].Price, bidAsk.Asks[0].Price, price, equalStatus))
			dealAmount := placeEqual(equalStatus, price, amount, model.OrderSideBuy) * equalStatus.Setting.GridAmount
			holding += dealAmount
			equalStatus.Holding += dealAmount
		}
	}
	if math.Abs(holding) > CompLineInMoney/holdingPrice {
		isEqual = false
	} else {
		isEqual = true
	}
	return isEqual, holding, errMsg
}

func placeEqual(status *model.CarryStatus, price, amount float64, orderSide string) (dealAmount float64) {
	if status == nil {
		// 可能由于头寸太小，不满足所有市场的下单要求，而holdingU刚好大于10u，此时认为已平
		util.Log(util.LogLevelError, fmt.Sprintf(`can not get status to equal`))
		return 0
	}
	if status.Market == model.Ftx {
		amount = math.Min(90000000, amount)
	}
	amount = math.Min(amount, compLimitInU/price)
	reduceOnly := status.ReduceOnlySell
	if orderSide == model.OrderSideBuy {
		reduceOnly = status.ReduceOnlyBuy
	}
	checkAmount := model.GetAmountInMarket(status.Market, status.Symbol, amount, price, reduceOnly)
	if checkAmount > 0 {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`do equal %f %f %#v`, price, amount, status))
		order := api.PlaceOrder(status.Account, orderSide, model.OrderTypeLimit,
			status.Market, status.Symbol, ``, model.FunctionCompAll, price, price, amount, false, nil)
		if order != nil && order.Status != model.CarryStatusFail {
			compOrders.Store(order.OrderId, order)
			if orderSide == model.OrderSideBuy {
				dealAmount += amount
			} else {
				dealAmount -= amount
			}
			saveCross(order, status.TradeLineBuy, status.TradeLineSell, status.Holding)
			//if status.market == model.Gate {
			//	api.SetGateBidAsk(status.account.Key, status.account.Secret, status.symbol)
			//}
			if orderSide == model.OrderSideSell {
				placeStatus(status, price, -1*amount)
			} else if orderSide == model.OrderSideBuy {
				placeStatus(status, price, amount)
			}
		} else {
			if orderSide == model.OrderSideBuy {
				status.AvailableBuy = 0
			} else if orderSide == model.OrderSideSell {
				status.AvailableSell = 0
			}
		}
	}
	return dealAmount
}

// ProcessCross setting.Chance<0时该币种只关仓
// setting.OpenShortMargin OpenShortMargin不等于0时作为开舱标准价格，否则使用通用价格
// setting.CloseShortMargin CloseShortMargin作为开关舱标准价格
// setting.SymbolRelated 用作不同名称的搬砖时，对应交易所市场内现货的symbol，用户更新holding
// setting.GridAmount 用作不同名称的搬砖时，symbol的交易量对应的coin的交易量
// setting.PriceX 用作不同名称的搬砖时，symbol的交易价格对应的coin的交易价格
// Order.Fee 记录了原始下单的价格，用以判断最终comp成功时损失了多少
var ProcessCross = func(setting *model.Setting, tick *model.BidAsk) {
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
	//tsMark := time.Now().UnixMicro()
	ts1 := time.Now().UnixMilli()
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || setting.Valid == false ||
		model.AppEnvironment.CrossEqualing || model.AppEnvironment.CrossPause ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) || settings == nil || len(settings) == 0 ||
		time.Now().Minute() < 3 {
		return
	}
	// 同一个coin cross之间互斥
	replaced := coinCrossing.CompareAndSwap(setting.Coin, false, true)
	if !replaced {
		return
	}
	defer coinCrossing.Store(setting.Coin, false)
	tickLimit := 50
	switch tick.Bids[0].Market {
	case model.Gate:
		tickLimit = 25
	case model.BitgetSpot, model.BitgetPerp:
		tickLimit = 25
	case model.BinanceSpot, model.BinancePerp:
		tickLimit = 15
	case model.OKEX:
		tickLimit = 60
	case model.Bybit:
		tickLimit = 60
	}
	if int(ts1)-tick.Ts > tickLimit {
		//util.LogLess(util.LogLevelError, fmt.Sprintf(`abandon tick limit %s %s %s limit %d %v`,
		//	setting.Coin, tick.Bids[0].Market, tick.Bids[0].Symbol, int(ts1)-tick.Ts, model.AppEnvironment.CrossEqualing))
		return
	}
	for _, settingRelate := range settings {
		tickGet, tickRelate := model.AppEnvironment.GetBidAsk(settingRelate.Market, settingRelate.Symbol)
		if !tickGet || setting.ID == settingRelate.ID || (!model.NonRTTicker[tick.Bids[0].Market] && model.NonRTTicker[tickRelate.Bids[0].Market]) || !settingRelate.Valid {
			continue
		}
		tickLimit = 500
		if int(ts1)-tickRelate.Ts > tickLimit {
			//util.LogLess(util.LogLevelError, fmt.Sprintf(`abandon tick limit relate %s %s %s limit %v`,
			//	setting.Coin, tick.Bids[0].Market, tick.Bids[0].Symbol, model.AppEnvironment.CrossEqualing))
			continue
		}
		for i := api.GetCrossLen() - 1; i >= 0; i-- {
			account := model.AppConfig.GetAccounts(setting.Market)[i]
			accountRelate := model.AppConfig.GetAccounts(settingRelate.Market)[i]
			if account == nil || accountRelate == nil {
				continue
			}
			lastOrder, _ := model.AppEnvironment.LastOrderMilli.Load(account.Key)
			lastOrderRelate, _ := model.AppEnvironment.LastOrderMilli.Load(accountRelate.Key)
			if (lastOrder != nil && ts1-lastOrder.(int64) < AccountOrderGap) || (lastOrderRelate != nil && ts1-lastOrderRelate.(int64) < AccountOrderGap) {
				continue
			}
			status, getStatus := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, account.Key)
			statusRelate, getRelate := util.LoadSyncMap(carryStatusMap, settingRelate.Coin, settingRelate.Market, settingRelate.Symbol, accountRelate.Key)
			carryCoin, getCoin := util.LoadSyncMap(carryCoinMap, setting.Coin, `0`)
			if status == nil || statusRelate == nil || status == statusRelate || carryCoin == nil || !getStatus || !getRelate || !getCoin {
				continue
			}
			delay, statusBuy, statusSell, amount, priceBuy, priceSell :=
				calcAmount(i, setting.Coin, status.(*model.CarryStatus), statusRelate.(*model.CarryStatus), carryCoin.(*model.CarryCoin), tick, tickRelate)
			if delay {
				return
			}
			if amount > 0 {
				//tsDis2 := time.Now().UnixMicro() - tsMark
				placeCross(carryCoin.(*model.CarryCoin), statusBuy, statusSell, priceBuy, priceSell, amount)
				//util.Log(util.LogLevelInfo, fmt.Sprintf(`time mark coin %s %s %s <- %s %s amt %f ts %d %d %d %d`,
				//	setting.Coin, statusBuy.Symbol, statusBuy.Market, statusSell.Symbol, statusSell.Market, amount, tsMark, tsDis1, tsDis2, time.Now().UnixMicro()-tsMark))
				model.AppEnvironment.LastOrderMilli.Store(statusBuy.Account.Key, time.Now().UnixMilli())
				model.AppEnvironment.LastOrderMilli.Store(statusSell.Account.Key, time.Now().UnixMilli())
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
			util.LogLess(util.LogLevelError, fmt.Sprintf(
				"币种：%s 市场 %s 有限价条件，但是没有标记价格", setting.Symbol, setting.Market))
			api.GetMarkPrice(account, setting.Market, setting.Symbol)
			return true
		}
		bidMaxPrice := markPriceInfo.MarkPrice * (1 + marketInfo.BuyLimitPriceRatio)
		bidMinPrice := markPriceInfo.MarkPrice * (1 - marketInfo.BuyLimitPriceRatio)
		if price > bidMaxPrice || price < bidMinPrice {
			util.LogLess(util.LogLevelInfo, fmt.Sprintf("币种：%s %s 被限买价，买上浮：%f，标记价：%f，限最高买价：%f，限最低买价：%f，当前最佳买价：%f",
				setting.Market, setting.Symbol, marketInfo.BuyLimitPriceRatio, markPriceInfo.MarkPrice, bidMaxPrice, bidMinPrice, price))
			return true
		}
	} else if marketInfo != nil && orderSide == model.OrderSideSell && marketInfo.SellLimitPriceRatio > 0 {
		markPriceInfo := model.AppEnvironment.GetMarkPriceInfo(setting.Symbol, setting.Market)
		if markPriceInfo == nil {
			util.LogLess(util.LogLevelError, fmt.Sprintf("币种：%s 市场 %s 有限价条件，但是没有标记价格", setting.Symbol, setting.Market))
			api.GetMarkPrice(account, setting.Market, setting.Symbol)
			return true
		}
		askMaxPrice := markPriceInfo.MarkPrice * (1 + marketInfo.SellLimitPriceRatio)
		askMinPrice := markPriceInfo.MarkPrice * (1 - marketInfo.SellLimitPriceRatio)
		if price > askMaxPrice || price < askMinPrice {
			util.LogLess(util.LogLevelError, fmt.Sprintf("币种：%s %s 被限卖价，卖下浮：%f，标记价：%f，限最高卖价：%f，限最低卖价：%f，当前最佳卖价：%f",
				setting.Market, setting.Symbol, marketInfo.SellLimitPriceRatio, markPriceInfo.MarkPrice, askMaxPrice, askMinPrice, price))
			//perpAskPrice = askMinPrice
			return true
		}
	}
	return false
}

//func placeOKPair() {
//if statusBuy.Market == model.OKEX && statusSell.Market == model.OKEX {
//	requestId := strconv.FormatInt(time.Now().UnixMicro(), 10)[3:]
//	clientOrdIdBuy := requestId + `b`
//	clientOrdIdSell := requestId + `s`
//	orderBuy := &model.Order{OrderSide: model.OrderSideBuy, OrderType: model.OrderTypeLimit, Market: model.OKEX,
//		Symbol: statusBuy.Symbol, Price: priceBuy, Amount: amountBuy, RefreshType: model.FunctionCross, OrderTime: util.GetNow(),
//		UnfilledQuantity: amountBuy, AccountIndex: statusBuy.Account.Index, Status: model.CarryStatusWorking, Function: model.Open,
//		OrderId: clientOrdIdBuy, ClientOrdId: clientOrdIdBuy, LineBuy: statusBuy.TradeLineBuy, LineSell: statusSell.TradeLineSell}
//	orderSell := &model.Order{OrderSide: model.OrderSideSell, OrderType: model.OrderTypeLimit, Market: model.OKEX,
//		Symbol: statusSell.Symbol, Price: priceSell, Amount: amountSell, RefreshType: model.FunctionCross, OrderTime: util.GetNow(),
//		UnfilledQuantity: amountSell, AccountIndex: statusSell.Account.Index, Status: model.CarryStatusWorking, Function: model.Open,
//		OrderId: clientOrdIdSell, ClientOrdId: clientOrdIdSell, LineBuy: statusSell.TradeLineBuy, LineSell: statusSell.TradeLineSell}
//	if statusBuy.Holding*-1 >= amountBuy {
//		orderBuy.Function = model.Close
//	}
//	if statusSell.Holding >= amountSell {
//		orderSell.Function = model.Close
//	}
//	orderBuy.Coin = statusBuy.Setting.Coin
//	orderSell.Coin = statusSell.Setting.Coin
//	success, msg := api.PlacePairOKEX(statusBuy.Account, requestId, statusBuy.Symbol, statusSell.Symbol, model.OrderTypeLimit,
//		priceBuy, priceSell, amountBuy, amountSell)
//	if success {
//		model.AppEnvironment.ReqIdOrders.Store(requestId+model.OrderSideBuy, orderBuy)
//		model.AppEnvironment.ReqIdOrders.Store(requestId+model.OrderSideSell, orderSell)
//		model.AppDB.Save(orderBuy)
//		model.AppDB.Save(orderSell)
//	} else {
//		orderBuy.Status, orderSell.Status = model.CarryStatusFail, model.CarryStatusFail
//		orderBuy.ErrCode, orderSell.ErrCode = msg, msg
//	}
//} else {
//}
//}

func placeCross(carryCoin *model.CarryCoin, statusBuy, statusSell *model.CarryStatus, priceBuy, priceSell, amount float64) {
	_, marketTypeBuy, _, _ := model.GetFromStandard(statusBuy.Market, statusBuy.Symbol)
	_, marketTypeSell, _, _ := model.GetFromStandard(statusSell.Market, statusSell.Symbol)
	if marketTypeBuy == model.MarketTypeSpot {
		priceBuy = priceBuy * (1 + crossSlide)
	}
	if marketTypeSell == model.MarketTypeSpot {
		priceSell = priceSell * (1 - crossSlide)
	}
	score := (priceSell - priceBuy) / math.Max(priceBuy, priceSell)
	amountBuy := amount / statusBuy.Setting.GridAmount
	amountSell := amount / statusSell.Setting.GridAmount
	go api.PlaceOrder(statusBuy.Account, model.OrderSideBuy, model.OrderTypeLimit, statusBuy.Market,
		statusBuy.Symbol, ``, model.FunctionCross, priceBuy, priceBuy, amountBuy, true, PostOrderCross)
	go api.PlaceOrder(statusSell.Account, model.OrderSideSell, model.OrderTypeLimit, statusSell.Market,
		statusSell.Symbol, ``, model.FunctionCross, priceSell, priceSell, amountSell, true, PostOrderCross)
	util.Log(util.LogLevelInfo, fmt.Sprintf(
		`place cross %s %s -> %s %s at %f %f amount %f %f %f score %f hold %f buy %f hold %f sell %f`,
		statusSell.Market, statusSell.Symbol, statusBuy.Market, statusBuy.Symbol, priceSell, priceBuy, amount, amountBuy,
		amountSell, score, statusBuy.Holding, statusBuy.TradeLineBuy, statusSell.Holding, statusSell.TradeLineSell))
	// 买入现货时要交手续费，故而实际到手少于下单量，校准以免未来买单时数量不足
	if marketTypeBuy == model.MarketTypeSpot {
		amountBuy = amountBuy * 0.9992
		//util.Log(util.LogLevelInfo, fmt.Sprintf(`spot buy amount before %d %s %s now %f %f buy %f sell %f`,
		//	statusBuy.Account.Index, statusBuy.Market, statusBuy.Symbol, statusBuy.LimitSell, statusBuy.AvailableSell, amountBuy, amountSell))
	}
	if carryCoin != nil && model.AppConfig.GetCrossStyles()[statusBuy.Account.Index] == crossGrid {
		carryCoin.AddTrade(statusBuy, statusSell, priceBuy, priceSell, amountSell)
	}
	placeStatus(statusBuy, priceBuy, amountBuy)
	placeStatus(statusSell, priceSell, -1*amountSell)
	//if marketTypeBuy == model.MarketTypeSpot {
	//	value, _ := util.LoadSyncMap(carryStatusMap, statusBuy.Setting.Coin, statusBuy.Market, statusBuy.Symbol, statusBuy.Account.Key)
	//	if value != nil {
	//		statusBuy = value.(*model.CarryStatus)
	//		util.Log(util.LogLevelInfo, fmt.Sprintf(`spot buy amount after %d %s %s now %f %f`,
	//			statusBuy.Account.Index, statusBuy.Market, statusBuy.Symbol, statusBuy.LimitSell, statusBuy.AvailableSell))
	//	}
	//}
}

func placeStatus(status *model.CarryStatus, price float64, amount float64) {
	valueSpot, _ := spotMarkets.Load(status.Account.Key)
	valueContract, _ := contractMarkets.Load(status.Account.Key)
	if status.IsSpot && valueSpot != nil {
		balance := valueSpot.(*spotMarket).balances[status.Symbol]
		if balance == nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`warning no balance %s %s %s`, status.Account.Key, status.Market, status.Symbol))
			balance = &model.Balance{Amount: amount, UsdValue: amount * price, Market: status.Market, Coin: status.Setting.Coin}
			valueSpot.(*spotMarket).balances[status.Symbol] = balance
		} else {
			balance.Amount += amount
			balance.UsdValue = balance.Amount * price
			balance.AvailableWithBorrow += amount
		}
		valueSpot.(*spotMarket).availableU -= amount * price
		if status.Account.IsUnified {
			if valueContract != nil {
				valueContract.(*contractMarket).collateralsAvailable -= amount * price
			}
			valueSpot.(*spotMarket).collateral.Available -= amount * price
		}
		status.AvailableSell += amount
	}
	if !status.IsSpot && valueContract != nil {
		position := valueContract.(*contractMarket).positions[status.Symbol]
		originFreeAbs := 0.0
		if position == nil {
			position = &model.Position{Holding: amount, EntryPrice: price, Market: status.Market, Currency: status.Setting.Symbol}
			valueContract.(*contractMarket).positions[status.Symbol] = position
		} else {
			originFreeAbs = math.Abs(position.Holding)
			position.Holding += amount
			position.EntryPrice = price
		}
		changeU := (originFreeAbs - math.Abs(position.Holding)) * price
		valueContract.(*contractMarket).collateralsAvailable += changeU * 0.2
		valueContract.(*contractMarket).contractValueInU += changeU
		if valueSpot != nil {
			if status.Market == model.Ftx {
				valueSpot.(*spotMarket).availableU += changeU * 0.2
			} else if status.Market == model.OKEX {
				valueSpot.(*spotMarket).collateral.Available += changeU * 0.1
				valueSpot.(*spotMarket).collateral.Occupied -= changeU * 0.1
				valueSpot.(*spotMarket).availableU += changeU * 0.1
			} else if status.Account.IsUnified {
				valueSpot.(*spotMarket).collateral.Available += changeU * 0.2
				valueSpot.(*spotMarket).collateral.Occupied -= changeU * 0.2
				valueSpot.(*spotMarket).availableU += changeU * 0.2
			}
		}
	}
	account := model.AppConfig.GetAccountFromKeyIndex(status.Market, status.Account.Key, -1)
	initStatus(account, status.Setting, true)
}

func handleCross(account *model.Account, order *model.Order) {
	if order.Status == model.CarryStatusWorking {
		time.Sleep(time.Minute)
	}
	value, _ := model.AppEnvironment.OrderIdOrders.Load(order.OrderId)
	if value == nil {
		return
	}
	order = value.(*model.Order)
	if order.Amount == order.DealAmount {
		order.Status = model.CarryStatusSuccess
	}
	v, _ := util.LoadSyncMap(model.MarketInfos, order.Market, order.Symbol)
	var marketInfo *model.MarketInfo
	if v != nil {
		marketInfo = v.(*model.MarketInfo)
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`not found marketInfo %s %s`, order.Market, order.Symbol))
		return
	}
	leftAmt := order.Amount - order.DealAmount
	model.AppEnvironment.OrderIdOrders.Delete(order.OrderId)
	if order.Status == model.CarryStatusFail {
		compOrder(account, order, leftAmt)
	} else if leftAmt > marketInfo.SizeMin && leftAmt*order.Price > marketInfo.MoneyMin && order.Status != model.CarryStatusSuccess && order.HaveId() {
		api.CancelOrder(account.Key, account.Secret, order.Market, order.Symbol, model.OrderTypeLimit, order.OrderId)
		time.Sleep(time.Second * 10)
		queryOrder := api.QueryOrderById(account.Key, account.Secret, order.Market, order.Symbol, order.OrderType, order.OrderId)
		if queryOrder != nil {
			leftAmt = queryOrder.Amount - queryOrder.DealAmount
			if leftAmt > marketInfo.SizeMin && leftAmt*order.Price > marketInfo.MoneyMin && queryOrder.Status != model.CarryStatusSuccess {
				compOrder(account, order, leftAmt)
			} else {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`order update fail %#v left %f %#v`, order, leftAmt, queryOrder))
			}
		} else {
			util.Log(util.LogLevelError, fmt.Sprintf(`order update fail query %#v`, order))
		}
	} else {
		if order.HaveId() {
			//util.Log(util.LogLevelInfo, fmt.Sprintf(`post handle done %#v`, order))
			order.Status = model.CarryStatusSuccess
		} else {
			order.OrderId = fmt.Sprintf("%d%s%s", time.Now().UnixMilli(), order.Market, order.Symbol)
			order.Status = model.CarryStatusFail
			util.Log(util.LogLevelError, fmt.Sprintf(`handle have no id %s %s %#v`, order.Market, order.Symbol, order))
		}
	}
	if order.ClientOrdId != `` {
		model.AppDB.Model(order).Where("client_ord_id = ?", order.ClientOrdId).Updates(map[string]interface{}{
			`deal_amount`: order.DealAmount, `deal_price`: order.DealPrice, `err_code`: order.ErrCode, `order_id`: order.OrderId, `status`: order.Status, `order_time`: order.OrderTime})
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`no client order id when handle cross %#v`, order))
	}
}

func ContinueComp() {
	for {
		compOrders.Range(func(key, value interface{}) bool {
			if value == nil {
				return true
			}
			order := value.(*model.Order)
			if order.OrderTime.Add(time.Minute).After(time.Now()) {
				return true
			}
			var marketInfo *model.MarketInfo
			v, _ := util.LoadSyncMap(model.MarketInfos, order.Market, order.Symbol)
			if v != nil {
				marketInfo = v.(*model.MarketInfo)
			} else {
				util.Log(util.LogLevelError, fmt.Sprintf(`continueComp fail not found marketInfo %s %s`, order.Market, order.Symbol))
				return true
			}
			account := model.AppConfig.GetAccountFromKeyIndex(order.Market, ``, order.AccountIndex)
			leftAmt := order.Amount
			queryOrder := api.QueryOrderById(account.Key, account.Secret, order.Market, order.Symbol, order.OrderType, order.OrderId)
			if queryOrder != nil {
				leftAmt = queryOrder.Amount - queryOrder.DealAmount
			} else {
				util.Log(util.LogLevelError, fmt.Sprintf(`continueComp fail to get comp order %s %s %s %#v`, order.Market, order.Symbol, order.OrderId, order))
				return true
			}
			price := order.Price
			_, bidAsk := model.AppEnvironment.GetBidAsk(order.Market, order.Symbol)
			if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 60000 {
				util.Log(util.LogLevelError, fmt.Sprintf(`continueComp fail can not get bidask for comp %s %s`, order.Market, order.Symbol))
				if order.OrderSide == model.OrderSideSell {
					price = price * (1 - compSlide)
				} else {
					price = price * (1 + compSlide)
				}
			} else {
				if order.OrderSide == model.OrderSideSell {
					price = bidAsk.Bids[0].Price * (1 - compSlide)
				} else {
					price = bidAsk.Asks[0].Price * (1 + compSlide)
				}
			}
			if leftAmt > marketInfo.SizeMin && leftAmt*order.Price > math.Max(10, marketInfo.MoneyMin) && queryOrder.Status != model.CarryStatusSuccess {
				result, _, _ := api.CancelOrder(account.Key, account.Secret, order.Market, order.Symbol, model.OrderTypeLimit, order.OrderId)
				if result {
					compOrders.Delete(order.OrderId)
					if model.AppEnvironment.CrossEqualing {
						return false
					}
					orderComp := api.PlaceOrder(account, order.OrderSide, model.OrderTypeLimit, order.Market, order.Symbol, ``,
						order.RefreshType, price, price, leftAmt, false, nil)
					if orderComp != nil && orderComp.Status != model.CarryStatusFail {
						orderComp.Fee = order.Fee
						compOrders.Store(orderComp.OrderId, orderComp)
					}
					model.AppDB.Save(orderComp)
					util.Log(util.LogLevelError, fmt.Sprintf(`continueComp success on fail to comp %s %s %f %f-%f ts %d %#v new comp %#v`,
						order.Market, order.Symbol, price, bidAsk.Bids[0].Price, bidAsk.Asks[0].Price, bidAsk.Ts, order, orderComp))
				} else {
					util.Log(util.LogLevelError, fmt.Sprintf(`continueComp fail to cancel %s %s %s %#v`, order.Market, order.Symbol, order.OrderId, order))
				}
			} else {
				compOrders.Delete(order.OrderId)
				util.Log(util.LogLevelInfo, fmt.Sprintf(`continueComp success comp no left %f/%f %s %s %#v`, leftAmt, order.Amount, order.Market, order.Symbol, queryOrder))
			}
			return true
		})
		time.Sleep(time.Second * 10)
	}
}

var PostOrderCross = func(order *model.Order) {
	if order == nil {
		return
	}
	setting := api.GetSetting(model.FunctionCross, order.Market, order.Symbol)
	if setting == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to get setting %s %s %s`, model.FunctionCross, order.Market, order.Symbol))
		return
	}
	account := model.AppConfig.GetAccountFromKeyIndex(order.Market, ``, order.AccountIndex)
	go handleCross(account, order)
	if !order.HaveId() || order.ErrCode != `` || order.Status == model.CarryStatusFail {
		value, _ := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, account.Key)
		if value != nil {
			status := value.(*model.CarryStatus)
			if order.OrderSide == model.OrderSideSell {
				status.TradeLineSell = 1
				status.LimitSell = 0
			}
			if order.OrderSide == model.OrderSideBuy {
				status.TradeLineBuy = 1
				status.LimitBuy = 0
			}
			util.Log(util.LogLevelError, fmt.Sprintf(`set trade line 1 fail order %s %s %s %s %s %s %s`,
				setting.Coin, setting.Market, setting.Symbol, account.Key, order.OrderId, order.ErrCode, order.OrderTime.Format(time.DateTime)))
		}
		//addCarryResult(account.Key, order.Market, ``, false)
		//unknownFail := true
		//if account != nil {
		//	status, ok := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, account.Key)
		//	switch order.Market {
		//	case model.OKEX:
		//		if InsufficientCodeOKEX[order.ErrCode] && setting != nil {
		//			util.Log(util.LogLevelInfo, fmt.Sprintf(
		//				`reset %s trade max with %s account index %d`, order.Market, order.ErrCode, order.AccountIndex))
		//			getMax, maxBuy, maxSell := api.GetTradeMaxOKEX(account.Key, account.Secret, setting.Symbol, 0)
		//			if getMax && ok && status != nil {
		//				status.(*CarryStatus).LimitSell = math.Min(status.(*CarryStatus).LimitSell, maxSell)
		//				status.(*CarryStatus).LimitBuy = math.Min(status.(*CarryStatus).LimitBuy, maxBuy)
		//			}
		//			unknownFail = false
		//		}
		//	case model.BinancePerp, model.BinanceSpot:
		//		if strings.Contains(InsufficientCodeBinance, order.ErrCode) {
		//			util.Log(util.LogLevelError, fmt.Sprintf(
		//				`reset binance trade max with %s account index %d`, order.ErrCode, order.AccountIndex))
		//			spotMarkets.Delete(account.Key)
		//			contractMarkets.Delete(account.Key)
		//			initStatus(account, setting)
		//			unknownFail = false
		//		}
		//	}
		//	util.Log(util.LogLevelInfo, fmt.Sprintf(
		//		`set 1 trade line after fail %s %s %s`, setting.Market, setting.Symbol, order.OrderSide))
		//	if status != nil {
		//		initTradeLine(account, setting, status.(*CarryStatus), true)
		//	}
		//}
		//if unknownFail {
		//	addCarryResult(account.Key, order.Market, order.ErrCode, false)
		//} else {
		//	addCarryResult(account.Key, order.Market, ``, true)
		//}
	}
}

// Order.Fee 记录了原始下单的价格，用以判断最终comp成功时损失了多少
func compOrder(account *model.Account, order *model.Order, leftAmt float64) {
	if !model.AppEnvironment.CrossEqualing {
		price := order.Price
		_, bidAsk := model.AppEnvironment.GetBidAsk(order.Market, order.Symbol)
		if bidAsk == nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`can not get bidask for comp %s %s`, order.Market, order.Symbol))
		} else {
			if order.OrderSide == model.OrderSideSell {
				price = bidAsk.Bids[0].Price * (1 - compSlide)
			} else {
				price = bidAsk.Asks[0].Price * (1 + compSlide)
			}
		}
		comp := api.PlaceOrder(account, order.OrderSide, model.OrderTypeLimit, order.Market, order.Symbol,
			``, model.FunctionComplement, price, price, leftAmt, false, nil)
		comp.Fee = order.Price
		compOrders.Store(comp.OrderId, comp)
		model.AppDB.Save(comp)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`post comp from %#v %#v not deal %f 百分之%f`,
			order, comp, leftAmt, math.Round(100*leftAmt/order.Amount)))
	} else {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`comp all processing and ignore %#v`, order))
	}
	time.Sleep(time.Millisecond * 100)
}

// FormatCrossPair 不支持以BTC或ETH计价的交易对，只支持USD类
// amountLimit=0表示无限制
func FormatCrossPair(statusBuy, statusSell *model.CarryStatus, bidAmount, askAmount, amountLimit, priceBuy, priceSell float64) (formattedAmount float64) {
	v, _ := util.LoadSyncMap(model.MarketInfos, statusBuy.Setting.Market, statusBuy.Setting.Symbol)
	var marketInfoBuy, marketInfoSell *model.MarketInfo
	if v != nil {
		marketInfoBuy = v.(*model.MarketInfo)
	}
	v, _ = util.LoadSyncMap(model.MarketInfos, statusSell.Setting.Market, statusSell.Setting.Symbol)
	if v != nil {
		marketInfoSell = v.(*model.MarketInfo)
	}
	if marketInfoBuy == nil || marketInfoSell == nil {
		util.Log(util.LogLevelInfo, fmt.Sprintf(
			`format %s %s %s %s %#v %#v`, statusBuy.Market, statusSell.Market, statusBuy.Symbol, statusSell.Symbol, marketInfoBuy, marketInfoSell))
		return
	}
	formattedAmount = math.Min(math.Min(statusBuy.LimitBuy, bidAmount)*statusBuy.Setting.GridAmount,
		math.Min(statusSell.LimitSell, askAmount)*statusSell.Setting.GridAmount)
	formattedAmount = math.Min(formattedAmount, statusBuy.Setting.GridAmount*openValueLimit/priceBuy)
	if amountLimit > 0 {
		formattedAmount = math.Min(formattedAmount, amountLimit)
	}
	minBuy := marketInfoBuy.SizeMin * statusBuy.Setting.GridAmount
	minSell := marketInfoSell.SizeMin * statusSell.Setting.GridAmount
	if statusBuy.Setting.Market == model.Bybit {
		minBuy = math.Max(5.5/priceBuy*statusBuy.Setting.GridAmount, minBuy)
	}
	if statusSell.Setting.Market == model.Bybit {
		minSell = math.Max(5.5/priceSell*statusSell.Setting.GridAmount, minSell)
	}
	if marketInfoBuy.MoneyMin > 0 {
		minBuy = math.Max(minBuy, marketInfoBuy.MoneyMin/priceBuy*statusBuy.Setting.GridAmount)
	}
	if marketInfoSell.MoneyMin > 0 {
		minSell = math.Max(minSell, marketInfoSell.MoneyMin/priceSell*statusSell.Setting.GridAmount)
	}
	//amountBuy := model.GetAmountInMarket(marketBuy, symbolBuy, amount, priceBuy, false)
	//_, amountBuy = model.ParseRealAmount(marketBuy, symbolBuy, amountBuy)
	//amountSell := model.GetAmountInMarket(marketSell, symbolSell, amount, priceBuy, false)
	//_, amountSell = model.ParseRealAmount(marketSell, symbolSell, amountSell)
	//formattedAmount = math.Min(amountBuy, amountSell)
	if formattedAmount < math.Max(minBuy, minSell) {
		return 0
	}
	return formattedAmount
}

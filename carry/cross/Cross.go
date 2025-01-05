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
	util.Log(util.LogLevelInfo, fmt.Sprintf(`get positions %s %s %#v account value %f available u %f maintain rate %f positions %#v`,
		market, key, success, accountValue, availableU, mmr, positions))
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
	//util.Info(fmt.Sprintf(`create sm %s %s`, key[:5], market))
	success, balances, totalInUsd, collateral := api.GetBalances(key, secret, market)
	util.Log(util.LogLevelInfo, fmt.Sprintf(`create spot market %s %#v`, market, balances))
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
func createFromPosition(account *model.Account, setting *model.Setting, valueLimit float64) (carryStatus *CarryStatus, doRevert bool) {
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
		util.Log(util.LogLevelError, fmt.Sprintf(`nil contract market %s %s`, setting.Market, setting.Symbol))
		return nil, false
	}
	cm := value.(*contractMarket)
	_, price := api.GetPriceForce(setting.Symbol, setting.Market)
	limitAmount := 0.0
	availableAmount := 0.0
	if price > 0 {
		limitAmount = math.Min(cm.accountValueInU/5, cm.collateralsAvailable) / price
		if setting.Market == model.Gate {
			riskLimit, loaded := util.LoadSyncMap(&model.AppEnvironment.RiskLimitsGate, account.Key, setting.Symbol)
			if loaded {
				limitAmount = math.Min(limitAmount, riskLimit.(float64))
			}
		}
		availableAmount = cm.collateralsAvailable / price
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %s %s price 0`, setting.Market, setting.Symbol))
		doRevert = true
	}
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
			carryStatus.reduceOnlySell = true
			carryStatus.LimitSell = cm.positions[setting.Symbol].Holding
		} else if cm.positions[setting.Symbol].Holding < 0 {
			carryStatus.reduceOnlyBuy = true
			carryStatus.LimitBuy = -1 * cm.positions[setting.Symbol].Holding
		}
	}
	rateLimitPosition := 2.8
	rateLimitHolding := 0.28
	switch setting.Market {
	case model.OKEX, model.Gate:
		if cm.mmr < 1.5 {
			util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %s %s %s mmr %f`,
				account.Key, setting.Market, setting.Symbol, cm.mmr))
			doRevert = true
		}
	case model.Bybit, model.BitgetPerp, model.BinancePerp:
		if cm.mmr > 0.66 {
			util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %s %s %s mmr %f`,
				account.Key, setting.Market, setting.Symbol, cm.mmr))
			doRevert = true
		}
	}
	if cm.contractValueInU/cm.accountValueInU > rateLimitPosition || valueInUsd > valueLimit ||
		valueInUsd/cm.accountValueInU > rateLimitHolding ||
		(setting.Market == model.BitgetPerp && (len(cm.positions) > BitgetPosLimit && carryStatus.Holding == 0)) {
		util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %s %s value big %f %f %f %f %f %f pos len %d`,
			setting.Market, setting.Symbol, cm.contractValueInU, cm.accountValueInU, rateLimitPosition, valueInUsd, valueLimit, rateLimitHolding, len(cm.positions)))
		doRevert = true
	}
	return carryStatus, doRevert
}

// absentRevert: 当cm或sm中没有这个symbol时，是否设置成revert模式
func createFromBalance(account *model.Account, setting *model.Setting, valueLimit float64) (carryStatus *CarryStatus, doRevert bool) {
	key := account.Key
	value, ok := spotMarkets.Load(key)
	if value == nil || !ok {
		spotMarkets.Store(key, createSpotMarket(key, account.Secret, setting.Market))
		value, ok = spotMarkets.Load(key)
	}
	success, price := api.GetPriceForce(setting.Symbol, setting.Market)
	if value == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`nil spot market %s %s getPrice %#v %f`, setting.Market, setting.Symbol, success, price))
		return nil, true
	}
	sm := value.(*spotMarket)
	limitBuy, limitSell, availableBuy := 0.0, 0.0, 0.0
	if price > 0 {
		limitBuy = math.Min(sm.availableU/5, sm.accountValueInU/15) / price
		availableBuy = sm.availableU / price
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %s %s price 0`, setting.Market, setting.Symbol))
		doRevert = true
	}
	carryStatus = &CarryStatus{isSpot: true, market: setting.Market, symbol: setting.Symbol, account: account,
		setting:       setting,
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
		carryStatus.RateInAll = math.Abs(carryStatus.Holding * price / sm.accountValueInU)
	}
	usdLowLine := math.Min(100000, 0.1*sm.accountValueInU)
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
		util.Log(util.LogLevelError, fmt.Sprintf(`do revert true %s %s value big balance u %f<%f || %f>0.2 %f`,
			setting.Market, setting.Symbol, sm.availableU, usdLowLine, carryStatus.RateInAll, valueLimit))
	}
	return carryStatus, doRevert
}

// absentRevert: 当cm或sm中没有这个symbol时，是否设置成revert模式
func initStatus(account *model.Account, setting *model.Setting) (status *CarryStatus) {
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
		if status.market == model.Mexc { // mexc要求持仓不能超过1500张合约
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
	util.Log(util.LogLevelInfo, fmt.Sprintf(`init status set %s %s %s`, account.Key, setting.Market, setting.Symbol))
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
		util.LogLess(util.LogLevelError, fmt.Sprintf(`fail to get ticket %s %s`, setting.Market, setting.Symbol))
	}
	jumpOpen := 30.0
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
	status.TradeLineBuy = math.Max(standardScoreBuy*(0.5+jumpBuy*status.RateInAll), lowestScore)
	status.TradeLineSell = math.Max(standardScoreSell*(0.5+jumpSell*status.RateInAll), lowestScore)
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

// FailOrdersReconnect 用于处理ws下单，但没有返回orderId的情况，此时有可能有成交
func FailOrdersReconnect() {
	for {
		ts := time.Now().Unix()
		failOrders := make(map[int]map[string]*model.Order)
		model.AppEnvironment.ReqIdOrders.Range(func(requestId, value interface{}) bool {
			if value == nil {
				return true
			}
			orderTs := value.(*model.Order).OrderTime.Unix()
			if ts-orderTs > 300 && ts-orderTs < 86400 && value.(*model.Order).OrderId == value.(*model.Order).ClientOrdId {
				order := value.(*model.Order)
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
		select {
		case <-time.After(time.Minute * 3):
		}
	}
}

func ClearCross() {
	for {
		model.AppEnvironment.CrossEqualing = true
		doEqual := false
		if time.Now().Unix()-model.AppEnvironment.CrossEqualTime.Unix() > 3600 {
			doEqual = true
			model.AppEnvironment.CrossEqualTime = time.Now()
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf("begin to clearing cross get set %s %v do equal %v",
			model.FunctionCross, model.AppEnvironment.CrossEqualing, doEqual))
		compOrders.Clear()
		carryStatusMap.Clear()
		spotMarkets.Clear()
		contractMarkets.Clear()
		coinCrossing.Clear()
		carryCoinMap.Clear()
		model.AppEnvironment.ReqIdOrders.Clear()
		for {
			leftOrders := 0
			model.AppEnvironment.OrderIdOrders.Range(func(k, v interface{}) bool {
				if time.Now().Unix()-v.(*model.Order).OrderTime.Unix() < 60 {
					leftOrders++
				}
				util.Log(util.LogLevelInfo, fmt.Sprintf(`left orders in order id orders %d key %v %#v`, leftOrders, k, v))
				return true
			})
			if leftOrders == 0 {
				break
			}
			time.Sleep(time.Second * 3)
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
			util.Log(util.LogLevelError, `fail to close db conn`+err.Error())
		}
		msg := fmt.Sprintf(`comp compare cross %f %f`, compInU, crossInU)
		util.Log(util.LogLevelInfo, msg)
		if model.AppConfig.Handle == `1` {
			equalAccounts(doEqual)
		}
		model.AppEnvironment.CrossEqualing = false
		util.Log(util.LogLevelInfo, fmt.Sprintf("end to clearing cross get set %v", model.AppEnvironment.CrossEqualing))
		time.Sleep(time.Minute * 30)
	}
}

func equalAccounts(doEqual bool) {
	util.Log(util.LogLevelInfo, `...... enter clearing cross all`)
	waitEqual := make(map[int]bool)
	equalChannel := make(chan int, 1)
	api.InitCrossMarketInfos(model.AppEnvironment.Markets)
	api.PrepareSettings()
	//needWaitEqual := false // 是否需要进入等待环节
	for i := 0; i < api.GetCrossLen(); i++ {
		accounts := make(map[string]*model.Account)
		indexAccounts := model.GetAccounts(i)
		for _, market := range model.AppEnvironment.Markets {
			accounts[market] = indexAccounts[market]
		}
		waitEqual[i] = true
		go equalAccount(i, equalChannel, accounts, doEqual)
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
	util.Log(util.LogLevelInfo, `...... exit clearing cross all`)
}

func createCarryCoin(accounts map[string]*model.Account, coin string, settings []*model.Setting) (carryCoin *CarryCoin) {
	carryCoin = &CarryCoin{
		Coin:         coin,
		CurrentStep:  0,
		Holding:      0,
		MoneyPerStep: 0,
	}
	bidHolding := 0.0
	for _, setting := range settings {
		value, get := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, accounts[setting.Market].Key)
		if value == nil || !get {
			util.Log(util.LogLevelError, fmt.Sprintf(`store carry nil coin %s %s %s %s`, setting.Coin, setting.Market, setting.Symbol, accounts[setting.Market].Key))
			return nil
		}
		status := value.(*CarryStatus)
		if status.Holding > 0 {
			bidHolding += status.Holding * status.setting.GridAmount
		}
	}
	carryCoin.Holding = bidHolding
	return
}

func equalAccount(i int, equalChan chan int, accounts map[string]*model.Account, doEqual bool) {
	util.Log(util.LogLevelInfo, fmt.Sprintf(`begin to clearing cross %d `, i))
	if accounts[model.BitgetPerp] != nil {
		liquidateSmallContracts(accounts[model.BitgetPerp])
	}
	for market, account := range accounts {
		if account.Index != i {
			continue
		}
		api.CancelAll(account.Key, account.Secret, market)
	}
	value := api.GetCoinSettings(model.FunctionCross)
	if value != nil {
		value.Range(func(coin, settings interface{}) bool {
			equalStatuses := make([]*CarryStatus, len(settings.([]*model.Setting)))
			for j, setting := range settings.([]*model.Setting) {
				account := accounts[setting.Market]
				if setting == nil || len(coin.(string)) == 0 || account == nil {
					util.Log(util.LogLevelError, `can not equal`)
					continue
				}
				equalStatuses[j] = initStatus(account, setting)
				if equalStatuses[j] == nil {
					util.Log(util.LogLevelError, fmt.Sprintf(`store carry nil coin %s %s %s %s %d`,
						setting.Coin, setting.Market, setting.Symbol, account.Key, account.Index))
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
			coinCarry := createCarryCoin(accounts, coin.(string), settings.([]*model.Setting))
			if coinCarry != nil {
				util.StoreSyncMap(carryCoinMap, coinCarry, coin.(string), strconv.Itoa(i))
			}
			return true
		})
	}
	equalChan <- i
	util.Log(util.LogLevelInfo, fmt.Sprintf(`...... exit clearing cross %d`, i))
}

// getHolding
// 注意:
//
//		1.获取到的是经过gridAmount 和priceX调整过后的price和amount
//	 2.获得到的价格是经过funding rate加权后的价格，实际下单时要进行还原
func getHolding(statuses []*CarryStatus) (bids, asks model.Ticks, statusMap map[string]*CarryStatus,
	holding, price float64, holdStr string) {
	bids = model.Ticks{}
	asks = model.Ticks{}
	statusMap = make(map[string]*CarryStatus)
	for _, status := range statuses {
		if status == nil {
			util.Log(util.LogLevelError, `warning: fail to get one status`)
			return
		}
		holding += status.Holding * status.setting.GridAmount
		holdStr += fmt.Sprintf(`[%s %s %f]`, status.market, status.symbol, status.Holding)
		marketInfo := model.GetMarketInfo(status.market, status.symbol)
		getTick, tick := model.AppEnvironment.GetBidAsk(status.market, status.symbol)
		if !getTick || marketInfo == nil {
			continue
		}
		getFunding, _, _, handledRate := handledFRate(status.account, status.market, status.symbol, marketInfo.FundingRateInterval)
		if !getFunding {
			continue
		}
		priceBid := tick.Bids[0].Price * (1 + handledRate)
		priceAsk := tick.Asks[0].Price * (1 + handledRate)
		bids = append(bids, model.Tick{Ts: int64(tick.Ts), Market: tick.Bids[0].Market, Symbol: tick.Bids[0].Symbol,
			Amount: tick.Bids[0].Amount, Price: priceBid})
		asks = append(asks, model.Tick{Ts: int64(tick.Ts), Market: tick.Asks[0].Market, Symbol: tick.Asks[0].Symbol,
			Amount: tick.Asks[0].Amount, Price: priceAsk})
		statusMap[fmt.Sprintf(`%s_%s`, status.market, status.symbol)] = status
		if price == 0 {
			price = bids[0].Price / status.setting.PriceX
		}
		if !status.setting.Valid {
			util.Log(util.LogLevelError, fmt.Sprintf(`setting still invalid %s %s %s funding %f interval %d %#v`,
				status.market, status.symbol, status.setting.Coin, handledRate, marketInfo.FundingRateInterval, status.setting))
			status.setting.Valid = true
		}
	}
	return bids, asks, statusMap, holding, price, holdStr
}

// settings []*model.Setting, coinStatus map[string]map[string]map[string]*CarryStatus
func equalCoin(index int, coin string, statuses []*CarryStatus) (isEqual bool, holding float64, errMsg string) {
	bids, asks, statusMap, holdingValue, holdingPrice, holdStr := getHolding(statuses)
	util.Log(util.LogLevelInfo, fmt.Sprintf(`compare holding %s status num index %d %d %s`, coin, index, len(statuses), holdStr))
	if math.IsNaN(holdingValue) {
		util.Log(util.LogLevelError, `hold value is NaN `)
		for _, status := range statusMap {
			util.Log(util.LogLevelError, fmt.Sprintf(`hold value is NaN %#v %#v`, status.setting.GridAmount, status.setting.PriceX))
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
	var equalStatus *CarryStatus
	sort.Sort(sort.Reverse(bids))
	for i := 0; i < len(bids); i++ {
		getBid, bidAsk := model.AppEnvironment.GetBidAsk(bids[i].Market, bids[i].Symbol)
		if !getBid {
			continue
		}
		price := bidAsk.Bids[0].Price * (1 - compSlide)
		if holding > CompLineInMoney/holdingPrice {
			status := statusMap[fmt.Sprintf(`%s_%s`, bids[i].Market, bids[i].Symbol)]
			if status == nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`no status when holding: %f %s %s`, holding, bids[i].Market, bids[i].Symbol))
				continue
			}
			checkAmount := model.GetAmountInMarket(status.market, status.symbol, math.Abs(holding/status.setting.GridAmount), price, status.reduceOnlySell)
			if checkAmount <= 0 {
				errMsg += fmt.Sprintf(`check amount %s %s %f < 0`, status.market, status.symbol, checkAmount)
				continue
			}
			amount := 0.0
			if status.AvailableSell > holding/status.setting.GridAmount {
				equalStatus = status
				amount = math.Abs(holding) / status.setting.GridAmount
			} else {
				checkAmount = model.GetAmountInMarket(status.market, status.symbol, status.AvailableSell, price, status.reduceOnlySell)
				if checkAmount > 0 && status.AvailableSell*price > SmallInU {
					equalStatus = status
					//holding = status.AvailableSell
					amount = status.AvailableSell
				} else {
					util.Log(util.LogLevelError, fmt.Sprintf(`check 0 amount in %#v`, status))
					continue
				}
			}
			util.Log(util.LogLevelInfo, fmt.Sprintf(`need equal holding %s %f list %s tick ts %d equal status %#v`,
				coin, holding, holdStr, time.Now().UnixMilli()-bids[i].Ts, equalStatus))
			dealAmount := placeEqual(equalStatus, price, amount, model.OrderSideSell) * equalStatus.setting.GridAmount
			holding += dealAmount
			equalStatus.Holding += dealAmount
		}
	}
	sort.Sort(asks)
	for i := 0; i < len(asks); i++ {
		getAsk, bidAsk := model.AppEnvironment.GetBidAsk(asks[i].Market, asks[i].Symbol)
		if !getAsk {
			continue
		}
		price := bidAsk.Asks[0].Price * (1 + compSlide)
		if holding < -CompLineInMoney/holdingPrice {
			status := statusMap[fmt.Sprintf(`%s_%s`, asks[i].Market, asks[i].Symbol)]
			if status == nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`no status when holding: %f %s %s`, holding, asks[i].Market, asks[i].Symbol))
				continue
			}
			checkAmount := model.GetAmountInMarket(status.market, status.symbol, math.Abs(holding)/status.setting.GridAmount, price, status.reduceOnlyBuy)
			if checkAmount <= 0 {
				errMsg += fmt.Sprintf(`check amount %s %s %f < 0`, status.market, status.symbol, checkAmount)
				continue
			}
			amount := 0.0
			if math.IsNaN(status.AvailableBuy) || status.AvailableBuy > math.Abs(holding)/status.setting.GridAmount {
				equalStatus = status
				amount = math.Abs(holding) / status.setting.GridAmount
			} else if !math.IsNaN(status.AvailableBuy) {
				checkAmount = model.GetAmountInMarket(status.market, status.symbol, status.AvailableBuy, price, status.reduceOnlyBuy)
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
			util.Log(util.LogLevelInfo, fmt.Sprintf(`need equal holding %s %f list %s tick ts %d equal status %#v`,
				coin, holding, holdStr, time.Now().UnixMilli()-asks[i].Ts, equalStatus))
			dealAmount := placeEqual(equalStatus, price, amount, model.OrderSideBuy) * equalStatus.setting.GridAmount
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

func placeEqual(status *CarryStatus, price, amount float64, orderSide string) (dealAmount float64) {
	if status == nil {
		// 可能由于头寸太小，不满足所有市场的下单要求，而holdingU刚好大于10u，此时认为已平
		util.Log(util.LogLevelError, fmt.Sprintf(`can not get status to equal`))
		return 0
	}
	if status.market == model.Ftx {
		amount = math.Min(90000000, amount)
	}
	amount = math.Min(amount, compLimitInU/price)
	reduceOnly := status.reduceOnlySell
	if orderSide == model.OrderSideBuy {
		reduceOnly = status.reduceOnlyBuy
	}
	checkAmount := model.GetAmountInMarket(status.market, status.symbol, amount, price, reduceOnly)
	if checkAmount > 0 {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`do equal %f %f %#v`, price, amount, status))
		order := api.PlaceOrder(status.account.Key, status.account.Secret, orderSide, model.OrderTypeLimit,
			status.market, status.symbol, ``, model.FunctionCompAll, price, price, amount, false, nil)
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

func inFundingTime() (in bool) {
	if time.Now().Hour()%4 == 0 && time.Now().Minute() <= 2 {
		return true
	}
	return false
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
	ts1 := time.Now().UnixMilli()
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || setting.Valid == false || model.AppEnvironment.CrossEqualing ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) || settings == nil || len(settings) == 0 || inFundingTime() {
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
	case model.BinanceSpot:
		tickLimit = 2
	case model.BinancePerp:
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
		tickLimit += 2000
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
			status, getStatus := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, account.Key)
			statusRelate, getRelate := util.LoadSyncMap(carryStatusMap, settingRelate.Coin, settingRelate.Market, settingRelate.Symbol, accountRelate.Key)
			carryCoin, getCoin := util.LoadSyncMap(carryCoinMap, setting.Coin, strconv.Itoa(i))
			if status == nil || statusRelate == nil || status == statusRelate || carryCoin == nil || !getStatus || !getRelate || !getCoin {
				continue
			}
			delay, statusBuy, statusSell, amount, priceBuy, priceSell, tickBuy, tickSell :=
				calcAmount(i, setting.Coin, status.(*CarryStatus), statusRelate.(*CarryStatus), carryCoin.(*CarryCoin), tick, tickRelate)
			if delay {
				return
			}
			if amount > 0 {
				nowTs := time.Now().UnixMilli()
				placeCross(carryCoin.(*CarryCoin), statusBuy, statusSell, priceBuy, priceSell, amount)
				util.Log(util.LogLevelInfo, fmt.Sprintf(`time mark %s amt %e status %s %s tick %s %e = %e %e %d <- status %s %s tick %s %e = %e %e %d`,
					time.Now().String(), amount,
					statusBuy.symbol, statusBuy.market, tickBuy.Asks[0].Market, tickBuy.Asks[0].Price, priceBuy, tickBuy.Asks[0].Amount, nowTs-int64(tickBuy.Ts),
					statusSell.symbol, statusSell.market, tickSell.Bids[0].Market, tickSell.Bids[0].Price, priceSell, tickSell.Bids[0].Amount, nowTs-int64(tickSell.Ts)))
				time.Sleep(time.Millisecond * 100)
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

func placeCross(carryCoin *CarryCoin, statusBuy, statusSell *CarryStatus, priceBuy, priceSell, amount float64) {
	_, marketType, _, _ := model.GetFromStandard(statusBuy.market, statusBuy.symbol)
	if marketType == model.MarketTypeSpot {
		priceBuy = priceBuy * (1 + crossSpotBuySlide)
	} else {
		priceBuy = priceBuy * (1 + crossSlide)
	}
	priceSell = priceSell * (1 - crossSlide)
	score := (priceSell - priceBuy) / math.Max(priceBuy, priceSell)
	amountBuy := amount / statusBuy.setting.GridAmount
	amountSell := amount / statusSell.setting.GridAmount
	util.Log(util.LogLevelInfo, fmt.Sprintf(
		`place cross %s %s -> %s %s at %f %f amount %f %f %f score %f hold %f buy %f hold %f sell %f`,
		statusSell.market, statusSell.symbol, statusBuy.market, statusBuy.symbol, priceSell, priceBuy, amount, amountBuy,
		amountSell, score, statusBuy.Holding, statusBuy.TradeLineBuy, statusSell.Holding, statusSell.TradeLineSell))
	if statusBuy.market == model.OKEX && statusSell.market == model.OKEX {
		requestId := strconv.FormatInt(time.Now().UnixMicro(), 10)[3:]
		clientOrdIdBuy := requestId + `b`
		clientOrdIdSell := requestId + `s`
		orderBuy := &model.Order{OrderSide: model.OrderSideBuy, OrderType: model.OrderTypeLimit, Market: model.OKEX,
			Symbol: statusBuy.symbol, Price: priceBuy, Amount: amountBuy, RefreshType: model.FunctionCross, OrderTime: util.GetNow(),
			UnfilledQuantity: amountBuy, AccountIndex: statusBuy.account.Index, Status: model.CarryStatusWorking, Function: model.Open,
			OrderId: clientOrdIdBuy, ClientOrdId: clientOrdIdBuy, LineBuy: statusBuy.TradeLineBuy, LineSell: statusSell.TradeLineSell}
		orderSell := &model.Order{OrderSide: model.OrderSideSell, OrderType: model.OrderTypeLimit, Market: model.OKEX,
			Symbol: statusSell.symbol, Price: priceSell, Amount: amountSell, RefreshType: model.FunctionCross, OrderTime: util.GetNow(),
			UnfilledQuantity: amountSell, AccountIndex: statusSell.account.Index, Status: model.CarryStatusWorking, Function: model.Open,
			OrderId: clientOrdIdSell, ClientOrdId: clientOrdIdSell, LineBuy: statusSell.TradeLineBuy, LineSell: statusSell.TradeLineSell}
		if statusBuy.Holding*-1 >= amountBuy {
			orderBuy.Function = model.Close
		}
		if statusSell.Holding >= amountSell {
			orderSell.Function = model.Close
		}
		orderBuy.Coin = statusBuy.setting.Coin
		orderSell.Coin = statusSell.setting.Coin
		success, msg := api.PlacePairOKEX(statusBuy.account, requestId, statusBuy.symbol, statusSell.symbol, model.OrderTypeLimit,
			priceBuy, priceSell, amountBuy, amountSell)
		if success {
			model.AppEnvironment.ReqIdOrders.Store(requestId+model.OrderSideBuy, orderBuy)
			model.AppEnvironment.ReqIdOrders.Store(requestId+model.OrderSideSell, orderSell)
			model.AppDB.Save(orderBuy)
			model.AppDB.Save(orderSell)
		} else {
			orderBuy.Status, orderSell.Status = model.CarryStatusFail, model.CarryStatusFail
			orderBuy.ErrCode, orderSell.ErrCode = msg, msg
		}
	} else {
		go api.PlaceOrder(statusBuy.account.Key, statusBuy.account.Secret, model.OrderSideBuy, model.OrderTypeLimit, statusBuy.market,
			statusBuy.symbol, ``, model.FunctionCross, priceBuy, priceBuy, amountBuy, true, PostOrderCross)
		go api.PlaceOrder(statusSell.account.Key, statusSell.account.Secret, model.OrderSideSell, model.OrderTypeLimit, statusSell.market,
			statusSell.symbol, ``, model.FunctionCross, priceSell, priceSell, amountSell, true, PostOrderCross)
	}
	// 买入现货时要交手续费，故而实际到手少于下单量，校准以免未来买单时数量不足
	if marketType == model.MarketTypeSpot {
		amountBuy = amountBuy * 0.9992
	}
	if carryCoin != nil && model.AppConfig.Cross == crossGrid {
		carryCoin.AddTrade(statusBuy, statusSell, priceBuy, priceSell, amountBuy)
	}
	placeStatus(statusBuy, priceBuy, amountBuy)
	placeStatus(statusSell, priceSell, -1*amountSell)
	time.Sleep(time.Millisecond * 100)
}

func placeStatus(status *CarryStatus, price float64, amount float64) {
	valueSpot, _ := spotMarkets.Load(status.account.Key)
	valueContract, _ := contractMarkets.Load(status.account.Key)
	if status.isSpot && valueSpot != nil {
		balance := valueSpot.(*spotMarket).balances[status.symbol]
		if balance == nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`warning no balance %s %s %s`, status.account.Key, status.market, status.symbol))
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
		status.AvailableSell += amount
	}
	if !status.isSpot && valueContract != nil {
		position := valueContract.(*contractMarket).positions[status.symbol]
		originFreeAbs := 0.0
		if position == nil {
			position = &model.Position{Holding: amount, EntryPrice: price, Market: status.market, Currency: status.setting.Symbol}
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
	initStatus(account, status.setting)
}

func handleCross(account *model.Account, order *model.Order) {
	model.AppEnvironment.OrderIdOrders.Delete(order.OrderId)
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
	if order.Status == model.CarryStatusFail {
		compOrder(account, order, leftAmt)
	} else if leftAmt > marketInfo.SizeMin && leftAmt*order.Price > marketInfo.MoneyMin && order.Status != model.CarryStatusSuccess && order.HaveId() {
		time.Sleep(time.Minute)
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
			util.Log(util.LogLevelInfo, fmt.Sprintf(`post handle done %#v`, order))
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
					orderComp := api.PlaceOrder(account.Key, account.Secret, order.OrderSide, model.OrderTypeLimit, order.Market, order.Symbol, ``,
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
		status, _ := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, account.Key)
		if status != nil {
			status.(*CarryStatus).TradeLineSell = 1
			status.(*CarryStatus).TradeLineBuy = 1
			status.(*CarryStatus).LimitSell = 0
			status.(*CarryStatus).LimitBuy = 0
			util.Log(util.LogLevelError, fmt.Sprintf(`set trade line 1 fail order %s %s %s %s`,
				account.Key, order.OrderId, order.ErrCode, order.OrderTime.Format(time.DateTime)))
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
		comp := api.PlaceOrder(account.Key, account.Secret, order.OrderSide, model.OrderTypeLimit, order.Market, order.Symbol,
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
func FormatCrossPair(statusBuy, statusSell *CarryStatus, bidAmount, askAmount, amountLimit, price float64) (formattedAmount float64) {
	v, _ := util.LoadSyncMap(model.MarketInfos, statusBuy.setting.Market, statusBuy.setting.Symbol)
	var marketInfoBuy, marketInfoSell *model.MarketInfo
	if v != nil {
		marketInfoBuy = v.(*model.MarketInfo)
	}
	v, _ = util.LoadSyncMap(model.MarketInfos, statusSell.setting.Market, statusSell.setting.Symbol)
	if v != nil {
		marketInfoSell = v.(*model.MarketInfo)
	}
	if marketInfoBuy == nil || marketInfoSell == nil {
		util.Log(util.LogLevelInfo, fmt.Sprintf(
			`format %s %s %s %s %#v %#v`, statusBuy.market, statusSell.market, statusBuy.symbol, statusSell.symbol, marketInfoBuy, marketInfoSell))
		return
	}
	formattedAmount = math.Min(math.Min(statusBuy.LimitBuy, bidAmount)*statusBuy.setting.GridAmount,
		math.Min(statusSell.LimitSell, askAmount)*statusSell.setting.GridAmount)
	formattedAmount = math.Min(formattedAmount, statusSell.setting.GridAmount*openValueLimit/price)
	if amountLimit > 0 {
		formattedAmount = math.Min(formattedAmount, amountLimit)
	}
	minBuy := marketInfoBuy.SizeMin
	minSell := marketInfoSell.SizeMin
	if statusBuy.setting.Market == model.Bybit {
		minBuy = math.Max(5.5/price, minBuy)
	}
	if statusSell.setting.Market == model.Bybit {
		minSell = math.Max(5.5/price, minSell)
	}
	if marketInfoBuy.MoneyMin > 0 {
		minBuy = math.Max(minBuy, marketInfoBuy.MoneyMin/price)
	}
	if marketInfoSell.MoneyMin > 0 {
		minSell = math.Max(minSell, marketInfoSell.MoneyMin/price)
	}
	//amountBuy := model.GetAmountInMarket(marketBuy, symbolBuy, amount, price, false)
	//_, amountBuy = model.ParseRealAmount(marketBuy, symbolBuy, amountBuy)
	//amountSell := model.GetAmountInMarket(marketSell, symbolSell, amount, price, false)
	//_, amountSell = model.ParseRealAmount(marketSell, symbolSell, amountSell)
	//formattedAmount = math.Min(amountBuy, amountSell)
	if formattedAmount < math.Max(minBuy*statusBuy.setting.GridAmount, minSell*statusSell.setting.GridAmount) {
		return 0
	}
	return formattedAmount
}

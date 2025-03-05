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

// step(n) = step(n-1) + 0.0012 + 0.0001*(n-1)
var stepScores = []float64{-0.001, 0, 0.001, 0.0021, 0.0033, 0.0046, 0.0060, 0.0075, 0.0091, 0.0108, 0.0126, 0.0145, 0.0165, 0.0186,
	0.0208, 0.0231, 0.0255, 0.0280, 0.0306, 0.0333, 0.0361, 0.0390, 0.0420, 0.0451, 0.0483, 0.0516, 0.0550, 0.0585, 0.0621, 0.0658, 0.0696,
	0.0735, 0.0775, 0.0816, 0.0858, 0.0901, 0.0945, 0.0990, 0.1036, 0.1083, 0.1131, 0.1180, 0.1230, 0.1281, 0.1333, 0.1386, 0.1440, 0.1495,
	0.1551, 0.1608, 0.1666, 0.1725, 0.1785, 0.1846, 0.1908, 0.1971, 0.2035, 0.2100, 0.2166, 0.2233, 0.2301, 0.2370, 0.2440, 0.2511, 0.2583,
	0.2656, 0.2730, 0.2805, 0.2881, 0.2958, 0.3036}

const GridGap = 4

const swapScore = 0.002 // 换仓要求的利润金额
const crossGrid = `grid`

// CalcGridLine
// step(n) = step(n-1) + base + 0.0001*(n-1)
// base: 0.0012
// before -0.0011，0，0.0012
func CalcGridLine(base float64) {
	p := make([]float64, 5555555)
	for n := 1; n <= 250000; n++ {
		if n == 1 {
			p[n] = base
		} else {
			p[n] = p[n-1] + base + 0.0001*float64(n-1)
			fmt.Print(fmt.Sprintf(`%.4f,`, p[n]))
		}
		if p[n] > 0.3 {
			break
		}
	}
}

var ProcessADL = func(accountKey, market, symbol, adlSide string, amount float64) {
	util.Log(util.LogLevelError, fmt.Sprintf(`process ADL %s %s %s %f`, market, symbol, adlSide, amount))
	triggerAccount := model.AppConfig.GetAccountFromKeyIndex(market, accountKey, -1)
	if triggerAccount == nil {
		return
	}
	accounts := model.GetAccounts(triggerAccount.Index)
	adlSetting := api.GetSetting(model.FunctionCross, market, symbol)
	if adlSetting == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to process ADL nil adlSetting %s %s`))
		return
	}
	// 由于symbol中取出来的coin不一定等于setting中的coin，所以先拿到setting再通过setting的coin获取setting数组
	coinSettings := api.GetCoinSettings(model.FunctionCross)
	if coinSettings == nil {
		return
	}
	settings, _ := coinSettings.Load(adlSetting.Coin)
	statuses := make([]*model.CarryStatus, 0)
	for _, setting := range settings.([]*model.Setting) {
		if setting == nil {
			continue
		}
		item, _ := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, accounts[setting.Market].Key)
		if item == nil {
			continue
		}
		status := item.(*model.CarryStatus)
		if status.Market == adlSetting.Market && status.Symbol == adlSetting.Symbol {
			if adlSide == model.OrderSideBuy {
				status.Holding += amount
				status.LimitSell, status.AvailableSell = 0.0, 0.0
			} else if adlSide == model.OrderSideSell {
				status.Holding -= amount
				status.LimitBuy, status.AvailableBuy = 0.0, 0.0
			}
			util.StoreSyncMap(&model.AppEnvironment.ADLSymbol, true, status.Market, status.Symbol, status.Account.Key)
		}
		statuses = append(statuses, status)
	}
	equalCoin(triggerAccount.Index, adlSetting.Coin, statuses)
}

var ProcessCrossPositions = func(market, accountKey string, positions []*model.Position) {
	value := api.GetCoinSettings(model.FunctionCross)
	if value == nil {
		return
	}
	triggerAccount := model.AppConfig.GetAccountFromKeyIndex(market, accountKey, -1)
	accounts := model.GetAccounts(triggerAccount.Index)
	for _, position := range positions {
		posSetting := api.GetSetting(model.FunctionCross, market, position.Currency)
		if posSetting == nil {
			continue
		}
		// 针对1000倍币等种进行coin转换
		settings, _ := value.Load(posSetting.Coin)
		if settings == nil {
			continue
		}
		holding := 0.0
		price := 0.0
		for _, setting := range settings.([]*model.Setting) {
			account := accounts[setting.Market]
			if account == nil {
				continue
			}
			item, _ := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, account.Key)
			if item == nil {
				continue
			}
			status := item.(*model.CarryStatus)
			if status.Market == position.Market && status.Symbol == position.Currency {
				status.Holding = position.Holding
				util.LogLess(util.LogLevelInfo, fmt.Sprintf(`update position holding %d %s %s %f to %f %#v`,
					account.Index, status.Market, status.Symbol, status.Holding, position.Holding, position))
			}
			holding += status.Holding * setting.GridAmount
			if price == 0 {
				_, price = api.GetPriceForce(setting.Market, setting.Symbol, false)
				price = price / setting.PriceX
			}
		}
		if math.Abs(price*holding) >= openValueLimit*5 {
			for _, setting := range settings.([]*model.Setting) {
				account := accounts[setting.Market]
				if account == nil {
					continue
				}
				util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, setting.Coin, setting.Market, setting.Symbol, account.Key, model.OrderSideSell)
				util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, setting.Coin, setting.Market, setting.Symbol, account.Key, model.OrderSideBuy)
				util.Log(util.LogLevelError, fmt.Sprintf(`pause trade when update position %s %s %s %f setting %s %s holding %e value %e`,
					market, accountKey, position.Currency, position.Holding, setting.Market, setting.Symbol, holding, math.Abs(holding*price)))
			}
		}
	}
}

var ProcessCrossBalances = func(market, accountKey string, balances []*model.Balance) {
	value := api.GetCoinSettings(model.FunctionCross)
	if value == nil {
		return
	}
	triggerAccount := model.AppConfig.GetAccountFromKeyIndex(market, accountKey, -1)
	accounts := model.GetAccounts(triggerAccount.Index)
	for _, balance := range balances {
		symbol := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
		balSetting := api.GetSetting(model.FunctionCross, market, symbol)
		if balSetting == nil {
			continue
		}
		// 针对1000倍币等种进行coin转换
		settings, _ := value.Load(balSetting.Coin)
		if settings == nil {
			continue
		}
		holding := 0.0
		price := 0.0
		for _, setting := range settings.([]*model.Setting) {
			account := accounts[setting.Market]
			if account == nil {
				continue
			}
			item, _ := util.LoadSyncMap(carryStatusMap, setting.Coin, setting.Market, setting.Symbol, account.Key)
			if item == nil {
				continue
			}
			status := item.(*model.CarryStatus)
			if status.Market == balance.Market && status.Symbol == symbol {
				status.Holding = balance.Amount
				if account.Index == 0 && account.Market != model.OKEX {
					util.LogLess(util.LogLevelInfo, fmt.Sprintf(`update limit sell %d %s %s %f to %f %#v`,
						account.Index, status.Market, status.Symbol, status.LimitSell, balance.AvailableWithBorrow-balance.FrozenAmount, balance))
				}
				status.LimitSell = math.Max(balance.Amount-balance.FrozenAmount, balance.AvailableWithBorrow)
				status.AvailableSell = math.Max(balance.Amount-balance.FrozenAmount, balance.AvailableWithBorrow)
			}
			holding += status.Holding * setting.GridAmount
			if price == 0 {
				_, price = api.GetPriceForce(setting.Market, setting.Symbol, false)
				price = price / setting.PriceX
			}
		}
		if math.Abs(price*holding) >= openValueLimit*5 {
			for _, setting := range settings.([]*model.Setting) {
				account := accounts[setting.Market]
				if account == nil {
					continue
				}
				util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, setting.Coin, setting.Market, setting.Symbol, account.Key, model.OrderSideSell)
				util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, setting.Coin, setting.Market, setting.Symbol, account.Key, model.OrderSideBuy)
				util.Log(util.LogLevelError, fmt.Sprintf(`pause trade when update balance %s %s %s %f setting %s %s holding %e value %e`,
					market, accountKey, balance.Coin, balance.Amount, setting.Market, setting.Symbol, holding, math.Abs(holding*price)))
			}
		}
	}
}

// ProcessCollateral accountType 为”代表统一账户，为marketTypePerp or marketTypeSpot代表只是期货或现货
var ProcessCollateral = func(accountKey, accountType string, reduceOnly bool, collateral *model.Collateral) {
	valueContract, _ := contractMarkets.Load(accountKey)
	util.Log(util.LogLevelInfo, fmt.Sprintf(`update account value %s %s %#v`, accountKey, accountType, collateral))
	if valueContract != nil && accountType != model.MarketTypeSpot {
		cm := valueContract.(*contractMarket)
		if collateral != nil {
			cm.collateralsAvailable = collateral.Available
		}
		if collateral.AccountValueInU > 0 {
			cm.accountValueInU = collateral.AccountValueInU
		}
		if (reduceOnly || cm.collateralsAvailable < math.Min(MarginULowLimit, 0.1*collateral.AccountValueInU) &&
			cm.collateralsAvailable/cm.accountValueInU < 0.05) && cm.reduceOnly == false {
			cm.reduceOnly = true
			carryStatusMap.Range(func(k, v interface{}) bool {
				if v == nil {
					return true
				}
				key := k.(string)
				status := v.(*model.CarryStatus)
				if strings.Contains(key, fmt.Sprintf("*%s*", cm.market)) && strings.Contains(key, accountKey) {
					if status.Holding >= 0 {
						status.TradeLineBuy = 1
						status.LimitBuy = 0
					}
					if status.Holding <= 0 {
						status.TradeLineSell = 1
						status.LimitSell = 0
					}
					util.Log(util.LogLevelInfo, fmt.Sprintf("%s set trade line 1 holding %f %f %f",
						key, status.Holding, status.TradeLineBuy, status.TradeLineSell))
				}
				return true
			})
		}
	}
	valueSpot, _ := spotMarkets.Load(accountKey)
	if valueSpot != nil && accountType != model.MarketTypePerp {
		sm := valueSpot.(*spotMarket)
		if collateral.AccountValueInU > 0 {
			sm.accountValueInU = collateral.AccountValueInU
		}
	}
}

func generateMonitorMsg(index int, coin, scoreType, scoreTypeR string, score, scoreRelate float64, carryStatus, carryStatusRelate *model.CarryStatus,
	marketInfo, marketInfoRelate *model.MarketInfo, fundingRate, fundingRateRelate *model.FundingRate, valid bool) {
	// 为了同一对交易对冲不出现两次，对前后进行排序
	mark := fmt.Sprintf(`%s-%s`, carryStatus.Market, carryStatus.Symbol)
	markRelate := fmt.Sprintf(`%s-%s`, carryStatusRelate.Market, carryStatusRelate.Symbol)
	//green := false
	//if math.Abs(fundingRateRelate.Rate) > 0.001 || math.Abs(fundingRate.Rate) > 0.001 {
	//	green = true
	//}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	fundingStr, fundingStrRelate := ``, ``
	updateTime := fundingRate.UpdateTime.In(loc)
	fundingStr = fmt.Sprintf(`%d:%d %e %dH %d:%d`,
		util.GetNow().Hour(), util.GetNow().Minute(), 100*fundingRate.Rate, marketInfo.FundingRateInterval/3600000, updateTime.Hour(), updateTime.Minute())
	updateTime = fundingRateRelate.UpdateTime.In(loc)
	fundingStrRelate = fmt.Sprintf(`%d:%d %e %dH %d:%d`,
		util.GetNow().Hour(), util.GetNow().Minute(), 100*fundingRateRelate.Rate, marketInfoRelate.FundingRateInterval/3600000, updateTime.Hour(), updateTime.Minute())
	var infoValue []string
	if mark < markRelate {
		mark = fmt.Sprintf(`%s|%s`, mark, markRelate)
		infoValue = []string{coin, carryStatus.Market, carryStatus.Symbol, fundingStr,
			fmt.Sprintf(`%.3f`, 100*carryStatus.TradeLineBuy),
			fmt.Sprintf(`%.3f`, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatus.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatus.LimitSell),
			carryStatusRelate.Market, carryStatusRelate.Symbol, fundingStrRelate,
			fmt.Sprintf(`%.3f`, 100*carryStatusRelate.TradeLineBuy),
			fmt.Sprintf(`%.3f`, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitSell),
			fmt.Sprintf(`%.3f%s`, 100*scoreRelate, scoreTypeR),
			fmt.Sprintf(`%.3f%s`, 100*score, scoreType),
			fmt.Sprintf(`%v`, valid)}
	} else {
		mark = fmt.Sprintf(`%s|%s`, markRelate, mark)
		infoValue = []string{coin, carryStatusRelate.Market, carryStatusRelate.Symbol, fundingStrRelate,
			fmt.Sprintf(`%.3f`, 100*carryStatusRelate.TradeLineBuy),
			fmt.Sprintf(`%.3f`, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitSell),
			carryStatus.Market, carryStatus.Symbol, fundingStr,
			fmt.Sprintf(`%.3f`, 100*carryStatus.TradeLineBuy),
			fmt.Sprintf(`%.3f`, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatus.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatus.LimitSell),
			fmt.Sprintf(`%.3f%s`, 100*score, scoreType),
			fmt.Sprintf(`%.3f%s`, 100*scoreRelate, scoreTypeR),
			fmt.Sprintf(`%v`, valid)}
	}
	go model.SetMonitorInfo(strconv.Itoa(index), model.FunctionCross, mark, infoValue)
}

// checkTradeLine 返回limit=0表示无限制
func checkTradeLine(statusBuy, statusSell *model.CarryStatus, carryCoin *model.CarryCoin, priceBuy, priceSell, scoreOpen,
	scoreClose, scoreSwitch float64) (valid bool, limit, scoreUse float64, scoreType string) {
	settingBuy := statusBuy.Setting
	settingSell := statusSell.Setting
	pauseBuy, _ := util.LoadSyncMap(&model.AppEnvironment.PauseTrade, settingBuy.Coin, settingBuy.Market, settingBuy.Symbol, statusBuy.Account.Key, model.OrderSideBuy)
	pauseSell, _ := util.LoadSyncMap(&model.AppEnvironment.PauseTrade, settingSell.Coin, settingSell.Market, settingSell.Symbol, statusSell.Account.Key, model.OrderSideSell)
	if (pauseBuy != nil && pauseBuy.(bool)) || (pauseSell != nil && pauseSell.(bool)) {
		return false, 0, 1, ``
	}
	buyCrossStyle := model.AppConfig.GetCrossStyles()[statusBuy.Account.Index]
	sellCrossStyle := model.AppConfig.GetCrossStyles()[statusSell.Account.Index]
	if (buyCrossStyle == crossGrid || sellCrossStyle == crossGrid) && carryCoin == nil {
		return false, 0, 1, ``
	}
	crossLimit := openValueLimit / priceBuy * statusBuy.Setting.GridAmount
	if statusBuy.Holding*priceBuy >= -1*model.SmallHolding && statusSell.Holding*priceSell <= model.SmallHolding { // 开仓
		if buyCrossStyle == crossGrid {
			if carryCoin.CurrentStep < 0 || carryCoin.CurrentStep >= len(stepScores)-GridGap {
				return false, 0, scoreOpen, `开`
			}
			currentStep := carryCoin.CurrentStep
			leftCurStep := carryCoin.MoneyPerStep - carryCoin.MoneyCurStep
			if leftCurStep < model.SmallHolding && carryCoin.CurrentStep < len(stepScores)-GridGap {
				leftCurStep += carryCoin.MoneyPerStep
				currentStep++
			}
			coinLimit := math.Min(leftCurStep, openValueLimit) / priceBuy * statusBuy.Setting.GridAmount
			statusBuy.TradeLineBuy = stepScores[currentStep+GridGap]
			statusSell.TradeLineSell = stepScores[currentStep+GridGap]
			return scoreOpen > stepScores[currentStep+GridGap], coinLimit, scoreOpen, `开`
		} else {
			return scoreOpen > statusBuy.TradeLineBuy && scoreOpen > statusSell.TradeLineSell, crossLimit, scoreOpen, `开`
		}
	} else if statusBuy.Holding*priceBuy < -1*model.SmallHolding && statusSell.Holding*priceSell > model.SmallHolding { // 平仓
		if buyCrossStyle == crossGrid {
			limit = math.Min(math.Abs(statusBuy.Holding)*statusBuy.Setting.GridAmount, statusSell.Holding*statusSell.Setting.GridAmount)
			var closeLimit float64
			currentStep := carryCoin.CurrentStep
			if carryCoin.MoneyCurStep > model.SmallHolding {
				closeLimit = carryCoin.MoneyCurStep / priceBuy * statusBuy.Setting.GridAmount
			} else if carryCoin.CurrentStep >= 1 {
				currentStep--
				closeLimit = (carryCoin.MoneyCurStep + carryCoin.MoneyPerStep) / priceBuy * statusBuy.Setting.GridAmount
			}
			//else { // current step = 0 and money current step < small holding
			//	statusBuy.TradeLineBuy = 0.0
			//	statusSell.TradeLineSell = 0.0
			//	return scoreClose >= 0.0, limit, scoreClose, `平`
			//}
			if currentStep < 0 || currentStep > len(stepScores)-GridGap {
				return false, 0, scoreClose, `平`
			}
			statusBuy.TradeLineBuy = -1 * stepScores[currentStep]
			statusSell.TradeLineSell = -1 * stepScores[currentStep]
			return scoreClose > -1*stepScores[currentStep], math.Min(limit, closeLimit), scoreClose, `平`
		} else {
			if scoreClose > statusBuy.TradeLineBuy {
				return true, math.Min(math.Abs(statusBuy.Holding)*statusBuy.Setting.GridAmount, crossLimit), scoreClose, `平`
			}
			if scoreClose > statusSell.TradeLineSell {
				return true, math.Min(statusSell.Holding*statusSell.Setting.GridAmount, crossLimit), scoreClose, `平`
			}
			return false, 0, scoreClose, `平`
		}
	} else { // 换仓
		if statusBuy.Holding*priceBuy < -1*model.SmallHolding {
			limit = math.Abs(statusBuy.Holding) * statusBuy.Setting.GridAmount
		} else if statusSell.Holding*priceSell > model.SmallHolding {
			limit = statusSell.Holding * statusSell.Setting.GridAmount
		}
		if buyCrossStyle == crossGrid {
			statusBuy.TradeLineBuy = swapScore
			statusSell.TradeLineSell = swapScore
			return scoreSwitch > swapScore, math.Min(limit, crossLimit), scoreSwitch, `换`
		} else {
			marketDis := (statusBuy.TradeLineBuy + statusSell.TradeLineSell) / 2
			return scoreSwitch > marketDis, math.Min(limit, crossLimit), scoreSwitch, `换`
		}
	}
}

// calcAmount
// 返回amount是经过gridAmount乘数计算之后的数量，用以针对1000PEPE与PEPE这类币种的对冲交易.priceX与gridAmount相对应
// ChanceLimit开仓、换仓资金费率倍数
// ChanceLimitCombine平仓资金费率倍数
func calcAmount(index int, coin string, carryStatus, carryStatusRelate *model.CarryStatus, carryCoin *model.CarryCoin, tick, tickRelate *model.BidAsk) (
	delay bool, statusBuy, statusSell *model.CarryStatus, amount, priceBuy, priceSell float64) {
	var bidAmount, askAmount float64
	marketInfo := model.GetMarketInfo(carryStatus.Market, carryStatus.Symbol)
	marketInfoRelate := model.GetMarketInfo(carryStatusRelate.Market, carryStatusRelate.Symbol)
	if marketInfo == nil || marketInfoRelate == nil {
		return false, nil, nil, 0, 0, 0
	}
	gotFr, useRest, fundingRate, handledRate := handledFRate(carryStatus, marketInfo, tick.Bids[0].Price)
	gotFrRelate, useRestRelate, fundingRateRelate, handledRateRelate := handledFRate(carryStatusRelate, marketInfoRelate, tickRelate.Bids[0].Price)
	if !gotFr || !gotFrRelate || useRest || useRestRelate {
		return useRest || useRestRelate, nil, nil, 0, 0, 0
	}
	priceAskRelate := tickRelate.Asks[0].Price * (1 + handledRateRelate)
	priceBidRelate := tickRelate.Bids[0].Price * (1 + handledRateRelate)
	priceAsk := tick.Asks[0].Price * (1 + handledRate)
	priceBid := tick.Bids[0].Price * (1 + handledRate)
	priceX := carryStatus.Setting.PriceX
	priceXRelate := carryStatusRelate.Setting.PriceX
	// 如果（卖方加权资金费率-买方加权资金费率）小于0，则开仓和换仓交易用4倍加权资金费率,平仓交易用2倍
	// 如果（卖方加权资金费率-买方加权资金费率）大于等于0，则开仓换仓和平仓都用加权资金费率
	var score, scoreR, scoreOpen, scoreClose, scoreSwitch, scoreOpenR, scoreCloseR, scoreSwitchR float64
	scoreOpen = (priceBid/priceX - priceAskRelate/priceXRelate) / math.Max(priceBid/priceX, priceAskRelate/priceXRelate)
	score = scoreOpen
	scoreSwitch = scoreOpen
	scoreClose = scoreOpen
	scoreOpenR = (priceBidRelate/priceXRelate - priceAsk/priceX) / math.Max(priceAsk/priceX, priceBidRelate/priceXRelate)
	scoreR = scoreOpenR
	scoreSwitchR = scoreOpenR
	scoreCloseR = scoreOpenR
	if handledRateRelate > handledRate { // R为买方＞0
		priceBid = tick.Bids[0].Price * (1 + carryStatus.Setting.AmountRate*handledRate)
		priceAskRelate = tickRelate.Asks[0].Price * (1 + carryStatus.Setting.AmountRate*handledRateRelate)
		scoreOpen = (priceBid/priceX - priceAskRelate/priceXRelate) / math.Max(priceBid/priceX, priceAskRelate/priceXRelate)
		scoreSwitch = scoreOpen
		priceBid = tick.Bids[0].Price * (1 + carryStatus.Setting.AmountRateCombine*handledRate)
		priceAskRelate = tickRelate.Asks[0].Price * (1 + carryStatusRelate.Setting.AmountRateCombine*handledRateRelate)
		scoreClose = (priceBid/priceX - priceAskRelate/priceXRelate) / math.Max(priceBid/priceX, priceAskRelate/priceXRelate)
	} else if handledRateRelate < handledRate { // R为卖方<0
		priceBidRelate = tickRelate.Bids[0].Price * (1 + carryStatusRelate.Setting.AmountRate*handledRateRelate)
		priceAsk = tick.Asks[0].Price * (1 + carryStatusRelate.Setting.AmountRate*handledRate)
		scoreOpenR = (priceBidRelate/priceXRelate - priceAsk/priceX) / math.Max(priceAsk/priceX, priceBidRelate/priceXRelate)
		scoreSwitchR = scoreOpenR
		priceBidRelate = tickRelate.Bids[0].Price * (1 + carryStatusRelate.Setting.AmountRateCombine*handledRateRelate)
		priceAsk = tick.Asks[0].Price * (1 + carryStatus.Setting.AmountRateCombine*handledRate)
		scoreCloseR = (priceBidRelate/priceXRelate - priceAsk/priceX) / math.Max(priceAsk/priceX, priceBidRelate/priceXRelate)
	}
	var valid bool
	var amountLimit, scoreUse, scoreUseR float64
	var scoreType, scoreTypeR string
	valid, amountLimit, scoreUse, scoreType = checkTradeLine(carryStatusRelate, carryStatus, carryCoin, tickRelate.Asks[0].Price, tick.Bids[0].Price, scoreOpen, scoreClose, scoreSwitch)
	if valid {
		statusSell = carryStatus
		statusBuy = carryStatusRelate
		priceSell = tick.Bids[0].Price
		priceBuy = tickRelate.Asks[0].Price
		askAmount = tick.Bids[0].Amount
		bidAmount = tickRelate.Asks[0].Amount
		_, _, scoreUseR, scoreTypeR = checkTradeLine(carryStatus, carryStatusRelate, carryCoin, tick.Asks[0].Price, tickRelate.Bids[0].Price, scoreOpenR, scoreCloseR, scoreSwitchR)
	} else {
		valid, amountLimit, scoreUseR, scoreTypeR = checkTradeLine(carryStatus, carryStatusRelate, carryCoin, tick.Asks[0].Price, tickRelate.Bids[0].Price, scoreOpenR, scoreCloseR, scoreSwitchR)
		if valid {
			statusSell = carryStatusRelate
			statusBuy = carryStatus
			priceSell = tickRelate.Bids[0].Price
			priceBuy = tick.Asks[0].Price
			askAmount = tickRelate.Bids[0].Amount
			bidAmount = tick.Asks[0].Amount
		}
	}
	generateMonitorMsg(index, coin, scoreType, scoreTypeR, scoreUse, scoreUseR, carryStatus, carryStatusRelate, marketInfo, marketInfoRelate, fundingRate, fundingRateRelate, valid)
	if !valid {
		return false, nil, nil, 0, 0, 0
	}
	if breakMarkPrice(statusBuy.Account, statusBuy.Setting, priceBuy, model.OrderSideBuy) ||
		breakMarkPrice(statusSell.Account, statusSell.Setting, priceSell, model.OrderSideSell) {
		return false, nil, nil, 0, 0, 0
	}
	amount = FormatCrossPair(statusBuy, statusSell, bidAmount, askAmount, amountLimit, priceBuy, priceSell)
	if score > 0.1 || scoreR > 0.1 {
		if (carryStatus.Setting.Valid || carryStatusRelate.Setting.Valid) && (score > 0.4 || scoreR > 0.4) {
			util.LogLess(util.LogLevelError, fmt.Sprintf(`possible mismatch coin %s %s %s %s score %f %f`,
				carryStatus.Market, carryStatus.Symbol, carryStatusRelate.Market, carryStatusRelate.Symbol, score, scoreR))
			carryStatus.Setting.MarketRelated = fmt.Sprintf(`price distance too big %s %s %d‰ %d‰ %s`,
				carryStatusRelate.Market, carryStatusRelate.Symbol, int(1000*score), int(1000*scoreR), time.Now().Format("2006-01-02 15:04:05"))
			carryStatusRelate.Setting.MarketRelated = fmt.Sprintf(`price distance too big %s %s %d‰ %d‰ %s`,
				carryStatus.Market, carryStatus.Symbol, int(1000*scoreR), int(1000*score), time.Now().Format("2006-01-02 15:04:05"))
		}
		if statusBuy.Holding*priceBuy > -model.SmallHolding && statusSell.Holding*priceSell < model.SmallHolding {
			return false, nil, nil, 0, 0, 0
		}
	}
	return false, statusBuy, statusSell, amount, priceBuy, priceSell
}

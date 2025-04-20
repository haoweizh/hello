package cross

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// step(n) = step(n-1) + 0.0012 + 0.0001*(n-1)
var stepScores = []float64{-0.0014, 0, 0.0014, 0.0030, 0.0048, 0.0068, 0.0090, 0.0114, 0.0140, 0.0168, 0.0198, 0.0230, 0.0264, 0.03,
	0.0338, 0.0378, 0.0420, 0.0464, 0.0510, 0.0558, 0.0608, 0.0660, 0.0714, 0.0770, 0.0828, 0.0888, 0.0950, 0.1014, 0.1080, 0.1148, 0.1218,
	0.1290, 0.1364, 0.1440, 0.1518, 0.1598, 0.1680, 0.1764, 0.1850, 0.1938, 0.2028, 0.2120, 0.2214, 0.2310, 0.2408, 0.2508, 0.2610, 0.2714,
	0.2820, 0.2928, 0.3038}

const GridGap = 3

const swapScore = 0.003 // 换仓要求的利润金额
const crossGrid = `grid`

// CalcGridLine
// step(n) = step(n-1) + base + 0.00015*(n-1)
// base: 0.0012
// before -0.0011，0，0.0012
func CalcGridLine(base float64) {
	p := make([]float64, 5555555)
	for n := 1; n <= 250000; n++ {
		if n == 1 {
			p[n] = base
		} else {
			p[n] = p[n-1] + base + 0.0002*float64(n-1)
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
		util.Log(util.LogLevelError, fmt.Sprintf(`adl init status %#v`, status))
	}
	util.Log(util.LogLevelError, fmt.Sprintf(`adl init status to equal %s %#v %d`, adlSetting, len(statuses)))
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
				holding += position.Holding * setting.GridAmount
				util.LogLess(util.LogLevelInfo, fmt.Sprintf(`update position holding %d %s-%s %s-%s %f to %f %#v`,
					account.Index, status.Market, position.Market, status.Symbol, position.Currency, status.Holding, position.Holding, position))
				// 当前某个交易所持仓很小的时候很容易造成买卖都被当作平仓来处理.只用推送的可借币数据来限制可借币数量，
				//推送的持仓数据只用来和计算的数据比较，差额超过一万u时停止交易防止风险 status.Holding = position.Holding
			} else {
				holding += status.Holding * setting.GridAmount
			}
			if price == 0 {
				_, price = api.GetPriceForce(setting.Market, setting.Symbol, false)
				price = price / setting.PriceX
			}
		}
		if math.Abs(price*holding) >= 50000 {
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
				if status.Holding >= 0 {
					util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, setting.Coin, setting.Market, setting.Symbol, account.Key, model.OrderSideBuy)
				}
				if status.Holding <= 0 {
					util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, setting.Coin, setting.Market, setting.Symbol, account.Key, model.OrderSideSell)
				}
				util.Log(util.LogLevelError, fmt.Sprintf(`pause trade when update position %s %d %s %f setting %s %s holding %e value %e`,
					market, account.Index, position.Currency, position.Holding, setting.Market, setting.Symbol, holding, math.Abs(holding*price)))
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
				// 当前某个交易所持仓很小的时候很容易造成买卖都被当作平仓来处理.只用推送的可借币数据来限制可借币数量，
				//推送的持仓数据只用来和计算的数据比较，差额超过一万u时停止交易防止风险 status.Holding = balance.Amount
				//status.LimitSell = math.Max(balance.Amount, balance.AvailableWithBorrow) - balance.FrozenAmount
				//status.AvailableSell = math.Max(balance.Amount, balance.AvailableWithBorrow) - balance.FrozenAmount
				holding += balance.Amount * setting.GridAmount
				util.LogLess(util.LogLevelInfo, fmt.Sprintf(`update limit sell %d %s %s %f to %f %#v`,
					account.Index, status.Market, status.Symbol, status.LimitSell, balance.AvailableWithBorrow-balance.FrozenAmount, balance))
			} else {
				holding += status.Holding * setting.GridAmount
			}
			if price == 0 {
				_, price = api.GetPriceForce(setting.Market, setting.Symbol, false)
				price = price / setting.PriceX
			}
		}
		if math.Abs(price*holding) >= 50000 {
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
				if status.Holding >= 0 {
					util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, setting.Coin, setting.Market, setting.Symbol, account.Key, model.OrderSideBuy)
				}
				if status.Holding <= 0 {
					util.StoreSyncMap(&model.AppEnvironment.PauseTrade, true, setting.Coin, setting.Market, setting.Symbol, account.Key, model.OrderSideSell)
				}
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
				return false, 0, scoreOpen, model.Open
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
			return scoreOpen > stepScores[currentStep+GridGap], coinLimit, scoreOpen, model.Open
		} else {
			return scoreOpen > statusBuy.TradeLineBuy && scoreOpen > statusSell.TradeLineSell, crossLimit, scoreOpen, model.Open
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
				return false, 0, scoreClose, model.Close
			}
			closeScore := -1 * (stepScores[currentStep]/2 - 0.0007)
			statusBuy.TradeLineBuy, statusSell.TradeLineSell = closeScore, closeScore
			return scoreClose > closeScore, math.Min(limit, closeLimit), scoreClose, model.Close
		} else {
			if scoreClose > statusBuy.TradeLineBuy {
				return true, math.Min(math.Abs(statusBuy.Holding)*statusBuy.Setting.GridAmount, crossLimit), scoreClose, model.Close
			}
			if scoreClose > statusSell.TradeLineSell {
				return true, math.Min(statusSell.Holding*statusSell.Setting.GridAmount, crossLimit), scoreClose, model.Close
			}
			return false, 0, scoreClose, model.Close
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
			return scoreSwitch > swapScore, math.Min(limit, crossLimit), scoreSwitch, model.Switch
		} else {
			marketDis := (statusBuy.TradeLineBuy + statusSell.TradeLineSell) / 2
			return scoreSwitch > marketDis, math.Min(limit, crossLimit), scoreSwitch, model.Switch
		}
	}
}

// 如果（卖方加权资金费率-买方加权资金费率）小于0，则开仓和换仓交易用4倍加权资金费率,平仓交易用2倍
// 如果（卖方加权资金费率-买方加权资金费率）大于等于0，则开仓换仓和平仓都用加权资金费率
func calcScores(statusBuy, statusSell *model.CarryStatus, marketInfoBuy, marketInfoSell *model.MarketInfo, tickBuy, tickSell *model.BidAsk) (
	success bool, scoreBase, scoreOpen, scoreSwitch, scoreClose float64, rBuy, rSell *model.FundingRate, scoreMsg string) {
	gotFrBuy, useRestBuy, rateBuy, handledRateBuy := handledFRate(statusBuy, marketInfoBuy, tickBuy.Bids[0].Price, model.OrderSideBuy)
	gotFrSell, useRestSell, rateSell, handledRateSell := handledFRate(statusSell, marketInfoSell, tickSell.Bids[0].Price, model.OrderSideSell)
	if !gotFrBuy || !gotFrSell || useRestBuy || useRestSell {
		return false, 0, 0, 0, 0, nil, nil, ``
	}
	rateDelta := handledRateSell - handledRateBuy
	priceBuy := tickBuy.Asks[0].Price
	priceSell := tickSell.Bids[0].Price
	priceXBuy := statusBuy.Setting.PriceX
	priceXSell := statusSell.Setting.PriceX
	scoreBase = (priceSell/priceXSell - priceBuy/priceXBuy) / (priceBuy / priceXBuy)
	scoreOpen, scoreSwitch, scoreClose = scoreBase, scoreOpen, scoreOpen
	scoreMsg = fmt.Sprintf(`score check1 %s %s %f rate %f score %f holding %f %s %s %f rate %f holding %f buyAsk0 %f sellBid0 %f`,
		statusBuy.Market, statusBuy.Symbol, priceBuy, handledRateBuy, scoreOpen, statusBuy.Holding, statusSell.Market, statusSell.Symbol,
		priceSell, handledRateSell, statusSell.Holding, tickBuy.Asks[0].Price, tickSell.Bids[0].Price)
	if rateDelta < 0 { // R为卖方<0
		scoreOpen = scoreBase + statusSell.Setting.AmountRate*rateDelta
		scoreSwitch = scoreOpen
		scoreClose = scoreBase + statusSell.Setting.AmountRateCombine*rateDelta
		scoreMsg += fmt.Sprintf(` x change %f %.0f %.0f`, scoreBase-scoreOpen, statusSell.Setting.AmountRate, statusBuy.Setting.AmountRate)
	} else {
		scoreOpen = scoreBase + rateDelta/2
		scoreSwitch = scoreOpen + rateDelta
	}
	scoreMsg += fmt.Sprintf(`after handled open %f close %f buyAsk0 %f sellBid0 %f`, scoreOpen, scoreClose, tickBuy.Asks[0].Price, tickSell.Bids[0].Price)
	return true, scoreBase, scoreOpen, scoreSwitch, scoreClose, rateBuy, rateSell, scoreMsg
}

// calcAmount
// 返回amount是经过gridAmount乘数计算之后的数量，用以针对1000PEPE与PEPE这类币种的对冲交易.priceX与gridAmount相对应
// ChanceLimit开仓、换仓资金费率倍数
// ChanceLimitCombine平仓资金费率倍数
func calcAmount(index int, coin string, carryStatus, carryStatusRelate *model.CarryStatus, carryCoin *model.CarryCoin, tick, tickRelate *model.BidAsk) (
	delay bool, statusBuy, statusSell *model.CarryStatus, amount, priceBuy, priceSell float64, closeType, logMsg string) {
	marketInfo := model.GetMarketInfo(carryStatus.Market, carryStatus.Symbol)
	marketInfoR := model.GetMarketInfo(carryStatusRelate.Market, carryStatusRelate.Symbol)
	if marketInfo == nil || marketInfoR == nil {
		return false, nil, nil, 0, 0, 0, ``, ``
	}
	scoreSuccess, scoreBase, scoreOpen, scoreSwitch, scoreClose, rate, rateR, scoreMsg := calcScores(carryStatus, carryStatusRelate, marketInfo, marketInfoR, tick, tickRelate)
	scoreSuccessR, scoreBaseR, scoreOpenR, scoreSwitchR, scoreCloseR, _, _, scoreMsgR := calcScores(carryStatusRelate, carryStatus, marketInfoR, marketInfo, tickRelate, tick)
	if !scoreSuccess || !scoreSuccessR {
		return true, nil, nil, 0, 0, 0, ``, ``
	}
	var bidAmount, askAmount, tradeLimit float64
	valid, amountLimit, scoreUse, scoreType := checkTradeLine(carryStatus, carryStatusRelate, carryCoin, tick.Asks[0].Price, tickRelate.Bids[0].Price, scoreOpen, scoreClose, scoreSwitch)
	validR, amountLimitR, scoreUseR, scoreTypeR := checkTradeLine(carryStatusRelate, carryStatus, carryCoin, tickRelate.Asks[0].Price, tick.Bids[0].Price, scoreOpenR, scoreCloseR, scoreSwitchR)
	if valid {
		tradeLimit = amountLimit
		statusBuy = carryStatus
		statusSell = carryStatusRelate
		priceBuy = tick.Asks[0].Price
		priceSell = tickRelate.Bids[0].Price
		bidAmount = tick.Asks[0].Amount
		askAmount = tickRelate.Bids[0].Amount
		closeType = scoreType
		scoreMsg += fmt.Sprintf(`score valid %s %s at %f %f use score %f %s line %f carry coin %f %f %f %#v`,
			statusBuy.Market, statusBuy.Symbol, priceBuy, priceSell, scoreUse, scoreType, statusBuy.TradeLineBuy, tradeLimit, bidAmount, askAmount, carryCoin)
	}
	if validR {
		tradeLimit = amountLimitR
		statusBuy = carryStatusRelate
		statusSell = carryStatus
		priceBuy = tickRelate.Asks[0].Price
		priceSell = tick.Bids[0].Price
		bidAmount = tickRelate.Asks[0].Amount
		askAmount = tick.Bids[0].Amount
		closeType = scoreTypeR
		scoreMsgR += fmt.Sprintf(`score valid %s %s at %f %f use score %f %s line %f carry coin %f %f %f %#v`,
			statusBuy.Market, statusBuy.Symbol, priceBuy, priceSell, scoreUseR, scoreTypeR, statusBuy.TradeLineBuy, tradeLimit, bidAmount, askAmount, carryCoin)
	}
	generateMonitorMsg(index, coin, scoreType, scoreTypeR, scoreUse, scoreUseR, carryStatus, carryStatusRelate, marketInfo, marketInfoR, rate, rateR, valid || validR)
	if statusBuy == nil {
		return false, nil, nil, 0, 0, 0, ``, ``
	}
	if breakMarkPrice(statusBuy.Account, statusBuy.Setting, priceBuy, model.OrderSideBuy) ||
		breakMarkPrice(statusSell.Account, statusSell.Setting, priceSell, model.OrderSideSell) {
		return false, nil, nil, 0, 0, 0, ``, ``
	}
	if statusSell.Market == model.Gate {
		_, marketType, _, _ := model.GetFromStandard(statusSell.Market, statusSell.Symbol)
		if marketType == model.MarketTypeSpot {
			sm, _ := spotMarkets.Load(statusSell.Account.Key)
			if sm != nil {
				balance := sm.(*spotMarket).balances[statusSell.Symbol]
				if balance != nil && balance.Amount <= askAmount { //gate借币
					//oldAmt := askAmount
					randRate := 0.55 + 0.4*rand.Float64()
					askAmount = askAmount * randRate
					//util.Log(util.LogLevelInfo, fmt.Sprintf(`rand gate borrow sell amt %s %f to rand %f %f`,
					//	statusSell.Symbol, oldAmt, randRate, askAmount))
				}
			}
		}
	}
	amount = FormatCrossPair(statusBuy, statusSell, bidAmount, askAmount, tradeLimit, priceBuy, priceSell)
	if scoreBase > 0.1 || scoreBaseR > 0.1 {
		if (carryStatus.Setting.Valid || carryStatusRelate.Setting.Valid) && (scoreBase > 0.4 || scoreBaseR > 0.4) {
			util.LogLess(util.LogLevelInfo, fmt.Sprintf(`possible mismatch coin %s %s %s %s score %f %f price %s %f-%f %s %f-%f`,
				carryStatus.Market, carryStatus.Symbol, carryStatusRelate.Market, carryStatusRelate.Symbol, scoreBase, scoreBaseR,
				tick.Bids[0].Market, tick.Bids[0].Price, tick.Asks[0].Price, tickRelate.Bids[0].Market, tickRelate.Bids[0].Price, tickRelate.Asks[0].Price))
			carryStatus.Setting.MarketRelated = fmt.Sprintf(`price distance too big %s %s %d‰ %d‰ %s price %s %f-%f %s %f-%f`,
				carryStatusRelate.Market, carryStatusRelate.Symbol, int(1000*scoreBase), int(1000*scoreBaseR), time.Now().Format("2006-01-02 15:04:05"),
				tick.Bids[0].Price, tick.Bids[0].Price, tick.Asks[0].Price, tickRelate.Bids[0].Market, tickRelate.Bids[0].Price, tickRelate.Asks[0].Price)
			carryStatusRelate.Setting.MarketRelated = fmt.Sprintf(`price distance too big %s %s %d‰ %d‰ %s price %s %f-%f %s %f-%f`,
				carryStatus.Market, carryStatus.Symbol, int(1000*scoreBase), int(1000*scoreBaseR), time.Now().Format("2006-01-02 15:04:05"),
				tick.Bids[0].Market, tick.Bids[0].Price, tick.Asks[0].Price, tickRelate.Bids[0].Market, tickRelate.Bids[0].Price, tickRelate.Asks[0].Price)
		}
		if statusBuy.Holding*priceBuy > -model.SmallHolding && statusSell.Holding*priceSell < model.SmallHolding {
			return false, nil, nil, 0, 0, 0, ``, ``
		}
	}
	if valid {
		return false, statusBuy, statusSell, amount, priceBuy, priceSell, closeType, scoreMsg
	} else {
		return false, statusBuy, statusSell, amount, priceBuy, priceSell, closeType, scoreMsgR
	}
}

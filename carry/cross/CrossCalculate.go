package cross

import (
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"time"
)

// step(n) = step(n-1) + 0.0012 + 0.0001*(n-1) 共22档
var stepScores = []float64{0, 0.0012, 0.0025, 0.0039, 0.0054, 0.007, 0.0087, 0.0105, 0.0124, 0.0144, 0.0165, 0.0187, 0.021, 0.0234, 0.0259,
	0.0285, 0.0312, 0.034, 0.0369, 0.0399, 0.043, 0.0462, 0.0495, 0.0529, 0.0564, 0.06, 0.0637, 0.0675, 0.0714, 0.0754, 0.0795, 0.0837,
	0.088, 0.0924, 0.0969, 0.1015, 0.1062, 0.111, 0.1159, 0.1209, 0.126, 0.1312, 0.1365, 0.1419, 0.1474, 0.153, 0.1587, 0.1645, 0.1704, 0.1764,
	0.1825, 0.1887, 0.195, 0.2014, 0.2079, 0.2145, 0.2212, 0.2280, 0.2349, 0.2419, 0.249, 0.2562, 0.2635, 0.2709, 0.2784, 0.286, 0.2937, 0.3015}

const swapScore = 0.0015 // 换仓要求的利润金额
const crossGrid = `grid`

var ProcessCollateral = func(accountKey string, reduceOnly bool, collateral *model.Collateral) {
	value, _ := contractMarkets.Load(accountKey)
	if value == nil {
		return
	}
	cm := value.(*contractMarket)
	if collateral != nil {
		cm.collateralsAvailable = collateral.Available
	}
	if (reduceOnly || cm.collateralsAvailable < MarginULowLimit && cm.collateralsAvailable/cm.accountValueInU < 0.05) && cm.reduceOnly == false {
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
				}
				if status.Holding <= 0 {
					status.TradeLineBuy = 1
				}
				util.Log(util.LogLevelInfo, fmt.Sprintf("%s set trade line 1 holding %f %f %f",
					key, status.Holding, status.TradeLineBuy, status.TradeLineSell))
			}
			return true
		})
	}
}

func generateMonitorMsg(index int, coin string, score, scoreRelate float64, carryStatus, carryStatusRelate *model.CarryStatus,
	marketInfo, marketInfoRelate *model.MarketInfo, fundingRate, fundingRateRelate *model.FundingRate, valid bool) {
	// 为了同一对交易对冲不出现两次，对前后进行排序
	mark := fmt.Sprintf(`%s-%s`, carryStatus.Market, carryStatus.Symbol)
	markRelate := fmt.Sprintf(`%s-%s`, carryStatusRelate.Market, carryStatusRelate.Symbol)
	coinValue := coin
	if !carryStatus.IsSpot {
		coinValue += `永`
	}
	coinValueRelate := coin
	if !carryStatusRelate.IsSpot {
		coinValueRelate += `永`
	}
	//green := false
	//if math.Abs(fundingRateRelate.Rate) > 0.001 || math.Abs(fundingRate.Rate) > 0.001 {
	//	green = true
	//}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	fundingStr, fundingStrRelate := ``, ``
	if !carryStatus.IsSpot {
		updateTime := fundingRate.UpdateTime.In(loc)
		fundingStr = fmt.Sprintf(`%d:%d %e %dH %d:%d`,
			util.GetNow().Hour(), util.GetNow().Minute(), 100*fundingRate.Rate, marketInfo.FundingRateInterval/3600000, updateTime.Hour(), updateTime.Minute())
	} else {
		fundingStr = fmt.Sprintf(`%d:%d`, util.GetNow().Hour(), util.GetNow().Minute())
	}
	if !carryStatusRelate.IsSpot {
		updateTime := fundingRateRelate.UpdateTime.In(loc)
		fundingStrRelate = fmt.Sprintf(`%d:%d %e %dH %d:%d`,
			util.GetNow().Hour(), util.GetNow().Minute(), 100*fundingRateRelate.Rate, marketInfoRelate.FundingRateInterval/3600000, updateTime.Hour(), updateTime.Minute())
	} else {
		fundingStrRelate = fmt.Sprintf(`%d:%d`, util.GetNow().Hour(), util.GetNow().Minute())
	}
	var infoValue []string
	if mark < markRelate {
		mark = fmt.Sprintf(`%s|%s`, mark, markRelate)
		infoValue = []string{coin, carryStatus.Market, coinValue, fundingStr,
			fmt.Sprintf(`%.3f`, 100*carryStatus.TradeLineBuy),
			fmt.Sprintf(`%.3f`, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatus.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatus.LimitSell),
			carryStatusRelate.Market, coinValueRelate, fundingStrRelate,
			fmt.Sprintf(`%.3f`, 100*carryStatusRelate.TradeLineBuy),
			fmt.Sprintf(`%.3f`, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitSell),
			fmt.Sprintf(`%.3f`, 100*scoreRelate),
			fmt.Sprintf(`%.3f`, 100*score),
			fmt.Sprintf(`%v`, valid)}
	} else {
		mark = fmt.Sprintf(`%s|%s`, markRelate, mark)
		infoValue = []string{coin, carryStatusRelate.Market, coinValueRelate, fundingStrRelate,
			fmt.Sprintf(`%.3f`, 100*carryStatusRelate.TradeLineBuy),
			fmt.Sprintf(`%.3f`, 100*carryStatusRelate.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatusRelate.LimitSell),
			carryStatus.Market, coinValue, fundingStr,
			fmt.Sprintf(`%.3f`, 100*carryStatus.TradeLineBuy),
			fmt.Sprintf(`%.3f`, 100*carryStatus.TradeLineSell),
			fmt.Sprintf(`%.0e`, carryStatus.LimitBuy),
			fmt.Sprintf(`%.0e`, carryStatus.LimitSell),
			fmt.Sprintf(`%.3f`, 100*score),
			fmt.Sprintf(`%.3f`, 100*scoreRelate),
			fmt.Sprintf(`%v`, valid)}
	}
	go model.SetMonitorInfo(strconv.Itoa(index), model.FunctionCross, mark, infoValue)
}

// checkTradeLine 返回limit=0表示无限制
func checkTradeLine(statusBuy, statusSell *model.CarryStatus, carryCoin *model.CarryCoin, priceBuy, priceSell, score, frBuy, frSell float64) (valid bool, limit float64) {
	if statusBuy.StopBuy || statusSell.StopSell {
		return false, 0
	}
	buyCrossStyle := model.AppConfig.GetCrossStyles()[statusBuy.Account.Index]
	sellCrossStyle := model.AppConfig.GetCrossStyles()[statusSell.Account.Index]
	if (buyCrossStyle == crossGrid || sellCrossStyle == crossGrid) && carryCoin == nil {
		return
	}
	crossLimit := openValueLimit / priceBuy * statusBuy.Setting.GridAmount
	if statusBuy.Holding*priceBuy >= -1*model.SmallHolding && statusSell.Holding*priceSell <= model.SmallHolding { // 开仓
		if frSell-frBuy < -0.001 {
			return false, 0
		}
		if buyCrossStyle == crossGrid {
			if carryCoin.CurrentStep < 0 || carryCoin.CurrentStep >= len(stepScores)-2 {
				return false, 0
			}
			currentStep := carryCoin.CurrentStep
			leftCurStep := carryCoin.MoneyPerStep - carryCoin.MoneyCurStep
			if leftCurStep < model.SmallHolding && carryCoin.CurrentStep < len(stepScores)-2 {
				leftCurStep += carryCoin.MoneyPerStep
				currentStep++
			}
			coinLimit := math.Min(leftCurStep, openValueLimit) / priceBuy * statusBuy.Setting.GridAmount
			statusBuy.TradeLineBuy = stepScores[currentStep+2]
			statusSell.TradeLineSell = stepScores[currentStep+2]
			return score > stepScores[currentStep+2], coinLimit
		} else {
			return score > statusBuy.TradeLineBuy && score > statusSell.TradeLineSell, crossLimit
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
			} else { // current step = 0 and money current step < small holding
				statusBuy.TradeLineBuy = 0.0
				statusSell.TradeLineSell = 0.0
				return score >= 0.0, limit
			}
			if currentStep < 0 || currentStep > len(stepScores)-2 {
				return false, 0
			}
			statusBuy.TradeLineBuy = -1 * stepScores[currentStep]
			statusSell.TradeLineSell = -1 * stepScores[currentStep]
			return score > -1*stepScores[currentStep], math.Min(limit, closeLimit)
		} else {
			if score > statusBuy.TradeLineBuy {
				return true, math.Min(math.Abs(statusBuy.Holding)*statusBuy.Setting.GridAmount, crossLimit)
			}
			if score > statusSell.TradeLineSell {
				return true, math.Min(statusSell.Holding*statusSell.Setting.GridAmount, crossLimit)
			}
			return false, 0
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
			return score > swapScore, math.Min(limit, crossLimit)
		} else {
			marketDis := (statusBuy.TradeLineBuy + statusSell.TradeLineSell) / 2
			return score > marketDis, math.Min(limit, crossLimit)
		}
	}
}

// calcAmount
// 返回amount是经过gridAmount乘数计算之后的数量，用以针对1000PEPE与PEPE这类币种的对冲交易.priceX与gridAmount相对应
func calcAmount(index int, coin string, carryStatus, carryStatusRelate *model.CarryStatus, carryCoin *model.CarryCoin, tick, tickRelate *model.BidAsk) (
	delay bool, statusBuy, statusSell *model.CarryStatus, amount, priceBuy, priceSell float64) {
	var bidAmount, askAmount float64
	marketInfo := model.GetMarketInfo(carryStatus.Market, carryStatus.Symbol)
	marketInfoRelate := model.GetMarketInfo(carryStatusRelate.Market, carryStatusRelate.Symbol)
	if marketInfo == nil || marketInfoRelate == nil {
		return false, nil, nil, 0, 0, 0
	}
	gotFr, useRest, fundingRate, handledRate := handledFRate(carryStatus.Account, carryStatus.Market, carryStatus.Symbol, marketInfo.FundingRateInterval)
	gotFrRelate, useRestRelate, fundingRateRelate, handledRateRelate := handledFRate(carryStatusRelate.Account, carryStatusRelate.Market, carryStatusRelate.Symbol, marketInfoRelate.FundingRateInterval)
	if !gotFr || !gotFrRelate || useRest || useRestRelate {
		return useRest || useRestRelate, nil, nil, 0, 0, 0
	}
	priceAskRelate := tickRelate.Asks[0].Price * (1 + handledRateRelate)
	priceBidRelate := tickRelate.Bids[0].Price * (1 + handledRateRelate)
	priceAsk := tick.Asks[0].Price * (1 + handledRate)
	priceBid := tick.Bids[0].Price * (1 + handledRate)
	priceX := carryStatus.Setting.PriceX
	priceXRelate := carryStatusRelate.Setting.PriceX
	score := (priceBid/priceX - priceAskRelate/priceXRelate) / math.Max(priceBid/priceX, priceAskRelate/priceXRelate)
	scoreRelate := (priceBidRelate/priceXRelate - priceAsk/priceX) / math.Max(priceAsk/priceX, priceBidRelate/priceXRelate)
	//mark := fmt.Sprintf(`%s_%s|%s_%s`, carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol)
	//if score > 0.01 && util.DoDebug {
	//	model.AppMetric.AddCarry(mark, score, 0)
	//}
	valid, amountLimit := checkTradeLine(carryStatusRelate, carryStatus, carryCoin, tickRelate.Asks[0].Price, tick.Bids[0].Price, score, handledRateRelate, handledRate)
	if valid {
		statusSell = carryStatus
		statusBuy = carryStatusRelate
		priceSell = tick.Bids[0].Price
		priceBuy = tickRelate.Asks[0].Price
		askAmount = tick.Bids[0].Amount
		bidAmount = tickRelate.Asks[0].Amount
	} else {
		valid, amountLimit = checkTradeLine(carryStatus, carryStatusRelate, carryCoin, tick.Asks[0].Price, tickRelate.Bids[0].Price, scoreRelate, handledRate, handledRateRelate)
		if valid {
			statusSell = carryStatusRelate
			statusBuy = carryStatus
			priceSell = tickRelate.Bids[0].Price
			priceBuy = tick.Asks[0].Price
			askAmount = tickRelate.Bids[0].Amount
			bidAmount = tick.Asks[0].Amount
		}
	}
	generateMonitorMsg(index, coin, score, scoreRelate, carryStatus, carryStatusRelate, marketInfo, marketInfoRelate, fundingRate, fundingRateRelate, valid)
	if statusBuy == nil {
		return false, nil, nil, 0, 0, 0
	}
	if breakMarkPrice(statusBuy.Account, statusBuy.Setting, priceBuy, model.OrderSideBuy) ||
		breakMarkPrice(statusSell.Account, statusSell.Setting, priceSell, model.OrderSideSell) {
		return false, nil, nil, 0, 0, 0
	}
	amount = FormatCrossPair(statusBuy, statusSell, bidAmount, askAmount, amountLimit, priceBuy, priceSell)
	if score > 0.1 || scoreRelate > 0.1 {
		if (carryStatus.Setting.Valid || carryStatusRelate.Setting.Valid) && (score > 0.4 || scoreRelate > 0.4) {
			util.LogLess(util.LogLevelError, fmt.Sprintf(`possible mismatch coin %s %s %s %s score %f %f`,
				carryStatus.Market, carryStatus.Symbol, carryStatusRelate.Market, carryStatusRelate.Symbol, score, scoreRelate))
			carryStatus.Setting.MarketRelated = fmt.Sprintf(`price distance too big %s %s %d‰ %d‰ %s`,
				carryStatusRelate.Market, carryStatusRelate.Symbol, int(1000*score), int(1000*scoreRelate), time.Now().Format("2006-01-02 15:04:05"))
			carryStatusRelate.Setting.MarketRelated = fmt.Sprintf(`price distance too big %s %s %d‰ %d‰ %s`,
				carryStatus.Market, carryStatus.Symbol, int(1000*scoreRelate), int(1000*score), time.Now().Format("2006-01-02 15:04:05"))
		}
		return false, nil, nil, 0, 0, 0
	}
	return false, statusBuy, statusSell, amount, priceBuy, priceSell
}

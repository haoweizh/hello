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

// step(n) = step(n-1) + 0.001 + 0.0001*(n-1) 共22档
var stepScores = []float64{0, 0.001, 0.0021, 0.0033, 0.0046, 0.006, 0.0075, 0.0091, 0.0108, 0.0126, 0.0145, 0.0165, 0.0186,
	0.0208, 0.0231, 0.0255, 0.028, 0.0306, 0.0333, 0.0361, 0.039, 0.042, 0.0451, 0.0483, 0.0516, 0.055, 0.0585, 0.0621, 0.0658,
	0.0696, 0.0735, 0.0775, 0.0816, 0.0858, 0.0901, 0.0945, 0.099, 0.1036, 0.1083, 0.1131, 0.118, 0.123, 0.1281, 0.1333, 0.1386, 0.144,
	0.1495, 0.1551, 0.1608, 0.1666, 0.1725, 0.1785, 0.1846, 0.1908, 0.1971, 0.2035}

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
		num := 0
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
				num++
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
func checkTradeLine(statusBuy, statusSell *model.CarryStatus, carryCoin *model.CarryCoin, priceBuy, priceSell, score float64) (valid bool, limit float64) {
	if (statusBuy.Account.CrossStyle == crossGrid || statusSell.Account.CrossStyle == crossGrid) && carryCoin == nil {
		return
	}
	crossLimit := openValueLimit / priceBuy * statusBuy.Setting.GridAmount
	if statusBuy.Holding*priceBuy >= -1*model.SmallHolding && statusSell.Holding*priceSell <= model.SmallHolding { // 开仓
		if statusBuy.Account.CrossStyle == crossGrid {
			if carryCoin.CurrentStep >= len(stepScores)-2 {
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
		if statusBuy.Account.CrossStyle == crossGrid {
			limit = math.Min(math.Abs(statusBuy.Holding)*statusBuy.Setting.GridAmount, statusSell.Holding*statusSell.Setting.GridAmount)
			var closeLimit float64
			currentStep := carryCoin.CurrentStep
			if carryCoin.MoneyCurStep > model.SmallHolding {
				closeLimit = carryCoin.MoneyCurStep / priceBuy * statusBuy.Setting.GridAmount
			} else if carryCoin.CurrentStep >= 1 {
				currentStep--
				closeLimit = (carryCoin.MoneyCurStep + carryCoin.MoneyPerStep) / priceBuy * statusBuy.Setting.GridAmount
			} else { // current step = 0 and money current step < small holding
				statusBuy.TradeLineBuy = 0.001
				statusSell.TradeLineSell = 0.001
				return score > 0.001, limit
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
		if statusBuy.Account.CrossStyle == crossGrid {
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
	timeMark := time.Now().UnixNano()
	valid, amountLimit := checkTradeLine(carryStatusRelate, carryStatus, carryCoin, tickRelate.Asks[0].Price, tick.Bids[0].Price, score)
	if valid {
		timeMark = time.Now().UnixNano() - timeMark
		util.Log(util.LogLevelInfo, fmt.Sprintf("time time nano %d", timeMark))
		statusSell = carryStatus
		statusBuy = carryStatusRelate
		priceSell = tick.Bids[0].Price
		priceBuy = tickRelate.Asks[0].Price
		askAmount = tick.Bids[0].Amount
		bidAmount = tickRelate.Asks[0].Amount
	} else {
		valid, amountLimit = checkTradeLine(carryStatus, carryStatusRelate, carryCoin, tick.Asks[0].Price, tickRelate.Bids[0].Price, scoreRelate)
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
	if checkScoreLimit(carryStatus.Market, carryStatus.Symbol, carryStatusRelate.Market, carryStatusRelate.Symbol, score, scoreRelate) {
		if carryStatus.Setting.Valid || carryStatusRelate.Setting.Valid {
			util.Log(util.LogLevelError, fmt.Sprintf(`possible mismatch coin %s %s %s %s score %f %f`,
				carryStatus.Market, carryStatus.Symbol, carryStatusRelate.Market, carryStatusRelate.Symbol, score, scoreRelate))
			carryStatus.Setting.Valid = false
			carryStatusRelate.Setting.Valid = false
			carryStatus.Setting.MarketRelated = fmt.Sprintf(`价差过大 %s %s %d‰ %d‰ %s`,
				carryStatusRelate.Market, carryStatusRelate.Symbol, int(1000*score), int(1000*scoreRelate), time.Now().Format("2006-01-02 15:04:05"))
			carryStatusRelate.Setting.MarketRelated = fmt.Sprintf(`价差过大 %s %s %d‰ %d‰ %s`,
				carryStatus.Market, carryStatus.Symbol, int(1000*scoreRelate), int(1000*score), time.Now().Format("2006-01-02 15:04:05"))
		}
		return false, nil, nil, 0, 0, 0
	}
	return false, statusBuy, statusSell, amount, priceBuy, priceSell
}

func checkScoreLimit(market, symbol, marketRelate, symbolRelate string, score, scoreRelate float64) (invalid bool) {
	if score > 0.3 || scoreRelate > 0.3 {
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
					util.Log(util.LogLevelError, fmt.Sprintf(`fail to send mail msg %s %s`, msg, err.Error()))
				}
			}()
		} else if score > 0.05 || scoreRelate > 0.05 {
			notifyTime.Store(checkKey, time.Now())
			notifyTime.Store(checkKeyRelate, time.Now())
		}
	}
	return
}

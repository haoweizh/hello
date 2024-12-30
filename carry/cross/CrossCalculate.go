package cross

import (
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"time"
)

func generateMonitorMsg(index int, coin string, score, scoreRelate float64, carryStatus, carryStatusRelate *CarryStatus, marketInfo, marketInfoRelate *model.MarketInfo, fundingRate, fundingRateRelate *model.FundingRate) {
	// 为了同一对交易对冲不出现两次，对前后进行排序
	mark := fmt.Sprintf(`%s-%s`, carryStatus.market, carryStatus.symbol)
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
	if math.Abs(fundingRateRelate.Rate) > 0.001 || math.Abs(fundingRate.Rate) > 0.001 {
		green = true
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	fundingStr, fundingStrRelate := ``, ``
	if !carryStatus.isSpot {
		updateTime := fundingRate.UpdateTime.In(loc)
		fundingStr = fmt.Sprintf(`%d:%d %e %dH %d:%d`,
			util.GetNow().Hour(), util.GetNow().Minute(), 100*fundingRate.Rate, marketInfo.FundingRateInterval/3600000, updateTime.Hour(), updateTime.Minute())
	} else {
		fundingStr = fmt.Sprintf(`%d:%d`, util.GetNow().Hour(), util.GetNow().Minute())
	}
	if !carryStatusRelate.isSpot {
		updateTime := fundingRateRelate.UpdateTime.In(loc)
		fundingStrRelate = fmt.Sprintf(`%d:%d %e %dH %d:%d`,
			util.GetNow().Hour(), util.GetNow().Minute(), 100*fundingRateRelate.Rate, marketInfoRelate.FundingRateInterval/3600000, updateTime.Hour(), updateTime.Minute())
	} else {
		fundingStrRelate = fmt.Sprintf(`%d:%d`, util.GetNow().Hour(), util.GetNow().Minute())
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
	go model.SetMonitorInfo(strconv.Itoa(index), model.FunctionCross, mark, infoValue)
}

func checkGridLine(statusBuy, statusSell *CarryStatus, score float64) {

}

func checkTradeLine(statusBuy, statusSell *CarryStatus, score float64) (valid bool, limit float64) {
	if statusBuy.Holding >= 0 && statusSell.Holding <= 0 {
		return score > statusBuy.TradeLineBuy && score > statusSell.TradeLineSell, limit
	} else if statusBuy.Holding < 0 && statusSell.Holding > 0 {
		if score > statusBuy.TradeLineBuy {
			return true, math.Min(limit, math.Abs(statusBuy.Holding))
		}
		if score > statusSell.TradeLineSell {
			return true, math.Min(limit, statusSell.Holding)
		}
		return false, 0
	} else {
		marketDis := (statusBuy.TradeLineBuy + statusSell.TradeLineSell) / 2
		if statusBuy.account.CarryClose && statusBuy.Holding < 0 {
			limit = math.Min(limit, math.Abs(statusBuy.Holding))
		}
		if statusSell.account.CarryClose && statusSell.Holding > 0 {
			limit = math.Min(limit, statusSell.Holding)
		}
		return score > marketDis, limit
	}
}

// calcAmount
// 返回amount是经过gridAmount乘数计算之后的数量，用以针对1000PEPE与PEPE这类币种的对冲交易.priceX与gridAmount相对应
func calcAmount(index int, coin string, carryStatus, carryStatusRelate *CarryStatus, tick, tickRelate *model.BidAsk) (
	delay bool, statusBuy, statusSell *CarryStatus, amount, priceBuy, priceSell float64, tickBuy, tickSell *model.BidAsk) {
	var bidAmount, askAmount float64
	marketInfo := model.GetMarketInfo(carryStatus.market, carryStatus.symbol)
	marketInfoRelate := model.GetMarketInfo(carryStatusRelate.market, carryStatusRelate.symbol)
	if marketInfo == nil || marketInfoRelate == nil {
		return false, nil, nil, 0, 0, 0, nil, nil
	}
	gotFr, useRest, fundingRate, handledRate := handledFRate(carryStatus.account, carryStatus.market, carryStatus.symbol, marketInfo.FundingRateInterval)
	gotFrRelate, useRestRelate, fundingRateRelate, handledRateRelate := handledFRate(carryStatusRelate.account, carryStatusRelate.market, carryStatusRelate.symbol, marketInfoRelate.FundingRateInterval)
	if !gotFr || !gotFrRelate || useRest || useRestRelate {
		return useRest || useRestRelate, nil, nil, 0, 0, 0, nil, nil
	}
	priceAskRelate := tickRelate.Asks[0].Price * (1 + handledRateRelate)
	priceBidRelate := tickRelate.Bids[0].Price * (1 + handledRateRelate)
	priceAsk := tick.Asks[0].Price * (1 + handledRate)
	priceBid := tick.Bids[0].Price * (1 + handledRate)
	priceX := carryStatus.setting.PriceX
	priceXRelate := carryStatusRelate.setting.PriceX
	score := (priceBid/priceX - priceAskRelate/priceXRelate) / math.Max(priceBid/priceX, priceAskRelate/priceXRelate)
	scoreRelate := (priceBidRelate/priceXRelate - priceAsk/priceX) / math.Max(priceAsk/priceX, priceBidRelate/priceXRelate)
	generateMonitorMsg(index, coin, score, scoreRelate, carryStatus, carryStatusRelate, marketInfo, marketInfoRelate, fundingRate, fundingRateRelate)
	mark := fmt.Sprintf(`%s_%s|%s_%s`, carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol)
	if score > 0.01 && util.DoDebug {
		model.AppMetric.AddCarry(mark, score, 0)
	}
	valid, _ := checkTradeLine(carryStatusRelate, carryStatus, score)
	if valid {
		statusSell = carryStatus
		statusBuy = carryStatusRelate
		tickSell = tick
		tickBuy = tickRelate
		priceSell = tick.Bids[0].Price
		priceBuy = tickRelate.Asks[0].Price
		askAmount = tick.Bids[0].Amount
		bidAmount = tickRelate.Asks[0].Amount
	} else {
		valid, _ = checkTradeLine(carryStatus, carryStatusRelate, scoreRelate)
		if valid {
			statusSell = carryStatusRelate
			statusBuy = carryStatus
			tickSell = tickRelate
			tickBuy = tick
			priceSell = tickRelate.Bids[0].Price
			priceBuy = tick.Asks[0].Price
			askAmount = tickRelate.Bids[0].Amount
			bidAmount = tick.Asks[0].Amount
		}
	}
	if statusBuy == nil {
		return false, nil, nil, 0, 0, 0, nil, nil
	}
	if breakMarkPrice(statusBuy.account, statusBuy.setting, priceBuy, model.OrderSideBuy) ||
		breakMarkPrice(statusSell.account, statusSell.setting, priceSell, model.OrderSideSell) {
		return false, nil, nil, 0, 0, 0, nil, nil
	}
	amount = FormatCrossPair(statusBuy, statusSell, bidAmount, askAmount, priceBuy)
	if checkScoreLimit(carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol, score, scoreRelate) {
		if carryStatus.setting.Valid || carryStatusRelate.setting.Valid {
			util.Log(util.LogLevelError, fmt.Sprintf(`possible mismatch coin %s %s %s %s score %f %f`,
				carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol, score, scoreRelate))
			carryStatus.setting.Valid = false
			carryStatusRelate.setting.Valid = false
			carryStatus.setting.MarketRelated = fmt.Sprintf(`价差过大 %s %s %d‰ %d‰ %s`,
				carryStatusRelate.market, carryStatusRelate.symbol, int(1000*score), int(1000*scoreRelate), time.Now().Format("2006-01-02 15:04:05"))
			carryStatusRelate.setting.MarketRelated = fmt.Sprintf(`价差过大 %s %s %d‰ %d‰ %s`,
				carryStatus.market, carryStatus.symbol, int(1000*scoreRelate), int(1000*score), time.Now().Format("2006-01-02 15:04:05"))
		}
		return false, nil, nil, 0, 0, 0, nil, nil
	}
	return false, statusBuy, statusSell, amount, priceBuy, priceSell, tickBuy, tickSell
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

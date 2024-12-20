package cross

import (
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"time"
)

// calcAmount
// 返回amount是经过gridAmount乘数计算之后的数量，用以针对1000PEPE与PEPE这类币种的对冲交易.priceX与gridAmount相对应
func calcAmount(index int, coin string, carryStatus, carryStatusRelate *CarryStatus, tick, tickRelate *model.BidAsk) (
	statusBuy, statusSell *CarryStatus, amount, priceBuy, priceSell float64, tickBuy, tickSell *model.BidAsk) {
	stopStatus, okStatus := carryStop.Load(carryStatus.account.Key)
	stopRelate, okRelate := carryStop.Load(carryStatusRelate.account.Key)
	if (okStatus && stopStatus.(bool)) || (okRelate && stopRelate.(bool)) {
		//util.Log(util.LogLevelError, fmt.Sprintf(`stop carry for 10 times unknown carry %s or %s %s`,
		//	carryStatus.account.Key, carryStatusRelate.account.Key, coin))
		return
	}
	var bidAmount, askAmount float64
	priceAskRelate := tickRelate.Asks[0].Price
	priceBidRelate := tickRelate.Bids[0].Price
	priceAsk := tick.Asks[0].Price
	priceBid := tick.Bids[0].Price
	priceX := carryStatus.setting.PriceX
	priceXRelate := carryStatusRelate.setting.PriceX
	score := (priceBid/priceX - priceAskRelate/priceXRelate) / math.Max(priceBid/priceX, priceAskRelate/priceXRelate)
	scoreRelate := (priceBidRelate/priceXRelate - priceAsk/priceX) / math.Max(priceAsk/priceX, priceBidRelate/priceXRelate)
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
		priceSell = priceBid
		priceBuy = priceAskRelate
		askAmount = tick.Bids[0].Amount
		bidAmount = tickRelate.Asks[0].Amount
	} else {
		valid, _ = checkTradeLine(carryStatus, carryStatusRelate, scoreRelate)
		if valid {
			statusSell = carryStatusRelate
			statusBuy = carryStatus
			tickSell = tickRelate
			tickBuy = tick
			priceSell = priceBidRelate
			priceBuy = priceAsk
			askAmount = tickRelate.Bids[0].Amount
			bidAmount = tick.Asks[0].Amount
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
	amount = FormatCrossPair(statusBuy, statusSell, bidAmount, askAmount, priceBuy)
	if checkScoreLimit(carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol, score, scoreRelate) {
		if carryStatus.setting.Valid || carryStatusRelate.setting.Valid {
			util.Log(util.LogLevelError, fmt.Sprintf(`possible mismatch coin %s %s %s %s score %f %f`,
				carryStatus.market, carryStatus.symbol, carryStatusRelate.market, carryStatusRelate.symbol, score, scoreRelate))
		}
		carryStatus.setting.Valid = false
		carryStatusRelate.setting.Valid = false
		carryStatus.setting.MarketRelated = fmt.Sprintf(`价差过大 %s %s %f %f`, carryStatusRelate.market, carryStatusRelate.symbol, score, scoreRelate)
		carryStatusRelate.setting.MarketRelated = fmt.Sprintf(`价差过大 %s %s %f %f`, carryStatus.market, carryStatus.symbol, scoreRelate, score)
		return nil, nil, 0, 0, 0, nil, nil
	}
	return statusBuy, statusSell, amount, priceBuy, priceSell, tickBuy, tickSell
}

func checkScoreLimit(market, symbol, marketRelate, symbolRelate string, score, scoreRelate float64) (invalid bool) {
	if (score > 0.3 || scoreRelate > 0.3) ||
		((score > 0.07 || scoreRelate > 0.07) && (market == model.Gate || marketRelate == model.Gate)) ||
		((score > 0.1 || scoreRelate > 0.1) && (!isValidSymbol(market, symbol) || !isValidSymbol(marketRelate, symbolRelate))) {
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

package regret

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strings"
	"sync"
	"time"
)

const NCalcLen = 50

var absentTurtles sync.Map // symbol_unix seconds bool

type TurtleData struct {
	highNear, lowNear, highFar, lowFar, n, m         float64
	Near, Far                                        int
	orderLong, orderLongRe, orderShort, orderShortRe *model.Order
	liquidated, useNear                              bool
	begin                                            time.Time
}

var slotLiquidated map[string]bool // market_orderSide_beginTimeString

func GetTurtleData(candles []*model.Candle, near, far, slot int, useNear bool) (turtleDataMap map[string]*TurtleData) {
	turtleDataMap = make(map[string]*TurtleData)
	for i := far; i < len(candles); i++ {
		turtleData := &TurtleData{liquidated: false, n: candles[i-1].N, m: candles[i-1].M, Near: near, Far: far, useNear: useNear, begin: candles[i].Begin}
		for j := 1; j <= far; j++ {
			if candles[i-j].PriceHigh > turtleData.highFar && j <= turtleData.Far {
				turtleData.highFar = candles[i-j].PriceHigh
			}
			if (turtleData.lowFar == 0 || turtleData.lowFar > candles[i-j].PriceLow) && j <= turtleData.Far {
				turtleData.lowFar = candles[i-j].PriceLow
			}
			if candles[i-j].PriceHigh > turtleData.highNear && j <= turtleData.Near {
				turtleData.highNear = candles[i-j].PriceHigh
			}
			if (turtleData.lowNear == 0 || turtleData.lowNear > candles[i-j].PriceLow) && j <= turtleData.Near {
				turtleData.lowNear = candles[i-j].PriceLow
			}
			turtleDataMap[getTurtleKey(candles[i], int64(slot))] = turtleData
		}
	}
	return turtleDataMap
}

func createTurtleOrder(setting *model.Setting, candle *model.Candle, orderSide string,
	price, amount, n float64, currentChances, allLimit int, posNum int64) (order *model.Order) {
	if allLimit > 0 && ((orderSide == model.OrderSideSell && currentChances <= -1*allLimit) ||
		(orderSide == model.OrderSideBuy && currentChances >= allLimit)) {
		return nil
	}
	if (orderSide == model.OrderSideSell && float64(setting.Chance) <= -1*setting.AmountLimit) || (orderSide == model.OrderSideBuy && float64(setting.Chance) >= setting.AmountLimit) {
		return nil
	}
	if amount == 0 {
		return nil
	}
	order = &model.Order{
		Amount:      amount,
		Fee:         n,
		Market:      setting.Market,
		OrderSide:   orderSide,
		Price:       price,
		RefreshType: model.FunctionSimulation,
		Status:      model.CarryStatusWorking,
		Symbol:      setting.Symbol,
		GridPos:     posNum,
		CreatedAt:   candle.Begin,
	}
	if orderSide == model.OrderSideBuy {
		order.DealPrice = price * (1 + setting.TradeCost)
		if setting.Chance < 0 {
			order.OrderType = model.OrderSideLiquidateShort
		}
	} else if orderSide == model.OrderSideSell {
		order.DealPrice = price * (1 - setting.TradeCost)
		if setting.Chance > 0 {
			order.OrderType = model.OrderSideLiquidateLong
		}
	}
	util.Info(fmt.Sprintf(`create turtle %s at %s %s %d`, orderSide, candle.Begin.String(), candle.Symbol, setting.Chance))
	return
}

// allLimit 小于0代表没有总仓位限制
// 在当前回归中，龟汤useNear false，海龟useNear true，龟汤数量为amount*m/n
func calcTurtleOrders(setting *model.Setting, turtleData *TurtleData, useM bool) (
	priceShort, priceLong, amountShort, amountLong float64, posNumShort, posNumLong int64) {
	priceShort = turtleData.lowFar
	priceLong = turtleData.highFar
	if setting.Chance == 0 && !turtleData.liquidated { // 开初始仓
		if !slotLiquidated[fmt.Sprintf(`%s_%s_%s_%s`, setting.Market, setting.Symbol, model.OrderSideBuy, turtleData.begin.String())] {
			amountLong = setting.GridAmount / turtleData.n
			if useM {
				amountLong = amountLong * turtleData.m / turtleData.n
			}
			posNumLong = 1
		} else {
			util.Info(fmt.Sprintf(`no new open buy as %s liquated`, turtleData.begin.String()))
		}
		if !slotLiquidated[fmt.Sprintf(`%s_%s_%s_%s`, setting.Market, setting.Symbol, model.OrderSideSell, turtleData.begin.String())] {
			amountShort = setting.GridAmount / turtleData.n
			if useM {
				amountShort = amountShort * turtleData.m / turtleData.n
			}
			posNumShort = 1
		} else {
			util.Info(fmt.Sprintf(`no new open sell as %s liquated`, turtleData.begin.String()))
		}
	} else if setting.Chance > 0 {
		priceLong = math.Max(turtleData.highFar, setting.PriceX+turtleData.n/2)
		if turtleData.useNear {
			priceShort = math.Max(setting.PriceX-2*turtleData.n, turtleData.lowNear)
			priceShort = math.Max(priceShort, turtleData.highFar-4*turtleData.n)
		} else {
			priceShort = turtleData.highFar - 2*turtleData.n
		}
		amountShort = setting.OpenShortMargin
		posNumShort = int64(math.Abs(float64(setting.Chance)))
		amountLong = setting.GridAmount / turtleData.n
		if useM {
			amountLong = amountLong * turtleData.m / turtleData.n
		}
		posNumLong = 1
	} else if setting.Chance < 0 {
		priceShort = math.Min(turtleData.lowFar, setting.PriceX-turtleData.n/2)
		if turtleData.useNear {
			priceLong = math.Min(setting.PriceX+2*turtleData.n, turtleData.highNear)
			priceLong = math.Min(priceLong, turtleData.lowFar+4*turtleData.n)
		} else {
			priceLong = turtleData.lowFar + 2*turtleData.n
		}
		amountLong = setting.OpenShortMargin
		posNumLong = int64(math.Abs(float64(setting.Chance)))
		posNumShort = 1
		amountShort = setting.GridAmount / turtleData.n
		if useM {
			amountShort = amountShort * turtleData.m / turtleData.n
		}
	}
	return
}

func getCurrentChances(settings map[string]*model.Setting) (chances int64) {
	for _, setting := range settings {
		chances += setting.Chance
	}
	return chances
}

func handlePrice(turtleData *TurtleData, candle *model.Candle, settings map[string]*model.Setting, allLimit int,
	sign string, useM bool) {
	var setting *model.Setting
	if settings != nil && candle != nil && settings[candle.Symbol] != nil {
		setting = settings[candle.Symbol]
	} else {
		util.Info(`fail to process handle price`)
		return
	}
	if !turtleData.useNear && candle.PriceHigh > turtleData.highFar {
		turtleData.highFar = candle.PriceHigh
		if turtleData.orderShort != nil && setting.Chance > 0 {
			turtleData.orderShort = nil
			turtleData.orderLong = nil
		}
	}
	if !turtleData.useNear && candle.PriceLow < turtleData.lowFar {
		turtleData.lowFar = candle.PriceLow
		if turtleData.orderLong != nil && setting.Chance < 0 {
			turtleData.orderShort = nil
			turtleData.orderLong = nil
		}
	}
	currentChances := getCurrentChances(settings)
	if turtleData.orderLong == nil && turtleData.orderShort == nil {
		priceShort, priceLong, amountShort, amountLong, posNumShort, posNumLong := calcTurtleOrders(setting, turtleData, useM)
		turtleData.orderShort = createTurtleOrder(setting, candle, model.OrderSideSell,
			priceShort, amountShort, turtleData.n, int(currentChances), allLimit, posNumShort)
		turtleData.orderLong = createTurtleOrder(setting, candle, model.OrderSideBuy,
			priceLong, amountLong, turtleData.n, int(currentChances), allLimit, posNumLong)
	}
	if turtleData.orderLong != nil && candle.PriceHigh >= turtleData.orderLong.Price {
		if setting.Chance >= 0 {
			setting.Chance += 1
			setting.OpenShortMargin += turtleData.orderLong.Amount
		} else {
			setting.Chance = 0
			setting.OpenShortMargin = 0
			turtleData.liquidated = true
			slotLiquidated[fmt.Sprintf(`%s_%s_%s_%s`, candle.Market, candle.Symbol, model.OrderSideBuy, turtleData.begin.String())] = true
			util.Info(fmt.Sprintf(`no new open after liquated buy %s %s`, candle.Symbol, turtleData.begin.String()))
		}
		setting.PriceX = turtleData.orderLong.Price
		turtleData.orderLong.Status = model.CarryStatusSuccess
		turtleData.orderLong.OrderTime = candle.Begin
		turtleData.orderLong.OrderId = fmt.Sprintf(`%s%s%s%s%d`,
			setting.Market, setting.Symbol, sign, turtleData.orderLong.OrderSide, candle.Begin.Unix())
		turtleData.orderLong.UnfilledQuantity = float64(currentChances)
		turtleData.orderLong.Function = sign
		util.Info(fmt.Sprintf(`deal long chance %d 总仓 %d save order %s at %s`,
			setting.Chance, currentChances, turtleData.orderLong.OrderId, turtleData.orderLong.OrderTime.String()))
		model.AppDB.Save(turtleData.orderLong)
		turtleData.orderLong = nil
		turtleData.orderShort = nil
	}
	if turtleData.orderShort != nil && candle.PriceLow <= turtleData.orderShort.Price {
		if setting.Chance <= 0 {
			setting.Chance -= 1
			setting.OpenShortMargin += turtleData.orderShort.Amount
		} else {
			setting.Chance = 0
			setting.OpenShortMargin = 0
			turtleData.liquidated = true
			slotLiquidated[fmt.Sprintf(`%s_%s_%s_%s`, candle.Market, candle.Symbol, model.OrderSideSell, turtleData.begin.String())] = true
			util.Info(fmt.Sprintf(`no new open after liquated sell %s %s`, candle.Symbol, turtleData.begin.String()))
		}
		setting.PriceX = turtleData.orderShort.Price
		turtleData.orderShort.Status = model.CarryStatusSuccess
		turtleData.orderShort.OrderTime = candle.Begin
		turtleData.orderShort.OrderId = fmt.Sprintf(`%s%s%s%s%d`,
			setting.Market, setting.Symbol, sign, turtleData.orderShort.OrderSide, candle.Begin.Unix())
		turtleData.orderShort.UnfilledQuantity = float64(currentChances)
		turtleData.orderShort.Function = sign
		util.Info(fmt.Sprintf(`deal short chance %d 总仓 %d save order %s at %s`,
			setting.Chance, currentChances, turtleData.orderShort.OrderId, turtleData.orderShort.OrderTime.String()))
		model.AppDB.Save(turtleData.orderShort)
		turtleData.orderLong = nil
		turtleData.orderShort = nil
	}
}

func getTurtleKey(candle *model.Candle, slot int64) (key string) {
	seconds := candle.Begin.Unix() - (candle.Begin.Unix() % slot)
	if candle.Market == model.OKEX {
		seconds = candle.Begin.Unix() - (candle.Begin.Unix()+28800)%slot
	}
	return fmt.Sprintf(`%s_%s_%d`, candle.Market, candle.Symbol, seconds)
}

// ProcessCandles
// setting.AmountLimit 开仓数上限
// setting.GridAmount 标准一仓的数量
// allLimit 小于0时代表没有限制
// setting.OpenShortMargin 已开仓数量
func ProcessCandles(start, end time.Time, far, allLimit int, useNear, useM bool, market, sign string, settings map[string]*model.Setting) {
	if settings == nil || len(settings) == 0 {
		return
	}
	util.StoreSyncMap(&model.CarryInfo, nil, `GetCandle`)
	sortedCandles := api.GetMultiCandle(model.AppConfig.GetAccounts(market)[0], market, 60, start, end, settings, false)
	ago := int(math.Max(NCalcLen, float64(far)))
	turtleDataMap := make(map[string]*TurtleData)
	slotLiquidated = make(map[string]bool)
	if sortedCandles != nil && sortedCandles.Len() > 0 && sortedCandles[0] != nil && sortedCandles[len(sortedCandles)-1] != nil {
		util.Info(fmt.Sprintf(`get sorted candles from %s %s to %s %s`,
			sortedCandles[0].Begin.String(), sortedCandles[0].Symbol,
			sortedCandles[sortedCandles.Len()-1].Begin.String(), sortedCandles[sortedCandles.Len()-1].Symbol))
	}
	var slotSeconds int64
	for _, setting := range settings {
		slotSeconds = setting.Seconds
		duration, _ := time.ParseDuration(fmt.Sprintf(`-%ds`, int(setting.Seconds)*ago))
		temp := api.CombineCandles(model.AppConfig.GetAccounts(market)[0], market, setting.Symbol, int(setting.Seconds),
			start.Add(duration), end)
		util.Info(fmt.Sprintf(`get turtle candle %s %s %d setting chance %d`,
			market, setting.Symbol, len(temp), setting.Chance))
		calcLenN := 10
		calcLenV := 20
		api.CalcCandleN(&model.SortedCandle{Value: temp}, calcLenN, calcLenV)
		tempTurtle := GetTurtleData(temp, int(setting.Near), int(setting.Far), int(setting.Seconds), useNear)
		for s, data := range tempTurtle {
			turtleDataMap[s] = data
		}
	}
	for i := 0; i < len(sortedCandles); i++ {
		if sortedCandles[i] == nil {
			//util.Info(`error nil sorted candle`)
			continue
		}
		turtleTime := sortedCandles[i].Begin
		if market == model.GXZQ { // 因为有夜盘的存在，所以算作前一天的
			if turtleTime.Hour() < 5 {
				turtleTime = turtleTime.Add(time.Second * -1 * 86400)
			}
		}
		turtleKey := getTurtleKey(sortedCandles[i], slotSeconds)
		if turtleDataMap[turtleKey] != nil {
			handlePrice(turtleDataMap[turtleKey], sortedCandles[i], settings, allLimit, sign, useM)
		} else {
			value, ok := absentTurtles.Load(fmt.Sprintf(`%s%d`, sortedCandles[i].Symbol, turtleTime.Unix()))
			if !ok || value == nil || !value.(bool) {
				absentTurtles.Store(fmt.Sprintf(`%s%d`, sortedCandles[i].Symbol, turtleTime.Unix()), true)
				util.Info(fmt.Sprintf(`fail to get turtle data parse time from %s to %s %s unix second %d`,
					sortedCandles[i].Begin.String(), turtleKey, sortedCandles[i].Symbol, turtleTime.Unix()))
			}
		}
	}
}

type Statistic struct {
	amountBuy, amountSell, priceBuy, priceSell, uBuy, uSell, earnRate,
	groupUBuy, groupUSell, groupAmountBuy, groupAmountSell, rateInAllWin, rateInAllLose float64
	loses, wins []float64
}

func ToString(orders []*model.Order, market, singleLimit string, gridAmount float64, begin, end time.Time) (str string) {
	str = ``
	earnRateAll := 0.0
	statistics := make(map[string]*Statistic)
	for _, order := range orders {
		if statistics[order.Symbol] == nil {
			statistics[order.Symbol] = &Statistic{}
		}
		sta := statistics[order.Symbol]
		if order.OrderSide == model.OrderSideBuy {
			sta.amountBuy += order.Amount
			sta.uBuy += order.Amount * order.DealPrice
			if order.GridPos >= 0 {
				sta.groupUBuy += order.Amount * order.DealPrice
				sta.groupAmountBuy += order.Amount
			} else {
				rate := (sta.groupUSell/sta.groupAmountSell - order.DealPrice) / order.DealPrice
				if rate > 0 {
					sta.wins = append(sta.wins, rate)
					sta.rateInAllWin += rate
				} else {
					sta.loses = append(sta.loses, rate)
					sta.rateInAllLose += rate
				}
				sta.groupUBuy = 0
				sta.groupUSell = 0
				sta.groupAmountBuy = 0
				sta.groupAmountSell = 0
			}
		} else if order.OrderSide == model.OrderSideSell {
			sta.amountSell += order.Amount
			sta.uSell += order.Amount * order.DealPrice
			if order.GridPos <= 0 {
				sta.groupUSell += order.Amount * order.DealPrice
				sta.groupAmountSell += order.Amount
			} else {
				rate := (order.DealPrice - sta.groupUBuy/sta.groupAmountBuy) / order.DealPrice
				if rate > 0 {
					sta.wins = append(sta.wins, rate)
					sta.rateInAllWin += rate
				} else {
					sta.loses = append(sta.loses, rate)
					sta.rateInAllLose += rate
				}
				sta.groupUBuy = 0
				sta.groupUSell = 0
				sta.groupAmountBuy = 0
				sta.groupAmountSell = 0
			}
		}
		str += fmt.Sprintf("%s %s %s %s %f at %f %f 仓位%d 总仓%.0f\n",
			order.OrderTime.String(), order.Market, order.Symbol, order.OrderSide, order.Amount, order.Price, order.DealPrice, order.GridPos, order.UnfilledQuantity)
	}
	for symbol, statistic := range statistics {
		if statistic.amountBuy > 0 {
			statistic.priceBuy = statistic.uBuy / statistic.amountBuy
		}
		if statistic.amountSell > 0 {
			statistic.priceSell = statistic.uSell / statistic.amountSell
		}
		statistic.earnRate = (statistic.priceSell - statistic.priceBuy) / statistic.priceBuy * math.Min(statistic.amountBuy, statistic.amountSell)
		earnRateAll += statistic.earnRate
		str += fmt.Sprintf("%s %s %s %s 平均价差:%f‰ earnRate %.2f‰ 仓位限制:%s"+
			"\nbuy%.0f次 avgPrice %f\n"+"sell%.0f次 avgPrice %f\n",
			market, symbol, begin.String(), end.String(), 1000*(statistic.priceSell-statistic.priceBuy)/statistic.priceBuy,
			statistic.earnRate, singleLimit, statistic.amountBuy/gridAmount, statistic.priceBuy, statistic.amountSell/gridAmount, statistic.priceSell)
		avgWinRate := 0.0
		if len(statistic.wins) > 0 {
			avgWinRate = gridAmount * statistic.rateInAllWin / float64(len(statistic.wins))
		}
		avgLoseRate := 0.0
		if len(statistic.loses) > 0 {
			avgLoseRate = gridAmount * statistic.rateInAllLose / float64(len(statistic.loses))
		}
		str += fmt.Sprintf("平仓盈利%d次平均%f‰ 平仓亏损%d次平均%f‰\n", len(statistic.wins), avgWinRate, len(statistic.loses), avgLoseRate)
	}
	str += fmt.Sprintf(`earnRateAll: %f`, earnRateAll)
	return str
}

func CutTail(market, coins, sign string) {
	coinArray := strings.Split(coins, `,`)
	for _, coin := range coinArray {
		symbol := strings.ToUpper(coin) + model.UniStandardTail[model.MarketTypePerp]
		orders := make([]*model.Order, 0)
		model.AppDB.Where(`refresh_type=? and function=? and symbol=? and (order_type=? or order_type=?)`,
			model.FunctionSimulation, sign, symbol, model.OrderSideLiquidateShort, model.OrderSideLiquidateLong).
			Order(`order_time desc`).Limit(1).Find(&orders)
		if len(orders) > 0 {
			delNum := model.AppDB.Where(`refresh_type=? and function=? and symbol=? and order_time>?`,
				model.FunctionSimulation, sign, symbol, orders[0].OrderTime).Delete(&model.Order{}).RowsAffected
			if delNum > 0 {
				fmt.Println(fmt.Sprintf(`cut %s %s tail num %d`, market, symbol, delNum))
			}
		} else {
			util.Info(fmt.Sprintf(`can not get orders from %s`, sign))
		}
	}

}

func CreateReport(function string, coins []string) {
	rows, _ := model.AppDB.Model(model.Order{}).Select(`function,symbol,order_side,sum(orders.deal_price*amount)/sum(amount),sum(amount),sum(grid_pos)`).
		Where(`function = ?`, function).
		Group(`function,symbol,order_side`).Order(`function`).Rows()
	if rows == nil {
		return
	}
	i := 0
	var amount, price float64
	var count int
	var symbol, orderSide string
	resultPrice := make(map[string]map[string]float64)
	resultAmt := make(map[string]map[string]float64)
	for rows.Next() {
		_ = rows.Scan(&function, &symbol, &orderSide, &price, &amount, &count)
		if resultPrice[function] == nil {
			resultPrice[function] = make(map[string]float64)
		}
		if resultAmt[function] == nil {
			resultAmt[function] = make(map[string]float64)
		}
		resultPrice[function][symbol+`_`+orderSide] = price
		resultAmt[function][symbol+`_`+orderSide] = amount
		fmt.Println(fmt.Sprintf(`%d %s %s`, i, function, symbol))
		i++
	}
	for function = range resultPrice {
		result := 0.0
		line := function
		for _, coin := range coins {
			symbol = strings.ToUpper(coin) + model.UniStandardTail[model.MarketTypePerp]
			priceBuy := resultPrice[function][symbol+`_buy`]
			priceSell := resultPrice[function][symbol+`_sell`]
			amtBuy := resultAmt[function][symbol+`_buy`]
			amtSell := resultAmt[function][symbol+`_sell`]
			valid := amtBuy == 0 || amtSell == 0 || amtSell/amtBuy < 1.01 && amtBuy/amtSell > 0.99
			line += fmt.Sprintf(`,%f,%f,%f,%f,%v,%.2f`,
				priceBuy, priceSell, amtBuy, amtSell, valid, (priceSell-priceBuy)*amtBuy)
			result += (priceSell - priceBuy) * amtBuy
		}
		line += fmt.Sprintf(`,%.2f`, result)
		util.InfoSync(line)
	}
}

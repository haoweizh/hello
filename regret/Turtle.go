package regret

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strings"
	"time"
)

const tradeCost = 0.004

type TurtleData struct {
	highNear, lowNear, highFar, lowFar, n float64
	Near, Far                             int
	orderLong, orderShort                 *model.Order
	liquidated, useNear                   bool
	begin                                 time.Time
}

var slotLiquidated map[string]bool // market_orderSide_beginTimeString

func GetTurtleData(candles []*model.Candle, near, far int, useNear bool) (turtleDataMap map[string]*TurtleData) {
	turtleDataMap = make(map[string]*TurtleData)
	for i := far; i < len(candles); i++ {
		turtleData := &TurtleData{liquidated: false, n: candles[i-1].N, Near: near, Far: far, useNear: useNear, begin: candles[i].Begin}
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
			turtleKey := fmt.Sprintf(`%s_%s_%d_%s`, candles[i].Market, candles[i].Symbol, candles[i].Seconds,
				candles[i].Begin.Format(time.RFC3339))
			turtleDataMap[turtleKey] = turtleData
		}
	}
	return turtleDataMap
}

func createTurtleOrders(setting *model.Setting, turtleData *TurtleData, candle *model.Candle, currentChances, allLimit int64) {
	priceLong := turtleData.highFar
	priceShort := turtleData.lowFar
	amountShot := 0.0
	amountLong := 0.0
	var liquidateLong, liquidateShort bool
	if setting.Chance == 0 && !turtleData.liquidated { // 开初始仓
		if !slotLiquidated[fmt.Sprintf(`%s_%s_%s`, setting.Market, model.OrderSideBuy, turtleData.begin.String())] {
			amountLong = setting.GridAmount
		} else {
			util.Info(fmt.Sprintf(`no new open buy as %s liquated`, turtleData.begin.String()))
		}
		if !slotLiquidated[fmt.Sprintf(`%s_%s_%s`, setting.Market, model.OrderSideSell, turtleData.begin.String())] {
			amountShot = setting.GridAmount
		} else {
			util.Info(fmt.Sprintf(`no new open sell as %s liquated`, turtleData.begin.String()))
		}
	} else if setting.Chance > 0 {
		priceLong = math.Max(turtleData.highFar, setting.PriceX+turtleData.n/2)
		if turtleData.useNear {
			priceShort = math.Max(setting.PriceX-2*turtleData.n, turtleData.lowNear)
		} else {
			priceShort = turtleData.highFar - 2*turtleData.n
		}
		amountShot = float64(setting.Chance) * setting.GridAmount
		liquidateLong = true
		if float64(setting.Chance) < setting.AmountLimit {
			amountLong = setting.GridAmount
		}
	} else if setting.Chance < 0 {
		priceShort = math.Min(turtleData.lowFar, setting.PriceX-turtleData.n/2)
		if turtleData.useNear {
			priceLong = math.Min(setting.PriceX+2*turtleData.n, turtleData.highNear)
		} else {
			priceLong = turtleData.lowFar + 2*turtleData.n
		}
		liquidateShort = true
		amountLong = math.Abs(float64(setting.Chance)) * setting.GridAmount
		if math.Abs(float64(setting.Chance)) < setting.AmountLimit {
			amountShot = setting.GridAmount
		}
	}
	if currentChances >= allLimit {
		amountLong = 0
	}
	if currentChances <= -1*allLimit {
		amountShot = 0
	}
	if amountShot > 0 {
		turtleData.orderShort = &model.Order{
			Amount:      amountShot,
			Market:      setting.Market,
			OrderSide:   model.OrderSideSell,
			Price:       priceShort,
			DealPrice:   priceShort * (1 - tradeCost),
			RefreshType: model.FunctionSimulation,
			Status:      model.CarryStatusWorking,
			Symbol:      setting.Symbol,
			GridPos:     setting.Chance,
			CreatedAt:   candle.Begin,
		}
		if liquidateLong {
			turtleData.orderShort.OrderType = model.OrderSideLiquidateLong
		}
		util.Info(fmt.Sprintf(`create turtle long at %s %s %d`, candle.Begin.String(), candle.Symbol, setting.Chance))
	}
	if amountLong > 0 {
		turtleData.orderLong = &model.Order{
			Amount:      amountLong,
			Market:      setting.Market,
			OrderSide:   model.OrderSideBuy,
			Price:       priceLong,
			DealPrice:   priceLong * (1 + tradeCost),
			RefreshType: model.FunctionSimulation,
			Status:      model.CarryStatusWorking,
			Symbol:      setting.Symbol,
			GridPos:     setting.Chance,
			CreatedAt:   candle.Begin,
		}
		if liquidateShort {
			turtleData.orderLong.OrderType = model.OrderSideLiquidateShort
		}
		util.Info(fmt.Sprintf(`create turtle short at %s %s %d`, candle.Begin.String(), candle.Symbol, setting.Chance))
	}
}

func getCurrentChances(settings map[string]*model.Setting) (chances int64) {
	for _, setting := range settings {
		chances += setting.Chance
	}
	return chances
}

func handlePrice(turtleData *TurtleData, candle *model.Candle, settings map[string]*model.Setting, allLimit int, sign string) {
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
		createTurtleOrders(setting, turtleData, candle, currentChances, int64(allLimit))
	}
	if currentChances >= int64(allLimit) {
		turtleData.orderLong = nil
	} else if currentChances <= -1*int64(allLimit) {
		turtleData.orderShort = nil
	}
	if turtleData.orderLong != nil && candle.PriceHigh >= turtleData.orderLong.Price {
		if setting.Chance >= 0 {
			setting.Chance += 1
		} else {
			setting.Chance = 0
			turtleData.liquidated = true
			slotLiquidated[fmt.Sprintf(`%s_%s_%s`, candle.Market, model.OrderSideBuy, turtleData.begin.String())] = true
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
		} else {
			setting.Chance = 0
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

func getTurtleCandles(candles []*model.Candle) {
	for i, candle := range candles {
		if i == 0 {
			candle.N = candle.PriceHigh - candle.PriceLow
		} else {
			candle.N = (candle.PriceHigh-candle.PriceLow)/20 + candles[i-1].N*0.95
		}
	}
}

// ProcessCandles
// setting.AmountLimit 开仓数上限
// setting.GridAmount 标准一仓的数量
func ProcessCandles(start, end time.Time, near, far, turtleSeconds, allLimit int, useNear bool, market, sign string, settings map[string]*model.Setting) {
	if settings == nil || len(settings) == 0 {
		return
	}
	util.StoreSyncMap(&model.CarryInfo, nil, `GetCandle`)
	key := model.AppConfig.GetAccounts(market)[0].Key
	secret := model.AppConfig.GetAccounts(market)[0].Secret
	sortedCandles := api.GetMultiCandle(key, secret, market, 60, start, end, settings)
	ago := int(math.Max(30, float64(far)))
	duration, _ := time.ParseDuration(fmt.Sprintf(`-%ds`, turtleSeconds*ago))
	turtleCandles := make(model.Candles, 0)
	turtleDataMap := make(map[string]*TurtleData)
	slotLiquidated = make(map[string]bool)
	if sortedCandles != nil && sortedCandles.Len() > 0 && sortedCandles[0] != nil && sortedCandles[len(sortedCandles)-1] != nil {
		util.Info(fmt.Sprintf(`get sorted candles from %s %s to %s %s`,
			sortedCandles[0].Begin.String(), sortedCandles[0].Symbol,
			sortedCandles[sortedCandles.Len()-1].Begin.String(), sortedCandles[sortedCandles.Len()-1].Symbol))
	}
	for _, setting := range settings {
		temp := api.GetCandle(key, secret, market, setting.Symbol, turtleSeconds, start.Add(duration), end)
		util.Info(fmt.Sprintf(`get turtle candle %s %s %d setting chance %d`,
			market, setting.Symbol, len(turtleCandles), setting.Chance))
		getTurtleCandles(temp)
		tempTurtle := GetTurtleData(temp, near, far, useNear)
		turtleCandles = append(turtleCandles, temp...)
		for s, data := range tempTurtle {
			turtleDataMap[s] = data
		}
	}
	for i := 0; i < len(sortedCandles); i++ {
		if sortedCandles[i] == nil {
			util.Info(`error nil sorted candle`)
			continue
		}
		turtleTime := time.Unix(sortedCandles[i].Begin.Unix()-sortedCandles[i].Begin.Unix()%int64(turtleSeconds), 0).In(time.UTC)
		turtleKey := fmt.Sprintf(`%s_%s_%d_%s`, market, sortedCandles[i].Symbol, turtleSeconds, turtleTime.Format(time.RFC3339))
		if turtleDataMap[turtleKey] != nil {
			handlePrice(turtleDataMap[turtleKey], sortedCandles[i], settings, allLimit, sign)
		} else {
			util.Info(fmt.Sprintf(`fail to parse time from %s to %s %s`,
				sortedCandles[i].Begin.String(), turtleTime.String(), turtleKey))
		}
	}
}

type Statistic struct {
	amountBuy, amountSell, priceBuy, priceSell, uBuy, uSell, earnRate,
	groupUBuy, groupUSell, groupAmountBuy, groupAmountSell, rateInAllWin, rateInAllLose float64
	loses, wins []float64
}

func ToString(orders []*model.Order, market, simType, singleLimit string, gridAmount float64, begin, end time.Time) (str string) {
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
		str += fmt.Sprintf("%s %s %s %s 平均价差:%f‰ earnRate %.2f‰ 滑点%f type%s 仓位限制:%s"+
			"\nbuy%.0f次 avgPrice %f\n"+"sell%.0f次 avgPrice %f\n",
			market, symbol, begin.String(), end.String(), 1000*(statistic.priceSell-statistic.priceBuy)/statistic.priceBuy,
			statistic.earnRate, tradeCost, simType, singleLimit, statistic.amountBuy/gridAmount, statistic.priceBuy, statistic.amountSell/gridAmount, statistic.priceSell)
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

func CutTail(coins, sign string) {
	coinArray := strings.Split(coins, `,`)
	for _, coin := range coinArray {
		symbol := strings.ToUpper(coin) + model.UniStandardTail[model.MarketTypePerp]
		orders := make([]*model.Order, 0)
		model.AppDB.Where(`refresh_type=? and function=? and symbol=? and (order_type=? or order_type=?)`,
			model.FunctionSimulation, sign, symbol, model.OrderSideLiquidateShort, model.OrderSideLiquidateLong).
			Order(`order_time desc`).Limit(1).Find(&orders)
		if len(orders) > 0 {
			go func() {
				delNum := model.AppDB.Where(`refresh_type=? and function=? and symbol=? and order_time>?`,
					model.FunctionSimulation, sign, symbol, orders[0].OrderTime).Delete(&model.Order{}).RowsAffected
				if delNum > 0 {
					fmt.Println(fmt.Sprintf(`cut %s tail num %d`, symbol, delNum))
				}
			}()
		} else {
			fmt.Println(`can not get orders from ` + sign)
		}
	}

}

func CreateReport(coins, timeRange string) {
	rows, _ := model.AppDB.Model(model.Order{}).Select(`function,symbol,order_side,sum(orders.deal_price*amount)/sum(amount),sum(amount)/1000`).
		Where(`function like ? and function like ?`, `%coins`+coins+`,seconds%`, `%`+timeRange+`%`).
		Group(`function,symbol,order_side`).Order(`function`).Rows()
	if rows == nil {
		return
	}
	i := 0
	var count, price float64
	var symbol, orderSide, function string
	result := make(map[string]map[string]string, 0)
	for rows.Next() {
		_ = rows.Scan(&function, &symbol, &orderSide, &price, &count)
		if result[function] == nil {
			result[function] = make(map[string]string)
		}
		result[function][symbol+`_`+orderSide] = fmt.Sprintf(`%f,%.0f`, price, count)
		fmt.Println(fmt.Sprintf(`%d %s %s`, i, function, symbol))
		i++
	}
	coinArray := strings.Split(coins, `,`)
	for function = range result {
		line := function
		for _, coin := range coinArray {
			symbol = strings.ToUpper(coin) + model.UniStandardTail[model.MarketTypePerp]
			line += fmt.Sprintf(`,%s,%s`, result[function][symbol+`_buy`], result[function][symbol+`_sell`])
		}
		util.InfoSync(line)
	}
}

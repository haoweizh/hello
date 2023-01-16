package regret

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"time"
)

const tradeCost = 0.004

type TurtleData struct {
	highNear, lowNear, highFar, lowFar, n float64
	Near, Far                             int
	orderLong, orderShort                 *model.Order
	liquidated, useNear                   bool
}

func GetTurtleData(candles []*model.Candle, near, far int, useNear bool) (turtleDataMap map[string]*TurtleData) {
	turtleDataMap = make(map[string]*TurtleData)
	for i := 20; i < len(candles); i++ {
		turtleData := &TurtleData{liquidated: false, n: candles[i-1].N, Near: near, Far: far, useNear: useNear}
		for j := 1; j < 21; j++ {
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
	amountLimit := strconv.FormatFloat(setting.AmountLimit, 'f', 0, 64)
	if setting.Chance == 0 && !turtleData.liquidated { // 开初始仓
		amountLong = setting.GridAmount
		amountShot = setting.GridAmount
	} else if setting.Chance > 0 {
		priceLong = math.Max(turtleData.highFar, setting.PriceX+turtleData.n/2)
		if turtleData.useNear {
			priceShort = math.Max(setting.PriceX-2*turtleData.n, turtleData.lowNear)
		} else {
			priceShort = turtleData.highFar - 2*turtleData.n
		}
		amountShot = float64(setting.Chance) * setting.GridAmount
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
			AmountType:  setting.SymbolRelated,
			Function:    amountLimit,
			CreatedAt:   candle.Begin,
			OrderType:   strconv.FormatBool(turtleData.useNear),
		}
		util.Info(fmt.Sprintf(`create turtle long at %s %d`, candle.Begin.String(), setting.Chance))
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
			AmountType:  setting.SymbolRelated,
			Function:    amountLimit,
			CreatedAt:   candle.Begin,
			OrderType:   strconv.FormatBool(turtleData.useNear),
		}
		util.Info(fmt.Sprintf(`create turtle short at %s %d`, candle.Begin.String(), setting.Chance))
	}
}

func getCurrentChances(settings map[string]*model.Setting) (chances int64) {
	for _, setting := range settings {
		chances += setting.Chance
	}
	return chances
}

func handlePrice(turtleData *TurtleData, candle *model.Candle, settings map[string]*model.Setting, allLimit int) {
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
		createTurtleOrders(setting, turtleData, candle, int64(allLimit), currentChances)
	}
	if turtleData.orderLong != nil && candle.PriceHigh >= turtleData.orderLong.Price {
		if setting.Chance >= 0 {
			setting.Chance += 1
		} else {
			setting.Chance = 0
			turtleData.liquidated = true
		}
		setting.PriceX = turtleData.orderLong.Price
		turtleData.orderLong.Status = model.CarryStatusSuccess
		turtleData.orderLong.OrderTime = candle.Begin
		turtleData.orderLong.OrderId = fmt.Sprintf(`%s%s%s%s%d%f%s`, setting.Market, setting.Symbol, setting.SymbolRelated,
			turtleData.orderLong.OrderSide, candle.Begin.Unix(), setting.AmountLimit, turtleData.orderLong.OrderType)
		util.Info(`deal long chance %d 总仓 %d save order %s at %s`,
			setting.Chance, currentChances, turtleData.orderLong.OrderId, turtleData.orderLong.OrderTime.String())
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
		}
		setting.PriceX = turtleData.orderShort.Price
		turtleData.orderShort.Status = model.CarryStatusSuccess
		turtleData.orderShort.OrderTime = candle.Begin
		turtleData.orderShort.OrderId = fmt.Sprintf(`%s%s%s%s%d%f%s`, setting.Market, setting.Symbol, setting.SymbolRelated,
			turtleData.orderShort.OrderSide, candle.Begin.Unix(), setting.AmountLimit, turtleData.orderShort.OrderType)
		util.Info(`deal short chance %d 总仓 %d save order %s at %s`,
			setting.Chance, currentChances, turtleData.orderShort.OrderId, turtleData.orderShort.OrderTime.String())
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
func ProcessCandles(start, end time.Time, near, far, turtleSeconds, allLimit int, useNear bool, market string, settings map[string]*model.Setting) {
	if settings == nil || len(settings) == 0 {
		return
	}
	util.StoreSyncMap(&model.CarryInfo, nil, `GetCandle`)
	key := model.AppConfig.GetAccounts(market)[0].Key
	secret := model.AppConfig.GetAccounts(market)[0].Secret
	sortedCandles := api.GetMultiCandle(key, secret, market, 60, start, end, settings)
	duration, _ := time.ParseDuration(fmt.Sprintf(`-%ds`, turtleSeconds*30))
	turtleCandles := make(model.Candles, 0)
	turtleDataMap := make(map[string]*TurtleData)
	for _, setting := range settings {
		temp := api.GetCandle(key, secret, market, setting.Symbol, turtleSeconds, start.Add(duration), end)
		util.Info(`get turtle candle %s %s %d setting chance %d`, market, setting.Symbol, len(turtleCandles), setting.Chance)
		getTurtleCandles(temp)
		tempTurtle := GetTurtleData(temp, near, far, useNear)
		turtleCandles = append(turtleCandles, temp...)
		for s, data := range tempTurtle {
			turtleDataMap[s] = data
		}
	}
	for i := 0; i < len(sortedCandles); i++ {
		turtleTime := time.Unix(sortedCandles[i].Begin.Unix()-sortedCandles[i].Begin.Unix()%int64(turtleSeconds), 0).In(time.UTC)
		turtleKey := fmt.Sprintf(`%s_%s_%d_%s`, market, sortedCandles[i].Symbol, turtleSeconds, turtleTime.Format(time.RFC3339))
		if turtleDataMap[turtleKey] != nil {
			handlePrice(turtleDataMap[turtleKey], sortedCandles[i], settings, allLimit)
		} else {
			util.Info(`fail to parse time from %s to %s %s`, sortedCandles[i].Begin.String(), turtleTime.String(), turtleKey)
		}
	}
}

func GetDBOrders(market, symbol, amountType, limit, orderType string, begin, end time.Time) (orders []*model.Order) {
	orders = []*model.Order{}
	model.AppDB.Where(`market=? and symbol=? and order_time>? and order_time<? and refresh_type=? and amount_type=? and function=? and order_type=?`,
		market, symbol, begin, end, model.FunctionSimulation, amountType, limit, orderType).Order(`order_time asc`).Find(&orders)
	return orders
}

func ToString(orders []*model.Order, setting *model.Setting, begin, end time.Time) (str string) {
	str = ``
	var amountBuy, amountSell, priceBuy, priceSell, uBuy, uSell, earnRate,
		groupUBuy, groupUSell, groupAmountBuy, groupAmountSell, rateInAllWin, rateInAllLose float64
	wins := make([]float64, 0)
	loses := make([]float64, 0)
	for _, order := range orders {
		if order.OrderSide == model.OrderSideBuy {
			amountBuy += order.Amount
			uBuy += order.Amount * order.DealPrice
			if order.GridPos >= 0 {
				groupUBuy += order.Amount * order.DealPrice
				groupAmountBuy += order.Amount
			} else {
				rate := (groupUSell/groupAmountSell - order.DealPrice) / order.DealPrice
				if rate > 0 {
					wins = append(wins, rate)
					rateInAllWin += rate
				} else {
					loses = append(loses, rate)
					rateInAllLose += rate
				}
				groupUBuy = 0
				groupUSell = 0
				groupAmountBuy = 0
				groupAmountSell = 0
			}
		} else if order.OrderSide == model.OrderSideSell {
			amountSell += order.Amount
			uSell += order.Amount * order.DealPrice
			if order.GridPos <= 0 {
				groupUSell += order.Amount * order.DealPrice
				groupAmountSell += order.Amount
			} else {
				rate := (order.DealPrice - groupUBuy/groupAmountBuy) / order.DealPrice
				if rate > 0 {
					wins = append(wins, rate)
					rateInAllWin += rate
				} else {
					loses = append(loses, rate)
					rateInAllLose += rate
				}
				groupUBuy = 0
				groupUSell = 0
				groupAmountBuy = 0
				groupAmountSell = 0
			}
		}
		str += fmt.Sprintf("%s %s %s %s %f at %f %f chance %d\n",
			order.OrderTime.String(), order.Market, order.Symbol, order.OrderSide, order.Amount, order.Price, order.DealPrice, order.GridPos)
	}
	if amountBuy > 0 {
		priceBuy = uBuy / amountBuy
	}
	if amountSell > 0 {
		priceSell = uSell / amountSell
	}
	earnRate = (priceSell - priceBuy) / priceBuy * math.Min(amountBuy, amountSell)
	str += fmt.Sprintf("%s %s %s %s 下单%d次平均价差:%f earnRate %.2f‰ 滑点%f type%s 仓位限制:%f"+
		"\nbuy%f次 avgPrice %f\n"+"sell%f次 avgPrice %f\n",
		setting.Market, setting.Symbol, begin.String(), end.String(), len(orders), (priceSell-priceBuy)/priceBuy,
		earnRate, tradeCost, setting.SymbolRelated, setting.AmountLimit, uBuy/setting.GridAmount, priceBuy, uSell/setting.GridAmount, priceSell)
	avgWinRate := 0.0
	if len(wins) > 0 {
		avgWinRate = setting.GridAmount * rateInAllWin / float64(len(wins))
	}
	avgLoseRate := 0.0
	if len(loses) > 0 {
		avgLoseRate = setting.GridAmount * rateInAllLose / float64(len(loses))
	}
	str += fmt.Sprintf("盈利%d次平均%f‰ 亏损%d次平均%f‰", len(wins), avgWinRate, len(loses), avgLoseRate)
	return str
}

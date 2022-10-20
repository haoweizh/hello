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
	high10, low10, high20, low20, high3, low3, n float64
	orderLong, orderShort                        *model.Order
	liquidated                                   bool
}

func GetTurtleData(candles []*model.Candle) (turtleDataMap map[string]*TurtleData) {
	turtleDataMap = make(map[string]*TurtleData)
	for i := 20; i < len(candles); i++ {
		turtleData := &TurtleData{liquidated: false, n: candles[i-1].N}
		for j := 1; j < 21; j++ {
			if candles[i-j].PriceHigh > turtleData.high20 && j <= 20 {
				turtleData.high20 = candles[i-j].PriceHigh
			}
			if (turtleData.low20 == 0 || turtleData.low20 > candles[i-j].PriceLow) && j <= 20 {
				turtleData.low20 = candles[i-j].PriceLow
			}
			if candles[i-j].PriceHigh > turtleData.high10 && j <= 10 {
				turtleData.high10 = candles[i-j].PriceHigh
			}
			if (turtleData.low10 == 0 || turtleData.low10 > candles[i-j].PriceLow) && j <= 10 {
				turtleData.low10 = candles[i-j].PriceLow
			}
			if candles[i-j].PriceHigh > turtleData.high3 && j <= 3 {
				turtleData.high3 = candles[i-j].PriceHigh
			}
			if (turtleData.low3 == 0 || turtleData.low3 > candles[i-j].PriceLow) && j <= 3 {
				turtleData.low3 = candles[i-j].PriceLow
			}
			turtleKey := fmt.Sprintf(`%s_%s_%d_%s`, candles[i].Market, candles[i].Symbol, candles[i].Seconds,
				candles[i].Begin.Format(time.RFC3339))
			turtleDataMap[turtleKey] = turtleData
		}
	}
	return turtleDataMap
}

func createTurtleOrders(setting *model.Setting, turtleData *TurtleData) {
	priceLong := turtleData.high20
	priceShort := turtleData.low20
	amountShot := 0.0
	amountLong := 0.0
	if setting.Chance == 0 && !turtleData.liquidated { // 开初始仓
		amountShot = setting.GridAmount
		amountLong = setting.GridAmount
	} else if setting.Chance > 0 {
		priceLong = math.Max(turtleData.high20, setting.PriceX+turtleData.n/2)
		if turtleData.low3 < setting.PriceX {
			priceShort = math.Max(setting.PriceX-2*turtleData.n, turtleData.low10)
		} else {
			priceShort = math.Max(turtleData.high20-2*turtleData.n, turtleData.low10)
		}
		amountShot = float64(setting.Chance) * setting.GridAmount
		if float64(setting.Chance) < setting.AmountLimit {
			amountLong = setting.GridAmount
		}
	} else if setting.Chance < 0 {
		priceShort = math.Min(turtleData.low20, setting.PriceX-turtleData.n/2)
		if turtleData.high3 > setting.PriceX {
			priceLong = math.Min(setting.PriceX+2*turtleData.n, turtleData.high10)
		} else {
			priceLong = math.Min(turtleData.low20+2*turtleData.n, turtleData.high10)
		}
		amountLong = math.Abs(float64(setting.Chance)) * setting.GridAmount
		if math.Abs(float64(setting.Chance)) < setting.AmountLimit {
			amountShot = setting.GridAmount
		}
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
		}
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
		}
	}
}

func handlePrice(turtleData *TurtleData, candle *model.Candle, setting *model.Setting) {
	if turtleData.orderLong == nil && turtleData.orderShort == nil {
		createTurtleOrders(setting, turtleData)
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
		turtleData.orderLong.OrderId = fmt.Sprintf(`%d%slong`, candle.Begin.Unix(), setting.SymbolRelated)
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
		turtleData.orderShort.OrderId = fmt.Sprintf(`%d%sshort`, candle.Begin.Unix(), setting.SymbolRelated)
		model.AppDB.Save(turtleData.orderShort)
		turtleData.orderLong = nil
		turtleData.orderShort = nil
	}
}

// ProcessCandles
// setting.AmountLimit 开仓数上限
// setting.GridAmount 标准一仓的数量
func ProcessCandles(market, symbol string, start, end time.Time, setting *model.Setting) {
	key := model.AppConfig.GetAccounts(market)[0].Key
	secret := model.AppConfig.GetAccounts(market)[0].Secret
	util.StoreSyncMap(&model.CarryInfo, nil, `GetCandle`)
	sortedCandles := api.GetCandle(key, secret, market, symbol, 15, start, end)
	turtleSeconds, _ := strconv.Atoi(setting.SymbolRelated)
	duration, _ := time.ParseDuration(fmt.Sprintf(`-%ds`, turtleSeconds*50))
	turtleCandles := api.GetCandle(key, secret, market, symbol, turtleSeconds, start.Add(duration), end)
	api.GetTurtleCandles(turtleCandles)
	turtleDataMap := GetTurtleData(turtleCandles)
	for _, candle := range sortedCandles {
		turtleTime := time.Date(candle.Begin.Year(), candle.Begin.Month(), candle.Begin.Day(), candle.Begin.Hour(),
			0, 0, 0, candle.Begin.Location())
		turtleKey := fmt.Sprintf(`%s_%s_%s_%s`, setting.Market, setting.Symbol, setting.SymbolRelated,
			turtleTime.Format(time.RFC3339))
		if turtleDataMap[turtleKey] != nil {
			handlePrice(turtleDataMap[turtleKey], candle, setting)
		}
	}
}

func GetDBOrders(market, symbol, amountType string, begin, end time.Time) (orders []*model.Order) {
	orders = []*model.Order{}
	model.AppDB.Where(`market=? and symbol=? and order_time>? and order_time<? and refresh_type=? and amount_type=?`,
		market, symbol, begin, end, model.FunctionSimulation, amountType).Order(`order_time asc`).Find(&orders)
	return orders
}

func ToString(orders []*model.Order, setting *model.Setting) (str string) {
	str = ``
	var amountBuy, amountSell, priceBuy, priceSell, uBuy, uSell, singleOrderU, earnRate,
		groupUBuy, groupUSell, groupAmountBuy, groupAmountSell, rateInAllWin, rateInAllLose float64
	wins := make([]float64, 0)
	loses := make([]float64, 0)
	for i, order := range orders {
		if i == 0 {
			singleOrderU = order.Price * order.Amount
		}
		if order.OrderSide == model.OrderSideBuy {
			amountBuy += order.Amount
			uBuy += order.Amount * order.DealPrice
			if int64(setting.AmountLimit) > order.GridPos {
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
			if int64(setting.AmountLimit) > order.GridPos {
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
	earnRate = (priceSell - priceBuy) * math.Min(amountBuy, amountSell) / singleOrderU
	str += fmt.Sprintf("\n%sbuy %f avgPrice %f cost %f sell %f avgPrice %f income %f earnRate %.2f‰ 滑点%f",
		time.Now().String(), amountBuy, priceBuy, uBuy, amountSell, priceSell, uSell, earnRate*1000, tradeCost)
	avgWinRate := 0.0
	if len(wins) > 0 {
		avgWinRate = 1000 * rateInAllWin / float64(len(wins))
	}
	avgLoseRate := 0.0
	if len(loses) > 0 {
		avgLoseRate = 1000 * rateInAllLose / float64(len(loses))
	}
	str += fmt.Sprintf("\n盈利%d次平均%f‰ 亏损%d次平均%f‰", len(wins), avgWinRate, len(loses), avgLoseRate)
	return str
}

package Grid

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"time"
)

// 滑点
const tradeCost = 0.002

type Data struct {
	priceLow, priceHigh, N float64
	orderBuy, orderSell    *model.Order
	begin                  time.Time
}

// setting.OpenShortMargin, CloseShortMargin 下单价格n的赚、亏加乘倍数
func getOrders(setting *model.Setting, data *Data) {
	if (data.orderBuy == nil || data.orderBuy.Status != model.CarryStatusWorking) && setting.Chance < 3 {
		price := data.priceLow + data.N/2
		if setting.Chance > 0 {
			price = math.Min(data.priceLow, setting.PriceX-data.N/2)
		} else if setting.Chance < 0 {
			priceChange := 1.5 * data.N
			if setting.Seconds == 14400 {
				priceChange = 2 * data.N
				if !model.CommonTurtleSymbols[setting.Symbol] {
					priceChange = 2.5 * data.N
				}
			}
			price = math.Max(setting.PriceX/3+data.priceHigh*2/3-priceChange, data.priceLow)
		}
		data.orderBuy = &model.Order{Amount: setting.GridAmount / price,
			Price:            price,
			UnfilledQuantity: setting.GridAmount / price,
			Function:         setting.Function,
			Market:           setting.Market,
			Fee:              data.N,
			OrderSide:        model.OrderSideBuy,
			OrderType:        model.OrderTypeLimit,
			RefreshType:      model.FunctionSimulation,
			Status:           model.CarryStatusWorking,
			Symbol:           setting.Symbol}
	}
	if (data.orderSell == nil || data.orderSell.Status != model.CarryStatusWorking) && setting.Chance > -3 {
		price := data.priceHigh - data.N/2
		if setting.Chance > 0 {
			priceChange := 1.5 * data.N
			if setting.Seconds == 14400 {
				priceChange = 2 * data.N
				if !model.CommonTurtleSymbols[setting.Symbol] {
					priceChange = 2.5 * data.N
				}
			}
			price = math.Min(setting.PriceX/3+data.priceLow*2/3+priceChange, data.priceHigh)
		} else if setting.Chance < 0 {
			price = math.Max(data.priceHigh, setting.PriceX+data.N/2)
		}
		data.orderSell = &model.Order{Amount: setting.GridAmount / price,
			Price:            price,
			UnfilledQuantity: setting.GridAmount / price,
			Function:         setting.Function,
			Market:           setting.Market,
			Fee:              data.N,
			OrderSide:        model.OrderSideSell,
			OrderType:        model.OrderTypeLimit,
			RefreshType:      model.FunctionSimulation,
			Status:           model.CarryStatusWorking,
			Symbol:           setting.Symbol}
	}
	return
}

var ProcessGrid = func(start, end time.Time, setting *model.Setting) {
	util.StoreSyncMap(&model.CarryInfo, nil, `gridInfo`)
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	candles := api.GetMultiCandle(account, setting.Market, 60, start, end,
		map[string]*model.Setting{setting.Symbol: setting}, false)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get market candle %s %s from %s len %d`,
		setting.Market, setting.Symbol, start.String(), len(candles)), `gridInfo`)
	gridCandles := api.CombineCandles(account, setting.Market, setting.Symbol, int(setting.Seconds), start.Add(time.Hour*-1200), end)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get grid candle %s %s from %s len %d`,
		setting.Market, setting.Symbol, start.Add(time.Hour*-1200).String(), len(gridCandles)), `gridInfo`)
	if gridCandles != nil && gridCandles.Len() > 0 && gridCandles[0] != nil && gridCandles[len(gridCandles)-1] != nil {
		util.Info(fmt.Sprintf(`get sorted gridCandles from %s %s to %s %s`,
			gridCandles[0].Begin.String(), gridCandles[0].Symbol,
			gridCandles[gridCandles.Len()-1].Begin.String(), gridCandles[gridCandles.Len()-1].Symbol))
	}

	beginPrice := 0.0
	calcLenN := 10
	if setting.Seconds == 14400 {
		calcLenN = 15
	}
	startPoint := int(math.Max(float64(calcLenN), float64(setting.Far)))
	for i := 0; i < startPoint; i++ {
		beginPrice += gridCandles[i].PriceHigh - gridCandles[i].PriceLow
	}
	gridMap := make(map[int64]*Data)
	gridCandles[calcLenN-1].N = beginPrice / float64(calcLenN)
	for i := startPoint; i < len(gridCandles); i++ {
		data := &Data{begin: gridCandles[i].Begin,
			N: (gridCandles[i-1].N*(float64(calcLenN)-1) + gridCandles[i].PriceHigh - gridCandles[i].PriceLow) / float64(calcLenN)}
		for j := i - int(setting.Far); j < i; j++ {
			if data.priceLow == 0 || data.priceLow > gridCandles[j].PriceLow {
				data.priceLow = gridCandles[j].PriceLow
			}
			if data.priceHigh < gridCandles[j].PriceHigh {
				data.priceHigh = gridCandles[j].PriceHigh
			}
		}
		beginUnix := candles[i].Begin.Unix() - candles[i].Begin.Unix()%setting.Seconds
		gridMap[beginUnix] = data
	}
	for i := 0; i < len(candles); i++ {
		beginUnix := candles[i].Begin.Unix() - candles[i].Begin.Unix()%setting.Seconds
		if gridMap[beginUnix] == nil {
			util.Notice(fmt.Sprintf(`fail to get combine candle %s %s %d`, candles[i].Market, candles[i].Symbol, beginUnix))
			continue
		}
		getOrders(setting, gridMap[beginUnix])
		handleGrid(setting, gridMap[beginUnix], candles[i])
		//handleOld(setting, currentGrid.oldOrders, candles[i], currentGrid.candle.N)
		util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`deal grid candle %s %s for grid %s`,
			setting.Market, setting.Symbol, candles[i].Begin.String()), `gridInfo`)
	}
}

func dealOldLiquidate(setting *model.Setting, oldOrder *model.Order, candle *model.Candle, side, orderType string, price, n float64) {
	dealGridSuccess(setting, &model.Order{Amount: oldOrder.Amount,
		Price:       price,
		Function:    oldOrder.Function,
		GridPos:     3,
		Market:      oldOrder.Market,
		Fee:         n,
		OrderId:     fmt.Sprintf(`liquidate%s`, oldOrder.OrderId),
		OrderSide:   side,
		OrderType:   orderType,
		RefreshType: model.FunctionSimulation,
		Symbol:      oldOrder.Symbol}, candle)
}

func dealGridSuccess(setting *model.Setting, order *model.Order, candle *model.Candle) {
	order.Status = model.CarryStatusSuccess
	order.DealAmount = order.Amount
	order.UnfilledQuantity = 0
	order.DealPrice = order.Price
	order.LineBuy = setting.OpenShortMargin
	order.LineSell = setting.CloseShortMargin
	order.OrderTime = candle.Begin
	order.OrderId = fmt.Sprintf(`%d_%d`, candle.Begin.Unix(), rand.Int())
	if order.OrderType != model.OrderTypeLimit {
		if order.OrderSide == model.OrderSideSell {
			order.DealPrice = order.Price * (1 - tradeCost)
		} else if order.OrderSide == model.OrderSideBuy {
			order.DealPrice = order.Price * (1 + tradeCost)
		}
	}
	model.AppDB.Save(order)
}

func handleOld(setting *model.Setting, orders map[string]*model.Order, candle *model.Candle, n float64) {
	for orderId, order := range orders {
		price := 0.0
		orderType := ``
		if order.OrderSide == model.OrderSideSell {
			if candle.PriceHigh > order.Price+n*setting.CloseShortMargin {
				price = order.Price + n*setting.CloseShortMargin
				orderType = model.OrderTypeStop
			} else if candle.PriceLow < order.Price-n*setting.OpenShortMargin {
				price = order.Price - n*setting.OpenShortMargin
				orderType = model.OrderTypeLimit
			}
		}
		if order.OrderSide == model.OrderSideBuy {
			if candle.PriceHigh > order.Price+n*setting.OpenShortMargin {
				price = order.Price + n*setting.OpenShortMargin
				orderType = model.OrderTypeLimit
			} else if candle.PriceLow < order.Price-n*setting.CloseShortMargin {
				price = order.Price - n*setting.CloseShortMargin
				orderType = model.OrderTypeStop
			}
		}
		if price != 0 {
			dealOldLiquidate(setting, order, candle, model.GetOppositeSide(order.OrderSide), orderType, price, n)
			delete(orders, orderId)
			util.Info(fmt.Sprintf(`remove old order %s left %d orders`, orderId, len(orders)))
		}
	}
}

func handleGrid(setting *model.Setting, data *Data, candle *model.Candle) {
	//if orders[0].Status == model.CarryStatusWorking {
	//	if (orders[0].OrderSide == model.OrderSideBuy && orders[0].Price > candle.PriceLow) ||
	//		(orders[0].OrderSide == model.OrderSideSell && orders[0].Price < candle.PriceHigh) {
	//		dealGridSuccess(setting, orders[0], candle)
	//		var winPrice, losePrice float64
	//		var side string
	//		if orders[0].OrderSide == model.OrderSideBuy {
	//			side = model.OrderSideSell
	//			winPrice = orders[0].Price + n*setting.OpenShortMargin
	//			losePrice = orders[0].Price - n*setting.CloseShortMargin
	//		} else {
	//			side = model.OrderSideBuy
	//			winPrice = orders[0].Price - n*setting.OpenShortMargin
	//			losePrice = orders[0].Price + n*setting.CloseShortMargin
	//		}
	//		orders[1] = &model.Order{Amount: setting.GridAmount / winPrice,
	//			Price:            winPrice,
	//			UnfilledQuantity: setting.GridAmount / winPrice,
	//			Function:         setting.Function,
	//			GridPos:          1,
	//			Market:           setting.Market,
	//			Fee:              n,
	//			OrderId:          fmt.Sprintf(`liquidate%s`, orders[0].OrderId),
	//			OrderSide:        side,
	//			OrderType:        model.OrderTypeLimit,
	//			RefreshType:      model.FunctionSimulation,
	//			Status:           model.CarryStatusWorking,
	//			Symbol:           setting.Symbol,
	//			OrderTime:        candle.Begin}
	//		orders[2] = &model.Order{Amount: setting.GridAmount / losePrice,
	//			Price:            losePrice,
	//			UnfilledQuantity: setting.GridAmount / losePrice,
	//			Function:         setting.Function,
	//			GridPos:          2,
	//			Market:           setting.Market,
	//			Fee:              n,
	//			OrderId:          fmt.Sprintf(`liquidate%s`, orders[0].OrderId),
	//			OrderSide:        side,
	//			OrderType:        model.OrderTypeStop,
	//			RefreshType:      model.FunctionSimulation,
	//			Status:           model.CarryStatusWorking,
	//			Symbol:           setting.Symbol,
	//			OrderTime:        candle.Begin}
	//	}
	//}
	//if orders[1] != nil && ((orders[1].OrderSide == model.OrderSideSell && candle.PriceHigh > orders[1].Price) ||
	//	(orders[1].OrderSide == model.OrderSideBuy && candle.PriceLow < orders[1].Price)) {
	//	dealGridSuccess(setting, orders[1], candle)
	//	orders[0] = nil
	//}
	//if orders[2] != nil && ((orders[2].OrderSide == model.OrderSideSell && candle.PriceLow < orders[2].Price) ||
	//	(orders[2].OrderSide == model.OrderSideBuy && candle.PriceHigh > orders[2].Price)) {
	//	dealGridSuccess(setting, orders[2], candle)
	//	orders[0] = nil
	//}
}

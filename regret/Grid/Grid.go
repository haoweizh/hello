package Grid

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"sync"
	"time"
)

var grids = sync.Map{} // market_symbol: []*Candle

type gridData struct {
	downOrders, upOrders []*model.Order // 0: open 1: stopWin 2: stopLoss
	candle               *model.Candle
}

const gridDayLen = 30

func initGridData(setting *model.Setting, sortedCandles []*model.Candle) {
	beginPrice := 0.0
	for i := 0; i < gridDayLen; i++ {
		beginPrice += sortedCandles[i].PriceHigh - sortedCandles[i].PriceLow
	}
	sortedCandles[gridDayLen-1].N = beginPrice / gridDayLen
	for i := gridDayLen; i < len(sortedCandles); i++ {
		sortedCandles[i].N = (sortedCandles[i-1].N*(gridDayLen-1) +
			sortedCandles[i].PriceHigh - sortedCandles[i].PriceLow) / gridDayLen
		orderBuy := &model.Order{Amount: setting.GridAmount,
			Price:            sortedCandles[i].PriceOpen - sortedCandles[i].N,
			UnfilledQuantity: setting.GridAmount,
			Function:         setting.Function,
			Market:           setting.Market,
			LineBuy:          sortedCandles[i].N,
			OrderId: fmt.Sprintf(`%s%s%s%s_0`,
				setting.Market, setting.Symbol, model.OrderSideBuy, sortedCandles[i].Begin.String()),
			OrderSide:   model.OrderSideBuy,
			OrderType:   model.OrderTypeLimit,
			RefreshType: model.FunctionSimulation,
			Status:      model.CarryStatusWorking,
			Symbol:      setting.Symbol,
			OrderTime:   sortedCandles[i].Begin}
		orderSell := &model.Order{Amount: setting.GridAmount,
			Price:            sortedCandles[i].PriceOpen + sortedCandles[i].N,
			UnfilledQuantity: setting.GridAmount,
			Function:         setting.Function,
			Market:           setting.Market,
			LineBuy:          sortedCandles[i].N,
			OrderId: fmt.Sprintf(`%s%s%s%s_0`,
				setting.Market, setting.Symbol, model.OrderSideSell, sortedCandles[i].Begin.String()),
			OrderSide:   model.OrderSideSell,
			OrderType:   model.OrderTypeLimit,
			RefreshType: model.FunctionSimulation,
			Status:      model.CarryStatusWorking,
			Symbol:      setting.Symbol,
			OrderTime:   sortedCandles[i].Begin}
		data := gridData{downOrders: []*model.Order{orderBuy, nil, nil}, upOrders: []*model.Order{orderSell, nil, nil},
			candle: sortedCandles[i]}
		grids.Store(fmt.Sprintf(`%s_%s_%d`, setting.Market, setting.Symbol, sortedCandles[i].Begin.Second()), data)
	}
}

var ProcessGrid = func(start, end time.Time, setting *model.Setting) {
	util.StoreSyncMap(&model.CarryInfo, nil, `gridInfo`)
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	candles := api.GetMultiCandle(account.Key, account.Secret, setting.Market, 60, start, end,
		map[string]*model.Setting{setting.Symbol: setting}, false)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get market candle %s %s from %s len %d`,
		setting.Market, setting.Symbol, start.String(), len(candles)), `gridInfo`)
	gridCandles := api.GetMultiCandle(account.Key, account.Secret, setting.Market, 14400,
		start.Add(time.Hour*-1200), end, map[string]*model.Setting{setting.Symbol: setting}, false)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get grid candle %s %s from %s len %d`,
		setting.Market, setting.Symbol, start.Add(time.Hour*-1200).String(), len(gridCandles)), `gridInfo`)
	if gridCandles != nil && gridCandles.Len() > 0 && gridCandles[0] != nil && gridCandles[len(gridCandles)-1] != nil {
		util.Info(fmt.Sprintf(`get sorted gridCandles from %s %s to %s %s`,
			gridCandles[0].Begin.String(), gridCandles[0].Symbol,
			gridCandles[gridCandles.Len()-1].Begin.String(), gridCandles[gridCandles.Len()-1].Symbol))
	}
	initGridData(setting, gridCandles)
	for i := 0; i < len(candles); i++ {
		gridKey := fmt.Sprintf(`%s_%s_%d`, setting.Market, setting.Symbol, candles[i].Begin.Second())
		value, ok := grids.Load(gridKey)
		if !ok || value == nil {
			util.Info(fmt.Sprintf(`fail to get grid data parse time from %s to %s`,
				candles[i].Begin.String(), gridKey))
			continue
		}
		handleGrid(setting, value.(*gridData).downOrders, value.(*gridData).candle, value.(*gridData).candle.N)
		handleGrid(setting, value.(*gridData).upOrders, value.(*gridData).candle, value.(*gridData).candle.N)
		util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`deal grid candle %s %s %s for grid %s`,
			setting.Market, setting.Symbol, candles[i].Begin.String(), value.(*gridData).candle.Begin.String()), `gridInfo`)
	}
}

// 滑点
const tradeCost = 0.004

func dealGridSuccess(order *model.Order) {
	order.Status = model.CarryStatusSuccess
	order.DealAmount = order.Amount
	order.UnfilledQuantity = 0
	order.DealPrice = order.Price
	if order.OrderType == model.OrderTypeStop {
		if order.OrderSide == model.OrderSideSell {
			order.DealPrice = order.Price * (1 - tradeCost)
		} else if order.OrderSide == model.OrderSideBuy {
			order.DealPrice = order.Price * (1 + tradeCost)
		}
	}
	model.AppDB.Save(order)
}

func handleGrid(setting *model.Setting, orders []*model.Order, candle *model.Candle, n float64) {
	// 已经完成了开仓关仓，设置为nil
	if orders[0] == nil {
		return
	}
	if orders[0].Status == model.CarryStatusWorking {
		if (orders[0].OrderSide == model.OrderSideBuy && orders[0].Price > candle.PriceLow) ||
			(orders[0].OrderSide == model.OrderSideSell && orders[0].Price < candle.PriceHigh) {
			dealGridSuccess(orders[0])
			var winPrice, losePrice float64
			var side string
			if orders[0].OrderSide == model.OrderSideBuy {
				side = model.OrderSideSell
				winPrice = orders[0].Price + n
				losePrice = orders[0].Price - n
			} else {
				side = model.OrderSideBuy
				winPrice = orders[0].Price - n
				losePrice = orders[0].Price + n
			}
			orders[1] = &model.Order{Amount: setting.GridAmount,
				Price:            winPrice,
				UnfilledQuantity: setting.GridAmount,
				Function:         setting.Function,
				Market:           setting.Market,
				LineBuy:          n,
				OrderId:          fmt.Sprintf(`%s%s%s%s_1`, setting.Market, setting.Symbol, side, candle.Begin.String()),
				OrderSide:        side,
				OrderType:        model.OrderTypeLimit,
				RefreshType:      model.FunctionSimulation,
				Status:           model.CarryStatusWorking,
				Symbol:           setting.Symbol,
				OrderTime:        candle.Begin}
			orders[2] = &model.Order{Amount: setting.GridAmount,
				Price:            losePrice,
				UnfilledQuantity: setting.GridAmount,
				Function:         setting.Function,
				Market:           setting.Market,
				LineBuy:          n,
				OrderId:          fmt.Sprintf(`%s%s%s%s_2`, setting.Market, setting.Symbol, side, candle.Begin.String()),
				OrderSide:        side,
				OrderType:        model.OrderTypeStop,
				RefreshType:      model.FunctionSimulation,
				Status:           model.CarryStatusWorking,
				Symbol:           setting.Symbol,
				OrderTime:        candle.Begin}
		}
	}
	if orders[1] != nil && ((orders[1].OrderSide == model.OrderSideSell && candle.PriceHigh > orders[1].Price) ||
		(orders[1].OrderSide == model.OrderSideBuy && candle.PriceLow < orders[1].Price)) {
		dealGridSuccess(orders[1])
		orders[0] = nil
	}
	if orders[2] != nil && ((orders[2].OrderSide == model.OrderSideSell && candle.PriceLow < orders[1].Price) ||
		(orders[2].OrderSide == model.OrderSideBuy && candle.PriceHigh > orders[2].Price)) {
		dealGridSuccess(orders[2])
		orders[0] = nil
	}
}

package Grid

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math/rand"
	"sync"
	"time"
)

// 滑点
const tradeCost = 0.002
const NCalcLen = 50

var grids = sync.Map{} // market_symbol: []*Candle

type gridData struct {
	downOrders, upOrders []*model.Order          // 0: open 1: stopWin 2: stopLoss
	oldOrders            map[string]*model.Order // orderId - *order
	candle               *model.Candle
}

// setting.OpenShortMargin, CloseShortMargin 下单价格n的赚、亏加乘倍数
func initGridData(setting *model.Setting, sortedCandles []*model.Candle) {
	beginPrice := 0.0
	for i := 0; i < NCalcLen; i++ {
		beginPrice += sortedCandles[i].PriceHigh - sortedCandles[i].PriceLow
	}
	sortedCandles[NCalcLen-1].N = beginPrice / NCalcLen
	for i := NCalcLen; i < len(sortedCandles); i++ {
		sortedCandles[i].N = (sortedCandles[i-1].N*(NCalcLen-1) +
			sortedCandles[i].PriceHigh - sortedCandles[i].PriceLow) / NCalcLen
		orderBuy := &model.Order{Amount: setting.GridAmount / (sortedCandles[i].PriceOpen - 2*sortedCandles[i].N),
			Price:            sortedCandles[i].PriceOpen - 2*sortedCandles[i].N,
			UnfilledQuantity: setting.GridAmount / (sortedCandles[i].PriceOpen - 2*sortedCandles[i].N),
			Function:         setting.Function,
			GridPos:          0,
			Market:           setting.Market,
			Fee:              sortedCandles[i].N,
			OrderId:          fmt.Sprintf(`%d_%d`, sortedCandles[i].Begin.Unix(), rand.Int()),
			OrderSide:        model.OrderSideBuy,
			OrderType:        model.OrderTypeLimit,
			RefreshType:      model.FunctionSimulation,
			Status:           model.CarryStatusWorking,
			Symbol:           setting.Symbol,
			OrderTime:        sortedCandles[i].Begin}
		orderSell := &model.Order{Amount: setting.GridAmount / (sortedCandles[i].PriceOpen + 2*sortedCandles[i].N),
			Price:            sortedCandles[i].PriceOpen + 2*sortedCandles[i].N,
			UnfilledQuantity: setting.GridAmount / (sortedCandles[i].PriceOpen + 2*sortedCandles[i].N),
			Function:         setting.Function,
			GridPos:          0,
			Market:           setting.Market,
			Fee:              sortedCandles[i].N,
			OrderId:          fmt.Sprintf(`%d_%d`, sortedCandles[i].Begin.Unix(), rand.Int()),
			OrderSide:        model.OrderSideSell,
			OrderType:        model.OrderTypeLimit,
			RefreshType:      model.FunctionSimulation,
			Status:           model.CarryStatusWorking,
			Symbol:           setting.Symbol,
			OrderTime:        sortedCandles[i].Begin}
		data := &gridData{downOrders: []*model.Order{orderBuy, nil, nil}, upOrders: []*model.Order{orderSell, nil, nil},
			candle: sortedCandles[i]}
		grids.Store(fmt.Sprintf(`%s_%s_%d`, setting.Market, setting.Symbol, sortedCandles[i].Begin.Unix()), data)
	}
}

var ProcessGrid = func(start, end time.Time, setting *model.Setting) {
	util.StoreSyncMap(&model.CarryInfo, nil, `gridInfo`)
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	candles := api.GetMultiCandle(account.Key, account.Secret, setting.Market, 60, start, end,
		map[string]*model.Setting{setting.Symbol: setting}, false)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get market candle %s %s from %s len %d`,
		setting.Market, setting.Symbol, start.String(), len(candles)), `gridInfo`)
	gridCandles := api.CombineCandles(account.Key, account.Secret, setting.Market, setting.Symbol, 14400, start.Add(time.Hour*-1200), end)
	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`get grid candle %s %s from %s len %d`,
		setting.Market, setting.Symbol, start.Add(time.Hour*-1200).String(), len(gridCandles)), `gridInfo`)
	if gridCandles != nil && gridCandles.Len() > 0 && gridCandles[0] != nil && gridCandles[len(gridCandles)-1] != nil {
		util.Info(fmt.Sprintf(`get sorted gridCandles from %s %s to %s %s`,
			gridCandles[0].Begin.String(), gridCandles[0].Symbol,
			gridCandles[gridCandles.Len()-1].Begin.String(), gridCandles[gridCandles.Len()-1].Symbol))
	}
	initGridData(setting, gridCandles)
	var currentGrid *gridData
	for i := 0; i < len(candles); i++ {
		beginUnix := candles[i].Begin.Unix() - candles[i].Begin.Unix()%14400
		gridKey := fmt.Sprintf(`%s_%s_%d`, setting.Market, setting.Symbol, beginUnix)
		value, ok := grids.Load(gridKey)
		if !ok || value == nil {
			util.Info(fmt.Sprintf(`fail to get grid data parse time from %s to %s`,
				candles[i].Begin.String(), gridKey))
			continue
		}
		if currentGrid != nil && currentGrid.candle.Begin.Unix() < beginUnix {
			if currentGrid.oldOrders == nil {
				currentGrid.oldOrders = make(map[string]*model.Order)
			}
			if currentGrid.upOrders[0] != nil && currentGrid.upOrders[0].Status == model.CarryStatusSuccess {
				currentGrid.oldOrders[currentGrid.upOrders[0].OrderId] = currentGrid.upOrders[0]
			}
			if currentGrid.downOrders[0] != nil && currentGrid.downOrders[0].Status == model.CarryStatusSuccess {
				currentGrid.oldOrders[currentGrid.downOrders[0].OrderId] = currentGrid.downOrders[0]
			}
			value.(*gridData).oldOrders = currentGrid.oldOrders
		}
		currentGrid = value.(*gridData)
		handleGrid(setting, currentGrid.downOrders, candles[i], currentGrid.candle.N)
		handleGrid(setting, currentGrid.upOrders, candles[i], currentGrid.candle.N)
		handleOld(setting, currentGrid.oldOrders, candles[i], currentGrid.candle.N)
		util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(`deal grid candle %s %s %s for grid %s`,
			setting.Market, setting.Symbol, candles[i].Begin.String(), currentGrid.candle.Begin.String()), `gridInfo`)
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

func handleGrid(setting *model.Setting, orders []*model.Order, candle *model.Candle, n float64) {
	// 已经完成了开仓关仓，设置为nil
	if orders[0] == nil {
		return
	}
	if orders[0].Status == model.CarryStatusWorking {
		if (orders[0].OrderSide == model.OrderSideBuy && orders[0].Price > candle.PriceLow) ||
			(orders[0].OrderSide == model.OrderSideSell && orders[0].Price < candle.PriceHigh) {
			dealGridSuccess(setting, orders[0], candle)
			var winPrice, losePrice float64
			var side string
			if orders[0].OrderSide == model.OrderSideBuy {
				side = model.OrderSideSell
				winPrice = orders[0].Price + n*setting.OpenShortMargin
				losePrice = orders[0].Price - n*setting.CloseShortMargin
			} else {
				side = model.OrderSideBuy
				winPrice = orders[0].Price - n*setting.OpenShortMargin
				losePrice = orders[0].Price + n*setting.CloseShortMargin
			}
			orders[1] = &model.Order{Amount: setting.GridAmount / winPrice,
				Price:            winPrice,
				UnfilledQuantity: setting.GridAmount / winPrice,
				Function:         setting.Function,
				GridPos:          1,
				Market:           setting.Market,
				Fee:              n,
				OrderId:          fmt.Sprintf(`liquidate%s`, orders[0].OrderId),
				OrderSide:        side,
				OrderType:        model.OrderTypeLimit,
				RefreshType:      model.FunctionSimulation,
				Status:           model.CarryStatusWorking,
				Symbol:           setting.Symbol,
				OrderTime:        candle.Begin}
			orders[2] = &model.Order{Amount: setting.GridAmount / losePrice,
				Price:            losePrice,
				UnfilledQuantity: setting.GridAmount / losePrice,
				Function:         setting.Function,
				GridPos:          2,
				Market:           setting.Market,
				Fee:              n,
				OrderId:          fmt.Sprintf(`liquidate%s`, orders[0].OrderId),
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
		dealGridSuccess(setting, orders[1], candle)
		orders[0] = nil
	}
	if orders[2] != nil && ((orders[2].OrderSide == model.OrderSideSell && candle.PriceLow < orders[2].Price) ||
		(orders[2].OrderSide == model.OrderSideBuy && candle.PriceHigh > orders[2].Price)) {
		dealGridSuccess(setting, orders[2], candle)
		orders[0] = nil
	}
}

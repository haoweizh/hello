package deprecated

//
//import (
//	"fmt"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"time"
//)
//
//type GridPos struct {
//	orders               []*model.Order // pos - orderId - order
//	pos                  []float64
//	amount, n            float64
//	orderLiquidate       *model.Order
//	posLength, posMiddle int
//}
//
//var dayGridPos = make(map[string]map[string]map[string]*GridPos) // dateStr - market - symbol - gridPos
//var gridCheckTime = util.GetNow()
//
//func calcGridAmount(key, secret, market, symbol string, price float64) (amount float64) {
//	switch market {
//	case model.Ftx:
//		_, _, value, _ := api.GetBalances(key, secret, market)
//		switch symbol { //使用20分之一的资本
//		case `BTC_PERP`:
//			amount = math.Round(value/price) / 20
//		case `LINK_PERP`:
//			amount = math.Round(value/price/2000) * 100
//		case `ETH_PERP`:
//			amount = math.Round(value / price / 20)
//		}
//	}
//	return amount
//}
//
//func getGridPos(key, secret string, setting *model.Setting) (gridPos *GridPos) {
//	today, _ := model.GetMarketToday(setting.Market)
//	yesterday, yesterdayStr := model.GetMarketYesterday(setting.Market)
//	if dayGridPos[yesterdayStr] != nil && dayGridPos[yesterdayStr][setting.Market] != nil &&
//		dayGridPos[yesterdayStr][setting.Market][setting.Symbol] != nil {
//		return dayGridPos[yesterdayStr][setting.Market][setting.Symbol]
//	}
//	candle := api.CalcCandleN(setting.Market, setting.Symbol, 86400, yesterday)
//	if candle == nil {
//		return
//	}
//	p := (candle.PriceHigh + candle.PriceLow + candle.PriceClose) / 3
//	util.Notice(fmt.Sprintf(`%s %s yesterday:%s grid candleh p: %f n:%f low %f high %f open %f close %f`,
//		setting.Market, setting.Symbol, yesterdayStr, p, candle.N, candle.PriceLow, candle.PriceHigh, candle.PriceOpen, candle.PriceClose))
//	if candle.PriceHigh-candle.PriceLow < candle.N*2/3 {
//		gridPos = &GridPos{orders: make([]*model.Order, 5), pos: make([]float64, 5), posLength: 5, posMiddle: 2}
//		gridPos.pos[1] = candle.PriceLow - 2*(candle.PriceHigh-p)
//		gridPos.pos[0] = 2*gridPos.pos[1] - p
//		gridPos.pos[2] = p
//		gridPos.pos[3] = candle.PriceHigh - 2*(candle.PriceLow-p)
//		gridPos.pos[4] = 2*gridPos.pos[3] - p
//	} else {
//		gridPos = &GridPos{orders: make([]*model.Order, 9), pos: make([]float64, 9), posLength: 9, posMiddle: 4}
//		gridPos.pos[1] = candle.PriceLow - 2*(candle.PriceHigh-p)
//		gridPos.pos[0] = 2*gridPos.pos[1] - p
//		gridPos.pos[2] = p - candle.PriceHigh + candle.PriceLow
//		gridPos.pos[3] = 2*p - candle.PriceHigh
//		gridPos.pos[4] = p
//		gridPos.pos[5] = 2*p - candle.PriceLow
//		gridPos.pos[6] = p + candle.PriceHigh - candle.PriceLow
//		gridPos.pos[7] = candle.PriceHigh - 2*(candle.PriceLow-p)
//		gridPos.pos[8] = 2*gridPos.pos[7] - p
//	}
//	gridPos.n = candle.N
//	gridPos.amount = calcGridAmount(key, secret, setting.Market, setting.Symbol, p)
//	if dayGridPos[yesterdayStr] == nil {
//		dayGridPos[yesterdayStr] = make(map[string]map[string]*GridPos)
//	}
//	if dayGridPos[yesterdayStr][setting.Market] == nil {
//		dayGridPos[yesterdayStr][setting.Market] = make(map[string]*GridPos)
//	}
//	dayGridPos[yesterdayStr][setting.Market][setting.Symbol] = gridPos
//	// load orders
//	var orders []*model.Order
//	model.AppDB.Where("market= ? and symbol= ? and refresh_type= ? and status=? and order_time>?",
//		setting.Market, setting.Symbol, model.FunctionGrid, model.CarryStatusWorking, yesterdayStr).Find(&orders)
//	util.Notice(fmt.Sprintf(`grid pos absent load orders %d from %s`, len(orders), yesterdayStr))
//	for _, order := range orders {
//		if order.OrderTime.Before(today) {
//			tempOrder := api.QueryOrderById(key, secret, order.Market, order.Symbol, order.OrderType, order.OrderId)
//			if tempOrder != nil {
//				order = tempOrder
//				if order.OrderSide == model.OrderSideBuy {
//					setting.GridAmount += math.Abs(order.DealAmount)
//				} else {
//					setting.GridAmount -= math.Abs(order.DealAmount)
//				}
//			}
//			if order.Status == model.CarryStatusWorking {
//				util.Notice(fmt.Sprintf(`cancel old grid order %s at %s`, order.OrderId, order.OrderTime))
//				api.MustCancel(key, secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, true)
//			}
//			continue
//		}
//		if order.GridPos == -1 {
//			gridPos.orderLiquidate = order
//		} else if ((order.OrderSide == model.OrderSideSell && order.GridPos > setting.Chance) ||
//			(order.OrderSide == model.OrderSideBuy && order.GridPos < setting.Chance)) &&
//			(gridPos.orders[order.GridPos] == nil || gridPos.orders[order.GridPos].OrderTime.Before(order.OrderTime)) {
//			gridPos.orders[order.GridPos] = order
//			util.Notice(fmt.Sprintf(`load orders[%d] add %s %s %s`,
//				len(orders), order.OrderSide, order.OrderId, order.OrderTime.String()))
//		}
//	}
//	for _, order := range gridPos.orders {
//		if order != nil {
//			return
//		}
//	}
//	setting.Chance = int64(gridPos.posMiddle)
//	model.AppDB.Save(setting)
//	//liquidateAmount := setting.GridAmount
//	for i := gridPos.posMiddle + 1; i < len(gridPos.pos); i++ {
//		amount := gridPos.amount
//		if setting.GridAmount > 0 && i == gridPos.posMiddle+1 {
//			amount = math.Min(2*gridPos.amount, gridPos.amount+setting.GridAmount)
//			//liquidateAmount = liquidateAmount + gridPos.amount - amount
//		}
//		orders = api.MustPlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, setting.Market, setting.Symbol,
//			``, model.FunctionGrid, gridPos.pos[i], gridPos.pos[i], amount, setting)
//		order := orders[0]
//		order.GridPos = int64(i)
//		dayGridPos[yesterdayStr][setting.Market][setting.Symbol].orders[i] = order
//		model.AppDB.Save(order)
//		util.Notice(fmt.Sprintf(`init grid %s %s sell at %d index %d pos %s %s %f %f`,
//			setting.Market, setting.Symbol, i, order.GridPos, order.OrderId, order.Status, order.Price, order.Amount))
//	}
//	for i := gridPos.posMiddle - 1; i >= 0; i-- {
//		amount := gridPos.amount
//		if setting.GridAmount < 0 && i == gridPos.posMiddle-1 {
//			amount = math.Min(2*gridPos.amount, gridPos.amount-setting.GridAmount)
//			//liquidateAmount = liquidateAmount + amount - gridPos.amount
//		}
//		orders = api.MustPlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, setting.Market, setting.Symbol,
//			``, model.FunctionGrid, gridPos.pos[i], gridPos.pos[i], amount, setting)
//		order := orders[0]
//		order.GridPos = int64(i)
//		dayGridPos[yesterdayStr][setting.Market][setting.Symbol].orders[i] = order
//		model.AppDB.Save(order)
//		util.Notice(fmt.Sprintf(`init grid %s %s buy at %d index %d pos %s %s %f %f`,
//			setting.Market, setting.Symbol, i, order.GridPos, order.OrderId, order.Status, order.Price, order.Amount))
//	}
//	return gridPos
//}
//
//// ProcessSimpleGrid setting: grid_amount持仓量, chance 当前position
//var ProcessSimpleGrid = func(setting *model.Setting, tick *model.BidAsk) {
//	if !api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, true) {
//		defer api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, false)
//	} else {
//		return
//	}
//	now := util.GetNowUnixMillion()
//	maintaining, ok := model.ChannelMaintaining.Load(setting.Market)
//	if setting == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.HandleLink != `1` ||
//		(ok && maintaining.(bool)) || now-int64(tick.Ts) > 1000 {
//		return
//	}
//	account := model.AppConfig.GetAccounts(setting.Market)[0]
//	gridPos := getGridPos(account.Key, account.Secret, setting)
//	if gridPos == nil {
//		return
//	}
//	showMsg := ``
//	duration, _ := time.ParseDuration(`-180s`)
//	checkTime := util.GetNow().Add(duration)
//	checkOrder := false
//	if checkTime.After(gridCheckTime) {
//		checkOrder = true
//		gridCheckTime = util.GetNow()
//	}
//	if setting.Chance-1 >= 0 {
//		i := setting.Chance - 1
//		order := gridPos.orders[i]
//		if checkOrder && order != nil {
//			tempOrder := api.QueryOrderById(account.Key, account.Secret, order.Market, order.Symbol, order.OrderType, order.OrderId)
//			if tempOrder != nil {
//				order.Status = tempOrder.Status
//				showMsg += fmt.Sprintf("%s %d %s %d %f %s %s %s %f\n",
//					order.Status, i, order.OrderSide, order.GridPos, order.Price, order.Market, order.Symbol, order.OrderId, order.Amount)
//			}
//		}
//		if order != nil && (order.Price > tick.Bids[0].Price || order.Status == model.CarryStatusSuccess) {
//			orderR := api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeLimit,
//				setting.Market, setting.Symbol, ``, model.FunctionGrid,
//				gridPos.pos[setting.Chance], gridPos.pos[setting.Chance], gridPos.amount, setting)
//			orderR[0].GridPos = setting.Chance
//			gridPos.orders[setting.Chance] = orderR[0]
//			setting.Chance = i
//			setting.PriceX = gridPos.orders[i].Price
//			setting.GridAmount += order.Amount
//			order.DealAmount = order.Amount
//			order.DealPrice = order.Price
//			order.Status = model.CarryStatusSuccess
//			model.AppDB.Save(orderR)
//			model.AppDB.Save(order)
//			model.AppDB.Save(setting)
//			gridPos.orders[i] = nil
//			util.Notice(fmt.Sprintf(`order success %s %s %s %s %s at %d %f with %f, new order %s %s at %d %f`,
//				order.Status, order.Market, order.Symbol, order.OrderSide, order.OrderId, i, order.Price, order.Amount,
//				orderR[0].OrderSide, orderR[0].OrderId, orderR[0].GridPos, orderR[0].Amount))
//		}
//	}
//	if setting.Chance+1 < int64(len(gridPos.pos)) {
//		i := setting.Chance + 1
//		order := gridPos.orders[i]
//		if checkOrder && order != nil {
//			tempOrder := api.QueryOrderById(account.Key, account.Secret, order.Market, order.Symbol, order.OrderType, order.OrderId)
//			if tempOrder != nil {
//				order.Status = tempOrder.Status
//				showMsg += fmt.Sprintf("%s %d %s %d %f %s %s %s %f\n",
//					order.Status, i, order.OrderSide, order.GridPos, order.Price, order.Market, order.Symbol, order.OrderId, order.Amount)
//			}
//		}
//		if order != nil && (order.Price < tick.Asks[0].Price || order.Status == model.CarryStatusSuccess) {
//			util.Notice(fmt.Sprintf(`check sell %d chance: %d order pos: %d ask0: %f order price %f`,
//				len(gridPos.pos), setting.Chance, order.GridPos, tick.Asks[0].Price, order.Price))
//			orderS := api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeLimit,
//				setting.Market, setting.Symbol, ``, model.FunctionGrid, gridPos.pos[setting.Chance],
//				gridPos.pos[setting.Chance], gridPos.amount, setting)
//			orderS[0].GridPos = setting.Chance
//			gridPos.orders[setting.Chance] = orderS[0]
//			setting.Chance = i
//			setting.PriceX = gridPos.orders[i].Price
//			setting.GridAmount -= order.Amount
//			order.DealAmount = order.Amount
//			order.DealPrice = order.Price
//			order.Status = model.CarryStatusSuccess
//			model.AppDB.Save(orderS)
//			model.AppDB.Save(order)
//			model.AppDB.Save(setting)
//			gridPos.orders[i] = nil
//			util.Notice(fmt.Sprintf(`order success %s %s %s %s %s at %d %f with %f, new order %s %s at %d %f`,
//				order.Status, order.Market, order.Symbol, order.OrderSide, order.OrderId, i, order.Price, order.Amount,
//				orderS[0].OrderSide, orderS[0].OrderId, orderS[0].GridPos, orderS[0].Amount))
//		}
//	}
//	if gridPos.orderLiquidate != nil {
//		showMsg += fmt.Sprintf("liquidate %s %d %f %s %s %s %f\n",
//			gridPos.orderLiquidate.OrderSide, gridPos.orderLiquidate.GridPos, gridPos.orderLiquidate.Price,
//			gridPos.orderLiquidate.Market, gridPos.orderLiquidate.Symbol, gridPos.orderLiquidate.OrderId,
//			gridPos.orderLiquidate.Amount)
//		dealAmount := 0.0
//		if gridPos.orderLiquidate.OrderSide == model.OrderSideBuy && tick.Bids[0].Price < gridPos.orderLiquidate.Price {
//			dealAmount = gridPos.orderLiquidate.Amount
//		}
//		if gridPos.orderLiquidate.OrderSide == model.OrderSideSell && tick.Asks[0].Price > gridPos.orderLiquidate.Price {
//			dealAmount -= gridPos.orderLiquidate.Amount
//		}
//		if dealAmount != 0.0 {
//			setting.PriceX = gridPos.orderLiquidate.Price
//			setting.GridAmount += dealAmount
//			gridPos.orderLiquidate.DealAmount = gridPos.orderLiquidate.Amount
//			gridPos.orderLiquidate.DealPrice = gridPos.orderLiquidate.Price
//			gridPos.orderLiquidate.Status = model.CarryStatusSuccess
//			model.AppDB.Save(gridPos.orderLiquidate)
//			model.AppDB.Save(setting)
//			util.Notice(fmt.Sprintf(`liquidation order success %s %s %s %s %f %f setting amount %f at %d`,
//				setting.Market, setting.Symbol, gridPos.orderLiquidate.OrderSide, gridPos.orderLiquidate.OrderId,
//				gridPos.orderLiquidate.Amount, gridPos.orderLiquidate.Price, setting.GridAmount, setting.Chance))
//			gridPos.orderLiquidate = nil
//		}
//	}
//	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionGrid, setting.Market, setting.Symbol)
//	util.StoreSyncMap(&model.CarryInfo, fmt.Sprintf(" chance:%d last price:%f holding:%f n值：%f\n%s",
//		setting.Chance, setting.PriceX, setting.GridAmount, gridPos.n, showMsg), account.Key, msgKey)
//}

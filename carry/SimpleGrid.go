package carry

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"sync"
	"time"
)

type GridPos struct {
	orders               []*model.Order // pos - orderId - order
	pos                  []float64
	amount, n            float64
	orderLiquidate       *model.Order
	posLength, posMiddle int
}

var dayGridPos = make(map[string]map[string]map[string]*GridPos) // dateStr - market - symbol - gridPos
var simpleGriding = false
var simpleGridLock sync.Mutex
var gridCheckTime = util.GetNow()

func setSimpleGriding(value bool) {
	simpleGridLock.Lock()
	defer simpleGridLock.Unlock()
	simpleGriding = value
}

func getSimpleGriding() (value bool) {
	simpleGridLock.Lock()
	defer simpleGridLock.Unlock()
	return simpleGriding
}

func calcGridAmount(market, symbol string, price float64) (amount float64) {
	switch market {
	case model.Ftx:
		value := api.GetUSDBalance(``, ``, market)
		switch symbol {
		case `btcusd_p`: //使用三分之一的资本
			amount = math.Round(10*value/price/5) / 10
		case `linkusd_p`: // 使用五分之一的资本
			amount = math.Round(value/price/5/100) * 100
		}
	}
	return amount
}

func getGridPos(setting *model.Setting) (gridPos *GridPos) {
	today, _ := model.GetMarketToday(setting.Market)
	yesterday, yesterdayStr := model.GetMarketYesterday(setting.Market)
	if dayGridPos[yesterdayStr] != nil && dayGridPos[yesterdayStr][setting.Market] != nil &&
		dayGridPos[yesterdayStr][setting.Market][setting.Symbol] != nil {
		return dayGridPos[yesterdayStr][setting.Market][setting.Symbol]
	}
	candle := api.GetDayCandle(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, ``, yesterday)
	p := (candle.PriceHigh + candle.PriceLow + candle.PriceClose) / 3
	util.Notice(fmt.Sprintf(`%s %s yesterday:%s get new grid pos with avgN:%f n:%f`,
		setting.Market, setting.Symbol, yesterdayStr, candle.PriceHigh-candle.PriceLow, candle.N))
	if candle.PriceHigh-candle.PriceLow < candle.N*2/3 {
		gridPos = &GridPos{orders: make([]*model.Order, 3), pos: make([]float64, 3), posLength: 3, posMiddle: 1}
		gridPos.pos[0] = candle.PriceLow - 2*(candle.PriceHigh-p)
		gridPos.pos[1] = p
		gridPos.pos[2] = candle.PriceHigh - 2*(candle.PriceLow-p)
	} else {
		gridPos = &GridPos{orders: make([]*model.Order, 7), pos: make([]float64, 7), posLength: 7, posMiddle: 3}
		gridPos.pos[0] = candle.PriceLow - 2*(candle.PriceHigh-p)
		gridPos.pos[1] = p - candle.PriceHigh + candle.PriceLow
		gridPos.pos[2] = 2*p - candle.PriceHigh
		gridPos.pos[3] = p
		gridPos.pos[4] = 2*p - candle.PriceLow
		gridPos.pos[5] = p + candle.PriceHigh - candle.PriceLow
		gridPos.pos[6] = candle.PriceHigh - 2*(candle.PriceLow-p)
	}
	gridPos.n = candle.N
	gridPos.amount = calcGridAmount(setting.Market, setting.Symbol, p)
	if dayGridPos[yesterdayStr] == nil {
		dayGridPos[yesterdayStr] = make(map[string]map[string]*GridPos)
	}
	if dayGridPos[yesterdayStr][setting.Market] == nil {
		dayGridPos[yesterdayStr][setting.Market] = make(map[string]*GridPos)
	}
	dayGridPos[yesterdayStr][setting.Market][setting.Symbol] = gridPos
	// load orders
	var orders []*model.Order
	model.AppDB.Where("market= ? and symbol= ? and refresh_type= ? and status=? and order_time>?",
		setting.Market, setting.Symbol, model.FunctionGrid, model.CarryStatusWorking, yesterdayStr).Find(&orders)
	util.Notice(fmt.Sprintf(`grid pos absent load orders %d from %s`, len(orders), yesterdayStr))
	for _, order := range orders {
		if order.OrderTime.Before(today) {
			tempOrder := api.QueryOrderById(model.KeyDefault, model.SecretDefault, order.Market, order.Symbol,
				order.Instrument, order.OrderType, order.OrderId)
			if tempOrder != nil {
				order = tempOrder
				if order.OrderSide == model.OrderSideBuy {
					setting.GridAmount += math.Abs(order.DealAmount)
				} else {
					setting.GridAmount -= math.Abs(order.DealAmount)
				}
			}
			if order.Status == model.CarryStatusWorking {
				util.Notice(fmt.Sprintf(`cancel old grid order %s at %s`, order.OrderId, order.OrderTime))
				api.MustCancel(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, ``,
					order.OrderType, order.OrderId, true)
			}
			continue
		}
		if order.GridPos == -1 {
			gridPos.orderLiquidate = order
		} else if ((order.OrderSide == model.OrderSideSell && order.GridPos > setting.Chance) ||
			(order.OrderSide == model.OrderSideBuy && order.GridPos < setting.Chance)) &&
			(gridPos.orders[order.GridPos] == nil || gridPos.orders[order.GridPos].OrderTime.Before(order.OrderTime)) {
			gridPos.orders[order.GridPos] = order
			util.Notice(fmt.Sprintf(`load orders[%d] add %s %s %s`,
				len(orders), order.OrderSide, order.OrderId, order.OrderTime.String()))
		}
	}
	for _, order := range gridPos.orders {
		if order != nil {
			return
		}
	}
	setting.Chance = int64(gridPos.posMiddle)
	model.AppDB.Save(setting)
	amount := 0.0
	orderSide := model.OrderSideSell
	for i := 0; i < len(gridPos.pos); i++ {
		if i < gridPos.posMiddle {
			orderSide = model.OrderSideBuy
			amount = gridPos.amount
		} else if i == gridPos.posMiddle {
			amount = math.Min(gridPos.amount, math.Abs(setting.GridAmount)-gridPos.amount)
			if setting.GridAmount > 0 {
				orderSide = model.OrderSideSell
			} else if setting.GridAmount < 0 {
				orderSide = model.OrderSideBuy
			}
		} else if i > gridPos.posMiddle {
			orderSide = model.OrderSideSell
			amount = gridPos.amount
		}
		if amount > 0 {
			order := api.PlaceOrder(model.KeyDefault, model.SecretDefault, orderSide, model.OrderTypeLimit,
				setting.Market, setting.Symbol, ``, ``, ``, ``,
				model.FunctionGrid, gridPos.pos[i], gridPos.pos[i], amount, false)
			if i != gridPos.posMiddle {
				order.GridPos = int64(i)
				dayGridPos[yesterdayStr][setting.Market][setting.Symbol].orders[i] = order
			} else {
				order.GridPos = -1
				gridPos.orderLiquidate = order
			}
			util.Notice(fmt.Sprintf(`init grid %s %s %s at %d index %d pos %s %s %f %f`,
				setting.Market, setting.Symbol, orderSide, i, order.GridPos, order.OrderId, order.Status, order.Price, order.Amount))
			model.AppDB.Save(order)
		}
	}
	return gridPos
}

// setting: grid_amount持仓量, chance 当前position
var ProcessSimpleGrid = func(setting *model.Setting) {
	result, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	now := util.GetNowUnixMillion()
	if !result || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		model.AppPause || now-int64(tick.Ts) > 1000 {
		return
	}
	if setting == nil || getSimpleGriding() {
		return
	}
	setSimpleGriding(true)
	defer setSimpleGriding(false)
	gridPos := getGridPos(setting)
	showMsg := ``
	duration, _ := time.ParseDuration(`-180s`)
	checkTime := util.GetNow().Add(duration)
	checkOrder := false
	if checkTime.After(gridCheckTime) {
		checkOrder = true
		gridCheckTime = util.GetNow()
	}
	if setting.Chance-1 >= 0 {
		i := setting.Chance - 1
		order := gridPos.orders[i]
		if checkOrder && order != nil {
			tempOrder := api.QueryOrderById(model.KeyDefault, model.SecretDefault, order.Market, order.Symbol, order.Instrument,
				order.OrderType, order.OrderId)
			if tempOrder != nil {
				order.Status = tempOrder.Status
				showMsg += fmt.Sprintf("%s %d %s %d %f %s %s %s %f\n",
					order.Status, i, order.OrderSide, order.GridPos, order.Price, order.Market, order.Symbol, order.OrderId, order.Amount)
			}
		}
		if order != nil && (order.Price > tick.Bids[0].Price || order.Status == model.CarryStatusSuccess) {
			orderR := api.PlaceOrder(model.KeyDefault, model.SecretDefault, model.OrderSideSell, model.OrderTypeLimit,
				setting.Market, setting.Symbol, ``, ``, ``, ``,
				model.FunctionGrid, gridPos.pos[setting.Chance], gridPos.pos[setting.Chance], gridPos.amount, false)
			orderR.GridPos = setting.Chance
			gridPos.orders[setting.Chance] = orderR
			setting.Chance = i
			setting.PriceX = gridPos.orders[i].Price
			setting.GridAmount += order.Amount
			order.DealAmount = order.Amount
			order.DealPrice = order.Price
			order.Status = model.CarryStatusSuccess
			model.AppDB.Save(orderR)
			model.AppDB.Save(order)
			model.AppDB.Save(setting)
			gridPos.orders[i] = nil
			util.Notice(fmt.Sprintf(`order success %s %s %s %s %s at %d %f with %f, new order %s %s at %d %f`,
				order.Status, order.Market, order.Symbol, order.OrderSide, order.OrderId, i, order.Price, order.Amount,
				orderR.OrderSide, orderR.OrderId, orderR.GridPos, orderR.Amount))
		}
	}
	if setting.Chance+1 < int64(len(gridPos.pos)) {
		i := setting.Chance + 1
		order := gridPos.orders[i]
		if checkOrder && order != nil {
			tempOrder := api.QueryOrderById(model.KeyDefault, model.SecretDefault, order.Market, order.Symbol, order.Instrument,
				order.OrderType, order.OrderId)
			if tempOrder != nil {
				order.Status = tempOrder.Status
				showMsg += fmt.Sprintf("%s %d %s %d %f %s %s %s %f\n",
					order.Status, i, order.OrderSide, order.GridPos, order.Price, order.Market, order.Symbol, order.OrderId, order.Amount)
			}
		}
		if order != nil && (order.Price < tick.Asks[0].Price || order.Status == model.CarryStatusSuccess) {
			util.Notice(fmt.Sprintf(`check sell %d chance: %d order pos: %d ask0: %f order price %f`,
				len(gridPos.pos), setting.Chance, order.GridPos, tick.Asks[0].Price, order.Price))
			orderS := api.PlaceOrder(model.KeyDefault, model.SecretDefault, model.OrderSideBuy, model.OrderTypeLimit,
				setting.Market, setting.Symbol, ``, ``, ``, ``,
				model.FunctionGrid, gridPos.pos[setting.Chance], gridPos.pos[setting.Chance], gridPos.amount, false)
			orderS.GridPos = setting.Chance
			gridPos.orders[setting.Chance] = orderS
			setting.Chance = i
			setting.PriceX = gridPos.orders[i].Price
			setting.GridAmount -= order.Amount
			order.DealAmount = order.Amount
			order.DealPrice = order.Price
			order.Status = model.CarryStatusSuccess
			model.AppDB.Save(orderS)
			model.AppDB.Save(order)
			model.AppDB.Save(setting)
			gridPos.orders[i] = nil
			util.Notice(fmt.Sprintf(`order success %s %s %s %s %s at %d %f with %f, new order %s %s at %d %f`,
				order.Status, order.Market, order.Symbol, order.OrderSide, order.OrderId, i, order.Price, order.Amount,
				orderS.OrderSide, orderS.OrderId, orderS.GridPos, orderS.Amount))
		}
	}
	if gridPos.orderLiquidate != nil {
		showMsg += fmt.Sprintf("liquidate %s %d %f %s %s %s %f\n",
			gridPos.orderLiquidate.OrderSide, gridPos.orderLiquidate.GridPos, gridPos.orderLiquidate.Price,
			gridPos.orderLiquidate.Market, gridPos.orderLiquidate.Symbol, gridPos.orderLiquidate.OrderId,
			gridPos.orderLiquidate.Amount)
		dealAmount := 0.0
		if gridPos.orderLiquidate.OrderSide == model.OrderSideBuy && tick.Bids[0].Price < gridPos.orderLiquidate.Price {
			dealAmount = gridPos.orderLiquidate.Amount
		}
		if gridPos.orderLiquidate.OrderSide == model.OrderSideSell && tick.Asks[0].Price > gridPos.orderLiquidate.Price {
			dealAmount -= gridPos.orderLiquidate.Amount
		}
		if dealAmount != 0.0 {
			setting.PriceX = gridPos.orderLiquidate.Price
			setting.GridAmount += dealAmount
			gridPos.orderLiquidate.DealAmount = gridPos.orderLiquidate.Amount
			gridPos.orderLiquidate.DealPrice = gridPos.orderLiquidate.Price
			gridPos.orderLiquidate.Status = model.CarryStatusSuccess
			model.AppDB.Save(gridPos.orderLiquidate)
			model.AppDB.Save(setting)
			util.Notice(fmt.Sprintf(`liquidation order success %s %s %s %s %f %f setting amount %f at %d`,
				setting.Market, setting.Symbol, gridPos.orderLiquidate.OrderSide, gridPos.orderLiquidate.OrderId,
				gridPos.orderLiquidate.Amount, gridPos.orderLiquidate.Price, setting.GridAmount, setting.Chance))
			gridPos.orderLiquidate = nil
		}
	}
	model.SetCarryInfo(fmt.Sprintf(`[Grid]%s_%s_%s`, setting.Market, setting.Symbol, model.FunctionGrid),
		fmt.Sprintf(" chance:%d last price:%f holding:%f n值：%f\n%s",
			setting.Chance, setting.PriceX, setting.GridAmount, gridPos.n, showMsg))
}

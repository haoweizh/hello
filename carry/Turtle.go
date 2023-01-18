package carry

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

type TurtleData struct {
	// useNear是否在海龟交易时使用lowDaysNear和highDaysNear和priceX作为触发条件
	useNear, waitBreakLong, waitBreakShort, breakLong, breakShort, liquidated bool
	turtleTime, checkTimeBreak, checkTimeOpen                                 time.Time
	highDaysNear, lowDaysNear, highDaysFar, lowDaysFar, lowAdjust, highAdjust float64
	highToday, lowToday, n, amount                                            float64
	daysNear, daysFar, daysAdjust                                             int
	symbol                                                                    string
	// 适应某些交易所单笔订单不能过大，大笔订单会拆分后下成多个
	orderLong, orderShort, orderAdjust []*model.Order
}

const turtleTriggerDelta = 0.01

var turtling = false
var turtleLock sync.Mutex
var turtleTime sync.Map // market_symbol_day time

func (turtleData *TurtleData) ToString() (str string) {
	if turtleData == nil {
		return `turtle data is nil`
	}
	return fmt.Sprintf(`%d日%f~%f n:%f amount:%f`,
		turtleData.daysFar, turtleData.lowDaysFar, turtleData.highDaysFar, turtleData.n, turtleData.amount)
}

func checkSetTurtling(value bool) (before bool) {
	turtleLock.Lock()
	defer turtleLock.Unlock()
	before = turtling
	if value == false || before == false {
		turtling = value
	}
	return before
}

var dataSet = make(map[string]map[string]map[string]*TurtleData) // market - symbol - 2019-12-06 - *turtleData

func calcTurtleAmount(key, secret string, setting *model.Setting, n float64) (amount float64) {
	var accountValue float64
	switch setting.Market {
	case model.BinancePerp:
		_, _, accountValue, _ = api.GetPositions(key, secret, setting.Market)
	case model.Ftx, model.OKEX:
		_, _, accountValue, _ = api.GetBalances(key, secret, setting.Market)
	}
	amount = 0.02 * accountValue / n
	_, _, coin, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	if model.CommonCoins[strings.ToLower(coin)] {
		amount = amount * 0.75
	} else {
		amount /= 4
	}
	//util.Notice(`calcTurtleAmount %s %s %f`, setting.Market, setting.Symbol, amount)
	return amount
}

func clearOrders(key, secret string, setting *model.Setting) {
	ordersLimit := api.QueryOpenOrders(key, secret, setting.Market, setting.Symbol, false)
	ordersStop := api.QueryOpenOrders(key, secret, setting.Market, setting.Symbol, true)
	for _, order := range ordersLimit {
		if order != nil {
			util.Notice(`cancel pending turtle limit order %s %s %s`,
				setting.Market, setting.Symbol, order.OrderId)
			api.MustCancel(key, secret, setting.Market, setting.Symbol, model.OrderTypeLimit, order.OrderId, true)
		}
	}
	for _, order := range ordersStop {
		if order != nil {
			util.Notice(`cancel pending turtle stop order %s %s %s`,
				setting.Market, setting.Symbol, order.OrderId)
			api.MustCancel(key, secret, setting.Market, setting.Symbol, model.OrderTypeStop, order.OrderId, true)
		}
	}
}

func adjustPosHolding(key, secret string, setting *model.Setting, turtleData *TurtleData) {
	success, marketPos, _, _ := api.GetPositions(key, secret, setting.Market)
	if !success {
		util.Notice(fmt.Sprintf(`fail to adjust position holdings %s %s`, setting.Market, setting.Symbol))
		return
	}
	posMap := make(map[string]*model.Position)
	for _, pos := range marketPos {
		posMap[strings.ToUpper(pos.Currency)] = pos
	}
	if posMap[setting.Symbol] != nil { //setting.Chance和pos.Holding相乘小于零代表方向相反，此时设置为0
		if float64(setting.Chance)*posMap[setting.Symbol].Holding <= 0 {
			util.Notice(`update turtle side %s %s %d %f %f`,
				setting.Market, setting.Symbol, setting.Chance, posMap[setting.Symbol].Holding, setting.GridAmount)
			setting.GridAmount = 0
			setting.Chance = 0
			if posMap[setting.Symbol].Holding > 0 {
				turtleData.orderAdjust = api.MustPlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, turtleData.lowAdjust*(1-turtleTriggerDelta), turtleData.lowAdjust, posMap[setting.Symbol].Holding, setting)
			} else if posMap[setting.Symbol].Holding < 0 {
				turtleData.orderAdjust = api.MustPlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, turtleData.highAdjust*(1+turtleTriggerDelta), turtleData.highAdjust, -1*posMap[setting.Symbol].Holding, setting)
			}
			for _, order := range turtleData.orderAdjust {
				if order != nil {
					order.RefreshType = model.FunctionTurtleAdjust
					model.AppDB.Save(order)
				}
			}
		} else if setting.GridAmount != math.Abs(posMap[setting.Symbol].Holding) {
			util.Notice(`update turtle grid amount %s %s %f to %f`,
				setting.Market, setting.Symbol, setting.GridAmount, posMap[setting.Symbol].Holding)
			setting.GridAmount = math.Abs(posMap[setting.Symbol].Holding)
		}
	} else {
		setting.GridAmount = 0
		setting.Chance = 0
		util.Notice(`update turtle when absent %s %s %d`, setting.Market, setting.Symbol, len(posMap))
		for s, position := range posMap {
			util.Notice(`present %s %s %f`, s, position.Currency, position.Holding)
		}
	}
	model.AppDB.Save(setting)
}

func GetTurtleData(key, secret string, setting *model.Setting) (turtleData *TurtleData) {
	today, todayStr := model.GetMarketToday(setting.Market)
	if dataSet[setting.Market] == nil {
		dataSet[setting.Market] = make(map[string]map[string]*TurtleData)
	}
	if dataSet[setting.Market][setting.Symbol] == nil {
		dataSet[setting.Market][setting.Symbol] = make(map[string]*TurtleData)
	}
	//util.Notice(`need to create turtle ` + setting.Market + setting.Symbol)
	duration, _ := time.ParseDuration(`-120s`)
	value, ok := turtleTime.Load(fmt.Sprintf(`%s_%s_%s`, setting.Market, setting.Symbol, todayStr))
	if (dataSet[setting.Market][setting.Symbol][todayStr] != nil) || (ok && value != nil && time.Now().Add(duration).Before(value.(time.Time))) {
		return dataSet[setting.Market][setting.Symbol][todayStr]
	}
	clearOrders(key, secret, setting)
	turtleTime.Store(fmt.Sprintf(`%s_%s_%s`, setting.Market, setting.Symbol, todayStr), time.Now())
	_, _, coin, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	far := 20
	near := 10
	if !model.CommonCoins[strings.ToLower(coin)] {
		far = 18
		near = 9
	}
	turtleData = &TurtleData{turtleTime: today, symbol: setting.Symbol, checkTimeBreak: util.GetNow(),
		checkTimeOpen: util.GetNow().Add(duration), waitBreakLong: false, waitBreakShort: false, breakLong: false,
		breakShort: false, liquidated: false, daysFar: far, daysNear: near, daysAdjust: 5, useNear: false}
	for i := 1; i < 21; i++ {
		duration, _ = time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		day := today.Add(duration)
		candle := api.GetTurtleCandle(key, secret, setting.Market, setting.Symbol, 86400, day)
		if candle == nil {
			if time.Now().Second() == 0 {
				util.Notice(`can not calc turtleDate as nil candle %s %s %s %s`,
					setting.Market, setting.Symbol, turtleData.symbol, day.String())
			}
			return nil
		}
		if candle.PriceHigh > turtleData.highDaysFar && i <= far {
			turtleData.highDaysFar = candle.PriceHigh
		}
		if (turtleData.lowDaysFar == 0 || turtleData.lowDaysFar > candle.PriceLow) && i <= far {
			turtleData.lowDaysFar = candle.PriceLow
		}
		if candle.PriceHigh > turtleData.highDaysNear && i <= near {
			turtleData.highDaysNear = candle.PriceHigh
		}
		if (turtleData.lowDaysNear == 0 || turtleData.lowDaysNear > candle.PriceLow) && i <= near {
			turtleData.lowDaysNear = candle.PriceLow
		}
		if candle.PriceHigh > turtleData.highAdjust && i <= turtleData.daysAdjust {
			turtleData.highAdjust = candle.PriceHigh
		}
		if (turtleData.lowAdjust == 0 || turtleData.lowAdjust > candle.PriceLow) && i <= turtleData.daysAdjust {
			turtleData.lowAdjust = candle.PriceLow
		}
		if i == 1 {
			turtleData.n = candle.N
			turtleData.amount = calcTurtleAmount(key, secret, setting, turtleData.n)
		}
	}
	if turtleData.amount > 0 && turtleData.n > 0 {
		dataSet[setting.Market][setting.Symbol][todayStr] = turtleData
		adjustPosHolding(key, secret, setting, turtleData)
		util.Notice(fmt.Sprintf(`%s %s set turtle data: amount:%f n:%f %d:%f-%f %d:%f-%f`,
			setting.Market, setting.Symbol, turtleData.amount, turtleData.n, turtleData.daysNear, turtleData.lowDaysNear,
			turtleData.highDaysNear, turtleData.daysFar, turtleData.lowDaysFar, turtleData.highDaysFar))
	}
	return
}

func checkTurtleOrders(key, secret string, setting *model.Setting, currentN float64, turtleData *TurtleData) (checked bool) {
	duration, _ := time.ParseDuration(`-1200s`)
	now := util.GetNow().Add(duration)
	checked = false
	if now.After(turtleData.checkTimeOpen) {
		today, _ := model.GetMarketToday(setting.Market)
		dayTime, _ := time.ParseDuration(`86400s`)
		var candles []*model.Candle
		// okex不返回尚未结束的当日candle，转成半小时的slot
		if setting.Market == model.OKEX {
			candles = api.GetCandle(key, secret, setting.Market, setting.Symbol, 1800, today, model.GetMarketNow(setting.Market))
		} else {
			candles = api.GetCandle(key, secret, setting.Market, setting.Symbol, 86400, today, today.Add(dayTime))
		}
		for i := 0; candles != nil && i < len(candles); i++ {
			if turtleData.highToday < candles[i].PriceHigh {
				turtleData.highToday = candles[i].PriceHigh
			}
			if turtleData.lowToday == 0 || turtleData.lowToday > candles[i].PriceLow {
				turtleData.lowToday = candles[i].PriceLow
			}
			util.Info(fmt.Sprintf(`get today len %s %s %d %f %f`,
				setting.Market, setting.Symbol, len(candles), candles[0].PriceLow, candles[0].PriceHigh))
		}
		checked = true
		turtleData.checkTimeOpen = util.GetNow()
		orders := api.QueryOpenOrders(key, secret, setting.Market, setting.Symbol, true)
		if orders == nil {
			return
		}
		for _, order := range orders {
			needCancel := true
			if turtleData.orderLong != nil {
				for _, long := range turtleData.orderLong {
					if order.OrderId == long.OrderId && (currentN < setting.AmountLimit || setting.Chance < 0) {
						needCancel = false
					}
				}
			}
			if turtleData.orderShort != nil {
				for _, short := range turtleData.orderShort {
					if order.OrderId == short.OrderId && (currentN > -1*setting.AmountLimit || setting.Chance > 0) {
						needCancel = false
					}
				}
			}
			if turtleData.orderAdjust != nil {
				for _, adjust := range turtleData.orderAdjust {
					if adjust != nil && order.OrderId == adjust.OrderId {
						needCancel = false
					}
				}
			}
			if needCancel {
				result := api.MustCancel(key, secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, true)
				util.Notice(`cancel extra turtle order %s %s %s %s return %v`,
					setting.Market, setting.Symbol, order.OrderType, order.OrderId, result)
				time.Sleep(time.Second)
			}
		}
	}
	return
}

// ProcessTurtle
// setting.GridAmount 当前已经持仓数量
// setting.Chance 当前开仓的个数
// setting.PriceX 上一次开仓的价格
// setting.OpenShortMargin 该单币种最多开仓个数
// setting.AmountLimit 总开仓上限
var ProcessTurtle = func(setting *model.Setting, tick *model.BidAsk) {
	if !checkSetTurtling(true) {
		defer checkSetTurtling(false)
	} else {
		return
	}
	now := util.GetNowUnixMillion()
	maintaining, ok := model.ChannelMaintaining.Load(setting.Market)
	if setting == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		(ok && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 1000) ||
		(time.Now().Hour() == 0 && time.Now().Minute() == 0) {
		return
	}
	if setting.Chance != 0 && setting.PriceX == 0 {
		util.Notice(fmt.Sprintf(`no last priceX %s %s %d %f`,
			setting.Market, setting.Symbol, setting.Chance, setting.PriceX))
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	turtleData := GetTurtleData(account.Key, account.Secret, setting)
	if turtleData == nil || turtleData.n == 0 || turtleData.amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle %s %s`, setting.Market, setting.Symbol))
		}
		return
	}
	currentN := api.GetCurrentN(setting)
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionTurtle, setting.Market, setting.Symbol)
	msg := fmt.Sprintf("[海龟参数]%s %s 次数限制:%f 当前已经持仓数量:%f 上一次开仓的价格:%f "+
		"%d日:%f-%f %d日:%f-%f n:%f 数量:%f %s 持仓数/限制:%d/%f 总持仓数%d bid-ask %f %f 当日有平仓：%v",
		turtleData.turtleTime.String()[0:10], msgKey, setting.AmountLimit, setting.GridAmount, setting.PriceX,
		turtleData.daysFar, turtleData.lowDaysFar, turtleData.highDaysFar, turtleData.daysNear, turtleData.lowDaysNear,
		turtleData.highDaysNear, turtleData.n, turtleData.amount, setting.Symbol, setting.Chance,
		setting.OpenShortMargin, currentN, tick.Bids[0].Price, tick.Asks[0].Price, turtleData.liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	priceLong := turtleData.highDaysFar
	priceShort := turtleData.lowDaysFar
	if checkTurtleOrders(account.Key, account.Secret, setting, float64(currentN), turtleData) ||
		checkTurtleBreak(account.Key, account.Secret, setting, turtleData, tick) {
		return
	}
	if setting.Chance == 0 && !turtleData.liquidated { // 开初始仓
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, currentN, priceShort, priceLong, tick)
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			setting.Chance = 1
			setting.GridAmount = turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破%d日高点 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				turtleData.daysFar, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN,
				priceShort, priceLong, setting.PriceX, turtleData.n))
		}
		if turtleData.breakShort && turtleData.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideSell)
			setting.Chance = -1
			setting.GridAmount = turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破%d日低点 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				turtleData.daysNear, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN,
				priceShort, priceLong, setting.PriceX, turtleData.n))
		}
	} else if setting.Chance > 0 {
		priceLong = math.Max(priceLong, setting.PriceX+turtleData.n/2)
		if turtleData.useNear {
			priceShort = math.Max(setting.PriceX-2*turtleData.n, turtleData.lowDaysNear)
		} else {
			if turtleData.highDaysFar < turtleData.highToday {
				if turtleData.orderShort != nil {
					for _, order := range turtleData.orderShort {
						go api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, true)
					}
				}
				turtleData.orderShort = nil
			}
			priceShort = math.Max(turtleData.highDaysFar, turtleData.highToday) - 2*turtleData.n
		}
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, currentN, priceShort, priceLong, tick)
		// 加仓一个单位
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			setting.Chance = setting.Chance + 1
			setting.GridAmount = setting.GridAmount + turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加多 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort, priceLong,
				setting.PriceX, turtleData.n))
		}
		// 平多
		if turtleData.breakShort && turtleData.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideSell)
			go api.SendMails(`平多`+setting.Market+setting.Symbol,
				fmt.Sprintf(`止盈止损at%f 仓位%d 数量 %f`, priceShort, setting.Chance, setting.GridAmount))
			turtleData.liquidated = true
			setting.Chance = 0
			setting.GridAmount = 0
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`liquidate long %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort, priceLong,
				setting.PriceX, turtleData.n))
		}
	} else if setting.Chance < 0 {
		priceShort = math.Min(priceShort, setting.PriceX-turtleData.n/2)
		if turtleData.useNear {
			priceLong = math.Min(setting.PriceX+2*turtleData.n, turtleData.highDaysNear)
		} else {
			if turtleData.lowToday > 0 && turtleData.lowToday < turtleData.lowDaysFar {
				if turtleData.orderLong != nil {
					for _, order := range turtleData.orderLong {
						go api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, true)
					}
				}
				turtleData.orderLong = nil
			}
			if turtleData.lowToday > 0 {
				priceLong = math.Min(turtleData.lowDaysFar, turtleData.lowToday) + 2*turtleData.n
			} else {
				priceLong = turtleData.lowDaysFar + 2*turtleData.n
			}
		}
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, currentN, priceShort, priceLong, tick)
		// 加仓一个单位
		if turtleData.breakShort && turtleData.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideSell)
			setting.Chance = setting.Chance - 1
			setting.GridAmount = setting.GridAmount + turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加空 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort, priceLong,
				setting.PriceX, turtleData.n))
		} // liquidate short
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			go api.SendMails(`平空`+setting.Market+setting.Symbol,
				fmt.Sprintf(`止盈止损at%f 仓位%d 数量 %f`,
					priceLong, setting.Chance, setting.GridAmount))
			setting.Chance = 0
			setting.GridAmount = 0
			turtleData.liquidated = true
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`liquidate short result: %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort, priceLong,
				setting.PriceX, turtleData.n))
		}
	}
}

func setTurtleOrderStatus(function, market, symbol, orderId, status string) {
	setting := api.GetSetting(function, market, symbol)
	if setting == nil {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	if account == nil {
		return
	}
	turtleData := GetTurtleData(account.Key, account.Secret, setting)
	if turtleData != nil && turtleData.orderLong != nil {
		for _, order := range turtleData.orderLong {
			if order.OrderId == orderId {
				order.Status = status
			}
		}
	}
	if turtleData != nil && turtleData.orderShort != nil {
		for _, order := range turtleData.orderShort {
			if order.OrderId == orderId {
				order.Status = status
			}
		}
	}
}

func checkTurtleBreak(key, secret string, setting *model.Setting, turtleData *TurtleData, tick *model.BidAsk) (checked bool) {
	duration, _ := time.ParseDuration(`-60s`)
	now := util.GetNow().Add(duration)
	checked = false
	if now.After(turtleData.checkTimeBreak) {
		turtleData.checkTimeBreak = util.GetNow()
		var orderLong, orderShort *model.Order
		if turtleData.orderLong != nil && len(turtleData.orderLong) > 0 {
			orderLong = turtleData.orderLong[0]
		}
		if turtleData.orderShort != nil && len(turtleData.orderShort) > 0 {
			orderShort = turtleData.orderShort[0]
		}
		if turtleData.orderLong != nil && (orderLong.Status == model.CarryStatusSuccess ||
			(orderLong.TriggerPrice > 0 && orderLong.TriggerPrice <= tick.Bids[0].Price)) {
			util.Notice(fmt.Sprintf(`-----chance %s %s %d bid-ask %f %f short %f`,
				setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price, orderLong.TriggerPrice))
			order := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, orderLong.OrderType, orderLong.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				turtleData.breakLong = true
				util.Notice(fmt.Sprintf(`-----order break long %s %s %d bid-ask %f %f short %f %v %v`,
					setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					orderLong.Price, turtleData.breakLong, turtleData.waitBreakLong))
			}
			checked = true
		}
		if turtleData.orderShort != nil && (orderShort.Status == model.CarryStatusSuccess ||
			(orderShort.TriggerPrice > 0 && orderShort.TriggerPrice >= tick.Asks[0].Price)) {
			util.Notice(fmt.Sprintf(`-----chance %s %s %d bid-ask %f %f long %f`,
				setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price, orderShort.TriggerPrice))
			order := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, orderShort.OrderType, orderShort.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				turtleData.breakShort = true
				util.Notice(fmt.Sprintf(`-----order break short %s %s %d bid-ask %f %f long %f %v %v`,
					setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					orderShort.Price, turtleData.breakShort, turtleData.waitBreakShort))
			}
			checked = true
		}
	}
	return checked
}

func handleBreak(key, secret string, setting *model.Setting, turtleData *TurtleData, orderSide string) {
	if turtleData == nil {
		//util.Notice(fmt.Sprintf(`fatal error, nil order to break`))
		return
	}
	turtleData.waitBreakLong = false
	turtleData.waitBreakShort = false
	orderQuery := turtleData.orderShort
	orderCancel := turtleData.orderLong
	if orderSide == model.OrderSideBuy {
		orderQuery = turtleData.orderLong
		orderCancel = turtleData.orderShort
	}
	if orderQuery != nil && len(orderQuery) > 0 {
		time.Sleep(time.Second * 3)
		util.Notice(fmt.Sprintf(`query turtle break %s %s %s %d`,
			setting.Market, setting.Symbol, orderSide, len(orderQuery)))
		setting.PriceX = orderQuery[0].TriggerPrice
		turtleData.orderLong = nil
		turtleData.orderShort = nil
		if orderCancel != nil {
			for _, order := range orderCancel {
				temp := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId)
				if temp != nil && temp.Status == model.CarryStatusWorking {
					go api.MustCancel(key, secret, order.Market, order.Symbol, order.OrderType, order.OrderId, true)
				}
			}
		}
		util.Notice(fmt.Sprintf(`clear %s %s opp-%s %v`, setting.Market, setting.Symbol, orderSide, orderCancel))
	}
}

func placeTurtleOrders(key, secret string, turtleData *TurtleData, setting *model.Setting,
	currentN int64, priceShort, priceLong float64, tick *model.BidAsk) {
	amountLimit := int64(setting.AmountLimit)
	coinLimit := int64(setting.OpenShortMargin)
	if turtleData.orderLong == nil && ((currentN < amountLimit && setting.Chance < coinLimit) || setting.Chance < 0) {
		orderSide := model.OrderSideBuy
		typeLong := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance < 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平空 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, currentN, priceShort,
				priceLong, setting.PriceX, turtleData.n))
		}
		if setting.SymbolRelated != model.SettingTurtleRemoved || setting.Chance < 0 {
			util.Notice(fmt.Sprintf(`%s %s place多单 chance:%d amount:%f priceX:%f currentN-limit:%d %f
			orderSide:%s h%d:%f h%d:%f l%d:%f h%d:%f coin limit:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, setting.PriceX, currentN, setting.AmountLimit,
				orderSide, turtleData.daysFar, turtleData.highDaysFar, turtleData.daysNear, turtleData.highDaysNear,
				turtleData.daysFar, turtleData.lowDaysFar, turtleData.daysNear, turtleData.lowDaysNear, setting.OpenShortMargin))
			priceOut := false
			if priceLong <= tick.Asks[0].Price {
				turtleData.orderLong = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, priceLong*(1+turtleTriggerDelta), priceLong, amount, setting)
				priceOut = true
			} else {
				turtleData.orderLong = api.MustPlaceOrder(key, secret, orderSide, typeLong, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, priceLong*(1+turtleTriggerDelta), priceLong, amount, setting)
			}
			if turtleData.orderLong != nil {
				turtleData.waitBreakLong = true
				turtleData.breakLong = false
				if priceOut {
					turtleData.breakLong = true
				}
				for _, order := range turtleData.orderLong {
					order.LineBuy = turtleData.n
					go model.AppDB.Save(order)
				}
			}
		}
	}
	if turtleData.orderShort == nil && ((currentN > -1*amountLimit && setting.Chance > -1*coinLimit) || setting.Chance > 0) {
		orderSide := model.OrderSideSell
		typeShort := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance > 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平多 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, currentN, priceShort,
				priceLong, setting.PriceX, turtleData.n))
		}
		if setting.SymbolRelated != model.SettingTurtleRemoved || setting.Chance > 0 {
			util.Notice(fmt.Sprintf(`%s %s place空单 chance:%d amount:%f priceX:%f currentN-limit:%d %f 
			orderSide:%s h%d:%f h%d:%f l%d:%f l%d:%f coin limit:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, setting.PriceX, currentN, setting.AmountLimit,
				orderSide, turtleData.daysFar, turtleData.highDaysFar, turtleData.daysNear, turtleData.highDaysNear,
				turtleData.daysFar, turtleData.lowDaysFar, turtleData.daysNear, turtleData.lowDaysNear, setting.OpenShortMargin))
			priceOut := false
			if priceShort >= tick.Bids[0].Price {
				turtleData.orderShort = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, priceShort*(1-turtleTriggerDelta), priceShort, amount, setting)
				priceOut = true
			} else {
				turtleData.orderShort = api.MustPlaceOrder(key, secret, orderSide, typeShort, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, priceShort*(1-turtleTriggerDelta), priceShort, amount, setting)
			}
			if turtleData.orderShort != nil {
				turtleData.waitBreakShort = true
				turtleData.breakShort = false
				if priceOut {
					turtleData.breakShort = true
				}
				for _, order := range turtleData.orderShort {
					order.LineBuy = turtleData.n
					go model.AppDB.Save(order)
				}
			}
		}
	}
}

package Turtle

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"time"
)

func checkTurtleOrders(key, secret string, setting *model.Setting, currentNum float64, turtleData *Data) (checked bool) {
	if turtleData.checkTimeOpen.Add(time.Minute * 20).After(util.GetNow()) {
		return false
	}
	if !turtleData.adjustChecked {
		adjustPosHolding(key, secret, setting, turtleData)
	}
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
	if !turtleData.useNear && turtleData.orderShort != nil && len(turtleData.orderShort) > 0 && setting.Chance > 0 &&
		turtleData.orderShort[0].OrderType == model.OrderTypeStop && turtleData.orderShort[0].TriggerPrice*(1+turtleTriggerDelta) <
		math.Max(turtleData.highToday, turtleData.highDaysFar)-2*turtleData.n {
		util.Notice(fmt.Sprintf(`today higher than far trigger%f<max(today%f,far%f)-2*%f chance%d`,
			turtleData.orderShort[0].TriggerPrice, turtleData.highToday, turtleData.highDaysFar, turtleData.n, setting.Chance))
		for _, order := range turtleData.orderShort {
			go api.MustCancel(key, secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, true)
		}
		turtleData.orderShort = nil
	}
	if !turtleData.useNear && turtleData.orderLong != nil && len(turtleData.orderLong) > 0 && turtleData.lowToday > 0 &&
		setting.Chance < 0 && turtleData.orderLong[0].OrderType == model.OrderTypeStop &&
		turtleData.orderLong[0].TriggerPrice*(1-turtleTriggerDelta) > math.Min(turtleData.lowToday, turtleData.lowDaysFar)+2*turtleData.n {
		util.Notice(fmt.Sprintf(`today lower than far trigger%f>min(today%f,far%f)+2*%f chance%d`,
			turtleData.orderLong[0].TriggerPrice, turtleData.lowToday, turtleData.lowDaysFar, turtleData.n, setting.Chance))
		for _, order := range turtleData.orderLong {
			go api.MustCancel(key, secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, true)
		}
		turtleData.orderLong = nil
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
				if order.OrderId == long.OrderId && (currentNum < setting.AmountLimit || setting.Chance < 0) {
					needCancel = false
				}
			}
		}
		if turtleData.orderShort != nil {
			for _, short := range turtleData.orderShort {
				if order.OrderId == short.OrderId && (currentNum > -1*setting.AmountLimit || setting.Chance > 0) {
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
	turtleData := GetTurtleData(account.Key, account.Secret, setting.Function, setting.Market, setting.Symbol, true)
	if turtleData == nil || turtleData.n == 0 || turtleData.amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle %s %s`, setting.Market, setting.Symbol))
		}
		return
	}
	chanceInAll := api.GetChanceInAll(setting)
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionTurtle, setting.Market, setting.Symbol)
	msg := fmt.Sprintf("[海龟参数]%s %s 次数限制:%f 当前已经持仓数量:%f 上一次开仓的价格:%f "+
		"%d日:%f-%f %d日:%f-%f n:%f 数量:%f %s 持仓数/限制:%d/%f 总持仓数%d bid-ask %f %f 当日有平仓：%v",
		turtleData.turtleTime.String()[0:10], msgKey, setting.AmountLimit, setting.GridAmount, setting.PriceX,
		turtleData.daysFar, turtleData.lowDaysFar, turtleData.highDaysFar, turtleData.daysNear, turtleData.lowDaysNear,
		turtleData.highDaysNear, turtleData.n, turtleData.amount, setting.Symbol, setting.Chance,
		setting.OpenShortMargin, chanceInAll, tick.Bids[0].Price, tick.Asks[0].Price, turtleData.liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	priceLong := turtleData.highDaysFar
	priceShort := turtleData.lowDaysFar
	if checkTurtleOrders(account.Key, account.Secret, setting, float64(chanceInAll), turtleData) ||
		checkTurtleBreak(account.Key, account.Secret, setting.Market, setting.Symbol, setting, turtleData, tick) {
		return
	}
	if setting.Chance == 0 && !turtleData.liquidated { // 开初始仓
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, chanceInAll, priceShort, priceLong, tick)
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			setting.Chance = 1
			setting.GridAmount = turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破%d日高点 %s %s chance:%d amount:%f chanceInAll:%d short-long:%f %f px:%f n:%f`,
				turtleData.daysFar, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll,
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
				`破%d日低点 %s %s chance:%d amount:%f chanceInAll:%d short-long:%f %f px:%f n:%f`,
				turtleData.daysNear, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll,
				priceShort, priceLong, setting.PriceX, turtleData.n))
		}
	} else if setting.Chance > 0 {
		priceLong = math.Max(priceLong, setting.PriceX+turtleData.n/2)
		if turtleData.useNear {
			priceShort = math.Max(setting.PriceX-2*turtleData.n, turtleData.lowDaysNear)
		} else {
			priceShort = math.Max(turtleData.highDaysFar, turtleData.highToday) - 2*turtleData.n
		}
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, chanceInAll, priceShort, priceLong, tick)
		// 加仓一个单位
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			setting.Chance = setting.Chance + 1
			setting.GridAmount = setting.GridAmount + turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加多 %s %s chance:%d amount:%f chanceInAll:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll, priceShort, priceLong,
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
			util.Notice(fmt.Sprintf(`liquidate long %s %s chance:%d amount:%f chanceInAll:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll, priceShort, priceLong,
				setting.PriceX, turtleData.n))
		}
	} else if setting.Chance < 0 {
		priceShort = math.Min(priceShort, setting.PriceX-turtleData.n/2)
		if turtleData.useNear {
			priceLong = math.Min(setting.PriceX+2*turtleData.n, turtleData.highDaysNear)
		} else {
			if turtleData.lowToday > 0 {
				priceLong = math.Min(turtleData.lowDaysFar, turtleData.lowToday) + 2*turtleData.n
			} else {
				priceLong = turtleData.lowDaysFar + 2*turtleData.n
			}
		}
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, chanceInAll, priceShort, priceLong, tick)
		// 加仓一个单位
		if turtleData.breakShort && turtleData.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideSell)
			setting.Chance = setting.Chance - 1
			setting.GridAmount = setting.GridAmount + turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加空 %s %s chance:%d amount:%f chanceInAll:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll, priceShort, priceLong,
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
			util.Notice(fmt.Sprintf(`liquidate short result: %s %s chance:%d amount:%f chanceInAll:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll, priceShort, priceLong,
				setting.PriceX, turtleData.n))
		}
	}
}

func SetTurtleOrderStatus(function, market, symbol, orderId, status string) {
	setting := api.GetSetting(function, market, symbol)
	if setting == nil {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	if account == nil {
		return
	}
	_, todayStr := model.GetMarketToday(market)
	value, ok := util.LoadSyncMap(turtleDataSet, function, market, symbol, todayStr)
	if ok && value != nil {
		turtleData := value.(*Data)
		if turtleData.orderLong != nil {
			for _, order := range turtleData.orderLong {
				if order.OrderId == orderId {
					order.Status = status
				}
			}
		}
		if turtleData.orderShort != nil {
			for _, order := range turtleData.orderShort {
				if order.OrderId == orderId {
					order.Status = status
				}
			}
		}
	}
}

func checkTurtleBreak(key, secret, market, symbol string, setting *model.Setting, turtleData *Data, tick *model.BidAsk) (checked bool) {
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
		if turtleData.orderLong != nil && (orderLong.Status == model.CarryStatusSuccess || (orderLong.TriggerPrice > 0 &&
			(orderLong.OrderType == model.OrderTypeStop && orderLong.TriggerPrice <= tick.Bids[0].Price) ||
			(orderLong.OrderType == model.OrderTypeLimit && orderLong.Price > tick.Bids[0].Price))) {
			util.Notice(fmt.Sprintf(`-----chance type: %s %s %s %d bid-ask %f %f short %f %f`,
				market, symbol, orderLong.OrderType, setting.Chance, tick.Bids[0].Price,
				tick.Asks[0].Price, orderLong.TriggerPrice, orderLong.Price))
			order := api.QueryOrderById(key, secret, market, symbol, orderLong.OrderType, orderLong.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				turtleData.breakLong = true
				util.Notice(fmt.Sprintf(`-----order break long %s %s %d bid-ask %f %f short %f %v %v`,
					market, symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					orderLong.Price, turtleData.breakLong, turtleData.waitBreakLong))
			}
			if order != nil && order.Status == model.CarryStatusFail {
				turtleData.orderLong = nil
			}
			checked = true
		}
		if turtleData.orderShort != nil && (orderShort.Status == model.CarryStatusSuccess || (orderShort.TriggerPrice > 0 &&
			(orderShort.OrderType == model.OrderTypeStop && orderShort.TriggerPrice >= tick.Asks[0].Price)) ||
			(orderShort.OrderType == model.OrderTypeLimit && orderShort.Price < tick.Asks[0].Price)) {
			util.Notice(fmt.Sprintf(`-----chance type: %s %s %s %d bid-ask %f %f long %f %f`,
				market, symbol, orderShort.OrderType, setting.Chance, tick.Bids[0].Price,
				tick.Asks[0].Price, orderShort.TriggerPrice, orderShort.Price))
			order := api.QueryOrderById(key, secret, market, symbol, orderShort.OrderType, orderShort.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				turtleData.breakShort = true
				util.Notice(fmt.Sprintf(`-----order break short %s %s %d bid-ask %f %f long %f %v %v`,
					market, symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					orderShort.Price, turtleData.breakShort, turtleData.waitBreakShort))
			}
			if order != nil && order.Status == model.CarryStatusFail {
				turtleData.orderShort = nil
			}
			checked = true
		}
	}
	return checked
}

func handleBreak(key, secret string, setting *model.Setting, turtleData *Data, orderSide string) {
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

func placeTurtleOrders(key, secret string, turtleData *Data, setting *model.Setting,
	currentNum int64, priceShort, priceLong float64, tick *model.BidAsk) {
	amountLimit := int64(setting.AmountLimit)
	coinLimit := int64(setting.OpenShortMargin)
	if turtleData.orderLong == nil && ((currentNum < amountLimit && setting.Chance < coinLimit) || setting.Chance < 0) {
		orderSide := model.OrderSideBuy
		typeLong := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance < 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平空 %s %s chance:%d amount:%f currentNum:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, currentNum, priceShort,
				priceLong, setting.PriceX, turtleData.n))
		}
		if setting.SymbolRelated != model.SettingTurtleRemoved || setting.Chance < 0 {
			util.Notice(fmt.Sprintf(`%s %s place多单 chance:%d amount:%f priceX:%f currentNum-limit:%d %f
			orderSide:%s h%d:%f h%d:%f l%d:%f h%d:%f coin limit:%d`,
				setting.Market, setting.Symbol, setting.Chance, amount, setting.PriceX, currentNum, setting.AmountLimit,
				orderSide, turtleData.daysFar, turtleData.highDaysFar, turtleData.daysNear, turtleData.highDaysNear,
				turtleData.daysFar, turtleData.lowDaysFar, turtleData.daysNear, turtleData.lowDaysNear, coinLimit))
			priceOut := false
			if priceLong <= tick.Asks[0].Price {
				turtleData.orderLong = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, priceLong*(1+turtleTriggerDelta/2), priceLong, amount, setting)
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
	if turtleData.orderShort == nil && ((currentNum > -1*amountLimit && setting.Chance > -1*coinLimit) || setting.Chance > 0) {
		orderSide := model.OrderSideSell
		typeShort := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance > 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平多 %s %s chance:%d amount:%f currentNum:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, currentNum, priceShort,
				priceLong, setting.PriceX, turtleData.n))
		}
		if setting.SymbolRelated != model.SettingTurtleRemoved || setting.Chance > 0 {
			util.Notice(fmt.Sprintf(`%s %s place空单 chance:%d amount:%f priceX:%f currentNum-limit:%d %f 
			orderSide:%s h%d:%f h%d:%f l%d:%f l%d:%f coin limit:%d`,
				setting.Market, setting.Symbol, setting.Chance, amount, setting.PriceX, currentNum, setting.AmountLimit,
				orderSide, turtleData.daysFar, turtleData.highDaysFar, turtleData.daysNear, turtleData.highDaysNear,
				turtleData.daysFar, turtleData.lowDaysFar, turtleData.daysNear, turtleData.lowDaysNear, coinLimit))
			priceOut := false
			if priceShort >= tick.Bids[0].Price {
				turtleData.orderShort = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, priceShort*(1-turtleTriggerDelta/2), priceShort, amount, setting)
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

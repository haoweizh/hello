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

var bigOrder *sync.Map

// ProcessCombineTurtle
// setting.GridAmount 当前已经持仓数量
// setting.Chance 当前开仓的个数
// setting.PriceX 上一次开仓的价格
// setting.OpenShortMargin 该单币种最多开仓个数
// setting.AmountLimit 总开仓上限
var ProcessCombineTurtle = func(settingLimit *model.Setting, tick *model.BidAsk) {
	if !checkSetTurtling(true) {
		defer checkSetTurtling(false)
	} else {
		return
	}
	market := settingLimit.Market
	symbol := settingLimit.Symbol
	now := util.GetNowUnixMillion()
	maintaining, ok := model.ChannelMaintaining.Load(market)
	if settingLimit == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		(ok && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 1000) ||
		(time.Now().Hour() == 0 && time.Now().Minute() == 0) {
		return
	}
	settingStop := api.GetInvalidTurtle(market, symbol)
	if settingStop == nil || settingStop.Valid { // 使用valid为false的turtle作为对应turtle,否则该算法不运行
		return
	}
	settings := []*model.Setting{settingLimit, settingStop}
	if (settingLimit.Chance != 0 && settingLimit.PriceX == 0) || (settingStop.Chance != 0 && settingStop.PriceX == 0) {
		util.Notice(fmt.Sprintf(`no last priceX %s %s %d %f %d %f`,
			market, symbol, settingLimit.Chance, settingLimit.PriceX, settingStop.Chance, settingStop.PriceX))
		return
	}
	account := model.AppConfig.GetAccounts(market)[0]
	turtleDataLimit := GetTurtleData(account.Key, account.Secret, settingLimit.Function, market, symbol, false)
	turtleDataStop := GetTurtleData(account.Key, account.Secret, model.FunctionTurtle, market, symbol, true)
	if turtleDataLimit == nil || turtleDataLimit.n == 0 || turtleDataLimit.amount == 0 ||
		turtleDataStop == nil || turtleDataStop.n == 0 || turtleDataStop.amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle combine & turtle %s %s`, market, symbol))
		}
		return
	}
	turtleData := []*TurtleData{turtleDataLimit, turtleDataStop}
	turtleCoins := api.GetTurtleSettingNum(settingLimit.Function, market)
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionCombineTurtle, market, symbol)
	msg := fmt.Sprintf("[海龟参数]%s %s 币种数:%d/%f %d日:%f-%f %d日:%f-%f n:%f 仓数限制：%f 单仓数量:%f bid-ask %f %f \n"+
		"海龟:仓数/持仓量/开仓价/今日平仓 %d/%f/%f/%v\n 反向:仓数/持仓量/开仓价/今日平仓 %d/%f/%f/%v",
		turtleDataLimit.turtleTime.String()[0:10], msgKey, turtleCoins, settingLimit.AmountLimit, turtleDataLimit.daysFar, turtleDataLimit.lowDaysFar,
		turtleDataLimit.highDaysFar, turtleDataLimit.daysNear, turtleDataLimit.lowDaysNear, turtleDataLimit.highDaysNear, turtleDataLimit.n, settingLimit.OpenShortMargin,
		turtleDataLimit.amount, tick.Bids[0].Price, tick.Asks[0].Price,
		settingStop.Chance, settingStop.GridAmount, settingStop.PriceX, turtleDataStop.liquidated,
		settingLimit.Chance, settingLimit.GridAmount, settingLimit.PriceX, turtleDataLimit.liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	if checkCombineTurtle(account.Key, account.Secret, market, symbol, settings, turtleData, float64(turtleCoins)) ||
		checkCombineBreak(account.Key, account.Secret, market, symbol, settings, turtleData, tick) {
		return
	}
	big := false
	value, getBigOrder := util.LoadSyncMap(bigOrder, market, symbol)
	if getBigOrder && value != nil {
		big = value.(bool)
	}
	minSize := 0.0
	marketInfo := model.GetMarketInfo(market, symbol)
	if marketInfo == nil {
		util.Notice(`fail to get marketInfo %s %s`, market, symbol)
	} else {
		if marketInfo.CTValue == 0 {
			minSize = marketInfo.SizeMin
		} else {
			minSize = marketInfo.SizeMin * marketInfo.CTValue
		}
	}
	placeTurtleLong(account.Key, account.Secret, model.OrderTypeStop, turtleDataStop, settingStop, minSize, turtleCoins, tick, big)
	placeTurtleShort(account.Key, account.Secret, model.OrderTypeStop, turtleDataStop, settingStop, minSize, turtleCoins, tick, big)
	placeTurtleLong(account.Key, account.Secret, model.OrderTypeLimit, turtleDataLimit, settingLimit, minSize, turtleCoins, tick, big)
	placeTurtleShort(account.Key, account.Secret, model.OrderTypeLimit, turtleDataLimit, settingLimit, minSize, turtleCoins, tick, big)
	needCheck := handleBreakLong(settingLimit, settingStop, turtleDataLimit, turtleDataStop, turtleCoins, big)
	if handleBreakShort(settingLimit, settingStop, turtleDataLimit, turtleDataStop, turtleCoins, big) {
		needCheck = true
	}
	if handleBreakLong(settingStop, settingLimit, turtleDataStop, turtleDataLimit, turtleCoins, big) {
		needCheck = true
	}
	if handleBreakShort(settingStop, settingLimit, turtleDataStop, turtleDataLimit, turtleCoins, big) {
		needCheck = true
	}
	if needCheck {
		clearExtraOrders(account.Key, account.Secret, market, symbol, float64(turtleCoins), settings, turtleData)
	}
}

func checkCombineBreak(key, secret, market, symbol string, settings []*model.Setting, turtleData []*TurtleData,
	tick *model.BidAsk) (checked bool) {
	checked = false
	if len(settings) != 2 || len(turtleData) != 2 {
		util.Notice(`wrong combine turtle parameter`)
		return true
	}
	if turtleData[0].checkTimeBreak.Add(time.Minute).After(util.GetNow()) {
		return false
	}
	for i, setting := range settings {
		data := turtleData[i]
		data.checkTimeBreak = util.GetNow()
		var orderLong, orderShort *model.Order
		if data.orderLong != nil && len(data.orderLong) > 0 {
			orderLong = data.orderLong[0]
		}
		if data.orderShort != nil && len(data.orderShort) > 0 {
			orderShort = data.orderShort[0]
		}
		if orderLong != nil && (orderLong.Status == model.CarryStatusSuccess || (orderLong.TriggerPrice > 0 &&
			(orderLong.OrderType == model.OrderTypeStop && orderLong.TriggerPrice <= tick.Bids[0].Price) ||
			(orderLong.OrderType == model.OrderTypeLimit && orderLong.Price > tick.Bids[0].Price))) {
			util.Notice(fmt.Sprintf(`-----chance type: %s %s %s %d bid-ask %f %f short %f %f`,
				market, symbol, orderLong.OrderType, setting.Chance, tick.Bids[0].Price,
				tick.Asks[0].Price, orderLong.TriggerPrice, orderLong.Price))
			order := api.QueryOrderById(key, secret, market, symbol, orderLong.OrderType, orderLong.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				data.breakLong = true
				util.Notice(fmt.Sprintf(`-----order break long %s %s %d bid-ask %f %f short %f %v %v`,
					market, symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					orderLong.Price, data.breakLong, data.waitBreakLong))
			}
			if order != nil && order.Status == model.CarryStatusFail {
				data.orderLong = nil
			}
		}
		if orderShort != nil && (orderShort.Status == model.CarryStatusSuccess || (orderShort.TriggerPrice > 0 &&
			(orderShort.OrderType == model.OrderTypeStop && orderShort.TriggerPrice >= tick.Asks[0].Price)) ||
			(orderShort.OrderType == model.OrderTypeLimit && orderShort.Price < tick.Asks[0].Price)) {
			util.Notice(fmt.Sprintf(`-----chance type: %s %s %s %d bid-ask %f %f long %f %f`,
				market, symbol, orderShort.OrderType, setting.Chance, tick.Bids[0].Price,
				tick.Asks[0].Price, orderShort.TriggerPrice, orderShort.Price))
			order := api.QueryOrderById(key, secret, market, symbol, orderShort.OrderType, orderShort.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				data.breakShort = true
				util.Notice(fmt.Sprintf(`-----order break short %s %s %d bid-ask %f %f long %f %v %v`,
					market, symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					orderShort.Price, data.breakShort, data.waitBreakShort))
			}
			if order != nil && order.Status == model.CarryStatusFail {
				data.orderShort = nil
			}
		}
	}
	return true
}

func handleBreakLong(setting, settingOpposite *model.Setting, turtleData, turtleDataOpposite *TurtleData,
	turtleCoins int64, big bool) (work bool) {
	if turtleData == nil || turtleData.orderLong == nil || len(turtleData.orderLong) == 0 || !turtleData.waitBreakLong || !turtleData.breakLong {
		return false
	}
	turtleData.waitBreakLong = false
	setting.PriceX = turtleData.orderLong[0].TriggerPrice
	util.Notice(fmt.Sprintf(`query turtle break buy %s %s %d`, setting.Market, setting.Symbol, len(turtleData.orderLong)))
	if turtleData.orderLong[0].Function == model.Close {
		msg := fmt.Sprintf(`liquidate long %s %s chance:%d amount:%f currentN:%d px:%f n:%f`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, turtleData.n)
		go api.SendMails(`平多`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		bigNew := false
		if settingOpposite.Chance != 0 {
			bigNew = true
		}
		if big != bigNew {
			util.StoreSyncMap(bigOrder, bigNew, setting.Market, setting.Symbol)
			removeOpenOrder(turtleDataOpposite)
		}
	} else if turtleData.orderLong[0].Function == model.Open {
		util.Notice(fmt.Sprintf(`加多 %s %s chance:%d amount:%f currentN:%d px:%f n:%f`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, turtleData.n))
		setting.Chance++
		setting.GridAmount += turtleData.amount
	}
	turtleData.orderShort = nil
	util.Notice(fmt.Sprintf(`clear turtle sell when buy break %s %s %v`, setting.Market, setting.Symbol, turtleData.orderShort))
	time.Sleep(time.Second * 3)
	turtleData.orderLong = nil
	turtleData.orderShort = nil
	model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
	return true
}

func handleBreakShort(setting, settingOpposite *model.Setting, turtleData, turtleDataOpposite *TurtleData,
	turtleCoins int64, big bool) (work bool) {
	if turtleData == nil || turtleData.orderShort == nil || len(turtleData.orderShort) == 0 || !turtleData.waitBreakShort || !turtleData.breakShort {
		return false
	}
	turtleData.waitBreakShort = false
	setting.PriceX = turtleData.orderShort[0].TriggerPrice
	util.Notice(fmt.Sprintf(`query turtle break sell %s %s %d`, setting.Market, setting.Symbol, len(turtleData.orderShort)))
	if turtleData.orderShort[0].Function == model.Close {
		msg := fmt.Sprintf(`liquidate short result: %s %s chance:%d amount:%f currentN:%d px:%f n:%f`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, turtleData.n)
		go api.SendMails(`平空`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		bigNew := false
		if settingOpposite.Chance != 0 {
			bigNew = true
		}
		if bigNew != big {
			util.StoreSyncMap(bigOrder, bigNew, setting.Market, setting.Symbol)
			removeOpenOrder(turtleDataOpposite)
		}
	} else {
		util.Notice(fmt.Sprintf(`加空 %s %s chance:%d amount:%f currentN:%d px:%f n:%f`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, turtleData.n))
		setting.Chance--
		setting.GridAmount += turtleData.amount
	}
	turtleData.orderLong = nil
	util.Notice(fmt.Sprintf(`clear turtle buy when sell break %s %s %v`, setting.Market, setting.Symbol, turtleData.orderLong))
	time.Sleep(time.Second * 3)
	turtleData.orderLong = nil
	turtleData.orderShort = nil
	model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
	return true
}

func removeOpenOrder(turtleData *TurtleData) {
	if turtleData != nil && turtleData.orderLong != nil && len(turtleData.orderLong) > 0 && turtleData.orderLong[0].Function == model.Open {
		turtleData.orderLong = nil
	}
	if turtleData != nil && turtleData.orderShort != nil && len(turtleData.orderShort) > 0 && turtleData.orderShort[0].Function == model.Open {
		turtleData.orderShort = nil
	}
}

func checkCombineTurtle(key, secret, market, symbol string, settings []*model.Setting, turtleData []*TurtleData,
	currentNum float64) (checked bool) {
	if len(settings) != 2 || len(turtleData) != 2 {
		util.Notice(`wrong combine turtle parameter`)
		return true
	}
	if turtleData[0].checkTimeOpen.Add(time.Minute * 20).After(util.GetNow()) {
		return false
	}
	if settings[0].Chance == 0 && !turtleData[1].adjustChecked {
		adjustPosHolding(key, secret, settings[1], turtleData[1])
	} else if settings[1].Chance == 0 && !turtleData[0].adjustChecked {
		adjustPosHolding(key, secret, settings[0], turtleData[0])
	}
	today, _ := model.GetMarketToday(market)
	dayTime, _ := time.ParseDuration(`86400s`)
	var candles []*model.Candle
	// okex不返回尚未结束的当日candle，转成半小时的slot
	if market == model.OKEX {
		candles = api.GetCandle(key, secret, market, symbol, 1800, today, model.GetMarketNow(market))
	} else {
		candles = api.GetCandle(key, secret, market, symbol, 86400, today, today.Add(dayTime))
	}
	for i, setting := range settings {
		data := turtleData[i]
		for i := 0; candles != nil && i < len(candles); i++ {
			if data.highToday < candles[i].PriceHigh {
				data.highToday = candles[i].PriceHigh
				util.Info(fmt.Sprintf(`get today len new high %s %s %d %f`, market, symbol, len(candles), candles[i].PriceHigh))
			}
			if data.lowToday == 0 || data.lowToday > candles[i].PriceLow {
				data.lowToday = candles[i].PriceLow
				util.Info(fmt.Sprintf(`get today len new low %s %s %d %f`, market, symbol, len(candles), candles[i].PriceLow))
			}
		}
		if !data.useNear && setting.Chance > 0 && data.lowToday > 0 && ((data.orderShort[0].OrderType == model.OrderTypeLimit && data.orderShort[0].Price > math.Min(data.lowToday, data.lowDaysFar)+2*data.n) ||
			(data.orderShort[0].OrderType == model.OrderTypeStop && data.orderShort[0].Price < math.Max(data.highDaysFar, data.highToday)-2*data.n)) {
			util.Notice(fmt.Sprintf(`today higher than far price%f<max(today%f,far%f)-2*%f chance%d`,
				data.orderShort[0].TriggerPrice, data.highToday, data.highDaysFar, data.n, setting.Chance))
			data.orderShort = nil
		}
		if !data.useNear && data.lowToday > 0 && setting.Chance < 0 && ((data.orderLong[0].OrderType == model.OrderTypeLimit && data.orderLong[0].Price < math.Max(data.highDaysFar, data.highToday)-2*data.n) ||
			(data.orderLong[0].OrderType == model.OrderTypeStop && data.orderLong[0].Price > math.Min(data.lowDaysFar, data.lowToday)+2*data.n)) {
			util.Notice(fmt.Sprintf(`today lower than far price%f>min(today%f,far%f)+2*%f chance%d`,
				data.orderLong[0].TriggerPrice, data.lowToday, data.lowDaysFar, data.n, setting.Chance))
			data.orderLong = nil
		}
		data.checkTimeOpen = util.GetNow()
	}
	clearExtraOrders(key, secret, market, symbol, currentNum, settings, turtleData)
	return true
}

func clearExtraOrders(key, secret, market, symbol string, currentNum float64, settings []*model.Setting,
	turtleData []*TurtleData) {
	keepOrders := make(map[string]bool)
	for i, setting := range settings {
		if currentNum < setting.AmountLimit || setting.Chance < 0 {
			for _, order := range turtleData[i].orderLong {
				keepOrders[order.OrderId] = true
			}
		}
		if currentNum > -1*setting.AmountLimit || setting.Chance > 0 {
			for _, order := range turtleData[i].orderShort {
				keepOrders[order.OrderId] = true
			}
		}
	}
	orders := api.QueryOpenOrders(key, secret, market, symbol, true)
	for _, order := range orders {
		if !keepOrders[order.OrderSide] {
			result := api.MustCancel(key, secret, market, symbol, order.OrderType, order.OrderId, true)
			util.Notice(`cancel extra turtle order %s %s %s %s return %v`, market, symbol, order.OrderType, order.OrderId, result)
			time.Sleep(time.Second)
		}
	}
}

func placeTurtleLong(key, secret, orderType string, turtleData *TurtleData, setting *model.Setting,
	minSize float64, currentNum int64, tick *model.BidAsk, big bool) {
	amount := turtleData.amount
	function := model.Open
	if setting.Chance < 0 {
		function = model.Close
		amount = setting.GridAmount
	} else if !big {
		amount = minSize
	}
	price := turtleData.highDaysFar
	if orderType == model.OrderTypeLimit {
		price = turtleData.lowDaysFar
		if setting.Chance > 0 {
			price = math.Min(turtleData.lowDaysFar, setting.PriceX-turtleData.n/2)
		} else if setting.Chance < 0 {
			if turtleData.useNear {
				price = math.Max(setting.PriceX-2*turtleData.n, turtleData.lowDaysNear)
			} else {
				price = math.Max(turtleData.highDaysFar, turtleData.highToday) - 2*turtleData.n
			}
		}
	} else if orderType == model.OrderTypeStop {
		if setting.Chance > 0 {
			price = math.Max(turtleData.highDaysFar, setting.PriceX+turtleData.n/2)
		} else if setting.Chance < 0 {
			if turtleData.useNear {
				price = math.Min(setting.PriceX+2*turtleData.n, turtleData.highDaysNear)
			} else {
				if turtleData.lowToday > 0 {
					price = math.Min(turtleData.lowDaysFar, turtleData.lowToday) + 2*turtleData.n
				} else {
					price = turtleData.lowDaysFar + 2*turtleData.n
				}
			}
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	amountLimit := int64(setting.AmountLimit)
	coinLimit := int64(setting.OpenShortMargin)
	turtleData.breakShort = false
	priceDeal := price
	if turtleData.orderLong == nil && (setting.Chance < 0 || (currentNum < amountLimit && setting.Chance < coinLimit &&
		setting.SymbolRelated != model.SettingTurtleRemoved)) {
		if orderType == model.OrderTypeStop {
			if price <= tick.Asks[0].Price {
				turtleData.breakLong = true
				orderType = model.OrderTypeLimit
				priceDeal = price * (1 + turtleTriggerDelta/2)
			} else {
				priceDeal = price * (1 + turtleTriggerDelta)
			}
		} else if orderType == model.OrderTypeLimit && price >= tick.Asks[0].Price {
			turtleData.breakLong = true
		}
		turtleData.orderLong = api.MustPlaceOrder(key, secret, model.OrderSideBuy, orderType, market, symbol, ``,
			setting.Function, priceDeal, price, amount, nil)
		turtleData.waitBreakLong = true
		for _, order := range turtleData.orderLong {
			order.LineBuy = turtleData.n
			order.Function = function
			go model.AppDB.Save(order)
		}
	}
}

func placeTurtleShort(key, secret, orderType string, turtleData *TurtleData, setting *model.Setting,
	minSize float64, currentNum int64, tick *model.BidAsk, big bool) {
	amount := turtleData.amount
	function := model.Open
	if setting.Chance > 0 {
		amount = setting.GridAmount
		function = model.Close
	} else if !big {
		amount = minSize
	}
	price := turtleData.lowDaysFar
	if orderType == model.OrderTypeLimit {
		price = turtleData.highDaysFar
		if setting.Chance > 0 {
			if turtleData.useNear {
				price = math.Min(setting.PriceX+2*turtleData.n, turtleData.highDaysNear)
			} else {
				if turtleData.lowToday > 0 {
					price = math.Min(turtleData.lowDaysFar, turtleData.lowToday) + 2*turtleData.n
				} else {
					price = turtleData.lowDaysFar + 2*turtleData.n
				}
			}
		} else if setting.Chance < 0 {
			price = math.Max(turtleData.highDaysFar, setting.PriceX+turtleData.n/2)
		}
	} else if orderType == model.OrderTypeStop {
		if setting.Chance > 0 {
			if turtleData.useNear {
				price = math.Max(setting.PriceX-2*turtleData.n, turtleData.lowDaysNear)
			} else {
				price = math.Max(turtleData.highDaysFar, turtleData.highToday) - 2*turtleData.n
			}
		} else if setting.Chance < 0 {
			price = math.Min(turtleData.lowDaysFar, setting.PriceX-turtleData.n/2)
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	amountLimit := int64(setting.AmountLimit)
	coinLimit := int64(setting.OpenShortMargin)
	turtleData.breakShort = false
	priceDeal := price
	if turtleData.orderShort == nil && (setting.Chance > 0 || (currentNum > -1*amountLimit &&
		setting.Chance > -1*coinLimit && setting.SymbolRelated != model.SettingTurtleRemoved)) {
		if orderType == model.OrderTypeStop {
			if price >= tick.Bids[0].Price {
				turtleData.breakShort = true
				orderType = model.OrderTypeLimit
				priceDeal = price * (1 - turtleTriggerDelta/2)
			} else {
				priceDeal = price * price * (1 - turtleTriggerDelta)
			}
		} else if orderType == model.OrderTypeLimit && price <= tick.Bids[0].Price {
			turtleData.breakShort = true
		}
		turtleData.orderShort = api.MustPlaceOrder(key, secret, model.OrderSideSell, orderType, market, symbol, ``,
			setting.Function, priceDeal, price, amount, nil)
		turtleData.waitBreakShort = true
		for _, order := range turtleData.orderShort {
			order.LineBuy = turtleData.n
			order.Function = function
			go model.AppDB.Save(order)
		}
	}
}

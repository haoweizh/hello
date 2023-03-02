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
	now := util.GetNowUnixMillion()
	maintaining, ok := model.ChannelMaintaining.Load(settingLimit.Market)
	if settingLimit == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		(ok && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 1000) ||
		(time.Now().Hour() == 0 && time.Now().Minute() == 0) {
		return
	}
	settingStop := api.GetInvalidTurtle(settingLimit.Market, settingLimit.Symbol)
	if settingStop == nil || settingStop.Valid { // 使用valid为false的turtle作为对应turtle,否则该算法不运行
		return
	}
	if (settingLimit.Chance != 0 && settingLimit.PriceX == 0) || (settingStop.Chance != 0 && settingStop.PriceX == 0) {
		util.Notice(fmt.Sprintf(`no last priceX %s %s %d %f %d %f`,
			settingLimit.Market, settingLimit.Symbol, settingLimit.Chance, settingLimit.PriceX, settingStop.Chance, settingStop.PriceX))
		return
	}
	account := model.AppConfig.GetAccounts(settingLimit.Market)[0]
	turtleDataLimit := GetTurtleData(account.Key, account.Secret, settingLimit.Function, settingLimit.Market, settingLimit.Symbol, false)
	turtleDataStop := GetTurtleData(account.Key, account.Secret, model.FunctionTurtle, settingStop.Market, settingStop.Symbol, true)
	if turtleDataLimit == nil || turtleDataLimit.n == 0 || turtleDataLimit.amount == 0 ||
		turtleDataStop == nil || turtleDataStop.n == 0 || turtleDataStop.amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle combine & turtle %s %s`, settingLimit.Market, settingLimit.Symbol))
		}
		return
	}
	turtleCoins := api.GetTurtleSettingNum(settingLimit.Function, settingLimit.Market)
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionCombineTurtle, settingLimit.Market, settingLimit.Symbol)
	msg := fmt.Sprintf("[海龟参数]%s %s 币种数:%d/%f %d日:%f-%f %d日:%f-%f n:%f 仓数限制：%f 单仓数量:%f bid-ask %f %f \n"+
		"海龟:仓数/持仓量/开仓价/今日平仓 %d/%f/%f/%v\n 反向:仓数/持仓量/开仓价/今日平仓 %d/%f/%f/%v",
		turtleDataLimit.turtleTime.String()[0:10], msgKey, turtleCoins, settingLimit.AmountLimit, turtleDataLimit.daysFar, turtleDataLimit.lowDaysFar,
		turtleDataLimit.highDaysFar, turtleDataLimit.daysNear, turtleDataLimit.lowDaysNear, turtleDataLimit.highDaysNear, turtleDataLimit.n, settingLimit.OpenShortMargin,
		turtleDataLimit.amount, tick.Bids[0].Price, tick.Asks[0].Price,
		settingStop.Chance, settingStop.GridAmount, settingStop.PriceX, turtleDataStop.liquidated,
		settingLimit.Chance, settingLimit.GridAmount, settingLimit.PriceX, turtleDataLimit.liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	if checkCombineTurtle(account.Key, account.Secret, settingLimit, settingStop, float64(turtleCoins), turtleDataLimit, turtleDataStop) ||
		checkTurtleBreak(account.Key, account.Secret, settingLimit, turtleDataLimit, tick) ||
		checkTurtleBreak(account.Key, account.Secret, settingStop, turtleDataStop, tick) {
		return
	}
	big := false
	value, getBigOrder := util.LoadSyncMap(bigOrder, settingLimit.Market, settingLimit.Symbol)
	if getBigOrder && value != nil {
		big = value.(bool)
	}
	minSize := 0.0
	marketInfo := model.GetMarketInfo(settingLimit.Market, settingLimit.Symbol)
	if marketInfo == nil {
		util.Notice(`fail to get marketInfo %s %s`, settingLimit.Market, settingLimit.Symbol)
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
	handleBreakLong(account.Key, account.Secret, settingLimit, turtleDataLimit, turtleCoins)
	handleBreakShort(account.Key, account.Secret, settingLimit, turtleDataLimit, turtleCoins)
	handleBreakLong(account.Key, account.Secret, settingStop, turtleDataStop, turtleCoins)
	handleBreakShort(account.Key, account.Secret, settingStop, turtleDataStop, turtleCoins)
}

func handleBreakLong(key, secret string, setting *model.Setting, turtleData *TurtleData, turtleCoins int64) {
	if turtleData == nil || turtleData.orderLong == nil || len(turtleData.orderLong) == 0 || !turtleData.waitBreakLong || !turtleData.breakLong {
		return
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
	} else if turtleData.orderLong[0].Function == model.Open {
		util.Notice(fmt.Sprintf(`加多 %s %s chance:%d amount:%f currentN:%d px:%f n:%f`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, turtleData.n))
		setting.Chance++
		setting.GridAmount += turtleData.amount
	}
	for _, order := range turtleData.orderShort {
		temp := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId)
		if temp != nil && temp.Status == model.CarryStatusWorking {
			go api.MustCancel(key, secret, order.Market, order.Symbol, order.OrderType, order.OrderId, true)
		}
	}
	util.Notice(fmt.Sprintf(`clear turtle sell when buy break %s %s %v`, setting.Market, setting.Symbol, turtleData.orderShort))
	time.Sleep(time.Second * 3)
	turtleData.orderLong = nil
	turtleData.orderShort = nil
	model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
}

func handleBreakShort(key, secret string, setting *model.Setting, turtleData *TurtleData, turtleCoins int64) {
	if turtleData == nil || turtleData.orderShort == nil || len(turtleData.orderShort) == 0 || !turtleData.waitBreakShort || !turtleData.breakShort {
		return
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
	} else {
		util.Notice(fmt.Sprintf(`加空 %s %s chance:%d amount:%f currentN:%d px:%f n:%f`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, turtleData.n))
		setting.Chance--
		setting.GridAmount += turtleData.amount
	}
	for _, order := range turtleData.orderLong {
		temp := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId)
		if temp != nil && temp.Status == model.CarryStatusWorking {
			go api.MustCancel(key, secret, order.Market, order.Symbol, order.OrderType, order.OrderId, true)
		}
	}
	util.Notice(fmt.Sprintf(`clear turtle buy when sell break %s %s %v`, setting.Market, setting.Symbol, turtleData.orderLong))
	time.Sleep(time.Second * 3)
	turtleData.orderLong = nil
	turtleData.orderShort = nil
	model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
}

func checkCombineTurtle(key, secret string, settingLimit, settingStop *model.Setting, currentNum float64,
	turtleDataLimit, turtleDataStop *TurtleData) (checked bool) {
	if turtleDataLimit.checkTimeOpen.Add(time.Minute * 20).After(util.GetNow()) {
		return false
	}
	if settingLimit.Chance == 0 && !turtleDataStop.adjustChecked {
		adjustPosHolding(key, secret, settingStop, turtleDataStop)
	} else if settingStop.Chance == 0 && !turtleDataLimit.adjustChecked {
		adjustPosHolding(key, secret, settingLimit, turtleDataLimit)
	}
	today, _ := model.GetMarketToday(settingLimit.Market)
	dayTime, _ := time.ParseDuration(`86400s`)
	var candles []*model.Candle
	// okex不返回尚未结束的当日candle，转成半小时的slot
	if settingLimit.Market == model.OKEX {
		candles = api.GetCandle(key, secret, settingLimit.Market, settingLimit.Symbol, 1800, today, model.GetMarketNow(settingLimit.Market))
	} else {
		candles = api.GetCandle(key, secret, settingLimit.Market, settingLimit.Symbol, 86400, today, today.Add(dayTime))
	}
	for i := 0; candles != nil && i < len(candles); i++ {
		if turtleDataLimit.highToday < candles[i].PriceHigh {
			turtleDataLimit.highToday = candles[i].PriceHigh
			turtleDataStop.highToday = candles[i].PriceHigh
		}
		if turtleDataLimit.lowToday == 0 || turtleDataLimit.lowToday > candles[i].PriceLow {
			turtleDataLimit.lowToday = candles[i].PriceLow
			turtleDataStop.lowToday = candles[i].PriceLow
		}
		util.Info(fmt.Sprintf(`get today len %s %s %d %f %f`,
			settingLimit.Market, settingLimit.Symbol, len(candles), candles[0].PriceLow, candles[0].PriceHigh))
	}
	if !turtleDataLimit.useNear && turtleDataLimit.orderShort != nil && len(turtleDataLimit.orderShort) > 0 && settingLimit.Chance > 0 &&
		turtleDataLimit.orderShort[0].OrderType == model.OrderTypeLimit &&
		turtleDataLimit.orderShort[0].Price > math.Min(turtleDataLimit.lowToday, turtleDataLimit.lowDaysFar)+2*turtleDataLimit.n {
		util.Notice(fmt.Sprintf(`today higher than far price%f<max(today%f,far%f)-2*%f chance%d`,
			turtleDataLimit.orderShort[0].TriggerPrice, turtleDataLimit.highToday, turtleDataLimit.highDaysFar, turtleDataLimit.n, settingLimit.Chance))
		turtleDataLimit.orderShort = nil
	}
	if !turtleDataLimit.useNear && turtleDataLimit.orderLong != nil && len(turtleDataLimit.orderLong) > 0 && turtleDataLimit.lowToday > 0 &&
		settingLimit.Chance < 0 && turtleDataLimit.orderLong[0].OrderType == model.OrderTypeLimit &&
		turtleDataLimit.orderLong[0].Price < math.Min(turtleDataLimit.highToday, turtleDataLimit.highDaysFar)-2*turtleDataLimit.n {
		util.Notice(fmt.Sprintf(`today lower than far price%f>min(today%f,far%f)+2*%f chance%d`,
			turtleDataLimit.orderLong[0].TriggerPrice, turtleDataLimit.lowToday, turtleDataLimit.lowDaysFar, turtleDataLimit.n, settingLimit.Chance))
		turtleDataLimit.orderLong = nil
	}
	checked = true
	turtleDataLimit.checkTimeOpen = util.GetNow()
	orders := api.QueryOpenOrders(key, secret, settingLimit.Market, settingLimit.Symbol, true)
	keepOrders := make(map[string]bool)
	if currentNum < settingLimit.AmountLimit || settingLimit.Chance < 0 {
		for _, order := range turtleDataLimit.orderLong {
			keepOrders[order.OrderId] = true
		}
	}
	if currentNum > -1*settingLimit.AmountLimit || settingLimit.Chance > 0 {
		for _, order := range turtleDataLimit.orderShort {
			keepOrders[order.OrderId] = true
		}
	}
	if currentNum < settingStop.AmountLimit || settingStop.Chance < 0 {
		for _, order := range turtleDataStop.orderLong {
			keepOrders[order.OrderId] = true
		}
	}
	if currentNum > -1*settingStop.AmountLimit || settingStop.Chance > 0 {
		for _, order := range turtleDataStop.orderShort {
			keepOrders[order.OrderId] = true
		}
	}
	for _, order := range orders {
		if !keepOrders[order.OrderSide] {
			result := api.MustCancel(key, secret, settingLimit.Market, settingLimit.Symbol, order.OrderType, order.OrderId, true)
			util.Notice(`cancel extra turtle order %s %s %s %s return %v`,
				settingLimit.Market, settingLimit.Symbol, order.OrderType, order.OrderId, result)
			time.Sleep(time.Second)
		}
	}
	return
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
			model.FunctionTurtle, priceDeal, price, amount, nil)
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
			model.FunctionTurtle, priceDeal, price, amount, nil)
		turtleData.waitBreakShort = true
		for _, order := range turtleData.orderShort {
			order.LineBuy = turtleData.n
			order.Function = function
			go model.AppDB.Save(order)
		}
	}
}

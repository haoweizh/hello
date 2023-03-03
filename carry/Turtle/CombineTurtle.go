package Turtle

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
	dataLimit := GetTurtleData(account.Key, account.Secret, settingLimit.Function, market, symbol, false)
	dataStop := GetTurtleData(account.Key, account.Secret, model.FunctionTurtle, market, symbol, true)
	if dataLimit == nil || dataLimit.n == 0 || dataLimit.amount == 0 ||
		dataStop == nil || dataStop.n == 0 || dataStop.amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle combine & turtle %s %s`, market, symbol))
		}
		return
	}
	if !dataLimit.orderCleared {
		clearOrders(account.Key, account.Secret, market, symbol)
		dataLimit.orderCleared = true
		return
	}
	turtleData := []*Data{dataLimit, dataStop}
	turtleCoins := api.GetChanceInAll(settingLimit.Function, market, symbol)
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionCombineTurtle, market, symbol)
	msg := fmt.Sprintf("[海龟参数]%s %s 币种数:%d/%f %d日:%f-%f %d日:%f-%f n:%e 仓数限制：%f 单仓数量:%f bid-ask %f %f \n"+
		"海龟:仓数/持仓量/开仓价/今日平仓 %d/%f/%f/%v\n 反向:仓数/持仓量/开仓价/今日平仓 %d/%f/%f/%v",
		dataLimit.turtleTime.String()[0:10], msgKey, turtleCoins, settingLimit.AmountLimit, dataLimit.daysFar, dataLimit.lowDaysFar,
		dataLimit.highDaysFar, dataLimit.daysNear, dataLimit.lowDaysNear, dataLimit.highDaysNear, dataLimit.n, settingLimit.OpenShortMargin,
		dataLimit.amount, tick.Bids[0].Price, tick.Asks[0].Price,
		settingStop.Chance, settingStop.GridAmount, settingStop.PriceX, dataStop.liquidated,
		settingLimit.Chance, settingLimit.GridAmount, settingLimit.PriceX, dataLimit.liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	if handleTraceOrders(account.Key, account.Secret, market, symbol, settings, turtleData, float64(turtleCoins)) ||
		checkBreak(account.Key, account.Secret, market, symbol, settings, turtleData, tick) {
		return
	}
	if !dataLimit.adjustChecked && !dataStop.adjustChecked {
		return
	}
	big := false
	value, getBig := util.LoadSyncMap(bigOrder, market, symbol)
	if getBig && value != nil {
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
	placeTurtleLong(account.Key, account.Secret, model.OrderTypeStop, dataStop, settingStop, minSize, turtleCoins, tick, big)
	placeTurtleShort(account.Key, account.Secret, model.OrderTypeStop, dataStop, settingStop, minSize, turtleCoins, tick, big)
	placeTurtleLong(account.Key, account.Secret, model.OrderTypeLimit, dataLimit, settingLimit, minSize, turtleCoins, tick, big)
	placeTurtleShort(account.Key, account.Secret, model.OrderTypeLimit, dataLimit, settingLimit, minSize, turtleCoins, tick, big)
	needCheck := handleBreakLong(settingLimit, settingStop, dataLimit, dataStop, turtleCoins, big)
	if handleBreakShort(settingLimit, settingStop, dataLimit, dataStop, turtleCoins, big) {
		needCheck = true
	}
	if handleBreakLong(settingStop, settingLimit, dataStop, dataLimit, turtleCoins, big) {
		needCheck = true
	}
	if handleBreakShort(settingStop, settingLimit, dataStop, dataLimit, turtleCoins, big) {
		needCheck = true
	}
	if needCheck {
		clearExtraOrders(account.Key, account.Secret, market, symbol, float64(turtleCoins), settings, turtleData)
	}
}

func handleBreakLong(setting, settingOpposite *model.Setting, data, dataOpposite *Data,
	turtleCoins int64, big bool) (work bool) {
	if data == nil || data.orderLong == nil || len(data.orderLong) == 0 || !data.waitBreakLong || !data.breakLong {
		return false
	}
	data.waitBreakLong = false
	setting.PriceX = data.orderLong[0].TriggerPrice
	util.Notice(fmt.Sprintf(`query turtle break buy %s %s %d`, setting.Market, setting.Symbol, len(data.orderLong)))
	if data.orderLong[0].Function == model.Close {
		msg := fmt.Sprintf(`liquidate long %s %s chance:%d amount:%f currentN:%d px:%f n:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, data.n)
		go api.SendMails(`平多`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		bigNew := false
		if settingOpposite.Chance != 0 {
			bigNew = true
		}
		if big != bigNew {
			util.StoreSyncMap(bigOrder, bigNew, setting.Market, setting.Symbol)
			removeOpenOrder(dataOpposite)
		}
	} else if data.orderLong[0].Function == model.Open {
		util.Notice(fmt.Sprintf(`加多 %s %s chance:%d amount:%f currentN:%d px:%f n:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, data.n))
		setting.Chance++
		setting.GridAmount += data.amount
	}
	data.orderShort = nil
	util.Notice(fmt.Sprintf(`clear turtle sell when buy break %s %s %v`, setting.Market, setting.Symbol, data.orderShort))
	time.Sleep(time.Second * 3)
	data.orderLong = nil
	data.orderShort = nil
	model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
	return true
}

func handleBreakShort(setting, settingOpposite *model.Setting, data, dataOpposite *Data,
	turtleCoins int64, big bool) (work bool) {
	if data == nil || data.orderShort == nil || len(data.orderShort) == 0 || !data.waitBreakShort || !data.breakShort {
		return false
	}
	data.waitBreakShort = false
	setting.PriceX = data.orderShort[0].TriggerPrice
	util.Notice(fmt.Sprintf(`query turtle break sell %s %s %d`, setting.Market, setting.Symbol, len(data.orderShort)))
	if data.orderShort[0].Function == model.Close {
		msg := fmt.Sprintf(`liquidate short result: %s %s chance:%d amount:%f currentN:%d px:%f n:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, data.n)
		go api.SendMails(`平空`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		bigNew := false
		if settingOpposite.Chance != 0 {
			bigNew = true
		}
		if bigNew != big {
			util.StoreSyncMap(bigOrder, bigNew, setting.Market, setting.Symbol)
			removeOpenOrder(dataOpposite)
		}
	} else {
		util.Notice(fmt.Sprintf(`加空 %s %s chance:%d amount:%f currentN:%d px:%f n:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, setting.PriceX, data.n))
		setting.Chance--
		setting.GridAmount += data.amount
	}
	data.orderLong = nil
	util.Notice(fmt.Sprintf(`clear turtle buy when sell break %s %s %v`, setting.Market, setting.Symbol, data.orderLong))
	time.Sleep(time.Second * 3)
	data.orderLong = nil
	data.orderShort = nil
	model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
	return true
}

func removeOpenOrder(data *Data) {
	if data != nil && data.orderLong != nil && len(data.orderLong) > 0 && data.orderLong[0].Function == model.Open {
		data.orderLong = nil
	}
	if data != nil && data.orderShort != nil && len(data.orderShort) > 0 && data.orderShort[0].Function == model.Open {
		data.orderShort = nil
	}
}

// todo 解决价格超出以后限价单被取消的问题
// todo 解决显示变少问题
func placeTurtleLong(key, secret, orderType string, data *Data, setting *model.Setting,
	minSize float64, currentNum int64, tick *model.BidAsk, big bool) {
	amount := data.amount
	function := model.Open
	if setting.Chance < 0 {
		function = model.Close
		amount = setting.GridAmount
	} else if !big {
		amount = minSize
	}
	price := data.highDaysFar
	if orderType == model.OrderTypeLimit {
		price = data.lowDaysFar
		if setting.Chance > 0 {
			price = math.Min(data.lowDaysFar, setting.PriceX-data.n/2)
		} else if setting.Chance < 0 {
			if data.useNear {
				price = math.Max(setting.PriceX-2*data.n, data.lowDaysNear)
			} else {
				price = math.Max(data.highDaysFar, data.highToday) - 2*data.n
			}
		}
	} else if orderType == model.OrderTypeStop {
		if setting.Chance > 0 {
			price = math.Max(data.highDaysFar, setting.PriceX+data.n/2)
		} else if setting.Chance < 0 {
			if data.useNear {
				price = math.Min(setting.PriceX+2*data.n, data.highDaysNear)
			} else {
				if data.lowToday > 0 {
					price = math.Min(data.lowDaysFar, data.lowToday) + 2*data.n
				} else {
					price = data.lowDaysFar + 2*data.n
				}
			}
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	amountLimit := int64(setting.AmountLimit)
	coinLimit := int64(setting.OpenShortMargin)
	data.breakShort = false
	priceDeal := price
	if data.orderLong == nil && (setting.Chance < 0 || (currentNum < amountLimit && setting.Chance < coinLimit &&
		setting.SymbolRelated != model.SettingTurtleRemoved)) {
		if orderType == model.OrderTypeStop {
			if price <= tick.Asks[0].Price {
				data.breakLong = true
				orderType = model.OrderTypeLimit
				priceDeal = price * (1 + turtleTriggerDelta/2)
			} else {
				priceDeal = price * (1 + turtleTriggerDelta)
			}
		} else if orderType == model.OrderTypeLimit && price >= tick.Asks[0].Price {
			data.breakLong = true
		}
		data.orderLong = api.MustPlaceOrder(key, secret, model.OrderSideBuy, orderType, market, symbol, ``,
			setting.Function, priceDeal, price, amount, nil)
		data.waitBreakLong = true
		for _, order := range data.orderLong {
			order.LineBuy = data.n
			order.Function = function
			go model.AppDB.Save(order)
		}
	}
}

func placeTurtleShort(key, secret, orderType string, data *Data, setting *model.Setting,
	minSize float64, currentNum int64, tick *model.BidAsk, big bool) {
	amount := data.amount
	function := model.Open
	if setting.Chance > 0 {
		amount = setting.GridAmount
		function = model.Close
	} else if !big {
		amount = minSize
	}
	price := data.lowDaysFar
	if orderType == model.OrderTypeLimit {
		price = data.highDaysFar
		if setting.Chance > 0 {
			if data.useNear {
				price = math.Min(setting.PriceX+2*data.n, data.highDaysNear)
			} else {
				if data.lowToday > 0 {
					price = math.Min(data.lowDaysFar, data.lowToday) + 2*data.n
				} else {
					price = data.lowDaysFar + 2*data.n
				}
			}
		} else if setting.Chance < 0 {
			price = math.Max(data.highDaysFar, setting.PriceX+data.n/2)
		}
	} else if orderType == model.OrderTypeStop {
		if setting.Chance > 0 {
			if data.useNear {
				price = math.Max(setting.PriceX-2*data.n, data.lowDaysNear)
			} else {
				price = math.Max(data.highDaysFar, data.highToday) - 2*data.n
			}
		} else if setting.Chance < 0 {
			price = math.Min(data.lowDaysFar, setting.PriceX-data.n/2)
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	amountLimit := int64(setting.AmountLimit)
	coinLimit := int64(setting.OpenShortMargin)
	data.breakShort = false
	priceDeal := price
	if data.orderShort == nil && (setting.Chance > 0 || (currentNum > -1*amountLimit &&
		setting.Chance > -1*coinLimit && setting.SymbolRelated != model.SettingTurtleRemoved)) {
		if orderType == model.OrderTypeStop {
			if price >= tick.Bids[0].Price {
				data.breakShort = true
				orderType = model.OrderTypeLimit
				priceDeal = price * (1 - turtleTriggerDelta/2)
			} else {
				priceDeal = price * price * (1 - turtleTriggerDelta)
			}
		} else if orderType == model.OrderTypeLimit && price <= tick.Bids[0].Price {
			data.breakShort = true
		}
		data.orderShort = api.MustPlaceOrder(key, secret, model.OrderSideSell, orderType, market, symbol, ``,
			setting.Function, priceDeal, price, amount, nil)
		data.waitBreakShort = true
		for _, order := range data.orderShort {
			order.LineBuy = data.n
			order.Function = function
			go model.AppDB.Save(order)
		}
	}
}

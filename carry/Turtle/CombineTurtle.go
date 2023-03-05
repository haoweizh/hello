package Turtle

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strings"
	"time"
)

// ProcessCombineTurtle
// setting.GridAmount 当前已经持仓数量
// setting.Chance 当前开仓的个数
// setting.PriceX 上一次开仓的价格
// setting.OpenShortMargin 该单币种最多开仓个数
// setting.AmountLimit 总开仓上限
var ProcessCombineTurtle = func(settingLimit *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetTurtling(true) {
		defer api.CheckSetTurtling(false)
	} else {
		return
	}
	market := settingLimit.Market
	symbol := settingLimit.Symbol
	now := util.GetNowUnixMillion()
	maintaining, ok := model.ChannelMaintaining.Load(market)
	if settingLimit == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		(ok && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 1000) ||
		(time.Now().Hour() == 0 && time.Now().Minute() == 0) || len(strings.Trim(symbol, ` `)) == 0 {
		return
	}
	settingStop := api.GetInvalidTurtle(market, symbol)
	if settingStop == nil || settingStop.Valid { // 使用valid为false的turtle作为对应turtle,否则该算法不运行
		return
	}
	settings := []*model.Setting{settingLimit, settingStop}
	if (settingLimit.Chance != 0 && settingLimit.PriceX == 0) || (settingStop.Chance != 0 && settingStop.PriceX == 0) {
		util.Notice(fmt.Sprintf(`no last priceX %s %s %d %e %d %e`,
			market, symbol, settingLimit.Chance, settingLimit.PriceX, settingStop.Chance, settingStop.PriceX))
		return
	}
	account := model.AppConfig.GetAccounts(market)[0]
	dataLimit := api.GetTurtleData(account.Key, account.Secret, settingLimit.Function, market, symbol)
	dataStop := api.GetTurtleData(account.Key, account.Secret, model.FunctionTurtle, market, symbol)
	if dataLimit == nil || dataLimit.N == 0 || dataLimit.Amount == 0 ||
		dataStop == nil || dataStop.N == 0 || dataStop.Amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle combine & turtle %s %s`, market, symbol))
		}
		return
	}
	dataStop.Amount = dataLimit.Amount
	if !dataLimit.OrderCleared {
		api.ClearOrders(account.Key, account.Secret, market, symbol)
		dataLimit.OrderCleared = true
		return
	}
	turtleData := []*api.TurtleData{dataLimit, dataStop}
	canOpenLimit, turtleCoins := api.CanOpenTurtle(settingLimit, dataLimit)
	canOpenStop, _ := api.CanOpenTurtle(settingStop, dataStop)
	canOpen := canOpenStop || canOpenLimit
	if canOpen {
		settingLimit.SymbolRelated = ``
	}
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionCombineTurtle, market, symbol)
	msg := fmt.Sprintf("[%s]%s可开%v 币种数:%d/%d %d日:%e-%e %d日:%e-%e N:%e 单币仓数：%d 单仓数量:%e bid-ask %e %e \n"+
		"海龟:仓数/持仓量/开仓价/今日平仓 %d/%e/%e/%v\n 龟汤:仓数/持仓量/开仓价/今日平仓 %d/%e/%e/%v",
		dataLimit.TurtleTime.String()[0:10], msgKey, canOpen, int(turtleCoins), int(settingLimit.AmountLimit),
		dataLimit.DaysFar, dataLimit.LowDaysFar, dataLimit.HighDaysFar, dataLimit.DaysNear, dataLimit.LowDaysNear,
		dataLimit.HighDaysNear, dataLimit.N, int(settingLimit.OpenShortMargin), dataLimit.Amount, tick.Bids[0].Price,
		tick.Asks[0].Price, settingStop.Chance, settingStop.GridAmount, settingStop.PriceX, dataStop.Liquidated,
		settingLimit.Chance, settingLimit.GridAmount, settingLimit.PriceX, dataLimit.Liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	if api.HandleTraceOrders(account.Key, account.Secret, market, symbol, settings, turtleData, turtleCoins) ||
		api.CheckBreak(account.Key, account.Secret, market, symbol, settings, turtleData, tick) {
		return
	}
	if !dataLimit.AdjustChecked && !dataStop.AdjustChecked {
		return
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
		minSize = math.Max(minSize, 2*marketInfo.MoneyMin/dataLimit.LowDaysFar)
	}
	//价格不一样：big=true
	//价格一样：仓数相加=0时big=false；仓数相加≠0时big=true
	big := true
	if settingLimit.Chance+settingStop.Chance == 0 && settingLimit.PriceX-settingStop.PriceX <= marketInfo.PriceIncrement {
		big = false
	}
	placeTurtleLong(account.Key, account.Secret, model.OrderTypeStop, dataStop, settingStop, minSize, tick, big, canOpen)
	placeTurtleShort(account.Key, account.Secret, model.OrderTypeStop, dataStop, settingStop, minSize, tick, big, canOpen)
	placeTurtleLong(account.Key, account.Secret, model.OrderTypeLimit, dataLimit, settingLimit, minSize, tick, big, canOpen)
	placeTurtleShort(account.Key, account.Secret, model.OrderTypeLimit, dataLimit, settingLimit, minSize, tick, big, canOpen)
	needCheck := false
	if handleBreakLong(settingLimit, settingStop, dataLimit, dataStop, turtleCoins, big) {
		needCheck = true
	}
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
		api.ClearExtraOrders(account.Key, account.Secret, market, symbol, turtleCoins, settings, turtleData)
	}
}

func handleBreakLong(setting, settingOpposite *model.Setting, data, dataOpposite *api.TurtleData,
	turtleCoins float64, big bool) (work bool) {
	if data == nil || data.OrderLong == nil || len(data.OrderLong) == 0 || !data.WaitBreakLong || !data.BreakLong {
		return false
	}
	data.WaitBreakLong = false
	setting.PriceX = data.OrderLong[0].TriggerPrice
	util.Notice(fmt.Sprintf(`query turtle break buy %s %s %d`, setting.Market, setting.Symbol, len(data.OrderLong)))
	if data.OrderLong[0].Function == model.Close {
		msg := fmt.Sprintf(`liquidate long %s %s chance:%d Amount:%e currentN:%d px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, int(turtleCoins), setting.PriceX, data.N)
		go api.SendMails(`平多`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		setting.PriceX = 0
		bigNew := false
		if settingOpposite.Chance != 0 {
			bigNew = true
		}
		if big != bigNew {
			removeOrders(dataOpposite)
		}
	} else if data.OrderLong[0].Function == model.Open {
		util.Notice(fmt.Sprintf(`加多 %s %s chance:%d Amount:%e currentN:%d px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, int(turtleCoins), setting.PriceX, data.N))
		setting.Chance++
		setting.GridAmount += data.Amount
	}
	util.Notice(fmt.Sprintf(`clear turtle sell when buy break %s %s %v`, setting.Market, setting.Symbol, data.OrderShort))
	time.Sleep(time.Second * 3)
	data.OrderLong = nil
	data.OrderShort = nil
	model.AppDB.Model(setting).Where("market= ? and Symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
	return true
}

func handleBreakShort(setting, settingOpposite *model.Setting, data, dataOpposite *api.TurtleData,
	turtleCoins float64, big bool) (work bool) {
	if data == nil || data.OrderShort == nil || len(data.OrderShort) == 0 || !data.WaitBreakShort || !data.BreakShort {
		util.Info(fmt.Sprintf(`handleBreakShort %v %v %v`, len(data.OrderShort) == 0, !data.WaitBreakShort, !data.BreakShort))
		return false
	}
	data.WaitBreakShort = false
	setting.PriceX = data.OrderShort[0].TriggerPrice
	util.Notice(fmt.Sprintf(`query turtle break sell %s %s %d`, setting.Market, setting.Symbol, len(data.OrderShort)))
	if data.OrderShort[0].Function == model.Close {
		msg := fmt.Sprintf(`liquidate short result: %s %s chance:%d Amount:%e currentN:%d px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, int(turtleCoins), setting.PriceX, data.N)
		go api.SendMails(`平空`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		setting.PriceX = 0
		bigNew := false
		if settingOpposite.Chance != 0 {
			bigNew = true
		}
		if bigNew != big {
			removeOrders(dataOpposite)
		}
	} else {
		util.Notice(fmt.Sprintf(`加空 %s %s chance:%d Amount:%e currentN:%d px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, int(turtleCoins), setting.PriceX, data.N))
		setting.Chance--
		setting.GridAmount += data.Amount
	}
	util.Notice(fmt.Sprintf(`clear turtle buy when sell break %s %s %v`, setting.Market, setting.Symbol, data.OrderLong))
	time.Sleep(time.Second * 3)
	data.OrderLong = nil
	data.OrderShort = nil
	model.AppDB.Model(setting).Where("market= ? and Symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
	return true
}

func removeOrders(data *api.TurtleData) {
	if data != nil && data.OrderLong != nil && len(data.OrderLong) > 0 && data.OrderLong[0].Function == model.Open {
		data.OrderLong = nil
	}
	if data != nil && data.OrderShort != nil && len(data.OrderShort) > 0 && data.OrderShort[0].Function == model.Open {
		data.OrderShort = nil
	}
}

func placeTurtleLong(key, secret, orderType string, data *api.TurtleData, setting *model.Setting,
	minSize float64, tick *model.BidAsk, big, canOpen bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance < 0 {
		function = model.Close
		amount = setting.GridAmount
	} else if !big {
		amount = minSize
	}
	price := data.HighDaysFar
	if orderType == model.OrderTypeLimit {
		price = data.LowDaysFar
		if setting.Chance > 0 {
			price = math.Min(data.LowDaysFar, setting.PriceX-data.N/2)
		} else if setting.Chance < 0 {
			if data.UseNear {
				price = math.Max(setting.PriceX-2*data.N, data.LowDaysNear)
			} else {
				price = math.Max(data.HighDaysFar, data.HighToday) - 2*data.N
			}
		}
	} else if orderType == model.OrderTypeStop {
		if setting.Chance > 0 {
			price = math.Max(data.HighDaysFar, setting.PriceX+data.N/2)
		} else if setting.Chance < 0 {
			if data.UseNear {
				price = math.Min(setting.PriceX+2*data.N, data.HighDaysNear)
			} else {
				if data.LowToday > 0 {
					price = math.Min(data.LowDaysFar, data.LowToday) + 2*data.N
				} else {
					price = data.LowDaysFar + 2*data.N
				}
			}
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	data.BreakLong = false
	data.WaitBreakLong = true
	priceDeal := price
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < setting.OpenShortMargin
	if data.OrderLong == nil && (setting.Chance < 0 || canOpen) {
		if orderType == model.OrderTypeStop {
			if price <= tick.Asks[0].Price {
				data.BreakLong = true
				orderType = model.OrderTypeLimit
				priceDeal = price * (1 + api.TurtleTriggerDelta/2)
			} else {
				priceDeal = price * (1 + api.TurtleTriggerDelta)
			}
		} else if orderType == model.OrderTypeLimit && price >= tick.Asks[0].Price {
			data.BreakLong = true
		}
		util.Notice(fmt.Sprintf(`place long %s %s %s %s %d %v at %e %e amt %e`,
			orderType, setting.Function, market, symbol, setting.Chance, canOpen, priceDeal, price, amount))
		data.OrderLong = api.MustPlaceOrder(key, secret, model.OrderSideBuy, orderType, market, symbol, ``,
			setting.Function, priceDeal, price, amount, nil)
		if data.OrderAdjust == nil {
			data.OrderAdjust = make([]*model.Order, 0)
		}
		for _, order := range data.OrderLong {
			order.LineBuy = data.N
			order.Function = function
			go model.AppDB.Save(order)
			if data.BreakLong {
				data.OrderAdjust = append(data.OrderAdjust, order)
			}
		}
	}
}

func placeTurtleShort(key, secret, orderType string, data *api.TurtleData, setting *model.Setting,
	minSize float64, tick *model.BidAsk, big, canOpen bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance > 0 {
		amount = setting.GridAmount
		function = model.Close
	} else if !big {
		amount = minSize
	}
	price := data.LowDaysFar
	if orderType == model.OrderTypeLimit {
		price = data.HighDaysFar
		if setting.Chance > 0 {
			if data.UseNear {
				price = math.Min(setting.PriceX+2*data.N, data.HighDaysNear)
			} else {
				if data.LowToday > 0 {
					price = math.Min(data.LowDaysFar, data.LowToday) + 2*data.N
				} else {
					price = data.LowDaysFar + 2*data.N
				}
			}
		} else if setting.Chance < 0 {
			price = math.Max(data.HighDaysFar, setting.PriceX+data.N/2)
		}
	} else if orderType == model.OrderTypeStop {
		if setting.Chance > 0 {
			if data.UseNear {
				price = math.Max(setting.PriceX-2*data.N, data.LowDaysNear)
			} else {
				price = math.Max(data.HighDaysFar, data.HighToday) - 2*data.N
			}
		} else if setting.Chance < 0 {
			price = math.Min(data.LowDaysFar, setting.PriceX-data.N/2)
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	data.BreakShort = false
	data.WaitBreakShort = true
	priceDeal := price
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < setting.OpenShortMargin
	if data.OrderShort == nil && (setting.Chance > 0 || canOpen) {
		if orderType == model.OrderTypeStop {
			if price >= tick.Bids[0].Price {
				data.BreakShort = true
				orderType = model.OrderTypeLimit
				priceDeal = price * (1 - api.TurtleTriggerDelta/2)
			} else {
				priceDeal = price * (1 - api.TurtleTriggerDelta)
			}
		} else if orderType == model.OrderTypeLimit && price <= tick.Bids[0].Price {
			data.BreakShort = true
		}
		util.Notice(fmt.Sprintf(`place short %s %s %s %s %d %v at %e %e amt %e`,
			orderType, setting.Function, market, symbol, setting.Chance, canOpen, priceDeal, price, amount))
		data.OrderShort = api.MustPlaceOrder(key, secret, model.OrderSideSell, orderType, market, symbol, ``,
			setting.Function, priceDeal, price, amount, nil)
		if data.OrderAdjust == nil {
			data.OrderAdjust = make([]*model.Order, 0)
		}
		for _, order := range data.OrderShort {
			order.LineBuy = data.N
			order.Function = function
			go model.AppDB.Save(order)
			if data.BreakShort {
				data.OrderAdjust = append(data.OrderAdjust, order)
			}
		}
	}
}

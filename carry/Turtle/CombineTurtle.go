package Turtle

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"time"
)

// ProcessCombineTurtle
// setting.GridAmount 当前已经持仓数量
// setting.Chance 当前开仓的个数
// setting.PriceX 上一次开仓的价格
// setting.OpenShortMargin 该单币种最多开仓个数
// setting.AmountLimit 总开仓上限
// Order.GridPos 存储turtleData 的 isBig
var ProcessCombineTurtle = func(settingCombine *model.Setting, tick *model.BidAsk) {
	market := settingCombine.Market
	symbol := settingCombine.Symbol
	if !api.CheckSetProcessing(settingCombine.Function, market, symbol, true) {
		defer api.CheckSetProcessing(settingCombine.Function, market, symbol, false)
	} else {
		return
	}
	now := util.GetNowUnixMillion()
	maintaining, _ := model.ChannelMaintaining.Load(market)
	success, _, _, _ := model.GetFromStandard(market, symbol)
	if settingCombine == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 120000) ||
		!success {
		return
	}
	settingNormal := api.GetSetting(model.FunctionTurtleNormal, market, symbol)
	if settingNormal == nil {
		util.Notice(fmt.Sprintf(`combine return no normal setting from %s %s`, market, symbol))
		return
	}
	if (settingCombine.Chance != 0 && settingCombine.PriceX == 0) || (settingNormal.Chance != 0 && settingNormal.PriceX == 0) {
		util.Notice(fmt.Sprintf(`combine return no last priceX %s %s %d %e %d %e`,
			market, symbol, settingCombine.Chance, settingCombine.PriceX, settingNormal.Chance, settingNormal.PriceX))
		return
	}
	settings := []*model.Setting{settingCombine, settingNormal}
	account := model.AppConfig.GetAccounts(market)[0]
	var dataCombine, dataNormal *model.TurtleData
	dataCombine, _ = api.GetTurtleData(account, settingCombine.Function, settingCombine.Market, settingCombine.Symbol,
		settingCombine.Far, settingCombine.Near, settingCombine.Seconds, settingCombine.AmountRate, true)
	dataNormal, _ = api.GetTurtleData(account, settingNormal.Function, settingNormal.Market, settingNormal.Symbol,
		settingNormal.Far, settingNormal.Near, settingNormal.Seconds, settingNormal.AmountRate, true)
	if dataCombine == nil || dataCombine.N == 0 || dataCombine.Amount == 0 || dataNormal == nil || dataNormal.N == 0 ||
		dataNormal.Amount == 0 || settingCombine == nil || settingNormal == nil || model.AppConfig.Env == `test` {
		if time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`combine return no turtle combine turtle %s %s`, market, symbol))
		}
		return
	}
	if !dataCombine.OrderCleared {
		api.ClearOrders(account.Key, account.Secret, market, symbol)
		dataCombine.OrderCleared = true
		util.Notice(fmt.Sprintf(`combine return not cleared %s %s %v`, market, symbol, dataCombine.OrderCleared))
		return
	}
	turtleData := []*model.TurtleData{dataCombine, dataNormal}
	canOpen, turtleCoins := api.CanOpenCombine(settingCombine, settingNormal, dataCombine, dataNormal)
	//if canOpen {
	//	settingCombine.SymbolRelated = ``
	//}
	if api.HandleOrders(account.Key, account.Secret, market, symbol, settings, turtleData) ||
		api.CheckBreak(account, market, symbol, settings, turtleData, tick) {
		//util.Notice(fmt.Sprintf(`combine return handle or break %s %s`, market, symbol))
		return
	}
	//if !dataCombine.AdjustChecked && !dataNormal.AdjustChecked {
	//	util.Notice(fmt.Sprintf(`combine return not adjusted %s %s`, market, symbol))
	//	return
	//}
	//价格不一样：big=true
	//价格一样：仓数相加=0时big=false；仓数相加≠0时big=true
	model.ResetBig(settingNormal, dataCombine, dataNormal)
	msgKey := model.GetMsgKey(model.FunctionCombineTurtle, market, symbol)
	msg := fmt.Sprintf("[%d-%d %d:%d]%s N-Volume %f 可开%v 币种数:%d/%d "+
		"单仓数量:%e bid-ask %e %e \n海龟:仓数/持仓量/开仓价/今日平仓 %d of %d/%e/%e/%v %s %d big:%d 日:%e-%e %d日:%e-%e N:%e"+
		"\n龟汤:仓数/持仓量/开仓价/今日平仓 %d of %d/%e/%e/%v %s%d big:%d 日:%e-%e %d日:%e-%e N:%e",
		dataCombine.TurtleTime.Month(), dataCombine.TurtleTime.Day(), time.Now().Hour(), time.Now().Minute(), msgKey,
		dataCombine.NVolume, canOpen, int(turtleCoins), int(settingCombine.AmountLimit), dataCombine.Amount,
		tick.Bids[0].Price, tick.Asks[0].Price,
		settingNormal.Chance, settingNormal.ChanceLimit, settingNormal.GridAmount, settingNormal.PriceX, dataNormal.Liquidated,
		dataNormal.GetIds(), dataNormal.Big, dataNormal.DaysFar, dataNormal.LowDaysFar, dataNormal.HighDaysFar,
		dataNormal.DaysNear, dataNormal.LowDaysNear, dataNormal.HighDaysNear, dataNormal.N,
		settingCombine.Chance, settingCombine.ChanceLimit, settingCombine.GridAmount, settingCombine.PriceX,
		dataCombine.Liquidated, dataCombine.GetIds(), dataCombine.Big, dataCombine.DaysFar, dataCombine.LowDaysFar,
		dataCombine.HighDaysFar, dataCombine.DaysNear, dataCombine.LowDaysNear, dataCombine.HighDaysNear, dataCombine.N)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	placeTurtleLong(account, model.OrderTypeStop, dataNormal, settingNormal, tick, canOpen)
	placeTurtleShort(account, model.OrderTypeStop, dataNormal, settingNormal, tick, canOpen)
	placeTurtleLong(account, model.OrderTypeLimit, dataCombine, settingCombine, tick, canOpen)
	placeTurtleShort(account, model.OrderTypeLimit, dataCombine, settingCombine, tick, canOpen)
	if handleAllBreak(settings, turtleData) {
		api.ClearExtraOrders(account.Key, account.Secret, market, symbol, turtleData)
	}
}

func handleAllBreak(settings []*model.Setting, turtles []*model.TurtleData) (needCheck bool) {
	if settings == nil || len(settings) != 2 || turtles == nil || len(turtles) != 2 {
		return false
	}
	// 每次只检查一个，如果同时检查多个，会导致一个里面更新的isBig在另一个里面没有更新
	if handleBreakLong(settings[0], settings[1], turtles[0]) {
		needCheck = true
	}
	if handleBreakShort(settings[0], settings[1], turtles[0]) {
		needCheck = true
	}
	if handleBreakLong(settings[1], settings[0], turtles[1]) {
		needCheck = true
	}
	if handleBreakShort(settings[1], settings[0], turtles[1]) {
		needCheck = true
	}
	return
}

func handleBreakLong(setting, settingOpposite *model.Setting, data *model.TurtleData) (work bool) {
	if data == nil || data.OrderLong == nil || len(data.OrderLong) == 0 || !data.BreakLong {
		return false
	}
	if data.OrderLong[0].TriggerPrice > 0 {
		setting.PriceX = data.OrderLong[0].TriggerPrice
	} else {
		setting.PriceX = data.OrderLong[0].Price
	}
	util.Notice(fmt.Sprintf(`query %s break buy %s %s %d %s %s chances %d %d`,
		setting.Function, setting.Market, setting.Symbol, len(data.OrderLong), data.OrderLong[0].OrderId,
		data.OrderLong[0].Function, setting.Chance, settingOpposite.Chance))
	if data.OrderLong[0].Function == model.Close {
		msg := fmt.Sprintf(`liquidate long %s %s chance:%d Amount:%e px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, setting.PriceX, data.N)
		go api.SendMails(`平多`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		setting.PriceX = 0
	} else if data.OrderLong[0].Function == model.Open {
		util.Notice(fmt.Sprintf(`加多 %s %s chance:%d Amount:%e px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, setting.PriceX, data.N))
		setting.Chance++
		for _, order := range data.OrderLong {
			setting.GridAmount += order.Amount
		}
	}
	for _, order := range data.OrderLong {
		data.OrderAdjust = append(data.OrderAdjust, order)
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

func handleBreakShort(setting, settingOpposite *model.Setting, data *model.TurtleData) (work bool) {
	if data == nil || data.OrderShort == nil || len(data.OrderShort) == 0 || !data.BreakShort {
		return false
	}
	if data.OrderShort[0].TriggerPrice > 0 {
		setting.PriceX = data.OrderShort[0].TriggerPrice
	} else {
		setting.PriceX = data.OrderShort[0].Price
	}
	util.Notice(fmt.Sprintf(`query %s break sell %s %s %d %s %s chances %d %d`,
		setting.Function, setting.Market, setting.Symbol, len(data.OrderShort), data.OrderShort[0].OrderId,
		data.OrderShort[0].Function, setting.Chance, settingOpposite.Chance))
	if data.OrderShort[0].Function == model.Close {
		msg := fmt.Sprintf(`liquidate short: %s %s chance:%d Amount:%e px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, setting.PriceX, data.N)
		go api.SendMails(`平空`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		setting.PriceX = 0
	} else {
		util.Notice(fmt.Sprintf(`加空 %s %s chance:%d Amount:%e px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, setting.PriceX, data.N))
		setting.Chance--
		for _, order := range data.OrderShort {
			setting.GridAmount += order.Amount
		}
	}
	for _, order := range data.OrderShort {
		data.OrderAdjust = append(data.OrderAdjust, order)
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

func placeTurtleLong(account *model.Account, orderType string, data *model.TurtleData, setting *model.Setting,
	tick *model.BidAsk, canOpen bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance < 0 {
		function = model.Close
		amount = setting.GridAmount
	} else if data.Big == -1 {
		amount = data.AmountMin
	}
	price := data.HighDaysFar
	if orderType == model.OrderTypeLimit {
		price = data.LowDaysFar + data.N/2
		if setting.Chance > 0 {
			price = math.Min(data.LowDaysFar, setting.PriceX-data.N/2)
		} else if setting.Chance < 0 {
			if data.UseNear {
				price = math.Max(setting.PriceX-2*data.N, data.LowDaysNear)
			} else {
				//price = math.Max(data.HighDaysFar, data.HighToday) - 2*data.N
				price = math.Max(data.HighDaysFar, setting.PriceX) - 2*data.N
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
	priceDeal := price
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < float64(setting.ChanceLimit)
	if data.OrderLong == nil && (setting.Chance < 0 || canOpen) {
		data.BreakLong = false
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
			priceDeal = tick.Asks[0].Price * (1 + api.TurtleTriggerDelta/2)
		}
		if data.BreakLong && data.Big == -1 && setting.Chance >= 0 {
			util.Notice(fmt.Sprintf(`already break place fake long %s %s %s`, orderType, setting.Function, symbol))
			data.OrderLong = []*model.Order{{
				Amount:       amount,
				DealAmount:   amount,
				DealPrice:    priceDeal,
				OrderId:      fmt.Sprintf(`fake%d`, time.Now().UnixNano()),
				LineBuy:      data.N,
				LineSell:     data.N,
				Price:        priceDeal,
				TriggerPrice: price,
				AccountIndex: account.Index,
				Market:       market,
				OrderSide:    model.OrderSideBuy,
				OrderType:    orderType,
				RefreshType:  setting.Function,
				Status:       model.CarryStatusSuccess,
				Symbol:       symbol}}
		} else {
			data.OrderLong = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, orderType, market, symbol, ``,
				setting.Function, priceDeal, price, amount, nil)
		}
		if data.OrderAdjust == nil {
			data.OrderAdjust = make([]*model.Order, 0)
		}
		util.Notice(fmt.Sprintf(`place long %s %s %s %v %d %v at %e %e amt %e %d`,
			orderType, setting.Function, symbol, data.OrderLong, setting.Chance, canOpen, priceDeal, price, amount, len(data.OrderLong)))
		for _, order := range data.OrderLong {
			order.LineBuy = data.N
			order.LineSell = data.N
			order.Function = function
			order.GridPos = data.Big
			go model.AppDB.Save(order)
			if data.BreakLong && order.Status != model.CarryStatusSuccess {
				util.Notice(`already break long move to adjust %s %v`, order.OrderId, order)
				data.OrderAdjust = append(data.OrderAdjust, order)
			}
		}
	}
}

func placeTurtleShort(account *model.Account, orderType string, data *model.TurtleData, setting *model.Setting,
	tick *model.BidAsk, canOpen bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance > 0 {
		amount = setting.GridAmount
		function = model.Close
	} else if data.Big == -1 {
		amount = data.AmountMin
	}
	price := data.LowDaysFar
	if orderType == model.OrderTypeLimit {
		price = data.HighDaysFar - data.N/2
		if setting.Chance > 0 {
			if data.UseNear {
				price = math.Min(setting.PriceX+2*data.N, data.HighDaysNear)
			} else {
				//if data.LowToday > 0 {
				//	price = math.Min(data.LowDaysFar, data.LowToday) + 2*data.N
				//} else {
				//	price = data.LowDaysFar + 2*data.N
				//}
				if setting.PriceX > 0 {
					price = math.Min(data.LowDaysFar, setting.PriceX) + 2*data.N
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
	priceDeal := price
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < float64(setting.ChanceLimit)
	if data.OrderShort == nil && (setting.Chance > 0 || canOpen) {
		data.BreakShort = false
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
			priceDeal = tick.Bids[0].Price * (1 - api.TurtleTriggerDelta/2)
		}
		util.Notice(fmt.Sprintf(`place short %s %s %s %s %d %v at %e %e amt %e`,
			orderType, setting.Function, market, symbol, setting.Chance, canOpen, priceDeal, price, amount))
		if data.BreakShort && data.Big == -1 && setting.Chance <= 0 {
			util.Notice(fmt.Sprintf(`already break place fake short %s %s %s`, orderType, setting.Function, symbol))
			data.OrderShort = []*model.Order{{
				Amount:       amount,
				DealAmount:   amount,
				DealPrice:    priceDeal,
				OrderId:      fmt.Sprintf(`fake%d`, time.Now().UnixNano()),
				LineBuy:      data.N,
				LineSell:     data.N,
				Price:        priceDeal,
				TriggerPrice: price,
				AccountIndex: account.Index,
				Market:       market,
				OrderSide:    model.OrderSideSell,
				OrderType:    orderType,
				RefreshType:  setting.Function,
				Status:       model.CarryStatusSuccess,
				Symbol:       symbol}}
		} else {
			data.OrderShort = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, orderType, market, symbol, ``,
				setting.Function, priceDeal, price, amount, nil)
		}
		if data.OrderAdjust == nil {
			data.OrderAdjust = make([]*model.Order, 0)
		}
		for _, order := range data.OrderShort {
			order.LineBuy = data.N
			order.LineSell = data.N
			order.Function = function
			order.GridPos = data.Big
			go model.AppDB.Save(order)
			if data.BreakShort && order.Status != model.CarryStatusSuccess {
				util.Notice(`already break short move to adjust %s %v`, order.OrderId, order)
				data.OrderAdjust = append(data.OrderAdjust, order)
			}
		}
	}
}

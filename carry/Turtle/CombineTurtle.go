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
	if settingCombine == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 10000) ||
		(time.Now().Hour() == 0 && time.Now().Minute() == 0) || len(strings.Trim(symbol, ` `)) == 0 {
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
	dataCombine := api.GetTurtleData(account.Key, account.Secret, settingCombine.Function, market, symbol)
	dataNormal := api.GetTurtleData(account.Key, account.Secret, model.FunctionTurtleNormal, market, symbol)
	if dataCombine == nil || dataCombine.N == 0 || dataCombine.Amount == 0 ||
		dataNormal == nil || dataNormal.N == 0 || dataNormal.Amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`combine return no turtle combine & turtle %s %s`, market, symbol))
		}
		return
	}
	dataNormal.N = dataCombine.N
	dataNormal.Amount = dataCombine.Amount
	if !dataCombine.OrderCleared {
		api.ClearOrders(account.Key, account.Secret, market, symbol)
		dataCombine.OrderCleared = true
		util.Notice(fmt.Sprintf(`combine return not cleared %s %s %v`, market, symbol, dataCombine.OrderCleared))
		return
	}
	turtleData := []*api.TurtleData{dataCombine, dataNormal}
	canOpen, turtleCoins := api.CanOpenCombine(settingCombine, settingNormal, dataCombine, dataNormal)
	if canOpen {
		settingCombine.SymbolRelated = ``
	}
	if api.HandleOrders(account.Key, account.Secret, market, symbol, settings, turtleData) ||
		api.CheckBreak(account.Key, account.Secret, market, symbol, settings, turtleData, tick) {
		util.Notice(fmt.Sprintf(`combine return handle or break %s %s`, market, symbol))
		return
	}
	if !dataCombine.AdjustChecked && !dataNormal.AdjustChecked {
		util.Notice(fmt.Sprintf(`combine return not adjusted %s %s`, market, symbol))
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
		minSize = math.Max(minSize, 2*marketInfo.MoneyMin/dataCombine.LowDaysFar)
	}
	//价格不一样：big=true
	//价格一样：仓数相加=0时big=false；仓数相加≠0时big=true
	isBig := dataCombine.IsBig(settingCombine, settingNormal, marketInfo)
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionCombineTurtle, market, symbol)
	msg := fmt.Sprintf("[%d-%d %d:%d]%s可开%vbig:%d 币种数:%d/%d %d日:%e-%e %d日:%e-%e N:%e 仓数上限%d 单仓数量:%e bid-ask %e %e \n"+
		"海龟:仓数/持仓量/开仓价/今日平仓 %d/%e/%e/%v %s\n龟汤:仓数/持仓量/开仓价/今日平仓 %d/%e/%e/%v %s",
		dataCombine.TurtleTime.Month(), dataCombine.TurtleTime.Day(), time.Now().Hour(), time.Now().Minute(), msgKey, canOpen, isBig,
		int(turtleCoins), int(settingCombine.AmountLimit), dataCombine.DaysFar, dataCombine.LowDaysFar, dataCombine.HighDaysFar,
		dataCombine.DaysNear, dataCombine.LowDaysNear, dataCombine.HighDaysNear, dataCombine.N, int(settingCombine.OpenShortMargin),
		dataCombine.Amount, tick.Bids[0].Price, tick.Asks[0].Price, settingNormal.Chance, settingNormal.GridAmount,
		settingNormal.PriceX, dataNormal.Liquidated, dataNormal.GetIds(), settingCombine.Chance, settingCombine.GridAmount,
		settingCombine.PriceX, dataCombine.Liquidated, dataCombine.GetIds())
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	placeTurtleLong(account, model.OrderTypeStop, dataNormal, settingNormal, minSize, tick, isBig, canOpen)
	placeTurtleShort(account, model.OrderTypeStop, dataNormal, settingNormal, minSize, tick, isBig, canOpen)
	placeTurtleLong(account, model.OrderTypeLimit, dataCombine, settingCombine, minSize, tick, isBig, canOpen)
	placeTurtleShort(account, model.OrderTypeLimit, dataCombine, settingCombine, minSize, tick, isBig, canOpen)
	needCheck := false
	// 每次只检查一个，如果同时检查多个，会导致一个里面更新的isBig在另一个里面没有更新
	if handleBreakLong(settingCombine, settingNormal, dataCombine, dataNormal, turtleCoins, isBig) {
		needCheck = true
		isBig = dataCombine.IsBig(settingCombine, settingNormal, marketInfo)
	}
	if handleBreakShort(settingCombine, settingNormal, dataCombine, dataNormal, turtleCoins, isBig) {
		needCheck = true
		isBig = dataCombine.IsBig(settingCombine, settingNormal, marketInfo)
	}
	if handleBreakLong(settingNormal, settingCombine, dataNormal, dataCombine, turtleCoins, isBig) {
		needCheck = true
		isBig = dataCombine.IsBig(settingCombine, settingNormal, marketInfo)
	}
	if handleBreakShort(settingNormal, settingCombine, dataNormal, dataCombine, turtleCoins, isBig) {
		needCheck = true
	}
	if needCheck {
		api.ClearExtraOrders(account.Key, account.Secret, market, symbol, turtleData)
	}
}

func handleBreakLong(setting, settingOpposite *model.Setting, data, dataOpposite *api.TurtleData,
	turtleCoins float64, big int) (work bool) {
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
		msg := fmt.Sprintf(`liquidate long %s %s chance:%d Amount:%e currentN:%d px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, int(turtleCoins), setting.PriceX, data.N)
		go api.SendMails(`平多`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		setting.PriceX = 0
		bigNew := -1
		if settingOpposite.Chance != 0 {
			bigNew = 1
		}
		data.SetBig(bigNew)
		dataOpposite.SetBig(bigNew)
		if big != bigNew {
			removeOpenOrders(dataOpposite)
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
	turtleCoins float64, big int) (work bool) {
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
		msg := fmt.Sprintf(`liquidate short: %s %s chance:%d Amount:%e currentN:%d px:%e N:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, int(turtleCoins), setting.PriceX, data.N)
		go api.SendMails(`平空`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		setting.PriceX = 0
		bigNew := -1
		if settingOpposite.Chance != 0 {
			bigNew = 1
		}
		data.SetBig(bigNew)
		dataOpposite.SetBig(bigNew)
		if bigNew != big {
			removeOpenOrders(dataOpposite)
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

func removeOpenOrders(data *api.TurtleData) {
	util.Notice(fmt.Sprintf(`remove open orders %s`, data.Symbol))
	if data != nil && data.OrderLong != nil && len(data.OrderLong) > 0 && data.OrderLong[0].Function == model.Open {
		data.OrderLong = nil
	}
	if data != nil && data.OrderShort != nil && len(data.OrderShort) > 0 && data.OrderShort[0].Function == model.Open {
		data.OrderShort = nil
	}
}

func placeTurtleLong(account *model.Account, orderType string, data *api.TurtleData, setting *model.Setting,
	minSize float64, tick *model.BidAsk, big int, canOpen bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance < 0 {
		function = model.Close
		amount = setting.GridAmount
	} else if big == -1 {
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
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < setting.OpenShortMargin
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
		}
		if data.BreakLong && big == -1 && setting.Chance >= 0 {
			util.Notice(fmt.Sprintf(`already break place fake long %s %s %s`, orderType, setting.Function, symbol))
			data.OrderLong = []*model.Order{{
				Amount:       amount,
				DealAmount:   amount,
				DealPrice:    priceDeal,
				OrderId:      fmt.Sprintf(`fake%d`, time.Now().UnixNano()),
				LineBuy:      data.N,
				LineSell:     data.N,
				Price:        price,
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
			go model.AppDB.Save(order)
			if data.BreakLong && order.Status != model.CarryStatusSuccess {
				util.Notice(`already break long move to adjust %s %v`, order.OrderId, order)
				data.OrderAdjust = append(data.OrderAdjust, order)
			}
		}
	}
}

func placeTurtleShort(account *model.Account, orderType string, data *api.TurtleData, setting *model.Setting,
	minSize float64, tick *model.BidAsk, big int, canOpen bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance > 0 {
		amount = setting.GridAmount
		function = model.Close
	} else if big == -1 {
		amount = minSize
	}
	price := data.LowDaysFar
	if orderType == model.OrderTypeLimit {
		price = data.HighDaysFar
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
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < setting.OpenShortMargin
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
		}
		util.Notice(fmt.Sprintf(`place short %s %s %s %s %d %v at %e %e amt %e`,
			orderType, setting.Function, market, symbol, setting.Chance, canOpen, priceDeal, price, amount))
		if data.BreakShort && big == -1 && setting.Chance <= 0 {
			util.Notice(fmt.Sprintf(`already break place fake short %s %s %s`, orderType, setting.Function, symbol))
			data.OrderShort = []*model.Order{{
				Amount:       amount,
				DealAmount:   amount,
				DealPrice:    priceDeal,
				OrderId:      fmt.Sprintf(`fake%d`, time.Now().UnixNano()),
				LineBuy:      data.N,
				LineSell:     data.N,
				Price:        price,
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
			go model.AppDB.Save(order)
			if data.BreakShort && order.Status != model.CarryStatusSuccess {
				util.Notice(`already break short move to adjust %v`, order.OrderId, order)
				data.OrderAdjust = append(data.OrderAdjust, order)
			}
		}
	}
}

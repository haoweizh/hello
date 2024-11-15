package Turtle

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"time"
)

// ProcessTurtle
// setting.GridAmount 当前已经持仓数量
// setting.Chance 当前开仓的个数
// setting.PriceX 上一次开仓的价格
// setting.OpenShortMargin 该单币种最多开仓个数
// setting.AmountLimit 总开仓上限
var ProcessTurtle = func(setting *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, true) {
		defer api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, false)
	} else {
		return
	}
	now := util.GetNowUnixMillion()
	maintaining, ok := model.ChannelMaintaining.Load(setting.Market)
	if setting == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		(ok && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 120000) {
		return
	}
	if setting.Chance != 0 && setting.PriceX == 0 {
		util.Notice(fmt.Sprintf(`no last priceX %s %s %d %e`,
			setting.Market, setting.Symbol, setting.Chance, setting.PriceX))
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	var data *model.TurtleData
	data, _ = api.GetTurtleData(account, setting.Function, setting.Market, setting.Symbol, setting.Far, setting.Near,
		setting.Seconds, setting.ChanceLimit, setting.AmountRate, true, setting.Chance == 0 && setting.SymbolRelated == model.SettingTurtleRemoved)
	if data == nil || setting == nil || model.AppConfig.Env == `test` || time.Now().After(data.Expire) {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle %s %s`, setting.Market, setting.Symbol))
		}
		return
	}
	if !data.OrderCleared {
		api.ClearOrders(account.Key, account.Secret, setting.Market, setting.Symbol, map[string]bool{model.OrderTypeTrailStop: true})
		data.OrderCleared = true
		return
	}
	canOpenTurtle, chanceInAll := api.CanOpenTurtle(setting, data)
	msgKey := model.GetMsgKey(setting.Function, setting.Market, setting.Symbol)
	msg := fmt.Sprintf("[%d:%d %d:%d]%s N-Volume %f 可开%v 当前已经持仓数量:%e 持仓数/限制:%d/%d "+
		"总仓数币数/仓数币数限制:%d %d 上一次开仓的价格:%e "+
		"%d日:%e-%e %d日:%e-%e N:%e 单次数量:%e bid-ask %e %e 当日有平仓：%v",
		data.TurtleTime.Month(), data.TurtleTime.Day(), time.Now().Hour(), time.Now().Minute(), msgKey, data.NVolume,
		canOpenTurtle, setting.GridAmount, setting.Chance, int(setting.ChanceLimit), int(chanceInAll),
		int(setting.AmountLimit), setting.PriceX, data.DaysFar, data.LowFar, data.HighFar, data.DaysNear,
		data.LowNear, data.HighNear, data.N, data.Amount, tick.Bids[0].Price, tick.Asks[0].Price, data.Liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	priceLong := data.HighFar
	priceShort := data.LowFar
	if api.HandleOrders(account.Key, account.Secret, setting.Market, setting.Symbol, []*model.Setting{setting}, []*model.TurtleData{data}, tick) ||
		api.CheckBreak(account, setting.Market, setting.Symbol, []*model.Setting{setting}, []*model.TurtleData{data}, tick) {
		return
	}
	if data.N == 0 || data.Amount == 0 {
		return
	}
	//if !data.AdjustChecked {
	//	return
	//}
	priceChange := 2 * data.N
	//if setting.Seconds == 14400 {
	//	priceChange = 2.5 * data.N
	//}
	if setting.Chance == 0 { // 开初始仓
		placeTurtleOrders(account.Key, account.Secret, data, setting, canOpenTurtle, chanceInAll, priceShort, priceLong, tick)
		if data.BreakLong && data.OrderLong != nil {
			handleTurtleBreak(account.Key, account.Secret, setting, data, model.OrderSideBuy)
			setting.Chance = 1
			setting.GridAmount = data.Amount
			model.AppDB.Model(setting).Where("market= ? and Symbol= ? and function= ?",
				setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破%d日高点 %s %s chance:%d Amount:%e chanceInAll:%e short-long:%e %e px:%e N:%e`,
				data.DaysFar, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll,
				priceShort, priceLong, setting.PriceX, data.N))
		}
		if data.BreakShort && data.OrderShort != nil {
			handleTurtleBreak(account.Key, account.Secret, setting, data, model.OrderSideSell)
			setting.Chance = -1
			setting.GridAmount = data.Amount
			model.AppDB.Model(setting).Where("market= ? and Symbol= ? and function= ?",
				setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破%d日低点 %s %s chance:%d Amount:%e chanceInAll:%d short-long:%e %e px:%e N:%e`,
				data.DaysNear, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, int(chanceInAll),
				priceShort, priceLong, setting.PriceX, data.N))
		}
	} else if setting.Chance > 0 {
		priceLong = math.Max(priceLong, setting.PriceX+data.N/2)
		if data.UseNear {
			priceShort = math.Max(setting.PriceX-priceChange, data.LowNear)
		} else {
			priceShort = math.Max(data.HighFar, data.HighLast) - priceChange
		}
		placeTurtleOrders(account.Key, account.Secret, data, setting, canOpenTurtle, chanceInAll, priceShort, priceLong, tick)
		// 加仓一个单位
		if data.BreakLong && data.OrderLong != nil {
			handleTurtleBreak(account.Key, account.Secret, setting, data, model.OrderSideBuy)
			setting.Chance = setting.Chance + 1
			setting.GridAmount = setting.GridAmount + data.Amount
			model.AppDB.Model(setting).Where("market= ? and Symbol= ? and function= ?",
				setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加多 %s %s chance:%d Amount:%e chanceInAll:%e short-long:%e %e px:%e N:%e`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll, priceShort, priceLong,
				setting.PriceX, data.N))
		}
		// 平多
		if data.BreakShort && data.OrderShort != nil {
			handleTurtleBreak(account.Key, account.Secret, setting, data, model.OrderSideSell)
			go api.SendMails(`平多`+setting.Market+setting.Symbol,
				fmt.Sprintf(`止盈止损at%e 仓位%d 数量 %e`, priceShort, setting.Chance, setting.GridAmount))
			data.Liquidated = true
			setting.Chance = 0
			setting.GridAmount = 0
			setting.PriceX = 0
			model.AppDB.Model(setting).Where("market= ? and Symbol= ? and function= ?",
				setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`liquidate long %s %s chance:%d Amount:%e chanceInAll:%d short-long:%e %e px:%e N:%e`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, int(chanceInAll), priceShort, priceLong,
				setting.PriceX, data.N))
		}
	} else if setting.Chance < 0 {
		priceShort = math.Min(priceShort, setting.PriceX-data.N/2)
		if data.UseNear {
			priceLong = math.Min(setting.PriceX+priceChange, data.HighNear)
		} else {
			if data.LowLast > 0 {
				priceLong = math.Min(data.LowFar, data.LowLast) + priceChange
			} else {
				priceLong = data.LowFar + priceChange
			}
		}
		placeTurtleOrders(account.Key, account.Secret, data, setting, canOpenTurtle, chanceInAll, priceShort, priceLong, tick)
		// 加仓一个单位
		if data.BreakShort && data.OrderShort != nil {
			handleTurtleBreak(account.Key, account.Secret, setting, data, model.OrderSideSell)
			setting.Chance = setting.Chance - 1
			setting.GridAmount = setting.GridAmount + data.Amount
			model.AppDB.Model(setting).Where("market= ? and Symbol= ? and function= ?",
				setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加空 %s %s chance:%d Amount:%e chanceInAll:%d short-long:%e %e px:%e N:%e`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, int(chanceInAll), priceShort, priceLong,
				setting.PriceX, data.N))
		} // liquidate short
		if data.BreakLong && data.OrderLong != nil {
			handleTurtleBreak(account.Key, account.Secret, setting, data, model.OrderSideBuy)
			go api.SendMails(`平空`+setting.Market+setting.Symbol,
				fmt.Sprintf(`止盈止损at%e 仓位%d 数量 %e`,
					priceLong, setting.Chance, setting.GridAmount))
			setting.Chance = 0
			setting.GridAmount = 0
			setting.PriceX = 0
			data.Liquidated = true
			model.AppDB.Model(setting).Where("market= ? and Symbol= ? and function= ?",
				setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`liquidate short result: %s %s chance:%d Amount:%e chanceInAll:%d short-long:%e %e px:%e N:%e`,
				setting.Market, setting.Symbol, int(setting.Chance), setting.GridAmount, int(chanceInAll), priceShort, priceLong,
				setting.PriceX, data.N))
		}
	}
}

func handleTurtleBreak(key, secret string, setting *model.Setting, turtleData *model.TurtleData, orderSide string) {
	if turtleData == nil {
		//util.Notice(fmt.Sprintf(`fatal error, nil order to break`))
		return
	}
	orderQuery := turtleData.OrderShort
	orderCancel := turtleData.OrderLong
	if orderSide == model.OrderSideBuy {
		orderQuery = turtleData.OrderLong
		orderCancel = turtleData.OrderShort
	}
	if orderQuery != nil && len(orderQuery) > 0 {
		time.Sleep(time.Second * 3)
		util.Notice(fmt.Sprintf(`query turtle break %s %s %s %d`,
			setting.Market, setting.Symbol, orderSide, len(orderQuery)))
		turtleTriggerDelta := api.GetTurtleTriggerDelta(setting.Market)
		setting.PriceX = orderQuery[0].Price / (1 + turtleTriggerDelta)
		if orderQuery[0].OrderSide == model.OrderSideSell {
			setting.PriceX = orderQuery[0].Price / (1 - turtleTriggerDelta)
		}
		if orderQuery[0].TriggerPrice > 0 {
			setting.PriceX = orderQuery[0].TriggerPrice
		}
		for _, order := range orderQuery {
			turtleData.OrderAdjust[order.OrderId] = order
		}
		turtleData.OrderLong = nil
		turtleData.OrderShort = nil
		if orderCancel != nil {
			for _, order := range orderCancel {
				temp := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId)
				if temp != nil && temp.Status == model.CarryStatusWorking {
					go api.MustCancel(key, secret, order.Market, order.Symbol, order.OrderType, order.OrderId, false)
				}
			}
		}
		util.Notice(fmt.Sprintf(`clear %s %s opp-%s %v`, setting.Market, setting.Symbol, orderSide, orderCancel))
	}
}

func placeTurtleOrders(key, secret string, turtleData *model.TurtleData, setting *model.Setting, canOpen bool, chanceInAll float64,
	priceShort, priceLong float64, tick *model.BidAsk) {
	coinLimit := setting.ChanceLimit
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < float64(setting.ChanceLimit)
	if turtleData.OrderLong == nil && (canOpen || setting.Chance < 0) {
		orderSide := model.OrderSideBuy
		typeLong := model.OrderTypeStop
		amount := turtleData.Amount
		if setting.Chance < 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平空 %s %s chance:%d Amount:%e chanceInAll:%d short-long:%e %e px:%e N:%e`,
				setting.Market, setting.Symbol, setting.Chance, amount, int(chanceInAll), priceShort,
				priceLong, setting.PriceX, turtleData.N))
		}
		util.Notice(fmt.Sprintf(`%s %s place多单 at %e chance:%d Amount:%e priceX:%e chanceInAll-limit:%d %d
			orderSide:%s h%d:%e h%d:%e l%d:%e l%d:%e coin limit:%d`,
			setting.Market, setting.Symbol, priceLong, setting.Chance, amount, setting.PriceX, int(chanceInAll), int(setting.AmountLimit),
			orderSide, turtleData.DaysFar, turtleData.HighFar, turtleData.DaysNear, turtleData.HighNear,
			turtleData.DaysFar, turtleData.LowFar, turtleData.DaysNear, turtleData.LowNear, coinLimit))
		turtleData.BreakLong = false
		turtleTriggerDelta := api.GetTurtleTriggerDelta(setting.Market)
		if priceLong <= tick.Asks[0].Price {
			turtleData.OrderLong = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
				setting.Function, priceLong*(1+turtleTriggerDelta), priceLong, amount, setting)
			for _, order := range turtleData.OrderLong {
				turtleData.OrderAdjust[order.OrderId] = order
			}
			turtleData.BreakLong = true
		} else {
			turtleData.OrderLong = api.MustPlaceOrder(key, secret, orderSide, typeLong, setting.Market, setting.Symbol, ``,
				setting.Function, priceLong*(1+turtleTriggerDelta), priceLong, amount, setting)
		}
		if turtleData.OrderLong != nil {
			for _, order := range turtleData.OrderLong {
				order.LineBuy = turtleData.N
				go model.AppDB.Save(order)
			}
		}
	}
	if turtleData.OrderShort == nil && (canOpen || setting.Chance > 0) {
		orderSide := model.OrderSideSell
		typeShort := model.OrderTypeStop
		amount := turtleData.Amount
		if setting.Chance > 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平多 %s %s chance:%d Amount:%e currentNum:%d short-long:%e %e px:%e N:%e`,
				setting.Market, setting.Symbol, setting.Chance, amount, int(chanceInAll), priceShort, priceLong, setting.PriceX, turtleData.N))
		}
		util.Notice(fmt.Sprintf(`%s %s place空单 at %e chance:%d Amount:%e priceX:%e currentNum-limit:%d %d 
				orderSide:%s h%d:%e h%d:%e l%d:%e l%d:%e coin limit:%d`,
			setting.Market, setting.Symbol, priceShort, setting.Chance, amount, setting.PriceX, int(chanceInAll), int(setting.AmountLimit),
			orderSide, turtleData.DaysFar, turtleData.HighFar, turtleData.DaysNear, turtleData.HighNear,
			turtleData.DaysFar, turtleData.LowFar, turtleData.DaysNear, turtleData.LowNear, coinLimit))
		turtleData.BreakShort = false
		turtleTriggerDelta := api.GetTurtleTriggerDelta(setting.Market)
		if priceShort >= tick.Bids[0].Price {
			turtleData.OrderShort = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
				setting.Function, priceShort*(1-turtleTriggerDelta), priceShort, amount, setting)
			for _, order := range turtleData.OrderShort {
				turtleData.OrderAdjust[order.OrderId] = order
			}
			turtleData.BreakShort = true
		} else {
			turtleData.OrderShort = api.MustPlaceOrder(key, secret, orderSide, typeShort, setting.Market, setting.Symbol, ``,
				setting.Function, priceShort*(1-turtleTriggerDelta), priceShort, amount, setting)
		}
		if turtleData.OrderShort != nil {
			for _, order := range turtleData.OrderShort {
				order.LineBuy = turtleData.N
				go model.AppDB.Save(order)
			}
		}
	}
}

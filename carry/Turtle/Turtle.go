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
	data := GetTurtleData(account.Key, account.Secret, setting.Function, setting.Market, setting.Symbol, true)
	if data == nil || data.n == 0 || data.amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle %s %s`, setting.Market, setting.Symbol))
		}
		return
	}
	if !data.orderCleared {
		clearOrders(account.Key, account.Secret, setting.Market, setting.Symbol)
		data.orderCleared = true
		return
	}
	chanceValid, chanceInAll := checkChance(setting)
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionTurtle, setting.Market, setting.Symbol)
	msg := fmt.Sprintf("[海龟参数]%s %s 次数限制:%f 当前已经持仓数量:%f 上一次开仓的价格:%f "+
		"%d日:%f-%f %d日:%f-%f n:%e 数量:%f %s 持仓数/限制:%d/%f 总持仓数%f bid-ask %f %f 当日有平仓：%v",
		data.turtleTime.String()[0:10], msgKey, setting.AmountLimit, setting.GridAmount, setting.PriceX,
		data.daysFar, data.lowDaysFar, data.highDaysFar, data.daysNear, data.lowDaysNear,
		data.highDaysNear, data.n, data.amount, setting.Symbol, setting.Chance,
		setting.OpenShortMargin, chanceInAll, tick.Bids[0].Price, tick.Asks[0].Price, data.liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	priceLong := data.highDaysFar
	priceShort := data.lowDaysFar
	if handleTraceOrders(account.Key, account.Secret, setting.Market, setting.Symbol, []*model.Setting{setting}, []*Data{data}, chanceInAll) ||
		checkBreak(account.Key, account.Secret, setting.Market, setting.Symbol, []*model.Setting{setting}, []*Data{data}, tick) {
		return
	}
	if !data.adjustChecked {
		return
	}
	if setting.Chance == 0 && !data.liquidated { // 开初始仓
		placeTurtleOrders(account.Key, account.Secret, data, setting, chanceValid, chanceInAll, priceShort, priceLong, tick)
		if data.breakLong && data.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideBuy)
			setting.Chance = 1
			setting.GridAmount = data.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破%d日高点 %s %s chance:%d amount:%f chanceInAll:%f short-long:%f %f px:%f n:%e`,
				data.daysFar, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll,
				priceShort, priceLong, setting.PriceX, data.n))
		}
		if data.breakShort && data.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideSell)
			setting.Chance = -1
			setting.GridAmount = data.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破%d日低点 %s %s chance:%d amount:%f chanceInAll:%f short-long:%f %f px:%f n:%e`,
				data.daysNear, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll,
				priceShort, priceLong, setting.PriceX, data.n))
		}
	} else if setting.Chance > 0 {
		priceLong = math.Max(priceLong, setting.PriceX+data.n/2)
		if data.useNear {
			priceShort = math.Max(setting.PriceX-2*data.n, data.lowDaysNear)
		} else {
			priceShort = math.Max(data.highDaysFar, data.highToday) - 2*data.n
		}
		placeTurtleOrders(account.Key, account.Secret, data, setting, chanceValid, chanceInAll, priceShort, priceLong, tick)
		// 加仓一个单位
		if data.breakLong && data.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideBuy)
			setting.Chance = setting.Chance + 1
			setting.GridAmount = setting.GridAmount + data.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加多 %s %s chance:%d amount:%f chanceInAll:%f short-long:%f %f px:%f n:%e`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll, priceShort, priceLong,
				setting.PriceX, data.n))
		}
		// 平多
		if data.breakShort && data.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideSell)
			go api.SendMails(`平多`+setting.Market+setting.Symbol,
				fmt.Sprintf(`止盈止损at%f 仓位%d 数量 %f`, priceShort, setting.Chance, setting.GridAmount))
			data.liquidated = true
			setting.Chance = 0
			setting.GridAmount = 0
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`liquidate long %s %s chance:%d amount:%f chanceInAll:%f short-long:%f %f px:%f n:%e`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll, priceShort, priceLong,
				setting.PriceX, data.n))
		}
	} else if setting.Chance < 0 {
		priceShort = math.Min(priceShort, setting.PriceX-data.n/2)
		if data.useNear {
			priceLong = math.Min(setting.PriceX+2*data.n, data.highDaysNear)
		} else {
			if data.lowToday > 0 {
				priceLong = math.Min(data.lowDaysFar, data.lowToday) + 2*data.n
			} else {
				priceLong = data.lowDaysFar + 2*data.n
			}
		}
		placeTurtleOrders(account.Key, account.Secret, data, setting, chanceValid, chanceInAll, priceShort, priceLong, tick)
		// 加仓一个单位
		if data.breakShort && data.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideSell)
			setting.Chance = setting.Chance - 1
			setting.GridAmount = setting.GridAmount + data.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加空 %s %s chance:%d amount:%f chanceInAll:%f short-long:%f %f px:%f n:%e`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll, priceShort, priceLong,
				setting.PriceX, data.n))
		} // liquidate short
		if data.breakLong && data.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideBuy)
			go api.SendMails(`平空`+setting.Market+setting.Symbol,
				fmt.Sprintf(`止盈止损at%f 仓位%d 数量 %f`,
					priceLong, setting.Chance, setting.GridAmount))
			setting.Chance = 0
			setting.GridAmount = 0
			data.liquidated = true
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`liquidate short result: %s %s chance:%d amount:%f chanceInAll:%f short-long:%f %f px:%f n:%e`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, chanceInAll, priceShort, priceLong,
				setting.PriceX, data.n))
		}
	}
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

func placeTurtleOrders(key, secret string, turtleData *Data, setting *model.Setting, chanceValid bool, chanceInAll float64,
	priceShort, priceLong float64, tick *model.BidAsk) {
	coinLimit := int64(setting.OpenShortMargin)
	if turtleData.orderLong == nil && (chanceValid || setting.Chance < 0) {
		orderSide := model.OrderSideBuy
		typeLong := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance < 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平空 %s %s chance:%d amount:%f chanceInAll:%f short-long:%f %f px:%f n:%e`,
				setting.Market, setting.Symbol, setting.Chance, amount, chanceInAll, priceShort,
				priceLong, setting.PriceX, turtleData.n))
		}
		util.Notice(fmt.Sprintf(`%s %s place多单 at %f chance:%d amount:%f priceX:%f chanceInAll-limit:%f %f
			orderSide:%s h%d:%f h%d:%f l%d:%f l%d:%f coin limit:%d`,
			setting.Market, setting.Symbol, priceLong, setting.Chance, amount, setting.PriceX, chanceInAll, setting.AmountLimit,
			orderSide, turtleData.daysFar, turtleData.highDaysFar, turtleData.daysNear, turtleData.highDaysNear,
			turtleData.daysFar, turtleData.lowDaysFar, turtleData.daysNear, turtleData.lowDaysNear, coinLimit))
		priceOut := false
		if priceLong <= tick.Asks[0].Price {
			adjusts := turtleData.orderAdjust
			turtleData.orderAdjust = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
				model.FunctionTurtle, priceLong*(1+turtleTriggerDelta/2), priceLong, amount, setting)
			for _, adjust := range adjusts {
				turtleData.orderAdjust = append(turtleData.orderAdjust, adjust)
			}
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
	if turtleData.orderShort == nil && (chanceValid || setting.Chance > 0) {
		orderSide := model.OrderSideSell
		typeShort := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance > 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平多 %s %s chance:%d amount:%f currentNum:%f short-long:%f %f px:%f n:%e`,
				setting.Market, setting.Symbol, setting.Chance, amount, chanceInAll, priceShort, priceLong, setting.PriceX, turtleData.n))
		}
		util.Notice(fmt.Sprintf(`%s %s place空单 at %f chance:%d amount:%f priceX:%f currentNum-limit:%f %f 
				orderSide:%s h%d:%f h%d:%f l%d:%f l%d:%f coin limit:%d`,
			setting.Market, setting.Symbol, priceShort, setting.Chance, amount, setting.PriceX, chanceInAll, setting.AmountLimit,
			orderSide, turtleData.daysFar, turtleData.highDaysFar, turtleData.daysNear, turtleData.highDaysNear,
			turtleData.daysFar, turtleData.lowDaysFar, turtleData.daysNear, turtleData.lowDaysNear, coinLimit))
		priceOut := false
		if priceShort >= tick.Bids[0].Price {
			adjusts := turtleData.orderAdjust
			turtleData.orderAdjust = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
				model.FunctionTurtle, priceShort*(1-turtleTriggerDelta/2), priceShort, amount, setting)
			for _, adjust := range adjusts {
				turtleData.orderAdjust = append(turtleData.orderAdjust, adjust)
			}
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

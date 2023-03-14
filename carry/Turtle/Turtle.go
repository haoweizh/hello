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

var turtling = false
var turtleLock sync.Mutex

func checkSetTurtling(value bool) (before bool) {
	turtleLock.Lock()
	defer turtleLock.Unlock()
	before = turtling
	if value == false || before == false {
		turtling = value
	}
	return before
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
		util.Notice(fmt.Sprintf(`no last priceX %s %s %d %e`,
			setting.Market, setting.Symbol, setting.Chance, setting.PriceX))
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	data := api.GetTurtleData(account.Key, account.Secret, setting.Function, setting.Market, setting.Symbol)
	if data == nil || data.N == 0 || data.Amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle %s %s`, setting.Market, setting.Symbol))
		}
		return
	}
	if !data.OrderCleared {
		api.ClearOrders(account.Key, account.Secret, setting.Market, setting.Symbol)
		data.OrderCleared = true
		return
	}
	canOpenTurtle, chanceInAll := api.CanOpenTurtle(setting, data)
	msgKey := fmt.Sprintf("%s_%s_%s", setting.Function, setting.Market, setting.Symbol)
	msg := fmt.Sprintf("[%s] %s 当前已经持仓数量:%e 持仓数/限制:%d/%d 总仓数币数/仓数币数限制:%d %d canOpen%v 上一次开仓的价格:%e "+
		"%d日:%e-%e %d日:%e-%e N:%e 单次数量:%e bid-ask %e %e 当日有平仓：%v",
		data.TurtleTime.String()[0:10], msgKey, setting.GridAmount, setting.Chance, int(setting.OpenShortMargin), int(chanceInAll),
		int(setting.AmountLimit), canOpenTurtle, setting.PriceX, data.DaysFar, data.LowDaysFar, data.HighDaysFar, data.DaysNear, data.LowDaysNear,
		data.HighDaysNear, data.N, data.Amount, tick.Bids[0].Price, tick.Asks[0].Price, data.Liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	priceLong := data.HighDaysFar
	priceShort := data.LowDaysFar
	if api.HandleOrders(account.Key, account.Secret, setting.Market, setting.Symbol, []*model.Setting{setting}, []*api.TurtleData{data}) ||
		api.CheckBreak(account.Key, account.Secret, setting.Market, setting.Symbol, []*model.Setting{setting}, []*api.TurtleData{data}, tick) {
		return
	}
	if !data.AdjustChecked {
		return
	}
	if setting.Chance == 0 { // 开初始仓
		placeTurtleOrders(account.Key, account.Secret, data, setting, canOpenTurtle, chanceInAll, priceShort, priceLong, tick)
		if data.BreakLong && data.OrderLong != nil {
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideBuy)
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
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideSell)
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
			priceShort = math.Max(setting.PriceX-2*data.N, data.LowDaysNear)
		} else {
			priceShort = math.Max(data.HighDaysFar, data.HighToday) - 2*data.N
		}
		placeTurtleOrders(account.Key, account.Secret, data, setting, canOpenTurtle, chanceInAll, priceShort, priceLong, tick)
		// 加仓一个单位
		if data.BreakLong && data.OrderLong != nil {
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideBuy)
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
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideSell)
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
			priceLong = math.Min(setting.PriceX+2*data.N, data.HighDaysNear)
		} else {
			if data.LowToday > 0 {
				priceLong = math.Min(data.LowDaysFar, data.LowToday) + 2*data.N
			} else {
				priceLong = data.LowDaysFar + 2*data.N
			}
		}
		placeTurtleOrders(account.Key, account.Secret, data, setting, canOpenTurtle, chanceInAll, priceShort, priceLong, tick)
		// 加仓一个单位
		if data.BreakShort && data.OrderShort != nil {
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideSell)
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
			handleBreak(account.Key, account.Secret, setting, data, model.OrderSideBuy)
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

func handleBreak(key, secret string, setting *model.Setting, turtleData *api.TurtleData, orderSide string) {
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
		setting.PriceX = orderQuery[0].TriggerPrice
		turtleData.OrderLong = nil
		turtleData.OrderShort = nil
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

func placeTurtleOrders(key, secret string, turtleData *api.TurtleData, setting *model.Setting, canOpen bool, chanceInAll float64,
	priceShort, priceLong float64, tick *model.BidAsk) {
	coinLimit := int64(setting.OpenShortMargin)
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < setting.OpenShortMargin
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
			orderSide, turtleData.DaysFar, turtleData.HighDaysFar, turtleData.DaysNear, turtleData.HighDaysNear,
			turtleData.DaysFar, turtleData.LowDaysFar, turtleData.DaysNear, turtleData.LowDaysNear, coinLimit))
		turtleData.BreakLong = false
		if priceLong <= tick.Asks[0].Price {
			turtleData.OrderLong = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
				setting.Function, priceLong*(1+api.TurtleTriggerDelta/2), priceLong, amount, setting)
			for _, order := range turtleData.OrderLong {
				turtleData.OrderAdjust = append(turtleData.OrderAdjust, order)
			}
			turtleData.BreakLong = true
		} else {
			turtleData.OrderLong = api.MustPlaceOrder(key, secret, orderSide, typeLong, setting.Market, setting.Symbol, ``,
				setting.Function, priceLong*(1+api.TurtleTriggerDelta), priceLong, amount, setting)
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
			orderSide, turtleData.DaysFar, turtleData.HighDaysFar, turtleData.DaysNear, turtleData.HighDaysNear,
			turtleData.DaysFar, turtleData.LowDaysFar, turtleData.DaysNear, turtleData.LowDaysNear, coinLimit))
		turtleData.BreakShort = false
		if priceShort >= tick.Bids[0].Price {
			turtleData.OrderShort = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
				setting.Function, priceShort*(1-api.TurtleTriggerDelta/2), priceShort, amount, setting)
			for _, order := range turtleData.OrderShort {
				turtleData.OrderAdjust = append(turtleData.OrderAdjust, order)
			}
			turtleData.BreakShort = true
		} else {
			turtleData.OrderShort = api.MustPlaceOrder(key, secret, orderSide, typeShort, setting.Market, setting.Symbol, ``,
				setting.Function, priceShort*(1-api.TurtleTriggerDelta), priceShort, amount, setting)
		}
		if turtleData.OrderShort != nil {
			for _, order := range turtleData.OrderShort {
				order.LineBuy = turtleData.N
				go model.AppDB.Save(order)
			}
		}
	}
}

package carry

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
var ProcessCombineTurtle = func(setting *model.Setting, tick *model.BidAsk) {
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
	if (setting.Chance != 0 && setting.PriceX == 0) || (setting.ChanceRe != 0 && setting.PriceXRe == 0) {
		util.Notice(fmt.Sprintf(`no last priceX/priceXRe %s %s %d %f %d %f`,
			setting.Market, setting.Symbol, setting.Chance, setting.PriceX, setting.ChanceRe, setting.PriceXRe))
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	turtleData := GetTurtleData(account.Key, account.Secret, setting.Market, setting.Symbol, setting)
	if turtleData == nil || turtleData.n == 0 || turtleData.amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle %s %s`, setting.Market, setting.Symbol))
		}
		return
	}
	currentN := api.GetCurrentN(setting)
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionTurtle, setting.Market, setting.Symbol)
	msg := fmt.Sprintf("[海龟参数]%s %s 次数限制:%f 当前已经持仓数量:%f 上一次开仓的价格:%f "+
		"%d日:%f-%f %d日:%f-%f n:%f 数量:%f %s 持仓数/限制:%d/%f 总持仓数%d bid-ask %f %f 当日有平仓：%v",
		turtleData.turtleTime.String()[0:10], msgKey, setting.AmountLimit, setting.GridAmount, setting.PriceX,
		turtleData.daysFar, turtleData.lowDaysFar, turtleData.highDaysFar, turtleData.daysNear, turtleData.lowDaysNear,
		turtleData.highDaysNear, turtleData.n, turtleData.amount, setting.Symbol, setting.Chance,
		setting.OpenShortMargin, currentN, tick.Bids[0].Price, tick.Asks[0].Price, turtleData.liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	priceLong := turtleData.highDaysFar
	priceShort := turtleData.lowDaysFar
	if checkTurtleOrders(account.Key, account.Secret, setting, float64(currentN), turtleData) ||
		checkTurtleBreak(account.Key, account.Secret, setting, turtleData, tick) {
		return
	}
	if setting.Chance == 0 && !turtleData.liquidated { // 开初始仓
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, currentN, priceShort, priceLong, tick)
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			setting.Chance = 1
			setting.GridAmount = turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破%d日高点 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				turtleData.daysFar, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN,
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
				`破%d日低点 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				turtleData.daysNear, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN,
				priceShort, priceLong, setting.PriceX, turtleData.n))
		}
	} else if setting.Chance > 0 {
		priceLong = math.Max(priceLong, setting.PriceX+turtleData.n/2)
		if turtleData.useNear {
			priceShort = math.Max(setting.PriceX-2*turtleData.n, turtleData.lowDaysNear)
		} else {
			priceShort = math.Max(turtleData.highDaysFar, turtleData.highToday) - 2*turtleData.n
		}
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, currentN, priceShort, priceLong, tick)
		// 加仓一个单位
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			setting.Chance = setting.Chance + 1
			setting.GridAmount = setting.GridAmount + turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加多 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort, priceLong,
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
			util.Notice(fmt.Sprintf(`liquidate long %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort, priceLong,
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
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, currentN, priceShort, priceLong, tick)
		// 加仓一个单位
		if turtleData.breakShort && turtleData.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideSell)
			setting.Chance = setting.Chance - 1
			setting.GridAmount = setting.GridAmount + turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加空 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort, priceLong,
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
			util.Notice(fmt.Sprintf(`liquidate short result: %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort, priceLong,
				setting.PriceX, turtleData.n))
		}
	}
}

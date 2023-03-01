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
// combine turtle 负责反向下限价单，invalid turtle负责正常下turtle单
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
	settingInvalid := api.GetInvalidTurtle(setting.Market, setting.Symbol)
	if settingInvalid == nil || settingInvalid.Valid { // 使用valid为false的turtle作为对应turtle,否则该算法不运行
		return
	}
	if (setting.Chance != 0 && setting.PriceX == 0) || (settingInvalid.Chance != 0 && settingInvalid.PriceX == 0) {
		util.Notice(fmt.Sprintf(`no last priceX %s %s %d %f %d %f`,
			setting.Market, setting.Symbol, setting.Chance, setting.PriceX, settingInvalid.Chance, settingInvalid.PriceX))
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	turtleData := GetTurtleData(account.Key, account.Secret, setting.Function, setting.Market, setting.Symbol, false)
	turtleDataInvalid := GetTurtleData(account.Key, account.Secret, model.FunctionTurtle, setting.Market, setting.Symbol, true)
	if turtleData == nil || turtleData.n == 0 || turtleData.amount == 0 ||
		turtleDataInvalid == nil || turtleDataInvalid.n == 0 || turtleDataInvalid.amount == 0 {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`fail to get turtle combine & turtle %s %s`, setting.Market, setting.Symbol))
		}
		return
	}
	turtleCoins := api.GetTurtleSettingNum(setting.Function, setting.Market)
	msgKey := fmt.Sprintf("%s_%s_%s", model.FunctionCombineTurtle, setting.Market, setting.Symbol)
	msg := fmt.Sprintf("[海龟参数]%s %s 币种数:%d/%f %d日:%f-%f %d日:%f-%f n:%f 仓数限制：%f 单仓数量:%f bid-ask %f %f \n"+
		"海龟:仓数/持仓量/开仓价/今日平仓 %d/%f/%f/%v\n 反向:仓数/持仓量/开仓价/今日平仓 %d/%f/%f/%v",
		turtleData.turtleTime.String()[0:10], msgKey, turtleCoins, setting.AmountLimit, turtleData.daysFar, turtleData.lowDaysFar,
		turtleData.highDaysFar, turtleData.daysNear, turtleData.lowDaysNear, turtleData.highDaysNear, turtleData.n, setting.OpenShortMargin,
		turtleData.amount, tick.Bids[0].Price, tick.Asks[0].Price,
		settingInvalid.Chance, settingInvalid.GridAmount, settingInvalid.PriceX, turtleDataInvalid.liquidated,
		setting.Chance, setting.GridAmount, setting.PriceX, turtleData.liquidated)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	priceLong := turtleData.highDaysFar
	priceShort := turtleData.lowDaysFar
	if checkTurtleOrders(account.Key, account.Secret, setting, float64(turtleCoins), turtleData) ||
		checkTurtleOrders(account.Key, account.Secret, settingInvalid, float64(turtleCoins), turtleDataInvalid) ||
		checkTurtleBreak(account.Key, account.Secret, setting, turtleData, tick) ||
		checkTurtleBreak(account.Key, account.Secret, settingInvalid, turtleDataInvalid, tick) {
		return
	}
	if setting.Chance == 0 && !turtleData.liquidated { // 开初始仓
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, turtleCoins, priceShort, priceLong, tick)
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			setting.Chance = 1
			setting.GridAmount = turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破%d日高点 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				turtleData.daysFar, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins,
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
				turtleData.daysNear, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins,
				priceShort, priceLong, setting.PriceX, turtleData.n))
		}
	} else if setting.Chance > 0 {
		priceLong = math.Max(priceLong, setting.PriceX+turtleData.n/2)
		if turtleData.useNear {
			priceShort = math.Max(setting.PriceX-2*turtleData.n, turtleData.lowDaysNear)
		} else {
			priceShort = math.Max(turtleData.highDaysFar, turtleData.highToday) - 2*turtleData.n
		}
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, turtleCoins, priceShort, priceLong, tick)
		// 加仓一个单位
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			setting.Chance = setting.Chance + 1
			setting.GridAmount = setting.GridAmount + turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加多 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, priceShort, priceLong,
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
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, priceShort, priceLong,
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
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, turtleCoins, priceShort, priceLong, tick)
		// 加仓一个单位
		if turtleData.breakShort && turtleData.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideSell)
			setting.Chance = setting.Chance - 1
			setting.GridAmount = setting.GridAmount + turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`加空 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, priceShort, priceLong,
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
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, turtleCoins, priceShort, priceLong,
				setting.PriceX, turtleData.n))
		}
	}
}

func initCombineAmount(turtleData *TurtleData, setting *model.Setting, currentNum int64, priceShort, priceLong,
	priceShortAnti, priceLongAnti float64) (amountBuy, amountSell float64) {
	amountLimit := int64(setting.AmountLimit)
	coinLimit := int64(setting.OpenShortMargin)
	if turtleData.orderLong == nil && (setting.Chance < 0 || (currentNum < amountLimit && setting.Chance < coinLimit &&
		setting.SymbolRelated != model.SettingTurtleRemoved)) {
		if setting.Chance < 0 {
			amountBuy = setting.GridAmount
		} else {
			amountBuy = turtleData.amount
		}
	}
	if turtleData.orderShort == nil && (setting.Chance > 0 || (currentNum > -1*amountLimit && setting.Chance > -1*coinLimit &&
		setting.SymbolRelated != model.SettingTurtleRemoved)) {
		if setting.Chance > 0 {
			amountSell = setting.GridAmount
		} else {
			amountSell = turtleData.amount
		}
	}
	minSize := 0.0
	marketInfo := model.GetMarketInfo(setting.Market, setting.Symbol)
	if marketInfo == nil {
		util.Notice(`fail to get marketInfo %s %s`, setting.Market, setting.Symbol)
	} else {
		if marketInfo.CTValue == 0 {
			minSize = marketInfo.SizeMin
		} else {
			minSize = marketInfo.SizeMin * marketInfo.CTValue
		}
	}
	if amountBuy > 0 && math.Abs(priceShortAnti-priceLong)/priceLong < turtleTriggerDelta {
		amountBuy = minSize
	}
	if amountSell > 0 && math.Abs(priceLongAnti-priceShort)/priceShort < turtleTriggerDelta {
		amountSell = minSize
	}
	return
}

func placeCombineOrders(key, secret string, turtleData, turtleDataInvalid *TurtleData, setting, settingInvalid *model.Setting,
	priceShort, priceLong, priceShortInvalid, priceLongInvalid float64, currentNum int64, tick *model.BidAsk) {
	amountBuy, amountSell := initCombineAmount(turtleData, setting, currentNum, priceShort, priceLong, priceShortInvalid, priceLongInvalid)
	amountBuyInvalid, amountSellInvalid := initCombineAmount(turtleDataInvalid, settingInvalid, currentNum, priceShortInvalid, priceLongInvalid, priceShort, priceLong)
	market := setting.Market
	symbol := setting.Symbol
	if amountSell > 0 {
		turtleData.orderShort = api.MustPlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, market, symbol, ``,
			model.FunctionTurtle, priceShort, priceShort, amountSell, nil)
		updateTurtleData(turtleData, turtleData.orderShort, priceShort < tick.Asks[0].Price)
	}
	if amountBuy > 0 {
		turtleData.orderLong = api.MustPlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, market, symbol, ``,
			model.FunctionTurtle, priceLong, priceLong, amountBuy, nil)
		updateTurtleData(turtleData, turtleData.orderLong, priceLong > tick.Bids[0].Price)
	}
	if amountSellInvalid > 0 {
		priceOut := false
		price := priceShortInvalid
		amount := amountSellInvalid
		if price >= tick.Bids[0].Price {
			turtleDataInvalid.orderShort = api.MustPlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, market,
				symbol, ``, model.FunctionTurtle, price*(1-turtleTriggerDelta/2), price, amount, nil)
			priceOut = true
		} else {
			turtleDataInvalid.orderShort = api.MustPlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeStop, market,
				symbol, ``, model.FunctionTurtle, price*(1-turtleTriggerDelta), price, amount, nil)
		}
		updateTurtleData(turtleDataInvalid, turtleDataInvalid.orderShort, priceOut)
	}
	if amountBuyInvalid > 0 {
		priceOut := false
		price := priceLongInvalid
		amount := amountBuyInvalid
		if price <= tick.Asks[0].Price {
			turtleDataInvalid.orderLong = api.MustPlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, market,
				symbol, ``, model.FunctionTurtle, price*(1+turtleTriggerDelta/2), price, amount, nil)
			priceOut = true
		} else {
			turtleDataInvalid.orderLong = api.MustPlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeStop, market,
				symbol, ``, model.FunctionTurtle, priceLong*(1+turtleTriggerDelta), price, amount, nil)
		}
		updateTurtleData(turtleDataInvalid, turtleDataInvalid.orderLong, priceOut)
	}
}

func updateTurtleData(turtleData *TurtleData, orders []*model.Order, priceOut bool) {
	if orders == nil || len(orders) == 0 {
		return
	}
	if orders[0].OrderSide == model.OrderSideSell {
		turtleData.waitBreakShort = true
		turtleData.breakShort = false
		if priceOut {
			turtleData.breakShort = true
		}
		for _, value := range turtleData.orderShort {
			value.LineBuy = turtleData.n
			go model.AppDB.Save(value)
		}
	}
	if orders[0].OrderSide == model.OrderSideBuy {
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

package Turtle

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"time"
)

//var debugMap sync.Map
//d, _ := debugMap.Load(settingCombine.Symbol)
//if d == nil {
//debugMap.Store(settingCombine.Symbol, 1)
//util.Notice(fmt.Sprintf(`first time %s %#v %#v %#v`, settingCombine.Symbol, canOpen, canStartTurtle, canStartCombine))
//}

// ProcessCombineTurtle
// setting.CloseShortMargin 是否下单的价格倍率限制
// setting.GridAmount 当前已经持仓数量
// setting.Chance 当前开仓的个数
// setting.PriceX 上一次开仓的价格
// setting.ChanceLimit 该单币种最多开仓个数
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
	success, _, _, _ := model.GetFromStandard(market, symbol)
	if settingCombine == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		(model.AppConfig.Env != `test` && now-int64(tick.Ts) > 120000) || !success {
		return
	}
	settingNormal := api.GetSetting(model.FunctionTurtleNormal, market, symbol)
	if settingNormal == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`combine return no normal setting from %s %s`, market, symbol))
		return
	}
	account := model.AppConfig.GetAccounts(market)[0]
	if (settingCombine.Chance != 0 && settingCombine.PriceX == 0) || (settingNormal.Chance != 0 && settingNormal.PriceX == 0) {
		if time.Now().Second() == 0 {
			util.Log(util.LogLevelError, fmt.Sprintf(`combine return no last priceX %s %s %d %e %d %e`,
				market, symbol, settingCombine.Chance, settingCombine.PriceX, settingNormal.Chance, settingNormal.PriceX))
		}
		return
	}
	settings := []*model.Setting{settingCombine, settingNormal}
	var dataCombine, dataNormal *model.TurtleData
	removed := settingNormal.Chance == 0 && settingNormal.SymbolRelated == model.SettingTurtleRemoved &&
		settingCombine.Chance == 0 && settingCombine.SymbolRelated == model.SettingTurtleRemoved
	if removed && time.Now().Minute() < 5 {
		return
	}
	dataCombine, _ = api.GetTurtleData(account, settingCombine, removed)
	dataNormal, _ = api.GetTurtleData(account, settingNormal, removed)
	if dataCombine == nil || dataNormal == nil || settingCombine == nil || settingNormal == nil ||
		model.AppConfig.Env == `test` || dataCombine.N == 0 || dataNormal.N == 0 {
		if !removed && (time.Now().Minute() == 0 && time.Now().Second() == 0) {
			util.Log(util.LogLevelError, fmt.Sprintf(`combine return no turtle combine turtle %s %s`, market, symbol))
		}
		return
	}
	if time.Now().After(dataCombine.Expire) || time.Now().After(dataNormal.Expire) {
		util.LogLess(util.LogLevelError, fmt.Sprintf(`turtle data expired %s %s`, settingCombine.Market, settingCombine.Symbol))
		return
	}
	if !dataCombine.OrderCleared {
		api.ClearOrders(account.Key, account.Secret, market, symbol, map[string]bool{model.OrderTypeTrailStop: true})
		dataCombine.OrderCleared = true
		util.Log(util.LogLevelInfo, fmt.Sprintf(`combine return not cleared %s %s %#v`, market, symbol, dataCombine.OrderCleared))
		return
	}
	if dataNormal.N == 0 || dataNormal.Amount == 0 || dataCombine.N == 0 || dataCombine.Amount == 0 {
		util.Log(util.LogLevelError, fmt.Sprintf(`invalid turtle data n %s %s %f %f %f %f`,
			settingCombine.Market, settingCombine.Symbol, dataNormal.N, dataNormal.Amount, dataCombine.N, dataCombine.Amount))
		return
	}
	turtleData := []*model.TurtleData{dataCombine, dataNormal}
	//checkFulled := true
	//if settingCombine.Seconds >= 43200 || settingCombine.Market == model.OKEX {
	//	checkFulled = false
	//}
	canOpen, canStartCombine, canStartTurtle, turtleSymbolNum, turtleCoins := api.CanOpenCombine(settingCombine, settingNormal, dataNormal)
	if api.HandleOrders(account.Key, account.Secret, market, symbol, settings, turtleData, tick) ||
		api.CheckBreak(account, market, symbol, settings, turtleData, tick) ||
		api.CheckActiveTrail(account, settingNormal, dataNormal, tick) {
		//util.Notice(fmt.Sprintf(`combine return handle or break %s %s`, market, symbol))
		return
	}
	model.ResetBig(dataCombine, dataNormal)
	msgKey := model.GetMsgKey(model.FunctionCombineTurtle, market, symbol)
	msg := fmt.Sprintf("[%d-%d %d:%d]%s N-Volume %f 可开%v龟仓数%d(海龟%v 龟汤%v) 币种数:%d/%d满币%v bid-ask %e %e \n",
		dataCombine.TurtleTime.Month(), dataCombine.TurtleTime.Day(), time.Now().Hour(), time.Now().Minute(), msgKey,
		dataCombine.NVolume, canOpen, int64(turtleSymbolNum), canStartTurtle, canStartCombine, int(turtleCoins),
		int(settingCombine.AmountLimit), turtleCoins >= settingCombine.AmountLimit, tick.Bids[0].Price, tick.Asks[0].Price)
	msg += fmt.Sprintf("海龟:仓数/持仓量/开仓价 %d of %d/%e/%e 平过%v 数量 %e %s%v big:%d 日:%e-%e %d日:%e-%e N:%e\n",
		settingNormal.Chance, settingNormal.ChanceLimit, settingNormal.GridAmount, settingNormal.PriceX, dataNormal.Liquidated,
		dataNormal.Amount*float64(settingNormal.ChanceLimit), dataNormal.GetIds(), dataNormal.IsBig, dataNormal.DaysFar,
		dataNormal.LowFar, dataNormal.HighFar, dataNormal.DaysNear, dataNormal.LowNear, dataNormal.HighNear, dataNormal.N)
	msg += fmt.Sprintf("龟汤:仓数/持仓量/开仓价 %d of %d/%e/%e 平过%v 数量 %e %s%v big:%d 日:%e-%e %d日:%e-%e N:%e",
		settingCombine.Chance, settingCombine.ChanceLimit, settingCombine.GridAmount, settingCombine.PriceX, dataCombine.Liquidated,
		dataCombine.Amount*float64(settingCombine.ChanceLimit), dataCombine.GetIds(), dataCombine.IsBig, dataCombine.DaysFar,
		dataCombine.LowFar, dataCombine.HighFar, dataCombine.DaysNear, dataCombine.LowNear, dataCombine.HighNear, dataCombine.N)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	placeCombineOrders(account, dataNormal, dataCombine, settingNormal, settingCombine, tick, canOpen, canStartTurtle, canStartCombine)
	needClear := false
	for i, setting := range settings {
		if handleBreak(setting, turtleData[i], turtleData[i].OrderLong, turtleData[i].BreakLong) {
			needClear = true
		}
		if handleBreak(setting, turtleData[i], turtleData[i].OrderShort, turtleData[i].BreakShort) {
			needClear = true
		}
	}
	if needClear {
		api.ClearExtraOrders(account.Key, account.Secret, market, symbol, turtleData)
	}
}

func placeCombineOrders(account *model.Account, dataNormal, dataCombine *model.TurtleData, settingNormal,
	settingCombine *model.Setting, tick *model.BidAsk, canOpen, canStartTurtle, canStartCombine bool) {
	if canOpen {
		placeTurtleLong(account, model.OrderTypeLimit, dataCombine, settingCombine, tick, canStartCombine, true)
		placeTurtleShort(account, model.OrderTypeLimit, dataCombine, settingCombine, tick, canStartCombine, true)
		placeTurtleLong(account, model.OrderTypeStop, dataNormal, settingNormal, tick, canStartTurtle, true)
		placeTurtleShort(account, model.OrderTypeStop, dataNormal, settingNormal, tick, canStartTurtle, true)
		if !canStartTurtle {
			removeLongOrders(account, settingNormal, dataNormal)
			removeShortOrders(account, settingNormal, dataNormal)
		}
		if !canStartCombine {
			removeLongOrders(account, settingCombine, dataCombine)
			removeShortOrders(account, settingCombine, dataCombine)
		}
	} else {
		removeLongOrders(account, settingNormal, dataNormal)
		removeShortOrders(account, settingNormal, dataNormal)
		removeLongOrders(account, settingCombine, dataCombine)
		removeShortOrders(account, settingCombine, dataCombine)
	}
}

func removeLongOrders(account *model.Account, setting *model.Setting, data *model.TurtleData) {
	if data == nil || setting == nil {
		return
	}
	if setting.Chance >= 0 {
		if data.OrderLong != nil {
			for _, order := range data.OrderLong {
				api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, false)
			}
		}
		data.OrderLong = nil
	}
}

func removeShortOrders(account *model.Account, setting *model.Setting, data *model.TurtleData) {
	if data == nil || setting == nil {
		return
	}
	if setting.Chance <= 0 {
		if data.OrderShort != nil {
			for _, order := range data.OrderShort {
				api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, false)
			}
		}
		data.OrderShort = nil
	}
}

// handleBreak
// 由于OrderTypeTrailStop订单有可能是通过API load进来的，所以没有 order.Function且此类订单都是close的，故特殊处理了
func handleBreak(setting *model.Setting, data *model.TurtleData, orders []*model.Order, orderBreak bool) (work bool) {
	if orders == nil || len(orders) == 0 || !orderBreak || data == nil {
		return false
	}
	turtleTriggerDelta := api.GetTurtleTriggerDelta(setting.Market)
	setting.PriceX = orders[0].Price / (1 + turtleTriggerDelta)
	if orders[0].OrderSide == model.OrderSideSell {
		setting.PriceX = orders[0].Price / (1 - turtleTriggerDelta)
	}
	if orders[0].TriggerPrice > 0 {
		setting.PriceX = orders[0].TriggerPrice
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`query %s break %s %s %s %d %s %s chances %d`,
		setting.Function, setting.Market, setting.Symbol, orders[0].OrderSide, len(orders), orders[0].OrderId, orders[0].Function, setting.Chance))
	if orders[0].RefreshType != model.FunctionTurtleAdjust && orders[0].Function == model.Close {
		msg := fmt.Sprintf(`liquidate: %s %s chance:%d Amount:%e px:%e %s order type %s`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, setting.PriceX, orders[0].OrderSide, orders[0].OrderType)
		go api.SendMails(`平`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		setting.PriceX = 0
		// OrderTypeTrailStop会在不同TurtleData周期之间继承，所以在下单的时候设定已平仓，在成交以后不再设定，这样可以让继承OrderTypeTrailStop的周期正常开仓。
		if orders[0].OrderType != model.OrderTypeTrailStop {
			data.Liquidated = true
		}
	} else if orders[0].Function == model.Open {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`加%s %s %s chance:%d Amount:%e px:%e order type %s`,
			orders[0].OrderSide, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, setting.PriceX, orders[0].OrderType))
		if orders[0].OrderSide == model.OrderSideSell {
			setting.Chance--
		} else if orders[0].OrderSide == model.OrderSideBuy {
			setting.Chance++
		}
		for _, order := range orders {
			setting.GridAmount += order.Amount
		}
	}
	// 保护起来，不进行主动撤单，以免未完全成交
	for _, order := range orders {
		data.OrderAdjust[order.OrderId] = order
	}
	time.Sleep(time.Second * 3)
	data.OrderLong = nil
	data.OrderShort = nil
	model.AppDB.Model(setting).Where("market= ? and Symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount, `liquidated`: data.Liquidated})
	util.Log(util.LogLevelInfo, fmt.Sprintf(`clear turtle buy when sell break %s %s %#v`, setting.Market, setting.Symbol, orders))
	return true
}

// 海龟没持仓: 龟汤的平仓单数量：实际龟汤仓位大小，
// 海龟有持仓: 龟汤的平仓单数量：龟汤仓数*大单
func placeTurtleLong(account *model.Account, orderType string, data *model.TurtleData, setting *model.Setting,
	tick *model.BidAsk, canOpen, isBig bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance < 0 {
		function = model.Close
		amount = setting.GridAmount
		if amount == 0 {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to place short amt 0 from grid amt, set chance 0 %s %s %d`,
				setting.Market, setting.Symbol, setting.Chance))
			setting.Chance = 0
			return
		}
	} else if !isBig {
		amount = data.Amount / 2
	}
	price := data.HighFar
	priceChange := 2 * data.N
	if setting.Seconds == 14400 {
		priceChange = 2.5 * data.N
	}
	if orderType == model.OrderTypeLimit {
		if setting.Chance < 0 {
			priceChange = 1.7 * data.N
			price = math.Max(math.Max(setting.PriceX, data.HighFar)-priceChange, data.LowFar)
		} else if setting.Chance == 0 {
			price = data.LowFar + data.N*0.4
		} else if setting.Chance == 1 {
			price = data.LowFar
		} else if setting.Chance > 1 {
			price = setting.PriceX - data.N*0.4
		}
	} else if orderType == model.OrderTypeStop {
		priceInc := 0.0
		v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
		if v != nil {
			priceInc = v.(*model.MarketInfo).PriceIncrement
		}
		if setting.Chance > 0 {
			if data.HighFar > setting.PriceX+data.N*0.4 {
				price = data.HighFar + priceInc
			} else {
				price = setting.PriceX + data.N*0.4
			}
		} else if setting.Chance < 0 {
			if data.UseNear {
				if setting.PriceX+priceChange < data.HighNear {
					price = setting.PriceX + priceChange
				} else {
					price = data.HighNear + priceInc
				}
			} else {
				price = data.LowFar + priceChange + priceInc
			}
		} else if setting.Chance == 0 {
			price += priceInc
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	canOpen = (canOpen && float64(setting.Chance) < float64(setting.ChanceLimit)) || setting.Chance < 0
	if data.OrderLong == nil && canOpen {
		priceDeal := price
		data.BreakLong = false
		turtleTriggerDelta := api.GetTurtleTriggerDelta(setting.Market)
		if orderType == model.OrderTypeStop {
			if price <= tick.Asks[0].Price {
				data.BreakLong = true
				orderType = model.OrderTypeLimit
			}
			priceDeal = price * (1 + turtleTriggerDelta)
		} else if orderType == model.OrderTypeLimit && price >= tick.Asks[0].Price {
			data.BreakLong = true
			price = tick.Asks[0].Price
			priceDeal = tick.Asks[0].Price * (1 + turtleTriggerDelta)
		}
		data.OrderLong = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, orderType, market, symbol,
			``, setting.Function, priceDeal, price, amount, true)
		if data.OrderAdjust == nil {
			data.OrderAdjust = make(map[string]*model.Order)
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(
			`place long %s %s %s %s %s %d %#v at %e %e amt %e, useNear %#v priceX %f n:%f seconds %d near %f %f far %f %f`,
			orderType, setting.Function, market, symbol, orderType, setting.Chance, canOpen, priceDeal, price, amount,
			data.UseNear, setting.PriceX, data.N, setting.Seconds, data.LowNear, data.HighNear, data.LowFar, data.HighFar))
		for _, order := range data.OrderLong {
			order.LineBuy = data.N
			order.LineSell = data.N
			order.Function = function
			if data.IsBig {
				order.GridPos = 1
			}
			go model.AppDB.Save(order)
			if data.BreakLong && order.Status != model.CarryStatusSuccess {
				util.Log(util.LogLevelInfo, fmt.Sprintf(
					`already break long move to adjust %s %#v`, order.OrderId, order))
				data.OrderAdjust[order.OrderId] = order
			}
		}
	}
}

// 海龟没持仓: 龟汤的平仓单数量：实际龟汤仓位大小，
// 海龟有持仓: 龟汤的平仓单数量：龟汤仓数*大单
func placeTurtleShort(account *model.Account, orderType string, data *model.TurtleData, setting *model.Setting,
	tick *model.BidAsk, canOpen, isBig bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance > 0 {
		amount = setting.GridAmount
		function = model.Close
		if amount == 0 {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to place short amt 0 from grid amt, set chance 0 %s %s %d`,
				setting.Market, setting.Symbol, setting.Chance))
			setting.Chance = 0
			return
		}
	} else if !isBig {
		amount = data.Amount / 2
	}
	price := data.LowFar
	priceChange := 2 * data.N
	if setting.Seconds == 14400 {
		priceChange = 2.5 * data.N
	}
	if orderType == model.OrderTypeLimit {
		if setting.Chance > 0 {
			priceChange = 1.7 * data.N
			price = math.Min(math.Min(setting.PriceX, data.LowFar)+priceChange, data.HighFar)
		} else if setting.Chance == 0 {
			price = data.HighFar - data.N*0.4
		} else if setting.Chance == -1 {
			price = data.HighFar
		} else if setting.Chance < -1 {
			price = setting.PriceX + data.N*0.4
		}
	} else if orderType == model.OrderTypeStop {
		priceInc := 0.0
		v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
		if v != nil {
			priceInc = v.(*model.MarketInfo).PriceIncrement
		}
		if setting.Chance > 0 {
			if setting.PriceX-priceChange > data.LowNear {
				price = setting.PriceX - priceChange
			} else {
				price = data.LowNear - priceInc
			}
		} else if setting.Chance < 0 {
			if data.LowFar < setting.PriceX-data.N*0.4 {
				price = data.LowFar - priceInc
			} else {
				price = setting.PriceX - data.N*0.4
			}
		} else if setting.Chance == 0 {
			price -= priceInc
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	canOpen = (canOpen && float64(setting.Chance) > -1*float64(setting.ChanceLimit)) || setting.Chance > 0
	if data.OrderShort == nil && canOpen {
		priceDeal := price
		data.BreakShort = false
		turtleTriggerDelta := api.GetTurtleTriggerDelta(setting.Market)
		if orderType == model.OrderTypeStop {
			if price >= tick.Bids[0].Price {
				data.BreakShort = true
				orderType = model.OrderTypeLimit
			}
			priceDeal = price * (1 - turtleTriggerDelta)
		} else if orderType == model.OrderTypeLimit && price <= tick.Bids[0].Price {
			data.BreakShort = true
			priceDeal = tick.Bids[0].Price * (1 - turtleTriggerDelta)
			price = tick.Bids[0].Price
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`place short %s %s %s %s %s %d %#v at %e %e amt %e, useNear %#v priceX %f n:%f seconds %d near %f %f far %f %f`,
			orderType, setting.Function, market, symbol, orderType, setting.Chance, canOpen, priceDeal, price, amount,
			data.UseNear, setting.PriceX, data.N, setting.Seconds, data.LowNear, data.HighNear, data.LowFar, data.HighFar))
		data.OrderShort = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, orderType, market, symbol,
			``, setting.Function, priceDeal, price, amount, true)
		if data.OrderAdjust == nil {
			data.OrderAdjust = make(map[string]*model.Order)
		}
		for _, order := range data.OrderShort {
			order.LineBuy = data.N
			order.LineSell = data.N
			order.Function = function
			if data.IsBig {
				order.GridPos = 1
			}
			go model.AppDB.Save(order)
			if data.BreakShort && order.Status != model.CarryStatusSuccess {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`already break short move to adjust %s %#v`, order.OrderId, order))
				data.OrderAdjust[order.OrderId] = order
			}
		}
	}
}

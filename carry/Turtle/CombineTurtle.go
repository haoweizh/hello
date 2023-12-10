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

// ProcessCombineTurtle
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
	account := model.AppConfig.GetAccounts(market)[0]
	if (settingCombine.Chance != 0 && settingCombine.PriceX == 0) || (settingNormal.Chance != 0 && settingNormal.PriceX == 0) {
		util.Notice(fmt.Sprintf(`combine return no last priceX %s %s %d %e %d %e`,
			market, symbol, settingCombine.Chance, settingCombine.PriceX, settingNormal.Chance, settingNormal.PriceX))
		return
	}
	settings := []*model.Setting{settingCombine, settingNormal}
	var dataCombine, dataNormal *model.TurtleData
	removed := settingNormal.Chance == 0 && settingNormal.SymbolRelated == model.SettingTurtleRemoved &&
		settingCombine.Chance == 0 && settingCombine.SymbolRelated == model.SettingTurtleRemoved
	if removed && time.Now().Minute() < 5 {
		return
	}
	dataCombine, _ = api.GetTurtleData(account, settingCombine.Function, settingCombine.Market, settingCombine.Symbol,
		settingCombine.Far, settingCombine.Near, settingCombine.Seconds, settingCombine.ChanceLimit, settingCombine.AmountRate, true, removed)
	dataNormal, _ = api.GetTurtleData(account, settingNormal.Function, settingNormal.Market, settingNormal.Symbol,
		settingNormal.Far, settingNormal.Near, settingNormal.Seconds, settingNormal.ChanceLimit, settingNormal.AmountRate, true, removed)
	if dataCombine == nil || dataNormal == nil || settingCombine == nil || settingNormal == nil ||
		model.AppConfig.Env == `test` || dataCombine.N == 0 || dataNormal.N == 0 {
		if !removed && (time.Now().Minute() == 0 && time.Now().Second() == 0) {
			util.Notice(fmt.Sprintf(`combine return no turtle combine turtle %s %s`, market, symbol))
		}
		return
	}
	if time.Now().After(dataCombine.Expire) || time.Now().After(dataNormal.Expire) {
		return
	}
	if !dataCombine.OrderCleared {
		api.ClearOrders(account.Key, account.Secret, market, symbol, map[string]bool{model.OrderTypeTrailStop: true})
		dataCombine.OrderCleared = true
		util.Notice(fmt.Sprintf(`combine return not cleared %s %s %v`, market, symbol, dataCombine.OrderCleared))
		return
	}
	if dataNormal.N == 0 || dataNormal.Amount == 0 || dataCombine.N == 0 || dataCombine.Amount == 0 {
		return
	}
	turtleData := []*model.TurtleData{dataCombine, dataNormal}
	checkFulled := true
	if settingCombine.Seconds > 43200 {
		checkFulled = false
	}
	canOpen, turtleCoins := api.CanOpenCombine(settingCombine, settingNormal, dataCombine, dataNormal, checkFulled)
	if api.HandleOrders(account.Key, account.Secret, market, symbol, settings, turtleData, tick) ||
		api.CheckBreak(account, market, symbol, settings, turtleData, tick) {
		//util.Notice(fmt.Sprintf(`combine return handle or break %s %s`, market, symbol))
		return
	}
	model.ResetBig(dataCombine, dataNormal)
	msgKey := model.GetMsgKey(model.FunctionCombineTurtle, market, symbol)
	msg := fmt.Sprintf("[%d-%d %d:%d]%s N-Volume %f 可开%v 币种数:%d/%d bid-ask %e %e \n",
		dataCombine.TurtleTime.Month(), dataCombine.TurtleTime.Day(), time.Now().Hour(), time.Now().Minute(), msgKey,
		dataCombine.NVolume, canOpen, int(turtleCoins), int(settingCombine.AmountLimit),
		tick.Bids[0].Price, tick.Asks[0].Price)
	msg += fmt.Sprintf("海龟:仓数/持仓量/开仓价 %d of %d/%e/%e 平过%v 数量 %e %s%d big:%d 日:%e-%e %d日:%e-%e N:%e\n",
		settingNormal.Chance, settingNormal.ChanceLimit, settingNormal.GridAmount, settingNormal.PriceX, dataNormal.Liquidated,
		dataNormal.Amount*float64(settingNormal.ChanceLimit), dataNormal.GetIds(), dataNormal.Big, dataNormal.DaysFar,
		dataNormal.LowFar, dataNormal.HighFar, dataNormal.DaysNear, dataNormal.LowNear, dataNormal.HighNear, dataNormal.N)
	msg += fmt.Sprintf("龟汤:仓数/持仓量/开仓价 %d of %d/%e/%e 平过%v 数量 %e %s%d big:%d 日:%e-%e %d日:%e-%e N:%e",
		settingCombine.Chance, settingCombine.ChanceLimit, settingCombine.GridAmount, settingCombine.PriceX, dataCombine.Liquidated,
		dataCombine.Amount*float64(settingCombine.ChanceLimit), dataCombine.GetIds(), dataCombine.Big, dataCombine.DaysFar,
		dataCombine.LowFar, dataCombine.HighFar, dataCombine.DaysNear, dataCombine.LowNear, dataCombine.HighNear, dataCombine.N)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	placeCombineOrders(account, dataNormal, dataCombine, settingNormal, settingCombine, tick, canOpen)
	tryOpen := false
	needClear := false
	for i, setting := range settings {
		if handleBreak(setting, turtleData[i], turtleData[i].OrderLong, turtleData[i].BreakLong) {
			needClear = true
			tryOpen = true
		}
		if handleBreak(setting, turtleData[i], turtleData[i].OrderShort, turtleData[i].BreakShort) {
			needClear = true
			tryOpen = true
		}
	}
	if needClear {
		allLiquidate := true
		for _, setting := range settings {
			if setting.Chance != 0 {
				allLiquidate = false
			}
		}
		if allLiquidate {
			for _, data := range turtleData {
				data.AdjustChecked = false
				util.Notice(fmt.Sprintf(`set need check adjust after both setting liquidate %s %s`,
					settingCombine.Market, data.Symbol))
			}
		}
		api.ClearExtraOrders(account.Key, account.Secret, market, symbol, turtleData)
	}
	if tryOpen {
		canOpen, _ = api.CanOpenCombine(settingCombine, settingNormal, dataCombine, dataNormal, false)
		placeCombineOrders(account, dataNormal, dataCombine, settingNormal, settingCombine, tick, canOpen)
	}
}

var placeTurtleLock = &sync.Map{} // key - *sync.Mutex{}

func placeCombineOrders(account *model.Account, dataNormal, dataCombine *model.TurtleData, settingNormal, settingCombine *model.Setting, tick *model.BidAsk, canOpen bool) {
	var lock *sync.Mutex
	lockValue, _ := placeTurtleLock.Load(account.Key)
	if lockValue == nil {
		lock = &sync.Mutex{}
		placeTurtleLock.Store(account.Key, lock)
	} else {
		lock = lockValue.(*sync.Mutex)
	}
	if settingNormal.Market != model.OKEX {
		defer lock.Unlock()
		lock.Lock()
	}
	placeTurtleLong(account, model.OrderTypeStop, dataNormal, settingNormal, tick, canOpen, true, false)
	placeTurtleShort(account, model.OrderTypeStop, dataNormal, settingNormal, tick, canOpen, true, false)
	isLongBigCombine := false
	isShortBigCombine := false
	if settingNormal.Chance > 0 {
		isShortBigCombine = true
	}
	if settingNormal.Chance < 0 {
		isLongBigCombine = true
	}
	placeTurtleLong(account, model.OrderTypeLimit, dataCombine, settingCombine, tick, canOpen, isLongBigCombine, settingNormal.Chance != 0)
	placeTurtleShort(account, model.OrderTypeLimit, dataCombine, settingCombine, tick, canOpen, isShortBigCombine, settingNormal.Chance != 0)
}

// handleBreak
// 由于OrderTypeTrailStop订单有可能是通过API load进来的，所以没有 order.Function且此类订单都是close的，故特殊处理了
func handleBreak(setting *model.Setting, data *model.TurtleData, orders []*model.Order, orderBreak bool) (work bool) {
	if orders == nil || len(orders) == 0 || !orderBreak || data == nil {
		return false
	}
	setting.PriceX = orders[0].Price / (1 + api.TurtleTriggerDelta)
	if orders[0].OrderSide == model.OrderSideSell {
		setting.PriceX = orders[0].Price / (1 - api.TurtleTriggerDelta)
	}
	if orders[0].TriggerPrice > 0 {
		setting.PriceX = orders[0].TriggerPrice
	}
	util.Notice(fmt.Sprintf(`query %s break %s %s %s %d %s %s chances %d`,
		setting.Function, setting.Market, setting.Symbol, orders[0].OrderSide, len(orders), orders[0].OrderId, orders[0].Function, setting.Chance))
	if orders[0].Function == model.Close || orders[0].OrderType == model.OrderTypeTrailStop {
		msg := fmt.Sprintf(`liquidate: %s %s chance:%d Amount:%e px:%e`,
			setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, setting.PriceX)
		go api.SendMails(`平`+setting.Market+setting.Symbol, msg)
		setting.Chance = 0
		setting.GridAmount = 0
		setting.PriceX = 0
		data.Liquidated = true
	} else if orders[0].Function == model.Open {
		util.Notice(fmt.Sprintf(`加%s %s %s chance:%d Amount:%e px:%e`,
			orders[0].OrderSide, setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, setting.PriceX))
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
		`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
	util.Notice(fmt.Sprintf(`clear turtle buy when sell break %s %s %v`, setting.Market, setting.Symbol, orders))
	return true
}

// 海龟没持仓: 龟汤的平仓单数量：实际龟汤仓位大小，
// 海龟有持仓: 龟汤的平仓单数量：龟汤仓数*大单
func placeTurtleLong(account *model.Account, orderType string, data *model.TurtleData, setting *model.Setting,
	tick *model.BidAsk, canOpen, isBig, normalHolding bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance < 0 {
		function = model.Close
		amount = setting.GridAmount
		if setting.Function == model.FunctionCombineTurtle && normalHolding {
			amount = math.Abs(float64(setting.Chance)) * data.Amount
		}
		if setting.Function == model.FunctionCombineTurtle {
			amount = math.Max(math.Min(1.5*setting.GridAmount, amount), 0.85*setting.GridAmount)
		}
	} else if !isBig {
		amount = data.Amount / 2
	}
	price := data.HighFar
	priceChange := 2 * data.N
	if setting.Seconds == 14400 {
		priceChange = 3 * data.N
	}
	if orderType == model.OrderTypeLimit {
		price = data.LowFar + data.N/2
		if setting.Chance > 0 {
			price = math.Min(data.LowFar, setting.PriceX-data.N/2)
		} else if setting.Chance < 0 {
			if setting.Seconds == 14400 {
				priceChange = 2 * data.N
			}
			if data.UseNear {
				price = math.Max(setting.PriceX-priceChange, data.LowNear)
			} else {
				//price = math.Max(data.HighFar, setting.PriceX) - priceChange
				price = setting.PriceX/3 + data.HighFar*2/3 - priceChange
			}
		}
	} else if orderType == model.OrderTypeStop {
		if setting.Chance > 0 {
			price = math.Max(data.HighFar, setting.PriceX+data.N/2)
		} else if setting.Chance < 0 {
			if data.UseNear {
				price = math.Min(setting.PriceX+priceChange, data.HighNear)
			} else {
				if data.LowToday > 0 {
					price = math.Min(data.LowFar, data.LowToday) + priceChange
				} else {
					price = data.LowFar + priceChange
				}
			}
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	priceDeal := price
	canOpen = (canOpen && float64(setting.Chance) < float64(setting.ChanceLimit)) || setting.Chance < 0
	if data.OrderLong == nil && canOpen {
		data.BreakLong = false
		if orderType == model.OrderTypeStop {
			if price <= tick.Asks[0].Price {
				data.BreakLong = true
				orderType = model.OrderTypeLimit
				price = tick.Asks[0].Price
			}
			priceDeal = price * (1 + api.TurtleTriggerDelta)
		} else if orderType == model.OrderTypeLimit && price >= tick.Asks[0].Price {
			data.BreakLong = true
			price = tick.Asks[0].Price
			priceDeal = tick.Asks[0].Price * (1 + api.TurtleTriggerDelta)
		}
		data.OrderLong = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, orderType, market, symbol, ``,
			setting.Function, priceDeal, price, amount, nil)
		if data.OrderAdjust == nil {
			data.OrderAdjust = make(map[string]*model.Order)
		}
		util.Notice(fmt.Sprintf(`place long %s %s %s %s %s %d %v at %e %e amt %e, useNear %v priceX %f n:%f seconds %d near %f %f far %f %f`,
			orderType, setting.Function, market, symbol, orderType, setting.Chance, canOpen, priceDeal, price, amount,
			data.UseNear, setting.PriceX, data.N, setting.Seconds, data.LowNear, data.HighNear, data.LowFar, data.HighFar))
		for _, order := range data.OrderLong {
			order.LineBuy = data.N
			order.LineSell = data.N
			order.Function = function
			order.GridPos = data.Big
			go model.AppDB.Save(order)
			if data.BreakLong && order.Status != model.CarryStatusSuccess {
				util.Notice(`already break long move to adjust %s %v`, order.OrderId, order)
				data.OrderAdjust[order.OrderId] = order
			}
		}
	}
}

// 海龟没持仓: 龟汤的平仓单数量：实际龟汤仓位大小，
// 海龟有持仓: 龟汤的平仓单数量：龟汤仓数*大单
func placeTurtleShort(account *model.Account, orderType string, data *model.TurtleData, setting *model.Setting,
	tick *model.BidAsk, canOpen, isBig, normalHolding bool) {
	amount := data.Amount
	function := model.Open
	if setting.Chance > 0 {
		amount = setting.GridAmount
		function = model.Close
		if setting.Function == model.FunctionCombineTurtle && normalHolding {
			amount = math.Abs(float64(setting.Chance)) * data.Amount
		}
		if setting.Function == model.FunctionCombineTurtle {
			amount = math.Max(math.Min(1.5*setting.GridAmount, amount), 0.85*setting.GridAmount)
		}
	} else if !isBig {
		amount = data.Amount / 2
	}
	price := data.LowFar
	priceChange := 2 * data.N
	if setting.Seconds == 14400 {
		priceChange = 3 * data.N
	}
	if orderType == model.OrderTypeLimit {
		price = data.HighFar - data.N/2
		if setting.Chance > 0 {
			if setting.Seconds == 14400 {
				priceChange = 2 * data.N
			}
			if data.UseNear {
				price = math.Min(setting.PriceX+priceChange, data.HighNear)
			} else {
				//if data.LowToday > 0 {
				//	price = math.Min(data.LowFar, data.LowToday) + 2*data.N
				//} else {
				//	price = data.LowFar + 2*data.N
				//}
				if setting.PriceX > 0 {
					//price = math.Min(data.LowFar, setting.PriceX) + priceChange
					price = setting.PriceX/3 + data.LowFar*2/3 + priceChange
				} else {
					price = data.LowFar + priceChange
				}
			}
		} else if setting.Chance < 0 {
			price = math.Max(data.HighFar, setting.PriceX+data.N/2)
		}
	} else if orderType == model.OrderTypeStop {
		if setting.Chance > 0 {
			if data.UseNear {
				price = math.Max(setting.PriceX-priceChange, data.LowNear)
			} else {
				price = math.Max(data.HighFar, data.HighToday) - priceChange
			}
		} else if setting.Chance < 0 {
			price = math.Min(data.LowFar, setting.PriceX-data.N/2)
		}
	}
	market := setting.Market
	symbol := setting.Symbol
	priceDeal := price
	canOpen = (canOpen && float64(setting.Chance) > -1*float64(setting.ChanceLimit)) || setting.Chance > 0
	if data.OrderShort == nil && canOpen {
		data.BreakShort = false
		if orderType == model.OrderTypeStop {
			if price >= tick.Bids[0].Price {
				data.BreakShort = true
				orderType = model.OrderTypeLimit
				price = tick.Bids[0].Price
			}
			priceDeal = price * (1 - api.TurtleTriggerDelta)
		} else if orderType == model.OrderTypeLimit && price <= tick.Bids[0].Price {
			data.BreakShort = true
			priceDeal = tick.Bids[0].Price * (1 - api.TurtleTriggerDelta)
			price = tick.Bids[0].Price
		}
		util.Notice(fmt.Sprintf(`place short %s %s %s %s %s %d %v at %e %e amt %e, useNear %v priceX %f n:%f seconds %d near %f %f far %f %f`,
			orderType, setting.Function, market, symbol, orderType, setting.Chance, canOpen, priceDeal, price, amount,
			data.UseNear, setting.PriceX, data.N, setting.Seconds, data.LowNear, data.HighNear, data.LowFar, data.HighFar))
		data.OrderShort = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, orderType, market, symbol, ``,
			setting.Function, priceDeal, price, amount, nil)
		if data.OrderAdjust == nil {
			data.OrderAdjust = make(map[string]*model.Order)
		}
		for _, order := range data.OrderShort {
			order.LineBuy = data.N
			order.LineSell = data.N
			order.Function = function
			order.GridPos = data.Big
			go model.AppDB.Save(order)
			if data.BreakShort && order.Status != model.CarryStatusSuccess {
				util.Notice(`already break short move to adjust %s %v`, order.OrderId, order)
				data.OrderAdjust[order.OrderId] = order
			}
		}
	}
}

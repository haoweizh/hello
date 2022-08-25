package carry

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strings"
	"sync"
	"time"
)

type TurtleData struct {
	turtleTime     time.Time
	checkTimeBreak time.Time
	checkTimeOpen  time.Time
	waitBreakLong  bool
	waitBreakShort bool
	breakLong      bool
	breakShort     bool
	highDays10     float64
	lowDays10      float64
	highDays20     float64
	lowDays20      float64
	highDays5      float64
	lowDays5       float64
	end1           float64
	n              float64
	amount         float64
	symbol         string
	orderLong      *model.Order
	orderShort     *model.Order
}

const turtleTriggerDelta = 0.01

var turtling = false
var turtleLock sync.Mutex

// 当天是否有平仓
var turtleClosed = make(map[string]map[string]bool) // market - symbol - closed

func (turtleData *TurtleData) ToString() (str string) {
	if turtleData == nil {
		return `turtle data is nil`
	}
	return fmt.Sprintf(`20日%f~%f n:%f amount:%f`, turtleData.lowDays20, turtleData.highDays20, turtleData.n, turtleData.amount)
}

func checkSetTurtling(value bool) (before bool) {
	turtleLock.Lock()
	defer turtleLock.Unlock()
	before = turtling
	if value == false || before == false {
		turtling = value
	}
	return before
}

var dataSet = make(map[string]map[string]map[string]*TurtleData) // market - symbol - 2019-12-06 - *turtleData

func calcTurtleAmount(key, secret string, setting *model.Setting, n float64) (amount float64) {
	var accountValue float64
	switch setting.Market {
	case model.BinancePerp:
		_, _, accountValue, _ = api.GetPositions(key, secret, setting.Market)
	case model.Ftx, model.OKEX:
		_, _, accountValue, _ = api.GetBalances(key, secret, setting.Market)
	}
	amount = 0.02 * accountValue / n
	_, _, coin, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	if !model.CommonCoins[strings.ToLower(coin)] {
		amount /= 2
	}
	util.Notice(`calcTurtleAmount %s %s %f`, setting.Market, setting.Symbol, amount)
	return amount
}

func GetTurtleData(key, secret string, setting *model.Setting) (turtleData *TurtleData) {
	today, todayStr := model.GetMarketToday(setting.Market)
	if dataSet[setting.Market] == nil {
		dataSet[setting.Market] = make(map[string]map[string]*TurtleData)
		turtleClosed[setting.Market] = make(map[string]bool)
	}
	if dataSet[setting.Market][setting.Symbol] == nil {
		dataSet[setting.Market][setting.Symbol] = make(map[string]*TurtleData)
	}
	if dataSet[setting.Market][setting.Symbol][todayStr] != nil {
		return dataSet[setting.Market][setting.Symbol][todayStr]
	}
	util.Notice(`need to create turtle ` + setting.Market + setting.Symbol)
	turtleClosed[setting.Market][setting.Symbol] = false
	duration, _ := time.ParseDuration(`-120s`)
	turtleData = &TurtleData{turtleTime: today, symbol: setting.Symbol, checkTimeBreak: util.GetNow(),
		checkTimeOpen: util.GetNow().Add(duration), waitBreakLong: false, waitBreakShort: false, breakLong: false, breakShort: false}
	//var longs, shorts []*model.Order
	//model.AppDB.Where("market= ? and symbol= ? and refresh_type= ? and amount>deal_amount and status=? and order_side=?",
	//	setting.Market, setting.Symbol, model.FunctionTurtle, model.CarryStatusWorking, model.OrderSideBuy).
	//	Order(`order_time desc`).Limit(int(setting.AmountLimit)).Find(&longs)
	//model.AppDB.Where("market= ? and symbol= ? and refresh_type= ? and amount>deal_amount and status=? and order_side=?",
	//	setting.Market, setting.Symbol, model.FunctionTurtle, model.CarryStatusWorking, model.OrderSideSell).
	//	Order(`order_time desc`).Limit(int(setting.AmountLimit)).Find(&shorts)
	//util.Notice(fmt.Sprintf(`load db turtle orders longs %d shorts %d`, len(longs), len(shorts)))
	//for _, order := range longs {
	//	if turtleData.orderLong == nil {
	//		turtleData.orderLong = order
	//	} else if order != nil && order.OrderId != `` && order.OrderTime.After(turtleData.orderLong.OrderTime) {
	//
	//	}
	//	if turtleData.orderLong == nil || (order != nil && order.OrderId != `` &&
	//		order.OrderTime.After(turtleData.orderLong.OrderTime)) {
	//		turtleData.orderLong = order
	//	}
	//}
	//for _, order := range shorts {
	//	if turtleData.orderShort == nil || (order != nil && order.OrderId != `` &&
	//		order.OrderTime.After(turtleData.orderShort.OrderTime)) {
	//		turtleData.orderShort = order
	//	}
	//}
	for i := 1; i < 21; i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		day := today.Add(duration)
		candle := api.GetDayCandle(key, secret, setting.Market, setting.Symbol, day)
		if candle == nil {
			if time.Now().Second() == 0 {
				util.Notice(`can not calc turtleDate as nil candle %s %s %s %s`,
					setting.Market, setting.Symbol, turtleData.symbol, day.String())
			}
			return nil
		}
		if i == 1 {
			turtleData.end1 = candle.PriceClose
		}
		if candle.PriceHigh > turtleData.highDays20 {
			turtleData.highDays20 = candle.PriceHigh
		}
		if turtleData.lowDays20 == 0 || turtleData.lowDays20 > candle.PriceLow {
			turtleData.lowDays20 = candle.PriceLow
		}
		if candle.PriceHigh > turtleData.highDays10 && i < 11 {
			turtleData.highDays10 = candle.PriceHigh
		}
		if (turtleData.lowDays10 == 0 || turtleData.lowDays10 > candle.PriceLow) && i < 11 {
			turtleData.lowDays10 = candle.PriceLow
		}
		if candle.PriceHigh > turtleData.highDays5 && i < 6 {
			turtleData.highDays5 = candle.PriceHigh
		}
		if (turtleData.lowDays5 == 0 || turtleData.lowDays5 > candle.PriceLow) && i < 6 {
			turtleData.lowDays5 = candle.PriceLow
		}
		if i == 1 {
			turtleData.n = candle.N
			turtleData.amount = calcTurtleAmount(key, secret, setting, turtleData.n)
		}
	}
	if turtleData.amount > 0 && turtleData.n > 0 {
		dataSet[setting.Market][setting.Symbol][todayStr] = turtleData
		util.Notice(fmt.Sprintf(`%s %s set turtle data: amount:%f n:%f end1:%f 20:%f %f 10:%f %f 5:%f %f`,
			setting.Market, setting.Symbol, turtleData.amount, turtleData.n, turtleData.end1, turtleData.lowDays20,
			turtleData.highDays20, turtleData.lowDays10, turtleData.highDays10, turtleData.lowDays5, turtleData.highDays5))
	}
	return
}

func checkTurtleOrders(key, secret string, setting *model.Setting, currentN float64, turtleData *TurtleData) (checked bool) {
	duration, _ := time.ParseDuration(`-120s`)
	now := util.GetNow().Add(duration)
	checked = false
	if now.After(turtleData.checkTimeOpen) {
		checked = true
		turtleData.checkTimeOpen = util.GetNow()
		orders := api.QueryOpenTriggerOrders(key, secret, setting.Market, setting.Symbol)
		if orders == nil {
			return
		}
		for _, order := range orders {
			if (turtleData.orderLong != nil && turtleData.orderLong.OrderId == order.OrderId &&
				(currentN < setting.AmountLimit || setting.Chance < 0)) ||
				(turtleData.orderShort != nil && turtleData.orderShort.OrderId == order.OrderId &&
					(currentN > -1*setting.AmountLimit || setting.Chance > 0)) {
				continue
			}
			result := api.MustCancel(key, secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, true)
			util.Notice(`cancel extra turtle order %s %s %s %s return %v`,
				setting.Market, setting.Symbol, order.OrderType, order.OrderId, result)
			time.Sleep(time.Second)
		}
	}
	return
}

// ProcessTurtle
// setting.GridAmount 当前已经持仓数量
// setting.Chance 当前开仓的个数
// setting.PriceX 上一次开仓的价格
// setting.OpenShortMargin 该单币种最多开仓个数
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
	turtleData := GetTurtleData(account.Key, account.Secret, setting)
	if turtleData == nil || turtleData.n == 0 || turtleData.amount == 0 {
		util.Notice(fmt.Sprintf(`fail to get turtle %s %s`, setting.Market, setting.Symbol))
		time.Sleep(time.Second * 30)
		return
	}
	currentN := api.GetCurrentN(setting)
	if checkTurtleOrders(account.Key, account.Secret, setting, float64(currentN), turtleData) {
		return
	}
	showMsg := fmt.Sprintf("%s_%s_%s", model.FunctionTurtle, setting.Market, setting.Symbol)
	model.SetCarryInfo(account.Key, showMsg, fmt.Sprintf("[海龟参数]%s %s 次数限制:%f 当前已经持仓数量:%f 上一次开仓的价格:%f "+
		"20日:%f-%f 10日:%f-%f n:%f 数量:%f %s 持仓数/限制:%d/%f 总持仓数%d bid-ask %f %f 当日有平仓：%v",
		turtleData.turtleTime.String()[0:10], showMsg, setting.AmountLimit, setting.GridAmount, setting.PriceX,
		turtleData.lowDays20, turtleData.highDays20, turtleData.lowDays10, turtleData.highDays10, turtleData.n,
		turtleData.amount, setting.Symbol, setting.Chance, setting.OpenShortMargin, currentN, tick.Bids[0].Price,
		tick.Asks[0].Price, turtleClosed[setting.Market][setting.Symbol]))
	priceLong := turtleData.highDays20
	priceShort := turtleData.lowDays20
	if checkTurtleBreak(account.Key, account.Secret, setting, turtleData, tick) {
		return
	}
	if setting.Chance == 0 && !turtleClosed[setting.Market][setting.Symbol] { // 开初始仓
		placeTurtleOrders(account.Key, account.Secret, turtleData, setting, currentN, priceShort, priceLong, tick)
		if turtleData.breakLong && turtleData.waitBreakLong {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideBuy)
			setting.Chance = 1
			setting.GridAmount = turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破20日高点 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort,
				priceLong, setting.PriceX, turtleData.n))
		}
		if turtleData.breakShort && turtleData.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideSell)
			setting.Chance = -1
			setting.GridAmount = turtleData.amount
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(
				`破20日低点 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort,
				priceLong, setting.PriceX, turtleData.n))
		}
	} else if setting.Chance > 0 {
		priceLong = math.Max(turtleData.highDays20, setting.PriceX+turtleData.n/2)
		if turtleData.lowDays10 < setting.PriceX-2*turtleData.n {
			if setting.PriceX-2*turtleData.n < tick.Bids[0].Price {
				priceShort = setting.PriceX - 2*turtleData.n
			} else {
				priceShort = turtleData.lowDays10
			}
		} else if turtleData.lowDays10 < setting.PriceX {
			priceShort = turtleData.lowDays10
		} else if turtleData.lowDays10 > setting.PriceX {
			if turtleData.highDays10-2*turtleData.n < tick.Bids[0].Price {
				priceShort = math.Max(turtleData.lowDays10, turtleData.highDays10-2*turtleData.n)
			} else {
				priceShort = turtleData.lowDays10
			}
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
		} // 平多
		if turtleData.breakShort && turtleData.waitBreakShort {
			handleBreak(account.Key, account.Secret, setting, turtleData, model.OrderSideSell)
			go api.SendMails(`平多`+setting.Market+setting.Symbol,
				fmt.Sprintf(`止盈止损at%f 仓位%d 数量 %f`, priceShort, setting.Chance, setting.GridAmount))
			turtleClosed[setting.Market][setting.Symbol] = true
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
		priceShort = math.Min(turtleData.lowDays20, setting.PriceX-turtleData.n/2)
		if turtleData.highDays10 > setting.PriceX+2*turtleData.n {
			if setting.PriceX+2*turtleData.n > tick.Asks[0].Price {
				priceLong = setting.PriceX + 2*turtleData.n
			} else {
				priceLong = turtleData.highDays10
			}
		} else if turtleData.highDays10 > setting.PriceX {
			priceLong = turtleData.highDays10
		} else if turtleData.highDays10 < setting.PriceX {
			if turtleData.lowDays10+2*turtleData.n > tick.Asks[0].Price {
				priceLong = math.Min(turtleData.highDays10, turtleData.lowDays10+2*turtleData.n)
			} else {
				priceLong = turtleData.highDays10
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
			turtleClosed[setting.Market][setting.Symbol] = true
			model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
				setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{
				`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
			util.Notice(fmt.Sprintf(`liquidate short result: %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, setting.GridAmount, currentN, priceShort, priceLong,
				setting.PriceX, turtleData.n))
		}
	}
}

func setTurtleOrderStatus(function, market, symbol, orderId, status string) {
	setting := api.GetSetting(function, market, symbol)
	if setting == nil {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	if account == nil {
		return
	}
	turtleData := GetTurtleData(account.Key, account.Secret, setting)
	if turtleData != nil && turtleData.orderLong != nil && turtleData.orderLong.OrderId == orderId {
		turtleData.orderLong.Status = status
	}
	if turtleData != nil && turtleData.orderShort != nil && turtleData.orderShort.OrderId == orderId {
		turtleData.orderShort.Status = status
	}
}

func checkTurtleBreak(key, secret string, setting *model.Setting, turtleData *TurtleData, tick *model.BidAsk) (checked bool) {
	duration, _ := time.ParseDuration(`-5s`)
	now := util.GetNow().Add(duration)
	checked = false
	if now.After(turtleData.checkTimeBreak) {
		turtleData.checkTimeBreak = util.GetNow()
		if turtleData.orderLong != nil && (turtleData.orderLong.Status == model.CarryStatusSuccess ||
			(turtleData.orderLong.TriggerPrice > 0 && turtleData.orderLong.TriggerPrice <= tick.Bids[0].Price)) {
			util.Notice(fmt.Sprintf(`-----chance %s %s %d bid-ask %f %f short %f`,
				setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price, turtleData.orderLong.TriggerPrice))
			order := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, turtleData.orderLong.OrderType, turtleData.orderLong.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				turtleData.breakLong = true
				util.Notice(fmt.Sprintf(`-----order break long %s %s %d bid-ask %f %f short %f %v %v`,
					setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					turtleData.orderLong.Price, turtleData.breakLong, turtleData.waitBreakLong))
			}
			checked = true
		}
		if turtleData.orderShort != nil && (turtleData.orderShort.Status == model.CarryStatusSuccess ||
			(turtleData.orderShort.TriggerPrice > 0 && turtleData.orderShort.TriggerPrice >= tick.Asks[0].Price)) {
			util.Notice(fmt.Sprintf(`-----chance %s %s %d bid-ask %f %f long %f`,
				setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price, turtleData.orderShort.TriggerPrice))
			order := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, turtleData.orderShort.OrderType, turtleData.orderShort.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				turtleData.breakShort = true
				util.Notice(fmt.Sprintf(`-----order break short %s %s %d bid-ask %f %f long %f %v %v`,
					setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					turtleData.orderShort.Price, turtleData.breakShort, turtleData.waitBreakShort))
			}
			checked = true
		}
	}
	return checked
}

func handleBreak(key, secret string, setting *model.Setting, turtleData *TurtleData, orderSide string) {
	if turtleData == nil {
		//util.Notice(fmt.Sprintf(`fatal error, nil order to break`))
		return
	}
	turtleData.waitBreakLong = false
	turtleData.waitBreakShort = false
	orderQuery := turtleData.orderShort
	if orderSide == model.OrderSideBuy {
		orderQuery = turtleData.orderLong
	}
	if orderQuery != nil {
		time.Sleep(time.Second * 3)
		util.Notice(fmt.Sprintf(`query turtle break %s %s`, orderSide, orderQuery.OrderId))
		setting.PriceX = orderQuery.TriggerPrice
		turtleData.orderLong = nil
		turtleData.orderShort = nil
		if orderSide == model.OrderSideBuy {
			if turtleData.orderShort != nil {
				temp := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, turtleData.orderShort.
					OrderType, turtleData.orderShort.OrderId)
				if temp != nil && temp.Status == model.CarryStatusWorking {
					go api.MustCancel(key, secret, turtleData.orderShort.Market, turtleData.orderShort.Symbol,
						turtleData.orderShort.OrderType, turtleData.orderShort.OrderId, true)
				}
			}
			util.Notice(fmt.Sprintf(`clear %s %s shorts %v`, setting.Market, setting.Symbol, turtleData.orderShort))
		} else {
			if turtleData.orderLong != nil {
				temp := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, turtleData.orderLong.OrderType,
					turtleData.orderLong.OrderId)
				if temp != nil && temp.Status == model.CarryStatusWorking {
					go api.MustCancel(key, secret, turtleData.orderLong.Market, turtleData.orderLong.Symbol,
						turtleData.orderLong.OrderType, turtleData.orderLong.OrderId, true)
				}
			}
			util.Notice(fmt.Sprintf(`clear %s %s long %v`, setting.Market, setting.Symbol, turtleData.orderLong))
		}
	}
}

func placeTurtleOrders(key, secret string, turtleData *TurtleData, setting *model.Setting,
	currentN int64, priceShort, priceLong float64, tick *model.BidAsk) {
	amountLimit := int64(setting.AmountLimit)
	coinLimit := int64(setting.OpenShortMargin)
	if turtleData.orderLong == nil && ((currentN < amountLimit && setting.Chance < coinLimit) || setting.Chance < 0) {
		liquidation := false
		orderSide := model.OrderSideBuy
		typeLong := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance < 0 {
			liquidation = true
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平空 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, currentN, priceShort,
				priceLong, setting.PriceX, turtleData.n))
		}
		if setting.SymbolRelated != model.SettingTurtleRemoved || setting.Chance < 0 {
			util.Notice(fmt.Sprintf(`%s %s place多单 chance:%d amount:%f priceX:%f currentN-limit:%d %f
			orderSide:%s end1:%f h20:%f h10:%f h5:%f l20:%f l10:%f l5%f coin limit:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, setting.PriceX, currentN, setting.AmountLimit,
				orderSide, turtleData.end1, turtleData.highDays20, turtleData.highDays10, turtleData.highDays5,
				turtleData.lowDays20, turtleData.lowDays10, turtleData.lowDays5, setting.OpenShortMargin))
			var order *model.Order
			priceOut := false
			if priceLong <= tick.Asks[0].Price {
				order = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, tick.Asks[0].Price*1.01, priceLong, amount, setting)
				priceOut = true
			} else {
				order = api.MustPlaceOrder(key, secret, orderSide, typeLong, setting.Market, setting.Symbol, ``, model.FunctionTurtle,
					priceLong*(1+turtleTriggerDelta), priceLong, amount, setting)
			}
			go model.AppDB.Save(order)
			if order != nil && order.OrderId != `` && order.Status != model.CarryStatusFail {
				if !liquidation {
					turtleData.amount = order.Amount
				}
				turtleData.orderLong = order
				turtleData.waitBreakLong = true
				turtleData.breakLong = false
				if priceOut {
					turtleData.breakLong = true
				}
			}
		}
	}
	if turtleData.orderShort == nil && ((currentN > -1*amountLimit && setting.Chance > -1*coinLimit) || setting.Chance > 0) {
		liquidation := false
		orderSide := model.OrderSideSell
		typeShort := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance > 0 {
			liquidation = true
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平多 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, currentN, priceShort,
				priceLong, setting.PriceX, turtleData.n))
		}
		if setting.SymbolRelated != model.SettingTurtleRemoved || setting.Chance > 0 {
			util.Notice(fmt.Sprintf(`%s %s place空单 chance:%d amount:%f priceX:%f currentN-limit:%d %f 
			orderSide:%s end1:%f h20:%f h10:%f h5:%f l20:%f l10:%f l5%f coin limit:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, setting.PriceX, currentN, setting.AmountLimit,
				orderSide, turtleData.end1, turtleData.highDays20, turtleData.highDays10, turtleData.highDays5,
				turtleData.lowDays20, turtleData.lowDays10, turtleData.lowDays5, setting.OpenShortMargin))
			var order *model.Order
			priceOut := false
			if priceShort >= tick.Bids[0].Price {
				order = api.MustPlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, tick.Bids[0].Price*0.99, priceShort, amount, setting)
				priceOut = true
			} else {
				order = api.MustPlaceOrder(key, secret, orderSide, typeShort, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, priceShort*(1-turtleTriggerDelta), priceShort, amount, setting)
			}
			go model.AppDB.Save(order)
			if order != nil && order.OrderId != `` && order.Status != model.CarryStatusFail {
				if !liquidation {
					turtleData.amount = order.Amount
				}
				turtleData.orderShort = order
				turtleData.waitBreakShort = true
				turtleData.breakShort = false
				if priceOut {
					turtleData.breakShort = true
				}
			}
		}
	}
}

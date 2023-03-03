package Turtle

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

type Data struct {
	// useNear是否在海龟交易时使用lowDaysNear和highDaysNear和priceX作为触发条件
	// adjustChecked在设置为true前，不允许使用本Data进行交易
	useNear, waitBreakLong, waitBreakShort, breakLong, breakShort, liquidated, adjustChecked, orderCleared bool
	turtleTime, checkTimeBreak, checkTimeOpen                                                              time.Time
	highDaysNear, lowDaysNear, highDaysFar, lowDaysFar, lowAdjust, highAdjust                              float64
	highToday, lowToday, n, amount                                                                         float64
	daysNear, daysFar, daysAdjust                                                                          int
	symbol                                                                                                 string
	// 适应某些交易所单笔订单不能过大，大笔订单会拆分后下成多个，因价格超出无法下成的单为了不被取消，也归入orderAdjust
	orderLong, orderShort, orderAdjust []*model.Order
}

const turtleTriggerDelta = 0.01

var turtling = false
var turtleLock sync.Mutex
var turtleDataSet = sync.Map{}  // function_market_symbol_2019-12-06 *Data
var queryDataTime = &sync.Map{} // function_market_symbol_2019-12-06 time

func (turtleData *Data) ToString() (str string) {
	if turtleData == nil {
		return `turtle data is nil`
	}
	return fmt.Sprintf(`%d日%f~%f n:%e amount:%f`,
		turtleData.daysFar, turtleData.lowDaysFar, turtleData.highDaysFar, turtleData.n, turtleData.amount)
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

func calcTurtleAmount(key, secret, market, symbol string, n float64) (amount float64) {
	var accountValue float64
	switch market {
	case model.BinancePerp:
		_, _, accountValue, _ = api.GetPositions(key, secret, market)
	case model.Ftx, model.OKEX:
		_, _, accountValue, _ = api.GetBalances(key, secret, market)
	}
	amount = 0.02 * accountValue / n
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	if model.CommonCoins[strings.ToLower(coin)] {
		amount = amount / 2
	} else {
		amount /= 4
	}
	//util.Notice(`calcTurtleAmount %s %s %f`, setting.Market, setting.Symbol, amount)
	return amount
}

// 取消market交易所中symbol交易对的所有limit、stop单
func clearOrders(key, secret, market, symbol string) {
	ordersLimit := api.QueryOpenOrders(key, secret, market, symbol, false)
	ordersStop := api.QueryOpenOrders(key, secret, market, symbol, true)
	for _, order := range ordersLimit {
		if order != nil {
			util.Notice(`cancel pending turtle limit order %s %s %s`, market, symbol, order.OrderId)
			api.MustCancel(key, secret, market, symbol, model.OrderTypeLimit, order.OrderId, true)
		}
	}
	for _, order := range ordersStop {
		if order != nil {
			util.Notice(`cancel pending turtle stop order %s %s %s`, market, symbol, order.OrderId)
			api.MustCancel(key, secret, market, symbol, model.OrderTypeStop, order.OrderId, true)
		}
	}
}

// 取消market交易所中symbol交易对中没有被纳入管理或已经超出仓数限制的订单
func clearExtraOrders(key, secret, market, symbol string, currentNum float64, settings []*model.Setting,
	data []*Data) {
	keepOrders := make(map[string]bool)
	for i, setting := range settings {
		if currentNum < setting.AmountLimit || setting.Chance < 0 {
			for _, order := range data[i].orderLong {
				keepOrders[order.OrderId] = true
			}
		}
		if currentNum > -1*setting.AmountLimit || setting.Chance > 0 {
			for _, order := range data[i].orderShort {
				keepOrders[order.OrderId] = true
			}
		}
		for _, order := range data[i].orderAdjust {
			keepOrders[order.OrderId] = true
		}
	}
	ordersStop := api.QueryOpenOrders(key, secret, market, symbol, true)
	for _, order := range ordersStop {
		if !keepOrders[order.OrderId] {
			result := api.MustCancel(key, secret, market, symbol, order.OrderType, order.OrderId, true)
			util.Notice(`cancel extra turtle order %s %s %s %s return %v`, market, symbol, order.OrderType, order.OrderId, result)
			time.Sleep(time.Second)
		}
	}
	ordersLimit := api.QueryOpenOrders(key, secret, market, symbol, false)
	for _, order := range ordersLimit {
		if !keepOrders[order.OrderId] {
			result := api.MustCancel(key, secret, market, symbol, order.OrderType, order.OrderId, true)
			util.Notice(`cancel extra turtle order %s %s %s %s return %v`, market, symbol, order.OrderType, order.OrderId, result)
			time.Sleep(time.Second)
		}
	}
}

func adjustPosHolding(key, secret string, setting *model.Setting, data *Data) {
	data.adjustChecked = true
	success, marketPos, _, _ := api.GetPositions(key, secret, setting.Market)
	if !success {
		util.Notice(fmt.Sprintf(`fail to adjust position holdings %s %s`, setting.Market, setting.Symbol))
		return
	}
	posMap := make(map[string]*model.Position)
	for _, pos := range marketPos {
		posMap[strings.ToUpper(pos.Currency)] = pos
	}
	if posMap[setting.Symbol] != nil { //setting.Chance和pos.Holding相乘小于零代表方向相反，此时设置为0
		if float64(setting.Chance)*posMap[setting.Symbol].Holding <= 0 {
			util.Notice(`update turtle side %s %s %d %f %f from %d`,
				setting.Market, setting.Symbol, setting.Chance, posMap[setting.Symbol].Holding, setting.GridAmount, setting.Chance)
			setting.GridAmount = 0
			setting.Chance = 0
			if posMap[setting.Symbol].Holding > 0 {
				data.orderAdjust = api.MustPlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtleAdjust, data.lowAdjust*(1-turtleTriggerDelta), data.lowAdjust, posMap[setting.Symbol].Holding, setting)
			} else if posMap[setting.Symbol].Holding < 0 {
				data.orderAdjust = api.MustPlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtleAdjust, data.highAdjust*(1+turtleTriggerDelta), data.highAdjust, -1*posMap[setting.Symbol].Holding, setting)
			}
			for _, order := range data.orderAdjust {
				if order != nil {
					order.RefreshType = model.FunctionTurtleAdjust
					model.AppDB.Save(order)
				}
			}
		} else if setting.GridAmount != math.Abs(posMap[setting.Symbol].Holding) {
			util.Notice(`update turtle grid amount %s %s %f to %f`,
				setting.Market, setting.Symbol, setting.GridAmount, posMap[setting.Symbol].Holding)
			setting.GridAmount = math.Abs(posMap[setting.Symbol].Holding)
		}
	} else {
		setting.GridAmount = 0
		setting.Chance = 0
		util.Notice(`update turtle when absent %s %s %d`, setting.Market, setting.Symbol, len(posMap))
		for s, position := range posMap {
			util.Notice(`present %s %s %f`, s, position.Currency, position.Holding)
		}
	}
	model.AppDB.Save(setting)
}

func handleTraceOrders(key, secret, market, symbol string, settings []*model.Setting, turtleData []*Data,
	currentNum float64) (checked bool) {
	if (len(settings) != 2 && len(settings) != 1) || len(settings) != len(turtleData) {
		util.Notice(`wrong combine turtle parameter`)
		return true
	}
	if turtleData[0].checkTimeOpen.Add(time.Minute * 10).After(util.GetNow()) {
		return false
	}
	if len(settings) == 1 && !turtleData[0].adjustChecked {
		adjustPosHolding(key, secret, settings[0], turtleData[0])
	} else if len(settings) == 2 {
		if settings[0].Chance == 0 && !turtleData[1].adjustChecked {
			adjustPosHolding(key, secret, settings[1], turtleData[1])
		} else if settings[1].Chance == 0 && !turtleData[0].adjustChecked {
			adjustPosHolding(key, secret, settings[0], turtleData[0])
		}
		turtleData[0].adjustChecked = true
		turtleData[1].adjustChecked = true
	}
	today, _ := model.GetMarketToday(market)
	dayTime, _ := time.ParseDuration(`86400s`)
	var candles []*model.Candle
	// okex不返回尚未结束的当日candle，转成半小时的slot
	if market == model.OKEX {
		candles = api.GetCandle(key, secret, market, symbol, 1800, today, model.GetMarketNow(market))
	} else {
		candles = api.GetCandle(key, secret, market, symbol, 86400, today, today.Add(dayTime))
	}
	for i, setting := range settings {
		data := turtleData[i]
		for j := 0; candles != nil && j < len(candles); j++ {
			if data.highToday < candles[j].PriceHigh {
				data.highToday = candles[j].PriceHigh
				util.Info(fmt.Sprintf(`get today len new high %s %s %d %f`, market, symbol, len(candles), candles[j].PriceHigh))
			}
			if data.lowToday == 0 || data.lowToday > candles[j].PriceLow {
				data.lowToday = candles[j].PriceLow
				util.Info(fmt.Sprintf(`get today len new low %s %s %d %f`, market, symbol, len(candles), candles[j].PriceLow))
			}
		}
		if data.orderShort == nil || len(data.orderShort) == 0 {
			data.orderShort = nil
		} else if !data.useNear && setting.Chance > 0 && data.lowToday > 0 &&
			((data.orderShort[0].OrderType == model.OrderTypeLimit && data.orderShort[0].Price > math.Min(data.lowToday, data.lowDaysFar)+2*data.n) ||
				(data.orderShort[0].OrderType == model.OrderTypeStop && data.orderShort[0].TriggerPrice < math.Max(data.highDaysFar, data.highToday)-2*data.n)) {
			util.Notice(fmt.Sprintf(`today higher than far price%f<max(today%f,far%f)-2*%f chance%d`,
				data.orderShort[0].TriggerPrice, data.highToday, data.highDaysFar, data.n, setting.Chance))
			data.orderShort = nil
		}
		if data.orderLong == nil || len(data.orderLong) == 0 {
			data.orderLong = nil
		} else if !data.useNear && setting.Chance < 0 && data.lowToday > 0 &&
			((data.orderLong[0].OrderType == model.OrderTypeLimit && data.orderLong[0].Price < math.Max(data.highDaysFar, data.highToday)-2*data.n) ||
				(data.orderLong[0].OrderType == model.OrderTypeStop && data.orderLong[0].TriggerPrice > math.Min(data.lowDaysFar, data.lowToday)+2*data.n)) {
			util.Notice(fmt.Sprintf(`today lower than far price%f>min(today%f,far%f)+2*%f chance%d`,
				data.orderLong[0].TriggerPrice, data.lowToday, data.lowDaysFar, data.n, setting.Chance))
			data.orderLong = nil
		}
		data.checkTimeOpen = util.GetNow()
	}
	clearExtraOrders(key, secret, market, symbol, currentNum, settings, turtleData)
	return true
}

func GetTurtleData(key, secret, function, market, symbol string, useNear bool) (data *Data) {
	today, todayStr := model.GetMarketToday(market)
	value, ok := util.LoadSyncMap(&turtleDataSet, function, market, symbol, todayStr)
	if ok && value != nil {
		return value.(*Data)
	}
	value, ok = util.LoadSyncMap(queryDataTime, function, market, symbol, todayStr)
	if ok && value != nil {
		if value.(time.Time).Add(time.Minute * 30).After(util.GetNow()) {
			return nil
		}
	}
	util.Notice(fmt.Sprintf(`need to create turtle data %s %s %s %s`, function, market, symbol, todayStr))
	util.StoreSyncMap(queryDataTime, util.GetNow(), function, market, symbol, todayStr)
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	far := 18
	if strings.ToUpper(coin) == `BTC` {
		far = 50
	}
	data = &Data{turtleTime: today, symbol: symbol, checkTimeBreak: util.GetNow(),
		checkTimeOpen: util.GetNow(), waitBreakLong: false, waitBreakShort: false, breakLong: false,
		breakShort: false, liquidated: false, daysFar: far, daysNear: far / 2, daysAdjust: 5, useNear: useNear}
	indexMax := math.Max(21.0, float64(data.daysFar))
	duration, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*int(indexMax)))
	candles := api.GetCandle(key, secret, market, symbol, 86400, today.Add(duration), today)
	keyedCandles := make(map[string]*model.Candle)
	for _, item := range candles {
		candleKey := fmt.Sprintf(`%s_%s_%d_%s`, market, symbol, item.Seconds, item.Begin.Format(time.RFC3339))
		keyedCandles[candleKey] = item
	}
	for i := 1; i < int(indexMax); i++ {
		duration, _ = time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		day := today.Add(duration)
		candle := api.GetTurtleCandle(market, symbol, 86400, day, keyedCandles)
		if candle == nil || candle.PriceHigh == 0 || candle.PriceLow == 0 {
			if time.Now().Second() == 0 {
				util.Notice(`can not calc turtleDate as nil candle %s %s %s %s`,
					market, symbol, data.symbol, day.String())
			}
			return nil
		}
		if candle.PriceHigh > data.highDaysFar && i <= data.daysFar {
			data.highDaysFar = candle.PriceHigh
		}
		if (data.lowDaysFar == 0 || data.lowDaysFar > candle.PriceLow) && i <= data.daysFar {
			data.lowDaysFar = candle.PriceLow
		}
		if candle.PriceHigh > data.highDaysNear && i <= data.daysNear {
			data.highDaysNear = candle.PriceHigh
		}
		if (data.lowDaysNear == 0 || data.lowDaysNear > candle.PriceLow) && i <= data.daysNear {
			data.lowDaysNear = candle.PriceLow
		}
		if candle.PriceHigh > data.highAdjust && i <= data.daysAdjust {
			data.highAdjust = candle.PriceHigh
		}
		if (data.lowAdjust == 0 || data.lowAdjust > candle.PriceLow) && i <= data.daysAdjust {
			data.lowAdjust = candle.PriceLow
		}
		if i == 1 {
			data.n = candle.N
			data.amount = calcTurtleAmount(key, secret, market, symbol, data.n)
		}
	}
	if data.amount > 0 && data.n > 0 {
		util.StoreSyncMap(&turtleDataSet, data, function, market, symbol, todayStr)
		util.Notice(fmt.Sprintf(`%s %s %s %s set turtle data: amount:%f n:%e %d:%f-%f %d:%f-%f`,
			function, market, symbol, todayStr, data.amount, data.n, data.daysNear, data.lowDaysNear,
			data.highDaysNear, data.daysFar, data.lowDaysFar, data.highDaysFar))
	}
	return
}

func SetTurtleOrderStatus(function, market, symbol, orderId, status string) {
	setting := api.GetSetting(function, market, symbol)
	if setting == nil {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	if account == nil {
		return
	}
	_, todayStr := model.GetMarketToday(market)
	value, ok := util.LoadSyncMap(&turtleDataSet, function, market, symbol, todayStr)
	if ok && value != nil {
		turtleData := value.(*Data)
		if turtleData.orderLong != nil {
			for _, order := range turtleData.orderLong {
				if order.OrderId == orderId {
					order.Status = status
				}
			}
		}
		if turtleData.orderShort != nil {
			for _, order := range turtleData.orderShort {
				if order.OrderId == orderId {
					order.Status = status
				}
			}
		}
	}
}

func checkBreak(key, secret, market, symbol string, settings []*model.Setting, turtleData []*Data,
	tick *model.BidAsk) (checked bool) {
	if (len(settings) != 2 && len(settings) != 1) || len(settings) != len(turtleData) {
		util.Notice(`wrong combine turtle parameter`)
		return true
	}
	if turtleData[0].checkTimeBreak.Add(time.Minute).After(util.GetNow()) {
		return false
	}
	for i, setting := range settings {
		data := turtleData[i]
		data.checkTimeBreak = util.GetNow()
		var orderLong, orderShort *model.Order
		if data.orderLong != nil && len(data.orderLong) > 0 {
			orderLong = data.orderLong[0]
		}
		if data.orderShort != nil && len(data.orderShort) > 0 {
			orderShort = data.orderShort[0]
		}
		if orderLong != nil && (orderLong.Status == model.CarryStatusSuccess || (orderLong.TriggerPrice > 0 &&
			(orderLong.OrderType == model.OrderTypeStop && orderLong.TriggerPrice <= tick.Bids[0].Price) ||
			(orderLong.OrderType == model.OrderTypeLimit && orderLong.Price > tick.Bids[0].Price))) {
			util.Notice(fmt.Sprintf(`-----chance type: %s %s %s %d bid-ask %f %f short %f %f`,
				market, symbol, orderLong.OrderType, setting.Chance, tick.Bids[0].Price,
				tick.Asks[0].Price, orderLong.TriggerPrice, orderLong.Price))
			order := api.QueryOrderById(key, secret, market, symbol, orderLong.OrderType, orderLong.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				data.breakLong = true
				util.Notice(fmt.Sprintf(`-----order break long %s %s %d bid-ask %f %f short %f %v %v`,
					market, symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					orderLong.Price, data.breakLong, data.waitBreakLong))
			}
			if order != nil && order.Status == model.CarryStatusFail {
				data.orderLong = nil
			}
		}
		if orderShort != nil && (orderShort.Status == model.CarryStatusSuccess || (orderShort.TriggerPrice > 0 &&
			(orderShort.OrderType == model.OrderTypeStop && orderShort.TriggerPrice >= tick.Asks[0].Price)) ||
			(orderShort.OrderType == model.OrderTypeLimit && orderShort.Price < tick.Asks[0].Price)) {
			util.Notice(fmt.Sprintf(`-----chance type: %s %s %s %d bid-ask %f %f long %f %f`,
				market, symbol, orderShort.OrderType, setting.Chance, tick.Bids[0].Price,
				tick.Asks[0].Price, orderShort.TriggerPrice, orderShort.Price))
			order := api.QueryOrderById(key, secret, market, symbol, orderShort.OrderType, orderShort.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				data.breakShort = true
				util.Notice(fmt.Sprintf(`-----order break short %s %s %d bid-ask %f %f long %f %v %v`,
					market, symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					orderShort.Price, data.breakShort, data.waitBreakShort))
			}
			if order != nil && order.Status == model.CarryStatusFail {
				data.orderShort = nil
			}
		}
	}
	return true
}

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
	useNear, waitBreakLong, waitBreakShort, breakLong, breakShort, liquidated, adjustChecked bool
	turtleTime, checkTimeBreak, checkTimeOpen                                                time.Time
	highDaysNear, lowDaysNear, highDaysFar, lowDaysFar, lowAdjust, highAdjust                float64
	highToday, lowToday, n, amount                                                           float64
	daysNear, daysFar, daysAdjust                                                            int
	symbol                                                                                   string
	// 适应某些交易所单笔订单不能过大，大笔订单会拆分后下成多个
	orderLong, orderShort, orderAdjust []*model.Order
}

const turtleTriggerDelta = 0.01

var turtling = false
var turtleLock sync.Mutex
var turtleDataSet *sync.Map // function_market_symbol_2019-12-06 *turtleData

func (turtleData *Data) ToString() (str string) {
	if turtleData == nil {
		return `turtle data is nil`
	}
	return fmt.Sprintf(`%d日%f~%f n:%f amount:%f`,
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
	}
	orders := api.QueryOpenOrders(key, secret, market, symbol, true)
	for _, order := range orders {
		if !keepOrders[order.OrderSide] {
			result := api.MustCancel(key, secret, market, symbol, order.OrderType, order.OrderId, true)
			util.Notice(`cancel extra turtle order %s %s %s %s return %v`, market, symbol, order.OrderType, order.OrderId, result)
			time.Sleep(time.Second)
		}
	}
}

func adjustPosHolding(key, secret string, setting *model.Setting, turtleData *Data) {
	turtleData.adjustChecked = true
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
				turtleData.orderAdjust = api.MustPlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, turtleData.lowAdjust*(1-turtleTriggerDelta), turtleData.lowAdjust, posMap[setting.Symbol].Holding, setting)
			} else if posMap[setting.Symbol].Holding < 0 {
				turtleData.orderAdjust = api.MustPlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtle, turtleData.highAdjust*(1+turtleTriggerDelta), turtleData.highAdjust, -1*posMap[setting.Symbol].Holding, setting)
			}
			for _, order := range turtleData.orderAdjust {
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

func GetTurtleData(key, secret, function, market, symbol string, useNear bool) (turtleData *Data) {
	today, todayStr := model.GetMarketToday(market)
	//util.Notice(`need to create turtle ` + setting.Market + setting.Symbol)
	value, ok := util.LoadSyncMap(turtleDataSet, function, market, symbol, todayStr)
	if ok && value != nil {
		return value.(*Data)
	}
	clearOrders(key, secret, market, symbol)
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	far := 18
	if strings.ToUpper(coin) == `BTC` {
		far = 50
	}
	turtleData = &Data{turtleTime: today, symbol: symbol, checkTimeBreak: util.GetNow(),
		checkTimeOpen: util.GetNow(), waitBreakLong: false, waitBreakShort: false, breakLong: false,
		breakShort: false, liquidated: false, daysFar: far, daysNear: far / 2, daysAdjust: 5, useNear: useNear}
	indexMax := math.Max(21.0, float64(turtleData.daysFar))
	for i := 1; i < int(indexMax); i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		day := today.Add(duration)
		candle := api.GetTurtleCandle(key, secret, market, symbol, 86400, day)
		if candle == nil || candle.PriceHigh == 0 || candle.PriceLow == 0 {
			if time.Now().Second() == 0 {
				util.Notice(`can not calc turtleDate as nil candle %s %s %s %s`,
					market, symbol, turtleData.symbol, day.String())
			}
			return nil
		} else {
			util.Notice(fmt.Sprintf(`get candle for turtle data %s %s %s price %f - %f`,
				market, symbol, day.String(), candle.PriceLow, candle.PriceHigh))
		}
		if candle.PriceHigh > turtleData.highDaysFar && i <= turtleData.daysFar {
			turtleData.highDaysFar = candle.PriceHigh
		}
		if (turtleData.lowDaysFar == 0 || turtleData.lowDaysFar > candle.PriceLow) && i <= turtleData.daysFar {
			turtleData.lowDaysFar = candle.PriceLow
		}
		if candle.PriceHigh > turtleData.highDaysNear && i <= turtleData.daysNear {
			turtleData.highDaysNear = candle.PriceHigh
		}
		if (turtleData.lowDaysNear == 0 || turtleData.lowDaysNear > candle.PriceLow) && i <= turtleData.daysNear {
			turtleData.lowDaysNear = candle.PriceLow
		}
		if candle.PriceHigh > turtleData.highAdjust && i <= turtleData.daysAdjust {
			turtleData.highAdjust = candle.PriceHigh
		}
		if (turtleData.lowAdjust == 0 || turtleData.lowAdjust > candle.PriceLow) && i <= turtleData.daysAdjust {
			turtleData.lowAdjust = candle.PriceLow
		}
		if i == 1 {
			turtleData.n = candle.N
			turtleData.amount = calcTurtleAmount(key, secret, market, symbol, turtleData.n)
		}
	}
	if turtleData.amount > 0 && turtleData.n > 0 {
		util.StoreSyncMap(turtleDataSet, turtleData, function, market, symbol)
		util.Notice(fmt.Sprintf(`%s %s set turtle data: amount:%f n:%f %d:%f-%f %d:%f-%f`,
			market, symbol, turtleData.amount, turtleData.n, turtleData.daysNear, turtleData.lowDaysNear,
			turtleData.highDaysNear, turtleData.daysFar, turtleData.lowDaysFar, turtleData.highDaysFar))
	}
	return
}

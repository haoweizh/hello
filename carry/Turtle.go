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
	checkTime      time.Time
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
	instrument     string
	orderLong      *model.Order
	orderShort     *model.Order
	longs          []*model.Order
	shorts         []*model.Order
}

const turtleTriggerDelta = 0.01

var turtling = false
var turtleLock sync.Mutex

// 当天是否有平仓
var turtleClosed = make(map[string]map[string]bool) // market - symbol - closed

func getTurtling() (value bool) {
	turtleLock.Lock()
	defer turtleLock.Unlock()
	return turtling
}

func setTurtling(value bool) {
	turtleLock.Lock()
	defer turtleLock.Unlock()
	turtling = value
}

var dataSet = make(map[string]map[string]map[string]*TurtleData) // market - symbol - 2019-12-06 - *turtleData

func calcTurtleAmount(key, secret string, setting *model.Setting, price, n float64) (amount float64) {
	switch setting.Market {
	case model.Bitmex:
		p := api.GetBtcBalanceBitmex(key, secret)
		switch setting.Symbol {
		case `btcusd_p`:
			amount = 0.02 * p / n * price * price
		case `ethusd_p`:
			amount = 20000 * p / n
		}
	case model.Ftx, model.OKEX:
		_, _, p, _ := api.GetBalances(key, secret, setting.Market)
		amount = 0.01 * p / n
		switch setting.Symbol {
		case `BTC-PERP`, `ETH-PERP`, `BTC-USDT-SWAP`, `ETH-USDT-SWAP`:
			amount *= 2
		}
	//case model.OKEX:
	//	_, _, p, _ := api.GetBalances(setting.Market)
	//	amount = 0.01 * p / n
	//	symbol := strings.ToUpper(setting.Symbol)
	//	if symbol != `ETH-USDT-SWAP` && symbol != `BTC-USDT-SWAP` {
	//		amount /= 2
	//	}
	case model.HuobiDM:
		coin := model.GetCoin(setting.Market, setting.Symbol)
		balance := api.GetBalance(key, secret, setting.Market, coin)
		if balance != nil {
			p := balance.Amount * price
			if strings.Contains(strings.ToLower(setting.Symbol), `btc`) {
				amount = 0.02 * p * price / n / model.OKEXBTCContractFaceValue
			} else {
				amount = 0.02 * p * price / n / model.OKEXOtherContractFaceValue
			}
			util.Notice(fmt.Sprintf(`%s get %f`, coin, balance.Amount))
		} else {
			util.Notice(fmt.Sprintf(`%s can not get balance`, coin))
		}
	}
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
	turtleData = &TurtleData{turtleTime: today, checkTime: util.GetNow(), waitBreakLong: false, waitBreakShort: false,
		breakLong: false, breakShort: false}
	var orderLong, orderShort *model.Order
	model.AppDB.Where("market= ? and symbol= ? and refresh_type= ? and amount>deal_amount and status=? and order_side=?",
		setting.Market, setting.Symbol, model.FunctionTurtle, model.CarryStatusWorking, model.OrderSideBuy).
		Order(`order_time desc`).Limit(int(setting.AmountLimit)).Find(&turtleData.longs)
	model.AppDB.Where("market= ? and symbol= ? and refresh_type= ? and amount>deal_amount and status=? and order_side=?",
		setting.Market, setting.Symbol, model.FunctionTurtle, model.CarryStatusWorking, model.OrderSideSell).
		Order(`order_time desc`).Limit(int(setting.AmountLimit)).Find(&turtleData.shorts)
	util.Notice(fmt.Sprintf(`load db turtle orders longs %d shorts %d`,
		len(turtleData.longs), len(turtleData.shorts)))
	for _, order := range turtleData.longs {
		if orderLong == nil || (order != nil && order.OrderId != `` && order.OrderTime.After(orderLong.OrderTime)) {
			orderLong = order
		}
	}
	for _, order := range turtleData.shorts {
		if orderShort == nil || (order != nil && order.OrderId != `` && order.OrderTime.After(orderShort.OrderTime)) {
			orderShort = order
		}
	}
	cross := false
	turtleData.instrument = api.GetCurrentInstrument(setting.Market, setting.Symbol)
	if orderLong != nil && orderLong.OrderId != `` {
		if orderLong.Instrument != turtleData.instrument {
			cross = true
		}
		go api.MustCancel(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol,
			orderLong.Instrument, orderLong.OrderType, orderLong.OrderId, true)
	}
	if orderShort != nil && orderShort.OrderId != `` {
		if orderShort.Instrument != turtleData.instrument {
			cross = true
		}
		go api.MustCancel(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol,
			orderShort.Instrument, orderShort.OrderType, orderShort.OrderId, true)
	}
	if cross {
		setting.Chance = 0
		model.AppDB.Model(setting).Where("market= ? and symbol= ? and function= ?",
			setting.Market, setting.Symbol, model.FunctionTurtle).Updates(map[string]interface{}{`chance`: 0})
		go func() {
			err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, "haoweizh@qq.com", `跨期交割`,
				setting.Market+turtleData.instrument)
			if err != nil {
				util.Notice(`fail to send mail %s`, err.Error())
			}
		}()
		channels := model.AppMarkets.GetDepthChan(setting.Market)
		ResetChannels(setting.Market, channels)
		util.Notice(fmt.Sprintf(`%s need to go cross %s to %s set chance 0`,
			setting.Market, setting.Symbol, turtleData.instrument))
	}
	for i := 1; i < 21; i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		day := today.Add(duration)
		candle := api.GetDayCandle(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol,
			turtleData.instrument, day)
		if candle == nil {
			util.Notice(`can not calc turtleDate as nil candle %s %s %s %s`,
				setting.Market, setting.Symbol, turtleData.instrument, day.String())
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
			turtleData.amount = calcTurtleAmount(key, secret, setting, candle.PriceOpen, turtleData.n)
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

// ProcessTurtle
// setting.GridAmount 当前已经持仓数量
// setting.Chance 当前开仓的个数
// setting.PriceX 上一次开仓的价格
// setting.OpenShortMargin 该单币种最多开仓个数
var ProcessTurtle = func(setting *model.Setting, tick *model.BidAsk) {
	now := util.GetNowUnixMillion()
	if tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		model.AppPause || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 1000) ||
		(time.Now().Hour() == 0 && time.Now().Minute() == 0) {
		return
	}
	if setting == nil || getTurtling() {
		return
	}
	if setting.Chance != 0 && setting.PriceX == 0 {
		util.Notice(fmt.Sprintf(`no last priceX %s %s %d %f`,
			setting.Market, setting.Symbol, setting.Chance, setting.PriceX))
		return
	}
	setTurtling(true)
	defer setTurtling(false)
	account := model.AppConfig.GetAccount(setting.Market, 0)
	turtleData := GetTurtleData(account.Key, account.Secret, setting)
	if turtleData == nil || turtleData.n == 0 || turtleData.amount == 0 {
		return
	}
	currentN := model.GetCurrentN(setting)
	showMsg := fmt.Sprintf("%s_%s_%s", model.FunctionTurtle, setting.Market, setting.Symbol)
	model.SetCarryInfo(account.Key, showMsg, fmt.Sprintf("[海龟参数]%s %s 次数限制:%f 当前已经持仓数量:%f 上一次开仓的价格:%f"+
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
		placeTurtleOrders(turtleData, setting, currentN, priceShort, priceLong)
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
			priceShort = setting.PriceX - 2*turtleData.n
		} else if turtleData.lowDays10 < setting.PriceX {
			priceShort = turtleData.lowDays10
		} else if turtleData.lowDays10 > setting.PriceX {
			priceShort = math.Max(turtleData.lowDays10, turtleData.highDays10-2*turtleData.n)
		}
		placeTurtleOrders(turtleData, setting, currentN, priceShort, priceLong)
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
			go func() {
				err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, `haoweizh@qq.com`, `平多`+setting.Market+setting.Symbol,
					fmt.Sprintf(`止盈止损at%f 仓位%d 数量 %f`, priceShort, setting.Chance, setting.GridAmount))
				if err != nil {
					util.Notice(`fail to send mail %s`, err.Error())
				}
			}()
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
			priceLong = setting.PriceX + 2*turtleData.n
		} else if turtleData.highDays10 > setting.PriceX {
			priceLong = turtleData.highDays10
		} else if turtleData.highDays10 < setting.PriceX {
			priceLong = math.Min(turtleData.highDays10, turtleData.lowDays10+2*turtleData.n)
		}
		placeTurtleOrders(turtleData, setting, currentN, priceShort, priceLong)
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
			go func() {
				err := util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, `haoweizh@qq.com`,
					`平空`+setting.Market+setting.Symbol, fmt.Sprintf(`止盈止损at%f 仓位%d 数量 %f`,
						priceLong, setting.Chance, setting.GridAmount))
				if err != nil {
					util.Notice(`fail to send mail %s`, err.Error())
				}
			}()
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

func checkTurtleBreak(key, secret string, setting *model.Setting, turtleData *TurtleData, tick *model.BidAsk) (checked bool) {
	duration, _ := time.ParseDuration(`-300s`)
	now := util.GetNow().Add(duration)
	if now.After(turtleData.checkTime) {
		turtleData.checkTime = util.GetNow()
		if turtleData.orderLong != nil {
			util.Debug(fmt.Sprintf(`-----chance %s %s %d bid-ask %f %f short %f`,
				setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price, turtleData.orderLong.Price))
			order := api.QueryOrderById(key, secret, setting.Market, setting.Symbol,
				turtleData.orderLong.Instrument, turtleData.orderLong.OrderType, turtleData.orderLong.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				turtleData.breakLong = true
				util.Debug(fmt.Sprintf(`-----order break long %s %s %d bid-ask %f %f short %f %v %v`,
					setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					turtleData.orderLong.Price, turtleData.breakLong, turtleData.waitBreakLong))
			}
		}
		if turtleData.orderShort != nil {
			util.Debug(fmt.Sprintf(`-----chance %s %s %d bid-ask %f %f long %f`,
				setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price, turtleData.orderShort.Price))
			order := api.QueryOrderById(key, secret, setting.Market, setting.Symbol,
				turtleData.orderShort.Instrument, turtleData.orderShort.OrderType, turtleData.orderShort.OrderId)
			if order != nil && order.Status == model.CarryStatusSuccess {
				turtleData.breakShort = true
				util.Debug(fmt.Sprintf(`-----order break short %s %s %d bid-ask %f %f long %f %v %v`,
					setting.Market, setting.Symbol, setting.Chance, tick.Bids[0].Price, tick.Asks[0].Price,
					turtleData.orderShort.Price, turtleData.breakShort, turtleData.waitBreakShort))
			}
		}
		return true
	}
	return false
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
			for _, short := range turtleData.shorts {
				temp := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, short.Instrument,
					short.OrderType, short.OrderId)
				if temp != nil && temp.Status == model.CarryStatusWorking {
					go api.MustCancel(model.KeyDefault, model.SecretDefault, short.Market, short.Symbol,
						short.Instrument, short.OrderType, short.OrderId, true)
				}
			}
			util.Notice(fmt.Sprintf(`clear %s %s shorts %d`, setting.Market, setting.Symbol, len(turtleData.shorts)))
			turtleData.shorts = []*model.Order{}
		} else {
			for _, long := range turtleData.longs {
				temp := api.QueryOrderById(key, secret, setting.Market, setting.Symbol, long.Instrument,
					long.OrderType, long.OrderId)
				if temp != nil && temp.Status == model.CarryStatusWorking {
					go api.MustCancel(model.KeyDefault, model.SecretDefault, long.Market, long.Symbol,
						long.Instrument, long.OrderType, long.OrderId, true)
				}
			}
			util.Notice(fmt.Sprintf(`clear %s %s longs %d`, setting.Market, setting.Symbol, len(turtleData.longs)))
			turtleData.longs = []*model.Order{}
		}
	}
}

func placeTurtleOrders(turtleData *TurtleData, setting *model.Setting,
	currentN int64, priceShort, priceLong float64) {
	amountLimit := int64(setting.AmountLimit)
	coinLimit := int64(setting.OpenShortMargin)
	//if setting.Chance > 0 && turtleData.end1/turtleData.highDays20 < 0.87 &&
	//	(currentN >= amountLimit || setting.Chance >= amountLimit) {
	//	priceShort = math.Max(turtleData.lowDays5, setting.PriceX-2*turtleData.n)
	//}
	//if setting.Chance < 0 && turtleData.end1/turtleData.lowDays20 > 1.13 &&
	//	(currentN <= -1*amountLimit || setting.Chance <= -1*amountLimit) {
	//	priceLong = math.Min(turtleData.highDays5, setting.PriceX+2*turtleData.n)
	//}
	if turtleData.orderLong == nil && ((currentN < amountLimit && setting.Chance < coinLimit) || setting.Chance < 0) {
		orderSide := model.OrderSideBuy
		typeLong := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance < 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平空 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, currentN, priceShort,
				priceLong, setting.PriceX, turtleData.n))
			if setting.Market == model.HuobiDM {
				orderSide = model.OrderSideLiquidateShort
			}
		}
		util.Notice(fmt.Sprintf(`%s %s place多单 chance:%d amount:%f price:%f currentN-limit:%d %f
			orderSide:%s end1:%f h20:%f h10:%f h5:%f l20:%f l10:%f l5%f coin limit:%f`,
			setting.Market, setting.Symbol, setting.Chance, amount, setting.PriceX, currentN, setting.AmountLimit,
			orderSide, turtleData.end1, turtleData.highDays20, turtleData.highDays10, turtleData.highDays5,
			turtleData.lowDays20, turtleData.lowDays10, turtleData.lowDays5, setting.OpenShortMargin))
		order := api.MustPlaceOrder(model.KeyDefault, model.SecretDefault, orderSide, typeLong, setting.Market,
			setting.Symbol, turtleData.instrument, ``, model.FunctionTurtle,
			priceLong*(1+turtleTriggerDelta), priceLong, amount, true, false, setting)
		if order != nil && order.OrderId != `` && order.Status != model.CarryStatusFail {
			turtleData.orderLong = order
			turtleData.longs = append(turtleData.longs, order)
			turtleData.waitBreakLong = true
			turtleData.breakLong = false
		}
	} else if turtleData.orderLong != nil && (currentN >= amountLimit || setting.Chance >= coinLimit) {
		go api.MustCancel(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol,
			turtleData.orderLong.Instrument, turtleData.orderLong.OrderType, turtleData.orderLong.OrderId, true)
		turtleData.orderLong = nil
	}
	if turtleData.orderShort == nil && ((currentN > -1*amountLimit && setting.Chance > -1*coinLimit) || setting.Chance > 0) {
		orderSide := model.OrderSideSell
		typeShort := model.OrderTypeStop
		amount := turtleData.amount
		if setting.Chance > 0 {
			amount = setting.GridAmount
			util.Notice(fmt.Sprintf(
				`平多 %s %s chance:%d amount:%f currentN:%d short-long:%f %f px:%f n:%f`,
				setting.Market, setting.Symbol, setting.Chance, amount, currentN, priceShort,
				priceLong, setting.PriceX, turtleData.n))
			if setting.Market == model.HuobiDM {
				orderSide = model.OrderSideLiquidateLong
			}
		}
		//if priceShort > tick.Bids[0].Price {
		//	util.Notice(fmt.Sprintf(`fatal issue: (stop short price)%f > %f(market price)`,
		//		priceShort, tick.Bids[0].Price))
		//	typeShort = model.OrderTypeLimit
		//}
		util.Notice(fmt.Sprintf(`%s %s place空单 chance:%d amount:%f price:%f currentN-limit:%d %f 
			orderSide:%s end1:%f h20:%f h10:%f h5:%f l20:%f l10:%f l5%f coin limit:%f`,
			setting.Market, setting.Symbol, setting.Chance, amount, setting.PriceX, currentN, setting.AmountLimit,
			orderSide, turtleData.end1, turtleData.highDays20, turtleData.highDays10, turtleData.highDays5,
			turtleData.lowDays20, turtleData.lowDays10, turtleData.lowDays5, setting.OpenShortMargin))
		order := api.MustPlaceOrder(model.KeyDefault, model.SecretDefault, orderSide, typeShort, setting.Market,
			setting.Symbol, turtleData.instrument, ``, model.FunctionTurtle,
			priceShort*(1-turtleTriggerDelta), priceShort, amount, true, false, setting)
		if order != nil && order.OrderId != `` && order.Status != model.CarryStatusFail {
			turtleData.orderShort = order
			turtleData.shorts = append(turtleData.shorts, order)
			turtleData.waitBreakShort = true
			turtleData.breakShort = false
		}
	} else if turtleData.orderShort != nil && (currentN <= -1*amountLimit || setting.Chance <= -1*coinLimit) {
		go api.MustCancel(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol,
			turtleData.orderShort.Instrument, turtleData.orderShort.OrderType, turtleData.orderShort.OrderId, true)
		turtleData.orderShort = nil
	}
}

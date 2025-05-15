package api

import (
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

//const TurtleTriggerDelta = 0.003

func GetTurtleTriggerDelta(market string) float64 {
	if market == model.OKEX {
		return 0.002
	}
	return 0.003
}

var positionsCache = &sync.Map{}   // key - symbol - position holding amount
var TurtleDataSet = sync.Map{}     // function_market_symbol_unix second *TurtleData
var accountValues = &sync.Map{}    // market value
var accountValueTime = &sync.Map{} // market time.Time
func CalcTurtleAmount(account *model.Account, n, amountRate float64, candle *model.Candle) (amount float64) {
	var accountValue float64
	valueTime, _ := util.LoadSyncMap(accountValueTime, candle.Market)
	if valueTime != nil && valueTime.(time.Time).Add(time.Hour).After(time.Now()) {
		value, _ := util.LoadSyncMap(accountValues, candle.Market)
		if value != nil {
			accountValue = value.(float64)
		}
	}
	if accountValue == 0 {
		switch candle.Market {
		case model.BinancePerp:
			_, _, accountValue, _, _ = GetPositions(account, candle.Market)
		case model.Bybit, model.OKEX:
			_, _, accountValue, _, _ = GetBalances(account, candle.Market)
		}
		util.StoreSyncMap(accountValues, accountValue, candle.Market)
		util.StoreSyncMap(accountValueTime, time.Now(), candle.Market)
	}
	amount = 0.02 * accountValue * amountRate / n
	util.Log(util.LogLevelInfo, fmt.Sprintf("get amount for %s %f %f", candle.Market, accountValue, amount))
	return amount
}

var clearLock sync.Mutex

// ClearOrders
// 取消market交易所中symbol交易对的所有limit、stop单
func ClearOrders(account *model.Account, market, symbol string, keepTypes map[string]bool) {
	defer clearLock.Unlock()
	clearLock.Lock()
	orders := QueryOpenOrders(account, market, symbol)
	for _, order := range orders {
		if keepTypes != nil && keepTypes[order.OrderType] {
			util.Log(util.LogLevelInfo, fmt.Sprintf(
				`keep order from ClearOrders %s %s %s %s`, market, symbol, order.OrderId, order.OrderType))
			continue
		}
		if order != nil {
			util.Log(util.LogLevelInfo, fmt.Sprintf(
				`cancel pending turtle order %s %s %s %s`, market, symbol, order.OrderId, order.OrderType))
			MustCancel(account, market, symbol, order.OrderType, order.OrderId, false)
		}
	}
}

// ClearExtraOrders
// 取消market交易所中symbol交易对中没有被纳入管理或已经超出仓数限制的订单
func ClearExtraOrders(account *model.Account, market, symbol string, dataArray []*model.TurtleData) {
	keepOrders := make(map[string]*model.Order)
	for _, data := range dataArray {
		for _, order := range data.OrderLong {
			keepOrders[order.OrderId] = order
		}
		for _, order := range data.OrderShort {
			keepOrders[order.OrderId] = order
		}
		for _, order := range data.OrderAdjust {
			keepOrders[order.OrderId] = order
		}
	}
	algoLimitOrders := make(map[string]*model.Order)
	for _, order := range keepOrders {
		if order != nil && order.OrderType != model.OrderTypeLimit && order.Market == model.OKEX {
			orderLimit := QueryOrderById(account, order.Market, order.Symbol, order.OrderType, order.OrderId)
			if orderLimit != nil && orderLimit.OrderId != `` {
				algoLimitOrders[orderLimit.OrderId] = orderLimit
				//util.Notice(fmt.Sprintf(`add okex algo order after break to normal order %s->%s`, order.OrderId, orderLimit.OrderId))
			}
		}
	}
	orders := QueryOpenOrders(account, market, symbol)
	for _, order := range orders {
		if order == nil {
			continue
		}
		if keepOrders[order.OrderId] == nil && algoLimitOrders[order.OrderId] == nil {
			result := MustCancel(account, market, symbol, order.OrderType, order.OrderId, false)
			util.Log(util.LogLevelInfo, fmt.Sprintf(
				`cancel extra order %s %s %s %s return %#v`, market, symbol, order.OrderType, order.OrderId, result))
		}
	}
}

func AdjustPosHolding(account *model.Account, setting *model.Setting, data *model.TurtleData, tick *model.BidAsk) {
	if data.AdjustChecked {
		return
	}
	data.AdjustChecked = true
	success, marketPos, _, _, _ := GetPositions(account, setting.Market)
	if !success {
		util.Log(util.LogLevelInfo, fmt.Sprintf(
			`fail to adjust position holdings %s %s`, setting.Market, setting.Symbol))
		return
	}
	posMap := make(map[string]*model.Position)
	for _, pos := range marketPos {
		posMap[strings.ToUpper(pos.Currency)] = pos
	}
	if posMap[setting.Symbol] != nil { //setting.Chance和pos.Holding相乘小于零代表方向相反，此时设置为0
		if float64(setting.Chance)*posMap[setting.Symbol].Holding <= 0 &&
			math.Abs(posMap[setting.Symbol].Holding)*data.HighNear > 110 {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`...place order to update turtle side %s %s %s holding %e grid amount %e chance %d`,
				setting.Market, setting.Symbol, setting.Function, posMap[setting.Symbol].Holding, setting.GridAmount, setting.Chance))
			setting.GridAmount = 0
			setting.Chance = 0
			setting.PriceX = 0
			var orders []*model.Order
			orderSide := ``
			orderType := model.OrderTypeStop
			var price, priceDeal float64
			turtleTriggerDelta := GetTurtleTriggerDelta(setting.Market)
			if posMap[setting.Symbol].Holding > 0 {
				orderSide = model.OrderSideSell
				price = data.LowAdjust
				priceDeal = data.LowAdjust * (1 - turtleTriggerDelta)
				if data.LowAdjust >= tick.Bids[0].Price {
					orderType = model.OrderTypeLimit
					priceDeal = price * (1 - turtleTriggerDelta)
				}
			} else if posMap[setting.Symbol].Holding < 0 {
				orderSide = model.OrderSideBuy
				price = data.HighAdjust
				priceDeal = data.HighAdjust * (1 + turtleTriggerDelta)
				if data.HighAdjust <= tick.Asks[0].Price {
					orderType = model.OrderTypeLimit
					priceDeal = price * (1 + turtleTriggerDelta)
				}
			}
			if orderSide != `` {
				orders = MustPlaceOrder(account, orderSide, orderType, setting.Market, setting.Symbol, ``,
					model.FunctionTurtleAdjust, priceDeal, price, math.Abs(posMap[setting.Symbol].Holding), true)
			}
			for _, order := range orders {
				if order != nil {
					order.RefreshType = model.FunctionTurtleAdjust
					order.Function = model.Close
					model.AppDB.Save(order)
					if data.OrderAdjust == nil {
						data.OrderAdjust = make(map[string]*model.Order)
					}
					data.OrderAdjust[order.OrderId] = order
				}
			}
		} else if setting.GridAmount != math.Abs(posMap[setting.Symbol].Holding) {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`update turtle grid Amount %s %s %s %e to %e %v`,
				setting.Market, setting.Symbol, setting.Function, setting.GridAmount, posMap[setting.Symbol].Holding, setting))
			setting.GridAmount = math.Abs(posMap[setting.Symbol].Holding)
		}
	} else {
		setting.GridAmount = 0
		setting.Chance = 0
		setting.PriceX = 0
		//util.Notice(`update turtle when absent %s %s %d`, setting.Market, setting.Symbol, len(posMap))
		//for s, position := range posMap {
		//	util.Notice(`present %s %s %e`, s, position.Currency, position.Holding)
		//}
	}
	SetSetting(setting.Function, setting.Market, setting.Symbol, setting)
	model.AppDB.Save(setting)
}

// CheckActiveTrail 海龟主流的追踪平仓单要注意一下，自己的两仓开仓单都满了或者整个主流的6仓单子都满了，只要价格到了就可以下追踪平仓单了
// 和非主流的不一样，非主流是要自己的两仓都满了才会有机会触发追踪单
func CheckActiveTrail(account *model.Account, setting *model.Setting, data *model.TurtleData, bidAsk *model.BidAsk,
	commonTurtleChances int64) (trailed bool) {
	if data.Liquidated {
		return false
	}
	if !model.CommonSymbols[setting.Symbol] && int64(math.Abs(float64(setting.Chance))) < setting.ChanceLimit {
		return false
	}
	if model.CommonSymbols[setting.Symbol] && int64(math.Abs(float64(setting.Chance))) < setting.ChanceLimit &&
		math.Abs(float64(commonTurtleChances)) < setting.AmountLimit {
		return false
	}
	var trails []*model.Order
	if setting.Chance > 0 && data.LowActTrail*data.ActivationRate > 0 && (data.LowActTrail+data.HighActTrail)*0.5*data.ActivationRate < bidAsk.Bids[0].Price {
		trailed = true
		data.OrderShort = nil
		trails = MustPlaceOrder(account, model.OrderSideSell, model.OrderTypeTrailStop, setting.Market, setting.Symbol, model.ReduceOnly,
			setting.Function, bidAsk.Bids[0].Price, data.CallBackRatio, setting.GridAmount, true)
		for _, order := range trails {
			order.Function = model.Close
			data.OrderAdjust[order.OrderId] = order
			util.Log(util.LogLevelInfo, fmt.Sprintf(`success trail sell %s %s amt %f at %f ratio %f ordId %s`,
				setting.Market, setting.Symbol, setting.GridAmount, data.LowActTrail*(1+data.ActivationRate), data.CallBackRatio, order.OrderId))
			go model.AppDB.Save(order)
		}
	} else if setting.Chance < 0 && data.ActivationRate > 0 && bidAsk.Asks[0].Price < (data.LowActTrail+data.HighActTrail)*0.5/data.ActivationRate {
		trailed = true
		data.OrderLong = nil
		trails = MustPlaceOrder(account, model.OrderSideBuy, model.OrderTypeTrailStop, setting.Market, setting.Symbol, model.ReduceOnly,
			setting.Function, bidAsk.Asks[0].Price, data.CallBackRatio, setting.GridAmount, true)
		for _, order := range trails {
			order.Function = model.Close
			data.OrderAdjust[order.OrderId] = order
			util.Log(util.LogLevelInfo, fmt.Sprintf(`success trail buy %s %s amt %f at %f ratio %f ordId %s`,
				setting.Market, setting.Symbol, setting.GridAmount, data.HighActTrail*(1-data.ActivationRate), data.CallBackRatio, order.OrderId))
			go model.AppDB.Save(order)
		}
	}
	if trailed {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`%s %s trailed and clear setting to 0`, setting.Market, setting.Symbol))
		setting.Chance = 0
		setting.GridAmount = 0
		setting.Liquidated = true
		data.Liquidated = true
		SetSetting(setting.Function, setting.Market, setting.Symbol, setting)
		model.AppDB.Save(setting)
	}
	return trailed
}

func HandleOrders(account *model.Account, market, symbol string, settings []*model.Setting, turtleData []*model.TurtleData,
	tick *model.BidAsk) (checked bool) {
	if (len(settings) != 2 && len(settings) != 1) || len(settings) != len(turtleData) {
		util.Log(util.LogLevelError, `wrong combine turtle parameter`)
		return false
	}
	if len(settings) == 1 {
		AdjustPosHolding(account, settings[0], turtleData[0], tick)
	} else if len(settings) == 2 {
		if settings[0].Chance == 0 {
			AdjustPosHolding(account, settings[1], turtleData[1], tick)
		} else if settings[1].Chance == 0 {
			AdjustPosHolding(account, settings[0], turtleData[0], tick)
		}
		turtleData[0].AdjustChecked = true
		turtleData[1].AdjustChecked = true
	}
	if turtleData[0].CheckTimeOpen.Add(time.Minute * 10).After(util.GetNow()) {
		return false
	}
	turtleData[0].CheckTimeOpen = util.GetNow()
	ClearExtraOrders(account, market, symbol, turtleData)
	return true
}

func clearTurtleOrders(account *model.Account, setting *model.Setting, turtle *model.TurtleData) (trailOrders []*model.Order) {
	longAmount := 0.0
	shortAmount := 0.0
	trailOrders = make([]*model.Order, 0)
	for _, order := range turtle.OrderLong {
		if order != nil {
			longAmount += order.Amount
			go MustCancel(account, setting.Market, setting.Symbol, order.OrderType, order.OrderId, false)
		}
	}
	for _, order := range turtle.OrderShort {
		if order != nil {
			shortAmount += order.Amount
			go MustCancel(account, setting.Market, setting.Symbol, order.OrderType, order.OrderId, false)
		}
	}
	for _, order := range turtle.OrderAdjust {
		if order != nil {
			if order.OrderType == model.OrderTypeTrailStop {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`get trail from last turtle data %s %s %s amt %f at %f ordId %s %s`,
					order.Market, order.Symbol, order.OrderSide, order.Amount, order.Price, order.OrderId, order.Status))
				trailOrders = append(trailOrders, order)
			} else {
				go MustCancel(account, setting.Market, setting.Symbol, order.OrderType, order.OrderId, false)
			}
		}
	}
	broken := false
	turtleTriggerDelta := GetTurtleTriggerDelta(setting.Market)
	if turtle.BreakLong && turtle.OrderLong != nil && len(turtle.OrderLong) > 0 {
		broken = true
		if setting.Chance >= 0 {
			setting.Chance++
			setting.GridAmount += longAmount
			setting.PriceX = turtle.OrderLong[0].Price / (1 + turtleTriggerDelta)
			if turtle.OrderLong[0].TriggerPrice > 0 {
				setting.PriceX = turtle.OrderLong[0].TriggerPrice
			}
		} else {
			setting.Chance = 0
			setting.GridAmount = 0
		}
	}
	if turtle.BreakShort && turtle.OrderShort != nil && len(turtle.OrderShort) > 0 {
		broken = true
		if setting.Chance <= 0 {
			setting.Chance--
			setting.GridAmount += shortAmount
			setting.PriceX = turtle.OrderShort[0].Price / (1 - turtleTriggerDelta)
			if turtle.OrderShort[0].TriggerPrice > 0 {
				setting.PriceX = turtle.OrderShort[0].TriggerPrice
			}
		} else {
			setting.Chance = 0
			setting.GridAmount = 0
		}
	}
	if broken {
		model.AppDB.Model(&model.Setting{}).Where("market= ? and Symbol= ? and function= ?",
			setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
			`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
	}
	turtle.OrderLong = nil
	turtle.OrderShort = nil
	return trailOrders
}

var getTurtleLock = sync.Map{} // key - *sync.Mutex{}

func handleLastTurtleData(account *model.Account, function, market, symbol, lastTime string) (lastHandled bool, trailOrders []*model.Order) {
	if function == model.FunctionCombineTurtle {
		settingCombine := GetSetting(model.FunctionCombineTurtle, market, symbol)
		if settingCombine != nil {
			valueCombine, _ := util.LoadSyncMap(&TurtleDataSet, model.FunctionCombineTurtle, market, symbol, lastTime)
			if valueCombine != nil {
				lastHandled = true
				util.Log(util.LogLevelInfo, fmt.Sprintf(`handle last turtle combine %s %s %s %s`, function, market, symbol, lastTime))
				//CheckBreak(account, market, symbol, settings, turtles, nil)
				clearTurtleOrders(account, settingCombine, valueCombine.(*model.TurtleData))
				util.DelSyncMap(&TurtleDataSet, model.FunctionCombineTurtle, market, symbol, lastTime)
			}
		}
	} else if function == model.FunctionTurtleNormal {
		settingNormal := GetSetting(model.FunctionTurtleNormal, market, symbol)
		if settingNormal != nil {
			valueNormal, _ := util.LoadSyncMap(&TurtleDataSet, model.FunctionTurtleNormal, market, symbol, lastTime)
			if valueNormal != nil {
				lastHandled = true
				util.Log(util.LogLevelInfo, fmt.Sprintf(
					`handle last turtle normal %s %s %s %s`, function, market, symbol, lastTime))
				trailOrders = clearTurtleOrders(account, settingNormal, valueNormal.(*model.TurtleData))
				util.DelSyncMap(&TurtleDataSet, model.FunctionTurtleNormal, market, symbol, lastTime)
			}
		}
	} else if function == model.FunctionTurtle {
		valueTurtle, _ := util.LoadSyncMap(&TurtleDataSet, function, market, symbol, lastTime)
		lastHandled = true
		util.Log(util.LogLevelInfo, fmt.Sprintf(
			`handle last turtle %s %s %s %s`, function, market, symbol, lastTime))
		//CheckBreak(account, market, symbol, settings, turtles, nil)
		clearTurtleOrders(account, GetSetting(function, market, symbol), valueTurtle.(*model.TurtleData))
		util.StoreSyncMap(&TurtleDataSet, nil, function, market, symbol, lastTime)
	}
	return lastHandled, trailOrders
}

func GetRankTurtleData(account *model.Account, symbol string, setting *model.Setting) (data *model.TurtleData, dataValid bool) {
	now := time.Now()
	nowPeriod, nowStr := model.GetNowPeriod(setting.Market, setting.Seconds, now)
	data = &model.TurtleData{TurtleTime: nowPeriod, Expire: nowPeriod.Add(time.Second * time.Duration(setting.Seconds)),
		IsBig: true, Symbol: symbol, DaysFar: int(setting.Far), DaysNear: int(setting.Near), DaysAdjust: 5,
		OrderAdjust: make(map[string]*model.Order), CallBackRatio: 0.05, ActivationRate: 1.6}
	util.Log(util.LogLevelInfo, fmt.Sprintf(
		`need to create turtle data rank %s %s %s %s %d`, setting.Function, setting.Market, symbol, nowStr, setting.Far))
	candles := getTurtleCandles(account, setting.Market, symbol, int(setting.Far), int(setting.Seconds), nowPeriod)
	getOne, getAll := CalcTurtleData(account, data, candles, int(setting.Seconds), setting.Market, setting.Function, float64(setting.ChanceLimit), setting.AmountRate)
	if !getOne {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to getOne %s %s %d %d`, setting.Market, symbol, data.DaysFar, setting.Seconds))
		return nil, false
	} else if !getAll {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to getAll %s %s %d %d`, setting.Market, symbol, data.DaysFar, setting.Seconds))
		return nil, true
	}
	if data.Amount > 0 && data.N > 0 {
		var marketInfo *model.MarketInfo
		v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, symbol)
		if v != nil {
			marketInfo = v.(*model.MarketInfo)
		}
		if marketInfo == nil || marketInfo.DeListing {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to get marketInfo %s %s`, setting.Market, symbol))
			return nil, false
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`set data %s %s %s %f %f`, setting.Market, symbol, setting.Function, data.N, data.Amount))
		return data, true
	} else {
		return nil, false
	}
}

func GetTurtleData(account *model.Account, setting *model.Setting, removed bool) (data *model.TurtleData, dataValid bool) {
	var lock *sync.Mutex
	lockValue, _ := util.LoadSyncMap(&getTurtleLock, account.Key, `refreshDynamic`)
	if lockValue == nil {
		lock = &sync.Mutex{}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`create lock %s %s`, account.Key, `refreshDynamic`))
		util.StoreSyncMap(&getTurtleLock, lock, account.Key, `refreshDynamic`)
	} else {
		lock = lockValue.(*sync.Mutex)
	}
	defer lock.Unlock()
	lock.Lock()
	now := time.Now()
	nowPeriod, nowStr := model.GetNowPeriod(setting.Market, setting.Seconds, now)
	value, ok := util.LoadSyncMap(&TurtleDataSet, setting.Function, setting.Market, setting.Symbol, nowStr)
	lastHandled := false
	var trailOrders []*model.Order
	if ok && value != nil {
		return value.(*model.TurtleData), true
	} else {
		lastTime := time.Unix(now.Unix()-setting.Seconds, 0)
		_, lastStr := model.GetNowPeriod(setting.Market, setting.Seconds, lastTime)
		lastHandled, trailOrders = handleLastTurtleData(account, setting.Function, setting.Market, setting.Symbol, lastStr)
	}
	if !model.CommonSymbols[setting.Symbol] {
		refreshValue, refreshOk := DynamicHandleTime.Load(setting.Market)
		if !refreshOk || refreshValue == nil || refreshValue.(time.Time).Before(nowPeriod) {
			if handleMarketDynamic(setting.Market) {
				PrepareSettings()
				success, positions, _, _, _ := GetPositions(account, setting.Market)
				if success {
					posMap := &sync.Map{}
					for _, position := range positions {
						posMap.Store(strings.ToUpper(position.Currency), position)
					}
					util.Log(util.LogLevelInfo, fmt.Sprintf(
						`update positions when refresh %s %s %d`, account.Key, setting.Market, len(positions)))
					positionsCache.Store(account.Key, posMap)
				}
				return nil, true
			}
		}
	}
	posValue, _ := positionsCache.Load(account.Key)
	if posValue != nil {
		holdingValue, _ := posValue.(*sync.Map).Load(setting.Symbol)
		if holdingValue != nil && holdingValue.(*model.Position).Holding != 0 {
			util.Log(util.LogLevelInfo, fmt.Sprintf(
				`not removed %s %s %f`, account.Key, setting.Symbol, holdingValue.(*model.Position).Holding))
			removed = false
		}
	}
	activationRate := 1.6
	if strings.ToLower(setting.Symbol) == `btc_perp` {
		activationRate = 1.2
	} else if strings.ToLower(setting.Symbol) == `eth_perp` || strings.ToLower(setting.Symbol) == `sol_perp` {
		activationRate = 1.3
	}
	// bybit 不开追踪单
	if setting.Market == model.Bybit {
		activationRate = 0
	}
	data = &model.TurtleData{TurtleTime: nowPeriod, Expire: nowPeriod.Add(time.Second * time.Duration(setting.Seconds)),
		IsBig: true, Symbol: setting.Symbol, DaysFar: int(setting.Far), DaysNear: int(setting.Near), DaysAdjust: 5,
		OrderAdjust: make(map[string]*model.Order), OrderCleared: lastHandled, CallBackRatio: 0.05, ActivationRate: activationRate}
	if removed {
		data.CheckTimeOpen = time.Now()
		util.StoreSyncMap(&TurtleDataSet, data, setting.Function, setting.Market, setting.Symbol, nowStr)
		return data, false
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`need to create turtle data %s %s %s %s %d`,
		setting.Function, setting.Market, setting.Symbol, nowStr, setting.Far))
	candles := getTurtleCandles(account, setting.Market, setting.Symbol, int(setting.Far), int(setting.Seconds), nowPeriod)
	getOne, getAll := CalcTurtleData(account, data, candles, int(setting.Seconds), setting.Market, setting.Function, float64(setting.ChanceLimit), setting.AmountRate)
	if !getOne {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to getOne %s %s %d %d`, setting.Market, setting.Symbol, data.DaysFar, setting.Seconds))
		return nil, false
	} else if !getAll {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to getAll %s %s %d %d`, setting.Market, setting.Symbol, data.DaysFar, setting.Seconds))
		return nil, true
	}
	if lastHandled {
		setting.Liquidated = false
		SetSetting(setting.Function, setting.Market, setting.Symbol, setting)
		go model.AppDB.Save(setting)
		data.CheckTimeOpen = time.Now()
		if trailOrders != nil {
			for _, trailOrder := range trailOrders {
				data.OrderAdjust[trailOrder.OrderId] = trailOrder
			}
		}
	} else {
		data.Liquidated = setting.Liquidated
		util.Log(util.LogLevelInfo, fmt.Sprintf(`set turtle data liquidated to setting %s %s %s %#v`,
			setting.Function, setting.Symbol, setting.Market, setting.Liquidated))
	}
	if data.Amount > 0 && data.N > 0 {
		var marketInfo *model.MarketInfo
		v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
		if v != nil {
			marketInfo = v.(*model.MarketInfo)
		}
		if marketInfo == nil || marketInfo.DeListing {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to get marketInfo %s %s`, setting.Market, setting.Symbol))
			return nil, false
		}
		util.StoreSyncMap(&TurtleDataSet, data, setting.Function, setting.Market, setting.Symbol, nowStr)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`set turtle value %s %s %s %s Amount:%e N:%e %d:%e-%e %d:%e-%e %e %e %#v`,
			setting.Function, setting.Market, setting.Symbol, nowStr, data.Amount, data.N, data.DaysNear, data.LowNear,
			data.HighNear, data.DaysFar, data.LowFar, data.HighFar, data.N, data.Amount, data))
		return data, true
	} else {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`set data no amount %s %s %s %f %f`, setting.Market, setting.Symbol, setting.Function, data.N, data.Amount))
		return nil, false
	}
}

func CalcTurtleData(account *model.Account, data *model.TurtleData, candles []*model.Candle, seconds int, market, function string, chanceLimit, amountRate float64) (
	getOne, getAll bool) {
	priceClose := 0.0
	trailDays := 1
	if strings.ToLower(data.Symbol) == `btc_perp` {
		trailDays = 2
	} else if strings.ToLower(data.Symbol) == `eth_perp` || strings.ToLower(data.Symbol) == `sol_perp` {
		trailDays = 2
	}
	for i := 1; i <= data.DaysFar; i++ {
		currentPeriod := data.TurtleTime.Add(time.Second * time.Duration(seconds*-i))
		candle := findCandle(candles, currentPeriod)
		if candle == nil || candle.PriceHigh == 0 || candle.PriceLow == 0 {
			util.LogLess(util.LogLevelInfo, fmt.Sprintf(`can not calc turtleDate as nil candle %s %s %s %d`,
				market, data.Symbol, currentPeriod.String(), len(candles)))
			return
		}
		getOne = true
		if candle.PriceHigh > data.HighActTrail && i <= trailDays {
			data.HighActTrail = candle.PriceHigh
		}
		if (data.LowActTrail == 0 || candle.PriceLow < data.HighActTrail) && i <= trailDays {
			data.LowActTrail = candle.PriceLow
		}
		if candle.PriceHigh > data.HighFar && i <= data.DaysFar {
			data.HighFar = candle.PriceHigh
		}
		if (data.LowFar == 0 || data.LowFar > candle.PriceLow) && i <= data.DaysFar {
			data.LowFar = candle.PriceLow
		}
		if candle.PriceHigh > data.HighNear && i <= data.DaysNear {
			data.HighNear = candle.PriceHigh
		}
		if (data.LowNear == 0 || data.LowNear > candle.PriceLow) && i <= data.DaysNear {
			data.LowNear = candle.PriceLow
		}
		if candle.PriceHigh > data.HighAdjust && i <= data.DaysAdjust {
			data.HighAdjust = candle.PriceHigh
		}
		if (data.LowAdjust == 0 || data.LowAdjust > candle.PriceLow) && i <= data.DaysAdjust {
			data.LowAdjust = candle.PriceLow
		}
		if i == 1 {
			go model.AppDB.Save(candle)
			data.N = candle.N
			data.NVolume = candle.NVolume
			data.M = candle.M
			priceClose = candle.PriceClose
			data.Amount = CalcTurtleAmount(account, data.N, amountRate, candle)
			util.Log(util.LogLevelInfo, fmt.Sprintf(
				`calc amt %s %s %s %f %f %f`, market, data.Symbol, function, data.N, amountRate, data.Amount))
		}
	}
	if function == model.FunctionTurtle || function == model.FunctionTurtleNormal {
		data.UseNear = true
	} else if function == model.FunctionCombineTurtle {
		data.UseNear = false
	}
	if model.CommonSymbols[data.Symbol] {
		if function == model.FunctionCombineTurtle {
			data.Amount = math.Min(data.Amount, 1920000/priceClose/chanceLimit)
		} else {
			data.Amount = math.Min(data.Amount, 2400000/priceClose/chanceLimit)
		}
	} else {
		if function == model.FunctionCombineTurtle {
			data.Amount = math.Min(data.Amount, 120000/priceClose/chanceLimit)
		} else {
			data.Amount = math.Min(data.Amount, 150000/priceClose/chanceLimit)
		}
	}
	getAll = true
	return
}

func getTurtleCandles(account *model.Account, market, symbol string, far, seconds int, currentPeriod time.Time) (candles []*model.Candle) {
	lastPeriod := time.Unix(currentPeriod.Unix()-int64(seconds)*2, 0)
	var lastCandles []*model.Candle
	model.AppDB.Model(&model.Candle{}).Where(`market=? and symbol=? and begin=? and seconds=?`,
		market, symbol, lastPeriod, seconds).Find(&lastCandles)
	slots := far
	var lastCandle2 *model.Candle
	if len(lastCandles) == 0 || lastCandles[0].N == 0 || lastCandles[0].NVolume == 0 {
		if seconds < 86400 {
			slots = 600
		} else {
			slots = 200
		}
	} else {
		lastCandle2 = lastCandles[0]
	}
	candles = CombineCandles(account, market, symbol, seconds,
		currentPeriod.Add(time.Second*time.Duration(seconds*-1*slots)), currentPeriod)
	if candles == nil || len(candles) == 0 { //|| len(candles) < far {
		return nil
	}
	calcLenN := 10
	calcLenV := 2
	if seconds == 14400 {
		calcLenN = 15
		calcLenV = 10
	}
	sortedCandles := &model.SortedCandle{Value: candles}
	sort.Sort(sortedCandles)
	if lastCandle2 == nil {
		CalcCandleN(sortedCandles, calcLenN, calcLenV)
	} else {
		lastCandle := sortedCandles.Value[len(sortedCandles.Value)-1]
		lastCandle.N = (lastCandle2.N*float64(calcLenN-1) + lastCandle.PriceHigh - lastCandle.PriceLow) / float64(calcLenN)
		lastCandle.NVolume = (lastCandle2.NVolume*float64(calcLenV-1) + lastCandle.Volume) / float64(calcLenV)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`base on last 2 candle %s %s far %d %d n-n %f %f nv-nv %f %f len %d volume len %d`,
			market, symbol, far, seconds, lastCandle2.N, lastCandle.N, lastCandle2.NVolume, lastCandle.NVolume, calcLenN, calcLenV))
	}
	return sortedCandles.Value
}

const CandleMLen = 10

func CalcCandleN(sortedCandles *model.SortedCandle, calcLenN, calcLenV int) {
	beginPrice := 0.0
	beginVolume := 0.0
	calcLenN = int(math.Min(float64(calcLenN), float64(len(sortedCandles.Value))))
	calcLenV = int(math.Min(float64(calcLenV), float64(len(sortedCandles.Value))))
	for i := 0; i < calcLenN; i++ {
		beginPrice += sortedCandles.Value[i].PriceHigh - sortedCandles.Value[i].PriceLow
	}
	for i := 0; i < calcLenV; i++ {
		beginVolume += sortedCandles.Value[i].Volume
	}
	sortedCandles.Value[calcLenN-1].N = beginPrice / float64(calcLenN)
	sortedCandles.Value[calcLenV-1].NVolume = beginVolume / float64(calcLenV)
	for i := calcLenN; i < len(sortedCandles.Value); i++ {
		sortedCandles.Value[i].N = (sortedCandles.Value[i-1].N*float64(calcLenN-1) + sortedCandles.Value[i].PriceHigh - sortedCandles.Value[i].PriceLow) / float64(calcLenN)

	}
	for i := calcLenV; i < len(sortedCandles.Value); i++ {
		sortedCandles.Value[i].NVolume = (sortedCandles.Value[i-1].NVolume*float64(calcLenV-1) + sortedCandles.Value[i].Volume) / float64(calcLenV)
	}
	disAll := 0.0
	for i := 0; i < len(sortedCandles.Value); i++ {
		disAll += sortedCandles.Value[i].PriceHigh - sortedCandles.Value[i].PriceLow
		if i >= CandleMLen {
			disAll = disAll - sortedCandles.Value[i-CandleMLen].PriceHigh + sortedCandles.Value[i-CandleMLen].PriceLow
			sortedCandles.Value[i].M = disAll / CandleMLen
		}
	}
}

func findCandle(candles []*model.Candle, begin time.Time) (resultCandle *model.Candle) {
	for _, candle := range candles {
		if candle.Begin == begin {
			return candle
		}
	}
	for _, candle := range candles {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`no found candle %s %s %s %s`,
			candle.Market, candle.Symbol, begin.String(), candle.Begin.String()))
	}
	return nil
}

func SetTurtleOrderStatus(function, market, symbol, orderId, status string) {
	setting := GetSetting(function, market, symbol)
	if setting == nil {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	if account == nil || setting.Seconds <= 0 {
		return
	}
	var nowStr string
	_, nowStr = model.GetNowPeriod(setting.Market, setting.Seconds, time.Now())
	value, ok := util.LoadSyncMap(&TurtleDataSet, function, market, symbol, nowStr)
	if ok && value != nil {
		turtleData := value.(*model.TurtleData)
		if turtleData.OrderLong != nil {
			for _, order := range turtleData.OrderLong {
				if order.OrderId == orderId {
					order.Status = status
				}
			}
		}
		if turtleData.OrderShort != nil {
			for _, order := range turtleData.OrderShort {
				if order.OrderId == orderId {
					order.Status = status
				}
			}
		}
	}
}

func CheckBreak(account *model.Account, market, symbol string, settings []*model.Setting, turtleData []*model.TurtleData,
	tick *model.BidAsk) (useApi bool) {
	if (len(settings) != 2 && len(settings) != 1) || len(settings) != len(turtleData) {
		util.Log(util.LogLevelError, `wrong combine turtle parameter`)
		return false
	}
	if turtleData[0].CheckUseApi.Add(time.Minute * 10).Before(util.GetNow()) {
		useApi = true
	}
	for i, setting := range settings {
		data := turtleData[i]
		if useApi {
			data.CheckUseApi = util.GetNow()
		}
		var orderLong, orderShort *model.Order
		if data.OrderLong != nil && len(data.OrderLong) > 0 {
			orderLong = data.OrderLong[0]
		}
		if data.OrderShort != nil && len(data.OrderShort) > 0 {
			orderShort = data.OrderShort[0]
		}
		if orderLong != nil {
			if tick != nil && orderLong.TriggerPrice > 0 &&
				((orderLong.OrderType == model.OrderTypeStop && orderLong.TriggerPrice <= tick.Bids[0].Price) ||
					(orderLong.OrderType == model.OrderTypeLimit && orderLong.Price >= tick.Asks[0].Price)) {
				orderLong.Status = model.CarryStatusSuccess
			}
			if orderLong.Status == model.CarryStatusWorking && (useApi || tick == nil) {
				orderLong = QueryOrderById(account, market, symbol, orderLong.OrderType, orderLong.OrderId)
			}
			if orderLong != nil && (orderLong.Status == model.CarryStatusSuccess || orderLong.DealAmount*3 > orderLong.Amount*2) {
				data.BreakLong = true
				for _, order := range data.OrderLong {
					data.OrderAdjust[order.OrderId] = order
					if order.Market == model.OKEX && order.OrderType != model.OrderTypeLimit {
						limitOrder := QueryOrderById(account, order.Market, order.Symbol, order.OrderType, order.OrderId)
						if limitOrder != nil {
							data.OrderAdjust[limitOrder.OrderId] = limitOrder
							util.Log(util.LogLevelInfo, fmt.Sprintf(`add okex created limit order into turtle adjust %s %s->%s`,
								order.Symbol, order.OrderId, limitOrder.OrderId))
						}
					}
				}
				util.Log(util.LogLevelInfo, fmt.Sprintf(`order break long %s %s %s %d %e %e id %s usdApi %#v`,
					market, symbol, orderLong.OrderType, setting.Chance, orderLong.TriggerPrice, orderLong.Price, orderLong.OrderId, useApi))
			}
		}
		if orderShort != nil {
			if tick != nil && orderShort.TriggerPrice > 0 &&
				((orderShort.OrderType == model.OrderTypeStop && orderShort.TriggerPrice >= tick.Asks[0].Price) ||
					(orderShort.OrderType == model.OrderTypeLimit && orderShort.Price <= tick.Bids[0].Price)) {
				orderShort.Status = model.CarryStatusSuccess
			}
			if orderShort.Status == model.CarryStatusWorking && (useApi || tick == nil) {
				orderShort = QueryOrderById(account, market, symbol, orderShort.OrderType, orderShort.OrderId)
			}
			if orderShort != nil && (orderShort.Status == model.CarryStatusSuccess || orderShort.DealAmount*3 > orderShort.Amount*2) {
				data.BreakShort = true
				for _, order := range data.OrderShort {
					data.OrderAdjust[order.OrderId] = order
					if order.Market == model.OKEX && order.OrderType != model.OrderTypeLimit {
						limitOrder := QueryOrderById(account, order.Market, order.Symbol, order.OrderType, order.OrderId)
						if limitOrder != nil {
							data.OrderAdjust[limitOrder.OrderId] = limitOrder
							util.Log(util.LogLevelInfo, fmt.Sprintf(
								`add okex created limit order into turtle adjust %s %s->%s`, order.Symbol, order.OrderId, limitOrder.OrderId))
						}
					}
				}
				util.Log(util.LogLevelInfo, fmt.Sprintf(
					`order break short %s %s %s %d %e %e id %s useApi %#v`,
					market, symbol, orderShort.OrderType, setting.Chance, orderShort.TriggerPrice, orderShort.Price, orderShort.OrderId, useApi))
			}
		}
	}
	return useApi
}

// CanOpenCombine
// change算法 币种数=海龟仓数绝对值+单汤币数
func CanOpenCombine(settingCombine, settingNormal *model.Setting, dataTurtle *model.TurtleData) (
	canOpen, canStartCombine, canStartTurtle bool, turtleSymbolNum, inAll float64, commonTurtleChances int64) {
	settingsCombine := GetSettings(settingCombine.Function, settingCombine.Market)
	settingsNormal := GetSettings(settingNormal.Function, settingCombine.Market)
	if settingsCombine == nil || settingsNormal == nil {
		return false, false, false, 0, 0, 0
	}
	tradingCombines := make(map[string]bool)
	// 计算龟汤持仓币种的数目，持多仓的和空仓的都+1
	sumCombineOnly := func(symbol, value any) bool {
		if value != nil {
			valueSetting := value.(*model.Setting)
			if !model.CommonSymbols[valueSetting.Symbol] && valueSetting.Valid {
				if valueSetting.Chance != 0 && valueSetting.Function == model.FunctionCombineTurtle {
					normal := GetSetting(model.FunctionTurtleNormal, valueSetting.Market, valueSetting.Symbol)
					if normal != nil && normal.Chance == 0 {
						tradingCombines[valueSetting.Symbol] = true
					}
				}
			}
		}
		return true
	}
	// 计算非主流海龟持仓币种的数目，持多仓的+1，空仓的-1
	sumTurtle := func(symbol, value any) bool {
		if value != nil {
			valueSetting := value.(*model.Setting)
			if !model.CommonSymbols[valueSetting.Symbol] && valueSetting.Valid {
				if valueSetting.Function == model.FunctionTurtleNormal {
					if valueSetting.Chance > 0 {
						turtleSymbolNum++
					} else if valueSetting.Chance < 0 {
						turtleSymbolNum--
					}
				}
			}
		}
		return true
	}
	//btcInTurtle := false
	//checkCommonTurtle := func(symbol, value any) bool {
	//	if value != nil {
	//		valueSetting := value.(*model.Setting)
	//		_, _, valueCoin, _ := model.GetFromStandard(valueSetting.Market, valueSetting.Symbol)
	//		if strings.ToLower(valueCoin) == `btc` {
	//			if valueSetting.Function == model.FunctionTurtleNormal && valueSetting.Chance != 0 {
	//				btcInTurtle = true
	//			}
	//		}
	//	}
	//	return true
	//}
	if model.CommonSymbols[settingCombine.Symbol] {
		canOpen = true
		canStartCombine = true
		canStartTurtle = false
		settingsNormal.Range(func(symbol, value any) bool {
			if value != nil {
				valueSetting := value.(*model.Setting)
				if model.CommonSymbols[valueSetting.Symbol] && valueSetting.Valid {
					if valueSetting.Function == model.FunctionTurtleNormal {
						commonTurtleChances += valueSetting.Chance
					}
				}
			}
			return true
		})
		if math.Abs(float64(commonTurtleChances)) < settingNormal.AmountLimit {
			canStartTurtle = true
		}
	} else {
		settingsNormal.Range(sumTurtle)
		settingsCombine.Range(sumCombineOnly)
		if settingNormal.MarketRelated == model.TurtleTypeChange && settingCombine.MarketRelated == model.TurtleTypeChange {
			btcSetting := GetSetting(model.FunctionTurtleNormal, settingNormal.Market, `BTC_PERP`)
			if btcSetting == nil || btcSetting.Chance == 0 {
				inAll = float64(len(tradingCombines)) + math.Abs(turtleSymbolNum)
				canStartTurtle = false
				canStartCombine = true
			} else if int(math.Abs(float64(btcSetting.Chance))) == 1 {
				inAll = turtleSymbolNum
				canStartTurtle = true
				canStartCombine = true
			} else if int(math.Abs(float64(btcSetting.Chance))) == 2 {
				inAll = turtleSymbolNum
				canStartTurtle = true
				canStartCombine = false
			}
		} else {
			inAll = turtleSymbolNum
			canStartTurtle = true
			canStartCombine = true
		}
		canOpen = settingCombine.Chance != 0 || settingNormal.Chance != 0 || (math.Abs(inAll) < settingCombine.AmountLimit &&
			settingCombine.SymbolRelated != model.SettingTurtleRemoved && settingNormal.SymbolRelated != model.SettingTurtleRemoved)
		//now := time.Now()
		//if canOpen == false && now.Minute()%10 == 0 && now.Second() == 0 {
		//	util.Notice(fmt.Sprintf(`can not open %s %s canCombine %#v canTurtle %#v turtle symbols %f inAll %f`,
		//		settingNormal.Market, settingNormal.Symbol, canStartCombine, canStartTurtle, turtleSymbolNum, inAll))
		//}
	}
	if dataTurtle.Liquidated {
		canStartTurtle = false
	}
	if dataTurtle.HighFar < settingCombine.CloseShortMargin*dataTurtle.LowFar {
		canStartCombine = false
	}
	if settingCombine.HighLowRate > 0 && dataTurtle.HighFar/dataTurtle.LowFar > settingCombine.HighLowRate {
		canStartCombine = false
	}
	if settingNormal.HighLowRate > 0 && dataTurtle.HighFar/dataTurtle.LowFar > settingNormal.HighLowRate {
		canStartTurtle = false
	}
	return canOpen, canStartCombine, canStartTurtle, turtleSymbolNum, inAll, commonTurtleChances
}

// CanOpenTurtle CanOpenTurtle  主流币检查仓位总数;非主流检查交易币种个数
// isChance true: 按照仓数计算
// isChance false: 按照开仓币种数计算
// 当setting.OpenShortMargin小于0时不限制本币种仓位，等于0时不许开仓（未实现）
// 当setting.AmountLimit小于0时不限制总仓位或开仓币数，等于0时不许开仓（未实现）
func CanOpenTurtle(setting *model.Setting, data *model.TurtleData) (canOpen bool, inAll float64) {
	settings := GetSettings(setting.Function, setting.Market)
	if settings == nil {
		return false, 0
	}
	if model.CommonSymbols[setting.Symbol] {
		settings.Range(func(symbol, value any) bool {
			if value != nil {
				valueSetting := value.(*model.Setting)
				if valueSetting.Market == setting.Market && valueSetting.Function == setting.Function &&
					model.CommonSymbols[valueSetting.Symbol] {
					inAll += float64(valueSetting.Chance)
				}
			}
			return true
		})
		canOpen = math.Abs(inAll) < setting.AmountLimit
		if !canOpen && inAll > 0 && setting.Chance >= 0 {
			data.OrderLong = nil
		}
		if !canOpen && inAll < 0 && setting.Chance <= 0 {
			data.OrderShort = nil
		}
	} else { // 非主流币检查开仓了的币种个数
		settings.Range(func(symbol, value any) bool {
			if value != nil {
				valueSetting := value.(*model.Setting)
				if valueSetting.Market == setting.Market && valueSetting.Function == setting.Function &&
					!model.CommonSymbols[valueSetting.Symbol] {
					if valueSetting.Chance != 0 {
						inAll++
					}
				}
			}
			return true
		})
		canOpen = setting.Chance != 0 || (setting.SymbolRelated != model.SettingTurtleRemoved && inAll < setting.AmountLimit)
		if setting.Chance == 0 && !canOpen && inAll >= setting.AmountLimit {
			data.OrderLong = nil
			data.OrderShort = nil
		}
	}
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < float64(setting.ChanceLimit)
	return canOpen, inAll
}

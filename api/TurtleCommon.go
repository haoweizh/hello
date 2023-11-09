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

const TurtleTriggerDelta = 0.005

var positionsCache = &sync.Map{}   // key - symbol - position holding amount
var TurtleDataSet = sync.Map{}     // function_market_symbol_unix second *TurtleData
var CombineFulled = &sync.Map{}    // market_unix seconds bool
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
			_, _, accountValue, _ = GetPositions(account.Key, account.Secret, candle.Market)
		case model.Ftx, model.OKEX:
			_, _, accountValue, _ = GetBalances(account.Key, account.Secret, candle.Market)
		}
		util.StoreSyncMap(accountValues, accountValue, candle.Market)
		util.StoreSyncMap(accountValueTime, time.Now(), candle.Market)
	}
	amount = 0.02 * accountValue / n
	amount *= amountRate
	return amount
}

var clearLock sync.Mutex

// ClearOrders
// 取消market交易所中symbol交易对的所有limit、stop单
func ClearOrders(key, secret, market, symbol string, keepTypes map[string]bool) {
	defer clearLock.Unlock()
	clearLock.Lock()
	orders := QueryOpenOrders(key, secret, market, symbol)
	for _, order := range orders {
		if keepTypes != nil && keepTypes[order.OrderType] {
			continue
		}
		if order != nil {
			util.Notice(`cancel pending turtle order %s %s %s`, market, symbol, order.OrderId)
			MustCancel(key, secret, market, symbol, order.OrderType, order.OrderId, true)
		}
	}
	time.Sleep(time.Second)
}

// ClearExtraOrders
// 取消market交易所中symbol交易对中没有被纳入管理或已经超出仓数限制的订单
func ClearExtraOrders(key, secret, market, symbol string, dataArray []*model.TurtleData) {
	keepOrders := make(map[string]bool)
	for _, data := range dataArray {
		for _, order := range data.OrderLong {
			keepOrders[order.OrderId] = true
		}
		for _, order := range data.OrderShort {
			keepOrders[order.OrderId] = true
		}
		for _, order := range data.OrderAdjust {
			keepOrders[order.OrderId] = true
		}
		for _, order := range data.OrderTrail {
			keepOrders[order.OrderId] = true
		}
	}
	orders := QueryOpenOrders(key, secret, market, symbol)
	for _, order := range orders {
		if order == nil {
			continue
		}
		if !keepOrders[order.OrderId] {
			result := MustCancel(key, secret, market, symbol, order.OrderType, order.OrderId, true)
			util.Notice(`cancel extra order %s %s %s %s return %v`, market, symbol, order.OrderType, order.OrderId, result)
			time.Sleep(time.Second)
			//} else {
			//	util.Notice(`keep stop order %s %s %s`, market, symbol, order.OrderId)
		}
	}
}

func AdjustPosHolding(key, secret string, setting *model.Setting, data *model.TurtleData) {
	if data.AdjustChecked {
		return
	}
	data.AdjustChecked = true
	success, marketPos, _, _ := GetPositions(key, secret, setting.Market)
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
			util.Notice(`...place order to update turtle side %s %s %s holding %e grid amount %e chance %d`,
				setting.Market, setting.Symbol, setting.Function, posMap[setting.Symbol].Holding, setting.GridAmount, setting.Chance)
			setting.GridAmount = 0
			setting.Chance = 0
			setting.PriceX = 0
			var orders []*model.Order
			if posMap[setting.Symbol].Holding > 0 {
				orders = MustPlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtleAdjust, data.LowAdjust*(1-TurtleTriggerDelta), data.LowAdjust, posMap[setting.Symbol].Holding, setting)
			} else if posMap[setting.Symbol].Holding < 0 {
				orders = MustPlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtleAdjust, data.HighAdjust*(1+TurtleTriggerDelta), data.HighAdjust, -1*posMap[setting.Symbol].Holding, setting)
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
			util.Notice(`update turtle grid Amount %s %s %e to %e`,
				setting.Market, setting.Symbol, setting.GridAmount, posMap[setting.Symbol].Holding)
			setting.GridAmount = math.Abs(posMap[setting.Symbol].Holding)
		}
	} else {
		setting.GridAmount = 0
		setting.Chance = 0
		setting.PriceX = 0
		//util.Notice(`update turtle when absent %s %s %d`, setting.Market, setting.Symbol, len(posMap))
		for s, position := range posMap {
			util.Notice(`present %s %s %e`, s, position.Currency, position.Holding)
		}
	}
	model.AppDB.Save(setting)
}

func HandleOrders(key, secret, market, symbol string, settings []*model.Setting, turtleData []*model.TurtleData) (checked bool) {
	if (len(settings) != 2 && len(settings) != 1) || len(settings) != len(turtleData) {
		util.Notice(`wrong combine turtle parameter`)
		return false
	}
	if len(settings) == 1 {
		AdjustPosHolding(key, secret, settings[0], turtleData[0])
	} else if len(settings) == 2 {
		if settings[0].Chance == 0 {
			AdjustPosHolding(key, secret, settings[1], turtleData[1])
		} else if settings[1].Chance == 0 {
			AdjustPosHolding(key, secret, settings[0], turtleData[0])
		}
		turtleData[0].AdjustChecked = true
		turtleData[1].AdjustChecked = true
	}
	if turtleData[0].CheckTimeOpen.Add(time.Minute * 10).After(util.GetNow()) {
		return false
	}
	turtleData[0].CheckTimeOpen = util.GetNow()
	ClearExtraOrders(key, secret, market, symbol, turtleData)
	return true
}

func clearTurtleOrders(account *model.Account, setting *model.Setting, turtle *model.TurtleData) {
	longAmount := 0.0
	shortAmount := 0.0
	for _, order := range turtle.OrderLong {
		if order != nil {
			longAmount += order.Amount
			MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, false)
			time.Sleep(time.Millisecond * 200)
		}
	}
	for _, order := range turtle.OrderShort {
		if order != nil {
			shortAmount += order.Amount
			MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, false)
			time.Sleep(time.Millisecond * 200)
		}
	}
	for _, order := range turtle.OrderAdjust {
		if order != nil {
			MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, false)
			time.Sleep(time.Millisecond * 200)
		}
	}
	broken := false
	if turtle.BreakLong && turtle.OrderLong != nil && len(turtle.OrderLong) > 0 {
		broken = true
		if setting.Chance >= 0 {
			setting.Chance++
			setting.GridAmount += longAmount
			setting.PriceX = turtle.OrderLong[0].TriggerPrice
		} else {
			setting.Chance = 0
		}
	}
	if turtle.BreakShort && turtle.OrderShort != nil && len(turtle.OrderShort) > 0 {
		broken = true
		if setting.Chance <= 0 {
			setting.Chance--
			setting.GridAmount += shortAmount
			setting.PriceX = turtle.OrderShort[0].TriggerPrice
		} else {
			setting.Chance = 0
		}
	}
	if setting.Chance == 0 {
		for _, order := range turtle.OrderTrail {
			MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, false)
			time.Sleep(time.Millisecond * 200)
		}
	}
	if turtle.BreakTrail {
		broken = true
		setting.Chance = 0
	}
	if broken {
		model.AppDB.Model(&model.Setting{}).Where("market= ? and Symbol= ? and function= ?",
			setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{
			`price_x`: setting.PriceX, `chance`: setting.Chance, `grid_amount`: setting.GridAmount})
	}
	turtle.OrderLong = nil
	turtle.OrderShort = nil
	turtle.OrderTrail = nil
}

var getTurtleLock = sync.Map{} // key - *sync.Mutex{}

func handleLastTurtleData(account *model.Account, function, market, symbol, lastTime string) (lastHandled bool) {
	var settings []*model.Setting
	var turtles []*model.TurtleData
	if function == model.FunctionCombineTurtle || function == model.FunctionTurtleNormal {
		settingCombine := GetSetting(model.FunctionCombineTurtle, market, symbol)
		settingNormal := GetSetting(model.FunctionTurtleNormal, market, symbol)
		if settingCombine != nil && settingNormal != nil {
			valueCombine, _ := util.LoadSyncMap(&TurtleDataSet, model.FunctionCombineTurtle, market, symbol, lastTime)
			valueNormal, _ := util.LoadSyncMap(&TurtleDataSet, model.FunctionTurtleNormal, market, symbol, lastTime)
			if valueCombine != nil && valueNormal != nil {
				util.Notice(fmt.Sprintf(`handle last turtle %s %s %s %s`, function, market, symbol, lastTime))
				lastHandled = true
				settings = []*model.Setting{settingCombine, settingNormal}
				turtles = []*model.TurtleData{valueCombine.(*model.TurtleData), valueNormal.(*model.TurtleData)}
				CheckBreak(account, market, symbol, settings, turtles, nil)
				clearTurtleOrders(account, settings[0], turtles[0])
				clearTurtleOrders(account, settings[1], turtles[1])
				util.DelSyncMap(&TurtleDataSet, model.FunctionCombineTurtle, market, symbol, lastTime)
				util.DelSyncMap(&TurtleDataSet, model.FunctionTurtleNormal, market, symbol, lastTime)
			}
		}
	} else if function == model.FunctionTurtle || function == model.FunctionBoost {
		settings = []*model.Setting{GetSetting(function, market, symbol)}
		valueTurtle, _ := util.LoadSyncMap(&TurtleDataSet, function, market, symbol, lastTime)
		if valueTurtle == nil || settings[0] == nil {
			return
		}
		util.Notice(fmt.Sprintf(`handle last turtle %s %s %s %s`, function, market, symbol, lastTime))
		lastHandled = true
		turtles = []*model.TurtleData{valueTurtle.(*model.TurtleData)}
		CheckBreak(account, market, symbol, settings, turtles, nil)
		clearTurtleOrders(account, settings[0], turtles[0])
		util.StoreSyncMap(&TurtleDataSet, nil, function, market, symbol, lastTime)
	}
	return lastHandled
}

// GetTurtleData refreshDynamic false时代表仅作为检查是否有足够turtleData作为top market info使用，此时不会存在缓存中，否则会引起far near错误
func GetTurtleData(account *model.Account, function, market, symbol string, far, near, seconds int64,
	amountRate float64, refreshDynamic, removed bool) (data *model.TurtleData, dataValid bool) {
	if refreshDynamic {
		var lock *sync.Mutex
		lockValue, _ := getTurtleLock.Load(account.Key)
		if lockValue == nil {
			lock = &sync.Mutex{}
			getTurtleLock.Store(account.Key, lock)
		} else {
			lock = lockValue.(*sync.Mutex)
		}
		defer lock.Unlock()
		lock.Lock()
	}
	now := time.Now()
	nowPeriod, nowStr := model.GetNowPeriod(market, seconds, now)
	value, ok := util.LoadSyncMap(&TurtleDataSet, function, market, symbol, nowStr)
	lastHandled := false
	if ok && value != nil {
		return value.(*model.TurtleData), true
	} else {
		lastTime := time.Unix(now.Unix()-seconds, 0)
		_, lastStr := model.GetNowPeriod(market, seconds, lastTime)
		lastHandled = handleLastTurtleData(account, function, market, symbol, lastStr)
	}
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	//today, _ := model.GetMarketToday(market)
	// today.Unix() == nowPeriod.Unix() &&
	if refreshDynamic && !model.CommonCoins[strings.ToLower(coin)] {
		refreshValue, refreshOk := DynamicHandleTime.Load(market)
		if !refreshOk || refreshValue == nil || refreshValue.(time.Time).Before(nowPeriod) {
			if handleMarketDynamic(market) {
				PrepareSettings()
				SetRequireReset(market)
				success, positions, _, _ := GetPositions(account.Key, account.Secret, market)
				if success {
					posMap := &sync.Map{}
					for _, position := range positions {
						posMap.Store(strings.ToUpper(position.Currency), position)
					}
					util.Notice(fmt.Sprintf(`update positions when refresh %s %s %d`, account.Key, market, len(positions)))
					positionsCache.Store(account.Key, posMap)
				}
				return nil, true
			}
		}
	}
	posValue, _ := positionsCache.Load(account.Key)
	if posValue != nil {
		holdingValue, _ := posValue.(*sync.Map).Load(symbol)
		if holdingValue != nil && holdingValue.(*model.Position).Holding != 0 {
			util.Notice(fmt.Sprintf(`not removed %s %s %f`, account.Key, symbol, holdingValue.(*model.Position).Holding))
			removed = false
		}
	}
	if removed {
		return nil, false
	}
	util.Notice(fmt.Sprintf(`need to create turtle data %s %s %s %s %d refresh %v`,
		function, market, symbol, nowStr, far, refreshDynamic))
	var getOne, getAll bool
	getOne, getAll, data = getCandleData(account, market, symbol, function, int(far), int(near), int(seconds), 5, amountRate, nowPeriod)
	if !getOne {
		util.Notice(fmt.Sprintf(`fail to getOne %s %s %d %d`, market, symbol, data.DaysFar, seconds))
		return nil, false
	} else if !getAll {
		util.Notice(fmt.Sprintf(`fail to getAll %s %s %d %d`, market, symbol, data.DaysFar, seconds))
		return nil, true
	}
	data.OrderCleared = lastHandled
	if lastHandled {
		data.CheckTimeOpen = time.Now()
	}
	if data.Amount > 0 && data.N > 0 {
		var marketInfo *model.MarketInfo
		v, _ := util.LoadSyncMap(model.MarketInfos, market, symbol)
		if v != nil {
			marketInfo = v.(*model.MarketInfo)
		}
		if marketInfo == nil {
			util.Notice(`fail to get marketInfo %s %s`, market, symbol)
			return nil, false
		} else {
			if marketInfo.CTValue == 0 {
				data.AmountMin = marketInfo.SizeMin
			} else {
				data.AmountMin = marketInfo.SizeMin * marketInfo.CTValue
			}
			data.AmountMin = math.Max(data.AmountMin, 2*marketInfo.MoneyMin/data.LowFar)
		}
		if refreshDynamic {
			util.StoreSyncMap(&TurtleDataSet, data, function, market, symbol, nowStr)
			util.Notice(fmt.Sprintf(`set turtle %s %s %s %s Amount:%e AmountMin:%e N:%e %d:%e-%e %d:%e-%e %v`,
				function, market, symbol, nowStr, data.Amount, data.AmountMin, data.N,
				data.DaysNear, data.LowNear, data.HighNear, data.DaysFar, data.LowFar, data.HighFar, data))
		}
		util.Notice(fmt.Sprintf(`set data %s %s %s %f %f`, market, symbol, function, data.N, data.Amount))
		return data, true
	} else {
		return nil, false
	}
}

func getCandleData(account *model.Account, market, symbol, function string, far, near, seconds, adjust int, amountRate float64, nowPeriod time.Time) (
	getOne, getAll bool, data *model.TurtleData) {
	candles := getTurtleCandles(account, market, symbol, far, seconds, nowPeriod)
	data = &model.TurtleData{TurtleTime: nowPeriod, Symbol: symbol, Big: 1, DaysFar: far, DaysNear: near,
		DaysAdjust: adjust, OrderAdjust: make(map[string]*model.Order)}
	priceClose := 0.0
	for i := 1; i <= far; i++ {
		currentPeriod := nowPeriod.Add(time.Second * time.Duration(seconds*-i))
		candle := findCandle(candles, currentPeriod)
		if candle == nil || candle.PriceHigh == 0 || candle.PriceLow == 0 {
			if time.Now().Second() == 0 {
				util.Notice(`can not calc turtleDate as nil candle %s %s %s %d`,
					market, symbol, currentPeriod.String(), len(candles))
			}
			return
		}
		getOne = true
		if candle.PriceHigh > data.HighFar && i <= far {
			data.HighFar = candle.PriceHigh
		}
		if (data.LowFar == 0 || data.LowFar > candle.PriceLow) && i <= far {
			data.LowFar = candle.PriceLow
		}
		if candle.PriceHigh > data.HighNear && i <= near {
			data.HighNear = candle.PriceHigh
		}
		if (data.LowNear == 0 || data.LowNear > candle.PriceLow) && i <= near {
			data.LowNear = candle.PriceLow
		}
		if candle.PriceHigh > data.HighAdjust && i <= adjust {
			data.HighAdjust = candle.PriceHigh
		}
		if (data.LowAdjust == 0 || data.LowAdjust > candle.PriceLow) && i <= adjust {
			data.LowAdjust = candle.PriceLow
		}
		if i == 1 {
			go model.AppDB.Save(candle)
			data.N = candle.N
			data.NVolume = candle.NVolume
			data.M = candle.M
			priceClose = candle.PriceClose
			data.Amount = CalcTurtleAmount(account, data.N, amountRate, candle)
			util.Info(fmt.Sprintf(`calc amt %s %s %s %f %f %f`, market, symbol, function, data.N, amountRate, data.Amount))
		}
	}
	if function == model.FunctionTurtle || function == model.FunctionTurtleNormal {
		data.UseNear = true
	} else if function == model.FunctionCombineTurtle {
		data.UseNear = false
	} else if function == model.FunctionBoost {
		data.UseNear = true
		openOrders := QueryOpenOrders(account.Key, account.Secret, market, symbol)
		data.OrderTrail = make([]*model.Order, 0)
		for _, order := range openOrders {
			if order.OrderType == model.OrderTypeTrailStop {
				order.Function = model.Close
				data.OrderTrail = append(data.OrderTrail, order)
			}
		}
	}
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	if model.CommonCoins[strings.ToLower(coin)] {
		if function == model.FunctionCombineTurtle {
			data.Amount = math.Min(data.Amount, 640000/priceClose)
		} else {
			data.Amount = math.Min(data.Amount, 800000/priceClose)
		}
	} else {
		if function == model.FunctionCombineTurtle {
			data.Amount = math.Min(data.Amount, 40000/priceClose)
		} else {
			data.Amount = math.Min(data.Amount, 50000/priceClose)
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
	calcLenV := 10
	sortedCandles := &model.SortedCandle{Value: candles}
	sort.Sort(sortedCandles)
	if lastCandle2 == nil {
		CalcCandleN(sortedCandles, calcLenN, calcLenV)
	} else {
		lastCandle := sortedCandles.Value[len(sortedCandles.Value)-1]
		lastCandle.N = (lastCandle2.N*float64(calcLenN-1) + lastCandle.PriceHigh - lastCandle.PriceLow) / float64(calcLenN)
		lastCandle.NVolume = (lastCandle2.NVolume*float64(calcLenV-1) + lastCandle.Volume) / float64(calcLenV)
		util.Notice(fmt.Sprintf(`base on last 2 candle %s %s far %d %d n-n %f %f nv-nv %f %f len %d volume len %d`,
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
		util.Notice(fmt.Sprintf(`no found candle %s %s %s %s`,
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
	if account == nil || setting == nil || setting.Seconds <= 0 {
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
		util.Notice(`wrong combine turtle parameter`)
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
		var orderLong, orderShort, orderTrail *model.Order
		if data.OrderLong != nil && len(data.OrderLong) > 0 {
			orderLong = data.OrderLong[0]
		}
		if data.OrderShort != nil && len(data.OrderShort) > 0 {
			orderShort = data.OrderShort[0]
		}
		if data.OrderTrail != nil && len(data.OrderTrail) > 0 {
			orderTrail = data.OrderTrail[0]
		}
		if orderLong != nil {
			if tick != nil && orderLong.TriggerPrice > 0 &&
				((orderLong.OrderType == model.OrderTypeStop && orderLong.TriggerPrice <= tick.Bids[0].Price) ||
					(orderLong.OrderType == model.OrderTypeLimit && orderLong.Price >= tick.Asks[0].Price)) {
				orderLong.Status = model.CarryStatusSuccess
			}
			if orderLong.Status == model.CarryStatusWorking && (useApi || tick == nil) {
				orderLong = QueryOrderById(account.Key, account.Secret, market, symbol, orderLong.OrderType, orderLong.OrderId)
				time.Sleep(time.Millisecond * 200)
			}
			if orderLong != nil && (orderLong.Status == model.CarryStatusSuccess || orderLong.DealAmount*3 > orderLong.Amount*2) {
				data.BreakLong = true
				for _, order := range data.OrderLong {
					data.OrderAdjust[order.OrderId] = order
				}
				util.Notice(fmt.Sprintf(`order break long %s %s %s %d %e %e id %s usdApi %v`,
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
				orderShort = QueryOrderById(account.Key, account.Secret, market, symbol, orderShort.OrderType, orderShort.OrderId)
				time.Sleep(time.Millisecond * 200)
			}
			if orderShort != nil && (orderShort.Status == model.CarryStatusSuccess || orderShort.DealAmount*3 > orderShort.Amount*2) {
				data.BreakShort = true
				for _, order := range data.OrderShort {
					data.OrderAdjust[order.OrderId] = order
				}
				util.Notice(fmt.Sprintf(`order break short %s %s %s %d %e %e id %s useApi %v`,
					market, symbol, orderShort.OrderType, setting.Chance, orderShort.TriggerPrice, orderShort.Price, orderShort.OrderId, useApi))
			}
		}
		if orderTrail != nil && (useApi || tick == nil) {
			orderTrail = QueryOrderById(account.Key, account.Secret, market, symbol, orderTrail.OrderType, orderTrail.OrderId)
			time.Sleep(time.Millisecond * 200)
			if orderTrail != nil && orderTrail.Status == model.CarryStatusSuccess {
				data.BreakTrail = true
				util.Notice(fmt.Sprintf(`order break trail %s %s %s %d %e %e id %s useApi %v`,
					market, symbol, orderTrail.OrderType, setting.Chance, orderTrail.TriggerPrice, orderTrail.Price, orderShort.OrderId, useApi))
			}
		}
	}
	return useApi
}

func CanOpenCombine(settingCombine, settingNormal *model.Setting, data, dataNormal *model.TurtleData, checkFulled bool) (canOpen bool, inAll float64) {
	success, _, coin, _ := model.GetFromStandard(settingCombine.Market, settingCombine.Symbol)
	if !success {
		return false, 0
	}
	settingsCombine := GetSettings(settingCombine.Function, settingCombine.Market)
	settingsNormal := GetSettings(settingNormal.Function, settingCombine.Market)
	if settingsCombine == nil || settingsNormal == nil {
		return false, 0
	}
	//tradingSymbols := make(map[string]bool)
	//addTrading := func(symbol, value any) bool {
	//	if value != nil {
	//		valueSetting := value.(*model.Setting)
	//		_, _, valueCoin, _ := model.GetFromStandard(valueSetting.Market, valueSetting.Symbol)
	//		if !model.CommonCoins[strings.ToLower(valueCoin)] {
	//			if valueSetting.Chance != 0 && valueSetting.Function == model.FunctionTurtleNormal {
	//				tradingSymbols[valueSetting.Symbol] = true
	//			}
	//		}
	//	}
	//	return true
	//}
	sumChance := func(symbol, value any) bool {
		if value != nil {
			valueSetting := value.(*model.Setting)
			_, _, valueCoin, _ := model.GetFromStandard(valueSetting.Market, valueSetting.Symbol)
			if !model.CommonCoins[strings.ToLower(valueCoin)] {
				if valueSetting.Function == model.FunctionTurtleNormal {
					inAll += float64(valueSetting.Chance)
				}
			}
		}
		return true
	}
	if model.CommonCoins[strings.ToLower(coin)] {
		return true, 0
	} else {
		settingsNormal.Range(sumChance)
		settingsCombine.Range(sumChance)
		canOpen = settingCombine.Chance != 0 || settingNormal.Chance != 0 || (inAll < settingCombine.AmountLimit &&
			settingCombine.SymbolRelated != model.SettingTurtleRemoved && settingNormal.SymbolRelated != model.SettingTurtleRemoved)
		if checkFulled {
			now := time.Now()
			_, nowStr := model.GetNowPeriod(settingCombine.Market, settingCombine.Seconds, now)
			fulled, _ := util.LoadSyncMap(CombineFulled, settingCombine.Market, nowStr)
			if fulled != nil && fulled.(bool) {
				canOpen = settingCombine.Chance != 0 || settingNormal.Chance != 0
			} else if inAll >= settingCombine.AmountLimit {
				util.StoreSyncMap(CombineFulled, true, settingCombine.Market, nowStr)
			}
			//settingNormal.ChanceLimitCombine = int64(inAll)
		}
		if settingCombine.Chance == 0 && !canOpen && inAll >= settingCombine.AmountLimit {
			data.OrderLong = nil
			data.OrderShort = nil
		}
		if settingNormal.Chance == 0 && !canOpen && inAll >= settingCombine.AmountLimit {
			dataNormal.OrderLong = nil
			dataNormal.OrderShort = nil
		}
	}
	return canOpen, inAll
}

// CanOpenTurtle CanOpenTurtle  主流币检查仓位总数;非主流检查交易币种个数
// isChance true: 按照仓数计算
// isChance false: 按照开仓币种数计算
// 当setting.OpenShortMargin小于0时不限制本币种仓位，等于0时不许开仓（未实现）
// 当setting.AmountLimit小于0时不限制总仓位或开仓币数，等于0时不许开仓（未实现）
func CanOpenTurtle(setting *model.Setting, data *model.TurtleData) (canOpen bool, inAll float64) {
	success, _, coin, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	if !success {
		return false, 0
	}
	settings := GetSettings(setting.Function, setting.Market)
	if settings == nil {
		return false, 0
	}
	if model.CommonCoins[strings.ToLower(coin)] {
		settings.Range(func(symbol, value any) bool {
			if value != nil {
				valueSetting := value.(*model.Setting)
				_, _, valueCoin, _ := model.GetFromStandard(valueSetting.Market, valueSetting.Symbol)
				if valueSetting.Market == setting.Market && valueSetting.Function == setting.Function &&
					model.CommonCoins[strings.ToLower(valueCoin)] {
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
				_, _, valueCoin, _ := model.GetFromStandard(valueSetting.Market, valueSetting.Symbol)
				if valueSetting.Market == setting.Market && valueSetting.Function == setting.Function &&
					!model.CommonCoins[strings.ToLower(valueCoin)] {
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

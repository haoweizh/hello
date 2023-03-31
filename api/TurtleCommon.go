package api

import (
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TurtleData struct {
	// useNear是否在海龟交易时使用lowDaysNear和highDaysNear和priceX作为触发条件
	// adjustChecked在设置为true前，不允许使用本Data进行交易
	UseNear, BreakLong, BreakShort, Liquidated, AdjustChecked, OrderCleared   bool
	TurtleTime, CheckTimeBreak, CheckTimeOpen                                 time.Time
	HighDaysNear, LowDaysNear, HighDaysFar, LowDaysFar, LowAdjust, HighAdjust float64
	HighToday, LowToday, N, Amount                                            float64
	DaysNear, DaysFar, DaysAdjust, combineBig                                 int // CombineBig: -1小单，1大单，0未初始化
	Symbol                                                                    string
	// 适应某些交易所单笔订单不能过大，大笔订单会拆分后下成多个，因价格超出无法下成的单为了不被取消，也归入orderAdjust
	OrderLong, OrderShort, OrderAdjust []*model.Order
}

func (turtleData *TurtleData) GetIds() (ids string) {
	ids = `long:`
	if turtleData.OrderLong != nil && len(turtleData.OrderLong) > 0 {
		ids += turtleData.OrderLong[0].OrderId
	}
	ids += `short:`
	if turtleData.OrderShort != nil && len(turtleData.OrderShort) > 0 {
		ids += turtleData.OrderShort[0].OrderId
	}
	return
}

func (turtleData *TurtleData) SetBig(isBig int) {
	turtleData.combineBig = isBig
}

func (turtleData *TurtleData) IsBig(settingCombine, settingNormal *model.Setting, marketInfo *model.MarketInfo) (isBig int) {
	if turtleData.combineBig == 0 {
		if settingCombine.Chance+settingNormal.Chance == 0 && math.Abs(settingCombine.PriceX-settingNormal.PriceX) < marketInfo.PriceIncrement {
			turtleData.combineBig = -1
		} else {
			turtleData.combineBig = 1
		}
	}
	return turtleData.combineBig
}

func (turtleData *TurtleData) ToString() (str string) {
	if turtleData == nil {
		return `turtle data is nil`
	}
	return fmt.Sprintf(`%d日%e~%e N:%e Amount:%e`,
		turtleData.DaysFar, turtleData.LowDaysFar, turtleData.HighDaysFar, turtleData.N, turtleData.Amount)
}

const TurtleTriggerDelta = 0.01
const TurtleFar = 18
const TurtleFarBTC = 50

var TurtleDataSet = sync.Map{} // function_market_symbol_2019-12-06 *TurtleData
var queryDataTime = &sync.Map{}

var accountValues = &sync.Map{}    // market value
var accountValueTime = &sync.Map{} // market time.Time
func CalcTurtleAmount(key, secret, market, symbol string, n float64) (amount float64) {
	var accountValue float64
	valueTime, _ := util.LoadSyncMap(accountValueTime, market)
	if valueTime != nil && valueTime.(time.Time).Add(time.Hour).After(time.Now()) {
		value, _ := util.LoadSyncMap(accountValues, market)
		if value != nil {
			accountValue = value.(float64)
		}
	}
	if accountValue == 0 {
		switch market {
		case model.BinancePerp:
			_, _, accountValue, _ = GetPositions(key, secret, market)
		case model.Ftx, model.OKEX:
			_, _, accountValue, _ = GetBalances(key, secret, market)
		}
		util.StoreSyncMap(accountValues, accountValue, market)
		util.StoreSyncMap(accountValueTime, time.Now(), market)
	}
	amount = 0.02 * accountValue / n
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	if model.CommonCoins[strings.ToLower(coin)] {
		amount = amount / 2
	} else {
		amount /= 4
	}
	//util.Notice(`CalcTurtleAmount %s %s %e`, setting.Market, setting.Symbol, Amount)
	return amount
}

// ClearOrders
// 取消market交易所中symbol交易对的所有limit、stop单
func ClearOrders(key, secret, market, symbol string) {
	ordersLimit := QueryOpenOrders(key, secret, market, symbol, false)
	ordersStop := QueryOpenOrders(key, secret, market, symbol, true)
	for _, order := range ordersLimit {
		if order != nil {
			util.Notice(`cancel pending turtle limit order %s %s %s`, market, symbol, order.OrderId)
			MustCancel(key, secret, market, symbol, model.OrderTypeLimit, order.OrderId, true)
		}
	}
	for _, order := range ordersStop {
		if order != nil {
			util.Notice(`cancel pending turtle stop order %s %s %s`, market, symbol, order.OrderId)
			MustCancel(key, secret, market, symbol, model.OrderTypeStop, order.OrderId, true)
		}
	}
}

// ClearExtraOrders
// 取消market交易所中symbol交易对中没有被纳入管理或已经超出仓数限制的订单
func ClearExtraOrders(key, secret, market, symbol string, dataArray []*TurtleData) {
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
	}
	ordersStop := QueryOpenOrders(key, secret, market, symbol, true)
	for _, order := range ordersStop {
		if !keepOrders[order.OrderId] {
			result := MustCancel(key, secret, market, symbol, order.OrderType, order.OrderId, true)
			util.Notice(`cancel extra stop order %s %s %s %s return %v`, market, symbol, order.OrderType, order.OrderId, result)
			time.Sleep(time.Second)
		}
	}
	ordersLimit := QueryOpenOrders(key, secret, market, symbol, false)
	for _, order := range ordersLimit {
		if !keepOrders[order.OrderId] {
			result := MustCancel(key, secret, market, symbol, order.OrderType, order.OrderId, true)
			util.Notice(`cancel extra limit order %s %s %s %s return %v`, market, symbol, order.OrderType, order.OrderId, result)
			time.Sleep(time.Second)
		}
	}
}

func AdjustPosHolding(key, secret string, setting *model.Setting, data *TurtleData) {
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
			util.Notice(`update turtle side %s %s %d %e %e from %d`,
				setting.Market, setting.Symbol, setting.Chance, posMap[setting.Symbol].Holding, setting.GridAmount, setting.Chance)
			setting.GridAmount = 0
			setting.Chance = 0
			setting.PriceX = 0
			if posMap[setting.Symbol].Holding > 0 {
				data.OrderAdjust = MustPlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtleAdjust, data.LowAdjust*(1-TurtleTriggerDelta), data.LowAdjust, posMap[setting.Symbol].Holding, setting)
			} else if posMap[setting.Symbol].Holding < 0 {
				data.OrderAdjust = MustPlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeStop, setting.Market, setting.Symbol, ``,
					model.FunctionTurtleAdjust, data.HighAdjust*(1+TurtleTriggerDelta), data.HighAdjust, -1*posMap[setting.Symbol].Holding, setting)
			}
			for _, order := range data.OrderAdjust {
				if order != nil {
					order.RefreshType = model.FunctionTurtleAdjust
					model.AppDB.Save(order)
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
		util.Notice(`update turtle when absent %s %s %d`, setting.Market, setting.Symbol, len(posMap))
		for s, position := range posMap {
			util.Notice(`present %s %s %e`, s, position.Currency, position.Holding)
		}
	}
	model.AppDB.Save(setting)
}

// handleTraceOrders
func _(key, secret, market, symbol string, settings []*model.Setting, turtleData []*TurtleData) {

	today, _ := model.GetMarketToday(market)
	dayTime, _ := time.ParseDuration(`86400s`)
	var candles []*model.Candle
	// okex不返回尚未结束的当日candle，转成半小时的slot
	if market == model.OKEX {
		candles = GetCandle(key, secret, market, symbol, 1800, today, model.GetMarketNow(market))
	} else {
		candles = GetCandle(key, secret, market, symbol, 86400, today, today.Add(dayTime))
	}
	for i, setting := range settings {
		data := turtleData[i]
		for j := 0; candles != nil && j < len(candles); j++ {
			if data.HighToday < candles[j].PriceHigh {
				data.HighToday = candles[j].PriceHigh
				util.Info(fmt.Sprintf(`get today len new high %s %s %d %e`, market, symbol, len(candles), candles[j].PriceHigh))
			}
			if data.LowToday == 0 || data.LowToday > candles[j].PriceLow {
				data.LowToday = candles[j].PriceLow
				util.Info(fmt.Sprintf(`get today len new low %s %s %d %e`, market, symbol, len(candles), candles[j].PriceLow))
			}
		}
		if data.OrderShort == nil || len(data.OrderShort) == 0 {
			data.OrderShort = nil
		} else if !data.UseNear && setting.Chance > 0 && data.LowToday > 0 &&
			((data.OrderShort[0].OrderType == model.OrderTypeLimit && data.OrderShort[0].Price > math.Min(data.LowToday, data.LowDaysFar)+2*data.N) ||
				(data.OrderShort[0].OrderType == model.OrderTypeStop && data.OrderShort[0].TriggerPrice < math.Max(data.HighDaysFar, data.HighToday)-2*data.N)) {
			util.Notice(fmt.Sprintf(`today higher than far price%e<max(today%e,far%e)-2*%e chance%d`,
				data.OrderShort[0].TriggerPrice, data.HighToday, data.HighDaysFar, data.N, setting.Chance))
			data.OrderShort = nil
		}
		if data.OrderLong == nil || len(data.OrderLong) == 0 {
			data.OrderLong = nil
		} else if !data.UseNear && setting.Chance < 0 && data.LowToday > 0 &&
			((data.OrderLong[0].OrderType == model.OrderTypeLimit && data.OrderLong[0].Price < math.Max(data.HighDaysFar, data.HighToday)-2*data.N) ||
				(data.OrderLong[0].OrderType == model.OrderTypeStop && data.OrderLong[0].TriggerPrice > math.Min(data.LowDaysFar, data.LowToday)+2*data.N)) {
			util.Notice(fmt.Sprintf(`today lower than far price%e>min(today%e,far%e)+2*%e chance%d`,
				data.OrderLong[0].TriggerPrice, data.LowToday, data.LowDaysFar, data.N, setting.Chance))
			data.OrderLong = nil
		}
	}
}

func HandleOrders(key, secret, market, symbol string, settings []*model.Setting, turtleData []*TurtleData) (checked bool) {
	if (len(settings) != 2 && len(settings) != 1) || len(settings) != len(turtleData) {
		util.Notice(`wrong combine turtle parameter`)
		return false
	}
	if turtleData[0].CheckTimeOpen.Add(time.Minute * 10).After(util.GetNow()) {
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
	turtleData[0].CheckTimeOpen = util.GetNow()
	ClearExtraOrders(key, secret, market, symbol, turtleData)
	return true
}

func GetTurtleData(key, secret, function, market, symbol string) (data *TurtleData) {
	today, todayStr := model.GetMarketToday(market)
	value, ok := util.LoadSyncMap(&TurtleDataSet, function, market, symbol, todayStr)
	if ok && value != nil {
		return value.(*TurtleData)
	}
	value, ok = util.LoadSyncMap(queryDataTime, function, market, symbol, todayStr)
	if ok && value != nil {
		if value.(time.Time).Add(time.Minute * 60).After(util.GetNow()) {
			return nil
		}
	}
	util.StoreSyncMap(queryDataTime, util.GetNow(), function, market, symbol, todayStr)
	util.Notice(fmt.Sprintf(`need to create turtle data %s %s %s %s`, function, market, symbol, todayStr))
	useNear := false
	if function == model.FunctionTurtle || function == model.FunctionTurtleNormal {
		useNear = true
	} else if function == model.FunctionCombineTurtle {
		useNear = false
	}
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	far := TurtleFar
	if strings.ToUpper(coin) == `BTC` {
		far = TurtleFarBTC
	}
	data = &TurtleData{TurtleTime: today, Symbol: symbol, BreakLong: false, BreakShort: false, Liquidated: false,
		DaysFar: far, DaysNear: far / 2, DaysAdjust: 5, UseNear: useNear}
	indexMax := math.Max(21.0, float64(data.DaysFar))
	duration, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*int(indexMax)))
	candles := GetCandle(key, secret, market, symbol, 86400, today.Add(duration), today)
	for _, item := range candles {
		if item == nil {
			continue
		}
		value, ok = util.LoadSyncMap(CandleMap, market, symbol, strconv.Itoa(item.Seconds), item.Begin.Format(time.RFC3339))
		if value == nil {
			util.StoreSyncMap(CandleMap, item, market, symbol, strconv.Itoa(item.Seconds), item.Begin.Format(time.RFC3339))
		}
	}
	for i := 1; i <= int(indexMax); i++ {
		duration, _ = time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
		day := today.Add(duration)
		candle := CalcCandleN(market, symbol, 86400, day)
		if candle == nil || candle.PriceHigh == 0 || candle.PriceLow == 0 {
			if time.Now().Second() == 0 {
				util.Notice(`can not calc turtleDate as nil candle %s %s %s %s`,
					market, symbol, data.Symbol, day.String())
			}
			return nil
		}
		if candle.PriceHigh > data.HighDaysFar && i <= data.DaysFar {
			data.HighDaysFar = candle.PriceHigh
		}
		if (data.LowDaysFar == 0 || data.LowDaysFar > candle.PriceLow) && i <= data.DaysFar {
			data.LowDaysFar = candle.PriceLow
		}
		if candle.PriceHigh > data.HighDaysNear && i <= data.DaysNear {
			data.HighDaysNear = candle.PriceHigh
		}
		if (data.LowDaysNear == 0 || data.LowDaysNear > candle.PriceLow) && i <= data.DaysNear {
			data.LowDaysNear = candle.PriceLow
		}
		if candle.PriceHigh > data.HighAdjust && i <= data.DaysAdjust {
			data.HighAdjust = candle.PriceHigh
		}
		if (data.LowAdjust == 0 || data.LowAdjust > candle.PriceLow) && i <= data.DaysAdjust {
			data.LowAdjust = candle.PriceLow
		}
		if i == 1 {
			data.N = candle.N
			data.Amount = CalcTurtleAmount(key, secret, market, symbol, data.N)
		}
	}
	if data.Amount > 0 && data.N > 0 {
		util.StoreSyncMap(&TurtleDataSet, data, function, market, symbol, todayStr)
		util.Notice(fmt.Sprintf(`set turtle data %v %s %s %s %s  Amount:%e N:%e %d:%e-%e %d:%e-%e`,
			data, function, market, symbol, todayStr, data.Amount, data.N, data.DaysNear, data.LowDaysNear,
			data.HighDaysNear, data.DaysFar, data.LowDaysFar, data.HighDaysFar))
	}
	return
}

func SetTurtleOrderStatus(function, market, symbol, orderId, status string) {
	setting := GetSetting(function, market, symbol)
	if setting == nil {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	if account == nil {
		return
	}
	_, todayStr := model.GetMarketToday(market)
	value, ok := util.LoadSyncMap(&TurtleDataSet, function, market, symbol, todayStr)
	if ok && value != nil {
		turtleData := value.(*TurtleData)
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

func CheckBreak(key, secret, market, symbol string, settings []*model.Setting, turtleData []*TurtleData,
	tick *model.BidAsk) (checked bool) {
	if (len(settings) != 2 && len(settings) != 1) || len(settings) != len(turtleData) {
		util.Notice(`wrong combine turtle parameter`)
		return false
	}
	if turtleData[0].CheckTimeBreak.Add(time.Minute * 5).After(util.GetNow()) {
		return false
	}
	for i, setting := range settings {
		data := turtleData[i]
		data.CheckTimeBreak = util.GetNow()
		var orderLong, orderShort *model.Order
		if data.OrderLong != nil && len(data.OrderLong) > 0 {
			orderLong = data.OrderLong[0]
		}
		if data.OrderShort != nil && len(data.OrderShort) > 0 {
			orderShort = data.OrderShort[0]
		}
		//if orderLong != nil && (orderLong.Status == model.CarryStatusSuccess || (orderLong.TriggerPrice > 0 &&
		//	(orderLong.OrderType == model.OrderTypeStop && orderLong.TriggerPrice <= tick.Bids[0].Price) ||
		//	(orderLong.OrderType == model.OrderTypeLimit && orderLong.Price > tick.Bids[0].Price))) {
		if orderLong != nil {
			if orderLong.Status == model.CarryStatusWorking {
				time.Sleep(time.Second * 3)
				orderLong = QueryOrderById(key, secret, market, symbol, orderLong.OrderType, orderLong.OrderId)
			}
			if orderLong != nil && orderLong.Status == model.CarryStatusSuccess {
				data.BreakLong = true
				util.Notice(fmt.Sprintf(`order break long %s %s %s %d bid-ask %e %e %e %e id %s`,
					market, symbol, orderLong.OrderType, setting.Chance, tick.Bids[0].Price,
					tick.Asks[0].Price, orderLong.TriggerPrice, orderLong.Price, orderLong.OrderId))
			}
		}
		//if orderShort != nil && (orderShort.Status == model.CarryStatusSuccess || (orderShort.TriggerPrice > 0 &&
		//	(orderShort.OrderType == model.OrderTypeStop && orderShort.TriggerPrice >= tick.Asks[0].Price)) ||
		//	(orderShort.OrderType == model.OrderTypeLimit && orderShort.Price < tick.Asks[0].Price)) {
		if orderShort != nil {
			if orderShort.Status == model.CarryStatusWorking {
				time.Sleep(time.Second * 3)
				orderShort = QueryOrderById(key, secret, market, symbol, orderShort.OrderType, orderShort.OrderId)
			}
			if orderShort != nil && orderShort.Status == model.CarryStatusSuccess {
				data.BreakShort = true
				util.Notice(fmt.Sprintf(`order break short %s %s %s %d bid-ask %e %e %e %e id %s`,
					market, symbol, orderShort.OrderType, setting.Chance, tick.Bids[0].Price,
					tick.Asks[0].Price, orderShort.TriggerPrice, orderShort.Price, orderShort.OrderId))
			}
		}
	}
	return true
}

func CanOpenCombine(setting, settingNormal *model.Setting, data, dataNormal *TurtleData) (canOpen bool, inAll float64) {
	success, _, coin, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	if !success {
		return false, 0
	}
	settings := GetSettings(setting.Function, setting.Market)
	settingsNormal := GetSettings(model.FunctionTurtleNormal, setting.Market)
	if settings == nil || settingsNormal == nil {
		return false, 0
	}
	tradingSymbols := make(map[string]bool)
	addChance := func(symbol, value any) bool {
		if value != nil {
			valueSetting := value.(*model.Setting)
			_, _, valueCoin, _ := model.GetFromStandard(valueSetting.Market, valueSetting.Symbol)
			if model.CommonCoins[strings.ToLower(valueCoin)] {
				inAll += float64(valueSetting.Chance)
			}
		}
		return true
	}
	addTrading := func(symbol, value any) bool {
		if value != nil {
			valueSetting := value.(*model.Setting)
			_, _, valueCoin, _ := model.GetFromStandard(valueSetting.Market, valueSetting.Symbol)
			if !model.CommonCoins[strings.ToLower(valueCoin)] {
				if valueSetting.Chance != 0 {
					tradingSymbols[valueSetting.Symbol] = true
				}
			}
		}
		return true
	}
	if model.CommonCoins[strings.ToLower(coin)] {
		settings.Range(addChance)
		settingsNormal.Range(addChance)
		canOpen = math.Abs(inAll) < setting.AmountLimit
		if !canOpen && inAll > 0 && setting.Chance >= 0 {
			data.OrderLong = nil
		}
		if !canOpen && inAll < 0 && setting.Chance <= 0 {
			data.OrderShort = nil
		}
		if !canOpen && inAll > 0 && settingNormal.Chance >= 0 {
			dataNormal.OrderLong = nil
		}
		if !canOpen && inAll < 0 && settingNormal.Chance <= 0 {
			dataNormal.OrderShort = nil
		}
	} else {
		settingsNormal.Range(addTrading)
		settings.Range(addTrading)
		inAll = float64(len(tradingSymbols))
		canOpen = setting.Chance != 0 || settingNormal.Chance != 0 || (math.Abs(inAll) < setting.AmountLimit &&
			setting.SymbolRelated != model.SettingTurtleRemoved && settingNormal.SymbolRelated != model.SettingTurtleRemoved)
		//if setting.Chance == 0 && !canOpen {
		//	data.OrderLong = nil
		//	data.OrderShort = nil
		//}
		//if settingNormal.Chance == 0 && !canOpen {
		//	dataNormal.OrderLong = nil
		//	dataNormal.OrderShort = nil
		//}
	}
	return canOpen, inAll
}

// CanOpenTurtle CanOpenTurtle  主流币检查仓位总数;非主流检查交易币种个数
// isChance true: 按照仓数计算
// isChance false: 按照开仓币种数计算
// 当setting.OpenShortMargin小于0时不限制本币种仓位，等于0时不许开仓（未实现）
// 当setting.AmountLimit小于0时不限制总仓位或开仓币数，等于0时不许开仓（未实现）
func CanOpenTurtle(setting *model.Setting, data *TurtleData) (canOpen bool, inAll float64) {
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
		canOpen = setting.Chance != 0 || (setting.SymbolRelated != model.SettingTurtleRemoved && math.Abs(inAll) < setting.AmountLimit)
		//if setting.Chance == 0 && !canOpen {
		//	data.OrderLong = nil
		//	data.OrderShort = nil
		//}
	}
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < setting.OpenShortMargin
	return canOpen, inAll
}

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

type TurtleData struct {
	// useNear是否在海龟交易时使用lowDaysNear和highDaysNear和priceX作为触发条件
	// adjustChecked在设置为true前，不允许使用本Data进行交易
	UseNear, BreakLong, BreakShort, Liquidated, AdjustChecked, OrderCleared   bool
	TurtleTime, CheckUseApi, CheckTimeOpen                                    time.Time
	HighDaysNear, LowDaysNear, HighDaysFar, LowDaysFar, LowAdjust, HighAdjust float64
	HighToday, LowToday, N, NVolume, Amount                                   float64
	DaysNear, DaysFar, DaysAdjust, combineBig                                 int // CombineBig: -1小单，1大单，0未初始化
	Symbol                                                                    string
	// 适应某些交易所单笔订单不能过大，大笔订单会拆分后下成多个，因价格超出无法下成的单为了不被取消，也归入orderAdjust
	OrderLong, OrderShort, OrderAdjust []*model.Order
}

type TurtleDataArray []*TurtleData

func (turtleDataArray TurtleDataArray) Len() int {
	return len(turtleDataArray)
}

func (turtleDataArray TurtleDataArray) Swap(i, j int) {
	turtleDataArray[i], turtleDataArray[j] = turtleDataArray[j], turtleDataArray[i]
}

func (turtleDataArray TurtleDataArray) Less(i, j int) bool {
	return turtleDataArray[i].NVolume < turtleDataArray[j].NVolume
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
	if settingCombine.Far != settingNormal.Far || settingCombine.Near != settingNormal.Near {
		return 1
	}
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

var TurtleDataSet = sync.Map{}     // function_market_symbol_unix second *TurtleData
var accountValues = &sync.Map{}    // market value
var accountValueTime = &sync.Map{} // market time.Time
func CalcTurtleAmount(key, secret string, setting *model.Setting, n float64) (amount float64) {
	var accountValue float64
	valueTime, _ := util.LoadSyncMap(accountValueTime, setting.Market)
	if valueTime != nil && valueTime.(time.Time).Add(time.Hour).After(time.Now()) {
		value, _ := util.LoadSyncMap(accountValues, setting.Market)
		if value != nil {
			accountValue = value.(float64)
		}
	}
	if accountValue == 0 {
		switch setting.Market {
		case model.BinancePerp:
			_, _, accountValue, _ = GetPositions(key, secret, setting.Market)
		case model.Ftx, model.OKEX:
			_, _, accountValue, _ = GetBalances(key, secret, setting.Market)
		}
		util.StoreSyncMap(accountValues, accountValue, setting.Market)
		util.StoreSyncMap(accountValueTime, time.Now(), setting.Market)
	}
	amount = 0.02 * accountValue / n
	amount *= setting.CloseShortMargin
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
		//util.Notice(`update turtle when absent %s %s %d`, setting.Market, setting.Symbol, len(posMap))
		for s, position := range posMap {
			util.Notice(`present %s %s %e`, s, position.Currency, position.Holding)
		}
	}
	model.AppDB.Save(setting)
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

const turtleNSlots = 100
const turtleNSlotsMin = 10

func GetTurtleData(key, secret, function, symbol string, setting *model.Setting, refreshDynamic bool) (data *TurtleData, settingRefresh *model.Setting) {
	var nowPeriod time.Time
	var nowStr string
	if setting.Seconds >= 86400 { // 周期大于1天时，需要考虑不同交易所的时区
		nowPeriod, nowStr = model.GetMarketToday(setting.Market)
	} else {
		nowPeriod, nowStr = model.GetNowPeriod(setting.Seconds)
	}
	value, ok := util.LoadSyncMap(&TurtleDataSet, function, setting.Market, symbol, nowStr)
	if ok && value != nil {
		return value.(*TurtleData), setting
	}
	_, _, coin, _ := model.GetFromStandard(setting.Market, symbol)
	if refreshDynamic && !model.CommonCoins[strings.ToLower(coin)] {
		refreshValue, refreshOk := DynamicHandleTime.Load(setting.Market)
		if !refreshOk || refreshValue == nil || refreshValue.(time.Time).Add(time.Hour).Before(time.Now()) {
			if handleMarketDynamic(setting.Market) {
				PrepareSettings()
				SetRequireReset(setting.Market)
				setting = GetSetting(function, setting.Market, symbol)
			}
		}
	}
	if setting == nil {
		util.Notice(fmt.Sprintf(`fatal error nil setting after get turtle data`))
	}
	util.Notice(fmt.Sprintf(`need to create turtle data %s %s %s %s %d %d`,
		function, setting.Market, symbol, nowStr, setting.Near, setting.Far))
	useNear := false
	if function == model.FunctionTurtle || function == model.FunctionTurtleNormal {
		useNear = true
	} else if function == model.FunctionCombineTurtle {
		useNear = false
	}
	data = &TurtleData{TurtleTime: nowPeriod, Symbol: symbol, BreakLong: false, BreakShort: false, Liquidated: false,
		DaysFar: int(setting.Far), DaysNear: int(setting.Near), DaysAdjust: 5, UseNear: useNear}
	indexMax := math.Max(turtleNSlotsMin, float64(data.DaysFar))
	candles := CombineCandles(key, secret, setting.Market, symbol, int(setting.Seconds),
		nowPeriod.Add(time.Second*time.Duration(setting.Seconds*-1*turtleNSlots)), nowPeriod)
	if !CalcCandleN(candles) {
		util.Notice(fmt.Sprintf(`fail to calc candles n %s %s candle num %d`, setting.Market, symbol, len(candles)))
		return nil, setting
	}
	for i := 1; i <= int(indexMax); i++ {
		currentPeriod := nowPeriod.Add(time.Second * time.Duration(setting.Seconds*int64(-i)))
		candle := findCandle(candles, currentPeriod)
		if candle == nil || candle.PriceHigh == 0 || candle.PriceLow == 0 {
			if time.Now().Second() == 0 {
				util.Notice(`can not calc turtleDate as nil candle %s %s %s %s`,
					setting.Market, symbol, data.Symbol, currentPeriod.String())
			}
			return nil, setting
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
			data.NVolume = candle.NVolume
			data.Amount = CalcTurtleAmount(key, secret, setting, data.N)
			util.Notice(fmt.Sprintf(`set data %s %f %f`, function, data.N, data.Amount))
		}
	}
	if data.Amount > 0 && data.N > 0 {
		util.StoreSyncMap(&TurtleDataSet, data, function, setting.Market, symbol, nowStr)
		util.Notice(fmt.Sprintf(`set turtle %s %s %s %s Amount:%e N:%e %d:%e-%e %d:%e-%e %v`,
			function, setting.Market, symbol, nowStr, data.Amount, data.N, data.DaysNear, data.LowDaysNear,
			data.HighDaysNear, data.DaysFar, data.LowDaysFar, data.HighDaysFar, data))
		return data, setting
	} else {
		return nil, setting
	}
}

func findCandle(candles []*model.Candle, begin time.Time) (resultCandle *model.Candle) {
	for _, candle := range candles {
		if candle.Begin == begin {
			return candle
		}
	}
	return nil
}

const CandleMLen = 20

func CalcCandleN(candles []*model.Candle) (success bool) {
	if len(candles) < turtleNSlotsMin {
		return false
	}
	sortedCandles := model.SortedCandle{Value: candles}
	sort.Sort(sortedCandles)
	beginPrice := 0.0
	beginVolume := 0.0
	for i := 0; i < turtleNSlotsMin; i++ {
		beginPrice += sortedCandles.Value[i].PriceHigh - sortedCandles.Value[i].PriceLow
		beginVolume += sortedCandles.Value[i].Volume
	}
	sortedCandles.Value[turtleNSlotsMin-1].N = beginPrice / turtleNSlotsMin
	sortedCandles.Value[turtleNSlotsMin-1].NVolume = beginVolume / turtleNSlotsMin
	for i := turtleNSlotsMin; i < len(sortedCandles.Value); i++ {
		sortedCandles.Value[i].N = (sortedCandles.Value[i-1].N*9 + sortedCandles.Value[i].PriceHigh - sortedCandles.Value[i].PriceLow) / 10
		sortedCandles.Value[i].NVolume = (sortedCandles.Value[i-1].NVolume*9 + sortedCandles.Value[i].Volume) / 10
	}
	disAll := 0.0
	for i := 0; i < len(sortedCandles.Value); i++ {
		disAll += sortedCandles.Value[i].PriceHigh - sortedCandles.Value[i].PriceLow
		if i >= CandleMLen {
			disAll = disAll - sortedCandles.Value[i-CandleMLen].PriceHigh + sortedCandles.Value[i-CandleMLen].PriceLow
			sortedCandles.Value[i].M = disAll / CandleMLen
		}
	}
	//for i, candle := range candles {
	//	util.Notice(fmt.Sprintf(`candle calc %d %s %s %s %f`, i, candle.Market, candle.Symbol, candle.Begin.String(), candle.N))
	//}
	return true
}

func SetTurtleOrderStatus(function, market, symbol, orderId, status string) {
	setting := GetSetting(function, market, symbol)
	if setting == nil {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	if account == nil || setting == nil {
		return
	}
	var nowStr string
	if setting.Seconds >= 86400 { // 周期大于1天时，需要考虑不同交易所的时区
		_, nowStr = model.GetMarketToday(setting.Market)
	} else {
		_, nowStr = model.GetNowPeriod(setting.Seconds)
	}
	value, ok := util.LoadSyncMap(&TurtleDataSet, function, market, symbol, nowStr)
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
		var orderLong, orderShort *model.Order
		if data.OrderLong != nil && len(data.OrderLong) > 0 {
			orderLong = data.OrderLong[0]
		}
		if data.OrderShort != nil && len(data.OrderShort) > 0 {
			orderShort = data.OrderShort[0]
		}
		if orderLong != nil {
			if orderLong.TriggerPrice > 0 &&
				(orderLong.OrderType == model.OrderTypeStop && orderLong.TriggerPrice <= tick.Bids[0].Price) ||
				(orderLong.OrderType == model.OrderTypeLimit && orderLong.Price > tick.Bids[0].Price) {
				orderLong.Status = model.CarryStatusSuccess
			}
			if orderLong.Status == model.CarryStatusWorking && useApi {
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
		if orderShort != nil {
			if (orderShort.TriggerPrice > 0 &&
				(orderShort.OrderType == model.OrderTypeStop && orderShort.TriggerPrice >= tick.Asks[0].Price)) ||
				(orderShort.OrderType == model.OrderTypeLimit && orderShort.Price < tick.Asks[0].Price) {
				orderShort.Status = model.CarryStatusSuccess
			}
			if orderShort.Status == model.CarryStatusWorking && useApi {
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
		canOpen = setting.Chance != 0 || settingNormal.Chance != 0 || (inAll < setting.AmountLimit &&
			setting.SymbolRelated != model.SettingTurtleRemoved && settingNormal.SymbolRelated != model.SettingTurtleRemoved)
		if setting.Chance == 0 && !canOpen && inAll >= setting.AmountLimit {
			data.OrderLong = nil
			data.OrderShort = nil
		}
		if settingNormal.Chance == 0 && !canOpen && inAll >= setting.AmountLimit {
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
		canOpen = setting.Chance != 0 || (setting.SymbolRelated != model.SettingTurtleRemoved && inAll < setting.AmountLimit)
		if setting.Chance == 0 && !canOpen && inAll >= setting.AmountLimit {
			data.OrderLong = nil
			data.OrderShort = nil
		}
	}
	canOpen = canOpen && math.Abs(float64(setting.Chance)) < setting.OpenShortMargin
	return canOpen, inAll
}

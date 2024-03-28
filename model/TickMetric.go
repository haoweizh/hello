package model

import (
	"fmt"
	"hello/util"
	"math"
	"sync"
	"time"
)

//const recentTickLength = 100
//const tickErrorMsg = `有效tick低于2成或平均延迟大于50ms[最近tick %s]`

type TickMetric struct {
	delayLow    int
	delayHigh   int
	delaySum    int
	betweenLow  int
	betweenHigh int
	betweenSum  int
	countValid  int
	countAll    int
	delayAvg    float64
	betweenAvg  float64 // 两个tick之间的平均时间
	priceLow    float64
	priceHigh   float64
	start       time.Time
	end         time.Time
}

type CarryMetric struct {
	count        int64
	carryHighest float64
	carryLowest  float64
	totalHigh    float64
	totalLow     float64
	avgHigh      float64
	avgLow       float64
}

type TickDelay struct {
	receiveTime time.Time
	delay       int // delay in million-seconds
}

type MetricManager struct {
	tickHour  sync.Map // market*symbol*M/D_H &TickMetric
	carryHour sync.Map // market*M/D_H &CarryMetric
	//metricTicks sync.Map // market*symbol*index - []*TickDelay
	//index       sync.Map // market*symbol - index
}

func (metricManager *MetricManager) AddCarry(mark string, carryOpen, carryClose float64) {
	current := util.GetNow()
	key := fmt.Sprintf(`%s*%d/%d_%d`, mark, current.Month(), current.Day(), current.Hour())
	value, ok := metricManager.carryHour.Load(key)
	var carryMetric *CarryMetric
	if !ok {
		carryMetric = &CarryMetric{carryHighest: math.NaN(), carryLowest: math.NaN()}
	} else if value != nil {
		carryMetric = value.(*CarryMetric)
	}
	carryMetric.count++
	if carryOpen > carryMetric.carryHighest || math.IsNaN(carryMetric.carryHighest) {
		carryMetric.carryHighest = carryOpen
	}
	if carryClose < carryMetric.carryLowest || math.IsNaN(carryMetric.carryLowest) {
		carryMetric.carryLowest = carryClose
	}
	if !math.IsNaN(carryOpen) {
		carryMetric.totalHigh += carryOpen
		carryMetric.avgHigh = carryMetric.totalHigh / float64(carryMetric.count)
	}
	if !math.IsNaN(carryClose) {
		carryMetric.totalLow += carryClose
		carryMetric.avgLow = carryMetric.totalLow / float64(carryMetric.count)
	}
	metricManager.carryHour.Store(key, carryMetric)
}

func (metricManager *MetricManager) AddTick(market, symbol string, current time.Time, lastBidAsk, bidAsk *BidAsk) {
	//key := fmt.Sprintf(`%s*%s%d/%d_%d`, market, symbol, current.Month(), current.Day(), current.Hour())
	if current.Second() != 0 {
		return
	}
	key := fmt.Sprintf(`%s %s %d`, market, symbol, current.Hour())
	value, ok := metricManager.tickHour.Load(key)
	var tickMetric *TickMetric
	if !ok || value == nil {
		tickMetric = &TickMetric{priceLow: 0, priceHigh: 0, start: current}
	} else {
		tickMetric = value.(*TickMetric)
		if tickMetric.start.Add(time.Hour).Before(current) {
			tickMetric = &TickMetric{priceLow: 0, priceHigh: 0, start: current}
			//util.Notice(fmt.Sprintf(`start to add tick in new hour %s %s`, key, current.String()))
		}
	}
	now := int(current.UnixNano() / int64(time.Millisecond))
	between := 0
	if lastBidAsk != nil {
		between = bidAsk.Ts - lastBidAsk.Ts
	}
	tickMetric.betweenSum += between
	if tickMetric.betweenLow == 0 || tickMetric.betweenLow > between {
		tickMetric.betweenLow = between
	}
	if tickMetric.betweenHigh < between {
		tickMetric.betweenHigh = between
	}
	delay := now - bidAsk.Ts
	if tickMetric.delayLow == 0 || tickMetric.delayLow > delay {
		tickMetric.delayLow = delay
	}
	if tickMetric.delayHigh < delay {
		tickMetric.delayHigh = delay
	}
	if delay < 100 {
		tickMetric.countValid++
	}
	if tickMetric.priceHigh < bidAsk.Asks[0].Price {
		tickMetric.priceHigh = bidAsk.Asks[0].Price
	}
	if tickMetric.priceLow == 0 || tickMetric.priceLow > bidAsk.Bids[0].Price {
		tickMetric.priceLow = bidAsk.Bids[0].Price
	}
	tickMetric.countAll++
	tickMetric.delaySum += delay
	tickMetric.delayAvg = float64(tickMetric.delaySum) / float64(tickMetric.countAll)
	tickMetric.betweenAvg = float64(tickMetric.betweenSum) / float64(tickMetric.countAll)
	//index := 0
	//marketSymbol := fmt.Sprintf(`%s*%s`, market, symbol)
	//indexValue, ok := metricManager.index.Load(marketSymbol)
	//if ok {
	//	index = indexValue.(int)
	//}
	//tickDelay := &TickDelay{receiveTime: current, delay: delay}
	//metricManager.metricTicks.Store(fmt.Sprintf(`%s*%d`, marketSymbol, index), tickDelay)
	//metricManager.index.Store(marketSymbol, (index+1)%recentTickLength)
	if AppConfig.MetricTick {
		metricManager.tickHour.Store(key, tickMetric)
	}
}

//func (metricManager *MetricManager) ToTables() (tables [][]map[string]interface{}) {
//	defer metricManager.Lock.Unlock()
//	metricManager.Lock.Lock()
//	tablePriceDis := make([]map[string]interface{}, 0)
//	tableTick := make([]map[string]interface{}, 0)
//	tableTickRecent := make([]map[string]interface{}, 0)
//	now := util.GetNow()
//	timeMap := make(map[string]bool, 12)
//	for i := 0; i < 12; i++ {
//		duration, _ := time.ParseDuration(fmt.Sprintf(`-%dh`, i))
//		then := now.Add(duration)
//		timeMap[fmt.Sprintf(`%d/%d_%d`, then.Month(), then.Day(), then.Hour())] = true
//	}
//	metricManager.carryHour.Range(func (key, value interface{}) bool {
//
//	})
//	for marketSymbol, timeMetric := range metricManager.carryHour {
//		for str, metric := range timeMetric {
//			if timeMap[str] {
//				metricMsg := map[string]interface{}{`价差`: marketSymbol, `time`: str, `count`: metric.count,
//					`平均正开仓差`: strconv.FormatFloat(metric.avgHigh, 'f', 5, 64),
//					`平均反开仓差`: strconv.FormatFloat(metric.avgLow, 'f', 5, 64)}
//				if !math.IsNaN(metric.carryHighest) {
//					metricMsg[`最大正开仓`] = metric.carryHighest
//				}
//				if !math.IsNaN(metric.carryLowest) {
//					metricMsg[`最大反开仓`] = metric.carryLowest
//				}
//				tablePriceDis = append(tablePriceDis, metricMsg)
//			}
//		}
//	}
//	for marketSymbol, timeMetric := range metricManager.tickHour {
//		for str, metric := range timeMetric {
//			if timeMap[str] {
//				metricMsg := map[string]interface{}{`tick`: marketSymbol, `time`: str, `all`: metric.countAll,
//					`valid`: metric.countValid, `delay_low`: metric.delayLow, `delay_high`: metric.delayHigh,
//					`delay_avg`: math.Round(metric.delayAvg), `price_low`: metric.priceLow, `price_high`: metric.priceHigh}
//				tableTick = append(tableTick, metricMsg)
//			}
//		}
//	}
//	for marketSymbol, metrics := range metricManager.metricTicks {
//		index := metricManager.index[marketSymbol]
//		pre := (metricManager.index[marketSymbol] - 1 + recentTickLength) % recentTickLength
//		if metrics[index] == nil {
//			index = 0
//		}
//		tickMetric := TickMetric{start: metrics[index].receiveTime, end: metrics[pre].receiveTime}
//		for _, tick := range metrics {
//			if tick == nil {
//				continue
//			}
//			if tick.delay > tickMetric.delayHigh {
//				tickMetric.delayHigh = tick.delay
//			}
//			if tick.delay < tickMetric.delayLow || tickMetric.delayLow == 0 {
//				tickMetric.delayLow = tick.delay
//			}
//			tickMetric.delaySum += tick.delay
//			tickMetric.countAll++
//			if tick.delay < 100 {
//				tickMetric.countValid++
//			}
//		}
//		tickMetric.delayAvg = float64(tickMetric.delaySum) / float64(tickMetric.countAll)
//		metricMsg := map[string]interface{}{`最近tick`: marketSymbol, `all`: tickMetric.countAll,
//			`valid`: tickMetric.countValid, `delay_low`: tickMetric.delayLow, `delay_high`: tickMetric.delayHigh,
//			`delay_avg`: math.Round(tickMetric.delayAvg),
//			`start`:     fmt.Sprintf(`%d:%d:%d`, tickMetric.start.Hour(), tickMetric.start.Minute(), tickMetric.start.Second()),
//			`end`:       fmt.Sprintf(`%d:%d:%d`, tickMetric.end.Hour(), tickMetric.end.Minute(), tickMetric.end.Second())}
//		tableTickRecent = append(tableTickRecent, metricMsg)
//	}
//	return [][]map[string]interface{}{tablePriceDis, tableTick, tableTickRecent}
//}

func (metricManager *MetricManager) ToArray() (tickInfo [][]string) {
	//now := util.GetNow()
	//timeMap := make(map[string]bool, 12)
	//for i := 0; i < 12; i++ {
	//	duration, _ := time.ParseDuration(fmt.Sprintf(`-%dh`, i))
	//	then := now.Add(duration)
	//	timeMap[fmt.Sprintf(`%d/%d_%d`, then.Month(), then.Day(), then.Hour())] = true
	//}
	tickInfo = make([][]string, 0)
	// tick状况
	metricManager.tickHour.Range(func(key, value interface{}) bool {
		metric := value.(*TickMetric)
		tickInfo = append(tickInfo, []string{key.(string),
			fmt.Sprintf(`%d/%d`, metric.countValid, metric.countAll),
			fmt.Sprintf(`%d:%d=%.1f`, metric.betweenLow, metric.betweenHigh, metric.betweenAvg),
			fmt.Sprintf(`%d:%d=%.1f`, metric.delayLow, metric.delayHigh, metric.delayAvg),
			fmt.Sprintf(`%f:%f`, metric.priceLow, metric.priceHigh)})
		return true
	})
	//for marketSymbol, timeMetric := range metricManager.tickHour {
	//	market := marketSymbol[0:strings.Index(marketSymbol, `_`)]
	//	symbol := marketSymbol[strings.Index(marketSymbol, `_`)+1:]
	//	for str, metric := range timeMetric {
	//		if timeMap[str] {
	//			tickInfo = append(tickInfo, []string{market, symbol, str,
	//				strconv.FormatInt(int64(metric.countAll), 10),
	//				strconv.FormatInt(int64(metric.countValid), 10),
	//				fmt.Sprintf(`%d~%d:%.0f`, metric.delayLow, metric.delayHigh, metric.delayAvg),
	//				fmt.Sprintf(`%d~%d:%.0f`, metric.betweenLow, metric.betweenHigh, metric.betweenAvg)})
	//		}
	//	}
	//}
	//recentTickInfo = make([][]string, 0)
	//for marketSymbol, metrics := range metricManager.metricTicks {
	//	market := marketSymbol[0:strings.Index(marketSymbol, `_`)]
	//	symbol := marketSymbol[strings.Index(marketSymbol, `_`)+1:]
	//	index := metricManager.index[marketSymbol]
	//	pre := (metricManager.index[marketSymbol] - 1 + recentTickLength) % recentTickLength
	//	if metrics[index] == nil {
	//		index = 0
	//	}
	//	tickMetric := TickMetric{start: metrics[index].receiveTime, end: metrics[pre].receiveTime}
	//	for _, tick := range metrics {
	//		if tick == nil {
	//			continue
	//		}
	//		if tick.delay > tickMetric.delayHigh {
	//			tickMetric.delayHigh = tick.delay
	//		}
	//		if tick.delay < tickMetric.delayLow || tickMetric.delayLow == 0 {
	//			tickMetric.delayLow = tick.delay
	//		}
	//		tickMetric.delaySum += tick.delay
	//		tickMetric.countAll++
	//		if tick.delay < 100 {
	//			tickMetric.countValid++
	//		}
	//	}
	//	tickMetric.delayAvg = float64(tickMetric.delaySum) / float64(tickMetric.countAll)
	//	// 最近tick
	//	recentTickInfo = append(recentTickInfo, []string{market, symbol,
	//		fmt.Sprintf(`%d:%d-%d:%d`, tickMetric.start.Hour(), tickMetric.start.Minute(),
	//			tickMetric.end.Hour(), tickMetric.end.Minute()),
	//		strconv.FormatInt(int64(tickMetric.countAll), 10),
	//		strconv.FormatInt(int64(tickMetric.countValid), 10),
	//		fmt.Sprintf(`%d~%d:%.0f`, tickMetric.delayLow, tickMetric.delayHigh, tickMetric.delayAvg)})
	//}
	return
}

func (metricManager *MetricManager) ToString() (metricStr string) {
	metricStr = ``
	now := util.GetNow()
	timeMap := make(map[string]bool, 12)
	for i := 0; i < 12; i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf(`-%dh`, i))
		then := now.Add(duration)
		timeMap[fmt.Sprintf(`%d/%d_%d`, then.Month(), then.Day(), then.Hour())] = true
	}
	metricManager.carryHour.Range(func(key, value interface{}) bool {
		metric := value.(*CarryMetric)
		metricStr += fmt.Sprintf("%s: all:%d lowest: %f highest: %f avgHigh: %f avgLow: %f\n",
			key, metric.count, metric.carryLowest, metric.carryHighest, metric.avgHigh, metric.avgLow)
		return true
	})
	metricManager.tickHour.Range(func(key, value interface{}) bool {
		metric := value.(*TickMetric)
		metricStr += fmt.Sprintf(
			"[%s tick状况]\n: all:%d <100:%d delay:[%d-%d %f] price[%f-%f] 间隔[%d-%d %f]\n",
			key.(string), metric.countAll, metric.countValid, metric.delayLow, metric.delayHigh, metric.delayAvg,
			metric.priceLow, metric.priceHigh, metric.betweenLow, metric.betweenHigh, metric.betweenAvg)
		return true
	})
	return
}

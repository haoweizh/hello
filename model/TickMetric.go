package model

import (
	"fmt"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const recentTickLength = 100

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
	delay       int // delay in million seconds
}

type MetricManager struct {
	Lock        sync.Mutex
	tickHour    map[string]map[string]*TickMetric  // market_symbol - MMDDHH - tickMetric
	carryHour   map[string]map[string]*CarryMetric // market_symbol - MMDDHH - carryMetric
	metricTicks map[string][]*TickDelay            // market_symbol - []TickDelay
	index       map[string]int                     // market_symbol - index
}

func (metricManager *MetricManager) AddCarry(mark string, carryOpen, carryClose float64) {
	defer metricManager.Lock.Unlock()
	metricManager.Lock.Lock()
	if metricManager.carryHour == nil {
		metricManager.carryHour = make(map[string]map[string]*CarryMetric)
	}
	if metricManager.carryHour[mark] == nil {
		metricManager.carryHour[mark] = make(map[string]*CarryMetric)
	}
	current := util.GetNow()
	timeStr := fmt.Sprintf(`%d/%d_%d`, current.Month(), current.Day(), current.Hour())
	if metricManager.carryHour[mark][timeStr] == nil {
		metricManager.carryHour[mark][timeStr] = &CarryMetric{carryHighest: math.NaN(), carryLowest: math.NaN()}
	}
	carryMetric := metricManager.carryHour[mark][timeStr]
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
}

func (metricManager *MetricManager) AddTick(market, symbol string, current time.Time, lastBidAsk, bidAsk *BidAsk) {
	defer metricManager.Lock.Unlock()
	metricManager.Lock.Lock()
	marketSymbol := fmt.Sprintf(`%s_%s`, market, symbol)
	if metricManager.tickHour == nil {
		metricManager.tickHour = make(map[string]map[string]*TickMetric)
	}
	if metricManager.tickHour[marketSymbol] == nil {
		metricManager.tickHour[marketSymbol] = make(map[string]*TickMetric)
	}
	timeStr := fmt.Sprintf(`%d/%d_%d`, current.Month(), current.Day(), current.Hour())
	if metricManager.tickHour[marketSymbol][timeStr] == nil {
		metricManager.tickHour[marketSymbol][timeStr] = &TickMetric{priceLow: 0, priceHigh: 0}
	}
	now := int(current.UnixNano() / int64(time.Millisecond))
	tickMetric := metricManager.tickHour[marketSymbol][timeStr]
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
	if metricManager.metricTicks == nil || metricManager.index == nil {
		metricManager.metricTicks = make(map[string][]*TickDelay)
		metricManager.index = make(map[string]int)
	}
	if metricManager.metricTicks[marketSymbol] == nil {
		metricManager.metricTicks[marketSymbol] = make([]*TickDelay, recentTickLength)
		metricManager.index[marketSymbol] = 0
	}
	tickDelay := &TickDelay{receiveTime: current, delay: delay}
	metricManager.metricTicks[marketSymbol][metricManager.index[marketSymbol]] = tickDelay
	metricManager.index[marketSymbol] = (metricManager.index[marketSymbol] + 1) % recentTickLength
}

func (metricManager *MetricManager) ToTables() (tables [][]map[string]interface{}) {
	defer metricManager.Lock.Unlock()
	metricManager.Lock.Lock()
	tablePriceDis := make([]map[string]interface{}, 0)
	tableTick := make([]map[string]interface{}, 0)
	tableTickRecent := make([]map[string]interface{}, 0)
	now := util.GetNow()
	timeMap := make(map[string]bool, 12)
	for i := 0; i < 12; i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf(`-%dh`, i))
		then := now.Add(duration)
		timeMap[fmt.Sprintf(`%d/%d_%d`, then.Month(), then.Day(), then.Hour())] = true
	}
	for marketSymbol, timeMetric := range metricManager.carryHour {
		for str, metric := range timeMetric {
			if timeMap[str] {
				metricMsg := map[string]interface{}{`价差`: marketSymbol, `time`: str, `count`: metric.count,
					`平均正开仓差`: strconv.FormatFloat(metric.avgHigh, 'f', 5, 64),
					`平均反开仓差`: strconv.FormatFloat(metric.avgLow, 'f', 5, 64)}
				if !math.IsNaN(metric.carryHighest) {
					metricMsg[`最大正开仓`] = metric.carryHighest
				}
				if !math.IsNaN(metric.carryLowest) {
					metricMsg[`最大反开仓`] = metric.carryLowest
				}
				tablePriceDis = append(tablePriceDis, metricMsg)
			}
		}
	}
	for marketSymbol, timeMetric := range metricManager.tickHour {
		for str, metric := range timeMetric {
			if timeMap[str] {
				metricMsg := map[string]interface{}{`tick`: marketSymbol, `time`: str, `all`: metric.countAll,
					`valid`: metric.countValid, `delay_low`: metric.delayLow, `delay_high`: metric.delayHigh,
					`delay_avg`: math.Round(metric.delayAvg), `price_low`: metric.priceLow, `price_high`: metric.priceHigh}
				tableTick = append(tableTick, metricMsg)
			}
		}
	}
	for marketSymbol, metrics := range metricManager.metricTicks {
		index := metricManager.index[marketSymbol]
		pre := (metricManager.index[marketSymbol] - 1 + recentTickLength) % recentTickLength
		if metrics[index] == nil {
			index = 0
		}
		tickMetric := TickMetric{start: metrics[index].receiveTime, end: metrics[pre].receiveTime}
		for _, tick := range metrics {
			if tick == nil {
				continue
			}
			if tick.delay > tickMetric.delayHigh {
				tickMetric.delayHigh = tick.delay
			}
			if tick.delay < tickMetric.delayLow || tickMetric.delayLow == 0 {
				tickMetric.delayLow = tick.delay
			}
			tickMetric.delaySum += tick.delay
			tickMetric.countAll++
			if tick.delay < 100 {
				tickMetric.countValid++
			}
		}
		tickMetric.delayAvg = float64(tickMetric.delaySum) / float64(tickMetric.countAll)
		metricMsg := map[string]interface{}{`最近tick`: marketSymbol, `all`: tickMetric.countAll,
			`valid`: tickMetric.countValid, `delay_low`: tickMetric.delayLow, `delay_high`: tickMetric.delayHigh,
			`delay_avg`: math.Round(tickMetric.delayAvg),
			`start`:     fmt.Sprintf(`%d:%d:%d`, tickMetric.start.Hour(), tickMetric.start.Minute(), tickMetric.start.Second()),
			`end`:       fmt.Sprintf(`%d:%d:%d`, tickMetric.end.Hour(), tickMetric.end.Minute(), tickMetric.end.Second())}
		tableTickRecent = append(tableTickRecent, metricMsg)
	}
	return [][]map[string]interface{}{tablePriceDis, tableTick, tableTickRecent}
}

func (metricManager *MetricManager) ToArray() (tickInfo, recentTickInfo [][]string) {
	defer metricManager.Lock.Unlock()
	metricManager.Lock.Lock()
	now := util.GetNow()
	timeMap := make(map[string]bool, 12)
	for i := 0; i < 12; i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf(`-%dh`, i))
		then := now.Add(duration)
		timeMap[fmt.Sprintf(`%d/%d_%d`, then.Month(), then.Day(), then.Hour())] = true
	}
	tickInfo = make([][]string, 0)
	// tick状况
	for marketSymbol, timeMetric := range metricManager.tickHour {
		market := marketSymbol[0:strings.Index(marketSymbol, `_`)]
		symbol := marketSymbol[strings.Index(marketSymbol, `_`)+1:]
		for str, metric := range timeMetric {
			if timeMap[str] {
				tickInfo = append(tickInfo, []string{market, symbol, str,
					strconv.FormatInt(int64(metric.countAll), 10),
					strconv.FormatInt(int64(metric.countValid), 10),
					fmt.Sprintf(`%d~%d:%.0f`, metric.delayLow, metric.delayHigh, metric.delayAvg),
					fmt.Sprintf(`%d~%d:%.0f`, metric.betweenLow, metric.betweenHigh, metric.betweenAvg)})
			}
		}
	}
	recentTickInfo = make([][]string, 0)
	for marketSymbol, metrics := range metricManager.metricTicks {
		market := marketSymbol[0:strings.Index(marketSymbol, `_`)]
		symbol := marketSymbol[strings.Index(marketSymbol, `_`)+1:]
		index := metricManager.index[marketSymbol]
		pre := (metricManager.index[marketSymbol] - 1 + recentTickLength) % recentTickLength
		if metrics[index] == nil {
			index = 0
		}
		tickMetric := TickMetric{start: metrics[index].receiveTime, end: metrics[pre].receiveTime}
		for _, tick := range metrics {
			if tick == nil {
				continue
			}
			if tick.delay > tickMetric.delayHigh {
				tickMetric.delayHigh = tick.delay
			}
			if tick.delay < tickMetric.delayLow || tickMetric.delayLow == 0 {
				tickMetric.delayLow = tick.delay
			}
			tickMetric.delaySum += tick.delay
			tickMetric.countAll++
			if tick.delay < 100 {
				tickMetric.countValid++
			}
		}
		tickMetric.delayAvg = float64(tickMetric.delaySum) / float64(tickMetric.countAll)
		// 最近tick
		recentTickInfo = append(recentTickInfo, []string{market, symbol,
			fmt.Sprintf(`%d:%d-%d:%d`, tickMetric.start.Hour(), tickMetric.start.Minute(),
				tickMetric.end.Hour(), tickMetric.end.Minute()),
			strconv.FormatInt(int64(tickMetric.countAll), 10),
			strconv.FormatInt(int64(tickMetric.countValid), 10),
			fmt.Sprintf(`%d~%d:%.0f`, tickMetric.delayLow, tickMetric.delayHigh, tickMetric.delayAvg)})
	}
	return
}

func (metricManager *MetricManager) ToString() (metricStr string) {
	defer metricManager.Lock.Unlock()
	metricManager.Lock.Lock()
	metricStr = ``
	now := util.GetNow()
	timeMap := make(map[string]bool, 12)
	for i := 0; i < 12; i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf(`-%dh`, i))
		then := now.Add(duration)
		timeMap[fmt.Sprintf(`%d/%d_%d`, then.Month(), then.Day(), then.Hour())] = true
	}
	for marketSymbol, timeMetric := range metricManager.carryHour {
		metricStr = metricStr + fmt.Sprintf("[%s 价差状况]\n", marketSymbol)
		for str, metric := range timeMetric {
			if timeMap[str] {
				metricStr += fmt.Sprintf("%s: all:%d lowest: %f highest: %f avgHigh: %f avgLow: %f\n",
					str, metric.count, metric.carryLowest, metric.carryHighest, metric.avgHigh, metric.avgLow)
			}
		}
	}
	for marketSymbol, timeMetric := range metricManager.tickHour {
		metricStr = metricStr + fmt.Sprintf("[%s tick状况]\n", marketSymbol)
		for str, metric := range timeMetric {
			if timeMap[str] {
				metricStr += fmt.Sprintf("%s: all:%d <100:%d delay:[%d-%d %f] price[%f-%f] 间隔[%d-%d %f]\n",
					str, metric.countAll, metric.countValid, metric.delayLow, metric.delayHigh, metric.delayAvg,
					metric.priceLow, metric.priceHigh, metric.betweenLow, metric.betweenHigh, metric.betweenAvg)
			}
		}
	}
	for marketSymbol, metrics := range metricManager.metricTicks {
		index := metricManager.index[marketSymbol]
		pre := (metricManager.index[marketSymbol] - 1 + recentTickLength) % recentTickLength
		if metrics[index] == nil {
			index = 0
		}
		tickMetric := TickMetric{start: metrics[index].receiveTime, end: metrics[pre].receiveTime}
		for _, tick := range metrics {
			if tick == nil {
				continue
			}
			if tick.delay > tickMetric.delayHigh {
				tickMetric.delayHigh = tick.delay
			}
			if tick.delay < tickMetric.delayLow || tickMetric.delayLow == 0 {
				tickMetric.delayLow = tick.delay
			}
			tickMetric.delaySum += tick.delay
			tickMetric.countAll++
			if tick.delay < 100 {
				tickMetric.countValid++
			}
		}
		tickMetric.delayAvg = float64(tickMetric.delaySum) / float64(tickMetric.countAll)
		//if float64(tickMetric.countValid)/float64(tickMetric.countAll) < 0.2 || tickMetric.delayAvg > 100 {
		//}
		metricStr = metricStr + fmt.Sprintf("[最近tick %s][%d:%d:%d-%d:%d:%d]all:%d <100:%d delay: %d-%d avg: %f\n",
			marketSymbol, tickMetric.start.Hour(), tickMetric.start.Minute(), tickMetric.start.Second(),
			tickMetric.end.Hour(), tickMetric.end.Minute(), tickMetric.end.Second(), tickMetric.countAll,
			tickMetric.countValid, tickMetric.delayLow, tickMetric.delayHigh, tickMetric.delayAvg)
	}
	return
}

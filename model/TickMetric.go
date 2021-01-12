package model

import (
	"fmt"
	"hello/util"
	"sync"
	"time"
)

const recentTickLength = 10000

type TickMetric struct {
	delayLow   int
	delayHigh  int
	delayAvg   float64
	delaySum   int
	priceLow   float64
	priceHigh  float64
	countValid int
	countAll   int
	start      time.Time
	end        time.Time
}

type TickDelay struct {
	receiveTime time.Time
	delay       int // delay in million seconds
}

type MetricManager struct {
	Lock        sync.Mutex
	metricHour  map[string]map[string]*TickMetric // market_symbol - MMDDHH - tickMetric
	metricTicks map[string][]*TickDelay           // market_symbol - []TickDelay
	index       map[string]int                    // market_symbol - index
}

func (metricManager *MetricManager) addTick(market, symbol string, current time.Time, bidAsk *BidAsk) {
	defer metricManager.Lock.Unlock()
	metricManager.Lock.Lock()
	marketSymbol := fmt.Sprintf(`%s_%s`, market, symbol)
	if metricManager.metricHour == nil {
		metricManager.metricHour = make(map[string]map[string]*TickMetric)
	}
	if metricManager.metricHour[marketSymbol] == nil {
		metricManager.metricHour[marketSymbol] = make(map[string]*TickMetric)
	}
	timeStr := fmt.Sprintf(`%d/%d %d`, current.Month(), current.Day(), current.Hour())
	if metricManager.metricHour[marketSymbol][timeStr] == nil {
		metricManager.metricHour[marketSymbol][timeStr] = &TickMetric{priceLow: 0, priceHigh: 0}
	}
	tickMetric := metricManager.metricHour[marketSymbol][timeStr]
	delay := int(current.UnixNano()/1000000) - bidAsk.Ts
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

func (metricManager *MetricManager) ToString() (metricStr string) {
	metricStr = ``
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
		metricStr = metricStr + fmt.Sprintf("[最近tick %s][%d:%d:%d-%d:%d:%d]all:%d <100:%d delay: %d-%d avg: %f\n",
			marketSymbol, tickMetric.start.Hour(), tickMetric.start.Minute(), tickMetric.start.Second(), tickMetric.end.Hour(),
			tickMetric.end.Minute(), tickMetric.end.Second(), tickMetric.countAll, tickMetric.countValid, tickMetric.delayLow,
			tickMetric.delayHigh, tickMetric.delayAvg)
	}
	now := util.GetNow()
	timeMap := make(map[string]bool, 12)
	for i := 0; i < 12; i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf(`-%dh`, i))
		then := now.Add(duration)
		timeMap[fmt.Sprintf(`%d/%d %d`, then.Month(), then.Day(), then.Hour())] = true
	}
	for marketSymbol, timeMetric := range metricManager.metricHour {
		metricStr = metricStr + fmt.Sprintf("[%s tick状况]\n", marketSymbol)
		for str, metric := range timeMetric {
			if timeMap[str] {
				metricStr += fmt.Sprintf("%s: all:%d <100:%d delay: %d-%d avg: %f tick low-high %f %f\n",
					str, metric.countAll, metric.countValid, metric.delayLow, metric.delayHigh, metric.delayAvg,
					metric.priceLow, metric.priceHigh)
			}
		}
	}
	return
}

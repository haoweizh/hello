package monitor

import (
	"encoding/json"
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"sync"
	"time"
)

var DataMonitor = &sync.Map{}

type AggregationCandle struct {
	slideRing                                            *SlideRing
	Start, End                                           *time.Time
	TimeInterval                                         time.Duration
	PriceHigh, PriceLow, Volume                          float64
	PriceStart, PriceCurrent, PriceIncrease, PriceChange float64
}

func (aggregationCandle *AggregationCandle) refresh() {
	for {
		tempStart, tempCurrent := aggregationCandle.slideRing.get()
		if tempStart == nil {
			if !aggregationCandle.slideRing.remove() {
				break
			}
		} else if tempCurrent != nil {
			start := tempStart.(*model.Candle)
			current := tempCurrent.(*model.Candle)
			if start.Begin.Add(aggregationCandle.TimeInterval).Before(current.Begin) {
				aggregationCandle.slideRing.remove()
			}
		}
	}
	tempStart, tempCurrent := aggregationCandle.slideRing.get()
	if tempStart == nil || tempCurrent == nil {
		return
	}
	aggregationCandle.Start = &tempStart.(*model.Candle).Begin
	aggregationCandle.End = &tempCurrent.(*model.Candle).Begin
	aggregationCandle.PriceStart = tempStart.(*model.Candle).PriceOpen
	aggregationCandle.PriceCurrent = tempCurrent.(*model.Candle).PriceClose
	aggregationCandle.Volume = 0
	for i := aggregationCandle.slideRing.start; i < aggregationCandle.slideRing.current; i++ {
		candleIndex := aggregationCandle.slideRing.data[i]
		if candleIndex != nil {
			aggregationCandle.Volume += candleIndex.(*model.Candle).VolumeQuote
			if aggregationCandle.PriceHigh < candleIndex.(*model.Candle).PriceHigh {
				aggregationCandle.PriceHigh = candleIndex.(*model.Candle).PriceHigh
			}
			if aggregationCandle.PriceLow > candleIndex.(*model.Candle).PriceLow || aggregationCandle.PriceLow == 0 {
				aggregationCandle.PriceLow = candleIndex.(*model.Candle).PriceLow
			}
		}
	}
	aggregationCandle.PriceChange = aggregationCandle.PriceHigh - aggregationCandle.PriceLow
	aggregationCandle.PriceIncrease = aggregationCandle.PriceCurrent - aggregationCandle.PriceStart
}

func (aggregationCandle *AggregationCandle) handle(candle *model.Candle) {
	if (*aggregationCandle).End == nil {
		aggregationCandle.End = &candle.Begin
	}
	if candle.Begin.Before(*aggregationCandle.End) {
		util.Info(fmt.Sprintf(`ignore passed by %s %s %s<%s`,
			candle.Market, candle.Symbol, candle.Begin.String(), aggregationCandle.End))
		return
	}
	aggregationCandle.slideRing.add(candle)
	if rand.Float64() > 0.999 {
		aggregationCandle.refresh()
		return
	}
	aggregationCandle.End = &candle.Begin
	aggregationCandle.PriceCurrent = candle.PriceClose
	aggregationCandle.Volume += candle.VolumeQuote
	for {
		tempStart, _ := aggregationCandle.slideRing.get()
		if tempStart == nil {
			if !aggregationCandle.slideRing.remove() {
				break
			}
		} else if tempStart.(*model.Candle).Begin.Add(aggregationCandle.TimeInterval).Before(*aggregationCandle.End) {
			aggregationCandle.Volume -= tempStart.(*model.Candle).VolumeQuote
			if aggregationCandle.PriceHigh == candle.PriceHigh {
				aggregationCandle.PriceHigh = 0
			}
			if aggregationCandle.PriceLow == candle.PriceLow {
				aggregationCandle.PriceLow = 0
			}
			aggregationCandle.slideRing.remove()
		} else {
			break
		}
	}
	tempStart, tempCurrent := aggregationCandle.slideRing.get()
	if tempStart == nil || tempCurrent == nil {
		aggregationCandle.Volume = candle.VolumeQuote
		aggregationCandle.Start = &candle.Begin
		aggregationCandle.PriceStart = candle.PriceHigh
		aggregationCandle.PriceHigh = candle.PriceHigh
		aggregationCandle.PriceLow = candle.PriceLow
		aggregationCandle.PriceIncrease = 0
		aggregationCandle.PriceChange = 0
		util.Info(fmt.Sprintf(`reset from slide ring with candle %s %s %s`, candle.Market, candle.Symbol, candle.Begin.String()))
		return
	}
	aggregationCandle.Start = &tempStart.(*model.Candle).Begin
	aggregationCandle.PriceStart = tempStart.(*model.Candle).PriceOpen
	if aggregationCandle.PriceHigh == 0 {
		aggregationCandle.PriceHigh = math.Max(tempStart.(*model.Candle).PriceHigh, candle.PriceHigh)
	} else if aggregationCandle.PriceHigh < candle.PriceHigh {
		aggregationCandle.PriceHigh = candle.PriceHigh
	}
	if aggregationCandle.PriceLow == 0 {
		aggregationCandle.PriceLow = math.Min(tempStart.(*model.Candle).PriceLow, candle.PriceLow)
	} else if aggregationCandle.PriceLow > candle.PriceLow {
		aggregationCandle.PriceLow = candle.PriceLow
	}
	aggregationCandle.PriceChange = aggregationCandle.PriceHigh - aggregationCandle.PriceLow
	aggregationCandle.PriceIncrease = aggregationCandle.PriceCurrent - aggregationCandle.PriceStart
}

func KLineServer() {
	markets := api.GetMarkets()
	for _, market := range markets {
		settings := api.GetSettings(model.FunctionKLine, market)
		if settings == nil {
			continue
		}
		settings.Range(func(symbol, setting any) bool {
			util.StoreSyncMap(DataMonitor, &AggregationCandle{
				TimeInterval: time.Duration(setting.(*model.Setting).Seconds) * time.Second, slideRing: &SlideRing{}},
				market, symbol.(string))
			return true
		})
	}
	for {
		candle := <-model.KLineChan
		aggregationCandle, success := util.LoadSyncMap(DataMonitor, candle.Market, candle.Symbol)
		if success && aggregationCandle != nil {
			aggregationCandle.(*AggregationCandle).handle(candle)
			jsonBytes, err := json.Marshal(aggregationCandle)
			if err == nil {
				fmt.Println(string(jsonBytes))
				api.AppWSManager.Send(jsonBytes, nil)
			}
		}
	}
}

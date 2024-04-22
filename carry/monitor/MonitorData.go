package monitor

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"sync"
	"time"
)

var DataMonitor = &sync.Map{}

type AggregationCandle struct {
	slideRing                                            *SlideRing
	timeStart, timeEnd                                   *time.Time
	timeInterval                                         time.Duration
	priceHigh, priceLow                                  float64
	priceStart, priceCurrent, priceIncrease, priceChange float64
	volume                                               float64
}

func (aggregationCandle *AggregationCandle) handle(candle *model.Candle) {
	if candle.Begin.Before(*aggregationCandle.timeEnd) {
		util.Info(fmt.Sprintf(`ignore passed by %s %s %s<%s`,
			candle.Market, candle.Symbol, candle.Begin.String(), aggregationCandle.timeEnd))
		return
	}
	aggregationCandle.slideRing.add(candle)
	aggregationCandle.timeEnd = &candle.Begin
	aggregationCandle.priceCurrent = candle.PriceClose
	aggregationCandle.volume += candle.Volume
	for {
		tempStart, _ := aggregationCandle.slideRing.get()
		if tempStart == nil {
			if !aggregationCandle.slideRing.remove() {
				break
			}
		} else if tempStart.(*model.Candle).Begin.Add(aggregationCandle.timeInterval).Before(*aggregationCandle.timeEnd) {
			aggregationCandle.volume -= tempStart.(*model.Candle).Volume
			if aggregationCandle.priceHigh == candle.PriceHigh {
				aggregationCandle.priceHigh = 0
			}
			if aggregationCandle.priceLow == candle.PriceLow {
				aggregationCandle.priceLow = 0
			}
		} else {
			break
		}
	}
	tempStart, tempCurrent := aggregationCandle.slideRing.get()
	if tempStart == nil || tempCurrent == nil {
		aggregationCandle.volume = candle.Volume
		aggregationCandle.timeStart = &candle.Begin
		aggregationCandle.priceStart = candle.PriceHigh
		aggregationCandle.priceHigh = candle.PriceHigh
		aggregationCandle.priceLow = candle.PriceLow
		aggregationCandle.priceIncrease = 0
		aggregationCandle.priceChange = 0
		util.Info(fmt.Sprintf(`reset from slide ring with candle %s %s %s`, candle.Market, candle.Symbol, candle.Begin.String()))
		return
	}
	aggregationCandle.timeStart = &tempStart.(*model.Candle).Begin
	aggregationCandle.priceStart = tempStart.(*model.Candle).PriceOpen
	if aggregationCandle.priceHigh == 0 {
		aggregationCandle.priceHigh = math.Max(tempStart.(*model.Candle).PriceHigh, candle.PriceHigh)
	} else if aggregationCandle.priceHigh < candle.PriceHigh {
		aggregationCandle.priceHigh = candle.PriceHigh
	}
	if aggregationCandle.priceLow == 0 {
		aggregationCandle.priceLow = math.Min(tempStart.(*model.Candle).PriceLow, candle.PriceLow)
	} else if aggregationCandle.priceLow > candle.PriceLow {
		aggregationCandle.priceLow = candle.PriceLow
	}
	aggregationCandle.priceChange = aggregationCandle.priceHigh - aggregationCandle.priceLow
	aggregationCandle.priceIncrease = aggregationCandle.priceCurrent - aggregationCandle.priceStart
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
				timeInterval: time.Duration(setting.(*model.Setting).Seconds) * time.Second, slideRing: &SlideRing{}},
				market, symbol.(string))
			return true
		})
	}
	for {
		candle := <-model.KLineChan
		aggregationCandle, success := util.LoadSyncMap(DataMonitor, candle.Market, candle.Symbol)
		if success && aggregationCandle != nil {
			aggregationCandle.(*AggregationCandle).handle(candle)
		}
	}
}

package model

import (
	"time"
)

type AggregateCandle struct {
	SlideRing                                            *SlideRing
	Start, End                                           *time.Time
	TimeInterval                                         time.Duration
	PriceHigh, PriceLow, Volume                          float64
	PriceStart, PriceCurrent, PriceIncrease, PriceChange float64
}

func (aggregationCandle *AggregateCandle) refresh() {
	for {
		tempStart, tempCurrent := aggregationCandle.SlideRing.get()
		if tempStart == nil {
			if !aggregationCandle.SlideRing.remove() {
				break
			}
		} else if tempCurrent != nil {
			start := tempStart.(*Candle)
			current := tempCurrent.(*Candle)
			if start != nil && current != nil && start.Begin.Add(aggregationCandle.TimeInterval).Before(current.Begin) {
				aggregationCandle.SlideRing.remove()
			}
		}
	}
	tempStart, tempCurrent := aggregationCandle.SlideRing.get()
	if tempStart == nil || tempCurrent == nil {
		return
	}
	aggregationCandle.Start = &tempStart.(*Candle).Begin
	aggregationCandle.End = &tempCurrent.(*Candle).Begin
	aggregationCandle.PriceStart = tempStart.(*Candle).PriceOpen
	aggregationCandle.PriceCurrent = tempCurrent.(*Candle).PriceClose
	aggregationCandle.Volume = 0
	for i := aggregationCandle.SlideRing.start; i < aggregationCandle.SlideRing.current; i++ {
		candleIndex := aggregationCandle.SlideRing.data[i]
		if candleIndex != nil {
			aggregationCandle.Volume += candleIndex.(*Candle).VolumeQuote
			if aggregationCandle.PriceHigh < candleIndex.(*Candle).PriceHigh {
				aggregationCandle.PriceHigh = candleIndex.(*Candle).PriceHigh
			}
			if aggregationCandle.PriceLow > candleIndex.(*Candle).PriceLow || aggregationCandle.PriceLow == 0 {
				aggregationCandle.PriceLow = candleIndex.(*Candle).PriceLow
			}
		}
	}
	aggregationCandle.PriceChange = aggregationCandle.PriceHigh - aggregationCandle.PriceLow
	aggregationCandle.PriceIncrease = aggregationCandle.PriceCurrent - aggregationCandle.PriceStart
}

func (aggregationCandle *AggregateCandle) Handle(candle *Candle) {
	//if aggregationCandle.End == nil {
	//	aggregationCandle.End = &candle.Begin
	//}
	//if candle.Begin.Before(*aggregationCandle.End) {
	//	util.Notice(fmt.Sprintf(`ignore passed by %s %s %s<%s`,
	//		candle.Market, candle.Symbol, candle.Begin.String(), aggregationCandle.End))
	//	return
	//}
	//aggregationCandle.SlideRing.add(candle)
	//if rand.Float64() > 0.999 {
	//	aggregationCandle.refresh()
	//	return
	//}
	//aggregationCandle.End = &candle.Begin
	//aggregationCandle.PriceCurrent = candle.PriceClose
	//aggregationCandle.Volume += candle.VolumeQuote
	//for {
	//	tempStart, _ := aggregationCandle.SlideRing.get()
	//	if tempStart == nil {
	//		if !aggregationCandle.SlideRing.remove() {
	//			break
	//		}
	//	} else if tempStart.(*Candle).Begin.Add(aggregationCandle.TimeInterval).Before(*aggregationCandle.End) {
	//		aggregationCandle.Volume -= tempStart.(*Candle).VolumeQuote
	//		if aggregationCandle.PriceHigh == candle.PriceHigh {
	//			aggregationCandle.PriceHigh = 0
	//		}
	//		if aggregationCandle.PriceLow == candle.PriceLow {
	//			aggregationCandle.PriceLow = 0
	//		}
	//		aggregationCandle.SlideRing.remove()
	//	} else {
	//		break
	//	}
	//}
	//tempStart, tempCurrent := aggregationCandle.SlideRing.get()
	//if tempStart == nil || tempCurrent == nil {
	//	aggregationCandle.Volume = candle.VolumeQuote
	//	aggregationCandle.Start = &candle.Begin
	//	aggregationCandle.PriceStart = candle.PriceHigh
	//	aggregationCandle.PriceHigh = candle.PriceHigh
	//	aggregationCandle.PriceLow = candle.PriceLow
	//	aggregationCandle.PriceIncrease = 0
	//	aggregationCandle.PriceChange = 0
	//	util.Info(fmt.Sprintf(`reset from slide ring with candle %s %s %s`, candle.Market, candle.Symbol, candle.Begin.String()))
	//	return
	//}
	//aggregationCandle.Start = &tempStart.(*Candle).Begin
	//aggregationCandle.PriceStart = tempStart.(*Candle).PriceOpen
	//if aggregationCandle.PriceHigh == 0 {
	//	aggregationCandle.PriceHigh = math.Max(tempStart.(*Candle).PriceHigh, candle.PriceHigh)
	//} else if aggregationCandle.PriceHigh < candle.PriceHigh {
	//	aggregationCandle.PriceHigh = candle.PriceHigh
	//}
	//if aggregationCandle.PriceLow == 0 {
	//	aggregationCandle.PriceLow = math.Min(tempStart.(*Candle).PriceLow, candle.PriceLow)
	//} else if aggregationCandle.PriceLow > candle.PriceLow {
	//	aggregationCandle.PriceLow = candle.PriceLow
	//}
	//aggregationCandle.PriceChange = aggregationCandle.PriceHigh - aggregationCandle.PriceLow
	//aggregationCandle.PriceIncrease = aggregationCandle.PriceCurrent - aggregationCandle.PriceStart
}

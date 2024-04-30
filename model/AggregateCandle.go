package model

import (
	"fmt"
	"hello/util"
	"time"
)

type AggregateCandle struct {
	LinkList                                             *util.LinkList
	Start, End                                           *time.Time
	TimeInterval                                         time.Duration
	PriceHigh, PriceLow, VolumeQuote                     float64
	PriceStart, PriceCurrent, PriceIncrease, PriceChange float64
}

func (aggregationCandle *AggregateCandle) refresh() {
	for {
		nodeHead := aggregationCandle.LinkList.Head
		nodeTail := aggregationCandle.LinkList.Tail
		if nodeHead == nil || nodeTail == nil {
			break
		} else {
			if nodeHead.Data.(*Candle).Begin.Add(aggregationCandle.TimeInterval).Before(nodeTail.Data.(*Candle).Begin) {
				aggregationCandle.LinkList.RemoveHead()
			}
		}
	}
	node := aggregationCandle.LinkList.Head
	if node == nil {
		return
	}
	aggregationCandle.Start = &node.Data.(*Candle).Begin
	aggregationCandle.End = &node.Data.(*Candle).Begin
	aggregationCandle.PriceStart = node.Data.(*Candle).PriceOpen
	aggregationCandle.PriceCurrent = node.Data.(*Candle).PriceClose
	aggregationCandle.PriceHigh = node.Data.(*Candle).PriceHigh
	aggregationCandle.PriceLow = node.Data.(*Candle).PriceLow
	aggregationCandle.VolumeQuote = 0
	for {
		if node == nil {
			break
		} else {
			aggregationCandle.VolumeQuote += node.Data.(*Candle).VolumeQuote
			if aggregationCandle.PriceHigh < node.Data.(*Candle).PriceHigh {
				aggregationCandle.PriceHigh = node.Data.(*Candle).PriceHigh
			}
			if aggregationCandle.PriceLow > node.Data.(*Candle).PriceLow || aggregationCandle.PriceLow == 0 {
				aggregationCandle.PriceLow = node.Data.(*Candle).PriceLow
			}
			node = node.Next
		}
	}
}

func (aggregationCandle *AggregateCandle) Handle(candle *Candle) {
	if candle.Begin.Add(aggregationCandle.TimeInterval * time.Second).Before(time.Now()) {
		util.Notice(fmt.Sprintf(`ignore passed by %s %s %s`,
			candle.Market, candle.Symbol, candle.Begin.String()))
		return
	}
	nodeCurrent := aggregationCandle.LinkList.Tail
	for {
		if nodeCurrent == nil {
			aggregationCandle.LinkList.AddHeadData(candle)
			break
		} else if nodeCurrent.Data.(*Candle).Begin.Before(candle.Begin) {
			aggregationCandle.LinkList.Insert(nodeCurrent, &util.Node{Data: candle})
			break
		} else {
			util.Notice(fmt.Sprintf(`wrong candle sequence %s %s %s > %s`,
				candle.Market, candle.Symbol, nodeCurrent.Data.(*Candle).Begin.String(), candle.Begin.String()))
			nodeCurrent = nodeCurrent.Prev
		}
	}
	aggregationCandle.End = &candle.Begin
	aggregationCandle.PriceCurrent = candle.PriceClose
	aggregationCandle.VolumeQuote += candle.VolumeQuote
	needRefresh := false
	for {
		head := aggregationCandle.LinkList.Head
		if head == nil {
			break
		} else if head.Data.(*Candle).Begin.Add(aggregationCandle.TimeInterval).Before(*aggregationCandle.End) {
			aggregationCandle.VolumeQuote -= head.Data.(*Candle).VolumeQuote
			if aggregationCandle.PriceHigh == candle.PriceHigh {
				needRefresh = true
			}
			if aggregationCandle.PriceLow == candle.PriceLow {
				needRefresh = true
			}
			aggregationCandle.LinkList.RemoveHead()
		} else {
			break
		}
	}
	headData := aggregationCandle.LinkList.Head.Data.(*Candle)
	aggregationCandle.Start = &headData.Begin
	aggregationCandle.PriceStart = headData.PriceOpen
	if needRefresh == true {
		util.Notice(fmt.Sprintf(`refresh link list %s %s seconds %s len %d`,
			candle.Market, candle.Symbol, aggregationCandle.TimeInterval.String(), aggregationCandle.LinkList.Len))
		aggregationCandle.refresh()
	} else {
		if aggregationCandle.PriceHigh < candle.PriceHigh {
			aggregationCandle.PriceHigh = candle.PriceHigh
		}
		if aggregationCandle.PriceLow == 0 || aggregationCandle.PriceLow > candle.PriceLow {
			aggregationCandle.PriceLow = candle.PriceLow
		}
	}
	aggregationCandle.PriceChange = (aggregationCandle.PriceHigh - aggregationCandle.PriceLow) / aggregationCandle.PriceCurrent
	aggregationCandle.PriceIncrease = (aggregationCandle.PriceCurrent - aggregationCandle.PriceStart) / aggregationCandle.PriceCurrent
}

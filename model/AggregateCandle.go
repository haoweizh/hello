package model

import (
	"fmt"
	"time"
)

type AggregateCandle struct {
	Market, Symbol                                       string
	Start, End                                           *time.Time
	TimeInterval                                         int
	PriceHigh, PriceLow, VolumeQuote                     float64
	PriceStart, PriceCurrent, PriceIncrease, PriceChange float64
}

func (aggregate *AggregateCandle) GetKey() string {
	return fmt.Sprintf(`%s*%s*%d*%d:%d`, aggregate.Market, aggregate.Symbol, aggregate.TimeInterval,
		aggregate.Start.Hour(), aggregate.Start.Minute())
}

//func (aggregate *AggregateCandle) refreshLink() {
//	for !util.Terminal {
//		nodeHead := aggregate.LinkList.Head
//		nodeTail := aggregate.LinkList.Tail
//		if nodeHead == nil || nodeTail == nil {
//			break
//		} else {
//			if nodeHead.Data.(*Candle).Begin.Add(aggregate.TimeInterval).Before(nodeTail.Data.(*Candle).Begin) {
//				aggregate.LinkList.RemoveHead()
//			}
//		}
//	}
//	node := aggregate.LinkList.Head
//	if node == nil {
//		return
//	}
//	aggregate.Start = &node.Data.(*Candle).Begin
//	aggregate.End = &node.Data.(*Candle).Begin
//	aggregate.PriceStart = node.Data.(*Candle).PriceOpen
//	aggregate.PriceCurrent = node.Data.(*Candle).PriceClose
//	aggregate.PriceHigh = node.Data.(*Candle).PriceHigh
//	aggregate.PriceLow = node.Data.(*Candle).PriceLow
//	aggregate.VolumeQuote = 0
//	for !util.Terminal {
//		if node == nil {
//			break
//		} else {
//			aggregate.VolumeQuote += node.Data.(*Candle).VolumeQuote
//			if aggregate.PriceHigh < node.Data.(*Candle).PriceHigh {
//				aggregate.PriceHigh = node.Data.(*Candle).PriceHigh
//			}
//			if aggregate.PriceLow > node.Data.(*Candle).PriceLow || aggregate.PriceLow == 0 {
//				aggregate.PriceLow = node.Data.(*Candle).PriceLow
//			}
//			node = node.Next
//		}
//	}
//}
//
//func (aggregate *AggregateCandle) HandleLink(candle *Candle) {
//	if aggregate.End == nil {
//		aggregate.End = &candle.Begin
//	} else if candle.Begin.Add(aggregate.TimeInterval).Before(*aggregate.End) {
//		util.Notice(fmt.Sprintf(`ignore passed by %s %s %s`,
//			candle.Market, candle.Symbol, candle.Begin.String()))
//		return
//	}
//	nodeCurrent := aggregate.LinkList.Tail
//	for !util.Terminal {
//		if nodeCurrent == nil {
//			aggregate.LinkList.AddHead(&util.Node{Data: candle})
//			break
//		} else if nodeCurrent.Data.(*Candle).Begin.Before(candle.Begin) {
//			aggregate.LinkList.Insert(nodeCurrent, &util.Node{Data: candle})
//			break
//		} else {
//			util.Notice(fmt.Sprintf(`wrong candle sequence %s %s %s > %s`,
//				candle.Market, candle.Symbol, nodeCurrent.Data.(*Candle).Begin.String(), candle.Begin.String()))
//			nodeCurrent = nodeCurrent.Prev
//		}
//	}
//	needRefresh := false
//	for !util.Terminal {
//		head := aggregate.LinkList.Head
//		if head == nil {
//			break
//		} else if head.Data.(*Candle).Begin.Add(aggregate.TimeInterval).Before(*aggregate.End) {
//			aggregate.VolumeQuote -= head.Data.(*Candle).VolumeQuote
//			if aggregate.PriceHigh == candle.PriceHigh {
//				needRefresh = true
//			}
//			if aggregate.PriceLow == candle.PriceLow {
//				needRefresh = true
//			}
//			aggregate.LinkList.RemoveHead()
//		} else {
//			break
//		}
//	}
//	if aggregate.LinkList.Head == nil || aggregate.LinkList.Tail == nil {
//		return
//	}
//	headData := aggregate.LinkList.Head.Data.(*Candle)
//	tailData := aggregate.LinkList.Tail.Data.(*Candle)
//	aggregate.Start = &headData.Begin
//	aggregate.End = &tailData.Begin
//	aggregate.PriceStart = headData.PriceOpen
//	aggregate.PriceCurrent = tailData.PriceClose
//	aggregate.VolumeQuote += candle.VolumeQuote
//	if needRefresh == true {
//		util.Notice(fmt.Sprintf(`refreshLink link list %s %s seconds %f len %d`,
//			candle.Market, candle.Symbol, aggregate.TimeInterval.Seconds(), aggregate.LinkList.Len))
//		aggregate.refreshLink()
//	} else {
//		if aggregate.PriceHigh < candle.PriceHigh {
//			aggregate.PriceHigh = candle.PriceHigh
//		}
//		if aggregate.PriceLow == 0 || aggregate.PriceLow > candle.PriceLow {
//			aggregate.PriceLow = candle.PriceLow
//		}
//	}
//	aggregate.PriceChange = (aggregate.PriceHigh - aggregate.PriceLow) / aggregate.PriceCurrent
//	aggregate.PriceIncrease = (aggregate.PriceCurrent - aggregate.PriceStart) / aggregate.PriceCurrent
//}

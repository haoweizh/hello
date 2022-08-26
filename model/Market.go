package model

import (
	"fmt"
	"hello/util"
	"sync"
	"time"
)

type KLinePoint struct {
	TS            int64
	EndPrice      float64
	HighPrice     float64
	LowPrice      float64
	RSI           float64
	RSIExpectBuy  float64
	RSIExpectSell float64
}

type Deal struct {
	Ts     int64
	Market string
	Symbol string
	Amount float64
	Id     string
	Side   string
	Price  float64
}

type BidAsk struct {
	Ts         int // time in unix epoch millionSeconds
	TsReceived int
	UpdateId   int64
	Bids       Ticks
	Asks       Ticks
}

type Rule struct {
	Margin float64
	Delay  float64
}

type Markets struct {
	bidAsks     sync.Map // symbol - market - bidAsk
	WsDepth     sync.Map // market - []chan struct{}
	WsInitTime  sync.Map // market - time
	Connections sync.Map // market - []*websocket.Conn
}

func (markets *Markets) ToStringBidAsk(bidAsk *BidAsk) (result string) {
	if bidAsk == nil || bidAsk.Bids == nil || bidAsk.Asks == nil {
		return ``
	}
	for i := bidAsk.Bids.Len() - 1; i >= 0; i-- {
		result += fmt.Sprintf(`%f,`, bidAsk.Bids[i].Price)
	}
	result += `--|--`
	for i := 0; i < bidAsk.Asks.Len(); i++ {
		result += fmt.Sprintf(`%f,`, bidAsk.Asks[i].Price)
	}
	return
}

// GetPriceForce 返回tick价格，如果没有tick，从其他setting对应的market中返回价格
func (markets *Markets) GetPriceForce(symbol, market string, settingMarkets []string) (result bool, price float64) {
	value, _ := markets.bidAsks.Load(symbol)
	if value != nil {
		item, _ := value.(*sync.Map).Load(market)
		if item != nil {
			return true, item.(*BidAsk).Bids[0].Price
		}
		for _, settingMarket := range settingMarkets {
			item, _ = value.(*sync.Map).Load(settingMarket)
			if item != nil {
				return true, item.(*BidAsk).Bids[0].Price
			}
		}
	}
	return false, 0
}

func (markets *Markets) GetBidAsk(symbol, market string) (result bool, bidAsk *BidAsk) {
	value, _ := markets.bidAsks.Load(symbol)
	if value != nil {
		item, _ := value.(*sync.Map).Load(market)
		if item != nil {
			return true, item.(*BidAsk)
		}
	}
	return false, nil
}

func (markets *Markets) SetBidAsk(symbol, marketName string, bidAsk *BidAsk) bool {
	if bidAsk == nil || bidAsk.Bids == nil || bidAsk.Asks == nil || bidAsk.Bids.Len() == 0 || bidAsk.Asks.Len() == 0 {
		util.SocketInfo(marketName + `do not set nil or empty bid ask` + symbol)
		return false
	}
	if bidAsk.Bids[0].Price >= bidAsk.Asks[0].Price || bidAsk.Bids[0].Price == 0 || bidAsk.Bids[0].Amount == 0 ||
		bidAsk.Asks[0].Price == 0 || bidAsk.Asks[0].Amount == 0 {
		if time.Now().Second() == 0 {
			util.SocketInfo(fmt.Sprintf(`do not set mistake %s %s bid %f ask %f`,
				marketName, symbol, bidAsk.Bids[0].Price, bidAsk.Asks[0].Price))
		}
		return false
	}
	value, _ := markets.bidAsks.Load(symbol)
	if value == nil {
		value = &sync.Map{}
		markets.bidAsks.Store(symbol, value)
	}
	oldBidAsk := value.(*sync.Map)
	last, _ := oldBidAsk.Load(marketName)
	if last == nil || last.(*BidAsk).Ts < bidAsk.Ts {
		//if last != nil && last.Bids[0].Price == bidAsk.Bids[0].Price && last.Bids[0].Amount == bidAsk.Bids[0].Amount &&
		//	last.Asks[0].Price == bidAsk.Asks[0].Price && last.Asks[0].Amount == bidAsk.Asks[0].Amount && symbol == `DMG/USD` {
		//	util.Info(fmt.Sprintf(`%s %s same as before`, marketName, symbol))
		//}
		oldBidAsk.Store(marketName, bidAsk)
		if last != nil {
			if marketName == BybitPerp {
				util.Info(fmt.Sprintf(`get bybitperp at %s %d`, symbol, bidAsk.Ts))
			}
			go AppMetric.AddTick(marketName, symbol, util.GetNow(), last.(*BidAsk), bidAsk)
		}
		return true
	}
	return false
}

func (markets *Markets) GetSymbols() (symbols map[string]bool) {
	symbols = make(map[string]bool)
	markets.bidAsks.Range(func(key, value interface{}) bool {
		symbols[key.(string)] = true
		return true
	})
	return
}

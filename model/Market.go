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

type Environment struct {
	markPriceInfos   sync.Map  // symbol - market - ticker 行情包含标记价格
	bidAsks          sync.Map  // symbol - market - bidAsk
	kLines           sync.Map  // symbol - market - *candle
	MsgChanTick      sync.Map  // market - []chan struct{}
	MsgChanKLine     sync.Map  // market - []chan struct{}
	WsInitTime       sync.Map  // market - time
	SocketsTick      sync.Map  // market - map[*websocket.Conn]bool for depth sockets
	AccountConns     sync.Map  // market*accountKey - *websocket.Conn
	AggregateCandles *sync.Map // sync.Map[market]*sync.Map[symbol]*sync.Map[mailAddress] *AggregateCandles
	WsManager        *WSManager
}

type MarkPriceInfo struct {
	MarkPrice float64
	Ts        int // time in unix epoch millionSeconds
}

func (environment *Environment) SetMarkPriceInfo(symbol, marketName string, ticker *MarkPriceInfo) {
	value, _ := environment.markPriceInfos.Load(symbol)
	if value == nil {
		value = &sync.Map{}
		environment.markPriceInfos.Store(symbol, value)
	}
	oldTicker := value.(*sync.Map)
	last, _ := oldTicker.Load(marketName)
	if last == nil || last.(*MarkPriceInfo).Ts <= ticker.Ts {
		oldTicker.Store(marketName, ticker)
	}
}

func (environment *Environment) GetMarkPriceInfo(symbol, marketName string) *MarkPriceInfo {
	value, _ := environment.markPriceInfos.Load(symbol)
	if value != nil {
		item, _ := value.(*sync.Map).Load(marketName)
		if item != nil {
			return item.(*MarkPriceInfo)
		}
	}
	return nil
}

func (environment *Environment) ToStringBidAsk(bidAsk *BidAsk) (result string) {
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

func (environment *Environment) GetKLine(symbol, market string) (result bool, candle *Candle) {
	value, _ := environment.kLines.Load(symbol)
	if value != nil {
		item, _ := value.(*sync.Map).Load(market)
		if item != nil {
			return true, item.(*Candle)
		}
	}
	return false, nil
}

func (environment *Environment) SetCandle(symbol, market string, candle *Candle) bool {
	value, _ := environment.kLines.Load(symbol)
	if value == nil {
		value = &sync.Map{}
		environment.kLines.Store(symbol, value)
	}
	symbolCandle := value.(*sync.Map)
	last, _ := symbolCandle.Load(market)
	if last == nil || last.(*Candle).Begin.Before(candle.Begin) {
		symbolCandle.Store(market, candle)
		return true
	}
	return false
}

func (environment *Environment) GetBidAsk(symbol, market string) (result bool, bidAsk *BidAsk) {
	value, _ := environment.bidAsks.Load(symbol)
	if value != nil {
		item, _ := value.(*sync.Map).Load(market)
		if item != nil {
			return true, item.(*BidAsk)
		}
	}
	return false, nil
}

func (environment *Environment) SetBidAsk(symbol, marketName string, bidAsk *BidAsk) bool {
	if bidAsk == nil || bidAsk.Bids == nil || bidAsk.Asks == nil || bidAsk.Bids.Len() == 0 || bidAsk.Asks.Len() == 0 {
		//util.SocketInfo(fmt.Sprintf(`do not set nil or empty bid ask %s %s data:%v`, marketName, symbol, bidAsk))
		return false
	}
	if bidAsk.Bids[0].Price >= bidAsk.Asks[0].Price || bidAsk.Bids[0].Price == 0 || bidAsk.Bids[0].Amount == 0 ||
		bidAsk.Asks[0].Price == 0 || bidAsk.Asks[0].Amount == 0 {
		if time.Now().Second() == 0 {
			util.SocketInfo(fmt.Sprintf(`do not set mistake %s %s bid %f ask %f data: %v`,
				marketName, symbol, bidAsk.Bids[0].Price, bidAsk.Asks[0].Price, bidAsk))
		}
		return false
	}
	//_, _, coin, _ := GetFromStandard(marketName, symbol)
	//if len(coin) > 4 && coin[:4] == `1000` {
	//	settings := api.GetSettingsFromCoin(coin[4:])
	//	if settings != nil && len(settings) > 0 {
	//		for i := 0; i < bidAsk.Bids.Len(); i++ {
	//			bidAsk.Bids[0].PriceDiv1000 = bidAsk.Bids[0].Price / 1000
	//			bidAsk.Bids[0].AmountMul1000 = bidAsk.Bids[0].Amount * 1000
	//		}
	//		for i := 0; i < bidAsk.Asks.Len(); i++ {
	//			bidAsk.Asks[0].PriceDiv1000 = bidAsk.Asks[0].Price / 1000
	//			bidAsk.Asks[0].AmountMul1000 = bidAsk.Asks[0].Amount * 1000
	//		}
	//	}
	//}
	value, _ := environment.bidAsks.Load(symbol)
	if value == nil {
		value = &sync.Map{}
		environment.bidAsks.Store(symbol, value)
	}
	oldBidAsk := value.(*sync.Map)
	last, _ := oldBidAsk.Load(marketName)
	if last == nil || last.(*BidAsk).Ts <= bidAsk.Ts {
		oldBidAsk.Store(marketName, bidAsk)
		if last != nil {
			go AppMetric.AddTick(marketName, symbol, util.GetNow(), last.(*BidAsk), bidAsk)
		}
		return true
	}
	return false
}

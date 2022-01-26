package model

import (
	"fmt"
	"github.com/gorilla/websocket"
	"hello/util"
	"sync"
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
	Ts         int // time in unix epoch million seconds
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
	lock            sync.Mutex
	bmPendingOrders map[string]*Order                     // bm中的orderId-order
	TrendEnd        map[string]map[string]*Deal           // symbol - market - deal
	TrendStart      map[string]map[string]*Deal           // symbol - market - deal
	bidAsks         map[string]map[string]*BidAsk         // symbol - market - bidAsk
	trade           map[int64]map[string]map[string]*Deal // time in second - symbol - market - deal
	BigDeals        map[string]map[string]*Deal           // symbol - market - Deal
	wsDepth         map[string][]chan struct{}            // market - []depth channel
	Connections     map[string][]*websocket.Conn          // market - conn
}

func NewMarkets() *Markets {
	return &Markets{bidAsks: make(map[string]map[string]*BidAsk), wsDepth: make(map[string][]chan struct{}),
		trade: make(map[int64]map[string]map[string]*Deal)}
}

func (markets *Markets) GetBmPendingOrders() (orders map[string]*Order) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.bmPendingOrders == nil {
		markets.bmPendingOrders = make(map[string]*Order)
	}
	return markets.bmPendingOrders
}

func (markets *Markets) RemoveBmPendingOrder() (order *Order) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.bmPendingOrders == nil {
		return nil
	}
	for orderId, item := range markets.bmPendingOrders {
		delete(markets.bmPendingOrders, orderId)
		return item
	}
	return nil
}

func (markets *Markets) AddBMPendingOrder(order *Order) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.bmPendingOrders == nil {
		markets.bmPendingOrders = make(map[string]*Order)
	}
	markets.bmPendingOrders[order.OrderId] = order
}

func (markets *Markets) GetTrends(symbol string) (start, end map[string]*Deal) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.TrendStart != nil {
		start = markets.TrendStart[symbol]
	}
	if markets.TrendEnd != nil {
		end = markets.TrendEnd[symbol]
	}
	return start, end
}

func (markets *Markets) SetTrade(deal *Deal) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.trade == nil {
		markets.trade = make(map[int64]map[string]map[string]*Deal)
	}
	second := deal.Ts / 1000
	symbol := deal.Symbol
	if len(markets.trade[second]) > 1000 {
		markets.trade = nil
		markets.TrendStart = nil
		util.Notice(fmt.Sprintf(`clear trade map and trend point`))
	}
	if markets.trade[second] == nil {
		markets.trade[second] = make(map[string]map[string]*Deal)
	}
	if markets.trade[second][symbol] == nil {
		markets.trade[second][symbol] = make(map[string]*Deal)
	}
	if markets.trade[second][symbol][deal.Market] != nil {
		return
	}
	markets.trade[second][symbol][deal.Market] = deal
	if markets.trade[second] != nil && markets.trade[second][symbol] != nil &&
		markets.trade[second][symbol][Bitmex] != nil {
		chance := 15.0
		compareSecond := second - int64(chance)
		compare := markets.trade[compareSecond]
		if compare != nil && compare[symbol] != nil && compare[symbol][Bitmex] != nil {
			markets.TrendStart = compare
			markets.TrendEnd = markets.trade[second]
		}
		delete(markets.trade, compareSecond)
	}
}

func (markets *Markets) SetConnections(market string, connections []*websocket.Conn) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.Connections == nil {
		markets.Connections = make(map[string][]*websocket.Conn)
	}
	util.Notice(`set connection %s, %d`, market, len(connections))
	markets.Connections[market] = connections
}

func (markets *Markets) GetBigDeal(symbol, market string) (deal *Deal) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.BigDeals == nil {
		markets.BigDeals = make(map[string]map[string]*Deal)
	}
	if markets.BigDeals[symbol] == nil {
		markets.BigDeals[symbol] = make(map[string]*Deal)
	}
	return markets.BigDeals[symbol][market]
}

func (markets *Markets) GetPrice(symbol string) (result bool, price float64) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	for _, bidAsks := range AppMarkets.bidAsks[symbol] {
		if bidAsks != nil && bidAsks.Bids != nil {
			return true, bidAsks.Bids[0].Price
		}
	}
	return false, 0
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

func (markets *Markets) CopyBidAsk(symbol, market string) (result bool, bidAsk *BidAsk) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.bidAsks == nil || markets.bidAsks[symbol] == nil || markets.bidAsks[symbol][market] == nil ||
		markets.bidAsks[symbol][market].Asks == nil || markets.bidAsks[symbol][market].Bids == nil ||
		markets.bidAsks[symbol][market].Asks.Len() == 0 || markets.bidAsks[symbol][market].Bids.Len() == 0 {
		return false, nil
	}
	bidAsk = &BidAsk{}
	bidAsk.Bids = make([]Tick, markets.bidAsks[symbol][market].Bids.Len())
	bidAsk.Asks = make([]Tick, markets.bidAsks[symbol][market].Asks.Len())
	for key, value := range markets.bidAsks[symbol][market].Bids {
		bidAsk.Bids[key] = value
	}
	for key, value := range markets.bidAsks[symbol][market].Asks {
		bidAsk.Asks[key] = value
	}
	return true, bidAsk
}

func (markets *Markets) GetBidAsk(symbol, market string) (result bool, bidAsk *BidAsk) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.bidAsks == nil || markets.bidAsks[symbol] == nil || markets.bidAsks[symbol][market] == nil ||
		markets.bidAsks[symbol][market].Asks == nil || markets.bidAsks[symbol][market].Bids == nil ||
		markets.bidAsks[symbol][market].Asks.Len() == 0 || markets.bidAsks[symbol][market].Bids.Len() == 0 {
		return false, nil
	}
	return true, markets.bidAsks[symbol][market]
}

func (markets *Markets) SetBidAsk(symbol, marketName string, bidAsk *BidAsk) bool {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	if markets.bidAsks == nil {
		markets.bidAsks = make(map[string]map[string]*BidAsk)
	}
	if markets.bidAsks[symbol] == nil {
		markets.bidAsks[symbol] = make(map[string]*BidAsk)
	}
	if bidAsk == nil || bidAsk.Bids == nil || bidAsk.Asks == nil || bidAsk.Bids.Len() == 0 || bidAsk.Asks.Len() == 0 {
		markets.bidAsks[symbol][marketName] = nil
		util.SocketInfo(marketName + `do not set nil or empty bid ask` + symbol)
		return false
	}
	if bidAsk.Bids[0].Price >= bidAsk.Asks[0].Price {
		util.SocketInfo(fmt.Sprintf(`do not set mistake %s %s bid %f ask %f`,
			marketName, symbol, bidAsk.Bids[0].Price, bidAsk.Asks[0].Price))
		return false
	}
	last := markets.bidAsks[symbol][marketName]
	if last == nil || last.Ts < bidAsk.Ts {
		//if last != nil && last.Bids[0].Price == bidAsk.Bids[0].Price && last.Bids[0].Amount == bidAsk.Bids[0].Amount &&
		//	last.Asks[0].Price == bidAsk.Asks[0].Price && last.Asks[0].Amount == bidAsk.Asks[0].Amount && symbol == `DMG/USD` {
		//	util.Info(fmt.Sprintf(`%s %s same as before`, marketName, symbol))
		//}
		markets.bidAsks[symbol][marketName] = bidAsk
		current := util.GetNow()
		AppMetric.AddTick(marketName, symbol, current, last, bidAsk)
		return true
	}
	return false
}

func (markets *Markets) GetDepthChan(marketName string) []chan struct{} {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	return markets.wsDepth[marketName]
}

func (markets *Markets) PutDepthChan(marketName string, channels []chan struct{}) {
	markets.lock.Lock()
	defer markets.lock.Unlock()
	markets.wsDepth[marketName] = channels
	util.Notice(`set connection stopC %s %d`, marketName, len(channels))
}

func (markets *Markets) GetSymbols() (symbols map[string]bool) {
	symbols = make(map[string]bool)
	for symbol := range markets.bidAsks {
		symbols[symbol] = true
	}
	return
}

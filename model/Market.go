package model

import (
	"fmt"
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

type WSResp struct {
	RequestId, Msg, OrderId string
	DealAmount              float64
	Success                 bool
}

type Environment struct {
	markPriceInfos  sync.Map // symbol - market - ticker 行情包含标记价格
	bidAsks         sync.Map // market*symbol - bidAsk
	kLines          sync.Map // symbol - market - *candle
	MsgChanTick     sync.Map // market - []chan struct{}
	MsgChanKLine    sync.Map // market - []chan struct{}
	WsInitTime      sync.Map // market - time
	ConnTick        sync.Map // market - map[*websocket.Conn]bool for depth sockets
	ConnOrder       sync.Map // market*accountKey / Gate*marketType*accountKey - *WSConn;
	ConnOrderUpdate sync.Map // market*accountKey / Gate*marketType*accountKey - *WSConn;
	WSOrderMap      sync.Map // requestId - *Order
	CrossOrders     sync.Map // orderId - *Order
	WSRespChan      chan WSResp
	MonitorSettings *sync.Map // sync.Map[market]*sync.Map[symbol]*sync.Map[interval]*sync.Map[address]*MonitorSetting
	WsManager       *WSManager
}

type MarkPriceInfo struct {
	MarkPrice float64
	Ts        int // time in unix epoch millionSeconds
}

func (environment *Environment) HandleWSResp() {
	for {
		wsResp := <-environment.WSRespChan
		value, _ := environment.WSOrderMap.Load(wsResp.RequestId)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`get ws order req id %s`, wsResp.RequestId))
		if value != nil {
			order := value.(*Order)
			order.OrderId = wsResp.OrderId
			if wsResp.Success {
				order.Status = CarryStatusWorking
			} else {
				order.Status = CarryStatusFail
				order.ErrCode = wsResp.Msg
			}
			environment.CrossOrders.Store(wsResp.OrderId, order)
			environment.WSOrderMap.Delete(wsResp.RequestId)
			util.Log(util.LogLevelInfo, fmt.Sprintf(`del request id %s store order %s %s type %s id %s`,
				wsResp.RequestId, order.Market, order.Symbol, order.RefreshType, order.OrderId))
			if AccountHandlerMap[order.RefreshType] != nil {
				AccountHandlerMap[order.RefreshType](order)
			}
		}
	}
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
	if last == nil || last.(*Candle).CreatedAt.Before(candle.CreatedAt) {
		//fmt.Println(fmt.Sprintf(`%s set candle to %s %s %s`, time.Now().String(), symbol, market, candle.CreatedAt.String()))
		symbolCandle.Store(market, candle)
		return true
	}
	return false
}

func (environment *Environment) GetBidAsk(market, symbol string) (result bool, bidAsk *BidAsk) {
	value, _ := util.LoadSyncMap(&environment.bidAsks, market, symbol)
	if value != nil {
		return true, value.(*BidAsk)
	}
	return false, nil
}

//var testConn sync.Map

func (environment *Environment) SetBidAsk(market, symbol string, bidAsk *BidAsk) bool {
	if bidAsk == nil || bidAsk.Bids == nil || bidAsk.Asks == nil || bidAsk.Bids.Len() == 0 || bidAsk.Asks.Len() == 0 {
		return false
	}
	if bidAsk.Bids[0].Price >= bidAsk.Asks[0].Price || bidAsk.Bids[0].Price == 0 || bidAsk.Bids[0].Amount == 0 ||
		bidAsk.Asks[0].Price == 0 || bidAsk.Asks[0].Amount == 0 {
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
	last, _ := util.LoadSyncMap(&environment.bidAsks, market, symbol)
	if last == nil || last.(*BidAsk).Ts <= bidAsk.Ts {
		//if market == BinancePerp {
		//	ts := fmt.Sprintf(`%s %d %d`, symbol, time.Now().Hour(), time.Now().Minute())
		//	minBidAsk, _ := testConn.Load(ts)
		//	if minBidAsk == nil {
		//		testConn.Store(ts, bidAsk)
		//		util.Info(fmt.Sprintf(`set bn tick %s %d`, symbol, bidAsk.Ts))
		//	}
		//}
		if last != nil {
			go AppMetric.AddTick(market, symbol, util.GetNow(), last.(*BidAsk), bidAsk)
		}
		util.StoreSyncMap(&environment.bidAsks, bidAsk, market, symbol)
		return true
	} else {
		//	util.Info(fmt.Sprintf(`8 test return no set old tick %s %d <= %d`, symbol, last.(*BidAsk).Ts, bidAsk.Ts))
	}
	return false
}

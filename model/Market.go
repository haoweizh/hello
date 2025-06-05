package model

import (
	"fmt"
	"hello/util"
	"strings"
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

type WSResp struct {
	RequestId, Msg, OrderId string
	DealAmount              float64
	Success                 bool
}

type Environment struct {
	markPriceInfos                               sync.Map // symbol - market - ticker 行情包含标记价格
	BidAsk                                       sync.Map // market*symbol - bidAsk
	kLines                                       sync.Map // symbol - market - *candle
	MsgChanKLine                                 sync.Map // market - []chan struct{}
	WsInitTime                                   sync.Map // market - time
	ConnTick                                     sync.Map // market(特殊处理gate） - map[*websocket.conn]bool for depth sockets
	ConnOrder                                    sync.Map // market*accountKey / Gate*marketType*accountKey - *WSConn;
	ConnOrderUpdate                              sync.Map // market*accountKey / Gate*marketType*accountKey - *WSConn;
	ReqIdOrders                                  sync.Map // requestId - *Order
	OrderIdOrders                                sync.Map // orderId - *Order
	RiskLimitsGate                               sync.Map // accountKey * symbol - money in usdt
	PauseTrade                                   sync.Map // coin*market*symbol*key*orderSide bool
	ADLSymbol                                    sync.Map // market symbol key
	WSRespChan                                   chan WSResp
	MonitorSettings                              sync.Map // sync.Map[market]*sync.Map[symbol]*sync.Map[interval]*sync.Map[address]*MonitorSetting
	WsManager                                    *WSManager
	Markets                                      []string
	Settings                                     []Setting
	CrossEqualing, CrossPause, CrossStop, Moving bool
	PriConnecting                                sync.Map // accountKey * market - bool
	SpecialChans                                 sync.Map // tsCode * wsType *WSConn
	LastOrderMilli                               sync.Map // account.Key - last order time in million-seconds
	PubChanNeedReset                             sync.Map // market - bool
	PubSubscribes                                sync.Map // market*wsUrl - []interface{}
	OkexPubMarkets                               sync.Map // channel*instid - string
	FundingFeeToday                              sync.Map // account index*market*symbol - value in usdt
}

type MarkPriceInfo struct {
	MarkPrice float64
	Ts        int // time in unix epoch millionSeconds
}

func (environment *Environment) AddWsClientOrder(clientOid string, order *Order) {
	environment.ReqIdOrders.Store(clientOid, order)
	ts := time.Now().Unix()
	failOrders := make(map[int]int)
	environment.ReqIdOrders.Range(func(requestId, value interface{}) bool {
		if value == nil {
			return true
		}
		failOrder := value.(*Order)
		orderTs := value.(*Order).OrderTime.Unix()
		if ts-orderTs > 180 && value.(*Order).OrderId == value.(*Order).ClientOrdId {
			failOrders[failOrder.AccountIndex] = failOrders[failOrder.AccountIndex] + 1
			util.Log(util.LogLevelInfo, fmt.Sprintf(`no response ws order %s %#v len %d`, requestId, value, failOrders[failOrder.AccountIndex]))
		}
		return true
	})
	for i, fails := range failOrders {
		if fails > 100 {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`stop cross no response ws order %d %#v`, i, failOrders[i]))
			environment.CrossStop = true
		}
	}
}

func (environment *Environment) HandleWSResp() {
	for !util.Terminal {
		wsResp := <-environment.WSRespChan
		value, _ := environment.ReqIdOrders.Load(wsResp.RequestId)
		if value == nil {
			value, _ = environment.ReqIdOrders.Load(wsResp.RequestId + OrderSideSell)
			//util.Log(util.LogLevelInfo, fmt.Sprintf(`get pair order sell %#v`, value))
			if value == nil {
				value, _ = environment.ReqIdOrders.Load(wsResp.RequestId + OrderSideBuy)
				//util.Log(util.LogLevelInfo, fmt.Sprintf(`get pair order buy %#v`, value))
			}
		}
		if value != nil {
			//if len(strings.Trim(wsResp.OrderId, ` `)) == 0 {
			//	continue
			//}
			order := value.(*Order)
			if len(strings.Trim(wsResp.OrderId, ` `)) < 4 {
				wsResp.Success = false
			}
			if wsResp.Success {
				order.Status = CarryStatusWorking
				order.OrderId = wsResp.OrderId
				environment.OrderIdOrders.Store(wsResp.OrderId, order)
			} else {
				order.Status = CarryStatusFail
				order.ErrCode = wsResp.Msg
				environment.OrderIdOrders.Store(wsResp.RequestId, order)
			}
			environment.ReqIdOrders.Delete(wsResp.RequestId)
			util.Log(util.LogLevelInfo, fmt.Sprintf(`del request store order %s %s %s %s %s %d %#v`,
				order.Market, order.Coin, order.Symbol, order.OrderSide, wsResp.RequestId, time.Now().Unix()-order.OrderTime.Unix(), order))
			if AccountHandlerMap[order.RefreshType] != nil {
				AccountHandlerMap[order.RefreshType](order)
			}
		} else {
			util.Log(util.LogLevelError, fmt.Sprintf(`no order for request id %s`, wsResp.RequestId))
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
	value, _ := util.LoadSyncMap(&environment.BidAsk, market, symbol)
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
	//last, _ := util.LoadSyncMap(&environment.BidAsk, market, symbol)
	//if last == nil || last.(*BidAsk).Ts <= bidAsk.Ts {
	//	if last != nil && AppConfig.Debug && time.Now().UnixMilli()-int64(bidAsk.Ts) < 100 {
	//		go AppMetric.AddTick(market, symbol, util.GetNow(), last.(*BidAsk), bidAsk)
	//	}
	//	util.StoreSyncMap(&environment.BidAsk, bidAsk, market, symbol)
	//	return true
	//}
	//go AppMetric.AddTick(market, symbol, util.GetNow(), nil, bidAsk)
	util.StoreSyncMap(&environment.BidAsk, bidAsk, market, symbol)
	return true
}

package api

import (
	"fmt"
	"hello/model"
	"hello/util"
	"strings"
	"sync"
	"time"
)

func GetWSSubscribes(market string, subTypes []string) []interface{} {
	symbols := GetMarketSymbols(market)
	subscribes := make([]interface{}, 0)
	for symbol := range symbols {
		if len(strings.Trim(symbol, ` `)) == 0 {
			continue
		}
		for _, value := range subTypes {
			subscribe := GetWSSubscribe(market, symbol, value)
			if subscribe == nil || subscribe == "" {
				continue
			}
			duplicated := false
			for _, sub := range subscribes {
				if market == model.Ftx { // subscribe类型为[]string
					itemSub := sub.([]string)
					itemSubscribe := subscribe.([]string)
					if itemSub[0] == itemSubscribe[0] && itemSub[1] == itemSubscribe[1] {
						duplicated = true
						break
					}
				} else { // subscribe类型为string
					if sub.(string) == subscribe.(string) {
						duplicated = true
						break
					}
				}
			}
			if !duplicated {
				subscribes = append(subscribes, subscribe)
			}
		}
	}
	if market == model.Bitmex {
		subscribes = append(subscribes, `position`)
		subscribes = append(subscribes, `order`)
	}
	return subscribes
}

func GetWSSubscribe(market, symbol, subType string) (subscribe interface{}) {
	_, _, _, dialectSymbol := model.GetFromStandard(market, symbol)
	switch market {
	//case model.Mexc:
	//	switch subType {
	//	case deprecated.mexcContractDepthIncSubType:
	//		return fmt.Sprintf(`{"method":"sub.depth","param":{"symbol":"%s","compress":true}}`, dialectSymbol)
	//	case deprecated.mexcContractDepthFullSubType:
	//		return fmt.Sprintf(`{"method":"sub.depth.full","param":{"symbol":"%s","limit":5}}`, dialectSymbol)
	//	case deprecated.mexcContractTickerSubType:
	//		return fmt.Sprintf(`{"method":"sub.ticker","param":{"symbol":"%s"}}`, dialectSymbol)
	//	}
	case model.OKEX:
		return dialectSymbol
	case model.BinancePerp:
		if subType == model.SubscribeDepth {
			return strings.ToLower(dialectSymbol) + `@depth5@100ms`
		} else if subType == model.SubscribeTicker {
			return strings.ToLower(dialectSymbol) + `@bookTicker`
		} else if subType == model.SubscribeMarkPrice {
			return strings.ToLower(dialectSymbol) + `@markPrice`
			//return `!markPrice@arr`
		}
	case model.BinanceSpot, model.BinanceMargin: // XRPUSDT: XRPUSDT@depth5   XRP-PERP: XRPUSDT@depth5
		if subType == model.SubscribeDepth {
			return strings.ToLower(dialectSymbol) + `@depth5@100ms`
		}
		return strings.ToLower(dialectSymbol) + `@bookTicker`
	case model.Ftx:
		if subType == model.SubscribeDepth {
			return []string{`orderbook`, dialectSymbol}
		} else if subType == model.SubscribeTicker {
			return []string{`ticker`, dialectSymbol}
		}
	case model.BitgetSpot:
		return fmt.Sprintf(`{"instType":"SPOT","channel":"books1","instId":"%s"}`, dialectSymbol)
	case model.BitgetPerp:
		if subType == model.SubscribeDepth {
			return fmt.Sprintf(`{"instType":"USDT-FUTURES","channel":"books1","instId":"%s"}`, dialectSymbol)
		} else if subType == model.SubscribeMarkPrice {
			return fmt.Sprintf(`{"instType":"USDT-FUTURES","channel":"ticker","instId":"%s"}`, dialectSymbol)
		}
	case model.DFuture:
		return `dfuture.market.` + dialectSymbol + `.kline.1min`
	}
	return ""
}

// CreateMarketKLineWS
func _(environment *model.Environment, market string, symbols map[string]bool) (
	socketMap map[*model.WSConn]bool, channels []chan struct{}) {
	switch market {
	case model.BinanceSpot:
		util.Log(util.LogLevelInfo, " create KLine ws chan for "+market)
		socketMap, channels, _ = WsKLineBinanceSpot(environment, market, symbols)
	}
	return
}

func CreateWSTick(environment *model.Environment, market string) (
	socketMap map[*model.WSConn]bool, channels []chan struct{}) {
	for {
		locking := CheckSetProcessing(model.FunctionTickMaintain, market, ``, true)
		if !locking {
			break
		}
		time.Sleep(time.Second)
	}
	util.Log(util.LogLevelInfo, " create depth chan for "+market)
	channels = make([]chan struct{}, 1)
	var err error
	switch market {
	case model.Gate: // Gate 代表spot；Gateperp 代表 futures
		socketMap, channels, err = WsTickServeGateSpot(market)
		socketMapPerp, channsPerp, _ := WsTickServeGatePerp(market)
		environment.ConnTick.Store(market+model.MarketTypePerp, socketMapPerp)
		environment.MsgChanTick.Store(market+model.MarketTypePerp, channsPerp)
	case model.OKEX:
		socketMap, channels, err = model.WsPublicClient(market, wsOKEX, GetWSSubscribes(market, []string{model.SubscribeDepth}),
			subscribeHandlerOKEX, wsHandlerOKEX, wsStepOKEX)
	case model.BinanceSpot, model.BinanceMargin:
		socketMap, channels, err = model.WsPublicClient(market, wsBinance+`/stream`, GetWSSubscribes(market, []string{model.SubscribeTicker}),
			subscribeHandlerBinance, wsHandlerBinanceSpot, wsStepBinance)
	case model.BinancePerp:
		socketMap, channels, err = model.WsPublicClient(market, wsBinancePerp+`/stream`, GetWSSubscribes(
			market, []string{model.SubscribeTicker, model.SubscribeMarkPrice}), subscribeHandlerBinance, wsHandlerBinancePerp, wsStepBinance)
		//model.SubscribeDepth
	//case model.HuobiPerp:
	//	socketMap, channels, err = model.WsPublicClient(market, deprecated.wsHuobiPerp, GetWSSubscribes(model.HuobiPerp, []string{model.SubscribeDepth}),
	//		deprecated.subscribeHandlerHuobiPerp, deprecated.wsMsgHandler, deprecated.wsStepHuobi)
	case model.Bybit:
		socketMap, channels, err = WsTickServeBybit(market)
	case model.BitgetSpot:
		socketMap, channels, err = model.WsPublicClient(market, bitgetPublic,
			GetWSSubscribes(market, []string{model.SubscribeDepth}), subscribeHandlerBitget, tickHandlerBitget, wsStepBitget)
	case model.BitgetPerp:
		socketMap, channels, err = WsTickServeBitgetPerp(market)
	}
	environment.ConnTick.Store(market, socketMap)
	environment.MsgChanTick.Store(market, channels)
	if err != nil {
		util.Log(util.LogLevelError, market+` can not create depth server `+err.Error())
	}
	model.AppEnvironment.WsInitTime.Store(market, util.GetNow())
	CheckSetProcessing(model.FunctionTickMaintain, market, ``, false)
	return socketMap, channels
}

var maintainingConnTick = sync.Map{}

func MaintainConns(market string) {
	value, _ := maintainingConnTick.Load(market)
	if value != nil && value.(bool) {
		return
	}
	maintainingConnTick.Store(market, true)
	accounts := model.AppConfig.GetAccounts(market)
	switch market {
	case model.Gate:
		go maintainConnsGate(accounts)
	case model.OKEX:
		go maintainConnsOKEX(accounts)
	case model.BinancePerp, model.BinanceSpot, model.BinanceMargin:
		go MaintainConnsBinance(market, accounts)
	case model.Bybit:
		go maintainConnsBybit(accounts)
	case model.BitgetSpot, model.BitgetPerp:
		go maintainConnsBitget(market, accounts)
	}
}

var wsLock sync.Map // market - *sync.Mutex

func SendToConnection(market string, connection *model.WSConn, msg []byte) (err error) {
	lock, _ := wsLock.Load(market)
	if lock == nil {
		lock = &sync.Mutex{}
		wsLock.Store(market, lock)
	}
	defer lock.(*sync.Mutex).Unlock()
	lock.(*sync.Mutex).Lock()
	if connection == nil {
		util.Log(util.LogLevelError, `fail to write to nil connection `+market)
		return
	}
	if err = connection.WriteMsg(msg); err != nil {
		util.Log(util.LogLevelError, `fail to write to connection `+market+string(msg)+err.Error())
	}
	//util.Log(util.LogLevelDebug, fmt.Sprintf(`send to connection %s %s`, market, string(msg)))
	return err
}

func SendToConnections(market string, connections map[*model.WSConn]bool, msg []byte) (err error) {
	lock, _ := wsLock.Load(market)
	if lock == nil {
		lock = &sync.Mutex{}
		wsLock.Store(market, lock)
	}
	defer lock.(*sync.Mutex).Unlock()
	lock.(*sync.Mutex).Lock()
	for connection := range connections {
		if connection == nil {
			continue
		}
		err = connection.WriteMsg(msg)
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(
				`fail to write to all connection %s %s return: %s`, market, msg, err.Error()))
			//SetRequireReset(market)
		}
	}
	return err
}

func UpdateOrderDeal(market, orderId, status, msg string, dealAmount float64) {
	var order *model.Order
	i := 0
	for ; i < 10; i++ {
		data, _ := model.AppEnvironment.OrderIdOrders.Load(orderId)
		if data != nil {
			order = data.(*model.Order)
			break
		} else {
			time.Sleep(3 * time.Second)
		}
	}
	if order != nil {
		preDeal := order.DealAmount
		if dealAmount >= preDeal {
			order.Status = status
			order.DealAmount = dealAmount
			util.Log(util.LogLevelInfo, fmt.Sprintf(`update deal %s at %d %s %s %s %f to %f %s`,
				orderId, i, order.Market, order.Symbol, order.OrderSide, preDeal, order.DealAmount, order.Status))
		}
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`no order stored %s %s %s`, market, orderId, msg))
	}
}

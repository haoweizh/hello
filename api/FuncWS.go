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
	util.Log(util.LogLevelInfo, fmt.Sprintf("get subscribes %s types %v symbols %d subs %d %#v",
		market, subTypes, len(symbols), len(subscribes), subscribes))
	return subscribes
}

func GetWSSubscribe(market, symbol, subType string) (subscribe interface{}) {
	_, _, _, dialectSymbol := model.GetFromStandard(market, symbol)
	switch market {
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
func _(market string, symbols map[string]bool) (
	socketMap map[*model.WSConn]bool, channels []chan struct{}) {
	switch market {
	case model.BinanceSpot:
		util.Log(util.LogLevelInfo, " create KLine ws chan for "+market)
		socketMap, channels, _ = WsKLineBinanceSpot(market, symbols)
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
		socketMap, err = WsTickServeGateSpot(market)
		environment.ConnTick.Store(GetPublicConnKey(model.Gate, model.MarketTypeSpot), socketMap)
		socketMapPerp, _ := WsTickServeGatePerp(market)
		environment.ConnTick.Store(GetPublicConnKey(model.Gate, model.MarketTypePerp), socketMapPerp)
	case model.OKEX:
		socketMap, err = model.WsPublicClient(market, model.WsOKEX, GetWSSubscribes(market, []string{model.SubscribeDepth}),
			subscribeHandlerOKEX, wsHandlerOKEX, wsStepOKEX, false)
		environment.ConnTick.Store(GetPublicConnKey(market, ``), socketMap)
	case model.BinanceSpot:
		socketMap, err = model.WsPublicClient(market, model.WsBinance+`/stream`, GetWSSubscribes(market, []string{model.SubscribeTicker}),
			subscribeHandlerBinance, wsHandlerBinanceSpot, wsStepBinance, false)
		environment.ConnTick.Store(GetPublicConnKey(market, ``), socketMap)
	case model.BinancePerp:
		socketMap, err = model.WsPublicClient(market, model.WsBinancePerp+`/stream`, GetWSSubscribes(
			market, []string{model.SubscribeTicker, model.SubscribeMarkPrice}), subscribeHandlerBinance, wsHandlerBinancePerp,
			wsStepBinance, false)
		//model.SubscribeDepth
		environment.ConnTick.Store(GetPublicConnKey(market, ``), socketMap)
	case model.Bybit:
		socketMap, err = WsTickServeBybit(market)
		environment.ConnTick.Store(GetPublicConnKey(market, ``), socketMap)
		//case model.BitgetSpot:
		//	socketMap, err = model.WsPublicClient(market, deprecated.bitgetPublic,
		//		GetWSSubscribes(market, []string{model.SubscribeDepth}), deprecated.subscribeHandlerBitget, deprecated.tickHandlerBitget, deprecated.wsStepBitget)
		//	environment.ConnTick.Store(GetPublicConnKey(market, ``), socketMap)
		//case model.BitgetPerp:
		//	socketMap, err = deprecated.WsTickServeBitgetPerp(market)
		//	environment.ConnTick.Store(GetPublicConnKey(market, ``), socketMap)
	}
	if err != nil {
		util.Log(util.LogLevelError, market+` can not create depth server `+err.Error())
	}
	model.AppEnvironment.WsInitTime.Store(market, util.GetNow())
	CheckSetProcessing(model.FunctionTickMaintain, market, ``, false)
	return socketMap, channels
}

func GetPublicConnKey(market, marketType string) string {
	switch market {
	case model.BinanceSpot, model.BinancePerp, model.BitgetSpot, model.BitgetPerp, model.OKEX, model.Bybit:
		return fmt.Sprintf(market)
	case model.Gate:
		return fmt.Sprintf(`%s*%s`, market, marketType)
	}
	return ``
}

func getPrivateConnKey(market, accountKey, marketType string) string {
	switch market {
	case model.BinanceSpot, model.BinancePerp, model.BitgetSpot, model.BitgetPerp, model.OKEX, model.Bybit:
		return fmt.Sprintf(`%s*%s`, market, accountKey)
	case model.Gate:
		return fmt.Sprintf(`%s*%s*%s`, market, accountKey, marketType)
	}
	return ``
}

var connectLock sync.Mutex

func HandleWsOrderConnFail(account *model.Account, market string, order *model.Order) {
	connectLock.Lock()
	defer func() {
		model.AppEnvironment.CrossPause = false
		connectLock.Unlock()
	}()
	model.AppEnvironment.CrossPause = true
	//兼容非order通道
	if order != nil {
		wsResp := model.WSResp{RequestId: order.ClientOrdId, Msg: fmt.Sprintf(`connection error and reconnect market %s order %#v`,
			market, order), Success: false}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`handle fail order and reconnect %s %s %s`, account.Key, market, order.ClientOrdId))
		model.AppEnvironment.WSRespChan <- wsResp
	}
	switch market {
	case model.Gate:
		if order != nil {
			_, marketType, _, _ := model.GetFromStandard(market, order.Symbol)
			WSOrderServeGate(account, marketType)
		}
	case model.OKEX:
		WsOrderServeOKEX(account)
	case model.BinancePerp, model.BinanceSpot, model.BinanceMargin:
		WsOrderServeBinance(account, market)
	case model.Bybit:
		WsOrderServeBybit(account)
		//case model.BitgetSpot, model.BitgetPerp:
		//	deprecated.WsOrderServeBitget(market, account)
	}
}

func MaintainConns(market string) {
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
		//case model.BitgetSpot, model.BitgetPerp:
		//	go deprecated.maintainConnsBitget(market, accounts)
	}
}

func SendToConnections(market string, connections map[*model.WSConn]bool, msg []byte) (err error) {
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

func UpdateOrderDeal(market, orderId, status, msg string, dealAmount float64) (find bool) {
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
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`update deal %s at %d %s %s %s %f to %f %s`,
			orderId, i, order.Market, order.Symbol, order.OrderSide, preDeal, order.DealAmount, order.Status))
		return true
	} else {
		util.Log(util.LogLevelError, fmt.Sprintf(`no order stored %s %s %s`, market, orderId, msg))
		return false
	}
}

package api

import (
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"strings"
	"sync"
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
	case model.Mexc:
		switch subType {
		case mexcContractDepthIncSubType:
			return fmt.Sprintf(`{"method":"sub.depth","param":{"symbol":"%s","compress":true}}`, dialectSymbol)
		case mexcContractDepthFullSubType:
			return fmt.Sprintf(`{"method":"sub.depth.full","param":{"symbol":"%s","limit":5}}`, dialectSymbol)
		case mexcContractTickerSubType:
			return fmt.Sprintf(`{"method":"sub.ticker","param":{"symbol":"%s"}}`, dialectSymbol)
		}
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
	socketMap map[*websocket.Conn]bool, channels []chan struct{}) {
	switch market {
	case model.BinanceSpot:
		util.Notice(" create KLine ws chan for " + market)
		socketMap, channels, _ = WsKLineBinanceSpot(environment, market, symbols)
	}
	return
}

func CreateWSTick(environment *model.Environment, market string) (
	socketMap map[*websocket.Conn]bool, channels []chan struct{}) {
	model.ChannelMaintaining.Store(market, true)
	util.Notice(" create depth chan for " + market)
	channels = make([]chan struct{}, 1)
	var err error
	switch market {
	case model.Gate:
		socketMap, channels, err = WsTickServeGateNew(market)
	case model.OKEX:
		socketMap, channels, err = WebSocketClient(market, wsOKEX, GetWSSubscribes(market, []string{model.SubscribeDepth}),
			subscribeHandlerOKEX, wsHandlerOKEX, wsStepOKEX)
	case model.BinanceSpot, model.BinanceMargin:
		socketMap, channels, err = WebSocketClient(market, wsBinance+`/stream`, GetWSSubscribes(market, []string{model.SubscribeTicker}),
			subscribeHandlerBinance, wsHandlerBinance, wsStepBinance)
	case model.BinancePerp:
		socketMap, channels, err = WebSocketClient(market, wsBinancePerp+`/stream`, GetWSSubscribes(
			market, []string{model.SubscribeTicker}), subscribeHandlerBinance, wsHandlerBinancePerp, wsStepBinance)
		mapMp, chans, _ := WebSocketClient(market, wsBinancePerp+`/stream`, GetWSSubscribes(
			market, []string{model.SubscribeMarkPrice}), subscribeHandlerBinance, wsHandlerBinancePerp, wsStepBinance)
		for conn, b := range mapMp {
			socketMap[conn] = b
		}
		for _, item := range chans {
			channels = append(channels, item)
		}
		//model.SubscribeDepth
	case model.HuobiPerp:
		socketMap, channels, err = WebSocketClient(market, wsHuobiPerp, GetWSSubscribes(model.HuobiPerp, []string{model.SubscribeDepth}),
			subscribeHandlerHuobiPerp, wsMsgHandler, wsStepHuobi)
	case model.Bybit:
		socketMap, channels, err = WsTickServeBybit(market)
	case model.BitgetSpot:
		socketMap, channels, err = WebSocketClient(market, bitgetPublic,
			GetWSSubscribes(market, []string{model.SubscribeDepth}), subscribeHandlerBitget, tickHandlerBitget, wsStepBitget)
	case model.BitgetPerp:
		socketMap, channels, err = WsTickServeBitgetPerp(market)
	}
	environment.ConnTick.Store(market, socketMap)
	environment.MsgChanTick.Store(market, channels)
	if err != nil {
		util.Notice(market + ` can not create depth server ` + err.Error())
	}
	model.AppEnvironment.WsInitTime.Store(market, util.GetNow())
	model.ChannelMaintaining.Store(market, false)
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

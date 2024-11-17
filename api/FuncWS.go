package api

import (
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"strings"
	"sync"
	"time"
)

func GetWSSubscribes(market, subType string) []interface{} {
	symbols := GetMarketSymbols(market)
	subscribes := make([]interface{}, 0)
	for symbol := range symbols {
		if len(strings.Trim(symbol, ` `)) == 0 {
			continue
		}
		subTypes := strings.Split(subType, `,`)
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
			return strings.ToLower(dialectSymbol) + `@markPrice@1s`
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

func CreateMarketKLineWS(environment *model.Environment, market string, symbols map[string]bool) (
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
	case model.KucoinSpot:
		channels, err = WsTickServeKucoinSpot()
	case model.KucoinPerp:
		channels, err = WsTickServeKucoinPerp()
	case model.Gate:
		socketMap, channels, err = WsTickServeGateNew(environment, market)
	case model.OKEX:
		socketMap, channels, err = WsTickServeOKEX(environment)
	case model.BinanceSpot, model.BinanceMargin:
		socketMap, channels, err = WsTickServeBinance(environment, market)
	case model.BinancePerp:
		socketMap, channels, err = WsTickServeBinancePerp(environment, market)
	case model.HuobiPerp:
		socketMap, channels, err = WsTickServeHuobiPerp(environment, market)
	case model.Bybit:
		socketMap, channels, err = WsTickServeBybit(environment, market)
	case model.HuobiSpot:
		socketMap, channels, err = WsTickServeHuobiSpot(environment, market)
	case model.Ftx:
		socketMap, channels, err = WsTickServeFtx(environment, market)
	case model.Mexc:
		socketMap, channels, err = WsTickServeMexc(environment, market, true)
	case model.BitgetSpot:
		socketMap, channels, err = WsTickServeBitgetSpot(environment, market)
	case model.BitgetPerp:
		socketMap, channels, err = WsTickServeBitgetPerp(environment, market)
	}
	if err != nil {
		util.Notice(market + ` can not create depth server ` + err.Error())
	}
	model.AppEnvironment.WsInitTime.Store(market, util.GetNow())
	model.ChannelMaintaining.Store(market, false)
	return socketMap, channels
}

var maintainingConnTick = sync.Map{}

func MaintainConnTick(market string) {
	value, _ := maintainingConnTick.Load(market)
	if value != nil && value.(bool) {
		return
	}
	maintainingConnTick.Store(market, true)
	switch market {
	case model.Gate:
		go func() {
			for {
				time.Sleep(time.Second * 15)
				if err := SendToAllTickerSockets(market, websocket.TextMessage, util.JsonEncodeToByte(map[string]interface{}{"time": time.Now().Unix(), "channel": "spot.ping"})); err != nil {
					util.SocketInfo(fmt.Sprintf("tick conn maintain error %s %s", market, err.Error()))
				}
				if err := SendToAllTickerSockets(market, websocket.TextMessage, util.JsonEncodeToByte(map[string]interface{}{"time": time.Now().Unix(), "channel": "futures.ping"})); err != nil {
					util.SocketInfo(fmt.Sprintf("tick conn maintain error %s %s", market, err.Error()))
				}
			}
		}()
	case model.OKEX:
		subscribes := GetWSSubscribes(market, model.SubscribeDepth)
		go func() {
			for {
				time.Sleep(time.Minute * 5)
				reSubscribe(subscribes)
			}
		}()
		go func() {
			for {
				time.Sleep(time.Second * 25)
				if err := SendToAllTickerSockets(market, websocket.TextMessage, []byte(`ping`)); err != nil {
					util.SocketInfo(fmt.Sprintf("tick conn maintain error %s %s", market, err.Error()))
				}
			}
		}()
	case model.BinanceSpot, model.BinancePerp, model.BinanceMargin:
		go func() {
			for {
				time.Sleep(time.Minute * 5)
				if err := SendToAllTickerSockets(market, websocket.PongMessage, []byte(`ping`)); err != nil {
					util.SocketInfo(fmt.Sprintf("tick conn maintain error %s %s", market, err.Error()))
				}
			}
		}()
	case model.Bybit:
		go func() {
			for {
				time.Sleep(time.Second * 20)
				if err := SendToAllTickerSockets(market, websocket.TextMessage, []byte(`{"req_id": "100001", "op": "ping"}`)); err != nil {
					util.SocketInfo(fmt.Sprintf("tick conn maintain error %s %s", market, err.Error()))
				}
			}
		}()
	case model.BitgetSpot, model.BitgetPerp:
		go func() {
			for {
				time.Sleep(time.Second * 20)
				if err := SendToAllTickerSockets(market, websocket.TextMessage, []byte(`ping`)); err != nil {
					util.SocketInfo(fmt.Sprintf("tick conn maintain error %s %s", market, err.Error()))
				}
			}
		}()
	}
}

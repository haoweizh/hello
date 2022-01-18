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

const restBybit = `https://api.bybit.com`
const wsBybitPerp = `wss://stream.bybit.com/realtime_public`
const wsStepBybit = 1

var socketLockBybit sync.Mutex

func maintainChannelFtx(subscribes []interface{}) {
	if !channelMaintainingFtx {
		channelMaintainingFtx = true
		for true {
			time.Sleep(time.Minute)
			for _, value := range subscribes {
				subscribe := value.([]string)
				_, bidAsk := model.AppMarkets.GetBidAsk(subscribe[1], model.Ftx)
				now := time.Now().UnixNano() / int64(time.Millisecond)
				if bidAsk == nil || now-int64(bidAsk.Ts) > 60000 {
					subCmd := fmt.Sprintf(`{"op": "subscribe", "channel": "%s", "market": "%s"}`,
						subscribe[0], subscribe[1])
					if ftxSymbolConnection[subscribe[1]] != nil {
						if err := sendToConnection(model.Ftx, ftxSymbolConnection[subscribe[1]], []byte(subCmd)); err != nil {
							util.SocketInfo("ftx can not resubscribe " + err.Error())
						}
					} else {
						util.Notice(`ftx can not get connection for %s`, subscribe[1])
					}
					util.Notice(`send resubscribe %s`, subCmd)
				}
			}
		}
	}
}

var subscribeHandlerBybit = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	subscribeMap := make(map[string]interface{})
	subscribeMap[`event`] = `sub`
	subscribeMap[`topic`] = `depth`
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = sendToConnection(model.BybitPerp, connection, subscribeMessage); err != nil {
		util.SocketInfo("bybit can not subscribe " + err.Error())
		return err
	}
	return err
}

func WsDepthServeBybit(markets *model.Markets, orderHandler OrderHandler) ([]chan struct{}, error) {
	lastPingTime := util.GetNow().Unix()
	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		socketLockBybit.Lock()
		defer socketLockBybit.Unlock()
		now := util.GetNow()
		if now.Unix()-lastPingTime > 30 { // ping ws server every 5 seconds
			lastPingTime = util.GetNow().Unix()
			if err := sendToAllConnections(model.BybitPerp, []byte(`{"op":"ping"}`)); err != nil {
				util.SocketInfo("bybit server ping client error " + err.Error())
			}
		}
		if len(event) == 0 {
			return
		}
		depthJson, depthErr := util.NewJSON(event)
		if depthJson == nil {
			return
		}
		topic := depthJson.Get(`topic`).MustString()
		ts := depthJson.Get(`timestamp_e6`).MustInt64()
		if depthErr != nil {
			util.SocketInfo(`bybit parse err` + string(event))
			return
		}
		if strings.Contains(topic, `orderBookL2_25.`) {
			//util.SocketInfo(string(event))
			symbol := model.GetStandardSymbol(model.Bybit, topic[strings.LastIndex(topic, `.`)+1:])
			handleOrderBookBybit(markets, symbol, ts, depthJson)
		} else if topic == `position` {
		}
	}
	subscribes := GetWSSubscribes(model.BybitPerp, model.SubscribeDepth)
	return WebSocketClient(model.Bybit, wsBybit, subscribes, subscribeHandlerBybit, wsHandler, orderHandler, wsStepBybit)
}

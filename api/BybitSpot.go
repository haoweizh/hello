package api

import (
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"strings"
)

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
			if err := sendToAllConnections(model.Bybit, []byte(fmt.Sprintf(`{"ping":%d}`, now.UnixMilli()))); err != nil {
				util.SocketInfo("bybit server ping client error " + err.Error())
			}
			if err := sendToAllConnections(model.Bybit, []byte(`{"op":"ping"}`)); err != nil {
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
	subscribes := GetWSSubscribes(model.Bybit, model.SubscribeDepth)
	return WebSocketClient(model.Bybit, wsBybit, subscribes, subscribeHandlerBybit, wsHandler, orderHandler, wsStepBybit)
}

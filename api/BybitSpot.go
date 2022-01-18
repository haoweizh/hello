package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"strings"
	"time"
)

const wsBybitSpot = `wss://stream.bybit.com/spot/quote/ws/v2`
const wsStepBybitSpot = 20

var bybitSpotSubConnection = make(map[string]*websocket.Conn)
var channelMaintainingBybitSpot = false

func maintainChannelBybitSpot(subscribes []interface{}) {
	if !channelMaintainingBybitSpot {
		channelMaintainingBybitSpot = true
		for true {
			time.Sleep(time.Minute)
			for _, value := range subscribes {
				_, bidAsk := model.AppMarkets.GetBidAsk(value.(string), model.BybitPerp)
				now := time.Now().UnixNano() / int64(time.Millisecond)
				if bidAsk == nil || now-int64(bidAsk.Ts) > 60000 {
					subCmd := fmt.Sprintf(
						`{"topic":"depth","event":"sub","params":{"symbol":"%s","binary":false}}`, value.(string))
					if bybitPerpSubConnection[value.(string)] != nil {
						if err := SendToConnection(model.BybitSpot, bybitSpotSubConnection[value.(string)],
							[]byte(subCmd)); err != nil {
							util.SocketInfo("bybitSpot can not resubscribe " + err.Error())
						}
					} else {
						util.Notice(`bybitPerp can not get connection for %s`, value.(string))
					}
					util.Notice(`send resubscribe %s`, subCmd)
				}
			}
		}
	}
}

var subscribeHandlerBybitSpot = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	expire := util.GetNowUnixMillion() + 1000
	toBeSign := fmt.Sprintf(`GET/realtime%d`, expire)
	account := model.AppConfig.GetAccounts(model.BybitPerp)[0]
	hash := hmac.New(sha256.New, []byte(account.Secret))
	hash.Write([]byte(toBeSign))
	sign := hex.EncodeToString(hash.Sum(nil))
	authCmd := fmt.Sprintf(`{"op": "auth", "args": ["%s", %d, "%s"]}`, account.Key, expire, sign)
	if err = SendToConnection(model.BybitPerp, connection, []byte(authCmd)); err != nil {
		util.SocketInfo("bybitSpot can not auth " + err.Error())
	}
	for _, subscribe := range subscribes {
		subscribeMessage := fmt.Sprintf(
			`{"topic":"depth","event":"sub","params":{"symbol":"%s","binary":false}}`, subscribe)
		if err = SendToConnection(model.BybitSpot, connection, []byte(subscribeMessage)); err != nil {
			util.SocketInfo("bybitSpot can not subscribe " + err.Error())
			return err
		}
		bybitPerpSubConnection[subscribe.(string)] = connection
	}
	return err
}

func WsDepthServeBybitSpot(markets *model.Markets, orderHandler OrderHandler) ([]chan struct{}, error) {
	lastPingTime := util.GetNow().Unix()
	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		now := util.GetNow()
		if now.Unix()-lastPingTime > 30 { // ping ws server every 5 seconds
			lastPingTime = util.GetNow().Unix()
			if err := SendToAllConnections(model.BybitSpot, []byte(fmt.Sprintf(`{"ping":%d}`,
				now.UnixNano()/int64(time.Millisecond)))); err != nil {
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
			symbol := model.GetStandardSymbol(model.BybitSpot, topic[strings.LastIndex(topic, `.`)+1:])
			fmt.Sprintf(`%s %d`, symbol, ts)
			//handleOrderBookBybit(markets, symbol, ts, depthJson)
		} else if topic == `position` {
		}
	}
	subscribes := GetWSSubscribes(model.BybitSpot, model.SubscribeDepth)
	bybitSpotSubConnection = make(map[string]*websocket.Conn)
	return WebSocketClient(model.BybitSpot, wsBybitSpot, subscribes, subscribeHandlerBybitSpot, wsHandler,
		orderHandler, wsStepBybitSpot)
}

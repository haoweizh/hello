package api

import (
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"net/http"
	"strings"
	"sync"
	"time"
)

type OrderHandler func(order *model.Order)
type MsgHandler func(message []byte)
type AccountMsgHandler func(market, key string, message []byte)
type SubscribeHandler func(market string, connection *websocket.Conn, subscribes []interface{}) error

var wsLock sync.Mutex

func SendToConnection(market string, connection *websocket.Conn, msg []byte) (err error) {
	defer wsLock.Unlock()
	wsLock.Lock()
	if connection == nil {
		util.Notice(`fail to write to nil connection`)
		return
	}
	if err = connection.WriteMessage(websocket.TextMessage, msg); err != nil {
		SetRequireReset(market)
		util.Notice(`fail to write to connection ` + market + string(msg) + err.Error())
	}
	return err
}

func SendToAllTickerSockets(market string, msg []byte) (err error) {
	defer wsLock.Unlock()
	wsLock.Lock()
	value, _ := model.AppEnvironment.SocketsTick.Load(market)
	if value == nil {
		return
	}
	connections := value.(map[*websocket.Conn]bool)
	for connection := range connections {
		if connection == nil {
			continue
		}
		if err = connection.WriteMessage(websocket.TextMessage, msg); err != nil {
			SetRequireReset(market)
			util.Notice(fmt.Sprintf(`fail to write to all connection %s %s return: %s`, market, msg, err.Error()))
		}
	}
	return err
}

func PongAllConnectionsInterval(market string, milliseconds int) (err error) {
	value, _ := model.AppEnvironment.SocketsTick.Load(market)
	if value == nil {
		return
	}
	connections := value.(map[*websocket.Conn]bool)
	for connection := range connections {
		if connection == nil {
			continue
		}
		deadline := time.Now().Add(5 * time.Second)
		if writeError := connection.WriteControl(websocket.PongMessage, []byte{}, deadline); writeError != nil {
			util.Notice(fmt.Sprintf(`fail to pong connection return: %s`, writeError.Error()))
			SetRequireReset(market)
			err = writeError
		}
		time.Sleep(time.Millisecond * time.Duration(milliseconds))
	}
	return err
}

func newConnection(url string) (*websocket.Conn, error) {
	var connErr error
	var c *websocket.Conn
	//for i := 0; i < 10; i++ {
	util.SocketInfo("try to connect " + url)
	dialer := &websocket.Dialer{
		Proxy: http.ProxyFromEnvironment,
		//EnableCompression: true,
	}
	c, _, connErr = dialer.Dial(url, nil)
	if connErr == nil {
		//	break
		if c != nil {
			c.EnableWriteCompression(true)
			//c.SetCompressionLevel()
		}
	} else {
		util.SocketInfo(`can not create new connection ` + connErr.Error())
		if c != nil {
			_ = c.Close()
		}
	}
	//	time.Sleep(1000)
	//}
	if connErr != nil {
		return nil, connErr
	}
	return c, nil
}

func chanHandler(market string, stopChan chan struct{}, connection *websocket.Conn, msgHandler MsgHandler) {
	defer func() {
		err := connection.Close()
		if err != nil {
			util.Notice(fmt.Sprintf(`connection closed %s`, err.Error()))
		}
	}()
	for {
		select {
		case <-stopChan:
			//util.Info("get stop struct, return")
			return
		default:
			_, message, err := connection.ReadMessage()
			if err != nil {
				if !strings.Contains(err.Error(), `EOF`) {
					SetRequireReset(market)
					util.Notice(fmt.Sprintf(`%s can not read from websocket: %s`, market, err.Error()))
				}
				return
			}
			//util.SocketInfo(string(message))
			msgHandler(message)
		}
	}
}

func WsAccountClient(market, key, url string, accountMsgHandler AccountMsgHandler) (connection *websocket.Conn, err error) {
	util.Notice(market + ` create account channel ` + url)
	connection, err = newConnection(url)
	if err != nil {
		util.Info("can not create web socket" + err.Error())
		return nil, err
	}
	util.StoreSyncMap(&model.AppEnvironment.AccountConns, connection, market, key)
	go func() {
		for {
			_, message, readErr := connection.ReadMessage()
			if readErr != nil {
				util.DelSyncMap(&model.AppEnvironment.AccountConns, market, key)
				closeErr := connection.Close()
				if closeErr != nil {
					util.Notice(fmt.Sprintf(`connection closed %s`, closeErr.Error()))
				}
				util.Notice(fmt.Sprintf(`%s can not read from account ws: %s`, market, readErr.Error()))
				return
			}
			if accountMsgHandler != nil {
				accountMsgHandler(market, key, message)
			}
		}
	}()
	return connection, nil
}

func WebSocketClient(market, url string, subscribes []interface{}, subHandler SubscribeHandler,
	msgHandler MsgHandler, step int) (socketMap map[*websocket.Conn]bool, msgChans []chan struct{}, connectErr error) {
	util.Notice(market + ` create depth channel ` + url)
	socketMap = make(map[*websocket.Conn]bool)
	msgChans = make([]chan struct{}, 0)
	var stepSubscribes []interface{}
	for i := 0; subscribes != nil && i*step < len(subscribes); i++ {
		if (i+1)*step < len(subscribes) {
			stepSubscribes = subscribes[i*step : (i+1)*step]
		} else {
			stepSubscribes = subscribes[i*step:]
		}
		connection, err := newConnection(url)
		if err != nil || connection == nil {
			if err != nil {
				util.SocketInfo(fmt.Sprintf("can not create web socket %s %s %s", market, url, err.Error()))
			}
			return nil, nil, err
		}
		connection.SetPingHandler(func(appData string) error {
			//fmt.Println(fmt.Sprintf("Ping received, sending Pong %s %s", appData, time.Now().String()))
			return connection.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Minute))
		})
		stopChan := make(chan struct{}, 2)
		go chanHandler(market, stopChan, connection, msgHandler)
		if subHandler != nil {
			_ = subHandler(market, connection, stepSubscribes)
		}
		msgChans = append(msgChans, stopChan)
		socketMap[connection] = true
	}
	util.Info(fmt.Sprintf(`ws client add conns %s sockets %d msgChans %d`, market, len(socketMap), len(msgChans)))
	return
}

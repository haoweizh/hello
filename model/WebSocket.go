package model

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"hello/util"
	"strings"
	"sync"
	"time"
)

type OrderHandler func(order *Order)
type MsgHandler func(market string, conn *WSConn, message []byte)
type AccountMsgHandler func(market, key string, message []byte)
type SubscribeHandler func(market string, connection *WSConn, subscribes []interface{}) error

var wsLock sync.Map // market - *sync.Mutex

type WSConn struct {
	Conn *websocket.Conn
}

func (wsConn *WSConn) WriteMsg(msg []byte) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if wsConn.Conn == nil {
		return fmt.Errorf(`nil Conn`)
	}
	return wsConn.Conn.Write(ctx, websocket.MessageText, msg)
}

func (wsConn *WSConn) WriteJson(body map[string]interface{}) (err error) {
	jsonData, _ := json.Marshal(body)
	return wsConn.WriteMsg(jsonData)
}

func SendToConnection(market string, connection *WSConn, msg []byte) (err error) {
	lock, _ := wsLock.Load(market)
	if lock == nil {
		lock = &sync.Mutex{}
		wsLock.Store(market, lock)
	}
	defer lock.(*sync.Mutex).Unlock()
	lock.(*sync.Mutex).Lock()
	if connection == nil {
		util.Notice(`fail to write to nil connection`)
		return
	}
	if err = connection.WriteMsg(msg); err != nil {
		util.Notice(`fail to write to connection ` + market + string(msg) + err.Error())
	}
	return err
}

func SendToConnections(market string, connections map[*WSConn]bool, msg []byte) (err error) {
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
			util.Notice(fmt.Sprintf(`fail to write to all connection %s %s return: %s`, market, msg, err.Error()))
		}
		//if msgType == websocket.MessageText {
		//	if err = connection.WriteMessage(msgType, msg); err != nil {
		//	}
		//} else if msgType == websocket.PongMessage || msgType == websocket.PingMessage {
		//	if err = connection.WriteMessage(msgType, msg); err != nil {
		//	}
		//}
		//time.Sleep(time.Millisecond * 100)
	}
	return err
}

func newConnection(url string) (conn *WSConn, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c, _, dialErr := websocket.Dial(ctx, url, &websocket.DialOptions{})
	if dialErr == nil {
		return &WSConn{Conn: c}, nil
	}
	return nil, dialErr
}

//func newConnection(url string) (*websocket.Conn, error) {
//	var connErr error
//	var c *websocket.Conn
//	util.SocketInfo("try to connect " + url)
//	dialer := &websocket.Dialer{
//		Proxy:          http.ProxyFromEnvironment,
//		ReadBufferSize: 1024 * 16,
//	}
//	c, _, connErr = dialer.Dial(url, nil)
//	if connErr == nil {
//		if c != nil {
//			c.EnableWriteCompression(true)
//			c.SetReadLimit(1024 * 1024 * 128)
//		}
//	} else {
//		util.SocketInfo(`can not create new connection ` + connErr.Error())
//		if c != nil {
//			_ = c.Close()
//		}
//	}
//	if connErr != nil {
//		return nil, connErr
//	}
//	return c, nil
//}

func chanHandler(market string, stopChan chan struct{}, connection *WSConn, msgHandler MsgHandler) {
	defer func() {
		err := connection.Conn.Close(websocket.StatusNormalClosure, "")
		if err != nil {
			util.Notice(fmt.Sprintf(`connection closed %s %s`, market, err.Error()))
		}
	}()
	for {
		select {
		case <-stopChan:
			util.Notice("get stop struct, return")
			return
		default:
			msgType, message, err := connection.Conn.Read(context.Background())
			if err != nil {
				if !strings.Contains(err.Error(), `EOF`) {
					//SetRequireReset(market)
					util.Notice(fmt.Sprintf(`%s can not read from websocket: %s`, market, err.Error()))
				}
				return
			}
			if msgType == websocket.MessageText {
				msgHandler(market, connection, message)
			}
		}
	}
}

func WsAccountClient(market, key, url string, accountMsgHandler AccountMsgHandler) (connection *WSConn, err error) {
	util.Notice(market + ` create account channel ` + url)
	connection, err = newConnection(url)
	if err != nil {
		util.Info("can not create web socket" + err.Error())
		return nil, err
	}
	//connection.SetPingHandler(func(appData string) error {
	//	accountMsgHandler(market, key, []byte(`ping pong received`))
	//	return connection.WriteMessage(websocket.PongMessage, []byte(appData))
	//})
	go func() {
		defer func() {
			closeErr := connection.Conn.Close(websocket.StatusNormalClosure, "")
			if closeErr != nil {
				util.Notice(fmt.Sprintf(`connection closed %s`, closeErr.Error()))
			}
		}()
		for {
			_, message, readErr := connection.Conn.Read(context.Background())
			if readErr != nil {
				util.DelSyncMap(&AppEnvironment.ConnOrder, market, key)
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
	msgHandler MsgHandler, step int) (socketMap map[*WSConn]bool, msgChans []chan struct{}, connectErr error) {
	util.Notice(market + ` create depth channel ` + url)
	socketMap = make(map[*WSConn]bool)
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
		//connection.SetPingHandler(func(appData string) error {
		//	errPing := connection.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second*60))
		//	if errPing != nil {
		//		util.Notice(fmt.Sprintf(`fail to handle ping %s %s %s`, market, url, errPing.Error()))
		//	}
		//	return errPing
		//})
		stopChan := make(chan struct{}, 2)
		go chanHandler(market, stopChan, connection, msgHandler)
		if subHandler != nil {
			_ = subHandler(market, connection, stepSubscribes)
		}
		msgChans = append(msgChans, stopChan)
		socketMap[connection] = true
		time.Sleep(time.Millisecond * 100)
	}
	util.Info(fmt.Sprintf(`ws client add conns %s sockets %d msgChans %d`, market, len(socketMap), len(msgChans)))
	return
}

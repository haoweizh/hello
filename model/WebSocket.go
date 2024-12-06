package model

import (
	"encoding/json"
	"fmt"
	//"github.com/coder/websocket"
	"github.com/gorilla/websocket"
	"hello/util"
	"net/http"
	"strings"
	"time"
)

type OrderHandler func(order *Order)
type MsgHandler func(market string, conn *WSConn, message []byte)
type AccountMsgHandler func(market, key string, message []byte)
type SubscribeHandler func(market string, connection *WSConn, subscribes []interface{}) error

type WSConn struct {
	Conn *websocket.Conn
}

func (wsConn *WSConn) Close() {
	err := wsConn.Conn.Close()
	if err != nil {
		util.Log(util.LogLevelError, `close conn err `+err.Error())
		return
	}
}

func (wsConn *WSConn) WriteMsg(msg []byte) (err error) {
	//ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	//defer cancel()
	if wsConn.Conn == nil {
		return fmt.Errorf(`nil Conn`)
	}
	//return wsConn.Conn.Write(ctx, websocket.MessageText, msg)
	return wsConn.Conn.WriteMessage(websocket.TextMessage, msg)
}

func (wsConn *WSConn) WriteJson(body map[string]interface{}) (err error) {
	jsonData, _ := json.Marshal(body)
	return wsConn.WriteMsg(jsonData)
}

//func newWsCoder(url string) (conn *WSConn, err error) {
//	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
//	defer cancel()
//	c, _, dialErr := websocket.Dial(ctx, url, &websocket.DialOptions{})
//	if dialErr == nil {
//		return &WSConn{Conn: c}, nil
//	}
//	return nil, dialErr
//}

func newWsGorilla(url string) (*WSConn, error) {
	var connErr error
	var c *websocket.Conn
	util.Log(util.LogLevelInfo, "try to connect "+url)
	dialer := &websocket.Dialer{
		Proxy:          http.ProxyFromEnvironment,
		ReadBufferSize: 1024 * 32,
	}
	c, _, connErr = dialer.Dial(url, nil)
	if connErr == nil {
		if c != nil {
			c.EnableWriteCompression(true)
			c.SetReadLimit(1024 * 1024 * 128)
			c.SetPingHandler(func(appData string) error {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`%s ping received %s`, url, appData))
				return c.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second*10))
			})
		}
	} else {
		util.Log(util.LogLevelError, `can not create new connection `+connErr.Error())
		if c != nil {
			_ = c.Close()
		}
	}
	if connErr != nil {
		return nil, connErr
	}
	return &WSConn{Conn: c}, nil
}

func chanHandler(market string, stopChan chan struct{}, connection *WSConn, msgHandler MsgHandler) {
	defer func() {
		//err := connection.Conn.Close(websocket.StatusNormalClosure, "")
		err := connection.Conn.Close()
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`connection closed %s %s`, market, err.Error()))
		}
	}()
	select {
	case <-stopChan:
		util.Log(util.LogLevelInfo, "get stop struct, return")
		return
	default:
		//msgType, message, err := connection.Conn.Read(context.Background())
		msgType, message, err := connection.Conn.ReadMessage()
		if err != nil {
			if !strings.Contains(err.Error(), `EOF`) {
				util.Log(util.LogLevelError, fmt.Sprintf(`%s can not read from websocket: %s`, market, err.Error()))
			}
			return
		}
		//if msgType == websocket.MessageText {
		if msgType == websocket.TextMessage {
			msgHandler(market, connection, message)
		}
	}
}

func WsAccountClient(market, key, url string, accountMsgHandler AccountMsgHandler) (connection *WSConn, err error) {
	util.Log(util.LogLevelInfo, market+` create account channel `+url)
	connection, err = newWsGorilla(url)
	if err != nil {
		util.Log(util.LogLevelError, url+"can not create web socket"+err.Error())
		return nil, err
	}
	go func() {
		defer func() {
			//closeErr := connection.Conn.Close(websocket.StatusNormalClosure, "")
			closeErr := connection.Conn.Close()
			if closeErr != nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`%s connection closed %s`, url, closeErr.Error()))
			}
		}()
		for {
			//_, message, readErr := connection.Conn.Read(context.Background())
			_, message, readErr := connection.Conn.ReadMessage()
			if readErr != nil {
				util.DelSyncMap(&AppEnvironment.ConnOrder, market, key)
				util.Log(util.LogLevelError, fmt.Sprintf(`%s %s can not read from account ws: %s`, market, url, readErr.Error()))
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
	util.Log(util.LogLevelInfo, market+` create depth channel `+url)
	socketMap = make(map[*WSConn]bool)
	msgChans = make([]chan struct{}, 0)
	var stepSubscribes []interface{}
	for i := 0; subscribes != nil && i*step < len(subscribes); i++ {
		if (i+1)*step < len(subscribes) {
			stepSubscribes = subscribes[i*step : (i+1)*step]
		} else {
			stepSubscribes = subscribes[i*step:]
		}
		connection, err := newWsGorilla(url)
		if err != nil || connection == nil {
			if err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("can not create web socket %s %s %s", market, url, err.Error()))
			}
			return nil, nil, err
		}
		stopChan := make(chan struct{}, 2)
		go chanHandler(market, stopChan, connection, msgHandler)
		if subHandler != nil {
			_ = subHandler(market, connection, stepSubscribes)
		}
		msgChans = append(msgChans, stopChan)
		socketMap[connection] = true
		time.Sleep(time.Millisecond * 100)
	}
	util.Log(util.LogLevelInfo,
		fmt.Sprintf(`ws client add conns %s sockets %d msgChans %d`, market, len(socketMap), len(msgChans)))
	return
}

package api

import (
	"encoding/json"
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
type WSMsgHandler func(client *WSClient, message []byte)
type SubscribeHandler func(market string, connection *websocket.Conn, subscribes []interface{}) error

var wsLock sync.Mutex

var AppWSManager = WSManager{
	Register:   make(chan *WSClient),
	Unregister: make(chan *WSClient),
	Clients:    make(map[*WSClient]bool),
}

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

func SendToAllConnections(market string, msg []byte) (err error) {
	defer wsLock.Unlock()
	wsLock.Lock()
	value, _ := model.AppMarkets.Connections.Load(market)
	if value == nil {
		return
	}
	connections := value.([]*websocket.Conn)
	for i, connection := range connections {
		if connection == nil {
			continue
		}
		if err = connection.WriteMessage(websocket.TextMessage, msg); err != nil {
			SetRequireReset(market)
			util.Notice(fmt.Sprintf(`fail to write to all connection %s %d %s return: %s`,
				market, i, msg, err.Error()))
		}
	}
	return err
}

func PongAllConnectionsInterval(market string, milliseconds int) (err error) {
	value, _ := model.AppMarkets.Connections.Load(market)
	if value == nil {
		return
	}
	connections := value.([]*websocket.Conn)
	for i, connection := range connections {
		if connection == nil {
			continue
		}
		deadline := time.Now().Add(5 * time.Second)
		if writeError := connection.WriteControl(websocket.PongMessage, []byte{}, deadline); writeError != nil {
			util.Notice(fmt.Sprintf(`fail to pong connection %d return: %s`, i, writeError.Error()))
			SetRequireReset(market)
			if writeError != nil {
				err = writeError
			}
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
				if market != model.Bybit {
					SetRequireReset(market)
				}
				util.Notice(fmt.Sprintf(`%s can not read from websocket: %s`, market, err.Error()))
				return
			}
			//util.SocketInfo(string(message))
			msgHandler(message)
		}
	}
}

func WsAccountClient(key, market, url string, msgHandler MsgHandler) (connection *websocket.Conn, err error) {
	util.Notice(market + ` create account channel ` + url)
	util.DelSyncMap(&model.AppMarkets.AccountConns, market, key)
	connection, err = newConnection(url)
	if err != nil {
		util.SocketInfo("can not create web socket" + err.Error())
		return nil, err
	}
	util.StoreSyncMap(&model.AppMarkets.AccountConns, connection, market, key)
	go func() {
		for {
			_, message, readErr := connection.ReadMessage()
			if readErr != nil {
				util.DelSyncMap(&model.AppMarkets.AccountConns, market, key)
				closeErr := connection.Close()
				if closeErr != nil {
					util.Notice(fmt.Sprintf(`connection closed %s`, closeErr.Error()))
				}
				util.Notice(fmt.Sprintf(`%s can not read from account ws: %s`, market, readErr.Error()))
				return
			}
			if msgHandler != nil {
				msgHandler(message)
			}
		}
	}()
	return connection, nil
}

func WebSocketClient(market, url string, subscribes []interface{}, subHandler SubscribeHandler,
	msgHandler MsgHandler, step int) (stopChans []chan struct{}, connectErr error) {
	util.Notice(market + ` create depth channel ` + url)
	connections := make([]*websocket.Conn, 0)
	model.AppMarkets.Connections.Delete(market)
	stopChans = make([]chan struct{}, 0)
	var stepSubscribes []interface{}
	for i := 0; subscribes != nil && i*step < len(subscribes); i++ {
		if (i+1)*step < len(subscribes) {
			stepSubscribes = subscribes[i*step : (i+1)*step]
		} else {
			stepSubscribes = subscribes[i*step:]
		}
		connection, err := newConnection(url)
		stopChan := make(chan struct{}, 2)
		if err != nil {
			util.SocketInfo("can not create web socket" + err.Error())
			return nil, connectErr
		}
		go chanHandler(market, stopChan, connection, msgHandler)
		if subHandler != nil {
			_ = subHandler(market, connection, stepSubscribes)
		}
		stopChans = append(stopChans, stopChan)
		connections = append(connections, connection)
	}
	model.AppMarkets.Connections.Store(market, connections)
	return
}

type WSManager struct {
	Clients    map[*WSClient]bool
	Register   chan *WSClient
	Unregister chan *WSClient
}

type WSClient struct {
	ID        string
	Socket    *websocket.Conn
	ChanWrite chan []byte
	ChanRead  chan []byte
	Manager   *WSManager
	Pinged    bool
	Timer     *time.Timer
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

func (manager *WSManager) Start() {
	for {
		select {
		case conn := <-manager.Register:
			manager.Clients[conn] = true
			jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
			manager.Send(jsonMessage, conn)
			fmt.Println(fmt.Sprintf(`after registerd %d`, len(manager.Clients)))
		case conn := <-manager.Unregister:
			if _, ok := manager.Clients[conn]; ok {
				conn.Close()
				delete(manager.Clients, conn)
				jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
				manager.Send(jsonMessage, conn)
				fmt.Println(fmt.Sprintf(`after unregister %d`, len(manager.Clients)))
			}
		}
	}
}

func (manager *WSManager) Send(message []byte, ignore *WSClient) {
	for conn := range manager.Clients {
		if conn != ignore {
			conn.ChanWrite <- message
		}
	}
}

func (c *WSClient) Close() {
	close(c.ChanRead)
	close(c.ChanWrite)
	_ = c.Socket.Close()
}

func (c *WSClient) Read(msgHandler WSMsgHandler) {
	defer func() {
		c.Manager.Unregister <- c
	}()
	go func() {
		for true {
			_, message, err := c.Socket.ReadMessage()
			if err != nil {
				break
			}
			c.ChanRead <- message
		}
	}()
	for {
		select {
		case message, ok := <-c.ChanRead:
			if !ok {
				return
			}
			jsonMessage, _ := json.Marshal(&Message{Sender: c.ID, Content: string(message)})
			if strings.Contains(string(jsonMessage), `ping`) {
				c.Pinged = true
			}
			if msgHandler != nil {
				msgHandler(c, jsonMessage)
			}
		case <-c.Timer.C:
			c.Timer.Reset(3600 * time.Second)
			if c.Pinged {
				c.Pinged = false
			} else {
				fmt.Println(`time out no ping`)
				return
			}
		}
	}
}

func (c *WSClient) Write() {
	defer func() {
		c.Manager.Unregister <- c
	}()
	for {
		select {
		case message, ok := <-c.ChanWrite:
			if !ok {
				return
			}
			err := c.Socket.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				util.Notice(fmt.Sprintf(`fail to send ws msg %s return %s`, message, err.Error()))
			}
		}
	}
}

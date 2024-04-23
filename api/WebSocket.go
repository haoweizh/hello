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
type WSMsgHandler func(client *WSAgent, message []byte)
type SubscribeHandler func(market string, connection *websocket.Conn, subscribes []interface{}) error

var wsLock sync.Mutex

var AppWSManager = WSManager{
	Register:    make(chan *WSAgent),
	Unregister:  make(chan *WSAgent),
	Connections: make(map[*WSAgent]bool),
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
			util.Info(fmt.Sprintf(`fail to write to all connection %s %s return: %s`, market, msg, err.Error()))
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

func WsAccountClient(key, market, url string, msgHandler MsgHandler) (connection *websocket.Conn, err error) {
	util.Notice(market + ` create account channel ` + url)
	util.DelSyncMap(&model.AppEnvironment.AccountConns, market, key)
	connection, err = newConnection(url)
	if err != nil {
		util.SocketInfo("can not create web socket" + err.Error())
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
				value, _ := model.AppEnvironment.SocketsTick.Load(market)
				if value != nil {
					delete(value.(map[*websocket.Conn]bool), connection)
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

type WSManager struct {
	Connections map[*WSAgent]bool
	Register    chan *WSAgent
	Unregister  chan *WSAgent
}

type WSAgent struct {
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
			manager.Connections[conn] = true
			jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
			manager.Send(jsonMessage, conn)
			util.Info(fmt.Sprintf(`after registerd %d`, len(manager.Connections)))
		case agent := <-manager.Unregister:
			if _, ok := manager.Connections[agent]; ok {
				agent.Close()
				delete(manager.Connections, agent)
				jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
				manager.Send(jsonMessage, agent)
				util.Info(fmt.Sprintf(`after unregister %d`, len(manager.Connections)))
			}
		}
	}
}

func (manager *WSManager) Send(message []byte, ignore *WSAgent) {
	for conn := range manager.Connections {
		if conn != ignore {
			conn.ChanWrite <- message
		}
	}
}

func (agent *WSAgent) Close() {
	close(agent.ChanRead)
	close(agent.ChanWrite)
	_ = agent.Socket.Close()
}

func (agent *WSAgent) Read(msgHandler WSMsgHandler) {
	defer func() {
		agent.Manager.Unregister <- agent
	}()
	go func() {
		for {
			_, message, err := agent.Socket.ReadMessage()
			if err != nil {
				break
			}
			agent.ChanRead <- message
		}
	}()
	for {
		select {
		case message, ok := <-agent.ChanRead:
			if !ok {
				return
			}
			jsonMessage, _ := json.Marshal(&Message{Sender: agent.ID, Content: string(message)})
			if strings.Contains(string(jsonMessage), `ping`) {
				agent.Pinged = true
			}
			if msgHandler != nil {
				msgHandler(agent, jsonMessage)
			}
		case <-agent.Timer.C:
			agent.Timer.Reset(300 * time.Second)
			if agent.Pinged {
				agent.Pinged = false
			} else {
				agent.Manager.Unregister <- agent
				util.Info(`time out without ping`)
				return
			}
		}
	}
}

func (agent *WSAgent) Write() {
	defer func() {
		agent.Manager.Unregister <- agent
	}()
	for {
		select {
		case message, ok := <-agent.ChanWrite:
			if !ok {
				return
			}
			err := agent.Socket.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				util.Notice(fmt.Sprintf(`fail to send ws msg %s return %s`, message, err.Error()))
			}
		}
	}
}

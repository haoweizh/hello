package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"hello/model"
	"hello/util"
	"net/http"
	"time"
)

type MsgHandler func(message []byte)
type WSMsgHandler func(client *WSClient, message []byte)
type SubscribeHandler func(subscribes []interface{}, subType string) error
type ErrHandler func(err error)

var AppWSManager = WSManager{
	Register:   make(chan *WSClient),
	Unregister: make(chan *WSClient),
	Clients:    make(map[*WSClient]bool),
}

func sendToWs(market string, msg []byte) (err error) {
	if model.AppMarkets.GetIsWriting(market) {
		return errors.New(fmt.Sprintf(`conn %s is writing`, market))
	}
	model.AppMarkets.SetIsWriting(market, true)
	defer model.AppMarkets.SetIsWriting(market, false)
	conn := model.AppMarkets.GetConn(market)
	if conn == nil {
		return errors.New(fmt.Sprintf(`conn %s is nil`, market))
	}
	return conn.WriteMessage(websocket.TextMessage, msg)
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

func chanHandler(market string, stopC chan struct{}, errHandler ErrHandler, msgHandler MsgHandler) {
	conn := model.AppMarkets.GetConn(market)
	defer func() {
		err := conn.Close()
		util.Notice(`connection closed`)
		if err != nil {
			errHandler(err)
		}
	}()
	for true {
		select {
		case <-stopC:
			util.Notice("get stop struct, return")
			return
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				util.Notice(market + " can not read from websocket: " + err.Error())
				return
			}
			//util.SocketInfo(string(message))
			msgHandler(message)
		}
	}
}

func WebSocketClient(market, url, subType string, subscribes []interface{}, subHandler SubscribeHandler,
	msgHandler MsgHandler, errHandler ErrHandler) (chan struct{}, error) {
	util.Notice(market + `creat depth channel ` + url)
	conn, err := newConnection(url)
	if err != nil {
		util.SocketInfo("can not create web socket" + err.Error())
		errHandler(err)
		return nil, err
	}
	model.AppMarkets.SetConn(market, conn)
	_ = subHandler(subscribes, subType)
	stopC := make(chan struct{}, 10)
	go chanHandler(market, stopC, errHandler, msgHandler)
	return stopC, err
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
		case conn := <-manager.Unregister:
			if _, ok := manager.Clients[conn]; ok {
				close(conn.ChanWrite)
				close(conn.ChanRead)
				delete(manager.Clients, conn)
				jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
				manager.Send(jsonMessage, conn)
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

func (c *WSClient) Read(msgHandler WSMsgHandler) {
	defer func() {
		c.Manager.Unregister <- c
		_ = c.Socket.Close()
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
			if msgHandler != nil {
				msgHandler(c, jsonMessage)
			}
		}
	}
}

func (c *WSClient) Write() {
	defer func() {
		c.Manager.Unregister <- c
		_ = c.Socket.Close()
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

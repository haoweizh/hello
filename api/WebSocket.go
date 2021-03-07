package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"hello/model"
	"hello/util"
	"net/http"
)

type MsgHandler func(message []byte)
type SubscribeHandler func(subscribes []interface{}, subType string) error
type ErrHandler func(err error)

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

type ClientManager struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
}

type Client struct {
	ID      string
	Socket  *websocket.Conn
	Channel chan []byte
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

var Manager = ClientManager{
	Register:   make(chan *Client),
	Unregister: make(chan *Client),
	Clients:    make(map[*Client]bool),
}

func (manager *ClientManager) Start() {
	for {
		select {
		case conn := <-manager.Register:
			manager.Clients[conn] = true
			jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
			manager.Send(jsonMessage, conn)
		case conn := <-manager.Unregister:
			if _, ok := manager.Clients[conn]; ok {
				close(conn.Channel)
				delete(manager.Clients, conn)
				jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
				manager.Send(jsonMessage, conn)
			}
		}
	}
}

func (manager *ClientManager) Send(message []byte, ignore *Client) {
	for conn := range manager.Clients {
		if conn != ignore {
			conn.Channel <- message
		}
	}
}

func (c *Client) Read(msgHandler MsgHandler) {
	defer func() {
		Manager.Unregister <- c
		_ = c.Socket.Close()
	}()
	for {
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			break
		}
		jsonMessage, _ := json.Marshal(&Message{Sender: c.ID, Content: string(message)})
		msgHandler(jsonMessage)
		//Manager.Broadcast <- jsonMessage
	}
}

func (c *Client) Write() {
	defer func() {
		Manager.Unregister <- c
		_ = c.Socket.Close()
	}()
	for {
		select {
		case message, ok := <-c.Channel:
			if !ok {
				return
			}
			_ = c.Socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}

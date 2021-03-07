package model

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/util"
)

type MsgHandler func(message []byte)
type SubscribeHandler func(subscribes []interface{}, subType string) error
type ErrHandler func(err error)

type WSManager struct {
	Clients    map[*WSClient]bool
	Broadcast  chan []byte
	Register   chan *WSClient
	Unregister chan *WSClient
}

type WSClient struct {
	ID      string
	Socket  *websocket.Conn
	Channel chan []byte
	Manager *WSManager
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
				close(conn.Channel)
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
			conn.Channel <- message
		}
	}
}

func (c *WSClient) Read(msgHandler MsgHandler) {
	defer func() {
		c.Manager.Unregister <- c
		_ = c.Socket.Close()
	}()
	for {
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			break
		}
		jsonMessage, _ := json.Marshal(&Message{Sender: c.ID, Content: string(message)})
		msgHandler(jsonMessage)
		util.Notice(`receive from ws ` + string(jsonMessage))
		//Manager.Broadcast <- jsonMessage
		c.Manager.Send(jsonMessage, nil)
	}
}

func (c *WSClient) Write() {
	defer func() {
		c.Manager.Unregister <- c
		_ = c.Socket.Close()
	}()
	for {
		select {
		case message, ok := <-c.Channel:
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

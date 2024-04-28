package model

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/util"
	"strings"
	"sync"
	"time"
)

type WSMsgHandler func(client *WSAgent, message []byte)

type WSManager struct {
	Connections *sync.Map // "market*symbol" - sync.Map[mailAddress] *WSAgent
	Register    chan *WSAgent
	Unregister  chan *WSAgent
}

type WSAgent struct {
	ID             string
	Socket         *websocket.Conn
	ChanRead       chan []byte
	Manager        *WSManager
	Pinged         bool
	SettingMonitor *SettingMonitor
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

func (manager *WSManager) Start() {
	for {
		select {
		case agent := <-manager.Register:
			monitorSetting := agent.SettingMonitor
			conns, ok := util.LoadSyncMap(manager.Connections, monitorSetting.Market, monitorSetting.Symbol)
			if !ok || conns == nil {
				conns = &sync.Map{}
				util.StoreSyncMap(manager.Connections, conns, monitorSetting.Market, monitorSetting.Symbol)
			}
			conns.(*sync.Map).Store(monitorSetting.MailAddress, agent)
			//jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
			//manager.Send(jsonMessage, agent)
			util.Info(fmt.Sprintf(`after registerd %s %s %s`,
				monitorSetting.Market, monitorSetting.Symbol, monitorSetting.MailAddress))
		case agent := <-manager.Unregister:
			monitorSetting := agent.SettingMonitor
			conns, ok := util.LoadSyncMap(manager.Connections, monitorSetting.Market, monitorSetting.Symbol)
			if !ok || conns == nil {
				continue
			}
			conns.(*sync.Map).Delete(monitorSetting.MailAddress)
			agent.Close()
			//jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
			//manager.Send(jsonMessage, agent)
			util.Info(fmt.Sprintf(`after unregister %s %s %s`,
				monitorSetting.Market, monitorSetting.Symbol, monitorSetting.MailAddress))
		}
	}
}

// Send TODO enhance aggregationCandle
func (manager *WSManager) Send(market, symbol string, aggregationCandle *AggregateCandle) {
	agent, ok := util.LoadSyncMap(manager.Connections, market, symbol)
	if !ok || agent == nil {
		return
	}
	agent.(*sync.Map).Range(func(address, value any) bool {
		if value == nil {
			return true
		}
		jsonBytes, err := json.Marshal(aggregationCandle)
		if err == nil {
			err = value.(*WSAgent).Socket.WriteMessage(websocket.TextMessage, jsonBytes)
			if err != nil {
				manager.Unregister <- value.(*WSAgent)
				util.Notice(fmt.Sprintf(`fail to send ws msg return, unregister %s`, err.Error()))
			}
		}
		return true
	})
}

func (agent *WSAgent) Close() {
	close(agent.ChanRead)
	_ = agent.Socket.Close()
}

func (agent *WSAgent) ReadServe(msgHandler WSMsgHandler) {
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
		case <-time.After(300 * time.Second):
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

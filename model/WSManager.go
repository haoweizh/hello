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
	WSAgents *sync.Map // mailAddress - *WSAgent
}

type WSAgent struct {
	ID       string
	Socket   *websocket.Conn
	ChanRead chan []byte
	Manager  *WSManager
	Pinged   bool
	Address  string                // mail address
	Data     map[string]*time.Time // key: market*symbol*Interval value:*time
}

type SettingMonitor struct {
	MailAddress     string `gorm:"index:address_market_symbol_interval,unique"`
	Market          string `gorm:"index:address_market_symbol_interval,unique"`
	Symbol          string `gorm:"index:address_market_symbol_interval,unique"`
	IntervalSeconds int    `gorm:"index:address_market_symbol_interval,unique"`
	WarnChange      float64
	WarnIncrease    float64
	WarnVolume      float64
	ID              uint `gorm:"primary_key"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

func (manager *WSManager) RemoveAgent(address string) {
	wsAgent, _ := manager.WSAgents.Load(address)
	if wsAgent != nil {
		manager.WSAgents.Delete(address)
		wsAgent.(*WSAgent).Close()
	}
	util.Notice(fmt.Sprintf(`remove agent %s`, address))
}

func (manager *WSManager) AddAgent(wsAgent *WSAgent) {
	if manager.WSAgents == nil {
		manager.WSAgents = &sync.Map{}
	}
	oldAgent, _ := manager.WSAgents.Load(wsAgent.Address)
	if oldAgent != nil {
		oldAgent.(*WSAgent).Close()
	}
	manager.WSAgents.Store(wsAgent.Address, wsAgent)
	//jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
	//manager.Send(jsonMessage, agent)
	util.Notice(fmt.Sprintf(`add agent %s`, wsAgent.Address))
}

func (agent *WSAgent) Close() {
	go func() {
		close(agent.ChanRead)
		_ = agent.Socket.Close()
	}()
}

func (agent *WSAgent) ReadServe(msgHandler WSMsgHandler) {
	defer func() {
		agent.Manager.RemoveAgent(agent.Address)
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
			if strings.Contains(string(jsonMessage), `ping`) || strings.Contains(string(jsonMessage), `hello`) {
				err := agent.Socket.WriteMessage(websocket.TextMessage, []byte(`pong`))
				if err != nil {
					agent.Manager.RemoveAgent(agent.Address)
					util.Notice(fmt.Sprintf(`fail to send ws msg return, unregister %s %s`, agent.Address, err.Error()))
				} else {
					agent.Pinged = true
				}
			}
			if msgHandler != nil {
				msgHandler(agent, jsonMessage)
			}
		case <-time.After(300 * time.Second):
			if agent.Pinged {
				agent.Pinged = false
			} else {
				util.Info(`time out without ping`)
				return
			}
		}
	}
}

func (agent *WSAgent) Update(aggregateCandle *AggregateCandle) {
	if agent.Data == nil {
		agent.Data = make(map[string]*time.Time)
	}
	needSend := false
	for key, t := range agent.Data {
		if t == nil || t.Add(time.Minute*5).Before(time.Now()) {
			delete(agent.Data, key)
			needSend = true
		}
	}
	key := fmt.Sprintf(`%s*%s*%d`, aggregateCandle.Market, aggregateCandle.Symbol, aggregateCandle.TimeInterval)
	if agent.Data[key] == nil {
		current := time.Now()
		agent.Data[key] = &current
		needSend = true
	}
	util.Notice(fmt.Sprintf(`send ws msg %s need %v %v`, aggregateCandle.GetKey(), needSend, agent.Data))
	if !needSend {
		return
	}
	jsonBytes, err := json.Marshal(agent.Data)
	err = agent.Socket.WriteMessage(websocket.TextMessage, jsonBytes)
	if err != nil {
		agent.Manager.RemoveAgent(agent.Address)
		util.Notice(fmt.Sprintf(`fail to send ws msg return, unregister %s %s`, agent.Address, err.Error()))
	}
}

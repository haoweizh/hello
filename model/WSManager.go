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
	WSAgents *sync.Map // mailAddress - sync[*WSAgent]bool
	Data     *sync.Map // mailAddress - sync[market*symbol*Interval]SettingMonitor
}

type WSAgent struct {
	ID       string
	Socket   *websocket.Conn
	ChanRead chan []byte
	Manager  *WSManager
	Pinged   bool
	Address  string // mail address
}

type SettingMonitor struct {
	MailAddress     string `gorm:"index:address_market_symbol_interval,unique"`
	Market          string `gorm:"index:address_market_symbol_interval,unique"`
	Symbol          string `gorm:"index:address_market_symbol_interval,unique"`
	IntervalSeconds int    `gorm:"index:address_market_symbol_interval,unique"`
	WarnChange      float64
	WarnIncrease    float64
	WarnVolume      float64
	ID              uint  `gorm:"primary_key"`
	WarnAt          int64 // warn at time in million seconds
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

var wsLock = sync.Mutex{}

func (manager *WSManager) Update(address string, aggregateCandle *AggregateCandle) {
	agents, _ := manager.WSAgents.Load(address)
	if agents == nil {
		return
	}
	if manager.Data == nil {
		manager.Data = &sync.Map{}
	}
	agents.(*sync.Map).Range(func(agent, value any) bool {
		data, _ := manager.Data.Load(address)
		if data == nil {
			data = &sync.Map{}
			manager.Data.Store(address, data)
		}
		data.(*sync.Map).Range(func(key, value any) bool {
			if value.(SettingMonitor).CreatedAt.Add(time.Hour * 24).Before(time.Now()) {
				data.(*sync.Map).Delete(key)
			}
			return true
		})
		key := fmt.Sprintf(`%s*%s*%d`, aggregateCandle.Market, aggregateCandle.Symbol, aggregateCandle.TimeInterval)
		data.(*sync.Map).Store(key, SettingMonitor{
			MailAddress:     address,
			Market:          aggregateCandle.Market,
			Symbol:          aggregateCandle.Symbol,
			IntervalSeconds: aggregateCandle.TimeInterval,
			CreatedAt:       time.Now(),
			WarnAt:          time.Now().UnixMilli()})
		msg := make(map[string]SettingMonitor)
		data.(*sync.Map).Range(func(key, value any) bool {
			msg[key.(string)] = value.(SettingMonitor)
			return true
		})
		util.Notice(fmt.Sprintf(`send ws msg %s need %v %v`,
			aggregateCandle.GetKey(), agent.(*WSAgent), msg))
		jsonBytes, err := json.Marshal(msg)
		wsLock.Lock()
		err = agent.(*WSAgent).Socket.WriteMessage(websocket.TextMessage, jsonBytes)
		if err != nil {
			//manager.RemoveAgent(address, agent.(*WSAgent))
			util.Notice(fmt.Sprintf(`fail to send ws update %s %v %s`, address, agent, err.Error()))
		}
		wsLock.Unlock()
		return true
	})
}

func (manager *WSManager) RemoveAgent(address string, wsAgent *WSAgent) {
	agents, _ := manager.WSAgents.Load(address)
	if agents != nil {
		wsAgent.Close()
		agents.(*sync.Map).Delete(wsAgent)
	}
	util.Notice(fmt.Sprintf(`remove agent %s %v`, address, wsAgent))
}

func (manager *WSManager) AddAgent(wsAgent *WSAgent) {
	if manager.WSAgents == nil {
		manager.WSAgents = &sync.Map{}
	}
	agents, _ := manager.WSAgents.Load(wsAgent.Address)
	if agents == nil {
		agents = &sync.Map{}
		manager.WSAgents.Store(wsAgent.Address, agents)
	}
	agents.(*sync.Map).Store(wsAgent, true)
	//jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
	//manager.Send(jsonMessage, agent)
	agents.(*sync.Map).Range(func(key, value any) bool {
		util.Notice(fmt.Sprintf(`got agent %s %v`, wsAgent.Address, key))
		return true
	})
}

func (agent *WSAgent) Close() {
	defer func() {
		if recover() != nil {
		}
	}()
	close(agent.ChanRead)
	_ = agent.Socket.Close()
}

func (agent *WSAgent) ReadServe(msgHandler WSMsgHandler) {
	defer func() {
		agent.Manager.RemoveAgent(agent.Address, agent)
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
					util.Notice(fmt.Sprintf(`fail to send ws msg return, unregister %s %s`, agent.Address, err.Error()))
					return
				} else {
					agent.Pinged = true
				}
			}
			if msgHandler != nil {
				msgHandler(agent, jsonMessage)
			}
		case <-time.After(20 * time.Second):
			if agent.Pinged {
				agent.Pinged = false
				err := agent.Socket.WriteMessage(websocket.TextMessage, []byte(`ping`))
				if err != nil {
					util.Notice(fmt.Sprintf(`timer trigger fail ping return %s %s`, agent.Address, err.Error()))
					return
				} else {
					agent.Pinged = true
				}
			} else {
				util.Notice(`time out without ping`)
				return
			}
		}
	}
}

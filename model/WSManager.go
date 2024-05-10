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
	Data     *sync.Map // mailAddress - sync[market*symbol*Interval]sync[*createdAt]SettingMonitor
}

type WSAgent struct {
	ID                  string
	Socket              *websocket.Conn
	ChanRead, ChanWrite chan []byte
	Manager             *WSManager
	Pinged              bool
	Address             string // mail address
}

type SettingMonitor struct {
	MailAddress     string `gorm:"index:address_market_symbol_interval,unique"`
	Market          string `gorm:"index:address_market_symbol_interval,unique"`
	Symbol          string `gorm:"index:address_market_symbol_interval,unique"`
	IntervalSeconds int    `gorm:"index:address_market_symbol_interval,unique"`
	WarnChange      float64
	WarnIncrease    float64
	WarnVolume      float64
	Volume24        float64
	WarnAt          int64 // warn at time in million-seconds
	ID              uint  `gorm:"primary_key"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

func (manager *WSManager) WrapSend(address string) {
	agents, _ := manager.WSAgents.Load(address)
	if agents == nil {
		return
	}
	data, _ := manager.Data.Load(address)
	if data == nil {
		return
	}
	msg := map[string]SettingMonitor{time.Now().String(): {
		Symbol:    "最后更新时间",
		CreatedAt: time.Time{},
	}}
	data.(*sync.Map).Range(func(key, value any) bool {
		if value == nil {
			return true
		}
		value.(*sync.Map).Range(func(createdAt, settingMonitor any) bool {
			if settingMonitor == nil {
				return true
			}
			if createdAt.(*time.Time).Add(time.Hour * 24).Before(time.Now()) {
				value.(*sync.Map).Delete(createdAt)
			} else {
				msg[fmt.Sprintf(`%v%v`, key, createdAt)] = settingMonitor.(SettingMonitor)
			}
			return true
		})
		return true
	})
	jsonBytes, err := json.Marshal(msg)
	if err == nil {
		agents.(*sync.Map).Range(func(agent, value any) bool {
			agent.(*WSAgent).ChanWrite <- jsonBytes
			return true
		})
	}
}

func (manager *WSManager) Update(address string, aggregateCandle *AggregateCandle) (duplicated bool) {
	if manager.Data == nil {
		manager.Data = &sync.Map{}
	}
	data, _ := manager.Data.Load(address)
	if data == nil {
		data = &sync.Map{}
		manager.Data.Store(address, data)
	}
	keyAggregate := fmt.Sprintf(`%s*%s*%d`, aggregateCandle.Market, aggregateCandle.Symbol, aggregateCandle.TimeInterval)
	value, _ := data.(*sync.Map).Load(keyAggregate)
	if value == nil {
		value = &sync.Map{}
		data.(*sync.Map).Store(keyAggregate, value)
	}
	value.(*sync.Map).Range(func(createdAt, settingMonitor any) bool {
		if settingMonitor == nil {
			return true
		}
		if createdAt.(*time.Time).Add(time.Minute * 5).After(time.Now()) {
			duplicated = true
			return false
		}
		return true
	})
	if !duplicated {
		temp := time.Now()
		value.(*sync.Map).Store(&temp, SettingMonitor{
			MailAddress:     address,
			Market:          aggregateCandle.Market,
			Symbol:          aggregateCandle.Symbol,
			IntervalSeconds: aggregateCandle.TimeInterval,
			CreatedAt:       time.Now(),
			WarnAt:          time.Now().UnixMilli()})
	}
	return duplicated
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
	close(agent.ChanWrite)
	_ = agent.Socket.Close()
}

func (agent *WSAgent) WriteServe() {
	for {
		select {
		case message, ok := <-agent.ChanWrite:
			if !ok {
				return
			}
			err := agent.Socket.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				//manager.RemoveAgent(address, agent.(*WSAgent))
				util.Notice(fmt.Sprintf(`fail to send ws update %v %s`, agent, err.Error()))
			}
		}
	}
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
				agent.Manager.WrapSend(agent.Address)
				agent.Pinged = true
			}
			if msgHandler != nil {
				msgHandler(agent, jsonMessage)
			}
		case <-time.After(20 * time.Second):
			if agent.Pinged {
				agent.Pinged = false
				agent.ChanWrite <- []byte(`ping`)
			} else {
				util.Notice(`time out without ping`)
				return
			}
		}
	}
}

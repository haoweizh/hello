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
	WSAgents *sync.Map // sync.Map[market]*sync.Map[symbol]*sync.Map[mailAddress] *WSAgent
}

type WSAgent struct {
	ID              string
	Socket          *websocket.Conn
	ChanRead        chan []byte
	Manager         *WSManager
	Pinged          bool
	SettingMonitor  *SettingMonitor
	AggregateCandle *AggregateCandle
}

type SettingMonitor struct {
	MailAddress     string `gorm:"index:address_market_symbol,unique"`
	Market          string `gorm:"index:address_market_symbol,unique"`
	Symbol          string `gorm:"index:address_market_symbol,unique"`
	IntervalSeconds int
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

func (manager *WSManager) RemoveAgent(market, symbol, address string) {
	mapMarket, _ := manager.WSAgents.Load(market)
	if mapMarket == nil {
		return
	}
	mapSymbol, _ := mapMarket.(*sync.Map).Load(symbol)
	if mapSymbol == nil {
		return
	}
	mapAddress, _ := mapSymbol.(*sync.Map).Load(address)
	if mapAddress != nil {
		agent, _ := mapAddress.(*sync.Map).Load(address)
		mapAddress.(*sync.Map).Delete(address)
		if agent != nil {
			agent.(*WSAgent).Close()
		}
	}
}

func (manager *WSManager) AddAgent(wsAgent *WSAgent) {
	monitorSetting := wsAgent.SettingMonitor
	if manager.WSAgents == nil {
		manager.WSAgents = &sync.Map{}
	}
	mapMarket, _ := manager.WSAgents.Load(monitorSetting.Market)
	if mapMarket == nil {
		mapMarket = &sync.Map{}
		manager.WSAgents.Store(monitorSetting.Market, mapMarket)
	}
	mapSymbol, _ := mapMarket.(*sync.Map).Load(monitorSetting.Symbol)
	if mapSymbol == nil {
		mapSymbol = &sync.Map{}
		mapMarket.(*sync.Map).Store(monitorSetting.Symbol, mapSymbol)
	}
	mapAddress, _ := mapSymbol.(*sync.Map).Load(monitorSetting.MailAddress)
	if mapAddress == nil {
		mapAddress = &sync.Map{}
		mapSymbol.(*sync.Map).Store(monitorSetting.MailAddress, wsAgent)
	}
	oldAgent, _ := mapAddress.(*sync.Map).Load(monitorSetting.MailAddress)
	if oldAgent != nil {
		oldAgent.(*WSAgent).Close()
	}
	mapAddress.(*sync.Map).Store(monitorSetting.MailAddress, wsAgent)
	//jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
	//manager.Send(jsonMessage, agent)
	util.Info(fmt.Sprintf(`after register %s %s %s`,
		monitorSetting.Market, monitorSetting.Symbol, monitorSetting.MailAddress))
}

func (agent *WSAgent) Close() {
	close(agent.ChanRead)
	_ = agent.Socket.Close()
}

func (agent *WSAgent) ReadServe(msgHandler WSMsgHandler) {
	defer func() {
		if agent.SettingMonitor != nil {
			agent.Manager.RemoveAgent(agent.SettingMonitor.Market, agent.SettingMonitor.Symbol, agent.SettingMonitor.MailAddress)
		}
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
				util.Info(`time out without ping`)
				return
			}
		}
	}
}

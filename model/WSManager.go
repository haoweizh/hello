package model

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/util"
	"strconv"
	"strings"
	"sync"
	"time"
)

type WSMsgHandler func(client *WSAgent, message []byte)

type WSManager struct {
	WSAgents *sync.Map // market*symbol*interval - *sync.Map[mailAddress] *WSAgent
}

type WSAgent struct {
	ID             string
	Socket         *websocket.Conn
	ChanRead       chan []byte
	Manager        *WSManager
	Pinged         bool
	SettingMonitor *SettingMonitor
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

func (manager *WSManager) RemoveAgent(market, symbol string, interval int, address string) {
	addressAgents, _ := util.LoadSyncMap(manager.WSAgents, market, symbol, strconv.Itoa(interval))
	wsAgent, _ := addressAgents.(*sync.Map).Load(address)
	if wsAgent != nil {
		addressAgents.(*sync.Map).Delete(address)
		wsAgent.(*WSAgent).Close()
	}
	util.Notice(fmt.Sprintf(`remove agent %s %s %d %s`, market, symbol, interval, address))
}

func (manager *WSManager) AddAgent(wsAgent *WSAgent) {
	monitorSetting := wsAgent.SettingMonitor
	if manager.WSAgents == nil {
		manager.WSAgents = &sync.Map{}
	}
	addressAgents, _ := util.LoadSyncMap(manager.WSAgents, monitorSetting.Market, monitorSetting.Symbol,
		strconv.Itoa(monitorSetting.IntervalSeconds))
	if addressAgents == nil {
		addressAgents = &sync.Map{}
		util.StoreSyncMap(manager.WSAgents, monitorSetting.Market, monitorSetting.Symbol, strconv.Itoa(monitorSetting.IntervalSeconds))
	}
	oldAgent, _ := addressAgents.(*sync.Map).Load(monitorSetting.MailAddress)
	if oldAgent != nil {
		oldAgent.(*WSAgent).Close()
	}
	addressAgents.(*sync.Map).Store(monitorSetting.MailAddress, wsAgent)
	//jsonMessage, _ := json.Marshal(&Message{Content: "/A new socket has connected."})
	//manager.Send(jsonMessage, agent)
	util.Notice(fmt.Sprintf(`add agent %s %s %d %s`,
		monitorSetting.Market, monitorSetting.Symbol, monitorSetting.IntervalSeconds, monitorSetting.MailAddress))
}

func (agent *WSAgent) Close() {
	go func() {
		close(agent.ChanRead)
		_ = agent.Socket.Close()
	}()
}

func (agent *WSAgent) ReadServe(msgHandler WSMsgHandler) {
	defer func() {
		if agent.SettingMonitor != nil {
			agent.Manager.RemoveAgent(agent.SettingMonitor.Market, agent.SettingMonitor.Symbol,
				agent.SettingMonitor.IntervalSeconds, agent.SettingMonitor.MailAddress)
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

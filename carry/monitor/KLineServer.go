package monitor

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"sync"
	"time"
)

func RefreshSettingMonitors(environment *model.Environment, settingMonitors []*model.SettingMonitor) {
	if environment.AggregateCandles == nil {
		environment.AggregateCandles = &sync.Map{}
	}
	for _, monitor := range settingMonitors {
		mapSymbol, _ := environment.AggregateCandles.Load(monitor.Market)
		if mapSymbol == nil {
			mapSymbol = &sync.Map{}
			environment.AggregateCandles.Store(monitor.Market, mapSymbol)
		}
		mapAddress, _ := mapSymbol.(*sync.Map).Load(monitor.Symbol)
		if mapAddress == nil {
			mapAddress = &sync.Map{}
			mapSymbol.(*sync.Map).Store(monitor.Symbol, mapAddress)
		}
		aggregateCandle, _ := mapAddress.(*sync.Map).Load(monitor.MailAddress)
		if aggregateCandle == nil {
			util.Notice(fmt.Sprintf(`create new aggregate for %s %s %s`, monitor.Market, monitor.Symbol, monitor.MailAddress))
			mapAddress.(*sync.Map).Store(monitor.MailAddress, &model.AggregateCandle{
				TimeInterval: time.Duration(monitor.IntervalSeconds) * time.Second, LinkList: &util.LinkList{}})
		}
	}
}

var ProcessMonitor = func(environment *model.Environment, candle *model.Candle) {
	symbolCandles, _ := environment.AggregateCandles.Load(candle.Market)
	if symbolCandles == nil {
		return
	}
	addressCandle, _ := symbolCandles.(*sync.Map).Load(candle.Symbol)
	if addressCandle == nil {
		return
	}
	var addressAgents interface{}
	symbolAgents, _ := environment.WsManager.WSAgents.Load(candle.Market)
	if symbolAgents != nil {
		addressAgents, _ = symbolAgents.(*sync.Map).Load(candle.Symbol)
	}
	addressCandle.(*sync.Map).Range(func(address, value any) bool {
		//aggregateCandle := value.(*model.AggregateCandle)
		//aggregateCandle.Handle(candle)
		//formatedData := map[string]interface{}{
		//	`开始`: fmt.Sprintf(`%s %.4e`, aggregateCandle.Start.Format(`2006-01-02 15:04:05`), aggregateCandle.PriceStart),
		//	`结束`: fmt.Sprintf(`%s %.4e`, aggregateCandle.End.Format(`2006-01-02 15:04:05`), aggregateCandle.PriceCurrent),
		//	`成交`: fmt.Sprintf(`%d秒 %.0e`, aggregateCandle.TimeInterval/time.Second, aggregateCandle.VolumeQuote),
		//	`价格`: fmt.Sprintf(`%.4e-%.4e 变化%.2f 涨幅%.2f`,
		//		aggregateCandle.PriceLow, aggregateCandle.PriceHigh, aggregateCandle.PriceChange, aggregateCandle.PriceIncrease),
		//	`PriceChange`: aggregateCandle.PriceChange, `PriceIncrease`: aggregateCandle.PriceIncrease, `Volume`: aggregateCandle.VolumeQuote}
		//jsonBytes, err := json.Marshal(formatedData)
		jsonBytes, err := json.Marshal(candle)
		if err == nil && addressAgents != nil {
			addressAgents.(*sync.Map).Range(func(address, agent interface{}) bool {
				err = agent.(*model.WSAgent).Socket.WriteMessage(websocket.TextMessage, jsonBytes)
				if err != nil {
					environment.WsManager.RemoveAgent(candle.Market, candle.Symbol, address.(string))
					util.Notice(fmt.Sprintf(`fail to send ws msg return, unregister %s`, err.Error()))
				}
				return true
			})
		}
		return true
	})
}

package monitor

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"sync"
)

func KLineServer(environment *model.Environment) {
	for {
		select {
		case candle := <-model.KLineChan:
			go func() {
				environment.WsManager.WSAgents.Range(func(market, marketMap interface{}) bool {
					if marketMap == nil || candle.Market != market {
						return true
					}
					marketMap.(*sync.Map).Range(func(symbol, symbolMap interface{}) bool {
						if symbolMap == nil || candle.Symbol != symbol {
							return true
						}
						symbolMap.(*sync.Map).Range(func(mailAddress, value interface{}) bool {
							if value == nil {
								return true
							}
							aggregateCandle := value.(*model.WSAgent).AggregateCandle
							if aggregateCandle != nil {
								aggregateCandle.Handle(candle)
								jsonBytes, err := json.Marshal(aggregateCandle)
								if err == nil {
									err = value.(*model.WSAgent).Socket.WriteMessage(websocket.TextMessage, jsonBytes)
									if err != nil {
										environment.WsManager.RemoveAgent(market.(string), symbol.(string), mailAddress.(string))
										util.Notice(fmt.Sprintf(`fail to send ws msg return, unregister %s`, err.Error()))
									}
								}
							}
							return true
						})
						return true
					})
					return true
				})
			}()
		}
	}
}

package monitor

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"sync"
	"time"
)

var aggregatePool = &sync.Map{} // market*symbol*interval*HH:MM - *AggregateCandle

func RefreshSettingMonitors(environment *model.Environment, settingMonitors []*model.SettingMonitor) {
	environment.MonitorSettings = &sync.Map{}
	for _, monitor := range settingMonitors {
		mapSymbol, _ := environment.MonitorSettings.Load(monitor.Market)
		if mapSymbol == nil {
			mapSymbol = &sync.Map{}
			environment.MonitorSettings.Store(monitor.Market, mapSymbol)
		}
		mapInterval, _ := mapSymbol.(*sync.Map).Load(monitor.Symbol)
		if mapInterval == nil {
			mapInterval = &sync.Map{}
			mapSymbol.(*sync.Map).Store(monitor.Symbol, mapInterval)
		}
		intervalMonitors, _ := mapInterval.(*sync.Map).Load(monitor.IntervalSeconds)
		if intervalMonitors == nil {
			util.Notice(fmt.Sprintf(`create new aggregate for %s %s %s`, monitor.Market, monitor.Symbol, monitor.MailAddress))
			mapInterval.(*sync.Map).Store(monitor.MailAddress, monitor)
		}
	}
}

func GetPooledAggregate(candle *model.Candle, interval int) (pooledAggregate *model.AggregateCandle) {
	historyTime := candle.Begin.Add(-1 * time.Duration(interval) * time.Second)
	pooledAggregate = &model.AggregateCandle{
		Market:       candle.Market,
		Symbol:       candle.Symbol,
		Start:        &historyTime,
		TimeInterval: interval,
	}
	value, _ := aggregatePool.Load(pooledAggregate.GetKey())
	if value != nil {
		return value.(*model.AggregateCandle)
	}
	util.Notice(fmt.Sprintf(`create pooled candle %s %s %d:%d %d`,
		candle.Market, candle.Symbol, historyTime.Hour(), historyTime.Minute(), interval))
	for begin := historyTime; begin.Before(candle.Begin); begin.Add(time.Minute) {
		key := fmt.Sprintf(`%s*%s*%d*%d:%d`,
			candle.Market, candle.Symbol, 60, begin.Hour(), begin.Minute())
		temp, _ := aggregatePool.Load(key)
		if temp != nil {
			if pooledAggregate.PriceStart == 0 {
				pooledAggregate.PriceStart = temp.(*model.AggregateCandle).PriceStart
			}
			if pooledAggregate.PriceLow == 0 {
				pooledAggregate.PriceLow = temp.(*model.AggregateCandle).PriceLow
			}
			pooledAggregate.PriceHigh = math.Max(pooledAggregate.PriceHigh, temp.(*model.AggregateCandle).PriceHigh)
			pooledAggregate.PriceLow = math.Min(pooledAggregate.PriceLow, temp.(*model.AggregateCandle).PriceLow)
			pooledAggregate.VolumeQuote += temp.(*model.AggregateCandle).VolumeQuote
			pooledAggregate.PriceCurrent = temp.(*model.AggregateCandle).PriceCurrent
		}
	}
	aggregatePool.Store(pooledAggregate.GetKey(), pooledAggregate)
	return pooledAggregate
}

var ProcessMonitor = func(environment *model.Environment, candle *model.Candle) {
	symbolMonitors, _ := environment.MonitorSettings.Load(candle.Market)
	if symbolMonitors == nil {
		return
	}
	intervalMonitors, _ := symbolMonitors.(*sync.Map).Load(candle.Symbol)
	if intervalMonitors == nil {
		return
	}
	minuteAggregate := &model.AggregateCandle{
		Market:       candle.Market,
		Symbol:       candle.Symbol,
		Start:        &candle.Begin,
		PriceHigh:    candle.PriceHigh,
		PriceLow:     candle.PriceLow,
		VolumeQuote:  candle.VolumeQuote,
		PriceStart:   candle.PriceOpen,
		PriceCurrent: candle.PriceClose}
	aggregatePool.Store(minuteAggregate.GetKey(), minuteAggregate)
	intervalMonitors.(*sync.Map).Range(func(interval, monitor any) bool {
		pooledAggregate := GetPooledAggregate(candle, interval.(int))
		pooledAggregate.PriceCurrent = candle.PriceClose
		pooledAggregate.End = &candle.CreatedAt
		pooledAggregate.VolumeQuote += candle.VolumeQuote
		pooledAggregate.PriceHigh = math.Max(pooledAggregate.PriceHigh, candle.PriceHigh)
		if pooledAggregate.PriceLow == 0 {
			pooledAggregate.PriceLow = candle.PriceLow
		} else {
			pooledAggregate.PriceLow = math.Min(pooledAggregate.PriceLow, candle.PriceLow)
		}
		pooledAggregate.PriceIncrease = (pooledAggregate.PriceCurrent - pooledAggregate.PriceStart) / pooledAggregate.PriceCurrent
		pooledAggregate.PriceChange = (pooledAggregate.PriceHigh - pooledAggregate.PriceLow) / pooledAggregate.PriceCurrent
		addressAgents, _ := util.LoadSyncMap(environment.WsManager.WSAgents, candle.Market, candle.Symbol,
			strconv.Itoa(interval.(int)))
		if addressAgents == nil {
			return true
		}
		addressAgents.(*sync.Map).Range(func(address, agent any) bool {
			formatedData := map[string]interface{}{
				`开始`: fmt.Sprintf(`%s %.4e`, pooledAggregate.Start.Format(`2006-01-02 15:04:05`), pooledAggregate.PriceStart),
				`结束`: fmt.Sprintf(`%s %.4e`, pooledAggregate.End.Format(`2006-01-02 15:04:05`), pooledAggregate.PriceCurrent),
				`成交`: fmt.Sprintf(`%d秒 %.0e`, pooledAggregate.TimeInterval, pooledAggregate.VolumeQuote),
				`价格`: fmt.Sprintf(`%.4e-%.4e 变化%.2f 涨幅%.2f`, pooledAggregate.PriceLow,
					pooledAggregate.PriceHigh, pooledAggregate.PriceChange, pooledAggregate.PriceIncrease),
				`PriceChange`: pooledAggregate.PriceChange, `PriceIncrease`: pooledAggregate.PriceIncrease,
				`Volume`: pooledAggregate.VolumeQuote}
			jsonBytes, err := json.Marshal(formatedData)
			err = agent.(*model.WSAgent).Socket.WriteMessage(websocket.TextMessage, jsonBytes)
			if err != nil {
				environment.WsManager.RemoveAgent(candle.Market, candle.Symbol, interval.(int), address.(string))
				util.Notice(fmt.Sprintf(`fail to send ws msg return, unregister %s %s %d %s %s`,
					candle.Market, candle.Symbol, interval, address.(string), err.Error()))
			}
			return true
		})
		return true
	})
}

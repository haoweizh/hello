package monitor

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"sync"
	"time"
)

var aggregatePool = &sync.Map{} // market*symbol*interval*HH:MM - *AggregateCandle
var lastRefreshTime = sync.Map{}

func RefreshSettingMonitors(environment *model.Environment, settingMonitors []*model.SettingMonitor) {
	environment.MonitorSettings = &sync.Map{}
	for _, monitor := range settingMonitors {
		refreshTime, _ := lastRefreshTime.Load(monitor.Market)
		if refreshTime == nil || refreshTime.(*time.Time).Add(time.Hour).Before(time.Now()) {
			api.InitMarketInfos(monitor.Market)
			temp := time.Now()
			lastRefreshTime.Store(monitor.Market, &temp)
			continue
		}
		symbolMonitor, _ := environment.MonitorSettings.Load(monitor.Market)
		if symbolMonitor == nil {
			symbolMonitor = &sync.Map{}
			environment.MonitorSettings.Store(monitor.Market, symbolMonitor)
		}
		intervalMonitor, _ := symbolMonitor.(*sync.Map).Load(monitor.Symbol)
		if intervalMonitor == nil {
			intervalMonitor = &sync.Map{}
			symbolMonitor.(*sync.Map).Store(monitor.Symbol, intervalMonitor)
		}
		addressMonitors, _ := intervalMonitor.(*sync.Map).Load(monitor.IntervalSeconds)
		if addressMonitors == nil {
			addressMonitors = &sync.Map{}
			intervalMonitor.(*sync.Map).Store(monitor.IntervalSeconds, addressMonitors)
		}
		addressMonitors.(*sync.Map).Store(monitor.MailAddress, monitor)
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
		pooledAggregate.PriceLow = value.(*model.AggregateCandle).PriceLow
		pooledAggregate.PriceHigh = value.(*model.AggregateCandle).PriceHigh
		pooledAggregate.PriceStart = value.(*model.AggregateCandle).PriceStart
		pooledAggregate.PriceCurrent = value.(*model.AggregateCandle).PriceCurrent
		pooledAggregate.VolumeQuote = value.(*model.AggregateCandle).VolumeQuote
		return pooledAggregate
	}
	for begin := historyTime; begin.Before(candle.Begin); {
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
		} else {
			util.Notice(fmt.Sprintf(` fail to get minute candle %s`, key))
		}
		begin = begin.Add(time.Minute)
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
		TimeInterval: 60,
		Start:        &candle.Begin,
		PriceHigh:    candle.PriceHigh,
		PriceLow:     candle.PriceLow,
		VolumeQuote:  candle.VolumeQuote,
		PriceStart:   candle.PriceOpen,
		PriceCurrent: candle.PriceClose}
	aggregatePool.Store(minuteAggregate.GetKey(), minuteAggregate)
	intervalMonitors.(*sync.Map).Range(func(interval, value any) bool {
		if value == nil {
			return true
		}
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
		value.(*sync.Map).Range(func(address, monitor any) bool {
			marketInfo, _ := util.LoadSyncMap(model.MarketInfos, monitor.(*model.SettingMonitor).Market, monitor.(*model.SettingMonitor).Symbol)
			if address == nil || monitor == nil || pooledAggregate.VolumeQuote < monitor.(*model.SettingMonitor).WarnVolume ||
				pooledAggregate.PriceIncrease < monitor.(*model.SettingMonitor).WarnIncrease ||
				pooledAggregate.PriceChange < monitor.(*model.SettingMonitor).WarnChange || marketInfo == nil ||
				marketInfo.(*model.MarketInfo).TradeAmount < monitor.(*model.SettingMonitor).Volume24 {
				return true
			}
			environment.WsManager.Update(address.(string), pooledAggregate)
			return true
		})
		return true
	})
}

package monitor

import (
	"hello/api"
	"hello/model"
	"hello/util"
	"time"
)

func KLineServer() {
	markets := api.GetMarkets()
	for _, market := range markets {
		settings := api.GetSettings(model.FunctionKLine, market)
		if settings == nil {
			continue
		}
		settings.Range(func(symbol, setting any) bool {
			util.StoreSyncMap(model.DataMonitor, &model.AggregateCandle{
				TimeInterval: time.Duration(setting.(*model.Setting).Seconds) * time.Second, SlideRing: &model.SlideRing{}},
				market, symbol.(string))
			return true
		})
	}
	for {
		select {
		case candle := <-model.KLineChan:
			go func() {
				aggregationCandle, success := util.LoadSyncMap(model.DataMonitor, candle.Market, candle.Symbol)
				if success && aggregationCandle != nil {
					aggregationCandle.(*model.AggregateCandle).Handle(candle)
					model.AppWSManager.Send(candle.Market, candle.Symbol, aggregationCandle.(*model.AggregateCandle))
				}
			}()
		}
	}
}

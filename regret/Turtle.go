package regret

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"sync"
	"time"
)

type TurtleData struct {
	high10, low10, high20, low20, high3, low3, n float64
	orderLong, orderShort                        *model.Order
}

var turtleDataMap sync.Map // market_symbol_slotSeconds_2019-12-06THH:MM:SS  *turtleData

func GetTurtleData(key, secret, market, symbol string, turtleTime time.Time, slotSeconds int, setting *model.Setting) (
	turtleData *TurtleData) {
	turtleKey := fmt.Sprintf(`%s_%s_%d_%s`, market, symbol, slotSeconds, turtleTime.Format(time.RFC3339))
	value, _ := turtleDataMap.Load(turtleKey)
	if value != nil {
		return value.(*TurtleData)
	}
	turtleData = &TurtleData{}
	for i := 1; i < 21; i++ {
		duration, _ := time.ParseDuration(fmt.Sprintf(`%ds`, -i*slotSeconds))
		candleTime := turtleTime.Add(duration)
		candle := api.GetTurtleCandle(key, secret, setting.Market, setting.Symbol, slotSeconds, candleTime)
		if candle == nil {
			util.Notice(`can not calc turtleDate as nil candle %s %s %s`,
				setting.Market, setting.Symbol, candleTime.String())
			return nil
		}
		if candle.PriceHigh > turtleData.high20 && i <= 20 {
			turtleData.high20 = candle.PriceHigh
		}
		if (turtleData.low20 == 0 || turtleData.low20 > candle.PriceLow) && i <= 20 {
			turtleData.low20 = candle.PriceLow
		}
		if candle.PriceHigh > turtleData.high10 && i <= 10 {
			turtleData.high10 = candle.PriceHigh
		}
		if (turtleData.low10 == 0 || turtleData.low10 > candle.PriceLow) && i <= 10 {
			turtleData.low10 = candle.PriceLow
		}
		if candle.PriceHigh > turtleData.high3 && i <= 3 {
			turtleData.high3 = candle.PriceHigh
		}
		if (turtleData.low3 == 0 || turtleData.low3 > candle.PriceLow) && i <= 3 {
			turtleData.low3 = candle.PriceLow
		}
		if i == 1 {
			turtleData.n = candle.N
		}
	}
	turtleDataMap.Store(turtleKey, turtleData)
	util.Notice(fmt.Sprintf(`%s %s set turtle data: amount:%f n:%f 20:%f %f 10:%f %f`,
		setting.Market, setting.Symbol, setting.AmountLimit, turtleData.n, turtleData.low20,
		turtleData.high20, turtleData.low10, turtleData.high10))
	time.Sleep(time.Millisecond * 100)
	return
}

func handlePrice(turtleData *TurtleData, candle *model.Candle) {
	if turtleData.orderLong != nil && candle.PriceHigh >= turtleData.orderLong.Price {
		turtleData.orderLong.Status = model.CarryStatusSuccess
		model.AppDB.Save(turtleData.orderLong)
		turtleData.orderLong = nil
		turtleData.orderShort = nil
	}
	if turtleData.orderShort != nil && candle.PriceLow <= turtleData.orderShort.Price {
		turtleData.orderShort.Status = model.CarryStatusSuccess
		model.AppDB.Save(turtleData.orderShort)
		turtleData.orderLong = nil
		turtleData.orderShort = nil
	}
}

func ProcessCandles(market, symbol string, start, end time.Time, setting *model.Setting) {
	key := model.AppConfig.GetAccounts(market)[0].Key
	secret := model.AppConfig.GetAccounts(market)[0].Secret
	candles := api.GetCandle(key, secret, market, symbol, 15, start, end)
	for _, candle := range candles {
		turtleTime := time.Date(candle.Begin.Year(), candle.Begin.Month(), candle.Begin.Day(), candle.Begin.Hour(),
			0, 0, 0, candle.Begin.Location())
		turtleData := GetTurtleData(key, secret, market, symbol, turtleTime, 3600, setting)
		handlePrice(turtleData, candle)
	}
}

package api

import (
	"hello/model"
	"time"
)

// 1m 3m 5m 15m 30m 1h 2h 4h 6h 8h 12h 1d 3d 1w 1M
func getCandlesGXZQDB(symbol string, begin, end time.Time, limit, slotSeconds int) (
	candles []*model.Candle, isCache bool) {
	candles = []*model.Candle{}
	model.AppDB.Model(model.Candle{}).Where(`market=? and symbol=? and seconds=? and begin=？and end=?`,
		model.GXZQ, symbol, slotSeconds, begin, end).Find(candles)
	return
}

package model

import (
	"time"
)

type Candle struct {
	Market                                     string    `gorm:"index:market_symbol_begin_seconds,unique"`
	Symbol                                     string    `gorm:"index:market_symbol_begin_seconds,unique"`
	Begin                                      time.Time `gorm:"index:market_symbol_begin_seconds,unique"`
	Seconds                                    int       `gorm:"index:market_symbol_begin_seconds,unique"` // period of seconds
	PriceOpen, PriceClose, PriceHigh, PriceLow float64
	Volume, VolumeQuote, N, M, NVolume         float64 // 用n值的平滑计算方法计算的交易量
	ID                                         uint    `gorm:"primary_key"`
	CreatedAt                                  time.Time
	UpdatedAt                                  time.Time
}

type SortedCandle struct {
	Value []*Candle
}

func (sortedCandle SortedCandle) Len() int {
	return len(sortedCandle.Value)
}

func (sortedCandle SortedCandle) Swap(i, j int) {
	sortedCandle.Value[i], sortedCandle.Value[j] = sortedCandle.Value[j], sortedCandle.Value[i]
}

func (sortedCandle SortedCandle) Less(i, j int) bool {
	return sortedCandle.Value[i].Begin.Before(sortedCandle.Value[j].Begin)
}

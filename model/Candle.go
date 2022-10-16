package model

import (
	"time"
)

type Candle struct {
	Market     string
	Symbol     string
	Begin      time.Time
	Seconds    int // period of seconds
	PriceOpen  float64
	PriceClose float64
	PriceHigh  float64
	PriceLow   float64
	N          float64 // n value for turtle
	ID         uint    `gorm:"primary_key"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type SortedCandle struct {
	Value []*Candle
}

func (sortedCandle *SortedCandle) Len() int {
	return len(sortedCandle.Value)
}

func (sortedCandle *SortedCandle) Swap(i, j int) {
	sortedCandle.Value[i], sortedCandle.Value[j] = sortedCandle.Value[j], sortedCandle.Value[i]
}

func (sortedCandle *SortedCandle) Less(i, j int) bool {
	return sortedCandle.Value[i].Begin.Before(sortedCandle.Value[j].Begin)
}

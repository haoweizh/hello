package model

import "time"

type Candle struct {
	Market           string
	SymbolInstrument string
	Ts               int64
	PriceFmex        float64
	PriceBitmex      float64
	UTCDate          string
	Period           string //[1m,5m,1h,1d]
	PriceOpen        float64
	PriceClose       float64
	PriceHigh        float64
	PriceLow         float64
	N                float64 // n value for turtle
	ID               uint    `gorm:"primary_key"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

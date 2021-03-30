package model

import "time"

type Candle struct {
	Market      string
	Symbol      string
	PriceBitmex float64
	UTCDate     string // UTC time in RFC3339[0:10]
	Period      string //[1m,5m,1h,1d]
	PriceOpen   float64
	PriceClose  float64
	PriceHigh   float64
	PriceLow    float64
	N           float64 // n value for turtle
	ID          uint    `gorm:"primary_key"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

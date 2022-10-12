package model

import "time"

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

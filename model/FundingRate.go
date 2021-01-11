package model

import "time"

type FundingRate struct {
	FundingTime time.Time // confirm time if transaction
	Rate        float64   // price in usdt
	Symbol      string
	ID          string `gorm:"primary_key"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

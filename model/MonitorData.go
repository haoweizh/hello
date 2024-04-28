package model

import (
	"time"
)

type SettingMonitor struct {
	MailAddress     string `gorm:"index:address_market_symbol,unique"`
	Market          string `gorm:"index:address_market_symbol,unique"`
	Symbol          string `gorm:"index:address_market_symbol,unique"`
	IntervalSeconds int
	WarnChange      float64
	WarnIncrease    float64
	WarnVolume      float64
	ID              uint `gorm:"primary_key"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

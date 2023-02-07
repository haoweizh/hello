package model

import (
	"time"
)

const SettingTurtleRemoved = `SettingTurtleRemoved`

type CarryHandler func(setting *Setting, bidAsk *BidAsk)

type Setting struct {
	Valid            bool
	Function         string `gorm:"index:function_market_symbol,unique"`
	Market           string `gorm:"index:function_market_symbol,unique"`
	Symbol           string `gorm:"index:function_market_symbol,unique"`
	Coin             string
	SymbolRelated    string // 在turtle算法中判断是否还被加入动态海龟
	PriceX           float64
	OpenShortMargin  float64 // arbitrary future use
	CloseShortMargin float64 // arbitrary future use
	Chance           int64   // arbitrary future use
	GridAmount       float64
	AmountLimit      float64
	ID               uint `gorm:"primary_key"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

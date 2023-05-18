package model

import (
	"fmt"
	"time"
)

const SettingTurtleRemoved = `SettingTurtleRemoved`

type CarryHandler func(setting *Setting, bidAsk *BidAsk)

type Setting struct {
	Valid              bool
	Function           string `gorm:"index:function_market_symbol,unique"`
	Market             string `gorm:"index:function_market_symbol,unique"`
	Symbol             string `gorm:"index:function_market_symbol,unique"`
	Coin               string
	SymbolRelated      string // 在turtle算法中判断是否还被加入动态海龟
	Chance             int64
	Far, Near, Seconds int64
	PriceX             float64
	OpenShortMargin    float64
	CloseShortMargin   float64
	GridAmount         float64
	AmountLimit        float64
	ID                 uint `gorm:"primary_key"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func GetMsgKey(function, market, symbol string) (msgKey string) {
	msgKey = fmt.Sprintf("%s_%s_%s", function, market, symbol)
	return msgKey
}

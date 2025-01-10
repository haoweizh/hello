package model

import (
	"fmt"
	"time"
)

const SettingTurtleRemoved = `SettingTurtleRemoved`

type CarryHandler func(setting *Setting, bidAsk *BidAsk)

type WsOrderHandler func(order *Order)
type WsCollateralHandler func(accountKey string, reduceOnly bool, collateral *Collateral)

type Setting struct {
	Valid, Liquidated                          bool
	Function                                   string `gorm:"index:function_market_symbol,unique"`
	Market                                     string `gorm:"index:function_market_symbol,unique"`
	Symbol                                     string `gorm:"index:function_market_symbol,unique"`
	Coin                                       string
	SymbolRelated, MarketRelated, WSType       string // 在turtle算法中判断是否还被加入动态海龟
	Chance, ChanceLimit, ChanceLimitCombine    int64
	Far, Near, Seconds                         int64
	FarCombine, NearCombine, SecondsCombine    int64
	PriceX, GridAmount                         float64
	OpenShortMargin, CloseShortMargin          float64
	AmountLimit, AmountRate, AmountRateCombine float64
	ID                                         uint `gorm:"primary_key"`
	CreatedAt                                  time.Time
	UpdatedAt                                  time.Time
}

func GetMsgKey(function, market, symbol string) (msgKey string) {
	msgKey = fmt.Sprintf("%s_%s_%s", function, market, symbol)
	return msgKey
}

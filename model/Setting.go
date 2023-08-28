package model

import (
	"fmt"
	"time"
)

const SettingTurtleRemoved = `SettingTurtleRemoved`

type CarryHandler func(setting *Setting, bidAsk *BidAsk)

type Setting struct {
	Valid                                      bool
	Function                                   string `gorm:"index:function_market_symbol_way,unique"`
	Market                                     string `gorm:"index:function_market_symbol_way,unique"`
	Symbol                                     string `gorm:"index:function_market_symbol_way,unique"`
	Way                                        string `gorm:"index:function_market_symbol_way,unique"`
	Coin                                       string
	SymbolRelated                              string // 在turtle算法中判断是否还被加入动态海龟
	Chance, ChanceLimit, ChanceLimitCombine    int64
	Far, Near, Seconds                         int64
	FarCombine, NearCombine, SecondsCombine    int64
	PriceX, GridAmount                         float64
	OpenShortMargin, CloseShortMargin          float64
	AmountLimit, AmountRate, AmountRateCombine float64
	TradeCost                                  float64
	ID                                         uint `gorm:"primary_key"`
	CreatedAt                                  time.Time
	UpdatedAt                                  time.Time
}

func (setting *Setting) GetKey() (key string) {
	return fmt.Sprintf("%s_%s_%s_%s", setting.Function, setting.Market, setting.Symbol, setting.Way)
}

func GetMsgKey(function, market, symbol string) (msgKey string) {
	msgKey = fmt.Sprintf("%s_%s_%s", function, market, symbol)
	return msgKey
}

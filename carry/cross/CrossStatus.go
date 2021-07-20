package cross

import (
	"hello/model"
	"sync"
	"time"
)

var status map[string]map[string]map[string]map[string]*CarryStatus // coin - market - symbol - key - CarryStatus
var usdAmount map[string]map[string]float64                         // key - market - amount
var okTradeMaxResetTime = make(map[string]map[string]int64)         // key - symbol - init time in second
var crossLock sync.Mutex
var crossing bool
var collaterals = make(map[string]*model.Collateral) // key - okex collateral status

type CarryStatus struct {
	Market                      string
	Symbol                      string
	LimitSell, LimitBuy         float64 // 最大可买卖币数
	TradeLineBuy, TradeLineSell float64 // 买卖盈利线（可为负数）
	Holding                     float64
	ValueInUsd                  float64
	RateInAll                   float64 // 当前币种或持仓占总权益的比例
}

func GetCollateral(key string) (collateral *model.Collateral) {
	defer crossLock.Unlock()
	crossLock.Lock()
	return collaterals[key]
}

func setCollateral(key string, collateral *model.Collateral) {
	defer crossLock.Unlock()
	crossLock.Lock()
	collaterals[key] = collateral
}

func getOKTradeMaxResetTime(key, symbol string) (resetTime int64) {
	defer crossLock.Unlock()
	crossLock.Lock()
	if okTradeMaxResetTime[key] == nil {
		return 0
	}
	return okTradeMaxResetTime[key][symbol]
}

func setOKTradeMaxResetTime(key, symbol string) {
	defer crossLock.Unlock()
	crossLock.Lock()
	if okTradeMaxResetTime[key] == nil {
		okTradeMaxResetTime[key] = make(map[string]int64)
	}
	okTradeMaxResetTime[key][symbol] = time.Now().Unix()
}

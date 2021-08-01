package cross

import (
	"hello/model"
	"sync"
	"time"
)

var status map[string]map[string]map[string]map[string]*CarryStatus // coin - market - symbol - key - CarryStatus
var usdAmount map[string]map[string]float64                         // key - market - amount
var okTradeMaxResetTime = make(map[string]map[string]int64)         // key - symbol - init time in second
var perpHoldInU map[string]map[string]float64                       // key - market - abs value in usd
var spotBalance map[string]map[string]float64                       // key - market - 统一账户或独立现货账户以usd计算的总权益
var perpMarginUsd map[string]map[string]float64                     // key - market - 期货账户中usd保证金数量
var balances map[string]map[string][]*model.Balance                 // key - market - balances
var positions map[string]map[string][]*model.Position               // key - market - positions
var crossLock sync.Mutex
var crossing bool
var collaterals = make(map[string]*model.Collateral) // key - okex collateral status

type CarryStatus struct {
	Market                      string
	Symbol                      string
	Type                        string  // spot or perp
	LimitSell, LimitBuy         float64 // 最大可买卖币数
	TradeLineBuy, TradeLineSell float64 // 买卖盈利线（可为负数）
	Holding                     float64
	ValueInUsd                  float64
	RateInAll                   float64 // 当前币种或持仓占总权益的比例
	IsUniAccount                bool    // 是否是统一账户
}

func getPerpMarginUsd(key, market string) float64 {
	if perpMarginUsd == nil || perpMarginUsd[key] == nil {
		return 0
	}
	return perpMarginUsd[key][market]
}

func getUsdAmount(key, market string) float64 {
	if usdAmount == nil || usdAmount[key] == nil {
		return 0
	}
	return usdAmount[key][market]
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

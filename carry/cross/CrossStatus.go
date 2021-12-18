package cross

import (
	"hello/model"
	"sync"
	"time"
)

var carryStatus = make(map[string]map[string]map[string]map[string]*CarryStatus) // coin - market - symbol - key - CarryStatus
var okTradeMaxResetTime = make(map[string]map[string]int64)                      // key - symbol - init time in second
var contractMarkets = make(map[string]map[string]*contractMarket)
var spotMarkets = make(map[string]map[string]*spotMarket)

var crossLock sync.Mutex
var crossing bool
var doCross = false

type contractMarket struct {
	key, market      string
	collateralsInU   float64 // 可用抵押币种价值总和，以U计算
	contractValueInU float64 // 当前价格下开仓总额，以U计算
	positions        map[string]*model.Position
}

type spotMarket struct {
	key, market     string
	availableU      float64
	accountValueInU float64
	balances        map[string]*model.Balance
	collateral      *model.Collateral
}

type CarryStatus struct {
	isSpot                      bool
	market, symbol, key, secret string
	LimitSell, LimitBuy         float64 // 最大可买卖币数
	TradeLineBuy, TradeLineSell float64 // 买卖盈利线（可为负数）
	Holding                     float64
	ValueInUsd                  float64
	RateInAll                   float64 // 现货：该币种占总权益的比例；永续：以开仓价算该币种持仓占保证金百分比
}

func getContractMarket(key, market string) *contractMarket {
	crossLock.Lock()
	defer crossLock.Unlock()
	if contractMarkets[key] == nil {
		return nil
	}
	return contractMarkets[key][market]
}

func setContractMarket(key, market string, cm *contractMarket) {
	crossLock.Lock()
	defer crossLock.Unlock()
	if contractMarkets[key] == nil {
		contractMarkets[key] = make(map[string]*contractMarket)
	}
	contractMarkets[key][market] = cm
}

func getSpotMarket(key, market string) *spotMarket {
	crossLock.Lock()
	defer crossLock.Unlock()
	if spotMarkets[key] == nil {
		spotMarkets[key] = make(map[string]*spotMarket)
	}
	return spotMarkets[key][market]
}

func setSpotMarket(key, market string, sm *spotMarket) {
	crossLock.Lock()
	defer crossLock.Unlock()
	if spotMarkets[key] == nil {
		spotMarkets[key] = make(map[string]*spotMarket)
	}
	spotMarkets[key][market] = sm
}

func getCarryStatus(coin, market, symbol, key string) *CarryStatus {
	crossLock.Lock()
	defer crossLock.Unlock()
	if carryStatus[coin] == nil || carryStatus[coin][market] == nil || carryStatus[coin][market][symbol] == nil {
		return nil
	}
	return carryStatus[coin][market][symbol][key]
}

func setCarryStatus(coin, market, symbol, key string, status *CarryStatus) {
	crossLock.Lock()
	defer crossLock.Unlock()
	if carryStatus[coin] == nil {
		carryStatus[coin] = make(map[string]map[string]map[string]*CarryStatus)
	}
	if carryStatus[coin][market] == nil {
		carryStatus[coin][market] = make(map[string]map[string]*CarryStatus)
	}
	if carryStatus[coin][market][symbol] == nil {
		carryStatus[coin][market][symbol] = make(map[string]*CarryStatus)
	}
	carryStatus[coin][market][symbol][key] = status
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

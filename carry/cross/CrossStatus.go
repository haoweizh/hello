package cross

import (
	"hello/model"
	"sync"
)

var carryStatus = make(map[string]map[string]map[string]map[string]*CarryStatus) // coin - market - symbol - key - CarryStatus
var contractMarkets = make(map[string]*contractMarket)                           // key - contractMarket
var spotMarkets = make(map[string]*spotMarket)                                   // key - spotMarket
var crossLock sync.Mutex
var crossing bool
var doCross = false

const holdingLimitInU = 500000.0
const openValueLimit = 10000.0

type contractMarket struct {
	key, market      string
	collateralsInU   float64 // 可用抵押币种价值总和（目前只有U）
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
	//ValueInUsd                  float64
	RateInAll float64 // 现货：该币种占总权益的比例；永续：以开仓价算该币种持仓占保证金百分比
}

func getCarryStop(key string) (stop bool) {
	defer carryLock.Unlock()
	carryLock.Lock()
	return carryStop[key]
}

func GetCrossMarketValue(key string) (market string, inAllSpot, collateral, holdingSpot, holdingFuture, unRealizedPnl float64) {
	if spotMarkets[key] != nil {
		market = spotMarkets[key].market
		inAllSpot = spotMarkets[key].accountValueInU
		holdingSpot = spotMarkets[key].accountValueInU - spotMarkets[key].availableU
	}
	if contractMarkets[key] != nil {
		if market == `` {
			market = contractMarkets[key].market
		}
		collateral = contractMarkets[key].collateralsInU
		for _, position := range contractMarkets[key].positions {
			unRealizedPnl += position.ProfitUnreal
		}
		holdingFuture = contractMarkets[key].contractValueInU
	}
	return
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

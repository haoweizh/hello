package carry

import (
	"hello/model"
	"hello/util"
)

var carryFail = make(map[string]int64)                        // key fail num
var carryStop = make(map[string]bool)                         // key - stop carry bool
var tradeMaxResetTime = make(map[string]int64)                // key - init time in second
var collaterals = make(map[string]*model.Collateral)          // key - okex collateral status
var usdAvailable = make(map[string]float64)                   // key - float64
var usdRate = make(map[string]float64)                        // key - float64
var balanceAll = make(map[string]float64)                     // key - balance value in all
var carryBalance = make(map[string]map[string]*model.Balance) // key - coin - balance
var posBal = make(map[string]float64)                         // key - coin - position balance
var carryAmount = make(map[string]map[string]float64)         // key - perp - float64
var tradeMax = make(map[string]map[string][]float64)          // key - instrument - [maxBuy合约张数/币币个数, maxSell]
var tradeMaxResetting = make(map[string]bool)                 // key - bool

func getTradeMaxResetting(key string) bool {
	defer carryLock.Unlock()
	carryLock.Lock()
	return tradeMaxResetting[key]
}

func setTradeMaxResetting(key string, value bool) {
	defer carryLock.Unlock()
	carryLock.Lock()
	tradeMaxResetting[key] = value
}

func getCarryStop(key string) (stop bool) {
	defer carryLock.Unlock()
	carryLock.Lock()
	return carryStop[key]
}
func GetCarryResult(key string) int64 {
	defer carryLock.Unlock()
	carryLock.Lock()
	return carryFail[key]
}

func addCarryResult(key string, success bool) {
	defer carryLock.Unlock()
	carryLock.Lock()
	if success {
		if carryFail[key] > 0 {
			carryFail[key] -= 1
		}
	} else {
		carryFail[key] += 2
		util.Notice(`---------- fail size %s %d`, key, carryFail[key])
	}
	if carryFail[key] > 6 {
		carryStop[key] = true
		util.Notice(`----------stop carry %s %d`, key, carryFail[key])
		carryFail[key] = 0
	}
}

func getPosBal(key string) (value float64) {
	defer carryLock.Unlock()
	carryLock.Lock()
	return posBal[key]
}

func setPosBal(key string, value float64) {
	defer carryLock.Unlock()
	carryLock.Lock()
	posBal[key] = value
}

func getTradeMaxResetTime(key string) (resetTime int64) {
	defer carryLock.Unlock()
	carryLock.Lock()
	return tradeMaxResetTime[key]
}

func setTradeMaxResetTime(key string, resetTime int64) {
	defer carryLock.Unlock()
	carryLock.Lock()
	tradeMaxResetTime[key] = resetTime
}

func GetCollateral(key string) (collateral *model.Collateral) {
	defer carryLock.Unlock()
	carryLock.Lock()
	return collaterals[key]
}

func setCollateral(key string, collateral *model.Collateral) {
	defer carryLock.Unlock()
	carryLock.Lock()
	collaterals[key] = collateral
}

func getTradeMax(key, instrument string) (maxBuy, maxSell float64) {
	defer carryLock.Unlock()
	carryLock.Lock()
	if tradeMax[key] == nil || tradeMax[key][instrument] == nil || len(tradeMax[key][instrument]) != 2 {
		return 0, 0
	}
	return tradeMax[key][instrument][0], tradeMax[key][instrument][1]
}

func setTradeMax(key, instrument string, maxBuy, maxSell float64) {
	defer carryLock.Unlock()
	carryLock.Lock()
	if tradeMax[key] == nil {
		tradeMax[key] = make(map[string][]float64)
	}
	tradeMax[key][instrument] = []float64{maxBuy, maxSell}
}

func getUsdAvailable(key string) float64 {
	carryLock.Lock()
	defer carryLock.Unlock()
	return usdAvailable[key]
}

func setUsdAvailable(key string, value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	usdAvailable[key] = value
}

func setBalanceAll(key string, value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	balanceAll[key] = value
}

func getBalanceAll(key string) (value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	return balanceAll[key]
}

func getUsdRate(key string) float64 {
	carryLock.Lock()
	defer carryLock.Unlock()
	return usdRate[key]
}

func setUsdRate(key string, value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	usdRate[key] = value
}

func getCarryAmount(key, perp string) float64 {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryAmount[key] == nil {
		return 0
	}
	return carryAmount[key][perp]
}

func setCarryAmount(key, perp string, amount float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryAmount[key] == nil {
		carryAmount[key] = make(map[string]float64)
	}
	carryAmount[key][perp] = amount
}

func setCarryBalance(key, coin string, balance *model.Balance) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryBalance[key] == nil {
		carryBalance[key] = make(map[string]*model.Balance)
	}
	carryBalance[key][coin] = balance
}

func getCarryBalance(key, coin string) (balance *model.Balance) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryBalance[key] == nil {
		return nil
	}
	return carryBalance[key][coin]
}

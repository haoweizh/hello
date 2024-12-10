package deprecated

//
//import (
//	"hello/model"
//	"hello/util"
//	"time"
//)
//
//func GetCarryCoins() (coins map[string]map[string]bool) { //  market - coin - bool
//	markets := GetMarkets()
//	coins = make(map[string]map[string]bool)
//	for _, market := range markets {
//		coins[market] = make(map[string]bool)
//		if marketInfos != nil && GetSettings(FunctionCarry, market) != nil {
//			symbols := GetMarketSymbols(market)
//			for symbol := range symbols {
//				coin := GetCoin(market, symbol)
//				coins[market][coin] = true
//			}
//		}
//	}
//	return coins
//}
//var carryFail = make(map[string]int64)                        // key fail num
//var carryStop = make(map[string]bool)                         // key - stop carry bool
//var tradeMaxResetTime = make(map[string]int64)                // key - init time in second
//var collaterals = make(map[string]*model.Collateral)          // key - okex collateral status
//var usdAvailable = make(map[string]float64)                   // key - float64
//var usdRate = make(map[string]float64)                        // key - float64
//var balanceAll = make(map[string]float64)                     // key - balance value in all
//var carryBalance = make(map[string]map[string]*model.Balance) // key - coin - balance
//var posBal = make(map[string]float64)                         // key - coin - position balance
//var tradeMaxResetting = make(map[string]bool)                 // key - bool
//var recentSymbol = make(map[string]time.Time)                 // (market-symbol)-time
//
//func isRecentCarry(market, symbol string) (value bool) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	duration, _ := time.ParseDuration(`30s`)
//	if time.Now().Before(recentSymbol[market+symbol].Add(duration)) {
//		return true
//	} else {
//		delete(recentSymbol, market+symbol)
//	}
//	return false
//}
//
//func setRecentCarryTime(market, symbol string) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	recentSymbol[market+symbol] = time.Now()
//}
//
//func getTradeMaxResetting(key string) bool {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	return tradeMaxResetting[key]
//}
//
//func setTradeMaxResetting(key string, value bool) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	tradeMaxResetting[key] = value
//}
//
//func getCarryStop(key string) (stop bool) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	return carryStop[key]
//}
//func GetCarryResult(key string) int64 {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	return carryFail[key]
//}
//
//func pauseCarry(key string) {
//	util.Notice(`%s carrying pause %#v`, key, true)
//	carryStop[key] = true
//	time.Sleep(time.Minute * 30)
//	util.Notice(`%s carrying pause %#v`, key, false)
//	carryStop[key] = false
//}
//
//func addCarryResult(key, market string, success bool) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	if success {
//		if carryFail[key] > 0 {
//			carryFail[key] = carryFail[key] - 1
//		}
//	} else {
//		carryFail[key] += 2
//	}
//	if carryFail[key] > 0 {
//		util.Notice(`---------- fail size %s %d`, key, carryFail[key])
//	}
//	if carryFail[key] > 6 {
//		go pauseCarry(key)
//		//accounts := model.AppConfig.GetAccounts(market)
//		//account := model.AppConfig.GetAccountFromKey(market, key)
//		//if accounts[0].Key == account.Key {
//		//	mailAddr = model.AppConfig.Mail
//		//}
//		util.Notice(`----------stop carry %s %d`, key, carryFail[key])
//		carryFail[key] = 0
//		for _, address := range model.TeamMails {
//			_ = util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, address,
//				`暂停下单`, `market: `+market+` stop `+key)
//		}
//	}
//}
//
//func getPosBal(key string) (value float64) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	return posBal[key]
//}
//
//func setPosBal(key string, value float64) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	posBal[key] = value
//}
//
//func getTradeMaxResetTime(key string) (resetTime int64) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	return tradeMaxResetTime[key]
//}
//
//func setTradeMaxResetTime(key string, resetTime int64) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	tradeMaxResetTime[key] = resetTime
//}
//
//func GetCollateral(key string) (collateral *model.Collateral) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	return collaterals[key]
//}
//
//func setCollateral(key string, collateral *model.Collateral) {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	collaterals[key] = collateral
//}
//
//func getUsdAvailable(key string) float64 {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	return usdAvailable[key]
//}
//
//func setUsdAvailable(key string, value float64) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	usdAvailable[key] = value
//}
//
//func setBalanceAll(key string, value float64) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	balanceAll[key] = value
//}
//
//func getBalanceAll(key string) (value float64) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	return balanceAll[key]
//}
//
//func getUsdRate(key string) float64 {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	return usdRate[key]
//}
//
//func setUsdRate(key string, value float64) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	usdRate[key] = value
//}
//
//func setCarryBalance(key, coin string, balance *model.Balance) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	if carryBalance[key] == nil {
//		carryBalance[key] = make(map[string]*model.Balance)
//	}
//	carryBalance[key][coin] = balance
//}
//
//func getCarryBalance(key, coin string) (balance *model.Balance) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	if carryBalance[key] == nil {
//		return nil
//	}
//	return carryBalance[key][coin]
//}
//
////var carryAmount = make(map[string]map[string]float64)         // key - perp - float64
////func getCarryAmount(key, perp string) float64 {
////	carryLock.Lock()
////	defer carryLock.Unlock()
////	if carryAmount[key] == nil {
////		return 0
////	}
////	return carryAmount[key][perp]
////}
////
////func setCarryAmount(key, symbol string, amount float64) {
////	carryLock.Lock()
////	defer carryLock.Unlock()
////	if carryAmount[key] == nil {
////		carryAmount[key] = make(map[string]float64)
////	}
////	carryAmount[key][symbol] = amount
////}

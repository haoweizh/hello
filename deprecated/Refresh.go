package deprecated

//import (
//	"fmt"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"time"
//)
//
//var chance = make(map[string]map[string]int64)                  // key - (market-symbol) - int
//var marketBalances = make(map[string]map[string]*model.Balance) // key - (market-coin) - *balance
//var updateTime = make(map[string]map[string]time.Time)          // key - market = time
//var refreshing = false
//
//// ProcessRefresh
//// setting.OpenShortMargin 币种占账户总价值的比例
//// setting.Chance 要进行多少次刷单交易
//// setting.GridAmount 下单数量
//// setting.AmountLimit 下单后休息时间in million second
//// setting.PriceX 买卖单价差大于等于多少才会下单
//var ProcessRefresh = func(setting *model.Setting, tick *model.BidAsk) {
//	if !refreshing {
//		defer setRefreshing(false)
//		setRefreshing(true)
//	} else {
//		return
//	}
//	million := util.GetNowUnixMillion()
//	delayTick := int64(0)
//	if tick != nil {
//		delayTick = million - int64(tick.Ts)
//	}
//	if delayTick < 800 {
//		delayTick = 30
//	}
//	account := model.AppConfig.GetAccounts(setting.Market)[0]
//	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || model.AppPause ||
//		(model.AppConfig.Env != `test` && (model.AppConfig.HandleLink != `1` || model.IsTickTimeout(setting.Market, delayTick))) ||
//		tick.Asks[0].Price-tick.Bids[0].Price < setting.PriceX {
//		return
//	}
//	if validBalance(account.Key, account.Secret, setting, tick) {
//		placeRefresh(account.Key, account.Secret, setting, tick.Bids[0].Price, tick.Asks[0].Price)
//		time.Sleep(time.Duration(setting.AmountLimit) * time.Millisecond)
//	}
//}
//
//func queryBalances(key, secret, market string) {
//	success, balances, _, _ := api.GetBalances(key, secret, market)
//	if success {
//		if updateTime[key] == nil {
//			updateTime[key] = make(map[string]time.Time)
//		}
//		updateTime[key][market] = time.Now()
//		for _, balance := range balances {
//			if marketBalances[key] == nil {
//				marketBalances[key] = make(map[string]*model.Balance)
//			}
//			marketBalances[key][market+`-`+balance.Coin] = balance
//		}
//	}
//}
//
//func validBalance(key, secret string, setting *model.Setting, tick *model.BidAsk) (valid bool) {
//	duration, _ := time.ParseDuration(`60s`)
//	if marketBalances[key] == nil || updateTime[key] == nil || updateTime[key][setting.Market].Add(duration).Before(time.Now()) {
//		api.CancelOrders(key, secret, setting.Market, setting.Symbol)
//		time.Sleep(time.Duration(2) * time.Second)
//		queryBalances(key, secret, setting.Market)
//		return false
//	}
//	coins := model.GetSpotCoins(setting.Market, setting.Symbol)
//	if marketBalances[key] == nil || updateTime[key] == nil {
//		return false
//	}
//	left := marketBalances[key][setting.Market+`-`+coins[0]]
//	right := marketBalances[key][setting.Market+`-`+coins[1]]
//	var order *model.Order
//	if left != nil && right != nil {
//		rate := left.Amount * tick.Asks[0].Price / right.Amount
//		if rate < 1.1*setting.OpenShortMargin && rate > 0.9*setting.OpenShortMargin {
//			return true
//		} else if rate > setting.OpenShortMargin {
//			amount := (rate - setting.OpenShortMargin) / 2 * (right.Amount / tick.Asks[0].Price)
//			pos := int(math.Min(4, float64(tick.Bids.Len()-1)))
//			order = api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
//				``, model.FunctionRefresh, tick.Bids[pos].Price, tick.Bids[pos].Price, amount, true,
//				false, nil, setting)
//		} else if rate < setting.OpenShortMargin {
//			amount := (setting.OpenShortMargin - rate) / 2 * (right.Amount / tick.Asks[0].Price)
//			pos := int(math.Min(4, float64(tick.Asks.Len()-1)))
//			order = api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
//				``, model.FunctionRefresh, tick.Asks[pos].Price, tick.Asks[pos].Price, amount, true,
//				false, nil, setting)
//		}
//		if order != nil && order.Status == model.CarryStatusWorking {
//			util.Notice(fmt.Sprintf(`coin rate=%f left:%s %f right:%s %f`,
//				rate, left.Coin, left.Amount, right.Coin, right.Amount/tick.Asks[0].Price))
//			time.Sleep(time.Duration(2) * time.Second)
//			api.CancelOrders(key, secret, setting.Market, setting.Symbol)
//		}
//	}
//	return false
//}
//
//func setRefreshing(value bool) {
//	refreshing = value
//}
//
//func getChance(key, market, symbol string) (value int64) {
//	if chance[key] == nil {
//		return 0
//	}
//	return chance[key][market+`-`+symbol]
//}
//
//func addChance(key, market, symbol string) {
//	if chance[key] == nil {
//		chance[key] = make(map[string]int64)
//	}
//	chance[key][market+`-`+symbol]++
//}
//
//func placeRefresh(key, secret string, setting *model.Setting, priceBuy, priceSell float64) {
//	addChance(key, setting.Market, setting.Symbol)
//	if getChance(key, setting.Market, setting.Symbol) > setting.Chance {
//		return
//	}
//	util.Notice(fmt.Sprintf(`place refresh %f index: %d`, setting.GridAmount, getChance(key, setting.Market, setting.Symbol)))
//	price := (priceBuy + priceSell) / 2
//	go api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
//		``, model.FunctionRefresh, price, price, setting.GridAmount, true, false, nil, setting)
//	api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
//		``, model.FunctionRefresh, price, price, setting.GridAmount, true, false, nil, setting)
//	time.Sleep(time.Duration(2) * time.Second)
//	api.CancelOrders(key, secret, setting.Market, setting.Symbol)
//}

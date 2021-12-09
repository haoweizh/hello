package refresh

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"sync"
	"time"
)

var refreshLock sync.Mutex
var buyMore = make(map[string]map[string]float64) // key - (market-symbol) - float
var chance = make(map[string]map[string]int64)    // key - (market-symbol) - int
//var marketBalances = make(map[string]map[string]*model.Balance) // key - coin - *balance
//var updateBalance = make(map[string]bool)                       // market - bool

// ProcessRefresh
// setting.OpenShortMargin 刚启动时buyMore数量
// setting.Chance 要进行多少次刷单交易
// setting.GridAmount 下单个数
// setting.AmountLimit 下单后休息时间in million second
// setting.PriceX 买卖单价差大于等于多少才会下单
var ProcessRefresh = func(setting *model.Setting, tick *model.BidAsk) {
	defer refreshLock.Unlock()
	refreshLock.Lock()
	//million := util.GetNowUnixMillion()
	//delayTick := int64(0)
	//if tick != nil {
	//	delayTick = million - int64(tick.Ts)
	//}
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || model.AppPause ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) || len(keys) != 1 ||
		tick.Asks[0].Price-tick.Bids[0].Price < setting.PriceX { //} || model.IsTickTimeout(setting.Market, delayTick) {
		return
	}
	placeRefresh(setting, keys[0], secrets[0], tick.Bids[0].Price, tick.Asks[0].Price)
	time.Sleep(time.Duration(setting.AmountLimit) * time.Millisecond)
}

func getChance(key, market, symbol string) (value int64) {
	if chance[key] == nil {
		return 0
	}
	return chance[key][market+`-`+symbol]
}

func addChance(key, market, symbol string) {
	if chance[key] == nil {
		chance[key] = make(map[string]int64)
	}
	chance[key][market+`-`+symbol]++
}

func getBuyMore(key, market, symbol string) (value float64) {
	if buyMore[key] == nil {
		return 0
	}
	return buyMore[key][market+`-`+symbol]
}

func addBuyMore(key, market, symbol string, amount float64) {
	if buyMore[key] == nil {
		buyMore[key] = make(map[string]float64)
	}
	buyMore[key][market+`-`+symbol] += amount
}

//func refreshBalances() {
//	for true {
//		for market := range updateBalance {
//			if updateBalance[market] {
//				keys, secrets := model.AppConfig.GetKeys(market)
//				for i, key := range keys {
//					success, balances, _, _ := api.GetBalances(key, secrets[i], market)
//					if success {
//						for _, balance := range balances {
//							if marketBalances[key] == nil {
//								marketBalances[key] = make(map[string]*model.Balance)
//								marketBalances[key][balance.Coin] = balance
//							}
//						}
//					}
//				}
//			}
//		}
//		time.Sleep(time.Minute)
//	}
//}

func placeRefresh(setting *model.Setting, key, secret string, priceBuy, priceSell float64) {
	if setting.OpenShortMargin != 0 {
		addBuyMore(key, setting.Market, setting.Symbol, setting.OpenShortMargin)
		setting.OpenShortMargin = 0
		model.AppDB.Save(setting)
		return
	}
	addChance(key, setting.Market, setting.Symbol)
	if getChance(key, setting.Market, setting.Symbol) > setting.Chance {
		return
	}
	localBuyMore := getBuyMore(key, setting.Market, setting.Symbol)
	if setting.GridAmount/3 < localBuyMore {
		go api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
			``, model.FunctionRefresh, priceBuy, priceBuy, localBuyMore, true, false, postOrderRefresh, setting)
	} else if setting.GridAmount/3 < -1*localBuyMore {
		go api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
			``, model.FunctionRefresh, priceSell, priceSell, -1*localBuyMore, true, false, postOrderRefresh, setting)
	} else {
		util.Notice(fmt.Sprintf(`place refresh %f index: %d`, setting.GridAmount, getChance(key, setting.Market, setting.Symbol)))
		price := (priceBuy + priceSell) / 2
		go api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
			``, model.FunctionRefresh, price, price, setting.GridAmount, true, false, postOrderRefresh, setting)
		go api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
			``, model.FunctionRefresh, price, price, setting.GridAmount, true, false, postOrderRefresh, setting)
	}
}

var postOrderRefresh = func(order *model.Order, setting *model.Setting) {
	if setting == nil {
		return
	}
	//if !updateBalance[setting.Market] {
	//	updateBalance[setting.Market] = true
	//	go refreshBalances()
	//}
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	var key, secret string
	for i, value := range keys {
		if value == order.AmountType {
			key = value
			secret = secrets[i]
		}
	}
	time.Sleep(time.Duration(setting.AmountLimit) * time.Millisecond)
	if order != nil && order.Status == model.CarryStatusWorking {
		order = api.QueryOrderById(key, secret, setting.Market, setting.Symbol, setting.Symbol, model.OrderTypeLimit, order.OrderId)
		if order.OrderSide == model.OrderSideBuy {
			addBuyMore(order.AmountType, setting.Market, setting.Symbol, order.DealAmount)
		} else if order.OrderSide == model.OrderSideSell {
			addBuyMore(order.AmountType, setting.Market, setting.Symbol, order.DealAmount*-1)
		}
		util.Notice(fmt.Sprintf(`refresh %s <%f %f> buy more: %f`, order.OrderSide, order.Amount, order.DealAmount,
			getBuyMore(order.AmountType, setting.Market, setting.Symbol)))
		if order.Status == model.CarryStatusWorking {
			api.MustCancel(key, secret, setting.Market, setting.Symbol, setting.Symbol, model.OrderTypeLimit, order.OrderId, true)
		}
	}
}

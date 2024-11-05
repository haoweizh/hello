package queue

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strings"
	"sync"
	"time"
)

type DataQueue struct {
	account, accountLiq                                    *model.Account
	setting                                                *model.Setting
	baseAvaWithBow, quoteAvaWithBow, baseHold, BaseHoldLiq float64
	QueueOrders                                            map[string]*model.Order
	updatedInMilli                                         int64
}

var DataMap = &sync.Map{} // market*symbol-*DataQueue

// ProcessQueue
// setting.OpenShortMargin 平仓绝对值价差
// setting.AmountLimit要求Tick买卖1的计价币种最低挂单数量要求，不达要求不排队
// setting.RateRelated related symbol对应的price乘数,price*settingRelated
var ProcessQueue = func(setting *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetProcessing(model.FunctionQueue, model.FunctionQueue, model.FunctionQueue, true) {
		defer api.CheckSetProcessing(model.FunctionQueue, model.FunctionQueue, model.FunctionQueue, false)
	} else {
		return
	}
	now := util.GetNowUnixMillion()
	maintaining, _ := model.ChannelMaintaining.Load(setting.Market)
	_, tickLiq := model.AppEnvironment.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
	if tick == nil || tick.Asks == nil || len(tick.Asks) == 0 || tick.Bids == nil || len(tick.Bids) == 0 ||
		model.AppConfig.Handle != `1` || tickLiq == nil || tickLiq.Asks == nil ||
		tickLiq.Bids == nil || len(tickLiq.Bids) == 0 || len(tickLiq.Asks) == 0 ||
		(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 1000) {
		return
	}
	cache, data := GetData(setting, false)
	if !cache || data == nil {
		return
	}
	canceled := false
	for _, order := range data.QueueOrders {
		if tick.Bids[0].Price == order.Price && order.OrderSide == model.OrderSideBuy {
			continue
		}
		if tick.Asks[0].Price == order.Price && order.OrderSide == model.OrderSideSell {
			continue
		}
		canceled = true
		delete(data.QueueOrders, order.OrderId)
		util.Notice(`cancel not bid ask 1 order %s %s %s`, setting.Market, setting.Symbol, order.OrderId)
		api.CancelOrder(data.account.Key, data.account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId)
		time.Sleep(time.Second)
	}
	if canceled {
		return
	}
	queueCode, cancelBuy, cancelSell := canQueue(setting, tick)
	if queueCode < 0 {
		for _, order := range data.QueueOrders {
			if (cancelBuy && order.OrderSide == model.OrderSideBuy) || (cancelSell && order.OrderSide == model.OrderSideSell) {
				success, _, _ := api.CancelOrder(data.account.Key, data.account.Secret, order.Market, order.Symbol,
					model.OrderTypeLimit, order.OrderId)
				if success {
					delete(data.QueueOrders, order.OrderId)
					util.Notice(fmt.Sprintf(`clear all order when can not queue code %d %s %s %s left %d tick [%e %e %e %e] liq[%e %e %e %e]`,
						queueCode, setting.Market, setting.Symbol, order.OrderId, len(data.QueueOrders), tick.Bids[0].Price, tick.Bids[0].Amount,
						tick.Asks[0].Price, tick.Asks[0].Amount, tickLiq.Bids[0].Price, tickLiq.Bids[0].Amount, tickLiq.Asks[0].Price, tickLiq.Asks[0].Amount))
					time.Sleep(time.Second)
				}
			}
		}
		return
	}
	if placeQueue(setting, data, tick) {
		time.Sleep(time.Second)
		util.DelSyncMap(DataMap, setting.Market, setting.Symbol)
		return
	}
	if liqQueue(setting, data, tick, tickLiq) {
		time.Sleep(time.Second)
		util.DelSyncMap(DataMap, setting.Market, setting.Symbol)
	}
}

func liqQueue(setting *model.Setting, data *DataQueue, tick, tickLiq *model.BidAsk) (placed bool) {
	holdInQuote := data.baseHold*tick.Bids[0].Price + data.BaseHoldLiq*tickLiq.Bids[0].Price
	if holdInQuote > 20 && tickLiq.Bids[0].Price*setting.RateRelated-tick.Bids[0].Price > setting.OpenShortMargin {
		util.Notice(fmt.Sprintf(`try to liq queue order %s %s %s %s value %f amt %f liq price %e > %e price tick %e liq %e`,
			setting.Function, setting.MarketRelated, setting.SymbolRelated, model.OrderSideSell, holdInQuote,
			holdInQuote/tickLiq.Bids[0].Price, tickLiq.Bids[0].Price*setting.RateRelated, setting.OpenShortMargin, tick.Bids[0].Price, tickLiq.Bids[0].Price))
		order := api.PlaceOrder(data.accountLiq.Key, data.accountLiq.Secret, model.OrderSideSell, model.OrderTypeMarket,
			setting.MarketRelated, setting.SymbolRelated, ``, model.FunctionQueue, tickLiq.Bids[0].Price, tickLiq.Bids[0].Price,
			holdInQuote/tickLiq.Bids[0].Price, false, nil, setting)
		model.AppDB.Save(&order)
		return true
	} else if holdInQuote < -20 && tick.Asks[0].Price-tickLiq.Asks[0].Price*setting.RateRelated > setting.OpenShortMargin {
		util.Notice(fmt.Sprintf(`try to liq queue order %s %s %s %s value %f amt %f %e > %e price tick %e liq %e`,
			setting.Function, setting.MarketRelated, setting.SymbolRelated, model.OrderSideBuy, holdInQuote,
			holdInQuote/tickLiq.Asks[0].Price, tick.Asks[0].Price-tickLiq.Asks[0].Price*setting.RateRelated, setting.OpenShortMargin, tick.Asks[0].Price, tickLiq.Asks[0].Price))
		order := api.PlaceOrder(data.accountLiq.Key, data.accountLiq.Secret, model.OrderSideBuy, model.OrderTypeMarket,
			setting.MarketRelated, setting.SymbolRelated, ``, model.FunctionQueue, tickLiq.Asks[0].Price, tickLiq.Asks[0].Price,
			math.Abs(holdInQuote)/tickLiq.Asks[0].Price, false, nil, setting)
		model.AppDB.Save(&order)
		return true
	}
	return false
}

func placeQueue(setting *model.Setting, data *DataQueue, tick *model.BidAsk) (placed bool) {
	var orders []*model.Order
	if data.baseAvaWithBow*tick.Bids[0].Price > 20 {
		util.Notice(fmt.Sprintf(`try to place queue order %s %s %s %s at %e amt %e`,
			setting.Function, setting.Market, setting.Symbol, model.OrderSideSell, tick.Asks[0].Price,
			data.baseAvaWithBow-10/tick.Asks[0].Price))
		orders = api.MustPlaceOrder(data.account.Key, data.account.Secret, model.OrderSideSell, model.OrderTypeLimit, setting.Market,
			setting.Symbol, ``, setting.Function, tick.Asks[0].Price, tick.Asks[0].Price,
			data.baseAvaWithBow-10/tick.Asks[0].Price, setting)
		for _, order := range orders {
			if order == nil || !order.HaveId() {
				continue
			}
			model.AppDB.Save(&order)
			data.QueueOrders[order.OrderId] = order
			util.Notice(fmt.Sprintf(`place queue order %s %s %s %s at %e amt %e return %s`,
				setting.Function, setting.Market, setting.Symbol, model.OrderSideBuy, order.Price, data.baseAvaWithBow, order.OrderId))
		}
		placed = true
	}
	if data.quoteAvaWithBow > 20 {
		util.Notice(fmt.Sprintf(`try to place queue order %s %s %s %s at %e amt in u %f amt %e`,
			setting.Function, setting.Market, setting.Symbol, model.OrderSideBuy, tick.Bids[0].Price, data.quoteAvaWithBow,
			(data.quoteAvaWithBow-10)/tick.Bids[0].Price))
		orders = api.MustPlaceOrder(data.account.Key, data.account.Secret, model.OrderSideBuy, model.OrderTypeLimit, setting.Market,
			setting.Symbol, ``, setting.Function, tick.Bids[0].Price, tick.Bids[0].Price,
			(data.quoteAvaWithBow-10)/tick.Bids[0].Price, setting)
		for _, order := range orders {
			if order == nil {
				continue
			}
			model.AppDB.Save(&order)
			data.QueueOrders[order.OrderId] = order
			util.Notice(fmt.Sprintf(`place queue order %s %s %s %s at %e amt %e return %s`,
				setting.Function, setting.Market, setting.Symbol, model.OrderSideSell, order.Price, data.quoteAvaWithBow/tick.Bids[0].Price, order.OrderId))
		}
		placed = true
	}
	return placed
}

func canQueue(setting *model.Setting, tick *model.BidAsk) (code int, cancelBuy, cancelSell bool) {
	v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
	if v == nil {
		return -1, true, true
	}
	marketInfo := v.(*model.MarketInfo)
	priceDis := tick.Asks[0].Price - tick.Bids[0].Price - marketInfo.PriceIncrement*1.1
	if priceDis > 0 {
		return -2, true, true
	}
	if tick.Asks[0].Price*tick.Asks[0].Amount < setting.AmountLimit && tick.Bids[0].Amount > tick.Asks[0].Amount*10 {
		return -3, false, true
	}
	if tick.Bids[0].Price*tick.Bids[0].Amount < setting.AmountLimit && tick.Asks[0].Amount > 10*tick.Bids[0].Amount {
		return -4, true, false
	}
	//if tick.Bids[0].Price >= tickLiq.Bids[0].Price*setting.RateRelated || tick.Asks[0].Price <= tickLiq.Asks[0].Price*setting.RateRelated {
	//	return -5
	//}
	return 1, false, false
}

func GetData(setting *model.Setting, refresh bool) (cache bool, data *DataQueue) {
	now := time.Now().UnixMilli()
	value, _ := util.LoadSyncMap(DataMap, setting.Market, setting.Symbol)
	if value != nil && now-value.(*DataQueue).updatedInMilli < 60000 && !refresh {
		return true, value.(*DataQueue)
	}
	data = &DataQueue{account: model.AppConfig.GetAccounts(setting.Market)[0], QueueOrders: make(map[string]*model.Order),
		accountLiq: model.AppConfig.GetAccounts(setting.MarketRelated)[0], setting: setting}
	orders := api.QueryOpenOrders(data.account.Key, data.account.Secret, setting.Market, setting.Symbol)
	for _, order := range orders {
		if order != nil {
			data.QueueOrders[order.OrderId] = order
			//util.Notice(fmt.Sprintf(`add order into queue %s %s %s`, order.Market, order.Symbol, order.OrderId))
		}
	}
	var success1, success2, success3, success4 bool
	success1, data.baseHold = api.GetHolding(data.account, setting.Market, setting.Symbol)
	success2, data.BaseHoldLiq = api.GetHolding(data.accountLiq, setting.MarketRelated, setting.SymbolRelated)
	success3, data.quoteAvaWithBow = getAvailable(data.account, setting.Market, `usdt`)
	_, _, coin, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	success4, data.baseAvaWithBow = getAvailable(data.account, setting.Market, coin)
	//util.Notice(fmt.Sprintf(`get data %v %v %v %v %e %e %e %e`,
	//	success1, success2, success3, success4, data.baseHold, data.BaseHoldLiq, data.quoteAvaWithBow, data.baseAvaWithBow))
	if success1 && success2 && success3 && success4 {
		data.updatedInMilli = now
		util.StoreSyncMap(DataMap, data, setting.Market, setting.Symbol)
		return false, data
	} else {
		time.Sleep(time.Minute)
		util.DelSyncMap(DataMap, setting.Market, setting.Symbol)
	}
	return false, nil
}

func getAvailable(account *model.Account, market, coin string) (success bool, holding float64) {
	_, balances, _, _ := api.GetBalances(account.Key, account.Secret, market)
	for _, balance := range balances {
		if strings.EqualFold(balance.Coin, coin) {
			return true, balance.AvailableWithBorrow
		}
	}
	return false, 0
}

var ProcessQueueLiq = func(order *model.Order) {
	for {
		if !api.CheckSetProcessing(model.FunctionQueue, model.FunctionQueue, model.FunctionQueue, true) {
			break
		} else {
			time.Sleep(time.Second)
		}
	}
	defer api.CheckSetProcessing(model.FunctionQueue, model.FunctionQueue, model.FunctionQueue, false)
	if order == nil || !order.HaveId() {
		return
	}
	model.AppDB.Model(order).Where(`order_id=?`, order.OrderId).Updates(map[string]interface{}{
		`deal_price`: order.DealPrice, `deal_amount`: order.DealAmount, `fee`: order.Fee, `status`: order.Status})
	accounts := model.AppConfig.GetAccounts(order.Market)
	setting := api.GetSetting(model.FunctionQueue, order.Market, order.Symbol)
	if setting == nil {
		return
	}
	for _, account := range accounts {
		_, tickRelated := model.AppEnvironment.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
		if account != nil && tickRelated != nil && tickRelated.Bids != nil && len(tickRelated.Bids) > 0 &&
			tickRelated.Asks != nil && len(tickRelated.Asks) > 0 {
			if order.Status == model.CarryStatusSuccess {
				util.Notice(fmt.Sprintf(`get order success, refresh data queue %v`, order))
				util.DelSyncMap(DataMap, setting.Market, setting.Symbol)
			}
		}
	}
}

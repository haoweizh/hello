package cross

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"sync"
	"time"
)

type DataQueue struct {
	Coin        string
	QueueOrders map[string]*model.Order
	Statuses    []*CarryStatus // map[market*symbol]*CarryStatus
	Settings    []*model.Setting
}

var QueueDataMap sync.Map // coin-*DataQueue
var DoQueue = false

// ProcessQueue
// setting.AmountLimit要求Tick买卖1的计价币种最低挂单数量要求，不达要求不排队
// setting.OpenShortMargin要求下单计价币种最低挂单数量要求，不达要求不排队
var ProcessQueue = func(setting *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetProcessing(model.FunctionQueue, model.FunctionQueue, model.FunctionQueue, true) {
		defer api.CheckSetProcessing(model.FunctionQueue, model.FunctionQueue, model.FunctionQueue, false)
	} else {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	if !DoQueue && model.AppConfig.Handle == `1` {
		go EqualQueue(account)
		DoQueue = true
		return
	}
	now := util.GetNowUnixMillion()
	maintaining, _ := model.ChannelMaintaining.Load(setting.Market)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` || account == nil ||
		(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 120000) ||
		setting.Coin == `` {
		return
	}
	cache, data := GetDataQueue(account, setting.Coin)
	if cache || data == nil {
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
		util.Notice(`cancel pending turtle order %s %s %s`, setting.Market, setting.Symbol, order.OrderId)
		api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, true)
		time.Sleep(time.Second)
	}
	if canceled {
		return
	}
	if !canQueue(setting, data, tick) {
		api.ClearOrders(account.Key, account.Secret, setting.Market, setting.Symbol, nil)
		util.Notice(fmt.Sprintf(`clear all order when can not queue %s %s`, setting.Market, setting.Symbol))
		return
	}
	amtBuy, amtSell := getQueueAmt(setting, data)
	if amtBuy > 0 {
		orders := api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeLimit, setting.Market,
			setting.Symbol, ``, setting.Function, tick.Bids[0].Price, tick.Bids[0].Price, amtBuy, setting)
		for _, order := range orders {
			if order == nil {
				continue
			}
			data.QueueOrders[order.OrderId] = order
			util.Notice(fmt.Sprintf(`place queue order %s %s %s %s at %f amt %f return %s`,
				setting.Function, setting.Market, setting.Symbol, model.OrderSideBuy, order.Price, amtBuy, order.OrderId))
		}
	}
	if amtSell > 0 {
		orders := api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeLimit, setting.Market,
			setting.Symbol, ``, setting.Function, tick.Asks[0].Price, tick.Asks[0].Price, amtSell, setting)
		for _, order := range orders {
			if order == nil {
				continue
			}
			data.QueueOrders[order.OrderId] = order
			util.Notice(fmt.Sprintf(`place queue order %s %s %s %s at %f amt %f return %s`,
				setting.Function, setting.Market, setting.Symbol, model.OrderSideSell, order.Price, amtSell, order.OrderId))
		}
	}
	// todo 2. add liquidate check termly
}

func EqualQueue(account *model.Account) {
	for DoQueue {
		for {
			if !api.CheckSetProcessing(model.FunctionQueue, model.FunctionQueue, model.FunctionQueue, true) {
				break
			} else {
				time.Sleep(time.Second)
			}
		}
		value := api.GetCoinSettings(model.FunctionQueue)
		if value != nil {
			value.Range(func(coin, settings interface{}) bool {
				QueueDataMap.Delete(coin)
				_, data := GetDataQueue(account, coin.(string))
				equalCoin(coin.(string), data.Statuses)
				return true
			})
		}
		api.CheckSetProcessing(model.FunctionQueue, model.FunctionQueue, model.FunctionQueue, false)
		time.Sleep(time.Minute * 5)
	}
}

func canQueue(setting *model.Setting, data *DataQueue, tick *model.BidAsk) (can bool) {
	v, ok := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
	if !ok || v == nil {
		return false
	}
	marketInfo := v.(*model.MarketInfo)
	if tick.Asks[0].Price-tick.Bids[0].Price > marketInfo.SizeIncrement {
		return false
	}
	if tick.Asks[0].Price*tick.Asks[0].Amount < setting.AmountLimit || tick.Bids[0].Price*tick.Bids[0].Amount < setting.AmountLimit {
		return false
	}
	for _, settingLiq := range data.Settings {
		if settingLiq.Function != model.FunctionQueueLiq {
			continue
		}
		_, tickLiq := model.AppMarkets.GetBidAsk(settingLiq.Symbol, settingLiq.Market)
		if tickLiq != nil && tickLiq.Bids != nil && len(tickLiq.Bids) > 0 && tickLiq.Asks != nil && len(tickLiq.Asks) > 0 &&
			(tickLiq.Bids[0].Price < tick.Asks[0].Price || tickLiq.Asks[0].Price > tick.Bids[0].Price) {
			return true
		}
	}
	return false
}

func getQueueAmt(setting *model.Setting, data *DataQueue) (amtBuy, amtSell float64) {
	isQueue := false
	for _, settingQueue := range data.Settings {
		if settingQueue.Market == setting.Market && settingQueue.Symbol == setting.Symbol && settingQueue.Function == model.FunctionQueue {
			isQueue = true
		}
	}
	if !isQueue {
		return 0, 0
	}
	var status *CarryStatus
	for _, value := range data.Statuses {
		if value.market == setting.Market && value.symbol == setting.Symbol {
			status = value
		}
	}
	if status == nil {
		return 0, 0
	}
	amtBuy = status.LimitBuy
	amtSell = status.LimitSell
	if amtBuy < setting.OpenShortMargin {
		amtBuy = 0
	}
	if amtSell < setting.OpenShortMargin {
		amtSell = 0
	}
	return amtBuy, amtSell
}

func GetDataQueue(account *model.Account, coin string) (cache bool, data *DataQueue) {
	value, ok := QueueDataMap.Load(coin)
	if ok && value != nil {
		return true, value.(*DataQueue)
	}
	data = &DataQueue{Coin: coin, Statuses: make([]*CarryStatus, 0), QueueOrders: make(map[string]*model.Order), Settings: make([]*model.Setting, 0)}
	QueueDataMap.Store(coin, data)
	settings := api.GetCoinSettings(model.FunctionQueue)
	if settings != nil {
		value, _ = settings.Load(coin)
		if value != nil {
			for _, setting := range (value).([]*model.Setting) {
				status := initStatus(account, setting, true)
				data.Statuses = append(data.Statuses, status)
				data.Settings = append(data.Settings, setting)
				orders := api.QueryOpenOrders(account.Key, account.Secret, setting.Market, setting.Symbol)
				for _, order := range orders {
					if order != nil {
						data.QueueOrders[order.OrderId] = order
					}
				}
			}
		}
	}
	settings = api.GetCoinSettings(model.FunctionQueueLiq)
	if settings != nil {
		value, _ = settings.Load(coin)
		if value != nil {
			for _, setting := range (value).([]*model.Setting) {
				status := initStatus(account, setting, true)
				data.Statuses = append(data.Statuses, status)
				data.Settings = append(data.Settings, setting)
			}
		}
	}
	return false, data
}

var ProcessQueueLiq = func(setting *model.Setting) {}

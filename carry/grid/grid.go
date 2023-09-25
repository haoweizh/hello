package grid

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"sync"
	"time"
)

type DataGrid struct {
	Market, Symbol        string
	Holding               float64
	OrderLong, orderShort []*model.Order
}

var dataRefreshTime *sync.Map // market*symbol *time.Time int64
var dataGrids *sync.Map       // market*symbol *DataGrid

var ProcessGrid = func(setting *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetProcessing(model.FunctionGrid, model.FunctionGrid, model.FunctionGrid, true) {
		defer api.CheckSetProcessing(model.FunctionGrid, model.FunctionGrid, model.FunctionGrid, false)
	} else {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	now := util.GetNowUnixMillion()
	maintaining, _ := model.ChannelMaintaining.Load(setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.Market)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` || account == nil ||
		(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 10000) ||
		setting.Coin == `` || tickRelated == nil || tickRelated.Asks == nil || tickRelated.Bids == nil ||
		(model.AppConfig.Env != `test` && now-int64(tickRelated.Ts) > 10000) {
		return
	}
	cache, data := GetDataGrid(setting.Market, setting.Symbol, false)
	if !cache || data == nil {
		return
	}
	refreshTime, ok := util.LoadSyncMap(dataRefreshTime, setting.Market, setting.Symbol)
	if refreshTime != nil && ok && now-refreshTime.(int64) > 60000 {
		GetDataGrid(setting.Market, setting.Symbol, true)
		return
	}
	if math.Abs(data.Holding)*tick.Bids[0].Price <= 20 {
		if !canOpen(setting, tick) {
			if data.OrderLong != nil {
				for _, order := range data.OrderLong {
					api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, model.OrderTypeLimit, order.OrderId, true)
					util.Notice(fmt.Sprintf(`can not open %s %s cancel %s %s`,
						setting.Market, setting.Symbol, order.OrderSide, order.OrderId))
				}
				data.OrderLong = nil
			}
			if data.orderShort != nil {
				for _, order := range data.orderShort {
					api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, model.OrderTypeLimit, order.OrderId, true)
					util.Notice(fmt.Sprintf(`can not open %s %s cancel %s %s`,
						setting.Market, setting.Symbol, order.OrderSide, order.OrderId))
				}
				data.orderShort = nil
			}
		} else if math.Abs(data.Holding)*tick.Bids[0].Price > 20 {
			placeGrid(setting, data, tick)
		}
	}
}

func liqGrid(account *model.Account, setting *model.Setting, data *DataGrid, tick, tickRelated *model.BidAsk) {
	if !canOpen(setting, tick) {
		var orders []*model.Order
		if data.Holding > 0 {
			price := tick.Bids[0].Price * 0.99
			orders = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeLimit, data.Market,
				data.Symbol, ``, model.FunctionComplement, price, price, data.Holding, setting)
			util.Notice(fmt.Sprintf(`fast liq when can not open %s %s %s holding %f at %f amt %f`,
				data.Market, data.Symbol, model.OrderSideSell, data.Holding, price, data.Holding))
		} else if data.Holding < 0 {
			price := tick.Asks[0].Price * 1.01
			orders = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeLimit, data.Market,
				data.Symbol, ``, model.FunctionComplement, price, price, math.Abs(data.Holding), setting)
			util.Notice(fmt.Sprintf(`fast liq when can not open %s %s %s holding %f at %f amt %f`,
				data.Market, data.Symbol, model.OrderSideBuy, data.Holding, price, data.Holding))
		}
		for _, order := range orders {
			model.AppDB.Save(order)
			time.Sleep(time.Second)
			api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, model.OrderTypeLimit, order.OrderId, true)
		}
		GetDataGrid(data.Market, data.Symbol, true)
		return
	}
	var ordersLiq []*model.Order
	if data.Holding > 0 {
		price := math.Min(tick.Asks[0].Price+setting.OpenShortMargin, tickRelated.Asks[0].Price)
		data.orderShort = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, setting.Market, setting.Symbol,
			model.OrderTypeLimit, ``, model.FunctionGrid, price, price, math.Abs(data.Holding), setting)
		ordersLiq = data.orderShort
	} else if data.Holding < 0 {
		price := math.Max(tick.Bids[0].Price-setting.OpenShortMargin, tickRelated.Bids[0].Price)
		data.OrderLong = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, setting.Market, setting.Symbol,
			model.OrderTypeLimit, ``, model.FunctionGrid, price, price, math.Abs(data.Holding), setting)
		ordersLiq = data.OrderLong
	}
	for _, order := range ordersLiq {
		model.AppDB.Save(order)
	}
	return
}

var ProcessOrderSuccess = func(setting *model.Setting) {
	for {
		for {
			if !api.CheckSetProcessing(model.FunctionGrid, model.FunctionGrid, model.FunctionGrid, true) {
				break
			} else {
				time.Sleep(time.Second)
			}
		}
		GetDataGrid(setting.Market, setting.Symbol, true)
		api.CheckSetProcessing(model.FunctionGrid, model.FunctionGrid, model.FunctionGrid, false)
		time.Sleep(time.Minute * 5)
	}
}

func placeGrid(setting *model.Setting, data *DataGrid, tick *model.BidAsk) {
	if data == nil || tick == nil {
		return
	}
	// tick.bid0.price - tickRelated.bid0.price > tickRelated.ask0.price - tick.ask0.price
	// limit: buy < 0.5 sell>0.5
	if data.OrderLong == nil {

	}
	if data.orderShort == nil {

	}
}

func GetDataGrid(market, symbol string, refresh bool) (cache bool, data *DataGrid) {
	value, ok := util.LoadSyncMap(dataGrids, market, symbol)
	if ok && value != nil && !refresh {
		return true, value.(*DataGrid)
	}
	// query long short orders, set nil when success/fail, if all working ,return false, nil
	if value != nil {
		data = value.(*DataGrid)
	} else {
		// cancel all open orders
		data = &DataGrid{Market: market, Symbol: symbol, Holding: 0}
	}
	// set holding
	util.StoreSyncMap(dataGrids, dataGrids, market, symbol)
	util.StoreSyncMap(dataRefreshTime, time.Now().UnixMilli(), market, symbol)
	return false, data
}

// canOpen
// 1. tick price between setting.SymbolRelated tick price
// 2. related amtBid/amtAsk [0.2,5] and amtBid > setting.AmountLimit
// 3. related priceBid+1 = priceAsk
func canOpen(setting *model.Setting, tick *model.BidAsk) (can bool) {
	return true
}

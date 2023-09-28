package grid

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

type DataGrid struct {
	Market, Symbol        string
	Holding               float64
	OrderLong, orderShort []*model.Order
	RefreshTime           int64 // million-seconds
}

var dataGrids = &sync.Map{} // market*symbol *DataGrid

// ProcessGrid
// settingRelate.AmountLimit 以计价货币为单位的最小下单数量检查
// setting.OpenShortMargin 平仓绝对值价差
// setting.GridAmount 下单绝对数量
// setting.RateRelated related symbol对应的price乘数,price*settingRelated和amount/settingRelated与tick进行比较
var ProcessGrid = func(setting *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetProcessing(model.FunctionGrid, model.FunctionGrid, model.FunctionGrid, true) {
		defer api.CheckSetProcessing(model.FunctionGrid, model.FunctionGrid, model.FunctionGrid, false)
	} else {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	now := util.GetNowUnixMillion()
	maintaining, _ := model.ChannelMaintaining.Load(setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` || account == nil ||
		(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 1000) ||
		tickRelated == nil || tickRelated.Asks == nil || tickRelated.Bids == nil ||
		(model.AppConfig.Env != `test` && now-int64(tickRelated.Ts) > 1000) {
		return
	}
	cache, data := GetDataGrid(account, setting, tickRelated, false)
	if !cache || data == nil {
		return
	}
	if now-data.RefreshTime > 60000 {
		util.Notice(fmt.Sprintf(`refresh time %d %d = %d`, now, data.RefreshTime, now-data.RefreshTime))
		GetDataGrid(account, setting, tickRelated, true)
		return
	}
	canOpen := canOpen(setting, tick, tickRelated)
	if math.Abs(data.Holding)*tick.Bids[0].Price <= 20 {
		if !canOpen {
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
		} else {
			placeGrid(account, setting, data, tick, tickRelated)
		}
	} else {
		liqGrid(account, setting, data, tick, tickRelated, canOpen)
	}
}

func liqGrid(account *model.Account, setting *model.Setting, data *DataGrid, tick, tickRelated *model.BidAsk, canOpen bool) {
	var refreshType, side string
	var price float64
	placeOrder := false
	if canOpen && data.Holding > 0 {
		refreshType = model.FunctionGrid
		side = model.OrderSideSell
		price = math.Min(tick.Asks[0].Price+setting.OpenShortMargin, tickRelated.Asks[0].Price*setting.RateRelated)
		if data.orderShort == nil {
			placeOrder = true
		}
	} else if canOpen && data.Holding < 0 {
		refreshType = model.FunctionGrid
		side = model.OrderSideBuy
		price = math.Max(tick.Bids[0].Price-setting.OpenShortMargin, tickRelated.Bids[0].Price*setting.RateRelated)
		if data.OrderLong == nil {
			placeOrder = true
		}
	} else if !canOpen && data.Holding > 0 {
		refreshType = model.FunctionComplement
		side = model.OrderSideSell
		price = tick.Bids[0].Price * 0.99
		placeOrder = true
	} else if !canOpen && data.Holding < 0 {
		refreshType = model.FunctionComplement
		side = model.OrderSideBuy
		price = tick.Asks[0].Price * 1.01
		placeOrder = true
	}
	if placeOrder {
		orders := api.MustPlaceOrder(account.Key, account.Secret, side, model.OrderTypeLimit, data.Market,
			data.Symbol, ``, refreshType, price, price, data.Holding, setting)
		util.Notice(fmt.Sprintf(`grid liq when canOpen:%v %s %s %s holding %f at %f amt %f`,
			canOpen, setting.Market, setting.Symbol, side, data.Holding, price, data.Holding))
		for _, order := range orders {
			model.AppDB.Save(order)
			if !canOpen {
				time.Sleep(time.Second)
				api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, model.OrderTypeLimit, order.OrderId, true)
			}
		}
		if canOpen {
			if side == model.OrderSideSell {
				data.orderShort = orders
			} else if side == model.OrderSideBuy {
				data.OrderLong = orders
			}
		}
	}
	if !canOpen {
		GetDataGrid(account, setting, tickRelated, true)
	}
}

func GetDataGrid(account *model.Account, setting *model.Setting, tickRelate *model.BidAsk, refresh bool) (cache bool, data *DataGrid) {
	value, ok := util.LoadSyncMap(dataGrids, setting.Market, setting.Symbol)
	if ok && value != nil && !refresh {
		return true, value.(*DataGrid)
	}
	data = &DataGrid{Market: setting.Market, Symbol: setting.Symbol, Holding: 0, RefreshTime: time.Now().UnixMilli()}
	orders := api.QueryOpenOrders(account.Key, account.Secret, setting.Market, setting.Symbol)
	for _, order := range orders {
		if order.Price > tickRelate.Asks[0].Price*setting.RateRelated || order.Price < tickRelate.Bids[0].Price*setting.RateRelated {
			util.Notice(fmt.Sprintf(`Cancel out price range order %s %s %s %s %f [%f %f]`,
				order.Market, order.Symbol, order.OrderSide, order.OrderId, order.Price, tickRelate.Bids[0].Price*setting.RateRelated, tickRelate.Asks[0].Price*setting.RateRelated))
			api.MustCancel(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId, true)
		} else {
			if order.OrderSide == model.OrderSideSell {
				if data.orderShort == nil {
					data.orderShort = []*model.Order{order}
				} else {
					data.orderShort = append(data.orderShort, order)
				}
			} else if order.OrderSide == model.OrderSideBuy {
				if data.OrderLong == nil {
					data.OrderLong = []*model.Order{order}
				} else {
					data.OrderLong = append(data.OrderLong, order)
				}
			}
			util.Notice(fmt.Sprintf(`add back order %s %s %s %s %f %f`,
				order.Market, order.Symbol, order.OrderSide, order.OrderId, order.Price, order.Amount))
		}
	}
	_, marketType, coin, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	if marketType == model.MarketTypePerp {
		_, positions, _, _ := api.GetPositions(account.Key, account.Secret, setting.Market)
		for _, position := range positions {
			if strings.EqualFold(coin, position.Currency) {
				data.Holding = position.Holding
				break
			}
		}
	} else if marketType == model.MarketTypeSpot {
		_, balances, _, _ := api.GetBalances(account.Key, account.Secret, setting.Market)
		for _, balance := range balances {
			if strings.EqualFold(balance.Coin, coin) {
				data.Holding = balance.Amount
			}
		}
	}
	util.StoreSyncMap(dataGrids, data, setting.Market, setting.Symbol)
	util.Notice(fmt.Sprintf(`set data %s %s %f refresh %v`, data.Market, data.Symbol, data.Holding, refresh))
	time.Sleep(time.Second * 2)
	return false, data
}

func canOpen(setting *model.Setting, tick, tickRelate *model.BidAsk) (can bool) {
	vRelate, _ := util.LoadSyncMap(model.MarketInfos, setting.MarketRelated, setting.SymbolRelated)
	if vRelate == nil {
		return false
	}
	marketInfoRelate := vRelate.(*model.MarketInfo)
	priceDis := tickRelate.Asks[0].Price - tickRelate.Bids[0].Price - marketInfoRelate.PriceIncrement*1.1
	if priceDis > 0 {
		return false
	}
	if tickRelate.Asks[0].Price*tickRelate.Asks[0].Amount < setting.AmountLimit ||
		tickRelate.Bids[0].Price*tickRelate.Bids[0].Amount < setting.AmountLimit {
		return false
	}
	if tickRelate.Asks[0].Amount > 5*tickRelate.Bids[0].Amount || tickRelate.Bids[0].Amount > tickRelate.Asks[0].Amount*5 {
		return false
	}
	if tick.Bids[0].Price <= tickRelate.Bids[0].Price*setting.RateRelated || tick.Asks[0].Price >= tickRelate.Asks[0].Price*setting.RateRelated {
		return false
	}
	return true
}

func placeGrid(account *model.Account, setting *model.Setting, data *DataGrid, tick, tickRelate *model.BidAsk) (success bool) {
	v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
	if v == nil {
		return false
	}
	inLong := tick.Bids[0].Price - tickRelate.Bids[0].Price*setting.RateRelated
	inShort := tickRelate.Asks[0].Price*setting.RateRelated - tick.Asks[0].Price
	if inLong <= 0 || inShort <= 0 {
		return false
	}
	var priceLong, priceShort float64
	priceDis := tickRelate.Asks[0].Price*setting.RateRelated - tickRelate.Bids[0].Price*setting.RateRelated
	if inLong > inShort {
		priceShort = math.Min(tickRelate.Asks[0].Price*setting.RateRelated, tick.Asks[0].Price+0.1*priceDis)
		priceLong = priceShort - setting.OpenShortMargin
	} else {
		priceLong = math.Max(tickRelate.Bids[0].Price*setting.RateRelated, tick.Bids[0].Price-0.1*priceDis)
		priceShort = priceLong + setting.OpenShortMargin
	}
	if data.OrderLong == nil {
		data.OrderLong = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeLimit,
			setting.Market, setting.Symbol, ``, model.FunctionGrid, priceLong, priceLong, setting.GridAmount, setting)
		util.Notice(fmt.Sprintf(`place grid %s %s at %f amt %f order %v`,
			setting.Market, setting.Symbol, priceLong, setting.GridAmount, data.OrderLong))
	}
	if data.orderShort == nil {
		data.orderShort = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeLimit,
			setting.Market, setting.Symbol, ``, model.FunctionGrid, priceShort, priceShort, setting.GridAmount, setting)
		util.Notice(fmt.Sprintf(`place grid %s %s at %f amt %f order %v`,
			setting.Market, setting.Symbol, priceLong, setting.GridAmount, data.orderShort))
	}
	return true
}

var ProcessGridOrder = func(order *model.Order) {
	for {
		if !api.CheckSetProcessing(model.FunctionGrid, model.FunctionGrid, model.FunctionGrid, true) {
			break
		} else {
			time.Sleep(time.Second)
		}
	}
	defer api.CheckSetProcessing(model.FunctionGrid, model.FunctionGrid, model.FunctionGrid, false)
	if order == nil || order.Status != model.CarryStatusSuccess {
		return
	}
	accounts := model.AppConfig.GetAccounts(order.Market)
	setting := api.GetSetting(model.FunctionGrid, order.Market, order.Symbol)
	if setting == nil {
		return
	}
	for _, account := range accounts {
		_, tickRelated := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
		if account != nil && tickRelated != nil && tickRelated.Bids != nil && len(tickRelated.Bids) > 0 &&
			tickRelated.Asks != nil && len(tickRelated.Asks) > 0 {
			setting := api.GetSetting(model.FunctionGrid, order.Market, order.Symbol)
			if setting != nil {
				util.Notice(fmt.Sprintf(`get order success, refresh data grid %v`, order))
				GetDataGrid(account, setting, tickRelated, true)
			}
		}
	}
}

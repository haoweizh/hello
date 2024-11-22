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

var dataGrids = &sync.Map{} // account.key*market*symbol *DataGrid

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
	_, tickRelated := model.AppEnvironment.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
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
	openCode := canOpen(setting, tick, tickRelated)
	if math.Abs(data.Holding)*tick.Bids[0].Price <= 20 {
		if openCode < 0 {
			if data.OrderLong != nil {
				for _, order := range data.OrderLong {
					api.CancelOrder(account.Key, account.Secret, setting.Market, setting.Symbol, model.OrderTypeLimit, order.OrderId)
					util.Notice(fmt.Sprintf(`can not open code %d %s %s cancel %s %s`,
						openCode, setting.Market, setting.Symbol, order.OrderSide, order.OrderId))
				}
				data.OrderLong = nil
				time.Sleep(time.Second * 3)
			}
			if data.orderShort != nil {
				for _, order := range data.orderShort {
					api.CancelOrder(account.Key, account.Secret, setting.Market, setting.Symbol, model.OrderTypeLimit, order.OrderId)
					util.Notice(fmt.Sprintf(`can not open code %d %s %s cancel %s %s`,
						openCode, setting.Market, setting.Symbol, order.OrderSide, order.OrderId))
				}
				data.orderShort = nil
				time.Sleep(time.Second * 3)
			}
		} else {
			placeGrid(account, setting, data, tick, tickRelated)
		}
	} else {
		liqGrid(account, setting, data, tick)
	}
}

func liqGrid(account *model.Account, setting *model.Setting, data *DataGrid, tick *model.BidAsk) {
	vRelate, _ := util.LoadSyncMap(model.MarketInfos, setting.MarketRelated, setting.SymbolRelated)
	if vRelate == nil {
		return
	}
	marketInfoRelated := vRelate.(*model.MarketInfo)
	var side string
	var price float64
	placeOrder := false
	var cancelOrders []*model.Order
	if data.Holding > 0 {
		side = model.OrderSideSell
		price = tick.Asks[0].Price
		if data.orderShort == nil {
			placeOrder = true
		} else {
			cancelOrders = data.orderShort
		}
		for _, order := range data.OrderLong {
			if order != nil {
				api.CancelOrder(account.Key, account.Secret, order.Market, order.Symbol, order.OrderType, order.OrderId)
				util.Notice(fmt.Sprintf(`can order %s when holding %f %s %s orderId %s`,
					order.OrderSide, data.Holding, order.Market, order.Symbol, order.OrderId))
			}
		}
	} else if data.Holding < 0 {
		side = model.OrderSideBuy
		price = tick.Bids[0].Price
		if data.OrderLong == nil {
			placeOrder = true
		} else {
			cancelOrders = data.OrderLong
		}
		for _, order := range data.orderShort {
			if order != nil {
				api.CancelOrder(account.Key, account.Secret, order.Market, order.Symbol, order.OrderType, order.OrderId)
				util.Notice(fmt.Sprintf(`can order %s when holding %f %s %s orderId %s`,
					order.OrderSide, data.Holding, order.Market, order.Symbol, order.OrderId))
			}
		}
	}
	if placeOrder {
		util.Notice(fmt.Sprintf(`grid liq when openCode:%s %s %s holding %f at %f amt %f`,
			setting.Market, setting.Symbol, side, data.Holding, price, data.Holding))
		orders := api.MustPlaceOrder(account.Key, account.Secret, side, model.OrderTypeLimit, data.Market,
			data.Symbol, ``, model.FunctionComplement, price, price, math.Abs(data.Holding), setting, true)
		for _, order := range orders {
			model.AppDB.Save(order)
		}
		if side == model.OrderSideSell {
			data.orderShort = orders
		} else if side == model.OrderSideBuy {
			data.OrderLong = orders
		}
	} else {
		needCancel := false
		for _, order := range cancelOrders {
			if order == nil {
				continue
			}
			priceFloor := math.Floor(order.Price/(setting.RateRelated*marketInfoRelated.PriceIncrement)) * setting.RateRelated * marketInfoRelated.PriceIncrement
			priceCeil := math.Ceil(order.Price/(setting.RateRelated*marketInfoRelated.PriceIncrement)) * setting.RateRelated * marketInfoRelated.PriceIncrement
			if tick.Asks[0].Price >= priceCeil || tick.Bids[0].Price <= priceFloor {
				needCancel = true
			}
		}
		if needCancel {
			for _, order := range cancelOrders {
				util.Notice(fmt.Sprintf(`liq cancel out range order %s %s %s %s %f tick [%f %f]`,
					order.Market, order.Symbol, order.OrderSide, order.OrderId, order.Price, tick.Bids[0].Price, tick.Asks[0].Price))
				api.CancelOrder(account.Key, account.Secret, data.Market, data.Symbol, model.OrderTypeLimit, order.OrderId)
			}
		}
	}
}

func GetDataGrid(account *model.Account, setting *model.Setting, tickRelate *model.BidAsk, refresh bool) (cache bool, data *DataGrid) {
	value, ok := util.LoadSyncMap(dataGrids, account.Key, setting.Market, setting.Symbol)
	if ok && value != nil && !refresh {
		return true, value.(*DataGrid)
	}
	data = &DataGrid{Market: setting.Market, Symbol: setting.Symbol, Holding: 0, RefreshTime: time.Now().UnixMilli()}
	orders := api.QueryOpenOrders(account.Key, account.Secret, setting.Market, setting.Symbol)
	for _, order := range orders {
		if order.Price > tickRelate.Asks[0].Price*setting.RateRelated || order.Price < tickRelate.Bids[0].Price*setting.RateRelated {
			util.Notice(fmt.Sprintf(`Cancel out price range order %s %s %s %s %f [%f %f]`,
				order.Market, order.Symbol, order.OrderSide, order.OrderId, order.Price, tickRelate.Bids[0].Price*setting.RateRelated, tickRelate.Asks[0].Price*setting.RateRelated))
			api.CancelOrder(account.Key, account.Secret, setting.Market, setting.Symbol, order.OrderType, order.OrderId)
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
			if strings.EqualFold(setting.Symbol, position.Currency) {
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
	util.StoreSyncMap(dataGrids, data, account.Key, setting.Market, setting.Symbol)
	util.Notice(fmt.Sprintf(`set data %s %s %s %f refresh %v`,
		account.Key, data.Market, data.Symbol, data.Holding, refresh))
	time.Sleep(time.Second * 2)
	return false, data
}

func canOpen(setting *model.Setting, tick, tickRelate *model.BidAsk) (can int) {
	vRelate, _ := util.LoadSyncMap(model.MarketInfos, setting.MarketRelated, setting.SymbolRelated)
	if vRelate == nil {
		return -1
	}
	marketInfoRelate := vRelate.(*model.MarketInfo)
	priceDis := tickRelate.Asks[0].Price - tickRelate.Bids[0].Price - marketInfoRelate.PriceIncrement*1.1
	if priceDis > 0 {
		return -2
	}
	if tickRelate.Asks[0].Price*tickRelate.Asks[0].Amount < setting.AmountLimit ||
		tickRelate.Bids[0].Price*tickRelate.Bids[0].Amount < setting.AmountLimit {
		return -3
	}
	if tickRelate.Asks[0].Amount > 5*tickRelate.Bids[0].Amount || tickRelate.Bids[0].Amount > tickRelate.Asks[0].Amount*5 {
		return -4
	}
	if tick.Bids[0].Price <= tickRelate.Bids[0].Price*setting.RateRelated || tick.Asks[0].Price >= tickRelate.Asks[0].Price*setting.RateRelated {
		return -5
	}
	return 1
}

func placeGrid(account *model.Account, setting *model.Setting, data *DataGrid, tick, tickRelate *model.BidAsk) (success bool) {
	//inLong := tick.Bids[0].Price - tickRelate.Bids[0].Price*setting.RateRelated
	//inShort := tickRelate.Asks[0].Price*setting.RateRelated - tick.Asks[0].Price
	//if inLong <= 0 || inShort <= 0 {
	//	return false
	//}
	//var priceLong, priceShort float64
	//priceDis := tickRelate.Asks[0].Price*setting.RateRelated - tickRelate.Bids[0].Price*setting.RateRelated
	//if inLong > inShort {
	//	priceShort = math.Min(tickRelate.Asks[0].Price*setting.RateRelated, tick.Asks[0].Price+0.5*setting.OpenShortMargin)
	//	priceLong = priceShort - setting.OpenShortMargin
	//} else {
	//	priceLong = math.Max(tickRelate.Bids[0].Price*setting.RateRelated, tick.Bids[0].Price-0.5*setting.OpenShortMargin)
	//	priceShort = priceLong + setting.OpenShortMargin
	//}
	priceLong := math.Max(tickRelate.Bids[0].Price*setting.RateRelated/2+tick.Bids[0].Price/2, tick.Bids[0].Price-setting.OpenShortMargin)
	priceShort := math.Min(tickRelate.Asks[0].Price*setting.RateRelated/2+tick.Asks[0].Price/2, tick.Asks[0].Price+setting.OpenShortMargin)
	if data.OrderLong == nil {
		data.OrderLong = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeLimit,
			setting.Market, setting.Symbol, ``, model.FunctionGrid, priceLong, priceLong, setting.GridAmount, setting, true)
		util.Notice(fmt.Sprintf(`place grid buy %s %s at %f amt %f order %v`,
			setting.Market, setting.Symbol, priceLong, setting.GridAmount, data.OrderLong))
		for _, order := range data.OrderLong {
			model.AppDB.Save(order)
		}
	}
	if data.orderShort == nil {
		data.orderShort = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeLimit,
			setting.Market, setting.Symbol, ``, model.FunctionGrid, priceShort, priceShort, setting.GridAmount, setting, true)
		util.Notice(fmt.Sprintf(`place grid sell %s %s at %f amt %f order %v`,
			setting.Market, setting.Symbol, priceShort, setting.GridAmount, data.orderShort))
		for _, order := range data.orderShort {
			model.AppDB.Save(order)
		}
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
	util.Notice(fmt.Sprintf(`deal grid order %s %s %s %s deal at %f deal amt %f`,
		order.Market, order.Symbol, order.OrderId, order.Status, order.DealPrice, order.DealAmount))
	if order == nil || order.DealAmount == 0 || order.DealPrice == 0 {
		return
	}
	model.AppDB.Model(order).Where(`order_id=?`, order.OrderId).Updates(map[string]interface{}{
		`deal_price`: order.DealPrice, `deal_amount`: order.DealAmount, `fee`: order.Fee, `status`: order.Status})
	accounts := model.AppConfig.GetAccounts(order.Market)
	setting := api.GetSetting(model.FunctionGrid, order.Market, order.Symbol)
	if setting == nil {
		return
	}
	for _, account := range accounts {
		_, tickRelated := model.AppEnvironment.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
		if account != nil && tickRelated != nil && tickRelated.Bids != nil && len(tickRelated.Bids) > 0 &&
			tickRelated.Asks != nil && len(tickRelated.Asks) > 0 {
			if order.Status == model.CarryStatusSuccess {
				util.Notice(fmt.Sprintf(`get order success, refresh data grid %v`, order))
				GetDataGrid(account, setting, tickRelated, true)
			}
		}
	}
}

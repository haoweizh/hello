package follow

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"time"
)

var normalPriceBid, normalPriceAsk, holding float64
var normalTimeBid, normalTimeAsk int64
var timeFollow = time.Now().UnixMilli()

// ProcessFollow
// setting.AmountLimit判定触发买卖1的计价币种为异常数量的挂单数量要求
// tick.Price*setting.RateRelated换算成tickOrder.price,即1000
// setting.OpenShortMargin 以related tick计的价差要求
// setting.GridAmount 以related tick计的数量
var ProcessFollow = func(setting *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetProcessing(model.FunctionFollow, model.FunctionFollow, model.FunctionFollow, true) {
		defer api.CheckSetProcessing(model.FunctionFollow, model.FunctionFollow, model.FunctionFollow, false)
	} else {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	now := util.GetNowUnixMillion()
	maintaining, _ := model.ChannelMaintaining.Load(setting.Market)
	_, tickOrder := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
	if tick == nil || tick.Asks == nil || len(tick.Asks) == 0 || tick.Bids == nil || len(tick.Bids) == 0 || model.AppConfig.Handle != `1` ||
		tickOrder == nil || tickOrder.Asks == nil || tickOrder.Bids == nil || len(tickOrder.Bids) == 0 || len(tickOrder.Asks) == 0 || account == nil ||
		(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && (now-int64(tick.Ts) > 1000) || (now-int64(tickOrder.Ts) > 1000)) {
		return
	}
	quantityBid := tick.Bids[0].Price * tick.Bids[0].Amount
	quantityAsk := tick.Asks[0].Price * tick.Asks[0].Amount
	if quantityBid > 3*setting.AmountLimit || quantityBid > 2*quantityAsk {
		normalPriceBid = tick.Bids[0].Price
		normalTimeBid = now
	}
	if quantityAsk > 3*setting.AmountLimit || quantityAsk > 2*quantityBid {
		normalPriceAsk = tick.Asks[0].Price
		normalTimeAsk = now
	}
	if timeFollow == 0 {
		if math.Abs(holding)*tickOrder.Bids[0].Price > 10 {
			var orderLiq *model.Order
			if holding > 0 && tickOrder.Bids[0].Price-tick.Bids[0].Price*setting.RateRelated > setting.OpenShortMargin {
				orderLiq = api.PlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeMarket, setting.MarketRelated, setting.SymbolRelated, ``,
					tickOrder.Bids[0].Price, tickOrder.Bids[0].Price, holding, false, nil, setting)
			} else if holding < 0 && tick.Asks[0].Price*setting.RateRelated-tickOrder.Asks[0].Price > setting.OpenShortMargin {
				orderLiq = api.PlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeMarket, setting.MarketRelated, setting.SymbolRelated, ``,
					tickOrder.Asks[0].Price, tickOrder.Asks[0].Price, math.Abs(holding), false, nil, setting)
			}
			if orderLiq != nil && orderLiq.HaveId() {
				timeFollow = now
				orderLiq.RefreshType = model.FunctionComplement
				model.AppDB.Save(&orderLiq)
				util.Notice(fmt.Sprintf(`liq order %s %s %s %s holding %e`,
					orderLiq.Market, orderLiq.Symbol, orderLiq.OrderSide, orderLiq.OrderId, holding))
			}
		} else {
			var order *model.Order
			if quantityBid < setting.AmountLimit && quantityBid*4 < quantityAsk && tick.Bids[0].Price == normalPriceBid && now-normalTimeBid < 10000 {
				order = api.PlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeLimit, setting.MarketRelated, setting.SymbolRelated, ``,
					tick.Bids[0].Price*setting.RateRelated, tick.Bids[0].Price*setting.RateRelated, setting.GridAmount, false, nil, setting)
			} else if quantityAsk < setting.AmountLimit && quantityAsk*4 < quantityBid && tick.Asks[0].Price == normalPriceAsk && now-normalTimeAsk < 10000 {
				order = api.PlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeLimit, setting.MarketRelated, setting.SymbolRelated, ``,
					tick.Asks[0].Price*setting.RateRelated, tick.Asks[0].Price*setting.RateRelated, setting.GridAmount, false, nil, setting)
			}
			if order != nil && order.HaveId() {
				order.RefreshType = model.FunctionQueue
				model.AppDB.Save(&order)
				timeFollow = now
				util.Notice(fmt.Sprintf(`chance to follow %s at %e amt %e return %s tick[%e %e %e %e] quantity[%e %e] normal[%e %d %e %d]`,
					order.OrderSide, order.Price, order.Amount, order.OrderId, tick.Bids[0].Price, tick.Bids[0].Amount, tick.Asks[0].Price,
					tick.Asks[0].Amount, quantityBid, quantityAsk, normalPriceBid, normalTimeBid, normalPriceAsk, normalTimeAsk))
			}
		}
	} else {
		time.Sleep(time.Second * 3)
		orders := api.QueryOpenOrders(account.Key, account.Secret, setting.MarketRelated, setting.SymbolRelated)
		if len(orders) == 0 {
			var success bool
			success, holding = api.GetHolding(account, setting.MarketRelated, setting.SymbolRelated)
			if success {
				timeFollow = 0
				util.Notice(fmt.Sprintf(`orders 0 set holding %e`, holding))
			}
		} else {
			for _, order := range orders {
				if order == nil || !order.HaveId() {
					continue
				}
				api.MustCancel(account.Key, account.Secret, setting.MarketRelated, setting.SymbolRelated, model.OrderTypeLimit, order.OrderId, true)
			}
		}
	}
}

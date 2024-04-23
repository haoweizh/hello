package follow

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"time"
)

var ordPriceBid, ordPriceAsk float64
var holding float64
var followOrderTime = time.Now().UnixMilli()

//var followStatus = StatusChaos
//
//const StatusNormal = `normal`
//const StatusDown = `down`
//const StatusUp = `up`
//const StatusChaos = `chaos`

// ProcessFollow
// setting.Far setting.Low 判定触发买卖1的计价币种为异常数量的挂单数量要求
// tick.Price*setting.RateRelated换算成tickOrder.price,即1000
// setting.OpenShortMargin settingCloseShortMargin 以related tick计的价差要求
// setting.GridAmount 以related tick计的数量
// setting.PriceX 判定down up时买卖盘数量的比例设定,即抛压、买压阈值
var ProcessFollow = func(setting *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetProcessing(model.FunctionFollow, model.FunctionFollow, model.FunctionFollow, true) {
		defer api.CheckSetProcessing(model.FunctionFollow, model.FunctionFollow, model.FunctionFollow, false)
	} else {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	now := util.GetNowUnixMillion()
	maintaining, _ := model.ChannelMaintaining.Load(setting.Market)
	_, tickOrder := model.AppEnvironment.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
	if tick == nil || tick.Asks == nil || len(tick.Asks) == 0 || tick.Bids == nil || len(tick.Bids) == 0 || model.AppConfig.Handle != `1` ||
		tickOrder == nil || tickOrder.Asks == nil || tickOrder.Bids == nil || len(tickOrder.Bids) == 0 || len(tickOrder.Asks) == 0 || account == nil ||
		(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && (now-int64(tick.Ts) > 1000) || (now-int64(tickOrder.Ts) > 1000)) {
		return
	}
	//updateStatus(setting, tick, tickOrder)
	if followOrderTime == 0 {
		if math.Abs(holding)*tickOrder.Bids[0].Price > 10 {
			if time.Now().Second() == 0 {
				util.Notice(fmt.Sprintf(`holding %s %s %e`, setting.MarketRelated, setting.SymbolRelated, holding))
			}
			placeLiq(account, setting, tick, tickOrder)
		} else {
			placeFollow(account, setting, tick, tickOrder)
		}
	} else {
		time.Sleep(time.Second * 5)
		orders := api.QueryOpenOrders(account.Key, account.Secret, setting.MarketRelated, setting.SymbolRelated)
		if len(orders) == 0 {
			var success bool
			success, holding = api.GetHolding(account, setting.MarketRelated, setting.SymbolRelated)
			if success {
				followOrderTime = 0
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

//func updateStatus(setting *model.Setting, tick, tickOrder *model.BidAsk) {
//	quantityBid := tick.Bids[0].Price * tick.Bids[0].Amount
//	quantityAsk := tick.Asks[0].Price * tick.Asks[0].Amount
//	var update = ``
//	v, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
//	if v == nil {
//		return
//	}
//	marketInfo := v.(*model.MarketInfo)
//	priceDis := tick.Asks[0].Price - tick.Bids[0].Price - marketInfo.PriceIncrement*1.1
//	if tick.Bids[0].Price != normalPriceBid || tick.Asks[0].Price != normalPriceAsk {
//		normalPriceAsk = 0
//		normalPriceBid = 0
//		update = StatusChaos
//	}
//	if quantityBid > float64(setting.Far) && quantityAsk > float64(setting.Far) && priceDis < 0 {
//		normalPriceBid = tick.Bids[0].Price
//		normalPriceAsk = tick.Asks[0].Price
//		update = StatusNormal
//	} else if quantityBid < float64(setting.Near) && quantityBid/quantityAsk < setting.PriceX && tick.Bids[0].Price == normalPriceBid {
//		update = StatusDown
//	} else if quantityAsk < float64(setting.Near) && quantityAsk/quantityBid < setting.PriceX && tick.Asks[0].Price == normalPriceAsk {
//		update = StatusUp
//	} else {
//		update = StatusChaos
//	}
//	if update != `` && update != followStatus {
//		util.Notice(fmt.Sprintf(`update follow status %s->%s normal price[%e %e] tick [%e %e %e %e] tickOrder[%e %e %e %e]`,
//			followStatus, update, normalPriceBid, normalPriceAsk, tick.Bids[0].Price, tick.Bids[0].Amount, tick.Asks[0].Price,
//			tick.Asks[0].Amount, tickOrder.Bids[0].Price, tickOrder.Bids[0].Amount, tickOrder.Asks[0].Price, tickOrder.Asks[0].Amount))
//		followStatus = update
//	}
//	return
//}

func placeLiq(account *model.Account, setting *model.Setting, tick, tickOrder *model.BidAsk) {
	if tick.Bids[0].Price == ordPriceBid || tick.Asks[0].Price == ordPriceAsk {
		return
	}
	var orderLiq *model.Order
	now := time.Now().UnixMilli()
	profitSell := tickOrder.Bids[0].Price - tick.Bids[0].Price*setting.RateRelated
	profitBuy := tick.Asks[0].Price*setting.RateRelated - tickOrder.Asks[0].Price
	if holding > 0 && profitSell > setting.CloseShortMargin {
		orderLiq = api.PlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeMarket, setting.MarketRelated, setting.SymbolRelated, ``,
			tickOrder.Bids[0].Price, tickOrder.Bids[0].Price, holding, false, nil, setting)
	} else if holding < 0 && profitBuy > setting.CloseShortMargin {
		orderLiq = api.PlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeMarket, setting.MarketRelated, setting.SymbolRelated, ``,
			tickOrder.Asks[0].Price, tickOrder.Asks[0].Price, math.Abs(holding), false, nil, setting)
	}
	if orderLiq != nil && orderLiq.HaveId() {
		followOrderTime = now
		orderLiq.RefreshType = model.FunctionComplement
		model.AppDB.Save(&orderLiq)
		util.Notice(fmt.Sprintf(`liq order %s %s %s %s holding %e tick[%e %e] tickOrder[%e %e] profit %e %e`,
			orderLiq.Market, orderLiq.Symbol, orderLiq.OrderSide, orderLiq.OrderId, holding, tick.Bids[0].Price,
			tick.Asks[0].Price, tickOrder.Bids[0].Price, tickOrder.Asks[0].Price, profitBuy, profitSell))
	}
}

func placeFollow(account *model.Account, setting *model.Setting, tick, tickOrder *model.BidAsk) {
	var order *model.Order
	//if (followStatus == StatusDown && tickOrder.Bids[0].Price-setting.RateRelated*tick.Bids[0].Price > setting.OpenShortMargin) ||
	//	(followStatus == StatusNormal && tickOrder.Bids[0].Price-setting.RateRelated*tick.Asks[0].Price > setting.OpenShortMargin) {
	//	order = api.PlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeLimit, setting.MarketRelated, setting.SymbolRelated, ``,
	//		tick.Bids[0].Price*setting.RateRelated, tick.Bids[0].Price*setting.RateRelated, setting.GridAmount, false, nil, setting)
	//}
	//if (followStatus == StatusUp && tick.Asks[0].Price*setting.RateRelated-tickOrder.Asks[0].Price > setting.OpenShortMargin) ||
	//	(followStatus == StatusNormal && tick.Bids[0].Price*setting.RateRelated-tickOrder.Asks[0].Price > setting.OpenShortMargin) {
	//	order = api.PlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeLimit, setting.MarketRelated, setting.SymbolRelated, ``,
	//		tick.Asks[0].Price*setting.RateRelated, tick.Asks[0].Price*setting.RateRelated, setting.GridAmount, false, nil, setting)
	//}
	if tickOrder.Bids[0].Price-setting.RateRelated*tick.Asks[0].Price >= 0 {
		order = api.PlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeMarket, setting.MarketRelated, setting.SymbolRelated, ``,
			tickOrder.Asks[0].Price, tickOrder.Asks[0].Price, setting.GridAmount, false, nil, setting)
	} else if tick.Bids[0].Price*setting.RateRelated-tickOrder.Asks[0].Price >= 0 {
		order = api.PlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeMarket, setting.MarketRelated, setting.SymbolRelated, ``,
			tickOrder.Bids[0].Price, tickOrder.Bids[0].Price, setting.GridAmount, false, nil, setting)
	}
	if order != nil && order.HaveId() {
		ordPriceBid = tick.Bids[0].Price
		ordPriceAsk = tick.Asks[0].Price
		order.RefreshType = model.FunctionQueue
		model.AppDB.Save(&order)
		followOrderTime = time.Now().UnixMilli()
		util.Notice(fmt.Sprintf(`chance to follow %s at %e amt %e return %s tick[%e %e %e %e] tickLiq[%e %e %e %e]`,
			order.OrderSide, order.Price, order.Amount, order.OrderId, tick.Bids[0].Price, tick.Bids[0].Amount, tick.Asks[0].Price,
			tick.Asks[0].Amount, tickOrder.Bids[0].Price, tickOrder.Bids[0].Amount, tickOrder.Asks[0].Price, tickOrder.Asks[0].Amount))
	}
}

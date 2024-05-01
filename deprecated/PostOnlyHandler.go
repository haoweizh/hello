package deprecated

//import (
//	"fmt"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"time"
//)
//
//var PostonlyHandler = func(setting *model.Setting, tick *model.BidAsk) {
//	orderPostonly := model.AppMarkets.RemoveBmPendingOrder()
//	if setting == nil || model.AppConfig.HandleLink != `1` || orderPostonly == nil {
//		return
//	}
//	startTime := util.GetNowUnixMillion()
//	if orderPostonly.RefreshType != model.PostOnly || orderPostonly.Amount-orderPostonly.DealAmount < 1 ||
//		orderPostonly.OrderId == `` || orderPostonly.Status != model.CarryStatusFail {
//		if orderPostonly.RefreshType == model.PostOnly {
//			util.Notice(fmt.Sprintf(`leave alone: %s %s %s`,
//				orderPostonly.OrderId, orderPostonly.Status, orderPostonly.RefreshType))
//		}
//		return
//	}
//	priceBid := 0.0
//	priceAsk := 0.0
//	if tick != nil && tick.Bids != nil && tick.Asks != nil && tick.Bids.Len() > 0 && tick.Asks.Len() > 0 {
//		priceBid = tick.Bids[0].Price
//		priceAsk = tick.Asks[0].Price
//	} else {
//		util.Notice(`fatal error1: tick absent`)
//		_, restBid, restAsk := api.GetOrderBook(setting.Symbol)
//		if restBid != nil && restAsk != nil {
//			priceBid = restBid.Price
//			priceAsk = restAsk.Price
//		} else {
//			util.Notice(`fatal error2: rest bid ask nil`)
//		}
//	}
//	//if tick.Bids[0].Price >= tick.Asks[0].Price {
//	//	util.Notice(fmt.Sprintf(`error2 bad tick %f-%f`, tick.Bids[0].Price, tick.Asks[0].Price))
//	//	return
//	//}
//	//if int(startTime)-tick.Ts > 200 {
//	//	util.Notice(fmt.Sprintf(`error3 delay too long %d %d`, startTime, tick.Ts))
//	//	return
//	//}
//	orderSide := orderPostonly.OrderSide
//	price := orderPostonly.Price
//	if priceAsk > 0 && priceBid > 0 {
//		price = priceAsk - api.GetPriceDistance(orderPostonly.Market, orderPostonly.Symbol)
//		if orderSide == model.OrderSideSell {
//			price = priceBid + api.GetPriceDistance(orderPostonly.Market, orderPostonly.Symbol)
//		}
//	}
//	//if tick != nil && tick.Asks != nil && tick.Bids != nil && tick.Asks.Len() > 0 && tick.Bids.Len() > 0 {
//	//	price = tick.Asks[0].Price - api.GetPriceDistance(orderPostonly.Market, orderPostonly.Symbol)
//	//	if orderSide == model.OrderSideSell {
//	//		price = tick.Bids[0].Price + api.GetPriceDistance(orderPostonly.Market, orderPostonly.Symbol)
//	//	}
//	//} else {
//	//	util.Notice(`fatal tick error`)
//	//}
//	amount := orderPostonly.Amount - orderPostonly.DealAmount
//	for true {
//		orderPostonly = api.PlaceOrder(orderSide, model.OrderTypeLimit, setting.Market, setting.Symbol,
//			``, ``, model.PostOnly, model.PostOnly, price, 0, amount, true)
//		if orderPostonly != nil && orderPostonly.OrderId != `` {
//			break
//		}
//		time.Sleep(time.Second)
//	}
//	util.Notice(fmt.Sprintf(`reorder: %s order %s %s %s %f %f orderParam:<%s> delay:%d`, setting.Market,
//		orderSide, model.OrderTypeLimit, setting.Symbol, price, amount, model.PostOnly, util.GetNowUnixMillion()-startTime))
//}

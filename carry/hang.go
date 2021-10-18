package carry

import (
	"hello/api"
	"hello/model"
	"hello/util"
	"time"
)

var orderHang = make([]*model.Order, 0)

// ProcessHang setting中chance代表以bid1价格为起点的价格单位，chance可以是负数
var ProcessHang = func(setting *model.Setting, tick *model.BidAsk) {
	marketInfo := model.GetMarketInfo(setting.Market, setting.Symbol)
	if marketInfo == nil || setting == nil || tick == nil || tick.Asks == nil || tick.Bids == nil ||
		model.AppConfig.Handle != `1` || model.AppPause || util.GetNowUnixMillion()-int64(tick.Ts) > 40 {
		return
	}
	price := tick.Bids[0].Price + float64(setting.Chance)*marketInfo.PriceIncrement
	if len(orderHang) == 0 {
		order := api.PlaceOrder(``, ``, model.OrderSideSell, model.OrderTypeLimit, setting.Market, setting.Symbol,
			setting.Symbol, ``, model.FunctionHang, price, price, setting.GridAmount, true, false, nil, setting)
		if order.Status != model.CarryStatusFail && order != nil && len(order.OrderId) != 0 {
			orderHang = append(orderHang, order)
		}
		time.Sleep(time.Second)
	} else {
		orders := make([]*model.Order, 0)
		for _, order := range orderHang {
			if (order.Price <= tick.Bids[0].Price && order.OrderSide == model.OrderSideSell) ||
				(order.Price >= tick.Asks[0].Price && order.OrderSide == model.OrderSideBuy) {
				order.Status = model.CarryStatusSuccess
				order.DealAmount = order.Amount
				go model.AppDB.Save(order)
			} else if order.Price == price {
				orders = append(orders, order)
			} else {
				api.CancelOrder(``, ``, setting.Market, setting.Symbol, ``, model.OrderTypeLimit, order.OrderId)
			}
		}
		orderHang = orders
	}
}

package deprecated

//var orderHang = make([]*model.Order, 0)
//var hanging = false
//var hangLock sync.Mutex
//var cancelAll = make(map[string]bool)
//
//func setCancelAll(symbol string, value bool) {
//	hangLock.Lock()
//	defer hangLock.Unlock()
//	cancelAll[symbol] = value
//}
//
//func setHanging(value bool) {
//	hangLock.Lock()
//	defer hangLock.Unlock()
//	hanging = value
//}
//
//func getHanging() (value bool) {
//	return hanging
//}
//
//// ProcessHang setting中chance代表以bid1价格为起点的价格单位，chance可以是负数
//// Setting.GridAmount 卖出数量
//// Setting.CloseShortMargin 最低卖出价格
//// Setting.AmountLimit 下单休息时间，单位为毫秒
//var ProcessHang = func(setting *model.Setting, tick *model.BidAsk) {
//	marketInfo := model.GetMarketInfo(setting.Market, setting.Symbol)
//	if marketInfo == nil || setting == nil || tick == nil || tick.Asks == nil || tick.Bids == nil || getHanging() ||
//		model.AppConfig.HandleLink != `1` || model.AppPause || util.GetNowUnixMillion()-int64(tick.Ts) > 40 {
//		return
//	}
//	account := model.AppConfig.GetAccounts(setting.Market)[0]
//	if cancelAll[setting.Symbol] == false {
//		api.CancelOrders(account.Key, account.Secret, setting.Market, setting.Symbol)
//		setCancelAll(setting.Symbol, true)
//	}
//	setHanging(true)
//	defer setHanging(false)
//	price := tick.Bids[0].Price + float64(setting.Chance)*marketInfo.PriceIncrement
//	if len(orderHang) == 0 && price > setting.CloseShortMargin {
//		order := api.PlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeLimit, setting.Market, setting.Symbol,
//			setting.Symbol, ``, model.FunctionHang, price, price, setting.GridAmount, true, false, nil, setting)
//		if order.Status != model.CarryStatusFail && order != nil && len(order.OrderId) != 0 {
//			orderHang = append(orderHang, order)
//		}
//		time.Sleep(time.Duration(int64(setting.AmountLimit)) * time.Millisecond)
//	} else {
//		orders := make([]*model.Order, 0)
//		for _, order := range orderHang {
//			if (order.Price <= tick.Bids[0].Price && order.OrderSide == model.OrderSideSell) ||
//				(order.Price >= tick.Asks[0].Price && order.OrderSide == model.OrderSideBuy) {
//				order.Status = model.CarryStatusSuccess
//				order.DealAmount = order.Amount
//				go model.AppDB.Save(order)
//			} else if order.Price == price {
//				order = api.QueryOrderById(account.Key, account.Secret, setting.Market, setting.Symbol, setting.Symbol, model.OrderTypeLimit, order.OrderId)
//				if order.Status == model.CarryStatusWorking && order.DealAmount < order.Amount {
//					orders = append(orders, order)
//				} else {
//					go model.AppDB.Save(order)
//				}
//			} else {
//				api.CancelOrder(account.Key, account.Secret, setting.Market, setting.Symbol, ``, model.OrderTypeLimit, order.OrderId)
//				order = api.QueryOrderById(account.Key, account.Secret, setting.Market, setting.Symbol, setting.Symbol, model.OrderTypeLimit, order.OrderId)
//				if order.Status == model.CarryStatusWorking {
//					orders = append(orders, order)
//				} else {
//					go model.AppDB.Save(order)
//				}
//			}
//		}
//		orderHang = orders
//	}
//}

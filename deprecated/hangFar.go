package deprecated

//import (
//	"fmt"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"strconv"
//	"strings"
//	"sync"
//)
//
//type HangFarOrders struct {
//	lock             sync.Mutex
//	hangingFar       bool
//	revertOrders     map[string]*model.Order // order id - order
//	needRevertOrders []*model.Order
//	bidOrders        map[string]map[string]*model.Order // symbol - position - orders
//	askOrders        map[string]map[string]*model.Order // symbol - position - orders
//}
//
//var hangFarOrders = &HangFarOrders{}
//
//func (hangFarOrders *HangFarOrders) setInHangingFar(in bool) {
//	hangFarOrders.hangingFar = in
//}
//
//func (hangFarOrders *HangFarOrders) checkRevertOrders(orderId string) (isRevert bool) {
//	defer hangFarOrders.lock.Unlock()
//	hangFarOrders.lock.Lock()
//	if hangFarOrders.revertOrders == nil {
//		return false
//	}
//	return hangFarOrders.revertOrders[orderId] != nil
//}
//
//func (hangFarOrders *HangFarOrders) addRevertOrder(order *model.Order) {
//	defer hangFarOrders.lock.Unlock()
//	hangFarOrders.lock.Lock()
//	if hangFarOrders.revertOrders == nil {
//		hangFarOrders.revertOrders = make(map[string]*model.Order)
//	}
//	if order != nil && order.OrderId != `` {
//		hangFarOrders.revertOrders[order.OrderId] = order
//	}
//}
//
//func (hangFarOrders *HangFarOrders) removeRevertOrder(orderId string) {
//	defer hangFarOrders.lock.Unlock()
//	hangFarOrders.lock.Lock()
//	if hangFarOrders.revertOrders != nil && hangFarOrders.revertOrders[orderId] != nil {
//		delete(hangFarOrders.revertOrders, orderId)
//	}
//}
//
//func (hangFarOrders *HangFarOrders) addNeedRevertOrder(order *model.Order) {
//	defer hangFarOrders.lock.Unlock()
//	hangFarOrders.lock.Lock()
//	if hangFarOrders.needRevertOrders == nil {
//		hangFarOrders.needRevertOrders = make([]*model.Order, 0)
//	}
//	if order != nil && order.OrderId != `` {
//		hangFarOrders.needRevertOrders = append(hangFarOrders.needRevertOrders, order)
//	}
//}
//
//func (hangFarOrders *HangFarOrders) removeNeedRevertOrder() (order *model.Order) {
//	defer hangFarOrders.lock.Unlock()
//	hangFarOrders.lock.Lock()
//	if hangFarOrders.needRevertOrders != nil && len(hangFarOrders.needRevertOrders) > 0 {
//		order = hangFarOrders.needRevertOrders[0]
//		hangFarOrders.needRevertOrders = hangFarOrders.needRevertOrders[1:]
//	}
//	return order
//}
//
//func (hangFarOrders *HangFarOrders) checkFarOrders(symbol, orderId string) (isHangFar bool) {
//	defer hangFarOrders.lock.Unlock()
//	hangFarOrders.lock.Lock()
//	isHangFar = false
//	if hangFarOrders.bidOrders == nil {
//		hangFarOrders.bidOrders = make(map[string]map[string]*model.Order)
//	}
//	if hangFarOrders.askOrders == nil {
//		hangFarOrders.askOrders = make(map[string]map[string]*model.Order)
//	}
//	for _, order := range hangFarOrders.bidOrders[symbol] {
//		if order != nil && order.OrderId == orderId {
//			return true
//		}
//	}
//	for _, order := range hangFarOrders.askOrders[symbol] {
//		if order != nil && order.OrderId == orderId {
//			return true
//		}
//	}
//	return false
//}
//
//func (hangFarOrders *HangFarOrders) getFarOrders(symbol string) (bidOrders, askOrders map[string]*model.Order) {
//	defer hangFarOrders.lock.Unlock()
//	hangFarOrders.lock.Lock()
//	if hangFarOrders.bidOrders == nil {
//		hangFarOrders.bidOrders = make(map[string]map[string]*model.Order)
//	}
//	if hangFarOrders.askOrders == nil {
//		hangFarOrders.askOrders = make(map[string]map[string]*model.Order)
//	}
//	if hangFarOrders.bidOrders[symbol] == nil {
//		hangFarOrders.bidOrders[symbol] = make(map[string]*model.Order)
//	}
//	if hangFarOrders.askOrders[symbol] == nil {
//		hangFarOrders.askOrders[symbol] = make(map[string]*model.Order)
//	}
//	return hangFarOrders.bidOrders[symbol], hangFarOrders.askOrders[symbol]
//}
//
//func (hangFarOrders *HangFarOrders) setFarOrders(symbol string, bidOrders, askOrders map[string]*model.Order) {
//	defer hangFarOrders.lock.Unlock()
//	hangFarOrders.lock.Lock()
//	if hangFarOrders.bidOrders == nil {
//		hangFarOrders.bidOrders = make(map[string]map[string]*model.Order)
//	}
//	if hangFarOrders.askOrders == nil {
//		hangFarOrders.askOrders = make(map[string]map[string]*model.Order)
//	}
//	hangFarOrders.bidOrders[symbol] = bidOrders
//	hangFarOrders.askOrders[symbol] = askOrders
//}
//
//var ProcessHangFar = func(setting *model.Setting, tick *model.BidAsk) {
//	start := util.GetNowUnixMillion()
//	if tick == nil || tick.Asks == nil || tick.Bids == nil || tick.Asks.Len() < 15 || tick.Bids.Len() < 15 ||
//		int(start)-tick.Ts > 500 {
//		timeDis := 0
//		if tick != nil {
//			timeDis = int(start) - tick.Ts
//		}
//		util.Notice(fmt.Sprintf(`[tick not good time]%s %s %d`, setting.Market, setting.Symbol, timeDis))
//		CancelHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol)
//		return
//	}
//	parameters := strings.Split(setting.FunctionParameter, `_`)
//	posStr := make([]string, len(parameters)/3)
//	pos := make(map[string]float64)
//	posDis := make(map[string]float64)
//	amount := make(map[string]float64)
//	for i := 0; i < len(parameters)/3; i++ {
//		posStr[i] = parameters[i*3]
//		pos[posStr[i]], _ = strconv.ParseFloat(posStr[i], 64)
//		posDis[posStr[i]], _ = strconv.ParseFloat(parameters[i*3+1], 64)
//		amount[posStr[i]], _ = strconv.ParseFloat(parameters[i*3+2], 64)
//	}
//	if util.GetNowUnixMillion()-int64(tick.Ts) > 1000 || model.AppConfig.HandleLink != `1` || model.AppPause {
//		util.Notice(fmt.Sprintf(`[status]%s is pause:%#v`, model.AppConfig.HandleLink, model.AppPause))
//		CancelHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol)
//		return
//	}
//	if hangFarOrders.hangingFar {
//		return
//	}
//	hangFarOrders.setInHangingFar(true)
//	defer hangFarOrders.setInHangingFar(false)
//	if validHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, pos, posDis, tick) {
//		return
//	}
//	if len(hangFarOrders.needRevertOrders) > 0 {
//		util.Notice(fmt.Sprintf(`=need cancel revert= need:%d revert:%d bid:%d ask:%d`,
//			len(hangFarOrders.needRevertOrders), len(hangFarOrders.revertOrders),
//			len(hangFarOrders.bidOrders[setting.Symbol]), len(hangFarOrders.askOrders[setting.Symbol])))
//		revertCancelOrder(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol,
//			setting.AccountType, tick)
//	} else if hang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, setting.AccountType, pos,
//		amount, tick) {
//		CancelNonHang(setting.Market, setting.Symbol)
//	}
//}
//
//func revertCancelOrder(key, secret, market, symbol, accountType string, tick *model.BidAsk) {
//	cancelOrder := hangFarOrders.removeNeedRevertOrder()
//	if cancelOrder != nil && cancelOrder.DealAmount > 0 {
//		orderSide := model.OrderSideBuy
//		price := tick.Bids[0].Price
//		if cancelOrder.OrderSide == model.OrderSideBuy {
//			orderSide = model.OrderSideSell
//			price = tick.Asks[0].Price
//		}
//		revertOrder := api.PlaceOrder(key, secret, orderSide, model.OrderTypeLimit, market, symbol, ``,
//			accountType, ``, ``, price, 0, cancelOrder.DealAmount, false)
//		if revertOrder != nil && revertOrder.OrderId != `` {
//			hangFarOrders.addRevertOrder(revertOrder)
//			util.Notice(fmt.Sprintf(`=place revert= %s-%s amount%f - %f price %f - %f`,
//				cancelOrder.OrderSide, revertOrder.OrderSide, cancelOrder.DealAmount, revertOrder.Amount,
//				cancelOrder.Price, revertOrder.Price))
//			revertOrder.Function = model.FunctionHangRevert
//			revertOrder.RefreshType = cancelOrder.OrderId
//			model.AppDB.Save(&revertOrder)
//		}
//	}
//}
//
//func CancelNonHang(market, symbol string) {
//	orders := api.QueryOrders(model.KeyDefault, model.SecretDefault, market, symbol)
//	ordersBids, orderAsks := hangFarOrders.getFarOrders(symbol)
//	util.SocketInfo(fmt.Sprintf(`=query orders cancel non-hang= open:%d need:%d revert:%d bid:%d ask:%d`,
//		len(orders), len(hangFarOrders.needRevertOrders), len(hangFarOrders.revertOrders),
//		len(ordersBids), len(orderAsks)))
//	for _, order := range orders {
//		if order != nil && order.OrderId != `` {
//			needCancel := true
//			for _, value := range ordersBids {
//				if value != nil && value.OrderId == order.OrderId {
//					needCancel = false
//				}
//			}
//			for _, value := range orderAsks {
//				if value != nil && value.OrderId == order.OrderId {
//					needCancel = false
//				}
//			}
//			if needCancel {
//				util.Notice(`need cancel non hang ` + order.OrderId)
//				hangFarCancel(model.KeyDefault, model.SecretDefault, market, symbol, order.OrderType, order.OrderId)
//			}
//		}
//	}
//}
//
//func hang(key, secret, market, symbol, accountType string, pos, amount map[string]float64, tick *model.BidAsk) (dosmth bool) {
//	ordersBids, orderAsks := hangFarOrders.getFarOrders(symbol)
//	dosmth = false
//	for str, value := range pos {
//		bidPrice := (tick.Bids[0].Price + tick.Asks[0].Price) / 2 * (1 - value)
//		askPrice := (tick.Bids[0].Price + tick.Asks[0].Price) / 2 * (1 + value)
//		if ordersBids[str] == nil {
//			dosmth = true
//			order := api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, market, symbol, ``,
//				accountType, ``, ``, bidPrice, 0, amount[str], false)
//			if order != nil && order.OrderId != `` {
//				util.Notice(fmt.Sprintf(`=hang= at %s %s`, str, order.OrderId))
//				ordersBids[str] = order
//				order.Function = model.FunctionHangFar
//				model.AppDB.Save(&order)
//			}
//		}
//		if orderAsks[str] == nil {
//			dosmth = true
//			order := api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, market, symbol, ``,
//				accountType, ``, ``, askPrice, 0, amount[str], false)
//			if order != nil && order.OrderId != `` {
//				util.Notice(fmt.Sprintf(`=hang= at %s %s`, str, order.OrderId))
//				orderAsks[str] = order
//				order.Function = model.FunctionHangFar
//				model.AppDB.Save(&order)
//			}
//		}
//	}
//	if dosmth {
//		util.Notice(fmt.Sprintf(`hang orders bid %d ask %d`, len(ordersBids), len(orderAsks)))
//		hangFarOrders.setFarOrders(symbol, ordersBids, orderAsks)
//	}
//	return dosmth
//}
//
//func validHang(key, secret, market, symbol string, pos, dis map[string]float64, tick *model.BidAsk) (doSmTh bool) {
//	doSmTh = false
//	bidOrders, askOrders := hangFarOrders.getFarOrders(symbol)
//	bidOrdersValid := make(map[string]*model.Order)
//	askOrdersValid := make(map[string]*model.Order)
//	for posStr, order := range bidOrders {
//		if pos[posStr] == 0 || (pos[posStr] > 0 &&
//			(order.Price <= tick.Asks[0].Price*(1-pos[posStr]-dis[posStr]) ||
//				order.Price >= tick.Bids[0].Price*(1-pos[posStr]+dis[posStr]))) {
//			util.Notice(`cancel invalid ` + order.OrderId)
//			hangFarCancel(key, secret, market, symbol, order.OrderType, order.OrderId)
//			doSmTh = true
//		} else {
//			bidOrdersValid[posStr] = order
//		}
//	}
//	for posStr, order := range askOrders {
//		if pos[posStr] == 0 || (pos[posStr] > 0 &&
//			(order.Price <= tick.Asks[0].Price*(1+pos[posStr]-dis[posStr]) ||
//				order.Price >= tick.Bids[0].Price*(1+pos[posStr]+dis[posStr]))) {
//			util.Notice(`cancel invalid ` + order.OrderId)
//			hangFarCancel(key, secret, market, symbol, order.OrderType, order.OrderId)
//			doSmTh = true
//		} else {
//			askOrdersValid[posStr] = order
//		}
//	}
//	hangFarOrders.setFarOrders(symbol, bidOrdersValid, askOrdersValid)
//	return doSmTh
//}
//
//func CancelHang(key, secret, market, symbol string) {
//	bidOrders, askOrders := hangFarOrders.getFarOrders(symbol)
//	hangFarOrders.setFarOrders(symbol, nil, nil)
//	util.Notice(`[cancel all orders]`)
//	for _, order := range bidOrders {
//		if order != nil && order.OrderId != `` {
//			util.Notice(`cancel hang all ` + order.OrderId)
//			hangFarCancel(key, secret, market, symbol, order.OrderType, order.OrderId)
//		}
//	}
//	for _, order := range askOrders {
//		if order != nil && order.OrderId != `` {
//			util.Notice(`cancel hang all ` + order.OrderId)
//			hangFarCancel(key, secret, market, symbol, order.OrderType, order.OrderId)
//		}
//	}
//}
//
//func hangFarCancel(key, secret, market, symbol, orderType, orderId string) {
//	if hangFarOrders.checkRevertOrders(orderId) {
//		util.Notice(fmt.Sprintf(`=keep revert= %s`, orderId))
//	} else {
//		util.Notice(fmt.Sprintf(`==cancel other pending== %s`, orderId))
//		_, cancelOrder := api.MustCancel(key, secret, market, symbol, ``, orderType, orderId, true)
//		if cancelOrder != nil && cancelOrder.DealAmount > 0 && hangFarOrders.checkFarOrders(symbol, orderId) {
//			util.Notice(fmt.Sprintf(`=add need revert= %s %s deal %f`,
//				cancelOrder.OrderId, cancelOrder.OrderSide, cancelOrder.DealAmount))
//			model.AppDB.Save(&cancelOrder)
//			hangFarOrders.addNeedRevertOrder(cancelOrder)
//		}
//	}
//}

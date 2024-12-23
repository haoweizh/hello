package deprecated

//
//import (
//	"crypto/hmac"
//	"crypto/sha256"
//	"encoding/hex"
//	"encoding/json"
//	"fmt"
//	"github.com/gorilla/websocket"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"sort"
//	"strconv"
//	"strings"
//	"sync"
//	"time"
//)
//
//const restBitmex = `https://www.bitmex.com/api/v1`
//const wsBitmex = `wss://www.bitmex.com/realtime/`
//const wsStepBitmex = 8
//
//var socketLockBitmex sync.Mutex
//var prePriceB, prePriceA, prePriceB10, prePriceA10 float64
//
//var subscribeHandlerBitmex = func(connection *websocket.conn, subscribes []interface{}) error {
//	var err error = nil
//	expire := util.GetNow().Unix() + 5
//	toBeSign := fmt.Sprintf(`GET/realtime%d`, expire)
//	account := model.AppConfig.GetAccounts(model.Bitmex)[0]
//	hash := hmac.New(sha256.New, []byte(account.Secret))
//	hash.WriteServe([]byte(toBeSign))
//	sign := hex.EncodeToString(hash.Sum(nil))
//	authCmd := fmt.Sprintf(`{"op": "authKeyExpires", "args": ["%s", %d, "%s"]}`, account.Key, expire, sign)
//	if err = api.SendToConnection(model.Bitmex, connection, []byte(authCmd)); err != nil {
//		util.SocketInfo("bitmex can not auth " + err.Error())
//	}
//	subscribeMap := make(map[string]interface{})
//	subscribeMap[`op`] = `subscribe`
//	subscribeMap[`args`] = subscribes
//	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
//	if err = api.SendToConnection(model.Bitmex, connection, subscribeMessage); err != nil {
//		util.SocketInfo("bitmex can not subscribe " + err.Error())
//		return err
//	}
//	return err
//}
//
//func WsDepthServeBitmex(markets *model.Markets, orderHandler api.OrderHandler) ([]chan struct{}, error) {
//	lastPingTime := util.GetNow().Unix()
//	wsHandler := func(connection *websocket.conn, event []byte, orderHandler api.OrderHandler) {
//		socketLockBitmex.Lock()
//		defer socketLockBitmex.Unlock()
//		if util.GetNow().Unix()-lastPingTime > 30 { // ping bitmex server every 5 seconds
//			lastPingTime = util.GetNow().Unix()
//			if err := api.SendToAllConnections(model.Bitmex, []byte(`ping`)); err != nil {
//				util.SocketInfo("bitmex server ping client error " + err.Error())
//			}
//		}
//		if len(event) == 0 {
//			return
//		}
//		depthJson, depthErr := util.NewJSON(event)
//		if depthJson == nil {
//			return
//		}
//		action, actionErr := depthJson.Get(`action`).String()
//		data, dataErr := depthJson.Get(`data`).Array()
//		table := depthJson.Get(`table`).MustString()
//		if depthErr != nil || actionErr != nil || dataErr != nil || data == nil {
//			util.SocketInfo(`bitmex parse err` + string(event))
//			return
//		}
//		switch table {
//		case `quote`:
//			handleQuote(markets, action, data)
//		case `trade`:
//			handleTrade(markets, action, data)
//		case `orderBookL2_25`:
//			//util.SocketInfo(`fm book25 ` + string(event))
//			handleOrderBook(markets, action, data)
//		case `orderBookL2`:
//			handleOrderBook(markets, action, data)
//		case `orderBook10`:
//			handOrderBook10(markets, data)
//		case `order`:
//			handleOrder(markets, action, data)
//		case `position`:
//		}
//	}
//	subscribes := api.GetWSSubscribes(model.Bitmex, model.SubscribeDepth)
//	return api.WebSocketClient(model.Bitmex, wsBitmex, subscribes, subscribeHandlerBitmex, wsHandler, orderHandler, wsStepBitmex)
//}
//
//func parseQuote(item map[string]interface{}) (bid, ask *model.Tick, quoteTime time.Time, symbol string) {
//	if item == nil {
//		return
//	}
//	bid = &model.Tick{Market: model.Bitmex, Side: model.OrderSideBuy}
//	ask = &model.Tick{Market: model.Bitmex, Side: model.OrderSideSell}
//	if item[`symbol`] != nil {
//		symbol = model.GetStandardPerp(model.Bitmex, item[`symbol`].(string))
//		bid.Symbol = symbol
//		ask.Symbol = symbol
//	}
//	if item[`bidPrice`] != nil {
//		price, err := item[`bidPrice`].(json.Number).Float64()
//		if err == nil {
//			bid.Price = price
//		}
//	}
//	if item[`askPrice`] != nil {
//		price, err := item[`askPrice`].(json.Number).Float64()
//		if err == nil {
//			ask.Price = price
//		}
//	}
//	if item[`bidSize`] != nil {
//		size, err := item[`bidSize`].(json.Number).Float64()
//		if err == nil {
//			bid.Amount = size
//		}
//	}
//	if item[`askSize`] != nil {
//		size, err := item[`askSize`].(json.Number).Float64()
//		if err == nil {
//			ask.Amount = size
//		}
//	}
//	if item[`timestamp`] != nil {
//		quoteTime, _ = time.Parse(time.RFC3339, item[`timestamp`].(string))
//	}
//	return
//}
//
//func parseTick(item map[string]interface{}) (tick *model.Tick) {
//	if item == nil {
//		return nil
//	}
//	tick = &model.Tick{Market: model.Bitmex}
//	if item[`symbol`] != nil {
//		tick.Symbol = model.GetStandardPerp(model.Bitmex, item[`symbol`].(string))
//	}
//	if item[`id`] != nil {
//		id, err := item[`id`].(json.Number).Int64()
//		if err == nil {
//			tick.Id = strconv.FormatInt(id, 10)
//		}
//	}
//	if item[`size`] != nil {
//		amount, err := item[`size`].(json.Number).Float64()
//		if err == nil {
//			tick.Amount = amount
//		}
//	}
//	if item[`price`] != nil {
//		price, err := item[`price`].(json.Number).Float64()
//		if err == nil {
//			tick.Price = price
//		}
//	}
//	if item[`side`] != nil {
//		tick.Side = strings.ToLower(item[`side`].(string))
//	}
//	return tick
//}
//
//func parseOrderBM(order *model.Order, item map[string]interface{}) {
//	if item[`orderID`] != nil {
//		order.OrderId = item[`orderID`].(string)
//	}
//	if item[`symbol`] != nil {
//		order.Symbol = model.GetStandardPerp(model.Bitmex, item[`symbol`].(string))
//	}
//	if item[`side`] != nil {
//		order.OrderSide = strings.ToLower(item[`side`].(string))
//	}
//	if item[`orderQty`] != nil {
//		order.Amount, _ = item[`orderQty`].(json.Number).Float64()
//	}
//	if item[`price`] != nil {
//		order.Price, _ = item[`price`].(json.Number).Float64()
//	}
//	if item[`workingIndicator`] != nil && item[`workingIndicator`].(bool) {
//		order.Status = model.CarryStatusWorking
//	} else if item[`ordStatus`] != nil {
//		order.Status = model.GetOrderStatus(model.Bitmex, item[`ordStatus`].(string))
//	}
//	if item[`cumQty`] != nil {
//		order.DealAmount, _ = item[`cumQty`].(json.Number).Float64()
//	}
//	if item[`avgPx`] != nil {
//		order.DealPrice, _ = item[`avgPx`].(json.Number).Float64()
//	}
//	if item[`execInst`] != nil && item[`execInst`] == model.PostOnly {
//		order.RefreshType = model.PostOnly
//	}
//	return
//}
//
//func handOrderBook10(markets *model.Markets, data []interface{}) {
//	if data == nil || len(data) < 1 {
//		return
//	}
//	bidAsk := &model.BidAsk{}
//	symbol := ``
//	item := data[0].(map[string]interface{})
//	if item[`symbol`] != nil {
//		symbol = model.GetStandardPerp(model.Bitmex, item[`symbol`].(string))
//	}
//	if item[`timestamp`] != nil {
//		tickTime, _ := time.Parse(time.RFC3339, item[`timestamp`].(string))
//		bidAsk.Ts = int(tickTime.UnixNano() / 1000000)
//		bidAsk.TsReceived = int(util.GetNowUnixMillion())
//	}
//	if item[`bids`] != nil {
//		bidItems := item[`bids`].([]interface{})
//		bidAsk.Bids = make([]model.Tick, len(bidItems))
//		for key, value := range bidItems {
//			if value == nil {
//				continue
//			}
//			pair := value.([]interface{})
//			if len(pair) == 2 {
//				price, _ := pair[0].(json.Number).Float64()
//				amount, _ := pair[1].(json.Number).Float64()
//				bidAsk.Bids[key] = model.Tick{Side: model.OrderSideBuy, Symbol: symbol, Price: price, Amount: amount, Market: model.Bitmex}
//			}
//		}
//	}
//	if item[`asks`] != nil {
//		askItems := item[`asks`].([]interface{})
//		bidAsk.Asks = make([]model.Tick, len(askItems))
//		for key, value := range askItems {
//			if value == nil {
//				continue
//			}
//			pair := value.([]interface{})
//			if len(pair) == 2 {
//				price, _ := pair[0].(json.Number).Float64()
//				amount, _ := pair[1].(json.Number).Float64()
//				bidAsk.Asks[key] = model.Tick{Side: model.OrderSideSell, Symbol: symbol, Price: price, Amount: amount, Market: model.Bitmex}
//			}
//		}
//	}
//	sort.Sort(bidAsk.Asks)
//	sort.Sort(sort.Reverse(bidAsk.Bids))
//	if model.AppConfig.Env == "test" && symbol == "btcusd_p" && bidAsk.Bids != nil && bidAsk.Asks != nil &&
//		bidAsk.Bids.Len() > 0 && bidAsk.Asks.Len() > 0 {
//		if prePriceB10 != bidAsk.Bids[0].Price || prePriceA10 != bidAsk.Asks[0].Price {
//			util.SocketInfo(fmt.Sprintf(`++++++%f-%f %f-%fbm delay %d %f-%f`,
//				bidAsk.Bids[0].Price, bidAsk.Asks[0].Price, bidAsk.Bids[0].Amount, bidAsk.Asks[0].Amount,
//				util.GetNowUnixMillion()-int64(bidAsk.Ts), prePriceB10, prePriceA10))
//			prePriceB10 = bidAsk.Bids[0].Price
//			prePriceA10 = bidAsk.Asks[0].Price
//		}
//	}
//	if markets.SetBidAsk(symbol, model.Bitmex, bidAsk) {
//		for function, handler := range model.GetFunctions(model.Bitmex, symbol) {
//			if function == model.FunctionPostonlyHandler {
//				continue
//			}
//			if handler != nil {
//				setting := model.GetSetting(function, model.Bitmex, symbol)
//				if setting != nil {
//					go handler(setting, bidAsk)
//				}
//			}
//		}
//	}
//}
//
//func handleQuote(markets *model.Markets, action string, data []interface{}) {
//	symbolTicks := make(map[string]*model.BidAsk)
//	var compareTime *time.Time
//	for _, value := range data {
//		item := value.(map[string]interface{})
//		bid, ask, quoteTime, symbol := parseQuote(item)
//		switch action {
//		case `partial`:
//			symbolTicks[symbol] = &model.BidAsk{Ts: int(quoteTime.UnixNano() / 1000000),
//				TsReceived: int(util.GetNowUnixMillion()), Bids: []model.Tick{*bid}, Asks: []model.Tick{*ask}}
//		case `insert`:
//			if compareTime == nil || compareTime.Before(quoteTime) {
//				compareTime = &quoteTime
//				symbolTicks[symbol] = &model.BidAsk{Ts: int(quoteTime.UnixNano() / 1000000),
//					TsReceived: int(util.GetNowUnixMillion()), Bids: []model.Tick{*bid}, Asks: []model.Tick{*ask}}
//			}
//		}
//	}
//	for symbol, bidAsks := range symbolTicks {
//		if markets.SetBidAsk(symbol, model.Bitmex, bidAsks) {
//			for function, handler := range model.GetFunctions(model.Bitmex, symbol) {
//				if handler != nil {
//					setting := model.GetSetting(function, model.Bitmex, symbol)
//					if setting != nil {
//						go handler(setting, bidAsks)
//					}
//				}
//			}
//		}
//	}
//}
//
//func handleOrder(markets *model.Markets, action string, data []interface{}) {
//	//var orders map[string]*model.Order
//	for _, value := range data {
//		switch action {
//		case `partial`:
//			//if orders == nil {
//			//	orders = make(map[string]*model.Order)
//			//}
//			//order := &model.Order{Market: model.Bitmex}
//			//parseOrderBM(order, value.(map[string]interface{}))
//			//if order.OrderId != `` {
//			//	orders[order.OrderId] = order
//			//}
//		case `insert`:
//			//if orders == nil {
//			//	orders = markets.GetBmPendingOrders()
//			//}
//			order := &model.Order{Market: model.Bitmex}
//			parseOrderBM(order, value.(map[string]interface{}))
//			//if order.OrderId != `` {
//			//	orders[order.OrderId] = order
//			//}
//			if order.OrderId == `` {
//				continue
//			}
//			for function, handler := range model.GetFunctions(model.Bitmex, order.Symbol) {
//				if handler != nil && function == model.FunctionPostonlyHandler && model.AppConfig.Env != `test` {
//					markets.AddBMPendingOrder(order)
//					setting := model.GetSetting(function, model.Bitmex, order.Symbol)
//					if setting != nil {
//						go handler(setting, nil)
//					}
//				}
//			}
//		case `update`:
//			//if orders == nil {
//			//	orders = markets.GetBmPendingOrders()
//			//}
//			//if orders != nil {
//			//	order := &model.Order{Market: model.Bitmex}
//			//	parseOrderBM(order, value.(map[string]interface{}))
//			//	if order.OrderId != `` {
//			//		orderOld := orders[order.OrderId]
//			//		if orderOld != nil {
//			//			parseOrderBM(orderOld, value.(map[string]interface{}))
//			//		}
//			//	}
//			//}
//		}
//	}
//	//markets.SetBMPendingOrders(orders)
//}
//
//func handleOrderBook(markets *model.Markets, action string, data []interface{}) {
//	symbolTicks := make(map[string]*model.BidAsk)
//	for _, value := range data {
//		tick := parseTick(value.(map[string]interface{}))
//		if tick == nil || tick.Symbol == `` {
//			continue
//		}
//		switch action {
//		case `partial`:
//			if symbolTicks[tick.Symbol] == nil {
//				symbolTicks[tick.Symbol] = &model.BidAsk{}
//				symbolTicks[tick.Symbol].Asks = model.Ticks{}
//				symbolTicks[tick.Symbol].Bids = model.Ticks{}
//			}
//			if tick.Side == model.OrderSideBuy {
//				symbolTicks[tick.Symbol].Bids = append(symbolTicks[tick.Symbol].Bids, *tick)
//			}
//			if tick.Side == model.OrderSideSell {
//				symbolTicks[tick.Symbol].Asks = append(symbolTicks[tick.Symbol].Asks, *tick)
//			}
//		case `update`:
//			if symbolTicks[tick.Symbol] == nil {
//				_, symbolTicks[tick.Symbol] = markets.CopyBidAsk(tick.Symbol, model.Bitmex)
//			}
//			if symbolTicks[tick.Symbol] == nil {
//				continue
//			}
//			newBids := model.Ticks{}
//			newAsks := model.Ticks{}
//			for _, ask := range symbolTicks[tick.Symbol].Asks {
//				if ask.Id == tick.Id {
//					if tick.Amount > 0 {
//						ask.Amount = tick.Amount
//					}
//					if tick.Side != `` {
//						ask.Side = tick.Side
//					}
//					if tick.Price > 0 {
//						ask.Price = tick.Price
//					}
//				}
//				newAsks = append(newAsks, ask)
//			}
//			symbolTicks[tick.Symbol].Asks = newAsks
//			for _, bid := range symbolTicks[tick.Symbol].Bids {
//				if bid.Id == tick.Id {
//					if tick.Amount > 0 {
//						bid.Amount = tick.Amount
//					}
//					if tick.Side != `` {
//						bid.Side = tick.Side
//					}
//					if tick.Price > 0 {
//						bid.Price = tick.Price
//					}
//				}
//				newBids = append(newBids, bid)
//			}
//			symbolTicks[tick.Symbol].Bids = newBids
//		case `insert`:
//			if symbolTicks[tick.Symbol] == nil {
//				_, symbolTicks[tick.Symbol] = markets.CopyBidAsk(tick.Symbol, model.Bitmex)
//				//util.SocketInfo(`copy bm from ` + model.AppMarkets.ToStringBidAsk(symbolTicks[tick.Symbol]))
//			}
//			if symbolTicks[tick.Symbol] == nil {
//				continue
//			}
//			if tick.Side == model.OrderSideBuy {
//				symbolTicks[tick.Symbol].Bids = append(symbolTicks[tick.Symbol].Bids, *tick)
//				//util.SocketInfo(fmt.Sprintf(`+++++buy %f`, tick.Price))
//			}
//			if tick.Side == model.OrderSideSell {
//				symbolTicks[tick.Symbol].Asks = append(symbolTicks[tick.Symbol].Asks, *tick)
//				//util.SocketInfo(fmt.Sprintf(`++++sell %f`, tick.Price))
//			}
//		case `delete`:
//			if symbolTicks[tick.Symbol] == nil {
//				_, symbolTicks[tick.Symbol] = markets.CopyBidAsk(tick.Symbol, model.Bitmex)
//				//util.SocketInfo(`copy bm from ` + model.AppMarkets.ToStringBidAsk(symbolTicks[tick.Symbol]))
//			}
//			if symbolTicks[tick.Symbol] == nil {
//				continue
//			}
//			if tick.Side == model.OrderSideBuy {
//				newBids := model.Ticks{}
//				for _, bid := range symbolTicks[tick.Symbol].Bids {
//					if bid.Id != tick.Id {
//						newBids = append(newBids, bid)
//					}
//					//else {
//					//	util.SocketInfo(fmt.Sprintf(`-----buy %s %f`, tick.Id, bid.Price))
//					//}
//				}
//				symbolTicks[tick.Symbol].Bids = newBids
//			}
//			if tick.Side == model.OrderSideSell {
//				newAsks := model.Ticks{}
//				for _, ask := range symbolTicks[tick.Symbol].Asks {
//					if ask.Id != tick.Id {
//						newAsks = append(newAsks, ask)
//					}
//					//else {
//					//	util.SocketInfo(fmt.Sprintf(`----sell %s %f`, tick.Id, ask.Price))
//					//}
//				}
//				symbolTicks[tick.Symbol].Asks = newAsks
//			}
//		}
//	}
//	for symbol, bidAsks := range symbolTicks {
//		if bidAsks == nil {
//			continue
//		}
//		sort.Sort(bidAsks.Asks)
//		sort.Sort(sort.Reverse(bidAsks.Bids))
//		if bidAsks.Bids != nil && bidAsks.Asks != nil {
//			bidAsks.Ts = int(util.GetNowUnixMillion())
//			bidAsks.TsReceived = int(util.GetNowUnixMillion())
//			if model.AppConfig.Env == "test" && symbol == "btcusd_p" &&
//				bidAsks.Bids.Len() > 0 && bidAsks.Asks.Len() > 0 {
//				if prePriceB != bidAsks.Bids[0].Price || prePriceA != bidAsks.Asks[0].Price {
//					util.SocketInfo(fmt.Sprintf(`------%f-%f %f-%fbm delay %d %f-%f`,
//						bidAsks.Bids[0].Price, bidAsks.Asks[0].Price, bidAsks.Bids[0].Amount, bidAsks.Asks[0].Amount,
//						util.GetNowUnixMillion()-int64(bidAsks.Ts), prePriceB, prePriceA))
//					prePriceB = bidAsks.Bids[0].Price
//					prePriceA = bidAsks.Asks[0].Price
//				}
//			}
//			if markets.SetBidAsk(symbol, model.Bitmex, bidAsks) {
//				for function, handler := range model.GetFunctions(model.Bitmex, symbol) {
//					if handler != nil && model.AppConfig.Env != `test` {
//						setting := model.GetSetting(function, model.Bitmex, symbol)
//						if setting != nil {
//							go handler(setting, bidAsks)
//						}
//					}
//				}
//			}
//		}
//	}
//}
//
//func handleTrade(markets *model.Markets, action string, data []interface{}) {
//	switch action {
//	case `partial`:
//	case `update`:
//	case `insert`:
//		for _, value := range data {
//			item := value.(map[string]interface{})
//			if item == nil {
//				continue
//			}
//			deal := &model.Deal{Market: model.Bitmex}
//			if item[`timestamp`] != nil {
//				dealTime, err := time.Parse(time.RFC3339, item[`timestamp`].(string))
//				if err == nil {
//					deal.Ts = dealTime.UnixNano() / 1000000
//				}
//			}
//			if item[`size`] != nil {
//				amount, err := item[`size`].(json.Number).Float64()
//				if err == nil {
//					deal.Amount = amount
//				}
//			}
//			if item[`price`] != nil {
//				price, err := item[`price`].(json.Number).Float64()
//				if err == nil {
//					deal.Price = price
//				}
//			}
//			if item[`symbol`] != nil {
//				deal.Symbol = model.GetStandardPerp(model.Bitmex, item[`symbol`].(string))
//			}
//			if item[`side`] != nil {
//				deal.Side = item[`side`].(string)
//			}
//			markets.SetTrade(deal)
//		}
//	case `delete`:
//	}
//}
//
//func SignedRequestBitmex(key, secret, method, path string, body map[string]interface{}) []byte {
//	uri := restBitmex + path
//	expire := util.GetNow().Unix() + 5
//	if method == `GET` && len(body) > 0 {
//		uri += `?` + util.ComposeParams(body)
//	}
//	signPath := uri
//	if strings.Contains(signPath, `//`) {
//		signPath = signPath[strings.Index(signPath, `//`)+2:]
//		signPath = signPath[strings.Index(signPath, `/`):]
//	}
//	toBeSign := fmt.Sprintf(`%s%s%d`, method, signPath, expire)
//	//if method != `GET` {
//	toBeSign += string(util.JsonEncodeToByte(body))
//	//}
//	hash := hmac.New(sha256.New, []byte(secret))
//	hash.WriteServe([]byte(toBeSign))
//	sign := hex.EncodeToString(hash.Sum(nil))
//	headers := map[string]string{`api-key`: key, `api-expires`: strconv.FormatInt(expire, 10),
//		`api-signature`: sign, "Content-Type": "application/json"}
//	var responseBody []byte
//	if body == nil {
//		responseBody, _ = util.HttpRequest(method, uri, ``, headers, 60)
//	} else {
//		responseBody, _ = util.HttpRequest(method, uri, string(util.JsonEncodeToByte(body)), headers, 60)
//	}
//	return responseBody
//}
//
//func cancelOrderBitmex(key, secret, orderId string) (result bool, errCode, msg string) {
//	postData := make(map[string]interface{})
//	postData[`orderID`] = orderId
//	response := SignedRequestBitmex(key, secret, `DELETE`, `/order`, postData)
//	util.Notice(fmt.Sprintf(`cancel bm order %s return %s`, orderId, string(response)))
//	orderJson, err := util.NewJSON(response)
//	if err == nil {
//		orderArray, arrayErr := orderJson.Array()
//		if arrayErr == nil {
//			for _, value := range orderArray {
//				item := value.(map[string]interface{})
//				if item == nil {
//					continue
//				}
//				if item[`orderID`] != nil {
//					itemId := item[`orderID`].(string)
//					if itemId == orderId {
//						if item[`ordStatus`] != nil {
//							status := item[`ordStatus`].(string)
//							status = model.GetOrderStatus(model.Bitmex, status)
//							if status != `` && status != model.CarryStatusWorking {
//								return true, ``, ``
//							}
//						}
//					}
//				}
//			}
//		}
//	} else {
//		return false, ``, err.Error()
//	}
//	return false, ``, ``
//}
//
////queryOrderBitmex
//func _(key, secret, symbol, orderId string) (orders []*model.Order) {
//	orders = make([]*model.Order, 0)
//	postData := make(map[string]interface{})
//	symbol = model.GetDialectPerp(model.Bitmex, symbol)
//	postData[`symbol`] = symbol
//	postData[`reverse`] = `true`
//	postData[`filter`] = fmt.Sprintf(`{"orderID":"%s"}`, orderId)
//	response := SignedRequestBitmex(key, secret, `GET`, `/order`, postData)
//	util.SocketInfo(`bm query order:` + string(response))
//	orderJson, err := util.NewJSON(response)
//	if err == nil {
//		orderArray, _ := orderJson.Array()
//		for _, data := range orderArray {
//			order := &model.Order{Market: model.Bitmex}
//			parseOrderBM(order, data.(map[string]interface{}))
//			if order.OrderId != `` {
//				orders = append(orders, order)
//			}
//		}
//	}
//	return
//}
//
//func placeOrderBitmex(order *model.Order, key, secret, orderSide, orderType, execInst, symbol string, price, amount float64) {
//	postData := make(map[string]interface{})
//	symbol = model.GetDialectPerp(model.Bitmex, symbol)
//	postData["symbol"] = symbol
//	postData["side"] = strings.ToUpper(orderSide[0:1]) + orderSide[1:]
//	postData["orderQty"] = amount
//	if orderType != model.OrderTypeMarket && orderType != model.OrderTypeStop {
//		postData[`price`] = price
//	}
//	postData["ordType"] = strings.ToUpper(orderType[0:1]) + orderType[1:]
//	if execInst != `` {
//		postData[`execInst`] = execInst
//	}
//	if orderType == model.OrderTypeStop {
//		postData[`stopPx`] = price
//		postData[`execInst`] = `LastPrice`
//	}
//	response := SignedRequestBitmex(key, secret, `POST`, `/order`, postData)
//	util.Notice(`---- place bitmex` + string(response))
//	orderJson, err := util.NewJSON(response)
//	if err == nil {
//		item, orderErr := orderJson.Map()
//		if orderErr == nil {
//			parseOrderBM(order, item)
//		}
//	}
//	return
//}
//
//// binSize [1m,5m,1h,1d]
//func getCandlesBitmex(key, secret, symbol, binSize string, start, end time.Time, count int) (
//	candles map[string]*model.Candle) {
//	candles = make(map[string]*model.Candle)
//	postData := make(map[string]interface{})
//	symbolNew := model.GetDialectPerp(model.Bitmex, symbol)
//	postData[`symbol`] = symbolNew
//	postData[`reverse`] = `false`
//	postData[`binSize`] = binSize
//	postData[`count`] = strconv.Itoa(count)
//	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
//	postData[`startTime`] = start.Format(time.RFC3339)
//	//fmt.Println(start.Format(time.RFC3339))
//	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
//	//fmt.Println(end.Format(time.RFC3339))
//	postData[`endTime`] = end.Format(time.RFC3339)
//	response := SignedRequestBitmex(key, secret, `GET`, `/trade/bucketed`, postData)
//	util.SocketInfo(fmt.Sprintf(`bitmex get candle %s`, string(response)))
//	candleJson, err := util.NewJSON(response)
//	if err == nil {
//		candleJsons, err := candleJson.Array()
//		if err == nil {
//			duration, _ := time.ParseDuration(`-24h`)
//			for _, value := range candleJsons {
//				item := value.(map[string]interface{})
//				candle := &model.Candle{Market: model.Bitmex, Symbol: symbol, Period: binSize}
//				if item[`open`] != nil {
//					candle.PriceOpen, _ = item[`open`].(json.Number).Float64()
//				}
//				if item[`close`] != nil {
//					candle.PriceClose, _ = item[`close`].(json.Number).Float64()
//				}
//				if item[`high`] != nil {
//					candle.PriceHigh, _ = item[`high`].(json.Number).Float64()
//				}
//				if item[`low`] != nil {
//					candle.PriceLow, _ = item[`low`].(json.Number).Float64()
//				}
//				if item[`timestamp`] != nil {
//					start, _ := time.Parse(time.RFC3339, item[`timestamp`].(string))
//					start = start.Add(duration) // bitmex 返回的是当前时间段的上一个时间段的数据
//					candle.UTCDate = start.Format(time.RFC3339)[0:10]
//					candles[candle.UTCDate] = candle
//				}
//			}
//		}
//	}
//	return
//}
//
//func GetBtcBalanceBitmex(key, secret string) (balance float64) {
//	postData := make(map[string]interface{})
//	postData[`currency`] = `XBt`
//	response := SignedRequestBitmex(key, secret, `GET`, `/user/margin`, postData)
//	balanceJson, err := util.NewJSON(response)
//	if err != nil || balanceJson == nil || balanceJson.Get(`marginBalance`) == nil {
//		time.Sleep(time.Minute * 5)
//		util.SocketInfo(`fail to get btc balance binance`)
//		return GetBtcBalanceBitmex(key, secret)
//	}
//	balanceJson = balanceJson.Get(`marginBalance`)
//	if balanceJson != nil {
//		balance, err = balanceJson.Float64()
//		balance = balance / 100000000
//	}
//	return balance
//}
//
//// update: 以unix second表示的下一次更新资金费率的时间
//func getFundingRateBitmex(key, secret, symbol string) (fundingRate float64, update int64) {
//	postData := make(map[string]interface{})
//	symbol = model.GetDialectPerp(model.Bitmex, symbol)
//	postData[`symbol`] = symbol
//	response := SignedRequestBitmex(key, secret, `GET`, `/instrument`, postData)
//	instrumentJson, err := util.NewJSON(response)
//	if err == nil {
//		array, err := instrumentJson.Array()
//		if err == nil {
//			for _, value := range array {
//				item := value.(map[string]interface{})
//				if item[`symbol`] != nil && item[`symbol`] == symbol && item["fundingRate"] != nil &&
//					item[`timestamp`] != nil {
//					fundingRate, err = item["fundingRate"].(json.Number).Float64()
//					updateTime, err := time.Parse(time.RFC3339, item[`fundingTimestamp`].(string))
//					if err == nil {
//						update = updateTime.Unix()
//					}
//				}
//			}
//		}
//	}
//	return
//}
//
//// GetOrderBook
//func _(key, secret, symbol string) (result bool, bid, ask *model.Tick) {
//	postData := make(map[string]interface{})
//	postData[`symbol`] = model.GetDialectPerp(model.Bitmex, symbol)
//	postData[`depth`] = `1`
//	response := SignedRequestBitmex(key, secret, `GET`, `/orderBook/L2`, postData)
//	bookJson, err := util.NewJSON(response)
//	if err == nil && bookJson != nil {
//		bidAsks, err := bookJson.Array()
//		if err == nil {
//			for _, value := range bidAsks {
//				tick := parseTick(value.(map[string]interface{}))
//				if tick == nil {
//					continue
//				}
//				if tick.Side == model.OrderSideSell && (ask == nil || tick.Price < ask.Price) {
//					ask = tick
//				}
//				if tick.Side == model.OrderSideBuy && (bid == nil || tick.Price > bid.Price) {
//					bid = tick
//				}
//			}
//		}
//	}
//	if bid != nil && ask != nil {
//		result = true
//	}
//	return result, bid, ask
//}
//
//// GetWalletHistoryBitmex
//func _(key, secret string) (amount float64, transfer string) {
//	postData := make(map[string]interface{})
//	response := SignedRequestBitmex(key, secret, `GET`, `/user/walletHistory`, postData)
//	util.Notice(`bitmex wallet: ` + string(response))
//	walletJson, err := util.NewJSON(response)
//	if err == nil {
//		items := walletJson.MustArray()
//		for _, item := range items {
//			data := item.(map[string]interface{})
//			if data == nil || data[`transactType`] == nil {
//				continue
//			}
//			if data[`transactType`] == `UnrealisedPNL` && amount == 0 {
//				amount, _ = data[`walletBalance`].(json.Number).Float64()
//			}
//			if data[`transactType`] == `Withdrawal` || data[`transactType`] == `Deposit` {
//				temp, _ := data[`amount`].(json.Number).Float64()
//				temp = temp / 100000000
//				transfer += fmt.Sprintf("%s %s %f\n", data[`timestamp`].(string),
//					data[`transactType`], temp)
//			}
//		}
//	}
//	amount = amount / 100000000
//	return
//}

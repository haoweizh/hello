package deprecated

//
//import (
//	"crypto/hmac"
//	"crypto/sha256"
//	"encoding/hex"
//	"encoding/json"
//	"fmt"
//	"github.com/bitly/go-simplejson"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"net/http"
//	"net/url"
//	"sort"
//	"strconv"
//	"strings"
//	"sync"
//	"time"
//)
//
//const restFtx = `https://ftx.com/api`
//const wsFtx = `wss://ftx.com/ws`
//const wsStepFtx = 50
//
//var channelMaintainingFtx = false
//var ftxSymbolConnection sync.Map
//
//func maintainChannelFtx(subscribes []interface{}) {
//	if !channelMaintainingFtx {
//		channelMaintainingFtx = true
//		go func() {
//			for {
//				time.Sleep(time.Second * 10)
//				value, _ := model.AppEnvironment.ConnTick.Load(model.Ftx)
//				if value == nil {
//					return
//				}
//				connections := value.(map[*model.WSConn]bool)
//				if sendErr := model.SendToConnections(model.Ftx, connections, []byte(`{"op":"ping"}`)); sendErr != nil {
//					util.SocketInfo("ftx server ping client error " + sendErr.Error())
//				}
//			}
//		}()
//		for {
//			time.Sleep(time.Minute * 5)
//			needReset := false
//			for _, value := range subscribes {
//				subscribe := value.([]string)
//				success, marketType, coin := model.GetCoinFromDialect(model.Ftx, subscribe[1])
//				if !success {
//					continue
//				}
//				standardSymbol := coin + model.UniStandardTail[marketType]
//				_, bidAsk := model.AppEnvironment.GetBidAsk(model.Ftx, standardSymbol)
//				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 120000 {
//					conn, ok := ftxSymbolConnection.Load(standardSymbol)
//					if conn != nil && ok {
//						cmdUnsub := fmt.Sprintf(`{"op": "unsubscribe", "channel": "%s", "market": "%s"}`,
//							subscribe[0], subscribe[1])
//						if bidAsk != nil {
//							util.Notice(fmt.Sprintf(`GetDialectFromStandard timeout %s %d %d`,
//								standardSymbol, time.Now().UnixMilli()-int64(bidAsk.Ts), bidAsk.Ts))
//						}
//						if err := model.SendToConnection(model.Ftx, conn.(*model.WSConn), []byte(cmdUnsub)); err != nil {
//							util.SocketInfo("ftx can not resubscribe " + err.Error())
//						}
//						time.Sleep(time.Second * 3)
//						cmdSub := fmt.Sprintf(`{"op": "subscribe", "channel": "%s", "market": "%s"}`,
//							subscribe[0], subscribe[1])
//						if err := model.SendToConnection(model.Ftx, conn.(*model.WSConn), []byte(cmdSub)); err != nil {
//							util.SocketInfo("ftx can not resubscribe " + err.Error())
//						}
//						util.Notice(`send unsubscribe-subscribe %s %s %s`, model.Ftx, cmdUnsub, cmdSub)
//					} else {
//						util.Notice(`ftx can not get connection for %s`, subscribe[1])
//					}
//				}
//				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 180000 {
//					util.Notice(fmt.Sprintf(`fail to get bidask ftx %s`, subscribe[1]))
//					api.SetRequireReset(model.Ftx)
//					needReset = true
//					break
//				}
//			}
//			if !needReset {
//				util.Info(`no need reset %s`, model.Ftx)
//			}
//		}
//	}
//}
//
//var subscribeHandlerFtx = func(market string, connection *model.WSConn, subscribes []interface{}) (err error) {
//	account := model.AppConfig.GetAccounts(model.Ftx)[0]
//	ts := time.Now().UnixNano() / int64(time.Millisecond)
//	toBeSign := fmt.Sprintf(`%dwebsocket_login`, ts)
//	hash := hmac.New(sha256.New, []byte(account.Secret))
//	hash.Write([]byte(toBeSign))
//	sign := hex.EncodeToString(hash.Sum(nil))
//	authCmd := fmt.Sprintf(`{"op":"login","args":{"key":"%s","sign":"%s","time":%d}}`, account.Key, sign, ts)
//	if err = model.SendToConnection(model.Ftx, connection, []byte(authCmd)); err != nil {
//		util.SocketInfo("ftx can not auth " + err.Error())
//	}
//	if err = model.SendToConnection(model.Ftx, connection, []byte(`{"op": "subscribe", "channel": "fills"}`)); err != nil {
//		util.SocketInfo("ftx can not subscribe fill " + err.Error())
//	}
//	for i := 0; i < len(subscribes); i++ {
//		cmdSubscribe := subscribes[i].([]string)
//		success, marketType, coin := model.GetCoinFromDialect(model.Ftx, cmdSubscribe[1])
//		if !success {
//			continue
//		}
//		ftxSymbolConnection.Store(coin+model.UniStandardTail[marketType], connection)
//		subCmd := fmt.Sprintf(`{"op": "subscribe", "channel": "%s", "market": "%s"}`,
//			cmdSubscribe[0], cmdSubscribe[1])
//		if err = model.SendToConnection(model.Ftx, connection, []byte(subCmd)); err != nil {
//			util.SocketInfo("ftx can not subscribe " + err.Error())
//			return err
//		}
//		util.Notice(`send subscribe ` + subCmd)
//	}
//	return err
//}
//var wsHandlerFtx = func(market string, conn *model.WSConn, event []byte) {
//	responseJson, err := util.NewJSON(event)
//	if err != nil {
//		util.SocketInfo(`fail to unmarshal json ` + err.Error())
//		return
//	}
//	if responseJson == nil {
//		return
//	}
//	msgType := responseJson.Get(`channel`).MustString()
//	if msgType == `orderbook` {
//		handleDepthFtx(model.AppEnvironment, responseJson)
//	} else if msgType == `ticker` {
//		handleTickerFtx(model.AppEnvironment, responseJson)
//	}
//}
//
//func WsTickServeFtx(environment *model.Environment, market string) (socketMap map[*model.WSConn]bool, msgChans []chan struct{}, err error) {
//	subscribes := api.GetWSSubscribes(model.Ftx, []string{model.SubscribeDepth, model.SubscribeTicker})
//	subscribes = append(subscribes, api.GetWSSubscribe(market, `USDT_USDT`, model.SubscribeDepth))
//	subscribes = append(subscribes, api.GetWSSubscribe(market, `USDT_USDT`, model.SubscribeTicker))
//	socketMap, msgChans, err = model.WebSocketClient(market, wsFtx, subscribes, subscribeHandlerFtx, wsHandlerFtx, wsStepFtx)
//	environment.ConnTick.Store(market, socketMap)
//	environment.MsgChanTick.Store(market, msgChans)
//	go maintainChannelFtx(subscribes)
//	return
//}
//
//func handleTickerFtx(environment *model.Environment, response *simplejson.Json) {
//	if response == nil {
//		return
//	}
//	success, marketType, coin := model.GetCoinFromDialect(model.Ftx, response.Get("market").MustString())
//	if !success {
//		return
//	}
//	standardSymbol := coin + model.UniStandardTail[marketType]
//	dataType := response.Get(`type`).MustString()
//	data := response.Get(`data`)
//	if data == nil || data.Get(`bid`) == nil || data.Get(`ask`) == nil || data.Get(`bidSize`) == nil ||
//		data.Get(`askSize`) == nil {
//		return
//	}
//	bidAsk := &model.BidAsk{}
//	ts := data.Get(`time`).MustFloat64()
//	bidAsk.Ts = int(ts * 1000)
//	bidAsk.TsReceived = int(util.GetNowUnixMillion())
//	if dataType == `update` {
//		bidAsk.Bids = []model.Tick{{Price: data.Get(`bid`).MustFloat64(), Amount: data.Get(`bidSize`).MustFloat64(),
//			Market: model.Ftx, Symbol: standardSymbol}}
//		bidAsk.Asks = []model.Tick{{Price: data.Get(`ask`).MustFloat64(), Amount: data.Get(`askSize`).MustFloat64(),
//			Market: model.Ftx, Symbol: standardSymbol}}
//	}
//	//for function, handler := range model.GetFunctions(model.Ftx, standardSymbol) {
//	//	if handler != nil {
//	//		setting := model.GetSetting(function, model.Ftx, standardSymbol)
//	//		if setting != nil && setting.Function == model.FunctionCross {
//	//			getUsdTick, usdtBidAsk := environment.GetBidAsk(`USDT_USDT`, model.Ftx)
//	//			if getUsdTick && bidAsk.Asks.Len() > 0 && bidAsk.Bids.Len() > 0 {
//	//				bidAsk.Asks[0].Price /= usdtBidAsk.Asks[0].Price
//	//				bidAsk.Bids[0].Price /= usdtBidAsk.Asks[0].Price
//	//				break
//	//			}
//	//		}
//	//	}
//	//}
//	if environment.SetBidAsk(model.Ftx, standardSymbol, bidAsk) {
//		funcHandlers := api.GetFunctions(model.Ftx, standardSymbol)
//		if funcHandlers != nil {
//			funcHandlers.Range(func(function, value interface{}) bool {
//				setting := api.GetSetting(function.(string), model.Ftx, standardSymbol)
//				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
//					go value.(model.CarryHandler)(setting, bidAsk)
//				}
//				return true
//			})
//		}
//	}
//}
//
//func handleDepthFtx(environment *model.Environment, response *simplejson.Json) {
//	if response == nil {
//		return
//	}
//	success, marketType, coin := model.GetCoinFromDialect(model.Ftx, response.Get("market").MustString())
//	if !success {
//		return
//	}
//	standardSymbol := coin + model.UniStandardTail[marketType]
//	dataType := response.Get(`type`).MustString()
//	data := response.Get(`data`)
//	if data.Interface() != nil && (dataType == `partial` || dataType == `update`) {
//		// ftx服务器传回来的数据是保留小数点后6位的，所以别担心精度不够
//		bidAsk := &model.BidAsk{Ts: int(data.Get(`time`).MustFloat64() * 1000),
//			TsReceived: int(util.GetNowUnixMillion()), Bids: make([]model.Tick, 0), Asks: make([]model.Tick, 0)}
//		bids := data.Get(`bids`).MustArray()
//		asks := data.Get(`asks`).MustArray()
//		if dataType == `partial` {
//			for _, item := range bids {
//				price, _ := item.([]interface{})[0].(json.Number).Float64()
//				size, _ := item.([]interface{})[1].(json.Number).Float64()
//				bidAsk.Bids = append(bidAsk.Bids, model.Tick{Price: price, Amount: size, Market: model.Ftx, Symbol: standardSymbol})
//			}
//			for _, item := range asks {
//				price, _ := item.([]interface{})[0].(json.Number).Float64()
//				size, _ := item.([]interface{})[1].(json.Number).Float64()
//				bidAsk.Asks = append(bidAsk.Asks, model.Tick{Price: price, Amount: size, Market: model.Ftx, Symbol: standardSymbol})
//			}
//		} else {
//			_, oldBidAsk := environment.GetBidAsk(model.Ftx, standardSymbol)
//			if oldBidAsk == nil {
//				util.SocketInfo(fmt.Sprintf(`fatal: can not have old bidask %s %s`, model.Ftx, standardSymbol))
//				oldBidAsk = &model.BidAsk{Ts: int(data.Get(`time`).MustFloat64() * 1000),
//					TsReceived: int(util.GetNowUnixMillion()), Bids: make([]model.Tick, 0), Asks: make([]model.Tick, 0)}
//			}
//			priceAmountBid := make(map[float64]*model.Tick)
//			priceAmountAsk := make(map[float64]*model.Tick)
//			for _, bid := range oldBidAsk.Bids {
//				priceAmountBid[bid.Price] = &model.Tick{Price: bid.Price, Amount: bid.Amount, Market: model.Ftx, Symbol: standardSymbol}
//			}
//			for _, item := range bids {
//				price, _ := item.([]interface{})[0].(json.Number).Float64()
//				size, _ := item.([]interface{})[1].(json.Number).Float64()
//				priceAmountBid[price] = &model.Tick{Price: price, Amount: size, Market: model.Ftx, Symbol: standardSymbol}
//			}
//			for _, ask := range oldBidAsk.Asks {
//				//priceAmountAsk[ask.Price] = &ask
//				priceAmountAsk[ask.Price] = &model.Tick{Price: ask.Price, Amount: ask.Amount, Market: model.Ftx, Symbol: standardSymbol}
//			}
//			for _, item := range asks {
//				price, _ := item.([]interface{})[0].(json.Number).Float64()
//				size, _ := item.([]interface{})[1].(json.Number).Float64()
//				priceAmountAsk[price] = &model.Tick{Price: price, Amount: size, Market: model.Ftx, Symbol: standardSymbol}
//			}
//			for _, tick := range priceAmountBid {
//				if tick.Amount > 0 {
//					bidAsk.Bids = append(bidAsk.Bids, *tick)
//				}
//			}
//			for _, tick := range priceAmountAsk {
//				if tick.Amount > 0 {
//					bidAsk.Asks = append(bidAsk.Asks, *tick)
//				}
//			}
//		}
//		sort.Sort(bidAsk.Asks)
//		sort.Sort(sort.Reverse(bidAsk.Bids))
//		//for function, handler := range model.GetFunctions(model.Ftx, standardSymbol) {
//		//	if handler != nil {
//		//		setting := model.GetSetting(function, model.Ftx, standardSymbol)
//		//		if setting != nil && setting.Function == model.FunctionCross {
//		//			getUsdTick, usdtBidAsk := environment.GetBidAsk(`USDT_USDT`, model.Ftx)
//		//			if getUsdTick && bidAsk.Asks.Len() > 0 && bidAsk.Bids.Len() > 0 {
//		//				bidAsk.Asks[0].Price /= usdtBidAsk.Asks[0].Price
//		//				bidAsk.Bids[0].Price /= usdtBidAsk.Asks[0].Price
//		//				break
//		//			}
//		//		}
//		//	}
//		//}
//		if environment.SetBidAsk(model.Ftx, standardSymbol, bidAsk) {
//			funcHandlers := api.GetFunctions(model.Ftx, standardSymbol)
//			if funcHandlers != nil {
//				funcHandlers.Range(func(function, value interface{}) bool {
//					setting := api.GetSetting(function.(string), model.Ftx, standardSymbol)
//					if setting != nil && value != nil && value.(model.CarryHandler) != nil {
//						go value.(model.CarryHandler)(setting, bidAsk)
//					}
//					return true
//				})
//			}
//		}
//	}
//}
//
//func getCandlesFtx(account *model.Account, symbol string, start, end time.Time, slotSeconds int) (
//	candles []*model.Candle) {
//	candles = make([]*model.Candle, 0)
//	param := make(map[string]interface{})
//	param[`resolution`] = fmt.Sprintf(`%d`, slotSeconds)
//	//param[`limit`] = fmt.Sprintf(`%d`, count)
//	param[`start_time`] = fmt.Sprintf(`%d`, start.Unix())
//	param[`end_time`] = fmt.Sprintf(`%d`, end.Unix())
//	_, _, _, dialectSymbol := model.GetFromStandard(model.Ftx, symbol)
//	response, _ := SignedRequestFtx(account.Key, account.Secret, `GET`,
//		fmt.Sprintf(`/markets/%s/candles`, dialectSymbol), param, nil)
//	candleJson, err := util.NewJSON(response)
//	if err == nil {
//		candleJsons := candleJson.Get(`result`).MustArray()
//		for _, value := range candleJsons {
//			item := value.(map[string]interface{})
//			candle := &model.Candle{Market: model.Ftx, Symbol: symbol, Seconds: slotSeconds}
//			if item[`open`] != nil {
//				candle.PriceOpen, _ = item[`open`].(json.Number).Float64()
//			}
//			if item[`close`] != nil {
//				candle.PriceClose, _ = item[`close`].(json.Number).Float64()
//			}
//			if item[`high`] != nil {
//				candle.PriceHigh, _ = item[`high`].(json.Number).Float64()
//			}
//			if item[`low`] != nil {
//				candle.PriceLow, _ = item[`low`].(json.Number).Float64()
//			}
//			if item[`startTime`] != nil {
//				candle.Begin, _ = time.Parse(time.RFC3339, item[`startTime`].(string))
//				candles = append(candles, candle)
//			}
//		}
//	}
//	return
//}
//
//func parseBalanceFtx(key string, data map[string]interface{}) (balance *model.Balance) {
//	if data[`coin`] == nil {
//		return nil
//	}
//	coin := data[`coin`].(string)
//	balance = &model.Balance{
//		Market:      model.Ftx,
//		Coin:        coin,
//		ID:          model.Ftx + `_` + coin + `_` + util.GetNow().Format(time.RFC3339)[0:10],
//		BalanceTime: util.GetNow(),
//		AccountId:   key}
//	//if data[`availableWithoutBorrow`] != nil {
//	//	balance.Available, _ = data[`availableWithoutBorrow`].(json.Number).Float64()
//	//}
//	if data[`total`] != nil { // 持仓
//		balance.Amount, _ = data[`total`].(json.Number).Float64()
//	}
//	if data[`usdValue`] != nil {
//		balance.UsdValue, _ = data[`usdValue`].(json.Number).Float64()
//	}
//	if data[`spotBorrow`] != nil { //已借数量
//		balance.Borrow, _ = data[`spotBorrow`].(json.Number).Float64()
//	}
//	if data[`free`] != nil { //可借+持仓-已挂的卖单=现在总的可下卖单数量（千万别用这个作为持仓！）
//		balance.AvailableWithBorrow, _ = data[`free`].(json.Number).Float64()
//	}
//	return balance
//}
//
//func parseTransactionFtx(key string, data map[string]interface{}, action float64) (balance *model.Balance) {
//	if data[`id`] == nil {
//		return nil
//	}
//	balance = &model.Balance{
//		Market:    model.Ftx,
//		ID:        model.Ftx + `_` + data[`id`].(json.Number).String(),
//		Action:    action,
//		AccountId: key}
//	if data[`notes`] != nil {
//		balance.Notes = data[`notes`].(string)
//	}
//	if data[`coin`] != nil {
//		balance.Coin = data[`coin`].(string)
//	}
//	if data[`fee`] != nil {
//		balance.Fee = data[`fee`].(json.Number).String()
//	}
//	if data[`size`] != nil {
//		balance.Amount, _ = data[`size`].(json.Number).Float64()
//	}
//	if data[`time`] != nil {
//		balance.BalanceTime, _ = time.Parse(time.RFC3339Nano, data[`time`].(string))
//	}
//	if data[`status`] != nil {
//		balance.Status, _ = data[`status`].(string)
//	}
//	if data[`address`] != nil {
//		if action == 1 {
//			address := data[`address`].(map[string]interface{})
//			if address != nil {
//				balance.Address = address[`address`].(string)
//			}
//		} else if action == -1 {
//			balance.Address = data[`address`].(string)
//		}
//	}
//	if data[`txid`] != nil {
//		balance.TransactionId, _ = data[`txid`].(string)
//	}
//	return balance
//}
//
//func getTransferFtx(key, secret string) (balances []*model.Balance) {
//	balances = make([]*model.Balance, 0)
//	response, _ := SignedRequestFtx(key, secret, `GET`, `/wallet/deposits`, nil, nil)
//	deposit, err := util.NewJSON(response)
//	if err == nil && deposit != nil {
//		for _, item := range deposit.Get(`result`).MustArray() {
//			balance := parseTransactionFtx(key, item.(map[string]interface{}), 1)
//			if balance != nil {
//				balances = append(balances, balance)
//			}
//		}
//	}
//	response, _ = SignedRequestFtx(key, secret, `GET`, `/wallet/withdrawals`, nil, nil)
//	withdraw, withdrawErr := util.NewJSON(response)
//	if withdrawErr == nil && withdraw != nil {
//		for _, item := range withdraw.Get(`result`).MustArray() {
//			balance := parseTransactionFtx(key, item.(map[string]interface{}), -1)
//			if balance != nil {
//				balances = append(balances, balance)
//			}
//		}
//	}
//	return
//}
//
//func getBalanceFtx(key, secret string) (success bool, balances []*model.Balance, totalInUsd float64) {
//	balances = make([]*model.Balance, 0)
//	response, _ := SignedRequestFtx(key, secret, `GET`, `/wallet/balances`, nil, nil)
//	balanceJson, err := util.NewJSON(response)
//	if err != nil || balanceJson == nil || balanceJson.Get(`success`).MustBool() != true {
//		util.SocketInfo(`fail to get ftx balance`)
//		time.Sleep(time.Minute * 5)
//		return getBalanceFtx(key, secret)
//	}
//	success = balanceJson.Get(`success`).MustBool()
//	for _, item := range balanceJson.Get(`result`).MustArray() {
//		balance := parseBalanceFtx(key, item.(map[string]interface{}))
//		if balance != nil {
//			balances = append(balances, balance)
//			totalInUsd += balance.UsdValue
//		}
//	}
//	return success, balances, totalInUsd
//}
//
//func cancelOrdersFtx(key, secret, symbol string) (result bool) {
//	postData := make(map[string]interface{})
//	postData[`market`] = symbol
//	response, _ := SignedRequestFtx(key, secret, http.MethodDelete, `/orders`, nil, postData)
//	util.SocketInfo(fmt.Sprintf(`[api cancelOrdersFtx]%s %s`, symbol, string(response)))
//	orderJson, err := util.NewJSON(response)
//	if err == nil {
//		return orderJson.Get(`success`).MustBool()
//	}
//	return false
//}
//
//// {"success":false,"error":"Order already closed"}
//// {"success":true,"result":"Order cancelled"}
//func cancelOrderFtx(key, secret, orderType, orderId string) (result bool) {
//	path := `/orders/%s`
//	if orderType == model.OrderTypeStop || orderType == model.OrderTypeTrailStop {
//		path = `/conditional_orders/%s`
//	}
//	response, _ := SignedRequestFtx(key, secret, `DELETE`, fmt.Sprintf(path, orderId), nil, nil)
//	util.Notice(fmt.Sprintf(`cancel ftx %s %s: %s`, orderType, orderId, string(response)))
//	orderJson, err := util.NewJSON(response)
//	if err == nil {
//		if strings.Contains(orderJson.Get(`error`).MustString(), `already closed`) {
//			return true
//		}
//		return orderJson.Get(`success`).MustBool()
//	}
//	return false
//}
//
//func queryTriggerOrderId(key, secret, id string) (orderId string) {
//	response, _ := SignedRequestFtx(key, secret, `GET`,
//		fmt.Sprintf(`/conditional_orders/%s/triggers`, id), nil, nil)
//	orderJson, err := util.NewJSON(response)
//	if err == nil && orderJson.Get(`success`).MustBool() {
//		orders := orderJson.Get(`result`).MustArray()
//		for _, item := range orders {
//			data := item.(map[string]interface{})
//			if data[`orderId`] != nil {
//				orderNumber, _ := data[`orderId`].(json.Number).Int64()
//				return fmt.Sprintf(`%d`, orderNumber)
//			}
//		}
//	}
//	return
//}
//
//// 查询未成交的触发单
//func queryOrdersFtx(key, secret, symbol string, isTrigger bool) (orders []*model.Order) {
//	param := make(map[string]interface{})
//	param[`market`] = symbol
//	path := `/orders`
//	if isTrigger {
//		path = `/conditional_orders`
//	}
//	response, _ := SignedRequestFtx(key, secret, `GET`, path, param, nil)
//	orderJson, err := util.NewJSON(response)
//	if err == nil {
//		result := orderJson.Get(`result`)
//		if result != nil && orderJson.Get(`success`).MustBool() {
//			orders = make([]*model.Order, 0)
//			orderArray := result.MustArray()
//			for _, value := range orderArray {
//				item := value.(map[string]interface{})
//				order := &model.Order{Market: model.Ftx}
//				parseOrderFtx(order, item)
//				orders = append(orders, order)
//			}
//		}
//	}
//	return orders
//}
//
//func queryTriggerOrderFtx(key, secret, symbol, triggerId string) (status string) {
//	orders := queryOrdersFtx(key, secret, symbol, true)
//	if orders == nil || len(orders) == 0 {
//		return model.CarryStatusFail
//	}
//	for _, order := range orders {
//		if order != nil && order.OrderId == triggerId {
//			return model.CarryStatusWorking
//		}
//	}
//	return model.CarryStatusFail
//}
//
//func queryOrderFtx(key, secret, orderId string) (order *model.Order) {
//	response, _ := SignedRequestFtx(key, secret, `GET`, fmt.Sprintf(`/orders/%s`, orderId), nil, nil)
//	orderJson, err := util.NewJSON(response)
//	if err == nil && orderJson.Get(`success`).MustBool() {
//		data, _ := orderJson.Get(`result`).Map()
//		order = &model.Order{Market: model.Ftx}
//		parseOrderFtx(order, data)
//	}
//	return
//}
//
//func getPositionsFtx(key, secret string) (success bool, positions []*api.Position, posBalance float64) {
//	response, _ := SignedRequestFtx(key, secret, `GET`, `/positions`, nil, nil)
//	positionJson, err := util.NewJSON(response)
//	if err != nil || positionJson == nil || positionJson.Get(`success`).MustBool() != true {
//		util.SocketInfo(`fail to refresh account ftx`)
//		time.Sleep(time.Minute * 5)
//		return getPositionsFtx(key, secret)
//	}
//	success = positionJson.Get(`success`).MustBool()
//	positionJson = positionJson.Get(`result`)
//	positions = make([]*api.Position, 0)
//	if positionJson != nil {
//		data := positionJson.MustArray()
//		for _, item := range data {
//			position := &api.Position{Market: model.Ftx, Ts: util.GetNowUnixMillion()}
//			parsePositionFtx(position, item.(map[string]interface{}))
//			if position.Holding != 0 {
//				positions = append(positions, position)
//			}
//		}
//	}
//	return success, positions, 0
//}
//
////func getAccountFtx(key, secret string, accounts *model.Accounts) (success bool) {
////	postData := make(map[string]interface{})
////	response := SignedRequestFtx(key, secret, `GET`, `/positions`, nil, postData)
////	positionJson, err := util.NewJSON(response)
////	if err != nil || positionJson == nil || positionJson.Get(`success`).MustBool() != true {
////		util.SocketInfo(`fail to refresh account ftx`)
////		time.Sleep(time.Minute * 5)
////		return getAccountFtx(key, secret, accounts)
////	}
////	positionJson = positionJson.Get(`result`)
////	if positionJson != nil {
////		data := positionJson.MustArray()
////		for _, item := range data {
////			account := &model.Account{Market: model.Ftx, Ts: util.GetNowUnixMillion()}
////			parseAccountFtx(account, item.(map[string]interface{}))
////			accounts.SetAccount(model.Ftx, account.Currency, account)
////		}
////	}
////	return true
////}
//
//// getMarketInfoFtx
//func _(key, secret, symbol string) (borrowAble float64) {
//	if !strings.Contains(symbol, `/`) {
//		return 0
//	}
//	coin := strings.Split(symbol, `/`)[0]
//	param := map[string]interface{}{`market`: symbol}
//	response, _ := SignedRequestFtx(key, secret, http.MethodGet, `/spot_margin/market_info`, param, nil)
//	borrowJson, err := util.NewJSON(response)
//	if err == nil {
//		results := borrowJson.Get(`result`).MustArray()
//		for _, result := range results {
//			value := result.(map[string]interface{})
//			if value[`coin`] == nil {
//				continue
//			}
//			if value[`coin`].(string) == coin {
//				if value[`free`] == nil {
//					return 0
//				}
//				borrowAble, _ = value[`free`].(json.Number).Float64()
//				return borrowAble
//			}
//		}
//	}
//	return 0
//}
//
//func getMarketsFtx(key, secret string) (marketInfos map[string]*model.MarketInfo) {
//	response, _ := SignedRequestFtx(key, secret, `GET`,
//		`/markets`, nil, nil)
//	marketInfos = make(map[string]*model.MarketInfo)
//	rateJson, err := util.NewJSON(response)
//	response, _ = SignedRequestFtx(key, secret, `GET`,
//		`/spot_margin/borrow_rates`, nil, nil)
//	borrowJson, _ := util.NewJSON(response)
//	if err != nil || rateJson.Get(`result`) == nil || borrowJson.Get(`result`) == nil ||
//		rateJson.Get(`success`).MustBool() == false || borrowJson.Get(`success`).MustBool() == false {
//		time.Sleep(time.Minute * 5)
//		return getMarketsFtx(key, secret)
//	} else {
//		items, _ := rateJson.Get(`result`).Array()
//		borrows, _ := borrowJson.Get(`result`).Array()
//		canBorrows := make(map[string]bool)
//		for _, borrow := range borrows {
//			value := borrow.(map[string]interface{})
//			canBorrows[value[`coin`].(string)] = true
//		}
//		for _, item := range items {
//			value := item.(map[string]interface{})
//			marketInfo := &model.MarketInfo{Market: model.Ftx, SizeMax: 90000000}
//			if value[`name`] != nil {
//				success, marketType, coin := model.GetCoinFromDialect(model.Ftx, value[`name`].(string))
//				if !success {
//					continue
//				}
//				marketInfo.Symbol = coin + model.UniStandardTail[marketType]
//			} else {
//				continue
//			}
//			if value[`baseCurrency`] != nil && value[`type`] != nil && value[`type`].(string) == `spot` &&
//				canBorrows[value[`baseCurrency`].(string)] {
//				marketInfo.CanBorrow = true
//			}
//			if value[`priceIncrement`] != nil {
//				marketInfo.PriceIncrement, _ = value[`priceIncrement`].(json.Number).Float64()
//			}
//			if value[`sizeIncrement`] != nil {
//				marketInfo.SizeIncrement, _ = value[`sizeIncrement`].(json.Number).Float64()
//			}
//			if value[`minProvideSize`] != nil {
//				marketInfo.SizeMin, _ = value[`minProvideSize`].(json.Number).Float64()
//				marketInfo.SizeMin = math.Max(marketInfo.SizeMin, marketInfo.SizeIncrement)
//			}
//			if value[`volumeUsd24h`] != nil {
//				marketInfo.TradeAmount, _ = value[`volumeUsd24h`].(json.Number).Float64()
//			}
//			marketInfos[marketInfo.Symbol] = marketInfo
//		}
//	}
//	return
//}
//
//func GetFundingRatesFtx(key, secret, symbol string) (fundingRate *model.FundingRate) {
//	_, _, _, dialectSymbol := model.GetFromStandard(model.Ftx, symbol)
//	response, _ := SignedRequestFtx(key, secret, `GET`,
//		fmt.Sprintf(`/futures/%s/stats`, dialectSymbol), nil, nil)
//	rateJson, err := util.NewJSON(response)
//	if err == nil && rateJson.Get(`result`) != nil {
//		fundingRate = &model.FundingRate{
//			Rate:       rateJson.GetPath(`result`, `nextFundingRate`).MustFloat64() * 5,
//			UpdateTime: util.GetNow(),
//		}
//		expireTime, _ := time.ParseInLocation(time.RFC3339, rateJson.GetPath(`result`, `nextFundingTime`).MustString(), time.UTC)
//		fundingRate.ExpireTime = expireTime.Unix()
//	}
//	return fundingRate
//}
//
//func parsePositionFtx(position *api.Position, item map[string]interface{}) {
//	if item[`entryPrice`] != nil {
//		position.EntryPrice, _ = item[`entryPrice`].(json.Number).Float64()
//	}
//	if item[`estimatedLiquidationPrice`] != nil {
//		position.LiquidationPrice, _ = item[`estimatedLiquidationPrice`].(json.Number).Float64()
//	}
//	if item[`future`] != nil {
//		_, _, coin := model.GetCoinFromDialect(model.Ftx, item[`future`].(string))
//		position.Currency = coin + model.UniStandardTail[model.MarketTypePerp]
//	}
//	if item[`netSize`] != nil {
//		position.Holding, _ = item[`netSize`].(json.Number).Float64()
//		//account.Free = account.Free * account.EntryPrice
//	}
//	if item[`realizedPnl`] != nil {
//		position.ProfitReal, _ = item[`realizedPnl`].(json.Number).Float64()
//	}
//	if item[`side`] != nil {
//		position.Direction = item[`side`].(string)
//	}
//	if item[`bust_price`] != nil {
//		position.BankruptcyPrice, _ = strconv.ParseFloat(item[`bust_price`].(string), 64)
//	}
//	if item[`position_margin`] != nil {
//		position.Margin, _ = strconv.ParseFloat(item[`position_margin`].(string), 64)
//	}
//	if item[`unrealizedPnl`] != nil {
//		position.ProfitUnreal, _ = item[`unrealizedPnl`].(json.Number).Float64()
//	}
//}
//
//// remainingSize	number	31431.0
//// reduceOnly	boolean	false
//// ioc	boolean	false
//// postOnly	boolean	false
//// orderPrice	number	null	price of the order sent when this stop loss triggered
//// retryUntilFilled	boolean	false	Whether or not to keep re-triggering until filled
//// orderType	string	market	Values are market and limit
//func parseOrderFtx(order *model.Order, item map[string]interface{}) {
//	if order == nil || item == nil {
//		return
//	}
//	if item[`createdAt`] != nil {
//		order.OrderTime, _ = time.Parse(time.RFC3339, item[`createdAt`].(string))
//	}
//	if item[`filledSize`] != nil {
//		order.DealAmount, _ = item[`filledSize`].(json.Number).Float64()
//	}
//	if item[`id`] != nil {
//		order.OrderId = item[`id`].(json.Number).String()
//	}
//	if item[`market`] != nil {
//		success, marketType, coin := model.GetCoinFromDialect(model.Ftx, item[`market`].(string))
//		if success {
//			order.Symbol = coin + model.UniStandardTail[marketType]
//		}
//	}
//	if item[`price`] != nil {
//		order.Price, _ = item[`price`].(json.Number).Float64()
//	}
//	if item[`avgFillPrice`] != nil {
//		order.DealPrice, _ = item[`avgFillPrice`].(json.Number).Float64()
//	}
//	if item[`side`] != nil {
//		order.OrderSide = strings.ToLower(item[`side`].(string))
//	}
//	if item[`size`] != nil {
//		order.Amount, _ = item[`size`].(json.Number).Float64()
//	}
//	if item[`type`] != nil {
//		order.OrderType = strings.ToLower(item[`type`].(string))
//		if order.OrderType == `limit` {
//			order.OrderType = model.OrderTypeLimit
//		} else if order.OrderType == `stop` {
//			order.OrderType = model.OrderTypeStop
//		} else if order.OrderType == `market` {
//			order.OrderType = model.OrderTypeMarket
//		}
//	}
//	if item[`status`] != nil {
//		order.Status = model.GetOrderStatus(model.Ftx, item[`status`].(string))
//		if order.DealAmount == 0 && order.Status == model.CarryStatusSuccess {
//			order.Status = model.CarryStatusFail
//		}
//	}
//	if item[`orderPrice`] != nil {
//		order.Price, _ = item[`orderPrice`].(json.Number).Float64()
//	}
//	if item[`triggeredAt`] != nil {
//		order.OrderUpdateTime, _ = time.Parse(time.RFC3339, item[`triggeredAt`].(string))
//	}
//	if order.Status != model.CarryStatusSuccess && order.Status != model.CarryStatusFail {
//		order.Status = model.CarryStatusWorking
//	}
//	if order.DealAmount == 0 || order.DealPrice == 0 {
//		order.DealPrice = order.Price
//	}
//	//order.Amount = order.Amount * order.Price
//	//order.DealAmount = order.DealAmount * order.Price
//	order.UnfilledQuantity = order.Amount - order.DealAmount
//	return
//}
//
//// orderType: "limit", "market", "stop", "trailingStop", "takeProfit"
//func placeOrderFtx(order *model.Order, key, secret, orderSide, orderType, orderParam, symbol string, orderPrice, triggerPrice, amount float64) {
//	uri := `/orders`
//	param := make(map[string]interface{})
//	param[`market`] = symbol
//	postData := make(map[string]interface{})
//	postData[`market`] = symbol
//	postData[`side`] = orderSide
//	postData[`size`] = amount
//	postData[`type`] = orderType
//	postData[`ioc`] = false
//	postData[`reduceOnly`] = false
//	if orderType == model.OrderTypeLimit || orderType == model.OrderTypeMarket {
//		postData[`price`] = orderPrice
//		if orderParam == model.PostOnly {
//			postData[`postOnly`] = true
//		}
//	} else if orderType == `stop` || orderType == `trailingStop` || orderType == `takeProfit` {
//		uri = `/conditional_orders`
//		postData[`triggerPrice`] = triggerPrice
//		if orderPrice > 0 {
//			postData[`orderPrice`] = orderPrice
//		}
//	}
//	response, httpErr := SignedRequestFtx(key, secret, `POST`, uri, param, postData)
//	if httpErr != nil {
//		order.ErrCode = httpErr.Error()
//		return
//	}
//	orderJson, err := util.NewJSON(response)
//	if err == nil {
//		success := orderJson.Get(`success`).MustBool()
//		if success {
//			data, _ := orderJson.Get(`result`).Map()
//			parseOrderFtx(order, data)
//		} else {
//			order.Status = model.CarryStatusFail
//			order.ErrCode = orderJson.Get(`error`).MustString()
//			order.OrderId = ``
//		}
//	}
//	return
//}
//
//func SignedRequestFtx(key, secret, method, path string, param, body map[string]interface{}) ([]byte, error) {
//	if body == nil {
//		body = make(map[string]interface{})
//	}
//	if body[`market`] != nil && len(body[`market`].(string)) > 0 {
//		_, _, _, dialectSymbol := model.GetFromStandard(model.Ftx, body[`market`].(string))
//		body[`market`] = dialectSymbol
//	}
//	u, _ := url.ParseRequestURI(restFtx)
//	u.Path += path
//	ts := fmt.Sprintf(`%d`, time.Now().UnixNano()/int64(time.Millisecond))
//	hash := hmac.New(sha256.New, []byte(secret))
//	bodyStr := string(util.JsonEncodeToByte(body))
//	q := u.Query()
//	for k, v := range param {
//		value := fmt.Sprintf(`%#v`, v)
//		if k == `market` {
//			_, _, _, value = model.GetFromStandard(model.Ftx, value)
//		}
//		q.Set(k, value)
//	}
//	u.RawQuery = q.Encode()
//	uri := u.Path
//	if u.Query().Encode() != `` {
//		uri = fmt.Sprintf(`%s?%s`, u.Path, u.Query().Encode())
//	}
//	if method == http.MethodPost || method == http.MethodDelete {
//		hash.Write([]byte(fmt.Sprintf(`%s%s%s%s`, ts, method, uri, bodyStr)))
//	} else {
//		hash.Write([]byte(fmt.Sprintf(`%s%s%s`, ts, method, uri)))
//		bodyStr = ``
//	}
//	sign := hex.EncodeToString(hash.Sum(nil))
//	headers := map[string]string{`FTX-KEY`: key, `FTX-TS`: ts, "FTX-SIGN": sign, "Content-Type": "application/json"}
//	account := model.AppConfig.GetAccountFromKeyIndex(model.Ftx, key, -1)
//	if account != nil && len(account.FtxSubAccount) > 1 {
//		headers[`FTX-SUBACCOUNT`] = account.FtxSubAccount
//	}
//	responseBody, httpErr := util.HttpRequest(method, u.String(), bodyStr, headers, 60)
//	//if !strings.Contains(path, `balances`) && !strings.Contains(path, `positions`) {
//	util.SocketInfo(fmt.Sprintf(`ftx key %s request %s %s body %s return %s`,
//		key, u.String(), method, bodyStr, string(responseBody)))
//	//}
//	return responseBody, httpErr
//}

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const restBinance = `https://api.binance.com`
const restDataBinance = `https://data.binance.com`
const wsBinance = "wss://stream.binance.com:9443/"
const wsStepBinance = 20

var maintainingAccountConnBinance = false
var channelMaintainingBinance = false

func getMarketsBinance(account *model.Account, market, marketType string) (marketInfos map[string]*model.MarketInfo) {
	util.Notice(fmt.Sprintf("start to getMarketsBinance %s", account.Key))
	marketInfos = make(map[string]*model.MarketInfo)
	client := binance.NewClient(account.Key, account.Secret)
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		util.Notice(fmt.Sprintf("getMarketsBinance %s err: %s %v休息五分钟", account.Key, err.Error(), exchangeInfo))
		time.Sleep(time.Minute * 5)
		return getMarketsBinance(account, market, marketType)
	}
	for _, item := range exchangeInfo.Symbols {
		if item.Status != "TRADING" || item.QuoteAsset != model.DialectTail[marketType][market] ||
			(market == model.BinanceSpot && !item.IsSpotTradingAllowed) ||
			(market == model.BinanceMargin && !item.IsMarginTradingAllowed) {
			continue
		}
		marketInfo := &model.MarketInfo{Market: market, MoneyMin: 10, Name: item.BaseAsset + model.UniStandardTail[marketType]}
		for _, data := range item.Filters {
			filterType := data[`filterType`].(string)
			if filterType == `PRICE_FILTER` {
				if data[`tickSize`] != nil {
					marketInfo.PriceIncrement, _ = strconv.ParseFloat(data[`tickSize`].(string), 64)
				}
				marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
			} else if filterType == `LOT_SIZE` {
				if data[`minQty`] != nil {
					marketInfo.SizeMin, _ = strconv.ParseFloat(data[`minQty`].(string), 64)
				}
				if data[`maxQty`] != nil {
					marketInfo.SizeMax, _ = strconv.ParseFloat(data[`maxQty`].(string), 64)
				}
				if data[`stepSize`] != nil {
					marketInfo.SizeIncrement, _ = strconv.ParseFloat(data[`stepSize`].(string), 64)
				}
			} else if filterType == `MIN_NOTIONAL` {
				if data[`minNotional`] != nil {
					marketInfo.MoneyMin, _ = strconv.ParseFloat(data[`minNotional`].(string), 64)
				}
			}
		}
		marketInfos[marketInfo.Name] = marketInfo
	}
	return marketInfos
}

func WsDepthServeBinance(marketConns *model.Markets, market string) (channels []chan struct{}, err error) {
	subType := model.SubscribeTicker
	wsHandlerBinance := func(event []byte) {
		result, wsErr := util.NewJSON(event)
		if wsErr != nil {
			util.Notice(`binance fail to unmarshal json ` + err.Error())
			return
		}
		subscribe, _ := result.Get("stream").String()
		result = result.Get(`data`)
		//data := new(binance.WsBookTickerEvent)
		//wsErr := json.Unmarshal(event, &data)
		if result == nil {
			return
		}
		dialectSymbol := result.Get(`s`).MustString()
		updateId := result.Get(`u`).MustInt64()
		if strings.Contains(subscribe, `@depth`) {
			updateId = result.Get(`lastUpdateId`).MustInt64()
		}
		success, _, coin := model.GetCoinFromDialect(market, dialectSymbol)
		if !success {
			return
		}
		standardSymbol := coin + model.UniStandardTail[model.MarketTypeSpot]
		if market == model.BinanceMargin {
			standardSymbol = coin + model.UniStandardTail[model.MarketTypeMargin]
		}
		haveOld, old := marketConns.GetBidAsk(standardSymbol, market)
		if haveOld && old.UpdateId > updateId {
			return
		}
		if strings.Contains(subscribe, `@depth`) {
			handleDepthBinance(marketConns, result, market, standardSymbol, updateId)
		} else if strings.Contains(subscribe, `@bookTicker`) {
			handleTickerBinance(marketConns, result, market, standardSymbol, updateId)
		}
	}
	channels = make([]chan struct{}, 0)
	spotSubs := GetWSSubscribes(market, subType)
	spotChans, spotErr := WebSocketClient(market, wsBinance+`stream`, spotSubs,
		subscribeHandlerBinance, wsHandlerBinance, wsStepBinance)
	if spotErr != nil {
		util.SocketInfo(`fail to create binance spot conn %s`, spotErr.Error())
	}
	return spotChans, err
}

var subscribeHandlerBinance = func(market string, connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	subParam := make(map[string]interface{})
	subParam["method"] = "SUBSCRIBE"
	subParam["params"] = subscribes
	subParam["id"] = int(rand.Float64() * 10000)
	subParamJson, _ := json.Marshal(subParam)
	if err = SendToConnection(market, connection, subParamJson); err != nil {
		util.SocketInfo("binance spot can not subscribe %s %s", subParamJson, err.Error())
	}
	util.Notice(`%s send subscribe: %s `, market, subParamJson)
	time.Sleep(time.Millisecond * 500)
	return err
}

func handleTickerBinance(markets *model.Markets, json *simplejson.Json, market, standardSymbol string, updateId int64) {
	bidPrice, _ := strconv.ParseFloat(json.Get(`b`).MustString(), 64)
	bidAmount, _ := strconv.ParseFloat(json.Get(`B`).MustString(), 64)
	askPrice, _ := strconv.ParseFloat(json.Get(`a`).MustString(), 64)
	askAmount, _ := strconv.ParseFloat(json.Get(`A`).MustString(), 64)
	ts := json.Get(`E`).MustInt()
	now := int(time.Now().UnixNano() / int64(time.Millisecond))
	if ts == 0 {
		ts = now
	}
	if bidPrice > 0 && bidAmount > 0 && askPrice > 0 && askAmount > 0 {
		bidAsk := model.BidAsk{Ts: ts, TsReceived: now, UpdateId: updateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: market, Symbol: standardSymbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: market, Symbol: standardSymbol}}}
		haveOld, old := markets.GetBidAsk(standardSymbol, market)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
			return
		}
		if markets.SetBidAsk(standardSymbol, market, &bidAsk) {
			funcHandlers := GetFunctions(market, standardSymbol)
			if funcHandlers != nil {
				funcHandlers.Range(func(function, value interface{}) bool {
					setting := GetSetting(function.(string), market, standardSymbol)
					if setting != nil && value != nil && value.(model.CarryHandler) != nil {
						go value.(model.CarryHandler)(setting, &bidAsk)
					}
					return true
				})
			}
		}
	}
}

func handleDepthBinance(markets *model.Markets, json *simplejson.Json, market, standardSymbol string, updateId int64) {
	now := int(util.GetNowUnixMillion())
	bidAsk := model.BidAsk{UpdateId: updateId, Ts: now, TsReceived: now}
	var bids, asks []interface{}
	bidArray, _ := json.Get(`bids`).Array()
	bids = bidArray
	askArray, _ := json.Get(`asks`).Array()
	asks = askArray
	bidAsk.Bids = make([]model.Tick, len(bids))
	for i, value := range bids {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: market, Symbol: standardSymbol}
	}
	bidAsk.Asks = make([]model.Tick, len(asks))
	for i, value := range asks {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: market, Symbol: standardSymbol}
	}
	sort.Sort(bidAsk.Asks)
	sort.Sort(sort.Reverse(bidAsk.Bids))
	if markets.SetBidAsk(standardSymbol, market, &bidAsk) {
		funcHandlers := GetFunctions(market, standardSymbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), market, standardSymbol)
				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
					go value.(model.CarryHandler)(setting, &bidAsk)
				}
				return true
			})
		}
	}
}

func maintainChannelBinance(market string, subscribes []interface{}) {
	if !channelMaintainingBinance {
		channelMaintainingBinance = true
		for {
			time.Sleep(time.Minute * 5)
			err := PongAllConnectionsInterval(market, 500)
			if err != nil {
				util.SocketInfo("pong binance spot server error " + err.Error())
			}
			timeoutNum := 0
			for _, subscribe := range subscribes {
				dialectSymbol := strings.ToUpper(subscribe.(string)[0:strings.Index(subscribe.(string), `@`)])
				success, marketType, coin := model.GetCoinFromDialect(market, dialectSymbol)
				if !success {
					continue
				}
				symbol := coin + model.UniStandardTail[marketType]
				_, bidAsk := model.AppMarkets.GetBidAsk(symbol, market)
				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 180000 {
					timeoutNum++
					util.Notice("binance spot subscribe timeout " + symbol)
				}
			}
			if len(subscribes) > 0 && timeoutNum*10 > len(subscribes) {
				SetRequireReset(market)
				util.Notice(`require reset binance spot %d in all %d`, timeoutNum, len(subscribes))
			} else {
				util.Info(`no need reset %s`, market)
			}
		}
	}
}

func placeOrderBinanceSpot(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	decimal := 0
	price, decimal = model.FormatPrice(model.BinanceSpot, symbol, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	formattedAmount := model.GetAmountInMarket(model.BinanceSpot, symbol, amount, price, false)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	order.Price = price
	order.TriggerPrice = price
	if success {
		client := binance.NewClient(key, secret)
		service := client.NewCreateOrderService().Symbol(dialectSymbol).Quantity(amountStr)
		if orderSide == model.OrderSideBuy {
			service.Side(binance.SideTypeBuy)
		} else if orderSide == model.OrderSideSell {
			service.Side(binance.SideTypeSell)
		}
		if orderType == model.OrderTypeMarket {
			service.Type(binance.OrderTypeMarket)
		} else if orderType == model.OrderTypeLimit {
			service.Type(binance.OrderTypeLimit)
			service.Price(priceStr)
			service.TimeInForce(binance.TimeInForceTypeGTC)
		}
		orderResponse, err := service.Do(context.Background())
		if err != nil {
			util.Notice(fmt.Sprintf(`placeOrderBinanceSpot err: %s amount %s`, err.Error(), amountStr))
			order.ErrCode = err.Error()
			order.OrderId = ``
		} else {
			order.OrderId = strconv.FormatInt(orderResponse.OrderID, 10)
			order.Amount, _ = strconv.ParseFloat(orderResponse.OrigQuantity, 64)
		}
	}
}

func parseOrderBinanceSpotSdk(orderResp *binance.Order) (order *model.Order) {
	if orderResp == nil {
		return nil
	}
	order = &model.Order{Market: model.BinanceSpot, Status: model.CarryStatusFail}
	order.OrderId = strconv.FormatInt(orderResp.OrderID, 10)
	_, marketType, coin := model.GetCoinFromDialect(model.BinanceSpot, orderResp.Symbol)
	order.Symbol = coin + model.UniStandardTail[marketType]
	order.OrderSide = strings.ToLower(string(orderResp.Side))
	order.OrderType = strings.ToLower(string(orderResp.Type))
	order.Amount, _ = strconv.ParseFloat(orderResp.OrigQuantity, 64)
	order.Price, _ = strconv.ParseFloat(orderResp.Price, 64)
	order.DealAmount, _ = strconv.ParseFloat(orderResp.ExecutedQuantity, 64)
	order.OrderTime = time.UnixMilli(orderResp.Time)
	order.OrderUpdateTime = time.UnixMilli(orderResp.UpdateTime)
	order.Status = model.GetOrderStatus(model.BinanceSpot, string(orderResp.Status))
	if order.Status != model.CarryStatusSuccess && order.Status != model.CarryStatusFail {
		order.Status = model.CarryStatusWorking
	}
	if order.DealAmount > 0 && order.DealPrice == 0 {
		order.DealPrice = order.Price
	}
	return order
}

// 兼容spot margin
func parseOrderBinance(market string, orderJson *simplejson.Json) (order *model.Order) {
	if orderJson == nil {
		return nil
	}
	order = &model.Order{Market: market}
	suc, _, coin := model.GetCoinFromDialect(market, orderJson.Get(`symbol`).MustString())
	if suc {
		order.Symbol = coin + model.UniStandardTail[model.MarketTypeSpot]
	}
	order.OrderId = strconv.Itoa(orderJson.Get("orderId").MustInt())
	order.OrderTime = time.UnixMilli(orderJson.Get("transactTime").MustInt64())
	order.Price, _ = strconv.ParseFloat(orderJson.Get(`price`).MustString(), 64)
	order.Amount, _ = strconv.ParseFloat(orderJson.Get("origQty").MustString(), 64)
	order.DealPrice, _ = strconv.ParseFloat(orderJson.Get(`executedQty`).MustString(), 64)
	if strings.EqualFold(orderJson.Get(`side`).MustString(), model.OrderSideSell) {
		order.OrderSide = model.OrderSideSell
	} else if strings.EqualFold(orderJson.Get(`side`).MustString(), model.OrderSideBuy) {
		order.OrderSide = model.OrderSideBuy
	}
	order.Status = model.GetOrderStatus(market, orderJson.Get(`status`).MustString())
	order.OrderType = GetStandardOrderType(market, orderJson.Get(`type`).MustString())
	return order
}

func WsAccountServeBinanceSpot() {
	var wsAccountHandler = func(event []byte) {
		result, wsErr := util.NewJSON(event)
		if wsErr != nil {
			util.Notice(`binanceSpot fail to unmarshal account ws json ` + wsErr.Error())
			return
		}
		if strings.EqualFold(result.Get(`e`).MustString(), `executionReport`) {
			order := parseOrderJsBinance(model.BinanceSpot, result)
			if !order.HaveId() {
				return
			}
			funcHandlers := GetFunctions(model.BinanceSpot, order.Symbol)
			if funcHandlers != nil {
				funcHandlers.Range(func(function, value interface{}) bool {
					if model.AccountHandlerMap[function.(string)] != nil {
						go model.AccountHandlerMap[function.(string)](order)
					}
					return true
				})
			}
		}
	}
	if !maintainingAccountConnBinance {
		maintainingAccountConnBinance = true
		created := true
		for {
			accounts := model.AppConfig.GetAccounts(model.BinanceSpot)
			for _, account := range accounts {
				if account == nil {
					return
				}
				success, listenKey := RenewListenKeyBinanceSpot(account)
				if success {
					_, err := WsAccountClient(account.Key, model.BinanceSpot, wsBinance+`ws/`+listenKey, wsAccountHandler)
					if err != nil {
						created = false
						util.Notice(fmt.Sprintf(`fail to create account ws BinanceSpot %s`, err.Error()))
						continue
					}
				} else {
					created = false
				}
			}
			if created {
				time.Sleep(time.Minute * 30)
			} else {
				time.Sleep(time.Minute * 3)
			}
		}
	}
}

func RenewListenKeyBinanceSpot(account *model.Account) (success bool, listenKey string) {
	//signedRequestBinance(account.Key, account.Secret, model.BinanceSpot, http.MethodDelete,
	//	restBinance+`/api/v3/userDataStream`, true, nil)
	response := signedRequestBinance(account.Key, account.Secret, model.BinanceSpot, http.MethodPost,
		restBinance+`/api/v3/userDataStream`, false, nil)
	keyJson, _ := util.NewJSON(response)
	if keyJson != nil && len(keyJson.Get(`listenKey`).MustString()) > 0 {
		return true, keyJson.Get(`listenKey`).MustString()
	}
	time.Sleep(time.Second * 3)
	util.Notice(fmt.Sprintf(`fail to renew binanceSpot listen key retry`))
	RenewListenKeyBinanceSpot(account)
	return false, ``
}

func cancelOrderBinance(key, secret, market, symbol, orderId string) (suc bool, order *model.Order) {
	var path string
	if market == model.BinanceSpot {
		path = "/api/v3/order"
	} else if market == model.BinanceMargin {
		path = `/sapi/v1/margin/order`
	}
	responseBody := signedRequestBinance(key, secret, model.BinanceSpot, http.MethodDelete, restBinance+path,
		true, map[string]interface{}{`symbol`: symbol, `orderId`: orderId})
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		order = parseOrderBinance(market, orderJson)
		if order != nil && order.Status == model.CarryStatusFail {
			return true, order
		}
	}
	return false, nil
}

func cancelOrdersBinance(key, secret, market, symbol string) bool {
	success, marketType, coin, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if !success {
		return false
	}
	client := binance.NewClient(key, secret)
	var err error
	if market == model.BinanceSpot {
		_, err = client.NewCancelOpenOrdersService().Symbol(dialectSymbol).Do(context.Background())
	} else if market == model.BinanceMargin {
		_, err = client.NewCancelMarginOrderService().Symbol(dialectSymbol).Do(context.Background())
	}
	if err != nil && !strings.Contains(err.Error(), `-2010`) {
		util.Notice("cancelOrdersBinance err: " + err.Error() + " symbol: " + symbol + " marketType: " + marketType + " coin: " + coin + " But dialectSymbol: " + dialectSymbol)
		return false
	}
	return true
}

func getBalanceBinanceMargin(key, secret string) (success bool, balances []*model.Balance) {
	client := binance.NewClient(key, secret)
	balanceResp, err := client.NewGetMarginAccountService().Do(context.Background())
	if err != nil {
		util.Notice(`fail to refresh binance balance ` + err.Error())
		time.Sleep(time.Minute * 5)
		return getBalanceBinanceMargin(key, secret)
	}
	if !balanceResp.TradeEnabled {
		util.Notice(`binance margin balance can not trade`)
		return false, balances
	}
	balances = make([]*model.Balance, 0)
	for _, data := range balanceResp.UserAssets {
		if data.Asset == "" {
			continue
		}
		coin := data.Asset
		balance := &model.Balance{
			Market:      model.BinanceMargin,
			Coin:        coin,
			ID:          model.BinanceMargin + `_` + coin + `_` + util.GetNow().Format(time.RFC3339)[0:10],
			BalanceTime: util.GetNow(),
			AccountId:   key}
		if data.Free != "" { // 持仓,此处按照不进行借币计算
			balance.AvailableWithBorrow, _ = strconv.ParseFloat(data.Free, 64)
		}
		if data.Locked != "" {
			lockAmount, _ := strconv.ParseFloat(data.Locked, 64)
			balance.Amount = balance.AvailableWithBorrow + lockAmount
		}
		if balance.UsdValue == 0 && balance.Amount > 0 {
			symbolStandard := balance.Coin + model.UniStandardTail[model.MarketTypeMargin]
			_, price := GetPriceForce(key, secret, symbolStandard, model.BinanceMargin)
			balance.UsdValue = balance.Amount * price
		}
		balances = append(balances, balance)
	}
	return true, balances
}

func getBalanceBinanceSpot(key string, secret string) (success bool, balances []*model.Balance) {
	client := binance.NewClient(key, secret)
	balanceResp, err := client.NewGetAccountService().Do(context.Background())
	if err != nil {
		util.Notice(`fail to refresh binance balance ` + err.Error())
		time.Sleep(time.Minute * 5)
		return getBalanceBinanceSpot(key, secret)
	}
	if !balanceResp.CanTrade {
		util.Notice(`binance balance can not trade`)
		return false, balances
	}
	balances = make([]*model.Balance, 0)
	for _, data := range balanceResp.Balances {
		if data.Asset == "" {
			continue
		}
		coin := data.Asset
		balance := &model.Balance{
			Market:      model.BinanceSpot,
			Coin:        coin,
			ID:          model.BinanceSpot + `_` + coin + `_` + util.GetNow().Format(time.RFC3339)[0:10],
			BalanceTime: util.GetNow(),
			AccountId:   key}
		if data.Free != "" { // 持仓,此处按照不进行借币计算
			balance.AvailableWithBorrow, _ = strconv.ParseFloat(data.Free, 64)
		}
		if data.Locked != "" {
			lockAmount, _ := strconv.ParseFloat(data.Locked, 64)
			balance.Amount = balance.AvailableWithBorrow + lockAmount
		}
		if balance.UsdValue == 0 && balance.Amount > 0 {
			symbolStandard := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
			_, price := GetPriceForce(key, secret, symbolStandard, model.BinanceSpot)
			balance.UsdValue = balance.Amount * price
		}
		//if asset[`netAsset`] != nil {
		//	balance.Amount, _ = strconv.ParseFloat(asset[`netAsset`].(string), 64)
		//}
		//if asset[`borrowed`] != nil { //已借数量
		//	balance.Borrow, _ = strconv.ParseFloat(asset[`borrowed`].(string), 64)
		//}
		balances = append(balances, balance)
	}
	return true, balances
}

// getPriceBinanceSpot
func _(key, secret, symbol string) (success bool, price float64) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if !success {
		return false, 0
	}
	client := binance.NewClient(key, secret)
	resPrice, err := client.NewListPricesService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil && !strings.Contains(err.Error(), `-2010`) {
		util.Notice(fmt.Sprintf("getPriceBinanceSpot err: %s standard %s dialect %s",
			err.Error(), symbol, dialectSymbol))
		time.Sleep(time.Minute)
		return false, 0
	}
	if len(resPrice) > 0 {
		price, err = strconv.ParseFloat(resPrice[0].Price, 64)
		return err == nil, price
	}
	return true, 0
}

func queryOpenOrdersBinanceSpot(key, secret, symbol string) (orders []*model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if success || symbol == `` {
		orders = make([]*model.Order, 0)
		listOpenOrderService := binance.NewClient(key, secret).NewListOpenOrdersService()
		if symbol != `` {
			listOpenOrderService = listOpenOrderService.Symbol(dialectSymbol)
		}
		resArray, err := listOpenOrderService.Do(context.Background())
		if err != nil {
			util.Notice(`queryOpenOrdersBinanceSpot err %s %s %s`, symbol, dialectSymbol, err.Error())
		}
		for _, res := range resArray {
			order := parseOrderBinanceSpotSdk(res)
			orders = append(orders, order)
		}
	}
	return
}

func queryOrderBinanceSpot(key, secret, symbol string, orderId string) (order *model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if success {
		orderIdInt, _ := strconv.ParseInt(orderId, 10, 64)
		client := binance.NewClient(key, secret)
		orderResp, err := client.NewGetOrderService().Symbol(dialectSymbol).OrderID(orderIdInt).Do(context.Background())
		if err != nil {
			util.Notice("queryOrderBinanceSpot err: " + err.Error())
			return
		}
		order = parseOrderBinanceSpotSdk(orderResp)
	}
	return
}

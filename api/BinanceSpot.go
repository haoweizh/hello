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
	"sort"
	"strconv"
	"strings"
	"time"
)

// const restBinanceSpot = `https://api.binance.com`
const restDataBinanceSpot = `https://data.binance.com`
const wsBinanceSpot = "wss://stream.binance.com:9443/stream"
const wsStepBinanceSpot = 20

var channelMaintainingBinanceSpot = false

func getMarketsBinanceSpot(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	util.Notice(fmt.Sprintf("start to getMarketsBinanceSpot %s", key))
	marketInfos = make(map[string]*model.MarketInfo)
	client := binance.NewClient(key, secret)
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		util.Notice(fmt.Sprintf("getMarketsBinanceSpot %s err: %s %v休息五分钟", key, err.Error(), exchangeInfo))
		time.Sleep(time.Minute * 5)
		return getMarketsBinanceSpot(key, secret)
	}
	for _, item := range exchangeInfo.Symbols {
		if item.QuoteAsset == "" || item.BaseAsset == "" {
			continue
		}
		haveSpot := false
		if item.Permissions != nil && item.Status == "TRADING" &&
			item.QuoteAsset == model.DialectTail[model.MarketTypeSpot][model.BinanceSpot] {
			for _, permission := range item.Permissions {
				if permission == `SPOT` {
					haveSpot = true
				}
			}
		}
		if !haveSpot {
			continue
		}
		symbol := item.BaseAsset + model.UniStandardTail[model.MarketTypeSpot]
		marketInfo := &model.MarketInfo{Market: model.BinanceSpot, Name: symbol, MoneyMin: 10}
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

func WsDepthServeBinanceSpot(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	subType := model.SubscribeTicker
	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
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
		success, _, coin := model.GetCoinFromDialect(model.BinanceSpot, dialectSymbol)
		if !success {
			return
		}
		standardSymbol := coin + model.UniStandardTail[model.MarketTypeSpot]
		haveOld, old := markets.GetBidAsk(standardSymbol, model.BinanceSpot)
		if haveOld && old.UpdateId > updateId {
			return
		}
		if strings.Contains(subscribe, `@depth`) {
			handleDepthBinanceSpot(markets, result, standardSymbol, updateId)
		} else if strings.Contains(subscribe, `@bookTicker`) {
			handleTickerBinanceSpot(markets, result, standardSymbol, updateId)
		}
	}
	channels = make([]chan struct{}, 0)
	spotSubs := GetWSSubscribes(model.BinanceSpot, subType)
	spotChans, spotErr := WebSocketClient(model.BinanceSpot, wsBinanceSpot, spotSubs,
		subscribeHandlerBinanceSpot, wsHandler, orderHandler, wsStepBinanceSpot)
	if spotErr != nil {
		util.SocketInfo(`fail to create binance spot conn %s`, spotErr.Error())
	}
	return spotChans, err
}

var subscribeHandlerBinanceSpot = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	subParam := make(map[string]interface{})
	subParam["method"] = "SUBSCRIBE"
	subParam["params"] = subscribes
	subParam["id"] = int(rand.Float64() * 10000)
	subParamJson, _ := json.Marshal(subParam)
	if err = SendToConnection(model.BinanceSpot, connection, subParamJson); err != nil {
		util.SocketInfo("binance spot can not subscribe %s %s", subParamJson, err.Error())
	}
	util.Notice(`%s send subscribe: %s `, model.BinanceSpot, subParamJson)
	time.Sleep(time.Millisecond * 500)
	return err
}

func handleTickerBinanceSpot(markets *model.Markets, json *simplejson.Json, standardSymbol string, updateId int64) {
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
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.BinanceSpot, Symbol: standardSymbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.BinanceSpot, Symbol: standardSymbol}}}
		haveOld, old := markets.GetBidAsk(standardSymbol, model.BinanceSpot)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
			return
		}
		if markets.SetBidAsk(standardSymbol, model.BinanceSpot, &bidAsk) {
			funcHandlers := GetFunctions(model.BinanceSpot, standardSymbol)
			if funcHandlers != nil {
				funcHandlers.Range(func(function, value interface{}) bool {
					setting := GetSetting(function.(string), model.BinanceSpot, standardSymbol)
					if setting != nil && value != nil {
						go value.(model.CarryHandler)(setting, &bidAsk)
					}
					return true
				})
			}
		}
	}
}

func handleDepthBinanceSpot(markets *model.Markets, json *simplejson.Json, standardSymbol string, updateId int64) {
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
		bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: model.BinanceSpot, Symbol: standardSymbol}
	}
	bidAsk.Asks = make([]model.Tick, len(asks))
	for i, value := range asks {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: model.BinanceSpot, Symbol: standardSymbol}
	}
	sort.Sort(bidAsk.Asks)
	sort.Sort(sort.Reverse(bidAsk.Bids))
	if markets.SetBidAsk(standardSymbol, model.BinanceSpot, &bidAsk) {
		funcHandlers := GetFunctions(model.BinanceSpot, standardSymbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), model.BinanceSpot, standardSymbol)
				if setting != nil && value != nil {
					go value.(model.CarryHandler)(setting, &bidAsk)
				}
				return true
			})
		}
	}
}

func maintainChannelBinanceSpot(subscribes []interface{}) {
	if !channelMaintainingBinanceSpot {
		channelMaintainingBinanceSpot = true
		for true {
			time.Sleep(time.Minute * 5)
			err := PongAllConnectionsInterval(model.BinanceSpot, 500)
			if err != nil {
				util.SocketInfo("pong binance spot server error " + err.Error())
			}
			timeoutNum := 0
			for _, subscribe := range subscribes {
				dialectSymbol := strings.ToUpper(subscribe.(string)[0:strings.Index(subscribe.(string), `@`)])
				success, marketType, coin := model.GetCoinFromDialect(model.BinanceSpot, dialectSymbol)
				if !success {
					continue
				}
				symbol := coin + model.UniStandardTail[marketType]
				_, bidAsk := model.AppMarkets.GetBidAsk(symbol, model.BinanceSpot)
				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 180000 {
					timeoutNum++
					util.Notice("binance spot subscribe timeout " + symbol)
				}
			}
			if len(subscribes) > 0 && timeoutNum*10 > len(subscribes) {
				SetRequireReset(model.BinanceSpot)
				util.Notice(`require reset binance spot %d in all %d`, timeoutNum, len(subscribes))
			} else {
				util.Notice(`no need reset %s`, model.BinanceSpot)
			}
		}
	}
}

func placeOrderBinanceSpot(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	price, decimal := model.FormatPrice(model.BinanceSpot, symbol, price)
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

func cancelOrdersBinanceSpot(key string, secret string, symbol string) bool {
	success, marketType, coin, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if !success {
		return false
	}
	client := binance.NewClient(key, secret)
	_, err := client.NewCancelOpenOrdersService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil && !strings.Contains(err.Error(), `-2010`) {
		util.Notice("cancelOrdersBinanceSpot err: " + err.Error() + " symbol: " + symbol + " marketType: " + marketType + " coin: " + coin + " But dialectSymbol: " + dialectSymbol)
		return false
	}
	return true
}

func getBalanceBinanceSpot(key string, secret string) (success bool, balances []*model.Balance) {
	client := binance.NewClient(key, secret)
	balanceResp, err := client.NewGetAccountService().Do(context.Background())
	if err != nil {
		util.SocketInfo(`fail to refresh binance balance ` + err.Error())
		time.Sleep(time.Minute * 5)
		return getBalanceBinanceSpot(key, secret)
	}
	if !balanceResp.CanTrade {
		util.SocketInfo(`binance balance can not trade`)
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
		order = &model.Order{Market: model.BinanceSpot, Status: model.CarryStatusFail}
		if orderResp != nil {
			order.OrderId = orderId
			order.Symbol = symbol
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
		}
	}
	return
}

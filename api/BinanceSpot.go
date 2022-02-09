package api

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"sort"
	"strconv"
	"strings"
	"time"
)

func getMarketsBinanceSpot(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	client := binance.NewClient(key, secret)
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		util.Notice("getMarketsBinanceSpot err: " + err.Error())
		time.Sleep(time.Second * 2)
		return getMarketsBinanceSpot(key, secret)
	}
	for _, item := range exchangeInfo.Symbols {
		if item.QuoteAsset == "" || item.BaseAsset == "" {
			continue
		}
		haveSpot := false
		if item.Permissions != nil && item.IsSpotTradingAllowed &&
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
				util.Notice("minNotional:" + data[`minNotional`].(string))
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
			util.SocketInfo(`binance fail to unmarshal json ` + err.Error())
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
		if dialectSymbol == `` {
			return
		}
		if strings.Contains(subscribe, `@depth`) {
			handleDepthBinanceSpot(markets, result, dialectSymbol, updateId)
		} else if strings.Contains(subscribe, `@bookTicker`) {
			handleTickerBinanceSpot(markets, result, dialectSymbol, updateId)
		}
	}
	channels = make([]chan struct{}, 0)
	spotSubs := GetWSSubscribes(model.BinanceSpot, subType)
	spotChans, spotErr := WebSocketClient(model.BinanceSpot, wsBinance, spotSubs,
		subscribeHandlerBinance, wsHandler, orderHandler, wsStepBinance)
	if spotErr != nil {
		util.SocketInfo(`fail to create binance spot conn %s`, spotErr.Error())
	}
	return spotChans, err
}

func handleTickerBinanceSpot(markets *model.Markets, json *simplejson.Json, dialectSymbol string, updateId int64) {
	bidPrice, _ := strconv.ParseFloat(json.Get(`b`).MustString(), 64)
	bidAmount, _ := strconv.ParseFloat(json.Get(`B`).MustString(), 64)
	askPrice, _ := strconv.ParseFloat(json.Get(`a`).MustString(), 64)
	askAmount, _ := strconv.ParseFloat(json.Get(`A`).MustString(), 64)
	ts := json.Get(`E`).MustInt()
	now := int(time.Now().UnixNano() / int64(time.Millisecond))
	if ts == 0 {
		ts = now
	}
	if dialectSymbol != `` && bidPrice > 0 && bidAmount > 0 && askPrice > 0 && askAmount > 0 {
		marketType := model.MarketTypeSpot
		success, _, coin := model.GetCoinFromDialect(model.BinanceSpot, dialectSymbol)
		if !success {
			return
		}
		standardSymbol := coin + model.UniStandardTail[marketType]
		bidAsk := model.BidAsk{Ts: ts, TsReceived: now, UpdateId: updateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.BinanceSpot, Symbol: standardSymbol, Side: model.OrderSideBuy}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.BinanceSpot, Symbol: standardSymbol, Side: model.OrderSideSell}}}
		haveOld, old := markets.GetBidAsk(standardSymbol, model.BinanceSpot)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
			return
		}
		if markets.SetBidAsk(standardSymbol, model.BinanceSpot, &bidAsk) {
			for function, handler := range model.GetFunctions(model.BinanceSpot, standardSymbol) {
				if handler != nil {
					setting := model.GetSetting(function, model.BinanceSpot, standardSymbol)
					if setting != nil {
						go handler(setting, &bidAsk)
					}
				}
			}
		}
	}
}

func handleDepthBinanceSpot(markets *model.Markets, json *simplejson.Json, dialectSymbol string, updateId int64) {
	var standardSymbol string
	bidAsk := model.BidAsk{UpdateId: updateId}
	var bids, asks []interface{}
	tickId, _ := json.Get(`lastUpdateId`).Int64()
	success, _, coin := model.GetCoinFromDialect(model.BinanceSpot, dialectSymbol)
	if !success {
		return
	}
	standardSymbol = coin + model.UniStandardTail[model.MarketTypeSpot]
	if tickId > getLastTickIdBinance(standardSymbol) {
		setLastTickIdBinance(standardSymbol, tickId)
		bidAsk.Ts = int(util.GetNowUnixMillion())
		bidAsk.TsReceived = int(util.GetNowUnixMillion())
	} else {
		return
	}
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
		bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: model.BinanceSpot, Symbol: standardSymbol, Side: model.OrderSideBuy}
	}
	bidAsk.Asks = make([]model.Tick, len(asks))
	for i, value := range asks {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: model.BinanceSpot, Symbol: standardSymbol, Side: model.OrderSideSell}
	}
	sort.Sort(bidAsk.Asks)
	sort.Sort(sort.Reverse(bidAsk.Bids))
	haveOld, old := markets.GetBidAsk(standardSymbol, model.BinanceSpot)
	if haveOld && old.UpdateId > bidAsk.UpdateId {
		return
	}
	if markets.SetBidAsk(standardSymbol, model.BinanceSpot, &bidAsk) {
		for function, handler := range model.GetFunctions(model.BinanceSpot, standardSymbol) {
			if handler != nil {
				setting := model.GetSetting(function, model.BinanceSpot, standardSymbol)
				if setting != nil {
					go handler(setting, &bidAsk)
				}
			}
		}
	}
}

func maintainChannelBinanceSpot() {
	if !channelMaintainingBinance {
		channelMaintainingBinance = true
		for true {
			time.Sleep(time.Minute * 5)
			ts := time.Now().UnixNano() / int64(time.Millisecond)
			pong := []byte(fmt.Sprintf(`{"method":"PONG","E":%d}`, ts))
			err := SendToAllConnections(model.BinanceSpot, pong)
			if err != nil {
				util.SocketInfo("pong binance server error " + err.Error())
			}
		}
	}
}

func placeOrderBinanceSpot(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	price, decimal := model.FormatPrice(model.BinanceSpot, symbol, orderSide, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	formattedAmount := model.GetAmountInMarket(model.BinanceSpot, symbol, amount, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
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
			util.Notice("placeOrderBinanceSpot err: " + err.Error())
			order.OrderId = ``
		} else {
			order.OrderId = strconv.FormatInt(orderResponse.OrderID, 10)
		}
	}
}

func cancelOrdersBinanceSpot(key string, secret string, symbol string) bool {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if !success {
		return false
	}
	client := binance.NewClient(key, secret)
	_, err := client.NewCancelOpenOrdersService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil {
		util.Notice("cancelOrdersBinanceSpot err: " + err.Error())
		return false
	}
	return true
}

func getBalanceBinanceSpot(key string, secret string) (success bool, balances []*model.Balance) {
	client := binance.NewClient(key, secret)
	balanceResp, err := client.NewGetAccountService().Do(context.Background())
	if err != nil {
		util.SocketInfo(`fail to refresh binance balance `)
		time.Sleep(time.Second * 2)
		return getBalanceBinance(key, secret)
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
			getTick, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.BinanceSpot)
			if getTick {
				balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
			}
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
			order.OrderTime = time.Unix(orderResp.Time, 0)
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

package api

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
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
		if item.Permissions != nil {
			for _, permission := range item.Permissions {
				if permission == `SPOT` && item.IsSpotTradingAllowed {
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
			handleDepthBinance(markets, result, dialectSymbol, updateId)
		} else if strings.Contains(subscribe, `@bookTicker`) {
			handleTickerBinance(markets, result, dialectSymbol, updateId)
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
	if success {
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

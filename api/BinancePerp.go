package api

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func getMarketsBinancePerp(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	client := binance.NewFuturesClient(key, secret)
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		util.Notice("getMarketsBinancePerp err: " + err.Error())
		time.Sleep(time.Second * 2)
		getMarketsBinancePerp(key, secret)
		return marketInfos
	}
	for _, item := range exchangeInfo.Symbols {
		if item.QuoteAsset == "" || item.BaseAsset == "" {
			continue
		}
		if item.ContractType == `PERPETUAL` && item.Status == "TRADING" {
			symbol := item.BaseAsset + model.UniStandardTail[model.MarketTypePerp]
			marketInfo := &model.MarketInfo{Market: model.BinancePerp, Name: symbol, MoneyMin: 10}
			marketInfos[marketInfo.Name] = marketInfo
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
	}
	return marketInfos
}

func WsDepthServeBinancePerp(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	subType := model.SubscribeTicker
	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		result, wsErr := util.NewJSON(event)
		if wsErr != nil {
			util.SocketInfo(`binance fail to unmarshal json ` + err.Error())
			return
		}
		subscribe, _ := result.Get("stream").String()
		result = result.Get(`data`)
		if result == nil {
			return
		}
		dialectSymbol := result.Get(`s`).MustString()
		updateId := result.Get(`u`).MustInt64()
		if dialectSymbol == `` {
			return
		}
		if strings.Contains(subscribe, `@depth`) {
			handleDepthBinancePerp(markets, result, dialectSymbol, updateId)
		} else if strings.Contains(subscribe, `@bookTicker`) {
			handleTickerBinancePerp(markets, result, dialectSymbol, updateId)
		}
	}
	channels = make([]chan struct{}, 0)
	perpSubs := GetWSSubscribes(model.BinancePerp, subType)
	perpChans, perpErr := WebSocketClient(model.BinancePerp, wsBinanceFuture, perpSubs,
		subscribeHandlerBinance, wsHandler, orderHandler, wsStepBinance)
	if perpErr != nil {
		util.SocketInfo(`fail to create binance perp conn %s`, perpErr.Error())
	}
	return perpChans, err
}

func handleTickerBinancePerp(markets *model.Markets, json *simplejson.Json, dialectSymbol string, updateId int64) {
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
		marketType := model.MarketTypePerp
		_, _, coin := model.GetCoinFromDialect(model.BinancePerp, dialectSymbol)
		standardSymbol := coin + model.UniStandardTail[marketType]
		bidAsk := model.BidAsk{Ts: ts, TsReceived: now, UpdateId: updateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.BinancePerp, Symbol: standardSymbol, Side: model.OrderSideBuy}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.BinancePerp, Symbol: standardSymbol, Side: model.OrderSideSell}}}
		haveOld, old := markets.GetBidAsk(standardSymbol, model.BinancePerp)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
			return
		}
		if markets.SetBidAsk(standardSymbol, model.BinancePerp, &bidAsk) {
			for function, handler := range model.GetFunctions(model.BinancePerp, standardSymbol) {
				if handler != nil {
					setting := model.GetSetting(function, model.BinancePerp, standardSymbol)
					if setting != nil {
						go handler(setting, &bidAsk)
					}
				}
			}
		}
	}
}

func handleDepthBinancePerp(markets *model.Markets, json *simplejson.Json, dialectSymbol string, updateId int64) {
	var standardSymbol string
	bidAsk := model.BidAsk{UpdateId: updateId}
	var bids, asks []interface{}
	_, _, coin := model.GetCoinFromDialect(model.BinancePerp, dialectSymbol)
	standardSymbol = coin + model.UniStandardTail[model.MarketTypePerp]
	nowTradeTime, _ := json.Get(`T`).Int64()
	if nowTradeTime <= 0 || nowTradeTime < getLastTradeTimeBinance(standardSymbol) {
		return
	}
	setLastTradeTimeBinance(standardSymbol, nowTradeTime)
	bidAsk.Ts = json.Get(`E`).MustInt()
	bidAsk.TsReceived = int(util.GetNowUnixMillion())
	bidArray, _ := json.Get(`b`).Array()
	bids = bidArray
	askArray, _ := json.Get(`a`).Array()
	asks = askArray
	dialectSymbol = json.Get(`s`).MustString()
	bidAsk.Bids = make([]model.Tick, len(bids))
	for i, value := range bids {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: model.BinancePerp, Symbol: standardSymbol, Side: model.OrderSideBuy}
	}
	bidAsk.Asks = make([]model.Tick, len(asks))
	for i, value := range asks {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: model.BinancePerp, Symbol: standardSymbol, Side: model.OrderSideSell}
	}
	sort.Sort(bidAsk.Asks)
	sort.Sort(sort.Reverse(bidAsk.Bids))
	haveOld, old := markets.GetBidAsk(standardSymbol, model.BinancePerp)
	if haveOld && old.UpdateId > bidAsk.UpdateId {
		return
	}
	if markets.SetBidAsk(standardSymbol, model.BinancePerp, &bidAsk) {
		for function, handler := range model.GetFunctions(model.BinancePerp, standardSymbol) {
			if handler != nil {
				setting := model.GetSetting(function, model.BinancePerp, standardSymbol)
				if setting != nil {
					go handler(setting, &bidAsk)
				}
			}
		}
	}
}

func maintainChannelBinancePerp() {
	if !channelMaintainingBinance {
		channelMaintainingBinance = true
		for true {
			time.Sleep(time.Minute * 5)
			ts := time.Now().UnixNano() / int64(time.Millisecond)
			pong := []byte(fmt.Sprintf(`{"method":"PONG","E":%d}`, ts))
			err := SendToAllConnections(model.BinancePerp, pong)
			if err != nil {
				util.SocketInfo("pong binance server error " + err.Error())
			}
		}
	}
}

func placeOrderBinancePerp(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	price, decimal := model.FormatPrice(model.BinancePerp, symbol, orderSide, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	formattedAmount := model.GetAmountInMarket(model.BinancePerp, symbol, amount, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	if success {
		client := binance.NewFuturesClient(key, secret)
		service := client.NewCreateOrderService().Symbol(dialectSymbol).Quantity(amountStr)
		if orderSide == model.OrderSideBuy {
			service.Side(futures.SideTypeBuy)
		} else if orderSide == model.OrderSideSell {
			service.Side(futures.SideTypeSell)
		}
		if orderType == model.OrderTypeMarket {
			service.Type(futures.OrderTypeMarket)
		} else if orderType == model.OrderTypeLimit {
			service.Type(futures.OrderTypeLimit)
			service.Price(priceStr)
			service.TimeInForce(futures.TimeInForceTypeGTC)
		}
		orderResponse, err := service.Do(context.Background())
		if err != nil {
			util.Notice("placeOrderBinancePerp err: " + err.Error())
			order.OrderId = ``
		} else {
			order.OrderId = strconv.FormatInt(orderResponse.OrderID, 10)
		}
	}
}

func cancelOrdersBinancePerp(key string, secret string, symbol string) bool {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	if success {
		return false
	}
	client := futures.NewClient(key, secret)
	err := client.NewCancelAllOpenOrdersService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil {
		util.Notice("cancelOrdersBinancePerp err: " + err.Error())
		return false
	}
	return true
}

//sdk暂不支持该接口
func getPositionsBinancePerp(key, secret string) (success bool, positions []*model.Position, accountValue, availableU float64) {
	//client := futures.NewClient(key, secret)
	//positionResp, err := client.NewGetAccountService().Do(context.Background())
	//if err != nil {
	//	util.SocketInfo(`fail to refresh binance position `)
	//	time.Sleep(time.Second * 2)
	//	return getPositionsBinancePerp(key, secret)
	//}
	//if !positionResp.CanTrade {
	//	util.SocketInfo(`binance position can not trade`)
	//	return false, nil, 0, 0
	//}
	//accountValue, _ = strconv.ParseFloat(positionResp.TotalWalletBalance, 64)
	//availableU, _ = strconv.ParseFloat(positionResp., 64)
	//positions = make([]*model.Position, 0)
	//for _, data := range positionResp.Positions {
	//
	//}

	//todo 验证accountValue
	responseBody := signedRequestBinance(key, secret, http.MethodGet, restBinanceFuture+"/fapi/v2/account", true, nil)
	positionJson, err := util.NewJSON(responseBody)
	if err != nil || positionJson == nil {
		util.SocketInfo(`fail to refresh binance position `)
		time.Sleep(time.Second * 2)
		return getPositionsBinancePerp(key, secret)
	}
	success = positionJson.Get("canTrade").MustBool()
	if success {
		positions = make([]*model.Position, 0)
		totalBalanceJson := positionJson.Get(`totalWalletBalance`).MustString()
		totalUnrealizedProfitJson := positionJson.Get(`totalUnrealizedProfit`).MustString()
		availableJson := positionJson.Get(`availableBalance`).MustString()
		unrealizedProfit, _ := strconv.ParseFloat(totalUnrealizedProfitJson, 64)
		totalBalance, _ := strconv.ParseFloat(totalBalanceJson, 64)
		accountValue = totalBalance + unrealizedProfit
		availableU, _ = strconv.ParseFloat(availableJson, 64)
		data := positionJson.Get("positions").MustArray()
		for _, item := range data {
			position := &model.Position{Market: model.BinancePerp, Ts: util.GetNowUnixMillion()}
			value := item.(map[string]interface{})
			if value[`symbol`] != nil {
				_, _, coin := model.GetCoinFromDialect(model.BinancePerp, value[`symbol`].(string))
				position.Currency = coin + model.UniStandardTail[model.MarketTypePerp]
			}
			if value[`positionAmt`] != nil {
				position.Holding, _ = strconv.ParseFloat(value[`positionAmt`].(string), 64)
			}
			if value[`entryPrice`] != nil {
				position.EntryPrice, _ = strconv.ParseFloat(value[`entryPrice`].(string), 64)
			}
			if value[`unrealizedProfit`] != nil {
				position.ProfitUnreal, _ = strconv.ParseFloat(value[`unrealizedProfit`].(string), 64)
			}
			positions = append(positions, position)
		}
	}
	return success, positions, accountValue, availableU
}

func getFundingRateBinancePerp(key, secret, symbol string) (fundingRate *model.FundingRate) {
	_, _, _, dialectSymbol := model.GetFromStandard(model.BinancePerp, symbol)
	client := futures.NewClient(key, secret)
	rateResp, err := client.NewPremiumIndexService().Symbol(dialectSymbol).Do(context.Background())
	if err != nil {
		util.Notice("getFundingRateBinancePerp err: " + err.Error())
		return
	}
	rateStr := rateResp[0].LastFundingRate
	rate, _ := strconv.ParseFloat(rateStr, 64)
	nextFundingTime := rateResp[0].NextFundingTime
	fundingRate = &model.FundingRate{
		Rate:       rate,
		UpdateTime: util.GetNow().Unix(),
		ExpireTime: nextFundingTime / 1000}
	return
}

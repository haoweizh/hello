package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/api/dtos"
	"hello/model"
	"hello/util"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const bitgetRestUrl = "https://api.bitget.com"
const bitgetSpotWsUrl = "wss://ws.bitget.com/spot/v1/stream"

var channelMaintainingBitgetSpot = false

func getMarketsBitgetSpot() (marketInfos map[string]*model.MarketInfo) {
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/spot/v1/public/products", "", map[string]string{}, 30)
	spotResp := &dtos.BitgetSpotMarketResp{}
	spotJsonErr := json.Unmarshal(httpResp, spotResp)
	if spotResp == nil || spotResp.Code != "00000" {
		util.Notice(fmt.Sprintf("get bitget spot market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, spotJsonErr))
		return
	}
	marketInfos = make(map[string]*model.MarketInfo)
	for _, symbolInfo := range spotResp.Data {
		if symbolInfo.Status != "online" || symbolInfo.QuoteCoin != "USDT" {
			continue
		}
		symbol := symbolInfo.BaseCoin + model.UniStandardTail[model.MarketTypeSpot]
		marketInfo := &model.MarketInfo{Name: symbol, Market: model.BitgetSpot}
		priceDecimal, _ := strconv.Atoi(symbolInfo.PriceScale)
		marketInfo.PriceDecimal = priceDecimal
		marketInfo.PriceIncrement = 1 / math.Pow10(priceDecimal)
		amountPrecision, _ := strconv.Atoi(symbolInfo.QuantityScale)
		marketInfo.SizeIncrement = 1 / math.Pow10(amountPrecision)
		marketInfo.SizeMin, _ = strconv.ParseFloat(symbolInfo.MinTradeAmount, 64)
		if marketInfo.SizeMin == 0 {
			marketInfo.SizeMin = marketInfo.SizeIncrement
		}
		marketInfo.SizeMax, _ = strconv.ParseFloat(symbolInfo.MaxTradeAmount, 64)
		marketInfo.MoneyMin, _ = strconv.ParseFloat(symbolInfo.MinTradeUSDT, 64)
		marketInfos[marketInfo.Name] = marketInfo
	}
	return marketInfos
}

func WsDepthServeBitgetSpot(markets *model.Markets) (channels []chan struct{}, err error) {
	bookWsHandler := func(event []byte) {
		//util.Notice(fmt.Sprintf("bitget spot ws book ticker: %s", event))
		if len(event) == 4 {
			return
		}
		bookWsResp := &dtos.BitgetBoosWsResp{}
		jsonErr := json.Unmarshal(event, bookWsResp)
		if jsonErr != nil {
			util.SocketInfo(`bitget fail to unmarshal book ws data json ` + jsonErr.Error())
			return
		}
		if bookWsResp.Arg.InstType == "sp" && bookWsResp.Action == "snapshot" {
			if bookWsResp.Arg.InstId == "" || !util.EndWith(bookWsResp.Arg.InstId, "USDT") || bookWsResp.Data == nil {
				return
			}
			symbol := bookWsResp.Arg.InstId[0:len(bookWsResp.Arg.InstId)-4] + model.UniStandardTail[model.MarketTypeSpot]
			bidAsk := model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
			if len(bookWsResp.Data) > 1 ||
				len(bookWsResp.Data[0].Bids) < 1 || len(bookWsResp.Data[0].Bids[0]) < 2 ||
				len(bookWsResp.Data[0].Asks) < 1 || len(bookWsResp.Data[0].Asks[0]) < 2 {
				return
			}
			bidPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][0], 64)
			bidAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][1], 64)
			bids := make([]model.Tick, 0)
			bids = append(bids, model.Tick{Price: bidPrice, Amount: bidAmount, Market: model.BitgetSpot, Symbol: symbol})
			bidAsk.Bids = bids
			askPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][0], 64)
			askAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][1], 64)
			asks := make([]model.Tick, 0)
			asks = append(asks, model.Tick{Price: askPrice, Amount: askAmount, Market: model.BitgetSpot, Symbol: symbol})
			bidAsk.Asks = asks
			bidAsk.Ts, _ = strconv.Atoi(bookWsResp.Data[0].Ts)
			bidAsk.UpdateId, _ = strconv.ParseInt(bookWsResp.Data[0].Ts, 10, 64)
			haveOld, old := markets.GetBidAsk(symbol, model.BitgetSpot)
			if haveOld && old.UpdateId > bidAsk.UpdateId {
				return
			}
			if markets.SetBidAsk(symbol, model.BitgetSpot, &bidAsk) {
				util.Info(fmt.Sprintf("success get bitget ticker: %s %f", symbol, bidAsk.Asks[0].Price))
				funcHandlers := GetFunctions(model.BitgetSpot, symbol)
				if funcHandlers != nil {
					funcHandlers.Range(func(function, value interface{}) bool {
						setting := GetSetting(function.(string), model.BitgetSpot, symbol)
						if setting != nil && value != nil && value.(model.CarryHandler) != nil {
							go value.(model.CarryHandler)(setting, &bidAsk)
						}
						return true
					})
				}
			}
		}
	}
	channels = make([]chan struct{}, 0)
	spotSubscribes := make([]interface{}, 0)
	symbols := GetMarketSymbols(model.BitgetSpot)
	for symbol := range symbols {
		spotSubscribes = append(spotSubscribes, symbol)
	}
	spotBookChannels, spotBookErr := WebSocketClient(model.BitgetSpot, bitgetSpotWsUrl,
		spotSubscribes, subscribeHandlerBitgetSpotBookTicker, bookWsHandler, 30)
	if spotBookErr == nil {
		util.Info(`finish connect public Bitget spot book wss `)
		channels = append(channels, spotBookChannels...)
	} else {
		util.Notice(`fail to connect public Bitget spot book wss `)
		return nil, spotBookErr
	}
	go maintainChannelBitgetSpot()
	return channels, nil
}

var subscribeHandlerBitgetSpotBookTicker = func(market string, connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	var params []map[string]string
	for _, subscribe := range subscribes {
		success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, subscribe.(string))
		if !success {
			continue
		}
		symbol := strings.Split(dialectSymbol, "_")[0]
		params = append(params, map[string]string{"instType": "sp", "channel": "books5", "instId": symbol})
	}
	subscribeMap := make(map[string]interface{})
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = params
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.BitgetSpot, connection, subscribeMessage); err != nil {
		util.Info(" bitget can not subscribe %s %s", subscribeMessage, err.Error())
	}
	util.Info(`bitget subscribed ` + string(subscribeMessage))
	time.Sleep(time.Second)
	return err
}

func maintainChannelBitgetSpot() {
	if !channelMaintainingBitgetSpot {
		channelMaintainingBitgetSpot = true
		go func() {
			for {
				time.Sleep(time.Second * 20)
				if err := SendToAllConnections(model.BitgetSpot, []byte(`ping`)); err != nil {
					util.Info("bitget spot channel ping error " + err.Error())
				} else {
					util.Info("bitget spot channel ping success")
				}
			}
		}()
	}
}

func getBalanceBitgetSpot(key string, secret string) (success bool, balances []*model.Balance) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	httpResp, httpErr := client.DoGet("/api/spot/v1/account/assets", map[string]string{})
	bitgetBalanceResp := &dtos.BitgetBalanceResp{}
	jsonErr := json.Unmarshal(httpResp, bitgetBalanceResp)
	if bitgetBalanceResp == nil || bitgetBalanceResp.Code != "00000" {
		util.SocketInfo(fmt.Sprintf("fail to refresh bitgetspot balance, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		time.Sleep(time.Second * 2)
		return getBalanceBitgetSpot(key, secret)
	} else {
		util.SocketInfo(fmt.Sprintf("get bitgetspot balance success, resp: %s ", httpResp))
	}
	balances = make([]*model.Balance, 0)
	for _, account := range bitgetBalanceResp.Data {
		balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.BitgetSpot, Coin: account.CoinName}
		balance.FrozenAmount, _ = strconv.ParseFloat(account.Frozen, 64)
		balance.AvailableWithBorrow, _ = strconv.ParseFloat(account.Available, 64)
		balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
		priceGet, price := GetPriceForce(key, secret, balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.BitgetSpot)
		//priceGet, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.BitgetSpot)
		if priceGet {
			balance.UsdValue = balance.Amount * price
		}
		balances = append(balances, balance)
	}
	return true, balances
}

func placeOrderBitgetSpot(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	priceSpot, decimalSpot := model.FormatPrice(model.BitgetSpot, symbol, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.BitgetSpot, symbol, amount, priceSpot, false)))
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
	if !success {
		util.Notice("fail to place spot order, GetFromStandard: " + symbol)
		return
	}
	ordType := ``
	if orderType == model.OrderTypeMarket {
		ordType = `market`
	} else if orderType == model.OrderTypeLimit {
		ordType = `limit`
	}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]string{
		"symbol":    dialectSymbol,
		"force":     "normal",
		"quantity":  amountStr,
		"price":     priceStr,
		"side":      orderSide,
		"orderType": ordType,
	}
	httpResp, httpErr := client.DoPost("/api/spot/v1/trade/orders", string(util.JsonEncodeToByte(params)))
	bitgetOrderResp := &dtos.BitgetOrderResp{}
	jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
	if bitgetOrderResp == nil {
		util.Notice(fmt.Sprintf("fail to create bitget spot order no resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	} else if bitgetOrderResp.Code == "00000" {
		order.Status = model.CarryStatusWorking
		order.OrderId = bitgetOrderResp.Data.OrderId
	} else {
		order.ErrCode = bitgetOrderResp.Code
		util.Notice(fmt.Sprintf("fail to create bitget spot order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	}
}

func cancelOrdersBitgetSpot(key, secret, symbol string) (result bool) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
	if !success {
		util.Notice("fail to cancel bitget spot order, GetFromStandard: " + symbol)
		return false
	}
	params := map[string]interface{}{
		"symbol": dialectSymbol,
	}
	httpResp, httpErr := client.DoPost("/api/spot/v1/trade/cancel-symbol-order", string(util.JsonEncodeToByte(params)))
	if httpErr != nil {
		util.Notice(fmt.Sprintf(`fail to post when cancelOrdersBitgetSpot %s`, httpErr.Error()))
		return false
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Notice(fmt.Sprintf(`fail to NewJson when cancelOrdersBitgetSpot %s`, jsonErr.Error()))
		return false
	}
	if jsonData != nil {
		code, _ := jsonData.Get("code").String()
		if code == "00000" {
			return true
		}
	}
	return false
}

func queryOrderBitgetSpot(key, secret, symbol string, orderId string) (order *model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
	if !success {
		util.Notice("fail to query bitget spot order, GetFromStandard: " + symbol)
		return order
	}
	order = &model.Order{Market: model.BitgetSpot, Status: model.CarryStatusWorking, OrderId: orderId, Symbol: symbol}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]string{"symbol": dialectSymbol, "orderId": orderId}
	httpResp, httpErr := client.DoPost("/api/spot/v1/trade/orderInfo", string(util.JsonEncodeToByte(params)))
	orderDetailResp := &dtos.BitgetSpotOrderDetailResp{}
	perpJsonErr := json.Unmarshal(httpResp, orderDetailResp)
	if orderDetailResp == nil || orderDetailResp.Code != "00000" {
		util.Notice(fmt.Sprintf("get bitget spot order detail error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
		return order
	} else {
		if len(orderDetailResp.Data) > 0 {
			orderResp := orderDetailResp.Data[0]
			order.DealPrice, _ = strconv.ParseFloat(orderResp.FillPrice, 64)
			order.DealAmount, _ = strconv.ParseFloat(orderResp.FillQuantity, 64)
			intOrderTime, _ := strconv.ParseInt(orderResp.CTime, 10, 64)
			order.OrderTime = time.Unix(intOrderTime, 0)
			amount, _ := strconv.ParseFloat(orderResp.Quantity, 64)
			order.UnfilledQuantity = amount - order.DealAmount
			if orderResp.Status == "cancelled" {
				order.Status = model.CarryStatusFail
			} else if orderResp.Status == "full_fill" || orderResp.Status == "partial_fill" {
				order.Status = model.CarryStatusSuccess
			}
		}
	}
	return order
}

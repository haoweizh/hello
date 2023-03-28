package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/api/dtos"
	"hello/model"
	"hello/util"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const bybitPerpPubWsUrl = "wss://stream.bybit.com/v5/public/linear"

var channelMaintainingBybitPerp = false

func getMarketsBybitPerp() (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	cursor := "init"
	for true {
		param := map[string]interface{}{"category": "linear", "limit": "1000"}
		if cursor != "" && cursor != "init" {
			param["cursor"] = cursor
		}
		composeParams := util.ComposeParams(param)
		httpResp, httpErr := util.HttpRequest(http.MethodGet, bybitRestUrl+"/v5/market/instruments-info?"+composeParams,
			"", map[string]string{}, 30)
		perpResp := &dtos.BybitPerpMarketResp{}
		perpJsonErr := json.Unmarshal(httpResp, perpResp)
		if perpResp == nil || perpResp.RetCode != 0 {
			util.Notice(fmt.Sprintf("get bybit perp market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
			return
		}
		for _, perpInfo := range perpResp.Result.List {
			if perpInfo.Status != "Trading" || perpInfo.QuoteCoin != "USDT" || perpInfo.ContractType != "LinearPerpetual" {
				continue
			}
			symbol := perpInfo.BaseCoin + model.UniStandardTail[model.MarketTypePerp]
			marketInfo := &model.MarketInfo{Name: symbol, Market: model.BybitPerp}
			marketInfo.PriceIncrement, _ = strconv.ParseFloat(perpInfo.PriceFilter.TickSize, 64)
			marketInfo.PriceDecimal, _ = strconv.Atoi(perpInfo.PriceScale)
			marketInfo.PriceMax, _ = strconv.ParseFloat(perpInfo.PriceFilter.MaxPrice, 64)
			priceMin, _ := strconv.ParseFloat(perpInfo.PriceFilter.MinPrice, 64)
			if priceMin != marketInfo.PriceIncrement {
				util.Notice(fmt.Sprintf("最小价格和价格步长不一致 perp info：%v", perpInfo))
				continue
			}
			maxLeverage, _ := strconv.ParseFloat(perpInfo.LeverageFilter.MaxLeverage, 64)
			if maxLeverage < 5 {
				util.Notice(fmt.Sprintf("最大杠杆小于5 perp info：%v", perpInfo))
				continue
			}
			marketInfo.SizeMin, _ = strconv.ParseFloat(perpInfo.LotSizeFilter.MinOrderQty, 64)
			marketInfo.SizeIncrement, _ = strconv.ParseFloat(perpInfo.LotSizeFilter.QtyStep, 64)
			marketInfos[symbol] = marketInfo
		}
		cursor = perpResp.Result.NextPageCursor
		if cursor == "" {
			return marketInfos
		}
	}
	return marketInfos
}

func WsDepthServeBybitPerp(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	perpBookWsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		//fmt.Println(fmt.Sprintf("perp book data: %s", event))
		bookWsResp := &dtos.BybitBookWsResp{}
		jsonErr := json.Unmarshal(event, bookWsResp)
		if jsonErr != nil {
			util.Notice(`fail to unmarshal bybit perp book ws data json ` + jsonErr.Error())
			return
		}
		if strings.Contains(bookWsResp.Topic, "orderbook") {
			parseBookOrderPerp(markets, bookWsResp)
		}
	}

	channels = make([]chan struct{}, 0)
	symbols := GetMarketSymbols(model.BybitPerp)
	futureSubscribes := make([]interface{}, 0)
	for symbol := range symbols {
		futureSubscribes = append(futureSubscribes, symbol)
	}
	perpBookChannels, perpBookErr := WebSocketClient(model.BybitPerp, bybitPerpPubWsUrl,
		futureSubscribes, subscribeHandlerBybitPerp, perpBookWsHandler, orderHandler, 10)
	if perpBookErr == nil {
		util.Notice(`finish connect public bybit perp book wss `)
		channels = append(channels, perpBookChannels...)
	}
	time.Sleep(time.Second * 1)

	go maintainChannelBybitPerp()
	return channels, nil
}

var subscribeHandlerBybitPerp = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	var params []string
	for _, subscribe := range subscribes {
		success, _, _, dialectSymbol := model.GetFromStandard(model.BybitPerp, subscribe.(string))
		if !success {
			continue
		}
		params = append(params, fmt.Sprintf("orderbook.1.%s", dialectSymbol))
	}
	subscribeMap := make(map[string]interface{})
	subscribeMap["req_id"] = int(rand.Float64() * 10000)
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = params
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.BybitPerp, connection, subscribeMessage); err != nil {
		util.Notice(" bybit perp can not subscribe %s %s", subscribeMessage, err.Error())
	}
	util.Notice(`bybit perp subscribed ` + string(subscribeMessage))
	time.Sleep(100 * time.Millisecond)
	return err
}

func maintainChannelBybitPerp() {
	if !channelMaintainingBybitPerp {
		channelMaintainingBybitPerp = true
		go func() {
			for true {
				time.Sleep(time.Second * 20)
				if err := SendToAllConnections(model.BybitPerp, []byte(`{"op": "ping"}`)); err != nil {
					util.Notice("bybit perp channel ping error " + err.Error())
				}
			}
		}()
	}
}

func parseBookOrderPerp(markets *model.Markets, bookWsResp *dtos.BybitBookWsResp) {
	if bookWsResp.Data.S == "" {
		return
	}
	success, _, coin := model.GetCoinFromDialect(model.BybitPerp, bookWsResp.Data.S)
	if !success {
		return
	}
	symbol := coin + model.UniStandardTail[model.MarketTypePerp]
	bidAsk := model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
	bidAsk.Ts = int(bookWsResp.Ts)
	bidAsk.UpdateId = bookWsResp.Data.Seq
	haveOld, old := markets.GetBidAsk(symbol, model.BybitPerp)
	if bookWsResp.Type == "snapshot" {
		bidPrice, _ := strconv.ParseFloat(bookWsResp.Data.B[0][0], 64)
		bidAmount, _ := strconv.ParseFloat(bookWsResp.Data.B[0][1], 64)
		askPrice, _ := strconv.ParseFloat(bookWsResp.Data.A[0][0], 64)
		askAmount, _ := strconv.ParseFloat(bookWsResp.Data.A[0][1], 64)
		bid := model.Tick{Price: bidPrice, Amount: bidAmount}
		ask := model.Tick{Price: askPrice, Amount: askAmount}
		bidAsk.Bids = []model.Tick{bid}
		bidAsk.Asks = []model.Tick{ask}
	} else if bookWsResp.Type == "delta" {
		if !haveOld {
			util.Notice(fmt.Sprintf("币种：%s bidask没有bidask 却收到delta ws", symbol))
			return
		}
		oldBid := old.Bids[0]
		oldAsk := old.Asks[0]
		if len(bookWsResp.Data.B) == 0 {
			bidAsk.Bids = []model.Tick{oldBid}
		} else {
			for _, bidStr := range bookWsResp.Data.B {
				bidAmount, _ := strconv.ParseFloat(bidStr[1], 64)
				if bidAmount == 0 {
					continue
				}
				bidPrice, _ := strconv.ParseFloat(bidStr[0], 64)
				bid := model.Tick{Price: bidPrice, Amount: bidAmount}
				bidAsk.Bids = []model.Tick{bid}
			}
		}
		if len(bookWsResp.Data.A) == 0 {
			bidAsk.Asks = []model.Tick{oldAsk}
		} else {
			for _, askStr := range bookWsResp.Data.A {
				askAmount, _ := strconv.ParseFloat(askStr[1], 64)
				askPrice, _ := strconv.ParseFloat(askStr[0], 64)
				if askAmount == 0 {
					continue
				}
				ask := model.Tick{Price: askPrice, Amount: askAmount}
				bidAsk.Asks = []model.Tick{ask}
			}
		}
	} else {
		return
	}
	if haveOld && old.Ts > bidAsk.Ts {
		return
	}
	if markets.SetBidAsk(symbol, model.BybitPerp, &bidAsk) {
		funcHandlers := GetFunctions(model.BybitPerp, symbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), model.BybitPerp, symbol)
				if setting != nil && value != nil {
					go value.(model.CarryHandler)(setting, &bidAsk)
				}
				return true
			})
		}
	}
}

func getPositionsBybitPerp(key, secret string) (success bool, positions []*model.Position, posBalance float64) {
	cursor := "init"
	positions = make([]*model.Position, 0)
	for true {
		param := map[string]interface{}{"category": "linear", "settleCoin": "USDT", "limit": "200"}
		if cursor != "" && cursor != "init" {
			param["cursor"] = cursor
		}
		positionHttpResp, positionHttpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/position/list", param)
		positionResp := &dtos.BybitPositionResp{}
		positionJsonErr := json.Unmarshal(positionHttpResp, positionResp)
		if positionResp == nil || positionResp.RetCode != 0 {
			util.Notice(fmt.Sprintf("fail to refresh perp position bybit, resp: %s httpErr: %v, jsonErr: %v", positionHttpResp, positionHttpErr, positionJsonErr))
			time.Sleep(time.Second * 2)
			return getPositionsBybitPerp(key, secret)
		} else {
			util.SocketInfo(fmt.Sprintf("get perp position bybit success, resp: %s ", positionHttpResp))
		}

		for _, contract := range positionResp.Result.List {
			if contract.TradeMode != 0 {
				continue
			}
			_, _, currency := model.GetCoinFromDialect(model.BybitPerp, contract.Symbol)
			symbol := currency + model.UniStandardTail[model.MarketTypePerp]
			position := &model.Position{Market: model.BybitPerp, Ts: util.GetNowUnixMillion(), Currency: symbol}
			if contract.Side == "Buy" {
				position.Holding, _ = strconv.ParseFloat(contract.Size, 64)
			} else if contract.Side == "Sell" {
				total, _ := strconv.ParseFloat(contract.Size, 64)
				position.Holding = -1 * total
			} else {
				position.Holding = 0
			}
			position.LeverRate, _ = strconv.ParseInt(contract.Leverage, 10, 64)
			position.EntryPrice, _ = strconv.ParseFloat(contract.AvgPrice, 64)
			position.BankruptcyPrice, _ = strconv.ParseFloat(contract.BustPrice, 64)
			position.LiquidationPrice, _ = strconv.ParseFloat(contract.LiqPrice, 64)
			position.Margin, _ = strconv.ParseFloat(contract.PositionMM, 64)
			positions = append(positions, position)
		}

		cursor = positionResp.Result.NextPageCursor
		if cursor == "" {
			return true, positions, 0
		}
	}
	return true, positions, 0
}

func setBybitPerpLeverage(key, secret string) {
	symbols := GetMarketSymbols(model.BybitPerp)
	for symbol, _ := range symbols {
		success, _, _, dialectSymbol := model.GetFromStandard(model.BybitPerp, symbol)
		if !success {
			continue
		}
		params := map[string]interface{}{"category": "linear", "buyLeverage": "5", "sellLeverage": "5", "symbol": dialectSymbol}
		httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/position/set-leverage", params)
		jsonData, jsonErr := util.NewJSON(httpResp)
		code, _ := jsonData.Get("retCode").Int()
		if jsonData == nil || code != 0 {
			util.Notice(fmt.Sprintf("fail to set bybit perp leverage , resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func placeOrderBybitPerp(key, secret string, order *model.Order, orderSide, orderType, orderParam, symbol string, price, amount float64) {
	reduceOnly := false
	if orderParam == model.ReduceOnly {
		reduceOnly = true
	}
	priceSpot, decimalSpot := model.FormatPrice(model.BybitPerp, symbol, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.BybitPerp, symbol, amount, priceSpot, reduceOnly)))
	success, _, _, dialectSymbol := model.GetFromStandard(model.BybitPerp, symbol)
	if !success {
		util.Notice("fail to place bybit perp order, GetFromStandard: " + symbol)
		return
	}
	var tradeSide, tradeOrderType string
	if orderSide == model.OrderSideBuy {
		tradeSide = "Buy"
	} else {
		tradeSide = "Sell"
	}
	if orderType == model.OrderTypeLimit {
		tradeOrderType = "Limit"
	} else if orderType == model.OrderTypeMarket {
		tradeOrderType = "Market"
	}
	param := map[string]interface{}{
		"category":  "linear",
		"symbol":    dialectSymbol,
		"side":      tradeSide,
		"orderType": tradeOrderType,
		"qty":       amountStr,
		"price":     priceStr,
	}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/create", param)
	bitgetOrderResp := &dtos.BybitOrderResp{}
	jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
	if bitgetOrderResp == nil || bitgetOrderResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("fail to create bybit perp order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	} else {
		order.Status = model.CarryStatusWorking
		order.OrderId = bitgetOrderResp.Result.OrderId
	}
}

func cancelOrdersBybitPerp(key, secret, symbol string) (result bool) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BybitPerp, symbol)
	if !success {
		util.Notice("fail to cancel bybit perp order, GetFromStandard: " + symbol)
		return
	}
	params := map[string]interface{}{
		"symbol":   dialectSymbol,
		"category": "linear",
	}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/cancel-all", params)
	jsonData, jsonErr := util.NewJSON(httpResp)
	code, _ := jsonData.Get("code").Int64()
	if jsonData == nil || code != 0 {
		util.Notice(fmt.Sprintf("fail to cancel bybit perp order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		return false
	}
	return true
}

func getFundingRateBybitPerp(symbol string) (fundingRate *model.FundingRate) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BybitPerp, symbol)
	if !success {
		util.Notice("fail to get bybit perp funding rate, GetFromStandard: " + symbol)
		return
	}
	param := map[string]interface{}{"category": "linear", "symbol": dialectSymbol}
	composeParams := util.ComposeParams(param)
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bybitRestUrl+"/v5/market/tickers?"+composeParams, "", map[string]string{}, 30)
	bybitTickersResp := &dtos.BybitTickersResp{}
	perpJsonErr := json.Unmarshal(httpResp, bybitTickersResp)
	if bybitTickersResp == nil || bybitTickersResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("get bybit perp funding rate error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
		return
	}
	for _, ticker := range bybitTickersResp.Result.List {
		rate, _ := strconv.ParseFloat(ticker.FundingRate, 64)
		nextFundingTime, _ := strconv.ParseInt(ticker.NextFundingTime, 10, 64)
		fundingRate = &model.FundingRate{
			Rate:       rate,
			UpdateTime: util.GetNow(),
			ExpireTime: nextFundingTime / 1000,
		}
	}
	return fundingRate
}

func queryOrderBybitPerp(key, secret, symbol, orderId string) *model.Order {
	param := map[string]interface{}{"orderId": orderId, "category": "linear"}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/order/history", param)
	orderResp := &dtos.BybitOrderDetailResp{}
	jsonErr := json.Unmarshal(httpResp, orderResp)
	if orderResp == nil || orderResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("get bybit perp order detail error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		return nil
	}
	order := &model.Order{Market: model.BybitPerp, Status: model.CarryStatusWorking, OrderId: orderId, Symbol: symbol}
	for _, orderDetail := range orderResp.Result.List {
		order.DealPrice, _ = strconv.ParseFloat(orderDetail.AvgPrice, 64)
		order.DealAmount, _ = strconv.ParseFloat(orderDetail.CumExecQty, 64)
		order.UnfilledQuantity, _ = strconv.ParseFloat(orderDetail.LeavesQty, 64)
		if orderDetail.OrderStatus == "Cancelled" || orderDetail.OrderStatus == "Rejected" {
			order.Status = model.CarryStatusFail
		} else if orderDetail.OrderStatus == "Filled" || orderDetail.OrderStatus == "PartiallyFilled" || orderDetail.OrderStatus == "PartiallyFilledCanceled" {
			order.Status = model.CarryStatusSuccess
		} else {
			util.Notice(fmt.Sprintf("unkown bybit perp order detail status, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		}
	}
	return order
}

package deprecated

//
//import (
//	"crypto/hmac"
//	"crypto/sha256"
//	"encoding/hex"
//	"encoding/json"
//	"fmt"
//	"github.com/gorilla/websocket"
//	"hello/api/dtos"
//	"hello/model"
//	"hello/util"
//	"math"
//	"math/rand"
//	"net/http"
//	"strconv"
//	"strings"
//	"time"
//)
//
//const bybitRestUrl = "https://api.bybit.com"
//const bybitSpotPubWsUrl = "wss://stream.bybit.com/v5/public/spot"
//
//var channelMaintainingBybitSpot = false
//
//func getMarketsBybitSpot() (marketInfos map[string]*model.MarketInfo) {
//	param := map[string]interface{}{"category": "spot", "limit": "1000"}
//	composeParams := util.ComposeParams(param)
//	httpResp, httpErr := util.HttpRequest(http.MethodGet, bybitRestUrl+"/v5/market/instruments-info?"+composeParams,
//		"", map[string]string{}, 30)
//	spotResp := &dtos.BybitSpotMarketResp{}
//	spotJsonErr := json.Unmarshal(httpResp, spotResp)
//	if spotResp == nil || spotResp.RetCode != 0 {
//		util.Notice(fmt.Sprintf("get bybit spot market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, spotJsonErr))
//		return
//	}
//	marketInfos = make(map[string]*model.MarketInfo)
//	for _, symbolInfo := range spotResp.Result.List {
//		if symbolInfo.Status != "Trading" && symbolInfo.QuoteCoin != "USDT" {
//			continue
//		}
//		symbol := symbolInfo.BaseCoin + model.UniStandardTail[model.MarketTypeSpot]
//		marketInfo := &model.MarketInfo{Name: symbol, Market: model.BybitSpot}
//		if symbolInfo.PriceFilter.TickSize == "" {
//			util.Notice(fmt.Sprintf("币种：%s 价格步长为空 resp：%v", symbol, symbolInfo))
//			continue
//		}
//		priceIncrement, _ := strconv.ParseFloat(symbolInfo.PriceFilter.TickSize, 64)
//		marketInfo.PriceIncrement = priceIncrement
//		marketInfo.PriceDecimal = util.NumDecPlaces(priceIncrement)
//
//		sizeIncrement, _ := strconv.ParseFloat(symbolInfo.LotSizeFilter.BasePrecision, 64)
//		marketInfo.SizeIncrement = sizeIncrement
//		marketInfo.SizeMin, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MinOrderQty, 64)
//		marketInfo.SizeMax, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MaxOrderQty, 64)
//		marketInfo.MoneyMin, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MinOrderAmt, 64)
//		marketInfo.MoneyMax, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MaxOrderAmt, 64)
//		marketInfos[marketInfo.Name] = marketInfo
//	}
//	return marketInfos
//}
//
//func WsDepthServeBybitSpot(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
//	spotBookWsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
//		//fmt.Println(fmt.Sprintf("spot book data: %s", event))
//		bookWsResp := &dtos.BybitBookWsResp{}
//		jsonErr := json.Unmarshal(event, bookWsResp)
//		if jsonErr != nil {
//			util.Notice(`fail to unmarshal bybit spot book ws data json ` + jsonErr.Error())
//			return
//		}
//		if strings.Contains(bookWsResp.Topic, "orderbook") {
//			parseBookOrderSpot(markets, bookWsResp)
//		}
//	}
//	channels = make([]chan struct{}, 0)
//	symbols := GetMarketSymbols(model.BybitSpot)
//	spotSubscribes := make([]interface{}, 0)
//	for symbol := range symbols {
//		spotSubscribes = append(spotSubscribes, symbol)
//	}
//	spotBookChannels, spotBookErr := WebSocketClient(model.BybitSpot, bybitSpotPubWsUrl,
//		spotSubscribes, subscribeHandlerBybitSpot, spotBookWsHandler, orderHandler, 10)
//	if spotBookErr == nil {
//		util.Notice(`finish connect public bybit spot book wss `)
//		channels = append(channels, spotBookChannels...)
//	}
//	time.Sleep(time.Second * 1)
//	go maintainChannelBybitSpot()
//	return channels, nil
//}
//
//var subscribeHandlerBybitSpot = func(connection *websocket.Conn, subscribes []interface{}) error {
//	var err error = nil
//	var params []string
//	for _, subscribe := range subscribes {
//		success, _, _, dialectSymbol := model.GetFromStandard(model.BybitSpot, subscribe.(string))
//		if !success {
//			continue
//		}
//		params = append(params, fmt.Sprintf("orderbook.1.%s", dialectSymbol))
//	}
//	subscribeMap := make(map[string]interface{})
//	subscribeMap["req_id"] = int(rand.Float64() * 100000)
//	subscribeMap["op"] = "subscribe"
//	subscribeMap["args"] = params
//	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
//	if err = SendToConnection(model.BybitSpot, connection, subscribeMessage); err != nil {
//		util.Notice(" bybit spot can not subscribe %s %s", subscribeMessage, err.Error())
//	}
//	util.Notice(`bybit spot subscribed ` + string(subscribeMessage))
//	time.Sleep(100 * time.Millisecond)
//	return err
//}
//
//func maintainChannelBybitSpot() {
//	if !channelMaintainingBybitSpot {
//		channelMaintainingBybitSpot = true
//		go func() {
//			for true {
//				time.Sleep(time.Second * 20)
//				if err := SendToAllConnections(model.BybitSpot, []byte(`{"op": "ping"}`)); err != nil {
//					util.Notice("bybit spot channel ping error " + err.Error())
//				}
//			}
//		}()
//	}
//}
//
//func parseBookOrderSpot(markets *model.Markets, bookWsResp *dtos.BybitBookWsResp) {
//	if bookWsResp.Data.S == "" {
//		return
//	}
//	success, _, coin := model.GetCoinFromDialect(model.BybitSpot, bookWsResp.Data.S)
//	if !success {
//		return
//	}
//	symbol := coin + model.UniStandardTail[model.MarketTypeSpot]
//	bidAsk := model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
//	bidAsk.Ts = int(bookWsResp.Ts)
//	bidAsk.UpdateId = bookWsResp.Data.Seq
//	haveOld, old := markets.GetBidAsk(symbol, model.BybitSpot)
//	if bookWsResp.Type == "snapshot" {
//		bidPrice, _ := strconv.ParseFloat(bookWsResp.Data.B[0][0], 64)
//		bidAmount, _ := strconv.ParseFloat(bookWsResp.Data.B[0][1], 64)
//		askPrice, _ := strconv.ParseFloat(bookWsResp.Data.A[0][0], 64)
//		askAmount, _ := strconv.ParseFloat(bookWsResp.Data.A[0][1], 64)
//		bid := model.Tick{Price: bidPrice, Amount: bidAmount}
//		ask := model.Tick{Price: askPrice, Amount: askAmount}
//		bidAsk.Bids = []model.Tick{bid}
//		bidAsk.Asks = []model.Tick{ask}
//	} else if bookWsResp.Type == "delta" {
//		if !haveOld {
//			util.Notice(fmt.Sprintf("币种：%s bidask没有bidask 却收到delta ws", symbol))
//			return
//		}
//		oldBid := old.Bids[0]
//		oldAsk := old.Asks[0]
//		if len(bookWsResp.Data.B) == 0 {
//			bidAsk.Bids = []model.Tick{oldBid}
//		} else {
//			for _, bidStr := range bookWsResp.Data.B {
//				bidAmount, _ := strconv.ParseFloat(bidStr[1], 64)
//				if bidAmount == 0 {
//					continue
//				}
//				bidPrice, _ := strconv.ParseFloat(bidStr[0], 64)
//				bid := model.Tick{Price: bidPrice, Amount: bidAmount}
//				bidAsk.Bids = []model.Tick{bid}
//			}
//		}
//		if len(bookWsResp.Data.A) == 0 {
//			bidAsk.Asks = []model.Tick{oldAsk}
//		} else {
//			for _, askStr := range bookWsResp.Data.A {
//				askAmount, _ := strconv.ParseFloat(askStr[1], 64)
//				askPrice, _ := strconv.ParseFloat(askStr[0], 64)
//				if askAmount == 0 {
//					continue
//				}
//				ask := model.Tick{Price: askPrice, Amount: askAmount}
//				bidAsk.Asks = []model.Tick{ask}
//			}
//		}
//	} else {
//		return
//	}
//	if haveOld && old.Ts > bidAsk.Ts {
//		return
//	}
//	if markets.SetBidAsk(symbol, model.BybitSpot, &bidAsk) {
//		funcHandlers := GetFunctions(model.BybitSpot, symbol)
//		if funcHandlers != nil {
//			funcHandlers.Range(func(function, value interface{}) bool {
//				setting := GetSetting(function.(string), model.BybitSpot, symbol)
//				if setting != nil && value != nil {
//					go value.(model.CarryHandler)(setting, &bidAsk)
//				}
//				return true
//			})
//		}
//	}
//}
//
//func getBalanceBybitSpot(key string, secret string) (success bool, balances []*model.Balance, totalInUsd float64, collateral *model.Collateral) {
//	coins := GetSettingCoins(model.FunctionCross, model.BybitSpot)
//	coinsStr := []string{"USDT"}
//	for coin := range coins {
//		coinsStr = append(coinsStr, coin)
//	}
//	param := map[string]interface{}{"accountType": "UNIFIED", "coin": strings.Join(coinsStr, ",")}
//	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/account/wallet-balance", param)
//	balanceResp := &dtos.BybitBalanceResp{}
//	jsonErr := json.Unmarshal(httpResp, balanceResp)
//	if balanceResp == nil || balanceResp.RetCode != 0 {
//		util.Notice(fmt.Sprintf("fail to refresh spot balance bybit, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
//		time.Sleep(time.Second * 2)
//		return getBalanceBybitSpot(key, secret)
//	} else {
//		util.SocketInfo(fmt.Sprintf("get spot balance bybit success, resp: %s ", httpResp))
//	}
//	balances = make([]*model.Balance, 0)
//	for _, account := range balanceResp.Result.List {
//		if account.AccountType == "UNIFIED" {
//			collateralAvailable, _ := strconv.ParseFloat(account.TotalAvailableBalance, 64)
//			totalMaintenanceMargin, _ := strconv.ParseFloat(account.TotalMaintenanceMargin, 64)
//			accountMMRate, _ := strconv.ParseFloat(account.AccountMMRate, 64)
//			collateral = &model.Collateral{Available: collateralAvailable, Occupied: totalMaintenanceMargin, Rate: accountMMRate}
//			for _, coinInfo := range account.Coin {
//				balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.BybitSpot, Coin: coinInfo.Coin}
//				balance.Borrow, _ = strconv.ParseFloat(coinInfo.BorrowAmount, 64)
//				canBorrow, _ := strconv.ParseFloat(coinInfo.AvailableToBorrow, 64)
//				holdAmount, _ := strconv.ParseFloat(coinInfo.WalletBalance, 64)
//				if coinInfo.Coin == "USDT" {
//					holdAmount, _ = strconv.ParseFloat(coinInfo.AvailableToWithdraw, 64)
//				}
//				balance.Amount = holdAmount
//				balance.AvailableWithBorrow = math.Max(0, balance.Amount) + canBorrow
//				usdValue, _ := strconv.ParseFloat(coinInfo.UsdValue, 64)
//				if usdValue == 0 {
//					priceGet, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.BybitSpot)
//					if priceGet {
//						usdValue = balance.Amount * bidAsk.Bids[0].Price
//					}
//				}
//				balance.UsdValue = usdValue
//				// 該幣種不能作為保證金的抵押品，則該數值為0
//				totalInUsd += usdValue
//				balances = append(balances, balance)
//			}
//		}
//	}
//	return true, balances, totalInUsd, collateral
//}
//
//func SignedRequestBybit(key, secret, method, host, path string, body map[string]interface{}) ([]byte, error) {
//	if body == nil {
//		body = make(map[string]interface{})
//	}
//	receiveWindow := "5000"
//	timestamp := strconv.FormatInt(util.GetNowUnixMillion(), 10)
//	hash := hmac.New(sha256.New, []byte(secret))
//	header := map[string]string{
//		"X-BAPI-API-KEY":     key,
//		"X-BAPI-TIMESTAMP":   timestamp,
//		"X-BAPI-RECV-WINDOW": receiveWindow,
//	}
//	if method == http.MethodGet {
//		composeParams := util.ComposeParams(body)
//		hash.Write([]byte(timestamp + key + receiveWindow + composeParams))
//		sign := hex.EncodeToString(hash.Sum(nil))
//		header["X-BAPI-SIGN"] = sign
//		httpResp, httpErr := util.HttpRequest(http.MethodGet, host+path+"?"+composeParams, "", header, 30)
//		return httpResp, httpErr
//	} else if method == http.MethodPost {
//		jsonParams := string(util.JsonEncodeToByte(body))
//		hash.Write([]byte(timestamp + key + receiveWindow + jsonParams))
//		sign := hex.EncodeToString(hash.Sum(nil))
//		header["X-BAPI-SIGN"] = sign
//		header["Content-Type"] = "application/json"
//		httpResp, httpErr := util.HttpRequest(http.MethodPost, host+path, jsonParams, header, 30)
//		return httpResp, httpErr
//	}
//	return nil, http.ErrNoLocation
//}
//
//func placeOrderBybitSpot(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
//	priceSpot, decimalSpot := model.FormatPrice(model.BybitSpot, symbol, price)
//	priceStr := util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
//	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.BybitSpot, symbol, amount, priceSpot, false)))
//	success, _, _, dialectSymbol := model.GetFromStandard(model.BybitSpot, symbol)
//	if !success {
//		util.Notice("fail to place bybit spot order, GetFromStandard: " + symbol)
//		return
//	}
//	var tradeSide, tradeOrderType string
//	if orderSide == model.OrderSideBuy {
//		tradeSide = "Buy"
//	} else {
//		tradeSide = "Sell"
//	}
//	if orderType == model.OrderTypeLimit {
//		tradeOrderType = "Limit"
//	} else if orderType == model.OrderTypeMarket {
//		tradeOrderType = "Market"
//	}
//	param := map[string]interface{}{
//		"category":  "spot",
//		"symbol":    dialectSymbol,
//		"side":      tradeSide,
//		"orderType": tradeOrderType,
//		"qty":       amountStr,
//		"price":     priceStr,
//	}
//	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/create", param)
//	bitgetOrderResp := &dtos.BybitOrderResp{}
//	jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
//	if bitgetOrderResp == nil || bitgetOrderResp.RetCode != 0 {
//		util.Notice(fmt.Sprintf("fail to create bybit spot order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
//	} else {
//		order.Status = model.CarryStatusWorking
//		order.OrderId = bitgetOrderResp.Result.OrderId
//	}
//}
//
//func cancelOrdersBybitSpot(key, secret, symbol string) (result bool) {
//	success, _, _, dialectSymbol := model.GetFromStandard(model.BybitSpot, symbol)
//	if !success {
//		util.Notice("fail to cancel bybit spot order, GetFromStandard: " + symbol)
//		return
//	}
//	params := map[string]interface{}{
//		"symbol":   dialectSymbol,
//		"category": "spot",
//	}
//	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/cancel-all", params)
//	jsonData, jsonErr := util.NewJSON(httpResp)
//	code, _ := jsonData.Get("code").Int64()
//	if jsonData == nil || code != 0 {
//		util.Notice(fmt.Sprintf("fail to cancel bybit spot order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
//		return false
//	}
//	return true
//}
//
//func queryOrderBybitSpot(key, secret, symbol, orderId string) *model.Order {
//	param := map[string]interface{}{"orderId": orderId, "category": "spot"}
//	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/order/history", param)
//	orderResp := &dtos.BybitOrderDetailResp{}
//	jsonErr := json.Unmarshal(httpResp, orderResp)
//	if orderResp == nil || orderResp.RetCode != 0 {
//		util.Notice(fmt.Sprintf("get bybit spot order detail error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
//		return nil
//	}
//	order := &model.Order{Market: model.BybitSpot, Status: model.CarryStatusWorking, OrderId: orderId, Symbol: symbol}
//	for _, orderDetail := range orderResp.Result.List {
//		order.DealPrice, _ = strconv.ParseFloat(orderDetail.AvgPrice, 64)
//		order.DealAmount, _ = strconv.ParseFloat(orderDetail.CumExecQty, 64)
//		order.UnfilledQuantity, _ = strconv.ParseFloat(orderDetail.LeavesQty, 64)
//		if orderDetail.OrderStatus == "Cancelled" || orderDetail.OrderStatus == "Rejected" {
//			order.Status = model.CarryStatusFail
//		} else if orderDetail.OrderStatus == "Filled" || orderDetail.OrderStatus == "PartiallyFilled" || orderDetail.OrderStatus == "PartiallyFilledCanceled" {
//			order.Status = model.CarryStatusSuccess
//		} else {
//			util.Notice(fmt.Sprintf("unkown bybit spot order detail status, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
//		}
//	}
//	return order
//}

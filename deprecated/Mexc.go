package deprecated

//
//import (
//	"crypto/hmac"
//	"crypto/sha256"
//	"encoding/hex"
//	"encoding/json"
//	"errors"
//	"fmt"
//	"hello/api"
//	"net/http"
//	"net/url"
//	"sort"
//	"strconv"
//	"sync"
//	"time"
//
//	"github.com/gorilla/websocket"
//
//	"hello/api/dtos"
//	"hello/model"
//	"hello/util"
//)
//
//const (
//	spotRestUrl                  = "www.mexc.com"
//	contractRestUrl              = "contract.mexc.com"
//	mexcContractWSUrl            = "wss://contract.mexc.com/ws"
//	mexcContractDepthIncSubType  = "mexcContractDepthIncreSubType" // Contract深度增量订阅
//	mexcContractDepthFullSubType = "mexcContractDepthFullSubType"  // Contract深度全量订阅（按档位）
//	mexcContractTickerSubType    = "mexcContractTickerSubType"
//	wsStepMexc                   = 20
//)
//
//// spot rest api path
//const (
//	spotPlaceOrderPath               = "/open/api/v2/order/place"            // 下单
//	spotCancelOrdersBySymbolPath     = "/open/api/v2/order/cancel_by_symbol" // 按交易对撤销订单
//	spotQueryOrderByIdPath           = "/open/api/v2/order/query"            // 查询订单
//	spot_get_all_market_symbols_path = "/open/api/v2/market/symbols"         // 所有交易对信息
//)
//
//// contract rest api path
//const (
//	contractPlaceOrderPath               = "/api/v1/private/order/submit"         // 下单
//	contractCancelOrdersBySymbolPath     = "/api/v1/private/order/cancel_all"     // 撤销某个合约下的所有未完成订单
//	contractQueryOrderByIdPath           = "api/v1/private/order/get"             // 根据订单号查询订单
//	contractGetSymbolMarketPath          = "/api/v1/contract/detail"              // 获取合约信息
//	contractGetSymbolDepthPathFmt        = "/api/v1/contract/depth/%s"            // 获取合约深度信息
//	contractGetSymbolDepthCommitsPathFmt = "/api/v1/contract/depth_commits/%s/%d" // 获取合约最近N条深度信息快照
//)
//
//var (
//	emptySymbolError        = errors.New("symbol is empty")
//	zeroPriceError          = errors.New("price is 0")
//	zeroAmountError         = errors.New("amount is 0")
//	failedToPlaceOrderError = errors.New("failed to place order")
//)
//
//var lastTickIdMexc sync.Map
//
//func SignedRequestMexc(key, secret, method, restUrl, path string, paramsInQuery map[string]interface{}, body string) ([]byte, error) {
//	var parameters string
//	if len(paramsInQuery) > 0 {
//		param := &url.Values{}
//		for k, v := range paramsInQuery {
//			param.Set(k, v.(string))
//		}
//		parameters = param.Encode()
//	}
//	reqTime := strconv.FormatInt(util.GetNow().UnixNano(), 10)[0:13]
//	var sign string
//	if method == http.MethodPost {
//		sign = genSign(key, secret, reqTime, body)
//	} else {
//		sign = genSign(key, secret, reqTime, parameters)
//	}
//	headers := map[string]string{
//		"Request-Time": reqTime,
//		"ApiKey":       key,
//		"signature":    sign,
//		"Content-Type": "application/json",
//		"Accept":       "application/json",
//	}
//	// util.ComposeParams(paramsInQuery)
//	requestUrl := fmt.Sprintf(`https://%s%s`, restUrl, path)
//	if len(parameters) > 0 {
//		requestUrl = requestUrl + "?" + parameters
//	}
//	responseBody, err := util.HttpRequest(method, requestUrl, body, headers, 60)
//	logMsg := fmt.Sprintf(`Mexc key %s, request %s, headers %v, parameters %s, body %s, return %s err %+v`,
//		key, requestUrl, headers, parameters, body, string(responseBody), err)
//	util.SocketInfo(logMsg)
//	return responseBody, err
//}
//
//func genSign(key, secret, reqTime, parameters string) string {
//	toBeSign := key + reqTime + parameters
//	hash := hmac.New(sha256.New, []byte(secret))
//	hash.WriteServe([]byte(toBeSign))
//	return hex.EncodeToString(hash.Sum(nil))
//}
//
//func publicRequestMexc(method, restUrl, path string, paramsInQuery map[string]interface{}, body string) ([]byte, error) {
//	var parameters string
//	if len(paramsInQuery) > 0 {
//		param := &url.Values{}
//		for k, v := range paramsInQuery {
//			param.Set(k, v.(string))
//		}
//		parameters = param.Encode()
//	}
//	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
//	// util.ComposeParams(paramsInQuery)
//	requestUrl := fmt.Sprintf(`https://%s%s`, restUrl, path)
//	if len(parameters) > 0 {
//		requestUrl = requestUrl + "?" + parameters
//	}
//	responseBody, err := util.HttpRequest(method, requestUrl, body, headers, 60)
//	logMsg := fmt.Sprintf(`Mexc request %s headers %v parameters %s return %s err %+v`, requestUrl, headers, parameters, string(responseBody), err)
//	util.SocketInfo(logMsg)
//	return responseBody, err
//}
//
//// region cancel orders
//func cancelOrdersMexc(key string, secret string, symbol string) (result bool) {
//	if util.EndWith(symbol, model.UniStandardTail[model.MarketTypePerp]) {
//		return contractCancelOrdersMexc(key, secret, symbol)
//	} else {
//		return spotCancelOrdersMexc(key, secret, symbol)
//	}
//}
//
//func spotCancelOrdersMexc(key string, secret string, symbol string) bool {
//	paramsInQuery := map[string]interface{}{"symbol": symbol}
//	responseBytes, err := SignedRequestMexc(key, secret, http.MethodDelete, spotRestUrl, spotCancelOrdersBySymbolPath, paramsInQuery, "")
//	if err != nil {
//		logMsg := fmt.Sprintf(`[spotCancelOrdersMexc] Failed to cancel orders by symbol %s err %+v`, symbol, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return false
//	}
//	response := &dtos.MexcSpotCancelOrderBySymbolResp{}
//	err = json.Unmarshal(responseBytes, response)
//	if err != nil || response.Code != http.StatusOK {
//		logMsg := fmt.Sprintf(`[spotCancelOrdersMexc] Failed to cancel orders by symbol %s statusCode %d err %+v`, symbol, response.Code, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return false
//	}
//	failedOrderIDs := response.GetFailedOrderIDsMexc()
//	if len(failedOrderIDs) > 0 {
//		logMsg := fmt.Sprintf(`[spotCancelOrdersMexc] Failed to cancel orders %+v by symbol %s`, failedOrderIDs, symbol)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return false
//	}
//
//	return true
//}
//
//func contractCancelOrdersMexc(key string, secret string, symbol string) bool {
//	body := fmt.Sprintf(`{"symbol":"%s"}`, symbol)
//	responseBytes, err := SignedRequestMexc(key, secret, http.MethodPost, contractRestUrl, contractCancelOrdersBySymbolPath, nil, body)
//	if err != nil {
//		logMsg := fmt.Sprintf(`[contractCancelOrdersMexc] Failed to cancel orders by symbol %s err %+v`, symbol, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return false
//	}
//	response := &dtos.MexcContractCancelOrderBySymbolResp{}
//	err = json.Unmarshal(responseBytes, response)
//	if err != nil || !response.Success {
//		logMsg := fmt.Sprintf(`[contractCancelOrdersMexc] Failed to cancel orders by symbol %s response %v err %+v`, symbol, response, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return false
//	}
//	return true
//}
//
//// endregion
//func queryOrderMexc(key, secret string, order *model.Order) {
//	if order.Market != model.Mexc || order.OrderId == "" {
//		return
//	}
//	if util.EndWith(order.Symbol, model.MarketTypePerp) {
//		contractQueryOrderMexc(key, secret, order)
//	} else {
//		spotQueryOrderMexc(key, secret, order)
//	}
//}
//
//func spotQueryOrderMexc(key, secret string, order *model.Order) {
//	if order.OrderId == "" {
//		return
//	}
//	paramsInQuery := map[string]interface{}{"order_ids": order.OrderId}
//	respBytes, err := SignedRequestMexc(key, secret, http.MethodGet, spotRestUrl, spotQueryOrderByIdPath, paramsInQuery, "")
//	if err != nil {
//		logMsg := fmt.Sprintf(`[spotQueryOrderMexc] Failed to query orders by order_id %s err %+v`, order.OrderId, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return
//	}
//	resp := &dtos.MexcSpotQueryOrderResp{}
//	err = json.Unmarshal(respBytes, resp)
//	if err != nil || resp.Code != http.StatusOK {
//		logMsg := fmt.Sprintf(`[spotQueryOrderMexc] Failed to query orders by order_id %s statusCode %d err %+v`, order.OrderId, resp.Code, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return
//	}
//	if len(resp.Data) == 1 {
//		success, marketType, coin := model.GetCoinFromDialect(model.Mexc, resp.Data[0].Symbol)
//		if success { // TODO 需要确保Mexc永续和现货tail不同，否则marketType不可用
//			order.Symbol = coin + model.UniStandardTail[marketType]
//		}
//		order.Price = getFloat64OrDefault(resp.Data[0].Price)             // 挂单价格
//		order.FrozenQuantity = getFloat64OrDefault(resp.Data[0].Quantity) // 挂单数量
//		order.DealPrice = getFloat64OrDefault(resp.Data[0].DealAmount)    // 成交金额
//		order.DealAmount = getFloat64OrDefault(resp.Data[0].DealQuantity) // 成交数量
//		order.CreatedAt = time.Unix(resp.Data[0].CreateTime, 0)
//		order.Status = resp.Data[0].State
//		order.OrderType = resp.Data[0].Type
//		return
//	}
//	logMsg := fmt.Sprintf(`[spotQueryOrderMexc] Get %d orders by order_id %s`, len(resp.Data), order.OrderId)
//	util.SocketInfo(logMsg)
//}
//
//func contractQueryOrderMexc(key, secret string, order *model.Order) {
//	if order.OrderId == "" {
//		return
//	}
//	paramsInQuery := map[string]interface{}{"order_id": order.OrderId}
//	respBytes, err := SignedRequestMexc(key, secret, http.MethodGet, contractRestUrl, contractQueryOrderByIdPath, paramsInQuery, "")
//	if err != nil {
//		logMsg := fmt.Sprintf(`[contractQueryOrderMexc] Failed to query orders by order_id %s err %+v`, order.OrderId, err)
//		util.Notice(logMsg)
//		fmt.Println(logMsg)
//		return
//	}
//	resp := &dtos.MexcContractQueryOrderResp{}
//	err = json.Unmarshal(respBytes, resp)
//	if err != nil || !resp.Success {
//		logMsg := fmt.Sprintf(`[contractQueryOrderMexc] Failed to query orders by order_id %s success %t statusCode %d err %+v`, order.OrderId, resp.Success, resp.Code, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return
//	}
//	success, marketType, coin := model.GetCoinFromDialect(model.Mexc, resp.Data.Symbol)
//	if success { // TODO 需要确保Mexc永续和现货tail不同，否则marketType不可用
//		order.Symbol = coin + model.UniStandardTail[marketType]
//	}
//	order.Price = resp.Data.Price            // 挂单价格
//	order.FrozenQuantity = resp.Data.Vol     // 挂单数量
//	order.DealPrice = resp.Data.DealAvgPrice // 成交金额
//	order.DealAmount = resp.Data.DealVol     // 成交数量
//	order.CreatedAt = time.Unix(resp.Data.CreateTime, 0)
//	order.Status = strconv.Itoa(int(resp.Data.State))
//	order.OrderType = strconv.Itoa(int(resp.Data.OrderType))
//}
//
//func getPositionsMexc(key string, secret string) (success bool, positions []*model.Position, posBalance float64) {
//	return
//}
//
//// region place order
//func placeOrderMexc(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) (orderId string, err error) {
//	if symbol == "" {
//		return "", emptySymbolError
//	}
//	if price == 0 {
//		return "", zeroPriceError
//	}
//	if amount == 0 {
//		return "", zeroAmountError
//	}
//	if orderType == model.OrderTypeLimit {
//		orderType = "LIMIT_ORDER"
//	}
//	price, decimal := model.FormatPrice(model.Mexc, symbol, orderSide, price)
//	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
//	formattedAmount := model.GetAmountInMarket(model.Mexc, symbol, amount, price)
//	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
//	if util.EndWith(symbol, model.UniStandardTail[model.MarketTypePerp]) {
//		return contractPlaceOrderMex(key, secret, order, orderSide, orderType, symbol, getFloat64OrDefault(priceStr), getFloat64OrDefault(amountStr))
//	} else {
//		return spotPlaceOrderMex(key, secret, order, orderSide, orderType, symbol, getFloat64OrDefault(priceStr), getFloat64OrDefault(amountStr))
//	}
//}
//
//func spotPlaceOrderMex(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) (orderId string, err error) {
//	request := dtos.GenMexcSpotPlaceOrderRequest(orderSide, orderType, symbol, price, amount)
//	requestBody, err := json.Marshal(request)
//	if err != nil {
//		logMsg := fmt.Sprintf(`[spotPlaceOrderMex] Failed to marshall request %+v err %s`, request, err.Error())
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return "", err
//	}
//	respBytes, err := SignedRequestMexc(key, secret, http.MethodPost, spotRestUrl, spotPlaceOrderPath, nil, string(requestBody))
//	if err != nil {
//		return "", err
//	}
//	placeOrderResp := &dtos.MexcSpotPlaceOrderResponse{}
//	err = json.Unmarshal(respBytes, placeOrderResp)
//	if err != nil || placeOrderResp.Code != http.StatusOK {
//		logMsg := fmt.Sprintf(`[spotPlaceOrderMex] Failed to place order, orderSide %s orderType %s symbol %s price %f amount %f requestBody %s resp %+v err %+v statusCode %d`,
//			orderSide, orderType, symbol, price, amount, requestBody, placeOrderResp, err, placeOrderResp.Code)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return "", failedToPlaceOrderError
//	}
//	return placeOrderResp.Data, nil
//}
//
//func contractPlaceOrderMex(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) (orderId string, err error) {
//	request := dtos.GenMexcContractPlaceOrderRequest(orderSide, orderType, symbol, price, amount)
//	requestBody, err := json.Marshal(request)
//	if err != nil {
//		logMsg := fmt.Sprintf(`[contractPlaceOrderMex] Failed to marshall request %+v err %s`, request, err.Error())
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return "", err
//	}
//	respBytes, err := SignedRequestMexc(key, secret, http.MethodPost, contractRestUrl, contractPlaceOrderPath, nil, string(requestBody))
//	if err != nil {
//		logMsg := fmt.Sprintf(`[contractPlaceOrderMex] SignedRequestMexc failed %+v, err %s, resp: %s`, request, err.Error(), string(respBytes))
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return "", err
//	}
//	resp := &dtos.MexcContractPlaceOrderResponse{}
//	err = json.Unmarshal(respBytes, resp)
//	if err != nil || !resp.Success {
//		logMsg := fmt.Sprintf(`[contractPlaceOrderMex] Failed to place order, orderSide %s orderType %s symbol %s price %f amount %f requestBody %s resp %+v err %+v`,
//			orderSide, orderType, symbol, price, amount, requestBody, resp, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return "", failedToPlaceOrderError
//	}
//	return strconv.FormatInt(resp.Data, 10), nil
//}
//
//func getMarketsMexc(key, secret string) (success bool, marketInfos map[string]*model.MarketInfo) {
//	marketInfos = make(map[string]*model.MarketInfo)
//	if err := appendContractMarketMexc(key, secret, marketInfos); err != nil {
//		util.Notice(fmt.Sprintf("appendContractMarketMexc failed: %v", err))
//		return false, marketInfos
//	}
//	return true, marketInfos
//}
//
//func appendContractMarketMexc(key, secret string, marketInfos map[string]*model.MarketInfo) error {
//	util.SocketInfo("[appendContractMarketMexc] Start to get contract market infos")
//	respBytes, err := SignedRequestMexc(key, secret, http.MethodGet, contractRestUrl, contractGetSymbolMarketPath, nil, "")
//	if err != nil {
//		return err
//	}
//	resp := &dtos.MexcContractGetMarketsResp{}
//	err = json.Unmarshal(respBytes, resp)
//	if err != nil {
//		logMsg := fmt.Sprintf(`[appendContractMarketMexc] Failed to get contract market symbols, err %s`, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return err
//	}
//	for _, symbolInfo := range resp.Data {
//		success, marketType, coin := model.GetCoinFromDialect(model.Mexc, symbolInfo.Symbol)
//		if !success {
//			continue
//		}
//		marketInfo := &model.MarketInfo{}
//		// TODO 此处需要确保Mexc的现货和期货的tail不同，否则marketTYpe不可用
//		marketInfo.Name = coin + model.UniStandardTail[marketType]
//		marketInfo.PriceIncrement = symbolInfo.PriceUnit       // 价格的最小步进单位
//		marketInfo.PriceDecimal = symbolInfo.PriceScale        // 价格精度
//		marketInfo.SizeMin = symbolInfo.MinVol                 // 订单张数下限
//		marketInfo.SizeMax = symbolInfo.MaxVol                 // 订单张数上限
//		marketInfo.SizeIncrement = float64(symbolInfo.VolUnit) // 数量的最小步进单位
//		marketInfo.CTCurrency = symbolInfo.BaseCoin
//		marketInfo.CTValue = symbolInfo.ContractSize // 一个合约等于多少个币
//		marketInfos[marketInfo.Name] = marketInfo
//	}
//	return nil
//}
//
///*WsDepthServeMexc 只支持永续合约
//如何维护增量深度信息:
//1. 通过接口 https://contract.mexc.com/api/v1/contract/depth/BTC_USDT获取全量深度信息，保存当前version
//2. 订阅ws深度信息，收到更新后，如果收到的数据version>当前version,同一个价位，后收到的更新覆盖前面的。
//3. 通过接口 https://contract.mexc.com/api/v1/contract/depth_commits/BTC_USDT/1000获取最新1000条深度快照
//4. 将目前缓存的深度信息中同一价格，version<步骤3获取到的快照中的version的数据丢弃
//5. 将深度快照中的内容更新至本地缓存，并从ws接收到的event开始继续更新
//6. 每一个新event的version应该恰好等于上一个event的version+1，否则可能出现了丢包，如出现丢包或者获取到的event的version不连续,请从步骤3重新进行初始化。
//7. 每一个event中的挂单量代表这个价格目前的挂单量绝对值，而不是相对变化。
//8. 如果某个价格对应的挂单量为0，表示该价位的挂单已经撤单或者被吃，应该移除这个价位
//*/
//func WsDepthServeMexc(markets *model.Markets, orderHandler api.OrderHandler, useFullDepthSub bool) (channels []chan struct{}, err error) {
//	symbols := model.GetMarketSymbols(model.Mexc)
//	if !useFullDepthSub {
//		limiter := time.Tick(100 * time.Millisecond)
//		//  初始化contract深度全量信息, 10次每秒
//		for symbol := range symbols {
//			<-limiter
//			_, _, _, dialectSymbol := model.GetFromStandard(model.Mexc, symbol)
//			initMexcContractDepth(markets, dialectSymbol)
//			<-limiter
//			syncMexcContractDepthCommits(markets, dialectSymbol)
//		}
//	}
//	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler api.OrderHandler) {
//		newJson, wsErr := util.NewJSON(event)
//		if wsErr != nil {
//			util.SocketInfo(`MEXC fail to unmarshal json ` + err.Error())
//			return
//		}
//		channel, _ := newJson.Get("channel").String()
//		ts, _ := newJson.Get("ts").Int64()
//		if ts != 0 { // contract类型的推送10档全量和增量的msg结构完全一样，所以只能解析之后通过bids/asks数量判断
//			if channel == "push.depth" {
//				resp := &dtos.MexcContractDepthWsResp{}
//				err := json.Unmarshal(event, resp)
//				if err != nil {
//					logMsg := fmt.Sprintf(`[contractDepthFullWsHandlerMexc] Failed to unmarshal MEXC contract full depth push message %s err: %s`, string(event), err.Error())
//					util.Notice(logMsg)
//					fmt.Println(logMsg)
//					return
//				}
//
//				if len(resp.Data.Asks) > 1 { // 深度档位大于1表示是按档位全量订阅的推送
//					contractDepthFullWsHandlerMexc(markets, resp, orderHandler)
//				} else { // 否则认为是增量推送
//					contractDepthIncreWsHandlerMexc(markets, resp, orderHandler)
//				}
//			}
//		}
//	}
//	channels = make([]chan struct{}, 0)
//	if !useFullDepthSub {
//		// 订阅contract深度增量
//		//channels = append(channels, initChannel(mexcContractWSUrl, mexcContractDepthIncreSubType, wsHandler, orderHandler)...)
//		contractIncreSubs := api.GetWSSubscribes(model.Mexc, mexcContractDepthIncSubType)
//		contractIncreChans, err := api.WebSocketClient(model.Mexc, mexcContractWSUrl, contractIncreSubs,
//			subscribeHandlerMexc, wsHandler, orderHandler, wsStepMexc)
//		if err != nil {
//			util.SocketInfo(`fail to create MEXC contract increment depth conn %s`, err.Error())
//		}
//		channels = append(channels, contractIncreChans...)
//	} else {
//		// 订阅contract 10档深度全量
//		//channels = append(channels, initChannel(mexcContractWSUrl, mexcContractDepthFullSubType, wsHandler, orderHandler)...)
//		contractFullSubs := api.GetWSSubscribes(model.Mexc, mexcContractDepthFullSubType)
//		contractFullChans, err := api.WebSocketClient(model.Mexc, mexcContractWSUrl, contractFullSubs,
//			subscribeHandlerMexc, wsHandler, orderHandler, wsStepMexc)
//		if err != nil {
//			util.SocketInfo(`fail to create MEXC contract full depth conn %s`, err.Error())
//		}
//		channels = append(channels, contractFullChans...)
//	}
//
//	// 订阅contract Ticker
//	//channels = append(channels, initChannel(mexcContractWSUrl, mexcContractTickerSubType, wsHandler, orderHandler)...)
//
//	//go maintainChannelMexc(useFullDepthSub)
//	return channels, err
//}
//
//// region contract深度相关
//
//// contract增量深度订阅handler
//func contractDepthIncreWsHandlerMexc(markets *model.Markets, resp *dtos.MexcContractDepthWsResp, orderHandler api.OrderHandler) {
//	if resp == nil {
//		return
//	}
//	if !resp.IsValidChannel() {
//		msg := fmt.Sprintf("InvalidChannel in %+v", resp)
//		util.Notice(msg)
//		fmt.Println(msg)
//		return
//	}
//	_, marketType, coin := model.GetCoinFromDialect(model.Mexc, resp.Symbol)
//	symbol := coin + model.UniStandardTail[marketType]
//	lastTickId, _ := lastTickIdMexc.Load(symbol)
//	if lastTickId != nil && (lastTickId == 0 || lastTickId == (resp.Data.Version-1)) {
//		lastTickIdMexc.Store(symbol, resp.Data.Version)
//	} else {
//		msg := fmt.Sprintf("Version did not increment by 1, lastTickId=%d, resp: %+v", lastTickId, resp)
//		util.Notice(msg)
//		fmt.Println(msg)
//		syncMexcContractDepthCommits(markets, symbol)
//	}
//	ts := resp.Ts
//	now := int(time.Now().UnixNano() / int64(time.Millisecond))
//	if ts == 0 {
//		ts = now
//	}
//	var asks []model.Tick
//	var bids []model.Tick
//	for _, ask := range resp.Data.Asks {
//		tick := getTick(ask)
//		if tick != nil {
//			asks = append(asks, *tick)
//		}
//	}
//	for _, bid := range resp.Data.Bids {
//		tick := getTick(bid)
//		if tick != nil {
//			bids = append(bids, *tick)
//		}
//	}
//	bidAsk := &model.BidAsk{
//		Ts:         ts,
//		TsReceived: now,
//		UpdateId:   resp.Data.Version,
//		Bids:       bids,
//		Asks:       asks,
//	}
//	setMexcAskBid(markets, symbol, bidAsk)
//}
//
//// contract全量深度订阅handler
//func contractDepthFullWsHandlerMexc(markets *model.Markets, resp *dtos.MexcContractDepthWsResp, orderHandler api.OrderHandler) {
//	if resp == nil {
//		return
//	}
//	if !resp.IsValidChannel() {
//		msg := fmt.Sprintf("InvalidChannel in %+v", resp)
//		util.Notice(msg)
//		fmt.Println(msg)
//		return
//	}
//	ts := resp.Ts
//	now := int(time.Now().UnixNano() / int64(time.Millisecond))
//	if ts == 0 {
//		ts = now
//	}
//	var asks []model.Tick
//	var bids []model.Tick
//	for _, ask := range resp.Data.Asks {
//		tick := getTick(ask)
//		if tick != nil {
//			asks = append(asks, *tick)
//		}
//	}
//	for _, bid := range resp.Data.Bids {
//		tick := getTick(bid)
//		if tick != nil {
//			bids = append(bids, *tick)
//		}
//	}
//	bidAsk := &model.BidAsk{
//		Ts:         ts,
//		TsReceived: now,
//		UpdateId:   resp.Data.Version,
//		Bids:       bids,
//		Asks:       asks,
//	}
//	_, marketType, coin := model.GetCoinFromDialect(model.Mexc, resp.Symbol)
//	symbol := coin + model.UniStandardTail[marketType]
//	haveOld, old := markets.GetBidAsk(symbol, model.Mexc)
//	if haveOld && old.UpdateId > bidAsk.UpdateId {
//		msg := fmt.Sprintf("[contractDepthFullWsHandlerMexc] Version too low, skip this bidAsk, cachedBidAsk: %v, newBidAsk: %v", old, bidAsk)
//		util.Notice(msg)
//		fmt.Println(msg)
//		return
//	}
//	// 直接覆盖
//	if markets.SetBidAsk(symbol, model.Mexc, bidAsk) {
//		for function, handler := range model.GetFunctions(model.Mexc, symbol) {
//			if handler != nil {
//				setting := model.GetSetting(function, model.Mexc, symbol)
//				if setting != nil {
//					go handler(setting, bidAsk)
//				}
//			}
//		}
//	}
//}
//
//func contractTickerWsHandlerMexc(markets *model.Markets, msg []byte, orderHandler api.OrderHandler) {
//	resp := &dtos.MexcContractTickerResp{}
//	err := json.Unmarshal(msg, resp)
//	if err != nil {
//		logMsg := fmt.Sprintf(`Failed to unmarshal MEXC contract ticker push message %s err: %s`, string(msg), err.Error())
//		util.Notice(logMsg)
//		fmt.Println(logMsg)
//		return
//	}
//	if !resp.IsValidChannel() {
//		return
//	}
//	ts := resp.Data.Timestamp
//	now := int(time.Now().UnixNano() / int64(time.Millisecond))
//	if ts == 0 {
//		ts = now
//	}
//	// Todo 没有version/UpdateId
//	bidAsk := model.BidAsk{Ts: ts, TsReceived: now,
//		Bids: []model.Tick{{Price: resp.Data.MaxBidPrice, Amount: resp.Data.Bid1}},
//		Asks: []model.Tick{{Price: resp.Data.MinAskPrice, Amount: resp.Data.Ask1}}}
//	success, marketType, coin := model.GetCoinFromDialect(model.Mexc, resp.Symbol)
//	if success {
//		setMexcAskBid(markets, coin+model.UniStandardTail[marketType], &bidAsk)
//	}
//	logMsg := fmt.Sprintf("mexc contract ticker sub result %+v \n", resp)
//	util.SocketInfo(logMsg)
//}
//
//func initMexcContractDepth(markets *model.Markets, symbol string) {
//	if markets == nil || symbol == "" {
//		return
//	}
//	// region 第1步 通过接口 https://contract.mexc.com/api/v1/contract/depth/BTC_USDT获取全量深度信息，保存当前version
//	depthResp, err := mexcGetContractSymbolDepth(symbol)
//	if err != nil || depthResp == nil {
//		return
//	}
//	ts := depthResp.Data.Timestamp
//	now := int(time.Now().UnixNano() / int64(time.Millisecond))
//	if ts == 0 {
//		ts = now
//	}
//	var asks []model.Tick
//	var bids []model.Tick
//	for _, ask := range depthResp.Data.Asks {
//		tick := getTick(ask)
//		if tick != nil {
//			asks = append(asks, *tick)
//		}
//	}
//	for _, bid := range depthResp.Data.Bids {
//		tick := getTick(bid)
//		if tick != nil {
//			bids = append(bids, *tick)
//		}
//	}
//	bidAsk := &model.BidAsk{
//		Ts:         ts,
//		TsReceived: now,
//		UpdateId:   depthResp.Data.Version,
//		Bids:       bids,
//		Asks:       asks,
//	}
//	setMexcAskBid(markets, symbol, bidAsk)
//	// endregion
//}
//
//func syncMexcContractDepthCommits(markets *model.Markets, symbol string) {
//	// 通过接口 https://contract.mexc.com/api/v1/contract/depth_commits/BTC_USDT/1000获取最新1000条深度快照
//	resp, err := mexcGetContractSymbolDepthCommits(symbol)
//	if err != nil || resp == nil {
//		return
//	}
//	now := int(time.Now().UnixNano() / int64(time.Millisecond))
//	ts := now
//	for _, data := range resp.Data {
//		var asks []model.Tick
//		var bids []model.Tick
//		for _, ask := range data.Asks {
//			tick := getTick(ask)
//			if tick != nil {
//				asks = append(asks, *tick)
//			}
//		}
//		for _, bid := range data.Bids {
//			tick := getTick(bid)
//			if tick != nil {
//				bids = append(bids, *tick)
//			}
//		}
//		bidAsk := &model.BidAsk{
//			Ts:         ts,
//			TsReceived: now,
//			UpdateId:   data.Version,
//			Bids:       bids,
//			Asks:       asks,
//		}
//		setMexcAskBid(markets, symbol, bidAsk)
//	}
//}
//
//func getTick(tickInfo []float64) *model.Tick {
//	if len(tickInfo) != 3 { // 备注: [411.8, 10, 1] 411.8为价格，10为此价格的合约张数, 1为订单数量
//		return nil
//	}
//	if tickInfo[0] <= 0 {
//		return nil
//	}
//	//if tickInfo[2] != 1 {
//	//	msg := fmt.Sprintf("contract count is not 1, tickInfo[2] is not handled, %v", tickInfo)
//	//	util.Notice(msg)
//	//	fmt.Println(msg)
//	//}
//	// TODO: 验证这个数量 是不是 合约张数x订单数量
//	return &model.Tick{Price: tickInfo[0], Amount: tickInfo[1] * tickInfo[2]}
//}
//
//func mergeTicks(oldTicks []model.Tick, incrementalTicks []model.Tick, ascending bool) []model.Tick {
//	// TODO: Do not recreate this map every time.
//	if oldTicks == nil || len(oldTicks) == 0 {
//		return incrementalTicks
//	}
//	if incrementalTicks == nil || len(incrementalTicks) == 0 {
//		return oldTicks
//	}
//	m := make(map[float64]model.Tick)
//	for _, tick := range oldTicks {
//		m[tick.Price] = tick
//	}
//	for _, tick := range incrementalTicks {
//		if tick.Amount == 0 {
//			delete(m, tick.Price)
//		} else {
//			m[tick.Price] = tick
//		}
//	}
//	keys := make([]float64, 0, len(m))
//	for k := range m {
//		keys = append(keys, k)
//	}
//	if ascending {
//		sort.Float64s(keys)
//	} else {
//		sort.Sort(sort.Reverse(sort.Float64Slice(keys)))
//	}
//	newTicks := make([]model.Tick, 0, len(m))
//	for _, key := range keys {
//		newTicks = append(newTicks, m[key])
//	}
//	return newTicks
//}
//
//func setMexcAskBid(markets *model.Markets, symbol string, bidAsk *model.BidAsk) {
//	if markets == nil || symbol == "" || bidAsk == nil {
//		return
//	}
//	haveOld, old := markets.GetBidAsk(symbol, model.Mexc)
//	// For contract the version might be always 0, in this case it will be lower version
//	// than version returned by full depth.
//	if haveOld && old.UpdateId > bidAsk.UpdateId && bidAsk.UpdateId != 0 {
//		msg := fmt.Sprintf("[setMexcAskBid] Version too low, skip this bidAsk, cachedBidAsk: %v, newBidAsk: %v", old, bidAsk)
//		util.Notice(msg)
//		fmt.Println(msg)
//		return
//	}
//	if old != nil {
//		bidAsk.Bids = mergeTicks(old.Bids, bidAsk.Bids, false)
//		bidAsk.Asks = mergeTicks(old.Asks, bidAsk.Asks, true)
//	}
//	if markets.SetBidAsk(symbol, model.Mexc, bidAsk) {
//		for function, handler := range model.GetFunctions(model.Mexc, symbol) {
//			if handler != nil {
//				setting := model.GetSetting(function, model.Mexc, symbol)
//				if setting != nil {
//					go handler(setting, bidAsk)
//				}
//			}
//		}
//	}
//}
//
//func mexcGetContractSymbolDepth(symbol string) (*dtos.MexcContractDepthHttpResp, error) {
//	path := fmt.Sprintf(contractGetSymbolDepthPathFmt, symbol)
//	respBytes, err := publicRequestMexc(http.MethodGet, contractRestUrl, path, nil, "")
//	if err != nil {
//		logMsg := fmt.Sprintf(`[mexcGetContractSymbolDepth] Failed to get depth info for symbol %s err %+v`, symbol, err)
//		util.Notice(logMsg)
//		fmt.Println(logMsg)
//		return nil, err
//	}
//	resp := &dtos.MexcContractDepthHttpResp{}
//	err = json.Unmarshal(respBytes, resp)
//	if err != nil || !resp.Success {
//		logMsg := fmt.Sprintf(`[mexcGetContractSymbolDepth] Failed to get depth info %s for symbol %s success %t err %+v`, string(respBytes), symbol, resp.Success, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return nil, err
//	}
//	return resp, nil
//}
//
//func mexcGetContractSymbolDepthCommits(symbol string) (*dtos.MexcContractDepthCommitsResp, error) {
//	path := fmt.Sprintf(contractGetSymbolDepthCommitsPathFmt, symbol, 10)
//	respBytes, err := publicRequestMexc(http.MethodGet, contractRestUrl, path, nil, "")
//	if err != nil {
//		logMsg := fmt.Sprintf(`[mexcGetContractSymbolDepthCommits] Failed to get depth info for symbol %s err %+v`, symbol, err)
//		util.Notice(logMsg)
//		fmt.Println(logMsg)
//		return nil, err
//	}
//	resp := &dtos.MexcContractDepthCommitsResp{}
//	err = json.Unmarshal(respBytes, resp)
//	if err != nil || !resp.Success {
//		logMsg := fmt.Sprintf(`[mexcGetContractSymbolDepthCommits] Failed to get depth info %s for symbol %s success %t err %+v`, string(respBytes), symbol, resp.Success, err)
//		fmt.Println(logMsg)
//		util.Notice(logMsg)
//		return nil, err
//	}
//	return resp, nil
//}
//
//// endregion
//var subscribeHandlerMexc = func(connection *websocket.Conn, subscribes []interface{}) error {
//	var err error
//	for _, subscribe := range subscribes {
//		subMsg := fmt.Sprintf(`%s`, subscribe)
//		if err = api.SendToConnection(model.Mexc, connection, []byte(subMsg)); err != nil {
//			util.SocketInfo(" MEXC can not subscribe %s %s", subscribe, err.Error())
//		}
//		time.Sleep(time.Millisecond * 300) // Todo - 这里的限速怎么设定
//	}
//	return err
//}
//
//func getFloat64OrDefault(val string) float64 {
//	ret, err := strconv.ParseFloat(val, 64)
//	if err != nil {
//		ret = 0
//	}
//	return ret
//}
//
//func GetWSSubscribeMexc(symbol string, subType string) string {
//	switch subType {
//	case mexcContractDepthIncSubType:
//		return fmt.Sprintf(`{
//				"method":"sub.depth",
//				"param":{
//					"symbol":"%s",
//					"compress":true
//				}
//			}`, symbol)
//	case mexcContractDepthFullSubType:
//		return fmt.Sprintf(`{
//				"method":"sub.depth.full",
//				"param":{
//					"symbol":"%s",
//					"limit":10
//				}
//			}`, symbol)
//	case mexcContractTickerSubType:
//		return fmt.Sprintf(`{
//				"method":"sub.ticker",
//				"param":{
//					"symbol":"%s"
//				}
//			}`, symbol)
//	}
//	return ""
//}

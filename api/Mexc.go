package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"hello/api/dtos"
	"hello/model"
	"hello/util"
)

const (
	spotRestUrl                  = "www.mexc.com"
	contractRestUrl              = "contract.mexc.com"
	mexcContractWSUrl            = "wss://contract.mexc.com/ws"
	mexcContractDepthIncSubType  = "mexcContractDepthIncreSubType" // Contract深度增量订阅
	mexcContractDepthFullSubType = "mexcContractDepthFullSubType"  // Contract深度全量订阅（按档位）
	mexcContractTickerSubType    = "mexcContractTickerSubType"
	wsStepMexc                   = 20
)

// spot rest api path
const (
	spotPlaceOrderPath               = "/open/api/v2/order/place"            // 下单
	spotCancelOrdersBySymbolPath     = "/open/api/v2/order/cancel_by_symbol" // 按交易对撤销订单
	spotQueryOrderByIdPath           = "/open/api/v2/order/query"            // 查询订单
	spot_get_all_market_symbols_path = "/open/api/v2/market/symbols"         // 所有交易对信息
)

// contract rest api path
const (
	contractPlaceOrderPath           = "/api/v1/private/order/submit"     // 下单
	contractCancelOrdersBySymbolPath = "/api/v1/private/order/cancel_all" // 撤销某个合约下的所有未完成订单
	contractQueryOrderByIdPath       = "api/v1/private/order/get"         // 根据订单号查询订单
)

var (
	emptySymbolError        = errors.New("symbol is empty")
	zeroPriceError          = errors.New("price is 0")
	zeroAmountError         = errors.New("amount is 0")
	failedToPlaceOrderError = errors.New("failed to place order")
	channelMaintainingMexc  = false
	mexcSymbolConnection    sync.Map
)

func SignedRequestMexc(key, secret, method, restUrl, path string, paramsInQuery map[string]interface{}, body string) ([]byte, error) {
	var parameters string
	if len(paramsInQuery) > 0 {
		param := &url.Values{}
		for k, v := range paramsInQuery {
			param.Set(k, v.(string))
		}
		parameters = param.Encode()
	}
	reqTime := strconv.FormatInt(util.GetNow().UnixNano(), 10)[0:13]
	var sign string
	if method == http.MethodPost {
		sign = genSign(key, secret, reqTime, body)
	} else {
		sign = genSign(key, secret, reqTime, parameters)
	}
	headers := map[string]string{
		"Request-Time": reqTime,
		"ApiKey":       key,
		"signature":    sign,
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
	// util.ComposeParams(paramsInQuery)
	requestUrl := fmt.Sprintf(`https://%s%s`, restUrl, path)
	if len(parameters) > 0 {
		requestUrl = requestUrl + "?" + parameters
	}
	responseBody, err := util.HttpRequest(method, requestUrl, body, headers, 60)
	logMsg := fmt.Sprintf(`Mexc key %s, request %s, headers %v, parameters %s, body %s, return %s err %+v`,
		key, requestUrl, headers, parameters, body, string(responseBody), err)
	util.SocketInfo(logMsg)
	return responseBody, err
}

func genSign(key, secret, reqTime, parameters string) string {
	toBeSign := key + reqTime + parameters
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(toBeSign))
	return hex.EncodeToString(hash.Sum(nil))
}

func publicRequestMexc(method, restUrl, path string, paramsInQuery map[string]interface{}, body string) ([]byte, error) {
	var parameters string
	if len(paramsInQuery) > 0 {
		param := &url.Values{}
		for k, v := range paramsInQuery {
			param.Set(k, v.(string))
		}
		parameters = param.Encode()
	}
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	// util.ComposeParams(paramsInQuery)
	requestUrl := fmt.Sprintf(`https://%s%s`, restUrl, path)
	if len(parameters) > 0 {
		requestUrl = requestUrl + "?" + parameters
	}
	responseBody, err := util.HttpRequest(method, requestUrl, body, headers, 60)
	logMsg := fmt.Sprintf(`Mexc request %s headers %v parameters %s return %s err %+v`, requestUrl, headers, parameters, string(responseBody), err)
	util.SocketInfo(logMsg)
	return responseBody, err
}

// region cancel orders
func cancelOrdersMexc(key string, secret string, symbol string) (result bool) {
	if util.EndWith(symbol, model.UniStandardTail[model.MarketTypePerp]) {
		return contractCancelOrdersMexc(key, secret, symbol)
	} else {
		return spotCancelOrdersMexc(key, secret, symbol)
	}
}

func spotCancelOrdersMexc(key string, secret string, symbol string) bool {
	paramsInQuery := map[string]interface{}{"symbol": symbol}
	responseBytes, err := SignedRequestMexc(key, secret, http.MethodDelete, spotRestUrl, spotCancelOrdersBySymbolPath, paramsInQuery, "")
	if err != nil {
		logMsg := fmt.Sprintf(`[spotCancelOrdersMexc] Failed to cancel orders by symbol %s err %+v`, symbol, err)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return false
	}
	response := &dtos.MexcSpotCancelOrderBySymbolResp{}
	err = json.Unmarshal(responseBytes, response)
	if err != nil || response.Code != http.StatusOK {
		logMsg := fmt.Sprintf(`[spotCancelOrdersMexc] Failed to cancel orders by symbol %s statusCode %d err %+v`, symbol, response.Code, err)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return false
	}
	failedOrderIDs := response.GetFailedOrderIDsMexc()
	if len(failedOrderIDs) > 0 {
		logMsg := fmt.Sprintf(`[spotCancelOrdersMexc] Failed to cancel orders %+v by symbol %s`, failedOrderIDs, symbol)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return false
	}

	return true
}

func contractCancelOrdersMexc(key string, secret string, symbol string) bool {
	body := fmt.Sprintf(`{"symbol":"%s"}`, symbol)
	responseBytes, err := SignedRequestMexc(key, secret, http.MethodPost, contractRestUrl, contractCancelOrdersBySymbolPath, nil, body)
	if err != nil {
		logMsg := fmt.Sprintf(`[contractCancelOrdersMexc] Failed to cancel orders by symbol %s err %+v`, symbol, err)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return false
	}
	response := &dtos.MexcContractCancelOrderBySymbolResp{}
	err = json.Unmarshal(responseBytes, response)
	if err != nil || !response.Success {
		logMsg := fmt.Sprintf(`[contractCancelOrdersMexc] Failed to cancel orders by symbol %s response %v err %+v`, symbol, response, err)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return false
	}
	return true
}

// endregion
func queryOrderMexc(key, secret string, order *model.Order) {
	if order.Market != model.Mexc || order.OrderId == "" {
		return
	}
	if util.EndWith(order.Symbol, model.MarketTypePerp) {
		contractQueryOrderMexc(key, secret, order)
	} else {
		spotQueryOrderMexc(key, secret, order)
	}
}

func spotQueryOrderMexc(key, secret string, order *model.Order) {
	if order.OrderId == "" {
		return
	}
	paramsInQuery := map[string]interface{}{"order_ids": order.OrderId}
	respBytes, err := SignedRequestMexc(key, secret, http.MethodGet, spotRestUrl, spotQueryOrderByIdPath, paramsInQuery, "")
	if err != nil {
		logMsg := fmt.Sprintf(`[spotQueryOrderMexc] Failed to query orders by order_id %s err %+v`, order.OrderId, err)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return
	}
	resp := &dtos.MexcSpotQueryOrderResp{}
	err = json.Unmarshal(respBytes, resp)
	if err != nil || resp.Code != http.StatusOK {
		logMsg := fmt.Sprintf(`[spotQueryOrderMexc] Failed to query orders by order_id %s statusCode %d err %+v`, order.OrderId, resp.Code, err)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return
	}
	if len(resp.Data) == 1 {
		success, marketType, coin := model.GetCoinFromDialect(model.Mexc, resp.Data[0].Symbol)
		if success { // TODO 需要确保Mexc永续和现货tail不同，否则marketType不可用
			order.Symbol = coin + model.UniStandardTail[marketType]
		}
		order.Price = getFloat64OrDefault(resp.Data[0].Price)             // 挂单价格
		order.DealPrice = getFloat64OrDefault(resp.Data[0].DealAmount)    // 成交金额
		order.DealAmount = getFloat64OrDefault(resp.Data[0].DealQuantity) // 成交数量
		order.CreatedAt = time.Unix(resp.Data[0].CreateTime, 0)
		order.Status = resp.Data[0].State
		order.OrderType = resp.Data[0].Type
		return
	}
	logMsg := fmt.Sprintf(`[spotQueryOrderMexc] Get %d orders by order_id %s`, len(resp.Data), order.OrderId)
	util.SocketInfo(logMsg)
}

func contractQueryOrderMexc(key, secret string, order *model.Order) {
	if order.OrderId == "" {
		return
	}
	paramsInQuery := map[string]interface{}{"order_id": order.OrderId}
	respBytes, err := SignedRequestMexc(key, secret, http.MethodGet, contractRestUrl, contractQueryOrderByIdPath, paramsInQuery, "")
	if err != nil {
		logMsg := fmt.Sprintf(`[contractQueryOrderMexc] Failed to query orders by order_id %s err %+v`, order.OrderId, err)
		util.Notice(logMsg)
		fmt.Println(logMsg)
		return
	}
	resp := &dtos.MexcContractQueryOrderResp{}
	err = json.Unmarshal(respBytes, resp)
	if err != nil || !resp.Success {
		logMsg := fmt.Sprintf(`[contractQueryOrderMexc] Failed to query orders by order_id %s success %t statusCode %d err %+v`, order.OrderId, resp.Success, resp.Code, err)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return
	}
	success, marketType, coin := model.GetCoinFromDialect(model.Mexc, resp.Data.Symbol)
	if success { // TODO 需要确保Mexc永续和现货tail不同，否则marketType不可用
		order.Symbol = coin + model.UniStandardTail[marketType]
	}
	order.Price = resp.Data.Price            // 挂单价格
	order.DealPrice = resp.Data.DealAvgPrice // 成交金额
	order.DealAmount = resp.Data.DealVol     // 成交数量
	order.CreatedAt = time.Unix(resp.Data.CreateTime, 0)
	order.Status = strconv.Itoa(int(resp.Data.State))
	order.OrderType = strconv.Itoa(int(resp.Data.OrderType))
}

func getPositionsMexc(key string, secret string) (success bool, positions []*model.Position, posBalance float64) {
	return
}

// region place order
func placeOrderMexc(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) (orderId string, err error) {
	if symbol == "" {
		return "", emptySymbolError
	}
	if price == 0 {
		return "", zeroPriceError
	}
	if amount == 0 {
		return "", zeroAmountError
	}
	if orderType == model.OrderTypeLimit {
		orderType = "LIMIT_ORDER"
	}
	price, decimal := model.FormatPrice(model.Mexc, symbol, orderSide, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	formattedAmount := model.GetAmountInMarket(model.Mexc, symbol, amount, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	if util.EndWith(symbol, model.UniStandardTail[model.MarketTypePerp]) {
		return contractPlaceOrderMex(key, secret, order, orderSide, orderType, symbol, getFloat64OrDefault(priceStr), getFloat64OrDefault(amountStr))
	} else {
		return spotPlaceOrderMex(key, secret, order, orderSide, orderType, symbol, getFloat64OrDefault(priceStr), getFloat64OrDefault(amountStr))
	}
}

func spotPlaceOrderMex(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) (orderId string, err error) {
	request := dtos.GenMexcSpotPlaceOrderRequest(orderSide, orderType, symbol, price, amount)
	requestBody, err := json.Marshal(request)
	if err != nil {
		logMsg := fmt.Sprintf(`[spotPlaceOrderMex] Failed to marshall request %+v err %s`, request, err.Error())
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return "", err
	}
	respBytes, err := SignedRequestMexc(key, secret, http.MethodPost, spotRestUrl, spotPlaceOrderPath, nil, string(requestBody))
	if err != nil {
		return "", err
	}
	placeOrderResp := &dtos.MexcSpotPlaceOrderResponse{}
	err = json.Unmarshal(respBytes, placeOrderResp)
	if err != nil || placeOrderResp.Code != http.StatusOK {
		logMsg := fmt.Sprintf(`[spotPlaceOrderMex] Failed to place order, orderSide %s orderType %s symbol %s price %f amount %f requestBody %s resp %+v err %+v statusCode %d`,
			orderSide, orderType, symbol, price, amount, requestBody, placeOrderResp, err, placeOrderResp.Code)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return "", failedToPlaceOrderError
	}
	return placeOrderResp.Data, nil
}

func contractPlaceOrderMex(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) (orderId string, err error) {
	request := dtos.GenMexcContractPlaceOrderRequest(orderSide, orderType, symbol, price, amount)
	requestBody, err := json.Marshal(request)
	if err != nil {
		logMsg := fmt.Sprintf(`[contractPlaceOrderMex] Failed to marshall request %+v err %s`, request, err.Error())
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return "", err
	}
	respBytes, err := SignedRequestMexc(key, secret, http.MethodPost, contractRestUrl, contractPlaceOrderPath, nil, string(requestBody))
	if err != nil {
		logMsg := fmt.Sprintf(`[contractPlaceOrderMex] SignedRequestMexc failed %+v, err %s, resp: %s`, request, err.Error(), string(respBytes))
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return "", err
	}
	resp := &dtos.MexcContractPlaceOrderResponse{}
	err = json.Unmarshal(respBytes, resp)
	if err != nil || !resp.Success {
		logMsg := fmt.Sprintf(`[contractPlaceOrderMex] Failed to place order, orderSide %s orderType %s symbol %s price %f amount %f requestBody %s resp %+v err %+v`,
			orderSide, orderType, symbol, price, amount, requestBody, resp, err)
		fmt.Println(logMsg)
		util.Notice(logMsg)
		return "", failedToPlaceOrderError
	}
	return strconv.FormatInt(resp.Data, 10), nil
}

func getMarketsMexc(key, secret string) (success bool, marketInfos map[string]*model.MarketInfo) {
	respBytes, err := SignedRequestMexc(key, secret, http.MethodGet, contractRestUrl, `/api/v1/contract/detail`, nil, "")
	if err != nil {
		return false, nil
	}
	marketInfos = make(map[string]*model.MarketInfo)
	resp := &dtos.MexcContractGetMarketsResp{}
	err = json.Unmarshal(respBytes, resp)
	if err != nil {
		util.Notice(fmt.Sprintf(`[appendContractMarketMexc] Failed to get contract market symbols, err %s`, err))
		return false, nil
	}
	i := 0
	for _, symbolInfo := range resp.Data {
		ok, marketType, coin := model.GetCoinFromDialect(model.Mexc, symbolInfo.Symbol)
		if !ok {
			continue
		}
		i++
		symbol := coin + model.UniStandardTail[marketType]
		marketInfos[symbol] = &model.MarketInfo{Market: model.Mexc, Name: symbol, CTCurrency: symbolInfo.BaseCoin,
			SizeIncrement:  symbolInfo.ContractSize,
			PriceIncrement: symbolInfo.PriceUnit,
			CTValue:        symbolInfo.ContractSize,
			PriceDecimal:   symbolInfo.PriceScale,
			SizeMax:        symbolInfo.MaxVol * symbolInfo.ContractSize,
			SizeMin:        symbolInfo.MinVol * symbolInfo.ContractSize,
		}
		fmt.Println(fmt.Sprintf(`%d[appendContractMarketMexc] market %s, symbol %s, ctCurrency %s, sizeIncrement %f, priceIncrement %f, ctValue %f, priceDecimal %d, sizeMax %f, sizeMin %f`,
			i, marketInfos[symbol].Market, marketInfos[symbol].Name, marketInfos[symbol].CTCurrency, marketInfos[symbol].SizeIncrement,
			marketInfos[symbol].PriceIncrement, marketInfos[symbol].CTValue, marketInfos[symbol].PriceDecimal,
			marketInfos[symbol].SizeMax, marketInfos[symbol].SizeMin))
	}
	return true, marketInfos
}

func appendContractMarketMexc(key, secret string, marketInfos map[string]*model.MarketInfo) error {
	util.SocketInfo("[appendContractMarketMexc] Start to get contract market infos")

	return nil
}

func maintainChannelMexc(subscribes []interface{}) {
	if !channelMaintainingMexc {
		channelMaintainingMexc = true
		go func() {
			for true {
				time.Sleep(time.Second * 10)
				if err := SendToAllConnections(model.Mexc, []byte(`{"method": "ping"}`)); err != nil {
					util.SocketInfo("mexc channel ping error " + err.Error())
				}
			}
		}()
		for true {
			time.Sleep(time.Minute * 3)
			needReset := false
			for _, subscribe := range subscribes {
				subJson, parseErr := util.NewJSON([]byte(subscribe.(string)))
				if subJson == nil || parseErr != nil {
					continue
				}
				dialectSymbol := subJson.GetPath(`param`, `symbol`).MustString()
				ok, marketType, coin := model.GetCoinFromDialect(model.Mexc, dialectSymbol)
				conn, connGet := mexcSymbolConnection.Load(subscribe)
				if !ok || !connGet {
					continue
				}
				symbol := coin + model.UniStandardTail[marketType]
				_, bidAsk := model.AppMarkets.GetBidAsk(symbol, model.Mexc)
				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 180000 {
					needReset = true
					break
				} else if bidAsk != nil && time.Now().UnixMilli()-int64(bidAsk.Ts) > 120000 {
					if err := SendToConnection(model.Mexc, conn.(*websocket.Conn), []byte(subscribe.(string))); err != nil {
						util.SocketInfo(" mexc can not subscribe %s %s", subscribe, err.Error())
					}
				}
				time.Sleep(time.Millisecond * 100)
			}
			if !needReset {
				util.Notice(`no need reset %s`, model.Mexc)
			} else {
				setRequireReset(model.Mexc)
			}
		}
	}
}

func WsDepthServeMexc(markets *model.Markets, orderHandler OrderHandler, useFullDepthSub bool) (channels []chan struct{}, err error) {
	symbols := model.GetMarketSymbols(model.Mexc)
	if !useFullDepthSub {
		limiter := time.Tick(time.Millisecond * 100)
		for symbol := range symbols {
			<-limiter
			initMexcContractDepth(markets, symbol)
		}
	}
	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		newJson, wsErr := util.NewJSON(event)
		if wsErr != nil {
			util.SocketInfo(`MEXC fail to unmarshal json ` + err.Error())
			return
		}
		channel, _ := newJson.Get("channel").String()
		ts, _ := newJson.Get("ts").Int64()
		if ts != 0 && channel == `push.depth.full` {
			resp := &dtos.MexcContractDepthWsResp{}
			if json.Unmarshal(event, resp) == nil {
				if resp == nil {
					util.Notice(fmt.Sprintf("InvalidChannel in %+v", resp))
					return
				}
				_, marketType, coin := model.GetCoinFromDialect(model.Mexc, resp.Symbol)
				symbol := coin + model.UniStandardTail[marketType]
				bidAsk := parseTicksMexc(symbol, resp.Ts, resp.Data.Version, resp.Data.Bids, resp.Data.Asks)
				fmt.Println(fmt.Sprintf(`%f %f ~ %f %f`, bidAsk.Bids[0].Price, bidAsk.Bids[0].Amount, bidAsk.Asks[0].Price, bidAsk.Asks[0].Amount))
				if markets.SetBidAsk(symbol, model.Mexc, bidAsk) {
					for function, handler := range model.GetFunctions(model.Mexc, symbol) {
						setting := model.GetSetting(function, model.Mexc, symbol)
						if handler != nil && setting != nil {
							go handler(setting, bidAsk)
						}
					}
				}
			}
		}
	}
	if !useFullDepthSub { // 订阅contract深度增量
		return WebSocketClient(model.Mexc, mexcContractWSUrl,
			GetWSSubscribes(model.Mexc, mexcContractDepthIncSubType), subscribeHandlerMexc, wsHandler, orderHandler, wsStepMexc)
	} else { // 订阅contract 5档深度全量
		return WebSocketClient(model.Mexc, mexcContractWSUrl, GetWSSubscribes(model.Mexc, mexcContractDepthFullSubType),
			subscribeHandlerMexc, wsHandler, orderHandler, wsStepMexc)
	}
}

func parseTicksMexc(symbol string, ts int, version int64, bidArray, asksArray [][]float64) *model.BidAsk {
	var bids, asks []model.Tick
	for _, tick := range asksArray {
		if tick == nil || len(tick) != 3 {
			continue
		}
		ok, amount := model.ParseRealAmount(model.Mexc, symbol, tick[1])
		if ok {
			asks = append(asks, model.Tick{Side: model.OrderSideSell, Market: model.Mexc, Symbol: symbol, Price: tick[0], Amount: amount})
		}
	}
	for _, tick := range bidArray {
		if tick == nil || len(tick) != 3 {
			continue
		}
		ok, amount := model.ParseRealAmount(model.Mexc, symbol, tick[1])
		if ok {
			bids = append(bids, model.Tick{Side: model.OrderSideBuy, Market: model.Mexc, Symbol: symbol, Price: tick[0], Amount: amount})
		}
	}
	return &model.BidAsk{Ts: ts, TsReceived: int(time.Now().UnixMilli()), UpdateId: version, Bids: bids, Asks: asks}
}

func initMexcContractDepth(markets *model.Markets, symbol string) {
	_, _, _, dialectSymbol := model.GetFromStandard(model.Mexc, symbol)
	path := fmt.Sprintf(`/api/v1/contract/depth/%s`, dialectSymbol)
	respBytes, err := publicRequestMexc(http.MethodGet, contractRestUrl, path, nil, ``)
	if err != nil {
		util.Notice(fmt.Sprintf(`[mexcGetContractSymbolDepth] Failed to get depth info for symbol %s err %+v`, symbol, err))
		return
	}
	resp := &dtos.MexcContractDepthHttpResp{}
	err = json.Unmarshal(respBytes, resp)
	if err != nil || !resp.Success {
		util.Notice(fmt.Sprintf(`[mexcGetContractSymbolDepth] Failed to get depth info %s for symbol %s success %t err %+v`,
			string(respBytes), symbol, resp.Success, err))
		return
	}
	markets.SetBidAsk(symbol, model.Mexc, parseTicksMexc(symbol, resp.Data.Timestamp, resp.Data.Version, resp.Data.Bids, resp.Data.Asks))
}

// endregion
var subscribeHandlerMexc = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error
	for _, subscribe := range subscribes {
		subMsg := fmt.Sprintf(`%s`, subscribe)
		if err = SendToConnection(model.Mexc, connection, []byte(subMsg)); err != nil {
			util.SocketInfo(" mexc can not subscribe %s %s", subscribe, err.Error())
		}
		mexcSymbolConnection.Store(subscribe.(string), connection)
		time.Sleep(time.Millisecond * 100)
	}
	return err
}

func getFloat64OrDefault(val string) float64 {
	ret, err := strconv.ParseFloat(val, 64)
	if err != nil {
		ret = 0
	}
	return ret
}

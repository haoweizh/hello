package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
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
	contractRestUrl              = "contract.mexc.com"
	mexcContractWSUrl            = "wss://contract.mexc.com/ws"
	mexcContractDepthIncSubType  = "mexcContractDepthIncreSubType" // Contract深度增量订阅
	mexcContractDepthFullSubType = "mexcContractDepthFullSubType"  // Contract深度全量订阅（按档位）
	mexcContractTickerSubType    = "mexcContractTickerSubType"
	wsStepMexc                   = 20
)

var (
	pingDepthMexc        = false
	mexcSymbolConnection sync.Map
)

func maintainChannelMexc(subscribes []interface{}) {
	if !pingDepthMexc {
		pingDepthMexc = true
		go func() {
			for {
				time.Sleep(time.Second * 10)
				value, _ := model.AppEnvironment.ConnTick.Load(model.Mexc)
				if value == nil {
					return
				}
				if err := SendToConnections(model.Mexc, value.(map[*websocket.Conn]bool), websocket.TextMessage, []byte(`{"method": "ping"}`)); err != nil {
					util.SocketInfo("mexc channel ping error " + err.Error())
				}
			}
		}()
		for {
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
				_, bidAsk := model.AppEnvironment.GetBidAsk(symbol, model.Mexc)
				if bidAsk == nil || time.Now().UnixMilli()-int64(bidAsk.Ts) > 180000 {
					needReset = true
					break
				} else if time.Now().UnixMilli()-int64(bidAsk.Ts) > 120000 {
					if err := SendToConnection(model.Mexc, conn.(*websocket.Conn), []byte(subscribe.(string))); err != nil {
						util.SocketInfo(" mexc can not subscribe %s %s", subscribe, err.Error())
					}
				}
				time.Sleep(time.Millisecond * 100)
			}
			if !needReset {
				util.Info(`no need reset %s`, model.Mexc)
			} else {
				SetRequireReset(model.Mexc)
			}
		}
	}
}

var wsHandlerMexc = func(market string, event []byte) {
	newJson, wsErr := util.NewJSON(event)
	if wsErr != nil {
		util.SocketInfo(`MEXC fail to unmarshal json ` + wsErr.Error())
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
			success, marketType, coin := model.GetCoinFromDialect(model.Mexc, resp.Symbol)
			if !success {
				return
			}
			symbol := coin + model.UniStandardTail[marketType]
			bidAsk := parseTicksMexc(symbol, resp.Ts, resp.Data.Version, resp.Data.Bids, resp.Data.Asks)
			//fmt.Println(fmt.Sprintf(`%s %f %f ~ %f %f`, symbol, bidAsk.Bids[0].Price, bidAsk.Bids[0].Amount, bidAsk.Asks[0].Price, bidAsk.Asks[0].Amount))
			if model.AppEnvironment.SetBidAsk(symbol, model.Mexc, bidAsk) {
				funcHandlers := GetFunctions(model.Mexc, symbol)
				if funcHandlers != nil {
					funcHandlers.Range(func(function, value interface{}) bool {
						setting := GetSetting(function.(string), model.Mexc, symbol)
						if setting != nil && value != nil && value.(model.CarryHandler) != nil {
							go value.(model.CarryHandler)(setting, bidAsk)
						}
						return true
					})
				}
			}
		}
	}
}

func WsTickServeMexc(environment *model.Environment, market string, useFullDepthSub bool) (socketMap map[*websocket.Conn]bool, msgChans []chan struct{}, connectErr error) {
	symbols := GetMarketSymbols(model.Mexc)
	if !useFullDepthSub {
		limiter := time.Tick(time.Millisecond * 100)
		for symbol := range symbols {
			<-limiter
			initMexcContractDepth(environment, symbol)
		}
	}
	subscribes := make([]interface{}, 0)
	if !useFullDepthSub { // 订阅contract深度增量
		subscribes = GetWSSubscribes(model.Mexc, mexcContractDepthIncSubType)
	} else { // 订阅contract 5档深度全量
		subscribes = GetWSSubscribes(model.Mexc, mexcContractDepthFullSubType)
	}
	socketMap, msgChans, connectErr = WebSocketClient(market, mexcContractWSUrl, subscribes, subscribeHandlerMexc, wsHandlerMexc, wsStepMexc)
	go maintainChannelMexc(subscribes)
	environment.ConnTick.Store(market, socketMap)
	environment.MsgChanTick.Store(market, msgChans)
	return
}

func parseTicksMexc(symbol string, ts int, version int64, bidArray, asksArray [][]float64) *model.BidAsk {
	var bids, asks []model.Tick
	for _, tick := range asksArray {
		if tick == nil || len(tick) != 3 {
			continue
		}
		var marketInfo *model.MarketInfo
		v, _ := util.LoadSyncMap(model.MarketInfos, model.Mexc, symbol)
		if v != nil {
			marketInfo = v.(*model.MarketInfo)
		}
		if marketInfo != nil {
			asks = append(asks, model.Tick{Market: model.Mexc, Symbol: symbol, Price: tick[0], Amount: tick[1] * marketInfo.SizeIncrement})
		}
	}
	for _, tick := range bidArray {
		if tick == nil || len(tick) != 3 {
			continue
		}
		var marketInfo *model.MarketInfo
		v, _ := util.LoadSyncMap(model.MarketInfos, model.Mexc, symbol)
		if v != nil {
			marketInfo = v.(*model.MarketInfo)
		}
		if marketInfo != nil {
			bids = append(bids, model.Tick{Market: model.Mexc, Symbol: symbol, Price: tick[0], Amount: tick[1] * marketInfo.SizeIncrement})
		}
	}
	return &model.BidAsk{Ts: ts, TsReceived: int(time.Now().UnixMilli()), UpdateId: version, Bids: bids, Asks: asks}
}

func initMexcContractDepth(environment *model.Environment, symbol string) {
	_, _, _, dialectSymbol := model.GetFromStandard(model.Mexc, symbol)
	path := fmt.Sprintf(`/api/v1/contract/depth/%s`, dialectSymbol)
	respBytes, err := SignedRequestMexc(``, ``, http.MethodGet, contractRestUrl, path, nil, nil)
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
	environment.SetBidAsk(symbol, model.Mexc, parseTicksMexc(symbol, resp.Data.Timestamp, resp.Data.Version, resp.Data.Bids, resp.Data.Asks))
}

// endregion
var subscribeHandlerMexc = func(market string, connection *websocket.Conn, subscribes []interface{}) error {
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

// placeOrderMexc region place order
func placeOrderMexc(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	var typeInReq string // 1：限价单，2：Post Only只做Maker，3：立即成交或立即取消，4：全部成交或者全部取消，5：市价单，6：市价转现价
	if orderType == model.OrderTypeLimit {
		typeInReq = `1`
	} else if orderType == model.OrderTypeMarket {
		typeInReq = `5`
	}
	var side int
	if orderSide == model.OrderSideBuy {
		side = 1
	} else {
		side = 3
	}
	price, decimal := model.FormatPrice(model.Mexc, symbol, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	var marketInfo *model.MarketInfo
	v, _ := util.LoadSyncMap(model.MarketInfos, model.Mexc, symbol)
	if v != nil {
		marketInfo = v.(*model.MarketInfo)
	}
	order.Price = price
	if marketInfo == nil {
		util.Notice(fmt.Sprintf(`[mexcPlaceOrder] market info is nil for symbol %s`, symbol))
		return
	}
	formattedAmount := strconv.Itoa(int(math.Floor(amount / marketInfo.SizeIncrement)))
	orderAmountReal, _ := strconv.ParseFloat(formattedAmount, 64)
	order.Amount = orderAmountReal * marketInfo.SizeIncrement
	body := map[string]interface{}{
		"symbol":       symbol,
		"side":         side,
		`leverage`:     model.DefaultLeverage,
		"type":         typeInReq,
		"price":        priceStr,
		`vol`:          formattedAmount,
		`openType`:     2,
		`positionMode`: 2,
	}
	respBytes, err := SignedRequestMexc(key, secret, http.MethodPost, contractRestUrl, `/api/v1/private/order/submit`,
		nil, body)
	order.Status = model.CarryStatusFail
	if err != nil {
		order.ErrCode = err.Error()
		return
	}
	orderJson, jsonErr := util.NewJSON(respBytes)
	if jsonErr != nil {
		util.Notice(fmt.Sprintf(`fail to placeOrderMexc %s`, jsonErr.Error()))
		return
	}
	if orderJson != nil && orderJson.Get("success").MustBool() {
		order.Status = model.CarryStatusWorking
		data, _ := orderJson.Get(`data`).Int64()
		order.OrderId = strconv.FormatInt(data, 10)
	} else if orderJson != nil {
		order.ErrCode = strconv.Itoa(orderJson.Get(`code`).MustInt()) + orderJson.Get(`message`).MustString()
	}
	return
}

func SignedRequestMexc(key, secret, method, restUrl, path string, paramsInQuery, body map[string]interface{}) ([]byte, error) {
	var parameters string
	if len(paramsInQuery) > 0 {
		param := &url.Values{}
		for k, v := range paramsInQuery {
			param.Set(k, v.(string))
		}
		parameters = param.Encode()
	}
	if body[`symbol`] != nil && len(body[`symbol`].(string)) > 0 {
		_, _, _, dialectSymbol := model.GetFromStandard(model.Mexc, body[`symbol`].(string))
		body[`symbol`] = dialectSymbol
	}
	bodyStr := string(util.JsonEncodeToByte(body))
	reqTime := strconv.FormatInt(util.GetNow().UnixNano(), 10)[0:13]
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	if len(key) > 0 {
		toBeSign := key + reqTime + parameters
		if method == http.MethodPost {
			toBeSign = key + reqTime + bodyStr
		}
		hash := hmac.New(sha256.New, []byte(secret))
		hash.Write([]byte(toBeSign))
		headers["Request-Time"] = reqTime
		headers["ApiKey"] = key
		headers["signature"] = hex.EncodeToString(hash.Sum(nil))
	}
	requestUrl := fmt.Sprintf(`https://%s%s`, restUrl, path)
	if len(parameters) > 0 {
		requestUrl = requestUrl + "?" + parameters
	}
	responseBody, err := util.HttpRequest(method, requestUrl, bodyStr, headers, 60)
	logMsg := fmt.Sprintf(`Mexc key %s, request %s, headers %v, parameters %s, body %s, return %s err %+v`,
		key, requestUrl, headers, parameters, body, string(responseBody), err)
	util.SocketInfo(logMsg)
	return responseBody, err
}

func cancelOrdersMexc(key string, secret string, symbol string) bool {
	responseBytes, err := SignedRequestMexc(key, secret, http.MethodPost, contractRestUrl,
		"/api/v1/private/order/cancel_all", nil, map[string]interface{}{`symbol`: symbol})
	if err != nil {
		util.Notice(fmt.Sprintf(`[contractCancelOrdersMexc] Failed to cancel orders by symbol %s err %+v`, symbol, err))
		return false
	}
	cancelJson, jsonErr := util.NewJSON(responseBytes)
	if cancelJson != nil && jsonErr == nil {
		return cancelJson.Get("success").MustBool()
	}
	return false
}

func getFundingRateMexc(key, secret, symbol string) (fundingRate *model.FundingRate) {
	_, _, _, dialectSymbol := model.GetFromStandard(model.Mexc, symbol)
	responseBytes, err := SignedRequestMexc(key, secret, http.MethodGet, contractRestUrl,
		"/api/v1/contract/funding_rate/"+dialectSymbol, nil, nil)
	if err != nil {
		util.Notice(fmt.Sprintf(`[contractGetFundingRateMexc] Failed to get funding rate by symbol %s err %+v`, symbol, err))
		return
	}
	fundingRateJson, fundingErr := util.NewJSON(responseBytes)
	if fundingRateJson != nil && fundingRateJson.Get(`success`).MustBool() && fundingErr == nil {
		fundingRate = &model.FundingRate{
			Rate:       fundingRateJson.Get(`data`).Get(`fundingRate`).MustFloat64(),
			UpdateTime: time.UnixMilli(fundingRateJson.Get(`data`).Get(`timestamp`).MustInt64()),
			ExpireTime: fundingRateJson.Get(`data`).Get(`nextSettleTime`).MustInt64() / 1000,
		}
	}
	return
}

func getPositionsMexc(key, secret string) (success bool, positions []*Position, accountValue, availableU float64) {
	valueResponse, valueErr := SignedRequestMexc(key, secret, http.MethodGet, contractRestUrl,
		"/api/v1/private/account/assets", nil, nil)
	posResponse, posErr := SignedRequestMexc(key, secret, http.MethodGet, contractRestUrl,
		`/api/v1/private/position/open_positions`, nil, nil)
	if valueErr != nil || posErr != nil {
		util.Notice(fmt.Sprintf(`[contractGetPositionsMexc] Failed to get positions by key %s err %+v %+v`, key, valueErr, posErr))
		time.Sleep(time.Minute * 5)
		return getPositionsMexc(key, secret)
	}
	valueJson, valueJsonErr := util.NewJSON(valueResponse)
	positionJson, positionErr := util.NewJSON(posResponse)
	if valueJson == nil || !valueJson.Get(`success`).MustBool() || positionJson == nil ||
		!positionJson.Get(`success`).MustBool() || valueJsonErr != nil || positionErr != nil {
		util.Notice(fmt.Sprintf(`[contractGetPositionsMexc] Failed to get positions by key %s err %+v %+v %v %v`,
			key, valueJson, positionJson, valueJsonErr, positionErr))
		return getPositionsMexc(key, secret)
	}
	assets := valueJson.Get(`data`).MustArray()
	for _, item := range assets {
		asset := item.(map[string]interface{})
		if asset[`currency`] != nil && asset[`currency`].(string) == `USDT` {
			if asset[`availableBalance`] != nil {
				availableU, _ = asset[`availableBalance`].(json.Number).Float64()
			}
			if asset[`equity`] != nil {
				accountValue, _ = asset[`equity`].(json.Number).Float64()
			}
		}
	}
	positions = make([]*Position, 0)
	posArray := positionJson.Get(`data`).MustArray()
	for _, item := range posArray {
		position := &Position{Market: model.Mexc, Ts: util.GetNowUnixMillion()}
		pos := item.(map[string]interface{})
		if pos[`symbol`] != nil {
			isSuccess, _, coin := model.GetCoinFromDialect(model.Mexc, pos[`symbol`].(string))
			if !isSuccess {
				continue
			}
			position.Currency = coin + model.UniStandardTail[model.MarketTypePerp]
		}
		//仓位状态,1持仓中2系统代持3已平仓
		if pos[`state`] != nil {
			state, _ := pos[`state`].(json.Number).Int64()
			if state != 1 {
				continue
			}
		}
		var marketInfo *model.MarketInfo
		v, _ := util.LoadSyncMap(model.MarketInfos, model.Mexc, position.Currency)
		if v != nil {
			marketInfo = v.(*model.MarketInfo)
		}
		if marketInfo == nil {
			continue
		}
		if pos[`holdVol`] != nil {
			position.Holding, _ = pos[`holdVol`].(json.Number).Float64()
			position.Holding = position.Holding * marketInfo.SizeIncrement
		}
		if pos[`positionType`] != nil {
			posType, _ := pos[`positionType`].(json.Number).Int64()
			if posType == 2 {
				position.Holding = -position.Holding
			}
		}
		if pos[`frozenVol`] != nil {
			position.Frozen, _ = pos[`frozenVol`].(json.Number).Float64()
			position.Frozen = position.Frozen * marketInfo.SizeIncrement
		}
		if pos[`leverage`] != nil {
			position.LeverRate, _ = pos[`leverage`].(json.Number).Int64()
		}
		if pos[`realised`] != nil {
			position.ProfitReal, _ = pos[`realised`].(json.Number).Float64()
		}
		if pos[`openAvgPrice`] != nil {
			position.EntryPrice, _ = pos[`openAvgPrice`].(json.Number).Float64()
		}
		if pos[`liquidatePrice`] != nil {
			position.LiquidationPrice, _ = pos[`liquidatePrice`].(json.Number).Float64()
		}
		// 初始保证金
		if pos[`im`] != nil {
			position.Margin, _ = pos[`im`].(json.Number).Float64()
		}
		positions = append(positions, position)
	}
	return true, positions, accountValue, availableU
}

func queryOrderMexc(key, secret, symbol string, orderId string) (order *model.Order) {
	order = &model.Order{Market: model.Mexc, OrderId: orderId, Symbol: symbol}
	respBytes, err := SignedRequestMexc(key, secret, http.MethodGet, contractRestUrl,
		fmt.Sprintf("/api/v1/private/order/get/%s", orderId), nil, nil)
	if err != nil {
		logMsg := fmt.Sprintf(`[contractQueryOrderMexc] Failed to query orders by order_id %s err %+v`, order.OrderId, err)
		util.Notice(logMsg)
		return
	}
	resp := &dtos.MexcContractQueryOrderResp{}
	err = json.Unmarshal(respBytes, resp)
	var marketInfo *model.MarketInfo
	v, _ := util.LoadSyncMap(model.MarketInfos, model.Mexc, order.Symbol)
	if v != nil {
		marketInfo = v.(*model.MarketInfo)
	}
	if err != nil || !resp.Success || marketInfo == nil {
		logMsg := fmt.Sprintf(`[contractQueryOrderMexc] Failed to query orders by order_id %s success %t statusCode %d err %+v`, order.OrderId, resp.Success, resp.Code, err)
		util.Notice(logMsg)
		return
	}
	order.Price = resp.Data.Price                                   // 挂单价格
	order.DealPrice = resp.Data.DealAvgPrice                        // 成交金额
	order.DealAmount = resp.Data.DealVol * marketInfo.SizeIncrement // 成交数量
	order.Amount = resp.Data.Vol * marketInfo.SizeIncrement         // 挂单数量
	order.OrderSide = model.OrderSideBuy
	// 订单方向 1开多,2平空,3开空,4平多
	if resp.Data.Side == 3 || resp.Data.Side == 4 {
		order.OrderSide = model.OrderSideSell
		order.Amount *= -1
		order.DealAmount *= -1
	}
	order.CreatedAt = time.Unix(resp.Data.CreateTime, 0)
	// 订单状态,1:待报,2未完成,3已完成,4已撤销,5无效
	switch resp.Data.State {
	case 1, 2:
		order.Status = model.CarryStatusWorking
	case 3:
		order.Status = model.CarryStatusSuccess
	case 4, 5:
		order.Status = model.CarryStatusFail
	}
	return
}

func getMarketsMexc(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	respBytes, err := SignedRequestMexc(key, secret, http.MethodGet, contractRestUrl, `/api/v1/contract/detail`, nil, nil)
	if err != nil {
		return nil
	}
	marketInfos = make(map[string]*model.MarketInfo)
	resp := &dtos.MexcContractGetMarketsResp{}
	err = json.Unmarshal(respBytes, resp)
	if err != nil {
		util.Notice(fmt.Sprintf(`[appendContractMarketMexc] Failed to get contract market symbols, err %s`, err))
		return nil
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
			SizeIncrement:  symbolInfo.ContractSize, //此处记录了一张合约中有多少币数，而非美元数
			PriceIncrement: symbolInfo.PriceUnit,
			CTValue:        0, // 由于张数中没有美元和币种之间的转换，故为0
			PriceDecimal:   symbolInfo.PriceScale,
			SizeMax:        symbolInfo.MaxVol * symbolInfo.ContractSize,
			SizeMin:        symbolInfo.MinVol * symbolInfo.ContractSize,
		}
	}
	return marketInfos
}

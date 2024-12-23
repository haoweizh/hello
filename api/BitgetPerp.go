package api

import (
	"encoding/json"
	"fmt"
	"hello/api/dtos"
	"hello/model"
	"hello/util"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const wsStepBitget = 40

func getMarketsBitgetPerp() (marketInfos map[string]*model.MarketInfo) {
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/v2/mix/market/contracts?productType=USDT-FUTURES", "", map[string]string{}, 30)
	perpResp := &dtos.BitgetPerpMarketResp{}
	perpJsonErr := json.Unmarshal(httpResp, perpResp)
	if perpResp == nil || perpResp.Code != "00000" {
		util.Log(util.LogLevelError, fmt.Sprintf("get bitget perp market error, resp: %s, httpErr: %#v, jsonErr: %#v", httpResp, httpErr, perpJsonErr))
		return
	}
	marketInfos = make(map[string]*model.MarketInfo)
	for _, perpInfo := range perpResp.Data {
		if perpInfo.QuoteCoin != "USDT" || perpInfo.SymbolStatus != "normal" || perpInfo.SymbolType != "perpetual" ||
			perpInfo.OffTime != "-1" || perpInfo.LimitOpenTime != "-1" {
			continue
		}
		symbol := perpInfo.BaseCoin + model.UniStandardTail[model.MarketTypePerp]
		marketInfo := &model.MarketInfo{Market: model.BitgetPerp, Name: symbol, CTCurrency: perpInfo.BaseCoin}
		marketInfo.PriceDecimal, _ = strconv.Atoi(perpInfo.PricePlace)
		priceEndStep, _ := strconv.ParseFloat(perpInfo.PriceEndStep, 64)
		marketInfo.PriceIncrement = priceEndStep * (1 / math.Pow10(marketInfo.PriceDecimal))
		marketInfo.SizeMin, _ = strconv.ParseFloat(perpInfo.MinTradeNum, 64)
		marketInfo.SizeIncrement, _ = strconv.ParseFloat(perpInfo.SizeMultiplier, 64)
		//marketInfo.CTValue, _ = strconv.ParseFloat(perpInfo.ContractSize, 64)
		marketInfo.BuyLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.BuyLimitPriceRatio, 64)
		marketInfo.SellLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.SellLimitPriceRatio, 64)
		marketInfo.MoneyMin, _ = strconv.ParseFloat(perpInfo.MinTradeUSDT, 64)
		marketInfos[symbol] = marketInfo
	}
	return marketInfos
}

// GetBitgetPosModes
func _(account *model.Account, symbol string) (mode string) {
	_, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: account.Key, ApiSecretKey: account.Secret}
	params := map[string]string{"productType": "USDT-FUTURES", "marginCoin": "USDT", `symbol`: dialectSymbol}
	httpResp, httpErr := client.DoGet("/api/v2/mix/account/account", params)
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to do post when setBitgetPositionMode %s`, httpErr.Error()))
		return
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when setBitgetPositionMode %s`, jsonErr.Error()))
		return
	}
	data := jsonData.GetPath(`data`, `posMode`).MustString()
	fmt.Println(data)
	return data
}

func setBitgetPositionMode(key, secret string) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]string{"productType": "USDT-FUTURES", "posMode": "one_way_mode"}
	httpResp, httpErr := client.DoPost("/api/v2/mix/account/set-position-mode", string(util.JsonEncodeToByte(params)))
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to do post when setBitgetPositionMode %s`, httpErr.Error()))
		return
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when setBitgetPositionMode %s`, jsonErr.Error()))
		return
	}
	if jsonData != nil {
		code, codeErr := jsonData.Get("code").String()
		if code != "00000" || codeErr != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("fail to set Bitgetperp Position Mode, resp: %s codeErr: %#v", httpResp, codeErr))
		}
	}
}

var markPriceWsHandler = func(market string, conn *model.WSConn, event []byte) {
	if strings.Contains(string(event), `ping`) {
		err := conn.WriteMsg([]byte(`pong`))
		if err != nil {
			return
		}
		return
	}
	tickerWsResp := &dtos.BitgetTickerWsResp{}
	jsonErr := json.Unmarshal(event, tickerWsResp)
	if jsonErr != nil {
		//util.SocketInfo(`bitget fail to unmarshal ticker ws data json ` + jsonErr.Error())
		return
	}
	if tickerWsResp.Arg.InstType == "USDT-FUTURES" && tickerWsResp.Action == "snapshot" {
		for _, tickerData := range tickerWsResp.Data {
			if tickerData.Symbol == "" {
				continue
			}
			_, _, symbol := model.GetFromDialect(model.BitgetPerp, model.MarketTypePerp, tickerData.Symbol)
			price, _ := strconv.ParseFloat(tickerData.MarkPrice, 64)
			ts, _ := strconv.ParseInt(tickerData.Ts, 10, 64)
			nextTs, _ := strconv.ParseInt(tickerData.NextFundingTime, 10, 64)
			ticker := &model.MarkPriceInfo{MarkPrice: price, Ts: int(ts)}
			model.AppEnvironment.SetMarkPriceInfo(symbol, model.BitgetPerp, ticker)
			rate, _ := strconv.ParseFloat(tickerData.FundingRate, 64)
			fundingRate := &model.FundingRate{
				Rate:       rate,
				UpdateTime: time.UnixMilli(ts),
				ExpireTime: nextTs / 1000,
			}
			SetFundingRate(model.BitgetPerp, symbol, fundingRate)
		}
	}
}

func WsTickServeBitgetPerp(market string) (socketMap map[*model.WSConn]bool, msgChans []chan struct{}, connectErr error) {
	msgChans = make([]chan struct{}, 0)
	socketMap = make(map[*model.WSConn]bool)
	depthSubs := GetWSSubscribes(market, []string{model.SubscribeDepth})
	marketPriceSubs := GetWSSubscribes(market, []string{model.SubscribeMarkPrice})
	markPriceSockets, markPriceChannels, markPriceErr := model.WsPublicClient(market, bitgetPublic,
		marketPriceSubs, subscribeHandlerBitget, markPriceWsHandler, wsStepBitget)
	if markPriceErr == nil {
		msgChans = append(msgChans, markPriceChannels...)
		for conn, b := range markPriceSockets {
			socketMap[conn] = b
		}
	} else {
		return nil, nil, markPriceErr
	}
	perpBookSockets, perpBookChannels, perpBookErr := model.WsPublicClient(market, bitgetPublic,
		depthSubs, subscribeHandlerBitget, tickHandlerBitget, wsStepBitget)
	if perpBookErr == nil {
		util.Log(util.LogLevelInfo, `finish connect public Bitget perp book wss `)
		msgChans = append(msgChans, perpBookChannels...)
		for conn, b := range perpBookSockets {
			socketMap[conn] = b
		}
	} else {
		return nil, nil, perpBookErr
	}
	return
}

func getPositionsBitgetPerp(key, secret string) (success bool, positions []*Position, accountValue, availableU, mmr float64) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	assetHttpResp, assetHttpErr := client.DoGet("/api/v2/mix/account/accounts", map[string]string{"productType": "USDT-FUTURES"})
	bitgetAssertResp := &dtos.BitgetAssertResp{}
	jsonErr := json.Unmarshal(assetHttpResp, bitgetAssertResp)
	if bitgetAssertResp == nil || bitgetAssertResp.Code != "00000" {
		util.Log(util.LogLevelError, fmt.Sprintf("fail to refresh bitgetperp asset , resp: %s httpErr: %#v, jsonErr: %#v",
			assetHttpResp, assetHttpErr, jsonErr))
		time.Sleep(time.Minute)
		return getPositionsBitgetPerp(key, secret)
	}
	positionHttpResp, positionHttpErr := client.DoGet("/api/v2/mix/position/all-position", map[string]string{"productType": "USDT-FUTURES"})
	bitgetPositionResp := &dtos.BitgetPositionResp{}
	positionJsonErr := json.Unmarshal(positionHttpResp, bitgetPositionResp)
	if bitgetPositionResp == nil || bitgetPositionResp.Code != "00000" {
		util.Log(util.LogLevelError, fmt.Sprintf("fail to refresh bitgetperp position, resp: %s httpErr: %#v, jsonErr: %#v",
			positionHttpResp, positionHttpErr, positionJsonErr))
		time.Sleep(time.Minute)
		return getPositionsBitgetPerp(key, secret)
	}
	for _, asset := range bitgetAssertResp.Data {
		if asset.MarginCoin == `USDT` {
			availableU, _ = strconv.ParseFloat(asset.Available, 64)
			accountValue, _ = strconv.ParseFloat(asset.AccountEquity, 64)
		}
		mmr, _ = strconv.ParseFloat(asset.CrossedRiskRate, 64)
	}
	positions = make([]*Position, 0)
	for _, contract := range bitgetPositionResp.Data {
		isSuccess, _, symbol := model.GetFromDialect(model.BitgetPerp, model.MarketTypePerp, contract.Symbol)
		if !isSuccess {
			continue
		}
		currency := symbol
		position := &Position{Market: model.BitgetPerp, Ts: util.GetNowUnixMillion(), Currency: currency}
		position.Direction = contract.HoldSide
		total, _ := strconv.ParseFloat(contract.Total, 64)
		if total == 0 {
			continue
		}
		if position.Direction == "long" {
			position.Frozen, _ = strconv.ParseFloat(contract.Locked, 64)
			position.Holding = total
		} else {
			frozen, _ := strconv.ParseFloat(contract.Locked, 64)
			position.Frozen = -1 * frozen
			position.Holding = -1 * total
		}
		position.LeverRate = int64(contract.Leverage)
		position.EntryPrice, _ = strconv.ParseFloat(contract.OpenPriceAvg, 64)
		position.Margin, _ = strconv.ParseFloat(contract.MarginSize, 64)
		if position.Holding != 0 {
			positions = append(positions, position)
			//util.Log(util.LogLevelInfo, fmt.Sprintf(`get position bitgetperp %#v`, position))
		}
	}
	if len(positions) == 0 && accountValue > 0 {
		util.Log(util.LogLevelError, fmt.Sprintf(`get pos error bitgetperp %d`, len(bitgetPositionResp.Data)))
	}
	return true, positions, accountValue, availableU, mmr
}

func getFundingRateBitgetPerp(symbol string) (fundingRate *model.FundingRate) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Log(util.LogLevelError, "fail to get perp funding rate , GetFromStandard: "+symbol)
		return
	}
	path := `/api/v2/mix/market/current-fund-rate`
	httpResp, httpErr := util.HttpRequest(http.MethodGet, fmt.Sprintf(`%s%s?symbol=%s&productType=%s`,
		bitgetRestUrl, path, dialectSymbol, `USDT-FUTURES`), ``, nil, 30)
	bitgetFundingResp := &dtos.BitgetFundingResp{}
	perpJsonErr := json.Unmarshal(httpResp, bitgetFundingResp)
	if bitgetFundingResp == nil || bitgetFundingResp.Code != "00000" {
		util.Log(util.LogLevelError, fmt.Sprintf("get bitget perp funding rate error, %s resp: %s, httpErr: %#v, jsonErr: %#v",
			symbol, httpResp, httpErr, perpJsonErr))
		return
	}
	if len(bitgetFundingResp.Data) == 0 {
		return &model.FundingRate{Rate: 0, UpdateTime: util.GetNow(), ExpireTime: util.GetNow().Unix() + 3600} //没有过期时间
	}
	data := bitgetFundingResp.Data[0]
	rate, _ := strconv.ParseFloat(data.FundingRate, 64)
	return &model.FundingRate{Rate: rate, UpdateTime: util.GetNow(), ExpireTime: util.GetNow().Unix() + 3600} //没有过期时间
}

func placeOrderBitgetPerp(key, secret string, order *model.Order, orderSide, orderType, orderParam, symbol string, price, amount float64) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Log(util.LogLevelError, "fail to place perp order, GetFromStandard: "+symbol)
		return
	}
	reduceOnly := false
	reduceOnlyStr := `NO`
	if orderParam == model.ReduceOnly {
		reduceOnly = true
		reduceOnlyStr = `YES`
	}
	formattedPrice, decimalPrice := model.FormatPrice(model.BitgetPerp, symbol, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.BitgetPerp, symbol, amount, formattedPrice, reduceOnly)))
	priceStr := util.CutTailZero(strconv.FormatFloat(formattedPrice, 'f', decimalPrice, 64))
	ordType := ``
	if orderType == model.OrderTypeMarket {
		ordType = `market`
	} else if orderType == model.OrderTypeLimit {
		ordType = `limit`
	}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]interface{}{
		"symbol":      dialectSymbol,
		"marginCoin":  "USDT",
		`productType`: `USDT-FUTURES`,
		`marginMode`:  `crossed`,
		"size":        amountStr,
		"price":       priceStr,
		"clientOid":   order.ClientOrdId,
		"side":        orderSide,
		"orderType":   ordType,
		"reduceOnly":  reduceOnlyStr}
	httpResp, httpErr := client.DoPost("/api/v2/mix/order/place-order", string(util.JsonEncodeToByte(params)))
	bitgetOrderResp := &dtos.BitgetOrderResp{}
	jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
	util.Log(util.LogLevelInfo, fmt.Sprintf(`place bitgetperp %#v`, params))
	if bitgetOrderResp == nil {
		util.Log(util.LogLevelError, fmt.Sprintf("fail to create bitget perp order no resp: %s httpErr: %#v, jsonErr: %#v",
			httpResp, httpErr, jsonErr))
	} else {
		if len(strings.Trim(bitgetOrderResp.Code, `0`)) == 0 {
			order.Status = model.CarryStatusWorking
			order.OrderId = bitgetOrderResp.Data.OrderId
		} else {
			order.ErrCode = bitgetOrderResp.Code
			order.Status = model.CarryStatusFail
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf("create bitget perp order resp: %s httpErr: %#v, jsonErr: %#v",
			httpResp, httpErr, jsonErr))
	}
}

func cancelOrderBitgetPerp(key, secret, symbol, orderId string) (result bool) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Log(util.LogLevelError, "fail to cancelOrderBitgetPerp GetFromStandard: "+symbol)
		return false
	}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	httpResp, httpErr := client.DoPost(`/api/v2/mix/order/cancel-order`, string(util.JsonEncodeToByte(map[string]string{
		`productType`: `USDT-FUTURES`, `symbol`: dialectSymbol, `orderId`: orderId})))
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to cancelOrderBitgetPerp %s`, httpErr.Error()))
		return false
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when cancelOrderBitgetPerp %s`, jsonErr.Error()))
		return false
	}
	if jsonData != nil {
		code, _ := jsonData.Get("code").String()
		if code == "00000" {
			util.Log(util.LogLevelInfo, fmt.Sprintf("success to cancel bitgetPerp order code %s", code))
			return true
		} else {
			util.Log(util.LogLevelError, fmt.Sprintf("fail to cancelOrder BitgetPerp code %s msg %s %s",
				code, jsonData.Get(`msg`).MustString(), string(httpResp)))
		}
	}
	return false
}

func cancelOrdersBitgetPerp(key, secret, symbol string) (result bool) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Log(util.LogLevelError, "fail to cancelOrdersBitgetPerp GetFromStandard: "+symbol)
		return false
	}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	httpResp, httpErr := client.DoPost(`/api/v2/mix/order/batch-cancel-orders`, string(util.JsonEncodeToByte(map[string]string{
		`productType`: `USDT-FUTURES`, `symbol`: dialectSymbol})))
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to do post when cancelOrdersBitgetPerp %s`, httpErr.Error()))
		return false
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when cancelOrdersBitgetPerp %s`, jsonErr.Error()))
		return false
	}
	if jsonData != nil {
		code, _ := jsonData.Get("code").String()
		if code == "00000" {
			util.Log(util.LogLevelInfo, fmt.Sprintf("success to cancel bitgetPerp orders code %s %d",
				code, len(jsonData.GetPath(`data`, `successList`).MustArray())))
			return true
		} else {
			util.Log(util.LogLevelError, fmt.Sprintf("fail to cancel bitget perp order, code: %s %s %s",
				code, jsonData.Get("msg").MustString(), string(httpResp)))
		}
	}
	return false
}
func cancelAllBitgetPerp(key, secret string) (result bool) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	httpResp, httpErr := client.DoPost(`/api/v2/mix/order/cancel-all-orders`,
		string(util.JsonEncodeToByte(map[string]string{`productType`: `USDT-FUTURES`})))
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to do post when cancelAllBitgetPerp %s`, httpErr.Error()))
		return
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when cancelAllBitgetPerp %s`, jsonErr.Error()))
		return
	}
	if jsonData != nil {
		code, _ := jsonData.Get("code").String()
		if code == "00000" {
			util.Log(util.LogLevelInfo, fmt.Sprintf("success to cancelAllBitgetPerp code %s %d",
				code, len(jsonData.GetPath(`data`, `successList`).MustArray())))
			return true
		} else {
			util.Log(util.LogLevelError, fmt.Sprintf("fail to cancelAllBitgetPerp, code: %s %s",
				code, string(httpResp)))
		}
	}
	return false
}

// queryOpenOrdersBitgetperp目前只查询live
// 若未指定，将查询所有状态live 等待成交（尚未有任何成交） live: 未成交；partially_filled：部分成交
func queryOpenOrdersBitgetPerp(key, secret string) (orders []*model.Order) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	httpResp, httpErr := client.DoGet(`/api/v2/mix/order/orders-pending`, map[string]string{`productType`: `USDT-FUTURES`})
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to do get queryOpenOrdersBitgetPerp %s`, httpErr.Error()))
		return
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to marshal queryOpenOrdersBitgetPerp %s`, jsonErr.Error()))
		return nil
	}
	if jsonData.Get("msg").MustString() != `success` {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to queryOpenOrdersBitgetPerp return code %s msg %s`,
			jsonData.Get("code").MustString(), jsonData.Get("msg").MustString()))
		return nil
	}
	orders = []*model.Order{}
	array := jsonData.GetPath(`data`, `entrustedList`).MustArray()
	for _, data := range array {
		value := data.(map[string]interface{})
		_, _, symbol := model.GetFromDialect(model.BitgetPerp, model.MarketTypePerp, value[`symbol`].(string))
		order := &model.Order{Market: model.BitgetPerp, Symbol: symbol, OrderId: value[`orderId`].(string), ClientOrdId: value[`clientOid`].(string)}
		if value[`size`] != nil {
			order.Amount, _ = strconv.ParseFloat(value[`size`].(string), 64)
		}
		if value[`baseVolume`] != nil {
			order.DealAmount, _ = strconv.ParseFloat(value[`baseVolume`].(string), 64)
		}
		if value[`fee`] != nil {
			order.Fee, _ = strconv.ParseFloat(value[`fee`].(string), 64)
		}
		if value[`price`] != nil {
			order.Price, _ = strconv.ParseFloat(value[`price`].(string), 64)
		}
		if value[`priceAvg`] != nil {
			order.DealPrice, _ = strconv.ParseFloat(value[`priceAvg`].(string), 64)
		}
		switch value[`status`] {
		case `live`, `partially_filled`:
			order.Status = model.CarryStatusWorking
		case `canceled`:
			order.Status = model.CarryStatusFail
		case `filled`:
			order.Status = model.CarryStatusSuccess
		}
		order.OrderSide = value[`side`].(string)
		if value[`orderType`] == `limit` {
			order.OrderType = model.OrderTypeLimit
		} else if value[`orderType`] == `market` {
			order.OrderType = model.OrderTypeMarket
		}
		if value[`cTime`] != nil {
			cTime, _ := strconv.ParseInt(value[`cTime`].(string), 10, 64)
			order.OrderTime = time.UnixMilli(cTime)
		}
		if value[`uTime`] != nil {
			uTime, _ := strconv.ParseInt(value[`uTime`].(string), 10, 64)
			order.OrderUpdateTime = time.UnixMilli(uTime)
		}
		orders = append(orders, order)
	}
	return orders
}

func queryOrderBitgetPerp(key, secret, symbol string, orderId string) (order *model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetPerp, symbol)
	if !success {
		util.Log(util.LogLevelError, "fail to query bitget perp order, GetFromStandard: "+symbol)
		return order
	}
	order = &model.Order{Market: model.BitgetPerp, Status: model.CarryStatusWorking, OrderId: orderId, Symbol: symbol}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	param := map[string]string{"symbol": dialectSymbol, "orderId": orderId, `productType`: `USDT-FUTURES`}
	httpResp, httpErr := client.DoGet("/api/v2/mix/order/detail", param)
	orderDetailResp := &dtos.BitgetPerpOrderDetailResp{}
	perpJsonErr := json.Unmarshal(httpResp, orderDetailResp)
	if orderDetailResp == nil || orderDetailResp.Code != "00000" {
		util.Log(util.LogLevelError, fmt.Sprintf(
			"get bitget perp order detail error, resp: %s, httpErr: %#v, jsonErr: %#v", httpResp, httpErr, perpJsonErr))
		return order
	} else {
		order.DealPrice, _ = strconv.ParseFloat(orderDetailResp.Data.PriceAvg, 64)
		order.Amount, _ = strconv.ParseFloat(orderDetailResp.Data.Size, 64)
		order.OrderId = orderDetailResp.Data.OrderId
		order.ClientOrdId = orderDetailResp.Data.ClientOid
		order.DealAmount, _ = strconv.ParseFloat(orderDetailResp.Data.BaseVolume, 64)
		order.Fee, _ = strconv.ParseFloat(orderDetailResp.Data.Fee, 64)
		order.Price, _ = strconv.ParseFloat(orderDetailResp.Data.Price, 64)
		order.OrderSide = orderDetailResp.Data.Side
		order.OrderType = orderDetailResp.Data.OrderType
		intOrderTime, _ := strconv.ParseInt(orderDetailResp.Data.CTime, 10, 64)
		order.OrderTime = time.UnixMilli(intOrderTime)
		intUpdateTime, _ := strconv.ParseInt(orderDetailResp.Data.UTime, 10, 64)
		order.OrderUpdateTime = time.UnixMilli(intUpdateTime)
		if orderDetailResp.Data.State == "canceled" {
			order.Status = model.CarryStatusFail
		} else if orderDetailResp.Data.State == "filled" {
			order.Status = model.CarryStatusSuccess
		} else if orderDetailResp.Data.State == "partially_filled" {
			order.Status = model.CarryStatusWorking
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`query result %#v %#v`, order, orderDetailResp))
	}
	return order
}

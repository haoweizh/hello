package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

const bitgetRestUrl = "https://api.bitget.com"
const bitgetPublic = "wss://ws.bitget.com/v2/ws/public"
const bitgetPrivate = `wss://ws.bitget.com/v2/ws/private`

func maintainConnsBitget(market string, accounts []*model.Account) {
	for {
		connTick, _ := model.AppEnvironment.ConnTick.Load(market)
		if connTick != nil {
			if err := SendToConnections(market, connTick.(map[*model.WSConn]bool), []byte(`ping`)); err != nil {
				util.Log(``, util.LogLevelError, ``, util.SystemAPI, fmt.Sprintf("tick conn maintain error %s %s", market, err.Error()))
			}
		}
		for _, account := range accounts {
			success := false
			valueUpdate, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, account.Key)
			if valueUpdate != nil {
				if err := valueUpdate.(*model.WSConn).WriteMsg([]byte(`ping`)); err != nil {
					util.Log(``, util.LogLevelError, ``, util.SystemAPI, fmt.Sprintf("order update conn maintain error %s %s", market, err.Error()))
				} else {
					success = true
				}
			}
			if !success {
				util.DelSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, account.Key)
				WsOrderServeBitget(market, account)
			}
		}
		time.Sleep(time.Second * 20)
	}
}

var wsOrderConnHandlerBitget = func(market, key string, event []byte) {
	value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, key)
	if value == nil || value.(*model.WSConn).Conn == nil {
		return
	}
	resJson, _ := util.NewJSON(event)
	if resJson == nil {
		return
	}
	instType := ``
	if market == model.BitgetSpot {
		instType = `SPOT`
	} else if market == model.BitgetPerp {
		instType = `USDT-FUTURES`
	}
	if resJson.Get(`event`).MustString() == `login` && resJson.Get(`code`).MustInt() == 0 {
		err := SendToConnection(market, value.(*model.WSConn), []byte(
			fmt.Sprintf(`{"op":"subscribe","args":[{"instType": "%s","channel":"orders","instId":"default"}]}`, instType)))
		if err != nil {
			util.DelSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, key)
		}
	}
	dataArray := resJson.Get(`data`).MustArray()
	for _, data := range dataArray {
		orderId := data.(map[string]interface{})[`orderId`].(string)
		dealAmount, _ := strconv.ParseFloat(data.(map[string]interface{})[`accBaseVolume`].(string), 64)
		status := model.CarryStatusWorking
		if data.(map[string]interface{})[`status`].(string) == `filled` {
			status = model.CarryStatusSuccess
		}
		UpdateOrderDeal(market, orderId, status, string(event), dealAmount)
	}
}

func wsLoginBitget(account *model.Account, conn *model.WSConn) (success bool) {
	ts := time.Now().Unix()
	hash := hmac.New(sha256.New, []byte(account.Secret))
	hash.Write([]byte(fmt.Sprintf(`%dGET/user/verify`, ts)))
	sign := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	if err := conn.WriteMsg([]byte(fmt.Sprintf(`{"op":"login","args":[{"apiKey":"%s","passphrase":"%s","timestamp":"%d","sign":"%s"}]}`,
		account.Key, model.AppConfig.Phase, ts, sign))); err != nil {
		return false
	}
	return true
}

func WsOrderServeBitget(market string, account *model.Account) {
	if account == nil {
		return
	}
	conn, err := model.WsAccountClient(market, account.Key, bitgetPrivate, wsOrderConnHandlerBitget)
	if err != nil {
		util.Log(``, util.LogLevelError, ``, util.SystemNetwork, "can not create web socket "+err.Error())
	} else if conn != nil {
		if wsLoginBitget(account, conn) {
			util.StoreSyncMap(&model.AppEnvironment.ConnOrderUpdate, conn, market, account.Key)
		}
	}
}

func getMarketsBitgetSpot() (marketInfos map[string]*model.MarketInfo) {
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/v2/spot/public/symbols", "", map[string]string{}, 30)
	spotResp := &dtos.BitgetSpotMarketResp{}
	spotJsonErr := json.Unmarshal(httpResp, spotResp)
	if spotResp == nil || spotResp.Code != "00000" {
		util.Log(``, util.LogLevelError, ``, util.SystemAPI, fmt.Sprintf(fmt.Sprintf(
			"get bitget spot market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, spotJsonErr)))
		return
	}
	marketInfos = make(map[string]*model.MarketInfo)
	for _, symbolInfo := range spotResp.Data {
		if symbolInfo.Status != "online" || symbolInfo.QuoteCoin != "USDT" {
			continue
		}
		tail := model.UniStandardTail[model.MarketTypeSpot]
		symbol := symbolInfo.BaseCoin + tail
		marketInfo := &model.MarketInfo{Name: symbol, Market: model.BitgetSpot, CTCurrency: strings.ToUpper(symbolInfo.BaseCoin)}
		marketInfo.PriceDecimal, _ = strconv.Atoi(symbolInfo.PricePrecision)
		marketInfo.PriceIncrement = 1 / math.Pow10(marketInfo.PriceDecimal)
		amountPrecision, _ := strconv.Atoi(symbolInfo.QuantityPrecision)
		marketInfo.SizeIncrement = 1 / math.Pow10(amountPrecision)
		marketInfo.SizeMin, _ = strconv.ParseFloat(symbolInfo.MinTradeAmount, 64)
		if marketInfo.SizeMin == 0 {
			marketInfo.SizeMin = marketInfo.SizeIncrement
		}
		marketInfo.SizeMax, _ = strconv.ParseFloat(symbolInfo.MaxTradeAmount, 64)
		if tail == `_USDT` {
			marketInfo.MoneyMin, _ = strconv.ParseFloat(symbolInfo.MinTradeUSDT, 64)
		}
		//marketInfo.BuyLimitPriceRatio, _ = strconv.ParseFloat(symbolInfo.BuyLimitPriceRatio, 64)
		//marketInfo.SellLimitPriceRatio, _ = strconv.ParseFloat(symbolInfo.SellLimitPriceRatio, 64)
		marketInfos[marketInfo.Name] = marketInfo
	}
	return marketInfos
}

func parseBidAskBitget(bookWsResp *dtos.BitgetBoosWsResp) (bidAsk *model.BidAsk) {
	if bookWsResp == nil {
		return nil
	}
	market := model.BitgetSpot
	if bookWsResp.Arg.InstType == `SPOT` {
		market = model.BitgetSpot
	} else if bookWsResp.Arg.InstType == `USDT-FUTURES` {
		market = model.BitgetPerp
	} else {
		return nil
	}
	success, marketType, coin := model.GetCoinFromDialect(market, bookWsResp.Arg.InstId)
	if !success {
		return nil
	}
	symbol := coin + model.UniStandardTail[marketType]
	switch bookWsResp.Action {
	case `snapshot`:
		bidAsk = &model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
		if len(bookWsResp.Data) > 1 ||
			len(bookWsResp.Data[0].Bids) < 1 || len(bookWsResp.Data[0].Bids[0]) < 2 ||
			len(bookWsResp.Data[0].Asks) < 1 || len(bookWsResp.Data[0].Asks[0]) < 2 {
			return nil
		}
		bidPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][0], 64)
		bidAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][1], 64)
		bids := make([]model.Tick, 0)
		bids = append(bids, model.Tick{Price: bidPrice, Amount: bidAmount, Market: market, Symbol: symbol})
		bidAsk.Bids = bids
		askPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][0], 64)
		askAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][1], 64)
		asks := make([]model.Tick, 0)
		asks = append(asks, model.Tick{Price: askPrice, Amount: askAmount, Market: market, Symbol: symbol})
		bidAsk.Asks = asks
		bidAsk.Ts, _ = strconv.Atoi(bookWsResp.Data[0].Ts)
		bidAsk.UpdateId, _ = strconv.ParseInt(bookWsResp.Data[0].Ts, 10, 64)
	case `update`:

	}
	return bidAsk
}

var tickHandlerBitget = func(market string, conn *model.WSConn, event []byte) {
	if strings.Contains(string(event), `ping`) {
		err := conn.WriteMsg([]byte(`pong`))
		if err != nil {
			return
		}
		return
	}
	bookWsResp := &dtos.BitgetBoosWsResp{}
	jsonErr := json.Unmarshal(event, bookWsResp)
	if jsonErr != nil {
		//util.SocketInfo(`bitget fail to unmarshal book ws data json ` + jsonErr.Error())
		return
	}
	bidAsk := parseBidAskBitget(bookWsResp)
	if bidAsk == nil || bidAsk.Bids.Len() == 0 {
		return
	}
	symbol := bidAsk.Bids[0].Symbol
	haveOld, old := model.AppEnvironment.GetBidAsk(market, symbol)
	if haveOld && old.UpdateId > bidAsk.UpdateId {
		return
	}
	if model.AppEnvironment.SetBidAsk(market, symbol, bidAsk) {
		funcHandlers := GetFunctions(market, symbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), market, symbol)
				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
					go value.(model.CarryHandler)(setting, bidAsk)
				}
				return true
			})
		}
	}
}

var subscribeHandlerBitget = func(market string, connection *model.WSConn, subscribes []interface{}) error {
	var err error = nil
	var params []string
	for _, subscribe := range subscribes {
		params = append(params, subscribe.(string))
	}
	subscribeMap := make(map[string]interface{})
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = params
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(market, connection, subscribeMessage); err != nil {
		util.Log(``, util.LogLevelInfo, ``, util.SystemAPI, fmt.Sprintf("%s can not subscribe %s %s", market, subscribeMessage, err.Error()))
	}
	util.Log(``, util.LogLevelInfo, ``, util.SystemAPI, `bitget subscribed `+string(subscribeMessage))
	return err
}

func getBalanceBitgetSpot(key string, secret string) (success bool, balances []*model.Balance) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	httpResp, httpErr := client.DoGet("/api/v2/spot/account/assets", map[string]string{})
	bitgetBalanceResp := &dtos.BitgetBalanceResp{}
	jsonErr := json.Unmarshal(httpResp, bitgetBalanceResp)
	if bitgetBalanceResp == nil || bitgetBalanceResp.Code != "00000" {
		util.Log(``, util.LogLevelError, ``, util.SystemAPI, fmt.Sprintf(
			"fail to refresh bitgetspot balance, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		time.Sleep(time.Second * 2)
		return getBalanceBitgetSpot(key, secret)
	} else {
		util.Log(``, util.LogLevelInfo, ``, util.SystemAPI, fmt.Sprintf("get bitgetspot balance success, resp: %s ", httpResp))
	}
	balances = make([]*model.Balance, 0)
	for _, account := range bitgetBalanceResp.Data {
		balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.BitgetSpot, Coin: strings.ToUpper(account.CoinName)}
		balance.FrozenAmount, _ = strconv.ParseFloat(account.Frozen, 64)
		balance.AvailableWithBorrow, _ = strconv.ParseFloat(account.Available, 64)
		balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
		priceGet, price := GetPriceForce(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.BitgetSpot)
		//priceGet, bidAsk := model.AppEnvironment.GetBidAsk(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.BitgetSpot)
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
	ordType := ``
	if orderType == model.OrderTypeMarket {
		ordType = `market`
	} else if orderType == model.OrderTypeLimit {
		ordType = `limit`
	}
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.BitgetSpot, symbol, amount, priceSpot, false)))
	if orderSide == model.OrderSideBuy && orderType == model.OrderTypeMarket {
		amountStr = fmt.Sprintf(`%f`, amount*priceSpot)
	}
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
	if !success {
		util.Log(key, util.LogLevelError, ``, util.SystemAPI, "fail to place spot order, GetFromStandard: "+symbol)
		return
	}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]interface{}{
		"symbol":    dialectSymbol,
		"force":     "gtc",
		"size":      amountStr,
		"price":     priceStr,
		"side":      orderSide,
		"orderType": ordType,
	}
	httpResp, httpErr := client.DoPost("/api/v2/spot/trade/place-order", string(util.JsonEncodeToByte(params)))
	bitgetOrderResp := &dtos.BitgetOrderResp{}
	jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
	if bitgetOrderResp == nil {
		util.Log(key, util.LogLevelError, ``, util.SystemAPI, fmt.Sprintf("fail to create bitget spot order no resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	} else if len(strings.Trim(bitgetOrderResp.Code, `0`)) == 0 {
		order.Status = model.CarryStatusWorking
		order.OrderId = bitgetOrderResp.Data.OrderId
	} else {
		order.Status = model.CarryStatusFail
		order.ErrCode = bitgetOrderResp.Code
		util.Log(key, util.LogLevelError, ``, util.SystemAPI, fmt.Sprintf("fail to create bitget spot order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	}
}

func cancelOrdersBitgetSpot(key, secret, symbol string) (result bool) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	success, _, _, dialectSymbol := model.GetFromStandard(model.BitgetSpot, symbol)
	if !success {
		util.Log(key, util.LogLevelError, ``, util.SystemAPI, "fail to cancel bitget spot order, GetFromStandard: "+symbol)
		return false
	}
	httpResp, httpErr := client.DoPost("/api/v2/spot/trade/cancel-symbol-order", string(util.JsonEncodeToByte(map[string]interface{}{"symbol": dialectSymbol})))
	if httpErr != nil {
		util.Log(key, util.LogLevelError, ``, util.SystemNetwork, fmt.Sprintf(`fail to post when cancelOrdersBitgetSpot %s`, httpErr.Error()))
		return false
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(key, util.LogLevelError, ``, util.SystemAPI, fmt.Sprintf(`fail to NewJson when cancelOrdersBitgetSpot %s`, jsonErr.Error()))
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
	order = &model.Order{Market: model.BitgetSpot, Status: model.CarryStatusWorking, OrderId: orderId, Symbol: symbol}
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	httpResp, httpErr := client.DoGet("/api/v2/spot/trade/orderInfo", map[string]string{`orderId`: orderId})
	orderDetailResp := &dtos.BitgetSpotOrderDetailResp{}
	perpJsonErr := json.Unmarshal(httpResp, orderDetailResp)
	if orderDetailResp == nil || orderDetailResp.Code != "00000" {
		util.Log(key, util.LogLevelError, ``, util.SystemAPI, fmt.Sprintf("get bitget spot order detail error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
		return order
	} else {
		if len(orderDetailResp.Data) > 0 {
			orderResp := orderDetailResp.Data[0]
			order.DealPrice, _ = strconv.ParseFloat(orderResp.PriceAvg, 64)
			order.DealAmount, _ = strconv.ParseFloat(orderResp.BaseVolume, 64)
			intOrderTime, _ := strconv.ParseInt(orderResp.CTime, 10, 64)
			order.OrderTime = time.Unix(intOrderTime, 0)
			order.Amount, _ = strconv.ParseFloat(orderResp.Size, 64)
			order.Price, _ = strconv.ParseFloat(orderResp.Price, 64)
			order.UnfilledQuantity = order.Amount - order.DealAmount
			order.OrderId = orderResp.OrderId
			order.OrderSide = orderResp.Side
			if orderResp.Status == "cancelled" {
				order.Status = model.CarryStatusFail
			} else if orderResp.Status == "full_fill" || orderResp.Status == "partial_fill" {
				order.Status = model.CarryStatusSuccess
			}
		}
	}
	util.Log(key, util.LogLevelInfo, ``, util.SystemAPI, fmt.Sprintf(`%s %s %s query result:%s %f %v`,
		order.Market, order.Symbol, order.OrderId, order.Status, order.DealAmount, orderDetailResp))
	return order
}

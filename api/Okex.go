package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hello/model"
	"hello/util"
	"net/http"
	"sort"
	"strconv"
	"time"
)

//type OKEXMessage struct {
//	Binary  int    `json:"binary"`
//	Channel string `json:"channel"`
//	Data    struct {
//		Asks      [][]string `json:"asks"`
//		Bids      [][]string `json:"bids"`
//		Timestamp int        `json:"timestamp"`
//	} `json:"data"`
//}

var subscribeHandlerOKEX = func(subscribes []interface{}, subType string) error {
	var err error = nil
	for _, v := range subscribes {
		subscribeMap := make(map[string]interface{})
		subscribeMap["op"] = "subscribe"
		subscribeMap["args"] = []map[string]string{{`channel`: `books5`, `instId`: v.(string)}}
		subscribeMessage := util.JsonEncodeMapToByte(subscribeMap)
		if err = sendToWs(model.OKEX, subscribeMessage); err != nil {
			util.SocketInfo("okex can not subscribe " + err.Error())
			return err
		}
	}
	return err
}

func WsDepthServeOKEX(markets *model.Markets, errHandler ErrHandler) (chan struct{}, error) {
	lastPingTime := util.GetNow().Unix()
	wsHandler := func(event []byte) {
		if util.GetNow().Unix()-lastPingTime > 20 { // ping okex server every 30 seconds
			lastPingTime = util.GetNow().Unix()
			if err := sendToWs(model.OKEX, []byte(`ping`)); err != nil {
				util.SocketInfo("okex server ping client error " + err.Error())
			}
		}
		responseJson, err := util.NewJSON(event)
		if err != nil || responseJson == nil || responseJson.Get(`data`) == nil ||
			len(responseJson.Get(`data`).MustArray()) == 0 || responseJson.GetPath(`arg`, `instId`) == nil {
			return
		}
		data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
		bidAsk := model.BidAsk{TsReceived: int(util.GetNowUnixMillion())}
		if data[`ts`] != nil {
			ts, _ := strconv.ParseInt(data[`ts`].(string), 10, 64)
			bidAsk.Ts = int(ts)
		}
		instrument := responseJson.GetPath(`arg`, `instId`).MustString()
		asks := data[`asks`].([]interface{})
		bidAsk.Asks = make([]model.Tick, len(asks))
		bids := data[`bids`].([]interface{})
		bidAsk.Bids = make([]model.Tick, len(bids))
		for i, ask := range asks {
			value := ask.([]string)
			if len(value) >= 2 {
				price, _ := strconv.ParseFloat(value[0], 64)
				amount, _ := strconv.ParseFloat(value[1], 64)
				bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Symbol: instrument}
			}
		}
		for i, bid := range bids {
			value := bid.([]string)
			if len(value) >= 2 {
				price, _ := strconv.ParseFloat(value[0], 64)
				amount, _ := strconv.ParseFloat(value[1], 64)
				bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Symbol: instrument}
			}
		}
		sort.Sort(bidAsk.Asks)
		sort.Sort(sort.Reverse(bidAsk.Bids))
		symbol := model.GetInstrumentSymbol(model.OKEX, instrument)
		if markets.SetBidAsk(instrument, model.OKEX, &bidAsk) {
			for function, handler := range model.GetFunctions(model.OKEX, symbol) {
				settings := model.GetSetting(function, model.OKEX, symbol)
				for _, setting := range settings {
					go handler(setting, &bidAsk)
				}
			}
		}
	}
	return WebSocketClient(model.OKEX, model.AppConfig.WSUrls[model.OKEX], model.SubscribeDepth,
		GetWSSubscribes(model.OKEX, model.SubscribeDepth), subscribeHandlerOKEX, wsHandler, errHandler)
}

func sendSignRequestOKEX(key, secret, method, path string, body map[string]interface{}) []byte {
	if key == `` || secret == `` {
		keys, secrets := model.AppConfig.GetKeys(model.OKEX)
		key = keys[0]
		secret = secrets[0]
	}
	if body == nil {
		body = make(map[string]interface{})
	}
	uri := model.AppConfig.RestUrls[model.OKEX] + path
	epoch := time.Now().UnixNano() / int64(time.Millisecond)
	timestamp := fmt.Sprintf(`%d.%d`, epoch/1000, epoch%1000)
	toBeSign := fmt.Sprintf(`%s%s%s`, timestamp, method, path)
	headers := map[string]string{`OK-ACCESS-KEY`: key, `OK-ACCESS-PASSPHRASE`: model.AppConfig.Phase,
		"OK-ACCESS-TIMESTAMP": timestamp}
	if method == http.MethodPost {
		toBeSign = toBeSign + string(util.JsonEncodeMapToByte(body))
		headers["Content-Type"] = "application/json"
	} else if method == http.MethodGet {
		uri = uri + `?` + util.ComposeParams(body)
	}
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(toBeSign))
	sign := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	headers[`OK-ACCESS-SIGN`] = sign
	responseBody, _ := util.HttpRequest(method, uri, string(util.JsonEncodeMapToByte(body)), headers, 60)
	util.SocketInfo(fmt.Sprintf(`okex key %s request %s body %s return %s`,
		key, uri, toBeSign, string(responseBody)))
	return responseBody
}

func placeOrderOKEX(key, secret string, order *model.Order, price, amount string) {
	postData := map[string]interface{}{`instId`: order.Instrument, `tdMode`: `cross`, `side`: order.OrderSide,
		`sz`: amount, `px`: price, `ordType`: order.OrderType}
	responseBody := sendSignRequestOKEX(key, secret, http.MethodPost,
		model.AppConfig.RestUrls[model.OKEX]+"/api/v5/trade/order", postData)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil && orderJson != nil && orderJson.Get(`data`) == nil {
		orders := orderJson.Get(`data`).MustArray()
		for _, item := range orders {
			if item == nil {
				continue
			}
			value := item.(map[string]interface{})
			if value[`sCode`] != nil && value[`sCode`] != `0` {
				order.Status = model.CarryStatusFail
			}
			if value[`sMsg`] != nil {
				order.ErrCode = value[`sMsg`].(string)
			}
			if value[`ordId`] != nil {
				order.OrderId = value[`ordId`].(string)
				return
			}
		}
	}
}

func cancelOrderOkex(key, secret, instrument string, orderId string) (result bool, errCode, msg string) {
	postData := map[string]interface{}{`instId`: instrument, `ordId`: orderId}
	uri := model.AppConfig.RestUrls[model.OKEX] + "/api/v5/trade/cancel-order"
	responseBody := sendSignRequestOKEX(key, secret, http.MethodPost, uri, postData)
	orderJson, err := util.NewJSON(responseBody)
	cancelResult := false
	if err == nil {
		items, _ := orderJson.Get(`data`).Array()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value[`ordId`].(string) == orderId && value[`sCode`].(string) == `0` {
				cancelResult = true
				break
			}
		}
		return cancelResult, ``, ``
	}
	return false, err.Error(), err.Error()
}

func parseOrderOKEX(value map[string]interface{}) (order *model.Order) {
	if value == nil {
		return nil
	}
	order = &model.Order{}
	if value[`avgPx`] != nil {
		order.DealPrice, _ = strconv.ParseFloat(value[`avgPx`].(string), 64)
	}
	if value[`instId`] != nil {
		order.Instrument = value[`instId`].(string)
	}
	if value[`ordId`] != nil {
		order.OrderId = value[`ordId`].(string)
	}
	if value[`px`] != nil {
		order.Price, _ = strconv.ParseFloat(value[`px`].(string), 64)
	}
	if value[`sz`] != nil {
		order.Amount, _ = strconv.ParseFloat(value[`sz`].(string), 64)
	}
	if value[`ordType`] != nil { // market：市价单 limit：限价单 post_only：只做maker单 fok：全部成交或立即取消 ioc：立即成交并取消剩余
		order.OrderType = value[`ordType`].(string)
	}
	if value[`side`] != nil {
		order.OrderSide = value[`side`].(string)
	}
	if value[`accFillSz`] != nil {
		order.DealAmount, _ = strconv.ParseFloat(value[`accFillSz`].(string), 64)
	}
	if value[`avgPx`] != nil {
		order.DealPrice, _ = strconv.ParseFloat(value[`avgPx`].(string), 64)
	}
	if value[`state`] != nil {
		status := value[`state`].(string)
		switch status {
		case `canceled`:
			order.Status = model.CarryStatusFail
		case `live`, `partially_filled`:
			order.Status = model.CarryStatusWorking
		case `filled`:
			order.Status = model.CarryStatusSuccess
		default:
			order.Status = model.CarryStatusFail
		}
	}
	if value[`fee`] != nil { // 订单交易手续费，平台向用户收取的交易手续费，手续费扣除 为负数。如： -0.01
		order.Fee, _ = strconv.ParseFloat(value[`fee`].(string), 64)
	}
	if value[`cTime`] != nil {
		ts, _ := strconv.ParseInt(value[`cTime`].(string), 10, 64)
		order.OrderTime = time.Unix(ts/1000, 0)
	}
	return order
}

func queryOrderOKEX(key, secret, instrument string, orderId string) (order *model.Order) {
	responseBody := sendSignRequestOKEX(key, secret, http.MethodGet,
		model.AppConfig.RestUrls[model.OKEX]+"/api/v5/trade/order",
		map[string]interface{}{"ordId": orderId, "instId": instrument})
	orderJson, err := util.NewJSON(responseBody)
	if err != nil || orderJson == nil || orderJson.Get(`data`) == nil {
		return nil
	}
	orders := orderJson.Get("data").MustArray()
	for _, item := range orders {
		value := item.(map[string]interface{})
		if value[`ordId`].(string) == orderId {
			return parseOrderOKEX(value)
		}
	}
	return nil
}

func getTransferOKEX(key, secret string) (balances []*model.Balance) {
	response := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/asset/deposit-history`, nil)
	responseJson, err := util.NewJSON(response)
	balances = make([]*model.Balance, 0)
	if err == nil && responseJson != nil && responseJson.Get(`data`) != nil {
		transfers := responseJson.Get(`data`).MustArray()
		for _, transfer := range transfers {
			balance := parseBalanceOKEX(transfer.(map[string]interface{}))
			if balance != nil {
				balances = append(balances, balance)
			}
		}
	}
	response = SignedRequestOKSwap(key, secret, http.MethodGet, `/api/v5/asset/withdrawal-history`, nil)
	responseJson, err = util.NewJSON(response)
	if err == nil && responseJson != nil && responseJson.Get(`data`) != nil {
		transfers := responseJson.Get(`data`).MustArray()
		for _, transfer := range transfers {
			balance := parseBalanceOKEX(transfer.(map[string]interface{}))
			if balance != nil {
				balances = append(balances, balance)
			}
		}
	}
	return balances
}

func parseBalanceOKEX(value map[string]interface{}) (balance *model.Balance) {
	if value == nil || value[`ccy`] == nil {
		return nil
	}
	balance = &model.Balance{Market: model.OKEX, Action: 0, Coin: value[`ccy`].(string),
		ID: model.OKEX + `_` + value[`ccy`].(string) + `_` + util.GetNow().String()[0:10]}
	// for transfer
	if value[`amt`] != nil {
		balance.Amount, _ = strconv.ParseFloat(value[`amt`].(string), 64)
	}
	if value[`from`] != nil && value[`to`] != nil {
		balance.Address = fmt.Sprintf(`%s : %s`, value[`from`].(string), value[`to`].(string))
	}
	if value[`ts`] != nil {
		ts, _ := strconv.ParseInt(value[`ts`].(string), 10, 64)
		balance.BalanceTime = time.Unix(ts/1000, 0)
	}
	if value[`txId`] != nil {
		balance.TransactionId = value[`txId`].(string)
	}
	if value[`fee`] != nil {
		balance.Fee, _ = value[`fee`].(string)
	}
	if value[`state`] != nil {
		switch value[`state`].(string) {
		case `-3`, `0`, `1`, `3`, `4`, `5`:
			balance.Status = model.CarryStatusWorking
		case `-2`, `-1`:
			balance.Status = model.CarryStatusFail
		case `2`:
			balance.Status = model.CarryStatusSuccess
		}
	}
	// for balance
	if value[`availEq`] != nil {
		balance.Available, _ = strconv.ParseFloat(value[`availEq`].(string), 64)
	}
	if value[`eq`] != nil {
		balance.Amount, _ = strconv.ParseFloat(value[`eq`].(string), 64)
	}
	if value[`disEq`] != nil {
		balance.UsdValue, _ = strconv.ParseFloat(value[`disEq`].(string), 64)
	}
	if value[`crossLiab`] != nil {
		balance.Borrow, _ = strconv.ParseFloat(value[`crossLiab`].(string), 64)
	}
	return
}

func getBalanceOKEX(key, secret string) (balances []*model.Balance) {
	response := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/account/balance`, nil)
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson == nil || responseJson.Get(`data`) == nil {
		util.SocketInfo(`fail to get okex balance`)
		time.Sleep(time.Second * 2)
		return getBalanceOKEX(key, secret)
	}
	balances = make([]*model.Balance, 0)
	balanceArray := responseJson.Get(`data`).MustArray()
	balances = make([]*model.Balance, 0)
	for _, item := range balanceArray {
		balance := parseBalanceOKEX(item.(map[string]interface{}))
		balances = append(balances, balance)
	}
	return balances
}

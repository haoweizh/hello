package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"hello/model"
	"hello/util"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const OKEXTag = `f924a8c6cc6fBCDE` // okx经纪商ID
const OKSeparator = `Sep`
const restOKEX = `https://www.okx.com`
const wsOKEX = `wss://ws.okx.com:8443/ws/v5/public`
const wsPrivateOKEX = `wss://ws.okx.com:8443/ws/v5/private`
const wsStepOKEX = 90
const ParamArrayOkex = `OK_ARRAY`
const chanelOKEX = `bbo-tbt` //`books5`
var lastSameTime = make(map[string]int64)
var lastCarryTime = int64(0)

// var wrongs = make(map[string]bool)
// var wrongLock sync.Mutex

func maintainConnsOKEX(accounts []*model.Account) {
	//subscribes := GetWSSubscribes(model.OKEX, []string{model.SubscribeDepth})
	//go func() {
	//	for {
	//		time.Sleep(time.Minute * 5)
	//		reSubscribe(subscribes)
	//	}
	//}()
	for {
		if !CheckSetProcessing(model.FunctionConnMaintain, model.OKEX, ``, true) {
			time.Sleep(2 * time.Second)
			continue
		}
		connTick, _ := model.AppEnvironment.ConnTick.Load(model.OKEX)
		if connTick != nil {
			if err := SendToConnections(model.OKEX, connTick.(map[*model.WSConn]bool), []byte(`ping`)); err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("tick conn maintain error %s %s", model.OKEX, err.Error()))
			}
		}
		for _, account := range accounts {
			if account == nil {
				continue
			}
			success := true
			value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, model.OKEX, account.Key)
			if value != nil {
				if err := SendToConnection(model.OKEX, value.(*model.WSConn), []byte(`ping`)); err != nil {
					util.Log(util.LogLevelError, "-test ok ws-okex server ping client error "+err.Error())
					success = false
					value.(*model.WSConn).Close()
				}
			} else {
				success = false
			}
			if !success {
				util.Log(util.LogLevelError, fmt.Sprintf(`-test ok ws- no private connection %s`, account.Key))
				util.DelSyncMap(&model.AppEnvironment.ConnOrder, model.OKEX, account.Key)
				WsOrderServeOKEX(account)
			}
		}
		CheckSetProcessing(model.FunctionConnMaintain, model.OKEX, ``, false)
		time.Sleep(time.Second * 20)
	}
}

//func getWrongs() []string {
//	defer wrongLock.Unlock()
//	wrongLock.Lock()
//	array := make([]string, 0)
//	for s := range wrongs {
//		array = append(array, s)
//	}
//	return array
//}

//func setWrong(symbol string, success bool) {
//	defer wrongLock.Unlock()
//	wrongLock.Lock()
//	if !success {
//		wrongs[symbol] = true
//	} else {
//		delete(wrongs, symbol)
//	}
//}

// books5首次推5档快照数据，以后定量推送，每100毫秒当5档快照数据有变化推送一次5档数据
// bbo-tbt 首次推1档快照数据，以后定量推送，每10毫秒当1档快照数据有变化推送一次1档数据
var subscribeHandlerOKEX = func(market string, connection *model.WSConn, subscribes []interface{}) error {
	var err error = nil
	subscribeMap := make(map[string]interface{})
	subscribeMap["op"] = "subscribe"
	subArray := make([]map[string]string, 0)
	for _, subscribe := range subscribes {
		subArray = append(subArray, map[string]string{`channel`: chanelOKEX, `instId`: subscribe.(string)})
		subArray = append(subArray, map[string]string{`channel`: `funding-rate`, `instId`: subscribe.(string)})
	}
	subscribeMap[`args`] = subArray
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.OKEX, connection, subscribeMessage); err != nil {
		util.Log(util.LogLevelError, "okex can not subscribe "+err.Error())
		return err
	}
	return err
}

var wsHandlerOKEX = func(market string, conn *model.WSConn, event []byte) {
	responseJson, err := util.NewJSON(event)
	if err != nil || responseJson == nil || responseJson.Get(`data`) == nil ||
		len(responseJson.Get(`data`).MustArray()) == 0 ||
		responseJson.GetPath(`arg`, `instId`) == nil {
		//util.Notice(`get wrong okex ws msg%s`, string(event))
		return
	}
	dialectSymbol := responseJson.GetPath(`arg`, `instId`).MustString()
	marketType := model.MarketTypeSpot
	if strings.Contains(dialectSymbol, `-USDT-SWAP`) {
		marketType = model.MarketTypePerp
	}
	getSymbol, _, symbol := model.GetFromDialect(model.OKEX, marketType, dialectSymbol)
	if !getSymbol {
		return
	}
	action := responseJson.Get(`action`).MustString()
	data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
	success := false
	var bidAsk *model.BidAsk
	if action == `update` {
		_, bidAsk = model.AppEnvironment.GetBidAsk(model.OKEX, symbol)
		if bidAsk != nil {
			success, bidAsk = handleBooksUpdate(symbol, data, bidAsk)
		}
	} else if action == `snapshot` || responseJson.GetPath(`arg`, `channel`).MustString() == chanelOKEX {
		bidAsk = handleBooksOKEX(symbol, data)
		success = true
	} else if responseJson.GetPath(`arg`, `channel`).MustString() == `funding-rate` {
		rate, _ := strconv.ParseFloat(data[`fundingRate`].(string), 64)
		rateNext, _ := strconv.ParseFloat(data[`nextFundingRate`].(string), 64)
		fundingTime, _ := strconv.ParseInt(data[`fundingTime`].(string), 10, 64)
		fundingTimeNext, _ := strconv.ParseInt(data[`nextFundingTime`].(string), 10, 64)
		marketInfo := model.GetMarketInfo(model.OKEX, symbol)
		if marketInfo != nil {
			marketInfo.FundingRateInterval = int(fundingTimeNext - fundingTime)
		}
		ts, _ := strconv.ParseInt(data[`ts`].(string), 10, 64)
		SetFundingRate(model.OKEX, symbol, &model.FundingRate{Rate: rate, RateNext: rateNext, ExpireTime: fundingTime / 1000, UpdateTime: time.UnixMilli(ts)})
		return
	}
	if bidAsk == nil || bidAsk.Bids == nil || bidAsk.Asks == nil || bidAsk.Bids.Len() == 0 || bidAsk.Asks.Len() == 0 || !success {
		return
	}
	//将最佳买一卖一的数量转换为币种的真实数量
	_, bidAsk.Bids[0].Amount = model.ParseRealAmount(model.OKEX, symbol, bidAsk.Bids[0].Amount)
	_, bidAsk.Asks[0].Amount = model.ParseRealAmount(model.OKEX, symbol, bidAsk.Asks[0].Amount)
	if model.AppEnvironment.SetBidAsk(model.OKEX, symbol, bidAsk) {
		funcHandlers := GetFunctions(model.OKEX, symbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), model.OKEX, symbol)
				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
					go value.(model.CarryHandler)(setting, bidAsk)
				}
				return true
			})
		}
	}
}

var wsAccountHandlerOKEX = func(market, key string, event []byte) {
	responseJson, err := util.NewJSON(event)
	if err != nil || responseJson == nil {
		return
	}
	if responseJson.Get(`event`).MustString() == `login` && responseJson.Get(`code`).MustString() == `0` {
		value, success := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, market, key)
		if !success || value == nil {
			return
		}
		err = value.(*model.WSConn).WriteJson(map[string]interface{}{"op": "subscribe", "args": []interface{}{map[string]string{"channel": "orders", "instType": "SPOT"}}})
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to sub %s spot order update`, market))
			util.DelSyncMap(&model.AppEnvironment.ConnOrder, market, key)
			return
		}
		err = value.(*model.WSConn).WriteJson(map[string]interface{}{"op": "subscribe", "args": []interface{}{map[string]string{"channel": "orders", "instType": "SWAP"}}})
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to sub %s swap order update`, market))
			util.DelSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, key)
			return
		}
	}
	if responseJson.GetPath(`arg`, `channel`).MustString() == `orders` {
		data := responseJson.Get(`data`).MustArray()
		for _, value := range data {
			order := parseOrderOKEX(value.(map[string]interface{}))
			UpdateOrderDeal(market, order.OrderId, order.Status, ``, order.DealAmount)
		}
	}
	if responseJson.Get(`op`).MustString() == `order` {
		data := responseJson.Get(`data`).MustArray()
		for i, item := range data {
			value := item.(map[string]interface{})
			wsResp := model.WSResp{RequestId: responseJson.Get(`id`).MustString()}
			wsResp.Msg = value[`sCode`].(string) + value[`sMsg`].(string)
			wsResp.OrderId = value[`ordId`].(string)
			if responseJson.Get(`code`).MustString() == `0` {
				wsResp.Success = true
			} else {
				wsResp.Success = false
			}
			if i > 0 {
				util.Log(util.LogLevelInfo, fmt.Sprintf("ok pair request %d %#v", i, wsResp))
			}
			model.AppEnvironment.WSRespChan <- wsResp
		}
	} else if responseJson.Get(`op`).MustString() == `batch-orders` {
		wsRespBuy := model.WSResp{RequestId: responseJson.Get(`id`).MustString() + model.OrderSideBuy}
		wsRespSell := model.WSResp{RequestId: responseJson.Get(`id`).MustString() + model.OrderSideSell}
		for _, value := range responseJson.Get(`data`).MustArray() {
			data := value.(map[string]interface{})
			if strings.Contains(data[`clOrdId`].(string), model.OrderSideBuy) {
				wsRespBuy.OrderId = data[`ordId`].(string)
				if len(wsRespBuy.OrderId) > 0 {
					wsRespBuy.Success = true
				}
			}
			if strings.Contains(data[`clOrdId`].(string), model.OrderSideSell) {
				wsRespSell.OrderId = data[`ordId`].(string)
				if len(wsRespSell.OrderId) > 0 {
					wsRespSell.Success = true
				}
			}
		}
		model.AppEnvironment.WSRespChan <- wsRespBuy
		model.AppEnvironment.WSRespChan <- wsRespSell
	}
}

func wsLogInOKEX(account *model.Account, conn *model.WSConn) (success bool) {
	loginMap := make(map[string]interface{})
	loginMap[`op`] = `login`
	timestamp := time.Now().Unix()
	toBeSign := fmt.Sprintf(`%dGET/users/self/verify`, timestamp)
	hash := hmac.New(sha256.New, []byte(account.Secret))
	hash.Write([]byte(toBeSign))
	sign := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	loginArray := []map[string]interface{}{{
		`apiKey`: account.Key, `passphrase`: model.AppConfig.OKPhase, `timestamp`: timestamp, `sign`: sign}}
	loginMap[`args`] = loginArray
	msg := util.JsonEncodeToByte(loginMap)
	if err := SendToConnection(model.OKEX, conn, msg); err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(
			`fail to login okex ws: %s return %s`, account.Key, err.Error()))
	} else {
		success = true
		util.Log(util.LogLevelInfo, fmt.Sprintf("log in conn %s %s", model.OKEX, string(msg)))
	}
	return
}

func WsOrderServeOKEX(account *model.Account) {
	if account == nil {
		return
	}
	conn, err := model.WsPrivateClient(model.OKEX, account.Key, wsPrivateOKEX, wsAccountHandlerOKEX)
	if err != nil {
		util.Log(util.LogLevelError, "can not create web socket "+err.Error())
	} else if conn != nil {
		if wsLogInOKEX(account, conn) {
			util.StoreSyncMap(&model.AppEnvironment.ConnOrder, conn, model.OKEX, account.Key)
		}
	}
}

func handleBooksUpdate(symbol string, data map[string]interface{}, bidAsk *model.BidAsk) (
	success bool, bidAskUpdate *model.BidAsk) {
	bidAskUpdate = handleBooksOKEX(symbol, data)
	if data[`ts`] != nil {
		ts, _ := strconv.ParseInt(data[`ts`].(string), 10, 64)
		bidAskUpdate.Ts = int(ts)
	}
	newAsks := make([]model.Tick, 0)
	newBids := make([]model.Tick, 0)
	i := 0
	j := 0
	for {
		if j >= len(bidAskUpdate.Asks) {
			if i < len(bidAsk.Asks) {
				newAsks = append(newAsks, bidAsk.Asks[i])
				i++
			} else {
				break
			}
		} else if i >= len(bidAsk.Asks) {
			if j < len(bidAskUpdate.Asks) {
				if bidAskUpdate.Asks[j].Amount > 0 {
					newAsks = append(newAsks, bidAskUpdate.Asks[j])
				}
				j++
			} else {
				break
			}
		} else {
			if bidAsk.Asks[i].Price < bidAskUpdate.Asks[j].Price {
				newAsks = append(newAsks, bidAsk.Asks[i])
				i++
			} else if bidAsk.Asks[i].Price == bidAskUpdate.Asks[j].Price {
				if bidAskUpdate.Asks[j].Amount > 0 {
					newAsks = append(newAsks, bidAskUpdate.Asks[j])
				}
				i++
				j++
			} else if bidAsk.Asks[i].Price > bidAskUpdate.Asks[j].Price {
				if bidAskUpdate.Asks[j].Amount > 0 {
					newAsks = append(newAsks, bidAskUpdate.Asks[j])
				}
				j++
			}
		}
	}
	i = 0
	j = 0
	for {
		if j >= len(bidAskUpdate.Bids) {
			if i < len(bidAsk.Bids) {
				newBids = append(newBids, bidAsk.Bids[i])
				i++
			} else {
				break
			}
		} else if i >= len(bidAsk.Bids) {
			if j < len(bidAskUpdate.Bids) {
				if bidAskUpdate.Bids[j].Amount > 0 {
					newBids = append(newBids, bidAskUpdate.Bids[j])
				}
				j++
			} else {
				break
			}
		} else {
			if bidAsk.Bids[i].Price > bidAskUpdate.Bids[j].Price {
				newBids = append(newBids, bidAsk.Bids[i])
				i++
			} else if bidAsk.Bids[i].Price == bidAskUpdate.Bids[j].Price {
				if bidAskUpdate.Bids[j].Amount > 0 {
					newBids = append(newBids, bidAskUpdate.Bids[j])
				}
				i++
				j++
			} else if bidAsk.Bids[i].Price < bidAskUpdate.Bids[j].Price {
				if bidAskUpdate.Bids[j].Amount > 0 {
					newBids = append(newBids, bidAskUpdate.Bids[j])
				}
				j++
			}
		}
	}
	// 由于处理压力大，暂时放弃计算checksum
	if data[`checksum`] != nil {
		bidAskUpdate.Bids = newBids
		bidAskUpdate.Asks = newAsks
		value, ok := okexCrossing.Load(symbol)
		success = true
		if ok && value.(bool) {
			checkStr := ``
			for index := 0; index < 25; index++ {
				if index < len(newBids) {
					checkStr += fmt.Sprintf(`%s:%s:`, newBids[index].PriceStr, newBids[index].AmountStr)
				}
				if index < len(newAsks) {
					checkStr += fmt.Sprintf(`%s:%s:`, newAsks[index].PriceStr, newAsks[index].AmountStr)
				}
			}
			// 以下语句并非无用，如果不加，会造成checksum计算错误
			checkStr = checkStr[0 : len(checkStr)-1]
			crcValue := int64(int32(crc32.ChecksumIEEE([]byte(checkStr))))
			compare, _ := data[`checksum`].(json.Number).Int64()
			if compare != crcValue {
				success = false
			}
			//setWrong(symbol, true)
			if !success && time.Now().Minute() == 0 && time.Now().Second() == 0 {
				util.Log(util.LogLevelError, fmt.Sprintf("%#v ts %d checksum %s %s \n %#v",
					success, bidAskUpdate.Ts, symbol, checkStr, data))
			}
		}
	}
	return success, bidAskUpdate
}

func handleBooksOKEX(symbol string, data map[string]interface{}) (bidAsk *model.BidAsk) {
	bidAsk = &model.BidAsk{TsReceived: int(util.GetNowUnixMillion())}
	if data[`ts`] != nil {
		ts, _ := strconv.ParseInt(data[`ts`].(string), 10, 64)
		bidAsk.Ts = int(ts)
	}
	asks := data[`asks`].([]interface{})
	bidAsk.Asks = make([]model.Tick, len(asks))
	bids := data[`bids`].([]interface{})
	bidAsk.Bids = make([]model.Tick, len(bids))
	for i, ask := range asks {
		value := ask.([]interface{})
		if len(value) >= 2 {
			priceStr := value[0].(string)
			amountStr := value[1].(string)
			price, _ := strconv.ParseFloat(priceStr, 64)
			amount, _ := strconv.ParseFloat(amountStr, 64)
			bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, PriceStr: priceStr,
				AmountStr: amountStr, Market: model.OKEX, Symbol: symbol}
		}
	}
	for i, bid := range bids {
		value := bid.([]interface{})
		if len(value) >= 2 {
			priceStr := value[0].(string)
			amountStr := value[1].(string)
			price, _ := strconv.ParseFloat(priceStr, 64)
			amount, _ := strconv.ParseFloat(amountStr, 64)
			bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, PriceStr: priceStr,
				AmountStr: amountStr, Market: model.OKEX, Symbol: symbol}
		}
	}
	//if data[`checksum`] != nil {
	//	checkStr := ``
	//	for index := 0; index < 25; index++ {
	//		if index < len(bidAsk.Bids) {
	//			checkStr += fmt.Sprintf(`%s:%s:`, bidAsk.Bids[index].PriceStr, bidAsk.Bids[index].AmountStr)
	//		}
	//		if index < len(bidAsk.Asks) {
	//			checkStr += fmt.Sprintf(`%s:%s:`, bidAsk.Asks[index].PriceStr, bidAsk.Asks[index].AmountStr)
	//		}
	//	}
	//	if len(checkStr) > 0 {
	//		checkStr = checkStr[0 : len(checkStr)-1]
	//	}
	//	crcValue := int64(int32(crc32.ChecksumIEEE([]byte(checkStr))))
	//	compare, _ := data[`checksum`].(json.Number).Int64()
	//	if compare == crcValue {
	//		util.Notice(fmt.Sprintf(`right checksum snapshot %s %d`, symbol, bidAsk.Bids.Len()))
	//	} else {
	//		util.Notice(fmt.Sprintf("%#v +++++++ checksum %s\n %s\n %#v",
	//			false, symbol, checkStr, data))
	//	}
	//}
	return
}

func sendSignRequestOKEX(key, secret, method, path string, param, body map[string]interface{}) ([]byte, error) {
	u, _ := url.ParseRequestURI(restOKEX)
	u.Path += path
	q := u.Query()
	for k, v := range param {
		value := fmt.Sprintf(`%v`, v)
		if k == `instId` {
			_, _, _, value = model.GetFromStandard(model.OKEX, value)
		}
		q.Set(k, value)
	}
	u.RawQuery = q.Encode()
	current := time.Now().In(time.UTC).Format(time.RFC3339)
	// , `x-simulated-trading`: `1`
	headers := map[string]string{`OK-ACCESS-KEY`: key, `OK-ACCESS-PASSPHRASE`: model.AppConfig.OKPhase,
		"OK-ACCESS-TIMESTAMP": current, "Content-Type": "application/json"}
	postContent := ``
	if body[ParamArrayOkex] == nil {
		if body[`instId`] != nil && len(body[`instId`].(string)) > 0 {
			_, _, _, dialectSymbol := model.GetFromStandard(model.OKEX, body[`instId`].(string))
			body[`instId`] = dialectSymbol
		}
		if method == http.MethodPost {
			postContent = string(util.JsonEncodeToByte(body))
		}
	} else {
		for _, value := range body[ParamArrayOkex].([]map[string]interface{}) {
			if value[`instId`] != nil && len(value[`instId`].(string)) > 0 {
				_, _, _, dialectSymbol := model.GetFromStandard(model.OKEX, value[`instId`].(string))
				value[`instId`] = dialectSymbol
			}
		}
		if method == http.MethodPost {
			postContent = string(util.JsonEncodeToByte(body[ParamArrayOkex]))
		}
	}
	if len(u.RawQuery) > 0 {
		path = fmt.Sprintf(`%s?%s`, u.Path, u.RawQuery)
	} else {
		path = u.Path
	}
	toBeSign := fmt.Sprintf(`%s%s%s%s`, current, method, path, postContent)
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(toBeSign))
	sign := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	headers[`OK-ACCESS-SIGN`] = sign
	responseBody, err := util.HttpRequest(method, u.String(), postContent, headers, 60)
	//logMsg := fmt.Sprintf(`okex key %s request %s body %s return %s`,
	//	key, u.String(), toBeSign, string(responseBody))
	//util.Log(util.LogLevelDebug, logMsg)
	//time.Sleep(time.Millisecond * 100)
	return responseBody, err
}

func getWSOrderArgOKEX(account *model.Account, requestId, symbol, orderSide, orderType string, price, amount float64) (args map[string]interface{}) {
	priceFormat, decimal := model.FormatPrice(model.OKEX, symbol, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(priceFormat, 'f', decimal, 64))
	formattedAmount := model.GetAmountInMarket(model.OKEX, symbol, amount, price, false)
	amountStrPerp := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	if orderType == model.OrderTypeMarket {
		usdAmount, _ := strconv.ParseFloat(amountStrPerp, 64)
		amountStrPerp = util.CutTailZero(fmt.Sprintf(`%f`, usdAmount*price))
	}
	_, _, _, dialectSymbol := model.GetFromStandard(model.OKEX, symbol)
	args = map[string]interface{}{`instId`: dialectSymbol, `tdMode`: `cross`, `side`: orderSide,
		`sz`: amountStrPerp, `ordType`: orderType, `px`: priceStr, `tag`: OKEXTag}
	if orderType == model.OrderTypeStop || orderType == model.OrderTypeTrailStop {
		args[`algoClOrdId`] = fmt.Sprintf(`%d%s%d%s`, account.Index, OKSeparator, time.Now().Nanosecond(), orderSide)
	} else {
		args[`clOrdId`] = fmt.Sprintf(`%d%s%d%s`, account.Index, OKSeparator, time.Now().Nanosecond(), orderSide)
	}
	args[`clOrdId`] = requestId + orderSide
	return args
}

func PlacePairOKEX(account *model.Account, requestId, symbolBuy, symbolSell, orderType string, priceBuy, priceSell,
	amountBuy, amountSell float64) (success bool, errMsg string) {
	if amountBuy == 0 || amountSell == 0 || priceBuy == 0 || priceSell == 0 {
		errMsg = fmt.Sprintf(`error: wrong PlacePairOKEX buy %f at %f sell %f at %f`, amountBuy, priceBuy, amountSell, priceSell)
		util.Log(util.LogLevelError, errMsg)
		return false, errMsg
	}
	now := time.Now().UnixNano()
	if time.Duration(now-lastCarryTime)/time.Millisecond < 50 {
		errMsg = symbolBuy + ` ignore carry for last time < 50ms`
		util.Log(util.LogLevelInfo, errMsg)
		return false, errMsg
	} else if time.Duration(now-lastSameTime[symbolBuy])/time.Millisecond < 100 {
		errMsg = symbolBuy + ` ignore carry for same pair last time < 200ms`
		util.Log(util.LogLevelInfo, errMsg)
		return false, errMsg
	}
	lastSameTime[symbolBuy] = now
	lastSameTime[symbolSell] = now
	lastCarryTime = now
	subscribeArgs := []map[string]interface{}{
		getWSOrderArgOKEX(account, requestId, symbolBuy, model.OrderSideBuy, orderType, priceBuy, amountBuy),
		getWSOrderArgOKEX(account, requestId, symbolSell, model.OrderSideSell, orderType, priceSell, amountSell)}
	subscribeMap := make(map[string]interface{})
	subscribeMap[`args`] = subscribeArgs
	subscribeMap[`id`] = requestId
	subscribeMap["op"] = "batch-orders"
	msg := util.JsonEncodeToByte(subscribeMap)
	value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, model.OKEX, account.Key)
	if value == nil {
		errMsg = fmt.Sprintf(`fail to get connection %s`, account.Key)
		util.Log(util.LogLevelError, errMsg)
		return false, errMsg
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`place pair %s`, msg))
	if model.AppConfig.Env != `test` {
		err := SendToConnection(model.OKEX, value.(*model.WSConn), msg)
		if err != nil {
			errMsg = fmt.Sprintf(`-test ok ws-fail to send order ws %s return %s`, account.Key, err.Error())
			util.Log(util.LogLevelError, errMsg)
			return false, errMsg
		} else {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`-test ok ws- success send ws order %s %s`, account.Key, msg))
		}
	}
	return true, ``
}

// orderType: move_order_stop 只支持立即触发
// amount、price
// 不能使用 fmt %#v 因为有e+5 的情况；
// 不能使用 fmt %f 因为有000后缀；
// 不能使用 strconv.FormatFloat 因为有 2.00000001问题
// priceStr := strconv.FormatFloat(order.Price, 'f', -1, 64)
// triggerPriceStr := strconv.FormatFloat(order.TriggerPrice, 'f', -1, 64)
func placeOrderOKEX(account *model.Account, isWs bool, order *model.Order) {
	price, decimal := model.FormatPrice(model.OKEX, order.Symbol, order.Price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	priceTrigger, decimalTrigger := model.FormatPrice(model.OKEX, order.Symbol, order.TriggerPrice)
	triggerPriceStr := util.CutTailZero(strconv.FormatFloat(priceTrigger, 'f', decimalTrigger, 64))
	formattedAmount := model.GetAmountInMarket(model.OKEX, order.Symbol, order.Amount, price, false)
	amount := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	order.Price = price
	order.TriggerPrice = priceTrigger
	orderAmountReal, _ := strconv.ParseFloat(amount, 64)
	_, order.Amount = model.ParseRealAmount(model.OKEX, order.Symbol, orderAmountReal)
	if (price == 0 && order.OrderType != model.OrderTypeTrailStop) || formattedAmount == 0 {
		order.Status = model.CarryStatusFail
		return
	}
	postData := map[string]interface{}{`instId`: order.Symbol, `tdMode`: `cross`, `side`: order.OrderSide,
		`sz`: amount, `ordType`: order.OrderType, `tag`: OKEXTag, `clOrdId`: order.ClientOrdId}
	path := "/api/v5/trade/order"
	if order.OrderType == model.OrderTypeStop {
		postData[`ordType`] = `conditional`
		postData[`slOrdPx`] = priceStr
		postData[`slTriggerPx`] = triggerPriceStr
		postData[`algoClOrdId`] = order.ClientOrdId
		path = `/api/v5/trade/order-algo`
	} else if order.OrderType == model.OrderTypeTrailStop {
		postData[`ordType`] = `move_order_stop`
		//if price > 0 {
		//	postData[`activePx`] = priceStr
		//}
		postData[`callbackRatio`] = triggerPriceStr
		postData[`algoClOrdId`] = order.ClientOrdId
		path = `/api/v5/trade/order-algo`
	} else {
		postData[`clOrdId`] = order.ClientOrdId
		postData[`px`] = priceStr
		_, marketType, _, _ := model.GetFromStandard(order.Market, order.Symbol)
		if order.OrderType == model.OrderTypeMarket && marketType == model.MarketTypeSpot {
			postData[`tgtCcy`] = `base_ccy`
		}
	}
	if isWs {
		// 通过ws的symbol需要处理成方言，通过rest的无需处理，已统一在发送的函数中处理
		_, _, _, dialectSymbol := model.GetFromStandard(model.OKEX, order.Symbol)
		postData[`instId`] = dialectSymbol
		subscribeMap := make(map[string]interface{})
		subscribeMap[`id`] = order.ClientOrdId
		subscribeMap["op"] = "order"
		subscribeMap[`args`] = []map[string]interface{}{postData}
		wsOrderMsg := util.JsonEncodeToByte(subscribeMap)
		util.Log(util.LogLevelInfo, `ws order `+string(wsOrderMsg))
		order.Status = model.CarryStatusWorking
		value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, model.OKEX, account.Key)
		if value == nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to get private connection when place okex order %s`, account.Key))
			order.Status = model.CarryStatusFail
		} else {
			if err := SendToConnection(model.OKEX, value.(*model.WSConn), wsOrderMsg); err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`fail to send ws place okex order %s %s return %s`, account.Key, order.Symbol, err.Error()))
				order.Status = model.CarryStatusFail
			} else {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`-test ok ws- success place okex ws order %s %s`, account.Key, order.Symbol))
			}
		}
	} else {
		responseBody, httpErr := sendSignRequestOKEX(account.Key, account.Secret, http.MethodPost, path, nil, postData)
		util.Log(util.LogLevelError, fmt.Sprintf(`place okex %s return %s`, path, string(responseBody)))
		if httpErr != nil {
			order.ErrCode = httpErr.Error()
			return
		}
		orderJson, err := util.NewJSON(responseBody)
		if err == nil && orderJson != nil && orderJson.Get(`data`) != nil {
			orders := orderJson.Get(`data`).MustArray()
			for _, item := range orders {
				if item == nil {
					continue
				}
				value := item.(map[string]interface{})
				if value[`sCode`] != nil && value[`sCode`] != `0` {
					order.Status = model.CarryStatusFail
					order.ErrCode = value[`sCode`].(string)
				}
				if value[`ordId`] != nil && len(value[`ordId`].(string)) > 0 && value[`ordId`].(string) != `0` {
					order.OrderId = value[`ordId`].(string)
					return
				} else if value[`algoId`] != nil {
					order.OrderId = value[`algoId`].(string)
					return
				}
			}
		}
	}
}

// consider spot future size calc
// 官方文档中对ctVal的描述有误，实际上ctVal在永续合约里是单张合约的交易币种的数量而非官方文档中描述的计价币种数量
func getMarketsOKEX(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	instTypes := []string{`SPOT`, `SWAP`}
	for _, instType := range instTypes {
		param := map[string]interface{}{`instType`: instType}
		basicBody, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/public/instruments`, param, nil)
		basicJson, err := util.NewJSON(basicBody)
		marketBody, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/market/tickers`, param, nil)
		marketJson, errMarket := util.NewJSON(marketBody)
		if err != nil || errMarket != nil || basicJson == nil || marketJson == nil ||
			basicJson.Get(`code`).MustString() != `0` || marketJson.Get(`code`).MustString() != `0` {
			time.Sleep(time.Minute * 5)
			return getMarketsOKEX(key, secret)
		} else {
			for _, info := range basicJson.Get(`data`).MustArray() {
				value := info.(map[string]interface{})
				if value[`instId`] != nil {
					marketInfo := &model.MarketInfo{Market: model.OKEX}
					marketType := model.MarketTypeSpot
					if strings.Contains(value[`instId`].(string), `-USDT-SWAP`) {
						marketType = model.MarketTypePerp
					}
					success, _, symbol := model.GetFromDialect(model.OKEX, marketType, value[`instId`].(string))
					if !success {
						continue
					}
					if value[`state`] != nil || value[`state`].(string) != `live` {
						continue
					}
					marketInfo.Symbol = symbol
					if value[`lotSz`] != nil {
						marketInfo.SizeIncrement, _ = strconv.ParseFloat(value[`lotSz`].(string), 64)
					}
					if value[`minSz`] != nil {
						marketInfo.SizeMin, _ = strconv.ParseFloat(value[`minSz`].(string), 64)
					}
					if value[`tickSz`] != nil {
						tickSz := strings.Trim(value[`tickSz`].(string), ` `)
						marketInfo.PriceIncrement, _ = strconv.ParseFloat(tickSz, 64)
						if marketInfo.PriceIncrement > 1 {
							marketInfo.PriceDecimal = 0
						} else if strings.Contains(tickSz, `.`) {
							tickSz = strings.Trim(tickSz, `0`)
							index := strings.Index(tickSz, `.`)
							marketInfo.PriceDecimal = len(tickSz[index+1:])
						}
					}
					if value[`ctVal`] != nil {
						marketInfo.CTValue, _ = strconv.ParseFloat(value[`ctVal`].(string), 64)
					}
					if value[`ctValCcy`] != nil {
						marketInfo.CTCurrency = value[`ctValCcy`].(string)
					}
					marketInfo.SizeMaxMarket, _ = strconv.ParseFloat(value[`maxMktSz`].(string), 64)
					if marketInfo.CTValue > 0 {
						marketInfo.SizeMaxMarket *= marketInfo.CTValue
						marketInfo.SizeMin *= marketInfo.CTValue
					}
					marketInfos[marketInfo.Symbol] = marketInfo
				}
			}
			for _, info := range marketJson.Get(`data`).MustArray() {
				value := info.(map[string]interface{})
				if value[`instId`] != nil {
					marketType := model.MarketTypeSpot
					if strings.Contains(value[`instId`].(string), `-USDT-SWAP`) {
						marketType = model.MarketTypePerp
					}
					success, _, symbol := model.GetFromDialect(model.OKEX, marketType, value[`instId`].(string))
					if !success {
						continue
					}
					if marketInfos[symbol] == nil {
						continue
					}
					if value[`volCcy24h`] != nil && value[`last`] != nil {
						if marketType == model.MarketTypeSpot {
							marketInfos[symbol].TradeAmount, _ = strconv.ParseFloat(value[`volCcy24h`].(string), 64)
						} else {
							vol, _ := strconv.ParseFloat(value[`volCcy24h`].(string), 64)
							lastPriceOKx, _ := strconv.ParseFloat(value[`last`].(string), 64)
							marketInfos[symbol].TradeAmount = vol * lastPriceOKx
						}
					}
				}
			}
		}
	}
	return
}

func cancelAllOkex(key, secret string) {
	orders := make([]*model.Order, 0)
	ordersConditional := queryOpenOrdersOKEX(key, secret, ``, true)
	ordersNormal := queryOpenOrdersOKEX(key, secret, ``, false)
	for _, order := range ordersConditional {
		orders = append(orders, order)
	}
	for _, order := range ordersNormal {
		orders = append(orders, order)
	}
	for _, order := range orders {
		result, errCode, msg := cancelOrderOkex(key, secret, order.Symbol, order.OrderId, order.OrderType)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`cancelAll orders success okex %s id %s type %s return %#v code %s %s`,
			order.Symbol, order.OrderId, order.OrderType, result, errCode, msg))
	}
}

// cancelOrdersOKEX 策略订单每次最多10个，非策略订单每次最多20个
func cancelOrdersOKEX(key, secret, symbol string) (result bool, code, msg string) {
	orders := queryOpenOrdersOKEX(key, secret, symbol, false)
	if len(orders) <= 0 {
		return true, ``, ``
	}
	normalOrders := make([]map[string]interface{}, 0)
	algoOrders := make([]map[string]interface{}, 0)
	advOrders := make([]map[string]interface{}, 0)
	for _, order := range orders {
		if order == nil {
			continue
		}
		if (order.OrderType == model.OrderTypeLimit || order.OrderType == model.OrderTypeMarket) && len(normalOrders) <= 20 {
			normalOrders = append(normalOrders, map[string]interface{}{`instId`: symbol, `ordId`: order.OrderId})
		} else if order.OrderType == model.OrderTypeStop && len(algoOrders) <= 10 {
			algoOrders = append(algoOrders, map[string]interface{}{`instId`: symbol, `algoId`: order.OrderId})
		} else if order.OrderType == model.OrderTypeTrailStop && len(advOrders) <= 10 {
			advOrders = append(advOrders, map[string]interface{}{`instId`: symbol, `algoId`: order.OrderId})
		}
	}
	if len(normalOrders) > 20 || len(algoOrders) > 10 || len(advOrders) > 10 {
		util.Log(util.LogLevelError, fmt.Sprintf(`fatal error: too many normal order to be canceled normal: %d algo: %d adv:%d`,
			len(normalOrders), len(algoOrders), len(advOrders)))
	}
	if len(normalOrders) > 0 {
		responseBody, _ := sendSignRequestOKEX(key, secret, http.MethodPost, "/api/v5/trade/cancel-batch-orders",
			nil, map[string]interface{}{ParamArrayOkex: normalOrders})
		resultJson, err := util.NewJSON(responseBody)
		if err == nil && resultJson != nil {
			result = true
			code = resultJson.Get(`code`).MustString()
			msg = resultJson.Get(`msg`).MustString()
		}
	}
	if len(algoOrders) > 0 {
		responseBody, _ := sendSignRequestOKEX(key, secret, http.MethodPost, `/api/v5/trade/cancel-algos`,
			nil, map[string]interface{}{ParamArrayOkex: algoOrders})
		resultJson, err := util.NewJSON(responseBody)
		if err == nil && resultJson != nil {
			result = true
			code += resultJson.Get(`code`).MustString()
			msg += resultJson.Get(`msg`).MustString()
		}
	}
	if len(advOrders) > 0 {
		responseBody, _ := sendSignRequestOKEX(key, secret, http.MethodPost, `/api/v5/trade/cancel-advance-algos`,
			nil, map[string]interface{}{ParamArrayOkex: advOrders})
		resultJson, err := util.NewJSON(responseBody)
		if err == nil && resultJson != nil {
			result = true
			code += resultJson.Get(`code`).MustString()
			msg += resultJson.Get(`msg`).MustString()
		}
	}
	return result, code, msg
}

func cancelOrderOkex(key, secret, symbol string, orderId, orderType string) (result bool, errCode, msg string) {
	postData := map[string]interface{}{`instId`: symbol}
	var responseBody []byte
	if orderType == model.OrderTypeStop {
		postData[`algoId`] = orderId
		data := []map[string]interface{}{postData}
		postArray := map[string]interface{}{ParamArrayOkex: data}
		responseBody, _ = sendSignRequestOKEX(key, secret, http.MethodPost, `/api/v5/trade/cancel-algos`, nil, postArray)
	} else if orderType == model.OrderTypeTrailStop {
		postData[`algoId`] = orderId
		data := []map[string]interface{}{postData}
		postArray := map[string]interface{}{ParamArrayOkex: data}
		responseBody, _ = sendSignRequestOKEX(key, secret, http.MethodPost, `/api/v5/trade/cancel-advance-algos`, nil, postArray)
	} else {
		postData[`ordId`] = orderId
		responseBody, _ = sendSignRequestOKEX(key, secret, http.MethodPost, "/api/v5/trade/cancel-order", nil, postData)
	}
	orderJson, err := util.NewJSON(responseBody)
	cancelResult := false
	if err == nil {
		items, _ := orderJson.Get(`data`).Array()
		for _, item := range items {
			value := item.(map[string]interface{})
			if value[`ordId`] != nil && value[`ordId`].(string) == orderId && value[`sCode`].(string) == `0` {
				cancelResult = true
				break
			} else if value[`algoId`] != nil && value[`algoId`].(string) == orderId &&
				(value[`sCode`].(string) == `0` || value[`sCode`].(string) == `51400`) {
				cancelResult = true
				break
			}
		}
		return cancelResult, ``, ``
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`fail to cancelOrder okex %#v`, orderJson))
	return false, err.Error(), err.Error()
}

func parseOrderOKEX(value map[string]interface{}) (order *model.Order) {
	if value == nil {
		return nil
	}
	order = &model.Order{Market: model.OKEX}
	if value[`ordId`] != nil && value[`ordId`].(string) != `0` && value[`ordId`].(string) != `` {
		order.OrderId = value[`ordId`].(string)
		order.ClientOrdId = value[`clOrdId`].(string)
	} else if value[`algoId`] != nil {
		order.OrderId = value[`algoId`].(string)
		order.ClientOrdId = value[`algoClOrdId`].(string)
	}
	if value[`px`] != nil && value[`px`] != `` {
		order.Price, _ = strconv.ParseFloat(value[`px`].(string), 64)
	}
	if value[`ordType`] != nil { // market：市价单 limit：限价单 post_only：只做maker单 fok：全部成交或立即取消 ioc：立即成交并取消剩余
		switch value[`ordType`].(string) {
		case `market`:
			order.OrderType = model.OrderTypeMarket
		case `limit`:
			order.OrderType = model.OrderTypeLimit
		case `conditional`:
			order.OrderType = model.OrderTypeStop
		case `move_order_stop`:
			order.OrderType = model.OrderTypeTrailStop
		}
	}
	if value[`side`] != nil {
		if strings.Contains(strings.ToLower(value[`side`].(string)), `buy`) {
			order.OrderSide = model.OrderSideBuy
		} else if strings.Contains(strings.ToLower(value[`side`].(string)), `sell`) {
			order.OrderSide = model.OrderSideSell
		}
	}
	if value[`sz`] != nil && value[`sz`] != `` {
		order.Amount, _ = strconv.ParseFloat(value[`sz`].(string), 64)
	}
	if value[`accFillSz`] != nil && value[`accFillSz`] != `` {
		order.DealAmount, _ = strconv.ParseFloat(value[`accFillSz`].(string), 64)
	}
	if value[`avgPx`] != nil && value[`avgPx`] != `` {
		order.DealPrice, _ = strconv.ParseFloat(value[`avgPx`].(string), 64)
	}
	if value[`slTriggerPx`] != nil && value[`slTriggerPx`] != `` {
		order.TriggerPrice, _ = strconv.ParseFloat(value[`slTriggerPx`].(string), 64)
	} else if value[`tpTriggerPx`] != nil && value[`tpTriggerPx`] != `` {
		order.TriggerPrice, _ = strconv.ParseFloat(value[`tpTriggerPx`].(string), 64)
	} else if value[`triggerPx`] != nil && value[`triggerPx`] != `` {
		order.TriggerPrice, _ = strconv.ParseFloat(value[`triggerPx`].(string), 64)
	} else if value[`moveTriggerPx`] != nil && value[`moveTriggerPx`] != `` {
		order.TriggerPrice, _ = strconv.ParseFloat(value[`moveTriggerPx`].(string), 64)
	}
	if value[`state`] != nil {
		status := value[`state`].(string)
		switch status {
		case `canceled`:
			order.Status = model.CarryStatusFail
		case `live`, `partially_filled`, `pending`:
			order.Status = model.CarryStatusWorking
		case `filled`:
			order.Status = model.CarryStatusSuccess
		default:
			order.Status = model.CarryStatusFail
		}
	}
	if value[`fee`] != nil && value[`fee`] != `` { // 订单交易手续费，平台向用户收取的交易手续费，手续费扣除 为负数。如： -0.01
		order.Fee, _ = strconv.ParseFloat(value[`fee`].(string), 64)
	}
	if value[`cTime`] != nil && value[`cTime`] != `` {
		ts, _ := strconv.ParseInt(value[`cTime`].(string), 10, 64)
		order.OrderTime = time.Unix(ts/1000, 0)
	}
	if value[`uTime`] != nil && value[`uTime`] != `` {
		ts, _ := strconv.ParseInt(value[`uTime`].(string), 10, 64)
		order.OrderUpdateTime = time.UnixMilli(ts)
	}
	clOrdId := ``
	if value[`algoClOrdId`] != nil {
		clOrdId = value[`algoClOrdId`].(string)
	} else if value[`clOrdId`] != nil {
		clOrdId = value[`clOrdId`].(string)
	}
	if strings.Contains(clOrdId, OKSeparator) {
		clOrdId = clOrdId[:strings.Index(clOrdId, OKSeparator)]
		order.AccountIndex, _ = strconv.Atoi(clOrdId)
	}
	if value[`sCode`] != nil {
		order.ErrCode = value[`sCode`].(string)
		if order.ErrCode != `0` && strings.Trim(order.ErrCode, ` `) != `` {
			order.Status = model.CarryStatusFail
		}
	}
	if value[`instId`] != nil {
		marketType := model.MarketTypeSpot
		if strings.Contains(value[`instId`].(string), `-USDT-SWAP`) {
			marketType = model.MarketTypePerp
		}
		success, _, symbol := model.GetFromDialect(model.OKEX, marketType, value[`instId`].(string))
		if success {
			order.Symbol = symbol
		} else {
			return nil
		}
		if marketType == model.MarketTypePerp {
			_, order.Amount = model.ParseRealAmount(model.OKEX, order.Symbol, order.Amount)
			_, order.DealAmount = model.ParseRealAmount(model.OKEX, order.Symbol, order.DealAmount)
		}
	}
	return order
}

// getPriceOKEX
func _(key, secret, symbol string) (success bool, price float64) {
	param := map[string]interface{}{`instId`: symbol}
	path := `/api/v5/market/ticker`
	responseBody, _ := sendSignRequestOKEX(key, secret, http.MethodGet, path, param, nil)
	tickerJson, err := util.NewJSON(responseBody)
	if err != nil || tickerJson == nil || tickerJson.Get(`data`) == nil || len(tickerJson.Get(`data`).MustArray()) <= 0 {
		return false, 0
	}
	tickerMap := tickerJson.Get(`data`).MustArray()[0].(map[string]interface{})
	price, err = strconv.ParseFloat(tickerMap[`last`].(string), 64)
	return err == nil, price
}

// re-query if return code 50011: Too Many Requests
func queryOpenOrdersOKEX(key, secret, symbol string, conditional bool) (orders []*model.Order) {
	param := make(map[string]interface{})
	if len(symbol) > 0 {
		param[`instId`] = symbol
	}
	path := `/api/v5/trade/orders-pending`
	if conditional {
		path = `/api/v5/trade/orders-algo-pending`
		param[`ordType`] = `conditional`
	}
	responseBody, _ := sendSignRequestOKEX(key, secret, http.MethodGet, path, param, nil)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		if `50011` == strings.Trim(orderJson.Get(`code`).MustString(), ` `) {
			util.Log(util.LogLevelError, `sleep 1 min and re-query orders when get code 50011`)
			time.Sleep(time.Minute)
			return queryOpenOrdersOKEX(key, secret, symbol, conditional)
		}
	}
	if err != nil || orderJson == nil || orderJson.Get(`data`) == nil {
		return
	}
	ordersJson := orderJson.Get("data").MustArray()
	orders = make([]*model.Order, 0)
	for _, item := range ordersJson {
		value := item.(map[string]interface{})
		order := parseOrderOKEX(value)
		if order.OrderId != `` {
			orders = append(orders, order)
		}
	}
	return
}

func getAlgoOrderIdOKEX(key, secret, algoId string) (ordId, clientOrderId string) {
	param := map[string]interface{}{`algoId`: algoId, `ordType`: `conditional`}
	responseBody, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/trade/orders-algo-history`, param, nil)
	orderJson, err := util.NewJSON(responseBody)
	if err != nil || orderJson == nil || orderJson.Get(`data`) == nil || orderJson.Get(`code`).MustString() != `0` {
		return ``, ``
	}
	orders := orderJson.Get(`data`).MustArray()
	for _, item := range orders {
		value := item.(map[string]interface{})
		if value[`algoId`] == algoId {
			return value[`ordId`].(string), value[`algoClOrdId`].(string)
		}
	}
	return ``, ``
}

func queryOrderOKEX(key, secret, symbol, orderId, orderType string) (order *model.Order) {
	path := `/api/v5/trade/order`
	var clientOrderId string
	param := map[string]interface{}{"ordId": orderId, "instId": symbol}
	if orderType == model.OrderTypeStop {
		path = `/api/v5/trade/order-algo`
		param = map[string]interface{}{`algoId`: orderId, `ordType`: `conditional`}
	} else if orderType == model.OrderTypeTrailStop {
		path = `/api/v5/trade/order-algo`
		param = map[string]interface{}{`algoId`: orderId, `ordType`: `move_order_stop`}
	}
	responseBody, _ := sendSignRequestOKEX(key, secret, http.MethodGet, path, param, nil)
	orderJson, err := util.NewJSON(responseBody)
	if err != nil || orderJson == nil || orderJson.Get(`data`) == nil || orderJson.Get(`code`) == nil {
		return nil
	}
	if strings.Trim(orderJson.Get(`code`).MustString(), ` `) == `51603` {
		if orderType == model.OrderTypeStop || orderType == model.OrderTypeTrailStop {
			orderId, clientOrderId = getAlgoOrderIdOKEX(key, secret, orderId)
			if orderId != `` {
				return queryOrderOKEX(key, secret, symbol, orderId, model.OrderTypeLimit)
			}
		}
		return &model.Order{OrderId: orderId, ClientOrdId: clientOrderId, OrderType: orderType, Status: model.CarryStatusFail, Symbol: symbol}
	}
	orders := orderJson.Get("data").MustArray()
	for _, item := range orders {
		value := item.(map[string]interface{})
		if orderType != model.OrderTypeStop && orderType != model.OrderTypeTrailStop {
			if value[`ordId`] != nil && value[`ordId`].(string) == orderId {
				order = parseOrderOKEX(value)
				break
			}
		} else {
			if value[`ordId`] != nil {
				ordId := strings.Trim(value[`ordId`].(string), ` `)
				if ordId != `` && ordId != `0` {
					orderType = model.OrderTypeLimit
					return queryOrderOKEX(key, secret, symbol, ordId, orderType)
				}
			}
			if value[`algoId`] == orderId {
				order = parseOrderOKEX(value)
				break
			}
		}
	}
	if order != nil {
		order.OrderType = orderType
	}
	return order
}

func getTransferOKEX(key, secret string) (balances []*model.Balance) {
	response, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/asset/deposit-history`, nil, nil)
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
	response, _ = sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/asset/withdrawal-history`, nil, nil)
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

func parsePositionOKEX(value map[string]interface{}) (success bool, position *Position) {
	position = &Position{Market: model.OKEX}
	if value[`lever`] != nil && value[`lever`] != `` { // 杠杆倍数，不适用于期权
		position.LeverRate, _ = strconv.ParseInt(value[`lever`].(string), 10, 64)
	}
	if value[`liqPx`] != nil && value[`liqPx`] != `` { // 预估强平价 不适用于跨币种保证金模式下交割/永续的全仓 不适用于期权
		position.LiquidationPrice, _ = strconv.ParseFloat(value[`liqPx`].(string), 64)
	}
	if value[`mmr`] != nil && value[`mmr`] != `` { // 维持保证金
		position.MinimumMaintenanceMargin, _ = strconv.ParseFloat(value[`mmr`].(string), 64)
	}
	if value[`avgPx`] != nil && value[`avgPx`] != `` { // 开仓平均价
		position.EntryPrice, _ = strconv.ParseFloat(value[`avgPx`].(string), 64)
	}
	if value[`upl`] != nil && value[`upl`] != `` { // 未实现收益
		position.ProfitUnreal, _ = strconv.ParseFloat(value[`upl`].(string), 64)
	}
	if value[`uTime`] != nil && value[`uTime`] != `` { // 最近一次持仓更新时间，Unix时间戳的毫秒数格式，如 1597026383085
		position.Ts, _ = strconv.ParseInt(value[`uTime`].(string), 10, 64)
	}
	// 持仓方向 long：双向持仓多头 short：双向持仓空头 net：单向持仓（交割/永续/期权：pos为正代表多头，pos为负代表空头。
	// 币币杠杆：posCcy为交易货币时，代表多头；posCcy为计价货币时，代表空头。）
	if value[`posSide`] != nil {
		position.Direction = value[`posSide`].(string)
	}
	if value[`margin`] != nil && value[`margin`] != `` { // 保证金余额，可增减，仅适用于逐仓
		position.Margin, _ = strconv.ParseFloat(value[`margin`].(string), 64)
	}
	if value[`instId`] != nil { // 	产品ID，如 BTC-USD-180216
		marketType := model.MarketTypeSpot
		if strings.Contains(value[`instId`].(string), `-USDT-SWAP`) {
			marketType = model.MarketTypePerp
		}
		getCoin, _, symbol := model.GetFromDialect(model.OKEX, marketType, value[`instId`].(string))
		if !getCoin {
			return false, nil
		}
		position.Currency = symbol
		//posCcy 仓位资产币种，仅适用于币币杠杆仓位
		if value[`pos`] != nil {
			pos, _ := strconv.ParseFloat(value[`pos`].(string), 64)
			if marketType == model.MarketTypePerp {
				success, position.Holding = model.ParseRealAmount(model.OKEX, position.Currency, pos)
			} else {
				position.Holding = pos
			}
		}
	}
	//pos 持仓数量
	return true, position
}

func parseBalanceOKEX(value map[string]interface{}) (balance *model.Balance) {
	if value == nil || value[`ccy`] == nil {
		return nil
	}
	balance = &model.Balance{Market: model.OKEX, Action: 0, Coin: value[`ccy`].(string),
		ID: model.OKEX + `_` + value[`ccy`].(string) + `_` + util.GetNow().String()[0:10]}
	// for transfer
	if value[`amt`] != nil && value[`amt`] != `` {
		balance.Amount, _ = strconv.ParseFloat(value[`amt`].(string), 64)
	}
	if value[`from`] != nil && value[`to`] != nil {
		balance.Address = fmt.Sprintf(`%s : %s`, value[`from`].(string), value[`to`].(string))
	}
	if value[`wdId`] != nil {
		balance.ID = value[`wdId`].(string)
		balance.Action = -1
	} else if value[`depId`] != nil {
		balance.ID = value[`depId`].(string)
		balance.Action = 1
		if value[`ts`] != nil && value[`ts`] != `` {
			ts, _ := strconv.ParseInt(value[`ts`].(string), 10, 64)
			balance.BalanceTime = time.Unix(ts/1000, 0)
		}
	}
	if value[`txId`] != nil {
		balance.TransactionId = value[`txId`].(string)
	}
	if value[`fee`] != nil {
		balance.Fee, _ = value[`fee`].(string)
	}
	if value[`state`] != nil {
		switch strings.Trim(value[`state`].(string), ` `) {
		case `-3`, `0`, `1`, `3`, `4`, `5`:
			balance.Status = model.CarryStatusWorking
		case `-2`, `-1`:
			balance.Status = model.CarryStatusFail
		case `2`:
			balance.Status = model.CarryStatusSuccess
		}
	}
	if value[`ts`] != nil {
		ts, _ := strconv.ParseInt(value[`ts`].(string), 10, 64)
		balance.CreatedAt = time.Unix(ts/1000, 0)
	}
	// for balance
	//if value[`availEq`] != nil && value[`availEq`] != `` {
	//	balance.Available, _ = strconv.ParseFloat(value[`availEq`].(string), 64)
	//}
	if value[`eq`] != nil && value[`eq`] != `` {
		balance.Amount, _ = strconv.ParseFloat(value[`eq`].(string), 64)
	}
	if value[`eqUsd`] != nil && value[`eqUsd`] != `` {
		balance.UsdValue, _ = strconv.ParseFloat(value[`eqUsd`].(string), 64)
	}
	if value[`crossLiab`] != nil && value[`crossLiab`] != `` {
		balance.Borrow, _ = strconv.ParseFloat(value[`crossLiab`].(string), 64)
	}
	if value[`maxLoan`] != nil && len(strings.TrimSpace(value[`maxLoan`].(string))) > 0 {
		balance.AvailableWithBorrow, _ = strconv.ParseFloat(value[`maxLoan`].(string), 64)
		balance.AvailableWithBorrow += balance.Amount
	}
	return
}

// margin: 可用保证金
func getBalanceOKEX(key, secret string) (success bool, balances []*model.Balance, totalInUsd float64, collateral *model.Collateral) {
	response, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/account/balance`, nil, nil)
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson == nil || responseJson.GetPath(`data`) == nil || responseJson.Get(`code`).MustString() != `0` {
		util.Log(util.LogLevelError, `fail to get okex balance `)
		time.Sleep(time.Minute * 5)
		return getBalanceOKEX(key, secret)
	}
	balances = make([]*model.Balance, 0)
	data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
	if data == nil {
		return
	}
	if data[`totalEq`] != nil {
		totalInUsd, _ = strconv.ParseFloat(data[`totalEq`].(string), 64)
	}
	collateral = &model.Collateral{}
	if data[`adjEq`] != nil {
		collateral.Available, _ = strconv.ParseFloat(data[`adjEq`].(string), 64) // 可用保证金
	}
	if data[`imr`] != nil {
		collateral.Occupied, _ = strconv.ParseFloat(data[`imr`].(string), 64) // 被占用保证金
	}
	if data[`mgnRatio`] != nil {
		collateral.Rate, _ = strconv.ParseFloat(data[`mgnRatio`].(string), 64)
	}
	for _, item := range data[`details`].([]interface{}) {
		balance := parseBalanceOKEX(item.(map[string]interface{}))
		if balance != nil {
			balances = append(balances, balance)
		}
	}
	return true, balances, totalInUsd, collateral
}

func getAccountConfigOKEX(key, secret string) (mode string) {
	response, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/account/config`, nil, nil)
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson.Get(`data`) == nil || len(responseJson.Get(`data`).MustArray()) == 0 {
		return ``
	}
	data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
	return data[`posMode`].(string)
}

func setAccountModeOKEX(key, secret string) (success bool) {
	response, _ := sendSignRequestOKEX(key, secret, http.MethodPost, `/api/v5/account/set-position-mode`, nil,
		map[string]interface{}{`posMode`: `net_mode`})
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson.Get(`code`).MustString() != `0` {
		return false
	}
	return true
}

var settingOkx = false

func setLeverageOkx(account *model.Account) (success bool) {
	if settingOkx {
		return
	}
	defer func() {
		settingOkx = false
	}()
	settingOkx = true
	symbols := GetMarketSymbols(model.OKEX)
	for symbol := range symbols {
		setSymbolLeverageOkx(account, symbol)
		time.Sleep(time.Minute)
	}
	return true
}

func setSymbolLeverageOkx(account *model.Account, symbol string) (setSuc bool) {
	ok, _, coin, _ := model.GetFromStandard(model.OKEX, symbol)
	if !ok {
		return false
	}
	leverage := model.DefaultLeverage
	if model.CommonCoins[strings.ToLower(coin)] {
		leverage = 10
	}
	response, _ := sendSignRequestOKEX(account.Key, account.Secret, http.MethodPost, `/api/v5/account/set-leverage`,
		nil, map[string]interface{}{`instId`: symbol, `mgnMode`: `cross`, `lever`: strconv.Itoa(leverage)})
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson.Get(`code`).MustString() != `0` {
		return false
	}
	return true
}

// getLastPriceOKEX
func _(key, secret, symbol string) (price float64) {
	response, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/market/ticker`,
		map[string]interface{}{`instId`: symbol}, nil)
	responseJson, err := util.NewJSON(response)
	if responseJson == nil || err != nil || responseJson.Get(`data`) == nil ||
		len(responseJson.Get(`data`).MustArray()) == 0 {
		return 0
	}
	value := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
	if value[`last`] != nil {
		price, _ = strconv.ParseFloat(value[`last`].(string), 64)
	}
	return price
}

// 目前只支持永续
func getPositionsOKEX(key, secret string) (success bool, positions []*Position) {
	param := map[string]interface{}{`instType`: `SWAP`}
	responseBody, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/account/positions`, param, nil)
	responseJson, err := util.NewJSON(responseBody)
	if err != nil || responseJson == nil || responseJson.Get(`code`).MustString() != `0` {
		util.Log(util.LogLevelError, `fail to get okex positions `)
		time.Sleep(time.Minute)
		return getPositionsOKEX(key, secret)
	}
	positions = make([]*Position, 0)
	positionArray, arrayErr := responseJson.Get(`data`).Array()
	if arrayErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to get okex positions %s`, arrayErr.Error()))
		time.Sleep(time.Minute)
		return getPositionsOKEX(key, secret)
	}
	for _, item := range positionArray {
		result, position := parsePositionOKEX(item.(map[string]interface{}))
		if result && position.Holding != 0 {
			positions = append(positions, position)
			//util.Log(util.LogLevelInfo, fmt.Sprintf(`get position okex %#v`, position))
		}
	}
	return true, positions
}

// getMaxSizeOKEX
// 实测现货maxBuy是对应的交易币的数量，现货maxSell是计价币的数量，故需除以价格；
// 期货的maxBuy和maxSell都是币的数量，无需除以价格
func getMaxSizeOKEX(key, secret, symbol string) (success bool, maxBuy, maxSell float64) {
	param := map[string]interface{}{`instId`: symbol, `tdMode`: `cross`}
	response, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/account/max-size`, param, nil)
	responseJson, err := util.NewJSON(response)
	if responseJson == nil || err != nil || responseJson.Get(`data`) == nil ||
		responseJson.Get(`data`).MustArray() == nil || len(responseJson.Get(`data`).MustArray()) == 0 {
		return false, 0, 0
	}
	data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
	if data[`maxBuy`] != nil {
		maxBuy, _ = strconv.ParseFloat(data[`maxBuy`].(string), 64)
	}
	if data[`maxSell`] != nil {
		maxSell, _ = strconv.ParseFloat(data[`maxSell`].(string), 64)
		_, marketType, _, _ := model.GetFromStandard(model.Kucoin, symbol)
		if marketType == model.MarketTypeSpot {
			ok, bidAsk := model.AppEnvironment.GetBidAsk(model.OKEX, symbol)
			if ok {
				maxSell = maxSell / bidAsk.Asks[0].Price
				//util.Info(`get max sell %f after price %f %s`, maxSell, bidAsk.Asks[0].Price, symbol)
			} else {
				util.LogLess(util.LogLevelError, fmt.Sprintf(`fail to get price from ok bidAsk %s`, symbol))
			}
		}
	}
	_, maxBuy = model.ParseRealAmount(model.OKEX, symbol, maxBuy)
	_, maxSell = model.ParseRealAmount(model.OKEX, symbol, maxSell)
	return true, maxBuy, maxSell
}

func getFundingRateOKEX(key, secret, symbol string) (fundingRate *model.FundingRate) {
	param := map[string]interface{}{`instId`: symbol}
	response, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/public/funding-rate`, param, nil)
	fundingJson, fundingErr := util.NewJSON(response)
	if fundingJson == nil || fundingJson.Get(`data`) == nil || fundingJson.Get(`data`).MustArray() == nil ||
		len(fundingJson.Get(`data`).MustArray()) == 0 || fundingErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to getFundingRateOKEX %#v`, fundingErr))
		return nil
	}
	data := fundingJson.Get(`data`).MustArray()[0].(map[string]interface{})
	rate, _ := strconv.ParseFloat(data[`fundingRate`].(string), 64)
	//rateNext, _ := strconv.ParseFloat(data[`nextFundingRate`].(string), 64)
	rateTime, _ := strconv.ParseInt(data[`fundingTime`].(string), 10, 64)
	fundingTimeNext, _ := strconv.ParseInt(data[`nextFundingTime`].(string), 10, 64)
	marketInfo := model.GetMarketInfo(model.OKEX, symbol)
	if marketInfo != nil {
		marketInfo.FundingRateInterval = int(fundingTimeNext - rateTime)
	}
	return &model.FundingRate{
		Rate:       rate,
		UpdateTime: util.GetNow(),
		ExpireTime: rateTime / 1000}
}

func getMaxLoanOKEX(key, secret, symbol string) (success bool, maxLoan float64) {
	param := map[string]interface{}{`instId`: symbol, `mgnMode`: `cross`}
	response, _ := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/account/max-loan`, param, nil)
	loanJson, err := util.NewJSON(response)
	success = false
	if loanJson == nil || err != nil || loanJson.Get(`data`) == nil {
		return false, 0
	}
	value := loanJson.Get(`data`).MustArray()
	for _, item := range value {
		data := item.(map[string]interface{})
		getCoin, _, coin, _ := model.GetFromStandard(model.OKEX, symbol)
		if data[`maxLoan`] != nil && data[`ccy`] != nil && data[`ccy`].(string) == coin && getCoin {
			maxLoan, _ = strconv.ParseFloat(data[`maxLoan`].(string), 64)
			success = true
		}
	}
	return success, maxLoan
}

// 参数before代表返回的candle是该事件之后的，且不好该时间，所以参数传入的时候需要减去一个slotSeconds
// getCandlesOKEX bar 1m/3m/5m/15m/30m/1H/2H/4H/6H/12H/1D/1W/1M/3M/6M/1Y
// 1m的数据只支持过去24小时内的
func getCandlesOKEX(account *model.Account, symbol string, before, after time.Time, count, slotSeconds int) (
	candles []*model.Candle, isCache bool) {
	candles = make([]*model.Candle, 0)
	bar := `1D`
	switch slotSeconds {
	case 60:
		bar = `1m`
	case 1800:
		bar = `30m`
	case 3600:
		bar = `1H`
	case 86400:
		bar = `1D`
	}
	path := `/api/v5/market/candles`
	//path := `/api/v5/market/history-candles`
	diffDuration, _ := time.ParseDuration(fmt.Sprintf(`-%ds`, slotSeconds))
	param := map[string]interface{}{`instId`: symbol, `bar`: bar, `limit`: count,
		`before`: before.Add(diffDuration).UnixNano() / int64(time.Millisecond), `after`: after.UnixNano() / int64(time.Millisecond)}
	redisKey := fmt.Sprintf(`%s_%s_%s_%d_%d_%d`, model.OKEX, symbol, bar, before.UnixMilli(), after.UnixMilli(), count)
	var responseBody []byte
	if model.AppRedis != nil {
		temp, redisErr := model.AppRedis.Get(context.Background(), redisKey).Result()
		if redisErr == nil {
			responseBody = util.UnGzip([]byte(temp))
			isCache = true
			util.Log(util.LogLevelInfo, fmt.Sprintf(`get candles from key %s %d`, redisKey, len(temp)))
		}
	}
	if responseBody == nil {
		isCache = false
		responseBody, _ = sendSignRequestOKEX(account.Key, account.Secret, http.MethodGet, path, param, nil)
	}
	candleJson, err := util.NewJSON(responseBody)
	if err != nil || candleJson == nil || candleJson.Get(`data`) == nil || len(candleJson.Get(`data`).MustArray()) == 0 {
		if model.AppRedis != nil {
			model.AppRedis.Del(context.Background(), redisKey)
			util.Log(util.LogLevelInfo, fmt.Sprintf(`del redis key %s`, redisKey))
		}
		return
	} else if !isCache && model.AppRedis != nil {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`set candles to cache %s len %d`, redisKey, len(string(responseBody))))
		model.AppRedis.Set(context.Background(), redisKey, util.Compress(responseBody), 0)
	}
	candleJsons := candleJson.Get(`data`).MustArray()
	if len(candleJsons) < count && isCache {
		responseBody, _ = sendSignRequestOKEX(account.Key, account.Secret, http.MethodGet, path, param, nil)
		model.AppRedis.Set(context.Background(), redisKey, util.Compress(responseBody), 0)
		candleJson, err = util.NewJSON(responseBody)
		if candleJson != nil {
			candleJsons = candleJson.Get(`data`).MustArray()
		}
	}
	// 由于okx交易所返回的数据是从近到以前的，所以进行了倒序
	for i := len(candleJsons) - 1; i >= 0; i-- {
		item := candleJsons[i].([]interface{})
		if len(item) < 7 {
			continue
		}
		candle := &model.Candle{Market: model.OKEX, Symbol: symbol, Seconds: slotSeconds}
		ts, _ := strconv.ParseInt(item[0].(string), 10, 64)
		candle.Begin = time.Unix(ts/1000, 0).In(before.Location())
		candle.PriceOpen, _ = strconv.ParseFloat(item[1].(string), 64)
		candle.PriceHigh, _ = strconv.ParseFloat(item[2].(string), 64)
		candle.PriceLow, _ = strconv.ParseFloat(item[3].(string), 64)
		candle.PriceClose, _ = strconv.ParseFloat(item[4].(string), 64)
		candle.Volume, _ = strconv.ParseFloat(item[7].(string), 64)
		candles = append(candles, candle)
	}
	return
}

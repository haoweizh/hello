package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"hash/crc32"
	"hello/model"
	"hello/util"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const wsPrivateOKEX = `wss://ws.okex.com:8443/ws/v5/private`

var msgChanOKEX = make(map[string]chan *simplejson.Json)
var wrongs = make(map[string]bool)
var wrongLock sync.Mutex

func init() {
	go reSubscribe()
	go pingOKEX()
}

func getWrongs() []string {
	defer wrongLock.Unlock()
	wrongLock.Lock()
	array := make([]string, len(wrongs))
	for s := range wrongs {
		array = append(array, s)
	}
	return array
}

func setWrong(instrument string, success bool) {
	defer wrongLock.Unlock()
	wrongLock.Lock()
	if !success {
		wrongs[instrument] = true
	} else {
		delete(wrongs, instrument)
	}
}

func pingOKEX() {
	for true {
		time.Sleep(time.Second * 20)
		go func() {
			keys, _ := model.AppConfig.GetKeys(model.OKEX)
			for _, key := range keys {
				channelKey := model.OKEX + `_` + key
				err := sendToWs(channelKey, []byte(`ping`))
				if err != nil {
					util.SocketInfo("okex server ping client error " + err.Error())
				}
			}
		}()
	}
}

func reSubscribe() {
	for true {
		time.Sleep(time.Minute)
		if len(wrongs) == 0 {
			continue
		}
		wrongArray := getWrongs()
		util.Notice(fmt.Sprintf(`>>>>>>>>wrong instrument %v`, wrongArray))
		subscribeMap := make(map[string]interface{})
		subscribeMap["op"] = "unsubscribe"
		subArray := make([]map[string]string, 0)
		for _, instrument := range wrongArray {
			subArray = append(subArray, map[string]string{`channel`: `books50-l2-tbt`, `instId`: instrument})
		}
		subscribeMap[`args`] = subArray
		err := sendToWs(model.OKEX, util.JsonEncodeToByte(subscribeMap))
		if err != nil {
			util.SocketInfo("okex can not unsubscribe " + err.Error())
		}
		time.Sleep(time.Second * 3)
		subscribeMap["op"] = "subscribe"
		err = sendToWs(model.OKEX, util.JsonEncodeToByte(subscribeMap))
		if err != nil {
			util.SocketInfo("okex can not re-subscribe " + err.Error())
		}
	}
}

var subscriberOKEXPrivate = func(subscribes []interface{}, key string) error {
	var err error = nil
	loginMap := make(map[string]interface{})
	loginMap[`op`] = `login`
	keys, secrets := model.AppConfig.GetKeys(model.OKEX)
	secret := ``
	for i, value := range keys {
		if value == key {
			secret = secrets[i]
		}
	}
	timestamp := time.Now().Unix()
	toBeSign := fmt.Sprintf(`%dGET/users/self/verify`, timestamp)
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(toBeSign))
	sign := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	loginArray := []map[string]interface{}{{
		`apiKey`: key, `passphrase`: model.AppConfig.Phase, `timestamp`: timestamp, `sign`: sign}}
	loginMap[`args`] = loginArray
	err = sendToWs(model.OKEX+`_`+key, util.JsonEncodeToByte(loginMap))
	if err != nil {
		util.SocketInfo(fmt.Sprintf(`fail to login okex ws: %s return %s`, key, err.Error()))
	}
	return err

}

var subscribeHandlerOKEX = func(subscribes []interface{}, subType string) error {
	var err error = nil
	step := 30
	for i := 0; i < len(subscribes); i += step {
		subscribeMap := make(map[string]interface{})
		subscribeMap["op"] = "subscribe"
		subArray := make([]map[string]string, 0)
		for j := i; j < len(subscribes) && j < i+step; j++ {
			subArray = append(subArray, map[string]string{`channel`: `books50-l2-tbt`, `instId`: subscribes[j].(string)})
			//subArray = append(subArray, map[string]string{`channel`: `books5`, `instId`: subscribes[j].(string)})
		}
		subscribeMap[`args`] = subArray
		subscribeMessage := util.JsonEncodeToByte(subscribeMap)
		if err = sendToWs(model.OKEX, subscribeMessage); err != nil {
			util.SocketInfo("okex can not subscribe " + err.Error())
			return err
		}
	}
	return err
}

func handleMsgOKEX(channel chan *simplejson.Json, instrument string) {
	var responseJson *simplejson.Json
	for responseJson = range channel {
		if len(channel) > 20 {
			util.Notice(fmt.Sprintf(`%s current chan to be handle %d`, instrument, len(channel)))
		}
		symbol := model.GetInstrumentSymbol(model.OKEX, instrument)
		action := responseJson.Get(`action`).MustString()
		data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
		_, bidAsk := model.AppMarkets.GetBidAsk(instrument, model.OKEX)
		success := false
		if action == `update` && bidAsk != nil {
			success, bidAsk = handleBooksUpdate(instrument, data, bidAsk)
		} else if action == `snapshot` || responseJson.GetPath(`arg`, `channel`).MustString() == `books5` {
			//if action == `snapshot` {
			//	util.Notice(fmt.Sprintf(`++++ %s initial ticker %v`, instrument, data))
			//}
			bidAsk = handleBooksOKEX(instrument, data)
			success = true
		}
		if bidAsk == nil || !success {
			continue
		}
		if model.AppMarkets.SetBidAsk(instrument, model.OKEX, bidAsk) {
			for function, handler := range model.GetFunctions(model.OKEX, symbol) {
				settings := model.GetSetting(function, model.OKEX, symbol)
				for _, setting := range settings {
					go handler(setting, bidAsk)
				}
			}
		}
	}
}

//lastPingTime := util.GetNow().Unix()
var wsHandler = func(channelKey string, event []byte, orderHandler OrderHandler) {
	//now := util.GetNow().Unix()
	//if now-lastPingTime > 25 { // ping okex server every 30 seconds
	//	lastPingTime = now
	//	go func() {
	//		err := sendToWs(model.OKEX, []byte(`ping`))
	//		if err != nil {
	//			util.SocketInfo("okex server ping client error " + err.Error())
	//		}
	//	}()
	//}
	responseJson, err := util.NewJSON(event)
	if err != nil || responseJson == nil || responseJson.Get(`data`) == nil ||
		len(responseJson.Get(`data`).MustArray()) == 0 ||
		responseJson.GetPath(`arg`, `instId`) == nil {
		return
	}
	instrument := responseJson.GetPath(`arg`, `instId`).MustString()
	channel := msgChanOKEX[instrument]
	if channel != nil {
		channel <- responseJson
	}
}

var wsHandlerPrivate = func(channelKey string, event []byte, orderHandler OrderHandler) {
	responseJson, err := util.NewJSON(event)
	if err != nil || responseJson == nil {
		return
	}
	if responseJson.Get(`event`).MustString() == `login` {
		subscribes := GetWSSubscribes(model.OKEX, model.SubscribeDepth)
		step := 30
		for i := 0; i < len(subscribes); i += step {
			subscribeMap := make(map[string]interface{})
			subscribeMap["op"] = "subscribe"
			subArray := make([]map[string]string, 0)
			for j := i; j < len(subscribes) && j < i+step; j++ {
				subArray = append(subArray, map[string]string{`channel`: `orders`, `instType`: `ANY`, `instId`: subscribes[j].(string)})
			}
			subscribeMap[`args`] = subArray
			if err = sendToWs(channelKey, util.JsonEncodeToByte(subscribeMap)); err != nil {
				util.SocketInfo("okex can not subscribe private " + err.Error())
				continue
			}
		}
	}
	util.Info(fmt.Sprintf(`>>> %s`, string(event)))
	if responseJson.Get(`data`) == nil || len(responseJson.Get(`data`).MustArray()) == 0 {
		return
	}
	if responseJson.Get(`code`).MustString() != `0` {
		util.Notice(fmt.Sprintf(`okex private channel error: %s`, string(event)))
	}
	data := responseJson.Get(`data`).MustArray()
	for _, item := range data {
		value := item.(map[string]interface{})
		value[`channelKey`] = channelKey
		go handleWSOrderOKEX(value, orderHandler)
	}
}

func handleWSOrderOKEX(value map[string]interface{}, orderHandler OrderHandler) {
	order := parseOrderOKEX(value)
	dbOrder := model.Order{}
	model.AppDB.Where(`order_id=?`, order.OrderId).First(&dbOrder)
	if dbOrder.OrderId != `` {
		order.ID = dbOrder.ID
		util.Info(fmt.Sprintf(`query order %s id %d`, order.OrderId, dbOrder.ID))
	} else {
		util.Info(`db can not get orderId %s`, order.OrderId)
	}
	model.AppDB.Save(&order)
	orderHandler(order)
}

func WsDepthServeOKEX(instruments map[string]bool, orderHandler OrderHandler) (chan struct{}, error) {
	for s := range instruments {
		if msgChanOKEX[s] == nil {
			msgChanOKEX[s] = make(chan *simplejson.Json, 1000)
			go handleMsgOKEX(msgChanOKEX[s], s)
		}
	}
	keys, _ := model.AppConfig.GetKeys(model.OKEX)
	for _, key := range keys {
		channelKey := key
		go func() {
			_, err := WebSocketClient(model.OKEX+`_`+channelKey, wsPrivateOKEX, channelKey,
				GetWSSubscribes(model.OKEX, model.SubscribeDepth), subscriberOKEXPrivate, wsHandlerPrivate, orderHandler)
			if err != nil {
				util.SocketInfo(fmt.Sprintf(`fail to connect okex private %s %s`, channelKey, err.Error()))
			}
		}()
	}
	return WebSocketClient(model.OKEX, model.AppConfig.WSUrls[model.OKEX], model.SubscribeDepth,
		GetWSSubscribes(model.OKEX, model.SubscribeDepth), subscribeHandlerOKEX, wsHandler, orderHandler)
}

func handleBooksUpdate(instrument string, data map[string]interface{}, bidAsk *model.BidAsk) (
	success bool, bidAskUpdate *model.BidAsk) {
	bidAskUpdate = handleBooksOKEX(instrument, data)
	if data[`ts`] != nil {
		ts, _ := strconv.ParseInt(data[`ts`].(string), 10, 64)
		bidAskUpdate.Ts = int(ts)
	}
	newAsks := make([]model.Tick, 0)
	newBids := make([]model.Tick, 0)
	//if bidAskUpdate.Asks.Len() == 0 || bidAskUpdate.Bids.Len() == 0 {
	//	util.Notice(fmt.Sprintf(`empty bid/ask update %s`, instrument)) //}
	i := 0
	j := 0
	for true {
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
	for true {
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
	if data[`checksum`] != nil {
		checkStr := ``
		for index := 0; index < 25; index++ {
			if index < len(newBids) {
				checkStr += fmt.Sprintf(`%s:%s:`, newBids[index].PriceStr, newBids[index].AmountStr)
			}
			if index < len(newAsks) {
				checkStr += fmt.Sprintf(`%s:%s:`, newAsks[index].PriceStr, newAsks[index].AmountStr)
			}
		}
		checkStr = checkStr[0 : len(checkStr)-1]
		crcValue := int64(int32(crc32.ChecksumIEEE([]byte(checkStr))))
		compare, _ := data[`checksum`].(json.Number).Int64()
		bidAskUpdate.Bids = newBids
		bidAskUpdate.Asks = newAsks
		if compare == crcValue {
			success = true
		} else {
			success = false
		}
		setWrong(instrument, success)
		if !success && time.Now().Second() == 0 {
			util.Info(fmt.Sprintf("%v ts %d checksum %s wrong size: %d \n [[[ %s\n ]]] %v",
				success, bidAskUpdate.Ts, instrument, len(wrongs), checkStr, data))
		}
	}
	return success, bidAskUpdate
}

func handleBooksOKEX(instrument string, data map[string]interface{}) (bidAsk *model.BidAsk) {
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
			bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Symbol: instrument, PriceStr: priceStr, AmountStr: amountStr}
		}
	}
	for i, bid := range bids {
		value := bid.([]interface{})
		if len(value) >= 2 {
			priceStr := value[0].(string)
			amountStr := value[1].(string)
			price, _ := strconv.ParseFloat(priceStr, 64)
			amount, _ := strconv.ParseFloat(amountStr, 64)
			bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Symbol: instrument, PriceStr: priceStr, AmountStr: amountStr}
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
	//		util.Notice(fmt.Sprintf(`right checksum snapshot %s %d`, instrument, bidAsk.Bids.Len()))
	//	} else {
	//		util.Notice(fmt.Sprintf("%v +++++++ checksum %s\n %s\n %v",
	//			false, instrument, checkStr, data))
	//	}
	//}
	return
}

func sendSignRequestOKEX(key, secret, method, path string, body interface{}) (responseBody []byte) {
	if key == `` || secret == `` {
		keys, secrets := model.AppConfig.GetKeys(model.OKEX)
		key = keys[0]
		secret = secrets[0]
	}
	uri := model.AppConfig.RestUrls[model.OKEX] + path
	current := time.Now().In(time.UTC).Format(time.RFC3339)
	// , `x-simulated-trading`: `1`
	headers := map[string]string{`OK-ACCESS-KEY`: key, `OK-ACCESS-PASSPHRASE`: model.AppConfig.Phase,
		"OK-ACCESS-TIMESTAMP": current, "Content-Type": "application/json"}
	postContent := ``
	if method == http.MethodPost {
		postContent = string(util.JsonEncodeToByte(body))
	}
	toBeSign := fmt.Sprintf(`%s%s%s%s`, current, method, path, postContent)
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(toBeSign))
	sign := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	headers[`OK-ACCESS-SIGN`] = sign
	responseBody, _ = util.HttpRequest(method, uri, postContent, headers, 60)
	if strings.Contains(uri, `/api/v5/trade/order`) {
		util.Notice(fmt.Sprintf(`okex key %s request %s body %s return %s`,
			key, uri, toBeSign, string(responseBody)))
	}
	util.SocketInfo(fmt.Sprintf(`okex key %s request %s body %s return %s`,
		key, uri, toBeSign, string(responseBody)))
	return responseBody
}

// amount、price
// 不能使用 fmt %v 因为有e+5 的情况；
// 不能使用 fmt %f 因为有000后缀；
// 不能使用 strconv.FormatFloat 因为有 2.00000001问题
//priceStr := strconv.FormatFloat(order.Price, 'f', -1, 64)
//triggerPriceStr := strconv.FormatFloat(order.TriggerPrice, 'f', -1, 64)
func placeOrderOKEX(key, secret string, isWs bool, order *model.Order) {
	price, decimal := FormatPrice(model.OKEX, order.Instrument, order.OrderSide, order.Price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	priceTrigger, decimal := FormatPrice(model.OKEX, order.Instrument, order.OrderSide, order.TriggerPrice)
	triggerPriceStr := util.CutTailZero(strconv.FormatFloat(priceTrigger, 'f', decimal, 64))
	formattedAmount := GetAmountInMarket(model.OKEX, order.Instrument, order.Amount)
	amount := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	if order.OrderType == model.OrderTypeMarket {
		usdAmount, _ := strconv.ParseFloat(amount, 64)
		amount = util.CutTailZero(fmt.Sprintf(`%f`, usdAmount*order.Price))
	}
	if price == 0 || formattedAmount == 0 {
		order.Status = model.CarryStatusFail
		return
	}
	postData := map[string]interface{}{`instId`: order.Instrument, `tdMode`: `cross`, `side`: order.OrderSide,
		`sz`: amount, `ordType`: order.OrderType}
	path := "/api/v5/trade/order"
	if order.OrderType == model.OrderTypeStop {
		postData[`ordType`] = `conditional`
		postData[`slOrdPx`] = priceStr
		postData[`slTriggerPx`] = triggerPriceStr
		path = `/api/v5/trade/order-algo`
	} else {
		postData[`px`] = priceStr
	}
	if isWs {
		subscribeMap := make(map[string]interface{})
		subscribeMap[`id`] = strconv.FormatInt(time.Now().UnixNano(), 10)
		subscribeMap["op"] = "batch-orders"
		postData[`tag`] = order.RefreshType
		subscribeMap[`args`] = []map[string]interface{}{postData}
		wsOrderMsg := util.JsonEncodeToByte(subscribeMap)
		util.Info(`ws order ` + string(wsOrderMsg))
		err := sendToWs(model.OKEX+`_`+key, wsOrderMsg)
		if err != nil {
			util.Notice(fmt.Sprintf(`fail to send order ws %s %s return %s`, key, order.Instrument, err.Error()))
		}
	} else {
		var responseBody []byte
		responseBody = sendSignRequestOKEX(key, secret, http.MethodPost, path, postData)
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
				}
				if value[`sMsg`] != nil {
					order.ErrCode = value[`sMsg`].(string)
				}
				if value[`ordId`] != nil {
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
func getMarketsOKEX() (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	instTypes := []string{`MARGIN`, `SWAP`}
	for _, instType := range instTypes {
		path := fmt.Sprintf(`/api/v5/public/instruments?%s`,
			util.ComposeParams(map[string]interface{}{`instType`: instType}))
		responseBody := sendSignRequestOKEX(``, ``, http.MethodGet, path, nil)
		resultJson, err := util.NewJSON(responseBody)
		if err == nil && resultJson != nil && resultJson.Get(`data`) != nil {
			for _, info := range resultJson.Get(`data`).MustArray() {
				value := info.(map[string]interface{})
				if value[`instId`] != nil {
					marketInfo := &model.MarketInfo{Name: value[`instId`].(string), CanBorrow: false}
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
					marketInfos[marketInfo.Name] = marketInfo
					//if instType == `SWAP` && strings.Contains(marketInfo.Name, `-USDT-SWAP`) {
					//	symbol := strings.Split(marketInfo.Name, `-`)[0] + `-USDT`
					//	if marketInfos[symbol] != nil {
					//		fmt.Println(marketInfo.Name)
					//	} else {
					//		fmt.Println(`do not have ` + marketInfo.Name)
					//	}
					//}
				}
			}
		}
	}
	return
}

// 暂不支持策略订单
func cancelOrdersOKEX(key, secret, instrument string) (result bool, code, msg string) {
	orders := queryPendingOrdersOKEX(key, secret, instrument)
	if len(orders) <= 0 {
		return true, ``, ``
	}
	postData := make([]map[string]string, len(orders))
	for i, order := range orders {
		postData[i] = map[string]string{`instId`: instrument, `ordId`: order.OrderId}
	}
	responseBody := sendSignRequestOKEX(key, secret, http.MethodPost, "/api/v5/trade/cancel-batch-orders", postData)
	resultJson, err := util.NewJSON(responseBody)
	if err == nil && resultJson != nil {
		code = resultJson.Get(`code`).MustString()
		if code == `0` {
			return true, code, resultJson.Get(`msg`).MustString()
		}
	}
	return false, code, ``
}

func cancelOrderOkex(key, secret, instrument string, orderId, orderType string) (result bool, errCode, msg string) {
	postData := map[string]interface{}{`instId`: instrument}
	var responseBody []byte
	if orderType == model.OrderTypeStop {
		postData[`algoId`] = orderId
		data := []interface{}{postData}
		responseBody = sendSignRequestOKEX(key, secret, http.MethodPost, `/api/v5/trade/cancel-algos`, data)
	} else {
		postData[`ordId`] = orderId
		responseBody = sendSignRequestOKEX(key, secret, http.MethodPost, "/api/v5/trade/cancel-order", postData)
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
			} else if value[`algoId`] != nil && value[`algoId`].(string) == orderId && value[`sCode`].(string) == `0` {
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
	order = &model.Order{Market: model.OKEX}
	if value[`avgPx`] != nil && value[`avgPx`] != `` {
		order.DealPrice, _ = strconv.ParseFloat(value[`avgPx`].(string), 64)
	}
	if value[`channelKey`] != nil {
		order.AmountType = value[`channelKey`].(string)
	}
	if value[`instId`] != nil {
		order.Instrument = value[`instId`].(string)
		order.Symbol = value[`instId`].(string)
	}
	if value[`ordId`] != nil {
		order.OrderId = value[`ordId`].(string)
	} else if value[`algoId`] != nil {
		order.OrderId = value[`algoId`].(string)
	}
	if value[`px`] != nil && value[`px`] != `` {
		order.Price, _ = strconv.ParseFloat(value[`px`].(string), 64)
	}
	//if value[`ordType`] != nil { // market：市价单 limit：限价单 post_only：只做maker单 fok：全部成交或立即取消 ioc：立即成交并取消剩余
	//	order.OrderType = value[`ordType`].(string)
	//}
	if value[`side`] != nil {
		order.OrderSide = value[`side`].(string)
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
	if value[`tag`] != nil {
		order.RefreshType = value[`tag`].(string)
	}
	if strings.Contains(order.Instrument, `SWAP`) || len(strings.Split(order.Instrument, `-`)) > 2 {
		_, order.Amount = ParseRealAmount(model.OKEX, order.Instrument, order.Amount)
		_, order.DealAmount = ParseRealAmount(model.OKEX, order.Instrument, order.DealAmount)
	}
	return order
}

// 暂不支持策略订单
func queryPendingOrdersOKEX(key, secret, instrument string) (orders []*model.Order) {
	path := fmt.Sprintf("/api/v5/trade/orders-pending?%s",
		util.ComposeParams(map[string]interface{}{`instId`: instrument}))
	responseBody := sendSignRequestOKEX(key, secret, http.MethodGet, path, nil)
	orderJson, err := util.NewJSON(responseBody)
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

func queryOrderOKEX(key, secret, instrument, orderId, orderType string) (order *model.Order) {
	path := fmt.Sprintf(`/api/v5/trade/order?%s`,
		util.ComposeParams(map[string]interface{}{"ordId": orderId, "instId": instrument}))
	if orderType == model.OrderTypeStop {
		path = fmt.Sprintf(`/api/v5/trade/orders-algo-pending?algoId=%s&ordType=conditional`, orderId)
	}
	responseBody := sendSignRequestOKEX(key, secret, http.MethodGet, path, nil)
	orderJson, err := util.NewJSON(responseBody)
	if err != nil || orderJson == nil || orderJson.Get(`data`) == nil || orderJson.Get(`code`) == nil {
		return nil
	}
	if strings.Trim(orderJson.Get(`code`).MustString(), ` `) == `51603` {
		return &model.Order{Instrument: instrument, OrderId: orderId, OrderType: orderType,
			Status: model.CarryStatusFail, Symbol: instrument}
	}
	orders := orderJson.Get("data").MustArray()
	for _, item := range orders {
		value := item.(map[string]interface{})
		if orderType != model.OrderTypeStop {
			if value[`ordId`] != nil && value[`ordId`].(string) == orderId {
				order = parseOrderOKEX(value)
				break
			}
		} else {
			if value[`ordId`] != nil {
				ordId := strings.Trim(value[`ordId`].(string), ` `)
				if ordId != `` && ordId != `0` {
					orderType = model.OrderTypeLimit
					return queryOrderOKEX(key, secret, instrument, ordId, orderType)
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
	response = sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/asset/withdrawal-history`, nil)
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

func parsePositionOKEX(value map[string]interface{}) (success bool, position *model.Position) {
	position = &model.Position{Market: model.OKEX}
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
		position.Currency = value[`instId`].(string)
	}
	//posCcy 仓位资产币种，仅适用于币币杠杆仓位
	if value[`pos`] != nil {
		pos, _ := strconv.ParseFloat(value[`pos`].(string), 64)
		if strings.Contains(position.Currency, `SWAP`) || len(strings.Split(position.Currency, `-`)) > 2 {
			success, position.Free = ParseRealAmount(model.OKEX, position.Currency, pos)
		} else {
			position.Free = pos
		}
	}
	//pos 持仓数量
	return
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
	if value[`ts`] != nil && value[`ts`] != `` {
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
		switch strings.Trim(value[`state`].(string), ` `) {
		case `-3`, `0`, `1`, `3`, `4`, `5`:
			balance.Status = model.CarryStatusWorking
		case `-2`, `-1`:
			balance.Status = model.CarryStatusFail
		case `2`:
			balance.Status = model.CarryStatusSuccess
		}
	}
	// for balance
	if value[`availEq`] != nil && value[`availEq`] != `` {
		balance.Available, _ = strconv.ParseFloat(value[`availEq`].(string), 64)
	}
	if value[`eq`] != nil && value[`eq`] != `` {
		balance.Amount, _ = strconv.ParseFloat(value[`eq`].(string), 64)
	}
	if value[`disEq`] != nil && value[`disEq`] != `` {
		balance.UsdValue, _ = strconv.ParseFloat(value[`disEq`].(string), 64)
		if balance.UsdValue == 0 && balance.Amount > 0 {
			success, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+`-USDT`, model.OKEX)
			if success {
				balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
			}
		}
	}
	if value[`crossLiab`] != nil && value[`crossLiab`] != `` {
		balance.Borrow, _ = strconv.ParseFloat(value[`crossLiab`].(string), 64)
	}
	//balance.AvailableWithBorrow = balance.Available
	return
}

// margin: 可用保证金
func getBalanceOKEX(key, secret string) (success bool, balances []*model.Balance, totalInUsd, margin float64) {
	response := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/account/balance`, nil)
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson == nil || responseJson.GetPath(`data`) == nil ||
		responseJson.Get(`data`).MustArray() == nil ||
		len(responseJson.Get(`data`).MustArray()) == 0 ||
		responseJson.Get(`data`).MustArray()[0].(map[string]interface{})[`details`] == nil {
		util.SocketInfo(`fail to get okex balance `)
		time.Sleep(time.Second * 2)
		return getBalanceOKEX(key, secret)
	}
	balances = make([]*model.Balance, 0)
	if responseJson.Get(`code`).MustString() == `0` {
		success = true
	}
	data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
	if data[`totalEq`] != nil {
		totalInUsd, _ = strconv.ParseFloat(data[`totalEq`].(string), 64)
	}
	if data[`adjEq`] != nil && data[`imr`] != nil {
		marginAll, _ := strconv.ParseFloat(data[`adjEq`].(string), 64)    // 可用保证金
		marginOccupied, _ := strconv.ParseFloat(data[`imr`].(string), 64) // 被占用保证金
		margin = marginAll - marginOccupied
	}
	for _, item := range data[`details`].([]interface{}) {
		balance := parseBalanceOKEX(item.(map[string]interface{}))
		balances = append(balances, balance)
	}
	return success, balances, totalInUsd, margin
}

func getAccountConfigOKEX(key, secret string) (mode string) {
	response := sendSignRequestOKEX(key, secret, http.MethodGet, `/api/v5/account/config`, nil)
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson.Get(`data`) == nil || len(responseJson.Get(`data`).MustArray()) == 0 {
		return ``
	}
	data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
	return data[`posMode`].(string)
}

func setAccountModeOKEX(key, secret string) (success bool) {
	response := sendSignRequestOKEX(key, secret, http.MethodPost, `/api/v5/account/set-position-mode`,
		map[string]interface{}{`posMode`: `net_mode`})
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson.Get(`code`).MustString() != `0` {
		return false
	}
	return true
}

func getLastPriceOKEX(key, secret, instrument string) (price float64) {
	path := fmt.Sprintf(`/api/v5/market/ticker?instId=%s`, instrument)
	response := sendSignRequestOKEX(key, secret, http.MethodGet, path, nil)
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

// 目前只支持永续 swap
func getPositionsOKEX(key, secret string) (success bool, positions []*model.Position) {
	path := fmt.Sprintf("/api/v5/account/positions?%s",
		util.ComposeParams(map[string]interface{}{"instType": `SWAP`}))
	responseBody := sendSignRequestOKEX(key, secret, http.MethodGet, path, nil)
	responseJson, err := util.NewJSON(responseBody)
	if err != nil || responseJson == nil || responseJson.Get(`data`) == nil {
		util.SocketInfo(`fail to get okex positions `)
		time.Sleep(time.Second * 2)
		return getPositionsOKEX(key, secret)
	}
	if responseJson.Get(`code`).MustString() == `0` {
		success = true
	}
	positions = make([]*model.Position, 0)
	positionArray := responseJson.Get(`data`).MustArray()
	for _, item := range positionArray {
		result, position := parsePositionOKEX(item.(map[string]interface{}))
		if result {
			positions = append(positions, position)
		} else {
			success = false
		}
	}
	return success, positions
}

func GetMaxSize(key, secret, instrument string) (success bool, maxBuy, maxSell float64) {
	response := sendSignRequestOKEX(key, secret, http.MethodGet,
		fmt.Sprintf(`/api/v5/account/max-size?instId=%s&tdMode=cross`, instrument), nil)
	responseJson, err := util.NewJSON(response)
	if responseJson == nil || err != nil || responseJson.Get(`data`) == nil ||
		responseJson.Get(`data`).MustArray() == nil || len(responseJson.Get(`data`).MustArray()) == 0 {
		return false, 0, 0
	}
	data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
	if data[`instId`] != nil && data[`instId`].(string) == instrument {
		if data[`maxBuy`] != nil {
			maxBuy, _ = strconv.ParseFloat(data[`maxBuy`].(string), 64)
		}
		if data[`maxSell`] != nil {
			maxSell, _ = strconv.ParseFloat(data[`maxSell`].(string), 64)
		}
	}
	return true, maxBuy, maxSell
}

func getFundingRateOKEX(key, secret, instrumentId string) (fundingRate *model.FundingRate) {
	path := fmt.Sprintf(`/api/v5/public/funding-rate?instId=%s`, instrumentId)
	response := sendSignRequestOKEX(key, secret, http.MethodGet, path, nil)
	fundingJson, _ := util.NewJSON(response)
	if fundingJson == nil || fundingJson.Get(`data`) == nil || fundingJson.Get(`data`).MustArray() == nil ||
		len(fundingJson.Get(`data`).MustArray()) == 0 {
		return nil
	}
	data := fundingJson.Get(`data`).MustArray()[0].(map[string]interface{})
	if data[`instId`] == instrumentId {
		rate, _ := strconv.ParseFloat(data[`fundingRate`].(string), 64)
		rateNext, _ := strconv.ParseFloat(data[`nextFundingRate`].(string), 64)
		rateTime, _ := strconv.ParseInt(data[`fundingTime`].(string), 10, 64)
		rateTime /= 1000
		return &model.FundingRate{
			FundingTime: time.Time{},
			Rate:        rate,
			RateNext:    rateNext,
			UpdateTime:  util.GetNow().Unix(),
			ExpireTime:  rateTime,
			Symbol:      instrumentId,
		}
	}
	return nil
}

func getMaxLoanOKEX(key, secret, coin string) (success bool, maxLoan float64) {
	path := fmt.Sprintf(`/api/v5/account/max-loan?instId=%s-USDT&mgnMode=cross`, coin)
	response := sendSignRequestOKEX(key, secret, http.MethodGet, path, nil)
	loanJson, err := util.NewJSON(response)
	succes := false
	if loanJson == nil || err != nil || loanJson.Get(`data`) == nil {
		return false, 0
	}
	value := loanJson.Get(`data`).MustArray()
	for _, item := range value {
		data := item.(map[string]interface{})
		if data[`maxLoan`] != nil && data[`ccy`] != nil && data[`ccy`].(string) == coin {
			maxLoan, _ = strconv.ParseFloat(data[`maxLoan`].(string), 64)
			succes = true
		}
	}
	return succes, maxLoan
}

// bar 1m/3m/5m/15m/30m/1H/2H/4H/6H/12H/1D/1W/1M/3M/6M/1Y
func getCandlesOKEX(key, secret, symbol, binSize string, before, after time.Time, count int) (
	candles map[string]*model.Candle) {
	candles = make(map[string]*model.Candle)
	path := fmt.Sprintf(`/api/v5/market/candles?instId=%s&bar=%s&before=%d&after=%d&limit=%d`,
		symbol, binSize, before.UnixNano()/int64(time.Millisecond), after.UnixNano()/int64(time.Millisecond), count)
	response := sendSignRequestOKEX(key, secret, http.MethodGet, path, nil)
	candleJson, err := util.NewJSON(response)
	if err != nil || candleJson == nil || candleJson.Get(`data`) == nil || len(candleJson.Get(`data`).MustArray()) == 0 {
		return
	}
	candleJsons := candleJson.Get(`data`).MustArray()
	location, _ := time.LoadLocation("Asia/Shanghai")
	for _, value := range candleJsons {
		item := value.([]interface{})
		if len(item) < 7 {
			continue
		}
		candle := &model.Candle{Market: model.OKEX, Symbol: symbol, Period: strings.ToLower(binSize)}
		ts, _ := strconv.ParseInt(item[0].(string), 10, 64)
		candle.UTCDate = time.Unix(ts/1000, 0).In(location).Format(time.RFC3339)[:10]
		candle.PriceOpen, _ = strconv.ParseFloat(item[1].(string), 64)
		candle.PriceHigh, _ = strconv.ParseFloat(item[2].(string), 64)
		candle.PriceLow, _ = strconv.ParseFloat(item[3].(string), 64)
		candle.PriceClose, _ = strconv.ParseFloat(item[4].(string), 64)
		candles[candle.UTCDate] = candle
	}
	return
}

package deprecated

//
//import (
//	"fmt"
//	"github.com/bitly/go-simplejson"
//	"github.com/gorilla/websocket"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"net/http"
//	"strconv"
//	"strings"
//	"time"
//)
//
//const nodeServer = `http://localhost:3000`
//
////const restDFuture = `https://openoracle_prod_heco.dfuture.com/dev/web`
//const wsDFuture = `wss://heco_prod_kline_wss.dfuture.com/ws`
//const wsStepDFuture = 50
//
//var lastDepthPingDFuture = util.GetNowUnixMillion()
//
//var subscribeHandlerDFuture = func(connection *websocket.conn, subscribes []interface{}) error {
//	var err error = nil
//	for _, subscribe := range subscribes {
//		subMsg := fmt.Sprintf(`{"id": "id1", "includeDfutureDay": "1", sub:"%s"}`, subscribe)
//		if err = api.SendToConnection(model.DFuture, connection, []byte(subMsg)); err != nil {
//			util.SocketInfo("dfuture can not subscribe fill " + err.Error())
//		}
//	}
//	return err
//}
//
//func WsDepthServeDFuture(markets *model.Markets, orderHandler api.OrderHandler) ([]chan struct{}, error) {
//	wsHandler := func(connection *websocket.conn, event []byte, orderHandler api.OrderHandler) {
//		responseJson, err := util.NewJSON(event)
//		if err != nil {
//			util.SocketInfo(`fail to unmarshal DFuture json ` + err.Error())
//			return
//		}
//		if responseJson == nil {
//			return
//		}
//		if !strings.Contains(string(event), `202`) {
//			fmt.Println(string(event))
//		}
//		if util.GetNowUnixMillion()-lastDepthPingDFuture > 15000 {
//			lastDepthPingDFuture = util.GetNowUnixMillion()
//			ts := time.Now().UnixNano() / int64(time.Second)
//			pingMsg := fmt.Sprintf(
//				`{"verify":1,"apiTime":%d,"deviceInfo":"326d7f7cfe5c0421cbfb50a1dc7e839f","token":"3f938b62ba748d891c48a6060bf8875b"}`, ts)
//			if err := api.SendToAllConnections(model.DFuture, []byte(pingMsg)); err != nil {
//				util.SocketInfo("dfuture server ping client error " + err.Error())
//			}
//		}
//		handleTickerDFuture(markets, responseJson)
//	}
//	subType := model.SubscribeDepth + `,` + model.SubscribeTicker
//	return api.WebSocketClient(model.DFuture, wsDFuture, api.GetWSSubscribes(model.DFuture, subType),
//		subscribeHandlerDFuture, wsHandler, orderHandler, wsStepDFuture)
//}
//
//func handleTickerDFuture(markets *model.Markets, response *simplejson.Json) {
//	if response == nil {
//		return
//	}
//	code := response.Get("code").MustInt64()
//	if code != 202 || response.GetPath(`data`, `tick`, `close`) == nil {
//		util.SocketInfo(`DFuture ws msg not 202`)
//		return
//	}
//	symbol := response.GetPath(`data`, `ch`).MustString()
//	chs := strings.Split(symbol, `.`)
//	if len(chs) > 2 {
//		symbol = chs[2]
//	}
//	price := response.GetPath(`data`, `tick`, `close`).MustFloat64()
//	bidAsk := &model.BidAsk{
//		Ts:         int(util.GetNowUnixMillion()),
//		TsReceived: int(util.GetNowUnixMillion()),
//		Bids:       []model.Tick{{Price: 0, Amount: 0, Market: model.DFuture, Symbol: symbol, Side: model.OrderSideBuy}},
//		Asks:       []model.Tick{{Price: price, Amount: 0, Market: model.DFuture, Symbol: symbol, Side: model.OrderSideSell}},
//	}
//	if markets.SetBidAsk(symbol, model.DFuture, bidAsk) {
//		for function, handler := range model.GetFunctions(model.DFuture, symbol) {
//			if handler != nil {
//				setting := model.GetSetting(function, model.DFuture, symbol)
//				if setting != nil {
//					go handler(setting, bidAsk)
//				}
//			}
//		}
//	}
//}
//
////
////func sendSignRequestDFuture(key, secret, method, path string, param, body map[string]interface{}) ([]byte){
////	if key == `` || secret == `` {
////		keys, secrets := model.AppConfig.GetKeys(model.DFuture)
////		key = keys[0]
////		secret = secrets[0]
////	}
////	if body == nil {
////		body = make(map[string]interface{})
////	}
////	timeStr := strconv.FormatInt(util.GetNowUnixMillion(), 10)
////	toBeSign := fmt.Sprintf(`%s&%s&%s`, key, timeStr, secret)
////	hash := hmac.New(md5.New, []byte(secret))
////	hash.WriteServe([]byte(toBeSign))
////	sign := hex.EncodeToString(hash.Sum(nil))
////	headers := map[string]string{`accessKey`: key, `accessTime`: timeStr, "token": sign, "Content-Type": "application/json"}
////	uri := model.AppConfig.RestUrls[model.DFuture] + path
////	bodyStr := string(util.JsonEncodeToByte(body))
////	responseBody, _ := util.HttpRequest(method, uri, bodyStr, headers, 60)
////	util.SocketInfo(fmt.Sprintf(`dfuture key %s request %s body %s return %s`,
////		key, uri, bodyStr, string(responseBody)))
////	return responseBody
////}
//
//func getAmountFromDFuture(symbol string, input float64) (output float64) {
//	switch symbol {
//	case `btc`:
//		return input * 0.001
//	case `eth`:
//		return input * 0.01
//	}
//	return 0
//}
//
//func getAmountToDFuture(symbol string, input float64) (output float64) {
//	switch symbol {
//	case `btc`:
//		return input / 0.001
//	case `eth`:
//		return input / 0.01
//	}
//	return 0
//}
//
//func sendRequestDFuture(key, secret, method, path string, body interface{}) (responseBody []byte) {
//	uri := nodeServer + path
//	headers := map[string]string{"Content-Type": "application/json"}
//	postContent := ``
//	if method == http.MethodPost {
//		postContent = string(util.JsonEncodeToByte(body))
//	}
//	responseBody, _ = util.HttpRequest(method, uri, postContent, headers, 60)
//	util.SocketInfo(fmt.Sprintf(`%s %s request %s body %s return %s`, key, secret, uri, postContent, string(responseBody)))
//	return responseBody
//}
//
//func openDFuture(key, secret, orderSide, symbol string, price, acceptablePrice, amount float64) (success bool) {
//	direction := 1
//	if orderSide == model.OrderSideBuy {
//		direction = 1
//	} else if orderSide == model.OrderSideSell {
//		direction = -1
//	}
//	approvedUsdt := math.Ceil(amount * price / 15)
//	amount = getAmountToDFuture(symbol, amount)
//	data := map[string]interface{}{"symbol": symbol, "direction": direction, "amount": amount,
//		"acceptable_price": int64(math.Round(acceptablePrice)), "approveUsdt": approvedUsdt, "access_key": key, "access_sk": secret,
//		"account_address": model.AppConfig.FutureAddress, "private_key": model.AppConfig.WalletKey, "gas_level": 5}
//	responseBody := sendRequestDFuture(key, secret, http.MethodPost, `/open`, data)
//	responseJson, _ := util.NewJSON(responseBody)
//	if responseJson != nil && responseJson.Get(`code`).MustInt() == 0 {
//		success = true
//	}
//	return success
//}
//
//func closeDFuture(key, secret, symbol string, price, acceptablePrice, amount float64) (success bool) {
//	approvedUsdt := math.Ceil(amount * price / 15)
//	amount = getAmountToDFuture(symbol, amount)
//	data := map[string]interface{}{"symbol": symbol, "amount": amount, "acceptable_price": int64(math.Round(acceptablePrice)),
//		"approveUsdt": approvedUsdt, "access_key": key, "access_sk": secret,
//		"account_address": model.AppConfig.FutureAddress, "private_key": model.AppConfig.WalletKey, "gas_level": 5}
//	responseBody := sendRequestDFuture(key, secret, http.MethodPost, `/close`, data)
//	responseJson, _ := util.NewJSON(responseBody)
//	if responseJson != nil && responseJson.Get(`code`).MustInt() == 0 {
//		success = true
//	}
//	return success
//}
//
//func getPositionsDFuture(symbol, address string) (success bool, position *model.Position) {
//	path := fmt.Sprintf(`%s/query?symbol=%s&account_address=%s`, nodeServer, symbol, address)
//	responseBody, _ := util.HttpRequest(http.MethodGet, path, ``, nil, 60)
//	responseJson, _ := util.NewJSON(responseBody)
//	if responseJson != nil && responseJson.Get(`code`).MustInt() == 0 {
//		success = true
//		value := responseJson.GetPath(`data`, `positionInfo`).MustArray()
//		if value != nil && len(value) > 3 {
//			amount, _ := strconv.ParseFloat(value[0].(string), 64)
//			amount = getAmountFromDFuture(symbol, amount)
//			if value[3].(string) == `-1` {
//				amount = -1 * amount
//			}
//			position = &model.Position{Market: model.DFuture, Currency: symbol, Holding: amount}
//		}
//	}
//	util.SocketInfo(fmt.Sprintf(`dfuture query %s return %#v`, path, success))
//	return success, position
//}

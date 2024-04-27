package deprecated

//
//import (
//	"crypto/hmac"
//	"crypto/md5"
//	"encoding/hex"
//	"encoding/json"
//	"fmt"
//	"github.com/gorilla/websocket"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"net/url"
//	"sort"
//	"strconv"
//	"strings"
//)
//
//const restCoinPark = "https://api.coinpark.cc/v1"
//const wsCoinPark = "wss://push.coinpark.cc/"
//
//var subscribeHandlerCoinpark = func(connection *websocket.Conn, subscribes []interface{}) error {
//	var err error = nil
//	for _, v := range subscribes {
//		subscribeMap := make(map[string]interface{})
//		subscribeMap["event"] = "addChannel"
//		subscribeMap["channel"] = v
//		subscribeMap[`binary`] = 0
//		subscribeMessage := util.JsonEncodeToByte(subscribeMap)
//		if err = api.SendToConnection(model.Coinpark, connection, subscribeMessage); err != nil {
//			util.SocketInfo("coinPark can not subscribe " + err.Error())
//			return err
//		}
//	}
//	return err
//}
//
//func WsDepthServeCoinpark(markets *model.Markets, orderHandler api.OrderHandler) ([]chan struct{}, error) {
//	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler api.OrderHandler) {
//		depthJson, err := util.NewJSON(event)
//		if err != nil {
//			util.SocketInfo(`fail to unmarshal coinpark json ` + err.Error())
//			return
//		}
//		if depthJson == nil {
//			return
//		}
//		depthArray, err := depthJson.Array()
//		if err == nil && len(depthArray) > 0 {
//			data := depthArray[0].(map[string]interface{})[`data`].(map[string]interface{})
//			if data != nil {
//				if data[`pair`] == nil {
//					return
//				}
//				symbol := strings.ToLower(data[`pair`].(string))
//				time, _ := data[`update_time`].(json.Number).Int64()
//				bidAsk := model.BidAsk{Ts: int(time), TsReceived: int(util.GetNowUnixMillion())}
//				askArray := data[`asks`].([]interface{})
//				bidArray := data[`bids`].([]interface{})
//				bidAsk.Asks = make([]model.Tick, len(askArray))
//				bidAsk.Bids = make([]model.Tick, len(bidArray))
//				for i, value := range bidArray {
//					str := value.(map[string]interface{})["price"].(string)
//					price, _ := strconv.ParseFloat(str, 64)
//					str = value.(map[string]interface{})["volume"].(string)
//					amount, _ := strconv.ParseFloat(str, 64)
//					bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: model.Coinpark, Symbol: symbol, Side: model.OrderSideBuy}
//				}
//				for i, value := range askArray {
//					str := value.(map[string]interface{})["price"].(string)
//					price, _ := strconv.ParseFloat(str, 64)
//					str = value.(map[string]interface{})["volume"].(string)
//					amount, _ := strconv.ParseFloat(str, 64)
//					bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: model.Coinpark, Symbol: symbol, Side: model.OrderSideSell}
//				}
//				sort.Sort(bidAsk.Asks)
//				sort.Sort(sort.Reverse(bidAsk.Bids))
//				if markets.SetBidAsk(symbol, model.Coinpark, &bidAsk) {
//					for function, handler := range model.GetFunctions(model.Coinpark, symbol) {
//						setting := model.GetSetting(function, model.Coinpark, symbol)
//						if setting != nil {
//							go handler(setting, &bidAsk)
//						}
//					}
//				}
//			}
//		}
//	}
//	subscribes := api.GetWSSubscribes(model.Coinpark, model.SubscribeDepth)
//	connectionNum := int(math.Ceil(float64(len(subscribes)) / 50))
//	return api.WebSocketClient(model.Coinpark, wsCoinPark, subscribes, subscribeHandlerCoinpark, wsHandler, orderHandler, connectionNum)
//}
//
//func SignedRequestCoinpark(key, secret, method, path, cmds string) []byte {
//	hash := hmac.New(md5.New, []byte(secret))
//	hash.WriteServe([]byte(cmds))
//	sign := hex.EncodeToString(hash.Sum(nil))
//	postData := &url.Values{}
//	postData.Set("cmds", cmds)
//	postData.Set("apikey", key)
//	postData.Set("sign", sign)
//	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded",
//		"User-Agent": "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.71 Safari/537.36"}
//	responseBody, _ := util.HttpRequest(method, restCoinPark+path,
//		postData.Encode(), headers, 60)
//	return responseBody
//}
//
//// order_side 交易方向，1-买，2-卖
//// order_type 交易类型，2-限价单
//func placeOrderCoinpark(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
//	if orderSide == model.OrderSideBuy {
//		orderSide = `1`
//	} else if orderSide == model.OrderSideSell {
//		orderSide = `2`
//		if amount > 50000 {
//			util.Notice(`%s ==sell==do not execute %f`, orderType, amount)
//		}
//	} else {
//		util.Notice(fmt.Sprintf(`[parameter error] order side: %s`, orderSide))
//	}
//	if orderType == model.OrderTypeLimit {
//		orderType = `2`
//	} else {
//		orderType = `2`
//		util.Info(fmt.Sprintf(`[parameter error] order type: %s`, orderType))
//	}
//	symbol = strings.ToUpper(symbol)
//	cmds := fmt.Sprintf(`[{"cmd":"orderpending/trade",
//		"body":{"pair":"%s","account_type":0,"order_type":%s,"order_side":"%s","price":%f,"amount":"%f"}}]`,
//		symbol, orderType, orderSide, price, amount)
//	responseBody := SignedRequestCoinpark(key, secret, `POST`, `/orderpending`, cmds)
//	util.Notice(cmds + `[place order]` + string(responseBody))
//	orderJson, _ := util.NewJSON(responseBody)
//	if orderJson == nil {
//		return
//	}
//	if orderJson.Get(`result`) != nil {
//		results, err := orderJson.Get("result").Array()
//		if err == nil && len(results) > 0 {
//			resultData := results[0].(map[string]interface{})["result"]
//			if resultData != nil {
//				str, _ := resultData.(json.Number).Int64()
//				order.OrderId = strconv.FormatInt(str, 10)
//			}
//		}
//	}
//	errorJson := orderJson.Get(`error`)
//	if errorJson.Get(`error`) != nil {
//		errorCodeJson := errorJson.Get(`code`)
//		if errorCodeJson != nil {
//			order.ErrCode, _ = errorCodeJson.String()
//		}
//	}
//}
//
////dealPrice 返回委托价格，市价单是0
//func queryOrderCoinpark(key, secret, orderId string) (dealAmount, dealPrice float64, status string) {
//	cmds := fmt.Sprintf(`[{"cmd":"orderpending/order","body":{"id":"%s"}}]`, orderId)
//	responseBody := SignedRequestCoinpark(key, secret, `POST`, `/orderpending`, cmds)
//	orderJson, err := util.NewJSON(responseBody)
//	util.Notice(string(responseBody))
//	if orderJson == nil {
//		return
//	}
//	results, err := orderJson.Get("result").Array()
//	if err == nil && len(results) > 0 {
//		resultData := results[0].(map[string]interface{})[`result`]
//		if resultData != nil {
//			strDealAmount := resultData.(map[string]interface{})[`deal_amount`].(string)
//			if strDealAmount != "" {
//				dealAmount, _ = strconv.ParseFloat(strDealAmount, 64)
//			}
//			strDealPrice := resultData.(map[string]interface{})[`price`].(string)
//			if strDealPrice != `` {
//				dealPrice, _ = strconv.ParseFloat(strDealPrice, 64)
//			}
//			intStatus, _ := resultData.(map[string]interface{})[`status`].(json.Number).Int64()
//			status = model.GetOrderStatus(model.Coinpark, fmt.Sprintf(`%s%d`, model.Coinpark, intStatus))
//		}
//	}
//	return dealAmount, dealPrice, status
//}
//
//func cancelOrderCoinpark(key, secret, orderId string) (result bool, code, msg string) {
//	cmds := fmt.Sprintf(`[{"cmd":"orderpending/cancelTrade","body":{"orders_id":"%s"}}]`, orderId)
//	responseBody := SignedRequestCoinpark(key, secret, `POST`, `/orderpending`, cmds)
//	util.Notice(orderId + `[cancel order]` + string(responseBody))
//	if strings.TrimSpace(string(responseBody)) == `` {
//		return
//	}
//	orderJson, _ := util.NewJSON(responseBody)
//	if orderJson == nil {
//		util.Notice(`no result in response coinpark ` + orderId)
//		return false, "", ""
//	}
//	orderJson = orderJson.Get("result")
//	if orderJson == nil {
//		util.Notice(`no result in response coinpark ` + orderId)
//		return false, "", ""
//	}
//	results, err := orderJson.Array()
//	if err == nil && len(results) > 0 {
//		errorData := results[0].(map[string]interface{})[`error`]
//		resultData := results[0].(map[string]interface{})["result"]
//		if resultData != nil {
//			return true, ``, resultData.(string)
//		}
//		if errorData != nil {
//			code = errorData.(map[string]interface{})[`code`].(string)
//			msg = errorData.(map[string]interface{})[`msg`].(string)
//			return false, code, msg
//		}
//	}
//	if err != nil {
//		return false, err.Error(), err.Error()
//	}
//	return false, ``, ``
//}

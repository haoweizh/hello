package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hello/model"
	"hello/util"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const restHuobi = `api-aws.huobi.pro`
const wsHuobi = `wss://api-aws.huobi.pro/ws`
const restHuobiDM = `api.hbdm.com`
const wsHuobiDM = `wss://api.hbdm.vn/linear-swap-ws`

//spot：现货账户, margin：逐仓杠杆账户, otc：OTC 账户, point：点卡账户, super-margin：全仓杠杆账户, investment: C2C杠杆借出账户,
//borrow: C2C杠杆借入账户，矿池账户: minepool, ETF账户: etf, 抵押借贷账户: crypto-loans
const spotAccount = "spot"

var huobiAccountMap = make(map[string]map[string]string) //key-type-accountId

type HuobiMessage struct {
	Ping   int    `json:"ping"`
	Ch     string `json:"ch"`
	Ts     int    `json:"ts"`
	Req    string `json:"req"`
	Rep    string `json:"rep"`
	Status string `json:"status"`
	Id     string `json:"id"`
	Tick   struct {
		SeqNum float64     `json:"seqNum"`
		Amount float64     `json:"amount"` // 成交量
		Count  int         `json:"count"`  // 成交笔数
		Open   float64     `json:"open"`   // 开盘价
		Close  float64     `json:"close"`  // 收盘价,当K线为最晚的一根时，是最新成交价
		Low    float64     `json:"low"`    // 最低价
		High   float64     `json:"high"`   // 最高价
		Vol    float64     `json:"vol"`    // 成交额, 即 sum(每一笔成交价 * 该笔的成交量)
		Bids   [][]float64 `json:"bids"`
		Asks   [][]float64 `json:"asks"`
	} `json:"tick"`
}

var subscribeHandlerHuobi = func(subscribes []interface{}, keyChannel string) error {
	var err error = nil
	for _, v := range subscribes {
		subscribeMap := make(map[string]interface{})
		subscribeMap["id"] = strconv.Itoa(util.GetNow().Nanosecond())
		subscribeMap["sub"] = v
		subscribeMessage := util.JsonEncodeToByte(subscribeMap)
		if err = sendToWs(keyChannel, subscribeMessage); err != nil {
			util.SocketInfo(" huobi can not subscribe %s %s %s", keyChannel, v, err.Error())
			//util.SocketInfo("huobi can not subscribe " + err.Error())
			//return err
		}
		util.Notice(`huobi subscribed ` + string(subscribeMessage))
	}
	return err
}

func WsDepthServeHuobi(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	wsHandler := func(channelKey string, event []byte, orderHandler OrderHandler) {
		res := util.UnGzip(event)
		responseJson, _ := util.NewJSON(res)
		if responseJson.Get(`ping`).MustInt() > 0 {
			pingMap := make(map[string]interface{})
			pingMap["pong"] = responseJson.Get(`ping`).MustInt()
			pingParams := util.JsonEncodeToByte(pingMap)
			if err := sendToWs(model.Huobi, pingParams); err != nil {
				util.SocketInfo("huobi server ping client error " + err.Error())
			}
		} else {
			tickJson := responseJson.Get(`tick`)
			if tickJson.Interface() == nil {
				return
			}

			now := int(time.Now().UnixNano() / int64(time.Millisecond))
			symbol := model.GetSymbol(model.Huobi, tickJson.Get("symbol").MustString())
			if symbol != "" {
				symbol = strings.ReplaceAll(symbol, "_", "")

				bidAsk := model.BidAsk{Ts: responseJson.Get("ts").MustInt(), TsReceived: now, UpdateId: tickJson.Get("quoteTime").MustInt64(),
					Bids: []model.Tick{{Price: tickJson.Get("bid").MustFloat64(), Amount: tickJson.Get("bidSize").MustFloat64()}},
					Asks: []model.Tick{{Price: tickJson.Get("ask").MustFloat64(), Amount: tickJson.Get("askSize").MustFloat64()}}}

				haveOld, old := markets.GetBidAsk(symbol, model.Huobi)
				if haveOld && old.UpdateId > bidAsk.UpdateId {
					return
				}

				if markets.SetBidAsk(symbol, model.Huobi, &bidAsk) {
					for function, handler := range model.GetFunctions(model.Huobi, symbol) {
						if handler != nil {
							settings := model.GetSetting(function, model.Huobi, symbol)
							for _, setting := range settings {
								go handler(setting, &bidAsk)
							}
						}
					}
				}
			}
		}
	}

	wsHandlerDM := func(channelKey string, event []byte, orderHandler OrderHandler) {
		res := util.UnGzip(event)
		responseJson, _ := util.NewJSON(res)
		if responseJson.Get(`ping`).MustInt() > 0 {
			pingMap := make(map[string]interface{})
			pingMap["pong"] = responseJson.Get(`ping`).MustInt()
			pingParams := util.JsonEncodeToByte(pingMap)
			if err := sendToWs(model.HuobiDM, pingParams); err != nil {
				util.SocketInfo("huobiDM server ping client error " + err.Error())
			}
		} else {
			tickJson := responseJson.Get(`tick`)
			if tickJson.Interface() == nil {
				return
			}
			symbol := responseJson.Get(`ch`).MustString()
			strs := strings.Split(symbol, `.`)
			if strs == nil || len(strs) <= 1 {
				return
			}
			symbol = strings.ToLower(strs[1])
			now := int(time.Now().UnixNano() / int64(time.Millisecond))

			bidAsk := model.BidAsk{Ts: responseJson.Get(`ts`).MustInt(), TsReceived: now, UpdateId: tickJson.Get("ts").MustInt64()}
			bid := tickJson.Get(`bid`).MustArray()
			ask := tickJson.Get(`ask`).MustArray()
			bidAsk.Bids = make([]model.Tick, 1)
			bidAsk.Asks = make([]model.Tick, 1)

			if bid == nil || len(bid) < 2 || ask == nil || len(ask) < 2 {
				return
			}

			bidAmount, _ := bid[1].(json.Number).Float64()
			bidPrice, _ := bid[0].(json.Number).Float64()
			bidSuccess, bidAmount := ParseRealAmount(model.HuobiDM, symbol, bidAmount)
			if !bidSuccess {
				return
			}

			askAmount, _ := ask[1].(json.Number).Float64()
			askPrice, _ := ask[0].(json.Number).Float64()
			askSuccess, askAmount := ParseRealAmount(model.HuobiDM, symbol, askAmount)
			if !askSuccess {
				return
			}

			bidAsk.Bids = []model.Tick{{Price: bidPrice, Amount: bidAmount}}
			bidAsk.Asks = []model.Tick{{Price: askPrice, Amount: askAmount}}

			haveOld, old := markets.GetBidAsk(symbol, model.HuobiDM)
			if haveOld && old.UpdateId > bidAsk.UpdateId {
				return
			}

			if markets.SetBidAsk(symbol, model.HuobiDM, &bidAsk) {
				for function, handler := range model.GetFunctions(model.HuobiDM, symbol) {
					if handler != nil {
						settings := model.GetSetting(function, model.HuobiDM, symbol)
						for _, setting := range settings {
							go handler(setting, &bidAsk)
						}
					}
				}
			}
		}
	}

	channels = make([]chan struct{}, 0)
	channel, channelErr := WebSocketClient(model.Huobi, wsHuobi, model.Huobi,
		GetWSSubscribes(model.Huobi, model.SubscribeTicker), subscribeHandlerHuobi, wsHandler, orderHandler)
	if channelErr != nil {
		util.SocketInfo(`fail to create huobi conn %s`, channelErr.Error())
	} else {
		channels = append(channels, channel)
	}

	dmChannel, dmChannelErr := WebSocketClient(model.HuobiDM, wsHuobiDM, model.HuobiDM,
		GetWSSubscribes(model.HuobiDM, model.SubscribeTicker), subscribeHandlerHuobi, wsHandlerDM, orderHandler)
	if dmChannelErr != nil {
		util.SocketInfo(`fail to create huobidm %s`, dmChannelErr.Error())
	} else {
		channels = append(channels, dmChannel)
	}

	return channels, err
}

func getMarketsHuobi() (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	requestUrls := []string{restHuobi + `/v1/common/symbols`, restHuobiDM + `/linear-swap-api/v1/swap_contract_info`}

	for _, requestUrl := range requestUrls {
		responseBody := SignedRequestHuobi(``, ``, http.MethodGet, requestUrl, nil)
		symbolsJson, err := util.NewJSON(responseBody)

		if err != nil || symbolsJson == nil || strings.ToLower(symbolsJson.Get(`status`).MustString()) != `ok` {
			continue
		}

		items, _ := symbolsJson.Get("data").Array()
		for _, item := range items {
			value := item.(map[string]interface{})

			if value[`contract_code`] != nil { //合约市场
				marketInfo := &model.MarketInfo{Name: strings.ToLower(value["contract_code"].(string))}

				if value["symbol"] != nil {
					marketInfo.CTCurrency = strings.ToLower(value["symbol"].(string))
				}
				if value["contract_size"] != nil {
					marketInfo.CTValue, _ = value["contract_size"].(json.Number).Float64()
				}
				if value["price_tick"] != nil {
					marketInfo.PriceIncrement, _ = value["price_tick"].(json.Number).Float64()
				}
				marketInfo.SizeIncrement = 1

				marketInfos[marketInfo.Name] = marketInfo
			} else { //现货市场
				if value["symbol"] == nil || value["api-trading"].(string) == "disabled" || value["quote-currency"].(string) != "usdt" {
					continue
				}
				marketInfo := &model.MarketInfo{Name: value["symbol"].(string)}
				if value["price-precision"] != nil {
					priceDecimal, _ := value["price-precision"].(json.Number).Int64()
					marketInfo.PriceDecimal = int(priceDecimal)
				}
				if value["limit-order-min-order-amt"] != nil {
					marketInfo.SizeMin, _ = value["limit-order-min-order-amt"].(json.Number).Float64()
				}

				//price-precision	true	integer	交易对报价的精度（小数点后位数）
				//amount-precision	true	integer	交易对基础币种计数精度（小数点后位数）

				marketInfos[marketInfo.Name] = marketInfo
			}
		}
	}

	return marketInfos
}

func SignedRequestHuobi(key, secret, method, restUrl string, data map[string]interface{}) []byte {
	if key == `` || secret == `` {
		keys, secrets := model.AppConfig.GetKeys(model.Huobi)
		key = keys[0]
		secret = secrets[0]
	}
	param := map[string]interface{}{"AccessKeyId": key, "SignatureMethod": "HmacSHA256",
		"SignatureVersion": "2", `Timestamp`: url.QueryEscape(time.Now().UTC().Format("2006-01-02T15:04:05"))}
	strData := ``
	if method == `GET` {
		for i, value := range data {
			param[i] = value
		}
	} else if method == `POST` && data != nil {
		strData = string(util.JsonEncodeToByte(data))
	}
	strParam := util.ComposeParams(param)
	toBeSign := fmt.Sprintf("%s\n%s\n%s", method, restUrl, strParam)
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(toBeSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(hash.Sum(nil)))
	param["Signature"] = sign
	requestUrl := fmt.Sprintf(`https://%s?%s`, restUrl, util.ComposeParams(param))
	headers := map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn",
		"User-Agent": "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.71 Safari/537.36"}
	responseBody, _ := util.HttpRequest(method, requestUrl, strData, headers, 60)
	util.SocketInfo(fmt.Sprintf(`%s %s %s`, requestUrl, strData, string(responseBody)))
	return responseBody
}

func GetAccountIdsHuobi(key, secret string) (err error) {
	responseBody := SignedRequestHuobi(key, secret, `GET`, restHuobi+"/v1/account/accounts", nil)
	util.SocketInfo(`get huobi accounts: ` + string(responseBody))
	accountJson, err := util.NewJSON(responseBody)
	if err == nil {
		accounts, _ := accountJson.Get("data").Array()
		for _, value := range accounts {
			account := value.(map[string]interface{})
			typeName := account["type"].(string)
			//accountIds[typeName] = account["id"].(json.Number).String()

			if huobiAccountMap[key] == nil {
				huobiAccountMap[key] = make(map[string]string)
			}
			if huobiAccountMap[key][typeName] == "" {
				huobiAccountMap[key][typeName] = account["id"].(json.Number).String()
			}
		}
	}
	return err
}

// orderType: buy-market：市价买, sell-market：市价卖, buy-limit：限价买, sell-limit：限价卖
// huobi中amount在市价买单中指的是右侧的钱
func placeOrderHuobi(key, secret string, order *model.Order, orderSide, orderType, symbol, price, amount string) {
	orderParam := ``
	if orderSide == model.OrderSideBuy && orderType == model.OrderTypeLimit {
		orderParam = `buy-limit`
	} else if orderSide == model.OrderSideBuy && orderType == model.OrderTypeMarket {
		orderParam = `buy-market`
	} else if orderSide == model.OrderSideSell && orderType == model.OrderTypeLimit {
		orderParam = `sell-limit`
	} else if orderSide == model.OrderSideSell && orderType == model.OrderTypeMarket {
		orderParam = `sell-market`
	} else {
		util.Notice(fmt.Sprintf(`[parameter error] order side: %s order type: %s`, orderSide, orderType))
	}

	if huobiAccountMap[key] == nil || huobiAccountMap[key][spotAccount] == "" {
		_ = GetAccountIdsHuobi(key, secret)
	}

	path := "/v1/order/orders/place"
	postData := make(map[string]interface{})
	postData["account-id"] = huobiAccountMap[key][spotAccount]
	postData["amount"] = amount
	postData["symbol"] = strings.ToLower(strings.Replace(symbol, "_", "", 1))
	postData["type"] = orderParam
	if orderType == model.OrderTypeLimit {
		postData["price"] = price
	}
	responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobi+path, postData)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		status, _ := orderJson.Get("status").String()
		if status == "ok" {
			order.OrderId, _ = orderJson.Get("data").String()
		} else if status == "error" {
			order.OrderId, _ = orderJson.Get("err-code").String()
		}
	}
	util.Notice(fmt.Sprintf(`[挂单huobi] %s side: %s type: %s price: %s amount: %s order id %s 返回%s`,
		symbol, orderSide, orderType, price, amount, order.OrderId, string(responseBody)))
}

func cancelOrderHuobi(key, secret, orderId string) (result bool, errCode, msg string) {
	path := fmt.Sprintf("/v1/order/orders/%s/submitcancel", orderId)
	responseBody := SignedRequestHuobi(key, secret, `POST`, restHuobi+path, nil)
	orderJson, err := util.NewJSON(responseBody)
	util.Notice("huobi cancel order" + orderId + string(responseBody))
	if err == nil {
		status, _ := orderJson.Get("status").String()
		if status == "ok" {
			return true, ``, ``
		} else if status == "error" {
			errCode, _ = orderJson.Get("err-code").String()
			msg, _ = orderJson.Get(`err-msg`).String()
			return false, errCode, msg
		}
	} else {
		return false, err.Error(), err.Error()
	}
	return false, ``, ``
}

func queryOrderHuobi(key, secret, orderId string) (dealAmount, dealPrice float64, status string) {
	path := fmt.Sprintf("/v1/order/orders/%s", orderId)
	responseBody := SignedRequestHuobi(key, secret, `GET`, restHuobi+path, nil)
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		status, _ = orderJson.GetPath("data", "state").String()
		status = model.GetOrderStatus(model.Huobi, status)
		str, _ := orderJson.GetPath("data", "field-amount").String()
		if str != "" {
			dealAmount, _ = strconv.ParseFloat(str, 64)
		}
		strDealPrice, _ := orderJson.GetPath(`data`, `price`).String()
		if strDealPrice != `` {
			dealPrice, _ = strconv.ParseFloat(strDealPrice, 64)
		}
	}
	util.Notice(fmt.Sprintf("%s huobi query order %f %s", status, dealAmount, responseBody))
	return dealAmount, dealPrice, status
}

func parseBalanceHuobi(key string, data map[string]interface{}, market string) (balance *model.Balance) {
	if data == nil || data[`id`] == nil {
		return nil
	}
	balance = &model.Balance{AccountId: key, Market: market}
	balance.ID = model.Huobi + `_` + data[`id`].(json.Number).String()
	if data[`type`] != nil {
		if data[`type`].(string) == `deposit` {
			balance.Action = 1
		} else if data[`type`].(string) == `withdraw` {
			balance.Action = -1
		}
	}
	if data[`currency`] != nil {
		balance.Coin = strings.ToLower(data[`currency`].(string))
	}
	if data[`amount`] != nil {
		balance.Amount, _ = data[`amount`].(json.Number).Float64()
	}
	if data[`address`] != nil {
		balance.Address, _ = data[`address`].(string)
	}
	if data[`fee`] != nil {
		balance.Fee = data[`fee`].(json.Number).String()
	}
	if data[`state`] != nil {
		balance.Status, _ = data[`state`].(string)
	}
	if data[`updated-at`] != nil {
		seconds, _ := data[`updated-at`].(json.Number).Int64()
		balance.BalanceTime = time.Unix(seconds/1000, 0)
		fmt.Println(balance.BalanceTime.String())
	}
	return balance
}

func getTransferHuobi(key, secret string) (balances []*model.Balance) {
	data := map[string]interface{}{`type`: `deposit`}
	response := SignedRequestHuobi(key, secret, http.MethodGet, restHuobi+`/v1/query/deposit-withdraw`, data)
	util.SocketInfo(`query huobi deposit: ` + string(response))
	responseJson, err := util.NewJSON(response)
	if err == nil && responseJson != nil && responseJson.Get(`data`) != nil {
		items := responseJson.Get(`data`).MustArray()
		for _, item := range items {
			balance := parseBalanceHuobi(key, item.(map[string]interface{}), model.Huobi)
			if balance != nil {
				balances = append(balances, balance)
			}
		}
	}
	data = map[string]interface{}{`type`: `withdraw`}
	response = SignedRequestHuobi(key, secret, http.MethodGet, restHuobi+`/v1/query/deposit-withdraw`, data)
	util.SocketInfo(`query huobi withdraw: ` + string(response))
	responseJson, err = util.NewJSON(response)
	if err == nil && responseJson != nil && responseJson.Get(`data`) != nil {
		items := responseJson.Get(`data`).MustArray()
		for _, item := range items {
			balance := parseBalanceHuobi(key, item.(map[string]interface{}), model.Huobi)
			if balances != nil {
				balances = append(balances, balance)
			}
		}
	}
	return balances
}

func getPositionsHuobi(key string, secret string) (success bool, positions []*model.Position, posBalance float64) {
	response := SignedRequestHuobi(key, secret, http.MethodGet, restHuobi+"/linear-swap-api/v1/swap_cross_account_position_info", nil)
	responseJson, err := util.NewJSON(response)
	if err != nil || responseJson == nil || strings.ToLower(responseJson.Get(`status`).MustString()) != `ok` {
		time.Sleep(time.Second * 2)
		util.SocketInfo(`fail to get huobiDM balance`)
		return getPositionsHuobi(key, secret)
	}
	positions = make([]*model.Position, 0)
	items := responseJson.Get(`data`).MustArray()
	posBalance = responseJson.Get(`margin_balance`).MustFloat64()

}

// 资产账户 getBalanceHuobi
func getBalanceHuobi(key string, secret string) (success bool, balances []*model.Balance) {
	if huobiAccountMap[key] == nil || huobiAccountMap[key][spotAccount] == "" {
		_ = GetAccountIdsHuobi(key, secret)
	}
	balances = make([]*model.Balance, 0)
	accountId := huobiAccountMap[key][spotAccount]
	path := fmt.Sprintf("/v1/account/accounts/%s/balance", accountId)
	response := SignedRequestHuobi(key, secret, http.MethodGet, restHuobi+path, nil)
	responseJson, err := util.NewJSON(response)
	if err == nil {
		balanceArray := responseJson.GetPath(`data`, `list`).MustArray()

		balanceMap := make(map[string]*model.Balance)
		for _, item := range balanceArray {
			value := item.(map[string]interface{})

			//trade: 交易余额，frozen: 冻结余额, loan: 待还借贷本金, interest: 待还借贷利息, lock: 锁仓, bank: 储蓄
			if (value["type"] != "trade" && value["type"] != "lock") || (value[`currency`] == nil) {
				continue
			}

			balance := &model.Balance{}
			coin := value[`currency`].(string)
			if balanceMap[coin] != nil {
				balance = balanceMap[coin]
			} else {
				balance.AccountId = accountId
				balance.BalanceTime = util.GetNow()
				balance.Market = model.Huobi
				balance.Coin = coin
			}
			if value[`type`] != nil && value[`balance`] != nil && value["type"] == "trade" {
				balance.Available, _ = strconv.ParseFloat(value[`balance`].(string), 64)
			}
			if value[`type`] != nil && value[`balance`] != nil && value["type"] == "lock" {
				balance.LockAmount, _ = strconv.ParseFloat(value[`balance`].(string), 64)
			}
			balance.Amount = balance.Available + balance.LockAmount

			//if value[`currency`] != nil {
			//	balance.Coin = value[`currency`].(string)
			//}
			//if value[`type`] != nil {
			//	balance.Status = value[`type`].(string)
			//}
			//if value[`balance`] != nil {
			//	balance.Amount, _ = strconv.ParseFloat(value[`balance`].(string), 64)
			//}
			if balance.Amount > 0 {
				balance.ID = fmt.Sprintf(`%s_%s_%s_%s`,
					balance.Market, balance.Coin, balance.Status, balance.BalanceTime.String()[0:10])
				balances = append(balances, balance)
			}
		}
	} else {
		time.Sleep(time.Second * 2)
		util.SocketInfo(`fail to refresh balance huobi`)
		return getBalanceHuobi(key, secret)
	}
	return
}

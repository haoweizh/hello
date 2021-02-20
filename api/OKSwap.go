package api

import (
	"bytes"
	"compress/flate"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/bitly/go-simplejson"
	"hello/model"
	"hello/util"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

//永续合约API

var subscribeHandlerOKSwap = func(subscribes []interface{}, subType string) error {
	keys, secrets := model.AppConfig.GetKeys(model.OKSwap)
	var err error = nil
	step := 5
	epoch := time.Now().UnixNano() / int64(time.Millisecond)
	timestamp := fmt.Sprintf(`%d.%d`, epoch/1000, epoch%1000)
	toBeSign := fmt.Sprintf(`%s%s%s`, timestamp, `GET`, `/users/self/verify`)
	hash := hmac.New(sha256.New, []byte(secrets[0]))
	hash.Write([]byte(toBeSign))
	sign := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	authCmd := fmt.Sprintf(`{"op":"login","args":["%s","%s","%s","%s"]}`, keys[0], `OKSwap`, timestamp, sign)
	if err = sendToWs(model.OKSwap, []byte(authCmd)); err != nil {
		util.SocketInfo("okswap can not auth " + err.Error())
	}
	stepSubscribes := make([]interface{}, 0)
	for i := 0; i*step < len(subscribes); i++ {
		subscribeMap := make(map[string]interface{})
		subscribeMap[`op`] = `subscribe`
		if (i+1)*step < len(subscribes) {
			stepSubscribes = subscribes[i*step : (i+1)*step]
		} else {
			stepSubscribes = subscribes[i*step:]
		}
		subscribeMap[`args`] = stepSubscribes
		subscribeMessage := util.JsonEncodeMapToByte(subscribeMap)
		if err = sendToWs(model.OKSwap, subscribeMessage); err != nil {
			util.SocketInfo("okswap can not subscribe " + err.Error())
			return err
		}
	}
	return err
}

func WsDepthServeOKSwap(markets *model.Markets, errHandler ErrHandler) (chan struct{}, error) {
	lastPingTime := util.GetNow().Unix()
	wsHandler := func(event []byte) {
		if util.GetNow().Unix()-lastPingTime > 30 { // ping ws server every 30 seconds
			lastPingTime = util.GetNow().Unix()
			if err := sendToWs(model.OKSwap, []byte(`ping`)); err != nil {
				util.SocketInfo("okex server ping client error " + err.Error())
			}
		}
		if len(event) == 0 {
			return
		}
		//messages := make([]OKEXMessage, 1)
		var out bytes.Buffer
		reader := flate.NewReader(bytes.NewReader(event))
		_, _ = io.Copy(&out, reader)
		event = out.Bytes()
		depthJson, depthErr := util.NewJSON(event)
		if depthErr != nil {
			if string(event) != `pong` {
				util.Notice(string(event) + `okswap depth event error ` + depthErr.Error())
			}
			return
		}
		table := depthJson.Get(`table`).MustString()
		if table == `swap/depth5` {
			handleDepthOkSwap(markets, depthJson.Get(`data`))
		} else if strings.Contains(table, `swap/position`) {
			handlePositionOKSwap(depthJson.Get(`data`))
			util.SocketInfo(fmt.Sprintf(`position get : %s`, string(event)))
		}
	}
	return WebSocketServe(model.OKSwap, model.AppConfig.WSUrls[model.OKSwap], model.SubscribeDepth,
		GetWSSubscribes(model.OKSwap, model.SubscribeDepth),
		subscribeHandlerOKSwap, wsHandler, errHandler)
}

func parseAccountOKSwap(account *model.Account, item map[string]interface{}) {
	if item[`liquidation_price`] != nil {
		account.LiquidationPrice, _ = strconv.ParseFloat(item[`liquidation_price`].(string), 64)
	}
	if item[`avail_position`] != nil && item[`side`] != nil {
		side := strings.ToLower(item[`side`].(string))
		if side == `long` {
			account.Direction = model.OrderSideBuy
		} else if side == `short` {
			account.Direction = model.OrderSideSell
		}
		free, err := strconv.ParseFloat(item[`avail_position`].(string), 64)
		if err == nil {
			if model.OrderSideSell == account.Direction {
				account.Free = math.Abs(free) * -1
			} else {
				account.Free = free
			}
			account.Free = 100 * account.Free
		}
	}
	if item[`avg_cost`] != nil {
		account.EntryPrice, _ = strconv.ParseFloat(item[`avg_cost`].(string), 64)
	}
	if item[`margin`] != nil {
		account.Margin, _ = strconv.ParseFloat(item[`margin`].(string), 64)
	}
	if item[`settled_pnl`] != nil {
		account.ProfitReal, _ = strconv.ParseFloat(item[`settled_pnl`].(string), 64)
	}
}

func handlePositionOKSwap(response *simplejson.Json) {
	if response == nil {
		return
	}
	data := response.MustArray()
	if data != nil && len(data) > 0 {
		for _, value := range data {
			if value != nil {
				item := value.(map[string]interface{})
				if item[`instrument_id`] != nil && item[`timestamp`] != nil {
					currency := model.GetStandardSymbol(model.OKSwap, item[`instrument_id`].(string))
					timestamp, _ := time.Parse(time.RFC3339, item[`timestamp`].(string))
					ts := timestamp.UnixNano() / 1000000
					if item[`holding`] != nil {
						holdings := item[`holding`].([]interface{})
						for _, holding := range holdings {
							account := &model.Account{Market: model.OKSwap, Ts: ts, Currency: currency}
							parseAccountOKSwap(account, holding.(map[string]interface{}))
							model.AppAccounts.SetAccount(model.OKSwap, account.Currency, account)
						}
					}
				}
			}
		}
	}
}

func handleDepthOkSwap(markets *model.Markets, response *simplejson.Json) {
	if response == nil {
		return
	}
	data := response.MustArray()
	if data != nil && len(data) > 0 {
		bidAsk := &model.BidAsk{}
		bidAsk.Bids = make([]model.Tick, 0)
		bidAsk.Asks = make([]model.Tick, 0)
		value := data[0].(map[string]interface{})
		if value[`instrument_id`] != nil && value[`timestamp`] != nil {
			symbol := model.GetStandardSymbol(model.OKSwap, value[`instrument_id`].(string))
			ts, err := time.Parse(time.RFC3339, value[`timestamp`].(string))
			if err == nil {
				bidAsk.Ts = int(ts.UnixNano()) / 1000000
			}
			if value[`bids`] != nil {
				for _, items := range value[`bids`].([]interface{}) {
					item := items.([]interface{})
					if item != nil && len(item) == 4 {
						tick := model.Tick{Symbol: symbol, Side: model.OrderSideBuy}
						tick.Price, _ = strconv.ParseFloat(item[0].(string), 64)
						tick.Amount, _ = strconv.ParseFloat(item[1].(string), 64)
						tick.Amount = tick.Amount * 100
						bidAsk.Bids = append(bidAsk.Bids, tick)
					}
				}
			}
			if value[`asks`] != nil {
				for _, items := range value[`asks`].([]interface{}) {
					item := items.([]interface{})
					if item != nil && len(item) == 4 {
						tick := model.Tick{Symbol: symbol, Side: model.OrderSideSell}
						tick.Price, _ = strconv.ParseFloat(item[0].(string), 64)
						tick.Amount, _ = strconv.ParseFloat(item[1].(string), 64)
						tick.Amount = tick.Amount * 100
						bidAsk.Asks = append(bidAsk.Asks, tick)
					}
				}
			}
			sort.Sort(bidAsk.Asks)
			sort.Sort(sort.Reverse(bidAsk.Bids))
			//util.SocketInfo(markets.ToStringBidAsk(bidAsk))
			if markets.SetBidAsk(symbol, model.OKSwap, bidAsk) {
				for function, handler := range model.GetFunctions(model.OKSwap, symbol) {
					if handler != nil {
						util.Notice(`handling by okswap`)
						settings := model.GetSetting(function, model.OKSwap, symbol)
						for _, setting := range settings {
							handler(setting)
						}
					}
				}
			}
		}
	}
}

func SignedRequestOKSwap(key, secret, method, path string, body map[string]interface{}) []byte {
	if key == `` || secret == `` {
		keys, secrets := model.AppConfig.GetKeys(model.OKSwap)
		key = keys[0]
		secret = secrets[0]
	}
	if body == nil {
		body = make(map[string]interface{})
	}
	uri := model.AppConfig.RestUrls[model.OKSwap] + path
	epoch := time.Now().UnixNano() / int64(time.Millisecond)
	timestamp := fmt.Sprintf(`%d.%d`, epoch/1000, epoch%1000)
	toBeSign := fmt.Sprintf(`%s%s%s`, timestamp, method, path)
	headers := map[string]string{`OK-ACCESS-KEY`: key, `OK-ACCESS-PASSPHRASE`: model.AppConfig.Phase,
		"OK-ACCESS-TIMESTAMP": timestamp}
	if method == `POST` {
		toBeSign = toBeSign + string(util.JsonEncodeMapToByte(body))
		headers["Content-Type"] = "application/json"
	}
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(toBeSign))
	sign := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	headers[`OK-ACCESS-SIGN`] = sign
	responseBody, _ := util.HttpRequest(method, uri, string(util.JsonEncodeMapToByte(body)), headers, 60)
	return responseBody
}

func getAccountOKSwap(key, secret, symbol string, accounts *model.Accounts) (success bool) {
	response := SignedRequestOKSwap(key, secret, `GET`,
		fmt.Sprintf(`/api/swap/v3/%s/position`, model.GetDialectSymbol(model.OKSwap, symbol)), nil)
	util.SocketInfo(fmt.Sprintf(`get account rest:%s`, string(response)))
	positionJson, err := util.NewJSON(response)
	if err != nil || positionJson == nil || positionJson.Get(`holding`) == nil {
		time.Sleep(time.Second * 2)
		util.SocketInfo(`fail to refresh accounts okswap`)
		return getAccountOKSwap(key, secret, symbol, accounts)
	}
	positionJson = positionJson.Get(`holding`)
	if positionJson != nil {
		for _, item := range positionJson.MustArray() {
			account := &model.Account{Market: model.OKSwap}
			parseAccountOKSwap(account, item.(map[string]interface{}))
			account.Currency = symbol
			accounts.SetAccount(model.OKSwap, account.Currency, account)
		}
	}
	return true
}

func getFundingRateOKSwap(symbol string) (fundingRate float64, expire int64) {
	response := SignedRequestOKSwap(``, ``, `GET`,
		fmt.Sprintf(`/api/swap/v3/instruments/%s/funding_time`,
			model.GetDialectSymbol(model.OKSwap, symbol)), nil)
	instrumentJson, err := util.NewJSON(response)
	if err == nil {
		data := instrumentJson.MustMap()
		if data != nil {
			if data["funding_rate"] != nil {
				fundingRate, _ = strconv.ParseFloat(data["funding_rate"].(string), 64)
			}
			if data["funding_time"] != nil {
				fundingTime, _ := time.Parse(time.RFC3339, data["funding_time"].(string))
				expire = fundingTime.Unix()
			}
		}
	}
	return
}

// type 1开多 2开空 3平多 4平空
// order_type 0：普通委托 1：只做Maker（Post only） 2：全部成交或立即取消（FOK） 3：立即成交并取消剩余（IOC）
func placeOrderOKSwap(order *model.Order, key, secret, orderSide, orderType, symbol, price,
	amount string) {
	postData := make(map[string]interface{})
	postData[`size`] = amount
	postData[`type`] = orderSide
	postData["order_type"] = orderType
	postData["instrument_id"] = model.GetDialectSymbol(model.OKSwap, symbol)
	postData[`price`] = price
	response := SignedRequestOKSwap(key, secret, `POST`, `/api/swap/v3/order`, postData)
	util.Notice(`place okswap` + string(response))
	orderJson, err := util.NewJSON(response)
	if err == nil {
		errCode := orderJson.Get(`error_code`).MustString()
		if errCode == "0" {
			order.OrderId = orderJson.Get(`order_id`).MustString()
		} else {
			order.ErrCode = errCode
			order.Status = model.CarryStatusFail
		}
	}
	return
}

func parseOrderOKSwap(order *model.Order, item map[string]interface{}) {
	if order == nil {
		return
	}
	if item[`order_id`] != nil {
		order.OrderId = item[`order_id`].(string)
	}
	if item["instrument_id"] != nil {
		order.Symbol = model.GetStandardSymbol(model.OKSwap, item["instrument_id"].(string))
	}
	if item[`size`] != nil {
		order.Amount, _ = strconv.ParseFloat(item[`size`].(string), 64)
		order.Amount = order.Amount * 100
	}
	if item[`timestamp`] != nil {
		order.OrderTime, _ = time.Parse(time.RFC3339, item[`timestamp`].(string))
	}
	if item["filled_qty"] != nil {
		order.DealAmount, _ = strconv.ParseFloat(item["filled_qty"].(string), 64)
		order.DealAmount = order.DealAmount * 100
	}
	if item[`fee`] != nil {
		order.Fee, _ = strconv.ParseFloat(item[`fee`].(string), 64)
	}
	if item[`price`] != nil {
		order.Price, _ = strconv.ParseFloat(item[`price`].(string), 64)
	}
	if item[`price_avg`] != nil {
		order.DealPrice, _ = strconv.ParseFloat(item[`price_avg`].(string), 64)
	}
	if item[`order_type`] != nil {
		order.OrderType = item[`order_type`].(string)
	}
	if item[`type`] != nil {
		if item[`type`] == `1` || item[`type`] == `4` {
			order.OrderSide = model.OrderSideBuy
		} else if item[`type`] == `2` || item[`type`] == `3` {
			order.OrderSide = model.OrderSideSell
		}
	}
	order.Status = model.CarryStatusWorking
	if item[`state`] != nil {
		order.Status = model.GetOrderStatus(model.OKSwap, item[`state`].(string))
	}
	if order.DealAmount > 0 && order.DealPrice == 0 {
		order.DealPrice = order.Price
	}
	return
}

func queryOrderOKSwap(key, secret, symbol, orderId string) (order *model.Order) {
	response := SignedRequestOKSwap(key, secret, `GET`,
		fmt.Sprintf(`/api/swap/v3/orders/%s/%s`, model.GetDialectSymbol(model.OKSwap, symbol), orderId), nil)
	util.Notice(`query order OKSwap: ` + string(response))
	orderJson, err := util.NewJSON(response)
	if err == nil {
		data := orderJson.MustMap()
		order = &model.Order{Market: model.OKSwap, Symbol: symbol, OrderId: orderId}
		parseOrderOKSwap(order, data)
	}
	return
}

func cancelOrderOKSwap(key, secret, symbol, orderId string) (result bool) {
	response := SignedRequestOKSwap(key, secret, `POST`,
		fmt.Sprintf(`/api/swap/v3/cancel_order/%s/%s`,
			model.GetDialectSymbol(model.OKSwap, symbol), orderId), nil)
	orderJson, err := util.NewJSON(response)
	util.Notice(fmt.Sprintf(`okswap cancel order %s return %s`, orderId, string(response)))
	result = false
	if err == nil {
		errCode := orderJson.Get(`error_code`).MustString()
		if errCode == "0" {
			result = true
		} else {
			result = false
		}
	}
	return
}

func _(key, secret string) (balance map[string]float64) {
	response := SignedRequestOKSwap(key, secret, `GET`, `/api/swap/v3/accounts`, nil)
	orderJson, err := util.NewJSON(response)
	util.Notice(`okswap wallet: ` + string(response))
	if err == nil {
		balance = make(map[string]float64)
		items := orderJson.Get(`info`).MustArray()
		for _, item := range items {
			wallet := item.(map[string]interface{})
			if wallet == nil {
				continue
			}
			if wallet[`instrument_id`] != nil && wallet[`equity`] != nil {
				symbol := model.GetStandardSymbol(model.OKSwap, wallet[`instrument_id`].(string))
				balance[symbol], _ =
					strconv.ParseFloat(wallet[`equity`].(string), 64)
			}
		}
	}
	return
}

func parseTransferAmount(response []byte) (info string) {
	transferJson, err := util.NewJSON(response)
	if err == nil {
		items := transferJson.MustArray()
		for _, item := range items {
			amount := 0.0
			jsonAmount := item.(map[string]interface{})[`amount`]
			if jsonAmount != nil {
				amount, _ = strconv.ParseFloat(jsonAmount.(string), 64)
			}
			jsonTime := item.(map[string]interface{})[`timestamp`]
			if jsonTime != nil {
				info += fmt.Sprintf("%s %f\n", jsonTime.(string), amount)
			}
		}
	}
	return
}

func parseBalanceOK(key string, data map[string]interface{}, market string) (balance *model.Balance) {
	balance = &model.Balance{AccountId: key, Market: market}
	if data[`deposit_id`] != nil {
		balance.ID = model.OKEX + `_` + data[`deposit_id`].(string)
		balance.Action = 1
		if data[`from`] != nil {
			balance.Address, _ = data[`from`].(string)
		}
	} else if data[`withdrawal_id`] != nil {
		balance.ID = model.OKEX + `_` + data[`withdrawal_id`].(string)
		balance.Action = -1
		if data[`to`] != nil {
			balance.Address, _ = data[`to`].(string)
		}
	} else {
		return nil
	}
	if data[`currency`] != nil {
		balance.Coin = strings.ToLower(data[`currency`].(string))
	}
	if data[`amount`] != nil {
		balance.Amount, _ = strconv.ParseFloat(data[`amount`].(string), 64)
	}
	if data[`txid`] != nil {
		balance.TransactionId, _ = data[`txid`].(string)
	}
	if data[`fee`] != nil {
		balance.Fee, _ = data[`fee`].(string)
	}
	if data[`status`] != nil {
		balance.Status, _ = data[`status`].(string)
	}
	if data[`timestamp`] != nil {
		balance.BalanceTime, _ = time.Parse(time.RFC3339, data[`timestamp`].(string))
		fmt.Println(balance.BalanceTime.String())
	}
	return balance
}

//GetWalletHistoryOKSwap
func _(key, secret, symbol string) (info string) {
	postData := make(map[string]interface{})
	postData[`type`] = `5`
	symbol = model.GetDialectSymbol(model.OKSwap, symbol)
	response := SignedRequestOKSwap(key, secret, `GET`,
		fmt.Sprintf(`/api/swap/v3/accounts/%s/ledger?type=5`, symbol), postData)
	util.Notice(`okswap wallet history 5: ` + string(response))
	info = parseTransferAmount(response)
	response = SignedRequestOKSwap(key, secret, `GET`,
		fmt.Sprintf(`/api/swap/v3/accounts/%s/ledger?type=6`, symbol), postData)
	util.Notice(`okswap wallet history 6: ` + string(response))
	info += parseTransferAmount(response)
	return
}

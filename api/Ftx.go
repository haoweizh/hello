package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"hello/model"
	"hello/util"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var lastDepthPingFtx = util.GetNowUnixMillion()
var socketLockFtx sync.Mutex

var subscribeHandlerFtx = func(subscribes []interface{}, subType string) error {
	var err error = nil
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	toBeSign := fmt.Sprintf(`%dwebsocket_login`, ts)
	hash := hmac.New(sha256.New, []byte(model.AppConfig.FtxSecret))
	hash.Write([]byte(toBeSign))
	sign := hex.EncodeToString(hash.Sum(nil))
	authCmd := fmt.Sprintf(`{"op":"login","args":{"key":"%s","sign":"%s","time":%d}}`,
		model.AppConfig.FtxKey, sign, ts)
	if err = sendToWs(model.Ftx, []byte(authCmd)); err != nil {
		util.SocketInfo("ftx can not auth " + err.Error())
	}
	if err = sendToWs(model.Ftx, []byte(`{"op": "subscribe", "channel": "fills"}`)); err != nil {
		util.SocketInfo("ftx can not subscribe fill " + err.Error())
	}
	for i := 0; i < len(subscribes); i++ {
		cmdSubscribe := subscribes[i].([]string)
		subCmd := fmt.Sprintf(`{"op": "subscribe", "channel": "%s", "market": "%s"}`,
			cmdSubscribe[0], cmdSubscribe[1])
		if err = sendToWs(model.Ftx, []byte(subCmd)); err != nil {
			util.SocketInfo("ftx can not subscribe " + err.Error())
			return err
		}
		util.Notice(`send subscribe ` + subCmd)
	}
	return err
}

func WsDepthServeFtx(markets *model.Markets, errHandler ErrHandler) (chan struct{}, error) {
	wsHandler := func(event []byte) {
		defer socketLockFtx.Unlock()
		socketLockFtx.Lock()
		responseJson, err := util.NewJSON(event)
		if err != nil {
			errHandler(err)
			return
		}
		if responseJson == nil {
			return
		}
		if util.GetNowUnixMillion()-lastDepthPingFtx > 15000 {
			lastDepthPingFtx = util.GetNowUnixMillion()
			pingMsg := []byte(`{"op":"ping"}`)
			if err := sendToWs(model.Ftx, pingMsg); err != nil {
				util.SocketInfo("ftx server ping client error " + err.Error())
			}
		}
		msgType := responseJson.Get(`channel`).MustString()
		if msgType == `orderbook` {
			handleDepthFtx(markets, responseJson)
		} else if msgType == `ticker` {
			handleTickerFtx(markets, responseJson)
		}
	}
	requestUrl := model.AppConfig.WSUrls[model.Ftx]
	//subType := model.SubscribeDepth + `,` + model.SubscribeDeal
	return WebSocketServe(model.Ftx, requestUrl, ``, GetWSSubscribes(model.Ftx, ``),
		subscribeHandlerFtx, wsHandler, errHandler)
}

func handleTickerFtx(markets *model.Markets, response *simplejson.Json) {
	if response == nil {
		return
	}
	symbol := response.Get("market").MustString()
	dataType := response.Get(`type`).MustString()
	data := response.Get(`data`)
	if data == nil || data.Get(`bid`) == nil || data.Get(`ask`) == nil || data.Get(`bidSize`) == nil ||
		data.Get(`askSize`) == nil {
		return
	}
	bidAsk := &model.BidAsk{}
	ts := data.Get(`time`).MustFloat64()
	bidAsk.Ts = int(ts * 1000)
	if dataType == `update` {
		bidAsk.Bids = []model.Tick{{Price: data.Get(`bid`).MustFloat64(), Amount: data.Get(`bidSize`).MustFloat64()}}
		bidAsk.Asks = []model.Tick{{Price: data.Get(`ask`).MustFloat64(), Amount: data.Get(`askSize`).MustFloat64()}}
	}
	if markets.SetBidAsk(symbol, model.Ftx, bidAsk) {
		for function, handler := range model.GetFunctions(model.Ftx, symbol) {
			if handler != nil {
				settings := model.GetSetting(function, model.Ftx, symbol)
				for _, setting := range settings {
					go handler(setting)
				}
			}
		}
	}
}

func handleDepthFtx(markets *model.Markets, response *simplejson.Json) {
	if response == nil {
		return
	}
	symbol := response.Get("market").MustString()
	dataType := response.Get(`type`).MustString()
	data := response.Get(`data`)
	if data != nil {
		bidAsk := &model.BidAsk{}
		ts := data.Get(`time`).MustFloat64()
		bidAsk.Ts = int(ts * 1000)
		bids := data.Get(`bids`).MustArray()
		asks := data.Get(`asks`).MustArray()
		bidAsk.Bids = make([]model.Tick, 0)
		bidAsk.Asks = make([]model.Tick, 0)
		if dataType == `partial` {
			for _, item := range bids {
				price, _ := item.([]interface{})[0].(json.Number).Float64()
				size, _ := item.([]interface{})[1].(json.Number).Float64()
				bidAsk.Bids = append(bidAsk.Bids, model.Tick{Price: price, Amount: size})
			}
			for _, item := range asks {
				price, _ := item.([]interface{})[0].(json.Number).Float64()
				size, _ := item.([]interface{})[1].(json.Number).Float64()
				bidAsk.Asks = append(bidAsk.Asks, model.Tick{Price: price, Amount: size})
			}
		} else if dataType == `update` {
			_, oldBidAsk := markets.GetBidAsk(symbol, model.Ftx)
			if oldBidAsk == nil {
				util.Notice(fmt.Sprintf(`fatal: can not have old bidask %s %s`, model.Ftx, symbol))
				return
			}
			priceAmountBid := make(map[float64]*model.Tick)
			priceAmountAsk := make(map[float64]*model.Tick)
			for _, item := range bids {
				price, _ := item.([]interface{})[0].(json.Number).Float64()
				size, _ := item.([]interface{})[1].(json.Number).Float64()
				priceAmountBid[price] = &model.Tick{Price: price, Amount: size}
			}
			for _, item := range asks {
				price, _ := item.([]interface{})[0].(json.Number).Float64()
				size, _ := item.([]interface{})[1].(json.Number).Float64()
				priceAmountAsk[price] = &model.Tick{Price: price, Amount: size}
			}
			for _, bid := range oldBidAsk.Bids {
				if priceAmountBid[bid.Price] == nil {
					bidAsk.Bids = append(bidAsk.Bids, bid)
				} else if priceAmountBid[bid.Price].Amount > 0 {
					bidAsk.Bids = append(bidAsk.Bids, *priceAmountBid[bid.Price])
				}
				delete(priceAmountBid, bid.Price)
			}
			for _, bid := range priceAmountBid {
				if bid.Amount > 0 {
					bidAsk.Bids = append(bidAsk.Bids, *bid)
				}
			}
			for _, ask := range oldBidAsk.Asks {
				if priceAmountAsk[ask.Price] == nil {
					bidAsk.Asks = append(bidAsk.Asks, ask)
				} else if priceAmountAsk[ask.Price].Amount > 0 {
					bidAsk.Asks = append(bidAsk.Asks, *priceAmountAsk[ask.Price])
				}
				delete(priceAmountAsk, ask.Price)
			}
			for _, ask := range priceAmountAsk {
				if ask.Amount > 0 {
					bidAsk.Asks = append(bidAsk.Asks, *ask)
				}
			}
		}
		sort.Sort(bidAsk.Asks)
		sort.Sort(sort.Reverse(bidAsk.Bids))
		if markets.SetBidAsk(symbol, model.Ftx, bidAsk) {
			//util.SocketInfo(markets.ToStringBidAsk(bidAsk))
			for function, handler := range model.GetFunctions(model.Ftx, symbol) {
				if handler != nil {
					settings := model.GetSetting(function, model.Ftx, symbol)
					for _, setting := range settings {
						go handler(setting)
					}
				}
			}
		}
	}
}

func getCandlesFtx(key, secret, symbol, binSize string, start, end time.Time, count int) (
	candles map[string]*model.Candle) {
	candles = make(map[string]*model.Candle)
	param := make(map[string]interface{})
	if binSize == `1d` {
		param[`resolution`] = `86400`
	}
	param[`limit`] = fmt.Sprintf(`%d`, count)
	param[`start_time`] = fmt.Sprintf(`%d`, start.Unix())
	param[`end_time`] = fmt.Sprintf(`%d`, end.Unix())
	response := SignedRequestFtx(key, secret, `GET`,
		fmt.Sprintf(`/markets/%s/candles`, symbol), param, nil)
	candleJson, err := util.NewJSON(response)
	if err == nil {
		candleJsons := candleJson.Get(`result`).MustArray()
		for _, value := range candleJsons {
			item := value.(map[string]interface{})
			candle := &model.Candle{Market: model.Ftx, Symbol: symbol, Period: binSize}
			if item[`open`] != nil {
				candle.PriceOpen, _ = item[`open`].(json.Number).Float64()
			}
			if item[`close`] != nil {
				candle.PriceClose, _ = item[`close`].(json.Number).Float64()
			}
			if item[`high`] != nil {
				candle.PriceHigh, _ = item[`high`].(json.Number).Float64()
			}
			if item[`low`] != nil {
				candle.PriceLow, _ = item[`low`].(json.Number).Float64()
			}
			if item[`startTime`] != nil {
				start, _ := time.Parse(time.RFC3339, item[`startTime`].(string))
				candle.UTCDate = start.Format(time.RFC3339)[0:10]
				candles[candle.UTCDate] = candle
			}
		}
	}
	return
}

func parseBalanceFtx(data map[string]interface{}) (balance *model.Balance) {
	if data[`coin`] == nil {
		return nil
	}
	coin := data[`coin`].(string)
	balance = &model.Balance{
		Market:      model.Ftx,
		Coin:        coin,
		ID:          model.Ftx + `_` + coin + `_` + util.GetNow().Format(time.RFC3339)[0:10],
		BalanceTime: util.GetNow(),
		AccountId:   model.AppConfig.FtxKey}
	if data[`availableWithoutBorrow`] != nil {
		balance.Available, _ = data[`availableWithoutBorrow`].(json.Number).Float64()
	}
	if data[`total`] != nil {
		balance.Amount, _ = data[`total`].(json.Number).Float64()
	}
	if data[`spotBorrow`] != nil {
		borrow, _ := data[`spotBorrow`].(json.Number).Float64()
		balance.Amount -= borrow
	}
	if data[`usdValue`] != nil {
		balance.UsdValue, _ = data[`usdValue`].(json.Number).Float64()
	}
	return balance
}

func parseTransactionFtx(data map[string]interface{}, action float64) (balance *model.Balance) {
	if data[`id`] == nil {
		return nil
	}
	balance = &model.Balance{
		Market:    model.Ftx,
		ID:        model.Ftx + `_` + data[`id`].(json.Number).String(),
		Action:    action,
		AccountId: model.AppConfig.FtxKey}
	if data[`notes`] != nil {
		balance.Notes = data[`notes`].(string)
	}
	if data[`coin`] != nil {
		balance.Coin = strings.ToLower(data[`coin`].(string))
	}
	if data[`fee`] != nil {
		balance.Fee = data[`fee`].(json.Number).String()
	}
	if data[`size`] != nil {
		balance.Amount, _ = data[`size`].(json.Number).Float64()
	}
	if data[`time`] != nil {
		balance.BalanceTime, _ = time.Parse(time.RFC3339Nano, data[`time`].(string))
	}
	if data[`status`] != nil {
		balance.Status, _ = data[`status`].(string)
	}
	if data[`address`] != nil {
		if action == 1 {
			address := data[`address`].(map[string]interface{})
			if address != nil {
				balance.Address = address[`address`].(string)
			}
		} else if action == -1 {
			balance.Address = data[`address`].(string)
		}
	}
	if data[`txid`] != nil {
		balance.TransactionId, _ = data[`txid`].(string)
	}
	return balance
}

func getTransferFtx(key, secret string) (balances []*model.Balance) {
	balances = make([]*model.Balance, 0)
	response := SignedRequestFtx(key, secret, `GET`, `/wallet/deposits`, nil, nil)
	util.SocketInfo(`ftx deposit: ` + string(response))
	deposit, err := util.NewJSON(response)
	if err == nil && deposit != nil {
		for _, item := range deposit.Get(`result`).MustArray() {
			balance := parseTransactionFtx(item.(map[string]interface{}), 1)
			if balance != nil {
				balances = append(balances, balance)
			}
		}
	}
	response = SignedRequestFtx(key, secret, `GET`, `/wallet/withdrawals`, nil, nil)
	util.SocketInfo(`ftx withdraw ` + string(response))
	withdraw, err := util.NewJSON(response)
	if err == nil && withdraw != nil {
		for _, item := range withdraw.Get(`result`).MustArray() {
			balance := parseTransactionFtx(item.(map[string]interface{}), -1)
			if balance != nil {
				balances = append(balances, balance)
			}
		}
	}
	return
}

func getBalanceFtx(key, secret string) (balances []*model.Balance) {
	balances = make([]*model.Balance, 0)
	response := SignedRequestFtx(key, secret, `GET`, `/wallet/balances`, nil, nil)
	util.SocketInfo(`get usd balance ftx: ` + string(response))
	balanceJson, err := util.NewJSON(response)
	if err == nil && balanceJson != nil {
		for _, item := range balanceJson.Get(`result`).MustArray() {
			balance := parseBalanceFtx(item.(map[string]interface{}))
			if balance != nil {
				balances = append(balances, balance)
			}
		}
	}
	return
}

func cancelOrdersFtx(key, secret, symbol string) (result bool) {
	postData := make(map[string]interface{})
	postData[`market`] = symbol
	response := SignedRequestFtx(key, secret, http.MethodDelete, `/orders`, nil, postData)
	util.Notice(fmt.Sprintf(`[api cancelOrdersFtx]%s %s`, symbol, string(response)))
	orderJson, err := util.NewJSON(response)
	if err == nil {
		return orderJson.Get(`success`).MustBool()
	}
	return false
}

func cancelOrderFtx(key, secret, orderType, orderId string) (result bool) {
	path := `/orders/%s`
	if orderType == model.OrderTypeStop {
		path = `/conditional_orders/%s`
	}
	response := SignedRequestFtx(key, secret, `DELETE`, fmt.Sprintf(path, orderId), nil, nil)
	util.Notice(fmt.Sprintf(`cancel ftx %s %s: %s`, orderType, orderId, string(response)))
	orderJson, err := util.NewJSON(response)
	if err == nil {
		if strings.Contains(orderJson.Get(`error`).MustString(), `already closed`) {
			return true
		}
		return orderJson.Get(`success`).MustBool()
	}
	return false
}

func queryTriggerOrderId(key, secret, id string) (orderId string) {
	response := SignedRequestFtx(key, secret, `GET`,
		fmt.Sprintf(`/conditional_orders/%s/triggers`, id), nil, nil)
	orderJson, err := util.NewJSON(response)
	util.SocketInfo(fmt.Sprintf(`query trigger orderid from %s:%s`, id, string(response)))
	if err == nil && orderJson.Get(`success`).MustBool() {
		orders := orderJson.Get(`result`).MustArray()
		for _, item := range orders {
			data := item.(map[string]interface{})
			if data[`orderId`] != nil {
				orderNumber, _ := data[`orderId`].(json.Number).Int64()
				return fmt.Sprintf(`%d`, orderNumber)
			}
		}
	}
	return
}

func queryOpenTriggerOrders(key, secret, symbol, triggerId string) (status string) {
	param := make(map[string]interface{})
	param[`market`] = symbol
	response := SignedRequestFtx(key, secret, `GET`, `/conditional_orders`, param, nil)
	util.SocketInfo(fmt.Sprintf(`query open trigger orders ftx %s: %s`, symbol, string(response)))
	orderJson, err := util.NewJSON(response)
	status = model.CarryStatusWorking
	if err == nil {
		result := orderJson.Get(`result`)
		if result != nil && orderJson.Get(`success`).MustBool() {
			status = model.CarryStatusFail
			orders := result.MustArray()
			for _, order := range orders {
				item := order.(map[string]interface{})
				if item[`id`] != nil {
					num, _ := item[`id`].(json.Number).Float64()
					if fmt.Sprintf(`%d`, int(num)) == triggerId {
						return model.CarryStatusWorking
					}
				}
			}
		}
	}
	return status
}

func queryOrderFtx(key, secret, orderId string) (order *model.Order) {
	response := SignedRequestFtx(key, secret, `GET`, fmt.Sprintf(`/orders/%s`, orderId), nil, nil)
	util.SocketInfo(fmt.Sprintf(`query orders ftx: %s`, string(response)))
	orderJson, err := util.NewJSON(response)
	if err == nil && orderJson.Get(`success`).MustBool() {
		data, _ := orderJson.Get(`result`).Map()
		order = &model.Order{Market: model.Ftx}
		parseOrderFtx(order, data)
	}
	return
}

func getAccountFtx(key, secret string, accounts *model.Accounts) {
	postData := make(map[string]interface{})
	response := SignedRequestFtx(key, secret, `GET`, `/positions`, nil, postData)
	util.SocketInfo(`get account ftx ` + fmt.Sprintf(string(response)))
	positionJson, err := util.NewJSON(response)
	if err == nil {
		positionJson = positionJson.Get(`result`)
		if positionJson != nil {
			data := positionJson.MustArray()
			for _, item := range data {
				account := &model.Account{Market: model.Ftx, Ts: util.GetNowUnixMillion()}
				parseAccountFtx(account, item.(map[string]interface{}))
				accounts.SetAccount(model.Ftx, account.Currency, account)
			}
		}
	}
}

func getMarketInfoFtx(symbol string) (borrowAble float64) {
	if !strings.Contains(symbol, `/`) {
		return 0
	}
	coin := strings.Split(symbol, `/`)[0]
	param := map[string]interface{}{`market`: symbol}
	response := SignedRequestFtx(``, ``, http.MethodGet, `/spot_margin/market_info`, param, nil)
	borrowJson, err := util.NewJSON(response)
	if err == nil {
		results := borrowJson.Get(`result`).MustArray()
		for _, result := range results {
			value := result.(map[string]interface{})
			if value[`coin`] == nil {
				continue
			}
			if value[`coin`].(string) == coin {
				if value[`free`] == nil {
					return 0
				}
				borrowAble, _ = value[`free`].(json.Number).Float64()
				return borrowAble
			}
		}
	}
	return 0
}

func getMarketsFtx() (marketInfos map[string]*model.MarketInfo) {
	response := SignedRequestFtx(``, ``, `GET`,
		`/markets`, nil, nil)
	util.SocketInfo(string(response))
	marketInfos = make(map[string]*model.MarketInfo)
	rateJson, err := util.NewJSON(response)
	response = SignedRequestFtx(``, ``, `GET`,
		`/spot_margin/borrow_rates`, nil, nil)
	borrowJson, _ := util.NewJSON(response)
	if err == nil && rateJson.Get(`result`) != nil && borrowJson.Get(`result`) != nil {
		items, _ := rateJson.Get(`result`).Array()
		borrows, _ := borrowJson.Get(`result`).Array()
		canBorrows := make(map[string]bool)
		for _, borrow := range borrows {
			value := borrow.(map[string]interface{})
			canBorrows[value[`coin`].(string)] = true
		}
		for _, item := range items {
			value := item.(map[string]interface{})
			marketInfo := &model.MarketInfo{CanBorrow: true}
			if value[`name`] != nil {
				marketInfo.Name = value[`name`].(string)
			} else {
				continue
			}
			if value[`baseCurrency`] != nil && value[`type`] != nil && value[`type`].(string) == `spot` &&
				!canBorrows[value[`baseCurrency`].(string)] {
				marketInfo.CanBorrow = false
			}
			if value[`priceIncrement`] != nil {
				marketInfo.PriceIncrement, _ = value[`priceIncrement`].(json.Number).Float64()
			}
			if value[`sizeIncrement`] != nil {
				marketInfo.SizeIncrement, _ = value[`sizeIncrement`].(json.Number).Float64()
			}
			marketInfos[marketInfo.Name] = marketInfo
		}
	}
	return
}

func getFundingRatesFtx() (fundingRates []*model.FundingRate) {
	response := SignedRequestFtx(``, ``, `GET`,
		`/funding_rates`, nil, nil)
	util.SocketInfo(string(response))
	rateJson, err := util.NewJSON(response)
	if err == nil && rateJson.Get(`result`) != nil {
		items, _ := rateJson.Get(`result`).Array()
		fundingRates = make([]*model.FundingRate, len(items))
		for i, item := range items {
			value := item.(map[string]interface{})
			fundingTime, _ := time.Parse(time.RFC3339, value[`time`].(string))
			rate, _ := value[`rate`].(json.Number).Float64()
			fundingRates[i] = &model.FundingRate{Symbol: value[`future`].(string), FundingTime: fundingTime,
				Rate: rate, ID: value[`future`].(string) + value[`time`].(string)}
		}
	}
	return fundingRates
}

func parseAccountFtx(account *model.Account, item map[string]interface{}) {
	if item[`entryPrice`] != nil {
		account.EntryPrice, _ = item[`entryPrice`].(json.Number).Float64()
	}
	if item[`estimatedLiquidationPrice`] != nil {
		account.LiquidationPrice, _ = item[`estimatedLiquidationPrice`].(json.Number).Float64()
	}
	if item[`future`] != nil {
		account.Currency = item[`future`].(string)
	}
	if item[`netSize`] != nil {
		account.Free, _ = item[`netSize`].(json.Number).Float64()
		account.Holding = account.Free
		//account.Free = account.Free * account.EntryPrice
	}
	if item[`realizedPnl`] != nil {
		account.ProfitReal, _ = item[`realizedPnl`].(json.Number).Float64()
	}
	if item[`side`] != nil {
		account.Direction = item[`side`].(string)
	}
	if item[`bust_price`] != nil {
		account.BankruptcyPrice, _ = strconv.ParseFloat(item[`bust_price`].(string), 64)
	}
	if item[`position_margin`] != nil {
		account.Margin, _ = strconv.ParseFloat(item[`position_margin`].(string), 64)
	}
	if item[`unrealizedPnl`] != nil {
		account.ProfitUnreal, _ = item[`unrealizedPnl`].(json.Number).Float64()
	}

}

//remainingSize	number	31431.0
//reduceOnly	boolean	false
//ioc	boolean	false
//postOnly	boolean	false
//orderPrice	number	null	price of the order sent when this stop loss triggered
//retryUntilFilled	boolean	false	Whether or not to keep re-triggering until filled
//orderType	string	market	Values are market and limit
func parseOrderFtx(order *model.Order, item map[string]interface{}) {
	if order == nil || item == nil {
		return
	}
	if item[`createdAt`] != nil {
		order.OrderTime, _ = time.Parse(time.RFC3339, item[`createdAt`].(string))
	}
	if item[`filledSize`] != nil {
		order.DealAmount, _ = item[`filledSize`].(json.Number).Float64()
	}
	if item[`id`] != nil {
		order.OrderId = item[`id`].(json.Number).String()
	}
	if item[`market`] != nil {
		order.Symbol = item[`market`].(string)
	}
	if item[`price`] != nil {
		order.Price, _ = item[`price`].(json.Number).Float64()
	}
	if item[`avgFillPrice`] != nil {
		order.DealPrice, _ = item[`avgFillPrice`].(json.Number).Float64()
	}
	if item[`side`] != nil {
		order.OrderSide = strings.ToLower(item[`side`].(string))
	}
	if item[`size`] != nil {
		order.Amount, _ = item[`size`].(json.Number).Float64()
	}
	if item[`type`] != nil {
		order.OrderType = strings.ToLower(item[`type`].(string))
	}
	if item[`status`] != nil {
		order.Status = model.GetOrderStatus(model.Ftx, item[`status`].(string))
		if order.DealAmount == 0 && order.Status == model.CarryStatusSuccess {
			order.Status = model.CarryStatusFail
		}
	}
	if item[`orderPrice`] != nil {
		order.Price, _ = item[`orderPrice`].(json.Number).Float64()
	}
	if item[`triggeredAt`] != nil {
		order.OrderUpdateTime, _ = time.Parse(time.RFC3339, item[`triggeredAt`].(string))
	}
	if order.Status != model.CarryStatusSuccess && order.Status != model.CarryStatusFail {
		order.Status = model.CarryStatusWorking
	}
	if order.DealAmount == 0 || order.DealPrice == 0 {
		order.DealPrice = order.Price
	}
	//order.Amount = order.Amount * order.Price
	//order.DealAmount = order.DealAmount * order.Price
	order.UnfilledQuantity = order.Amount - order.DealAmount
	return
}

//orderType: "limit", "market", "stop", "trailingStop", "takeProfit"
func placeOrderFtx(order *model.Order, key, secret, orderSide, orderType, accountType, orderParam, symbol, orderPrice, triggerPrice,
	amount string) {
	uri := `/orders`
	param := make(map[string]interface{})
	param[`market`] = symbol
	postData := make(map[string]interface{})
	postData[`market`] = symbol
	postData[`side`] = orderSide
	postData[`size`], _ = strconv.ParseFloat(amount, 64)
	postData[`type`] = orderType
	if accountType == model.AccountTypeReduce {
		postData[`reduceOnly`] = true
	}
	if orderType == `limit` || orderType == `market` {
		postData[`price`], _ = strconv.ParseFloat(orderPrice, 64)
		if orderParam == model.PostOnly {
			postData[`postOnly`] = true
		}
	} else if orderType == `stop` || orderType == `trailingStop` || orderType == `takeProfit` {
		uri = `/conditional_orders`
		postData[`triggerPrice`], _ = strconv.ParseFloat(triggerPrice, 64)
		orderPriceValue, _ := strconv.ParseFloat(orderPrice, 64)
		if orderPriceValue > 0 {
			postData[`orderPrice`] = orderPriceValue
		}
	}
	response := SignedRequestFtx(key, secret, `POST`, uri, param, postData)
	util.Notice(`place ftx: ` + string(response))
	orderJson, err := util.NewJSON(response)
	if err == nil {
		success := orderJson.Get(`success`).MustBool()
		if success {
			data, _ := orderJson.Get(`result`).Map()
			parseOrderFtx(order, data)
		} else {
			order.Status = model.CarryStatusFail
			order.OrderId = ``
		}
	}
	return
}

func SignedRequestFtx(key, secret, method, path string, param, body map[string]interface{}) []byte {
	if key == `` {
		key = model.AppConfig.FtxKey
	}
	if secret == `` {
		secret = model.AppConfig.FtxSecret
	}
	if body == nil {
		body = make(map[string]interface{})
	}
	u, _ := url.ParseRequestURI(model.AppConfig.RestUrls[model.Ftx])
	u.Path += path
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	hash := hmac.New(sha256.New, []byte(secret))
	bodyStr := string(util.JsonEncodeMapToByte(body))
	q := u.Query()
	for k, v := range param {
		q.Set(k, v.(string))
	}
	u.RawQuery = q.Encode()
	uri := u.Path
	if u.Query().Encode() != `` {
		uri = fmt.Sprintf(`%s?%s`, u.Path, u.Query().Encode())
	}
	if method == http.MethodPost || method == http.MethodDelete {
		hash.Write([]byte(fmt.Sprintf(`%d%s%s%s`, ts, method, uri, bodyStr)))
	} else {
		hash.Write([]byte(fmt.Sprintf(`%d%s%s`, ts, method, uri)))
		bodyStr = ``
	}
	sign := hex.EncodeToString(hash.Sum(nil))
	headers := map[string]string{`FTX-KEY`: key, `FTX-TS`: strconv.FormatInt(ts, 10), "FTX-SIGN": sign,
		"Content-Type": "application/json"}
	responseBody, _ := util.HttpRequest(method, u.String(), bodyStr, headers, 60)
	return responseBody
}

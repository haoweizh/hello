package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/bitly/go-simplejson"
	"hello/model"
	"hello/util"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const restBinance = `https://api.binance.com`
const restDataBinance = `https://data.binance.com`

const wsStepBinance = 100

// 用于wss api调用,实际值为method
const wsAccountStatusV2 = "account-status-v2-"
const wsAccountBalanceV2 = `account-balance-v2-`

// 定义键值对
var accountMethodMap = map[string]string{
	wsAccountStatusV2:  "v2/account.status",
	wsAccountBalanceV2: `v2/account.balance`}

func GetMarketsBinance(account *model.Account, market string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	client := binance.NewClient(account.Key, account.Secret)
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	stats, _ := client.NewListPriceChangeStatsService().Do(context.Background())
	if err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf("GetMarketsBinance %s err: %s %#v休息五分钟", account.Key, err.Error(), exchangeInfo))
		time.Sleep(time.Minute * 5)
		return GetMarketsBinance(account, market)
	}
	for _, item := range exchangeInfo.Symbols {
		if item.QuoteAsset != model.DialectTail[model.MarketTypeSpot][market] || !item.IsMarginTradingAllowed {
			continue
		}
		tail := model.UniStandardTail[model.MarketTypeSpot]
		marketInfo := &model.MarketInfo{Market: market, Symbol: item.BaseAsset + tail, FundingRateInterval: 8 * 3600000}
		if item.Status != "TRADING" { // (item.Status != "TRADING" && item.Status != `BREAK`) ||
			marketInfo.DeListing = true
		}
		for _, data := range item.Filters {
			filterType := data[`filterType`].(string)
			if filterType == `PRICE_FILTER` {
				if data[`tickSize`] != nil {
					marketInfo.PriceIncrement, _ = strconv.ParseFloat(data[`tickSize`].(string), 64)
				}
				marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
			} else if filterType == `LOT_SIZE` {
				if data[`minQty`] != nil {
					marketInfo.SizeMin, _ = strconv.ParseFloat(data[`minQty`].(string), 64)
				}
				if data[`maxQty`] != nil {
					marketInfo.SizeMax, _ = strconv.ParseFloat(data[`maxQty`].(string), 64)
				}
				if data[`stepSize`] != nil {
					marketInfo.SizeIncrement, _ = strconv.ParseFloat(data[`stepSize`].(string), 64)
				}
			} else if filterType == `NOTIONAL` {
				if data[`minNotional`] != nil {
					marketInfo.MoneyMin, _ = strconv.ParseFloat(data[`minNotional`].(string), 64)
				} else if tail == `_USDT` {
					marketInfo.MoneyMin = 10
				}
			}
		}
		marketInfos[marketInfo.Symbol] = marketInfo
	}
	for _, stat := range stats {
		if stat == nil {
			continue
		}
		success, _, symbol := model.GetFromDialect(model.BinanceSpot, model.MarketTypeSpot, stat.Symbol)
		if !success {
			continue
		}
		if marketInfos[symbol] != nil {
			marketInfos[symbol].TradeAmount, _ = strconv.ParseFloat(stat.QuoteVolume, 64)
		}
	}
	return marketInfos
}

var KLineMsgHandlerBinanceSpot = func(market string, conn *model.WSConn, event []byte) {
	result, wsErr := util.NewJSON(event)
	if wsErr != nil {
		util.Log(util.LogLevelError, `KLineMsgHandlerBinanceSpot binance fail to unmarshal json `+wsErr.Error())
		return
	}
	result = result.Get(`data`)
	if result == nil {
		return
	}
	subscribe, _ := result.Get("e").String()
	dialectSymbol := result.Get(`s`).MustString()
	if subscribe != `kline` {
		return
	}
	success, _, standardSymbol := model.GetFromDialect(market, model.MarketTypeSpot, dialectSymbol)
	if !success {
		return
	}
	createdAt := time.UnixMilli(result.Get(`E`).MustInt64())
	result = result.Get(`k`)
	candle := &model.Candle{Market: market, Symbol: standardSymbol, CreatedAt: createdAt,
		Begin: time.UnixMilli(result.Get(`t`).MustInt64()), Seconds: 60,
		End: time.UnixMilli(result.Get(`T`).MustInt64())}
	candle.PriceOpen, _ = strconv.ParseFloat(result.Get(`o`).MustString(), 64)
	candle.PriceClose, _ = strconv.ParseFloat(result.Get(`c`).MustString(), 64)
	candle.PriceHigh, _ = strconv.ParseFloat(result.Get(`h`).MustString(), 64)
	candle.PriceLow, _ = strconv.ParseFloat(result.Get(`l`).MustString(), 64)
	candle.Volume, _ = strconv.ParseFloat(result.Get(`v`).MustString(), 64)
	candle.VolumeQuote, _ = strconv.ParseFloat(result.Get(`q`).MustString(), 64)
	if model.AppEnvironment.SetCandle(candle.Symbol, candle.Market, candle) {
		for _, handler := range model.CandleHandlers {
			handler(model.AppEnvironment, candle)
		}
	}
}

// WsKLineBinanceSpot TODO 由于同一个交易所多次调用WsPublicClient，所以不支持使用specialChan，需要做相应改动
func WsKLineBinanceSpot(market string, symbols map[string]bool) (
	socketMap map[*model.WSConn]bool, msgChans []chan struct{}, connectErr error) {
	subs := make([]interface{}, 0)
	for symbol := range symbols {
		_, _, _, dialectSymbol := model.GetFromStandard(market, symbol)
		subs = append(subs, strings.ToLower(dialectSymbol)+`@kline_1m`)
	}
	socketMap, connectErr = model.WsPublicClient(market, model.WsBinance+`/stream`, subs,
		subscribeHandlerBinance, KLineMsgHandlerBinanceSpot, wsStepBinance, true)
	return
}

var wsHandlerBinanceSpot = func(market string, conn *model.WSConn, event []byte) {
	result, wsErr := util.NewJSON(event)
	if wsErr != nil {
		util.Log(util.LogLevelError, `wsHandlerBinanceSpot binance fail to unmarshal json `+wsErr.Error())
		return
	}
	if result.Get(`data`).MustMap() != nil {
		result = result.Get(`data`)
	}
	if result == nil {
		return
	}
	dialectSymbol := result.Get(`s`).MustString()
	updateId := result.Get(`u`).MustInt64()
	success, _, standardSymbol := model.GetFromDialect(market, model.MarketTypeSpot, dialectSymbol)
	if !success {
		return
	}
	handleTickerBinance(model.AppEnvironment, result, market, standardSymbol, updateId)
}

//var subIdBinance sync.Map

var subscribeHandlerBinance = func(market string, connection *model.WSConn, subscribes []interface{}) (err error) {
	subParam := make(map[string]interface{})
	subParam["method"] = "SUBSCRIBE"
	subParam["params"] = subscribes
	txId := time.Now().UnixMilli()
	subParam["id"] = txId
	subParamJson, _ := json.Marshal(subParam)
	if err = connection.WriteMsg(subParamJson); err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf("subscribeHandlerBinance spot can not subscribe %s %s", subParamJson, err.Error()))
	}
	return err
}

func handleTickerBinance(environment *model.Environment, json *simplejson.Json, market, standardSymbol string, updateId int64) {
	bidPrice, _ := strconv.ParseFloat(json.Get(`b`).MustString(), 64)
	bidAmount, _ := strconv.ParseFloat(json.Get(`B`).MustString(), 64)
	askPrice, _ := strconv.ParseFloat(json.Get(`a`).MustString(), 64)
	askAmount, _ := strconv.ParseFloat(json.Get(`A`).MustString(), 64)
	ts := json.Get(`E`).MustInt()
	now := int(time.Now().UnixNano() / int64(time.Millisecond))
	if ts == 0 {
		ts = now
	}
	if bidPrice > 0 && bidAmount > 0 && askPrice > 0 && askAmount > 0 {
		bidAsk := model.BidAsk{Ts: ts, TsReceived: now, UpdateId: updateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: market, Symbol: standardSymbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: market, Symbol: standardSymbol}}}
		if environment.SetBidAsk(market, standardSymbol, &bidAsk) {
			funcHandlers := GetFunctions(market, standardSymbol)
			if funcHandlers != nil {
				funcHandlers.Range(func(function, value interface{}) bool {
					setting := GetSetting(function.(string), market, standardSymbol)
					if setting != nil && value != nil && value.(model.CarryHandler) != nil {
						go value.(model.CarryHandler)(setting, &bidAsk)
					}
					return true
				})
			}
		}
	}
}

// handleDepthBinance
func _(environment *model.Environment, json *simplejson.Json, market, standardSymbol string, updateId int64) {
	now := int(util.GetNowUnixMillion())
	bidAsk := model.BidAsk{UpdateId: updateId, Ts: now, TsReceived: now}
	var bids, asks []interface{}
	bidArray, _ := json.Get(`bids`).Array()
	bids = bidArray
	askArray, _ := json.Get(`asks`).Array()
	asks = askArray
	bidAsk.Bids = make([]model.Tick, len(bids))
	for i, value := range bids {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Bids[i] = model.Tick{Price: price, Amount: amount, Market: market, Symbol: standardSymbol}
	}
	bidAsk.Asks = make([]model.Tick, len(asks))
	for i, value := range asks {
		if len(value.([]interface{})) < 2 {
			return
		}
		price, _ := strconv.ParseFloat(value.([]interface{})[0].(string), 64)
		amount, _ := strconv.ParseFloat(value.([]interface{})[1].(string), 64)
		bidAsk.Asks[i] = model.Tick{Price: price, Amount: amount, Market: market, Symbol: standardSymbol}
	}
	sort.Sort(bidAsk.Asks)
	sort.Sort(sort.Reverse(bidAsk.Bids))
	if environment.SetBidAsk(market, standardSymbol, &bidAsk) {
		funcHandlers := GetFunctions(market, standardSymbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), market, standardSymbol)
				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
					go value.(model.CarryHandler)(setting, &bidAsk)
				}
				return true
			})
		}
	}
}

func placeOrderBinanceSpot(account *model.Account, isWs bool, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	decimal := 0
	price, decimal = model.FormatPrice(model.BinanceSpot, symbol, price)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	formattedAmount, format := model.GetAmountInMarket(model.BinanceSpot, symbol, amount, price, false)
	amountStr := util.CutTailZero(fmt.Sprintf(format, formattedAmount))
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	order.Price = price
	order.TriggerPrice = price
	if !success {
		return
	}
	if isWs {
		if orderSide == model.OrderSideBuy {
			orderSide = string(binance.SideTypeBuy)
		} else if orderSide == model.OrderSideSell {
			orderSide = string(binance.SideTypeSell)
		}
		ts := time.Now().UnixMilli()
		param := url.Values{}
		param.Set("symbol", dialectSymbol)
		param.Set("side", orderSide)
		param.Set("type", strings.ToUpper(orderType))
		param.Set("timeInForce", `GTC`)
		param.Set(`price`, priceStr)
		param.Set(`quantity`, amountStr)
		param.Set(`apiKey`, account.Key)
		param.Set(`timestamp`, fmt.Sprintf(`%d`, ts))
		param.Set(`newClientOrderId`, order.ClientOrdId)
		hash := hmac.New(sha256.New, []byte(account.Secret))
		hash.Write([]byte(param.Encode()))
		msg := fmt.Sprintf(`{"id": "%s","method": "order.place","params":{"symbol": "%s","side": "%s","type": "%s",
			"timeInForce": "GTC","price": "%s","quantity": "%s","apiKey": "%s","signature": "%s","timestamp": %d, "newClientOrderId": "%s"}}`,
			order.ClientOrdId, dialectSymbol, orderSide, strings.ToUpper(orderType), priceStr, amountStr, account.Key,
			hex.EncodeToString(hash.Sum(nil)), ts, order.ClientOrdId)
		connKey := getPrivateConnKey(model.BinanceSpot, account.Key, ``)
		value, _ := model.AppEnvironment.ConnOrder.Load(connKey)
		if value == nil {
			order.Status = model.CarryStatusFail
		} else {
			if err := value.(*model.WSConn).WriteMsg([]byte(msg)); err != nil {
				model.AppEnvironment.ConnOrder.Delete(connKey)
				order.Status = model.CarryStatusFail
				util.Log(util.LogLevelError, fmt.Sprintf(`fail to place binancespot order return: %s`, err.Error()))
			}
		}
		if order.Status == model.CarryStatusFail {
			HandleWsOrderConnFail(account, model.BinanceSpot, order)
		}
	} else {
		client := binance.NewClient(account.Key, account.Secret)
		service := client.NewCreateOrderService().Symbol(dialectSymbol).Quantity(amountStr)
		if orderSide == model.OrderSideBuy {
			service.Side(binance.SideTypeBuy)
		} else if orderSide == model.OrderSideSell {
			service.Side(binance.SideTypeSell)
		}
		if orderType == model.OrderTypeMarket {
			service.Type(binance.OrderTypeMarket)
		} else if orderType == model.OrderTypeLimit {
			service.Type(binance.OrderTypeLimit)
			service.Price(priceStr)
			service.TimeInForce(binance.TimeInForceTypeGTC)
		}
		service.NewClientOrderID(order.ClientOrdId)
		orderResponse, err := service.Do(context.Background())
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`placeOrderBinanceSpot err: %s amount %s`, err.Error(), amountStr))
			order.ErrCode = err.Error()
		} else {
			order.OrderId = strconv.FormatInt(orderResponse.OrderID, 10)
			order.Amount, _ = strconv.ParseFloat(orderResponse.OrigQuantity, 64)
		}
	}
}

func parseOrderBinanceSpotSdk(orderResp *binance.Order) (order *model.Order) {
	if orderResp == nil {
		return nil
	}
	order = &model.Order{Market: model.BinanceSpot, Status: model.CarryStatusFail}
	order.OrderId = strconv.FormatInt(orderResp.OrderID, 10)
	order.ClientOrdId = orderResp.ClientOrderID
	_, _, order.Symbol = model.GetFromDialect(model.BinanceSpot, model.MarketTypeSpot, orderResp.Symbol)
	order.OrderSide = strings.ToLower(string(orderResp.Side))
	order.OrderType = strings.ToLower(string(orderResp.Type))
	order.Amount, _ = strconv.ParseFloat(orderResp.OrigQuantity, 64)
	order.Price, _ = strconv.ParseFloat(orderResp.Price, 64)
	order.DealAmount, _ = strconv.ParseFloat(orderResp.ExecutedQuantity, 64)
	order.OrderTime = time.UnixMilli(orderResp.Time)
	order.OrderUpdateTime = time.UnixMilli(orderResp.UpdateTime)
	order.Status = model.GetOrderStatus(model.BinanceSpot, string(orderResp.Status))
	if order.Status != model.CarryStatusSuccess && order.Status != model.CarryStatusFail {
		order.Status = model.CarryStatusWorking
	}
	if order.DealAmount > 0 && order.DealPrice == 0 {
		order.DealPrice = order.Price
	}
	return order
}

// 兼容spot margin
func parseOrderBinanceSpot(market string, orderJson *simplejson.Json) (order *model.Order) {
	if orderJson == nil {
		return nil
	}
	order = &model.Order{Market: market}
	_, _, order.Symbol = model.GetFromDialect(market, model.MarketTypeSpot, orderJson.Get(`symbol`).MustString())
	order.OrderId = strconv.Itoa(orderJson.Get("orderId").MustInt())
	order.ClientOrdId = orderJson.Get(`clientOrderId`).MustString()
	order.OrderTime = time.UnixMilli(orderJson.Get("transactTime").MustInt64())
	order.Price, _ = strconv.ParseFloat(orderJson.Get(`price`).MustString(), 64)
	order.Amount, _ = strconv.ParseFloat(orderJson.Get("origQty").MustString(), 64)
	order.DealPrice, _ = strconv.ParseFloat(orderJson.Get(`executedQty`).MustString(), 64)
	if strings.EqualFold(orderJson.Get(`side`).MustString(), model.OrderSideSell) {
		order.OrderSide = model.OrderSideSell
	} else if strings.EqualFold(orderJson.Get(`side`).MustString(), model.OrderSideBuy) {
		order.OrderSide = model.OrderSideBuy
	}
	order.Status = model.GetOrderStatus(market, orderJson.Get(`status`).MustString())
	order.OrderType = GetStandardOrderType(market, orderJson.Get(`type`).MustString())
	return order
}

var wsOrderUpdateBinance = func(market, key string, msg []byte) {
	resJson, _ := util.NewJSON(msg)
	if resJson == nil {
		return
	}
	switch resJson.Get(`e`).MustString() {
	case `ORDER_TRADE_UPDATE`:
		orderId := strconv.Itoa(resJson.GetPath(`o`, `i`).MustInt())
		clientOid := resJson.GetPath(`o`, `c`).MustString()
		dealAmount, _ := strconv.ParseFloat(resJson.GetPath(`o`, `z`).MustString(), 64)
		status := model.GetOrderStatus(market, resJson.Get(`X`).MustString())
		UpdateOrderDeal(market, orderId, clientOid, status, string(msg), dealAmount)
	case `executionReport`:
		orderId := strconv.Itoa(resJson.Get(`i`).MustInt())
		dealAmount, _ := strconv.ParseFloat(resJson.Get(`z`).MustString(), 64)
		status := model.GetOrderStatus(market, resJson.Get(`X`).MustString())
		clientOId := resJson.GetPath(`c`).MustString()
		UpdateOrderDeal(market, orderId, clientOId, status, string(msg), dealAmount)
	case `outboundAccountPosition`:
		//https://developers.binance.com/docs/binance-spot-api-docs/user-data-stream#account-update
		dataArray := resJson.Get(`B`).MustArray()
		var balances []*model.Balance
		if dataArray != nil {
			for _, item := range dataArray {
				value := item.(map[string]interface{})
				balance := &model.Balance{Market: market, Coin: value[`a`].(string)}
				free, _ := strconv.ParseFloat(value[`f`].(string), 64)
				balance.BalanceTime = time.UnixMilli(resJson.Get(`u`).MustInt64())
				balance.FrozenAmount, _ = strconv.ParseFloat(value[`l`].(string), 64)
				balance.Amount = free + balance.FrozenAmount
				balances = append(balances, balance)
			}
		}
		if balances != nil && len(balances) > 0 {
			//util.LogLess(util.LogLevelInfo, fmt.Sprintf(`risk check ws update balances %s %#v %s`, market, balances, key))
			model.CrossBalancesHandler(market, key, balances)
		}
	case `ACCOUNT_UPDATE`:
		//https://developers.binance.com/docs/zh-CN/derivatives/usds-margined-futures/user-data-streams/Event-Balance-and-Position-Update
		util.LogLess(util.LogLevelInfo, "risk check ws update positions binance "+string(msg))
		//	collateral := &model.Collateral{AccountKey: key}
		//	dataarray := resJson.GetPath(`a`, `B`).MustArray()
		//	for _, v := range dataarray {
		//		value := v.(map[string]interface{})
		//		if value[`a`] != nil && value[`a`] == `USDT` {
		//			collateral.Available, _ = strconv.ParseFloat(value[`cw`].(string), 64)
		//		}
		//	}
		//	util.Log(util.LogLevelInfo, fmt.Sprintf("binance unified %s %f", collateral.AccountKey, collateral.Available))
		//	model.CollateralHandler(collateral)
		dataarray := resJson.GetPath(`a`, `P`).MustArray()
		positions := make([]*model.Position, 0)
		for _, v := range dataarray {
			value := v.(map[string]interface{})
			position := &model.Position{Market: model.BinancePerp, Ts: util.GetNowUnixMillion()}
			if value[`s`] != nil {
				isSuccess, _, symbol := model.GetFromDialect(model.BinancePerp, model.MarketTypePerp, value[`s`].(string))
				if !isSuccess {
					continue
				}
				position.Currency = symbol
			}
			if value[`pa`] != nil {
				position.Holding, _ = strconv.ParseFloat(value[`pa`].(string), 64)
			}
			if value[`ep`] != nil {
				position.EntryPrice, _ = strconv.ParseFloat(value[`ep`].(string), 64)
			}
			if value[`up`] != nil {
				position.ProfitUnreal, _ = strconv.ParseFloat(value[`up`].(string), 64)
			}
			if value[`bep`] != nil {
				position.BankruptcyPrice, _ = strconv.ParseFloat(value[`bep`].(string), 64)
			}
			if position.Holding != 0 {
				positions = append(positions, position)
			}
		}
		if len(positions) > 0 {
			model.CrossPositionsHandler(market, key, positions)
		}
	}
}

var wsActHandlerBinance = func(market, key string, event []byte) {
	responseJson, err := util.NewJSON(event)
	if err == nil && responseJson != nil {
		//返回状态，status=200则为正确
		status := responseJson.Get(`status`).MustInt()
		requestId := responseJson.Get(`id`).MustString()
		//基于ID类型判断是否为wsAccountStatusV2=v2/account.status
		if strings.HasPrefix(requestId, wsAccountStatusV2) {
			if status == 200 {
				collateral := &model.Collateral{AccountKey: key}
				collateral.Available, _ = strconv.ParseFloat(responseJson.GetPath(`result`, `totalMarginBalance`).MustString(), 64)
				collateral.AccountValueInU, _ = strconv.ParseFloat(responseJson.GetPath(`result`, `totalWalletBalance`).MustString(), 64)
				totalUnrealizedProfit, _ := strconv.ParseFloat(responseJson.GetPath(`result`, `totalUnrealizedProfit`).MustString(), 64)
				collateral.AccountValueInU += totalUnrealizedProfit
				util.Log(util.LogLevelInfo, fmt.Sprintf("binance unified %s %f %f %s %f", collateral.AccountKey,
					collateral.Available, collateral.AccountValueInU, responseJson.GetPath(`result`, `totalWalletBalance`), totalUnrealizedProfit))
				model.CollateralHandler(key, model.MarketTypePerp, false, collateral)
			} else {
				code := responseJson.GetPath(`error`, `code`).MustInt()
				util.Log(util.LogLevelError, fmt.Sprintf("binance unified request %s code %d msg %s", requestId, code, responseJson.GetPath(`error`, `msg`)))
			}
		} else if strings.HasPrefix(requestId, wsAccountBalanceV2) {
			if status == 200 {
				collateral := &model.Collateral{AccountKey: key}
				assets := responseJson.Get(`result`).MustArray()
				for _, item := range assets {
					asset := item.(map[string]interface{})[`asset`].(string)
					balance, _ := strconv.ParseFloat(item.(map[string]interface{})[`balance`].(string), 64)
					crossUnPnl, _ := strconv.ParseFloat(item.(map[string]interface{})[`crossUnPnl`].(string), 64)
					availableBalance, _ := strconv.ParseFloat(item.(map[string]interface{})[`availableBalance`].(string), 64)
					if asset == `USDT` {
						collateral.Available = availableBalance
						collateral.AccountValueInU += balance
						collateral.AccountValueInU += crossUnPnl
					} else {
						getPrice, price := GetPriceForce(model.BinancePerp, asset+model.UniStandardTail[model.MarketTypePerp], false)
						if getPrice {
							collateral.AccountValueInU += balance * price
						}
					}
				}
				model.CollateralHandler(key, model.MarketTypePerp, false, collateral)
			} else {
				code := responseJson.GetPath(`error`, `code`).MustInt()
				util.Log(util.LogLevelError, fmt.Sprintf("binance unified request %s code %d msg %s", requestId, code, responseJson.GetPath(`error`, `msg`)))
			}
		} else {
			idInt := responseJson.GetPath(`result`, `orderId`).MustInt()
			wsResp := model.WSResp{RequestId: responseJson.Get(`id`).MustString(), OrderId: strconv.Itoa(idInt)}
			if status == 200 {
				wsResp.Success = true
			} else {
				wsResp.Success = false
				code := responseJson.GetPath(`error`, `code`).MustInt()
				wsResp.Msg = fmt.Sprintf(`%d%s`, code, responseJson.GetPath(`error`, `msg`))
			}
			model.AppEnvironment.WSRespChan <- wsResp
		}
	}
}

func WsOrderServeBinance(account *model.Account, market string) {
	if account == nil {
		return
	}
	replaced := model.AppEnvironment.PriConnecting.CompareAndSwap(market+account.Key, false, true)
	if !replaced {
		return
	}
	defer func() {
		select {
		case <-time.After(time.Second * 10):
		}
		model.AppEnvironment.PriConnecting.Store(market+account.Key, false)
	}()
	apiUrl := ``
	streamUrl := ``
	if market == model.BinanceSpot {
		apiUrl = model.WsBinanceSpotApi
		streamUrl = model.WsBinance
	} else if market == model.BinancePerp {
		apiUrl = model.WsBinancePerpApi
		streamUrl = model.WsBinancePerp
	}
	connKey := getPrivateConnKey(market, account.Key, ``)
	conn, err := model.WsPrivateClient(account, &model.AppEnvironment.ConnOrder, connKey, market, apiUrl, wsActHandlerBinance, false)
	if err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to create account ws %s %s`, market, err.Error()))
	} else {
		model.AppEnvironment.ConnOrder.Store(connKey, conn)
	}
	_, listenKey := RenewListenKeyBinance(account, market)
	msg := fmt.Sprintf(`%s/ws/%s`, streamUrl, listenKey)
	connUpdate, errUpdate := model.WsPrivateClient(account, &model.AppEnvironment.ConnOrderUpdate, connKey, market, msg, wsOrderUpdateBinance, true)
	if errUpdate != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to create order update ws %s %s`, market, errUpdate.Error()))
	} else {
		model.AppEnvironment.ConnOrderUpdate.Store(connKey, connUpdate)
		util.Log(util.LogLevelInfo, fmt.Sprintf("log in conn %s %s", market, msg))
	}
}

func cancelOrderBinanceSpot(key, secret, market, symbol, orderId string) (suc bool, order *model.Order) {
	var path string
	if market == model.BinanceSpot {
		path = "/api/v3/order"
	} else if market == model.BinanceMargin {
		path = `/sapi/v1/margin/order`
	}
	responseBody := signedRequestBinance(key, secret, model.BinanceSpot, http.MethodDelete, restBinance+path,
		true, map[string]interface{}{`symbol`: symbol, `orderId`: orderId})
	orderJson, err := util.NewJSON(responseBody)
	if err == nil {
		order = parseOrderBinanceSpot(market, orderJson)
		if order != nil && order.Status == model.CarryStatusFail {
			return true, order
		}
	}
	util.Log(util.LogLevelError, fmt.Sprintf(`fail to cancel order binanceSpot %s`, string(responseBody)))
	return false, nil
}

func cancelOrdersBinance(key, secret, market, symbol string) bool {
	success, marketType, coin, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if !success {
		return false
	}
	client := binance.NewClient(key, secret)
	var err error
	if market == model.BinanceSpot {
		_, err = client.NewCancelOpenOrdersService().Symbol(dialectSymbol).Do(context.Background())
	} else if market == model.BinanceMargin {
		_, err = client.NewCancelMarginOrderService().Symbol(dialectSymbol).Do(context.Background())
	}
	if err != nil && !strings.Contains(err.Error(), `-2010`) && !strings.Contains(err.Error(), `-2011`) {
		util.Log(util.LogLevelError, "cancelOrdersBinance err: "+err.Error()+" symbol: "+
			symbol+" marketType: "+marketType+" coin: "+coin+" But dialectSymbol: "+dialectSymbol)
		return false
	}
	return true
}

func getBalanceBinanceMargin(key, secret string) (success bool, balances []*model.Balance) {
	client := binance.NewClient(key, secret)
	balanceResp, err := client.NewGetMarginAccountService().Do(context.Background())
	if err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to refresh binance balance `+err.Error()))
		time.Sleep(time.Minute * 5)
		return getBalanceBinanceMargin(key, secret)
	}
	if !balanceResp.TradeEnabled {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`binance margin balance can not trade`))
		return false, balances
	}
	balances = make([]*model.Balance, 0)
	for _, data := range balanceResp.UserAssets {
		if data.Asset == "" {
			continue
		}
		coin := data.Asset
		balance := &model.Balance{
			Market:      model.BinanceMargin,
			Coin:        coin,
			ID:          model.BinanceMargin + `_` + coin + `_` + util.GetNow().Format(time.RFC3339)[0:10],
			BalanceTime: util.GetNow(),
			AccountId:   key}
		if data.Free != "" { // 持仓,此处按照不进行借币计算
			balance.AvailableWithBorrow, _ = strconv.ParseFloat(data.Free, 64)
		}
		if data.Locked != "" {
			lockAmount, _ := strconv.ParseFloat(data.Locked, 64)
			balance.Amount = balance.AvailableWithBorrow + lockAmount
		}
		if balance.UsdValue == 0 && balance.Amount > 0 {
			symbolStandard := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
			_, price := GetPriceForce(model.BinanceMargin, symbolStandard, false)
			balance.UsdValue = balance.Amount * price
		}
		balances = append(balances, balance)
	}
	if len(balances) == 0 {
		util.Log(util.LogLevelError, `fail to refresh binance balance len 0`)
		time.Sleep(time.Second * 5)
		return getBalanceBinanceMargin(key, secret)
	}
	return true, balances
}

func getBalanceBinanceSpot(key string, secret string) (success bool, totalInUsdt float64, balances []*model.Balance) {
	client := binance.NewClient(key, secret)
	balanceResp, err := client.NewGetAccountService().Do(context.Background())
	if err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to refresh binance balance `+err.Error()))
		time.Sleep(time.Minute * 5)
		return getBalanceBinanceSpot(key, secret)
	}
	if !balanceResp.CanTrade {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`binance balance can not trade`))
		return false, 0, balances
	}
	balances = make([]*model.Balance, 0)
	for _, data := range balanceResp.Balances {
		if data.Asset == "" {
			continue
		}
		coin := data.Asset
		balance := &model.Balance{
			Market:      model.BinanceSpot,
			Coin:        coin,
			ID:          model.BinanceSpot + `_` + coin + `_` + util.GetNow().Format(time.RFC3339)[0:10],
			BalanceTime: util.GetNow(),
			AccountId:   key}
		if data.Free != "" { // 持仓,此处按照不进行借币计算
			balance.AvailableWithBorrow, _ = strconv.ParseFloat(data.Free, 64)
		}
		if data.Locked != "" {
			lockAmount, _ := strconv.ParseFloat(data.Locked, 64)
			balance.Amount = balance.AvailableWithBorrow + lockAmount
		}
		if balance.UsdValue == 0 && balance.Amount > 0 {
			symbolStandard := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
			_, price := GetPriceForce(model.BinanceSpot, symbolStandard, false)
			balance.UsdValue = balance.Amount * price
		}
		//if asset[`netAsset`] != nil {
		//	balance.Amount, _ = strconv.ParseFloat(asset[`netAsset`].(string), 64)
		//}
		//if asset[`borrowed`] != nil { //已借数量
		//	balance.Borrow, _ = strconv.ParseFloat(asset[`borrowed`].(string), 64)
		//}
		balances = append(balances, balance)
	}
	if len(balances) == 0 {
		util.Log(util.LogLevelError, `fail to refresh binance balance 0`)
		time.Sleep(time.Second * 5)
		return getBalanceBinanceSpot(key, secret)
	}
	_, btcPrice := GetPriceForce(model.BinanceSpot, `BTC_USDT`, false)
	btcValue := 0.0
	walletResp := signedRequestBinance(key, secret, model.BinanceSpot, http.MethodGet,
		restBinance+`/sapi/v1/asset/wallet/balance`, true, nil)
	if walletResp != nil {
		walletJson, _ := util.NewJSON(walletResp)
		for _, item := range walletJson.MustArray() {
			if item == nil {
				continue
			}
			wallet := item.(map[string]interface{})
			if wallet[`walletName`] != nil && wallet[`walletName`].(string) == `Spot` {
				btcValue, _ = strconv.ParseFloat(wallet[`balance`].(string), 64)
			}
		}
	}
	totalInUsdt = btcValue * btcPrice
	return true, totalInUsdt, balances
}

func getPriceBinanceSpot(key, secret, symbol string) (success bool, price float64) {
	_, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	resp := signedRequestBinance(key, secret, model.BinanceSpot, http.MethodGet,
		restBinance+`/api/v3/avgPrice?symbol=`+dialectSymbol, false, nil)
	if resp != nil {
		respJson, _ := util.NewJSON(resp)
		if respJson != nil {
			success = true
			price, _ = strconv.ParseFloat(respJson.Get(`price`).MustString(), 64)
		}
	}
	return success, price
}
func queryOpenOrdersBinanceSpot(key, secret, symbol string) (orders []*model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if success || symbol == `` {
		orders = make([]*model.Order, 0)
		listOpenOrderService := binance.NewClient(key, secret).NewListOpenOrdersService()
		if symbol != `` {
			listOpenOrderService = listOpenOrderService.Symbol(dialectSymbol)
		}
		resArray, err := listOpenOrderService.Do(context.Background())
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`queryOpenOrdersBinanceSpot err %s %s %s`, symbol, dialectSymbol, err.Error()))
		}
		for _, res := range resArray {
			order := parseOrderBinanceSpotSdk(res)
			orders = append(orders, order)
		}
	}
	return
}

func queryOrderBinanceSpot(key, secret, symbol string, orderId string) (order *model.Order) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.BinanceSpot, symbol)
	if success {
		orderIdInt, _ := strconv.ParseInt(orderId, 10, 64)
		client := binance.NewClient(key, secret)
		orderResp, err := client.NewGetOrderService().Symbol(dialectSymbol).OrderID(orderIdInt).Do(context.Background())
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("queryOrderBinanceSpot err: "+err.Error()))
			return
		}
		order = parseOrderBinanceSpotSdk(orderResp)
	}
	return
}

func parseTransfer(value map[string]interface{}) (balance *model.Balance, external bool) {
	balance = &model.Balance{}
	if value[`id`] != nil {
		balance.ID = value[`id`].(string)
	}
	if value[`coin`] != nil {
		balance.Coin = value[`coin`].(string)
	}
	if value[`amount`] != nil {
		balance.Amount, _ = strconv.ParseFloat(value[`amount`].(string), 64)
	}
	if value[`status`] != nil {
		balance.Status = value[`status`].(json.Number).String()
	}
	if value[`address`] != nil {
		balance.Address = value[`address`].(string)
	}
	if value[`txId`] != nil {
		balance.TransactionId = value[`txId`].(string)
	}
	if value[`applyTime`] != nil {
		balance.CreatedAt, _ = time.Parse(time.DateTime, value[`applyTime`].(string))
	}
	if value[`insertTime`] != nil {
		insertTime, timeErr := value[`insertTime`].(json.Number).Int64()
		if timeErr == nil {
			balance.CreatedAt = time.Unix(insertTime, 0)
		}
	}
	if value[`completeTime`] != nil {
		balance.UpdatedAt, _ = time.Parse(time.DateTime, value[`completeTime`].(string))
	}
	if value[`transferType`] != nil {
		transferType, typeErr := value[`transferType`].(json.Number).Int64()
		if typeErr == nil {
			if transferType == 0 {
				external = true
			}
		}
	}
	return balance, external
}

// GetWithdrawInfo
// status 0:Email Sent,1:Cancelled 2:Awaiting Approval 3:Rejected 4:Processing 5:Failure 6:Completed
// transferType: 1 for internal transfer, 0 for external transfer
func GetWithdrawInfo(market, key, secret string) (balances []*model.Balance) {
	balances = make([]*model.Balance, 0)
	responseBody := signedRequestBinance(key, secret, model.BinanceSpot, http.MethodGet, restBinance+`/sapi/v1/capital/withdraw/history`, true, nil)
	withdrawJson, withdrawErr := util.NewJSON(responseBody)
	if withdrawErr == nil {
		for _, data := range withdrawJson.MustArray() {
			if data == nil {
				continue
			}
			balance, external := parseTransfer(data.(map[string]interface{}))
			if balance != nil && external == true {
				balance.Action = -1
				balance.Market = market
				balances = append(balances, balance)
			}
		}
	}
	responseBody = signedRequestBinance(key, secret, model.BinanceSpot, http.MethodGet, restBinance+`/sapi/v1/capital/deposit/hisrec`, true, nil)
	depositJson, depositErr := util.NewJSON(responseBody)
	if depositErr == nil {
		for _, data := range depositJson.MustArray() {
			if data == nil {
				continue
			}
			balance, external := parseTransfer(data.(map[string]interface{}))
			if balance != nil && external == true {
				balance.Action = 1
				balance.Market = market
				balances = append(balances, balance)
			}
		}
	}
	return balances
}

// GetAccountFromWsAPI 尝试从WebSocket API获取账户信息。
// 该函数根据提供的账户信息、方法名和市场标识来建立请求，并发送给对应的WebSocket连接。
// 参数:
//
//	account - 指向账户信息的指针，包含访问API所需的密钥和秘密。
//	method - 要调用的API方法的名称。 比如v2/account.status
//	market - 市场标识，用于识别特定的WebSocket连接。
func GetAccountFromWsAPI(account *model.Account, method, market string) {
	if market != model.BinancePerp {
		return
	}
	ts := time.Now().UnixMilli()
	requestId := fmt.Sprintf(`%s%d`, method, ts)
	param := url.Values{}
	param.Set(`apiKey`, account.Key)
	param.Set(`timestamp`, fmt.Sprintf(`%d`, ts))
	hash := hmac.New(sha256.New, []byte(account.Secret))
	hash.Write([]byte(param.Encode()))
	msg := fmt.Sprintf(`{"id": "%s","method": "%s","params":{"apiKey": "%s","signature": "%s","timestamp": %d}}`,
		requestId, accountMethodMap[method], account.Key, hex.EncodeToString(hash.Sum(nil)), ts)
	connKey := getPrivateConnKey(market, account.Key, ``)
	value, _ := model.AppEnvironment.ConnOrder.Load(connKey)
	Status := "true"
	if value == nil {
		Status = model.CarryStatusFail
	} else {
		if err := value.(*model.WSConn).WriteMsg([]byte(msg)); err != nil {
			model.AppEnvironment.ConnOrder.Delete(connKey)
			Status = model.CarryStatusFail
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to get account status return: %s`, err.Error()))
		}
	}
	if Status == model.CarryStatusFail {
		HandleWsOrderConnFail(account, market, nil)
	}
}

package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/antihax/optional"
	gateApi "github.com/gateio/gateapi-go/v6"
	gateWs "github.com/gateio/gatews/go"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"time"
)

var apiClientsGate = make(map[string]*gateApi.APIClient)
var apiCtxGate = make(map[string]context.Context)

const wsStepGate = 50

func getClientGate(key, secret string) (apiClient *gateApi.APIClient, ctx context.Context) {
	if apiClientsGate[key] == nil {
		apiClientsGate[key] = gateApi.NewAPIClient(gateApi.NewConfiguration())
		apiCtxGate[key] = context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
			Key: key, Secret: secret})
	}
	return apiClientsGate[key], apiCtxGate[key]
}

func getMarketsGate(key, secret string) (success bool, marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	appendSpotMarketsGate(key, secret, marketInfos)
	appendFutureMarketGate(key, secret, marketInfos)
	return true, marketInfos
}

// 市场minSize按照张数计算的
func appendFutureMarketGate(key, secret string, marketInfos map[string]*model.MarketInfo) {
	client, ctx := getClientGate(key, secret)
	contracts, _, futureErr := client.FuturesApi.ListFuturesContracts(ctx, `usdt`, nil)
	if futureErr != nil {
		panicGateError(key, "ListFuturesContracts", futureErr)
		time.Sleep(time.Minute * 5)
		appendFutureMarketGate(key, secret, marketInfos)
		return
	}
	for _, contract := range contracts {
		if contract.InDelisting {
			continue
		}
		if !contract.EnableCredit {
			continue
		}
		marketInfo := &model.MarketInfo{Market: model.Gate, FundingRateInterval: 8 * 3600000}
		success, coin, symbol := model.GetFromDialect(model.Gate, model.MarketTypePerp, contract.Name)
		if !success {
			continue
		}
		minPrice, _ := strconv.ParseFloat(contract.OrderPriceRound, 64)
		marketInfo.FundingRateInterval = int(contract.FundingInterval) * 1000
		marketInfo.PriceIncrement = minPrice
		marketInfo.PriceDecimal = util.NumDecPlaces(minPrice)
		marketInfo.SizeMin = float64(contract.OrderSizeMin)
		marketInfo.SizeMax = float64(contract.OrderSizeMax)
		marketInfo.Symbol = symbol
		marketInfo.CTCurrency = coin
		marketInfo.CTValue, _ = strconv.ParseFloat(contract.QuantoMultiplier, 64)
		marketInfo.BuyLimitPriceRatio, _ = strconv.ParseFloat(contract.OrderPriceDeviate, 64)
		marketInfo.SellLimitPriceRatio, _ = strconv.ParseFloat(contract.OrderPriceDeviate, 64)
		// 数量颗粒度不随CTValue乘数而变化，故放在处理乘数之前
		marketInfo.SizeIncrement = marketInfo.SizeMin
		if marketInfo.CTValue > 0 {
			marketInfo.SizeMin *= marketInfo.CTValue
			marketInfo.SizeMax *= marketInfo.CTValue
		}
		marketInfos[marketInfo.Symbol] = marketInfo
	}
}

// 市场minSize未根据最小下单美元数进行计算
func appendSpotMarketsGate(key, secret string, marketInfos map[string]*model.MarketInfo) {
	client, ctx := getClientGate(key, secret)
	spotCurrencyPairs, _, spotErr := client.SpotApi.ListCurrencyPairs(ctx)
	if spotErr != nil {
		time.Sleep(time.Minute * 5)
		panicGateError(key, "ListCurrencyPairs", spotErr)
		appendSpotMarketsGate(key, secret, marketInfos)
		return
	}
	for _, spot := range spotCurrencyPairs {
		success, _, symbol := model.GetFromDialect(model.Gate, model.MarketTypeSpot, spot.Id)
		if spot.TradeStatus != "tradable" || !success {
			continue
		}
		marketInfo := &model.MarketInfo{Market: model.Gate, FundingRateInterval: 8 * 3600000}
		marketInfo.Symbol = symbol
		marketInfo.PriceDecimal = int(spot.Precision)
		marketInfo.PriceIncrement = 1 / math.Pow10(int(spot.Precision))
		marketInfo.SizeIncrement = 1 / math.Pow10(int(spot.AmountPrecision))
		if spot.MinQuoteAmount != "" {
			marketInfo.MoneyMin, _ = strconv.ParseFloat(spot.MinQuoteAmount, 64)
			marketInfo.SizeMin = marketInfo.SizeIncrement
		}
		if spot.MinBaseAmount != "" {
			marketInfo.SizeMin, _ = strconv.ParseFloat(spot.MinBaseAmount, 64)
		}
		marketInfos[spot.Id] = marketInfo
	}
}

// setLeverageGate 设置杠杆和risk limit
func setLeverageGate(account *model.Account) (success bool) {
	symbols := GetMarketSymbols(model.Gate)
	for symbol := range symbols {
		_, marketType, _, _ := model.GetFromStandard(model.Gate, symbol)
		if marketType == model.MarketTypePerp {
			setSymbolLeverageGate(account, symbol)
			time.Sleep(time.Millisecond * 200)
		}
	}
	return true
}

type Tier struct {
	LeverageMax float64
	RiskLimit   float64
}

// setSymbolLeverageGate 设置杠杆率和risk limit
// 需要先设置risk再更新杠杆率才能成功
func setSymbolLeverageGate(account *model.Account, symbol string) (success bool) {
	_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	client, ctx := getClientGate(account.Key, account.Secret)
	tiers, _, errTiers := client.FuturesApi.ListRiskLimitTiers(ctx, `usdt`, dialectSymbol)
	if errTiers != nil {
		return false
	}
	tierArray := make([]*Tier, 0)
	for _, tier := range tiers {
		tierLeverage, parseErr := strconv.ParseFloat(tier.LeverageMax, 64)
		if parseErr != nil {
			continue
		}
		tierLimit, _ := strconv.ParseFloat(tier.RiskLimit, 64)
		tierArray = append(tierArray, &Tier{LeverageMax: tierLeverage, RiskLimit: tierLimit})
	}
	for i := 0; i < len(tierArray); i++ {
		for j := len(tierArray) - 1; j > i; j-- {
			if tierArray[j].LeverageMax > tierArray[j-1].LeverageMax {
				tierArray[j], tierArray[j-1] = tierArray[j-1], tierArray[j]
			}
		}
	}
	leverSet := 0.0
	for i := account.GateLeverMax; i >= account.GateLeverMin && leverSet == 0; i-- {
		for j := 0; j < len(tierArray); j++ {
			if tierArray[j].LeverageMax >= i && tierArray[j].RiskLimit >= account.GateRiskLimit {
				leverSet = i
				break
			}
		}
	}
	if leverSet == 0 {
		leverSet = account.GateLeverMin
	}
	limit := account.GateRiskLimit
	insideTiers := false
	for i := len(tierArray) - 1; i >= 0; i-- {
		if tierArray[i].LeverageMax >= leverSet {
			limit = tierArray[i].RiskLimit
			insideTiers = true
			break
		}
	}
	if !insideTiers {
		limit = tierArray[0].RiskLimit
		leverSet = tierArray[0].LeverageMax
	}
	strMaxLimit := strconv.FormatFloat(limit, 'f', 0, 64)
	_, _, errLimit := client.FuturesApi.UpdatePositionRiskLimit(ctx, `usdt`, dialectSymbol, strMaxLimit)
	if errLimit != nil {
		panicGateError(fmt.Sprintf(`%s %s %s`, symbol, strMaxLimit, account.Key), `UpdatePositionRiskLimit`, errLimit)
		return false
	}
	_, _, errLeverage := client.FuturesApi.UpdatePositionLeverage(ctx, `usdt`, dialectSymbol, `0`,
		&gateApi.UpdatePositionLeverageOpts{CrossLeverageLimit: optional.NewString(strconv.FormatFloat(leverSet, 'f', 1, 64))})
	if errLeverage != nil {
		panicGateError(fmt.Sprintf(`%s %f %s`, symbol, leverSet, account.Key), `UpdatePositionLeverage`, errLeverage)
		return false
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`set gate lever and risk %d %s %f %f`,
		account.Index, symbol, leverSet, limit))
	util.StoreSyncMap(&model.AppEnvironment.RiskLimitsGate, limit, account.Key, symbol)
	return true
}

func setPosSideGate(key, secret string) {
	client, ctx := getClientGate(key, secret)
	mode, _, err := client.FuturesApi.SetDualMode(ctx, `usdt`, false)
	if err != nil {
		panicGateError(key, "setPosSideGate", err)
	}
	marshal, _ := json.Marshal(mode)
	util.Log(util.LogLevelInfo, fmt.Sprintf("set gate dual mode success,position: %s", marshal))
}

func setMarginSettingGate(key, secret string) {
	client, ctx := getClientGate(key, secret)
	mode, _, err := client.MarginApi.SetAutoRepay(ctx, "on")
	if err != nil {
		panicGateError(key, "setMarginSettingGate", err)
	}
	marshal, _ := json.Marshal(mode)
	util.Log(util.LogLevelInfo, fmt.Sprintf("set gate margin auto repay success,response: %s", marshal))
}

func TransferGate(key string, secret string, transferType, currency string, amount float64) {
	client, ctx := getClientGate(key, secret)
	param := gateApi.Transfer{Currency: currency, Amount: fmt.Sprintf("%f", amount), Settle: `usdt`}
	if transferType == "MAIN_UMFUTURE" {
		param.From = "spot"
		param.To = "futures"
		_, res, endErr := client.WalletApi.Transfer(ctx, param)
		if endErr != nil {
			if res != nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`fail to transfer status %s`, res.Status))
			}
			panicGateError(key, "transferGate", endErr)
		}
	} else if transferType == "UMFUTURE_MAIN" {
		param.From = "futures"
		param.To = "spot"
		_, res, err := client.WalletApi.Transfer(ctx, param)
		if err != nil {
			panicGateError(key, "transferGate", err)
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to transfer status %s`, res.Status))
		}
	}
}

func panicGateError(key, function string, err error) {
	var e gateApi.GateAPIError
	if errors.As(err, &e) {
		util.Log(util.LogLevelError, fmt.Sprintf("key %s function: %s Gate API error, label: %s, message: %s",
			key, function, e.Label, e.Message))
	}
	util.Log(util.LogLevelError, function+err.Error())
}

var tickerHandler = gateWs.NewCallBack(func(msg *gateWs.UpdateMsg) {
	if msg.Error != nil && (!strings.Contains(msg.Error.Message, "futures.ping") && !strings.Contains(msg.Error.Message, "spot.ping")) {
		util.Log(util.LogLevelError, fmt.Sprintf("callback error in ticker: %s %s", msg.Channel, msg.Error.Error()))
		return
	}
	var bidAsk model.BidAsk
	var symbol string
	success := true
	switch msg.Channel {
	case gateWs.ChannelSpotBookTicker:
		var update gateWs.SpotBookTickerMsg
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("spot book ticker Unmarshal err:%s %s", model.Gate, err.Error()))
			return
		}
		success, _, symbol = model.GetFromDialect(model.Gate, model.MarketTypeSpot, update.CurrencyPair)
		if !success || len(strconv.Itoa(int(update.TimeInMilli))) != 13 {
			return
		}
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(update.Bid, 64)
		bidAmount, _ := strconv.ParseFloat(update.BidSize, 64)
		askPrice, _ := strconv.ParseFloat(update.Ask, 64)
		askAmount, _ := strconv.ParseFloat(update.AskSize, 64)
		bidAsk = model.BidAsk{Ts: int(update.TimeInMilli), TsReceived: now, UpdateId: update.LastId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.Gate, Symbol: symbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.Gate, Symbol: symbol}}}
	// Periodically notify top bids and asks snapshot with limited levels.
	case gateWs.ChannelSpotOrderBook:
		var update gateWs.SpotUpdateAllDepthMsg
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("spot book ticker Unmarshal err:%s %s", model.Gate, err.Error()))
			return
		}
		success, _, symbol = model.GetFromDialect(model.Gate, model.MarketTypeSpot, update.CurrencyPair)
		if !success || len(strconv.Itoa(int(update.TimeInMilli))) != 13 {
			return
		}
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidAsk = model.BidAsk{Ts: int(update.TimeInMilli), TsReceived: now, UpdateId: update.LastUpdateId,
			Bids: []model.Tick{}, Asks: []model.Tick{}}
		getLast, last := model.AppEnvironment.GetBidAsk(model.Gate, symbol)
		for i := 0; i < len(update.Bid) && i < len(update.Ask); i++ {
			var bidPrice, askPrice, bidAmount, askAmount float64
			if update.Bid[0][0] == `` && getLast && len(update.Bid) == 1 && last != nil {
				bidPrice = last.Bids[0].Price
				bidAmount = last.Bids[0].Amount
			} else {
				bidPrice, _ = strconv.ParseFloat(update.Bid[0][0], 64)
				bidAmount, _ = strconv.ParseFloat(update.Bid[0][1], 64)
			}
			if update.Ask[0][0] == `` && getLast && len(update.Ask) == 1 && last != nil {
				askPrice = last.Asks[0].Price
				askAmount = last.Asks[0].Amount
			} else {
				askPrice, _ = strconv.ParseFloat(update.Ask[0][0], 64)
				askAmount, _ = strconv.ParseFloat(update.Ask[0][1], 64)
			}
			bidAsk.Bids = append(bidAsk.Bids, model.Tick{Price: bidPrice, Amount: bidAmount, Market: model.Gate, Symbol: symbol})
			bidAsk.Asks = append(bidAsk.Asks, model.Tick{Price: askPrice, Amount: askAmount, Market: model.Gate, Symbol: symbol})
		}
	// Push best bid and ask in real-time.
	case gateWs.ChannelFutureBookTicker:
		var update gateWs.FuturesBookTicker
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("future book ticker Unmarshal err:%s %s", model.Gate, err.Error()))
			return
		}
		success, _, symbol = model.GetFromDialect(model.Gate, model.MarketTypePerp, update.Contract)
		if !success || len(strconv.Itoa(int(update.TimeMillis))) != 13 {
			return
		}
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(update.BestBidPrice, 64)
		_, bidAmount := model.ParseRealAmount(model.Gate, symbol, float64(update.BestBidSize))
		askPrice, _ := strconv.ParseFloat(update.BestAskPrice, 64)
		_, askAmount := model.ParseRealAmount(model.Gate, symbol, float64(update.BestAskSize))
		bidAsk = model.BidAsk{Ts: int(update.TimeMillis), TsReceived: now, UpdateId: update.UpdateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.Gate, Symbol: symbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.Gate, Symbol: symbol}}}
	}
	markets := model.AppEnvironment
	if markets.SetBidAsk(model.Gate, symbol, &bidAsk) {
		funcHandlers := GetFunctions(model.Gate, symbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), model.Gate, symbol)
				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
					go value.(model.CarryHandler)(setting, &bidAsk)
				}
				return true
			})
		}
	}
})

var markPriceHandler = gateWs.NewCallBack(func(msg *gateWs.UpdateMsg) {
	if msg.Error != nil && (!strings.Contains(msg.Error.Message, "futures.ping") && !strings.Contains(msg.Error.Message, "spot.ping")) {
		util.Log(util.LogLevelError, fmt.Sprintf("callback error: %s %s", msg.Channel, msg.Error.Error()))
		return
	}
	if msg.Channel == gateWs.ChannelFutureTicker {
		var tickers []gateWs.FuturesTicker
		if err := json.Unmarshal(msg.Result, &tickers); err != nil {
			//util.LogLess(util.LogLevelError, fmt.Sprintf("future mark price Unmarshal err:%s %s %s", model.Gate, err.Error(), msg.Result))
			return
		}
		for _, update := range tickers {
			success, _, symbol := model.GetFromDialect(model.Gate, model.MarketTypePerp, update.Contract)
			if !success {
				return
			}
			price, _ := strconv.ParseFloat(update.MarkPrice, 64)
			ticker := &model.MarkPriceInfo{MarkPrice: price, Ts: int(msg.TimeMs)}
			model.AppEnvironment.SetMarkPriceInfo(symbol, model.Gate, ticker)
			rate, _ := strconv.ParseFloat(update.FundingRate, 64)
			rateNext, _ := strconv.ParseFloat(update.FundingRateIndicative, 64)
			SetFundingRate(model.Gate, symbol, &model.FundingRate{Rate: rate, RateNext: rateNext,
				UpdateTime: time.UnixMilli(msg.TimeMs), ExpireTime: 0}) // futures.tickers中未返回结算时间
		}
	}
	return
})

var wsPriHandlerGateUnified = func(market, key string, msg []byte) {
	responseJson, err := util.NewJSON(msg)
	if err != nil || responseJson == nil {
		return
	}
	if responseJson.Get(`channel`).MustString() != `unified.assets` {
		return
	}
	value := responseJson.GetPath(`result`).MustMap()
	collateral := &model.Collateral{AccountKey: key}
	if value[`e`] == nil {
		return
	} else {
		collateral.AccountValueInU, _ = strconv.ParseFloat(value[`e`].(string), 64)
	}
	if value[`a`] != nil {
		collateral.Available, _ = strconv.ParseFloat(value[`a`].(string), 64)
	}
	if value[`R`] != nil {
		collateral.Rate, _ = strconv.ParseFloat(value[`R`].(string), 64)
	}
	//util.Log(util.LogLevelInfo, fmt.Sprintf("gate unified %s %f", collateral.AccountKey, collateral.Available))
	model.CollateralHandler(key, ``, false, collateral)
}

var wsPriHandlerGateSpot = func(market, key string, msg []byte) {
	responseJson, err := util.NewJSON(msg)
	if err != nil || responseJson == nil {
		return
	}
	if responseJson.Get(`ack`).MustBool() {
		return
	}
	channel := responseJson.Get(`channel`).MustString()
	//ts := responseJson.Get(`time_ms`).MustInt64()
	result := responseJson.GetPath(`header`, `status`).MustString()
	connKey := getPrivateConnKey(model.Gate, key, model.MarketTypeSpot)
	valueSpot, _ := model.AppEnvironment.ConnOrder.Load(connKey)
	if channel == `spot.ping` {
		if valueSpot == nil {
			return
		}
		err := valueSpot.(*model.WSConn).WriteMsg([]byte(fmt.Sprintf(
			`{"time" : %d, "channel" : "spot.pong"}`, time.Now().Unix())))
		if err != nil {
			model.AppEnvironment.ConnOrder.Delete(connKey)
			return
		}
	} else if channel == `spot.orders` {
		data := responseJson.Get(`result`).MustArray()
		for _, datum := range data {
			value := datum.(map[string]interface{})
			orderId := value[`id`].(string)
			status := model.CarryStatusWorking
			if value[`finish_as`] == `filled` {
				status = model.CarryStatusSuccess
			}
			if value[`filled_amount`] != nil {
				deal, _ := strconv.ParseFloat(value[`filled_amount`].(string), 64)
				UpdateOrderDeal(market, orderId, ``, status, string(msg), deal)
			}
			//dealPrice, _ := strconv.ParseFloat(value[`avg_deal_price`].(string), 64)
		}
	} else if channel == `spot.balances` {
		//https://www.gate.io/docs/developers/apiv4/ws/zh_CN/#%E5%AE%A2%E6%88%B7%E7%AB%AF%E8%AE%A2%E9%98%85-12
		util.LogLess(util.LogLevelInfo, "risk check ws update balances gate "+string(msg))
	} else {
		channel = responseJson.GetPath(`header`, `channel`).MustString()
		if channel == `spot.order_place` {
			if responseJson.GetPath(`header`, `status`).MustString() == `400` {
				requestId := responseJson.Get(`request_id`).MustString()
				wsResp := model.WSResp{RequestId: requestId, Success: false, Msg: responseJson.GetPath(`data`, `errs`, `message`).MustString()}
				model.AppEnvironment.WSRespChan <- wsResp
				if responseJson.GetPath(`data`, `errs`, `label`).MustString() == `AUTHENTICATION_FAILED` {
					account := model.AppConfig.GetAccountFromKeyIndex(model.Gate, key, -1)
					if valueSpot == nil {
						return
					}
					wsLoginGateOrder(account, valueSpot.(*model.WSConn), model.MarketTypeSpot)
				}
			} else if !responseJson.Get(`ack`).MustBool() {
				requestId := responseJson.Get(`request_id`).MustString()
				wsResp := model.WSResp{RequestId: requestId, OrderId: responseJson.GetPath(`data`, `result`, `id`).MustString()}
				if result == `200` {
					wsResp.Success = true
				} else {
					wsResp.Success = false
					wsResp.Msg = responseJson.GetPath(`data`, `errs`, `message`).MustString()
				}
				model.AppEnvironment.WSRespChan <- wsResp
			}
		} else if channel == `spot.login` {
			if result == `200` {
				if valueSpot == nil {
					return
				}
				subscribePrivateGateSpot(valueSpot.(*model.WSConn), connKey)
			}
		}
	}
}

func subscribePrivateGateSpot(conn *model.WSConn, connKey string) {
	msgSend := fmt.Sprintf(`{"time":%d,"channel":"spot.orders","event":"subscribe","payload":["!all"]}`, time.Now().Unix())
	err := conn.WriteMsg([]byte(msgSend))
	if err != nil {
		model.AppEnvironment.ConnOrder.Delete(connKey)
		return
	}
	msgSend = fmt.Sprintf(`{"time":%d,"channel":"spot.balances","event":"subscribe"}`, time.Now().Unix())
	err = conn.WriteMsg([]byte(msgSend))
	if err != nil {
		model.AppEnvironment.ConnOrder.Delete(connKey)
		return
	}
}

func maintainConnsGate(accounts []*model.Account) {
	for _, account := range accounts {
		model.AppEnvironment.PriConnecting.Store(model.Gate+model.MarketTypeSpot+account.Key, false)
		model.AppEnvironment.PriConnecting.Store(model.Gate+model.MarketTypePerp+account.Key, false)
	}
	loginTSSpot := time.Now().Unix()
	loginTSPerp := time.Now().Unix()
	for {
		connTick, _ := model.AppEnvironment.ConnTick.Load(GetPublicConnKey(model.Gate, model.MarketTypeSpot))
		if connTick != nil {
			if err := SendToConnections(model.Gate, connTick.(map[*model.WSConn]bool),
				util.JsonEncodeToByte(map[string]interface{}{"time": time.Now().Unix(), "channel": "spot.ping"})); err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("tick conn maintain error %s %s", model.Gate, err.Error()))
			}
		}
		connTickPerp, _ := model.AppEnvironment.ConnTick.Load(GetPublicConnKey(model.Gate, model.MarketTypePerp))
		if connTickPerp != nil {
			if err := SendToConnections(model.Gate, connTickPerp.(map[*model.WSConn]bool),
				util.JsonEncodeToByte(map[string]interface{}{"time": time.Now().Unix(), "channel": "futures.ping"})); err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("tick conn maintain error %s %s", model.Gate, err.Error()))
			}
		}
		for _, account := range accounts {
			successSpot := true
			connKeySpot := getPrivateConnKey(model.Gate, account.Key, model.MarketTypeSpot)
			wsSpot, _ := model.AppEnvironment.ConnOrder.Load(connKeySpot)
			if wsSpot != nil {
				if err := wsSpot.(*model.WSConn).WriteMsg([]byte(fmt.Sprintf(`{"time": %d, "channel" : "spot.ping"}`, time.Now().Unix()))); err != nil {
					successSpot = false
					model.AppEnvironment.ConnOrder.Delete(connKeySpot)
					//wsSpot.(*model.WSConn).Close()
					util.Log(util.LogLevelError, fmt.Sprintf("send account spot ping message err:%s %s", model.Gate, err.Error()))
				}
				if time.Now().Unix()-loginTSSpot > 600 {
					wsLoginGateOrder(account, wsSpot.(*model.WSConn), model.MarketTypeSpot)
					loginTSSpot = time.Now().Unix()
				}
			} else {
				successSpot = false
			}
			if !successSpot {
				go WSOrderServeGate(account, model.MarketTypeSpot)
			}
			successPerp := true
			connKeyPerp := getPrivateConnKey(model.Gate, account.Key, model.MarketTypePerp)
			wsFuture, _ := model.AppEnvironment.ConnOrder.Load(connKeyPerp)
			if wsFuture != nil {
				pingMsg := fmt.Sprintf(`{"time": %d, "channel" : "futures.ping"}`, time.Now().Unix())
				if err := wsFuture.(*model.WSConn).WriteMsg([]byte(pingMsg)); err != nil {
					model.AppEnvironment.ConnOrder.Delete(connKeyPerp)
					util.Log(util.LogLevelError, fmt.Sprintf("send account futures ping message err:%s %s", model.Gate, err.Error()))
					//wsFuture.(*model.WSConn).Close()
					successPerp = false
				}
				if time.Now().Unix()-loginTSPerp > 600 {
					wsLoginGateOrder(account, wsFuture.(*model.WSConn), model.MarketTypePerp)
					loginTSPerp = time.Now().Unix()
				}
			} else {
				successPerp = false
			}
			if !successPerp {
				go WSOrderServeGate(account, model.MarketTypePerp)
			}
		}
		time.Sleep(time.Second * 20)
	}
}

func wsLoginUnified(account *model.Account, conn *model.WSConn) (success bool) {
	if conn == nil {
		return false
	}
	hash := hmac.New(sha512.New, []byte(account.Secret))
	ts := time.Now().Unix()
	hash.Write([]byte(fmt.Sprintf(`channel=unified.assets&event=subscribe&time=%d`, ts)))
	sign := hex.EncodeToString(hash.Sum(nil))
	msg := fmt.Sprintf(`{"time":%d,"channel":"unified.assets","event":"subscribe","payload":[],"auth":
			{"method":"api_key","KEY":"%s","SIGN": "%s"}}`, ts, account.Key, sign)
	if err := conn.WriteMsg([]byte(msg)); err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf("send account unified login message err: %s %s", model.Gate, err.Error()))
		return
	}
	return true
}

func wsLoginGateOrder(account *model.Account, conn *model.WSConn, marketType string) (success bool) {
	if conn == nil {
		return false
	}
	logInCode := ``
	if marketType == model.MarketTypeSpot {
		logInCode = `spot`
	} else if marketType == model.MarketTypePerp {
		logInCode = `futures`
	}
	ts := time.Now().Unix()
	hash := hmac.New(sha512.New, []byte(account.Secret))
	hash.Write([]byte(fmt.Sprintf("api\n%s.login\n\n%d", logInCode, ts)))
	sign := hex.EncodeToString(hash.Sum(nil))
	msg := fmt.Sprintf(`{"time": %d,"channel": "%s.login","event": "api","payload": {"api_key": "%s",
    		"signature": "%s","timestamp": "%d","req_id": "request%d"}}`, ts, logInCode, account.Key, sign, ts, ts)
	if err := conn.WriteMsg([]byte(msg)); err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf("send account login message err: %s %s %s", model.Gate, marketType, err.Error()))
		return
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf("log in conn %s %s %s", model.Gate, marketType, msg))
	return true
}

func WSOrderServeGate(account *model.Account, marketType string) {
	if account == nil {
		return
	}
	replaced := model.AppEnvironment.PriConnecting.CompareAndSwap(model.Gate+marketType+account.Key, false, true)
	if !replaced {
		return
	}
	defer func() {
		select {
		case <-time.After(time.Second * 3):
		}
		model.AppEnvironment.PriConnecting.Store(model.Gate+marketType+account.Key, false)
	}()
	connKey := getPrivateConnKey(model.Gate, account.Key, marketType)
	if marketType == model.MarketTypeSpot {
		conn, err := model.WsPrivateClient(account, &model.AppEnvironment.ConnOrder, connKey, model.Gate, gateWs.BaseUrl, wsPriHandlerGateSpot, false)
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("get private conn err: %s %s %s", model.Gate, marketType, err.Error()))
			return
		}
		if wsLoginGateOrder(account, conn, marketType) {
			model.AppEnvironment.ConnOrder.Store(connKey, conn)
		}
	} else if marketType == model.MarketTypePerp {
		conn, err := model.WsPrivateClient(account, &model.AppEnvironment.ConnOrder, connKey, model.Gate, gateWs.FuturesUsdtUrl, wsPriHandlerGatePerp, false)
		if wsLoginGateOrder(account, conn, marketType) {
			model.AppEnvironment.ConnOrder.Store(connKey, conn)
		}
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("get private conn err: %s %s %s", model.Gate, marketType, err.Error()))
			return
		}
		conn, err = model.WsPrivateClient(account, &model.AppEnvironment.ConnOrderUpdate, connKey, model.Gate, model.UnifiedUrlGate, wsPriHandlerGateUnified, true)
		if wsLoginUnified(account, conn) {
			model.AppEnvironment.ConnOrderUpdate.Store(connKey, conn)
		}
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf("gate wsAccount unified connect err: %s %s", err.Error(), account.Key))
			return
		}
	}
}

func WsTickServeGateSpot(market string) (socketMap map[*model.WSConn]bool, connectErr error) {
	var spotSubs []interface{}
	symbols := GetMarketSymbols(market)
	for symbol := range symbols {
		if strings.LastIndex(symbol, model.UniStandardTail[model.MarketTypeSpot]) == len(symbol)-len(model.UniStandardTail[model.MarketTypeSpot]) &&
			len(symbol)-len(model.UniStandardTail[model.MarketTypeSpot]) > 0 {
			spotSubs = append(spotSubs, symbol)
			//spotOrderBookSubs = append(spotOrderBookSubs, []string{symbol, "5", "100ms"})
		}
	}
	//spotOrderBookSockets, spotOrderBookChannels, spotOrderBookErr := WsPublicClient(model.Gate, gateWs.BaseUrl, spotOrderBookSubs, subscribeHandler, wsHandlerGate, wsStepGate)
	//if spotOrderBookErr == nil {
	//	util.Info(`finish connect public gate spot order book ws `)
	//	msgChans = append(msgChans, spotOrderBookChannels...)
	//	for conn, b := range spotOrderBookSockets {
	//		socketMap[conn] = b
	//	}
	//}
	return model.WsPublicClient(model.Gate, gateWs.BaseUrl, spotSubs, subscribeHandler, wsHandlerGate, wsStepGate, false)
}

func WsTickServeGatePerp(market string) (socketMap map[*model.WSConn]bool, connectErr error) {
	var futureSubs []interface{}
	socketMap = make(map[*model.WSConn]bool)
	symbols := GetMarketSymbols(market)
	for symbol := range symbols {
		if strings.LastIndex(symbol, model.UniStandardTail[model.MarketTypePerp]) == len(symbol)-len(model.UniStandardTail[model.MarketTypePerp]) &&
			len(symbol)-len(model.UniStandardTail[model.MarketTypePerp]) > 0 {
			futureSubs = append(futureSubs, symbol)
		}
	}
	return model.WsPublicClient(model.Gate, gateWs.FuturesUsdtUrl, futureSubs, subscribeHandler, wsHandlerGate, wsStepGate, false)
}

var wsHandlerGate = func(market string, conn *model.WSConn, event []byte) {
	respJson, _ := util.NewJSON(event)
	if respJson != nil {
		channel := respJson.Get(`channel`).MustString()
		if strings.Contains(channel, `ping`) {
			strBack := `spot.pong`
			if strings.Contains(channel, `futures`) {
				strBack = `futures.pong`
			}
			if conn != nil {
				err := conn.WriteMsg([]byte(fmt.Sprintf(`{"time" : %d, "channel" : "%s"}`, time.Now().Unix(), strBack)))
				if err != nil {
					return
				}
				util.Log(util.LogLevelInfo, fmt.Sprintf("send ping message to channel %s %s from %s", market, channel, string(event)))
			}
			return
		}
	}
	msg := &gateWs.UpdateMsg{}
	if err := json.Unmarshal(event, msg); err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf("gate ws message Unmarshal err:%s", err.Error()))
		return
	}
	if msg.Channel == gateWs.ChannelFutureTicker {
		markPriceHandler(msg)
	} else {
		tickerHandler(msg)
	}
}

var subscribeHandler = func(market string, connection *model.WSConn, subscribes []interface{}) error {
	var err error = nil
	switch subscribes[0].(type) {
	case string: //ticker订阅
		if strings.LastIndex(subscribes[0].(string), model.UniStandardTail[model.MarketTypePerp]) ==
			len(subscribes[0].(string))-len(model.UniStandardTail[model.MarketTypePerp]) &&
			len(subscribes[0].(string))-len(model.UniStandardTail[model.MarketTypePerp]) > 0 { //合约ticker订阅
			var symbols []string
			for _, subscribe := range subscribes {
				_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, subscribe.(string))
				symbols = append(symbols, dialectSymbol)
			}
			if err = connection.WriteMsg(util.JsonEncodeToByte(map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "futures.book_ticker",
				"event":   "subscribe",
				"payload": symbols,
			})); err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("gate can not subscribe gate futures.book_ticker %v %s", symbols, err.Error()))
			}
			if err = connection.WriteMsg(util.JsonEncodeToByte(map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "futures.tickers",
				"event":   "subscribe",
				"payload": symbols,
			})); err != nil {
				util.Log(util.LogLevelInfo, fmt.Sprintf("gate can not subscribe gate futures.tickers %v %s", symbols, err.Error()))
			}
		} else { //现货ticker订阅
			var symbols []string
			for _, subscribe := range subscribes {
				_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, subscribe.(string))
				symbols = append(symbols, dialectSymbol)
			}
			subscribeMap := map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "spot.book_ticker",
				"event":   "subscribe",
				"payload": symbols,
			}
			subscribeMessage := util.JsonEncodeToByte(subscribeMap)
			if err = connection.WriteMsg(subscribeMessage); err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("gate can not subscribe spot symbols %s %s", subscribeMessage, err.Error()))
			}
		}
	case []string: //orderbook订阅
		for _, subscribe := range subscribes {
			_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, subscribe.([]string)[0])
			subscribe.([]string)[0] = dialectSymbol
			subscribeMap := map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "spot.order_book",
				"event":   "subscribe",
				"payload": subscribe,
			}
			subscribeMessage := util.JsonEncodeToByte(subscribeMap)
			if err = connection.WriteMsg(subscribeMessage); err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("gate can not subscribe spot order book symbol %s %s", subscribeMessage, err.Error()))
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	return err
}

func parseOrderGatePerp(gateOrder *gateApi.FuturesOrder) (order *model.Order) {
	if gateOrder == nil {
		return nil
	}
	_, _, symbol := model.GetFromDialect(model.Gate, model.MarketTypePerp, gateOrder.Contract)
	order = &model.Order{Market: model.Gate, OrderId: fmt.Sprintf(`%d`, gateOrder.Id), Symbol: symbol}
	order.OrderTime = time.Unix(int64(gateOrder.CreateTime), 0)
	order.UpdatedAt = time.Unix(int64(gateOrder.FinishTime), 0)
	if gateOrder.Status == `open` {
		order.Status = model.CarryStatusWorking
	} else if gateOrder.Status == `finished` {
		switch gateOrder.FinishAs {
		case `filled`:
			order.Status = model.CarryStatusSuccess
		case `cancelled`, `liquidated`, `ioc`, `auto_deleveraged`, `reduce_only`, `position_closed`, `reduce_out`:
			order.Status = model.CarryStatusFail
		}
	}
	order.DealPrice, _ = strconv.ParseFloat(gateOrder.FillPrice, 64)
	if gateOrder.Size > 0 {
		order.OrderSide = model.OrderSideBuy
	} else if gateOrder.Size < 0 {
		order.OrderSide = model.OrderSideSell
	}
	_, order.Amount = model.ParseRealAmount(model.Gate, order.Symbol, math.Abs(float64(gateOrder.Size)))
	_, left := model.ParseRealAmount(model.Gate, order.Symbol, math.Abs(float64(gateOrder.Left)))
	order.DealAmount = order.Amount - left
	price, _ := strconv.ParseFloat(gateOrder.Price, 64)
	if price > 0 {
		order.Price = price
	}
	return order
}
func parseOrderGateSpot(gateOrder *gateApi.Order) (order *model.Order) {
	if gateOrder == nil {
		return nil
	}
	order = &model.Order{Market: model.Gate, OrderId: gateOrder.Id}
	order.OrderTime = time.UnixMilli(gateOrder.CreateTimeMs)
	if gateOrder.Side == "buy" {
		order.OrderSide = model.OrderSideBuy
	} else if gateOrder.Side == "sell" {
		order.OrderSide = model.OrderSideSell
	}
	// Order status  - `open`: to be filled - `closed`: filled - `cancelled`: cancelled
	switch gateOrder.Status {
	case `open`:
		order.Status = model.CarryStatusWorking
	case `closed`:
		if gateOrder.FinishAs == `filled` {
			order.Status = model.CarryStatusSuccess
		} else {
			order.Status = model.CarryStatusFail
		}
	case `cancelled`:
		order.Status = model.CarryStatusFail
	}
	intCreateTime, _ := strconv.ParseInt(gateOrder.CreateTime, 10, 64)
	intUpdateTime, _ := strconv.ParseInt(gateOrder.UpdateTime, 10, 64)
	order.OrderTime = time.Unix(intCreateTime, 0)
	order.OrderUpdateTime = time.Unix(intUpdateTime, 0)
	// Order Type    - limit : Limit Order - market : Market Order
	if gateOrder.Type == `limit` {
		order.OrderType = model.OrderTypeLimit
	} else if gateOrder.Type == `market` {
		order.OrderType = model.OrderTypeMarket
	}
	order.Price, _ = strconv.ParseFloat(gateOrder.Price, 64)
	_, _, order.Symbol = model.GetFromDialect(model.Gate, model.MarketTypeSpot, gateOrder.CurrencyPair)
	// When `type` is limit, it refers to base currency.  For instance, `BTC_USDT` means `BTC`
	// When `type` is `market`, it refers to different currency according to `side`
	//- `side` : `buy` means quote currency, `BTC_USDT` means `USDT`
	//- `side` : `sell` means base currency，`BTC_USDT` means `BTC`
	order.Amount, _ = strconv.ParseFloat(gateOrder.Amount, 64)
	if order.OrderType == model.OrderTypeMarket && order.OrderSide == model.OrderSideBuy {
		order.Amount /= order.Price
	}
	order.DealAmount, _ = strconv.ParseFloat(gateOrder.FilledAmount, 64)
	order.DealAmount = math.Abs(order.DealAmount)
	order.DealPrice, _ = strconv.ParseFloat(gateOrder.AvgDealPrice, 64)
	order.Fee, _ = strconv.ParseFloat(gateOrder.Fee, 64)
	return order
}

func cancelAllGate(key, secret string) {
	client, ctx := getClientGate(key, secret)
	futureOrders, _, queryErr := client.FuturesApi.ListFuturesOrders(ctx, `usdt`, `open`, nil)
	if queryErr != nil {
		panicGateError(key, `cancelAllGate perp query`, queryErr)
	} else {
		contracts := make(map[string]bool)
		for _, order := range futureOrders {
			contracts[order.Contract] = true
		}
		for contract := range contracts {
			cancelOrders, _, errPerp := client.FuturesApi.CancelFuturesOrders(ctx, `usdt`, contract, nil)
			if errPerp != nil {
				panicGateError(key, `cancelAllGate perp`, errPerp)
			} else {
				util.Log(util.LogLevelInfo, fmt.Sprintf("cancelAll orders success gate perp cancel %s %d", contract, len(cancelOrders)))
			}
		}
	}
	spotOrders, _, err := client.SpotApi.ListAllOpenOrders(ctx, nil)
	if err != nil {
		panicGateError(key, `queryOpenOrdersGate spot`, err)
	}
	for _, order := range spotOrders {
		cancelOrders, _, errSpot := client.SpotApi.CancelOrders(ctx, order.CurrencyPair,
			&gateApi.CancelOrdersOpts{Account: optional.NewString("spot")})
		if errSpot != nil {
			panicGateError(key, fmt.Sprintf("cancelAllGate %#v", cancelOrders), err)
		} else {
			util.Log(util.LogLevelInfo, fmt.Sprintf(
				"cancelAll orders success gate spot cancel %s %d", order.CurrencyPair, len(cancelOrders)))
		}
	}
}

func queryOpenOrdersGate(key, secret, symbol string) (orders []*model.Order) {
	client, ctx := getClientGate(key, secret)
	gateOrders, _, err := client.SpotApi.ListAllOpenOrders(ctx, nil)
	if err != nil {
		panicGateError(key, `queryOpenOrdersGate spot`, err)
	}
	orders = make([]*model.Order, 0)
	_, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	if marketType == model.MarketTypeSpot {
		for _, gateOrder := range gateOrders {
			if gateOrder.CurrencyPair == dialectSymbol {
				for _, data := range gateOrder.Orders {
					order := parseOrderGateSpot(&data)
					if order != nil {
						orders = append(orders, order)
					}
				}
			}
		}
	} else if marketType == model.MarketTypePerp {
		futureOrders, _, futureErr := client.FuturesApi.ListFuturesOrders(ctx, `usdt`, `open`,
			&gateApi.ListFuturesOrdersOpts{Contract: optional.NewString(dialectSymbol)})
		if futureErr != nil {
			panicGateError(key, `queryOpenOrdersGate future`, futureErr)
		}
		for _, futureOrder := range futureOrders {
			orders = append(orders, parseOrderGatePerp(&futureOrder))
		}
	}
	return
}

func getBalanceGate(key, secret string) (success bool, balances []*model.Balance, totalInUsd float64, collateral *model.Collateral) {
	client, ctx := getClientGate(key, secret)
	portfolioAccount, _, portfolioErr := client.UnifiedApi.ListUnifiedAccounts(ctx, nil)
	if portfolioErr != nil {
		panicGateError(key, "getBalanceGate", portfolioErr)
		time.Sleep(time.Minute * 5)
		util.Log(util.LogLevelError, `fail to refresh balance gate`)
		return getBalanceGate(key, secret)
	}
	if portfolioAccount.Locked {
		util.Log(util.LogLevelInfo, "portfolio account is locked")
		return false, balances, 0, nil
	}
	totalInUsd, _ = strconv.ParseFloat(portfolioAccount.UnifiedAccountTotalEquity, 64)
	collateralAvailable, _ := strconv.ParseFloat(portfolioAccount.TotalAvailableMargin, 64)
	totalMaintenanceMargin, _ := strconv.ParseFloat(portfolioAccount.TotalMaintenanceMargin, 64)
	maintenanceRate, _ := strconv.ParseFloat(portfolioAccount.TotalMaintenanceMarginRate, 64)
	collateral = &model.Collateral{Available: collateralAvailable, Occupied: totalMaintenanceMargin, Rate: maintenanceRate}
	balances = make([]*model.Balance, 0)
	for coin, item := range portfolioAccount.Balances {
		balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Gate, Coin: coin}
		balance.FrozenAmount, _ = strconv.ParseFloat(item.Freeze, 64)
		balance.Borrow, _ = strconv.ParseFloat(item.Borrowed, 64)
		balance.AvailableWithBorrow, _ = strconv.ParseFloat(item.Available, 64)
		balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
		canBorrow := 0.0 //不允许借币
		balance.AvailableWithBorrow = math.Max(0, balance.Amount) + canBorrow
		_, price := GetPriceForce(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.Gate)
		balance.UsdValue = balance.Amount * price
		balances = append(balances, balance)
	}
	if len(balances) == 0 {
		util.Log(util.LogLevelInfo, "balances is empty gate")
		return getBalanceGate(key, secret)
	}
	return true, balances, totalInUsd, collateral
}

func getPositionsGate(key string, secret string) (success bool, positions []*model.Position) {
	client, ctx := getClientGate(key, secret)
	positionList, _, positionsErr := client.FuturesApi.ListPositions(ctx, `usdt`, nil)
	if positionsErr != nil {
		panicGateError(key, `getPositionsGate`, positionsErr)
		time.Sleep(time.Minute)
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to refresh future balance gate %d`, len(positionList)))
		return getPositionsGate(key, secret)
	}
	positions = make([]*model.Position, 0)
	for _, item := range positionList {
		getCoin, _, symbol := model.GetFromDialect(model.Gate, model.MarketTypePerp, item.Contract)
		if !getCoin {
			continue
		}
		position := &model.Position{Market: model.Gate, Ts: util.GetNowUnixMillion(), Currency: symbol}
		_, realAmount := model.ParseRealAmount(model.Gate, symbol, float64(item.Size))
		position.Holding = realAmount
		position.LeverRate, _ = strconv.ParseInt(item.CrossLeverageLimit, 10, 64)
		position.EntryPrice, _ = strconv.ParseFloat(item.EntryPrice, 64)
		position.Margin, _ = strconv.ParseFloat(item.Margin, 64)
		position.LiquidationPrice, _ = strconv.ParseFloat(item.LiqPrice, 64)
		position.ProfitUnreal, _ = strconv.ParseFloat(item.UnrealisedPnl, 64)
		position.MinimumMaintenanceMargin, _ = strconv.ParseFloat(item.MaintenanceMargin, 64)
		currentMargin, _ := strconv.ParseFloat(item.Margin, 64)        //当前保证金，盈亏也算在里面
		initialMargin, _ := strconv.ParseFloat(item.InitialMargin, 64) //初始保证金
		position.Margin = math.Max(currentMargin, initialMargin)
		position.RiskLimit, _ = strconv.ParseFloat(item.RiskLimit, 64)
		// 由于需要获取某个币种的风险限额，所以无论是否有holding都要保存position
		positions = append(positions, position)
	}
	if len(positions) == 0 {
		time.Sleep(time.Second * 5)
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to refresh future positions gate %d`, len(positionList)))
		return getPositionsGate(key, secret)
	}
	return true, positions
}

// getPriceGate
func _(key, secret, symbol string) (success bool, price float64) {
	client, ctx := getClientGate(key, secret)
	_, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	if marketType == model.MarketTypeSpot {
		param := &gateApi.ListTickersOpts{CurrencyPair: optional.NewString(dialectSymbol)}
		tickers, _, _ := client.SpotApi.ListTickers(ctx, param)
		if tickers != nil && len(tickers) > 0 {
			tickerBytes, err := json.Marshal(tickers[0].Last)
			if err == nil {
				str := strings.Replace(string(tickerBytes), `"`, ``, -1)
				price, err = strconv.ParseFloat(str, 64)
				return err == nil, price
			}
		}
	} else if marketType == model.MarketTypePerp {
		contract, _, err := client.FuturesApi.GetFuturesContract(ctx, `usdt`, dialectSymbol)
		if err == nil {
			price, err = strconv.ParseFloat(contract.LastPrice, 64)
			return err == nil, price
		}
	}
	return false, 0
}

func cancelOrderGate(key, secret, symbol, orderId string) (result bool) {
	client, ctx := getClientGate(key, secret)
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	if success && marketType == model.MarketTypeSpot {
		param := &gateApi.CancelOrderOpts{}
		param.Account = optional.NewString("spot")
		order, _, err := client.SpotApi.CancelOrder(ctx, orderId, dialectSymbol, param)
		if err != nil {
			panicGateError(key, fmt.Sprintf("cancelSpotOrdersGate %s %s %#v", dialectSymbol, orderId, order), err)
			return false
		}
		marshal, _ := json.Marshal(order)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`fail to cancel order gate spot response: %s`, marshal))
		return true
	} else if success && marketType == model.MarketTypePerp {
		order, _, err := client.FuturesApi.CancelFuturesOrder(ctx, `usdt`, orderId)
		if err != nil {
			panicGateError(key, fmt.Sprintf(`cancelFutureOrderGate %#v`, order), err)
			return false
		}
		marshal, _ := json.Marshal(order)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`fail to cancel order gate future order response: %s`, marshal))
		return true
	}
	util.Log(util.LogLevelError, fmt.Sprintf(`cancel can not recognize gate symbol %s`, dialectSymbol))
	return false
}

func cancelOrdersGate(key string, secret string, symbol string) (result bool) {
	client, ctx := getClientGate(key, secret)
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	if success && marketType == model.MarketTypeSpot {
		param := &gateApi.CancelOrdersOpts{Account: optional.NewString("spot")}
		orders, _, err := client.SpotApi.CancelOrders(ctx, dialectSymbol, param)
		if err != nil {
			panicGateError(key, fmt.Sprintf("cancelOrdersGate %#v", orders), err)
			return false
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`cancelOrdersGate response: %#v`, orders))
		return true
	} else if success && marketType == model.MarketTypePerp {
		orders, _, err := client.FuturesApi.CancelFuturesOrders(ctx, `usdt`, dialectSymbol, nil)
		if err != nil {
			panicGateError(key, fmt.Sprintf("cancelFutureOrdersGate %#v", orders), err)
			return false
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`cancel future orders response: %d %#v`, len(orders), orders))
		return true
	}
	util.Log(util.LogLevelError, fmt.Sprintf(`cancel orders can not recognize gate symbol %s`, symbol))
	return false
}

func placeOrderGate(account *model.Account, isWs bool, order *model.Order, orderSide, orderType, orderParam, symbol string, price, amount float64) {
	client, ctx := getClientGate(account.Key, account.Secret)
	orderPrice, decimal := model.FormatPrice(model.Gate, symbol, price)
	orderPriceStr := util.CutTailZero(strconv.FormatFloat(orderPrice, 'f', decimal, 64))
	tif := `gtc`
	if orderType == model.OrderTypeMarket {
		orderPriceStr = `0`
		tif = `ioc`
	}
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	order.Symbol = symbol
	order.Price = orderPrice
	if !success {
		return
	}
	ts := time.Now().Unix()
	if marketType == model.MarketTypeSpot {
		relatedOrder := gateApi.Order{Price: orderPriceStr, Side: orderSide, CurrencyPair: dialectSymbol, Type: orderType, TimeInForce: tif, AutoRepay: true}
		relatedOrder.Account = "spot"
		formattedAmount, format := model.GetAmountInMarket(model.Gate, symbol, amount, price, false)
		relatedOrder.Amount = util.CutTailZero(fmt.Sprintf(format, formattedAmount))
		if orderType == model.OrderTypeMarket && orderSide == model.OrderSideBuy {
			relatedOrder.Amount = fmt.Sprintf(`%f`, amount*price)
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`create spot order request: %#v`, relatedOrder))
		if isWs {
			param := map[string]interface{}{"text": `t-` + order.ClientOrdId, `currency_pair`: dialectSymbol, `type`: orderType,
				`account`: `spot`, `side`: orderSide, `amount`: relatedOrder.Amount, `price`: orderPriceStr,
				`time_in_force`: tif, `auto_repay`: true}
			reqMap := map[string]interface{}{`time`: ts, `channel`: `spot.order_place`, `event`: `api`,
				`payload`: map[string]interface{}{`req_id`: order.ClientOrdId, `req_param`: param}}
			wsOrderMsg := util.JsonEncodeToByte(reqMap)
			connKey := getPrivateConnKey(model.Gate, account.Key, model.MarketTypeSpot)
			value, _ := model.AppEnvironment.ConnOrder.Load(connKey)
			if value != nil {
				if err := value.(*model.WSConn).WriteMsg(wsOrderMsg); err != nil {
					model.AppEnvironment.ConnOrder.Delete(connKey)
					order.Status = model.CarryStatusFail
					util.Log(util.LogLevelError, fmt.Sprintf(`fail to order gate ws %s %s`, string(wsOrderMsg), err.Error()))
				}
			} else {
				order.Status = model.CarryStatusFail
			}
			if order.Status == model.CarryStatusFail {
				HandleWsOrderConnFail(account, model.Gate, order)
			}
		} else {
			createOrder, _, err := client.SpotApi.CreateOrder(ctx, relatedOrder)
			if err != nil {
				panicGateError(account.Key, "placeSpotOrderGate", err)
				order.Status = model.CarryStatusFail
				order.ErrCode = err.Error()
			} else {
				//orderResp, _ := json.Marshal(createOrder)
				//util.Log(util.LogLevelInfo, fmt.Sprintf(`create spot order response: %s`, orderResp))
				order.OrderId = createOrder.Id
				//order.Symbol = createOrder.CurrencyPair
				secondUnix, _ := strconv.ParseInt(createOrder.CreateTime, 10, 64)
				order.OrderTime = time.Unix(secondUnix, 0)
				//order.Price, _ = strconv.ParseFloat(createOrder.Price, 64)
				//order.Amount, _ = strconv.ParseFloat(createOrder.Amount, 64)
				order.OrderSide = createOrder.Side
				order.OrderType = createOrder.Type
				if createOrder.Status == "cancelled" {
					order.Status = model.CarryStatusFail
				} else {
					order.Status = model.CarryStatusWorking
				}
			}
		}
	} else if marketType == model.MarketTypePerp {
		futuresOrder := gateApi.FuturesOrder{Price: orderPriceStr, Contract: dialectSymbol, Tif: tif}
		formattedAmount, format := model.GetAmountInMarket(model.Gate, symbol, amount, price, false)
		futuresOrder.Size, _ = strconv.ParseInt(util.CutTailZero(fmt.Sprintf(format, formattedAmount)), 10, 64)
		if orderSide == model.OrderSideSell {
			futuresOrder.Size = -1 * futuresOrder.Size
		}
		if orderParam == model.CloseContract {
			futuresOrder.Size = 0
			futuresOrder.Close = true
		}
		if orderParam == model.ReduceOnly || symbol == `MUBI_PERP` || symbol == `DOP_PERP` || symbol == `BBL_PERP` || symbol == `NRN_PERP` {
			futuresOrder.ReduceOnly = true
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`create future order request: %#v`, futuresOrder))
		if isWs {
			param := map[string]interface{}{`contract`: dialectSymbol, `size`: futuresOrder.Size, `price`: orderPriceStr,
				`tif`: tif, `text`: `t-` + order.ClientOrdId, `reduce_only`: futuresOrder.ReduceOnly, `close`: futuresOrder.Close}
			reqMap := map[string]interface{}{`time`: ts, `channel`: `futures.order_place`, `event`: `api`,
				`payload`: map[string]interface{}{`req_id`: order.ClientOrdId, `req_param`: param}}
			wsOrderMsg := util.JsonEncodeToByte(reqMap)
			connKey := getPrivateConnKey(model.Gate, account.Key, model.MarketTypePerp)
			value, _ := model.AppEnvironment.ConnOrder.Load(connKey)
			if value != nil {
				if err := value.(*model.WSConn).WriteMsg(wsOrderMsg); err != nil {
					model.AppEnvironment.ConnOrder.Delete(connKey)
					order.Status = model.CarryStatusFail
					util.Log(util.LogLevelError, fmt.Sprintf(`fail to order gate ws %s %s`, string(wsOrderMsg), err.Error()))
				}
			} else {
				order.Status = model.CarryStatusFail
			}
			if order.Status == model.CarryStatusFail {
				HandleWsOrderConnFail(account, model.Gate, order)
			}
		} else {
			createFuturesOrder, _, err := client.FuturesApi.CreateFuturesOrder(ctx, `usdt`, futuresOrder)
			if err != nil {
				panicGateError(account.Key, "placeFutureOrderGate", err)
				order.Status = model.CarryStatusFail
				order.ErrCode = err.Error()
			} else {
				//orderResp, _ := json.Marshal(createFuturesOrder)
				//util.Log(util.LogLevelError, fmt.Sprintf(`create future order response: %s`, orderResp))
				if createFuturesOrder.IsLiq {
					util.Log(util.LogLevelError, fmt.Sprintf("warning warning, blow up!!!"))
				}
				order.OrderId = strconv.FormatInt(createFuturesOrder.Id, 10)
				order.OrderTime = time.Unix(int64(createFuturesOrder.CreateTime), 0)
				order.Price, _ = strconv.ParseFloat(createFuturesOrder.Price, 64)
				//_, order.Amount = model.ParseRealAmount(model.Gate, order.Symbol, float64(createFuturesOrder.Size))
				order.Status = model.CarryStatusWorking
			}
		}
	}
}

func getMaxLoanGate(symbol string) (success bool, maxLoan float64) {
	v, _ := util.LoadSyncMap(model.MarketInfos, model.Gate, symbol)
	_, tickRelated := model.AppEnvironment.GetBidAsk(model.Gate, symbol)
	if tickRelated != nil && v != nil {
		maxLoan = v.(*model.MarketInfo).BorrowUsdtMax / tickRelated.Bids[0].Price
	}
	return true, maxLoan
}

func getFundingRateGate(key, secret, symbol string) (fundingRate *model.FundingRate) {
	client, ctx := getClientGate(key, secret)
	_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	contract, _, err := client.FuturesApi.GetFuturesContract(ctx, `usdt`, dialectSymbol)
	if err != nil {
		panicGateError(key, "getFundingRateGate", err)
		return nil
	}
	rate, _ := strconv.ParseFloat(contract.FundingRate, 64)
	return &model.FundingRate{
		Rate:       rate,
		UpdateTime: time.Now(),
		ExpireTime: int64(contract.FundingNextApply)}
}

// SetGateBidAsk 用于处理永续合约买卖一不准确（现货无需，因为订阅方式不同）
func _(key, secret, symbol string) {
	client, ctx := getClientGate(key, secret)
	_, _, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	orderBook, _, err := client.FuturesApi.ListFuturesOrderBook(ctx, `usdt`, dialectSymbol,
		&gateApi.ListFuturesOrderBookOpts{Limit: optional.NewInt32(1)})
	if err != nil {
		panicGateError(key, "setFutureTicker", err)
	}
	result, oldBidAsk := model.AppEnvironment.GetBidAsk(model.Gate, symbol)
	if result && float64(oldBidAsk.Ts) > orderBook.Update*1000 || orderBook.Bids == nil || len(orderBook.Bids) < 1 ||
		orderBook.Asks == nil || len(orderBook.Asks) < 1 {
		return
	}
	bidPrice, _ := strconv.ParseFloat(orderBook.Bids[0].P, 64)
	_, bidAmount := model.ParseRealAmount(model.Gate, symbol, float64(orderBook.Bids[0].S))
	askPrice, _ := strconv.ParseFloat(orderBook.Asks[0].P, 64)
	_, askAmount := model.ParseRealAmount(model.Gate, symbol, float64(orderBook.Asks[0].S))
	bidAsk := model.BidAsk{Ts: int(orderBook.Update * 1000),
		TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond)),
		Bids:       []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.Gate, Symbol: symbol}},
		Asks:       []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.Gate, Symbol: symbol}}}
	model.AppEnvironment.SetBidAsk(model.Gate, symbol, &bidAsk)
}

func queryOrderGate(key, secret, symbol, orderId string) (order *model.Order) {
	client, ctx := getClientGate(key, secret)
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Gate, symbol)
	if success && marketType == model.MarketTypePerp {
		orderFuture, _, err := client.FuturesApi.GetFuturesOrder(ctx, `usdt`, orderId)
		if err != nil {
			panicGateError(key, "GetFuturesOrder", err)
			return
		}
		order = parseOrderGatePerp(&orderFuture)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`queryOrderGate query result %#v \n %#v`,
			order, orderFuture))
	} else if success && marketType == model.MarketTypeSpot {
		orderSpot, _, err := client.SpotApi.GetOrder(ctx, orderId, dialectSymbol, nil)
		if err != nil {
			panicGateError(key, "GetSpotOrder", err)
			return
		}
		order = parseOrderGateSpot(&orderSpot)
		util.Log(util.LogLevelInfo, fmt.Sprintf(`queryOrderGate query result %#v \n %#v`,
			order, orderSpot))
	}
	return order
}

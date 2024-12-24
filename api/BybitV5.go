package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"hello/api/dtos"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const bybitRestUrl = "https://api.bybit.com"
const bybitStreamUrl = "wss://stream.bybit.com"
const bybitTradeWsUrl = "wss://stream.bybit.com/v5/trade?max_active_time=10m"

const wsStepBybit = 10

var wsOrdUdtHandlerBybit = func(market, key string, msg []byte) {
	value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, key)
	if value == nil {
		return
	}
	responseJson, err := util.NewJSON(msg)
	if err != nil || responseJson == nil {
		return
	}
	if responseJson.Get(`op`).MustString() == `auth` && responseJson.Get(`success`).MustBool() {
		err := SendToConnection(model.Bybit, value.(*model.WSConn), []byte(`{"op":"subscribe","args": ["order"]}`))
		if err != nil {
			util.DelSyncMap(&model.AppEnvironment.ConnOrderUpdate, market, key)
		}
	}
	orderResp := &dtos.BybitOrderUpdateResp{}
	jsonErr := json.Unmarshal(msg, orderResp)
	if jsonErr == nil {
		for _, data := range orderResp.Data {
			status := model.CarryStatusWorking
			if data.OrderStatus == `Filled` {
				status = model.CarryStatusSuccess
			}
			dealAmount, _ := strconv.ParseFloat(data.CumExecQty, 64)
			UpdateOrderDeal(market, data.OrderId, status, string(msg), dealAmount)
		}
	}
}

var wsOrderHandlerBybit = func(market, key string, event []byte) {
	responseJson, err := util.NewJSON(event)
	if err != nil || responseJson == nil {
		return
	}
	if responseJson.Get(`op`).MustString() != `order.create` {
		return
	}
	wsResp := model.WSResp{RequestId: responseJson.Get(`reqId`).MustString(),
		OrderId: responseJson.GetPath(`data`, `orderId`).MustString()}
	code := responseJson.Get(`retCode`).MustInt64()
	if code == 0 && responseJson.Get(`retMsg`).MustString() == `OK` {
		wsResp.Success = true
	} else {
		wsResp.Success = false
		wsResp.Msg = fmt.Sprintf(`%d %s`, code, responseJson.Get(`retMsg`).MustString())
	}
	model.AppEnvironment.WSRespChan <- wsResp
}

func maintainConnsBybit(accounts []*model.Account) {
	for {
		if !CheckSetProcessing(model.FunctionConnMaintain, model.Bybit, ``, true) {
			time.Sleep(2 * time.Second)
			continue
		}
		pingMsg := []byte(fmt.Sprintf(`{ "req_id": "maintain %d","op": "ping"}`, time.Now().UnixMilli()))
		connTick, _ := model.AppEnvironment.ConnTick.Load(model.Bybit)
		if connTick != nil {
			if err := SendToConnections(model.Bybit, connTick.(map[*model.WSConn]bool), pingMsg); err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("tick conn maintain error %s %s", model.Bybit, err.Error()))
			}
		}
		for _, account := range accounts {
			if account == nil {
				continue
			}
			success := true
			errMsg := ``
			connOrder, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, model.Bybit, account.Key)
			if connOrder != nil {
				if err := SendToConnection(model.Bybit, connOrder.(*model.WSConn), pingMsg); err != nil {
					errMsg += err.Error()
					success = false
					util.Log(util.LogLevelError, "-ws-bybit trade ws ping client error "+err.Error())
				}
			} else {
				success = false
			}
			connOrderUpdate, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrderUpdate, model.Bybit, account.Key)
			if connOrderUpdate != nil {
				if err := SendToConnection(model.Bybit, connOrderUpdate.(*model.WSConn), pingMsg); err != nil {
					errMsg += err.Error()
					success = false
					util.Log(util.LogLevelError, "ws-bybit order update ws ping client error "+err.Error())
				}
			} else {
				success = false
			}
			if !success {
				if connOrderUpdate != nil {
					connOrderUpdate.(*model.WSConn).Close()
				}
				if connOrder != nil {
					connOrder.(*model.WSConn).Close()
				}
				util.Log(util.LogLevelError, "fail to ping ws bybit "+errMsg)
				util.DelSyncMap(&model.AppEnvironment.ConnOrder, model.Bybit, account.Key)
				util.DelSyncMap(&model.AppEnvironment.ConnOrderUpdate, model.Bybit, account.Key)
				WsOrderServeBybit(account)
			}
		}
		CheckSetProcessing(model.FunctionConnMaintain, model.Bybit, ``, true)
		time.Sleep(time.Second * 20)
	}
}

func WsLogInBybit(account *model.Account, conn *model.WSConn) (success bool) {
	loginMap := make(map[string]interface{})
	loginMap[`op`] = `auth`
	timestamp := time.Now().UnixMilli() + 1000
	toBeSign := fmt.Sprintf(`GET/realtime%d`, timestamp)
	hash := hmac.New(sha256.New, []byte(account.Secret))
	hash.Write([]byte(toBeSign))
	sign := hex.EncodeToString(hash.Sum(nil))
	loginArray := []interface{}{account.Key, timestamp, sign}
	loginMap[`args`] = loginArray
	loginBytes := util.JsonEncodeToByte(loginMap)
	if err := SendToConnection(model.Bybit, conn, loginBytes); err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(
			`fail to login bybit trade ws: %s return %s`, account.Key, err.Error()))
	} else {
		util.Log(util.LogLevelInfo, fmt.Sprintf("log in conn %s %s", model.Bybit, string(loginBytes)))
		success = true
	}
	return success
}

func WsOrderServeBybit(account *model.Account) {
	if account == nil {
		return
	}
	valueOrder, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, model.Bybit, account.Key)
	valueOrderUpdate, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrderUpdate, model.Bybit, account.Key)
	if valueOrder == nil || valueOrderUpdate == nil {
		connOrder, errOrder := model.WsPrivateClient(model.Bybit, account.Key, bybitTradeWsUrl, wsOrderHandlerBybit)
		if errOrder != nil {
			util.Log(util.LogLevelError, "bybit can not create ws order "+errOrder.Error())
		} else if connOrder != nil {
			if WsLogInBybit(account, connOrder) {
				util.StoreSyncMap(&model.AppEnvironment.ConnOrder, connOrder, model.Bybit, account.Key)
			}
		}
		connOrderUpdate, errOrderUpdate := model.WsPrivateClient(model.Bybit, account.Key, bybitStreamUrl+`/v5/private`, wsOrdUdtHandlerBybit)
		if errOrderUpdate != nil {
			util.Log(util.LogLevelError, "bybit can not create ws order update"+errOrderUpdate.Error())
		} else if connOrderUpdate != nil {
			if WsLogInBybit(account, connOrderUpdate) {
				util.StoreSyncMap(&model.AppEnvironment.ConnOrderUpdate, connOrderUpdate, model.Bybit, account.Key)
			}
		}
	}
}

func getMarketsBybit() (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	getMarketsBybitSpot(marketInfos)
	getMarketsBybitPerp(marketInfos)
	return marketInfos
}

func getMarketsBybitSpot(marketInfos map[string]*model.MarketInfo) {
	param := map[string]interface{}{"category": "spot", "limit": "1000"}
	composeParams := util.ComposeParams(param)
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bybitRestUrl+"/v5/market/instruments-info?"+composeParams,
		"", map[string]string{}, 30)
	spotResp := &dtos.BybitSpotMarketResp{}
	spotJsonErr := json.Unmarshal(httpResp, spotResp)
	if spotResp == nil || spotResp.RetCode != 0 {
		util.Log(util.LogLevelError, fmt.Sprintf(
			"get bybit spot market error, resp: %s, httpErr: %#v, jsonErr: %#v", httpResp, httpErr, spotJsonErr))
		return
	}
	for _, symbolInfo := range spotResp.Result.List {
		if symbolInfo.Status != "Trading" || symbolInfo.QuoteCoin != "USDT" {
			continue
		}
		symbol := symbolInfo.BaseCoin + model.UniStandardTail[model.MarketTypeSpot]
		marketInfo := &model.MarketInfo{Symbol: symbol, Market: model.Bybit, FundingRateInterval: 8 * 3600000}
		if symbolInfo.PriceFilter.TickSize == "" {
			util.Log(util.LogLevelError, fmt.Sprintf("币种：%s 价格步长为空 resp：%#v", symbol, symbolInfo))
			continue
		}
		priceIncrement, _ := strconv.ParseFloat(symbolInfo.PriceFilter.TickSize, 64)
		marketInfo.PriceIncrement = priceIncrement
		marketInfo.PriceDecimal = util.NumDecPlaces(priceIncrement)
		sizeIncrement, _ := strconv.ParseFloat(symbolInfo.LotSizeFilter.BasePrecision, 64)
		marketInfo.SizeIncrement = sizeIncrement
		marketInfo.SizeMin, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MinOrderQty, 64)
		marketInfo.SizeMax, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MaxOrderQty, 64)
		marketInfo.MoneyMin, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MinOrderAmt, 64)
		marketInfo.QuoteMax, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MaxOrderAmt, 64)
		marketInfos[marketInfo.Symbol] = marketInfo
	}
}

func getMarketsBybitPerp(marketInfos map[string]*model.MarketInfo) {
	cursor := "init"
	for {
		param := map[string]interface{}{"category": "linear", "limit": "1000"}
		if cursor != "" && cursor != "init" {
			param["cursor"] = cursor
		}
		composeParams := util.ComposeParams(param)
		httpResp, httpErr := util.HttpRequest(http.MethodGet, bybitRestUrl+"/v5/market/instruments-info?"+composeParams,
			"", map[string]string{}, 30)
		perpResp := &dtos.BybitPerpMarketResp{}
		perpJsonErr := json.Unmarshal(httpResp, perpResp)
		if perpResp == nil || perpResp.RetCode != 0 {
			util.Log(util.LogLevelError, fmt.Sprintf(
				"get bybit perp market error, resp: %s, httpErr: %#v, jsonErr: %#v", httpResp, httpErr, perpJsonErr))
			return
		}
		for _, perpInfo := range perpResp.Result.List {
			if perpInfo.Status != "Trading" || perpInfo.QuoteCoin != "USDT" || perpInfo.ContractType != "LinearPerpetual" {
				continue
			}
			symbol := perpInfo.BaseCoin + model.UniStandardTail[model.MarketTypePerp]
			marketInfo := &model.MarketInfo{Symbol: symbol, Market: model.Bybit, FundingRateInterval: 8 * 3600000}
			marketInfo.FundingRateInterval = perpInfo.FundingInterval * 60000
			marketInfo.PriceIncrement, _ = strconv.ParseFloat(perpInfo.PriceFilter.TickSize, 64)
			marketInfo.PriceDecimal, _ = strconv.Atoi(perpInfo.PriceScale)
			marketInfo.PriceMax, _ = strconv.ParseFloat(perpInfo.PriceFilter.MaxPrice, 64)
			priceMin, _ := strconv.ParseFloat(perpInfo.PriceFilter.MinPrice, 64)
			if priceMin != marketInfo.PriceIncrement {
				util.Log(util.LogLevelError, fmt.Sprintf("最小价格和价格步长不一致 perp info：%#v", perpInfo))
				continue
			}
			maxLeverage, _ := strconv.ParseFloat(perpInfo.LeverageFilter.MaxLeverage, 64)
			if maxLeverage < model.DefaultLeverage {
				util.Log(util.LogLevelError, fmt.Sprintf("最大杠杆小于%d perp info：%#v", model.DefaultLeverage, perpInfo))
				continue
			}
			marketInfo.SizeMin, _ = strconv.ParseFloat(perpInfo.LotSizeFilter.MinOrderQty, 64)
			marketInfo.SizeMax, _ = strconv.ParseFloat(perpInfo.LotSizeFilter.MaxOrderQty, 64)
			marketInfo.SizeIncrement, _ = strconv.ParseFloat(perpInfo.LotSizeFilter.QtyStep, 64)
			marketInfos[symbol] = marketInfo
		}
		cursor = perpResp.Result.NextPageCursor
		if cursor == "" {
			return
		}
	}
}

func handleBookBybit(environment *model.Environment, bookWsResp *dtos.BybitBookWsResp, symbol string) {
	bidAsk := &model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
	bidAsk.Ts = int(bookWsResp.Ts)
	bidAsk.UpdateId = bookWsResp.Data.Seq
	haveOld, old := environment.GetBidAsk(model.Bybit, symbol)
	if bookWsResp.Type == "snapshot" {
		if len(bookWsResp.Data.B) == 0 || len(bookWsResp.Data.A) == 0 {
			util.Log(util.LogLevelError, fmt.Sprintf(`bybit no book data %s`, bookWsResp.Data.S))
			return
		}
		bidPrice, _ := strconv.ParseFloat(bookWsResp.Data.B[0][0], 64)
		bidAmount, _ := strconv.ParseFloat(bookWsResp.Data.B[0][1], 64)
		askPrice, _ := strconv.ParseFloat(bookWsResp.Data.A[0][0], 64)
		askAmount, _ := strconv.ParseFloat(bookWsResp.Data.A[0][1], 64)
		bid := model.Tick{Price: bidPrice, Amount: bidAmount, Market: model.Bybit, Symbol: symbol}
		ask := model.Tick{Price: askPrice, Amount: askAmount, Market: model.Bybit, Symbol: symbol}
		bidAsk.Bids = []model.Tick{bid}
		bidAsk.Asks = []model.Tick{ask}
	} else if bookWsResp.Type == "delta" {
		if !haveOld {
			util.Log(util.LogLevelError, fmt.Sprintf("币种：%s bidask没有bidask 却收到delta ws", symbol))
			return
		}
		oldBid := old.Bids[0]
		oldAsk := old.Asks[0]
		if len(bookWsResp.Data.B) == 0 {
			bidAsk.Bids = []model.Tick{oldBid}
		} else {
			for _, bidStr := range bookWsResp.Data.B {
				bidAmount, _ := strconv.ParseFloat(bidStr[1], 64)
				if bidAmount == 0 {
					continue
				}
				bidPrice, _ := strconv.ParseFloat(bidStr[0], 64)
				bid := model.Tick{Price: bidPrice, Amount: bidAmount, Market: model.Bybit, Symbol: symbol}
				bidAsk.Bids = []model.Tick{bid}
			}
		}
		if len(bookWsResp.Data.A) == 0 {
			bidAsk.Asks = []model.Tick{oldAsk}
		} else {
			for _, askStr := range bookWsResp.Data.A {
				askAmount, _ := strconv.ParseFloat(askStr[1], 64)
				askPrice, _ := strconv.ParseFloat(askStr[0], 64)
				if askAmount == 0 {
					continue
				}
				ask := model.Tick{Price: askPrice, Amount: askAmount, Market: model.Bybit, Symbol: symbol}
				bidAsk.Asks = []model.Tick{ask}
			}
		}
	} else {
		return
	}
	if environment.SetBidAsk(model.Bybit, symbol, bidAsk) {
		funcHandlers := GetFunctions(model.Bybit, symbol)
		if funcHandlers != nil {
			funcHandlers.Range(func(function, value interface{}) bool {
				setting := GetSetting(function.(string), model.Bybit, symbol)
				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
					go value.(model.CarryHandler)(setting, bidAsk)
				}
				return true
			})
		}
	}
}

var spotBookWsHandler = func(market string, conn *model.WSConn, event []byte) {
	bookWsResp := &dtos.BybitBookWsResp{}
	jsonErr := json.Unmarshal(event, bookWsResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, `fail to unmarshal bybit spot book ws data json `+jsonErr.Error())
		return
	}
	if strings.Contains(bookWsResp.Topic, "orderbook") {
		if bookWsResp.Data.S == "" {
			return
		}
		success, _, symbol := model.GetFromDialect(model.Bybit, model.MarketTypeSpot, bookWsResp.Data.S)
		if !success {
			return
		}
		handleBookBybit(model.AppEnvironment, bookWsResp, symbol)
	}
}

var tickHandlerBybit = func(market string, conn *model.WSConn, event []byte) {
	tickResp := &dtos.BybitTickResp{}
	jsonErr := json.Unmarshal(event, tickResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, `fail to unmarshal bybit perp tick ws data json `+jsonErr.Error())
		return
	}
	if strings.Contains(tickResp.Topic, "tickers") {
		success, _, symbol := model.GetFromDialect(model.Bybit, model.MarketTypePerp, tickResp.Data.Symbol)
		if !success {
			return
		}
		rate, _ := strconv.ParseFloat(tickResp.Data.FundingRate, 64)
		nextFundingTime, _ := strconv.ParseInt(tickResp.Data.NextFundingTime, 10, 64)
		fmt.Println(string(event))
		if nextFundingTime > time.Now().Unix() {
			SetFundingRate(model.Bybit, symbol, &model.FundingRate{
				Rate:       rate,
				UpdateTime: time.UnixMilli(tickResp.Ts),
				ExpireTime: nextFundingTime / 1000,
			})
		}
	}
}

var perpBookWsHandler = func(market string, conn *model.WSConn, event []byte) {
	bookWsResp := &dtos.BybitBookWsResp{}
	jsonErr := json.Unmarshal(event, bookWsResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, `fail to unmarshal bybit perp book ws data json `+jsonErr.Error())
		return
	}
	if strings.Contains(bookWsResp.Topic, "orderbook") {
		if bookWsResp.Data.S == "" {
			return
		}
		success, _, symbol := model.GetFromDialect(model.Bybit, model.MarketTypePerp, bookWsResp.Data.S)
		if !success {
			return
		}
		handleBookBybit(model.AppEnvironment, bookWsResp, symbol)
	}
}

func WsTickServeBybit(market string) (socketMap map[*model.WSConn]bool, msgChans []chan struct{}, connectErr error) {
	msgChans = make([]chan struct{}, 0)
	socketMap = make(map[*model.WSConn]bool)
	symbols := GetMarketSymbols(model.Bybit)
	spotSubBook := make([]interface{}, 0)
	futureSubBook := make([]interface{}, 0)
	futureSubTick := make([]interface{}, 0)
	for symbol := range symbols {
		success, marketType, _, dialectSymbol := model.GetFromStandard(model.Bybit, symbol)
		if !success {
			continue
		}
		if marketType == model.MarketTypePerp {
			futureSubBook = append(futureSubBook, fmt.Sprintf("orderbook.1.%s", dialectSymbol))
			futureSubTick = append(futureSubTick, fmt.Sprintf(`tickers.%s`, dialectSymbol))
		} else {
			spotSubBook = append(spotSubBook, fmt.Sprintf("orderbook.1.%s", dialectSymbol))
		}
	}
	spotBookSockets, spotBookChannels, spotBookErr := model.WsPublicClient(model.Bybit, bybitStreamUrl+`/v5/public/spot`,
		spotSubBook, subscribeHandlerBybit, spotBookWsHandler, wsStepBybit)
	if spotBookErr == nil {
		msgChans = append(msgChans, spotBookChannels...)
		for conn, b := range spotBookSockets {
			socketMap[conn] = b
		}
	}
	perpBookSockets, perpBookChannels, perpBookErr := model.WsPublicClient(market, bybitStreamUrl+`/v5/public/linear`,
		futureSubBook, subscribeHandlerBybit, perpBookWsHandler, wsStepBybit)
	if perpBookErr == nil {
		msgChans = append(msgChans, perpBookChannels...)
		for conn, b := range perpBookSockets {
			socketMap[conn] = b
		}
	}
	//perpTickConns, perpTickChans, perpTickErr := model.WsPublicClient(market, bybitStreamUrl+`/v5/public/linear`,
	//	futureSubTick, subscribeHandlerBybit, tickHandlerBybit, wsStepBybit)
	//if perpTickErr == nil {
	//	msgChans = append(msgChans, perpTickChans...)
	//	for conn, b := range perpTickConns {
	//		socketMap[conn] = b
	//	}
	//}
	return
}

var subscribeHandlerBybit = func(market string, connection *model.WSConn, subscribes []interface{}) error {
	var err error = nil
	subscribeMap := make(map[string]interface{})
	subscribeMap["req_id"] = int(rand.Float64() * 10000)
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = subscribes
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.Bybit, connection, subscribeMessage); err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(" bybit can not subscribe %s %s", subscribeMessage, err.Error()))
	}
	return err
}

func GetCoinBalanceBybit(key, secret, accountType string) (balances []*model.Balance) {
	param := map[string]interface{}{"accountType": `FUND`}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/asset/transfer/query-account-coins-balance", param)
	balanceResp := &dtos.BybitBalanceCoinResp{}
	jsonErr := json.Unmarshal(httpResp, balanceResp)
	if balanceResp == nil || balanceResp.RetCode != 0 {
		util.Log(util.LogLevelError, fmt.Sprintf(
			"fail to refresh spot balance bybit, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
		time.Sleep(time.Minute)
		return GetCoinBalanceBybit(key, secret, accountType)
	}
	balances = make([]*model.Balance, 0)
	for _, account := range balanceResp.Result.List {
		//if account.AccountType == "FUND" {
		balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Bybit, Coin: account.Coin}
		balance.Amount, _ = strconv.ParseFloat(account.TransferBalance, 64)
		balances = append(balances, balance)
	}
	return balances
}

func getBalanceBybit(key string, secret string) (success bool, balances []*model.Balance, totalInUsd float64, collateral *model.Collateral) {
	//marketInfos := model.GetMarketInfos(model.Bybit, model.MarketTypeSpot)
	//coinsStr := make([]string, 0)
	//for symbol, value := range marketInfos {
	//	if value == nil {
	//		continue
	//	}
	//	_, _, coin, _ := model.GetFromStandard(model.Bybit, symbol)
	//	coinsStr = append(coinsStr, coin)
	//}
	//param := map[string]interface{}{"accountType": "UNIFIED", "coin": strings.Join(coinsStr, ",")}
	param := map[string]interface{}{"accountType": "UNIFIED"}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/account/wallet-balance", param)
	balanceResp := &dtos.BybitBalanceResp{}
	jsonErr := json.Unmarshal(httpResp, balanceResp)
	if balanceResp == nil || balanceResp.RetCode != 0 {
		util.Log(util.LogLevelError, fmt.Sprintf(
			"fail to refresh spot balance bybit, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
		time.Sleep(time.Minute)
		return getBalanceBybit(key, secret)
	}
	balances = make([]*model.Balance, 0)
	for _, account := range balanceResp.Result.List {
		if account.AccountType == "UNIFIED" {
			maintenanceRate, _ := strconv.ParseFloat(account.AccountMMRate, 64)
			collateralAvailable, _ := strconv.ParseFloat(account.TotalAvailableBalance, 64)
			totalMaintenanceMargin, _ := strconv.ParseFloat(account.TotalMaintenanceMargin, 64)
			totalInUsd, _ = strconv.ParseFloat(account.TotalEquity, 64)
			collateral = &model.Collateral{Available: collateralAvailable, Occupied: totalMaintenanceMargin, Rate: maintenanceRate}
			for _, coinInfo := range account.Coin {
				balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Bybit, Coin: coinInfo.Coin}
				balance.Borrow, _ = strconv.ParseFloat(coinInfo.BorrowAmount, 64)
				canBorrow, _ := strconv.ParseFloat(coinInfo.AvailableToBorrow, 64)
				holdAmount, _ := strconv.ParseFloat(coinInfo.WalletBalance, 64)
				if coinInfo.Coin == "USDT" {
					holdAmount, _ = strconv.ParseFloat(coinInfo.AvailableToWithdraw, 64)
				}
				balance.Amount = holdAmount
				balance.AvailableWithBorrow = math.Max(0, balance.Amount) + canBorrow
				usdValue, _ := strconv.ParseFloat(coinInfo.UsdValue, 64)
				if usdValue == 0 {
					priceGet, bidAsk := model.AppEnvironment.GetBidAsk(model.Bybit, balance.Coin+model.UniStandardTail[model.MarketTypeSpot])
					if priceGet {
						usdValue = balance.Amount * bidAsk.Bids[0].Price
					}
				}
				balance.UsdValue = usdValue
				balances = append(balances, balance)
			}
		}
	}
	if collateral == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(
			"fail to refresh spot balance bybit, resp: %s httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
		time.Sleep(time.Minute)
		return getBalanceBybit(key, secret)
	}
	return true, balances, totalInUsd, collateral
}

func getPositionsBybit(key, secret string) (success bool, positions []*Position, posBalance float64) {
	cursor := "init"
	positions = make([]*Position, 0)
	for {
		param := map[string]interface{}{"category": "linear", "settleCoin": "USDT", "limit": "200"}
		if cursor != "" && cursor != "init" {
			param["cursor"] = cursor
		}
		positionHttpResp, positionHttpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/position/list", param)
		positionResp := &dtos.BybitPositionResp{}
		positionJsonErr := json.Unmarshal(positionHttpResp, positionResp)
		if positionResp == nil || positionResp.RetCode != 0 {
			util.Log(util.LogLevelError, fmt.Sprintf("fail to refresh perp position bybit, resp: %s httpErr: %#v, jsonErr: %#v",
				positionHttpResp, positionHttpErr, positionJsonErr))
			time.Sleep(time.Minute)
			return getPositionsBybit(key, secret)
		}
		for _, contract := range positionResp.Result.List {
			if contract.TradeMode != 0 {
				continue
			}
			_, _, currency := model.GetFromDialect(model.Bybit, model.MarketTypePerp, contract.Symbol)
			position := &Position{Market: model.Bybit, Ts: util.GetNowUnixMillion(), Currency: currency}
			if contract.Side == "Buy" {
				position.Holding, _ = strconv.ParseFloat(contract.Size, 64)
			} else if contract.Side == "Sell" {
				total, _ := strconv.ParseFloat(contract.Size, 64)
				position.Holding = -1 * total
			} else {
				position.Holding = 0
			}
			position.LeverRate, _ = strconv.ParseInt(contract.Leverage, 10, 64)
			position.EntryPrice, _ = strconv.ParseFloat(contract.AvgPrice, 64)
			position.BankruptcyPrice, _ = strconv.ParseFloat(contract.BustPrice, 64)
			position.LiquidationPrice, _ = strconv.ParseFloat(contract.LiqPrice, 64)
			position.Margin, _ = strconv.ParseFloat(contract.PositionMM, 64)
			if position.Holding != 0 {
				positions = append(positions, position)
				//util.Log(util.LogLevelInfo, fmt.Sprintf(`get position bybit %#v`, position))
			}
		}
		cursor = positionResp.Result.NextPageCursor
		if cursor == "" {
			return true, positions, 0
		}
	}
}

func SignedRequestBybit(key, secret, method, host, path string, body map[string]interface{}) ([]byte, error) {
	if body == nil {
		body = make(map[string]interface{})
	}
	receiveWindow := "5000"
	timestamp := strconv.FormatInt(util.GetNowUnixMillion(), 10)
	hash := hmac.New(sha256.New, []byte(secret))
	header := map[string]string{
		"X-BAPI-API-KEY":     key,
		"X-BAPI-TIMESTAMP":   timestamp,
		"X-BAPI-RECV-WINDOW": receiveWindow,
	}
	if method == http.MethodGet {
		composeParams := util.ComposeParams(body)
		hash.Write([]byte(timestamp + key + receiveWindow + composeParams))
		sign := hex.EncodeToString(hash.Sum(nil))
		header["X-BAPI-SIGN"] = sign
		httpResp, httpErr := util.HttpRequest(http.MethodGet, host+path+"?"+composeParams, "", header, 30)
		return httpResp, httpErr
	} else if method == http.MethodPost {
		jsonParams := string(util.JsonEncodeToByte(body))
		hash.Write([]byte(timestamp + key + receiveWindow + jsonParams))
		sign := hex.EncodeToString(hash.Sum(nil))
		header["X-BAPI-SIGN"] = sign
		header["Content-Type"] = "application/json"
		httpResp, httpErr := util.HttpRequest(http.MethodPost, host+path, jsonParams, header, 30)
		return httpResp, httpErr
	}
	return nil, http.ErrNoLocation
}

func WithdrawBybit(key, secret, coin, chain, address, amount string) bool {
	param := map[string]interface{}{`chain`: chain, `coin`: coin, `amount`: amount,
		`address`: address, `accountType`: `FUND`, `timestamp`: time.Now().UnixMilli()}
	_, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl,
		`/v5/asset/withdraw/create`, param)
	if httpErr != nil {
		util.Log(util.LogLevelError, `fail to withdraw bybit`+httpErr.Error())
		return false
	} else {
		return true
	}
}

// TransferInnerBybit
func _(key, secret, coin, amount, fromType, toType string) bool {
	transferId := uuid.NewV4()
	param := map[string]interface{}{"transferId": transferId.String(), `coin`: coin, `amount`: amount,
		`fromAccountType`: fromType, `toAccountType`: toType}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl,
		`/v5/asset/transfer/inter-transfer`, param)
	if httpErr == nil {
		return true
	} else {
		util.Log(util.LogLevelError, `fail to transfer `+httpErr.Error()+string(httpResp))
	}
	return false
}

func setBybitMarginLeverage(key, secret string) {
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl,
		"/v5/spot-margin-trade/set-leverage", map[string]interface{}{"leverage": strconv.Itoa(model.DefaultLeverage)})
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to setBybitMarginLeverage when post %s`, httpErr.Error()))
		return
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to setBybitMarginLeverage when NewJson %s`, jsonErr.Error()))
		return
	}
	if jsonData != nil {
		code, codeErr := jsonData.Get("retCode").Int()
		if code != 0 || codeErr != nil || jsonData.Get(`retMsg`).MustString() != `OK` {
			util.Log(util.LogLevelError, fmt.Sprintf("fail to set bybit margin leverage, resp: %s codeErr: %#v", httpResp, codeErr))
		}
	}
}

func setSymbolLeverageBybit(account *model.Account, symbol string) (setSuc bool) {
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.Bybit, symbol)
	if !success {
		return false
	}
	if marketType == model.MarketTypePerp {
		params := map[string]interface{}{"category": "linear", "buyLeverage": strconv.Itoa(model.DefaultLeverage),
			"sellLeverage": strconv.Itoa(model.DefaultLeverage), "symbol": dialectSymbol}
		httpResp, httpErr := SignedRequestBybit(account.Key, account.Secret, http.MethodPost, bybitRestUrl, "/v5/position/set-leverage", params)
		if httpErr != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to setBybitPerpLeverage when request %s %s`, symbol, httpErr.Error()))
			return false
		}
		jsonData, jsonErr := util.NewJSON(httpResp)
		if jsonErr != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to setBybitPerpLeverage when NewJson %s %s`, symbol, jsonErr.Error()))
			return false
		}
		if jsonData != nil {
			code, codeErr := jsonData.Get("retCode").Int()
			if code != 0 || codeErr != nil || jsonData.Get(`retMsg`).MustString() != `OK` {
				util.Log(util.LogLevelError, fmt.Sprintf("fail to set bybit perp leverage , resp: %s codeErr: %#v", httpResp, codeErr))
				return false
			} else {
				return true
			}
		}
	}
	return false
}

func setBybitPerpLeverage(key, secret string) {
	symbols := GetMarketSymbols(model.Bybit)
	for symbol := range symbols {
		success, marketType, _, dialectSymbol := model.GetFromStandard(model.Bybit, symbol)
		if !success {
			continue
		}
		if marketType == model.MarketTypePerp {
			params := map[string]interface{}{"category": "linear", "buyLeverage": strconv.Itoa(model.DefaultLeverage),
				"sellLeverage": strconv.Itoa(model.DefaultLeverage), "symbol": dialectSymbol}
			httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/position/set-leverage", params)
			if httpErr != nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`fail to setBybitPerpLeverage when request %s %s`, symbol, httpErr.Error()))
				continue
			}
			jsonData, jsonErr := util.NewJSON(httpResp)
			if jsonErr != nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`fail to setBybitPerpLeverage when NewJson %s %s`, symbol, jsonErr.Error()))
				continue
			}
			if jsonData != nil {
				code, codeErr := jsonData.Get("retCode").Int()
				if code != 0 || codeErr != nil || jsonData.Get(`retMsg`).MustString() != `OK` {
					util.Log(util.LogLevelError, fmt.Sprintf("fail to set bybit perp leverage , resp: %s codeErr: %#v", httpResp, codeErr))
				}
			}
			time.Sleep(time.Millisecond * 200)
		}
	}
}

func placeOrderBybit(account *model.Account, isWs bool, order *model.Order, orderParam string) {
	//}, orderSide, orderType, orderParam, symbol string, price, amount float64) {
	reduceOnly := false
	if orderParam == model.ReduceOnly {
		reduceOnly = true
	}
	price, decimal := model.FormatPrice(model.Bybit, order.Symbol, order.Price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Bybit, order.Symbol, order.Amount, price, reduceOnly)))
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	_, marketType, _, dialectSymbol := model.GetFromStandard(model.Bybit, order.Symbol)
	var tradeSide, tradeOrderType string
	if order.OrderSide == model.OrderSideBuy {
		tradeSide = "Buy"
	} else {
		tradeSide = "Sell"
	}
	if order.OrderType == model.OrderTypeLimit {
		tradeOrderType = "Limit"
	} else if order.OrderType == model.OrderTypeMarket {
		tradeOrderType = "Market"
	}
	param := map[string]interface{}{
		"symbol":      dialectSymbol,
		"side":        tradeSide,
		"orderType":   tradeOrderType,
		"qty":         amountStr,
		"price":       priceStr,
		`marketUnit`:  `baseCoin`,
		`orderLinkId`: order.ClientOrdId}
	if marketType == model.MarketTypePerp {
		param["category"] = "linear"
	} else {
		param["category"] = "spot"
	}
	value, _ := util.LoadSyncMap(&model.AppEnvironment.ConnOrder, model.Bybit, account.Key)
	if isWs && value != nil {
		msgMap := map[string]interface{}{"reqId": order.ClientOrdId, `op`: "order.create", "args": []interface{}{param},
			"header": map[string]string{"X-BAPI-TIMESTAMP": fmt.Sprintf(`%d`, time.Now().UnixMilli())}}
		msg := util.JsonEncodeToByte(msgMap)
		if err := SendToConnection(model.Bybit, value.(*model.WSConn), msg); err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to place bybit ws order %s %s`, string(msg), err.Error()))
		}
	} else {
		httpResp, httpErr := SignedRequestBybit(account.Key, account.Secret, http.MethodPost, bybitRestUrl, "/v5/order/create", param)
		bybitOrderResp := &dtos.BybitOrderResp{}
		jsonErr := json.Unmarshal(httpResp, bybitOrderResp)
		if bybitOrderResp == nil || bybitOrderResp.RetCode != 0 {
			if bybitOrderResp != nil {
				order.ErrCode = strconv.Itoa(bybitOrderResp.RetCode)
			}
			util.Log(util.LogLevelError, fmt.Sprintf(
				"fail to create bybit order request: %#v resp: %s httpErr: %#v, jsonErr: %#v", param, httpResp, httpErr, jsonErr))
		} else {
			order.Status = model.CarryStatusWorking
			order.OrderId = bybitOrderResp.Result.OrderId
		}
	}
}

func cancelAllBybit(key, secret, category string) (success bool) {
	param := map[string]interface{}{"category": category}
	if category == `linear` {
		param[`settleCoin`] = `USDT`
	}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/cancel-all", param)
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to do post when cancelOrdersBybit %s`, httpErr.Error()))
		return
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when cancelOrdersBybit %s`, jsonErr.Error()))
		return
	}
	if jsonData != nil {
		code, _ := jsonData.Get("retCode").Int64()
		if code == 0 && jsonData.Get(`retMsg`).MustString() == `OK` {
			util.Log(util.LogLevelInfo, "cancelAll orders success bybit "+category)
			return true
		} else {
			util.Log(util.LogLevelError, fmt.Sprintf("fail to cancel bybit order, code: %d %s", code, string(httpResp)))
		}
	}
	return false
}

func cancelOrderBybit(key, secret, symbol, orderId string) (success bool) {
	param := map[string]interface{}{"orderId": orderId}
	_, marketType, _, dialectSymbol := model.GetFromStandard(model.Bybit, symbol)
	if marketType == model.MarketTypePerp {
		param["category"] = "linear"
	} else {
		param["category"] = "spot"
	}
	param["symbol"] = dialectSymbol
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/cancel", param)
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to do post cancelOrderBybit %s`, httpErr.Error()))
		return false
	}
	respJson, err := util.NewJSON(httpResp)
	if err != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when cancelOrderBybit %s`, err.Error()))
	}
	if respJson != nil && respJson.Get(`retCode`).MustInt() == 0 {
		return true
	} else {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`fail to cancelOrder Bybit %#v`, respJson))
	}
	return false
}

func cancelOrdersBybit(key, secret, symbol string) (result bool) {
	_, marketType, _, dialectSymbol := model.GetFromStandard(model.Bybit, symbol)
	param := map[string]interface{}{
		"symbol": dialectSymbol,
	}
	if marketType == model.MarketTypePerp {
		param["category"] = "linear"
	} else {
		param["category"] = "spot"
	}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/cancel-all", param)
	if httpErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to do post when cancelOrdersBybit %s`, httpErr.Error()))
		return
	}
	jsonData, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to NewJson when cancelOrdersBybit %s`, jsonErr.Error()))
		return
	}
	if jsonData != nil {
		code, _ := jsonData.Get("retCode").Int64()
		if code == 0 && jsonData.Get(`retMsg`).MustString() == `OK` {
			return true
		} else {
			util.Log(util.LogLevelError, fmt.Sprintf("fail to cancel bybit order, code: %d %s", code, string(httpResp)))
		}
	}
	return false
}

func getFundingRateBybit(symbol string) (fundingRate *model.FundingRate) {
	success, _, _, dialectSymbol := model.GetFromStandard(model.Bybit, symbol)
	if !success {
		util.Log(util.LogLevelError, "fail to get bybit perp funding rate, GetFromStandard: "+symbol)
		return
	}
	param := map[string]interface{}{"category": "linear", "symbol": dialectSymbol}
	composeParams := util.ComposeParams(param)
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bybitRestUrl+"/v5/market/tickers?"+composeParams, "", map[string]string{}, 30)
	bybitTickersResp := &dtos.BybitTickersResp{}
	perpJsonErr := json.Unmarshal(httpResp, bybitTickersResp)
	if bybitTickersResp == nil || bybitTickersResp.RetCode != 0 {
		util.Log(util.LogLevelError, fmt.Sprintf(
			"get bybit perp funding rate error, resp: %s, httpErr: %#v, jsonErr: %#v", httpResp, httpErr, perpJsonErr))
		return
	}
	for _, ticker := range bybitTickersResp.Result.List {
		rate, _ := strconv.ParseFloat(ticker.FundingRate, 64)
		nextFundingTime, _ := strconv.ParseInt(ticker.NextFundingTime, 10, 64)
		fundingRate = &model.FundingRate{
			Rate:       rate,
			UpdateTime: util.GetNow(),
			ExpireTime: nextFundingTime / 1000,
		}
	}
	return fundingRate
}

func parseOrderBybit(value map[string]interface{}, symbol string) (order *model.Order) {
	if value == nil {
		return nil
	}
	order = &model.Order{Market: model.Bybit, ClientOrdId: value["orderLinkId"].(string), Symbol: symbol}
	if value[`orderId`] != nil && value[`orderId`].(string) != `0` && value[`orderId`].(string) != `` {
		order.OrderId = value[`orderId`].(string)
	}
	if value[`price`] != nil && value[`price`] != `` {
		order.Price, _ = strconv.ParseFloat(value[`price`].(string), 64)
	}
	if value[`qty`] != nil {
		order.Amount, _ = strconv.ParseFloat(value[`qty`].(string), 64)
	}
	if strings.ToLower(value[`side`].(string)) == `buy` {
		order.OrderSide = model.OrderSideBuy
	} else if strings.ToLower(value[`side`].(string)) == `sell` {
		order.OrderSide = model.OrderSideSell
	}
	if value[`avgPrice`] != nil && value[`avgPrice`] != `` {
		order.DealPrice, _ = strconv.ParseFloat(value[`avgPrice`].(string), 64)
	}
	if value[`cumExecQty`] != nil && value[`cumExecQty`] != `` {
		order.DealAmount, _ = strconv.ParseFloat(value[`cumExecQty`].(string), 64)
	}
	if value[`orderType`] != nil { // market：市价单 limit：限价单 post_only：只做maker单 fok：全部成交或立即取消 ioc：立即成交并取消剩余
		switch strings.ToLower(value[`orderType`].(string)) {
		case `market`:
			order.OrderType = model.OrderTypeMarket
		case `limit`:
			order.OrderType = model.OrderTypeLimit
		}
	}
	if value[`stopOrderType`] != nil {
		switch strings.ToLower(value[`stopOrderType`].(string)) {
		case `Stop`:
			order.OrderType = model.OrderTypeStop
		case `TrailingStop`:
			order.OrderType = model.OrderTypeTrailStop
		}
	}
	if value[`triggerPrice`] != nil && value[`triggerPrice`] != `` {
		order.TriggerPrice, _ = strconv.ParseFloat(value[`triggerPrice`].(string), 64)
	}
	if value[`orderStatus`] != nil {
		status := value[`orderStatus`].(string)
		switch status {
		case `New`, `PartiallyFilled`, `Untriggered`:
			order.Status = model.CarryStatusWorking
		case `Rejected`, `Deactivated`, `Cancelled`, `PartiallyFilledCanceled`:
			order.Status = model.CarryStatusFail
		case `Filled`:
			order.Status = model.CarryStatusSuccess
		default:
			order.Status = model.CarryStatusFail
		}
	}
	if value[`cumExecFee`] != nil && value[`cumExecFee`] != `` { // 订单交易手续费，平台向用户收取的交易手续费，手续费扣除 为负数。如： -0.01
		order.Fee, _ = strconv.ParseFloat(value[`cumExecFee`].(string), 64)
	}
	if value[`createdTime`] != nil && value[`createdTime`] != `` {
		ts, _ := strconv.ParseInt(value[`createdTime`].(string), 10, 64)
		order.OrderTime = time.UnixMilli(ts)
	}
	if value[`1684738540561`] != nil && value[`1684738540561`] != `` {
		ts, _ := strconv.ParseInt(value[`1684738540561`].(string), 10, 64)
		order.OrderUpdateTime = time.UnixMilli(ts)
	}
	return order
}

func queryOpenOrdersBybit(key, secret, symbol string) (orders []*model.Order) {
	param := make(map[string]interface{})
	_, marketType, _, dialectSymbol := model.GetFromStandard(model.Bybit, symbol)
	if marketType == model.MarketTypePerp {
		param["category"] = "linear"
	} else {
		param["category"] = "spot"
	}
	param["symbol"] = dialectSymbol
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/order/realtime", param)
	if httpErr != nil {
		util.Log(util.LogLevelError, `queryOpenOrdersBybit http err `+httpErr.Error())
		return nil
	}
	respJson, jsonErr := util.NewJSON(httpResp)
	if jsonErr != nil || respJson == nil {
		msg := `queryOpenOrdersBybit json err `
		if jsonErr != nil {
			msg += jsonErr.Error()
		}
		util.Log(util.LogLevelError, msg)
		return nil
	}
	array := respJson.GetPath(`result`, `list`).MustArray()
	orders = make([]*model.Order, 0)
	for _, data := range array {
		order := parseOrderBybit(data.(map[string]interface{}), symbol)
		if order != nil {
			orders = append(orders, order)
		}
	}
	return orders
}

func queryOrderBybit(key, secret, symbol, orderId string) *model.Order {
	param := map[string]interface{}{"orderId": orderId}
	_, marketType, _, _ := model.GetFromStandard(model.Bybit, symbol)
	if marketType == model.MarketTypePerp {
		param["category"] = "linear"
	} else {
		param["category"] = "spot"
	}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/order/history", param)
	orderResp := &dtos.BybitOrderDetailResp{}
	jsonErr := json.Unmarshal(httpResp, orderResp)
	if orderResp == nil || orderResp.RetCode != 0 {
		util.Log(util.LogLevelError, fmt.Sprintf(
			"get bybit order detail error, resp: %s, httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
		return nil
	}
	order := &model.Order{Market: model.Bybit, Status: model.CarryStatusWorking, OrderId: orderId, Symbol: symbol}
	for _, orderDetail := range orderResp.Result.List {
		order.ClientOrdId = orderDetail.OrderLinkId
		order.Price, _ = strconv.ParseFloat(orderDetail.Price, 64)
		order.DealPrice, _ = strconv.ParseFloat(orderDetail.AvgPrice, 64)
		order.Amount, _ = strconv.ParseFloat(orderDetail.Qty, 64)
		order.DealAmount, _ = strconv.ParseFloat(orderDetail.CumExecQty, 64)
		order.UnfilledQuantity, _ = strconv.ParseFloat(orderDetail.LeavesQty, 64)
		intCreateTime, _ := strconv.ParseInt(orderDetail.CreatedTime, 10, 64)
		intUpdateTime, _ := strconv.ParseInt(orderDetail.UpdatedTime, 10, 64)
		order.OrderTime = time.UnixMilli(intCreateTime)
		order.OrderUpdateTime = time.UnixMilli(intUpdateTime)
		if orderDetail.OrderStatus == "Cancelled" || orderDetail.OrderStatus == "Rejected" || orderDetail.OrderStatus == "PartiallyFilledCanceled" {
			order.Status = model.CarryStatusFail
		} else if orderDetail.OrderStatus == "Filled" {
			order.Status = model.CarryStatusSuccess
		} else {
			util.Log(util.LogLevelError, fmt.Sprintf(
				"unkown bybit order detail status, resp: %s, httpErr: %#v, jsonErr: %#v", httpResp, httpErr, jsonErr))
		}
	}
	return order
}

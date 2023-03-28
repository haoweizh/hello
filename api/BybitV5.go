package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
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

//const bybitRestUrl = "https://api.bybit.com"
//const bybitSpotPubWsUrl = "wss://stream.bybit.com/v5/public/spot"
const bybitPerpPubWsUrl = "wss://stream.bybit.com/v5/public/linear"
const bybitPriWsUrl = "wss://stream.bybit.com/v5/private"

var channelMaintainingBybit = false

func getMarketsBybit(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	getMarketsBybitSpot(marketInfos)
	getMarketsBybitPerp(marketInfos)
	return marketInfos
}

//func getMarketsBybitSpot(marketInfos map[string]*model.MarketInfo) {
//	param := map[string]interface{}{"category": "spot", "limit": "1000"}
//	composeParams := util.ComposeParams(param)
//	httpResp, httpErr := util.HttpRequest(http.MethodGet, bybitRestUrl+"/v5/market/instruments-info?"+composeParams,
//		"", map[string]string{}, 30)
//	spotResp := &dtos.BybitSpotMarketResp{}
//	spotJsonErr := json.Unmarshal(httpResp, spotResp)
//	if spotResp == nil || spotResp.RetCode != 0 {
//		util.Notice(fmt.Sprintf("get bybit spot market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, spotJsonErr))
//		return
//	}
//	for _, symbolInfo := range spotResp.Result.List {
//		if symbolInfo.Status != "Trading" && symbolInfo.QuoteCoin != "USDT" {
//			continue
//		}
//		symbol := symbolInfo.BaseCoin + model.GetSpotTail(model.Bybit)
//		marketInfo := &model.MarketInfo{Name: symbol}
//		if symbolInfo.PriceFilter.TickSize == "" {
//			util.Notice(fmt.Sprintf("币种：%s 价格步长为空 resp：%v", symbol, symbolInfo))
//			continue
//		}
//		priceIncrement, _ := strconv.ParseFloat(symbolInfo.PriceFilter.TickSize, 64)
//		marketInfo.PriceIncrement = priceIncrement
//		marketInfo.PriceDecimal = util.NumDecPlaces(priceIncrement)
//
//		sizeIncrement, _ := strconv.ParseFloat(symbolInfo.LotSizeFilter.BasePrecision, 64)
//		marketInfo.SizeIncrement = sizeIncrement
//		marketInfo.SizeMin, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MinOrderQty, 64)
//		marketInfo.SizeMax, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MaxOrderQty, 64)
//		marketInfo.UsdtMin, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MinOrderAmt, 64)
//		marketInfo.UsdtMax, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MaxOrderAmt, 64)
//		marketInfos[marketInfo.Name] = marketInfo
//	}
//}

func getBorrowingRateBybit(market, coin string) (success bool, borrowingRate *model.BorrowingRate) {
	collateralHttpResp, collateralHttpErr := SignedRequestBybit(model.AppConfig.BybitKey, model.AppConfig.BybitSecret, http.MethodGet, bybitRestUrl,
		"/v5/account/collateral-info", map[string]interface{}{"currency": coin})
	//fmt.Println(fmt.Sprintf("coin: %s resp: %s", coin, collateralHttpResp))
	collateralResp := &dtos.BybitCollateralResp{}
	jsonErr := json.Unmarshal(collateralHttpResp, collateralResp)
	if collateralResp == nil || collateralResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("get coin: %s bybit collateral resp error, resp: %s, httpErr: %v, jsonErr: %v",
			coin, collateralHttpResp, collateralHttpErr, jsonErr))
		return false, nil
	}
	borrowingRate = &model.BorrowingRate{UpdateTime: util.GetNow().Unix(), Borrowable: false}
	for _, collateral := range collateralResp.Result.List {
		if collateral.Borrowable {
			borrowingRate.Borrowable = true
			rate, _ := strconv.ParseFloat(collateral.HourlyBorrowRate, 64)
			borrowingRate.Rate = rate * 12
		}
	}
	return true, borrowingRate
}

func getMarketsBybitPerp(marketInfos map[string]*model.MarketInfo) {
	cursor := "init"
	for true {
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
			util.Notice(fmt.Sprintf("get bybit perp market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
			return
		}

		for _, perpInfo := range perpResp.Result.List {
			if perpInfo.Status != "Trading" || perpInfo.QuoteCoin != "USDT" || perpInfo.ContractType != "LinearPerpetual" {
				continue
			}
			symbol := perpInfo.BaseCoin + model.GetPerpTail(model.Bybit)
			marketInfo := &model.MarketInfo{Name: symbol}
			marketInfo.PriceIncrement, _ = strconv.ParseFloat(perpInfo.PriceFilter.TickSize, 64)
			marketInfo.PriceDecimal, _ = strconv.Atoi(perpInfo.PriceScale)
			marketInfo.PriceMax, _ = strconv.ParseFloat(perpInfo.PriceFilter.MaxPrice, 64)
			priceMin, _ := strconv.ParseFloat(perpInfo.PriceFilter.MinPrice, 64)
			if priceMin != marketInfo.PriceIncrement {
				util.Notice(fmt.Sprintf("最小价格和价格步长不一致 perp info：%v", perpInfo))
				continue
			}
			maxLeverage, _ := strconv.ParseFloat(perpInfo.LeverageFilter.MaxLeverage, 64)
			if maxLeverage < 5 {
				util.Notice(fmt.Sprintf("最大杠杆小于5 perp info：%v", perpInfo))
				continue
			}
			marketInfo.SizeMin, _ = strconv.ParseFloat(perpInfo.LotSizeFilter.MinOrderQty, 64)
			marketInfo.SizeIncrement, _ = strconv.ParseFloat(perpInfo.LotSizeFilter.QtyStep, 64)
			marketInfos[symbol] = marketInfo
		}

		cursor = perpResp.Result.NextPageCursor
		if cursor == "" {
			return
		}
	}
}

func parseBookOrder(markets *model.Markets, bookWsResp *dtos.BybitBookWsResp) {
	if bookWsResp.Data.S == "" {
		return
	}
	symbol := bookWsResp.Data.S
	bidAsk := model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
	bidAsk.Ts = int(bookWsResp.Ts)
	bidAsk.UpdateId = bookWsResp.Data.Seq
	haveOld, old := markets.GetBidAsk(symbol, model.Bybit)
	if bookWsResp.Type == "snapshot" {
		bidPrice, _ := strconv.ParseFloat(bookWsResp.Data.B[0][0], 64)
		bidAmount, _ := strconv.ParseFloat(bookWsResp.Data.B[0][1], 64)
		askPrice, _ := strconv.ParseFloat(bookWsResp.Data.A[0][0], 64)
		askAmount, _ := strconv.ParseFloat(bookWsResp.Data.A[0][1], 64)
		bid := model.Tick{Price: bidPrice, Amount: bidAmount}
		ask := model.Tick{Price: askPrice, Amount: askAmount}
		bidAsk.Bids = []model.Tick{bid}
		bidAsk.Asks = []model.Tick{ask}
	} else if bookWsResp.Type == "delta" {
		if !haveOld {
			util.Notice(fmt.Sprintf("币种：%s bidask没有bidask 却收到delta ws", symbol))
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
				bid := model.Tick{Price: bidPrice, Amount: bidAmount}
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
				ask := model.Tick{Price: askPrice, Amount: askAmount}
				bidAsk.Asks = []model.Tick{ask}
			}
		}
	} else {
		return
	}
	if haveOld && old.Ts > bidAsk.Ts {
		return
	}
	if bidAsk.Bids[0].Price == bidAsk.Asks[0].Price {
		fmt.Println(fmt.Sprintf(`symbol: %s 买一价和卖一价相同 
		oldBid: %v, oldAsk: %v
		wsBid：%v, wsAsk: %v`, symbol, old.Bids[0], old.Asks[0], bookWsResp.Data.B, bookWsResp.Data.A))
	}
	if bidAsk.Bids[0].Amount == 0 || bidAsk.Asks[0].Amount == 0 {
		fmt.Println(fmt.Sprintf(`symbol: %s 有数量为0, bidAsk: %v`, symbol, bidAsk))
	}
	if markets.SetBidAsk(symbol, model.Bybit, &bidAsk) {
		//fmt.Println(fmt.Sprintf("symbol: %s now bidAsk: %v", symbol, bidAsk))
		for function, handler := range model.GetFunctions(model.Bybit, symbol) {
			if handler != nil {
				settings := model.GetSetting(function, model.Bybit, symbol)
				for _, setting := range settings {
					go handler(setting, &bidAsk)
				}
			}
		}
	}
}

func WsDepthServeBybit(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	spotBookWsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		//fmt.Println(fmt.Sprintf("spot book data: %s", event))
		bookWsResp := &dtos.BybitBookWsResp{}
		jsonErr := json.Unmarshal(event, bookWsResp)
		if jsonErr != nil {
			util.Notice(`fail to unmarshal bybit book ws data json ` + jsonErr.Error())
			return
		}
		if strings.Contains(bookWsResp.Topic, "orderbook") {
			parseBookOrder(markets, bookWsResp)
		}
	}
	perpBookWsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		//fmt.Println(fmt.Sprintf("perp book data: %s", event))
		bookWsResp := &dtos.BybitBookWsResp{}
		jsonErr := json.Unmarshal(event, bookWsResp)
		if jsonErr != nil {
			util.Notice(`fail to unmarshal bybit book ws data json ` + jsonErr.Error())
			return
		}
		if strings.Contains(bookWsResp.Topic, "orderbook") {
			if bookWsResp.Data.S != "" {
				bookWsResp.Data.S = model.GetCoin(model.Bybit, bookWsResp.Data.S) + model.GetPerpTail(model.Bybit)
			}
			parseBookOrder(markets, bookWsResp)
		}
	}

	channels = make([]chan struct{}, 0)
	symbols := model.GetMarketSymbols(model.Bybit)
	spotSubscribes := make([]interface{}, 0)
	futureSubscribes := make([]interface{}, 0)
	for symbol := range symbols {
		if strings.Contains(symbol, model.GetPerpTail(model.Bybit)) {
			futureSubscribes = append(futureSubscribes, symbol)
		} else {
			spotSubscribes = append(spotSubscribes, symbol)
		}
	}

	spotBookChannels, spotBookErr := WebSocketClient(model.Bybit, bybitSpotPubWsUrl, model.SubscribeDepth,
		spotSubscribes, subscribeHandlerBybit, spotBookWsHandler, orderHandler, 10)
	if spotBookErr == nil {
		util.Notice(`finish connect public bybit spot book wss `)
		channels = append(channels, spotBookChannels...)
	}

	perpBookChannels, perpBookErr := WebSocketClient(model.Bybit, bybitPerpPubWsUrl, model.SubscribeDepth,
		futureSubscribes, subscribeHandlerBybit, perpBookWsHandler, orderHandler, 10)
	if perpBookErr == nil {
		util.Notice(`finish connect public bybit perp book wss `)
		channels = append(channels, perpBookChannels...)
	}
	time.Sleep(time.Second * 1)

	go maintainChannelBybit()
	return channels, nil
}

var subscribeHandlerBybit = func(connection *websocket.Conn, subscribes []interface{}, keyChannel string) error {
	var err error = nil
	var params []string
	for _, subscribe := range subscribes {
		if strings.Contains(subscribe.(string), model.GetPerpTail(model.Bybit)) {
			perpSymbol := strings.Split(subscribe.(string), "-")[0] + model.GetSpotTail(model.Bybit)
			params = append(params, fmt.Sprintf("orderbook.1.%s", perpSymbol))
		} else {
			params = append(params, fmt.Sprintf("orderbook.1.%s", subscribe.(string)))
		}
	}
	subscribeMap := make(map[string]interface{})
	subscribeMap["req_id"] = int(rand.Float64() * 10000)
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = params
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = sendToConnection(connection, subscribeMessage); err != nil {
		util.Notice(" bybit can not subscribe %s %s", subscribeMessage, err.Error())
	}
	util.Notice(`bybit subscribed ` + string(subscribeMessage))
	time.Sleep(100 * time.Millisecond)
	return err
}

func maintainChannelBybit() {
	if !channelMaintainingBybit {
		channelMaintainingBybit = true
		go func() {
			for true {
				time.Sleep(time.Second * 20)
				if err := sendToAllConnections(model.Bybit, []byte(`{"op": "ping"}`)); err != nil {
					util.Notice("bybit channel ping error " + err.Error())
				}
			}
		}()
	}
}

func getBalanceBybit(key string, secret string) (success bool, balances []*model.Balance, totalInUsd float64, collateral *model.Collateral) {
	coins := model.GetSettingCoins(model.FunctionCarry, model.Bybit)
	coinsStr := []string{"USDT"}
	for coin, _ := range coins {
		coinsStr = append(coinsStr, coin)
	}
	param := map[string]interface{}{"accountType": "UNIFIED", "coin": strings.Join(coinsStr, ",")}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/account/wallet-balance", param)
	balanceResp := &dtos.BybitBalanceResp{}
	jsonErr := json.Unmarshal(httpResp, balanceResp)
	if balanceResp == nil || balanceResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("fail to refresh spot balance bybit, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		time.Sleep(time.Second * 2)
		return getBalanceBybit(key, secret)
	} else {
		util.SocketInfo(fmt.Sprintf("get spot balance bybit success, resp: %s ", httpResp))
	}
	balances = make([]*model.Balance, 0)
	for _, account := range balanceResp.Result.List {
		if account.AccountType == "UNIFIED" {
			collateralAvailable, _ := strconv.ParseFloat(account.TotalAvailableBalance, 64)
			totalMaintenanceMargin, _ := strconv.ParseFloat(account.TotalMaintenanceMargin, 64)
			accountMMRate, _ := strconv.ParseFloat(account.AccountMMRate, 64)
			collateral = &model.Collateral{Available: collateralAvailable, Occupied: totalMaintenanceMargin, Rate: accountMMRate}
			for _, coinInfo := range account.Coin {
				balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Bybit, Coin: coinInfo.Coin}
				balance.Borrow, _ = strconv.ParseFloat(coinInfo.BorrowAmount, 64)
				canBorrow, _ := strconv.ParseFloat(coinInfo.AvailableToBorrow, 64)
				holdAmount, _ := strconv.ParseFloat(coinInfo.WalletBalance, 64)
				if coinInfo.Coin == "USDT" {
					holdAmount, _ = strconv.ParseFloat(coinInfo.AvailableToWithdraw, 64)
				}
				balance.Amount = holdAmount
				balance.Available = holdAmount
				balance.AvailableWithBorrow = math.Max(0, balance.Amount) + canBorrow
				usdValue, _ := strconv.ParseFloat(coinInfo.UsdValue, 64)
				if usdValue == 0 {
					priceGet, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.GetSpotTail(model.Bybit), model.Bybit)
					if priceGet {
						usdValue = balance.Amount * bidAsk.Bids[0].Price
					}
				}
				balance.UsdValue = usdValue
				// 該幣種不能作為保證金的抵押品，則該數值為0
				totalInUsd += usdValue
				balances = append(balances, balance)
			}
		}
	}
	return true, balances, totalInUsd, collateral
}

func getPositionsBybit(key, secret string) (success bool, positions []*model.Position, posBalance float64) {
	cursor := "init"
	positions = make([]*model.Position, 0)
	for true {
		param := map[string]interface{}{"category": "linear", "settleCoin": "USDT", "limit": "200"}
		if cursor != "" && cursor != "init" {
			param["cursor"] = cursor
		}
		positionHttpResp, positionHttpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/position/list", param)
		positionResp := &dtos.BybitPositionResp{}
		positionJsonErr := json.Unmarshal(positionHttpResp, positionResp)
		if positionResp == nil || positionResp.RetCode != 0 {
			util.Notice(fmt.Sprintf("fail to refresh perp position bybit, resp: %s httpErr: %v, jsonErr: %v", positionHttpResp, positionHttpErr, positionJsonErr))
			time.Sleep(time.Second * 2)
			return getPositionsBitget(key, secret)
		} else {
			util.SocketInfo(fmt.Sprintf("get perp position bybit success, resp: %s ", positionHttpResp))
		}

		for _, contract := range positionResp.Result.List {
			if contract.TradeMode != 0 {
				continue
			}
			currency := model.GetCoin(model.Bybit, contract.Symbol) + model.GetPerpTail(model.Bybit)
			position := &model.Position{Market: model.Bybit, Ts: util.GetNowUnixMillion(), Currency: currency}
			if contract.Side == "Buy" {
				position.Free, _ = strconv.ParseFloat(contract.Size, 64)
			} else if contract.Side == "Sell" {
				total, _ := strconv.ParseFloat(contract.Size, 64)
				position.Free = -1 * total
			} else {
				position.Free = 0
			}
			position.LeverRate, _ = strconv.ParseInt(contract.Leverage, 10, 64)
			position.EntryPrice, _ = strconv.ParseFloat(contract.AvgPrice, 64)
			position.BankruptcyPrice, _ = strconv.ParseFloat(contract.BustPrice, 64)
			position.LiquidationPrice, _ = strconv.ParseFloat(contract.LiqPrice, 64)
			position.Margin, _ = strconv.ParseFloat(contract.PositionMM, 64)
			positions = append(positions, position)
		}

		cursor = positionResp.Result.NextPageCursor
		if cursor == "" {
			return true, positions, 0
		}
	}
	return true, positions, 0
}

func SignedRequestBybit(key, secret, method, host, path string, body map[string]interface{}) ([]byte, error) {
	if key == `` || secret == `` {
		keys, secrets := model.AppConfig.GetKeys(model.Bybit)
		key = keys[0]
		secret = secrets[0]
	}
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

func upgradeBybitUta(key, secret string) {
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/account/upgrade-to-uta", nil)
	upgradeResp := &dtos.BybitUpgradeUtaResp{}
	jsonErr := json.Unmarshal(httpResp, upgradeResp)
	if upgradeResp == nil || upgradeResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("fail to refresh perp position bybit, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	} else {
		if upgradeResp.Result.UnifiedUpdateStatus == "FAIL" {
			util.Notice(fmt.Sprintf("bybit 升级统一保证金账户失败 原因：%v", upgradeResp.Result.UnifiedUpdateMsg))
		}
	}
}

func setBybitMarginMode(key, secret string) {
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/account/set-margin-mode", map[string]interface{}{"setMarginMode": "PORTFOLIO_MARGIN"})
	marginModeResp := &dtos.BybitMarginModeResp{}
	jsonErr := json.Unmarshal(httpResp, marginModeResp)
	if marginModeResp == nil || marginModeResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("fail to set bybit margin mode, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	}
}

func setBybitMarginLeverage(key, secret string) {
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/spot-margin-trade/switch-mode", map[string]interface{}{"spotMarginMode": "1"})
	jsonData, jsonErr := util.NewJSON(httpResp)
	code, _ := jsonData.Get("retCode").Int()
	if jsonData == nil || code != 0 {
		util.Notice(fmt.Sprintf("fail to switch bybit margin mode , resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		return
	}
	httpResp, httpErr = SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/spot-margin-trade/set-leverage", map[string]interface{}{"leverage": "5"})
	jsonData, jsonErr = util.NewJSON(httpResp)
	code, _ = jsonData.Get("retCode").Int()
	if jsonData == nil || code != 0 {
		util.Notice(fmt.Sprintf("fail to set bybit margin leverage, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	}
}

func setBybitPerpLeverage(key, secret string) {
	symbols := model.GetMarketSymbols(model.Bybit)
	for symbol, _ := range symbols {
		if strings.Contains(symbol, model.GetPerpTail(model.Bybit)) {
			perpSymbol := strings.Split(symbol, "-")[0] + model.GetSpotTail(model.Bybit)
			params := map[string]interface{}{"category": "linear", "buyLeverage": "5", "sellLeverage": "5", "symbol": perpSymbol}
			httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/position/set-leverage", params)
			jsonData, jsonErr := util.NewJSON(httpResp)
			code, _ := jsonData.Get("retCode").Int()
			if jsonData == nil || code != 0 {
				util.Notice(fmt.Sprintf("fail to set bybit perp leverage , resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func placeOrderBybit(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	priceSpot, decimalSpot := model.FormatPrice(model.Bybit, symbol, orderSide, price)
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Bybit, symbol, amount)))
	priceStr := util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
	if strings.Contains(symbol, model.GetPerpTail(model.Bybit)) {
		perpSymbol := strings.Split(symbol, "-")[0] + model.GetSpotTail(model.Bybit)
		var tradeSide, tradeOrderType string
		if orderSide == model.OrderSideBuy {
			tradeSide = "Buy"
		} else {
			tradeSide = "Sell"
		}
		if orderType == model.OrderTypeLimit {
			tradeOrderType = "Limit"
		} else if orderType == model.OrderTypeMarket {
			tradeOrderType = "Market"
		}
		param := map[string]interface{}{
			"category":  "linear",
			"symbol":    perpSymbol,
			"side":      tradeSide,
			"orderType": tradeOrderType,
			"qty":       amountStr,
			"price":     priceStr,
		}
		httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/create", param)
		bitgetOrderResp := &dtos.BybitOrderResp{}
		jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
		if bitgetOrderResp == nil || bitgetOrderResp.RetCode != 0 {
			util.Notice(fmt.Sprintf("fail to create bybit perp order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		} else {
			order.Status = model.CarryStatusWorking
			order.OrderId = bitgetOrderResp.Result.OrderId
		}
	} else {
		var tradeSide, tradeOrderType string
		if orderSide == model.OrderSideBuy {
			tradeSide = "Buy"
		} else {
			tradeSide = "Sell"
		}
		if orderType == model.OrderTypeLimit {
			tradeOrderType = "Limit"
		} else if orderType == model.OrderTypeMarket {
			tradeOrderType = "Market"
		}
		param := map[string]interface{}{
			"category":  "spot",
			"symbol":    symbol,
			"side":      tradeSide,
			"orderType": tradeOrderType,
			"qty":       amountStr,
			"price":     priceStr,
		}
		borrowingRate := model.GetBorrowingRate(model.Bybit, model.GetCoin(model.Bybit, symbol))
		if borrowingRate != nil && borrowingRate.Borrowable {
			param["isLeverage"] = "1"
		}
		httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/create", param)
		bitgetOrderResp := &dtos.BybitOrderResp{}
		jsonErr := json.Unmarshal(httpResp, bitgetOrderResp)
		if bitgetOrderResp == nil || bitgetOrderResp.RetCode != 0 {
			util.Notice(fmt.Sprintf("fail to create bybit spot order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		} else {
			order.Status = model.CarryStatusWorking
			order.OrderId = bitgetOrderResp.Result.OrderId
		}
	}
}

func cancelOrdersBybit(key, secret, symbol string) (result bool) {
	if strings.Contains(symbol, model.GetPerpTail(model.Bybit)) {
		perpSymbol := strings.Split(symbol, "-")[0] + model.GetSpotTail(model.Bybit)
		params := map[string]interface{}{
			"symbol":   perpSymbol,
			"category": "linear",
		}
		httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/cancel-all", params)
		jsonData, jsonErr := util.NewJSON(httpResp)
		code, _ := jsonData.Get("code").Int64()
		if jsonData == nil || code != 0 {
			util.Notice(fmt.Sprintf("fail to cancel bybit perp order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
			return false
		}
	} else {
		params := map[string]interface{}{
			"symbol":   symbol,
			"category": "spot",
		}
		httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodPost, bybitRestUrl, "/v5/order/cancel-all", params)
		jsonData, jsonErr := util.NewJSON(httpResp)
		code, _ := jsonData.Get("code").Int64()
		if jsonData == nil || code != 0 {
			util.Notice(fmt.Sprintf("fail to cancel bybit spot order resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
			return false
		}
	}
	return true
}

func getFundingRateBybit(symbol string) (fundingRate *model.FundingRate) {
	perpSymbol := strings.Split(symbol, "-")[0] + model.GetSpotTail(model.Bybit)
	param := map[string]interface{}{"category": "linear", "symbol": perpSymbol}
	composeParams := util.ComposeParams(param)
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bybitRestUrl+"/v5/market/tickers?"+composeParams, "", map[string]string{}, 30)
	bybitTickersResp := &dtos.BybitTickersResp{}
	perpJsonErr := json.Unmarshal(httpResp, bybitTickersResp)
	if bybitTickersResp == nil || bybitTickersResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("get bybit perp funding rate error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
		return
	}
	for _, ticker := range bybitTickersResp.Result.List {
		rate, _ := strconv.ParseFloat(ticker.FundingRate, 64)
		nextFundingTime, _ := strconv.ParseInt(ticker.NextFundingTime, 10, 64)
		fundingRate = &model.FundingRate{
			Rate:       rate,
			UpdateTime: util.GetNow().Unix(),
			ExpireTime: nextFundingTime / 1000,
		}
	}
	return fundingRate
}

func queryOrderBybit(key, secret string, order *model.Order) {
	param := map[string]interface{}{"orderId": order.OrderId}
	if strings.Contains(order.Symbol, model.GetPerpTail(model.Bybit)) {
		param["category"] = "linear"
	} else {
		param["category"] = "spot"
	}
	httpResp, httpErr := SignedRequestBybit(key, secret, http.MethodGet, bybitRestUrl, "/v5/order/history", param)
	orderResp := &dtos.BybitOrderDetailResp{}
	jsonErr := json.Unmarshal(httpResp, orderResp)
	if orderResp == nil || orderResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("get bybit order detail error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		return
	}
	for _, orderDetail := range orderResp.Result.List {
		order.DealPrice, _ = strconv.ParseFloat(orderDetail.AvgPrice, 64)
		order.DealAmount, _ = strconv.ParseFloat(orderDetail.CumExecQty, 64)
		order.UnfilledQuantity, _ = strconv.ParseFloat(orderDetail.LeavesQty, 64)
		order.Status = model.CarryStatusWorking
		if orderDetail.OrderStatus == "Cancelled" || orderDetail.OrderStatus == "Rejected" {
			order.Status = model.CarryStatusFail
		} else if orderDetail.OrderStatus == "Filled" || orderDetail.OrderStatus == "PartiallyFilled" || orderDetail.OrderStatus == "PartiallyFilledCanceled" {
			order.Status = model.CarryStatusSuccess
		} else {
			util.Notice(fmt.Sprintf("unkown bybit order detail status, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
		}
	}
}

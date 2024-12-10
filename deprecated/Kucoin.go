package deprecated

//var relatedSettingMarkets = make(map[string]bool)

//func kucoinFutureClient(key, secret, passPhrase string) *kumex.ApiService {
//	if key == "" || secret == "" || passPhrase == "" {
//		key = model.AppConfig.KucoinFutureKey
//		secret = model.AppConfig.KucoinFutureSecret
//		passPhrase = model.AppConfig.Phase
//	}
//	client := kumex.NewApiService(
//		kumex.ApiKeyOption(key),
//		kumex.ApiSecretOption(secret),
//		kumex.ApiPassPhraseOption(passPhrase),
//		kumex.ApiKeyVersionOption("2"))
//	return client
//}

//func kucoinRelatedClient(key, secret, passPhrase string) *kucoin.ApiService {
//	if key == "" || secret == "" || passPhrase == "" {
//		key = model.AppConfig.KucoinRelatedKey
//		secret = model.AppConfig.KucoinRelatedSecret
//		passPhrase = model.AppConfig.Phase
//	}
//	client := kucoin.NewApiService(
//		kucoin.ApiKeyOption(key),
//		kucoin.ApiSecretOption(secret),
//		kucoin.ApiPassPhraseOption(passPhrase),
//		kucoin.ApiKeyVersionOption("2"))
//	return client
//}

//func setFutureAutoDeposit() {
//	coins := model.GetSettingCoins(model.FunctionCarry, model.Kucoin)
//	for coin := range coins {
//		params := make(map[string]string)
//		params["symbol"] = coin + `USDTM`
//		params["status"] = "true"
//		resp, err := kucoinFutureClient("", "", "").AutoDepositStatus(params)
//		if err != nil || !resp.HttpSuccessful() || !resp.ApiSuccessful() {
//			util.SocketInfo(fmt.Sprintf("function: %s symbol: %s kucoin API error", "setFutureAutoDeposit", params["symbol"]))
//		}
//	}
//
//}

//func getMarketsKucoin(key string) (success bool, marketInfos map[string]*model.MarketInfo) {
//	marketInfos = make(map[string]*model.MarketInfo)
//	appendRelatedMarketsKucoin(key, marketInfos)
//	appendFutureMarketKucoin(key, marketInfos)
//	return true, marketInfos
//}

//func appendRelatedMarketsKucoin(key string, marketInfos map[string]*model.MarketInfo) {
//	client := kucoinRelatedClient("", "", "")
//	resp, err := client.Symbols("")
//	if err != nil {
//		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API error", key, "appendRelatedMarketsKucoin"))
//		return
//	}
//	symbols := kucoin.SymbolsModel{}
//	if err := resp.ReadData(&symbols); err != nil {
//		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API read data error", key, "appendRelatedMarketsKucoin"))
//		return
//	}
//	for _, related := range symbols {
//		if !related.EnableTrading || related.QuoteCurrency != `USDT` {
//			continue
//		}
//		success, marketType, coin := model.GetCoinFromDialect(model.Kucoin, related.Symbol)
//		if !model.AppConfig.KucoinSpot && !related.IsMarginEnabled || !success {
//			continue
//		}
//		marketInfo := &model.MarketInfo{Market: model.Kucoin}
//		// TODO 此处需要确保kucoin的现货和期货的tail不同，否则marketTYpe不可用
//		marketInfo.Name = coin + model.UniStandardTail[marketType]
//		marketInfo.PriceIncrement, _ = strconv.ParseFloat(related.PriceIncrement, 64)
//		marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
//		marketInfo.SizeMin, _ = strconv.ParseFloat(related.BaseMinSize, 64)
//		marketInfo.SizeMax, _ = strconv.ParseFloat(related.BaseMaxSize, 64)
//		marketInfo.SizeIncrement, _ = strconv.ParseFloat(related.BaseIncrement, 64)
//		marketInfos[marketInfo.Name] = marketInfo
//	}
//	relatedSettingMarkets = model.GetMarketSymbols(model.Kucoin)
//}

//type KucoinContractModel struct {
//	BaseCurrency       string  `json:"baseCurrency"`
//	FairMethod         string  `json:"fairMethod"`
//	FundingBaseSymbol  string  `json:"fundingBaseSymbol"`
//	FundingQuoteSymbol string  `json:"fundingQuoteSymbol"`
//	FundingRateSymbol  string  `json:"fundingRateSymbol"`
//	IndexSymbol        string  `json:"indexSymbol"`
//	InitialMargin      float64 `json:"initialMargin"`
//	IsDeleverage       bool    `json:"isDeleverage"`
//	IsInverse          bool    `json:"isInverse"`
//	IsQuanto           bool    `json:"isQuanto"`
//	LotSize            float64 `json:"lotSize"`
//	MaintainMargin     float64 `json:"maintainMargin"`
//	MakerFeeRate       float64 `json:"makerFeeRate"`
//	MakerFixFee        float64 `json:"makerFixFee"`
//	MarkMethod         string  `json:"markMethod"`
//	MaxOrderQty        float64 `json:"maxOrderQty"`
//	MaxPrice           float64 `json:"maxPrice"`
//	MaxRiskLimit       float64 `json:"maxRiskLimit"`
//	MinRiskLimit       float64 `json:"minRiskLimit"`
//	Multiplier         float64 `json:"multiplier"`
//	QuoteCurrency      string  `json:"quoteCurrency"`
//	RiskStep           int     `json:"riskStep"`
//	RootSymbol         string  `json:"rootSymbol"`
//	Status             string  `json:"status"`
//	Symbol             string  `json:"symbol"`
//	TakerFeeRate       float64 `json:"takerFeeRate"`
//	TakerFixFee        float64 `json:"takerFixFee"`
//	TickSize           float64 `json:"tickSize"`
//	Type               string  `json:"type"`
//	MaxLeverage        float64 `json:"maxLeverage"`
//	VolumeOf24h        float64 `json:"volumeOf24h"`
//	TurnoverOf24h      float64 `json:"turnoverOf24h"`
//	OpenInterest       string  `json:"openInterest"`
//}
//type KucoinContractsModels []*KucoinContractModel

//func appendFutureMarketKucoin(key string, marketInfos map[string]*model.MarketInfo) {
//	client := kucoinFutureClient("", "", "")
//	resp, err := client.ActiveContracts()
//	if err != nil {
//		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API error", key, "appendFutureMarketKucoin"))
//		return
//	}
//	contracts := KucoinContractsModels{}
//	if err := resp.ReadData(&contracts); err != nil {
//		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API read data error", key, "appendFutureMarketKucoin"))
//		return
//	}
//	for _, contract := range contracts {
//		if contract.Status != "Open" || contract.QuoteCurrency != `USDT` {
//			continue
//		}
//		marketInfo := &model.MarketInfo{Market: model.Kucoin}
//		marketInfo.Name = contract.BaseCurrency + model.UniStandardTail[model.MarketTypePerp]
//		marketInfo.PriceIncrement = contract.TickSize
//		marketInfo.PriceDecimal = util.NumDecPlaces(contract.TickSize)
//		marketInfo.SizeMin = contract.LotSize
//		marketInfo.SizeIncrement = marketInfo.SizeMin
//		marketInfo.CTCurrency = contract.BaseCurrency
//		marketInfo.SizeMax = contract.MaxOrderQty
//		//marketInfo.PriceMax = contract.MaxPrice
//		marketInfo.CTValue = contract.Multiplier
//		marketInfos[marketInfo.Name] = marketInfo
//	}
//}

//func WsDepthServeKucoin() (channels []chan struct{}, err error) {
//	relatedClient := kucoinRelatedClient("", "", "")
//	futureClient := kucoinFutureClient("", "", "")
//	relatedRsp, relatedErr := relatedClient.WebSocketPublicToken()
//	futureRsp, futureErr := futureClient.WebSocketPublicToken()
//	if relatedErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket error:%s", "WsDepthServeKucoin", relatedErr))
//		return channels, relatedErr
//	}
//	if futureErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket error:%s", "WsDepthServeKucoin", futureErr))
//		return channels, futureErr
//	}
//	relatedToken := &kucoin.WebSocketTokenModel{}
//	if relatedTokenErr := relatedRsp.ReadData(relatedToken); relatedTokenErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket error:%s", "WsDepthServeKucoin", relatedTokenErr))
//		return channels, relatedTokenErr
//	}
//	futureToken := &kumex.WebSocketTokenModel{}
//	if futureTokenErr := futureRsp.ReadData(futureToken); futureTokenErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket error:%s", "WsDepthServeKucoin", futureTokenErr))
//		return channels, futureTokenErr
//	}
//	relatedChannel := relatedClient.NewWebSocketClient(relatedToken)
//	relatedMsg, relatedChannelError, relatedConnectErr := relatedChannel.Connect()
//	if relatedConnectErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket connect error:%s", "WsDepthServeKucoin", relatedConnectErr))
//		retrySuccess := false
//		for i := 0; i < 10; i++ {
//			i++
//			relatedChannel = relatedClient.NewWebSocketClient(relatedToken)
//			relatedMsg, relatedChannelError, relatedConnectErr = relatedChannel.Connect()
//			if relatedConnectErr != nil {
//				util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket connect retry：%d error:%s", "WsDepthServeKucoin", i, relatedConnectErr))
//				time.Sleep(time.Minute * 5)
//				continue
//			} else {
//				retrySuccess = true
//				util.SocketInfo(fmt.Sprintf("kucoin related websocket connect retry success"))
//				break
//			}
//		}
//		if !retrySuccess {
//			return channels, relatedConnectErr
//		}
//	}
//	futureChannel := futureClient.NewWebSocketClient(futureToken)
//	futureMsg, futureChannelError, futureConnectErr := futureChannel.Connect()
//	if futureConnectErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket connect error:%s", "WsDepthServeKucoin", futureConnectErr))
//		retrySuccess := false
//		for i := 0; i < 10; i++ {
//			i++
//			futureChannel = futureClient.NewWebSocketClient(futureToken)
//			futureMsg, futureChannelError, futureConnectErr = futureChannel.Connect()
//			if futureConnectErr != nil {
//				util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket connect retry：%d error:%s", "WsDepthServeKucoin", i, futureConnectErr))
//				time.Sleep(time.Minute * 5)
//				continue
//			} else {
//				retrySuccess = true
//				util.SocketInfo(fmt.Sprintf("kucoin future websocket connect retry success"))
//				break
//			}
//		}
//		if !retrySuccess {
//			return channels, futureConnectErr
//		}
//	}
//	symbols := model.GetMarketSymbols(model.Kucoin)
//	futureSubscribes := make([]*kumex.WebSocketSubscribeMessage, 0)
//	for symbol := range symbols {
//		success, marketType, _, _ := model.GetFromStandard(model.Kucoin, symbol)
//		if success && marketType == model.MarketTypePerp {
//			topic := "/contractMarket/tickerV2:" + strings.Split(symbol, "-")[0] + `USDTM`
//			futureSubscribes = append(futureSubscribes, kumex.NewSubscribeMessage(topic, false))
//		}
//	}
//	relatedSubscribe := kucoin.NewSubscribeMessage("/market/ticker:all", false)
//
//	if relatedSubscribeErr := relatedChannel.Subscribe(relatedSubscribe); relatedSubscribeErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket subscribe error:%s", "WsDepthServeKucoin", relatedSubscribeErr))
//		return channels, relatedSubscribeErr
//	}
//	util.Notice(fmt.Sprintf("kucoin finish create related websocket subscribe"))
//	relatedStopC := make(chan struct{}, 10)
//	go handlerKucoinRelatedWS(relatedChannelError, relatedMsg, relatedChannel, relatedStopC)
//	channels = append(channels, relatedStopC)
//	if futureSubscribeErr := futureChannel.Subscribe(futureSubscribes...); futureSubscribeErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket subscribe error:%s", "WsDepthServeKucoin", futureSubscribeErr))
//		return channels, futureSubscribeErr
//	}
//	util.Notice(fmt.Sprintf("kucoin finish create future websocket subscribe"))
//	futureStopC := make(chan struct{}, 10)
//	go handlerKucoinFutureWS(futureChannelError, futureMsg, futureChannel, futureStopC)
//	channels = append(channels, futureStopC)
//	return channels, err
//}

//func handlerKucoinFutureWS(futureChannelError <-chan error, futureMsg <-chan *kumex.WebSocketDownstreamMessage, channel *kumex.WebSocketClient, stopC chan struct{}) {
//	defer func() {
//		channel.Stop()
//	}()
//	for {
//		select {
//		case <-stopC:
//			util.Notice("get stop struct, return")
//			return
//		case cError := <-futureChannelError:
//			util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket channel error:%s", "WsDepthServeKucoin", cError.Error()))
//			return
//		case msg := <-futureMsg:
//			handleKucoinWS(nil, msg)
//		}
//	}
//}

//func handlerKucoinRelatedWS(relatedChannelError <-chan error, relatedMsg <-chan *kucoin.WebSocketDownstreamMessage, channel *kucoin.WebSocketClient, stopC chan struct{}) {
//	defer func() {
//		channel.Stop()
//	}()
//	for {
//		select {
//		case <-stopC:
//			util.Notice("get stop struct, return")
//			return
//		case cError := <-relatedChannelError:
//			util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket channel error:%s", "WsDepthServeKucoin", cError.Error()))
//			return
//		case msg := <-relatedMsg:
//			handleKucoinWS(msg, nil)
//		}
//	}
//}

//func handleKucoinWS(relatedMsg *kucoin.WebSocketDownstreamMessage, futureMsg *kumex.WebSocketDownstreamMessage) {
//	if futureMsg != nil && strings.Contains(futureMsg.Topic, "/contractMarket/tickerV2") {
//		ticker := &kumex.TickerLevel1Model{}
//		if err := futureMsg.ReadData(ticker); err != nil {
//			util.Notice(fmt.Sprintf("future ticker Unmarshal err:%s", err.Error()))
//		}
//		if ticker.Symbol == "" {
//			return
//		}
//		symbol := strings.ReplaceAll(ticker.Symbol, `USDTM`, ``) + model.UniStandardTail[model.MarketTypePerp]
//		now := int(time.Now().UnixNano() / int64(time.Millisecond))
//		ts := int(ticker.Ts / int64(time.Millisecond))
//		bidPrice, _ := strconv.ParseFloat(ticker.BestBidPrice, 64)
//		_, bidAmount := model.ParseRealAmount(model.Kucoin, symbol, float64(ticker.BestBidSize))
//		askPrice, _ := strconv.ParseFloat(ticker.BestAskPrice, 64)
//		_, askAmount := model.ParseRealAmount(model.Kucoin, symbol, float64(ticker.BestAskSize))
//		bidAsk := model.BidAsk{Ts: ts, TsReceived: now,
//			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.Kucoin, Symbol: symbol, Side: model.OrderSideBuy}},
//			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.Kucoin, Symbol: symbol, Side: model.OrderSideSell}}}
//		markets := model.AppMarkets
//		haveOld, old := markets.GetBidAsk(symbol, model.Kucoin)
//		if haveOld && old.Ts > bidAsk.Ts {
//			return
//		}
//		if markets.SetBidAsk(symbol, model.Kucoin, &bidAsk) {
//			for function, handler := range model.GetFunctions(model.Kucoin, symbol) {
//				if handler != nil {
//					setting := model.GetSetting(function, model.Kucoin, symbol)
//					if setting != nil {
//						go handler(setting, &bidAsk)
//					}
//				}
//			}
//		}
//	} else {
//		ticker := &kucoin.TickerLevel1Model{}
//		if err := relatedMsg.ReadData(ticker); err != nil {
//			util.Notice(fmt.Sprintf("jsonerr Unmarshal err:%s", err.Error()))
//		}
//		if relatedMsg.Subject == "" {
//			return
//		}
//		if relatedSettingMarkets[relatedMsg.Subject] == false {
//			return
//		}
//		success, marketType, coin := model.GetCoinFromDialect(model.Kucoin, relatedMsg.Subject)
//		if !success {
//			return
//		}
//		// TODO 需要确保Kucoin的期货现货tail不同，否则marketType不可用
//		symbol := coin + model.UniStandardTail[marketType]
//		now := int(time.Now().UnixNano() / int64(time.Millisecond))
//		//util.Notice(fmt.Sprintf("币种：%s，当前ts：%d", symbol, now))
//		updateId, _ := strconv.ParseInt(ticker.Sequence, 10, 64)
//		bidPrice, _ := strconv.ParseFloat(ticker.BestBid, 64)
//		bidAmount, _ := strconv.ParseFloat(ticker.BestBidSize, 64)
//		askPrice, _ := strconv.ParseFloat(ticker.BestAsk, 64)
//		askAmount, _ := strconv.ParseFloat(ticker.BestAskSize, 64)
//		bidAsk := model.BidAsk{Ts: now, TsReceived: now, UpdateId: updateId,
//			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.Kucoin, Symbol: symbol, Side: model.OrderSideBuy}},
//			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.Kucoin, Symbol: symbol, Side: model.OrderSideSell}}}
//		markets := model.AppMarkets
//		haveOld, old := markets.GetBidAsk(symbol, model.Kucoin)
//		if haveOld && old.UpdateId > bidAsk.UpdateId {
//			return
//		}
//		if markets.SetBidAsk(symbol, model.Kucoin, &bidAsk) {
//			for function, handler := range model.GetFunctions(model.Kucoin, symbol) {
//				if handler != nil {
//					setting := model.GetSetting(function, model.Kucoin, symbol)
//					if setting != nil {
//						go handler(setting, &bidAsk)
//					}
//				}
//			}
//		}
//	}
//}

//func getBalanceKucoin(key string, secret string) (success bool, balances []*model.Balance) {
//	if model.AppConfig.KucoinSpot {
//		accountResp, err := kucoinRelatedClient("", "", "").Accounts("", "trade")
//		if err != nil {
//			util.SocketInfo(fmt.Sprintf("fail to refresh spot balance kucoin, err:%s", err))
//			time.Sleep(time.Minute * 5)
//			return getBalanceKucoin(key, secret)
//		}
//		marshal, _ := json.Marshal(accountResp)
//		util.SocketInfo(fmt.Sprintf(`get spot balance response: %s`, marshal))
//		spotAccounts := &kucoin.AccountsModel{}
//		respError := accountResp.ReadData(spotAccounts)
//		if respError != nil {
//			util.SocketInfo(fmt.Sprintf("fail to get spot balance response kucoin, err:%s", respError))
//			return false, balances
//		}
//		balances = make([]*model.Balance, 0)
//		for _, account := range *spotAccounts {
//			balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Kucoin, Coin: account.Currency}
//			balance.FrozenAmount, _ = strconv.ParseFloat(account.Holds, 64)
//			balance.Amount, _ = strconv.ParseFloat(account.Balance, 64)
//			balance.AvailableWithBorrow, _ = strconv.ParseFloat(account.Available, 64)
//			_, price := model.AppMarkets.GetPriceForce(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.Kucoin)
//			balance.UsdValue = balance.Amount * price
//			balances = append(balances, balance)
//		}
//	} else {
//		accountResp, err := kucoinRelatedClient("", "", "").MarginAccount()
//		if err != nil {
//			util.SocketInfo(fmt.Sprintf("fail to refresh margin balance kucoin, err:%s", err))
//			time.Sleep(time.Minute * 5)
//			return getBalanceKucoin(key, secret)
//		}
//		marshal, _ := json.Marshal(accountResp)
//		util.SocketInfo(fmt.Sprintf(`get margin balance response: %s`, marshal))
//		marginAccount := &kucoin.MarginAccountModel{}
//		respError := accountResp.ReadData(marginAccount)
//		if respError != nil {
//			util.SocketInfo(fmt.Sprintf("fail to get margin balance response kucoin, err:%s", respError))
//			return false, balances
//		}
//		balances = make([]*model.Balance, 0)
//		for _, account := range marginAccount.Accounts {
//			balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Kucoin, Coin: account.Currency}
//			balance.FrozenAmount, _ = account.HoldBalance.Float64()
//			available, _ := account.AvailableBalance.Float64()
//			balance.Borrow, _ = account.Liability.Float64()
//			canBorrow, _ := account.MaxBorrowSize.Float64()
//			balance.AvailableWithBorrow = available + canBorrow
//			balance.Amount, _ = account.TotalBalance.Float64()
//			balance.Amount = balance.Amount - balance.Borrow
//			_, price := model.AppMarkets.GetPriceForce(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.Kucoin)
//			balance.UsdValue = balance.Amount * price
//			balances = append(balances, balance)
//		}
//	}
//	return true, balances
//}

//type KucoinPositionModel struct {
//	Id                string  `json:"id"`
//	Symbol            string  `json:"symbol"`
//	AutoDeposit       bool    `json:"autoDeposit"`
//	MaintMarginReq    float64 `json:"maintMarginReq"`
//	RiskLimit         int     `json:"riskLimit"`
//	RealLeverage      float64 `json:"realLeverage"`
//	CrossMode         bool    `json:"crossMode"`
//	DelevPercentage   float64 `json:"delevPercentage"`
//	OpeningTimestamp  int64   `json:"openingTimestamp"`
//	CurrentTimestamp  int64   `json:"currentTimestamp"`
//	CurrentQty        int64   `json:"currentQty"`
//	CurrentCost       float64 `json:"currentCost"`
//	CurrentComm       float64 `json:"currentComm"`
//	UnrealisedCost    float64 `json:"unrealisedCost"`
//	RealisedGrossCost float64 `json:"realisedGrossCost"`
//	RealisedCost      float64 `json:"realisedCost"`
//	IsOpen            bool    `json:"isOpen"`
//	MarkPrice         float64 `json:"markPrice"`
//	MarkValue         float64 `json:"markValue"`
//	PosCost           float64 `json:"posCost"`
//	PosCross          float64 `json:"posCross"`
//	PosInit           float64 `json:"posInit"`
//	PosComm           float64 `json:"posComm"`
//	PosLoss           float64 `json:"posLoss"`
//	PosMargin         float64 `json:"posMargin"`
//	PosMaint          float64 `json:"posMaint"`
//	MaintMargin       float64 `json:"maintMargin"`
//	RealisedGrossPnl  float64 `json:"realisedGrossPnl"`
//	RealisedPnl       float64 `json:"realisedPnl"`
//	UnrealisedPnl     float64 `json:"unrealisedPnl"`
//	UnrealisedPnlPcnt float64 `json:"unrealisedPnlPcnt"`
//	UnrealisedRoePcnt float64 `json:"unrealisedRoePcnt"`
//	AvgEntryPrice     float64 `json:"avgEntryPrice"`
//	LiquidationPrice  float64 `json:"liquidationPrice"`
//	BankruptPrice     float64 `json:"bankruptPrice"`
//	SettleCurrency    string  `json:"settleCurrency"`
//}
//
//type PositionsModel []*KucoinPositionModel

//func getPositionsKucoin(key string, secret string) (success bool, positions []*model.Position, accountValue, availableU float64) {
//	params := make(map[string]string)
//	params["currency"] = `USDT`
//	accountResp, accountErr := kucoinFutureClient("", "", "").AccountOverview(params)
//	contractResp, err := kucoinFutureClient("", "", "").Positions()
//	if err != nil || accountErr != nil {
//		if accountErr != nil {
//			util.SocketInfo(fmt.Sprintf("fail to refresh future account kucoin, err:%s", err))
//		}
//		if err != nil {
//			util.SocketInfo(fmt.Sprintf("fail to refresh future position kucoin, err:%s", err))
//		}
//		time.Sleep(time.Minute * 5)
//		return getPositionsKucoin(key, secret)
//	}
//	account := &kumex.AccountModel{}
//	accountRespError := accountResp.ReadData(account)
//	if accountRespError != nil {
//		util.SocketInfo(fmt.Sprintf("fail to get future account response kucoin, err:%s", accountRespError))
//		return false, positions, 0, 0
//	}
//	accountRespJson, _ := json.Marshal(account)
//	util.SocketInfo(fmt.Sprintf(`get future account response: %s`, accountRespJson))
//	accountValue = account.AccountEquity
//	availableU = account.AvailableBalance
//	contracts := &PositionsModel{}
//	contractRespError := contractResp.ReadData(contracts)
//	if contractRespError != nil {
//		util.SocketInfo(fmt.Sprintf("fail to get future position response kucoin, err:%s", contractRespError))
//		return false, positions, 0, 0
//	}
//	contractRespJson, _ := json.Marshal(contracts)
//	util.SocketInfo(fmt.Sprintf(`get future position response: %s`, contractRespJson))
//	positions = make([]*model.Position, 0)
//	for _, contract := range *contracts {
//		currency := strings.ReplaceAll(contract.Symbol, `USDTM`, ``) + model.UniStandardTail[model.MarketTypePerp]
//		position := &model.Position{Market: model.Kucoin, Ts: util.GetNowUnixMillion(), Currency: currency}
//		_, realAmount := model.ParseRealAmount(model.Kucoin, currency, float64(contract.CurrentQty))
//		position.Holding = realAmount
//		position.LeverRate = int64(contract.RealLeverage)
//		position.EntryPrice = contract.AvgEntryPrice
//		position.Margin = contract.PosMargin
//		position.LiquidationPrice = contract.LiquidationPrice
//		position.ProfitUnreal = contract.UnrealisedPnl
//		if position.Holding != 0 {
//			positions = append(positions, position)
//		}
//	}
//	return true, positions, accountValue, availableU
//}

//func cancelOrdersKucoin(symbol string) (result bool) {
//	success, marketType, _, _ := model.GetFromStandard(model.Kucoin, symbol)
//	if success && marketType == model.MarketTypePerp {
//		symbol = strings.Split(symbol, "-")[0] + `USDTM`
//		apiResponse, err := kucoinFutureClient("", "", "").CancelOrders(symbol)
//		if err != nil {
//			util.SocketInfo(fmt.Sprintf("function: %s fail to cancel future orders kucoin, err:%s", "cancelOrdersKucoin", err))
//			return false
//		}
//		orders := &kumex.CancelOrderResultModel{}
//		if cancelErr := apiResponse.ReadData(orders); cancelErr != nil {
//			util.SocketInfo(fmt.Sprintf("fail to get cancel future orders response kucoin, err:%s", cancelErr))
//			return false
//		}
//	} else if success && marketType == model.MarketTypeSpot {
//		param := map[string]string{}
//		if model.AppConfig.KucoinSpot {
//			param["tradeType"] = "TRADE"
//		} else {
//			param["tradeType"] = "MARGIN_TRADE"
//		}
//		param["symbol"] = symbol
//		req := kucoin.NewRequest(http.MethodDelete, "/api/v1/orders", param)
//		apiResponse, err := kucoinRelatedClient("", "", "").Call(req)
//		if err != nil {
//			util.SocketInfo(fmt.Sprintf("function: %s fail to cancel related orders kucoin, err:%s", "cancelOrdersKucoin", err))
//			return false
//		}
//		orders := &kucoin.CancelOrderResultModel{}
//		if cancelErr := apiResponse.ReadData(orders); cancelErr != nil {
//			util.SocketInfo(fmt.Sprintf("fail to get cancel related orders response kucoin, err:%s", cancelErr))
//			return false
//		}
//	}
//	return true
//}

//type CreateMarginOrderModel struct {
//	// BASE PARAMETERS
//	ClientOid  string `json:"clientOid"`
//	Side       string `json:"side"`
//	Symbol     string `json:"symbol,omitempty"`
//	Type       string `json:"type,omitempty"`
//	Remark     string `json:"remark,omitempty"`
//	Stop       string `json:"stop,omitempty"`
//	StopPrice  string `json:"stopPrice,omitempty"`
//	STP        string `json:"stp,omitempty"`
//	TradeType  string `json:"tradeType,omitempty"`
//	MarginMode string `json:"marginMode,omitempty"`
//	AutoBorrow bool   `json:"autoBorrow,omitempty"`
//
//	// LIMIT ORDER PARAMETERS
//	Price       string `json:"price,omitempty"`
//	Size        string `json:"size,omitempty"`
//	TimeInForce string `json:"timeInForce,omitempty"`
//	CancelAfter uint64 `json:"cancelAfter,omitempty"`
//	PostOnly    bool   `json:"postOnly,omitempty"`
//	Hidden      bool   `json:"hidden,omitempty"`
//	IceBerg     bool   `json:"iceberg,omitempty"`
//	VisibleSize string `json:"visibleSize,omitempty"`
//
//	// MARKET ORDER PARAMETERS
//	// Size  string `json:"size"`
//	Funds string `json:"funds,omitempty"`
//}

//func placeOrderKucoin(order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
//	success, marketType, _, _ := model.GetFromStandard(model.Kucoin, symbol)
//	if success && marketType == model.MarketTypePerp {
//		futureSymbol := strings.Split(symbol, "-")[0] + `USDTM`
//		params := make(map[string]string)
//		params["clientOid"] = "f" + strconv.FormatInt(time.Now().UnixNano(), 10)
//		params["side"] = orderSide
//		params["symbol"] = futureSymbol
//		params["type"] = orderType
//		params["leverage"] = "10"
//		priceFuture, decimalFuture := model.FormatPrice(model.Kucoin, symbol, orderSide, price)
//		params["price"] = util.CutTailZero(strconv.FormatFloat(priceFuture, 'f', decimalFuture, 64))
//		params["size"] = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Kucoin, symbol, amount, price)))
//		util.SocketInfo(fmt.Sprintf(`create future order request: %s`, params))
//		futureOrderResp, err := kucoinFutureClient("", "", "").CreateOrder(params)
//		if err != nil {
//			util.SocketInfo(fmt.Sprintf("function: %s fail to create future order kucoin, err:%s", "placeOrderKucoin", err))
//			order.Status = model.CarryStatusFail
//			order.OrderId = ``
//			return
//		} else {
//			util.SocketInfo(fmt.Sprintf(`create future order response: %#v`, futureOrderResp))
//			orderResult := &kumex.CreateOrderResultModel{}
//			respErr := futureOrderResp.ReadData(orderResult)
//			if respErr != nil {
//				util.SocketInfo(fmt.Sprintf("function: %s fail to get create future order response kucoin, err:%s", "placeOrderKucoin", respErr))
//				order.Status = model.CarryStatusFail
//				order.OrderId = ``
//				return
//			}
//			order.OrderId = orderResult.OrderId
//			order.Symbol = symbol
//			order.OrderTime = time.Now()
//			order.Price, _ = strconv.ParseFloat(params["price"], 64)
//			order.OrderSide = orderSide
//			orderAmount, _ := strconv.ParseFloat(params["size"], 64)
//			_, order.Amount = model.ParseRealAmount(model.Kucoin, order.Symbol, orderAmount)
//			order.OrderType = orderType
//			order.Status = model.CarryStatusWorking
//			return
//		}
//	} else if success && marketType == model.MarketTypeSpot {
//		if model.AppConfig.KucoinSpot {
//			createOrder := &kucoin.CreateOrderModel{}
//			createOrder.ClientOid = "r" + strconv.FormatInt(time.Now().UnixNano(), 10)
//			createOrder.Symbol = symbol
//			createOrder.Side = orderSide
//			createOrder.Type = orderType
//			priceSpot, decimalSpot := model.FormatPrice(model.Kucoin, symbol, orderSide, price)
//			createOrder.Price = util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
//			createOrder.Size = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Kucoin, symbol, amount, price)))
//			util.SocketInfo(fmt.Sprintf(`create spot order request: %#v`, createOrder))
//			spotOrderResponse, err := kucoinRelatedClient("", "", "").CreateOrder(createOrder)
//			if err != nil {
//				util.SocketInfo(fmt.Sprintf("function: %s fail to create spot order kucoin, err:%s", "placeOrderKucoin", err))
//				order.Status = model.CarryStatusFail
//				order.OrderId = ``
//				return
//			} else {
//				util.SocketInfo(fmt.Sprintf(`create spot order response: %#v`, spotOrderResponse))
//				orderResult := &kucoin.CreateOrderResultModel{}
//				respErr := spotOrderResponse.ReadData(orderResult)
//				if respErr != nil {
//					util.SocketInfo(fmt.Sprintf("function: %s fail to get create spot order response kucoin, err:%s", "placeOrderKucoin", respErr))
//					order.Status = model.CarryStatusFail
//					order.OrderId = ``
//					return
//				}
//				order.OrderId = orderResult.OrderId
//				order.Symbol = symbol
//				order.OrderTime = time.Now()
//				order.Price, _ = strconv.ParseFloat(createOrder.Price, 64)
//				order.OrderSide = createOrder.Side
//				order.Amount, _ = strconv.ParseFloat(createOrder.Size, 64)
//				order.OrderType = createOrder.Type
//				order.Status = model.CarryStatusWorking
//				return
//			}
//		} else {
//			createOrder := &CreateMarginOrderModel{}
//			createOrder.ClientOid = "r" + strconv.FormatInt(time.Now().UnixNano(), 10)
//			createOrder.Symbol = symbol
//			createOrder.Side = orderSide
//			createOrder.Type = orderType
//			createOrder.MarginMode = "cross"
//			createOrder.AutoBorrow = true
//			priceSpot, decimalSpot := model.FormatPrice(model.Kucoin, symbol, orderSide, price)
//			createOrder.Price = util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
//			createOrder.Size = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Kucoin, symbol, amount, price)))
//			util.SocketInfo(fmt.Sprintf(`create margin order request: %#v`, createOrder))
//			req := kucoin.NewRequest(http.MethodPost, "/api/v1/margin/order", createOrder)
//			marginOrderResp, err := kucoinRelatedClient("", "", "").Call(req)
//			if err != nil {
//				util.SocketInfo(fmt.Sprintf("function: %s fail to create margin order kucoin, err:%s", "placeOrderKucoin", err))
//				order.Status = model.CarryStatusFail
//				order.OrderId = ``
//				return
//			} else {
//				util.SocketInfo(fmt.Sprintf(`create margin order response: %#v`, marginOrderResp))
//				orderResult := &kucoin.CreateOrderResultModel{}
//				respErr := marginOrderResp.ReadData(orderResult)
//				if respErr != nil {
//					util.SocketInfo(fmt.Sprintf("function: %s fail to get create margin order response kucoin, err:%s", "placeOrderKucoin", respErr))
//					order.Status = model.CarryStatusFail
//					order.OrderId = ``
//					return
//				}
//				order.OrderId = orderResult.OrderId
//				order.Symbol = symbol
//				order.OrderTime = time.Now()
//				order.Price, _ = strconv.ParseFloat(createOrder.Price, 64)
//				order.OrderSide = createOrder.Side
//				order.Amount, _ = strconv.ParseFloat(createOrder.Size, 64)
//				order.OrderType = createOrder.Type
//				order.Status = model.CarryStatusWorking
//				return
//			}
//		}
//	}
//}

//func transferKucoin(transferType string, amount float64) {
//	orderId := "t" + strconv.FormatInt(time.Now().UnixNano(), 10)
//	amountStr := util.CutTailZero(strconv.FormatFloat(amount, 'f', 8, 64))
//	var from, to string
//	if transferType == "MAIN_UMFUTURE" {
//		to = "contract"
//		if model.AppConfig.KucoinSpot {
//			from = "trade"
//		} else {
//			from = "margin"
//		}
//		apiResponse, err := kucoinRelatedClient("", "", "").InnerTransferV2(orderId, `USDT`, from, to, amountStr)
//		if err != nil {
//			util.SocketInfo(fmt.Sprintf("function: %s transfer related to future kucoin, err:%s", "transferKucoin", err))
//		}
//		transferResult := &kucoin.InnerTransferResultModel{}
//		if respErr := apiResponse.ReadData(transferResult); respErr != nil {
//			util.SocketInfo(fmt.Sprintf("function: %s fail to get transfer related to future response kucoin, err:%s", "transferKucoin", respErr))
//		}
//	} else {
//		from = "contract"
//		if model.AppConfig.KucoinSpot {
//			to = "trade"
//		} else {
//			to = "margin"
//		}
//		apiResponse, err := kucoinRelatedClient("", "", "").InnerTransferV2(orderId, `USDT`, from, to, amountStr)
//		if err != nil {
//			util.SocketInfo(fmt.Sprintf("function: %s transfer future to related kucoin, err:%s", "transferKucoin", err))
//		}
//		transferResult := &kucoin.InnerTransferResultModel{}
//		if respErr := apiResponse.ReadData(transferResult); respErr != nil {
//			util.SocketInfo(fmt.Sprintf("function: %s fail to get transfer future to related response kucoin, err:%s", "transferKucoin", respErr))
//		}
//	}
//}

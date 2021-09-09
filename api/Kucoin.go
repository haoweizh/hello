package api

import (
	"fmt"
	"github.com/Kucoin/kucoin-go-sdk"
	"github.com/Kucoin/kumex-go-sdk"
	"hello/model"
	"hello/util"
	"strconv"
	"strings"
	"time"
)

var relatedSettingMarkets = make(map[string]bool)

func kucoinFutureClient(key, secret, passPhrase string) *kumex.ApiService {
	if key == "" || secret == "" || passPhrase == "" {
		key = model.AppConfig.KucoinFutureKey
		secret = model.AppConfig.KucoinFutureSecret
		passPhrase = model.AppConfig.Phase
	}
	client := kumex.NewApiService(
		kumex.ApiKeyOption(key),
		kumex.ApiSecretOption(secret),
		kumex.ApiPassPhraseOption(passPhrase),
		kumex.ApiKeyVersionOption("2"))

	return client
}

func kucoinRelatedClient(key, secret, passPhrase string) *kucoin.ApiService {
	if key == "" || secret == "" || passPhrase == "" {
		key = model.AppConfig.KucoinRelatedKey
		secret = model.AppConfig.KucoinRelatedSecret
		passPhrase = model.AppConfig.Phase
	}
	client := kucoin.NewApiService(
		kucoin.ApiKeyOption(key),
		kucoin.ApiSecretOption(secret),
		kucoin.ApiPassPhraseOption(passPhrase),
		kucoin.ApiKeyVersionOption("2"))
	return client
}

func getMarketsKucoin(key, secret string) (success bool, marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	appendRelatedMarketsKucoin(key, secret, marketInfos)
	appendFutureMarketKucoin(key, secret, marketInfos)
	return true, marketInfos
}

func appendRelatedMarketsKucoin(key, secret string, marketInfos map[string]*model.MarketInfo) {
	client := kucoinRelatedClient("", "", "")
	resp, err := client.Symbols("")
	if err != nil {
		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API error", key, "appendRelatedMarketsKucoin"))
		return
	}
	symbols := kucoin.SymbolsModel{}
	if err := resp.ReadData(&symbols); err != nil {
		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API read data error", key, "appendRelatedMarketsKucoin"))
		return
	}
	for _, related := range symbols {
		if !related.EnableTrading || related.QuoteCurrency != "USDT" {
			continue
		}
		if !related.IsMarginEnabled {
			continue
		}
		marketInfo := &model.MarketInfo{}
		marketInfo.Name = related.Symbol
		marketInfo.PriceIncrement, _ = strconv.ParseFloat(related.PriceIncrement, 64)
		marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
		marketInfo.SizeMin, _ = strconv.ParseFloat(related.BaseMinSize, 64)
		marketInfo.SizeMax, _ = strconv.ParseFloat(related.BaseMaxSize, 64)
		marketInfo.SizeIncrement, _ = strconv.ParseFloat(related.BaseIncrement, 64)
		marketInfos[marketInfo.Name] = marketInfo
	}
	relatedSettingMarkets = model.GetMarketSymbols(model.Kucoin)
}

func appendFutureMarketKucoin(key, secret string, marketInfos map[string]*model.MarketInfo) {
	client := kucoinFutureClient("", "", "")
	resp, err := client.ActiveContracts()
	if err != nil {
		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API error", key, "appendFutureMarketKucoin"))
		return
	}
	contracts := kumex.ContractsModels{}
	if err := resp.ReadData(&contracts); err != nil {
		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API read data error", key, "appendFutureMarketKucoin"))
		return
	}
	for _, contract := range contracts {
		if contract.Status != "Open" || contract.QuoteCurrency != "USDT" {
			continue
		}
		marketInfo := &model.MarketInfo{}
		marketInfo.Name = contract.BaseCurrency + model.GetPerpTail(model.Kucoin)
		marketInfo.PriceIncrement = float64(contract.TickSize)
		marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
		marketInfo.SizeMin = float64(contract.LotSize)
		marketInfo.SizeIncrement = marketInfo.SizeMin
		marketInfo.CTCurrency = contract.BaseCurrency
		marketInfo.SizeMax = float64(contract.MaxOrderQty)
		marketInfo.PriceMax = float64(contract.MaxPrice)
		marketInfo.CTValue = float64(contract.Multiplier)
		marketInfos[marketInfo.Name] = marketInfo
	}
}

func WsDepthServeKucoin() (err error) {
	relatedClient := kucoinRelatedClient("", "", "")
	futureClient := kucoinFutureClient("", "", "")
	relatedRsp, relatedErr := relatedClient.WebSocketPublicToken()
	futureRsp, futureErr := futureClient.WebSocketPublicToken()
	if relatedErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket error:%s", "WsDepthServeKucoin", relatedErr))
		return relatedErr
	}
	if futureErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket error:%s", "WsDepthServeKucoin", futureErr))
		return futureErr
	}
	relatedToken := &kucoin.WebSocketTokenModel{}
	if relatedTokenErr := relatedRsp.ReadData(relatedToken); relatedTokenErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket error:%s", "WsDepthServeKucoin", relatedTokenErr))
		return relatedTokenErr
	}
	futureToken := &kumex.WebSocketTokenModel{}
	if futureTokenErr := futureRsp.ReadData(futureToken); futureTokenErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket error:%s", "WsDepthServeKucoin", futureTokenErr))
		return futureTokenErr
	}
	relatedChannel := relatedClient.NewWebSocketClient(relatedToken)
	relatedMsg, relatedChannelError, relatedConnectErr := relatedChannel.Connect()
	if relatedConnectErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket connect error:%s", "WsDepthServeKucoin", relatedConnectErr))
		retrySuccess := false
		for i := 0; i < 10; i++ {
			i++
			relatedChannel = relatedClient.NewWebSocketClient(relatedToken)
			relatedMsg, relatedChannelError, relatedConnectErr = relatedChannel.Connect()
			if relatedConnectErr != nil {
				util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket connect retry：%d error:%s", "WsDepthServeKucoin", i, relatedConnectErr))
				time.Sleep(time.Second * 2)
				continue
			} else {
				retrySuccess = true
				break
			}
		}
		if !retrySuccess {
			return relatedConnectErr
		}
	}
	time.Sleep(time.Second * 2)
	futureChannel := futureClient.NewWebSocketClient(futureToken)
	futureMsg, futureChannelError, futureConnectErr := futureChannel.Connect()
	if futureConnectErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket connect error:%s", "WsDepthServeKucoin", futureConnectErr))
		retrySuccess := false
		for i := 0; i < 10; i++ {
			i++
			futureChannel = futureClient.NewWebSocketClient(futureToken)
			futureMsg, futureChannelError, futureConnectErr = futureChannel.Connect()
			if futureConnectErr != nil {
				util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket connect retry：%d error:%s", "WsDepthServeKucoin", i, futureConnectErr))
				time.Sleep(time.Second * 2)
				continue
			} else {
				retrySuccess = true
				break
			}
		}
		if !retrySuccess {
			return futureConnectErr
		}
	}
	symbols := model.GetMarketSymbols(model.Kucoin)
	futureSubscribes := make([]*kumex.WebSocketSubscribeMessage, 0)
	for symbol := range symbols {
		if strings.Contains(symbol, model.GetPerpTail(model.Kucoin)) {
			topic := "/contractMarket/tickerV2:" + strings.Split(symbol, "-")[0] + "USDTM"
			futureSubscribes = append(futureSubscribes, kumex.NewSubscribeMessage(topic, false))
		}
	}
	relatedSubscribe := kucoin.NewSubscribeMessage("/market/ticker:all", false)
	if relatedSubscribeErr := relatedChannel.Subscribe(relatedSubscribe); relatedSubscribeErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket subscribe error:%s", "WsDepthServeKucoin", relatedSubscribeErr))
		return relatedSubscribeErr
	}
	if futureSubscribeErr := futureChannel.Subscribe(futureSubscribes...); futureSubscribeErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket subscribe error:%s", "WsDepthServeKucoin", futureSubscribeErr))
		return futureSubscribeErr
	}

	for {
		select {
		case cError := <-relatedChannelError:
			//channel.Stop()
			util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket channel error:%s", "WsDepthServeKucoin", cError.Error()))
			return cError
		case cError := <-futureChannelError:
			//channel.Stop()
			util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket channel error:%s", "WsDepthServeKucoin", cError.Error()))
			return cError
		case msg := <-relatedMsg:
			handleKucoinWS(msg, nil)
		case msg := <-futureMsg:
			handleKucoinWS(nil, msg)
		}
	}
}

func handleKucoinWS(relatedMsg *kucoin.WebSocketDownstreamMessage, futureMsg *kumex.WebSocketDownstreamMessage) {
	if futureMsg != nil && strings.Contains(futureMsg.Topic, "/contractMarket/tickerV2") {
		ticker := &kumex.TickerLevel1Model{}
		if err := futureMsg.ReadData(ticker); err != nil {
			util.Notice(fmt.Sprintf("future ticker Unmarshal err:%s", err.Error()))
		}
		if ticker.Symbol == "" {
			return
		}
		symbol := strings.ReplaceAll(ticker.Symbol, "USDTM", "") + model.GetPerpTail(model.Kucoin)
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		ts := int(ticker.Ts / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(ticker.BestBidPrice, 64)
		_, bidAmount := model.ParseRealAmount(model.Kucoin, symbol, float64(ticker.BestBidSize))
		askPrice, _ := strconv.ParseFloat(ticker.BestAskPrice, 64)
		_, askAmount := model.ParseRealAmount(model.Kucoin, symbol, float64(ticker.BestAskSize))
		bidAsk := model.BidAsk{Ts: ts, TsReceived: now,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount}}}
		markets := model.AppMarkets
		haveOld, old := markets.GetBidAsk(symbol, model.Kucoin)
		if haveOld && old.Ts > bidAsk.Ts {
			return
		}
		if markets.SetBidAsk(symbol, model.Kucoin, &bidAsk) {
			for function, handler := range model.GetFunctions(model.Kucoin, symbol) {
				if handler != nil {
					settings := model.GetSetting(function, model.Kucoin, symbol)
					for _, setting := range settings {
						go handler(setting, &bidAsk)
					}
				}
			}
		}
	} else {
		ticker := &kucoin.TickerLevel1Model{}
		if err := relatedMsg.ReadData(ticker); err != nil {
			util.Notice(fmt.Sprintf("jsonerr Unmarshal err:%s", err.Error()))
		}
		if relatedMsg.Subject == "" {
			return
		}
		if relatedSettingMarkets[relatedMsg.Subject] == false {
			return
		}
		symbol := relatedMsg.Subject
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		//util.Notice(fmt.Sprintf("币种：%s，当前ts：%d", symbol, now))
		updateId, _ := strconv.ParseInt(ticker.Sequence, 10, 64)
		bidPrice, _ := strconv.ParseFloat(ticker.BestBid, 64)
		bidAmount, _ := strconv.ParseFloat(ticker.BestBidSize, 64)
		askPrice, _ := strconv.ParseFloat(ticker.BestAsk, 64)
		askAmount, _ := strconv.ParseFloat(ticker.BestAskSize, 64)
		bidAsk := model.BidAsk{Ts: now, TsReceived: now, UpdateId: updateId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount}}}
		markets := model.AppMarkets
		haveOld, old := markets.GetBidAsk(symbol, model.Kucoin)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
			return
		}
		if markets.SetBidAsk(symbol, model.Kucoin, &bidAsk) {
			for function, handler := range model.GetFunctions(model.Kucoin, symbol) {
				if handler != nil {
					settings := model.GetSetting(function, model.Kucoin, symbol)
					for _, setting := range settings {
						go handler(setting, &bidAsk)
					}
				}
			}
		}
	}
}

func getBalanceKucoin(key string, secret string) (success bool, balances []*model.Balance) {
	accountResp, err := kucoinRelatedClient("", "", "").MarginAccount()
	if err != nil {
		util.SocketInfo(fmt.Sprintf("fail to refresh margin balance kucoin, err:%s", err))
		time.Sleep(time.Second * 2)
		return getBalanceKucoin(key, secret)
	}
	marginAccount := &kucoin.MarginAccountModel{}
	respError := accountResp.ReadData(marginAccount)
	if respError != nil {
		util.SocketInfo(fmt.Sprintf("fail to get margin balance response kucoin, err:%s", respError))
		return false, balances
	}
	balances = make([]*model.Balance, 0)
	for _, account := range marginAccount.Accounts {
		balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Kucoin, Coin: account.Currency}
		balance.Amount, _ = account.TotalBalance.Float64()
		balance.FrozenAmount, _ = account.HoldBalance.Float64()
		available, _ := account.AvailableBalance.Float64()
		balance.Borrow, _ = account.MaxBorrowSize.Float64()
		balance.AvailableWithBorrow = available + balance.Borrow
		priceGet, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.GetSpotTail(model.Kucoin), model.Kucoin)
		if priceGet {
			balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
		}
		balances = append(balances, balance)
	}
	return true, balances
}

type PositionsModel []*kumex.PositionModel

func getPositionsKucoin(key string, secret string) (success bool, positions []*model.Position, posBalance float64) {
	params := make(map[string]string)
	params["currency"] = "USDT"
	accountResp, accountErr := kucoinFutureClient("", "", "").AccountOverview(params)
	contractResp, err := kucoinFutureClient("", "", "").Positions()
	if err != nil || accountErr != nil {
		if accountErr != nil {
			util.SocketInfo(fmt.Sprintf("fail to refresh future account kucoin, err:%s", err))
		}
		if err != nil {
			util.SocketInfo(fmt.Sprintf("fail to refresh future position kucoin, err:%s", err))
		}
		time.Sleep(time.Second * 2)
		return getPositionsKucoin(key, secret)
	}
	account := &kumex.AccountModel{}
	accountRespError := accountResp.ReadData(account)
	if accountRespError != nil {
		util.SocketInfo(fmt.Sprintf("fail to get future account response kucoin, err:%s", accountRespError))
		return false, positions, 0
	}
	posBalance = account.AvailableBalance
	contracts := &PositionsModel{}
	contractRespError := contractResp.ReadData(contracts)
	if contractRespError != nil {
		util.SocketInfo(fmt.Sprintf("fail to get future position response kucoin, err:%s", contractRespError))
		return false, positions, 0
	}
	positions = make([]*model.Position, 0)
	for _, contract := range *contracts {
		currency := strings.ReplaceAll(contract.Symbol, "USDTM", "") + model.GetPerpTail(model.Kucoin)
		position := &model.Position{Market: model.Kucoin, Ts: util.GetNowUnixMillion(), Currency: currency}
		currentQty, _ := strconv.ParseFloat(contract.CurrentQty, 64)
		_, realAmount := model.ParseRealAmount(model.Kucoin, currency, currentQty)
		position.Free = realAmount
		position.LeverRate, _ = strconv.ParseInt(contract.RealLeverage, 10, 64)
		position.EntryPrice, _ = strconv.ParseFloat(contract.AvgEntryPrice, 64)
		position.Margin, _ = strconv.ParseFloat(contract.PosMargin, 64)
		position.LiquidationPrice, _ = strconv.ParseFloat(contract.LiquidationPrice, 64)
		positions = append(positions, position)
	}
	return true, positions, posBalance
}

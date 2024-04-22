package api

import (
	"fmt"
	kumex "github.com/Kucoin/kucoin-futures-go-sdk"
	"hello/model"
	"hello/util"
	"strconv"
	"strings"
	"time"
)

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

func getMarketsKucoinPerp(key string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	appendFutureMarketKucoin(key, marketInfos)
	return marketInfos
}

type KucoinContractModel struct {
	BaseCurrency       string  `json:"baseCurrency"`
	FairMethod         string  `json:"fairMethod"`
	FundingBaseSymbol  string  `json:"fundingBaseSymbol"`
	FundingQuoteSymbol string  `json:"fundingQuoteSymbol"`
	FundingRateSymbol  string  `json:"fundingRateSymbol"`
	IndexSymbol        string  `json:"indexSymbol"`
	InitialMargin      float64 `json:"initialMargin"`
	IsDeleverage       bool    `json:"isDeleverage"`
	IsInverse          bool    `json:"isInverse"`
	IsQuanto           bool    `json:"isQuanto"`
	LotSize            float64 `json:"lotSize"`
	MaintainMargin     float64 `json:"maintainMargin"`
	MakerFeeRate       float64 `json:"makerFeeRate"`
	MakerFixFee        float64 `json:"makerFixFee"`
	MarkMethod         string  `json:"markMethod"`
	MaxOrderQty        float64 `json:"maxOrderQty"`
	MaxPrice           float64 `json:"maxPrice"`
	MaxRiskLimit       float64 `json:"maxRiskLimit"`
	MinRiskLimit       float64 `json:"minRiskLimit"`
	Multiplier         float64 `json:"multiplier"`
	QuoteCurrency      string  `json:"quoteCurrency"`
	RiskStep           int     `json:"riskStep"`
	RootSymbol         string  `json:"rootSymbol"`
	Status             string  `json:"status"`
	Symbol             string  `json:"symbol"`
	TakerFeeRate       float64 `json:"takerFeeRate"`
	TakerFixFee        float64 `json:"takerFixFee"`
	TickSize           float64 `json:"tickSize"`
	Type               string  `json:"type"`
	MaxLeverage        float64 `json:"maxLeverage"`
	VolumeOf24h        float64 `json:"volumeOf24h"`
	TurnoverOf24h      float64 `json:"turnoverOf24h"`
	OpenInterest       string  `json:"openInterest"`
}
type KucoinContractsModels []*KucoinContractModel

func appendFutureMarketKucoin(key string, marketInfos map[string]*model.MarketInfo) {
	client := kucoinFutureClient("", "", "")
	resp, err := client.ActiveContracts()
	if err != nil || resp.Code != "200000" {
		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API error, response:%v", key, "appendFutureMarketKucoin", resp))
		return
	}
	contracts := KucoinContractsModels{}
	if err := resp.ReadData(&contracts); err != nil {
		util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API read data error", key, "appendFutureMarketKucoin"))
		return
	}
	for _, contract := range contracts {
		if contract.Status != "Open" || contract.QuoteCurrency != `USDT` {
			continue
		}

		if contract.MaxLeverage < model.DefaultLeverage {
			util.Info(fmt.Sprintf(contract.BaseCurrency+model.UniStandardTail[model.MarketTypePerp]+"杠杆倍数：%d", contract.MaxLeverage))
			continue
		}

		marketInfo := &model.MarketInfo{Market: model.KucoinPerp}
		marketInfo.Name = contract.BaseCurrency + model.UniStandardTail[model.MarketTypePerp]
		marketInfo.PriceIncrement = contract.TickSize
		marketInfo.PriceDecimal = util.NumDecPlaces(contract.TickSize)
		marketInfo.SizeMin = contract.LotSize
		marketInfo.SizeIncrement = marketInfo.SizeMin
		marketInfo.CTCurrency = contract.BaseCurrency
		marketInfo.SizeMax = contract.MaxOrderQty
		//marketInfo.PriceMax = contract.MaxPrice
		marketInfo.CTValue = contract.Multiplier
		marketInfos[marketInfo.Name] = marketInfo
	}
}

var settingKucoinPerp = false

func setFutureAutoDeposit() {
	if settingKucoinPerp {
		return
	}
	defer func() {
		settingKucoinPerp = false
	}()
	settingKucoinPerp = true
	settings := GetSettings(model.FunctionCross, model.KucoinPerp)
	if settings == nil {
		return
	}
	settings.Range(func(key, value any) bool {
		if value != nil {
			setting := value.(*model.Setting)
			params := make(map[string]string)
			params["symbol"] = setting.Coin + model.DialectTail[model.MarketTypePerp][model.KucoinPerp]
			params["status"] = "true"
			resp, err := kucoinFutureClient("", "", "").AutoDepositStatus(params)
			if err != nil || !resp.HttpSuccessful() || !resp.ApiSuccessful() {
				util.SocketInfo(fmt.Sprintf("function: %s symbol: %s kucoin API error", "setFutureAutoDeposit", params["symbol"]))
			}
		}
		return true
	})
}

func WsDepthServeKucoinPerp() (channels []chan struct{}, err error) {
	symbols := GetMarketSymbols(model.KucoinPerp)
	futureSubscribes := make([]*kumex.WebSocketSubscribeMessage, 0)
	for symbol := range symbols {
		if strings.LastIndex(symbol, model.UniStandardTail[model.MarketTypePerp]) == len(symbol)-len(model.UniStandardTail[model.MarketTypePerp]) &&
			len(symbol)-len(model.UniStandardTail[model.MarketTypePerp]) > 0 {
			success, _, _, dialectSymbol := model.GetFromStandard(model.KucoinPerp, symbol)
			if success {
				topic := "/contractMarket/tickerV2:" + dialectSymbol
				futureSubscribes = append(futureSubscribes, kumex.NewSubscribeMessage(topic, false))
			}
		}
		//if strings.Contains(symbol, model.GetPerpTail(model.KucoinPerp)) {
		//	topic := "/contractMarket/tickerV2:" + strings.Split(symbol, "-")[0] + "USDTM"
		//	futureSubscribes = append(futureSubscribes, kumex.NewSubscribeMessage(topic, false))
		//}
	}
	step := 50
	for i := 0; i < len(futureSubscribes); i += step {
		curFutureSubscribes := make([]*kumex.WebSocketSubscribeMessage, 0)
		for j := i; j < len(futureSubscribes) && j < i+step; j++ {
			curFutureSubscribes = append(curFutureSubscribes, futureSubscribes[j])
		}
		client, token, channel, futureErr := getKucoinFutureWsClient()
		if futureErr != nil {
			util.SocketInfo(fmt.Sprintf("function: %s error: %s step:%d", "getKucoinFutureWsClient", futureErr.Error()), step)
			continue
		}
		futureChannelError, futureMsg := kucoinFutureChannelConnect(client, token, channel)
		if futureSubscribeErr := channel.Subscribe(curFutureSubscribes...); futureSubscribeErr != nil {
			util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket subscribe error:%s step:%d ", "WsDepthServeKucoin", futureSubscribeErr, i))
			return channels, futureSubscribeErr
		}
		futureStopC := make(chan struct{}, 10)
		go handlerKucoinFutureWS(futureChannelError, futureMsg, channel, futureStopC)
		channels = append(channels, futureStopC)
		time.Sleep(1 * time.Second)
	}
	return channels, err
}

func getKucoinFutureWsClient() (futureClient *kumex.ApiService, futureToken *kumex.WebSocketTokenModel, futureChannel *kumex.WebSocketClient, err error) {
	futureClient = kucoinFutureClient("", "", "")
	futureRsp, futureErr := futureClient.WebSocketPublicToken()
	if futureErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket error:%s", "WsDepthServeKucoin", futureErr))
		return nil, nil, nil, futureErr
	}
	futureToken = &kumex.WebSocketTokenModel{}
	if futureTokenErr := futureRsp.ReadData(futureToken); futureTokenErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket error:%s", "WsDepthServeKucoin", futureTokenErr))
		return nil, nil, nil, futureTokenErr
	}
	futureChannel = futureClient.NewWebSocketClient(futureToken)
	return futureClient, futureToken, futureChannel, nil
}

func kucoinFutureChannelConnect(futureClient *kumex.ApiService, futureToken *kumex.WebSocketTokenModel, futureChannel *kumex.WebSocketClient) (<-chan error, <-chan *kumex.WebSocketDownstreamMessage) {
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
				time.Sleep(time.Minute * 5)
				continue
			} else {
				retrySuccess = true
				util.SocketInfo(fmt.Sprintf("kucoin future websocket connect retry success"))
				break
			}
		}
		if !retrySuccess {
			return futureChannelError, futureMsg
		}
	}
	return futureChannelError, futureMsg
}

func handlerKucoinFutureWS(futureChannelError <-chan error, futureMsg <-chan *kumex.WebSocketDownstreamMessage, channel *kumex.WebSocketClient, stopC chan struct{}) {
	defer func() {
		channel.Stop()
	}()
	for {
		select {
		case <-stopC:
			util.Notice("get stop perp struct, return")
			return
		case cError := <-futureChannelError:
			util.SocketInfo(fmt.Sprintf("function: %s kucoin future websocket channel error:%s", "WsDepthServeKucoin", cError.Error()))
			return
		case msg := <-futureMsg:
			handleKucoinPerpWS(msg)
		}
	}
}

func handleKucoinPerpWS(futureMsg *kumex.WebSocketDownstreamMessage) {
	if futureMsg != nil && strings.Contains(futureMsg.Topic, "/contractMarket/tickerV2") {
		ticker := &kumex.TickerLevel1Model{}
		if err := futureMsg.ReadData(ticker); err != nil {
			util.Notice(fmt.Sprintf("future ticker Unmarshal err:%s", err.Error()))
		}
		if ticker.Symbol == "" {
			return
		}
		symbol := strings.ReplaceAll(ticker.Symbol, `USDTM`, ``) + model.UniStandardTail[model.MarketTypePerp]
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		ts := int(ticker.Ts / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(ticker.BestBidPrice, 64)
		_, bidAmount := model.ParseRealAmount(model.KucoinPerp, symbol, float64(ticker.BestBidSize))
		askPrice, _ := strconv.ParseFloat(ticker.BestAskPrice, 64)
		_, askAmount := model.ParseRealAmount(model.KucoinPerp, symbol, float64(ticker.BestAskSize))
		bidAsk := model.BidAsk{Ts: ts, TsReceived: now,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.KucoinPerp, Symbol: symbol}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.KucoinPerp, Symbol: symbol}}}
		markets := model.AppMarkets
		haveOld, old := markets.GetBidAsk(symbol, model.KucoinPerp)
		if haveOld && old.Ts > bidAsk.Ts {
			return
		}
		if markets.SetBidAsk(symbol, model.KucoinPerp, &bidAsk) {

			funcHandlers := GetFunctions(model.KucoinPerp, symbol)
			if funcHandlers != nil {
				funcHandlers.Range(func(function, value interface{}) bool {
					setting := GetSetting(function.(string), model.KucoinPerp, symbol)
					if setting != nil && value != nil && value.(model.CarryHandler) != nil {
						go value.(model.CarryHandler)(setting, &bidAsk)
					}
					return true
				})
			}
		}
	}
}

type KucoinPositionModel struct {
	Id                string  `json:"id"`
	Symbol            string  `json:"symbol"`
	AutoDeposit       bool    `json:"autoDeposit"`
	MaintMarginReq    float64 `json:"maintMarginReq"`
	RiskLimit         int     `json:"riskLimit"`
	RealLeverage      float64 `json:"realLeverage"`
	CrossMode         bool    `json:"crossMode"`
	DelevPercentage   float64 `json:"delevPercentage"`
	OpeningTimestamp  int64   `json:"openingTimestamp"`
	CurrentTimestamp  int64   `json:"currentTimestamp"`
	CurrentQty        int64   `json:"currentQty"`
	CurrentCost       float64 `json:"currentCost"`
	CurrentComm       float64 `json:"currentComm"`
	UnrealisedCost    float64 `json:"unrealisedCost"`
	RealisedGrossCost float64 `json:"realisedGrossCost"`
	RealisedCost      float64 `json:"realisedCost"`
	IsOpen            bool    `json:"isOpen"`
	MarkPrice         float64 `json:"markPrice"`
	MarkValue         float64 `json:"markValue"`
	PosCost           float64 `json:"posCost"`
	PosCross          float64 `json:"posCross"`
	PosInit           float64 `json:"posInit"`
	PosComm           float64 `json:"posComm"`
	PosLoss           float64 `json:"posLoss"`
	PosMargin         float64 `json:"posMargin"`
	PosMaint          float64 `json:"posMaint"`
	MaintMargin       float64 `json:"maintMargin"`
	RealisedGrossPnl  float64 `json:"realisedGrossPnl"`
	RealisedPnl       float64 `json:"realisedPnl"`
	UnrealisedPnl     float64 `json:"unrealisedPnl"`
	UnrealisedPnlPcnt float64 `json:"unrealisedPnlPcnt"`
	UnrealisedRoePcnt float64 `json:"unrealisedRoePcnt"`
	AvgEntryPrice     float64 `json:"avgEntryPrice"`
	LiquidationPrice  float64 `json:"liquidationPrice"`
	BankruptPrice     float64 `json:"bankruptPrice"`
	SettleCurrency    string  `json:"settleCurrency"`
}

type PositionsModel []*KucoinPositionModel

func getPositionsKucoinPerp(key string, secret string) (success bool, positions []*model.Position, accountValue, availableU float64) {
	//params := make(map[string]string)
	//params["currency"] = `USDT`
	//accountResp, accountErr := kucoinFutureClient("", "", "").AccountOverview(params)
	//contractResp, err := kucoinFutureClient(``, "", "").Positions()
	//if err != nil || accountErr != nil || accountResp.Code != "200000" || contractResp.Code != "200000" {
	//	if accountErr != nil {
	//		util.SocketInfo(fmt.Sprintf("fail to refresh future account kucoin, err:%s, response:%v", err, accountResp))
	//	}
	//	if err != nil {
	//		util.SocketInfo(fmt.Sprintf("fail to refresh future position kucoin, err:%s, response:%v", err, contractResp))
	//	}
	//	time.Sleep(time.Minute * 5)
	//	return getPositionsKucoinPerp(key, secret)
	//}
	//account := &kumex.AccountModel{}
	//accountRespError := accountResp.ReadData(account)
	//if accountRespError != nil {
	//	util.SocketInfo(fmt.Sprintf("fail to get future account response kucoin, err:%s", accountRespError))
	//	return false, positions, 0, 0
	//}
	//accountRespJson, _ := json.Marshal(account)
	//util.SocketInfo(fmt.Sprintf(`get future account response: %s`, accountRespJson))
	//accountValue = account.AccountEquity
	//availableU = account.AvailableBalance
	//contracts := &PositionsModel{}
	//contractRespError := contractResp.ReadData(contracts)
	//if contractRespError != nil {
	//	util.SocketInfo(fmt.Sprintf("fail to get future position response kucoin, err:%s", contractRespError))
	//	return false, positions, 0, 0
	//}
	//contractRespJson, _ := json.Marshal(contracts)
	//util.SocketInfo(fmt.Sprintf(`get future position response: %s`, contractRespJson))
	//positions = make([]*model.Position, 0)
	//for _, contract := range *contracts {
	//	currency := strings.ReplaceAll(contract.Symbol, `USDTM`, ``) + model.UniStandardTail[model.MarketTypePerp]
	//	position := &model.Position{Market: model.KucoinPerp, Ts: util.GetNowUnixMillion(), Currency: currency}
	//	_, realAmount := model.ParseRealAmount(model.KucoinPerp, currency, float64(contract.CurrentQty))
	//	position.Holding = realAmount
	//	position.LeverRate = int64(contract.RealLeverage)
	//	position.EntryPrice = contract.AvgEntryPrice
	//	position.Margin = contract.PosMargin
	//	position.LiquidationPrice = contract.LiquidationPrice
	//	position.ProfitUnreal = contract.UnrealisedPnl
	//	if position.Holding != 0 {
	//		positions = append(positions, position)
	//	}
	//}
	return true, positions, accountValue, availableU
}

func cancelOrdersKucoinPerp(symbol string) (result bool) {
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.KucoinPerp, symbol)
	if success && marketType == model.MarketTypePerp {
		apiResponse, err := kucoinFutureClient("", "", "").CancelOrders(dialectSymbol)
		if err != nil || apiResponse.Code != "200000" {
			util.SocketInfo(fmt.Sprintf("function: %s fail to cancel future orders kucoin, err:%s, response:%v", "cancelOrdersKucoin", err, apiResponse))
			return false
		}
		orders := &kumex.CancelOrderResultModel{}
		if cancelErr := apiResponse.ReadData(orders); cancelErr != nil {
			util.SocketInfo(fmt.Sprintf("fail to get cancel future orders response kucoin, err:%s", cancelErr))
			return false
		}
	}
	return true
}

func placeOrderKucoinPerp(order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	success, marketType, _, dialectSymbol := model.GetFromStandard(model.KucoinPerp, symbol)
	if success && marketType == model.MarketTypePerp {
		params := make(map[string]string)
		params["clientOid"] = "f" + strconv.FormatInt(time.Now().UnixNano(), 10)
		params["side"] = orderSide
		params["symbol"] = dialectSymbol
		params["type"] = orderType
		params["leverage"] = strconv.Itoa(model.DefaultLeverage)
		priceFuture, decimalFuture := model.FormatPrice(model.KucoinPerp, symbol, price)
		order.Price = priceFuture
		params["price"] = util.CutTailZero(strconv.FormatFloat(priceFuture, 'f', decimalFuture, 64))
		params["size"] = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.KucoinPerp, symbol, amount, price, false)))
		//util.SocketInfo(fmt.Sprintf(`create future order request: %s`, params))
		futureOrderResp, err := kucoinFutureClient("", "", "").CreateOrder(params)
		if err != nil || futureOrderResp.Code != "200000" {
			util.SocketInfo(fmt.Sprintf("function: %s fail to create future order kucoin, err:%s, response:%v", "placeOrderKucoin", err, futureOrderResp))
			order.Status = model.CarryStatusFail
			order.OrderId = ``
			return
		} else {
			//util.SocketInfo(fmt.Sprintf(`create future order response: %v`, futureOrderResp))
			orderResult := &kumex.CreateOrderResultModel{}
			respErr := futureOrderResp.ReadData(orderResult)
			if respErr != nil {
				util.SocketInfo(fmt.Sprintf("function: %s fail to get create future order response kucoin, err:%s", "placeOrderKucoin", respErr))
				order.Status = model.CarryStatusFail
				order.OrderId = ``
				return
			}
			order.OrderId = orderResult.OrderId
			order.Symbol = symbol
			order.OrderTime = time.Now()
			order.Price, _ = strconv.ParseFloat(params["price"], 64)
			order.OrderSide = orderSide
			orderAmount, _ := strconv.ParseFloat(params["size"], 64)
			_, order.Amount = model.ParseRealAmount(model.KucoinPerp, order.Symbol, orderAmount)
			order.OrderType = orderType
			order.Status = model.CarryStatusWorking
			return
		}
	}
}

func queryOrderKucoinPerp(symbol string, orderId string) (order *model.Order) {
	orderResponse, respErr := kucoinFutureClient("", "", "").Order(orderId)
	if respErr != nil || orderResponse.Code != "200000" {
		util.SocketInfo(fmt.Sprintf("function: %s fail to query kucoin perp order , err:%s, response:%v", "queryOrderKucoinPerp", respErr, orderResponse))
		return
	}
	orderResult := &kumex.OrderModel{}
	readErr := orderResponse.ReadData(orderResult)
	if readErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s fail to parse query kucoin perp order response , err:%s", "queryOrderKucoinSpot", readErr))
		return
	}
	order = &model.Order{Market: model.KucoinPerp, Status: model.CarryStatusFail}
	if orderResult != nil {
		order.OrderId = orderId
		order.Symbol = symbol
		order.OrderSide = strings.ToLower(orderResult.Side)
		order.OrderType = strings.ToLower(orderResult.Type)
		order.Price, _ = strconv.ParseFloat(orderResult.Price, 64)
		_, order.Amount = model.ParseRealAmount(model.KucoinPerp, order.Symbol, float64(orderResult.Size))
		_, order.DealAmount = model.ParseRealAmount(model.KucoinPerp, order.Symbol, float64(orderResult.DealSize))
		order.OrderTime = time.Unix(orderResult.CreatedAt, 0)
		if orderResult.IsActive {
			order.Status = model.CarryStatusWorking
		} else {
			if order.DealAmount > 0 {
				order.Status = model.CarryStatusSuccess
			} else {
				order.Status = model.CarryStatusFail
			}
		}
		if order.DealAmount > 0 && order.DealPrice == 0 {
			order.DealPrice = order.Price
		}
	}
	return
}

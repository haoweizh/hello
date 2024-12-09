package deprecated

//
//import (
//	"encoding/json"
//	"fmt"
//	"github.com/Kucoin/kucoin-go-sdk"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"net/http"
//	"strconv"
//	"strings"
//	"time"
//)
//
//var relatedSettingMarkets = make(map[string]bool)
//
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
//
//func getMarketsKucoinSpot(key string) (marketInfos map[string]*model.MarketInfo) {
//	marketInfos = make(map[string]*model.MarketInfo)
//	appendRelatedMarketsKucoin(key, marketInfos)
//	return marketInfos
//}
//
//type KucoinSymbolModel struct {
//	Symbol          string `json:"symbol"`
//	Name            string `json:"name"`
//	BaseCurrency    string `json:"baseCurrency"`
//	QuoteCurrency   string `json:"quoteCurrency"`
//	Market          string `json:"market"`
//	BaseMinSize     string `json:"baseMinSize"`
//	QuoteMinSize    string `json:"quoteMinSize"`
//	BaseMaxSize     string `json:"baseMaxSize"`
//	QuoteMaxSize    string `json:"quoteMaxSize"`
//	BaseIncrement   string `json:"baseIncrement"`
//	QuoteIncrement  string `json:"quoteIncrement"`
//	PriceIncrement  string `json:"priceIncrement"`
//	FeeCurrency     string `json:"feeCurrency"`
//	EnableTrading   bool   `json:"enableTrading"`
//	IsMarginEnabled bool   `json:"isMarginEnabled"`
//	PriceLimitRate  string `json:"priceLimitRate"`
//	MinFunds        string `json:"minFunds"`
//}
//
//type KucoinSymbolsModel []*KucoinSymbolModel
//
//func appendRelatedMarketsKucoin(key string, marketInfos map[string]*model.MarketInfo) {
//	//client := kucoinRelatedClient("", "", "")
//	//resp, err := client.Symbols("")
//	//if err != nil || resp.Code != "200000" {
//	//	util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API error, response:%#v", key, "appendRelatedMarketsKucoin", resp))
//	//	return
//	//}
//	//symbols := KucoinSymbolsModel{}
//	//if err := resp.ReadData(&symbols); err != nil {
//	//	util.SocketInfo(fmt.Sprintf("key %s function: %s kucoin API read data error", key, "appendRelatedMarketsKucoin"))
//	//	return
//	//}
//	//for _, related := range symbols {
//	//	if !related.EnableTrading || related.QuoteCurrency != `USDT` {
//	//		continue
//	//	}
//	//	success, marketType, coin := model.GetCoinFromDialect(model.KucoinSpot, related.Symbol)
//	//	if !model.AppConfig.KucoinSpot && !related.IsMarginEnabled || !success {
//	//		continue
//	//	}
//	//	marketInfo := &model.MarketInfo{Market: model.KucoinSpot}
//	//	// TODO 此处需要确保kucoin的现货和期货的tail不同，否则marketTYpe不可用
//	//	marketInfo.Name = coin + model.UniStandardTail[marketType]
//	//	marketInfo.PriceIncrement, _ = strconv.ParseFloat(related.PriceIncrement, 64)
//	//	marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
//	//	marketInfo.SizeMin, _ = strconv.ParseFloat(related.BaseMinSize, 64)
//	//	marketInfo.SizeMax, _ = strconv.ParseFloat(related.BaseMaxSize, 64)
//	//	marketInfo.SizeIncrement, _ = strconv.ParseFloat(related.BaseIncrement, 64)
//	//	marketInfo.MoneyMin, _ = strconv.ParseFloat(related.MinFunds, 64)
//	//	marketInfos[marketInfo.Name] = marketInfo
//	//}
//	//relatedSettingMarkets = GetMarketSymbols(model.KucoinSpot)
//	util.NoticeLess(`%s %#v`, key, marketInfos)
//}
//
//func WsTickServeKucoinSpot() (channels []chan struct{}, err error) {
//	relatedClient := kucoinRelatedClient("", "", "")
//	relatedRsp, relatedErr := relatedClient.WebSocketPublicToken(nil)
//	if relatedErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket error:%s", "WsDepthServeKucoin", relatedErr))
//		return channels, relatedErr
//	}
//	relatedToken := &kucoin.WebSocketTokenModel{}
//	if relatedTokenErr := relatedRsp.ReadData(relatedToken); relatedTokenErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket error:%s", "WsDepthServeKucoin", relatedTokenErr))
//		return channels, relatedTokenErr
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
//	relatedSubscribe := kucoin.NewSubscribeMessage("/market/ticker:all", false)
//	if relatedSubscribeErr := relatedChannel.Subscribe(relatedSubscribe); relatedSubscribeErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket subscribe error:%s", "WsDepthServeKucoin", relatedSubscribeErr))
//		return channels, relatedSubscribeErr
//	}
//	util.Notice(fmt.Sprintf("kucoin finish create related websocket subscribe"))
//	relatedStopC := make(chan struct{}, 10)
//	go handlerKucoinRelatedWS(relatedChannelError, relatedMsg, relatedChannel, relatedStopC)
//	channels = append(channels, relatedStopC)
//	return channels, err
//}
//
//func handlerKucoinRelatedWS(relatedChannelError <-chan error, relatedMsg <-chan *kucoin.WebSocketDownstreamMessage, channel *kucoin.WebSocketClient, stopC chan struct{}) {
//	defer func() {
//		channel.Stop()
//	}()
//	for {
//		select {
//		case <-stopC:
//			util.Notice("get stop spot struct, return")
//			return
//		case cError := <-relatedChannelError:
//			util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket channel error:%s", "WsDepthServeKucoin", cError.Error()))
//			return
//		case msg := <-relatedMsg:
//			handleKucoinSpotWS(msg)
//		}
//	}
//}
//
//func handleKucoinSpotWS(relatedMsg *kucoin.WebSocketDownstreamMessage) {
//	ticker := &kucoin.TickerLevel1Model{}
//	if err := relatedMsg.ReadData(ticker); err != nil {
//		util.Notice(fmt.Sprintf("jsonerr Unmarshal err:%s", err.Error()))
//	}
//	if relatedMsg.Subject == "" {
//		return
//	}
//	if relatedSettingMarkets[relatedMsg.Subject] == false {
//		return
//	}
//	success, marketType, coin := model.GetCoinFromDialect(model.KucoinSpot, relatedMsg.Subject)
//	if !success {
//		return
//	}
//	// TODO 需要确保Kucoin的期货现货tail不同，否则marketType不可用
//	symbol := coin + model.UniStandardTail[marketType]
//	now := int(time.Now().UnixNano() / int64(time.Millisecond))
//	//util.Notice(fmt.Sprintf("币种：%s，当前ts：%d", symbol, now))
//	updateId, _ := strconv.ParseInt(ticker.Sequence, 10, 64)
//	bidPrice, _ := strconv.ParseFloat(ticker.BestBid, 64)
//	bidAmount, _ := strconv.ParseFloat(ticker.BestBidSize, 64)
//	askPrice, _ := strconv.ParseFloat(ticker.BestAsk, 64)
//	askAmount, _ := strconv.ParseFloat(ticker.BestAskSize, 64)
//	bidAsk := model.BidAsk{Ts: now, TsReceived: now, UpdateId: updateId,
//		Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount, Market: model.KucoinSpot, Symbol: symbol}},
//		Asks: []model.Tick{{Price: askPrice, Amount: askAmount, Market: model.KucoinSpot, Symbol: symbol}}}
//	markets := model.AppEnvironment
//	haveOld, old := markets.GetBidAsk(model.KucoinSpot, symbol)
//	if haveOld && old.UpdateId > bidAsk.UpdateId {
//		return
//	}
//	if markets.SetBidAsk(model.KucoinSpot, symbol, &bidAsk) {
//		funcHandlers := api.GetFunctions(model.KucoinSpot, symbol)
//		if funcHandlers != nil {
//			funcHandlers.Range(func(function, value interface{}) bool {
//				setting := api.GetSetting(function.(string), model.KucoinSpot, symbol)
//				if setting != nil && value != nil && value.(model.CarryHandler) != nil {
//					go value.(model.CarryHandler)(setting, &bidAsk)
//				}
//				return true
//			})
//		}
//	}
//}
//
//func getBalanceKucoinSpot(key string, secret string) (success bool, balances []*model.Balance) {
//	if model.AppConfig.KucoinSpot {
//		accountResp, err := kucoinRelatedClient("", "", "").Accounts(nil, "", "trade")
//		if err != nil || accountResp.Code != "200000" {
//			util.SocketInfo(fmt.Sprintf("fail to refresh spot balance kucoin, err:%s, response:%#v", err, accountResp))
//			time.Sleep(time.Minute * 5)
//			return getBalanceKucoinSpot(key, secret)
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
//			balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.KucoinSpot, Coin: account.Currency}
//			balance.FrozenAmount, _ = strconv.ParseFloat(account.Holds, 64)
//			balance.Amount, _ = strconv.ParseFloat(account.Balance, 64)
//			balance.AvailableWithBorrow, _ = strconv.ParseFloat(account.Available, 64)
//			_, price := api.GetPriceForce(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.KucoinSpot)
//			balance.UsdValue = balance.Amount * price
//			balances = append(balances, balance)
//		}
//	} else {
//		accountResp, err := kucoinRelatedClient("", "", "").MarginAccount(nil)
//		if err != nil || accountResp.Code != "200000" {
//			util.SocketInfo(fmt.Sprintf("fail to refresh margin balance kucoin, err:%s, response:%#v", err, accountResp))
//			time.Sleep(time.Minute * 5)
//			return getBalanceKucoinSpot(key, secret)
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
//			balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.KucoinSpot, Coin: account.Currency}
//			balance.FrozenAmount, _ = account.HoldBalance.Float64()
//			available, _ := account.AvailableBalance.Float64()
//			balance.Borrow, _ = account.Liability.Float64()
//			canBorrow, _ := account.MaxBorrowSize.Float64()
//			balance.AvailableWithBorrow = available + canBorrow
//			balance.Amount, _ = account.TotalBalance.Float64()
//			balance.Amount = balance.Amount - balance.Borrow
//			_, price := api.GetPriceForce(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], model.KucoinSpot)
//			balance.UsdValue = balance.Amount * price
//			balances = append(balances, balance)
//		}
//	}
//	return true, balances
//}
//
//func cancelOrdersKucoinSpot(symbol string) (result bool) {
//	success, marketType, _, dialectSymbol := model.GetFromStandard(model.KucoinSpot, symbol)
//	if success && marketType == model.MarketTypeSpot {
//		param := map[string]string{}
//		if model.AppConfig.KucoinSpot {
//			param["tradeType"] = "TRADE"
//		} else {
//			param["tradeType"] = "MARGIN_TRADE"
//		}
//		param["symbol"] = dialectSymbol
//		apiResponse, err := kucoinRelatedClient("", "", "").CancelOrders(nil, param)
//		if err != nil || apiResponse.Code != "200000" {
//			util.SocketInfo(fmt.Sprintf("function: %s fail to cancel related orders kucoin, err:%s, response:%#v", "cancelOrdersKucoin", err, apiResponse))
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
//
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
//
//func placeOrderKucoinSpot(order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
//	success, marketType, _, dialectSymbol := model.GetFromStandard(model.KucoinSpot, symbol)
//	if success && marketType == model.MarketTypeSpot {
//		if model.AppConfig.KucoinSpot {
//			createOrder := &kucoin.CreateOrderModel{}
//			createOrder.ClientOid = "r" + strconv.FormatInt(time.Now().UnixNano(), 10)
//			createOrder.Symbol = dialectSymbol
//			createOrder.Side = orderSide
//			createOrder.Type = orderType
//			priceSpot, decimalSpot := model.FormatPrice(model.KucoinSpot, symbol, price)
//			order.Price = priceSpot
//			createOrder.Price = util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
//			createOrder.Size = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.KucoinSpot, symbol, amount, price, false)))
//			util.SocketInfo(fmt.Sprintf(`create spot order request: %#v`, createOrder))
//			spotOrderResponse, err := kucoinRelatedClient("", "", "").CreateOrder(nil, createOrder)
//			if err != nil || spotOrderResponse.Code != "200000" {
//				util.SocketInfo(fmt.Sprintf("function: %s fail to create spot order kucoin, err:%s, response:%#v", "placeOrderKucoin", err, spotOrderResponse))
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
//			createOrder.Symbol = dialectSymbol
//			createOrder.Side = orderSide
//			createOrder.Type = orderType
//			createOrder.MarginMode = "cross"
//			createOrder.AutoBorrow = true
//			priceSpot, decimalSpot := model.FormatPrice(model.KucoinSpot, symbol, price)
//			createOrder.Price = util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
//			createOrder.Size = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Kucoin, symbol, amount, price, false)))
//			util.SocketInfo(fmt.Sprintf(`create margin order request: %#v`, createOrder))
//			req := kucoin.NewRequest(http.MethodPost, "/api/v1/margin/order", createOrder)
//			//todo CreateMarginOrder
//			marginOrderResp, err := kucoinRelatedClient("", "", "").Call(nil, req)
//			if err != nil || marginOrderResp.Code != "200000" {
//				util.SocketInfo(fmt.Sprintf("function: %s fail to create margin order kucoin, err:%s, response:%#v", "placeOrderKucoin", err, marginOrderResp))
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
//
//func queryOrderKucoinSpot(symbol string, orderId string) (order *model.Order) {
//	orderResponse, respErr := kucoinRelatedClient("", "", "").Order(nil, orderId)
//	if respErr != nil || orderResponse.Code != "200000" {
//		util.SocketInfo(fmt.Sprintf("function: %s fail to query kucoin spot order , err:%s, response:%#v", "queryOrderKucoinSpot", respErr, orderResponse))
//		return
//	}
//	orderResult := &kucoin.OrderModel{}
//	readErr := orderResponse.ReadData(orderResult)
//	if readErr != nil {
//		util.SocketInfo(fmt.Sprintf("function: %s fail to parse query kucoin spot order response , err:%s", "queryOrderKucoinSpot", readErr))
//		return
//	}
//	order = &model.Order{Market: model.KucoinSpot, Status: model.CarryStatusFail}
//	if orderResult != nil {
//		order.OrderId = orderId
//		order.Symbol = symbol
//		order.OrderSide = strings.ToLower(orderResult.Side)
//		order.OrderType = strings.ToLower(orderResult.Type)
//		order.Amount, _ = strconv.ParseFloat(orderResult.Size, 64)
//		order.Price, _ = strconv.ParseFloat(orderResult.Price, 64)
//		order.DealAmount, _ = strconv.ParseFloat(orderResult.DealSize, 64)
//		order.OrderTime = time.Unix(orderResult.CreatedAt, 0)
//		if orderResult.IsActive {
//			order.Status = model.CarryStatusWorking
//		} else {
//			if order.DealAmount > 0 {
//				order.Status = model.CarryStatusSuccess
//			} else {
//				order.Status = model.CarryStatusFail
//			}
//		}
//		if order.DealAmount > 0 && order.DealPrice == 0 {
//			order.DealPrice = order.Price
//		}
//	}
//	return
//}

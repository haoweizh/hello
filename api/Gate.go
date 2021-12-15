package api

import (
	"context"
	"encoding/json"
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

func getClientGate(key, secret string) (apiClient *gateApi.APIClient, ctx context.Context) {
	if key == `` {
		keys, secrets := model.AppConfig.GetKeys(model.Gate)
		if len(keys) > 0 && len(secrets) > 0 {
			key = keys[0]
			secret = secrets[0]
		}
	}
	if apiClientsGate[key] == nil {
		apiClientsGate[key] = gateApi.NewAPIClient(gateApi.NewConfiguration())
		apiCtxGate[key] = context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
			Key: key, Secret: secret})
	}
	return apiClientsGate[key], apiCtxGate[key]
}

func getMarketsGate(key, secret string) (success bool, marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	appendRelatedMarketsGate(key, secret, marketInfos)
	appendFutureMarketGate(key, secret, marketInfos)
	return true, marketInfos
}

func appendFutureMarketGate(key, secret string, marketInfos map[string]*model.MarketInfo) {
	client, ctx := getClientGate(key, secret)
	contracts, _, futureErr := client.FuturesApi.ListFuturesContracts(ctx, "usdt")
	if futureErr != nil {
		panicGateError(key, "ListFuturesContracts", futureErr)
	}
	for _, contract := range contracts {
		if contract.InDelisting {
			continue
		}
		marketInfo := &model.MarketInfo{Market: model.Gate}
		coin := strings.Split(contract.Name, `_`)[0]
		marketInfo.Name = coin + model.GetPerpTail(model.Gate)
		minPrice, _ := strconv.ParseFloat(contract.OrderPriceRound, 64)
		marketInfo.PriceIncrement = minPrice
		marketInfo.PriceDecimal = util.NumDecPlaces(minPrice)
		marketInfo.SizeMin = float64(contract.OrderSizeMin)
		marketInfo.SizeMax = float64(contract.OrderSizeMax)
		marketInfo.SizeIncrement = marketInfo.SizeMin
		marketInfo.CTCurrency = coin
		marketInfo.CTValue, _ = strconv.ParseFloat(contract.QuantoMultiplier, 64)
		marketInfos[marketInfo.Name] = marketInfo
	}
}

func appendRelatedMarketsGate(key, secret string, marketInfos map[string]*model.MarketInfo) {
	client, ctx := getClientGate(key, secret)
	spotCurrencyPairs, _, spotErr := client.SpotApi.ListCurrencyPairs(ctx)
	if spotErr != nil {
		panicGateError(key, "ListCurrencyPairs", spotErr)
	}
	if model.AppConfig.GateSpot {
		for _, spot := range spotCurrencyPairs {
			if spot.TradeStatus != "tradable" {
				continue
			}
			marketInfo := &model.MarketInfo{Market: model.Gate}
			marketInfo.Name = spot.Id
			marketInfo.PriceDecimal = int(spot.Precision)
			marketInfo.PriceIncrement = 1 / math.Pow10(int(spot.Precision))
			marketInfo.SizeIncrement = 1 / math.Pow10(int(spot.AmountPrecision))
			if spot.MinQuoteAmount != "" {
				marketInfo.UsdtMin, _ = strconv.ParseFloat(spot.MinQuoteAmount, 64)
				marketInfo.SizeMin = marketInfo.SizeIncrement
			}
			if spot.MinBaseAmount != "" {
				marketInfo.SizeMin, _ = strconv.ParseFloat(spot.MinBaseAmount, 64)
			}
			marketInfos[spot.Id] = marketInfo
		}
	} else {
		marginCurrencyPairs, _, marginErr := client.MarginApi.ListCrossMarginCurrencies(ctx)
		if marginErr != nil {
			panicGateError(key, "ListCrossMarginCurrencies", marginErr)
		}
		for _, margin := range marginCurrencyPairs {
			symbol := margin.Name + model.GetSpotTail(model.Gate)
			for _, spot := range spotCurrencyPairs {
				if spot.Id == symbol {
					if spot.TradeStatus != "tradable" {
						break
					}
					//spotData, _ := json.Marshal(spot)
					//marginData, _ := json.Marshal(margin)
					//util.Notice(fmt.Sprintf("现货交易对：%s", spotData))
					//util.Notice(fmt.Sprintf("杠杆交易对：%s", marginData))
					marketInfo := &model.MarketInfo{Market: model.Gate}
					marketInfo.Name = symbol
					marketInfo.PriceDecimal = int(spot.Precision)
					marketInfo.PriceIncrement = 1 / math.Pow10(int(spot.Precision))
					marketInfo.SizeIncrement = 1 / math.Pow10(int(spot.AmountPrecision))
					if spot.MinQuoteAmount != "" {
						marketInfo.UsdtMin, _ = strconv.ParseFloat(spot.MinQuoteAmount, 64)
						marketInfo.SizeMin = marketInfo.SizeIncrement
					}
					if spot.MinBaseAmount != "" {
						marketInfo.SizeMin, _ = strconv.ParseFloat(spot.MinBaseAmount, 64)
					}
					marketInfo.BorrowSizeMin, _ = strconv.ParseFloat(margin.MinBorrowAmount, 64)
					marketInfo.BorrowUsdtMax, _ = strconv.ParseFloat(margin.UserMaxBorrowAmount, 64)
					marketInfos[symbol] = marketInfo
					break
				}
			}
		}
	}
}

func setPosSideGate(key, secret string) {
	client, ctx := getClientGate(key, secret)
	mode, _, err := client.FuturesApi.SetDualMode(ctx, "usdt", false)
	if err != nil {
		panicGateError(key, "setPosSideGate", err)
	}
	marshal, _ := json.Marshal(mode)
	util.Notice(fmt.Sprintf("set gate dual mode success,position: %s", marshal))
}

func setMarginSettingGate(key, secret string) {
	client, ctx := getClientGate(key, secret)
	mode, _, err := client.MarginApi.SetAutoRepay(ctx, "on")
	if err != nil {
		panicGateError(key, "setMarginSettingGate", err)
	}
	marshal, _ := json.Marshal(mode)
	util.Notice(fmt.Sprintf("set gate margin auto repay success,response: %s", marshal))
}

func transferGate(key string, secret string, transferType string, amount float64) {
	client, ctx := getClientGate(key, secret)
	param := gateApi.Transfer{Currency: "USDT", Amount: fmt.Sprintf("%.6f", amount), Settle: "usdt"}
	if transferType == "MAIN_UMFUTURE" {
		if model.AppConfig.GateSpot {
			param.From = "spot"
			param.To = "futures"
			_, endErr := client.WalletApi.Transfer(ctx, param)
			if endErr != nil {
				panicGateError(key, "transferGate", endErr)
			}
		} else {
			param.From = "cross_margin"
			param.To = "spot"
			_, err := client.WalletApi.Transfer(ctx, param)
			if err != nil {
				panicGateError(key, "transferGate", err)
			} else {
				param.From = "spot"
				param.To = "futures"
				_, endErr := client.WalletApi.Transfer(ctx, param)
				if endErr != nil {
					panicGateError(key, "transferGate", endErr)
				}
			}
		}
	} else if transferType == "UMFUTURE_MAIN" {
		param.From = "futures"
		param.To = "spot"
		_, err := client.WalletApi.Transfer(ctx, param)
		if err != nil {
			panicGateError(key, "transferGate", err)
		} else {
			if !model.AppConfig.GateSpot {
				param.From = "spot"
				param.To = "cross_margin"
				_, endErr := client.WalletApi.Transfer(ctx, param)
				if endErr != nil {
					panicGateError(key, "transferGate", endErr)
				}
			}
		}
	}
}

func panicGateError(key, function string, err error) {
	if e, ok := err.(gateApi.GateAPIError); ok {
		util.SocketInfo(fmt.Sprintf("key %s function: %s Gate API error, label: %s, message: %s",
			key, function, e.Label, e.Message))
	}
	util.Notice(function + err.Error())
}

type FuturesBookTickerModel struct {
	TimeMillis   int64  `json:"t"`
	Contract     string `json:"s"`
	FirstId      int64  `json:"U"`
	LastId       int64  `json:"u"`
	BestBidPrice string `json:"b"`
	BestBidSize  int64  `json:"B"`
	BestAskPrice string `json:"a"`
	BestAskSize  int64  `json:"A"`
}

var tickerHandler = gateWs.NewCallBack(func(msg *gateWs.UpdateMsg) {
	if msg.Error != nil {
		util.Notice(fmt.Sprintf("callback error: %s %s", msg.Channel, msg.Error.Error()))
	}
	var bidAsk model.BidAsk
	var symbol string
	switch msg.Channel {
	// Pushes any update about the price and amount of best bid or ask price in realtime for subscribed currency pairs.
	case gateWs.ChannelSpotBookTicker:
		var update gateWs.SpotBookTickerMsg
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Notice(fmt.Sprintf("spot book ticker Unmarshal err:%s", err.Error()))
		}
		if update.CurrencyPair == "" {
			return
		}
		symbol = update.CurrencyPair
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(update.Bid, 64)
		bidAmount, _ := strconv.ParseFloat(update.BidSize, 64)
		askPrice, _ := strconv.ParseFloat(update.Ask, 64)
		askAmount, _ := strconv.ParseFloat(update.AskSize, 64)
		bidAsk = model.BidAsk{Ts: int(update.TimeInMilli), TsReceived: now, UpdateId: update.LastId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount}}}
	// Periodically notify top bids and asks snapshot with limited levels.
	case gateWs.ChannelSpotOrderBook:
		var update gateWs.SpotUpdateAllDepthMsg
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Notice(fmt.Sprintf("spot book ticker Unmarshal err:%s", err.Error()))
		}
		if update.CurrencyPair == "" {
			return
		}
		symbol = update.CurrencyPair
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidAsk = model.BidAsk{Ts: int(update.TimeInMilli), TsReceived: now, UpdateId: update.LastUpdateId,
			Bids: []model.Tick{}, Asks: []model.Tick{}}
		for i := 0; i < len(update.Bid) && i < len(update.Ask); i++ {
			bidPrice, _ := strconv.ParseFloat(update.Bid[0][0], 64)
			bidAmount, _ := strconv.ParseFloat(update.Bid[0][1], 64)
			askPrice, _ := strconv.ParseFloat(update.Ask[0][0], 64)
			askAmount, _ := strconv.ParseFloat(update.Ask[0][1], 64)
			bidAsk.Bids = append(bidAsk.Bids, model.Tick{Price: bidPrice, Amount: bidAmount})
			bidAsk.Asks = append(bidAsk.Asks, model.Tick{Price: askPrice, Amount: askAmount})
		}
	// Push best bid and ask in real-time.
	case gateWs.ChannelFutureBookTicker:
		var update FuturesBookTickerModel
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Notice(fmt.Sprintf("future book ticker Unmarshal err:%s", err.Error()))
		}
		if update.Contract == "" {
			return
		}
		symbol = strings.Split(update.Contract, "_")[0] + model.GetPerpTail(model.Gate)
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(update.BestBidPrice, 64)
		_, bidAmount := model.ParseRealAmount(model.Gate, symbol, float64(update.BestBidSize))
		askPrice, _ := strconv.ParseFloat(update.BestAskPrice, 64)
		_, askAmount := model.ParseRealAmount(model.Gate, symbol, float64(update.BestAskSize))
		bidAsk = model.BidAsk{Ts: int(update.TimeMillis), TsReceived: now, UpdateId: update.LastId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount}}}
	}
	markets := model.AppMarkets
	haveOld, old := markets.GetBidAsk(symbol, model.Gate)
	if haveOld && old.Ts > bidAsk.Ts {
		return
	}
	if markets.SetBidAsk(symbol, model.Gate, &bidAsk) {
		for function, handler := range model.GetFunctions(model.Gate, symbol) {
			if handler != nil {
				settings := model.GetSetting(function, model.Gate, symbol)
				for _, setting := range settings {
					go handler(setting, &bidAsk)
				}
			}
		}
	}
})

//	var update gateWs.FuturesOrderBookUpdate
//	if err := json.Unmarshal(msg.Result, &update); err != nil {
//		util.Notice(fmt.Sprintf("future book ticker Unmarshal err:%s", err.Error()))
//	}
//	if update.Contract == "" {
//		return
//	}
//	symbol := strings.Split(update.Contract, "_")[0] + model.GetPerpTail(model.Gate)
//	now := int(time.Now().UnixNano() / int64(time.Millisecond))
//	if len(update.Bids) == 0 || len(update.Asks) == 0 {
//		return
//	}
//	var bidPrice,bidAmount,askPrice,askAmount float64
//	for _, bid := range update.Bids {
//		if bid.P == "" || bid.S == 0 {
//			continue
//		}
//		bidPrice, _ = strconv.ParseFloat(bid.P, 64)
//		_, bidAmount = model.ParseRealAmount(model.Gate, symbol, float64(bid.S))
//		break
//	}
//	for _, ask := range update.Asks {
//		if ask.P == "" || ask.S == 0 {
//			continue
//		}
//		askPrice, _ = strconv.ParseFloat(ask.P, 64)
//		_, askAmount = model.ParseRealAmount(model.Gate, symbol, float64(ask.S))
//		break
//	}
//	if bidPrice == 0 || bidAmount == 0 || askPrice == 0 || askAmount == 0 {
//		return
//	}
//	bidAsk := model.BidAsk{Ts: int(update.TimeMillis), TsReceived: now, UpdateId: update.LastId,
//		Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount}},
//		Asks: []model.Tick{{Price: askPrice, Amount: askAmount}}}

//futureBookWs, futureBookErr := gateWs.NewWsService(nil, nil, gateWs.NewConnConfFromOption(&gateWs.ConfOptions{
//	URL:          gateWs.FuturesUsdtUrl,
//	Key:          keys[0],
//	Secret:       secrets[0],
//	MaxRetryConn: 10,
//}))
//if futureBookErr != nil {
//	util.Notice(fmt.Sprintf("new future book wsService err:%s", futureBookErr))
//}

func WsDepthServeGate() (err error) {
	keys, secrets := model.AppConfig.GetKeys(model.Gate)
	wsSpotUpdate, spotErr := gateWs.NewWsService(nil, nil, gateWs.NewConnConfFromOption(&gateWs.ConfOptions{
		URL:          gateWs.BaseUrl,
		Key:          keys[0],
		Secret:       secrets[0],
		MaxRetryConn: 10,
	}))
	wsFutureUpdate, futureErr := gateWs.NewWsService(nil, nil, gateWs.NewConnConfFromOption(&gateWs.ConfOptions{
		URL:          gateWs.FuturesUsdtUrl,
		Key:          keys[0],
		Secret:       secrets[0],
		MaxRetryConn: 10,
	}))
	wsSpot, spotBookErr := gateWs.NewWsService(nil, nil, gateWs.NewConnConfFromOption(&gateWs.ConfOptions{
		URL:          gateWs.BaseUrl,
		Key:          keys[0],
		Secret:       secrets[0],
		MaxRetryConn: 10,
	}))
	if spotBookErr != nil {
		util.Notice(fmt.Sprintf("new spot book wsService err:%s", spotBookErr))
	}
	if spotErr != nil {
		util.Notice(fmt.Sprintf("new spot wsService err:%s", spotErr))
		return spotErr
	}
	if futureErr != nil {
		util.Notice(fmt.Sprintf("new future wsService err:%s", futureErr))
		return futureErr
	}
	spotSubs := make([]string, 0)
	futureSubs := make([]string, 0)
	symbols := model.GetMarketSymbols(model.Gate)
	for symbol := range symbols {
		if strings.LastIndex(symbol, model.GetSpotTail(model.Gate)) == len(symbol)-len(model.GetSpotTail(model.Gate)) &&
			len(symbol)-len(model.GetSpotTail(model.Gate)) > 0 {
			spotSubs = append(spotSubs, symbol)
			spotPayload := append(make([]string, 0), symbol, "5", "100ms")
			if spotBookSubErr := wsSpot.Subscribe(gateWs.ChannelSpotOrderBook, spotPayload); spotBookSubErr != nil {
				util.Notice(fmt.Sprintf("spotBookWs Subscribe err:%s", spotBookSubErr.Error()))
			}
		}
		if strings.LastIndex(symbol, model.GetPerpTail(model.Gate)) == len(symbol)-len(model.GetPerpTail(model.Gate)) &&
			len(symbol)-len(model.GetPerpTail(model.Gate)) > 0 {
			futureSubs = append(futureSubs, symbol)
		}
	}
	wsSpot.SetCallBack(gateWs.ChannelSpotOrderBook, tickerHandler)
	wsSpotUpdate.SetCallBack(gateWs.ChannelSpotBookTicker, tickerHandler)
	if spotSubErr := wsSpotUpdate.Subscribe(gateWs.ChannelSpotBookTicker, spotSubs); spotSubErr != nil {
		util.Notice(fmt.Sprintf("spotWs Subscribe err:%s", spotSubErr.Error()))
		return spotSubErr
	}
	wsFutureUpdate.SetCallBack(gateWs.ChannelFutureBookTicker, tickerHandler)
	if futureSubErr := wsFutureUpdate.Subscribe(gateWs.ChannelFutureBookTicker, futureSubs); futureSubErr != nil {
		util.Notice(fmt.Sprintf("futureWs Subscribe err:%s", futureSubErr.Error()))
		return futureSubErr
	}
	return err
}

func getBalanceGate(key string, secret string) (success bool, balances []*model.Balance) {
	client, ctx := getClientGate(key, secret)
	if model.AppConfig.GateSpot {
		accounts, _, err := client.SpotApi.ListSpotAccounts(ctx, nil)
		if err != nil {
			panicGateError(key, "getBalanceGate", err)
			time.Sleep(time.Second * 2)
			util.SocketInfo(`fail to refresh spot balance gate`)
			return getBalanceGate(key, secret)
		}
		for _, account := range accounts {
			balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Gate, Coin: account.Currency}
			balance.FrozenAmount, _ = strconv.ParseFloat(account.Locked, 64)
			// 此处未计算可以借入的金额
			balance.AvailableWithBorrow, _ = strconv.ParseFloat(account.Available, 64)
			balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
			priceGet, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.GetSpotTail(model.Gate), model.Gate)
			if priceGet {
				balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
			}
			balances = append(balances, balance)
		}
	} else {
		account, _, err := client.MarginApi.GetCrossMarginAccount(ctx)
		if err != nil {
			panicGateError(key, "getBalanceGate", err)
			time.Sleep(time.Second * 2)
			util.SocketInfo(`fail to refresh margin balance gate`)
			return getBalanceGate(key, secret)
		}
		if account.Locked {
			util.Notice(fmt.Sprintf("margin account is locked"))
			return false, balances
		}
		balances = make([]*model.Balance, 0)
		for index, item := range account.Balances {
			balance := &model.Balance{AccountId: key, BalanceTime: util.GetNow(), Market: model.Gate, Coin: index}
			balance.FrozenAmount, _ = strconv.ParseFloat(item.Freeze, 64)
			balance.Borrow, _ = strconv.ParseFloat(item.Borrowed, 64)
			// 此处未计算可以借入的金额
			balance.AvailableWithBorrow, _ = strconv.ParseFloat(item.Available, 64)
			balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
			priceGet, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.GetSpotTail(model.Gate), model.Gate)
			if priceGet {
				balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
			}
			balances = append(balances, balance)
		}
	}
	return true, balances
}

func getPositionsGate(key string, secret string) (success bool, positions []*model.Position, posBalance float64) {
	client, ctx := getClientGate(key, secret)
	account, _, accountErr := client.FuturesApi.ListFuturesAccounts(ctx, "usdt")
	positionList, _, positionsErr := client.FuturesApi.ListPositions(ctx, "usdt")
	if accountErr != nil || positionsErr != nil {
		if accountErr != nil {
			panicGateError(key, "getFuturesAccountsGate", accountErr)
		}
		if positionsErr != nil {
			panicGateError(key, "getPositionsGate", positionsErr)
		}
		time.Sleep(time.Second * 2)
		util.SocketInfo(`fail to refresh future balance gate`)
		return getPositionsGate(key, secret)
	}
	posBalance, _ = strconv.ParseFloat(account.Total, 64)
	positions = make([]*model.Position, 0)
	for _, item := range positionList {
		currency := strings.Split(item.Contract, "_")[0] + model.GetPerpTail(model.Gate)
		position := &model.Position{Market: model.Gate, Ts: util.GetNowUnixMillion(), Currency: currency}
		_, realAmount := model.ParseRealAmount(model.Gate, currency, float64(item.Size))
		position.Free = realAmount
		position.LeverRate, _ = strconv.ParseInt(item.Leverage, 10, 64)
		position.EntryPrice, _ = strconv.ParseFloat(item.EntryPrice, 64)
		position.Margin, _ = strconv.ParseFloat(item.Margin, 64)
		position.LiquidationPrice, _ = strconv.ParseFloat(item.LiqPrice, 64)
		positions = append(positions, position)
	}
	return true, positions, posBalance
}

func cancelOrderGate(key, secret, symbol, orderId string) (result bool) {
	client, ctx := getClientGate(key, secret)
	if strings.Contains(symbol, model.GetSpotTail(model.Gate)) {
		param := &gateApi.CancelOrderOpts{}
		if model.AppConfig.GateSpot {
			param.Account = optional.NewString("spot")
		} else {
			param.Account = optional.NewString("cross_margin")
		}
		order, _, err := client.SpotApi.CancelOrder(ctx, orderId, symbol, param)
		if err != nil {
			panicGateError(key, fmt.Sprintf("cancelSpotOrdersGate %s %s", symbol, orderId), err)
			return false
		}
		marshal, _ := json.Marshal(order)
		util.SocketInfo(`cancel related order response: %s`, marshal)
		return true
	} else {
		symbol = model.GetCoin(model.Gate, symbol) + model.GetSpotTail(model.Gate)
		order, _, err := client.FuturesApi.CancelFuturesOrder(ctx, "usdt", orderId)
		if err != nil {
			panicGateError(key, "cancelFutureOrderGate", err)
			return false
		}
		marshal, _ := json.Marshal(order)
		util.SocketInfo(`cancel future order response: %s`, marshal)
		return true
	}
}

func cancelOrdersGate(key string, secret string, symbol string) (result bool) {
	client, ctx := getClientGate(key, secret)
	if strings.Contains(symbol, model.GetSpotTail(model.Gate)) {
		param := &gateApi.CancelOrdersOpts{}
		if model.AppConfig.GateSpot {
			param.Account = optional.NewString("spot")
		} else {
			param.Account = optional.NewString("cross_margin")
		}
		orders, _, err := client.SpotApi.CancelOrders(ctx, symbol, param)
		if err != nil {
			panicGateError(key, "cancelSpotOrdersGate", err)
			return false
		}
		marshal, _ := json.Marshal(orders)
		util.SocketInfo(`cancel related orders response: %s`, marshal)
		return true
	} else {
		symbol = model.GetCoin(model.Gate, symbol) + model.GetSpotTail(model.Gate)
		orders, _, err := client.FuturesApi.CancelFuturesOrders(ctx, "usdt", symbol, nil)
		if err != nil {
			panicGateError(key, "cancelFutureOrdersGate", err)
			return false
		}
		marshal, _ := json.Marshal(orders)
		util.SocketInfo(`cancel future orders response: %s`, marshal)
		return true
	}
}

func placeOrderGate(key, secret string, order *model.Order, orderSide, orderType, symbol string, price, amount float64) {
	client, ctx := getClientGate(key, secret)
	orderPrice, decimal := model.FormatPrice(model.Gate, symbol, model.OrderSideBuy, price)
	orderPriceStr := util.CutTailZero(strconv.FormatFloat(orderPrice, 'f', decimal, 64))
	if strings.Contains(symbol, model.GetSpotTail(model.Gate)) {
		relatedOrder := gateApi.Order{Price: orderPriceStr, Side: orderSide, CurrencyPair: symbol, Type: orderType}
		if model.AppConfig.GateSpot {
			relatedOrder.Account = "spot"
		} else {
			relatedOrder.Account = "cross_margin"
			if orderSide == model.OrderSideBuy {
				relatedOrder.AutoRepay = true
			} else {
				relatedOrder.AutoBorrow = true
			}
		}
		relatedOrder.Amount = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Gate, symbol, amount)))
		util.Notice(`create related order request: %v`, relatedOrder)
		createOrder, _, err := client.SpotApi.CreateOrder(ctx, relatedOrder)
		if err != nil {
			panicGateError(key, "placeSpotOrderGate", err)
			order.Status = model.CarryStatusFail
			order.OrderId = ``
		} else {
			orderResp, _ := json.Marshal(createOrder)
			util.Notice(`create related order response: %s`, orderResp)
			order.OrderId = createOrder.Id
			order.Symbol = createOrder.CurrencyPair
			secondUnix, _ := strconv.ParseInt(createOrder.CreateTime, 10, 64)
			order.OrderTime = time.Unix(secondUnix, 0)
			if orderPriceStr != createOrder.Price {
				util.Notice(fmt.Sprintf(`diff spot price req %s resp %s`, orderPriceStr, createOrder.Price))
			}
			order.Price, _ = strconv.ParseFloat(createOrder.Price, 64)
			order.OrderSide = createOrder.Side
			order.Amount, _ = strconv.ParseFloat(createOrder.Amount, 64)
			order.OrderType = createOrder.Type
			if createOrder.Status == "cancelled" {
				order.Status = model.CarryStatusFail
			} else {
				order.Status = model.CarryStatusWorking
			}
			return
		}
	} else {
		futuresOrder := gateApi.FuturesOrder{Price: orderPriceStr,
			Contract: model.GetCoin(model.Gate, symbol) + model.GetSpotTail(model.Gate)}
		futuresOrder.Size, _ = strconv.ParseInt(util.CutTailZero(
			fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Gate, symbol, amount))), 10, 64)
		if orderSide == model.OrderSideSell {
			futuresOrder.Size = -1 * futuresOrder.Size
		}
		util.Notice(`create future order request: %v`, futuresOrder)
		createFuturesOrder, _, err := client.FuturesApi.CreateFuturesOrder(ctx, "usdt", futuresOrder)
		if err != nil {
			panicGateError(key, "placeFutureOrderGate", err)
			order.Status = model.CarryStatusFail
			order.OrderId = ``
		} else {
			orderResp, _ := json.Marshal(createFuturesOrder)
			util.Notice(`create future order response: %s`, orderResp)
			if createFuturesOrder.IsLiq {
				util.Notice(fmt.Sprintf("warning warning, blow up!!!"))
			}
			order.OrderId = strconv.FormatInt(createFuturesOrder.Id, 10)
			order.Symbol = model.GetCoin(model.Gate, symbol) + model.GetPerpTail(model.Gate)
			order.OrderTime = time.Unix(int64(createFuturesOrder.CreateTime), 0)
			if orderPriceStr != createFuturesOrder.Price {
				util.Notice(fmt.Sprintf(`diff future price req %s resp %s`, orderPriceStr, createFuturesOrder.Price))
			}
			order.Price, _ = strconv.ParseFloat(createFuturesOrder.Price, 64)
			_, order.Amount = model.ParseRealAmount(model.Gate, order.Symbol, float64(createFuturesOrder.Size))
			order.Status = model.CarryStatusWorking
			return
		}
	}
}

func getMaxLoanGate(coin string) (success bool, maxLoan float64) {
	symbol := coin + model.GetSpotTail(model.Gate)
	marketMargin := model.GetMarketInfo(model.Gate, symbol)
	_, tickRelated := model.AppMarkets.GetBidAsk(symbol, model.Gate)
	if tickRelated != nil && marketMargin != nil {
		maxLoan = marketMargin.BorrowUsdtMax / tickRelated.Bids[0].Price
	}
	return true, maxLoan
}

func getFundingRateGate(key, secret, symbol string) (fundingRate *model.FundingRate) {
	client, ctx := getClientGate(key, secret)
	contract, _, err := client.FuturesApi.GetFuturesContract(
		ctx, `usdt`, model.GetCoin(model.Gate, symbol)+model.GetSpotTail(model.Gate))
	if err != nil {
		panicGateError(key, "getFundingRateGate", err)
		return nil
	}
	rate, _ := strconv.ParseFloat(contract.FundingRate, 64)
	return &model.FundingRate{
		Rate:       rate,
		UpdateTime: time.Now().UnixNano(),
		ExpireTime: int64(contract.FundingNextApply),
		Symbol:     symbol,
	}
}

func setBidAskGate(key, secret, symbol string) {
	client, ctx := getClientGate(key, secret)
	contract := model.GetCoin(model.Gate, symbol) + `_USDT`
	orderBook, _, err := client.FuturesApi.ListFuturesOrderBook(ctx, `usdt`, contract,
		&gateApi.ListFuturesOrderBookOpts{Limit: optional.NewInt32(1)})
	if err != nil {
		panicGateError(key, "setFutureTicker", err)
	}
	result, oldBidAsk := model.AppMarkets.GetBidAsk(symbol, model.Gate)
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
		Bids:       []model.Tick{{Price: bidPrice, Amount: bidAmount}},
		Asks:       []model.Tick{{Price: askPrice, Amount: askAmount}}}
	model.AppMarkets.SetBidAsk(symbol, model.Gate, &bidAsk)
}

func queryOrderGate(key, secret string, order *model.Order) {
	client, ctx := getClientGate(key, secret)
	tailPerp := model.GetPerpTail(model.Gate)
	tailSpot := model.GetSpotTail(model.Gate)
	if tailPerp == order.Symbol[len(order.Symbol)-len(tailPerp):] {
		orderFuture, _, err := client.FuturesApi.GetFuturesOrder(ctx, `usdt`, order.OrderId)
		if err != nil {
			panicGateError(key, "GetFuturesOrder", err)
			return
		}
		order.DealPrice, _ = strconv.ParseFloat(orderFuture.FillPrice, 64)
		if orderFuture.Status == `open` {
			order.Status = model.CarryStatusWorking
		} else if orderFuture.Status == `finished` {
			switch orderFuture.FinishAs {
			case `filled`:
				order.Status = model.CarryStatusSuccess
			case `cancelled`, `liquidated`, `ioc`, `auto_deleveraged`, `reduce_only`, `position_closed`, `reduce_out`:
				order.Status = model.CarryStatusFail
			}
		}
		_, order.DealAmount = model.ParseRealAmount(order.Market, order.Symbol, float64(orderFuture.Size-orderFuture.Left))
		util.SocketInfo(`%s %s %s query result:%s %f %v`,
			order.Market, order.Symbol, order.OrderId, order.Status, order.DealAmount, orderFuture)
	} else if tailSpot == order.Symbol[len(order.Symbol)-len(tailSpot):] {
		orderSpot, _, err := client.SpotApi.GetOrder(ctx, order.OrderId, order.Symbol, nil)
		if err != nil {
			panicGateError(key, "GetSpotOrder", err)
			return
		}
		order.DealAmount, _ = strconv.ParseFloat(orderSpot.FilledTotal, 64)
		order.DealPrice, _ = strconv.ParseFloat(orderSpot.FillPrice, 64)
		switch orderSpot.Status {
		case `open`:
			order.Status = model.CarryStatusWorking
		case `closed`:
			order.Status = model.CarryStatusSuccess
		case `cancelled`:
			order.Status = model.CarryStatusFail
		}
		util.SocketInfo(`%s %s %s query result:%s %f %v`,
			order.Market, order.Symbol, order.OrderId, order.Status, order.DealAmount, orderSpot)
	}
}

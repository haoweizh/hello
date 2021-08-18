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

func getMarketsGate(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	client := gateApi.NewAPIClient(gateApi.NewConfiguration())
	ctx := context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
		Key: key, Secret: secret})
	marginCurrencyPairs, _, marginErr := client.MarginApi.ListCrossMarginCurrencies(ctx)
	spotCurrencyPairs, _, spotErr := client.SpotApi.ListCurrencyPairs(ctx)
	if marginErr != nil {
		panicGateError(key, "ListCrossMarginCurrencies", marginErr)
	}
	if spotErr != nil {
		panicGateError(key, "ListCurrencyPairs", spotErr)
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
				marketInfo := &model.MarketInfo{}
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
	contracts, _, futureErr := client.FuturesApi.ListFuturesContracts(ctx, "usdt")
	if futureErr != nil {
		panicGateError(key, "ListFuturesContracts", futureErr)
	}
	for _, contract := range contracts {
		if contract.InDelisting {
			continue
		}
		marketInfo := &model.MarketInfo{}
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
	return marketInfos
}

func setPosSideGate(key, secret string) {
	client := gateApi.NewAPIClient(gateApi.NewConfiguration())
	ctx := context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
		Key:    key,
		Secret: secret,
	})
	mode, _, err := client.FuturesApi.SetDualMode(ctx, "usdt", false)
	if err != nil {
		panicGateError(key, "setPosSideGate", err)
	}
	marshal, _ := json.Marshal(mode)
	util.Notice(fmt.Sprintf("set gate dual mode success,position: %s", marshal))
}

func setMarginSettingGate(key, secret string) {
	client := gateApi.NewAPIClient(gateApi.NewConfiguration())
	ctx := context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
		Key:    key,
		Secret: secret,
	})
	mode, _, err := client.MarginApi.SetAutoRepay(ctx, "on")
	if err != nil {
		panicGateError(key, "setMarginSettingGate", err)
	}
	marshal, _ := json.Marshal(mode)
	util.Notice(fmt.Sprintf("set gate margin auto repay success,response: %s", marshal))
}

func transferGate(key string, secret string, transferType string, amount float64) {
	client := gateApi.NewAPIClient(gateApi.NewConfiguration())
	ctx := context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
		Key:    key,
		Secret: secret,
	})
	param := gateApi.Transfer{Currency: "USDT", Amount: fmt.Sprintf("%.6f", amount), Settle: "usdt"}
	if transferType == "MAIN_UMFUTURE" {
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
	} else if transferType == "UMFUTURE_MAIN" {
		param.From = "futures"
		param.To = "spot"
		_, err := client.WalletApi.Transfer(ctx, param)
		if err != nil {
			panicGateError(key, "transferGate", err)
		} else {
			param.From = "spot"
			param.To = "cross_margin"
			_, endErr := client.WalletApi.Transfer(ctx, param)
			if endErr != nil {
				panicGateError(key, "transferGate", endErr)
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

func WsDepthServeGate(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	keys, secrets := model.AppConfig.GetKeys(model.Gate)
	spotWs, spotErr := gateWs.NewWsService(nil, nil, gateWs.NewConnConfFromOption(&gateWs.ConfOptions{
		URL:          gateWs.BaseUrl,
		Key:          keys[0],
		Secret:       model.AppConfig.GateSecret,
		MaxRetryConn: 10,
	}))
	futureWs, futureErr := gateWs.NewWsService(nil, nil, gateWs.NewConnConfFromOption(&gateWs.ConfOptions{
		URL:          gateWs.FuturesUsdtUrl,
		Key:          secrets[0],
		Secret:       model.AppConfig.GateSecret,
		MaxRetryConn: 10,
	}))
	if spotErr != nil {
		util.Notice(fmt.Sprintf("new spot wsService err:%s", spotErr))
		return channels, spotErr
	}
	if futureErr != nil {
		util.Notice(fmt.Sprintf("new future wsService err:%s", futureErr))
		return channels, futureErr
	}
	callSpotBookTicker := gateWs.NewCallBack(func(msg *gateWs.UpdateMsg) {
		var update gateWs.SpotBookTickerMsg
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Notice(fmt.Sprintf("spot book ticker Unmarshal err:%s", err.Error()))
		}
		//responseJson, _ := util.NewJSON(msg.Result)
		//log.Printf("%+v", responseJson)
		//log.Printf("%+v", update)
		if update.CurrencyPair == "" {
			return
		}
		symbol := update.CurrencyPair
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(update.Bid, 64)
		bidAmount, _ := strconv.ParseFloat(update.BidSize, 64)
		askPrice, _ := strconv.ParseFloat(update.Ask, 64)
		askAmount, _ := strconv.ParseFloat(update.AskSize, 64)
		bidAsk := model.BidAsk{Ts: int(update.TimeInMilli), TsReceived: now, UpdateId: update.LastId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount}}}
		haveOld, old := markets.GetBidAsk(symbol, model.Gate)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
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
	callFutureBookTicker := gateWs.NewCallBack(func(msg *gateWs.UpdateMsg) {
		var update FuturesBookTickerModel
		if err := json.Unmarshal(msg.Result, &update); err != nil {
			util.Notice(fmt.Sprintf("future book ticker Unmarshal err:%s", err.Error()))
		}
		//responseJson, _ := util.NewJSON(msg.Result)
		//log.Printf("%+v", responseJson)
		//log.Printf("%+v", update)
		if update.Contract == "" {
			return
		}
		symbol := strings.Split(update.Contract, "_")[0] + model.GetPerpTail(model.Gate)
		now := int(time.Now().UnixNano() / int64(time.Millisecond))
		bidPrice, _ := strconv.ParseFloat(update.BestBidPrice, 64)
		_, bidAmount := model.ParseRealAmount(model.Gate, symbol, float64(update.BestBidSize))
		askPrice, _ := strconv.ParseFloat(update.BestAskPrice, 64)
		_, askAmount := model.ParseRealAmount(model.Gate, symbol, float64(update.BestAskSize))
		bidAsk := model.BidAsk{Ts: int(update.TimeMillis), TsReceived: now, UpdateId: update.LastId,
			Bids: []model.Tick{{Price: bidPrice, Amount: bidAmount}},
			Asks: []model.Tick{{Price: askPrice, Amount: askAmount}}}
		haveOld, old := markets.GetBidAsk(symbol, model.Gate)
		if haveOld && old.UpdateId > bidAsk.UpdateId {
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
	spotSubscribes := make([]string, 0)
	futureSubscribes := make([]string, 0)
	symbols := model.GetMarketSymbols(model.Gate)
	for symbol := range symbols {
		if strings.Contains(symbol, model.GetSpotTail(model.Gate)) {
			spotSubscribes = append(spotSubscribes, symbol)
		} else if strings.Contains(symbol, model.GetPerpTail(model.Gate)) {
			futureSubscribes = append(futureSubscribes, symbol)
		}
	}
	spotWs.SetCallBack(gateWs.ChannelSpotBookTicker, callSpotBookTicker)
	if err := spotWs.Subscribe(gateWs.ChannelSpotBookTicker, spotSubscribes); err != nil {
		util.Notice(fmt.Sprintf("spotWs Subscribe err:%s", err.Error()))
		return channels, err
	}
	futureWs.SetCallBack(gateWs.ChannelFutureBookTicker, callFutureBookTicker)
	if err := futureWs.Subscribe(gateWs.ChannelFutureBookTicker, spotSubscribes); err != nil {
		util.Notice(fmt.Sprintf("futureWs Subscribe err:%s", err.Error()))
		return channels, err
	}

	channels = make([]chan struct{}, 0)
	ch := make(chan struct{}, 10)
	channels = append(channels, ch)
	//defer close(ch)
	//for {
	//	select {
	//	case <-ch:
	//		log.Printf("manual done")
	//	case <-time.After(time.Second * 1000):
	//		log.Printf("auto done")
	//		return
	//	}
	//}
	return channels, err
}

func getBalanceGate(key string, secret string) (success bool, balances []*model.Balance) {
	client := gateApi.NewAPIClient(gateApi.NewConfiguration())
	//client.ChangeBasePath(config.BaseUrl)
	ctx := context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
		Key:    key,
		Secret: secret,
	})
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
		//balance.Borrow = balance.Borrow * -1
		// 此处未计算可以借入的金额
		balance.AvailableWithBorrow, _ = strconv.ParseFloat(item.Available, 64)
		balance.Amount = balance.AvailableWithBorrow + balance.FrozenAmount - balance.Borrow
		priceGet, bidAsk := model.AppMarkets.GetBidAsk(balance.Coin+model.GetSpotTail(model.Gate), model.Gate)
		if priceGet {
			balance.UsdValue = balance.Amount * bidAsk.Bids[0].Price
		}
		balances = append(balances, balance)
	}
	return true, balances
}

func getPositionsGate(key string, secret string) (success bool, positions []*model.Position, posBalance float64) {
	client := gateApi.NewAPIClient(gateApi.NewConfiguration())
	ctx := context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
		Key:    key,
		Secret: secret,
	})
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

func cancelOrdersGate(key string, secret string, symbol string) (result bool) {
	client := gateApi.NewAPIClient(gateApi.NewConfiguration())
	ctx := context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
		Key:    key,
		Secret: secret,
	})
	if strings.Contains(symbol, model.GetSpotTail(model.Gate)) {
		param := &gateApi.CancelOrdersOpts{Account: optional.NewString("cross_margin")}
		orders, _, err := client.SpotApi.CancelOrders(ctx, symbol, param)
		if err != nil {
			panicGateError(key, "cancelSpotOrdersGate", err)
			return false
		}
		marshal, _ := json.Marshal(orders)
		util.SocketInfo(`cancel margin orders response: %s`, marshal)
		return true
	} else {
		symbol = strings.Split(symbol, "_")[0] + model.GetSpotTail(model.Gate)
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
	client := gateApi.NewAPIClient(gateApi.NewConfiguration())
	ctx := context.WithValue(context.Background(), gateApi.ContextGateAPIV4, gateApi.GateAPIV4{
		Key:    key,
		Secret: secret,
	})
	if strings.Contains(symbol, model.GetSpotTail(model.Gate)) {
		marginOrder := gateApi.Order{}
		marginOrder.CurrencyPair = symbol
		marginOrder.Type = orderType
		marginOrder.Account = "cross_margin"
		marginOrder.Side = orderSide
		marginOrder.Amount = util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Gate, symbol, amount)))
		priceSpot, decimalSpot := model.FormatPrice(model.Gate, symbol, model.OrderSideBuy, price)
		marginOrder.Price = util.CutTailZero(strconv.FormatFloat(priceSpot, 'f', decimalSpot, 64))
		if orderSide == model.OrderSideBuy {
			marginOrder.AutoRepay = true
		} else {
			marginOrder.AutoBorrow = true
		}
		createOrder, _, err := client.SpotApi.CreateOrder(ctx, marginOrder)
		if err != nil {
			panicGateError(key, "placeSpotOrderGate", err)
			order.Status = model.CarryStatusFail
			order.OrderId = ``
		} else {
			marshal, _ := json.Marshal(createOrder)
			util.SocketInfo(`create margin order response: %s`, marshal)
			order.OrderId = createOrder.Id
			order.Symbol = createOrder.CurrencyPair
			order.OrderTime, _ = time.Parse(time.RFC3339, createOrder.CreateTime)
			order.Price, _ = strconv.ParseFloat(createOrder.Price, 64)
			order.OrderSide = createOrder.Side
			order.Amount, _ = strconv.ParseFloat(createOrder.Amount, 64)
			order.DealAmount = order.Amount
			order.OrderType = createOrder.Type
			if createOrder.Status == "cancelled" {
				order.Status = model.CarryStatusFail
			} else {
				order.Status = model.CarryStatusWorking
			}
			return
		}
	} else {
		futuresOrder := gateApi.FuturesOrder{}
		futuresOrder.Size, _ = strconv.ParseInt(util.CutTailZero(
			fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Gate, symbol, amount))), 10, 64)
		priceFuture, decimalFuture := model.FormatPrice(model.Gate, symbol, model.OrderSideBuy, price)
		priceStrFuture := util.CutTailZero(strconv.FormatFloat(priceFuture, 'f', decimalFuture, 64))
		futuresOrder.Price = priceStrFuture
		if orderSide == model.OrderSideSell {
			futuresOrder.Size = -1 * futuresOrder.Size
		}
		futuresOrder.Contract = strings.Split(symbol, "_")[0] + model.GetSpotTail(model.Gate)
		createFuturesOrder, _, err := client.FuturesApi.CreateFuturesOrder(ctx, "usdt", futuresOrder)
		if err != nil {
			panicGateError(key, "placeFutureOrderGate", err)
			order.Status = model.CarryStatusFail
			order.OrderId = ``
		} else {
			marshal, _ := json.Marshal(createFuturesOrder)
			util.SocketInfo(`create future order response: %s`, marshal)
			if createFuturesOrder.IsLiq {
				util.Notice(fmt.Sprintf("warning warning, blow up!!!"))
			}
			order.OrderId = strconv.FormatInt(createFuturesOrder.Id, 10)
			order.Symbol = strings.Split(symbol, "_")[0] + model.GetPerpTail(model.Gate)
			order.OrderTime = time.Unix(int64(createFuturesOrder.CreateTime), 0)
			order.Price, _ = strconv.ParseFloat(createFuturesOrder.Price, 64)
			_, order.Amount = model.ParseRealAmount(model.Gate, order.Symbol, float64(createFuturesOrder.Size))
			order.DealAmount = order.Amount
			order.Status = model.CarryStatusWorking
			return
		}
	}
}

func getMaxLoanGate(key string, coin string) (success bool, maxLoan float64) {
	symbol := coin + model.GetSpotTail(model.Gate)
	marketMargin := model.GetMarketInfo(model.Gate+`_`+key, symbol)
	_, tickRelated := model.AppMarkets.GetBidAsk(symbol, model.Gate)
	if tickRelated != nil {
		maxLoan = marketMargin.BorrowUsdtMax / tickRelated.Bids[0].Price
	}
	return true, maxLoan
}

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gateio/gateapi-go/v6"
	gate "github.com/gateio/gatews/go"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"time"
)

func getMarketsGate() (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	client := gateapi.NewAPIClient(gateapi.NewConfiguration())
	//client.ChangeBasePath(config.BaseUrl)
	ctx := context.WithValue(context.Background(), gateapi.ContextGateAPIV4, gateapi.GateAPIV4{
		Key:    model.AppConfig.Gatekey,
		Secret: model.AppConfig.GateSecret,
	})
	marginCurrencyPairs, _, marginErr := client.MarginApi.ListCrossMarginCurrencies(ctx)
	spotCurrencyPairs, _, spotErr := client.SpotApi.ListCurrencyPairs(ctx)
	if marginErr != nil {
		panicGateError(marginErr)
	}
	if spotErr != nil {
		panicGateError(spotErr)
	}
A:
	for _, margin := range marginCurrencyPairs {
		symbol := margin.Name + "_USDT"
		for _, spot := range spotCurrencyPairs {
			if spot.Id == symbol {
				if spot.TradeStatus != "tradable" {
					continue A
				}
				//spotData, _ := json.Marshal(spot)
				//marginData, _ := json.Marshal(margin)
				//util.Notice(fmt.Sprintf("现货交易对：%s", spotData))
				//util.Notice(fmt.Sprintf("杠杆交易对：%s", marginData))
				//util.Notice(fmt.Sprintln())

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

				marketInfos[symbol] = marketInfo
				//todo 最大最小借款数量
			}
		}
	}

	contracts, _, futureErr := client.FuturesApi.ListFuturesContracts(ctx, "usdt")
	if futureErr != nil {
		panicGateError(futureErr)
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
		marketInfo.SizeIncrement = marketInfo.SizeMin
		marketInfo.CTCurrency = coin
		marketInfo.CTValue, _ = strconv.ParseFloat(contract.QuantoMultiplier, 64)

		marketInfos[marketInfo.Name] = marketInfo

		//todo 最大下单数量
	}
	return marketInfos
}

func panicGateError(err error) {
	if e, ok := err.(gateapi.GateAPIError); ok {
		util.SocketInfo(fmt.Sprintf("Gate API error, label: %s, message: %s", e.Label, e.Message))
	}
	util.Notice(err.Error())
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
	spotWs, spotErr := gate.NewWsService(nil, nil, gate.NewConnConfFromOption(&gate.ConfOptions{
		URL:          gate.BaseUrl,
		Key:          model.AppConfig.Gatekey,
		Secret:       model.AppConfig.GateSecret,
		MaxRetryConn: 10,
	}))
	futureWs, futureErr := gate.NewWsService(nil, nil, gate.NewConnConfFromOption(&gate.ConfOptions{
		URL:          gate.FuturesUsdtUrl,
		Key:          model.AppConfig.Gatekey,
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
	callSpotBookTicker := gate.NewCallBack(func(msg *gate.UpdateMsg) {
		var update gate.SpotBookTickerMsg
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
	callFutureBookTicker := gate.NewCallBack(func(msg *gate.UpdateMsg) {
		// parse the message to struct we need
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
		bidAmount := float64(update.BestBidSize)
		askPrice, _ := strconv.ParseFloat(update.BestAskPrice, 64)
		askAmount := float64(update.BestAskSize)
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
	spotWs.SetCallBack(gate.ChannelSpotBookTicker, callSpotBookTicker)
	if err := spotWs.Subscribe(gate.ChannelSpotBookTicker, spotSubscribes); err != nil {
		util.Notice(fmt.Sprintf("spotWs Subscribe err:%s", err.Error()))
		return channels, err
	}
	futureWs.SetCallBack(gate.ChannelFutureBookTicker, callFutureBookTicker)
	if err := futureWs.Subscribe(gate.ChannelFutureBookTicker, spotSubscribes); err != nil {
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

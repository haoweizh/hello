package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"hello/api/dtos"
	"hello/model"
	"hello/util"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const bitgetRestUrl = "https://api.bitget.com"
const bitgetSpotWsUrl = "wss://ws.bitget.com/spot/v1/stream"

var channelMaintainingBitgetSpot = false

func getMarketsBitgetSpot(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/spot/v1/public/products", "", map[string]string{}, 30)
	spotResp := &dtos.BitgetSpotMarketResp{}
	spotJsonErr := json.Unmarshal(httpResp, spotResp)
	if spotResp == nil || spotResp.Code != "00000" {
		util.Notice(fmt.Sprintf("get bitget spot market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, spotJsonErr))
		return
	}
	marketInfos = make(map[string]*model.MarketInfo)
	for _, symbolInfo := range spotResp.Data {
		if symbolInfo.Status != "online" && symbolInfo.QuoteCoin != "USDT" {
			continue
		}
		symbol := symbolInfo.BaseCoin + model.UniStandardTail[model.MarketTypeSpot]
		marketInfo := &model.MarketInfo{Name: symbol}
		priceDecimal, _ := strconv.Atoi(symbolInfo.PriceScale)
		marketInfo.PriceDecimal = priceDecimal
		marketInfo.PriceIncrement = 1 / math.Pow10(priceDecimal)
		amountPrecision, _ := strconv.Atoi(symbolInfo.QuantityScale)
		marketInfo.SizeIncrement = 1 / math.Pow10(amountPrecision)
		marketInfo.SizeMin, _ = strconv.ParseFloat(symbolInfo.MinTradeAmount, 64)
		marketInfo.SizeMax, _ = strconv.ParseFloat(symbolInfo.MaxTradeAmount, 64)
		marketInfo.MoneyMin, _ = strconv.ParseFloat(symbolInfo.MinTradeUSDT, 64)
		marketInfos[marketInfo.Name] = marketInfo
	}
	return marketInfos
}

func WsDepthServeBitgetSpot(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	bookWsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		//util.Notice(fmt.Sprintf("ws data: %s", event))
		if len(event) == 4 {
			return
		}
		bookWsResp := &dtos.BitgetBoosWsResp{}
		jsonErr := json.Unmarshal(event, bookWsResp)
		if jsonErr != nil {
			util.SocketInfo(`bitget fail to unmarshal book ws data json ` + jsonErr.Error())
			return
		}
		if bookWsResp.Arg.InstType == "sp" && bookWsResp.Action == "snapshot" {
			if bookWsResp.Arg.InstId == "" || bookWsResp.Data == nil {
				return
			}
			symbol := bookWsResp.Arg.InstId[0:len(bookWsResp.Arg.InstId)-4] + model.UniStandardTail[model.MarketTypeSpot]
			bidAsk := model.BidAsk{TsReceived: int(time.Now().UnixNano() / int64(time.Millisecond))}
			if len(bookWsResp.Data) > 1 {
				return
			}
			bidPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][0], 64)
			bidAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Bids[0][1], 64)
			bids := make([]model.Tick, 0)
			bids = append(bids, model.Tick{Price: bidPrice, Amount: bidAmount})
			bidAsk.Bids = bids

			askPrice, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][0], 64)
			askAmount, _ := strconv.ParseFloat(bookWsResp.Data[0].Asks[0][1], 64)
			asks := make([]model.Tick, 0)
			asks = append(asks, model.Tick{Price: askPrice, Amount: askAmount})
			bidAsk.Asks = asks

			bidAsk.Ts, _ = strconv.Atoi(bookWsResp.Data[0].Ts)
			bidAsk.UpdateId, _ = strconv.ParseInt(bookWsResp.Data[0].Ts, 10, 64)
			haveOld, old := markets.GetBidAsk(symbol, model.BitgetSpot)
			if haveOld && old.UpdateId > bidAsk.UpdateId {
				return
			}
			if markets.SetBidAsk(symbol, model.BitgetSpot, &bidAsk) {
				funcHandlers := GetFunctions(model.BitgetSpot, symbol)
				if funcHandlers != nil {
					funcHandlers.Range(func(function, value interface{}) bool {
						if model.IgnoreFunctions[function.(string)] {
							return true
						}
						setting := GetSetting(function.(string), model.BitgetSpot, symbol)
						if setting != nil && value != nil {
							go value.(model.CarryHandler)(setting, &bidAsk)
						}
						return true
					})
				}
			}
		}
	}
	channels = make([]chan struct{}, 0)
	spotSubscribes := make([]interface{}, 0)
	symbols := GetMarketSymbols(model.BitgetSpot)
	for symbol, _ := range symbols {
		spotSubscribes = append(spotSubscribes, symbol)
	}
	spotBookChannels, spotBookErr := WebSocketClient(model.BitgetSpot, bitgetSpotWsUrl,
		spotSubscribes, subscribeHandlerBitgetSpotBookTicker, bookWsHandler, orderHandler, 30)
	if spotBookErr == nil {
		util.Notice(`finish connect public Bitget spot book wss `)
		channels = append(channels, spotBookChannels...)
	} else {
		util.Notice(`fail to connect public Bitget spot book wss `)
		return nil, spotBookErr
	}
	go maintainChannelBitgetSpot()
	return channels, nil
}

var subscribeHandlerBitgetSpotBookTicker = func(connection *websocket.Conn, subscribes []interface{}) error {
	var err error = nil
	var params []map[string]string
	for _, subscribe := range subscribes {
		symbol := strings.Split(subscribe.(string), "_")[0]
		params = append(params, map[string]string{"instType": "sp", "channel": "books5", "instId": symbol})
	}
	subscribeMap := make(map[string]interface{})
	subscribeMap["op"] = "subscribe"
	subscribeMap["args"] = params
	subscribeMessage := util.JsonEncodeToByte(subscribeMap)
	if err = SendToConnection(model.BitgetSpot, connection, subscribeMessage); err != nil {
		util.SocketInfo(" bitget can not subscribe %s %s", subscribeMessage, err.Error())
	}
	util.Notice(`bitget subscribed ` + string(subscribeMessage))
	time.Sleep(1200 * time.Millisecond)
	return err
}

func maintainChannelBitgetSpot() {
	if !channelMaintainingBitgetSpot {
		channelMaintainingBitgetSpot = true
		go func() {
			for true {
				time.Sleep(time.Second * 20)
				if err := SendToAllConnections(model.BitgetSpot, []byte(`ping`)); err != nil {
					util.SocketInfo("xt channel ping error " + err.Error())
				}
			}
		}()
	}
}

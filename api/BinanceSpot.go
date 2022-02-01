package api

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/gorilla/websocket"
	"hello/model"
	"hello/util"
	"strconv"
	"strings"
	"time"
)

func getMarketsBinanceSpot(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	client := binance.NewClient(key, secret)
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		util.Notice("getMarketsBinanceSpot err: " + err.Error())
		return marketInfos
	}
	for _, item := range exchangeInfo.Symbols {
		if item.QuoteAsset == "" || item.BaseAsset == "" {
			continue
		}
		haveSpot := false
		if item.Permissions != nil {
			for _, permission := range item.Permissions {
				if permission == `SPOT` && item.IsSpotTradingAllowed {
					haveSpot = true
				}
			}
		}
		if !haveSpot {
			continue
		}
		symbol := item.BaseAsset + model.UniStandardTail[model.MarketTypeSpot]
		marketInfo := &model.MarketInfo{Market: model.BinanceSpot, Name: symbol, MoneyMin: 10}
		for _, data := range item.Filters {
			filterType := data[`filterType`].(string)
			if filterType == `PRICE_FILTER` {
				if data[`tickSize`] != nil {
					marketInfo.PriceIncrement, _ = strconv.ParseFloat(data[`tickSize`].(string), 64)
				}
				marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
			} else if filterType == `LOT_SIZE` {
				if data[`minQty`] != nil {
					marketInfo.SizeMin, _ = strconv.ParseFloat(data[`minQty`].(string), 64)
				}
				if data[`maxQty`] != nil {
					marketInfo.SizeMax, _ = strconv.ParseFloat(data[`maxQty`].(string), 64)
				}
				if data[`stepSize`] != nil {
					marketInfo.SizeIncrement, _ = strconv.ParseFloat(data[`stepSize`].(string), 64)
				}
			}
		}
		marketInfos[marketInfo.Name] = marketInfo
	}
	return marketInfos
}

func WsDepthServeBinanceSpot(markets *model.Markets, orderHandler OrderHandler) (channels []chan struct{}, err error) {
	subType := model.SubscribeTicker
	wsHandler := func(connection *websocket.Conn, event []byte, orderHandler OrderHandler) {
		result, wsErr := util.NewJSON(event)
		if wsErr != nil {
			util.SocketInfo(`binance fail to unmarshal json ` + err.Error())
			return
		}
		subscribe, _ := result.Get("stream").String()
		result = result.Get(`data`)
		//data := new(binance.WsBookTickerEvent)
		//wsErr := json.Unmarshal(event, &data)
		if result == nil {
			return
		}
		dialectSymbol := result.Get(`s`).MustString()
		updateId := result.Get(`u`).MustInt64()
		if dialectSymbol == `` {
			return
		}
		if strings.Contains(subscribe, `@depth`) {
			handleDepthBinance(markets, result, dialectSymbol, updateId)
		} else if strings.Contains(subscribe, `@bookTicker`) {
			handleTickerBinance(markets, result, dialectSymbol, updateId)
		}
	}
	channels = make([]chan struct{}, 0)
	spotSubs := GetWSSubscribes(model.BinanceSpot, subType)
	spotChans, spotErr := WebSocketClient(model.BinanceSpot, wsBinance, spotSubs,
		subscribeHandlerBinance, wsHandler, orderHandler, wsStepBinance)
	if spotErr != nil {
		util.SocketInfo(`fail to create binance spot conn %s`, spotErr.Error())
	}
	return spotChans, err
}

func maintainChannelBinanceSpot() {
	if !channelMaintainingBinance {
		channelMaintainingBinance = true
		for true {
			time.Sleep(time.Minute * 5)
			ts := time.Now().UnixNano() / int64(time.Millisecond)
			pong := []byte(fmt.Sprintf(`{"method":"PONG","E":%d}`, ts))
			err := SendToAllConnections(model.BinanceSpot, pong)
			if err != nil {
				util.SocketInfo("pong binance server error " + err.Error())
			}
		}
	}
}

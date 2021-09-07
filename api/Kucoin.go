package api

import (
	"fmt"
	"github.com/Kucoin/kucoin-go-sdk"
	"github.com/Kucoin/kumex-go-sdk"
	"hello/model"
	"hello/util"
	"log"
	"strconv"
	"strings"
)

func kucoinFutureClient(key, secret, passPhrase string) *kumex.ApiService {
	if key == "" || secret == "" || passPhrase == "" {
		keys, secrets := model.AppConfig.GetKeys(model.Gate)
		key = keys[0]
		secret = secrets[0]
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
		keys, secrets := model.AppConfig.GetKeys(model.Gate)
		key = keys[0]
		secret = secrets[0]
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
	client := kucoinRelatedClient("", "", "")
	rsp, err := client.WebSocketPublicToken()
	if err != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket error:%s", "WsDepthServeKucoin", err))
	}
	token := &kucoin.WebSocketTokenModel{}
	if tokenErr := rsp.ReadData(token); tokenErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket error:%s", "WsDepthServeKucoin", tokenErr))
	}
	channel := client.NewWebSocketClient(token)
	mc, channelError, connectErr := channel.Connect()
	if connectErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket connect error:%s", "WsDepthServeKucoin", connectErr))
	}
	symbols := model.GetMarketSymbols(model.Kucoin)
	futureSubscribes := make([]*kucoin.WebSocketSubscribeMessage, 0)
	for symbol := range symbols {
		topic := "/contractMarket/tickerV2:" + strings.Split(symbol, "-")[0] + "USDTM"
		futureSubscribes = append(futureSubscribes, kucoin.NewSubscribeMessage(topic, false))
	}
	relatedSubscribe := kucoin.NewSubscribeMessage("/market/ticker:all", false)
	futureSubscribes = append(futureSubscribes, relatedSubscribe)
	if subscribeErr := channel.Subscribe(futureSubscribes...); subscribeErr != nil {
		util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket subscribe error:%s", "WsDepthServeKucoin", subscribeErr))
		return
	}
	for {
		select {
		case cError := <-channelError:
			//channel.Stop()
			util.SocketInfo(fmt.Sprintf("function: %s kucoin related websocket channel error:%s", "WsDepthServeKucoin", cError.Error()))
			return
		case msg := <-mc:
			msg.Subject
		}
	}
}

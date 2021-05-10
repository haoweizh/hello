package model

import "sync"

var marketInfos = make(map[string]map[string]*MarketInfo) // market - symbol - MarketInfo
var marketInfoLock sync.Mutex

type MarketInfo struct {
	Name, CTCurrency                                string
	SizeMin, SizeIncrement, PriceIncrement, CTValue float64
	PriceDecimal                                    int // 价格精确到小数点后几位
	//CanBorrow                                       bool
	//"ask":2.2966,
	//"baseCurrency":null,
	//"bid":2.2943,
	//"change1h":0.010924147652189235,
	//"change24h":-0.08310027966440271,
	//"changeBod":-0.01557071162012611,
	//"enabled":true,
	//"highLeverageFeeExempt":false,
	//"last":2.295,
	//"minProvideSize":1,
	//"name":"1INCH-PERP",
	//"postOnly":false,
	//"price":2.295,
	//"priceIncrement":0.0001,
	//"quoteCurrency":null,
	//"quoteVolume24h":19693372.9568,
	//"restricted":false,
	//"sizeIncrement":1,
	//"type":"future",
	//"underlying":"1INCH",
	//"volumeUsd24h":19693372.9568
}

func GetMarketInfo(market, instrument string) (marketInfo *MarketInfo) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	if marketInfos == nil || marketInfos[market] == nil {
		return nil
	}
	return marketInfos[market][instrument]
}

//func SetMarketInfo(market, instrument string, marketInfo *MarketInfo) {
//	defer marketInfoLock.Unlock()
//	marketInfoLock.Lock()
//	if marketInfos[market] == nil {
//		marketInfos[market] = make(map[string]*MarketInfo)
//	}
//	marketInfos[market][instrument] = marketInfo
//}

func SetMarketInfos(market string, value map[string]*MarketInfo) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	marketInfos[market] = value
}

func GetMarketInfos(market string) (value map[string]*MarketInfo) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	return marketInfos[market]
}

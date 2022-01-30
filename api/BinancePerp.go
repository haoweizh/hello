package api

//
//import (
//	"context"
//	"github.com/adshao/go-binance/v2"
//	"hello/model"
//	"hello/util"
//	"strconv"
//)
//
//func getMarketsBinancePerp(key, secret string) (marketInfos map[string]*model.MarketInfo) {
//	marketInfos = make(map[string]*model.MarketInfo)
//	client := binance.NewFuturesClient(key, secret)
//	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
//	if err != nil {
//		util.Notice("getMarketsBinanceSpot err: " + err.Error())
//		return marketInfos
//	}
//	for _, item := range exchangeInfo.Symbols {
//		if item.QuoteAsset == "" || item.BaseAsset == "" {
//			continue
//		}
//		haveSpot := false
//		if item.Permissions != nil {
//			for _, permission := range item.Permissions {
//				if permission == `SPOT` && item.IsSpotTradingAllowed {
//					haveSpot = true
//				}
//			}
//		}
//		if !haveSpot {
//			continue
//		}
//		symbol := item.BaseAsset + model.UniStandardTail[model.MarketTypeSpot]
//		marketInfo := &model.MarketInfo{Market: model.BinanceSpot, Name: symbol, MoneyMin: 10}
//		for _, data := range item.Filters {
//			filterType := data[`filterType`].(string)
//			if filterType == `PRICE_FILTER` {
//				if data[`tickSize`] != nil {
//					marketInfo.PriceIncrement, _ = strconv.ParseFloat(data[`tickSize`].(string), 64)
//				}
//				marketInfo.PriceDecimal = util.NumDecPlaces(marketInfo.PriceIncrement)
//			} else if filterType == `LOT_SIZE` {
//				if data[`minQty`] != nil {
//					marketInfo.SizeMin, _ = strconv.ParseFloat(data[`minQty`].(string), 64)
//				}
//				if data[`maxQty`] != nil {
//					marketInfo.SizeMax, _ = strconv.ParseFloat(data[`maxQty`].(string), 64)
//				}
//				if data[`stepSize`] != nil {
//					marketInfo.SizeIncrement, _ = strconv.ParseFloat(data[`stepSize`].(string), 64)
//				}
//			}
//		}
//		marketInfos[marketInfo.Name] = marketInfo
//	}
//	return marketInfos
//}

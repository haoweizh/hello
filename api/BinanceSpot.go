package api

import (
	"context"
	"github.com/adshao/go-binance/v2"
	"hello/model"
	"hello/util"
	"strconv"
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

//func getMarketsBinance(key, secret string) (marketInfos map[string]*model.MarketInfo) {
//	marketInfos = make(map[string]*model.MarketInfo)
//	requestUrls := []string{restBinance + `/api/v3/exchangeInfo`, restBinanceFuture + `/fapi/v1/exchangeInfo`}
//	for _, requestUrl := range requestUrls {
//		responseBody := signedRequestBinance(key, secret, http.MethodGet, requestUrl, false, nil)
//		resultJson, err := util.NewJSON(responseBody)
//		if err == nil && resultJson.Get(`symbols`) != nil {
//			data := resultJson.Get(`symbols`).MustArray()
//			for _, item := range data {
//				value := item.(map[string]interface{})
//				if value[`quoteAsset`] == nil || value[`baseAsset`] == nil {
//					continue
//				}
//				var symbol string
//				if value[`contractType`] == nil {
//					haveSpot := false
//					if value[`permissions`] != nil {
//						permissions := value[`permissions`].([]interface{})
//						for _, permission := range permissions {
//							if permission.(string) == `SPOT` {
//								haveSpot = true
//							}
//						}
//					}
//					if !haveSpot {
//						continue
//					}
//					//symbol = value[`baseAsset`].(string) + value[`quoteAsset`].(string)
//					symbol = value[`baseAsset`].(string) + model.UniStandardTail[model.MarketTypeSpot]
//				} else if value[`contractType`] != nil && value[`contractType`].(string) == `PERPETUAL` {
//					symbol = value[`baseAsset`].(string) + model.UniStandardTail[model.MarketTypePerp]
//				} else {
//					continue
//				}
//				marketInfo := &model.MarketInfo{Market: model.Binance, Name: symbol,
//					CTCurrency: value[`baseAsset`].(string), MoneyMin: 10}
//				setMarketInfoFilters(marketInfo, value[`filters`].([]interface{}))
//				marketInfos[marketInfo.Name] = marketInfo
//			}
//		}
//	}
//	return marketInfos
//}

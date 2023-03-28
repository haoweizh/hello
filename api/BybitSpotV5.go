package api

import (
	"encoding/json"
	"fmt"
	"hello/api/dtos"
	"hello/model"
	"hello/util"
	"net/http"
	"strconv"
)

const bybitRestUrl = "https://api.bybit.com"
const bybitSpotPubWsUrl = "wss://stream.bybit.com/v5/public/spot"

var channelMaintainingBybitSpot = false

func getMarketsBybitSpot() (marketInfos map[string]*model.MarketInfo) {
	param := map[string]interface{}{"category": "spot", "limit": "1000"}
	composeParams := util.ComposeParams(param)
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bybitRestUrl+"/v5/market/instruments-info?"+composeParams,
		"", map[string]string{}, 30)
	spotResp := &dtos.BybitSpotMarketResp{}
	spotJsonErr := json.Unmarshal(httpResp, spotResp)
	if spotResp == nil || spotResp.RetCode != 0 {
		util.Notice(fmt.Sprintf("get bybit spot market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, spotJsonErr))
		return
	}
	for _, symbolInfo := range spotResp.Result.List {
		if symbolInfo.Status != "Trading" && symbolInfo.QuoteCoin != "USDT" {
			continue
		}
		symbol := symbolInfo.BaseCoin + model.GetSpotTail(model.Bybit)
		marketInfo := &model.MarketInfo{Name: symbol}
		if symbolInfo.PriceFilter.TickSize == "" {
			util.Notice(fmt.Sprintf("币种：%s 价格步长为空 resp：%v", symbol, symbolInfo))
			continue
		}
		priceIncrement, _ := strconv.ParseFloat(symbolInfo.PriceFilter.TickSize, 64)
		marketInfo.PriceIncrement = priceIncrement
		marketInfo.PriceDecimal = util.NumDecPlaces(priceIncrement)

		sizeIncrement, _ := strconv.ParseFloat(symbolInfo.LotSizeFilter.BasePrecision, 64)
		marketInfo.SizeIncrement = sizeIncrement
		marketInfo.SizeMin, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MinOrderQty, 64)
		marketInfo.SizeMax, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MaxOrderQty, 64)
		marketInfo.UsdtMin, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MinOrderAmt, 64)
		marketInfo.UsdtMax, _ = strconv.ParseFloat(symbolInfo.LotSizeFilter.MaxOrderAmt, 64)
		marketInfos[marketInfo.Name] = marketInfo
	}
}

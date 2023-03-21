package api

import (
	"encoding/json"
	"fmt"
	"hello/api/dtos"
	"hello/model"
	"hello/util"
	"math"
	"net/http"
	"strconv"
)

func getMarketsBitgetPerp(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	httpResp, httpErr := util.HttpRequest(http.MethodGet, bitgetRestUrl+"/api/mix/v1/market/contracts?productType=umcbl", "", map[string]string{}, 30)
	perpResp := &dtos.BitgetPerpMarketResp{}
	perpJsonErr := json.Unmarshal(httpResp, perpResp)
	if perpResp == nil || perpResp.Code != "00000" {
		util.Notice(fmt.Sprintf("get bitget perp market error, resp: %s, httpErr: %v, jsonErr: %v", httpResp, httpErr, perpJsonErr))
		return
	}
	marketInfos = make(map[string]*model.MarketInfo)
	for _, perpInfo := range perpResp.Data {
		if perpInfo.QuoteCoin != "USDT" || perpInfo.SymbolStatus != "normal" || perpInfo.SymbolType != "perpetual" ||
			perpInfo.OffTime != "-1" || perpInfo.LimitOpenTime != "-1" {
			continue
		}
		symbol := perpInfo.BaseCoin + model.UniStandardTail[model.MarketTypePerp]
		marketInfo := &model.MarketInfo{Name: symbol, CTCurrency: perpInfo.BaseCoin}
		marketInfo.PriceDecimal, _ = strconv.Atoi(perpInfo.PricePlace)
		priceEndStep, _ := strconv.ParseFloat(perpInfo.PriceEndStep, 64)
		marketInfo.PriceIncrement = priceEndStep * (1 / math.Pow10(marketInfo.PriceDecimal))
		marketInfo.SizeMin, _ = strconv.ParseFloat(perpInfo.MinTradeNum, 64)
		marketInfo.SizeIncrement, _ = strconv.ParseFloat(perpInfo.SizeMultiplier, 64)
		//marketInfo.CTValue, _ = strconv.ParseFloat(perpInfo.ContractSize, 64)
		marketInfo.BuyLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.BuyLimitPriceRatio, 64)
		marketInfo.SellLimitPriceRatio, _ = strconv.ParseFloat(perpInfo.SellLimitPriceRatio, 64)
		marketInfos[symbol] = marketInfo
	}
	return marketInfos
}

func setBitgetPositionMode(key, secret string) {
	client := dtos.BitgetRestClient{BaseUrl: bitgetRestUrl, Passphrase: model.AppConfig.Phase, ApiKey: key, ApiSecretKey: secret}
	params := map[string]string{"productType": "umcbl", "holdMode": "single_hold"}
	httpResp, httpErr := client.DoPost("/api/mix/v1/account/setPositionMode", string(util.JsonEncodeToByte(params)))
	jsonData, jsonErr := util.NewJSON(httpResp)
	code, _ := jsonData.Get("code").String()
	if jsonData == nil || code != "00000" {
		util.SocketInfo(fmt.Sprintf("fail to set Bitget Position Mode, resp: %s httpErr: %v, jsonErr: %v", httpResp, httpErr, jsonErr))
	}
}

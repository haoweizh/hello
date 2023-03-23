package dtos

type BitgetSpotMarketResp struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		Symbol         string `json:"symbol"`
		SymbolName     string `json:"symbolName"`
		BaseCoin       string `json:"baseCoin"`
		QuoteCoin      string `json:"quoteCoin"`
		MinTradeAmount string `json:"minTradeAmount"`
		MaxTradeAmount string `json:"maxTradeAmount"`
		TakerFeeRate   string `json:"takerFeeRate"`
		MakerFeeRate   string `json:"makerFeeRate"`
		PriceScale     string `json:"priceScale"`
		QuantityScale  string `json:"quantityScale"`
		MinTradeUSDT   string `json:"minTradeUSDT"`
		Status         string `json:"status"`
	} `json:"data"`
}

type BitgetPerpMarketResp struct {
	Code string `json:"code"`
	Data []struct {
		BaseCoin            string   `json:"baseCoin"`
		BuyLimitPriceRatio  string   `json:"buyLimitPriceRatio"`
		FeeRateUpRatio      string   `json:"feeRateUpRatio"`
		MakerFeeRate        string   `json:"makerFeeRate"`
		MinTradeNum         string   `json:"minTradeNum"`
		OpenCostUpRatio     string   `json:"openCostUpRatio"`
		PriceEndStep        string   `json:"priceEndStep"`
		PricePlace          string   `json:"pricePlace"`
		QuoteCoin           string   `json:"quoteCoin"`
		SellLimitPriceRatio string   `json:"sellLimitPriceRatio"`
		SizeMultiplier      string   `json:"sizeMultiplier"`
		SupportMarginCoins  []string `json:"supportMarginCoins"`
		Symbol              string   `json:"symbol"`
		TakerFeeRate        string   `json:"takerFeeRate"`
		VolumePlace         string   `json:"volumePlace"`
		SymbolType          string   `json:"symbolType"`
		SymbolStatus        string   `json:"symbolStatus"`
		OffTime             string   `json:"offTime"`
		LimitOpenTime       string   `json:"limitOpenTime"`
	} `json:"data"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
}

type BitgetBoosWsResp struct {
	Action string `json:"action"`
	Arg    struct {
		InstType string `json:"instType"`
		Channel  string `json:"channel"`
		InstId   string `json:"instId"`
	} `json:"arg"`
	Data []struct {
		Asks [][]string `json:"asks"`
		Bids [][]string `json:"bids"`
		Ts   string     `json:"ts"`
	} `json:"data"`
}

type BitgetTickerWsResp struct {
	Action string `json:"action"`
	Arg    struct {
		InstType string `json:"instType"`
		Channel  string `json:"channel"`
		InstId   string `json:"instId"`
	} `json:"arg"`
	Data []struct {
		InstId             string `json:"instId"`
		Last               string `json:"last"`
		BestAsk            string `json:"bestAsk"`
		BestBid            string `json:"bestBid"`
		High24H            string `json:"high24h"`
		Low24H             string `json:"low24h"`
		PriceChangePercent string `json:"priceChangePercent"`
		CapitalRate        string `json:"capitalRate"`
		NextSettleTime     int64  `json:"nextSettleTime"`
		SystemTime         int64  `json:"systemTime"`
		MarkPrice          string `json:"markPrice"`
		IndexPrice         string `json:"indexPrice"`
		Holding            string `json:"holding"`
		BaseVolume         string `json:"baseVolume"`
		QuoteVolume        string `json:"quoteVolume"`
		OpenUtc            string `json:"openUtc"`
		ChgUTC             string `json:"chgUTC"`
		SymbolType         int    `json:"symbolType"`
		SymbolId           string `json:"symbolId"`
		DeliveryPrice      string `json:"deliveryPrice"`
		BidSz              string `json:"bidSz"`
		AskSz              string `json:"askSz"`
	} `json:"data"`
}

type BitgetBalanceResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		CoinId    string `json:"coinId"`
		CoinName  string `json:"coinName"`
		Available string `json:"available"`
		Frozen    string `json:"frozen"`
		Lock      string `json:"lock"`
		UTime     string `json:"uTime"`
	} `json:"data"`
}

type BitgetAssertResp struct {
	Code string `json:"code"`
	Data []struct {
		MarginCoin        string `json:"marginCoin"`
		Locked            string `json:"locked"`
		Available         string `json:"available"`
		CrossMaxAvailable string `json:"crossMaxAvailable"`
		FixedMaxAvailable string `json:"fixedMaxAvailable"`
		MaxTransferOut    string `json:"maxTransferOut"`
		Equity            string `json:"equity"`
		UsdtEquity        string `json:"usdtEquity"`
		BtcEquity         string `json:"btcEquity"`
	} `json:"data"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
}

type BitgetPositionResp struct {
	Code string `json:"code"`
	Data []struct {
		MarginCoin        string `json:"marginCoin"`
		Symbol            string `json:"symbol"`
		HoldSide          string `json:"holdSide"`
		OpenDelegateCount string `json:"openDelegateCount"`
		Margin            string `json:"margin"`
		Available         string `json:"available"`
		Locked            string `json:"locked"`
		Total             string `json:"total"`
		Leverage          int    `json:"leverage"`
		AchievedProfits   string `json:"achievedProfits"`
		AverageOpenPrice  string `json:"averageOpenPrice"`
		MarginMode        string `json:"marginMode"`
		HoldMode          string `json:"holdMode"`
		UnrealizedPL      string `json:"unrealizedPL"`
		KeepMarginRate    string `json:"keepMarginRate"`
		MarketPrice       string `json:"marketPrice"`
		CTime             string `json:"cTime"`
	} `json:"data"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
}

type BitgetOrderResp struct {
	Code string `json:"code"`
	Data struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	} `json:"data"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
}

type BitgetSpotOpenOrderResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		AccountId       string `json:"accountId"`
		Symbol          string `json:"symbol"`
		OrderId         string `json:"orderId"`
		ClientOrderId   string `json:"clientOrderId"`
		Price           string `json:"price"`
		Quantity        string `json:"quantity"`
		OrderType       string `json:"orderType"`
		Side            string `json:"side"`
		Status          string `json:"status"`
		FillPrice       string `json:"fillPrice"`
		FillQuantity    string `json:"fillQuantity"`
		FillTotalAmount string `json:"fillTotalAmount"`
		CTime           string `json:"cTime"`
	} `json:"data"`
}

type BitgetPerpOpenOrderResp struct {
	Code        string `json:"code"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
	Data        []struct {
		Symbol       string  `json:"symbol"`
		Size         float64 `json:"size"`
		OrderId      string  `json:"orderId"`
		ClientOid    string  `json:"clientOid"`
		FilledQty    float64 `json:"filledQty"`
		Fee          float64 `json:"fee"`
		Price        float64 `json:"price"`
		State        string  `json:"state"`
		Side         string  `json:"side"`
		TimeInForce  string  `json:"timeInForce"`
		TotalProfits float64 `json:"totalProfits"`
		PosSide      string  `json:"posSide"`
		MarginCoin   string  `json:"marginCoin"`
		FilledAmount float64 `json:"filledAmount"`
		OrderType    string  `json:"orderType"`
		Leverage     string  `json:"leverage"`
		MarginMode   string  `json:"marginMode"`
		ReduceOnly   bool    `json:"reduceOnly"`
		CTime        string  `json:"cTime"`
		UTime        string  `json:"uTime"`
	} `json:"data"`
}

type BitgetFundingResp struct {
	Code string `json:"code"`
	Data struct {
		Symbol      string `json:"symbol"`
		FundingRate string `json:"fundingRate"`
	} `json:"data"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
}

type BitgetPerpOrderDetailResp struct {
	Code        string `json:"code"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
	Data        struct {
		Symbol       string  `json:"symbol"`
		Size         float64 `json:"size"`
		OrderId      string  `json:"orderId"`
		ClientOid    string  `json:"clientOid"`
		FilledQty    float64 `json:"filledQty"`
		Fee          float64 `json:"fee"`
		Price        float64 `json:"price"`
		PriceAvg     float64 `json:"priceAvg"`
		State        string  `json:"state"`
		Side         string  `json:"side"`
		TimeInForce  string  `json:"timeInForce"`
		TotalProfits float64 `json:"totalProfits"`
		PosSide      string  `json:"posSide"`
		MarginCoin   string  `json:"marginCoin"`
		FilledAmount float64 `json:"filledAmount"`
		OrderType    string  `json:"orderType"`
		Leverage     string  `json:"leverage"`
		MarginMode   string  `json:"marginMode"`
		ReduceOnly   bool    `json:"reduceOnly"`
		CTime        string  `json:"cTime"`
		UTime        string  `json:"uTime"`
	} `json:"data"`
}

type BitgetSpotOrderDetailResp struct {
	Code        string `json:"code"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
	Data        []struct {
		AccountId       string `json:"accountId"`
		Symbol          string `json:"symbol"`
		OrderId         string `json:"orderId"`
		ClientOrderId   string `json:"clientOrderId"`
		Price           string `json:"price"`
		Quantity        string `json:"quantity"`
		OrderType       string `json:"orderType"`
		Side            string `json:"side"`
		Status          string `json:"status"`
		FillPrice       string `json:"fillPrice"`
		FillQuantity    string `json:"fillQuantity"`
		FillTotalAmount string `json:"fillTotalAmount"`
		CTime           string `json:"cTime"`
	} `json:"data"`
}

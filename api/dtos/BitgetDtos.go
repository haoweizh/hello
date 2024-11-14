package dtos

type BitgetSpotMarketResp struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		Symbol              string `json:"symbol"`
		BaseCoin            string `json:"baseCoin"`
		QuoteCoin           string `json:"quoteCoin"`
		MinTradeAmount      string `json:"minTradeAmount"`
		MaxTradeAmount      string `json:"maxTradeAmount"`
		TakerFeeRate        string `json:"takerFeeRate"`
		MakerFeeRate        string `json:"makerFeeRate"`
		MinTradeUSDT        string `json:"minTradeUSDT"`
		Status              string `json:"status"`
		PricePrecision      string `json:"pricePrecision"`
		QuantityPrecision   string `json:"quantityPrecision"`
		QuotePrecision      string `json:"quotePrecision"`
		BuyLimitPriceRatio  string `json:"buyLimitPriceRatio"`
		SellLimitPriceRatio string `json:"sellLimitPriceRatio"`
		OrderQuantity       string `json:"orderQuantity"`
	} `json:"data"`
}

type BitgetPerpMarketResp struct {
	Code string `json:"code"`
	Data []struct {
		Symbol              string   `json:"symbol"`
		BaseCoin            string   `json:"baseCoin"`
		QuoteCoin           string   `json:"quoteCoin"`
		BuyLimitPriceRatio  string   `json:"buyLimitPriceRatio"`
		SellLimitPriceRatio string   `json:"sellLimitPriceRatio"`
		FeeRateUpRatio      string   `json:"feeRateUpRatio"`
		MakerFeeRate        string   `json:"makerFeeRate"`
		TakerFeeRate        string   `json:"takerFeeRate"`
		OpenCostUpRatio     string   `json:"openCostUpRatio"`
		SupportMarginCoins  []string `json:"supportMarginCoins"`
		MinTradeNum         string   `json:"minTradeNum"`
		PriceEndStep        string   `json:"priceEndStep"`
		VolumePlace         string   `json:"volumePlace"`
		PricePlace          string   `json:"pricePlace"`
		SizeMultiplier      string   `json:"sizeMultiplier"`
		SymbolType          string   `json:"symbolType"`
		SymbolStatus        string   `json:"symbolStatus"`
		MinTradeUSDT        string   `json:"minTradeUSDT"`
		MaxSymbolOrderNum   string   `json:"maxSymbolOrderNum"`
		MaxProductOrderNum  string   `json:"maxProductOrderNum"`
		MaxPositionNum      string   `json:"maxPositionNum"`
		DeliveryTime        string   `json:"deliveryTime"`
		DeliveryStartTime   string   `json:"deliveryStartTime"`
		LaunchTime          string   `json:"launchTime"`
		FundInterval        string   `json:"fundInterval"`
		MinLever            string   `json:"minLever"`
		MaxLever            string   `json:"maxLever"`
		PosLimit            string   `json:"posLimit"`
		MaintainTime        string   `json:"maintainTime"`
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
		InstId          string `json:"instId"`
		LastPr          string `json:"lastPr"`
		AskPr           string `json:"askPr"`
		BidPr           string `json:"bidPr"`
		BidSz           string `json:"bidSz"`
		AskSz           string `json:"askSz"`
		Open24H         string `json:"open24h"`
		High24H         string `json:"high24h"`
		Low24H          string `json:"low24h"`
		Change24h       string `json:"change24h"`
		FundingRate     string `json:"fundingRate"`
		NextFundingTime string `json:"nextFundingTime"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		HoldingAmount   string `json:"holdingAmount"`
		BaseVolume      string `json:"baseVolume"`
		QuoteVolume     string `json:"quoteVolume"`
		OpenUtc         string `json:"openUtc"`
		SymbolType      string `json:"symbolType"`
		Symbol          string `json:"symbol"`
		DeliveryPrice   string `json:"deliveryPrice"`
		Ts              string `json:"ts"`
	} `json:"data"`
}

type BitgetBalanceResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		CoinName  string `json:"coin"`
		Available string `json:"available"`
		Frozen    string `json:"frozen"`
		Lock      string `json:"locked"`
		UTime     string `json:"uTime"`
	} `json:"data"`
}

type BitgetAssertResp struct {
	Code string `json:"code"`
	Data []struct {
		MarginCoin           string `json:"marginCoin"`
		Locked               string `json:"locked"`
		Available            string `json:"available"`
		CrossMaxAvailable    string `json:"crossMaxAvailable"`
		IsolatedMaxAvailable string `json:"isolatedMaxAvailable"`
		AccountEquity        string `json:"accountEquity"`
		CrossedRiskRate      string `json:"crossedRiskRate"`
		UnrealizedPL         string `json:"unrealizedPL"`
		Coupon               string `json:"coupon"`
		UnionTotalMagin      string `json:"unionTotalMagin"`
		UnionAvailable       string `json:"unionAvailable"`
		UnionMm              string `json:"unionMm"`
		MaxTransferOut       string `json:"maxTransferOut"`
		UsdtEquity           string `json:"usdtEquity"`
		BtcEquity            string `json:"btcEquity"`
		AssetList            []struct {
			Coin    string `json:"coin"`
			Balance string `json:"balance"`
		} `json:"assetList"`
	} `json:"data"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
}

type BitgetPositionResp struct {
	Code string `json:"code"`
	Data []struct {
		MarginCoin       string `json:"marginCoin"`
		Symbol           string `json:"symbol"`
		HoldSide         string `json:"holdSide"`
		OpenDelegateSize string `json:"openDelegateSize"`
		MarginSize       string `json:"marginSize"`
		OpenPriceAvg     string `json:"openPriceAvg"`
		PosMode          string `json:"posMode"`
		LiquidationPrice string `json:"liquidationPrice"`
		MarkPrice        string `json:"markPrice"`
		BreakEvenPrice   string `json:"breakEvenPrice"`
		TotalFee         string `json:"totalFee"`
		DeductedFee      string `json:"deductedFee"`
		MarginRatio      string `json:"marginRatio"`
		AssetMode        string `json:"assetMode"`
		UTime            string `json:"uTime"`
		Available        string `json:"available"`
		Locked           string `json:"locked"`
		Total            string `json:"total"`
		Leverage         int    `json:"leverage"`
		AchievedProfits  string `json:"achievedProfits"`
		MarginMode       string `json:"marginMode"`
		UnrealizedPL     string `json:"unrealizedPL"`
		KeepMarginRate   string `json:"keepMarginRate"`
		CTime            string `json:"cTime"`
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
		UserId     string `json:"userId"`
		Symbol     string `json:"symbol"`
		OrderId    string `json:"orderId"`
		ClientOid  string `json:"clientOid"`
		Price      string `json:"price"`
		Size       string `json:"size"`
		OrderType  string `json:"orderType"`
		Side       string `json:"side"`
		Status     string `json:"status"`
		PriceAvg   string `json:"priceAvg"`
		BaseVolume string `json:"baseVolume"`
		CTime      string `json:"cTime"`
	} `json:"data"`
}

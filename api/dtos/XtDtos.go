package dtos

type SpotMarketResp struct {
	Rc     int           `json:"rc"`
	Mc     string        `json:"mc"`
	Ma     []interface{} `json:"ma"`
	Result struct {
		Time    int64  `json:"time"`
		Version string `json:"version"`
		Symbols []struct {
			Id                     int           `json:"id"`
			Symbol                 string        `json:"symbol"`
			State                  string        `json:"state"`
			TradingEnabled         bool          `json:"tradingEnabled"`
			OpenapiEnabled         bool          `json:"openapiEnabled"`
			NextStateTime          interface{}   `json:"nextStateTime"`
			NextState              interface{}   `json:"nextState"`
			DepthMergePrecision    int           `json:"depthMergePrecision"`
			BaseCurrency           string        `json:"baseCurrency"`
			BaseCurrencyPrecision  int           `json:"baseCurrencyPrecision"`
			BaseCurrencyId         int           `json:"baseCurrencyId"`
			QuoteCurrency          string        `json:"quoteCurrency"`
			QuoteCurrencyPrecision int           `json:"quoteCurrencyPrecision"`
			QuoteCurrencyId        int           `json:"quoteCurrencyId"`
			PricePrecision         int           `json:"pricePrecision"`
			QuantityPrecision      int           `json:"quantityPrecision"`
			OrderTypes             []string      `json:"orderTypes"`
			TimeInForces           []string      `json:"timeInForces"`
			DisplayWeight          int           `json:"displayWeight"`
			DisplayLevel           string        `json:"displayLevel"`
			Plates                 []interface{} `json:"plates"`
			Filters                []struct {
				Filter           string `json:"filter"`
				BuyMaxDeviation  string `json:"buyMaxDeviation,omitempty"`
				SellMaxDeviation string `json:"sellMaxDeviation,omitempty"`
				MaxDeviation     string `json:"maxDeviation,omitempty"`
				DurationSeconds  string `json:"durationSeconds,omitempty"`
				MaxPriceMultiple string `json:"maxPriceMultiple,omitempty"`
				Min              string `json:"min"`
				Max              string `json:"max"`
				TickSize         string `json:"tickSize"`
			} `json:"filters"`
		} `json:"symbols"`
	} `json:"result"`
}

type PerpMarketResp struct {
	Error struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	} `json:"error"`
	MsgInfo string `json:"msgInfo"`
	Result  []struct {
		Id                        int         `json:"id"`
		SymbolGroupId             int         `json:"symbolGroupId"`
		Symbol                    string      `json:"symbol"`
		Pair                      string      `json:"pair"`
		ContractType              string      `json:"contractType"`
		ProductType               string      `json:"productType"`
		PredictEventType          interface{} `json:"predictEventType"`
		UnderlyingType            string      `json:"underlyingType"`
		ContractSize              string      `json:"contractSize"`
		TradeSwitch               bool        `json:"tradeSwitch"`
		IsDisplay                 bool        `json:"isDisplay"`
		IsOpenApi                 bool        `json:"isOpenApi"`
		State                     int         `json:"state"`
		InitLeverage              int         `json:"initLeverage"`
		InitPositionType          string      `json:"initPositionType"`
		BaseCoin                  string      `json:"baseCoin"`
		QuoteCoin                 string      `json:"quoteCoin"`
		BaseCoinPrecision         int         `json:"baseCoinPrecision"`
		BaseCoinDisplayPrecision  int         `json:"baseCoinDisplayPrecision"`
		QuoteCoinPrecision        int         `json:"quoteCoinPrecision"`
		QuoteCoinDisplayPrecision int         `json:"quoteCoinDisplayPrecision"`
		QuantityPrecision         int         `json:"quantityPrecision"`
		PricePrecision            int         `json:"pricePrecision"`
		SupportOrderType          string      `json:"supportOrderType"`
		SupportTimeInForce        string      `json:"supportTimeInForce"`
		SupportEntrustType        string      `json:"supportEntrustType"`
		SupportPositionType       string      `json:"supportPositionType"`
		MinQty                    string      `json:"minQty"`
		MinNotional               string      `json:"minNotional"`
		MaxNotional               string      `json:"maxNotional"`
		MultiplierDown            string      `json:"multiplierDown"`
		MultiplierUp              string      `json:"multiplierUp"`
		MaxOpenOrders             int         `json:"maxOpenOrders"`
		MaxEntrusts               int         `json:"maxEntrusts"`
		MakerFee                  string      `json:"makerFee"`
		TakerFee                  string      `json:"takerFee"`
		LiquidationFee            string      `json:"liquidationFee"`
		MarketTakeBound           string      `json:"marketTakeBound"`
		DepthPrecisionMerge       int         `json:"depthPrecisionMerge"`
		Labels                    []string    `json:"labels"`
		OnboardDate               int64       `json:"onboardDate"`
		EnName                    string      `json:"enName"`
		CnName                    string      `json:"cnName"`
		MinStepPrice              string      `json:"minStepPrice"`
		MinPrice                  interface{} `json:"minPrice"`
		MaxPrice                  interface{} `json:"maxPrice"`
		DeliveryDate              int64       `json:"deliveryDate"`
		DeliveryPrice             interface{} `json:"deliveryPrice"`
		DeliveryCompletion        bool        `json:"deliveryCompletion"`
		CnDesc                    interface{} `json:"cnDesc"`
		EnDesc                    interface{} `json:"enDesc"`
	} `json:"result"`
	ReturnCode int `json:"returnCode"`
}

type BookWsResp struct {
	Topic string `json:"topic"`
	Event string `json:"event"`
	Data  struct {
		Id string     `json:"id"`
		S  string     `json:"s"`
		I  int        `json:"i"`
		T  int        `json:"t"`
		A  [][]string `json:"a"`
		B  [][]string `json:"b"`
	} `json:"data"`
}

type MarkPriceResp struct {
	Topic string `json:"topic"`
	Event string `json:"event"`
	Data  struct {
		S string `json:"s"`
		P string `json:"p"`
		T int    `json:"t"`
	} `json:"data"`
}

type BalanceResp struct {
	Rc int    `json:"rc"`
	Mc string `json:"mc"`
	Ma []struct {
	} `json:"ma"`
	Result struct {
		TotalBtcAmount int `json:"totalBtcAmount"`
		Assets         []struct {
			Currency          string `json:"currency"`
			CurrencyId        int    `json:"currencyId"`
			FrozenAmount      string `json:"frozenAmount"`
			AvailableAmount   string `json:"availableAmount"`
			TotalAmount       string `json:"totalAmount"`
			ConvertBtcAmount  string `json:"convertBtcAmount"`
			ConvertUsdtAmount string `json:"convertUsdtAmount"`
		} `json:"assets"`
	} `json:"result"`
}

type XtContractAssetResp struct {
	Error struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	} `json:"error"`
	MsgInfo string `json:"msgInfo"`
	Result  []struct {
		AvailableBalance      string `json:"availableBalance"`
		Coin                  string `json:"coin"`
		IsolatedMargin        string `json:"isolatedMargin"`
		OpenOrderMarginFrozen string `json:"openOrderMarginFrozen"`
		WalletBalance         string `json:"walletBalance"`
	} `json:"result"`
	ReturnCode int `json:"returnCode"`
}

type XtContractPositionResp struct {
	Error struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	} `json:"error"`
	MsgInfo string `json:"msgInfo"`
	Result  []struct {
		AutoMargin            bool    `json:"autoMargin"`
		AvailableCloseSize    int     `json:"availableCloseSize"`
		CloseOrderSize        int     `json:"closeOrderSize"`
		EntryPrice            float64 `json:"entryPrice"`
		IsolatedMargin        float64 `json:"isolatedMargin"`
		Leverage              int     `json:"leverage"`
		OpenOrderMarginFrozen float64 `json:"openOrderMarginFrozen"`
		PositionSide          string  `json:"positionSide"`
		PositionSize          int     `json:"positionSize"`
		PositionType          string  `json:"positionType"`
		RealizedProfit        int     `json:"realizedProfit"`
		Symbol                string  `json:"symbol"`
	} `json:"result"`
	ReturnCode int `json:"returnCode"`
}

type XtContractCommonResp struct {
	Error struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	} `json:"error"`
	MsgInfo string `json:"msgInfo"`
	Result  struct {
	} `json:"result"`
	ReturnCode int `json:"returnCode"`
}

type XtContractOrderResp struct {
	Error struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	} `json:"error"`
	MsgInfo string `json:"msgInfo"`
	Result  struct {
	} `json:"result"`
	ReturnCode int `json:"returnCode"`
}

type XtSpotOrderResp struct {
	Rc int    `json:"rc"`
	Mc string `json:"mc"`
	Ma []struct {
	} `json:"ma"`
	Result struct {
		OrderId string `json:"orderId"`
	} `json:"result"`
}

type XtCancelOrderResp struct {
	Rc int    `json:"rc"`
	Mc string `json:"mc"`
	Ma []struct {
	} `json:"ma"`
	Result struct {
	} `json:"result"`
}

type XtFundingResp struct {
	ReturnCode int         `json:"returnCode"`
	MsgInfo    string      `json:"msgInfo"`
	Error      interface{} `json:"error"`
	Result     struct {
		Symbol             string `json:"symbol"`
		FundingRate        string `json:"fundingRate"`
		NextCollectionTime int64  `json:"nextCollectionTime"`
		CollectionInternal int    `json:"collectionInternal"`
	} `json:"result"`
}

package dtos

type BybitSpotMarketResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Category string `json:"category"`
		List     []struct {
			Symbol        string `json:"symbol"`
			BaseCoin      string `json:"baseCoin"`
			QuoteCoin     string `json:"quoteCoin"`
			Innovation    string `json:"innovation"`
			Status        string `json:"status"`
			LotSizeFilter struct {
				BasePrecision  string `json:"basePrecision"`
				QuotePrecision string `json:"quotePrecision"`
				MinOrderQty    string `json:"minOrderQty"`
				MaxOrderQty    string `json:"maxOrderQty"`
				MinOrderAmt    string `json:"minOrderAmt"`
				MaxOrderAmt    string `json:"maxOrderAmt"`
			} `json:"lotSizeFilter"`
			PriceFilter struct {
				TickSize string `json:"tickSize"`
			} `json:"priceFilter"`
		} `json:"list"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

type BybitPerpMarketResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Category string `json:"category"`
		List     []struct {
			Symbol          string `json:"symbol"`
			ContractType    string `json:"contractType"`
			Status          string `json:"status"`
			BaseCoin        string `json:"baseCoin"`
			QuoteCoin       string `json:"quoteCoin"`
			LaunchTime      string `json:"launchTime"`
			DeliveryTime    string `json:"deliveryTime"`
			DeliveryFeeRate string `json:"deliveryFeeRate"`
			PriceScale      string `json:"priceScale"`
			LeverageFilter  struct {
				MinLeverage  string `json:"minLeverage"`
				MaxLeverage  string `json:"maxLeverage"`
				LeverageStep string `json:"leverageStep"`
			} `json:"leverageFilter"`
			PriceFilter struct {
				MinPrice string `json:"minPrice"`
				MaxPrice string `json:"maxPrice"`
				TickSize string `json:"tickSize"`
			} `json:"priceFilter"`
			LotSizeFilter struct {
				MaxOrderQty         string `json:"maxOrderQty"`
				MinOrderQty         string `json:"minOrderQty"`
				QtyStep             string `json:"qtyStep"`
				PostOnlyMaxOrderQty string `json:"postOnlyMaxOrderQty"`
			} `json:"lotSizeFilter"`
			UnifiedMarginTrade bool   `json:"unifiedMarginTrade"`
			FundingInterval    int    `json:"fundingInterval"`
			SettleCoin         string `json:"settleCoin"`
		} `json:"list"`
		NextPageCursor string `json:"nextPageCursor"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

type BybitTickResp struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	Ts    int64  `json:"ts"`
	Cs    int64  `json:"cs"`
	Data  struct {
		Symbol            string `json:"symbol"`
		TickDirection     string `json:"tickDirection"`
		Price25hPcnt      string `json:"price24hPcnt"`
		LastPrice         string `json:"lastPrice"`
		PrevPrice24h      string `json:"prevPrice24h"`
		HighPrice24h      string `json:"highPrice24h"`
		LowPrice24h       string `json:"lowPrice24h"`
		PrevPrice1h       string `json:"prevPrice1h"`
		MarkPrice         string `json:"markPrice"`
		IndexPrice        string `json:"indexPrice"`
		OpenInterest      string `json:"openInterest"`
		OpenInterestValue string `json:"openInterestValue"`
		Turnover24h       string `json:"turnover24h"`
		Volume24h         string `json:"volume24h"`
		NextFundingTime   string `json:"nextFundingTime"`
		FundingRate       string `json:"fundingRate"`
		Bid1Price         string `json:"bid1Price"`
		Bid1Size          string `json:"bid1Size"`
		Ask1Price         string `json:"ask1Price"`
		Ask1Size          string `json:"ask1Size"`
	} `json:"data"`
}

type BybitBookWsResp struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	Ts    int64  `json:"ts"`
	Data  struct {
		S   string     `json:"s"`
		B   [][]string `json:"b"`
		A   [][]string `json:"a"`
		U   int        `json:"u"`
		Seq int64      `json:"seq"`
	} `json:"data"`
}

type BybitBalanceCoinResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		MemberId    string `json:"memberId"`
		AccountType string `json:"accountType"`
		List        []struct {
			Coin               string `json:"coin"`
			TransferBalance    string `json:"transferBalance"`
			TotalMarginBalance string `json:"totalMarginBalance"`
			WalletBalance      string `json:"walletBalance"`
			Bonus              string `json:"bonus"`
		} `json:"balance"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

type BybitBalanceResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []struct {
			TotalEquity            string `json:"totalEquity"`
			AccountIMRate          string `json:"accountIMRate"`
			TotalMarginBalance     string `json:"totalMarginBalance"`
			TotalInitialMargin     string `json:"totalInitialMargin"`
			AccountType            string `json:"accountType"`
			TotalAvailableBalance  string `json:"totalAvailableBalance"`
			AccountMMRate          string `json:"accountMMRate"`
			TotalPerpUPL           string `json:"totalPerpUPL"`
			TotalWalletBalance     string `json:"totalWalletBalance"`
			AccountLTV             string `json:"accountLTV"`
			TotalMaintenanceMargin string `json:"totalMaintenanceMargin"`
			Coin                   []struct {
				AvailableToBorrow   string `json:"availableToBorrow"`
				Bonus               string `json:"bonus"`
				AccruedInterest     string `json:"accruedInterest"`
				AvailableToWithdraw string `json:"availableToWithdraw"`
				TotalOrderIM        string `json:"totalOrderIM"`
				Equity              string `json:"equity"`
				TotalPositionMM     string `json:"totalPositionMM"`
				UsdValue            string `json:"usdValue"`
				UnrealisedPnl       string `json:"unrealisedPnl"`
				BorrowAmount        string `json:"borrowAmount"`
				TotalPositionIM     string `json:"totalPositionIM"`
				WalletBalance       string `json:"walletBalance"`
				CumRealisedPnl      string `json:"cumRealisedPnl"`
				Coin                string `json:"coin"`
			} `json:"coin"`
		} `json:"list"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

//type BybitBalanceResp struct {
//	RetCode int    `json:"retCode"`
//	RetMsg  string `json:"retMsg"`
//	Result  struct {
//		Spot struct {
//			Status string `json:"status"`
//			Assets []struct {
//				Coin     string `json:"coin"`
//				Frozen   string `json:"frozen"`
//				Free     string `json:"free"`
//				Withdraw string `json:"withdraw"`
//			} `json:"assets"`
//		} `json:"spot"`
//	} `json:"result"`
//	RetExtInfo struct {
//	} `json:"retExtInfo"`
//	Time int64 `json:"time"`
//}

type BybitPositionResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		NextPageCursor string `json:"nextPageCursor"`
		Category       string `json:"category"`
		List           []struct {
			Symbol         string `json:"symbol"`
			Leverage       string `json:"leverage"`
			AvgPrice       string `json:"avgPrice"`
			LiqPrice       string `json:"liqPrice"`
			RiskLimitValue string `json:"riskLimitValue"`
			TakeProfit     string `json:"takeProfit"`
			PositionValue  string `json:"positionValue"`
			TpslMode       string `json:"tpslMode"`
			RiskId         int    `json:"riskId"`
			TrailingStop   string `json:"trailingStop"`
			UnrealisedPnl  string `json:"unrealisedPnl"`
			MarkPrice      string `json:"markPrice"`
			CumRealisedPnl string `json:"cumRealisedPnl"`
			PositionMM     string `json:"positionMM"`
			CreatedTime    string `json:"createdTime"`
			PositionIdx    int    `json:"positionIdx"`
			PositionIM     string `json:"positionIM"`
			UpdatedTime    string `json:"updatedTime"`
			Side           string `json:"side"`
			BustPrice      string `json:"bustPrice"`
			Size           string `json:"size"`
			PositionStatus string `json:"positionStatus"`
			StopLoss       string `json:"stopLoss"`
			TradeMode      int    `json:"tradeMode"`
		} `json:"list"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

type BybitUpgradeUtaResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		UnifiedUpdateStatus string `json:"unifiedUpdateStatus"`
		UnifiedUpdateMsg    struct {
			Msg []string `json:"msg"`
		} `json:"unifiedUpdateMsg"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

type BybitMarginModeResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Reasons []struct {
			ReasonCode string `json:"reasonCode"`
			ReasonMsg  string `json:"reasonMsg"`
		} `json:"reasons"`
	} `json:"result"`
}

type BybitOrderUpdateResp struct {
	Id           string `json:"id"`
	Topic        string `json:"topic"`
	CreationTime int64  `json:"creationTime"`
	Data         []struct {
		Symbol             string `json:"symbol"`
		OrderId            string `json:"orderId"`
		Side               string `json:"side"`
		OrderType          string `json:"orderType"`
		CancelType         string `json:"cancelType"`
		Price              string `json:"price"`
		Qty                string `json:"qty"`
		OrderIv            string `json:"orderIv"`
		TimeInForce        string `json:"timeInForce"`
		OrderStatus        string `json:"orderStatus"`
		OrderLinkId        string `json:"orderLinkId"`
		LastPriceOnCreated string `json:"lastPriceOnCreated"`
		ReduceOnly         bool   `json:"reduceOnly"`
		LeavesQty          string `json:"leavesQty"`
		LeavesValue        string `json:"leavesValue"`
		CumExecQty         string `json:"cumExecQty"`
		CumExecValue       string `json:"cumExecValue"`
		AvgPrice           string `json:"avgPrice"`
		BlockTradeId       string `json:"blockTradeId"`
		PositionIdx        int    `json:"positionIdx"`
		CumExecFee         string `json:"cumExecFee"`
		ClosedPnl          string `json:"closedPnl"`
		CreatedTime        string `json:"createdTime"`
		UpdatedTime        string `json:"updatedTime"`
		RejectReason       string `json:"rejectReason"`
		StopOrderType      string `json:"stopOrderType"`
		TpslMode           string `json:"tpslMode"`
		TriggerPrice       string `json:"triggerPrice"`
		TakeProfit         string `json:"takeProfit"`
		StopLoss           string `json:"stopLoss"`
		TpTriggerBy        string `json:"tpTriggerBy"`
		SlTriggerBy        string `json:"slTriggerBy"`
		TpLimitPrice       string `json:"tpLimitPrice"`
		SlLimitPrice       string `json:"slLimitPrice"`
		TriggerDirection   int    `json:"triggerDirection"`
		TriggerBy          string `json:"triggerBy"`
		CloseOnTrigger     bool   `json:"closeOnTrigger"`
		Category           string `json:"category"`
		PlaceType          string `json:"placeType"`
		SmpType            string `json:"smpType"`
		SmpGroup           int    `json:"smpGroup"`
		SmpOrderId         string `json:"smpOrderId"`
		FeeCurrency        string `json:"feeCurrency"`
	} `json:"data"`
}

type BybitOrderResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		OrderId     string `json:"orderId"`
		OrderLinkId string `json:"orderLinkId"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

type BybitTickersResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Category string `json:"category"`
		List     []struct {
			Symbol                 string `json:"symbol"`
			LastPrice              string `json:"lastPrice"`
			IndexPrice             string `json:"indexPrice"`
			MarkPrice              string `json:"markPrice"`
			PrevPrice24H           string `json:"prevPrice24h"`
			Price24HPcnt           string `json:"price24hPcnt"`
			HighPrice24H           string `json:"highPrice24h"`
			LowPrice24H            string `json:"lowPrice24h"`
			PrevPrice1H            string `json:"prevPrice1h"`
			OpenInterest           string `json:"openInterest"`
			OpenInterestValue      string `json:"openInterestValue"`
			Turnover24H            string `json:"turnover24h"`
			Volume24H              string `json:"volume24h"`
			FundingRate            string `json:"fundingRate"`
			NextFundingTime        string `json:"nextFundingTime"`
			PredictedDeliveryPrice string `json:"predictedDeliveryPrice"`
			BasisRate              string `json:"basisRate"`
			DeliveryFeeRate        string `json:"deliveryFeeRate"`
			DeliveryTime           string `json:"deliveryTime"`
			Ask1Size               string `json:"ask1Size"`
			Bid1Price              string `json:"bid1Price"`
			Ask1Price              string `json:"ask1Price"`
			Bid1Size               string `json:"bid1Size"`
			Basis                  string `json:"basis"`
		} `json:"list"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

type BybitOrderDetailResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		NextPageCursor string `json:"nextPageCursor"`
		Category       string `json:"category"`
		List           []struct {
			Symbol             string `json:"symbol"`
			OrderType          string `json:"orderType"`
			OrderLinkId        string `json:"orderLinkId"`
			OrderId            string `json:"orderId"`
			CancelType         string `json:"cancelType"`
			AvgPrice           string `json:"avgPrice"`
			StopOrderType      string `json:"stopOrderType"`
			LastPriceOnCreated string `json:"lastPriceOnCreated"`
			OrderStatus        string `json:"orderStatus"`
			TakeProfit         string `json:"takeProfit"`
			CumExecValue       string `json:"cumExecValue"`
			TriggerDirection   int    `json:"triggerDirection"`
			BlockTradeId       string `json:"blockTradeId"`
			RejectReason       string `json:"rejectReason"`
			IsLeverage         string `json:"isLeverage"`
			Price              string `json:"price"`
			OrderIv            string `json:"orderIv"`
			CreatedTime        string `json:"createdTime"`
			TpTriggerBy        string `json:"tpTriggerBy"`
			PositionIdx        int    `json:"positionIdx"`
			TimeInForce        string `json:"timeInForce"`
			LeavesValue        string `json:"leavesValue"`
			UpdatedTime        string `json:"updatedTime"`
			Side               string `json:"side"`
			TriggerPrice       string `json:"triggerPrice"`
			CumExecFee         string `json:"cumExecFee"`
			SlTriggerBy        string `json:"slTriggerBy"`
			LeavesQty          string `json:"leavesQty"`
			CloseOnTrigger     bool   `json:"closeOnTrigger"`
			CumExecQty         string `json:"cumExecQty"`
			ReduceOnly         bool   `json:"reduceOnly"`
			Qty                string `json:"qty"`
			StopLoss           string `json:"stopLoss"`
			TriggerBy          string `json:"triggerBy"`
		} `json:"list"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

type BybitCollateralResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []struct {
			BorrowAmount        string `json:"borrowAmount"`
			AvailableToBorrow   string `json:"availableToBorrow"`
			FreeBorrowingAmount string `json:"freeBorrowingAmount"`
			Borrowable          bool   `json:"borrowable"`
			Currency            string `json:"currency"`
			MaxBorrowingAmount  string `json:"maxBorrowingAmount"`
			HourlyBorrowRate    string `json:"hourlyBorrowRate"`
			MarginCollateral    bool   `json:"marginCollateral"`
			CollateralRatio     string `json:"collateralRatio"`
		} `json:"list"`
	} `json:"result"`
	RetExtInfo struct {
	} `json:"retExtInfo"`
	Time int64 `json:"time"`
}

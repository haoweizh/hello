package dtos

type MexcContractGetMarketsResp struct {
	Success bool `json:"success"`
	Code    int  `json:"code"`
	Data    []struct {
		Symbol                    string   `json:"symbol"`
		DisplayName               string   `json:"displayName"`
		DisplayNameEn             string   `json:"displayNameEn"`
		PositionOpenType          int      `json:"positionOpenType"`
		BaseCoin                  string   `json:"baseCoin"`
		QuoteCoin                 string   `json:"quoteCoin"`
		SettleCoin                string   `json:"settleCoin"`
		ContractSize              float64  `json:"contractSize"`
		MinLeverage               int      `json:"minLeverage"`
		MaxLeverage               int      `json:"maxLeverage"`
		PriceScale                int      `json:"priceScale"`
		VolScale                  int      `json:"volScale"`
		AmountScale               int      `json:"amountScale"`
		PriceUnit                 float64  `json:"priceUnit"`
		VolUnit                   int      `json:"volUnit"`
		MinVol                    float64  `json:"minVol"`
		MaxVol                    float64  `json:"maxVol"`
		BidLimitPriceRate         float64  `json:"bidLimitPriceRate"`
		AskLimitPriceRate         float64  `json:"askLimitPriceRate"`
		TakerFeeRate              float64  `json:"takerFeeRate"`
		MakerFeeRate              float64  `json:"makerFeeRate"`
		MaintenanceMarginRate     float64  `json:"maintenanceMarginRate"`
		InitialMarginRate         float64  `json:"initialMarginRate"`
		RiskBaseVol               int      `json:"riskBaseVol"`
		RiskIncrVol               int      `json:"riskIncrVol"`
		RiskIncrMmr               float64  `json:"riskIncrMmr"`
		RiskIncrImr               float64  `json:"riskIncrImr"`
		RiskLevelLimit            int      `json:"riskLevelLimit"`
		PriceCoefficientVariation float64  `json:"priceCoefficientVariation"`
		IndexOrigin               []string `json:"indexOrigin"`
		State                     int      `json:"state"`
		IsNew                     bool     `json:"isNew"`
		IsHot                     bool     `json:"isHot"`
		IsHidden                  bool     `json:"isHidden"`
	} `json:"data"`
}

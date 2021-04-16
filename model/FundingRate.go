package model

import "time"

var fundingRates = make(map[string]map[string]*FundingRate) // market - symbol - funding rate

type FundingRate struct {
	FundingTime            time.Time // confirm time if transaction
	Rate, RateNext         float64
	UpdateTime, ExpireTime int64
	Symbol                 string
}

func GetFundingRate(market, symbol string) (rate *FundingRate) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if fundingRates == nil || fundingRates[market] == nil {
		return nil
	}
	return fundingRates[market][symbol]
}

func SetFundingRate(market, symbol string, fundingRate *FundingRate) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if fundingRates[market] == nil {
		fundingRates[market] = make(map[string]*FundingRate)
	}
	fundingRates[market][symbol] = fundingRate
}

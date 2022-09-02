package model

import "time"

var fundingRates = make(map[string]map[string]*FundingRate) // market - symbol - funding rate

type FundingRate struct {
	Rate, RateNext float64
	UpdateTime     time.Time // api访问时间
	ExpireTime     int64     // 按秒计算的 unix time
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

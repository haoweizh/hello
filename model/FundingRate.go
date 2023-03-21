package model

import (
	"hello/util"
	"math"
	"sync"
	"time"
)

var FundingRates = &sync.Map{} // market - symbol - *FundingRate

type FundingRate struct {
	Rate, RateNext float64
	UpdateTime     time.Time // api访问时间
	ExpireTime     int64     // 按秒计算的 unix time
}

func SetFundingRate(market, symbol string, fundingRate *FundingRate) {
	if math.Abs(fundingRate.Rate) > 0.02 {
		fundingRate.Rate = 3 * fundingRate.Rate
	} else {
		fRate := fundingRate.Rate * 100
		if fRate >= 0 {
			fundingRate.Rate = fRate * (1 + fRate) / 100
		} else {
			fRate = -1 * fRate
			fundingRate.Rate = -1 * fRate * (1 + fRate) / 100
		}
	}
	if math.Abs(fundingRate.RateNext) > 0.02 {
		fundingRate.RateNext = 3 * fundingRate.RateNext
	} else {
		fRateNext := fundingRate.RateNext * 100
		if fRateNext > 0 {
			fundingRate.RateNext = fRateNext * (1 + fRateNext) / 100
		} else {
			fRateNext = -1 * fRateNext
			fundingRate.RateNext = -1 * fRateNext * (1 + fRateNext) / 100
		}
	}
	util.StoreSyncMap(FundingRates, fundingRate, market, symbol)
}

package model

import "time"

var fundingRate = make(map[string]map[string]float64)     // market - symbol - funding rate
var fundingRateUpdate = make(map[string]map[string]int64) // market - symbol - update time

type FundingRate struct {
	FundingTime time.Time // confirm time if transaction
	Rate        float64   // price in usdt
	Symbol      string
	ID          string `gorm:"primary_key"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func GetFundingRate(market, symbol string) (rate float64, updateTime int64) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if fundingRate == nil {
		fundingRate = make(map[string]map[string]float64)
	}
	if fundingRate[market] == nil {
		fundingRate[market] = make(map[string]float64)
	}
	if fundingRateUpdate == nil {
		fundingRateUpdate = make(map[string]map[string]int64)
	}
	if fundingRateUpdate[market] == nil {
		fundingRateUpdate[market] = make(map[string]int64)
	}
	return fundingRate[market][symbol], fundingRateUpdate[market][symbol]
}

func SetFundingRate(market, symbol string, rate float64, updateTime int64) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if fundingRate == nil {
		fundingRate = make(map[string]map[string]float64)
	}
	if fundingRate[market] == nil {
		fundingRate[market] = make(map[string]float64)
	}
	if fundingRateUpdate == nil {
		fundingRateUpdate = make(map[string]map[string]int64)
	}
	if fundingRateUpdate[market] == nil {
		fundingRateUpdate[market] = make(map[string]int64)
	}
	fundingRate[market][symbol] = rate
	fundingRateUpdate[market][symbol] = updateTime
}

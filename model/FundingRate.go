package model

import (
	"sync"
	"time"
) // market - symbol - *FundingRate

var FundingRates = &sync.Map{}

type FundingRate struct {
	Rate, RateNext float64
	UpdateTime     time.Time // api访问时间
	ExpireTime     int64     // 按秒计算的 unix time
}

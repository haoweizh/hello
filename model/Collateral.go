package model

type Collateral struct {
	AccountKey string  // Account key
	Available  float64 // 可用
	Occupied   float64 // 已被占用
	Rate       float64 // 保证金率
}

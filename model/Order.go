package model

import (
	"strings"
	"time"
)

type Order struct {
	Amount            float64
	DealAmount        float64
	DealPrice         float64
	Fee               float64
	LineBuy, LineSell float64
	Price             float64
	TriggerPrice      float64
	UnfilledQuantity  float64 //未成交数量
	GridPos           int64
	AccountIndex      int // 记录配置账户序号index
	Coin              string
	ErrCode           string
	Function          string
	Market            string
	OrderId           string `gorm:"unique"`
	OrderSide         string
	OrderType         string
	RefreshType       string // 1: near refreshLink 2: far refreshLink
	Status            string
	Symbol            string
	OrderTime         time.Time
	OrderUpdateTime   time.Time
	ID                uint      `gorm:"primary_key"`
	CreatedAt         time.Time `gorm:"idx_create_time"`
	UpdatedAt         time.Time
}

func (order *Order) HaveId() (result bool) {
	orderId := strings.Trim(order.OrderId, ` `)
	if orderId == `` || orderId == `0` || strings.Contains(orderId, `error`) {
		return false
	}
	return true
}

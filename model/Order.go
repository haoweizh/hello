package model

import (
	"strings"
	"time"
)

type Order struct {
	Amount            float64
	AmountType        string
	Coin              string
	DealAmount        float64
	DealPrice         float64
	ErrCode           string
	Fee               float64
	FeeIncome         float64
	Function          string
	GridPos           int64
	LineBuy, LineSell float64
	Market            string
	OrderId           string `gorm:"unique"`
	OrderSide         string
	OrderTime         time.Time
	OrderType         string
	OrderUpdateTime   time.Time
	Price             float64
	RefreshType       string // 1: near refresh 2: far refresh
	Status            string
	Symbol            string
	TriggerPrice      float64
	UnfilledQuantity  float64 //未成交数量
	ID                uint    `gorm:"primary_key"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (order *Order) HaveId() (result bool) {
	orderId := strings.Trim(order.OrderId, ` `)
	if orderId == `` || orderId == `0` || strings.Contains(orderId, `error`) {
		return false
	}
	return true
}

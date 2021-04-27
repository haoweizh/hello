package model

import "time"

type Order struct {
	Amount       float64
	AmountType   string
	DealAmount   float64
	DealPrice    float64
	ErrCode      string
	Fee          float64
	FeeIncome    float64
	Function     string
	GridPos      int64
	Instrument   string
	Market       string
	OrderId      string `gorm:"unique"`
	OrderSide    string
	OrderTime    time.Time
	OrderType    string
	Price        float64
	TriggerPrice float64
	RefreshType  string // 1: near refresh 2: far refresh
	Status       string
	Symbol       string
	ID           uint `gorm:"primary_key"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// fmex
	Features          int64   //订单特性，每个bit表示一种特性：0x01=FOK，0x02=post_only，0x04=Hidden，0x08=IOC，0x8000=爆仓单"
	UnfilledQuantity  float64 //未成交数量
	MakerFeeRate      float64 //maker费率
	TakerFeeRate      float64 //taker费率
	TriggerDirection  string  //触发方向
	TriggerOn         float64
	TrailingBasePrice float64 //触发基础价格
	TrailingDistance  float64 //触发距离
	FrozenMargin      float64 //冻结margin
	FrozenQuantity    float64 //冻结数量
	Hidden            bool    //是否隐藏
	OrderUpdateTime   time.Time
}

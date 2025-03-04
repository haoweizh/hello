package model

import "time"

type ProcessBalancesHandler func(market, accountKey string, balances []*Balance)

type Balance struct {
	AccountId           string
	Coin                string
	Fee                 string // for transaction
	Market              string
	Notes               string
	Status              string // for transaction
	TransactionId       string
	Address             string  // for transaction
	Action              float64 // 1: deposit, -1: withdraw, 0: snapshot
	Amount              float64 //实际持仓头寸
	AvailableWithBorrow float64 //可借+持仓-已挂的卖单=现在总的可下卖单数量（千万别用这个作为持仓！）,binance、huobi现货按0计算可借
	FrozenAmount        float64 //冻结数量
	Borrow              float64
	BalanceTime         time.Time // confirm time if transaction
	UsdValue            float64
	ID                  string `gorm:"primary_key"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type FundingFee struct {
	Market    string `gorm:"index:index_market_symbol_ts,unique"`
	Symbol    string `gorm:"index:index_market_symbol_ts,unique"`
	BalChg    float64
	Index     int   `gorm:"index:index_market_symbol_ts,unique"`
	Ts        int64 `gorm:"index:index_market_symbol_ts,unique"`
	Ccy       string
	Id        uint `json:"id" gorm:"unique;primaryKey;autoIncrement"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

//var balanceValue = make(map[string][]*Balance) // market -
//var balanceUpdate = make(map[string]int64)     // market - update time in unix seconds
//var marginOKEX = make(map[string]float64)      // market - margin for okex only
//var fullValue = make(map[string]float64)       // market total in usd
//
//func GetBalance(market string) (balances []*Balance, value float64, collateral *Collateral, update int64) {
//	infoLock.Lock()
//	defer infoLock.Unlock()
//	if balanceValue[market] == nil {
//		return nil, 0, nil, 0
//	}
//	return balanceValue[market], fullValue[market], marginOKEX[market], balanceUpdate[market]
//}
//
//func SetBalance(market string, balances []*Balance, totalInUsd, margin float64, update int64) {
//	infoLock.Lock()
//	defer infoLock.Unlock()
//	balanceValue[market] = balances
//	balanceUpdate[market] = update
//	fullValue[market] = totalInUsd
//	marginOKEX[market] = margin
//}

package model

import "time"

var balanceValue = make(map[string][]*Balance) // market -
var balanceUpdate = make(map[string]int64)     // market - update time in unix seconds

type Balance struct {
	AccountId     string
	Action        float64 // 1: deposit, -1: withdraw, 0: snapshot
	Address       string  // for transaction
	Amount        float64
	Available     float64
	BalanceTime   time.Time // confirm time if transaction
	Coin          string
	Fee           string // for transaction
	Market        string
	Notes         string
	Price         float64 // price in usdt
	Status        string  // for transaction
	TransactionId string
	UsdValue      float64
	ID            string `gorm:"primary_key"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func GetBalance(market string) (balances []*Balance, update int64) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if balanceValue[market] == nil {
		return nil, 0
	}
	return balanceValue[market], balanceUpdate[market]
}

func SetBalance(market string, balances []*Balance, update int64) {
	infoLock.Lock()
	defer infoLock.Unlock()
	balanceValue[market] = balances
	balanceUpdate[market] = update
}

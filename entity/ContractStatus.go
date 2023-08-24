package entity

import (
	"sync"
	"time"
)

type Contract struct {
	CoinLeft, CoinRight     string
	AmountLeft, AmountRight float64
	Chain                   string
	Market                  string
	Symbol                  string
	Address                 string `gorm:"index:address,unique"`
	BlockHeight             int64
	ID                      uint `gorm:"primary_key"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

var ContractPool sync.Map        // chain*market*symbol contract
var ContractPoolPending sync.Map // chain*market*symbol contract

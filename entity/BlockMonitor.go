package entity

import (
	"time"
)

var DealBlockHeight int64
var DealBlockTime time.Time

// LoadBlockDB
// 1. load blockHeight: select max(BlockHeight) from Contract
// 2. load to set ContractPool & ContractPoolPending
func LoadBlockDB() {

}

func SaveBlockDB() {

}

// BlockSubscribe subscribe to get block msg
func BlockSubscribe() {

}

// MemPoolSubscribe subscribe to get mem-pool pending info
func MemPoolSubscribe() {

}

// BlockMsgHandler
// 1. deal with block msg
// 2. set ContractPool & ContractPoolPending
func BlockMsgHandler() {

}

// MemPoolPendingHandler
// 1. get pending msg, filter out msg later than DealBlockTime
// 2. set  ContractPoolPending
// 3. throw msg into channel
func MemPoolPendingHandler() {

}

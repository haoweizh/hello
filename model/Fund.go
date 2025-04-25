package model

import "time"

type Fund struct {
	Market                                                   string `gorm:"index:market_key_seconds,unique"`
	Key                                                      string `gorm:"index:market_key_seconds,unique"`
	Seconds                                                  int64  `gorm:"index:market_key_seconds,unique"` // unix seconds
	Index                                                    int
	Value, HoldingSpot, HoldingFuture, BorrowSpot, Available float64
	ID                                                       uint `gorm:"primary_key"`
	CreatedAt                                                time.Time
	UpdatedAt                                                time.Time
}

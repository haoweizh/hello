package model

import "fmt"

type Pool struct {
	Address            string
	Fee                float64
	Token0, Token1     string
	Reserve0, Reserve1 float64
	Decimal0, Decimal1 float64
	DealPrice          func(pool *Pool) (price float64)
}

type Pair struct {
	Pools []*Pool
	Price float64
}

func (pool *Pool) UniV2() (price float64) {
	return pool.Reserve1 / pool.Reserve0
}

func (pair *Pair) GetKey() (key string) {
	return fmt.Sprintf(`%s_%s`, pair.GetHead(), pair.GetTail())
}

func (pair *Pair) GetHead() (head string) {
	if len(pair.Pools) < 1 {
		return ``
	}
	return pair.Pools[0].Token0
}

func (pair *Pair) GetTail() (tail string) {
	if len(pair.Pools) < 1 {
		return ``
	}
	return pair.Pools[len(pair.Pools)-1].Token1
}

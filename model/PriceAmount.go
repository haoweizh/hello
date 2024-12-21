package model

type Ticks []Tick

type Tick struct {
	Id                  string
	Market, Symbol      string
	PriceStr, AmountStr string
	Ts                  int // unix time in million-seconds
	Price, Amount       float64
}

func (ticks Ticks) Len() int {
	return len(ticks)
}

func (ticks Ticks) Swap(i, j int) {
	ticks[i], ticks[j] = ticks[j], ticks[i]
}

func (ticks Ticks) Less(i, j int) bool {
	return ticks[i].Price < ticks[j].Price
}

func _(tickMap map[float64]*Tick) (ticks Ticks) {
	ticks = make([]Tick, len(tickMap))
	index := 0
	for _, value := range tickMap {
		ticks[index] = *value
		index++
	}
	return ticks
}

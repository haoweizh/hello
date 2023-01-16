package model

type Candles []*Candle

func (candles Candles) Len() int {
	return len(candles)
}

func (candles Candles) Swap(i, j int) {
	candles[i], candles[j] = candles[j], candles[i]
}

func (candles Candles) Less(i, j int) bool {
	return candles[i].Begin.Before(candles[j].Begin)
}

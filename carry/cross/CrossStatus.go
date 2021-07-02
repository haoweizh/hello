package cross

var status = make(map[string]map[string]map[string]map[string]*CarryStatus) // coin - market - symbol - key - CarryStatus

type CarryStatus struct {
	Market                      string
	Symbol                      string
	LimitSell, LimitBuy         float64 // 最大可买卖币数
	TradeLineBuy, TradeLineSell float64 // 买卖盈利线（可为负数）
	Holding                     float64
	UsdValue                    float64
	RateInAll                   float64 // 当前币种或持仓占总权益的比例
}

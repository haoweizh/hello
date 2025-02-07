package model

type Position struct {
	Direction                string
	Market                   string
	Currency                 string
	LeverRate                int64 // 杠杆倍数
	Ts                       int64
	Holding                  float64 // 持仓数量
	Frozen                   float64
	ProfitReal               float64
	ProfitUnreal             float64
	Margin                   float64 // 仓位保证金，必须为正
	BankruptcyPrice          float64 // 破产价格，以该价格平仓，扣除taker手续费后，其权益恰好为0
	LiquidationPrice         float64 // 强平价格，以该价格平仓，扣除taker手续费后，其剩余权益恰好为仓位价值 x 维持保证金率
	EntryPrice               float64 // 开仓均价，每次仓位增加或减少时，开仓均价都会调整
	MinimumMaintenanceMargin float64 // 最小维持保证金，如果仓位保证金降低到此，将立刻触发强平
	RiskLimit                float64
	DirectionDetail          map[string]float64 //key-开仓方向sell/buy value-数量，带+-号
}

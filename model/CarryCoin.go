package model

import (
	"fmt"
	"hello/util"
	"time"
)

const SmallHolding = 20 // 设定以money计20位较少持仓，可以归入下一等级

type CarryStatus struct {
	IsSpot                        bool
	Market, Symbol                string
	ReduceOnlyBuy, ReduceOnlySell bool
	Setting                       *Setting
	Account                       *Account
	LimitSell, LimitBuy           float64 // 未经过setting.GridAmount处理过的原始下单数量限制，用于comp
	AvailableSell, AvailableBuy   float64 // 未经过setting.GridAmount处理过的原始最大可买卖数（不管有无机会，能下的数量),用于comp
	TradeLineBuy, TradeLineSell   float64 // 买卖盈利线（可为负数）
	Holding                       float64 // 未经过setting.GridAmount处理过的原始持仓数量
	RateInAll                     float64 // 现货：该币种占总权益的比例；永续：以开仓价算该币种持仓占保证金百分比
}

type CarryCoin struct {
	AccountIndex int     `gorm:"index:account_coin,unique"`
	Coin         string  `gorm:"index:account_coin,unique"`
	CurrentStep  int     // 网格搬砖中表示当前持仓所属的n值
	Holding      float64 // 当前持仓数量
	MoneyPerStep float64 // 网格搬砖中每一档位以定价币为单位的金额
	MoneyCurStep float64 // 当前档位已开仓金额
	Price        float64
	ID           uint `gorm:"primary_key"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (carryCoin *CarryCoin) AddTrade(statusBuy, statusSell *CarryStatus, priceBuy, priceSell, amountSell float64) {
	change := false
	if statusBuy.Holding*priceBuy >= -SmallHolding && statusSell.Holding*priceSell <= SmallHolding { // 加仓
		change = true
		carryCoin.MoneyCurStep += amountSell * priceSell
		carryCoin.Holding += amountSell * statusSell.Setting.GridAmount
		if carryCoin.MoneyCurStep > carryCoin.MoneyPerStep {
			carryCoin.CurrentStep++
			carryCoin.MoneyCurStep -= carryCoin.MoneyPerStep
		}
		util.Log(util.LogLevelInfo, fmt.Sprintf(`add trade deal open %#v amount %f price %f`, carryCoin, amountSell, priceSell))
	} else if statusBuy.Holding*priceBuy < -SmallHolding && statusSell.Holding*priceSell > SmallHolding { // 平仓
		change = true
		carryCoin.Holding -= amountSell * statusSell.Setting.GridAmount
		carryCoin.MoneyCurStep -= amountSell * priceSell
		if carryCoin.Holding*priceBuy/statusBuy.Setting.GridAmount >= SmallHolding {
			if carryCoin.MoneyCurStep < 0 {
				if carryCoin.CurrentStep >= 1 {
					carryCoin.CurrentStep--
					carryCoin.MoneyCurStep += carryCoin.MoneyPerStep
				} else {
					carryCoin.MoneyCurStep = 0
				}
			}
			util.Log(util.LogLevelInfo, fmt.Sprintf(`add trade deal close %#v amount %f price %f`, carryCoin, amountSell, priceSell))
		} else {
			carryCoin.MoneyCurStep = 0
			carryCoin.CurrentStep = 0
			util.Log(util.LogLevelInfo, fmt.Sprintf(`add trade deal close no holding %#v amount %f price %f`, carryCoin, amountSell, priceSell))
		}
	}
	if change {
		AppDB.Model(carryCoin).Where(`coin=? and account_index=?`, carryCoin.Coin, carryCoin.AccountIndex).Updates(
			map[string]interface{}{`current_step`: carryCoin.CurrentStep, `money_cur_step`: carryCoin.MoneyCurStep,
				`holding`: carryCoin.Holding, `price`: priceSell / statusSell.Setting.PriceX})
	}
}

package model

import (
	"fmt"
	"math"
	"time"
)

type TurtleData struct {
	// useNear是否在海龟交易时使用lowDaysNear和highDaysNear和priceX作为触发条件
	// adjustChecked在设置为true前，不允许使用本Data进行交易
	UseNear, BreakLong, BreakShort, Liquidated, AdjustChecked, OrderCleared   bool
	TurtleTime, CheckUseApi, CheckTimeOpen                                    time.Time
	HighDaysNear, LowDaysNear, HighDaysFar, LowDaysFar, LowAdjust, HighAdjust float64
	HighToday, LowToday, N, NVolume, Amount                                   float64
	DaysNear, DaysFar, DaysAdjust, combineBig                                 int // CombineBig: -1小单，1大单，0未初始化
	Symbol                                                                    string
	// 适应某些交易所单笔订单不能过大，大笔订单会拆分后下成多个，因价格超出无法下成的单为了不被取消，也归入orderAdjust
	OrderLong, OrderShort, OrderAdjust []*Order
}

type TurtleDataArray []*TurtleData

func (turtleDataArray TurtleDataArray) Len() int {
	return len(turtleDataArray)
}

func (turtleDataArray TurtleDataArray) Swap(i, j int) {
	turtleDataArray[i], turtleDataArray[j] = turtleDataArray[j], turtleDataArray[i]
}

func (turtleDataArray TurtleDataArray) Less(i, j int) bool {
	return turtleDataArray[i].NVolume < turtleDataArray[j].NVolume
}

func (turtleData *TurtleData) GetIds() (ids string) {
	ids = `long:`
	if turtleData.OrderLong != nil && len(turtleData.OrderLong) > 0 {
		ids += turtleData.OrderLong[0].OrderId
	}
	ids += `short:`
	if turtleData.OrderShort != nil && len(turtleData.OrderShort) > 0 {
		ids += turtleData.OrderShort[0].OrderId
	}
	return
}

func (turtleData *TurtleData) SetBig(isBig int) {
	turtleData.combineBig = isBig
}

func (turtleData *TurtleData) IsBig(settingCombine, settingNormal *Setting, marketInfo *MarketInfo) (isBig int) {
	if settingCombine.Far != settingNormal.Far || settingCombine.Near != settingNormal.Near {
		return 1
	}
	if turtleData.combineBig == 0 {
		if settingCombine.Chance+settingNormal.Chance == 0 && math.Abs(settingCombine.PriceX-settingNormal.PriceX) < marketInfo.PriceIncrement {
			turtleData.combineBig = -1
		} else {
			turtleData.combineBig = 1
		}
	}
	return turtleData.combineBig
}

func (turtleData *TurtleData) ToString() (str string) {
	if turtleData == nil {
		return `turtle data is nil`
	}
	return fmt.Sprintf(`%d日%e~%e N:%e Amount:%e`,
		turtleData.DaysFar, turtleData.LowDaysFar, turtleData.HighDaysFar, turtleData.N, turtleData.Amount)
}

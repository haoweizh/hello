package model

import (
	"fmt"
	"time"
)

type TurtleData struct {
	// useNear是否在海龟交易时使用lowDaysNear和highDaysNear和priceX作为触发条件
	UseNear bool
	// adjustChecked在设置为true前，不允许使用本Data进行交易
	AdjustChecked                                             bool
	BreakLong, BreakShort, Liquidated, OrderCleared, IsBig    bool
	TurtleTime, CheckUseApi, CheckTimeOpen, Expire            time.Time
	HighNear, LowNear, HighFar, LowFar, LowAdjust, HighAdjust float64
	HighLast, LowLast, N, M, NVolume, Amount                  float64
	// CallBackRatio: 跟踪单回撤比例 ActivationRate: 跟踪单激活比例
	CallBackRatio, ActivationRate float64
	DaysNear, DaysFar, DaysAdjust int // CombineBig: -1小单，1大单，0未初始化
	Symbol                        string
	OrderLong, OrderShort         []*Order
	// 适应某些交易所单笔订单不能过大，大笔订单会拆分后下成多个，因价格超出无法下成的单为了不被取消，也归入orderAdjust
	OrderAdjust map[string]*Order
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

func ResetBig(dataCombine, dataNormal *TurtleData) {
	dataNormal.IsBig = true
	dataCombine.IsBig = false
}

func (turtleData *TurtleData) ToString() (str string) {
	if turtleData == nil {
		return `turtle data is nil`
	}
	return fmt.Sprintf(`%d日%e~%e N:%e Amount:%e`,
		turtleData.DaysFar, turtleData.LowFar, turtleData.HighFar, turtleData.N, turtleData.Amount)
}

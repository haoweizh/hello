package model

import (
	"fmt"
	"time"
)

type TurtleData struct {
	// useNear是否在海龟交易时使用lowDaysNear和highDaysNear和priceX作为触发条件
	// adjustChecked在设置为true前，不允许使用本Data进行交易
	UseNear, BreakLong, BreakShort, Liquidated, AdjustChecked, OrderCleared bool
	TurtleTime, CheckUseApi, CheckTimeOpen, Expire                          time.Time
	HighNear, LowNear, HighFar, LowFar, LowAdjust, HighAdjust               float64
	HighToday, LowToday, N, M, NVolume, Amount                              float64
	DaysNear, DaysFar, DaysAdjust                                           int // CombineBig: -1小单，1大单，0未初始化
	Big                                                                     int64
	Symbol                                                                  string
	// 适应某些交易所单笔订单不能过大，大笔订单会拆分后下成多个，因价格超出无法下成的单为了不被取消，也归入orderAdjust
	OrderLong, OrderShort []*Order
	OrderAdjust           map[string]*Order
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
	dataNormal.Big = 1
	dataCombine.Big = -1
}

func (turtleData *TurtleData) ToString() (str string) {
	if turtleData == nil {
		return `turtle data is nil`
	}
	return fmt.Sprintf(`%d日%e~%e N:%e Amount:%e`,
		turtleData.DaysFar, turtleData.LowFar, turtleData.HighFar, turtleData.N, turtleData.Amount)
}

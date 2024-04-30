package util

import (
	"encoding/json"
	"fmt"
	"hello/model"
	"sync"
	"testing"
)

func Test_ClearSpot(t *testing.T) {
	sr := SlideRing{
		Start:   0,
		Current: 0,
		Data:    nil,
	}
	for i := 1; i < 10; i++ {
		sr.Add(i)
	}
	for i := 1; i < 5; i++ {
		sr.Remove()
	}
	for i := 1; i < 15; i++ {
		fmt.Println(sr.Remove())
	}
	fmt.Println(sr.Get())
}

func Test_Map(t *testing.T) {
	candle1 := &model.Candle{Market: `market1`}
	candle2 := &model.Candle{Market: `market2`}
	value := map[*model.Candle]bool{candle1: true, candle2: true}
	m := sync.Map{}
	m.Store(`1`, value)
	afterLoad, _ := m.Load(`1`)
	delete(afterLoad.(map[*model.Candle]bool), candle2)
	afterDel, _ := m.Load(`1`)
	for candle := range afterDel.(map[*model.Candle]bool) {
		fmt.Println(candle)
	}
}

func TestRange(t *testing.T) {
	testMap := map[string]string{}
	testMap = nil
	for s, s2 := range testMap {
		fmt.Println(s, s2)
	}
	fmt.Println(`done`)
}

func Test_unmarshal(t *testing.T) {
	sm := &model.SettingMonitor{
		MailAddress:     "haoweizh@qq.com",
		Market:          "binanceSpot",
		Symbol:          "BTC_PERP",
		IntervalSeconds: 0,
		WarnChange:      0,
		WarnIncrease:    0,
		WarnVolume:      0,
	}
	marshal, err := json.Marshal(sm)
	if err != nil {
		return
	}
	fmt.Println(string(marshal))
	var sm1 *model.SettingMonitor
	err = json.Unmarshal(marshal, &sm1)
	if err != nil {
		return
	}
	fmt.Println(sm1.Market)
}

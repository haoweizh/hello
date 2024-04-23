package monitor

import (
	"fmt"
	"hello/model"
	"sync"
	"testing"
)

func Test_ClearSpot(t *testing.T) {
	sr := SlideRing{
		start:   0,
		current: 0,
		data:    nil,
	}
	for i := 1; i < 10; i++ {
		sr.add(i)
	}
	for i := 1; i < 5; i++ {
		sr.remove()
	}
	for i := 1; i < 15; i++ {
		fmt.Println(sr.remove())
	}
	fmt.Println(sr.get())
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

package Ring

import (
	"fmt"
	"hello/model"
	"testing"
)

func TestRing(t *testing.T) {
	settings := make([]*model.Setting, 0)
	markets := []string{`甲`, `乙`} //, `丙`, `丁`}
	coin0s := []string{`btc`, `eth`, `a`, `b`, `c`, `d`, `e`, `f`, `g`, `h`, `i`}
	coin1s := []string{`btc`} //, `eth`}
	for _, market := range markets {
		for _, coin0 := range coin0s {
			for _, coin1 := range coin1s {
				if coin0 >= coin1 {
					continue
				}
				settings = append(settings, &model.Setting{
					Function: model.FunctionRing,
					Market:   market,
					Symbol:   fmt.Sprintf(`%s_%s`, coin0, coin1),
					Way:      "spot",
					Coin0:    coin0,
					Coin1:    coin1,
				})
			}
		}
	}
	InitRingPool(settings)
	for s := range CompleteRings {
		fmt.Println(s)
	}
	InitSettingRings()
	for s, m := range SettingRings {
		fmt.Println(fmt.Sprintf(`%s %v`, s, m))
	}
}

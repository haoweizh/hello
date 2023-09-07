package Ring

import (
	"fmt"
	"hello/model"
)

// todo update syncMap

var CompleteRings = make(map[string]*Ring)   // ringKey - *Ring
var SettingRings map[string]map[string]*Ring // settingKey - [ringKey]*Ring

type Edge struct {
	OrderSide string
	BidAsk    *model.BidAsk
}

type Ring struct {
	Complete           bool // already be a ring or not
	LinkedCoins        map[string]bool
	CoinHead, CoinTail string
	Edges              map[string]*Edge // settingKey - orderSide
	SortedSettings     []string
	Key                string
}

func (ring *Ring) Copy() (copy *Ring) {
	copy = &Ring{
		Complete:       ring.Complete,
		LinkedCoins:    make(map[string]bool),
		Edges:          make(map[string]*Edge),
		SortedSettings: make([]string, len(ring.SortedSettings)),
		CoinHead:       ring.CoinHead,
		CoinTail:       ring.CoinTail,
		Key:            ring.Key}
	for coin, b := range ring.LinkedCoins {
		copy.LinkedCoins[coin] = b
	}
	for settingKey, edge := range ring.Edges {
		copy.Edges[settingKey] = &Edge{OrderSide: edge.OrderSide}
	}
	for i, settingKey := range ring.SortedSettings {
		copy.SortedSettings[i] = settingKey
	}
	return
}

func (ring *Ring) AddSetting(setting *model.Setting) (added bool) {
	if ring.LinkedCoins == nil {
		ring.LinkedCoins = make(map[string]bool)
	}
	if ring.Edges == nil {
		ring.Edges = make(map[string]*Edge)
	}
	if ring.SortedSettings == nil {
		ring.SortedSettings = make([]string, 0)
	}
	if ring.Complete || ring.LinkedCoins[setting.Coin0] || ring.LinkedCoins[setting.Coin1] ||
		(ring.CoinHead != setting.Coin0 && ring.CoinHead != setting.Coin1 && ring.CoinTail != setting.Coin0 && ring.CoinTail != setting.Coin1) {
		return false
	}
	settingKey := setting.GetKey()
	if ring.Edges[settingKey] != nil {
		return false
	}
	if ring.CoinHead == setting.Coin0 {
		ring.Edges[settingKey] = &Edge{OrderSide: model.OrderSideBuy}
		ring.CoinHead = setting.Coin1
		ring.LinkedCoins[setting.Coin0] = true
		added = true
	} else if ring.CoinHead == setting.Coin1 {
		ring.Edges[settingKey] = &Edge{OrderSide: model.OrderSideSell}
		ring.CoinHead = setting.Coin0
		ring.LinkedCoins[setting.Coin1] = true
		added = true
	}
	if ring.CoinTail == setting.Coin0 {
		ring.Edges[settingKey] = &Edge{OrderSide: model.OrderSideSell}
		ring.CoinTail = setting.Coin1
		ring.LinkedCoins[setting.Coin0] = true
		added = true
	} else if ring.CoinTail == setting.Coin1 {
		ring.Edges[settingKey] = &Edge{OrderSide: model.OrderSideBuy}
		ring.CoinTail = setting.Coin0
		ring.LinkedCoins[setting.Coin1] = true
		added = true
	}
	if !added {
		return added
	}
	if ring.LinkedCoins[setting.Coin0] && ring.LinkedCoins[setting.Coin1] {
		ring.Complete = true
	}
	sortedSettings := make([]string, len(ring.SortedSettings)+1)
	ring.Key = ``
	index := 0
	for ; index < len(ring.SortedSettings) && ring.SortedSettings[index] < settingKey; index++ {
		sortedSettings[index] = ring.SortedSettings[index]
	}
	sortedSettings[index] = settingKey
	for i := index + 1; i < len(sortedSettings); i++ {
		sortedSettings[i] = ring.SortedSettings[i-1]
	}
	ring.SortedSettings = sortedSettings
	for i, sortedSetting := range sortedSettings {
		if i == 0 {
			ring.Key = fmt.Sprintf(`%s_%s`, sortedSetting, ring.Edges[sortedSetting].OrderSide)
		} else {
			ring.Key = fmt.Sprintf(`%s_%s_%s`, ring.Key, sortedSetting, ring.Edges[sortedSetting].OrderSide)
		}
	}
	return added
}

func (ring *Ring) Equals(compRing *Ring) (isEqual bool) {
	if compRing == nil || ring.Edges == nil || compRing.Edges == nil || len(ring.Edges) != len(compRing.Edges) {
		return false
	}
	for settingKey, edge := range ring.Edges {
		if compRing.Edges[settingKey].OrderSide != edge.OrderSide {
			return false
		}
	}
	return true
}

func (ring *Ring) SetBidAsk(settingKey string, bidAsk *model.BidAsk) bool {
	if ring.Edges == nil || ring.Edges[settingKey] == nil {
		return false
	}
	ring.Edges[settingKey].BidAsk = bidAsk
	// todo 计算总体利润和数量限制
	return true
}

func InitRingPool(settings []*model.Setting) {
	CompleteRings = make(map[string]*Ring)
	rings := make(map[string]*Ring)
	for _, setting := range settings {
		settingKey := setting.GetKey()
		ringKey := fmt.Sprintf(`%s_%s`, settingKey, model.OrderSideSell)
		rings[ringKey] = &Ring{
			LinkedCoins:    make(map[string]bool),
			CoinHead:       setting.Coin0,
			CoinTail:       setting.Coin1,
			Edges:          map[string]*Edge{settingKey: {OrderSide: model.OrderSideSell}},
			SortedSettings: []string{settingKey},
			Key:            ringKey}
		ringKey = fmt.Sprintf(`%s_%s`, settingKey, model.OrderSideBuy)
		rings[ringKey] = &Ring{
			LinkedCoins:    make(map[string]bool),
			CoinHead:       setting.Coin1,
			CoinTail:       setting.Coin0,
			Edges:          map[string]*Edge{settingKey: {OrderSide: model.OrderSideBuy}},
			SortedSettings: []string{settingKey},
			Key:            ringKey}
	}
	addStep(rings, settings)
}

// EnhanceRingPool 假定当前是n阶，增强为n+1阶
func addStep(stepRings map[string]*Ring, settings []*model.Setting) (nextRings map[string]*Ring) {
	nextRings = make(map[string]*Ring)
	for _, ring := range stepRings {
		for _, setting := range settings {
			copyRing := ring.Copy()
			if copyRing.AddSetting(setting) {
				if copyRing.Complete {
					CompleteRings[copyRing.Key] = copyRing
				} else {
					nextRings[copyRing.Key] = copyRing
				}
			}
		}
	}
	if len(nextRings) > 0 {
		fmt.Println(fmt.Sprintf(`level %d`, len(nextRings)))
		addStep(nextRings, settings)
	}
	return nextRings
}

func InitSettingRings() {
	SettingRings = make(map[string]map[string]*Ring)
	for ringKey, ring := range CompleteRings {
		for _, settingKey := range ring.SortedSettings {
			if SettingRings[settingKey] == nil {
				SettingRings[settingKey] = make(map[string]*Ring)
			}
			SettingRings[settingKey][ringKey] = ring
		}
	}
}

func SetRingBidAsk(market, symbol string, bidAsk *model.BidAsk) bool {
	settingKey := model.ComposeSettingKey(model.FunctionRing, market, symbol, `spot`)
	if SettingRings == nil || SettingRings[settingKey] == nil {
		return false
	}
	for _, ring := range SettingRings[settingKey] {
		ring.SetBidAsk(settingKey, bidAsk)
	}
	return true
}

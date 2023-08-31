package model

import (
	"fmt"
)

type Ring struct {
	Complete           bool // already be a ring or not
	LinkedCoins        map[string]bool
	CoinHead, CoinTail string
	SettingSide        map[string]string // settingKey - orderSide
	SortedSettings     []string
	Key                string
}

func (ring *Ring) Copy() (copy *Ring) {
	copy = &Ring{
		Complete:       ring.Complete,
		LinkedCoins:    make(map[string]bool),
		SettingSide:    make(map[string]string),
		SortedSettings: make([]string, 0),
		CoinHead:       ring.CoinHead,
		CoinTail:       ring.CoinTail,
		Key:            ring.Key}
	for coin, b := range ring.LinkedCoins {
		copy.LinkedCoins[coin] = b
	}
	for settingKey, side := range ring.SettingSide {
		copy.SettingSide[settingKey] = side
	}
	for i, settingKey := range ring.SortedSettings {
		copy.SortedSettings[i] = settingKey
	}
	return
}

func (ring *Ring) AddSetting(setting *Setting) (added bool) {
	if ring.LinkedCoins == nil {
		ring.LinkedCoins = make(map[string]bool)
	}
	if ring.SettingSide == nil {
		ring.SettingSide = make(map[string]string)
	}
	if ring.SortedSettings == nil {
		ring.SortedSettings = make([]string, 0)
	}
	if ring.Complete || ring.LinkedCoins[setting.Coin0] || ring.LinkedCoins[setting.Coin1] ||
		(ring.CoinHead != setting.Coin0 && ring.CoinHead != setting.Coin1 && ring.CoinTail != setting.Coin0 && ring.CoinTail != setting.Coin1) {
		return false
	}
	settingKey := setting.GetKey()
	if ring.CoinHead == setting.Coin0 {
		ring.SettingSide[settingKey] = OrderSideBuy
		ring.CoinHead = setting.Coin1
		ring.LinkedCoins[setting.Coin0] = true
		added = true
	}
	if ring.CoinHead == setting.Coin1 {
		ring.SettingSide[settingKey] = OrderSideSell
		ring.CoinHead = setting.Coin0
		ring.LinkedCoins[setting.Coin1] = true
		added = true
	}
	if ring.CoinTail == setting.Coin0 {
		ring.SettingSide[settingKey] = OrderSideSell
		ring.CoinTail = setting.Coin1
		ring.LinkedCoins[setting.Coin0] = true
		added = true
	}
	if ring.CoinTail == setting.Coin1 {
		ring.SettingSide[settingKey] = OrderSideBuy
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
	index := 0
	ring.Key = ``
	for i := 0; i < len(ring.SortedSettings); i++ {
		if ring.SortedSettings[i] < settingKey {
			sortedSettings[i] = ring.SortedSettings[i]
			ring.Key = fmt.Sprintf(`%s_%s_%v`, ring.Key, sortedSettings[i], ring.SettingSide[sortedSettings[i]])
		} else {
			index = i
			sortedSettings[i] = settingKey
			ring.Key = fmt.Sprintf(`%s_%s_%v`, ring.Key, sortedSettings[i], ring.SettingSide[sortedSettings[i]])
			break
		}
	}
	for i := index + 1; i < len(ring.SortedSettings); i++ {
		sortedSettings[i] = ring.SortedSettings[i]
		ring.Key = fmt.Sprintf(`%s_%s_%v`, ring.Key, sortedSettings[i], ring.SettingSide[sortedSettings[i]])
	}
	return added
}

func (ring *Ring) Equals(compRing *Ring) (isEqual bool) {
	if compRing == nil || ring.SettingSide == nil || compRing.SettingSide == nil || len(ring.SettingSide) != len(compRing.SettingSide) {
		return false
	}
	for settingKey, orderSide := range ring.SettingSide {
		if compRing.SettingSide[settingKey] != orderSide {
			return false
		}
	}
	return true
}

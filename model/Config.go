package model

import (
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	lock                                                                                           sync.Mutex
	ChannelSlot, Delay                                                                             float64
	KucoinSpot, GateSpot                                                                           bool
	KucoinRelatedKey, KucoinRelatedSecret, KucoinFutureKey, KucoinFutureSecret                     string
	GateKey, GateSecret, GateCarryClose, GateCarryRate                                             string
	HuobiKey, HuobiSecret, HuobiCarryClose, HuobiCarryRate                                         string
	OkexKey, OkexSecret, OkexCarryClose, OkexCarryRate                                             string
	FtxKey, FtxSecret, FtxCarryClose, FtxCarryRate                                                 string
	BybitKey, BybitSecret, BybitCarryClose, BybitCarryRate                                         string
	BinanceKey, BinanceSecret, BinanceCarryClose, BinanceCarryRate                                 string
	CoinparkKey, CoinparkSecret, CoinparkCarryClose, CoinparkCarryRate                             string
	DFutureKey, DFutureSecret                                                                      string
	BitmexKey, BitmexSecret, BitmexCarryClose, BitmexCarryRate                                     string
	Phase, Handle, Mail, FromMail, FromMailAuth, Port, WalletKey, DBConnection, Env, FutureAddress string
}

func (config *Config) GetCarrySetting(market string, index int) (closeCarry bool, rate float64) {
	carryRate := ``
	carryClose := ``
	closeCarry = false
	rate = 1
	switch market {
	case Kucoin:
		return false, 1
	case Gate:
		carryClose = config.GateCarryClose
		carryRate = config.GateCarryRate
	case Ftx:
		carryClose = config.FtxCarryClose
		carryRate = config.FtxCarryRate
	case Huobi, HuobiDM:
		carryClose = config.HuobiCarryClose
		carryRate = config.HuobiCarryRate
	case OKEX:
		carryClose = config.OkexCarryClose
		carryRate = config.OkexCarryRate
	case Binance:
		carryClose = config.BinanceCarryClose
		carryRate = config.BinanceCarryRate
	case Coinpark:
		carryClose = config.CoinparkCarryClose
		carryRate = config.CoinparkCarryRate
	case Bitmex:
		carryClose = config.BitmexCarryClose
		carryRate = config.BitmexCarryRate
	case Bybit:
		carryClose = config.BybitCarryClose
		carryRate = config.BybitCarryRate
	case DFuture:
		return false, 1
	}
	closeValues := strings.Split(carryClose, `,`)
	for i, str := range closeValues {
		if i == index {
			closeCarry = str == `true`
		}
	}
	rateValues := strings.Split(carryRate, `,`)
	for i, str := range rateValues {
		if i == index {
			rate, _ = strconv.ParseFloat(str, 64)
		}
	}
	return closeCarry, rate
}

func (config *Config) GetSecret(market, key string) (secret string) {
	keys, secrets := config.GetKeys(market)
	for i, value := range keys {
		if value == key {
			return secrets[i]
		}
	}
	return ``
}

func (config *Config) GetKey(market string, index int) (success bool, key, secret string) {
	keys, secrets := config.GetKeys(market)
	for i, value := range keys {
		if i == index {
			return true, value, secrets[i]
		}
	}
	return false, ``, ``
}

func (config *Config) GetKeys(market string) (keys, secrets []string) {
	switch market {
	case Kucoin:
		return strings.Split(config.KucoinRelatedSecret, `,`), strings.Split(config.KucoinRelatedSecret, `,`)
	case Gate:
		return strings.Split(config.GateKey, `,`), strings.Split(config.GateSecret, `,`)
	case Ftx:
		return strings.Split(config.FtxKey, `,`), strings.Split(config.FtxSecret, `,`)
	case Huobi, HuobiDM:
		return strings.Split(config.HuobiKey, `,`), strings.Split(config.HuobiSecret, `,`)
	case OKEX:
		return strings.Split(config.OkexKey, `,`), strings.Split(config.OkexSecret, `,`)
	case Binance:
		return strings.Split(config.BinanceKey, `,`), strings.Split(config.BinanceSecret, `,`)
	case Coinpark:
		return strings.Split(config.CoinparkKey, `,`), strings.Split(config.CoinparkSecret, `,`)
	case Bitmex:
		return strings.Split(config.BitmexKey, `,`), strings.Split(config.BitmexSecret, `,`)
	case Bybit:
		return strings.Split(config.BybitKey, `,`), strings.Split(config.BybitSecret, `,`)
	case DFuture:
		return strings.Split(config.DFutureKey, `,`), strings.Split(config.DFutureSecret, `,`)
	}
	return nil, nil
}

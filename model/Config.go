package model

import (
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	lock                                                                                           sync.Mutex
	Accounts                                                                                       int //账户个数
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

type Account struct {
	Key, Secret string
	CarryClose  bool
	CarryRate   float64
}

var accounts = make(map[string][]*Account)

func (config *Config) GetAccountFromKey(market, key string) (account *Account) {
	if accounts[market] == nil {
		config.GetAccount(market, 0)
	}
	if accounts[market] == nil {
		return nil
	}
	for _, item := range accounts[market] {
		if item.Key == key {
			return item
		}
	}
	return nil
}

func (config *Config) GetAccount(market string, index int) (account *Account) {
	if index >= config.Accounts {
		return nil
	}
	if accounts[market] != nil {
		return accounts[market][index]
	}
	accounts[market] = make([]*Account, config.Accounts)
	var rateValues, closeValues, keys, secrets []string
	switch market {
	//case Kucoin, DFuture:
	//	return false, 1
	case Gate:
		keys = strings.Split(config.GateKey, `,`)
		secrets = strings.Split(config.GateSecret, `,`)
		closeValues = strings.Split(config.GateCarryClose, `,`)
		rateValues = strings.Split(config.GateCarryRate, `,`)
	case Ftx:
		keys = strings.Split(config.FtxKey, `,`)
		secrets = strings.Split(config.FtxSecret, `,`)
		closeValues = strings.Split(config.FtxCarryClose, `,`)
		rateValues = strings.Split(config.FtxCarryRate, `,`)
	case Huobi, HuobiDM:
		keys = strings.Split(config.HuobiKey, `,`)
		secrets = strings.Split(config.HuobiSecret, `,`)
		closeValues = strings.Split(config.HuobiCarryClose, `,`)
		rateValues = strings.Split(config.HuobiCarryRate, `,`)
	case OKEX:
		keys = strings.Split(config.OkexKey, `,`)
		secrets = strings.Split(config.OkexSecret, `,`)
		closeValues = strings.Split(config.OkexCarryClose, `,`)
		rateValues = strings.Split(config.OkexCarryRate, `,`)
	case Binance:
		keys = strings.Split(config.BinanceKey, `,`)
		secrets = strings.Split(config.BinanceSecret, `,`)
		closeValues = strings.Split(config.BinanceCarryClose, `,`)
		rateValues = strings.Split(config.BinanceCarryRate, `,`)
	case Coinpark:
		keys = strings.Split(config.CoinparkKey, `,`)
		secrets = strings.Split(config.CoinparkSecret, `,`)
		closeValues = strings.Split(config.CoinparkCarryClose, `,`)
		rateValues = strings.Split(config.CoinparkCarryRate, `,`)
	case Bitmex:
		keys = strings.Split(config.BitmexKey, `,`)
		secrets = strings.Split(config.BitmexSecret, `,`)
		closeValues = strings.Split(config.BitmexCarryClose, `,`)
		rateValues = strings.Split(config.BitmexCarryRate, `,`)
	case Bybit:
		keys = strings.Split(config.BybitKey, `,`)
		secrets = strings.Split(config.BybitSecret, `,`)
		closeValues = strings.Split(config.BybitCarryClose, `,`)
		rateValues = strings.Split(config.BybitCarryRate, `,`)
	}
	for i := 0; i < config.Accounts; i++ {
		account = &Account{Key: keys[i], Secret: secrets[i]}
		account.CarryClose, _ = strconv.ParseBool(closeValues[index])
		account.CarryRate, _ = strconv.ParseFloat(rateValues[index], 64)
	}
	return accounts[market][index]
}

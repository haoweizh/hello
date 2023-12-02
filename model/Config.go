package model

import (
	"fmt"
	"hello/util"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Delay                                                                                            float64
	KucoinSpot, MetricTick                                                                           bool
	KucoinRelatedKey, KucoinRelatedSecret, KucoinFutureKey, KucoinFutureSecret                       string
	KucoinCarryClose, KucoinCarryRate, Simulation, Equal                                             string
	GateKey, GateSecret, GateCarryClose, GateCarryRate                                               string
	HuobiKey, HuobiSecret, HuobiCarryClose, HuobiCarryRate                                           string
	OkexKey, OkexSecret, OkexCarryClose, OkexCarryRate                                               string
	FtxKey, FtxSecret, FtxCarryClose, FtxCarryRate, RedisAddr, RedisPassword                         string
	BybitKey, BybitSecret, BybitCarryClose, BybitCarryRate                                           string
	BinanceKey, BinanceSecret, BinanceCarryClose, BinanceCarryRate                                   string
	CoinparkKey, CoinparkSecret, CoinparkCarryClose, CoinparkCarryRate                               string
	BitgetKey, BitgetSecret, BitgetCarryClose, BitgetCarryRate                                       string
	MexcKey, MexcSecret, MexcCarryClose, MexcCarryRate                                               string
	BitmexKey, BitmexSecret, BitmexCarryClose, BitmexCarryRate, FtxSubAccount, Phase                 string
	OKPhase, Handle, Mail, FromMail, FromMailAuth, Port, WalletKey, DBConnection, Env, FutureAddress string
}

type Account struct {
	Index                              int // 账户索引
	Market, Key, Secret, FtxSubAccount string
	CarryClose, IsUnified              bool
	CarryRate                          float64
}

var appAccounts []map[string]*Account // account index/map/account
var marketAccounts sync.Map           //market - []*Account

func (config *Config) GetAccountFromKeyIndex(market, key string, index int) (account *Account) {
	accounts := config.GetAccounts(market)
	if accounts == nil {
		return nil
	}
	for _, item := range accounts {
		if item == nil {
			return nil
		}
		if item.Key == key {
			return item
		}
		if index == item.Index {
			return item
		}
	}
	return nil
}

func (config *Config) GetAccounts(market string) []*Account {
	value, ok := marketAccounts.Load(market)
	if ok && value != nil {
		return value.([]*Account)
	}
	isUnified := false
	var rateValues, closeValues, keys, secrets, ftxSubAccounts []string
	switch market {
	case GXZQ:
		keys = []string{``}
		secrets = []string{``}
		closeValues = []string{`false`}
		rateValues = []string{`1`}
	//case Kucoin, DFuture:
	//	return false, 1
	case KucoinSpot:
		keys = strings.Split(config.KucoinRelatedKey, `,`)
		secrets = strings.Split(config.KucoinRelatedSecret, `,`)
		closeValues = strings.Split(config.KucoinCarryClose, `,`)
		rateValues = strings.Split(config.KucoinCarryRate, `,`)
	case KucoinPerp:
		keys = strings.Split(config.KucoinFutureKey, `,`)
		secrets = strings.Split(config.KucoinFutureSecret, `,`)
		closeValues = strings.Split(config.KucoinCarryClose, `,`)
		rateValues = strings.Split(config.KucoinCarryRate, `,`)
	case Gate:
		isUnified = true
		keys = strings.Split(config.GateKey, `,`)
		secrets = strings.Split(config.GateSecret, `,`)
		closeValues = strings.Split(config.GateCarryClose, `,`)
		rateValues = strings.Split(config.GateCarryRate, `,`)
	case Ftx:
		isUnified = true
		keys = strings.Split(config.FtxKey, `,`)
		secrets = strings.Split(config.FtxSecret, `,`)
		closeValues = strings.Split(config.FtxCarryClose, `,`)
		rateValues = strings.Split(config.FtxCarryRate, `,`)
		ftxSubAccounts = strings.Split(config.FtxSubAccount, `,`)
	case BitgetSpot, BitgetPerp:
		keys = strings.Split(config.BitgetKey, `,`)
		secrets = strings.Split(config.BitgetSecret, `,`)
		closeValues = strings.Split(config.BitgetCarryClose, `,`)
		rateValues = strings.Split(config.BitgetCarryRate, `,`)
	case OKEX:
		isUnified = true
		keys = strings.Split(config.OkexKey, `,`)
		secrets = strings.Split(config.OkexSecret, `,`)
		closeValues = strings.Split(config.OkexCarryClose, `,`)
		rateValues = strings.Split(config.OkexCarryRate, `,`)
	case BinanceSpot, BinancePerp, BinanceMargin:
		keys = strings.Split(config.BinanceKey, `,`)
		secrets = strings.Split(config.BinanceSecret, `,`)
		closeValues = strings.Split(config.BinanceCarryClose, `,`)
		rateValues = strings.Split(config.BinanceCarryRate, `,`)
	case Bitmex:
		keys = strings.Split(config.BitmexKey, `,`)
		secrets = strings.Split(config.BitmexSecret, `,`)
		closeValues = strings.Split(config.BitmexCarryClose, `,`)
		rateValues = strings.Split(config.BitmexCarryRate, `,`)
	case Bybit:
		isUnified = true
		keys = strings.Split(config.BybitKey, `,`)
		secrets = strings.Split(config.BybitSecret, `,`)
		closeValues = strings.Split(config.BybitCarryClose, `,`)
		rateValues = strings.Split(config.BybitCarryRate, `,`)
	case Mexc:
		keys = strings.Split(config.MexcKey, `,`)
		secrets = strings.Split(config.MexcSecret, `,`)
		closeValues = strings.Split(config.MexcCarryClose, `,`)
		rateValues = strings.Split(config.MexcCarryRate, `,`)
	}
	if len(keys) != len(secrets) || len(keys) != len(closeValues) || len(keys) != len(rateValues) {
		fmt.Println(fmt.Sprintf(`wrong config format %s keys:%d secrets:%d close:%d rate:%d`,
			market, len(keys), len(secrets), len(closeValues), len(rateValues)))
		os.Exit(1)
	}
	accounts := make([]*Account, len(keys))
	for i := 0; i < len(keys); i++ {
		account := &Account{Key: keys[i], Secret: secrets[i], Index: i, Market: market, IsUnified: isUnified}
		util.Notice(fmt.Sprintf(`create account %d %s %s`, account.Index, account.Market, account.Key))
		if market == Ftx {
			account.FtxSubAccount = ftxSubAccounts[i]
		}
		account.CarryClose, _ = strconv.ParseBool(closeValues[i])
		account.CarryRate, _ = strconv.ParseFloat(rateValues[i], 64)
		if len(strings.TrimSpace(account.Key)) > 0 || market == GXZQ {
			accounts[i] = account
		} else {
			accounts[i] = nil
		}
	}
	marketAccounts.Store(market, accounts)
	return accounts
}

func GetAccounts(index int) (accounts map[string]*Account) {
	if appAccounts != nil {
		if len(appAccounts) > index {
			return appAccounts[index]
		} else {
			return nil
		}
	}
	// 注意: 以okex的key个数作为size，如果不使用okex，请及时更换
	size := len(OKEX)
	appAccounts = make([]map[string]*Account, size)
	for i := 0; i < size; i++ {
		if appAccounts[i] == nil {
			appAccounts[i] = make(map[string]*Account)
		}
	}
	tempAccounts := AppConfig.GetAccounts(Ftx)
	for i, account := range tempAccounts {
		appAccounts[i][Ftx] = account
	}
	tempAccounts = AppConfig.GetAccounts(OKEX)
	for i, account := range tempAccounts {
		appAccounts[i][OKEX] = account
	}
	tempAccounts = AppConfig.GetAccounts(BinanceSpot)
	for i, account := range tempAccounts {
		appAccounts[i][BinanceSpot] = account
	}
	tempAccounts = AppConfig.GetAccounts(BinancePerp)
	for i, account := range tempAccounts {
		appAccounts[i][BinancePerp] = account
	}
	tempAccounts = AppConfig.GetAccounts(Gate)
	for i, account := range tempAccounts {
		appAccounts[i][Gate] = account
	}
	tempAccounts = AppConfig.GetAccounts(Bybit)
	for i, account := range tempAccounts {
		appAccounts[i][Bybit] = account
	}
	tempAccounts = AppConfig.GetAccounts(Kucoin)
	for i, account := range tempAccounts {
		appAccounts[i][Kucoin] = account
	}
	tempAccounts = AppConfig.GetAccounts(KucoinSpot)
	for i, account := range tempAccounts {
		appAccounts[i][KucoinSpot] = account
	}
	tempAccounts = AppConfig.GetAccounts(KucoinPerp)
	for i, account := range tempAccounts {
		appAccounts[i][KucoinPerp] = account
	}
	tempAccounts = AppConfig.GetAccounts(Mexc)
	for i, account := range tempAccounts {
		appAccounts[i][Mexc] = account
	}
	tempAccounts = AppConfig.GetAccounts(BitgetSpot)
	for i, account := range tempAccounts {
		appAccounts[i][BitgetSpot] = account
	}
	tempAccounts = AppConfig.GetAccounts(BitgetPerp)
	for i, account := range tempAccounts {
		appAccounts[i][BitgetPerp] = account
	}
	return appAccounts[index]
}

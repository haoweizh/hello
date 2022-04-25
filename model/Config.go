package model

import (
	"fmt"
	"hello/util"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
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
	DFutureKey, DFutureSecret, MexcKey, MexcSecret, MexcCarryClose, MexcCarryRate                  string
	BitmexKey, BitmexSecret, BitmexCarryClose, BitmexCarryRate                                     string
	Phase, Handle, Mail, FromMail, FromMailAuth, Port, WalletKey, DBConnection, Env, FutureAddress string
}

type Account struct {
	Key, Secret string
	CarryClose  bool
	CarryRate   float64
}

var crossLen int
var AppAccounts []map[string]*Account // account index/map/account
var marketAccounts sync.Map           //market - []*Account

func GetAccounts(index int) (accounts map[string]*Account) {
	if AppAccounts != nil && len(AppAccounts) > index {
		return AppAccounts[index]
	}
	tempAccounts := AppConfig.GetAccounts(Ftx)
	size := int(math.Max(float64(AppConfig.GetCrossLen()), float64(len(tempAccounts))))
	AppAccounts = make([]map[string]*Account, size)
	for i := 0; i < size; i++ {
		if AppAccounts[i] == nil {
			AppAccounts[i] = make(map[string]*Account)
		}
	}
	for i, account := range tempAccounts {
		AppAccounts[i][Ftx] = account
	}
	tempAccounts = AppConfig.GetAccounts(OKEX)
	for i, account := range tempAccounts {
		AppAccounts[i][OKEX] = account
	}
	tempAccounts = AppConfig.GetAccounts(BinanceSpot)
	for i, account := range tempAccounts {
		AppAccounts[i][BinanceSpot] = account
	}
	tempAccounts = AppConfig.GetAccounts(BinancePerp)
	for i, account := range tempAccounts {
		AppAccounts[i][BinancePerp] = account
	}
	tempAccounts = AppConfig.GetAccounts(Gate)
	for i, account := range tempAccounts {
		AppAccounts[i][Gate] = account
	}
	tempAccounts = AppConfig.GetAccounts(BybitPerp)
	for i, account := range tempAccounts {
		AppAccounts[i][BybitPerp] = account
	}
	tempAccounts = AppConfig.GetAccounts(BybitSpot)
	for i, account := range tempAccounts {
		AppAccounts[i][BybitSpot] = account
	}
	tempAccounts = AppConfig.GetAccounts(Kucoin)
	for i, account := range tempAccounts {
		AppAccounts[i][Kucoin] = account
	}
	tempAccounts = AppConfig.GetAccounts(Mexc)
	for i, account := range tempAccounts {
		AppAccounts[i][Mexc] = account
	}
	return AppAccounts[index]
}

func (config *Config) GetCrossLen() int {
	if crossLen > 0 {
		return crossLen
	}
	markets := GetMarkets()
	for _, market := range markets {
		accounts := config.GetAccounts(market)
		if crossLen == 0 {
			crossLen = len(accounts)
		} else if len(accounts) != crossLen {
			util.Notice(fmt.Sprintf(`wrong cross config %s accounts:%d`, market, len(accounts)))
			os.Exit(2)
		}
	}
	return crossLen
}

//// GetCrossAccounts markets: market - bool
//func (config *Config) GetCrossAccounts(markets map[string]bool) (crossAccounts []map[string]*Account) {
//	for market := range markets {
//		accounts := config.GetAccounts(market)
//		if crossAccounts == nil {
//			crossAccounts = make([]map[string]*Account, len(accounts))
//		} else if len(crossAccounts) != len(accounts) {
//			fmt.Println(fmt.Sprintf(`wrong cross config %s keys:%d accounts:%d`, market, len(accounts), len(accounts)))
//			os.Exit(2)
//		}
//		for i, account := range accounts {
//			if crossAccounts[i] == nil {
//				crossAccounts[i] = make(map[string]*Account)
//			}
//			crossAccounts[i][market] = account
//		}
//	}
//	return
//}

func (config *Config) GetAccountFromKey(market, key string) (account *Account) {
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
	}
	return nil
}

func (config *Config) GetAccounts(market string) []*Account {
	value, ok := marketAccounts.Load(market)
	if ok && value != nil {
		return value.([]*Account)
	}
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
	case OKEX:
		keys = strings.Split(config.OkexKey, `,`)
		secrets = strings.Split(config.OkexSecret, `,`)
		closeValues = strings.Split(config.OkexCarryClose, `,`)
		rateValues = strings.Split(config.OkexCarryRate, `,`)
	case BinanceSpot, BinancePerp:
		keys = strings.Split(config.BinanceKey, `,`)
		secrets = strings.Split(config.BinanceSecret, `,`)
		closeValues = strings.Split(config.BinanceCarryClose, `,`)
		rateValues = strings.Split(config.BinanceCarryRate, `,`)
	case Bitmex:
		keys = strings.Split(config.BitmexKey, `,`)
		secrets = strings.Split(config.BitmexSecret, `,`)
		closeValues = strings.Split(config.BitmexCarryClose, `,`)
		rateValues = strings.Split(config.BitmexCarryRate, `,`)
	case BybitPerp, BybitSpot:
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
		account := &Account{Key: keys[i], Secret: secrets[i]}
		account.CarryClose, _ = strconv.ParseBool(closeValues[i])
		account.CarryRate, _ = strconv.ParseFloat(rateValues[i], 64)
		if len(strings.TrimSpace(account.Key)) > 0 {
			accounts[i] = account
		} else {
			accounts[i] = nil
		}
	}
	marketAccounts.Store(market, accounts)
	return accounts
}

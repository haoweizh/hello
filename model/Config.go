package model

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Delay                         float64
	Debug, KucoinSpot, MetricTick bool
	// SpecialChan 1 代表使用特殊通道
	SpecialChan, CrossStyle, MoneyPerStep                                                            string
	KucoinRelatedKey, KucoinRelatedSecret, KucoinFutureKey, KucoinFutureSecret                       string
	KucoinCarryClose, KucoinCarryRate, Simulation, Equal, Log                                        string
	GateKey, GateSecret, GateCarryClose, GateCarryRate, GateLeverMax, GateLeverMin, GateRiskLimit    string
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
	Index                                                              int // 账户索引
	Market, Key, Secret, FtxSubAccount, CrossStyle                     string
	CarryClose, IsUnified                                              bool
	CarryRate, GateLeverMax, GateLeverMin, GateRiskLimit, MoneyPerStep float64
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
	if market != Gate && market != BinanceSpot && market != BinancePerp && market != BinanceMargin && market != BitgetSpot &&
		market != BitgetPerp && market != OKEX && market != Bybit {
		return nil
	}
	value, ok := marketAccounts.Load(market)
	if ok && value != nil {
		return value.([]*Account)
	}
	isUnified := false
	var rateValues, closeValues, keys, secrets, gateLeverMax, gateLeverMin, gateRiskLimit []string
	// ftxSubAccounts
	moneyPerSteps := strings.Split(config.MoneyPerStep, ",")
	crossStyles := strings.Split(config.CrossStyle, `,`)
	switch market {
	case Gate:
		isUnified = true
		keys = strings.Split(config.GateKey, `,`)
		secrets = strings.Split(config.GateSecret, `,`)
		closeValues = strings.Split(config.GateCarryClose, `,`)
		rateValues = strings.Split(config.GateCarryRate, `,`)
		gateLeverMax = strings.Split(config.GateLeverMax, `,`)
		gateLeverMin = strings.Split(config.GateLeverMin, `,`)
		gateRiskLimit = strings.Split(config.GateRiskLimit, `,`)
		if len(keys) != len(gateLeverMin) || len(gateLeverMin) != len(gateLeverMax) || len(gateLeverMax) != len(gateRiskLimit) {
			fmt.Println(fmt.Sprintf(`wrong config format %s keys:%d lever min:%d lever max:%d limit:%d`,
				market, len(keys), len(gateLeverMin), len(gateLeverMax), len(gateRiskLimit)))
			os.Exit(1)
		}
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
	case Bybit:
		isUnified = true
		keys = strings.Split(config.BybitKey, `,`)
		secrets = strings.Split(config.BybitSecret, `,`)
		closeValues = strings.Split(config.BybitCarryClose, `,`)
		rateValues = strings.Split(config.BybitCarryRate, `,`)
	case BinanceSpot, BinancePerp:
		keys = strings.Split(config.BinanceKey, `,`)
		secrets = strings.Split(config.BinanceSecret, `,`)
		closeValues = strings.Split(config.BinanceCarryClose, `,`)
		rateValues = strings.Split(config.BinanceCarryRate, `,`)
	}
	if len(keys) != len(secrets) || len(keys) != len(closeValues) || len(keys) != len(rateValues) ||
		len(rateValues) != len(crossStyles) || len(crossStyles) != len(moneyPerSteps) {
		fmt.Println(fmt.Sprintf(`wrong config format %s keys:%d secrets:%d close:%d rate:%d crossStyle:%d moneyPerSteps:%d`,
			market, len(keys), len(secrets), len(closeValues), len(rateValues), len(crossStyles), len(moneyPerSteps)))
		os.Exit(1)
	}
	accounts := make([]*Account, len(keys))
	for i := 0; i < len(keys); i++ {
		account := &Account{Key: keys[i], Secret: secrets[i], Index: i, Market: market, IsUnified: isUnified, CrossStyle: crossStyles[i]}
		account.MoneyPerStep, _ = strconv.ParseFloat(moneyPerSteps[i], 64)
		//if market == Ftx {
		//	account.FtxSubAccount = ftxSubAccounts[i]
		//}
		if market == Gate {
			account.GateLeverMax, _ = strconv.ParseFloat(gateLeverMax[i], 64)
			account.GateLeverMin, _ = strconv.ParseFloat(gateLeverMin[i], 64)
			account.GateRiskLimit, _ = strconv.ParseFloat(gateRiskLimit[i], 64)
		}
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

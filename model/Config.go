package model

import (
	"strings"
	"sync"
)

type Config struct {
	lock           sync.Mutex
	ChannelSlot    float64
	CarryClose     string
	Delay          float64
	AccountRate    string // 不同账户开仓门槛比例
	DBConnection   string
	Env            string
	FutureAddress  string
	HuobiKey       string
	HuobiSecret    string
	OkexKey        string
	OkexSecret     string
	FtxKey         string
	FtxSecret      string
	BinanceKey     string
	BinanceSecret  string
	CoinparkKey    string
	CoinparkSecret string
	DFutureKey     string
	DFutureSecret  string
	BitmexKey      string
	BitmexSecret   string
	BybitKey       string
	BybitSecret    string
	Phase          string
	Handle         string // 0 不执行处理程序，1执行处理程序
	Mail           string
	FromMail       string
	FromMailAuth   string
	Port           string
	WalletKey      string
}

func (config *Config) GetKeys(market string) (keys, secrets []string) {
	config.lock.Lock()
	defer config.lock.Unlock()
	switch market {
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

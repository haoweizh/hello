package model

import (
	"strings"
	"sync"
)

type Config struct {
	lock            sync.Mutex
	Channels        int
	InChina         int // 1 in china, otherwise outter china
	RefreshTimeSlot int
	Between         int64
	ChannelSlot     float64
	Delay           float64
	PreDealDis      float64
	AmountRate      float64 // 刷单填写数量比率
	BinanceOrderDis float64
	Amount          float64
	WSUrls          map[string]string // marketName - ws url
	RestUrls        map[string]string // marketName - rest url
	DBConnection    string
	Env             string
	HuobiKey        string
	HuobiSecret     string
	OkexKey         string
	OkexSecret      string
	FtxKey          string
	FtxSecret       string
	BinanceKey      string
	BinanceSecret   string
	CoinparkKey     string
	CoinparkSecret  string
	BitmexKey       string
	BitmexSecret    string
	BybitKey        string
	BybitSecret     string
	Phase           string
	Handle          string // 0 不执行处理程序，1执行处理程序
	Mail            string
	FromMail        string
	FromMailAuth    string
	Port            string
	SymbolPrice     map[string]float64 // symbol - price
	UpdatePriceTime map[string]int64   // symbol -time
}

func (config *Config) SetSymbolPrice(symbol string, price float64) {
	config.lock.Lock()
	defer config.lock.Unlock()
	config.SymbolPrice[symbol] = price
}

func (config *Config) SetUpdatePriceTime(symbol string, updateTime int64) {
	config.lock.Lock()
	defer config.lock.Unlock()
	config.UpdatePriceTime[symbol] = updateTime
}

func (config *Config) GetKeys(market string) (keys, secrets []string) {
	config.lock.Lock()
	defer config.lock.Unlock()
	switch market {
	case Ftx:
		return strings.Split(config.FtxKey, `,`), strings.Split(config.FtxSecret, `,`)
	case Huobi, HuobiDM:
		return strings.Split(config.HuobiKey, `,`), strings.Split(config.HuobiSecret, `,`)
	case OKEX, OKFUTURE:
		return strings.Split(config.OkexKey, `,`), strings.Split(config.OkexSecret, `,`)
	case Binance:
		return strings.Split(config.BinanceKey, `,`), strings.Split(config.BinanceSecret, `,`)
	case Coinpark:
		return strings.Split(config.CoinparkKey, `,`), strings.Split(config.CoinparkSecret, `,`)
	case Bitmex:
		return strings.Split(config.BitmexKey, `,`), strings.Split(config.BitmexSecret, `,`)
	case Bybit:
		return strings.Split(config.BybitKey, `,`), strings.Split(config.BybitSecret, `,`)
	}
	return nil, nil
}

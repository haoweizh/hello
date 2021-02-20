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
	huobiKey        string
	huobiSecret     string
	okexKey         string
	okexSecret      string
	ftxKey          string
	ftxSecret       string
	binanceKey      string
	binanceSecret   string
	coinbigKey      string
	coinbigSecret   string
	coinparkKey     string
	coinparkSecret  string
	bitmexKey       string
	bitmexSecret    string
	bybitKey        string
	bybitSecret     string
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
		return strings.Split(config.ftxKey, `,`), strings.Split(config.ftxSecret, `,`)
	case Huobi, HuobiDM:
		return strings.Split(config.huobiKey, `,`), strings.Split(config.huobiSecret, `,`)
	case OKEX, OKFUTURE, OKSwap:
		return strings.Split(config.okexKey, `,`), strings.Split(config.okexSecret, `,`)
	case Binance:
		return strings.Split(config.binanceKey, `,`), strings.Split(config.binanceSecret, `,`)
	case Coinbig:
		return strings.Split(config.coinbigKey, `,`), strings.Split(config.coinbigSecret, `,`)
	case Coinpark:
		return strings.Split(config.coinparkKey, `,`), strings.Split(config.coinparkSecret, `,`)
	case Bitmex:
		return strings.Split(config.bitmexKey, `,`), strings.Split(config.bitmexSecret, `,`)
	case Bybit:
		return strings.Split(config.bybitKey, `,`), strings.Split(config.bybitSecret, `,`)
	}
	return nil, nil
}

package model

import (
	"fmt"
	"github.com/jinzhu/configor"
	"gorm.io/gorm"
	"hello/util"
	"sort"
	"strings"
	"sync"
	"time"
)

type PostOrder func(order *Order, setting *Setting) // 处理下单后的函数
var HandlerMap = make(map[string]CarryHandler)
var infoLock sync.Mutex
var Currencies = []string{`btc`, `eth`, `usdt`, `pax`, `usdc`, `tusd`}
var TeamMails = []string{`13581512402@139.com`, `haoweizh@qq.com`}
var CarryInfo = make(map[string]map[string]string) // userKey - function - msg
var AppMetric = &MetricManager{}

const OKEXBTCContractFaceValue = 100.0
const OKEXOtherContractFaceValue = 10.0
const Kucoin = `kucoin`
const Gate = `gate`
const DFuture = `dfuture`
const Bybit = `bybit`
const OKEX = "okex"
const Huobi = "huobi"
const HuobiDM = `huobiDM`
const Binance = "binance"
const Ftx = `ftx`
const Coinpark = "coinpark"
const Bitmex = `bitmex`
const SubscribeDepth = `SubscribeDepth`
const SubscribeTicker = `ticker`
const SubscribeDeal = `subscribeDeal`
const CarryStatusSuccess = "success"
const CarryStatusFail = "fail"
const CarryStatusWorking = "working"
const OrderTypeLimit = `limit`
const OrderTypeMarket = `market`
const OrderTypeStop = `stop`
const OrderSideBuy = `buy`
const OrderSideSell = `sell`
const OrderSideLiquidateLong = `liquidateLong`
const OrderSideLiquidateShort = `liquidateShort`
const FunctionHang = `hang`
const FunctionTurtle = `turtle`
const FunctionGrid = `grid`
const FunctionCarry = `carry`
const FunctionCross = `cross`
const FunctionRefresh = `refresh`

const FunctionDCarry = `dcarry`
const FunctionComplement = `comp`
const FunctionPostonlyHandler = `postonly`
const PostOnly = `ParticipateDoNotInitiate`

var AppDB *gorm.DB
var AppSettings []Setting
var AppConfig *Config
var AppMarkets = NewMarkets()

var AppPause = false

func IsTickTimeout(market string, delay int64) (timeout bool) {
	switch market {
	case Gate:
		return delay > 40
	case Binance, Ftx:
		return delay > 100
	case OKEX, Huobi, Kucoin:
		return delay > 25
	}
	return true
}

func IsRelatedTickTimeout(market string, delayRelated int64) (timeout bool) {
	switch market {
	case Binance:
		return delayRelated > 100
	case OKEX, Ftx, Huobi:
		return delayRelated > 300
	case Gate:
		return delayRelated > 30000
	case Kucoin:
		return delayRelated > 1000
	}
	return true
}

func GetDialectSymbol(market, symbol string) (dialectSymbol string) {
	switch market {
	case Bitmex:
		symbol = strings.Replace(symbol, `btc`, `xbt`, -1)
		return strings.ToUpper(strings.Split(symbol, `_`)[0])
	case Bybit:
		return strings.ToUpper(strings.Split(symbol, `_`)[0])
	}
	return ``
}

func GetStandardSymbol(market, symbol string) (standardSymbol string) {
	symbol = strings.ToLower(symbol)
	switch market {
	case Bitmex:
		return strings.Replace(symbol, `xbt`, `btc`, -1) + `_p`
	case Bybit:
		return symbol + `_p`
	}
	return standardSymbol
}

var orderStatusMap = map[string]map[string]string{ // market - market status - united status
	Binance: {
		"NEW":              CarryStatusWorking,
		"PARTIALLY_FILLED": CarryStatusWorking,
		"PENDING_CANCEL":   CarryStatusWorking,
		"FILLED":           CarryStatusSuccess,
		"CANCELED":         CarryStatusFail,
		"REJECTED":         CarryStatusFail,
		"EXPIRED":          CarryStatusFail},
	Huobi: {
		`submitting`:       CarryStatusWorking, //已提交
		`submitted`:        CarryStatusWorking, //已提交,
		`partial-filled`:   CarryStatusWorking, //部分成交,
		`partial-canceled`: CarryStatusSuccess, //部分成交撤销,
		`filled`:           CarryStatusSuccess, //完全成交,
		`canceled`:         CarryStatusFail},   //已撤销
	Bitmex: {
		"New":             CarryStatusWorking,
		"PartiallyFilled": CarryStatusWorking,
		"Filled":          CarryStatusSuccess,
		"DoneForDay":      CarryStatusWorking,
		"Canceled":        CarryStatusFail,
		"PendingCancel":   CarryStatusWorking,
		"Stopped":         CarryStatusFail,
		"Rejected":        CarryStatusFail,
		"PendingNew":      CarryStatusWorking,
		"Expired":         CarryStatusFail,
	},
	Coinpark: {
		`1`: CarryStatusWorking, //待成交
		`2`: CarryStatusSuccess, //部分成交
		`3`: CarryStatusSuccess, //完全成交
		`4`: CarryStatusSuccess, //部分撤销
		`5`: CarryStatusFail,    //完全撤销
		`6`: CarryStatusWorking, //待撤销
	},
	Bybit: {
		`Created`:         CarryStatusWorking,
		`New`:             CarryStatusWorking,
		`PartiallyFilled`: CarryStatusWorking,
		`Filled`:          CarryStatusSuccess,
		`Cancelled`:       CarryStatusFail,
		`Rejected`:        CarryStatusFail,
		`PendingCancel`:   CarryStatusWorking,
		`Deactivated`:     CarryStatusFail,
	},
	Ftx: {
		`new`:       CarryStatusWorking,
		`open`:      CarryStatusWorking,
		`closed`:    CarryStatusSuccess,
		`cancelled`: CarryStatusFail,
		`triggered`: CarryStatusSuccess,
	},
}

func GetCarryInfo(userKey, key string) (info string) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if key == `` {
		items := make([]string, 0)
		for item := range CarryInfo[userKey] {
			items = append(items, item)
		}
		sort.Strings(items)

		for _, item := range items {
			info += CarryInfo[userKey][item] + "\n"
		}
	} else {
		return CarryInfo[userKey][key]
	}
	return info
}

// SetCarryInfo userKey[0] vs slaves
func SetCarryInfo(userKey, key, value string) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if CarryInfo[userKey] == nil {
		CarryInfo[userKey] = make(map[string]string)
	}
	CarryInfo[userKey][key] = value
}

func RemoveCarryInfo(userKey, key string) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if CarryInfo[userKey] != nil {
		delete(CarryInfo[userKey], key)
	}
}

func GetOrderStatus(market, marketStatus string) (status string) {
	if orderStatusMap[market] == nil {
		return CarryStatusWorking
	}
	if orderStatusMap[market][marketStatus] == `` {
		return CarryStatusWorking
	}
	return orderStatusMap[market][marketStatus]
}

func GetSymbolWithSplit(original, split string) (symbol string) {
	original = strings.ToLower(original)
	for _, currency := range Currencies {
		if strings.Contains(original, currency) && strings.LastIndex(original, currency)+len(currency) == len(original) {
			return original[0:strings.LastIndex(original, currency)] + split + currency
		}
	}
	util.Notice(`can not parse symbol for currency absent ` + original)
	return ``
}

func NewConfig() {
	AppConfig = &Config{GateSpot: true, KucoinSpot: true}
	err := configor.Load(AppConfig, "./config.yml")
	if err != nil {
		util.Notice(err.Error())
		return
	}
}

func GetMarketYesterday(market string) (yesterday time.Time, strYesterday string) {
	yesterday = time.Now().In(time.UTC)
	if market == HuobiDM || market == OKEX {
		yesterday = util.GetNow()
	}
	duration, _ := time.ParseDuration(`-24h`)
	yesterday = yesterday.Add(duration)
	yesterday = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	return yesterday, yesterday.String()[0:10]
}

func GetMarketToday(market string) (today time.Time, strToday string) {
	today = time.Now().In(time.UTC)
	if market == OKEX || market == HuobiDM {
		today = util.GetNow()
	}
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	return today, today.String()[0:10]
}

func (config *Config) ToString() string {
	str := "markets-carry cost:\n"
	str += fmt.Sprintf("delay: %f\n", config.Delay)
	str += fmt.Sprintf("channelslot: %f\n", config.ChannelSlot)
	str += fmt.Sprintf("handle: %s\n", config.Handle)
	return str
}

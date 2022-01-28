package model

import (
	"fmt"
	"github.com/jinzhu/configor"
	"gorm.io/gorm"
	"hello/util"
	"sync"
	"time"
)

type PostOrder func(order *Order, setting *Setting) // 处理下单后的函数
var HandlerMap = make(map[string]CarryHandler)
var infoLock sync.Mutex
var TeamMails = []string{`13581512402@139.com`, `haoweizh@qq.com`}
var CarryInfo = make(map[string]map[string]string)                // userKey - function - msg
var monitorInfo = make(map[string]map[string]map[string][]string) // userKey - table - item - value array
var AppMetric = &MetricManager{}

const Kucoin = `kucoin`
const Gate = `gate`
const Mexc = `mexc`
const DFuture = `dfuture`
const BybitSpot = `bybitspot`
const BybitPerp = `bybitperp`
const OKEX = "okex"
const Binance = "binance"
const Ftx = `ftx`
const Bitmex = `bitmex`
const SubscribeDepth = `SubscribeDepth`
const SubscribeTicker = `ticker`
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
const FunctionTurtle = `turtle`
const FunctionGrid = `grid`
const FunctionCarry = `carry`
const FunctionCross = `cross`
const MarketTypePerp = `perp`
const MarketTypeSpot = `spot`
const FunctionDCarry = `dcarry`
const FunctionComplement = `comp`
const PostOnly = `ParticipateDoNotInitiate`

//const SubscribeDeal = `subscribeDeal`
//const FunctionHang = `hang`
//const FunctionPostonlyHandler = `postonly`
//const OKEXBTCContractFaceValue = 100.0
//const OKEXOtherContractFaceValue = 10.0

var AppDB *gorm.DB
var AppSettings []Setting
var AppConfig *Config
var AppMarkets = NewMarkets()
var AppPause = false
var DialectTail = map[string]map[string]string{
	MarketTypeSpot: {Gate: `_USDT`, Ftx: `/USD`, OKEX: `-USDT`, BybitSpot: `USDT`, Binance: `USDT`},
	MarketTypePerp: {Gate: `_USDT`, Ftx: `-PERP`, OKEX: `-USDT-SWAP`, BybitPerp: `USDT`, Binance: `USDT`}}
var UniStandardTail = map[string]string{MarketTypeSpot: `_USDT`, MarketTypePerp: `_PERP`}

// GetCoinFromStandard from uni-tail formatted symbol
func GetCoinFromStandard(standardSymbol string) (success bool, marketType, coin string) {
	for mType, tail := range UniStandardTail {
		coinLen := len(standardSymbol) - len(tail)
		if coinLen > 0 && standardSymbol[coinLen:] == tail {
			return true, mType, standardSymbol[0 : len(standardSymbol)-len(tail)]
		}
	}
	util.Notice(`fail to parse standard symbol %s`, standardSymbol)
	return false, ``, ``
}

func GetStandardFromDialect(marketType, market, dialectSymbol string) (success bool, coin string) {
	if DialectTail[marketType] != nil && DialectTail[marketType][market] != `` {
		dialectTail := DialectTail[marketType][market]
		coinLen := len(dialectSymbol) - len(dialectTail)
		if coinLen > 0 && dialectSymbol[coinLen:] == dialectTail {
			return true, dialectSymbol[0:coinLen] + UniStandardTail[marketType] + UniStandardTail[marketType]
		}
	}
	util.Notice(`fail to GetStandardSymbol %s %s %s`, marketType, market, dialectSymbol)
	return false, ``
}

func GetDialectFromStandard(market, standardSymbol string) (success bool, dialect string) {
	success, marketType, coin := GetCoinFromStandard(standardSymbol)
	if success && DialectTail[marketType] != nil && DialectTail[marketType][market] != `` {
		return true, coin + DialectTail[marketType][market]
	}
	util.Notice(`fail to GetDialect %s %s %s`, marketType, market, standardSymbol)
	return false, ``
}

func IsTickTimeout(market string, delay int64) (timeout bool) {
	switch market {
	case OKEX, Gate, Ftx:
		return delay > 40
	case Binance:
		return delay > 100
	case Kucoin:
		return delay > 25
	}
	return true
}

func IsRelatedTickTimeout(market string, delayRelated int64) (timeout bool) {
	switch market {
	case Binance:
		return delayRelated > 100
	case OKEX, Ftx:
		return delayRelated > 300
	case Kucoin, Gate:
		return delayRelated > 1000
	}
	return true
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
		"Expired":         CarryStatusFail},
	BybitPerp: {
		`Created`:         CarryStatusWorking,
		`New`:             CarryStatusWorking,
		`PartiallyFilled`: CarryStatusWorking,
		`Filled`:          CarryStatusSuccess,
		`Cancelled`:       CarryStatusFail,
		`Rejected`:        CarryStatusFail,
		`PendingCancel`:   CarryStatusWorking},
	BybitSpot: {
		`NEW`:              CarryStatusWorking,
		`PARTIALLY_FILLED`: CarryStatusWorking,
		`FILLED`:           CarryStatusSuccess,
		`CANCELED`:         CarryStatusFail,
		`PENDING_CANCEL`:   CarryStatusFail,
		`PENDING_NEW`:      CarryStatusWorking,
		`REJECTED`:         CarryStatusFail},
	Ftx: {
		`new`:       CarryStatusWorking,
		`open`:      CarryStatusWorking,
		`closed`:    CarryStatusSuccess,
		`cancelled`: CarryStatusFail,
		`triggered`: CarryStatusSuccess},
}

func GetMonitorInfo(key, table string) (valueArray [][]string) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if monitorInfo == nil || monitorInfo[key] == nil || monitorInfo[key][table] == nil {
		return nil
	}
	valueMap := monitorInfo[key][table]
	valueArray = make([][]string, len(valueMap))
	i := 0
	for _, value := range valueMap {
		valueArray[i] = value
		i++
	}
	for i = len(valueArray) - 1; i > 0; i-- {
		for j := 0; j < i; j++ {
			if valueArray[j][0] > valueArray[i][0] {
				valueArray[j], valueArray[i] = valueArray[i], valueArray[j]
			}
		}
	}
	return valueArray
}

func SetMonitorInfo(key, table, item string, value []string) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if monitorInfo == nil {
		monitorInfo = map[string]map[string]map[string][]string{}
	}
	if monitorInfo[key] == nil {
		monitorInfo[key] = map[string]map[string][]string{}
	}
	if monitorInfo[key][table] == nil {
		monitorInfo[key][table] = map[string][]string{}
	}
	monitorInfo[key][table][item] = value
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

//func RemoveCarryInfo(userKey, key string) {
//	infoLock.Lock()
//	defer infoLock.Unlock()
//	if CarryInfo[userKey] != nil {
//		delete(CarryInfo[userKey], key)
//	}
//}

func GetOrderStatus(market, marketStatus string) (status string) {
	if orderStatusMap[market] == nil {
		return CarryStatusWorking
	}
	if orderStatusMap[market][marketStatus] == `` {
		return CarryStatusWorking
	}
	return orderStatusMap[market][marketStatus]
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
	if market == OKEX {
		yesterday = util.GetNow()
	}
	duration, _ := time.ParseDuration(`-24h`)
	yesterday = yesterday.Add(duration)
	yesterday = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	return yesterday, yesterday.String()[0:10]
}

func GetMarketToday(market string) (today time.Time, strToday string) {
	today = time.Now().In(time.UTC)
	if market == OKEX {
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

//func GetCoin(market, symbol string) (coin string) {
//	var tails []string
//	switch market {
//	case Ftx:
//		tails = []string{`/USD`, `-PERP`}
//	case OKEX:
//		tails = []string{`-USDT`, `-USDT-SWAP`}
//	case Gate:
//		tails = []string{`_USDT`, `_PERP`}
//	case Kucoin:
//		tails = []string{`-USDT`, `-PERP`}
//	case Binance:
//		tails = []string{`-PERP`}
//	case BybitPerp:
//		tails = []string{`-PERP`}
//	case BybitSpot:
//		tails = []string{`-USDT`}
//	case Huobi:
//		tails = []string{`usdt`, `-usdt`}
//	}
//	for _, tail := range tails {
//		if symbol[len(symbol)-len(tail):] == tail {
//			return symbol[0 : len(symbol)-len(tail)]
//		}
//	}
//	return
//}
//
//func GetDialectPerp(market, symbol string) (dialectSymbol string) {
//	switch market {
//	case Bitmex:
//		symbol = strings.Replace(symbol, `btc`, `xbt`, -1)
//		return strings.ToUpper(strings.Split(symbol, `_`)[0])
//	case BybitPerp:
//		tail := GetPerpTail(market)
//		if len(symbol) > len(tail) && symbol[len(symbol)-len(tail):] == tail {
//			return symbol[0:len(symbol)-len(tail)] + `USDT`
//		}
//	case BybitSpot:
//		tail := GetSpotTail(market)
//		if len(symbol) > len(tail) && symbol[len(symbol)-len(tail):] == tail {
//			return symbol[0:len(symbol)-len(tail)] + `USDT`
//		}
//	}
//	return ``
//}
//
//func GetDialectSpot(market, symbol string) (dialectSymbol string) {
//	switch market {
//	case BybitSpot:
//		tail := GetSpotTail(market)
//		if len(symbol) > len(tail) && symbol[len(symbol)-len(tail):] == tail {
//			return symbol[0:len(symbol)-len(tail)] + `USDT`
//		}
//	}
//	return ``
//}
//
//// GetStandardPerp from dialect perp symbol
//func GetStandardPerp(market, symbol string) (standardSymbol string) {
//	symbol = strings.ToLower(symbol)
//	switch market {
//	case Bitmex:
//		return strings.Replace(symbol, `xbt`, `btc`, -1) + `_p`
//	case BybitPerp:
//		if len(symbol) > 4 && strings.EqualFold(symbol[len(symbol)-4:], `usdt`) {
//			return strings.ToUpper(symbol[0:len(symbol)-4] + GetPerpTail(market))
//		}
//	case BybitSpot:
//		if len(symbol) > 4 && strings.EqualFold(symbol[len(symbol)-4:], `usdt`) {
//			return strings.ToUpper(symbol[0:len(symbol)-4] + GetSpotTail(market))
//		}
//	}
//	return standardSymbol
//}
//
//// GetStandardSpot from dialect spot symbol
//func GetStandardSpot(market, symbol string) (standardSymbol string) {
//	symbol = strings.ToLower(symbol)
//	switch market {
//	case BybitSpot:
//		if len(symbol) > 4 && strings.EqualFold(symbol[len(symbol)-4:], `usdt`) {
//			return strings.ToUpper(symbol[0:len(symbol)-4] + GetSpotTail(market))
//		}
//	}
//	return standardSymbol
//}
//
//func GetSpotTail(market string) string {
//	switch market {
//	case Huobi:
//		return `usdt`
//	case Ftx:
//		return `/USD`
//	case OKEX, Kucoin, BybitSpot:
//		return `-USDT`
//	case Binance:
//		return `USDT`
//	case Gate:
//		return `_USDT`
//	}
//	return ``
//}
//
//func GetPerpTail(market string) string {
//	switch market {
//	case Huobi:
//		return `-usdt`
//	case OKEX:
//		return `-USDT-SWAP`
//	case Binance, Kucoin, Ftx, BybitPerp:
//		return `-PERP`
//	case Gate:
//		return `_PERP`
//	}
//	return ``
//}

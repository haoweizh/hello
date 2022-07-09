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
const BinanceSpot = "binancespot"
const BinancePerp = "binanceperp"
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
const FunctionDynamicTurtle = `dynamic_turtle`
const FunctionGrid = `grid`
const FunctionCarry = `carry`
const FunctionCross = `cross`
const MarketTypePerp = `perp`
const MarketTypeSpot = `spot`
const FunctionDCarry = `dcarry`
const FunctionComplement = `comp`
const FunctionCrossOpen = `open`
const FunctionCrossClose = `close`
const FunctionHang = `hang`
const PostOnly = `ParticipateDoNotInitiate`

//const SubscribeDeal = `subscribeDeal`
//const FunctionHang = `hang`
//const FunctionPostonlyHandler = `postonly`
//const OKEXBTCContractFaceValue = 100.0
//const OKEXOtherContractFaceValue = 10.0

var AppDB *gorm.DB
var AppConfig *Config
var AppMarkets = &Markets{}
var ChannelMaintaining sync.Map // market - bool
var DialectTail = map[string]map[string]string{
	MarketTypeSpot: {Gate: `_USDT`, Ftx: `/USD`, OKEX: `-USDT`, BybitSpot: `USDT`, BinanceSpot: `USDT`},
	MarketTypePerp: {Gate: `_USDT`, Ftx: `-PERP`, OKEX: `-USDT-SWAP`, BybitPerp: `USDT`, BinancePerp: `USDT`, Mexc: `_USDT`}}
var UniStandardTail = map[string]string{MarketTypeSpot: `_USDT`, MarketTypePerp: `_PERP`}

func GetFromStandard(market, standardSymbol string) (success bool, marketType, coinValue, dialectSymbol string) {
	for mType, tail := range UniStandardTail {
		if util.EndWith(standardSymbol, tail) {
			coin := standardSymbol[0 : len(standardSymbol)-len(tail)]
			return true, mType, coin, coin + DialectTail[mType][market]
		}
	}
	util.Notice(`fail to parse standard symbol %s`, standardSymbol)
	return false, ``, ``, ``
}

// GetCoinFromDialect 注意如果交易所不同市场的symbol有相同的tail，此时marketType有可能匹配错误，慎用
func GetCoinFromDialect(market, dialectSymbol string) (success bool, marketType, coin string) {
	for mType, tails := range DialectTail {
		if tails[market] == `` {
			continue
		}
		if util.EndWith(dialectSymbol, tails[market]) {
			return true, mType, dialectSymbol[0 : len(dialectSymbol)-len(tails[market])]
		}
	}
	return false, ``, ``
}

var orderStatusMap = map[string]map[string]string{ // market - market status - united status
	BinancePerp: {
		"NEW":              CarryStatusWorking,
		"PARTIALLY_FILLED": CarryStatusWorking,
		"PENDING_CANCEL":   CarryStatusWorking,
		"FILLED":           CarryStatusSuccess,
		"CANCELED":         CarryStatusFail,
		"REJECTED":         CarryStatusFail,
		"EXPIRED":          CarryStatusFail,
	},
	BinanceSpot: {
		"NEW":              CarryStatusWorking,
		"PARTIALLY_FILLED": CarryStatusWorking,
		"PENDING_CANCEL":   CarryStatusWorking,
		"FILLED":           CarryStatusSuccess,
		"CANCELED":         CarryStatusFail,
		"REJECTED":         CarryStatusFail,
		"EXPIRED":          CarryStatusFail,
	},
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

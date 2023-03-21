package model

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/configor"
	"gorm.io/gorm"
	"hello/util"
	"strings"
	"sync"
	"time"
)

type PostOrder func(order *Order, setting *Setting) // 处理下单后的函数
var HandlerMap = make(map[string]CarryHandler)
var infoLock sync.Mutex
var CarryInfo sync.Map                                            // userKey - function - msg
var monitorInfo = make(map[string]map[string]map[string][]string) // userKey - table - item - value array
var AppMetric = &MetricManager{}
var IgnoreFunctions = map[string]bool{FunctionDynamicTurtle: true, FunctionTurtleNormal: true, FunctionDynamicCombine: true}

const BitgetSpot = `bitgetspot`
const BitgetPerp = `bitgetperp`
const Kucoin = `kucoin`
const KucoinSpot = `kucoinspot`
const KucoinPerp = `kucoinperp`
const HuobiSpot = `huobispot`
const HuobiPerp = `huobiperp`
const Gate = `gate`
const Mexc = `mexc`
const DFuture = `dfuture`
const BybitSpot = `bybitspot`
const BybitPerp = `bybitperp`
const GXZQ = `GXZQ`
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
const FunctionSimulation = `simulation`
const FunctionTurtle = `turtle`
const FunctionTurtleAdjust = `turtle_adjust`
const FunctionDynamicTurtle = `dynamic_turtle`
const FunctionDynamicCombine = `dynamic_combine`
const FunctionCombineTurtle = `combine_turtle`
const FunctionTurtleNormal = `turtle_normal`
const FunctionGrid = `grid`
const FunctionCross = `cross`
const MarketTypePerp = `perp`
const MarketTypeSpot = `spot`
const MarketTypeFuture = `future`
const FunctionDCarry = `dcarry`
const FunctionComplement = `comp`
const Open = `open`
const Close = `close`
const FunctionHang = `hang`
const PostOnly = `ParticipateDoNotInitiate`

var AppDB *gorm.DB
var AppRedis *redis.Client
var AppConfig *Config
var AppMarkets = &Markets{}
var ChannelMaintaining sync.Map // market - bool
var DialectTail = map[string]map[string]string{
	MarketTypeSpot:   {Gate: `_USDT`, Ftx: `/USD`, OKEX: `-USDT`, BybitSpot: `USDT`, BinanceSpot: `USDT`, KucoinSpot: `-USDT`, BitgetSpot: `USDT_SPBL`},
	MarketTypePerp:   {Gate: `_USDT`, Ftx: `-PERP`, OKEX: `-USDT-SWAP`, BybitPerp: `USDT`, BinancePerp: `USDT`, Mexc: `_USDT`, KucoinPerp: `USDTM`, BitgetPerp: `USDT_UMCBL`},
	MarketTypeFuture: {GXZQ: ``},
}
var UniStandardTail = map[string]string{MarketTypeSpot: `_USDT`, MarketTypePerp: `_PERP`, MarketTypeFuture: `_FUTURE`}

func GetFromStandard(market, standardSymbol string) (success bool, marketType, coinValue, dialectSymbol string) {
	if len(strings.Trim(standardSymbol, ` `)) == 0 {
		return
	}
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

func GetMarketNow(market string) time.Time {
	switch market {
	case OKEX:
		location, err := time.LoadLocation("Asia/Shanghai")
		if err == nil {
			return time.Now().In(location)
		}
		return time.Now()
	}
	return time.Now().In(time.UTC)
}

func (config *Config) ToString() string {
	str := "markets-carry cost:\n"
	str += fmt.Sprintf("delay: %f\n", config.Delay)
	str += fmt.Sprintf("handle: %s\n", config.Handle)
	return str
}

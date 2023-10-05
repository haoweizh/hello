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

type PostOrder func(order *Order) // 处理下单后的函数
var HandlerMap = make(map[string]CarryHandler)
var AccountHandlerMap = make(map[string]WsOrderHandler)

var CarryInfo sync.Map        // userKey - function - msg
var monitorInfo = &sync.Map{} // userIndex - table - syncMap[string -array[]string]
var AppMetric = &MetricManager{}
var IgnoreFunctions = map[string]bool{FunctionDynamicTurtle: true, FunctionTurtleNormal: true,
	FunctionDynamicCombine: true, FunctionDynamicBoost: true}

const DefaultLeverage = 5
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
const Bybit = `bybit`
const GXZQ = `GXZQ`
const OKEX = "okex"
const BinanceSpot = "binancespot"
const BinanceMargin = `binancemargin`
const BinancePerp = "binanceperp"
const Ftx = `ftx`
const Bitmex = `bitmex`
const SubscribeDepth = `SubscribeDepth`
const SubscribeTicker = `ticker`
const SubscribeMarkPrice = `markPrice`
const CarryStatusSuccess = "success"
const CarryStatusFail = "fail"
const CarryStatusWorking = "working"
const OrderTypeLimit = `limit`
const OrderTypeMarket = `market`
const OrderTypeStop = `stop`
const OrderTypeTrailStop = `trail_stop`
const OrderSideBuy = `buy`
const OrderSideSell = `sell`
const OrderSideLiquidateLong = `liquidateLong`
const OrderSideLiquidateShort = `liquidateShort`
const FunctionSimulation = `simulation`
const FunctionTurtle = `turtle`
const FunctionGrid = `grid`
const FunctionBoost = `boost`
const FunctionTurtleAdjust = `turtle_adjust`
const FunctionDynamicTurtle = `dynamic_turtle`
const FunctionDynamicCombine = `dynamic_combine`
const FunctionDynamicBoost = `dynamic_boost`
const FunctionCombineTurtle = `combine_turtle`
const FunctionTurtleNormal = `turtle_normal`
const FunctionCross = `cross`
const FunctionQueue = `queue`
const MarketTypePerp = `perp`
const MarketTypeSpot = `spot`
const MarketTypeMargin = `margin`
const MarketTypeFuture = `future`
const FunctionDCarry = `dcarry`
const FunctionComplement = `comp`
const FunctionBitgetLiq = `liquidate`
const Open = `open`
const Close = `close`
const FunctionHang = `hang`
const PostOnly = `ParticipateDoNotInitiate`
const ReduceOnly = `reduceOnly`
const TopCross = `top_cross`

var AppDB *gorm.DB
var AppRedis *redis.Client
var AppConfig *Config
var AppMarkets = &Markets{}
var ChannelMaintaining sync.Map // market - bool
var DialectTail = map[string]map[string]string{
	MarketTypeSpot:   {Gate: `_USDT`, Ftx: `/USD`, OKEX: `-USDT`, Bybit: `USDT`, BinanceSpot: `USDT`, BinanceMargin: `USDT`, KucoinSpot: `-USDT`, BitgetSpot: `USDT_SPBL`},
	MarketTypePerp:   {Gate: `_USDT`, Ftx: `-PERP`, OKEX: `-USDT-SWAP`, Bybit: `USDT`, BinancePerp: `USDT`, Mexc: `_USDT`, KucoinPerp: `USDTM`, BitgetPerp: `USDT_UMCBL`},
	MarketTypeFuture: {GXZQ: ``},
	MarketTypeMargin: {BinanceMargin: `USDT`, BinanceSpot: `USDT`},
}
var UniStandardTail = map[string]string{MarketTypeSpot: `_USDT`, MarketTypeMargin: `_USDT`, MarketTypePerp: `_PERP`,
	MarketTypeFuture: `_FUTURE`}

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
	util.Notice(`fail to parse standard symbol %s %s`, market, standardSymbol)
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
	BinanceMargin: {
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
	Ftx: {
		`new`:       CarryStatusWorking,
		`open`:      CarryStatusWorking,
		`closed`:    CarryStatusSuccess,
		`cancelled`: CarryStatusFail,
		`triggered`: CarryStatusSuccess},
}

func GetMonitorInfo(index, table string) (valueArray [][]string) {
	v, ok := util.LoadSyncMap(monitorInfo, index, table)
	if !ok || v == nil {
		return
	}
	valueArray = make([][]string, 0)
	v.(*sync.Map).Range(func(key, value any) bool {
		if value == nil {
			return true
		}
		valueArray = append(valueArray, value.([]string))
		return true
	})
	for i := len(valueArray) - 1; i > 0; i-- {
		for j := 0; j < i; j++ {
			if valueArray[j][0] > valueArray[i][0] {
				valueArray[j], valueArray[i] = valueArray[i], valueArray[j]
			}
		}
	}
	return valueArray
}

func SetMonitorInfo(index, table, item string, value []string) {
	var infoMap *sync.Map
	v, ok := util.LoadSyncMap(monitorInfo, index, table)
	if ok && v != nil {
		infoMap = v.(*sync.Map)
	} else {
		infoMap = &sync.Map{}
	}
	infoMap.Store(item, value)
	util.StoreSyncMap(monitorInfo, infoMap, index, table)
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
	AppConfig = &Config{GateSpot: false, KucoinSpot: true}
	err := configor.Load(AppConfig, "./config.yml")
	if err != nil {
		util.Notice(err.Error())
		return
	}
}

func GetNowPeriod(market string, periodInSecond int64, periodTime time.Time) (nowPeriod time.Time, nowStr string) {
	remainder := periodTime.Unix() % periodInSecond
	if market == OKEX {
		remainder = (periodTime.Unix() + 28800) % periodInSecond
	}
	seconds := periodTime.Unix() - remainder
	nowPeriod = time.Unix(seconds, 0)
	return nowPeriod, fmt.Sprintf(`%d`, nowPeriod.Unix())
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
	str += fmt.Sprintf("handle: %s\n", config.Handle)
	return str
}

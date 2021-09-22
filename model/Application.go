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
var Currencies = []string{`btc`, `eth`, `usdt`, `ft`, `ft1808`, `pax`, `usdc`, `tusd`}
var CarryInfo = make(map[string]string)                             // function - msg
var carryInfos = make(map[string]map[string]map[string]interface{}) // table - line name - key - value
var AppMetric = &MetricManager{}

const KeyDefault = ``
const SecretDefault = ``

const OKEXBTCContractFaceValue = 100.0
const OKEXOtherContractFaceValue = 10.0
const Kucoin = `kucoin`
const Gate = `gate`
const DFuture = `dfuture`
const Bybit = `bybit`
const OKEX = "okex"
const Huobi = "huobi"
const HuobiDM = `huobiDM`
const HuobiFuture = `HuobiFuture`
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
const FunctionTurtle = `turtle`
const FunctionGrid = `grid`
const FunctionCarry = `carry`
const FunctionCross = `cross`
const FunctionDCarry = `dcarry`
const FunctionComplement = `comp`
const FunctionPostonlyHandler = `postonly`
const PostOnly = `ParticipateDoNotInitiate`
const SymbolTypeSpot = `spot`
const SymbolTypePerp = `perp`

var AppDB *gorm.DB
var AppSettings []Setting
var AppConfig *Config
var AppMarkets = NewMarkets()

var AppPause = false

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

func GetCarryInfos(excludeTable string) (info [][]map[string]interface{}) {
	infoLock.Lock()
	defer infoLock.Unlock()
	info = make([][]map[string]interface{}, 0)
	for table, carryInfo := range carryInfos {
		if table == excludeTable {
			continue
		}
		copyTable := make([]map[string]interface{}, 0)
		for _, item := range carryInfo {
			copyItem := make(map[string]interface{})
			for keyItem, value := range item {
				copyItem[keyItem] = value
			}
			copyTable = append(copyTable, copyItem)
		}
		info = append(info, copyTable)
	}
	return
}

func RemoveCarryInfos(table, key string) {
	infoLock.Lock()
	defer infoLock.Unlock()
	carryInfo := carryInfos[table]
	if carryInfo == nil || len(carryInfo) == 0 {
		return
	}
	delete(carryInfo, key)
}

func SetCarryInfos(table, key string, item map[string]interface{}) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if carryInfos[table] == nil {
		carryInfos[table] = make(map[string]map[string]interface{})
	}
	carryInfos[table][key] = item
}

func GetCarryInfo(mark, except string) (info string) {
	infoLock.Lock()
	defer infoLock.Unlock()
	keys := make([]string, 0)
	for key := range CarryInfo {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if (mark == `` || strings.Contains(key, mark)) && (except == `` || !strings.Contains(key, except)) {
			info += fmt.Sprintf("%s\n", CarryInfo[key])
		}
	}
	return
}

func SetCarryInfo(key, value string) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if CarryInfo == nil {
		CarryInfo = make(map[string]string)
	}
	CarryInfo[key] = value
}

func RemoveCarryInfo(key string) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if CarryInfo != nil {
		delete(CarryInfo, key)
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

func getSymbolWithSplit(original, split string) (symbol string) {
	original = strings.ToLower(original)
	for _, currency := range Currencies {
		if strings.Contains(original, currency) && strings.LastIndex(original, currency)+len(currency) == len(original) {
			return original[0:strings.LastIndex(original, currency)] + split + currency
		}
	}
	util.Notice(`can not parse symbol for currency absent ` + original)
	return ``
}

func GetSymbol(market, subscribe string) (symbol string) {
	switch market {
	case Huobi: //market.xrpbtc.depth.step0: xrp_btc
		if strings.Contains(subscribe, "bbo") {
			return strings.Split(subscribe, ".")[1]
		}
		subscribe = strings.Replace(subscribe, "market.", "", 1)
		subscribe = strings.Replace(subscribe, ".depth.step0", "", 1)
		return getSymbolWithSplit(subscribe, "_")
	case OKEX: //ok_sub_spot_xrp_btc_depth_5: xrp_btc
		subscribe = strings.Replace(subscribe, "ok_sub_spot_", "", 1)
		subscribe = strings.Replace(subscribe, "_depth_5", "", 1)
		return subscribe
	case Binance: // eosusdt@depth5: xrpbtc
		if strings.Index(subscribe, `@`) == -1 {
			return ``
		}
		subscribe = subscribe[0:strings.Index(subscribe, `@`)]
		return strings.ToUpper(subscribe) //返回格式 XRPUSDT
		//return getSymbolWithSplit(subscribe, `_`)
	case Coinpark: //BTC_USDT bibox_sub_spot_BTC_USDT_ticker
		subscribe = strings.Replace(subscribe, `bibox_sub_spot_`, ``, 1)
		subscribe = strings.Replace(subscribe, `_ticker`, ``, 1)
		return subscribe
	case Bitmex:
		return subscribe
	}
	return ""
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

package model

import (
	"fmt"
	"github.com/jinzhu/configor"
	"github.com/jinzhu/gorm"
	"hello/util"
	"strings"
	"sync"
	"time"
)

var HandlerMap = make(map[string]CarryHandler)
var infoLock sync.Mutex
var Currencies = []string{`btc`, `eth`, `usdt`, `ft`, `ft1808`, `pax`, `usdc`, `tusd`}

//var btcBalance = make(map[string]float64) // market+rfc3339, btc balance
//var usdBalance = make(map[string]float64) // market_rfc3339, usd balance
var candles = make(map[string]*Candle)                              // market+symbolInstrument+period+rfc3339, candle
var CarryInfo = make(map[string]string)                             // function - msg
var carryInfos = make(map[string]map[string]map[string]interface{}) // table - line name - key - value
var AppMetric = &MetricManager{}

const KeyDefault = ``
const SecretDefault = ``

//const ArbitraryCarryUSDT = 100.0
const OKEXBTCContractFaceValue = 100.0
const OKEXOtherContractFaceValue = 10.0
const DFuture = `dfuture`
const Bybit = `bybit`
const OKEX = "okex"
const OKFUTURE = `okfuture`
const OKSwap = `okswap`
const Huobi = "huobi"
const HuobiDM = `huobiDM`
const Binance = "binance"
const Ftx = `ftx`
const Coinpark = "coinpark"
const Bitmex = `bitmex`
const AccountTypeReduce = `reduceOnly`
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

//const CarryTypeFuture = `future`
//const CarryTypeArbitrarySell = `arbitrarysell`
//const CarryTypeArbitraryBuy = `arbitrarybuy`
const AmountTypeNew = `new` // 用于okswap 开仓，而不平仓

const FunctionTurtle = `turtle`
const FunctionGrid = `grid`
const FunctionCarry = `carry`
const FunctionComplement = `complement`
const FunctionHangFar = `hang_far`
const FunctionHangRevert = `hang_revert`
const FunctionPostonlyHandler = `postonly`

//const FunctionRefresh = `refresh`
//const FunctionRank = `rank`
//const FunctionHang = `hang`

const PostOnly = `ParticipateDoNotInitiate`

var AppDB *gorm.DB
var AppSettings []Setting
var AppConfig *Config
var AppMarkets = NewMarkets()
var AppAccounts = NewAccounts()
var HuobiAccountIds = make(map[string]string)
var AppPause = false

func GetDialectSymbol(market, symbol string) (dialectSymbol string) {
	switch market {
	case Bitmex:
		symbol = strings.Replace(symbol, `btc`, `xbt`, -1)
		return strings.ToUpper(strings.Split(symbol, `_`)[0])
	case Bybit:
		return strings.ToUpper(strings.Split(symbol, `_`)[0])
	case OKSwap:
		if strings.Contains(symbol, `usd_p`) {
			return strings.ToUpper(strings.Split(symbol, `usd`)[0]) + `-USD-SWAP`
		} else if strings.Contains(symbol, `usdt_p`) {
			return strings.ToUpper(strings.Split(symbol, `usdt`)[0]) + `-USDT-SWAP`
		}
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
	case OKSwap:
		if strings.Contains(symbol, `-usd-swap`) {
			return strings.Split(symbol, `-usd`)[0] + `usd_p`
		} else if strings.Contains(symbol, `-usdt-swap`) {
			return strings.Split(symbol, `-usd`)[0] + `usdt_p`
		}
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
	OKFUTURE: {
		`0`:  CarryStatusWorking, //等待成交
		`1`:  CarryStatusWorking, //部分成交
		`2`:  CarryStatusSuccess, //完全成交
		`3`:  CarryStatusWorking, //下单中
		`4`:  CarryStatusWorking, //撤单中
		`-1`: CarryStatusFail,    //撤单成功
		`-2`: CarryStatusFail,    //失败
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
	OKSwap: {
		`-2`: CarryStatusFail,    // 失败
		`-1`: CarryStatusFail,    // 撤单成功
		`0`:  CarryStatusWorking, // 等待成交
		`1`:  CarryStatusWorking, // 部分成交
		`2`:  CarryStatusSuccess, // 完全成交
		`3`:  CarryStatusWorking, // 下单中
		`4`:  CarryStatusWorking, // 撤单中
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
	for key, value := range CarryInfo {
		if (mark == `` || strings.Contains(key, mark)) && (except == `` || !strings.Contains(value, except)) {
			info += fmt.Sprintf("%s: %s\n", key, value)
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

func GetCandle(market, symbolInstrument, period, utcDate string) (candle *Candle) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if candles == nil {
		candles = make(map[string]*Candle)
	}
	key := market + symbolInstrument + period + utcDate
	return candles[key]
}

func SetCandle(market, symbol, period, utcDate string, candle *Candle) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if candles == nil {
		candles = make(map[string]*Candle)
	}
	key := market + symbol + period + utcDate
	candles[key] = candle
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

func GetWSSubscribePos(market, symbol string) (subscribe interface{}) {
	switch market {
	case OKSwap:
		return `swap/position:` + GetDialectSymbol(market, symbol)
	}
	return ``
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
		subscribe = strings.Replace(subscribe, "market.", "", 1)
		subscribe = strings.Replace(subscribe, ".depth.step0", "", 1)
		return getSymbolWithSplit(subscribe, "_")
	case OKEX: //ok_sub_spot_xrp_btc_depth_5: xrp_btc
		subscribe = strings.Replace(subscribe, "ok_sub_spot_", "", 1)
		subscribe = strings.Replace(subscribe, "_depth_5", "", 1)
		return subscribe
	case OKFUTURE: // ok_sub_futureusd_btc_depth_this_week_5: btc_this_week
		subscribe = strings.Replace(subscribe, `ok_sub_futureusd_`, ``, 1)
		subscribe = strings.Replace(subscribe, `_depth`, ``, 1)
		subscribe = strings.Replace(subscribe, `_5`, ``, 1)
		return subscribe
	case Binance: // eosusdt@depth5: xrp_btc
		if strings.Index(subscribe, `@`) == -1 {
			return ``
		}
		subscribe = subscribe[0:strings.Index(subscribe, `@`)]
		return getSymbolWithSplit(subscribe, `_`)
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
	AppConfig = &Config{}
	err := configor.Load(AppConfig, "./config.yml")
	if err != nil {
		util.Notice(err.Error())
		return
	}
	AppConfig.WSUrls = make(map[string]string)
	AppConfig.RestUrls = make(map[string]string)
	AppConfig.WSUrls[Huobi] = `wss://api-aws.huobi.pro/feed`
	AppConfig.WSUrls[HuobiDM] = `wss://api.hbdm.com/`
	AppConfig.WSUrls[Binance] = "wss://stream.binance.com:9443/stream?streams="
	AppConfig.WSUrls[Ftx] = `wss://ftx.com/ws`
	AppConfig.WSUrls[OKEX] = `wss://ws.okex.com:8443/ws/v5/public?brokerId=9999`
	if AppConfig.Env == `test` {
		AppConfig.WSUrls[Bybit] = `wss://stream.bybit.com/realtime`
		AppConfig.RestUrls[Bybit] = `https://api.bybit.com`
	} else {
		AppConfig.WSUrls[Bybit] = `wss://stream.bybit.com/realtime`
		AppConfig.RestUrls[Bybit] = `https://api.bybit.com`
	}
	AppConfig.WSUrls[Coinpark] = "wss://push.coinpark.cc/"
	AppConfig.WSUrls[Bitmex] = `wss://www.bitmex.com/realtime/`
	AppConfig.WSUrls[OKFUTURE] = `wss://real.okex.com:8443/ws/v3`
	AppConfig.WSUrls[OKSwap] = `wss://real.okex.com:8443/ws/v3`
	AppConfig.WSUrls[DFuture] = `wss://heco_prod_kline_wss.dfuture.com/ws`
	//AppConfig.WSUrls[Bitmex] = `wss://testnet.bitmex.com/realtime`
	// HUOBI用于交易的API，可能不适用于行情
	//config.RestUrls[Huobi] = "https://api.huobipro.com/v1"
	//AppConfig.RestUrls[Huobi] = "https://api.huobi.pro"
	AppConfig.RestUrls[OKEX] = `https://www.okex.com`
	AppConfig.RestUrls[Huobi] = `api-aws.huobi.pro`
	AppConfig.RestUrls[HuobiDM] = `api.hbdm.com`
	AppConfig.RestUrls[OKSwap] = `https://www.okex.com`
	AppConfig.RestUrls[OKFUTURE] = `https://www.okex.com`
	AppConfig.RestUrls[Binance] = "https://api.binance.com"
	AppConfig.RestUrls[Coinpark] = "https://api.coinpark.cc/v1"
	//AppConfig.RestUrls[Bitmex] = `https://testnet.bitmex.com`
	AppConfig.RestUrls[Bitmex] = `https://www.bitmex.com/api/v1`
	AppConfig.RestUrls[Ftx] = `https://ftx.com/api`
	AppConfig.SymbolPrice = make(map[string]float64)
	AppConfig.UpdatePriceTime = make(map[string]int64)
}

func GetMarketYesterday(market string) (yesterday time.Time, strYesterday string) {
	yesterday = time.Now().In(time.UTC)
	if market == OKFUTURE || market == HuobiDM {
		yesterday = util.GetNow()
	}
	duration, _ := time.ParseDuration(`-24h`)
	yesterday = yesterday.Add(duration)
	yesterday = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	return yesterday, yesterday.String()[0:10]
}

func GetMarketToday(market string) (today time.Time, strToday string) {
	today = time.Now().In(time.UTC)
	if market == OKFUTURE || market == HuobiDM {
		today = util.GetNow()
	}
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	return today, today.String()[0:10]
}

func (config *Config) ToString() string {
	str := "markets-carry cost:\n"
	str += fmt.Sprintf("delay: %f\n", config.Delay)
	str += fmt.Sprintf("channelslot: %f\n", config.ChannelSlot)
	str += fmt.Sprintf("PreDealDis: %f Binance order dis: %f\n", config.PreDealDis, config.BinanceOrderDis)
	str += fmt.Sprintf("channels: %d \n", config.Channels)
	str += fmt.Sprintf("handle: %s\n", config.Handle)
	str += fmt.Sprintf("amountrate: %f\n", config.AmountRate)
	str += fmt.Sprintf("amount: %f\n", config.Amount)
	return str
}

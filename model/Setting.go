package model

import (
	"fmt"
	"hello/util"
	"strings"
	"time"
)

type CarryHandler func(setting *Setting, bidAsk *BidAsk)

type Setting struct {
	Valid            bool
	Function         string `gorm:"index:function_market_symbol,unique"`
	Market           string `gorm:"index:function_market_symbol,unique"`
	MarketRelated    string
	Symbol           string `gorm:"index:function_market_symbol,unique"`
	Coin             string
	SymbolRelated    string
	PriceX           float64
	OpenShortMargin  float64 // arbitrary future use
	CloseShortMargin float64 // arbitrary future use
	Chance           int64   // arbitrary future use
	GridAmount       float64
	AmountLimit      float64
	ID               uint `gorm:"primary_key"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

var marketSymbolSetting map[string]map[string]map[string]*Setting // function - marketName - symbol - setting
var handlers map[string]map[string]map[string]CarryHandler        // market - symbol - function- carryHandler
var coinSettings map[string]map[string][]*Setting                 // function - coin - setting

func GetSettingCoins(function, market string) (coins map[string]bool) {
	if handlers == nil {
		LoadSettings()
	}
	if marketSymbolSetting[function] == nil {
		return nil
	}
	if marketSymbolSetting[function][market] == nil {
		return nil
	}
	coins = make(map[string]bool)
	for _, setting := range marketSymbolSetting[function][market] {
		if setting == nil {
			continue
		}
		coins[GetCoin(setting.Market, setting.Symbol)] = true
	}
	return
}

func GetSettings(function, market string) map[string]*Setting {
	if handlers == nil {
		LoadSettings()
	}
	if marketSymbolSetting[function] == nil {
		return nil
	}
	return marketSymbolSetting[function][market]
}

func GetSetting(function, market, symbol string) *Setting {
	if handlers == nil {
		LoadSettings()
	}
	if marketSymbolSetting[function] == nil || marketSymbolSetting[function][market] == nil {
		return nil
	}
	return marketSymbolSetting[function][market][symbol]
}

func GetCurrentN(setting *Setting) (currentN int64) {
	if setting == nil || marketSymbolSetting[setting.Function] == nil ||
		marketSymbolSetting[setting.Function][setting.Market] == nil {
		return 0
	}
	for _, value := range marketSymbolSetting[setting.Function][setting.Market] {
		if value != nil && value.Market == setting.Market && value.Function == setting.Function {
			currentN += value.Chance
		}
	}
	return currentN
}

func GetFunctions(market, symbol string) map[string]CarryHandler {
	infoLock.Lock()
	defer infoLock.Unlock()
	if handlers == nil {
		LoadSettings()
	}
	if handlers[market] == nil {
		return nil
	}
	return handlers[market][symbol]
}

func GetCoinSetting(function, coin string) []*Setting {
	infoLock.Lock()
	defer infoLock.Unlock()
	if coinSettings == nil || coinSettings[function] == nil {
		return nil
	}
	return coinSettings[function][coin]
}

func LoadSettings() {
	infoLock.Lock()
	defer infoLock.Unlock()
	AppSettings = []Setting{}
	AppDB.Where(`valid = ?`, true).Find(&AppSettings)
	marketSymbolSetting = make(map[string]map[string]map[string]*Setting)
	handlers = make(map[string]map[string]map[string]CarryHandler)
	coinSettings = make(map[string]map[string][]*Setting)
	for i := range AppSettings {
		market := AppSettings[i].Market
		function := AppSettings[i].Function
		if coinSettings[function] == nil {
			coinSettings[function] = make(map[string][]*Setting)
		}
		if coinSettings[function][AppSettings[i].Coin] == nil {
			coinSettings[function][AppSettings[i].Coin] = make([]*Setting, 0)
		}
		coinSettings[function][AppSettings[i].Coin] =
			append(coinSettings[function][AppSettings[i].Coin], &AppSettings[i])
		symbols := []string{AppSettings[i].Symbol}
		if function == FunctionCarry {
			symbols = append(symbols, AppSettings[i].SymbolRelated)
		}
		for _, symbol := range symbols {
			if marketSymbolSetting[function] == nil {
				marketSymbolSetting[function] = make(map[string]map[string]*Setting)
			}
			if marketSymbolSetting[function][market] == nil {
				marketSymbolSetting[function][market] = make(map[string]*Setting)
			}
			marketSymbolSetting[function][market][symbol] = &AppSettings[i]
			if handlers[market] == nil {
				handlers[market] = make(map[string]map[string]CarryHandler)
			}
			if handlers[market][symbol] == nil {
				handlers[market][symbol] = make(map[string]CarryHandler)
			}
			if handlers[market][symbol][function] == nil {
				handlers[market][symbol][function] = HandlerMap[function]
			} else {
				handlers[market][symbol][fmt.Sprintf(`%s_%d`, function, util.GetNow().UnixNano())] =
					HandlerMap[function]
			}
		}
	}
	for _, setting := range AppSettings {
		util.Notice(fmt.Sprintf(`load setting %s %s %s %v`,
			setting.Market, setting.Symbol, setting.Function, setting.Valid))
	}
}

func GetMarketSymbols(market string) map[string]bool {
	if AppSettings == nil {
		LoadSettings()
	}
	symbols := make(map[string]bool)
	for _, value := range AppSettings {
		if value.Market == market {
			related := value.SymbolRelated
			symbols[value.Symbol] = true
			if related != `` {
				symbols[related] = true
			}
		}
	}
	return symbols
}

func GetCoinSettings(function string) map[string][]*Setting {
	if AppSettings == nil {
		LoadSettings()
	}
	if coinSettings == nil {
		return nil
	}
	return coinSettings[function]
}

func GetMarkets() []string {
	if AppSettings == nil {
		LoadSettings()
	}
	marketMap := make(map[string]bool)
	for _, value := range AppSettings {
		marketMap[value.Market] = true
		if value.MarketRelated != "" {
			marketMap[value.MarketRelated] = true
		}
	}
	markets := make([]string, len(marketMap))
	i := 0
	for key := range marketMap {
		markets[i] = key
		i++
	}
	return markets
}

func GetCoin(market, symbol string) (coin string) {
	switch market {
	case Binance:
		tails := []string{`-PERP`, `USDT`}
		for _, tail := range tails {
			if symbol[len(symbol)-len(tail):] == tail {
				return symbol[0 : len(symbol)-len(tail)]
			}
		}
	case Ftx:
		tails := []string{`-PERP`, `/USD`}
		for _, tail := range tails {
			if strings.Contains(symbol, tail) {
				coin = symbol[0:strings.Index(symbol, tail)]
			}
		}
	case HuobiDM: // btc_cq
		parts := strings.Split(symbol, `_`)
		if len(parts) == 2 {
			coin = parts[0]
		}
	case Huobi: // btc-usdt
		parts := strings.Split(symbol, `-`)
		if len(parts) == 2 {
			coin = parts[0]
		}
	case OKEX:
		index := strings.Index(symbol, `-`)
		if index > 0 {
			coin = symbol[0:index]
		}
	case Gate:
		parts := strings.Split(symbol, `_`)
		if len(parts) == 2 {
			coin = parts[0]
		}
	case Kucoin:
		parts := strings.Split(symbol, `-`)
		if len(parts) == 2 {
			coin = parts[0]
		}
	}
	return coin
}

func IsSpot(market, symbol string) (coin string, result bool) {
	tail := GetSpotTail(market)
	lastIndex := strings.LastIndex(symbol, tail)
	if lastIndex > 0 && symbol[lastIndex:] == tail {
		result = true
		coin = symbol[0:lastIndex]
	}
	return
}

func IsPerp(market, symbol string) (coin string, result bool) {
	tail := GetPerpTail(market)
	lastIndex := strings.LastIndex(symbol, tail)
	if lastIndex > 0 && symbol[lastIndex:] == tail {
		result = true
		coin = symbol[0:lastIndex]
	}
	return
}

func GetSpotTail(market string) string {
	switch market {
	case Huobi:
		return "usdt"
	case Ftx:
		return `/USD`
	case OKEX, Kucoin, BybitSpot:
		return `-USDT`
	case Binance:
		return `USDT`
	case Gate:
		return `_USDT`
	}
	return ``
}

func GetPerpTail(market string) string {
	switch market {
	case Huobi:
		return `-usdt`
	case OKEX:
		return `-USDT-SWAP`
	case Binance, Kucoin, Ftx, BybitPerp:
		return `-PERP`
	case Gate:
		return `_PERP`
	}
	return ``
}

func GetCrossCoin(market, symbol string) (coin string) {
	var tails []string
	switch market {
	case Ftx:
		tails = []string{`/USD`, `-PERP`}
	case OKEX:
		tails = []string{`-USDT`, `-USDT-SWAP`}
	case Gate:
		tails = []string{`_USDT`, `_PERP`}
	case Binance:
		tails = []string{`-PERP`}
	}
	for _, tail := range tails {
		coinLen := len(symbol) - len(tail)
		if strings.LastIndex(symbol, tail) == coinLen && coinLen > 0 {
			coin = symbol[:len(symbol)-len(tail)]
			//if market == Gate && coin == `MBABYDOGE` {
			//	coin = `BABYDOGE`
			//}
			return coin
		}
	}
	return
}

func GetSpotCoins(market, symbol string) []string {
	switch market {
	case Huobi:
		if strings.LastIndex(symbol, `usdt`) == len(symbol)-4 {
			return []string{symbol[0 : len(symbol)-4], `usdt`}
		}
		return nil
	case Ftx:
		return strings.Split(symbol, `/`)
	case OKEX, Kucoin:
		return strings.Split(symbol, `-`)
	case Binance:
		if strings.LastIndex(symbol, `USDT`) == len(symbol)-4 {
			return []string{symbol[0 : len(symbol)-4], `USDT`}
		}
		return nil
	case Gate:
		return strings.Split(symbol, `_`)
	}
	return nil
}

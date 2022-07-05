package model

import (
	"fmt"
	"hello/util"
	"time"
)

type CarryHandler func(setting *Setting, bidAsk *BidAsk)

type Setting struct {
	Valid            bool
	Function         string `gorm:"index:function_market_symbol,unique"`
	Market           string `gorm:"index:function_market_symbol,unique"`
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
		success, _, coin, _ := GetFromStandard(setting.Market, setting.Symbol)
		if success {
			coins[coin] = true
		}
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
		setting := &AppSettings[i]
		market := setting.Market
		function := setting.Function
		coin := setting.Coin
		if coinSettings[function] == nil {
			coinSettings[function] = make(map[string][]*Setting)
		}
		if coinSettings[function][coin] == nil {
			coinSettings[function][coin] = make([]*Setting, 0)
		}
		coinSettings[function][coin] = append(coinSettings[function][coin], setting)
		symbols := []string{setting.Symbol}
		for _, symbol := range symbols {
			if marketSymbolSetting[function] == nil {
				marketSymbolSetting[function] = make(map[string]map[string]*Setting)
			}
			if marketSymbolSetting[function][market] == nil {
				marketSymbolSetting[function][market] = make(map[string]*Setting)
			}
			marketSymbolSetting[function][market][symbol] = setting
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
	if AppSettings == nil || len(AppSettings) == 0 {
		LoadSettings()
	}
	marketMap := make(map[string]bool)
	for _, value := range AppSettings {
		marketMap[value.Market] = true
	}
	markets := make([]string, len(marketMap))
	i := 0
	for key := range marketMap {
		markets[i] = key
		i++
	}
	return markets
}

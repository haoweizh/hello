package model

import (
	"fmt"
	"hello/util"
	"strings"
	"time"
)

type CarryHandler func(setting *Setting, bidAsk *BidAsk)

type Setting struct {
	Valid             bool
	Function          string
	Market            string
	MarketRelated     string
	Symbol            string
	FunctionParameter string
	AccountType       string
	PriceX            float64
	OpenShortMargin   float64 // arbitrary future use
	CloseShortMargin  float64 // arbitrary future use
	Chance            int64   // arbitrary future use
	GridAmount        float64
	GridPriceDistance float64
	AmountLimit       float64
	RefreshLimit      float64
	RefreshLimitLow   float64
	BinanceDisMin     float64
	BinanceDisMax     float64
	ID                uint `gorm:"primary_key"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

var marketSymbolSetting map[string]map[string]map[string][]*Setting // function - marketName - symbol - setting
var handlers map[string]map[string]map[string]CarryHandler          // market - symbol - function- carryHandler

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
	for _, settings := range marketSymbolSetting[function][market] {
		for _, setting := range settings {
			if setting == nil {
				continue
			}
			coins[setting.GetCoin()] = true
		}
	}
	return
}

func GetSettings(function, market string) map[string][]*Setting {
	if handlers == nil {
		LoadSettings()
	}
	if marketSymbolSetting[function] == nil {
		return nil
	}
	return marketSymbolSetting[function][market]
}

func GetSetting(function, market, symbol string) []*Setting {
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
		for _, item := range value {
			if item != nil && item.FunctionParameter == setting.FunctionParameter {
				currentN += item.Chance
			}
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

func LoadSettings() {
	infoLock.Lock()
	defer infoLock.Unlock()
	AppSettings = []Setting{}
	AppDB.Where(`valid = ?`, true).Find(&AppSettings)
	marketSymbolSetting = make(map[string]map[string]map[string][]*Setting)
	//binanceSettings := make(map[string]*Setting)
	relatedSettings := make(map[string]*Setting)
	handlers = make(map[string]map[string]map[string]CarryHandler)
	for i := range AppSettings {
		market := AppSettings[i].Market
		function := AppSettings[i].Function
		symbol := AppSettings[i].Symbol
		if marketSymbolSetting[function] == nil {
			marketSymbolSetting[function] = make(map[string]map[string][]*Setting)
		}
		if marketSymbolSetting[function][market] == nil {
			marketSymbolSetting[function][market] = make(map[string][]*Setting)
		}
		if marketSymbolSetting[function][market][symbol] == nil {
			marketSymbolSetting[function][market][symbol] = make([]*Setting, 0)
		}
		marketSymbolSetting[function][market][symbol] = append(marketSymbolSetting[function][market][symbol],
			&AppSettings[i])
		if AppSettings[i].MarketRelated != `` {
			marketsRelated := strings.Split(AppSettings[i].MarketRelated, `,`)
			for _, value := range marketsRelated {
				AppSettings = append(AppSettings, Setting{Market: value, Symbol: AppSettings[i].Symbol, Valid: true})
			}
		}
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
	for _, setting := range relatedSettings {
		AppSettings = append(AppSettings, *setting)
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
			related := value.GetRelatedSymbol()
			symbols[value.Symbol] = true
			if related != `` {
				symbols[related] = true
			}
		}
	}
	return symbols
}

func GetMarkets() []string {
	if AppSettings == nil {
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

func GetInstrumentSymbol(market, instrument string) (symbol string) {
	switch market {
	case OKEX, OKSwap:
		return instrument
	case OKFUTURE:
		parts := strings.Split(instrument, `-`)
		if len(parts) > 2 {
			return parts[0] + `-` + parts[1]
		}
	}
	return
}

func (setting *Setting) GetRelatedSymbol() (related string) {
	switch setting.Market {
	case Ftx:
		parts := strings.Split(setting.Symbol, `-`)
		if len(parts) == 2 && setting.Function == FunctionCarry {
			related = parts[0] + `/USD`
		}
	case OKEX:
		parts := strings.Split(setting.Symbol, `-`)
		if len(parts) > 2 {
			if parts[1] == `USD` {
				parts[1] = `USDT`
			}
			return parts[0] + `-` + parts[1]
		}
	}
	return related
}

func (setting *Setting) GetCoin() (coin string) {
	switch setting.Market {
	case Ftx, OKFUTURE: // ftx:BTC-PERP okfuture:btc-usd
		parts := strings.Split(setting.Symbol, `-`)
		if len(parts) == 2 {
			coin = parts[0]
		}
	case HuobiDM: // btc_cq
		parts := strings.Split(setting.Symbol, `_`)
		if len(parts) == 2 {
			coin = parts[0]
		}
	}
	return coin
}

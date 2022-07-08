package api

import (
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"os"
	"sync"
)

var symbolSettings *sync.Map // function*market - map[symbol]*setting
var handlers *sync.Map       //market*symbol / map[function]carryHandler
var coinSettings *sync.Map   // function / map[coin][]*model.Setting
var appSettings []model.Setting
var appMarkets []string
var crossLen int

func GetSettingCoins(function, market string) (coins map[string]bool) {
	handlerInitialized := false
	handlers.Range(func(key, value interface{}) bool {
		handlerInitialized = true
		return true
	})
	if !handlerInitialized {
		LoadSettings()
	}
	value, ok := util.LoadSyncMap(symbolSettings, function, market)
	if ok {
		coins = make(map[string]bool)
		for _, setting := range value.(map[string]*model.Setting) {
			if setting == nil {
				continue
			}
			success, _, coin, _ := model.GetFromStandard(setting.Market, setting.Symbol)
			if success {
				coins[coin] = true
			}
		}
	}
	return
}

func GetSettings(function, market string) map[string]*model.Setting {
	handlerInitialized := false
	handlers.Range(func(key, value interface{}) bool {
		handlerInitialized = true
		return true
	})
	if !handlerInitialized {
		LoadSettings()
	}
	value, ok := util.LoadSyncMap(symbolSettings, function, market)
	if ok {
		return value.(map[string]*model.Setting)
	}
	return nil
}

func GetSetting(function, market, symbol string) *model.Setting {
	settings := GetSettings(function, market)
	if settings != nil {
		return settings[symbol]
	}
	return nil
}

func GetCurrentN(setting *model.Setting) (currentN int64) {
	settings := GetSettings(setting.Function, setting.Market)
	if settings == nil {
		return 0
	}
	for _, value := range settings {
		if value != nil && value.Market == setting.Market && value.Function == setting.Function {
			currentN += value.Chance
		}
	}
	return currentN
}

func GetFunctions(market, symbol string) map[string]model.CarryHandler {
	handlerInitialized := false
	handlers.Range(func(key, value interface{}) bool {
		handlerInitialized = true
		return true
	})
	if !handlerInitialized {
		LoadSettings()
	}
	value, ok := util.LoadSyncMap(handlers, market, symbol)
	if ok {
		return value.(map[string]model.CarryHandler)
	}
	return nil
}

func LoadSettings() {
	symbolSettings = &sync.Map{}
	handlers = &sync.Map{}
	coinSettings = &sync.Map{}
	appSettings = []model.Setting{}
	marketMap := make(map[string]bool)
	model.AppDB.Where(`valid = ?`, true).Find(&appSettings)
	for _, setting := range appSettings {
		market := setting.Market
		marketMap[market] = true
		function := setting.Function
		coin := setting.Coin
		value, ok := util.LoadSyncMap(handlers, market, setting.Symbol)
		var functions map[string]model.CarryHandler
		if ok {
			functions = value.(map[string]model.CarryHandler)
		} else {
			functions = make(map[string]model.CarryHandler)
		}
		functions[function] = model.HandlerMap[function]
		util.StoreSyncMap(handlers, functions, market, setting.Symbol)

		var settings map[string][]*model.Setting
		value, ok = coinSettings.Load(function)
		if ok {
			settings = value.(map[string][]*model.Setting)
		} else {
			settings = make(map[string][]*model.Setting)
		}
		if settings[coin] == nil {
			settings[coin] = make([]*model.Setting, 0)
		}
		settings[coin] = append(settings[coin], &setting)
		coinSettings.Store(function, settings)

		var functionMarketSettings map[string]*model.Setting
		value, ok = util.LoadSyncMap(symbolSettings, function, market)
		if ok {
			functionMarketSettings = value.(map[string]*model.Setting)
		} else {
			functionMarketSettings = make(map[string]*model.Setting)
		}
		functionMarketSettings[setting.Symbol] = &setting
		util.StoreSyncMap(symbolSettings, functionMarketSettings, function, market)

		util.Notice(fmt.Sprintf(`load setting %s %s %s %v`,
			setting.Market, setting.Symbol, setting.Function, setting.Valid))
	}
	appMarkets = make([]string, len(marketMap))
	i := 0
	for key := range marketMap {
		appMarkets[i] = key
		i++
	}
}

func GetMarketSymbols(market string) map[string]bool {
	if appSettings == nil {
		LoadSettings()
	}
	symbols := make(map[string]bool)
	for _, value := range appSettings {
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

func GetCoinSettings(function string) map[string][]*model.Setting {
	if appSettings == nil {
		LoadSettings()
	}
	value, ok := coinSettings.Load(function)
	if ok {
		return value.(map[string][]*model.Setting)
	}
	return nil
}

func GetMarkets() []string {
	if appSettings == nil || len(appSettings) == 0 {
		LoadSettings()
	}
	return appMarkets
}

func GetAccounts(index int) (accounts map[string]*model.Account) {
	if model.AppAccounts != nil && len(model.AppAccounts) > index {
		return model.AppAccounts[index]
	}
	tempAccounts := model.AppConfig.GetAccounts(model.Ftx)
	size := int(math.Max(float64(GetCrossLen()), float64(len(tempAccounts))))
	model.AppAccounts = make([]map[string]*model.Account, size)
	for i := 0; i < size; i++ {
		if model.AppAccounts[i] == nil {
			model.AppAccounts[i] = make(map[string]*model.Account)
		}
	}
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.Ftx] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.OKEX)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.OKEX] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.BinanceSpot)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.BinanceSpot] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.BinancePerp)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.BinancePerp] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.Gate)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.Gate] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.BybitPerp)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.BybitPerp] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.BybitSpot)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.BybitSpot] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.Kucoin)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.Kucoin] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.Mexc)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.Mexc] = account
	}
	return model.AppAccounts[index]
}

func GetCrossLen() int {
	if crossLen > 0 {
		return crossLen
	}
	markets := GetMarkets()
	for _, market := range markets {
		accounts := model.AppConfig.GetAccounts(market)
		if crossLen == 0 {
			crossLen = len(accounts)
		} else if len(accounts) != crossLen {
			util.Notice(fmt.Sprintf(`wrong cross config %s accounts:%d`, market, len(accounts)))
			os.Exit(2)
		}
	}
	return crossLen
}

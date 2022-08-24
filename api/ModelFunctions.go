package api

import (
	"fmt"
	"hello/model"
	"hello/util"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
)

var symbolSettings *sync.Map // function*market - map[symbol]*setting
var handlers *sync.Map       //market*symbol / *sync.Map:map[function]carryHandler
var coinSettings *sync.Map   // function / *sync.Map:map[coin][]*model.Setting
var appSettings []model.Setting
var appMarkets []string
var crossLen int
var settingLoading bool

func GetSettingCoins(function, market string) (coins map[string]bool) {
	handlerInitialized := false
	handlers.Range(func(key, value interface{}) bool {
		handlerInitialized = true
		return true
	})
	if !handlerInitialized {
		util.Notice(`load setting GetSettingCoins %s %s`, function, market)
		if !LoadSettings() {
			return nil
		}
	}
	value, ok := util.LoadSyncMap(symbolSettings, function, market)
	if ok {
		coins = make(map[string]bool)
		value.(*sync.Map).Range(func(key, setting interface{}) bool {
			if setting == nil {
				return true
			}
			success, _, coin, _ := model.GetFromStandard(setting.(*model.Setting).Market, setting.(*model.Setting).Symbol)
			if success {
				coins[coin] = true
			}
			return true
		})
	}
	return
}

func GetSettings(function, market string) *sync.Map {
	handlerInitialized := false
	handlers.Range(func(key, value interface{}) bool {
		handlerInitialized = true
		return true
	})
	if !handlerInitialized {
		util.Notice(`load setting GetSettings %s %s`, function, market)
		//if !LoadSettings() {
		//return nil
		//}
		LoadSettings()
	}
	value, ok := util.LoadSyncMap(symbolSettings, function, market)
	if ok {
		return value.(*sync.Map)
	}
	return nil
}

func GetSetting(function, market, symbol string) *model.Setting {
	settings := GetSettings(function, market)
	if settings != nil {
		value, _ := settings.Load(symbol)
		if value != nil {
			return value.(*model.Setting)
		}
	}
	return nil
}

func GetCurrentN(setting *model.Setting) (currentN int64) {
	settings := GetSettings(setting.Function, setting.Market)
	if settings == nil {
		return 0
	}
	settings.Range(func(key, value interface{}) bool {
		valueSetting := value.(*model.Setting)
		if value != nil && valueSetting.Market == setting.Market && valueSetting.Function == setting.Function {
			currentN += valueSetting.Chance
		}
		return true
	})
	return currentN
}

func GetFunctions(market, symbol string) *sync.Map {
	handlerInitialized := false
	handlers.Range(func(key, value interface{}) bool {
		handlerInitialized = true
		return true
	})
	if !handlerInitialized {
		util.Notice(`load setting GetFunctions %s %s`, market, symbol)
		if !LoadSettings() {
			return nil
		}
	}
	value, ok := util.LoadSyncMap(handlers, market, symbol)
	if ok {
		return value.(*sync.Map)
	}
	return nil
}

const topMarketInfoLen = 15

func prepareSettings() {
	localSymbolSettings := &sync.Map{}
	localHandlers := &sync.Map{}
	localCoinSettings := &sync.Map{}
	appSettings = []model.Setting{}
	marketMap := make(map[string]bool)
	model.AppDB.Where(`valid = ?`, true).Find(&appSettings)
	util.Notice(`start to load settings %d`, len(appSettings))
	for i := 0; i < len(appSettings); i++ {
		setting := &appSettings[i]
		//util.Notice(fmt.Sprintf(`load setting %s %s %s %v`,
		//	setting.Market, setting.Symbol, setting.Function, setting.Valid))
		marketMap[setting.Market] = true
		value, ok := util.LoadSyncMap(localHandlers, setting.Market, setting.Symbol)
		var functions *sync.Map
		if ok {
			functions = value.(*sync.Map)
		}
		if functions == nil {
			functions = &sync.Map{}
		}
		functions.Store(setting.Function, model.HandlerMap[setting.Function])
		util.StoreSyncMap(localHandlers, functions, setting.Market, setting.Symbol)
		var settings *sync.Map
		value, ok = localCoinSettings.Load(setting.Function)
		if ok {
			settings = value.(*sync.Map)
		}
		if settings == nil {
			settings = &sync.Map{}
		}
		settingArray, _ := settings.Load(setting.Coin)
		if settingArray == nil {
			settingArray = make([]*model.Setting, 0)
		}
		exist := false
		for _, item := range settingArray.([]*model.Setting) {
			if item.Market == setting.Market && item.Symbol == setting.Symbol && item.Function == setting.Function {
				exist = true
			}
		}
		if !exist {
			settingArray = append(settingArray.([]*model.Setting), setting)
		}
		//util.Notice(fmt.Sprintf(`add setting array %s %s %d`, setting.Market, setting.Symbol, len(settingArray.([]*model.Setting))))
		settings.Store(setting.Coin, settingArray)
		localCoinSettings.Store(setting.Function, settings)
		var functionMarketSettings *sync.Map
		value, ok = util.LoadSyncMap(localSymbolSettings, setting.Function, setting.Market)
		if ok {
			functionMarketSettings = value.(*sync.Map)
		}
		if functionMarketSettings == nil {
			functionMarketSettings = &sync.Map{}
		}
		functionMarketSettings.Store(setting.Symbol, setting)
		util.StoreSyncMap(localSymbolSettings, functionMarketSettings, setting.Function, setting.Market)
	}
	localAppMarkets := make([]string, len(marketMap))
	i := 0
	for key := range marketMap {
		localAppMarkets[i] = key
		i++
	}
	appMarkets = localAppMarkets
	coinSettings = localCoinSettings
	handlers = localHandlers
	symbolSettings = localSymbolSettings
}

func handleSettings() (handled bool) {
	for _, market := range appMarkets {
		_, hava := util.LoadSyncMap(symbolSettings, model.FunctionDynamicTurtle, market)
		if !hava {
			continue
		}
		handled = true
		topMarketInfos := make(map[string]*model.MarketInfo)
		marketInfos := GetMarketInfos(market)
		marketInfoArray := model.MarketInfoArray{}
		for _, info := range marketInfos {
			marketInfoArray = append(marketInfoArray, info)
		}
		sort.Sort(sort.Reverse(marketInfoArray))
		for i := 0; i < marketInfoArray.Len() && len(topMarketInfos) < topMarketInfoLen; i++ {
			_, marketType, coinValue, _ := model.GetFromStandard(market, marketInfoArray[i].Name)
			if strings.EqualFold(marketType, model.MarketTypePerp) && !model.CommonCoins[strings.ToLower(coinValue)] {
				topMarketInfos[marketInfoArray[i].Name] = marketInfoArray[i]
			}
		}
		var settings *sync.Map
		value, ok := util.LoadSyncMap(symbolSettings, model.FunctionTurtle, market)
		if ok {
			settings = value.(*sync.Map)
		} else {
			settings = &sync.Map{}
		}
		for _, info := range topMarketInfos {
			value, ok = settings.Load(info.Name)
			if value == nil {
				setting := &model.Setting{
					Valid:           true,
					Function:        model.FunctionTurtle,
					Market:          market,
					Symbol:          info.Name,
					Chance:          0,
					GridAmount:      0,
					OpenShortMargin: 3,
					AmountLimit:     12,
				}
				model.AppDB.Save(setting)
				util.Notice(`add setting %v`, setting.Symbol)
			} else if value.(*model.Setting).SymbolRelated == model.SettingTurtleRemoved {
				value.(*model.Setting).SymbolRelated = ``
				model.AppDB.Save(value.(*model.Setting))
				util.Notice(`add setting back %s`, info.Name)
			}
		}
		settings.Range(func(symbol, setting interface{}) bool {
			_, _, coinValue, _ := model.GetFromStandard(market, symbol.(string))
			if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] &&
				setting.(*model.Setting).Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
				model.AppDB.Save(setting)
				util.Notice(`add setting remove %s`, setting.(*model.Setting).Symbol)
			} else if market == model.BinancePerp {
				accounts := model.AppConfig.GetAccounts(model.BinancePerp)
				for _, account := range accounts {
					success := SetLeverageBinancePerp(account.Key, account.Secret, symbol.(string), 5)
					if !success {
						util.Notice(fmt.Sprintf(`fail to set leverage binanceperp %s`, symbol.(string)))
					}
				}
			}
			return true
		})
	}
	return
}

func LoadSettings() bool {
	if settingLoading {
		return false
	}
	prepareSettings()
	if handleSettings() {
		prepareSettings()
	}
	settingLoading = false
	util.Notice(`finish load settings`)
	return true
}

func GetMarketSymbols(market string) map[string]bool {
	if appSettings == nil {
		util.Notice(`load setting GetMarketSymbols %s`, market)
		if !LoadSettings() {
			return nil
		}
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

func GetCoinSettings(function string) *sync.Map {
	if appSettings == nil {
		util.Notice(`load setting GetCoinSettings %s`, function)
		if !LoadSettings() {
			return nil
		}
	}
	value, ok := coinSettings.Load(function)
	if ok {
		return value.(*sync.Map)
	}
	return nil
}

func GetMarkets() []string {
	if appSettings == nil || len(appSettings) == 0 {
		util.Notice(`load setting GetMarkets`)
		if !LoadSettings() {
			return nil
		}
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
	tempAccounts = model.AppConfig.GetAccounts(model.KucoinSpot)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.KucoinSpot] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.KucoinPerp)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.KucoinPerp] = account
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

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
	"time"
)

var symbolSettings = &sync.Map{} // function*market - map[symbol]*setting
var handlers = &sync.Map{}       //market*symbol / *sync.Map:map[function]carryHandler
var coinSettings = &sync.Map{}   // function / *sync.Map:map[coin][]*model.Setting
var appSettings []model.Setting
var appMarkets []string
var crossLen int
var settingLoading bool
var invalidTurtles = &sync.Map{} //function*market*symbol *setting

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

func GetSettings(function, market string) (settingMap *sync.Map, symbols []string) {
	//handlerInitialized := false
	//if handlers != nil {
	//	handlers.Range(func(key, value interface{}) bool {
	//		handlerInitialized = true
	//		return true
	//	})
	//}
	//if !handlerInitialized {
	//	util.Notice(`load setting GetSettings %s %s`, function, market)
	//	LoadSettings()
	//}
	symbols = make([]string, 0)
	value, ok := util.LoadSyncMap(symbolSettings, function, market)
	if ok {
		value.(*sync.Map).Range(func(symbol, value any) bool {
			//util.Notice(`get symbol %s`, symbol)
			symbols = append(symbols, symbol.(string))
			return true
		})
		return value.(*sync.Map), symbols
	}
	return nil, nil
}

// GetInvalidTurtle
// 允许返回valid=false的setting
// antiTurtle的function为model.FunctionTurtle
func GetInvalidTurtle(market, symbol string) *model.Setting {
	value, ok := util.LoadSyncMap(invalidTurtles, model.FunctionTurtle, market, symbol)
	if ok && value != nil {
		return value.(*model.Setting)
	}
	appSettings = []model.Setting{}
	model.AppDB.Where(`function=? and market=? and symbol=?`, model.FunctionTurtle, market, symbol).Find(&appSettings)
	if len(appSettings) == 1 {
		util.StoreSyncMap(invalidTurtles, &appSettings[0], model.FunctionTurtle, market, symbol)
		return &appSettings[0]
	}
	return nil
}

func GetSetting(function, market, symbol string) *model.Setting {
	settings, _ := GetSettings(function, market)
	if settings != nil {
		value, _ := settings.Load(symbol)
		if value != nil {
			return value.(*model.Setting)
		}
	}
	return nil
}

//// GetChanceInAll
//// setting是主流币种：返回function market下所有主流币种仓数sum
//// setting是非主流币种：返回function market下有仓位的币种个数
//func GetChanceInAll(function, market, symbol string) (inALL int64) {
//	success, _, coin, _ := model.GetFromStandard(market, symbol)
//	if !success {
//		return 0
//	}
//	settings := GetSettings(function, market)
//	if settings == nil {
//		return 0
//	}
//	if model.CommonCoins[strings.ToLower(coin)] {
//		settings.Range(func(key, value interface{}) bool {
//			if value == nil {
//				return false
//			}
//			valueSetting := value.(*model.Setting)
//			_, _, valueCoin, _ := model.GetFromStandard(valueSetting.Market, valueSetting.Symbol)
//			if valueSetting.Market == market && valueSetting.Function == function && model.CommonCoins[strings.ToLower(valueCoin)] {
//				inALL += valueSetting.Chance
//			}
//			return true
//		})
//	} else {
//		settings.Range(func(key, value any) bool {
//			if value == nil {
//				return false
//			}
//			valueSetting := value.(*model.Setting)
//			_, _, valueCoin, _ := model.GetFromStandard(valueSetting.Market, valueSetting.Symbol)
//			var settingInvalid *model.Setting
//			if valueSetting.Function == model.FunctionCombineTurtle {
//				settingInvalid = GetInvalidTurtle(valueSetting.Market, valueSetting.Symbol)
//			}
//			if valueSetting.Market == market && valueSetting.Function == function && !model.CommonCoins[strings.ToLower(valueCoin)] {
//				if (settingInvalid != nil && settingInvalid.Chance != 0) || valueSetting.Chance != 0 {
//					inALL++
//				}
//			}
//			return true
//		})
//	}
//	return inALL
//}

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

const topMarketInfoLen = 12

func prepareSettings() {
	localSymbolSettings := &sync.Map{}
	localHandlers := &sync.Map{}
	localCoinSettings := &sync.Map{}
	appSettings = []model.Setting{}
	marketMap := make(map[string]bool)
	model.AppDB.Where(`valid = ?`, true).Find(&appSettings)
	util.Notice(`start to load settings %d`, len(appSettings))
	go func() {
		for _, setting := range appSettings {
			if setting.Market == model.BinancePerp {
				accounts := model.AppConfig.GetAccounts(model.BinancePerp)
				for _, account := range accounts {
					success := SetLeverageBinancePerp(account.Key, account.Secret, setting.Symbol, 5)
					if success {
						util.Notice(fmt.Sprintf(`set leverage binanceperp %s`, setting.Symbol))
					} else {
						util.Notice(fmt.Sprintf(`fail to set leverage binanceperp %s`, setting.Symbol))
						time.Sleep(time.Minute)
					}
				}
				time.Sleep(time.Second)
			}
		}
	}()
	for i := 0; i < len(appSettings); i++ {
		setting := &appSettings[i]
		value, ok := util.LoadSyncMap(symbolSettings, setting.Function, setting.Market)
		if ok && value != nil {
			oldSetting, oldOk := value.(*sync.Map).Load(setting.Symbol)
			if oldOk && oldSetting != nil {
				setting = oldSetting.(*model.Setting)
			}
		}
		util.Notice(fmt.Sprintf(`load setting %s %s %s %v %d`,
			setting.Market, setting.Symbol, setting.Function, setting.Valid, setting.Chance))
		marketMap[setting.Market] = true
		value, ok = util.LoadSyncMap(localHandlers, setting.Market, setting.Symbol)
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
		_, haveDynamic := util.LoadSyncMap(symbolSettings, model.FunctionDynamicTurtle, market)
		_, haveCombine := util.LoadSyncMap(symbolSettings, model.FunctionDynamicCombine, market)
		accounts := model.AppConfig.GetAccounts(market)
		if (!haveDynamic && !haveCombine) || accounts == nil || len(accounts) == 0 {
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
				turtleData := GetTurtleData(accounts[0].Key, accounts[0].Secret, model.FunctionTurtle, market, marketInfoArray[i].Name)
				if turtleData != nil {
					topMarketInfos[marketInfoArray[i].Name] = marketInfoArray[i]
				}
			}
		}
		mapTurtle := &sync.Map{}
		mapCombine := &sync.Map{}
		value, ok := util.LoadSyncMap(symbolSettings, model.FunctionTurtle, market)
		if ok && value != nil {
			mapTurtle = value.(*sync.Map)
		}
		value, ok = util.LoadSyncMap(symbolSettings, model.FunctionCombineTurtle, market)
		if ok && value != nil {
			mapCombine = value.(*sync.Map)
		}
		for _, info := range topMarketInfos {
			value, ok = mapTurtle.Load(info.Name)
			settingTurtle := &model.Setting{Valid: true, Function: model.FunctionTurtle, Market: market, Symbol: info.Name,
				OpenShortMargin: 3, AmountLimit: 10}
			if value == nil {
				if settingTurtle.Market == model.BinancePerp {
					for _, account := range accounts {
						success := SetLeverageBinancePerp(account.Key, account.Secret, settingTurtle.Symbol, 5)
						if success {
							util.Notice(fmt.Sprintf(`set leverage binanceperp %s`, settingTurtle.Symbol))
						} else {
							util.Notice(fmt.Sprintf(`fail to set leverage binanceperp %s`, settingTurtle.Symbol))
							time.Sleep(time.Minute)
						}
					}
					time.Sleep(time.Second)
				}
				util.Notice(`add settingTurtle %v`, settingTurtle.Symbol)
			} else {
				settingTurtle = value.(*model.Setting)
				settingTurtle.SymbolRelated = ``
				util.Notice(`add settingTurtle back %s`, info.Name)
			}
			if haveCombine {
				settingTurtle.Valid = false
				value, ok = mapCombine.Load(info.Name)
				settingCombine := &model.Setting{Valid: true, Function: model.FunctionCombineTurtle, Market: market, Symbol: info.Name,
					OpenShortMargin: 3, AmountLimit: 10}
				if value != nil {
					settingCombine = value.(*model.Setting)
					settingCombine.SymbolRelated = ``
					util.Notice(`add settingCombine back %s`, info.Name)
				}
				model.AppDB.Save(settingCombine)
			}
			model.AppDB.Save(settingTurtle)
		}
		handleRemove := func(symbol, setting interface{}) bool {
			_, _, coinValue, _ := model.GetFromStandard(market, symbol.(string))
			if setting == nil {
				return true
			}
			if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
				if setting.(*model.Setting).Chance == 0 {
					setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
				}
				model.AppDB.Save(setting)
				util.Notice(`add setting remove %s`, setting.(*model.Setting).Symbol)
			}
			return true
		}
		mapTurtle.Range(handleRemove)
		mapCombine.Range(handleRemove)
	}
	return
}

func LoadSettings() bool {
	if settingLoading {
		return false
	}
	prepareSettings()
	for _, market := range appMarkets {
		setRequireReset(market)
	}
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
			symbols[value.Symbol] = true
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

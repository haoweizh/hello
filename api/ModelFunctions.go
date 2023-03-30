package api

import (
	"fmt"
	"hello/model"
	"hello/util"
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

const dynamicSingleLimit = 3
const dynamicInAllLimit = 10

var processLock sync.Mutex
var processing = &sync.Map{}

func CheckSetProcessing(function, market, symbol string, value bool) (before bool) {
	processLock.Lock()
	defer processLock.Unlock()
	v, ok := util.LoadSyncMap(processing, function, market, symbol)
	if ok && v != nil {
		before = v.(bool)
	}
	if value == false || before == false {
		util.StoreSyncMap(processing, value, function, market, symbol)
	}
	return before
}

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

func GetSettings(function, market string) (settingMap *sync.Map) {
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
	value, ok := util.LoadSyncMap(symbolSettings, function, market)
	if ok {
		return value.(*sync.Map)
	}
	return nil
}

func GetSetting(function, market, symbol string) *model.Setting {
	settings := GetSettings(function, market)
	if settings != nil {
		value, ok := settings.Load(symbol)
		if ok && value != nil {
			return value.(*model.Setting)
		}
	}
	return nil
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
	if ok && value != nil {
		return value.(*sync.Map)
	}
	return nil
}

const topMarketInfoLen = 12

func PrepareSettings() {
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
					if len(strings.Trim(setting.Symbol, ` `)) == 0 {
						continue
					}
					success := SetLeverageBinancePerp(account.Key, account.Secret, setting.Symbol, 5)
					if success {
						//util.Notice(fmt.Sprintf(`set leverage binanceperp %s`, setting.Symbol))
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
		//util.Notice(fmt.Sprintf(`load setting %s %s %s %v %d`,
		//	setting.Market, setting.Symbol, setting.Function, setting.Valid, setting.Chance))
		marketMap[setting.Market] = true
		value, ok = util.LoadSyncMap(localHandlers, setting.Market, setting.Symbol)
		var functions *sync.Map
		if ok {
			functions = value.(*sync.Map)
		}
		if functions == nil {
			functions = &sync.Map{}
		}
		if !model.IgnoreFunctions[setting.Function] {
			functions.Store(setting.Function, model.HandlerMap[setting.Function])
		}
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
			//util.Notice(fmt.Sprintf(`add setting array %s %s %d`, setting.Market, setting.Symbol, len(settingArray.([]*model.Setting))))
			settings.Store(setting.Coin, settingArray)
		}
		localCoinSettings.Store(setting.Function, settings)
		var functionMarketSettings *sync.Map
		value, ok = util.LoadSyncMap(localSymbolSettings, setting.Function, setting.Market)
		if ok {
			functionMarketSettings = value.(*sync.Map)
		}
		if functionMarketSettings == nil {
			functionMarketSettings = &sync.Map{}
		}
		util.Notice(fmt.Sprintf(`add setting %s %s %s`, setting.Function, setting.Market, setting.Symbol))
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

func handleCombineSettings(market string, topMarketInfos map[string]*model.MarketInfo) {
	combineMap := &sync.Map{}
	value, ok := util.LoadSyncMap(symbolSettings, model.FunctionCombineTurtle, market)
	if ok && value != nil {
		combineMap = value.(*sync.Map)
	}
	normalMap := &sync.Map{}
	value, ok = util.LoadSyncMap(symbolSettings, model.FunctionTurtleNormal, market)
	if ok && value != nil {
		normalMap = value.(*sync.Map)
	}
	for _, info := range topMarketInfos {
		valueCombine, _ := combineMap.Load(info.Name)
		valueNormal, _ := normalMap.Load(info.Name)
		settingCombine := &model.Setting{Valid: true, Function: model.FunctionCombineTurtle, Market: market, Symbol: info.Name,
			OpenShortMargin: dynamicSingleLimit, AmountLimit: dynamicInAllLimit}
		settingNormal := &model.Setting{Valid: true, Function: model.FunctionTurtleNormal, Market: market, Symbol: info.Name,
			OpenShortMargin: dynamicSingleLimit, AmountLimit: dynamicInAllLimit}
		if valueCombine == nil {
			util.Notice(`add combine binance %v`, info.Name)
		} else {
			settingCombine = valueCombine.(*model.Setting)
			settingCombine.SymbolRelated = ``
			util.Notice(`add back combine %s`, info.Name)
		}
		if valueNormal != nil {
			settingNormal = valueNormal.(*model.Setting)
			settingNormal.SymbolRelated = ``
			util.Notice(`add back normal %s`, info.Name)
		}
		model.AppDB.Save(settingCombine)
		model.AppDB.Save(settingNormal)
	}
	combineMap.Range(func(symbol, setting interface{}) bool {
		if setting == nil {
			return true
		}
		var normalSetting *model.Setting
		normalValue, _ := normalMap.Load(symbol)
		if normalValue == nil {
			normalSetting = &model.Setting{Valid: true, Function: model.FunctionTurtleNormal, Market: market,
				Symbol: symbol.(string), OpenShortMargin: dynamicSingleLimit, AmountLimit: dynamicInAllLimit}
			model.AppDB.Save(normalSetting)
		} else {
			normalSetting = normalValue.(*model.Setting)
		}
		_, _, coinValue, _ := model.GetFromStandard(market, symbol.(string))
		if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
			if setting.(*model.Setting).Chance == 0 && normalSetting.Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
			util.Notice(`remove setting%s`, setting.(*model.Setting).Symbol)
		}
		return true
	})
	normalMap.Range(func(symbol, setting interface{}) bool {
		if setting == nil {
			return true
		}
		var combineSetting *model.Setting
		combineValue, _ := combineMap.Load(symbol)
		if combineValue == nil {
			combineSetting = &model.Setting{Valid: true, Function: model.FunctionCombineTurtle, Market: market,
				Symbol: symbol.(string), OpenShortMargin: dynamicSingleLimit, AmountLimit: dynamicInAllLimit}
			model.AppDB.Save(combineSetting)
		} else {
			combineSetting = combineValue.(*model.Setting)
		}
		_, _, coinValue, _ := model.GetFromStandard(market, symbol.(string))
		if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
			if setting.(*model.Setting).Chance == 0 && combineSetting.Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
		}
		util.Notice(`remove setting%s`, setting.(*model.Setting).Symbol)
		return true
	})
}

func handleTurtleSettings(function, market string, topMarketInfos map[string]*model.MarketInfo) {
	settingMap := &sync.Map{}
	value, ok := util.LoadSyncMap(symbolSettings, function, market)
	if ok && value != nil {
		settingMap = value.(*sync.Map)
	}
	for _, info := range topMarketInfos {
		value, _ = settingMap.Load(info.Name)
		settingTurtle := &model.Setting{Valid: true, Function: function, Market: market, Symbol: info.Name,
			OpenShortMargin: dynamicSingleLimit, AmountLimit: dynamicInAllLimit}
		if value == nil {
			util.Notice(`add settingTurtle %v`, settingTurtle.Symbol)
		} else {
			settingTurtle = value.(*model.Setting)
			settingTurtle.SymbolRelated = ``
			util.Notice(`add settingTurtle back %s`, info.Name)
		}
		model.AppDB.Save(settingTurtle)
	}
	settingMap.Range(func(symbol, setting interface{}) bool {
		if setting == nil {
			return true
		}
		_, _, coinValue, _ := model.GetFromStandard(market, symbol.(string))
		if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
			if setting.(*model.Setting).Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
			util.Notice(`remove setting%s`, setting.(*model.Setting).Symbol)
		}
		return true
	})
}

func getTopMarketInfos(function, market string, accounts []*model.Account) (topMarketInfos map[string]*model.MarketInfo) {
	topMarketInfos = make(map[string]*model.MarketInfo)
	marketInfoArray := model.MarketInfoArray{}
	model.MarketInfos.Range(func(key, value any) bool {
		if value == nil {
			return true
		}
		if value.(*model.MarketInfo).Market == market {
			marketInfoArray = append(marketInfoArray, value.(*model.MarketInfo))
		}
		return true
	})
	util.Notice(fmt.Sprintf(`get top market info array %d`, len(marketInfoArray)))
	sort.Sort(sort.Reverse(marketInfoArray))
	for i := 0; i < marketInfoArray.Len() && len(topMarketInfos) < topMarketInfoLen; i++ {
		_, marketType, coinValue, _ := model.GetFromStandard(market, marketInfoArray[i].Name)
		if strings.EqualFold(marketType, model.MarketTypePerp) && !model.CommonCoins[strings.ToLower(coinValue)] {
			turtleData := GetTurtleData(accounts[0].Key, accounts[0].Secret, function, market, marketInfoArray[i].Name)
			if turtleData != nil {
				topMarketInfos[marketInfoArray[i].Name] = marketInfoArray[i]
				util.Notice(fmt.Sprintf(`get top turtle done %s %s`, market, marketInfoArray[i].Name))
			} else {
				util.Notice(fmt.Sprintf(`get top turtle data fail %s %s`, market, marketInfoArray[i].Name))
			}
		}
	}
	return topMarketInfos
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
		var function = ``
		if haveCombine {
			function = model.FunctionCombineTurtle
		} else if haveDynamic {
			function = model.FunctionTurtle
		}
		topMarketInfos := getTopMarketInfos(function, market, accounts)
		if haveCombine {
			handleCombineSettings(market, topMarketInfos)
		} else if haveDynamic {
			handleTurtleSettings(function, market, topMarketInfos)
		}
	}
	return
}

func LoadSettings() bool {
	if settingLoading {
		return false
	}
	for _, market := range appMarkets {
		setRequireReset(market)
	}
	if handleSettings() {
		PrepareSettings()
	}
	util.Notice(`finish load settings`)
	time.Sleep(time.Second * 5)
	settingLoading = false
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
	util.Notice(`load setting GetMarkets %d`, len(appMarkets))
	return appMarkets
}

func GetAccounts(index int) (accounts map[string]*model.Account) {
	if model.AppAccounts != nil && len(model.AppAccounts) > index {
		return model.AppAccounts[index]
	}
	// 注意: 以okex的key个数作为size，如果不使用okex，请及时更换
	size := len(model.OKEX)
	model.AppAccounts = make([]map[string]*model.Account, size)
	for i := 0; i < size; i++ {
		if model.AppAccounts[i] == nil {
			model.AppAccounts[i] = make(map[string]*model.Account)
		}
	}
	tempAccounts := model.AppConfig.GetAccounts(model.Ftx)
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
	tempAccounts = model.AppConfig.GetAccounts(model.Bybit)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.Bybit] = account
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
	tempAccounts = model.AppConfig.GetAccounts(model.BitgetSpot)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.BitgetSpot] = account
	}
	tempAccounts = model.AppConfig.GetAccounts(model.BitgetPerp)
	for i, account := range tempAccounts {
		model.AppAccounts[i][model.BitgetPerp] = account
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
			fmt.Println(fmt.Sprintf(`wrong cross config %s accounts:%d`, market, len(accounts)))
			os.Exit(2)
		}
	}
	return crossLen
}

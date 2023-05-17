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

var DynamicHandleTime = &sync.Map{} // market - handle time.Time
var symbolSettings = &sync.Map{}    // function*market - map[symbol]*setting
var handlers = &sync.Map{}          //market*symbol / *sync.Map:map[function]carryHandler
var coinSettings = &sync.Map{}      // function / *sync.Map:map[coin][]*model.Setting
var appSettings []model.Setting
var appMarkets []string
var crossLen int
var settingLoading bool
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
		if !InitApp(true) {
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
	//	InitApp()
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
		if !InitApp(true) {
			return nil
		}
	}
	value, ok := util.LoadSyncMap(handlers, market, symbol)
	if ok && value != nil {
		return value.(*sync.Map)
	}
	return nil
}

const topMarketInfoLen = 30
const topTurtleDataLen = 10

func PrepareSettings() {
	localSymbolSettings := &sync.Map{}
	localHandlers := &sync.Map{}
	localCoinSettings := &sync.Map{}
	appSettings = []model.Setting{}
	marketMap := make(map[string]bool)
	model.AppDB.Where(`valid = ?`, true).Find(&appSettings)
	util.Notice(`start to load settings %d`, len(appSettings))
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
		if setting.Function != model.FunctionCross {
			util.Notice(fmt.Sprintf(`load setting %s %s %s`, setting.Function, setting.Market, setting.Symbol))
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

func handleCombineSettings(dyComSetting *model.Setting, topMarketInfos map[string]*model.MarketInfo) {
	combineMap := &sync.Map{}
	value, ok := util.LoadSyncMap(symbolSettings, model.FunctionCombineTurtle, dyComSetting.Market)
	if ok && value != nil {
		combineMap = value.(*sync.Map)
	}
	normalMap := &sync.Map{}
	value, ok = util.LoadSyncMap(symbolSettings, model.FunctionTurtleNormal, dyComSetting.Market)
	if ok && value != nil {
		normalMap = value.(*sync.Map)
	}
	for _, info := range topMarketInfos {
		valueCombine, _ := combineMap.Load(info.Name)
		valueNormal, _ := normalMap.Load(info.Name)
		settingCombine := &model.Setting{Valid: true, Function: model.FunctionCombineTurtle, Market: dyComSetting.Market,
			Symbol: info.Name, OpenShortMargin: dyComSetting.OpenShortMargin, AmountLimit: dyComSetting.AmountLimit,
			Far: dyComSetting.Far, Near: dyComSetting.Near}
		settingNormal := &model.Setting{Valid: true, Function: model.FunctionTurtleNormal, Market: dyComSetting.Market,
			Symbol: info.Name, OpenShortMargin: dyComSetting.OpenShortMargin, AmountLimit: dyComSetting.AmountLimit,
			Far: dyComSetting.Far, Near: dyComSetting.Near}
		if valueCombine == nil {
			util.Notice(`add combine %s %v`, dyComSetting.Market, info.Name)
			if dyComSetting.Market == model.BinancePerp || dyComSetting.Market == model.Bybit {
				accounts := model.AppConfig.GetAccounts(dyComSetting.Market)
				for _, account := range accounts {
					if account != nil {
						if dyComSetting.Market == model.BinancePerp {
							SetSymbolLeverageBinancePerp(account, settingCombine.Symbol)
						} else if dyComSetting.Market == model.Bybit {
							setSymbolLeverageBybit(account, settingCombine.Symbol)
						}
						time.Sleep(time.Second * 10)
					}
				}
			}
		} else {
			settingCombine = valueCombine.(*model.Setting)
			settingCombine.SymbolRelated = ``
			util.Notice(`add back combine %s %s`, dyComSetting.Market, info.Name)
		}
		if valueNormal != nil {
			settingNormal = valueNormal.(*model.Setting)
			settingNormal.SymbolRelated = ``
			util.Notice(`add back normal %s %s`, dyComSetting.Market, info.Name)
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
			normalSetting = &model.Setting{Valid: true, Function: model.FunctionTurtleNormal, Market: dyComSetting.Market,
				Symbol: symbol.(string), OpenShortMargin: dyComSetting.OpenShortMargin, AmountLimit: dyComSetting.AmountLimit,
				Far: dyComSetting.Far, Near: dyComSetting.Near}
			model.AppDB.Save(normalSetting)
		} else {
			normalSetting = normalValue.(*model.Setting)
		}
		_, _, coinValue, _ := model.GetFromStandard(dyComSetting.Market, symbol.(string))
		if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
			if setting.(*model.Setting).Chance == 0 && normalSetting.Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
			util.Info(`remove setting%s`, setting.(*model.Setting).Symbol)
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
			combineSetting = &model.Setting{Valid: true, Function: model.FunctionCombineTurtle, Market: dyComSetting.Market,
				Symbol: symbol.(string), OpenShortMargin: dyComSetting.OpenShortMargin, AmountLimit: dyComSetting.AmountLimit,
				Far: dyComSetting.Far, Near: dyComSetting.Near}
			model.AppDB.Save(combineSetting)
		} else {
			combineSetting = combineValue.(*model.Setting)
		}
		_, _, coinValue, _ := model.GetFromStandard(dyComSetting.Market, symbol.(string))
		if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
			if setting.(*model.Setting).Chance == 0 && combineSetting.Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
		}
		util.Info(`remove setting%s`, setting.(*model.Setting).Symbol)
		return true
	})
}

func handleTurtleSettings(dySetting *model.Setting, function string, topMarketInfos map[string]*model.MarketInfo) {
	settingMap := &sync.Map{}
	value, ok := util.LoadSyncMap(symbolSettings, function, dySetting.Market)
	if ok && value != nil {
		settingMap = value.(*sync.Map)
	}
	for _, info := range topMarketInfos {
		value, _ = settingMap.Load(info.Name)
		settingTurtle := &model.Setting{Valid: true, Function: function, Market: dySetting.Market, Symbol: info.Name,
			OpenShortMargin: dySetting.OpenShortMargin, AmountLimit: dySetting.AmountLimit, Far: dySetting.Far, Near: dySetting.Near}
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
		_, _, coinValue, _ := model.GetFromStandard(dySetting.Market, symbol.(string))
		if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
			if setting.(*model.Setting).Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
			util.Info(`remove setting%s`, setting.(*model.Setting).Symbol)
		}
		return true
	})
}

func getSortedInfos(market string, num int) (marketInfoArray model.MarketInfoArray, topInfos map[string]*model.MarketInfo) {
	marketInfoArray = model.MarketInfoArray{}
	topInfos = make(map[string]*model.MarketInfo)
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
	for i := 0; i < num && i < len(marketInfoArray); i++ {
		topInfos[marketInfoArray[i].Name] = marketInfoArray[i]
	}
	return marketInfoArray, topInfos
}

func getDynamicMarketInfos(function, market string, accounts []*model.Account) (topMarketInfos map[string]*model.MarketInfo) {
	topMarketInfos = make(map[string]*model.MarketInfo)
	turtleDataArray := TurtleDataArray{}
	marketInfoArray, _ := getSortedInfos(market, topMarketInfoLen)
	for i := 0; i < marketInfoArray.Len() && len(topMarketInfos) < topMarketInfoLen; i++ {
		_, marketType, coinValue, _ := model.GetFromStandard(market, marketInfoArray[i].Name)
		if strings.EqualFold(marketType, model.MarketTypePerp) && !model.CommonCoins[strings.ToLower(coinValue)] {
			setting := GetSetting(function, market, marketInfoArray[i].Name)
			turtleData, _ := GetTurtleData(accounts[0].Key, accounts[0].Secret, setting, false)
			if turtleData != nil {
				topMarketInfos[marketInfoArray[i].Name] = marketInfoArray[i]
				turtleDataArray = append(turtleDataArray, turtleData)
				util.Notice(fmt.Sprintf(`get top turtle done %s %s`, market, marketInfoArray[i].Name))
			} else {
				util.Notice(fmt.Sprintf(`get top turtle data fail %s %s`, market, marketInfoArray[i].Name))
			}
		}
	}
	sort.Sort(turtleDataArray)
	for i := 0; i < turtleDataArray.Len()-topTurtleDataLen; i++ {
		delete(topMarketInfos, turtleDataArray[i].Symbol)
		util.Notice(fmt.Sprintf(`remove not topped %s NVolume %f left %d`,
			turtleDataArray[i].Symbol, turtleDataArray[i].NVolume, len(topMarketInfos)))
	}
	return topMarketInfos
}

func handleMarketDynamic(market string) (handled bool) {
	valueDynamic, haveDynamic := util.LoadSyncMap(symbolSettings, model.FunctionDynamicTurtle, market)
	valueCombine, haveCombine := util.LoadSyncMap(symbolSettings, model.FunctionDynamicCombine, market)
	accounts := model.AppConfig.GetAccounts(market)
	if (!haveDynamic && !haveCombine) || accounts == nil || len(accounts) == 0 {
		return false
	}
	DynamicHandleTime.Store(market, time.Now())
	var function = ``
	if haveCombine {
		function = model.FunctionCombineTurtle
	} else if haveDynamic {
		function = model.FunctionTurtle
	}
	topMarketInfos := getDynamicMarketInfos(function, market, accounts)
	var setting *model.Setting
	getOneSetting := func(key, value any) bool {
		if value != nil {
			valueSetting := value.(*model.Setting)
			if valueSetting.Market != `` && valueSetting.Near > 0 && valueSetting.Far > 0 &&
				valueSetting.OpenShortMargin > 0 && valueSetting.AmountLimit > 0 {
				setting = valueSetting
				return false
			}
		}
		return true
	}
	if haveCombine {
		valueCombine.(*sync.Map).Range(getOneSetting)
		if setting != nil {
			handleCombineSettings(setting, topMarketInfos)
		}
	} else if haveDynamic {
		valueDynamic.(*sync.Map).Range(getOneSetting)
		if setting != nil {
			handleTurtleSettings(setting, function, topMarketInfos)
		}
	}
	util.Notice(fmt.Sprintf(`handleMarketDynamic %s %v`, market, true))
	return true
}

func InitApp(refreshDynamic bool) bool {
	if settingLoading {
		return false
	}
	PrepareSettings()
	handled := false
	for _, market := range appMarkets {
		InitMarketInfos(market)
		if refreshDynamic {
			if handleMarketDynamic(market) {
				handled = true
			}
		}
	}
	if handled {
		PrepareSettings()
	}
	for _, market := range appMarkets {
		setRequireReset(market)
	}
	util.Notice(`finish load settings`)
	settingLoading = false
	return true
}

func GetMarketSymbols(market string) map[string]bool {
	if appSettings == nil {
		util.Notice(`load setting GetMarketSymbols %s`, market)
		if !InitApp(true) {
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
		if !InitApp(true) {
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
		if !InitApp(true) {
			return nil
		}
	}
	return appMarkets
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

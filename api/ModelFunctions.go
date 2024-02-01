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
	value, ok := util.LoadSyncMap(handlers, market, symbol)
	if ok && value != nil {
		return value.(*sync.Map)
	}
	return nil
}

var lockRefreshSetting sync.Mutex

func PrepareSettings() {
	lockRefreshSetting.Lock()
	defer lockRefreshSetting.Unlock()
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
		util.StoreSyncMap(localHandlers, functions, setting.Market, strings.TrimSpace(setting.Symbol))
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
			util.Notice(fmt.Sprintf(`load setting %s %s %s %s %d %d`,
				setting.Function, setting.Market, setting.Symbol, setting.SymbolRelated, setting.Far, setting.Near))
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

func handleCombineSettings(mumSetting *model.Setting, topMarketInfos map[string]*model.MarketInfo) {
	combineMap := &sync.Map{}
	value, ok := util.LoadSyncMap(symbolSettings, model.FunctionCombineTurtle, mumSetting.Market)
	if ok && value != nil {
		combineMap = value.(*sync.Map)
	}
	normalMap := &sync.Map{}
	value, ok = util.LoadSyncMap(symbolSettings, model.FunctionTurtleNormal, mumSetting.Market)
	if ok && value != nil {
		normalMap = value.(*sync.Map)
	}
	for _, info := range topMarketInfos {
		valueCombine, _ := combineMap.Load(info.Name)
		valueNormal, _ := normalMap.Load(info.Name)
		settingCombine := &model.Setting{Valid: true, Function: model.FunctionCombineTurtle, Market: mumSetting.Market,
			Symbol: info.Name, ChanceLimit: mumSetting.ChanceLimitCombine, AmountRate: mumSetting.AmountRateCombine,
			AmountLimit: mumSetting.AmountLimit, Far: mumSetting.FarCombine, Near: mumSetting.NearCombine,
			Seconds: mumSetting.SecondsCombine, MarketRelated: mumSetting.MarketRelated}
		settingNormal := &model.Setting{Valid: true, Function: model.FunctionTurtleNormal, Market: mumSetting.Market,
			Symbol: info.Name, ChanceLimit: mumSetting.ChanceLimit, AmountRate: mumSetting.AmountRate,
			AmountLimit: mumSetting.AmountLimit, Far: mumSetting.Far, Near: mumSetting.Near, Seconds: mumSetting.Seconds,
			MarketRelated: mumSetting.MarketRelated}
		if valueCombine == nil {
			util.Notice(`add combine %s %v`, mumSetting.Market, info.Name)
			accounts := model.AppConfig.GetAccounts(mumSetting.Market)
			for _, account := range accounts {
				if account != nil {
					SetSymbolLeverage(account, settingCombine.Market, settingCombine.Symbol)
				}
			}
		} else {
			settingCombine = valueCombine.(*model.Setting)
			settingCombine.SymbolRelated = ``
			util.Notice(`add back combine %s %s of tops %d`, mumSetting.Market, info.Name, len(topMarketInfos))
		}
		if valueNormal != nil {
			settingNormal = valueNormal.(*model.Setting)
			settingNormal.SymbolRelated = ``
			util.Notice(`add back normal %s %s of tops %d`, mumSetting.Market, info.Name, len(topMarketInfos))
		}
		combineMap.Store(info.Name, settingCombine)
		normalMap.Store(info.Name, settingNormal)
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
			normalSetting = &model.Setting{Valid: true, Function: model.FunctionTurtleNormal, Market: mumSetting.Market,
				Symbol: symbol.(string), ChanceLimit: mumSetting.ChanceLimit, AmountRate: mumSetting.AmountRate,
				AmountLimit: mumSetting.AmountLimit, Far: mumSetting.Far, Near: mumSetting.Near, Seconds: mumSetting.Seconds}
			model.AppDB.Save(normalSetting)
		} else {
			normalSetting = normalValue.(*model.Setting)
		}
		_, _, coinValue, _ := model.GetFromStandard(mumSetting.Market, symbol.(string))
		if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
			if setting.(*model.Setting).Chance == 0 && normalSetting.Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
			util.Notice(`remove setting combine %s %s`, setting.(*model.Setting).Market, setting.(*model.Setting).Symbol)
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
			combineSetting = &model.Setting{Valid: true, Function: model.FunctionCombineTurtle, Market: mumSetting.Market,
				Symbol: symbol.(string), ChanceLimit: mumSetting.ChanceLimitCombine, AmountRate: mumSetting.AmountRateCombine,
				AmountLimit: mumSetting.AmountLimit, Far: mumSetting.FarCombine, Near: mumSetting.NearCombine, Seconds: mumSetting.SecondsCombine}
			model.AppDB.Save(combineSetting)
		} else {
			combineSetting = combineValue.(*model.Setting)
		}
		_, _, coinValue, _ := model.GetFromStandard(mumSetting.Market, symbol.(string))
		if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
			if setting.(*model.Setting).Chance == 0 && combineSetting.Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
			util.Notice(`remove setting turtle %s %s`, setting.(*model.Setting).Market, setting.(*model.Setting).Symbol)
		}
		return true
	})
}

func handleSingleSettings(mumSetting *model.Setting, topMarketInfos map[string]*model.MarketInfo, function string) {
	settingMap := &sync.Map{}
	value, ok := util.LoadSyncMap(symbolSettings, function, mumSetting.Market)
	if ok && value != nil {
		settingMap = value.(*sync.Map)
	}
	for _, info := range topMarketInfos {
		value, _ = settingMap.Load(info.Name)
		settingNew := &model.Setting{Valid: true, Function: function, Market: mumSetting.Market, Symbol: info.Name,
			ChanceLimit: mumSetting.ChanceLimit, AmountRate: mumSetting.AmountRate, AmountRateCombine: mumSetting.AmountRateCombine,
			AmountLimit: mumSetting.AmountLimit, Far: mumSetting.Far, Near: mumSetting.Near, Seconds: mumSetting.Seconds,
			FarCombine: mumSetting.FarCombine, NearCombine: mumSetting.NearCombine, SecondsCombine: mumSetting.SecondsCombine}
		if value == nil {
			util.Notice(`add settingNew %v`, settingNew.Symbol)
			accounts := model.AppConfig.GetAccounts(mumSetting.Market)
			for _, account := range accounts {
				if account != nil {
					SetSymbolLeverage(account, settingNew.Market, settingNew.Symbol)
				}
			}
		} else {
			settingNew = value.(*model.Setting)
			settingNew.SymbolRelated = ``
			util.Notice(`add settingNew back %s`, info.Name)
		}
		settingMap.Store(info.Name, settingNew)
		model.AppDB.Save(settingNew)
	}
	settingMap.Range(func(symbol, setting interface{}) bool {
		if setting == nil {
			return true
		}
		_, _, coinValue, _ := model.GetFromStandard(mumSetting.Market, symbol.(string))
		if topMarketInfos[symbol.(string)] == nil && !model.CommonCoins[strings.ToLower(coinValue)] {
			//if setting.(*model.Setting).Chance == 0 {
			//}
			setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
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
	sort.Sort(sort.Reverse(marketInfoArray))
	for i := 0; i < num && i < len(marketInfoArray); i++ {
		topInfos[marketInfoArray[i].Name] = marketInfoArray[i]
		util.Notice(fmt.Sprintf(`get top market info to array %s %s trade amount %fu`,
			market, marketInfoArray[i].Name, marketInfoArray[i].TradeAmount))
	}
	return marketInfoArray, topInfos
}

func getDynamicMarketInfos(mumSetting *model.Setting, accounts []*model.Account, function string, lenInfo, lenData int) (
	topMarketInfos map[string]*model.MarketInfo) {
	topMarketInfos = make(map[string]*model.MarketInfo)
	turtleDataArray := model.TurtleDataArray{}
	marketInfoArray, _ := getSortedInfos(mumSetting.Market, lenInfo)
	for i := 0; i < marketInfoArray.Len() && len(topMarketInfos) < lenInfo; i++ {
		_, marketType, coinValue, _ := model.GetFromStandard(mumSetting.Market, marketInfoArray[i].Name)
		if strings.EqualFold(marketType, model.MarketTypePerp) && !model.CommonCoins[strings.ToLower(coinValue)] &&
			!model.NoTurtleCoins[strings.ToLower(coinValue)] {
			tried := false
			var turtleData *model.TurtleData
			for time.Now().Minute() <= 6 || !tried {
				tried = true
				dataValid := false
				far := mumSetting.Far
				near := mumSetting.Near
				seconds := mumSetting.Seconds
				//checkSetting := &model.Setting{Function: function, Market: mumSetting.Market, Symbol: marketInfoArray[i].Name,
				//	Far: mumSetting.Far, Near: mumSetting.Near, Seconds: mumSetting.Seconds, AmountRate: mumSetting.AmountRate}
				if function == model.FunctionDynamicCombine && far*seconds < mumSetting.FarCombine*mumSetting.SecondsCombine {
					far = mumSetting.FarCombine
					near = mumSetting.NearCombine
					seconds = mumSetting.SecondsCombine
				}
				if near > 0 && far >= near && seconds > 0 {
					turtleData, dataValid = GetTurtleData(accounts[0], function, mumSetting.Market, marketInfoArray[i].Name,
						mumSetting.Far, mumSetting.Near, mumSetting.Seconds, mumSetting.ChanceLimit, mumSetting.AmountRate, false, false)
					if turtleData != nil {
						topMarketInfos[marketInfoArray[i].Name] = marketInfoArray[i]
						turtleDataArray = append(turtleDataArray, turtleData)
						util.Notice(fmt.Sprintf(`get top turtle done %d of %d %s %s %d n:%f nVolume:%f`,
							i, lenInfo, mumSetting.Market, marketInfoArray[i].Name, mumSetting.Seconds, turtleData.N, turtleData.NVolume))
						break
					} else if dataValid {
						util.Notice(fmt.Sprintf(`get top turtle data fail for new coin reason`))
						break
					} else {
						util.Notice(fmt.Sprintf(`get top turtle data fail %s %s`, mumSetting.Market, marketInfoArray[i].Name))
						time.Sleep(time.Second)
					}
				}
			}
		}
	}
	if function == model.FunctionDynamicBoost {
		marketInfo24hr := make(map[string]*model.MarketInfo)
		num := 0
		for i := 0; i < len(marketInfoArray) && num < lenData; i++ {
			if marketInfoArray[i] != nil && topMarketInfos[marketInfoArray[i].Name] != nil {
				marketInfo24hr[marketInfoArray[i].Name] = marketInfoArray[i]
				num++
				util.Notice(fmt.Sprintf(`keep topped %s %s %s index %d num %d`,
					function, mumSetting.Market, marketInfoArray[i].Name, i, num))
			} else {
				util.Notice(fmt.Sprintf(`remove nil data %s %s %s index %d num %d`,
					function, mumSetting.Market, marketInfoArray[i].Name, i, num))
			}
		}
		return marketInfo24hr
	} else {
		sort.Sort(turtleDataArray)
		for i := 0; i < turtleDataArray.Len(); i++ {
			if i < turtleDataArray.Len()-lenData {
				delete(topMarketInfos, turtleDataArray[i].Symbol)
				util.Notice(fmt.Sprintf(`remove not topped last %s %s %d of %d NVolume %f left %d`,
					mumSetting.Market, turtleDataArray[i].Symbol, i, lenData, turtleDataArray[i].NVolume, len(topMarketInfos)))
			} else {
				util.Notice(fmt.Sprintf(`keep topped %s %s last %d of %d NVolume %f left %d`,
					mumSetting.Market, turtleDataArray[i].Symbol, i, lenData, turtleDataArray[i].NVolume, len(topMarketInfos)))
			}
		}
		return topMarketInfos
	}
}

func handleMarketDynamic(market string) (handled bool) {
	settingDynamicTurtle := GetSetting(model.FunctionDynamicTurtle, market, ``)
	settingDynamicCombine := GetSetting(model.FunctionDynamicCombine, market, ``)
	settingDynamicBoost := GetSetting(model.FunctionDynamicBoost, market, ``)
	accounts := model.AppConfig.GetAccounts(market)
	if (settingDynamicTurtle == nil && settingDynamicCombine == nil && settingDynamicBoost == nil) ||
		accounts == nil || len(accounts) == 0 {
		return false
	}
	InitMarketInfos(market)
	if settingDynamicCombine != nil {
		topMarketInfos := getDynamicMarketInfos(settingDynamicCombine, accounts, settingDynamicCombine.Function, 45, 30)
		handleCombineSettings(settingDynamicCombine, topMarketInfos)
	} else if settingDynamicTurtle != nil {
		topMarketInfos := getDynamicMarketInfos(settingDynamicTurtle, accounts, settingDynamicTurtle.Function, 45, 30)
		handleSingleSettings(settingDynamicTurtle, topMarketInfos, model.FunctionTurtle)
	}
	DynamicHandleTime.Store(market, time.Now())
	util.Notice(fmt.Sprintf(`handle Dynamic settings %s`, market))
	return true
}

func InitApp(refreshDynamic bool) bool {
	if settingLoading {
		return false
	}
	PrepareSettings()
	handled := false
	for _, market := range appMarkets {
		if refreshDynamic {
			if handleMarketDynamic(market) {
				handled = true
			}
		}
		if !handled {
			InitMarketInfos(market)
		}
	}
	if handled {
		PrepareSettings()
	}
	for _, market := range appMarkets {
		SetRequireReset(market)
		accounts := model.AppConfig.GetAccounts(market)
		for _, account := range accounts {
			go CancelAll(account.Key, account.Secret, market)
		}
	}
	util.Notice(`finish load settings`)
	settingLoading = false
	return true
}

func GetMarketSymbols(market string) map[string]bool {
	if appSettings == nil {
		util.Notice(`load setting GetMarketSymbols %s`, market)
		return nil
	}
	symbols := make(map[string]bool)
	for _, value := range appSettings {
		if value.Market == market {
			symbols[value.Symbol] = true
		}
	}
	return symbols
}

//func GetSettingsFromCoin(coin string) (settings []*model.Setting) {
//	if appSettings == nil || coinSettings == nil {
//		util.Notice(`load setting GetSettingsFromCoin fail`)
//		return nil
//	}
//	settings = make([]*model.Setting, 0)
//	coinSettings.Range(func(function, value any) bool {
//		if value == nil {
//			return true
//		}
//		array, _ := value.(*sync.Map).Load(coin)
//		if array != nil {
//			for _, setting := range array.([]*model.Setting) {
//				settings = append(settings, setting)
//			}
//		}
//		return true
//	})
//	return settings
//}

func GetCoinSettings(function string) *sync.Map {
	if appSettings == nil || coinSettings == nil {
		util.Notice(`load setting GetCoinSettings %s`, function)
		return nil
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
		return nil
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

package api

import (
	"fmt"
	//"hello/deprecated"
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
var CarryCoins = &sync.Map{}        // accountIndex*coin *CarryCoin
var crossLen int
var settingLoading bool
var processLock sync.Mutex
var processing = &sync.Map{}

func CheckSetProcessing(function, market, symbol string, requestValue bool) (before bool) {
	processLock.Lock()
	defer processLock.Unlock()
	v, ok := util.LoadSyncMap(processing, function, market, symbol)
	if ok && v != nil {
		before = v.(bool)
	}
	if requestValue == false || before == false {
		util.StoreSyncMap(processing, requestValue, function, market, symbol)
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
	symbolSettings.Clear()
	handlers.Clear()
	coinSettings.Clear()
	CarryCoins.Clear()
	util.Log(util.LogLevelInfo, fmt.Sprintf("Settings loaded %#v", coinSettings))
	var appSettings []model.Setting
	var appCarryCoins []model.CarryCoin
	marketMap := make(map[string]bool)
	model.AppDB.Where(`valid = ?`, true).Find(&appSettings)
	model.AppDB.Find(&appCarryCoins)
	for _, carryCoin := range appCarryCoins {
		util.StoreSyncMap(CarryCoins, &carryCoin, carryCoin.Coin, `0`)
	}
	util.Log(util.LogLevelInfo, fmt.Sprintf(`start to load settings carry coins %d %d`, len(appSettings), len(appCarryCoins)))
	for i := 0; i < len(appSettings); i++ {
		setting := &appSettings[i]
		value, ok := util.LoadSyncMap(symbolSettings, setting.Function, setting.Market)
		if ok && value != nil {
			oldSetting, oldOk := value.(*sync.Map).Load(setting.Symbol)
			if oldOk && oldSetting != nil {
				setting = oldSetting.(*model.Setting)
			}
		}
		//util.Log(util.LogLevelInfo, fmt.Sprintf(`load setting %s %s %s %#v %d`,
		//	setting.Market, setting.Symbol, setting.Function, setting.Valid, setting.Chance))
		marketMap[setting.Market] = true
		value, ok = util.LoadSyncMap(handlers, setting.Market, setting.Symbol)
		var functions *sync.Map
		if ok {
			functions = value.(*sync.Map)
		}
		if functions == nil {
			functions = &sync.Map{}
		}
		if !model.IgnoreFunctions[setting.Function] {
			functions.Store(setting.Function, model.TickHandlers[setting.Function])
		}
		util.StoreSyncMap(handlers, functions, setting.Market, strings.TrimSpace(setting.Symbol))
		var settings *sync.Map
		value, ok = coinSettings.Load(setting.Function)
		if ok && value != nil {
			settings = value.(*sync.Map)
		} else {
			settings = &sync.Map{}
			coinSettings.Store(setting.Function, settings)
		}
		settingArray, _ := settings.Load(setting.Coin)
		if settingArray == nil {
			settingArray = make([]*model.Setting, 0)
		}
		settingArray = append(settingArray.([]*model.Setting), setting)
		if setting.Coin == `FIRE` || setting.MarketRelated != `` || !setting.Valid {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`add setting array under monitor %s %s %s %v %d %s`,
				setting.Coin, setting.Market, setting.Symbol, setting.Valid, len(settingArray.([]*model.Setting)), setting.MarketRelated))
		}
		settings.Store(setting.Coin, settingArray)
		var functionMarketSettings *sync.Map
		value, ok = util.LoadSyncMap(symbolSettings, setting.Function, setting.Market)
		if ok {
			functionMarketSettings = value.(*sync.Map)
		}
		if functionMarketSettings == nil {
			functionMarketSettings = &sync.Map{}
		}
		functionMarketSettings.Store(setting.Symbol, setting)
		util.StoreSyncMap(symbolSettings, functionMarketSettings, setting.Function, setting.Market)
	}
	localAppMarkets := make([]string, len(marketMap))
	i := 0
	for key := range marketMap {
		localAppMarkets[i] = key
		i++
	}
	model.AppEnvironment.Settings = appSettings
	model.AppEnvironment.Markets = localAppMarkets
	util.Log(util.LogLevelInfo, fmt.Sprintf(`finish loading settings from markets %#v %+v`, model.AppEnvironment.Markets, coinSettings))
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
		valueCombine, _ := combineMap.Load(info.Symbol)
		valueNormal, _ := normalMap.Load(info.Symbol)
		settingCombine := &model.Setting{Valid: true, Function: model.FunctionCombineTurtle, Market: mumSetting.Market,
			Symbol: info.Symbol, ChanceLimit: mumSetting.ChanceLimitCombine, AmountRate: mumSetting.AmountRateCombine,
			AmountLimit: mumSetting.AmountLimit, CloseShortMargin: mumSetting.CloseShortMargin, Far: mumSetting.FarCombine,
			Near: mumSetting.NearCombine, Seconds: mumSetting.SecondsCombine, MarketRelated: mumSetting.MarketRelated,
			WSType: model.WSTypeTicker}
		settingNormal := &model.Setting{Valid: true, Function: model.FunctionTurtleNormal, Market: mumSetting.Market,
			Symbol: info.Symbol, ChanceLimit: mumSetting.ChanceLimit, AmountRate: mumSetting.AmountRate,
			AmountLimit: mumSetting.AmountLimit, CloseShortMargin: mumSetting.CloseShortMargin, Far: mumSetting.Far,
			Near: mumSetting.Near, Seconds: mumSetting.Seconds, MarketRelated: mumSetting.MarketRelated, WSType: model.WSTypeTicker}
		if valueCombine == nil {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`add combine %s %#v`, mumSetting.Market, info.Symbol))
			accounts := model.AppConfig.GetAccounts(mumSetting.Market)
			for _, account := range accounts {
				if account != nil {
					SetSymbolLeverage(account, settingCombine.Market, settingCombine.Symbol)
				}
			}
		} else {
			settingCombine = valueCombine.(*model.Setting)
			settingCombine.SymbolRelated = ``
			if settingCombine.Chance == 0 {
				settingCombine.GridAmount = 0
			}
			util.Log(util.LogLevelInfo, fmt.Sprintf(`add back combine %s %s of tops %d`, mumSetting.Market, info.Symbol, len(topMarketInfos)))
		}
		if valueNormal != nil {
			settingNormal = valueNormal.(*model.Setting)
			settingNormal.SymbolRelated = ``
			if settingNormal.Chance == 0 {
				settingNormal.GridAmount = 0
			}
			util.Log(util.LogLevelInfo, fmt.Sprintf(`add back normal %s %s of tops %d`, mumSetting.Market, info.Symbol, len(topMarketInfos)))
		}
		combineMap.Store(info.Symbol, settingCombine)
		normalMap.Store(info.Symbol, settingNormal)
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
				WSType: model.WSTypeTicker, AmountLimit: mumSetting.AmountLimit, CloseShortMargin: mumSetting.CloseShortMargin,
				Far: mumSetting.Far, Near: mumSetting.Near, Seconds: mumSetting.Seconds}
			model.AppDB.Save(normalSetting)
		} else {
			normalSetting = normalValue.(*model.Setting)
		}
		if topMarketInfos[symbol.(string)] == nil && !model.CommonSymbols[symbol.(string)] {
			if setting.(*model.Setting).Chance == 0 && normalSetting.Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
			util.Log(util.LogLevelInfo, fmt.Sprintf(
				`remove setting combine %s %s`, setting.(*model.Setting).Market, setting.(*model.Setting).Symbol))
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
				WSType: model.WSTypeTicker, Symbol: symbol.(string), ChanceLimit: mumSetting.ChanceLimitCombine,
				AmountRate: mumSetting.AmountRateCombine, AmountLimit: mumSetting.AmountLimit, CloseShortMargin: mumSetting.CloseShortMargin,
				Far: mumSetting.FarCombine, Near: mumSetting.NearCombine, Seconds: mumSetting.SecondsCombine}
			model.AppDB.Save(combineSetting)
		} else {
			combineSetting = combineValue.(*model.Setting)
		}
		if topMarketInfos[symbol.(string)] == nil && !model.CommonSymbols[symbol.(string)] {
			if setting.(*model.Setting).Chance == 0 && combineSetting.Chance == 0 {
				setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			}
			model.AppDB.Save(setting)
			util.Log(util.LogLevelInfo, fmt.Sprintf(
				`remove setting turtle %s %s`, setting.(*model.Setting).Market, setting.(*model.Setting).Symbol))
		}
		return true
	})
}

func handleMoveMarkets(setting *model.Setting) {
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	_, positions, _, _, _ := GetPositions(account, setting.Market)
	_, balances, _, _ := GetBalances(account, setting.Market)
	for _, position := range positions {
		settingNew := &model.Setting{Valid: true, Function: model.FunctionMove, Market: setting.Market, Symbol: position.Currency}
		model.AppDB.Save(settingNew)
	}
	for _, balance := range balances {
		settingNew := &model.Setting{Valid: true, Function: model.FunctionMove, Market: setting.Market, Symbol: balance.Coin + model.UniStandardTail[model.MarketTypeSpot]}
		model.AppDB.Save(settingNew)
	}
}

func handleSingleSettings(mumSetting *model.Setting, topMarketInfos map[string]*model.MarketInfo, function string) {
	settingMap := &sync.Map{}
	value, ok := util.LoadSyncMap(symbolSettings, function, mumSetting.Market)
	if ok && value != nil {
		settingMap = value.(*sync.Map)
	}
	for _, info := range topMarketInfos {
		value, _ = settingMap.Load(info.Symbol)
		settingNew := &model.Setting{Valid: true, Function: function, Market: mumSetting.Market, Symbol: info.Symbol, WSType: model.WSTypeTicker,
			ChanceLimit: mumSetting.ChanceLimit, AmountRate: mumSetting.AmountRate, AmountRateCombine: mumSetting.AmountRateCombine,
			AmountLimit: mumSetting.AmountLimit, Far: mumSetting.Far, Near: mumSetting.Near, Seconds: mumSetting.Seconds,
			FarCombine: mumSetting.FarCombine, NearCombine: mumSetting.NearCombine, SecondsCombine: mumSetting.SecondsCombine}
		if value == nil {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`add settingNew %#v`, settingNew.Symbol))
			accounts := model.AppConfig.GetAccounts(mumSetting.Market)
			for _, account := range accounts {
				if account != nil {
					SetSymbolLeverage(account, settingNew.Market, settingNew.Symbol)
				}
			}
		} else {
			settingNew = value.(*model.Setting)
			settingNew.SymbolRelated = ``
			util.Log(util.LogLevelInfo, fmt.Sprintf(`add settingNew back %s`, info.Symbol))
		}
		settingMap.Store(info.Symbol, settingNew)
		model.AppDB.Save(settingNew)
	}
	settingMap.Range(func(symbol, setting interface{}) bool {
		if setting == nil {
			return true
		}
		if topMarketInfos[symbol.(string)] == nil && !model.CommonSymbols[symbol.(string)] {
			//if setting.(*model.Setting).Chance == 0 {
			//}
			setting.(*model.Setting).SymbolRelated = model.SettingTurtleRemoved
			model.AppDB.Save(setting)
			util.Log(util.LogLevelInfo, fmt.Sprintf(`remove setting%s`, setting.(*model.Setting).Symbol))
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
		topInfos[marketInfoArray[i].Symbol] = marketInfoArray[i]
		util.Log(util.LogLevelInfo, fmt.Sprintf(`get top market info to array %s %s trade amount %fu`,
			market, marketInfoArray[i].Symbol, marketInfoArray[i].TradeAmount))
	}
	return marketInfoArray, topInfos
}

func getDynamicMarketInfos(mumSetting *model.Setting, accounts []*model.Account, function string, lenInfo, lenData int) (
	topMarketInfos map[string]*model.MarketInfo) {
	topMarketInfos = make(map[string]*model.MarketInfo)
	turtleDataArray := model.TurtleDataArray{}
	marketInfoArray, _ := getSortedInfos(mumSetting.Market, lenInfo)
	for i := 0; i < marketInfoArray.Len() && len(topMarketInfos) < lenInfo; i++ {
		_, marketType, coinValue, _ := model.GetFromStandard(mumSetting.Market, marketInfoArray[i].Symbol)
		if strings.EqualFold(marketType, model.MarketTypePerp) && !model.CommonSymbols[marketInfoArray[i].Symbol] &&
			!model.NoTurtleCoins[strings.ToLower(coinValue)] {
			tried := false
			var turtleData *model.TurtleData
			for time.Now().Minute() <= 6 || !tried {
				tried = true
				dataValid := false
				far := mumSetting.Far
				near := mumSetting.Near
				seconds := mumSetting.Seconds
				//checkSetting := &model.Setting{Function: function, Market: mumSetting.Market, Symbol: marketInfoArray[i].Symbol,
				//	Far: mumSetting.Far, Near: mumSetting.Near, Seconds: mumSetting.Seconds, AmountRate: mumSetting.AmountRate}
				if function == model.FunctionDynamicCombine && far*seconds < mumSetting.FarCombine*mumSetting.SecondsCombine {
					far = mumSetting.FarCombine
					near = mumSetting.NearCombine
					seconds = mumSetting.SecondsCombine
				}
				if near > 0 && far >= near && seconds > 0 {
					turtleData, dataValid = GetRankTurtleData(accounts[0], marketInfoArray[i].Symbol, mumSetting)
					if turtleData != nil {
						topMarketInfos[marketInfoArray[i].Symbol] = marketInfoArray[i]
						turtleDataArray = append(turtleDataArray, turtleData)
						util.Log(util.LogLevelInfo, fmt.Sprintf(
							`get top turtle done %d of %d %s %s %d n:%f nVolume:%f`,
							i, lenInfo, mumSetting.Market, marketInfoArray[i].Symbol, mumSetting.Seconds, turtleData.N, turtleData.NVolume))
						break
					} else if dataValid {
						util.Log(util.LogLevelInfo, fmt.Sprintf(`get top turtle data fail for new coin reason`))
						break
					} else {
						util.Log(util.LogLevelInfo, fmt.Sprintf(`get top turtle data fail %s %s`, mumSetting.Market, marketInfoArray[i].Symbol))
						time.Sleep(time.Second)
					}
				}
			}
		}
	}
	sort.Sort(turtleDataArray)
	for i := 0; i < turtleDataArray.Len(); i++ {
		if i < turtleDataArray.Len()-lenData {
			delete(topMarketInfos, turtleDataArray[i].Symbol)
			util.Log(util.LogLevelInfo, fmt.Sprintf(`remove not topped last %s %s %d of %d NVolume %f left %d`,
				mumSetting.Market, turtleDataArray[i].Symbol, i, lenData, turtleDataArray[i].NVolume, len(topMarketInfos)))
		} else {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`keep topped %s %s last %d of %d NVolume %f left %d`,
				mumSetting.Market, turtleDataArray[i].Symbol, i, lenData, turtleDataArray[i].NVolume, len(topMarketInfos)))
		}
	}
	return topMarketInfos
}

func handleMarketDynamic(market string) (handled bool) {
	settingDynamicTurtle := GetSetting(model.FunctionDynamicTurtle, market, ``)
	settingDynamicCombine := GetSetting(model.FunctionDynamicCombine, market, ``)
	settingMoveMarket := GetSetting(model.FunctionMoveMarket, market, ``)
	accounts := model.AppConfig.GetAccounts(market)
	if (settingDynamicTurtle == nil && settingDynamicCombine == nil) ||
		accounts == nil || len(accounts) == 0 {
		return false
	}
	InitMarketInfos(market)
	topLen := 70
	if settingDynamicCombine != nil {
		topMarketInfos := getDynamicMarketInfos(settingDynamicCombine, accounts, settingDynamicCombine.Function, topLen, int(settingDynamicCombine.OpenShortMargin))
		handleCombineSettings(settingDynamicCombine, topMarketInfos)
	}
	if settingDynamicTurtle != nil {
		topMarketInfos := getDynamicMarketInfos(settingDynamicTurtle, accounts, settingDynamicTurtle.Function, topLen, int(settingDynamicTurtle.OpenShortMargin))
		handleSingleSettings(settingDynamicTurtle, topMarketInfos, model.FunctionTurtle)
	}
	if settingMoveMarket != nil {
		handleMoveMarkets(settingMoveMarket)
	}
	DynamicHandleTime.Store(market, time.Now())
	util.Log(util.LogLevelInfo, fmt.Sprintf(`handle Dynamic settings %s`, market))
	return true
}

func InitApp(refreshDynamic bool) bool {
	util.Log(util.LogLevelInfo, `begin to init app`)
	if settingLoading {
		return false
	}
	PrepareSettings()
	handled := false
	for _, market := range model.AppEnvironment.Markets {
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
	for _, market := range model.AppEnvironment.Markets {
		accounts := model.AppConfig.GetAccounts(market)
		for _, account := range accounts {
			go CancelAll(account, market)
			go initMarketMode(account, market)
		}
		MaintainConns(market)
	}
	util.Log(util.LogLevelInfo, `finish load settings`)
	settingLoading = false
	return true
}

func initMarketMode(account *model.Account, market string) {
	switch market {
	case model.OKEX:
		accountMode := getAccountConfigOKEX(account)
		util.Log(util.LogLevelInfo, `okex config and set: `+accountMode)
		if accountMode != `net_mode` {
			setAccountModeOKEX(account)
		}
		setLeverageOkx(account)
	case model.BinancePerp:
		setPosSideBinancePerp(account.Key, account.Secret)
		setLeverageBinancePerp(account.Key, account.Secret)
	case model.Gate:
		setPosSideGate(account.Key, account.Secret)
		setMarginSettingGate(account.Key, account.Secret)
		setLeverageGate(account)
	case model.Bybit:
		setBybitMarginLeverage(account.Key, account.Secret)
		setBybitPerpLeverage(account.Key, account.Secret)
		//case model.BitgetPerp:
		//	deprecated.setBitgetPositionMode(account.Key, account.Secret)
		//	deprecated.setLeverageBitgetPerp(account)
	}
}

func GetMarketSymbols(market string) map[string]bool {
	if model.AppConfig.Env == `test` {
		if market == model.BinancePerp {
			return map[string]bool{`1000BONK_PERP`: true, `1000SHIB_PERP`: true, `AAVE_PERP`: true, `ADA_PERP`: true, `APE_PERP`: true, `APT_PERP`: true, `ARB_PERP`: true, `ATOM_PERP`: true,
				`AVAX_PERP`: true, `BTC_PERP`: true, `CRV_PERP`: true, `DOGE_PERP`: true, `DOT_PERP`: true, `ENA_PERP`: true, `ENS_PERP`: true, `ETHFI_PERP`: true, `ETH_PERP`: true, `FET_PERP`: true,
				`FIL_PERP`: true, `FTM_PERP`: true, `GALA_PERP`: true, `GLM_PERP`: true, `GOAT_PERP`: true, `HBAR_PERP`: true, `INJ_PERP`: true, `JTO_PERP`: true, `KSM_PERP`: true, `MANA_PERP`: true,
				`MOODENG_PERP`: true, `NEAR_PERP`: true, `NEIRO_PERP`: true, `NOT_PERP`: true, `OP_PERP`: true, `ORDI_PERP`: true, `SAND_PERP`: true, `SEI_PERP`: true, `SOL_PERP`: true, `STX_PERP`: true,
				`SUI_PERP`: true, `TAO_PERP`: true, `TIA_PERP`: true, `TON_PERP`: true, `WIF_PERP`: true, `WLD_PERP`: true, `XLM_PERP`: true}
		} else if market == model.OKEX {
			return map[string]bool{`AAVE_PERP`: true, `ADA_PERP`: true, `ALGO_PERP`: true, `APT_PERP`: true, `ARB_PERP`: true, `ATOM_PERP`: true, `AVAX_PERP`: true, `BONK_PERP`: true,
				`BTC_PERP`: true, `CRV_PERP`: true, `DOGE_PERP`: true, `DOGS_PERP`: true, `DOT_PERP`: true, `ENS_PERP`: true, `ETHFI_PERP`: true, `ETH_PERP`: true, `FIL_PERP`: true, `FTM_PERP`: true,
				`GALA_PERP`: true, `GLM_PERP`: true, `GRASS_PERP`: true, `HBAR_PERP`: true, `JTO_PERP`: true, `KSM_PERP`: true, `MANA_PERP`: true, `MOODENG_PERP`: true, `NEAR_PERP`: true,
				`NEIRO_PERP`: true, `NOT_PERP`: true, `OP_PERP`: true, `ORDI_PERP`: true, `PEOPLE_PERP`: true, `PEPE_PERP`: true, `POL_PERP`: true, `PUFFER_PERP`: true, `SAND_PERP`: true,
				`SATS_PERP`: true, `SHIB_PERP`: true, `SOL_PERP`: true, `SUI_PERP`: true, `TIA_PERP`: true, `TON_PERP`: true, `TURBO_PERP`: true, `WIF_PERP`: true, `WLD_PERP`: true, `XLM_PERP`: true, `X_PERP`: true}
		} else if market == model.Gate {
			return map[string]bool{`AAVE_PERP`: true}
		} else if market == model.Bybit {
			return map[string]bool{`VTHO_PERP`: true}
		} else if market == model.BinanceSpot {
			return map[string]bool{`ADA_USDT`: true}
		}
	}
	if model.AppEnvironment.Settings == nil {
		util.Log(util.LogLevelInfo, fmt.Sprintf(`load setting GetMarketSymbols %s`, market))
		return nil
	}
	symbols := make(map[string]bool)
	for _, value := range model.AppEnvironment.Settings {
		marketInfo, getMarketInfo := util.LoadSyncMap(model.MarketInfos, market, value.Symbol)
		if value.Market == market && marketInfo != nil && getMarketInfo {
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

func GetCarryCoin(coin string) (carryCoin *model.CarryCoin) {
	value, ok := util.LoadSyncMap(CarryCoins, coin, `0`)
	if ok && value != nil {
		return value.(*model.CarryCoin)
	}
	return nil
}

func GetCoinSettings(function string) *sync.Map {
	if coinSettings == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`load setting GetCoinSettings %s`, function))
		return nil
	}
	value, ok := coinSettings.Load(function)
	if ok {
		return value.(*sync.Map)
	}
	return nil
}

func GetCrossLen() int {
	if crossLen > 0 {
		return crossLen
	}
	for _, market := range model.AppEnvironment.Markets {
		accounts := model.AppConfig.GetAccounts(market)
		if crossLen == 0 {
			crossLen = len(accounts)
		} else if len(accounts) != crossLen {
			util.Log(util.LogLevelInfo, fmt.Sprintf(`wrong cross config %s accounts:%d`, market, len(accounts)))
			os.Exit(2)
		}
	}
	return crossLen
}

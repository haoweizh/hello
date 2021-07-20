package cross

import (
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"time"
)

// todo 1. postOrderCarry 2. usdRate < 20% revert
const loseRateMax = -0.005
const winRateMin = 0.005

var spotBalance map[string]map[string]float64         // key - market - spot account balance in usd
var swapBalance map[string]map[string]float64         // key - market - swap account balance in usd
var balances map[string]map[string][]*model.Balance   // key - market - balances
var positions map[string]map[string][]*model.Position // key - market - positions

func checkSetCrossing(value bool) (before bool) {
	crossLock.Lock()
	defer crossLock.Unlock()
	if value && crossing {
		return crossing
	} else {
		temp := crossing
		crossing = value
		return temp
	}
}

func createFromPosition(key, secret string, setting *model.Setting) (carryStatus *CarryStatus) {
	if positions[key] == nil {
		positions[key] = make(map[string][]*model.Position)
	}
	if positions[key][setting.Market] == nil {
		success, apiPositions, posBalance := api.GetPositions(key, secret, setting.Market)
		if success {
			positions[key][setting.Market] = apiPositions
			swapBalance[key][setting.Market] = posBalance
		}
	}
	for _, position := range positions[key][setting.Market] {
		if position == nil || position.Currency != setting.Symbol {
			continue
		}
		carryStatus = &CarryStatus{Market: setting.Market, Symbol: setting.Symbol, LimitSell: math.NaN(),
			LimitBuy: math.NaN(), Holding: position.Free}
	}
	if carryStatus == nil {
		carryStatus = &CarryStatus{Market: setting.Market, Symbol: setting.Symbol, LimitSell: math.NaN(),
			LimitBuy: math.NaN(), Holding: 0}
	}
	return carryStatus
}

func createFromBalance(key, secret string, setting *model.Setting) (carryStatus *CarryStatus) {
	if balances[key] == nil {
		balances[key] = make(map[string][]*model.Balance)
	}
	if usdAmount[key] == nil {
		usdAmount[key] = make(map[string]float64)
	}
	if balances[key][setting.Market] == nil {
		success, apiBalances, totalInUsd, collateral := api.GetBalances(key, secret, setting.Market)
		if success && totalInUsd > 0 {
			setCollateral(key, collateral)
			balances[key][setting.Market] = apiBalances
			spotBalance[key][setting.Market] = totalInUsd
		}
	}
	for _, balance := range balances[key][setting.Market] {
		if balance == nil || strings.ToUpper(balance.Coin) != setting.Coin {
			continue
		}
		if (strings.ToUpper(balance.Coin) == `USDT` && setting.Market != model.Ftx) ||
			(strings.ToUpper(balance.Coin) == `USD` && setting.Market == model.Ftx) {
			usdAmount[key][setting.Market] += balance.Amount
			// todo 可用usd数量需要减去现有所有借币负债总额
		}
		carryStatus = &CarryStatus{Market: setting.Market, Symbol: setting.Symbol, LimitSell: balance.AvailableWithBorrow,
			LimitBuy: math.NaN(), Holding: balance.Amount, ValueInUsd: balance.UsdValue,
			RateInAll: balance.UsdValue / spotBalance[key][setting.Market]}
	}
	if carryStatus == nil { // todo 目前都按照不可借币处理
		carryStatus = &CarryStatus{Market: setting.Market, Symbol: setting.Symbol, LimitSell: 0, LimitBuy: math.NaN(),
			Holding: 0, ValueInUsd: 0, RateInAll: 0}
	}
	return carryStatus
}

func initStatus(key, secret string, setting *model.Setting) {
	if setting == nil {
		return
	}
	tailSpot := model.GetSpotTail(setting.Market)
	tailPerp := model.GetPerpTail(setting.Market)
	var carryStatus *CarryStatus
	fundingRate := 0.0
	if setting.Symbol[len(setting.Symbol)-len(tailSpot):] == tailSpot {
		carryStatus = createFromBalance(key, secret, setting)
	} else if setting.Symbol[len(setting.Symbol)-len(tailPerp):] == tailPerp {
		carryStatus = createFromPosition(key, secret, setting)
		_, fundingRate = api.GetFundingRate(setting.Market, setting.Symbol, nil)
		fundingRate *= 0.9
	}
	if carryStatus == nil {
		return
	}
	getTick, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	status[setting.Coin][setting.Market][setting.Symbol][key] = carryStatus
	if getTick && carryStatus.ValueInUsd == 0 {
		carryStatus.ValueInUsd = carryStatus.Holding * tick.Bids[0].Price
		marketBalance := swapBalance[key][setting.Market]
		if marketBalance == 0 {
			marketBalance = spotBalance[key][setting.Market]
		}
		if marketBalance > 0 {
			carryStatus.RateInAll = carryStatus.ValueInUsd / marketBalance
		}
	}
	now := time.Now().Unix()
	resetTime := getOKTradeMaxResetTime(key, setting.Symbol) + 600
	if setting.Market == model.OKEX && now > resetTime {
		setOKTradeMaxResetTime(key, setting.Symbol)
		getMax, maxBuy, maxSell := api.GetMaxSize(key, secret, setting.Symbol)
		if getMax {
			carryStatus.LimitSell = maxSell
			carryStatus.LimitBuy = maxBuy
		}
	}
	jump := 7.0
	if carryStatus.RateInAll > 0 {
		carryStatus.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*carryStatus.RateInAll), winRateMin) + fundingRate
		carryStatus.TradeLineSell = math.Max(setting.OpenShortMargin*(0.5-jump*carryStatus.RateInAll), loseRateMax) - fundingRate
	} else {
		carryStatus.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*carryStatus.RateInAll), loseRateMax) + fundingRate
		carryStatus.TradeLineSell = math.Max(setting.OpenShortMargin*(0.5-jump*carryStatus.RateInAll), winRateMin) - fundingRate
	}
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	doReverts := strings.Split(model.AppConfig.CarryClose, `,`)
	accountRates := strings.Split(model.AppConfig.AccountRate, `,`)
	for i := 1; i < len(keys); i++ {
		if keys[i] == key {
			if doReverts[i] == `true` {
				carryStatus.TradeLineBuy = 1
				carryStatus.TradeLineSell = 1
			} else {
				rate, _ := strconv.ParseFloat(accountRates[i], 64)
				carryStatus.TradeLineBuy *= rate
				carryStatus.TradeLineSell *= rate
			}
		}
	}
	if setting.Market == model.OKEX {
		collateral := GetCollateral(key)
		if collateral != nil && collateral.Available > 0 && (collateral.Available-collateral.Occupied)/collateral.Available < 0.1 {
			util.Notice(`doRevert true %s %f %f`, key, collateral.Available, collateral.Occupied, collateral.Rate)
			carryStatus.TradeLineBuy = 1
			carryStatus.TradeLineSell = 1
		}
	}
}

func ClearCarry() {
	timer := time.NewTimer(time.Second)
	for {
		<-timer.C
		for true {
			if !checkSetCrossing(true) {
				break
			} else {
				time.Sleep(time.Millisecond * 10)
			}
		}
		if status == nil {
			status = make(map[string]map[string]map[string]map[string]*CarryStatus)
		}
		balances = make(map[string]map[string][]*model.Balance)
		usdAmount = make(map[string]map[string]float64)
		positions = make(map[string]map[string][]*model.Position)
		coinSettings := model.GetCoinSettings(model.FunctionCross)
		for coin, settings := range coinSettings {
			if status[coin] == nil {
				status[coin] = make(map[string]map[string]map[string]*CarryStatus)
			}
			for _, setting := range settings {
				if setting == nil {
					continue
				}
				if status[coin][setting.Market] == nil {
					status[coin][setting.Market] = make(map[string]map[string]*CarryStatus)
				}
				if status[coin][setting.Market][setting.Symbol] == nil {
					status[coin][setting.Market][setting.Symbol] = make(map[string]*CarryStatus)
				}
				keys, secrets := model.AppConfig.GetKeys(setting.Market)
				for i, key := range keys {
					initStatus(key, secrets[i], setting)
				}
			}
		}
		for coin, settings := range coinSettings {
			makeEqual(settings, status[coin])
		}
		timer.Reset(time.Second * 60)
	}
}

func makeEqual(settings []*model.Setting, coinStatus map[string]map[string]map[string]*CarryStatus) {
	var holdings []float64
	for _, setting := range settings {
		keys, _ := model.AppConfig.GetKeys(setting.Market)
		if holdings == nil {
			holdings = make([]float64, len(keys))
		}
		for i, key := range keys {
			if coinStatus[setting.Market] != nil && coinStatus[setting.Market][setting.Symbol] != nil ||
				coinStatus[setting.Market][setting.Symbol][key] == nil {
				util.Notice(`fail to get status makeEqual %s %s %s`,
					setting.Market, setting.Symbol, key)
				continue
			}
			holdings[i] += coinStatus[setting.Market][setting.Symbol][key].Holding
		}
	}
	var price float64
	var settingEqual *model.Setting
	orderSide := ``
	for i, holding := range holdings {
		for _, setting := range settings {
			keys, secrets := model.AppConfig.GetKeys(setting.Market)
			tickGet, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
			if coinStatus[setting.Market] != nil && coinStatus[setting.Market][setting.Symbol] != nil ||
				coinStatus[setting.Market][setting.Symbol][keys[i]] == nil || !tickGet {
				util.Notice(`fail to get status makeEqual %s %s %s`,
					setting.Market, setting.Symbol, keys[i])
				continue
			}
			carryStatus := coinStatus[setting.Market][setting.Symbol][keys[i]]
			if holding*tick.Bids[0].Price > 10 {
				orderSide = model.OrderSideSell
				if (math.IsNaN(carryStatus.LimitBuy) || carryStatus.LimitBuy > math.Abs(holding)) &&
					tick.Bids[0].Price > price {
					price = tick.Bids[0].Price
					settingEqual = setting
				}
				go api.CancelOrders(keys[i], secrets[i], setting.Market, setting.Symbol)
			}
			if holding*tick.Asks[0].Price < -10 {
				orderSide = model.OrderSideBuy
				if (math.IsNaN(carryStatus.LimitSell) || carryStatus.LimitSell > math.Abs(holding)) &&
					(tick.Asks[0].Price < price || price == 0) {
					price = tick.Asks[0].Price
					settingEqual = setting
				}
				go api.CancelOrders(keys[i], secrets[i], setting.Market, setting.Symbol)
			}
		}
		if price > 0 && settingEqual != nil {
			amount := math.Min(90000000, math.Min(math.Abs(holding), 20000/price))
			amount = model.GetAmountInMarket(settingEqual.Market, settingEqual.Symbol, amount)
			if amount > 0 {
				keys, secrets := model.AppConfig.GetKeys(settingEqual.Market)
				api.PlaceOrder(keys[i], secrets[i], orderSide, model.OrderTypeLimit, settingEqual.Market,
					settingEqual.Symbol, settingEqual.Symbol, ``, model.FunctionComplement, price, price,
					amount, true, true, nil)
			}
		}
	}
}

// ProcessCross todo 计算fundingRate后30s不下单
var ProcessCross = func(setting *model.Setting, tick *model.BidAsk) {
	million := util.GetNowUnixMillion()
	delayTick := int64(0)
	if tick != nil {
		delayTick = million - int64(tick.Ts)
	}
	settings := model.GetCoinSetting(setting.Function, setting.Coin)
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || model.AppPause ||
		(model.AppConfig.Env != `test` && (model.AppConfig.Handle != `1` || delayTick > 30)) ||
		status[setting.Coin] == nil || status[setting.Coin][setting.Market] == nil ||
		status[setting.Coin][setting.Market][setting.Symbol] == nil || settings == nil || len(settings) == 0 {
		return
	}
	for _, settingRelate := range settings {
		tickGet, tickRelate := model.AppMarkets.GetBidAsk(settingRelate.Symbol, settingRelate.Market)
		if !tickGet || million-int64(tickRelate.Ts) > 100 {
			continue
		}
		keysRelate, secretsRelate := model.AppConfig.GetKeys(settingRelate.Market)
		for i, keyRelated := range keysRelate {
			if status[settingRelate.Coin] == nil || status[settingRelate.Coin][settingRelate.Market] == nil ||
				status[settingRelate.Coin][settingRelate.Market][settingRelate.Symbol] != nil ||
				status[settingRelate.Coin][settingRelate.Market][settingRelate.Symbol][keyRelated] == nil || !tickGet {
				util.Notice(`fail to get status makeEqual %s %s %s`,
					settingRelate.Market, settingRelate.Symbol, keyRelated)
				continue
			}
			statusCross := status[setting.Coin][setting.Market][setting.Symbol][keys[i]]
			statusRelate := status[settingRelate.Coin][settingRelate.Market][setting.Symbol][keysRelate[i]]
			if statusCross == nil || statusRelate == nil {
				continue
			}
			amount := math.Min(tick.Bids[0].Amount, tickRelate.Asks[0].Amount)
			line := (tick.Bids[0].Price - tickRelate.Asks[0].Price) / tick.Bids[0].Price
			if (math.IsNaN(statusCross.LimitSell) || statusCross.LimitSell > amount) &&
				(math.IsNaN(statusRelate.LimitBuy) || statusRelate.LimitBuy > amount) &&
				statusCross.TradeLineSell < line && statusRelate.TradeLineBuy < line {
				util.Notice(`cross trade `)
				go api.PlaceOrder(keys[i], secrets[i], model.OrderSideSell, model.OrderTypeLimit, setting.Market,
					setting.Symbol, setting.Symbol, ``, model.FunctionCross, tick.Bids[0].Price, tick.Bids[0].Price, amount, true, true, nil)
				return
			}
			amount = math.Min(tick.Asks[0].Amount, tickRelate.Bids[0].Amount)
			line = (tickRelate.Bids[0].Price - tick.Asks[0].Price) / tick.Asks[0].Price
			if (math.IsNaN(statusCross.LimitBuy) || statusCross.LimitBuy > amount) &&
				(math.IsNaN(statusRelate.LimitSell) || statusRelate.LimitSell > amount) &&
				statusCross.TradeLineBuy < line && statusRelate.TradeLineSell < line {
				util.Notice(`cross trade `)
				return
			}
		}
	}
}

func placeCross(key, keyRelate, secret, secretRelate, side, sideRelate string, price, priceRelate, amount, amountRelate float64) {
	if !checkSetCrossing(true) {
		defer checkSetCrossing(false)
	} else {
		//util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
		return
	}
	placeSuccess := true
	if setting.Market == model.OKEX {
		placeSuccess = api.PlacePairOKEX(key, model.GetCoin(setting.Market, setting.Symbol), sidePerp, sideRelated,
			model.OrderTypeLimit, perpPrice, relatedPrice, amount)
	} else {
		go api.PlaceOrder(key, secret, sidePerp, model.OrderTypeLimit, setting.Market, setting.Symbol,
			``, ``, model.FunctionCarry, perpPrice, perpPrice,
			amount, true, true, postOrderCarry)
		api.PlaceOrder(key, secret, sideRelated, model.OrderTypeLimit, setting.Market, setting.SymbolRelated,
			``, ``, model.FunctionCarry, relatedPrice, relatedPrice,
			amount, true, true, postOrderCarry)
		time.Sleep(time.Second / 5)
	}
	if placeSuccess {
		usdAvailable := getUsdAvailable(key)
		balanceAllValue := getBalanceAll(key)
		if sidePerp == model.OrderSideSell {
			perpPrice = tickPerp.Bids[0].Price
			relatedPrice = tickRelated.Asks[0].Price
			setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)+amount)
			balance.Amount += amount
			balance.AvailableWithBorrow += amount
			balance.UsdValue += amount * perpPrice
			if carryType == carryTypeOpen {
				usdAvailable -= amount * perpPrice
				setUsdAvailable(key, usdAvailable)
			}
		} else if sidePerp == model.OrderSideBuy {
			perpPrice = tickPerp.Asks[0].Price
			relatedPrice = tickRelated.Bids[0].Price
			setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)-amount)
			balance.Amount -= amount
			balance.AvailableWithBorrow -= amount
			balance.UsdValue -= amount * perpPrice
			if carryType == carryTypeRevert {
				usdAvailable += amount * relatedPrice
				setUsdAvailable(key, usdAvailable)
			}
		}
		setCarryBalance(key, coin, balance)
		setUsdRate(key, usdAvailable/balanceAllValue)
	}
}

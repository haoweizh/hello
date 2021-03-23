package carry

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strings"
	"sync"
	"time"
)

const OrderPriceLimit = 0
const UsdUpLine = 500000.0 //单币种持仓最大限额，正负范围
const revertDis = 0.005
const openValueLimit = 10000.0

var carryLock sync.Mutex
var carrying bool
var doCarry = false
var symbolHighest, symbolLowest string
var lowest = math.NaN()
var highest = math.NaN()
var usdAvailable = make(map[string]float64)                   // key - float64
var usdRate = make(map[string]float64)                        // key - float64
var balanceAll = make(map[string]float64)                     // key - balance value in all
var carryBalance = make(map[string]map[string]*model.Balance) // key - coin - balance
var carryAmount = make(map[string]map[string]float64)         // key - perp - float64

func getUsdAvailable(key string) float64 {
	carryLock.Lock()
	defer carryLock.Unlock()
	return usdAvailable[key]
}

func setUsdAvailable(key string, value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	usdAvailable[key] = value
}

func setBalanceAll(key string, value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	balanceAll[key] = value
}

func getBalanceAll(key string) (value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	return balanceAll[key]
}

func getUsdRate(key string) float64 {
	carryLock.Lock()
	defer carryLock.Unlock()
	return usdRate[key]
}

func setUsdRate(key string, value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	usdRate[key] = value
}

func getCarryAmount(key, perp string) float64 {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryAmount[key] == nil {
		return 0
	}
	return carryAmount[key][perp]
}

func setCarryAmount(key, perp string, amount float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryAmount[key] == nil {
		carryAmount[key] = make(map[string]float64)
	}
	carryAmount[key][perp] = amount
}

func setCarryBalance(key, coin string, balance *model.Balance) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryBalance[key] == nil {
		carryBalance[key] = make(map[string]*model.Balance)
	}
	carryBalance[key][coin] = balance
}

func getCarryBalance(key, coin string) (balance *model.Balance) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryBalance[key] == nil {
		return nil
	}
	return carryBalance[key][coin]
}

func checkSetCarrying(value bool) (before bool) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if value && carrying {
		return carrying
	} else {
		temp := carrying
		carrying = value
		return temp
	}
}

func clearCarryBalance() {
	for doCarry {
		for true {
			if !checkSetCarrying(true) {
				break
			} else {
				time.Sleep(time.Millisecond * 200)
			}
		}
		util.Notice(`...... enter clearing carry balance`)
		time.Sleep(time.Second * 3)
		markets := model.GetMarkets()
		for _, market := range markets {
			keys, secrets := model.AppConfig.GetKeys(market)
			for i, key := range keys {
				_, balances := api.GetBalances(key, secrets[i], market, 0)
				_, accounts := api.GetAccounts(key, secrets[i], market)
				balanceAllValue := 0.0
				localUsdAvailable := 0.0
				for _, value := range balances {
					coin := strings.ToUpper(value.Coin)
					setCarryBalance(key, coin, value)
					settingCoins := model.GetSettingCoins(model.FunctionCarry, model.Ftx)
					if settingCoins[coin] {
						balanceAllValue += value.UsdValue
					}
					if coin == `USD` {
						localUsdAvailable = value.Available
						setUsdAvailable(key, value.Available)
						balanceAllValue += value.Available
					}
				}
				setUsdRate(key, localUsdAvailable/balanceAllValue)
				setBalanceAll(key, balanceAllValue)
				util.Notice(fmt.Sprintf(`[carry] %s usd:%f %f len(blances):%d`,
					key, localUsdAvailable, usdRate[key], len(balances)))
				settings := model.GetSettings(model.FunctionCarry, market)
				for _, items := range settings {
					for _, item := range items {
						makeEqual(key, secrets[i], item, balances, accounts)
					}
				}
			}
		}
		util.Notice(`...... exit clearing carry balance`)
		checkSetCarrying(false)
		time.Sleep(time.Second * 60)
	}
}

// setting.GridPriceDistance: 收回下单是要求的利润(可以为负数)
var ProcessCarry = func(setting *model.Setting, tick *model.BidAsk) {
	if !doCarry {
		go clearCarryBalance()
		doCarry = true
		return
	}
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	symbolRelated := setting.GetRelatedSymbol()
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	now := time.Now()
	million := util.GetNowUnixMillion()
	if tickPerp == nil || tickRelated == nil || tickPerp.Asks == nil || tickPerp.Bids == nil ||
		tickRelated.Asks == nil || tickRelated.Bids == nil || model.AppConfig.Handle != `1` || setting == nil ||
		model.AppPause || (model.AppConfig.Env != `test` &&
		(million-int64(tickRelated.Ts) > 2000 || million-int64(tickPerp.Ts) > 2000 || million-int64(tick.Ts) > 20)) {
		return
	}
	scoreOpen := 1 - tickRelated.Asks[0].Price/tickPerp.Bids[0].Price
	scoreClose := 1 - tickRelated.Bids[0].Price/tickPerp.Asks[0].Price
	model.AppMetric.AddCarry(setting.Market, setting.Symbol, scoreOpen, scoreClose)
	if math.IsNaN(highest) || scoreOpen > highest || setting.Symbol == symbolHighest {
		highest = scoreOpen
		symbolHighest = setting.Symbol
		model.AppMetric.AddCarry(setting.Market, `ftx开仓价差++++`, highest, math.NaN())
	}
	if math.IsNaN(lowest) || scoreClose < lowest || setting.Symbol == symbolLowest {
		lowest = scoreClose
		symbolLowest = setting.Symbol
		model.AppMetric.AddCarry(setting.Market, `ftx开仓价差----`, math.NaN(), lowest)
	}
	model.SetCarryInfo(`[current high-low]`, fmt.Sprintf(`highest %s %f lowest %s %f`, symbolHighest, highest, symbolLowest, lowest))
	marketInfo := map[string]interface{}{`symbol highest`: symbolHighest, `symbol lowest`: symbolLowest}
	if !math.IsNaN(highest) {
		marketInfo[`highest`] = highest
	}
	if !math.IsNaN(lowest) {
		marketInfo[`lowest`] = lowest
	}
	model.SetCarryInfos(`market_info`, `market_info`, marketInfo)
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	begin := 0
	step := 1
	if (now.Hour() < 6 && now.Hour() > 2) || now.Second()%2 == 0 {
		begin = len(keys) - 1
		step = -1
	}
	for i := begin; i >= 0 && i < len(keys); i += step {
		sidePerp, sideRelated, amount := calcCarryOpen(setting, tickPerp, tickRelated, keys[i], setting.Symbol,
			setting.Symbol, scoreOpen, scoreClose, scoreOpen, scoreClose)
		if amount > 0 {
			go placeCarry(setting, tickPerp, tickRelated, keys[i], secrets[i], sidePerp, sideRelated,
				scoreOpen, scoreClose, amount)
			break
		}
	}
}

func placeCarry(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, secret, sidePerp, sideRelated string,
	scoreOpen, scoreClose, amount float64) {
	if !checkSetCarrying(true) {
		defer checkSetCarrying(false)
	} else {
		util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
		return
	}
	symbolRelated := setting.GetRelatedSymbol()
	balance := getCarryBalance(key, setting.GetCoin())
	if balance == nil {
		return
	}
	perpPrice := tickPerp.Asks[0].Price
	relatedPrice := tickRelated.Bids[0].Price
	if sidePerp == model.OrderSideSell {
		perpPrice = tickPerp.Bids[0].Price
		relatedPrice = tickRelated.Asks[0].Price
		setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)+amount)
		balance.Amount += amount
		balance.UsdValue += amount * perpPrice
		setUsdAvailable(key, getUsdAvailable(key)-amount*perpPrice)
	} else if sidePerp == model.OrderSideBuy {
		perpPrice = tickPerp.Asks[0].Price
		relatedPrice = tickRelated.Bids[0].Price
		setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)-amount)
		balance.Amount -= amount
		balance.UsdValue -= amount * perpPrice
		setUsdAvailable(key, getUsdAvailable(key)+amount*relatedPrice)
	}
	now := int(util.GetNowUnixMillion())
	util.Notice(fmt.Sprintf(`carry%s->%s delay %d %d perp[%f %f %f %f] related[%f %f %f %f] with score open:%f close:%f 
	rate sum %f amount %f worth %f time in million %d`,
		setting.Symbol, symbolRelated, now-tickPerp.TsReceived, now-tickRelated.TsReceived, tickPerp.Bids[0].Price,
		tickPerp.Bids[0].Amount, tickPerp.Asks[0].Price, tickPerp.Asks[0].Amount, tickRelated.Bids[0].Price,
		tickRelated.Bids[0].Amount, tickRelated.Asks[0].Price, tickRelated.Asks[0].Amount, scoreOpen, scoreClose,
		0.0, amount, amount*tickPerp.Asks[0].Price, util.GetNowUnixMillion()))
	go api.PlaceOrder(key, secret, sidePerp, model.OrderTypeLimit, setting.Market, setting.Symbol,
		``, ``, ``, ``, model.FunctionCarry, perpPrice, perpPrice,
		amount, true)
	api.PlaceOrder(key, secret, sideRelated, model.OrderTypeLimit, setting.Market, symbolRelated,
		``, ``, ``, ``, model.FunctionCarry, relatedPrice, relatedPrice,
		amount, true)
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	if key == keys[0] {
		time.Sleep(time.Millisecond * 150)
	} else {
		time.Sleep(time.Millisecond * 200)
	}
}

func getCarryAmounts(setting *model.Setting, balances []*model.Balance, accounts []*model.Account) (
	success bool, amountPerp, amountRelated float64) {
	for _, account := range accounts {
		if account != nil && account.Currency == setting.Symbol {
			amountPerp = account.Free
			for _, balance := range balances {
				if strings.ToUpper(balance.Coin)+`-PERP` == strings.ToUpper(account.Currency) {
					amountRelated = balance.Amount
					return true, amountPerp, amountRelated
				}
			}
		}
	}
	return false, amountPerp, amountRelated
}

func makeEqual(key, secret string, setting *model.Setting, balances []*model.Balance, accounts []*model.Account) (
	symbol string, price float64, equal bool) {
	settingSymbol := setting.Symbol
	coin := setting.GetCoin()
	symbolRelated := setting.GetRelatedSymbol()
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	if tickPerp == nil || tickRelated == nil {
		return ``, 0, true
	}
	success, amountPerp, amountRelated := getCarryAmounts(setting, balances, accounts)
	if !success {
		return
	}
	amount := amountPerp + amountRelated
	orderSide := model.OrderSideBuy
	if amount < math.Max(math.Abs(amountPerp), math.Abs(amountRelated)) {
		if amountPerp < 0 && amountRelated > 0 {
			setCarryAmount(key, settingSymbol, math.Min(math.Abs(amountPerp), math.Abs(amountRelated)))
		} else if amountPerp > 0 && amountRelated < 0 {
			setCarryAmount(key, settingSymbol, -1*math.Min(math.Abs(amountPerp), math.Abs(amountRelated)))
		}
	} else {
		setCarryAmount(key, settingSymbol, 0)
	}
	balance := getCarryBalance(key, coin)
	if amount > 0 {
		orderSide = model.OrderSideSell
		if tickPerp.Bids[0].Price < (1-revertDis)*tickRelated.Bids[0].Price && amount < balance.AvailableWithBorrow {
			symbol = symbolRelated
			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
		} else if tickPerp.Bids[0].Price > (1+revertDis)*tickRelated.Bids[0].Price {
			symbol = settingSymbol
			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
		} else if math.Abs(amountPerp) < math.Abs(amountRelated) && amount < balance.AvailableWithBorrow {
			symbol = symbolRelated
			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
		} else {
			symbol = settingSymbol
			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
		}
	} else {
		orderSide = model.OrderSideBuy
		if tickPerp.Asks[0].Price < (1-revertDis)*tickRelated.Asks[0].Price {
			symbol = settingSymbol
			price = tickPerp.Asks[0].Price * (1 + OrderPriceLimit)
		} else if tickPerp.Asks[0].Price > (1+revertDis)*tickRelated.Asks[0].Price {
			symbol = symbolRelated
			price = tickRelated.Asks[0].Price * (1 + OrderPriceLimit)
		} else if math.Abs(amountPerp) > math.Abs(amountRelated) {
			symbol = settingSymbol
			price = tickPerp.Asks[0].Price * (1 + OrderPriceLimit)
		} else {
			symbol = symbolRelated
			price = tickRelated.Asks[0].Price * (1 + OrderPriceLimit)
		}
		balance := getCarryBalance(key, `USD`)
		if symbol == symbolRelated && (balance != nil && balance.Borrow > 0) {
			amount = 0
		}
	}
	amount = math.Min(math.Abs(amount), 20000/price)
	amount = api.FormatAmount(setting.Market, symbol, math.Abs(amount))
	if amount > 0 {
		resultPerp := api.CancelOrders(key, secret, setting.Market, settingSymbol)
		resultRelated := api.CancelOrders(key, secret, setting.Market, symbolRelated)
		util.Notice(fmt.Sprintf(`cancel all perp:%v related:%v >>>>>> equal %s %f, %s %f = %s %f`,
			resultPerp, resultRelated, settingSymbol, amountPerp, symbolRelated, amountRelated, orderSide, amount))
		api.PlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, symbol, ``,
			``, ``, ``, model.FunctionComplement, price, price, amount, true)
	}
	return
}

func calcCarryOpen(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, symbolHigh, symbolLow string,
	scoreOpen, scoreClose, scoreHigh, scoreLow float64) (sidePerp, sideRelated string, amount float64) {
	var bidAmount, askAmount float64
	setOpen := setting.OpenShortMargin
	setClose := setting.CloseShortMargin
	valueLow := setting.AmountLimit
	usdBalance := getCarryBalance(key, `USD`)
	usdRate := getUsdRate(key)
	usdAvailable := getUsdAvailable(key)
	coin := setting.GetCoin()
	balance := getCarryBalance(key, coin)
	if balance == nil {
		model.SetCarryInfo(`warning `+coin, fmt.Sprintf(`slave: balace not available!!! %s`, key))
		model.SetCarryInfos(`coin_absent`, key+`_`+coin, map[string]interface{}{`absent`: coin, `key`: key})
		return ``, ``, 0
	} else {
		model.RemoveCarryInfo(`warning ` + coin)
		model.RemoveCarryInfos(`coin_absent`, key+`_`+coin)
	}
	if usdBalance == nil {
		return ``, ``, 0
	}
	balanceAllValue := getBalanceAll(key)
	coinRate := math.Abs(balance.UsdValue) / balanceAllValue
	usdLowLine := model.AppConfig.Amount
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	localOpenValueLimit := math.Min(openValueLimit, 0.5*balanceAllValue)
	localUsdUpLine := UsdUpLine
	setOpen = (1.5 - usdRate) * setOpen
	revert := math.Abs(setting.GridPriceDistance) * (usdRate - 0.5)
	setClose = -1
	table := fmt.Sprintf(`%s_dynamic_`, model.FunctionCarry)
	if len(keys) > 1 && keys[0] != key {
		table += `slave`
		localOpenValueLimit = 3500
		valueLow = 0
		usdLowLine = 30000
		localUsdUpLine = 60000
	}
	jump := 7.0
	jumpRevert := 5.0
	setOpen = math.Max(setOpen*(0.5+jump*coinRate), 0.003)
	if revert > 0 {
		revert = revert / (1 + jumpRevert*coinRate)
	} else {
		revert = revert / (1 - math.Min(0.9, jumpRevert*coinRate))
	}
	revert = math.Max(revert, -0.003)
	model.SetCarryInfo(table+setting.Symbol,
		fmt.Sprintf(`%s 参数:(%f %f %f) 计算结果(%f %f %f) 当前市场(%f %f) usdRate:%favailable:%f coinRate: %f`,
			table, setting.OpenShortMargin, setting.CloseShortMargin, setting.GridPriceDistance, setOpen, setClose,
			revert, scoreOpen, scoreClose, usdRate, usdAvailable, coinRate))
	carryInfo := map[string]interface{}{`+开仓`: setting.OpenShortMargin, `-开仓`: setting.CloseShortMargin,
		`平仓`: setting.GridPriceDistance, `动态+开仓`: setOpen, `动态-开仓`: setClose, `动态平仓`: revert,
		table: setting.Symbol, `市场+开`: scoreOpen, `市场-开`: scoreClose, `usdRate`: usdRate,
		`usdAvailable`: usdAvailable, `coinRate`: coinRate}
	model.SetCarryInfos(table, setting.Symbol, carryInfo)
	carryAmount := getCarryAmount(key, setting.Symbol)
	if (scoreLow < setClose && setting.Symbol == symbolLow) || (carryAmount > 0 && scoreClose <= -1*revert) {
		bidAmount = tickPerp.Asks[0].Amount
		askAmount = tickRelated.Bids[0].Amount
		sidePerp = model.OrderSideBuy
		sideRelated = model.OrderSideSell
	} else if (scoreHigh > setOpen && setting.Symbol == symbolHigh) || (carryAmount < 0 && scoreOpen >= revert) {
		bidAmount = tickRelated.Asks[0].Amount
		askAmount = tickPerp.Bids[0].Amount
		sidePerp = model.OrderSideSell
		sideRelated = model.OrderSideBuy
	}
	markPrice := tickPerp.Asks[0].Price
	amount = math.Min(bidAmount, askAmount)
	// 开仓时:数量<持仓+可借
	if (setting.Symbol == symbolLow && scoreLow < setClose) || (setting.Symbol == symbolHigh && scoreHigh > setOpen) {
		if sideRelated == model.OrderSideSell {
			amount = math.Max(0, math.Min(balance.AvailableWithBorrow-localOpenValueLimit/markPrice, math.Abs(amount)))
		}
	} else { // 反向关仓量要<=持仓
		amount = math.Min(math.Abs(carryAmount), amount)
	}
	if sideRelated == model.OrderSideBuy {
		amount = math.Min(amount, usdAvailable/markPrice)
	}
	amount = math.Min(amount, localOpenValueLimit/markPrice)
	amountPerp := api.FormatAmount(setting.Market, setting.Symbol, amount)
	amountRelated := api.FormatAmount(setting.Market, setting.GetRelatedSymbol(), amount)
	amount = math.Min(amountPerp, amountRelated)
	// usd所剩太少且还要再买 || 持仓太多且还要再买 || 下单太小
	if (sideRelated == model.OrderSideBuy && (usdAvailable < usdLowLine || balance.UsdValue > localUsdUpLine)) ||
		(sideRelated == model.OrderSideSell && (balance.UsdValue < localUsdUpLine/-10 || balance.UsdValue < -50000.0)) ||
		math.Abs(amount)*markPrice < valueLow {
		amount = 0
	}
	if amount > 0 {
		util.Notice(fmt.Sprintf(`>>>> %s high:%s %f low:%s %f symbl: %s %s usd available:%f amount：%f 
			carryAmount: %f`,
			key, symbolHigh, scoreHigh, symbolLow, scoreLow, setting.Symbol, sidePerp, usdAvailable, amount, carryAmount))
	}
	return sidePerp, sideRelated, amount
}

func InitFtxBalance(key, secret, function string) {
	api.InitMarketInfos()
	settings := model.GetSettings(function, model.Ftx)
	_, balances := api.GetBalances(key, secret, model.Ftx, 0)
	balanceMap := make(map[string]*model.Balance)
	for _, balance := range balances {
		balanceMap[balance.Coin] = balance
	}
	i := 0
	for _, items := range settings {
		coin := items[0].GetCoin()
		balance := balanceMap[coin]
		if balance == nil {
			if model.MarketInfos[model.Ftx] == nil {
				continue
			}
			marketInfo := model.MarketInfos[model.Ftx][items[0].GetRelatedSymbol()]
			if marketInfo == nil {
				continue
			}
			order := api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeMarket, model.Ftx, items[0].Symbol, ``,
				``, ``, ``, ``, 0, 0, marketInfo.SizeIncrement, false)
			if order.OrderId == `` {
				i++
				fmt.Println(fmt.Sprintf(`%d order return :%s %s`, i, order.ErrCode, items[0].Symbol))
			}
		}
	}
}

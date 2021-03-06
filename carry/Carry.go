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
const UsdUpLine = 300000
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
				balanceAll := 0.0
				usdAvailable := 0.0
				for _, value := range balances {
					coin := strings.ToUpper(value.Coin)
					setCarryBalance(key, coin, value)
					balanceAll += value.UsdValue
					if coin == `USD` {
						usdAvailable = value.Available
						setUsdAvailable(key, value.Available)
					}
				}
				setUsdRate(key, usdAvailable/balanceAll)
				util.Notice(fmt.Sprintf(`[carry] %s usd:%f %f`, key, usdAvailable, usdRate[key]))
				settings := model.GetSettings(model.FunctionCarry, market)
				for _, items := range settings {
					for _, item := range items {
						if item.Function == model.FunctionCarry {
							makeEqual(key, secrets[i], item, balances, accounts)
						}
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
		model.AppPause || million-int64(tickRelated.Ts) > 2000 || million-int64(tickPerp.Ts) > 2000 || million-int64(tick.Ts) > 20 {
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
	model.SetCarryInfo(`[grid] `+setting.Symbol,
		fmt.Sprintf(`current: [%s] score range: [%f ~ %f] revert: [%f] [open %f close: %f]`,
			setting.Symbol, setting.OpenShortMargin, setting.CloseShortMargin, setting.GridPriceDistance,
			scoreOpen, scoreClose))
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	begin := 0
	step := 1
	if (now.Hour() < 6 && now.Hour() > 2) || now.Second()%3 == 0 {
		begin = len(keys) - 1
		step = -1
	}
	for i := begin; i >= 0 && i < len(keys); i += step {
		usdRate := getUsdRate(keys[i])
		usdAvailable := getUsdAvailable(keys[i])
		carryInfo := fmt.Sprintf("%s limit :%f [lowest:%s %f][highest: %s %f][available usd: <%f> %f]",
			keys[0], model.AppConfig.Amount, symbolLowest, lowest, symbolHighest, highest, usdAvailable, usdRate)
		model.SetCarryInfo(fmt.Sprintf(`[grid-setting]`), carryInfo)
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
		setUsdAvailable(key, getUsdAvailable(key)-amount*perpPrice)
	} else if sidePerp == model.OrderSideBuy {
		perpPrice = tickPerp.Asks[0].Price
		relatedPrice = tickRelated.Bids[0].Price
		setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)-amount)
		balance.Amount -= amount
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
	revert := setting.GridPriceDistance
	valueLow := setting.AmountLimit
	usdBalance := getCarryBalance(key, `USD`)
	usdRate := getUsdRate(key)
	usdAvailable := getUsdAvailable(key)
	coin := setting.GetCoin()
	balance := getCarryBalance(key, coin)
	if balance == nil {
		model.SetCarryInfo(`warning`+coin, fmt.Sprintf(`slave: balace not available!!! %s`, key))
		return ``, ``, 0
	}
	if usdBalance == nil {
		return ``, ``, 0
	}
	line := model.AppConfig.Amount
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	localLimit := openValueLimit
	if len(keys) > 1 && keys[0] != key && setting.Symbol == `BTMX-PERP` {
		revert = -0.0035
	}
	if len(keys) > 1 && keys[0] != key && setting.Symbol != `BTMX-PERP` && setting.Symbol != `AMPL-PERP` {
		setOpen = (1.5 - usdRate) * setOpen
		if revert == 0 {
			revert = 0.001
		}
		if revert > 0 {
			revert = revert * (usdRate - 0.5)
		} else if revert < 0 {
			revert = revert * (1.5 - usdRate)
		}
		valueLow = 0
		line = 10000
		localLimit = 1000.0
		setClose = -1 * setOpen
		if keys[1] != key {
			//setOpen = 0.01
			//setClose = -0.01
			//revert = 0.001
			line = 1000
			localLimit = 10
		}
		model.SetCarryInfo(`[dynamic]`, fmt.Sprintf(`[lowest:%s %f][highest: %s %f] open:%f close:%f 
			revert:%f usdRate:%favailable:%f`,
			symbolLowest, lowest, symbolHighest, highest, setOpen, setClose, revert, usdRate, usdAvailable))
	}
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
			amount = math.Max(0, math.Min(balance.AvailableWithBorrow-localLimit/markPrice, math.Abs(amount)))
		}
	} else { // 反向关仓量要<=持仓
		amount = math.Min(math.Abs(carryAmount), amount)
	}
	if sideRelated == model.OrderSideBuy {
		amount = math.Min(amount, usdAvailable/markPrice)
	}
	amount = math.Min(amount, localLimit/markPrice)
	amountPerp := api.FormatAmount(setting.Market, setting.Symbol, amount)
	amountRelated := api.FormatAmount(setting.Market, setting.GetRelatedSymbol(), amount)
	amount = math.Min(amountPerp, amountRelated)
	// usd所剩太少且还要再买 || 持仓太多且还要再买 || 下单太小
	if (sideRelated == model.OrderSideBuy && (usdAvailable < line || balance.UsdValue > UsdUpLine)) ||
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
			fmt.Println(`absent ` + coin)
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

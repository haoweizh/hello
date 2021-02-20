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

var carryLock sync.Mutex
var carrying bool
var doCarry = false
var symbolHighest, symbolLowest string
var highest, lowest float64

var usdAvailable = make(map[string]float64)
var usdRate = make(map[string]float64)
var carryBalance = make(map[string]map[string]*model.Balance)

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
				api.RefreshAccount(key, secrets[i], market)
				settings := model.GetSettings(model.FunctionCarry, market)
				for _, items := range settings {
					for _, item := range items {
						if item.Function == model.FunctionCarry {
							makeEqual(key, secrets[i], item, balances)
						}
					}
				}
				//symbols := model.GetMarketSymbols(market)
				balanceAll := 0.0
				if carryBalance[key] == nil {
					carryBalance[key] = make(map[string]*model.Balance)
				}
				for _, value := range balances {
					coin := strings.ToUpper(value.Coin)
					carryBalance[key][coin] = value
					balanceAll += value.UsdValue
					if coin == `USD` {
						usdAvailable[key] = value.Available
					}
				}
				usdRate[key] = usdAvailable[key] / balanceAll
				util.Notice(fmt.Sprintf(`[carry] usd:%f %f`, usdAvailable[key], usdRate[key]))
			}
		}
		util.Notice(`...... exit clearing carry balance`)
		checkSetCarrying(false)
		time.Sleep(time.Second * 60)
	}
}

// setting.GridPriceDistance: 收回下单是要求的利润(可以为负数)
var ProcessCarry = func(setting *model.Setting) {
	if !doCarry {
		go clearCarryBalance()
		doCarry = true
		return
	}
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	symbolRelated := setting.GetRelatedSymbol()
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	now := util.GetNowUnixMillion()
	if tickPerp == nil || tickRelated == nil || tickPerp.Asks == nil || tickPerp.Bids == nil ||
		tickRelated.Asks == nil || tickRelated.Bids == nil || model.AppConfig.Handle != `1` ||
		model.AppPause || now-int64(tickRelated.Ts) > 50 || now-int64(tickPerp.Ts) > 50 || setting == nil {
		return
	}
	scoreOpen := 1 - tickRelated.Asks[0].Price/tickPerp.Bids[0].Price
	scoreClose := 1 - tickRelated.Bids[0].Price/tickPerp.Asks[0].Price
	if scoreOpen > highest || setting.Symbol == symbolHighest {
		highest = scoreOpen
		symbolHighest = setting.Symbol
	}
	if scoreClose < lowest || setting.Symbol == symbolLowest {
		lowest = scoreClose
		symbolLowest = setting.Symbol
	}
	model.SetCarryInfo(`[grid] `+setting.Symbol,
		fmt.Sprintf(`current: [%s] score range: [%f ~ %f] revert: [%f] [open %f close: %f]`,
			setting.Symbol, setting.OpenShortMargin, setting.CloseShortMargin, setting.GridPriceDistance,
			scoreOpen, scoreClose))
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	for i, key := range keys {
		carryInfo := fmt.Sprintf("limit :%f [lowest:%s %f][highest: %s %f][available usd: <%f> %f]",
			model.AppConfig.Amount, symbolLowest, lowest, symbolHighest, highest, usdAvailable[key], usdRate[key])
		model.SetCarryInfo(fmt.Sprintf(`[grid-setting]%s`, key[0:5]), carryInfo)
		sidePerp, sideRelated, amount := calcCarryOpen(setting, tickPerp, tickRelated, key, setting.Symbol, setting.Symbol,
			scoreOpen, scoreClose, scoreOpen, scoreClose)
		if amount > 0 {
			go placeCarry(setting, tickPerp, tickRelated, key, secrets[i], sidePerp, sideRelated, scoreOpen, scoreClose, amount)
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
		setting.GridAmount += amount
		balance.Amount += amount
		usdAvailable[key] -= amount * perpPrice
	} else if sidePerp == model.OrderSideBuy {
		perpPrice = tickPerp.Asks[0].Price
		relatedPrice = tickRelated.Bids[0].Price
		setting.GridAmount -= amount
		balance.Amount -= amount
		usdAvailable[key] += amount * relatedPrice
	}
	before := time.Now().UnixNano() / 1000000
	util.Notice(fmt.Sprintf(`carry%s->%s delay %d %d perp[%f %f %f %f] related[%f %f %f %f] with score open:%f close:%f 
	rate sum %f amount %f worth %f`,
		setting.Symbol, symbolRelated, before-int64(tickPerp.Ts), before-int64(tickRelated.Ts), tickPerp.Bids[0].Price,
		tickPerp.Bids[0].Amount, tickPerp.Asks[0].Price, tickPerp.Asks[0].Amount, tickRelated.Bids[0].Price,
		tickRelated.Bids[0].Amount, tickRelated.Asks[0].Price, tickRelated.Asks[0].Amount, scoreOpen, scoreClose,
		0.0, amount, amount*tickPerp.Asks[0].Price))
	go api.PlaceOrder(key, secret, sidePerp, model.OrderTypeLimit, setting.Market, setting.Symbol,
		``, ``, ``, ``, model.FunctionCarry, perpPrice, perpPrice,
		amount, true)
	api.PlaceOrder(key, secret, sideRelated, model.OrderTypeLimit, setting.Market, symbolRelated,
		``, ``, ``, ``, model.FunctionCarry, relatedPrice, relatedPrice,
		amount, true)
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	if key == keys[0] {
		time.Sleep(time.Millisecond * 100)
	} else {
		time.Sleep(time.Millisecond * 200)
	}
}

func getCarryAmounts(setting *model.Setting, balances []*model.Balance) (amountPerp, amountRelated float64) {
	account := model.AppAccounts.GetAccount(setting.Market, setting.Symbol)
	if account == nil || account.Currency == `` || !strings.Contains(account.Currency, `-`) {
		amountPerp = 0
	} else {
		amountPerp = account.Free
		for _, balance := range balances {
			if strings.ToUpper(balance.Coin)+`-PERP` == strings.ToUpper(account.Currency) {
				amountRelated = balance.Amount
				break
			}
		}
	}
	return amountPerp, amountRelated
}

func makeEqual(key, secret string, setting *model.Setting, balances []*model.Balance) (
	symbol string, price float64, equal bool) {
	settingSymbol := setting.Symbol
	symbolRelated := setting.GetRelatedSymbol()
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	if tickPerp == nil || tickRelated == nil {
		return ``, 0, true
	}
	amountPerp, amountRelated := getCarryAmounts(setting, balances)
	amount := amountPerp + amountRelated
	orderSide := model.OrderSideBuy
	if amount < math.Max(math.Abs(amountPerp), math.Abs(amountRelated)) {
		if amountPerp < 0 && amountRelated > 0 {
			setting.GridAmount = math.Min(math.Abs(amountPerp), math.Abs(amountRelated))
		} else if amountPerp > 0 && amountRelated < 0 {
			setting.GridAmount = -1 * math.Min(math.Abs(amountPerp), math.Abs(amountRelated))
		}
	} else {
		setting.GridAmount = 0
	}
	go model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
		setting.Market, setting.Symbol, setting.Function).Updates(
		map[string]interface{}{`grid_amount`: setting.GridAmount})
	if amount > 0 {
		orderSide = model.OrderSideSell
		if tickPerp.Bids[0].Price < (1-revertDis)*tickRelated.Bids[0].Price {
			symbol = symbolRelated
			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
		} else if tickPerp.Bids[0].Price > (1+revertDis)*tickRelated.Bids[0].Price {
			symbol = settingSymbol
			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
		} else if math.Abs(amountPerp) > math.Abs(amountRelated) {
			symbol = settingSymbol
			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
		} else {
			symbol = symbolRelated
			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
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
	revert := setting.GridPriceDistance
	if usdRate[key] == 0 {
		return ``, ``, 0
	}
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	if keys[1] == key {
		setOpen += (1 - usdRate[key]) * 0.02
		revert += usdRate[key] * 0.008
		model.SetCarryInfo(`[dynamic]`, fmt.Sprintf(`%f %f %f`, setOpen, usdRate[key], revert))
	}
	if (scoreLow < -1*setOpen && setting.Symbol == symbolLow) ||
		(setting.GridAmount > 0 && scoreClose <= -1*revert) {
		bidAmount = tickPerp.Asks[0].Amount
		askAmount = tickRelated.Bids[0].Amount
		sidePerp = model.OrderSideBuy
		sideRelated = model.OrderSideSell
	} else if (scoreHigh > setOpen && setting.Symbol == symbolHigh) ||
		(setting.GridAmount < 0 && scoreOpen >= revert) {
		bidAmount = tickRelated.Asks[0].Amount
		askAmount = tickPerp.Bids[0].Amount
		sidePerp = model.OrderSideSell
		sideRelated = model.OrderSideBuy
	}
	if (setting.Symbol == symbolLow && scoreLow < -1*setOpen) ||
		(setting.Symbol == symbolHigh && scoreHigh > setOpen) {
		amount = math.Min(usdAvailable[key]/tickPerp.Asks[0].Price, math.Min(bidAmount, askAmount))
	} else {
		amount = math.Min(math.Abs(setting.GridAmount), math.Min(bidAmount, askAmount))
		if sideRelated == model.OrderSideBuy {
			amount = math.Min(amount, usdAvailable[key]/tickRelated.Asks[0].Price)
		}
	}
	line := model.AppConfig.Amount
	if line <= 0 {
		line = 100000
	}
	amount = math.Min(amount, 10000/tickPerp.Asks[0].Price)
	amountPerp := api.FormatAmount(setting.Market, setting.Symbol, amount)
	amountRelated := api.FormatAmount(setting.Market, setting.GetRelatedSymbol(), amount)
	amount = math.Min(amountPerp, amountRelated)
	if (sideRelated == model.OrderSideBuy && usdAvailable[key] < line) ||
		(math.Abs(setting.GridAmount)*tickPerp.Asks[0].Price > setting.AmountLimit &&
			amount*tickPerp.Asks[0].Price < setting.AmountLimit) {
		amount = 0
	}
	if amount*tickPerp.Asks[0].Price < setting.AmountLimit &&
		((sidePerp == model.OrderSideBuy && setting.GridAmount < 0) ||
			(sidePerp == model.OrderSideSell && setting.GridAmount > 0)) {
		amount = 0
	}
	coin := setting.GetCoin()
	balance := getCarryBalance(key, coin)
	if balance == nil || math.Abs(balance.UsdValue) > UsdUpLine ||
		(sideRelated == model.OrderSideSell && amount > balance.Amount) {
		amount = 0
	}
	if amount > 0 {
		util.Notice(fmt.Sprintf(`>>>> %s high:%s %f low:%s %f symbl: %s usd available:%f amount：%f`,
			key, symbolHigh, scoreHigh, symbolLow, scoreLow, setting.Symbol, usdAvailable[key], amount))
	}
	return sidePerp, sideRelated, amount
}

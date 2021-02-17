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
const UsdUpLine = 200000

//const FtxTakerFee = 0.0004275

var carryLock sync.Mutex
var carrying bool
var holding, usdAvailable float64
var doCarry = false
var carryBalance = make(map[string]*model.Balance)
var symbolHighest, symbolLowest string
var highest, lowest float64

func getCarryBalance(coin string) (balance *model.Balance) {
	carryLock.Lock()
	defer carryLock.Unlock()
	return carryBalance[coin]
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
			_, balances := api.GetBalance(``, ``, market, 0)
			api.RefreshAccount(``, ``, market)
			settings := model.GetSettings(model.FunctionCarry, market)
			for _, items := range settings {
				for _, item := range items {
					if item.Function == model.FunctionCarry {
						makeEqual(item, balances)
					}
				}
			}
			symbols := model.GetMarketSymbols(market)
			holding = 0
			valueUsd := 0.0
			for _, value := range balances {
				coin := strings.ToUpper(value.Coin)
				usdSymbol := coin + `/USD`
				carryBalance[coin] = value
				if coin == `USD` {
					usdAvailable = value.Available
				} else if coin == `BTC` || coin == `USDT` || coin == `FTT` {
					valueUsd += value.UsdValue
				} else if symbols[usdSymbol] {
					//coinBorrowAble[usdSymbol] = api.GetMarketInfo(market, usdSymbol)
					holding += math.Abs(value.UsdValue)
				}
			}
			util.Notice(fmt.Sprintf(`[carry] usd:%f valuedUsd:%f holding:%f`, usdAvailable, valueUsd, holding))
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
	carryInfo := fmt.Sprintf("limit :%f current: [%s] score range: [%f ~ %f] revert: [%f]\n"+
		"[open %f close: %f][lowest:%s %f][highest: %s %f][available usd: <%f>] holding: %f",
		model.AppConfig.Amount, setting.Symbol, setting.OpenShortMargin, setting.CloseShortMargin,
		setting.GridPriceDistance, scoreOpen, scoreClose, symbolLowest, lowest, symbolHighest, highest, usdAvailable, holding)
	model.SetCarryInfo(`[grid-setting]`, carryInfo)
	sidePerp, sideRelated, amount := calcCarryOpen(setting, tickPerp, tickRelated, setting.Symbol, setting.Symbol,
		scoreOpen, scoreClose, scoreOpen, scoreClose)
	if scoreOpen > 0.01 || scoreClose < -0.01 {
		before := time.Now().UnixNano() / 1000000
		util.Notice(fmt.Sprintf(`...... perp: %d related: %d %s`, before-int64(tickPerp.Ts),
			before-int64(tickRelated.Ts), carryInfo))
	}
	if amount > 0 {
		go placeCarry(setting, tickPerp, tickRelated, sidePerp, sideRelated, scoreOpen, scoreClose, amount)
	}
}

func placeCarry(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, sidePerp, sideRelated string,
	scoreOpen, scoreClose, amount float64) {
	if !checkSetCarrying(true) {
		defer checkSetCarrying(false)
	} else {
		util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
		return
	}
	symbolRelated := setting.GetRelatedSymbol()
	balance := getCarryBalance(setting.GetCoin())
	if balance == nil {
		return
	}
	util.Notice(fmt.Sprintf(`carry%s->%s with score open:%f close:%f rate sum %f amount %f worth %f`,
		setting.Symbol, symbolRelated, scoreOpen, scoreClose, 0.0, amount, amount*tickPerp.Asks[0].Price))
	perpPrice := tickPerp.Asks[0].Price
	relatedPrice := tickRelated.Bids[0].Price
	if sidePerp == model.OrderSideSell {
		perpPrice = tickPerp.Bids[0].Price
		relatedPrice = tickRelated.Asks[0].Price
		setting.GridAmount += amount
		balance.Free += amount
		usdAvailable -= amount * perpPrice
	} else if sidePerp == model.OrderSideBuy {
		perpPrice = tickPerp.Asks[0].Price
		relatedPrice = tickRelated.Bids[0].Price
		setting.GridAmount -= amount
		balance.Free -= amount
		usdAvailable += amount * relatedPrice
	}
	go api.PlaceOrder(``, ``, sidePerp, model.OrderTypeLimit, setting.Market, setting.Symbol,
		``, ``, ``, ``, model.FunctionCarry, perpPrice, perpPrice,
		amount, true)
	api.PlaceOrder(``, ``, sideRelated, model.OrderTypeLimit, setting.Market, symbolRelated,
		``, ``, ``, ``, model.FunctionCarry, relatedPrice, relatedPrice,
		amount, true)
	time.Sleep(time.Millisecond * 200)
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

func makeEqual(setting *model.Setting, balances []*model.Balance) (symbol string, price float64, equal bool) {
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
		if tickPerp.Bids[0].Price < 0.99*tickRelated.Bids[0].Price {
			symbol = symbolRelated
			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
		} else if tickPerp.Bids[0].Price > 1.01*tickRelated.Bids[0].Price {
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
		if tickPerp.Asks[0].Price < 0.99*tickRelated.Asks[0].Price {
			symbol = settingSymbol
			price = tickPerp.Asks[0].Price * (1 + OrderPriceLimit)
		} else if tickPerp.Asks[0].Price > 1.01*tickRelated.Asks[0].Price {
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
	amount = math.Min(amount, 10000/price)
	amount = api.FormatAmount(setting.Market, symbol, math.Abs(amount))
	if amount > 0 {
		resultPerp := api.CancelOrders(``, ``, setting.Market, settingSymbol)
		resultRelated := api.CancelOrders(``, ``, setting.Market, symbolRelated)
		util.Notice(fmt.Sprintf(`cancel all perp:%v related:%v >>>>>> equal %s %f, %s %f = %s %f`,
			resultPerp, resultRelated, settingSymbol, amountPerp, symbolRelated, amountRelated, orderSide, amount))
		api.PlaceOrder(``, ``, orderSide, model.OrderTypeLimit, setting.Market, symbol, ``,
			``, ``, ``, model.FunctionComplement, price, price, amount, true)
	}
	return
}

func calcCarryOpen(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, symbolHigh, symbolLow string,
	scoreOpen, scoreClose, scoreHigh, scoreLow float64) (sidePerp, sideRelated string, amount float64) {
	var bidAmount, askAmount float64
	if (scoreLow < setting.CloseShortMargin && setting.Symbol == symbolLow) ||
		(setting.GridAmount > 0 && scoreClose <= -1*setting.GridPriceDistance) {
		bidAmount = tickPerp.Asks[0].Amount
		askAmount = tickRelated.Bids[0].Amount
		sidePerp = model.OrderSideBuy
		sideRelated = model.OrderSideSell
	} else if (scoreHigh > setting.OpenShortMargin && setting.Symbol == symbolHigh) ||
		(setting.GridAmount < 0 && scoreOpen >= setting.GridPriceDistance) {
		bidAmount = tickRelated.Asks[0].Amount
		askAmount = tickPerp.Bids[0].Amount
		sidePerp = model.OrderSideSell
		sideRelated = model.OrderSideBuy
	}
	if (setting.Symbol == symbolLow && scoreLow < setting.CloseShortMargin) ||
		(setting.Symbol == symbolHigh && scoreHigh > setting.OpenShortMargin) {
		amount = math.Min(usdAvailable/tickPerp.Asks[0].Price, math.Min(bidAmount, askAmount))
	} else {
		amount = math.Min(math.Abs(setting.GridAmount), math.Min(bidAmount, askAmount))
		if sideRelated == model.OrderSideBuy {
			amount = math.Min(amount, usdAvailable/tickRelated.Asks[0].Price)
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
	if (sideRelated == model.OrderSideBuy && usdAvailable < line) ||
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
	balance := getCarryBalance(coin)
	if balance == nil || math.Abs(balance.UsdValue) > UsdUpLine ||
		(sideRelated == model.OrderSideSell && amount > balance.Free) {
		amount = 0
	}
	if amount > 0 {
		util.Notice(fmt.Sprintf(`>>>> high:%s %f low:%s %f symbl: %s usd available:%f amount：%f`,
			symbolHigh, scoreHigh, symbolLow, scoreLow, setting.Symbol, usdAvailable, amount))
	}
	return sidePerp, sideRelated, amount
}

//var carryScoreOpen = make(map[string]float64)
//var carryScoreClose = make(map[string]float64)
//func setScore(symbol string, open, close float64) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	carryScoreOpen[symbol] = open
//	carryScoreClose[symbol] = close
//}
//func rankCarryScore(market string, amountLimit float64) (symbolHigh, symbolLow string, scoreHigh, scoreLow float64) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	scoreMsg := "\n[score list]\n"
//	i := 0
//	for symbol, valueOpen := range carryScoreOpen {
//		i++
//		parts := strings.Split(symbol, `-`)
//		if len(parts) != 2 {
//			continue
//		}
//		coin := parts[0]
//		related := parts[0] + `/USD`
//		_, bidAskPerp := model.AppMarkets.GetBidAsk(symbol, market)
//		_, bidAskRelated := model.AppMarkets.GetBidAsk(related, market)
//		if carryBalance[coin] == nil {
//			scoreMsg += fmt.Sprintf("没有查到账户币种信息：%s \n", coin)
//		}
//		if bidAskPerp == nil || bidAskRelated == nil || carryBalance[coin] == nil {
//			continue
//		}
//		if valueOpen > scoreHigh && bidAskPerp.Bids[0].Price*bidAskPerp.Bids[0].Amount > amountLimit &&
//			bidAskRelated.Asks[0].Price*bidAskRelated.Asks[0].Amount > amountLimit &&
//			carryBalance[coin].UsdValue < UsdUpLine {
//			symbolHigh = symbol
//			scoreHigh = valueOpen
//		}
//		valueClose := carryScoreClose[symbol]
//		if valueClose < scoreLow && bidAskRelated.Bids[0].Price*bidAskRelated.Bids[0].Amount > amountLimit &&
//			bidAskPerp.Asks[0].Price*bidAskPerp.Asks[0].Amount > amountLimit &&
//			carryBalance[coin].UsdValue > -1*UsdUpLine &&
//			bidAskRelated.Bids[0].Amount < carryBalance[coin].Free {
//			symbolLow = symbol
//			scoreLow = valueClose
//		}
//		scoreMsg += fmt.Sprintf("[%d/%d %s] open-close [%f ~ %f] amount limit:%f %s in usd:%f free: [%f]\n",
//			i, len(carryScoreOpen), symbol, valueOpen, valueClose, amountLimit, coin, carryBalance[coin].UsdValue,
//			carryBalance[coin].Free)
//	}
//	model.SetCarryInfo(`[grid]`, scoreMsg)
//	return
//}

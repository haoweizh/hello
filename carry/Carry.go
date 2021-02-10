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

const FTXHighOpen = 0.004
const FTXLowOpen = -0.006
const OrderPriceLimit = 0.002

//const FtxTakerFee = 0.0004275

var carryLock sync.Mutex
var carrying bool
var holding, usdAvailable float64
var doCarry = false
var carryScoreOpen = make(map[string]float64)
var carryScoreClose = make(map[string]float64)
var coinUsdValue = make(map[string]float64)
var coinBorrowAble = make(map[string]float64)

var stChan = make(chan *model.Order, 2)

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
			}
		}
		util.Notice(`...... enter clearing carry balance`)
		time.Sleep(time.Second)
		markets := model.GetMarkets()
		for _, market := range markets {
			balances := api.GetBalance(``, ``, market, 0)
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
				usdSymbol := strings.ToUpper(value.Coin) + `/USD`
				coin := strings.ToLower(value.Coin)
				coinUsdValue[coin] = value.UsdValue
				util.Notice(fmt.Sprintf(`set usd symbol %s balance %f `, usdSymbol, value.Amount))
				if coin == `usd` {
					usdAvailable = value.Amount
				} else if coin == `btc` || coin == `usdt` || coin == `ftt` {
					valueUsd += value.UsdValue
				} else if symbols[usdSymbol] {
					coinBorrowAble[usdSymbol] = api.GetMarketInfo(market, usdSymbol)
					util.Notice(fmt.Sprintf(`%s borrowable %f`, usdSymbol, coinBorrowAble[usdSymbol]))
					holding += math.Abs(value.UsdValue)
				}
			}
			if model.AppConfig.Env == `simon` {
				usdAvailable = (usdAvailable - holding) / 2
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
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	symbolRelated := setting.GetRelatedSymbol()
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	now := util.GetNowUnixMillion()
	if tickPerp == nil || tickRelated == nil || tickPerp.Asks == nil || tickPerp.Bids == nil ||
		tickRelated.Asks == nil || tickRelated.Bids == nil || model.AppConfig.Handle != `1` ||
		model.AppPause || now-int64(tickRelated.Ts) > 1000 || now-int64(tickPerp.Ts) > 1000 || setting == nil {
		return
	}
	if !checkSetCarrying(true) {
		defer checkSetCarrying(false)
	} else {
		return
	}
	if !doCarry {
		go clearCarryBalance()
		doCarry = true
	}
	scoreOpen := 1 - tickRelated.Asks[0].Price/tickPerp.Bids[0].Price
	scoreClose := 1 - tickRelated.Bids[0].Price/tickPerp.Asks[0].Price
	if setting.OpenShortMargin <= 0 {
		setting.OpenShortMargin = FTXHighOpen
	}
	if setting.CloseShortMargin >= 0 {
		setting.CloseShortMargin = FTXLowOpen
	}
	carryScoreOpen[setting.Symbol] = scoreOpen
	carryScoreClose[setting.Symbol] = scoreClose
	var scoreHigh, scoreLow float64
	var symbolHigh, symbolLow string
	scoreMsg := "\n[score list]\n"
	for symbol, valueOpen := range carryScoreOpen {
		_, bidAskPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
		_, bidAskRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
		if bidAskPerp == nil || bidAskRelated == nil {
			continue
		}
		coinUsd := 0.0
		if strings.Contains(symbolRelated, `/`) {
			coinUsd = coinUsdValue[strings.ToLower(strings.Split(symbolRelated, `/`)[0])]
			util.Notice(fmt.Sprintf(`coin value %f`, coinUsd))
		}
		if valueOpen > scoreHigh && bidAskPerp.Bids[0].Price*bidAskPerp.Bids[0].Amount > setting.AmountLimit &&
			bidAskRelated.Asks[0].Price*bidAskRelated.Asks[0].Amount > setting.AmountLimit && coinUsd < 150000 {
			symbolHigh = symbol
			scoreHigh = valueOpen
		}
		valueClose := carryScoreClose[symbol]
		if valueClose < scoreLow && bidAskRelated.Bids[0].Price*bidAskRelated.Bids[0].Amount > setting.AmountLimit &&
			bidAskPerp.Asks[0].Price*bidAskPerp.Asks[0].Amount > setting.AmountLimit && coinUsd > -150000 &&
			bidAskRelated.Bids[0].Amount < coinBorrowAble[symbolRelated] {
			symbolLow = symbol
			scoreLow = valueClose
		}
		scoreMsg += fmt.Sprintf("[%s] open-close [%f ~ %f] amount limit:%f borrowAble: %f\n",
			symbol, valueOpen, valueClose, setting.AmountLimit, coinBorrowAble[symbolRelated])
	}
	carryInfo := fmt.Sprintf(`line :%f current: [%s] score: [%f ~ %f] revert: [%f] 
		symbol low-high: [%s %f %s %f] available usd: %f holding: %f %s`,
		model.AppConfig.Amount, setting.Symbol, setting.OpenShortMargin, setting.CloseShortMargin, setting.GridPriceDistance,
		symbolLow, scoreLow, symbolHigh, scoreHigh, usdAvailable, holding, scoreMsg)
	model.SetCarryInfo(`[grid]`, carryInfo)
	sidePerp, sideRelated, amount := calcCarryOpen(setting, tickPerp, tickRelated, symbolHigh, symbolLow, scoreOpen,
		scoreClose, scoreHigh, scoreLow)
	if amount > 0 {
		util.Notice(fmt.Sprintf(`carry%s->%s with score open:%f close:%f rate sum %f amount %f worth %f`,
			setting.Symbol, symbolRelated, scoreOpen, scoreClose, 0.0, amount, amount*tickPerp.Asks[0].Price))
		perpPrice := tickPerp.Asks[0].Price
		relatedPrice := tickRelated.Bids[0].Price
		if sidePerp == model.OrderSideSell {
			perpPrice = tickPerp.Bids[0].Price
			relatedPrice = tickRelated.Asks[0].Price
			setting.GridAmount += amount
		} else if sidePerp == model.OrderSideBuy {
			perpPrice = tickPerp.Asks[0].Price
			relatedPrice = tickRelated.Bids[0].Price
			setting.GridAmount -= amount
		}
		go api.PlaceSyncOrders(``, ``, sidePerp, model.OrderTypeMarket, setting.Market, setting.Symbol,
			``, ``, ``, ``, model.FunctionCarry, perpPrice, perpPrice,
			amount, true, stChan, 1)
		go api.PlaceSyncOrders(``, ``, sideRelated, model.OrderTypeMarket, setting.Market, symbolRelated,
			``, ``, ``, ``, model.FunctionCarry, relatedPrice, relatedPrice,
			amount, true, stChan, 1)
		for true {
			<-stChan
			<-stChan
			break
		}
		time.Sleep(time.Millisecond * 200)
		//model.AppDB.Save(&setting)
		model.AppDB.Model(&setting).Where("market= ? and symbol= ? and function= ?",
			setting.Market, setting.Symbol, setting.Function).Updates(map[string]interface{}{`grid_amunt`: setting.GridAmount})
	}
}

func getCarryAmounts(setting *model.Setting, balances []*model.Balance) (amountPerp, amountRelated float64) {
	account := model.AppAccounts.GetAccount(setting.Market, setting.Symbol)
	if account == nil || account.Currency == `` || !strings.Contains(account.Currency, `-`) {
		amountPerp = 0
	} else {
		amountPerp = account.Free
		for _, balance := range balances {
			coin := strings.ToUpper(strings.Split(account.Currency, `-`)[0])
			if strings.ToUpper(balance.Coin) == coin {
				amountRelated = balance.Amount
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
	//marketSymbols := model.GetMarketSymbols(setting.Market)
	//if float64(len(perpSnapshot)) < 0.45*float64(len(marketSymbols)) {
	//	return ``, ``, 0
	//}
	var bidAmount, askAmount float64
	if (scoreLow < setting.CloseShortMargin && setting.Symbol == symbolLow) ||
		(setting.GridAmount > 0 && scoreClose <= -1*setting.GridPriceDistance) {
		bidAmount = tickPerp.Asks[0].Amount * 0.8
		askAmount = tickRelated.Bids[0].Amount * 0.8
		sidePerp = model.OrderSideBuy
		sideRelated = model.OrderSideSell
	} else if (scoreHigh > setting.OpenShortMargin && setting.Symbol == symbolHigh) ||
		(setting.GridAmount < 0 && scoreOpen >= setting.GridPriceDistance) {
		bidAmount = tickRelated.Asks[0].Amount * 0.8
		askAmount = tickPerp.Bids[0].Amount * 0.8
		sidePerp = model.OrderSideSell
		sideRelated = model.OrderSideBuy
	}
	if (setting.Symbol == symbolLow && scoreLow < setting.CloseShortMargin) ||
		(setting.Symbol == symbolHigh && scoreHigh > setting.OpenShortMargin) {
		amount = math.Min(usdAvailable/tickPerp.Asks[0].Price, math.Min(bidAmount, askAmount))
	} else {
		amount = math.Min(math.Abs(setting.GridAmount), math.Min(bidAmount, askAmount))
	}
	line := model.AppConfig.Amount
	if line <= 0 {
		line = 100000
	}
	amount = math.Min(amount, 10000/tickPerp.Asks[0].Price)
	amountPerp := api.FormatAmount(setting.Market, setting.Symbol, amount)
	amountRelated := api.FormatAmount(setting.Market, setting.GetRelatedSymbol(), amount)
	amount = math.Min(amountPerp, amountRelated)
	if sideRelated == model.OrderSideBuy && usdAvailable < line {
		//util.Notice(fmt.Sprintf(`not enough usd to carry %s %f<%f`, setting.Symbol, usdAvailable, line))
		amount = 0
	}
	return sidePerp, sideRelated, amount
}

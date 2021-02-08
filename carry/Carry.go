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
const OrderLimitUsd = 10.0

//const FtxTakerFee = 0.0004275

var carryLock sync.Mutex
var carrying bool
var holding, usdAvailable float64
var holdingUpdateTime = util.GetNow()

var carryScoreOpen = make(map[string]float64)
var carryScoreClose = make(map[string]float64)
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

// setting.GridPriceDistance: 收回下单是要求的利润(可以为负数)
var ProcessCarry = func(setting *model.Setting) {
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	symbolRelated := setting.GetRelatedSymbol()
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	status := checkSetCarrying(true)
	defer checkSetCarrying(false)
	now := util.GetNowUnixMillion()
	if status || tickPerp == nil || tickRelated == nil || tickPerp.Asks == nil || tickPerp.Bids == nil ||
		tickRelated.Asks == nil || tickRelated.Bids == nil || model.AppConfig.Handle != `1` ||
		model.AppPause || now-int64(tickRelated.Ts) > 1000 || now-int64(tickPerp.Ts) > 1000 || setting == nil {
		return
	}
	duration, _ := time.ParseDuration(`-30s`)
	current := util.GetNow()
	if usdAvailable == 0 || holdingUpdateTime.Before(current.Add(duration)) {
		holdingUpdateTime = current
		balances := api.GetBalance(``, ``, setting.Market, 0)
		api.RefreshAccount(``, ``, setting.Market)
		settings := model.GetSettings(setting.Function, setting.Market)
		for _, items := range settings {
			for _, item := range items {
				if item.Function == model.FunctionCarry {
					makeEqual(item, balances)
				}
			}
		}
		symbols := model.GetMarketSymbols(setting.Market)
		usdAvailable = 0
		holding = 0
		for _, value := range balances {
			usdSymbol := strings.ToUpper(value.Coin) + `/USD`
			util.Notice(fmt.Sprintf(`set usd symbol %s balance %f `, usdSymbol, value.Amount))
			if strings.ToLower(value.Coin) == `usd` {
				usdAvailable = value.Amount
			} else if symbols[usdSymbol] {
				holding += math.Abs(value.UsdValue)
			}
		}
		util.Notice(fmt.Sprintf(`[carry] set holding %f usd %f`, holding, usdAvailable))
		if model.AppConfig.Env != `dk` {
			usdAvailable = usdAvailable - holding
		}
		usdAvailable /= 2
		return
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
		if valueOpen > scoreHigh {
			symbolHigh = symbol
			scoreHigh = valueOpen
		}
		valueClose := carryScoreClose[symbol]
		if valueClose < scoreLow {
			symbolLow = symbol
			scoreLow = valueClose
		}
		scoreMsg += fmt.Sprintf("[%s] open-close [%f ~ %f]\n", symbol, valueOpen, valueClose)
	}
	model.SetCarryInfo(`[grid]`,
		fmt.Sprintf(`current: [%s] score: [%f ~ %f] revert: [%f] symbol low-high: [%s %f %s %f] available usd: %f holding: %f %s`,
			setting.Symbol, setting.OpenShortMargin, setting.CloseShortMargin, setting.GridPriceDistance,
			symbolLow, scoreLow, symbolHigh, scoreHigh, usdAvailable, holding, scoreMsg))
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
		time.Sleep(time.Millisecond * 00)
		model.AppDB.Save(&setting)
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

func makeEqual(setting *model.Setting, balances []*model.Balance) (equal bool) {
	settingSymbol := setting.Symbol
	if strings.Contains(settingSymbol, `BTC`) || strings.Contains(settingSymbol, `ETH`) ||
		strings.Contains(settingSymbol, `LINK`) || strings.Contains(settingSymbol, `DOGE`) ||
		strings.Contains(settingSymbol, `FTT`) {
		return true
	}
	amountPerp, amountRelated := getCarryAmounts(setting, balances)
	amount := amountPerp + amountRelated
	symbolRelated := setting.GetRelatedSymbol()
	score := carryScoreOpen[setting.Symbol]
	orderSide := model.OrderSideBuy
	if amount > 0 {
		orderSide = model.OrderSideSell
	}
	amount = math.Abs(amount)
	orders := make([]*model.Order, 0)
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	if tickPerp == nil || tickRelated == nil {
		return true
	}
	if amount < math.Max(math.Abs(amountPerp), math.Abs(amountRelated)) {
		symbol := setting.Symbol
		if amountPerp < 0 && amountRelated > 0 {
			if amountPerp+amountRelated < 0 {
				if score > 0.005 {
					symbol = symbolRelated
				} else {
					symbol = settingSymbol
				}
			} else {
				if score > 0.005 {
					symbol = settingSymbol
				} else {
					symbol = symbolRelated
				}
			}
		} else if amountPerp > 0 && amountRelated < 0 {
			if amountPerp+amountRelated < 0 {
				if score < -0.005 {
					symbol = settingSymbol
				} else {
					symbol = symbolRelated
				}
			} else {
				if score < -0.005 {
					symbol = symbolRelated
				} else {
					symbol = settingSymbol
				}
			}
		}
		amount = api.FormatAmount(setting.Market, symbol, amount)
		price := tickPerp.Bids[0].Price
		if symbol == settingSymbol {
			price = tickPerp.Bids[0].Price
			if orderSide == model.OrderSideBuy {
				price = tickPerp.Asks[0].Price
			}
		} else {
			price = tickRelated.Bids[0].Price
			if orderSide == model.OrderSideBuy {
				price = tickRelated.Asks[0].Price
			}
		}
		if amount > 0 {
			util.Notice(fmt.Sprintf(`equal one %s %s %f %s %f %s %f`,
				symbol, orderSide, amount, setting.Market, amountPerp, symbolRelated, amountRelated))
			orders = append(orders, api.PlaceOrder(``, ``, orderSide, model.OrderTypeMarket, setting.Market,
				symbol, ``, ``, ``, ``, model.FunctionComplement, price, price,
				amount, true))
		}
	} else {
		price := tickPerp.Asks[0].Price
		if amountPerp > 0 {
			price = tickPerp.Bids[0].Price
		}
		amountPerp = api.FormatAmount(model.Ftx, setting.Symbol, math.Abs(amountPerp))
		if amountPerp > 0 {
			util.Notice(fmt.Sprintf(`equal perp %s %s %f`, setting.Market, orderSide, amount))
			orders = append(orders, api.PlaceOrder(``, ``, orderSide, model.OrderTypeMarket, setting.Market,
				setting.Symbol, ``, ``, ``, ``, model.FunctionComplement,
				price, price, amountPerp, true))
		}
		price = tickRelated.Asks[0].Price
		if amountRelated > 0 {
			price = tickRelated.Bids[0].Price
		}
		amountRelated = api.FormatAmount(model.Ftx, symbolRelated, math.Abs(amountRelated))
		if amountRelated > 0 {
			util.Notice(fmt.Sprintf(`equal related %s %s %f`, setting.Market, orderSide, amount))
			orders = append(orders, api.PlaceOrder(``, ``, orderSide, model.OrderTypeMarket, setting.Market,
				symbolRelated, ``, ``, ``, ``, model.FunctionComplement,
				price, price, amountRelated, true))
		}
	}
	allDone := false
	for i := 0; i < 30 && !allDone; i++ {
		allDone = true
		for _, order := range orders {
			if order == nil {
				continue
			}
			order = api.QueryOrderById(``, ``, order.Market, order.Symbol, order.Instrument,
				order.OrderType, order.OrderId)
			if order == nil {
				allDone = false
			} else if order.Status == model.CarryStatusWorking {
				allDone = false
				util.Notice(fmt.Sprintf(`working set all done %v %s deal %f`,
					allDone, order.Status, order.DealAmount))
				time.Sleep(time.Second * 5)
			}
		}
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
	if bidAmount == 0 || askAmount == 0 {
		return ``, ``, 0
	}
	if tickPerp.Asks[0].Price*bidAmount > OrderLimitUsd {
		bidAmount /= 2
	}
	if tickPerp.Asks[0].Price*askAmount > OrderLimitUsd {
		askAmount /= 2
	}
	if (setting.Symbol == symbolLow && scoreLow < setting.CloseShortMargin) ||
		(setting.Symbol == symbolHigh && scoreHigh > setting.OpenShortMargin) {
		amount = math.Min(usdAvailable/tickPerp.Asks[0].Price, math.Min(bidAmount, askAmount))
	} else {
		amount = math.Min(math.Abs(setting.GridAmount), math.Min(bidAmount, askAmount))
	}
	amount = math.Min(amount, 10000/tickPerp.Asks[0].Price)
	amount = api.FormatAmount(setting.Market, setting.Symbol, amount)
	return sidePerp, sideRelated, amount
}

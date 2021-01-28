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
const FTXClose = 0.001
const OrderLimitUsd = 10.0

var carryLock sync.Mutex
var carrying bool
var holding, usdAvailable float64
var holdingUpdateTime = util.GetNow()

var perpSnapshot = make(map[string]float64)
var carryBalances = make(map[string]float64)

func isCarrying() (value bool) {
	carryLock.Lock()
	defer carryLock.Unlock()
	return carrying
}

func setCarrying(value bool) {
	carryLock.Lock()
	defer carryLock.Unlock()
	carrying = value
}

//ProcessCarry
var ProcessCarry = func(setting *model.Setting) {
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	symbolRelated := setting.GetRelatedSymbol()
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	marketInfo := model.MarketInfos[setting.Market][setting.Symbol]
	marketInfoRelated := model.MarketInfos[setting.Market][symbolRelated]
	now := util.GetNowUnixMillion()
	if tickPerp == nil || tickRelated == nil || tickPerp.Asks == nil || tickPerp.Bids == nil ||
		tickRelated.Asks == nil || tickRelated.Bids == nil || model.AppConfig.Handle != `1` ||
		model.AppPause || now-int64(tickRelated.Ts) > 1000 || now-int64(tickPerp.Ts) > 1000 ||
		marketInfo == nil || marketInfoRelated == nil {
		return
	}
	if setting == nil || isCarrying() {
		return
	}
	setCarrying(true)
	defer setCarrying(false)
	rates, _ := api.GetFundingRate(setting.Market, setting.Symbol)
	rateSum := 0.0
	for _, item := range rates.([]*model.FundingRate) {
		rateSum += item.Rate
	}
	duration, _ := time.ParseDuration(`-600s`)
	current := util.GetNow()
	if usdAvailable == 0 || holdingUpdateTime.Before(current.Add(duration)) {
		holdingUpdateTime = current
		balances := api.GetBalance(``, ``, setting.Market, 0)
		symbols := model.GetMarketSymbols(setting.Market)
		usdAvailable = 0
		holding = 0
		for _, value := range balances {
			usdSymbol := strings.ToUpper(value.Coin) + `/USD`
			carryBalances[usdSymbol] = value.Amount
			util.Notice(fmt.Sprintf(`set usd symbol %s balance %f `, usdSymbol, value.Amount))
			tempMarketInfo := model.MarketInfos[setting.Market][usdSymbol]
			if tempMarketInfo != nil {
				util.Notice(fmt.Sprintf(`can borrow:%v`, tempMarketInfo.CanBorrow))
			}
			if strings.ToLower(value.Coin) == `usd` {
				usdAvailable = value.Amount
			} else if symbols[usdSymbol] {
				holding += math.Abs(value.UsdValue)
			}
		}
		usdAvailable = (usdAvailable - holding) / 2
		util.Notice(fmt.Sprintf(`[carry] set holding %f usd %f`, holding, usdAvailable))
		return
	}
	score := math.Max(1-tickRelated.Asks[0].Price/tickPerp.Bids[0].Price+rateSum, 0)
	if rateSum < 0 {
		score = math.Min(1-tickRelated.Bids[0].Price/tickPerp.Asks[0].Price+rateSum, 0)
	}
	if setting.OpenShortMargin <= 0 {
		setting.OpenShortMargin = FTXHighOpen
	}
	if setting.CloseShortMargin >= 0 {
		setting.CloseShortMargin = FTXLowOpen
	}
	checkPair := tickRelated.Bids[0].Price / tickPerp.Bids[0].Price
	if checkPair > 1.2 || checkPair < 0.8 {
		util.Info(fmt.Sprintf(`fatal %s %s %f %f`,
			setting.Symbol, symbolRelated, tickPerp.Bids[0].Price, tickRelated.Bids[0].Price))
		//return
	}
	perpSnapshot[setting.Symbol] = score
	var scoreHigh, scoreLow float64
	var symbolHigh, symbolLow string
	scoreMsg := "[score list]\n"
	for symbol, value := range perpSnapshot {
		if value > scoreHigh {
			symbolHigh = symbol
			scoreHigh = value
		} else if value < scoreLow {
			symbolLow = symbol
			scoreLow = value
		}
		scoreMsg += fmt.Sprintf("%s %f\n", symbol, value)
	}
	model.SetCarryInfo(`[grid]`,
		fmt.Sprintf(`symbol low: %s %f high: %s %f symbols: %d available usd: %f holding: %f %s`,
			symbolLow, scoreLow, symbolHigh, scoreHigh, len(perpSnapshot), usdAvailable, holding, scoreMsg))
	sidePerp, sideRelated, amount := calcCarryOpen(setting, marketInfo, marketInfoRelated, tickPerp,
		tickRelated, symbolHigh, symbolLow, score, scoreHigh, scoreLow)
	if amount > 0 {
		util.Notice(fmt.Sprintf(`carry between %s %s with score %f rate sum %f amount %f worth %f`,
			setting.Symbol, symbolRelated, score, rateSum, amount, amount*tickPerp.Asks[0].Price))
		perpPrice := tickPerp.Asks[0].Price
		relatedPrice := tickRelated.Bids[0].Price
		if sidePerp == model.OrderSideSell {
			setting.GridAmount += amount
			perpPrice = tickPerp.Bids[0].Price
			relatedPrice = tickRelated.Asks[0].Price
		} else if sidePerp == model.OrderSideBuy {
			setting.GridAmount -= amount
			perpPrice = tickPerp.Asks[0].Price
			relatedPrice = tickRelated.Bids[0].Price
		}
		perpAmount := amount
		relatedAmount := amount
		if amount > carryBalances[symbolRelated] && !marketInfoRelated.CanBorrow {
			relatedAmount = carryBalances[symbolRelated]
			util.Notice(fmt.Sprintf(`adjust usd symbol %s sell amount %f -> %f`, symbolRelated, amount, relatedAmount))
		}
		go api.PlaceOrder(``, ``, sidePerp, model.OrderTypeMarket, setting.Market, setting.Symbol, ``,
			``, ``, ``, model.FunctionCarry, perpPrice, perpPrice, perpAmount, true)
		go api.PlaceOrder(``, ``, sideRelated, model.OrderTypeMarket, setting.Market, symbolRelated, ``,
			``, ``, ``, model.FunctionCarry, relatedPrice, relatedPrice, amount, true)
		model.AppDB.Save(&setting)
		holding = 0
		usdAvailable = 0
		time.Sleep(time.Second * 30)
	}
}

func calcCarryOpen(setting *model.Setting, marketInfo, marketInfoRelated *model.MarketInfo, tickPerp, tickRelated *model.BidAsk,
	symbolHigh, symbolLow string, score, scoreHigh, scoreLow float64) (sidePerp, sideRelated string, amount float64) {
	//marketSymbols := model.GetMarketSymbols(setting.Market)
	//if float64(len(perpSnapshot)) < 0.45*float64(len(marketSymbols)) {
	//	return ``, ``, 0
	//}
	var bidAmount, askAmount float64
	if (scoreLow < setting.CloseShortMargin && setting.Symbol == symbolLow) || (setting.GridAmount > 0 && score < -1*FTXClose) {
		bidAmount = tickPerp.Asks[0].Amount
		askAmount = tickRelated.Bids[0].Amount
		sidePerp = model.OrderSideBuy
		sideRelated = model.OrderSideSell
	} else if (scoreHigh > setting.OpenShortMargin && setting.Symbol == symbolHigh) || (setting.GridAmount < 0 && score > FTXClose) {
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
	if setting.Symbol == symbolLow || setting.Symbol == symbolHigh {
		amount = math.Min(usdAvailable/tickPerp.Asks[0].Price, math.Min(bidAmount, askAmount))
	} else {
		amount = math.Min(math.Abs(setting.GridAmount), math.Min(bidAmount, askAmount))
	}
	amount = math.Floor(amount/marketInfo.SizeIncrement) * marketInfo.SizeIncrement
	if amount < marketInfo.SizeIncrement || amount < marketInfoRelated.SizeIncrement {
		util.Notice(fmt.Sprintf(`size not enough order size %f < %s size %f or %s size %f %s:%f %s:%f`,
			amount, marketInfo.Name, marketInfo.SizeIncrement, marketInfoRelated.Name,
			marketInfoRelated.SizeIncrement, symbolLow, scoreLow, symbolHigh, scoreHigh))
		time.Sleep(time.Second * 30)
		return ``, ``, 0
	}
	return sidePerp, sideRelated, amount
}

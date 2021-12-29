package model

import (
	"fmt"
	"hello/util"
	"math"
	"strconv"
	"sync"
)

var marketInfos = make(map[string]map[string]*MarketInfo) // market - symbol - MarketInfo
var marketInfoLock sync.Mutex
var tradeMax = make(map[string]map[string][]float64) // key - instrument - [maxBuy合约张数/币币个数, maxSell]

type MarketInfo struct {
	Market, Name, CTCurrency                        string
	SizeMin, SizeIncrement, PriceIncrement, CTValue float64
	PriceDecimal                                    int     // 价格精确到小数点后几位
	UsdtMin                                         float64 //最小下单金额需达到的usdt值
	BorrowSizeMin                                   float64 //最小借款数量
	BorrowUsdtMax                                   float64 //最大借款usdt数额
	SizeMax                                         float64 //最大下单数量
	PriceMax                                        float64 //最大下单价格
}

func GetTradeMax(key, instrument string) (maxBuy, maxSell float64) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	if tradeMax[key] == nil || tradeMax[key][instrument] == nil || len(tradeMax[key][instrument]) != 2 {
		return 0, 0
	}
	return tradeMax[key][instrument][0], tradeMax[key][instrument][1]
}

func SetTradeMax(key, instrument string, maxBuy, maxSell float64) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	if tradeMax[key] == nil {
		tradeMax[key] = make(map[string][]float64)
	}
	tradeMax[key][instrument] = []float64{maxBuy, maxSell}
}

//func GetAmountWithinLimit(market, symbol string, amount float64) (within float64) {
//	switch market {
//	case OKEX:
//		amountInPerp := model.GetAmountInMarket(setting.Market, setting.Symbol, amount)
//		maxBuyPerp, maxSellPerp := getTradeMax(key, setting.Symbol)
//		maxBuyRelated, maxSellRelated := getTradeMax(key, setting.SymbolRelated)
//		maxBuyRelated += balance.Borrow
//		maxSellRelated = math.Max(maxSellRelated, balance.AvailableWithBorrow)
//		if sidePerp == model.OrderSideBuy && sideRelated == model.OrderSideSell {
//			amountInPerp = math.Min(amountInPerp, maxBuyPerp)
//			amount = math.Min(amount, maxSellRelated)
//		} else if sidePerp == model.OrderSideSell && sideRelated == model.OrderSideBuy {
//			amountInPerp = math.Min(amountInPerp, maxSellPerp)
//			amount = math.Min(amount, maxBuyRelated)
//		}
//		_, amountInReal := model.ParseRealAmount(setting.Market, setting.Symbol, amountInPerp)
//		amount = math.Min(amount, amountInReal)
//		amount = FormatAmountPair(setting.Market, setting.Symbol, setting.SymbolRelated, amount)
//	case Gate:
//		marketPerp := model.GetMarketInfo(setting.Market, setting.Symbol)
//		_, amountInReal := model.ParseRealAmount(setting.Market, setting.Symbol, marketPerp.SizeMax)
//		amount = math.Min(amount, amountInReal)
//		if !model.AppConfig.GateSpot && (scoreClose < setClose || scoreOpen > setOpen) && sideRelated == model.OrderSideSell {
//			//开仓且卖现货时，最小单笔可借数量限制。有持仓的，需要卖出所有持仓数额再加上最小可借
//			marketRelated := model.GetMarketInfo(setting.Market, setting.SymbolRelated)
//			if balance.Amount > 0 && amount < balance.Amount+marketRelated.BorrowSizeMin {
//				amount = math.Min(amount, balance.Amount)
//			} else if balance.Amount < 0 {
//				amount = math.Max(amount, marketRelated.BorrowSizeMin)
//			}
//		}
//	case Ftx:
//		within = math.Min(amount, 90000000)
//	}
//	return within
//}

func GetMarketRealAmount(market, instrument string) (success bool, sizeInc, sizeMin float64) {
	marketInfo := GetMarketInfo(market, instrument)
	if marketInfo == nil {
		return
	}
	if marketInfo.CTValue > 0 && marketInfo.CTCurrency == GetCoin(market, instrument) {
		return success, marketInfo.SizeIncrement * marketInfo.CTValue, marketInfo.SizeMin * marketInfo.CTValue
	}
	return success, marketInfo.SizeIncrement, marketInfo.SizeMin
}

func GetMarketInfo(market, instrument string) (marketInfo *MarketInfo) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	if marketInfos == nil || marketInfos[market] == nil {
		return nil
	}
	return marketInfos[market][instrument]
}

func SetMarketInfos(market string, value map[string]*MarketInfo) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	marketInfos[market] = value
}

func GetMarketInfos(market string) (value map[string]*MarketInfo) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	return marketInfos[market]
}

func GetCarryCoins() (coins map[string]map[string]bool) { //  market - coin - bool
	markets := GetMarkets()
	coins = make(map[string]map[string]bool)
	for _, market := range markets {
		coins[market] = make(map[string]bool)
		if marketInfos != nil && GetSettings(FunctionCarry, market) != nil {
			symbols := GetMarketSymbols(market)
			for symbol := range symbols {
				coin := GetCoin(market, symbol)
				coins[market][coin] = true
			}
		}
	}
	return coins
}

// ParseRealAmount 返回以币为单位的数量
func ParseRealAmount(market, symbol string, amount float64) (success bool, realAmount float64) {
	marketInfo := GetMarketInfo(market, symbol)
	if marketInfo == nil || marketInfo.SizeIncrement == 0 || marketInfo.CTCurrency != GetCoin(market, symbol) {
		return false, 0
	}
	if marketInfo.CTValue == 0 {
		return true, amount
	}
	return true, amount * marketInfo.CTValue
}

// GetAmountInMarket 返回交易所认可的下单数量，可能是币数、张数等
// amount: 搬砖程序中使用的币数量
func GetAmountInMarket(market, symbol string, amount float64) (formattedAmount float64) {
	marketInfo := GetMarketInfo(market, symbol)
	if marketInfo == nil || marketInfo.SizeIncrement == 0 || marketInfo.SizeMin == 0 {
		return 0
	}
	if marketInfo.CTValue > 0 && marketInfo.CTCurrency == GetCoin(market, symbol) {
		amount = amount / marketInfo.CTValue
	}
	formattedAmount = marketInfo.SizeIncrement * math.Floor(amount/marketInfo.SizeIncrement)
	decimal := util.NumDecPlaces(marketInfo.SizeIncrement)
	format := `%.` + strconv.Itoa(decimal) + `f`
	formattedAmount, _ = strconv.ParseFloat(fmt.Sprintf(format, formattedAmount), 64)
	if formattedAmount < marketInfo.SizeMin || marketInfo.SizeMin == 0 {
		return 0
	}
	return formattedAmount
}

func FormatPrice(market, symbol, orderSide string, price float64) (formattedPrice float64, decimal int) {
	marketInfo := GetMarketInfo(market, symbol)
	if marketInfo == nil || marketInfo.SizeIncrement == 0 {
		return 0, 0
	}
	if orderSide == OrderSideBuy {
		return marketInfo.PriceIncrement * math.Ceil(price/marketInfo.PriceIncrement), marketInfo.PriceDecimal
	} else {
		return marketInfo.PriceIncrement * math.Floor(price/marketInfo.PriceIncrement), marketInfo.PriceDecimal
	}
}

// FormatAmountPair symbol 期货; related 现货
func FormatAmountPair(market, symbolPerp, symbolRelated string, amount float64) (formattedAmount float64) {
	marketPerp := GetMarketInfo(market, symbolPerp)
	marketRelated := GetMarketInfo(market, symbolRelated)
	if marketPerp == nil || marketPerp.SizeIncrement == 0 || marketPerp.SizeMin == 0 ||
		marketRelated == nil || marketRelated.SizeIncrement == 0 || marketRelated.SizeMin == 0 {
		return 0
	}
	sizeInc := marketPerp.SizeIncrement
	sizeMinPerp := marketPerp.SizeMin
	if marketPerp.CTValue > 0 && marketPerp.CTCurrency == GetCoin(market, symbolPerp) {
		sizeInc = sizeInc * marketPerp.CTValue
		sizeMinPerp = sizeMinPerp * marketPerp.CTValue
	}
	if sizeInc < marketRelated.SizeIncrement {
		sizeInc = marketRelated.SizeIncrement
	}
	formattedAmount = math.Floor(amount/sizeInc) * sizeInc
	if formattedAmount < sizeMinPerp || formattedAmount < marketRelated.SizeMin {
		return 0
	}
	return formattedAmount
}

// FormatCrossPair symbol 期货; related 现货
func FormatCrossPair(marketBuy, marketSell, symbolBuy, symbolSell string, amount float64) (formattedAmount float64) {
	successBuy, incBuy, minBuy := GetMarketRealAmount(marketBuy, symbolBuy)
	successSell, incSell, minSell := GetMarketRealAmount(marketSell, symbolSell)
	if !successBuy || !successSell {
		return 0
	}
	sizeInc := math.Max(incBuy, incSell)
	formattedAmount = math.Floor(amount/sizeInc) * sizeInc
	if formattedAmount < minBuy || formattedAmount < minSell {
		return 0
	}
	return formattedAmount
}

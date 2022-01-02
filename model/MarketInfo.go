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

type MarketInfo struct {
	Market, Name, CTCurrency               string
	SizeIncrement, PriceIncrement, CTValue float64
	PriceDecimal                           int     // 价格精确到小数点后几位
	UsdtMin                                float64 //最小下单金额需达到的usdt值
	BorrowSizeMin                          float64 //最小借款数量
	BorrowUsdtMax                          float64 //最大借款usdt数额
	SizeMax, SizeMin                       float64 //最大最小下单数量，当CTValue=0（现货）时为交易币种数量，CTValue>0(永续)为张数，在使用时乘以CTValue转换成币数
	PriceMax                               float64 //最大下单价格
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
	if marketInfo == nil || marketInfo.SizeIncrement == 0 {
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

// FormatCrossPair 不支持以BTC或ETH计价的交易对，只支持USD类
func FormatCrossPair(marketBuy, marketSell, symbolBuy, symbolSell string, amount, price float64) (
	formattedAmount float64) {
	marketInfoBuy := GetMarketInfo(marketBuy, symbolBuy)
	marketInfoSell := GetMarketInfo(marketSell, symbolSell)
	if marketInfoBuy == nil || marketInfoSell == nil {
		return
	}
	incBuy := marketInfoBuy.SizeIncrement
	incSell := marketInfoSell.SizeIncrement
	minBuy := marketInfoBuy.SizeMin
	minSell := marketInfoSell.SizeMin
	if marketInfoBuy.CTCurrency == GetCoin(marketBuy, symbolBuy) && marketInfoBuy.CTValue > 0 {
		incBuy, minBuy = incBuy*marketInfoBuy.CTValue, minBuy*marketInfoBuy.CTValue
	}
	if marketInfoSell.CTCurrency == GetCoin(marketSell, symbolSell) && marketInfoSell.CTValue > 0 {
		incSell, minSell = incSell*marketInfoSell.CTValue, minSell*marketInfoSell.CTValue
	}
	sizeInc := math.Max(incBuy, incSell)
	formattedAmount = math.Floor(amount/sizeInc) * sizeInc
	if formattedAmount < math.Max(minBuy, minSell) {
		return 0
	}
	if (marketInfoBuy.UsdtMin > 0 && formattedAmount*price < marketInfoBuy.UsdtMin) ||
		(marketInfoSell.UsdtMin > 0 && formattedAmount*price < marketInfoSell.UsdtMin) {
		return 0
	}
	return formattedAmount
}

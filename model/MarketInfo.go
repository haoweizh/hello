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
	CanBorrow                              bool
	SizeIncrement, PriceIncrement, CTValue float64
	PriceDecimal                           int     // 价格精确到小数点后几位
	MoneyMin                               float64 //最小下单金额需达到的计费货币数值
	BorrowSizeMin                          float64 //最小借款数量
	BorrowUsdtMax                          float64 //最大借款usdt数额
	SizeMax, SizeMin                       float64 //最大最小下单数量，当CTValue=0（现货）时为交易币种数量，CTValue>0(永续)为张数，在使用时乘以CTValue转换成币数
}

func GetMarketInfo(market, symbol string) (marketInfo *MarketInfo) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	if marketInfos == nil || marketInfos[market] == nil {
		return nil
	}
	return marketInfos[market][symbol]
}

func SetMarketInfos(market string, value map[string]*MarketInfo) {
	defer marketInfoLock.Unlock()
	marketInfoLock.Lock()
	marketInfos[market] = value
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
func GetAmountInMarket(market string, symbol string, amount, price float64) (formattedAmount float64) {
	marketInfo := GetMarketInfo(market, symbol)
	if marketInfo == nil || marketInfo.SizeIncrement == 0 || marketInfo.SizeMin == 0 {
		return 0
	}
	success, _, coin, _ := GetFromStandard(market, symbol)
	if success && marketInfo.CTValue > 0 && marketInfo.CTCurrency == coin {
		amount = amount / marketInfo.CTValue
	}
	formattedAmount = marketInfo.SizeIncrement * math.Floor(amount/marketInfo.SizeIncrement)
	decimal := util.NumDecPlaces(marketInfo.SizeIncrement)
	format := `%.` + strconv.Itoa(decimal) + `f`
	formattedAmount, _ = strconv.ParseFloat(fmt.Sprintf(format, formattedAmount), 64)
	if formattedAmount < marketInfo.SizeMin || marketInfo.SizeMin == 0 ||
		(marketInfo.MoneyMin > 0 && marketInfo.MoneyMin > price*formattedAmount) {
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

// FormatCrossPair 不支持以BTC或ETH计价的交易对，只支持USD类
func FormatCrossPair(marketBuy, marketSell, symbolBuy, symbolSell string, amount, price float64) (
	formattedAmount float64) {
	marketInfoBuy := GetMarketInfo(marketBuy, symbolBuy)
	marketInfoSell := GetMarketInfo(marketSell, symbolSell)
	if marketInfoBuy == nil || marketInfoSell == nil {
		util.Notice(`format %s %s %s %s %v %v`, marketBuy, marketSell, symbolBuy, symbolSell, marketInfoBuy, marketInfoSell)
		return
	}
	incBuy := marketInfoBuy.SizeIncrement
	incSell := marketInfoSell.SizeIncrement
	minBuy := marketInfoBuy.SizeMin
	minSell := marketInfoSell.SizeMin
	success, _, coin, _ := GetFromStandard(marketBuy, symbolBuy)
	if success && marketInfoBuy.CTCurrency == coin && marketInfoBuy.CTValue > 0 {
		incBuy, minBuy = incBuy*marketInfoBuy.CTValue, minBuy*marketInfoBuy.CTValue
	}
	success, _, coin, _ = GetFromStandard(marketSell, symbolSell)
	if success && marketInfoSell.CTCurrency == coin && marketInfoSell.CTValue > 0 {
		incSell, minSell = incSell*marketInfoSell.CTValue, minSell*marketInfoSell.CTValue
	}
	sizeInc := math.Max(incBuy, incSell)
	formattedAmount = math.Floor(amount/sizeInc) * sizeInc
	if marketInfoBuy.Market == OKEX {
		util.Notice(fmt.Sprintf(`format cross pair %s %s %f %f from %f to %f`,
			marketInfoBuy.Market, marketInfoBuy.Name, incBuy, minBuy, amount, formattedAmount))
	} else if marketInfoSell.Market == OKEX {
		util.Notice(fmt.Sprintf(`format cross pair %s %s %f %f from %f to %f`,
			marketInfoSell.Market, marketInfoSell.Name, incSell, minSell, amount, formattedAmount))
	}
	if formattedAmount < math.Max(minBuy, minSell) {
		return 0
	}
	if (marketInfoBuy.MoneyMin > 0 && formattedAmount*price < marketInfoBuy.MoneyMin) ||
		(marketInfoSell.MoneyMin > 0 && formattedAmount*price < marketInfoSell.MoneyMin) {
		return 0
	}
	return formattedAmount
}

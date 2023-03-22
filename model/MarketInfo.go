package model

import (
	"fmt"
	"hello/util"
	"math"
	"strconv"
	"sync"
)

var MarketInfos *sync.Map // market - symbol - *MarketInfo

var CommonCoins = map[string]bool{`btc`: true, `eth`: true, `ltc`: true, `bch`: true, `eos`: true, `xrp`: true,
	`usdt`: true, `etc`: true, `link`: true}

type MarketInfo struct {
	Market, Name, CTCurrency                string
	CanBorrow                               bool
	SizeIncrement, PriceIncrement, CTValue  float64
	PriceDecimal                            int     // 价格精确到小数点后几位
	TradeAmount                             float64 // 过去24小时以usd记交易额
	MoneyMin                                float64 //最小下单金额需达到的计费货币数值
	BorrowSizeMin                           float64 //最小借款数量
	BorrowUsdtMax                           float64 //最大借款usdt数额
	SizeMax, SizeMin                        float64 //最大最小下单数量，当CTValue=0（现货）时为交易币种数量，CTValue>0(永续)为张数，在使用时乘以CTValue转换成币数
	BuyLimitPriceRatio, SellLimitPriceRatio float64 //买卖价限价比例
}

type MarketInfoArray []*MarketInfo

func (marketInfoArray MarketInfoArray) Len() int {
	return len(marketInfoArray)
}

func (marketInfoArray MarketInfoArray) Swap(i, j int) {
	marketInfoArray[i], marketInfoArray[j] = marketInfoArray[j], marketInfoArray[i]
}

func (marketInfoArray MarketInfoArray) Less(i, j int) bool {
	return marketInfoArray[i].TradeAmount < marketInfoArray[j].TradeAmount
}

// ParseRealAmount 返回以币为单位的数量
func ParseRealAmount(market, symbol string, amount float64) (success bool, realAmount float64) {
	v, _ := util.LoadSyncMap(MarketInfos, market, symbol)
	if v == nil || v.(*MarketInfo).SizeIncrement == 0 {
		return false, 0
	}
	if v.(*MarketInfo).CTValue == 0 {
		return true, amount
	}
	return true, amount * v.(*MarketInfo).CTValue
}

// GetAmountInMarket 返回交易所认可的下单数量，可能是币数、张数等
// amount: 搬砖程序中使用的币数量
func GetAmountInMarket(market string, symbol string, amount, price float64) (formattedAmount float64) {
	v, _ := util.LoadSyncMap(MarketInfos, market, symbol)
	if v == nil || v.(*MarketInfo).SizeIncrement == 0 || v.(*MarketInfo).SizeMin == 0 {
		return 0
	}
	success, _, coin, _ := GetFromStandard(market, symbol)
	if success && v.(*MarketInfo).CTValue > 0 && v.(*MarketInfo).CTCurrency == coin {
		amount = amount / v.(*MarketInfo).CTValue
	}
	formattedAmount = v.(*MarketInfo).SizeIncrement * math.Floor(amount/v.(*MarketInfo).SizeIncrement)
	decimal := util.NumDecPlaces(v.(*MarketInfo).SizeIncrement)
	format := `%.` + strconv.Itoa(decimal) + `f`
	formattedAmount, _ = strconv.ParseFloat(fmt.Sprintf(format, formattedAmount), 64)
	if formattedAmount < v.(*MarketInfo).SizeMin || v.(*MarketInfo).SizeMin == 0 ||
		(v.(*MarketInfo).MoneyMin > 0 && v.(*MarketInfo).MoneyMin > price*formattedAmount) {
		return 0
	}
	return formattedAmount
}

func FormatPrice(market, symbol string, price float64) (formattedPrice float64, decimal int) {
	v, _ := util.LoadSyncMap(MarketInfos, market, symbol)
	if v == nil || v.(*MarketInfo).SizeIncrement == 0 {
		return 0, 0
	}
	return v.(*MarketInfo).PriceIncrement * math.Round(price/v.(*MarketInfo).PriceIncrement), v.(*MarketInfo).PriceDecimal
	//if orderSide == OrderSideBuy {
	//	return marketInfo.PriceIncrement * math.Ceil(price/marketInfo.PriceIncrement), marketInfo.PriceDecimal
	//} else {
	//	return marketInfo.PriceIncrement * math.Floor(price/marketInfo.PriceIncrement), marketInfo.PriceDecimal
	//}
}

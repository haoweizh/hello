package model

import (
	"fmt"
	"hello/util"
	"math"
	"strconv"
	"sync"
)

var MarketInfos = &sync.Map{} // market - symbol - *MarketInfo

var CommonCoins = map[string]bool{`btc`: true, `eth`: true, `ltc`: true, `bch`: true, `eos`: true, `xrp`: true,
	`usdt`: true, `etc`: true, `link`: true, `trx`: true, `bnb`: true}

var CommonTurtleSymbols = map[string]bool{`btc_perp`: true, `BTC_PERP`: true, `ETH_PERP`: true, `eth_perp`: true,
	`LTC_PERP`: true, `ltc_perp`: true, `BCH_PERP`: true, `bch_perp`: true, `EOS_PERP`: true, `eos_perp`: true, `XRP_PERP`: true,
	`xrp_perp`: true, `USDT_PERP`: true, `usdt_perp`: true, `ETC_PERP`: true, `etc_perp`: true, `LINK_PERP`: true, `link_perp`: true,
	`TRX_PERP`: true, `trx_perp`: true, `BNB_PERP`: true, `bnb_perp`: true, `UNI_PERP`: true, `uni_perp`: true}

var NoTurtleCoins = map[string]bool{`yfii`: true}

type MarketInfo struct {
	Market, Name, CTCurrency                string
	CanBorrow                               bool
	SizeIncrement, PriceIncrement, CTValue  float64
	PriceDecimal                            int     // 价格精确到小数点后几位
	PriceMax                                float64 //最高单价
	TradeAmount                             float64 // 过去24小时以usd记交易额
	MoneyMin                                float64 //最小下单金额需达到的计费货币数值
	QuoteMax                                float64 //最大下单金额的计费货币数值
	BorrowSizeMin                           float64 //最小借款数量
	BorrowUsdtMax                           float64 //最大借款usdt数额
	SizeMax, SizeMin, SizeMaxMarket         float64 //最大最小下单数量，当CTValue=0（现货）时为交易币种数量，CTValue>0(永续)为张数，在使用时乘以CTValue转换成币数
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

// GetMarketInfos
func _(market, marketType string) (marketInfos map[string]*MarketInfo) {
	marketInfos = make(map[string]*MarketInfo)
	MarketInfos.Range(func(key, value any) bool {
		if value == nil {
			return true
		}
		if value.(*MarketInfo).Market == market {
			symbol := value.(*MarketInfo).Name
			_, mt, _ := GetCoinFromDialect(market, symbol)
			if mt == marketType {
				marketInfos[symbol] = value.(*MarketInfo)
			}
		}
		return true
	})
	return
}

func GetMarketInfo(market, symbol string) (marketInfo *MarketInfo) {
	v, _ := util.LoadSyncMap(MarketInfos, market, symbol)
	if v == nil {
		return nil
	}
	return v.(*MarketInfo)
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
func GetAmountInMarket(market string, symbol string, amount, price float64, reduceOnly bool) (formattedAmount float64) {
	marketInfo := GetMarketInfo(market, symbol)
	if marketInfo == nil || marketInfo.SizeIncrement == 0 || marketInfo.SizeMin == 0 {
		return 0
	}
	success, _, coin, _ := GetFromStandard(market, symbol)
	if success && marketInfo.CTValue > 0 && marketInfo.CTCurrency == coin {
		amount = amount / marketInfo.CTValue
	}
	decimal := util.NumDecPlaces(marketInfo.SizeIncrement)
	format := `%.` + strconv.Itoa(decimal) + `f`
	amount, _ = strconv.ParseFloat(fmt.Sprintf(format, amount), 64)
	formattedAmount = marketInfo.SizeIncrement * math.Floor(amount/marketInfo.SizeIncrement)
	formattedAmount, _ = strconv.ParseFloat(fmt.Sprintf(format, formattedAmount), 64)
	// bitgetperp reduce的时候应该不受最小下单金额限制
	if reduceOnly && market == BitgetPerp {
		return formattedAmount
	}
	if formattedAmount < marketInfo.SizeMin || marketInfo.SizeMin == 0 ||
		(marketInfo.MoneyMin > 0 && marketInfo.MoneyMin > price*formattedAmount) {
		return 0
	}
	return formattedAmount
}

func FormatPrice(market, symbol string, price float64) (formattedPrice float64, decimal int) {
	v, _ := util.LoadSyncMap(MarketInfos, market, symbol)
	if v == nil || v.(*MarketInfo).SizeIncrement == 0 {
		return 0, 0
	}
	priceIncrement := v.(*MarketInfo).PriceIncrement
	return priceIncrement * math.Round(price/priceIncrement), v.(*MarketInfo).PriceDecimal
	//if orderSide == OrderSideBuy {
	//	return marketInfo.PriceIncrement * math.Ceil(price/marketInfo.PriceIncrement), marketInfo.PriceDecimal
	//} else {
	//	return marketInfo.PriceIncrement * math.Floor(price/marketInfo.PriceIncrement), marketInfo.PriceDecimal
	//}
}

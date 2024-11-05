package hang

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

var doWork = false
var placeTime sync.Map // market_symbol/timeInMilli
var hangSide sync.Map  // market_symbol/orderSide
// var hangPrice sync.Map // market_symbol/bid1Price-ask1Price
var dealInU sync.Map // market_symbol_timeStr/deal_in_u

// ProcessHang
// setting.chance 下单后多少million seconds cancel并进行后续下单
// setting.gridAmount 当日下单上限 in usd
// setting.amountLimit 每次下单价差jump距离
// setting.priceX 做市摆单价差，默认为0，值越大，摆单后买卖1之间价差越大
// setting.OpenShortMargin 处于吃单模式时，可吃单的上限
// setting.CloseShortMargin 大于0时同时进行吃单
var ProcessHang = func(setting *model.Setting, tick *model.BidAsk) {
	if !doWork && model.AppConfig.Handle == `1` {
		go refreshDeal(setting)
		doWork = true
		return
	}
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || setting.Valid == false ||
		(model.AppConfig.Env != `test` && (model.AppConfig.Handle != `1` || time.Now().UnixMilli()-int64(tick.Ts) > 100)) {
		return
	}
	marketSymbol := fmt.Sprintf(`%s_%s`, setting.Market, setting.Symbol)
	orderTime, okPlace := placeTime.Load(marketSymbol)
	now := time.Now().UnixMilli()
	accounts := model.GetAccounts(0)
	if accounts == nil || len(accounts) == 0 || accounts[setting.Market] == nil {
		return
	}
	account := accounts[setting.Market]
	if !okPlace {
		handle(account, setting, tick)
	} else {
		if now-orderTime.(int64) > setting.Chance {
			//util.Notice(fmt.Sprintf(`cancel %s `, marketSymbol))
			if api.CancelOrders(account.Key, account.Secret, setting.Market, setting.Symbol) {
				placeTime.Delete(marketSymbol)
				model.AppDB.Where(`market=? and symbol=? and status=? and function=?`,
					setting.Market, setting.Symbol, model.CarryStatusFail, model.FunctionHang).Delete(&model.Order{})
			}
		}
	}
}

func placeHang(account *model.Account, setting *model.Setting, marketInfo *model.MarketInfo,
	side string, tick *model.BidAsk) {
	utcTime := time.Now().In(time.UTC)
	year, month, day := utcTime.Date()
	marketSymbolDate := fmt.Sprintf(`%s_%s_%d-%d-%d`, setting.Market, setting.Symbol, year, month, day)
	dealAmount, okDeal := dealInU.Load(marketSymbolDate)
	if !okDeal {
		dealAmount = 0.0
	} else if dealAmount.(float64) > setting.GridAmount {
		util.Notice(fmt.Sprintf(`exceed amount limit %s %f > %f`,
			marketSymbolDate, dealAmount.(float64), setting.GridAmount))
		return
	}
	steps := (tick.Asks[0].Price-tick.Bids[0].Price-setting.PriceX)/marketInfo.PriceIncrement - 1
	steps = math.Ceil(steps * (setting.GridAmount - dealAmount.(float64)) / setting.GridAmount)
	inc := 1.0
	beginPrice := 0.0
	//priceMark := ``
	if side == model.OrderSideBuy {
		beginPrice = tick.Bids[0].Price
		inc = 1.0
		//bidPrice, decimal := model.FormatPrice(setting.Market, setting.Symbol, model.OrderSideBuy, tick.Bids[0].Price+marketInfo.PriceIncrement*steps)
		//bidStr := util.CutTailZero(strconv.FormatFloat(bidPrice, 'f', decimal, 64))
		//priceMark = fmt.Sprintf(`%s-%s`, bidStr, util.CutTailZero(strconv.FormatFloat(tick.Asks[0].Price, 'f', decimal, 64)))
	} else if side == model.OrderSideSell {
		beginPrice = tick.Asks[0].Price
		inc = -1.0
		//askPrice, decimal := model.FormatPrice(setting.Market, setting.Symbol, model.OrderSideSell, tick.Asks[0].Price-marketInfo.PriceIncrement*steps)
		//askStr := util.CutTailZero(strconv.FormatFloat(askPrice, 'f', decimal, 64))
		//priceMark = fmt.Sprintf(`%s-%s`, util.CutTailZero(strconv.FormatFloat(tick.Bids[0].Price, 'f', decimal, 64)), askStr)
	}
	jump := int(math.Ceil(steps / setting.AmountLimit))
	for i := int(steps); i > 0; i = i - jump {
		price := beginPrice + inc*marketInfo.PriceIncrement*float64(i)
		amount := marketInfo.MoneyMin/price + marketInfo.SizeIncrement*(steps+1-float64(i))*math.Ceil(rand.Float64()*5)
		util.Notice(fmt.Sprintf(`hang %s %s steps: %f [%f-%f] %f %f`,
			setting.Market, setting.Symbol, steps, tick.Bids[0].Price, tick.Asks[0].Price, price, amount))
		order := api.PlaceOrder(account.Key, account.Secret, side, model.OrderTypeLimit, setting.Market,
			setting.Symbol, ``, model.FunctionHang, price, price, amount, false, nil, setting)
		if order != nil {
			order.Function = model.FunctionHang
			model.AppDB.Save(order)
		}
		time.Sleep(time.Millisecond * 500)
	}
}

func handle(account *model.Account, setting *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, true) {
		defer api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, false)
	} else {
		return
	}
	marketInfo, _ := util.LoadSyncMap(model.MarketInfos, setting.Market, setting.Symbol)
	if marketInfo == nil {
		api.InitMarketInfos(setting.Market)
		return
	}
	marketSymbol := setting.Market + `_` + setting.Symbol
	value, okHang := hangSide.Load(marketSymbol)
	if !okHang || value == nil {
		return
	}
	side := value.(string)
	//util.Notice(fmt.Sprintf(`status %s %s %s priceX %f market bid %f`,
	//	setting.Market, setting.Symbol, side, setting.PriceX, tick.Asks[0].Amount))
	placeHang(account, setting, marketInfo.(*model.MarketInfo), side, tick)
	if setting.CloseShortMargin > 0 && tick.Asks[0].Amount < 5000 && tick.Asks[0].Amount < setting.OpenShortMargin &&
		side == model.OrderSideBuy && tick.Asks[0].Price < 0.1 { // 吃单拉价格模式
		order := api.PlaceOrder(account.Key, account.Secret, side, model.OrderTypeLimit, setting.Market, setting.Symbol,
			``, model.FunctionHang, tick.Asks[0].Price, tick.Asks[0].Price, tick.Asks[0].Amount, false, nil, setting)
		if order != nil {
			order.Function = model.FunctionHang
			model.AppDB.Save(order)
		}
	}
	//util.Notice(fmt.Sprintf(`hang prices %s %s`, setting.Market, setting.Symbol))
	//hangPrice.Store(marketSymbol, priceMark)
	placeTime.Store(marketSymbol, time.Now().UnixMilli())
}

func refreshDeal(setting *model.Setting) {
	for doWork {
		for true {
			if !api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, true) {
				break
			} else {
				time.Sleep(time.Millisecond * 200)
			}
		}
		utcTime := time.Now().In(time.UTC)
		year, month, day := utcTime.Date()
		dateStr := fmt.Sprintf(`%d-%d-%d`, year, month, day)
		coinSettings := api.GetCoinSettings(model.FunctionHang)
		if coinSettings != nil {
			coinSettings.Range(func(key, value interface{}) bool {
				for _, setting := range value.([]*model.Setting) {
					var deal, usd, usdCoin float64
					side := ``
					model.AppDB.Table(`orders`).Select(`sum(deal_amount*deal_price)`).Where(
						"market= ? and symbol= ? and refresh_type= ? and status=? and order_time>?",
						setting.Market, setting.Symbol, model.FunctionHang, model.CarryStatusSuccess, dateStr).First(&deal)
					dealInU.Store(setting.Market+`_`+setting.Symbol+`_`+dateStr, deal)
					accounts := model.GetAccounts(0)
					if accounts != nil && len(accounts) > 0 && accounts[setting.Market] != nil {
						account := accounts[setting.Market]
						success, balances, _, _ := api.GetBalances(account.Key, account.Secret, setting.Market)
						if success && balances != nil {
							_, _, coin, _ := model.GetFromStandard(setting.Market, setting.Symbol)
							for _, balance := range balances {
								if strings.EqualFold(balance.Coin, `usd`) || strings.EqualFold(balance.Coin, `usdt`) {
									usd += balance.Amount
								} else if balance.Coin == coin {
									_, price := api.GetPriceForce(account.Key, account.Secret, setting.Symbol, setting.Market)
									usdCoin = balance.Amount * price
								}
							}
							if usd > usdCoin {
								side = model.OrderSideBuy
							} else {
								side = model.OrderSideSell
							}
							hangSide.Store(setting.Market+`_`+setting.Symbol, side)
						}
					}
					util.Notice(fmt.Sprintf(`refreshDeal %s %s %s deal %fu account %fu %fcoin in u side %s`,
						setting.Market, setting.Symbol, dateStr, deal, usd, usdCoin, side))
				}
				return true
			})
		}
		api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, false)
		time.Sleep(time.Minute * 10)
	}
}

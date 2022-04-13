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

var doHang = false
var hanging = false
var placeTime sync.Map // market_symbol/timeInMilli
var hangSide sync.Map  // market_symbol/orderSide
var dealInU sync.Map   // market_symbol_timeStr/deal_in_u
var hangLock sync.Mutex

func checkSetHanging(value bool) (before bool) {
	hangLock.Lock()
	defer hangLock.Unlock()
	before = hanging
	if value == false || before == false {
		hanging = value
	}
	return before
}

// ProcessHang
// setting.chance 下单后多少million seconds cancel
// setting.gridAmount 当日下单上线 in usd
var ProcessHang = func(setting *model.Setting, tick *model.BidAsk) {
	if !doHang && model.AppConfig.Handle == `1` {
		go refreshDeal()
		doHang = true
		return
	}
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || setting.Valid == false ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) ||
		time.Now().UnixMilli()-int64(tick.Ts) > 100 {
		return
	}
	marketSymbol := fmt.Sprintf(`%s_%s`, setting.Market, setting.Symbol)
	orderTime, ok := placeTime.Load(marketSymbol)
	now := time.Now().UnixMilli()
	accounts := model.GetAccounts(0)
	if accounts == nil || len(accounts) == 0 || accounts[setting.Market] == nil {
		return
	}
	account := accounts[setting.Market]
	if !ok {
		placeHang(account, setting, tick)
		placeTime.Store(marketSymbol, now)
	} else if now-orderTime.(int64) > setting.Chance {
		if api.CancelOrders(account.Key, account.Secret, setting.Market, setting.Symbol) {
			placeTime.Delete(marketSymbol)
		}
	}
}

func placeHang(account *model.Account, setting *model.Setting, tick *model.BidAsk) {
	if !checkSetHanging(true) {
		defer checkSetHanging(false)
	} else {
		return
	}
	marketInfo := model.GetMarketInfo(setting.Market, setting.Symbol)
	if marketInfo == nil {
		api.InitMarketInfos()
		return
	}
	side, okHang := hangSide.Load(setting.Market + `_` + setting.Symbol)
	if !okHang || side == `` {
		return
	}
	dealAmount, okDeal := dealInU.Load(setting.Market + `_` + setting.Symbol)
	if !okDeal {
		dealAmount = 0.0
	} else if dealAmount.(float64) > setting.GridAmount {
		return
	}
	utcTime := time.Now().In(time.UTC)
	year, month, day := utcTime.Date()
	keyStr := fmt.Sprintf(`%s_%s_%d-%d-%d 08:00:00`, setting.Market, setting.Symbol, year, month, day)
	steps := (tick.Asks[0].Price-tick.Bids[0].Price)/marketInfo.PriceIncrement - 1
	steps = math.Ceil(steps * (setting.GridAmount - dealAmount.(float64)) / setting.GridAmount)
	inc := 1.0
	beginPrice := 0.0
	if side.(string) == model.OrderSideBuy {
		beginPrice = tick.Bids[0].Price
		inc = 1.0
	} else if side.(string) == model.OrderSideSell {
		beginPrice = tick.Asks[0].Price
		inc = -1.0
	}
	for i := int(steps); i > 0; i-- {
		price := beginPrice + inc*marketInfo.PriceIncrement*float64(i)
		amount := marketInfo.SizeMin + marketInfo.SizeIncrement*(steps+1-float64(i))*math.Round(rand.Float64()*5)
		order := api.PlaceOrder(account.Key, account.Secret, side.(string), model.OrderTypeLimit, setting.Market,
			setting.Symbol, ``, price, price, amount, false, nil, setting)
		if order != nil {
			model.AppDB.Save(order)
		}
		time.Sleep(time.Millisecond * 500)
	}
	placeTime.Store(keyStr, time.Now().UnixMilli())
}

func refreshDeal() {
	for doHang {
		for true {
			if !checkSetHanging(true) {
				break
			} else {
				time.Sleep(time.Millisecond * 200)
			}
		}
		utcTime := time.Now().In(time.UTC)
		year, month, day := utcTime.Date()
		dateStr := fmt.Sprintf(`%d-%d-%d 08:00:00`, year, month, day)
		coinSettings := model.GetCoinSettings(model.FunctionHang)
		for _, settings := range coinSettings {
			for _, setting := range settings {
				var deal, usd, usdCoin float64
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
								tickGet, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
								if tickGet && tick != nil {
									usdCoin = balance.Amount * tick.Bids[0].Price
								}
							}
						}
						if usd > usdCoin {
							hangSide.Store(setting.Market+`_`+setting.Symbol, model.OrderSideBuy)
						} else {
							hangSide.Store(setting.Market+`_`+setting.Symbol, model.OrderSideSell)
						}
					}
				}
				util.Notice(fmt.Sprintf(`refreshDeal %s %s %s deal %fu account %fu %fcoin in u`,
					setting.Market, setting.Symbol, dateStr, deal, usd, usdCoin))
			}
		}
		checkSetHanging(false)
		time.Sleep(time.Minute * 10)
	}
}

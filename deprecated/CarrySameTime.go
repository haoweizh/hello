package deprecated

//import (
//	"fmt"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"sync"
//	"time"
//)
//
//// setting:
//// gridAmount 下单数量（图上20000）
//// gridPriceDistance （图上5）
//// amountLimit 总量限制（A总）
//// refreshLimitLow 最低数量限制（图上4000）
//// priceX （价格偏移基数）
//var carrySameTimeLock sync.Mutex
//
//var lastOrders = make(map[string]map[string]*model.Order)
//var carryChannels = make(map[string]chan model.Order)
//
//func setLastOrder(key, market string, order *model.Order) {
//	defer carrySameTimeLock.Unlock()
//	carrySameTimeLock.Lock()
//	if lastOrders == nil {
//		lastOrders = make(map[string]map[string]*model.Order)
//	}
//	if lastOrders[key] == nil {
//		lastOrders[key] = make(map[string]*model.Order)
//	}
//	lastOrders[key][market] = order
//}
//
////getLastOrder
//func _(key, market string) (order *model.Order) {
//	defer carrySameTimeLock.Unlock()
//	carrySameTimeLock.Lock()
//	if lastOrders == nil {
//		lastOrders = make(map[string]map[string]*model.Order)
//	}
//	if lastOrders[key] == nil {
//		lastOrders[key] = make(map[string]*model.Order)
//	}
//	return lastOrders[key][market]
//}
//
////getCarryChannel
//func _(key string) (carryChannel chan model.Order) {
//	defer carrySameTimeLock.Unlock()
//	carrySameTimeLock.Lock()
//	if carryChannels == nil {
//		carryChannels = make(map[string]chan model.Order)
//	}
//	if carryChannels[key] == nil {
//		carryChannels[key] = make(chan model.Order, 0)
//	}
//	return carryChannels[key]
//}
//
////ProcessCarrySameTime
//var _ = func(setting *model.Setting) {
//	startTime := util.GetNowUnixMillion()
//	if setting == nil || setting.MarketRelated == `` {
//		return
//	}
//	_, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
//	_, tickRelated := model.AppMarkets.GetBidAsk(setting.Symbol, setting.MarketRelated)
//	//account := model.AppAccounts.GetAccount(market, symbol)
//	accountRelated := model.AppAccounts.GetAccount(setting.MarketRelated, setting.Symbol)
//	if (setting.MarketRelated != model.OKSwap && accountRelated == nil) || (setting.MarketRelated == model.OKSwap &&
//		model.AppAccounts.GetAccount(model.OKSwap, model.OrderSideBuy+setting.Symbol) == nil &&
//		model.AppAccounts.GetAccount(model.OKSwap, model.OrderSideSell+setting.Symbol) == nil) {
//		api.RefreshAccount(setting.MarketRelated)
//		util.Info(`error1 account is nil, refresh and return`)
//		return
//	}
//	freeRelated := 0.0
//	if setting.MarketRelated == model.OKSwap {
//		freeRelated = combineOKSwapAccounts(setting.Symbol)
//	} else {
//		freeRelated = accountRelated.Free
//	}
//	if tickRelated == nil || tick == nil || tickRelated.Asks == nil || tickRelated.Bids == nil || tick.Asks == nil ||
//		tick.Bids == nil {
//		if tick == nil {
//			util.Info(fmt.Sprintf(`error2 %s tick is nil`, setting.Market))
//		}
//		if tickRelated == nil {
//			util.Info(fmt.Sprintf(`error2 %s tick is nil`, setting.MarketRelated))
//		}
//		return
//	}
//	if tickRelated.Asks.Len() < 5 || tickRelated.Bids.Len() < 5 || tick.Asks.Len() < 5 || tick.Bids.Len() < 5 ||
//		tick.Bids[0].Price >= tick.Asks[0].Price || tickRelated.Bids[0].Price >= tickRelated.Asks[0].Price {
//		util.Info(fmt.Sprintf(`error3 %s %d %d %f-%f or %s %d %d %f-%fdepth tick length not good`,
//			setting.Market, tick.Bids.Len(), tick.Asks.Len(), tick.Bids[0].Price, tick.Asks[0].Price, setting.MarketRelated,
//			tickRelated.Bids.Len(), tickRelated.Asks.Len(), tickRelated.Bids[0].Price, tickRelated.Asks[0].Price))
//		return
//	}
//	if (int(startTime)-tick.Ts > 400 || int(startTime)-tickRelated.Ts > 200) ||
//		model.AppConfig.HandleLink != `1` || model.AppPause {
//		//util.Info(fmt.Sprintf(`error4 now:%d related:%s tick_%s delta:%d tick_%s delta:%d`,
//		//	startTime, setting.MarketRelated, setting.Market, int(startTime)-tick.Ts, setting.MarketRelated,
//		//	int(startTime)-tickRelated.Ts))
//		return
//	}
//	key := fmt.Sprintf(`%s-%s-%s`, setting.Market, setting.MarketRelated, setting.Symbol)
//	placeBothOrders(setting.Market, setting.Symbol, key, tick, tickRelated, freeRelated, setting)
//}
//
//func combineOKSwapAccounts(symbol string) (free float64) {
//	accountBuy := model.AppAccounts.GetAccount(model.OKSwap, model.OrderSideBuy+symbol)
//	accountSell := model.AppAccounts.GetAccount(model.OKSwap, model.OrderSideSell+symbol)
//	freeBuy := 0.0
//	freeSell := 0.0
//	if accountBuy != nil {
//		freeBuy = accountBuy.Free
//	}
//	if accountSell != nil {
//		freeSell = accountSell.Free
//	}
//	return freeBuy - math.Abs(freeSell)
//}
//
//// account.free被设置成-1 * accountRelated.free
//func placeBothOrders(market, symbol, key string, tick, tickRelated *model.BidAsk, freeRelated float64,
//	setting *model.Setting) {
//	p1 := 0.0
//	p2 := 0.0
//	a1 := setting.AmountLimit
//	a2 := setting.AmountLimit
//	value, expired := api.GetFundingRate(market, symbol)
//	zFee := value.(float64)
//	value, expiredRelated := api.GetFundingRate(setting.MarketRelated, symbol)
//	zFeeRelated := value.(float64)
//	if setting.MarketRelated == model.Bybit || setting.MarketRelated == model.OKSwap {
//		if expired > expiredRelated {
//			zFee = 0
//		} else {
//			zFeeRelated = 0
//		}
//	}
//	priceX := setting.PriceX + (zFeeRelated-zFee)*(tickRelated.Bids[0].Price+tickRelated.Asks[0].Price)/2
//	py := priceX
//	if freeRelated > setting.AmountLimit/10 && -1*freeRelated < setting.AmountLimit/-10 {
//		p1 = 0
//		p2 = -1 * freeRelated / setting.AmountLimit
//		a1 = freeRelated
//		a2 = setting.AmountLimit - freeRelated
//		priceX -= 5 * p2
//	} else if freeRelated < setting.AmountLimit/-10 &&
//		-1*freeRelated > setting.AmountLimit/10 {
//		p1 = freeRelated / setting.AmountLimit
//		p2 = 0
//		a1 = setting.AmountLimit + freeRelated
//		a2 = -1 * freeRelated
//		priceX += 5 * p1
//	}
//	if priceX > 7 {
//		priceX = priceX/2 + 3.5
//	} else if priceX < -7 {
//		priceX = priceX/2 - 3.5
//	}
//	carryInfo := map[string]interface{}{`zFee`: zFee, `zFeeRelated`: zFeeRelated, `related`: setting.MarketRelated,
//		`p1`: p1, `p2`: p2, `py`: py, `priceX`: priceX, `freeRelated`: freeRelated, `time`: util.GetNow().String(),
//		`延时`: util.GetNowUnixMillion() - int64(tick.Ts), `grid amount`: setting.GridAmount,
//		`grid distance`: setting.GridPriceDistance, `最低数量amount limit`: setting.AmountLimit,
//		`refresh limit low`: setting.RefreshLimitLow,
//		`bid len`:           tick.Bids.Len(), `ask len`: tick.Asks.Len(), `bid5`: tick.Bids[4].Price, `bid4`: tick.Bids[3].Price,
//		`bid3`: tick.Bids[2].Price, `bid2`: tick.Bids[1].Price, `bid1`: tick.Bids[0].Price, `ask1`: tick.Asks[0].Price,
//		`ask2`: tick.Asks[1].Price, `ask3`: tick.Asks[2].Price, `ask4`: tick.Asks[3].Price, `ask5`: tick.Asks[4].Price}
//	model.SetCarryInfos(model.FunctionCarry+`_`+market, setting.MarketRelated, carryInfo)
//	priceDistance := 0.1 / math.Pow(10, api.GetPriceDecimal(setting.MarketRelated, symbol))
//	calcAmtPriceBuy := tick.Asks[0].Price - api.GetPriceDistance(market, symbol) +
//		setting.GridPriceDistance - p1 - priceX
//	calcAmtPriceSell := tick.Bids[0].Price + api.GetPriceDistance(market, symbol) -
//		setting.GridPriceDistance + p2 - priceX
//	fmba := getDepthAmountBuy(calcAmtPriceBuy, 0, tickRelated)
//	fmsa := getDepthAmountSell(calcAmtPriceSell, 0, tickRelated)
//	calcAmtPriceBuyNew := tick.Asks[0].Price*1002/1000 + setting.GridPriceDistance - p1 - priceX
//	calcAmtPriceSellNew := tick.Bids[0].Price*998/1000 - setting.GridPriceDistance + p2 - priceX
//	fmbaNew := getDepthAmountBuy(calcAmtPriceBuyNew, 0, tickRelated)
//	fmsaNew := getDepthAmountSell(calcAmtPriceSellNew, 0, tickRelated)
//	fmb1 := tickRelated.Bids[0].Price + priceX
//	fms1 := tickRelated.Asks[0].Price + priceX
//	amount := 0.0
//	orderSide := ``
//	orderSideRelated := ``
//	orderPrice := 0.0
//	orderPriceRelated := 0.0
//	carryType := 0
//	orderParam := ``
//	amountLine := 900000.0
//	bidAskRate := 1.0 / 9.0
//	if fmb1+priceDistance >= calcAmtPriceBuyNew+priceX && fmbaNew >= setting.RefreshLimitLow {
//		amount = math.Min(math.Min(0.8*fmbaNew, a1), setting.GridAmount)
//		orderSideRelated = model.OrderSideSell
//		orderSide = model.OrderSideBuy
//		orderPriceRelated = calcAmtPriceBuyNew
//		orderPrice = tick.Asks[0].Price * 1.001
//		carryType = 1
//	} else if fmb1+priceDistance >= calcAmtPriceBuy+priceX && fmba >= setting.RefreshLimitLow &&
//		tick.Bids[0].Amount < bidAskRate*tick.Asks[0].Amount && tick.Asks[0].Amount > amountLine {
//		//checkLastBid(setting.MarketRelated, symbol, model.OrderSideBuy) {
//		amount = math.Min(math.Min(fmba*0.8, a1), setting.GridAmount)
//		orderSideRelated = model.OrderSideSell
//		orderPriceRelated = calcAmtPriceBuy
//		orderSide = model.OrderSideBuy
//		orderPrice = tick.Asks[0].Price - api.GetPriceDistance(market, symbol)
//		orderParam = model.PostOnly
//		carryType = 2
//	} else if fms1-priceDistance <= calcAmtPriceSellNew+priceX && fmsaNew >= setting.RefreshLimitLow {
//		amount = math.Min(math.Min(0.8*fmsaNew, a2), setting.GridAmount)
//		orderSideRelated = model.OrderSideBuy
//		orderSide = model.OrderSideSell
//		orderPrice = tick.Bids[0].Price * 0.999
//		orderPriceRelated = calcAmtPriceSellNew
//		carryType = 3
//	} else if fms1-priceDistance <= calcAmtPriceSell+priceX && fmsa >= setting.RefreshLimitLow &&
//		tick.Asks[0].Amount < tick.Bids[0].Amount*bidAskRate && tick.Bids[0].Amount > amountLine {
//		//checkLastBid(market, symbol, model.OrderSideSell) {
//		amount = math.Min(math.Min(fmsa*0.8, a2), setting.GridAmount)
//		orderSideRelated = model.OrderSideBuy
//		orderSide = model.OrderSideSell
//		orderPriceRelated = calcAmtPriceSell
//		orderParam = model.PostOnly
//		orderPrice = tick.Bids[0].Price + api.GetPriceDistance(market, symbol)
//		carryType = 4
//	}
//	amount = math.Floor(amount/100) * 100
//	if amount > 1 && (carryType == 2 || carryType == 4) {
//		util.Notice(fmt.Sprintf(`情况%d %s-%s, 资金费率: %f-%f priceX:%f fmb1:%f fms1:%f fmba:%f fmsa:%f fmbaNew:%f
//			fmsaNew:%f tick price:%f-%f tick amount:%f-%f tickRelatedPrice:%f-%f tickRelatedAmount:%f-%f p1:%f p2:%f 持仓:%f`,
//			carryType, market, setting.MarketRelated, zFee, zFeeRelated, priceX, fmb1, fms1, fmba, fmsa, fmbaNew,
//			fmsaNew, tick.Bids[0].Price, tick.Asks[0].Price, tick.Bids[0].Amount, tick.Asks[0].Amount,
//			tickRelated.Bids[0].Price, tickRelated.Asks[0].Price, tickRelated.Bids[0].Amount, tickRelated.Asks[0].Amount,
//			p1, p2, freeRelated))
//		setLastOrder(key, market, nil)
//		setLastOrder(key, setting.MarketRelated, nil)
//		//carryChannel := getCarryChannel(key)
//		refreshType := fmt.Sprintf(`%s_%s_%s`, model.FunctionCarry, setting.Market, setting.MarketRelated)
//		api.PlaceOrder(orderSideRelated, model.OrderTypeLimit, setting.MarketRelated, symbol, ``,
//			``, setting.AccountType, ``, refreshType, orderPriceRelated, 0, amount, true)
//		time.Sleep(time.Millisecond * 500)
//		util.Notice(fmt.Sprintf(`ignore order %s %s %s %f %s`,
//			setting.Market, symbol, orderSide, orderPrice, orderParam))
//		api.RefreshAccount(setting.MarketRelated)
//		//go api.PlaceSyncOrders(orderSideRelated, model.OrderTypeLimit, setting.MarketRelated, symbol,
//		//	``, ``, setting.AccountType, ``, refreshType, orderPriceRelated, amount,
//		//	true, carryChannel, -1)
//		//go api.PlaceSyncOrders(orderSide, model.OrderTypeLimit, market, symbol, ``,
//		//	``, setting.AccountType, orderParam, refreshType, orderPrice, amount, true, carryChannel, -1)
//		//for true {
//		//	order := <-carryChannel
//		//	util.Notice(fmt.Sprintf(`---- get order %s %s %s`, order.Market, order.OrderId, order.Status))
//		//	setLastOrder(key, order.Market, &order)
//		//	if getLastOrder(key, market) != nil && getLastOrder(key, setting.MarketRelated) != nil {
//		//		util.Notice(`---- get both, break`)
//		//		break
//		//	}
//		//}
//		//orderMarket := getLastOrder(key, market)
//		//orderRelated := getLastOrder(key, setting.MarketRelated)
//		//if api.IsValid(orderMarket) && api.IsValid(orderRelated) {
//		//	time.Sleep(time.Second)
//		//	if setting.MarketRelated == model.Bybit {
//		//		if orderRelated.OrderSide == model.OrderSideSell {
//		//			freeRelated -= orderRelated.Amount
//		//		} else {
//		//			freeRelated += orderRelated.Amount
//		//		}
//		//		accountBybit := model.AppAccounts.GetAccount(model.Bybit, symbol)
//		//		if accountBybit != nil {
//		//			accountBybit.Free = freeRelated
//		//		}
//		//		model.AppAccounts.SetAccount(setting.MarketRelated, symbol, accountBybit)
//		//	} else {
//		//		api.RefreshAccount(model.OKSwap)
//		//	}
//		//}
//	}
//}
//
//func getDepthAmountSell(price, priceDistance float64, tick *model.BidAsk) (amount float64) {
//	amount = 0
//	for i := 0; i < tick.Asks.Len(); i++ {
//		if price > tick.Asks[i].Price-priceDistance {
//			amount += tick.Asks[i].Amount
//		} else {
//			break
//		}
//	}
//	return
//}
//
//func getDepthAmountBuy(price, priceDistance float64, tick *model.BidAsk) (amount float64) {
//	amount = 0
//	for i := 0; i < tick.Bids.Len(); i++ {
//		if price < tick.Bids[i].Price+priceDistance {
//			amount += tick.Bids[i].Amount
//		} else {
//			break
//		}
//	}
//	return
//}

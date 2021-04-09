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

const OrderPriceLimit = 0
const revertDis = 0.005
const openValueLimit = 10000.0

var resetInitialized = false
var borrowInitialized = false
var carryLock sync.Mutex
var carrying bool
var doCarry = false
var symbolHighest, symbolLowest string
var lowest = math.NaN()
var highest = math.NaN()
var usdAvailable = make(map[string]float64)                   // key - float64
var usdRate = make(map[string]float64)                        // key - float64
var balanceAll = make(map[string]float64)                     // key - balance value in all
var carryBalance = make(map[string]map[string]*model.Balance) // key - coin - balance
var carryAmount = make(map[string]map[string]float64)         // key - perp - float64
var tradeMax = make(map[string]map[string][]float64)          // key - instrument - [maxBuy合约张数/币币个数, maxSell]

var postOrderCarry = func(order *model.Order) {
	if order == nil || order.OrderId == `` || order.Status == model.CarryStatusFail {
		if order != nil {
			resetSingleTradeMax(order.AmountType, order.Market, order.Symbol)
			util.Notice(fmt.Sprintf(`order fail, reset trade max %s %s %s`,
				order.Instrument, order.AmountType, order.ErrCode))
		}
		//resetTradeMax(model.OKEX)
		return
	}
	maxBuy, maxSell := getTradeMax(order.AmountType, order.Symbol)
	// 需要经过转化成张数（合约）
	amount := api.GetAmountInPerpOKEX(model.OKEX, order.Instrument, order.Amount)
	if order.OrderSide == model.OrderSideBuy {
		maxBuy -= amount
		maxSell += amount
	} else if order.OrderSide == model.OrderSideSell {
		maxBuy += amount
		maxSell -= amount
	}
	setTradeMax(order.AmountType, order.Instrument, maxBuy, maxSell)
}

func getTradeMax(key, instrument string) (maxBuy, maxSell float64) {
	defer carryLock.Unlock()
	carryLock.Lock()
	if tradeMax[key] == nil {
		return 0, 0
	}
	if tradeMax[key][instrument] == nil || len(tradeMax[key][instrument]) != 2 {
		return 0, 0
	}
	return tradeMax[key][instrument][0], tradeMax[key][instrument][1]
}

func setTradeMax(key, instrument string, maxBuy, maxSell float64) {
	defer carryLock.Unlock()
	carryLock.Lock()
	if tradeMax[key] == nil {
		tradeMax[key] = make(map[string][]float64)
	}
	tradeMax[key][instrument] = []float64{maxBuy, maxSell}
}

func getUsdAvailable(key string) float64 {
	carryLock.Lock()
	defer carryLock.Unlock()
	return usdAvailable[key]
}

func setUsdAvailable(key string, value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	usdAvailable[key] = value
}

func setBalanceAll(key string, value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	balanceAll[key] = value
}

func getBalanceAll(key string) (value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	return balanceAll[key]
}

func getUsdRate(key string) float64 {
	carryLock.Lock()
	defer carryLock.Unlock()
	return usdRate[key]
}

func setUsdRate(key string, value float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	usdRate[key] = value
}

func getCarryAmount(key, perp string) float64 {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryAmount[key] == nil {
		return 0
	}
	return carryAmount[key][perp]
}

func setCarryAmount(key, perp string, amount float64) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryAmount[key] == nil {
		carryAmount[key] = make(map[string]float64)
	}
	carryAmount[key][perp] = amount
}

func setCarryBalance(key, coin string, balance *model.Balance) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryBalance[key] == nil {
		carryBalance[key] = make(map[string]*model.Balance)
	}
	carryBalance[key][coin] = balance
}

func getCarryBalance(key, coin string) (balance *model.Balance) {
	carryLock.Lock()
	defer carryLock.Unlock()
	if carryBalance[key] == nil {
		return nil
	}
	return carryBalance[key][coin]
}

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

func getSpotTail(market string) string {
	switch market {
	case model.Ftx:
		return `/USD`
	case model.OKEX:
		return `-USDT`
	}
	return ``
}

func getPerpTail(market string) string {
	switch market {
	case model.Ftx:
		return `-PERP`
	case model.OKEX:
		return `-USDT-SWAP`
	}
	return ``
}

func resetSingleTradeMax(key, market, symbol string) {
	setTradeMax(key, symbol, 0, 0)
	keys, secrets := model.AppConfig.GetKeys(market)
	for i, value := range keys {
		if value == key {
			_, maxBuy, maxSell := api.GetMaxSize(key, secrets[i], symbol)
			setTradeMax(key, symbol, maxBuy, maxSell)
		}
	}
}

func resetTradeMax(market string) {
	resetInitialized = true
	if market != model.OKEX {
		return
	}
	keys, secrets := model.AppConfig.GetKeys(market)
	settings := model.GetSettings(model.FunctionCarry, market)
	for _, items := range settings {
		if items == nil || len(items) == 0 {
			continue
		}
		for i := range keys {
			_, maxBuy, maxSell := api.GetMaxSize(keys[i], secrets[i], items[0].Symbol)
			setTradeMax(keys[i], items[0].Symbol, maxBuy, maxSell)
			related := items[0].GetRelatedSymbol()
			_, maxBuy, maxSell = api.GetMaxSize(keys[i], secrets[i], related)
			setTradeMax(keys[i], related, maxBuy, maxSell)
		}
		time.Sleep(time.Millisecond * 200)
	}
}

func clearCarryBalance() {
	for doCarry {
		for true {
			if !checkSetCarrying(true) {
				break
			} else {
				time.Sleep(time.Millisecond * 200)
			}
		}
		util.Notice(`...... enter clearing carry balance`)
		time.Sleep(time.Second * 2)
		markets := model.GetMarkets()
		for _, market := range markets {
			keys, secrets := model.AppConfig.GetKeys(market)
			for i, key := range keys {
				resultBalance, balances, _ := api.GetBalances(key, secrets[i], market, 0)
				resultPosition, positions := api.GetPositions(key, secrets[i], market)
				if !resultBalance || !resultPosition {
					util.Notice(`fatal error: can not get balance/position ` + market)
					continue
				}
				balanceAllValue := 0.0
				localUsdAvailable := 0.0
				borrowAll := 0.0
				for _, value := range balances {
					coin := strings.ToUpper(value.Coin)
					setCarryBalance(key, coin, value)
					settingCoins := model.GetSettingCoins(model.FunctionCarry, market)
					if settingCoins[coin] {
						balanceAllValue += value.UsdValue
					}
					if (coin == `USD` && market == model.Ftx) || (coin == `USDT` && market == model.OKEX) {
						localUsdAvailable = value.Amount
						setUsdAvailable(key, value.Amount)
						balanceAllValue += value.Amount
						borrowAll += value.Borrow
					}
					success, bidAsk := model.AppMarkets.GetBidAsk(coin+getSpotTail(market), market)
					if success {
						borrow := value.Borrow * bidAsk.Bids[0].Price
						borrowAll += borrow
						if !borrowInitialized || borrow > 100 {
							success, maxLoan := api.GetMaxLoan(key, secrets[i], market, coin)
							if success {
								value.AvailableWithBorrow = maxLoan
							}
							time.Sleep(time.Second / 6)
						}
					} else {
						util.Notice(fmt.Sprintf(`fatal: can not get price %s %s`, market, coin))
					}
				}
				localUsdAvailable = localUsdAvailable - borrowAll
				setUsdRate(key, localUsdAvailable/balanceAllValue)
				setBalanceAll(key, balanceAllValue)
				util.Notice(fmt.Sprintf(`[carry] %s usd:%f %f len(blances):%d`,
					key, localUsdAvailable, usdRate[key], len(balances)))
				settings := model.GetSettings(model.FunctionCarry, market)
				for _, items := range settings {
					for _, item := range items {
						makeEqual(key, secrets[i], item, balances, positions)
					}
				}
			}
		}
		if time.Now().Minute()%9 == 0 || !resetInitialized {
			resetTradeMax(model.OKEX)
		}
		borrowInitialized = true
		util.Notice(`...... exit clearing carry balance`)
		checkSetCarrying(false)
		time.Sleep(time.Second * 60)
	}
}

// setting.GridPriceDistance: 收回下单是要求的利润(可以为负数)
var ProcessCarry = func(setting *model.Setting, tick *model.BidAsk) {
	if !doCarry {
		go clearCarryBalance()
		doCarry = true
		return
	}
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	symbolRelated := setting.GetRelatedSymbol()
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	now := time.Now()
	million := util.GetNowUnixMillion()
	if tickPerp == nil || tickRelated == nil || tickPerp.Asks == nil || tickPerp.Bids == nil ||
		tickRelated.Asks == nil || tickRelated.Bids == nil || model.AppConfig.Handle != `1` || setting == nil ||
		model.AppPause || (model.AppConfig.Env != `test` &&
		(million-int64(tickRelated.Ts) > 2000 || million-int64(tickPerp.Ts) > 2000 || million-int64(tick.Ts) > 20)) {
		return
	}
	scoreOpen := 1 - tickRelated.Asks[0].Price/tickPerp.Bids[0].Price
	scoreClose := 1 - tickRelated.Bids[0].Price/tickPerp.Asks[0].Price
	model.AppMetric.AddCarry(setting.Market, setting.Symbol, scoreOpen, scoreClose)
	if math.IsNaN(highest) || scoreOpen > highest || setting.Symbol == symbolHighest {
		highest = scoreOpen
		symbolHighest = setting.Symbol
		model.AppMetric.AddCarry(setting.Market, setting.Market+`开仓价差++++`, highest, math.NaN())
	}
	if math.IsNaN(lowest) || scoreClose < lowest || setting.Symbol == symbolLowest {
		lowest = scoreClose
		symbolLowest = setting.Symbol
		model.AppMetric.AddCarry(setting.Market, setting.Market+`开仓价差----`, math.NaN(), lowest)
	}
	model.SetCarryInfo(`[current high-low]`, fmt.Sprintf(`highest %s %f lowest %s %f`, symbolHighest, highest, symbolLowest, lowest))
	marketInfo := map[string]interface{}{`symbol_highest`: symbolHighest, `symbol_lowest`: symbolLowest}
	if !math.IsNaN(highest) {
		marketInfo[`highest`] = highest
	}
	if !math.IsNaN(lowest) {
		marketInfo[`lowest`] = lowest
	}
	model.SetCarryInfos(`market_info`, `market_info`, marketInfo)
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	begin := 0
	step := 1
	if (now.Hour() < 6 && now.Hour() > 2 && now.Second()%4 != 0) || now.Second()%2 == 0 {
		begin = len(keys) - 1
		step = -1
	}
	for i := begin; i >= 0 && i < len(keys); i += step {
		sidePerp, sideRelated, amount := calcCarryOpen(setting, tickPerp, tickRelated, keys[i], setting.Symbol,
			setting.Symbol, scoreOpen, scoreClose, scoreOpen, scoreClose)
		if amount > 0 {
			go placeCarry(setting, tickPerp, tickRelated, keys[i], secrets[i], sidePerp, sideRelated,
				scoreOpen, scoreClose, amount)
			break
		}
	}
}

func placeCarry(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, secret, sidePerp, sideRelated string,
	scoreOpen, scoreClose, amount float64) {
	if !checkSetCarrying(true) {
		defer checkSetCarrying(false)
	} else {
		util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
		return
	}
	symbolRelated := setting.GetRelatedSymbol()
	coin := model.GetCoin(setting.Market, setting.Symbol)
	balance := getCarryBalance(key, coin)
	if balance == nil {
		return
	}
	perpPrice := tickPerp.Asks[0].Price
	relatedPrice := tickRelated.Bids[0].Price
	usdAvailable := getUsdAvailable(key)
	balanceAllValue := getBalanceAll(key)
	if sidePerp == model.OrderSideSell {
		perpPrice = tickPerp.Bids[0].Price
		relatedPrice = tickRelated.Asks[0].Price
		setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)+amount)
		balance.Amount += amount
		balance.AvailableWithBorrow += amount
		balance.UsdValue += amount * perpPrice
		usdAvailable -= amount * perpPrice
		setUsdAvailable(key, usdAvailable)
		setCarryBalance(key, coin, balance)
		setUsdRate(key, usdAvailable/balanceAllValue)
	} else if sidePerp == model.OrderSideBuy {
		perpPrice = tickPerp.Asks[0].Price
		relatedPrice = tickRelated.Bids[0].Price
		setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)-amount)
		balance.Amount -= amount
		balance.AvailableWithBorrow -= amount
		balance.UsdValue -= amount * perpPrice
		usdAvailable += amount * relatedPrice
		setCarryBalance(key, coin, balance)
		setUsdAvailable(key, usdAvailable)
		setUsdRate(key, usdAvailable/balanceAllValue)
	}
	now := int(util.GetNowUnixMillion())
	util.Notice(fmt.Sprintf(`carry%s->%s delay %d %d perp[%f %f %f %f] related[%f %f %f %f] with score open:%f close:%f 
	    amount %f worth %f time in million %d`,
		setting.Symbol, symbolRelated, now-tickPerp.TsReceived, now-tickRelated.TsReceived, tickPerp.Bids[0].Price,
		tickPerp.Bids[0].Amount, tickPerp.Asks[0].Price, tickPerp.Asks[0].Amount, tickRelated.Bids[0].Price,
		tickRelated.Bids[0].Amount, tickRelated.Asks[0].Price, tickRelated.Asks[0].Amount, scoreOpen, scoreClose,
		amount, amount*tickPerp.Asks[0].Price, util.GetNowUnixMillion()))
	go api.PlaceOrder(key, secret, sidePerp, model.OrderTypeLimit, setting.Market, setting.Symbol,
		``, ``, ``, model.FunctionCarry, perpPrice, perpPrice,
		amount, true, postOrderCarry)
	api.PlaceOrder(key, secret, sideRelated, model.OrderTypeLimit, setting.Market, symbolRelated,
		``, ``, ``, model.FunctionCarry, relatedPrice, relatedPrice,
		amount, true, postOrderCarry)
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	if key == keys[0] {
		time.Sleep(time.Second / 10)
	} else {
		time.Sleep(time.Second / 5)
	}
}

func getCarryAmounts(setting *model.Setting, balances []*model.Balance, positions []*model.Position) (
	success bool, amountPerp, amountRelated float64) {
	tail := getPerpTail(setting.Market)
	positionExist := false
	balanceExist := false
	for _, position := range positions {
		if position != nil && position.Currency == setting.Symbol {
			amountPerp = position.Free
			positionExist = true
		}
	}
	for _, balance := range balances {
		if strings.ToUpper(balance.Coin+tail) == setting.Symbol {
			amountRelated = balance.Amount
			balanceExist = true
		}
	}
	return positionExist && balanceExist, amountPerp, amountRelated
}

func makeEqual(key, secret string, setting *model.Setting, balances []*model.Balance, positions []*model.Position) (
	symbol string, price float64, equal bool) {
	settingSymbol := setting.Symbol
	coin := model.GetCoin(setting.Market, setting.Symbol)
	symbolRelated := setting.GetRelatedSymbol()
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
	if tickPerp == nil || tickRelated == nil {
		return ``, 0, true
	}
	success, amountPerp, amountRelated := getCarryAmounts(setting, balances, positions)
	if !success {
		return
	}
	amount := amountPerp + amountRelated
	orderSide := model.OrderSideBuy
	if amount < math.Max(math.Abs(amountPerp), math.Abs(amountRelated)) {
		if amountPerp < 0 && amountRelated > 0 {
			setCarryAmount(key, settingSymbol, math.Min(math.Abs(amountPerp), math.Abs(amountRelated)))
		} else if amountPerp > 0 && amountRelated < 0 {
			setCarryAmount(key, settingSymbol, -1*math.Min(math.Abs(amountPerp), math.Abs(amountRelated)))
		}
	} else {
		setCarryAmount(key, settingSymbol, 0)
	}
	balance := getCarryBalance(key, coin)
	if amount > 0 {
		orderSide = model.OrderSideSell
		if tickPerp.Bids[0].Price < (1-revertDis)*tickRelated.Bids[0].Price && amount < balance.AvailableWithBorrow {
			symbol = symbolRelated
			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
		} else if tickPerp.Bids[0].Price > (1+revertDis)*tickRelated.Bids[0].Price {
			symbol = settingSymbol
			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
		} else if math.Abs(amountPerp) < math.Abs(amountRelated) && amount < balance.AvailableWithBorrow {
			symbol = symbolRelated
			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
		} else {
			symbol = settingSymbol
			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
		}
	} else {
		orderSide = model.OrderSideBuy
		if tickPerp.Asks[0].Price < (1-revertDis)*tickRelated.Asks[0].Price {
			symbol = settingSymbol
			price = tickPerp.Asks[0].Price * (1 + OrderPriceLimit)
		} else if tickPerp.Asks[0].Price > (1+revertDis)*tickRelated.Asks[0].Price {
			symbol = symbolRelated
			price = tickRelated.Asks[0].Price * (1 + OrderPriceLimit)
		} else if math.Abs(amountPerp) > math.Abs(amountRelated) {
			symbol = settingSymbol
			price = tickPerp.Asks[0].Price * (1 + OrderPriceLimit)
		} else {
			symbol = symbolRelated
			price = tickRelated.Asks[0].Price * (1 + OrderPriceLimit)
		}
		usdBalance := getCarryBalance(key, `USD`)
		if symbol == symbolRelated && (usdBalance != nil && usdBalance.Borrow > 0) {
			amount = 0
		}
	}
	amount = math.Min(math.Abs(amount), 20000/price)
	if amount <= 0 {
		return
	}
	orderAmount := api.GetAmountInPerpOKEX(setting.Market, symbol, math.Abs(amount))
	if orderAmount > 0 {
		resultPerp := api.CancelOrders(key, secret, setting.Market, settingSymbol)
		resultRelated := api.CancelOrders(key, secret, setting.Market, symbolRelated)
		util.Notice(fmt.Sprintf(`%s cancel all perp:%v related:%v >>>>>> equal %s %f, %s %f = %s %f`,
			setting.Market, resultPerp, resultRelated, settingSymbol, amountPerp, symbolRelated, amountRelated, orderSide, amount))
		api.PlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, symbol, symbol,
			``, ``, model.FunctionComplement, price, price, amount, true, nil)
	}
	return
}

func calcCarryOpen(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, symbolHigh, symbolLow string,
	scoreOpen, scoreClose, scoreHigh, scoreLow float64) (sidePerp, sideRelated string, amount float64) {
	var bidAmount, askAmount float64
	valueLow := setting.AmountLimit
	usdRate := getUsdRate(key)
	usdAvailable := getUsdAvailable(key)
	coin := model.GetCoin(setting.Market, setting.Symbol)
	balance := getCarryBalance(key, coin)
	fundingRate := 0.0
	if setting.Market == model.OKEX {
		fundingRate, _ = api.GetFundingRate(setting.Market, setting.Symbol)
		fundingRate *= 0.9
	}
	if balance == nil {
		model.SetCarryInfo(`warning `+coin, fmt.Sprintf(`slave: balace not available!!! %s`, key))
		model.SetCarryInfos(`coin_absent`, key+`_`+coin, map[string]interface{}{`absent`: coin, `key`: key})
		initEmptyBalance(key, setting.Market, coin)
		time.Sleep(time.Second / 6)
		return ``, ``, 0
	} else {
		model.RemoveCarryInfo(`warning ` + coin)
		model.RemoveCarryInfos(`coin_absent`, key+`_`+coin)
	}
	balanceAllValue := getBalanceAll(key)
	if balanceAllValue == 0 {
		return
	}
	coinRate := math.Abs(balance.UsdValue) / balanceAllValue
	jump := 5.0
	jumpRevert := 5.0
	setOpen := math.Max((1.5-usdRate)*setting.OpenShortMargin*(0.5+jump*coinRate), 0.003) - fundingRate
	setClose := -1.0
	if setting.Market == model.OKEX {
		setClose = math.Min(setting.CloseShortMargin*(0.5+jump*coinRate), -0.003) - fundingRate
	}
	revertOpen := math.Abs(setting.GridPriceDistance) * (usdRate - 0.5)
	if revertOpen > 0 {
		revertOpen = revertOpen / (1 + jumpRevert*coinRate)
	} else {
		revertOpen = revertOpen / (1 - math.Min(0.9, jumpRevert*coinRate))
	}
	revertOpen = math.Max(revertOpen, -0.003) + fundingRate
	revertClose := math.Max(-0.0005/(1-math.Min(0.9, jumpRevert*coinRate)), -0.003) - fundingRate
	usdLowLine := model.AppConfig.Amount
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	localOpenValueLimit := math.Min(openValueLimit, 0.5*balanceAllValue)
	table := fmt.Sprintf(`%s_dynamic_`, model.FunctionCarry)
	if len(keys) > 1 && keys[0] != key {
		table += `slave`
		localOpenValueLimit = model.AppConfig.SimonOpenMax
		usdLowLine = model.AppConfig.SimonUsdLow
		valueLow = 0
		carryCloses := model.AppConfig.GetCarryClose()
		if len(carryCloses) > 1 && carryCloses[1] == `true` {
			setOpen = 1
			setClose = -1
		}
	}
	model.SetCarryInfo(table+setting.Symbol,
		fmt.Sprintf(`%s 参数:(%f %f %f) 计算结果(%f %f %f %f) 当前市场(%f %f) usdRate:%favailable:%f coinRate:%f 资金费率 %f`,
			table, setting.OpenShortMargin, setting.CloseShortMargin, setting.GridPriceDistance, setOpen, setClose,
			revertOpen, revertClose, scoreOpen, scoreClose, usdRate, usdAvailable, balance.UsdValue/balanceAllValue, fundingRate))
	carryInfo := map[string]interface{}{`+开仓`: setting.OpenShortMargin, `-开仓`: setting.CloseShortMargin,
		`平仓`: setting.GridPriceDistance, `动态+开仓`: setOpen, `动态-开仓`: setClose, `open平仓`: revertOpen,
		`close平仓`: revertClose, table: setting.Symbol, `市场+开`: scoreOpen, `市场-开`: scoreClose, `usdRate`: usdRate,
		`usdAvailable`: usdAvailable, `coinRate`: balance.UsdValue / balanceAllValue, `fundingRate`: fundingRate}
	model.SetCarryInfos(table, setting.Symbol, carryInfo)
	carryAmount := getCarryAmount(key, setting.Symbol)
	if (scoreLow < setClose && setting.Symbol == symbolLow) || (carryAmount > 0 && scoreClose <= -1*revertOpen) {
		bidAmount = tickPerp.Asks[0].Amount
		askAmount = tickRelated.Bids[0].Amount
		sidePerp = model.OrderSideBuy
		sideRelated = model.OrderSideSell
	} else if (scoreHigh > setOpen && setting.Symbol == symbolHigh) || (carryAmount < 0 && scoreOpen >= revertClose) {
		bidAmount = tickRelated.Asks[0].Amount
		askAmount = tickPerp.Bids[0].Amount
		sidePerp = model.OrderSideSell
		sideRelated = model.OrderSideBuy
	}
	markPrice := tickPerp.Asks[0].Price
	amount = math.Min(bidAmount, askAmount)
	// 开仓时:数量<持仓+可借
	if (setting.Symbol == symbolLow && scoreLow < setClose) || (setting.Symbol == symbolHigh && scoreHigh > setOpen) {
		if sideRelated == model.OrderSideSell {
			if balance.AvailableWithBorrow == 0 {
				util.Notice(fmt.Sprintf(`with borrow %s %f`, setting.Symbol, balance.AvailableWithBorrow))
			}
			amount = math.Min(balance.AvailableWithBorrow, math.Abs(amount))
		}
	} else { // 反向关仓量要<=持仓
		amount = math.Min(math.Abs(carryAmount), amount)
	}
	if sideRelated == model.OrderSideBuy {
		amount = math.Min(amount, usdAvailable/markPrice)
	}
	amount = math.Min(amount, localOpenValueLimit/markPrice)
	// usd所剩太少且还要再买 || 反向持仓太多且还要再卖 || 下单太小
	if (sideRelated == model.OrderSideBuy && (usdAvailable < usdLowLine || (balance.UsdValue > 0 && coinRate > 0.5))) ||
		(sideRelated == model.OrderSideSell && (balance.UsdValue < 0 && coinRate > 0.5)) ||
		math.Abs(amount)*markPrice < valueLow {
		amount = 0
	}
	amount = api.FormatAmountPair(setting.Market, setting.Symbol, setting.GetRelatedSymbol(), amount)
	if model.OKEX == setting.Market {
		amountInPerp := api.GetAmountInPerpOKEX(setting.Market, setting.Symbol, amount)
		maxBuyPerp, maxSellPerp := getTradeMax(key, setting.Symbol)
		maxBuyRelated, maxSellRelated := getTradeMax(key, setting.GetRelatedSymbol())
		if sidePerp == model.OrderSideBuy && sideRelated == model.OrderSideSell {
			amountInPerp = math.Min(amountInPerp, maxBuyPerp)
			amount = math.Min(amount, maxSellRelated)
		} else if sidePerp == model.OrderSideSell && sideRelated == model.OrderSideBuy {
			amountInPerp = math.Min(amountInPerp, maxSellPerp)
			amount = math.Min(amount, maxBuyRelated)
		}
		_, amountInReal := api.ParseRealAmount(setting.Market, setting.Symbol, amountInPerp)
		amount = math.Min(amount, amountInReal)
		amount = api.FormatAmountPair(setting.Market, setting.Symbol, setting.GetRelatedSymbol(), amount)
	}
	if amount > 0 {
		util.Notice(fmt.Sprintf(`+++ %s high:%s %f low:%s %f symbol: %s %s usd available:%f amount %f carryAmount: %f`,
			key, symbolHigh, scoreHigh, symbolLow, scoreLow, setting.Symbol, sidePerp, usdAvailable, amount, carryAmount))
	}
	return sidePerp, sideRelated, amount
}

//var cachePositions = make(map[string][]*model.Position)       // key - []position
//func setPositions(key string, value []*model.Position) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	cachePositions[key] = value
//}
//func getPositions(key string) []*model.Position {
//	defer carryLock.Unlock()
//	carryLock.Lock()
//	return cachePositions[key]
//}

func initEmptyBalance(key, market, coin string) (balance *model.Balance) {
	balance = &model.Balance{
		Amount:   0,
		Borrow:   0,
		Coin:     coin,
		Market:   market,
		Price:    0,
		UsdValue: 0,
	}
	keys, secrets := model.AppConfig.GetKeys(market)
	for i, current := range keys {
		if current == key {
			success, maxLoan := api.GetMaxLoan(key, secrets[i], market, coin)
			balance.AvailableWithBorrow = maxLoan
			if success {
				setCarryBalance(key, coin, balance)
			}
			util.Notice(fmt.Sprintf(`need to set empty balance for %s %s %v`, market, coin, success))
		}
	}
	return balance
}

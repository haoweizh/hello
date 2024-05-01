package deprecated

//
//import (
//	"fmt"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"strconv"
//	"strings"
//	"sync"
//	"time"
//)

//func initCombineAmount(turtleData *TurtleData, setting *model.Setting, currentNum int64) (amountBuy, amountSell float64) {
//	amountLimit := int64(setting.AmountLimit)
//	coinLimit := int64(setting.OpenShortMargin)
//	if turtleData.orderLong == nil && (setting.Chance < 0 || (currentNum < amountLimit && setting.Chance < coinLimit &&
//		setting.SymbolRelated != model.SettingTurtleRemoved)) {
//		if setting.Chance < 0 {
//			amountBuy = setting.GridAmount
//		} else {
//			amountBuy = turtleData.amount
//		}
//	}
//	if turtleData.orderShort == nil && (setting.Chance > 0 || (currentNum > -1*amountLimit && setting.Chance > -1*coinLimit &&
//		setting.SymbolRelated != model.SettingTurtleRemoved)) {
//		if setting.Chance > 0 {
//			amountSell = setting.GridAmount
//		} else {
//			amountSell = turtleData.amount
//		}
//	}
//	return
//}
//
//// filterCombineAmount
//// price,priceOpposite,amount,amountOpposite本方、另外的海龟相反方向的下单价格、数量
//func filterCombineAmount(market, symbol string, price, priceOpposite, amount, amountOpposite float64) (
//	amountFiltered, amountOppositeFiltered float64) {
//	minSize := 0.0
//	marketInfo := model.GetMarketInfo(market, symbol)
//	if marketInfo == nil {
//		util.Notice(`fail to get marketInfo %s %s`, market, symbol)
//	} else {
//		if marketInfo.CTValue == 0 {
//			minSize = marketInfo.SizeMin
//		} else {
//			minSize = marketInfo.SizeMin * marketInfo.CTValue
//		}
//	}
//	if amount > 0 && price == priceOpposite && amount == amountOpposite {
//		return minSize, minSize
//	}
//	return amount, amountOpposite
//}
//func clearCarryBalance() {
//	for doCarry {
//		for true {
//			if !checkSetCarrying(true) {
//				break
//			} else {
//				time.Sleep(time.Millisecond * 200)
//			}
//		}
//		util.Notice(`...... enter clearing carry balance`)
//		time.Sleep(time.Minute * 5)
//		markets := model.GetMarkets()
//		for _, market := range markets {
//			settings := model.GetSettings(model.FunctionCarry, market)
//			keys, secrets := model.AppConfig.GetKeys(market)
//			for i, key := range keys {
//				resultBalance, balances, _, margin := api.GetBalances(key, secrets[i], market, 0)
//				if market == model.OKEX {
//					setMarginOKEX(key, margin)
//				}
//				resultPosition, positions := api.GetPositions(key, secrets[i], market)
//				if !resultBalance || !resultPosition {
//					util.Notice(`fatal error: can not get balance/position ` + market)
//					continue
//				}
//				balanceAllValue := 0.0
//				localUsdAvailable := 0.0
//				borrowAll := 0.0
//				for _, value := range balances {
//					coin := strings.ToUpper(value.Coin)
//					if market == model.OKEX { // 针对okex不能从balance获取可借数的问题进行特殊处理
//						preBalance := getCarryBalance(key, coin)
//						if preBalance != nil {
//							value.AvailableWithBorrow = preBalance.AvailableWithBorrow
//						}
//					}
//					setCarryBalance(key, coin, value)
//					settingCoins := model.GetSettingCoins(model.FunctionCarry, market)
//					if settingCoins[coin] {
//						balanceAllValue += value.UsdValue
//					}
//					if (coin == `USD` && market == model.Ftx) || (coin == `USDT` && market == model.OKEX) {
//						localUsdAvailable = value.Amount
//						balanceAllValue += value.Amount
//						borrowAll += value.Borrow
//					} else {
//						symbolPerp := coin + api.GetPerpTail(market)
//						success, bidAsk := model.AppMarkets.GetBidAsk(coin+api.GetSpotTail(market), market)
//						if settings[symbolPerp] != nil && len(settings[symbolPerp]) > 0 && success {
//							borrowAll += value.Borrow * bidAsk.Bids[0].Price
//						} else {
//							util.Notice(fmt.Sprintf(`fatal: can not get price %s %s`, market, coin))
//						}
//					}
//				}
//				localUsdAvailable = localUsdAvailable - borrowAll
//				setUsdAvailable(key, localUsdAvailable)
//				setUsdRate(key, localUsdAvailable/balanceAllValue)
//				setBalanceAll(key, balanceAllValue)
//				util.Notice(fmt.Sprintf(`[carry] %s usd:%f %f len(balances):%d`,
//					key, localUsdAvailable, usdRate[key], len(balances)))
//				for _, items := range settings {
//					for _, item := range items {
//						makeEqual(key, secrets[i], item, balances, positions)
//					}
//				}
//				if time.Now().Unix()-tradeMaxResetTime > 600 {
//					resetTradeMax(keys, secrets, model.OKEX)
//				}
//				initEmptyBalance(key, secrets[i], market)
//			}
//		}
//		util.Notice(`...... exit clearing carry balance`)
//		checkSetCarrying(false)
//		time.Sleep(time.Second * 60)
//	}
//}
//
//var ProcessSpotCarry = func(setting *model.Setting, tick *model.BidAsk) {
//	_, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
//	symbolRelated := setting.SymbolRelated
//	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
//	now := time.Now()
//	million := util.GetNowUnixMillion()
//	if tick == nil || tickRelated == nil || tick.Asks == nil || tick.Bids == nil ||
//		tickRelated.Asks == nil || tickRelated.Bids == nil || setting == nil ||
//		model.AppPause || (model.AppConfig.Env != `test` && (model.AppConfig.HandleLink != `1` ||
//		(million-int64(tickRelated.Ts) > 2000 || million-int64(tick.Ts) > 2000 || million-int64(tick.Ts) > 25))) {
//		return
//	}
//	scoreOpen := 1 - tickRelated.Asks[0].Price/tickPerp.Bids[0].Price
//	scoreClose := 1 - tickRelated.Bids[0].Price/tickPerp.Asks[0].Price
//	model.AppMetric.AddCarry(setting.Market, setting.Symbol, scoreOpen, scoreClose)
//	if math.IsNaN(highest) || scoreOpen > highest || setting.Symbol == symbolHighest {
//		highest = scoreOpen
//		symbolHighest = setting.Symbol
//		model.AppMetric.AddCarry(setting.Market, setting.Market+`开仓价差++++`, highest, math.NaN())
//	}
//	if math.IsNaN(lowest) || scoreClose < lowest || setting.Symbol == symbolLowest {
//		lowest = scoreClose
//		symbolLowest = setting.Symbol
//		model.AppMetric.AddCarry(setting.Market, setting.Market+`开仓价差----`, math.NaN(), lowest)
//	}
//	model.SetCarryInfo(`[current high-low]`, fmt.Sprintf(`highest %s %f lowest %s %f`, symbolHighest, highest, symbolLowest, lowest))
//	keys, secrets := model.AppConfig.GetKeys(setting.Market)
//	doReverts := strings.Split(model.AppConfig.CarryClose, `,`)
//	begin := 0
//	step := 1
//	if (now.Hour() < 6 && now.Hour() > 2 && now.Second()%4 != 0) || now.Second()%2 == 0 {
//		begin = len(keys) - 1
//		step = -1
//	}
//	for i := begin; i >= 0 && i < len(keys); i += step {
//		sidePerp, sideRelated, amount, carryType := calcCarryOpen(setting, tickPerp, tickRelated, keys[i],
//			doReverts[i], scoreOpen, scoreClose, scoreOpen, scoreClose)
//		if amount > 0 {
//			go placeCarry(setting, tickPerp, tickRelated, keys[i], secrets[i], sidePerp, sideRelated, carryType,
//				scoreOpen, scoreClose, amount)
//			break
//		}
//	}
//}
//
//func placeCarry(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, secret, sidePerp,
//	sideRelated, carryType string, scoreOpen, scoreClose, amount float64) {
//	if !checkSetCarrying(true) {
//		defer checkSetCarrying(false)
//	} else {
//		util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
//		return
//	}
//	symbolRelated := setting.GetRelatedSymbol()
//	coin := model.GetCoin(setting.Market, setting.Symbol)
//	balance := getCarryBalance(key, coin)
//	if balance == nil {
//		util.Notice(fmt.Sprintf(`no coin balance %s %s`, coin, key))
//		return
//	}
//	perpPrice := tickPerp.Asks[0].Price
//	relatedPrice := tickRelated.Bids[0].Price
//	usdAvailable := getUsdAvailable(key)
//	balanceAllValue := getBalanceAll(key)
//	if sidePerp == model.OrderSideSell {
//		perpPrice = tickPerp.Bids[0].Price
//		relatedPrice = tickRelated.Asks[0].Price
//		setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)+amount)
//		balance.Amount += amount
//		balance.AvailableWithBorrow += amount
//		balance.UsdValue += amount * perpPrice
//		if carryType == carryTypeOpen {
//			usdAvailable -= amount * perpPrice
//			setUsdAvailable(key, usdAvailable)
//		}
//		setCarryBalance(key, coin, balance)
//		setUsdRate(key, usdAvailable/balanceAllValue)
//	} else if sidePerp == model.OrderSideBuy {
//		perpPrice = tickPerp.Asks[0].Price
//		relatedPrice = tickRelated.Bids[0].Price
//		setCarryAmount(key, setting.Symbol, getCarryAmount(key, setting.Symbol)-amount)
//		balance.Amount -= amount
//		balance.AvailableWithBorrow -= amount
//		balance.UsdValue -= amount * perpPrice
//		if carryType == carryTypeRevert {
//			usdAvailable += amount * relatedPrice
//			setUsdAvailable(key, usdAvailable)
//		}
//		setCarryBalance(key, coin, balance)
//		setUsdRate(key, usdAvailable/balanceAllValue)
//	}
//	now := int(util.GetNowUnixMillion())
//	util.Notice(fmt.Sprintf(`carry%s->%s delay %d %d perp[%f %f %f %f] related[%f %f %f %f] with score open:%f close:%f
//	    amount %f worth %f time in million %d`,
//		setting.Symbol, symbolRelated, now-tickPerp.TsReceived, now-tickRelated.TsReceived, tickPerp.Bids[0].Price,
//		tickPerp.Bids[0].Amount, tickPerp.Asks[0].Price, tickPerp.Asks[0].Amount, tickRelated.Bids[0].Price,
//		tickRelated.Bids[0].Amount, tickRelated.Asks[0].Price, tickRelated.Asks[0].Amount, scoreOpen, scoreClose,
//		amount, amount*tickPerp.Asks[0].Price, util.GetNowUnixMillion()))
//	if setting.Market == model.OKEX {
//		api.PlacePairOKEX(key, model.GetCoin(setting.Market, setting.Symbol), sidePerp, sideRelated,
//			model.OrderTypeLimit, perpPrice, relatedPrice, amount)
//	} else {
//		go api.PlaceOrder(key, secret, sidePerp, model.OrderTypeLimit, setting.Market, setting.Symbol,
//			``, ``, model.FunctionCarry, perpPrice, perpPrice,
//			amount, true, true, postOrderCarry)
//		api.PlaceOrder(key, secret, sideRelated, model.OrderTypeLimit, setting.Market, symbolRelated,
//			``, ``, model.FunctionCarry, relatedPrice, relatedPrice,
//			amount, true, true, postOrderCarry)
//		time.Sleep(time.Second / 5)
//	}
//}
//
//func getCarryAmounts(setting *model.Setting, balances []*model.Balance, positions []*model.Position) (
//	success bool, amountPerp, amountRelated float64) {
//	tail := api.GetPerpTail(setting.Market)
//	//positionExist := false
//	balanceExist := false
//	for _, position := range positions {
//		if position != nil && position.Currency == setting.Symbol {
//			amountPerp = position.Free
//			//positionExist = true
//		}
//	}
//	for _, balance := range balances {
//		if strings.ToUpper(balance.Coin+tail) == setting.Symbol {
//			amountRelated = balance.Amount
//			balanceExist = true
//		}
//	}
//	return balanceExist, amountPerp, amountRelated
//}
//
//func makeEqual(key, secret string, setting *model.Setting, balances []*model.Balance, positions []*model.Position) (
//	symbol string, price float64, equal bool) {
//	settingSymbol := setting.Symbol
//	coin := model.GetCoin(setting.Market, setting.Symbol)
//	symbolRelated := setting.GetRelatedSymbol()
//	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
//	_, tickRelated := model.AppMarkets.GetBidAsk(symbolRelated, setting.Market)
//	if tickPerp == nil || tickRelated == nil {
//		return ``, 0, true
//	}
//	success, amountPerp, amountRelated := getCarryAmounts(setting, balances, positions)
//	if !success {
//		return
//	}
//	amount := amountPerp + amountRelated
//	orderSide := model.OrderSideBuy
//	if amount < math.Max(math.Abs(amountPerp), math.Abs(amountRelated)) {
//		if amountPerp < 0 && amountRelated > 0 {
//			setCarryAmount(key, settingSymbol, math.Min(math.Abs(amountPerp), math.Abs(amountRelated)))
//		} else if amountPerp > 0 && amountRelated < 0 {
//			setCarryAmount(key, settingSymbol, -1*math.Min(math.Abs(amountPerp), math.Abs(amountRelated)))
//		}
//	} else {
//		setCarryAmount(key, settingSymbol, 0)
//	}
//	balance := getCarryBalance(key, coin)
//	if amount > 0 {
//		orderSide = model.OrderSideSell
//		if tickPerp.Bids[0].Price < (1-revertDis)*tickRelated.Bids[0].Price && amount < balance.AvailableWithBorrow {
//			symbol = symbolRelated
//			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
//		} else if tickPerp.Bids[0].Price > (1+revertDis)*tickRelated.Bids[0].Price {
//			symbol = settingSymbol
//			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
//		} else if math.Abs(amountPerp) < math.Abs(amountRelated) && amount < balance.AvailableWithBorrow {
//			symbol = symbolRelated
//			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
//		} else {
//			symbol = settingSymbol
//			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
//		}
//	} else {
//		orderSide = model.OrderSideBuy
//		if tickPerp.Asks[0].Price < (1-revertDis)*tickRelated.Asks[0].Price {
//			symbol = settingSymbol
//			price = tickPerp.Asks[0].Price * (1 + OrderPriceLimit)
//		} else if tickPerp.Asks[0].Price > (1+revertDis)*tickRelated.Asks[0].Price {
//			symbol = symbolRelated
//			price = tickRelated.Asks[0].Price * (1 + OrderPriceLimit)
//		} else if math.Abs(amountPerp) > math.Abs(amountRelated) {
//			symbol = settingSymbol
//			price = tickPerp.Asks[0].Price * (1 + OrderPriceLimit)
//		} else {
//			symbol = symbolRelated
//			price = tickRelated.Asks[0].Price * (1 + OrderPriceLimit)
//		}
//		usdBalance := getCarryBalance(key, `USD`)
//		if symbol == symbolRelated && (usdBalance != nil && usdBalance.Borrow > 0) {
//			amount = 0
//		}
//	}
//	amount = math.Min(math.Abs(amount), 20000/price)
//	if amount <= 0 {
//		return
//	}
//	checkAmount := api.GetAmountInMarket(setting.Market, symbol, amount)
//	if checkAmount > 0 {
//		resultPerp := api.CancelOrders(key, secret, setting.Market, settingSymbol)
//		resultRelated := api.CancelOrders(key, secret, setting.Market, symbolRelated)
//		util.Notice(fmt.Sprintf(`%s cancel all perp:%v related:%v >>>>>> equal %s %f, %s %f = %s %f`,
//			setting.Market, resultPerp, resultRelated, settingSymbol, amountPerp, symbolRelated, amountRelated, orderSide, amount))
//		api.PlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, symbol, symbol,
//			``, model.FunctionComplement, price, price, amount, true, true, nil)
//	}
//	return
//}
//
//func initEmptyBalance(key, secret, market string) {
//	now := util.GetNow().Unix()
//	if now-marketInitTime[key] < 600 {
//		return
//	} else {
//		marketInitTime[key] = now
//	}
//	coins := api.GetCarryCoins()
//	if coins == nil || coins[market] == nil {
//		return
//	}
//	for coin := range coins[market] {
//		balance := getCarryBalance(key, coin)
//		if balance == nil {
//			balance = &model.Balance{Coin: coin, Market: market}
//		}
//		if market == model.OKEX {
//			success, maxLoan := api.GetMaxLoan(key, secret, market, coin)
//			if success {
//				balance.AvailableWithBorrow = maxLoan + math.Max(0, balance.Amount)
//			}
//			time.Sleep(time.Second / 8)
//		}
//		setCarryBalance(key, coin, balance)
//	}
//	util.Notice(fmt.Sprintf(`set available with borrow %s %s`, market, key))
//}
//
//func calcCarryOpen(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, doRevert string,
//	scoreOpen, scoreClose, scoreHigh, scoreLow float64) (sidePerp, sideRelated string, amount float64, carryType string) {
//	var bidAmount, askAmount float64
//	valueLow := setting.AmountLimit
//	usdRate := getUsdRate(key)
//	usdAvailable := getUsdAvailable(key)
//	coin := model.GetCoin(setting.Market, setting.Symbol)
//	balance := getCarryBalance(key, coin)
//	fundingRate := 0.0
//	if setting.Market == model.OKEX {
//		now := time.Now()
//		if now.Hour()%8 == 0 && now.Minute() == 0 && now.Second() < 30 {
//			return
//		}
//		fundingRate = api.GetFundingRate(setting.Market, setting.Symbol, &carryLock)
//		fundingRate *= 0.9
//	}
//	if balance == nil {
//		model.SetCarryInfo(`warning `+coin, fmt.Sprintf(`slave: balace not available!!! %s`, key))
//		model.SetCarryInfos(`coin_absent`, key+`_`+coin, map[string]interface{}{`absent`: coin, `key`: key})
//		return ``, ``, 0, carryType
//	} else {
//		model.RemoveCarryInfo(`warning ` + coin)
//		model.RemoveCarryInfos(`coin_absent`, key+`_`+coin)
//	}
//	balanceAllValue := getBalanceAll(key)
//	if balanceAllValue == 0 {
//		return
//	}
//	coinRate := math.Abs(balance.UsdValue) / balanceAllValue
//	jump := 5.0
//	jumpRevert := 5.0
//	setOpen := math.Max((1.5-usdRate)*setting.OpenShortMargin*(0.5+jump*coinRate), 0.003) - fundingRate
//	setClose := math.Min(setting.CloseShortMargin*(0.5+jump*coinRate), -0.003) - fundingRate
//	revertOpen := math.Abs(setting.GridPriceDistance) * (usdRate - 0.5)
//	if revertOpen > 0 {
//		revertOpen = revertOpen / (1 + jumpRevert*coinRate)
//	} else {
//		revertOpen = revertOpen / (1 - math.Min(0.9, jumpRevert*coinRate))
//	}
//	revertOpen = math.Max(revertOpen, -0.003) + fundingRate
//	revertClose := math.Max(-0.0005/(1-math.Min(0.9, jumpRevert*coinRate)), -0.003) - fundingRate
//	usdLowLine := model.AppConfig.Amount
//	keys, _ := model.AppConfig.GetKeys(setting.Market)
//	localOpenValueLimit := math.Min(openValueLimit, 0.5*balanceAllValue)
//	table := fmt.Sprintf(`%s_dynamic_`, model.FunctionCarry)
//	if len(keys) > 1 && keys[0] != key {
//		setOpen *= 0.8
//		setClose *= 0.8
//		table += fmt.Sprintf(`slave%s`, key[0:5])
//		if usdRate > 0 {
//			usdLowLine = 0.2 * usdAvailable / usdRate
//			localOpenValueLimit = math.Min(usdLowLine, openValueLimit)
//		} else {
//			localOpenValueLimit = 0
//		}
//		valueLow = 0
//	}
//	if setting.Market == model.OKEX {
//		margin := getMarginOKEX(key)
//		if margin < usdLowLine {
//			doRevert = `true`
//		}
//		if setting.Symbol == `IOTA-USDT-SWAP` {
//			setOpen = 0.02 - fundingRate
//			setClose = 0 - fundingRate
//			revertOpen = -0.01 + fundingRate
//			revertClose = 0.01 - fundingRate
//		}
//	}
//	if doRevert == `true` {
//		setOpen = 1
//		setClose = -1
//		revertOpen = -0.003
//		revertClose = -0.003
//	}
//	carryAmount := getCarryAmount(key, setting.Symbol)
//	if scoreLow < setClose || (carryAmount > 0 && scoreClose <= -1*revertOpen) {
//		bidAmount = tickPerp.Asks[0].Amount
//		if setting.Market == model.OKEX {
//			_, bidAmount = api.ParseRealAmount(setting.Market, setting.Symbol, bidAmount)
//		}
//		askAmount = tickRelated.Bids[0].Amount
//		sidePerp = model.OrderSideBuy
//		sideRelated = model.OrderSideSell
//		if scoreLow < setClose {
//			carryType = carryTypeClose
//		} else {
//			carryType = carryTypeRevert
//		}
//	} else if scoreHigh > setOpen || (carryAmount < 0 && scoreOpen >= revertClose) {
//		bidAmount = tickRelated.Asks[0].Amount
//		askAmount = tickPerp.Bids[0].Amount
//		if setting.Market == model.OKEX {
//			_, askAmount = api.ParseRealAmount(setting.Market, setting.Symbol, askAmount)
//		}
//		sidePerp = model.OrderSideSell
//		sideRelated = model.OrderSideBuy
//		if scoreHigh > setOpen {
//			carryType = carryTypeOpen
//		} else {
//			carryType = carryTypeRevert
//		}
//	}
//	markPrice := tickPerp.Asks[0].Price
//	amount = math.Min(bidAmount, askAmount)
//	// 开仓时:数量<持仓+可借
//	if scoreLow < setClose || scoreHigh > setOpen {
//		if sideRelated == model.OrderSideSell {
//			amount = math.Min(balance.AvailableWithBorrow, math.Abs(amount))
//		}
//	} else { // 反向关仓量要<=持仓
//		amount = math.Min(math.Abs(carryAmount), amount)
//	}
//	if sideRelated == model.OrderSideBuy {
//		amount = math.Min(amount, usdAvailable/markPrice)
//	}
//	amount = math.Min(amount, localOpenValueLimit/markPrice)
//	// usd所剩太少且还要再买 || 反向持仓太多且还要再卖 || 下单太小
//	if (sideRelated == model.OrderSideBuy && (usdAvailable < usdLowLine || (balance.UsdValue > 0 && coinRate > 0.5))) ||
//		(sideRelated == model.OrderSideSell && (balance.UsdValue < 0 && coinRate > 0.5)) ||
//		math.Abs(amount)*markPrice < valueLow {
//		amount = 0
//	}
//	amount = api.FormatAmountPair(setting.Market, setting.Symbol, setting.GetRelatedSymbol(), amount)
//	if model.OKEX == setting.Market {
//		amountInPerp := api.GetAmountInMarket(setting.Market, setting.Symbol, amount)
//		maxBuyPerp, maxSellPerp := getTradeMax(key, setting.Symbol)
//		maxBuyRelated, maxSellRelated := getTradeMax(key, setting.GetRelatedSymbol())
//		if sidePerp == model.OrderSideBuy && sideRelated == model.OrderSideSell {
//			amountInPerp = math.Min(amountInPerp, maxBuyPerp)
//			amount = math.Min(amount, maxSellRelated)
//		} else if sidePerp == model.OrderSideSell && sideRelated == model.OrderSideBuy {
//			amountInPerp = math.Min(amountInPerp, maxSellPerp)
//			amount = math.Min(amount, maxBuyRelated)
//		}
//		_, amountInReal := api.ParseRealAmount(setting.Market, setting.Symbol, amountInPerp)
//		amount = math.Min(amount, amountInReal)
//		amount = api.FormatAmountPair(setting.Market, setting.Symbol, setting.GetRelatedSymbol(), amount)
//	}
//	if amount > 0 {
//		util.Notice(fmt.Sprintf(`+++ usdRate: %f coinRate: %f %s high: %f low: %f symbol: %s %s
//			usd available:%f amount %f carryAmount: %f scoreHigh: %f setOpen: %f scoreLow: %f setClose: %f
//			revertOpen: %f revertClose: %f do revert: %s`,
//			usdRate, coinRate, key, scoreHigh, scoreLow, setting.Symbol, sidePerp, usdAvailable, amount, carryAmount,
//			scoreHigh, setOpen, scoreLow, setClose, revertOpen, revertClose, doRevert))
//	}
//	msg := setting.Symbol
//	if keys[0] != key {
//		msg = key[0:5] + msg
//	}
//	model.SetCarryInfo(table+setting.Symbol,
//		fmt.Sprintf("%s\n%f %f %f usdAva:%s usdRate:%s 计算%s %s %s %s 市场%s %s 资金费率:%s coinRate:%s 可用:%s ",
//			msg, setting.OpenShortMargin, setting.CloseShortMargin, setting.GridPriceDistance,
//			strconv.FormatFloat(usdAvailable, 'f', 0, 64),
//			strconv.FormatFloat(100*usdRate, 'f', 0, 64)+"%",
//			strconv.FormatFloat(setOpen, 'f', 4, 64),
//			strconv.FormatFloat(setClose, 'f', 4, 64),
//			strconv.FormatFloat(revertOpen, 'f', 4, 64),
//			strconv.FormatFloat(revertClose, 'f', 4, 64),
//			strconv.FormatFloat(scoreOpen, 'f', 4, 64),
//			strconv.FormatFloat(scoreClose, 'f', 4, 64),
//			strconv.FormatFloat(fundingRate*1000, 'f', 2, 64)+"‰",
//			strconv.FormatFloat(100*balance.UsdValue/balanceAllValue, 'f', 1, 64)+"%",
//			strconv.FormatFloat(balance.AvailableWithBorrow, 'f', 2, 64)))
//	carryInfo := map[string]interface{}{`01.动态正开仓`: setOpen, `02.动态负开仓`: setClose, `03.动态平仓`: revertOpen,
//		`04.动态平仓`: revertClose, `0.5市场开仓`: scoreOpen, `06.市场关仓`: scoreClose, `07.usd rate`: usdRate,
//		`08.usd available`: usdAvailable, `09. coin rate`: balance.UsdValue / balanceAllValue,
//		`10.可用`: balance.AvailableWithBorrow, `11.资金费率`: fundingRate, `12.` + table: setting.Symbol,
//		`13.正开仓`: setting.OpenShortMargin, `14.负开仓`: setting.CloseShortMargin, `15.平仓`: setting.GridPriceDistance}
//	model.SetCarryInfos(table, setting.Symbol, carryInfo)
//	return sidePerp, sideRelated, amount, carryType
//}

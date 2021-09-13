package carry

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const OrderPriceLimit = 0
const revertDis = 0.005
const openValueLimit = 10000.0
const carryTypeOpen = `carryOpen`
const carryTypeClose = `carryClose`
const carryTypeRevert = `carryRevert`
const InsufficientCodeBinance = `-2010`

var InsufficientCodeOKEX = map[string]bool{`51008`: true, `51119`: true, `51120`: true, `51131`: true, `51502`: true, `58350`: true, `59108`: true, `59200`: true}
var marketInitTime = make(map[string]int64) // market - initTime
var carryLock sync.Mutex
var carrying bool
var doCarry = false
var symbolHighest, symbolLowest string
var lowest = math.NaN()
var highest = math.NaN()

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

// 专用于处理ok可买卖数量限制
var postOrderCarry = func(order *model.Order) {
	if order == nil {
		return
	}
	unknownFail := true
	if order.OrderId == `` && (order.ErrCode == `` || order.ErrCode == `0`) {
		if order.Market == `` || order.Market == model.OKEX {
			keys, secrets := model.AppConfig.GetKeys(model.OKEX)
			for i, key := range keys {
				if key == order.AmountType && InsufficientCodeOKEX[order.ErrCode] {
					util.Notice(`reset okex trade max with %s %s`, order.ErrCode, order.AmountType)
					resetTradeMax(key, secrets[i], model.OKEX)
					unknownFail = false
				}
			}
		} else if order.Market == model.Binance {
			keys, secrets := model.AppConfig.GetKeys(model.Binance)
			for i, key := range keys {
				if key == order.AmountType && strings.Contains(InsufficientCodeBinance, order.ErrCode) {
					util.Notice(`reset binance trade max with %s %s`, order.ErrCode, order.AmountType)
					clearCarry(model.Binance, key, secrets[i])
					unknownFail = false
				}
			}
		}
	} else {
		unknownFail = false
		maxBuy, maxSell := getTradeMax(order.AmountType, order.Symbol)
		amount := model.GetAmountInMarket(model.OKEX, order.Instrument, order.Amount)
		if order.OrderSide == model.OrderSideBuy {
			maxBuy -= amount
			maxSell += amount
		} else if order.OrderSide == model.OrderSideSell {
			maxBuy += amount
			maxSell -= amount
		}
		setTradeMax(order.AmountType, order.Instrument, maxBuy, maxSell)
	}
	if unknownFail {
		addCarryResult(order.AmountType, false)
	} else {
		addCarryResult(order.AmountType, true)
	}
}

func resetTradeMax(key, secret string, market string) {
	if market == model.Ftx {
		return // ftx无需设置
	}
	if getTradeMaxResetting(key) {
		return
	}
	defer setTradeMaxResetting(key, false)
	setTradeMaxResetting(key, true)
	setTradeMaxResetTime(key, time.Now().Unix())
	util.Notice(fmt.Sprintf(`reset all trade max %s %s`, key, market))
	coins := model.GetCarryCoins()
	if coins == nil || coins[market] == nil {
		return
	}
	for coin := range coins[market] {
		switch market {
		case model.OKEX:
			symbolPerp := coin + model.GetPerpTail(market)
			symbolRelated := coin + model.GetSpotTail(market)
			_, maxBuy, maxSell := api.GetMaxSize(key, secret, symbolPerp)
			setTradeMax(key, symbolPerp, maxBuy, maxSell)
			_, maxBuy, maxSell = api.GetMaxSize(key, secret, symbolRelated)
			setTradeMax(key, symbolRelated, maxBuy, maxSell)
			time.Sleep(time.Second / 5)
		case model.Binance:
			//_, maxLoan := api.GetMaxLoan(key, secret, market, coin)
			//balance := getCarryBalance(key, coin)
			//if balance != nil {
			//	balance.AvailableWithBorrow = balance.Amount + maxLoan
			//	setCarryBalance(key, coin, balance)
			//}
		}
	}
}

// MARGIN_UMFUTURE 杠杆全仓钱包转向U本位合约钱包
// UMFUTURE_MARGIN U本位合约钱包转向杠杆全仓钱包
// 7:3和8:2是比例范围，超过范围自动平衡成7.5:2.5
func checkProcessTransfer(key, secret, market string) {
	switch market {
	case model.Binance, model.Huobi, model.Gate, model.Kucoin:
		balance := getBalanceAll(key)
		balancePos := getPosBal(key)
		if balance/balancePos > 4 {
			api.Transfer(key, secret, market, `MAIN_UMFUTURE`, 0.25*balance-0.75*balancePos)
		} else if balance/balancePos < 2.33 {
			api.Transfer(key, secret, market, `UMFUTURE_MAIN`, 0.75*balancePos-0.25*balance)
		}
	}
}

func clearCarry(market, key, secret string) {
	settings := model.GetSettings(model.FunctionCarry, market)
	settingCoins := model.GetSettingCoins(model.FunctionCarry, market)
	resultBalance, balances, _, collateral := api.GetBalances(key, secret, market)
	setCollateral(key, collateral)
	resultPosition, positions, posBalance := api.GetPositions(key, secret, market)
	setPosBal(key, posBalance)
	if !resultBalance || !resultPosition {
		util.Notice(`%s %s fatal error: can not get balance %v position %v`, key, market, resultBalance, resultPosition)
		return
	}
	balanceAllValue := 0.0
	localUsdAvailable := 0.0
	borrowAll := 0.0
	for _, value := range balances {
		coin := strings.ToUpper(value.Coin)
		if model.Huobi == market {
			coin = strings.ToLower(value.Coin)
		}
		success, bidAsk := model.AppMarkets.GetBidAsk(coin+model.GetSpotTail(market), market)
		if !success && settingCoins[coin] {
			util.Notice(`fail to get setting coin bid ask %s , return`, coin)
			continue
		}
		if market == model.OKEX { // 针对okex不能从balance获取可借数的问题进行特殊处理
			preBalance := getCarryBalance(key, coin)
			if preBalance != nil {
				value.AvailableWithBorrow = preBalance.AvailableWithBorrow
			}
		}
		setCarryBalance(key, coin, value)
		if coin == `BTC` && market == model.OKEX { // 把okex中btc价值的usd按一半计入
			localUsdAvailable += value.UsdValue / 2
			balanceAllValue += value.UsdValue / 2
		}
		if (coin == `USD` && market == model.Ftx) ||
			(coin == `USDT` && (market == model.OKEX || market == model.Binance || market == model.Gate || market == model.Kucoin)) ||
			(coin == `usdt` && market == model.Huobi) {
			localUsdAvailable += value.Amount
			balanceAllValue += value.Amount
			borrowAll += value.Borrow
		} else if settingCoins[coin] {
			if value.UsdValue > 0 {
				balanceAllValue += value.UsdValue
			} else {
				balanceAllValue += value.Amount * bidAsk.Bids[0].Price
			}
			borrowAll += value.Borrow * bidAsk.Bids[0].Price
		}
	}
	localUsdAvailable = localUsdAvailable - borrowAll
	setUsdAvailable(key, localUsdAvailable)
	setUsdRate(key, localUsdAvailable/balanceAllValue)
	setBalanceAll(key, balanceAllValue)
	util.Notice(fmt.Sprintf(`[carry] %s usd:%f %f len(balances):%d`,
		key, localUsdAvailable, usdRate[key], len(balances)))
	equalSettings := make(map[string]*model.Setting)
	for _, setting := range settings {
		equalSettings[setting[0].Symbol] = setting[0]
	}
	for _, setting := range equalSettings {
		makeEqual(key, secret, setting, balances, positions)
	}
	initEmptyBalance(key, secret, market)
	checkProcessTransfer(key, secret, market)
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
				clearCarry(market, key, secrets[i])
			}
			for i, key := range keys {
				localMaxResetTime := getTradeMaxResetTime(key)
				if time.Now().Unix()-localMaxResetTime > 600 {
					go resetTradeMax(key, secrets[i], market)
				}
			}
		}
		util.Notice(`...... exit clearing carry balance`)
		checkSetCarrying(false)
		time.Sleep(time.Second * 60)
	}
}

var ProcessCarry = func(setting *model.Setting, tick *model.BidAsk) {
	if !doCarry && model.AppConfig.Handle == `1` {
		go clearCarryBalance()
		doCarry = true
		return
	}
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
	million := util.GetNowUnixMillion()
	delayTick := int64(0)
	if tick != nil {
		delayTick = million - int64(tick.Ts)
	}
	delayPerp := int64(0)
	if tickPerp != nil {
		delayPerp = million - int64(tickPerp.Ts)
	}
	delayRelated := int64(0)
	if tickRelated != nil {
		delayRelated = million - int64(tickRelated.Ts)
	}
	exit := false
	if tickPerp == nil || tickRelated == nil || tickPerp.Asks == nil || tickPerp.Bids == nil ||
		tickRelated.Asks == nil || tickRelated.Bids == nil || setting == nil || model.AppPause ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) {
		exit = true
	}
	switch setting.Market {
	case model.Binance:
		if delayPerp > 100 || delayRelated > 100 {
			exit = true
		}
	case model.OKEX:
		if delayTick > 25 || delayRelated > 300 || delayPerp > 300 {
			exit = true
		}
	case model.Ftx:
		if delayTick > 95 || delayRelated > 300 || delayPerp > 300 {
			exit = true
		}
	case model.Huobi:
		if delayTick > 25 || delayRelated > 300 || delayPerp > 300 {
			exit = true
		}
	case model.Gate:
		if delayTick > 40 || delayRelated > 30000 || delayPerp > 30000 {
			exit = true
		}
	case model.Kucoin:
		if delayTick > 25 || delayRelated > 1000 || delayPerp > 1000 {
			exit = true
		}
	}
	if exit {
		return
	}
	scoreOpen := 1 - tickRelated.Asks[0].Price/tickPerp.Bids[0].Price
	scoreClose := 1 - tickRelated.Bids[0].Price/tickPerp.Asks[0].Price
	mark := fmt.Sprintf(`%s_%s<->%s_%s`, setting.Market, setting.Symbol, setting.MarketRelated, setting.SymbolRelated)
	model.AppMetric.AddCarry(mark, scoreOpen, scoreClose)
	if math.IsNaN(highest) || scoreOpen > highest || setting.Symbol == symbolHighest {
		highest = scoreOpen
		symbolHighest = setting.Symbol
		model.AppMetric.AddCarry(`开仓价差++++`, highest, math.NaN())
	}
	if math.IsNaN(lowest) || scoreClose < lowest || setting.Symbol == symbolLowest {
		lowest = scoreClose
		symbolLowest = setting.Symbol
		model.AppMetric.AddCarry(`开仓价差----`, math.NaN(), lowest)
	}
	model.SetCarryInfo(`[current high-low]`, fmt.Sprintf(`highest %s %f lowest %s %f time:%s`,
		symbolHighest, highest, symbolLowest, lowest, time.Now().String()))
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	doReverts := strings.Split(model.AppConfig.CarryClose, `,`)
	begin := len(keys) - 1
	step := -1
	//now := time.Now()
	//if (now.Hour() < 8 && now.Hour() > 2 && now.Second()%8 != 0) || now.Second()%3 == 0 {
	//	begin = len(keys) - 1
	//	step = -1
	//}
	for i := begin; i >= 0 && i < len(keys); i += step {
		sidePerp, sideRelated, amount, carryType := calcCarryOpen(setting, tickPerp, tickRelated, keys[i],
			doReverts[i], scoreOpen, scoreClose)
		if amount > 0 {
			go placeCarry(setting, tickPerp, tickRelated, keys[i], secrets[i], sidePerp, sideRelated, carryType,
				scoreOpen, scoreClose, amount)
			return
		}
	}
}

func placeCarry(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, secret, sidePerp,
	sideRelated, carryType string, scoreOpen, scoreClose, amount float64) {
	if !checkSetCarrying(true) {
		defer checkSetCarrying(false)
	} else {
		//util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
		return
	}
	coin := model.GetCoin(setting.Market, setting.Symbol)
	balance := getCarryBalance(key, coin)
	if balance == nil {
		util.Notice(fmt.Sprintf(`no coin balance %s %s`, coin, key))
		return
	}
	perpPrice := tickPerp.Asks[0].Price
	relatedPrice := tickRelated.Bids[0].Price
	now := int(util.GetNowUnixMillion())
	util.Notice(fmt.Sprintf(`carry%s->%s delay %d %d perp[%f %f %f %f] related[%f %f %f %f] with score open:%f close:%f 
	    amount %f worth %f time in million %d key %s`,
		setting.Symbol, setting.SymbolRelated, now-tickPerp.TsReceived, now-tickRelated.TsReceived, tickPerp.Bids[0].Price,
		tickPerp.Bids[0].Amount, tickPerp.Asks[0].Price, tickPerp.Asks[0].Amount, tickRelated.Bids[0].Price,
		tickRelated.Bids[0].Amount, tickRelated.Asks[0].Price, tickRelated.Asks[0].Amount, scoreOpen, scoreClose,
		amount, amount*tickPerp.Asks[0].Price, util.GetNowUnixMillion(), key))
	placeSuccess := true
	if sidePerp == model.OrderSideBuy && sideRelated == model.OrderSideSell {
		perpPrice = tickPerp.Asks[0].Price
		relatedPrice = tickRelated.Bids[0].Price
	} else if sidePerp == model.OrderSideSell && sideRelated == model.OrderSideBuy {
		perpPrice = tickPerp.Bids[0].Price
		relatedPrice = tickRelated.Asks[0].Price
	}
	if setting.Market == model.OKEX {
		placeSuccess = api.PlacePairOKEX(key, model.GetCoin(setting.Market, setting.Symbol), sidePerp, sideRelated,
			model.OrderTypeLimit, perpPrice, relatedPrice, amount)
	} else {
		go api.PlaceOrder(key, secret, sidePerp, model.OrderTypeLimit, setting.Market, setting.Symbol,
			``, ``, model.FunctionCarry, perpPrice, perpPrice,
			amount, true, true, postOrderCarry)
		api.PlaceOrder(key, secret, sideRelated, model.OrderTypeLimit, setting.Market, setting.SymbolRelated,
			``, ``, model.FunctionCarry, relatedPrice, relatedPrice,
			amount, true, true, postOrderCarry)
		time.Sleep(time.Second / 5)
	}
	if placeSuccess {
		usdAvailable := getUsdAvailable(key)
		balanceAllValue := getBalanceAll(key)
		if sidePerp == model.OrderSideSell {
			perpPrice = tickPerp.Bids[0].Price
			relatedPrice = tickRelated.Asks[0].Price
			balance.Amount += amount
			balance.AvailableWithBorrow += amount
			balance.UsdValue += amount * perpPrice
			if carryType == carryTypeOpen {
				usdAvailable -= amount * perpPrice
				setUsdAvailable(key, usdAvailable)
			}
		} else if sidePerp == model.OrderSideBuy {
			perpPrice = tickPerp.Asks[0].Price
			relatedPrice = tickRelated.Bids[0].Price
			balance.Amount -= amount
			balance.AvailableWithBorrow -= amount
			balance.UsdValue -= amount * perpPrice
			if carryType == carryTypeRevert {
				usdAvailable += amount * relatedPrice
				setUsdAvailable(key, usdAvailable)
			}
		}
		setCarryBalance(key, coin, balance)
		setUsdRate(key, usdAvailable/balanceAllValue)
	}
}

func getCarryAmounts(setting *model.Setting, balances []*model.Balance, positions []*model.Position) (
	relatedBalance *model.Balance, amountPerp, amountRelated float64) {
	tail := model.GetPerpTail(setting.Market)
	for _, position := range positions {
		if position != nil && position.Currency == setting.Symbol {
			amountPerp = position.Free
		}
	}
	for _, balance := range balances {
		if strings.ToUpper(balance.Coin+tail) == setting.Symbol || (setting.Market == model.Huobi && strings.ToLower(balance.Coin+tail) == setting.Symbol) {
			amountRelated = balance.Amount
			relatedBalance = balance
		}
	}
	return relatedBalance, amountPerp, amountRelated
}

func makeEqual(key, secret string, setting *model.Setting, balances []*model.Balance, positions []*model.Position) (
	symbol string, price float64, equal bool) {
	coin := model.GetCoin(setting.Market, setting.Symbol)
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.Market)
	if tickPerp == nil || tickRelated == nil {
		return ``, 0, true
	}
	balance, amountPerp, amountRelated := getCarryAmounts(setting, balances, positions)
	if balance == nil {
		if amountPerp != 0 {
			balance = &model.Balance{Coin: coin, Market: setting.Market}
		} else {
			//util.Notice(`func:makeEqual can not get balance %s %s`, key, coin)
			return
		}
	}
	usdAvailable := getUsdAvailable(key)
	amount := amountPerp + amountRelated
	orderSide := model.OrderSideBuy
	if amount > 0 { //现货数量多、合约数量少
		orderSide = model.OrderSideSell
		if tickPerp.Bids[0].Price < (1-revertDis)*tickRelated.Bids[0].Price && amount < balance.AvailableWithBorrow {
			symbol = setting.SymbolRelated
			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
		} else if tickPerp.Bids[0].Price > (1+revertDis)*tickRelated.Bids[0].Price {
			symbol = setting.Symbol
			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
		} else if math.Abs(amountPerp) < math.Abs(amountRelated) && amount < balance.AvailableWithBorrow {
			symbol = setting.SymbolRelated
			price = tickRelated.Bids[0].Price * (1 - OrderPriceLimit)
		} else {
			symbol = setting.Symbol
			price = tickPerp.Bids[0].Price * (1 - OrderPriceLimit)
		}
	} else if amount < 0 { //合约数量多、现货数量少
		orderSide = model.OrderSideBuy
		if tickPerp.Asks[0].Price < (1-revertDis)*tickRelated.Asks[0].Price {
			symbol = setting.Symbol
			price = tickPerp.Asks[0].Price * (1 + OrderPriceLimit)
		} else if tickPerp.Asks[0].Price > (1+revertDis)*tickRelated.Asks[0].Price && amount < usdAvailable/tickRelated.Asks[0].Price {
			symbol = setting.SymbolRelated
			price = tickRelated.Asks[0].Price * (1 + OrderPriceLimit)
		} else if math.Abs(amountPerp) > math.Abs(amountRelated) {
			symbol = setting.Symbol
			price = tickPerp.Asks[0].Price * (1 + OrderPriceLimit)
		} else {
			symbol = setting.SymbolRelated
			price = tickRelated.Asks[0].Price * (1 + OrderPriceLimit)
		}
		usdBalance := getCarryBalance(key, `USD`)
		if symbol == setting.SymbolRelated && (usdBalance != nil && usdBalance.Borrow > 0) {
			amount = 0
		}
	}
	amount = math.Min(math.Abs(amount), 20000/price)
	switch setting.Market {
	case model.Ftx:
		amount = math.Min(amount, 90000000)
	case model.Binance:
		if (symbol == setting.Symbol && price*amount < 5) || (symbol == setting.SymbolRelated && price*amount < 10) {
			util.Notice(fmt.Sprintf("binance can't order %s low fee: %f ", symbol, price*amount))
			amount = 0
		}
	case model.Gate:
		if price*amount < 1 {
			amount = 0
		}
	}
	if amount <= 0 {
		return
	}
	checkAmount := model.GetAmountInMarket(setting.Market, symbol, amount)
	if checkAmount > 0 {
		resultPerp := api.CancelOrders(key, secret, setting.Market, setting.Symbol)
		resultRelated := api.CancelOrders(key, secret, setting.Market, setting.SymbolRelated)
		util.Notice(fmt.Sprintf(`%s %s cancel all perp:%v related:%v >>>>>> equal %s %f, %s %f = %s %f`,
			key, setting.Market, resultPerp, resultRelated, setting.Symbol, amountPerp, setting.SymbolRelated, amountRelated, orderSide, amount))
		api.PlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, symbol, symbol,
			``, model.FunctionComplement, price, price, amount, true, true, nil)
		go api.SetBidAsk(key, secret, setting.Market, setting.Symbol)
	}
	return
}

func initEmptyBalance(key, secret, market string) {
	now := util.GetNow().Unix()
	if now-marketInitTime[key] < 600 {
		return
	} else {
		marketInitTime[key] = now
	}
	coins := model.GetCarryCoins()
	if coins == nil || coins[market] == nil {
		return
	}
	for coin := range coins[market] {
		balance := getCarryBalance(key, coin)
		if balance == nil {
			balance = &model.Balance{Coin: coin, Market: market}
		}
		if market == model.OKEX || market == model.Binance || market == model.Gate {
			success, maxLoan := api.GetMaxLoan(key, secret, market, coin)
			if success {
				balance.AvailableWithBorrow = maxLoan + math.Max(0, balance.Amount)
			}
			time.Sleep(time.Second / 8)
		}
		setCarryBalance(key, coin, balance)
	}
	util.Notice(fmt.Sprintf(`set available with borrow %s %s`, market, key))
}

// revertOpen: 已经正向开仓情况下，平仓时可接受的最低盈利率（可以为负数）
// revertClose: 已经负向开仓的情况下，平仓时可接受的最低盈利率（可以为负数）
// setting.GridAmount: revertOpen/revertClose的调整值
func calcCarryOpen(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, doRevert string,
	scoreOpen, scoreClose float64) (sidePerp, sideRelated string, amount float64, carryType string) {
	var bidAmount, askAmount float64
	valueLow := setting.AmountLimit
	usdRate := getUsdRate(key)
	usdAvailable := getUsdAvailable(key)
	coin := model.GetCoin(setting.Market, setting.Symbol)
	balance := getCarryBalance(key, coin)
	now := time.Now()
	if now.Hour()%8 == 0 && now.Minute() == 0 && now.Second() < 30 {
		return
	}
	fundingRateSuccess, fundingRate := api.GetFundingRate(setting.Market, setting.Symbol, &carryLock)
	if !fundingRateSuccess {
		return
	}
	if balance == nil {
		model.SetCarryInfo(`warning `+coin, fmt.Sprintf(`slave: balace not available!!! %s %s`, key, coin))
		model.SetCarryInfos(`coin_absent`, key+`_`+coin, map[string]interface{}{`absent`: coin, `key`: key})
		util.Debug(fmt.Sprintf(`calc amount fail balance absent %s %s`, key, coin))
		return ``, ``, 0, carryType
	} else {
		model.RemoveCarryInfo(`warning ` + coin)
		model.RemoveCarryInfos(`coin_absent`, key+`_`+coin)
	}
	balanceAllValue := getBalanceAll(key)
	if balanceAllValue == 0 {
		util.Debug(`balance all value 0 %s %s`, key, coin)
		return
	}
	if getCarryStop(key) {
		util.Debug(`stop carry for 10 times unknown carry %s %s`, key, coin)
		return
	}
	coinRate := math.Abs(balance.UsdValue) / balanceAllValue
	jump := 7.0
	setOpen := math.Max((1.5-usdRate)*setting.OpenShortMargin*(0.5+jump*coinRate), 0.003) - fundingRate
	setClose := math.Min(setting.CloseShortMargin*(0.5+jump*coinRate), -0.003) - fundingRate
	revertOpen := math.NaN()
	revertClose := math.NaN()
	if balance.Amount < 0 {
		revertClose = (setClose+fundingRate)/float64(setting.Chance) + setting.GridAmount - fundingRate
	} else {
		revertOpen = -1*(setOpen+fundingRate)/float64(setting.Chance) + setting.GridAmount + fundingRate
	}
	//revertOpen = math.Abs(setting.GridPriceDistance) * (usdRate - 0.5)
	//if revertOpen > 0 {
	//	revertOpen = revertOpen / (1 + jump*coinRate)
	//} else {
	//	revertOpen = revertOpen / (1 - math.Min(0.9, jump*coinRate))
	//}
	//revertOpen = math.Max(revertOpen, setting.CloseShortMargin/2) + fundingRate + 0.001
	//revertClose = math.Max(-0.0005/(1-math.Min(0.9, jump*coinRate)), setting.CloseShortMargin/2) - fundingRate + 0.001
	usdLowLine := 0.1 * balanceAllValue
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	localOpenValueLimit := math.Min(openValueLimit, usdLowLine/3)
	table := fmt.Sprintf(`%s_dynamic_`, model.FunctionCarry)
	accountRates := strings.Split(model.AppConfig.AccountRate, `,`)
	for i := 1; i < len(keys); i++ {
		if keys[i] == key {
			if len(accountRates) > i {
				rate, _ := strconv.ParseFloat(accountRates[i], 64)
				setOpen *= rate
				setClose *= rate
			}
			table += fmt.Sprintf(`slave%s`, key[0:5])
			usdLowLine = 0.2 * balanceAllValue
		}
	}
	if setting.Market == model.Binance || setting.MarketRelated == model.Binance {
		valueLow = 11
	}
	if setting.Market == model.OKEX {
		collateral := GetCollateral(key)
		//if setting.Symbol == `MINA-USDT-SWAP` {
		//	setOpen += 0.01
		//	setClose += 0.005
		//	revertOpen -= 0.01
		//	revertClose += 0.005
		//}
		if collateral == nil || (keys[0] != key && collateral.Rate < 5) || (keys[0] == key && (collateral.Available-collateral.Occupied)/collateral.Available < 0.1) {
			util.Notice(`doRevert true %s %f %f`, key, collateral.Available, collateral.Occupied, collateral.Rate)
			doRevert = `true`
		}
	}
	if doRevert == `true` {
		setOpen = 1
		setClose = -1
	}
	if scoreClose < setClose || (balance.Amount > 0 && scoreClose <= -1*revertOpen) {
		bidAmount = tickPerp.Asks[0].Amount
		if setting.Market == model.OKEX {
			_, bidAmount = model.ParseRealAmount(setting.Market, setting.Symbol, bidAmount)
		}
		askAmount = tickRelated.Bids[0].Amount
		sidePerp = model.OrderSideBuy
		sideRelated = model.OrderSideSell
		if scoreClose < setClose {
			carryType = carryTypeClose
		} else {
			carryType = carryTypeRevert
		}
	} else if scoreOpen > setOpen || (balance.Amount < 0 && scoreOpen >= revertClose) {
		bidAmount = tickRelated.Asks[0].Amount
		askAmount = tickPerp.Bids[0].Amount
		if setting.Market == model.OKEX {
			_, askAmount = model.ParseRealAmount(setting.Market, setting.Symbol, askAmount)
		}
		sidePerp = model.OrderSideSell
		sideRelated = model.OrderSideBuy
		if scoreOpen > setOpen {
			carryType = carryTypeOpen
		} else {
			carryType = carryTypeRevert
		}
	}
	markPrice := tickPerp.Asks[0].Price
	amount = math.Min(bidAmount, askAmount) * 0.9
	// 开仓时:数量<持仓+可借
	if scoreClose < setClose || scoreOpen > setOpen {
		if sideRelated == model.OrderSideSell {
			amount = math.Min(balance.AvailableWithBorrow, math.Abs(amount))
		}
	} else { // 反向关仓量要<=持仓
		amount = math.Min(math.Abs(balance.Amount), amount)
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
	amount = model.FormatAmountPair(setting.Market, setting.Symbol, setting.SymbolRelated, amount)
	if model.OKEX == setting.Market && amount > 0 {
		amountInPerp := model.GetAmountInMarket(setting.Market, setting.Symbol, amount)
		maxBuyPerp, maxSellPerp := getTradeMax(key, setting.Symbol)
		maxBuyRelated, maxSellRelated := getTradeMax(key, setting.SymbolRelated)
		maxBuyRelated += balance.Borrow
		maxSellRelated = math.Max(maxSellRelated, balance.AvailableWithBorrow)
		if sidePerp == model.OrderSideBuy && sideRelated == model.OrderSideSell {
			amountInPerp = math.Min(amountInPerp, maxBuyPerp)
			amount = math.Min(amount, maxSellRelated)
		} else if sidePerp == model.OrderSideSell && sideRelated == model.OrderSideBuy {
			amountInPerp = math.Min(amountInPerp, maxSellPerp)
			amount = math.Min(amount, maxBuyRelated)
		}
		_, amountInReal := model.ParseRealAmount(setting.Market, setting.Symbol, amountInPerp)
		amount = math.Min(amount, amountInReal)
		amount = model.FormatAmountPair(setting.Market, setting.Symbol, setting.SymbolRelated, amount)
	} else if model.Ftx == setting.Market && amount > 90000000 {
		amount = 90000000
	} else if model.Gate == setting.Market && amount > 0 { //gate限制合约最大下单数量
		marketPerp := model.GetMarketInfo(setting.Market, setting.Symbol)
		_, amountInReal := model.ParseRealAmount(setting.Market, setting.Symbol, marketPerp.SizeMax)
		amount = math.Min(amount, amountInReal)
		if (scoreClose < setClose || scoreOpen > setOpen) && sideRelated == model.OrderSideSell {
			//开仓且卖现货时，最小单笔可借数量限制。有持仓的，需要卖出所有持仓数额再加上最小可借
			marketRelated := model.GetMarketInfo(setting.Market, setting.SymbolRelated)
			minBorrow := marketRelated.BorrowSizeMin
			if balance.Amount > 0 {
				minBorrow += balance.Amount
			}
			if amount < minBorrow {
				amount = math.Abs(balance.Amount)
			}
		}
	}
	if amount > 0 {
		util.Info(fmt.Sprintf(`+++ usdRate: %f coinRate: %f %s symbol: %s %s 
			usd available:%f amount %f balance.Amount: %f scoreHigh: %f setOpen: %f scoreLow: %f setClose: %f
			revertOpen: %f revertClose: %f do revert: %s`,
			usdRate, coinRate, key, setting.Symbol, sidePerp,
			usdAvailable, amount, balance.Amount, scoreOpen, setOpen, scoreClose, setClose,
			revertOpen, revertClose, doRevert))
	}
	msg := setting.Symbol
	if keys[0] != key {
		msg = key[0:5] + msg
	}
	model.SetCarryInfo(table+setting.Symbol,
		fmt.Sprintf("%s\n%f %f usdAva:%s usdRate:%s 计算%s %s %s %s 市场%s %s 资金费率:%s coinRate:%s 可用:%s ",
			msg, setting.OpenShortMargin, setting.CloseShortMargin,
			strconv.FormatFloat(usdAvailable, 'f', 0, 64),
			strconv.FormatFloat(100*usdRate, 'f', 0, 64)+"%",
			strconv.FormatFloat(setOpen, 'f', 4, 64),
			strconv.FormatFloat(setClose, 'f', 4, 64),
			strconv.FormatFloat(revertOpen, 'f', 4, 64),
			strconv.FormatFloat(revertClose, 'f', 4, 64),
			strconv.FormatFloat(scoreOpen, 'f', 4, 64),
			strconv.FormatFloat(scoreClose, 'f', 4, 64),
			strconv.FormatFloat(fundingRate*1000, 'f', 2, 64)+"‰",
			strconv.FormatFloat(100*balance.UsdValue/balanceAllValue, 'f', 1, 64)+"%",
			strconv.FormatFloat(balance.AvailableWithBorrow, 'f', 2, 64)))
	carryInfo := map[string]interface{}{`01.动态正开仓`: setOpen, `02.动态负开仓`: setClose, `03.动态平仓`: revertOpen,
		`04.动态平仓`: revertClose, `0.5市场开仓`: scoreOpen, `06.市场关仓`: scoreClose, `07.usd rate`: usdRate,
		`08.usd available`: usdAvailable, `09. coin rate`: balance.UsdValue / balanceAllValue,
		`10.可用`: balance.AvailableWithBorrow, `11.资金费率`: fundingRate, `12.` + table: setting.Symbol,
		`13.正开仓`: setting.OpenShortMargin, `14.负开仓`: setting.CloseShortMargin}
	model.SetCarryInfos(table, setting.Symbol, carryInfo)
	return sidePerp, sideRelated, amount, carryType
}

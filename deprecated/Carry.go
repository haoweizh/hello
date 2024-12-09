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
//
//const revertDis = 0.005
//const openValueLimit = 10000.0
//const holdingLimitInU = 500000.0
//const carryTypeOpen = `carry`
//const carryTypeClose = `carryClose`
//const carryTypeRevert = `carryRevert`
//const InsufficientCodeBinance = `-2010`
//
//var InsufficientCodeOKEX = map[string]bool{`51008`: true, `51119`: true, `51120`: true, `51131`: true, `51502`: true, `58350`: true, `59108`: true, `59200`: true}
//var marketInitTime = make(map[string]int64) // market - initTime
//var carryLock sync.Mutex
//var carrying bool
//var doCarry = false
//var symbolHighest, symbolLowest string
//var lowest = math.NaN()
//var highest = math.NaN()
//var lastOrderIndex = make(map[string]map[string]int64)                       // market - symbol - index
//var lastOrders = make(map[string]map[string][]*model.Order, lastOrderLength) // market - symbol - []order
//
//const lastOrderLength = 8
//
//func setSettingStatus(setting *model.Setting, status bool) {
//	time.Sleep(time.Minute * 20)
//	setting.Valid = status
//}
//
//func addLastCarry(order *model.Order, setting *model.Setting) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	if order == nil || setting == nil {
//		return
//	}
//	if lastOrders[setting.Market] == nil {
//		lastOrders[setting.Market] = make(map[string][]*model.Order)
//		lastOrderIndex[setting.Market] = make(map[string]int64)
//	}
//	if lastOrders[setting.Market][setting.Symbol] == nil {
//		lastOrders[setting.Market][setting.Symbol] = make([]*model.Order, lastOrderLength)
//		lastOrderIndex[setting.Market][setting.Symbol] = 0
//	}
//	lastOrders[setting.Market][setting.Symbol][lastOrderIndex[setting.Market][setting.Symbol]%lastOrderLength] = order
//	lastOrderIndex[setting.Market][setting.Symbol]++
//	noDealNum := 0
//	tenMin, _ := time.ParseDuration(`10m`)
//	second, _ := time.ParseDuration(`500ms`)
//	for i, lastOrder := range lastOrders[setting.Market][setting.Symbol] {
//		account := model.AppConfig.GetAccountFromKey(order.Market, order.AccountIndex)
//		now := time.Now()
//		if lastOrder == nil || order.OrderTime.Add(tenMin).Before(now) || order.OrderTime.Add(second).After(now) || account == nil {
//			continue
//		}
//		queryOrder := api.QueryOrderById(lastOrder.AccountIndex, account.Secret, lastOrder.Market, lastOrder.Symbol,
//			lastOrder.Symbol, lastOrder.OrderType, lastOrder.OrderId)
//		if queryOrder == nil {
//			continue
//		}
//		model.AppDB.Model(&queryOrder).Where(`order_id=?`, queryOrder.OrderId).Updates(
//			map[string]interface{}{`deal_amount`: queryOrder.DealAmount, `deal_price`: queryOrder.DealPrice, `status`: queryOrder.Status})
//		util.Notice(fmt.Sprintf(`query last %s %s %s %f index %d`,
//			queryOrder.Symbol, queryOrder.OrderId, queryOrder.Status, queryOrder.DealAmount, lastOrderIndex[setting.Market][setting.Symbol]))
//		if queryOrder.DealAmount == 0 && order.Status != model.CarryStatusFail {
//			noDealNum++
//			if noDealNum > 3 {
//				util.Notice(fmt.Sprintf(`no deal order %s %s %d %d stop at %d`,
//					setting.Market, setting.Symbol, len(lastOrders), noDealNum, lastOrderIndex[setting.Market][setting.Symbol]))
//				setting.Valid = false
//				setting.UpdatedAt = now
//				lastOrders[setting.Market][setting.Symbol] = make([]*model.Order, lastOrderLength)
//				lastOrderIndex[setting.Market][setting.Symbol] = 0
//				go setSettingStatus(setting, true)
//				break
//			}
//		} else {
//			lastOrders[setting.Market][setting.Symbol][i] = nil
//		}
//	}
//	util.Notice(`---- add done %s`, setting.Symbol)
//}
//
//func checkSetCarrying(value bool) (before bool) {
//	carryLock.Lock()
//	defer carryLock.Unlock()
//	if value && carrying {
//		return carrying
//	} else {
//		temp := carrying
//		carrying = value
//		return temp
//	}
//}
//
//var postOrderCarry = func(order *model.Order, setting *model.Setting) {
//	if order == nil {
//		return
//	}
//	if order.HaveId() {
//		_, maxBuy, maxSell := api.GetTradeMaxOKEX(order.AccountIndex, ``, order.Symbol, -1)
//		if order.OrderSide == model.OrderSideBuy {
//			maxBuy -= order.Amount
//			maxSell += order.Amount
//		} else if order.OrderSide == model.OrderSideSell {
//			maxBuy += order.Amount
//			maxSell -= order.Amount
//		}
//		api.SetTradeMax(order.AccountIndex, order.Symbol, maxBuy, maxSell)
//		addLastCarry(order, setting)
//		addCarryResult(order.AccountIndex, order.Market, true)
//	} else {
//		unknownFail := true
//		account := model.AppConfig.GetAccountFromKey(order.Market, order.AccountIndex)
//		if account != nil {
//			switch order.Market {
//			case model.OKEX:
//				if InsufficientCodeOKEX[order.ErrCode] {
//					util.Notice(`reset %s trade max with %s %s`, order.Market, order.ErrCode, order.AccountIndex)
//					resetTradeMax(account.Key, account.Secret, model.OKEX)
//					unknownFail = false
//				}
//			case model.Binance:
//				if strings.Contains(InsufficientCodeBinance, order.ErrCode) {
//					util.Notice(`reset binance trade max with %s %s`, order.ErrCode, order.AccountIndex)
//					clearCarry(account.Key, account.Secret, order.Market)
//					unknownFail = false
//				}
//			}
//		}
//		if unknownFail {
//			addCarryResult(order.AccountIndex, order.Market, false)
//		} else {
//			addCarryResult(order.AccountIndex, order.Market, true)
//		}
//	}
//}
//
//func resetTradeMax(key, secret string, market string) {
//	if market == model.Ftx {
//		return // ftx无需设置
//	}
//	if getTradeMaxResetting(key) {
//		return
//	}
//	defer setTradeMaxResetting(key, false)
//	setTradeMaxResetting(key, true)
//	setTradeMaxResetTime(key, time.Now().Unix())
//	util.Notice(fmt.Sprintf(`reset all trade max %s %s`, key, market))
//	coins := model.GetCarryCoins()
//	if coins == nil || coins[market] == nil {
//		return
//	}
//	for coin := range coins[market] {
//		switch market {
//		case model.OKEX:
//			symbolPerp := coin + model.GetPerpTail(market)
//			symbolRelated := coin + model.GetSpotTail(market)
//			api.GetTradeMaxOKEX(key, secret, symbolPerp, 600)
//			api.GetTradeMaxOKEX(key, secret, symbolRelated, 600)
//			time.Sleep(time.Second / 5)
//		case model.Binance:
//			//_, maxLoan := api.GetMaxLoan(key, secret, market, coin)
//			//balance := getCarryBalance(key, coin)
//			//if balance != nil {
//			//	balance.AvailableWithBorrow = balance.Amount + maxLoan
//			//	setCarryBalance(key, coin, balance)
//			//}
//		}
//	}
//}
//
//// MARGIN_UMFUTURE 杠杆全仓钱包转向U本位合约钱包
//// UMFUTURE_MARGIN U本位合约钱包转向杠杆全仓钱包
//// 7:3和8:2是比例范围，超过范围自动平衡成7.5:2.5
//func checkProcessTransfer(key, secret, market string) {
//	switch market {
//	case model.Binance, model.Gate, model.Kucoin:
//		balance := getBalanceAll(key)
//		balancePos := getPosBal(key)
//		if balance/balancePos > 4 {
//			api.Transfer(key, secret, market, `MAIN_UMFUTURE`, 0.25*balance-0.75*balancePos)
//		} else if balance/balancePos < 2.33 {
//			api.Transfer(key, secret, market, `UMFUTURE_MAIN`, 0.75*balancePos-0.25*balance)
//		}
//	}
//}
//
//func clearCarry(key, secret, market string) {
//	settings := model.GetSettings(model.FunctionCarry, market)
//	settingCoins := model.GetSettingCoins(model.FunctionCarry, market)
//	resultBalance, balances, _, collateral := api.GetBalances(key, secret, market)
//	setCollateral(key, collateral)
//	resultPosition, positions, usdInFuture, _ := api.GetPositions(key, secret, market)
//	setPosBal(key, usdInFuture)
//	if !resultBalance || !resultPosition {
//		util.Notice(`%s %s fatal error: can not get balance %#v position %#v`, key, market, resultBalance, resultPosition)
//		return
//	}
//	balanceAllValue := 0.0
//	localUsdAvailable := 0.0
//	borrowAll := 0.0
//	for _, value := range balances {
//		coin := strings.ToUpper(value.Coin)
//		success, bidAsk := model.AppMarkets.GetBidAsk(coin+model.GetSpotTail(market), market)
//		if !success && settingCoins[coin] {
//			util.Notice(`fail to get setting coin bid ask %s , return`, coin)
//			continue
//		}
//		if market == model.OKEX { // 针对okex不能从balance获取可借数的问题进行特殊处理
//			preBalance := getCarryBalance(key, coin)
//			if preBalance != nil {
//				value.AvailableWithBorrow = preBalance.AvailableWithBorrow
//			}
//		}
//		setCarryBalance(key, coin, value)
//		if coin == `BTC` && market == model.OKEX { // 把okex中btc价值的usd按一半计入
//			localUsdAvailable += value.UsdValue / 2
//			balanceAllValue += value.UsdValue / 2
//		}
//		if (coin == `USD` && market == model.Ftx) ||
//			(coin == `USDT` && (market == model.OKEX || market == model.Binance || market == model.Gate || market == model.Kucoin)) {
//			localUsdAvailable += value.Amount
//			balanceAllValue += value.Amount
//			borrowAll += value.Borrow
//		} else if settingCoins[coin] {
//			if value.UsdValue > 0 {
//				balanceAllValue += value.UsdValue
//			} else {
//				balanceAllValue += value.Amount * bidAsk.Bids[0].Price
//			}
//			borrowAll += value.Borrow * bidAsk.Bids[0].Price
//		}
//	}
//	localUsdAvailable = localUsdAvailable - borrowAll
//	setUsdAvailable(key, localUsdAvailable)
//	setUsdRate(key, localUsdAvailable/balanceAllValue)
//	setBalanceAll(key, balanceAllValue)
//	util.Notice(fmt.Sprintf(`[carry] %s usd:%f/%f=%f len(balances):%d`,
//		key, localUsdAvailable, balanceAllValue, usdRate[key], len(balances)))
//	equalSettings := make(map[string]*model.Setting)
//	for _, setting := range settings {
//		equalSettings[setting.Symbol] = setting
//	}
//	for _, setting := range equalSettings {
//		makeEqual(key, secret, setting, balances, positions)
//	}
//	initEmptyBalance(key, market)
//	checkProcessTransfer(key, secret, market)
//}
//
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
//			accounts := model.AppConfig.GetAccounts(market)
//			for _, account := range accounts {
//				if account == nil {
//					util.Notice(`fail to load account`)
//					continue
//				}
//				clearCarry(account.Key, account.Secret, market)
//				localMaxResetTime := getTradeMaxResetTime(account.Key)
//				if time.Now().Unix()-localMaxResetTime > 600 {
//					go resetTradeMax(account.Key, account.Secret, market)
//				}
//			}
//		}
//		util.Notice(`...... exit clearing carry balance`)
//		checkSetCarrying(false)
//		time.Sleep(time.Second * 60)
//	}
//}
//
//var ProcessCarry = func(setting *model.Setting, tick *model.BidAsk) {
//	if !doCarry && model.AppConfig.HandleLink == `1` {
//		go clearCarryBalance()
//		doCarry = true
//		return
//	}
//	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
//	_, tickRelated := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
//	million := util.GetNowUnixMillion()
//	delayTick := int64(0)
//	if tick != nil {
//		delayTick = million - int64(tick.Ts)
//	}
//	delayPerp := int64(0)
//	if tickPerp != nil {
//		delayPerp = million - int64(tickPerp.Ts)
//	}
//	delayRelated := int64(0)
//	if tickRelated != nil {
//		delayRelated = million - int64(tickRelated.Ts)
//	}
//	exit := false
//	if tickPerp == nil || tickRelated == nil || tickPerp.Asks == nil || tickPerp.Bids == nil ||
//		tickRelated.Asks == nil || tickRelated.Bids == nil || setting == nil || model.AppPause ||
//		(model.AppConfig.Env != `test` && model.AppConfig.HandleLink != `1`) || setting.Valid == false {
//		exit = true
//	}
//	switch setting.Market {
//	case model.Binance:
//		if delayPerp > 100 || delayRelated > 100 {
//			exit = true
//		}
//	case model.OKEX:
//		if delayTick > 25 || delayRelated > 300 || delayPerp > 300 {
//			exit = true
//		}
//	case model.Ftx:
//		if delayTick > 95 || delayRelated > 300 || delayPerp > 300 {
//			exit = true
//		}
//	case model.Gate:
//		if delayTick > 40 || delayRelated > 30000 || delayPerp > 30000 {
//			exit = true
//		}
//	case model.Kucoin:
//		if delayTick > 25 || delayRelated > 1000 || delayPerp > 1000 {
//			exit = true
//		}
//	}
//	if exit {
//		return
//	}
//	scoreOpen := 1 - tickRelated.Asks[0].Price/tickPerp.Bids[0].Price
//	scoreClose := 1 - tickRelated.Bids[0].Price/tickPerp.Asks[0].Price
//	mark := fmt.Sprintf(`%s_%s|%s_%s`, setting.Market, setting.Symbol, setting.MarketRelated, setting.SymbolRelated)
//	model.AppMetric.AddCarry(mark, scoreOpen, scoreClose)
//	if math.IsNaN(highest) || scoreOpen > highest || setting.Symbol == symbolHighest {
//		highest = scoreOpen
//		symbolHighest = setting.Symbol
//		model.AppMetric.AddCarry(`开仓_价差|++_++`, highest, math.NaN())
//	}
//	if math.IsNaN(lowest) || scoreClose < lowest || setting.Symbol == symbolLowest {
//		lowest = scoreClose
//		symbolLowest = setting.Symbol
//		model.AppMetric.AddCarry(`开仓_价差|--_--`, math.NaN(), lowest)
//	}
//	begin := 0
//	step := 1
//	accounts := model.AppConfig.GetAccounts(setting.Market)
//	if isRecentCarry(setting.Market, setting.Symbol) {
//		begin = len(accounts) - 1
//		step = -1
//	}
//	//now := time.Now()
//	//if (now.Hour() < 8 && now.Hour() > 2 && now.Second()%8 != 0) || now.Second()%3 == 0 {
//	//	begin = len(keys) - 1
//	//	step = -1
//	//}
//	for i := begin; i >= 0 && i < len(accounts); i += step {
//		if i == 0 {
//			model.SetCarryInfo(accounts[i].Key, `[current high-low]`, fmt.Sprintf(`highest %s %f lowest %s %f time:%s`,
//				symbolHighest, highest, symbolLowest, lowest, time.Now().String()))
//		}
//		sidePerp, sideRelated, amount, carryType := calcCarryOpen(setting, tickPerp, tickRelated, accounts[i].Key,
//			accounts[i].Secret, accounts[i].CarryClose, accounts[i].CarryRate, scoreOpen, scoreClose)
//		if amount > 0 {
//			setRecentCarryTime(setting.Market, setting.Symbol)
//			go placeCarry(setting, tickPerp, tickRelated, accounts[i].Key, accounts[i].Secret, sidePerp, sideRelated, carryType,
//				scoreOpen, scoreClose, amount)
//			return
//		}
//	}
//}
//
//func placeCarry(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, secret, sidePerp,
//	sideRelated, carryType string, scoreOpen, scoreClose, amount float64) {
//	if !checkSetCarrying(true) {
//		defer checkSetCarrying(false)
//	} else {
//		//util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
//		return
//	}
//	coin := model.GetCoin(setting.Market, setting.Symbol)
//	balance := getCarryBalance(key, coin)
//	if balance == nil {
//		util.Notice(fmt.Sprintf(`no coin balance %s %s`, coin, key))
//		return
//	}
//	perpPrice := tickPerp.Asks[0].Price
//	relatedPrice := tickRelated.Bids[0].Price
//	now := int(util.GetNowUnixMillion())
//	util.Notice(fmt.Sprintf(`do carry %s->%s delay %d %d perp[%f %f %f %f] related[%f %f %f %f] with score open:%f close:%f
//	    amount %f worth %f time in million %d key %s`,
//		setting.Symbol, setting.SymbolRelated, now-tickPerp.TsReceived, now-tickRelated.TsReceived, tickPerp.Bids[0].Price,
//		tickPerp.Bids[0].Amount, tickPerp.Asks[0].Price, tickPerp.Asks[0].Amount, tickRelated.Bids[0].Price,
//		tickRelated.Bids[0].Amount, tickRelated.Asks[0].Price, tickRelated.Asks[0].Amount, scoreOpen, scoreClose,
//		amount, amount*tickPerp.Asks[0].Price, util.GetNowUnixMillion(), key))
//	placeSuccess := true
//	if sidePerp == model.OrderSideBuy && sideRelated == model.OrderSideSell {
//		perpPrice = tickPerp.Asks[0].Price
//		relatedPrice = tickRelated.Bids[0].Price
//	} else if sidePerp == model.OrderSideSell && sideRelated == model.OrderSideBuy {
//		perpPrice = tickPerp.Bids[0].Price
//		relatedPrice = tickRelated.Asks[0].Price
//	}
//	if setting.Market == model.OKEX {
//		placeSuccess = api.PlacePairOKEX(key, model.GetCoin(setting.Market, setting.Symbol), sidePerp, sideRelated,
//			model.OrderTypeLimit, model.FunctionCarry, perpPrice, relatedPrice, amount)
//	} else {
//		go api.PlaceOrder(key, secret, sidePerp, model.OrderTypeLimit, setting.Market, setting.Symbol,
//			``, ``, carryType, perpPrice, perpPrice,
//			amount, true, true, postOrderCarry, setting)
//		api.PlaceOrder(key, secret, sideRelated, model.OrderTypeLimit, setting.Market, setting.SymbolRelated,
//			``, ``, carryType, relatedPrice, relatedPrice,
//			amount, true, true, postOrderCarry, setting)
//	}
//	time.Sleep(time.Second / 4)
//	if placeSuccess {
//		usdAvailable := getUsdAvailable(key)
//		balanceAllValue := getBalanceAll(key)
//		if sidePerp == model.OrderSideSell {
//			perpPrice = tickPerp.Bids[0].Price
//			relatedPrice = tickRelated.Asks[0].Price
//			balance.Amount += amount
//			balance.AvailableWithBorrow += amount
//			balance.UsdValue += amount * perpPrice
//			if carryType == carryTypeOpen {
//				usdAvailable -= amount * perpPrice
//				setUsdAvailable(key, usdAvailable)
//			}
//		} else if sidePerp == model.OrderSideBuy {
//			perpPrice = tickPerp.Asks[0].Price
//			relatedPrice = tickRelated.Bids[0].Price
//			balance.Amount -= amount
//			balance.AvailableWithBorrow -= amount
//			balance.UsdValue -= amount * perpPrice
//			if carryType == carryTypeRevert {
//				usdAvailable += amount * relatedPrice
//				setUsdAvailable(key, usdAvailable)
//			}
//		}
//		setCarryBalance(key, coin, balance)
//		setUsdRate(key, usdAvailable/balanceAllValue)
//	}
//}
//
//func getCarryAmounts(setting *model.Setting, balances []*model.Balance, positions []*model.Position) (
//	relatedBalance *model.Balance, amountPerp, amountRelated float64) {
//	tail := model.GetPerpTail(setting.Market)
//	for _, position := range positions {
//		if position != nil && position.Currency == setting.Symbol {
//			amountPerp = position.Holding
//		}
//	}
//	for _, balance := range balances {
//		if strings.ToUpper(balance.Coin+tail) == setting.Symbol || (strings.ToLower(balance.Coin+tail) == setting.Symbol) {
//			amountRelated = balance.Amount
//			relatedBalance = balance
//		}
//	}
//	return relatedBalance, amountPerp, amountRelated
//}
//
//func makeEqual(key, secret string, setting *model.Setting, balances []*model.Balance, positions []*model.Position) (
//	symbol string, price float64, equal bool) {
//	coin := model.GetCoin(setting.Market, setting.Symbol)
//	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
//	_, tickRelated := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.Market)
//	if tickPerp == nil || tickRelated == nil {
//		return ``, 0, true
//	}
//	balance, amountPerp, amountRelated := getCarryAmounts(setting, balances, positions)
//	if balance == nil {
//		if amountPerp != 0 {
//			balance = &model.Balance{Coin: coin, Market: setting.Market}
//		} else {
//			//util.Notice(`func:makeEqual can not get balance %s %s`, key, coin)
//			return
//		}
//	}
//	usdAvailable := getUsdAvailable(key)
//	amount := amountPerp + amountRelated
//	orderSide := model.OrderSideBuy
//	if amount > 0 {
//		orderSide = model.OrderSideSell
//		if tickPerp.Bids[0].Price < (1-revertDis)*tickRelated.Bids[0].Price && amount < balance.AvailableWithBorrow {
//			symbol = setting.SymbolRelated
//			price = tickRelated.Bids[0].Price
//		} else if tickPerp.Bids[0].Price > (1+revertDis)*tickRelated.Bids[0].Price {
//			symbol = setting.Symbol
//			price = tickPerp.Bids[0].Price
//		} else if math.Abs(amountPerp) < math.Abs(amountRelated) && amount < balance.AvailableWithBorrow {
//			symbol = setting.SymbolRelated
//			price = tickRelated.Bids[0].Price
//		} else {
//			symbol = setting.Symbol
//			price = tickPerp.Bids[0].Price
//		}
//	} else if amount < 0 {
//		orderSide = model.OrderSideBuy
//		if tickPerp.Asks[0].Price < (1-revertDis)*tickRelated.Asks[0].Price {
//			symbol = setting.Symbol
//			price = tickPerp.Asks[0].Price
//		} else if tickPerp.Asks[0].Price > (1+revertDis)*tickRelated.Asks[0].Price && amount < usdAvailable/tickRelated.Asks[0].Price {
//			symbol = setting.SymbolRelated
//			price = tickRelated.Asks[0].Price
//		} else if math.Abs(amountPerp) > math.Abs(amountRelated) {
//			symbol = setting.Symbol
//			price = tickPerp.Asks[0].Price
//		} else {
//			symbol = setting.SymbolRelated
//			price = tickRelated.Asks[0].Price
//		}
//		usdBalance := getCarryBalance(key, `USD`)
//		if symbol == setting.SymbolRelated && (usdBalance != nil && usdBalance.Borrow > 0) {
//			amount = 0
//		}
//	}
//	if setting.Market == model.OKEX {
//		if orderSide == model.OrderSideBuy {
//			success, maxBuy, _ := api.GetTradeMaxOKEX(key, secret, symbol, 600)
//			if success && amount > maxBuy {
//				if symbol == setting.Symbol {
//					symbol = setting.SymbolRelated
//					price = tickRelated.Asks[0].Price
//				} else if symbol == setting.SymbolRelated {
//					symbol = setting.Symbol
//					price = tickPerp.Asks[0].Price
//				}
//			}
//		}
//		if orderSide == model.OrderSideSell {
//			success, _, maxSell := api.GetTradeMaxOKEX(key, secret, symbol, 600)
//			if success && amount > maxSell {
//				if symbol == setting.Symbol {
//					symbol = setting.SymbolRelated
//					price = tickRelated.Bids[0].Price
//				} else if symbol == setting.SymbolRelated {
//					symbol = setting.Symbol
//					price = tickPerp.Bids[0].Price
//				}
//			}
//		}
//	}
//	amount = math.Min(math.Abs(amount), 20000/price)
//	switch setting.Market {
//	case model.Ftx:
//		amount = math.Min(amount, 90000000)
//	case model.Binance:
//		if (symbol == setting.Symbol && price*amount < 5) || (symbol == setting.SymbolRelated && price*amount < 10) {
//			util.Notice(fmt.Sprintf("binance can't order %s low fee: %f ", symbol, price*amount))
//			amount = 0
//		}
//	case model.Gate:
//		if price*amount < 1 {
//			amount = 0
//		}
//	}
//	if amount <= 0 {
//		return
//	}
//	// 折算一下在真实市场中下单的数额用于判断是否大于零，实际传入api.PlaceOrder方法中不适用这个数量
//	checkAmount := model.GetAmountInMarket(setting.Market, symbol, amount, price)
//	if checkAmount > 0 {
//		resultPerp := api.CancelOrders(key, secret, setting.Market, setting.Symbol)
//		resultRelated := api.CancelOrders(key, secret, setting.Market, setting.SymbolRelated)
//		util.Notice(fmt.Sprintf(`%s %s cancel all perp:%#v related:%#v >>>>>> equal %s %f, %s %f = %s %f`,
//			key, setting.Market, resultPerp, resultRelated, setting.Symbol, amountPerp, setting.SymbolRelated, amountRelated, orderSide, amount))
//		api.PlaceOrder(key, secret, orderSide, model.OrderTypeLimit, setting.Market, symbol, symbol,
//			``, model.FunctionComplement, price, price, amount, true, true, nil, setting)
//		go api.SetBidAsk(key, secret, setting.Market, setting.Symbol)
//	}
//	return
//}
//
//func initEmptyBalance(key, market string) {
//	now := util.GetNow().Unix()
//	if now-marketInitTime[key] < 600 {
//		return
//	} else {
//		marketInitTime[key] = now
//	}
//	coins := model.GetCarryCoins()
//	if coins == nil || coins[market] == nil {
//		return
//	}
//	for coin := range coins[market] {
//		balance := getCarryBalance(key, coin)
//		if balance == nil {
//			balance = &model.Balance{Coin: coin, Market: market}
//		}
//		//if market == model.Binance || market == model.Gate {
//		//	success, maxLoan := api.GetMaxLoan(key, secret, market, coin)
//		//	if success {
//		//		balance.AvailableWithBorrow = maxLoan + math.Max(0, balance.Amount)
//		//	}
//		//	time.Sleep(time.Second / 8)
//		//}
//		//setCarryBalance(key, coin, balance)
//	}
//	util.Notice(fmt.Sprintf(`set available with borrow %s %s`, market, key))
//}
//
//// revertOpen: 已经正向开仓情况下，平仓时可接受的最低盈利率（可以为负数）
//// revertClose: 已经负向开仓的情况下，平仓时可接受的最低盈利率（可以为负数）
//// setting.GridAmount: revertOpen/revertClose的调整值
//func calcCarryOpen(setting *model.Setting, tickPerp, tickRelated *model.BidAsk, key, secret string, carryClose bool, carryRate,
//	scoreOpen, scoreClose float64) (sidePerp, sideRelated string, amount float64, carryType string) {
//	var bidAmount, askAmount float64
//	valueLow := setting.AmountLimit
//	localUsdRate := getUsdRate(key)
//	localUsdAvailable := getUsdAvailable(key)
//	coin := model.GetCoin(setting.Market, setting.Symbol)
//	balance := getCarryBalance(key, coin)
//	now := time.Now()
//	if now.Hour()%8 == 0 && now.Minute() == 0 && now.Second() < 30 {
//		return
//	}
//	fundingRateSuccess, fundingRate := api.GetFundingRate(key, secret, setting.Market, setting.Symbol, &carryLock)
//	if !fundingRateSuccess {
//		return
//	}
//	if balance == nil {
//		model.SetCarryInfo(key, `warning `+coin, fmt.Sprintf(`balace not available!!! %s %s`, key, coin))
//		util.Debug(fmt.Sprintf(`calc amount fail balance absent %s %s`, key, coin))
//		return ``, ``, 0, carryType
//	} else {
//		model.RemoveCarryInfo(key, `warning `+coin)
//	}
//	balanceAllValue := getBalanceAll(key)
//	if balanceAllValue == 0 {
//		util.Debug(`balance all value 0 %s %s`, key, coin)
//		return
//	}
//	if getCarryStop(key) {
//		util.Debug(`stop carry for 10 times unknown carry %s %s`, key, coin)
//		return
//	}
//	coinRate := math.Abs(balance.UsdValue) / balanceAllValue
//	jump := 7.0
//	setOpen := math.Max((1.5-localUsdRate)*setting.OpenShortMargin*(0.5+jump*coinRate), 0.003) - fundingRate
//	setClose := math.Min(setting.CloseShortMargin*(0.5+jump*coinRate), -0.003) - fundingRate
//	revertOpen := math.NaN()
//	revertClose := math.NaN()
//	if balance.Amount < 0 {
//		revertClose = (setClose+fundingRate)/float64(setting.Chance) + setting.GridAmount - fundingRate
//	} else {
//		revertOpen = -1*(setOpen+fundingRate)/float64(setting.Chance) + setting.GridAmount + fundingRate
//	}
//	//revertOpen = math.Abs(setting.GridPriceDistance) * (usdRate - 0.5)
//	//if revertOpen > 0 {
//	//	revertOpen = revertOpen / (1 + jump*coinRate)
//	//} else {
//	//	revertOpen = revertOpen / (1 - math.Min(0.9, jump*coinRate))
//	//}
//	//revertOpen = math.Max(revertOpen, setting.CloseShortMargin/2) + fundingRate + 0.001
//	//revertClose = math.Max(-0.0005/(1-math.Min(0.9, jump*coinRate)), setting.CloseShortMargin/2) - fundingRate + 0.001
//	//usdLowLine := 0.1 * balanceAllValue
//	table := fmt.Sprintf(`%s_dynamic_`, model.FunctionCarry)
//	setOpen *= carryRate
//	setClose *= carryRate
//	table += fmt.Sprintf(`slave%s`, key[0:5])
//	usdLowLine := 0.2 * balanceAllValue
//	localOpenValueLimit := math.Min(openValueLimit, usdLowLine/3)
//	if setting.Market == model.Binance || setting.MarketRelated == model.Binance {
//		valueLow = 11
//	}
//	if setting.Market == model.OKEX {
//		collateral := GetCollateral(key)
//		//if setting.Symbol == `MINA-USDT-SWAP` {
//		//	setOpen += 0.01
//		//	setClose += 0.005
//		//	revertOpen -= 0.01
//		//	revertClose += 0.005
//		//}
//		if collateral == nil || (collateral.Rate > 0 && (collateral.Rate < 10 ||
//			(collateral.Available-collateral.Occupied)/collateral.Available < 0.1)) {
//			if collateral != nil {
//				util.Notice(`doRevert true %s %f %f %f`, key, collateral.Available, collateral.Occupied, collateral.Rate)
//			}
//			carryClose = true
//		}
//	}
//	if carryClose {
//		setOpen = 1
//		setClose = -1
//	}
//	localHolingLimit := holdingLimitInU
//	account := model.AppConfig.GetAccounts(setting.Market)[0]
//	if account.Key != key {
//		localHolingLimit = holdingLimitInU / 10
//	}
//	// 针对第一个key(dk)关闭反向开仓
//	//if len(keys) > 0 && keys[0] == key {
//	setClose = -1
//	//}
//	scoreValid := false
//	carryType = carryTypeOpen
//	if scoreClose < setClose || (balance.Amount > 0 && scoreClose <= -1*revertOpen) {
//		bidAmount = tickPerp.Asks[0].Amount
//		askAmount = tickRelated.Bids[0].Amount
//		sidePerp = model.OrderSideBuy
//		sideRelated = model.OrderSideSell
//		if scoreClose < setClose {
//			carryType = carryTypeClose
//		}
//		scoreValid = true
//	} else if scoreOpen > setOpen || (balance.Amount < 0 && scoreOpen >= revertClose) {
//		bidAmount = tickRelated.Asks[0].Amount
//		askAmount = tickPerp.Bids[0].Amount
//		sidePerp = model.OrderSideSell
//		sideRelated = model.OrderSideBuy
//		scoreValid = true
//	}
//	markPrice := tickPerp.Asks[0].Price
//	amount = math.Min(bidAmount, askAmount) * 0.9
//	// 开仓时:数量<持仓+可借
//	if scoreClose < setClose || scoreOpen > setOpen {
//		if sideRelated == model.OrderSideSell {
//			amount = math.Min(balance.AvailableWithBorrow, math.Abs(amount))
//		}
//	} else { // 反向关仓量要<=持仓
//		amount = math.Min(math.Abs(balance.Amount), amount)
//	}
//	if sideRelated == model.OrderSideBuy {
//		amount = math.Min(amount, localUsdAvailable/markPrice)
//	}
//	amount = math.Min(amount, localOpenValueLimit/markPrice)
//	// usd所剩太少且还要再买 || 反向持仓太多且还要再卖 || 下单太小
//	if (sideRelated == model.OrderSideBuy &&
//		(localUsdAvailable < usdLowLine || (balance.UsdValue > 0 && coinRate > 0.5)) || balance.UsdValue > localHolingLimit) ||
//		(sideRelated == model.OrderSideSell &&
//			((balance.UsdValue < 0 && coinRate > 0.5) || balance.UsdValue < -1*localHolingLimit)) {
//		amount = 0
//	}
//	amount = model.FormatAmountPair(setting.Market, setting.Symbol, setting.SymbolRelated, amount)
//	if model.OKEX == setting.Market && amount > 0 {
//		_, maxBuyPerp, maxSellPerp := api.GetTradeMaxOKEX(key, secret, setting.Symbol, -1)
//		_, maxBuyRelated, maxSellRelated := api.GetTradeMaxOKEX(key, secret, setting.SymbolRelated, -1)
//		if sidePerp == model.OrderSideBuy && sideRelated == model.OrderSideSell {
//			amount = math.Min(amount, math.Min(maxSellRelated, maxBuyPerp))
//		} else if sidePerp == model.OrderSideSell && sideRelated == model.OrderSideBuy {
//			amount = math.Min(amount, math.Min(maxSellPerp, maxBuyRelated))
//		}
//		amount = model.FormatAmountPair(setting.Market, setting.Symbol, setting.SymbolRelated, amount)
//		if amount < 0 {
//			util.Notice(fmt.Sprintf(`%s %s max size %f %f %f %f limit, no order`,
//				setting.Market, setting.Symbol, maxBuyPerp, maxSellPerp, maxBuyRelated, maxSellRelated))
//		}
//	} else if model.Ftx == setting.Market && amount > 90000000 {
//		amount = 90000000
//	} else if model.Gate == setting.Market && amount > 0 { //gate限制合约最大下单数量
//		marketPerp := model.GetMarketInfo(setting.Market, setting.Symbol)
//		_, amountInReal := model.ParseRealAmount(setting.Market, setting.Symbol, marketPerp.SizeMax)
//		amount = math.Min(amount, amountInReal)
//		if !model.AppConfig.GateSpot && (scoreClose < setClose || scoreOpen > setOpen) && sideRelated == model.OrderSideSell {
//			//开仓且卖现货时，最小单笔可借数量限制。有持仓的，需要卖出所有持仓数额再加上最小可借
//			marketRelated := model.GetMarketInfo(setting.Market, setting.SymbolRelated)
//			if balance.Amount > 0 && amount < balance.Amount+marketRelated.BorrowSizeMin {
//				amount = math.Min(amount, balance.Amount)
//			} else if balance.Amount < 0 {
//				amount = math.Max(amount, marketRelated.BorrowSizeMin)
//			}
//		}
//	}
//	if math.Abs(amount)*tickPerp.Bids[0].Price < valueLow {
//		amount = 0
//	}
//	if amount > 0 {
//		util.Debug(fmt.Sprintf(`+++ usdRate: %f coinRate: %f %s symbol: %s %s
//			usd available:%f amount %f balance.Amount: %f scoreHigh: %f setOpen: %f scoreLow: %f setClose: %f
//			revertOpen: %f revertClose: %f do revert: %#v`,
//			localUsdRate, coinRate, key, setting.Symbol, sidePerp,
//			localUsdAvailable, amount, balance.Amount, scoreOpen, setOpen, scoreClose, setClose,
//			revertOpen, revertClose, carryClose))
//	}
//	model.SetCarryInfo(key, table+setting.Symbol,
//		fmt.Sprintf("%s\n %#v %f %f usdAva:%s usdRate:%s 计算%s %s %s %s 市场%s %s 资金费率:%s coinRate:%s 持仓:%s 可用:%s ",
//			setting.Symbol, scoreValid, setting.OpenShortMargin, setting.CloseShortMargin,
//			strconv.FormatFloat(localUsdAvailable, 'f', 0, 64),
//			strconv.FormatFloat(100*localUsdRate, 'f', 0, 64)+"%",
//			strconv.FormatFloat(setOpen, 'f', 4, 64),
//			strconv.FormatFloat(setClose, 'f', 4, 64),
//			strconv.FormatFloat(revertOpen, 'f', 4, 64),
//			strconv.FormatFloat(revertClose, 'f', 4, 64),
//			strconv.FormatFloat(scoreOpen, 'f', 4, 64),
//			strconv.FormatFloat(scoreClose, 'f', 4, 64),
//			strconv.FormatFloat(fundingRate*1000, 'f', 2, 64)+"‰",
//			strconv.FormatFloat(100*coinRate, 'f', 1, 64)+"%",
//			strconv.FormatFloat(balance.UsdValue, 'f', 1, 64),
//			strconv.FormatFloat(balance.AvailableWithBorrow, 'f', 2, 64)))
//	return sidePerp, sideRelated, amount, carryType
//}
//func SetBidAsk(key, secret, market, symbol string) {
//	switch market {
//	case model.Gate:
//		tailPerp := model.GetPerpTail(model.Gate)
//		if symbol[len(symbol)-len(tailPerp):] == tailPerp {
//			setBidAskGate(key, secret, symbol)
//		}
//	}
//}
//func SetTradeMax(key, symbol string, maxBuy, maxSell float64) {
//	defer symbolLock.Unlock()
//	symbolLock.Lock()
//	if tradeMax[key] == nil {
//		tradeMax[key] = make(map[string][]float64)
//	}
//	tradeMax[key][symbol] = []float64{maxBuy, maxSell}
//}
// FormatAmountPair symbol 期货; related 现货
//func FormatAmountPair(market, symbolPerp, symbolRelated string, amount float64) (formattedAmount float64) {
//	marketPerp := GetMarketInfo(market, symbolPerp)
//	marketRelated := GetMarketInfo(market, symbolRelated)
//	if marketPerp == nil || marketPerp.SizeIncrement == 0 || marketPerp.SizeMin == 0 ||
//		marketRelated == nil || marketRelated.SizeIncrement == 0 || marketRelated.SizeMin == 0 {
//		return 0
//	}
//	sizeInc := marketPerp.SizeIncrement
//	sizeMinPerp := marketPerp.SizeMin
//	success, _, coin := GetCoinFromStandard(symbolPerp)
//	if marketPerp.CTValue > 0 && marketPerp.CTCurrency == coin && success {
//		sizeInc = sizeInc * marketPerp.CTValue
//		sizeMinPerp = sizeMinPerp * marketPerp.CTValue
//	}
//	if sizeInc < marketRelated.SizeIncrement {
//		sizeInc = marketRelated.SizeIncrement
//	}
//	formattedAmount = math.Floor(amount/sizeInc) * sizeInc
//	if formattedAmount < sizeMinPerp || formattedAmount < marketRelated.SizeMin {
//		return 0
//	}
//	return formattedAmount
//}

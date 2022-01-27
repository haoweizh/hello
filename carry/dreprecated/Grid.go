package dreprecated

//var marketSymbolGrid = make(map[string]map[string]*grid)
//var syncGrid = make(chan interface{}, 10)
//var gridLock sync.Mutex
//
//type grid struct {
//	griding             bool
//	sellOrder, buyOrder *model.Order
//	lastPrice           float64
//	lastSide            string
//	sameSide            int64
//}
//
//func setGriding(market, symbol string, ing bool) {
//	gridLock.Lock()
//	defer gridLock.Unlock()
//	if marketSymbolGrid[market] == nil {
//		marketSymbolGrid[market] = make(map[string]*grid)
//	}
//	if marketSymbolGrid[market][symbol] == nil {
//		marketSymbolGrid[market][symbol] = &grid{griding: ing}
//	}
//	marketSymbolGrid[market][symbol].griding = ing
//}
//
//func getGrid(market, symbol string) (gridType *grid) {
//	gridLock.Lock()
//	defer gridLock.Unlock()
//	if marketSymbolGrid[market] == nil {
//		marketSymbolGrid[market] = make(map[string]*grid)
//	}
//	if marketSymbolGrid[market][symbol] == nil {
//		marketSymbolGrid[market][symbol] = &grid{}
//	}
//	return marketSymbolGrid[market][symbol]
//}
//
//func placeGridOrders(key, secret string, setting *model.Setting, bidAsk *model.BidAsk) (result bool) {
//	coins := strings.Split(setting.Symbol, `_`)
//	if len(coins) != 2 {
//		util.Notice(`symbol format not supported ` + setting.Symbol)
//		return false
//	}
//	grid := getGrid(setting.Market, setting.Symbol)
//	if grid.lastPrice == 0 {
//		grid.lastPrice = (bidAsk.Bids[0].Price + bidAsk.Asks[0].Price) / 2
//	}
//	usdtSymbol := coins[0] + `_usdt`
//	result, tick := model.AppMarkets.GetBidAsk(usdtSymbol, setting.Market)
//	if !result {
//		util.Notice(fmt.Sprintf(`%s 没有usdt价格 %s`, setting.Symbol, usdtSymbol))
//		return false
//	}
//	priceDistance := 1 / math.Pow(10, api.GetPriceDecimal(setting.Market, setting.Symbol))
//	priceSell := math.Max(math.Min(bidAsk.Asks[0].Price*(1+setting.GridPriceDistance),
//		grid.lastPrice*(1+setting.GridPriceDistance+0.0007)), bidAsk.Bids[0].Price+priceDistance)
//	priceBuy := priceSell * (1 - 2*setting.GridPriceDistance)
//	if grid.lastSide == model.OrderSideBuy {
//		priceBuy = math.Min(math.Max(bidAsk.Bids[0].Price*(1-setting.GridPriceDistance),
//			grid.lastPrice*(1-setting.GridPriceDistance-0.0007)), bidAsk.Asks[0].Price-priceDistance)
//		priceSell = priceBuy * (1 + 2*setting.GridPriceDistance)
//	}
//	amountBuy := setting.GridAmount / tick.Bids[0].Price
//	amountSell := setting.GridAmount / tick.Asks[0].Price
//	if usdtSymbol == setting.Symbol {
//		amountBuy = setting.GridAmount / priceBuy
//		amountSell = setting.GridAmount / priceSell
//	}
//	leftFree, rightFree, _, _, err := getBalance(key, secret, setting.Market, setting.Symbol, setting.AccountType)
//	if err != nil || (leftFree < amountSell && rightFree < setting.GridAmount) {
//		util.Notice(fmt.Sprintf(`balance not enough %s %s %f %f %f %f`,
//			setting.Market, setting.Symbol, leftFree, rightFree, amountSell, setting.GridAmount))
//		return false
//	}
//	grid.buyOrder = nil
//	grid.sellOrder = nil
//	go placeGridOrder(key, secret, model.OrderSideSell, setting.Market, setting.Symbol, setting.AccountType, priceSell, amountSell)
//	go placeGridOrder(key, secret, model.OrderSideBuy, setting.Market, setting.Symbol, setting.AccountType, priceBuy, amountBuy)
//	return true
//}
//
//func placeGridOrder(key, secret, orderSide, market, symbol, accountType string, price, amount float64) {
//	if price <= 0 {
//		return
//	}
//	order := &model.Order{OrderSide: orderSide, OrderType: model.OrderTypeLimit, Market: market, Symbol: symbol,
//		AmountType: ``, Price: price, Amount: amount, OrderId: ``, ErrCode: ``,
//		Status: model.CarryStatusFail, DealAmount: 0, DealPrice: price}
//	order = api.PlaceOrder(key, secret, orderSide, model.OrderTypeLimit, market, symbol, ``, ``,
//		accountType, ``, ``, price, 0, amount, true)
//	order.Function = model.FunctionGrid
//	grid := getGrid(order.Market, order.Symbol)
//	if order.OrderSide == model.OrderSideBuy {
//		grid.buyOrder = order
//	}
//	if order.OrderSide == model.OrderSideSell {
//		grid.sellOrder = order
//	}
//	syncGrid <- struct{}{}
//}
//
//func handleOrderDeal(key, secret string, grid *grid, order *model.Order, setting *model.Setting,
//	orderSide string) {
//	if grid.lastSide == orderSide {
//		grid.sameSide++
//		if grid.sameSide > 15 {
//			setting.GridPriceDistance = setting.GridPriceDistance * 2
//			model.AppDB.Save(setting)
//			grid.sameSide = 0
//		}
//	} else {
//		grid.sameSide = 0
//	}
//	grid.lastSide = orderSide
//	grid.lastPrice = order.Price
//	order.DealPrice = order.Price
//	order.DealAmount = order.Amount
//	order.Function = model.FunctionGrid
//	util.Notice(fmt.Sprintf(`set buyId %s sellId %s to nil `, grid.buyOrder.OrderId, grid.sellOrder.OrderId))
//	grid.sellOrder = nil
//	grid.buyOrder = nil
//	if order.OrderId != `` {
//		api.RefreshAccount(key, secret, setting.Market)
//	}
//}
//
//// ProcessGrid
//var _ = func(setting *model.Setting) {
//	grid := getGrid(setting.Market, setting.Symbol)
//	if grid.griding || model.AppConfig.Handle != `1` || model.AppPause {
//		return
//	}
//	setGriding(setting.Market, setting.Symbol, true)
//	defer setGriding(setting.Market, setting.Symbol, false)
//	result, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
//	if !result {
//		return
//	}
//	delay := util.GetNowUnixMillion() - int64(tick.Ts)
//	if delay > 100 {
//		util.Notice(fmt.Sprintf(`[delay too long] %d`, delay))
//		return
//	}
//	if grid.sellOrder == nil || grid.buyOrder == nil {
//		if placeGridOrders(setting, tick) {
//			for true {
//				<-syncGrid
//				if grid.sellOrder != nil && grid.buyOrder != nil {
//					break
//				}
//			}
//		}
//	} else if grid.sellOrder != nil && grid.sellOrder.Price < tick.Asks[0].Price {
//		util.Notice(fmt.Sprintf(` sell id %s at price %f < %f`, grid.sellOrder.OrderId, grid.sellOrder.Price,
//			tick.Asks[0].Price))
//		handleOrderDeal(grid, grid.sellOrder, setting, model.OrderSideSell)
//	} else if grid.buyOrder != nil && grid.buyOrder.Price > tick.Bids[0].Price {
//		util.Notice(fmt.Sprintf(`buy id %s at price %f < %f`, grid.buyOrder.OrderId, grid.buyOrder.Price,
//			tick.Bids[0].Price))
//		handleOrderDeal(grid, grid.buyOrder, setting, model.OrderSideBuy)
//	}
//	time.Sleep(time.Microsecond * 100)
//}
///// 以下是refresh内容

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
////function_parameter:0.2_30_
//// coinpark://4003 调用次数繁忙 //2085 最小下单数量限制 //2027 可用余额不足
//const RefreshTypeSequence = `sequence`
//const RefreshTypeFar = `far`
//const RefreshTypeGrid = `refresh_grid`
//
//var refreshOrders = &RefreshOrders{}
//
//type RefreshBidAsk struct {
//	lock sync.Mutex
//	bid  *model.Order
//	ask  *model.Order
//}
//
//func (refreshBidAsk *RefreshBidAsk) get() (bidOrder, askOrder *model.Order) {
//	defer refreshBidAsk.lock.Unlock()
//	refreshBidAsk.lock.Lock()
//	return refreshBidAsk.bid, refreshBidAsk.ask
//}
//
//func (refreshBidAsk *RefreshBidAsk) set(bidOrder, askOrder *model.Order) {
//	defer refreshBidAsk.lock.Unlock()
//	refreshBidAsk.lock.Lock()
//	if bidOrder != nil {
//		refreshBidAsk.bid = bidOrder
//	}
//	if askOrder != nil {
//		refreshBidAsk.ask = askOrder
//	}
//}
//
//type RefreshOrders struct {
//	lock             sync.Mutex
//	refreshing       bool
//	hanging          bool
//	samePriceCount   map[string]map[string]int             // market - symbol - continue same price count
//	samePriceTime    map[string]map[string]*time.Time      // market - symbol - first time new order price
//	bidOrders        map[string]map[string][]*model.Order  // market - symbol - orders
//	askOrders        map[string]map[string][]*model.Order  // market - symbol - orders
//	amountLimit      map[string]map[string]map[int]float64 // market - symbol - time start point - amount
//	lastChancePrice  map[string]map[string]float64         // market - symbol - chance price
//	lastRefreshPrice map[string]map[string]float64         // market - symbol - refresh price
//	fcoinHang        map[string][]*model.Order             // symbol - refresh hang orders
//	inRefresh        map[string]bool                       // symbol - bool
//	waiting          map[string]bool                       // symbol - wait
//	needReset        map[string]map[string]string          // symbol - accountType - coin
//	amountIndex      int
//}
//
//func (refreshOrders *RefreshOrders) getNeedReset(symbol, accountType string) (coin string) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.needReset == nil || refreshOrders.needReset[symbol] == nil {
//		return ``
//	}
//	return refreshOrders.needReset[symbol][accountType]
//}
//
//func (refreshOrders *RefreshOrders) setNeedReset(symbol, accountType, coin string) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.needReset == nil {
//		refreshOrders.needReset = make(map[string]map[string]string)
//	}
//	if refreshOrders.needReset[symbol] == nil {
//		refreshOrders.needReset[symbol] = make(map[string]string)
//	}
//	refreshOrders.needReset[symbol][accountType] = coin
//}
//
//func (refreshOrders *RefreshOrders) setWaiting(symbol string, in bool) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.waiting == nil {
//		refreshOrders.waiting = make(map[string]bool)
//	}
//	refreshOrders.waiting[symbol] = in
//}
//
//func (refreshOrders *RefreshOrders) getWaiting(symbol string) (out bool) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.waiting == nil {
//		return false
//	}
//	return refreshOrders.waiting[symbol]
//}
//
//func (refreshOrders *RefreshOrders) setInRefresh(symbol string, in bool) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.inRefresh == nil {
//		refreshOrders.inRefresh = make(map[string]bool)
//	}
//	refreshOrders.inRefresh[symbol] = in
//}
//
//func (refreshOrders *RefreshOrders) getInRefresh(symbol string) (in bool) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.inRefresh == nil {
//		return false
//	}
//	return refreshOrders.inRefresh[symbol]
//}
//
//func (refreshOrders *RefreshOrders) getGridHang(symbol, orderSide string, price, priceDistance float64) (
//	order *model.Order) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.fcoinHang == nil || refreshOrders.fcoinHang[symbol] == nil {
//		return nil
//	}
//	for i := 0; i < len(refreshOrders.fcoinHang[symbol]); i++ {
//		if refreshOrders.fcoinHang[symbol][i] == nil || refreshOrders.fcoinHang[symbol][i].OrderId == `` {
//			continue
//		}
//		if refreshOrders.fcoinHang[symbol][i].OrderSide == orderSide &&
//			math.Abs(refreshOrders.fcoinHang[symbol][i].Price-price) < 0.1*priceDistance {
//			return refreshOrders.fcoinHang[symbol][i]
//		}
//	}
//	return nil
//}
//
//func (refreshOrders *RefreshOrders) removeRefreshHang(key, secret, symbol string, order *model.Order, needCancel bool) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.fcoinHang == nil || refreshOrders.fcoinHang[symbol] == nil {
//		return
//	}
//	util.Notice(fmt.Sprintf(`remove refresh hang %s %s %s %s price %f`,
//		symbol, order.OrderSide, order.RefreshType, order.OrderId, order.Price))
//	orders := make([]*model.Order, 0)
//	for i := 0; i < len(refreshOrders.fcoinHang[symbol]); i++ {
//		if refreshOrders.fcoinHang[symbol][i] == nil || refreshOrders.fcoinHang[symbol][i].OrderId == `` {
//			continue
//		}
//		if refreshOrders.fcoinHang[symbol][i].OrderId == order.OrderId {
//			if needCancel {
//				api.MustCancel(key, secret, order.Market, order.Symbol, ``, order.OrderType, order.OrderId, true)
//			}
//		} else {
//			orders = append(orders, refreshOrders.fcoinHang[symbol][i])
//		}
//	}
//	refreshOrders.fcoinHang[symbol] = orders
//}
//
//func (refreshOrders *RefreshOrders) addRefreshHang(symbol string, hang *model.Order) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.fcoinHang == nil {
//		refreshOrders.fcoinHang = make(map[string][]*model.Order)
//	}
//	if refreshOrders.fcoinHang[symbol] == nil {
//		refreshOrders.fcoinHang[symbol] = make([]*model.Order, 0)
//	}
//	refreshOrders.fcoinHang[symbol] = append(refreshOrders.fcoinHang[symbol], hang)
//}
//
//func (refreshOrders *RefreshOrders) getRefreshHang(symbol string) (orders []*model.Order) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.fcoinHang == nil {
//		refreshOrders.fcoinHang = make(map[string][]*model.Order)
//	}
//	return refreshOrders.fcoinHang[symbol]
//}
//
//func (refreshOrders *RefreshOrders) getFarHangNum(symbol string) (farBidNum, farAskNum int) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.fcoinHang == nil {
//		refreshOrders.fcoinHang = make(map[string][]*model.Order)
//	}
//	for _, order := range refreshOrders.fcoinHang[symbol] {
//		if order != nil && order.OrderId != `` {
//			if order.RefreshType == RefreshTypeFar {
//				if order.OrderSide == model.OrderSideBuy {
//					farBidNum++
//				}
//				if order.OrderSide == model.OrderSideSell {
//					farAskNum++
//				}
//			}
//		}
//	}
//	return farBidNum, farAskNum
//}
//
//func (refreshOrders *RefreshOrders) SetLastChancePrice(market, symbol string, price float64) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.lastChancePrice == nil {
//		refreshOrders.lastChancePrice = make(map[string]map[string]float64)
//	}
//	if refreshOrders.lastChancePrice[market] == nil {
//		refreshOrders.lastChancePrice[market] = make(map[string]float64)
//	}
//	refreshOrders.lastChancePrice[market][symbol] = price
//}
//
//func (refreshOrders *RefreshOrders) CheckLastChancePrice(market, symbol string, _, _ float64) (same bool) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.lastChancePrice == nil || refreshOrders.lastChancePrice[market] == nil {
//		return false
//	}
//	if refreshOrders.lastChancePrice[market][symbol] > 0 {
//		return true
//	}
//	return false
//}
//
//func (refreshOrders *RefreshOrders) SetLastRefreshPrice(market, symbol string, price, priceDistance float64) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.lastRefreshPrice == nil {
//		refreshOrders.lastRefreshPrice = make(map[string]map[string]float64)
//		refreshOrders.samePriceCount = make(map[string]map[string]int)
//		refreshOrders.samePriceTime = make(map[string]map[string]*time.Time)
//	}
//	if refreshOrders.lastRefreshPrice[market] == nil {
//		refreshOrders.lastRefreshPrice[market] = make(map[string]float64)
//		refreshOrders.samePriceCount[market] = make(map[string]int)
//		refreshOrders.samePriceTime[market] = make(map[string]*time.Time)
//	}
//	if math.Abs(refreshOrders.lastRefreshPrice[market][symbol]-price) < priceDistance {
//		refreshOrders.samePriceCount[market][symbol]++
//	} else {
//		refreshOrders.lastRefreshPrice[market][symbol] = price
//		refreshOrders.samePriceCount[market][symbol] = 1
//		samePriceTime := util.GetNow()
//		refreshOrders.samePriceTime[market][symbol] = &samePriceTime
//	}
//}
//
//func (refreshOrders *RefreshOrders) CheckLastRefreshPrice(market, symbol string, price, priceDistance float64) (over bool) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.lastRefreshPrice == nil || refreshOrders.lastRefreshPrice[market] == nil {
//		return false
//	}
//	d, _ := time.ParseDuration("-601s")
//	timeLine := util.GetNow().Add(d)
//	if refreshOrders.samePriceTime[market][symbol] != nil && refreshOrders.samePriceTime[market][symbol].Before(timeLine) {
//		util.Notice(fmt.Sprintf(`[10min clear]%s %f %d`,
//			symbol, price, refreshOrders.samePriceTime[market][symbol].Unix()))
//		refreshOrders.lastRefreshPrice[market][symbol] = 0
//		refreshOrders.samePriceTime[market][symbol] = nil
//		refreshOrders.samePriceCount[market][symbol] = 0
//		return false
//	}
//	if math.Abs(refreshOrders.lastRefreshPrice[market][symbol]-price) < priceDistance &&
//		refreshOrders.samePriceCount[market][symbol] >= 2 {
//		util.Info(fmt.Sprintf(`[jump 2] %s %s %f`, market, symbol, price))
//		return true
//	}
//	return false
//}
//
//func (refreshOrders *RefreshOrders) CheckAmountLimit(market, symbol string, amountLimit float64) (
//	underLimit bool, amountIndex int) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	now := util.GetNow()
//	amountIndex = int((now.Hour()*3600 + now.Minute()*60 + now.Second()) / model.AppConfig.RefreshTimeSlot)
//	if refreshOrders.amountLimit == nil {
//		refreshOrders.amountLimit = make(map[string]map[string]map[int]float64)
//	}
//	if refreshOrders.amountLimit[market] == nil {
//		refreshOrders.amountLimit[market] = make(map[string]map[int]float64)
//	}
//	if refreshOrders.amountLimit[market][symbol] == nil {
//		refreshOrders.amountLimit[market][symbol] = make(map[int]float64)
//	}
//	refreshOrders.amountLimit[market][symbol][amountIndex+1] = 0
//	refreshOrders.amountLimit[market][symbol][amountIndex-1] = 0
//	if refreshOrders.amountLimit[market][symbol][amountIndex] < amountLimit {
//		return true, amountIndex
//	}
//	if amountLimit > 0 {
//		util.Notice(fmt.Sprintf(`[limit full]%s %s %d %f`, market, symbol, amountIndex,
//			refreshOrders.amountLimit[market][symbol][amountIndex]))
//	}
//	return false, amountIndex
//}
//
//func (refreshOrders *RefreshOrders) AddRefreshAmount(market, symbol string, amount, _ float64) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if refreshOrders.amountLimit == nil {
//		refreshOrders.amountLimit = make(map[string]map[string]map[int]float64)
//	}
//	if refreshOrders.amountLimit[market] == nil {
//		refreshOrders.amountLimit[market] = make(map[string]map[int]float64)
//	}
//	if refreshOrders.amountLimit[market][symbol] == nil {
//		refreshOrders.amountLimit[market][symbol] = make(map[int]float64)
//	}
//	now := util.GetNow()
//	slotNum := int((now.Hour()*3600 + now.Minute()*60 + now.Second()) / model.AppConfig.RefreshTimeSlot)
//	refreshOrders.amountLimit[market][symbol][slotNum] += amount
//	util.Notice(fmt.Sprintf(`[+limit amount]%s %s %d %f`, market, symbol, slotNum, amount))
//}
//
//func (refreshOrders *RefreshOrders) AddRefreshOrders(market, symbol, orderSide string, order *model.Order) {
//	defer refreshOrders.lock.Unlock()
//	refreshOrders.lock.Lock()
//	if orderSide == model.OrderSideBuy {
//		if refreshOrders.bidOrders == nil {
//			refreshOrders.bidOrders = make(map[string]map[string][]*model.Order)
//		}
//		if refreshOrders.bidOrders[market] == nil {
//			refreshOrders.bidOrders[market] = make(map[string][]*model.Order)
//		}
//		if refreshOrders.bidOrders[market][symbol] == nil {
//			refreshOrders.bidOrders[market][symbol] = make([]*model.Order, 0)
//		}
//		priceEqual := false
//		for key, value := range refreshOrders.bidOrders[order.Market][order.Symbol] {
//			if value.Price == order.Price {
//				priceEqual = true
//				if value.OrderTime.Before(order.OrderTime) {
//					refreshOrders.bidOrders[order.Market][order.Symbol][key] = order
//				}
//			}
//		}
//		if !priceEqual {
//			refreshOrders.bidOrders[order.Market][order.Symbol] =
//				append(refreshOrders.bidOrders[order.Market][order.Symbol], order)
//		}
//	}
//	if orderSide == model.OrderSideSell {
//		if refreshOrders.askOrders == nil {
//			refreshOrders.askOrders = make(map[string]map[string][]*model.Order)
//		}
//		if refreshOrders.askOrders[market] == nil {
//			refreshOrders.askOrders[market] = make(map[string][]*model.Order)
//		}
//		if refreshOrders.askOrders[market][symbol] == nil {
//			refreshOrders.askOrders[market][symbol] = make([]*model.Order, 0)
//		}
//		priceEqual := false
//		for key, value := range refreshOrders.askOrders[order.Market][order.Symbol] {
//			if value.Price == order.Price {
//				priceEqual = true
//				if value.OrderTime.Before(order.OrderTime) {
//					refreshOrders.askOrders[order.Market][order.Symbol][key] = order
//				}
//			}
//		}
//		if !priceEqual {
//			refreshOrders.askOrders[order.Market][order.Symbol] =
//				append(refreshOrders.askOrders[order.Market][order.Symbol], order)
//		}
//	}
//}
//
//func (refreshOrders *RefreshOrders) setRefreshing(in bool) {
//	refreshOrders.refreshing = in
//}
//
//func (refreshOrders *RefreshOrders) setHanging(in bool) {
//	refreshOrders.hanging = in
//}
//
//var ProcessRefresh = func(setting *model.Setting, tick *model.BidAsk) {
//	start := util.GetNowUnixMillion()
//	if tick == nil || tick.Asks == nil || tick.Bids == nil || tick.Asks.Len() < 15 || tick.Bids.Len() < 15 ||
//		int(start)-tick.Ts > 500 {
//		timeDis := 0
//		if tick != nil {
//			timeDis = int(start) - tick.Ts
//		}
//		util.Notice(fmt.Sprintf(`[tick not good time]%s %s %d`, setting.Market, setting.Symbol, timeDis))
//		CancelRefreshHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, RefreshTypeGrid+RefreshTypeFar)
//		return
//	}
//	gridSettings := model.GetSetting(model.FunctionGrid, setting.Market, setting.Symbol)
//	if len(gridSettings) < 0 {
//		return
//	}
//	gridSetting := gridSettings[0]
//	result, otherPrice := true, (tick.Bids[0].Price+tick.Asks[0].Price)/2
//	if setting.BinanceDisMin > -0.9 && setting.BinanceDisMax < 0.9 {
//		result, otherPrice = getOtherPrice(setting.Market, setting.Symbol, model.Huobi)
//	}
//	if !result || otherPrice == 0 {
//		util.Notice(fmt.Sprintf(`[get other price]%s %f`, setting.Symbol, otherPrice))
//		//CancelRefreshHang(market, symbol)
//		return
//	}
//	leftFree, rightFree, _, _, err := getBalance(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol,
//		setting.AccountType)
//	if gridSetting != nil {
//		leftFree = leftFree - gridSetting.GridAmount/tick.Bids[0].Price
//		rightFree = rightFree - gridSetting.GridAmount
//	}
//	if err != nil || (leftFree <= 0 && rightFree <= 0) {
//		util.Notice(fmt.Sprintf(`balance not good %s %s`, setting.Market, setting.Symbol))
//		CancelRefreshHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, RefreshTypeGrid)
//		return
//	}
//	hangRate := 0.0
//	amountLimit := 0.0
//	farRate := 0.0
//	farPlaces := make([]float64, 0)
//	finalPlace := 0.21
//	parameters := strings.Split(setting.FunctionParameter, `_`)
//	if len(parameters) >= 4 {
//		hangRate, _ = strconv.ParseFloat(parameters[0], 64)
//		amountLimit, _ = strconv.ParseFloat(parameters[1], 64)
//		farRate, _ = strconv.ParseFloat(parameters[2], 64)
//		for i := 3; i < len(parameters)-1; i++ {
//			place, _ := strconv.ParseFloat(parameters[i], 64)
//			farPlaces = append(farPlaces, place)
//		}
//		finalPlace, _ = strconv.ParseFloat(parameters[len(parameters)-1], 64)
//	}
//	priceDistance := 1 / math.Pow(10, api.GetPriceDecimal(setting.Market, setting.Symbol))
//	if util.GetNowUnixMillion()-int64(tick.Ts) > 1000 {
//		util.SocketInfo(fmt.Sprintf(`socekt old tick %d %d`, util.GetNowUnixMillion(), tick.Ts))
//		CancelRefreshHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, RefreshTypeFar+RefreshTypeGrid)
//		return
//	}
//	go validRefreshHang(model.KeyDefault, model.SecretDefault, setting.Symbol,
//		amountLimit, otherPrice, priceDistance, tick)
//	if model.AppConfig.Handle != `1` || model.AppPause {
//		util.Notice(fmt.Sprintf(`[status]%s %v is pause:%v`,
//			model.AppConfig.Handle, refreshOrders.refreshing, model.AppPause))
//		CancelRefreshHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, ``)
//		return
//	}
//	if refreshOrders.refreshing {
//		return
//	}
//	refreshOrders.setRefreshing(true)
//	defer refreshOrders.setRefreshing(false)
//	delay := util.GetNowUnixMillion() - int64(tick.Ts)
//	//checkDelay := model.AppConfig.Delay
//	//if strings.Contains(symbol, `pax`) {
//	//	checkDelay = 20 * checkDelay
//	//}
//	//if float64(delay) > checkDelay {
//	//	util.Notice(fmt.Sprintf(`[delay long, cancel hang], %s %s`, market, symbol))
//	//	CancelRefreshHang(market, symbol)
//	//}
//	if delay > 500 {
//		util.Info(fmt.Sprintf(`%s %s [delay too long] %d`, setting.Market, setting.Symbol, delay))
//		return
//	}
//	resetCoin := refreshOrders.getNeedReset(setting.Symbol, setting.AccountType)
//	if resetCoin != `` {
//		time.Sleep(time.Second)
//		util.Notice(fmt.Sprintf(`[reset balance]%s %s %s %s`,
//			setting.Market, setting.Symbol, resetCoin, setting.AccountType))
//		//api.RefreshCoinAccount(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, resetCoin,
//		//	setting.AccountType)
//		refreshOrders.setNeedReset(setting.Symbol, setting.AccountType, ``)
//		return
//	}
//	haveAmount, index := refreshOrders.CheckAmountLimit(setting.Market, setting.Symbol, setting.AmountLimit)
//	if index == 0 {
//		refreshOrders.amountIndex = 0
//	}
//	if index > refreshOrders.amountIndex {
//		util.Notice(fmt.Sprintf(`index %d -> %d`, index, refreshOrders.amountIndex))
//		util.Notice(`[before 10min canceling]`)
//		time.Sleep(time.Second * 2)
//		refreshOrders.amountIndex = index
//		symbols := model.GetMarketSymbols(setting.Market)
//		for key := range symbols {
//			CancelRefreshHang(setting.Market, setting.Symbol, RefreshTypeGrid)
//			refreshOrders.setInRefresh(key, true)
//		}
//		time.Sleep(time.Second * 2)
//		api.RefreshAccount(model.KeyDefault, model.SecretDefault, setting.Market)
//		util.Notice(`[after 10min canceling]`)
//		return
//	}
//	amount := math.Min(leftFree, rightFree/tick.Asks[0].Price) * model.AppConfig.AmountRate
//	resize := 1.0
//	if model.AppConfig.Env == `simon` {
//		resize = 100
//	}
//	refreshAble, orderSide, orderPrice := preDeal(setting, setting.Market, setting.Symbol,
//		otherPrice, amount*resize, tick)
//	if refreshOrders.CheckLastChancePrice(setting.Market, setting.Symbol, orderPrice, 0.9*priceDistance) {
//		refreshOrders.SetLastChancePrice(setting.Market, setting.Symbol, 0)
//		refreshAble = false
//	}
//	if refreshOrders.getWaiting(setting.Symbol) {
//		time.Sleep(time.Second)
//		refreshOrders.setWaiting(setting.Symbol, false)
//		return
//	}
//	if refreshOrders.getInRefresh(setting.Symbol) {
//		//util.Notice(fmt.Sprintf(`[in refreshing %s]`, symbol))
//		if haveAmount {
//			if refreshAble {
//				doRefresh(model.KeyDefault, model.SecretDefault, setting, setting.Market, setting.Symbol, setting.AccountType,
//					orderSide, orderPrice, 0.9*priceDistance, amount, tick)
//			} else {
//				util.Notice(fmt.Sprintf(`[in refreshing not refreshable %s]`, setting.Symbol))
//			}
//		} else {
//			refreshOrders.setInRefresh(setting.Symbol, false)
//			time.Sleep(time.Second)
//		}
//	} else {
//		//util.Info(fmt.Sprintf(`[in hang %s]`, symbol))
//		if haveAmount {
//			if refreshAble {
//				//if index > refreshOrders.amountIndex {
//				//	//api.RefreshAccount(market)
//				//}
//				util.Notice(fmt.Sprintf(`in hang refreshable %s %s %f %f`,
//					setting.Market, setting.Symbol, leftFree, rightFree))
//				refreshOrders.setInRefresh(setting.Symbol, true)
//				CancelRefreshHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, RefreshTypeGrid)
//				time.Sleep(time.Second)
//				util.Notice(fmt.Sprintf(`set done refreshable %s %s`, setting.Market, setting.Symbol))
//			} else {
//				util.Notice(fmt.Sprintf(`in hang not refreshable %s left %f right %f`,
//					setting.Symbol, leftFree, rightFree))
//				refreshHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, setting.AccountType,
//					hangRate, amountLimit, farRate, finalPlace, leftFree, rightFree, otherPrice, priceDistance,
//					farPlaces, setting, tick)
//			}
//		} else {
//			util.Notice(fmt.Sprintf(`in hang not have amount %s`, setting.Symbol))
//			refreshHang(model.KeyDefault, model.SecretDefault, setting.Market, setting.Symbol, setting.AccountType,
//				hangRate, amountLimit, farRate, finalPlace, leftFree, rightFree, otherPrice, priceDistance, farPlaces,
//				setting, tick)
//		}
//	}
//}
//
//func hangSequence(key, secret, market, symbol, accountType string, leftFree, rightFree, otherPrice, hangRate,
//	amountLimit, priceDistance float64, coins []string, tick *model.BidAsk) {
//	bidAll := tick.Bids[0].Amount
//	askAll := tick.Asks[0].Amount
//	bidStart := 0
//	askStart := 0
//	for i := 1; i <= 11; i++ {
//		bidAll += tick.Bids[i].Amount
//		if bidAll > amountLimit {
//			bidStart = i
//			break
//		}
//	}
//	for i := 1; i <= 11; i++ {
//		askAll += tick.Asks[i].Amount
//		if askAll > amountLimit {
//			askStart = i
//			break
//		}
//	}
//	orders := refreshOrders.getRefreshHang(symbol)
//	if bidStart < 11 && otherPrice*1.0005 >= tick.Bids[bidStart].Price {
//		amount := rightFree * hangRate / float64(11-bidStart) / tick.Bids[bidStart].Price
//		for i := bidStart; i < 11 && amount*tick.Bids[bidStart].Price > 10; i++ {
//			alreadyExist := false
//			for _, value := range orders {
//				if math.Abs(value.Price-tick.Bids[i].Price) < 0.1*priceDistance &&
//					value.OrderSide == model.OrderSideBuy {
//					alreadyExist = true
//					break
//				}
//			}
//			if !alreadyExist {
//				util.Notice(fmt.Sprintf(`try hang sequence bid %s amount %f ---pos:%d`, symbol, amount, i))
//				sequenceBid := api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, market, symbol, ``,
//					``, accountType, ``, ``, tick.Bids[i].Price, 0, amount, false)
//				if sequenceBid != nil && sequenceBid.OrderId != `` && sequenceBid.Status != model.CarryStatusFail {
//					sequenceBid.Function = model.FunctionHang
//					sequenceBid.RefreshType = RefreshTypeSequence
//					refreshOrders.addRefreshHang(symbol, sequenceBid)
//					model.AppDB.Save(&sequenceBid)
//				} else if sequenceBid != nil && sequenceBid.ErrCode == `1016` {
//					discountBalance(market, symbol, accountType, coins[1], 0.8)
//				}
//			}
//		}
//	}
//	if askStart < 11 && otherPrice*0.9995 <= tick.Asks[askStart].Price {
//		amount := leftFree * hangRate / float64(11-askStart)
//		for i := askStart; i < 11 && amount*tick.Asks[askStart].Price > 10; i++ {
//			alreadyExist := false
//			for _, value := range orders {
//				if math.Abs(value.Price-tick.Asks[i].Price) < 0.1*priceDistance &&
//					value.OrderSide == model.OrderSideSell {
//					alreadyExist = true
//					break
//				}
//			}
//			if !alreadyExist {
//				util.Notice(fmt.Sprintf(`try hang sequence ask %s amount %f ---pos:%d`, symbol, amount, i))
//				sequenceAsk := api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, market, symbol, ``,
//					``, accountType, ``, ``, tick.Asks[i].Price, 0, amount, false)
//				if sequenceAsk != nil && sequenceAsk.OrderId != `` && sequenceAsk.Status != model.CarryStatusFail {
//					sequenceAsk.Function = model.FunctionHang
//					sequenceAsk.RefreshType = RefreshTypeSequence
//					refreshOrders.addRefreshHang(symbol, sequenceAsk)
//					model.AppDB.Save(&sequenceAsk)
//				} else if sequenceAsk != nil && sequenceAsk.ErrCode == `1016` {
//					discountBalance(market, symbol, accountType, coins[0], 0.8)
//				}
//			}
//		}
//	}
//}
//
//func hangGrid(key, secret, market, symbol, accountType string, setting *model.Setting, tick *model.BidAsk) {
//	priceDistance := 1 / math.Pow(10, api.GetPriceDecimal(market, symbol))
//	for i := 1; i < 10; i++ {
//		if i%2 == 0 {
//			continue
//		}
//		bid := refreshOrders.getGridHang(symbol, model.OrderSideBuy, tick.Bids[i].Price, priceDistance)
//		if bid == nil {
//			bid = api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, market, symbol, ``, ``,
//				accountType, ``, ``, tick.Bids[i].Price, 0, setting.GridAmount, false)
//			if bid != nil && bid.OrderId != `` && bid.Status != model.CarryStatusFail {
//				bid.Function = model.FunctionHang
//				bid.RefreshType = RefreshTypeGrid
//				refreshOrders.addRefreshHang(symbol, bid)
//				model.AppDB.Save(&bid)
//			}
//		}
//		ask := refreshOrders.getGridHang(symbol, model.OrderSideSell, tick.Asks[i].Price, priceDistance)
//		if ask == nil {
//			ask = api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, market, symbol, ``, ``,
//				accountType, ``, ``, tick.Asks[i].Price, 0, setting.GridAmount, false)
//			if ask != nil && ask.OrderId != `` && ask.Status != model.CarryStatusFail {
//				ask.Function = model.FunctionHang
//				ask.RefreshType = RefreshTypeGrid
//				refreshOrders.addRefreshHang(symbol, ask)
//				model.AppDB.Save(&ask)
//			}
//		}
//		time.Sleep(time.Millisecond * 100)
//	}
//}
//
//func hangFar(key, secret, market, symbol, accountType string, farRate, finalPlace,
//	leftFree, rightFree float64, coins []string, farPlaces []float64, tick *model.BidAsk) {
//	bidAmount := rightFree * farRate / float64(len(farPlaces))
//	askAmount := leftFree * farRate / float64(len(farPlaces))
//	farBidNum, farAskNum := refreshOrders.getFarHangNum(symbol)
//	if farBidNum == 0 && farAskNum == 0 && len(farPlaces) > 0 && farRate > 0 {
//		for _, place := range farPlaces {
//			farBidPrice := tick.Bids[0].Price * (1 - place)
//			farBidAmount := bidAmount / farBidPrice
//			util.Notice(fmt.Sprintf(`try hang far bid %s price %f amount %f place %f`,
//				symbol, farBidPrice, farBidAmount, place))
//			farBid := api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, market, symbol, ``,
//				``, accountType, ``, ``, farBidPrice, 0, farBidAmount, false)
//			if farBid != nil && farBid.OrderId != `` && farBid.Status != model.CarryStatusFail {
//				farBid.Function = model.FunctionHang
//				farBid.RefreshType = RefreshTypeFar
//				refreshOrders.addRefreshHang(symbol, farBid)
//				model.AppDB.Save(&farBid)
//				farBidNum++
//			} else if farBid != nil && farBid.ErrCode == `1016` {
//				discountBalance(market, symbol, accountType, coins[1], 0.8)
//				break
//			}
//			util.Notice(fmt.Sprintf(`try hang far ask %s %f`, symbol, place))
//			farAsk := api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, market, symbol, ``,
//				``, accountType, ``, ``, tick.Asks[0].Price*(1+place), 0, askAmount, false)
//			if farAsk != nil && farAsk.OrderId != `` && farAsk.Status != model.CarryStatusFail {
//				farAsk.Function = model.FunctionHang
//				farAsk.RefreshType = RefreshTypeFar
//				refreshOrders.addRefreshHang(symbol, farAsk)
//				model.AppDB.Save(&farAsk)
//				farAskNum++
//			} else if farAsk != nil && farAsk.ErrCode == `1016` {
//				discountBalance(market, symbol, accountType, coins[0], 0.8)
//				break
//			}
//		}
//	}
//	if farBidNum < len(farPlaces) && finalPlace > 0 && bidAmount > 0 && len(farPlaces) > 0 {
//		farBidPrice := tick.Bids[0].Price * (1 - finalPlace)
//		farBidAmount := bidAmount * float64(len(farPlaces)-farBidNum) / float64(len(farPlaces)) / farBidPrice
//		util.Notice(fmt.Sprintf(`place bid final %s %f price %f amount %f`,
//			symbol, finalPlace, farBidPrice, farBidAmount))
//		farBid := api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, market, symbol, ``,
//			``, accountType, ``, ``, farBidPrice, 0, farBidAmount, false)
//		if farBid != nil && farBid.OrderId != `` && farBid.Status != model.CarryStatusFail {
//			farBid.Function = model.FunctionHang
//			farBid.RefreshType = RefreshTypeFar
//			refreshOrders.addRefreshHang(symbol, farBid)
//			model.AppDB.Save(&farBid)
//		} else if farBid != nil && farBid.ErrCode == `1016` {
//			discountBalance(market, symbol, accountType, coins[1], 0.8)
//		}
//	}
//	if farAskNum < len(farPlaces) && finalPlace > 0 && len(farPlaces) > 0 {
//		farAskPrice := tick.Asks[0].Price * (1 + finalPlace)
//		farAskAmount := askAmount * float64(len(farPlaces)-farAskNum) / float64(len(farPlaces)) / farAskPrice
//		util.Notice(fmt.Sprintf(`place ask final %s %f price %f amount %f`,
//			symbol, finalPlace, farAskPrice, askAmount))
//		farAsk := api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, market, symbol, ``,
//			``, accountType, ``, ``, farAskPrice, 0, farAskAmount, false)
//		if farAsk != nil && farAsk.OrderId != `` && farAsk.Status != model.CarryStatusFail {
//			farAsk.Function = model.FunctionHang
//			farAsk.RefreshType = RefreshTypeFar
//			refreshOrders.addRefreshHang(symbol, farAsk)
//			model.AppDB.Save(&farAsk)
//		} else if farAsk != nil && farAsk.ErrCode == `1016` {
//			discountBalance(market, symbol, accountType, coins[0], 0.8)
//		}
//	}
//}
//
//func refreshHang(key, secret, market, symbol, accountType string, hangRate, amountLimit, farRate, finalPlace,
//	leftFree, rightFree, otherPrice, priceDistance float64, farPlaces []float64, setting *model.Setting, tick *model.BidAsk) {
//	util.Info(fmt.Sprintf(`[refreshhang]%s`, symbol))
//	if refreshOrders.hanging {
//		return
//	}
//	defer refreshOrders.setHanging(false)
//	refreshOrders.setHanging(true)
//	coins := strings.Split(symbol, `_`)
//	if len(coins) != 2 {
//		util.Notice(fmt.Sprintf(`[wrong symbol]%s`, symbol))
//		return
//	}
//	if setting.GridAmount > 0 {
//		hangGrid(key, secret, market, symbol, accountType, setting, tick)
//	} else {
//		hangSequence(key, secret, market, symbol, accountType, leftFree, rightFree, otherPrice, hangRate,
//			amountLimit, priceDistance, coins, tick)
//	}
//	hangFar(key, secret, market, symbol, accountType, farRate, finalPlace, leftFree, rightFree, coins, farPlaces, tick)
//}
//
//func validRefreshHang(key, secret, symbol string, amountLimit, otherPrice, priceDistance float64, tick *model.BidAsk) {
//	orders := refreshOrders.getRefreshHang(symbol)
//	//util.Notice(`[valid hang] ` + symbol)
//	for _, order := range orders {
//		if order == nil || order.OrderId == `` {
//			continue
//		}
//		switch order.RefreshType {
//		case RefreshTypeGrid:
//			if (order.OrderSide == model.OrderSideBuy && order.Price < tick.Bids[14].Price) ||
//				(order.OrderSide == model.OrderSideSell && order.Price > tick.Asks[14].Price) {
//				refreshOrders.removeRefreshHang(key, secret, symbol, order, true)
//				refreshOrders.setWaiting(symbol, true)
//			}
//			if (order.OrderSide == model.OrderSideBuy && order.Price > tick.Bids[0].Price) ||
//				(order.OrderSide == model.OrderSideSell && order.Price < tick.Asks[0].Price) {
//				refreshOrders.removeRefreshHang(key, secret, symbol, order, false)
//			}
//		case RefreshTypeSequence:
//			if order.OrderSide == model.OrderSideBuy {
//				bidAll := 0.0
//				for i := 0; i < tick.Bids.Len() && tick.Bids[i].Price-0.1*priceDistance > order.Price; i++ {
//					bidAll += tick.Bids[i].Amount
//				}
//				if order.Price > tick.Bids[1].Price+0.1*priceDistance ||
//					order.Price < tick.Bids[10].Price-0.1*priceDistance ||
//					bidAll < amountLimit || order.Price > 1.0005*otherPrice {
//					refreshOrders.removeRefreshHang(key, secret, symbol, order, true)
//					refreshOrders.setWaiting(symbol, true)
//				}
//			}
//			if order.OrderSide == model.OrderSideSell {
//				askAll := 0.0
//				for i := 0; i < tick.Asks.Len() && tick.Asks[i].Price+0.1*priceDistance < order.Price; i++ {
//					askAll += tick.Asks[i].Amount
//				}
//				if order.Price < tick.Asks[1].Price-0.1*priceDistance ||
//					order.Price > tick.Asks[10].Price+0.1*priceDistance ||
//					askAll < amountLimit || order.Price < 0.9995*otherPrice {
//					refreshOrders.removeRefreshHang(key, secret, symbol, order, true)
//					refreshOrders.setWaiting(symbol, true)
//				}
//			}
//		case RefreshTypeFar:
//			if order.OrderSide == model.OrderSideBuy && order.Price > tick.Bids[0].Price*0.995 {
//				refreshOrders.removeRefreshHang(key, secret, symbol, order, true)
//				refreshOrders.setWaiting(symbol, true)
//			}
//			if order.OrderSide == model.OrderSideSell && order.Price < tick.Asks[0].Price*1.005 {
//				refreshOrders.removeRefreshHang(key, secret, symbol, order, true)
//				refreshOrders.setWaiting(symbol, true)
//			}
//		}
//	}
//}
//
//func CancelRefreshHang(key, secret, market, symbol, keep string) {
//	orders := refreshOrders.getRefreshHang(symbol)
//	for _, order := range orders {
//		util.Notice(market + `[cancel orders] all hang but ` + symbol + ` ` + keep)
//		if order != nil && order.OrderId != `` && !strings.Contains(keep, order.RefreshType) {
//			refreshOrders.removeRefreshHang(key, secret, symbol, order, true)
//			time.Sleep(time.Millisecond * 50)
//		}
//	}
//}
//
//func preDeal(setting *model.Setting, market, symbol string, otherPrice, amount float64, tick *model.BidAsk) (
//	result bool, orderSide string, orderPrice float64) {
//	priceDistance := 1 / math.Pow(10, api.GetPriceDecimal(market, symbol))
//	if tick.Asks[0].Price > 1.0003*tick.Bids[0].Price && symbol != `btc_pax` &&
//		tick.Bids[0].Price+1.1*priceDistance < tick.Asks[0].Price {
//		return false, "", 0
//	}
//	if tick.Bids[0].Price > otherPrice*(1+setting.BinanceDisMin) &&
//		tick.Bids[0].Price < otherPrice*(1+setting.BinanceDisMax) {
//		if tick.Bids[0].Price <= otherPrice*(1+model.AppConfig.BinanceOrderDis) {
//			if tick.Bids[0].Amount < amount*setting.RefreshLimit &&
//				tick.Bids[0].Amount > amount*setting.RefreshLimitLow &&
//				tick.Asks[0].Amount > 2*tick.Bids[0].Amount &&
//				tick.Asks[0].Amount < model.AppConfig.PreDealDis*tick.Bids[0].Amount {
//				return true, model.OrderSideBuy, tick.Bids[0].Price
//			}
//		} else {
//			bidAmount := tick.Bids[0].Amount
//			orderPrice = tick.Bids[0].Price - priceDistance
//			if len(tick.Bids) > 1 && tick.Bids[0].Price-tick.Bids[1].Price < 1.1*priceDistance {
//				bidAmount += tick.Bids[1].Amount
//			}
//			if bidAmount < amount*setting.RefreshLimit && bidAmount > amount*setting.RefreshLimitLow &&
//				tick.Asks[0].Amount > 2*tick.Bids[0].Amount &&
//				tick.Asks[0].Amount < model.AppConfig.PreDealDis*tick.Bids[0].Amount {
//				if orderPrice > tick.Bids[0].Price {
//					util.Notice(fmt.Sprintf(`[price error] order price: %f bid1: %f`,
//						orderPrice, tick.Bids[0].Price))
//					orderPrice = tick.Bids[0].Price
//				}
//				return true, model.OrderSideBuy, orderPrice
//			} else if tick.Bids[0].Amount < amount*setting.RefreshLimit &&
//				tick.Bids[0].Amount > amount*setting.RefreshLimitLow &&
//				tick.Asks[0].Amount > 2*tick.Bids[0].Amount &&
//				tick.Asks[0].Amount < model.AppConfig.PreDealDis*tick.Bids[0].Amount {
//				return true, model.OrderSideBuy, tick.Bids[0].Price
//			}
//		}
//	}
//	if tick.Asks[0].Price > otherPrice*(1-setting.BinanceDisMax) &&
//		tick.Asks[0].Price < otherPrice*(1-setting.BinanceDisMin) {
//		if tick.Asks[0].Price >= otherPrice*(1-model.AppConfig.BinanceOrderDis) {
//			if tick.Asks[0].Amount < amount*setting.RefreshLimit &&
//				tick.Asks[0].Amount > amount*setting.RefreshLimitLow &&
//				tick.Bids[0].Amount > 2*tick.Asks[0].Amount &&
//				tick.Bids[0].Amount < model.AppConfig.PreDealDis*tick.Asks[0].Amount {
//				return true, model.OrderSideSell, tick.Asks[0].Price
//			}
//		} else {
//			askAmount := tick.Asks[0].Amount
//			orderPrice = tick.Asks[0].Price + priceDistance
//			if len(tick.Asks) > 1 && tick.Asks[1].Price-tick.Asks[0].Price < 1.1*priceDistance {
//				askAmount += tick.Asks[1].Amount
//			}
//			if askAmount < amount*setting.RefreshLimit && askAmount > amount*setting.RefreshLimitLow &&
//				tick.Bids[0].Amount > 2*tick.Asks[0].Amount &&
//				tick.Bids[0].Amount < model.AppConfig.PreDealDis*tick.Asks[0].Amount {
//				if orderPrice < tick.Asks[0].Price {
//					util.Notice(fmt.Sprintf(`[price error] order price: %f ask1 %f`,
//						orderPrice, tick.Asks[0].Price))
//					orderPrice = tick.Asks[0].Price
//				}
//				return true, model.OrderSideSell, orderPrice
//			} else if tick.Asks[0].Amount < amount*setting.RefreshLimit &&
//				tick.Asks[0].Amount > amount*setting.RefreshLimitLow &&
//				tick.Bids[0].Amount > 2*tick.Asks[0].Amount &&
//				tick.Bids[0].Amount < model.AppConfig.PreDealDis*tick.Asks[0].Amount {
//				return true, model.OrderSideSell, tick.Asks[0].Price
//			}
//		}
//	}
//	return false, ``, 0
//}
//
//func getOtherPrice(market, symbol, otherMarket string) (result bool, otherPrice float64) {
//	if market == model.Fcoin && symbol == `btc_pax` {
//		btcUsdtResult, tickBtcUsdt := model.AppMarkets.GetBidAsk(`btc_usdt`, model.Fcoin)
//		paxUsdtResult, tickPaxUsdt := model.AppMarkets.GetBidAsk(`pax_usdt`, model.Fcoin)
//		if btcUsdtResult && paxUsdtResult {
//			pricePaxUsdt := (tickPaxUsdt.Asks[0].Price + tickPaxUsdt.Bids[0].Price) / 2
//			priceBtcUsdt := (tickBtcUsdt.Asks[0].Price + tickBtcUsdt.Bids[0].Price) / 2
//			if pricePaxUsdt == 0 {
//				return false, 0
//			} else {
//				return true, priceBtcUsdt / pricePaxUsdt
//			}
//		} else {
//			return false, 0
//		}
//	}
//	otherResult, otherTick := model.AppMarkets.GetBidAsk(symbol, otherMarket)
//	if !otherResult {
//		return false, 0
//	}
//	delay := util.GetNowUnixMillion() - int64(otherTick.Ts)
//	if delay > 9000 {
//		util.Notice(fmt.Sprintf(`[%s %s]delay %d`, otherMarket, symbol, delay))
//		return false, 0
//	}
//	return true, (otherTick.Bids[0].Price + otherTick.Asks[0].Price) / 2
//}
//
//func doRefresh(key, secret string, setting *model.Setting, market, symbol, accountType, orderSide string,
//	price, priceDistance, amount float64, tick *model.BidAsk) {
//	util.Notice(fmt.Sprintf(`[doRefresh]%s %s %s %f %f`, symbol, accountType, orderSide, price, amount))
//	orders := &RefreshBidAsk{}
//	go receiveRefresh(key, secret, orders, market, symbol, accountType, price, priceDistance, amount, setting.AmountLimit)
//	bidAmount := amount
//	askAmount := amount
//	if tick.Asks[0].Price-price > priceDistance {
//		bidAmount = 0.9998 * amount
//	}
//	if price-tick.Bids[0].Price > priceDistance {
//		askAmount = 0.9998 * amount
//	}
//	go placeRefreshOrder(key, secret, orders, model.OrderSideBuy, market, symbol, accountType, price, bidAmount)
//	placeRefreshOrder(key, secret, orders, model.OrderSideSell, market, symbol, accountType, price, askAmount)
//	time.Sleep(time.Second)
//}
//
//func receiveRefresh(key, secret string, orders *RefreshBidAsk, market, symbol, accountType string,
//	price, priceDistance, amount, amountLimit float64) {
//	for true {
//		refreshLastBid, refreshLastAsk := orders.get()
//		if refreshLastBid != nil && refreshLastAsk != nil {
//			if refreshLastBid.Status == model.CarryStatusWorking &&
//				refreshLastAsk.Status == model.CarryStatusWorking {
//				refreshOrders.AddRefreshAmount(market, symbol, 2*amount*price, amountLimit)
//				refreshOrders.SetLastChancePrice(market, symbol, price)
//				refreshOrders.SetLastRefreshPrice(market, symbol, price, priceDistance)
//			} else {
//				if refreshLastBid.Status == model.CarryStatusWorking &&
//					refreshLastAsk.Status == model.CarryStatusFail {
//					api.MustCancel(key, secret, refreshLastBid.Market, refreshLastBid.Symbol, ``,
//						refreshLastBid.OrderType, refreshLastBid.OrderId, true)
//				} else if refreshLastAsk.Status == model.CarryStatusWorking &&
//					refreshLastBid.Status == model.CarryStatusFail {
//					api.MustCancel(key, secret, refreshLastAsk.Market, refreshLastAsk.Symbol, ``,
//						refreshLastAsk.OrderType, refreshLastAsk.OrderId, true)
//				}
//				if refreshLastBid.Status == model.CarryStatusFail || refreshLastAsk.Status == model.CarryStatusFail {
//					refreshOrders.setWaiting(symbol, true)
//				}
//				coins := strings.Split(symbol, `_`)
//				coin := ``
//				if refreshLastAsk.ErrCode == `1016` {
//					coin = coins[0]
//				}
//				if refreshLastBid.ErrCode == `1016` {
//					coin = coins[1]
//				}
//				refreshOrders.setNeedReset(symbol, accountType, coin)
//			}
//			break
//		} else {
//			time.Sleep(time.Millisecond * 100)
//		}
//	}
//}
//
//func placeRefreshOrder(key, secret string, orders *RefreshBidAsk, orderSide, market, symbol, accountType string,
//	price, amount float64) {
//	order := api.PlaceOrder(key, secret, orderSide, model.OrderTypeLimit, market, symbol, ``, accountType,
//		``, ``, ``, price, 0, amount, false)
//	if order.Status == model.CarryStatusFail && order.ErrCode == `1002` {
//		time.Sleep(time.Millisecond * 500)
//		order = api.PlaceOrder(key, secret, orderSide, model.OrderTypeLimit, market, symbol, ``,
//			``, accountType, ``, ``, price, 0, amount, false)
//	}
//	order.Function = model.FunctionRefresh
//	if orderSide == model.OrderSideBuy {
//		orders.set(order, nil)
//	} else {
//		orders.set(nil, order)
//	}
//	model.AppDB.Save(&order)
//	if order != nil && order.ErrCode == `1016` {
//		coins := strings.Split(symbol, `_`)
//		if len(coins) == 2 {
//			if orderSide == model.OrderSideBuy {
//				discountBalance(market, symbol, accountType, coins[1], 0.8)
//			} else if orderSide == model.OrderSideSell {
//				discountBalance(market, symbol, accountType, coins[0], 0.8)
//			}
//		}
//	}
//}

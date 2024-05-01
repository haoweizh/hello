package deprecated

//import (
//	"fmt"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"strconv"
//	"strings"
//	"sync"
//)
//
//var rank = &Rank{}
//
////const RANK_OPPO = `rank_opposite`
//const RankRebalance = `rank_rebalance`
//const RankSequence = `rank_sequence`
//
//type Rank struct {
//	lock sync.Mutex
//	//ranking bool
//	checkTime map[string]int64          // symbol - time
//	orders    map[string][]*model.Order // symbol - orders
//	score     map[string]*model.Score   // symbol - score
//}
//
//func (rank *Rank) getCheckTime(symbol string) (checkTime int64) {
//	rank.lock.Lock()
//	defer rank.lock.Unlock()
//	if rank.checkTime == nil {
//		return 0
//	}
//	return rank.checkTime[symbol]
//}
//
//func (rank *Rank) setCheckTime(symbol string) {
//	rank.lock.Lock()
//	defer rank.lock.Unlock()
//	if rank.checkTime == nil {
//		rank.checkTime = make(map[string]int64)
//	}
//	rank.checkTime[symbol] = util.GetNowUnixMillion()
//}
//
////func (rank *Rank) getScore(orderSide, symbol string) (score *model.Score) {
////	rank.lock.Lock()
////	defer rank.lock.Unlock()
////	if rank.score == nil {
////		return nil
////	}
////	return rank.score[symbol]
////}
//
//func (rank *Rank) getOrders(symbol string) (order []*model.Order) {
//	rank.lock.Lock()
//	defer rank.lock.Unlock()
//	if rank.orders == nil {
//		return nil
//	}
//	return rank.orders[symbol]
//}
//
//func (rank *Rank) setOrders(symbol string, orders []*model.Order) {
//	rank.lock.Lock()
//	defer rank.lock.Unlock()
//	if rank.orders == nil {
//		rank.orders = make(map[string][]*model.Order)
//	}
//	rank.orders[symbol] = orders
//}
//
//var ProcessRank = func(setting *model.Setting, tick *model.BidAsk) {
//	if tick == nil || tick.Asks == nil || tick.Bids == nil || tick.Asks.Len() < 11 ||
//		tick.Bids.Len() < 11 {
//		util.Notice(fmt.Sprintf(`[tick not good]%s %s`, setting.Market, setting.Symbol))
//		return
//	}
//	if model.AppConfig.HandleLink != `1` || model.AppPause {
//		return
//	}
//	delay := util.GetNowUnixMillion() - int64(tick.Ts)
//	if delay > 500 {
//		util.Notice(fmt.Sprintf(`%s %s [delay too long] %d`, setting.Market, setting.Symbol, delay))
//		return
//	}
//	priceDistance := 1 / math.Pow(10, api.GetPriceDecimal(setting.Market, setting.Symbol))
//	checkDistance := priceDistance / 10
//	completeTick(setting.Market, setting.Symbol, tick, priceDistance, checkDistance)
//	if setting == nil {
//		return
//	}
//	didSmth := false
//	scoreBid, scoreAsk := calcHighestScore(setting, tick)
//	score := scoreBid
//	cancelSide := model.OrderSideSell
//	trendSide := calcTrend(tick)
//	if trendSide == model.OrderSideSell {
//		score = scoreAsk
//		cancelSide = model.OrderSideBuy
//	}
//	orders := rank.getOrders(setting.Symbol)
//	newOrders := make([]*model.Order, 0)
//	for _, order := range orders {
//		orderScore := calcOrderScore(order, setting, tick)
//		if order.OrderSide == cancelSide {
//			util.Notice(fmt.Sprintf(`--- cancel less side order %s %s`, setting.Symbol, order.OrderId))
//			api.CancelOrder(setting.Market, setting.Symbol, ``, order.OrderType, order.OrderId)
//			didSmth = true
//		} else {
//			if orderScore.Point > (setting.OpenShortMargin+setting.CloseShortMargin)/4 {
//				newOrders = append(newOrders, order)
//			} else if (order.OrderSide == model.OrderSideBuy && order.Price < tick.Bids[0].Price+checkDistance) ||
//				(order.OrderSide == model.OrderSideSell && order.Price > tick.Asks[0].Price-checkDistance) {
//				api.CancelOrder(setting.Market, setting.Symbol, ``, order.OrderType, order.OrderId)
//				didSmth = true
//			}
//		}
//	}
//	if util.GetNowUnixMillion()-rank.getCheckTime(setting.Symbol) > 300000 {
//		queryOrders := api.QueryOrders(setting.Market, ``)
//		for _, queryOrder := range queryOrders {
//			for _, order := range newOrders {
//				if queryOrder.OrderId == order.OrderId {
//					queryOrder.RefreshType = order.RefreshType
//					break
//				}
//			}
//		}
//		newOrders = queryOrders
//		util.Info(fmt.Sprintf(`get working orders from api %s %d`, setting.Symbol, len(newOrders)))
//		rank.setCheckTime(setting.Symbol)
//	} else if !didSmth && score.Point > (setting.OpenShortMargin+setting.CloseShortMargin)/2 {
//		leftFree, rightFree, _, _, _ := getBalance(model.KeyDefault, model.SecretDefault, setting.Market,
//			setting.Symbol, setting.AccountType)
//		if (score.OrderSide == model.OrderSideBuy && rightFree/score.Price > score.Amount) ||
//			(score.OrderSide == model.OrderSideSell && leftFree > score.Amount) {
//			order := api.PlaceOrder(score.OrderSide, model.OrderTypeLimit, setting.Market,
//				setting.Symbol, ``, ``, setting.AccountType, ``, ``,
//				score.Price, 0, score.Amount, false)
//			if order.OrderId != `` {
//				order.Status = model.CarryStatusWorking
//				order.RefreshType = RankSequence
//				newOrders = append(newOrders, order)
//				model.AppDB.Save(&order)
//			}
//		} else {
//			coins := strings.Split(setting.Symbol, `_`)
//			if score.OrderSide == model.OrderSideBuy {
//				util.Info(fmt.Sprintf(`--- coin influient %s %f<%f point:%f`,
//					coins[1], rightFree, score.Price*score.Amount, score.Point))
//			} else {
//				util.Info(fmt.Sprintf(`--- coin influient %s %f<%f point:%f`,
//					coins[0], leftFree, score.Amount, score.Point))
//			}
//		}
//	}
//	rank.setOrders(setting.Symbol, newOrders)
//}
//
//func completeTick(market, symbol string, tick *model.BidAsk, priceDistance, checkDistance float64) {
//	newBids := make([]model.Tick, 11)
//	newAsks := make([]model.Tick, 11)
//	basePrice := (tick.Bids[0].Price + tick.Asks[0].Price) / 2
//	for i := 0; i < 11; i++ {
//		askPrice, _ := util.FormatNum(basePrice+priceDistance*(float64(i)+0.5), api.GetPriceDecimal(market, symbol))
//		bidPrice, _ := util.FormatNum(basePrice-priceDistance*(float64(i)+0.5), api.GetPriceDecimal(market, symbol))
//		newAsks[i] = model.Tick{Symbol: symbol, Price: askPrice, Amount: 0}
//		newBids[i] = model.Tick{Symbol: symbol, Price: bidPrice, Amount: 0}
//	}
//	posBid := 0
//	posAsk := 0
//	for i := 0; i < 11; i++ {
//		for ; posBid < 11; posBid++ {
//			if math.Abs(tick.Bids[posBid].Price-newBids[i].Price) < checkDistance {
//				newBids[i].Amount = tick.Bids[posBid].Amount
//				break
//			}
//		}
//		for ; posAsk < 11; posAsk++ {
//			if math.Abs(tick.Asks[posAsk].Price-newAsks[i].Price) < checkDistance {
//				newAsks[i].Amount = tick.Asks[posAsk].Amount
//				break
//			}
//		}
//	}
//	tick.Asks = newAsks
//	tick.Bids = newBids
//}
//
//func calcTrend(tick *model.BidAsk) (trend string) {
//	amountBid := tick.Bids[0].Amount * 10
//	amountAsk := tick.Asks[0].Amount * 10
//	for i := 1; i < 11; i++ {
//		amountBid += tick.Bids[i].Amount
//		amountAsk += tick.Asks[i].Amount
//	}
//	if amountBid > amountAsk {
//		return model.OrderSideBuy
//	}
//	return model.OrderSideSell
//}
//
//func calcHighestScore(setting *model.Setting, tick *model.BidAsk) (scoreBid, scoreAsk *model.Score) {
//	coins := strings.Split(setting.Symbol, `_`)
//	perUsdt, _ := api.GetPrice(coins[1] + `_usdt`)
//	rankFt, err := strconv.ParseFloat(setting.FunctionParameter, 64)
//	if err != nil {
//		util.Notice(`rank function parameter err ` + setting.FunctionParameter)
//		return nil, nil
//	}
//	minAmount := api.GetMinAmount(setting.Market, setting.Symbol)
//	for key, value := range tick.Bids {
//		if value.Amount < minAmount {
//			tick.Bids[key].Amount += minAmount
//		} else {
//			tick.Bids[key].Amount *= 2
//		}
//	}
//	for key, value := range tick.Asks {
//		if value.Amount < minAmount {
//			tick.Asks[key].Amount += minAmount
//		} else {
//			tick.Asks[key].Amount *= 2
//		}
//	}
//	rankFt = rankFt / 5760
//	scoreBid = &model.Score{Symbol: setting.Symbol, OrderSide: model.OrderSideBuy, Amount: tick.Bids[0].Amount,
//		Price: tick.Bids[0].Price, Point: rankFt / (tick.Bids[0].Price * tick.Bids[0].Amount * perUsdt), Position: 0}
//	scoreAsk = &model.Score{Symbol: setting.Symbol, OrderSide: model.OrderSideSell, Amount: tick.Asks[0].Amount,
//		Price: tick.Asks[0].Price, Point: rankFt / (tick.Asks[0].Price * tick.Asks[0].Amount * perUsdt), Position: 0}
//	score10Bid := &model.Score{Symbol: setting.Symbol, OrderSide: model.OrderSideBuy,
//		Amount: tick.Bids[1].Amount, Price: tick.Bids[1].Price}
//	score10Ask := &model.Score{Symbol: setting.Symbol, OrderSide: model.OrderSideSell,
//		Amount: tick.Asks[1].Amount, Price: tick.Asks[1].Price}
//	for i := 1; i < 11; i++ {
//		if score10Bid.Amount > tick.Bids[i].Amount {
//			score10Bid.Amount = tick.Bids[i].Amount
//			score10Bid.Price = tick.Bids[i].Price
//			score10Bid.Position = i
//		}
//		if score10Ask.Amount > tick.Asks[i].Amount {
//			score10Ask.Amount = tick.Asks[i].Amount
//			score10Ask.Price = tick.Asks[i].Price
//			score10Ask.Position = i
//		}
//	}
//	score10Bid.Point = rankFt / 10 / (score10Bid.Price * score10Bid.Amount * perUsdt)
//	score10Ask.Point = rankFt / 10 / (score10Ask.Price * score10Ask.Amount * perUsdt)
//	if scoreBid.Point < score10Bid.Point {
//		scoreBid = score10Bid
//	}
//	if scoreAsk.Point < score10Ask.Point {
//		scoreAsk = score10Ask
//	}
//	if scoreBid.Point > setting.OpenShortMargin {
//		model.AppDB.Save(&scoreBid)
//	}
//	if scoreAsk.Point > setting.CloseShortMargin {
//		model.AppDB.Save(&scoreAsk)
//	}
//	return scoreBid, scoreAsk
//}
//
//func calcOrderScore(order *model.Order, setting *model.Setting, tick *model.BidAsk) (score *model.Score) {
//	if order.Price < tick.Bids[10].Price || order.Price > tick.Asks[10].Price ||
//		(order.OrderSide == model.OrderSideBuy && order.Price > tick.Bids[0].Price) ||
//		(order.OrderSide == model.OrderSideSell && order.Price < tick.Asks[0].Price) {
//		return &model.Score{Point: 0}
//	}
//	coins := strings.Split(setting.Symbol, `_`)
//	perUsdt, _ := api.GetPrice(coins[1] + `_usdt`)
//	rankFt, err := strconv.ParseFloat(setting.FunctionParameter, 64)
//	if err != nil {
//		util.Notice(`rank function parameter err ` + setting.FunctionParameter)
//		return &model.Score{Point: 0}
//	}
//	rankFt = rankFt / 5760
//	score = &model.Score{Symbol: setting.Symbol, OrderSide: order.OrderSide, Price: order.Price, Point: 0}
//	if order.OrderSide == model.OrderSideBuy && order.Price == tick.Bids[0].Price {
//		score.Amount = tick.Bids[0].Amount
//		score.Point = rankFt / (tick.Bids[0].Price * tick.Bids[0].Amount * perUsdt)
//	} else if order.OrderSide == model.OrderSideSell && order.Price == tick.Asks[0].Price {
//		score.Amount = tick.Asks[0].Amount
//		score.Point = rankFt / (tick.Asks[0].Price * tick.Asks[0].Amount * perUsdt)
//	} else {
//		for i := 1; i < 11; i++ {
//			if order.OrderSide == model.OrderSideBuy && order.Price == tick.Bids[i].Price {
//				score.Amount = tick.Bids[i].Amount
//				score.Point = rankFt / 10 / (tick.Bids[i].Price * tick.Bids[i].Amount * perUsdt)
//				break
//			}
//			if order.OrderSide == model.OrderSideSell && order.Price == tick.Asks[i].Price {
//				score.Amount = tick.Asks[i].Amount
//				score.Point = rankFt / 10 / (tick.Asks[i].Price * tick.Asks[i].Amount * perUsdt)
//				break
//			}
//		}
//	}
//	return score
//}

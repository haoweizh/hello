package deprecated

//
//import (
//	"fmt"
//	"hello/api"
//	"hello/model"
//	"hello/util"
//	"math"
//	"strings"
//	"sync"
//)
//
//var dCarrying = false
//var dCarryLock sync.Mutex
//var marketPositions = make(map[string]map[string]map[string]*model.Position) // address - market - symbol - position
//
//func getPosition(address, market, symbol string) (position *model.Position) {
//	defer dCarryLock.Unlock()
//	dCarryLock.Lock()
//	if marketPositions[address] == nil || marketPositions[address][market] == nil {
//		return nil
//	}
//	return marketPositions[address][market][symbol]
//}
//
//func setPosition(address, market, symbol string, position *model.Position) {
//	defer dCarryLock.Unlock()
//	dCarryLock.Lock()
//	if marketPositions[address] == nil {
//		marketPositions[address] = make(map[string]map[string]*model.Position)
//	}
//	if marketPositions[address][market] == nil {
//		marketPositions[address][market] = make(map[string]*model.Position)
//	}
//	marketPositions[address][market][symbol] = position
//}
//
//func checkSetDCarrying(value bool) (before bool) {
//	dCarryLock.Lock()
//	defer dCarryLock.Unlock()
//	if value && dCarrying {
//		return dCarrying
//	} else {
//		temp := dCarrying
//		dCarrying = value
//		return temp
//	}
//}
//
//// ProcessDCarry setting.GridAmount 下单数量
//// setting.open_short_margin 开仓利润线
//// setting.close_short_margin 关仓利润线
//var ProcessDCarry = func(setting *model.Setting, tickD *model.BidAsk) {
//	_, tickRelated := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.MarketRelated)
//	million := util.GetNowUnixMillion()
//	if tickD == nil || tickRelated == nil || tickD.Asks == nil || tickRelated.Asks == nil ||
//		model.AppConfig.HandleLink != `1` || model.AppPause || (model.AppConfig.Env != `test` &&
//		(million-int64(tickRelated.Ts) > 2000 || million-int64(tickD.Ts) > 2000 || million-int64(tickD.Ts) > 25)) {
//		return
//	}
//	addresses := strings.Split(model.AppConfig.FutureAddress, `,`)
//	if !checkSetDCarrying(true) {
//		defer checkSetDCarrying(false)
//	} else {
//		util.Notice(fmt.Sprintf(`waiting for other ordering %s`, setting.Symbol))
//		return
//	}
//	for _, address := range addresses {
//		account := model.AppConfig.GetAccounts(setting.Market)[0]
//		position := getPosition(address, setting.Market, setting.Symbol)
//		if position == nil {
//			_, pos := api.GetPosition(setting.Market, setting.Symbol, address)
//			setPosition(address, setting.Market, setting.Symbol, pos)
//			return
//		}
//		tickDPrice := tickD.Asks[0].Price
//		tickRelatedPrice := tickRelated.Asks[0].Price
//		model.SetCarryInfo(account.Key, fmt.Sprintf(`dcarry price %s %s`, setting.Market, setting.Symbol),
//			fmt.Sprintf(`d: %f %s-%s %f rate: %f position: %f`,
//				tickDPrice, setting.MarketRelated, setting.SymbolRelated, tickRelatedPrice,
//				(tickDPrice-tickRelatedPrice)/tickRelatedPrice, position.Holding))
//		amount := 0.0
//		orderSide := model.OrderSideBuy
//		line := setting.OpenShortMargin
//		if position.Holding != 0 {
//			line = setting.CloseShortMargin
//			amount = math.Abs(position.Holding)
//			if position.Holding > 0 {
//				orderSide = model.OrderSideSell
//			}
//		}
//		if (tickDPrice-tickRelatedPrice)/tickRelatedPrice > line {
//			orderSide = model.OrderSideSell
//			if amount == 0 {
//				amount = setting.GridAmount
//			}
//		} else if (tickRelatedPrice-tickDPrice)/tickRelatedPrice > line {
//			orderSide = model.OrderSideBuy
//			if amount == 0 {
//				amount = setting.GridAmount
//			}
//		} else {
//			return
//		}
//		if amount > 0 {
//			price := tickD.Asks[0].Price
//			acceptablePrice := price
//			if orderSide == model.OrderSideBuy {
//				acceptablePrice = 1.001 * price
//			} else if orderSide == model.OrderSideSell {
//				acceptablePrice = 0.999 * price
//			}
//			decimalLength := util.NumDecPlaces(price)
//			acceptablePrice, _ = util.FormatNum(acceptablePrice, float64(decimalLength))
//			orderSideType := ``
//			if line > setting.CloseShortMargin {
//				orderSideType = `open`
//				api.PlaceOrder(account.Key, account.Secret, orderSide, ``, model.DFuture, setting.Symbol, ``,
//					`open`, model.FunctionDCarry+orderSideType, acceptablePrice, price, amount, true, false, nil, setting)
//			} else {
//				orderSideType = `close`
//				api.PlaceOrder(account.Key, account.Secret, orderSide, ``, model.DFuture, setting.Symbol, ``,
//					`close`, model.FunctionDCarry+orderSideType, acceptablePrice, price, amount, true, false, nil, setting)
//			}
//			util.Notice(fmt.Sprintf(`dcarry market %s vs %s symbol %s vs %s %s %s [%f %f] price: %f amount:%f`,
//				setting.Market, setting.MarketRelated, setting.Symbol, setting.SymbolRelated, orderSide, orderSideType,
//				tickDPrice, tickRelatedPrice, price, amount))
//			_, pos := api.GetPosition(setting.Market, setting.Symbol, address)
//			setPosition(address, setting.Market, setting.Symbol, pos)
//			util.Notice(fmt.Sprintf(`set position %s %f`, pos.Currency, pos.Holding))
//		}
//	}
//}

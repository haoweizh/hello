package cross

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"sync"
	"time"
)

const lowestScore = -0.005
const lastOrderLength = 8
const holdingLimitInU = 500000.0
const openValueLimit = 10000.0
const InsufficientCodeBinance = `-2010`

var InsufficientCodeOKEX = map[string]bool{`51008`: true, `51119`: true, `51120`: true, `51131`: true, `51502`: true,
	`58350`: true, `59108`: true, `59200`: true}

// market/symbol/bool经过人工确认可以cross的币种
var validSymbol = map[string]map[string]bool{
					model.Gate: {`AE_USDT`: true, `HC_USDT`: true, `REEF_PERP`: true, `REEF_USDT`: true},
					model.OKEX: {`AE-USDT`: true, `HC-USDT`: true},
					model.Ftx:  {`REEF/USD`: true, `REEF-PERP`: true}}
var carryFail = make(map[string]int64) // key fail num
var carryStop = make(map[string]bool)
var lastOrderIndex = make(map[string]map[string]int64)                           // market - symbol - index
var lastOrders = make(map[string]map[string][]*model.Order, lastOrderLength)     // market - symbol - []order
var carryStatus = make(map[string]map[string]map[string]map[string]*CarryStatus) // coin/market/symbol/key/CarryStatus
var contractMarkets = make(map[string]*contractMarket)                           // key - contractMarket
var spotMarkets = make(map[string]*spotMarket)                                   // key - spotMarket
var lastOrderSymbol map[string]map[string]string                                 // key/market/symbol
var crossLock sync.Mutex
var crossing bool
var doCross = false

type contractMarket struct {
	key, market      string
	collateralsInU   float64                    // 可用抵押币种价值总和（目前只有U）
	contractValueInU float64                    // 当前价格下开仓总额，以U计算
	positions        map[string]*model.Position // symbol/position
}

type spotMarket struct {
	key, market     string
	availableU      float64
	accountValueInU float64
	balances        map[string]*model.Balance // symbol/balance
	collateral      *model.Collateral
}

type CarryStatus struct {
	isSpot                      bool
	market, symbol              string
	setting                     *model.Setting
	account                     *model.Account
	LimitSell, LimitBuy         float64 // 最大可开仓买卖数（有机会）
	AvailableSell, AvailableBuy float64 // 最大可买卖数（不管有无机会，能下的数量）
	TradeLineBuy, TradeLineSell float64 // 买卖盈利线（可为负数）
	Holding                     float64
	RateInAll                   float64 // 现货：该币种占总权益的比例；永续：以开仓价算该币种持仓占保证金百分比
}

func isValidSymbol(market, symbol string) bool {
	if validSymbol[market] == nil {
		return false
	}
	return validSymbol[market][symbol]
}

func isFresh(key, market, symbol string) bool {
	defer crossLock.Unlock()
	crossLock.Lock()
	if lastOrderSymbol == nil || lastOrderSymbol[key] == nil || len(lastOrderSymbol) == 0 {
		return true
	}
	if symbol == lastOrderSymbol[key][market] {
		return true
	}
	return false
}

func setFresh(key, market, symbol string) {
	defer crossLock.Unlock()
	crossLock.Lock()
	if lastOrderSymbol == nil {
		lastOrderSymbol = make(map[string]map[string]string)
	}
	if lastOrderSymbol[key] == nil {
		lastOrderSymbol[key] = make(map[string]string)
	}
	lastOrderSymbol[key][market] = symbol
}

func getCarryStop(key string) (stop bool) {
	defer crossLock.Unlock()
	crossLock.Lock()
	return carryStop[key]
}

func GetHoldings(accounts map[string]*model.Account) (holding [][]interface{}) {
	defer crossLock.Unlock()
	crossLock.Lock()
	holding = make([][]interface{}, 0)
	coinHold := make(map[string]float64)
	coinPrice := make(map[string]float64)
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if spotMarkets[account.Key] != nil && spotMarkets[account.Key].balances != nil {
			for _, balance := range spotMarkets[account.Key].balances {
				if balance != nil && balance.Amount != 0 {
					symbol := balance.Coin + model.GetSpotTail(balance.Market)
					holding = append(holding, []interface{}{balance.Market, balance.Coin, symbol, balance.Amount, balance.UsdValue})
					coinHold[balance.Coin] += balance.Amount
					if coinPrice[balance.Coin] == 0 {
						tickGet, tick := model.AppMarkets.GetBidAsk(symbol, balance.Market)
						if tickGet {
							coinPrice[balance.Coin] = tick.Bids[0].Price
						}
					}
				}
			}
		}
		if contractMarkets[account.Key] != nil && contractMarkets[account.Key].positions != nil {
			for _, position := range contractMarkets[account.Key].positions {
				tickGet, tick := model.AppMarkets.GetBidAsk(position.Currency, position.Market)
				if position != nil && position.Free != 0 {
					coin := model.GetCoin(position.Market, position.Currency)
					if tickGet {
						holding = append(holding, []interface{}{position.Market, coin, position.Currency, position.Free, tick.Bids[0].Price * position.Free})
						coinHold[coin] += position.Free
						coinPrice[coin] = tick.Bids[0].Price
					} else {
						holding = append(holding, []interface{}{position.Market, coin, position.Currency, position.Free, 0.0})
					}
				}
			}
		}
	}
	for i := len(holding) - 1; i >= 0; i-- {
		for j := 0; j < i; j++ {
			if math.Abs(holding[j][4].(float64)) < math.Abs(holding[j+1][4].(float64)) {
				holding[j], holding[j+1] = holding[j+1], holding[j]
			}
		}
	}
	for i := range holding {
		coin := holding[i][1].(string)
		money := math.Floor(coinHold[coin]*coinPrice[coin]/10) * 10
		if money < 0 {
			money = math.Ceil(coinHold[coin]*coinPrice[coin]/10) * 10
		}
		holding[i] = append(holding[i], money)
	}
	return
}

func GetCrossMarketValue(key string) (market string, inAllSpot, collateral, holdingSpot, holdingFuture, unRealizedPnl float64) {
	if spotMarkets[key] != nil {
		market = spotMarkets[key].market
		inAllSpot = spotMarkets[key].accountValueInU
		if market == model.Ftx && spotMarkets[key].balances != nil && spotMarkets[key].balances[`FTT/USD`] != nil {
			inAllSpot -= spotMarkets[key].balances[`FTT/USD`].UsdValue
		}
		holdingSpot = spotMarkets[key].accountValueInU - spotMarkets[key].availableU
	}
	if contractMarkets[key] != nil {
		if market == `` {
			market = contractMarkets[key].market
		}
		collateral = contractMarkets[key].collateralsInU
		for _, position := range contractMarkets[key].positions {
			unRealizedPnl += position.ProfitUnreal
		}
		holdingFuture = contractMarkets[key].contractValueInU
	}
	return
}

func getCarryStatus(coin, market, symbol, key string) *CarryStatus {
	crossLock.Lock()
	defer crossLock.Unlock()
	if carryStatus[coin] == nil || carryStatus[coin][market] == nil || carryStatus[coin][market][symbol] == nil {
		return nil
	}
	return carryStatus[coin][market][symbol][key]
}

func setCarryStatus(coin, market, symbol, key string, status *CarryStatus) {
	crossLock.Lock()
	defer crossLock.Unlock()
	if carryStatus[coin] == nil {
		carryStatus[coin] = make(map[string]map[string]map[string]*CarryStatus)
	}
	if carryStatus[coin][market] == nil {
		carryStatus[coin][market] = make(map[string]map[string]*CarryStatus)
	}
	if carryStatus[coin][market][symbol] == nil {
		carryStatus[coin][market][symbol] = make(map[string]*CarryStatus)
	}
	carryStatus[coin][market][symbol][key] = status
}

func pauseCarry(key string) {
	util.Notice(`%s carrying pause %v`, key, true)
	carryStop[key] = true
	time.Sleep(time.Minute * 30)
	util.Notice(`%s carrying pause %v`, key, false)
	carryStop[key] = false
}

func addCarryResult(key, market string, success bool) {
	defer crossLock.Unlock()
	crossLock.Lock()
	if success {
		if carryFail[key] > 0 {
			carryFail[key] = carryFail[key] - 1
		}
	} else {
		carryFail[key] += 2
	}
	if carryFail[key] > 0 {
		util.Notice(`---------- fail size %s %d`, key, carryFail[key])
	}
	if carryFail[key] > 6 {
		go pauseCarry(key)
		util.Notice(`----------stop carry %s %d`, key, carryFail[key])
		carryFail[key] = 0
		for _, address := range model.TeamMails {
			_ = util.SendMail(model.AppConfig.FromMail, model.AppConfig.FromMailAuth, address,
				`暂停下单`, `market: `+market+` stop `+key)
		}
	}
}

func addLastCarry(order *model.Order, setting *model.Setting) {
	crossLock.Lock()
	defer crossLock.Unlock()
	if order == nil || setting == nil {
		return
	}
	if lastOrders[setting.Market] == nil {
		lastOrders[setting.Market] = make(map[string][]*model.Order)
		lastOrderIndex[setting.Market] = make(map[string]int64)
	}
	if lastOrders[setting.Market][setting.Symbol] == nil {
		lastOrders[setting.Market][setting.Symbol] = make([]*model.Order, lastOrderLength)
		lastOrderIndex[setting.Market][setting.Symbol] = 0
	}
	lastOrders[setting.Market][setting.Symbol][lastOrderIndex[setting.Market][setting.Symbol]%lastOrderLength] = order
	lastOrderIndex[setting.Market][setting.Symbol]++
	noDealNum := 0
	tenMin, _ := time.ParseDuration(`10m`)
	second, _ := time.ParseDuration(`500ms`)
	for i, lastOrder := range lastOrders[setting.Market][setting.Symbol] {
		account := model.AppConfig.GetAccountFromKey(order.Market, order.AmountType)
		now := time.Now()
		if lastOrder == nil || order.OrderTime.Add(tenMin).Before(now) || order.OrderTime.Add(second).After(now) || account == nil {
			continue
		}
		queryOrder := api.QueryOrderById(lastOrder.AmountType, account.Secret, lastOrder.Market, lastOrder.Symbol,
			lastOrder.Instrument, lastOrder.OrderType, lastOrder.OrderId)
		if queryOrder == nil {
			continue
		}
		model.AppDB.Model(&queryOrder).Where(`order_id=?`, queryOrder.OrderId).Updates(
			map[string]interface{}{`deal_amount`: queryOrder.DealAmount, `deal_price`: queryOrder.DealPrice, `status`: queryOrder.Status})
		util.Notice(fmt.Sprintf(`query last %s %s %s %f index %d`,
			queryOrder.Symbol, queryOrder.OrderId, queryOrder.Status, queryOrder.DealAmount, lastOrderIndex[setting.Market][setting.Symbol]))
		if queryOrder.DealAmount == 0 && order.Status != model.CarryStatusFail {
			noDealNum++
			if noDealNum > 3 {
				util.Notice(fmt.Sprintf(`no deal order %s %s %d %d stop at %d`,
					setting.Market, setting.Symbol, len(lastOrders), noDealNum, lastOrderIndex[setting.Market][setting.Symbol]))
				setting.Valid = false
				setting.UpdatedAt = now
				lastOrders[setting.Market][setting.Symbol] = make([]*model.Order, lastOrderLength)
				lastOrderIndex[setting.Market][setting.Symbol] = 0
				go setSettingStatus(setting, true)
				break
			}
		} else {
			lastOrders[setting.Market][setting.Symbol][i] = nil
		}
	}
	util.Notice(`---- add done %s`, setting.Symbol)
}

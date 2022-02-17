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

const lowestScore = -0.009
const lastOrderLength = 8
const holdingLimitInU = 500000.0
const openValueLimit = 10000.0
const InsufficientCodeBinance = `-2010`

var InsufficientCodeOKEX = map[string]bool{`51008`: true, `51119`: true, `51120`: true, `51131`: true, `51502`: true,
	`58350`: true, `59108`: true, `59200`: true}

// market/symbol/bool经过人工确认可以cross的币种
var validCrossCoin = map[string][]string{
										model.Gate: {`AE`, `HC`, `REEF`, `ONE`, `LSK`, `GLMR`, `LEASH`},
										model.OKEX: {`AE`, `HC`, `ORBS`, `ONE`, `LSK`, `GLMR`, `LEASH`},
										model.Ftx:  {`REEF`, `ORBS`, `ONE`}}
var lastOrderIndex = make(map[string]map[string]int64)                        // market - symbol - index
var lastOrders = make(map[string]map[string][]*model.Order, lastOrderLength)  // market - symbol - []order
var statuses = make(map[string]map[string]map[string]map[string]*CarryStatus) // coin/market/symbol/key/CarryStatus
var lastCrosses map[string]map[string]string                                  // key/market/symbol
var crossLock sync.Mutex
var spotMarkets, contractMarkets sync.Map // key - spotMarket/contractMarket
var carryFail sync.Map                    // key fail num
var carryStop sync.Map                    // key bool
var crossing bool
var doCross = false

type contractMarket struct {
	key, market          string
	collateralsAvailable float64                    // 可用保证金U数
	contractValueInU     float64                    // 当前价格下开仓总额，以U计算
	accountValueInU      float64                    // 期货权益InU
	positions            map[string]*model.Position // symbol/position
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
	LimitSell, LimitBuy         float64 // 最大可开仓买卖数（有机会），用于cross
	AvailableSell, AvailableBuy float64 // 最大可买卖数（不管有无机会，能下的数量),用于comp
	TradeLineBuy, TradeLineSell float64 // 买卖盈利线（可为负数）
	Holding                     float64
	RateInAll                   float64 // 现货：该币种占总权益的比例；永续：以开仓价算该币种持仓占保证金百分比
}

func isValidSymbol(market, symbol string) bool {
	if validCrossCoin[market] == nil {
		return false
	}
	_, _, coin, _ := model.GetFromStandard(market, symbol)
	for _, validCoin := range validCrossCoin[market] {
		if validCoin == coin {
			return true
		}
	}
	return false
}

func isLastCross(key, market, symbol string) bool {
	defer crossLock.Unlock()
	crossLock.Lock()
	if lastCrosses == nil || lastCrosses[key] == nil || len(lastCrosses) == 0 {
		return true
	}
	if symbol == lastCrosses[key][market] {
		return true
	}
	return false
}

func setLastCross(key, market, symbol string) {
	defer crossLock.Unlock()
	crossLock.Lock()
	if lastCrosses == nil {
		lastCrosses = make(map[string]map[string]string)
	}
	if lastCrosses[key] == nil {
		lastCrosses[key] = make(map[string]string)
	}
	lastCrosses[key][market] = symbol
}

func getLastCrosses(key string) (crosses map[string]string) {
	defer crossLock.Unlock()
	crossLock.Lock()
	if lastCrosses == nil {
		return nil
	}
	return lastCrosses[key]
}

func setLastCrosses(key string, crosses map[string]string) {
	defer crossLock.Unlock()
	crossLock.Lock()
	if lastCrosses == nil {
		lastCrosses = make(map[string]map[string]string)
	}
	lastCrosses[key] = crosses
}

func GetHoldings(accounts map[string]*model.Account) (holding [][]interface{}) {
	defer crossLock.Unlock()
	crossLock.Lock()
	holding = make([][]interface{}, 0)
	coinHold := make(map[string]float64)
	coinPrice := make(map[string]float64)
	uniAccounts := make(map[string]*model.Account)
	for _, account := range accounts {
		if account != nil {
			uniAccounts[account.Key] = account
		}
	}
	for _, account := range uniAccounts {
		if account == nil {
			continue
		}
		value, ok := spotMarkets.Load(account.Key)
		if ok && value != nil {
			sm := value.(*spotMarket)
			for _, balance := range sm.balances {
				if balance != nil && balance.Amount != 0 {
					symbol := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
					valid := false
					setting := model.GetSetting(model.FunctionCross, balance.Market, symbol)
					if setting != nil {
						valid = setting.Valid
					}
					holding = append(holding, []interface{}{balance.Market, balance.Coin, symbol,
						math.Round(balance.Amount), math.Round(balance.UsdValue), valid})
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
		value, ok = contractMarkets.Load(account.Key)
		if ok && value != nil {
			cm := value.(*contractMarket)
			for _, position := range cm.positions {
				valid := false
				setting := model.GetSetting(model.FunctionCross, position.Market, position.Currency)
				if setting != nil {
					valid = setting.Valid
				}
				tickGet, tick := model.AppMarkets.GetBidAsk(position.Currency, position.Market)
				if position != nil && position.Holding != 0 {
					success, _, coin, _ := model.GetFromStandard(position.Market, position.Currency)
					if tickGet && success {
						holding = append(holding, []interface{}{position.Market, coin, position.Currency,
							math.Round(position.Holding), math.Round(tick.Bids[0].Price * position.Holding), valid})
						coinHold[coin] += position.Holding
						coinPrice[coin] = tick.Bids[0].Price
					} else {
						holding = append(holding, []interface{}{position.Market, coin, position.Currency,
							position.Holding, 0.0, valid})
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

func GetCrossMarketValue(key string) (market string, inAllSpot, contractAccountValue, holdingSpot, holdingFuture, unRealizedPnl float64) {
	value, ok := spotMarkets.Load(key)
	if value != nil && ok {
		sm := value.(*spotMarket)
		market = sm.market
		inAllSpot = sm.accountValueInU
		settings := model.GetSettings(model.FunctionCross, market)
		for _, setting := range settings {
			if sm.balances != nil && sm.balances[setting.Symbol] != nil {
				holdingSpot += sm.balances[setting.Symbol].UsdValue
			}
		}
	}
	value, ok = contractMarkets.Load(key)
	if value != nil && ok {
		cm := value.(*contractMarket)
		market = cm.market
		contractAccountValue = cm.accountValueInU
		for _, position := range cm.positions {
			unRealizedPnl += position.ProfitUnreal
		}
		holdingFuture = cm.contractValueInU
	}
	return
}

func getCarryStatus(coin, market, symbol, key string) *CarryStatus {
	crossLock.Lock()
	defer crossLock.Unlock()
	if statuses[coin] == nil || statuses[coin][market] == nil || statuses[coin][market][symbol] == nil {
		return nil
	}
	return statuses[coin][market][symbol][key]
}

func setCarryStatus(coin, market, symbol, key string, status *CarryStatus) {
	crossLock.Lock()
	defer crossLock.Unlock()
	if statuses[coin] == nil {
		statuses[coin] = make(map[string]map[string]map[string]*CarryStatus)
	}
	if statuses[coin][market] == nil {
		statuses[coin][market] = make(map[string]map[string]*CarryStatus)
	}
	if statuses[coin][market][symbol] == nil {
		statuses[coin][market][symbol] = make(map[string]*CarryStatus)
	}
	statuses[coin][market][symbol][key] = status
}

func pauseCarry(key string) {
	util.Notice(`%s carrying pause %v`, key, true)
	carryStop.Store(key, true)
	time.Sleep(time.Minute * 30)
	util.Notice(`%s carrying pause %v`, key, false)
	carryStop.Store(key, false)
}

func addCarryResult(key, market string, success bool) {
	value, ok := carryFail.Load(key)
	fails := 0
	if ok {
		fails = value.(int)
	}
	if success {
		if fails > 0 {
			carryFail.Store(key, fails-1)
		}
	} else {
		carryFail.Store(key, fails+2)
	}
	if fails > 0 {
		util.Notice(`---------- fail size %s %d`, key, fails)
	}
	if fails > 6 {
		go pauseCarry(key)
		util.Notice(`----------stop carry %s %d`, key, fails)
		carryFail.Store(key, 0)
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
		queryOrder := api.QueryOrderById(lastOrder.AmountType, account.Secret, lastOrder.Market, lastOrder.Symbol, lastOrder.OrderType, lastOrder.OrderId)
		if queryOrder == nil {
			continue
		}
		model.AppDB.Model(&queryOrder).Where(`order_id=?`, queryOrder.OrderId).Updates(
			map[string]interface{}{`deal_amount`: queryOrder.DealAmount, `deal_price`: queryOrder.DealPrice})
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

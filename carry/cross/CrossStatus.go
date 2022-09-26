package cross

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

const lowestScore = -0.02
const standardScoreOpen = 0.02  // 开仓标准利润,不得小于0
const standardScoreClose = 0.01 // 平仓标准利润,不得小于0
const lastOrderLength = 8
const holdingLimitInU = 500000.0
const openValueLimit = 10000.0
const compLimitInU = 30000.0
const compTooBig = 70000.0
const crossLimitInU = 10000.0
const InsufficientCodeBinance = `-2010`

var InsufficientCodeOKEX = map[string]bool{`51008`: true, `51119`: true, `51120`: true, `51131`: true, `51502`: true,
	`58350`: true, `59108`: true, `59200`: true}

// market/symbol/bool经过人工确认可以cross的币种
var validCrossCoin = map[string][]string{model.BinanceSpot: {`TORN`, `ANC`, `UST`},
							model.BinancePerp: {`TORN`, `ANC`, `UST`},
							model.Gate:        {`AE`, `HC`, `REEF`, `ONE`, `LSK`, `GLMR`, `LEASH`, `KDA`, `BLOK`, `ANC`, `UST`},
							model.OKEX:        {`AE`, `HC`, `ORBS`, `ONE`, `LSK`, `GLMR`, `LEASH`, `KLAY`, `KDA`, `BLOK`, `TORN`, `ANC`, `UST`},
							model.Ftx:         {`REEF`, `ORBS`, `ONE`, `LUNA`, `UST`},
							model.BybitPerp:   {`KLAY`, `ANC`, `UST`}}
var lastOrderIndex = make(map[string]map[string]int64) // market - symbol - index

var lockCrossing, lockLastCarry sync.Mutex
var lastOrders = make(map[string]map[string][]*model.Order, lastOrderLength) // market - symbol - []order
var lastCrosses sync.Map                                                     // key*market:symbol
var spotMarkets, contractMarkets sync.Map                                    // key - spotMarket/contractMarket
var carryStatusMap sync.Map                                                  // coin*market*symbol*key / CarryStatus
var carryFail sync.Map                                                       // key fail num
var carryStop sync.Map                                                       // key bool
var notifyTime sync.Map                                                      // 1. market_symbol_market_symbol/time 2. funding_market_symbol/time
var getMarketInfoMail sync.Map                                               // FormatCrossPair执行无法获取marketInfo时发送邮件，key为FormatCrossPair，value是当时时间
var placeTick sync.Map                                                       // market_symbol_orderSide:price_amount
var crossing bool
var doCross = false

//var firstComp = false
var wsCross = true

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
	FoundingRate                float64
	LimitSell, LimitBuy         float64 // 最大可开仓买卖数（有机会），用于cross
	AvailableSell, AvailableBuy float64 // 最大可买卖数（不管有无机会，能下的数量),用于comp
	TradeLineBuy, TradeLineSell float64 // 买卖盈利线（可为负数）
	Holding                     float64
	RateInAll                   float64 // 现货：该币种占总权益的比例；永续：以开仓价算该币种持仓占保证金百分比
	FundingRateUpdateTime       time.Time
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

func GetHoldings(accounts map[string]*model.Account) (holding [][]interface{}) {
	holding = make([][]interface{}, 0)
	coinHold := make(map[string]float64)
	coinValue := make(map[string]float64)
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
					setting := api.GetSetting(model.FunctionCross, balance.Market, symbol)
					if setting != nil {
						valid = setting.Valid
					}
					holding = append(holding, []interface{}{balance.Market, balance.Coin, symbol,
						fmt.Sprintf(`%.2f`, balance.Amount), math.Round(balance.UsdValue), valid})
					coinHold[balance.Coin] += balance.Amount
					coinValue[balance.Coin] += math.Round(balance.UsdValue)
				}
			}
		}
		value, ok = contractMarkets.Load(account.Key)
		if ok && value != nil {
			cm := value.(*contractMarket)
			for _, position := range cm.positions {
				valid := false
				setting := api.GetSetting(model.FunctionCross, position.Market, position.Currency)
				if setting != nil {
					valid = setting.Valid
				}
				if position != nil && position.Holding != 0 {
					success, _, coin, _ := model.GetFromStandard(position.Market, position.Currency)
					if success {
						coinHold[coin] += position.Holding
						_, price := model.AppMarkets.GetPriceForce(position.Currency, position.Market, api.GetMarkets())
						holdingLine := []interface{}{position.Market, coin, position.Currency,
							position.Holding, math.Round(price * position.Holding), valid}
						coinValue[coin] += math.Round(price * position.Holding)
						if price > 0 {
							coinPrice[coin] = price
						}
						holding = append(holding, holdingLine)
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
		if coinPrice[coin] == 0 {
			_, coinPrice[coin] = model.AppMarkets.GetPriceForce(coin+model.UniStandardTail[model.MarketTypeSpot], ``, api.GetMarkets())
		}
		money := math.Floor(coinHold[coin]*coinPrice[coin]/10) * 10
		if money < 0 {
			money = math.Ceil(coinHold[coin]*coinPrice[coin]/10) * 10
		}
		holding[i] = append(holding[i], money)
		holding[i] = append(holding[i], math.Round(coinValue[coin]/10)*10)
	}
	return
}

// GetCrossMarketValue keepInU: 不计入总价值的保留不交易币种
func GetCrossMarketValue(key, secret, market string, force bool) (inAllSpot, contractAccountValue, holdingSpot,
	holdingFuture, unRealizedPnl, keepInU float64) {
	value, ok := spotMarkets.Load(key)
	if (!ok || value == nil) && force {
		switch market {
		case model.BybitPerp:
			value = createSpotMarket(key, secret, model.BybitSpot)
		case model.BinancePerp:
			value = createSpotMarket(key, secret, model.BinanceSpot)
		default:
			value = createSpotMarket(key, secret, market)
		}
	}
	if value != nil {
		sm := value.(*spotMarket)
		if sm != nil {
			inAllSpot = sm.accountValueInU
			settings := api.GetSettings(model.FunctionCross, market)
			if settings != nil {
				settings.Range(func(symbol, value interface{}) bool {
					if sm.balances != nil && sm.balances[symbol.(string)] != nil {
						holdingSpot += sm.balances[symbol.(string)].UsdValue
					}
					return true
				})
			}
			if sm.balances != nil {
				if sm.balances[`FTT_USDT`] != nil {
					keepInU += sm.balances[`FTT_USDT`].UsdValue
				}
				if sm.balances[`BTC_USDT`] != nil {
					keepInU += sm.balances[`BTC_USDT`].UsdValue
				}
			}
		}
	}
	value, ok = contractMarkets.Load(key)
	if (!ok || value == nil) && force {
		switch market {
		case model.BinanceSpot:
			value = createContractMarket(key, secret, model.BinancePerp)
		case model.BybitSpot:
			value = createContractMarket(key, secret, model.BybitPerp)
		}
	}
	if value != nil {
		cm := value.(*contractMarket)
		if cm != nil {
			contractAccountValue = cm.accountValueInU
			for _, position := range cm.positions {
				unRealizedPnl += position.ProfitUnreal
			}
			holdingFuture = cm.contractValueInU
		}
	}
	return
}

func pauseCarry(key string, seconds int) {
	util.Notice(`%s carrying pause %v`, key, true)
	carryStop.Store(key, true)
	time.Sleep(time.Second * time.Duration(seconds))
	util.Notice(`%s carrying pause %v`, key, false)
	carryStop.Store(key, false)
}

func addCarryResult(key, market, msg string, success bool) {
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
		carryFail.Store(key, fails+1)
		if market == model.OKEX && wsCross {
			go func() {
				wsCross = false
				util.Notice(fmt.Sprintf(`fail to order okex by ws, change ok ws cross for 10 mins`))
				time.Sleep(time.Minute * 10)
				wsCross = true
			}()
		}
	}
	if fails > 6 {
		if strings.Trim(msg, " ") != "" {
			go pauseCarry(key, 1800)
		} else {
			util.Notice(`key is nil pause all %s accounts`, market)
			accounts := model.AppConfig.GetAccounts(market)
			for _, account := range accounts {
				go pauseCarry(account.Key, 1800)
			}
		}
		util.Notice(`----------stop carry %s %d`, key, fails)
		carryFail.Store(key, 0)
		go api.SendMails(`暂停下单`, market+`msg: `+msg)
	} else if fails > 0 {
		if strings.Trim(msg, ` `) != "" {
			go pauseCarry(key, 300)
		}
		util.Notice(`---------- fail size %s %d`, key, fails)
	}
}

// 某个交易对过去8次交易不成交次数达到3，暂停下单
func addLastCarry(order *model.Order, setting *model.Setting) {
	lockLastCarry.Lock()
	defer lockLastCarry.Unlock()
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
				api.SendMails(`连续交易失败3次`, fmt.Sprintf(`%s %s交易失败%d次`, setting.Market, setting.Symbol, noDealNum))
				go func() {
					time.Sleep(time.Minute * 20)
					setting.Valid = true
				}()
				break
			}
		} else {
			lastOrders[setting.Market][setting.Symbol][i] = nil
		}
	}
	util.Notice(`---- add done %s`, setting.Symbol)
}

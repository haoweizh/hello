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
const standardScoreOpen = 0.002 // 开仓标准利润,不得小于0
// const standardScoreClose = 0.01 // 平仓标准利润,不得小于0
const lastOrderLength = 8
const holdingLimitInU = 500000.0
const openValueLimit = 2000.0
const compLimitInU = 30000.0
const compTooBig = 70000.0
const InsufficientCodeBinance = `-2010`
const SmallInU = 10
const BitgetPosLimit = 130

// TradeLineExtra 由于comp比例过高或亏损过多，需要增加的额外开仓数额
type TradeLineExtra struct {
	coin                string
	buyExtra, sellExtra float64
	updateTime          time.Time
}

//var extras = sync.Map{} // coin - *TradeLineExtra

var InsufficientCodeOKEX = map[string]bool{`51008`: true, `51119`: true, `51120`: true, `51131`: true, `51502`: true,
	`58350`: true, `59108`: true, `59200`: true}

// market/symbol/bool经过人工确认可以cross的币种
var validCrossCoin = map[string][]string{model.BinanceSpot: {`TORN`, `ANC`, `UST`},
	model.BinancePerp: {`TORN`, `ANC`, `UST`},
	model.Gate:        {`AE`, `HC`, `REEF`, `ONE`, `LSK`, `GLMR`, `LEASH`, `KDA`, `BLOK`, `ANC`, `UST`},
	model.OKEX:        {`AE`, `HC`, `ORBS`, `ONE`, `LSK`, `GLMR`, `LEASH`, `KLAY`, `KDA`, `BLOK`, `TORN`, `ANC`, `UST`},
	model.Ftx:         {`REEF`, `ORBS`, `ONE`, `LUNA`, `UST`, `HT`, `TRX`, `ASD`, `FTT`, `BTT`, `JST`, `SUN`},
	model.Bybit:       {`KLAY`, `ANC`, `UST`}}

var liquidBitgetTime = &sync.Map{}        // key - unix second int64
var lastOrderIndex = &sync.Map{}          // market - symbol - index int
var lastOrders = &sync.Map{}              // market - symbol - []*Order
var lastCrosses sync.Map                  // key*market:symbol
var spotMarkets, contractMarkets sync.Map // key - spotMarket/contractMarket
var carryStatusMap = &sync.Map{}          // coin*market*symbol*key / CarryStatus
var carryFail sync.Map                    // key fail num
var carryStop sync.Map                    // key bool
var notifyTime sync.Map                   // 1. market_symbol_market_symbol/time 2. funding_market_symbol/time
var getMarketInfoMail sync.Map            // FormatCrossPair执行无法获取marketInfo时发送邮件，key为FormatCrossPair，value是当时时间
var placeTick sync.Map                    // market_symbol_orderSide:price_amount
var doCross = false

type contractMarket struct {
	key, market          string
	collateralsAvailable float64                  // 可用保证金U数
	contractValueInU     float64                  // 当前价格下开仓总额，以U计算
	accountValueInU      float64                  // 期货权益InU
	positions            map[string]*api.Position // symbol/position
}

type spotMarket struct {
	key, market     string
	availableU      float64
	accountValueInU float64
	balances        map[string]*model.Balance // symbol/balance
	collateral      *api.Collateral
}

type CarryStatus struct {
	isSpot                        bool
	market, symbol                string
	reduceOnlyBuy, reduceOnlySell bool
	setting                       *model.Setting
	account                       *model.Account
	FoundingRate                  float64
	LimitSell, LimitBuy           float64 // 最大可开仓买卖数（有机会），用于cross
	AvailableSell, AvailableBuy   float64 // 最大可买卖数（不管有无机会，能下的数量),用于comp
	TradeLineBuy, TradeLineSell   float64 // 买卖盈利线（可为负数）
	Holding                       float64
	RateInAll                     float64 // 现货：该币种占总权益的比例；永续：以开仓价算该币种持仓占保证金百分比
	FundingRateUpdateTime         time.Time
}

func getTradeLineExtra(coin string, closeLine float64) (tradeLineExtra *TradeLineExtra) {
	now := time.Now()
	//value, ok := extras.Load(coin)
	//if ok && value != nil && value.(*TradeLineExtra).updateTime.Add(time.Minute*10).After(now) {
	//	return value.(*TradeLineExtra)
	//}
	tradeLineExtra = &TradeLineExtra{coin: coin, updateTime: now}
	crossRows, _ := model.AppDB.Model(model.Order{}).Select(`order_side, refresh_type,sum(price*abs(amount)),avg(price)`).
		Where(`coin=? and created_at>?`, coin, time.Now().Add(time.Minute*-180)).Group(`order_side, refresh_type`).Rows()
	if crossRows != nil {
		crossValues := make(map[string]float64)
		crossPrices := make(map[string]float64)
		for crossRows.Next() {
			var orderSide, refreshType string
			var valueAll, priceAvg float64
			_ = crossRows.Scan(&orderSide, &refreshType, &valueAll, &priceAvg)
			crossValues[orderSide+refreshType] = valueAll
			crossPrices[orderSide+refreshType] = priceAvg
		}
		err := crossRows.Close()
		if err != nil {
			util.Notice(fmt.Sprintf(`fail to close db query %s`, err.Error()))
		}
		// 当发生comp的单均利润小于closeShortMargin，同向comp占比越大，开仓line越高
		if crossValues[model.OrderSideBuy+model.FunctionComplement] > 300 &&
			crossPrices[model.OrderSideSell+model.FunctionCross] > 0 && crossValues[model.OrderSideBuy+model.FunctionCross] > 0 {
			lossRate := crossPrices[model.OrderSideBuy+model.FunctionComplement]/crossPrices[model.OrderSideSell+model.FunctionCross] - 1
			if lossRate > 0 {
				amtRate := crossValues[model.OrderSideBuy+model.FunctionComplement] / crossValues[model.OrderSideBuy+model.FunctionCross]
				extra := math.Max(closeLine, 0.001) * math.Pow(amtRate, 3) * lossRate * 400000
				extra = math.Max(extra, 3*lossRate)
				tradeLineExtra.buyExtra = math.Min(extra, 0.4)
				util.Notice(fmt.Sprintf(`comp extra buy %s compU %f compRate %f lossRate %f add %f`,
					coin, crossValues[model.OrderSideBuy+model.FunctionComplement], amtRate, lossRate, extra))
			}
		}
		if crossValues[model.OrderSideSell+model.FunctionComplement] > 300 &&
			crossPrices[model.OrderSideSell+model.FunctionComplement] > 0 && crossValues[model.OrderSideSell+model.FunctionCross] > 0 {
			lossRate := crossPrices[model.OrderSideBuy+model.FunctionCross]/crossPrices[model.OrderSideSell+model.FunctionComplement] - 1
			if lossRate > 0 {
				amtRate := crossValues[model.OrderSideSell+model.FunctionComplement] / crossValues[model.OrderSideSell+model.FunctionCross]
				extra := math.Max(closeLine, 0.001) * math.Pow(amtRate, 3) * lossRate * 400000
				extra = math.Max(extra, 3*lossRate)
				tradeLineExtra.sellExtra = math.Min(extra, 0.4)
				util.Notice(fmt.Sprintf(`comp extra sell %s compU %f compRate %f lossRate %f add %f`,
					coin, crossValues[model.OrderSideSell+model.FunctionComplement], amtRate, lossRate, extra))
			}
		}
	}
	//extras.Store(coin, tradeLineExtra)
	return
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
	var defaultKey, defaultSecret string
	for _, account := range uniAccounts {
		if account == nil {
			continue
		}
		defaultKey = account.Key
		defaultSecret = account.Secret
		value, ok := spotMarkets.Load(account.Key)
		if ok && value != nil {
			sm := value.(*spotMarket)
			for _, balance := range sm.balances {
				if balance != nil && balance.Amount != 0 {
					symbol := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
					valid := `false`
					setting := api.GetSetting(model.FunctionCross, balance.Market, symbol)
					if setting != nil && setting.Valid {
						valid = `true`
					} else if api.FilterCross(balance.Market, symbol) || balance.Amount == 0 {
						valid = `filter`
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
				valid := `false`
				setting := api.GetSetting(model.FunctionCross, position.Market, position.Currency)
				if setting != nil && setting.Valid {
					valid = `true`
				} else if api.FilterCross(position.Market, position.Currency) || position.Holding == 0 {
					valid = `filter`
				}
				if position.Holding != 0 {
					success, _, coin, _ := model.GetFromStandard(position.Market, position.Currency)
					if success {
						coinHold[coin] += position.Holding
						_, price := api.GetPriceForce(account.Key, account.Secret, position.Currency, position.Market)
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
		market := holding[i][0].(string)
		if coinPrice[coin] == 0 {
			_, coinPrice[coin] = api.GetPriceForce(defaultKey, defaultSecret, coin+model.UniStandardTail[model.MarketTypeSpot], market)
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
		case model.BinancePerp:
			value = createSpotMarket(key, secret, model.BinanceSpot)
		case model.BitgetPerp:
			value = createSpotMarket(key, secret, model.BitgetSpot)
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
				//if sm.balances[`FTT_USDT`] != nil {
				//	keepInU += sm.balances[`FTT_USDT`].UsdValue
				//}
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
		case model.BitgetSpot:
			value = createContractMarket(key, secret, model.BitgetPerp)
		default:
			value = createContractMarket(key, secret, market)
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
		//if market == model.OKEX && wsCross {
		//	go func() {
		//		wsCross = false
		//		util.Notice(fmt.Sprintf(`fail to order okex by ws, change ok ws cross for 10 mins`))
		//		time.Sleep(time.Minute * 10)
		//		wsCross = true
		//	}()
		//}
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
	if order == nil || setting == nil {
		return
	}
	var orders []*model.Order
	v, _ := util.LoadSyncMap(lastOrders, setting.Market, setting.Symbol)
	if v != nil {
		orders = v.([]*model.Order)
	}
	if orders == nil {
		orders = make([]*model.Order, lastOrderLength)
	}
	index := 0
	vIndex, _ := util.LoadSyncMap(lastOrderIndex, setting.Market, setting.Symbol)
	if vIndex != nil {
		index = vIndex.(int)
	}
	orders[index%lastOrderLength] = order
	index++
	util.StoreSyncMap(lastOrderIndex, index, setting.Market, setting.Symbol)
	noDealNum := 0
	tenMin, _ := time.ParseDuration(`10m`)
	second, _ := time.ParseDuration(`500ms`)
	for i, lastOrder := range orders {
		account := model.AppConfig.GetAccountFromKeyIndex(order.Market, ``, order.AccountIndex)
		now := time.Now()
		if lastOrder == nil || order.OrderTime.Add(tenMin).Before(now) || order.OrderTime.Add(second).After(now) || account == nil {
			continue
		}
		queryOrder := api.QueryOrderById(account.Key, account.Secret, lastOrder.Market, lastOrder.Symbol, lastOrder.OrderType, lastOrder.OrderId)
		if queryOrder == nil {
			continue
		}
		model.AppDB.Model(&queryOrder).Where(`order_id=?`, queryOrder.OrderId).Updates(
			map[string]interface{}{`deal_amount`: queryOrder.DealAmount, `deal_price`: queryOrder.DealPrice})
		util.Notice(fmt.Sprintf(`query last %s %s %s %f index %d`,
			queryOrder.Symbol, queryOrder.OrderId, queryOrder.Status, queryOrder.DealAmount, index))
		if queryOrder.DealAmount == 0 && order.Status != model.CarryStatusFail {
			noDealNum++
			if noDealNum > 3 {
				util.Notice(fmt.Sprintf(`no deal order %s %s %d %d stop at %d`,
					setting.Market, setting.Symbol, len(orders), noDealNum, index))
				setting.Valid = false
				setting.UpdatedAt = now
				util.StoreSyncMap(lastOrders, make([]*model.Order, lastOrderLength), setting.Market, setting.Symbol)
				util.StoreSyncMap(lastOrderIndex, 0, setting.Market, setting.Symbol)
				api.SendMails(`连续交易失败3次`, fmt.Sprintf(`%s %s交易失败%d次`, setting.Market, setting.Symbol, noDealNum))
				go func() {
					time.Sleep(time.Minute * 5)
					setting.Valid = true
				}()
				break
			}
		} else {
			orders[i] = nil
		}
	}
	//util.Notice(`---- add done %s`, setting.Symbol)
}

func liquidateBitgetPerp(account *model.Account) {
	now := time.Now().Unix()
	v, _ := liquidBitgetTime.Load(account.Key)
	if v != nil && now-v.(int64) < 3600 {
		return
	}
	success, positions, _, _ := api.GetPositions(account.Key, account.Secret, model.BitgetPerp)
	if success {
		liquidBitgetTime.Store(account.Key, now)
		for _, position := range positions {
			holding := math.Abs(position.Holding)
			if position.EntryPrice*holding < SmallInU {
				orderSide := model.OrderSideBuy
				if position.Holding > 0 {
					orderSide = model.OrderSideSell
				}
				util.Notice(fmt.Sprintf(`do liquidate bitgetperp %s %s price %f hold %f`,
					position.Currency, orderSide, position.EntryPrice, position.Holding))
				order := api.PlaceOrder(account.Key, account.Secret, orderSide, model.OrderTypeMarket, model.BitgetPerp,
					position.Currency, model.ReduceOnly, position.EntryPrice, position.EntryPrice, holding, false, nil, nil)
				order.RefreshType = model.FunctionBitgetLiq
				_, _, coin, _ := model.GetFromStandard(model.BitgetPerp, position.Currency)
				saveCross(order, coin, model.FunctionBitgetLiq, 0, 0, position.Holding)
			} else {
				util.Notice(fmt.Sprintf(`not liquidate bitgetperp for big perp %s %f %f value %f`,
					position.Currency, position.EntryPrice, position.Holding, position.EntryPrice*position.Holding))
			}
		}
	}
}

func saveCross(order *model.Order, coin, refreshType string, lineBuy, lineSell, holding float64) {
	if order != nil {
		order.Coin = coin
		order.LineBuy = lineBuy
		order.LineSell = lineSell
		order.Function = model.Open
		if math.Abs(holding) >= order.Amount {
			if (holding > 0 && order.OrderSide == model.OrderSideSell) || (holding < 0 && order.OrderSide == model.OrderSideBuy) {
				order.Function = model.Close
			}
		}
		order.RefreshType = refreshType
		model.AppDB.Save(order)
	}
}

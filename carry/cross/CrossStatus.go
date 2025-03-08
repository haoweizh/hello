package cross

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"sync"
	"time"
)

// const FundingRateBase = 12.0
const openValueLimit = 2000.0
const compLimitInU = 3000.0
const MarginULowLimit = 50000
const AccountOrderGap = 100 // account order no deal time in million-second
// const holdingLimitInU = 500000.0
// const openValueLimit = 2000
// const compLimitInU = 30000.0
// const InsufficientCodeBinance = `-2010`
// const standardScoreClose = 0.01 // 平仓标准利润,不得小于0
const lowestScore = -0.02
const standardScoreOpen = 0.002 // 开仓标准利润,不得小于0
const lastOrderLength = 8
const compTooBig = 70000.0
const SmallInU = 20
const CompLineInMoney = 50
const crossSlideSpot = 0.001
const crossSlidePerp = 0.001
const compSlide = 0.003

// TradeLineExtra 由于comp比例过高或亏损过多，需要增加的额外开仓数额
type TradeLineExtra struct {
	coin                string
	buyExtra, sellExtra float64
	updateTime          time.Time
}

//var extras = sync.Map{} // coin - *TradeLineExtra
//var InsufficientCodeOKEX = map[string]bool{`51008`: true, `51119`: true, `51120`: true, `51131`: true, `51502`: true,
//	`58350`: true, `59108`: true, `59200`: true}

var lastOrderIndex = &sync.Map{}  // market - symbol - index int
var lastOrders = &sync.Map{}      // market - symbol - []*Order
var compOrders = &sync.Map{}      // orderId - comp order
var spotMarkets = &sync.Map{}     // key - *spotMarket
var contractMarkets = &sync.Map{} // key - *contractMarket
var carryStatusMap = &sync.Map{}  // coin*market*symbol*key / CarryStatus
var carryCoinMap = &sync.Map{}    // coin*accountIndex - *carryCoin
var coinCrossing = &sync.Map{}

type contractMarket struct {
	key, market          string
	collateralsAvailable float64                    // 可用保证金U数
	contractValueInU     float64                    // 当前价格下开仓总额，以U计算
	accountValueInU      float64                    // 期货权益InU
	mmr                  float64                    // 维持保证金率
	reduceOnly           bool                       // 只减仓模式
	positions            map[string]*model.Position // symbol/position
}

type spotMarket struct {
	key, market     string
	availableU      float64
	accountValueInU float64
	balances        map[string]*model.Balance // symbol/balance
	collateral      *model.Collateral
	reduceOnly      bool // 只减仓模式
}

// (1+(1-剩余/周期）^2)*2/周期
func handledFRate(status *model.CarryStatus, marketInfo *model.MarketInfo, price float64) (got, delayed bool, fundingRate *model.FundingRate, handledFr float64) {
	if status.IsSpot {
		if status.Holding*price > model.SmallHolding {
			return true, false, &model.FundingRate{Rate: 0, UpdateTime: time.Now()}, 0
		} else {
			rate := marketInfo.InterestRate / -4
			return true, false, &model.FundingRate{Rate: rate, UpdateTime: time.Now()}, rate
		}
	} else {
		got, delayed, fundingRate = api.GetFundingRate(status.Account, status.Market, status.Symbol, false)
		if !got {
			return
		}
		hours := float64(marketInfo.FundingRateInterval) / 3600000.0
		leftHours := float64(fundingRate.ExpireTime-time.Now().Unix()) / 3600.0
		if fundingRate.ExpireTime < time.Now().Unix() {
			util.Log(util.LogLevelError, fmt.Sprintf(`funding rate expired %s %s %d`, status.Market, status.Symbol, fundingRate.ExpireTime))
			leftHours = 2.0
		}
		if leftHours > hours {
			leftHours = hours
		}
		//handledFr = fundingRate.Rate * (2/hours + (1.0-leftHours/hours)*(1.0-leftHours/hours)/2)
		handledFr = fundingRate.Rate * (1 + (1.0-leftHours/hours)*(1.0-leftHours/hours)) * 2 / hours
		if handledFr > 0.1 || handledFr < -0.1 {
			got = false
			util.Log(util.LogLevelError, fmt.Sprintf(`fatal error funding rate break %s %s %f %#v %d`,
				status.Market, status.Symbol, handledFr, fundingRate, marketInfo.FundingRateInterval))
			return
		}
	}
	return
}

func PostInit(market string) {
	switch market {
	case model.OKEX:
		// 更新资金费率
		api.InitMarketInfos(model.OKEX)
	}
}

func GetHoldings(accounts map[string]*model.Account) (holding [][]interface{}) {
	holding = make([][]interface{}, 0)
	coinHold := make(map[string]float64)
	coinValue := make(map[string]float64)
	volume := make(map[string]float64)
	monitorCoins := make(map[string]bool)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	coinSettings := api.GetCoinSettings(model.FunctionCross)
	if coinSettings == nil {
		return
	}
	coinSettings.Range(func(coin, value interface{}) bool {
		if value == nil {
			return true
		}
		settings := value.([]*model.Setting)
		for _, setting := range settings {
			if !setting.Valid || setting.MarketRelated != `` {
				monitorCoins[setting.Coin] = true
			}
			account := accounts[setting.Market]
			valid := `false`
			_, marketType, _, _ := model.GetFromStandard(setting.Market, setting.Symbol)
			_, price := api.GetPriceForce(setting.Market, setting.Symbol, false)
			if marketType == model.MarketTypeSpot {
				smValue, _ := spotMarkets.Load(account.Key)
				balance := smValue.(*spotMarket).balances[setting.Symbol]
				if (balance != nil && balance.Amount != 0) || !setting.Valid || setting.MarketRelated != `` {
					if setting.Valid && setting.MarketRelated == `` {
						valid = `true`
						if !setting.Liquidated {
							valid = `removed`
						}
					} else {
						util.Log(util.LogLevelInfo, fmt.Sprintf("setting still false %#v", setting))
					}
					amount := 0.0
					usdValue := 0.0
					if balance != nil {
						amount = balance.Amount
						if price > 0 {
							usdValue = price * amount
						} else {
							usdValue = balance.UsdValue
						}
					}
					holding = append(holding, []interface{}{setting.Market, coin.(string), setting.Symbol,
						fmt.Sprintf(`%.2f`, amount), math.Round(usdValue), valid, setting.MarketRelated})
					coinHold[coin.(string)] += amount * setting.GridAmount
					coinValue[coin.(string)] += math.Round(usdValue)
					volume[coin.(string)] += math.Abs(usdValue)
				}
			} else if marketType == model.MarketTypePerp {
				cm, _ := contractMarkets.Load(account.Key)
				position := cm.(*contractMarket).positions[setting.Symbol]
				if (position != nil && position.Holding != 0) || !setting.Valid || setting.MarketRelated != `` {
					if setting.Valid && setting.MarketRelated == `` {
						valid = `true`
					}
					if !setting.Liquidated {
						valid = `removed`
					}
					posHolding := 0.0
					if position != nil {
						posHolding = position.Holding
					}
					coinHold[coin.(string)] += posHolding * setting.GridAmount
					coinValue[coin.(string)] += math.Round(price * posHolding)
					volume[coin.(string)] += math.Abs(price * posHolding)
					holding = append(holding, []interface{}{setting.Market, coin, setting.Symbol, posHolding,
						math.Round(price * posHolding), valid, setting.MarketRelated})
				}
			}
		}
		return true
	})
	for _, account := range accounts {
		if account == nil {
			continue
		}
		value, ok := spotMarkets.Load(account.Key)
		if ok && value != nil {
			sm := value.(*spotMarket)
			for _, balance := range sm.balances {
				if balance != nil && balance.Amount != 0 {
					symbol := balance.Coin + model.UniStandardTail[model.MarketTypeSpot]
					valid := `extra`
					setting := api.GetSetting(model.FunctionCross, balance.Market, symbol)
					if setting != nil {
						continue
					} else if api.FilterCross(balance.Market, symbol) || balance.Amount == 0 {
						valid = `filter`
					}
					holding = append(holding, []interface{}{balance.Market, balance.Coin, symbol,
						fmt.Sprintf(`%.2f`, balance.Amount), math.Round(balance.UsdValue), valid, `未纳入监管`})
					coinHold[balance.Coin] += balance.Amount
					coinValue[balance.Coin] += math.Round(balance.UsdValue)
					volume[balance.Coin] += math.Abs(balance.UsdValue)
				}
			}
		}
		value, ok = contractMarkets.Load(account.Key)
		if ok && value != nil {
			cm := value.(*contractMarket)
			for _, position := range cm.positions {
				if position.Holding != 0 {
					valid := `extra`
					setting := api.GetSetting(model.FunctionCross, position.Market, position.Currency)
					if setting != nil {
						continue
					} else if api.FilterCross(position.Market, position.Currency) || position.Holding == 0 {
						valid = `filter`
					}
					if position.Holding != 0 {
						success, _, coin, _ := model.GetFromStandard(position.Market, position.Currency)
						if success {
							coinHold[coin] += position.Holding
							_, price := api.GetPriceForce(position.Market, position.Currency, false)
							holdingLine := []interface{}{position.Market, coin, position.Currency,
								position.Holding, math.Round(price * position.Holding), valid, `未纳入监管`}
							coinValue[coin] += math.Round(price * position.Holding)
							volume[coin] += math.Abs(price * position.Holding)
							holding = append(holding, holdingLine)
						}
					}
				}
			}
		}
	}
	for i := len(holding) - 1; i >= 0; i-- {
		for j := 0; j < i; j++ {
			if volume[holding[j][1].(string)] < volume[holding[j+1][1].(string)] {
				holding[j], holding[j+1] = holding[j+1], holding[j]
			}
		}
	}
	for i := len(holding) - 1; i >= 0; i-- {
		for j := 0; j < i; j++ {
			if (holding[j][5] != `false` && !monitorCoins[holding[j][1].(string)]) &&
				(holding[j+1][5] == `false` || monitorCoins[holding[j+1][1].(string)]) {
				holding[j], holding[j+1] = holding[j+1], holding[j]
			}
		}
	}
	for i := range holding {
		coin := holding[i][1].(string)
		market := holding[i][0].(string)
		symbol := holding[i][2].(string)
		_, price := api.GetPriceForce(market, symbol, false)
		_, marketType, _, _ := model.GetFromStandard(market, symbol)
		value, _ := util.LoadSyncMap(carryCoinMap, coin, `0`)
		currentStep := 0
		moneyCurStep := 0.0
		moneyPerStep := 0.0
		if value != nil {
			currentStep = value.(*model.CarryCoin).CurrentStep
			moneyCurStep = value.(*model.CarryCoin).MoneyCurStep
			moneyPerStep = value.(*model.CarryCoin).MoneyPerStep
			//util.Log(util.LogLevelError, fmt.Sprintf(`success get carry coin %d %f`, currentStep, moneyCurStep))
			//} else {
			//util.Log(util.LogLevelError, fmt.Sprintf(`fail to get carry coin %s %s`, coin, indexStr))
		}
		holding[i] = append(holding[i], math.Round(coinValue[coin]/10)*10)
		holding[i] = append(holding[i], price)
		holding[i] = append(holding[i], currentStep)
		holding[i] = append(holding[i], moneyCurStep)
		holding[i] = append(holding[i], moneyPerStep)
		_, _, fr := api.GetFundingRate(nil, market, symbol, true)
		marketInfo := model.GetMarketInfo(market, symbol)
		if fr != nil && marketInfo != nil {
			fundingStr := fmt.Sprintf(`%d:%d`, util.GetNow().Hour(), util.GetNow().Minute())
			if marketType == model.MarketTypePerp {
				updateTime := fr.UpdateTime.In(loc)
				fundingStr = fmt.Sprintf(`%e %dH %d:%d`,
					100*fr.Rate, marketInfo.FundingRateInterval/3600000, updateTime.Hour(), updateTime.Minute())
			} else {
				fundingStr = fmt.Sprintf(`%e`, 100*marketInfo.InterestRate/-4)
			}
			fundingFeeValue, _ := util.LoadSyncMap(&model.AppEnvironment.FundingFeeToday, strconv.Itoa(accounts[market].Index), market, symbol)
			if fundingFeeValue != nil {
				fundingStr = fundingStr + fmt.Sprintf("\n%.2fU", fundingFeeValue.(float64))
				if marketType == model.MarketTypeSpot {
					util.Log(util.LogLevelInfo, fmt.Sprintf(`add in all fee symbol %s %s %v`, market, symbol, fundingFeeValue))
				}
			}
			holding[i] = append(holding[i], fundingStr)
		} else {
			holding[i] = append(holding[i], ``)
		}
	}
	return
}

// GetCrossMarketValue keepInU: 不计入总价值的保留不交易币种
func GetCrossMarketValue(account *model.Account, market string, force bool) (inAllSpot, contractAccountValue,
	holdingSpot, borrowSpot, holdingFuture, marginAvailable float64) {
	value, ok := spotMarkets.Load(account.Key)
	if (!ok || value == nil) && force {
		switch market {
		case model.BinancePerp:
			value = createSpotMarket(account, model.BinanceSpot)
		case model.BitgetPerp:
			value = createSpotMarket(account, model.BitgetSpot)
		default:
			value = createSpotMarket(account, market)
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
						if sm.balances[symbol.(string)].UsdValue > 0 {
							holdingSpot += sm.balances[symbol.(string)].UsdValue
						} else if sm.balances[symbol.(string)].UsdValue < 0 {
							borrowSpot += sm.balances[symbol.(string)].UsdValue
						}
					}
					return true
				})
			}
		}
	}
	value, ok = contractMarkets.Load(account.Key)
	if (!ok || value == nil) && force {
		switch market {
		case model.BinanceSpot:
			value = createContractMarket(account, model.BinancePerp)
		case model.BitgetSpot:
			value = createContractMarket(account, model.BitgetPerp)
		default:
			value = createContractMarket(account, market)
		}
	}
	if value != nil {
		cm := value.(*contractMarket)
		if cm != nil {
			contractAccountValue = cm.accountValueInU
			marginAvailable = cm.collateralsAvailable
			holdingFuture = cm.contractValueInU
		}
	}
	return
}

// 某个交易对过去8次交易不成交次数达到3，暂停下单
// addLastCarry
func _(order *model.Order, setting *model.Setting) {
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
		queryOrder := api.QueryOrderById(account, lastOrder.Market, lastOrder.Symbol, lastOrder.OrderType, lastOrder.OrderId)
		if queryOrder == nil {
			continue
		}
		model.AppDB.Model(&queryOrder).Where(`order_id=?`, queryOrder.OrderId).Updates(
			map[string]interface{}{`deal_amount`: queryOrder.DealAmount, `deal_price`: queryOrder.DealPrice})
		util.Log(util.LogLevelInfo, fmt.Sprintf(`query last %s %s %s %f index %d`,
			queryOrder.Symbol, queryOrder.OrderId, queryOrder.Status, queryOrder.DealAmount, index))
		if queryOrder.DealAmount == 0 && order.Status != model.CarryStatusFail {
			noDealNum++
			if noDealNum > 3 {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`no deal order %s %s %d %d stop at %d`,
					setting.Market, setting.Symbol, len(orders), noDealNum, index))
				setting.Valid = false
				setting.MarketRelated = fmt.Sprintf(`trade fail %d`, noDealNum)
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

// syncFees
// 包括funding fee和interest
func syncFees(account *model.Account) {
	success, fundingFees := api.GetBills(account, time.Now().UnixMilli()-3600000, time.Now().UnixMilli())
	if success {
		for _, fee := range fundingFees {
			model.AppDB.Save(&fee)
		}
	}
	fees := make(map[string]float64)
	idxStr := strconv.Itoa(account.Index)
	fundingRows, _ := model.AppDB.Model(model.FundingFee{}).Select(`ccy,symbol,bal_chg`).
		Where(`ts>=? and market=? and index=?`, util.GetToday().UnixMilli(), account.Market, account.Index).Rows()
	if fundingRows != nil {
		for fundingRows.Next() {
			var coin, symbol string
			balChg := 0.0
			_ = fundingRows.Scan(&coin, &symbol, &balChg)
			price := 0.0
			if coin == `USDT` {
				price = 1.0
			} else {
				_, price = api.GetPriceForce(account.Market, symbol, false)
			}
			fees[symbol] += balChg * price
			//util.Log(util.LogLevelInfo, fmt.Sprintf(`set market symbol fee %s %s %s %f*%f=%f`,
			//	idxStr, account.Market, symbol, balChg, price, fees[symbol]))
		}
		for symbol, value := range fees {
			util.StoreSyncMap(&model.AppEnvironment.FundingFeeToday, value, idxStr, account.Market, symbol)
		}
		err := fundingRows.Close()
		if err != nil {
			return
		}
	}
}

func liquidateSmallContracts(account *model.Account, market string) {
	success, positions, _, _, _ := api.GetPositions(account, market)
	if success {
		for _, position := range positions {
			holding := math.Abs(position.Holding)
			if holding <= 0 {
				continue
			}
			if position.EntryPrice*holding < SmallInU {
				orderSide := model.OrderSideBuy
				if position.Holding > 0 {
					orderSide = model.OrderSideSell
				}
				util.Log(util.LogLevelInfo, fmt.Sprintf(`do liquidate market %s %s %s price %f hold %f`,
					market, position.Currency, orderSide, position.EntryPrice, position.Holding))
				orderParam := model.ReduceOnly
				if market == model.Gate {
					orderParam = model.CloseContract
				}
				order := api.PlaceOrder(account, orderSide, model.OrderTypeMarket, market, position.Currency, orderParam,
					model.FunctionLiq, ``, position.EntryPrice, position.EntryPrice, holding, false, nil)
				if order.HaveId() {
					saveCross(order, 0, 0, position.Holding)
				}
			}
		}
	}
}

func saveCross(order *model.Order, lineBuy, lineSell, holding float64) {
	if order != nil {
		order.LineBuy = lineBuy
		order.LineSell = lineSell
		order.Function = model.Open
		if math.Abs(holding) >= order.Amount {
			if (holding > 0 && order.OrderSide == model.OrderSideSell) || (holding < 0 && order.OrderSide == model.OrderSideBuy) {
				order.Function = model.Close
			}
		}
		model.AppDB.Save(order)
	}
}

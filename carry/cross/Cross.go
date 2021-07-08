package cross

import (
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
	"strings"
	"time"
)

// todo 1. postOrderCarry 2. usdRate < 20% revert
const loseRateMax = -0.005
const winRateMin = 0.005

var spotBalance map[string]map[string]float64         // key - market - spot account balance in usd
var swapBalance map[string]map[string]float64         // key - market - swap account balance in usd
var balances map[string]map[string][]*model.Balance   // key - market - balances
var positions map[string]map[string][]*model.Position // key - market - positions

func createFromPosition(key, secret string, setting *model.Setting) *CarryStatus {
	if positions[key] == nil {
		positions[key] = make(map[string][]*model.Position)
	}
	if positions[key][setting.Market] == nil {
		success, apiPositions, posBalance := api.GetPositions(key, secret, setting.Market)
		if success {
			positions[key][setting.Market] = apiPositions
			swapBalance[key][setting.Market] = posBalance
		}
	}
	for _, position := range positions[key][setting.Market] {
		if position == nil || position.Currency != setting.Symbol {
			continue
		}
		return &CarryStatus{Market: setting.Market, Symbol: setting.Symbol, LimitSell: math.NaN(), LimitBuy: math.NaN(), Holding: position.Free}
	}
	return nil
}

func createFromBalance(key, secret string, setting *model.Setting) *CarryStatus {
	if balances[key] == nil {
		balances[key] = make(map[string][]*model.Balance)
	}
	if balances[key][setting.Market] == nil {
		success, apiBalances, totalInUsd, collateral := api.GetBalances(key, secret, setting.Market)
		if success && totalInUsd > 0 {
			setCollateral(key, collateral)
			balances[key][setting.Market] = apiBalances
			spotBalance[key][setting.Market] = totalInUsd
		}
	}
	for _, balance := range balances[key][setting.Market] {
		if balance == nil || balance.Coin != setting.Coin {
			continue
		}
		return &CarryStatus{Market: setting.Market, Symbol: setting.Symbol, LimitSell: balance.AvailableWithBorrow,
			LimitBuy: math.NaN(), Holding: balance.Amount, UsdValue: balance.UsdValue,
			RateInAll: balance.UsdValue / spotBalance[key][setting.Market]}
	}
	return nil
}

func initStatus(key, secret string, setting *model.Setting) {
	if setting == nil {
		return
	}
	tailSpot := model.GetSpotTail(setting.Market)
	tailPerp := model.GetPerpTail(setting.Market)
	var carryStatus *CarryStatus
	fundingRate := 0.0
	if setting.Symbol[len(setting.Symbol)-len(tailSpot):] == tailSpot {
		carryStatus = createFromBalance(key, secret, setting)
	} else if setting.Symbol[len(setting.Symbol)-len(tailPerp):] == tailPerp {
		carryStatus = createFromPosition(key, secret, setting)
		_, fundingRate = api.GetFundingRate(setting.Market, setting.Symbol, nil)
		fundingRate *= 0.9
	}
	if carryStatus == nil {
		return
	}
	getTick, tick := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	status[setting.Coin][setting.Market][setting.Symbol][key] = carryStatus
	if getTick && carryStatus.UsdValue == 0 {
		carryStatus.UsdValue = carryStatus.Holding * tick.Bids[0].Price
		marketBalance := swapBalance[key][setting.Market]
		if marketBalance == 0 {
			marketBalance = spotBalance[key][setting.Market]
		}
		if marketBalance > 0 {
			carryStatus.RateInAll = carryStatus.UsdValue / marketBalance
		}
	}
	now := time.Now().Unix()
	resetTime := getOKTradeMaxResetTime(key, setting.Symbol) + 600
	if setting.Market == model.OKEX && now > resetTime {
		setOKTradeMaxResetTime(key, setting.Symbol)
		getMax, maxBuy, maxSell := api.GetMaxSize(key, secret, setting.Symbol)
		if getMax {
			carryStatus.LimitSell = maxSell
			carryStatus.LimitBuy = maxBuy
		}
	}

	jump := 7.0
	if carryStatus.RateInAll > 0 {
		carryStatus.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*carryStatus.RateInAll), winRateMin) + fundingRate
		carryStatus.TradeLineSell = math.Max(setting.OpenShortMargin*(0.5-jump*carryStatus.RateInAll), loseRateMax) - fundingRate
	} else {
		carryStatus.TradeLineBuy = math.Max(setting.OpenShortMargin*(0.5+jump*carryStatus.RateInAll), loseRateMax) + fundingRate
		carryStatus.TradeLineSell = math.Max(setting.OpenShortMargin*(0.5-jump*carryStatus.RateInAll), winRateMin) - fundingRate
	}
	keys, _ := model.AppConfig.GetKeys(setting.Market)
	doReverts := strings.Split(model.AppConfig.CarryClose, `,`)
	accountRates := strings.Split(model.AppConfig.AccountRate, `,`)
	for i := 1; i < len(keys); i++ {
		if keys[i] == key {
			if doReverts[i] == `true` {
				carryStatus.TradeLineBuy = 1
				carryStatus.TradeLineSell = 1
			} else {
				rate, _ := strconv.ParseFloat(accountRates[i], 64)
				carryStatus.TradeLineBuy *= rate
				carryStatus.TradeLineSell *= rate
			}
		}
	}
	if setting.Market == model.OKEX {
		collateral := GetCollateral(key)
		if collateral != nil && collateral.Available > 0 && (collateral.Available-collateral.Occupied)/collateral.Available < 0.1 {
			util.Notice(`doRevert true %s %f %f`, key, collateral.Available, collateral.Occupied, collateral.Rate)
			carryStatus.TradeLineBuy = 1
			carryStatus.TradeLineSell = 1
		}
	}
}

func clearCarry() {
	if status == nil {
		status = make(map[string]map[string]map[string]map[string]*CarryStatus)
	}
	balances = make(map[string]map[string][]*model.Balance)
	positions = make(map[string]map[string][]*model.Position)
	coinSettings := model.GetCoinSettings(model.FunctionCross)
	for coin, settings := range coinSettings {
		if status[coin] == nil {
			status[coin] = make(map[string]map[string]map[string]*CarryStatus)
		}
		for _, setting := range settings {
			if setting == nil {
				continue
			}
			if status[coin][setting.Market] == nil {
				status[coin][setting.Market] = make(map[string]map[string]*CarryStatus)
			}
			if status[coin][setting.Market][setting.Symbol] == nil {
				status[coin][setting.Market][setting.Symbol] = make(map[string]*CarryStatus)
			}
			keys, secrets := model.AppConfig.GetKeys(setting.Market)
			for i, key := range keys {
				initStatus(key, secrets[i], setting)
			}
		}
	}
}

// ProcessCross todo 计算fundingRate后30s不下单
var ProcessCross = func(setting *model.Setting, tick *model.BidAsk) {
	million := util.GetNowUnixMillion()
	delayTick := int64(0)
	if tick != nil {
		delayTick = million - int64(tick.Ts)
	}
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || model.AppPause ||
		(model.AppConfig.Env != `test` && (model.AppConfig.Handle != `1` || delayTick > 30)) {
		return
	}
	settings := model.GetCoinSetting(setting.Function, setting.SymbolRelated)
	if settings == nil || len(settings) == 0 {
		return
	}
	for _, s := range settings {
		model.AppMarkets.GetBidAsk(s.Symbol, s.Market)
	}
}

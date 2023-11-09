package Turtle

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"time"
)

var ProcessBoost = func(setting *model.Setting, tick *model.BidAsk) {
	if !api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, true) {
		defer api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, false)
	} else {
		return
	}
	now := util.GetNowUnixMillion()
	maintaining, _ := model.ChannelMaintaining.Load(setting.Market)
	success, _, _, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
		(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 120000) ||
		!success {
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	if setting.Chance != 0 && setting.PriceX == 0 {
		util.Notice(fmt.Sprintf(`boost return no last priceX %s %s %d %e %d %e`,
			setting.Market, setting.Symbol, setting.Chance, setting.PriceX, setting.Chance, setting.PriceX))
		return
	}
	data, _ := api.GetTurtleData(account, setting.Function, setting.Market, setting.Symbol, setting.Far, setting.Near,
		setting.Seconds, setting.ChanceLimit, setting.AmountRate, true, setting.Chance == 0 && setting.SymbolRelated == model.SettingTurtleRemoved)
	if data == nil || setting == nil || model.AppConfig.Env == `test` {
		if time.Now().Minute() == 0 && time.Now().Second() == 0 {
			util.Notice(fmt.Sprintf(`boost return no turtle boost %s %s`, setting.Market, setting.Symbol))
		}
		return
	}
	if setting.Function == model.FunctionBoost {
		dataLiquid, _ := api.GetTurtleData(account, `calc_n`, setting.Market, setting.Symbol, setting.FarCombine,
			setting.NearCombine, setting.SecondsCombine, setting.ChanceLimitCombine, setting.AmountRateCombine,
			true, setting.Chance == 0 && setting.SymbolRelated == model.SettingTurtleRemoved)
		if dataLiquid != nil {
			data.N = dataLiquid.N
			data.Amount = dataLiquid.Amount
		}
	}
	if !data.OrderCleared {
		api.ClearOrders(account.Key, account.Secret, setting.Market, setting.Symbol, map[string]bool{model.OrderTypeTrailStop: true})
		data.OrderCleared = true
		util.Notice(fmt.Sprintf(`boost return not cleared %s %s %v`, setting.Market, setting.Symbol, data.OrderCleared))
		return
	}
	canOpen, turtleCoins := api.CanOpenTurtle(setting, data)
	if api.HandleOrders(account.Key, account.Secret, setting.Market, setting.Symbol, []*model.Setting{setting}, []*model.TurtleData{data}) ||
		api.CheckBreak(account, setting.Market, setting.Symbol, []*model.Setting{setting}, []*model.TurtleData{data}, tick) {
		return
	}
	if data.Liquidated || data.Amount == 0 || data.N == 0 {
		return
	}
	msgKey := model.GetMsgKey(model.FunctionBoost, setting.Market, setting.Symbol)
	msg := fmt.Sprintf("[%d-%d %d:%d]%s N-Volume %f 可开%v 币种数:%d/%d 单仓数量:%e bid-ask %e %e BOOST:仓数/币种上限/开仓量/持仓价/平过仓"+
		" %d of %d/%e/%e/%v %s [%d seconds %d %d]:N:%e\n%d: TERM%d:%e-%e TERM%d:%e-%e",
		data.TurtleTime.Month(), data.TurtleTime.Day(), time.Now().Hour(), time.Now().Minute(), msgKey, data.NVolume,
		canOpen, int(turtleCoins), int(setting.AmountLimit), data.Amount, tick.Bids[0].Price, tick.Asks[0].Price,
		setting.Chance, setting.ChanceLimit, setting.GridAmount, setting.PriceX, data.Liquidated, data.GetIds(),
		setting.SecondsCombine, setting.FarCombine, setting.NearCombine, data.N,
		setting.Seconds, data.DaysFar, data.LowFar, data.HighFar, data.DaysNear, data.LowNear, data.HighNear)
	util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	placeTurtleLong(account, model.OrderTypeStop, data, setting, tick, canOpen, true, false)
	placeTurtleShort(account, model.OrderTypeStop, data, setting, tick, canOpen, true, false)
	if (data.OrderTrail == nil || len(data.OrderTrail) == 0) && setting.Function == model.FunctionBoost {
		if setting.Chance == setting.ChanceLimit {
			util.Notice(fmt.Sprintf(`place trail liquid %s %s %s amt %f`, model.OrderSideSell, setting.Market, setting.Symbol, setting.GridAmount))
			data.OrderTrail = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideSell, model.OrderTypeTrailStop,
				setting.Market, setting.Symbol, ``, model.FunctionBoost, 1.1*setting.PriceX, 0.03, setting.GridAmount, setting)
		} else if setting.Chance == -1*setting.ChanceLimit {
			util.Notice(fmt.Sprintf(`place trail liquid %s %s %s amt %f`, model.OrderSideBuy, setting.Market, setting.Symbol, setting.GridAmount))
			data.OrderTrail = api.MustPlaceOrder(account.Key, account.Secret, model.OrderSideBuy, model.OrderTypeTrailStop,
				setting.Market, setting.Symbol, ``, model.FunctionBoost, 0.9*setting.PriceX, 0.03, setting.GridAmount, setting)
		}
		if data.OrderTrail != nil {
			for _, order := range data.OrderTrail {
				order.Function = model.Close
				model.AppDB.Save(order)
			}
		}
	}
	if handleBreak(setting, data, data.OrderLong, data.BreakLong) ||
		handleBreak(setting, data, data.OrderShort, data.BreakShort) ||
		handleBreak(setting, data, data.OrderTrail, data.BreakTrail) {
		api.ClearExtraOrders(account.Key, account.Secret, setting.Market, setting.Symbol, []*model.TurtleData{data})
	}
}

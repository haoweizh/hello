package Turtle

import (
	"hello/model"
)

var ProcessBoost = func(setting *model.Setting, tick *model.BidAsk) {
	//if !api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, true) {
	//	defer api.CheckSetProcessing(setting.Function, setting.Market, setting.Symbol, false)
	//} else {
	//	return
	//}
	//now := util.GetNowUnixMillion()
	//maintaining, _ := model.ChannelMaintaining.Load(setting.Market)
	//success, _, _, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	//if tick == nil || tick.Asks == nil || tick.Bids == nil || model.AppConfig.Handle != `1` ||
	//	(maintaining != nil && maintaining.(bool)) || (model.AppConfig.Env != `test` && now-int64(tick.Ts) > 120000) ||
	//	!success {
	//	return
	//}
	//if setting.Chance == 0 && setting.SymbolRelated == model.SettingTurtleRemoved {
	//	return
	//}
	//if setting.Chance != 0 && setting.PriceX == 0 {
	//	util.Notice(fmt.Sprintf(`combine return no last priceX %s %s %d %e %d %e`,
	//		setting.Market, setting.Symbol, setting.Chance, setting.PriceX, setting.Chance, setting.PriceX))
	//	return
	//}
	//account := model.AppConfig.GetAccounts(setting.Market)[0]
	//data, _ := api.GetTurtleData(account, setting, true)
	//if data == nil || data.N == 0 || data.Amount == 0 || setting == nil || model.AppConfig.Env == `test` {
	//	if time.Now().Second() == 0 {
	//		util.Notice(fmt.Sprintf(`combine return no turtle combine turtle %s %s`, setting.Market, setting.Symbol))
	//	}
	//	return
	//}
	//if !data.OrderCleared {
	//	api.ClearOrders(account.Key, account.Secret, setting.Market, setting.Symbol)
	//	data.OrderCleared = true
	//	util.Notice(fmt.Sprintf(`combine return not cleared %s %s %v`, setting.Market, setting.Symbol, data.OrderCleared))
	//	return
	//}
	//canOpen, turtleCoins := api.CanOpenTurtle(setting, data)
	//if api.HandleOrders(account.Key, account.Secret, setting.Market, setting.Symbol, []*model.Setting{setting}, []*model.TurtleData{data}) ||
	//	api.CheckBreak(account, setting.Market, setting.Symbol, []*model.Setting{setting}, []*model.TurtleData{data}, tick) {
	//	//util.Notice(fmt.Sprintf(`combine return handle or break %s %s`, market, symbol))
	//	return
	//}
	//msgKey := model.GetMsgKey(model.FunctionBoost, setting.Market, setting.Symbol)
	//msg := fmt.Sprintf("[%d-%d %d:%d]%s N-Volume %f 可开%v 币种数:%d/%d 单仓数量:%e bid-ask %e %e BOOST:仓数/币种上限/开仓量/持仓价/平过仓"+
	//	" %d of %d/%e/%e/%v %s N:%e\n%d: TERM%d:%e-%e TERM%d:%e-%e %d: TERM%d:%e-%e TERM%d:%e-%e",
	//	data.TurtleTime.Month(), data.TurtleTime.Day(), time.Now().Hour(), time.Now().Minute(), msgKey, data.NVolume,
	//	canOpen, int(turtleCoins), int(setting.AmountLimit), data.Amount, tick.Bids[0].Price, tick.Asks[0].Price,
	//	setting.Chance, setting.ChanceLimit, setting.GridAmount, setting.PriceX, data.Liquidated, data.GetIds(), data.N,
	//	setting.Seconds, data.DaysFar, data.LowFar, data.HighFar, data.DaysNear, data.LowNear, data.HighNear,
	//	setting.SecondsCombine, data.DaysFarRe, data.LowFarRe, data.HighFarRe, data.DaysNearRe, data.LowNearRe, data.HighNearRe)
	//util.StoreSyncMap(&model.CarryInfo, msg, account.Key, msgKey)
	//placeTurtleLong(account, model.OrderTypeStop, dataNormal, settingNormal, tick, canOpen)
	//placeTurtleShort(account, model.OrderTypeStop, dataNormal, settingNormal, tick, canOpen)
	//placeTurtleLong(account, model.OrderTypeLimit, dataCombine, settingCombine, tick, canOpen)
	//placeTurtleShort(account, model.OrderTypeLimit, dataCombine, settingCombine, tick, canOpen)
	//if handleAllBreak(settings, turtleData) {
	//	for _, data := range turtleData {
	//		data.AdjustChecked = true
	//	}
	//	api.ClearExtraOrders(account.Key, account.Secret, market, symbol, turtleData)
	//}
}

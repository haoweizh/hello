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
	////if !dataCombine.AdjustChecked && !dataNormal.AdjustChecked {
	////	util.Notice(fmt.Sprintf(`combine return not adjusted %s %s`, market, symbol))
	////	return
	////}
	////价格不一样：big=true
	////价格一样：仓数相加=0时big=false；仓数相加≠0时big=true
	//model.ResetBig(settingNormal, dataCombine, dataNormal)
	//msgKey := model.GetMsgKey(model.FunctionCombineTurtle, market, symbol)
	//msg := fmt.Sprintf("[%d-%d %d:%d]%s N-Volume %f 可开%v 币种数:%d/%d "+
	//	"单仓数量:%e bid-ask %e %e \n海龟:仓数/持仓量/开仓价/今日平仓 %d of %d/%e/%e/%v %s %d big:%d 日:%e-%e %d日:%e-%e N:%e"+
	//	"\n龟汤:仓数/持仓量/开仓价/今日平仓 %d of %d/%e/%e/%v %s%d big:%d 日:%e-%e %d日:%e-%e N:%e",
	//	dataCombine.TurtleTime.Month(), dataCombine.TurtleTime.Day(), time.Now().Hour(), time.Now().Minute(), msgKey,
	//	dataCombine.NVolume, canOpen, int(turtleCoins), int(settingCombine.AmountLimit), dataCombine.Amount,
	//	tick.Bids[0].Price, tick.Asks[0].Price,
	//	settingNormal.Chance, settingNormal.ChanceLimit, settingNormal.GridAmount, settingNormal.PriceX, dataNormal.Liquidated,
	//	dataNormal.GetIds(), dataNormal.Big, dataNormal.DaysFar, dataNormal.LowFar, dataNormal.HighFar,
	//	dataNormal.DaysNear, dataNormal.LowNear, dataNormal.HighNear, dataNormal.N,
	//	settingCombine.Chance, settingCombine.ChanceLimit, settingCombine.GridAmount, settingCombine.PriceX,
	//	dataCombine.Liquidated, dataCombine.GetIds(), dataCombine.Big, dataCombine.DaysFar, dataCombine.LowFar,
	//	dataCombine.HighFar, dataCombine.DaysNear, dataCombine.LowNear, dataCombine.HighNear, dataCombine.N)
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

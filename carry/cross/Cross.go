package cross

import (
	"hello/model"
	"hello/util"
)

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

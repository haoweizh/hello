package carry

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strings"
	"sync"
)

// open amount: min(usd value - all current other coin value, best bid amount, best ask amount)
// open price: lose 0.0014 price distance < last 6 hour funding rate * 4, ioc
// close price: win 0.0014 price distance or last 6 hour funding rate * 4, market

var carryLock sync.Mutex
var carrying bool
var wealth, usdValue float64

type symbolRate struct {
	amount, score, holding, rateSum float64
}

var perpSnapshot map[string]*symbolRate = make(map[string]*symbolRate)

func isCarrying() (value bool) {
	carryLock.Lock()
	defer carryLock.Unlock()
	return carrying
}

func setCarrying(value bool) {
	carryLock.Lock()
	defer carryLock.Unlock()
	carrying = value
}

//ProcessCarry
var _ = func(setting *model.Setting) {
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(setting.SymbolRelated, setting.Market)
	now := util.GetNowUnixMillion()
	if tickPerp == nil || tickRelated == nil || tickPerp.Asks == nil || tickPerp.Bids == nil ||
		tickRelated.Asks == nil || tickRelated.Bids == nil || model.AppConfig.Handle != `1` ||
		model.AppPause || now-int64(tickRelated.Ts) > 1000 || now-int64(tickPerp.Ts) > 1000 {
		return
	}
	if setting == nil || isCarrying() {
		return
	}
	setCarrying(true)
	defer setCarrying(false)
	rates, _ := api.GetFundingRate(setting.Market, setting.Symbol)
	rateSum := 0.0
	for _, item := range rates.([]*model.FundingRate) {
		rateSum += item.Rate
	}
	accountUSD := model.AppAccounts.GetAccount(setting.Market, `USD`)
	if accountUSD == nil {
		return
	}
	if wealth == 0 || usdValue == 0 {
		balances := api.GetBalance(``, ``, setting.Market, 0)
		for _, value := range balances {
			wealth += value.UsdValue
			if strings.ToLower(value.Coin) == `usd` {
				usdValue = value.Amount
			}
		}
		util.Notice(fmt.Sprintf(`[carry] set wealth %f usd %f`, wealth, usdValue))
		return
	}
	openAmount := math.Min((accountUSD.Free-wealth/2)/tickPerp.Asks[0].Price,
		math.Min(tickPerp.Bids[0].Amount/2, tickRelated.Asks[0].Amount/2))
	//score := tickPerp.Bids[0]
	perpSnapshot[setting.Symbol] = &symbolRate{
		rateSum: rateSum,
		score:   0,
		holding: setting.GridAmount,
		amount:  openAmount,
	}
}

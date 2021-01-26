package carry

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

// open amount: min(usd value - all current other coin value, best bid amount, best ask amount)
// open price: lose 0.0014 price distance < last 6 hour funding rate * 4, ioc
// close price: win 0.0014 price distance or last 6 hour funding rate * 4, market
const FTXFee = 0.0007

var carryLock sync.Mutex
var carrying bool
var wealth, usdValue float64

var perpSnapshot = make(map[string]float64)

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
var ProcessCarry = func(setting *model.Setting) {
	_, tickPerp := model.AppMarkets.GetBidAsk(setting.Symbol, setting.Market)
	_, tickRelated := model.AppMarkets.GetBidAsk(setting.GetRelatedSymbol(), setting.Market)
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
	openAmount := math.Min((usdValue-wealth/2)/tickPerp.Asks[0].Price,
		math.Min(tickPerp.Bids[0].Amount/2, tickRelated.Asks[0].Amount/2))
	score := 1 - tickRelated.Asks[0].Price*(1+2*FTXFee)/tickPerp.Bids[0].Price + rateSum
	perpSnapshot[setting.Symbol] = score
	highestScore := 0.0
	highestSymbol := ``
	for symbol, value := range perpSnapshot {
		if value > highestScore {
			highestSymbol = symbol
		}
	}
	if highestSymbol == setting.Symbol {
		for s, value := range perpSnapshot {
			util.Notice(fmt.Sprintf(`size: %d %s score: %f rateSum %f holding %f open: %f`,
				len(perpSnapshot), s, value, rateSum, setting.GridAmount, openAmount))
		}
		time.Sleep(time.Minute * 5)
	}
}

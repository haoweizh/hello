package carry

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"sync"
	"time"
)

var positions = &sync.Map{}
var balances = &sync.Map{}

const moveLimitU = 20

func initPos(account *model.Account, market string) {
	success, pos, _, _, _ := api.GetPositions(account, market)
	if success {
		positions.Clear()
		for _, position := range pos {
			positions.Store(position.Currency, position)
		}
	}
}

func initBalances(account *model.Account, market string) {
	success, bal, _, _ := api.GetBalances(account, market)
	if success {
		balances.Clear()
		for _, balance := range bal {
			balances.Store(balance.Coin+model.UniStandardTail[model.MarketTypeSpot], balance)
		}
	}
}

var ProcessMove = func(setting *model.Setting, tick *model.BidAsk) {
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || setting.Valid == false ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) {
		return
	}
	if model.AppEnvironment.Moving {
		return
	}
	model.AppEnvironment.Moving = true
	defer func() {
		model.AppEnvironment.Moving = false
	}()
	marketInfo := model.GetMarketInfo(setting.Market, setting.Symbol)
	if marketInfo == nil {
		util.Log(util.LogLevelError, fmt.Sprintf(`fail to get market info for move %s %s`, setting.Market, setting.Symbol))
		return
	}
	account := model.AppConfig.GetAccounts(setting.Market)[0]
	_, marketType, _, _ := model.GetFromStandard(setting.Market, setting.Symbol)
	holding := 0.0
	if marketType == model.MarketTypeSpot {
		bal, _ := balances.Load(setting.Symbol)
		if bal != nil {
			holding = bal.(*model.Balance).Amount
		} else {
			initBalances(account, setting.Market)
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to get holding bal %s %s`, setting.Market, setting.Symbol))
			return
		}
	} else if marketType == model.MarketTypePerp {
		pos, _ := positions.Load(setting.Symbol)
		if pos != nil {
			holding = pos.(*model.Position).Holding
		} else {
			initPos(account, setting.Market)
			util.Log(util.LogLevelError, fmt.Sprintf(`fail to get holding pos %s %s`, setting.Market, setting.Symbol))
			return
		}
	}
	if holding*tick.Bids[0].Price > moveLimitU {
		holding = moveLimitU / tick.Bids[0].Price
	}
	if holding*tick.Bids[0].Price < -moveLimitU {
		holding = -moveLimitU / tick.Bids[0].Price
	}
	if math.Abs(holding)*tick.Bids[0].Price < 10 {
		return
	}
	orderSideFrom := model.OrderSideSell
	orderSideTo := model.OrderSideBuy
	if holding < 0 {
		orderSideFrom = model.OrderSideBuy
		orderSideTo = model.OrderSideSell
	}
	if tick.Asks[0].Price-tick.Bids[0].Price <= marketInfo.PriceIncrement {
		util.LogLess(util.LogLevelInfo, fmt.Sprintf(`price near %s %s %f %f`,
			setting.Market, setting.Symbol, tick.Bids[0].Price, tick.Asks[0].Price))
		return
	}
	price := tick.Bids[0].Price/2 + tick.Asks[0].Price/2
	go api.PlaceOrder(account, orderSideFrom, model.OrderTypeLimit, setting.Market, setting.Symbol, ``, model.FunctionMove,
		price, price, math.Abs(holding), true, nil)
	accountTo := &model.Account{Index: 1, Market: setting.Market, Key: model.AppConfig.ToKey, Secret: model.AppConfig.ToSecret, OKPhase: model.AppConfig.ToPhase}
	api.PlaceOrder(accountTo, orderSideTo, model.OrderTypeLimit, setting.Market, setting.Symbol, ``, model.FunctionMove,
		price, price, math.Abs(holding), true, nil)
	if marketType == model.MarketTypeSpot {
		bal, _ := balances.Load(setting.Symbol)
		if bal != nil {
			bal.(*model.Balance).Amount -= holding
		}
	} else if marketType == model.MarketTypePerp {
		pos, _ := positions.Load(setting.Symbol)
		if pos != nil {
			pos.(*model.Position).Holding -= holding
		}
	}
	time.Sleep(time.Second * 3)
}

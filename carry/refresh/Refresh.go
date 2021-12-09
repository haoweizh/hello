package refresh

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"sync"
	"time"
)

var buyMore float64
var refreshLock sync.Mutex
var chance int64

// ProcessRefresh
// setting.Chance 要进行多少次刷单交易
// setting.GridAmount 下单个数
// setting.AmountLimit 下单后休息时间in million second
// setting.PriceX 买卖单价差大于等于多少才会下单
var ProcessRefresh = func(setting *model.Setting, tick *model.BidAsk) {
	defer refreshLock.Unlock()
	refreshLock.Lock()
	//million := util.GetNowUnixMillion()
	//delayTick := int64(0)
	//if tick != nil {
	//	delayTick = million - int64(tick.Ts)
	//}
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	if tick == nil || tick.Asks == nil || tick.Bids == nil || setting == nil || model.AppPause ||
		(model.AppConfig.Env != `test` && model.AppConfig.Handle != `1`) || len(keys) != 1 ||
		tick.Asks[0].Price-tick.Bids[0].Price < setting.PriceX { //} || model.IsTickTimeout(setting.Market, delayTick) {
		return
	}
	placeRefresh(setting, keys[0], secrets[0], (tick.Bids[0].Price+tick.Asks[0].Price)/2)
	time.Sleep(time.Duration(setting.AmountLimit) * time.Millisecond)
}

func placeRefresh(setting *model.Setting, key, secret string, price float64) {
	chance++
	if chance > setting.Chance {
		return
	}
	amountBuy := setting.GridAmount
	amountSell := setting.GridAmount
	if buyMore > 0 {
		amountBuy -= buyMore
	} else {
		amountSell += buyMore
	}
	util.Notice(fmt.Sprintf(`place refresh buy %f sell %f index: %d`, amountBuy, amountSell, chance))
	if amountBuy > 0 {
		go api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
			``, model.FunctionRefresh, price, price, amountBuy, true, false, postOrderRefresh, setting)
	}
	if amountSell > 0 {
		go api.PlaceOrder(key, secret, model.OrderSideSell, model.OrderTypeLimit, setting.Market, setting.Symbol, setting.Symbol,
			``, model.FunctionRefresh, price, price, amountSell, true, false, postOrderRefresh, setting)
	}
}

var postOrderRefresh = func(order *model.Order, setting *model.Setting) {
	keys, secrets := model.AppConfig.GetKeys(setting.Market)
	var key, secret string
	for i, value := range keys {
		if value == order.AmountType {
			key = value
			secret = secrets[i]
		}
	}
	time.Sleep(time.Duration(setting.AmountLimit) * time.Millisecond)
	if order != nil && order.Status == model.CarryStatusWorking {
		order = api.QueryOrderById(key, secret, setting.Market, setting.Symbol, setting.Symbol, model.OrderTypeLimit, order.OrderId)
		if order.OrderSide == model.OrderSideBuy {
			buyMore += order.DealAmount
		} else if order.OrderSide == model.OrderSideSell {
			buyMore -= order.DealAmount
		}
		util.Notice(fmt.Sprintf(`refresh %s <%f %f> buy more: %f`, order.OrderSide, order.Amount, order.DealAmount, buyMore))
		if order.Status == model.CarryStatusWorking {
			api.MustCancel(key, secret, setting.Market, setting.Symbol, setting.Symbol, model.OrderTypeLimit, order.OrderId, true)
		}
	}
}

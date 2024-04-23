package carry

import (
	"fmt"
	"hello/api"
	"hello/carry/Turtle"
	"hello/carry/cross"
	"hello/carry/follow"
	"hello/carry/grid"
	"hello/carry/hang"
	"hello/carry/monitor"
	"hello/carry/queue"
	"hello/model"
	"hello/util"
	"sync"
	"time"
)

var feeIndex int
var balanceMaintainDay = util.GetNow()

// MaintainBalance
func _(key, secret string) {
	for {
		markets := api.GetMarkets()
		balances := make([]*model.Balance, 0)
		balanceTime := util.GetNow()
		duration, _ := time.ParseDuration(`-24h`)
		balanceTime = balanceTime.Add(duration)
		for _, market := range markets {
			if balanceTime.After(balanceMaintainDay) {
				balances = append(balances, api.GetTransfers(key, secret, market)...)
				balanceMaintainDay = util.GetNow()
			}
			_, balance, _, _ := api.GetBalances(key, secret, market)
			balances = append(balances, balance...)
			//for _, item := range balance {
			//	key := fmt.Sprintf(`[balance]%s_%s`, item.Market, item.Coin)
			//	if item.Amount > 10 || strings.ToLower(item.Coin) == `btc` {
			//		model.SetCarryInfo(key, fmt.Sprintf(`%f %s`, item.Amount, item.BalanceTime.String()))
			//	}
			//}
			//util.Notice(fmt.Sprintf(`get balances %s %d`, market, len(balances)))
		}
		//for _, balance := range balances {
		//util.Notice(fmt.Sprintf(`balance info: %s %s %s %f %f %s`,
		//	balance.ID, balance.Market, balance.Coin, balance.Action, balance.Amount, balance.BalanceTime.String()))
		//if balance.Amount > 1 {
		//	model.AppDB.Save(&balance)
		//}
		//}
		util.Notice(fmt.Sprintf(`get markets %d balances %d`, len(markets), len(balances)))
		time.Sleep(time.Hour * 12)
	}
}

func MaintainTransFee() {
	for {
		var orders []model.Order
		for {
			d, _ := time.ParseDuration("-240h")
			dMin10, _ := time.ParseDuration("-10m")
			now := util.GetNow()
			lastDays2 := now.Add(d)
			lastMin10 := now.Add(dMin10)
			model.AppDB.Limit(500).Offset(feeIndex).Where(
				`created_at>? and created_at<? and status=? and refresh_type!=? and refresh_type!=? and refresh_type!=? and refresh_type!=?`,
				lastDays2, lastMin10, model.CarryStatusWorking, model.FunctionDCarry, model.FunctionCross, model.FunctionComplement, model.FunctionSimulation).
				Find(&orders)
			util.Info(fmt.Sprintf(`--- get working orders %d %v %v`, len(orders), lastDays2, lastMin10))
			if len(orders) == 0 {
				break
			}
			feeIndex += len(orders)
			for i, value := range orders {
				util.Info(fmt.Sprintf(`%d --- id %s`, i, value.OrderId))
				account := model.AppConfig.GetAccountFromKeyIndex(value.Market, ``, value.AccountIndex)
				if account == nil {
					util.Notice(`can not maintain order status for nil account %s %s`, value.Market, value.AccountIndex)
					continue
				}
				order := api.QueryOrderById(account.Key, account.Secret, value.Market, value.Symbol, value.OrderType, value.OrderId)
				if order == nil {
					//if value.Market == model.Ftx && (strings.Contains(value.RefreshType, `carry`) ||
					//	strings.Contains(value.RefreshType, `comp`)) {
					//	value.Status = model.CarryStatusNotWorking
					//	model.AppDB.Save(&value)
					//}
					continue
				}
				value.OrderUpdateTime = order.OrderUpdateTime
				value.Fee = order.Fee
				value.DealAmount = order.DealAmount
				if order.Status != `` {
					value.Status = order.Status
				}
				if order.Status == model.CarryStatusSuccess {
					api.SetTurtleOrderStatus(value.RefreshType, value.Market, value.Symbol, value.OrderId, order.Status)
				}
				value.DealPrice = order.DealPrice
				model.AppDB.Save(&value)
				util.Info(fmt.Sprintf(`save order %s %s %s %s status:%s update %s`,
					value.OrderId, value.Symbol, value.OrderSide, value.OrderTime.String(), value.Status, value.OrderUpdateTime.String()))
				time.Sleep(time.Second)
			}
		}
		feeIndex = 0
		time.Sleep(time.Minute * 5)
	}
}

func ClearChannels(market string, chanMap *sync.Map) {
	accounts := model.AppConfig.GetAccounts(market)
	for _, account := range accounts {
		if account == nil {
			continue
		}
		util.DelSyncMap(&model.AppEnvironment.AccountConns, market, account.Key)
	}
	if chanMap != nil {
		channels, _ := chanMap.Load(market)
		for i, channel := range channels.([]chan struct{}) {
			util.Notice(`send to stop connection %s %d`, market, i)
			channel <- struct{}{}
			close(channel)
		}
		chanMap.Delete(market)
	}
}

func MaintainMarketChan() (reset bool) {
	for _, market := range api.GetMarkets() {
		depthChans, _ := model.AppEnvironment.MsgChanTick.Load(market)
		marketReset := false
		clearDepthConns := false
		if depthChans == nil || len(depthChans.([]chan struct{})) == 0 {
			marketReset = true
			model.AppEnvironment.MsgChanTick.Store(market, api.CreateMarketTickerWS(model.AppEnvironment, market))
			model.AppEnvironment.WsInitTime.Store(market, util.GetNow())
			util.Notice(fmt.Sprintf("%s create new depth channel ", market))
		} else if api.RequireDepthChanReset(model.AppEnvironment, market) {
			reset = true
			marketReset = true
			clearDepthConns = true
			util.Notice(fmt.Sprintf("%s require new depth channel ", market))
			model.ChannelMaintaining.Store(market, true)
			ClearChannels(market, &model.AppEnvironment.MsgChanTick)
			model.AppEnvironment.MsgChanTick.Store(market, api.CreateMarketTickerWS(model.AppEnvironment, market))
			model.AppEnvironment.WsInitTime.Store(market, util.GetNow())
			model.ChannelMaintaining.Store(market, false)
			util.Notice(market + " reset depth channel done")
		}
		accounts := model.AppConfig.GetAccounts(market)
		for _, account := range accounts {
			value, _ := util.LoadSyncMap(&model.AppEnvironment.AccountConns, market, account.Key)
			if value == nil || marketReset {
				api.CreateAccountWsServer(market)
			}
		}
		settings := api.GetSettings(model.FunctionKLine, market)
		doKLine := false
		clearKLineConns := false
		if settings != nil {
			settings.Range(func(symbol, setting any) bool {
				doKLine = true
				return false
			})
		}
		if doKLine {
			klineWS, _ := model.AppEnvironment.MsgChanKLine.Load(market)
			if klineWS == nil || len(klineWS.([]chan struct{})) == 0 {
				model.AppEnvironment.MsgChanKLine.Store(market, api.CreateMarketKLineWS(market))
				util.Notice(fmt.Sprintf("create new kline channel %s", market))
			} else if api.RequireKLineReset(model.AppEnvironment, market) {
				reset = true
				clearKLineConns = true
				util.Notice(fmt.Sprintf("%s require new kline channel ", market))
				model.ChannelMaintaining.Store(market, true)
				ClearChannels(market, &model.AppEnvironment.MsgChanKLine)
				model.AppEnvironment.MsgChanTick.Store(market, api.CreateMarketKLineWS(market))
				model.ChannelMaintaining.Store(market, false)
				util.Notice(market + " reset depth channel done")
			}
		}
		if clearKLineConns && clearDepthConns {
			model.AppEnvironment.SocketsTick.Delete(market)
		}
	}
	return reset
}

func Maintain() {
	util.Notice("start carrying")
	model.HandlerMap[model.FunctionTurtle] = Turtle.ProcessTurtle
	model.HandlerMap[model.FunctionCross] = cross.ProcessCross
	model.HandlerMap[model.FunctionHang] = hang.ProcessHang
	model.HandlerMap[model.FunctionCombineTurtle] = Turtle.ProcessCombineTurtle
	model.HandlerMap[model.FunctionGrid] = grid.ProcessGrid
	model.HandlerMap[model.FunctionQueue] = queue.ProcessQueue
	model.HandlerMap[model.FunctionFollow] = follow.ProcessFollow
	model.AccountHandlerMap[model.FunctionGrid] = grid.ProcessGridOrder
	model.AccountHandlerMap[model.FunctionQueue] = queue.ProcessQueueLiq
	model.AccountHandlerMap[model.FunctionCross] = cross.PostOrderCross
	_ = model.AppDB.AutoMigrate(&model.Setting{})
	_ = model.AppDB.AutoMigrate(&model.Order{})
	_ = model.AppDB.AutoMigrate(&model.Balance{})
	_ = model.AppDB.AutoMigrate(&model.Candle{})
	//model.HandlerMap[model.FunctionDCarry] = dreprecated2.ProcessDCarry
	//model.HandlerMap[model.FunctionHang] = dreprecated2.ProcessHang
	//api.CancelOrders(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx, `LINK-PERP`)
	//go CheckPastRefresh()
	//go util.StartMidNightTimer(CancelAllOrders)
	//go MaintainBalance()
	go MaintainTransFee()
	api.InitApp(true)
	go monitor.KLineServer()
	//go func() {
	//	for true {
	//		time.Sleep(time.Hour * 24)
	//		markets := api.GetMarkets()
	//		for _, market := range markets {
	//			api.InitMarketInfos(market)
	//		}
	//	}
	//}()
	for {
		if MaintainMarketChan() {
			time.Sleep(time.Millisecond * time.Duration(model.AppConfig.Delay*2))
		} else {
			time.Sleep(time.Millisecond * time.Duration(model.AppConfig.Delay/10))
		}
	}
}

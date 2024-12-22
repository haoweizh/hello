package carry

import (
	"fmt"
	"hello/api"
	"hello/carry/Turtle"
	"hello/carry/cross"
	"hello/carry/monitor"
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
		util.Log(util.LogLevelInfo, fmt.Sprintf(`get markets %d balances %d`, len(markets), len(balances)))
		time.Sleep(time.Hour * 12)
	}
}

// MaintainTransFee
func _() {
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
			util.Log(util.LogLevelInfo, fmt.Sprintf(`--- get working orders %d %#v %#v`, len(orders), lastDays2, lastMin10))
			if len(orders) == 0 {
				break
			}
			feeIndex += len(orders)
			for i, value := range orders {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`%d --- id %s`, i, value.OrderId))
				account := model.AppConfig.GetAccountFromKeyIndex(value.Market, ``, value.AccountIndex)
				if account == nil {
					util.Log(util.LogLevelError, fmt.Sprintf(`can not maintain order status for nil account %s %s`, value.Market, value.AccountIndex))
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
				util.Log(util.LogLevelInfo, fmt.Sprintf(`save order %s %s %s %s status:%s update %s`,
					value.OrderId, value.Symbol, value.OrderSide, value.OrderTime.String(), value.Status, value.OrderUpdateTime.String()))
				time.Sleep(time.Second)
			}
		}
		feeIndex = 0
		time.Sleep(time.Minute * 5)
	}
}

func ClearChannels(market string, chanMap *sync.Map) {
	if chanMap != nil {
		channels, _ := chanMap.Load(market)
		if channels != nil {
			for i, channel := range channels.([]chan struct{}) {
				util.Log(util.LogLevelInfo, fmt.Sprintf(`send to stop connection %s %d`, market, i))
				channel <- struct{}{}
				close(channel)
			}
			chanMap.Delete(market)
		}
	}
}

func ManageConnTicks(market string) (reset bool) {
	depthChans, _ := model.AppEnvironment.MsgChanTick.Load(market)
	if depthChans == nil || len(depthChans.([]chan struct{})) == 0 {
		api.CreateWSTick(model.AppEnvironment, market)
	} else if api.RequireConnTickReset(model.AppEnvironment, market) {
		reset = true
		ClearChannels(market, &model.AppEnvironment.MsgChanTick)
		api.CreateWSTick(model.AppEnvironment, market)
	}
	//var settingMonitors []*model.SettingMonitor
	//model.AppDB.Find(&settingMonitors)
	//kLines := make(map[string]map[string]bool)
	//monitor.RefreshSettingMonitors(model.AppEnvironment, settingMonitors)
	//for _, settingMonitor := range settingMonitors {
	//	if kLines[settingMonitor.Market] == nil {
	//		kLines[settingMonitor.Market] = make(map[string]bool)
	//	}
	//	kLines[settingMonitor.Market][settingMonitor.Symbol] = true
	//}
	//for marketKline, symbols := range kLines {
	//	klineWS, _ := model.AppEnvironment.MsgChanKLine.Load(marketKline)
	//	if klineWS == nil || len(klineWS.([]chan struct{})) == 0 {
	//		api.CreateMarketKLineWS(model.AppEnvironment, marketKline, symbols)
	//	} else if api.RequireKLineReset(model.AppEnvironment, marketKline, symbols) {
	//		reset = true
	//		ClearChannels(marketKline, &model.AppEnvironment.MsgChanKLine)
	//		api.CreateMarketKLineWS(model.AppEnvironment, marketKline, symbols)
	//	}
	//}
	return reset
}

func Maintain() {
	util.Log(util.LogLevelInfo, "start carrying")
	model.TickHandlers[model.FunctionTurtle] = Turtle.ProcessTurtle
	model.TickHandlers[model.FunctionCross] = cross.ProcessCross
	model.TickHandlers[model.FunctionCombineTurtle] = Turtle.ProcessCombineTurtle
	model.AccountHandlerMap[model.FunctionCross] = cross.PostOrderCross
	model.CandleHandlers[model.FunctionMonitorKLine] = monitor.ProcessMonitor
	_ = model.AppDB.AutoMigrate(&model.Setting{})
	_ = model.AppDB.AutoMigrate(&model.Order{})
	_ = model.AppDB.AutoMigrate(&model.Balance{})
	_ = model.AppDB.AutoMigrate(&model.Candle{})
	_ = model.AppDB.AutoMigrate(&model.SettingMonitor{})
	//model.TickHandlers[model.FunctionDCarry] = dreprecated2.ProcessDCarry
	//model.TickHandlers[model.FunctionHang] = dreprecated2.ProcessHang
	//api.CancelOrders(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx, `LINK-PERP`)
	//go CheckPastRefresh()
	//go util.StartMidNightTimer(CancelAllOrders)
	//go MaintainBalance()
	//go MaintainTransFee()
	api.InitApp(true)
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
		for _, market := range api.GetMarkets() {
			go ManageConnTicks(market)
		}
		time.Sleep(time.Minute * 1)
	}
}

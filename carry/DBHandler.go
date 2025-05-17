package carry

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"hello/api"
	"hello/carry/Turtle"
	"hello/carry/cross"
	"hello/carry/monitor"
	"hello/model"
	"hello/util"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"
)

var feeIndex int
var balanceMaintainDay = util.GetNow()

// MaintainBalance
func _(account *model.Account) {
	for !util.Terminal {
		markets := model.AppEnvironment.Markets
		balances := make([]*model.Balance, 0)
		balanceTime := util.GetNow()
		duration, _ := time.ParseDuration(`-24h`)
		balanceTime = balanceTime.Add(duration)
		for _, market := range markets {
			if balanceTime.After(balanceMaintainDay) {
				balances = append(balances, api.GetTransfers(account, market)...)
				balanceMaintainDay = util.GetNow()
			}
			_, balance, _, _, _ := api.GetBalances(account, market)
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
	for !util.Terminal {
		var orders []model.Order
		for !util.Terminal {
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
				order := api.QueryOrderById(account, value.Market, value.Symbol, value.OrderType, value.OrderId)
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

func ManageConnTicks(market string) (reset bool) {
	initialized := false
	if market != model.Gate {
		value, _ := model.AppEnvironment.ConnTick.Load(api.GetPublicConnKey(market, ``))
		if value != nil {
			initialized = true
		}
	} else {
		valueSpot, _ := model.AppEnvironment.ConnTick.Load(api.GetPublicConnKey(model.Gate, model.MarketTypeSpot))
		valuePerp, _ := model.AppEnvironment.ConnTick.Load(api.GetPublicConnKey(model.Gate, model.MarketTypePerp))
		if valueSpot != nil && valuePerp != nil {
			initialized = true
		}
	}
	if !initialized {
		api.CreateWSTick(model.AppEnvironment, market)
	} else if api.RequireConnTickReset(model.AppEnvironment, market) {
		reset = true
		api.CreateWSTick(model.AppEnvironment, market)
		//if api.GetSpecialChan(market) == 1 {
		//	if market == model.Gate {
		//		model.SubRecover(market, model.MarketTypePerp, model.ChanTypeMarket)
		//		time.Sleep(time.Second)
		//		model.SubRecover(market, model.MarketTypeSpot, model.ChanTypeMarket)
		//	} else {
		//		model.SubRecover(market, ``, model.ChanTypeMarket)
		//	}
		//	util.Log(util.LogLevelLocal, fmt.Sprintf(`special chan reset %s`, market))
		//}
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
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	model.TickHandlers[model.FunctionTurtle] = Turtle.ProcessTurtle
	model.TickHandlers[model.FunctionCross] = cross.ProcessCross
	model.TickHandlers[model.FunctionCombineTurtle] = Turtle.ProcessCombineTurtle
	model.TickHandlers[model.FunctionMove] = ProcessMove
	model.AccountHandlerMap[model.FunctionCross] = cross.PostOrderCross
	model.CandleHandlers[model.FunctionMonitorKLine] = monitor.ProcessMonitor
	model.CollateralHandler = cross.ProcessCollateral
	model.CrossBalancesHandler = cross.ProcessCrossBalances
	model.CrossPositionsHandler = cross.ProcessCrossPositions
	model.ADLHandler = cross.ProcessADL
	_ = model.AppDB.AutoMigrate(&model.Setting{})
	_ = model.AppDB.AutoMigrate(&model.Order{})
	_ = model.AppDB.AutoMigrate(&model.Balance{})
	_ = model.AppDB.AutoMigrate(&model.Candle{})
	_ = model.AppDB.AutoMigrate(&model.SettingMonitor{})
	_ = model.AppDB.AutoMigrate(&model.CarryCoin{})
	_ = model.AppDB.AutoMigrate(&model.FundingFee{})
	_ = model.AppDB.AutoMigrate(&model.Fund{})
	//go CheckPastRefresh()
	//go util.StartMidNightTimer(CancelAllOrders)
	//go MaintainBalance()
	//go MaintainTransFee()
	api.InitApp()
	initCross := false
	for _, market := range model.AppEnvironment.Markets {
		crossSettings := api.GetSettings(model.FunctionCross, market)
		if crossSettings != nil {
			initCross = true
		}
	}
	if initCross && model.AppConfig.Handle == `1` {
		go cross.ContinueComp()
		go cross.ClearCross()
		c := cron.New()
		_, err := c.AddFunc("15,45 * * * ?", cross.ClearCross)
		if err != nil {
			util.Log(util.LogLevelError, `fail to cron clear cross `+err.Error())
		}
		_, errMarketInfo := c.AddFunc("5,15,35,45 * * * ?", cross.RefreshMarkets)
		if errMarketInfo != nil {
			util.Log(util.LogLevelError, `fail to cron refresh market info `+errMarketInfo.Error())
		}
		_, errSyncFees := c.AddFunc("5 * * * ?", cross.SyncFees)
		if errSyncFees != nil {
			util.Log(util.LogLevelError, `fail to cron sync fees `+errSyncFees.Error())
		}
		_, errSaveFund := c.AddFunc("3,13,23,33,43,53 * * * ?", SaveFunc)
		if errSaveFund != nil {
			util.Log(util.LogLevelError, `fail to cron save fund `+errSaveFund.Error())
		}
		c.Start()
	}
	// 监听信号的 goroutine
	go func() {
		sig := <-sigs
		fmt.Println(time.Now().String()+"Received signal:", sig)
		fmt.Println("Gracefully shutting down...")
		util.Terminal = true
	}()
	go ManageMarketConnTicks()
	var pprofTime time.Time
	for !util.Terminal {
		time.Sleep(time.Second * 10)
		if time.Now().Sub(pprofTime) > time.Second*3600 {
			pprofTime = time.Now()
			fileName := fmt.Sprintf(`mem%d%d%d%d%d.profile`, pprofTime.Year(), pprofTime.Month(), pprofTime.Day(), pprofTime.Hour(), pprofTime.Minute())
			f, _ := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0644)
			err := pprof.Lookup("heap").WriteTo(f, 0)
			if err != nil {
				fmt.Println(time.Now().String() + "fail to lookup heap" + err.Error())
			}
			err = f.Close()
		}
	}
	fmt.Println(time.Now().String() + "Cleanup done. Exiting.")
}

func ManageMarketConnTicks() {
	for !util.Terminal {
		for _, market := range model.AppEnvironment.Markets {
			go ManageConnTicks(market)
		}
		time.Sleep(time.Minute * 1)
	}
}

var SaveFunc = func() {
	seconds := time.Now().Unix()
	accountsLen := api.GetAccountsLen()
	for i := 0; i < accountsLen; i++ {
		accounts := model.GetAccounts(i)
		for market, account := range accounts {
			inAllSpot, contractAccountValue, holdingSpot, borrowSpot, holdingFuture, marginAvailable :=
				cross.GetCrossMarketValue(account, market, false)
			value := contractAccountValue
			if !account.IsUnified && market == model.BinanceSpot {
				value = inAllSpot
			}
			fund := model.Fund{
				Market:        market,
				Key:           account.Key,
				Seconds:       seconds,
				Index:         account.Index,
				Value:         value,
				HoldingSpot:   holdingSpot,
				HoldingFuture: holdingFuture,
				BorrowSpot:    borrowSpot,
				Available:     marginAvailable,
			}
			model.AppDB.Save(&fund)
		}
	}
}

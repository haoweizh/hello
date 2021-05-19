package carry

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/api"
	"hello/model"
	"hello/util"
	"time"
)

var feeIndex int
var balanceMaintainDay = util.GetNow()

//MaintainBalance
func _() {
	for true {
		markets := model.GetMarkets()
		balances := make([]*model.Balance, 0)
		balanceTime := util.GetNow()
		duration, _ := time.ParseDuration(`-24h`)
		balanceTime = balanceTime.Add(duration)
		for _, market := range markets {
			if balanceTime.After(balanceMaintainDay) {
				balances = append(balances, api.GetTransfers(``, ``, market)...)
				balanceMaintainDay = util.GetNow()
			}
			_, balance, _, _ := api.GetBalances(``, ``, market, 0)
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

func RefreshMarketInfo() {
	for true {
		time.Sleep(time.Hour * 8)
		if !api.InitMarketInfos() {
			util.Notice(`fatal error: can not set okex account mode to net!!`)
		}
	}
}

func MaintainTransFee(key, secret string) {
	for true {
		var orders []model.Order
		for true {
			d, _ := time.ParseDuration("-48h")
			now := util.GetNow()
			lastDays2 := now.Add(d)
			model.AppDB.Limit(100).Offset(feeIndex).Where(
				`date(order_time)>? and status=? and refresh_type!=? and refresh_type!=? and refresh_type!=?`,
				lastDays2, model.CarryStatusWorking, model.FunctionComplement, model.FunctionCarry, model.FunctionDCarry).Find(&orders)
			if len(orders) == 0 {
				break
			}
			util.Info(fmt.Sprintf(`--- get working orders %d`, len(orders)))
			feeIndex += len(orders)
			for _, value := range orders {
				order := api.QueryOrderById(key, secret, value.Market, value.Symbol, value.Instrument,
					value.OrderType, value.OrderId)
				if order == nil {
					continue
				}
				value.Fee = order.Fee
				value.FeeIncome = order.FeeIncome
				value.DealAmount = order.DealAmount
				if order.Status != `` {
					value.Status = order.Status
				}
				value.DealPrice = order.DealPrice
				model.AppDB.Save(&value)
				util.Info(fmt.Sprintf(`save order %s %s %s %s`,
					value.Symbol, value.OrderSide, value.OrderTime.String(), value.Status))
				time.Sleep(time.Second)
			}
		}
		feeIndex = 0
		time.Sleep(time.Minute * 5)
	}
}

var socketMaintaining = false

func ResetChannels(market string, channels []chan struct{}) {
	model.AppPause = true
	model.AppMarkets.PutDepthChan(market, nil)
	for _, channel := range channels {
		channel <- struct{}{}
		close(channel)
	}
	model.AppMarkets.PutDepthChan(market, api.CreateMarketDepthServer(model.AppMarkets, market, postOrderCarry))
	model.AppPause = false
	util.SocketInfo(market + " reset depth channel ")
}

func MaintainMarketChan() {
	if socketMaintaining {
		return
	}
	socketMaintaining = true
	for _, market := range model.GetMarkets() {
		channels := model.AppMarkets.GetDepthChan(market)
		if channels == nil || len(channels) == 0 {
			model.AppMarkets.PutDepthChan(market, api.CreateMarketDepthServer(model.AppMarkets, market, postOrderCarry))
			util.Notice(fmt.Sprintf("%s create new depth channel ", market))
		} else {
			if api.RequireDepthChanReset(model.AppMarkets, market) {
				util.Notice(fmt.Sprintf("%s require new depth channel ", market))
				ResetChannels(market, channels)
				time.Sleep(time.Minute)
			}
		}
	}
	socketMaintaining = false
}

func Maintain() {
	util.Notice("start carrying")
	var err error
	model.AppDB, err = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	if err != nil {
		util.Notice(err.Error())
		return
	}
	model.HandlerMap[model.FunctionGrid] = ProcessSimpleGrid
	model.HandlerMap[model.FunctionTurtle] = ProcessTurtle
	model.HandlerMap[model.FunctionCarry] = ProcessCarry
	model.HandlerMap[model.FunctionDCarry] = ProcessDCarry
	_ = model.AppDB.AutoMigrate(&model.Setting{})
	_ = model.AppDB.AutoMigrate(&model.Order{})
	_ = model.AppDB.AutoMigrate(&model.Balance{})
	//api.CancelOrders(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx, `LINK-PERP`)
	//go CheckPastRefresh()
	go MaintainTransFee(model.KeyDefault, model.SecretDefault)
	//go util.StartMidNightTimer(CancelAllOrders)
	//go MaintainBalance()
	api.InitMarketInfos()
	go RefreshMarketInfo()
	for true {
		go MaintainMarketChan()
		time.Sleep(time.Duration(model.AppConfig.ChannelSlot) * time.Millisecond)
	}
}

package carry

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"hello/api"
	"hello/model"
	"hello/util"
	"strings"
	"time"
)

var WSErrHandler = func(err error) {
	print(err)
	util.SocketInfo(`get error ` + err.Error())
}

//CancelAllOrders
//var _ = func() {
//	model.AppPause = true
//	time.Sleep(time.Second)
//	markets := model.GetMarkets()
//	for _, market := range markets {
//		symbols := model.GetMarketSymbols(market)
//		for symbol := range symbols {
//			util.Notice(fmt.Sprintf(`[cancel old orders] %s %s`, market, symbol))
//			orders := api.QueryOrders(model.KeyDefault, model.SecretDefault, market, symbol, model.CarryStatusWorking,
//				model.AccountTypeLever+model.AccountTypeNormal, 0, 0)
//			for _, order := range orders {
//				if order != nil && order.OrderId != `` {
//					result, errCode, msg, _ := api.CancelOrder(model.KeyDefault, model.SecretDefault,
//						market, symbol, ``, order.OrderId)
//					util.Notice(fmt.Sprintf(`[cancel old]%v %s %s`, result, errCode, msg))
//					time.Sleep(time.Millisecond * 100)
//				}
//			}
//		}
//	}
//	model.LoadSettings()
//	model.AppPause = false
//}

func discountBalance(market, symbol, accountType, coin string, discountRate float64) {
	leverMarket := market
	if accountType == model.AccountTypeLever {
		leverMarket = fmt.Sprintf(`%s_%s_%s`, market, model.AccountTypeLever,
			strings.Replace(symbol, `_`, ``, 1))
	}
	account := model.AppAccounts.GetAccount(leverMarket, coin)
	if account != nil {
		util.Notice(fmt.Sprintf(`discount account %s %s %f`, market, coin, discountRate))
		account.Free = account.Free * discountRate
		model.AppAccounts.SetAccount(leverMarket, coin, account)
	}
}

func getBalance(key, secret, market, symbol, accountType string) (
	left, right, leftFroze, rightFroze float64, err error) {
	leverMarket := market
	if accountType == model.AccountTypeLever {
		leverMarket = fmt.Sprintf(`%s_%s_%s`, market, model.AccountTypeLever,
			strings.Replace(symbol, `_`, ``, 1))
	}
	coins := strings.Split(symbol, `_`)
	leftAccount := model.AppAccounts.GetAccount(leverMarket, coins[0])
	if leftAccount == nil {
		util.Notice(`nil account ` + market + coins[0])
		//time.Sleep(time.Second * 2)
		api.RefreshAccount(key, secret, market)
		return 0, 0, 0, 0, errors.New(`no left balance`)
	}
	rightAccount := model.AppAccounts.GetAccount(leverMarket, coins[1])
	if rightAccount == nil {
		util.Notice(`nil account ` + market + coins[1])
		//time.Sleep(time.Second * 2)
		api.RefreshAccount(key, secret, market)
		return 0, 0, 0, 0, errors.New(`no right balance`)
	}
	return leftAccount.Free, rightAccount.Free, leftAccount.Frozen, rightAccount.Frozen, nil
}

//func RefreshAccounts() {
//	for true {
//		if model.AppConfig.Handle == `1` {
//			model.AppConfig.Handle = `0`
//			time.Sleep(time.Second * 5)
//			model.AppConfig.Handle = `1`
//		}
//		markets := model.GetMarkets()
//		timestamp := util.GetNow()
//		for _, value := range markets {
//			api.RefreshAccount(value)
//			if model.AppAccounts.Data[value] == nil {
//				continue
//			}
//			//accounts.MarketTotal[marketName] = 0
//			symbols := model.GetMarketSymbols(value)
//			for key, account := range model.AppAccounts.Data[value] {
//				if symbols[key+"_usdt"] || key == `usdt` {
//					account.PriceInUsdt, _ = api.GetPrice(key + "_usdt")
//					account.Timestamp = timestamp
//				}
//			}
//			model.AccountChannel <- model.AppAccounts.Data[value]
//		}
//		time.Sleep(time.Hour)
//	}
//}

var feeIndex int
var balanceMaintainDay = util.GetNow()

func MaintainBalance() {
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
			_, balance := api.GetBalances(``, ``, market, 0)
			balances = append(balances, balance...)
			for _, item := range balance {
				key := fmt.Sprintf(`[balance]%s_%s`, item.Market, item.Coin)
				if item.Amount > 10 || strings.ToLower(item.Coin) == `btc` {
					model.SetCarryInfo(key, fmt.Sprintf(`%f %s`, item.Amount, item.BalanceTime.String()))
				}
			}
			util.Notice(fmt.Sprintf(`get balances %s %d`, market, len(balances)))
		}
		for _, balance := range balances {
			util.Notice(fmt.Sprintf(`balance info: %s %s %s %f %f %s`,
				balance.ID, balance.Market, balance.Coin, balance.Action, balance.Amount, balance.BalanceTime.String()))
			if balance.Amount > 1 {
				model.AppDB.Save(&balance)
			}
		}
		util.Notice(fmt.Sprintf(`get markets %d balances %d`, len(markets), len(balances)))
		time.Sleep(time.Hour * 4)
	}
}

func MaintainTransFee(key, secret string) {
	for true {
		d, _ := time.ParseDuration("-48h")
		lastDays2 := util.GetNow().Add(d)
		//d, _ = time.ParseDuration(`-1h`)
		//lastHour := util.GetNow().Add(d)
		var orders []model.Order
		for true {
			model.AppDB.Limit(100).Offset(feeIndex).Where(
				`date(order_time)>? and status=? and refresh_type!=?`,
				lastDays2, model.CarryStatusWorking, model.FunctionCarry).Find(&orders)
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
			time.Sleep(time.Minute * 5)
		}
		feeIndex = 0
	}
}

func createMarketDepthServer(markets *model.Markets, market string) chan struct{} {
	util.SocketInfo(" create depth chan for " + market)
	var channel chan struct{}
	var err error
	switch market {
	case model.Huobi:
		channel, err = api.WsDepthServeHuobi(markets, WSErrHandler)
	case model.HuobiDM:
		channel, err = api.WsDepthServeHuobiDM(markets, WSErrHandler)
	case model.OKEX:
		channel, err = api.WsDepthServeOkex(markets, WSErrHandler)
	case model.OKFUTURE:
		channel, err = api.WsDepthServeOKFuture(markets, WSErrHandler)
	case model.OKSwap:
		channel, err = api.WsDepthServeOKSwap(markets, WSErrHandler)
	case model.Binance:
		channel, err = api.WsDepthServeBinance(markets, WSErrHandler)
	case model.Coinpark:
		channel, err = api.WsDepthServeCoinpark(markets, WSErrHandler)
	case model.Bitmex:
		channel, err = api.WsDepthServeBitmex(markets, WSErrHandler)
	case model.Bybit:
		channel, err = api.WsDepthServeBybit(markets, WSErrHandler)
	case model.Ftx:
		channel, err = api.WsDepthServeFtx(markets, WSErrHandler)
	case model.DFuture:
		channel, err = api.WsDepthServeDFuture(markets, WSErrHandler)
	}
	if err != nil {
		util.SocketInfo(market + ` can not create depth server ` + err.Error())
	}
	return channel
}

var socketMaintaining = false

func ResetChannel(market string, channel chan struct{}) {
	model.AppPause = true
	model.AppMarkets.PutDepthChan(market, 0, nil)
	channel <- struct{}{}
	close(channel)
	model.AppMarkets.PutDepthChan(market, 0, createMarketDepthServer(model.AppMarkets, market))
	model.AppPause = false
	util.SocketInfo(market + " reset depth channel ")
}

func MaintainMarketChan() {
	if socketMaintaining {
		return
	}
	socketMaintaining = true
	for _, market := range model.GetMarkets() {
		channel := model.AppMarkets.GetDepthChan(market, 0)
		if channel == nil {
			model.AppMarkets.PutDepthChan(market, 0, createMarketDepthServer(model.AppMarkets, market))
			util.Notice(fmt.Sprintf("%s create new depth channel ", market))
		} else {
			if api.RequireDepthChanReset(model.AppMarkets, market) {
				util.Notice(fmt.Sprintf("%s require new depth channel ", market))
				ResetChannel(market, channel)
			}
		}
	}
	socketMaintaining = false
}

func Maintain() {
	util.Notice("start carrying")
	var err error
	model.AppDB, err = gorm.Open("postgres", model.AppConfig.DBConnection)
	if err != nil {
		util.Notice(err.Error())
		return
	}
	model.HandlerMap[model.FunctionGrid] = ProcessSimpleGrid
	model.HandlerMap[model.FunctionRefresh] = ProcessRefresh
	model.HandlerMap[model.FunctionTurtle] = ProcessTurtle
	model.HandlerMap[model.FunctionCarry] = ProcessCarry
	model.HandlerMap[model.FunctionHang] = ProcessHang
	model.HandlerMap[model.FunctionRank] = ProcessRank
	model.HandlerMap[model.FunctionHangFar] = ProcessHangFar
	model.HandlerMap[model.FunctionPostonlyHandler] = PostonlyHandler
	defer model.AppDB.Close()
	if model.AppConfig.Env != `test` {
		model.AppDB.AutoMigrate(&model.Account{})
		model.AppDB.AutoMigrate(&model.Setting{})
		model.AppDB.AutoMigrate(&model.Order{})
		model.AppDB.AutoMigrate(&model.Score{})
		model.AppDB.AutoMigrate(&model.Candle{})
		model.AppDB.AutoMigrate(&model.Balance{})
		model.AppDB.AutoMigrate(&model.FundingRate{})
	}
	model.LoadSettings()
	api.InitMarketInfos()
	//go CheckPastRefresh()
	go MaintainTransFee(model.KeyDefault, model.SecretDefault)
	//go util.StartMidNightTimer(CancelAllOrders)
	go MaintainBalance()
	for true {
		go MaintainMarketChan()
		time.Sleep(time.Duration(model.AppConfig.ChannelSlot) * time.Millisecond)
	}
}

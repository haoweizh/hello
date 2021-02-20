package main

import (
	"fmt"
	"github.com/jinzhu/configor"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"hello/api"
	"hello/carry"
	"hello/model"
	"hello/util"
	"math"
	"strings"
	"testing"
)

func Test_chan(t *testing.T) {
	model.NewConfig()
	var err error
	model.AppDB, err = gorm.Open("postgres", model.AppConfig.DBConnection)
	if err != nil {
		util.Notice(err.Error())
		return
	}
	defer model.AppDB.Close()
	carry.MaintainMarketChan()
}

func returnParam(a int) (returnA int) {
	a = 2
	return a
}

func Test_initTurtleN(t *testing.T) {
	a := 1
	a = returnParam(a)
	fmt.Println(a)
	//model.NewConfig()
	//_ = configor.Load(model.AppConfig, "./config.yml")
	//balances := api.GetTransfers(``, ``, model.OKEX)
	//balances = append(balances, api.GetBalance(``, ``, model.OKEX)...)
	//var err error
	//model.AppDB, err = gorm.Open("postgres", model.AppConfig.DBConnection)
	//if err != nil {
	//	util.Notice(err.Error())
	//	return
	//}
	//model.AppDB.AutoMigrate(&model.Candle{})
	//order := api.QueryOrderById(``, ``, model.OKFUTURE, `btcusd_p`, `BTC-USD-201225`,
	//	model.OrderTypeStop, `6024698277970944`)
	//fmt.Println(order.Status)
	//today := time.Now().In(time.UTC)
	//fmt.Println(today.String())
	//today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	//symbol := `ETH-USD-200605`
	//instrument, _ := api.GetCurrentInstrument(model.OKFUTURE, symbol)
	//api.GetDayCandle(``, ``, model.OKFUTURE, symbol, instrument, today)
	//for i := 100; i > 0; i-- {
	//	d, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24*i))
	//	index := today.Add(d)
	//	fmt.Println(index.String())
	//	//api.GetDayCandle(`I9ZmxUz8KsgH6AekmsdQtIdZ33T7bH7SPg_WuBsD`, `WtGav2ou_f9HYUT4B9zj66kig7dJW8t1GEmsgFJp`,
	//	//	model.Ftx, `ethusd_p`, index)
	//	//api.GetDayCandle(`I9ZmxUz8KsgH6AekmsdQtIdZ33T7bH7SPg_WuBsD`, `WtGav2ou_f9HYUT4B9zj66kig7dJW8t1GEmsgFJp`,
	//	//	model.Ftx, `btcusd_p`, index)
	//	//api.GetDayCandle(`I9ZmxUz8KsgH6AekmsdQtIdZ33T7bH7SPg_WuBsD`, `WtGav2ou_f9HYUT4B9zj66kig7dJW8t1GEmsgFJp`,
	//	//	model.Ftx, `eosusd_p`, index)
	//	//api.GetDayCandle(`I9ZmxUz8KsgH6AekmsdQtIdZ33T7bH7SPg_WuBsD`, `WtGav2ou_f9HYUT4B9zj66kig7dJW8t1GEmsgFJp`,
	//	//	model.Ftx, `htusd_p`, index)
	//	//api.GetDayCandle(`I9ZmxUz8KsgH6AekmsdQtIdZ33T7bH7SPg_WuBsD`, `WtGav2ou_f9HYUT4B9zj66kig7dJW8t1GEmsgFJp`,
	//	//	model.Ftx, `bnbusd_p`, index)
	//	//api.GetDayCandle(`I9ZmxUz8KsgH6AekmsdQtIdZ33T7bH7SPg_WuBsD`, `WtGav2ou_f9HYUT4B9zj66kig7dJW8t1GEmsgFJp`,
	//	//	model.Ftx, `okbusd_p`, index)
	//}
	//fmt.Println(`done`)
	////go carry.CheckPastRefresh()
	////for true {
	////	time.Sleep(time.Minute)
	////}
	////d, _ := time.ParseDuration("-24h")
	////timeLine := util.GetNow().Add(d)
	////before := util.GetNow().Unix()
	////after := timeLine.Unix()
	////orders := api.QueryOrders(model.Fcoin, `eos_usdt`, model.CarryStatusWorking, before, after)
	////for _, order := range orders {
	////	if order != nil && order.OrderId != `` {
	////		//result, errCode, msg := api.CancelOrder(market, symbol, order.OrderId)
	////		util.Notice(fmt.Sprintf(`[cancel old]%v %s %f`, true, order.OrderId, order.Price))
	////		time.Sleep(time.Millisecond * 100)
	////	}
	////}
	////api.QueryOrderDealsFcoin(`3BgqYy6o70gMlDiCgH0JJEEynoJPqYnz5SZSq-No0EhA2-D4pKe6BB0RqdfJ0fXTDCfKUfhBVHyAFphKAWwylA==`)
	////orders := api.QueryOrders(model.Fcoin, `btc_usdt`, `success`,
	////	1557529200, 1557504000)
	////for _, value := range orders {
	////	util.Notice(fmt.Sprintf(`,symbol:%s,%s,%s,%s,%s,%f,%f,%f,%f`,
	////		value.Symbol, value.OrderTime.String(), value.Function, value.OrderSide, value.Status,
	////		value.DealAmount, value.DealPrice, value.Fee, value.FeeIncome))
	////}
}

func Test_wallet(t *testing.T) {
	model.NewConfig()
	_ = configor.Load(model.AppConfig, "./config.yml")

	keys, secrets := model.AppConfig.GetKeys(model.Huobi)
	key := keys[0]
	secret := secrets[0]
	fmt.Println(key)
	fmt.Println(secret)
	model.AppDB, _ = gorm.Open("postgres", model.AppConfig.DBConnection)
	a := api.GetMarketInfo(model.Ftx, `SECO/USD`)
	fmt.Println(a)
	api.InitCarryFtx(1)
	api.GetFundingRate(model.Ftx, `BTC-PERP`)
	api.RefreshAccount(``, ``, model.Ftx)
	_, balances := api.GetBalances(``, ``, model.Ftx, 0)
	for _, balance := range balances {
		if balance.Coin == `USD` {
			fmt.Println(fmt.Sprintf(`free %f `, balance.Amount))
		}
	}
	symbols := model.GetMarketSymbols(model.Ftx)
	usdAvailable := 0.0
	holding := 0.0
	for _, value := range balances {
		if strings.ToLower(value.Coin) == `usd` {
			usdAvailable = value.Amount
		} else if symbols[strings.ToUpper(value.Coin)+`/USD`] {
			holding += math.Abs(value.UsdValue)
		}
	}
	fmt.Println(usdAvailable)
	fmt.Println(holding)

	//carry.GetTurtleData(model.Ftx, `okbusd_p`)
	//var err error
	//model.AppDB, err = gorm.Open("postgres", model.AppConfig.DBConnection)
	//if err != nil {
	//	util.Notice(err.Error())
	//	return
	//}
	//today := time.Now().In(time.UTC)
	//today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	//api.GetDayCandle(model.AppConfig.BitmexKey, model.AppConfig.BitmexSecret, model.Bitmex, `btcusd_p`, today)
	//api.GetDayCandle(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx, `htusd_p`, today)
	//balanceUSD := api.GetWalletHistoryFtx(model.AppConfig.FtxKey, model.AppConfig.FtxSecret)
	//balanceUSD := api.GetUSDBalance(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx)
	//fmt.Print(balanceUSD)
	//api.RefreshAccount(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx)
	//order := api.QueryOrderById(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx,
	//	`btcusd_p`, model.OrderTypeStop, `903993`)
	//fmt.Print(order.DealPrice)
	//amount, transfer := api.GetWalletHistoryBitmex(model.AppConfig.BitmexKey, model.AppConfig.BitmexSecret)
	//fmt.Println(fmt.Sprintf("%f \n%s", amount, transfer))
	//fmt.Println(api.GetWalletBybit(model.AppConfig.BybitKey, model.AppConfig.BybitSecret))
	//balance := api.GetWalletOKSwap(model.AppConfig.OkexKey, model.AppConfig.OkexSecret)
	//for symbol, amount := range balance {
	//	if amount > 0 {
	//		info := api.GetWalletHistoryOKSwap(model.AppConfig.OkexKey, model.AppConfig.OkexSecret, symbol)
	//		fmt.Println(fmt.Sprintf("%s %f\n %s", symbol, amount, info))
	//	}
	//}
}

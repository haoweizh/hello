package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/jinzhu/configor"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"hello/api"
	"hello/model"
	"net/url"
	"testing"
	"time"
)

func timeWriter(conn *websocket.Conn) {
	for {
		time.Sleep(time.Second * 2)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(time.Now().Format("2006-01-02 15:04:05")+`ping`))
	}
}

func Test_ws(t *testing.T) {
	var addr = flag.String("addr", "ec2-18-179-17-108.ap-northeast-1.compute.amazonaws.com:443", "http service address")
	//var addr = flag.String("addr", "localhost:443", "http service address")

	u := url.URL{Scheme: "wss", Host: *addr, Path: "/wss"}
	var dialer *websocket.Dialer

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	go timeWriter(conn)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("read:", err)
			return
		}
		fmt.Printf("received: %s\n", message)
	}
}

func Test_initTurtleN(t *testing.T) {
	begin := time.Now()
	time.Sleep(time.Second * 5)

	duration := begin.Sub(time.Now())
	fmt.Println(duration)
	model.NewConfig()
	_ = configor.Load(model.AppConfig, "./config.yml")
	//balances := api.GetTransfers(``, ``, model.OKEX)
	//balances = append(balances, api.GetBalance(``, ``, model.OKEX)...)
	//var err error
	model.AppDB, _ = gorm.Open("postgres", model.AppConfig.DBConnection)
	//if err != nil {
	//	util.Notice(err.Error())
	//	return
	//}
	symbols := model.GetSettingCoins(model.FunctionCarry, model.Ftx)
	fmt.Println(len(symbols))
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
	balance := api.GetBalance(``, ``, model.OKEX, `USDT`, 0)
	fmt.Println(balance.Amount)
	fmt.Println(balance.Borrow)
	model.AppDB, _ = gorm.Open("postgres", model.AppConfig.DBConnection)
	api.InitCoinBalance(``, ``, model.FunctionCarry, model.OKEX)
	//api.InitMarketInfos()
	//order := api.PlaceOrder(``, ``, model.OrderSideSell, model.OrderTypeMarket, model.OKEX, `BTC-USDT-SWAP`,
	//	`BTC-USDT-SWAP`, ``, ``, model.FunctionCarry, 5555.23452, 0, 0.1444444444876, false)
	//fmt.Println(order.OrderId)
	//suc, pos := api.GetPositions(``, ``, model.Ftx)
	//fmt.Println(fmt.Sprintf(`%v %d`, suc, len(pos)))
	//a := api.GetMarketInfo(model.Ftx, `SECO/USD`)
	//fmt.Println(a)
	//carry.InitFtxBalance(`ZK6_FPdUIhDnjv_JWklHAkgWXRRindBw-qIg18bU`,
	//	`4bbFXcuVk_VE2JJ0_mnLFa3J-kcCOeUVM0sRmYN5`, model.FunctionCarry)
	api.InitCarryFtx(1)
	api.GetFundingRate(model.Ftx, `BTC-PERP`)
	//carry.GetTurtleData(model.Ftx, `okbusd_p`)
	//var err error
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

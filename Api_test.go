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
	"hello/util"
	"math"
	"net/url"
	"strconv"
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

func getAmount(amountIn float64) {
	amount := api.FormatAmountPair(model.OKEX, `BTT-USDT-SWAP`, `BTT-USDT`, amountIn)
	amountInPerp := api.GetAmountInPerpOKEX(model.OKEX, `BTT-USDT-SWAP`, amount)
	_, amountInReal := api.ParseRealAmount(model.OKEX, `BTT-USDT-SWAP`, amountInPerp)
	amount = math.Min(amount, amountInReal)
	amount = api.FormatAmountPair(model.OKEX, `BTT-USDT-SWAP`, `BTT-USDT`, amount)
	fmt.Println(fmt.Sprintf(`%f %f`, amountIn, amount))
}

func Test_OKFormatAmount(t *testing.T) {
	model.NewConfig()
	_ = configor.Load(model.AppConfig, "./config.yml")
	model.AppDB, _ = gorm.Open("postgres", model.AppConfig.DBConnection)
	api.InitMarketInfos()

	getAmount(0.1)
	getAmount(0)
	getAmount(0.1)
	getAmount(4444)
	getAmount(9999)
	getAmount(19999)
	getAmount(2349999)

	price, decimal := api.FormatPrice(model.OKEX, `JST-USDT`, model.OrderSideBuy, 0.13175)
	priceStr := util.CutTailZero(strconv.FormatFloat(price, 'f', decimal, 64))
	fmt.Println(priceStr)
	//amountPerp, _ := api.FormatAmount(model.OKEX, `BTT-USDT-SWAP`, 285854.42347684357)
	//amountRelated, _ := api.FormatAmount(model.OKEX, `BTT-USDT`, 285854.42347684357)
	//_, amountPerp = api.ParseRealAmount(model.OKEX, `BTT-USDT-SWAP`, amountPerp)
	//amount = math.Min(amountPerp, amountRelated)
}

func Test_initTurtleN(t *testing.T) {
	a := 0.23423423
	fmt.Println(strconv.FormatFloat(a, 'f', -1, 64))
	fmt.Println(fmt.Sprintf(`%f`, a))
	amount := 27213.361367394158 / 10000
	formattedAmount := math.Round(amount/0.0000001) * 0.0000001
	amountStr := util.CutTailZero(fmt.Sprintf(`%f`, formattedAmount))
	fmt.Println(amountStr)
	model.NewConfig()
	_ = configor.Load(model.AppConfig, "./config.yml")
	_, buy, sell := api.GetMaxSize(``, ``, `TORN-USDT-SWAP`)
	fmt.Println(buy)
	fmt.Println(sell)
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
	rate, timeInt := api.GetFundingRate(model.OKEX, `BTC-USDT-SWAP`)
	fmt.Println(rate)
	fmt.Println(time.Unix(timeInt, 0).String())
	rate, timeInt = api.GetFundingRate(model.OKEX, `BTC-USDT-SWAP`)
	fmt.Println(rate)
	fmt.Println(time.Unix(timeInt, 0).String())
	//balance := api.GetBalance(``, ``, model.OKEX, `USDT`, 0)
	today := time.Now().In(time.UTC)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	duration, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24))
	today = today.Add(duration)
	candle := api.GetDayCandle(``, ``, model.OKEX, ``, `BTC-USDT-SWAP`, today)
	fmt.Println(candle.N)
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
	//carry.GetTurtleData(model.Ftx, `okbusd_p`)
	//var err error
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

package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/jinzhu/configor"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"hello/api"
	"hello/model"
	"hello/util"
	"net/url"
	"strconv"
	"testing"
	"time"
)

var msgChan = make(chan string, 10)

func init() {
	go handleMsg()
}

func handleMsg() {
	for true {
		msg := <-msgChan
		fmt.Println(fmt.Sprintf(`%s %d`, msg, len(msgChan)))
	}
}

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

//func getAmount(amountIn float64) {
//	amount := api.FormatAmountPair(model.OKEX, `BTT-USDT-SWAP`, `BTT-USDT`, amountIn)
//	amountInPerp := api.GetAmountInPerp(model.OKEX, `BTT-USDT-SWAP`, amount)
//	_, amountInReal := api.ParseRealAmount(model.OKEX, `BTT-USDT-SWAP`, amountInPerp)
//	amount = math.Min(amount, amountInReal)
//	amount = api.FormatAmountPair(model.OKEX, `BTT-USDT-SWAP`, `BTT-USDT`, amount)
//	fmt.Println(fmt.Sprintf(`%f %f`, amountIn, amount))
//}

func send() {
	for true {
		now := time.Now().Second()
		testChan <- now
		testChan <- now
		testChan <- now
		time.Sleep(time.Second)
	}
}

var testChan = make(chan int, 1000)

func Test_OKFormatAmount(t *testing.T) {
	go send()
	msg := 1
	for msg = range testChan {
		fmt.Println(fmt.Sprintf(`get %d length %d`, msg, len(testChan)))
		time.Sleep(time.Second * 5)
	}
	price, _ := strconv.ParseFloat(`5077.13021003`, 64)
	amount := util.CutTailZero(fmt.Sprintf(`%f`, price))
	fmt.Println(amount)
	strs := []string{`1`, `2`, `3`, `4`, `5`}
	for _, str := range strs {
		msgChan <- str
	}
	select {}
	//a := math.Floor(0.1 / 0.000001)
	//fmt.Println(a)
	//a = a * 0.000001
	//fmt.Println(a)
	//model.NewConfig()
	//_ = configor.Load(model.AppConfig, "./config.yml")
	//model.AppDB, _ = gorm.Open("postgres", model.AppConfig.DBConnection)
	//api.InitMarketInfos()
	//api.PlaceOrder(``, ``, model.OrderSideBuy, model.OrderTypeLimit, model.OKEX, `DORA-USDT`,
	//	`DORA-USDT`, ``, ``, 5555.23452, 0.1444444444876, 0.1, false, nil)
	//getAmount(0.1)
	//getAmount(0)
	//getAmount(0.1)
	//getAmount(4444)
	//getAmount(9999)
	//getAmount(19999)
	//getAmount(2349999)
}

func Test_initTurtleN(t *testing.T) {
	model.NewConfig()
	_ = configor.Load(model.AppConfig, "./config.yml")
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	fmt.Println(api.GetPosBal(``, ``, model.Binance))
	api.Transfer(``, ``, model.Binance, `MARGIN_UMFUTURE`, 75)
	api.GetFundingRate(model.Binance, `BTCUSDT`, nil)
	api.InitMarketInfos()
	//order := api.PlaceOrder(``, ``, model.OrderSideBuy, model.OrderTypeLimit, model.Binance, `BTCUSDT`,
	//	``, ``, ``, 41678.12345, 45678, 0.001, false, false, nil)
	//fmt.Println(order.OrderId)
	//api.CancelOrders(``, ``, model.Binance, `BTCUSDT`)
	api.GetPositions(``, ``, model.Binance)
	api.GetBalances(``, ``, model.Binance)
	//api.PlaceOrder(model.AppConfig.DFutureKey, model.AppConfig.DFutureSecret, model.OrderSideBuy, ``,
	//	model.DFuture, `ethusdt`, ``, `open`, model.FunctionDCarry, 2222, 2222, 0.1, false, false, nil)
}

func Test_wallet(t *testing.T) {
	model.NewConfig()
	_ = configor.Load(model.AppConfig, "./config.yml")
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos()
	amount := api.GetAmountInMarket(model.OKEX, `DOGE-USDT-SWAP`, 4)
	fmt.Println(amount)
	api.PlaceOrder(model.AppConfig.DFutureKey, model.AppConfig.DFutureSecret, model.OrderSideBuy, ``,
		model.DFuture, `ethusdt`, ``, `open`, model.FunctionDCarry, 2222, 2222, 0.1, true, false, nil)
	_, loan := api.GetMaxLoan(model.AppConfig.OkexKey, model.AppConfig.OkexSecret, model.OKEX, `XEM`)
	fmt.Println(loan)
	//balance := api.GetBalance(``, ``, model.OKEX, `USDT`, 0)
	today := time.Now().In(time.UTC)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	duration, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24))
	today = today.Add(duration)
	candle := api.GetDayCandle(``, ``, model.OKEX, ``, `BTC-USDT-SWAP`, today)
	fmt.Println(candle.N)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//api.InitCoinBalance(``, ``, model.FunctionCarry, model.OKEX)
	//api.InitMarketInfos()
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

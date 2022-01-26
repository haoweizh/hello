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
	"net/http"
	"net/url"
	"strings"
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

func Test_getCommonMarketInfos(t *testing.T) {
	model.NewConfig()
	_ = configor.Load(model.AppConfig, "./config.yml")
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitCrossMarketInfos()
}

func Test_initTurtleN(t *testing.T) {
	model.NewConfig()
	_ = configor.Load(model.AppConfig, "./config.yml")
	_, balances, total, collateral := api.GetBalances(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.Gate)
	fmt.Println(collateral)
	fmt.Println(total)
	for _, balance := range balances {
		if balance.Amount > 0 {
			fmt.Println(fmt.Sprintf(`%s %f %f`, balance.Coin, balance.Amount, balance.UsdValue))
		}
	}
	accounts := make([]*model.Account, 0)
	account := model.AppConfig.GetAccounts(model.Binance)[0]
	if account == nil {
		fmt.Println(`right`)
	} else {
		fmt.Println(`wrong` + account.Key)
	}
	accounts = append(accounts, account)
	accounts = append(accounts, nil)
	fmt.Println(len(accounts))
	result, _, _, order := api.CancelOrder(model.AppConfig.GateKey, model.AppConfig.GateSecret, `gate`, `SUN_USDT`, ``,
		model.OrderTypeLimit, `86007650678`)
	fmt.Println(result)
	fmt.Println(fmt.Sprintf(`%v %s %f`, result, order.Status, order.DealAmount))
	testOrder := api.QueryOrderById(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx, ``, ``, model.OrderTypeLimit, `82424115039`)
	fmt.Println(testOrder.Market)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//api.GetFundingRate(model.Binance, `BTCUSDT`, nil)
	api.InitMarketInfos()
	out := util.CutTailZero(fmt.Sprintf(`%f`, model.GetAmountInMarket(model.Huobi, `rsr-usdt`, 20, 0)))
	fmt.Println(out)
	//fmt.Println(order.OrderId)
	//api.PlaceOrder(model.AppConfig.DFutureKey, model.AppConfig.DFutureSecret, model.OrderSideBuy, ``,
}

func downList(workId, pageId int) (fileNum int) {
	targetUrl := fmt.Sprintf(`https://user-api.foundingaz.com/api/submit-work/student-works?work_id=%d&page=%d&limit=1000`, workId, pageId)
	headers := map[string]string{`Accept`: `application/json, text/plain, */*`, `Origin`: `https://xuexi.cyjiaomu.com`,
		`token`: `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ0b2tlbi1kYXRhIjoieDlFdWd0ZmhmLVZRRWxoMmhSV2pTQ09XWHkxNUJxZnBJZE1Jc2FFZy13YS13MEV5UHV1LWNyYktFVmZuc3FNMHk1ZHB3LVpvanJkd1N4Li0uRjRmcGoxbUlZOGhabWV2V2RPTDR1Q21NZFVaLWtxS3JUdkNqVnVtTE8uQ0JYUGlBQlBDcmRMYXN5Ry1RNFFsNXdGRWNhIn0.vwFkT2FBH4ck3KprFyfuxJxSdL29TQGxa09KvgvOjR4`}
	response, _ := util.HttpRequest(http.MethodGet, targetUrl, ``, headers, 200)
	json, _ := util.NewJSON(response)
	list := json.GetPath(`data`, `list`).MustArray()
	for _, value := range list {
		item := value.(map[string]interface{})[`file_list`]
		if item == nil {
			continue
		}
		files := item.([]interface{})
		for _, file := range files {
			fileValue := file.(map[string]interface{})[`content`]
			if fileValue != nil {
				if strings.Contains(strings.ToLower(fileValue.(string)), `review`) {
					util.Notice(fmt.Sprintf(`%s`, fileValue))
					fmt.Println(fileValue.(string))
					fileNum++
				}
			}
		}
	}
	return
}

func Test_download(t *testing.T) {
	for workId := 51; workId > 0; workId-- {
		//fmt.Println(fmt.Sprintf(`try work %d`, workId))
		for pageId := 1; pageId < 20; pageId++ {
			files := downList(workId, pageId)
			if files <= 0 {
				break
			}
			//fmt.Println(fmt.Sprintf(`get revew %d from work %d page %d`, files, workId, pageId))
		}
	}
	//curl 'https://user-api.foundingaz.com/api/submit-work/student-works?work_id=51&class_id=0&class_name=&group_id=0&group_name=&page=1&limit=10' \
	//-H 'Referer: https://xuexi.cyjiaomu.com/' \
	//-H 'Host: user-api.foundingaz.com' \
	//-H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.1 Safari/605.1.15' \
	//-H 'Accept-Language: zh-CN,zh-Hans;q=0.9' \
	//-H 'Accept-Encoding: gzip, deflate, br' \
	//-H 'Connection: keep-alive' \
}

func Test_wallet(t *testing.T) {
	model.NewConfig()
	//mi := api.GetMarketInfos(model.BybitSpot)
	//fmt.Println(len(mi))
	key, secret := `LWaglQgg6eiJDuTmwG`, `mOnuz8yeZmqGLUT2bNDqgA7kuhKT8QYOyUon`
	cancelResult := api.CancelOrders(key, secret, model.BybitSpot, `ETH-USDT`)
	fmt.Println(cancelResult)
	//api.CancelOrder(key, secret, model.BybitPerp, `ETH-PERP`, `ETH-PERP`, model.OrderTypeLimit, `bf928aac-c375-4d0d-9c00-c45584a91ac7`)
	orderBybit := api.QueryOrderById(key, secret, model.BybitPerp, `ETH-PERP`, `ETH-PERP`,
		model.OrderTypeLimit, `bf928aac-c375-4d0d-9c00-c45584a91ac7`)
	//marketInfos := api.GetMarketInfos(model.BybitPerp)
	//model.SetMarketInfos(model.BybitPerp, marketInfos)
	//orderBybit := api.PlaceOrder(key, secret, model.OrderSideBuy, model.OrderTypeLimit,
	//	model.BybitPerp, `ETH-PERP`, `ETH-PERP`, ``, ``, 2300.1234567, 2000.1234,
	//	0.012345678, false, false, nil, nil)
	fmt.Println(orderBybit.OrderId)
	_, rate := api.GetFundingRate(`LWaglQgg6eiJDuTmwG`, `mOnuz8yeZmqGLUT2bNDqgA7kuhKT8QYOyUon`, model.BybitPerp, `BTC-PERP`, nil)
	fmt.Println(fmt.Sprintf(`%f`, rate))
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos()
	orderQuery := api.QueryOrderById(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.Gate, `CFX_PERP`, `CFX_USDT`, model.OrderTypeLimit, `79852794326`)
	fmt.Println(orderQuery.OrderSide)
	order := api.PlaceOrder(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.OrderSideBuy, model.OrderTypeLimit,
		model.Gate, `ETH_USDT`, ``, ``, `carry`,
		2000, 2000, 0.1, true, false, nil, nil)
	fmt.Println(order.OrderId)
	api.CancelOrders(model.AppConfig.GateKey, model.AppConfig.GateSecret, model.Gate, `ETH_USDT`)
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos()
	model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	api.InitMarketInfos()
	fmt.Println(order.DealAmount)
	fmt.Println(order.DealPrice)
	fmt.Println(order.Status)
	//amount := api.GetAmountInMarket(model.OKEX, `DOGE-USDT-SWAP`, 4)
	//fmt.Println(amount)
	//_, loan := api.GetMaxLoan(model.AppConfig.OkexKey, model.AppConfig.OkexSecret, model.OKEX, `XEM`)
	//fmt.Println(loan)
	//today := time.Now().In(time.UTC)
	//today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	//duration, _ := time.ParseDuration(fmt.Sprintf(`%dh`, -24))
	//today = today.Add(duration)
	//fmt.Println(candle.N)
	//model.AppDB, _ = gorm.Open(postgres.Open(model.AppConfig.DBConnection), &gorm.Config{})
	//api.InitMarketInfos()
	//fmt.Println(fmt.Sprintf(`%v %d`, suc, len(pos)))
	//a := api.GetMarketInfo(model.Ftx, `SECO/USD`)
	account := model.AppConfig.GetAccounts(model.Ftx)[0]
	api.InitCarryFtx(account.Key, account.Secret, 1)
	//carry.GetTurtleData(model.Ftx, `okbusd_p`)
	//var err error
	//api.GetDayCandle(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx, `htusd_p`, today)
	//balanceUSD := api.GetWalletHistoryFtx(model.AppConfig.FtxKey, model.AppConfig.FtxSecret)
	//balanceUSD := api.GetUSDBalance(model.AppConfig.FtxKey, model.AppConfig.FtxSecret, model.Ftx)
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
